package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cloudpass/internal/models"
	"cloudpass/internal/multipass"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestValidateInstanceName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid simple", "vm1", true},
		{"valid with hyphen", "my-vm", true},
		{"valid with numbers", "test-vm-1", true},
		{"valid two chars", "ab", true},
		{"valid max length", strings.Repeat("a", 63), true},
		{"invalid starts with number", "123vm", false},
		{"invalid ends with hyphen", "vm-", false},
		{"invalid uppercase", "VM", false},
		{"invalid special chars", "vm@test", false},
		{"invalid underscore", "vm_test", false},
		{"invalid single char hyphen", "-", false},
		{"invalid single char dot", ".", false},
		{"empty", "", false},
		{"too long", strings.Repeat("a", 64), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateInstanceName(tt.input)
			if tt.expected {
				assert.NoError(t, err, "expected no error for %s", tt.input)
			} else {
				assert.Error(t, err, "expected error for %s", tt.input)
			}
		})
	}
}

func setupInstanceRouter(client multipass.Client) *echo.Echo {
	e := echo.New()
	handler := NewInstanceHandler(client)
	e.GET("/instances", handler.List)
	e.GET("/instances/:name", handler.Get)
	e.POST("/instances", handler.Create)
	e.DELETE("/instances/:name", handler.Delete)
	e.POST("/instances/:name/suspend", handler.Suspend)
	e.POST("/instances/:name/resume", handler.Resume)
	e.POST("/instances/:name/start", handler.Start)
	e.POST("/instances/:name/stop", handler.Stop)
	e.POST("/instances/:name/restart", handler.Restart)
	return e
}

func TestSuspend_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/suspend", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Instance suspended")
}

func TestSuspend_EmptyName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances//suspend", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSuspend_InvalidName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/123invalid/suspend", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "must start with a letter")
}

func TestSuspend_InstanceNotFound(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetSuspendErr(errors.New("instance does not exist"))

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/suspend", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "does not exist")
}

func TestSuspend_MultipassError(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetSuspendErr(errors.New("multipass service unavailable"))

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/suspend", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestResume_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Stopped"}})

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/resume", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Instance resumed")
}

func TestResume_EmptyName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances//resume", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestResume_InvalidName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/vm-test-/resume", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "must end with an alphanumeric")
}

func TestResume_InstanceNotFound(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetResumeErr(errors.New("instance does not exist"))

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/resume", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestResume_MultipassError(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetResumeErr(errors.New("multipass service unavailable"))

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/resume", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestList_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "vm1", State: "Running"}})

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/instances", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "vm1")
}

func TestList_Empty(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{})

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/instances", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "[]")
}

func TestGet_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "vm1", State: "Running", CPU: 4}})

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/instances/vm1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "vm1")
}

func TestGet_NotFound(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetGetInstanceErr(errors.New("instance not found"))

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/instances/vm1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGet_EmptyName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/instances/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCreate_InvalidBody(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances", strings.NewReader("invalid"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCreate_InvalidName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupInstanceRouter(mockClient)

	body := `{"name": "123invalid"}`
	req := httptest.NewRequest(http.MethodPost, "/instances", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestStart_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/test/start", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestStop_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/test/stop", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRestart_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupInstanceRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/test/restart", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}
