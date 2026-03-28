package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cloudpass/internal/models"
	"cloudpass/internal/multipass"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func setupNetworkRouter(client multipass.Client) *echo.Echo {
	e := echo.New()
	handler := NewNetworkHandler(client)
	e.POST("/networks", handler.Create)
	e.DELETE("/networks/:name", handler.Delete)
	e.GET("/networks", handler.List)
	return e
}

func TestNetworkHandler_List_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetNetworks([]models.Network{
		{Name: "test-net", Type: "bridge", IPv4: "10.0.0.1"},
	})

	e := setupNetworkRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/networks", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "test-net")
}

func TestNetworkHandler_List_Error(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetListNetworksErr(assert.AnError)

	e := setupNetworkRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/networks", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestNetworkHandler_Create_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupNetworkRouter(mockClient)

	body := `{"name": "my-network"}`
	req := httptest.NewRequest(http.MethodPost, "/networks", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "Network created")
}

func TestNetworkHandler_Create_EmptyName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupNetworkRouter(mockClient)

	body := `{"name": ""}`
	req := httptest.NewRequest(http.MethodPost, "/networks", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "network name is required")
}

func TestNetworkHandler_Create_InvalidBody(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupNetworkRouter(mockClient)

	body := `{"invalid`
	req := httptest.NewRequest(http.MethodPost, "/networks", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNetworkHandler_Create_Error(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetCreateNetworkErr(assert.AnError)

	e := setupNetworkRouter(mockClient)

	body := `{"name": "my-network"}`
	req := httptest.NewRequest(http.MethodPost, "/networks", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestNetworkHandler_Create_WithModeAndMAC(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupNetworkRouter(mockClient)

	body := `{"name": "my-network", "mode": "manual", "mac": "aa:bb:cc:dd:ee:ff"}`
	req := httptest.NewRequest(http.MethodPost, "/networks", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestNetworkHandler_Delete_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupNetworkRouter(mockClient)

	req := httptest.NewRequest(http.MethodDelete, "/networks/test-net", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Network deleted")
}

func TestNetworkHandler_Delete_Error(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetDeleteNetworkErr(assert.AnError)

	e := setupNetworkRouter(mockClient)

	req := httptest.NewRequest(http.MethodDelete, "/networks/test-net", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestNetworkHandler_Delete_EmptyName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := echo.New()
	handler := NewNetworkHandler(mockClient)
	e.DELETE("/networks/:name", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/networks/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
