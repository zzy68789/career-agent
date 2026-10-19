package agent

import (
	"encoding/json"
	"strings"
)

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func unmarshalJSON(text string, dst any) error {
	raw, err := extractJSON(text)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

func extractJSON(text string) (json.RawMessage, error) {
	s := strings.TrimSpace(text)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```JSON")
		s = strings.TrimPrefix(s, "```")
		if i := strings.LastIndex(s, "```"); i >= 0 {
			s = s[:i]
		}
		s = strings.TrimSpace(s)
	}
	start := strings.IndexAny(s, "{[")
	if start < 0 {
		return nil, errJSON
	}
	dec := json.NewDecoder(strings.NewReader(s[start:]))
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}
