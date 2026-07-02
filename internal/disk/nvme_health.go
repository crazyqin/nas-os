// Package disk 提供NVMe健康监控功能
// Version: v2.387.0 - NVMe S.M.A.R.T增强监控 + 三级预警 + 健康预测
// 参考 TrueNAS 25.10 NVMe S.M.A.R.T测试功能实现
package disk

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AlertLevel 三级预警等级枚举 - 对标TrueNAS 25.10.
type AlertLevel string

const (
	// AlertLevelNormal 正常状态.
	AlertLevelNormal AlertLevel = "normal"
	// AlertLevelWarning 警告级别 - 需要关注.
	AlertLevelWarning AlertLevel = "warning"
	// AlertLevelCritical 严重级别 - 需要尽快处理.
	AlertLevelCritical AlertLevel = "critical"
	// AlertLevelEmergency 紧急级别 - 需要立即处理.
	AlertLevelEmergency AlertLevel = "emergency"
)

// AlertThresholds 预警阈值配置.
type AlertThresholds struct {
	// 温度阈值 (摄氏度)
	TempWarning  uint8 `json:"temp_warning"`  // 70°C
	TempCritical uint8 `json:"temp_critical"` // 85°C

	// 健康度阈值 (百分比)
	HealthWarning   uint8 `json:"health_warning"`   // 90%
	HealthCritical  uint8 `json:"health_critical"`  // 80%
	HealthEmergency uint8 `json:"health_emergency"` // 70%

	// 写入量阈值 (TBW百分比)
	TBWWarning  uint8 `json:"tbw_warning"`  // 80%
	TBWCritical uint8 `json:"tbw_critical"` // 90%
}

// DefaultAlertThresholds 默认预警阈值.
var DefaultAlertThresholds = &AlertThresholds{
	TempWarning:     70,
	TempCritical:    85,
	HealthWarning:   90,
	HealthCritical:  80,
	HealthEmergency: 70,
	TBWWarning:      80,
	TBWCritical:     90,
}

// NVMeHealthInfo NVMe健康信息.
type NVMeHealthInfo struct {
	// 设备信息
	Device     string `json:"device"`
	Model      string `json:"model"`
	Serial     string `json:"serial"`
	Firmware   string `json:"firmware"`
	Controller string `json:"controller"`
	Size       uint64 `json:"size"`

	// 健康状态
	OverallHealth    string       `json:"overallHealth"`    // ok/warn/critical
	SmartStatus      string       `json:"smartStatus"`      // PASSED/FAILED
	HealthPercentage uint8        `json:"healthPercentage"` // 0-100
	Status           DiskStatus   `json:"status"`
	HealthScore      *HealthScore `json:"healthScore,omitempty"`

	// 三级预警 - v2.387.0
	AlertLevel      AlertLevel       `json:"alertLevel"`                // 当前预警等级
	AlertReasons    []string         `json:"alertReasons"`              // 预警原因列表
	AlertThresholds *AlertThresholds `json:"alertThresholds,omitempty"` // 使用的阈值配置

	// NVMe SMART属性
	Temperature      *NVMeTempInfo  `json:"temperature,omitempty"`
	Usage            *NVMeUsageInfo `json:"usage,omitempty"`
	PowerOnHours     uint64         `json:"powerOnHours"`
	PowerCycles      uint64         `json:"powerCycles"`
	AvailableSpare   *NVMeSpareInfo `json:"availableSpare,omitempty"`
	MediaErrors      uint64         `json:"mediaErrors"`
	CriticalWarnings uint8          `json:"criticalWarnings"`

	// 详细SMART属性
	ControllerBusyTime uint64 `json:"controllerBusyTime"`
	UnsafeShutdowns    uint64 `json:"unsafeShutdowns"`
	IntegrityErrors    uint64 `json:"integrityErrors"`
	ErrorLogEntries    uint64 `json:"errorLogEntries"`
	WarningTempTime    uint64 `json:"warningTempTime"`  // 温度超过警告阈值的时间(分钟)
	CriticalTempTime   uint64 `json:"criticalTempTime"` // 温度超过严重阈值的时间(分钟)

	// 历史数据
	DataUnitsRead     uint64 `json:"dataUnitsRead"`
	DataUnitsWritten  uint64 `json:"dataUnitsWritten"`
	HostReadCommands  uint64 `json:"hostReadCommands"`
	HostWriteCommands uint64 `json:"hostWriteCommands"`

	// 测试状态
	LastTestResult *NVMeTestResult `json:"lastTestResult,omitempty"`

	// 元数据
	LastCheck time.Time `json:"lastCheck"`
}

// NVMeTempInfo NVMe温度信息.
type NVMeTempInfo struct {
	Current        uint8  `json:"current"`        // 当前温度
	Warning        uint8  `json:"warning"`        // 警告阈值
	Critical       uint8  `json:"critical"`       // 严重阈值
	MinTemp        uint8  `json:"minTemp"`        // 历史最低温度
	MaxTemp        uint8  `json:"maxTemp"`        // 历史最高温度
	CompositeTemp  uint8  `json:"compositeTemp"`  // 复合温度
	OverTempEvents uint64 `json:"overTempEvents"` // 过热事件次数
}

// NVMeUsageInfo NVMe使用情况信息.
type NVMeUsageInfo struct {
	PercentageUsed   uint8   `json:"percentageUsed"`   // 已使用百分比 (0-100)
	DataUnitsWritten uint64  `json:"dataUnitsWritten"` // 写入数据单位
	TBW              float64 `json:"tbw"`              // 写入量(TB)
	TotalWrites      float64 `json:"totalWrites"`      // 总写入量(GB)
	TotalReads       float64 `json:"totalReads"`       // 总读取量(GB)
	WearLevel        string  `json:"wearLevel"`        // low/medium/high
	EstimatedLife    string  `json:"estimatedLife"`    // 预估剩余寿命

	// 寿命预测增强 - v2.387.0
	LifePrediction *NVMeLifePrediction `json:"lifePrediction,omitempty"` // 寿命预测
	EstimatedTBW   float64             `json:"estimatedTBW,omitempty"`   // 预估TBW总容量
	TBWUsedPercent float64             `json:"tbwUsedPercent,omitempty"` // TBW使用百分比
}

// NVMeLifePrediction NVMe寿命预测 - 基于写入量、温度历史、磨损程度.
type NVMeLifePrediction struct {
	// 预测结果
	RemainingLifePercent float64   `json:"remainingLifePercent"` // 剩余寿命百分比
	EstimatedDaysLeft    int       `json:"estimatedDaysLeft"`    // 预估剩余天数
	EstimatedEndDate     time.Time `json:"estimatedEndDate"`     // 预估寿命终结日期
	ConfidenceLevel      string    `json:"confidenceLevel"`      // 预测置信度: high/medium/low

	// 预测因子
	WriteAmplificationFactor float64 `json:"writeAmplificationFactor"` // 写放大因子
	AverageDailyWrites       float64 `json:"averageDailyWrites"`       // 平均每日写入量(GB)
	TemperatureImpact        float64 `json:"temperatureImpact"`        // 温度对寿命的影响系数 (0-1)
	WearImpact               float64 `json:"wearImpact"`               // 磨损程度影响系数 (0-1)

	// 历史数据
	TemperatureHistory    []TempRecord `json:"temperatureHistory,omitempty"` // 温度历史
	WeeklyWriteRates      []float64    `json:"weeklyWriteRates,omitempty"`   // 每周写入率历史
	PredictionLastUpdated time.Time    `json:"predictionLastUpdated"`
}

// TempRecord 温度历史记录.
type TempRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Temp      uint8     `json:"temp"`
	Duration  float64   `json:"duration"` // 持续时间(分钟)
}

// NVMeSpareInfo NVMe备用空间信息.
type NVMeSpareInfo struct {
	Available  uint8 `json:"available"`  // 可用备用空间百分比
	Threshold  uint8 `json:"threshold"`  // 阈值
	Percentage uint8 `json:"percentage"` // 当前百分比
}

// NVMeTestType NVMe测试类型.
type NVMeTestType string

// NVMe测试类型常量.
const (
	// NVMeTestShort 短测试 (~2分钟).
	NVMeTestShort     NVMeTestType = "short"     // 短测试 (~2分钟)
	NVMeTestLong      NVMeTestType = "long"      // 长测试 (扩展测试)
	NVMeTestVendor    NVMeTestType = "vendor"    // 厂商特定测试
	NVMeTestVerify    NVMeTestType = "verify"    // 数据验证测试
	NVMeTestReadWrite NVMeTestType = "readwrite" // 读写测试
)

// NVMeTestResult NVMe测试结果.
type NVMeTestResult struct {
	TestType     NVMeTestType  `json:"testType"`
	Device       string        `json:"device"`
	Status       string        `json:"status"`   // running/complete/aborted/failed
	Result       string        `json:"result"`   // pass/fail
	Progress     uint8         `json:"progress"` // 0-100
	StartTime    time.Time     `json:"startTime"`
	EndTime      *time.Time    `json:"endTime,omitempty"`
	Duration     time.Duration `json:"duration"`
	ErrorCode    uint8         `json:"errorCode,omitempty"`
	ErrorMessage string        `json:"errorMessage,omitempty"`
	NumErrors    uint8         `json:"numErrors"` // 测试发现的错误数
}

// NVMeMonitor NVMe监控器.
type NVMeMonitor struct {
	devices   map[string]*NVMeHealthInfo
	testQueue map[string]*NVMeTestResult // 正在运行的测试
	mu        sync.RWMutex

	// v2.387.0 增强
	alertThresholds *AlertThresholds               // 预警阈值
	lifePredictions map[string]*NVMeLifePrediction // 设备寿命预测缓存
	predictionMu    sync.RWMutex
	tempHistory     map[string][]TempRecord // 设备温度历史
	historyMu       sync.RWMutex
}

// NewNVMeMonitor 创建NVMe监控器.
func NewNVMeMonitor() *NVMeMonitor {
	return &NVMeMonitor{
		devices:         make(map[string]*NVMeHealthInfo),
		testQueue:       make(map[string]*NVMeTestResult),
		alertThresholds: DefaultAlertThresholds,
		lifePredictions: make(map[string]*NVMeLifePrediction),
		tempHistory:     make(map[string][]TempRecord),
	}
}

// SetAlertThresholds 设置预警阈值.
func (m *NVMeMonitor) SetAlertThresholds(thresholds *AlertThresholds) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertThresholds = thresholds
}

// GetAlertThresholds 获取预警阈值.
func (m *NVMeMonitor) GetAlertThresholds() *AlertThresholds {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alertThresholds
}

// ScanNVMeDevices 扫描NVMe设备.
func (m *NVMeMonitor) ScanNVMeDevices() ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "nvme", "list", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		// 尝试使用 smartctl 列出设备
		return m.scanWithSmartctl()
	}

	var list struct {
		Devices []struct {
			DevicePath string `json:"DevicePath"`
			ModelName  string `json:"ModelNumber"`
		} `json:"Devices"`
	}

	if err := json.Unmarshal(output, &list); err != nil {
		return m.scanWithSmartctl()
	}

	devices := make([]string, 0, len(list.Devices))
	for _, dev := range list.Devices {
		devices = append(devices, dev.DevicePath)
	}

	return devices, nil
}

// scanWithSmartctl 使用smartctl扫描NVMe设备.
func (m *NVMeMonitor) scanWithSmartctl() ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "lsblk", "-d", "-n", "-o", "NAME")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("扫描设备失败: %w", err)
	}

	devices := []string{}
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		name := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(name, "nvme") {
			devices = append(devices, "/dev/"+name)
		}
	}

	return devices, nil
}

// GetNVMeHealth 获取NVMe设备健康信息.
func (m *NVMeMonitor) GetNVMeHealth(device string) (*NVMeHealthInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查缓存（5分钟内有效）
	if info, exists := m.devices[device]; exists {
		if time.Since(info.LastCheck) < 5*time.Minute {
			return info, nil
		}
	}

	info, err := m.collectNVMeHealth(device)
	if err != nil {
		return nil, err
	}

	m.devices[device] = info
	return info, nil
}

// collectNVMeHealth 收集NVMe健康数据.
func (m *NVMeMonitor) collectNVMeHealth(device string) (*NVMeHealthInfo, error) {
	info := &NVMeHealthInfo{
		Device:          device,
		LastCheck:       time.Now(),
		Status:          StatusUnknown,
		AlertThresholds: m.alertThresholds,
	}

	// 1. 使用 nvme-cli 获取 SMART 数据
	if err := m.getNVMeSmartLog(device, info); err != nil {
		// 2. 回退到 smartctl
		if fallbackErr := m.getNVMeSmartctl(device, info); fallbackErr != nil {
			return nil, fmt.Errorf("获取NVMe健康数据失败: %v (nvme-cli: %v, smartctl: %v)",
				fallbackErr, err, fallbackErr)
		}
	}

	// 3. 计算健康评分
	m.calculateNVMeHealthScore(info)

	// 4. v2.387.0: 三级预警评估
	alertLevel, alertReasons := m.EvaluateAlertLevel(info)
	info.AlertLevel = alertLevel
	info.AlertReasons = alertReasons

	// 5. v2.387.0: 寿命预测
	prediction := m.PredictRemainingLife(info)
	if prediction != nil && info.Usage != nil {
		info.Usage.LifePrediction = prediction
	}

	// 6. 根据预警等级调整状态
	switch alertLevel {
	case AlertLevelEmergency:
		info.Status = StatusCritical
		info.OverallHealth = "emergency"
	case AlertLevelCritical:
		info.Status = StatusCritical
		info.OverallHealth = "critical"
	case AlertLevelWarning:
		if info.Status != StatusCritical {
			info.Status = StatusWarning
			info.OverallHealth = "warn"
		}
	case AlertLevelNormal:
		// 保持原有状态
	}

	// 7. 记录温度历史
	if info.Temperature != nil {
		m.RecordTemperature(device, info.Temperature.Current, 5.0) // 默认5分钟周期
	}

	return info, nil
}

// getNVMeSmartLog 使用nvme-cli获取SMART日志.
func (m *NVMeMonitor) getNVMeSmartLog(device string, info *NVMeHealthInfo) error {
	// 获取设备信息
	cmd := exec.CommandContext(context.Background(), "nvme", "id-ctrl", device, "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("nvme id-ctrl 失败: %w", err)
	}

	var ctrlData struct {
		ModelNumber  string `json:"mn"`
		SerialNumber string `json:"sn"`
		Firmware     string `json:"fr"`
		TotalNVM     uint64 `json:"tnvmcap"`
		IEEE         string `json:"ieee"`
	}

	if err := json.Unmarshal(output, &ctrlData); err != nil {
		return fmt.Errorf("解析控制器信息失败: %w", err)
	}

	info.Model = strings.TrimSpace(ctrlData.ModelNumber)
	info.Serial = strings.TrimSpace(ctrlData.SerialNumber)
	info.Firmware = strings.TrimSpace(ctrlData.Firmware)
	info.Size = ctrlData.TotalNVM / (1024 * 1024) // 转换为MB

	// 获取 SMART 日志
	cmd = exec.CommandContext(context.Background(), "nvme", "smart-log", device, "-o", "json")
	output, err = cmd.Output()
	if err != nil {
		return fmt.Errorf("nvme smart-log 失败: %w", err)
	}

	var smartLog struct {
		CriticalWarning         uint8  `json:"critical_warning"`
		Temperature             uint16 `json:"temperature"`
		AvailableSpare          uint8  `json:"available_spare"`
		AvailableSpareThreshold uint8  `json:"available_spare_threshold"`
		PercentageUsed          uint8  `json:"percentage_used"`
		DataUnitsRead           uint64 `json:"data_units_read,string"`
		DataUnitsWritten        uint64 `json:"data_units_written,string"`
		HostReadCommands        uint64 `json:"host_read_commands,string"`
		HostWriteCommands       uint64 `json:"host_write_commands,string"`
		ControllerBusyTime      uint64 `json:"controller_busy_time,string"`
		PowerCycles             uint64 `json:"power_cycles,string"`
		PowerOnHours            uint64 `json:"power_on_hours,string"`
		UnsafeShutdowns         uint64 `json:"unsafe_shutdowns,string"`
		MediaErrors             uint64 `json:"media_errors,string"`
		NumErrLogEntries        uint64 `json:"num_err_log_entries,string"`
		WarningTempTime         uint32 `json:"warning_temp_time"`
		CriticalCompTempTime    uint32 `json:"critical_comp_temp_time"`
		ThmTemp1TransCount      uint32 `json:"thm_temp1_trans_count,string"`
		ThmTemp2TransCount      uint32 `json:"thm_temp2_trans_count,string"`
		ThmTemp1TotalTime       uint32 `json:"thm_temp1_total_time,string"`
		ThmTemp2TotalTime       uint32 `json:"thm_temp2_total_time,string"`
	}

	if err := json.Unmarshal(output, &smartLog); err != nil {
		return fmt.Errorf("解析SMART日志失败: %w", err)
	}

	// 填充信息
	info.CriticalWarnings = smartLog.CriticalWarning
	info.HealthPercentage = 100 - smartLog.PercentageUsed
	info.PowerOnHours = smartLog.PowerOnHours
	info.PowerCycles = smartLog.PowerCycles
	info.MediaErrors = smartLog.MediaErrors
	info.ControllerBusyTime = smartLog.ControllerBusyTime
	info.UnsafeShutdowns = smartLog.UnsafeShutdowns
	info.IntegrityErrors = smartLog.MediaErrors
	info.ErrorLogEntries = smartLog.NumErrLogEntries
	info.DataUnitsRead = smartLog.DataUnitsRead
	info.DataUnitsWritten = smartLog.DataUnitsWritten
	info.HostReadCommands = smartLog.HostReadCommands
	info.HostWriteCommands = smartLog.HostWriteCommands
	info.WarningTempTime = uint64(smartLog.WarningTempTime)
	info.CriticalTempTime = uint64(smartLog.CriticalCompTempTime)

	// 温度信息
	info.Temperature = &NVMeTempInfo{
		Current:        uint8(smartLog.Temperature - 273), // 开尔文转摄氏度
		OverTempEvents: uint64(smartLog.ThmTemp1TransCount + smartLog.ThmTemp2TransCount),
	}

	// 使用情况信息
	info.Usage = &NVMeUsageInfo{
		PercentageUsed:   smartLog.PercentageUsed,
		DataUnitsWritten: smartLog.DataUnitsWritten,
		TBW:              float64(smartLog.DataUnitsWritten) * 512 / (1024 * 1024 * 1024), // 转换为TB
		TotalWrites:      float64(smartLog.DataUnitsWritten) * 512 / (1024 * 1024),        // 转换为GB
		TotalReads:       float64(smartLog.DataUnitsRead) * 512 / (1024 * 1024),           // 转换为GB
	}

	// 根据使用百分比判断磨损等级
	switch {
	case smartLog.PercentageUsed < 10:
		info.Usage.WearLevel = "low"
		info.Usage.EstimatedLife = ">90%"
	case smartLog.PercentageUsed < 50:
		info.Usage.WearLevel = "low"
		info.Usage.EstimatedLife = ">50%"
	case smartLog.PercentageUsed < 80:
		info.Usage.WearLevel = "medium"
		info.Usage.EstimatedLife = "20-50%"
	default:
		info.Usage.WearLevel = "high"
		info.Usage.EstimatedLife = "<20%"
	}

	// 备用空间信息
	info.AvailableSpare = &NVMeSpareInfo{
		Available:  smartLog.AvailableSpare,
		Threshold:  smartLog.AvailableSpareThreshold,
		Percentage: smartLog.AvailableSpare,
	}

	// 确定健康状态
	if smartLog.CriticalWarning == 0 && smartLog.PercentageUsed < 80 {
		info.OverallHealth = "ok"
		info.SmartStatus = "PASSED"
		info.Status = StatusHealthy
	} else if smartLog.CriticalWarning > 0 || smartLog.PercentageUsed >= 90 {
		info.OverallHealth = "critical"
		info.SmartStatus = "FAILED"
		info.Status = StatusCritical
	} else {
		info.OverallHealth = "warn"
		info.SmartStatus = "PASSED"
		info.Status = StatusWarning
	}

	return nil
}

// getNVMeSmartctl 使用smartctl获取NVMe数据（回退方案）.
func (m *NVMeMonitor) getNVMeSmartctl(device string, info *NVMeHealthInfo) error {
	cmd := exec.CommandContext(context.Background(), "smartctl", "-a", device)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("smartctl 失败: %w", err)
	}

	return m.parseSmartctlOutput(string(output), info)
}

// parseSmartctlOutput 解析smartctl输出.
func (m *NVMeMonitor) parseSmartctlOutput(output string, info *NVMeHealthInfo) error {
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()

		// 解析型号
		if strings.HasPrefix(line, "Model Number:") {
			info.Model = strings.TrimSpace(strings.TrimPrefix(line, "Model Number:"))
		}

		// 解析序列号
		if strings.HasPrefix(line, "Serial Number:") {
			info.Serial = strings.TrimSpace(strings.TrimPrefix(line, "Serial Number:"))
		}

		// 解析固件版本
		if strings.HasPrefix(line, "Firmware Version:") {
			info.Firmware = strings.TrimSpace(strings.TrimPrefix(line, "Firmware Version:"))
		}

		// 解析SMART状态
		if strings.Contains(line, "SMART overall-health self-assessment test result:") {
			if strings.Contains(line, "PASSED") {
				info.SmartStatus = "PASSED"
				info.OverallHealth = "ok"
				info.Status = StatusHealthy
			} else {
				info.SmartStatus = "FAILED"
				info.OverallHealth = "critical"
				info.Status = StatusCritical
			}
		}

		// 解析温度
		if strings.Contains(line, "Temperature:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Temperature:" && i+1 < len(fields) {
					temp, _ := strconv.ParseUint(fields[i+1], 10, 8)
					if info.Temperature == nil {
						info.Temperature = &NVMeTempInfo{}
					}
					info.Temperature.Current = uint8(temp)
				}
			}
		}

		// 解析使用百分比
		if strings.Contains(line, "Percentage Used:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Used:" && i+1 < len(fields) {
					pct, _ := strconv.ParseFloat(strings.TrimSuffix(fields[i+1], "%"), 64)
					if info.Usage == nil {
						info.Usage = &NVMeUsageInfo{}
					}
					info.Usage.PercentageUsed = uint8(pct)
					info.HealthPercentage = uint8(100 - pct)
				}
			}
		}

		// 解析数据单元写入
		if strings.Contains(line, "Data Units Written:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Written:" && i+1 < len(fields) {
					units, _ := strconv.ParseUint(strings.ReplaceAll(fields[i+1], ",", ""), 10, 64)
					if info.Usage == nil {
						info.Usage = &NVMeUsageInfo{}
					}
					info.Usage.DataUnitsWritten = units
					info.Usage.TBW = float64(units) * 512 / (1024 * 1024 * 1024)
				}
			}
		}

		// 解析电源时间
		if strings.Contains(line, "Power On Hours:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Hours:" && i+1 < len(fields) {
					hours, _ := strconv.ParseUint(strings.ReplaceAll(fields[i+1], ",", ""), 10, 64)
					info.PowerOnHours = hours
				}
			}
		}

		// 解析电源循环次数
		if strings.Contains(line, "Power Cycles:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Cycles:" && i+1 < len(fields) {
					cycles, _ := strconv.ParseUint(strings.ReplaceAll(fields[i+1], ",", ""), 10, 64)
					info.PowerCycles = cycles
				}
			}
		}

		// 解析媒体错误
		if strings.Contains(line, "Media and Data Integrity Errors:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Errors:" && i+1 < len(fields) {
					errors, _ := strconv.ParseUint(strings.ReplaceAll(fields[i+1], ",", ""), 10, 64)
					info.MediaErrors = errors
				}
			}
		}

		// 解析可用备用空间
		if strings.Contains(line, "Available Spare:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Spare:" && i+1 < len(fields) {
					spare, _ := strconv.ParseFloat(strings.TrimSuffix(fields[i+1], "%"), 64)
					if info.AvailableSpare == nil {
						info.AvailableSpare = &NVMeSpareInfo{}
					}
					info.AvailableSpare.Percentage = uint8(spare)
					info.AvailableSpare.Available = uint8(spare)
				}
			}
		}
	}

	return nil
}

// calculateNVMeHealthScore 计算NVMe健康评分.
func (m *NVMeMonitor) calculateNVMeHealthScore(info *NVMeHealthInfo) {
	components := &ScoreComponents{}

	// 1. 温度评分
	components.Temperature = m.calculateNVMeTempScore(info)

	// 2. 使用寿命评分
	components.Age = m.calculateNVMeWearScore(info)

	// 3. 错误评分
	components.Errors = m.calculateNVMeErrorScore(info)

	// 4. 备用空间评分
	components.Stability = m.calculateNVMeSpareScore(info)

	// 计算总分（NVMe权重调整）
	weights := &ScoreWeights{
		Temperature:  0.20,
		Reallocation: 0.20, // 用于使用寿命
		Pending:      0.10,
		Errors:       0.25,
		Age:          0.15,
		Stability:    0.10,
	}

	totalScore := int(
		float64(components.Temperature.Score)*weights.Temperature +
			float64(components.Age.Score)*weights.Reallocation +
			float64(components.Errors.Score)*weights.Errors +
			float64(components.Stability.Score)*weights.Stability,
	)

	if totalScore > 100 {
		totalScore = 100
	}
	if totalScore < 0 {
		totalScore = 0
	}

	grade, status := m.getNVMeGradeAndStatus(totalScore, info)

	recommendations := m.generateNVMeRecommendations(info, components)

	info.HealthScore = &HealthScore{
		Score:           totalScore,
		Grade:           grade,
		Status:          status,
		Components:      components,
		Recommendations: recommendations,
		Timestamp:       time.Now(),
	}

	info.Status = status
}

// calculateNVMeTempScore 计算NVMe温度评分.
func (m *NVMeMonitor) calculateNVMeTempScore(info *NVMeHealthInfo) ComponentScore {
	if info.Temperature == nil {
		return ComponentScore{Score: 100, Weight: 0.20, Status: "ok", Message: "无温度数据"}
	}

	temp := info.Temperature.Current
	score := ComponentScore{
		Weight: 0.20,
		Value:  temp,
	}

	switch {
	case temp <= 45:
		score.Score = 100
		score.Status = "ok"
		score.Message = "温度正常"
	case temp <= 55:
		score.Score = 85
		score.Status = "ok"
		score.Message = "温度偏高但正常"
	case temp <= 65:
		score.Score = 65
		score.Status = "warning"
		score.Message = "温度警告"
	case temp <= 75:
		score.Score = 40
		score.Status = "warning"
		score.Message = "温度过高，建议改善散热"
	default:
		score.Score = 15
		score.Status = "critical"
		score.Message = "温度严重过高，立即检查散热"
	}

	return score
}

// calculateNVMeWearScore 计算NVMe磨损评分.
func (m *NVMeMonitor) calculateNVMeWearScore(info *NVMeHealthInfo) ComponentScore {
	if info.Usage == nil {
		return ComponentScore{Score: 100, Weight: 0.15, Status: "ok", Message: "无使用数据"}
	}

	pctUsed := info.Usage.PercentageUsed
	score := ComponentScore{
		Weight: 0.15,
		Value:  pctUsed,
	}

	switch {
	case pctUsed < 10:
		score.Score = 100
		score.Status = "ok"
		score.Message = "使用寿命充足"
	case pctUsed < 30:
		score.Score = 90
		score.Status = "ok"
		score.Message = "使用正常"
	case pctUsed < 50:
		score.Score = 75
		score.Status = "ok"
		score.Message = "使用较多，注意监控"
	case pctUsed < 70:
		score.Score = 50
		score.Status = "warning"
		score.Message = "使用寿命下降，建议备份"
	case pctUsed < 90:
		score.Score = 25
		score.Status = "warning"
		score.Message = "使用寿命较低，建议更换"
	default:
		score.Score = 5
		score.Status = "critical"
		score.Message = "使用寿命即将耗尽，立即更换"
	}

	return score
}

// calculateNVMeErrorScore 计算NVMe错误评分.
func (m *NVMeMonitor) calculateNVMeErrorScore(info *NVMeHealthInfo) ComponentScore {
	var totalErrors uint64
	var errorTypes []string

	if info.MediaErrors > 0 {
		totalErrors += info.MediaErrors
		errorTypes = append(errorTypes, "媒体错误")
	}
	if info.IntegrityErrors > 0 {
		totalErrors += info.IntegrityErrors
		errorTypes = append(errorTypes, "完整性错误")
	}
	if info.ErrorLogEntries > 0 {
		totalErrors += info.ErrorLogEntries
		errorTypes = append(errorTypes, "日志错误")
	}
	if info.UnsafeShutdowns > 10 {
		totalErrors += info.UnsafeShutdowns / 10
		errorTypes = append(errorTypes, "非安全关机")
	}

	score := ComponentScore{
		Weight: 0.25,
		Value:  totalErrors,
	}

	switch {
	case totalErrors == 0:
		score.Score = 100
		score.Status = "ok"
		score.Message = "无错误"
	case totalErrors <= 5:
		score.Score = 80
		score.Status = "ok"
		score.Message = "少量错误"
	case totalErrors <= 50:
		score.Score = 50
		score.Status = "warning"
		score.Message = fmt.Sprintf("存在错误: %v", strings.Join(errorTypes, ", "))
	default:
		score.Score = 10
		score.Status = "critical"
		score.Message = fmt.Sprintf("错误过多: %v", strings.Join(errorTypes, ", "))
	}

	return score
}

// calculateNVMeSpareScore 计算NVMe备用空间评分.
func (m *NVMeMonitor) calculateNVMeSpareScore(info *NVMeHealthInfo) ComponentScore {
	if info.AvailableSpare == nil {
		return ComponentScore{Score: 100, Weight: 0.10, Status: "ok", Message: "无备用空间数据"}
	}

	spare := info.AvailableSpare.Percentage
	threshold := info.AvailableSpare.Threshold
	score := ComponentScore{
		Weight: 0.10,
		Value:  spare,
	}

	switch {
	case spare >= 80:
		score.Score = 100
		score.Status = "ok"
		score.Message = "备用空间充足"
	case spare >= 50:
		score.Score = 80
		score.Status = "ok"
		score.Message = "备用空间正常"
	case spare >= threshold:
		score.Score = 50
		score.Status = "warning"
		score.Message = "备用空间偏低"
	case spare > 0:
		score.Score = 20
		score.Status = "critical"
		score.Message = "备用空间严重不足"
	default:
		score.Score = 0
		score.Status = "critical"
		score.Message = "备用空间已耗尽"
	}

	return score
}

// getNVMeGradeAndStatus 获取NVMe等级和状态.
func (m *NVMeMonitor) getNVMeGradeAndStatus(score int, info *NVMeHealthInfo) (string, DiskStatus) {
	// 如果有关键警告，直接标记为严重
	if info.CriticalWarnings > 0 {
		return "F", StatusCritical
	}

	switch {
	case score >= 90:
		return "A", StatusHealthy
	case score >= 80:
		return "B", StatusHealthy
	case score >= 70:
		return "C", StatusWarning
	case score >= 50:
		return "D", StatusWarning
	default:
		return "F", StatusCritical
	}
}

// generateNVMeRecommendations 生成NVMe建议.
func (m *NVMeMonitor) generateNVMeRecommendations(info *NVMeHealthInfo, components *ScoreComponents) []string {
	var recommendations []string

	// 温度建议
	switch components.Temperature.Status {
	case "critical":
		recommendations = append(recommendations, "NVMe温度严重过高，立即安装散热片或改善散热")
	case "warning":
		recommendations = append(recommendations, "NVMe温度偏高，建议安装散热片")
	}

	// 使用寿命建议
	if info.Usage != nil {
		if info.Usage.PercentageUsed >= 90 {
			recommendations = append(recommendations, "NVMe使用寿命即将耗尽，立即备份数据并更换")
		} else if info.Usage.PercentageUsed >= 70 {
			recommendations = append(recommendations, "NVMe使用寿命下降，建议规划更换")
		}
	}

	// 错误建议
	switch components.Errors.Status {
	case "critical":
		recommendations = append(recommendations, "检测到严重错误，建议立即备份并更换设备")
	case "warning":
		recommendations = append(recommendations, "存在错误日志，建议运行完整诊断测试")
	}

	// 备用空间建议
	if info.AvailableSpare != nil && info.AvailableSpare.Percentage < 20 {
		recommendations = append(recommendations, "备用块空间不足，设备可能出现问题")
	}

	// 非安全关机建议
	if info.UnsafeShutdowns > 10 {
		recommendations = append(recommendations, fmt.Sprintf("检测到%d次非安全关机，建议检查电源稳定性", info.UnsafeShutdowns))
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "NVMe设备状态良好，继续保持定期备份")
	}

	return recommendations
}

// RunNVMeTest 运行NVMe测试.
func (m *NVMeMonitor) RunNVMeTest(device string, testType NVMeTestType) (*NVMeTestResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已有测试在运行
	if result, exists := m.testQueue[device]; exists && result.Status == "running" {
		return nil, fmt.Errorf("设备 %s 已有测试正在运行", device)
	}

	result := &NVMeTestResult{
		TestType:  testType,
		Device:    device,
		Status:    "running",
		StartTime: time.Now(),
	}

	m.testQueue[device] = result

	// 在后台运行测试
	go m.executeNVMeTest(device, testType, result)

	return result, nil
}

// executeNVMeTest 执行NVMe测试.
func (m *NVMeMonitor) executeNVMeTest(device string, testType NVMeTestType, result *NVMeTestResult) {
	defer func() {
		m.mu.Lock()
		delete(m.testQueue, device)
		m.mu.Unlock()
	}()

	var cmd *exec.Cmd
	switch testType {
	case NVMeTestShort:
		cmd = exec.CommandContext(context.Background(), "nvme", "device-self-test", device, "-n", "1")
	case NVMeTestLong:
		cmd = exec.CommandContext(context.Background(), "nvme", "device-self-test", device, "-n", "2")
	case NVMeTestVendor:
		cmd = exec.CommandContext(context.Background(), "nvme", "device-self-test", device, "-n", "0xe")
	default:
		cmd = exec.CommandContext(context.Background(), "nvme", "device-self-test", device, "-n", "1")
	}

	output, err := cmd.CombinedOutput()
	now := time.Now()
	result.EndTime = &now
	result.Duration = now.Sub(result.StartTime)

	if err != nil {
		result.Status = "failed"
		result.Result = "fail"
		result.ErrorMessage = string(output)
		return
	}

	// 获取测试结果
	testResult := m.getNVMeTestResult(device)
	if testResult != nil {
		result.Status = testResult.Status
		result.Result = testResult.Result
		result.NumErrors = testResult.NumErrors
		result.ErrorCode = testResult.ErrorCode
	} else {
		result.Status = "complete"
		result.Result = "pass"
	}
}

// getNVMeTestResult 获取NVMe测试结果.
func (m *NVMeMonitor) getNVMeTestResult(device string) *NVMeTestResult {
	cmd := exec.CommandContext(context.Background(), "nvme", "self-test-log", device, "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var log struct {
		CurrentOperation uint8 `json:"current_operation"`
		Completion       uint8 `json:"completion"`
		Results          []struct {
			Status           uint8  `json:"status"`
			Segment          uint8  `json:"segment"`
			ValidDiagnostics uint8  `json:"valid_diagnostic"`
			PowerOnHours     uint64 `json:"power_on_hours"`
		} `json:"results"`
	}

	if err := json.Unmarshal(output, &log); err != nil {
		return nil
	}

	if len(log.Results) == 0 {
		return nil
	}

	lastResult := log.Results[0]
	result := &NVMeTestResult{
		Progress: log.Completion,
	}

	// 解析状态码
	// NVMe规范: status & 0xf
	// 0 = 测试通过无错误
	// 1-7 = 不同程度的错误
	statusCode := lastResult.Status & 0x0F
	switch statusCode {
	case 0:
		result.Result = "pass"
		result.Status = "complete"
	case 1:
		result.Result = "fail"
		result.Status = "complete"
		result.ErrorMessage = "测试发现一个或多个致命错误"
	case 2:
		result.Result = "fail"
		result.Status = "complete"
		result.ErrorMessage = "测试发现未知错误"
	default:
		result.Result = "fail"
		result.Status = "complete"
		result.ErrorCode = statusCode
	}

	return result
}

// GetTestStatus 获取测试状态.
func (m *NVMeMonitor) GetTestStatus(device string) (*NVMeTestResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if result, exists := m.testQueue[device]; exists {
		return result, nil
	}

	// 检查设备是否支持自检
	result := m.getNVMeTestResult(device)
	if result != nil {
		return result, nil
	}

	return nil, fmt.Errorf("设备 %s 无测试结果", device)
}

// AbortTest 中止测试.
func (m *NVMeMonitor) AbortTest(device string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if result, exists := m.testQueue[device]; exists {
		result.Status = "aborted"
		delete(m.testQueue, device)
		return nil
	}

	return fmt.Errorf("设备 %s 无正在运行的测试", device)
}

// GetAllNVMeDevices 获取所有NVMe设备信息.
func (m *NVMeMonitor) GetAllNVMeDevices() ([]*NVMeHealthInfo, error) {
	devices, err := m.ScanNVMeDevices()
	if err != nil {
		return nil, err
	}

	infos := make([]*NVMeHealthInfo, 0, len(devices))
	for _, device := range devices {
		info, err := m.GetNVMeHealth(device)
		if err != nil {
			continue
		}
		infos = append(infos, info)
	}

	return infos, nil
}

// ClearCache 清除缓存.
func (m *NVMeMonitor) ClearCache(device string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device == "" {
		m.devices = make(map[string]*NVMeHealthInfo)
	} else {
		delete(m.devices, device)
	}
}

// ==================== v2.387.0 三级预警 + 寿命预测增强 ====================

// EvaluateAlertLevel 评估三级预警等级.
func (m *NVMeMonitor) EvaluateAlertLevel(info *NVMeHealthInfo) (AlertLevel, []string) {
	m.mu.RLock()
	thresholds := m.alertThresholds
	m.mu.RUnlock()

	var reasons []string
	alertLevel := AlertLevelNormal

	// 1. 温度预警检查
	if info.Temperature != nil {
		temp := info.Temperature.Current
		if temp >= thresholds.TempCritical {
			alertLevel = maxAlertLevel(alertLevel, AlertLevelCritical)
			reasons = append(reasons, fmt.Sprintf("温度严重过高: %d°C >= %d°C (Critical)", temp, thresholds.TempCritical))
		} else if temp >= thresholds.TempWarning {
			alertLevel = maxAlertLevel(alertLevel, AlertLevelWarning)
			reasons = append(reasons, fmt.Sprintf("温度偏高: %d°C >= %d°C (Warning)", temp, thresholds.TempWarning))
		}
	}

	// 2. 健康度预警检查
	healthPct := info.HealthPercentage
	if healthPct < thresholds.HealthEmergency {
		alertLevel = maxAlertLevel(alertLevel, AlertLevelEmergency)
		reasons = append(reasons, fmt.Sprintf("健康度紧急: %d%% < %d%% (Emergency)", healthPct, thresholds.HealthEmergency))
	} else if healthPct < thresholds.HealthCritical {
		alertLevel = maxAlertLevel(alertLevel, AlertLevelCritical)
		reasons = append(reasons, fmt.Sprintf("健康度严重: %d%% < %d%% (Critical)", healthPct, thresholds.HealthCritical))
	} else if healthPct < thresholds.HealthWarning {
		alertLevel = maxAlertLevel(alertLevel, AlertLevelWarning)
		reasons = append(reasons, fmt.Sprintf("健康度警告: %d%% < %d%% (Warning)", healthPct, thresholds.HealthWarning))
	}

	// 3. TBW写入量预警检查
	if info.Usage != nil && info.Usage.TBWUsedPercent > 0 {
		tbwPct := info.Usage.TBWUsedPercent
		if tbwPct >= float64(thresholds.TBWCritical) {
			alertLevel = maxAlertLevel(alertLevel, AlertLevelCritical)
			reasons = append(reasons, fmt.Sprintf("TBW写入量严重: %.1f%% >= %d%% (Critical)", tbwPct, thresholds.TBWCritical))
		} else if tbwPct >= float64(thresholds.TBWWarning) {
			alertLevel = maxAlertLevel(alertLevel, AlertLevelWarning)
			reasons = append(reasons, fmt.Sprintf("TBW写入量警告: %.1f%% >= %d%% (Warning)", tbwPct, thresholds.TBWWarning))
		}
	}

	// 4. 关键警告检查
	if info.CriticalWarnings > 0 {
		alertLevel = maxAlertLevel(alertLevel, AlertLevelCritical)
		reasons = append(reasons, fmt.Sprintf("检测到 %d 个关键警告标志", info.CriticalWarnings))
	}

	// 5. 媒体错误检查
	if info.MediaErrors > 100 {
		alertLevel = maxAlertLevel(alertLevel, AlertLevelCritical)
		reasons = append(reasons, fmt.Sprintf("媒体错误过多: %d 次", info.MediaErrors))
	} else if info.MediaErrors > 10 {
		alertLevel = maxAlertLevel(alertLevel, AlertLevelWarning)
		reasons = append(reasons, fmt.Sprintf("媒体错误: %d 次", info.MediaErrors))
	}

	// 6. 备用空间检查
	if info.AvailableSpare != nil {
		spare := info.AvailableSpare.Percentage
		threshold := info.AvailableSpare.Threshold
		if spare < threshold {
			alertLevel = maxAlertLevel(alertLevel, AlertLevelEmergency)
			reasons = append(reasons, fmt.Sprintf("备用空间低于阈值: %d%% < %d%%", spare, threshold))
		} else if spare < 20 {
			alertLevel = maxAlertLevel(alertLevel, AlertLevelWarning)
			reasons = append(reasons, fmt.Sprintf("备用空间偏低: %d%%", spare))
		}
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "所有指标正常")
	}

	return alertLevel, reasons
}

// maxAlertLevel 返回两个预警等级中更高的一个.
func maxAlertLevel(a, b AlertLevel) AlertLevel {
	levels := map[AlertLevel]int{
		AlertLevelNormal:    0,
		AlertLevelWarning:   1,
		AlertLevelCritical:  2,
		AlertLevelEmergency: 3,
	}
	if levels[b] > levels[a] {
		return b
	}
	return a
}

// PredictRemainingLife 预测NVMe剩余寿命.
// 基于写入量、温度历史、磨损程度综合计算.
func (m *NVMeMonitor) PredictRemainingLife(info *NVMeHealthInfo) *NVMeLifePrediction {
	if info.Usage == nil {
		return nil
	}

	prediction := &NVMeLifePrediction{
		PredictionLastUpdated: time.Now(),
		ConfidenceLevel:       "medium",
		TemperatureHistory:    []TempRecord{},
		WeeklyWriteRates:      []float64{},
	}

	// 获取或初始化历史数据
	m.historyMu.Lock()
	if m.tempHistory == nil {
		m.tempHistory = make(map[string][]TempRecord)
	}
	tempHist := m.tempHistory[info.Device]
	m.historyMu.Unlock()

	// 1. 计算写入量预测
	usage := info.Usage
	pctUsed := float64(usage.PercentageUsed)
	remainingPct := 100.0 - pctUsed

	// 估算TBW容量 (基于已使用百分比)
	var estimatedTBW float64
	if usage.TBW > 0 && pctUsed > 0 {
		estimatedTBW = usage.TBW / (pctUsed / 100.0)
	} else {
		// 根据容量估算 (通常消费级 1TB = 600TBW)
		estimatedTBW = float64(info.Size/1024/1024) * 600.0 / 1024.0 // 假设600TBW/1TB
	}

	// 保存估算TBW到Usage结构
	usage.EstimatedTBW = estimatedTBW

	// 计算TBW使用百分比
	if estimatedTBW > 0 {
		tbwPct := (usage.TBW / estimatedTBW) * 100.0
		prediction.RemainingLifePercent = math.Max(0, 100.0-tbwPct)
		usage.TBWUsedPercent = tbwPct
	} else {
		prediction.RemainingLifePercent = remainingPct
	}

	// 2. 计算温度影响系数 (高温会加速NAND磨损)
	var tempImpact = 1.0
	if info.Temperature != nil {
		temp := float64(info.Temperature.Current)
		// 温度在40°C以下影响最小，每升高10°C影响增加约15%
		if temp > 40 {
			tempImpact = 1.0 + ((temp-40.0)/10.0)*0.15
		}
		// 读取温度历史
		if len(tempHist) > 0 {
			var avgTemp float64
			var totalDuration float64
			for _, record := range tempHist {
				avgTemp += float64(record.Temp) * record.Duration
				totalDuration += record.Duration
			}
			if totalDuration > 0 {
				avgTemp /= totalDuration
				if avgTemp > 50 {
					tempImpact *= 1.0 + ((avgTemp-50.0)/10.0)*0.1
				}
			}
		}
	}
	prediction.TemperatureImpact = tempImpact

	// 3. 计算磨损程度影响系数
	wearImpact := 1.0 + (pctUsed/100.0)*0.5 // 使用越多，磨损加速
	prediction.WearImpact = wearImpact

	// 4. 计算平均每日写入量 (基于开机时间)
	var avgDailyWrites float64
	if info.PowerOnHours > 0 {
		totalWritesGB := usage.TotalWrites
		hours := float64(info.PowerOnHours)
		days := hours / 24.0
		if days > 0 {
			avgDailyWrites = totalWritesGB / days
		}
	}
	prediction.AverageDailyWrites = avgDailyWrites

	// 5. 估算写放大因子 (通常SSD写放大在1.1-3.0之间)
	// 简化计算：基于随机/顺序写入比例
	writeAmp := 1.5 // 默认假设
	if info.HostWriteCommands > 0 && info.HostReadCommands > 0 {
		// 读写比可能影响写放大
		ratio := float64(info.HostWriteCommands) / float64(info.HostReadCommands+info.HostWriteCommands)
		if ratio > 0.5 {
			writeAmp = 1.5 + (ratio-0.5)*2.0 // 高写入比例意味着更高的写放大
		}
	}
	prediction.WriteAmplificationFactor = writeAmp

	// 6. 综合计算剩余天数
	// 公式: 剩余TBW / (日均写入 * 写放大 * 温度影响 * 磨损影响)
	var estimatedDays int
	if avgDailyWrites > 0 {
		remainingTBW := estimatedTBW - usage.TBW
		if remainingTBW > 0 {
			effectiveDailyWrites := avgDailyWrites * writeAmp * tempImpact * wearImpact
			daysLeft := (remainingTBW * 1024) / effectiveDailyWrites // TB转GB计算
			estimatedDays = int(daysLeft)
			if estimatedDays < 0 {
				estimatedDays = 0
			}
		}
	} else {
		// 无写入数据，基于使用百分比估算
		// 假设SSD寿命3-5年
		estimatedDays = int(remainingPct / 100.0 * 365.0 * 4.0)
	}

	prediction.EstimatedDaysLeft = estimatedDays
	prediction.EstimatedEndDate = time.Now().AddDate(0, 0, estimatedDays)

	// 7. 确定置信度
	if info.PowerOnHours > 24*30 { // 超过1个月的数据
		prediction.ConfidenceLevel = "high"
	} else if info.PowerOnHours > 24*7 { // 超过1周的数据
		prediction.ConfidenceLevel = "medium"
	} else {
		prediction.ConfidenceLevel = "low"
	}

	// 8. 更新预估寿命描述
	switch {
	case prediction.RemainingLifePercent > 80:
		usage.EstimatedLife = ">5年"
	case prediction.RemainingLifePercent > 60:
		usage.EstimatedLife = "3-5年"
	case prediction.RemainingLifePercent > 40:
		usage.EstimatedLife = "1-3年"
	case prediction.RemainingLifePercent > 20:
		usage.EstimatedLife = "6-12月"
	case prediction.RemainingLifePercent > 10:
		usage.EstimatedLife = "3-6月"
	default:
		usage.EstimatedLife = "<3月"
	}

	// 缓存预测结果
	m.predictionMu.Lock()
	m.lifePredictions[info.Device] = prediction
	m.predictionMu.Unlock()

	return prediction
}

// RecordTemperature 记录温度历史 (用于寿命预测).
func (m *NVMeMonitor) RecordTemperature(device string, temp uint8, durationMinutes float64) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()

	record := TempRecord{
		Timestamp: time.Now(),
		Temp:      temp,
		Duration:  durationMinutes,
	}

	// 保留最近7天的数据
	maxRecords := 7 * 24 * 60 / 5 // 每5分钟一条，7天
	records := m.tempHistory[device]
	records = append(records, record)
	if len(records) > maxRecords {
		records = records[len(records)-maxRecords:]
	}
	m.tempHistory[device] = records
}

// GetLifePrediction 获取设备的寿命预测.
func (m *NVMeMonitor) GetLifePrediction(device string) *NVMeLifePrediction {
	m.predictionMu.RLock()
	defer m.predictionMu.RUnlock()
	return m.lifePredictions[device]
}

// GetAlertSummary 获取所有NVMe设备的预警摘要.
func (m *NVMeMonitor) GetAlertSummary() map[string]struct {
	Level     AlertLevel
	Reasons   []string
	HealthPct uint8
} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := make(map[string]struct {
		Level     AlertLevel
		Reasons   []string
		HealthPct uint8
	})

	for device, info := range m.devices {
		level, reasons := m.EvaluateAlertLevel(info)
		summary[device] = struct {
			Level     AlertLevel
			Reasons   []string
			HealthPct uint8
		}{
			Level:     level,
			Reasons:   reasons,
			HealthPct: info.HealthPercentage,
		}
	}

	return summary
}

// GetTemperatureHistory 获取设备温度历史.
func (m *NVMeMonitor) GetTemperatureHistory(device string) []TempRecord {
	m.historyMu.RLock()
	defer m.historyMu.RUnlock()

	history := m.tempHistory[device]
	if history == nil {
		return []TempRecord{}
	}

	// 返回副本
	result := make([]TempRecord, len(history))
	copy(result, history)
	return result
}
