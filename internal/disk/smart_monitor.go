// Package disk 提供磁盘健康监控功能
// Version: v2.45.0 - 智能磁盘健康监控
package disk

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SMARTMonitor SMART 磁盘健康监控器
type SMARTMonitor struct {
	config       *MonitorConfig
	disks        map[string]*DiskInfo
	alertRules   []*AlertRule
	alerts       []*SMARTAlert
	history      map[string][]*SMARTHistoryPoint
	mu           sync.RWMutex
	stopChan     chan struct{}
	notifyFunc   func(alert *SMARTAlert)
	scoreWeights *ScoreWeights
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	// CheckInterval 检查间隔
	CheckInterval time.Duration `json:"checkInterval"`
	// HistoryRetention 历史数据保留时间（小时）
	HistoryRetention int `json:"historyRetention"`
	// MaxHistoryPoints 最大历史数据点数
	MaxHistoryPoints int `json:"maxHistoryPoints"`
	// EnableAutoScan 是否启用自动扫描磁盘
	EnableAutoScan bool `json:"enableAutoScan"`
	// EnablePrediction 是否启用故障预测
	EnablePrediction bool `json:"enablePrediction"`
}

// DefaultMonitorConfig 默认监控配置
var DefaultMonitorConfig = &MonitorConfig{
	CheckInterval:    30 * time.Minute,
	HistoryRetention: 720, // 30 天
	MaxHistoryPoints: 1000,
	EnableAutoScan:   true,
	EnablePrediction: true,
}

// DiskInfo holds information about a disk.
//
//nolint:revive // DiskInfo is intentional for clarity in API responses
type DiskInfo struct {
	Device       string        `json:"device"`
	Model        string        `json:"model"`
	Serial       string        `json:"serial"`
	Firmware     string        `json:"firmware"`
	Size         uint64        `json:"size"`         // 字节
	RotationRate int           `json:"rotationRate"` // RPM，SSD 为 0
	IsSSD        bool          `json:"isSSD"`
	SmartData    *SMARTData    `json:"smartData"`
	HealthScore  *HealthScore  `json:"healthScore"`
	Status       DiskStatus    `json:"status"`
	LastCheck    time.Time     `json:"lastCheck"`
	Predictions  []*Prediction `json:"predictions,omitempty"`
}

// DiskStatus represents the health status of a disk.
//
//nolint:revive // DiskStatus is intentional for clarity in API responses
type DiskStatus string

const (
	// StatusHealthy indicates the disk is healthy.
	StatusHealthy DiskStatus = "healthy"
	// StatusWarning indicates the disk has warnings.
	StatusWarning DiskStatus = "warning"
	// StatusCritical indicates the disk is in critical condition.
	StatusCritical DiskStatus = "critical"
	// StatusUnknown indicates the disk status is unknown.
	StatusUnknown DiskStatus = "unknown"
	// StatusOffline indicates the disk is offline.
	StatusOffline DiskStatus = "offline"
)

// SMARTData SMART 数据
type SMARTData struct {
	// 基本属性
	OverallHealth string `json:"overallHealth"` // PASSED, FAILED, UNKNOWN

	// 关键 SMART 属性
	Temperature        *SMARTAttribute `json:"temperature"`
	ReallocatedSectors *SMARTAttribute `json:"reallocatedSectors"`
	PendingSectors     *SMARTAttribute `json:"pendingSectors"`
	Uncorrectable      *SMARTAttribute `json:"uncorrectable"`
	SeekErrors         *SMARTAttribute `json:"seekErrors"`
	PowerOnHours       *SMARTAttribute `json:"powerOnHours"`
	PowerCycles        *SMARTAttribute `json:"powerCycles"`
	ReadErrors         *SMARTAttribute `json:"readErrors"`
	WriteErrors        *SMARTAttribute `json:"writeErrors"`
	SpinRetryCount     *SMARTAttribute `json:"spinRetryCount"`
	CRCErrors          *SMARTAttribute `json:"crcErrors"`

	// NVMe 专用属性
	PercentageUsed *SMARTAttribute `json:"percentageUsed,omitempty"`
	MediaErrors    *SMARTAttribute `json:"mediaErrors,omitempty"`
	AvailableSpare *SMARTAttribute `json:"availableSpare,omitempty"`

	// 原始属性列表
	AllAttributes map[string]*SMARTAttribute `json:"allAttributes"`
}

// SMARTAttribute SMART 属性
type SMARTAttribute struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Value       uint   `json:"value"`       // 标准化值 (0-100)
	Worst       uint   `json:"worst"`       // 最差值
	Threshold   uint   `json:"threshold"`   // 阈值
	Raw         uint64 `json:"raw"`         // 原始值
	Flags       uint16 `json:"flags"`       // 属性标志
	IsPrefail   bool   `json:"isPrefail"`   // 是否为预失败属性
	IsCritical  bool   `json:"isCritical"`  // 是否关键
	Description string `json:"description"` // 描述
}

// HealthScore 健康评分
type HealthScore struct {
	Score           int              `json:"score"`           // 0-100
	Grade           string           `json:"grade"`           // A/B/C/D/F
	Status          DiskStatus       `json:"status"`          // 健康状态
	Components      *ScoreComponents `json:"components"`      // 分项评分
	Recommendations []string         `json:"recommendations"` // 建议
	Timestamp       time.Time        `json:"timestamp"`
	Trend           string           `json:"trend"` // up/down/stable
}

// ScoreComponents 分项评分
type ScoreComponents struct {
	Temperature  ComponentScore `json:"temperature"`
	Reallocation ComponentScore `json:"reallocation"`
	Pending      ComponentScore `json:"pending"`
	Errors       ComponentScore `json:"errors"`
	Age          ComponentScore `json:"age"`
	Stability    ComponentScore `json:"stability"`
}

// ComponentScore 组件评分
type ComponentScore struct {
	Score   int         `json:"score"`   // 0-100
	Weight  float64     `json:"weight"`  // 权重
	Status  string      `json:"status"`  // ok/warning/critical
	Message string      `json:"message"` // 说明
	Value   interface{} `json:"value"`
}

// ScoreWeights 评分权重
type ScoreWeights struct {
	Temperature  float64 `json:"temperature"`
	Reallocation float64 `json:"reallocation"`
	Pending      float64 `json:"pending"`
	Errors       float64 `json:"errors"`
	Age          float64 `json:"age"`
	Stability    float64 `json:"stability"`
}

// DefaultScoreWeights 默认评分权重
var DefaultScoreWeights = &ScoreWeights{
	Temperature:  0.15,
	Reallocation: 0.25,
	Pending:      0.25,
	Errors:       0.20,
	Age:          0.10,
	Stability:    0.05,
}

// AlertRule 告警规则
type AlertRule struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	Attribute     string               `json:"attribute"` // temperature, reallocated, pending, etc.
	Condition     string               `json:"condition"` // gt, lt, eq
	Threshold     float64              `json:"threshold"`
	Severity      AlertSeverity        `json:"severity"` // info, warning, critical
	Enabled       bool                 `json:"enabled"`
	Cooldown      time.Duration        `json:"cooldown"` // 同一磁盘告警冷却时间
	LastTriggered map[string]time.Time `json:"-"`
}

// AlertSeverity represents the severity level of an alert.
type AlertSeverity string

const (
	// AlertInfo is an informational alert.
	AlertInfo AlertSeverity = "info"
	// AlertWarning is a warning-level alert.
	AlertWarning AlertSeverity = "warning"
	// AlertCritical is a critical-level alert.
	AlertCritical AlertSeverity = "critical"
)

// SMARTAlert SMART 告警
type SMARTAlert struct {
	ID           string        `json:"id"`
	Device       string        `json:"device"`
	RuleID       string        `json:"ruleId"`
	Attribute    string        `json:"attribute"`
	Severity     AlertSeverity `json:"severity"`
	Message      string        `json:"message"`
	Value        interface{}   `json:"value"`
	Threshold    float64       `json:"threshold"`
	Timestamp    time.Time     `json:"timestamp"`
	Acknowledged bool          `json:"acknowledged"`
	Resolved     bool          `json:"resolved"`
}

// Prediction 故障预测
type Prediction struct {
	Type        string    `json:"type"`        // temperature, wear, error_rate
	Probability float64   `json:"probability"` // 0-1
	ETA         time.Time `json:"eta"`         // 预计故障时间
	Confidence  float64   `json:"confidence"`  // 置信度
	Description string    `json:"description"`
}

// SMARTHistoryPoint SMART 历史数据点
type SMARTHistoryPoint struct {
	Timestamp          time.Time `json:"timestamp"`
	HealthScore        int       `json:"healthScore"`
	Temperature        int       `json:"temperature"`
	ReallocatedSectors uint64    `json:"reallocatedSectors"`
	PendingSectors     uint64    `json:"pendingSectors"`
	ReadErrors         uint64    `json:"readErrors"`
	WriteErrors        uint64    `json:"writeErrors"`
}

// NewSMARTMonitor 创建 SMART 监控器
func NewSMARTMonitor(config *MonitorConfig) *SMARTMonitor {
	if config == nil {
		config = DefaultMonitorConfig
	}

	m := &SMARTMonitor{
		config:       config,
		disks:        make(map[string]*DiskInfo),
		alertRules:   getDefaultAlertRules(),
		alerts:       make([]*SMARTAlert, 0),
		history:      make(map[string][]*SMARTHistoryPoint),
		stopChan:     make(chan struct{}),
		scoreWeights: DefaultScoreWeights,
	}

	// 初始扫描磁盘
	if config.EnableAutoScan {
		_ = m.ScanDisks()
	}

	// 启动定期检查
	go m.startPeriodicCheck()

	return m
}

// getDefaultAlertRules 获取默认告警规则
func getDefaultAlertRules() []*AlertRule {
	return []*AlertRule{
		{
			ID:            "temp-warning",
			Name:          "温度警告",
			Attribute:     "temperature",
			Condition:     "gt",
			Threshold:     50,
			Severity:      AlertWarning,
			Enabled:       true,
			Cooldown:      1 * time.Hour,
			LastTriggered: make(map[string]time.Time),
		},
		{
			ID:            "temp-critical",
			Name:          "温度严重",
			Attribute:     "temperature",
			Condition:     "gt",
			Threshold:     60,
			Severity:      AlertCritical,
			Enabled:       true,
			Cooldown:      30 * time.Minute,
			LastTriggered: make(map[string]time.Time),
		},
		{
			ID:            "reallocated-warning",
			Name:          "重映射扇区警告",
			Attribute:     "reallocatedSectors",
			Condition:     "gt",
			Threshold:     10,
			Severity:      AlertWarning,
			Enabled:       true,
			Cooldown:      24 * time.Hour,
			LastTriggered: make(map[string]time.Time),
		},
		{
			ID:            "reallocated-critical",
			Name:          "重映射扇区严重",
			Attribute:     "reallocatedSectors",
			Condition:     "gt",
			Threshold:     100,
			Severity:      AlertCritical,
			Enabled:       true,
			Cooldown:      6 * time.Hour,
			LastTriggered: make(map[string]time.Time),
		},
		{
			ID:            "pending-warning",
			Name:          "待映射扇区警告",
			Attribute:     "pendingSectors",
			Condition:     "gt",
			Threshold:     10,
			Severity:      AlertWarning,
			Enabled:       true,
			Cooldown:      24 * time.Hour,
			LastTriggered: make(map[string]time.Time),
		},
		{
			ID:            "pending-critical",
			Name:          "待映射扇区严重",
			Attribute:     "pendingSectors",
			Condition:     "gt",
			Threshold:     100,
			Severity:      AlertCritical,
			Enabled:       true,
			Cooldown:      6 * time.Hour,
			LastTriggered: make(map[string]time.Time),
		},
		{
			ID:            "health-critical",
			Name:          "健康状态严重",
			Attribute:     "healthScore",
			Condition:     "lt",
			Threshold:     50,
			Severity:      AlertCritical,
			Enabled:       true,
			Cooldown:      1 * time.Hour,
			LastTriggered: make(map[string]time.Time),
		},
		// NVMe 专用告警规则
		{
			ID:            "nvme-wear-warning",
			Name:          "NVMe使用寿命警告",
			Attribute:     "nvmePercentageUsed",
			Condition:     "gt",
			Threshold:     70,
			Severity:      AlertWarning,
			Enabled:       true,
			Cooldown:      24 * time.Hour,
			LastTriggered: make(map[string]time.Time),
		},
		{
			ID:            "nvme-wear-critical",
			Name:          "NVMe使用寿命严重",
			Attribute:     "nvmePercentageUsed",
			Condition:     "gt",
			Threshold:     90,
			Severity:      AlertCritical,
			Enabled:       true,
			Cooldown:      6 * time.Hour,
			LastTriggered: make(map[string]time.Time),
		},
		{
			ID:            "nvme-spare-warning",
			Name:          "NVMe备用空间警告",
			Attribute:     "nvmeAvailableSpare",
			Condition:     "lt",
			Threshold:     20,
			Severity:      AlertWarning,
			Enabled:       true,
			Cooldown:      24 * time.Hour,
			LastTriggered: make(map[string]time.Time),
		},
		{
			ID:            "nvme-media-errors",
			Name:          "NVMe媒体错误",
			Attribute:     "nvmeMediaErrors",
			Condition:     "gt",
			Threshold:     0,
			Severity:      AlertWarning,
			Enabled:       true,
			Cooldown:      12 * time.Hour,
			LastTriggered: make(map[string]time.Time),
		},
		{
			ID:            "nvme-critical-warning",
			Name:          "NVMe关键警告",
			Attribute:     "nvmeCriticalWarning",
			Condition:     "gt",
			Threshold:     0,
			Severity:      AlertCritical,
			Enabled:       true,
			Cooldown:      1 * time.Hour,
			LastTriggered: make(map[string]time.Time),
		},
		{
			ID:            "nvme-temp-high",
			Name:          "NVMe温度过高",
			Attribute:     "temperature",
			Condition:     "gt",
			Threshold:     65,
			Severity:      AlertWarning,
			Enabled:       true,
			Cooldown:      2 * time.Hour,
			LastTriggered: make(map[string]time.Time),
		},
	}
}

// SetNotifyFunc 设置通知回调函数
func (m *SMARTMonitor) SetNotifyFunc(fn func(alert *SMARTAlert)) {
	m.notifyFunc = fn
}

// ScanDisks 扫描系统磁盘
func (m *SMARTMonitor) ScanDisks() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 使用 lsblk 列出块设备
	cmd := exec.Command("lsblk", "-d", "-n", "-o", "NAME,SIZE,ROTA")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("扫描磁盘失败: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		deviceName := fields[0]
		device := "/dev/" + deviceName

		// 只处理硬盘和 NVMe 设备
		if !strings.HasPrefix(deviceName, "sd") &&
			!strings.HasPrefix(deviceName, "nvme") &&
			!strings.HasPrefix(deviceName, "hd") &&
			!strings.HasPrefix(deviceName, "vd") {
			continue
		}

		// 解析大小
		var size uint64
		if len(fields) >= 2 {
			size = parseSize(fields[1])
		}

		// 判断是否为 SSD
		isSSD := false
		if len(fields) >= 3 {
			rota := fields[2]
			isSSD = (rota == "0")
		}

		// 创建磁盘信息
		disk := &DiskInfo{
			Device: device,
			Size:   size,
			IsSSD:  isSSD,
			Status: StatusUnknown,
		}

		// 获取 SMART 数据
		smartData, err := m.getSMARTData(device)
		if err == nil {
			disk.SmartData = smartData
			// 从 SMART 属性中获取型号
			if attr, ok := smartData.AllAttributes["model"]; ok && attr != nil {
				disk.Model = attr.Description
			}
			// 从 SMART 属性中获取序列号
			if attr, ok := smartData.AllAttributes["serial"]; ok && attr != nil {
				disk.Serial = attr.Description
			}
			// 从 SMART 属性中获取固件版本
			if attr, ok := smartData.AllAttributes["firmware"]; ok && attr != nil {
				disk.Firmware = attr.Description
			}
			disk.LastCheck = time.Now()
		}

		m.disks[device] = disk
	}

	// 更新健康评分
	for _, disk := range m.disks {
		m.calculateHealthScore(disk)
	}

	return nil
}

// getSMARTData 获取 SMART 数据
func (m *SMARTMonitor) getSMARTData(device string) (*SMARTData, error) {
	// 检查 smartctl 是否可用
	if _, err := exec.LookPath("smartctl"); err != nil {
		return nil, fmt.Errorf("smartctl 未安装")
	}

	// NVMe 设备使用不同的参数
	args := []string{"-A", "-i", "-H", device}
	if strings.HasPrefix(device, "/dev/nvme") {
		args = []string{"-a", device}
	}

	cmd := exec.Command("smartctl", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取 SMART 数据失败: %w", err)
	}

	return m.parseSMARTOutput(string(output), device), nil
}

// parseSMARTOutput 解析 smartctl 输出
func (m *SMARTMonitor) parseSMARTOutput(output string, _ string) *SMARTData {
	data := &SMARTData{
		OverallHealth: "UNKNOWN",
		AllAttributes: make(map[string]*SMARTAttribute),
	}

	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()

		// 解析健康状态
		if strings.Contains(line, "SMART overall-health self-assessment test result:") {
			if strings.Contains(line, "PASSED") {
				data.OverallHealth = "PASSED"
			} else if strings.Contains(line, "FAILED") {
				data.OverallHealth = "FAILED"
			}
		}

		// 解析型号
		if strings.HasPrefix(line, "Device Model:") || strings.HasPrefix(line, "Model Number:") {
			model := strings.TrimSpace(strings.TrimPrefix(line, "Device Model:"))
			if model == "" {
				model = strings.TrimSpace(strings.TrimPrefix(line, "Model Number:"))
			}
			data.AllAttributes["model"] = &SMARTAttribute{
				Name:  "model",
				Value: 0,
				Raw:   0,
			}
			data.AllAttributes["model"].Description = model
		}

		// 解析序列号
		if strings.HasPrefix(line, "Serial Number:") {
			data.AllAttributes["serial"] = &SMARTAttribute{
				Name:        "serial",
				Description: strings.TrimSpace(strings.TrimPrefix(line, "Serial Number:")),
			}
		}

		// 解析固件版本
		if strings.HasPrefix(line, "Firmware Version:") {
			data.AllAttributes["firmware"] = &SMARTAttribute{
				Name:        "firmware",
				Description: strings.TrimSpace(strings.TrimPrefix(line, "Firmware Version:")),
			}
		}

		// 解析 SMART 属性表
		if strings.HasPrefix(line, "ID#") || strings.Contains(line, "ATTRIBUTE_NAME") {
			// 跳过表头
			continue
		}

		// 解析属性行
		// 格式: ID# ATTRIBUTE_NAME          FLAG     VALUE WORST THRESH TYPE      UPDATED  WHEN_FAILED RAW_VALUE
		fields := strings.Fields(line)
		if len(fields) >= 10 {
			id, err := strconv.ParseUint(fields[0], 10, 32)
			if err != nil {
				continue
			}

			attr := &SMARTAttribute{
				ID:   uint(id),
				Name: fields[1],
			}

			// 解析 flag
			if flag, err := strconv.ParseUint(fields[2], 16, 16); err == nil {
				attr.Flags = uint16(flag)
				attr.IsPrefail = (flag & 0x0001) != 0
			}

			// 解析值
			if val, err := strconv.ParseUint(fields[3], 10, 32); err == nil {
				attr.Value = uint(val)
			}
			if val, err := strconv.ParseUint(fields[4], 10, 32); err == nil {
				attr.Worst = uint(val)
			}
			if val, err := strconv.ParseUint(fields[5], 10, 32); err == nil {
				attr.Threshold = uint(val)
			}

			// 解析原始值（最后一个字段）
			rawStr := fields[len(fields)-1]
			if raw, err := strconv.ParseUint(rawStr, 10, 64); err == nil {
				attr.Raw = raw
			}

			// 标记关键属性
			attr.IsCritical = isCriticalAttribute(attr.ID)

			// 存储到对应字段
			data.AllAttributes[attr.Name] = attr
			m.mapToSMARTField(data, attr)
		}

		// NVMe 特有属性解析
		if strings.Contains(line, "Temperature:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Temperature:" && i+1 < len(fields) {
					temp, _ := strconv.ParseUint(strings.TrimSuffix(fields[i+1], "C"), 10, 32)
					data.Temperature = &SMARTAttribute{
						Name:  "temperature",
						Value: uint(temp),
						Raw:   temp,
					}
				}
			}
		}

		if strings.Contains(line, "Percentage Used:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Used:" && i+1 < len(fields) {
					pct, _ := strconv.ParseFloat(strings.TrimSuffix(fields[i+1], "%"), 64)
					data.PercentageUsed = &SMARTAttribute{
						Name:  "percentageUsed",
						Value: uint(100 - pct),
						Raw:   uint64(pct),
					}
				}
			}
		}
	}

	return data
}

// mapToSMARTField 映射属性到结构体字段
func (m *SMARTMonitor) mapToSMARTField(data *SMARTData, attr *SMARTAttribute) {
	switch attr.ID {
	case 5: // Reallocated_Sector_Ct
		data.ReallocatedSectors = attr
	case 9: // Power_On_Hours
		data.PowerOnHours = attr
	case 10: // Spin_Retry_Count
		data.SpinRetryCount = attr
	case 12: // Power_Cycle_Count
		data.PowerCycles = attr
	case 184: // End-to-End_Error
		data.Uncorrectable = attr
	case 187: // Reported_Uncorrect
		if data.Uncorrectable == nil {
			data.Uncorrectable = attr
		}
	case 188: // Command_Timeout
		data.CRCErrors = attr
	case 189: // High_Fly_Writes
		data.ReadErrors = attr
	case 190: // Airflow_Temperature_Cel
		if data.Temperature == nil {
			data.Temperature = attr
		}
	case 194: // Temperature_Celsius
		data.Temperature = attr
	case 196: // Reallocated_Event_Count
		data.SeekErrors = attr
	case 197: // Current_Pending_Sector
		data.PendingSectors = attr
	case 198: // Offline_Uncorrectable
		data.WriteErrors = attr
	case 199: // UDMA_CRC_Error_Count
		data.CRCErrors = attr
	case 200: // Multi_Zone_Error_Rate
		data.ReadErrors = attr
	}
}

// isCriticalAttribute 判断是否为关键属性
func isCriticalAttribute(id uint) bool {
	criticalIDs := []uint{5, 10, 184, 187, 188, 196, 197, 198, 199}
	for _, cid := range criticalIDs {
		if id == cid {
			return true
		}
	}
	return false
}

// calculateHealthScore 计算健康评分
func (m *SMARTMonitor) calculateHealthScore(disk *DiskInfo) {
	if disk.SmartData == nil {
		disk.HealthScore = &HealthScore{
			Score:      0,
			Grade:      "?",
			Status:     StatusUnknown,
			Components: &ScoreComponents{},
			Timestamp:  time.Now(),
		}
		return
	}

	components := &ScoreComponents{}
	smartData := disk.SmartData

	// 1. 温度评分
	components.Temperature = m.calculateTempScore(smartData)

	// 2. 重映射扇区评分
	components.Reallocation = m.calculateReallocationScore(smartData)

	// 3. 待映射扇区评分
	components.Pending = m.calculatePendingScore(smartData)

	// 4. 错误评分
	components.Errors = m.calculateErrorScore(smartData)

	// 5. 使用年限评分
	components.Age = m.calculateAgeScore(smartData)

	// 6. 稳定性评分
	components.Stability = m.calculateStabilityScore(disk.Device)

	// 计算总分
	weights := m.scoreWeights
	totalScore := int(
		float64(components.Temperature.Score)*weights.Temperature +
			float64(components.Reallocation.Score)*weights.Reallocation +
			float64(components.Pending.Score)*weights.Pending +
			float64(components.Errors.Score)*weights.Errors +
			float64(components.Age.Score)*weights.Age +
			float64(components.Stability.Score)*weights.Stability,
	)

	// 确保分数在 0-100 范围内
	if totalScore > 100 {
		totalScore = 100
	}
	if totalScore < 0 {
		totalScore = 0
	}

	// 确定等级和状态
	grade, status := m.getGradeAndStatus(totalScore)

	// 生成建议
	recommendations := m.generateRecommendations(disk, components)

	// 计算趋势
	trend := m.calculateTrend(disk.Device, totalScore)

	disk.HealthScore = &HealthScore{
		Score:           totalScore,
		Grade:           grade,
		Status:          status,
		Components:      components,
		Recommendations: recommendations,
		Timestamp:       time.Now(),
		Trend:           trend,
	}

	disk.Status = status
}

// calculateTempScore 计算温度评分
func (m *SMARTMonitor) calculateTempScore(smartData *SMARTData) ComponentScore {
	if smartData.Temperature == nil {
		return ComponentScore{Score: 100, Weight: m.scoreWeights.Temperature, Status: "ok", Message: "无温度数据"}
	}

	// 安全转换：温度值限制在合理范围
	temp := safeUint64ToBoundedInt(smartData.Temperature.Raw, 0, 200)
	score := ComponentScore{
		Weight: m.scoreWeights.Temperature,
		Value:  temp,
	}

	switch {
	case temp <= 40:
		score.Score = 100
		score.Status = "ok"
		score.Message = "温度正常"
	case temp <= 50:
		score.Score = 90
		score.Status = "ok"
		score.Message = "温度偏高但正常"
	case temp <= 55:
		score.Score = 70
		score.Status = "warning"
		score.Message = "温度警告"
	case temp <= 60:
		score.Score = 50
		score.Status = "warning"
		score.Message = "温度过高"
	default:
		score.Score = 20
		score.Status = "critical"
		score.Message = "温度严重过高，请检查散热"
	}

	return score
}

// calculateReallocationScore 计算重映射扇区评分
func (m *SMARTMonitor) calculateReallocationScore(smartData *SMARTData) ComponentScore {
	if smartData.ReallocatedSectors == nil {
		return ComponentScore{Score: 100, Weight: m.scoreWeights.Reallocation, Status: "ok", Message: "无重映射数据"}
	}

	count := smartData.ReallocatedSectors.Raw
	score := ComponentScore{
		Weight: m.scoreWeights.Reallocation,
		Value:  count,
	}

	switch {
	case count == 0:
		score.Score = 100
		score.Status = "ok"
		score.Message = "无重映射扇区"
	case count <= 10:
		score.Score = 80
		score.Status = "warning"
		score.Message = "少量重映射扇区，需关注"
	case count <= 100:
		score.Score = 50
		score.Status = "warning"
		score.Message = "重映射扇区较多，建议备份"
	default:
		score.Score = 10
		score.Status = "critical"
		score.Message = "重映射扇区过多，建议更换磁盘"
	}

	return score
}

// calculatePendingScore 计算待映射扇区评分
func (m *SMARTMonitor) calculatePendingScore(smartData *SMARTData) ComponentScore {
	if smartData.PendingSectors == nil {
		return ComponentScore{Score: 100, Weight: m.scoreWeights.Pending, Status: "ok", Message: "无待映射数据"}
	}

	count := smartData.PendingSectors.Raw
	score := ComponentScore{
		Weight: m.scoreWeights.Pending,
		Value:  count,
	}

	switch {
	case count == 0:
		score.Score = 100
		score.Status = "ok"
		score.Message = "无待映射扇区"
	case count <= 10:
		score.Score = 75
		score.Status = "warning"
		score.Message = "存在待映射扇区"
	case count <= 100:
		score.Score = 40
		score.Status = "warning"
		score.Message = "待映射扇区较多，数据风险高"
	default:
		score.Score = 5
		score.Status = "critical"
		score.Message = "待映射扇区过多，数据可能已损坏"
	}

	return score
}

// calculateErrorScore 计算错误评分
func (m *SMARTMonitor) calculateErrorScore(smartData *SMARTData) ComponentScore {
	var totalErrors uint64
	var errorTypes []string

	if smartData.ReadErrors != nil {
		totalErrors += smartData.ReadErrors.Raw
		if smartData.ReadErrors.Raw > 0 {
			errorTypes = append(errorTypes, "读取错误")
		}
	}
	if smartData.WriteErrors != nil {
		totalErrors += smartData.WriteErrors.Raw
		if smartData.WriteErrors.Raw > 0 {
			errorTypes = append(errorTypes, "写入错误")
		}
	}
	if smartData.CRCErrors != nil {
		totalErrors += smartData.CRCErrors.Raw
		if smartData.CRCErrors.Raw > 0 {
			errorTypes = append(errorTypes, "CRC错误")
		}
	}
	if smartData.SeekErrors != nil {
		totalErrors += smartData.SeekErrors.Raw
		if smartData.SeekErrors.Raw > 0 {
			errorTypes = append(errorTypes, "寻道错误")
		}
	}
	if smartData.Uncorrectable != nil {
		totalErrors += smartData.Uncorrectable.Raw * 10 // 加权
		if smartData.Uncorrectable.Raw > 0 {
			errorTypes = append(errorTypes, "不可纠正错误")
		}
	}

	score := ComponentScore{
		Weight: m.scoreWeights.Errors,
		Value:  totalErrors,
	}

	switch {
	case totalErrors == 0:
		score.Score = 100
		score.Status = "ok"
		score.Message = "无错误"
	case totalErrors <= 10:
		score.Score = 80
		score.Status = "ok"
		score.Message = "少量错误，属正常范围"
	case totalErrors <= 100:
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

// calculateAgeScore 计算使用年限评分
func (m *SMARTMonitor) calculateAgeScore(smartData *SMARTData) ComponentScore {
	if smartData.PowerOnHours == nil {
		return ComponentScore{Score: 100, Weight: m.scoreWeights.Age, Status: "ok", Message: "无使用时间数据"}
	}

	hours := smartData.PowerOnHours.Raw
	years := float64(hours) / (24 * 365)
	score := ComponentScore{
		Weight: m.scoreWeights.Age,
		Value:  hours,
	}

	switch {
	case years < 1:
		score.Score = 100
		score.Status = "ok"
		score.Message = "使用时间少于1年"
	case years < 2:
		score.Score = 95
		score.Status = "ok"
		score.Message = "使用时间正常"
	case years < 3:
		score.Score = 85
		score.Status = "ok"
		score.Message = "使用时间较长"
	case years < 5:
		score.Score = 70
		score.Status = "ok"
		score.Message = "使用时间较长，注意备份"
	case years < 7:
		score.Score = 50
		score.Status = "warning"
		score.Message = "使用时间很长，建议评估更换"
	default:
		score.Score = 30
		score.Status = "warning"
		score.Message = "使用时间过长，建议更换"
	}

	return score
}

// calculateStabilityScore 计算稳定性评分
func (m *SMARTMonitor) calculateStabilityScore(device string) ComponentScore {
	m.mu.RLock()
	history, exists := m.history[device]
	m.mu.RUnlock()

	if !exists || len(history) < 2 {
		return ComponentScore{Score: 100, Weight: m.scoreWeights.Stability, Status: "ok", Message: "无历史数据"}
	}

	// 计算健康评分的变化趋势
	var totalChange int
	for i := 1; i < len(history); i++ {
		change := history[i-1].HealthScore - history[i].HealthScore
		if change > 0 {
			totalChange += change
		}
	}

	score := ComponentScore{
		Weight: m.scoreWeights.Stability,
		Value:  totalChange,
	}

	switch {
	case totalChange == 0:
		score.Score = 100
		score.Status = "ok"
		score.Message = "状态稳定"
	case totalChange <= 5:
		score.Score = 90
		score.Status = "ok"
		score.Message = "轻微变化"
	case totalChange <= 15:
		score.Score = 70
		score.Status = "warning"
		score.Message = "状态有变化趋势"
	default:
		score.Score = 40
		score.Status = "warning"
		score.Message = "状态变化较大，注意监控"
	}

	return score
}

// getGradeAndStatus 根据分数获取等级和状态
func (m *SMARTMonitor) getGradeAndStatus(score int) (string, DiskStatus) {
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

// generateRecommendations 生成建议
func (m *SMARTMonitor) generateRecommendations(_ *DiskInfo, components *ScoreComponents) []string {
	var recommendations []string

	switch components.Temperature.Status {
	case "critical":
		recommendations = append(recommendations, "立即检查机箱散热，添加风扇或改善通风")
	case "warning":
		recommendations = append(recommendations, "建议检查散热系统，确保风道通畅")
	}

	switch components.Reallocation.Status {
	case "critical":
		recommendations = append(recommendations, "重映射扇区过多，建议立即备份数据并更换磁盘")
	case "warning":
		recommendations = append(recommendations, "存在重映射扇区，建议增加备份频率")
	}

	switch components.Pending.Status {
	case "critical":
		recommendations = append(recommendations, "待映射扇区过多，数据可能已损坏，立即检查重要文件")
	case "warning":
		recommendations = append(recommendations, "存在待映射扇区，建议运行磁盘检查")
	}

	switch components.Errors.Status {
	case "critical":
		recommendations = append(recommendations, "检测到大量错误，建议立即更换磁盘")
	case "warning":
		recommendations = append(recommendations, "存在I/O错误，检查磁盘连接和数据线")
	}

	switch components.Age.Status {
	case "warning":
		recommendations = append(recommendations, "磁盘使用时间较长，建议规划更换")
	case "old":
		// 可添加针对老旧磁盘的建议
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "磁盘状态良好，继续保持定期备份")
	}

	return recommendations
}

// calculateTrend 计算趋势
func (m *SMARTMonitor) calculateTrend(device string, currentScore int) string {
	m.mu.RLock()
	history, exists := m.history[device]
	m.mu.RUnlock()

	if !exists || len(history) < 2 {
		return "stable"
	}

	prevScore := history[len(history)-1].HealthScore
	diff := currentScore - prevScore

	switch {
	case diff > 5:
		return "up"
	case diff < -5:
		return "down"
	default:
		return "stable"
	}
}

// startPeriodicCheck 启动定期检查
func (m *SMARTMonitor) startPeriodicCheck() {
	// 如果 CheckInterval 为 0 或负数，不启动定时器
	if m.config.CheckInterval <= 0 {
		return
	}

	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = m.CheckAllDisks()
		case <-m.stopChan:
			return
		}
	}
}

// CheckAllDisks 检查所有磁盘
func (m *SMARTMonitor) CheckAllDisks() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for device, disk := range m.disks {
		smartData, err := m.getSMARTData(device)
		if err != nil {
			disk.Status = StatusOffline
			continue
		}

		disk.SmartData = smartData
		disk.LastCheck = time.Now()
		m.calculateHealthScore(disk)

		// 保存历史数据
		m.saveHistoryPoint(disk)

		// 检查告警规则
		m.checkAlertRules(disk)

		// 预测分析
		if m.config.EnablePrediction {
			m.predictFailures(disk)
		}
	}

	return nil
}

// saveHistoryPoint 保存历史数据点
func (m *SMARTMonitor) saveHistoryPoint(disk *DiskInfo) {
	if disk.SmartData == nil || disk.HealthScore == nil {
		return
	}

	point := &SMARTHistoryPoint{
		Timestamp:          time.Now(),
		HealthScore:        disk.HealthScore.Score,
		ReallocatedSectors: getAttrRaw(disk.SmartData.ReallocatedSectors),
		PendingSectors:     getAttrRaw(disk.SmartData.PendingSectors),
		ReadErrors:         getAttrRaw(disk.SmartData.ReadErrors),
		WriteErrors:        getAttrRaw(disk.SmartData.WriteErrors),
	}

	if disk.SmartData.Temperature != nil {
		// 安全转换：温度值限制在合理范围
		point.Temperature = safeUint64ToBoundedInt(disk.SmartData.Temperature.Raw, 0, 200)
	}

	history := m.history[disk.Device]
	history = append(history, point)

	// 限制历史数据量
	if len(history) > m.config.MaxHistoryPoints {
		history = history[len(history)-m.config.MaxHistoryPoints:]
	}

	m.history[disk.Device] = history
}

// getAttrRaw 安全获取属性原始值
func getAttrRaw(attr *SMARTAttribute) uint64 {
	if attr == nil {
		return 0
	}
	return attr.Raw
}

// checkAlertRules 检查告警规则
func (m *SMARTMonitor) checkAlertRules(disk *DiskInfo) {
	if disk.SmartData == nil {
		return
	}

	now := time.Now()

	for _, rule := range m.alertRules {
		if !rule.Enabled {
			continue
		}

		// 检查冷却时间
		if lastTime, exists := rule.LastTriggered[disk.Device]; exists {
			if now.Sub(lastTime) < rule.Cooldown {
				continue
			}
		}

		value := m.getAttributeValue(disk, rule.Attribute)
		threshold := rule.Threshold

		triggered := false
		switch rule.Condition {
		case "gt":
			triggered = value > threshold
		case "lt":
			triggered = value < threshold
		case "eq":
			triggered = value == threshold
		}

		if triggered {
			alert := &SMARTAlert{
				ID:        generateAlertID(),
				Device:    disk.Device,
				RuleID:    rule.ID,
				Attribute: rule.Attribute,
				Severity:  rule.Severity,
				Value:     value,
				Threshold: threshold,
				Timestamp: now,
				Message:   fmt.Sprintf("%s: %s (%v)", disk.Device, rule.Name, value),
			}
			m.alerts = append(m.alerts, alert)
			rule.LastTriggered[disk.Device] = now

			// 发送通知
			if m.notifyFunc != nil {
				go m.notifyFunc(alert)
			}
		}
	}
}

// getAttributeValue 获取属性值
func (m *SMARTMonitor) getAttributeValue(disk *DiskInfo, attrName string) float64 {
	switch attrName {
	case "temperature":
		if disk.SmartData.Temperature != nil {
			return float64(disk.SmartData.Temperature.Raw)
		}
	case "reallocatedSectors":
		if disk.SmartData.ReallocatedSectors != nil {
			return float64(disk.SmartData.ReallocatedSectors.Raw)
		}
	case "pendingSectors":
		if disk.SmartData.PendingSectors != nil {
			return float64(disk.SmartData.PendingSectors.Raw)
		}
	case "healthScore":
		if disk.HealthScore != nil {
			return float64(disk.HealthScore.Score)
		}
	case "powerOnHours":
		if disk.SmartData.PowerOnHours != nil {
			return float64(disk.SmartData.PowerOnHours.Raw)
		}
	// NVMe 专用属性
	case "nvmePercentageUsed":
		if disk.SmartData.PercentageUsed != nil {
			return float64(disk.SmartData.PercentageUsed.Raw)
		}
	case "nvmeAvailableSpare":
		if disk.SmartData.AvailableSpare != nil {
			return float64(disk.SmartData.AvailableSpare.Value)
		}
	case "nvmeMediaErrors":
		if disk.SmartData.MediaErrors != nil {
			return float64(disk.SmartData.MediaErrors.Raw)
		}
	case "nvmeCriticalWarning":
		// 从SMART数据中提取关键警告标志
		if disk.SmartData.OverallHealth == "FAILED" {
			return 1.0
		}
	}
	return 0
}

// predictFailures 预测故障
func (m *SMARTMonitor) predictFailures(disk *DiskInfo) {
	m.mu.RLock()
	history := m.history[disk.Device]
	m.mu.RUnlock()

	if len(history) < 10 {
		return
	}

	disk.Predictions = nil

	// 预测温度问题
	if disk.SmartData.Temperature != nil {
		tempTrend := m.calculateTrendValue(history, func(h *SMARTHistoryPoint) float64 {
			return float64(h.Temperature)
		})
		if tempTrend > 0.5 {
			disk.Predictions = append(disk.Predictions, &Prediction{
				Type:        "temperature",
				Probability: 0.7,
				Description: "温度呈上升趋势，可能存在散热问题",
				Confidence:  0.6,
			})
		}
	}

	// 预测扇区问题
	reallocTrend := m.calculateTrendValue(history, func(h *SMARTHistoryPoint) float64 {
		return float64(h.ReallocatedSectors)
	})
	if reallocTrend > 0 {
		pendingTrend := m.calculateTrendValue(history, func(h *SMARTHistoryPoint) float64 {
			return float64(h.PendingSectors)
		})
		if pendingTrend > 0 || reallocTrend > 1 {
			prob := 0.5 + reallocTrend*0.1
			if prob > 1 {
				prob = 1
			}
			disk.Predictions = append(disk.Predictions, &Prediction{
				Type:        "wear",
				Probability: prob,
				Description: "检测到扇区增长趋势，磁盘可能接近寿命终点",
				Confidence:  0.7,
			})
		}
	}

	// 健康评分下降预测
	if disk.HealthScore != nil && disk.HealthScore.Score < 70 {
		scoreTrend := m.calculateTrendValue(history, func(h *SMARTHistoryPoint) float64 {
			return float64(h.HealthScore)
		})
		if scoreTrend < -0.5 {
			// 预计达到失败阈值的时间
			daysUntilFailure := int(float64(disk.HealthScore.Score-30) / (-scoreTrend * 24))
			if daysUntilFailure > 0 && daysUntilFailure < 90 {
				eta := time.Now().AddDate(0, 0, daysUntilFailure)
				disk.Predictions = append(disk.Predictions, &Prediction{
					Type:        "health",
					Probability: 0.8,
					ETA:         eta,
					Description: fmt.Sprintf("按当前趋势，预计约 %d 天后健康评分降至危险水平", daysUntilFailure),
					Confidence:  0.65,
				})
			}
		}
	}
}

// calculateTrendValue 计算趋势值
func (m *SMARTMonitor) calculateTrendValue(history []*SMARTHistoryPoint, getValue func(*SMARTHistoryPoint) float64) float64 {
	if len(history) < 2 {
		return 0
	}

	n := len(history)
	var sumX, sumY, sumXY, sumX2 float64

	for i, h := range history {
		x := float64(i)
		y := getValue(h)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	// 线性回归斜率
	denominator := float64(n)*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	slope := (float64(n)*sumXY - sumX*sumY) / denominator
	return slope
}

// GetDiskInfo 获取磁盘信息
func (m *SMARTMonitor) GetDiskInfo(device string) (*DiskInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disk, exists := m.disks[device]
	if !exists {
		return nil, fmt.Errorf("磁盘不存在: %s", device)
	}
	return disk, nil
}

// GetAllDisks 获取所有磁盘信息
func (m *SMARTMonitor) GetAllDisks() []*DiskInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	disks := make([]*DiskInfo, 0, len(m.disks))
	for _, disk := range m.disks {
		disks = append(disks, disk)
	}
	return disks
}

// GetAlerts 获取告警列表
func (m *SMARTMonitor) GetAlerts(device string, includeAcknowledged bool) []*SMARTAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]*SMARTAlert, 0)
	for _, alert := range m.alerts {
		if device != "" && alert.Device != device {
			continue
		}
		if !includeAcknowledged && alert.Acknowledged {
			continue
		}
		alerts = append(alerts, alert)
	}
	return alerts
}

// AcknowledgeAlert 确认告警
func (m *SMARTMonitor) AcknowledgeAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.alerts {
		if alert.ID == alertID {
			alert.Acknowledged = true
			return nil
		}
	}
	return fmt.Errorf("告警不存在: %s", alertID)
}

// SetAlertRule 设置告警规则
func (m *SMARTMonitor) SetAlertRule(rule *AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setAlertRuleLocked(rule)
}

// setAlertRuleLocked 设置告警规则（调用者必须持有锁）
func (m *SMARTMonitor) setAlertRuleLocked(rule *AlertRule) {
	// 查找并更新或添加
	for i, r := range m.alertRules {
		if r.ID == rule.ID {
			m.alertRules[i] = rule
			return
		}
	}
	m.alertRules = append(m.alertRules, rule)
}

// GetAlertRules 获取告警规则
func (m *SMARTMonitor) GetAlertRules() []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*AlertRule, len(m.alertRules))
	copy(rules, m.alertRules)
	return rules
}

// SetScoreWeights 设置评分权重
func (m *SMARTMonitor) SetScoreWeights(weights *ScoreWeights) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 确保权重总和为 1
	total := weights.Temperature + weights.Reallocation + weights.Pending +
		weights.Errors + weights.Age + weights.Stability
	if total != 1.0 {
		// 归一化
		weights.Temperature /= total
		weights.Reallocation /= total
		weights.Pending /= total
		weights.Errors /= total
		weights.Age /= total
		weights.Stability /= total
	}

	m.scoreWeights = weights
}

// GetHistory 获取历史数据
func (m *SMARTMonitor) GetHistory(device string, duration time.Duration) []*SMARTHistoryPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	history, exists := m.history[device]
	if !exists {
		return nil
	}

	cutoff := time.Now().Add(-duration)
	var result []*SMARTHistoryPoint
	for _, point := range history {
		if point.Timestamp.After(cutoff) {
			result = append(result, point)
		}
	}
	return result
}

// Stop 停止监控
func (m *SMARTMonitor) Stop() {
	close(m.stopChan)
}

// parseSize 解析大小字符串
func parseSize(sizeStr string) uint64 {
	sizeStr = strings.ToUpper(sizeStr)
	multiplier := uint64(1)

	if strings.HasSuffix(sizeStr, "T") {
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

	size, _ := strconv.ParseUint(sizeStr, 10, 64)
	return size * multiplier
}

// generateAlertID 生成告警 ID
func generateAlertID() string {
	return fmt.Sprintf("alert-%d", time.Now().UnixNano())
}

// ExportJSON 导出为 JSON
func (m *SMARTMonitor) ExportJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	export := struct {
		Disks      []*DiskInfo   `json:"disks"`
		Alerts     []*SMARTAlert `json:"alerts"`
		AlertRules []*AlertRule  `json:"alertRules"`
		Timestamp  time.Time     `json:"timestamp"`
	}{
		Disks:      m.GetAllDisks(),
		Alerts:     m.alerts,
		AlertRules: m.alertRules,
		Timestamp:  time.Now(),
	}

	return json.MarshalIndent(export, "", "  ")
}

// ImportJSON 从 JSON 导入配置
func (m *SMARTMonitor) ImportJSON(data []byte) error {
	var importData struct {
		AlertRules []*AlertRule `json:"alertRules"`
	}

	if err := json.Unmarshal(data, &importData); err != nil {
		return fmt.Errorf("解析 JSON 失败: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 更新告警规则
	for _, rule := range importData.AlertRules {
		if rule.LastTriggered == nil {
			rule.LastTriggered = make(map[string]time.Time)
		}
		m.setAlertRuleLocked(rule)
	}

	return nil
}

// RunHealthCheck 执行健康检查（实现 health.HealthChecker 接口）
func (m *SMARTMonitor) RunHealthCheck(_ context.Context) map[string]interface{} {
	_ = m.CheckAllDisks()

	result := make(map[string]interface{})
	disks := m.GetAllDisks()

	healthyCount := 0
	warningCount := 0
	criticalCount := 0

	for _, disk := range disks {
		switch disk.Status {
		case StatusHealthy:
			healthyCount++
		case StatusWarning:
			warningCount++
		case StatusCritical:
			criticalCount++
		}
	}

	result["total_disks"] = len(disks)
	result["healthy"] = healthyCount
	result["warning"] = warningCount
	result["critical"] = criticalCount
	result["disks"] = disks

	return result
}

// safeUint64ToBoundedInt 安全地将 uint64 转换为有边界的 int
// 用于温度等已知范围的值
func safeUint64ToBoundedInt(v uint64, min, max int) int {
	if v < uint64(min) {
		return min
	}
	if v > uint64(max) {
		return max
	}
	return int(v)
}
