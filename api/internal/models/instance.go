package models

import "time"

type Instance struct {
	Name      string                 `json:"name"`
	State     string                 `json:"state"`
	IPv4      []string               `json:"ipv4,omitempty"`
	IPv6      []string               `json:"ipv6,omitempty"`
	CPU       int                    `json:"cpu,omitempty"`
	Memory    string                 `json:"memory,omitempty"`
	Disk      string                 `json:"disk,omitempty"`
	Image     string                 `json:"image,omitempty"`
	Release   string                 `json:"release,omitempty"`
	CreatedAt string                 `json:"created_at,omitempty"`
	Mounts    []Mount                `json:"mounts,omitempty"`
	Disks     []Disk                 `json:"disks,omitempty"`
	Load      []float64              `json:"load,omitempty"`
	Network   map[string]NetworkInfo `json:"network,omitempty"`
}

type InstanceState struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

type Mount struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

type Disk struct {
	Name  string `json:"name"`
	Total string `json:"total"`
	Used  string `json:"used"`
}

type NetworkInfo struct {
	IPv4 string `json:"ipv4,omitempty"`
	IPv6 string `json:"ipv6,omitempty"`
}

type InstanceList struct {
	Instances []Instance `json:"instances"`
}

type CreateInstanceRequest struct {
	Name      string `json:"name"`
	Image     string `json:"image,omitempty"`
	CPUs      int    `json:"cpus,omitempty"`
	Memory    string `json:"memory,omitempty"`
	Disk      string `json:"disk,omitempty"`
	Network   string `json:"network,omitempty"`
	CloudInit string `json:"cloud_init,omitempty"`
}

type InstanceResponse struct {
	Message string `json:"message,omitempty"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type Image struct {
	Alias   string   `json:"alias"`
	Version string   `json:"version"`
	Release string   `json:"release"`
	Remote  string   `json:"remote"`
	OS      string   `json:"os"`
	Aliases []string `json:"aliases"`
}

type ImageList struct {
	Images []Image `json:"images"`
}

type Network struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	IPv4        string `json:"ipv4,omitempty"`
	Description string `json:"description,omitempty"`
}

type NetworkList struct {
	Networks []Network `json:"networks"`
}

type CreateNetworkRequest struct {
	Name string `json:"name"`
	Mode string `json:"mode,omitempty"`
	MAC  string `json:"mac,omitempty"`
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

type Snapshot struct {
	Name       string `json:"name"`
	Instance   string `json:"instance"`
	CreatedAt  string `json:"created_at,omitempty"`
	Comment    string `json:"comment,omitempty"`
	Parent     string `json:"parent,omitempty"`
	Children   int    `json:"children,omitempty"`
	StateSize  int64  `json:"state_size,omitempty"`
	DiskSize   int64  `json:"disk_size,omitempty"`
	MemorySize int64  `json:"memory_size,omitempty"`
}

type SnapshotList struct {
	Snapshots []Snapshot `json:"snapshots"`
}

type CreateSnapshotRequest struct {
	Name    string `json:"name,omitempty"`
	Comment string `json:"comment,omitempty"`
}

type SnapshotResponse struct {
	Message         string `json:"message"`
	SnapshotName    string `json:"snapshot_name"`
	InstanceName    string `json:"instance_name"`
	InstanceStopped bool   `json:"instance_stopped"`
	InstanceStarted bool   `json:"instance_started"`
}

type RestoreSnapshotRequest struct {
	Name string `json:"name,omitempty"`
}

type ExportInstanceRequest struct {
	OutputPath string `json:"output_path,omitempty"`
	Format     string `json:"format,omitempty"`
}

type ImportInstanceRequest struct {
	Name      string `json:"name,omitempty"`
	ImagePath string `json:"image_path"`
	CPUs      int    `json:"cpus,omitempty"`
	Memory    string `json:"memory,omitempty"`
	Disk      string `json:"disk,omitempty"`
}

type InstanceExport struct {
	Message   string `json:"message,omitempty"`
	ImagePath string `json:"image_path,omitempty"`
}

type MountRequest struct {
	SourcePath string `json:"source_path"`
	TargetPath string `json:"target_path"`
}

type UnmountRequest struct {
	TargetPath string `json:"target_path"`
}

type MountResponse struct {
	Message string `json:"message,omitempty"`
	Source  string `json:"source,omitempty"`
	Target  string `json:"target,omitempty"`
}

type ServerConfigResponse struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}

type SecurityConfigResponse struct {
	AllowedIPs          []string `json:"allowed_ips"`
	WebsocketTimeoutMin int      `json:"websocket_timeout_minutes"`
}

type MultipassConfigResponse struct {
	SocketPath        string `json:"socket_path"`
	DefaultTimeoutSec int    `json:"default_timeout_seconds"`
}

type LoggingConfigResponse struct {
	Level  string `json:"level"`
	Format string `json:"format"`
	Output string `json:"output"`
}

type ConfigResponse struct {
	Server    ServerConfigResponse    `json:"server"`
	Security  SecurityConfigResponse  `json:"security"`
	Multipass MultipassConfigResponse `json:"multipass"`
	Logging   LoggingConfigResponse   `json:"logging"`
}

type ConfigUpdateRequest struct {
	Server    *ServerUpdateRequest    `json:"server,omitempty"`
	Security  *SecurityUpdateRequest  `json:"security,omitempty"`
	Multipass *MultipassUpdateRequest `json:"multipass,omitempty"`
	Logging   *LoggingUpdateRequest   `json:"logging,omitempty"`
}

type ServerUpdateRequest struct {
	Host string `json:"host,omitempty"`
	Port int    `json:"port,omitempty"`
}

type SecurityUpdateRequest struct {
	AllowedIPs          []string `json:"allowed_ips,omitempty"`
	WebsocketTimeoutMin int      `json:"websocket_timeout_minutes,omitempty"`
}

type MultipassUpdateRequest struct {
	SocketPath        string `json:"socket_path,omitempty"`
	DefaultTimeoutSec int    `json:"default_timeout_seconds,omitempty"`
}

type LoggingUpdateRequest struct {
	Level  string `json:"level,omitempty"`
	Format string `json:"format,omitempty"`
	Output string `json:"output,omitempty"`
}
