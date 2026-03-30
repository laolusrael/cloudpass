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
}

func NewJobStorage(dataDir string) (*JobStorage, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	filePath := filepath.Join(dataDir, "jobs.json")
	storage := &JobStorage{
		filePath: filePath,
		jobs:     make(map[string]*models.Job),
	}

	if err := storage.load(); err != nil {
		logger.API.Warn().Err(err).Msg("failed to load jobs file, starting fresh")
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
	}

	logger.API.Info().Int("count", len(s.jobs)).Msg("loaded jobs from storage")
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
	s.mu.Unlock()

	if err := s.save(); err != nil {
		logger.API.Error().Err(err).Str("job_id", job.ID).Msg("failed to save job")
	}
}

func (s *JobStorage) Delete(id string) {
	s.mu.Lock()
	delete(s.jobs, id)
	s.mu.Unlock()

	if err := s.save(); err != nil {
		logger.API.Error().Err(err).Str("job_id", id).Msg("failed to delete job")
	}
}

func (s *JobStorage) Cleanup(maxAge time.Duration) int {
	s.mu.Lock()

	cutoff := time.Now().Add(-maxAge)
	deleted := 0

	for id, job := range s.jobs {
		if job.Status == models.JobStatusCompleted || job.Status == models.JobStatusFailed {
			if job.UpdatedAt.Before(cutoff) {
				delete(s.jobs, id)
				deleted++
			}
		}
	}

	s.mu.Unlock()

	if deleted > 0 {
		if err := s.save(); err != nil {
			logger.API.Error().Err(err).Msg("failed to save after cleanup")
		}
		logger.API.Info().Int("deleted", deleted).Msg("cleaned up old jobs")
	}

	return deleted
}
