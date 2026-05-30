// Package hwhealthpredict 提供硬件健康预测功能
package hwhealthpredict

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// 设备类型常量
const (
	DeviceTypeHDD    = "HDD"    // 机械硬盘
	DeviceTypeSSD    = "SSD"    // 固态硬盘
	DeviceTypeCPU    = "CPU"    // 处理器
	DeviceTypeMemory = "Memory" // 内存
	DeviceTypePSU    = "PSU"    // 电源
	DeviceTypeGPU    = "GPU"    // 显卡
	DeviceTypeNIC    = "NIC"    // 网卡
	DeviceTypeRAID   = "RAID"   // RAID卡
)

// 健康状态常量
const (
	HealthStatusExcellent = "excellent" // 优秀 90-100
	HealthStatusGood      = "good"      // 良好 70-89
	HealthStatusFair      = "fair"      // 一般 50-69
	HealthStatusPoor      = "poor"      // 较差 30-49
	HealthStatusCritical  = "critical"  // 严重 0-29
)

// 告警级别常量
const (
	AlertLevelInfo     = "info"     // 信息
	AlertLevelWarning  = "warning"  // 警告
	AlertLevelCritical = "critical" // 严重
	AlertLevelFatal    = "fatal"    // 致命
)

// 默认阈值配置
const (
	DefaultWarningThreshold  = 50 // 健康分低于此值发出警告
	DefaultCriticalThreshold = 30 // 健康分低于此值发出严重告警
	DefaultFatalThreshold    = 10 // 健康分低于此值发出致命告警
)

// 错误定义
var (
	ErrDeviceNotFound    = errors.New("设备未找到")
	ErrDeviceExists      = errors.New("设备已存在")
	ErrInvalidDeviceType = errors.New("无效的设备类型")
	ErrInvalidSMARTData  = errors.New("无效的SMART数据")
	ErrNoHistoryData     = errors.New("没有历史数据")
)

// Device 硬件设备信息
type Device struct {
	ID           string            `json:"id"`            // 设备唯一标识
	Name         string            `json:"name"`          // 设备名称
	Type         string            `json:"type"`          // 设备类型
	Model        string            `json:"model"`         // 设备型号
	Serial       string            `json:"serial"`        // 序列号
	Manufacturer string            `json:"manufacturer"`  // 制造商
	Firmware     string            `json:"firmware"`      // 固件版本
	Capacity     int64             `json:"capacity"`      // 容量(字节)
	InstallDate  time.Time         `json:"install_date"`  // 安装日期
	Location     string            `json:"location"`      // 安装位置
	Tags         map[string]string `json:"tags"`          // 自定义标签
	CreatedAt    time.Time         `json:"created_at"`    // 创建时间
	UpdatedAt    time.Time         `json:"updated_at"`    // 更新时间
}

// SMARTData SMART监控数据
type SMARTData struct {
	DeviceID         string    `json:"device_id"`          // 设备ID
	Timestamp        time.Time `json:"timestamp"`          // 采集时间
	Temperature      int       `json:"temperature"`        // 温度(℃)
	PowerOnHours     int64     `json:"power_on_hours"`     // 通电时间(小时)
	PowerCycleCount  int64     `json:"power_cycle_count"`  // 通电次数
	BadSectors       int64     `json:"bad_sectors"`        // 坏扇区数
	ReallocatedSects int64     `json:"reallocated_sects"`  // 重映射扇区数
	PendingSects     int64     `json:"pending_sects"`      // 待映射扇区数
	UDMAErrors       int64     `json:"udma_errors"`        // UDMA CRC错误数
	ReadErrorRate    float64   `json:"read_error_rate"`    // 读取错误率
	WriteErrorRate   float64   `json:"write_error_rate"`   // 写入错误率
	SeekErrorRate    float64   `json:"seek_error_rate"`    // 寻道错误率(仅HDD)
	SpinRetryCount   int64     `json:"spin_retry_count"`   // 主轴起旋重试次数(仅HDD)
	SSDLifeLeft      int       `json:"ssd_life_left"`      // SSD剩余寿命百分比
	WearLeveling     int       `json:"wear_leveling"`      // 磨损均衡计数(仅SSD)
	TotalWritten     int64     `json:"total_written"`      // 总写入量(TB)
	TotalRead        int64     `json:"total_read"`         // 总读取量(TB)
	CPUUsage         float64   `json:"cpu_usage"`          // CPU使用率(仅CPU)
	CPUTemp          int       `json:"cpu_temp"`           // CPU温度(仅CPU)
	MemoryUsage      float64   `json:"memory_usage"`       // 内存使用率(仅Memory)
	MemoryErrors     int64     `json:"memory_errors"`      // 内存错误数(仅Memory)
	PSUVoltage       float64   `json:"psu_voltage"`        // 电源电压(仅PSU)
	PSUCurrent       float64   `json:"psu_current"`        // 电源电流(仅PSU)
	PSUWattage       float64   `json:"psu_wattage"`        // 电源功率(仅PSU)
	RawData          string    `json:"raw_data,omitempty"` // 原始数据JSON
}

// HealthScore 健康评分
type HealthScore struct {
	DeviceID    string             `json:"device_id"`    // 设备ID
	Score       int                `json:"score"`        // 总分(0-100)
	Status      string             `json:"status"`       // 健康状态
	Breakdown   map[string]float64 `json:"breakdown"`    // 分项评分
	Timestamp   time.Time          `json:"timestamp"`    // 评分时间
	Suggestions []string           `json:"suggestions"`  // 改进建议
	RiskFactors []string           `json:"risk_factors"` // 风险因素
}

// LifePrediction 寿命预测
type LifePrediction struct {
	DeviceID          string    `json:"device_id"`           // 设备ID
	PredictedLifeDays int       `json:"predicted_life_days"` // 预测剩余寿命(天)
	Confidence        float64   `json:"confidence"`          // 置信度(0-1)
	EstimatedFailDate time.Time `json:"estimated_fail_date"` // 预计故障日期
	Trend             string    `json:"trend"`               // 趋势(stable/improving/degrading)
	Factors           []string  `json:"factors"`             // 影响因素
	Timestamp         time.Time `json:"timestamp"`           // 预测时间
}

// Alert 告警信息
type Alert struct {
	ID           string    `json:"id"`           // 告警ID
	DeviceID     string    `json:"device_id"`    // 设备ID
	DeviceName   string    `json:"device_name"`  // 设备名称
	Level        string    `json:"level"`        // 告警级别
	Title        string    `json:"title"`        // 告警标题
	Message      string    `json:"message"`      // 告警详情
	Score        int       `json:"score"`        // 当前健康分
	Threshold    int       `json:"threshold"`    // 阈值
	Timestamp    time.Time `json:"timestamp"`    // 告警时间
	Acknowledged bool      `json:"acknowledged"` // 是否已确认
}

// MaintenancePlan 维护计划
type MaintenancePlan struct {
	DeviceID    string            `json:"device_id"`    // 设备ID
	DeviceName  string            `json:"device_name"`  // 设备名称
	DeviceType  string            `json:"device_type"`  // 设备类型
	HealthScore int               `json:"health_score"` // 当前健康分
	Items       []MaintenanceItem `json:"items"`        // 维护项目
	Replacement *ReplacementPlan  `json:"replacement"`  // 更换计划
	Priority    string            `json:"priority"`     // 优先级(high/medium/low)
	GeneratedAt time.Time         `json:"generated_at"` // 生成时间
}

// MaintenanceItem 维护项目
type MaintenanceItem struct {
	ID          string    `json:"id"`          // 项目ID
	Type        string    `json:"type"`        // 类型(inspection/clean/replace/upgrade)
	Title       string    `json:"title"`       // 标题
	Description string    `json:"description"` // 描述
	DueDate     time.Time `json:"due_date"`    // 建议执行日期
	Estimated   string    `json:"estimated"`   // 预计耗时
	Urgency     string    `json:"urgency"`     // 紧急程度
}

// ReplacementPlan 更换计划
type ReplacementPlan struct {
	Recommended   bool      `json:"recommended"`   // 是否建议更换
	Reason        string    `json:"reason"`        // 更换原因
	Deadline      time.Time `json:"deadline"`      // 建议更换日期
	Budget        float64   `json:"budget"`        // 预算估计
	Compatibility []string  `json:"compatibility"` // 兼容型号
}

// HealthHistory 历史记录查询结果
type HealthHistory struct {
	DeviceID string        `json:"device_id"` // 设备ID
	Records  []HealthScore `json:"records"`   // 历史记录
	Total    int           `json:"total"`     // 总数
	From     time.Time     `json:"from"`      // 起始时间
	To       time.Time     `json:"to"`        // 结束时间
}

// Config 硬件健康预测配置
type Config struct {
	WarningThreshold  int  // 警告阈值
	CriticalThreshold int  // 严重告警阈值
	FatalThreshold    int  // 致命告警阈值
	MaxHistoryDays    int  // 历史数据保留天数
	EnablePrediction  bool // 启用寿命预测
}

// HardwareHealthPredictor 硬件健康预测器
type HardwareHealthPredictor struct {
	devices      map[string]*Device
	smartData    map[string][]*SMARTData
	healthScores map[string][]*HealthScore
	alerts       []*Alert
	config       *Config
	alertHandler func(*Alert)
	mu           sync.RWMutex
}

// NewHardwareHealthPredictor 创建硬件健康预测器
func NewHardwareHealthPredictor(config *Config) *HardwareHealthPredictor {
	if config == nil {
		config = &Config{
			WarningThreshold:  DefaultWarningThreshold,
			CriticalThreshold: DefaultCriticalThreshold,
			FatalThreshold:    DefaultFatalThreshold,
			MaxHistoryDays:    365,
			EnablePrediction:  true,
		}
	}
	return &HardwareHealthPredictor{
		devices:      make(map[string]*Device),
		smartData:    make(map[string][]*SMARTData),
		healthScores: make(map[string][]*HealthScore),
		alerts:       make([]*Alert, 0),
		config:       config,
	}
}

// SetAlertHandler 设置告警回调函数
func (p *HardwareHealthPredictor) SetAlertHandler(handler func(*Alert)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.alertHandler = handler
}

// RegisterDevice 注册硬件设备
func (p *HardwareHealthPredictor) RegisterDevice(device *Device) error {
	if device == nil {
		return errors.New("设备信息不能为空")
	}
	if device.ID == "" {
		return errors.New("设备ID不能为空")
	}
	if !isValidDeviceType(device.Type) {
		return ErrInvalidDeviceType
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.devices[device.ID]; exists {
		return ErrDeviceExists
	}

	now := time.Now()
	device.CreatedAt = now
	device.UpdatedAt = now
	if device.Tags == nil {
		device.Tags = make(map[string]string)
	}

	p.devices[device.ID] = device
	return nil
}

// UnregisterDevice 注销硬件设备
func (p *HardwareHealthPredictor) UnregisterDevice(deviceID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.devices[deviceID]; !exists {
		return ErrDeviceNotFound
	}

	delete(p.devices, deviceID)
	delete(p.smartData, deviceID)
	delete(p.healthScores, deviceID)
	return nil
}

// GetDevice 获取设备信息
func (p *HardwareHealthPredictor) GetDevice(deviceID string) (*Device, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	device, exists := p.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}
	return device, nil
}

// ListDevices 列出所有设备
func (p *HardwareHealthPredictor) ListDevices() []*Device {
	p.mu.RLock()
	defer p.mu.RUnlock()

	devices := make([]*Device, 0, len(p.devices))
	for _, d := range p.devices {
		devices = append(devices, d)
	}
	return devices
}

// ListDevicesByType 按类型列出设备
func (p *HardwareHealthPredictor) ListDevicesByType(deviceType string) []*Device {
	p.mu.RLock()
	defer p.mu.RUnlock()

	devices := make([]*Device, 0)
	for _, d := range p.devices {
		if d.Type == deviceType {
			devices = append(devices, d)
		}
	}
	return devices
}

// RecordSMARTData 记录SMART数据
func (p *HardwareHealthPredictor) RecordSMARTData(data *SMARTData) error {
	if data == nil {
		return ErrInvalidSMARTData
	}
	if data.DeviceID == "" {
		return errors.New("设备ID不能为空")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.devices[data.DeviceID]; !exists {
		return ErrDeviceNotFound
	}

	if data.Timestamp.IsZero() {
		data.Timestamp = time.Now()
	}

	p.smartData[data.DeviceID] = append(p.smartData[data.DeviceID], data)

	// 自动计算健康分并检查告警
	score := p.calculateHealthScore(data)
	p.healthScores[data.DeviceID] = append(p.healthScores[data.DeviceID], score)

	// 检查是否需要告警
	p.checkAndCreateAlert(data.DeviceID, score)

	return nil
}

// GetLatestSMARTData 获取最新SMART数据
func (p *HardwareHealthPredictor) GetLatestSMARTData(deviceID string) (*SMARTData, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	dataList, exists := p.smartData[deviceID]
	if !exists || len(dataList) == 0 {
		return nil, ErrNoHistoryData
	}
	return dataList[len(dataList)-1], nil
}

// GetSMARTHistory 获取SMART历史数据
func (p *HardwareHealthPredictor) GetSMARTHistory(deviceID string, from, to time.Time) ([]*SMARTData, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	dataList, exists := p.smartData[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}

	result := make([]*SMARTData, 0)
	for _, data := range dataList {
		if (from.IsZero() || !data.Timestamp.Before(from)) &&
			(to.IsZero() || !data.Timestamp.After(to)) {
			result = append(result, data)
		}
	}

	if len(result) == 0 {
		return nil, ErrNoHistoryData
	}
	return result, nil
}

// CalculateHealthScore 计算设备健康评分
func (p *HardwareHealthPredictor) CalculateHealthScore(deviceID string) (*HealthScore, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if _, exists := p.devices[deviceID]; !exists {
		return nil, ErrDeviceNotFound
	}

	dataList, exists := p.smartData[deviceID]
	if !exists || len(dataList) == 0 {
		return nil, ErrNoHistoryData
	}

	latest := dataList[len(dataList)-1]
	return p.calculateHealthScore(latest), nil
}

// GetHealthHistory 获取健康评分历史
func (p *HardwareHealthPredictor) GetHealthHistory(deviceID string, from, to time.Time) (*HealthHistory, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if _, exists := p.devices[deviceID]; !exists {
		return nil, ErrDeviceNotFound
	}

	scores, exists := p.healthScores[deviceID]
	if !exists || len(scores) == 0 {
		return nil, ErrNoHistoryData
	}

	result := make([]HealthScore, 0)
	for _, score := range scores {
		if (from.IsZero() || !score.Timestamp.Before(from)) &&
			(to.IsZero() || !score.Timestamp.After(to)) {
			result = append(result, *score)
		}
	}

	if len(result) == 0 {
		return nil, ErrNoHistoryData
	}

	return &HealthHistory{
		DeviceID: deviceID,
		Records:  result,
		Total:    len(result),
		From:     from,
		To:       to,
	}, nil
}

// PredictLifespan 预测设备剩余寿命
func (p *HardwareHealthPredictor) PredictLifespan(deviceID string) (*LifePrediction, error) {
	if !p.config.EnablePrediction {
		return nil, errors.New("寿命预测功能未启用")
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	if _, exists := p.devices[deviceID]; !exists {
		return nil, ErrDeviceNotFound
	}

	dataList, exists := p.smartData[deviceID]
	if !exists || len(dataList) < 2 {
		return nil, ErrNoHistoryData
	}

	return p.predictLifespan(deviceID, dataList), nil
}

// GetAlerts 获取告警列表
func (p *HardwareHealthPredictor) GetAlerts(deviceID string, unacknowledgedOnly bool) []*Alert {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make([]*Alert, 0)
	for _, alert := range p.alerts {
		if deviceID != "" && alert.DeviceID != deviceID {
			continue
		}
		if unacknowledgedOnly && alert.Acknowledged {
			continue
		}
		result = append(result, alert)
	}
	return result
}

// AcknowledgeAlert 确认告警
func (p *HardwareHealthPredictor) AcknowledgeAlert(alertID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, alert := range p.alerts {
		if alert.ID == alertID {
			alert.Acknowledged = true
			return nil
		}
	}
	return errors.New("告警未找到")
}

// GenerateMaintenancePlan 生成维护计划
func (p *HardwareHealthPredictor) GenerateMaintenancePlan(deviceID string) (*MaintenancePlan, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	device, exists := p.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}

	dataList, exists := p.smartData[deviceID]
	if !exists || len(dataList) == 0 {
		return nil, ErrNoHistoryData
	}

	latest := dataList[len(dataList)-1]
	score := p.calculateHealthScore(latest)

	return p.generateMaintenancePlan(device, latest, score), nil
}

// ExportData 导出设备数据为JSON
func (p *HardwareHealthPredictor) ExportData(deviceID string) ([]byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	device, exists := p.devices[deviceID]
	if !exists {
		return nil, ErrDeviceNotFound
	}

	export := struct {
		Device      *Device        `json:"device"`
		SMARTData   []*SMARTData   `json:"smart_data"`
		HealthScore []*HealthScore `json:"health_scores"`
		Alerts      []*Alert       `json:"alerts"`
	}{
		Device:      device,
		SMARTData:   p.smartData[deviceID],
		HealthScore: p.healthScores[deviceID],
	}

	for _, alert := range p.alerts {
		if alert.DeviceID == deviceID {
			export.Alerts = append(export.Alerts, alert)
		}
	}

	return json.MarshalIndent(export, "", "  ")
}

// GetSummary 获取设备健康摘要
func (p *HardwareHealthPredictor) GetSummary() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	totalDevices := len(p.devices)
	healthyCount := 0
	warningCount := 0
	criticalCount := 0

	deviceSummaries := make([]map[string]interface{}, 0)

	for _, device := range p.devices {
		summary := map[string]interface{}{
			"id":   device.ID,
			"name": device.Name,
			"type": device.Type,
		}

		scores, exists := p.healthScores[device.ID]
		if exists && len(scores) > 0 {
			latest := scores[len(scores)-1]
			summary["health_score"] = latest.Score
			summary["status"] = latest.Status

			switch {
			case latest.Score >= 70:
				healthyCount++
			case latest.Score >= 50:
				warningCount++
			default:
				criticalCount++
			}
		} else {
			summary["health_score"] = nil
			summary["status"] = "unknown"
		}

		deviceSummaries = append(deviceSummaries, summary)
	}

	return map[string]interface{}{
		"total_devices":    totalDevices,
		"healthy_devices":  healthyCount,
		"warning_devices":  warningCount,
		"critical_devices": criticalCount,
		"devices":          deviceSummaries,
		"generated_at":     time.Now(),
	}
}

// ========== 内部方法 ==========

// isValidDeviceType 验证设备类型
func isValidDeviceType(deviceType string) bool {
	validTypes := []string{
		DeviceTypeHDD, DeviceTypeSSD, DeviceTypeCPU,
		DeviceTypeMemory, DeviceTypePSU, DeviceTypeGPU,
		DeviceTypeNIC, DeviceTypeRAID,
	}
	for _, t := range validTypes {
		if t == deviceType {
			return true
		}
	}
	return false
}

// calculateHealthScore 计算健康评分（内部方法，需要持有锁）
func (p *HardwareHealthPredictor) calculateHealthScore(data *SMARTData) *HealthScore {
	score := &HealthScore{
		DeviceID:    data.DeviceID,
		Breakdown:   make(map[string]float64),
		Timestamp:   time.Now(),
		Suggestions: make([]string, 0),
		RiskFactors: make([]string, 0),
	}

	device := p.devices[data.DeviceID]
	if device == nil {
		return score
	}

	switch device.Type {
	case DeviceTypeHDD:
		p.calculateDiskHealthScore(data, score, true)
	case DeviceTypeSSD:
		p.calculateDiskHealthScore(data, score, false)
	case DeviceTypeCPU:
		p.calculateCPUHealthScore(data, score)
	case DeviceTypeMemory:
		p.calculateMemoryHealthScore(data, score)
	case DeviceTypePSU:
		p.calculatePSUHealthScore(data, score)
	default:
		p.calculateGenericHealthScore(data, score)
	}

	// 确保分数在0-100范围内
	score.Score = int(math.Max(0, math.Min(100, float64(score.Score))))
	score.Status = getHealthStatus(score.Score)

	return score
}

// calculateDiskHealthScore 计算磁盘健康评分
func (p *HardwareHealthPredictor) calculateDiskHealthScore(data *SMARTData, score *HealthScore, isHDD bool) {
	totalScore := 0.0
	weightSum := 0.0

	// 1. 温度评分 (权重: 20%)
	tempScore := 100.0
	if data.Temperature > 60 {
		tempScore = math.Max(0, 100-float64(data.Temperature-60)*5)
		score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("温度过高: %d℃", data.Temperature))
	} else if data.Temperature > 50 {
		tempScore = 80.0
		score.Suggestions = append(score.Suggestions, "建议改善散热条件")
	}
	score.Breakdown["temperature"] = tempScore
	totalScore += tempScore * 0.2
	weightSum += 0.2

	// 2. 通电时间评分 (权重: 15%)
	hoursScore := 100.0
	if data.PowerOnHours > 50000 {
		hoursScore = math.Max(0, 100-float64(data.PowerOnHours-50000)/1000)
		score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("通电时间过长: %d小时", data.PowerOnHours))
	} else if data.PowerOnHours > 30000 {
		hoursScore = 70.0
	}
	score.Breakdown["power_on_hours"] = hoursScore
	totalScore += hoursScore * 0.15
	weightSum += 0.15

	// 3. 坏扇区评分 (权重: 25%)
	badSectScore := 100.0
	if data.BadSectors > 0 || data.ReallocatedSects > 0 || data.PendingSects > 0 {
		totalBad := data.BadSectors + data.ReallocatedSects + data.PendingSects
		badSectScore = math.Max(0, 100-float64(totalBad)*2)
		if totalBad > 10 {
			score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("存在坏扇区: %d个", totalBad))
		}
		score.Suggestions = append(score.Suggestions, "建议备份数据并监控坏扇区增长趋势")
	}
	score.Breakdown["bad_sectors"] = badSectScore
	totalScore += badSectScore * 0.25
	weightSum += 0.25

	// 4. 错误率评分 (权重: 20%)
	errorRateScore := 100.0
	maxErrorRate := math.Max(data.ReadErrorRate, data.WriteErrorRate)
	if isHDD {
		maxErrorRate = math.Max(maxErrorRate, data.SeekErrorRate)
	}
	if maxErrorRate > 0 {
		errorRateScore = math.Max(0, 100-maxErrorRate*100)
		if maxErrorRate > 0.1 {
			score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("读写错误率偏高: %.2f%%", maxErrorRate*100))
		}
	}
	score.Breakdown["error_rate"] = errorRateScore
	totalScore += errorRateScore * 0.2
	weightSum += 0.2

	// 5. SSD特有指标 (权重: 20%)
	if !isHDD && data.SSDLifeLeft >= 0 {
		lifeLeftScore := float64(data.SSDLifeLeft)
		if data.SSDLifeLeft < 20 {
			score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("SSD剩余寿命不足: %d%%", data.SSDLifeLeft))
		}
		score.Breakdown["ssd_life_left"] = lifeLeftScore
		totalScore += lifeLeftScore * 0.2
		weightSum += 0.2
	}

	// 6. UDMA错误 (额外扣分)
	if data.UDMAErrors > 0 {
		penalty := math.Min(20, float64(data.UDMAErrors)*5)
		totalScore -= penalty
		score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("UDMA CRC错误: %d次", data.UDMAErrors))
	}

	// 7. HDD特有: 主轴起旋重试
	if isHDD && data.SpinRetryCount > 0 {
		penalty := math.Min(30, float64(data.SpinRetryCount)*10)
		totalScore -= penalty
		score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("主轴起旋重试: %d次", data.SpinRetryCount))
	}

	if weightSum > 0 {
		score.Score = int(totalScore / weightSum)
	}
}

// calculateCPUHealthScore 计算CPU健康评分
func (p *HardwareHealthPredictor) calculateCPUHealthScore(data *SMARTData, score *HealthScore) {
	totalScore := 0.0

	// 1. 温度评分 (权重: 40%)
	tempScore := 100.0
	if data.CPUTemp > 90 {
		tempScore = math.Max(0, 100-float64(data.CPUTemp-90)*10)
		score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("CPU温度过高: %d℃", data.CPUTemp))
	} else if data.CPUTemp > 80 {
		tempScore = 60.0
		score.Suggestions = append(score.Suggestions, "建议检查CPU散热器")
	}
	score.Breakdown["cpu_temperature"] = tempScore
	totalScore += tempScore * 0.4

	// 2. 使用率评分 (权重: 30%)
	usageScore := 100.0
	if data.CPUUsage > 95 {
		usageScore = 50.0
		score.Suggestions = append(score.Suggestions, "CPU使用率持续过高，建议优化负载")
	} else if data.CPUUsage > 80 {
		usageScore = 75.0
	}
	score.Breakdown["cpu_usage"] = usageScore
	totalScore += usageScore * 0.3

	// 3. 通电时间评分 (权重: 30%)
	hoursScore := 100.0
	if data.PowerOnHours > 60000 {
		hoursScore = 70.0
	}
	score.Breakdown["power_on_hours"] = hoursScore
	totalScore += hoursScore * 0.3

	score.Score = int(totalScore)
}

// calculateMemoryHealthScore 计算内存健康评分
func (p *HardwareHealthPredictor) calculateMemoryHealthScore(data *SMARTData, score *HealthScore) {
	totalScore := 0.0

	// 1. 错误率评分 (权重: 50%)
	errorScore := 100.0
	if data.MemoryErrors > 0 {
		errorScore = math.Max(0, 100-float64(data.MemoryErrors)*10)
		if data.MemoryErrors > 5 {
			score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("内存错误数过多: %d次", data.MemoryErrors))
			score.Suggestions = append(score.Suggestions, "建议运行内存诊断测试")
		}
	}
	score.Breakdown["memory_errors"] = errorScore
	totalScore += errorScore * 0.5

	// 2. 使用率评分 (权重: 30%)
	usageScore := 100.0
	if data.MemoryUsage > 95 {
		usageScore = 60.0
		score.Suggestions = append(score.Suggestions, "内存使用率过高，建议增加内存")
	} else if data.MemoryUsage > 85 {
		usageScore = 80.0
	}
	score.Breakdown["memory_usage"] = usageScore
	totalScore += usageScore * 0.3

	// 3. 温度评分 (权重: 20%)
	tempScore := 100.0
	if data.Temperature > 85 {
		tempScore = math.Max(0, 100-float64(data.Temperature-85)*5)
		score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("内存温度过高: %d℃", data.Temperature))
	}
	score.Breakdown["temperature"] = tempScore
	totalScore += tempScore * 0.2

	score.Score = int(totalScore)
}

// calculatePSUHealthScore 计算电源健康评分
func (p *HardwareHealthPredictor) calculatePSUHealthScore(data *SMARTData, score *HealthScore) {
	totalScore := 0.0

	// 1. 电压稳定性评分 (权重: 40%)
	voltageScore := 100.0
	expectedVoltage := 12.0
	if data.PSUVoltage > 0 {
		deviation := math.Abs(data.PSUVoltage-expectedVoltage) / expectedVoltage * 100
		if deviation > 5 {
			voltageScore = math.Max(0, 100-deviation*10)
			score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("电压偏差过大: %.1f%%", deviation))
		} else if deviation > 3 {
			voltageScore = 80.0
			score.Suggestions = append(score.Suggestions, "电压略有偏差，建议监控")
		}
	}
	score.Breakdown["voltage_stability"] = voltageScore
	totalScore += voltageScore * 0.4

	// 2. 温度评分 (权重: 30%)
	tempScore := 100.0
	if data.Temperature > 70 {
		tempScore = math.Max(0, 100-float64(data.Temperature-70)*5)
		score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("电源温度过高: %d℃", data.Temperature))
		score.Suggestions = append(score.Suggestions, "建议检查电源散热")
	}
	score.Breakdown["temperature"] = tempScore
	totalScore += tempScore * 0.3

	// 3. 通电时间评分 (权重: 30%)
	hoursScore := 100.0
	if data.PowerOnHours > 50000 {
		hoursScore = 70.0
		score.Suggestions = append(score.Suggestions, "电源使用时间较长，建议考虑更换")
	}
	score.Breakdown["power_on_hours"] = hoursScore
	totalScore += hoursScore * 0.3

	score.Score = int(totalScore)
}

// calculateGenericHealthScore 计算通用设备健康评分
func (p *HardwareHealthPredictor) calculateGenericHealthScore(data *SMARTData, score *HealthScore) {
	totalScore := 100.0

	if data.Temperature > 80 {
		totalScore -= float64(data.Temperature-80) * 2
		score.RiskFactors = append(score.RiskFactors, fmt.Sprintf("温度过高: %d℃", data.Temperature))
	}

	if data.PowerOnHours > 50000 {
		totalScore -= 10
	}

	score.Breakdown["overall"] = totalScore
	score.Score = int(totalScore)
}

// getHealthStatus 获取健康状态文本
func getHealthStatus(score int) string {
	switch {
	case score >= 90:
		return HealthStatusExcellent
	case score >= 70:
		return HealthStatusGood
	case score >= 50:
		return HealthStatusFair
	case score >= 30:
		return HealthStatusPoor
	default:
		return HealthStatusCritical
	}
}

// checkAndCreateAlert 检查并创建告警（内部方法，需要持有锁）
func (p *HardwareHealthPredictor) checkAndCreateAlert(deviceID string, score *HealthScore) {
	device := p.devices[deviceID]
	if device == nil {
		return
	}

	var level string
	var threshold int

	switch {
	case score.Score <= p.config.FatalThreshold:
		level = AlertLevelFatal
		threshold = p.config.FatalThreshold
	case score.Score <= p.config.CriticalThreshold:
		level = AlertLevelCritical
		threshold = p.config.CriticalThreshold
	case score.Score <= p.config.WarningThreshold:
		level = AlertLevelWarning
		threshold = p.config.WarningThreshold
	default:
		return
	}

	alert := &Alert{
		ID:         fmt.Sprintf("alert-%s-%d", deviceID, time.Now().UnixNano()),
		DeviceID:   deviceID,
		DeviceName: device.Name,
		Level:      level,
		Title:      fmt.Sprintf("设备健康告警: %s", device.Name),
		Message:    fmt.Sprintf("设备 %s 健康分 %d 低于阈值 %d", device.Name, score.Score, threshold),
		Score:      score.Score,
		Threshold:  threshold,
		Timestamp:  time.Now(),
	}

	p.alerts = append(p.alerts, alert)

	// 调用告警回调
	if p.alertHandler != nil {
		p.alertHandler(alert)
	}
}

// predictLifespan 预测设备寿命（内部方法，需要持有锁）
func (p *HardwareHealthPredictor) predictLifespan(deviceID string, dataList []*SMARTData) *LifePrediction {
	device := p.devices[deviceID]
	prediction := &LifePrediction{
		DeviceID:  deviceID,
		Timestamp: time.Now(),
		Factors:   make([]string, 0),
	}

	// 计算健康分趋势
	scores := make([]float64, 0)
	for _, data := range dataList {
		score := p.calculateHealthScore(data)
		scores = append(scores, float64(score.Score))
	}

	// 简单线性回归预测
	if len(scores) >= 2 {
		n := float64(len(scores))
		var sumX, sumY, sumXY, sumX2 float64
		for i, y := range scores {
			x := float64(i)
			sumX += x
			sumY += y
			sumXY += x * y
			sumX2 += x * x
		}

		slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
		intercept := (sumY - slope*sumX) / n

		// 计算趋势
		if slope > 0.5 {
			prediction.Trend = "improving"
		} else if slope < -0.5 {
			prediction.Trend = "degrading"
			prediction.Factors = append(prediction.Factors, "健康分呈下降趋势")
		} else {
			prediction.Trend = "stable"
		}

		// 预测何时降到0分
		if slope < 0 {
			daysToZero := -intercept / slope
			if daysToZero > 0 {
				prediction.PredictedLifeDays = int(daysToZero)
				prediction.EstimatedFailDate = time.Now().AddDate(0, 0, int(daysToZero))
				prediction.Confidence = math.Min(0.9, math.Max(0.3, 1-math.Abs(slope)/10))
			} else {
				prediction.PredictedLifeDays = 3650 // 默认10年
				prediction.EstimatedFailDate = time.Now().AddDate(10, 0, 0)
				prediction.Confidence = 0.3
			}
		} else {
			prediction.PredictedLifeDays = 3650
			prediction.EstimatedFailDate = time.Now().AddDate(10, 0, 0)
			prediction.Confidence = 0.5
		}
	}

	// 基于设备类型添加影响因素
	if device != nil {
		switch device.Type {
		case DeviceTypeHDD, DeviceTypeSSD:
			if dataList[len(dataList)-1].Temperature > 50 {
				prediction.Factors = append(prediction.Factors, "温度偏高影响寿命")
			}
			if dataList[len(dataList)-1].BadSectors > 0 {
				prediction.Factors = append(prediction.Factors, "存在坏扇区")
			}
		case DeviceTypeCPU:
			if dataList[len(dataList)-1].CPUTemp > 80 {
				prediction.Factors = append(prediction.Factors, "CPU温度偏高")
			}
		case DeviceTypeMemory:
			if dataList[len(dataList)-1].MemoryErrors > 0 {
				prediction.Factors = append(prediction.Factors, "存在内存错误")
			}
		}
	}

	// 确保预测值合理
	if prediction.PredictedLifeDays <= 0 {
		prediction.PredictedLifeDays = 30
		prediction.EstimatedFailDate = time.Now().AddDate(0, 0, 30)
		prediction.Confidence = 0.2
	}

	return prediction
}

// generateMaintenancePlan 生成维护计划（内部方法，需要持有锁）
func (p *HardwareHealthPredictor) generateMaintenancePlan(device *Device, data *SMARTData, score *HealthScore) *MaintenancePlan {
	plan := &MaintenancePlan{
		DeviceID:    device.ID,
		DeviceName:  device.Name,
		DeviceType:  device.Type,
		HealthScore: score.Score,
		Items:       make([]MaintenanceItem, 0),
		GeneratedAt: time.Now(),
	}

	// 根据健康分确定优先级
	switch {
	case score.Score < 30:
		plan.Priority = "high"
	case score.Score < 60:
		plan.Priority = "medium"
	default:
		plan.Priority = "low"
	}

	itemID := 0
	makeItem := func(itemType, title, desc, urgency string, dueDays int, estimated string) MaintenanceItem {
		itemID++
		return MaintenanceItem{
			ID:          fmt.Sprintf("maint-%s-%d", device.ID, itemID),
			Type:        itemType,
			Title:       title,
			Description: desc,
			DueDate:     time.Now().AddDate(0, 0, dueDays),
			Estimated:   estimated,
			Urgency:     urgency,
		}
	}

	// 基于设备类型生成维护建议
	switch device.Type {
	case DeviceTypeHDD, DeviceTypeSSD:
		// 温度相关
		if data.Temperature > 50 {
			plan.Items = append(plan.Items, makeItem(
				"inspection", "检查散热系统",
				fmt.Sprintf("当前温度 %d℃，建议检查散热风扇和通风环境", data.Temperature),
				"high", 7, "30分钟",
			))
		}

		// 坏扇区相关
		totalBad := data.BadSectors + data.ReallocatedSects + data.PendingSects
		if totalBad > 0 {
			urgency := "medium"
			if totalBad > 100 {
				urgency = "high"
			}
			plan.Items = append(plan.Items, makeItem(
				"inspection", "磁盘完整性检查",
				fmt.Sprintf("发现 %d 个坏扇区/重映射扇区，建议运行磁盘检查", totalBad),
				urgency, 3, "2小时",
			))
		}

		// SSD寿命
		if device.Type == DeviceTypeSSD && data.SSDLifeLeft < 50 {
			plan.Items = append(plan.Items, makeItem(
				"replace", "SSD更换计划",
				fmt.Sprintf("SSD剩余寿命 %d%%，建议规划更换", data.SSDLifeLeft),
				"high", 30, "需要停机维护",
			))
		}

		// 定期备份建议
		plan.Items = append(plan.Items, makeItem(
			"inspection", "定期数据备份",
			"建议定期备份重要数据",
			"low", 30, "根据数据量而定",
		))

	case DeviceTypeCPU:
		if data.CPUTemp > 70 {
			plan.Items = append(plan.Items, makeItem(
				"clean", "清理CPU散热器",
				fmt.Sprintf("CPU温度 %d℃，建议清理散热器灰尘并更换硅脂", data.CPUTemp),
				"medium", 14, "1小时",
			))
		}
		if data.CPUUsage > 90 {
			plan.Items = append(plan.Items, makeItem(
				"inspection", "性能优化",
				"CPU使用率持续过高，建议检查运行进程",
				"medium", 7, "30分钟",
			))
		}

	case DeviceTypeMemory:
		if data.MemoryErrors > 0 {
			plan.Items = append(plan.Items, makeItem(
				"inspection", "内存诊断",
				fmt.Sprintf("检测到 %d 次内存错误，建议运行内存诊断", data.MemoryErrors),
				"high", 3, "2小时",
			))
		}
		if data.MemoryUsage > 90 {
			plan.Items = append(plan.Items, makeItem(
				"upgrade", "内存扩容",
				fmt.Sprintf("内存使用率 %.1f%%，建议增加内存", data.MemoryUsage),
				"medium", 30, "需要停机维护",
			))
		}

	case DeviceTypePSU:
		if data.Temperature > 60 {
			plan.Items = append(plan.Items, makeItem(
				"inspection", "电源散热检查",
				fmt.Sprintf("电源温度 %d℃，建议检查电源风扇", data.Temperature),
				"high", 7, "30分钟",
			))
		}
		if data.PowerOnHours > 50000 {
			plan.Items = append(plan.Items, makeItem(
				"replace", "电源更换评估",
				"电源使用时间较长，建议评估是否需要更换",
				"medium", 30, "需要停机维护",
			))
		}
	}

	// 通电时间通用建议
	if data.PowerOnHours > 40000 {
		plan.Items = append(plan.Items, makeItem(
			"inspection", "设备老化检查",
			fmt.Sprintf("设备已运行 %d 小时，建议进行全面检查", data.PowerOnHours),
			"low", 30, "1小时",
		))
	}

	// 排序：按紧急程度
	urgencyOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
	sort.Slice(plan.Items, func(i, j int) bool {
		return urgencyOrder[plan.Items[i].Urgency] < urgencyOrder[plan.Items[j].Urgency]
	})

	// 生成更换计划
	plan.Replacement = p.generateReplacementPlan(device, data, score)

	return plan
}

// generateReplacementPlan 生成更换计划（内部方法，需要持有锁）
func (p *HardwareHealthPredictor) generateReplacementPlan(device *Device, data *SMARTData, score *HealthScore) *ReplacementPlan {
	rp := &ReplacementPlan{
		Compatibility: make([]string, 0),
	}

	// 基于健康分判断是否需要更换
	if score.Score < 30 {
		rp.Recommended = true
		rp.Reason = fmt.Sprintf("设备健康分 %d 过低，存在故障风险", score.Score)
		rp.Deadline = time.Now().AddDate(0, 0, 30)
	} else if score.Score < 50 {
		rp.Recommended = true
		rp.Reason = fmt.Sprintf("设备健康分 %d，建议提前规划更换", score.Score)
		rp.Deadline = time.Now().AddDate(0, 3, 0)
	}

	// 基于设备类型设置预算和兼容型号
	switch device.Type {
	case DeviceTypeHDD:
		rp.Budget = 500
		rp.Compatibility = []string{"HDD 3.5寸 SATA", "HDD 2.5寸 SATA"}
	case DeviceTypeSSD:
		rp.Budget = 800
		rp.Compatibility = []string{"SATA SSD", "NVMe SSD"}
	case DeviceTypeCPU:
		rp.Budget = 2000
		rp.Compatibility = []string{"同代CPU", "兼容主板型号"}
	case DeviceTypeMemory:
		rp.Budget = 600
		rp.Compatibility = []string{"同频率DDR", "同代内存"}
	case DeviceTypePSU:
		rp.Budget = 800
		rp.Compatibility = []string{"同功率ATX电源", "模组电源"}
	default:
		rp.Budget = 1000
	}

	return rp
}

// unused import guard
var _ = strings.Contains
var _ = sort.Slice
