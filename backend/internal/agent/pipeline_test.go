package agent

import (
	"context"
	"strings"
	"testing"

	"career-agent/internal/jobs"
	"career-agent/internal/llm"
	"career-agent/internal/model"
)

const bachelorResume = `张同学，郑州大学计算机与人工智能学院软件工程专业，2026 届本科。
熟悉 Go 语言，课程中使用 MySQL 与 Redis 完成宿舍报修系统，负责接口和简单缓存。
曾在郑州一家软件公司实习三个月，使用 Git 做版本管理，在 Linux 上部署后端服务。
了解 Java，项目主力语言是 Go。没有前端框架经验。
求职意向：郑州，后端开发。`

const masterResume = `李同学，郑州大学计算机与人工智能学院计算机科学与技术专业，2026 届硕士。
研究方向为自然语言处理，熟悉 Python、PyTorch，完成过简历与岗位文本匹配实验。
一篇中文核心论文在投。本科阶段做过数据处理，SQL 使用一般。
求职意向：算法工程师，城市不限。`

const electricalResume = `王同学，郑州大学电气与信息工程学院电气工程及其自动化专业，2026 届本科。
课程设计完成小型供配电方案，能使用 AutoCAD 绘制电气图纸，了解 PLC。
参加过单片机实验，用 C 语言读取传感器并控制流水灯。
求职意向：郑州，电气设计或嵌入式软件。`

func testService(t *testing.T) *Service {
	t.Helper()
	store, err := jobs.Load()
	if err != nil {
		t.Fatal(err)
	}
	return New(store, &llm.Client{})
}

func TestOfflineSamplesRankExpectedJobs(t *testing.T) {
	svc := testService(t)
	ctx := context.Background()

	bachelor, err := svc.Parse(ctx, "bachelor", bachelorResume)
	if err != nil {
		t.Fatal(err)
	}
	if bachelor.Profile.Major != "软件工程" {
		t.Fatalf("major = %q", bachelor.Profile.Major)
	}
	if strings.Join(bachelor.Profile.IntentRoles, ",") != "后端开发" {
		t.Fatalf("roles = %v", bachelor.Profile.IntentRoles)
	}
	bMatch, err := svc.Match(ctx, bachelorResume, bachelor.Profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(bMatch.Matches) == 0 || bMatch.Matches[0].Job.ID != "cs-go-backend" {
		t.Fatalf("bachelor top = %v", ids(bMatch))
	}
	for _, match := range bMatch.Matches {
		if match.Job.MinDegree == "master" {
			t.Fatalf("bachelor saw master job %s", match.Job.ID)
		}
		assertSpans(t, bachelorResume, match)
	}

	master, err := svc.Parse(ctx, "master", masterResume)
	if err != nil {
		t.Fatal(err)
	}
	mMatch, err := svc.Match(ctx, masterResume, master.Profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(mMatch.Matches) == 0 || mMatch.Matches[0].Job.ID != "cs-algo-nlp" {
		t.Fatalf("master top = %v", ids(mMatch))
	}

	ee, err := svc.Parse(ctx, "bachelor", electricalResume)
	if err != nil {
		t.Fatal(err)
	}
	eMatch, err := svc.Match(ctx, electricalResume, ee.Profile)
	if err != nil {
		t.Fatal(err)
	}
	if len(eMatch.Matches) == 0 || eMatch.Matches[0].Job.ID != "ee-design" {
		t.Fatalf("ee top = %v", ids(eMatch))
	}
}

func TestUngroundedQuoteFallsBack(t *testing.T) {
	resume := "熟悉 Go 语言，并完成过课程项目。"
	req := "熟悉 Go 或 Java"
	got := mergeEvidence(resume, []string{req}, []model.Evidence{{
		Requirement: req,
		ResumeSpan:  "熟悉 Java 语言",
		Status:      "covered",
	}})
	if strings.Contains(got[0].ResumeSpan, "Java") {
		t.Fatalf("kept ungrounded span: %+v", got[0])
	}
	if got[0].ResumeSpan != "" && !spanInResume(resume, got[0].ResumeSpan) {
		t.Fatalf("span not in resume: %+v", got[0])
	}
}

func TestPunctuationKeepsSkillBoundaries(t *testing.T) {
	text := words("掌握 Go、Java 或 Python")
	for _, term := range []string{"Go", "Java", "Python"} {
		if !containsTerm(text, words(term)) {
			t.Fatalf("%s not found in %q", term, text)
		}
	}
}

func TestSQLDoesNotMatchMySQL(t *testing.T) {
	if containsTerm(normalize("熟悉 MySQL"), normalize("SQL")) {
		t.Fatal("SQL matched inside MySQL")
	}
	if !containsTerm(normalize("SQL 使用一般"), normalize("SQL")) {
		t.Fatal("SQL should match its own word")
	}
}

func TestExtractJSONFence(t *testing.T) {
	var out struct {
		A string `json:"a"`
	}
	if err := unmarshalJSON("说明如下：\n```json\n{\"a\":\"ok\"}\n```", &out); err != nil {
		t.Fatal(err)
	}
	if out.A != "ok" {
		t.Fatal(out.A)
	}
}

func ids(result MatchResult) []string {
	out := make([]string, 0, len(result.Matches))
	for _, match := range result.Matches {
		out = append(out, match.Job.ID)
	}
	return out
}

func assertSpans(t *testing.T, resume string, match model.Match) {
	t.Helper()
	for _, item := range match.Evidence {
		if item.ResumeSpan == "" {
			continue
		}
		if !spanInResume(resume, item.ResumeSpan) {
			t.Fatalf("%s span %q not in resume", match.Job.ID, item.ResumeSpan)
		}
	}
}
