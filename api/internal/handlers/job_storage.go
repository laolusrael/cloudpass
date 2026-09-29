package handlers

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"cloudpass/internal/logger"
	"cloudpass/internal/models"
)

type JobStorage struct {
	filePath string
	mu       sync.RWMutex
	jobs     map[string]*models.Job
	// idempotency maps an Idempotency-Key header value to the job it
	// created. Rebuilt on load; entries die with their job.
	idempotency map[string]string
}

func NewJobStorage(dataDir string) (*JobStorage, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	filePath := filepath.Join(dataDir, "jobs.json")
	storage := &JobStorage{
		filePath:    filePath,
		jobs:        make(map[string]*models.Job),
		idempotency: make(map[string]string),
	}

	if err := storage.load(); err != nil {
		logger.API.Load().Warn().Err(err).Msg("failed to load jobs file, starting fresh")
	}

	return storage, nil
}

func (s *JobStorage) load() error {
	data, err := os.ReadFile(s.filePath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	var jobs []*models.Job
	if err := json.Unmarshal(data, &jobs); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range jobs {
		s.jobs[job.ID] = job
		if job.IdempotencyKey != "" {
			s.idempotency[job.IdempotencyKey] = job.ID
		}
	}

	logger.API.Load().Info().Int("count", len(s.jobs)).Msg("loaded jobs from storage")
	return nil
}

func (s *JobStorage) save() error {
	s.mu.RLock()
	jobs := make([]*models.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	s.mu.RUnlock()

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return err
	}

	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		return err
	}

	return nil
}

func (s *JobStorage) Get(id string) (*models.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	return job, ok
}

func (s *JobStorage) List() []*models.Job {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*models.Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

func (s *JobStorage) Set(job *models.Job) {
	s.mu.Lock()
	s.jobs[job.ID] = job
	if job.IdempotencyKey != "" {
		s.idempotency[job.IdempotencyKey] = job.ID
	}
	s.mu.Unlock()

	if err := s.save(); err != nil {
		logger.API.Load().Error().Err(err).Str("job_id", job.ID).Msg("failed to save job")
	}
}

func (s *JobStorage) Delete(id string) {
	s.mu.Lock()
	if job, ok := s.jobs[id]; ok && job.IdempotencyKey != "" {
		if mapped, ok := s.idempotency[job.IdempotencyKey]; ok && mapped == id {
			delete(s.idempotency, job.IdempotencyKey)
		}
	}
	delete(s.jobs, id)
	s.mu.Unlock()

	if err := s.save(); err != nil {
		logger.API.Load().Error().Err(err).Str("job_id", id).Msg("failed to delete job")
	}
}

// GetByIdempotencyKey returns the job previously created with the given
// Idempotency-Key header value, if it still exists.
func (s *JobStorage) GetByIdempotencyKey(key string) (*models.Job, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.idempotency[key]
	if !ok {
		return nil, false
	}
	job, ok := s.jobs[id]
	return job, ok
}

func (s *JobStorage) Cleanup(maxAge time.Duration) int {
	s.mu.Lock()

	cutoff := time.Now().Add(-maxAge)
	deleted := 0

	for id, job := range s.jobs {
		if job.Status == models.JobStatusCompleted || job.Status == models.JobStatusFailed {
			if job.UpdatedAt.Before(cutoff) {
				delete(s.jobs, id)
				if job.IdempotencyKey != "" {
					if mapped, ok := s.idempotency[job.IdempotencyKey]; ok && mapped == id {
						delete(s.idempotency, job.IdempotencyKey)
					}
				}
				deleted++
			}
		}
	}

	s.mu.Unlock()

	if deleted > 0 {
		if err := s.save(); err != nil {
			logger.API.Load().Error().Err(err).Msg("failed to save after cleanup")
		}
		logger.API.Load().Info().Int("deleted", deleted).Msg("cleaned up old jobs")
	}

	return deleted
}
