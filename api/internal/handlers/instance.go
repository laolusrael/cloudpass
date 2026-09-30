package handlers

import (
	"cloudpass/internal/config"
	"cloudpass/internal/logger"
	"cloudpass/internal/models"
	"cloudpass/internal/multipass"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
)

type InstanceHandler struct {
	client     multipass.Client
	cfgManager *config.ConfigManager
}

func NewInstanceHandler(client multipass.Client, cfgManager *config.ConfigManager) *InstanceHandler {
	return &InstanceHandler{client: client, cfgManager: cfgManager}
}

var instanceNameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

// multipartOverheadBytes is extra headroom above the file-size limit for
// multipart framing (boundaries, headers, form fields) when capping the
// request body. The file content itself is still capped at maxSize.
// It is a var (not const) so tests can shrink it to exercise the
// request-body cap without sending multi-megabyte bodies.
var multipartOverheadBytes = int64(10 * 1024 * 1024)

// sanitizeUploadFilename strips any directory components from a client
// supplied file name and rejects empty or hostile values.
func sanitizeUploadFilename(name string) (string, error) {
	if strings.ContainsRune(name, '\x00') {
		return "", errors.New("file name contains invalid characters")
	}
	base := path.Base(strings.ReplaceAll(name, "\\", "/"))
	if base == "" || base == "." || base == "/" || base == ".." {
		return "", errors.New("file name is required")
	}
	return base, nil
}

// validateGuestPath rejects NUL bytes and ".." segments in a guest (Linux)
// path. Callers must Clean the path first, which already neutralizes ".."
// (path.Clean resolves dot-dot segments), so the segment check below is
// belt-and-braces. It does not cover guest symlink escapes via mkdir -p.
func validateGuestPath(path string) error {
	if strings.ContainsRune(path, '\x00') {
		return errors.New("path contains invalid characters")
	}
	for _, seg := range strings.Split(path, "/") {
		if seg == ".." {
			return errors.New("path contains invalid traversal characters")
		}
	}
	return nil
}

// resolveUploadTarget maps the optional target_path form value to the guest
// destination path: empty means DefaultPath + filename, a trailing slash
// means directory (filename appended), otherwise the value is the full
// destination file path and must be absolute. Guest paths always use POSIX
// semantics, even when the server runs on Windows.
func resolveUploadTarget(target, filename, defaultDir string) (string, error) {
	if target == "" {
		return path.Join(defaultDir, filename), nil
	}
	if strings.HasSuffix(target, "/") {
		cleaned := path.Clean(target)
		if cleaned == "/" {
			return "", errors.New("target path must not be the filesystem root")
		}
		if err := validateGuestPath(cleaned); err != nil {
			return "", err
		}
		return path.Join(cleaned, filename), nil
	}
	if !strings.HasPrefix(target, "/") {
		return "", errors.New("target path must be absolute or empty")
	}
	cleaned := path.Clean(target)
	if err := validateGuestPath(cleaned); err != nil {
		return "", err
	}
	return cleaned, nil
}

func validateInstanceName(name string) error {
	if name == "" {
		return errors.New("instance name is required")
	}
	if len(name) > 63 {
		return errors.New("instance name must be 63 characters or less")
	}
	if !instanceNameRegex.MatchString(name) {
		return errors.New("instance name must consist of lowercase letters, numbers, and hyphens, must start with a letter, and must end with an alphanumeric character")
	}
	return nil
}

func (h *InstanceHandler) List(c echo.Context) error {
	instances, err := h.client.ListInstances()
	if err != nil {
		logger.API.Load().Error().Err(err).Msg("failed to list instances")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	for i := range instances {
		instance, err := h.client.GetInstance(instances[i].Name)
		if err != nil {
			logger.API.Load().Warn().
				Str("name", instances[i].Name).
				Err(err).
				Msg("failed to get instance details, using basic info")
			continue
		}
		instances[i] = *instance
	}

	logger.API.Load().Debug().Int("count", len(instances)).Msg("listed instances")
	return c.JSON(http.StatusOK, models.InstanceList{
		Instances: instances,
	})
}

func (h *InstanceHandler) Get(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	instance, err := h.client.GetInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to get instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Debug().Str("name", name).Msg("got instance")
	return c.JSON(http.StatusOK, instance)
}

func (h *InstanceHandler) GetState(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	instance, err := h.client.GetInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to get instance state")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Debug().Str("name", name).Str("state", instance.State).Msg("got instance state")
	return c.JSON(http.StatusOK, models.InstanceState{
		Name:  name,
		State: instance.State,
	})
}

func (h *InstanceHandler) Create(c echo.Context) error {
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

	instance, err := h.client.CreateInstance(req)
	if err != nil {
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", req.Name).Msg("failed to create instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", instance.Name).Msg("instance created")
	return c.JSON(http.StatusCreated, instance)
}

func (h *InstanceHandler) Delete(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.DeleteInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to delete instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Msg("instance deleted")
	return c.JSON(http.StatusOK, map[string]string{"name": name, "status": "deleted"})
}

func (h *InstanceHandler) Start(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.StartInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to start instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Msg("instance started")
	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Instance started",
	})
}

func (h *InstanceHandler) Stop(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.StopInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to stop instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Msg("instance stopped")
	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Instance stopped",
	})
}

func (h *InstanceHandler) Restart(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.RestartInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to restart instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Msg("instance restarted")
	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Instance restarted",
	})
}

func (h *InstanceHandler) Suspend(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.SuspendInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to suspend instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Msg("instance suspended")
	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Instance suspended",
	})
}

func (h *InstanceHandler) Resume(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.ResumeInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to resume instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Msg("instance resumed")
	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Instance resumed",
	})
}

func (h *InstanceHandler) Export(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	var req models.ExportInstanceRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		req = models.ExportInstanceRequest{}
	}

	imagePath, err := h.client.ExportInstance(name, req.OutputPath)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to export instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Str("path", imagePath).Msg("instance exported")
	return c.JSON(http.StatusOK, models.InstanceExport{
		Message:   "Instance exported successfully",
		ImagePath: imagePath,
	})
}

func (h *InstanceHandler) Import(c echo.Context) error {
	var req models.ImportInstanceRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.ImagePath == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("image_path is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "image_path is required",
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

	instance, err := h.client.ImportInstance(req.ImagePath, req.Name, req.CPUs, req.Memory, req.Disk)
	if err != nil {
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("image", req.ImagePath).Msg("failed to import instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", instance.Name).Msg("instance imported")
	return c.JSON(http.StatusCreated, instance)
}

func (h *InstanceHandler) CreateSnapshot(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	var req models.CreateSnapshotRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		req = models.CreateSnapshotRequest{}
	}

	err := h.client.CreateSnapshot(name, req.Name, req.Comment)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to create snapshot")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Str("snapshot", req.Name).Msg("snapshot created")
	return c.JSON(http.StatusCreated, models.SnapshotResponse{
		Message:      "Snapshot created successfully",
		SnapshotName: req.Name,
		InstanceName: name,
	})
}

func (h *InstanceHandler) ListSnapshots(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	snapshots, err := h.client.ListSnapshots(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to list snapshots")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Debug().Int("count", len(snapshots)).Str("name", name).Msg("listed snapshots")
	return c.JSON(http.StatusOK, models.SnapshotList{
		Snapshots: snapshots,
	})
}

func (h *InstanceHandler) RestoreSnapshot(c echo.Context) error {
	name := c.Param("name")
	snapshotID := c.Param("id")

	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	snapshotName := snapshotID
	if snapshotName == "" {
		var req models.RestoreSnapshotRequest
		if err := c.Bind(&req); err == nil && req.Name != "" {
			snapshotName = req.Name
		}
	}

	err := h.client.RestoreSnapshot(name, snapshotName)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance or snapshot not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Str("snapshot", snapshotName).Msg("failed to restore snapshot")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Str("snapshot", snapshotName).Msg("snapshot restored")
	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Snapshot restored",
	})
}

func (h *InstanceHandler) DeleteSnapshot(c echo.Context) error {
	name := c.Param("name")
	snapshotID := c.Param("id")

	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.DeleteSnapshot(name, snapshotID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance or snapshot not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Str("snapshot", snapshotID).Msg("failed to delete snapshot")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Str("snapshot", snapshotID).Msg("snapshot deleted")
	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Snapshot deleted",
	})
}

func (h *InstanceHandler) Mount(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
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

	mountType := req.MountType
	if mountType == "" {
		mountType = "classic"
	}
	if mountType != "classic" && mountType != "native" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "mount_type must be classic or native",
		})
	}
	if err := validateMountIDMap(c, req.UIDMap, "uid_map"); err != nil {
		return err
	}
	if err := validateMountIDMap(c, req.GIDMap, "gid_map"); err != nil {
		return err
	}

	err := h.client.MountInstance(name, req.SourcePath, req.TargetPath, multipass.MountOptions{
		Type:   mountType,
		UIDMap: req.UIDMap,
		GIDMap: req.GIDMap,
	})
	if err != nil {
		// NOTE: "source path" must be checked before "does not exist" /
		// "not found" — source errors contain those phrases but mean the
		// host path is bad, not the instance.
		if strings.Contains(err.Error(), "source path") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("source", req.SourcePath).Msg("source path error")
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "invalid_request",
				Message: err.Error(),
			})
		}
		if strings.Contains(err.Error(), "already mounted") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Str("target", req.TargetPath).Msg("target already mounted")
			return c.JSON(http.StatusConflict, models.ErrorResponse{
				Error:   "conflict",
				Message: err.Error(),
			})
		}
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not found") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		if strings.Contains(err.Error(), "is not running") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not running")
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "instance_not_running",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to mount directory")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Str("source", req.SourcePath).Str("target", req.TargetPath).Str("type", mountType).Msg("directory mounted")
	return c.JSON(http.StatusCreated, models.MountResponse{
		Message: "Directory mounted",
		Source:  req.SourcePath,
		Target:  req.TargetPath,
	})
}

// validateMountIDMap rejects malformed optional "host:instance" ID mappings
// with a 400 response, or returns nil when valid (empty means unset).
func validateMountIDMap(c echo.Context, mapping string, field string) error {
	if mapping == "" {
		return nil
	}
	parts := strings.Split(mapping, ":")
	if len(parts) != 2 {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: fmt.Sprintf("%s must be host:instance IDs", field),
		})
	}
	for _, part := range parts {
		id, err := strconv.Atoi(part)
		if err != nil || id < 0 {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "invalid_request",
				Message: fmt.Sprintf("%s must be host:instance IDs", field),
			})
		}
	}
	return nil
}

func (h *InstanceHandler) Unmount(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	var req models.UnmountRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.TargetPath == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "target_path is required",
		})
	}

	err := h.client.UnmountInstance(name, req.TargetPath)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") || strings.Contains(err.Error(), "not found") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to unmount directory")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Str("target", req.TargetPath).Msg("directory unmounted")
	return c.JSON(http.StatusOK, models.MountResponse{
		Message: "Directory unmounted",
		Target:  req.TargetPath,
	})
}

func (h *InstanceHandler) Upload(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	uploadCfg := h.cfgManager.GetUploadConfig()
	maxSize := int64(uploadCfg.MaxFileSizeMB) * 1024 * 1024
	if maxSize <= 0 {
		maxSize = 100 * 1024 * 1024
	}
	tooLargeResponse := func(c echo.Context) error {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "file_too_large",
			Message: fmt.Sprintf("file size exceeds maximum of %d MB", uploadCfg.MaxFileSizeMB),
		})
	}

	// Cap the whole request body so an oversized upload is rejected while
	// streaming instead of after buffering the full multipart body.
	c.Request().Body = http.MaxBytesReader(c.Response().Writer, c.Request().Body, maxSize+multipartOverheadBytes)

	file, err := c.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("upload exceeds maximum request size")
			return tooLargeResponse(c)
		}
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("no file in request")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "file is required",
		})
	}

	// Cheap header check first; the streamed copy below enforces the real cap
	// since the header value is client-supplied.
	if file.Size > maxSize {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Int64("size", file.Size).Int64("max", maxSize).Msg("file too large")
		return tooLargeResponse(c)
	}

	filename, err := sanitizeUploadFilename(file.Filename)
	if err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid file name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	targetPath, err := resolveUploadTarget(c.FormValue("target_path"), filename, uploadCfg.DefaultPath)
	if err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("path", c.FormValue("target_path")).Msg("invalid target path")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	src, err := file.Open()
	if err != nil {
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Msg("failed to open uploaded file")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "upload_error",
			Message: "failed to read uploaded file",
		})
	}
	defer src.Close()

	tmpFile, err := os.CreateTemp("", "cloudpass-upload-*")
	if err != nil {
		logger.API.Load().Error().Err(err).Msg("failed to create temp file")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "upload_error",
			Message: "failed to process uploaded file",
		})
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	written, err := io.Copy(tmpFile, io.LimitReader(src, maxSize+1))
	if closeErr := tmpFile.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		logger.API.Load().Error().Err(err).Msg("failed to write temp file")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "upload_error",
			Message: "failed to process uploaded file",
		})
	}
	if written == 0 {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("empty file upload rejected")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "file is empty",
		})
	}
	// Defense-in-depth: unreachable via honest clients (the header pre-check
	// above catches oversized files since Go derives file.Size from the
	// actual part bytes), but guards against size-spoofing transports.
	if written > maxSize {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Int64("size", written).Int64("max", maxSize).Msg("file too large")
		return tooLargeResponse(c)
	}

	if err := h.client.UploadFile(name, tmpName, targetPath); err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		if strings.Contains(err.Error(), "is not running") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not running")
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "instance_not_running",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to upload file")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Str("target", targetPath).Msg("file uploaded")
	return c.JSON(http.StatusCreated, models.UploadResponse{
		Message: "File uploaded",
		Path:    targetPath,
	})
}

func (h *InstanceHandler) UpdateResources(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("instance name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	var req models.UpdateResourcesRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
		})
	}

	instance, err := h.client.GetInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to get instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	if instance.State != "Stopped" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Str("state", instance.State).Msg("instance must be stopped")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "instance_not_stopped",
			Message: fmt.Sprintf("instance must be stopped to modify resources (current state: %s)", instance.State),
		})
	}

	hostInfo, err := h.client.GetHostInfo()
	if err != nil {
		logger.API.Load().Error().Err(err).Msg("failed to get host info")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "host_error",
			Message: "failed to get host information",
		})
	}

	if req.CPUs > 0 && int64(req.CPUs) > hostInfo.CPUAvailable {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Int("requested", req.CPUs).Int64("available", hostInfo.CPUAvailable).Msg("CPU exceeds available")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "resource_exceeds_host",
			Message: fmt.Sprintf("requested CPUs (%d) exceeds available host CPUs (%d)", req.CPUs, hostInfo.CPUAvailable),
		})
	}

	if req.Memory != "" {
		reqBytes, err := parseMemoryString(req.Memory)
		if err != nil {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "invalid_request",
				Message: "invalid memory format",
			})
		}
		if reqBytes > hostInfo.MemoryAvailable {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Int64("requested", reqBytes).Int64("available", hostInfo.MemoryAvailable).Msg("memory exceeds available")
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "resource_exceeds_host",
				Message: fmt.Sprintf("requested memory (%s) exceeds available host memory (%s)", req.Memory, formatBytesHost(hostInfo.MemoryAvailable)),
			})
		}
	}

	if req.Disk != "" {
		currentDiskBytes := parseDiskString(instance.Disk)
		reqDiskBytes, err := parseMemoryString(req.Disk)
		if err != nil {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "invalid_request",
				Message: "invalid disk format",
			})
		}
		if reqDiskBytes < currentDiskBytes {
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "invalid_request",
				Message: "disk size can only be increased, not decreased",
			})
		}
		if reqDiskBytes > hostInfo.DiskAvailable {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Int64("requested", reqDiskBytes).Int64("available", hostInfo.DiskAvailable).Msg("disk exceeds available")
			return c.JSON(http.StatusBadRequest, models.ErrorResponse{
				Error:   "resource_exceeds_host",
				Message: fmt.Sprintf("requested disk (%s) exceeds available host disk (%s)", req.Disk, formatBytesHost(hostInfo.DiskAvailable)),
			})
		}
	}

	err = h.client.SetInstanceResources(name, req.CPUs, req.Memory, req.Disk)
	if err != nil {
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to update resources")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Msg("instance resources updated")
	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Instance resources updated",
	})
}

func parseMemoryString(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	var multiplier int64 = 1
	if strings.HasSuffix(s, "B") {
		if len(s) > 1 {
			s = s[:len(s)-1]
			if strings.HasSuffix(s, "K") || strings.HasSuffix(s, "k") {
				multiplier = 1024
				s = s[:len(s)-1]
			} else if strings.HasSuffix(s, "M") || strings.HasSuffix(s, "m") {
				multiplier = 1024 * 1024
				s = s[:len(s)-1]
			} else if strings.HasSuffix(s, "G") || strings.HasSuffix(s, "g") {
				multiplier = 1024 * 1024 * 1024
				s = s[:len(s)-1]
			} else if strings.HasSuffix(s, "T") || strings.HasSuffix(s, "t") {
				multiplier = 1024 * 1024 * 1024 * 1024
				s = s[:len(s)-1]
			}
		}
	} else if strings.HasSuffix(s, "K") || strings.HasSuffix(s, "k") {
		multiplier = 1024
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "M") || strings.HasSuffix(s, "m") {
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "G") || strings.HasSuffix(s, "g") {
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "T") || strings.HasSuffix(s, "t") {
		multiplier = 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(val * float64(multiplier)), nil
}

func parseDiskString(s string) int64 {
	if s == "" {
		return 0
	}
	val, _ := parseMemoryString(s)
	return val
}

func formatBytesHost(n int64) string {
	if n == 0 {
		return "0 B"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for n >= int64(div*unit) {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
