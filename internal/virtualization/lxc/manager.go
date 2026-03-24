package lxc

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Manager provides LXC container management capabilities.
// It supports both Incus and LXD backends for container operations.
type Manager struct {
	mu          sync.RWMutex
	socketPath  string
	project     string
	remote      string
	backend     Backend
}

// Backend represents the LXC backend type.
type Backend string

const (
	BackendIncus Backend = "incus"
	BackendLXD   Backend = "lxd"
)

// ManagerOption is a functional option for Manager configuration.
type ManagerOption func(*Manager)

// WithSocketPath sets a custom socket path.
func WithSocketPath(path string) ManagerOption {
	return func(m *Manager) {
		m.socketPath = path
	}
}

// WithProject sets the LXC project.
func WithProject(project string) ManagerOption {
	return func(m *Manager) {
		m.project = project
	}
}

// WithRemote sets the LXC remote.
func WithRemote(remote string) ManagerOption {
	return func(m *Manager) {
		m.remote = remote
	}
}

// WithBackend sets the backend type explicitly.
func WithBackend(backend Backend) ManagerOption {
	return func(m *Manager) {
		m.backend = backend
	}
}

// NewManager creates a new LXC container manager.
func NewManager(opts ...ManagerOption) (*Manager, error) {
	m := &Manager{
		project: "default",
	}

	for _, opt := range opts {
		opt(m)
	}

	// Auto-detect backend if not specified
	if m.backend == "" {
		if m.detectBackend() != nil {
			return nil, fmt.Errorf("neither incus nor lxd found in PATH")
		}
	}

	// Set default socket path
	if m.socketPath == "" {
		switch m.backend {
		case BackendIncus:
			m.socketPath = "/var/lib/incus/unix.socket"
		case BackendLXD:
			m.socketPath = "/var/snap/lxd/common/lxd/unix.socket"
		}
	}

	return m, nil
}

// detectBackend detects whether Incus or LXD is available.
func (m *Manager) detectBackend() error {
	if _, err := exec.LookPath("incus"); err == nil {
		m.backend = BackendIncus
		return nil
	}
	if _, err := exec.LookPath("lxd"); err == nil {
		m.backend = BackendLXD
		return nil
	}
	return fmt.Errorf("no LXC backend found")
}

// cmd returns a configured exec.Cmd for the LXC CLI.
func (m *Manager) cmd(args ...string) *exec.Cmd {
	baseCmd := string(m.backend)

	// Add remote prefix if specified
	if m.remote != "" {
		args = append([]string{m.remote + ":"}, args...)
	}

	// Add project flag if not default
	if m.project != "" && m.project != "default" {
		args = append([]string{"--project", m.project}, args...)
	}

	return exec.Command(baseCmd, args...)
}

// cmdWithEnv returns an exec.Cmd with custom environment.
func (m *Manager) cmdWithEnv(env map[string]string, args ...string) *exec.Cmd {
	cmd := m.cmd(args...)
	if len(env) > 0 {
		cmd.Env = os.Environ()
		for k, v := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return cmd
}

// IsAvailable checks if the LXC backend is running.
func (m *Manager) IsAvailable() bool {
	cmd := m.cmd("info")
	return cmd.Run() == nil
}

// GetVersion returns the backend version information.
func (m *Manager) GetVersion() (map[string]string, error) {
	cmd := m.cmd("version", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get version: %w", err)
	}

	var raw struct {
		Client string `json:"client"`
		Server string `json:"server"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		// Try parsing non-JSON output
		lines := strings.Split(string(output), "\n")
		result := make(map[string]string)
		for _, line := range lines {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				result[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		return result, nil
	}

	return map[string]string{
		"client": raw.Client,
		"server": raw.Server,
		"backend": string(m.backend),
	}, nil
}

// ListContainers lists all containers.
func (m *Manager) ListContainers(ctx context.Context) ([]*Container, error) {
	cmd := m.cmd("list", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var raw []struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Status      string                 `json:"status"`
		Architecture string                `json:"architecture"`
		Profiles    []string               `json:"profiles"`
		Config      map[string]interface{} `json:"config"`
		Devices     map[string]interface{} `json:"devices"`
		ExpandedConfig map[string]interface{} `json:"expanded_config"`
		ExpandedDevices map[string]interface{} `json:"expanded_devices"`
		CreatedAt   time.Time              `json:"created_at"`
		LastUsedAt  time.Time              `json:"last_used_at"`
		State       struct {
			Status     string `json:"status"`
			Pid        int    `json:"pid"`
			Processes  int    `json:"processes"`
			Memory     struct {
				Usage  uint64 `json:"usage"`
				Limit  uint64 `json:"limit"`
			} `json:"memory"`
			Network map[string]struct {
				Addresses []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
					Netmask string `json:"netmask"`
				} `json:"addresses"`
			} `json:"network"`
		} `json:"state"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse container list: %w", err)
	}

	var containers []*Container
	for _, c := range raw {
		container := &Container{
			ID:          c.Name,
			Name:        c.Name,
			Description: c.Description,
			Status:      parseStatus(c.Status),
			CreatedAt:   c.CreatedAt,
			UpdatedAt:   c.LastUsedAt,
			Profiles:    c.Profiles,
			Arch:        c.Architecture,
			Tags:        make(map[string]string),
			Config:      make(map[string]string),
			Devices:     make(map[string]Device),
		}

		// Parse config
		for k, v := range c.ExpandedConfig {
			if vs, ok := v.(string); ok {
				container.Config[k] = vs
			}
		}

		// Extract image info
		if image, ok := container.Config["image.alias"]; ok {
			container.Image = image
		}

		// Parse resources from config
		container.Resources = parseResourcesFromConfig(container.Config)

		// Parse network
		container.Networks = parseNetworkFromState(c.State.Network)

		// Parse devices
		container.Devices = parseDevices(c.ExpandedDevices)

		containers = append(containers, container)
	}

	return containers, nil
}

// GetContainer retrieves a specific container by name.
func (m *Manager) GetContainer(ctx context.Context, name string) (*Container, error) {
	cmd := m.cmd("show", name, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get container %s: %w", name, err)
	}

	var raw struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		Status      string                 `json:"status"`
		Architecture string                `json:"architecture"`
		Profiles    []string               `json:"profiles"`
		Config      map[string]interface{} `json:"config"`
		Devices     map[string]interface{} `json:"devices"`
		ExpandedConfig map[string]interface{} `json:"expanded_config"`
		ExpandedDevices map[string]interface{} `json:"expanded_devices"`
		CreatedAt   time.Time              `json:"created_at"`
		LastUsedAt  time.Time              `json:"last_used_at"`
		Ephemeral   bool                   `json:"ephemeral"`
		Stateful    bool                   `json:"stateful"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse container info: %w", err)
	}

	container := &Container{
		ID:          raw.Name,
		Name:        raw.Name,
		Description: raw.Description,
		Status:      parseStatus(raw.Status),
		CreatedAt:   raw.CreatedAt,
		UpdatedAt:   raw.LastUsedAt,
		Profiles:    raw.Profiles,
		Arch:        raw.Architecture,
		Tags:        make(map[string]string),
		Config:      make(map[string]string),
		Devices:     make(map[string]Device),
	}

	// Parse config
	for k, v := range raw.ExpandedConfig {
		if vs, ok := v.(string); ok {
			container.Config[k] = vs
		}
	}

	// Extract image info
	if image, ok := container.Config["image.alias"]; ok {
		container.Image = image
	}

	// Parse resources
	container.Resources = parseResourcesFromConfig(container.Config)

	// Parse devices
	container.Devices = parseDevices(raw.ExpandedDevices)

	// Parse root disk from devices
	for name, dev := range container.Devices {
		if dev.Type == "disk" && dev.Target == "/" {
			container.RootDisk = StorageConfig{
				Name: name,
				Pool: dev.Config["pool"],
				Path: "/",
			}
			if size, ok := dev.Config["size"]; ok {
				container.RootDisk.Size = parseSize(size)
			}
		}
	}

	return container, nil
}

// CreateContainer creates a new container.
func (m *Manager) CreateContainer(ctx context.Context, config *CreateConfig) (*Container, error) {
	// Validate config
	if config.Name == "" {
		return nil, fmt.Errorf("container name is required")
	}
	if config.Image == "" {
		return nil, fmt.Errorf("image is required")
	}

	args := []string{"launch", config.Image, config.Name}

	// Add profiles
	for _, profile := range config.Profiles {
		args = append(args, "-p", profile)
	}

	// Add resource limits
	args = append(args, buildResourceArgs(&config.Resources)...)

	// Add network configuration
	for _, net := range config.Networks {
		args = append(args, buildNetworkArgs(&net)...)
	}

	// Add storage configuration
	if config.RootDisk.Size > 0 {
		args = append(args, "--device", fmt.Sprintf("root,disk,size=%dGB", config.RootDisk.Size))
	}

	// Add volumes
	for _, vol := range config.Volumes {
		args = append(args, "--device", buildVolumeDevice(&vol))
	}

	// Add config options
	for k, v := range config.Config {
		args = append(args, "--config", fmt.Sprintf("%s=%s", k, v))
	}

	// Add security options
	if config.Privileged {
		args = append(args, "--config", "security.privileged=true")
	}
	for _, opt := range config.Security {
		args = append(args, "--config", opt)
	}

	// Add tags
	for k, v := range config.Tags {
		args = append(args, "--config", fmt.Sprintf("user.%s=%s", k, v))
	}

	// Set auto-start
	if config.AutoStart {
		args = append(args, "--config", "boot.autostart=true")
	}

	cmd := m.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w, output: %s", err, string(output))
	}

	return m.GetContainer(ctx, config.Name)
}

// StartContainer starts a container.
func (m *Manager) StartContainer(ctx context.Context, name string) error {
	cmd := m.cmd("start", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start container %s: %w, output: %s", name, err, string(output))
	}
	return nil
}

// StopContainer stops a container.
func (m *Manager) StopContainer(ctx context.Context, name string, force bool, timeout int) error {
	args := []string{"stop", name}
	if force {
		args = append(args, "--force")
	}
	if timeout > 0 {
		args = append(args, "--timeout", fmt.Sprintf("%d", timeout))
	}

	cmd := m.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop container %s: %w, output: %s", name, err, string(output))
	}
	return nil
}

// RestartContainer restarts a container.
func (m *Manager) RestartContainer(ctx context.Context, name string, force bool, timeout int) error {
	args := []string{"restart", name}
	if force {
		args = append(args, "--force")
	}
	if timeout > 0 {
		args = append(args, "--timeout", fmt.Sprintf("%d", timeout))
	}

	cmd := m.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restart container %s: %w, output: %s", name, err, string(output))
	}
	return nil
}

// PauseContainer pauses (freezes) a container.
func (m *Manager) PauseContainer(ctx context.Context, name string) error {
	cmd := m.cmd("pause", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pause container %s: %w, output: %s", name, err, string(output))
	}
	return nil
}

// ResumeContainer resumes (unfreezes) a container.
func (m *Manager) ResumeContainer(ctx context.Context, name string) error {
	cmd := m.cmd("resume", name)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to resume container %s: %w, output: %s", name, err, string(output))
	}
	return nil
}

// DeleteContainer deletes a container.
func (m *Manager) DeleteContainer(ctx context.Context, name string, force bool) error {
	args := []string{"delete", name}
	if force {
		args = append(args, "--force")
	}

	cmd := m.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete container %s: %w, output: %s", name, err, string(output))
	}
	return nil
}

// UpdateContainer updates a container's configuration.
func (m *Manager) UpdateContainer(ctx context.Context, name string, updates map[string]string) error {
	args := []string{"config", "set", name}
	for k, v := range updates {
		args = append(args, fmt.Sprintf("%s=%s", k, v))
	}

	cmd := m.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update container %s: %w, output: %s", name, err, string(output))
	}
	return nil
}

// UpdateResources updates container resource limits.
func (m *Manager) UpdateResources(ctx context.Context, name string, resources *ResourceConfig) error {
	args := []string{"config", "set", name}
	args = append(args, buildResourceArgs(resources)...)

	if len(args) <= 3 {
		return nil // No resource updates
	}

	cmd := m.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update resources for %s: %w, output: %s", name, err, string(output))
	}
	return nil
}

// GetStats retrieves container statistics.
func (m *Manager) GetStats(ctx context.Context, name string) (*Stats, error) {
	cmd := m.cmd("info", name, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get stats for %s: %w", name, err)
	}

	var raw struct {
		Status    string `json:"status"`
		State     struct {
			Pid        int `json:"pid"`
			Processes  int `json:"processes"`
			CPU        struct {
				Usage int64 `json:"usage"`
			} `json:"cpu"`
			Memory struct {
				Usage  uint64 `json:"usage"`
				Limit  uint64 `json:"limit"`
				SwapUsage uint64 `json:"swap_usage"`
			} `json:"memory"`
			Network map[string]struct {
				Counters struct {
					BytesReceived uint64 `json:"bytes_received"`
					BytesSent     uint64 `json:"bytes_sent"`
				} `json:"counters"`
			} `json:"network"`
			Disk struct {
				Root struct {
					Usage uint64 `json:"usage"`
				} `json:"root"`
			} `json:"disk"`
			StartedAt time.Time `json:"started_at"`
		} `json:"state"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse stats: %w", err)
	}

	stats := &Stats{
		MemoryUsage:  raw.State.Memory.Usage / (1024 * 1024), // Convert to MB
		MemoryLimit:  raw.State.Memory.Limit / (1024 * 1024),
		ProcessCount: raw.State.Processes,
		Timestamp:    time.Now(),
	}

	// Calculate uptime
	if !raw.State.StartedAt.IsZero() {
		stats.Uptime = int64(time.Since(raw.State.StartedAt).Seconds())
	}

	// Sum network I/O
	for _, net := range raw.State.Network {
		stats.NetworkRx += net.Counters.BytesReceived / (1024 * 1024)
		stats.NetworkTx += net.Counters.BytesSent / (1024 * 1024)
	}

	// Disk usage
	stats.DiskRead = raw.State.Disk.Root.Usage / (1024 * 1024)

	// Calculate CPU usage (simplified)
	if raw.State.CPU.Usage > 0 {
		stats.CPUUsage = float64(raw.State.CPU.Usage) / 1e9 // Convert from nanoseconds
	}

	return stats, nil
}

// Exec runs a command in a container.
func (m *Manager) Exec(ctx context.Context, name string, config *ExecConfig) (*ExecResult, error) {
	args := []string{"exec", name, "--"}

	// Add environment variables
	for k, v := range config.Environment {
		args = append(args, "--env", fmt.Sprintf("%s=%s", k, v))
	}

	// Add working directory
	if config.WorkingDir != "" {
		args = append(args, "--cwd", config.WorkingDir)
	}

	// Add user/group
	if config.User != "" {
		args = append(args, "--user", config.User)
	}
	if config.Group != "" {
		args = append(args, "--group", config.Group)
	}

	// Add command
	args = append(args, config.Command...)

	cmd := m.cmd(args...)
	startTime := time.Now()

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	duration := time.Since(startTime).Milliseconds()

	result := &ExecResult{
		Duration: duration,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
			result.Stderr = err.Error()
		}
	}

	return result, nil
}

// GetLogs retrieves container logs.
func (m *Manager) GetLogs(ctx context.Context, name string, tail int) ([]LogEntry, error) {
	args := []string{"info", name, "--log"}
	if tail > 0 {
		args = append(args, fmt.Sprintf("--tail=%d", tail))
	}

	cmd := m.cmd(args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get logs for %s: %w", name, err)
	}

	var logs []LogEntry
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		logs = append(logs, LogEntry{
			Timestamp: time.Now(),
			Message:   line,
			Level:     "info",
		})
	}

	return logs, nil
}

// CreateSnapshot creates a container snapshot.
func (m *Manager) CreateSnapshot(ctx context.Context, name, snapshotName string, stateful bool) (*Snapshot, error) {
	args := []string{"snapshot", "create", name, snapshotName}
	if stateful {
		args = append(args, "--stateful")
	}

	cmd := m.cmd(args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w, output: %s", err, string(output))
	}

	return m.GetSnapshot(ctx, name, snapshotName)
}

// GetSnapshot retrieves a specific snapshot.
func (m *Manager) GetSnapshot(ctx context.Context, name, snapshotName string) (*Snapshot, error) {
	snapshots, err := m.ListSnapshots(ctx, name)
	if err != nil {
		return nil, err
	}

	for _, snap := range snapshots {
		if snap.Name == snapshotName {
			return snap, nil
		}
	}

	return nil, fmt.Errorf("snapshot %s not found", snapshotName)
}

// ListSnapshots lists all snapshots for a container.
func (m *Manager) ListSnapshots(ctx context.Context, name string) ([]*Snapshot, error) {
	cmd := m.cmd("snapshot", "list", name, "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	var raw []struct {
		Name      string    `json:"name"`
		CreatedAt time.Time `json:"created_at"`
		Stateful  bool      `json:"stateful"`
		Size      uint64    `json:"size"`
	}

	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse snapshots: %w", err)
	}

	var snapshots []*Snapshot
	for _, s := range raw {
		snapshots = append(snapshots, &Snapshot{
			Name:      s.Name,
			CreatedAt: s.CreatedAt,
			Stateful:  s.Stateful,
			Size:      s.Size / (1024 * 1024), // Convert to MB
		})
	}

	return snapshots, nil
}

// RestoreSnapshot restores a container to a snapshot.
func (m *Manager) RestoreSnapshot(ctx context.Context, name, snapshotName string) error {
	cmd := m.cmd("snapshot", "restore", name, snapshotName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to restore snapshot: %w, output: %s", err, string(output))
	}
	return nil
}

// DeleteSnapshot deletes a snapshot.
func (m *Manager) DeleteSnapshot(ctx context.Context, name, snapshotName string) error {
	cmd := m.cmd("snapshot", "delete", name, snapshotName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w, output: %s", err, string(output))
	}
	return nil
}

// FilePush pushes a file to a container.
func (m *Manager) FilePush(ctx context.Context, name, src, dest string) error {
	cmd := m.cmd("file", "push", src, fmt.Sprintf("%s%s", name, dest))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push file: %w, output: %s", err, string(output))
	}
	return nil
}

// FilePull pulls a file from a container.
func (m *Manager) FilePull(ctx context.Context, name, src, dest string) error {
	cmd := m.cmd("file", "pull", fmt.Sprintf("%s%s", name, src), dest)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pull file: %w, output: %s", err, string(output))
	}
	return nil
}

// parseStatus converts a status string to ContainerStatus.
func parseStatus(status string) ContainerStatus {
	switch strings.ToLower(status) {
	case "running":
		return StatusRunning
	case "stopped":
		return StatusStopped
	case "frozen":
		return StatusFrozen
	case "starting":
		return StatusStarting
	case "stopping":
		return StatusStopping
	case "error":
		return StatusError
	default:
		return StatusStopped
	}
}

// parseResourcesFromConfig extracts resource config from container config.
func parseResourcesFromConfig(config map[string]string) ResourceConfig {
	resources := ResourceConfig{}

	if limit, ok := config["limits.cpu"]; ok {
		if n, err := strconv.Atoi(limit); err == nil {
			resources.CPUCores = n
		}
	}

	if limit, ok := config["limits.memory"]; ok {
		resources.MemoryLimit = parseSize(limit)
	}

	if priority, ok := config["limits.cpu.priority"]; ok {
		if n, err := strconv.Atoi(priority); err == nil {
			resources.CPUPriority = n
		}
	}

	return resources
}

// parseNetworkFromState extracts network info from state.
func parseNetworkFromState(state map[string]struct {
	Addresses []struct {
		Family  string `json:"family"`
		Address string `json:"address"`
		Netmask string `json:"netmask"`
	} `json:"addresses"`
}) []NetworkConfig {
	var networks []NetworkConfig

	for name, net := range state {
		config := NetworkConfig{Name: name}
		for _, addr := range net.Addresses {
			if addr.Family == "inet" {
				config.IPAddress = addr.Address
				config.Subnet = addr.Netmask
			}
		}
		networks = append(networks, config)
	}

	return networks
}

// parseDevices extracts devices from raw data.
func parseDevices(raw map[string]interface{}) map[string]Device {
	devices := make(map[string]Device)

	for name, d := range raw {
		if devMap, ok := d.(map[string]interface{}); ok {
			device := Device{
				Config: make(map[string]string),
			}
			if t, ok := devMap["type"].(string); ok {
				device.Type = t
			}
			if source, ok := devMap["source"].(string); ok {
				device.Source = source
			}
			if target, ok := devMap["path"].(string); ok {
				device.Target = target
			}
			for k, v := range devMap {
				if vs, ok := v.(string); ok && k != "type" && k != "source" && k != "path" {
					device.Config[k] = vs
				}
			}
			devices[name] = device
		}
	}

	return devices
}

// parseSize parses a size string (e.g., "1GB", "512MB") to uint64 in MB.
func parseSize(s string) uint64 {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)

	re := regexp.MustCompile(`^(\d+(?:\.\d+)?)([KMGTP]?B?)?$`)
	matches := re.FindStringSubmatch(s)
	if len(matches) < 2 {
		n, _ := strconv.ParseUint(s, 10, 64)
		return n
	}

	value, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0
	}

	unit := ""
	if len(matches) > 2 {
		unit = matches[2]
	}

	switch unit {
	case "KB", "K":
		return uint64(value / 1024)
	case "MB", "M":
		return uint64(value)
	case "GB", "G":
		return uint64(value * 1024)
	case "TB", "T":
		return uint64(value * 1024 * 1024)
	case "PB", "P":
		return uint64(value * 1024 * 1024 * 1024)
	default:
		return uint64(value / (1024 * 1024))
	}
}

// buildResourceArgs builds CLI args from ResourceConfig.
func buildResourceArgs(r *ResourceConfig) []string {
	var args []string

	if r.CPUCores > 0 {
		args = append(args, "--config", fmt.Sprintf("limits.cpu=%d", r.CPUCores))
	}
	if r.CPUPriority > 0 {
		args = append(args, "--config", fmt.Sprintf("limits.cpu.priority=%d", r.CPUPriority))
	}
	if r.MemoryLimit > 0 {
		args = append(args, "--config", fmt.Sprintf("limits.memory=%dMB", r.MemoryLimit))
	}
	if r.MemorySwapLimit > 0 {
		args = append(args, "--config", fmt.Sprintf("limits.memory.swap=%dMB", r.MemorySwapLimit))
	}
	if r.DiskReadRate > 0 {
		args = append(args, "--config", fmt.Sprintf("limits.disk.read=%dMB", r.DiskReadRate))
	}
	if r.DiskWriteRate > 0 {
		args = append(args, "--config", fmt.Sprintf("limits.disk.write=%dMB", r.DiskWriteRate))
	}
	if r.NetworkIngress > 0 {
		args = append(args, "--config", fmt.Sprintf("limits.network.ingress=%dMbit", r.NetworkIngress))
	}
	if r.NetworkEgress > 0 {
		args = append(args, "--config", fmt.Sprintf("limits.network.egress=%dMbit", r.NetworkEgress))
	}
	if r.ProcessLimit > 0 {
		args = append(args, "--config", fmt.Sprintf("limits.processes=%d", r.ProcessLimit))
	}

	return args
}

// buildNetworkArgs builds CLI args from NetworkConfig.
func buildNetworkArgs(n *NetworkConfig) []string {
	var args []string

	if n.Network != "" {
		deviceConfig := fmt.Sprintf("eth0,nic,network=%s", n.Network)
		if n.IPAddress != "" {
			deviceConfig += fmt.Sprintf(",ipv4.address=%s", n.IPAddress)
		}
		if n.MAC != "" {
			deviceConfig += fmt.Sprintf(",hwaddr=%s", n.MAC)
		}
		args = append(args, "--device", deviceConfig)
	}

	return args
}

// buildVolumeDevice builds a device string for a volume.
func buildVolumeDevice(v *StorageConfig) string {
	config := fmt.Sprintf("%s,disk", v.Name)
	if v.Pool != "" {
		config += fmt.Sprintf(",pool=%s", v.Pool)
	}
	if v.Path != "" {
		config += fmt.Sprintf(",path=%s", v.Path)
	}
	if v.Size > 0 {
		config += fmt.Sprintf(",size=%dGB", v.Size)
	}
	if v.ReadOnly {
		config += ",readonly=true"
	}
	return config
}