package handlers

import (
	"cloudpass/internal/logger"
	"cloudpass/internal/models"
	"cloudpass/internal/multipass"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

type JobHandler struct {
	storage  *JobStorage
	eventHub *EventHub
	mpClient multipass.Client
	timeout  time.Duration
}

func generateJobID() string {
	bytes := make([]byte, 8)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func NewJobHandler(mpClient multipass.Client, timeoutSec int, storage *JobStorage, eventHub *EventHub) *JobHandler {
	return &JobHandler{
		storage:  storage,
		eventHub: eventHub,
		mpClient: mpClient,
		timeout:  time.Duration(timeoutSec) * time.Second,
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

	job, ok := h.storage.Get(id)

	if !ok {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "not_found",
			Message: "job not found",
		})
	}

	return c.JSON(http.StatusOK, models.JobResponse{Job: job})
}

func (h *JobHandler) List(c echo.Context) error {
	jobs := h.storage.List()

	return c.JSON(http.StatusOK, models.JobListResponse{Jobs: jobs})
}

func (h *JobHandler) Stream(c echo.Context) error {
	c.Response().Header().Set("Content-Type", "text/event-stream")
	c.Response().Header().Set("Cache-Control", "no-cache")
	c.Response().Header().Set("Connection", "keep-alive")
	c.Response().Header().Set("Access-Control-Allow-Origin", "*")
	c.Response().Header().Set("X-Accel-Buffering", "no")

	flusher, ok := c.Response().Writer.(interface{ Flush() })
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, "streaming not supported")
	}
	flusher.Flush()

	ch := h.eventHub.Subscribe()
	defer h.eventHub.Unsubscribe(ch)

	jobs := h.storage.List()
	for _, job := range jobs {
		if job.Status == models.JobStatusPending || job.Status == models.JobStatusRunning {
			event := JobEvent{
				Type: "job.updated",
				Job:  job,
			}
			data, err := json.Marshal(event)
			if err == nil {
				if _, err := c.Response().Write([]byte("data: " + string(data) + "\n\n")); err != nil {
					return nil
				}
				flusher.Flush()
			}
		}
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case event := <-ch:
			data, err := json.Marshal(event)
			if err != nil {
				logger.API.Error().Err(err).Msg("failed to marshal event")
				continue
			}
			if _, err := c.Response().Write([]byte("data: " + string(data) + "\n\n")); err != nil {
				return nil
			}
			flusher.Flush()
		case <-ticker.C:
			if _, err := c.Response().Write([]byte(": heartbeat\n\n")); err != nil {
				return nil
			}
			flusher.Flush()
		case <-c.Request().Context().Done():
			return nil
		}
	}
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

	h.storage.Set(job)

	if h.eventHub != nil {
		h.eventHub.BroadcastJobUpdate(job)
	}

	go h.runInstanceCreation(jobID, req, instanceName)

	logger.API.Info().Str("job_id", jobID).Str("instance", instanceName).Msg("instance creation job started")

	return c.JSON(http.StatusAccepted, models.JobResponse{Job: job})
}

func (h *JobHandler) runInstanceCreation(jobID string, req models.CreateInstanceRequest, instanceName string) {
	job, ok := h.storage.Get(jobID)
	if !ok {
		logger.API.Error().Str("job_id", jobID).Msg("job not found")
		return
	}

	job.Status = models.JobStatusRunning
	job.UpdatedAt = time.Now()
	h.storage.Set(job)
	if h.eventHub != nil {
		h.eventHub.BroadcastJobUpdate(job)
	}
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

	if err := h.mpClient.LaunchInstanceBackground(opts); err != nil {
		job, _ := h.storage.Get(jobID)
		if job != nil {
			job.Status = models.JobStatusFailed
			job.Error = err.Error()
			job.UpdatedAt = time.Now()
			h.storage.Set(job)
			if h.eventHub != nil {
				h.eventHub.BroadcastJobUpdate(job)
			}
		}
		logger.API.Error().Err(err).Str("job_id", jobID).Str("instance", instanceName).Msg("failed to start instance creation")
		return
	}

	instance, err := h.mpClient.WaitForInstance(instanceName, h.timeout)

	job, _ = h.storage.Get(jobID)

	if err != nil {
		job.Status = models.JobStatusFailed
		job.Error = err.Error()
		job.UpdatedAt = time.Now()
		h.storage.Set(job)
		if h.eventHub != nil {
			h.eventHub.BroadcastJobUpdate(job)
		}
		logger.API.Error().Err(err).Str("job_id", jobID).Str("instance", instanceName).Msg("instance creation failed")
		return
	}

	job.Status = models.JobStatusCompleted
	job.InstanceName = instance.Name
	job.UpdatedAt = time.Now()
	h.storage.Set(job)
	if h.eventHub != nil {
		h.eventHub.BroadcastJobUpdate(job)
	}
	logger.API.Info().Str("job_id", jobID).Str("instance", instance.Name).Msg("instance creation completed")
}
