package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstance_JSONSerialization(t *testing.T) {
	instance := Instance{
		Name:    "test-vm",
		State:   "Running",
		IPv4:    []string{"192.168.1.100"},
		IPv6:    []string{"fd00::100"},
		CPU:     4,
		Memory:  "8G",
		Disk:    "50G",
		Image:   "ubuntu:22.04",
		Release: "Ubuntu 22.04 LTS",
		Mounts: []Mount{
			{Source: "/home/user", Target: "/mnt"},
		},
		Network: map[string]NetworkInfo{
			"eth0": {IPv4: "192.168.1.100"},
		},
	}

	data, err := json.Marshal(instance)
	require.NoError(t, err)

	var decoded Instance
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, instance.Name, decoded.Name)
	assert.Equal(t, instance.State, decoded.State)
	assert.Equal(t, instance.IPv4, decoded.IPv4)
	assert.Equal(t, instance.CPU, decoded.CPU)
}

func TestCreateInstanceRequest_Validation(t *testing.T) {
	tests := []struct {
		name    string
		request CreateInstanceRequest
		valid   bool
	}{
		{
			name: "valid with all fields",
			request: CreateInstanceRequest{
				Name:      "my-vm",
				Image:     "22.04",
				CPUs:      4,
				Memory:    "8G",
				Disk:      "50G",
				Network:   "bridge",
				CloudInit: "#cloud-config",
			},
			valid: true,
		},
		{
			name: "valid minimal",
			request: CreateInstanceRequest{
				Name: "minimal-vm",
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.request)
			require.NoError(t, err)

			var decoded CreateInstanceRequest
			err = json.Unmarshal(data, &decoded)
			require.NoError(t, err)

			assert.Equal(t, tt.request.Name, decoded.Name)
		})
	}
}

func TestNetwork_JSON(t *testing.T) {
	network := Network{
		Name:        "br0",
		Type:        "bridge",
		IPv4:        "192.168.100.1/24",
		Description: "Custom bridge network",
	}

	data, err := json.Marshal(network)
	require.NoError(t, err)

	var decoded Network
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "br0", decoded.Name)
	assert.Equal(t, "bridge", decoded.Type)
	assert.Equal(t, "192.168.100.1/24", decoded.IPv4)
}

func TestHealthResponse_JSON(t *testing.T) {
	now := time.Now()
	health := HealthResponse{
		Status:    "healthy",
		Timestamp: now,
	}

	data, err := json.Marshal(health)
	require.NoError(t, err)

	var decoded HealthResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "healthy", decoded.Status)
}

func TestErrorResponse_JSON(t *testing.T) {
	errResp := ErrorResponse{
		Error:   "validation_error",
		Message: "name is required",
	}

	data, err := json.Marshal(errResp)
	require.NoError(t, err)

	var decoded ErrorResponse
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "validation_error", decoded.Error)
	assert.Equal(t, "name is required", decoded.Message)
}

func TestCreateNetworkRequest_JSON(t *testing.T) {
	req := CreateNetworkRequest{
		Name: "custom-net",
		Mode: "manual",
		MAC:  "aa:bb:cc:dd:ee:ff",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var decoded CreateNetworkRequest
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "custom-net", decoded.Name)
	assert.Equal(t, "manual", decoded.Mode)
	assert.Equal(t, "aa:bb:cc:dd:ee:ff", decoded.MAC)
}

func TestInstanceList_JSON(t *testing.T) {
	list := InstanceList{
		Instances: []Instance{
			{Name: "vm1", State: "Running"},
			{Name: "vm2", State: "Stopped"},
		},
	}

	data, err := json.Marshal(list)
	require.NoError(t, err)

	var decoded InstanceList
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Instances, 2)
	assert.Equal(t, "vm1", decoded.Instances[0].Name)
	assert.Equal(t, "vm2", decoded.Instances[1].Name)
}

func TestNetworkList_JSON(t *testing.T) {
	list := NetworkList{
		Networks: []Network{
			{Name: "net1", Type: "bridge"},
			{Name: "net2", Type: "dhcp"},
		},
	}

	data, err := json.Marshal(list)
	require.NoError(t, err)

	var decoded NetworkList
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Networks, 2)
}

func TestImage_JSON(t *testing.T) {
	image := Image{
		Alias:   "22.04",
		Version: "22.04.3",
		Release: "Ubuntu 22.04.3 LTS",
		Remote:  "ubuntu",
	}

	data, err := json.Marshal(image)
	require.NoError(t, err)

	var decoded Image
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "22.04", decoded.Alias)
	assert.Equal(t, "ubuntu", decoded.Remote)
}

func TestMount_JSON(t *testing.T) {
	mount := Mount{
		Source: "/home/data",
		Target: "/mnt/data",
	}

	data, err := json.Marshal(mount)
	require.NoError(t, err)

	var decoded Mount
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "/home/data", decoded.Source)
	assert.Equal(t, "/mnt/data", decoded.Target)
}
