package multipass

import (
	"os"
	"path/filepath"
	"testing"

	"cloudpass/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMountInstance_SourcePathNotExist(t *testing.T) {
	client := NewClient(30)

	// Anchored at the temp dir so the path is absolute on every OS.
	missing := filepath.Join(os.TempDir(), "nonexistent-cloudpass-xyz")
	err := client.MountInstance("test-vm", missing, "/home/ubuntu/mount", MountOptions{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestMountInstance_SourcePathHandling(t *testing.T) {
	client := NewClient(30)

	tests := []struct {
		name   string
		source string
		want   string
	}{
		{"tilde rejected", "~/projects", "must be absolute"},
		{"relative rejected", "projects", "must be absolute"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := client.MountInstance("test-vm", tt.source, "/home/ubuntu/mount", MountOptions{})
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}

	t.Run("file rejected", func(t *testing.T) {
		f, err := os.CreateTemp("", "mount-src-*")
		require.NoError(t, err)
		name := f.Name()
		require.NoError(t, f.Close())
		t.Cleanup(func() { _ = os.Remove(name) })

		err = client.MountInstance("test-vm", name, "/home/ubuntu/mount", MountOptions{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "is not a directory")
	})
}

func TestMountInstance_InvalidOptions(t *testing.T) {
	client := NewClient(30)
	dir := t.TempDir()

	err := client.MountInstance("test-vm", dir, "/home/ubuntu/mount", MountOptions{Type: "smb"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be classic or native")

	err = client.MountInstance("test-vm", dir, "/home/ubuntu/mount", MountOptions{UIDMap: "abc"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host:instance")

	err = client.MountInstance("test-vm", dir, "/home/ubuntu/mount", MountOptions{GIDMap: "1000"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "host:instance")
}

func TestEnsureTargetPath(t *testing.T) {
	mockClient := NewMockClient()

	err := mockClient.ensureTargetPath("test-vm", "/home/ubuntu/testpath")
	assert.NoError(t, err)
}

func TestMountInstance_Success(t *testing.T) {
	mockClient := NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	err := mockClient.MountInstance("test-vm", "/tmp", "/home/ubuntu/mount", MountOptions{})
	assert.NoError(t, err)
}
