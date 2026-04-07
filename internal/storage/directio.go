// Package storage provides Direct I/O optimization for ZFS.
package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// DirectIOConfig represents Direct I/O configuration.
type DirectIOConfig struct {
	Enabled        bool     `json:"enabled"`           // Global enable/disable
	PoolWhiteList  []string `json:"pool_whitelist"`    // Pools using Direct I/O
	PoolBlackList  []string `json:"pool_blacklist"`    // Pools bypassing Direct I/O
	MinFileSizeMB  int      `json:"min_file_size_mb"`  // Minimum file size for Direct I/O
	MaxFileSizeMB  int      `json:"max_file_size_mb"`  // Maximum file size (fallback to cached)
	BypassRead     bool     `json:"bypass_read"`       // Bypass ARC for reads
	BypassWrite    bool     `json:"bypass_write"`      // Bypass ZIL for writes
	SyncWrite      bool     `json:"sync_write"`        // Force synchronous writes
}

// DirectIOMetrics represents Direct I/O performance metrics.
type DirectIOMetrics struct {
	PoolName          string    `json:"pool_name"`
	ReadOpsDirect     int64     `json:"read_ops_direct"`    // Direct read operations
	ReadOpsCached     int64     `json:"read_ops_cached"`    // Cached read operations
	WriteOpsDirect    int64     `json:"write_ops_direct"`   // Direct write operations
	WriteOpsCached    int64     `json:"write_ops_cached"`   // Cached write operations
	AvgReadLatencyMs  float64   `json:"avg_read_latency_ms"` // Average read latency
	AvgWriteLatencyMs float64   `json:"avg_write_latency_ms"` // Average write latency
	BytesRead         int64     `json:"bytes_read"`
	BytesWritten      int64     `json:"bytes_written"`
	CacheHitRate      float64   `json:"cache_hit_rate"`    // Cache hit rate (for cached ops)
	LastUpdated       time.Time `json:"last_updated"`
}

// DirectIOManager manages Direct I/O configuration and metrics.
type DirectIOManager struct {
	mu      sync.RWMutex
	config  *DirectIOConfig
	metrics map[string]*DirectIOMetrics // Per-pool metrics
	logger  *zap.Logger
	configPath string
}

// NewDirectIOManager creates a new Direct I/O manager.
func NewDirectIOManager(configPath string, logger *zap.Logger) (*DirectIOManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	config := &DirectIOConfig{
		Enabled:        false,    // Disabled by default
		PoolWhiteList:  []string{},
		PoolBlackList:  []string{},
		MinFileSizeMB:  1,        // Minimum 1MB for Direct I/O
		MaxFileSizeMB:  0,        // No upper limit
		BypassRead:     true,
		BypassWrite:    true,
		SyncWrite:      false,
	}

	m := &DirectIOManager{
		config:     config,
		metrics:    make(map[string]*DirectIOMetrics),
		logger:     logger,
		configPath: configPath,
	}

	if err := m.loadConfig(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return m, nil
}

// EnableDirectIO enables Direct I/O globally or for specific pools.
func (m *DirectIOManager) EnableDirectIO(ctx context.Context, pools []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Enabled = true
	if len(pools) > 0 {
		m.config.PoolWhiteList = pools
	}

	m.logger.Info("Enabled Direct I/O",
		zap.Bool("global", len(pools) == 0),
		zap.Strings("pools", pools))

	return m.saveConfig()
}

// DisableDirectIO disables Direct I/O.
func (m *DirectIOManager) DisableDirectIO(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.Enabled = false
	m.logger.Info("Disabled Direct I/O")
	return m.saveConfig()
}

// AddPoolToWhitelist adds a pool to Direct I/O whitelist.
func (m *DirectIOManager) AddPoolToWhitelist(ctx context.Context, poolName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already in whitelist
	for _, p := range m.config.PoolWhiteList {
		if p == poolName {
			return nil
		}
	}

	m.config.PoolWhiteList = append(m.config.PoolWhiteList, poolName)
	m.logger.Info("Added pool to Direct I/O whitelist", zap.String("pool", poolName))

	return m.saveConfig()
}

// RemovePoolFromWhitelist removes a pool from whitelist.
func (m *DirectIOManager) RemovePoolFromWhitelist(ctx context.Context, poolName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	newList := []string{}
	for _, p := range m.config.PoolWhiteList {
		if p != poolName {
			newList = append(newList, p)
		}
	}
	m.config.PoolWhiteList = newList

	m.logger.Info("Removed pool from Direct I/O whitelist", zap.String("pool", poolName))
	return m.saveConfig()
}

// AddPoolToBlacklist adds a pool to blacklist (force cached).
func (m *DirectIOManager) AddPoolToBlacklist(ctx context.Context, poolName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.config.PoolBlackList {
		if p == poolName {
			return nil
		}
	}

	m.config.PoolBlackList = append(m.config.PoolBlackList, poolName)
	m.logger.Info("Added pool to Direct I/O blacklist", zap.String("pool", poolName))

	return m.saveConfig()
}

// SetFileSizeThresholds sets file size thresholds for Direct I/O.
func (m *DirectIOManager) SetFileSizeThresholds(ctx context.Context, minMB, maxMB int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.MinFileSizeMB = minMB
	m.config.MaxFileSizeMB = maxMB

	m.logger.Info("Set Direct I/O file size thresholds",
		zap.Int("min_mb", minMB),
		zap.Int("max_mb", maxMB))

	return m.saveConfig()
}

// SetBypassOptions sets bypass options for reads and writes.
func (m *DirectIOManager) SetBypassOptions(ctx context.Context, bypassRead, bypassWrite, syncWrite bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.BypassRead = bypassRead
	m.config.BypassWrite = bypassWrite
	m.config.SyncWrite = syncWrite

	m.logger.Info("Set Direct I/O bypass options",
		zap.Bool("bypass_read", bypassRead),
		zap.Bool("bypass_write", bypassWrite),
		zap.Bool("sync_write", syncWrite))

	return m.saveConfig()
}

// ShouldUseDirectIO determines if Direct I/O should be used for an operation.
func (m *DirectIOManager) ShouldUseDirectIO(poolName string, fileSizeMB int) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.config.Enabled {
		return false
	}

	// Check blacklist first
	for _, p := range m.config.PoolBlackList {
		if p == poolName {
			return false
		}
	}

	// If whitelist is empty, apply to all pools (except blacklisted)
	if len(m.config.PoolWhiteList) == 0 {
		return m.checkFileSize(fileSizeMB)
	}

	// Check whitelist
	for _, p := range m.config.PoolWhiteList {
		if p == poolName {
			return m.checkFileSize(fileSizeMB)
		}
	}

	return false
}

// checkFileSize checks if file size falls within Direct I/O thresholds.
func (m *DirectIOManager) checkFileSize(fileSizeMB int) bool {
	if fileSizeMB < m.config.MinFileSizeMB {
		return false
	}
	if m.config.MaxFileSizeMB > 0 && fileSizeMB > m.config.MaxFileSizeMB {
		return false
	}
	return true
}

// RecordMetrics records Direct I/O metrics for a pool.
func (m *DirectIOManager) RecordMetrics(ctx context.Context, poolName string, metrics *DirectIOMetrics) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics.LastUpdated = time.Now()
	m.metrics[poolName] = metrics

	m.logger.Debug("Recorded Direct I/O metrics",
		zap.String("pool", poolName),
		zap.Float64("read_latency", metrics.AvgReadLatencyMs))

	return nil
}

// GetMetrics returns Direct I/O metrics for a pool.
func (m *DirectIOManager) GetMetrics(ctx context.Context, poolName string) (*DirectIOMetrics, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	metrics, exists := m.metrics[poolName]
	if !exists {
		return nil, fmt.Errorf("no metrics for pool %s", poolName)
	}

	return metrics, nil
}

// GetAllMetrics returns all pool metrics.
func (m *DirectIOManager) GetAllMetrics(ctx context.Context) map[string]*DirectIOMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*DirectIOMetrics)
	for k, v := range m.metrics {
		result[k] = v
	}
	return result
}

// GetConfig returns current Direct I/O configuration.
func (m *DirectIOManager) GetConfig(ctx context.Context) *DirectIOConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// GetPerformanceReport generates a performance comparison report.
func (m *DirectIOManager) GetPerformanceReport(ctx context.Context) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := map[string]interface{}{
		"enabled":          m.config.Enabled,
		"pools_with_direct_io": m.config.PoolWhiteList,
		"pools_excluded":   m.config.PoolBlackList,
		"pool_metrics":     []map[string]interface{}{},
	}

	// Calculate overall performance impact
	totalDirectReads := int64(0)
	totalCachedReads := int64(0)
	totalDirectWrites := int64(0)
	totalCachedWrites := int64(0)
	avgDirectReadLatency := 0.0
	avgCachedReadLatency := 0.0

	directCount := 0
	cachedCount := 0

	for pool, metrics := range m.metrics {
		poolData := map[string]interface{}{
			"pool":             pool,
			"read_ops_direct":  metrics.ReadOpsDirect,
			"read_ops_cached":  metrics.ReadOpsCached,
			"write_ops_direct": metrics.WriteOpsDirect,
			"write_ops_cached": metrics.WriteOpsCached,
			"avg_read_latency": metrics.AvgReadLatencyMs,
			"avg_write_latency": metrics.AvgWriteLatencyMs,
			"cache_hit_rate":   metrics.CacheHitRate,
		}
		report["pool_metrics"] = append(report["pool_metrics"].([]map[string]interface{}), poolData)

		totalDirectReads += metrics.ReadOpsDirect
		totalCachedReads += metrics.ReadOpsCached
		totalDirectWrites += metrics.WriteOpsDirect
		totalCachedWrites += metrics.WriteOpsCached

		if metrics.ReadOpsDirect > 0 {
			avgDirectReadLatency += metrics.AvgReadLatencyMs
			directCount++
		}
		if metrics.ReadOpsCached > 0 {
			avgCachedReadLatency += metrics.AvgReadLatencyMs
			cachedCount++
		}
	}

	// Calculate averages
	if directCount > 0 {
		avgDirectReadLatency /= float64(directCount)
	}
	if cachedCount > 0 {
		avgCachedReadLatency /= float64(cachedCount)
	}

	report["summary"] = map[string]interface{}{
		"total_direct_ops":  totalDirectReads + totalDirectWrites,
		"total_cached_ops":  totalCachedReads + totalCachedWrites,
		"avg_direct_latency": avgDirectReadLatency,
		"avg_cached_latency": avgCachedReadLatency,
		"latency_improvement": avgCachedReadLatency - avgDirectReadLatency, // Positive = Direct I/O faster
	}

	return report
}

// loadConfig loads Direct I/O configuration.
func (m *DirectIOManager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var cfg struct {
		Config  *DirectIOConfig           `json:"config"`
		Metrics map[string]*DirectIOMetrics `json:"metrics"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Config != nil {
		m.config = cfg.Config
	}
	m.metrics = cfg.Metrics

	return nil
}

// saveConfig saves Direct I/O configuration.
func (m *DirectIOManager) saveConfig() error {
	cfg := struct {
		Config  *DirectIOConfig           `json:"config"`
		Metrics map[string]*DirectIOMetrics `json:"metrics"`
	}{
		Config:  m.config,
		Metrics: m.metrics,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0644)
}