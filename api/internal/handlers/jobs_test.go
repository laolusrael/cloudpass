package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"cloudpass/internal/config"
	"cloudpass/internal/models"
	"cloudpass/internal/multipass"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testJobHandler(t *testing.T) (*JobHandler, *JobStorage, *multipass.MockClient) {
	t.Helper()
	storage, err := NewJobStorage(t.TempDir())
	require.NoError(t, err)
	mock := multipass.NewMockClient()
	handler := NewJobHandler(mock, 60, storage, NewEventHub(), testConfigManager(t, &config.Config{}), mustNetworkUsage(t))
	return handler, storage, mock
}

func doPostAsync(handler *JobHandler, name, idempotencyKey string) (int, models.JobResponse, error) {
	e := echo.New()

	body := fmt.Sprintf(`{"name":%q}`, name)
	req := httptest.NewRequest(http.MethodPost, "/api/instances/async", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if idempotencyKey != "" {
		req.Header.Set(IdempotencyKeyHeader, idempotencyKey)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := handler.CreateInstanceAsync(c); err != nil {
		return 0, models.JobResponse{}, err
	}

	var resp models.JobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		return 0, models.JobResponse{}, err
	}
	if resp.Job == nil {
		return 0, models.JobResponse{}, fmt.Errorf("nil job in response")
	}
	return rec.Code, resp, nil
}

func postAsyncJob(t *testing.T, handler *JobHandler, name, idempotencyKey string) (int, models.JobResponse) {
	t.Helper()
	code, resp, err := doPostAsync(handler, name, idempotencyKey)
	require.NoError(t, err)
	return code, resp
}

func waitForJobSettled(t *testing.T, storage *JobStorage, id string) *models.Job {
	t.Helper()
	var job *models.Job
	require.Eventually(t, func() bool {
		var ok bool
		job, ok = storage.Get(id)
		if !ok {
			return false
		}
		return job.Status == models.JobStatusCompleted || job.Status == models.JobStatusFailed
	}, 5*time.Second, 10*time.Millisecond)
	return job
}

func TestCreateInstanceAsync_SameKeyReplaysSameJob(t *testing.T) {
	handler, storage, mock := testJobHandler(t)

	code1, resp1 := postAsyncJob(t, handler, "idem-replay", "key-replay-1")
	assert.Equal(t, http.StatusAccepted, code1)

	code2, resp2 := postAsyncJob(t, handler, "idem-replay", "key-replay-1")
	assert.Equal(t, http.StatusOK, code2)
	assert.Equal(t, resp1.Job.ID, resp2.Job.ID)

	job := waitForJobSettled(t, storage, resp1.Job.ID)
	assert.Equal(t, models.JobStatusCompleted, job.Status)

	instances, err := mock.ListInstances()
	require.NoError(t, err)
	assert.Len(t, instances, 1, "replay must not launch a second instance")
}

func TestCreateInstanceAsync_DifferentKeysCreateDistinctJobs(t *testing.T) {
	handler, storage, _ := testJobHandler(t)

	code1, resp1 := postAsyncJob(t, handler, "idem-a", "key-a")
	code2, resp2 := postAsyncJob(t, handler, "idem-b", "key-b")

	assert.Equal(t, http.StatusAccepted, code1)
	assert.Equal(t, http.StatusAccepted, code2)
	assert.NotEqual(t, resp1.Job.ID, resp2.Job.ID)

	// Wait for both background workers so their final storage writes land
	// before TempDir cleanup (otherwise cleanup races the writers).
	waitForJobSettled(t, storage, resp1.Job.ID)
	waitForJobSettled(t, storage, resp2.Job.ID)
}

func TestCreateInstanceAsync_NoKeyPreservesLegacyBehavior(t *testing.T) {
	handler, storage, _ := testJobHandler(t)

	code1, resp1 := postAsyncJob(t, handler, "idem-plain-1", "")
	code2, resp2 := postAsyncJob(t, handler, "idem-plain-2", "")

	assert.Equal(t, http.StatusAccepted, code1)
	assert.Equal(t, http.StatusAccepted, code2)
	assert.NotEqual(t, resp1.Job.ID, resp2.Job.ID)

	// See above: settle background workers before TempDir cleanup.
	waitForJobSettled(t, storage, resp1.Job.ID)
	waitForJobSettled(t, storage, resp2.Job.ID)
}

func TestCreateInstanceAsync_RejectsInvalidKeys(t *testing.T) {
	handler, _, _ := testJobHandler(t)

	for _, key := range []string{"has space", "semi;colon", strings.Repeat("k", 129)} {
		e := echo.New()
		body := `{"name":"idem-badkey"}`
		req := httptest.NewRequest(http.MethodPost, "/api/instances/async", strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		req.Header.Set(IdempotencyKeyHeader, key)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		require.NoError(t, handler.CreateInstanceAsync(c))
		assert.Equal(t, http.StatusBadRequest, rec.Code, "key %q", key)
		assert.Contains(t, rec.Body.String(), "invalid_request")
	}
}

func TestCreateInstanceAsync_ConcurrentSameKeySingleFlight(t *testing.T) {
	handler, storage, mock := testJobHandler(t)

	const callers = 8
	var mu sync.Mutex
	ids := make([]string, 0, callers)
	codes := make([]int, 0, callers)
	errs := make([]error, 0, callers)

	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, resp, err := doPostAsync(handler, "idem-conc", "key-conc")
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			ids = append(ids, resp.Job.ID)
			codes = append(codes, code)
		}()
	}
	wg.Wait()

	assert.Empty(t, errs)
	require.Len(t, ids, callers)

	// Exactly one request creates (202); the rest replay (200) — all agree.
	created, replayed := 0, 0
	for _, code := range codes {
		switch code {
		case http.StatusAccepted:
			created++
		case http.StatusOK:
			replayed++
		}
	}
	assert.Equal(t, 1, created)
	assert.Equal(t, callers-1, replayed)
	for _, id := range ids[1:] {
		assert.Equal(t, ids[0], id)
	}

	job := waitForJobSettled(t, storage, ids[0])
	assert.Equal(t, models.JobStatusCompleted, job.Status)

	instances, err := mock.ListInstances()
	require.NoError(t, err)
	assert.Len(t, instances, 1, "concurrent replays must launch exactly once")
}

func TestCreateInstanceAsync_ReplayReturnsFailedJob(t *testing.T) {
	handler, storage, _ := testJobHandler(t)

	failed := &models.Job{
		ID:             "failed-job-1",
		Type:           "create_instance",
		Status:         models.JobStatusFailed,
		InstanceName:   "idem-failed",
		Error:          "boom",
		IdempotencyKey: "key-failed",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	storage.Set(failed)

	// Same key after failure returns the recorded failure; callers that want
	// a fresh attempt must use a new key.
	_, resp := postAsyncJob(t, handler, "idem-failed", "key-failed")
	assert.Equal(t, "failed-job-1", resp.Job.ID)
	assert.Equal(t, models.JobStatusFailed, resp.Job.Status)
	assert.Equal(t, "boom", resp.Job.Error)
}

func TestJobStorage_IdempotencyIndexSurvivesReload(t *testing.T) {
	dir := t.TempDir()
	storage, err := NewJobStorage(dir)
	require.NoError(t, err)

	storage.Set(&models.Job{
		ID:             "persist-1",
		Type:           "create_instance",
		Status:         models.JobStatusRunning,
		IdempotencyKey: "key-persist",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	})

	reloaded, err := NewJobStorage(dir)
	require.NoError(t, err)

	job, ok := reloaded.GetByIdempotencyKey("key-persist")
	require.True(t, ok)
	assert.Equal(t, "persist-1", job.ID)

	reloaded.Delete("persist-1")
	_, ok = reloaded.GetByIdempotencyKey("key-persist")
	assert.False(t, ok, "index entry must die with its job")
}

func TestJobStorage_CleanupPurgesIdempotencyIndex(t *testing.T) {
	storage, err := NewJobStorage(t.TempDir())
	require.NoError(t, err)

	old := time.Now().Add(-48 * time.Hour)
	storage.Set(&models.Job{
		ID:             "old-1",
		Type:           "create_instance",
		Status:         models.JobStatusCompleted,
		IdempotencyKey: "key-old",
		CreatedAt:      old,
		UpdatedAt:      old,
	})

	assert.Equal(t, 1, storage.Cleanup(24*time.Hour))
	_, ok := storage.GetByIdempotencyKey("key-old")
	assert.False(t, ok, "expired jobs must release their keys")
}
