package multipass

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"cloudpass/internal/logger"
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
	MountInstance(instanceName string, sourcePath string, targetPath string) error
	UnmountInstance(instanceName string, targetPath string) error
	CreateSnapshot(instanceName string, snapshotName string, comment string) (string, bool, error)
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

	logger.Multipass.Debug().Msg("executing: multipass list")
	cmd := exec.CommandContext(ctx, "multipass", "list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Msg("failed to list instances")
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

	logger.Multipass.Debug().Str("name", name).Msg("executing: multipass info")
	cmd := exec.CommandContext(ctx, "multipass", "info", name, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			logger.Multipass.Warn().Str("name", name).Msg("instance not found")
			return nil, fmt.Errorf("instance %q not found", name)
		}
		logger.Multipass.Error().Err(err).Str("name", name).Msg("failed to get instance info")
		return nil, fmt.Errorf("failed to get instance info: %w", err)
	}

	var raw struct {
		Info map[string]json.RawMessage `json:"info"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse info output: %w", err)
	}

	instanceData, ok := raw.Info[name]
	if !ok {
		return nil, fmt.Errorf("instance %q not found", name)
	}

	var i struct {
		Name     string   `json:"name"`
		State    string   `json:"state"`
		IPv4     []string `json:"ipv4"`
		IPv6     []string `json:"ipv6"`
		CPUCount string   `json:"cpu_count"`
		Memory   struct {
			Total uint64 `json:"total"`
			Used  uint64 `json:"used"`
		} `json:"memory"`
		Disks map[string]struct {
			Total string `json:"total"`
			Used  string `json:"used"`
		} `json:"disks"`
		Image   string `json:"image_release"`
		Release string `json:"release"`
		Mounts  map[string]struct {
			SourcePath string `json:"source_path"`
			TargetPath string `json:"target_path"`
		} `json:"mounts"`
		Load    []float64 `json:"load"`
		Network map[string]struct {
			IPv4 string `json:"ipv4"`
			IPv6 string `json:"ipv6"`
		} `json:"network"`
	}

	if err := json.Unmarshal(instanceData, &i); err != nil {
		return nil, fmt.Errorf("failed to parse instance data: %w", err)
	}

	instance := &models.Instance{
		Name:    name,
		State:   i.State,
		IPv4:    i.IPv4,
		IPv6:    i.IPv6,
		CPU:     parseCPU(i.CPUCount),
		Memory:  formatBytes(i.Memory.Total),
		Image:   i.Image,
		Release: i.Release,
		Load:    i.Load,
		Mounts:  make([]models.Mount, 0),
		Network: make(map[string]models.NetworkInfo),
	}

	// Calculate disk space from disks map
	for _, disk := range i.Disks {
		instance.Disk = formatBytes(parseBytes(disk.Total))
		break // Use first disk
	}

	for targetPath, m := range i.Mounts {
		instance.Mounts = append(instance.Mounts, models.Mount{
			Source: m.SourcePath,
			Target: targetPath,
		})
	}

	for netName, net := range i.Network {
		instance.Network[netName] = models.NetworkInfo{
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

	logger.Multipass.Info().Str("name", opts.Name).Msg("creating instance")
	cmd := exec.CommandContext(ctx, "multipass", args...)
	output, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("name", opts.Name).Msg("failed to create instance")
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

	logger.Multipass.Debug().Str("name", name).Msg("starting instance")
	cmd := exec.CommandContext(ctx, "multipass", "start", name)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("name", name).Msg("failed to start instance")
		return fmt.Errorf("failed to start instance: %w", err)
	}

	logger.Multipass.Info().Str("name", name).Msg("instance started")
	return nil
}

func (c *multipassClient) StopInstance(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	logger.Multipass.Debug().Str("name", name).Msg("stopping instance")
	cmd := exec.CommandContext(ctx, "multipass", "stop", name)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("name", name).Msg("failed to stop instance")
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	logger.Multipass.Info().Str("name", name).Msg("instance stopped")
	return nil
}

func (c *multipassClient) RestartInstance(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	logger.Multipass.Debug().Str("name", name).Msg("restarting instance")
	cmd := exec.CommandContext(ctx, "multipass", "restart", name)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("name", name).Msg("failed to restart instance")
		return fmt.Errorf("failed to restart instance: %w", err)
	}

	logger.Multipass.Info().Str("name", name).Msg("instance restarted")
	return nil
}

func (c *multipassClient) DeleteInstance(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	logger.Multipass.Info().Str("name", name).Msg("deleting instance")
	cmd := exec.CommandContext(ctx, "multipass", "delete", name)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("name", name).Msg("failed to delete instance")
		return fmt.Errorf("failed to delete instance: %w", err)
	}

	logger.Multipass.Info().Str("name", name).Msg("instance deleted")
	return nil
}

func (c *multipassClient) SuspendInstance(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	logger.Multipass.Debug().Str("name", name).Msg("suspending instance")
	cmd := exec.CommandContext(ctx, "multipass", "suspend", name)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("name", name).Msg("failed to suspend instance")
		return fmt.Errorf("failed to suspend instance: %w", err)
	}

	logger.Multipass.Info().Str("name", name).Msg("instance suspended")
	return nil
}

func (c *multipassClient) ResumeInstance(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	logger.Multipass.Debug().Str("name", name).Msg("resuming instance")
	cmd := exec.CommandContext(ctx, "multipass", "start", name)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("name", name).Msg("failed to resume instance")
		return fmt.Errorf("failed to resume instance: %w", err)
	}

	logger.Multipass.Info().Str("name", name).Msg("instance resumed")
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

	logger.Multipass.Info().Str("name", name).Str("mode", mode).Msg("creating network")
	cmd := exec.CommandContext(ctx, "multipass", args...)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("name", name).Msg("failed to create network")
		return fmt.Errorf("failed to create network: %w", err)
	}

	logger.Multipass.Info().Str("name", name).Msg("network created")
	return nil
}

func (c *multipassClient) DeleteNetwork(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	logger.Multipass.Info().Str("name", name).Msg("deleting network")
	cmd := exec.CommandContext(ctx, "multipass", "networks", "delete", name)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("name", name).Msg("failed to delete network")
		return fmt.Errorf("failed to delete network: %w", err)
	}

	logger.Multipass.Info().Str("name", name).Msg("network deleted")
	return nil
}

func (c *multipassClient) ListImages() ([]models.Image, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	logger.Multipass.Debug().Msg("listing images")
	cmd := exec.CommandContext(ctx, "multipass", "find", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Msg("failed to list images")
		return nil, fmt.Errorf("failed to list images: %w", err)
	}

	var images []models.Image
	if err := json.Unmarshal(output, &images); err != nil {
		logger.Multipass.Error().Err(err).Msg("failed to parse images")
		return nil, fmt.Errorf("failed to parse images: %w", err)
	}

	logger.Multipass.Debug().Int("count", len(images)).Msg("listed images")
	return images, nil
}

func (c *multipassClient) ListNetworks() ([]models.Network, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	logger.Multipass.Debug().Msg("listing networks")
	cmd := exec.CommandContext(ctx, "multipass", "networks", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Msg("failed to list networks")
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}

	var result struct {
		Networks []models.Network `json:"list"`
	}
	if err := json.Unmarshal(output, &result); err != nil {
		logger.Multipass.Error().Err(err).Msg("failed to parse networks")
		return nil, fmt.Errorf("failed to parse networks: %w", err)
	}

	logger.Multipass.Debug().Int("count", len(result.Networks)).Msg("listed networks")
	return result.Networks, nil
}

func (c *multipassClient) MountInstance(instanceName string, sourcePath string, targetPath string) error {
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return fmt.Errorf("source path %q does not exist", sourcePath)
	} else if err != nil {
		return fmt.Errorf("cannot access source path %q: %w", sourcePath, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	args := []string{"mount", sourcePath, instanceName + ":" + targetPath}

	logger.Multipass.Info().
		Str("instance", instanceName).
		Str("source", sourcePath).
		Str("target", targetPath).
		Msg("mounting directory")

	cmd := exec.CommandContext(ctx, "multipass", args...)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().
			Err(err).
			Str("instance", instanceName).
			Msg("failed to mount directory")
		return fmt.Errorf("failed to mount directory: %w", err)
	}

	logger.Multipass.Info().
		Str("instance", instanceName).
		Str("target", targetPath).
		Msg("directory mounted")
	return nil
}

func (c *multipassClient) UnmountInstance(instanceName string, targetPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	args := []string{"umount", instanceName + ":" + targetPath}

	logger.Multipass.Info().
		Str("instance", instanceName).
		Str("target", targetPath).
		Msg("unmounting directory")

	cmd := exec.CommandContext(ctx, "multipass", args...)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().
			Err(err).
			Str("instance", instanceName).
			Msg("failed to unmount directory")
		return fmt.Errorf("failed to unmount directory: %w", err)
	}

	logger.Multipass.Info().
		Str("instance", instanceName).
		Str("target", targetPath).
		Msg("directory unmounted")
	return nil
}

func (c *multipassClient) PurgeDeleted() error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	logger.Multipass.Info().Msg("purging deleted instances")
	cmd := exec.CommandContext(ctx, "multipass", "purge")
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Msg("failed to purge")
		return fmt.Errorf("failed to purge: %w", err)
	}

	logger.Multipass.Info().Msg("purged deleted instances")
	return nil
}

func (c *multipassClient) CreateSnapshot(instanceName string, snapshotName string, comment string) (string, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	instance, err := c.GetInstance(instanceName)
	if err != nil {
		return "", false, err
	}

	wasRunning := instance.State == "Running"

	if wasRunning {
		logger.Multipass.Info().Str("instance", instanceName).Msg("stopping instance for snapshot")
		if err := c.StopInstance(instanceName); err != nil {
			return "", false, fmt.Errorf("failed to stop instance for snapshot: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	args := []string{"snapshot"}
	if snapshotName != "" {
		args = append(args, "--name", snapshotName)
	}
	if comment != "" {
		args = append(args, "--comment", comment)
	}
	args = append(args, instanceName)

	logger.Multipass.Info().Str("instance", instanceName).Str("snapshot", snapshotName).Msg("creating snapshot")
	cmd := exec.CommandContext(ctx, "multipass", args...)
	output, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().
			Str("instance", instanceName).
			Str("output", string(output)).
			Msg("failed to create snapshot")
		return "", false, fmt.Errorf("failed to create snapshot: %w", err)
	}

	createdName := snapshotName
	if createdName == "" {
		createdName = "snapshot0"
	}

	logger.Multipass.Info().Str("instance", instanceName).Str("snapshot", createdName).Msg("snapshot created")
	return createdName, wasRunning, nil
}

func (c *multipassClient) RestoreSnapshot(instanceName string, snapshotName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout*2)
	defer cancel()

	args := []string{"restore"}
	if snapshotName != "" {
		args = append(args, "--name", snapshotName)
	}
	args = append(args, instanceName)

	logger.Multipass.Info().Str("instance", instanceName).Str("snapshot", snapshotName).Msg("restoring snapshot")
	cmd := exec.CommandContext(ctx, "multipass", args...)
	output, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().
			Str("instance", instanceName).
			Str("output", string(output)).
			Msg("failed to restore snapshot")
		return fmt.Errorf("failed to restore snapshot: %w", err)
	}

	logger.Multipass.Info().Str("instance", instanceName).Str("snapshot", snapshotName).Msg("snapshot restored")
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

	logger.Multipass.Info().Str("instance", instanceName).Str("snapshot", snapshotName).Msg("deleting snapshot")
	cmd := exec.CommandContext(ctx, "multipass", "delete", instanceName, "--snapshot", snapshotName)
	output, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().
			Str("instance", instanceName).
			Str("output", string(output)).
			Msg("failed to delete snapshot")
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	logger.Multipass.Info().Str("instance", instanceName).Str("snapshot", snapshotName).Msg("snapshot deleted")
	return nil
}

func (c *multipassClient) ExportInstance(instanceName string, outputPath string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout*3)
	defer cancel()

	logger.Multipass.Info().Str("instance", instanceName).Str("output", outputPath).Msg("exporting instance")
	instance, err := c.GetInstance(instanceName)
	if err != nil {
		return "", err
	}

	if instance.State != "Stopped" {
		logger.Multipass.Info().Str("instance", instanceName).Msg("stopping instance for export")
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
		logger.Multipass.Error().Err(err).Str("instance", instanceName).Msg("failed to export instance")
		return "", fmt.Errorf("failed to export instance: %w", err)
	}

	logger.Multipass.Info().Str("instance", instanceName).Str("output", outputPath).Msg("instance exported")
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

	logger.Multipass.Info().Str("image", imagePath).Str("name", name).Msg("importing instance")
	cmd := exec.CommandContext(ctx, "multipass", args...)
	output, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("image", imagePath).Msg("failed to import instance")
		return nil, fmt.Errorf("failed to import instance: %w", err)
	}

	outputStr := string(output)
	if strings.Contains(outputStr, "Launched:") {
		parts := strings.Split(outputStr, ":")
		if len(parts) >= 2 {
			instanceName := strings.TrimSpace(parts[1])
			logger.Multipass.Info().Str("name", instanceName).Msg("instance imported")
			return c.GetInstance(instanceName)
		}
	}

	return c.GetInstance(name)
}

func parseCPU(cpuStr string) int {
	if cpuStr == "" {
		return 0
	}
	n, err := strconv.Atoi(cpuStr)
	if err != nil {
		return 0
	}
	return n
}

func parseBytes(s string) uint64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func formatBytes(n uint64) string {
	if n == 0 {
		return "0 B"
	}
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for n >= div*unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}
