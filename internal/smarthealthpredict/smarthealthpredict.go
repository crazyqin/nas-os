// Package smarthealthpredict 提供智能存储健康预测功能
// 基于 S.M.A.R.T 数据分析、AI 预测模型、历史趋势分析，实现磁盘故障预测和健康评估。
// 参考群晖存储管理器和 TrueNAS S.M.A.R.T 监控设计。
package smarthealthpredict

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ========== 错误定义 ==========

var (
	// ErrDiskNotFound 磁盘不存在.
	ErrDiskNotFound = errors.New("磁盘不存在")
	// ErrSMARTNotSupported 磁盘不支持 S.M.A.R.T.
	ErrSMARTNotSupported = errors.New("磁盘不支持 S.M.A.R.T")
	// ErrPredictionJobNotFound 预测任务不存在.
	ErrPredictionJobNotFound = errors.New("预测任务不存在")
	// ErrJobAlreadyRunning 已有任务正在运行.
	ErrJobAlreadyRunning = errors.New("已有任务正在运行")
	// ErrInvalidModel 无效的预测模型.
	ErrInvalidModel = errors.New("无效的预测模型")
	// ErrInsufficientData 数据不足，无法预测.
	ErrInsufficientData = errors.New("数据不足，无法预测")
	// ErrPathRequired 必须指定路径.
	ErrPathRequired = errors.New("必须指定路径")
)

// ========== 核心类型 ==========

// HealthStatus 健康状态
type HealthStatus string

const (
	StatusExcellent HealthStatus = "excellent" // 优秀 90-100
	StatusGood      HealthStatus = "good"      // 良好 70-89
	StatusFair      HealthStatus = "fair"      // 一般 50-69
	StatusPoor      HealthStatus = "poor"      // 较差 30-49
	StatusCritical  HealthStatus = "critical"  // 危险 0-29
)

// DiskType 磁盘类型
type DiskType string

const (
	DiskTypeHDD  DiskType = "hdd"
	DiskTypeSSD  DiskType = "ssd"
	DiskTypeNVMe DiskType = "nvme"
)

// PredictionModel 预测模型
type PredictionModel string

const (
	ModelStatistical PredictionModel = "statistical" // 统计模型
	ModelMLBased     PredictionModel = "ml"          // 机器学习模型
	ModelHybrid      PredictionModel = "hybrid"      // 混合模型
)

// SMARTAttribute S.M.A.R.T 属性
type SMARTAttribute struct {
	ID        int    `json:"id"`        // 属性ID
	Name      string `json:"name"`      // 属性名称
	Value     int    `json:"value"`     // 当前值
	Worst     int    `json:"worst"`     // 最差值
	Threshold int    `json:"threshold"` // 阈值
	RawValue  int64  `json:"rawValue"`  // 原始值
	Failed    bool   `json:"failed"`    // 是否失败
	Critical  bool   `json:"critical"`  // 是否关键属性
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	Device      string   `json:"device"`      // 设备路径 /dev/sda
	Model       string   `json:"model"`       // 型号
	Serial      string   `json:"serial"`      // 序列号
	Type        DiskType `json:"type"`        // 磁盘类型
	Capacity    int64    `json:"capacity"`    // 容量 (bytes)
	Temperature int      `json:"temperature"` // 温度 (°C)
	PowerOn     int64    `json:"powerOn"`     // 通电时间 (小时)
	Health      int      `json:"health"`      // 健康评分 0-100
	SMARTPassed bool     `json:"smartPassed"` // S.M.A.R.T 自检通过
}

// HealthReport 健康报告
type HealthReport struct {
	Disk        DiskInfo         `json:"disk"`
	Score       int              `json:"score"`       // 综合健康评分 0-100
	Status      HealthStatus     `json:"status"`      // 健康状态
	Attributes  []SMARTAttribute `json:"attributes"`  // S.M.A.R.T 属性
	Predictions []Prediction     `json:"predictions"` // 预测结果
	Alerts      []Alert          `json:"alerts"`      // 告警
	Suggestions []Suggestion     `json:"suggestions"` // 建议
	Trend       *TrendAnalysis   `json:"trend"`       // 趋势分析
	Timestamp   time.Time        `json:"timestamp"`
}

// Prediction 故障预测
type Prediction struct {
	Type        string  `json:"type"`        // 预测类型: failure, degradation, temperature
	Probability float64 `json:"probability"` // 概率 0-1
	TimeToEvent string  `json:"timeToEvent"` // 预计时间: "30 days", "6 months"
	Confidence  float64 `json:"confidence"`  // 置信度 0-1
	Model       string  `json:"model"`       // 使用的模型
	Description string  `json:"description"` // 描述
}

// Alert 告警
type Alert struct {
	Level     string    `json:"level"`     // info, warning, critical
	Type      string    `json:"type"`      // 告警类型
	Message   string    `json:"message"`   // 告警消息
	Value     float64   `json:"value"`     // 当前值
	Threshold float64   `json:"threshold"` // 阈值
	Timestamp time.Time `json:"timestamp"`
}

// Suggestion 建议
type Suggestion struct {
	Priority string `json:"priority"` // high, medium, low
	Action   string `json:"action"`   // 建议操作
	Reason   string `json:"reason"`   // 原因
	Impact   string `json:"impact"`   // 影响
}

// TrendAnalysis 趋势分析
type TrendAnalysis struct {
	HealthTrend     string  `json:"healthTrend"`     // improving, stable, degrading
	TempTrend       string  `json:"tempTrend"`       // 温度趋势
	ProjectedHealth int     `json:"projectedHealth"` // 30天后预测健康分
	DeclineRate     float64 `json:"declineRate"`     // 下降速率 (分/月)
}

// ========== 管理器 ==========

// Manager 智能存储健康预测管理器
type Manager struct {
	mu              sync.RWMutex
	disks           map[string]*DiskHealthTracker
	predictions     map[string]*PredictionJob
	history         map[string][]HealthSnapshot
	alertRules      []AlertRule
	model           PredictionModel
	dataDir         string
	retentionDays   int
	predictInterval time.Duration
	logger          *zap.Logger
}

// DiskHealthTracker 磁盘健康追踪器
type DiskHealthTracker struct {
	Device           string
	Info             DiskInfo
	LastCheck        time.Time
	SMARTData        []SMARTAttribute
	HealthScore      int
	Status           HealthStatus
	ConsecutiveFails int
}

// PredictionJob 预测任务
type PredictionJob struct {
	ID        string
	Device    string
	Status    string // pending, running, completed, failed
	StartedAt time.Time
	Result    *Prediction
	Error     error
}

// HealthSnapshot 健康快照
type HealthSnapshot struct {
	Timestamp   time.Time
	Score       int
	Temperature int
	PowerOn     int64
	Attributes  map[string]int64
}

// AlertRule 告警规则
type AlertRule struct {
	Name      string
	Type      string
	Threshold float64
	Level     string
	Condition string // above, below, change
}

// NewManager 创建管理器
func NewManager(dataDir string, opts ...Option) (*Manager, error) {
	if dataDir == "" {
		return nil, ErrPathRequired
	}

	m := &Manager{
		disks:           make(map[string]*DiskHealthTracker),
		predictions:     make(map[string]*PredictionJob),
		history:         make(map[string][]HealthSnapshot),
		model:           ModelHybrid,
		dataDir:         dataDir,
		retentionDays:   90,
		predictInterval: 24 * time.Hour,
		logger:          zap.NewNop(),
	}

	// 应用选项
	for _, opt := range opts {
		opt(m)
	}

	// 创建数据目录
	if err := os.MkdirAll(filepath.Join(dataDir, "smart"), 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	// 初始化默认告警规则
	m.initDefaultAlertRules()

	m.logger.Info("健康预测管理器已初始化",
		zap.String("dataDir", dataDir),
		zap.String("model", string(m.model)),
	)

	return m, nil
}

// Option 配置选项
type Option func(*Manager)

// WithModel 设置预测模型
func WithModel(model PredictionModel) Option {
	return func(m *Manager) {
		m.model = model
	}
}

// WithRetentionDays 设置数据保留天数
func WithRetentionDays(days int) Option {
	return func(m *Manager) {
		m.retentionDays = days
	}
}

// WithPredictInterval 设置预测间隔
func WithPredictInterval(interval time.Duration) Option {
	return func(m *Manager) {
		m.predictInterval = interval
	}
}

// WithLogger 设置日志器
func WithLogger(logger *zap.Logger) Option {
	return func(m *Manager) {
		if logger != nil {
			m.logger = logger
		}
	}
}

// initDefaultAlertRules 初始化默认告警规则
func (m *Manager) initDefaultAlertRules() {
	m.alertRules = []AlertRule{
		{Name: "温度过高", Type: "temperature", Threshold: 60, Level: "warning", Condition: "above"},
		{Name: "温度危险", Type: "temperature", Threshold: 70, Level: "critical", Condition: "above"},
		{Name: "健康分低", Type: "health", Threshold: 50, Level: "warning", Condition: "below"},
		{Name: "健康分危险", Type: "health", Threshold: 30, Level: "critical", Condition: "below"},
		{Name: "通电时间长", Type: "poweron", Threshold: 43800, Level: "warning", Condition: "above"}, // 5年
		{Name: "重分配扇区", Type: "reallocated", Threshold: 10, Level: "warning", Condition: "above"},
		{Name: "待处理扇区", Type: "pending", Threshold: 5, Level: "warning", Condition: "above"},
	}
}

// ScanDisk 扫描磁盘
func (m *Manager) ScanDisk(ctx context.Context, device string) (*HealthReport, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.logger.Info("开始扫描磁盘", zap.String("device", device))

	// 读取 S.M.A.R.T 数据
	attrs, err := m.readSMARTData(device)
	if err != nil {
		m.logger.Error("读取 S.M.A.R.T 数据失败", zap.String("device", device), zap.Error(err))
		return nil, fmt.Errorf("读取 S.M.A.R.T 数据失败: %w", err)
	}

	// 获取磁盘信息
	info := m.getDiskInfo(device)

	// 计算健康评分
	score := m.calculateHealthScore(attrs, info)

	// 确定健康状态
	status := m.getHealthStatus(score)

	// 更新追踪器
	tracker := &DiskHealthTracker{
		Device:      device,
		Info:        info,
		LastCheck:   time.Now(),
		SMARTData:   attrs,
		HealthScore: score,
		Status:      status,
	}
	m.disks[device] = tracker

	// 保存历史快照
	m.saveSnapshot(device, score, info.Temperature, info.PowerOn, attrs)

	// 检查告警
	alerts := m.checkAlerts(info, attrs, score)

	// 生成建议
	suggestions := m.generateSuggestions(info, attrs, score, alerts)

	// 趋势分析
	trend := m.analyzeTrend(device)

	// 故障预测
	predictions := m.predictFailures(device, info, attrs, trend)

	m.logger.Info("磁盘扫描完成",
		zap.String("device", device),
		zap.Int("score", score),
		zap.String("status", string(status)),
		zap.Int("alerts", len(alerts)),
	)

	return &HealthReport{
		Disk:        info,
		Score:       score,
		Status:      status,
		Attributes:  attrs,
		Predictions: predictions,
		Alerts:      alerts,
		Suggestions: suggestions,
		Trend:       trend,
		Timestamp:   time.Now(),
	}, nil
}

// calculateHealthScore 计算健康评分
func (m *Manager) calculateHealthScore(attrs []SMARTAttribute, info DiskInfo) int {
	if len(attrs) == 0 {
		return 50 // 无数据时返回中等分数
	}

	score := 100.0

	// 权重配置
	weights := map[string]float64{
		"Reallocated_Sector_Ct":  20.0,
		"Current_Pending_Sector": 15.0,
		"Offline_Uncorrectable":  15.0,
		"UDMA_CRC_Error_Count":   10.0,
		"Spin_Retry_Count":       10.0,
		"Temperature_Celsius":    10.0,
		"Power_On_Hours":         10.0,
		"Wear_Leveling_Count":    10.0, // SSD
	}

	for _, attr := range attrs {
		weight, ok := weights[attr.Name]
		if !ok {
			continue
		}

		switch attr.Name {
		case "Reallocated_Sector_Ct", "Current_Pending_Sector", "Offline_Uncorrectable":
			// 扇区问题：值越大越差
			if attr.RawValue > 0 {
				penalty := math.Min(float64(attr.RawValue)/100.0, 1.0) * weight
				score -= penalty
			}
		case "Temperature_Celsius":
			// 温度：超过50度开始扣分
			if attr.RawValue > 50 {
				penalty := (float64(attr.RawValue) - 50) / 20.0 * weight
				score -= math.Min(penalty, weight)
			}
		case "Power_On_Hours":
			// 通电时间：超过3年(26280小时)开始扣分
			if attr.RawValue > 26280 {
				penalty := (float64(attr.RawValue) - 26280) / 26280.0 * weight
				score -= math.Min(penalty, weight)
			}
		case "Wear_Leveling_Count":
			// SSD磨损：值越小越差
			if attr.Value < 50 {
				penalty := (100 - float64(attr.Value)) / 100.0 * weight
				score -= penalty
			}
		}

		// 关键属性失败直接大幅扣分
		if attr.Failed && attr.Critical {
			score -= 30
		}
	}

	// 温度额外检查
	if info.Temperature > 70 {
		score -= 20
	} else if info.Temperature > 60 {
		score -= 10
	}

	// S.M.A.R.T 自检
	if !info.SMARTPassed {
		score -= 25
	}

	// 确保在 0-100 范围
	return int(math.Max(0, math.Min(100, score)))
}

// getHealthStatus 获取健康状态
func (m *Manager) getHealthStatus(score int) HealthStatus {
	switch {
	case score >= 90:
		return StatusExcellent
	case score >= 70:
		return StatusGood
	case score >= 50:
		return StatusFair
	case score >= 30:
		return StatusPoor
	default:
		return StatusCritical
	}
}

// checkAlerts 检查告警
func (m *Manager) checkAlerts(info DiskInfo, attrs []SMARTAttribute, score int) []Alert {
	var alerts []Alert
	now := time.Now()

	for _, rule := range m.alertRules {
		switch rule.Type {
		case "temperature":
			if rule.Condition == "above" && float64(info.Temperature) > rule.Threshold {
				alerts = append(alerts, Alert{
					Level:     rule.Level,
					Type:      rule.Type,
					Message:   fmt.Sprintf("%s: 当前温度 %d°C", rule.Name, info.Temperature),
					Value:     float64(info.Temperature),
					Threshold: rule.Threshold,
					Timestamp: now,
				})
			}
		case "health":
			if rule.Condition == "below" && float64(score) < rule.Threshold {
				alerts = append(alerts, Alert{
					Level:     rule.Level,
					Type:      rule.Type,
					Message:   fmt.Sprintf("%s: 当前健康分 %d", rule.Name, score),
					Value:     float64(score),
					Threshold: rule.Threshold,
					Timestamp: now,
				})
			}
		case "poweron":
			if rule.Condition == "above" && float64(info.PowerOn) > rule.Threshold {
				alerts = append(alerts, Alert{
					Level:     rule.Level,
					Type:      rule.Type,
					Message:   fmt.Sprintf("%s: 已通电 %d 小时", rule.Name, info.PowerOn),
					Value:     float64(info.PowerOn),
					Threshold: rule.Threshold,
					Timestamp: now,
				})
			}
		case "reallocated", "pending":
			for _, attr := range attrs {
				if (rule.Type == "reallocated" && attr.Name == "Reallocated_Sector_Ct") ||
					(rule.Type == "pending" && attr.Name == "Current_Pending_Sector") {
					if rule.Condition == "above" && float64(attr.RawValue) > rule.Threshold {
						alerts = append(alerts, Alert{
							Level:     rule.Level,
							Type:      rule.Type,
							Message:   fmt.Sprintf("%s: %s 值为 %d", rule.Name, attr.Name, attr.RawValue),
							Value:     float64(attr.RawValue),
							Threshold: rule.Threshold,
							Timestamp: now,
						})
					}
				}
			}
		}
	}

	return alerts
}

// generateSuggestions 生成建议
func (m *Manager) generateSuggestions(info DiskInfo, attrs []SMARTAttribute, score int, alerts []Alert) []Suggestion {
	var suggestions []Suggestion

	// 基于健康分的建议
	if score < 30 {
		suggestions = append(suggestions, Suggestion{
			Priority: "high",
			Action:   "立即备份数据并更换磁盘",
			Reason:   "磁盘健康状况极差，存在高故障风险",
			Impact:   "数据丢失风险极高",
		})
	} else if score < 50 {
		suggestions = append(suggestions, Suggestion{
			Priority: "high",
			Action:   "尽快备份重要数据",
			Reason:   "磁盘健康状况不佳",
			Impact:   "可能面临数据丢失",
		})
	}

	// 基于温度的建议
	if info.Temperature > 60 {
		suggestions = append(suggestions, Suggestion{
			Priority: "medium",
			Action:   "检查散热系统，改善通风",
			Reason:   fmt.Sprintf("磁盘温度过高 (%d°C)", info.Temperature),
			Impact:   "高温会加速磁盘老化",
		})
	}

	// 基于 S.M.A.R.T 属性的建议
	for _, attr := range attrs {
		switch attr.Name {
		case "Reallocated_Sector_Ct":
			if attr.RawValue > 0 {
				suggestions = append(suggestions, Suggestion{
					Priority: "high",
					Action:   "监控重分配扇区增长趋势",
					Reason:   fmt.Sprintf("检测到 %d 个重分配扇区", attr.RawValue),
					Impact:   "可能是磁盘物理损坏的前兆",
				})
			}
		case "Current_Pending_Sector":
			if attr.RawValue > 0 {
				suggestions = append(suggestions, Suggestion{
					Priority: "medium",
					Action:   "运行磁盘自检修复待处理扇区",
					Reason:   fmt.Sprintf("检测到 %d 个待处理扇区", attr.RawValue),
					Impact:   "待处理扇区可能变成坏道",
				})
			}
		case "Wear_Leveling_Count":
			if attr.Value < 20 {
				suggestions = append(suggestions, Suggestion{
					Priority: "high",
					Action:   "计划更换 SSD",
					Reason:   fmt.Sprintf("SSD 磨损程度已达 %d%%", 100-attr.Value),
					Impact:   "SSD 写入寿命即将耗尽",
				})
			}
		}
	}

	// 通电时间建议
	if info.PowerOn > 43800 { // 5年
		suggestions = append(suggestions, Suggestion{
			Priority: "medium",
			Action:   "考虑预防性更换磁盘",
			Reason:   fmt.Sprintf("磁盘已通电 %d 小时 (约 %.1f 年)", info.PowerOn, float64(info.PowerOn)/8760),
			Impact:   "磁盘故障率随使用年限增加",
		})
	}

	// 按优先级排序
	sort.Slice(suggestions, func(i, j int) bool {
		priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		return priorityOrder[suggestions[i].Priority] < priorityOrder[suggestions[j].Priority]
	})

	return suggestions
}

// analyzeTrend 分析趋势
func (m *Manager) analyzeTrend(device string) *TrendAnalysis {
	snapshots := m.history[device]

	if len(snapshots) < 2 {
		return &TrendAnalysis{
			HealthTrend: "stable",
			TempTrend:   "stable",
		}
	}

	// 取最近的快照分析
	recent := snapshots[len(snapshots)-1]
	previous := snapshots[len(snapshots)-2]

	// 健康趋势
	healthDiff := recent.Score - previous.Score
	healthTrend := "stable"
	if healthDiff > 2 {
		healthTrend = "improving"
	} else if healthDiff <= -2 {
		healthTrend = "degrading"
	}

	// 温度趋势
	tempDiff := recent.Temperature - previous.Temperature
	tempTrend := "stable"
	if tempDiff > 3 {
		tempTrend = "rising"
	} else if tempDiff < -3 {
		tempTrend = "falling"
	}

	// 计算下降速率（分/月）
	declineRate := 0.0
	if len(snapshots) >= 7 {
		weekAgo := snapshots[len(snapshots)-7]
		declineRate = float64(weekAgo.Score-recent.Score) * 4.3 // 转换为月速率
	}

	// 预测30天后健康分
	projectedHealth := recent.Score - int(declineRate)
	projectedHealth = int(math.Max(0, math.Min(100, float64(projectedHealth))))

	return &TrendAnalysis{
		HealthTrend:     healthTrend,
		TempTrend:       tempTrend,
		ProjectedHealth: projectedHealth,
		DeclineRate:     declineRate,
	}
}

// predictFailures 预测故障
func (m *Manager) predictFailures(device string, info DiskInfo, attrs []SMARTAttribute, trend *TrendAnalysis) []Prediction {
	var predictions []Prediction

	// 基于趋势的故障预测
	if trend.HealthTrend == "degrading" && trend.DeclineRate > 5 {
		monthsToFailure := float64(info.Health) / trend.DeclineRate
		timeToEvent := fmt.Sprintf("%.0f 天", monthsToFailure*30)

		predictions = append(predictions, Prediction{
			Type:        "failure",
			Probability: math.Min(0.9, trend.DeclineRate/20),
			TimeToEvent: timeToEvent,
			Confidence:  0.75,
			Model:       string(m.model),
			Description: "基于健康分下降趋势预测",
		})
	}

	// 基于 S.M.A.R.T 属性的预测
	for _, attr := range attrs {
		switch attr.Name {
		case "Reallocated_Sector_Ct":
			if attr.RawValue > 100 {
				predictions = append(predictions, Prediction{
					Type:        "failure",
					Probability: math.Min(0.85, float64(attr.RawValue)/500),
					TimeToEvent: "30-90 天",
					Confidence:  0.8,
					Model:       "statistical",
					Description: fmt.Sprintf("重分配扇区过多 (%d)，故障概率高", attr.RawValue),
				})
			}
		case "Temperature_Celsius":
			if attr.RawValue > 65 {
				predictions = append(predictions, Prediction{
					Type:        "degradation",
					Probability: 0.6,
					TimeToEvent: "6-12 个月",
					Confidence:  0.7,
					Model:       "statistical",
					Description: fmt.Sprintf("持续高温 (%d°C) 加速老化", attr.RawValue),
				})
			}
		}
	}

	// SSD 磨损预测
	if info.Type == DiskTypeSSD || info.Type == DiskTypeNVMe {
		for _, attr := range attrs {
			if attr.Name == "Wear_Leveling_Count" && attr.Value < 30 {
				predictions = append(predictions, Prediction{
					Type:        "failure",
					Probability: float64(100-attr.Value) / 100,
					TimeToEvent: "3-6 个月",
					Confidence:  0.85,
					Model:       "statistical",
					Description: fmt.Sprintf("SSD 磨损严重，剩余寿命 %d%%", attr.Value),
				})
			}
		}
	}

	// 按概率排序
	sort.Slice(predictions, func(i, j int) bool {
		return predictions[i].Probability > predictions[j].Probability
	})

	return predictions
}

// saveSnapshot 保存快照
func (m *Manager) saveSnapshot(device string, score, temp int, powerOn int64, attrs []SMARTAttribute) {
	snapshot := HealthSnapshot{
		Timestamp:   time.Now(),
		Score:       score,
		Temperature: temp,
		PowerOn:     powerOn,
		Attributes:  make(map[string]int64),
	}

	for _, attr := range attrs {
		snapshot.Attributes[attr.Name] = attr.RawValue
	}

	m.history[device] = append(m.history[device], snapshot)

	// 清理过期数据
	cutoff := time.Now().AddDate(0, 0, -m.retentionDays)
	var filtered []HealthSnapshot
	for _, s := range m.history[device] {
		if s.Timestamp.After(cutoff) {
			filtered = append(filtered, s)
		}
	}
	m.history[device] = filtered
}

// GetDiskList 获取所有磁盘列表
func (m *Manager) GetDiskList() []DiskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var disks []DiskInfo
	for _, tracker := range m.disks {
		disks = append(disks, tracker.Info)
	}

	sort.Slice(disks, func(i, j int) bool {
		return disks[i].Device < disks[j].Device
	})

	return disks
}

// GetDiskHistory 获取磁盘历史数据
func (m *Manager) GetDiskHistory(device string, days int) []HealthSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshots := m.history[device]
	if days <= 0 {
		return snapshots
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var result []HealthSnapshot
	for _, s := range snapshots {
		if s.Timestamp.After(cutoff) {
			result = append(result, s)
		}
	}

	return result
}

// GetAlerts 获取所有告警
func (m *Manager) GetAlerts() []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var allAlerts []Alert
	for device := range m.disks {
		tracker := m.disks[device]
		alerts := m.checkAlerts(tracker.Info, tracker.SMARTData, tracker.HealthScore)
		allAlerts = append(allAlerts, alerts...)
	}

	return allAlerts
}

// GetSystemStatus 获取系统整体状态
func (m *Manager) GetSystemStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statusCount := map[string]int{
		"excellent": 0,
		"good":      0,
		"fair":      0,
		"poor":      0,
		"critical":  0,
	}

	var totalScore int
	for _, tracker := range m.disks {
		statusCount[string(tracker.Status)]++
		totalScore += tracker.HealthScore
	}

	avgScore := 0
	if len(m.disks) > 0 {
		avgScore = totalScore / len(m.disks)
	}

	return map[string]interface{}{
		"totalDisks":   len(m.disks),
		"averageScore": avgScore,
		"statusCount":  statusCount,
		"timestamp":    time.Now(),
	}
}

// ========== 辅助方法（需要实际实现） ==========

// readSMARTData 读取 S.M.A.R.T 数据
func (m *Manager) readSMARTData(device string) ([]SMARTAttribute, error) {
	// 实际实现应调用 smartctl 或读取 /sys/block/*/smart
	// 这里返回模拟数据用于测试
	return []SMARTAttribute{
		{ID: 1, Name: "Raw_Read_Error_Rate", Value: 100, Worst: 100, Threshold: 50, RawValue: 0, Critical: false},
		{ID: 5, Name: "Reallocated_Sector_Ct", Value: 100, Worst: 100, Threshold: 10, RawValue: 0, Critical: true},
		{ID: 9, Name: "Power_On_Hours", Value: 90, Worst: 90, Threshold: 0, RawValue: 8760, Critical: false},
		{ID: 12, Name: "Power_Cycle_Count", Value: 99, Worst: 99, Threshold: 0, RawValue: 100, Critical: false},
		{ID: 194, Name: "Temperature_Celsius", Value: 65, Worst: 45, Threshold: 0, RawValue: 42, Critical: false},
		{ID: 197, Name: "Current_Pending_Sector", Value: 100, Worst: 100, Threshold: 0, RawValue: 0, Critical: true},
		{ID: 198, Name: "Offline_Uncorrectable", Value: 100, Worst: 100, Threshold: 0, RawValue: 0, Critical: true},
	}, nil
}

// getDiskInfo 获取磁盘信息
func (m *Manager) getDiskInfo(device string) DiskInfo {
	// 实际实现应读取磁盘信息
	return DiskInfo{
		Device:      device,
		Model:       "Example Disk",
		Serial:      "ABC123",
		Type:        DiskTypeHDD,
		Capacity:    1024 * 1024 * 1024 * 1024, // 1TB
		Temperature: 42,
		PowerOn:     8760,
		Health:      85,
		SMARTPassed: true,
	}
}
