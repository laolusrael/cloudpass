package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cloudpass/internal/config"
	"cloudpass/internal/models"
	"cloudpass/internal/multipass"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func testConfig() *config.Config {
	return &config.Config{
		Server: config.ServerConfig{Host: "0.0.0.0", Port: 8080},
		Multipass: config.MultipassConfig{
			DefaultTimeoutSec: 300,
		},
		Upload: config.UploadConfig{
			MaxFileSizeMB: 100,
			DefaultPath:   "/home/ubuntu",
		},
	}
}

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
	handler := NewInstanceHandler(client, testConfig())
	e.GET("/instances", handler.List)
	e.GET("/instances/:name", handler.Get)
	e.POST("/instances", handler.Create)
	e.DELETE("/instances/:name", handler.Delete)
	e.POST("/instances/:name/suspend", handler.Suspend)
	e.POST("/instances/:name/resume", handler.Resume)
	e.POST("/instances/:name/start", handler.Start)
	e.POST("/instances/:name/stop", handler.Stop)
	e.POST("/instances/:name/restart", handler.Restart)
	e.PUT("/instances/:name/resources", handler.UpdateResources)
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

func setupSnapshotRouter(client multipass.Client) *echo.Echo {
	e := echo.New()
	handler := NewInstanceHandler(client, testConfig())
	e.POST("/instances/:name/snapshots", handler.CreateSnapshot)
	e.GET("/instances/:name/snapshots", handler.ListSnapshots)
	e.POST("/instances/:name/snapshots/:id/restore", handler.RestoreSnapshot)
	e.DELETE("/instances/:name/snapshots/:id", handler.DeleteSnapshot)
	e.POST("/instances/:name/export", handler.Export)
	e.POST("/instances/import", handler.Import)
	return e
}

func TestCreateSnapshot_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupSnapshotRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/snapshots", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "Snapshot created")
}

func TestCreateSnapshot_EmptyName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupSnapshotRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances//snapshots", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestListSnapshots_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupSnapshotRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/instances/test-vm/snapshots", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "snap1")
}

func TestListSnapshots_EmptyName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupSnapshotRouter(mockClient)

	req := httptest.NewRequest(http.MethodGet, "/instances//snapshots", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestRestoreSnapshot_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupSnapshotRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/snapshots/snap1/restore", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Snapshot restored")
}

func TestRestoreSnapshot_EmptyName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupSnapshotRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances//snapshots/snap1/restore", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestDeleteSnapshot_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupSnapshotRouter(mockClient)

	req := httptest.NewRequest(http.MethodDelete, "/instances/test-vm/snapshots/snap1", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "Snapshot deleted")
}

func TestExport_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupSnapshotRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/export", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "exported successfully")
}

func TestExport_EmptyName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupSnapshotRouter(mockClient)

	req := httptest.NewRequest(http.MethodPost, "/instances//export", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestImport_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupSnapshotRouter(mockClient)

	body := `{"image_path": "/path/to/image.img", "name": "imported-vm"}`
	req := httptest.NewRequest(http.MethodPost, "/instances/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
}

func TestImport_MissingImagePath(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupSnapshotRouter(mockClient)

	body := `{"name": "imported-vm"}`
	req := httptest.NewRequest(http.MethodPost, "/instances/import", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "image_path is required")
}

func setupMountRouter(client multipass.Client) *echo.Echo {
	e := echo.New()
	handler := NewInstanceHandler(client, testConfig())
	e.POST("/instances/:name/mounts", handler.Mount)
	e.DELETE("/instances/:name/mounts", handler.Unmount)
	return e
}

func TestMount_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	e := setupMountRouter(mockClient)

	body := `{"source_path": "/home/user/projects", "target_path": "/home/ubuntu/projects"}`
	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/mounts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "mounted")
}

func TestMount_MissingSourcePath(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	e := setupMountRouter(mockClient)

	body := `{"target_path": "/home/ubuntu/projects"}`
	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/mounts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "source_path is required")
}

func TestMount_MissingTargetPath(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	e := setupMountRouter(mockClient)

	body := `{"source_path": "/home/user/projects"}`
	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/mounts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "target_path is required")
}

func TestMount_EmptyName(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupMountRouter(mockClient)

	body := `{"source_path": "/home/user/projects", "target_path": "/home/ubuntu/projects"}`
	req := httptest.NewRequest(http.MethodPost, "/instances//mounts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUnmount_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	e := setupMountRouter(mockClient)

	body := `{"target_path": "/home/ubuntu/projects"}`
	req := httptest.NewRequest(http.MethodDelete, "/instances/test-vm/mounts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "unmounted")
}

func TestUnmount_MissingTargetPath(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	e := setupMountRouter(mockClient)

	body := `{}`
	req := httptest.NewRequest(http.MethodDelete, "/instances/test-vm/mounts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "target_path is required")
}

func TestUpdateResources_Success(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Stopped", CPU: 1, Memory: "1G", Disk: "5G"}})

	e := setupInstanceRouter(mockClient)

	body := `{"cpus": 2, "memory": "2G", "disk": "10G"}`
	req := httptest.NewRequest(http.MethodPut, "/instances/test-vm/resources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "updated")
}

func TestUpdateResources_InstanceNotStopped(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Running", CPU: 1, Memory: "1G", Disk: "5G"}})

	e := setupInstanceRouter(mockClient)

	body := `{"cpus": 2}`
	req := httptest.NewRequest(http.MethodPut, "/instances/test-vm/resources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "instance_not_stopped")
}

func TestUpdateResources_CPUExceedsAvailable(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Stopped", CPU: 1, Memory: "1G", Disk: "5G"}})

	e := setupInstanceRouter(mockClient)

	body := `{"cpus": 100}`
	req := httptest.NewRequest(http.MethodPut, "/instances/test-vm/resources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "resource_exceeds_host")
}

func TestUpdateResources_MemoryExceedsAvailable(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Stopped", CPU: 1, Memory: "1G", Disk: "5G"}})

	e := setupInstanceRouter(mockClient)

	body := `{"memory": "100G"}`
	req := httptest.NewRequest(http.MethodPut, "/instances/test-vm/resources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "resource_exceeds_host")
}

func TestUpdateResources_DiskShrinkRejected(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Stopped", CPU: 1, Memory: "1G", Disk: "10G"}})

	e := setupInstanceRouter(mockClient)

	body := `{"disk": "5G"}`
	req := httptest.NewRequest(http.MethodPut, "/instances/test-vm/resources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "can only be increased")
}

func TestUpdateResources_DiskExceedsAvailable(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Stopped", CPU: 1, Memory: "1G", Disk: "5G"}})

	e := setupInstanceRouter(mockClient)

	body := `{"disk": "1000G"}`
	req := httptest.NewRequest(http.MethodPut, "/instances/test-vm/resources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "resource_exceeds_host")
}

func TestUpdateResources_InstanceNotFound(t *testing.T) {
	mockClient := multipass.NewMockClient()

	e := setupInstanceRouter(mockClient)

	body := `{"cpus": 2}`
	req := httptest.NewRequest(http.MethodPut, "/instances/nonexistent/resources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestUpdateResources_InvalidMemoryFormat(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Stopped", CPU: 1, Memory: "1G", Disk: "5G"}})

	e := setupInstanceRouter(mockClient)

	body := `{"memory": "invalid"}`
	req := httptest.NewRequest(http.MethodPut, "/instances/test-vm/resources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid memory format")
}

func TestUpdateResources_DecimalMemoryFormat(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Stopped", CPU: 1, Memory: "1G", Disk: "5G"}})

	e := setupInstanceRouter(mockClient)

	body := `{"memory": "4.0G"}`
	req := httptest.NewRequest(http.MethodPut, "/instances/test-vm/resources", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestParseMemoryString(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1G", 1024 * 1024 * 1024},
		{"4.0G", 4 * 1024 * 1024 * 1024},
		{"25.0G", 25 * 1024 * 1024 * 1024},
		{"512M", 512 * 1024 * 1024},
		{"2.5G", int64(2.5 * 1024 * 1024 * 1024)},
		{"1024", 1024},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := parseMemoryString(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
