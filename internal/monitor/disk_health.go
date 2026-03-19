// Package monitor 提供磁盘健康监控功能
// 实现 SMART 数据解析和磁盘健康评估
package monitor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DiskHealthMonitor 磁盘健康监控器
type DiskHealthMonitor struct {
	mu           sync.RWMutex
	disks        map[string]*DiskHealthInfo
	alertManager *AlertingManager
	checkTicker  *time.Ticker
	stopCh       chan struct{}
}

// DiskHealthInfo 磁盘健康信息
type DiskHealthInfo struct {
	Device          string                    `json:"device"`
	Model           string                    `json:"model"`
	SerialNumber    string                    `json:"serialNumber"`
	FirmwareVersion string                    `json:"firmwareVersion"`
	Capacity        uint64                    `json:"capacity"`     // 字节
	RotationRate    int                       `json:"rotationRate"` // RPM, SSD 为 0
	IsSSD           bool                      `json:"isSSD"`
	Temperature     int                       `json:"temperature"` // 摄氏度
	HealthStatus    HealthStatus              `json:"healthStatus"`
	HealthScore     int                       `json:"healthScore"` // 0-100
	PowerOnHours    uint64                    `json:"powerOnHours"`
	PowerCycleCount uint64                    `json:"powerCycleCount"`
	SmartAttributes map[string]SMARTAttribute `json:"smartAttributes"`
	LastCheck       time.Time                 `json:"lastCheck"`
	Errors          []DiskError               `json:"errors,omitempty"`
	Warnings        []string                  `json:"warnings,omitempty"`
}

// HealthStatus 健康状态
type HealthStatus string

// 健康状态常量
const (
	// HealthStatusHealthy 健康
	HealthStatusHealthy  HealthStatus = "healthy"
	// HealthStatusWarning 警告
	HealthStatusWarning  HealthStatus = "warning"
	// HealthStatusDegraded 性能下降
	HealthStatusDegraded HealthStatus = "degraded"
	// HealthStatusFailed 失败
	HealthStatusFailed   HealthStatus = "failed"
	// HealthStatusUnknown 未知
	HealthStatusUnknown  HealthStatus = "unknown"
)

// SMARTAttribute SMART 属性
type SMARTAttribute struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Value       int    `json:"value"`
	Worst       int    `json:"worst"`
	Threshold   int    `json:"threshold"`
	RawValue    uint64 `json:"rawValue"`
	Unit        string `json:"unit"`
	Description string `json:"description"`
	IsCritical  bool   `json:"isCritical"`
}

// DiskError 磁盘错误
type DiskError struct {
	Type      string    `json:"type"` // smart, io, temperature, etc.
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Count     uint64    `json:"count"`
}

// SMARTThresholdConfig SMART 阈值配置
type SMARTThresholdConfig struct {
	// 温度阈值
	TempWarning  int `json:"tempWarning"`  // 温度警告阈值
	TempCritical int `json:"tempCritical"` // 温度严重阈值

	// SMART 属性阈值
	ReallocatedSectorWarning  int `json:"reallocatedSectorWarning"`
	ReallocatedSectorCritical int `json:"reallocatedSectorCritical"`
	PendingSectorWarning      int `json:"pendingSectorWarning"`
	PendingSectorCritical     int `json:"pendingSectorCritical"`
	SeekErrorWarning          int `json:"seekErrorWarning"`
	UDMACRCErrorWarning       int `json:"udmaCrcErrorWarning"`

	// 健康评分权重
	AttributeWeights map[string]float64 `json:"attributeWeights"`
}

// DefaultSMARTThresholds 默认 SMART 阈值
func DefaultSMARTThresholds() *SMARTThresholdConfig {
	return &SMARTThresholdConfig{
		TempWarning:               50,
		TempCritical:              60,
		ReallocatedSectorWarning:  10,
		ReallocatedSectorCritical: 100,
		PendingSectorWarning:      10,
		PendingSectorCritical:     100,
		SeekErrorWarning:          100,
		UDMACRCErrorWarning:       100,
		AttributeWeights: map[string]float64{
			"Reallocated_Sector_Ct":   0.25,
			"Reallocated_Event_Count": 0.15,
			"Current_Pending_Sector":  0.20,
			"Offline_Uncorrectable":   0.15,
			"UDMA_CRC_Error_Count":    0.10,
			"Temperature_Celsius":     0.10,
			"Power_On_Hours":          0.05,
		},
	}
}

// NewDiskHealthMonitor 创建磁盘健康监控器
func NewDiskHealthMonitor(alertMgr *AlertingManager) *DiskHealthMonitor {
	return &DiskHealthMonitor{
		disks:        make(map[string]*DiskHealthInfo),
		alertManager: alertMgr,
		stopCh:       make(chan struct{}),
	}
}

// Start 启动监控
func (m *DiskHealthMonitor) Start(checkInterval time.Duration) {
	m.checkTicker = time.NewTicker(checkInterval)
	go func() {
		// 立即执行一次检查
		_ = m.CheckAllDisks()

		for {
			select {
			case <-m.checkTicker.C:
				_ = m.CheckAllDisks()
			case <-m.stopCh:
				return
			}
		}
	}()
}

// Stop 停止监控
func (m *DiskHealthMonitor) Stop() {
	if m.checkTicker != nil {
		m.checkTicker.Stop()
	}
	close(m.stopCh)
}

// CheckAllDisks 检查所有磁盘
func (m *DiskHealthMonitor) CheckAllDisks() error {
	devices, err := m.listBlockDevices()
	if err != nil {
		return fmt.Errorf("列出块设备失败: %w", err)
	}

	for _, device := range devices {
		info, err := m.checkDisk(device)
		if err != nil {
			continue
		}

		m.mu.Lock()
		m.disks[device] = info
		m.mu.Unlock()

		// 触发告警
		m.evaluateHealthAlerts(info)
	}

	return nil
}

// GetDiskHealth 获取磁盘健康信息
func (m *DiskHealthMonitor) GetDiskHealth(device string) (*DiskHealthInfo, error) {
	m.mu.RLock()
	info, exists := m.disks[device]
	m.mu.RUnlock()

	if exists {
		return info, nil
	}

	// 如果不在缓存中，立即检查
	return m.checkDisk(device)
}

// GetAllDisksHealth 获取所有磁盘健康信息
func (m *DiskHealthMonitor) GetAllDisksHealth() []*DiskHealthInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*DiskHealthInfo, 0, len(m.disks))
	for _, info := range m.disks {
		result = append(result, info)
	}
	return result
}

// checkDisk 检查单个磁盘
func (m *DiskHealthMonitor) checkDisk(device string) (*DiskHealthInfo, error) {
	info := &DiskHealthInfo{
		Device:          device,
		SmartAttributes: make(map[string]SMARTAttribute),
		Errors:          make([]DiskError, 0),
		Warnings:        make([]string, 0),
		LastCheck:       time.Now(),
	}

	// 检查 smartctl 是否可用
	if _, err := exec.LookPath("smartctl"); err != nil {
		return nil, fmt.Errorf("smartctl 未安装")
	}

	// 获取基本信息
	if err := m.getBasicInfo(info); err != nil {
		return nil, err
	}

	// 获取 SMART 属性
	if err := m.getSMARTAttributes(info); err != nil {
		return nil, err
	}

	// 计算健康评分
	m.calculateHealthScore(info)

	return info, nil
}

// getBasicInfo 获取基本信息
func (m *DiskHealthMonitor) getBasicInfo(info *DiskHealthInfo) error {
	cmd := exec.Command("smartctl", "-i", info.Device)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("获取磁盘信息失败: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()

		// 解析型号
		if strings.HasPrefix(line, "Device Model:") || strings.HasPrefix(line, "Model Number:") {
			info.Model = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}

		// 解析序列号
		if strings.HasPrefix(line, "Serial Number:") {
			info.SerialNumber = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}

		// 解析固件版本
		if strings.HasPrefix(line, "Firmware Version:") {
			info.FirmwareVersion = strings.TrimSpace(strings.SplitN(line, ":", 2)[1])
		}

		// 解析容量
		if strings.HasPrefix(line, "User Capacity:") {
			re := regexp.MustCompile(`(\d+)`)
			matches := re.FindAllString(line, -1)
			if len(matches) > 0 {
				capacity, _ := strconv.ParseUint(strings.Join(matches, ""), 10, 64)
				info.Capacity = capacity
			}
		}

		// 解析转速
		if strings.Contains(line, "Rotation Rate:") {
			if strings.Contains(line, "Solid State Device") {
				info.IsSSD = true
				info.RotationRate = 0
			} else {
				re := regexp.MustCompile(`(\d+)`)
				match := re.FindString(line)
				if match != "" {
					info.RotationRate, _ = strconv.Atoi(match)
				}
			}
		}

		// 判断是否为 SSD
		if strings.Contains(line, "Solid State Device") {
			info.IsSSD = true
		}
	}

	return nil
}

// getSMARTAttributes 获取 SMART 属性
func (m *DiskHealthMonitor) getSMARTAttributes(info *DiskHealthInfo) error {
	// 获取 SMART 属性
	cmd := exec.Command("smartctl", "-A", "-H", info.Device)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("获取 SMART 属性失败: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	inAttributes := false

	// 关键属性定义
	criticalAttrs := map[int]bool{
		5:   true, // Reallocated_Sector_Ct
		10:  true, // Spin_Retry_Count
		184: true, // End-to-End_Error
		187: true, // Reported_Uncorrect
		188: true, // Command_Timeout
		196: true, // Reallocated_Event_Count
		197: true, // Current_Pending_Sector
		198: true, // Offline_Uncorrectable
		201: true, // Soft_Read_Error_Rate
	}

	for scanner.Scan() {
		line := scanner.Text()

		// 检测健康状态
		if strings.Contains(line, "SMART overall-health self-assessment test result:") {
			if strings.Contains(line, "PASSED") {
				info.HealthStatus = HealthStatusHealthy
			} else if strings.Contains(line, "FAILED") {
				info.HealthStatus = HealthStatusFailed
			} else {
				info.HealthStatus = HealthStatusUnknown
			}
		}

		// 检测属性表开始
		if strings.HasPrefix(line, "ID#") || strings.Contains(line, "ATTRIBUTE_NAME") {
			inAttributes = true
			continue
		}

		if !inAttributes {
			continue
		}

		// 解析属性行
		fields := strings.Fields(line)
		if len(fields) >= 10 {
			id, err := strconv.Atoi(fields[0])
			if err != nil {
				continue
			}

			value, _ := strconv.Atoi(fields[2])
			worst, _ := strconv.Atoi(fields[3])
			threshold, _ := strconv.Atoi(fields[4])

			// 解析原始值
			rawValueStr := fields[9]
			// 提取数字部分
			re := regexp.MustCompile(`^(\d+)`)
			match := re.FindString(rawValueStr)
			rawValue, _ := strconv.ParseUint(match, 10, 64)

			name := fields[1]

			attr := SMARTAttribute{
				ID:         id,
				Name:       name,
				Value:      value,
				Worst:      worst,
				Threshold:  threshold,
				RawValue:   rawValue,
				IsCritical: criticalAttrs[id],
			}

			// 特殊处理温度
			if name == "Temperature_Celsius" || name == "Airflow_Temperature_Cel" {
				info.Temperature = int(rawValue)
				attr.Unit = "°C"
			}

			// 特殊处理通电时间
			if name == "Power_On_Hours" || name == "Power_On_Hours_and_Msec" {
				info.PowerOnHours = rawValue
				attr.Unit = "hours"
			}

			// 特殊处理通电次数
			if name == "Power_Cycle_Count" {
				info.PowerCycleCount = rawValue
			}

			info.SmartAttributes[name] = attr
		}
	}

	return nil
}

// calculateHealthScore 计算健康评分
func (m *DiskHealthMonitor) calculateHealthScore(info *DiskHealthInfo) {
	thresholds := DefaultSMARTThresholds()
	score := 100.0

	// 检查温度
	if info.Temperature > thresholds.TempCritical {
		score -= 20
		info.Warnings = append(info.Warnings, fmt.Sprintf("温度过高: %d°C (严重阈值: %d°C)", info.Temperature, thresholds.TempCritical))
	} else if info.Temperature > thresholds.TempWarning {
		score -= 10
		info.Warnings = append(info.Warnings, fmt.Sprintf("温度偏高: %d°C (警告阈值: %d°C)", info.Temperature, thresholds.TempWarning))
	}

	// 检查重分配扇区
	if attr, ok := info.SmartAttributes["Reallocated_Sector_Ct"]; ok {
		if attr.RawValue > uint64(thresholds.ReallocatedSectorCritical) {
			score -= 30
			info.Errors = append(info.Errors, DiskError{
				Type:      "smart",
				Code:      "reallocated_sectors",
				Message:   fmt.Sprintf("重分配扇区数过多: %d", attr.RawValue),
				Timestamp: time.Now(),
				Count:     attr.RawValue,
			})
		} else if attr.RawValue > uint64(thresholds.ReallocatedSectorWarning) {
			score -= 15
			info.Warnings = append(info.Warnings, fmt.Sprintf("重分配扇区: %d", attr.RawValue))
		}
	}

	// 检查待定扇区
	if attr, ok := info.SmartAttributes["Current_Pending_Sector"]; ok {
		if attr.RawValue > uint64(thresholds.PendingSectorCritical) {
			score -= 25
			info.Errors = append(info.Errors, DiskError{
				Type:      "smart",
				Code:      "pending_sectors",
				Message:   fmt.Sprintf("待定扇区数过多: %d", attr.RawValue),
				Timestamp: time.Now(),
				Count:     attr.RawValue,
			})
		} else if attr.RawValue > uint64(thresholds.PendingSectorWarning) {
			score -= 10
			info.Warnings = append(info.Warnings, fmt.Sprintf("待定扇区: %d", attr.RawValue))
		}
	}

	// 检查 UDMA CRC 错误
	if attr, ok := info.SmartAttributes["UDMA_CRC_Error_Count"]; ok {
		if attr.RawValue > uint64(thresholds.UDMACRCErrorWarning) {
			score -= 10
			info.Warnings = append(info.Warnings, fmt.Sprintf("UDMA CRC 错误: %d", attr.RawValue))
		}
	}

	// 确保分数在 0-100 之间
	if score < 0 {
		score = 0
	}
	info.HealthScore = int(score)

	// 更新健康状态
	if info.HealthScore >= 90 {
		if info.HealthStatus != HealthStatusFailed {
			info.HealthStatus = HealthStatusHealthy
		}
	} else if info.HealthScore >= 70 {
		info.HealthStatus = HealthStatusWarning
	} else if info.HealthScore >= 40 {
		info.HealthStatus = HealthStatusDegraded
	} else {
		info.HealthStatus = HealthStatusFailed
	}
}

// evaluateHealthAlerts 评估健康告警
func (m *DiskHealthMonitor) evaluateHealthAlerts(info *DiskHealthInfo) {
	if m.alertManager == nil {
		return
	}

	// 严重告警
	if info.HealthStatus == HealthStatusFailed {
		m.alertManager.triggerAlert(
			"disk_health",
			"critical",
			fmt.Sprintf("磁盘 %s 健康状态异常，请立即更换", info.Device),
			info.Device,
			map[string]interface{}{
				"health_score": info.HealthScore,
				"errors":       info.Errors,
				"model":        info.Model,
				"serial":       info.SerialNumber,
			},
		)
		return
	}

	// 警告
	if info.HealthStatus == HealthStatusDegraded {
		m.alertManager.triggerAlert(
			"disk_health",
			"warning",
			fmt.Sprintf("磁盘 %s 性能下降，建议检查", info.Device),
			info.Device,
			map[string]interface{}{
				"health_score": info.HealthScore,
				"warnings":     info.Warnings,
			},
		)
	}

	// 温度告警
	if info.Temperature > 60 {
		m.alertManager.triggerAlert(
			"disk_temperature",
			"critical",
			fmt.Sprintf("磁盘 %s 温度过高: %d°C", info.Device, info.Temperature),
			info.Device,
			map[string]interface{}{
				"temperature": info.Temperature,
			},
		)
	} else if info.Temperature > 50 {
		m.alertManager.triggerAlert(
			"disk_temperature",
			"warning",
			fmt.Sprintf("磁盘 %s 温度偏高: %d°C", info.Device, info.Temperature),
			info.Device,
			map[string]interface{}{
				"temperature": info.Temperature,
			},
		)
	}
}

// listBlockDevices 列出块设备
func (m *DiskHealthMonitor) listBlockDevices() ([]string, error) {
	var devices []string

	// 使用 lsblk 列出设备
	cmd := exec.Command("lsblk", "-d", "-n", "-o", "NAME,TYPE")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法列出块设备: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == "disk" {
			device := "/dev/" + fields[0]
			devices = append(devices, device)
		}
	}

	return devices, nil
}

// RunShortTest 运行短测试
func (m *DiskHealthMonitor) RunShortTest(device string) error {
	cmd := exec.Command("smartctl", "-t", "short", device)
	return cmd.Run()
}

// RunLongTest 运行长测试
func (m *DiskHealthMonitor) RunLongTest(device string) error {
	cmd := exec.Command("smartctl", "-t", "long", device)
	return cmd.Run()
}

// GetTestStatus 获取测试状态
func (m *DiskHealthMonitor) GetTestStatus(device string) (string, error) {
	cmd := exec.Command("smartctl", "-l", "selftest", device)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// 解析测试状态
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Self-test execution status:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Self-test execution status:")), nil
		}
	}

	return "unknown", nil
}

// ExportSMARTData 导出 SMART 数据为 JSON
func (m *DiskHealthMonitor) ExportSMARTData(device string) ([]byte, error) {
	info, err := m.GetDiskHealth(device)
	if err != nil {
		return nil, err
	}

	return json.MarshalIndent(info, "", "  ")
}

// GetDiskSummary 获取磁盘摘要
func (m *DiskHealthMonitor) GetDiskSummary() *DiskHealthSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &DiskHealthSummary{
		TotalDisks:  len(m.disks),
		Healthy:     0,
		Warning:     0,
		Degraded:    0,
		Failed:      0,
		Unknown:     0,
		AvgTemp:     0,
		TotalErrors: 0,
	}

	totalTemp := 0
	tempCount := 0

	for _, disk := range m.disks {
		switch disk.HealthStatus {
		case HealthStatusHealthy:
			summary.Healthy++
		case HealthStatusWarning:
			summary.Warning++
		case HealthStatusDegraded:
			summary.Degraded++
		case HealthStatusFailed:
			summary.Failed++
		default:
			summary.Unknown++
		}

		if disk.Temperature > 0 {
			totalTemp += disk.Temperature
			tempCount++
		}

		summary.TotalErrors += len(disk.Errors)
	}

	if tempCount > 0 {
		summary.AvgTemp = totalTemp / tempCount
	}

	return summary
}

// DiskHealthSummary 磁盘健康摘要
type DiskHealthSummary struct {
	TotalDisks  int `json:"totalDisks"`
	Healthy     int `json:"healthy"`
	Warning     int `json:"warning"`
	Degraded    int `json:"degraded"`
	Failed      int `json:"failed"`
	Unknown     int `json:"unknown"`
	AvgTemp     int `json:"avgTemp"`
	TotalErrors int `json:"totalErrors"`
}
