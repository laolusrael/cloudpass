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
	Alias   string `json:"alias"`
	Version string `json:"version"`
	Release string `json:"release"`
	Remote  string `json:"remote"`
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
