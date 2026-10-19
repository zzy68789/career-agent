package agent

import (
	"context"
	"errors"
	"strings"
	"sync"

	"career-agent/internal/jobs"
	"career-agent/internal/llm"
	"career-agent/internal/model"
)

// ErrJobNotFound means the id is outside the demo corpus.
var ErrJobNotFound = errors.New("岗位不存在")

// InputError is a request the user can fix.
type InputError struct{ Msg string }

func (e *InputError) Error() string { return e.Msg }

var errJSON = errors.New("模型输出不是 JSON")

// Service runs portrait, match, diagnosis and interview.
type Service struct {
	jobs *jobs.Store
	llm  *llm.Client

	mu       sync.Mutex
	jobVec   map[string][]float64
	vecModel string
}

// New builds a service. An unconfigured client keeps the pipeline on local rules.
func New(store *jobs.Store, client *llm.Client) *Service {
	if client == nil {
		client = &llm.Client{}
	}
	return &Service{jobs: store, llm: client, jobVec: map[string][]float64{}}
}

// Health describes which model path is available at startup.
type Health struct {
	OK        bool   `json:"ok"`
	Mode      string `json:"mode"`
	Model     string `json:"model,omitempty"`
	Embedding bool   `json:"embedding"`
}

// Health reports startup mode. A failed model call later is marked "degraded" on that result.
func (s *Service) Health() Health {
	if s.llm.Configured() {
		return Health{
			OK:        true,
			Mode:      "llm",
			Model:     s.llm.Model,
			Embedding: s.llm.EmbeddingConfigured(),
		}
	}
	return Health{OK: true, Mode: "offline"}
}

// Jobs returns the demo corpus.
func (s *Service) Jobs() []model.Job {
	return s.jobs.All()
}

func (s *Service) chatJSON(ctx context.Context, system, user string, temperature float64, dst any) error {
	if !s.llm.Configured() {
		return llm.ErrNotConfigured
	}
	text, err := s.llm.Chat(ctx, system, user, temperature)
	if err != nil {
		return err
	}
	if err := unmarshalJSON(text, dst); err != nil {
		retry, err2 := s.llm.Chat(ctx, system, user+"\n\n上次输出无法解析。这次只输出一个 JSON 对象，不要 Markdown。", temperature)
		if err2 != nil {
			return err
		}
		if err := unmarshalJSON(retry, dst); err != nil {
			return err
		}
	}
	return nil
}

func validateResume(degree, resume string) error {
	switch degree {
	case "bachelor", "master":
	default:
		return &InputError{Msg: "学历只能是本科或硕士"}
	}
	n := len([]rune(strings.TrimSpace(resume)))
	if n < 20 {
		return &InputError{Msg: "简历至少 20 个字"}
	}
	if n > 8000 {
		return &InputError{Msg: "简历请控制在 8000 字以内"}
	}
	return nil
}
