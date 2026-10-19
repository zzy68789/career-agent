package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNotConfigured means the process has no model credentials.
var ErrNotConfigured = errors.New("llm not configured")

// Client talks to an OpenAI-compatible chat and embedding API.
// Bailian and Volcengine Ark both work when BaseURL, APIKey and Model are set.
type Client struct {
	BaseURL    string
	APIKey     string
	Model      string
	EmbedModel string
	HTTP       *http.Client
}

// Configured reports whether chat calls can be made.
func (c *Client) Configured() bool {
	return c != nil && c.APIKey != "" && c.Model != "" && c.BaseURL != ""
}

// EmbeddingConfigured reports whether vector recall can be made.
func (c *Client) EmbeddingConfigured() bool {
	return c.Configured() && c.EmbedModel != ""
}

// Chat sends one system/user turn and returns the assistant text.
func (c *Client) Chat(ctx context.Context, system, user string, temperature float64) (string, error) {
	if !c.Configured() {
		return "", ErrNotConfigured
	}
	body := map[string]any{
		"model":       c.Model,
		"temperature": temperature,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
	}
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := c.post(ctx, "/chat/completions", body, &out); err != nil {
		return "", err
	}
	if out.Error != nil && out.Error.Message != "" {
		return "", errors.New(out.Error.Message)
	}
	if len(out.Choices) == 0 || strings.TrimSpace(out.Choices[0].Message.Content) == "" {
		return "", errors.New("model returned empty content")
	}
	return out.Choices[0].Message.Content, nil
}

// Embed returns one vector per input text, in the same order.
func (c *Client) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	if !c.EmbeddingConfigured() {
		return nil, ErrNotConfigured
	}
	body := map[string]any{
		"model": c.EmbedModel,
		"input": texts,
	}
	var out struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float64 `json:"embedding"`
		} `json:"data"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := c.post(ctx, "/embeddings", body, &out); err != nil {
		return nil, err
	}
	if out.Error != nil && out.Error.Message != "" {
		return nil, errors.New(out.Error.Message)
	}
	vecs := make([][]float64, len(texts))
	for _, item := range out.Data {
		if item.Index < 0 || item.Index >= len(texts) {
			return nil, fmt.Errorf("embedding index %d out of range", item.Index)
		}
		vecs[item.Index] = item.Embedding
	}
	for i, vec := range vecs {
		if len(vec) == 0 {
			return nil, fmt.Errorf("missing embedding for input %d", i)
		}
	}
	return vecs, nil
}

func (c *Client) post(ctx context.Context, path string, body any, dst any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := strings.TrimRight(c.BaseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 70 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 300 {
			msg = msg[:300]
		}
		return fmt.Errorf("model http %d: %s", res.StatusCode, msg)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("decode model response: %w", err)
	}
	return nil
}
