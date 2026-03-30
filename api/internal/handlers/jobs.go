package handlers

import (
	"cloudpass/internal/logger"
	"cloudpass/internal/models"
	"cloudpass/internal/multipass"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

type JobHandler struct {
	jobs     map[string]*models.Job
	mu       sync.RWMutex
	mpClient multipass.Client
}

func generateJobID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func NewJobHandler(mpClient multipass.Client) *JobHandler {
	return &JobHandler{
		jobs:     make(map[string]*models.Job),
		mpClient: mpClient,
	}
}

func (h *JobHandler) Get(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "job id is required",
		})
	}

	h.mu.RLock()
	job, ok := h.jobs[id]
	h.mu.RUnlock()

	if !ok {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "not_found",
			Message: "job not found",
		})
	}

	return c.JSON(http.StatusOK, models.JobResponse{Job: job})
}

func (h *JobHandler) CreateInstanceAsync(c echo.Context) error {
	var req models.CreateInstanceRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.Name != "" {
		if err := validateInstanceName(req.Name); err != nil {
			logger.API.Warn().Err(err).Str("ip", c.RealIP()).Str("name", req.Name).Msg("invalid instance name")
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "invalid_request",
				Message: err.Error(),
			})
		}
	}

	jobID := generateJobID()
	instanceName := req.Name
	if instanceName == "" {
		instanceName = "inst-" + strings.ToLower(generateJobID()[:8])
	}

	job := &models.Job{
		ID:           jobID,
		Type:         "create_instance",
		Status:       models.JobStatusPending,
		InstanceName: instanceName,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	h.mu.Lock()
	h.jobs[jobID] = job
	h.mu.Unlock()

	go h.runInstanceCreation(jobID, req, instanceName)

	logger.API.Info().Str("job_id", jobID).Str("instance", instanceName).Msg("instance creation job started")

	return c.JSON(http.StatusAccepted, models.JobResponse{Job: job})
}

func (h *JobHandler) runInstanceCreation(jobID string, req models.CreateInstanceRequest, instanceName string) {
	h.mu.Lock()
	job := h.jobs[jobID]
	h.mu.Unlock()

	if job == nil {
		logger.API.Error().Str("job_id", jobID).Msg("job not found")
		return
	}

	job.Status = models.JobStatusRunning
	job.UpdatedAt = time.Now()
	logger.API.Info().Str("job_id", jobID).Msg("job status: running")

	opts := models.CreateInstanceRequest{
		Name:      instanceName,
		CPUs:      req.CPUs,
		Memory:    req.Memory,
		Disk:      req.Disk,
		Network:   req.Network,
		CloudInit: req.CloudInit,
		Image:     req.Image,
	}

	instance, err := h.mpClient.CreateInstance(opts)

	h.mu.Lock()
	job = h.jobs[jobID]
	h.mu.Unlock()

	if err != nil {
		job.Status = models.JobStatusFailed
		job.Error = err.Error()
		job.UpdatedAt = time.Now()
		logger.API.Error().Err(err).Str("job_id", jobID).Str("instance", instanceName).Msg("instance creation failed")
		return
	}

	job.Status = models.JobStatusCompleted
	job.InstanceName = instance.Name
	job.UpdatedAt = time.Now()
	logger.API.Info().Str("job_id", jobID).Str("instance", instance.Name).Msg("instance creation completed")
}
