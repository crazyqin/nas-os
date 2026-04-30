// Package hybridpool 提供混合闪存池智能分层存储功能
// 对标 TrueNAS 26 OpenZFS 2.4 Hybrid Flash Pool
// 支持 NVMe SSD + HDD 混合存储，自动分层热数据到SSD、冷数据到HDD
package hybridpool

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// TierLevel 存储层级
type TierLevel string

const (
	// TierHot 热数据层（NVMe SSD）
	TierHot TierLevel = "hot"
	// TierWarm 温数据层
	TierWarm TierLevel = "warm"
	// TierCold 冷数据层（HDD）
	TierCold TierLevel = "cold"
)

// PoolTier 存储池层级配置
type PoolTier struct {
	Level       TierLevel `json:"level"`
	DevicePaths []string  `json:"devicePaths"` // 设备路径
	Capacity    int64     `json:"capacity"`     // 总容量(字节)
	Used        int64     `json:"used"`         // 已用(字节)
	IOPS        int64     `json:"iops"`         // 当前IOPS
	Bandwidth   int64     `json:"bandwidth"`    // 当前带宽(bytes/s)
	Role        string    `json:"role"`         // data/slog/zil/l2arc
}

// HybridPoolConfig 混合存储池配置
type HybridPoolConfig struct {
	Name              string        `json:"name"`
	Tiers             []PoolTier    `json:"tiers"`
	PromoteThreshold  float64       `json:"promoteThreshold"`  // 提升到SSD的访问频率阈值(次/小时)
	DemoteThreshold   float64       `json:"demoteThreshold"`   // 降级到HDD的访问频率阈值(次/小时)
	TieringInterval   time.Duration `json:"tieringInterval"`   // 分层扫描间隔
	MinFileSize       int64         `json:"minFileSize"`       // 最小参与分层的文件大小
	MaxFileSize       int64         `json:"maxFileSize"`       // 最大参与分层的文件大小
	EnableAutoTiering bool          `json:"enableAutoTiering"` // 启用自动分层
	EnableSLOG        bool          `json:"enableSLOG"`        // 启用SSD作为SLOG
	EnableL2ARC       bool          `json:"enableL2ARC"`       // 启用SSD作为L2ARC
	ScrubOnTierChange bool          `json:"scrubOnTierChange"` // 分层变更后触发Scrub
}

// FileHeatMap 文件热度信息
type FileHeatMap struct {
	FilePath     string    `json:"filePath"`
	AccessCount  int64     `json:"accessCount"`  // 访问次数
	LastAccess   time.Time `json:"lastAccess"`   // 最后访问时间
	ReadBytes    int64     `json:"readBytes"`    // 读取字节数
	WriteBytes   int64     `json:"writeBytes"`   // 写入字节数
	HeatScore    float64   `json:"heatScore"`    // 热度评分(0-100)
	CurrentTier  TierLevel `json:"currentTier"`  // 当前所在层级
	TargetTier   TierLevel `json:"targetTier"`   // 目标层级
	Size         int64     `json:"size"`         // 文件大小
}

// TieringStats 分层统计
type TieringStats struct {
	TotalFiles     int64   `json:"totalFiles"`
	HotFiles       int64   `json:"hotFiles"`
	WarmFiles      int64   `json:"warmFiles"`
	ColdFiles      int64   `json:"coldFiles"`
	PromotedToday  int64   `json:"promotedToday"`  // 今日提升文件数
	DemotedToday   int64   `json:"demotedToday"`   // 今日降级文件数
	TotalPromoted  int64   `json:"totalPromoted"`  // 累计提升
	TotalDemoted   int64   `json:"totalDemoted"`   // 累计降级
	HitRate        float64 `json:"hitRate"`        // SSD命中率
	AvgLatencyHot  float64 `json:"avgLatencyHot"`  // 热数据平均延迟(ms)
	AvgLatencyCold float64 `json:"avgLatencyCold"` // 冷数据平均延迟(ms)
}

// HybridPool 混合闪存池管理器
type HybridPool struct {
	config    HybridPoolConfig
	heatMap   map[string]*FileHeatMap
	stats     TieringStats
	tiers     map[TierLevel]*PoolTier
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	running   bool
	onPromote func(filePath string, from, to TierLevel) // 提升回调
	onDemote  func(filePath string, from, to TierLevel) // 降级回调
}

// NewHybridPool 创建混合闪存池
func NewHybridPool(config HybridPoolConfig) *HybridPool {
	ctx, cancel := context.WithCancel(context.Background())
	hp := &HybridPool{
		config:  config,
		heatMap: make(map[string]*FileHeatMap),
		tiers:   make(map[TierLevel]*PoolTier),
		ctx:     ctx,
		cancel:  cancel,
	}
	for i := range config.Tiers {
		tier := &config.Tiers[i]
		hp.tiers[tier.Level] = tier
	}
	return hp
}

// Start 启动自动分层
func (hp *HybridPool) Start() error {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	if hp.running {
		return fmt.Errorf("hybrid pool %s already running", hp.config.Name)
	}
	if !hp.config.EnableAutoTiering {
		return nil
	}
	hp.running = true
	go hp.tieringLoop()
	log.Printf("[HybridPool] %s started, tiers: %d, interval: %v", hp.config.Name, len(hp.tiers), hp.config.TieringInterval)
	return nil
}

// Stop 停止自动分层
func (hp *HybridPool) Stop() {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	if !hp.running {
		return
	}
	hp.cancel()
	hp.running = false
	log.Printf("[HybridPool] %s stopped", hp.config.Name)
}

// RecordAccess 记录文件访问
func (hp *HybridPool) RecordAccess(filePath string, readBytes, writeBytes int64) {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	heat, exists := hp.heatMap[filePath]
	if !exists {
		heat = &FileHeatMap{
			FilePath:    filePath,
			CurrentTier: TierCold, // 默认冷数据
		}
		hp.heatMap[filePath] = heat
	}
	heat.AccessCount++
	heat.LastAccess = time.Now()
	heat.ReadBytes += readBytes
	heat.WriteBytes += writeBytes
	heat.HeatScore = hp.calculateHeatScore(heat)
	heat.TargetTier = hp.determineTier(heat)
}

// GetStats 获取分层统计
func (hp *HybridPool) GetStats() TieringStats {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	stats := hp.stats
	for _, heat := range hp.heatMap {
		switch heat.CurrentTier {
		case TierHot:
			stats.HotFiles++
		case TierWarm:
			stats.WarmFiles++
		case TierCold:
			stats.ColdFiles++
		}
		stats.TotalFiles++
	}
	if stats.TotalFiles > 0 {
		stats.HitRate = float64(stats.HotFiles) / float64(stats.TotalFiles) * 100
	}
	return stats
}

// GetHeatMap 获取文件热度图
func (hp *HybridPool) GetHeatMap() map[string]*FileHeatMap {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	result := make(map[string]*FileHeatMap, len(hp.heatMap))
	for k, v := range hp.heatMap {
		cp := *v
		result[k] = &cp
	}
	return result
}

// GetTierStatus 获取层级状态
func (hp *HybridPool) GetTierStatus() map[TierLevel]*PoolTier {
	hp.mu.RLock()
	defer hp.mu.RUnlock()
	result := make(map[TierLevel]*PoolTier)
	for level, tier := range hp.tiers {
		cp := *tier
		result[level] = &cp
	}
	return result
}

// calculateHeatScore 计算热度评分 (0-100)
// 加权算法：访问频率40% + 最近访问时间30% + 读写量30%
func (hp *HybridPool) calculateHeatScore(heat *FileHeatMap) float64 {
	// 访问频率评分 (基于对数衰减)
	freqScore := float64(0)
	if heat.AccessCount > 0 {
		freqScore = min(100, float64(heat.AccessCount)*10)
	}

	// 最近访问评分 (时间衰减)
	recencyScore := float64(0)
	if !heat.LastAccess.IsZero() {
		hoursSince := time.Since(heat.LastAccess).Hours()
		switch {
		case hoursSince < 1:
			recencyScore = 100
		case hoursSince < 24:
			recencyScore = 80
		case hoursSince < 168: // 1周
			recencyScore = 50
		case hoursSince < 720: // 1月
			recencyScore = 20
		default:
			recencyScore = 5
		}
	}

	// 读写量评分
	totalBytes := heat.ReadBytes + heat.WriteBytes
	volumeScore := min(100, float64(totalBytes)/(1024*1024)*5) // 每MB 5分

	// 加权计算: 访问频率40% + 最近访问30% + 读写量30%
	score := freqScore*0.4 + recencyScore*0.3 + volumeScore*0.3
	if score > 100 {
		score = 100
	}
	return score
}

// determineTier 根据热度评分确定目标层级
func (hp *HybridPool) determineTier(heat *FileHeatMap) TierLevel {
	if heat.HeatScore >= hp.config.PromoteThreshold {
		return TierHot
	}
	if heat.HeatScore >= hp.config.DemoteThreshold {
		return TierWarm
	}
	return TierCold
}

// tieringLoop 自动分层循环
func (hp *HybridPool) tieringLoop() {
	ticker := time.NewTicker(hp.config.TieringInterval)
	defer ticker.Stop()
	for {
		select {
		case <-hp.ctx.Done():
			return
		case <-ticker.C:
			hp.runTiering()
		}
	}
}

// runTiering 执行一轮分层
func (hp *HybridPool) runTiering() {
	hp.mu.Lock()
	defer hp.mu.Unlock()
	var promoted, demoted int64
	for _, heat := range hp.heatMap {
		// 文件大小过滤
		if heat.Size < hp.config.MinFileSize || (hp.config.MaxFileSize > 0 && heat.Size > hp.config.MaxFileSize) {
			continue
		}
		if heat.TargetTier == heat.CurrentTier {
			continue
		}
		// 执行迁移
		switch {
		case heat.TargetTier == TierHot && heat.CurrentTier != TierHot:
			// 提升到SSD
			if err := hp.migrateFile(heat, heat.CurrentTier, TierHot); err != nil {
				log.Printf("[HybridPool] promote %s failed: %v", heat.FilePath, err)
				continue
			}
			heat.CurrentTier = TierHot
			promoted++
			if hp.onPromote != nil {
				hp.onPromote(heat.FilePath, TierCold, TierHot)
			}
		case heat.TargetTier == TierCold && heat.CurrentTier != TierCold:
			// 降级到HDD
			if err := hp.migrateFile(heat, heat.CurrentTier, TierCold); err != nil {
				log.Printf("[HybridPool] demote %s failed: %v", heat.FilePath, err)
				continue
			}
			heat.CurrentTier = TierCold
			demoted++
			if hp.onDemote != nil {
				hp.onDemote(heat.FilePath, TierHot, TierCold)
			}
		}
	}
	hp.stats.PromotedToday += promoted
	hp.stats.DemotedToday += demoted
	hp.stats.TotalPromoted += promoted
	hp.stats.TotalDemoted += demoted
	if promoted+demoted > 0 {
		log.Printf("[HybridPool] tiering complete: promoted=%d, demoted=%d", promoted, demoted)
	}
}

// migrateFile 迁移文件到目标层级
func (hp *HybridPool) migrateFile(heat *FileHeatMap, from, to TierLevel) error {
	// 实际迁移逻辑：移动文件到目标设备
	// 这里是框架实现，实际需要与文件系统交互
	log.Printf("[HybridPool] migrating %s: %s -> %s", heat.FilePath, from, to)
	return nil
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
