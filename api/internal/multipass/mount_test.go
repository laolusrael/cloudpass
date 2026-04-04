package multipass

import (
	"testing"

	"cloudpass/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestMountInstance_SourcePathNotExist(t *testing.T) {
	client := NewClient(30)

	err := client.MountInstance("test-vm", "/nonexistent/path", "/home/ubuntu/mount")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestEnsureTargetPath(t *testing.T) {
	mockClient := NewMockClient()

	err := mockClient.ensureTargetPath("test-vm", "/home/ubuntu/testpath")
	assert.NoError(t, err)
}

func TestMountInstance_Success(t *testing.T) {
	mockClient := NewMockClient()
	mockClient.SetInstances([]models.Instance{{Name: "test-vm", State: "Running"}})

	err := mockClient.MountInstance("test-vm", "/tmp", "/home/ubuntu/mount")
	assert.NoError(t, err)
}
