package jobs

import (
	_ "embed"
	"encoding/json"
	"fmt"

	"career-agent/internal/model"
)

//go:embed jobs.json
var raw []byte

// Store is the in-memory job corpus.
type Store struct {
	all  []model.Job
	byID map[string]model.Job
}

// Load reads the embedded corpus.
func Load() (*Store, error) {
	var all []model.Job
	if err := json.Unmarshal(raw, &all); err != nil {
		return nil, err
	}
	byID := make(map[string]model.Job, len(all))
	for _, job := range all {
		if job.ID == "" {
			return nil, fmt.Errorf("job missing id")
		}
		if _, ok := byID[job.ID]; ok {
			return nil, fmt.Errorf("duplicate job id %s", job.ID)
		}
		byID[job.ID] = job
	}
	return &Store{all: all, byID: byID}, nil
}

// All returns every opening.
func (s *Store) All() []model.Job {
	out := make([]model.Job, len(s.all))
	copy(out, s.all)
	return out
}

// Get returns one opening.
func (s *Store) Get(id string) (model.Job, bool) {
	job, ok := s.byID[id]
	return job, ok
}
