// Package nvme_pool implements NVMe-optimized storage pool management
package nvme_pool

import (
	"context"
	"fmt"
	"time"
)

// PoolType defines NVMe pool types
type PoolType string

const (
	PoolTypeAllFlash PoolType = "all_flash" // 全NVMe池
	PoolTypeHybrid   PoolType = "hybrid"    // NVMe缓存+HDD存储
	PoolTypeFusion   PoolType = "fusion"    // 元数据在NVMe
)

// PoolConfig defines NVMe pool configuration
type PoolConfig struct {
	Name          string   `json:"name"`
	Type          PoolType `json:"type"`
	NVMeDevices   []string `json:"nvme_devices"`
	HDDDevices    []string `json:"hdd_devices,omitempty"`
	CacheRatio    float64  `json:"cache_ratio"`   // 缓存比例
	BlockSize     int      `json:"block_size"`    // 块大小
	Compression   bool     `json:"compression"`   // 压缩启用
	Deduplication bool     `json:"deduplication"` // 去重启用
}

// NVMePool represents an NVMe-optimized storage pool
type NVMePool struct {
	config      *PoolConfig
	nvmeDevices []*NVMeDevice
	status      PoolStatus
	createdAt   time.Time
}

// NVMeDevice represents an NVMe SSD
type NVMeDevice struct {
	Path        string `json:"path"`
	Model       string `json:"model"`
	Size        int64  `json:"size"`        // bytes
	Temperature int    `json:"temperature"` // Celsius
	Health      int    `json:"health"`      // 0-100%
	WriteLevel  int    `json:"write_level"` // 写入百分比
	ReadLevel   int    `json:"read_level"`  // 读取百分比
}

// PoolStatus represents pool health status
type PoolStatus struct {
	State       string    `json:"state"` // healthy/degraded/faulted
	TotalSize   int64     `json:"total_size"`
	UsedSize    int64     `json:"used_size"`
	Available   int64     `json:"available"`
	IOPSRead    int64     `json:"iops_read"`  // 读IOPS
	IOPSWrite   int64     `json:"iops_write"` // 写IOPS
	Throughput  int64     `json:"throughput"` // MB/s
	LastUpdated time.Time `json:"last_updated"`
}

// Manager manages NVMe pools
type Manager struct {
	pools map[string]*NVMePool
}

// NewManager creates a new NVMe pool manager
func NewManager() *Manager {
	return &Manager{
		pools: make(map[string]*NVMePool),
	}
}

// CreatePool creates a new NVMe pool
func (m *Manager) CreatePool(ctx context.Context, config *PoolConfig) (*NVMePool, error) {
	if config.Name == "" {
		return nil, fmt.Errorf("pool name required")
	}
	if len(config.NVMeDevices) == 0 {
		return nil, fmt.Errorf("at least one NVMe device required")
	}

	pool := &NVMePool{
		config:    config,
		createdAt: time.Now(),
		status: PoolStatus{
			State:       "healthy",
			LastUpdated: time.Now(),
		},
	}

	m.pools[config.Name] = pool
	return pool, nil
}

// GetPool retrieves a pool by name
func (m *Manager) GetPool(name string) (*NVMePool, error) {
	pool, exists := m.pools[name]
	if !exists {
		return nil, fmt.Errorf("pool not found: %s", name)
	}
	return pool, nil
}

// GetStatus returns pool status
func (p *NVMePool) GetStatus() PoolStatus {
	return p.status
}

// OptimizeForWorkload adjusts pool settings for workload type
func (p *NVMePool) OptimizeForWorkload(workload string) error {
	switch workload {
	case "database":
		// 高IOPS优化
		p.config.BlockSize = 8192
		p.config.CacheRatio = 0.9
	case "media":
		// 高吞吐优化
		p.config.BlockSize = 1048576
		p.config.CacheRatio = 0.5
	case "archive":
		// 高容量优化
		p.config.BlockSize = 65536
		p.config.Compression = true
	default:
		return fmt.Errorf("unknown workload type: %s", workload)
	}
	return nil
}
