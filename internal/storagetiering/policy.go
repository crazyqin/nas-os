package storagetiering

import (
	"path/filepath"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// Policy 分层策略引擎
// 管理热/温/冷数据自动迁移策略
type Policy struct {
	mu     sync.RWMutex
	config PolicyConfig
	logger *zap.Logger

	// 各层容量使用情况
	tierUsage map[Tier]int64 // tier -> used bytes
	tierTotal map[Tier]int64 // tier -> total bytes
}

// NewPolicy 创建分层策略
func NewPolicy(config PolicyConfig, tierCapacities []TierCapacity, logger *zap.Logger) *Policy {
	if logger == nil {
		logger = zap.NewNop()
	}
	tierTotal := make(map[Tier]int64)
	for _, tc := range tierCapacities {
		tierTotal[tc.Tier] = tc.TotalBytes
	}
	return &Policy{
		config:    config,
		logger:    logger,
		tierUsage: make(map[Tier]int64),
		tierTotal: tierTotal,
	}
}

// UpdateTierUsage 更新层级使用量
func (p *Policy) UpdateTierUsage(tier Tier, usedBytes int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tierUsage[tier] = usedBytes
}

// GetTierUsage 获取层级使用量
func (p *Policy) GetTierUsage(tier Tier) (used int64, total int64) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tierUsage[tier], p.tierTotal[tier]
}

// TierUsageRatio 返回层级使用率
func (p *Policy) TierUsageRatio(tier Tier) float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	total := p.tierTotal[tier]
	if total == 0 {
		return 0
	}
	return float64(p.tierUsage[tier]) / float64(total)
}

// NeedsEviction 判断某层是否需要驱逐
func (p *Policy) NeedsEviction(tier Tier) bool {
	return p.TierUsageRatio(tier) >= p.config.CapacityHighPct
}

// EvictionTargetReached 判断驱逐是否已达到目标
func (p *Policy) EvictionTargetReached(tier Tier) bool {
	return p.TierUsageRatio(tier) <= p.config.CapacityLowPct
}

// RecommendTier 根据热度评分推荐目标层级
func (p *Policy) RecommendTier(heatScore float64, filePath string, fileSize int64) Tier {
	adjustedScore := p.adjustScore(heatScore, filePath, fileSize)

	if adjustedScore >= p.config.Thresholds.HotMinScore {
		return TierSSD
	}
	if adjustedScore >= p.config.Thresholds.WarmMinScore {
		return TierHDD
	}
	return TierCold
}

// adjustScore 调整热度评分（文件类型加成 + 大文件惩罚）
func (p *Policy) adjustScore(score float64, filePath string, fileSize int64) float64 {
	adjusted := score

	// 文件类型加成
	ext := strings.ToLower(filepath.Ext(filePath))
	if boost, ok := p.config.FileTypeBoosts[ext]; ok {
		adjusted += boost
	}

	// 大文件惩罚 (>1GB)
	if fileSize > 1024*1024*1024 {
		adjusted *= (1.0 - p.config.LargeFilePenalty)
	}

	if adjusted > 100 {
		adjusted = 100
	}
	if adjusted < 0 {
		adjusted = 0
	}
	return adjusted
}

// ShouldMigrate 判断是否应该迁移
func (p *Policy) ShouldMigrate(currentTier Tier, heatScore float64, filePath string, fileSize int64) (bool, Tier) {
	recommended := p.RecommendTier(heatScore, filePath, fileSize)
	if currentTier == recommended {
		return false, currentTier
	}

	// 热数据升级：如果目标层已满，检查是否可以驱逐
	if recommended < currentTier { // 数值越小层级越高
		if p.NeedsEviction(recommended) {
			p.logger.Info("target tier full, will try eviction first",
				zap.String("tier", recommended.String()),
				zap.Float64("usage", p.TierUsageRatio(recommended)))
			// 仍然返回迁移建议，由迁移器处理驱逐
		}
	}

	return true, recommended
}

// GetEvictionCandidates 获取需要从某层驱逐的文件（按热度从低到高排序）
func (p *Policy) GetEvictionCandidates(tier Tier, files map[string]*FileEntry) []*FileEntry {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var candidates []*FileEntry
	for _, entry := range files {
		if entry.CurrentTier == tier {
			candidates = append(candidates, entry)
		}
	}

	// 按热度从低到高排序（冒泡）
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if candidates[j].HeatScore < candidates[i].HeatScore {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}

	return candidates
}

// CanFit 检查目标层是否能容纳指定大小
func (p *Policy) CanFit(tier Tier, size int64) bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	total := p.tierTotal[tier]
	used := p.tierUsage[tier]
	return (used + size) <= total
}

// FreeSpace 返回某层可用空间
func (p *Policy) FreeSpace(tier Tier) int64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	total := p.tierTotal[tier]
	used := p.tierUsage[tier]
	free := total - used
	if free < 0 {
		return 0
	}
	return free
}

// GetConfig 返回策略配置
func (p *Policy) GetConfig() PolicyConfig {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.config
}

// UpdateConfig 更新策略配置
func (p *Policy) UpdateConfig(config PolicyConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = config
	p.logger.Info("policy config updated",
		zap.Float64("hot_min", config.Thresholds.HotMinScore),
		zap.Float64("warm_min", config.Thresholds.WarmMinScore),
		zap.Float64("capacity_high", config.CapacityHighPct))
}
