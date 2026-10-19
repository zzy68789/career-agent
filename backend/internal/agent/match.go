package agent

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"career-agent/internal/model"
)

// MatchResult is the ranked shortlist.
type MatchResult struct {
	Matches    []model.Match `json:"matches"`
	Mode       string        `json:"mode"`
	Warning    string        `json:"warning,omitempty"`
	Considered int           `json:"considered"`
	Recall     string        `json:"recall"`
}

// Match ranks jobs that pass degree and major checks.
// Recall is lexical unless an embedding model is configured and responds.
// Coverage quotes must appear in the resume.
func (s *Service) Match(ctx context.Context, resume string, profile model.Profile) (MatchResult, error) {
	resume = strings.TrimSpace(resume)
	if err := validateResume(profile.Degree, resume); err != nil {
		return MatchResult{}, err
	}
	cleanProfile(&profile)
	passed := s.eligible(profile)
	result := MatchResult{
		Mode:       "offline",
		Considered: len(passed),
		Recall:     "lexical",
		Matches:    []model.Match{},
	}
	if s.llm.Configured() {
		result.Mode = "llm"
	}
	if len(passed) == 0 {
		return result, nil
	}

	semantic, recall, semErr := s.semantics(ctx, profile, resume, passed)
	result.Recall = recall
	if semErr != nil {
		result.Mode = "degraded"
		result.Warning = "向量召回失败，已改用字面相似度：" + semErr.Error()
		log.Printf("match recall: %v", semErr)
	}

	rows := make([]candidate, len(passed))
	for i, job := range passed {
		rows[i] = candidate{
			job:      job,
			semantic: semantic[i],
			evidence: heuristicEvidence(resume, job.Requirements),
		}
	}
	sortCandidates(rows, profile.IntentCities)
	if len(rows) > 8 {
		rows = rows[:8]
	}
	if s.llm.Configured() {
		merged, err := s.explain(ctx, resume, rows)
		if err != nil {
			result.Mode = "degraded"
			msg := "模型解释失败，已改用本地覆盖判断：" + err.Error()
			if result.Warning != "" {
				result.Warning += " " + msg
			} else {
				result.Warning = msg
			}
			log.Printf("match explain: %v", err)
		} else {
			rows = merged
		}
	}
	sortCandidates(rows, profile.IntentCities)
	if len(rows) > 5 {
		rows = rows[:5]
	}
	matches := make([]model.Match, 0, len(rows))
	for _, row := range rows {
		matches = append(matches, row.toMatch(profile.IntentCities))
	}
	result.Matches = matches
	return result, nil
}

func (s *Service) eligible(profile model.Profile) []model.Job {
	fams := familiesOf(profile.Major, profile.College)
	var passed []model.Job
	for _, job := range s.jobs.All() {
		if degreeOK(profile.Degree, job.MinDegree) && familyOK(fams, job.Families) {
			passed = append(passed, job)
		}
	}
	return passed
}

func degreeOK(profileDegree, minDegree string) bool {
	switch minDegree {
	case "", "bachelor":
		return profileDegree == "bachelor" || profileDegree == "master"
	case "master":
		return profileDegree == "master"
	default:
		return false
	}
}

func familiesOf(major, college string) []string {
	text := major + college
	var out []string
	if containsAny(text, []string{"计算机", "软件", "人工智能", "数据科学", "网络工程", "信息安全"}) {
		out = append(out, "cs")
	}
	if containsAny(text, []string{"电气", "自动化", "电子信息", "通信", "测控", "电力"}) {
		out = append(out, "ee")
	}
	return out
}

func familyOK(profileFamilies, jobFamilies []string) bool {
	if len(profileFamilies) == 0 || len(jobFamilies) == 0 {
		return true
	}
	for _, a := range profileFamilies {
		for _, b := range jobFamilies {
			if a == b {
				return true
			}
		}
	}
	return false
}

func cityMismatch(cities []string, jobCity string) bool {
	if len(cities) == 0 || jobCity == "" {
		return false
	}
	for _, city := range cities {
		if city == "不限" || strings.Contains(jobCity, city) || strings.Contains(city, jobCity) {
			return false
		}
	}
	return true
}

type candidate struct {
	job      model.Job
	semantic float64
	evidence []model.Evidence
}

func (c candidate) coverage() (float64, int) {
	if len(c.evidence) == 0 {
		return 0, 0
	}
	var sum float64
	covered := 0
	for _, item := range c.evidence {
		switch item.Status {
		case "covered":
			sum += 1
			covered++
		case "partial":
			sum += 0.5
		}
	}
	return sum / float64(len(c.evidence)), covered
}

func (c candidate) score(cities []string) float64 {
	cov, _ := c.coverage()
	s := 0.4*clamp01(c.semantic) + 0.6*cov
	if cityMismatch(cities, c.job.City) {
		s *= 0.95
	}
	return roundTo(s*100, 1)
}

func (c candidate) toMatch(cities []string) model.Match {
	cov, covered := c.coverage()
	explanation := fmt.Sprintf("硬性条件通过，完全覆盖 %d/%d。", covered, len(c.evidence))
	if cityMismatch(cities, c.job.City) {
		explanation += "意向城市与岗位不一致，综合分已略降。"
	}
	return model.Match{
		Job:         c.job,
		Score:       c.score(cities),
		Semantic:    roundTo(c.semantic, 2),
		Coverage:    roundTo(cov, 2),
		Covered:     covered,
		Total:       len(c.evidence),
		Evidence:    c.evidence,
		Explanation: explanation,
	}
}

func sortCandidates(rows []candidate, cities []string) {
	sort.SliceStable(rows, func(i, j int) bool {
		si, sj := rows[i].score(cities), rows[j].score(cities)
		if si == sj {
			return rows[i].job.ID < rows[j].job.ID
		}
		return si > sj
	})
}

func heuristicEvidence(resume string, requirements []string) []model.Evidence {
	out := make([]model.Evidence, 0, len(requirements))
	for _, req := range requirements {
		out = append(out, heuristicOne(resume, req))
	}
	return out
}

func heuristicOne(resume, requirement string) model.Evidence {
	reqTerms := termsIn(requirement)
	if len(reqTerms) == 0 {
		sim := cosine(features(resume), features(requirement))
		status := "missing"
		span := ""
		if sim >= 0.45 {
			status = "covered"
		} else if sim >= 0.22 {
			status = "partial"
		}
		if status != "missing" {
			span = bestSentence(resume, requirement)
			if !spanInResume(resume, span) {
				span = ""
				status = "partial"
			}
		}
		return model.Evidence{Requirement: requirement, ResumeSpan: span, Status: status}
	}
	hits := 0
	var hitTerms []string
	normResume := words(resume)
	for _, term := range reqTerms {
		if containsTerm(normResume, words(term)) {
			hits++
			hitTerms = append(hitTerms, term)
		}
	}
	status := "missing"
	switch {
	case hits == len(reqTerms):
		status = "covered"
	case hits > 0:
		status = "partial"
	}
	span := ""
	if status != "missing" {
		span = bestSentence(resume, strings.Join(hitTerms, " "))
		if !spanInResume(resume, span) {
			span = ""
		}
	}
	return model.Evidence{Requirement: requirement, ResumeSpan: span, Status: status}
}

func mergeEvidence(resume string, requirements []string, llmItems []model.Evidence) []model.Evidence {
	byReq := map[string]model.Evidence{}
	for _, item := range llmItems {
		byReq[item.Requirement] = item
	}
	out := make([]model.Evidence, 0, len(requirements))
	for _, req := range requirements {
		base := heuristicOne(resume, req)
		item, ok := byReq[req]
		if !ok {
			out = append(out, base)
			continue
		}
		switch item.Status {
		case "missing":
			out = append(out, model.Evidence{Requirement: req, Status: "missing"})
		case "covered", "partial":
			if spanInResume(resume, item.ResumeSpan) {
				out = append(out, model.Evidence{
					Requirement: req,
					ResumeSpan:  strings.TrimSpace(item.ResumeSpan),
					Status:      item.Status,
				})
			} else {
				out = append(out, base)
			}
		default:
			out = append(out, base)
		}
	}
	return out
}

type explainPayload struct {
	Jobs []struct {
		ID    string           `json:"id"`
		Items []model.Evidence `json:"items"`
	} `json:"jobs"`
}

const explainSystem = `你是岗位匹配说明助手。只输出一个 JSON 对象，不要 Markdown。
对每个岗位的每条要求判断简历是否覆盖。
status 只能是 covered、partial 或 missing。
resumeSpan 必须是简历中的连续原文。找不到就留空，并且 status 不能是 covered 或 partial。
不要编造简历里没有的经历。requirement 必须与输入原文一致。
输出形状：{"jobs":[{"id":"","items":[{"requirement":"","resumeSpan":"","status":"covered"}]}]}`

func (s *Service) explain(ctx context.Context, resume string, rows []candidate) ([]candidate, error) {
	type brief struct {
		ID           string   `json:"id"`
		Title        string   `json:"title"`
		Requirements []string `json:"requirements"`
	}
	briefs := make([]brief, 0, len(rows))
	for _, row := range rows {
		briefs = append(briefs, brief{ID: row.job.ID, Title: row.job.Title, Requirements: row.job.Requirements})
	}
	user := fmt.Sprintf("简历：\n---\n%s\n---\n岗位：\n%s", resume, mustJSON(briefs))
	var payload explainPayload
	if err := s.chatJSON(ctx, explainSystem, user, 0.1, &payload); err != nil {
		return nil, err
	}
	byID := map[string][]model.Evidence{}
	for _, job := range payload.Jobs {
		byID[job.ID] = job.Items
	}
	out := make([]candidate, len(rows))
	copy(out, rows)
	for i, row := range out {
		out[i].evidence = mergeEvidence(resume, row.job.Requirements, byID[row.job.ID])
	}
	return out, nil
}

func (s *Service) semantics(ctx context.Context, profile model.Profile, resume string, list []model.Job) ([]float64, string, error) {
	lexical := make([]float64, len(list))
	doc := profileDoc(profile, resume)
	docFeat := features(doc)
	for i, job := range list {
		lexical[i] = cosine(docFeat, features(jobDoc(job)))
	}
	if !s.llm.EmbeddingConfigured() {
		return lexical, "lexical", nil
	}
	vecs, err := s.embedJobs(ctx, list)
	if err != nil {
		return lexical, "lexical", err
	}
	profileVecs, err := s.llm.Embed(ctx, []string{doc})
	if err != nil {
		return lexical, "lexical", err
	}
	out := make([]float64, len(list))
	for i, job := range list {
		out[i] = cosineVec(profileVecs[0], vecs[job.ID])
	}
	return out, "embedding", nil
}

func (s *Service) embedJobs(ctx context.Context, list []model.Job) (map[string][]float64, error) {
	s.mu.Lock()
	if s.vecModel != s.llm.EmbedModel {
		s.jobVec = map[string][]float64{}
		s.vecModel = s.llm.EmbedModel
	}
	var missing []model.Job
	for _, job := range list {
		if _, ok := s.jobVec[job.ID]; !ok {
			missing = append(missing, job)
		}
	}
	s.mu.Unlock()
	if len(missing) > 0 {
		texts := make([]string, len(missing))
		for i, job := range missing {
			texts[i] = jobDoc(job)
		}
		vecs, err := s.llm.Embed(ctx, texts)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		for i, job := range missing {
			s.jobVec[job.ID] = vecs[i]
		}
		s.mu.Unlock()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string][]float64, len(list))
	for _, job := range list {
		out[job.ID] = s.jobVec[job.ID]
	}
	return out, nil
}

func profileDoc(profile model.Profile, resume string) string {
	return strings.Join([]string{
		profile.Major,
		profile.College,
		strings.Join(profile.Skills, " "),
		strings.Join(profile.Experiences, " "),
		profile.Research,
		strings.Join(profile.Achievements, " "),
		profile.Summary,
		resume,
	}, "\n")
}

func jobDoc(job model.Job) string {
	return job.Title + "\n" + strings.Join(job.Requirements, "\n") + "\n" + job.JD
}
