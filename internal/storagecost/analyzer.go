// Package storagecost 提供存储成本优化分析能力
// 对标群晖分层存储，支持冷热数据识别、成本优化建议
package storagecost

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Analyzer 存储成本分析器.
type Analyzer struct {
	mu      sync.RWMutex
	config  *Config
	storage StorageBackend
	logger  Logger
}

// Config 配置.
type Config struct {
	HotDataThresholdDays  int     // 热数据阈值（天）
	WarmDataThresholdDays int     // 温数据阈度（天）
	CostPerGBPerMonth     float64 // 每GB每月成本
	EnableAutoTiering     bool    // 启用自动分层
}

// StorageBackend 存储后端接口.
type StorageBackend interface {
	GetStorageStats() (*StorageStats, error)
	GetFileAccessPatterns() ([]*FileAccessPattern, error)
	MoveToTier(path string, tier StorageTier) error
}

// StorageStats 存储统计.
type StorageStats struct {
	TotalCapacity     int64
	UsedCapacity      int64
	AvailableCapacity int64
	ByTier            map[StorageTier]int64
	ByType            map[string]int64
}

// StorageTier 存储层级.
type StorageTier string

const (
	TierHot  StorageTier = "hot"  // 高性能存储
	TierWarm StorageTier = "warm" // 中等性能存储
	TierCold StorageTier = "cold" // 低成本存储
)

// FileAccessPattern 文件访问模式.
type FileAccessPattern struct {
	Path        string
	Size        int64
	LastAccess  time.Time
	AccessCount int64
	Tier        StorageTier
}

// CostReport 成本报告.
type CostReport struct {
	GeneratedAt      time.Time
	TotalCost        float64
	CostByTier       map[StorageTier]float64
	OptimizationTips []OptimizationTip
	SavingsEstimate  float64
	TierDistribution map[StorageTier]TierInfo
}

// TierInfo 层级信息.
type TierInfo struct {
	Count      int
	TotalSize  int64
	Cost       float64
	Percentage float64
}

// OptimizationTip 优化建议.
type OptimizationTip struct {
	Priority    string
	Category    string
	Description string
	Savings     float64
	Action      string
}

// Logger 日志接口.
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
}

// NewAnalyzer 创建新的存储成本分析器.
func NewAnalyzer(config *Config, storage StorageBackend, logger Logger) *Analyzer {
	return &Analyzer{
		config:  config,
		storage: storage,
		logger:  logger,
	}
}

// Analyze 分析存储成本.
func (a *Analyzer) Analyze() (*CostReport, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	// 获取存储统计
	stats, err := a.storage.GetStorageStats()
	if err != nil {
		return nil, fmt.Errorf("获取存储统计失败: %w", err)
	}

	// 获取文件访问模式
	patterns, err := a.storage.GetFileAccessPatterns()
	if err != nil {
		return nil, fmt.Errorf("获取访问模式失败: %w", err)
	}

	// 生成报告
	report := &CostReport{
		GeneratedAt:      time.Now(),
		CostByTier:       make(map[StorageTier]float64),
		TierDistribution: make(map[StorageTier]TierInfo),
	}

	// 计算各层级成本
	for tier, size := range stats.ByTier {
		cost := float64(size) / (1024 * 1024 * 1024) * a.config.CostPerGBPerMonth
		report.CostByTier[tier] = cost
		report.TotalCost += cost

		report.TierDistribution[tier] = TierInfo{
			TotalSize:  size,
			Cost:       cost,
			Percentage: float64(size) / float64(stats.UsedCapacity) * 100,
		}
	}

	// 生成优化建议
	report.OptimizationTips = a.generateOptimizationTips(patterns, stats)
	report.SavingsEstimate = a.calculateSavingsEstimate(patterns)

	return report, nil
}

// generateOptimizationTips 生成优化建议.
func (a *Analyzer) generateOptimizationTips(patterns []*FileAccessPattern, stats *StorageStats) []OptimizationTip {
	var tips []OptimizationTip

	// 识别冷数据
	var coldDataSize int64
	var coldDataCount int
	for _, p := range patterns {
		if time.Since(p.LastAccess) > time.Duration(a.config.WarmDataThresholdDays)*24*time.Hour {
			coldDataSize += p.Size
			coldDataCount++
		}
	}

	if coldDataSize > 0 {
		savings := float64(coldDataSize) / (1024 * 1024 * 1024) * a.config.CostPerGBPerMonth * 0.7 // 冷存储节省70%
		tips = append(tips, OptimizationTip{
			Priority:    "高",
			Category:    "数据分层",
			Description: fmt.Sprintf("发现 %d 个冷数据文件 (%.2f GB)，建议迁移到冷存储", coldDataCount, float64(coldDataSize)/(1024*1024*1024)),
			Savings:     savings,
			Action:      "move_to_cold",
		})
	}

	// 检查存储使用率
	usageRate := float64(stats.UsedCapacity) / float64(stats.TotalCapacity) * 100
	if usageRate > 80 {
		tips = append(tips, OptimizationTip{
			Priority:    "高",
			Category:    "容量规划",
			Description: fmt.Sprintf("存储使用率 %.1f%%，建议扩容或清理", usageRate),
			Savings:     0,
			Action:      "expand_or_cleanup",
		})
	}

	// 检查重复数据
	tips = append(tips, OptimizationTip{
		Priority:    "中",
		Category:    "数据去重",
		Description: "建议启用数据去重功能，可节省约15-30%存储空间",
		Savings:     float64(stats.UsedCapacity) / (1024 * 1024 * 1024) * a.config.CostPerGBPerMonth * 0.2,
		Action:      "enable_dedup",
	})

	return tips
}

// calculateSavingsEstimate 计算预估节省.
func (a *Analyzer) calculateSavingsEstimate(patterns []*FileAccessPattern) float64 {
	var totalSavings float64

	for _, p := range patterns {
		if time.Since(p.LastAccess) > time.Duration(a.config.HotDataThresholdDays)*24*time.Hour {
			// 温数据可节省30%
			totalSavings += float64(p.Size) / (1024 * 1024 * 1024) * a.config.CostPerGBPerMonth * 0.3
		}
		if time.Since(p.LastAccess) > time.Duration(a.config.WarmDataThresholdDays)*24*time.Hour {
			// 冷数据可额外节省40%
			totalSavings += float64(p.Size) / (1024 * 1024 * 1024) * a.config.CostPerGBPerMonth * 0.4
		}
	}

	return totalSavings
}

// GetTierRecommendation 获取分层建议.
func (a *Analyzer) GetTierRecommendation(pattern *FileAccessPattern) StorageTier {
	daysSinceAccess := time.Since(pattern.LastAccess).Hours() / 24

	if daysSinceAccess <= float64(a.config.HotDataThresholdDays) {
		return TierHot
	} else if daysSinceAccess <= float64(a.config.WarmDataThresholdDays) {
		return TierWarm
	}
	return TierCold
}

// AutoTier 自动分层.
func (a *Analyzer) AutoTier(ctx context.Context) (int, error) {
	if !a.config.EnableAutoTiering {
		return 0, fmt.Errorf("自动分层未启用")
	}

	patterns, err := a.storage.GetFileAccessPatterns()
	if err != nil {
		return 0, err
	}

	movedCount := 0
	for _, p := range patterns {
		recommendedTier := a.GetTierRecommendation(p)
		if recommendedTier != p.Tier {
			if err := a.storage.MoveToTier(p.Path, recommendedTier); err != nil {
				a.logger.Error("迁移文件失败: %s - %v", p.Path, err)
				continue
			}
			movedCount++
		}
	}

	return movedCount, nil
}
