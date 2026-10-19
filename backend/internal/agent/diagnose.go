package agent

import (
	"context"
	"fmt"
	"strings"

	"career-agent/internal/model"
)

// DiagnoseResult is the one-page review for a chosen job.
type DiagnoseResult struct {
	Diagnosis model.Diagnosis `json:"diagnosis"`
	Job       model.Job       `json:"job"`
	Mode      string          `json:"mode"`
	Warning   string          `json:"warning,omitempty"`
}

const diagnoseSystem = `你是简历诊断助手。只输出一个 JSON 对象，不要 Markdown。
字段：summary, structureIssues, keywordGaps, rewrites。
summary 用两句话说明与目标岗位最主要的匹配和缺口。
structureIssues 是简历结构问题，没有则为空数组。
keywordGaps 是相对该岗位缺失的能力，用短词。
rewrites 给三句可以直接放进简历的改写。改写只能重组简历里已经写过的经历，不能新增没做过的实习、论文或技能。`

// Diagnose reviews the resume against one job.
func (s *Service) Diagnose(ctx context.Context, resume string, profile model.Profile, jobID string, evidence []model.Evidence) (DiagnoseResult, error) {
	resume = strings.TrimSpace(resume)
	if err := validateResume(profile.Degree, resume); err != nil {
		return DiagnoseResult{}, err
	}
	job, ok := s.jobs.Get(jobID)
	if !ok {
		return DiagnoseResult{}, ErrJobNotFound
	}
	cleanProfile(&profile)
	ev := evidenceFor(resume, job, evidence)
	offline := diagnoseOffline(resume, profile, job, ev)
	if !s.llm.Configured() {
		return DiagnoseResult{Diagnosis: offline, Job: job, Mode: "offline"}, nil
	}
	user := fmt.Sprintf("目标岗位：%s\n岗位要求：\n%s\n已核对的覆盖情况：\n%s\n简历：\n---\n%s\n---",
		job.Title, strings.Join(job.Requirements, "\n"), mustJSON(ev), resume)
	var got model.Diagnosis
	if err := s.chatJSON(ctx, diagnoseSystem, user, 0.2, &got); err != nil {
		return DiagnoseResult{
			Diagnosis: offline,
			Job:       job,
			Mode:      "degraded",
			Warning:   "模型诊断失败，已改用本地规则：" + err.Error(),
		}, nil
	}
	got = normalizeDiagnosis(got, offline)
	return DiagnoseResult{Diagnosis: got, Job: job, Mode: "llm"}, nil
}

func evidenceFor(resume string, job model.Job, evidence []model.Evidence) []model.Evidence {
	if len(evidence) == 0 {
		return heuristicEvidence(resume, job.Requirements)
	}
	return mergeEvidence(resume, job.Requirements, evidence)
}

func diagnoseOffline(resume string, profile model.Profile, job model.Job, evidence []model.Evidence) model.Diagnosis {
	_, covered := candidate{evidence: evidence}.coverage()
	gaps := gapRequirements(evidence)
	summary := fmt.Sprintf("与「%s」相比，%d 条要求已完全覆盖，%d 条仍有缺口。", job.Title, covered, len(gaps))
	var issues []string
	if len([]rune(resume)) < 80 {
		issues = append(issues, "简历过短，项目和实习缺少具体做法")
	}
	if !strings.Contains(resume, "实习") && !strings.Contains(resume, "项目") && !strings.Contains(resume, "课程") {
		issues = append(issues, "缺少项目、实习或课程实践描述")
	}
	if profile.College == "" && profile.Major == "" {
		issues = append(issues, "未识别到学院或专业，建议写在简历开头")
	}
	rewrites := make([]string, 0, 3)
	for _, gap := range gaps {
		if len(rewrites) == 3 {
			break
		}
		rewrites = append(rewrites, fmt.Sprintf("在已有经历中补充与「%s」相关的做法、工具和结果，不要写入没有做过的内容。", gap))
	}
	if len(rewrites) == 0 {
		rewrites = append(rewrites, "把已经匹配的经历补上量化结果，例如用户量、接口数量或实验指标。")
	}
	return model.Diagnosis{
		Summary:         summary,
		StructureIssues: cleanList(issues, 4),
		KeywordGaps:     cleanList(gaps, 6),
		Rewrites:        cleanList(rewrites, 3),
	}
}

func normalizeDiagnosis(got, fallback model.Diagnosis) model.Diagnosis {
	got.Summary = strings.TrimSpace(got.Summary)
	got.StructureIssues = cleanList(got.StructureIssues, 4)
	got.KeywordGaps = cleanList(got.KeywordGaps, 6)
	got.Rewrites = cleanList(got.Rewrites, 3)
	if got.Summary == "" {
		got.Summary = fallback.Summary
	}
	if len(got.Rewrites) == 0 {
		got.Rewrites = fallback.Rewrites
	}
	got.StructureIssues = nonNil(got.StructureIssues)
	got.KeywordGaps = nonNil(got.KeywordGaps)
	return got
}

func gapRequirements(evidence []model.Evidence) []string {
	var gaps []string
	for _, item := range evidence {
		if item.Status != "covered" {
			gaps = append(gaps, item.Requirement)
		}
	}
	return gaps
}

func nonNil(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}
