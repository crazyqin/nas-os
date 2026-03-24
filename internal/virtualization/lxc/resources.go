package lxc

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ResourceManager handles container resource configuration.
type ResourceManager struct {
	manager *Manager
}

// NewResourceManager creates a new ResourceManager.
func NewResourceManager(manager *Manager) *ResourceManager {
	return &ResourceManager{manager: manager}
}

// SetCPULimit sets the CPU limit for a container.
// count can be:
//   - A number (e.g., 2) - number of CPU cores
//   - A percentage (e.g., 50%) - percentage of a single core
//   - "balanced" - auto-balance among containers
func (r *ResourceManager) SetCPULimit(ctx context.Context, name string, count string) error {
	return r.manager.UpdateContainer(ctx, name, map[string]string{
		"limits.cpu": count,
	})
}

// SetCPUPriority sets the CPU priority (1-10, higher is more important).
func (r *ResourceManager) SetCPUPriority(ctx context.Context, name string, priority int) error {
	if priority < 1 || priority > 10 {
		return fmt.Errorf("CPU priority must be between 1 and 10")
	}
	return r.manager.UpdateContainer(ctx, name, map[string]string{
		"limits.cpu.priority": strconv.Itoa(priority),
	})
}

// SetMemoryLimit sets the memory limit for a container.
// limit should be specified with unit (e.g., "512MB", "2GB") or as bytes.
func (r *ResourceManager) SetMemoryLimit(ctx context.Context, name string, limit string) error {
	// Validate format
	if _, err := parseSizeToBytes(limit); err != nil {
		return fmt.Errorf("invalid memory limit format: %w", err)
	}
	return r.manager.UpdateContainer(ctx, name, map[string]string{
		"limits.memory": limit,
	})
}

// SetMemorySwap sets the swap limit (or disables swap).
// Set to "false" to disable swap, or specify size with unit.
func (r *ResourceManager) SetMemorySwap(ctx context.Context, name string, swap string) error {
	return r.manager.UpdateContainer(ctx, name, map[string]string{
		"limits.memory.swap": swap,
	})
}

// SetDiskIOLimit sets disk I/O limits.
func (r *ResourceManager) SetDiskIOLimit(ctx context.Context, name string, readMBs, writeMBs uint64) error {
	updates := make(map[string]string)
	if readMBs > 0 {
		updates["limits.disk.read"] = fmt.Sprintf("%dMB", readMBs)
	}
	if writeMBs > 0 {
		updates["limits.disk.write"] = fmt.Sprintf("%dMB", writeMBs)
	}
	if len(updates) == 0 {
		return nil
	}
	return r.manager.UpdateContainer(ctx, name, updates)
}

// SetNetworkLimit sets network bandwidth limits.
func (r *ResourceManager) SetNetworkLimit(ctx context.Context, name string, ingressMbps, egressMbps uint64) error {
	updates := make(map[string]string)
	if ingressMbps > 0 {
		updates["limits.network.ingress"] = fmt.Sprintf("%dMbit", ingressMbps)
	}
	if egressMbps > 0 {
		updates["limits.network.egress"] = fmt.Sprintf("%dMbit", egressMbps)
	}
	if len(updates) == 0 {
		return nil
	}
	return r.manager.UpdateContainer(ctx, name, updates)
}

// SetProcessLimit sets the maximum number of processes.
func (r *ResourceManager) SetProcessLimit(ctx context.Context, name string, limit int) error {
	return r.manager.UpdateContainer(ctx, name, map[string]string{
		"limits.processes": strconv.Itoa(limit),
	})
}

// GetResourceUsage retrieves current resource usage for a container.
func (r *ResourceManager) GetResourceUsage(ctx context.Context, name string) (*ResourceUsage, error) {
	stats, err := r.manager.GetStats(ctx, name)
	if err != nil {
		return nil, err
	}

	container, err := r.manager.GetContainer(ctx, name)
	if err != nil {
		return nil, err
	}

	usage := &ResourceUsage{
		CPU: CPUMetrics{
			UsagePercent: stats.CPUUsage,
			Limit:        container.Resources.CPUCores,
		},
		Memory: MemoryMetrics{
			UsedMB:  stats.MemoryUsage,
			LimitMB: container.Resources.MemoryLimit,
			CacheMB: stats.MemoryCache,
		},
		Disk: DiskMetrics{
			ReadMB:  stats.DiskRead,
			WriteMB: stats.DiskWrite,
		},
		Network: NetworkMetrics{
			RxMB: stats.NetworkRx,
			TxMB: stats.NetworkTx,
		},
		Processes: stats.ProcessCount,
	}

	if container.Resources.MemoryLimit > 0 {
		usage.Memory.UsagePercent = float64(stats.MemoryUsage) / float64(container.Resources.MemoryLimit) * 100
	}

	return usage, nil
}

// ResourceUsage contains detailed resource usage metrics.
type ResourceUsage struct {
	CPU       CPUMetrics    `json:"cpu"`
	Memory    MemoryMetrics `json:"memory"`
	Disk      DiskMetrics   `json:"disk"`
	Network   NetworkMetrics `json:"network"`
	Processes int           `json:"processes"`
}

// CPUMetrics contains CPU usage metrics.
type CPUMetrics struct {
	UsagePercent float64 `json:"usagePercent"`
	Limit        int     `json:"limit"` // 0 = unlimited
	Priority     int     `json:"priority"`
}

// MemoryMetrics contains memory usage metrics.
type MemoryMetrics struct {
	UsedMB       uint64  `json:"usedMB"`
	LimitMB      uint64  `json:"limitMB"` // 0 = unlimited
	CacheMB      uint64  `json:"cacheMB"`
	UsagePercent float64 `json:"usagePercent"`
}

// DiskMetrics contains disk I/O metrics.
type DiskMetrics struct {
	ReadMB   uint64 `json:"readMB"`
	WriteMB  uint64 `json:"writeMB"`
	ReadRate uint64 `json:"readRate"`  // MB/s limit
	WriteRate uint64 `json:"writeRate"` // MB/s limit
}

// NetworkMetrics contains network metrics.
type NetworkMetrics struct {
	RxMB       uint64 `json:"rxMB"`
	TxMB       uint64 `json:"txMB"`
	IngressMbps uint64 `json:"ingressMbps"` // Limit
	EgressMbps  uint64 `json:"egressMbps"`  // Limit
}

// parseSizeToBytes parses a size string to bytes.
func parseSizeToBytes(s string) (uint64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}

	// Try parsing as plain number
	if n, err := strconv.ParseUint(s, 10, 64); err == nil {
		return n, nil
	}

	// Parse with unit
	s = strings.ToUpper(s)
	var multiplier uint64 = 1

	switch {
	case strings.HasSuffix(s, "TB"):
		multiplier = 1024 * 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "TB")
	case strings.HasSuffix(s, "GB"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "GB")
	case strings.HasSuffix(s, "MB"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "MB")
	case strings.HasSuffix(s, "KB"):
		multiplier = 1024
		s = strings.TrimSuffix(s, "KB")
	case strings.HasSuffix(s, "T"):
		multiplier = 1024 * 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "T")
	case strings.HasSuffix(s, "G"):
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "G")
	case strings.HasSuffix(s, "M"):
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "M")
	case strings.HasSuffix(s, "K"):
		multiplier = 1024
		s = strings.TrimSuffix(s, "K")
	}

	value, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %s", s)
	}

	return uint64(value * float64(multiplier)), nil
}

// ValidateResourceConfig validates a resource configuration.
func ValidateResourceConfig(config *ResourceConfig) error {
	if config.CPUCores < 0 {
		return fmt.Errorf("CPU cores cannot be negative")
	}
	if config.CPUPriority < 0 || config.CPUPriority > 10 {
		return fmt.Errorf("CPU priority must be between 0 and 10")
	}
	if config.MemoryLimit < 0 {
		return fmt.Errorf("memory limit cannot be negative")
	}
	if config.DiskReadRate < 0 || config.DiskWriteRate < 0 {
		return fmt.Errorf("disk I/O rates cannot be negative")
	}
	if config.NetworkIngress < 0 || config.NetworkEgress < 0 {
		return fmt.Errorf("network limits cannot be negative")
	}
	if config.ProcessLimit < 0 {
		return fmt.Errorf("process limit cannot be negative")
	}
	return nil
}

// DefaultResourceConfig returns a default resource configuration.
func DefaultResourceConfig() ResourceConfig {
	return ResourceConfig{
		CPUCores:    1,
		CPUPriority: 5,
		MemoryLimit: 1024, // 1GB
	}
}

// MinimalResourceConfig returns a minimal resource configuration.
func MinimalResourceConfig() ResourceConfig {
	return ResourceConfig{
		CPUCores:    1,
		MemoryLimit: 512, // 512MB
	}
}

// HighPerformanceResourceConfig returns a high-performance resource configuration.
func HighPerformanceResourceConfig() ResourceConfig {
	return ResourceConfig{
		CPUCores:     4,
		CPUPriority:  10,
		MemoryLimit:  8192, // 8GB
		ProcessLimit: 4096,
	}
}