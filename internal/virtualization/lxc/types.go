// Package lxc provides LXC container management for nas-os.
// This module supports Incus/LXD-style container operations including
// creation, lifecycle management, resource limits, and network configuration.
package lxc

import "time"

// ContainerStatus represents the current status of an LXC container.
type ContainerStatus string

// Container status constants.
const (
	StatusRunning  ContainerStatus = "running"
	StatusStopped  ContainerStatus = "stopped"
	StatusStarting ContainerStatus = "starting"
	StatusStopping ContainerStatus = "stopping"
	StatusFrozen   ContainerStatus = "frozen"
	StatusError    ContainerStatus = "error"
	StatusCreating ContainerStatus = "creating"
	StatusDeleting ContainerStatus = "deleting"
)

// Container represents an LXC container instance.
type Container struct {
	// Basic information
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Status      ContainerStatus `json:"status"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`

	// Image/Template
	Image    string   `json:"image"`    // Image name or alias
	OSType   string   `json:"osType"`   // linux/windows
	Arch     string   `json:"arch"`     // x86_64, aarch64, etc.
	Profiles []string `json:"profiles"` // Configuration profiles

	// Resource limits
	Resources ResourceConfig `json:"resources"`

	// Network configuration
	Networks []NetworkConfig `json:"networks"`

	// Storage
	RootDisk StorageConfig   `json:"rootDisk"`
	Volumes  []StorageConfig `json:"volumes"`

	// Security
	Privileged   bool     `json:"privileged"`
	SecurityOpts []string `json:"securityOpts"`

	// Snapshots
	Snapshots []Snapshot `json:"snapshots"`

	// Metadata
	Tags    map[string]string `json:"tags"`
	Config  map[string]string `json:"config"`
	Devices map[string]Device `json:"devices"`
}

// ResourceConfig defines resource limits for a container.
type ResourceConfig struct {
	// CPU limits
	CPUCores    int    `json:"cpuCores"`    // Number of CPU cores
	CPUPriority int    `json:"cpuPriority"` // CPU priority (1-10)
	CPULimit    string `json:"cpuLimit"`    // e.g., "50%" or "2"

	// Memory limits
	MemoryLimit     uint64 `json:"memoryLimit"`     // Memory limit in MB
	MemorySwapLimit uint64 `json:"memorySwapLimit"` // Swap limit in MB (0 = unlimited)

	// Disk I/O limits
	DiskReadRate  uint64 `json:"diskReadRate"`  // Read rate limit in MB/s (0 = unlimited)
	DiskWriteRate uint64 `json:"diskWriteRate"` // Write rate limit in MB/s (0 = unlimited)

	// Network limits
	NetworkIngress uint64 `json:"networkIngress"` // Ingress rate limit in Mbps (0 = unlimited)
	NetworkEgress  uint64 `json:"networkEgress"`  // Egress rate limit in Mbps (0 = unlimited)

	// Process limits
	ProcessLimit int `json:"processLimit"` // Max number of processes (0 = unlimited)
}

// NetworkConfig defines network settings for a container.
type NetworkConfig struct {
	Name      string `json:"name"`      // Network interface name
	Type      string `json:"type"`      // bridge, macvlan, ipvlan, physical
	Network   string `json:"network"`   // Network name (e.g., "lxdbr0")
	IPAddress string `json:"ipAddress"` // Static IP address (e.g., "192.168.1.100")
	Gateway   string `json:"gateway"`   // Gateway address
	Subnet    string `json:"subnet"`    // Subnet mask or CIDR
	DNS       string `json:"dns"`       // DNS servers
	MAC       string `json:"mac"`       // MAC address
	MTU       int    `json:"mtu"`       // MTU size

	// Host access
	HostAccess bool `json:"hostAccess"` // Allow host network access
}

// StorageConfig defines storage settings for a container.
type StorageConfig struct {
	Name       string `json:"name"`       // Volume name
	Pool       string `json:"pool"`       // Storage pool name
	Size       uint64 `json:"size"`       // Size in GB
	Path       string `json:"path"`       // Mount path in container
	SourceType string `json:"sourceType"` // "volume", "dir", "block"
	ReadOnly   bool   `json:"readOnly"`   // Read-only mount
}

// Device represents a device passthrough configuration.
type Device struct {
	Type   string            `json:"type"`   // Device type
	Source string            `json:"source"` // Source path/device
	Target string            `json:"target"` // Target path in container
	Config map[string]string `json:"config"` // Additional device config
}

// Snapshot represents a container snapshot.
type Snapshot struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
	Size        uint64    `json:"size"`     // Size in MB
	Stateful    bool      `json:"stateful"` // Includes runtime state
}

// CreateConfig holds parameters for creating a new container.
type CreateConfig struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Image       string            `json:"image"`      // Image/alias name
	Profiles    []string          `json:"profiles"`   // Profiles to apply
	Resources   ResourceConfig    `json:"resources"`  // Resource limits
	Networks    []NetworkConfig   `json:"networks"`   // Network config
	RootDisk    StorageConfig     `json:"rootDisk"`   // Root disk config
	Volumes     []StorageConfig   `json:"volumes"`    // Additional volumes
	Privileged  bool              `json:"privileged"` // Run as privileged
	Security    []string          `json:"security"`   // Security options
	Config      map[string]string `json:"config"`     // Additional config
	Devices     map[string]Device `json:"devices"`    // Additional devices
	Tags        map[string]string `json:"tags"`       // Tags/labels
	AutoStart   bool              `json:"autoStart"`  // Start on boot
}

// Stats holds real-time container statistics.
type Stats struct {
	CPUUsage     float64   `json:"cpuUsage"`     // CPU usage percentage
	MemoryUsage  uint64    `json:"memoryUsage"`  // Memory usage in MB
	MemoryCache  uint64    `json:"memoryCache"`  // Cache memory in MB
	MemoryLimit  uint64    `json:"memoryLimit"`  // Memory limit in MB
	DiskRead     uint64    `json:"diskRead"`     // Total disk read in MB
	DiskWrite    uint64    `json:"diskWrite"`    // Total disk write in MB
	NetworkRx    uint64    `json:"networkRx"`    // Total network RX in MB
	NetworkTx    uint64    `json:"networkTx"`    // Total network TX in MB
	ProcessCount int       `json:"processCount"` // Number of processes
	Uptime       int64     `json:"uptime"`       // Uptime in seconds
	Timestamp    time.Time `json:"timestamp"`    // Measurement timestamp
}

// Image represents a container image/template.
type Image struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	OS          string            `json:"os"`      // e.g., "ubuntu", "alpine"
	Release     string            `json:"release"` // e.g., "22.04", "3.18"
	Arch        string            `json:"arch"`    // Architecture
	Variant     string            `json:"variant"` // e.g., "default", "cloud"
	Size        uint64            `json:"size"`    // Size in MB
	CreatedAt   time.Time         `json:"createdAt"`
	Properties  map[string]string `json:"properties"` // Image properties
	Aliases     []string          `json:"aliases"`    // Image aliases
}

// Network represents an LXC network.
type Network struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"` // bridge, macvlan, etc.
	Description string            `json:"description"`
	Subnet      string            `json:"subnet"`   // IPv4 subnet
	Subnet6     string            `json:"subnet6"`  // IPv6 subnet
	Gateway     string            `json:"gateway"`  // IPv4 gateway
	Gateway6    string            `json:"gateway6"` // IPv6 gateway
	DHCP        bool              `json:"dhcp"`     // DHCP enabled
	DNS         string            `json:"dns"`      // DNS servers
	Managed     bool              `json:"managed"`  // Managed by LXC
	InUse       bool              `json:"inUse"`    // In use by containers
	Config      map[string]string `json:"config"`   // Network config
}

// StoragePool represents an LXC storage pool.
type StoragePool struct {
	Name        string            `json:"name"`
	Driver      string            `json:"driver"` // zfs, btrfs, dir, lvm, etc.
	Description string            `json:"description"`
	TotalSize   uint64            `json:"totalSize"` // Total size in GB
	UsedSize    uint64            `json:"usedSize"`  // Used size in GB
	Available   uint64            `json:"available"` // Available size in GB
	InUse       bool              `json:"inUse"`     // In use by containers
	Config      map[string]string `json:"config"`    // Pool config
}

// Profile represents an LXC profile (configuration template).
type Profile struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Config      map[string]string `json:"config"`  // Profile config
	Devices     map[string]Device `json:"devices"` // Profile devices
	UsedBy      []string          `json:"usedBy"`  // Containers using this profile
}

// ExecConfig holds parameters for executing commands in a container.
type ExecConfig struct {
	Command     []string          `json:"command"`     // Command to execute
	Environment map[string]string `json:"environment"` // Environment variables
	WorkingDir  string            `json:"workingDir"`  // Working directory
	User        string            `json:"user"`        // User to run as (e.g., "root", "1000")
	Group       string            `json:"group"`       // Group to run as
	Interactive bool              `json:"interactive"` // Interactive mode
	Width       int               `json:"width"`       // Terminal width
	Height      int               `json:"height"`      // Terminal height
	RecordOut   bool              `json:"recordOut"`   // Record stdout
	RecordErr   bool              `json:"recordErr"`   // Record stderr
}

// ExecResult holds the result of a command execution.
type ExecResult struct {
	ExitCode int    `json:"exitCode"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	Duration int64  `json:"duration"` // Duration in milliseconds
}

// MigrationConfig holds parameters for container migration.
type MigrationConfig struct {
	TargetHost    string `json:"targetHost"`    // Target host address
	TargetPort    int    `json:"targetPort"`    // Target host port (default 8443)
	Live          bool   `json:"live"`          // Live migration
	Stateful      bool   `json:"stateful"`      // Stateful migration
	AllowInsecure bool   `json:"allowInsecure"` // Allow insecure connection
}

// BackupConfig holds parameters for container backup.
type BackupConfig struct {
	Name             string     `json:"name"`
	StoragePool      string     `json:"storagePool"`
	Compression      string     `json:"compression"`      // gzip, bzip2, xz, none
	InstanceOnly     bool       `json:"instanceOnly"`     // Only the instance, no snapshots
	OptimizedStorage bool       `json:"optimizedStorage"` // Use optimized storage
	Expiration       *time.Time `json:"expiration"`       // Backup expiration time
}

// LogEntry represents a container log entry.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // info, warn, error
	Message   string    `json:"message"`
	Source    string    `json:"source"` // Source component
}

// Operation represents a long-running operation.
type Operation struct {
	ID          string    `json:"id"`
	Description string    `json:"description"`
	Status      string    `json:"status"`   // running, success, failure
	Progress    int       `json:"progress"` // Progress percentage (0-100)
	Err         string    `json:"error"`    // Error message if failed
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
