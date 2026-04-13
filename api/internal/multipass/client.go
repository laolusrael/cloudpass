package multipass

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"cloudpass/internal/logger"
	"cloudpass/internal/models"
)

type Client interface {
	Authenticate(passphrase string) error
	ListInstances() ([]models.Instance, error)
	GetInstance(name string) (*models.Instance, error)
	GetInstanceResources(name string) (*models.InstanceResources, error)
	GetInstanceIP(name string) (string, error)
	CreateInstance(opts models.CreateInstanceRequest) (*models.Instance, error)
	LaunchInstanceBackground(opts models.CreateInstanceRequest) error
	WaitForInstance(name string, timeout time.Duration) (*models.Instance, error)
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
	UploadFile(instanceName string, localPath string, targetPath string) error
	CreateSnapshot(instanceName string, snapshotName string, comment string) error
	RestoreSnapshot(instanceName string, snapshotName string) error
	ListSnapshots(instanceName string) ([]models.Snapshot, error)
	DeleteSnapshot(instanceName string, snapshotName string) error
	ExportInstance(instanceName string, outputPath string) (string, error)
	ImportInstance(imagePath string, name string, cpus int, memory string, disk string) (*models.Instance, error)
	GetHostInfo() (*models.HostInfo, error)
	SetInstanceResources(name string, cpus int, memory string, disk string) error
}

type multipassClient struct {
	timeout time.Duration
}

func NewClient(timeoutSec int) Client {
	return &multipassClient{
		timeout: time.Duration(timeoutSec) * time.Second,
	}
}

func (c *multipassClient) Authenticate(passphrase string) error {
	if passphrase == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	logger.Multipass.Debug().Msg("authenticating with multipass")
	cmd := exec.CommandContext(ctx, "multipass", "authenticate")
	cmd.Stdin = strings.NewReader(passphrase)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Msg("multipass authentication failed")
		return fmt.Errorf("authentication failed: %w", err)
	}

	logger.Multipass.Info().Msg("multipass authenticated successfully")
	return nil
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

	// For stopped instances, multipass info may return empty/zero values for resources
	// Fall back to multipass get to get the configured values
	if instance.CPU == 0 || instance.Memory == "" || instance.Disk == "" {
		resources, err := c.GetInstanceResources(name)
		if err == nil {
			if instance.CPU == 0 && resources.CPUs > 0 {
				instance.CPU = resources.CPUs
			}
			if instance.Memory == "" && resources.Memory != "" {
				instance.Memory = resources.Memory
			}
			if instance.Disk == "" && resources.Disk != "" {
				instance.Disk = resources.Disk
			}
		}
	}

	return instance, nil
}

func (c *multipassClient) GetInstanceResources(name string) (*models.InstanceResources, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	resources := &models.InstanceResources{}

	cpusCmd := exec.CommandContext(ctx, "multipass", "get", fmt.Sprintf("local.%s.cpus", name))
	cpusOutput, err := cpusCmd.Output()
	if err != nil {
		logger.Multipass.Debug().Str("name", name).Err(err).Msg("failed to get CPU config")
	} else {
		cpus, err := strconv.Atoi(strings.TrimSpace(string(cpusOutput)))
		if err != nil {
			logger.Multipass.Warn().Str("name", name).Err(err).Msg("failed to parse CPU config")
		} else {
			resources.CPUs = cpus
		}
	}

	memoryCmd := exec.CommandContext(ctx, "multipass", "get", fmt.Sprintf("local.%s.memory", name))
	memoryOutput, err := memoryCmd.Output()
	if err != nil {
		logger.Multipass.Debug().Str("name", name).Err(err).Msg("failed to get memory config")
	} else {
		resources.Memory = strings.TrimSpace(string(memoryOutput))
	}

	diskCmd := exec.CommandContext(ctx, "multipass", "get", fmt.Sprintf("local.%s.disk", name))
	diskOutput, err := diskCmd.Output()
	if err != nil {
		logger.Multipass.Debug().Str("name", name).Err(err).Msg("failed to get disk config")
	} else {
		resources.Disk = strings.TrimSpace(string(diskOutput))
	}

	logger.Multipass.Debug().
		Str("name", name).
		Int("cpus", resources.CPUs).
		Str("memory", resources.Memory).
		Str("disk", resources.Disk).
		Msg("got instance resources config")

	return resources, nil
}

func (c *multipassClient) GetInstanceIP(name string) (string, error) {
	instance, err := c.GetInstance(name)
	if err != nil {
		return "", err
	}

	if len(instance.IPv4) == 0 {
		return "", fmt.Errorf("instance has no IPv4 address")
	}

	return instance.IPv4[0], nil
}

func (c *multipassClient) CreateInstance(opts models.CreateInstanceRequest) (*models.Instance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	args := []string{"launch"}

	if opts.Image != "" {
		args = append(args, opts.Image)
	}

	args = append(args, "--timeout", fmt.Sprintf("%d", int(c.timeout.Seconds())))

	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}
	if opts.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", opts.CPUs))
	}
	if opts.Memory != "" {
		args = append(args, "--memory", opts.Memory)
	}
	if opts.Disk != "" {
		args = append(args, "--disk", opts.Disk)
	}
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}
	if opts.CloudInit != "" {
		args = append(args, "--cloud-init", "-")
	}

	if opts.Image != "" {
		logger.Multipass.Info().Str("image", opts.Image).Msg("starting instance creation with image")
	} else {
		logger.Multipass.Info().Msg("starting instance creation with default image")
	}

	cmd := exec.CommandContext(ctx, "multipass", args...)
	output, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("name", opts.Name).Msg("failed to create instance")
		return nil, fmt.Errorf("failed to create instance: %w", err)
	}

	logger.Multipass.Info().Str("name", opts.Name).Msg("instance creation command completed, fetching instance info")

	outputStr := string(output)
	if strings.Contains(outputStr, "Launched:") {
		parts := strings.Split(outputStr, ":")
		if len(parts) >= 2 {
			name := strings.TrimSpace(parts[1])
			logger.Multipass.Info().Str("name", name).Msg("instance launched, retrieving details")
			instance, err := c.GetInstance(name)
			if err != nil {
				logger.Multipass.Error().Err(err).Str("name", name).Msg("failed to get instance details after launch")
				return nil, fmt.Errorf("failed to get instance: %w", err)
			}
			logger.Multipass.Info().Str("name", instance.Name).Str("state", instance.State).Msg("instance ready")
			return instance, nil
		}
	}

	logger.Multipass.Info().Str("name", opts.Name).Msg("retrieving instance info")
	instance, err := c.GetInstance(opts.Name)
	if err != nil {
		logger.Multipass.Error().Err(err).Str("name", opts.Name).Msg("failed to get instance info")
		return nil, fmt.Errorf("failed to get instance: %w", err)
	}
	logger.Multipass.Info().Str("name", instance.Name).Str("state", instance.State).Msg("instance ready")
	return instance, nil
}

func (c *multipassClient) LaunchInstanceBackground(opts models.CreateInstanceRequest) error {
	args := []string{"launch"}

	if opts.Image != "" {
		args = append(args, opts.Image)
	}

	args = append(args, "--timeout", fmt.Sprintf("%d", int(c.timeout.Seconds())))

	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}
	if opts.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", opts.CPUs))
	}
	if opts.Memory != "" {
		args = append(args, "--memory", opts.Memory)
	}
	if opts.Disk != "" {
		args = append(args, "--disk", opts.Disk)
	}
	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}
	if opts.CloudInit != "" {
		args = append(args, "--cloud-init", "-")
	}

	if opts.Image != "" {
		logger.Multipass.Info().Str("image", opts.Image).Str("name", opts.Name).Msg("starting instance creation in background")
	} else {
		logger.Multipass.Info().Str("name", opts.Name).Msg("starting instance creation in background with default image")
	}

	cmd := exec.Command("multipass", args...)
	if err := cmd.Start(); err != nil {
		logger.Multipass.Error().Err(err).Str("name", opts.Name).Msg("failed to start multipass launch")
		return fmt.Errorf("failed to start instance creation: %w", err)
	}

	logger.Multipass.Info().Str("name", opts.Name).Msg("instance launch started in background")
	return nil
}

func (c *multipassClient) WaitForInstance(name string, timeout time.Duration) (*models.Instance, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	timeoutChan := time.After(timeout)

	logger.Multipass.Info().Str("name", name).Msg("waiting for instance to be created")

	for {
		select {
		case <-timeoutChan:
			logger.Multipass.Error().Str("name", name).Msg("instance creation timed out")
			return nil, fmt.Errorf("instance creation timed out after %v", timeout)
		case <-ticker.C:
			instances, err := c.ListInstances()
			if err != nil {
				logger.Multipass.Warn().Err(err).Str("name", name).Msg("failed to list instances, retrying")
				continue
			}

			for _, inst := range instances {
				if inst.Name == name {
					logger.Multipass.Info().Str("name", name).Str("state", inst.State).Msg("instance found")

					instance, err := c.GetInstance(name)
					if err != nil {
						return nil, fmt.Errorf("failed to get instance details: %w", err)
					}

					logger.Multipass.Info().Str("name", name).Str("state", instance.State).Msg("instance ready")
					return instance, nil
				}
			}

			logger.Multipass.Debug().Str("name", name).Msg("instance not yet created, waiting...")
		}
	}
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

	var raw struct {
		Images map[string]json.RawMessage `json:"images"`
	}
	if err := json.Unmarshal(output, &raw); err != nil {
		logger.Multipass.Error().Err(err).Msg("failed to parse images")
		return nil, fmt.Errorf("failed to parse images: %w", err)
	}

	images := make([]models.Image, 0, len(raw.Images))
	for alias, data := range raw.Images {
		var img models.Image
		if err := json.Unmarshal(data, &img); err != nil {
			logger.Multipass.Warn().Str("alias", alias).Err(err).Msg("failed to parse image data")
			continue
		}
		img.Alias = alias
		images = append(images, img)
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

	instance, err := c.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		return fmt.Errorf("instance %q not found", instanceName)
	}
	if instance.State != "Running" {
		return fmt.Errorf("instance %q is not running (current state: %s)", instanceName, instance.State)
	}

	if err := c.ensureTargetPath(instanceName, targetPath); err != nil {
		return fmt.Errorf("failed to create target path: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	var args []string
	if runtime.GOOS == "linux" {
		args = []string{"mount", "--type=native", sourcePath, instanceName + ":" + targetPath}
	} else {
		args = []string{"mount", sourcePath, instanceName + ":" + targetPath}
	}

	logger.Multipass.Info().
		Str("instance", instanceName).
		Str("source", sourcePath).
		Str("target", targetPath).
		Msg("mounting directory")

	cmd := exec.CommandContext(ctx, "multipass", args...)
	var stderrOut string
	_, err = cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderrOut = string(exitErr.Stderr)
			logger.Multipass.Error().
				Err(err).
				Str("instance", instanceName).
				Str("stderr", stderrOut).
				Msg("failed to mount directory")
			return fmt.Errorf("failed to mount directory: %s", stderrOut)
		}
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

func (c *multipassClient) ensureTargetPath(instanceName string, targetPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mkdirCmd := exec.CommandContext(ctx, "multipass", "exec", instanceName, "--", "mkdir", "-p", targetPath)
	if err := mkdirCmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			return fmt.Errorf("failed to create target path %q in instance: %s", targetPath, stderr)
		}
		return fmt.Errorf("failed to create target path %q in instance: %w", targetPath, err)
	}

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

func (c *multipassClient) UploadFile(instanceName string, localPath string, targetPath string) error {
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		return fmt.Errorf("local file %q does not exist", localPath)
	} else if err != nil {
		return fmt.Errorf("cannot access local file %q: %w", localPath, err)
	}

	instance, err := c.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}
	if instance == nil {
		return fmt.Errorf("instance %q not found", instanceName)
	}
	if instance.State != "Running" {
		return fmt.Errorf("instance %q is not running (current state: %s)", instanceName, instance.State)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	targetDir := filepath.Dir(targetPath)
	if targetDir != "." {
		mkdirCmd := exec.CommandContext(ctx, "multipass", "exec", instanceName, "--", "mkdir", "-p", targetDir)
		if err := mkdirCmd.Run(); err != nil {
			return fmt.Errorf("failed to create target directory: %w", err)
		}
	}

	args := []string{"transfer", "--parents", localPath, instanceName + ":" + targetPath}

	logger.Multipass.Info().
		Str("instance", instanceName).
		Str("local", localPath).
		Str("target", targetPath).
		Msg("uploading file to instance")

	cmd := exec.CommandContext(ctx, "multipass", args...)
	var stderrOut string
	_, err = cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderrOut = string(exitErr.Stderr)
			logger.Multipass.Error().
				Err(err).
				Str("instance", instanceName).
				Str("stderr", stderrOut).
				Msg("failed to upload file")
			return fmt.Errorf("failed to upload file: %s", stderrOut)
		}
		logger.Multipass.Error().
			Err(err).
			Str("instance", instanceName).
			Msg("failed to upload file")
		return fmt.Errorf("failed to upload file: %w", err)
	}

	logger.Multipass.Info().
		Str("instance", instanceName).
		Str("target", targetPath).
		Msg("file uploaded to instance")
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

func (c *multipassClient) CreateSnapshot(instanceName string, snapshotName string, comment string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout*2)
	defer cancel()

	instance, err := c.GetInstance(instanceName)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}

	wasRunning := instance.State == "Running"

	if wasRunning {
		logger.Multipass.Info().Str("instance", instanceName).Msg("stopping instance for snapshot")
		stopCtx, stopCancel := context.WithTimeout(context.Background(), c.timeout)
		defer stopCancel()

		cmd := exec.CommandContext(stopCtx, "multipass", "stop", instanceName)
		_, err = cmd.Output()
		if err != nil {
			logger.Multipass.Error().Err(err).Str("instance", instanceName).Msg("failed to stop instance for snapshot")
			return fmt.Errorf("failed to stop instance: %w", err)
		}

		for i := 0; i < 10; i++ {
			time.Sleep(500 * time.Millisecond)
			instance, err = c.GetInstance(instanceName)
			if err != nil {
				return fmt.Errorf("failed to get instance state: %w", err)
			}
			if instance.State == "Stopped" {
				break
			}
		}

		if instance.State != "Stopped" {
			return fmt.Errorf("instance did not stop in time, current state: %s", instance.State)
		}
		logger.Multipass.Info().Str("instance", instanceName).Msg("instance stopped, creating snapshot")
	}

	args := []string{"snapshot", instanceName}
	if snapshotName != "" {
		args = append(args, snapshotName)
	}

	logger.Multipass.Info().Str("instance", instanceName).Str("snapshot", snapshotName).Msg("creating snapshot")
	cmd := exec.CommandContext(ctx, "multipass", args...)
	output, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("instance", instanceName).Msg("failed to create snapshot")
		return fmt.Errorf("failed to create snapshot: %w", err)
	}

	if comment != "" {
		setCtx, setCancel := context.WithTimeout(context.Background(), c.timeout)
		defer setCancel()
		setCmd := exec.CommandContext(setCtx, "multipass", "set", fmt.Sprintf("local.%s.%s.comment", instanceName, snapshotName), comment)
		_, setErr := setCmd.Output()
		if setErr != nil {
			logger.Multipass.Warn().Err(setErr).Str("instance", instanceName).Str("snapshot", snapshotName).Msg("failed to set snapshot comment")
		}
	}

	_ = output

	logger.Multipass.Info().Str("instance", instanceName).Str("snapshot", snapshotName).Msg("snapshot created")
	return nil
}

func (c *multipassClient) RestoreSnapshot(instanceName string, snapshotName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout*2)
	defer cancel()

	args := []string{"restore", instanceName}
	if snapshotName != "" {
		args = append(args, snapshotName)
	}

	logger.Multipass.Info().Str("instance", instanceName).Str("snapshot", snapshotName).Msg("restoring snapshot")
	cmd := exec.CommandContext(ctx, "multipass", args...)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("instance", instanceName).Msg("failed to restore snapshot")
		return fmt.Errorf("failed to restore snapshot: %w", err)
	}

	logger.Multipass.Info().Str("instance", instanceName).Str("snapshot", snapshotName).Msg("snapshot restored")
	return nil
}

func (c *multipassClient) ListSnapshots(instanceName string) ([]models.Snapshot, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "multipass", "list", "--snapshots", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return nil, fmt.Errorf("instance %q not found", instanceName)
		}
		return nil, fmt.Errorf("failed to get snapshot list: %w", err)
	}

	var result struct {
		Errors []string `json:"errors"`
		Info   map[string]map[string]struct {
			Comment string `json:"comment"`
			Parent  string `json:"parent"`
		} `json:"info"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot list output: %w", err)
	}

	if len(result.Errors) > 0 {
		return nil, fmt.Errorf("multipass error: %s", result.Errors[0])
	}

	instanceSnapshots, ok := result.Info[instanceName]
	if !ok {
		return []models.Snapshot{}, nil
	}

	snapshots := make([]models.Snapshot, 0, len(instanceSnapshots))
	for name, snap := range instanceSnapshots {
		snapshots = append(snapshots, models.Snapshot{
			Name:     name,
			Instance: instanceName,
			Comment:  snap.Comment,
			Parent:   snap.Parent,
		})
	}

	return snapshots, nil
}

func (c *multipassClient) DeleteSnapshot(instanceName string, snapshotName string) error {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	logger.Multipass.Info().Str("instance", instanceName).Str("snapshot", snapshotName).Msg("deleting snapshot")
	cmd := exec.CommandContext(ctx, "multipass", "delete", instanceName, "--snapshot", snapshotName)
	_, err := cmd.Output()
	if err != nil {
		logger.Multipass.Error().Err(err).Str("instance", instanceName).Msg("failed to delete snapshot")
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

func (c *multipassClient) GetHostInfo() (*models.HostInfo, error) {
	const (
		cpuReserved    int64 = 2
		memoryReserved int64 = 2 * 1024 * 1024 * 1024
	)

	var cpuCores int64
	var memoryBytes int64
	var diskBytes int64

	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("powershell", "-Command", "(Get-CimInstance Win32_Processor).NumberOfLogicalProcessors")
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get CPU count: %w", err)
		}
		n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CPU count: %w", err)
		}
		cpuCores = n

		cmd = exec.Command("powershell", "-Command", "(Get-CimInstance Win32_OperatingSystem).TotalVisibleMemorySize * 1024")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get memory: %w", err)
		}
		n, err = strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse memory: %w", err)
		}
		memoryBytes = n

		cmd = exec.Command("powershell", "-Command", "(Get-PSDrive C).Free * 1024")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get disk space: %w", err)
		}
		n, err = strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse disk space: %w", err)
		}
		diskBytes = n

	case "darwin":
		cmd := exec.Command("sysctl", "-n", "hw.ncpu")
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get CPU count: %w", err)
		}
		n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CPU count: %w", err)
		}
		cpuCores = n

		cmd = exec.Command("sysctl", "-n", "hw.memsize")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get memory: %w", err)
		}
		n, err = strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse memory: %w", err)
		}
		memoryBytes = n

		cmd = exec.Command("df", "-bk", "/")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get disk space: %w", err)
		}
		lines := strings.Split(string(out), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 4 {
				n, err = strconv.ParseInt(fields[3], 10, 64)
				if err != nil {
					return nil, fmt.Errorf("failed to parse disk space: %w", err)
				}
				diskBytes = n * 1024
			}
		}

	case "linux":
		cmd := exec.Command("nproc")
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get CPU count: %w", err)
		}
		n, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("failed to parse CPU count: %w", err)
		}
		cpuCores = n

		cmd = exec.Command("free", "-b")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get memory: %w", err)
		}
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "Mem:" {
				n, err = strconv.ParseInt(fields[1], 10, 64)
				if err != nil {
					return nil, fmt.Errorf("failed to parse memory: %w", err)
				}
				memoryBytes = n
				break
			}
		}

		cmd = exec.Command("df", "-B1", "/")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("failed to get disk space: %w", err)
		}
		lines = strings.Split(string(out), "\n")
		if len(lines) >= 2 {
			fields := strings.Fields(lines[1])
			if len(fields) >= 4 {
				n, err = strconv.ParseInt(fields[3], 10, 64)
				if err != nil {
					return nil, fmt.Errorf("failed to parse disk space: %w", err)
				}
				diskBytes = n
			}
		}

	default:
		return nil, fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	cpuAvailable := cpuCores - cpuReserved
	if cpuAvailable < 1 {
		cpuAvailable = 1
	}

	memoryAvailable := memoryBytes - memoryReserved
	if memoryAvailable < 512*1024*1024 {
		memoryAvailable = 512 * 1024 * 1024
	}

	return &models.HostInfo{
		CPUCores:        cpuCores,
		CPUAvailable:    cpuAvailable,
		CPUReserved:     cpuReserved,
		MemoryBytes:     memoryBytes,
		MemoryAvailable: memoryAvailable,
		MemoryReserved:  memoryReserved,
		DiskBytes:       diskBytes,
		DiskAvailable:   diskBytes,
	}, nil
}

func (c *multipassClient) SetInstanceResources(name string, cpus int, memory string, disk string) error {
	instance, err := c.GetInstance(name)
	if err != nil {
		return fmt.Errorf("failed to get instance: %w", err)
	}

	if instance.State != "Stopped" {
		return fmt.Errorf("instance must be stopped to modify resources (current state: %s)", instance.State)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	if cpus > 0 {
		logger.Multipass.Info().Str("name", name).Int("cpus", cpus).Msg("setting CPU")
		cmd := exec.CommandContext(ctx, "multipass", "set", fmt.Sprintf("local.%s.cpus", name), strconv.Itoa(cpus))
		if _, err := cmd.Output(); err != nil {
			return fmt.Errorf("failed to set CPU: %w", err)
		}
	}

	if memory != "" {
		logger.Multipass.Info().Str("name", name).Str("memory", memory).Msg("setting memory")
		cmd := exec.CommandContext(ctx, "multipass", "set", fmt.Sprintf("local.%s.memory", name), memory)
		if _, err := cmd.Output(); err != nil {
			return fmt.Errorf("failed to set memory: %w", err)
		}
	}

	if disk != "" {
		logger.Multipass.Info().Str("name", name).Str("disk", disk).Msg("setting disk")
		cmd := exec.CommandContext(ctx, "multipass", "set", fmt.Sprintf("local.%s.disk", name), disk)
		if _, err := cmd.Output(); err != nil {
			return fmt.Errorf("failed to set disk: %w", err)
		}
	}

	logger.Multipass.Info().Str("name", name).Msg("instance resources updated")
	return nil
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
