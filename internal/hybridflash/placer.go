// Package hybridflash 提供 SSD/HDD 智能混合分层存储管理.
//
// SmartDataPlacer: 智能数据放置器，根据访问模式和 ML 预测自动迁移数据.
package hybridflash

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SmartDataPlacer 智能数据放置器.
//
// 结合 ML 预测、热度分析和成本模型，实现最优数据放置策略.
type SmartDataPlacer struct {
	mu              sync.RWMutex
	logger          *zap.Logger
	mlEngine        *MLTieringEngine
	config          *PlacerConfig
	migrationQueue  chan *PlacementDecision
	activePlacements map[string]*PlacementDecision
	completedPlacements []*PlacementDecision
	placementStats  *PlacementStats
}

// PlacerConfig 放置器配置.
type PlacerConfig struct {
	// Enabled 启用智能放置.
	Enabled bool `json:"enabled"`
	// MigrationBatchSize 每批迁移的块数.
	MigrationBatchSize int `json:"migrationBatchSize"`
	// MaxConcurrentMigrations 最大并发迁移数.
	MaxConcurrentMigrations int `json:"maxConcurrentMigrations"`
	// MinBenefitThreshold 最小收益阈值 (%).
	MinBenefitThreshold float64 `json:"minBenefitThreshold"`
	// CooldownPeriod 迁移冷却期（防止频繁迁移）.
	CooldownPeriod time.Duration `json:"cooldownPeriod"`
	// EnablePrefetch 启用预取（基于 ML 预测）.
	EnablePrefetch bool `json:"enablePrefetch"`
	// PrefetchWindow 预取窗口.
	PrefetchWindow time.Duration `json:"prefetchWindow"`
	// CostWeight 成本权重（用于放置决策）.
	CostWeight float64 `json:"costWeight"`
	// PerformanceWeight 性能权重.
	PerformanceWeight float64 `json:"performanceWeight"`
	// EnableAdaptiveLearning 启用自适应学习.
	EnableAdaptiveLearning bool `json:"enableAdaptiveLearning"`
}

// DefaultPlacerConfig 默认放置器配置.
func DefaultPlacerConfig() *PlacerConfig {
	return &PlacerConfig{
		Enabled:                 true,
		MigrationBatchSize:      100,
		MaxConcurrentMigrations: 4,
		MinBenefitThreshold:     10.0,
		CooldownPeriod:          30 * time.Minute,
		EnablePrefetch:          true,
		PrefetchWindow:          1 * time.Hour,
		CostWeight:              0.3,
		PerformanceWeight:       0.7,
		EnableAdaptiveLearning:  true,
	}
}

// PlacementDecision 放置决策.
type PlacementDecision struct {
	ID              string         `json:"id"`
	BlockID         string         `json:"blockId"`
	CurrentTier     FlashType      `json:"currentTier"`
	TargetTier      FlashType      `json:"targetTier"`
	Reason          string         `json:"reason"`
	PredictedBenefit float64       `json:"predictedBenefit"`
	Cost            float64        `json:"cost"`
	Confidence      float64        `json:"confidence"`
	CreatedAt       time.Time      `json:"createdAt"`
	StartedAt       time.Time      `json:"startedAt,omitempty"`
	CompletedAt     time.Time      `json:"completedAt,omitempty"`
	Status          PlacementStatus `json:"status"`
	Prediction      *PredictionResult `json:"prediction,omitempty"`
}

// PlacementStatus 放置状态.
type PlacementStatus string

const (
	PlacementStatusPending   PlacementStatus = "pending"
	PlacementStatusRunning   PlacementStatus = "running"
	PlacementStatusCompleted PlacementStatus = "completed"
	PlacementStatusFailed    PlacementStatus = "failed"
	PlacementStatusCancelled PlacementStatus = "cancelled"
)

// PlacementStats 放置统计.
type PlacementStats struct {
	mu                   sync.RWMutex
	TotalPlacements      int64   `json:"totalPlacements"`
	SuccessfulPlacements int64   `json:"successfulPlacements"`
	FailedPlacements     int64   `json:"failedPlacements"`
	TotalBytesMigrated   int64   `json:"totalBytesMigrated"`
	AvgBenefit           float64 `json:"avgBenefit"`
	AvgMigrationTime     float64 `json:"avgMigrationTime"` // 毫秒
	HitRateImprovement   float64 `json:"hitRateImprovement"`
}

// NewSmartDataPlacer 创建智能数据放置器.
func NewSmartDataPlacer(logger *zap.Logger, mlEngine *MLTieringEngine, config *PlacerConfig) *SmartDataPlacer {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultPlacerConfig()
	}

	return &SmartDataPlacer{
		logger:           logger,
		mlEngine:         mlEngine,
		config:           config,
		migrationQueue:   make(chan *PlacementDecision, 1000),
		activePlacements: make(map[string]*PlacementDecision),
		completedPlacements: make([]*PlacementDecision, 0, 1000),
		placementStats:   &PlacementStats{},
	}
}

// AnalyzeAndPlace 分析数据并生成放置建议.
func (p *SmartDataPlacer) AnalyzeAndPlace(poolID string, blocks []*BlockAccessRecord) []*PlacementDecision {
	if !p.config.Enabled {
		return nil
	}

	decisions := make([]*PlacementDecision, 0, len(blocks))

	for _, block := range blocks {
		// 获取 ML 预测
		prediction := p.mlEngine.Predict(block.BlockID)

		// 生成放置决策
		decision := p.makeDecision(block, prediction)

		if decision != nil {
			decisions = append(decisions, decision)
		}
	}

	// 按收益排序
	sortDecisionsByBenefit(decisions)

	// 限制批次大小
	if len(decisions) > p.config.MigrationBatchSize {
		decisions = decisions[:p.config.MigrationBatchSize]
	}

	p.logger.Info("生成放置决策",
		zap.String("poolId", poolID),
		zap.Int("totalBlocks", len(blocks)),
		zap.Int("decisions", len(decisions)),
	)

	return decisions
}

// makeDecision 生成单个放置决策.
func (p *SmartDataPlacer) makeDecision(block *BlockAccessRecord, prediction *PredictionResult) *PlacementDecision {
	// 检查是否需要迁移
	if prediction.PredictedTier == block.CurrentTier {
		return nil // 已在正确层级
	}

	// 检查置信度
	if prediction.Confidence < 0.5 {
		return nil // 置信度不足
	}

	// 计算收益
	benefit := p.calculateBenefit(block, prediction)

	// 检查收益阈值
	if benefit < p.config.MinBenefitThreshold {
		return nil
	}

	// 计算迁移成本
	cost := p.calculateCost(block)

	// 综合评分
	score := benefit*p.config.PerformanceWeight - cost*p.config.CostWeight
	if score <= 0 {
		return nil
	}

	return &PlacementDecision{
		ID:               fmt.Sprintf("placement-%s-%d", block.BlockID, time.Now().UnixNano()),
		BlockID:          block.BlockID,
		CurrentTier:      block.CurrentTier,
		TargetTier:       prediction.PredictedTier,
		Reason:           prediction.Recommendation,
		PredictedBenefit: benefit,
		Cost:             cost,
		Confidence:       prediction.Confidence,
		CreatedAt:        time.Now(),
		Status:           PlacementStatusPending,
		Prediction:       prediction,
	}
}

// calculateBenefit 计算迁移收益.
func (p *SmartDataPlacer) calculateBenefit(block *BlockAccessRecord, prediction *PredictionResult) float64 {
	// 基础收益来自 ML 预测
	baseBenefit := prediction.EstimatedBenefit

	// 访问频率加成
	freqBonus := float64(block.AccessCount) / 100.0
	if freqBonus > 2.0 {
		freqBonus = 2.0
	}

	// 热度级别加成
	heatBonus := 1.0
	switch block.HeatLevel {
	case HeatLevelHot:
		heatBonus = 2.0
	case HeatLevelWarm:
		heatBonus = 1.5
	case HeatLevelCold:
		heatBonus = 1.0
	case HeatLevelFrozen:
		heatBonus = 0.5
	}

	return baseBenefit * freqBonus * heatBonus
}

// calculateCost 计算迁移成本.
func (p *SmartDataPlacer) calculateCost(block *BlockAccessRecord) float64 {
	// 基础成本（IO 开销）
	ioCost := float64(block.Size) / (1024 * 1024) // 每 MB 的成本

	// 当前层级到目标层级的迁移成本
	tierCost := getTierMigrationCost(block.CurrentTier, FlashTypeNVMe)

	return ioCost * tierCost
}

// getTierMigrationCost 获取层级间迁移成本.
func getTierMigrationCost(from, to FlashType) float64 {
	costMap := map[FlashType]map[FlashType]float64{
		FlashTypeHDD: {
			FlashTypeSSD:  1.0,
			FlashTypeNVMe: 1.5,
		},
		FlashTypeSSD: {
			FlashTypeHDD:  0.5,
			FlashTypeNVMe: 0.8,
		},
		FlashTypeNVMe: {
			FlashTypeHDD: 0.3,
			FlashTypeSSD: 0.4,
		},
	}

	if fromCost, ok := costMap[from]; ok {
		if cost, ok := fromCost[to]; ok {
			return cost
		}
	}

	return 1.0
}

// ExecutePlacements 执行放置决策.
func (p *SmartDataPlacer) ExecutePlacements(decisions []*PlacementDecision) {
	if !p.config.Enabled {
		return
	}

	for _, decision := range decisions {
		p.migrationQueue <- decision
	}

	p.logger.Info("已提交放置任务",
		zap.Int("count", len(decisions)),
	)
}

// Start 启动放置器工作线程.
func (p *SmartDataPlacer) Start() {
	go p.workerLoop()
	p.logger.Info("智能数据放置器已启动")
}

// workerLoop 工作循环.
func (p *SmartDataPlacer) workerLoop() {
	semaphore := make(chan struct{}, p.config.MaxConcurrentMigrations)

	for {
		decision, ok := <-p.migrationQueue
		if !ok {
			return
		}

		semaphore <- struct{}{}

		go func(d *PlacementDecision) {
			defer func() { <-semaphore }()
			p.executePlacement(d)
		}(decision)
	}
}

// executePlacement 执行单个放置.
func (p *SmartDataPlacer) executePlacement(decision *PlacementDecision) {
	p.mu.Lock()
	decision.Status = PlacementStatusRunning
	decision.StartedAt = time.Now()
	p.activePlacements[decision.ID] = decision
	p.mu.Unlock()

	defer func() {
		p.mu.Lock()
		delete(p.activePlacements, decision.ID)
		p.completedPlacements = append(p.completedPlacements, decision)
		// 保留最近 1000 条
		if len(p.completedPlacements) > 1000 {
			p.completedPlacements = p.completedPlacements[len(p.completedPlacements)-1000:]
		}
		p.mu.Unlock()
	}()

	// 模拟迁移（实际应调用存储层 API）
	p.logger.Debug("执行放置",
		zap.String("blockId", decision.BlockID),
		zap.String("from", string(decision.CurrentTier)),
		zap.String("to", string(decision.TargetTier)),
	)

	// 更新统计
	p.placementStats.mu.Lock()
	p.placementStats.TotalPlacements++
	p.placementStats.SuccessfulPlacements++
	p.placementStats.TotalBytesMigrated += int64(decision.Prediction.Features.DataSize)
	p.placementStats.AvgBenefit = (p.placementStats.AvgBenefit*float64(p.placementStats.SuccessfulPlacements-1) +
		decision.PredictedBenefit) / float64(p.placementStats.SuccessfulPlacements)
	p.placementStats.mu.Unlock()

	decision.Status = PlacementStatusCompleted
	decision.CompletedAt = time.Now()

	// 收集训练样本（自适应学习）
	if p.config.EnableAdaptiveLearning {
		isHot := decision.TargetTier == FlashTypeNVMe || decision.TargetTier == FlashTypeSSD
		p.mlEngine.CollectTrainingSample(decision.BlockID, isHot)
	}
}

// GetStats 获取放置统计.
func (p *SmartDataPlacer) GetStats() *PlacementStats {
	p.placementStats.mu.RLock()
	defer p.placementStats.mu.RUnlock()

	return &PlacementStats{
		TotalPlacements:      p.placementStats.TotalPlacements,
		SuccessfulPlacements: p.placementStats.SuccessfulPlacements,
		FailedPlacements:     p.placementStats.FailedPlacements,
		TotalBytesMigrated:   p.placementStats.TotalBytesMigrated,
		AvgBenefit:           p.placementStats.AvgBenefit,
		AvgMigrationTime:     p.placementStats.AvgMigrationTime,
		HitRateImprovement:   p.placementStats.HitRateImprovement,
	}
}

// GetActivePlacements 获取活跃放置任务.
func (p *SmartDataPlacer) GetActivePlacements() []*PlacementDecision {
	p.mu.RLock()
	defer p.mu.RUnlock()

	placements := make([]*PlacementDecision, 0, len(p.activePlacements))
	for _, d := range p.activePlacements {
		placements = append(placements, d)
	}

	return placements
}

// GetRecentPlacements 获取最近完成的放置.
func (p *SmartDataPlacer) GetRecentPlacements(limit int) []*PlacementDecision {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if limit <= 0 || limit > len(p.completedPlacements) {
		limit = len(p.completedPlacements)
	}

	start := len(p.completedPlacements) - limit
	if start < 0 {
		start = 0
	}

	return p.completedPlacements[start:]
}

// UpdateConfig 更新配置.
func (p *SmartDataPlacer) UpdateConfig(config *PlacerConfig) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.config = config
}

// sortDecisionsByBenefit 按收益降序排序决策.
func sortDecisionsByBenefit(decisions []*PlacementDecision) {
	// 简单冒泡排序（决策数量通常不大）
	for i := 0; i < len(decisions); i++ {
		for j := i + 1; j < len(decisions); j++ {
			if decisions[j].PredictedBenefit > decisions[i].PredictedBenefit {
				decisions[i], decisions[j] = decisions[j], decisions[i]
			}
		}
	}
}

// PredictAndPrefetch 预测并预取数据.
func (p *SmartDataPlacer) PredictAndPrefetch(poolID string, blocks []*BlockAccessRecord) {
	if !p.config.Enabled || !p.config.EnablePrefetch {
		return
	}

	for _, block := range blocks {
		prediction := p.mlEngine.Predict(block.BlockID)

		// 高概率热数据预取到快速层
		if prediction.HotProbability > 0.8 && prediction.Confidence > 0.7 {
			if block.CurrentTier == FlashTypeHDD {
				decision := &PlacementDecision{
					ID:               fmt.Sprintf("prefetch-%s-%d", block.BlockID, time.Now().UnixNano()),
					BlockID:          block.BlockID,
					CurrentTier:      block.CurrentTier,
					TargetTier:       prediction.PredictedTier,
					Reason:           "预取: 预测为热数据",
					PredictedBenefit: prediction.EstimatedBenefit,
					Confidence:       prediction.Confidence,
					CreatedAt:        time.Now(),
					Status:           PlacementStatusPending,
					Prediction:       prediction,
				}

				p.migrationQueue <- decision

				p.logger.Debug("预取数据",
					zap.String("blockId", block.BlockID),
					zap.Float64("hotProb", prediction.HotProbability),
				)
			}
		}
	}
}
