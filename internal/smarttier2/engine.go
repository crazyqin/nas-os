package smarttier2

import (
	"sort"
	"sync"
	"time"
)

// Tier 存储层。
type Tier string

const (
	TierHot    Tier = "hot"    // NVMe/SSD 高速层
	TierWarm   Tier = "warm"   // 混合层
	TierCold   Tier = "cold"   // HDD 归档层
	TierFrozen Tier = "frozen" // 离线归档
)

// DataClass 数据分类。
type DataClass struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Tier        Tier    `json:"tier"`
	AccessFreq  float64 `json:"access_freq"`  // 访问频率 (次/天)
	SizeBytes   int64   `json:"size_bytes"`
	LastAccess  time.Time `json:"last_access"`
	FilePath    string  `json:"file_path"`
}

// TierConfig 层配置。
type TierConfig struct {
	Tier          Tier    `json:"tier"`
	MaxSizeGB     int64   `json:"max_size_gb"`
	ThresholdDays int     `json:"threshold_days"` // 停留天数阈值
	Priority      int     `json:"priority"`
}

// Policy 分层策略。
type Policy struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Rules     []TierRule   `json:"rules"`
	Enabled   bool         `json:"enabled"`
	CreatedAt time.Time    `json:"created_at"`
}

// TierRule 分层规则。
type TierRule struct {
	SourceTier Tier    `json:"source_tier"`
	TargetTier Tier    `json:"target_tier"`
	Condition  string  `json:"condition"` // "access_freq < 0.1", "days_idle > 30"
}

// Stats 分层统计。
type Stats struct {
	TierSizes   map[Tier]int64 `json:"tier_sizes"`
	TierCounts  map[Tier]int64 `json:"tier_counts"`
	TotalSize   int64          `json:"total_size"`
	MigratedGB  float64        `json:"migrated_gb_24h"`
}

// Engine 智能分层引擎。
type Engine struct {
	mu       sync.RWMutex
	data     map[string]*DataClass
	configs  map[Tier]*TierConfig
	policies map[string]*Policy
	stats    Stats
}

// NewEngine 创建新的分层引擎。
func NewEngine() *Engine {
	e := &Engine{
		data:     make(map[string]*DataClass),
		configs:  make(map[Tier]*TierConfig),
		policies: make(map[string]*Policy),
	}
	e.initDefaultConfigs()
	return e
}

func (e *Engine) initDefaultConfigs() {
	e.configs[TierHot] = &TierConfig{
		Tier:          TierHot,
		MaxSizeGB:     500,
		ThresholdDays: 7,
		Priority:      1,
	}
	e.configs[TierWarm] = &TierConfig{
		Tier:          TierWarm,
		MaxSizeGB:     2000,
		ThresholdDays: 30,
		Priority:      2,
	}
	e.configs[TierCold] = &TierConfig{
		Tier:          TierCold,
		MaxSizeGB:     10000,
		ThresholdDays: 90,
		Priority:      3,
	}
	e.configs[TierFrozen] = &TierConfig{
		Tier:          TierFrozen,
		MaxSizeGB:     0, // 无限制
		ThresholdDays: 0,
		Priority:      4,
	}
}

// AddData 添加数据项。
func (e *Engine) AddData(dc *DataClass) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.data[dc.ID] = dc
	e.updateStats()
}

// GetData 获取数据项。
func (e *Engine) GetData(id string) (*DataClass, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	dc, exists := e.data[id]
	return dc, exists
}

// Migrate 执行数据迁移。
func (e *Engine) Migrate(id string, targetTier Tier) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	dc, exists := e.data[id]
	if !exists {
		return ErrDataNotFound
	}
	dc.Tier = targetTier
	e.updateStats()
	return nil
}

// Analyze 分析并推荐迁移。
func (e *Engine) Analyze() []MigrationRecommendation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var recs []MigrationRecommendation
	now := time.Now()

	for _, dc := range e.data {
		daysSinceAccess := now.Sub(dc.LastAccess).Hours() / 24
		targetTier := e.recommendTier(dc.AccessFreq, daysSinceAccess)
		if targetTier != dc.Tier {
			recs = append(recs, MigrationRecommendation{
				DataID:     dc.ID,
				FilePath:   dc.FilePath,
				SourceTier: dc.Tier,
				TargetTier: targetTier,
				Reason:     e.getReason(dc.AccessFreq, daysSinceAccess),
			})
		}
	}

	sort.Slice(recs, func(i, j int) bool {
		return recs[i].TargetTier < recs[j].TargetTier
	})
	return recs
}

func (e *Engine) recommendTier(freq, daysIdle float64) Tier {
	if freq > 1.0 || daysIdle < 1 {
		return TierHot
	}
	if freq > 0.1 || daysIdle < 30 {
		return TierWarm
	}
	if freq > 0.01 || daysIdle < 90 {
		return TierCold
	}
	return TierFrozen
}

func (e *Engine) getReason(freq, daysIdle float64) string {
	if freq > 1.0 {
		return "高频访问，建议提升至热层"
	}
	if daysIdle > 90 {
		return "长期未访问，建议归档"
	}
	return "访问频率降低，建议迁移"
}

func (e *Engine) updateStats() {
	e.stats.TierSizes = make(map[Tier]int64)
	e.stats.TierCounts = make(map[Tier]int64)
	for _, dc := range e.data {
		e.stats.TierSizes[dc.Tier] += dc.SizeBytes
		e.stats.TierCounts[dc.Tier]++
		e.stats.TotalSize += dc.SizeBytes
	}
}

// GetStats 获取统计信息。
func (e *Engine) GetStats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// MigrationRecommendation 迁移建议。
type MigrationRecommendation struct {
	DataID     string `json:"data_id"`
	FilePath   string `json:"file_path"`
	SourceTier Tier   `json:"source_tier"`
	TargetTier Tier   `json:"target_tier"`
	Reason     string `json:"reason"`
}

// 错误定义。
var (
	ErrDataNotFound = &TierError{"data not found"}
)

type TierError struct {
	msg string
}

func (e *TierError) Error() string {
	return e.msg
}
