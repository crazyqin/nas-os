// Package prediction 提供智能存储预测功能
// 实现存储使用趋势分析、容量预测和智能优化建议
package prediction

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// Manager 预测管理器
type Manager struct {
	mu sync.RWMutex

	// 历史数据存储
	history *HistoryStore

	// 预测配置
	config *Config

	// 预测模型
	model *PredictionModel

	// 异常检测器
	anomalyDetector *AnomalyDetector

	// 优化建议生成器
	advisor *Advisor

	// 停止信号
	stopChan chan struct{}

	// 是否已初始化
	initialized bool
}

// Config 预测配置
type Config struct {
	// 数据收集间隔
	CollectionInterval time.Duration `json:"collectionInterval"`

	// 历史数据保留天数
	HistoryRetentionDays int `json:"historyRetentionDays"`

	// 预测时间范围（天）
	PredictionDays int `json:"predictionDays"`

	// 异常检测灵敏度（0-1）
	AnomalySensitivity float64 `json:"anomalySensitivity"`

	// 预警阈值（使用率百分比）
	WarningThreshold  float64 `json:"warningThreshold"`
	CriticalThreshold float64 `json:"criticalThreshold"`

	// 趋势分析窗口（天）
	TrendWindowDays int `json:"trendWindowDays"`

	// 是否启用自动优化建议
	EnableAutoAdvice bool `json:"enableAutoAdvice"`
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		CollectionInterval:   5 * time.Minute,
		HistoryRetentionDays: 90,
		PredictionDays:       30,
		AnomalySensitivity:   0.8,
		WarningThreshold:     75.0,
		CriticalThreshold:    90.0,
		TrendWindowDays:      7,
		EnableAutoAdvice:     true,
	}
}

// HistoryStore 历史数据存储
type HistoryStore struct {
	mu sync.RWMutex

	// 按卷名存储历史数据
	VolumeData map[string]*VolumeHistory

	// 全局历史数据
	GlobalData *GlobalHistory
}

// VolumeHistory 卷历史数据
type VolumeHistory struct {
	VolumeName string `json:"volumeName"`

	// 使用率历史（时间戳 -> 使用率）
	UsageHistory []UsageRecord `json:"usageHistory"`

	// 容量历史
	CapacityHistory []CapacityRecord `json:"capacityHistory"`

	// I/O 历史统计
	IOHistory []IORecord `json:"ioHistory"`

	// 最后更新时间
	LastUpdated time.Time `json:"lastUpdated"`
}

// UsageRecord 使用率记录
type UsageRecord struct {
	Timestamp time.Time `json:"timestamp"`
	UsedGB    float64   `json:"usedGB"`
	TotalGB   float64   `json:"totalGB"`
	UsageRate float64   `json:"usageRate"` // 百分比
}

// CapacityRecord 容量记录
type CapacityRecord struct {
	Timestamp time.Time `json:"timestamp"`
	TotalGB   float64   `json:"totalGB"`
	FreeGB    float64   `json:"freeGB"`
}

// IORecord I/O 记录
type IORecord struct {
	Timestamp time.Time `json:"timestamp"`
	ReadMBps  float64   `json:"readMBps"`
	WriteMBps float64   `json:"writeMBps"`
	IOPS      uint64    `json:"iops"`
}

// GlobalHistory 全局历史数据
type GlobalHistory struct {
	// 总容量历史
	TotalCapacityHistory []CapacityRecord `json:"totalCapacityHistory"`

	// 最后更新时间
	LastUpdated time.Time `json:"lastUpdated"`
}

// PredictionModel 预测模型
type PredictionModel struct {
	mu sync.RWMutex

	// 线性回归参数
	slope     float64
	intercept float64

	// 季节性参数
	seasonalityEnabled bool
	seasonalPeriod     int // 季节周期（天）
	seasonalFactors    []float64

	// 模型准确度
	accuracy float64

	// 最后训练时间
	lastTrained time.Time
}

// AnomalyDetector 异常检测器
type AnomalyDetector struct {
	mu sync.RWMutex

	// 基线值
	baselineMean   float64
	baselineStdDev float64

	// 异常阈值（标准差倍数）
	threshold float64

	// 最近检测到的异常
	anomalies []Anomaly

	// 配置
	sensitivity float64
}

// Anomaly 异常记录
type Anomaly struct {
	Timestamp   time.Time `json:"timestamp"`
	Type        string    `json:"type"`        // "usage_spike", "usage_drop", "growth_rate", etc.
	Severity    string    `json:"severity"`    // "low", "medium", "high", "critical"
	Value       float64   `json:"value"`       // 实际值
	Expected    float64   `json:"expected"`    // 期望值
	Deviation   float64   `json:"deviation"`   // 偏差程度
	Description string    `json:"description"` // 人类可读描述
}

// Advisor 优化建议生成器
type Advisor struct {
	mu sync.RWMutex

	// 建议规则
	rules []AdviceRule

	// 生成的建议
	advices []Advice
}

// AdviceRule 建议规则
type AdviceRule struct {
	Name      string `json:"name"`
	Condition func(*PredictionResult) bool
	Generate  func(*PredictionResult) Advice
	Priority  int    `json:"priority"` // 优先级（1-10，越高越重要）
	Category  string `json:"category"` // "capacity", "performance", "cost", "security"
}

// Advice 优化建议
type Advice struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Category    string    `json:"category"`    // "capacity", "performance", "cost", "security"
	Priority    int       `json:"priority"`    // 1-10
	Title       string    `json:"title"`       // 简短标题
	Description string    `json:"description"` // 详细描述
	Action      string    `json:"action"`      // 建议的操作
	Impact      string    `json:"impact"`      // 预期影响
	Savings     string    `json:"savings"`     // 预计节省（容量/成本）
	Applied     bool      `json:"applied"`     // 是否已应用
}

// PredictionResult 预测结果
type PredictionResult struct {
	VolumeName string `json:"volumeName"`

	// 当前状态
	CurrentUsage     float64   `json:"currentUsage"`     // 当前使用量 GB
	CurrentTotal     float64   `json:"currentTotal"`     // 当前总容量 GB
	CurrentUsageRate float64   `json:"currentUsageRate"` // 当前使用率 %
	Timestamp        time.Time `json:"timestamp"`

	// 趋势分析
	Trend             string  `json:"trend"`             // "increasing", "decreasing", "stable"
	GrowthRateDaily   float64 `json:"growthRateDaily"`   // 日增长率 GB/天
	GrowthRateWeekly  float64 `json:"growthRateWeekly"`  // 周增长率 GB/周
	GrowthRateMonthly float64 `json:"growthRateMonthly"` // 月增长率 GB/月

	// 预测值
	PredictedUsage     []PredictedPoint `json:"predictedUsage"`
	FullInDays         int              `json:"fullInDays"`         // 预计多少天后满
	WarningInDays      int              `json:"warningInDays"`      // 预计多少天达到预警阈值
	CriticalInDays     int              `json:"criticalInDays"`     // 预计多少天达到危险阈值
	PredictedUsage30d  float64          `json:"predictedUsage30d"`  // 30天后预计使用量 GB
	PredictedUsage90d  float64          `json:"predictedUsage90d"`  // 90天后预计使用量 GB
	PredictedUsage365d float64          `json:"predictedUsage365d"` // 365天后预计使用量 GB

	// 异常信息
	Anomalies []Anomaly `json:"anomalies"`

	// 优化建议
	Advices []Advice `json:"advices"`

	// 模型置信度
	Confidence float64 `json:"confidence"` // 0-1
}

// PredictedPoint 预测数据点
type PredictedPoint struct {
	Date      time.Time `json:"date"`
	UsageGB   float64   `json:"usageGB"`
	UsageRate float64   `json:"usageRate"` // 预计使用率 %
}

// NewManager 创建预测管理器
func NewManager(cfg *Config) (*Manager, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 验证配置
	if err := validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	m := &Manager{
		config:   cfg,
		stopChan: make(chan struct{}),
		history: &HistoryStore{
			VolumeData: make(map[string]*VolumeHistory),
			GlobalData: &GlobalHistory{},
		},
		model: &PredictionModel{
			seasonalityEnabled: true,
			seasonalPeriod:     7, // 周周期
		},
		anomalyDetector: &AnomalyDetector{
			threshold:   2.0, // 2倍标准差
			sensitivity: cfg.AnomalySensitivity,
			anomalies:   make([]Anomaly, 0),
		},
		advisor: &Advisor{
			rules:   getDefaultAdviceRules(),
			advices: make([]Advice, 0),
		},
	}

	// 启动后台数据收集
	go m.startCollection()

	m.initialized = true
	return m, nil
}

// validateConfig 验证配置
func validateConfig(cfg *Config) error {
	if cfg.CollectionInterval < time.Minute {
		return fmt.Errorf("数据收集间隔不能小于1分钟")
	}
	if cfg.HistoryRetentionDays < 1 {
		return fmt.Errorf("历史数据保留天数不能小于1天")
	}
	if cfg.PredictionDays < 1 {
		return fmt.Errorf("预测天数不能小于1天")
	}
	if cfg.AnomalySensitivity < 0 || cfg.AnomalySensitivity > 1 {
		return fmt.Errorf("异常检测灵敏度必须在0-1之间")
	}
	if cfg.WarningThreshold >= cfg.CriticalThreshold {
		return fmt.Errorf("预警阈值必须小于危险阈值")
	}
	return nil
}

// getDefaultAdviceRules 获取默认建议规则
func getDefaultAdviceRules() []AdviceRule {
	return []AdviceRule{
		{
			Name:     "capacity_warning",
			Priority: 8,
			Category: "capacity",
			Condition: func(r *PredictionResult) bool {
				return r.WarningInDays > 0 && r.WarningInDays <= 30
			},
			Generate: func(r *PredictionResult) Advice {
				return Advice{
					Category:    "capacity",
					Priority:    8,
					Title:       "存储容量即将达到预警阈值",
					Description: fmt.Sprintf("卷 %s 预计在 %d 天后达到预警阈值（%.1f%%）", r.VolumeName, r.WarningInDays, 75.0),
					Action:      "考虑扩容或清理无用数据",
					Impact:      "避免存储空间不足影响业务",
					Savings:     fmt.Sprintf("建议预留 %.0f GB", r.PredictedUsage30d*0.2),
				}
			},
		},
		{
			Name:     "capacity_critical",
			Priority: 10,
			Category: "capacity",
			Condition: func(r *PredictionResult) bool {
				return r.CriticalInDays > 0 && r.CriticalInDays <= 14
			},
			Generate: func(r *PredictionResult) Advice {
				return Advice{
					Category:    "capacity",
					Priority:    10,
					Title:       "存储容量即将达到危险阈值",
					Description: fmt.Sprintf("卷 %s 预计在 %d 天后达到危险阈值（%.1f%%）", r.VolumeName, r.CriticalInDays, 90.0),
					Action:      "立即扩容或清理数据",
					Impact:      "紧急：避免系统停止运行",
					Savings:     fmt.Sprintf("至少需要 %.0f GB 额外空间", r.PredictedUsage30d-r.CurrentTotal*0.1),
				}
			},
		},
		{
			Name:     "fast_growth",
			Priority: 6,
			Category: "capacity",
			Condition: func(r *PredictionResult) bool {
				return r.GrowthRateDaily > 1.0 // 日增长超过1GB
			},
			Generate: func(r *PredictionResult) Advice {
				return Advice{
					Category:    "capacity",
					Priority:    6,
					Title:       "存储使用快速增长",
					Description: fmt.Sprintf("卷 %s 日均增长 %.2f GB，建议关注", r.VolumeName, r.GrowthRateDaily),
					Action:      "检查是否有异常数据写入",
					Impact:      "提前规划扩容需求",
					Savings:     fmt.Sprintf("月增长约 %.0f GB", r.GrowthRateMonthly),
				}
			},
		},
		{
			Name:     "cleanup_recommendation",
			Priority: 5,
			Category: "cost",
			Condition: func(r *PredictionResult) bool {
				return r.CurrentUsageRate > 60.0 && r.FullInDays < 90
			},
			Generate: func(r *PredictionResult) Advice {
				return Advice{
					Category:    "cost",
					Priority:    5,
					Title:       "建议清理无用数据",
					Description: fmt.Sprintf("卷 %s 使用率 %.1f%%，建议清理临时文件和旧快照", r.VolumeName, r.CurrentUsageRate),
					Action:      "运行存储分析工具，识别可清理数据",
					Impact:      "节省存储成本",
					Savings:     "预计可回收 10-30% 空间",
				}
			},
		},
		{
			Name:     "stable_usage",
			Priority: 3,
			Category: "performance",
			Condition: func(r *PredictionResult) bool {
				return r.Trend == "stable" && r.CurrentUsageRate < 50.0
			},
			Generate: func(r *PredictionResult) Advice {
				return Advice{
					Category:    "performance",
					Priority:    3,
					Title:       "存储使用稳定",
					Description: fmt.Sprintf("卷 %s 使用趋势稳定，当前使用率 %.1f%%", r.VolumeName, r.CurrentUsageRate),
					Action:      "继续保持良好的数据管理习惯",
					Impact:      "存储状态健康",
					Savings:     "暂无操作需要",
				}
			},
		},
	}
}

// startCollection 启动数据收集
func (m *Manager) startCollection() {
	ticker := time.NewTicker(m.getCollectionInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.collectData()
		case <-m.stopChan:
			return
		}
	}
}

// getCollectionInterval 安全获取收集间隔
func (m *Manager) getCollectionInterval() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.CollectionInterval
}

// collectData 收集数据（由外部调用 RecordUsage 触发）
func (m *Manager) collectData() {
	// 清理过期历史数据
	m.cleanupOldHistory()
}

// cleanupOldHistory 清理过期历史数据
func (m *Manager) cleanupOldHistory() {
	m.history.mu.Lock()
	defer m.history.mu.Unlock()

	// 安全读取配置
	m.mu.RLock()
	retentionDays := m.config.HistoryRetentionDays
	m.mu.RUnlock()

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	for volName, volHistory := range m.history.VolumeData {
		// 过滤使用率历史
		var filteredUsage []UsageRecord
		for _, record := range volHistory.UsageHistory {
			if record.Timestamp.After(cutoff) {
				filteredUsage = append(filteredUsage, record)
			}
		}
		volHistory.UsageHistory = filteredUsage

		// 过滤容量历史
		var filteredCapacity []CapacityRecord
		for _, record := range volHistory.CapacityHistory {
			if record.Timestamp.After(cutoff) {
				filteredCapacity = append(filteredCapacity, record)
			}
		}
		volHistory.CapacityHistory = filteredCapacity

		// 过滤IO历史
		var filteredIO []IORecord
		for _, record := range volHistory.IOHistory {
			if record.Timestamp.After(cutoff) {
				filteredIO = append(filteredIO, record)
			}
		}
		volHistory.IOHistory = filteredIO

		m.history.VolumeData[volName] = volHistory
	}
}

// RecordUsage 记录使用数据
func (m *Manager) RecordUsage(volumeName string, usedGB, totalGB float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history.mu.Lock()
	defer m.history.mu.Unlock()

	// 获取或创建卷历史
	volHistory, exists := m.history.VolumeData[volumeName]
	if !exists {
		volHistory = &VolumeHistory{
			VolumeName: volumeName,
		}
	}

	// 记录使用数据
	now := time.Now()
	usageRate := 0.0
	if totalGB > 0 {
		usageRate = (usedGB / totalGB) * 100
	}

	record := UsageRecord{
		Timestamp: now,
		UsedGB:    usedGB,
		TotalGB:   totalGB,
		UsageRate: usageRate,
	}

	volHistory.UsageHistory = append(volHistory.UsageHistory, record)
	volHistory.LastUpdated = now

	// 限制历史记录数量（保留最近1000条）
	if len(volHistory.UsageHistory) > 1000 {
		volHistory.UsageHistory = volHistory.UsageHistory[len(volHistory.UsageHistory)-1000:]
	}

	m.history.VolumeData[volumeName] = volHistory

	// 检测异常
	m.detectAnomaly(volumeName, usageRate)

	return nil
}

// RecordIO 记录IO数据
func (m *Manager) RecordIO(volumeName string, readMBps, writeMBps float64, iops uint64) error {
	m.history.mu.Lock()
	defer m.history.mu.Unlock()

	volHistory, exists := m.history.VolumeData[volumeName]
	if !exists {
		volHistory = &VolumeHistory{
			VolumeName: volumeName,
		}
	}

	record := IORecord{
		Timestamp: time.Now(),
		ReadMBps:  readMBps,
		WriteMBps: writeMBps,
		IOPS:      iops,
	}

	volHistory.IOHistory = append(volHistory.IOHistory, record)
	volHistory.LastUpdated = time.Now()

	// 限制历史记录数量
	if len(volHistory.IOHistory) > 1000 {
		volHistory.IOHistory = volHistory.IOHistory[len(volHistory.IOHistory)-1000:]
	}

	m.history.VolumeData[volumeName] = volHistory

	return nil
}

// detectAnomaly 检测异常
func (m *Manager) detectAnomaly(volumeName string, currentValue float64) {
	m.anomalyDetector.mu.Lock()
	defer m.anomalyDetector.mu.Unlock()

	// 如果基线未建立，跳过检测
	if m.anomalyDetector.baselineStdDev == 0 {
		return
	}

	// 计算偏差
	deviation := math.Abs(currentValue - m.anomalyDetector.baselineMean)
	stdDevMultiple := deviation / m.anomalyDetector.baselineStdDev

	// 判断是否异常
	threshold := m.anomalyDetector.threshold / m.anomalyDetector.sensitivity
	if stdDevMultiple > threshold {
		severity := "low"
		if stdDevMultiple > 3 {
			severity = "critical"
		} else if stdDevMultiple > 2.5 {
			severity = "high"
		} else if stdDevMultiple > 2 {
			severity = "medium"
		}

		anomalyType := "usage_spike"
		if currentValue < m.anomalyDetector.baselineMean {
			anomalyType = "usage_drop"
		}

		anomaly := Anomaly{
			Timestamp:   time.Now(),
			Type:        anomalyType,
			Severity:    severity,
			Value:       currentValue,
			Expected:    m.anomalyDetector.baselineMean,
			Deviation:   stdDevMultiple,
			Description: fmt.Sprintf("卷 %s 使用率 %.1f%% 异常偏离基线 %.1f%%（偏差 %.1f 倍标准差）", volumeName, currentValue, m.anomalyDetector.baselineMean, stdDevMultiple),
		}

		m.anomalyDetector.anomalies = append(m.anomalyDetector.anomalies, anomaly)

		// 只保留最近100条异常
		if len(m.anomalyDetector.anomalies) > 100 {
			m.anomalyDetector.anomalies = m.anomalyDetector.anomalies[len(m.anomalyDetector.anomalies)-100:]
		}
	}
}

// Predict 预测存储使用
func (m *Manager) Predict(volumeName string) (*PredictionResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.history.mu.RLock()
	volHistory, exists := m.history.VolumeData[volumeName]
	m.history.mu.RUnlock()

	if !exists || len(volHistory.UsageHistory) < 2 {
		return nil, fmt.Errorf("卷 %s 没有足够的历史数据", volumeName)
	}

	// 训练模型
	if err := m.trainModel(volHistory); err != nil {
		return nil, fmt.Errorf("训练模型失败: %w", err)
	}

	// 获取最新数据
	latestRecord := volHistory.UsageHistory[len(volHistory.UsageHistory)-1]

	// 计算趋势
	trend, growthRateDaily := m.analyzeTrend(volHistory)

	// 安全读取模型准确度
	m.model.mu.RLock()
	accuracy := m.model.accuracy
	m.model.mu.RUnlock()

	// 生成预测
	result := &PredictionResult{
		VolumeName:        volumeName,
		CurrentUsage:      latestRecord.UsedGB,
		CurrentTotal:      latestRecord.TotalGB,
		CurrentUsageRate:  latestRecord.UsageRate,
		Timestamp:         time.Now(),
		Trend:             trend,
		GrowthRateDaily:   growthRateDaily,
		GrowthRateWeekly:  growthRateDaily * 7,
		GrowthRateMonthly: growthRateDaily * 30,
		Confidence:        accuracy,
	}

	// 计算未来预测点
	m.predictFuture(result, latestRecord)

	// 获取异常
	result.Anomalies = m.getAnomalies(volumeName)

	// 生成建议
	if m.config.EnableAutoAdvice {
		result.Advices = m.generateAdvices(result)
	}

	return result, nil
}

// trainModel 训练预测模型
func (m *Manager) trainModel(history *VolumeHistory) error {
	m.model.mu.Lock()
	defer m.model.mu.Unlock()

	records := history.UsageHistory
	if len(records) < 2 {
		return fmt.Errorf("数据点不足")
	}

	// 简单线性回归
	n := float64(len(records))
	var sumX, sumY, sumXY, sumX2 float64

	for i, record := range records {
		x := float64(i)
		y := record.UsedGB
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// 计算斜率和截距
	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return fmt.Errorf("无法计算回归参数")
	}

	m.model.slope = (n*sumXY - sumX*sumY) / denominator
	m.model.intercept = (sumY - m.model.slope*sumX) / n

	// 计算模型准确度（R²）
	var ssTot, ssRes float64
	meanY := sumY / n
	for i, record := range records {
		predicted := m.model.intercept + m.model.slope*float64(i)
		ssTot += math.Pow(record.UsedGB-meanY, 2)
		ssRes += math.Pow(record.UsedGB-predicted, 2)
	}
	if ssTot > 0 {
		m.model.accuracy = 1 - ssRes/ssTot
	}

	m.model.lastTrained = time.Now()

	// 更新异常检测基线
	m.updateAnomalyBaseline(records)

	return nil
}

// updateAnomalyBaseline 更新异常检测基线
func (m *Manager) updateAnomalyBaseline(records []UsageRecord) {
	if len(records) < 10 {
		return
	}

	// 计算最近数据的均值和标准差
	recentRecords := records
	if len(records) > 100 {
		recentRecords = records[len(records)-100:]
	}

	var sum, sumSq float64
	for _, r := range recentRecords {
		sum += r.UsageRate
		sumSq += r.UsageRate * r.UsageRate
	}

	n := float64(len(recentRecords))
	mean := sum / n
	variance := sumSq/n - mean*mean
	stdDev := math.Sqrt(variance)

	m.anomalyDetector.mu.Lock()
	m.anomalyDetector.baselineMean = mean
	m.anomalyDetector.baselineStdDev = stdDev
	m.anomalyDetector.mu.Unlock()
}

// analyzeTrend 分析趋势
func (m *Manager) analyzeTrend(history *VolumeHistory) (string, float64) {
	records := history.UsageHistory
	if len(records) < 2 {
		return "unknown", 0
	}

	// 计算日均增长
	first := records[0]
	last := records[len(records)-1]

	days := last.Timestamp.Sub(first.Timestamp).Hours() / 24
	if days == 0 {
		return "unknown", 0
	}

	growthRate := (last.UsedGB - first.UsedGB) / days

	// 判断趋势
	if math.Abs(growthRate) < 0.01 { // 日增长小于 0.01 GB
		return "stable", growthRate
	} else if growthRate > 0 {
		return "increasing", growthRate
	} else {
		return "decreasing", growthRate
	}
}

// predictFuture 预测未来使用
func (m *Manager) predictFuture(result *PredictionResult, latest UsageRecord) {
	m.model.mu.RLock()
	slope := m.model.slope
	intercept := m.model.intercept
	m.model.mu.RUnlock()

	now := time.Now()
	totalGB := latest.TotalGB

	// 生成未来30天的预测点
	for day := 1; day <= 30; day++ {
		predictedGB := intercept + slope*float64(result.PredictedPoints()+day)
		if predictedGB < 0 {
			predictedGB = 0
		}

		predictedRate := 0.0
		if totalGB > 0 {
			predictedRate = (predictedGB / totalGB) * 100
		}

		result.PredictedUsage = append(result.PredictedUsage, PredictedPoint{
			Date:      now.AddDate(0, 0, day),
			UsageGB:   predictedGB,
			UsageRate: predictedRate,
		})
	}

	// 计算关键时间点
	// 何时达到预警阈值
	warningGB := totalGB * (m.config.WarningThreshold / 100)
	criticalGB := totalGB * (m.config.CriticalThreshold / 100)
	fullGB := totalGB

	if result.GrowthRateDaily > 0 {
		// 预警时间
		if latest.UsedGB < warningGB {
			result.WarningInDays = int((warningGB - latest.UsedGB) / result.GrowthRateDaily)
		}
		// 危险时间
		if latest.UsedGB < criticalGB {
			result.CriticalInDays = int((criticalGB - latest.UsedGB) / result.GrowthRateDaily)
		}
		// 满容量时间
		if latest.UsedGB < fullGB {
			result.FullInDays = int((fullGB - latest.UsedGB) / result.GrowthRateDaily)
		}
	}

	// 计算未来使用量
	if len(result.PredictedUsage) > 0 {
		// 30天
		if len(result.PredictedUsage) >= 30 {
			result.PredictedUsage30d = result.PredictedUsage[29].UsageGB
		}
		// 90天（外推）
		result.PredictedUsage90d = intercept + slope*float64(len(result.PredictedUsage)+60)
		// 365天（外推，仅供参考）
		result.PredictedUsage365d = intercept + slope*float64(len(result.PredictedUsage)+335)
	}
}

// getAnomalies 获取异常
func (m *Manager) getAnomalies(volumeName string) []Anomaly {
	m.anomalyDetector.mu.RLock()
	defer m.anomalyDetector.mu.RUnlock()

	// 返回最近的异常
	anomalies := make([]Anomaly, 0)
	for i := len(m.anomalyDetector.anomalies) - 1; i >= 0; i-- {
		if len(anomalies) >= 10 {
			break
		}
		anomalies = append(anomalies, m.anomalyDetector.anomalies[i])
	}

	return anomalies
}

// generateAdvices 生成优化建议
func (m *Manager) generateAdvices(result *PredictionResult) []Advice {
	m.advisor.mu.Lock()
	defer m.advisor.mu.Unlock()

	advices := make([]Advice, 0)
	now := time.Now()

	for _, rule := range m.advisor.rules {
		if rule.Condition(result) {
			advice := rule.Generate(result)
			advice.ID = fmt.Sprintf("%s-%d", rule.Name, now.Unix())
			advice.Timestamp = now
			advices = append(advices, advice)
		}
	}

	return advices
}

// GetHistory 获取历史数据
func (m *Manager) GetHistory(volumeName string, days int) ([]UsageRecord, error) {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()

	volHistory, exists := m.history.VolumeData[volumeName]
	if !exists {
		return nil, fmt.Errorf("卷 %s 不存在", volumeName)
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var records []UsageRecord
	for _, record := range volHistory.UsageHistory {
		if record.Timestamp.After(cutoff) {
			records = append(records, record)
		}
	}

	return records, nil
}

// ListVolumes 列出所有有历史数据的卷
func (m *Manager) ListVolumes() []string {
	m.history.mu.RLock()
	defer m.history.mu.RUnlock()

	volumes := make([]string, 0, len(m.history.VolumeData))
	for name := range m.history.VolumeData {
		volumes = append(volumes, name)
	}

	return volumes
}

// Stop 停止预测管理器
func (m *Manager) Stop() {
	close(m.stopChan)
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *Config) error {
	if err := validateConfig(cfg); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg

	return nil
}

// GetAllPredictions 获取所有卷的预测
func (m *Manager) GetAllPredictions() (map[string]*PredictionResult, error) {
	volumes := m.ListVolumes()
	results := make(map[string]*PredictionResult)

	for _, vol := range volumes {
		result, err := m.Predict(vol)
		if err != nil {
			continue
		}
		results[vol] = result
	}

	return results, nil
}

// PredictedPoints 返回预测数据点数量
func (r *PredictionResult) PredictedPoints() int {
	return len(r.PredictedUsage)
}

// HasWarning 是否有预警
func (r *PredictionResult) HasWarning() bool {
	return r.WarningInDays > 0 && r.WarningInDays <= 30
}

// IsCritical 是否处于危险状态
func (r *PredictionResult) IsCritical() bool {
	return r.CriticalInDays > 0 && r.CriticalInDays <= 14
}
