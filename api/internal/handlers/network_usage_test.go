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

func mustNetworkUsage(t *testing.T) *NetworkUsageStore {
	t.Helper()
	store, err := NewNetworkUsageStore(t.TempDir())
	require.NoError(t, err)
	return store
}

func TestNetworkUsageStore_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	store, err := NewNetworkUsageStore(dir)
	require.NoError(t, err)

	store.RecordInstanceNetwork("web-1", "br01")
	store.RecordInstanceNetwork("", "br01")
	store.RecordInstanceNetwork("web-2", "")
	store.RecordCreatedNetwork("br01")
	store.RecordCreatedNetwork("")

	network, ok := store.InstanceNetwork("web-1")
	assert.True(t, ok)
	assert.Equal(t, "br01", network)
	_, ok = store.InstanceNetwork("web-2")
	assert.False(t, ok)
	assert.True(t, store.IsManaged("br01"))
	assert.False(t, store.IsManaged("docker0"))

	reloaded, err := NewNetworkUsageStore(dir)
	require.NoError(t, err)
	network, ok = reloaded.InstanceNetwork("web-1")
	assert.True(t, ok)
	assert.Equal(t, "br01", network)
	assert.True(t, reloaded.IsManaged("br01"))

	reloaded.ClearInstanceNetwork("web-1")
	_, ok = reloaded.InstanceNetwork("web-1")
	assert.False(t, ok)
	reloaded.RemoveCreatedNetwork("br01")
	assert.False(t, reloaded.IsManaged("br01"))
}

func TestNetworkUsageStore_PruneManaged(t *testing.T) {
	store := mustNetworkUsage(t)
	store.RecordCreatedNetwork("br01")
	store.RecordCreatedNetwork("gone")

	store.PruneManaged([]string{"br01", "docker0"})
	assert.True(t, store.IsManaged("br01"))
	assert.False(t, store.IsManaged("gone"))
}

func TestNetworkUsageStore_UsageReport(t *testing.T) {
	store := mustNetworkUsage(t)
	store.RecordInstanceNetwork("web-1", "br01")
	store.RecordInstanceNetwork("web-2", "br01")

	instances := []models.Instance{
		{Name: "web-1", State: "Running", IPv4: []string{"10.0.0.1", "192.168.1.10"}},
		{Name: "web-2", State: "Stopped"},
		// Default-only interface: safe to ignore.
		{Name: "plain", State: "Running", IPv4: []string{"10.0.0.2"}},
		// Extra interface, untracked: unknown.
		{Name: "mystery", State: "Running", IPv4: []string{"10.0.0.3", "192.168.1.11"}},
		// Stopped with empty info: unknown.
		{Name: "old", State: "Stopped"},
	}

	usedBy, unverified := store.UsageReport(instances)
	assert.Equal(t, map[string][]string{"br01": {"web-1", "web-2"}}, usedBy)
	assert.Equal(t, []string{"mystery", "old"}, unverified)
}

func TestNetworkHandler_Delete_InUse(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{
		{Name: "web-1", State: "Running", IPv4: []string{"10.0.0.1", "192.168.1.10"}},
	})
	store := mustNetworkUsage(t)
	store.RecordInstanceNetwork("web-1", "br01")

	e := echo.New()
	handler := NewNetworkHandler(mockClient, store)
	e.DELETE("/networks/:name", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/networks/br01", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusConflict, rec.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "network_in_use", body["error"])
	assert.Contains(t, rec.Body.String(), "web-1")
}

func TestNetworkHandler_Delete_Unused(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{
		{Name: "plain", State: "Running", IPv4: []string{"10.0.0.1"}},
	})
	store := mustNetworkUsage(t)
	store.RecordCreatedNetwork("br01")

	e := echo.New()
	handler := NewNetworkHandler(mockClient, store)
	e.DELETE("/networks/:name", handler.Delete)

	req := httptest.NewRequest(http.MethodDelete, "/networks/br01", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, store.IsManaged("br01"))
}

func TestNetworkHandler_List_Enrichment(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetNetworks([]models.Network{
		{Name: "br01", Type: "bridge"},
		{Name: "docker0", Type: "bridge"},
	})
	mockClient.SetInstances([]models.Instance{
		{Name: "web-1", State: "Running", IPv4: []string{"10.0.0.1", "192.168.1.10"}},
		{Name: "old", State: "Stopped"},
	})
	store := mustNetworkUsage(t)
	store.RecordInstanceNetwork("web-1", "br01")

	e := echo.New()
	handler := NewNetworkHandler(mockClient, store)
	e.GET("/networks", handler.List)

	req := httptest.NewRequest(http.MethodGet, "/networks", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var list models.NetworkList
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.Len(t, list.Networks, 2)
	assert.Equal(t, []string{"web-1"}, list.Networks[0].UsedBy)
	assert.Empty(t, list.Networks[1].UsedBy)
	assert.Equal(t, []string{"old"}, list.UnverifiedInstances)
}

func TestNetworkHandler_ClaimUnclaim(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetNetworks([]models.Network{{Name: "br01", Type: "bridge"}})
	store := mustNetworkUsage(t)

	e := echo.New()
	handler := NewNetworkHandler(mockClient, store)
	e.POST("/networks/:name/claim", handler.ClaimNetwork)
	e.DELETE("/networks/:name/claim", handler.UnclaimNetwork)

	req := httptest.NewRequest(http.MethodPost, "/networks/br01/claim", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, store.IsManaged("br01"))

	req = httptest.NewRequest(http.MethodPost, "/networks/nope/claim", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	req = httptest.NewRequest(http.MethodDelete, "/networks/br01/claim", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.False(t, store.IsManaged("br01"))
}

func TestInstanceHandler_InstanceNetwork(t *testing.T) {
	mockClient := multipass.NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "web-1", State: "Running"}})
	store := mustNetworkUsage(t)
	handler := NewInstanceHandler(mockClient, testConfig(), store)

	e := echo.New()
	e.GET("/instances/:name/network", handler.GetInstanceNetwork)
	e.POST("/instances/:name/network", handler.SetInstanceNetwork)

	// Unknown before attribution.
	req := httptest.NewRequest(http.MethodGet, "/instances/web-1/network", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"verified":false`)

	// Attributing a missing instance is a 404.
	req = httptest.NewRequest(http.MethodPost, "/instances/ghost/network",
		strings.NewReader(`{"network":"br01"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusNotFound, rec.Code)

	// Attribute, verify, then clear.
	req = httptest.NewRequest(http.MethodPost, "/instances/web-1/network",
		strings.NewReader(`{"network":"br01"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"verified":true`)

	req = httptest.NewRequest(http.MethodPost, "/instances/web-1/network",
		strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"verified":false`)
}

func TestInstanceHandler_CreateDelete_NetworkTracking(t *testing.T) {
	mockClient := multipass.NewMockClient()
	store := mustNetworkUsage(t)
	handler := NewInstanceHandler(mockClient, testConfig(), store)

	e := echo.New()
	e.POST("/instances", handler.Create)
	e.DELETE("/instances/:name", handler.Delete)

	req := httptest.NewRequest(http.MethodPost, "/instances",
		strings.NewReader(`{"name":"web-1","network":"br01"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)
	network, ok := store.InstanceNetwork("web-1")
	assert.True(t, ok)
	assert.Equal(t, "br01", network)

	req = httptest.NewRequest(http.MethodDelete, "/instances/web-1", nil)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	_, ok = store.InstanceNetwork("web-1")
	assert.False(t, ok)
}
