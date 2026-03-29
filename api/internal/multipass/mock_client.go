package multipass

import (
	"fmt"

	"cloudpass/internal/models"
)

type MockClient struct {
	instances        []models.Instance
	images           []models.Image
	networks         []models.Network
	getInstanceErr   error
	listImagesErr    error
	listNetworksErr  error
	createNetworkErr error
	deleteNetworkErr error
	suspendErr       error
	resumeErr        error
}

func (m *MockClient) SetListImagesErr(err error) {
	m.listImagesErr = err
}

func NewMockClient() *MockClient {
	return &MockClient{
		instances: []models.Instance{},
		images:    []models.Image{},
		networks:  []models.Network{},
	}
}

func (m *MockClient) ListInstances() ([]models.Instance, error) {
	return m.instances, nil
}

func (m *MockClient) GetInstance(name string) (*models.Instance, error) {
	if m.getInstanceErr != nil {
		return nil, m.getInstanceErr
	}
	for _, inst := range m.instances {
		if inst.Name == name {
			return &inst, nil
		}
	}
	return nil, nil
}

func (m *MockClient) GetInstanceIP(name string) (string, error) {
	inst, err := m.GetInstance(name)
	if err != nil {
		return "", err
	}
	if inst == nil {
		return "", fmt.Errorf("instance %q not found", name)
	}
	if len(inst.IPv4) == 0 {
		return "", fmt.Errorf("instance has no IPv4 address")
	}
	return inst.IPv4[0], nil
}

func (m *MockClient) CreateInstance(opts models.CreateInstanceRequest) (*models.Instance, error) {
	inst := models.Instance{Name: opts.Name}
	m.instances = append(m.instances, inst)
	return &inst, nil
}

func (m *MockClient) StartInstance(name string) error {
	return nil
}

func (m *MockClient) StopInstance(name string) error {
	return nil
}

func (m *MockClient) RestartInstance(name string) error {
	return nil
}

func (m *MockClient) SuspendInstance(name string) error {
	if m.suspendErr != nil {
		return m.suspendErr
	}
	return nil
}

func (m *MockClient) ResumeInstance(name string) error {
	if m.resumeErr != nil {
		return m.resumeErr
	}
	return nil
}

func (m *MockClient) DeleteInstance(name string) error {
	return nil
}

func (m *MockClient) ListImages() ([]models.Image, error) {
	if m.listImagesErr != nil {
		return nil, m.listImagesErr
	}
	return m.images, nil
}

func (m *MockClient) ListNetworks() ([]models.Network, error) {
	if m.listNetworksErr != nil {
		return nil, m.listNetworksErr
	}
	return m.networks, nil
}

func (m *MockClient) CreateNetwork(name string, mode string, mac string) error {
	if m.createNetworkErr != nil {
		return m.createNetworkErr
	}
	m.networks = append(m.networks, models.Network{Name: name})
	return nil
}

func (m *MockClient) DeleteNetwork(name string) error {
	if m.deleteNetworkErr != nil {
		return m.deleteNetworkErr
	}
	return nil
}

func (m *MockClient) SetInstances(instances []models.Instance) {
	m.instances = instances
}

func (m *MockClient) SetImages(images []models.Image) {
	m.images = images
}

func (m *MockClient) SetNetworks(networks []models.Network) {
	m.networks = networks
}

func (m *MockClient) SetGetInstanceErr(err error) {
	m.getInstanceErr = err
}

func (m *MockClient) SetSuspendErr(err error) {
	m.suspendErr = err
}

func (m *MockClient) SetResumeErr(err error) {
	m.resumeErr = err
}

func (m *MockClient) SetCreateNetworkErr(err error) {
	m.createNetworkErr = err
}

func (m *MockClient) SetDeleteNetworkErr(err error) {
	m.deleteNetworkErr = err
}

func (m *MockClient) SetListNetworksErr(err error) {
	m.listNetworksErr = err
}

func (m *MockClient) MountInstance(instanceName string, sourcePath string, targetPath string) error {
	return nil
}

func (m *MockClient) UnmountInstance(instanceName string, targetPath string) error {
	return nil
}

func (m *MockClient) PurgeDeleted() error {
	return nil
}

func (m *MockClient) CreateSnapshot(instanceName string, snapshotName string, comment string) error {
	return nil
}

func (m *MockClient) RestoreSnapshot(instanceName string, snapshotName string) error {
	return nil
}

func (m *MockClient) ListSnapshots(instanceName string) ([]models.Snapshot, error) {
	return []models.Snapshot{
		{
			Name:      "snap1",
			Instance:  instanceName,
			CreatedAt: "2024-01-01T00:00:00Z",
		},
	}, nil
}

func (m *MockClient) DeleteSnapshot(instanceName string, snapshotName string) error {
	return nil
}

func (m *MockClient) ExportInstance(instanceName string, outputPath string) (string, error) {
	if outputPath == "" {
		outputPath = "./" + instanceName + ".img"
	}
	return outputPath, nil
}

func (m *MockClient) ImportInstance(imagePath string, name string, cpus int, memory string, disk string) (*models.Instance, error) {
	inst := models.Instance{Name: name}
	m.instances = append(m.instances, inst)
	return &inst, nil
}
