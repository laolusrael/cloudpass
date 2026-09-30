package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cloudpass/internal/models"
	"cloudpass/internal/multipass"

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

func runningAsyncMock() *multipass.MockClient {
	mock := multipass.NewMockClient()
	mock.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})
	return mock
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
		`{}`,
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
