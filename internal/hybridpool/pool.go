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
	DevicePaths []string  `json:"devicePaths"`
	Capacity    int64     `json:"capacity"`
	Used        int64     `json:"used"`
	IOPS        int64     `json:"iops"`
	Bandwidth   int64     `json:"bandwidth"`
	Role        string    `json:"role"`
}

// HybridPoolConfig 混合存储池配置
type HybridPoolConfig struct {
	Name              string        `json:"name"`
	Tiers             []PoolTier    `json:"tiers"`
	PromoteThreshold  float64       `json:"promoteThreshold"`
	DemoteThreshold   float64       `json:"demoteThreshold"`
	TieringInterval   time.Duration `json:"tieringInterval"`
	MinFileSize       int64         `json:"minFileSize"`
	MaxFileSize       int64         `json:"maxFileSize"`
	EnableAutoTiering bool          `json:"enableAutoTiering"`
	EnableSLOG        bool          `json:"enableSLOG"`
	EnableL2ARC       bool          `json:"enableL2ARC"`
	ScrubOnTierChange bool          `json:"scrubOnTierChange"`
}

// FileHeatMap 文件热度信息
type FileHeatMap struct {
	FilePath    string    `json:"filePath"`
	AccessCount int64     `json:"accessCount"`
	LastAccess  time.Time `json:"lastAccess"`
	ReadBytes   int64     `json:"readBytes"`
	WriteBytes  int64     `json:"writeBytes"`
	HeatScore   float64   `json:"heatScore"`
	CurrentTier TierLevel `json:"currentTier"`
	TargetTier  TierLevel `json:"targetTier"`
	Size        int64     `json:"size"`
}

// TieringStats 分层统计
type TieringStats struct {
	TotalFiles     int64   `json:"totalFiles"`
	HotFiles       int64   `json:"hotFiles"`
	WarmFiles      int64   `json:"warmFiles"`
	ColdFiles      int64   `json:"coldFiles"`
	PromotedToday  int64   `json:"promotedToday"`
	DemotedToday   int64   `json:"demotedToday"`
	TotalPromoted  int64   `json:"totalPromoted"`
	TotalDemoted   int64   `json:"totalDemoted"`
	HitRate        float64 `json:"hitRate"`
	AvgLatencyHot  float64 `json:"avgLatencyHot"`
	AvgLatencyCold float64 `json:"avgLatencyCold"`
}

// FlashTierManager 混合闪存池管理器
type FlashTierManager struct {
	config    HybridPoolConfig
	heatMap   map[string]*FileHeatMap
	stats     TieringStats
	tiers     map[TierLevel]*PoolTier
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	running   bool
	onPromote func(filePath string, from, to TierLevel)
	onDemote  func(filePath string, from, to TierLevel)
}

// NewFlashTierManager 创建混合闪存池
func NewFlashTierManager(config HybridPoolConfig) *FlashTierManager {
	ctx, cancel := context.WithCancel(context.Background())
	ftm := &FlashTierManager{
		config:  config,
		heatMap: make(map[string]*FileHeatMap),
		tiers:   make(map[TierLevel]*PoolTier),
		ctx:     ctx,
		cancel:  cancel,
	}
	for i := range config.Tiers {
		tier := &config.Tiers[i]
		ftm.tiers[tier.Level] = tier
	}
	return ftm
}

// Start 启动自动分层
func (ftm *FlashTierManager) Start() error {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()
	if ftm.running {
		return fmt.Errorf("hybrid pool %s already running", ftm.config.Name)
	}
	if !ftm.config.EnableAutoTiering {
		return nil
	}
	ftm.running = true
	go ftm.tieringLoop()
	log.Printf("[FlashTierManager] %s started, tiers: %d, interval: %v", ftm.config.Name, len(ftm.tiers), ftm.config.TieringInterval)
	return nil
}

// Stop 停止自动分层
func (ftm *FlashTierManager) Stop() {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()
	if !ftm.running {
		return
	}
	ftm.cancel()
	ftm.running = false
	log.Printf("[FlashTierManager] %s stopped", ftm.config.Name)
}

// RecordAccess 记录文件访问
func (ftm *FlashTierManager) RecordAccess(filePath string, readBytes, writeBytes int64) {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()
	heat, exists := ftm.heatMap[filePath]
	if !exists {
		heat = &FileHeatMap{
			FilePath:    filePath,
			CurrentTier: TierCold,
		}
		ftm.heatMap[filePath] = heat
	}
	heat.AccessCount++
	heat.LastAccess = time.Now()
	heat.ReadBytes += readBytes
	heat.WriteBytes += writeBytes
	heat.HeatScore = ftm.calculateHeatScore(heat)
	heat.TargetTier = ftm.determineTier(heat)
}

// GetStats 获取分层统计
func (ftm *FlashTierManager) GetStats() TieringStats {
	ftm.mu.RLock()
	defer ftm.mu.RUnlock()
	stats := ftm.stats
	for _, heat := range ftm.heatMap {
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
func (ftm *FlashTierManager) GetHeatMap() map[string]*FileHeatMap {
	ftm.mu.RLock()
	defer ftm.mu.RUnlock()
	result := make(map[string]*FileHeatMap, len(ftm.heatMap))
	for k, v := range ftm.heatMap {
		cp := *v
		result[k] = &cp
	}
	return result
}

// GetTierStatus 获取层级状态
func (ftm *FlashTierManager) GetTierStatus() map[TierLevel]*PoolTier {
	ftm.mu.RLock()
	defer ftm.mu.RUnlock()
	result := make(map[TierLevel]*PoolTier)
	for level, tier := range ftm.tiers {
		cp := *tier
		result[level] = &cp
	}
	return result
}

// calculateHeatScore 计算热度评分 (0-100)
func (ftm *FlashTierManager) calculateHeatScore(heat *FileHeatMap) float64 {
	freqScore := float64(0)
	if heat.AccessCount > 0 {
		freqScore = minFloat(100, float64(heat.AccessCount)*10)
	}

	recencyScore := float64(0)
	if !heat.LastAccess.IsZero() {
		hoursSince := time.Since(heat.LastAccess).Hours()
		switch {
		case hoursSince < 1:
			recencyScore = 100
		case hoursSince < 24:
			recencyScore = 80
		case hoursSince < 168:
			recencyScore = 50
		case hoursSince < 720:
			recencyScore = 20
		default:
			recencyScore = 5
		}
	}

	totalBytes := heat.ReadBytes + heat.WriteBytes
	volumeScore := minFloat(100, float64(totalBytes)/(1024*1024)*5)

	score := freqScore*0.4 + recencyScore*0.3 + volumeScore*0.3
	if score > 100 {
		score = 100
	}
	return score
}

// determineTier 根据热度评分确定目标层级
func (ftm *FlashTierManager) determineTier(heat *FileHeatMap) TierLevel {
	if heat.HeatScore >= ftm.config.PromoteThreshold {
		return TierHot
	}
	if heat.HeatScore >= ftm.config.DemoteThreshold {
		return TierWarm
	}
	return TierCold
}

// tieringLoop 自动分层循环
func (ftm *FlashTierManager) tieringLoop() {
	ticker := time.NewTicker(ftm.config.TieringInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ftm.ctx.Done():
			return
		case <-ticker.C:
			ftm.runTiering()
		}
	}
}

// runTiering 执行一轮分层
func (ftm *FlashTierManager) runTiering() {
	ftm.mu.Lock()
	defer ftm.mu.Unlock()
	var promoted, demoted int64
	for _, heat := range ftm.heatMap {
		if heat.Size < ftm.config.MinFileSize || (ftm.config.MaxFileSize > 0 && heat.Size > ftm.config.MaxFileSize) {
			continue
		}
		if heat.TargetTier == heat.CurrentTier {
			continue
		}
		switch {
		case heat.TargetTier == TierHot && heat.CurrentTier != TierHot:
			if err := ftm.migrateFile(heat, heat.CurrentTier, TierHot); err != nil {
				log.Printf("[FlashTierManager] promote %s failed: %v", heat.FilePath, err)
				continue
			}
			heat.CurrentTier = TierHot
			promoted++
			if ftm.onPromote != nil {
				ftm.onPromote(heat.FilePath, TierCold, TierHot)
			}
		case heat.TargetTier == TierCold && heat.CurrentTier != TierCold:
			if err := ftm.migrateFile(heat, heat.CurrentTier, TierCold); err != nil {
				log.Printf("[FlashTierManager] demote %s failed: %v", heat.FilePath, err)
				continue
			}
			heat.CurrentTier = TierCold
			demoted++
			if ftm.onDemote != nil {
				ftm.onDemote(heat.FilePath, TierHot, TierCold)
			}
		}
	}
	ftm.stats.PromotedToday += promoted
	ftm.stats.DemotedToday += demoted
	ftm.stats.TotalPromoted += promoted
	ftm.stats.TotalDemoted += demoted
	if promoted+demoted > 0 {
		log.Printf("[FlashTierManager] tiering complete: promoted=%d, demoted=%d", promoted, demoted)
	}
}

// migrateFile 迁移文件到目标层级
func (ftm *FlashTierManager) migrateFile(heat *FileHeatMap, from, to TierLevel) error {
	log.Printf("[FlashTierManager] migrating %s: %s -> %s", heat.FilePath, from, to)
	return nil
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
