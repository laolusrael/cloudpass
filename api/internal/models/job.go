package models

import "time"

type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"
	JobStatusRunning   JobStatus = "running"
	JobStatusCompleted JobStatus = "completed"
	JobStatusFailed    JobStatus = "failed"
)

type Job struct {
	ID             string    `json:"id"`
	Type           string    `json:"type"`
	Status         JobStatus `json:"status"`
	InstanceName   string    `json:"instance_name,omitempty"`
	Result         string    `json:"result,omitempty"`
	Error          string    `json:"error,omitempty"`
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CreateJobRequest struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

type JobResponse struct {
	Job *Job `json:"job"`
}

type JobListResponse struct {
	Jobs []*Job `json:"jobs"`
}
