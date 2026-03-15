// Package optimizer 提供资源配额优化功能
package optimizer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 配额优化类型定义 ==========

// OptimizationType 优化类型
type OptimizationType string

const (
	OptimizationAutoAdjust OptimizationType = "auto_adjust" // 自动调整建议
	OptimizationPrediction OptimizationType = "prediction"  // 配额使用预测
	OptimizationViolation  OptimizationType = "violation"   // 配额违规检测
	OptimizationReportType OptimizationType = "report"      // 配额优化报告
)

// OptimizationSuggestion 优化建议
type OptimizationSuggestion struct {
	ID               string           `json:"id"`
	Type             OptimizationType `json:"type"`
	QuotaID          string           `json:"quota_id"`
	TargetID         string           `json:"target_id"`
	TargetName       string           `json:"target_name"`
	VolumeName       string           `json:"volume_name"`
	CurrentLimit     uint64           `json:"current_limit"`
	SuggestedLimit   uint64           `json:"suggested_limit"`
	AdjustmentReason string           `json:"adjustment_reason"`
	Confidence       float64          `json:"confidence"` // 置信度 0-1
	Priority         string           `json:"priority"`   // high, medium, low
	Impact           string           `json:"impact"`     // 影响描述
	CreatedAt        time.Time        `json:"created_at"`
	Status           string           `json:"status"` // pending, applied, dismissed
	AppliedAt        *time.Time       `json:"applied_at,omitempty"`
}

// ========== 自动配额调整建议 ==========

// AutoAdjustConfig 自动调整配置
type AutoAdjustConfig struct {
	Enabled              bool          `json:"enabled"`
	CheckInterval        time.Duration `json:"check_interval"`
	MinUtilization       float64       `json:"min_utilization"`        // 最低利用率阈值
	MaxUtilization       float64       `json:"max_utilization"`        // 最高利用率阈值
	AdjustmentThreshold  float64       `json:"adjustment_threshold"`   // 触发调整的阈值
	MaxAdjustmentPercent float64       `json:"max_adjustment_percent"` // 单次最大调整百分比
	MinQuotaGB           float64       `json:"min_quota_gb"`           // 最小配额限制
	GrowthBufferPercent  float64       `json:"growth_buffer_percent"`  // 增长缓冲百分比
	ShrinkCooldownDays   int           `json:"shrink_cooldown_days"`   // 缩减冷却期（天）
	ExpandCooldownDays   int           `json:"expand_cooldown_days"`   // 扩展冷却期（天）
}

// DefaultAutoAdjustConfig 默认自动调整配置
func DefaultAutoAdjustConfig() AutoAdjustConfig {
	return AutoAdjustConfig{
		Enabled:              true,
		CheckInterval:        24 * time.Hour,
		MinUtilization:       0.3,  // 30%
		MaxUtilization:       0.85, // 85%
		AdjustmentThreshold:  0.1,  // 10%
		MaxAdjustmentPercent: 0.5,  // 50%
		MinQuotaGB:           1.0,  // 最小1GB
		GrowthBufferPercent:  0.2,  // 20%增长缓冲
		ShrinkCooldownDays:   30,   // 30天冷却期
		ExpandCooldownDays:   7,    // 7天冷却期
	}
}

// AutoAdjustResult 自动调整结果
type AutoAdjustResult struct {
	QuotaID           string    `json:"quota_id"`
	TargetName        string    `json:"target_name"`
	OriginalLimit     uint64    `json:"original_limit"`
	NewLimit          uint64    `json:"new_limit"`
	Reason            string    `json:"reason"`
	AdjustedAt        time.Time `json:"adjusted_at"`
	UtilizationBefore float64   `json:"utilization_before"`
	UtilizationAfter  float64   `json:"utilization_after_estimated"`
}

// ========== 配额使用预测 ==========

// UsagePrediction 使用预测
type UsagePrediction struct {
	QuotaID              string         `json:"quota_id"`
	TargetID             string         `json:"target_id"`
	TargetName           string         `json:"target_name"`
	VolumeName           string         `json:"volume_name"`
	CurrentUsage         uint64         `json:"current_usage"`
	CurrentLimit         uint64         `json:"current_limit"`
	PredictedUsage       uint64         `json:"predicted_usage"`         // 预测使用量
	PredictedGrowth      uint64         `json:"predicted_growth"`        // 预测增长量
	PredictedDaysToLimit int            `json:"predicted_days_to_limit"` // 预计达到限制天数
	PredictedDaysToFull  int            `json:"predicted_days_to_full"`  // 预计填满天数
	GrowthRate           float64        `json:"growth_rate"`             // 字节/天
	GrowthTrend          string         `json:"growth_trend"`            // increasing, decreasing, stable
	Confidence           float64        `json:"confidence"`              // 预测置信度
	PredictionPeriod     time.Duration  `json:"prediction_period"`       // 预测周期
	HistoryPoints        []HistoryPoint `json:"history_points"`          // 历史数据点
	Recommendation       string         `json:"recommendation"`          // 建议
	GeneratedAt          time.Time      `json:"generated_at"`
}

// HistoryPoint 历史数据点
type HistoryPoint struct {
	Timestamp    time.Time `json:"timestamp"`
	UsedBytes    uint64    `json:"used_bytes"`
	UsagePercent float64   `json:"usage_percent"`
}

// PredictionMethod 预测方法
type PredictionMethod string

const (
	PredictionLinear      PredictionMethod = "linear"      // 线性回归
	PredictionExponential PredictionMethod = "exponential" // 指数平滑
	PredictionMovingAvg   PredictionMethod = "moving_avg"  // 移动平均
	PredictionARIMA       PredictionMethod = "arima"       // ARIMA模型
)

// PredictionConfig 预测配置
type PredictionConfig struct {
	Method           PredictionMethod `json:"method"`
	HistoryDays      int              `json:"history_days"`       // 使用多少历史数据
	PredictionDays   int              `json:"prediction_days"`    // 预测未来多少天
	ConfidenceLevel  float64          `json:"confidence_level"`   // 置信水平
	MinHistoryPoints int              `json:"min_history_points"` // 最少历史数据点
	OutlierThreshold float64          `json:"outlier_threshold"`  // 异常值阈值
}

// DefaultPredictionConfig 默认预测配置
func DefaultPredictionConfig() PredictionConfig {
	return PredictionConfig{
		Method:           PredictionLinear,
		HistoryDays:      30,
		PredictionDays:   7,
		ConfidenceLevel:  0.95,
		MinHistoryPoints: 7,
		OutlierThreshold: 3.0, // 3倍标准差
	}
}

// ========== 配额违规检测 ==========

// ViolationType 违规类型
type ViolationType string

const (
	ViolationHardLimit ViolationType = "hard_limit" // 硬限制违规
	ViolationSoftLimit ViolationType = "soft_limit" // 软限制违规
	ViolationProjected ViolationType = "projected"  // 预测违规
	ViolationAnomaly   ViolationType = "anomaly"    // 异常使用
	ViolationPolicy    ViolationType = "policy"     // 策略违规
)

// ViolationRecord 违规记录
type ViolationRecord struct {
	ID           string           `json:"id"`
	QuotaID      string           `json:"quota_id"`
	TargetID     string           `json:"target_id"`
	TargetName   string           `json:"target_name"`
	VolumeName   string           `json:"volume_name"`
	Type         ViolationType    `json:"type"`
	Severity     string           `json:"severity"` // info, warning, critical
	UsedBytes    uint64           `json:"used_bytes"`
	LimitBytes   uint64           `json:"limit_bytes"`
	OverageBytes uint64           `json:"overage_bytes"`
	UsagePercent float64          `json:"usage_percent"`
	Message      string           `json:"message"`
	DetectedAt   time.Time        `json:"detected_at"`
	ResolvedAt   *time.Time       `json:"resolved_at,omitempty"`
	ResolvedBy   string           `json:"resolved_by,omitempty"`
	Status       string           `json:"status"`  // active, resolved, ignored
	Actions      []string         `json:"actions"` // 建议操作
	History      []ViolationEvent `json:"history,omitempty"`
}

// ViolationEvent 违规事件
type ViolationEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	By        string    `json:"by,omitempty"`
	Notes     string    `json:"notes,omitempty"`
}

// ViolationConfig 违规检测配置
type ViolationConfig struct {
	Enabled              bool          `json:"enabled"`
	CheckInterval        time.Duration `json:"check_interval"`
	AlertOnHardViolation bool          `json:"alert_on_hard_violation"`
	AlertOnSoftViolation bool          `json:"alert_on_soft_violation"`
	GracePeriodMinutes   int           `json:"grace_period_minutes"` // 宽限期
	AutoEnforce          bool          `json:"auto_enforce"`         // 自动强制执行
	EnforcementAction    string        `json:"enforcement_action"`   // enforce动作
	NotifyEmails         []string      `json:"notify_emails"`
	NotifyWebhooks       []string      `json:"notify_webhooks"`
}

// DefaultViolationConfig 默认违规检测配置
func DefaultViolationConfig() ViolationConfig {
	return ViolationConfig{
		Enabled:              true,
		CheckInterval:        5 * time.Minute,
		AlertOnHardViolation: true,
		AlertOnSoftViolation: true,
		GracePeriodMinutes:   15,
		AutoEnforce:          false,
		NotifyEmails:         []string{},
		NotifyWebhooks:       []string{},
	}
}

// ========== 配额优化报告 ==========

// OptimizationReport 优化报告
type OptimizationReport struct {
	ID              string                   `json:"id"`
	GeneratedAt     time.Time                `json:"generated_at"`
	PeriodStart     time.Time                `json:"period_start"`
	PeriodEnd       time.Time                `json:"period_end"`
	Summary         OptimizationSummary      `json:"summary"`
	Suggestions     []OptimizationSuggestion `json:"suggestions"`
	Predictions     []UsagePrediction        `json:"predictions"`
	Violations      []ViolationRecord        `json:"violations"`
	CostImpact      CostImpactAnalysis       `json:"cost_impact"`
	Recommendations []string                 `json:"recommendations"`
}

// OptimizationSummary 优化摘要
type OptimizationSummary struct {
	TotalQuotas          int     `json:"total_quotas"`
	AnalyzedQuotas       int     `json:"analyzed_quotas"`
	SuggestionsCount     int     `json:"suggestions_count"`
	HighPriorityCount    int     `json:"high_priority_count"`
	MediumPriorityCount  int     `json:"medium_priority_count"`
	LowPriorityCount     int     `json:"low_priority_count"`
	ViolationsCount      int     `json:"violations_count"`
	CriticalViolations   int     `json:"critical_violations"`
	PotentialSavings     float64 `json:"potential_savings"`     // 潜在节省（元）
	PotentialAdjustments int     `json:"potential_adjustments"` // 潜在调整数量
	AvgUtilization       float64 `json:"avg_utilization"`       // 平均利用率
	OverutilizedCount    int     `json:"overutilized_count"`    // 过度使用数量
	UnderutilizedCount   int     `json:"underutilized_count"`   // 低利用率数量
}

// CostImpactAnalysis 成本影响分析
type CostImpactAnalysis struct {
	CurrentMonthlyCost   float64 `json:"current_monthly_cost"`
	OptimizedMonthlyCost float64 `json:"optimized_monthly_cost"`
	SavingsAmount        float64 `json:"savings_amount"`
	SavingsPercent       float64 `json:"savings_percent"`
	Currency             string  `json:"currency"`
}

// ========== 配额优化器 ==========

// QuotaOptimizer 配额优化器
type QuotaOptimizer struct {
	mu              sync.RWMutex
	dataDir         string
	quotaProvider   QuotaDataProvider
	historyProvider HistoryDataProvider
	config          OptimizerConfig
	suggestions     map[string]*OptimizationSuggestion
	violations      map[string]*ViolationRecord
	adjustHistory   []AutoAdjustResult
	lastAnalysis    time.Time
}

// QuotaDataProvider 配额数据提供者接口
type QuotaDataProvider interface {
	GetAllUsage() ([]*QuotaUsageInfo, error)
	GetUserUsage(username string) ([]*QuotaUsageInfo, error)
	GetQuota(quotaID string) (*QuotaInfo, error)
	UpdateQuota(quotaID string, newLimit uint64) error
}

// HistoryDataProvider 历史数据提供者接口
type HistoryDataProvider interface {
	GetHistory(quotaID string, days int) ([]HistoryPoint, error)
	GetGrowthRate(quotaID string) (float64, error)
}

// QuotaUsageInfo 配额使用信息
type QuotaUsageInfo struct {
	QuotaID      string    `json:"quota_id"`
	TargetID     string    `json:"target_id"`
	TargetName   string    `json:"target_name"`
	VolumeName   string    `json:"volume_name"`
	Type         string    `json:"type"`
	HardLimit    uint64    `json:"hard_limit"`
	SoftLimit    uint64    `json:"soft_limit"`
	UsedBytes    uint64    `json:"used_bytes"`
	Available    uint64    `json:"available"`
	UsagePercent float64   `json:"usage_percent"`
	IsOverSoft   bool      `json:"is_over_soft"`
	IsOverHard   bool      `json:"is_over_hard"`
	LastChecked  time.Time `json:"last_checked"`
}

// QuotaInfo 配额信息
type QuotaInfo struct {
	ID         string    `json:"id"`
	TargetID   string    `json:"target_id"`
	TargetName string    `json:"target_name"`
	VolumeName string    `json:"volume_name"`
	Type       string    `json:"type"`
	HardLimit  uint64    `json:"hard_limit"`
	SoftLimit  uint64    `json:"soft_limit"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// OptimizerConfig 优化器配置
type OptimizerConfig struct {
	AutoAdjust AutoAdjustConfig `json:"auto_adjust"`
	Prediction PredictionConfig `json:"prediction"`
	Violation  ViolationConfig  `json:"violation"`
	PricePerGB float64          `json:"price_per_gb"`
	Currency   string           `json:"currency"`
}

// DefaultOptimizerConfig 默认优化器配置
func DefaultOptimizerConfig() OptimizerConfig {
	return OptimizerConfig{
		AutoAdjust: DefaultAutoAdjustConfig(),
		Prediction: DefaultPredictionConfig(),
		Violation:  DefaultViolationConfig(),
		PricePerGB: 0.1,
		Currency:   "CNY",
	}
}

// NewQuotaOptimizer 创建配额优化器
func NewQuotaOptimizer(dataDir string, quotaProvider QuotaDataProvider, historyProvider HistoryDataProvider, config OptimizerConfig) *QuotaOptimizer {
	optimizer := &QuotaOptimizer{
		dataDir:         dataDir,
		quotaProvider:   quotaProvider,
		historyProvider: historyProvider,
		config:          config,
		suggestions:     make(map[string]*OptimizationSuggestion),
		violations:      make(map[string]*ViolationRecord),
		adjustHistory:   make([]AutoAdjustResult, 0),
	}

	// 加载已有数据
	optimizer.load()

	return optimizer
}

// load 加载数据
func (o *QuotaOptimizer) load() error {
	// 加载优化建议
	suggestionPath := filepath.Join(o.dataDir, "suggestions.json")
	if data, err := os.ReadFile(suggestionPath); err == nil {
		var suggestions []*OptimizationSuggestion
		if err := json.Unmarshal(data, &suggestions); err == nil {
			for _, s := range suggestions {
				o.suggestions[s.ID] = s
			}
		}
	}

	// 加载违规记录
	violationPath := filepath.Join(o.dataDir, "violations.json")
	if data, err := os.ReadFile(violationPath); err == nil {
		var violations []*ViolationRecord
		if err := json.Unmarshal(data, &violations); err == nil {
			for _, v := range violations {
				o.violations[v.ID] = v
			}
		}
	}

	// 加载调整历史
	adjustPath := filepath.Join(o.dataDir, "adjust_history.json")
	if data, err := os.ReadFile(adjustPath); err == nil {
		json.Unmarshal(data, &o.adjustHistory)
	}

	return nil
}

// save 保存数据
func (o *QuotaOptimizer) save() error {
	if err := os.MkdirAll(o.dataDir, 0755); err != nil {
		return err
	}

	// 保存优化建议
	suggestions := make([]*OptimizationSuggestion, 0, len(o.suggestions))
	for _, s := range o.suggestions {
		suggestions = append(suggestions, s)
	}
	if data, err := json.MarshalIndent(suggestions, "", "  "); err == nil {
		os.WriteFile(filepath.Join(o.dataDir, "suggestions.json"), data, 0644)
	}

	// 保存违规记录
	violations := make([]*ViolationRecord, 0, len(o.violations))
	for _, v := range o.violations {
		violations = append(violations, v)
	}
	if data, err := json.MarshalIndent(violations, "", "  "); err == nil {
		os.WriteFile(filepath.Join(o.dataDir, "violations.json"), data, 0644)
	}

	// 保存调整历史
	if data, err := json.MarshalIndent(o.adjustHistory, "", "  "); err == nil {
		os.WriteFile(filepath.Join(o.dataDir, "adjust_history.json"), data, 0644)
	}

	return nil
}

// ========== 自动配额调整建议 ==========

// GenerateAdjustmentSuggestions 生成调整建议
func (o *QuotaOptimizer) GenerateAdjustmentSuggestions() ([]OptimizationSuggestion, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	usages, err := o.quotaProvider.GetAllUsage()
	if err != nil {
		return nil, err
	}

	suggestions := make([]OptimizationSuggestion, 0)

	for _, usage := range usages {
		suggestion := o.analyzeQuotaForAdjustment(usage)
		if suggestion != nil {
			suggestions = append(suggestions, *suggestion)

			// 保存到内存
			o.suggestions[suggestion.ID] = suggestion
		}
	}

	o.lastAnalysis = time.Now()
	o.save()

	return suggestions, nil
}

// analyzeQuotaForAdjustment 分析配额是否需要调整
func (o *QuotaOptimizer) analyzeQuotaForAdjustment(usage *QuotaUsageInfo) *OptimizationSuggestion {
	cfg := o.config.AutoAdjust
	if !cfg.Enabled {
		return nil
	}

	utilization := usage.UsagePercent / 100 // 转换为小数
	var suggestedLimit uint64
	var reason string
	var priority string
	var confidence float64

	// 判断是否需要调整
	if utilization > cfg.MaxUtilization {
		// 使用率过高，建议扩展
		// 计算建议的新限制
		growthRate, _ := o.historyProvider.GetGrowthRate(usage.QuotaID)
		buffer := uint64(float64(usage.UsedBytes) * cfg.GrowthBufferPercent)

		// 新限制 = 当前使用 + 增长缓冲 + 预测增长
		predictedGrowth := uint64(growthRate * 30) // 30天预测
		suggestedLimit = usage.UsedBytes + buffer + predictedGrowth

		// 应用最大调整百分比限制
		maxLimit := uint64(float64(usage.HardLimit) * (1 + cfg.MaxAdjustmentPercent))
		if suggestedLimit > maxLimit {
			suggestedLimit = maxLimit
		}

		reason = fmt.Sprintf("使用率 %.1f%% 超过阈值 %.1f%%，建议扩展配额", utilization*100, cfg.MaxUtilization*100)
		priority = "high"
		confidence = 0.8

	} else if utilization < cfg.MinUtilization {
		// 使用率过低，建议缩减
		// 计算建议的新限制
		suggestedLimit = uint64(float64(usage.UsedBytes) / cfg.MaxUtilization)

		// 应用最小配额限制
		minLimit := uint64(cfg.MinQuotaGB * 1024 * 1024 * 1024)
		if suggestedLimit < minLimit {
			suggestedLimit = minLimit
		}

		// 应用最大调整百分比限制
		minAllowed := uint64(float64(usage.HardLimit) * (1 - cfg.MaxAdjustmentPercent))
		if suggestedLimit < minAllowed {
			suggestedLimit = minAllowed
		}

		reason = fmt.Sprintf("使用率 %.1f%% 低于阈值 %.1f%%，建议缩减配额", utilization*100, cfg.MinUtilization*100)
		priority = "low"
		confidence = 0.7

	} else {
		// 使用率正常，无需调整
		return nil
	}

	// 检查变化是否足够大
	changePercent := float64(abs(int64(suggestedLimit)-int64(usage.HardLimit))) / float64(usage.HardLimit)
	if changePercent < cfg.AdjustmentThreshold {
		return nil
	}

	return &OptimizationSuggestion{
		ID:               generateID(),
		Type:             OptimizationAutoAdjust,
		QuotaID:          usage.QuotaID,
		TargetID:         usage.TargetID,
		TargetName:       usage.TargetName,
		VolumeName:       usage.VolumeName,
		CurrentLimit:     usage.HardLimit,
		SuggestedLimit:   suggestedLimit,
		AdjustmentReason: reason,
		Confidence:       confidence,
		Priority:         priority,
		Impact:           o.calculateImpact(usage, suggestedLimit),
		CreatedAt:        time.Now(),
		Status:           "pending",
	}
}

// ========== 配额使用预测 ==========

// PredictUsage 预测配额使用
func (o *QuotaOptimizer) PredictUsage(quotaID string) (*UsagePrediction, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// 获取当前使用情况
	usage, err := o.quotaProvider.GetQuota(quotaID)
	if err != nil {
		return nil, err
	}

	// 获取当前用量
	usages, _ := o.quotaProvider.GetAllUsage()
	var currentUsage *QuotaUsageInfo
	for _, u := range usages {
		if u.QuotaID == quotaID {
			currentUsage = u
			break
		}
	}

	if currentUsage == nil {
		return nil, fmt.Errorf("配额使用信息不存在: %s", quotaID)
	}

	// 获取历史数据
	history, err := o.historyProvider.GetHistory(quotaID, o.config.Prediction.HistoryDays)
	if err != nil || len(history) < o.config.Prediction.MinHistoryPoints {
		// 历史数据不足，返回简单预测
		return o.simplePrediction(currentUsage, usage), nil
	}

	// 使用选择的预测方法
	prediction := &UsagePrediction{
		QuotaID:          quotaID,
		TargetID:         usage.TargetID,
		TargetName:       usage.TargetName,
		VolumeName:       usage.VolumeName,
		CurrentUsage:     currentUsage.UsedBytes,
		CurrentLimit:     currentUsage.HardLimit,
		HistoryPoints:    history,
		PredictionPeriod: time.Duration(o.config.Prediction.PredictionDays) * 24 * time.Hour,
		GeneratedAt:      time.Now(),
	}

	switch o.config.Prediction.Method {
	case PredictionLinear:
		o.linearPrediction(prediction, history)
	case PredictionMovingAvg:
		o.movingAvgPrediction(prediction, history)
	default:
		o.linearPrediction(prediction, history)
	}

	// 计算预计填满时间
	if prediction.GrowthRate > 0 {
		prediction.PredictedDaysToFull = int(float64(currentUsage.HardLimit-currentUsage.UsedBytes) / prediction.GrowthRate)
		prediction.PredictedDaysToLimit = prediction.PredictedDaysToFull
	}

	// 生成建议
	prediction.Recommendation = o.generatePredictionRecommendation(prediction)

	return prediction, nil
}

// PredictAllUsage 预测所有配额使用
func (o *QuotaOptimizer) PredictAllUsage() ([]*UsagePrediction, error) {
	usages, err := o.quotaProvider.GetAllUsage()
	if err != nil {
		return nil, err
	}

	predictions := make([]*UsagePrediction, 0, len(usages))
	for _, usage := range usages {
		pred, err := o.PredictUsage(usage.QuotaID)
		if err == nil {
			predictions = append(predictions, pred)
		}
	}

	return predictions, nil
}

// simplePrediction 简单预测（历史数据不足时）
func (o *QuotaOptimizer) simplePrediction(usage *QuotaUsageInfo, quota *QuotaInfo) *UsagePrediction {
	prediction := &UsagePrediction{
		QuotaID:      usage.QuotaID,
		TargetID:     quota.TargetID,
		TargetName:   quota.TargetName,
		VolumeName:   quota.VolumeName,
		CurrentUsage: usage.UsedBytes,
		CurrentLimit: usage.HardLimit,
		GrowthRate:   0,
		GrowthTrend:  "stable",
		Confidence:   0.5,
		GeneratedAt:  time.Now(),
	}

	// 使用当前使用量作为预测值
	prediction.PredictedUsage = usage.UsedBytes
	prediction.PredictedGrowth = 0
	prediction.Recommendation = "历史数据不足，建议持续监控"

	return prediction
}

// linearPrediction 线性回归预测
func (o *QuotaOptimizer) linearPrediction(prediction *UsagePrediction, history []HistoryPoint) {
	if len(history) < 2 {
		prediction.GrowthRate = 0
		prediction.GrowthTrend = "stable"
		prediction.Confidence = 0.5
		return
	}

	// 简单线性回归
	n := len(history)
	var sumX, sumY, sumXY, sumX2 float64

	for i, point := range history {
		x := float64(i)
		y := float64(point.UsedBytes)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// 计算斜率（增长率）
	denominator := float64(n)*sumX2 - sumX*sumX
	if denominator == 0 {
		prediction.GrowthRate = 0
	} else {
		slope := (float64(n)*sumXY - sumX*sumY) / denominator
		prediction.GrowthRate = slope // 字节/数据点
	}

	// 计算预测值
	lastValue := float64(history[len(history)-1].UsedBytes)
	prediction.PredictedGrowth = uint64(prediction.GrowthRate * float64(o.config.Prediction.PredictionDays))
	prediction.PredictedUsage = uint64(lastValue + prediction.GrowthRate*float64(o.config.Prediction.PredictionDays))

	// 判断趋势
	if prediction.GrowthRate > 0 {
		prediction.GrowthTrend = "increasing"
	} else if prediction.GrowthRate < 0 {
		prediction.GrowthTrend = "decreasing"
	} else {
		prediction.GrowthTrend = "stable"
	}

	// 计算置信度（基于数据点数量和变化一致性）
	prediction.Confidence = math.Min(0.9, 0.5+float64(n)/float64(o.config.Prediction.MinHistoryPoints)*0.4)
}

// movingAvgPrediction 移动平均预测
func (o *QuotaOptimizer) movingAvgPrediction(prediction *UsagePrediction, history []HistoryPoint) {
	if len(history) < 2 {
		prediction.GrowthRate = 0
		prediction.GrowthTrend = "stable"
		prediction.Confidence = 0.5
		return
	}

	// 计算移动平均增长率
	window := 7 // 7天窗口
	if len(history) < window {
		window = len(history)
	}

	var totalGrowth float64
	for i := len(history) - window; i < len(history); i++ {
		if i > 0 {
			growth := float64(history[i].UsedBytes) - float64(history[i-1].UsedBytes)
			totalGrowth += growth
		}
	}

	avgGrowth := totalGrowth / float64(window-1)
	prediction.GrowthRate = avgGrowth

	// 预测
	prediction.PredictedGrowth = uint64(avgGrowth * float64(o.config.Prediction.PredictionDays))
	prediction.PredictedUsage = history[len(history)-1].UsedBytes + prediction.PredictedGrowth

	// 趋势
	if avgGrowth > 0 {
		prediction.GrowthTrend = "increasing"
	} else if avgGrowth < 0 {
		prediction.GrowthTrend = "decreasing"
	} else {
		prediction.GrowthTrend = "stable"
	}

	prediction.Confidence = 0.7
}

// generatePredictionRecommendation 生成预测建议
func (o *QuotaOptimizer) generatePredictionRecommendation(prediction *UsagePrediction) string {
	if prediction.PredictedDaysToFull > 0 && prediction.PredictedDaysToFull <= 30 {
		return fmt.Sprintf("预计 %d 天内将达到配额限制，建议立即扩展配额", prediction.PredictedDaysToFull)
	} else if prediction.PredictedDaysToFull > 30 && prediction.PredictedDaysToFull <= 90 {
		return fmt.Sprintf("预计 %d 天内将达到配额限制，建议规划配额扩展", prediction.PredictedDaysToFull)
	} else if prediction.GrowthTrend == "increasing" {
		return "存储使用呈增长趋势，建议持续监控并规划未来容量"
	} else if prediction.GrowthTrend == "stable" {
		return "存储使用稳定，当前配额配置合理"
	}
	return "配额使用情况正常"
}

// ========== 配额违规检测 ==========

// DetectViolations 检测违规
func (o *QuotaOptimizer) DetectViolations() ([]ViolationRecord, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	usages, err := o.quotaProvider.GetAllUsage()
	if err != nil {
		return nil, err
	}

	violations := make([]ViolationRecord, 0)
	now := time.Now()

	for _, usage := range usages {
		// 检查硬限制违规
		if usage.IsOverHard {
			violation := ViolationRecord{
				ID:           generateID(),
				QuotaID:      usage.QuotaID,
				TargetID:     usage.TargetID,
				TargetName:   usage.TargetName,
				VolumeName:   usage.VolumeName,
				Type:         ViolationHardLimit,
				Severity:     "critical",
				UsedBytes:    usage.UsedBytes,
				LimitBytes:   usage.HardLimit,
				OverageBytes: usage.UsedBytes - usage.HardLimit,
				UsagePercent: usage.UsagePercent,
				Message:      fmt.Sprintf("配额使用超过硬限制 %.1f%%", usage.UsagePercent),
				DetectedAt:   now,
				Status:       "active",
				Actions:      []string{"立即清理文件", "申请增加配额", "联系管理员"},
			}
			violations = append(violations, violation)
			o.violations[violation.ID] = &violation
		}

		// 检查软限制违规
		if usage.IsOverSoft && !usage.IsOverHard {
			violation := ViolationRecord{
				ID:           generateID(),
				QuotaID:      usage.QuotaID,
				TargetID:     usage.TargetID,
				TargetName:   usage.TargetName,
				VolumeName:   usage.VolumeName,
				Type:         ViolationSoftLimit,
				Severity:     "warning",
				UsedBytes:    usage.UsedBytes,
				LimitBytes:   usage.SoftLimit,
				OverageBytes: usage.UsedBytes - usage.SoftLimit,
				UsagePercent: usage.UsagePercent,
				Message:      fmt.Sprintf("配额使用超过软限制 %.1f%%", usage.UsagePercent),
				DetectedAt:   now,
				Status:       "active",
				Actions:      []string{"清理不需要的文件", "关注存储使用情况"},
			}
			violations = append(violations, violation)
			o.violations[violation.ID] = &violation
		}

		// 检查预测违规（使用简单的增长率判断，避免死锁）
		growthRate, _ := o.historyProvider.GetGrowthRate(usage.QuotaID)
		if growthRate > 0 {
			remaining := float64(usage.HardLimit - usage.UsedBytes)
			daysToFull := int(remaining / growthRate)
			if daysToFull > 0 && daysToFull <= 7 {
				violation := ViolationRecord{
					ID:           generateID(),
					QuotaID:      usage.QuotaID,
					TargetID:     usage.TargetID,
					TargetName:   usage.TargetName,
					VolumeName:   usage.VolumeName,
					Type:         ViolationProjected,
					Severity:     "warning",
					UsedBytes:    usage.UsedBytes,
					LimitBytes:   usage.HardLimit,
					UsagePercent: usage.UsagePercent,
					Message:      fmt.Sprintf("预计 %d 天内将超出配额限制", daysToFull),
					DetectedAt:   now,
					Status:       "active",
					Actions:      []string{"提前扩展配额", "清理历史数据"},
				}
				violations = append(violations, violation)
				o.violations[violation.ID] = &violation
			}
		}
	}

	o.save()

	return violations, nil
}

// ResolveViolation 解决违规
func (o *QuotaOptimizer) ResolveViolation(violationID, resolvedBy string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	violation, exists := o.violations[violationID]
	if !exists {
		return fmt.Errorf("违规记录不存在: %s", violationID)
	}

	now := time.Now()
	violation.Status = "resolved"
	violation.ResolvedAt = &now
	violation.ResolvedBy = resolvedBy
	violation.History = append(violation.History, ViolationEvent{
		Timestamp: now,
		Action:    "resolved",
		By:        resolvedBy,
	})

	o.save()

	return nil
}

// ========== 配额优化报告 ==========

// GenerateOptimizationReport 生成优化报告
func (o *QuotaOptimizer) GenerateOptimizationReport() (*OptimizationReport, error) {
	// 先获取需要的数据（不持有锁）
	usages, _ := o.quotaProvider.GetAllUsage()

	// 生成预测（需要独立获取锁）
	predictions, _ := o.PredictAllUsage()

	o.mu.Lock()
	defer o.mu.Unlock()

	now := time.Now()
	report := &OptimizationReport{
		ID:              generateID(),
		GeneratedAt:     now,
		PeriodStart:     now.AddDate(0, 0, -30),
		PeriodEnd:       now,
		Suggestions:     make([]OptimizationSuggestion, 0),
		Predictions:     make([]UsagePrediction, 0),
		Violations:      make([]ViolationRecord, 0),
		Recommendations: make([]string, 0),
	}

	// 生成调整建议
	for _, usage := range usages {
		suggestion := o.analyzeQuotaForAdjustment(usage)
		if suggestion != nil {
			report.Suggestions = append(report.Suggestions, *suggestion)
		}
	}

	// 添加预测
	for i := range predictions {
		report.Predictions = append(report.Predictions, *predictions[i])
	}

	// 获取活跃违规
	for _, v := range o.violations {
		if v.Status == "active" {
			report.Violations = append(report.Violations, *v)
		}
	}

	// 计算摘要
	report.Summary = o.calculateSummary(usages, report)

	// 计算成本影响
	report.CostImpact = o.calculateCostImpact(report)

	// 生成建议
	report.Recommendations = o.generateRecommendations(report)

	return report, nil
}

// ========== 辅助方法 ==========

// calculateImpact 计算影响
func (o *QuotaOptimizer) calculateImpact(usage *QuotaUsageInfo, suggestedLimit uint64) string {
	changePercent := float64(int64(suggestedLimit)-int64(usage.HardLimit)) / float64(usage.HardLimit) * 100
	if changePercent > 0 {
		return fmt.Sprintf("配额将增加 %.1f%%，用户可用空间增加 %s",
			changePercent, formatBytes(suggestedLimit-usage.HardLimit))
	}
	return fmt.Sprintf("配额将减少 %.1f%%，用户可用空间减少 %s",
		-changePercent, formatBytes(usage.HardLimit-suggestedLimit))
}

// calculateSummary 计算摘要
func (o *QuotaOptimizer) calculateSummary(usages []*QuotaUsageInfo, report *OptimizationReport) OptimizationSummary {
	summary := OptimizationSummary{
		TotalQuotas:      len(usages),
		AnalyzedQuotas:   len(usages),
		SuggestionsCount: len(report.Suggestions),
		ViolationsCount:  len(report.Violations),
	}

	var totalUtil float64
	for _, u := range usages {
		totalUtil += u.UsagePercent

		if u.UsagePercent > 85 {
			summary.OverutilizedCount++
		} else if u.UsagePercent < 30 {
			summary.UnderutilizedCount++
		}
	}

	if len(usages) > 0 {
		summary.AvgUtilization = totalUtil / float64(len(usages))
	}

	for _, s := range report.Suggestions {
		switch s.Priority {
		case "high":
			summary.HighPriorityCount++
		case "medium":
			summary.MediumPriorityCount++
		case "low":
			summary.LowPriorityCount++
		}
	}

	for _, v := range report.Violations {
		if v.Severity == "critical" {
			summary.CriticalViolations++
		}
	}

	return summary
}

// calculateCostImpact 计算成本影响
func (o *QuotaOptimizer) calculateCostImpact(report *OptimizationReport) CostImpactAnalysis {
	impact := CostImpactAnalysis{
		Currency: o.config.Currency,
	}

	// 计算当前成本
	usages, _ := o.quotaProvider.GetAllUsage()
	var totalBytes uint64
	for _, u := range usages {
		totalBytes += u.HardLimit
	}
	impact.CurrentMonthlyCost = float64(totalBytes) / (1024 * 1024 * 1024) * o.config.PricePerGB * 30

	// 计算优化后成本
	var optimizedBytes uint64
	for _, u := range usages {
		// 查找是否有建议调整
		adjusted := false
		for _, s := range report.Suggestions {
			if s.QuotaID == u.QuotaID && s.Status == "pending" {
				optimizedBytes += s.SuggestedLimit
				adjusted = true
				break
			}
		}
		if !adjusted {
			optimizedBytes += u.HardLimit
		}
	}
	impact.OptimizedMonthlyCost = float64(optimizedBytes) / (1024 * 1024 * 1024) * o.config.PricePerGB * 30

	// 计算节省
	if impact.OptimizedMonthlyCost < impact.CurrentMonthlyCost {
		impact.SavingsAmount = impact.CurrentMonthlyCost - impact.OptimizedMonthlyCost
		impact.SavingsPercent = impact.SavingsAmount / impact.CurrentMonthlyCost * 100
	}

	return impact
}

// generateRecommendations 生成建议
func (o *QuotaOptimizer) generateRecommendations(report *OptimizationReport) []string {
	recs := make([]string, 0)

	if report.Summary.CriticalViolations > 0 {
		recs = append(recs, fmt.Sprintf("发现 %d 个严重违规，建议立即处理", report.Summary.CriticalViolations))
	}

	if report.Summary.HighPriorityCount > 0 {
		recs = append(recs, fmt.Sprintf("发现 %d 个高优先级优化建议，建议优先处理", report.Summary.HighPriorityCount))
	}

	if report.Summary.OverutilizedCount > 0 {
		recs = append(recs, fmt.Sprintf("发现 %d 个配额使用率超过85%%，建议扩展或清理", report.Summary.OverutilizedCount))
	}

	if report.Summary.UnderutilizedCount > 0 {
		recs = append(recs, fmt.Sprintf("发现 %d 个配额使用率低于30%%，建议回收或调整", report.Summary.UnderutilizedCount))
	}

	if report.CostImpact.SavingsAmount > 0 {
		recs = append(recs, fmt.Sprintf("优化后可节省 %.2f 元/月", report.CostImpact.SavingsAmount))
	}

	if len(recs) == 0 {
		recs = append(recs, "当前配额配置合理，无需优化")
	}

	return recs
}

// ApplySuggestion 应用建议
func (o *QuotaOptimizer) ApplySuggestion(suggestionID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	suggestion, exists := o.suggestions[suggestionID]
	if !exists {
		return fmt.Errorf("建议不存在: %s", suggestionID)
	}

	if suggestion.Status != "pending" {
		return fmt.Errorf("建议状态不是待处理")
	}

	// 应用调整
	err := o.quotaProvider.UpdateQuota(suggestion.QuotaID, suggestion.SuggestedLimit)
	if err != nil {
		return err
	}

	// 更新状态
	now := time.Now()
	suggestion.Status = "applied"
	suggestion.AppliedAt = &now

	// 记录调整历史
	o.adjustHistory = append(o.adjustHistory, AutoAdjustResult{
		QuotaID:       suggestion.QuotaID,
		TargetName:    suggestion.TargetName,
		OriginalLimit: suggestion.CurrentLimit,
		NewLimit:      suggestion.SuggestedLimit,
		Reason:        suggestion.AdjustmentReason,
		AdjustedAt:    now,
	})

	o.save()

	return nil
}

// DismissSuggestion 忽略建议
func (o *QuotaOptimizer) DismissSuggestion(suggestionID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	suggestion, exists := o.suggestions[suggestionID]
	if !exists {
		return fmt.Errorf("建议不存在: %s", suggestionID)
	}

	suggestion.Status = "dismissed"
	o.save()

	return nil
}

// GetSuggestions 获取建议列表
func (o *QuotaOptimizer) GetSuggestions(status string) []OptimizationSuggestion {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]OptimizationSuggestion, 0)
	for _, s := range o.suggestions {
		if status == "" || s.Status == status {
			result = append(result, *s)
		}
	}
	return result
}

// GetViolations 获取违规列表
func (o *QuotaOptimizer) GetViolations(status string) []ViolationRecord {
	o.mu.RLock()
	defer o.mu.RUnlock()

	result := make([]ViolationRecord, 0)
	for _, v := range o.violations {
		if status == "" || v.Status == status {
			result = append(result, *v)
		}
	}
	return result
}

// GetAdjustHistory 获取调整历史
func (o *QuotaOptimizer) GetAdjustHistory(limit int) []AutoAdjustResult {
	o.mu.RLock()
	defer o.mu.RUnlock()

	if limit <= 0 || limit > len(o.adjustHistory) {
		limit = len(o.adjustHistory)
	}

	result := make([]AutoAdjustResult, limit)
	copy(result, o.adjustHistory[len(o.adjustHistory)-limit:])
	return result
}

// ========== 工具函数 ==========

func generateID() string {
	return fmt.Sprintf("opt-%d-%s", time.Now().UnixNano(), randomString(6))
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().Nanosecond()%len(letters)]
	}
	return string(b)
}

func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func formatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
