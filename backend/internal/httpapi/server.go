package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"career-agent/internal/agent"
	"career-agent/internal/model"
)

// New returns the HTTP handler for the MVP pipeline.
func New(svc *agent.Service) http.Handler {
	s := &server{svc: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/jobs", s.jobs)
	mux.HandleFunc("POST /api/profile", s.profile)
	mux.HandleFunc("POST /api/match", s.match)
	mux.HandleFunc("POST /api/diagnose", s.diagnose)
	mux.HandleFunc("POST /api/interview", s.interview)
	return withLog(withCORS(mux))
}

type server struct {
	svc *agent.Service
}

func (s *server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.svc.Health())
}

func (s *server) jobs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"jobs": s.svc.Jobs()})
}

func (s *server) profile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Degree string `json:"degree"`
		Resume string `json:"resume"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	result, err := s.svc.Parse(r.Context(), req.Degree, req.Resume)
	if err != nil {
		writeAgentErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) match(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Resume  string        `json:"resume"`
		Profile model.Profile `json:"profile"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	result, err := s.svc.Match(r.Context(), req.Resume, req.Profile)
	if err != nil {
		writeAgentErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) diagnose(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Resume   string           `json:"resume"`
		Profile  model.Profile    `json:"profile"`
		JobID    string           `json:"jobId"`
		Evidence []model.Evidence `json:"evidence"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	result, err := s.svc.Diagnose(r.Context(), req.Resume, req.Profile, req.JobID, req.Evidence)
	if err != nil {
		writeAgentErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) interview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Resume   string           `json:"resume"`
		Profile  model.Profile    `json:"profile"`
		JobID    string           `json:"jobId"`
		Evidence []model.Evidence `json:"evidence"`
		Messages []model.Turn     `json:"messages"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	result, err := s.svc.Interview(r.Context(), req.Resume, req.Profile, req.JobID, req.Evidence, req.Messages)
	if err != nil {
		writeAgentErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(dst); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体不是有效 JSON")
		return false
	}
	return true
}

func writeAgentErr(w http.ResponseWriter, err error) {
	var input *agent.InputError
	switch {
	case errors.As(err, &input):
		writeErr(w, http.StatusBadRequest, input.Error())
	case errors.Is(err, agent.ErrJobNotFound):
		writeErr(w, http.StatusNotFound, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write json: %v", err)
	}
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func withLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}
