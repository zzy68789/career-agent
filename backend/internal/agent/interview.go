package agent

import (
	"context"
	"fmt"
	"strings"

	"career-agent/internal/model"
)

// InterviewResult is the next interviewer turn, or the closing score.
type InterviewResult struct {
	Message  model.Turn   `json:"message"`
	Finished bool         `json:"finished"`
	Score    *model.Score `json:"score,omitempty"`
	Mode     string       `json:"mode"`
	Warning  string       `json:"warning,omitempty"`
}

const questionSystem = `你是校招模拟面试官。只用中文提出一个问题，两句话以内。
问题要针对尚未覆盖的岗位要求，或追问学生刚刚说过的具体做法。
不要一次问多个问题，不要给分，不要写 JSON。`

const scoreSystem = `你是校招模拟面试官。只输出一个 JSON 对象，不要 Markdown。
字段：overall, strengths, improvements, summary。
overall 是 0 到 100 的整数。有具体做法和结果再给 70 分以上，空泛回答低于 60。
不要编造学生没说过的经历。`

// Interview continues a three-answer mock interview.
// An empty message list returns the opening question.
func (s *Service) Interview(ctx context.Context, resume string, profile model.Profile, jobID string, evidence []model.Evidence, messages []model.Turn) (InterviewResult, error) {
	resume = strings.TrimSpace(resume)
	if err := validateResume(profile.Degree, resume); err != nil {
		return InterviewResult{}, err
	}
	job, ok := s.jobs.Get(jobID)
	if !ok {
		return InterviewResult{}, ErrJobNotFound
	}
	cleanProfile(&profile)
	ev := evidenceFor(resume, job, evidence)
	messages = trimTurns(messages)
	if userCount(messages) >= 3 {
		return s.scoreInterview(ctx, profile, job, ev, messages)
	}
	return s.askInterview(ctx, profile, job, ev, messages)
}

func (s *Service) askInterview(ctx context.Context, profile model.Profile, job model.Job, evidence []model.Evidence, messages []model.Turn) (InterviewResult, error) {
	if !s.llm.Configured() {
		return InterviewResult{
			Message: model.Turn{Role: "assistant", Content: offlineQuestion(job, evidence, messages)},
			Mode:    "offline",
		}, nil
	}
	user := interviewContext(profile, job, evidence, messages) + "\n请提出下一个问题。"
	text, err := s.llm.Chat(ctx, questionSystem, user, 0.4)
	if err != nil {
		return InterviewResult{
			Message: model.Turn{Role: "assistant", Content: offlineQuestion(job, evidence, messages)},
			Mode:    "degraded",
			Warning: "模型提问失败，已改用本地问题：" + err.Error(),
		}, nil
	}
	text = strings.TrimSpace(text)
	if text == "" {
		text = offlineQuestion(job, evidence, messages)
	}
	return InterviewResult{Message: model.Turn{Role: "assistant", Content: clip(text, 240)}, Mode: "llm"}, nil
}

func (s *Service) scoreInterview(ctx context.Context, profile model.Profile, job model.Job, evidence []model.Evidence, messages []model.Turn) (InterviewResult, error) {
	offline := offlineScore(job, evidence, messages)
	finish := func(score model.Score, mode, warning string) InterviewResult {
		return InterviewResult{
			Message:  model.Turn{Role: "assistant", Content: score.Summary},
			Finished: true,
			Score:    &score,
			Mode:     mode,
			Warning:  warning,
		}
	}
	if !s.llm.Configured() {
		return finish(offline, "offline", ""), nil
	}
	user := interviewContext(profile, job, evidence, messages) + "\n学生已回答 3 次，请评分。"
	var got model.Score
	if err := s.chatJSON(ctx, scoreSystem, user, 0.2, &got); err != nil {
		return finish(offline, "degraded", "模型评分失败，已改用本地规则："+err.Error()), nil
	}
	got = normalizeScore(got, offline)
	return finish(got, "llm", ""), nil
}

func offlineQuestion(job model.Job, evidence []model.Evidence, messages []model.Turn) string {
	gaps := gapRequirements(evidence)
	n := userCount(messages)
	switch {
	case n == 0 && len(gaps) > 0:
		return fmt.Sprintf("你应聘%s。简历里还看不清「%s」。请举一个你实际做过的例子，说明做法和结果。", job.Title, gaps[0])
	case n == 0:
		return fmt.Sprintf("请用一个项目说明你为什么适合%s，重点讲你的职责和结果。", job.Title)
	case n == 1:
		return "补充你具体使用的工具、你负责的部分，以及最后的结果。不要只说参与了。"
	default:
		gap := "目前最弱的一块"
		if len(gaps) > 0 {
			gap = gaps[0]
		}
		return fmt.Sprintf("如果入职这个岗位，你第一个月准备怎么补上「%s」？", gap)
	}
}

func offlineScore(job model.Job, evidence []model.Evidence, messages []model.Turn) model.Score {
	var buf strings.Builder
	for _, turn := range messages {
		if turn.Role == "user" {
			buf.WriteString(turn.Content)
		}
	}
	text := buf.String()
	overall := 48
	if len([]rune(text)) > 50 {
		overall += 12
	}
	if len([]rune(text)) > 140 {
		overall += 10
	}
	if len(termsIn(text)) >= 2 {
		overall += 10
	}
	if len(termsIn(text)) >= 4 {
		overall += 8
	}
	if overall > 90 {
		overall = 90
	}
	var strengths []string
	for _, item := range evidence {
		if item.Status == "covered" && len(strengths) < 3 {
			strengths = append(strengths, item.Requirement)
		}
	}
	improvements := gapRequirements(evidence)
	if len(improvements) > 3 {
		improvements = improvements[:3]
	}
	summary := fmt.Sprintf("本轮围绕%s，回答长度为 %d 字。", job.Title, len([]rune(text)))
	if overall >= 70 {
		summary += "已经能讲到具体做法。"
	} else {
		summary += "还需要补上工具、个人职责和结果。"
	}
	return normalizeScore(model.Score{
		Overall:      overall,
		Strengths:    strengths,
		Improvements: improvements,
		Summary:      summary,
	}, model.Score{Overall: overall, Summary: summary})
}

func normalizeScore(got, fallback model.Score) model.Score {
	if got.Overall < 0 {
		got.Overall = 0
	}
	if got.Overall > 100 {
		got.Overall = 100
	}
	if got.Overall == 0 {
		got.Overall = fallback.Overall
	}
	got.Summary = strings.TrimSpace(got.Summary)
	if got.Summary == "" {
		got.Summary = fallback.Summary
	}
	got.Strengths = nonNil(cleanList(got.Strengths, 4))
	got.Improvements = nonNil(cleanList(got.Improvements, 4))
	got.Summary = clip(got.Summary, 120)
	return got
}

func interviewContext(profile model.Profile, job model.Job, evidence []model.Evidence, messages []model.Turn) string {
	var b strings.Builder
	fmt.Fprintf(&b, "岗位：%s\n要求：\n", job.Title)
	for _, req := range job.Requirements {
		fmt.Fprintf(&b, "- %s\n", req)
	}
	b.WriteString("覆盖情况：\n")
	for _, item := range evidence {
		fmt.Fprintf(&b, "- %s：%s\n", item.Requirement, item.Status)
	}
	fmt.Fprintf(&b, "学生：%s，%s，技能 %s\n对话：\n", profile.Major, profile.Grade, strings.Join(profile.Skills, "、"))
	if len(messages) == 0 {
		b.WriteString("（面试尚未开始）")
		return b.String()
	}
	for _, turn := range messages {
		speaker := "学生"
		if turn.Role == "assistant" {
			speaker = "面试官"
		}
		fmt.Fprintf(&b, "%s：%s\n", speaker, turn.Content)
	}
	return b.String()
}

func trimTurns(messages []model.Turn) []model.Turn {
	out := make([]model.Turn, 0, len(messages))
	for _, turn := range messages {
		content := strings.TrimSpace(turn.Content)
		if content == "" {
			continue
		}
		role := "user"
		if turn.Role == "assistant" {
			role = "assistant"
		}
		out = append(out, model.Turn{Role: role, Content: clip(content, 800)})
		if len(out) == 12 {
			break
		}
	}
	return out
}

func userCount(messages []model.Turn) int {
	n := 0
	for _, turn := range messages {
		if turn.Role == "user" {
			n++
		}
	}
	return n
}
