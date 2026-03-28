package multipass

import (
	"os"
	"path/filepath"
	"testing"

	"cloudpass/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetKnownHostsPath(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	defer func() {
		if origHome != "" {
			os.Setenv("HOME", origHome)
		}
		if origUserProfile != "" {
			os.Setenv("USERPROFILE", origUserProfile)
		}
	}()

	os.Setenv("HOME", tmpDir)
	os.Setenv("USERPROFILE", tmpDir)

	path, err := getKnownHostsPath()
	require.NoError(t, err)

	expected := filepath.Join(tmpDir, sshKeyCacheDir, knownHostsFile)
	assert.Equal(t, expected, path)

	_, err = os.Stat(filepath.Dir(path))
	assert.NoError(t, err)
}

func TestGetKnownHostsPath_NoHome(t *testing.T) {
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	defer func() {
		if origHome != "" {
			os.Setenv("HOME", origHome)
		}
		if origUserProfile != "" {
			os.Setenv("USERPROFILE", origUserProfile)
		}
	}()

	os.Unsetenv("HOME")
	os.Unsetenv("USERPROFILE")

	_, err := getKnownHostsPath()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot determine home directory")
}

func TestGetInstanceIP_Success(t *testing.T) {
	mockClient := NewMockClient()
	instances := []models.Instance{
		{Name: "test-vm", State: "Running", IPv4: []string{"192.168.1.100"}, IPv6: []string{"fd00::100"}},
	}
	mockClient.SetInstances(instances)

	ip, err := mockClient.GetInstanceIP("test-vm")
	assert.NoError(t, err)
	assert.Equal(t, "192.168.1.100", ip)
}

func TestGetInstanceIP_NoIP(t *testing.T) {
	mockClient := NewMockClient()
	instances := []models.Instance{
		{Name: "test-vm", State: "Running", IPv4: []string{}},
	}
	mockClient.SetInstances(instances)

	ip, err := mockClient.GetInstanceIP("test-vm")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no IPv4 address")
	assert.Empty(t, ip)
}

func TestGetInstanceIP_NotFound(t *testing.T) {
	mockClient := NewMockClient()
	mockClient.SetInstances([]models.Instance{})

	ip, err := mockClient.GetInstanceIP("nonexistent")
	assert.Error(t, err)
	assert.Empty(t, ip)
}

func TestGetInstanceIP_MultipleIPv4(t *testing.T) {
	mockClient := NewMockClient()
	instances := []models.Instance{
		{Name: "test-vm", State: "Running", IPv4: []string{"192.168.1.100", "10.0.0.50"}},
	}
	mockClient.SetInstances(instances)

	ip, err := mockClient.GetInstanceIP("test-vm")
	assert.NoError(t, err)
	assert.Equal(t, "192.168.1.100", ip)
}
