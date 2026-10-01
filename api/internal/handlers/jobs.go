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
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
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
		h.runExport(jobID, name, req.OutputPath, req.Overwrite, req.Compress == nil || *req.Compress)
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

func (h *JobHandler) runExport(jobID string, instanceName string, outputPath string, overwrite bool, compress bool) {
	if h.markRunning(jobID) == nil {
		return
	}
	if outputPath == "" {
		staging := h.cfgManager.EffectiveStagingDir()
		if err := os.MkdirAll(staging, 0700); err != nil {
			h.failJob(jobID, fmt.Errorf("failed to prepare staging directory: %w", err), "export staging failed")
			return
		}
		outputPath = filepath.Join(staging, fmt.Sprintf("cloudpass-export-%s-%d.img", instanceName, time.Now().Unix()))
	}
	if !overwrite {
		if _, err := os.Stat(outputPath); err == nil {
			h.failJob(jobID, fmt.Errorf("output file %q already exists (set overwrite to replace it)", outputPath), "export refused: output exists")
			return
		}
	}
	imagePath, err := h.mpClient.ExportInstance(instanceName, outputPath)
	if err != nil {
		h.failJob(jobID, err, "export job failed")
		SweepArtifacts(h.cfgManager.EffectiveStagingDir())
		return
	}
	compressed := "none"
	if compress {
		zstPath, cerr := compressImage(imagePath)
		if cerr != nil {
			h.failJob(jobID, fmt.Errorf("exported but compression failed: %w", cerr), "export compression failed")
			SweepArtifacts(h.cfgManager.EffectiveStagingDir())
			return
		}
		_ = os.Remove(imagePath)
		imagePath = zstPath
		compressed = "zstd"
	}
	h.writeImageSidecar(instanceName, imagePath, compressed)
	h.completeJob(jobID, imagePath, "export job completed")
	SweepArtifacts(h.cfgManager.EffectiveStagingDir())
}

// compressImage streams src through zstd into src+".zst" and returns the new
// path. The caller removes the original on success.
func compressImage(srcPath string) (string, error) {
	dstPath := srcPath + ".zst"

	src, err := os.Open(srcPath)
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return "", err
	}
	enc, err := zstd.NewWriter(dst, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		_ = dst.Close()
		_ = os.Remove(dstPath)
		return "", err
	}
	if _, err := io.Copy(enc, src); err != nil {
		_ = enc.Close()
		_ = dst.Close()
		_ = os.Remove(dstPath)
		return "", err
	}
	if err := enc.Close(); err != nil {
		_ = dst.Close()
		_ = os.Remove(dstPath)
		return "", err
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(dstPath)
		return "", err
	}
	return dstPath, nil
}

// isZstdImage sniffs the zstd magic bytes so detection never trusts the extension.
func isZstdImage(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()

	var magic [4]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return false
	}
	return magic[0] == 0x28 && magic[1] == 0xB5 && magic[2] == 0x2F && magic[3] == 0xFD
}

// decompressImage streams a zstd file back to a sibling .img in the staging
// directory. Returns the decoded path.
func decompressImage(staging string, srcPath string) (string, error) {
	if err := os.MkdirAll(staging, 0700); err != nil {
		return "", fmt.Errorf("failed to prepare staging directory: %w", err)
	}
	dst, err := os.CreateTemp(staging, "cloudpass-import-*.img")
	if err != nil {
		return "", err
	}
	dstName := dst.Name()

	src, err := os.Open(srcPath)
	if err != nil {
		_ = dst.Close()
		_ = os.Remove(dstName)
		return "", err
	}
	defer src.Close()

	dec, err := zstd.NewReader(src)
	if err != nil {
		_ = dst.Close()
		_ = os.Remove(dstName)
		return "", err
	}
	defer dec.Close()

	if _, err := io.Copy(dst, dec); err != nil {
		_ = dst.Close()
		_ = os.Remove(dstName)
		return "", err
	}
	if err := dst.Close(); err != nil {
		_ = os.Remove(dstName)
		return "", err
	}
	return dstName, nil
}

// checkImageSidecar warns on driver mismatches between an import image and
// this host. Informational only: same-driver imports just work, cross-driver
// ones may need qemu-img conversion (see docs).
func (h *JobHandler) checkImageSidecar(imagePath string) {
	data, err := os.ReadFile(imagePath + ".json")
	if err != nil {
		return
	}
	var sidecar models.ImageSidecar
	if err := json.Unmarshal(data, &sidecar); err != nil {
		return
	}
	if sidecar.Driver == "" {
		return
	}
	if env := h.cfgManager.GetEnvironment(); env.Driver != "" && env.Driver != sidecar.Driver {
		logger.API.Load().Warn().
			Str("image_driver", sidecar.Driver).
			Str("host_driver", env.Driver).
			Str("image", imagePath).
			Msg("importing image exported for a different driver; cross-driver imports may need conversion")
	}
}

// writeImageSidecar records portability metadata next to an exported image.
// Best-effort: a sidecar failure is logged but never fails the export.
func (h *JobHandler) writeImageSidecar(instanceName string, imagePath string, compressed string) {
	env := h.cfgManager.GetEnvironment()
	sidecar := models.ImageSidecar{
		Instance:   instanceName,
		Driver:     env.Driver,
		Arch:       runtime.GOOS + "/" + runtime.GOARCH,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
		Compressed: compressed,
	}
	if inst, err := h.mpClient.GetInstance(instanceName); err == nil && inst != nil {
		sidecar.CPUs = inst.CPU
		sidecar.Memory = inst.Memory
		sidecar.Disk = inst.Disk
	}
	data, err := json.MarshalIndent(sidecar, "", "  ")
	if err != nil {
		logger.API.Load().Warn().Err(err).Str("image", imagePath).Msg("failed to encode image sidecar")
		return
	}
	if err := os.WriteFile(imagePath+".json", data, 0600); err != nil {
		logger.API.Load().Warn().Err(err).Str("image", imagePath).Msg("failed to write image sidecar")
	}
}

// ImportAsync validates synchronously and imports the image in the background.
// It accepts JSON (server-local image_path) or multipart form data (browser
// file upload staged server-side, capped by max_image_size_mb).
func (h *JobHandler) ImportAsync(c echo.Context) error {
	var req models.ImportInstanceRequest
	stagedUpload := ""
	var err error

	if strings.HasPrefix(c.Request().Header.Get("Content-Type"), "multipart/form-data") {
		req, stagedUpload, err = h.stageImportUpload(c)
		if err != nil {
			return err
		}
	} else {
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
	}

	if req.Name != "" {
		if inst, gerr := h.mpClient.GetInstance(req.Name); gerr == nil && inst != nil {
			if stagedUpload != "" {
				_ = os.Remove(stagedUpload)
			}
			return c.JSON(http.StatusConflict, models.ErrorResponse{
				Error:   "conflict",
				Message: fmt.Sprintf("instance %q already exists (state: %s); choose a different name or delete it first", req.Name, inst.State),
			})
		}
	}

	job, replayed, err := h.startJob(c, "import", req.Name, func(jobID string) {
		h.runImport(jobID, req, stagedUpload)
	})
	if err != nil {
		if stagedUpload != "" {
			_ = os.Remove(stagedUpload)
		}
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid idempotency key")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}
	return h.replyWithJob(c, job, replayed)
}

// stageImportUpload streams a browser-uploaded image into the staging
// directory. Returns the import request (image path pre-filled) and the
// staged path the worker must remove after launching.
func (h *JobHandler) stageImportUpload(c echo.Context) (models.ImportInstanceRequest, string, error) {
	var req models.ImportInstanceRequest

	fail := func(code int, errCode string, msg string) (models.ImportInstanceRequest, string, error) {
		return models.ImportInstanceRequest{}, "", c.JSON(code, models.ErrorResponse{Error: errCode, Message: msg})
	}

	maxSize := int64(h.cfgManager.GetUploadConfig().MaxImageSizeMB) * 1024 * 1024
	if maxSize <= 0 {
		maxSize = 10240 * 1024 * 1024
	}
	c.Request().Body = http.MaxBytesReader(c.Response().Writer, c.Request().Body, maxSize+multipartOverheadBytes)

	file, err := c.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			return fail(http.StatusBadRequest, "file_too_large", fmt.Sprintf("image size exceeds maximum of %d MB", h.cfgManager.GetUploadConfig().MaxImageSizeMB))
		}
		return fail(http.StatusBadRequest, "invalid_request", "image file is required")
	}
	if file.Size > maxSize {
		return fail(http.StatusBadRequest, "file_too_large", fmt.Sprintf("image size exceeds maximum of %d MB", h.cfgManager.GetUploadConfig().MaxImageSizeMB))
	}

	filename, err := sanitizeUploadFilename(file.Filename)
	if err != nil {
		return fail(http.StatusBadRequest, "invalid_request", err.Error())
	}

	staging := h.cfgManager.EffectiveStagingDir()
	if err := os.MkdirAll(staging, 0700); err != nil {
		logger.API.Load().Error().Err(err).Str("dir", staging).Msg("failed to prepare import staging")
		return fail(http.StatusInternalServerError, "upload_error", "failed to prepare upload staging")
	}
	tmpFile, err := os.CreateTemp(staging, "cloudpass-import-*")
	if err != nil {
		return fail(http.StatusInternalServerError, "upload_error", "failed to process uploaded image")
	}
	tmpName := tmpFile.Name()

	src, err := file.Open()
	if err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
		return fail(http.StatusInternalServerError, "upload_error", "failed to read uploaded image")
	}
	defer src.Close()

	written, err := io.Copy(tmpFile, io.LimitReader(src, maxSize+1))
	if closeErr := tmpFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(tmpName)
		return fail(http.StatusInternalServerError, "upload_error", "failed to process uploaded image")
	}
	if written == 0 {
		_ = os.Remove(tmpName)
		return fail(http.StatusBadRequest, "invalid_request", "image file is empty")
	}
	if written > maxSize {
		_ = os.Remove(tmpName)
		return fail(http.StatusBadRequest, "file_too_large", fmt.Sprintf("image size exceeds maximum of %d MB", h.cfgManager.GetUploadConfig().MaxImageSizeMB))
	}

	req.ImagePath = tmpName
	if name := strings.TrimSpace(c.FormValue("name")); name != "" {
		if verr := validateInstanceName(name); verr != nil {
			_ = os.Remove(tmpName)
			return fail(http.StatusBadRequest, "invalid_request", verr.Error())
		}
		req.Name = name
	}
	if cpus := strings.TrimSpace(c.FormValue("cpus")); cpus != "" {
		if n, nerr := strconv.Atoi(cpus); nerr != nil || n <= 0 {
			_ = os.Remove(tmpName)
			return fail(http.StatusBadRequest, "invalid_request", "cpus must be a positive integer")
		} else {
			req.CPUs = n
		}
	}
	req.Memory = strings.TrimSpace(c.FormValue("memory"))
	req.Disk = strings.TrimSpace(c.FormValue("disk"))
	if req.Name == "" {
		// Default the instance name from the uploaded file; multipass
		// generates a random one when this stays empty.
		req.Name = deriveInstanceName(filename)
	}

	return req, tmpName, nil
}

// deriveInstanceName turns an uploaded file name into a valid instance name,
// or "" when nothing usable remains (multipass then picks a random name).
func deriveInstanceName(filename string) string {
	base := strings.TrimSuffix(filename, filepath.Ext(filename))
	base = strings.ToLower(base)
	var b strings.Builder
	for _, r := range base {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	name := strings.Trim(b.String(), "-")
	if err := validateInstanceName(name); err != nil {
		return ""
	}
	return name
}

func (h *JobHandler) runImport(jobID string, req models.ImportInstanceRequest, stagedUpload string) {
	if h.markRunning(jobID) == nil {
		return
	}
	launchPath := req.ImagePath
	var decoded string
	if isZstdImage(req.ImagePath) {
		var err error
		decoded, err = decompressImage(h.cfgManager.EffectiveStagingDir(), req.ImagePath)
		if err != nil {
			h.failJob(jobID, fmt.Errorf("failed to decompress image: %w", err), "import decompression failed")
			return
		}
		defer os.Remove(decoded)
		launchPath = decoded
	} else {
		h.checkImageSidecar(req.ImagePath)
	}
	if stagedUpload != "" {
		defer os.Remove(stagedUpload)
	}
	instance, err := h.mpClient.ImportInstance(launchPath, req.Name, req.CPUs, req.Memory, req.Disk)
	if err != nil {
		h.failJob(jobID, err, "import job failed")
		SweepArtifacts(h.cfgManager.EffectiveStagingDir())
		return
	}
	job, _ := h.storage.Get(jobID)
	if job != nil {
		job.InstanceName = instance.Name
		h.storage.Set(job)
	}
	h.completeJob(jobID, instance.Name, "import job completed")
	SweepArtifacts(h.cfgManager.EffectiveStagingDir())
}

// DownloadExport streams a completed export job's image file (or its JSON
// sidecar with ?sidecar=true) as a browser download. Only files inside the
// staging directory are servable; explicit-path exports must be fetched from
// the server itself. Served files are deleted afterwards (retention policy).
func (h *JobHandler) DownloadExport(c echo.Context) error {
	name := c.Param("name")
	if err := validateAsyncInstanceName(c, name); err != nil {
		return err
	}

	job, ok := h.storage.Get(c.QueryParam("job_id"))
	if !ok {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "not_found",
			Message: "export job not found",
		})
	}
	if job.Type != "export" || job.InstanceName != name {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "not_found",
			Message: "export job not found",
		})
	}
	if job.Status != models.JobStatusCompleted {
		return c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "conflict",
			Message: fmt.Sprintf("export is %s, not completed", job.Status),
		})
	}

	artifact := job.Result
	filename := filepath.Base(artifact)
	if c.QueryParam("sidecar") == "true" {
		artifact += ".json"
		filename += ".json"
		c.Response().Header().Set("Content-Type", "application/json")
	}
	if !isStagedArtifact(h.cfgManager.EffectiveStagingDir(), artifact) {
		return c.JSON(http.StatusConflict, models.ErrorResponse{
			Error:   "conflict",
			Message: "this export was written outside the staging directory; fetch it from the server path",
		})
	}

	f, err := os.Open(artifact)
	if err != nil {
		logger.API.Load().Warn().Err(err).Str("job_id", job.ID).Msg("export artifact gone")
		return c.JSON(http.StatusGone, models.ErrorResponse{
			Error:   "gone",
			Message: "export file has expired; export the instance again",
		})
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return c.JSON(http.StatusGone, models.ErrorResponse{
			Error:   "gone",
			Message: "export file has expired; export the instance again",
		})
	}

	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, filename))
	http.ServeContent(c.Response(), c.Request(), filename, info.ModTime(), f)

	// Close before removing: Windows refuses to delete open files.
	_ = f.Close()

	// Delete-after-download retention: remove the image and its sidecar.
	_ = os.Remove(strings.TrimSuffix(artifact, ".json"))
	_ = os.Remove(strings.TrimSuffix(artifact, ".json") + ".json")
	SweepArtifacts(h.cfgManager.EffectiveStagingDir())
	logger.API.Load().Info().Str("job_id", job.ID).Str("file", filename).Msg("export artifact downloaded and removed")
	return nil
}

// isStagedArtifact reports whether path is confined inside dir (host
// semantics). Download and cleanup paths must never escape staging.
func isStagedArtifact(dir string, path string) bool {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	return absPath == absDir || strings.HasPrefix(absPath, absDir+string(filepath.Separator))
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
