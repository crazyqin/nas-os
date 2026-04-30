// Package storage provides storage efficiency statistics and analysis.
package storage

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// EfficiencyStats holds storage efficiency statistics.
type EfficiencyStats struct {
	TotalSpace       int64   `json:"total_space"`
	UsedSpace        int64   `json:"used_space"`
	FreeSpace        int64   `json:"free_space"`
	CompressionRatio float64 `json:"compression_ratio"`
	DedupRatio       float64 `json:"dedup_ratio"`
	ThinProvisioned  int64   `json:"thin_provisioned"`
	SnapshotSpace    int64   `json:"snapshot_space"`
	DataWritten      int64   `json:"data_written"`
	DataRead         int64   `json:"data_read"`
	OverallSaving    float64 `json:"overall_saving_percent"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// PoolEfficiency holds per-pool efficiency data.
type PoolEfficiency struct {
	PoolName         string  `json:"pool_name"`
	TotalSpace       int64   `json:"total_space"`
	UsedSpace        int64   `json:"used_space"`
	CompressionRatio float64 `json:"compression_ratio"`
	DedupRatio       float64 `json:"dedup_ratio"`
	OverallSaving    float64 `json:"overall_saving_percent"`
}

// EfficiencyCollector collects storage efficiency statistics.
type EfficiencyCollector struct {
	mu       sync.RWMutex
	stats    *EfficiencyStats
	pools    map[string]*PoolEfficiency
	dataDir  string
	interval time.Duration
}

// NewEfficiencyCollector creates a new efficiency collector.
func NewEfficiencyCollector(dataDir string) *EfficiencyCollector {
	return &EfficiencyCollector{
		stats:    &EfficiencyStats{},
		pools:    make(map[string]*PoolEfficiency),
		dataDir:  dataDir,
		interval: 5 * time.Minute,
	}
}

// GetEfficiency returns current storage efficiency statistics.
func (ec *EfficiencyCollector) GetEfficiency() *EfficiencyStats {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	stats := *ec.stats
	stats.UpdatedAt = time.Now()
	return &stats
}

// GetPoolEfficiency returns efficiency stats for a specific pool.
func (ec *EfficiencyCollector) GetPoolEfficiency(poolName string) (*PoolEfficiency, error) {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	pool, exists := ec.pools[poolName]
	if !exists {
		return nil, fmt.Errorf("pool %s not found", poolName)
	}
	return pool, nil
}

// ListPoolEfficiencies returns efficiency stats for all pools.
func (ec *EfficiencyCollector) ListPoolEfficiencies() []*PoolEfficiency {
	ec.mu.RLock()
	defer ec.mu.RUnlock()

	result := make([]*PoolEfficiency, 0, len(ec.pools))
	for _, pool := range ec.pools {
		result = append(result, pool)
	}
	return result
}

// Refresh forces a refresh of efficiency statistics.
func (ec *EfficiencyCollector) Refresh(ctx context.Context) error {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	// Collect disk usage
	var totalSpace, usedSpace int64
	entries, err := os.ReadDir(ec.dataDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				size := dirSize(filepath.Join(ec.dataDir, entry.Name()))
				usedSpace += size
			}
		}
	}

	// Get filesystem stats
	var stat syscall.Statfs_t
	if err := syscall.Statfs(ec.dataDir, &stat); err == nil {
		totalSpace = int64(stat.Blocks) * int64(stat.Bsize)
		freeSpace := int64(stat.Bavail) * int64(stat.Bsize)
		ec.stats.TotalSpace = totalSpace
		ec.stats.FreeSpace = freeSpace
	}
	ec.stats.UsedSpace = usedSpace
	ec.stats.UpdatedAt = time.Now()

	return nil
}

// dirSize returns the total size of a directory.
func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
