package handlers

import (
	"errors"
	"net/http"
	"regexp"
	"strings"

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
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
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

	instance, err := h.client.CreateInstance(req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

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
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.DeleteInstance(name)
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
		Message: "Instance deleted",
	})
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
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.StartInstance(name)
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
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: err.Error(),
		})
	}

	err := h.client.StopInstance(name)
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
