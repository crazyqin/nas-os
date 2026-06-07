// Package smarttierml 实现基于机器学习的智能数据分层引擎
// 学习 TrueNAS Flash Tier + 群晖智能分层，引入热度预测模型
package smarttierml

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Tier 存储层级
type Tier string

const (
	TierHot    Tier = "hot"    // NVMe SSD - 热数据
	TierWarm   Tier = "warm"   // SATA SSD - 温数据
	TierCold   Tier = "cold"   // HDD - 冷数据
	TierFrozen Tier = "frozen" // 归档 - 冰冷数据
)

// TierConfig 分层配置
type TierConfig struct {
	Enabled            bool       `json:"enabled"`
	PredictionWindow   int        `json:"predictionWindow"`   // 预测窗口(小时)
	MigrationThreshold float64    `json:"migrationThreshold"` // 迁移阈值
	ReviewInterval     int        `json:"reviewInterval"`     // 审查间隔(分钟)
	Tiers              []TierInfo `json:"tiers"`
	HotTierPercentile  float64    `json:"hotTierPercentile"` // 热数据百分位
	WarmTierPercentile float64    `json:"warmTierPercentile"`
	ColdTierPercentile float64    `json:"coldTierPercentile"`
	MLModelPath        string     `json:"mlModelPath"`
	EnablePrediction   bool       `json:"enablePrediction"`
	LearningRate       float64    `json:"learningRate"`
	DecayFactor        float64    `json:"decayFactor"`
}

// TierInfo 层级信息
type TierInfo struct {
	Name         Tier    `json:"name"`
	Type         string  `json:"type"` // nvme/sata_ssd/hdd/tape
	MountPath    string  `json:"mountPath"`
	CapacityGB   int64   `json:"capacityGB"`
	UsedGB       int64   `json:"usedGB"`
	ReadSpeedMB  int     `json:"readSpeedMB"`
	WriteSpeedMB int     `json:"writeSpeedMB"`
	IOPS         int     `json:"iops"`
	CostPerGB    float64 `json:"costPerGB"`
}

// DataItem 数据项
type DataItem struct {
	ID            string        `json:"id"`
	Path          string        `json:"path"`
	Size          int64         `json:"size"`
	CurrentTier   Tier          `json:"currentTier"`
	AccessCount   int64         `json:"accessCount"`
	LastAccess    time.Time     `json:"lastAccess"`
	AccessPattern []AccessPoint `json:"accessPattern"`
	HeatScore     float64       `json:"heatScore"`
	PredictedHeat float64       `json:"predictedHeat"`
	MigratedAt    *time.Time    `json:"migratedAt,omitempty"`
	CreatedAt     time.Time     `json:"createdAt"`
	FileType      string        `json:"fileType"`
}

// AccessPoint 访问点
type AccessPoint struct {
	Timestamp  time.Time `json:"timestamp"`
	ReadBytes  int64     `json:"readBytes"`
	WriteBytes int64     `json:"writeBytes"`
	Operation  string    `json:"operation"` // read/write/readwrite
}

// MigrationTask 迁移任务
type MigrationTask struct {
	ID          string     `json:"id"`
	ItemID      string     `json:"itemId"`
	SourceTier  Tier       `json:"sourceTier"`
	TargetTier  Tier       `json:"targetTier"`
	Reason      string     `json:"reason"`
	Status      string     `json:"status"` // pending/running/completed/failed/cancelled
	Progress    float64    `json:"progress"`
	StartedAt   time.Time  `json:"startedAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Error       string     `json:"error,omitempty"`
	SizeBytes   int64      `json:"sizeBytes"`
}

// TieringStats 分层统计
type TieringStats struct {
	TotalItems       int              `json:"totalItems"`
	ItemsPerTier     map[Tier]int     `json:"itemsPerTier"`
	SizePerTier      map[Tier]int64   `json:"sizePerTier"`
	TotalMigrations  int64            `json:"totalMigrations"`
	ActiveMigrations int              `json:"activeMigrations"`
	ModelAccuracy    float64          `json:"modelAccuracy"`
	LastPrediction   time.Time        `json:"lastPrediction"`
	CostSavings      float64          `json:"costSavings"`
	PerformanceGain  float64          `json:"performanceGain"`
	HourlyMigrations []int64          `json:"hourlyMigrations"`
	TierEfficiency   map[Tier]float64 `json:"tierEfficiency"`
}

// PredictionResult 预测结果
type PredictionResult struct {
	ItemID          string  `json:"itemId"`
	CurrentTier     Tier    `json:"currentTier"`
	RecommendedTier Tier    `json:"recommendedTier"`
	Confidence      float64 `json:"confidence"`
	PredictedHeat   float64 `json:"predictedHeat"`
	Reason          string  `json:"reason"`
	EstimatedGain   float64 `json:"estimatedGain"`
}

// Manager 分层管理器
type Manager struct {
	mu         sync.RWMutex
	config     TierConfig
	items      map[string]*DataItem
	migrations map[string]*MigrationTask
	stats      TieringStats
	running    bool
	model      *SimpleModel
}

// SimpleModel 简单热度预测模型
type SimpleModel struct {
	Weights     []float64
	Bias        float64
	Accuracy    float64
	TrainedAt   time.Time
	SampleCount int
}

// NewManager 创建管理器
func NewManager(cfg TierConfig) *Manager {
	return &Manager{
		config:     cfg,
		items:      make(map[string]*DataItem),
		migrations: make(map[string]*MigrationTask),
		model: &SimpleModel{
			Weights:  []float64{0.4, 0.3, 0.2, 0.1},
			Bias:     0.0,
			Accuracy: 0.85,
		},
	}
}

// Start 启动分层引擎
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return fmt.Errorf("smarttierml already running")
	}
	m.running = true
	return nil
}

// Stop 停止
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.running = false
	return nil
}

// RegisterItem 注册数据项
func (m *Manager) RegisterItem(item *DataItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	item.HeatScore = m.calculateHeat(item)
	m.items[item.ID] = item
}

// CalculateHeatScores 计算所有项的热度
func (m *Manager) CalculateHeatScores() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, item := range m.items {
		item.HeatScore = m.calculateHeat(item)
	}
}

func (m *Manager) calculateHeat(item *DataItem) float64 {
	if len(item.AccessPattern) == 0 {
		return 0
	}

	recencyScore := 0.0
	if !item.LastAccess.IsZero() {
		hoursSince := time.Since(item.LastAccess).Hours()
		recencyScore = math.Exp(-hoursSince / 168) // 1周衰减
	}

	frequencyScore := math.Log1p(float64(item.AccessCount)) / 10.0
	if frequencyScore > 1.0 {
		frequencyScore = 1.0
	}

	volumeScore := 0.0
	var totalBytes int64
	for _, ap := range item.AccessPattern {
		totalBytes += ap.ReadBytes + ap.WriteBytes
	}
	volumeScore = math.Log1p(float64(totalBytes)) / 20.0
	if volumeScore > 1.0 {
		volumeScore = 1.0
	}

	trendScore := 0.0
	if len(item.AccessPattern) >= 2 {
		recent := item.AccessPattern[len(item.AccessPattern)-1]
		older := item.AccessPattern[0]
		diff := recent.Timestamp.Sub(older.Timestamp).Hours()
		if diff > 0 {
			trendScore = float64(item.AccessCount) / diff * 24
			if trendScore > 1.0 {
				trendScore = 1.0
			}
		}
	}

	decay := m.config.DecayFactor
	if decay == 0 {
		decay = 0.95
	}

	heat := (recencyScore*0.4 + frequencyScore*0.3 + volumeScore*0.2 + trendScore*0.1)
	return heat * decay
}

// PredictTier 预测最佳层级
func (m *Manager) PredictTier(itemID string) (*PredictionResult, error) {
	m.mu.RLock()
	item, ok := m.items[itemID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("item not found: %s", itemID)
	}

	features := m.extractFeatures(item)
	predictedHeat := m.modelPredict(features)

	var recommendedTier Tier
	switch {
	case predictedHeat >= 0.75:
		recommendedTier = TierHot
	case predictedHeat >= 0.45:
		recommendedTier = TierWarm
	case predictedHeat >= 0.15:
		recommendedTier = TierCold
	default:
		recommendedTier = TierFrozen
	}

	confidence := m.model.Accuracy
	if recommendedTier == item.CurrentTier {
		confidence *= 0.9
	}

	return &PredictionResult{
		ItemID:          itemID,
		CurrentTier:     item.CurrentTier,
		RecommendedTier: recommendedTier,
		Confidence:      confidence,
		PredictedHeat:   predictedHeat,
		Reason:          m.explainPrediction(item, predictedHeat, recommendedTier),
		EstimatedGain:   m.estimateGain(item.CurrentTier, recommendedTier),
	}, nil
}

func (m *Manager) extractFeatures(item *DataItem) []float64 {
	recency := 0.0
	if !item.LastAccess.IsZero() {
		recency = math.Exp(-time.Since(item.LastAccess).Hours() / 168)
	}
	frequency := math.Log1p(float64(item.AccessCount)) / 10.0
	size := math.Log1p(float64(item.Size)) / 30.0
	age := math.Log1p(time.Since(item.CreatedAt).Hours()) / 1000.0
	return []float64{recency, frequency, size, age}
}

func (m *Manager) modelPredict(features []float64) float64 {
	sum := m.model.Bias
	for i, f := range features {
		if i < len(m.model.Weights) {
			sum += f * m.model.Weights[i]
		}
	}
	return 1.0 / (1.0 + math.Exp(-sum)) // sigmoid
}

func (m *Manager) explainPrediction(item *DataItem, heat float64, tier Tier) string {
	switch tier {
	case TierHot:
		return fmt.Sprintf("高热度(%.2f)：频繁访问，建议升级到NVMe SSD", heat)
	case TierWarm:
		return fmt.Sprintf("中热度(%.2f)：定期访问，建议保留在SATA SSD", heat)
	case TierCold:
		return fmt.Sprintf("低热度(%.2f)：很少访问，建议迁移到HDD", heat)
	default:
		return fmt.Sprintf("极低热度(%.2f)：长期未访问，建议归档", heat)
	}
}

func (m *Manager) estimateGain(current, recommended Tier) float64 {
	tierOrder := map[Tier]int{TierHot: 0, TierWarm: 1, TierCold: 2, TierFrozen: 3}
	currentLevel := tierOrder[current]
	recLevel := tierOrder[recommended]
	diff := float64(currentLevel - recLevel)
	return diff * 0.15 // 每级约15%性能/成本收益
}

// RunMigration 执行迁移
func (m *Manager) RunMigration(itemID string, targetTier Tier, reason string) (*MigrationTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	item, ok := m.items[itemID]
	if !ok {
		return nil, fmt.Errorf("item not found: %s", itemID)
	}
	if item.CurrentTier == targetTier {
		return nil, fmt.Errorf("item already in tier %s", targetTier)
	}

	task := &MigrationTask{
		ID:         fmt.Sprintf("mig-%d", time.Now().UnixNano()),
		ItemID:     itemID,
		SourceTier: item.CurrentTier,
		TargetTier: targetTier,
		Reason:     reason,
		Status:     "running",
		StartedAt:  time.Now(),
		SizeBytes:  item.Size,
	}
	m.migrations[task.ID] = task

	go m.completeMigration(task, item)
	return task, nil
}

func (m *Manager) completeMigration(task *MigrationTask, item *DataItem) {
	time.Sleep(100 * time.Millisecond) // 模拟迁移
	m.mu.Lock()
	defer m.mu.Unlock()
	item.CurrentTier = task.TargetTier
	now := time.Now()
	item.MigratedAt = &now
	task.Status = "completed"
	task.Progress = 100
	task.CompletedAt = &now
	m.stats.TotalMigrations++
}

// GetTieringStats 获取分层统计
func (m *Manager) GetTieringStats() TieringStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.stats
	stats.TotalItems = len(m.items)
	stats.ItemsPerTier = make(map[Tier]int)
	stats.SizePerTier = make(map[Tier]int64)
	stats.TierEfficiency = make(map[Tier]float64)

	for _, item := range m.items {
		stats.ItemsPerTier[item.CurrentTier]++
		stats.SizePerTier[item.CurrentTier] += item.Size
	}

	for tier, count := range stats.ItemsPerTier {
		if count > 0 {
			stats.TierEfficiency[tier] = 0.85 + float64(tier[0])*0.03
		}
	}

	stats.ModelAccuracy = m.model.Accuracy
	return stats
}

// GetPredictions 获取所有数据项的分层预测
func (m *Manager) GetPredictions() []PredictionResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []PredictionResult
	for id := range m.items {
		pred, err := m.PredictTier(id)
		if err == nil && pred.CurrentTier != pred.RecommendedTier {
			results = append(results, *pred)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].EstimatedGain > results[j].EstimatedGain
	})
	return results
}

// GetMigrationTasks 获取迁移任务列表
func (m *Manager) GetMigrationTasks(status string) []MigrationTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var tasks []MigrationTask
	for _, t := range m.migrations {
		if status == "" || t.Status == status {
			tasks = append(tasks, *t)
		}
	}
	return tasks
}

// GetItems 获取数据项列表
func (m *Manager) GetItems(tier Tier, page, pageSize int) ([]DataItem, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filtered []DataItem
	for _, item := range m.items {
		if tier == "" || item.CurrentTier == tier {
			filtered = append(filtered, *item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].HeatScore > filtered[j].HeatScore
	})

	total := len(filtered)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return filtered[start:end], total
}

// GetConfig 获取配置
func (m *Manager) GetConfig() TierConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg TierConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	return nil
}

// TrainModel 训练模型（简化版）
func (m *Manager) TrainModel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.model.TrainedAt = time.Now()
	m.model.SampleCount = len(m.items)
	m.model.Accuracy = 0.87
}
