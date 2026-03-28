package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

	"cloudpass/internal/logger"
	"cloudpass/internal/models"
	"cloudpass/internal/multipass"

	"github.com/labstack/echo/v4"
)

type InstanceHandler struct {
	client multipass.Client
}

func NewInstanceHandler(client multipass.Client) *InstanceHandler {
	return &InstanceHandler{client: client}
}

var instanceNameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*[a-z0-9]$`)

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
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	for i := range instances {
		instance, err := h.client.GetInstance(instances[i].Name)
		if err != nil {
			continue
		}
		instances[i] = *instance
	}

	return c.JSON(http.StatusOK, models.InstanceList{
		Instances: instances,
	})
}

func (h *InstanceHandler) Get(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	instance, err := h.client.GetInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, instance)
}

func (h *InstanceHandler) Create(c echo.Context) error {
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

	instance, err := h.client.CreateInstance(req)
	if err != nil {
		logger.API.Error().Err(err).Str("ip", c.RealIP()).Str("name", req.Name).Msg("failed to create instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Info().Str("ip", c.RealIP()).Str("name", instance.Name).Msg("instance created")
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
		logger.API.Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.DeleteInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			logger.API.Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to delete instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Info().Str("ip", c.RealIP()).Str("name", name).Msg("instance deleted")
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
		logger.API.Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.StartInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			logger.API.Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to start instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Info().Str("ip", c.RealIP()).Str("name", name).Msg("instance started")
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
		logger.API.Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.StopInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			logger.API.Warn().Str("ip", c.RealIP()).Str("name", name).Msg("instance not found")
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		logger.API.Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to stop instance")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	logger.API.Info().Str("ip", c.RealIP()).Str("name", name).Msg("instance stopped")
	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Instance stopped",
	})
}

func (h *InstanceHandler) Restart(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "instance name is required",
		})
	}

	if err := validateInstanceName(name); err != nil {
		logger.API.Warn().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("invalid instance name")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	if err := validateInstanceName(name); err != nil {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.RestartInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Instance restarted",
	})
}

func (h *InstanceHandler) Suspend(c echo.Context) error {
	name := c.Param("name")
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

	err := h.client.SuspendInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Instance suspended",
	})
}

func (h *InstanceHandler) Resume(c echo.Context) error {
	name := c.Param("name")
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

	err := h.client.ResumeInstance(name)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Instance resumed",
	})
}

func (h *InstanceHandler) Export(c echo.Context) error {
	name := c.Param("name")
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

	var req models.ExportInstanceRequest
	if err := c.Bind(&req); err != nil {
		req = models.ExportInstanceRequest{}
	}

	imagePath, err := h.client.ExportInstance(name, req.OutputPath)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, models.InstanceExport{
		Message:   "Instance exported successfully",
		ImagePath: imagePath,
	})
}

func (h *InstanceHandler) Import(c echo.Context) error {
	var req models.ImportInstanceRequest
	if err := c.Bind(&req); err != nil {
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

	instance, err := h.client.ImportInstance(req.ImagePath, req.Name, req.CPUs, req.Memory, req.Disk)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, instance)
}

func (h *InstanceHandler) CreateSnapshot(c echo.Context) error {
	name := c.Param("name")
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

	var req models.CreateSnapshotRequest
	if err := c.Bind(&req); err != nil {
		req = models.CreateSnapshotRequest{}
	}

	err := h.client.CreateSnapshot(name, req.Name, req.Comment)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusCreated, models.InstanceResponse{
		Message: "Snapshot created",
	})
}

func (h *InstanceHandler) ListSnapshots(c echo.Context) error {
	name := c.Param("name")
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

	snapshots, err := h.client.ListSnapshots(name)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, models.SnapshotList{
		Snapshots: snapshots,
	})
}

func (h *InstanceHandler) RestoreSnapshot(c echo.Context) error {
	name := c.Param("name")
	snapshotID := c.Param("id")

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
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Snapshot restored",
	})
}

func (h *InstanceHandler) DeleteSnapshot(c echo.Context) error {
	name := c.Param("name")
	snapshotID := c.Param("id")

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

	err := h.client.DeleteSnapshot(name, snapshotID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return c.JSON(http.StatusNotFound, models.ErrorResponse{
				Error:   "not_found",
				Message: err.Error(),
			})
		}
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Snapshot deleted",
	})
}
