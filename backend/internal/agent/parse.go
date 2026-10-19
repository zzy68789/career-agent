package agent

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"career-agent/internal/model"
)

// ParseResult is a structured resume reading.
type ParseResult struct {
	Profile model.Profile `json:"profile"`
	Mode    string        `json:"mode"`
	Warning string        `json:"warning,omitempty"`
}

const parseSystem = `你是简历结构化助手。只输出一个 JSON 对象，不要 Markdown。
不要编造简历里没有的公司、论文、技能和城市。找不到的字段用空字符串或空数组。
字段为 college, major, grade, name, skills, experiences, research, achievements, intentCities, intentRoles, summary。
experiences 每条一句话。summary 不超过 50 字。skills 只放简历明确写过的技能。`

// Parse turns resume text into a profile. degree is bachelor or master.
func (s *Service) Parse(ctx context.Context, degree, resume string) (ParseResult, error) {
	resume = strings.TrimSpace(resume)
	if err := validateResume(degree, resume); err != nil {
		return ParseResult{}, err
	}
	offline := parseOffline(degree, resume)
	if !s.llm.Configured() {
		return ParseResult{Profile: offline, Mode: "offline"}, nil
	}
	var parsed model.Profile
	user := fmt.Sprintf("学历以用户选择为准：%s。bachelor 是本科，master 是硕士。\n简历：\n---\n%s\n---", degree, resume)
	if err := s.chatJSON(ctx, parseSystem, user, 0.1, &parsed); err != nil {
		return ParseResult{
			Profile: offline,
			Mode:    "degraded",
			Warning: "模型解析失败，已改用本地规则：" + err.Error(),
		}, nil
	}
	parsed.Degree = degree
	if strings.TrimSpace(parsed.Major) == "" {
		parsed.Major = offline.Major
	}
	if strings.TrimSpace(parsed.College) == "" {
		parsed.College = offline.College
	}
	if strings.TrimSpace(parsed.Name) == "" {
		parsed.Name = offline.Name
	}
	cleanProfile(&parsed)
	if parsed.Summary == "" && len(parsed.Skills) == 0 && parsed.Major == "" {
		return ParseResult{
			Profile: offline,
			Mode:    "degraded",
			Warning: "模型没有抽出有效画像，已改用本地规则",
		}, nil
	}
	return ParseResult{Profile: parsed, Mode: "llm"}, nil
}

func cleanProfile(p *model.Profile) {
	p.Name = strings.TrimSpace(p.Name)
	p.College = strings.TrimSpace(p.College)
	p.Major = strings.TrimSpace(p.Major)
	p.Grade = strings.TrimSpace(p.Grade)
	p.Research = strings.TrimSpace(p.Research)
	p.Summary = clip(p.Summary, 80)
	p.Skills = cleanList(p.Skills, 12)
	p.Experiences = cleanList(p.Experiences, 6)
	p.Achievements = cleanList(p.Achievements, 4)
	p.IntentCities = cleanList(p.IntentCities, 4)
	p.IntentRoles = cleanList(p.IntentRoles, 4)
	p.FillStrings()
}

var (
	reCollege = regexp.MustCompile(`([\p{Han}]{2,40}学院)`)
	reMajor   = regexp.MustCompile(`学院([\p{Han}A-Za-z0-9]{2,20})专业`)
	reGrade   = regexp.MustCompile(`(20\d{2}\s*届)`)
	reName    = regexp.MustCompile(`^([\p{Han}]{1,4})同学`)
)

var roleTerms = []string{
	"嵌入式软件", "电气设计", "电力电子", "后端开发", "数据分析", "算法工程师",
	"云计算", "后端", "前端", "算法", "测试", "嵌入式", "电气",
}

var cityTerms = []string{"郑州", "北京", "上海", "杭州", "苏州", "西安", "深圳", "广州"}

func parseOffline(degree, resume string) model.Profile {
	focused := focusIntent(resume)
	p := model.Profile{
		Degree:       degree,
		Skills:       skillsIn(resume),
		IntentCities: citiesIn(focused),
		IntentRoles:  rolesIn(focused),
	}
	if m := reName.FindStringSubmatch(strings.TrimSpace(resume)); m != nil {
		p.Name = m[0]
	}
	if m := reCollege.FindStringSubmatch(resume); m != nil {
		p.College = m[1]
	}
	if m := reMajor.FindStringSubmatch(resume); m != nil {
		p.Major = m[1]
	}
	if m := reGrade.FindStringSubmatch(resume); m != nil {
		p.Grade = strings.ReplaceAll(m[1], " ", "")
	}
	var experiences, achievements []string
	for _, sentence := range sentences(resume) {
		switch {
		case strings.Contains(sentence, "论文") || strings.Contains(sentence, "竞赛") || strings.Contains(sentence, "获奖"):
			achievements = append(achievements, sentence)
		case strings.Contains(sentence, "研究"):
			if p.Research == "" {
				p.Research = sentence
			}
		case containsAny(sentence, []string{"实习", "项目", "课程", "完成", "负责", "参加", "实验"}):
			experiences = append(experiences, sentence)
		}
	}
	p.Experiences = experiences
	p.Achievements = achievements
	if lines := sentences(resume); len(lines) > 0 {
		p.Summary = lines[0]
	}
	cleanProfile(&p)
	return p
}

func focusIntent(text string) string {
	var parts []string
	for _, sentence := range sentences(text) {
		if strings.Contains(sentence, "意向") || strings.Contains(sentence, "求职") {
			parts = append(parts, sentence)
		}
	}
	if len(parts) == 0 {
		return text
	}
	return strings.Join(parts, "\n")
}

func citiesIn(text string) []string {
	if strings.Contains(text, "不限") {
		return []string{"不限"}
	}
	var out []string
	for _, city := range cityTerms {
		if strings.Contains(text, city) {
			out = append(out, city)
		}
	}
	return out
}

func rolesIn(text string) []string {
	var out []string
	used := map[string]bool{}
	for _, role := range roleTerms {
		if used[role] || !strings.Contains(text, role) {
			continue
		}
		skip := false
		for _, longer := range roleTerms {
			if longer != role && strings.Contains(longer, role) && strings.Contains(text, longer) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		used[role] = true
		out = append(out, role)
	}
	return out
}

func containsAny(s string, words []string) bool {
	for _, word := range words {
		if strings.Contains(s, word) {
			return true
		}
	}
	return false
}
