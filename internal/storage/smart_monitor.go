// Package storage 提供存储管理功能
package storage

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"nas-os/pkg/safeguards"
)

// SMARTMonitor SMART 磁盘健康监控器
type SMARTMonitor struct {
	mu sync.RWMutex

	// 磁盘监控状态
	disks map[string]*DiskHealth

	// 配置
	config SMARTConfig

	// 告警回调
	alertHandlers []AlertHandler

	// 历史数据
	history map[string][]HealthSnapshot

	// 停止信号
	stopChan chan struct{}
}

// SMARTConfig SMART 监控配置
type SMARTConfig struct {
	// 检查间隔
	CheckInterval time.Duration
	// 温度警告阈值（摄氏度）
	TempWarningThreshold int
	// 温度严重阈值
	TempCriticalThreshold int
	// 重分配扇区警告阈值
	ReallocatedWarning int
	// 重分配扇区严重阈值
	ReallocatedCritical int
	// 待映射扇区警告阈值
	PendingWarning int
	// 待映射扇区严重阈值
	PendingCritical int
	// 寻道错误率警告阈值（百分比）
	SeekErrorWarning float64
	// 启用自动检查
	AutoCheck bool
	// 历史记录保留天数
	HistoryRetentionDays int
}

// DefaultSMARTConfig 默认 SMART 配置
var DefaultSMARTConfig = SMARTConfig{
	CheckInterval:         30 * time.Minute,
	TempWarningThreshold:  50,
	TempCriticalThreshold: 60,
	ReallocatedWarning:    10,
	ReallocatedCritical:   100,
	PendingWarning:        10,
	PendingCritical:       100,
	SeekErrorWarning:      5.0,
	AutoCheck:             true,
	HistoryRetentionDays:  30,
}

// DiskHealth 磁盘健康状态
type DiskHealth struct {
	Device          string `json:"device"`
	Model           string `json:"model"`
	Serial          string `json:"serial"`
	Size            uint64 `json:"size"` // 字节
	Temperature     int    `json:"temperature"`
	PowerOnHours    uint64 `json:"powerOnHours"`
	PowerCycleCount uint64 `json:"powerCycleCount"`

	// SMART 属性
	ReallocatedSectors   uint64  `json:"reallocatedSectors"`
	PendingSectors       uint64  `json:"pendingSectors"`
	OfflineUncorrectable uint64  `json:"offlineUncorrectable"`
	UDMACRCErrorCount    uint64  `json:"udmaCrcErrorCount"`
	SeekErrorRate        float64 `json:"seekErrorRate"`
	ReadErrorRate        float64 `json:"readErrorRate"`
	WriteErrorRate       float64 `json:"writeErrorRate"`

	// 状态
	SMARTStatus      SMARTStatus  `json:"smartStatus"`
	HealthStatus     HealthStatus `json:"healthStatus"`
	HealthScore      int          `json:"healthScore"` // 0-100
	LastCheckTime    time.Time    `json:"lastCheckTime"`
	LastAlertTime    time.Time    `json:"lastAlertTime"`
	AlertCount       int          `json:"alertCount"`
	PredictedFailure bool         `json:"predictedFailure"`
	FailurePredicted string       `json:"failurePredicted"` // 预测失败原因

	// NVMe 特有属性
	NVMeAvailableSpare   int    `json:"nvmeAvailableSpare"`   // 可用备用空间百分比
	NVMePercentageUsed   int    `json:"nvmePercentageUsed"`   // 使用百分比
	NVMeDataUnitsRead    uint64 `json:"nvmeDataUnitsRead"`    // 读取数据单元
	NVMeDataUnitsWritten uint64 `json:"nvmeDataUnitsWritten"` // 写入数据单元

	// 原始 SMART 属性
	Attributes map[string]SMARTAttribute `json:"attributes"`
}

// SMARTAttribute SMART 属性
type SMARTAttribute struct {
	ID          uint8  `json:"id"`
	Name        string `json:"name"`
	Value       uint8  `json:"value"`
	Worst       uint8  `json:"worst"`
	Threshold   uint8  `json:"threshold"`
	RawValue    uint64 `json:"rawValue"`
	Normalized  int    `json:"normalized"`
	Description string `json:"description"`
}

// SMARTStatus SMART 状态
type SMARTStatus string

const (
	SMARTStatusPASSED      SMARTStatus = "PASSED"
	SMARTStatusWARNING     SMARTStatus = "WARNING"
	SMARTStatusFAILING     SMARTStatus = "FAILING"
	SMARTStatusUNKNOWN     SMARTStatus = "UNKNOWN"
	SMARTStatusUNSUPPORTED SMARTStatus = "UNSUPPORTED"
)

// HealthStatus 健康状态
type HealthStatus string

const (
	HealthStatusExcellent HealthStatus = "EXCELLENT" // 90-100
	HealthStatusGood      HealthStatus = "GOOD"      // 70-89
	HealthStatusFair      HealthStatus = "FAIR"      // 50-69
	HealthStatusPoor      HealthStatus = "POOR"      // 25-49
	HealthStatusCritical  HealthStatus = "CRITICAL"  // 0-24
)

// HealthSnapshot 健康快照
type HealthSnapshot struct {
	Timestamp          time.Time    `json:"timestamp"`
	Temperature        int          `json:"temperature"`
	ReallocatedSectors uint64       `json:"reallocatedSectors"`
	PendingSectors     uint64       `json:"pendingSectors"`
	HealthScore        int          `json:"healthScore"`
	HealthStatus       HealthStatus `json:"healthStatus"`
}

// AlertType 告警类型
type AlertType string

const (
	AlertTypeTemperature  AlertType = "TEMPERATURE"
	AlertTypeReallocated  AlertType = "REALLOCATED_SECTORS"
	AlertTypePending      AlertType = "PENDING_SECTORS"
	AlertTypePredictFail  AlertType = "PREDICTED_FAILURE"
	AlertTypeSMARTFailure AlertType = "SMART_FAILURE"
	AlertTypeCRCError     AlertType = "CRC_ERROR"
	AlertTypeSeekError    AlertType = "SEEK_ERROR"
)

// Alert 告警
type Alert struct {
	Type        AlertType   `json:"type"`
	Device      string      `json:"device"`
	Severity    string      `json:"severity"` // INFO, WARNING, CRITICAL
	Message     string      `json:"message"`
	Value       interface{} `json:"value"`
	Threshold   interface{} `json:"threshold"`
	Timestamp   time.Time   `json:"timestamp"`
	HealthScore int         `json:"healthScore"`
}

// AlertHandler 告警处理器
type AlertHandler func(alert Alert)

// NewSMARTMonitor 创建 SMART 监控器
func NewSMARTMonitor(config SMARTConfig) *SMARTMonitor {
	if config.CheckInterval <= 0 {
		config.CheckInterval = DefaultSMARTConfig.CheckInterval
	}
	if config.TempWarningThreshold <= 0 {
		config.TempWarningThreshold = DefaultSMARTConfig.TempWarningThreshold
	}
	if config.TempCriticalThreshold <= 0 {
		config.TempCriticalThreshold = DefaultSMARTConfig.TempCriticalThreshold
	}

	return &SMARTMonitor{
		disks:         make(map[string]*DiskHealth),
		config:        config,
		alertHandlers: make([]AlertHandler, 0),
		history:       make(map[string][]HealthSnapshot),
		stopChan:      make(chan struct{}),
	}
}

// Start 启动监控
func (m *SMARTMonitor) Start() error {
	// 初始检查
	if err := m.CheckAll(); err != nil {
		return fmt.Errorf("初始检查失败: %w", err)
	}

	// 启动定期检查
	if m.config.AutoCheck {
		go m.checkLoop()
	}

	return nil
}

// Stop 停止监控
func (m *SMARTMonitor) Stop() {
	close(m.stopChan)
}

// checkLoop 定期检查循环
func (m *SMARTMonitor) checkLoop() {
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := m.CheckAll(); err != nil {
				log.Printf("SMART 检查失败: %v", err)
			}
		case <-m.stopChan:
			return
		}
	}
}

// CheckAll 检查所有磁盘
func (m *SMARTMonitor) CheckAll() error {
	// 获取所有磁盘
	disks, err := m.detectDisks()
	if err != nil {
		return fmt.Errorf("检测磁盘失败: %w", err)
	}

	for _, device := range disks {
		if err := m.CheckDevice(device); err != nil {
			// 记录错误但继续检查其他磁盘
			continue
		}
	}

	return nil
}

// detectDisks 检测系统磁盘
func (m *SMARTMonitor) detectDisks() ([]string, error) {
	var disks []string

	// 使用 lsblk 或 /proc/partitions 检测
	cmd := exec.Command("lsblk", "-d", "-o", "NAME", "-n")
	output, err := cmd.Output()
	if err != nil {
		// 回退到读取 /proc/partitions
		data, err := exec.Command("cat", "/proc/partitions").Output()
		if err != nil {
			return nil, err
		}
		lines := strings.Split(string(data), "\n")
		for i, line := range lines {
			if i < 2 { // 跳过头部
				continue
			}
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				name := fields[2]
				// 只取主设备（不包含分区号）
				if isMainDevice(name) {
					disks = append(disks, "/dev/"+name)
				}
			}
		}
	} else {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if isMainDevice(line) {
				disks = append(disks, "/dev/"+line)
			}
		}
	}

	return disks, nil
}

// isMainDevice 判断是否为主设备（不含分区）
func isMainDevice(name string) bool {
	// SATA/SAS: sda, sdb, etc.
	// NVMe: nvme0n1, nvme1n1, etc.
	// MMC: mmcblk0, mmcblk1, etc.

	// 排除分区 (sda1, nvme0n1p1, mmcblk0p1)
	if strings.Contains(name, "p") && len(name) > 4 {
		return false
	}

	// 检查是否是主设备模式
	sataPattern := regexp.MustCompile(`^sd[a-z]$`)
	nvmePattern := regexp.MustCompile(`^nvme\d+n\d+$`)
	mmcPattern := regexp.MustCompile(`^mmcblk\d+$`)
	vdPattern := regexp.MustCompile(`^vd[a-z]$`) // 虚拟磁盘

	return sataPattern.MatchString(name) ||
		nvmePattern.MatchString(name) ||
		mmcPattern.MatchString(name) ||
		vdPattern.MatchString(name)
}

// CheckDevice 检查指定设备
func (m *SMARTMonitor) CheckDevice(device string) error {
	// 判断是否为 NVMe
	isNVMe := strings.Contains(device, "nvme")

	var health *DiskHealth
	var err error

	if isNVMe {
		health, err = m.checkNVMeDevice(device)
	} else {
		health, err = m.checkSATADevice(device)
	}

	if err != nil {
		return err
	}

	// 计算健康分数
	m.calculateHealthScore(health)

	// 检查告警
	m.checkAlerts(health)

	// 保存快照
	m.saveSnapshot(health)

	// 更新状态
	m.mu.Lock()
	m.disks[device] = health
	m.mu.Unlock()

	return nil
}

// checkSATADevice 检查 SATA/SAS 设备
func (m *SMARTMonitor) checkSATADevice(device string) (*DiskHealth, error) {
	health := &DiskHealth{
		Device:        device,
		Attributes:    make(map[string]SMARTAttribute),
		LastCheckTime: time.Now(),
	}

	// 验证设备路径（防止命令注入）
	if device == "" || strings.ContainsAny(device, ";|&$`()<>") {
		return nil, fmt.Errorf("无效的设备路径")
	}
	if !strings.HasPrefix(device, "/dev/") {
		return nil, fmt.Errorf("设备路径必须以 /dev/ 开头")
	}

	// 获取 SMART 信息
	// #nosec G204 -- 设备路径已验证
	cmd := exec.Command("smartctl", "-a", device)
	output, err := cmd.Output()
	if err != nil {
		// smartctl 可能返回非零退出码即使有输出
		if len(output) == 0 {
			health.SMARTStatus = SMARTStatusUNSUPPORTED
			return health, nil
		}
	}

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	// 解析基本信息
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 模型
		if strings.HasPrefix(line, "Device Model:") || strings.HasPrefix(line, "Model Family:") {
			health.Model = strings.TrimSpace(strings.TrimPrefix(line, "Device Model:"))
			if strings.HasPrefix(line, "Model Family:") {
				health.Model = strings.TrimSpace(strings.TrimPrefix(line, "Model Family:"))
			}
		}

		// 序列号
		if strings.HasPrefix(line, "Serial Number:") {
			health.Serial = strings.TrimSpace(strings.TrimPrefix(line, "Serial Number:"))
		}

		// 容量
		if strings.HasPrefix(line, "User Capacity:") {
			health.Size = parseCapacity(line)
		}

		// SMART 状态
		if strings.Contains(line, "SMART overall-health self-assessment") {
			if strings.Contains(line, "PASSED") {
				health.SMARTStatus = SMARTStatusPASSED
			} else if strings.Contains(line, "FAILED") {
				health.SMARTStatus = SMARTStatusFAILING
			}
		}
	}

	// 解析 SMART 属性
	health.Attributes = m.parseSMARTAttributes(outputStr)

	// 提取关键属性
	for name, attr := range health.Attributes {
		switch name {
		case "Temperature_Celsius", "Temperature":
			// 安全转换：温度值通常在 0-100 范围内
			if attr.RawValue > 1000 {
				health.Temperature = 1000 // 异常值，设置上限
			} else if val, err := safeguards.SafeUint64ToInt(attr.RawValue); err == nil {
				health.Temperature = val
			} else {
				health.Temperature = math.MaxInt // 溢出时设置为最大值
			}
		case "Power_On_Hours":
			health.PowerOnHours = attr.RawValue
		case "Power_Cycle_Count":
			health.PowerCycleCount = attr.RawValue
		case "Reallocated_Sector_Ct":
			health.ReallocatedSectors = attr.RawValue
		case "Current_Pending_Sector":
			health.PendingSectors = attr.RawValue
		case "Offline_Uncorrectable":
			health.OfflineUncorrectable = attr.RawValue
		case "UDMA_CRC_Error_Count", "CRC_Error_Count":
			health.UDMACRCErrorCount = attr.RawValue
		case "Seek_Error_Rate":
			health.SeekErrorRate = float64(attr.RawValue)
		case "Read_Error_Rate", "Raw_Read_Error_Rate":
			health.ReadErrorRate = float64(attr.RawValue)
		case "Write_Error_Rate":
			health.WriteErrorRate = float64(attr.RawValue)
		}
	}

	return health, nil
}

// checkNVMeDevice 检查 NVMe 设备
func (m *SMARTMonitor) checkNVMeDevice(device string) (*DiskHealth, error) {
	health := &DiskHealth{
		Device:        device,
		Attributes:    make(map[string]SMARTAttribute),
		LastCheckTime: time.Now(),
	}

	// 验证设备路径（防止命令注入）
	if device == "" || strings.ContainsAny(device, ";|&$`()<>") {
		return nil, fmt.Errorf("无效的设备路径")
	}
	if !strings.HasPrefix(device, "/dev/") {
		return nil, fmt.Errorf("设备路径必须以 /dev/ 开头")
	}

	// 获取 NVMe SMART 信息
	// #nosec G204 -- 设备路径已验证
	cmd := exec.Command("smartctl", "-a", device)
	output, err := cmd.Output()
	if err != nil {
		if len(output) == 0 {
			health.SMARTStatus = SMARTStatusUNSUPPORTED
			return health, nil
		}
	}

	outputStr := string(output)
	lines := strings.Split(outputStr, "\n")

	// 解析 NVMe 基本信息
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 模型
		if strings.HasPrefix(line, "Model Number:") {
			health.Model = strings.TrimSpace(strings.TrimPrefix(line, "Model Number:"))
		}

		// 序列号
		if strings.HasPrefix(line, "Serial Number:") {
			health.Serial = strings.TrimSpace(strings.TrimPrefix(line, "Serial Number:"))
		}

		// 容量
		if strings.HasPrefix(line, "Namespace 1 Size/Capacity:") ||
			strings.HasPrefix(line, "Total NVM Capacity:") {
			health.Size = parseCapacity(line)
		}

		// SMART 状态
		if strings.Contains(line, "SMART overall-health") {
			if strings.Contains(line, "PASSED") {
				health.SMARTStatus = SMARTStatusPASSED
			} else if strings.Contains(line, "FAILED") {
				health.SMARTStatus = SMARTStatusFAILING
			}
		}

		// 温度
		if strings.HasPrefix(line, "Temperature:") {
			health.Temperature = parseNVMeTemperature(line)
		}

		// 可用备用空间
		if strings.HasPrefix(line, "Available Spare:") {
			health.NVMeAvailableSpare = parsePercentage(line)
		}

		// 使用百分比
		if strings.HasPrefix(line, "Percentage Used:") {
			health.NVMePercentageUsed = parsePercentage(line)
		}

		// 数据单元读取
		if strings.HasPrefix(line, "Data Units Read:") {
			health.NVMeDataUnitsRead = parseDataUnits(line)
		}

		// 数据单元写入
		if strings.HasPrefix(line, "Data Units Written:") {
			health.NVMeDataUnitsWritten = parseDataUnits(line)
		}

		// 电源周期
		if strings.HasPrefix(line, "Power Cycles:") {
			health.PowerCycleCount = parseNVMeCount(line)
		}

		// 电源小时
		if strings.HasPrefix(line, "Power On Hours:") {
			health.PowerOnHours = parseNVMeCount(line)
		}

		// 不安全关机
		if strings.HasPrefix(line, "Unsafe Shutdowns:") {
			health.UDMACRCErrorCount = parseNVMeCount(line)
		}
	}

	// NVMe 预测失败
	if strings.Contains(outputStr, "Critical Warning") {
		health.PredictedFailure = true
		health.SMARTStatus = SMARTStatusWARNING
	}

	return health, nil
}

// parseSMARTAttributes 解析 SMART 属性
func (m *SMARTMonitor) parseSMARTAttributes(output string) map[string]SMARTAttribute {
	attributes := make(map[string]SMARTAttribute)

	// SMART 属性段落在 "SMART Attributes Data Structure" 之后
	sectionStart := strings.Index(output, "SMART Attributes Data Structure")
	if sectionStart == -1 {
		return attributes
	}

	section := output[sectionStart:]
	lines := strings.Split(section, "\n")

	// 属性行格式: ID# ATTRIBUTE_NAME          FLAG     VALUE WORST THRESH TYPE      UPDATED  WHEN_FAILED RAW_VALUE
	attrPattern := regexp.MustCompile(`^\s*(\d+)\s+(\S+)\s+[\-x0-9A-Fa-f]+\s+(\d+)\s+(\d+)\s+(\d+)\s+.*\s+(\d+)$`)

	for _, line := range lines {
		matches := attrPattern.FindStringSubmatch(line)
		if len(matches) >= 7 {
			id, _ := strconv.ParseUint(matches[1], 10, 8)
			value, _ := strconv.ParseUint(matches[3], 10, 8)
			worst, _ := strconv.ParseUint(matches[4], 10, 8)
			threshold, _ := strconv.ParseUint(matches[5], 10, 8)
			rawValue, _ := strconv.ParseUint(matches[6], 10, 64)

			attr := SMARTAttribute{
				ID:         uint8(id),
				Name:       matches[2],
				Value:      uint8(value),
				Worst:      uint8(worst),
				Threshold:  uint8(threshold),
				RawValue:   rawValue,
				Normalized: int(value),
			}

			attributes[matches[2]] = attr
		}
	}

	return attributes
}

// calculateHealthScore 计算健康分数
func (m *SMARTMonitor) calculateHealthScore(health *DiskHealth) {
	score := 100

	// SMART 状态
	if health.SMARTStatus == SMARTStatusFAILING {
		score = 0
		health.HealthStatus = HealthStatusCritical
		health.HealthScore = score
		return
	}

	// 安全的 uint64 到 int 转换，防止溢出
	safeUint64ToInt := func(u uint64) int {
		maxInt := uint64(math.MaxInt)
		if u > maxInt {
			return math.MaxInt
		}
		return int(u)
	}

	// 重分配扇区扣分
	if health.ReallocatedSectors > 0 {
		if health.ReallocatedSectors >= uint64(m.config.ReallocatedCritical) {
			score -= 40
		} else if health.ReallocatedSectors >= uint64(m.config.ReallocatedWarning) {
			score -= 20
		} else {
			score -= safeUint64ToInt(health.ReallocatedSectors)
		}
	}

	// 待映射扇区扣分
	if health.PendingSectors > 0 {
		if health.PendingSectors >= uint64(m.config.PendingCritical) {
			score -= 30
		} else if health.PendingSectors >= uint64(m.config.PendingWarning) {
			score -= 15
		} else {
			score -= safeUint64ToInt(health.PendingSectors)
		}
	}

	// 温度扣分
	if health.Temperature >= m.config.TempCriticalThreshold {
		score -= 20
	} else if health.Temperature >= m.config.TempWarningThreshold {
		score -= 10
	}

	// CRC 错误扣分
	if health.UDMACRCErrorCount > 10 {
		score -= 10
	}

	// NVMe 特有扣分
	if health.NVMePercentageUsed > 90 {
		score -= 20
	} else if health.NVMePercentageUsed > 80 {
		score -= 10
	}

	if health.NVMeAvailableSpare < 10 {
		score -= 30
	} else if health.NVMeAvailableSpare < 20 {
		score -= 15
	}

	// 限制范围
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	health.HealthScore = score

	// 确定健康状态
	switch {
	case score >= 90:
		health.HealthStatus = HealthStatusExcellent
	case score >= 70:
		health.HealthStatus = HealthStatusGood
	case score >= 50:
		health.HealthStatus = HealthStatusFair
	case score >= 25:
		health.HealthStatus = HealthStatusPoor
	default:
		health.HealthStatus = HealthStatusCritical
	}
}

// checkAlerts 检查告警
func (m *SMARTMonitor) checkAlerts(health *DiskHealth) {
	now := time.Now()

	// SMART 失败
	if health.SMARTStatus == SMARTStatusFAILING {
		m.sendAlert(Alert{
			Type:        AlertTypeSMARTFailure,
			Device:      health.Device,
			Severity:    "CRITICAL",
			Message:     fmt.Sprintf("磁盘 %s SMART 检测失败，建议立即更换", health.Device),
			Timestamp:   now,
			HealthScore: health.HealthScore,
		})
		health.LastAlertTime = now
		health.AlertCount++
	}

	// 温度告警
	if health.Temperature >= m.config.TempCriticalThreshold {
		m.sendAlert(Alert{
			Type:        AlertTypeTemperature,
			Device:      health.Device,
			Severity:    "CRITICAL",
			Message:     fmt.Sprintf("磁盘 %s 温度过高: %d°C", health.Device, health.Temperature),
			Value:       health.Temperature,
			Threshold:   m.config.TempCriticalThreshold,
			Timestamp:   now,
			HealthScore: health.HealthScore,
		})
		health.LastAlertTime = now
		health.AlertCount++
	} else if health.Temperature >= m.config.TempWarningThreshold {
		m.sendAlert(Alert{
			Type:        AlertTypeTemperature,
			Device:      health.Device,
			Severity:    "WARNING",
			Message:     fmt.Sprintf("磁盘 %s 温度偏高: %d°C", health.Device, health.Temperature),
			Value:       health.Temperature,
			Threshold:   m.config.TempWarningThreshold,
			Timestamp:   now,
			HealthScore: health.HealthScore,
		})
		health.LastAlertTime = now
		health.AlertCount++
	}

	// 重分配扇区告警
	if health.ReallocatedSectors >= uint64(m.config.ReallocatedCritical) {
		m.sendAlert(Alert{
			Type:        AlertTypeReallocated,
			Device:      health.Device,
			Severity:    "CRITICAL",
			Message:     fmt.Sprintf("磁盘 %s 重分配扇区过多: %d", health.Device, health.ReallocatedSectors),
			Value:       health.ReallocatedSectors,
			Threshold:   m.config.ReallocatedCritical,
			Timestamp:   now,
			HealthScore: health.HealthScore,
		})
		health.PredictedFailure = true
		health.FailurePredicted = "重分配扇区过多"
		health.LastAlertTime = now
		health.AlertCount++
	} else if health.ReallocatedSectors >= uint64(m.config.ReallocatedWarning) {
		m.sendAlert(Alert{
			Type:        AlertTypeReallocated,
			Device:      health.Device,
			Severity:    "WARNING",
			Message:     fmt.Sprintf("磁盘 %s 存在重分配扇区: %d", health.Device, health.ReallocatedSectors),
			Value:       health.ReallocatedSectors,
			Threshold:   m.config.ReallocatedWarning,
			Timestamp:   now,
			HealthScore: health.HealthScore,
		})
		health.LastAlertTime = now
		health.AlertCount++
	}

	// 待映射扇区告警
	if health.PendingSectors >= uint64(m.config.PendingCritical) {
		m.sendAlert(Alert{
			Type:        AlertTypePending,
			Device:      health.Device,
			Severity:    "CRITICAL",
			Message:     fmt.Sprintf("磁盘 %s 待映射扇区过多: %d", health.Device, health.PendingSectors),
			Value:       health.PendingSectors,
			Threshold:   m.config.PendingCritical,
			Timestamp:   now,
			HealthScore: health.HealthScore,
		})
		health.LastAlertTime = now
		health.AlertCount++
	} else if health.PendingSectors >= uint64(m.config.PendingWarning) {
		m.sendAlert(Alert{
			Type:        AlertTypePending,
			Device:      health.Device,
			Severity:    "WARNING",
			Message:     fmt.Sprintf("磁盘 %s 存在待映射扇区: %d", health.Device, health.PendingSectors),
			Value:       health.PendingSectors,
			Threshold:   m.config.PendingWarning,
			Timestamp:   now,
			HealthScore: health.HealthScore,
		})
		health.LastAlertTime = now
		health.AlertCount++
	}
}

// sendAlert 发送告警
func (m *SMARTMonitor) sendAlert(alert Alert) {
	for _, handler := range m.alertHandlers {
		go handler(alert)
	}
}

// AddAlertHandler 添加告警处理器
func (m *SMARTMonitor) AddAlertHandler(handler AlertHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertHandlers = append(m.alertHandlers, handler)
}

// saveSnapshot 保存健康快照
func (m *SMARTMonitor) saveSnapshot(health *DiskHealth) {
	if m.config.HistoryRetentionDays <= 0 {
		return
	}

	snapshot := HealthSnapshot{
		Timestamp:          time.Now(),
		Temperature:        health.Temperature,
		ReallocatedSectors: health.ReallocatedSectors,
		PendingSectors:     health.PendingSectors,
		HealthScore:        health.HealthScore,
		HealthStatus:       health.HealthStatus,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.history[health.Device] = append(m.history[health.Device], snapshot)

	// 清理过期数据
	cutoff := time.Now().AddDate(0, 0, -m.config.HistoryRetentionDays)
	var validSnapshots []HealthSnapshot
	for _, s := range m.history[health.Device] {
		if s.Timestamp.After(cutoff) {
			validSnapshots = append(validSnapshots, s)
		}
	}
	m.history[health.Device] = validSnapshots
}

// GetDiskHealth 获取磁盘健康状态
func (m *SMARTMonitor) GetDiskHealth(device string) (*DiskHealth, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health, exists := m.disks[device]
	if !exists {
		return nil, false
	}

	// 返回副本
	copy := *health
	copy.Attributes = make(map[string]SMARTAttribute)
	for k, v := range health.Attributes {
		copy.Attributes[k] = v
	}

	return &copy, true
}

// GetAllDisks 获取所有磁盘状态
func (m *SMARTMonitor) GetAllDisks() map[string]*DiskHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*DiskHealth)
	for k, v := range m.disks {
		copy := *v
		copy.Attributes = make(map[string]SMARTAttribute)
		for ak, av := range v.Attributes {
			copy.Attributes[ak] = av
		}
		result[k] = &copy
	}

	return result
}

// GetHistory 获取历史数据
func (m *SMARTMonitor) GetHistory(device string) []HealthSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.history[device]
}

// RunSelfTest 运行自检
func (m *SMARTMonitor) RunSelfTest(device string, testType string) error {
	// 验证设备路径（防止命令注入）
	if device == "" || strings.ContainsAny(device, ";|&$`()<>") {
		return fmt.Errorf("无效的设备路径")
	}
	if !strings.HasPrefix(device, "/dev/") {
		return fmt.Errorf("设备路径必须以 /dev/ 开头")
	}

	// testType: short, long, conveyance, offline
	validTypes := map[string]bool{
		"short":      true,
		"long":       true,
		"conveyance": true,
		"offline":    true,
	}

	if !validTypes[testType] {
		return fmt.Errorf("无效的测试类型: %s", testType)
	}

	// #nosec G204 -- 设备路径已验证，testType 在白名单中验证
	cmd := exec.Command("smartctl", "-t", testType, device)
	return cmd.Run()
}

// GetSelfTestStatus 获取自检状态
func (m *SMARTMonitor) GetSelfTestStatus(device string) (string, error) {
	// 验证设备路径（防止命令注入）
	if device == "" || strings.ContainsAny(device, ";|&$`()<>") {
		return "", fmt.Errorf("无效的设备路径")
	}
	if !strings.HasPrefix(device, "/dev/") {
		return "", fmt.Errorf("设备路径必须以 /dev/ 开头")
	}

	// #nosec G204 -- 设备路径已验证
	cmd := exec.Command("smartctl", "-l", "selftest", device)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	// 解析自检结果
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Self-test execution status:") {
			return strings.TrimSpace(line), nil
		}
	}

	return "未找到自检信息", nil
}

// ExportHealth 导出健康报告
func (m *SMARTMonitor) ExportHealth() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := struct {
		Timestamp time.Time              `json:"timestamp"`
		Disks     map[string]*DiskHealth `json:"disks"`
		Config    SMARTConfig            `json:"config"`
	}{
		Timestamp: time.Now(),
		Disks:     m.disks,
		Config:    m.config,
	}

	return json.MarshalIndent(report, "", "  ")
}

// parseCapacity 解析容量
func parseCapacity(line string) uint64 {
	// 格式: "User Capacity:    1,000,204,886,016 bytes [1.00 TB]"
	re := regexp.MustCompile(`(\d[\d,]+)\s*bytes`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 2 {
		value := strings.ReplaceAll(matches[1], ",", "")
		size, _ := strconv.ParseUint(value, 10, 64)
		return size
	}
	return 0
}

// parseNVMeTemperature 解析 NVMe 温度
func parseNVMeTemperature(line string) int {
	// 格式: "Temperature:                    35 Celsius"
	re := regexp.MustCompile(`(\d+)\s*Celsius`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 2 {
		temp, _ := strconv.Atoi(matches[1])
		return temp
	}
	return 0
}

// parsePercentage 解析百分比
func parsePercentage(line string) int {
	// 格式: "Available Spare:                    100%"
	re := regexp.MustCompile(`(\d+)%`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 2 {
		val, _ := strconv.Atoi(matches[1])
		return val
	}
	return 0
}

// parseDataUnits 解析数据单元
func parseDataUnits(line string) uint64 {
	// 格式: "Data Units Read:                  1,234,567 [6.34 GB]"
	re := regexp.MustCompile(`(\d[\d,]+)\s*\[`)
	matches := re.FindStringSubmatch(line)
	if len(matches) >= 2 {
		value := strings.ReplaceAll(matches[1], ",", "")
		val, _ := strconv.ParseUint(value, 10, 64)
		return val
	}
	return 0
}

// parseNVMeCount 解析 NVMe 计数
func parseNVMeCount(line string) uint64 {
	// 格式: "Power Cycles:                      123"
	parts := strings.Split(line, ":")
	if len(parts) >= 2 {
		value := strings.TrimSpace(parts[1])
		val, _ := strconv.ParseUint(value, 10, 64)
		return val
	}
	return 0
}
