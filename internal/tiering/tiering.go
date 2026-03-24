// Package tiering implements intelligent data tiering for NAS-OS
// Inspired by Synology Tiering - automatically migrates data between storage tiers
// based on access frequency and patterns
package tiering

import (
	"context"
	"sync"
	"time"
)

// Tier represents a storage tier with different performance characteristics
type Tier int

const (
	// TierHot is high-performance storage (SSD/NVMe)
	TierHot Tier = iota
	// TierWarm is standard storage (HDD)
	TierWarm
	// TierCold is archive storage (tape/cloud)
	TierCold
)

// TierConfig defines configuration for a storage tier
type TierConfig struct {
	Name        string
	Type        Tier
	Path        string
	MaxCapacity int64
	MinFree     int64 // Minimum free space to maintain
	Priority    int   // Higher priority = preferred for hot data
}

// FileAccessRecord tracks access patterns for a file
type FileAccessRecord struct {
	Path         string
	LastAccess   time.Time
	AccessCount  int64
	Size         int64
	CurrentTier  Tier
	PromotionScore float64 // Higher = should move to faster tier
	DemotionScore  float64 // Higher = should move to slower tier
}

// Policy defines tiering behavior
type Policy struct {
	// Hot tier thresholds
	HotAccessThreshold  int64         // Access count to promote to hot
	HotAgeThreshold     time.Duration // Max age for hot data
	
	// Cold tier thresholds
	ColdAccessThreshold int64         // Access count below which to demote
	ColdAgeThreshold    time.Duration // Age after which to consider cold
	
	// Scan interval
	ScanInterval time.Duration
	
	// Enable automatic tiering
	AutoTiering bool
}

// DefaultPolicy returns recommended tiering policy
func DefaultPolicy() Policy {
	return Policy{
		HotAccessThreshold:  10,
		HotAgeThreshold:     7 * 24 * time.Hour, // 7 days
		ColdAccessThreshold: 2,
		ColdAgeThreshold:    30 * 24 * time.Hour, // 30 days
		ScanInterval:        1 * time.Hour,
		AutoTiering:         true,
	}
}

// Manager handles intelligent data tiering
type Manager struct {
	mu       sync.RWMutex
	tiers    map[Tier]*TierConfig
	policy   Policy
	records  map[string]*FileAccessRecord
	stopCh   chan struct{}
	running  bool
}

// NewManager creates a new tiering manager
func NewManager(policy Policy) *Manager {
	return &Manager{
		tiers:   make(map[Tier]*TierConfig),
		policy:  policy,
		records: make(map[string]*FileAccessRecord),
		stopCh:  make(chan struct{}),
	}
}

// AddTier registers a storage tier
func (m *Manager) AddTier(config TierConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.tiers[config.Type] = &config
	return nil
}

// RecordAccess records a file access for tiering decisions
func (m *Manager) RecordAccess(path string, size int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	record, exists := m.records[path]
	if !exists {
		record = &FileAccessRecord{
			Path:        path,
			Size:        size,
			CurrentTier: TierWarm, // Default to warm tier
		}
		m.records[path] = record
	}
	
	record.LastAccess = time.Now()
	record.AccessCount++
	
	// Update promotion/demotion scores
	m.updateScores(record)
}

// updateScores calculates tiering scores based on access patterns
func (m *Manager) updateScores(record *FileAccessRecord) {
	now := time.Now()
	age := now.Sub(record.LastAccess)
	
	// Promotion score: frequent recent access
	record.PromotionScore = float64(record.AccessCount) / (1 + age.Hours())
	
	// Demotion score: infrequent access
	record.DemotionScore = float64(age.Hours()) / (1 + float64(record.AccessCount))
}

// GetTieringRecommendations returns files that should be moved
func (m *Manager) GetTieringRecommendations(ctx context.Context) (promote, demote []string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	for path, record := range m.records {
		// Promote to hot tier
		if record.PromotionScore > float64(m.policy.HotAccessThreshold) {
			if record.CurrentTier != TierHot {
				promote = append(promote, path)
			}
		}
		
		// Demote to cold tier
		if record.DemotionScore > float64(m.policy.ColdAgeThreshold.Hours()) {
			if record.CurrentTier != TierCold {
				demote = append(demote, path)
			}
		}
	}
	
	return promote, demote
}

// Start begins the tiering background process
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil
	}
	m.running = true
	m.mu.Unlock()
	
	go m.run(ctx)
	return nil
}

// Stop halts the tiering process
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.running {
		close(m.stopCh)
		m.running = false
	}
}

func (m *Manager) run(ctx context.Context) {
	ticker := time.NewTicker(m.policy.ScanInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			if m.policy.AutoTiering {
				m.scanAndTier(ctx)
			}
		}
	}
}

func (m *Manager) scanAndTier(ctx context.Context) {
	promote, demote := m.GetTieringRecommendations(ctx)
	
	// Log tiering actions (actual migration would be implemented here)
	_ = promote
	_ = demote
}

// GetStats returns tiering statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	stats := map[string]interface{}{
		"total_files":  len(m.records),
		"auto_tiering": m.policy.AutoTiering,
		"tiers":        len(m.tiers),
	}
	
	// Count files per tier
	tierCounts := make(map[Tier]int64)
	for _, record := range m.records {
		tierCounts[record.CurrentTier]++
	}
	stats["files_per_tier"] = tierCounts
	
	return stats
}