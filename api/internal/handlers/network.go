package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"cloudpass/internal/logger"
	"cloudpass/internal/models"
	"cloudpass/internal/multipass"

	"github.com/labstack/echo/v4"
)

type NetworkHandler struct {
	client   multipass.Client
	netUsage *NetworkUsageStore
}

func NewNetworkHandler(client multipass.Client, netUsage *NetworkUsageStore) *NetworkHandler {
	return &NetworkHandler{client: client, netUsage: netUsage}
}

func (h *NetworkHandler) List(c echo.Context) error {
	networks, err := h.client.ListNetworks()
	if err != nil {
		logger.API.Load().Error().Err(err).Msg("failed to list networks")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	resp := models.NetworkList{Networks: networks}
	if h.netUsage != nil {
		present := make([]string, 0, len(networks))
		for _, network := range networks {
			present = append(present, network.Name)
		}
		h.netUsage.PruneManaged(present)

		instances, err := h.client.GetAllInstancesInfo()
		if err != nil {
			logger.API.Load().Warn().Err(err).Msg("failed to list instances for network usage, omitting usage info")
		} else {
			// Drop attributions for instances that no longer exist so
			// the store cannot block deletions forever.
			known := make(map[string]bool, len(instances))
			for _, instance := range instances {
				known[instance.Name] = true
			}
			for _, instance := range h.netUsage.trackedInstances() {
				if !known[instance] {
					h.netUsage.ClearInstanceNetwork(instance)
				}
			}

			usedBy, unverified := h.netUsage.UsageReport(instances)
			for i := range resp.Networks {
				resp.Networks[i].UsedBy = usedBy[resp.Networks[i].Name]
			}
			resp.UnverifiedInstances = unverified
		}
	}

	logger.API.Load().Debug().Int("count", len(networks)).Msg("listed networks")
	return c.JSON(http.StatusOK, resp)
}

func (h *NetworkHandler) Create(c echo.Context) error {
	var req models.CreateNetworkRequest
	if err := c.Bind(&req); err != nil {
		logger.API.Load().Warn().Err(err).Str("ip", c.RealIP()).Msg("invalid request body")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "invalid request body",
		})
	}

	if req.Name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("network name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "network name is required",
		})
	}

	err := h.client.CreateNetwork(req.Name, req.Mode, req.MAC)
	if err != nil {
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", req.Name).Msg("failed to create network")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	if h.netUsage != nil {
		h.netUsage.RecordCreatedNetwork(req.Name)
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", req.Name).Msg("network created")
	return c.JSON(http.StatusCreated, models.InstanceResponse{
		Message: "Network created",
	})
}

func (h *NetworkHandler) Delete(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		logger.API.Load().Warn().Str("ip", c.RealIP()).Msg("network name is required")
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "network name is required",
		})
	}

	// Deleting a network in use strands the attached instances, so a
	// confirmed user blocks the delete outright. Anything CloudPass
	// cannot see (external or unknown attachments) is left to
	// multipassd, whose own refusal is surfaced verbatim below.
	if h.netUsage != nil {
		instances, err := h.client.GetAllInstancesInfo()
		if err != nil {
			logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Msg("failed to list instances for network delete guard")
			return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
				Error:   "multipass_error",
				Message: err.Error(),
			})
		}
		usedBy, _ := h.netUsage.UsageReport(instances)
		if users := usedBy[name]; len(users) > 0 {
			logger.API.Load().Warn().Str("ip", c.RealIP()).Str("name", name).Strs("used_by", users).Msg("refusing to delete network in use")
			return c.JSON(http.StatusConflict, models.NetworkInUseResponse{
				Error:   "network_in_use",
				Message: fmt.Sprintf("network %q is in use by: %s", name, strings.Join(users, ", ")),
				UsedBy:  users,
			})
		}
	}

	err := h.client.DeleteNetwork(name)
	if err != nil {
		logger.API.Load().Error().Err(err).Str("ip", c.RealIP()).Str("name", name).Msg("failed to delete network")
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}

	if h.netUsage != nil {
		h.netUsage.RemoveCreatedNetwork(name)
	}

	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Msg("network deleted")
	return c.JSON(http.StatusOK, models.InstanceResponse{
		Message: "Network deleted",
	})
}

// ClaimNetwork marks an existing network (e.g. created before tracking
// began) as CloudPass-managed so it can be offered for deletion.
func (h *NetworkHandler) ClaimNetwork(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "network name is required",
		})
	}
	if h.netUsage == nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "unavailable",
			Message: "network usage tracking is not configured",
		})
	}

	networks, err := h.client.ListNetworks()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "multipass_error",
			Message: err.Error(),
		})
	}
	found := false
	for _, network := range networks {
		if network.Name == name {
			found = true
			break
		}
	}
	if !found {
		return c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error:   "not_found",
			Message: fmt.Sprintf("network %q not found", name),
		})
	}

	h.netUsage.RecordCreatedNetwork(name)
	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Msg("network claimed as managed")
	return c.JSON(http.StatusOK, map[string]string{"name": name, "status": "claimed"})
}

// UnclaimNetwork drops the managed mark. The network itself is untouched.
func (h *NetworkHandler) UnclaimNetwork(c echo.Context) error {
	name := c.Param("name")
	if name == "" {
		return c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:   "invalid_request",
			Message: "network name is required",
		})
	}
	if h.netUsage == nil {
		return c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error:   "unavailable",
			Message: "network usage tracking is not configured",
		})
	}

	h.netUsage.RemoveCreatedNetwork(name)
	logger.API.Load().Info().Str("ip", c.RealIP()).Str("name", name).Msg("network unclaimed")
	return c.JSON(http.StatusOK, map[string]string{"name": name, "status": "unclaimed"})
}
