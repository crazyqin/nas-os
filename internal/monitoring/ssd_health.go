// Package monitoring 提供 SSD 健康监控功能
// Version: v1.0.0 - SSD 磨损监控
package monitoring

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SSDHealth SSD 健康状态.
type SSDHealth struct {
	Device             string                `json:"device"`
	Model              string                `json:"model"`
	Serial             string                `json:"serial"`
	Firmware           string                `json:"firmware"`
	Size               uint64                `json:"size"`                      // 字节
	InterfaceType      string                `json:"interfaceType"`             // NVMe, SATA, SAS
	HealthPercent      float64               `json:"healthPercent"`             // 健康百分比 (0-100)
	LifeUsedPercent    float64               `json:"lifeUsedPercent"`           // 已用寿命百分比
	TotalWrites        uint64                `json:"totalWrites"`               // 总写入量 (字节)
	WriteAmplification float64               `json:"writeAmplification"`        // 写放大因子
	Temperature        int                   `json:"temperature"`               // 摄氏度
	AvailableSpare     float64               `json:"availableSpare"`            // 可用备用空间百分比
	MediaErrors        uint64                `json:"mediaErrors"`               // 媒体错误数
	PowerOnHours       uint64                `json:"powerOnHours"`              // 通电时间 (小时)
	PowerCycles        uint64                `json:"powerCycles"`               // 通电次数
	AlertLevel         AlertLevel            `json:"alertLevel"`                // 告警级别
	AlertMessage       string                `json:"alertMessage"`              // 告警消息
	Status             SSDStatus             `json:"status"`                    // 状态
	LastCheck          time.Time             `json:"lastCheck"`                 // 最后检查时间
	PredictedLife      *PredictedLife        `json:"predictedLife,omitempty"`   // 预测寿命
	SMARTAttributes    map[string]*SMARTAttr `json:"smartAttributes,omitempty"` // 原始 SMART 属性
}

// SSDStatus SSD 状态.
type SSDStatus string

// SSD 状态常量.
const (
	SSDStatusHealthy   SSDStatus = "healthy"   // 健康
	SSDStatusWarning   SSDStatus = "warning"   // 警告
	SSDStatusCritical  SSDStatus = "critical"  // 严重
	SSDStatusEmergency SSDStatus = "emergency" // 紧急
	SSDStatusUnknown   SSDStatus = "unknown"   // 未知
	SSDStatusOffline   SSDStatus = "offline"   // 离线
)

// AlertLevel 告警级别.
type AlertLevel string

// 告警级别常量.
const (
	AlertLevelNone      AlertLevel = "none"      // 无告警
	AlertLevelWarning   AlertLevel = "warning"   // 80% 预警
	AlertLevelCritical  AlertLevel = "critical"  // 90% 严重
	AlertLevelEmergency AlertLevel = "emergency" // 95% 紧急
)

// PredictedLife 预测寿命.
type PredictedLife struct {
	RemainingDays    int       `json:"remainingDays"`    // 预计剩余天数
	EstimatedEndDate time.Time `json:"estimatedEndDate"` // 预计失效日期
	WriteRatePerDay  uint64    `json:"writeRatePerDay"`  // 日均写入量 (字节)
	Confidence       float64   `json:"confidence"`       // 置信度 (0-1)
	Method           string    `json:"method"`           // 预测方法
}

// SMARTAttr SMART 属性.
type SMARTAttr struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Value       uint   `json:"value"`       // 标准化值
	Raw         uint64 `json:"raw"`         // 原始值
	Threshold   uint   `json:"threshold"`   // 阈值
	Description string `json:"description"` // 描述
}

// SSDHealthMonitor SSD 健康监控器.
type SSDHealthMonitor struct {
	ssds           map[string]*SSDHealth
	history        map[string][]*SSDHealthHistory
	alertCallbacks []func(*SSDHealthAlert)
	mu             sync.RWMutex
	config         *SSDMonitorConfig
	stopChan       chan struct{}
}

// SSDMonitorConfig 监控配置.
type SSDMonitorConfig struct {
	CheckInterval      time.Duration `json:"checkInterval"`      // 检查间隔
	HistoryRetention   int           `json:"historyRetention"`   // 历史保留天数
	EnablePrediction   bool          `json:"enablePrediction"`   // 启用预测
	WarningThreshold   float64       `json:"warningThreshold"`   // 警告阈值 (默认 80%)
	CriticalThreshold  float64       `json:"criticalThreshold"`  // 严重阈值 (默认 90%)
	EmergencyThreshold float64       `json:"emergencyThreshold"` // 紧急阈值 (默认 95%)
}

// DefaultSSDMonitorConfig 默认配置.
var DefaultSSDMonitorConfig = &SSDMonitorConfig{
	CheckInterval:      30 * time.Minute,
	HistoryRetention:   30,
	EnablePrediction:   true,
	WarningThreshold:   80, // 80% 寿命已用
	CriticalThreshold:  90, // 90% 寿命已用
	EmergencyThreshold: 95, // 95% 寿命已用
}

// SSDHealthHistory 历史数据.
type SSDHealthHistory struct {
	Timestamp       time.Time `json:"timestamp"`
	HealthPercent   float64   `json:"healthPercent"`
	LifeUsedPercent float64   `json:"lifeUsedPercent"`
	TotalWrites     uint64    `json:"totalWrites"`
	Temperature     int       `json:"temperature"`
}

// SSDHealthAlert SSD 健康告警.
type SSDHealthAlert struct {
	Device       string     `json:"device"`
	AlertLevel   AlertLevel `json:"alertLevel"`
	Message      string     `json:"message"`
	LifeUsed     float64    `json:"lifeUsed"`
	Timestamp    time.Time  `json:"timestamp"`
	Acknowledged bool       `json:"acknowledged"`
}

// NewSSDHealthMonitor 创建 SSD 健康监控器.
func NewSSDHealthMonitor(config *SSDMonitorConfig) *SSDHealthMonitor {
	if config == nil {
		config = DefaultSSDMonitorConfig
	}

	m := &SSDHealthMonitor{
		ssds:           make(map[string]*SSDHealth),
		history:        make(map[string][]*SSDHealthHistory),
		alertCallbacks: make([]func(*SSDHealthAlert), 0),
		config:         config,
		stopChan:       make(chan struct{}),
	}

	// 初始扫描
	_ = m.ScanSSDs()

	// 启动定期检查
	go m.startPeriodicCheck()

	return m
}

// RegisterAlertCallback 注册告警回调.
func (m *SSDHealthMonitor) RegisterAlertCallback(callback func(*SSDHealthAlert)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertCallbacks = append(m.alertCallbacks, callback)
}

// ScanSSDs 扫描 SSD 设备.
func (m *SSDHealthMonitor) ScanSSDs() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 使用 lsblk 扫描块设备
	cmd := exec.CommandContext(context.Background(), "lsblk", "-d", "-n", "-o", "NAME,SIZE,ROTA,TYPE")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("扫描块设备失败: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		deviceName := fields[0]
		// 只处理磁盘设备
		if fields[3] != "disk" {
			continue
		}

		device := "/dev/" + deviceName

		// 判断是否为 SSD (ROTA = 0 表示非旋转设备)
		isSSD := false
		if len(fields) >= 3 && fields[2] == "0" {
			isSSD = true
		}

		// 只监控 SSD
		if !isSSD {
			continue
		}

		// 解析大小
		var size uint64
		if len(fields) >= 2 {
			size = parseSize(fields[1])
		}

		// 获取 SSD 健康信息
		health, err := m.getSSDHealth(device)
		if err != nil {
			// 创建基本信息
			health = &SSDHealth{
				Device:    device,
				Size:      size,
				Status:    SSDStatusUnknown,
				LastCheck: time.Now(),
			}
		}
		health.Size = size

		m.ssds[device] = health
	}

	return nil
}

// getSSDHealth 获取 SSD 健康信息.
func (m *SSDHealthMonitor) getSSDHealth(device string) (*SSDHealth, error) {
	// 检查 smartctl
	if _, err := exec.LookPath("smartctl"); err != nil {
		return nil, fmt.Errorf("smartctl 未安装")
	}

	health := &SSDHealth{
		Device:          device,
		Status:          SSDStatusUnknown,
		LastCheck:       time.Now(),
		SMARTAttributes: make(map[string]*SMARTAttr),
	}

	// 判断接口类型
	if strings.HasPrefix(device, "/dev/nvme") {
		health.InterfaceType = "NVMe"
		err := m.parseNVMeSMART(health)
		if err != nil {
			return nil, err
		}
	} else {
		health.InterfaceType = "SATA"
		err := m.parseSATASMART(health)
		if err != nil {
			return nil, err
		}
	}

	// 计算健康百分比
	m.calculateHealthPercent(health)

	// 评估告警级别
	m.evaluateAlertLevel(health)

	return health, nil
}

// parseNVMeSMART 解析 NVMe SSD SMART 数据.
func (m *SSDHealthMonitor) parseNVMeSMART(health *SSDHealth) error {
	cmd := exec.CommandContext(context.Background(), "smartctl", "-a", health.Device)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("获取 NVMe SMART 数据失败: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()

		// 解析型号
		if strings.HasPrefix(line, "Model Number:") {
			health.Model = strings.TrimSpace(strings.TrimPrefix(line, "Model Number:"))
		}

		// 解析序列号
		if strings.HasPrefix(line, "Serial Number:") {
			health.Serial = strings.TrimSpace(strings.TrimPrefix(line, "Serial Number:"))
		}

		// 解析固件
		if strings.HasPrefix(line, "Firmware Version:") {
			health.Firmware = strings.TrimSpace(strings.TrimPrefix(line, "Firmware Version:"))
		}

		// 解析温度
		if strings.Contains(line, "Temperature:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Temperature:" && i+1 < len(fields) {
					temp, _ := strconv.Atoi(strings.TrimSuffix(fields[i+1], "C"))
					health.Temperature = temp
				}
			}
		}

		// 解析 Percent_Lifetime_Used (NVMe: Percentage Used)
		// 格式: Percentage Used: 5%
		if strings.Contains(line, "Percentage Used:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Used:" && i+1 < len(fields) {
					pct, _ := strconv.ParseFloat(strings.TrimSuffix(fields[i+1], "%"), 64)
					health.LifeUsedPercent = pct
					health.HealthPercent = 100 - pct
				}
			}
		}

		// 解析 Data Units Written (转换为字节)
		// 格式: Data Units Written: 1,234,567 [6.33 TB]
		if strings.Contains(line, "Data Units Written:") {
			health.TotalWrites = parseDataUnits(line)
		}

		// 解析 Available Spare
		if strings.Contains(line, "Available Spare:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Spare:" && i+1 < len(fields) {
					pct, _ := strconv.ParseFloat(strings.TrimSuffix(fields[i+1], "%"), 64)
					health.AvailableSpare = pct
				}
			}
		}

		// 解析 Media Errors
		if strings.Contains(line, "Media and Data Integrity Errors:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Errors:" && i+1 < len(fields) {
					val, _ := strconv.ParseUint(strings.ReplaceAll(fields[i+1], ",", ""), 10, 64)
					health.MediaErrors = val
				}
			}
		}

		// 解析 Power On Hours
		if strings.Contains(line, "Power On Hours:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Hours:" && i+1 < len(fields) {
					val, _ := strconv.ParseUint(strings.ReplaceAll(fields[i+1], ",", ""), 10, 64)
					health.PowerOnHours = val
				}
			}
		}

		// 解析 Power Cycles
		if strings.Contains(line, "Power Cycles:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Cycles:" && i+1 < len(fields) {
					val, _ := strconv.ParseUint(strings.ReplaceAll(fields[i+1], ",", ""), 10, 64)
					health.PowerCycles = val
				}
			}
		}
	}

	return nil
}

// parseSATASMART 解析 SATA SSD SMART 数据.
func (m *SSDHealthMonitor) parseSATASMART(health *SSDHealth) error {
	cmd := exec.CommandContext(context.Background(), "smartctl", "-A", "-i", health.Device)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("获取 SATA SMART 数据失败: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	for scanner.Scan() {
		line := scanner.Text()

		// 解析型号
		if strings.HasPrefix(line, "Device Model:") {
			health.Model = strings.TrimSpace(strings.TrimPrefix(line, "Device Model:"))
		}
		if strings.HasPrefix(line, "Model Number:") {
			health.Model = strings.TrimSpace(strings.TrimPrefix(line, "Model Number:"))
		}

		// 解析序列号
		if strings.HasPrefix(line, "Serial Number:") {
			health.Serial = strings.TrimSpace(strings.TrimPrefix(line, "Serial Number:"))
		}

		// 解析固件
		if strings.HasPrefix(line, "Firmware Version:") {
			health.Firmware = strings.TrimSpace(strings.TrimPrefix(line, "Firmware Version:"))
		}

		// 解析 SMART 属性表
		// 格式: ID# ATTRIBUTE_NAME          FLAG     VALUE WORST THRESH TYPE      UPDATED  WHEN_FAILED RAW_VALUE
		fields := strings.Fields(line)
		if len(fields) >= 10 {
			id, err := strconv.ParseUint(fields[0], 10, 32)
			if err != nil {
				continue
			}

			attr := &SMARTAttr{
				ID:   uint(id),
				Name: fields[1],
			}

			// 解析值
			if val, err := strconv.ParseUint(fields[3], 10, 32); err == nil {
				attr.Value = uint(val)
			}
			if val, err := strconv.ParseUint(fields[5], 10, 32); err == nil {
				attr.Threshold = uint(val)
			}

			// 解析原始值
			rawStr := fields[len(fields)-1]
			if raw, err := strconv.ParseUint(strings.ReplaceAll(rawStr, ",", ""), 10, 64); err == nil {
				attr.Raw = raw
			}

			health.SMARTAttributes[attr.Name] = attr

			// 映射关键属性
			m.mapSMARTAttribute(health, attr)
		}
	}

	return nil
}

// mapSMARTAttribute 映射 SMART 属性到健康信息.
func (m *SSDHealthMonitor) mapSMARTAttribute(health *SSDHealth, attr *SMARTAttr) {
	switch attr.ID {
	case 5: // Reallocated_Sector_Ct
		// 重映射扇区，可能表示 SSD 磨损

	case 9: // Power_On_Hours
		health.PowerOnHours = attr.Raw

	case 12: // Power_Cycle_Count
		health.PowerCycles = attr.Raw

	case 177: // Wear_Leveling_Count (Samsung)
		// 磨损 leveling 计数，值越低表示磨损越严重
		if health.LifeUsedPercent == 0 {
			health.HealthPercent = float64(attr.Value)
			health.LifeUsedPercent = 100 - float64(attr.Value)
		}

	case 179: // Used_Rsvd_Blk_Cnt_Tot (Samsung)
		// 使用的保留块总数

	case 181: // Program_Fail_Cnt_Total
		// 编程失败总数

	case 182: // Erase_Fail_Count_Total
		// 擦除失败总数

	case 183: // Runtime_Bad_Count (Samsung)
		// 运行时坏块计数

	case 184: // End-to-End_Error
		health.MediaErrors = attr.Raw

	case 187: // Reported_Uncorrect
		// 报告的不可纠正错误
		if health.MediaErrors == 0 {
			health.MediaErrors = attr.Raw
		}

	case 190: // Airflow_Temperature_Cel
		fallthrough
	case 194: // Temperature_Celsius
		// 安全转换：温度值通常在 -40 到 200 范围内
		if attr.Raw <= 200 {
			health.Temperature = int(attr.Raw)
		} else {
			health.Temperature = 200 // 限制最大值
		}

	case 202: // Percent_Lifetime_Remain (Crucial/Micron)
		// 剩余寿命百分比
		health.HealthPercent = float64(attr.Value)
		health.LifeUsedPercent = 100 - float64(attr.Value)

	case 231: // SSD_Life_Left (Intel)
		// SSD 剩余寿命
		if health.HealthPercent == 0 {
			health.HealthPercent = float64(attr.Value)
			health.LifeUsedPercent = 100 - float64(attr.Value)
		}

	case 232: // Available_Reservd_Space
		// 可用保留空间
		health.AvailableSpare = float64(attr.Value)

	case 233: // Media_Wearout_Indicator (Intel)
		// 媒体磨损指示器，值越低表示磨损越严重
		if health.HealthPercent == 0 {
			health.HealthPercent = float64(attr.Value)
			health.LifeUsedPercent = 100 - float64(attr.Value)
		}

	case 241: // Total_LBAs_Written
		// 计算 TotalWrites (假设 512 字节扇区)
		if health.TotalWrites == 0 {
			health.TotalWrites = attr.Raw * 512
		}

	case 242: // Total_LBAs_Read
		// 读取总量

	case 244: // NAND_Writes_1GiB
		// NAND 写入量 (GiB)
		if health.TotalWrites == 0 {
			health.TotalWrites = attr.Raw * 1024 * 1024 * 1024
		}

	case 245: // Host_Writes_1GiB
		// 主机写入量 (GiB)
		if health.TotalWrites == 0 {
			health.TotalWrites = attr.Raw * 1024 * 1024 * 1024
		}

	case 249: // NAND_Writes_2GiB
		// NAND 写入量 (2GiB 单位)
		if health.TotalWrites == 0 {
			health.TotalWrites = attr.Raw * 2 * 1024 * 1024 * 1024
		}
	}
}

// calculateHealthPercent 计算健康百分比.
func (m *SSDHealthMonitor) calculateHealthPercent(health *SSDHealth) {
	// 如果已经有健康百分比，直接返回
	if health.HealthPercent > 0 {
		return
	}

	// 根据 Available Spare 估算
	if health.AvailableSpare > 0 {
		// Available Spare 通常从 100% 开始下降
		health.HealthPercent = health.AvailableSpare
		health.LifeUsedPercent = 100 - health.AvailableSpare
		return
	}

	// 尝试从重映射扇区估算
	if attr, ok := health.SMARTAttributes["Reallocated_Sector_Ct"]; ok && attr != nil {
		if attr.Value > 0 {
			health.HealthPercent = float64(attr.Value)
			health.LifeUsedPercent = 100 - float64(attr.Value)
		}
	}
}

// evaluateAlertLevel 评估告警级别.
func (m *SSDHealthMonitor) evaluateAlertLevel(health *SSDHealth) {
	lifeUsed := health.LifeUsedPercent

	switch {
	case lifeUsed >= m.config.EmergencyThreshold:
		health.AlertLevel = AlertLevelEmergency
		health.AlertMessage = fmt.Sprintf("SSD 寿命已使用 %.1f%%，超过紧急阈值 %.0f%%，请立即更换！",
			lifeUsed, m.config.EmergencyThreshold)
		health.Status = SSDStatusEmergency

	case lifeUsed >= m.config.CriticalThreshold:
		health.AlertLevel = AlertLevelCritical
		health.AlertMessage = fmt.Sprintf("SSD 寿命已使用 %.1f%%，超过严重阈值 %.0f%%，请尽快更换！",
			lifeUsed, m.config.CriticalThreshold)
		health.Status = SSDStatusCritical

	case lifeUsed >= m.config.WarningThreshold:
		health.AlertLevel = AlertLevelWarning
		health.AlertMessage = fmt.Sprintf("SSD 寿命已使用 %.1f%%，超过警告阈值 %.0f%%，建议准备更换",
			lifeUsed, m.config.WarningThreshold)
		health.Status = SSDStatusWarning

	default:
		health.AlertLevel = AlertLevelNone
		health.AlertMessage = "SSD 状态正常"
		health.Status = SSDStatusHealthy
	}

	// 检查媒体错误
	if health.MediaErrors > 10 {
		if health.Status != SSDStatusEmergency {
			health.Status = SSDStatusCritical
			health.AlertMessage += fmt.Sprintf("；检测到 %d 个媒体错误", health.MediaErrors)
		}
	}

	// 检查温度
	if health.Temperature > 70 {
		health.Status = SSDStatusWarning
		health.AlertMessage += fmt.Sprintf("；温度过高 (%d°C)", health.Temperature)
	}
}

// startPeriodicCheck 启动定期检查.
func (m *SSDHealthMonitor) startPeriodicCheck() {
	if m.config.CheckInterval <= 0 {
		return
	}

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = m.CheckAllSSDs()
		case <-m.stopChan:
			return
		}
	}
}

// CheckAllSSDs 检查所有 SSD.
func (m *SSDHealthMonitor) CheckAllSSDs() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for device := range m.ssds {
		health, err := m.getSSDHealth(device)
		if err != nil {
			m.ssds[device].Status = SSDStatusOffline
			continue
		}

		// 保存历史数据
		m.saveHistory(health)

		// 检查告警
		if health.AlertLevel != AlertLevelNone {
			m.triggerAlert(health)
		}

		// 预测寿命
		if m.config.EnablePrediction {
			m.predictLife(health)
		}

		m.ssds[device] = health
	}

	return nil
}

// saveHistory 保存历史数据.
func (m *SSDHealthMonitor) saveHistory(health *SSDHealth) {
	point := &SSDHealthHistory{
		Timestamp:       time.Now(),
		HealthPercent:   health.HealthPercent,
		LifeUsedPercent: health.LifeUsedPercent,
		TotalWrites:     health.TotalWrites,
		Temperature:     health.Temperature,
	}

	history := m.history[health.Device]
	history = append(history, point)

	// 限制历史数据量 (默认保留 30 天，每 30 分钟一次，约 1440 条)
	maxPoints := m.config.HistoryRetention * 48
	if len(history) > maxPoints {
		history = history[len(history)-maxPoints:]
	}

	m.history[health.Device] = history
}

// triggerAlert 触发告警.
func (m *SSDHealthMonitor) triggerAlert(health *SSDHealth) {
	alert := &SSDHealthAlert{
		Device:     health.Device,
		AlertLevel: health.AlertLevel,
		Message:    health.AlertMessage,
		LifeUsed:   health.LifeUsedPercent,
		Timestamp:  time.Now(),
	}

	for _, callback := range m.alertCallbacks {
		go callback(alert)
	}
}

// predictLife 预测寿命.
func (m *SSDHealthMonitor) predictLife(health *SSDHealth) {
	m.mu.RLock()
	history := m.history[health.Device]
	m.mu.RUnlock()

	if len(history) < 10 {
		return
	}

	// 计算写入速率趋势
	var totalWriteIncrease uint64
	var days float64

	for i := 1; i < len(history); i++ {
		if history[i].TotalWrites > history[i-1].TotalWrites {
			totalWriteIncrease += history[i].TotalWrites - history[i-1].TotalWrites
		}
	}

	// 计算时间跨度
	if len(history) >= 2 {
		duration := history[len(history)-1].Timestamp.Sub(history[0].Timestamp)
		days = duration.Hours() / 24
	}

	if days == 0 || totalWriteIncrease == 0 {
		return
	}

	// 日均写入量
	writeRatePerDay := uint64(float64(totalWriteIncrease) / days)

	// 估算剩余寿命 (假设 TBW 为总写入量 / (1 - 健康百分比) * 健康百分比)
	// 简化：使用当前写入速率估算
	if health.HealthPercent > 0 && writeRatePerDay > 0 {
		// 假设 TBW 已用比例与健康百分比对应
		estimatedRemainingWrites := uint64(float64(health.TotalWrites) / health.LifeUsedPercent * health.HealthPercent)
		// 安全转换：限制剩余天数在合理范围内
		var remainingDays int
		if estimatedRemainingWrites/writeRatePerDay > uint64(36500) { // 100 年
			remainingDays = 36500
		} else {
			remainingDays = int(estimatedRemainingWrites / writeRatePerDay)
		}

		// 置信度基于历史数据量
		confidence := float64(len(history)) / 1440.0 // 30 天数据 = 1.0
		if confidence > 1 {
			confidence = 1
		}
		confidence = confidence * 0.7 // 最高 0.7 置信度

		health.PredictedLife = &PredictedLife{
			RemainingDays:    remainingDays,
			EstimatedEndDate: time.Now().AddDate(0, 0, remainingDays),
			WriteRatePerDay:  writeRatePerDay,
			Confidence:       confidence,
			Method:           "write_rate_projection",
		}
	}
}

// GetSSDHealth 获取单个 SSD 健康信息.
func (m *SSDHealthMonitor) GetSSDHealth(device string) (*SSDHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health, exists := m.ssds[device]
	if !exists {
		return nil, fmt.Errorf("SSD 不存在: %s", device)
	}
	return health, nil
}

// GetAllSSDs 获取所有 SSD 健康信息.
func (m *SSDHealthMonitor) GetAllSSDs() []*SSDHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SSDHealth, 0, len(m.ssds))
	for _, health := range m.ssds {
		result = append(result, health)
	}
	return result
}

// GetSSDHistory 获取 SSD 历史数据.
func (m *SSDHealthMonitor) GetSSDHistory(device string, days int) []*SSDHealthHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, exists := m.history[device]
	if !exists {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var result []*SSDHealthHistory
	for _, point := range history {
		if point.Timestamp.After(cutoff) {
			result = append(result, point)
		}
	}
	return result
}

// Stop 停止监控.
func (m *SSDHealthMonitor) Stop() {
	close(m.stopChan)
}

// parseSize 解析大小字符串.
func parseSize(sizeStr string) uint64 {
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))
	multiplier := uint64(1)

	// 处理单位
	if strings.HasSuffix(sizeStr, "TB") {
		multiplier = 1024 * 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "TB")
	} else if strings.HasSuffix(sizeStr, "GB") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "GB")
	} else if strings.HasSuffix(sizeStr, "MB") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "MB")
	} else if strings.HasSuffix(sizeStr, "KB") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "KB")
	} else if strings.HasSuffix(sizeStr, "T") {
		multiplier = 1024 * 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "T")
	} else if strings.HasSuffix(sizeStr, "G") {
		multiplier = 1024 * 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "G")
	} else if strings.HasSuffix(sizeStr, "M") {
		multiplier = 1024 * 1024
		sizeStr = strings.TrimSuffix(sizeStr, "M")
	} else if strings.HasSuffix(sizeStr, "K") {
		multiplier = 1024
		sizeStr = strings.TrimSuffix(sizeStr, "K")
	}

	sizeStr = strings.TrimSpace(sizeStr)

	// 处理小数
	if strings.Contains(sizeStr, ".") {
		if val, err := strconv.ParseFloat(sizeStr, 64); err == nil {
			return uint64(val * float64(multiplier))
		}
	}

	size, _ := strconv.ParseUint(sizeStr, 10, 64)
	return size * multiplier
}

// parseDataUnits 解析数据单元 (NVMe)
// 格式: Data Units Written: 1,234,567 [6.33 TB].
func parseDataUnits(line string) uint64 {
	// 提取方括号中的大小
	start := strings.Index(line, "[")
	end := strings.Index(line, "]")
	if start == -1 || end == -1 || end <= start {
		return 0
	}

	sizeStr := strings.TrimSpace(line[start+1 : end])
	return parseSize(sizeStr)
}
