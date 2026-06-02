package smartquotarecommender

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

// QuotaRecommender 配额推荐引擎
type QuotaRecommender struct {
	mu              sync.RWMutex
	profiles        map[string]*UserProfile      // 用户画像映射
	departments     map[string]*DepartmentPolicy // 部门策略映射
	forecastModel   *ForecastModel               // 容量预测模型
	anomalyDetector *AnomalyDetector             // 异常检测器
	alertManager    *AlertManager                // 告警管理器
	compliance      *ComplianceChecker           // 合规检查器
	reportGen       *ReportGenerator             // 报表生成器
}

// UserProfile 用户画像
type UserProfile struct {
	UserID        string          // 用户ID
	Department    string          // 所属部门
	Role          string          // 角色（admin/manager/developer/designer等）
	CurrentQuota  int64           // 当前配额（字节）
	CurrentUsage  int64           // 当前使用量（字节）
	History       []UsageSnapshot // 历史用量快照
	CreatedAt     time.Time       // 创建时间
	LastActive    time.Time       // 最后活跃时间
	BusinessType  string          // 业务类型（开发/设计/测试/运营等）
	PriorityLevel int             // 优先级（1-5，5最高）
}

// UsageSnapshot 用量快照
type UsageSnapshot struct {
	Timestamp time.Time // 快照时间
	Used      int64     // 使用量（字节）
	Quota     int64     // 配额（字节）
}

// UsagePattern 使用模式分析
type UsagePattern struct {
	DailyGrowth    float64       // 日均增量（字节/天）
	PeakUsage      int64         // 历史峰值（字节）
	AverageUsage   int64         // 平均使用量（字节）
	Trend          TrendType     // 趋势类型
	WeeklyPattern  [7]float64    // 每周使用模式（周一到周日）
	MonthlyPattern [12]float64   // 每月使用模式（1-12月）
	Volatility     float64       // 波动率（标准差/均值）
	AnalysisPeriod time.Duration // 分析周期
}

// TrendType 趋势类型
type TrendType string

const (
	TrendGrowing   TrendType = "growing"   // 增长趋势
	TrendDeclining TrendType = "declining" // 下降趋势
	TrendStable    TrendType = "stable"    // 稳定
	TrendVolatile  TrendType = "volatile"  // 波动大
	TrendSeasonal  TrendType = "seasonal"  // 季节性
)

// QuotaRecommendation 推荐结果
type QuotaRecommendation struct {
	UserID           string    // 用户ID
	CurrentQuota     int64     // 当前配额（字节）
	RecommendedQuota int64     // 推荐配额（字节）
	Confidence       float64   // 置信度（0.0-1.0）
	Reasons          []string  // 推荐理由列表
	UrgencyLevel     int       // 紧急程度（1-5）
	EffectiveDate    time.Time // 建议生效日期
	ReviewDate       time.Time // 复审日期
	AlternativeQuota int64     // 备选配额（保守方案）
}

// DepartmentPolicy 部门配额策略
type DepartmentPolicy struct {
	DepartmentID    string        // 部门ID
	Name            string        // 部门名称
	DefaultQuota    int64         // 默认配额（字节）
	MaxQuota        int64         // 最大配额（字节）
	MinQuota        int64         // 最小配额（字节）
	GrowthRate      float64       // 允许的增长率（%）
	ReviewCycle     time.Duration // 复审周期
	ApprovalNeeded  bool          // 超额需要审批
	ComplianceRules []string      // 合规规则
}

// AnomalyDetector 用量异常检测
type AnomalyDetector struct {
	mu               sync.RWMutex
	threshold        float64        // 异常阈值（标准差倍数）
	windowSize       int            // 窗口大小
	anomalies        []AnomalyEvent // 异常事件记录
	detectionEnabled bool           // 是否启用检测
}

// AnomalyEvent 异常事件
type AnomalyEvent struct {
	UserID    string      // 用户ID
	Timestamp time.Time   // 发生时间
	Type      AnomalyType // 异常类型
	Value     float64     // 异常值
	Expected  float64     // 期望值
	Severity  int         // 严重程度（1-5）
	Resolved  bool        // 是否已解决
}

// AnomalyType 异常类型
type AnomalyType string

const (
	AnomalySuddenSpike   AnomalyType = "sudden_spike"   // 突然激增
	AnomalyRapidGrowth   AnomalyType = "rapid_growth"   // 快速增长
	AnomalyUnusualAccess AnomalyType = "unusual_access" // 异常访问
	AnomalyQuotaExceeded AnomalyType = "quota_exceeded" // 配额超限
)

// ForecastModel 容量预测模型
type ForecastModel struct {
	mu             sync.RWMutex
	Method         ForecastMethod // 预测方法
	WindowSize     int            // 窗口大小
	Alpha          float64        // 指数平滑系数
	Beta           float64        // 趋势系数
	Gamma          float64        // 季节性系数
	SeasonalPeriod int            // 季节性周期（天）
	trained        bool           // 是否已训练
	historicalData []float64      // 历史数据
}

// ForecastMethod 预测方法
type ForecastMethod string

const (
	MethodLinearRegression     ForecastMethod = "linear_regression"     // 线性回归
	MethodExponentialSmoothing ForecastMethod = "exponential_smoothing" // 指数平滑
	MethodDoubleExponential    ForecastMethod = "double_exponential"    // 双指数平滑（Holt）
	MethodTripleExponential    ForecastMethod = "triple_exponential"    // 三指数平滑（Holt-Winters）
)

// ForecastResult 预测结果
type ForecastResult struct {
	Predictions []float64      // 预测值
	Confidence  []float64      // 置信区间上界
	LowerBound  []float64      // 置信区间下界
	Periods     int            // 预测期数
	Method      ForecastMethod // 使用的方法
	MSE         float64        // 均方误差
}

// AlertThreshold 告警阈值管理
type AlertThreshold struct {
	mu          sync.RWMutex
	thresholds  map[string]*ThresholdConfig // 阈值配置映射
	subscribers map[string][]string         // 订阅者列表
}

// ThresholdConfig 阈值配置
type ThresholdConfig struct {
	Name          string        // 阈值名称
	Level         AlertLevel    // 告警级别
	Percentage    float64       // 使用率阈值（百分比）
	Duration      time.Duration // 持续时间
	Enabled       bool          // 是否启用
	LastTriggered time.Time     // 最后触发时间
}

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertInfo     AlertLevel = "info"     // 信息
	AlertWarning  AlertLevel = "warning"  // 警告
	AlertCritical AlertLevel = "critical" // 严重
	AlertUrgent   AlertLevel = "urgent"   // 紧急
)

// Alert 告警消息
type Alert struct {
	ID        string     // 告警ID
	UserID    string     // 用户ID
	Level     AlertLevel // 告警级别
	Message   string     // 告警消息
	Value     float64    // 当前值
	Threshold float64    // 阈值
	Timestamp time.Time  // 触发时间
	Resolved  bool       // 是否已解决
}

// AlertManager 告警管理器
type AlertManager struct {
	alerts []Alert
	mu     sync.RWMutex
}

// ComplianceChecker 合规检查器
type ComplianceChecker struct {
	rules []ComplianceRule
	mu    sync.RWMutex
}

// ComplianceRule 合规规则
type ComplianceRule struct {
	ID          string  // 规则ID
	Name        string  // 规则名称
	Description string  // 规则描述
	Department  string  // 适用部门（空表示全部）
	MinQuota    int64   // 最小配额限制
	MaxQuota    int64   // 最大配额限制
	MaxGrowth   float64 // 最大增长率（%）
	Enabled     bool    // 是否启用
}

// ComplianceResult 合规检查结果
type ComplianceResult struct {
	Compliant  bool      // 是否合规
	Violations []string  // 违规项
	Warnings   []string  // 警告项
	CheckedAt  time.Time // 检查时间
}

// ReportGenerator 报表生成器
type ReportGenerator struct {
	mu      sync.RWMutex
	reports []Report // 已生成报表
}

// Report 报表
type Report struct {
	ID          string        // 报表ID
	Type        ReportType    // 报表类型
	Title       string        // 标题
	GeneratedAt time.Time     // 生成时间
	Period      time.Duration // 统计周期
	Summary     ReportSummary // 摘要
	Details     interface{}   // 详细数据
}

// ReportType 报表类型
type ReportType string

const (
	ReportUsage      ReportType = "usage"      // 使用量报表
	ReportQuota      ReportType = "quota"      // 配额报表
	ReportAnomaly    ReportType = "anomaly"    // 异常报表
	ReportCompliance ReportType = "compliance" // 合规报表
)

// ReportSummary 报表摘要
type ReportSummary struct {
	TotalUsers     int      // 总用户数
	TotalQuota     int64    // 总配额（字节）
	TotalUsage     int64    // 总使用量（字节）
	UsageRate      float64  // 使用率
	AnomalyCount   int      // 异常数量
	ComplianceRate float64  // 合规率
	TopConsumers   []string // 消耗大户
}

// NewQuotaRecommender 创建新的配额推荐引擎
func NewQuotaRecommender() *QuotaRecommender {
	return &QuotaRecommender{
		profiles:        make(map[string]*UserProfile),
		departments:     make(map[string]*DepartmentPolicy),
		forecastModel:   NewForecastModel(),
		anomalyDetector: NewAnomalyDetector(),
		alertManager:    NewAlertManager(),
		compliance:      NewComplianceChecker(),
		reportGen:       NewReportGenerator(),
	}
}

// NewForecastModel 创建预测模型
func NewForecastModel() *ForecastModel {
	return &ForecastModel{
		Method:         MethodExponentialSmoothing,
		WindowSize:     30,
		Alpha:          0.3,
		Beta:           0.1,
		Gamma:          0.1,
		SeasonalPeriod: 7,
	}
}

// NewAnomalyDetector 创建异常检测器
func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{
		threshold:        2.0,
		windowSize:       30,
		detectionEnabled: true,
	}
}

// NewAlertManager 创建告警管理器
func NewAlertManager() *AlertManager {
	return &AlertManager{}
}

// NewComplianceChecker 创建合规检查器
func NewComplianceChecker() *ComplianceChecker {
	return &ComplianceChecker{}
}

// NewReportGenerator 创建报表生成器
func NewReportGenerator() *ReportGenerator {
	return &ReportGenerator{}
}

// AddProfile 添加用户画像
func (qr *QuotaRecommender) AddProfile(profile *UserProfile) error {
	if profile.UserID == "" {
		return errors.New("用户ID不能为空")
	}
	qr.mu.Lock()
	defer qr.mu.Unlock()
	qr.profiles[profile.UserID] = profile
	return nil
}

// GetProfile 获取用户画像
func (qr *QuotaRecommender) GetProfile(userID string) (*UserProfile, error) {
	qr.mu.RLock()
	defer qr.mu.RUnlock()
	profile, exists := qr.profiles[userID]
	if !exists {
		return nil, fmt.Errorf("用户 %s 不存在", userID)
	}
	return profile, nil
}

// AddDepartmentPolicy 添加部门策略
func (qr *QuotaRecommender) AddDepartmentPolicy(policy *DepartmentPolicy) error {
	if policy.DepartmentID == "" {
		return errors.New("部门ID不能为空")
	}
	qr.mu.Lock()
	defer qr.mu.Unlock()
	qr.departments[policy.DepartmentID] = policy
	return nil
}

// AnalyzeUsagePattern 分析使用模式
func (qr *QuotaRecommender) AnalyzeUsagePattern(userID string) (*UsagePattern, error) {
	qr.mu.RLock()
	profile, exists := qr.profiles[userID]
	qr.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("用户 %s 不存在", userID)
	}

	history := profile.History
	if len(history) < 2 {
		return nil, errors.New("历史数据不足，至少需要2条记录")
	}

	pattern := &UsagePattern{
		AnalysisPeriod: time.Duration(len(history)) * 24 * time.Hour,
	}

	// 计算日均增量
	totalGrowth := float64(history[len(history)-1].Used - history[0].Used)
	pattern.DailyGrowth = totalGrowth / float64(len(history))

	// 计算峰值和平均值
	var sum int64
	pattern.PeakUsage = history[0].Used
	for _, snap := range history {
		sum += snap.Used
		if snap.Used > pattern.PeakUsage {
			pattern.PeakUsage = snap.Used
		}
	}
	pattern.AverageUsage = sum / int64(len(history))

	// 计算波动率
	var variance float64
	for _, snap := range history {
		diff := float64(snap.Used) - float64(pattern.AverageUsage)
		variance += diff * diff
	}
	stddev := math.Sqrt(variance / float64(len(history)))
	if pattern.AverageUsage > 0 {
		pattern.Volatility = stddev / float64(pattern.AverageUsage)
	}

	// 确定趋势
	pattern.Trend = determineTrend(history)

	return pattern, nil
}

// determineTrend 根据历史数据确定趋势
func determineTrend(history []UsageSnapshot) TrendType {
	if len(history) < 2 {
		return TrendStable
	}

	// 计算斜率（简化版线性回归）
	n := float64(len(history))
	var sumX, sumY, sumXY, sumX2 float64
	for i, snap := range history {
		x := float64(i)
		y := float64(snap.Used)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	avgY := sumY / n

	// 计算标准差
	var variance float64
	for _, snap := range history {
		diff := float64(snap.Used) - avgY
		variance += diff * diff
	}
	stddev := math.Sqrt(variance / n)

	// 根据斜率和波动率判断趋势
	if stddev/avgY > 0.3 {
		return TrendVolatile
	}
	if slope > avgY*0.01 {
		return TrendGrowing
	}
	if slope < -avgY*0.01 {
		return TrendDeclining
	}
	return TrendStable
}

// RecommendQuota 生成配额推荐
func (qr *QuotaRecommender) RecommendQuota(userID string) (*QuotaRecommendation, error) {
	qr.mu.RLock()
	profile, exists := qr.profiles[userID]
	qr.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("用户 %s 不存在", userID)
	}

	// 分析使用模式
	pattern, err := qr.AnalyzeUsagePattern(userID)
	if err != nil {
		return nil, fmt.Errorf("分析使用模式失败: %w", err)
	}

	recommendation := &QuotaRecommendation{
		UserID:        userID,
		CurrentQuota:  profile.CurrentQuota,
		EffectiveDate: time.Now(),
		ReviewDate:    time.Now().AddDate(0, 1, 0), // 一个月后复审
	}

	// 基于趋势和预测计算推荐配额
	var recommendedQuota int64

	switch pattern.Trend {
	case TrendGrowing:
		// 增长趋势：预测未来30天需求
		predictedGrowth := pattern.DailyGrowth * 30
		recommendedQuota = profile.CurrentUsage + int64(predictedGrowth)
		// 预留20%缓冲
		recommendedQuota = int64(float64(recommendedQuota) * 1.2)
		recommendation.Reasons = append(recommendation.Reasons,
			"检测到增长趋势，预测30天后需求量")
		recommendation.UrgencyLevel = 3

	case TrendDeclining:
		// 下降趋势：适度降低配额
		recommendedQuota = int64(float64(profile.CurrentUsage) * 1.1)
		recommendation.Reasons = append(recommendation.Reasons,
			"检测到下降趋势，建议适度降低配额")

	case TrendVolatile:
		// 波动大：保持当前配额，加缓冲
		recommendedQuota = int64(float64(pattern.PeakUsage) * 1.3)
		recommendation.Reasons = append(recommendation.Reasons,
			"检测到高波动性，建议预留充足缓冲空间")
		recommendation.UrgencyLevel = 2

	default: // Stable
		// 稳定：基于平均使用量+缓冲
		recommendedQuota = int64(float64(pattern.AverageUsage) * 1.5)
		recommendation.Reasons = append(recommendation.Reasons,
			"使用量稳定，建议基于平均使用量配额")
	}

	// 应用部门策略约束
	if policy, exists := qr.departments[profile.Department]; exists {
		if recommendedQuota < policy.MinQuota {
			recommendedQuota = policy.MinQuota
			recommendation.Reasons = append(recommendation.Reasons,
				fmt.Sprintf("已调整至部门最低配额限制 %d GB", policy.MinQuota/1024/1024/1024))
		}
		if recommendedQuota > policy.MaxQuota {
			recommendedQuota = policy.MaxQuota
			recommendation.Reasons = append(recommendation.Reasons,
				fmt.Sprintf("已限制至部门最大配额 %d GB", policy.MaxQuota/1024/1024/1024))
			recommendation.UrgencyLevel = 4 // 需要关注
		}
	}

	recommendation.RecommendedQuota = recommendedQuota

	// 计算置信度（基于数据量和波动性）
	dataConfidence := math.Min(float64(len(profile.History))/30.0, 1.0)
	volatilityPenalty := pattern.Volatility * 0.3
	recommendation.Confidence = math.Max(0, math.Min(1.0,
		dataConfidence-volatilityPenalty))

	// 备选方案（保守）
	recommendation.AlternativeQuota = int64(float64(recommendedQuota) * 0.85)

	return recommendation, nil
}

// TrainForecastModel 训练预测模型
func (fm *ForecastModel) Train(data []float64) error {
	if len(data) < fm.WindowSize {
		return fmt.Errorf("数据不足，需要至少 %d 条记录", fm.WindowSize)
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.historicalData = make([]float64, len(data))
	copy(fm.historicalData, data)
	fm.trained = true

	return nil
}

// Forecast 执行预测
func (fm *ForecastModel) Forecast(periods int) (*ForecastResult, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	if !fm.trained {
		return nil, errors.New("模型未训练")
	}

	result := &ForecastResult{
		Periods:     periods,
		Method:      fm.Method,
		Predictions: make([]float64, periods),
		Confidence:  make([]float64, periods),
		LowerBound:  make([]float64, periods),
	}

	switch fm.Method {
	case MethodExponentialSmoothing:
		result.Predictions = fm.exponentialSmoothing(periods)
	case MethodLinearRegression:
		result.Predictions = fm.linearRegression(periods)
	default:
		result.Predictions = fm.exponentialSmoothing(periods)
	}

	// 计算置信区间（简化版）
	for i := 0; i < periods; i++ {
		margin := result.Predictions[i] * 0.1 * float64(i+1) / float64(periods)
		result.Confidence[i] = result.Predictions[i] + margin
		result.LowerBound[i] = math.Max(0, result.Predictions[i]-margin)
	}

	// 计算MSE
	result.MSE = fm.calculateMSE()

	return result, nil
}

// exponentialSmoothing 指数平滑预测
func (fm *ForecastModel) exponentialSmoothing(periods int) []float64 {
	data := fm.historicalData
	n := len(data)

	// 初始值
	smoothed := data[0]
	for i := 1; i < n; i++ {
		smoothed = fm.Alpha*data[i] + (1-fm.Alpha)*smoothed
	}

	predictions := make([]float64, periods)
	for i := 0; i < periods; i++ {
		predictions[i] = smoothed
	}

	return predictions
}

// linearRegression 线性回归预测
func (fm *ForecastModel) linearRegression(periods int) []float64 {
	data := fm.historicalData
	n := float64(len(data))

	var sumX, sumY, sumXY, sumX2 float64
	for i, y := range data {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	intercept := (sumY - slope*sumX) / n

	predictions := make([]float64, periods)
	for i := 0; i < periods; i++ {
		x := float64(len(data) + i)
		predictions[i] = intercept + slope*x
	}

	return predictions
}

// calculateMSE 计算均方误差
func (fm *ForecastModel) calculateMSE() float64 {
	if len(fm.historicalData) < 2 {
		return 0
	}

	// 使用最后20%数据作为验证集
	n := len(fm.historicalData)
	validSize := n / 5
	if validSize < 1 {
		validSize = 1
	}

	trainSize := n - validSize
	trainData := fm.historicalData[:trainSize]

	// 计算训练集预测
	alpha := fm.Alpha
	smoothed := trainData[0]
	for i := 1; i < len(trainData); i++ {
		smoothed = alpha*trainData[i] + (1-alpha)*smoothed
	}

	// 计算验证集MSE
	var mse float64
	for i := trainSize; i < n; i++ {
		predicted := smoothed
		actual := fm.historicalData[i]
		mse += (predicted - actual) * (predicted - actual)
	}

	return mse / float64(validSize)
}

// DetectAnomalies 检测异常
func (ad *AnomalyDetector) DetectAnomalies(userID string, data []float64) []AnomalyEvent {
	if !ad.detectionEnabled || len(data) < ad.windowSize {
		return nil
	}

	ad.mu.Lock()
	defer ad.mu.Unlock()

	var anomalies []AnomalyEvent

	// 计算窗口统计
	window := data[len(data)-ad.windowSize:]
	mean, stddev := calculateStats(window)

	// 检测异常点
	for i, value := range data {
		if math.Abs(value-mean) > ad.threshold*stddev {
			anomaly := AnomalyEvent{
				UserID:    userID,
				Timestamp: time.Now().AddDate(0, 0, i-len(data)),
				Value:     value,
				Expected:  mean,
				Resolved:  false,
			}

			// 判断异常类型
			if value > mean+3*stddev {
				anomaly.Type = AnomalySuddenSpike
				anomaly.Severity = 4
			} else if value > mean+2*stddev {
				anomaly.Type = AnomalyRapidGrowth
				anomaly.Severity = 3
			} else {
				anomaly.Type = AnomalyUnusualAccess
				anomaly.Severity = 2
			}

			anomalies = append(anomalies, anomaly)
			ad.anomalies = append(ad.anomalies, anomaly)
		}
	}

	return anomalies
}

// calculateStats 计算均值和标准差
func calculateStats(data []float64) (mean, stddev float64) {
	if len(data) == 0 {
		return 0, 0
	}

	var sum float64
	for _, v := range data {
		sum += v
	}
	mean = sum / float64(len(data))

	var variance float64
	for _, v := range data {
		diff := v - mean
		variance += diff * diff
	}
	stddev = math.Sqrt(variance / float64(len(data)))

	return mean, stddev
}

// SetThreshold 设置告警阈值
func (at *AlertThreshold) SetThreshold(config *ThresholdConfig) error {
	if config.Name == "" {
		return errors.New("阈值名称不能为空")
	}
	if config.Percentage < 0 || config.Percentage > 100 {
		return errors.New("百分比必须在0-100之间")
	}

	at.mu.Lock()
	defer at.mu.Unlock()
	at.thresholds[config.Name] = config
	return nil
}

// CheckThresholds 检查是否触发告警
func (at *AlertThreshold) CheckThresholds(userID string, usageRate float64) []Alert {
	at.mu.RLock()
	defer at.mu.RUnlock()

	var alerts []Alert
	for _, config := range at.thresholds {
		if !config.Enabled {
			continue
		}
		if usageRate >= config.Percentage {
			alert := Alert{
				ID:        fmt.Sprintf("%s-%s-%d", userID, config.Name, time.Now().Unix()),
				UserID:    userID,
				Level:     config.Level,
				Message:   fmt.Sprintf("使用率 %.1f%% 超过阈值 %.1f%%", usageRate, config.Percentage),
				Value:     usageRate,
				Threshold: config.Percentage,
				Timestamp: time.Now(),
				Resolved:  false,
			}
			alerts = append(alerts, alert)
		}
	}

	return alerts
}

// AddRule 添加合规规则
func (cc *ComplianceChecker) AddRule(rule ComplianceRule) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.rules = append(cc.rules, rule)
}

// CheckCompliance 检查合规性
func (cc *ComplianceChecker) CheckCompliance(profile *UserProfile, policy *DepartmentPolicy) *ComplianceResult {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	result := &ComplianceResult{
		Compliant: true,
		CheckedAt: time.Now(),
	}

	// 检查配额是否在部门范围内
	if policy != nil {
		if profile.CurrentQuota < policy.MinQuota {
			result.Compliant = false
			result.Violations = append(result.Violations,
				fmt.Sprintf("配额 %d GB 低于部门最低限制 %d GB",
					profile.CurrentQuota/1024/1024/1024,
					policy.MinQuota/1024/1024/1024))
		}
		if profile.CurrentQuota > policy.MaxQuota {
			result.Compliant = false
			result.Violations = append(result.Violations,
				fmt.Sprintf("配额 %d GB 超过部门最大限制 %d GB",
					profile.CurrentQuota/1024/1024/1024,
					policy.MaxQuota/1024/1024/1024))
		}
	}

	// 检查自定义规则
	for _, rule := range cc.rules {
		if !rule.Enabled {
			continue
		}
		if rule.Department != "" && rule.Department != profile.Department {
			continue
		}
		if rule.MinQuota > 0 && profile.CurrentQuota < rule.MinQuota {
			result.Compliant = false
			result.Violations = append(result.Violations,
				fmt.Sprintf("违反规则 %s: 配额低于最低限制", rule.Name))
		}
		if rule.MaxQuota > 0 && profile.CurrentQuota > rule.MaxQuota {
			result.Compliant = false
			result.Violations = append(result.Violations,
				fmt.Sprintf("违反规则 %s: 配额超过最大限制", rule.Name))
		}
	}

	return result
}

// GenerateReport 生成报表
func (rg *ReportGenerator) GenerateReport(reportType ReportType, profiles map[string]*UserProfile, period time.Duration) (*Report, error) {
	if len(profiles) == 0 {
		return nil, errors.New("没有用户数据")
	}

	rg.mu.Lock()
	defer rg.mu.Unlock()

	report := &Report{
		ID:          fmt.Sprintf("report-%d", time.Now().UnixNano()),
		Type:        reportType,
		GeneratedAt: time.Now(),
		Period:      period,
	}

	// 统计摘要
	summary := ReportSummary{
		TotalUsers: len(profiles),
	}

	for _, profile := range profiles {
		summary.TotalQuota += profile.CurrentQuota
		summary.TotalUsage += profile.CurrentUsage
	}

	if summary.TotalQuota > 0 {
		summary.UsageRate = float64(summary.TotalUsage) / float64(summary.TotalQuota) * 100
	}

	// 找出消耗大户（使用量超过80%的用户）
	for _, profile := range profiles {
		if profile.CurrentQuota > 0 {
			rate := float64(profile.CurrentUsage) / float64(profile.CurrentQuota) * 100
			if rate > 80 {
				summary.TopConsumers = append(summary.TopConsumers, profile.UserID)
			}
		}
	}

	report.Summary = summary

	// 设置标题
	switch reportType {
	case ReportUsage:
		report.Title = fmt.Sprintf("使用量报表 - %s", period.String())
	case ReportQuota:
		report.Title = fmt.Sprintf("配额报表 - %s", period.String())
	case ReportAnomaly:
		report.Title = fmt.Sprintf("异常报表 - %s", period.String())
	case ReportCompliance:
		report.Title = fmt.Sprintf("合规报表 - %s", period.String())
	}

	rg.reports = append(rg.reports, *report)

	return report, nil
}

// GetReports 获取所有报表
func (rg *ReportGenerator) GetReports() []Report {
	rg.mu.RLock()
	defer rg.mu.RUnlock()
	return rg.reports
}
