package handlers

import (
	"cloudpass/internal/config"
	"cloudpass/internal/logger"
	"cloudpass/internal/models"
	"cloudpass/internal/multipass"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

// IdempotencyKeyHeader carries a client-generated key that makes async
// creation safe to retry: replays with the same key return the original job
// instead of launching a duplicate instance.
const IdempotencyKeyHeader = "Idempotency-Key"

const (
	maxIdempotencyKeyLen = 128
)

var idempotencyKeyRegex = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

var (
	errIdempotencyKeyLength  = errors.New("idempotency key must be 1-128 characters")
	errIdempotencyKeyCharset = errors.New("idempotency key must match [A-Za-z0-9_-]+")
)

func validateIdempotencyKey(key string) error {
	if len(key) == 0 || len(key) > maxIdempotencyKeyLen {
		return errIdempotencyKeyLength
	}
	if !idempotencyKeyRegex.MatchString(key) {
		return errIdempotencyKeyCharset
	}
	return nil
}

type JobHandler struct {
	storage    *JobStorage
	eventHub   *EventHub
	mpClient   multipass.Client
	cfgManager *config.ConfigManager
	timeout    time.Duration
	// idempotencyMu serializes same-key creates so concurrent replays
	// collapse onto a single job instead of racing check-then-create.
	idempotencyMu sync.Mutex
}

func generateJobID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}

func NewJobHandler(mpClient multipass.Client, timeoutSec int, storage *JobStorage, eventHub *EventHub, cfgManager *config.ConfigManager) *JobHandler {
	return &JobHandler{
		storage:    storage,
		eventHub:   eventHub,
		mpClient:   mpClient,
		cfgManager: cfgManager,
		timeout:    time.Duration(timeoutSec) * time.Second,
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
				select {
				case <-c.Request().Context().Done():
					return nil
				default:
					if _, err := c.Response().Write([]byte("data: " + string(data) + "\n\n")); err != nil {
						logger.API.Load().Debug().Err(err).Msg("SSE initial write error")
						return nil
					}
					flusher.Flush()
				}
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
				logger.API.Load().Error().Err(err).Msg("failed to marshal event")
				continue
			}
			if _, err := c.Response().Write([]byte("data: " + string(data) + "\n\n")); err != nil {
				logger.API.Load().Debug().Err(err).Msg("SSE write error, closing connection")
				return nil
			}
			flusher.Flush()
		case <-ticker.C:
			select {
			case <-c.Request().Context().Done():
				return nil
			default:
				if _, err := c.Response().Write([]byte(": heartbeat\n\n")); err != nil {
					logger.API.Load().Debug().Err(err).Msg("SSE heartbeat write error, closing connection")
					return nil
				}
				flusher.Flush()
			}
		case <-c.Request().Context().Done():
			logger.API.Load().Debug().Msg("SSE client disconnected")
			return nil
		}
	}
}

func (h *JobHandler) CreateInstanceAsync(c echo.Context) error {
	var req models.CreateInstanceRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.Name != "" {
		if err := validateInstanceName(req.Name); err != nil {
			logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", req.Name).Msg("invalid instance name")
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "invalid_request",
				Message: err.Error(),
			})
		}
	}

	idempotencyKey := strings.TrimSpace(c.Request().Header.Get(IdempotencyKeyHeader))
	if idempotencyKey != "" {
		if err := validateIdempotencyKey(idempotencyKey); err != nil {
			logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid idempotency key")
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "invalid_request",
				Message: err.Error(),
			})
		}

		h.idempotencyMu.Lock()
		defer h.idempotencyMu.Unlock()

		if existing, ok := h.storage.GetByIdempotencyKey(idempotencyKey); ok {
			logger.API.Load().Info().Str("job_id", existing.ID).Str("key", idempotencyKey).Msg("idempotent replay: returning existing job")
			return c.JSON(http.StatusOK, models.JobResponse{Job: existing})
		}
	}

	jobID := generateJobID()
	instanceName := req.Name
	if instanceName == "" {
		instanceName = "inst-" + strings.ToLower(generateJobID()[:8])
	}

	job := &models.Job{
		ID:             jobID,
		Type:           "create_instance",
		Status:         models.JobStatusPending,
		InstanceName:   instanceName,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	h.storage.Set(job)

	if h.eventHub != nil {
		h.eventHub.BroadcastJobUpdate(job)
	}

	go h.runInstanceCreation(jobID, req, instanceName)

	logger.API.Load().Info().Str("job_id", jobID).Str("instance", instanceName).Msg("instance creation job started")

	return c.JSON(http.StatusAccepted, models.JobResponse{Job: job})
}

// startJob validates the Idempotency-Key header, replays a same-key job
// when one exists, and otherwise enqueues a pending job running run in the
// background. It returns the job and whether it was a replay (replays answer
// 200, fresh jobs 202).
func (h *JobHandler) startJob(c echo.Context, jobType string, instanceName string, run func(jobID string)) (*models.Job, bool, error) {
	idempotencyKey := strings.TrimSpace(c.Request().Header.Get(IdempotencyKeyHeader))
	if idempotencyKey != "" {
		if err := validateIdempotencyKey(idempotencyKey); err != nil {
			return nil, false, err
		}

		h.idempotencyMu.Lock()
		defer h.idempotencyMu.Unlock()

		if existing, ok := h.storage.GetByIdempotencyKey(idempotencyKey); ok {
			logger.API.Load().Info().Str("job_id", existing.ID).Str("key", idempotencyKey).Msg("idempotent replay: returning existing job")
			return existing, true, nil
		}
	}

	jobID := generateJobID()
	now := time.Now()
	job := &models.Job{
		ID:             jobID,
		Type:           jobType,
		Status:         models.JobStatusPending,
		InstanceName:   instanceName,
		IdempotencyKey: idempotencyKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	h.storage.Set(job)

	if h.eventHub != nil {
		h.eventHub.BroadcastJobUpdate(job)
	}

	go run(jobID)

	logger.API.Load().Info().Str("job_id", jobID).Str("type", jobType).Str("instance", instanceName).Msg("job started")

	return job, false, nil
}

func (h *JobHandler) replyWithJob(c echo.Context, job *models.Job, replayed bool) error {
	if replayed {
		return c.JSON(http.StatusOK, models.JobResponse{Job: job})
	}
	return c.JSON(http.StatusAccepted, models.JobResponse{Job: job})
}

func (h *JobHandler) markRunning(jobID string) *models.Job {
	job, ok := h.storage.Get(jobID)
	if !ok {
		logger.API.Load().Error().Str("job_id", jobID).Msg("job not found")
		return nil
	}
	job.Status = models.JobStatusRunning
	job.UpdatedAt = time.Now()
	h.storage.Set(job)
	if h.eventHub != nil {
		h.eventHub.BroadcastJobUpdate(job)
	}
	return job
}

func (h *JobHandler) failJob(jobID string, err error, msg string) {
	job, _ := h.storage.Get(jobID)
	if job == nil {
		logger.API.Load().Error().Str("job_id", jobID).Msg("job not found")
		return
	}
	job.Status = models.JobStatusFailed
	job.Error = err.Error()
	job.UpdatedAt = time.Now()
	h.storage.Set(job)
	if h.eventHub != nil {
		h.eventHub.BroadcastJobUpdate(job)
	}
	logger.API.Load().Error().Err(err).Str("job_id", jobID).Msg(msg)
}

func (h *JobHandler) completeJob(jobID string, result string, msg string) {
	job, _ := h.storage.Get(jobID)
	if job == nil {
		logger.API.Load().Error().Str("job_id", jobID).Msg("job not found")
		return
	}
	job.Status = models.JobStatusCompleted
	job.Result = result
	job.UpdatedAt = time.Now()
	h.storage.Set(job)
	if h.eventHub != nil {
		h.eventHub.BroadcastJobUpdate(job)
	}
	logger.API.Load().Info().Str("job_id", jobID).Str("result", result).Msg(msg)
}

func validateAsyncInstanceName(c echo.Context, name string) error {
	if name == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}
	if err := validateInstanceName(name); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}
	return nil
}

// CreateMountAsync validates a mount request synchronously and runs the
// (potentially minutes-long) multipass mount in the background.
func (h *JobHandler) CreateMountAsync(c echo.Context) error {
	name := c.Param("name")
	if err := validateAsyncInstanceName(c, name); err != nil {
		return err
	}

	var req models.MountRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
		})
	}
	if req.SourcePath == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "source_path is required",
		})
	}
	if req.TargetPath == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "target_path is required",
		})
	}

	opts, err := resolveMountOptions(req, h.cfgManager.GetEnvironment())
	if err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	job, replayed, err := h.startJob(c, "mount", name, func(jobID string) {
		h.runMount(jobID, name, req.SourcePath, req.TargetPath, opts)
	})
	if err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid idempotency key")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}
	return h.replyWithJob(c, job, replayed)
}

func (h *JobHandler) runMount(jobID string, instanceName string, source string, target string, opts multipass.MountOptions) {
	if h.markRunning(jobID) == nil {
		return
	}
	if err := h.mpClient.MountInstance(instanceName, source, target, opts); err != nil {
		h.failJob(jobID, err, "mount job failed")
		return
	}
	h.completeJob(jobID, source+" -> "+target, "mount job completed")
}

// CreateSnapshotAsync validates synchronously and snapshots in the background
// (snapshotting stops the instance and copies disk state: minutes on large disks).
func (h *JobHandler) CreateSnapshotAsync(c echo.Context) error {
	name := c.Param("name")
	if err := validateAsyncInstanceName(c, name); err != nil {
		return err
	}

	var req models.CreateSnapshotRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		req = models.CreateSnapshotRequest{}
	}

	job, replayed, err := h.startJob(c, "snapshot_create", name, func(jobID string) {
		h.runSnapshotCreate(jobID, name, req.Name, req.Comment)
	})
	if err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid idempotency key")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}
	return h.replyWithJob(c, job, replayed)
}

func (h *JobHandler) runSnapshotCreate(jobID string, instanceName string, snapshotName string, comment string) {
	if h.markRunning(jobID) == nil {
		return
	}
	if err := h.mpClient.CreateSnapshot(instanceName, snapshotName, comment); err != nil {
		h.failJob(jobID, err, "snapshot creation job failed")
		return
	}
	h.completeJob(jobID, snapshotName, "snapshot creation job completed")
}

// RestoreSnapshotAsync validates synchronously and restores in the background.
func (h *JobHandler) RestoreSnapshotAsync(c echo.Context) error {
	name := c.Param("name")
	if err := validateAsyncInstanceName(c, name); err != nil {
		return err
	}

	snapshotName := c.Param("id")
	if snapshotName == "" {
		var req models.RestoreSnapshotRequest
		if err := c.Bind(&req); err == nil && req.Name != "" {
			snapshotName = req.Name
		}
	}
	if snapshotName == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "snapshot name is required",
		})
	}

	job, replayed, err := h.startJob(c, "snapshot_restore", name, func(jobID string) {
		h.runSnapshotRestore(jobID, name, snapshotName)
	})
	if err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid idempotency key")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}
	return h.replyWithJob(c, job, replayed)
}

func (h *JobHandler) runSnapshotRestore(jobID string, instanceName string, snapshotName string) {
	if h.markRunning(jobID) == nil {
		return
	}
	if err := h.mpClient.RestoreSnapshot(instanceName, snapshotName); err != nil {
		h.failJob(jobID, err, "snapshot restore job failed")
		return
	}
	h.completeJob(jobID, snapshotName, "snapshot restore job completed")
}

// ExportAsync validates synchronously and copies the disk image in the background.
func (h *JobHandler) ExportAsync(c echo.Context) error {
	name := c.Param("name")
	if err := validateAsyncInstanceName(c, name); err != nil {
		return err
	}

	var req models.ExportInstanceRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		req = models.ExportInstanceRequest{}
	}

	job, replayed, err := h.startJob(c, "export", name, func(jobID string) {
		h.runExport(jobID, name, req.OutputPath)
	})
	if err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid idempotency key")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}
	return h.replyWithJob(c, job, replayed)
}

func (h *JobHandler) runExport(jobID string, instanceName string, outputPath string) {
	if h.markRunning(jobID) == nil {
		return
	}
	imagePath, err := h.mpClient.ExportInstance(instanceName, outputPath)
	if err != nil {
		h.failJob(jobID, err, "export job failed")
		return
	}
	h.completeJob(jobID, imagePath, "export job completed")
}

// ImportAsync validates synchronously and imports the image in the background.
func (h *JobHandler) ImportAsync(c echo.Context) error {
	var req models.ImportInstanceRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
		})
	}
	if req.ImagePath == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "image_path is required",
		})
	}
	if req.Name != "" {
		if err := validateInstanceName(req.Name); err != nil {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "invalid_request",
				Message: err.Error(),
			})
		}
	}

	job, replayed, err := h.startJob(c, "import", req.Name, func(jobID string) {
		h.runImport(jobID, req)
	})
	if err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid idempotency key")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}
	return h.replyWithJob(c, job, replayed)
}

func (h *JobHandler) runImport(jobID string, req models.ImportInstanceRequest) {
	if h.markRunning(jobID) == nil {
		return
	}
	instance, err := h.mpClient.ImportInstance(req.ImagePath, req.Name, req.CPUs, req.Memory, req.Disk)
	if err != nil {
		h.failJob(jobID, err, "import job failed")
		return
	}
	job, _ := h.storage.Get(jobID)
	if job != nil {
		job.InstanceName = instance.Name
		h.storage.Set(job)
	}
	h.completeJob(jobID, instance.Name, "import job completed")
}

func (h *JobHandler) runInstanceCreation(jobID string, req models.CreateInstanceRequest, instanceName string) {
	job, ok := h.storage.Get(jobID)
	if !ok {
		logger.API.Load().Error().Str("job_id", jobID).Msg("job not found")
		return
	}

	job.Status = models.JobStatusRunning
	job.UpdatedAt = time.Now()
	h.storage.Set(job)
	if h.eventHub != nil {
		h.eventHub.BroadcastJobUpdate(job)
	}
	logger.API.Load().Info().Str("job_id", jobID).Msg("job status: running")

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
		logger.API.Load().Error().Err(err).Str("job_id", jobID).Str("instance", instanceName).Msg("failed to start instance creation")
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
		logger.API.Load().Error().Err(err).Str("job_id", jobID).Str("instance", instanceName).Msg("instance creation failed")
		return
	}

	job.Status = models.JobStatusCompleted
	job.InstanceName = instance.Name
	job.UpdatedAt = time.Now()
	h.storage.Set(job)
	if h.eventHub != nil {
		h.eventHub.BroadcastJobUpdate(job)
	}
	logger.API.Load().Info().Str("job_id", jobID).Str("instance", instance.Name).Msg("instance creation completed")
}
