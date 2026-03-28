package multipass

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"cloudpass/internal/models"
)

type Client interface {
	ListInstances() ([]models.Instance, error)
	GetInstance(name string) (*models.Instance, error)
	GetInstanceIP(name string) (string, error)
	CreateInstance(opts models.CreateInstanceRequest) (*models.Instance, error)
	StartInstance(name string) error
	StopInstance(name string) error
	RestartInstance(name string) error
	SuspendInstance(name string) error
	ResumeInstance(name string) error
	DeleteInstance(name string) error
	PurgeDeleted() error
	ListImages() ([]models.Image, error)
	ListNetworks() ([]models.Network, error)
	CreateNetwork(name string, mode string, mac string) error
	DeleteNetwork(name string) error
	CreateSnapshot(instanceName string, snapshotName string, comment string) error
	RestoreSnapshot(instanceName string, snapshotName string) error
	ListSnapshots(instanceName string) ([]models.Snapshot, error)
	DeleteSnapshot(instanceName string, snapshotName string) error
	ExportInstance(instanceName string, outputPath string) (string, error)
	ImportInstance(imagePath string, name string, cpus int, memory string, disk string) (*models.Instance, error)
}

type multipassClient struct {
	timeout time.Duration
}

func NewClient(timeoutSec int) Client {
	return &multipassClient{
		timeout: time.Duration(timeoutSec) * time.Second,
	}
}

func (c *multipassClient) ListInstances() ([]models.Instance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	var result struct {
		List []struct {
			Name  string `json:"name"`
			State string `json:"state"`
		} `json:"list"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse list output: %w", err)
	}

	instances := make([]models.Instance, 0, len(result.List))
	for _, item := range result.List {
		instances = append(instances, models.Instance{
			Name:  item.Name,
			State: item.State,
		})
	}

	return instances, nil
}

func (c *multipassClient) GetInstance(name string) (*models.Instance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "info", name, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return nil, fmt.Errorf("instance %q not found", name)
		}
		return nil, fmt.Errorf("failed to get instance info: %w", err)
	}

	var info struct {
		Info []struct {
			Name      string   `json:"name"`
			State     string   `json:"state"`
			IPv4      []string `json:"ipv4"`
			IPv6      []string `json:"ipv6"`
			CPUs      int      `json:"cpus"`
			Memory    string   `json:"memory"`
			DiskSpace string   `json:"disk_space"`
			Image     string   `json:"image"`
			Release   string   `json:"release"`
			Mounts    []struct {
				SourcePath string `json:"source_path"`
				TargetPath string `json:"target_path"`
			} `json:"mounts"`
			Load    []float64 `json:"load"`
			Network map[string]struct {
				IPv4 string `json:"ipv4"`
				IPv6 string `json:"ipv6"`
			} `json:"network"`
		} `json:"info"`
	}

	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse info output: %w", err)
	}

	if len(info.Info) == 0 {
		return nil, fmt.Errorf("instance %q not found", name)
	}

	i := info.Info[0]
	instance := &models.Instance{
		Name:    i.Name,
		State:   i.State,
		IPv4:    i.IPv4,
		IPv6:    i.IPv6,
		CPU:     i.CPUs,
		Memory:  i.Memory,
		Disk:    i.DiskSpace,
		Image:   i.Image,
		Release: i.Release,
		Load:    i.Load,
		Mounts:  make([]models.Mount, 0),
		Network: make(map[string]models.NetworkInfo),
	}

	for _, m := range i.Mounts {
		instance.Mounts = append(instance.Mounts, models.Mount{
			Source: m.SourcePath,
			Target: m.TargetPath,
		})
	}

	for name, net := range i.Network {
		instance.Network[name] = models.NetworkInfo{
			IPv4: net.IPv4,
			IPv6: net.IPv6,
		}
	}

	return instance, nil
}

func (c *multipassClient) CreateInstance(opts models.CreateInstanceRequest) (*models.Instance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	args := []string{"launch"}

	if opts.Name != "" {
		args = append(args, "-n", opts.Name)
	}
	if opts.CPUs > 0 {
		args = append(args, "-c", fmt.Sprintf("%d", opts.CPUs))
	}
	if opts.Memory != "" {
		args = append(args, "-m", opts.Memory)
	}
	if opts.Disk != "" {
		args = append(args, "-d", opts.Disk)
	}
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}
	if opts.CloudInit != "" {
		args = append(args, "--cloud-init", "-")
	}
	if opts.Image != "" {
		args = append(args, opts.Image)
	}

	cmd := exec.CommandContext(ctx, "multipass", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "Launched:") {
		parts := strings.Split(outputStr, ":")
		if len(parts) >= 2 {
			name := strings.TrimSpace(parts[1])
			return c.GetInstance(name)
		}
	}

	return c.GetInstance(opts.Name)
}

func (c *multipassClient) StartInstance(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "start", name)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to start instance: %w", err)
	}

	return nil
}

func (c *multipassClient) StopInstance(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "stop", name)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	return nil
}

func (c *multipassClient) RestartInstance(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "restart", name)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to restart instance: %w", err)
	}

	return nil
}

func (c *multipassClient) DeleteInstance(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "delete", name)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	return nil
}

func (c *multipassClient) SuspendInstance(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "suspend", name)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to suspend instance: %w", err)
	}

	return nil
}

func (c *multipassClient) ResumeInstance(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "start", name)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to resume instance: %w", err)
	}

	return nil
}

func (c *multipassClient) CreateNetwork(name string, mode string, mac string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	args := []string{"networks", "create"}
	if mode != "" {
		args = append(args, "--mode", mode)
	}
	if mac != "" {
		args = append(args, "--mac", mac)
	}
	args = append(args, name)

	cmd := exec.CommandContext(ctx, "multipass", args...)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	return nil
}

func (c *multipassClient) DeleteNetwork(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "networks", "delete", name)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to delete network: %w", err)
	}

	return nil
}

func (c *multipassClient) ListImages() ([]models.Image, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "find", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	var images []models.Image
	if err := json.Unmarshal(output, &images); err != nil {
		return nil, fmt.Errorf("failed to parse images: %w", err)
	}

	return images, nil
}

func (c *multipassClient) ListNetworks() ([]models.Network, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "networks", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	var networks []models.Network
	if err := json.Unmarshal(output, &networks); err != nil {
		return nil, fmt.Errorf("failed to parse networks: %w", err)
	}

	return networks, nil
}

func (c *multipassClient) PurgeDeleted() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "purge")
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to purge: %w", err)
	}

	return nil
}

func (c *multipassClient) CreateSnapshot(instanceName string, snapshotName string, comment string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	args := []string{"snapshot", instanceName}
	if snapshotName != "" {
		args = append(args, snapshotName)
	}

	cmd := exec.CommandContext(ctx, "multipass", args...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	if comment != "" && strings.Contains(string(output), "comment") {
		ctx2, cancel2 := context.WithTimeout(context.Background(), c.timeout)
		defer cancel2()
		exec.CommandContext(ctx2, "multipass", "set", fmt.Sprintf("local.%s.%s.comment", instanceName, snapshotName), comment).Output()
	}

	return nil
}

func (c *multipassClient) RestoreSnapshot(instanceName string, snapshotName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout*2)
	defer cancel()

	args := []string{"restore", instanceName}
	if snapshotName != "" {
		args = append(args, snapshotName)
	}

	cmd := exec.CommandContext(ctx, "multipass", args...)
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to restore snapshot: %w", err)
	}

	return nil
}

func (c *multipassClient) ListSnapshots(instanceName string) ([]models.Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "info", instanceName, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return nil, fmt.Errorf("instance %q not found", instanceName)
		}
		return nil, fmt.Errorf("failed to get instance info: %w", err)
	}

	var info struct {
		Info []struct {
			Snapshots []struct {
				Name       string `json:"name"`
				Created    string `json:"created"`
				Comment    string `json:"comment"`
				Parent     string `json:"parent"`
				Children   int    `json:"children"`
				StateSize  int64  `json:"state_size"`
				DiskSize   int64  `json:"disk_size"`
				MemorySize int64  `json:"memory_size"`
			} `json:"snapshots"`
		} `json:"info"`
	}

	if err := json.Unmarshal(output, &info); err != nil {
		return nil, fmt.Errorf("failed to parse info output: %w", err)
	}

	if len(info.Info) == 0 {
		return nil, fmt.Errorf("instance %q not found", instanceName)
	}

	snapshots := make([]models.Snapshot, 0, len(info.Info[0].Snapshots))
	for _, s := range info.Info[0].Snapshots {
		snapshots = append(snapshots, models.Snapshot{
			Name:       s.Name,
			Instance:   instanceName,
			CreatedAt:  s.Created,
			Comment:    s.Comment,
			Parent:     s.Parent,
			Children:   s.Children,
			StateSize:  s.StateSize,
			DiskSize:   s.DiskSize,
			MemorySize: s.MemorySize,
		})
	}

	return snapshots, nil
}

func (c *multipassClient) DeleteSnapshot(instanceName string, snapshotName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "delete", instanceName, "-p")
	_, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	return nil
}

func (c *multipassClient) ExportInstance(instanceName string, outputPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout*3)
	defer cancel()

	instance, err := c.GetInstance(instanceName)
	if err != nil {
		return "", err
	}

	if instance.State != "Stopped" {
		if err := c.StopInstance(instanceName); err != nil {
			return "", fmt.Errorf("instance must be stopped for export: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	if outputPath == "" {
		outputPath = fmt.Sprintf("./%s.img", instanceName)
	}

	cmd := exec.CommandContext(ctx, "multipass", "export", instanceName, outputPath)
	_, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to export instance: %w", err)
	}

	return outputPath, nil
}

func (c *multipassClient) ImportInstance(imagePath string, name string, cpus int, memory string, disk string) (*models.Instance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout*3)
	defer cancel()

	args := []string{"launch"}

	if name != "" {
		args = append(args, "-n", name)
	}
	if cpus > 0 {
		args = append(args, "-c", fmt.Sprintf("%d", cpus))
	}
	if memory != "" {
		args = append(args, "-m", memory)
	}
	if disk != "" {
		args = append(args, "-d", disk)
	}

	args = append(args, imagePath)

	cmd := exec.CommandContext(ctx, "multipass", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to import instance: %w", err)
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "Launched:") {
		parts := strings.Split(outputStr, ":")
		if len(parts) >= 2 {
			instanceName := strings.TrimSpace(parts[1])
			return c.GetInstance(instanceName)
		}
	}

	return c.GetInstance(name)
}
