package multipass

import (
	"testing"
	"time"

	"cloudpass/internal/models"

	"github.com/stretchr/testify/assert"
)

func TestGetInstanceResources_Success(t *testing.T) {
	client := &multipassClient{timeout: 30 * time.Second}

	resources, err := client.GetInstanceResources("test-vm")

	assert.NoError(t, err)
	assert.NotNil(t, resources)
}

func TestGetInstanceResources_NotFound(t *testing.T) {
	client := &multipassClient{timeout: 30 * time.Second}

	resources, err := client.GetInstanceResources("nonexistent-vm")

	assert.NoError(t, err)
	assert.NotNil(t, resources)
}

func TestMockClient_GetInstanceResources(t *testing.T) {
	mockClient := NewMockClient()
	mockClient.SetInstances([]models.Instance{
		{Name: "test-vm", CPU: 4, Memory: "4G", Disk: "20G"},
	})

	resources, err := mockClient.GetInstanceResources("test-vm")

	assert.NoError(t, err)
	assert.NotNil(t, resources)
	assert.Equal(t, 4, resources.CPUs)
	assert.Equal(t, "4G", resources.Memory)
	assert.Equal(t, "20G", resources.Disk)
}

func TestMockClient_GetInstanceResources_NotFound(t *testing.T) {
	mockClient := NewMockClient()
	mockClient.SetInstances([]models.Instance{
		{Name: "other-vm", CPU: 2, Memory: "2G", Disk: "10G"},
	})

	resources, err := mockClient.GetInstanceResources("nonexistent")

	assert.Error(t, err)
	assert.Nil(t, resources)
	assert.Contains(t, err.Error(), "not found")
}
