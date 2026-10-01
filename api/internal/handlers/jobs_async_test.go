package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloudpass/internal/models"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func doAsync(t *testing.T, handler *JobHandler, method func(echo.Context) error, url string, params map[string]string, body string, key string) (int, models.JobResponse) {
	t.Helper()

	e := echo.New()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(http.MethodPost, url, nil)
	} else {
		req = httptest.NewRequest(http.MethodPost, url, strings.NewReader(body))
	}
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if key != "" {
		req.Header.Set(IdempotencyKeyHeader, key)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if len(params) > 0 {
		names := make([]string, 0, len(params))
		values := make([]string, 0, len(params))
		for k, v := range params {
			names = append(names, k)
			values = append(values, v)
		}
		c.SetParamNames(names...)
		c.SetParamValues(values...)
	}

	require.NoError(t, method(c))

	var resp models.JobResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotNil(t, resp.Job)
	return rec.Code, resp
}

func TestMountAsync_Completes(t *testing.T) {
	handler, storage, mock := testJobHandler(t)
	mock.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	code, resp := doAsync(t, handler, handler.CreateMountAsync, "/instances/test-vm/mounts/async",
		map[string]string{"name": "test-vm"},
		`{"source_path": "/home/user/projects", "target_path": "/home/ubuntu/projects"}`,
		"key-mount-1")

	assert.Equal(t, http.StatusAccepted, code)
	assert.Equal(t, "mount", resp.Job.Type)
	// The background worker may already have picked the job up.
	assert.Contains(t, []models.JobStatus{models.JobStatusPending, models.JobStatusRunning}, resp.Job.Status)

	job := waitForJobSettled(t, storage, resp.Job.ID)
	assert.Equal(t, models.JobStatusCompleted, job.Status)
	assert.Contains(t, job.Result, "/home/user/projects")

	source, target, opts := mock.LastMount()
	assert.Equal(t, "/home/user/projects", source)
	assert.Equal(t, "/home/ubuntu/projects", target)
	assert.Equal(t, "classic", opts.Type)
}

func TestMountAsync_Replay(t *testing.T) {
	handler, storage, mock := testJobHandler(t)
	mock.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	first := func() (int, models.JobResponse) {
		return doAsync(t, handler, handler.CreateMountAsync, "/instances/test-vm/mounts/async",
			map[string]string{"name": "test-vm"},
			`{"source_path": "/a", "target_path": "/b"}`,
			"key-mount-replay")
	}

	code1, resp1 := first()
	assert.Equal(t, http.StatusAccepted, code1)
	code2, resp2 := first()
	assert.Equal(t, http.StatusOK, code2)
	assert.Equal(t, resp1.Job.ID, resp2.Job.ID)

	waitForJobSettled(t, storage, resp1.Job.ID)
}

func TestMountAsync_Validation(t *testing.T) {
	handler, _, _ := testJobHandler(t)

	cases := []struct {
		name   string
		params map[string]string
		body   string
		want   string
	}{
		{"missing source", map[string]string{"name": "test-vm"}, `{"target_path": "/b"}`, "source_path is required"},
		{"bad type", map[string]string{"name": "test-vm"}, `{"source_path": "/a", "target_path": "/b", "mount_type": "smb"}`, "mount_type must be classic or native"},
		{"bad key", map[string]string{"name": "test-vm"}, `{"source_path": "/a", "target_path": "/b"}`, "idempotency key"},
	}

	keys := map[string]string{"missing source": "", "bad type": "", "bad key": "bad key!!"}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(tt.body))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			if keys[tt.name] != "" {
				req.Header.Set(IdempotencyKeyHeader, keys[tt.name])
			}
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("name")
			c.SetParamValues("test-vm")

			require.NoError(t, handler.CreateMountAsync(c))
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, rec.Body.String(), tt.want)
		})
	}
}

func TestSnapshotCreateAsync_Completes(t *testing.T) {
	handler, storage, mock := testJobHandler(t)
	mock.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	code, resp := doAsync(t, handler, handler.CreateSnapshotAsync, "/instances/test-vm/snapshots/async",
		map[string]string{"name": "test-vm"},
		`{"name": "snap1"}`,
		"key-snap-1")

	assert.Equal(t, http.StatusAccepted, code)
	assert.Equal(t, "snapshot_create", resp.Job.Type)

	job := waitForJobSettled(t, storage, resp.Job.ID)
	assert.Equal(t, models.JobStatusCompleted, job.Status)
	assert.Equal(t, "snap1", job.Result)
}

func TestSnapshotRestoreAsync_Completes(t *testing.T) {
	handler, storage, mock := testJobHandler(t)
	mock.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	code, resp := doAsync(t, handler, handler.RestoreSnapshotAsync, "/instances/test-vm/snapshots/snap1/restore/async",
		map[string]string{"name": "test-vm", "id": "snap1"},
		"",
		"key-restore-1")

	assert.Equal(t, http.StatusAccepted, code)
	assert.Equal(t, "snapshot_restore", resp.Job.Type)

	job := waitForJobSettled(t, storage, resp.Job.ID)
	assert.Equal(t, models.JobStatusCompleted, job.Status)
	assert.Equal(t, "snap1", job.Result)
}

func TestExportAsync_Completes(t *testing.T) {
	handler, storage, mock := testJobHandler(t)
	mock.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	code, resp := doAsync(t, handler, handler.ExportAsync, "/instances/test-vm/export/async",
		map[string]string{"name": "test-vm"},
		`{"compress": false}`,
		"key-export-1")

	assert.Equal(t, http.StatusAccepted, code)
	assert.Equal(t, "export", resp.Job.Type)

	job := waitForJobSettled(t, storage, resp.Job.ID)
	assert.Equal(t, models.JobStatusCompleted, job.Status)
	assert.Contains(t, job.Result, "test-vm")
}

func TestImportAsync_Completes(t *testing.T) {
	handler, storage, _ := testJobHandler(t)

	code, resp := doAsync(t, handler, handler.ImportAsync, "/instances/import/async",
		nil,
		`{"image_path": "/images/base.img", "name": "imported-vm"}`,
		"key-import-1")

	assert.Equal(t, http.StatusAccepted, code)
	assert.Equal(t, "import", resp.Job.Type)

	job := waitForJobSettled(t, storage, resp.Job.ID)
	assert.Equal(t, models.JobStatusCompleted, job.Status)
	assert.Equal(t, "imported-vm", job.InstanceName)
	assert.Equal(t, "imported-vm", job.Result)
}

func TestImportAsync_MissingImage(t *testing.T) {
	handler, _, _ := testJobHandler(t)

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/instances/import/async", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.ImportAsync(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "image_path is required")
}

func newImportUploadRequest(t *testing.T, url string, filename string, content string, fields map[string]string) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	if filename != "" {
		fw, err := w.CreateFormFile("file", filename)
		require.NoError(t, err)
		_, err = io.WriteString(fw, content)
		require.NoError(t, err)
	}
	for k, v := range fields {
		require.NoError(t, w.WriteField(k, v))
	}
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestImportAsync_MultipartStagesAndCleans(t *testing.T) {
	handler, storage, mock := testJobHandler(t)

	e := echo.New()
	req := newImportUploadRequest(t, "/instances/import/async", "web-server.img", "fake-image", map[string]string{})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.ImportAsync(c))
	assert.Equal(t, http.StatusAccepted, rec.Code)

	var resp models.JobResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	job := waitForJobSettled(t, storage, resp.Job.ID)
	require.Equal(t, models.JobStatusCompleted, job.Status)
	assert.Equal(t, "web-server", job.InstanceName)

	staged := mock.LastImportPath()
	require.NotEmpty(t, staged)
	_, err := os.Stat(staged)
	assert.True(t, os.IsNotExist(err), "staged upload should be removed after launch")
}

func TestImportAsync_DuplicateNameIsConflict(t *testing.T) {
	handler, _, mock := testJobHandler(t)
	mock.SetInstances([]models.Instance{{Name: "taken", State: "Stopped"}})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/instances/import/async", strings.NewReader(`{"image_path": "/images/base.img", "name": "taken"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	require.NoError(t, handler.ImportAsync(c))
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "already exists")
	assert.Contains(t, rec.Body.String(), "Stopped")
}

func TestDeriveInstanceName(t *testing.T) {
	assert.Equal(t, "web-server", deriveInstanceName("web-server.img"))
	assert.Equal(t, "myvm", deriveInstanceName("My_VM.qcow2"))
	assert.Equal(t, "", deriveInstanceName("!!!.img"))
	assert.Equal(t, "", deriveInstanceName(".img"))
}

func TestAsyncJob_FailureRecorded(t *testing.T) {
	handler, storage, mock := testJobHandler(t)
	mock.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})
	mock.SetMountErr(assert.AnError)

	_, resp := doAsync(t, handler, handler.CreateMountAsync, "/instances/test-vm/mounts/async",
		map[string]string{"name": "test-vm"},
		`{"source_path": "/a", "target_path": "/b"}`,
		"key-mount-fail")

	job := waitForJobSettled(t, storage, resp.Job.ID)
	assert.Equal(t, models.JobStatusFailed, job.Status)
	assert.NotEmpty(t, job.Error)
}

func seedExportJob(t *testing.T, storage *JobStorage, instanceName string, result string) string {
	t.Helper()

	job := &models.Job{
		ID:           generateJobID(),
		Type:         "export",
		Status:       models.JobStatusCompleted,
		InstanceName: instanceName,
		Result:       result,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	storage.Set(job)
	return job.ID
}

func TestDownloadExport_ServesAndRemoves(t *testing.T) {
	handler, storage, _ := testJobHandler(t)

	content := []byte("fake-image-bytes")
	imgPath := filepath.Join(t.TempDir(), "cloudpass-export-test-vm-1.img")
	require.NoError(t, os.WriteFile(imgPath, content, 0600))
	require.NoError(t, os.WriteFile(imgPath+".json", []byte(`{"instance":"test-vm"}`), 0600))
	jobID := seedExportJob(t, storage, "test-vm", imgPath)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/instances/test-vm/export/download?job_id="+jobID, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("name")
	c.SetParamValues("test-vm")

	require.NoError(t, handler.DownloadExport(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "cloudpass-export-test-vm-1.img")
	assert.Equal(t, content, rec.Body.Bytes())
	assert.NoFileExists(t, imgPath)
	assert.NoFileExists(t, imgPath+".json")
}

func TestDownloadExport_Sidecar(t *testing.T) {
	handler, storage, _ := testJobHandler(t)

	imgPath := filepath.Join(t.TempDir(), "cloudpass-export-test-vm-2.img")
	require.NoError(t, os.WriteFile(imgPath, []byte("img"), 0600))
	sidecar := []byte(`{"instance":"test-vm","driver":"qemu"}`)
	require.NoError(t, os.WriteFile(imgPath+".json", sidecar, 0600))
	jobID := seedExportJob(t, storage, "test-vm", imgPath)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/instances/test-vm/export/download?job_id="+jobID+"&sidecar=true", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("name")
	c.SetParamValues("test-vm")

	require.NoError(t, handler.DownloadExport(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, sidecar, rec.Body.Bytes())
}

func TestDownloadExport_States(t *testing.T) {
	handler, storage, _ := testJobHandler(t)

	pending := &models.Job{ID: generateJobID(), Type: "export", Status: models.JobStatusRunning, InstanceName: "test-vm", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	storage.Set(pending)

	get := func(name string, jobID string) *httptest.ResponseRecorder {
		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/instances/"+name+"/export/download?job_id="+jobID, nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("name")
		c.SetParamValues(name)
		require.NoError(t, handler.DownloadExport(c))
		return rec
	}

	assert.Equal(t, http.StatusNotFound, get("test-vm", "nope").Code)
	assert.Equal(t, http.StatusNotFound, get("other-vm", pending.ID).Code)
	assert.Equal(t, http.StatusConflict, get("test-vm", pending.ID).Code)

	goneID := seedExportJob(t, storage, "test-vm", filepath.Join(t.TempDir(), "missing.img"))
	assert.Equal(t, http.StatusGone, get("test-vm", goneID).Code)

	outsideID := seedExportJob(t, storage, "test-vm", string(filepath.Separator)+"outside-cloudpass.img")
	assert.Equal(t, http.StatusConflict, get("test-vm", outsideID).Code)
}

func TestIsStagedArtifact(t *testing.T) {
	dir := t.TempDir()
	assert.True(t, isStagedArtifact(dir, filepath.Join(dir, "a.img")))
	assert.False(t, isStagedArtifact(dir, filepath.Join(dir, "..", "escape.img")))
	assert.False(t, isStagedArtifact(dir, filepath.Join(t.TempDir(), "other.img")))
}

func TestExportAsync_OverwriteGuard(t *testing.T) {
	handler, storage, _ := testJobHandler(t)

	existing := filepath.Join(t.TempDir(), "taken.img")
	require.NoError(t, os.WriteFile(existing, []byte("x"), 0600))

	body := `{"output_path":` + strconv.Quote(existing) + `}`
	_, resp := doAsync(t, handler, handler.ExportAsync, "/instances/test-vm/export/async",
		map[string]string{"name": "test-vm"}, body, "key-export-guard")

	job := waitForJobSettled(t, storage, resp.Job.ID)
	assert.Equal(t, models.JobStatusFailed, job.Status)
	assert.Contains(t, job.Error, "already exists")
}

func TestExportAsync_DeletedRejectedSync(t *testing.T) {
	handler, _, mock := testJobHandler(t)
	mock.SetInstances([]models.Instance{{Name: "test-vm", State: "Deleted"}})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/instances/test-vm/export/async", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("name")
	c.SetParamValues("test-vm")

	require.NoError(t, handler.ExportAsync(c))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "recover it before exporting")
}

func TestCompressDecompressRoundTrip(t *testing.T) {
	dir := t.TempDir()
	// Zero-heavy content like a thin VM disk.
	content := append(bytes.Repeat([]byte{0}, 256*1024), bytes.Repeat([]byte("cloudpass"), 1024)...)
	src := filepath.Join(dir, "disk.img")
	require.NoError(t, os.WriteFile(src, content, 0600))

	zst, err := compressImage(src)
	require.NoError(t, err)
	assert.Equal(t, src+".zst", zst)
	assert.True(t, isZstdImage(zst))
	assert.False(t, isZstdImage(src))

	infoSrc, _ := os.Stat(src)
	infoZst, _ := os.Stat(zst)
	require.NotNil(t, infoSrc)
	require.NotNil(t, infoZst)
	assert.Less(t, infoZst.Size(), infoSrc.Size(), "compressed image should be smaller")

	out, err := decompressImage(dir, zst)
	require.NoError(t, err)
	roundTripped, err := os.ReadFile(out)
	require.NoError(t, err)
	assert.Equal(t, content, roundTripped)
}

func TestIsZstdImage_MissingFile(t *testing.T) {
	assert.False(t, isZstdImage(filepath.Join(t.TempDir(), "nope.img")))
}
