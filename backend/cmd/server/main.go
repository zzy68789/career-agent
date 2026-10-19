package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"career-agent/internal/agent"
	"career-agent/internal/httpapi"
	"career-agent/internal/jobs"
	"career-agent/internal/llm"
)

func main() {
	loadEnv("backend/.env")
	loadEnv(".env")
	store, err := jobs.Load()
	if err != nil {
		log.Fatal(err)
	}
	client := &llm.Client{
		BaseURL:    os.Getenv("LLM_BASE_URL"),
		APIKey:     os.Getenv("LLM_API_KEY"),
		Model:      os.Getenv("LLM_MODEL"),
		EmbedModel: os.Getenv("EMBEDDING_MODEL"),
	}
	svc := agent.New(store, client)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           httpapi.New(svc),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       20 * time.Second,
		WriteTimeout:      90 * time.Second,
	}
	health := svc.Health()
	log.Printf("career-agent listening on %s mode=%s embedding=%t", srv.Addr, health.Mode, health.Embedding)
	log.Fatal(srv.ListenAndServe())
}

func loadEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.Trim(strings.TrimSpace(val), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, val)
	}
}
