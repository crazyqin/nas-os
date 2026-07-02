package hybridflash

import (
	"fmt"
	"sync"
	"time"
)

// TierType represents storage tier type.
type TierType string

const (
	TierFlash TierType = "flash"
	TierHDD   TierType = "hdd"
	TierNVMe  TierType = "nvme"
	TierSSD   TierType = "ssd"
)

// DatasetPriority represents dataset access priority.
type DatasetPriority string

const (
	PriorityHot  DatasetPriority = "hot"
	PriorityWarm DatasetPriority = "warm"
	PriorityCold DatasetPriority = "cold"
)

// FlashPool represents a hybrid flash pool configuration.
type FlashPool struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	FlashVdevs    []VDev        `json:"flash_vdevs"`
	HDDVdevs      []VDev        `json:"hdd_vdevs"`
	TieringPolicy TieringPolicy `json:"tiering_policy"`
	CreatedAt     time.Time     `json:"created_at"`
	TotalFlashGB  int64         `json:"total_flash_gb"`
	TotalHDDGB    int64         `json:"total_hdd_gb"`
	Status        string        `json:"status"`
}

// VDev represents a virtual device in a pool.
type VDev struct {
	ID       string   `json:"id"`
	Type     TierType `json:"type"`
	Devices  []string `json:"devices"`
	SizeGB   int64    `json:"size_gb"`
	UsedGB   int64    `json:"used_gb"`
	RAIDType string   `json:"raid_type"` // mirror, raidz1, raidz2, raidz3
}

// TieringPolicy defines how data moves between tiers.
type TieringPolicy struct {
	Enabled          bool    `json:"enabled"`
	HotThreshold     float64 `json:"hot_threshold"`    // access frequency > N/day = hot
	ColdThreshold    float64 `json:"cold_threshold"`   // access frequency < N/day = cold
	MigrationWindow  string  `json:"migration_window"` // e.g., "02:00-06:00"
	MinFileAgeHours  int     `json:"min_file_age_hours"`
	MetadataOnFlash  bool    `json:"metadata_on_flash"` // always keep metadata on flash
	SmallFileOnFlash bool    `json:"small_file_on_flash"`
	SmallFileMaxKB   int     `json:"small_file_max_kb"`
}

// DatasetTierBinding binds a dataset to a specific tier.
type DatasetTierBinding struct {
	DatasetPath string          `json:"dataset_path"`
	Tier        TierType        `json:"tier"`
	Priority    DatasetPriority `json:"priority"`
	MaxSizeGB   int64           `json:"max_size_gb"`
}

// PoolStats contains pool performance statistics.
type PoolStats struct {
	PoolID         string  `json:"pool_id"`
	FlashReadMBps  float64 `json:"flash_read_mbps"`
	FlashWriteMBps float64 `json:"flash_write_mbps"`
	HDDReadMBps    float64 `json:"hdd_read_mbps"`
	HDDWriteMBps   float64 `json:"hdd_write_mbps"`
	FlashUsedPct   float64 `json:"flash_used_pct"`
	HDDUsedPct     float64 `json:"hdd_used_pct"`
	CacheHitRate   float64 `json:"cache_hit_rate"`
	TierMigrations int64   `json:"tier_migrations_24h"`
}

// HybridFlashPoolManager manages hybrid flash/HDD pools.
type HybridFlashPoolManager struct {
	mu       sync.RWMutex
	pools    map[string]*FlashPool
	bindings map[string]*DatasetTierBinding // dataset path -> binding
	stats    map[string]*PoolStats
}

// NewHybridFlashPoolManager creates a new pool manager.
func NewHybridFlashPoolManager() *HybridFlashPoolManager {
	return &HybridFlashPoolManager{
		pools:    make(map[string]*FlashPool),
		bindings: make(map[string]*DatasetTierBinding),
		stats:    make(map[string]*PoolStats),
	}
}

// CreatePool creates a new hybrid flash pool.
func (m *HybridFlashPoolManager) CreatePool(pool FlashPool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pool.ID == "" {
		return fmt.Errorf("pool ID cannot be empty")
	}
	if _, exists := m.pools[pool.ID]; exists {
		return fmt.Errorf("pool already exists: %s", pool.ID)
	}

	// Calculate totals
	for _, vdev := range pool.FlashVdevs {
		pool.TotalFlashGB += vdev.SizeGB
	}
	for _, vdev := range pool.HDDVdevs {
		pool.TotalHDDGB += vdev.SizeGB
	}

	pool.CreatedAt = time.Now()
	pool.Status = "online"
	m.pools[pool.ID] = &pool

	// Initialize stats
	m.stats[pool.ID] = &PoolStats{
		PoolID: pool.ID,
	}

	return nil
}

// DeletePool deletes a pool.
func (m *HybridFlashPoolManager) DeletePool(poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pools[poolID]; !exists {
		return fmt.Errorf("pool not found: %s", poolID)
	}
	delete(m.pools, poolID)
	delete(m.stats, poolID)
	return nil
}

// GetPool returns a pool by ID.
func (m *HybridFlashPoolManager) GetPool(poolID string) (*FlashPool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return nil, fmt.Errorf("pool not found: %s", poolID)
	}
	return pool, nil
}

// ListPools returns all pools.
func (m *HybridFlashPoolManager) ListPools() []FlashPool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]FlashPool, 0, len(m.pools))
	for _, p := range m.pools {
		result = append(result, *p)
	}
	return result
}

// BindDataset binds a dataset to a specific tier.
func (m *HybridFlashPoolManager) BindDataset(binding DatasetTierBinding) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.bindings[binding.DatasetPath] = &binding
	return nil
}

// GetDatasetTier returns the tier for a dataset.
func (m *HybridFlashPoolManager) GetDatasetTier(datasetPath string) (TierType, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	binding, exists := m.bindings[datasetPath]
	if !exists {
		return TierHDD, nil // default to HDD for unknown datasets
	}
	return binding.Tier, nil
}

// UpdateTieringPolicy updates the tiering policy for a pool.
func (m *HybridFlashPoolManager) UpdateTieringPolicy(poolID string, policy TieringPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	pool, exists := m.pools[poolID]
	if !exists {
		return fmt.Errorf("pool not found: %s", poolID)
	}
	pool.TieringPolicy = policy
	return nil
}

// GetPoolStats returns performance statistics for a pool.
func (m *HybridFlashPoolManager) GetPoolStats(poolID string) (*PoolStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats, exists := m.stats[poolID]
	if !exists {
		return nil, fmt.Errorf("pool not found: %s", poolID)
	}
	return stats, nil
}

// RecommendTier recommends a tier based on access patterns.
func (m *HybridFlashPoolManager) RecommendTier(fileSizeKB int64, accessFreq float64, isMetadata bool) TierType {
	// Metadata always goes to flash
	if isMetadata {
		return TierNVMe
	}
	// Small files on flash
	if fileSizeKB <= 256 {
		return TierSSD
	}
	// High frequency access on flash
	if accessFreq > 10.0 {
		return TierNVMe
	}
	// Medium frequency on SSD
	if accessFreq > 1.0 {
		return TierSSD
	}
	// Cold data on HDD
	return TierHDD
}
