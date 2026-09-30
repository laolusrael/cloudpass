package handlers

import (
	"bytes"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"cloudpass/internal/config"
	"cloudpass/internal/models"
	"cloudpass/internal/multipass"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUploadRouter(client multipass.Client, mgr *config.ConfigManager) *echo.Echo {
	e := echo.New()
	handler := NewInstanceHandler(client, mgr)
	e.POST("/instances/:name/upload", handler.Upload)
	return e
}

func uploadTestConfig() *config.ConfigManager {
	return config.NewConfigManager(&config.Config{
		Upload: config.UploadConfig{
			MaxFileSizeMB: 100,
			DefaultPath:   "/home/ubuntu",
		},
	}, "")
}

func newUploadRequest(t *testing.T, url, filename, content, targetPath string) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if filename != "" {
		fw, err := w.CreateFormFile("file", filename)
		require.NoError(t, err)
		_, err = io.WriteString(fw, content)
		require.NoError(t, err)
	}
	if targetPath != "" {
		require.NoError(t, w.WriteField("target_path", targetPath))
	}
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func runningMock() *multipass.MockClient {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})
	return mockClient
}

func TestUpload_SuccessDefaultPath(t *testing.T) {
	mockClient := runningMock()
	e := setupUploadRouter(mockClient, uploadTestConfig())

	req := newUploadRequest(t, "/instances/test-vm/upload", "test.txt", "hello", "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "/home/ubuntu/test.txt")

	_, target := mockClient.LastUpload()
	assert.Equal(t, "/home/ubuntu/test.txt", target)
}

func TestUpload_SuccessTrailingSlashAppendsFilename(t *testing.T) {
	mockClient := runningMock()
	e := setupUploadRouter(mockClient, uploadTestConfig())

	req := newUploadRequest(t, "/instances/test-vm/upload", "test.txt", "hello", "/data/")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "/data/test.txt")

	_, target := mockClient.LastUpload()
	assert.Equal(t, "/data/test.txt", target)
}

func TestUpload_SuccessExplicitFilePath(t *testing.T) {
	mockClient := runningMock()
	e := setupUploadRouter(mockClient, uploadTestConfig())

	req := newUploadRequest(t, "/instances/test-vm/upload", "test.txt", "hello", "/data/custom.txt")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "/data/custom.txt")

	_, target := mockClient.LastUpload()
	assert.Equal(t, "/data/custom.txt", target)
}

func TestUpload_StripsDirectoryFromFilename(t *testing.T) {
	mockClient := runningMock()
	e := setupUploadRouter(mockClient, uploadTestConfig())

	req := newUploadRequest(t, "/instances/test-vm/upload", "../evil.txt", "hello", "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Contains(t, rec.Body.String(), "/home/ubuntu/evil.txt")
}

func TestUpload_MissingFile(t *testing.T) {
	mockClient := runningMock()
	e := setupUploadRouter(mockClient, uploadTestConfig())

	req := newUploadRequest(t, "/instances/test-vm/upload", "", "", "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "file is required")
}

func TestUpload_EmptyFile(t *testing.T) {
	mockClient := runningMock()
	e := setupUploadRouter(mockClient, uploadTestConfig())

	req := newUploadRequest(t, "/instances/test-vm/upload", "empty.txt", "", "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "file is empty")
}

func TestUpload_TooLarge(t *testing.T) {
	mockClient := runningMock()
	mgr := config.NewConfigManager(&config.Config{
		Upload: config.UploadConfig{
			MaxFileSizeMB: 1,
			DefaultPath:   "/home/ubuntu",
		},
	}, "")
	e := setupUploadRouter(mockClient, mgr)

	big := strings.Repeat("a", 1024*1024+100)
	req := newUploadRequest(t, "/instances/test-vm/upload", "big.bin", big, "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "file_too_large")
}

func TestUpload_RequestBodyCapExceeded(t *testing.T) {
	oldOverhead := multipartOverheadBytes
	multipartOverheadBytes = 50
	t.Cleanup(func() { multipartOverheadBytes = oldOverhead })

	mockClient := runningMock()
	mgr := config.NewConfigManager(&config.Config{
		Upload: config.UploadConfig{
			MaxFileSizeMB: 1,
			DefaultPath:   "/home/ubuntu",
		},
	}, "")
	e := setupUploadRouter(mockClient, mgr)

	// Content fits the 1 MB file limit, but multipart framing pushes the
	// total body over the shrunken cap, exercising the MaxBytesReader path.
	content := strings.Repeat("a", 1024*1024)
	req := newUploadRequest(t, "/instances/test-vm/upload", "big.bin", content, "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "file_too_large")
}

func TestUpload_RelativeTargetRejected(t *testing.T) {
	mockClient := runningMock()
	e := setupUploadRouter(mockClient, uploadTestConfig())

	req := newUploadRequest(t, "/instances/test-vm/upload", "test.txt", "hello", "relative/path.txt")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "must be absolute")
}

func TestUpload_InvalidInstanceName(t *testing.T) {
	mockClient := runningMock()
	e := setupUploadRouter(mockClient, uploadTestConfig())

	req := newUploadRequest(t, "/instances/123invalid/upload", "test.txt", "hello", "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestUpload_InstanceNotFound(t *testing.T) {
	mockClient := multipass.NewMockClient()
	e := setupUploadRouter(mockClient, uploadTestConfig())

	req := newUploadRequest(t, "/instances/missing/upload", "test.txt", "hello", "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")
}

func TestUpload_NotRunning(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Stopped"}})
	e := setupUploadRouter(mockClient, uploadTestConfig())

	req := newUploadRequest(t, "/instances/test-vm/upload", "test.txt", "hello", "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "instance_not_running")
}

func TestUpload_MultipassError(t *testing.T) {
	mockClient := runningMock()
	mockClient.SetUploadErr(errors.New("transfer exploded"))
	e := setupUploadRouter(mockClient, uploadTestConfig())

	req := newUploadRequest(t, "/instances/test-vm/upload", "test.txt", "hello", "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "multipass_error")
}

func TestUpload_TempFileRemoved(t *testing.T) {
	mockClient := runningMock()
	e := setupUploadRouter(mockClient, uploadTestConfig())

	req := newUploadRequest(t, "/instances/test-vm/upload", "test.txt", "hello", "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	local, _ := mockClient.LastUpload()
	require.NotEmpty(t, local)
	_, err := os.Stat(local)
	assert.True(t, os.IsNotExist(err), "staging temp file should be removed")
}

func TestUpload_RespectsStagingDir(t *testing.T) {
	staging := t.TempDir()
	mgr := config.NewConfigManager(&config.Config{
		Upload: config.UploadConfig{
			MaxFileSizeMB: 100,
			DefaultPath:   "/home/ubuntu",
			StagingDir:    staging,
		},
	}, "")

	mockClient := runningMock()
	e := setupUploadRouter(mockClient, mgr)

	req := newUploadRequest(t, "/instances/test-vm/upload", "test.txt", "hello", "")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	local, _ := mockClient.LastUpload()
	require.NotEmpty(t, local)
	assert.True(t, strings.HasPrefix(local, staging), "temp file should live in the staging dir, got %s", local)
	_, err := os.Stat(local)
	assert.True(t, os.IsNotExist(err), "staging temp file should be removed")
}

func TestResolveUploadTarget(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		filename string
		defDir   string
		expected string
		wantErr  string
	}{
		{"empty uses default dir", "", "a.txt", "/home/ubuntu", "/home/ubuntu/a.txt", ""},
		{"trailing slash appends", "/data/", "a.txt", "/home/ubuntu", "/data/a.txt", ""},
		{"explicit file path", "/data/b.txt", "a.txt", "/home/ubuntu", "/data/b.txt", ""},
		{"cleans redundant separators", "/data//b.txt", "a.txt", "/home/ubuntu", "/data/b.txt", ""},
		{"relative rejected", "data/b.txt", "a.txt", "/home/ubuntu", "", "must be absolute"},
		{"filesystem root rejected", "/", "a.txt", "/home/ubuntu", "", "must not be the filesystem root"},
		{"dotdot collapsing to root rejected", "/../", "a.txt", "/home/ubuntu", "", "must not be the filesystem root"},
		{"nul rejected", "/data/a\x00.txt", "a.txt", "/home/ubuntu", "", "invalid characters"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveUploadTarget(tt.target, tt.filename, tt.defDir)
			if tt.wantErr == "" {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			}
		})
	}
}

func TestSanitizeUploadFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"a.txt", "a.txt", false},
		{"../evil.txt", "evil.txt", false},
		{`..\\evil.txt`, "evil.txt", false},
		{"/abs/path/a.txt", "a.txt", false},
		{"", "", true},
		{"..", "", true},
		{".", "", true},
		{"a\x00.txt", "", true},
	}

	for _, tt := range tests {
		t.Run("input="+tt.input, func(t *testing.T) {
			got, err := sanitizeUploadFilename(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}
