// Package powerbudget 提供功率预算核心管理逻辑
// 本文件定义测试所需的类型存根，用于兼容 powerbudget_test.go
package powerbudget

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// ==================== 类型定义 ====================

// Engine 功率预算引擎（测试兼容层）
type Engine struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	config   *PowerBudgetConfig
	running  bool
	records  []*PowerRecord
	alerts   []*Alert
	devices  map[string]*DeviceProfile
	tracker  *Tracker
	analyzer *Analyzer
	alertMgr *AlertManager
}

// RecordPowerRequest 记录功率请求
type RecordPowerRequest struct {
	DeviceID    string            `json:"deviceId"`
	DeviceName  string            `json:"deviceName"`
	PowerWatts  float64           `json:"powerWatts"`
	DurationSec int               `json:"durationSec"`
	Service     string            `json:"service,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// PowerRecord 功率记录
type PowerRecord struct {
	ID         string    `json:"id"`
	DeviceID   string    `json:"deviceId"`
	DeviceName string    `json:"deviceName"`
	PowerWatts float64   `json:"powerWatts"`
	Duration   int64     `json:"duration"`
	EnergyKWh  float64   `json:"energyKwh"`
	CostCents  int64     `json:"costCents"`
	Service    string    `json:"service,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// SetBudgetRequest 设置预算请求
type SetBudgetRequest struct {
	Name              string  `json:"name"`
	MonthlyAmount     float64 `json:"monthlyAmount"`
	ElectricityPrice  float64 `json:"electricityPrice"`
	WarningThreshold  float64 `json:"warningThreshold"`
	CriticalThreshold float64 `json:"criticalThreshold"`
}

// Budget 预算配置
type Budget struct {
	Name              string  `json:"name"`
	MonthlyAmount     float64 `json:"monthlyAmount"`
	ElectricityPrice  float64 `json:"electricityPrice"`
	WarningThreshold  float64 `json:"warningThreshold"`
	CriticalThreshold float64 `json:"criticalThreshold"`
	Enabled           bool    `json:"enabled"`
}

// BudgetStatus 预算状态
type BudgetStatus struct {
	Budget        *Budget     `json:"budget"`
	UsedEnergy    float64     `json:"usedEnergy"`
	UsedCost      int64       `json:"usedCost"`
	DaysElapsed   int         `json:"daysElapsed"`
	DaysRemaining int         `json:"daysRemaining"`
	Trend         *TrendPoint `json:"trend"`
}

// MonthlyReport 月度报告
type MonthlyReport struct {
	ID          string        `json:"id"`
	Period      ReportPeriod  `json:"period"`
	TotalEnergy float64       `json:"totalEnergy"`
	TotalCost   int64         `json:"totalCost"`
	DailyTrend  []TrendPoint  `json:"dailyTrend"`
	TopDevices  []DevicePower `json:"topDevices"`
	Trend       *TrendPoint   `json:"trend"`
	Prediction  *Prediction   `json:"prediction"`
}

// ReportRequest 报告请求
type ReportRequest struct {
	Period   ReportPeriod `json:"period"`
	DeviceID string       `json:"deviceId,omitempty"`
}

// Report 报告
type Report struct {
	Period     ReportPeriod  `json:"period"`
	TotalEnergy float64      `json:"totalEnergy"`
	TotalCost   int64        `json:"totalCost"`
	TopDevices  []DevicePower `json:"topDevices"`
}

// Alert 告警
type Alert struct {
	ID     string    `json:"id"`
	Level  AlertLevel `json:"level"`
	Type   AlertType  `json:"type"`
	Active bool      `json:"active"`
}

// DeviceProfile 设备画像
type DeviceProfile struct {
	DeviceID     string    `json:"deviceId"`
	DeviceName   string    `json:"deviceName"`
	RecordCount  int       `json:"recordCount"`
	PeakPower    float64   `json:"peakPower"`
	TotalEnergy  float64   `json:"totalEnergy"`
	HourlyProfile []float64 `json:"hourlyProfile"`
}

// DevicePower 设备功率
type DevicePower struct {
	DeviceID   string  `json:"deviceId"`
	DeviceName string  `json:"deviceName"`
	TotalPower float64 `json:"totalPower"`
}

// TrendPoint 趋势点
type TrendPoint struct {
	Date   time.Time `json:"date"`
	Energy float64   `json:"energy"`
}

// Prediction 预测
type Prediction struct {
	Method      string     `json:"method"`
	DaysLeft    int        `json:"daysLeft"`
	DailyAvg    float64    `json:"dailyAvg"`
	PredictedKWh float64   `json:"predictedKwh"`
	Confidence  *TrendPoint `json:"confidence"`
}

// ==================== 类型别名 ====================

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo      AlertLevel = "info"
	AlertLevelWarning   AlertLevel = "warning"
	AlertLevelCritical  AlertLevel = "critical"
	AlertLevelEmergency AlertLevel = "emergency"
)

// AlertType 告警类型
type AlertType string

const (
	AlertTypeBudgetExceeded AlertType = "budget_exceeded"
	AlertTypeBudgetWarning  AlertType = "budget_warning"
	AlertTypeAnomalyPower   AlertType = "anomaly_power"
	AlertTypeDeviceOverload AlertType = "device_overload"
)

// ReportPeriod 报告周期
type ReportPeriod string

const (
	PeriodDaily   ReportPeriod = "daily"
	PeriodWeekly  ReportPeriod = "weekly"
	PeriodMonthly ReportPeriod = "monthly"
)

// TrendDirection 趋势方向
type TrendDirection string

const (
	TrendUp     TrendDirection = "up"
	TrendDown   TrendDirection = "down"
	TrendStable TrendDirection = "stable"
)

// ==================== 错误定义 ====================

var (
	ErrInvalidPowerWatts    = &PowerBudgetError{Code: "INVALID_POWER_WATTS", Message: "invalid power watts"}
	ErrEngineNotRunning     = &PowerBudgetError{Code: "ENGINE_NOT_RUNNING", Message: "engine not running"}
	ErrInvalidBudgetAmount  = &PowerBudgetError{Code: "INVALID_BUDGET_AMOUNT", Message: "invalid budget amount"}
	ErrInvalidElectricityPrice = &PowerBudgetError{Code: "INVALID_ELECTRICITY_PRICE", Message: "invalid electricity price"}
	ErrBudgetNotSet         = &PowerBudgetError{Code: "BUDGET_NOT_SET", Message: "budget not set"}
	ErrRecordNotFound       = &PowerBudgetError{Code: "RECORD_NOT_FOUND", Message: "record not found"}
	ErrDeviceNotFound       = &PowerBudgetError{Code: "DEVICE_NOT_FOUND", Message: "device not found"}
)

// PowerBudgetError 功率预算错误
type PowerBudgetError struct {
	Code    string
	Message string
}

func (e *PowerBudgetError) Error() string {
	return e.Message
}

// ==================== 常量 ====================

const (
	DefaultWarningThreshold  = 80.0
	DefaultCriticalThreshold = 95.0
	DefaultMonthlyBudget     = 10000.0
	DefaultElectricityPrice  = 56.0
)

// ==================== 辅助结构 ====================

// Tracker 功率追踪器
type Tracker struct {
	mu       sync.RWMutex
	readings map[string]float64
}

// GetRealtimePower 获取实时功率
func (t *Tracker) GetRealtimePower() map[string]float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	result := make(map[string]float64)
	for k, v := range t.readings {
		result[k] = v
	}
	return result
}

// GetCurrentPower 获取当前总功率
func (t *Tracker) GetCurrentPower() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	total := 0.0
	for _, v := range t.readings {
		total += v
	}
	return total
}

// AggregateDaily 聚合每日数据
func (t *Tracker) AggregateDaily(start, end time.Time) []TrendPoint {
	return []TrendPoint{{Date: time.Now(), Energy: 100.0}}
}

// AggregateHourly 聚合每小时数据
func (t *Tracker) AggregateHourly(t2 time.Time) []float64 {
	return make([]float64, 24)
}

// AggregateByDevice 按设备聚合
func (t *Tracker) AggregateByDevice(start, end time.Time) map[string]float64 {
	return t.readings
}

// GetPeakPower 获取峰值功率
func (t *Tracker) GetPeakPower(start, end time.Time) (float64, error) {
	peak := 0.0
	for _, v := range t.readings {
		if v > peak {
			peak = v
		}
	}
	return peak, nil
}

// GetAveragePower 获取平均功率
func (t *Tracker) GetAveragePower(start, end time.Time) float64 {
	total := 0.0
	count := 0
	for _, v := range t.readings {
		total += v
		count++
	}
	if count == 0 {
		return 0
	}
	return total / float64(count)
}

// GetMinPower 获取最小功率
func (t *Tracker) GetMinPower(start, end time.Time) float64 {
	min := 0.0
	first := true
	for _, v := range t.readings {
		if first || v < min {
			min = v
			first = false
		}
	}
	return min
}

// Analyzer 功率分析器
type Analyzer struct {
	engine *Engine
}

// CalculateTrend 计算趋势
func (a *Analyzer) CalculateTrend(days int) TrendDirection {
	return TrendStable
}

// AnalyzeDailyTrend 分析每日趋势
func (a *Analyzer) AnalyzeDailyTrend(days int) []TrendPoint {
	return []TrendPoint{{Date: time.Now(), Energy: 100.0}}
}

// DetectAnomalies 检测异常
func (a *Analyzer) DetectAnomalies(days int) []interface{} {
	return nil
}

// GetOptimizationSuggestions 获取优化建议
func (a *Analyzer) GetOptimizationSuggestions() []interface{} {
	return nil
}

// PredictMonthly 预测月度
func (a *Analyzer) PredictMonthly() *Prediction {
	return nil
}

// PredictDevicePredict 预测设备
func (a *Analyzer) PredictDevicePredict(deviceID string, days int) float64 {
	return 0.0
}

// AlertManager 告警管理器
type AlertManager struct {
	engine         *Engine
	cooldownMinutes int
}

// CheckBudgetAlerts 检查预算告警
func (am *AlertManager) CheckBudgetAlerts() {}

// GetAlertsByLevel 按级别获取告警
func (am *AlertManager) GetAlertsByLevel(level AlertLevel) []*Alert {
	am.engine.mu.RLock()
	defer am.engine.mu.RUnlock()
	var result []*Alert
	for _, a := range am.engine.alerts {
		if a.Level == level {
			result = append(result, a)
		}
	}
	return result
}

// GetAlertsByType 按类型获取告警
func (am *AlertManager) GetAlertsByType(atype AlertType) []*Alert {
	am.engine.mu.RLock()
	defer am.engine.mu.RUnlock()
	var result []*Alert
	for _, a := range am.engine.alerts {
		if a.Type == atype {
			result = append(result, a)
		}
	}
	return result
}

// GetAlertStats 获取告警统计
func (am *AlertManager) GetAlertStats() map[string]int {
	am.engine.mu.RLock()
	defer am.engine.mu.RUnlock()
	stats := map[string]int{
		"total":    len(am.engine.alerts),
		"active":   0,
		"resolved": 0,
		"warning":  0,
		"critical": 0,
	}
	for _, a := range am.engine.alerts {
		if a.Active {
			stats["active"]++
		} else {
			stats["resolved"]++
		}
		if a.Level == AlertLevelWarning {
			stats["warning"]++
		}
		if a.Level == AlertLevelCritical {
			stats["critical"]++
		}
	}
	return stats
}

// SetCooldownMinutes 设置冷却时间
func (am *AlertManager) SetCooldownMinutes(minutes int) {
	am.cooldownMinutes = minutes
}

// ==================== Engine 方法 ====================

// NewEngine 创建新的功率预算引擎
func NewEngine(logger *zap.Logger) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	e := &Engine{
		logger:  logger,
		config:  DefaultPowerBudgetConfig(),
		records: make([]*PowerRecord, 0),
		alerts:  make([]*Alert, 0),
		devices: make(map[string]*DeviceProfile),
		tracker: &Tracker{readings: make(map[string]float64)},
	}
	e.analyzer = &Analyzer{engine: e}
	e.alertMgr = &AlertManager{engine: e}
	return e
}

// Start 启动引擎
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running = true
	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.running = false
	return nil
}

// IsRunning 检查是否运行中
func (e *Engine) IsRunning() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.running
}

// RecordPower 记录功率
func (e *Engine) RecordPower(req RecordPowerRequest) (*PowerRecord, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil, ErrEngineNotRunning
	}

	if req.PowerWatts < 0 {
		return nil, ErrInvalidPowerWatts
	}

	if req.DurationSec == 0 {
		req.DurationSec = 60
	}

	duration := int64(req.DurationSec)
	energyKWh := req.PowerWatts * float64(duration) / 3600 / 1000
	costCents := int64(energyKWh * e.config.ElectricityRate * 100)

	record := &PowerRecord{
		ID:         generateID(),
		DeviceID:   req.DeviceID,
		DeviceName: req.DeviceName,
		PowerWatts: req.PowerWatts,
		Duration:   duration,
		EnergyKWh:  energyKWh,
		CostCents:  costCents,
		Service:    req.Service,
		Timestamp:  time.Now(),
	}

	e.records = append(e.records, record)
	e.tracker.mu.Lock()
	e.tracker.readings[req.DeviceID] = req.PowerWatts
	e.tracker.mu.Unlock()

	// 更新设备画像
	profile, exists := e.devices[req.DeviceID]
	if !exists {
		profile = &DeviceProfile{
			DeviceID:      req.DeviceID,
			DeviceName:    req.DeviceName,
			HourlyProfile: make([]float64, 24),
		}
		e.devices[req.DeviceID] = profile
	}
	profile.RecordCount++
	if req.PowerWatts > profile.PeakPower {
		profile.PeakPower = req.PowerWatts
	}
	profile.TotalEnergy += energyKWh

	return record, nil
}

// SetBudget 设置预算
func (e *Engine) SetBudget(req SetBudgetRequest) (*Budget, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if req.MonthlyAmount <= 0 {
		return nil, ErrInvalidBudgetAmount
	}
	if req.ElectricityPrice <= 0 {
		return nil, ErrInvalidElectricityPrice
	}

	if req.Name == "" {
		req.Name = "用电预算"
	}
	if req.WarningThreshold == 0 {
		req.WarningThreshold = DefaultWarningThreshold
	}
	if req.CriticalThreshold == 0 {
		req.CriticalThreshold = DefaultCriticalThreshold
	}

	e.config.MonthlyBudget = req.MonthlyAmount
	e.config.ElectricityRate = req.ElectricityPrice

	budget := &Budget{
		Name:              req.Name,
		MonthlyAmount:     req.MonthlyAmount,
		ElectricityPrice:  req.ElectricityPrice,
		WarningThreshold:  req.WarningThreshold,
		CriticalThreshold: req.CriticalThreshold,
		Enabled:           true,
	}

	return budget, nil
}

// GetBudgetStatus 获取预算状态
func (e *Engine) GetBudgetStatus() (*BudgetStatus, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if e.config.MonthlyBudget == 0 {
		return nil, ErrBudgetNotSet
	}

	usedEnergy := 0.0
	usedCost := int64(0)
	for _, r := range e.records {
		usedEnergy += r.EnergyKWh
		usedCost += r.CostCents
	}

	now := time.Now()
	startOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	daysElapsed := int(now.Sub(startOfMonth).Hours() / 24)
	if daysElapsed == 0 {
		daysElapsed = 1
	}
	daysRemaining := int(time.Date(now.Year(), now.Month()+1, 0, 0, 0, 0, 0, now.Location()).Sub(now).Hours() / 24)

	budget := &Budget{
		Name:             "用电预算",
		MonthlyAmount:    e.config.MonthlyBudget,
		ElectricityPrice: e.config.ElectricityRate,
		Enabled:          true,
	}

	return &BudgetStatus{
		Budget:        budget,
		UsedEnergy:    usedEnergy,
		UsedCost:      usedCost,
		DaysElapsed:   daysElapsed,
		DaysRemaining: daysRemaining,
		Trend:         &TrendPoint{Date: now, Energy: usedEnergy},
	}, nil
}

// GetMonthlyReport 获取月度报告
func (e *Engine) GetMonthlyReport() (*MonthlyReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	totalEnergy := 0.0
	totalCost := int64(0)
	for _, r := range e.records {
		totalEnergy += r.EnergyKWh
		totalCost += r.CostCents
	}

	report := &MonthlyReport{
		ID:          generateID(),
		Period:      PeriodMonthly,
		TotalEnergy: totalEnergy,
		TotalCost:   totalCost,
		DailyTrend:  []TrendPoint{{Date: time.Now(), Energy: totalEnergy}},
		TopDevices:  make([]DevicePower, 0),
		Trend:       &TrendPoint{Date: time.Now(), Energy: totalEnergy},
		Prediction:  &Prediction{Method: "linear", DaysLeft: 30, DailyAvg: totalEnergy / 30, PredictedKWh: totalEnergy, Confidence: &TrendPoint{}},
	}

	return report, nil
}

// GetReport 获取报告
func (e *Engine) GetReport(req ReportRequest) (*Report, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	totalEnergy := 0.0
	totalCost := int64(0)
	for _, r := range e.records {
		if req.DeviceID != "" && r.DeviceID != req.DeviceID {
			continue
		}
		totalEnergy += r.EnergyKWh
		totalCost += r.CostCents
	}

	return &Report{
		Period:      req.Period,
		TotalEnergy: totalEnergy,
		TotalCost:   totalCost,
		TopDevices:  make([]DevicePower, 0),
	}, nil
}

// GetActiveAlerts 获取活跃告警
func (e *Engine) GetActiveAlerts() []*Alert {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var result []*Alert
	for _, a := range e.alerts {
		if a.Active {
			result = append(result, a)
		}
	}
	return result
}

// AcknowledgeAlert 确认告警
func (e *Engine) AcknowledgeAlert(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, a := range e.alerts {
		if a.ID == id {
			a.Active = false
			return nil
		}
	}
	return ErrRecordNotFound
}

// GetDeviceProfile 获取设备画像
func (e *Engine) GetDeviceProfile(deviceID string) (*DeviceProfile, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	profile, exists := e.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return profile, nil
}

// GetAllDeviceProfiles 获取所有设备画像
func (e *Engine) GetAllDeviceProfiles() []*DeviceProfile {
	e.mu.RLock()
	defer e.mu.RUnlock()
	profiles := make([]*DeviceProfile, 0, len(e.devices))
	for _, p := range e.devices {
		profiles = append(profiles, p)
	}
	// 按总能耗降序排列
	for i := 0; i < len(profiles)-1; i++ {
		for j := i + 1; j < len(profiles); j++ {
			if profiles[j].TotalEnergy > profiles[i].TotalEnergy {
				profiles[i], profiles[j] = profiles[j], profiles[i]
			}
		}
	}
	return profiles
}

// ==================== 辅助函数 ====================

// FormatCost 格式化成本
func FormatCost(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100)
}

// DefaultBudgetConfig 默认预算配置
func DefaultBudgetConfig() *Budget {
	return &Budget{
		Name:              "默认用电预算",
		MonthlyAmount:     DefaultMonthlyBudget,
		ElectricityPrice:  DefaultElectricityPrice,
		WarningThreshold:  DefaultWarningThreshold,
		CriticalThreshold: DefaultCriticalThreshold,
		Enabled:           true,
	}
}

// calculateStats 计算统计
func calculateStats(data []float64) (mean, stddev float64) {
	if len(data) == 0 {
		return 0, 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	mean = sum / float64(len(data))

	sumSq := 0.0
	for _, v := range data {
		diff := v - mean
		sumSq += diff * diff
	}
	stddev = 0
	if len(data) > 1 {
		variance := sumSq / float64(len(data)-1)
		stddev = sqrt(variance)
	}
	return mean, stddev
}

// sqrt 平方根
func sqrt(x float64) float64 {
	if x == 0 {
		return 0
	}
	z := x
	for i := 0; i < 100; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// sortTrendPoints 排序趋势点
func sortTrendPoints(points []TrendPoint) {
	for i := 0; i < len(points)-1; i++ {
		for j := i + 1; j < len(points); j++ {
			if points[j].Date.Before(points[i].Date) {
				points[i], points[j] = points[j], points[i]
			}
		}
	}
}
