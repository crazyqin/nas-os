// Package hwcompat 实现硬件兼容性检测模块，对标飞牛硬件兼容性检测
package hwcompat

import (
	"fmt"
	"sync"
	"time"
)

// HWCompatChecker 硬件兼容性检测器
type HWCompatChecker struct {
	mu              sync.RWMutex
	hardware        *HardwareInfo
	drivers         map[string]*DriverInfo
	compatRules     []*CompatRule
	reports         map[string]*CompatReport
	tempMonitor     *TempMonitor
	stopChan        chan struct{}
}

// HardwareInfo 硬件信息
type HardwareInfo struct {
	CPU       *CPUInfo       `json:"cpu"`
	Memory    *MemoryInfo    `json:"memory"`
	Disks     []*DiskInfo    `json:"disks"`
	Network   []*NetworkInfo `json:"network"`
	GPU       []*GPUInfo     `json:"gpu"`
	Motherboard *MotherboardInfo `json:"motherboard"`
	Timestamp time.Time      `json:"timestamp"`
}

// CPUInfo CPU 信息
type CPUInfo struct {
	Model       string  `json:"model"`
	Vendor      string  `json:"vendor"`
	Cores       int     `json:"cores"`
	Threads     int     `json:"threads"`
	Frequency   float64 `json:"frequency"`     // GHz
	CacheSize   int64   `json:"cache_size"`    // bytes
	Flags       []string `json:"flags"`
	VTEnabled   bool    `json:"vt_enabled"`   // 虚拟化支持
	AESNI       bool    `json:"aes_ni"`       // AES-NI 支持
	Temperature float64 `json:"temperature"`
}

// MemoryInfo 内存信息
type MemoryInfo struct {
	Total       int64          `json:"total"`
	Used        int64          `json:"used"`
	Free        int64          `json:"free"`
	Buffers     int64          `json:"buffers"`
	Cached      int64          `json:"cached"`
	Modules     []*MemoryModule `json:"modules"`
	ECC         bool           `json:"ecc"`
}

// MemoryModule 内存模块
type MemoryModule struct {
	Slot     string `json:"slot"`
	Size     int64  `json:"size"`
	Type     string `json:"type"`     // DDR4, DDR5
	Speed    int    `json:"speed"`    // MHz
	Manufacturer string `json:"manufacturer"`
	PartNumber string `json:"part_number"`
}

// DiskInfo 磁盘信息
type DiskInfo struct {
	ID              string       `json:"id"`
	Name            string       `json:"name"`
	Path            string       `json:"path"`
	Model           string       `json:"model"`
	Serial          string       `json:"serial"`
	Size            int64        `json:"size"`
	Type            string       `json:"type"`     // ssd, hdd, nvme
	Interface       string       `json:"interface"` // sata, sas, nvme
	RotationSpeed   int          `json:"rotation_speed"`
	Temperature     float64      `json:"temperature"`
	PowerOnHours    int64        `json:"power_on_hours"`
	SMARTStatus     SMARTStatus  `json:"smart_status"`
	Partitions      []*Partition `json:"partitions"`
}

// SMARTStatus SMART 状态
type SMARTStatus struct {
	Healthy             bool  `json:"healthy"`
	ReallocatedSectors  int64 `json:"reallocated_sectors"`
	PendingSectors      int64 `json:"pending_sectors"`
	OfflineUncorrectable int64 `json:"offline_uncorrectable"`
}

// Partition 分区
type Partition struct {
	Name   string `json:"name"`
	Start  int64  `json:"start"`
	Size   int64  `json:"size"`
	Type   string `json:"type"`
	UUID   string `json:"uuid"`
}

// NetworkInfo 网卡信息
type NetworkInfo struct {
	Name         string   `json:"name"`
	MAC          string   `json:"mac"`
	IP           []string `json:"ip"`
	Speed        int      `json:"speed"`      // Mbps
	Driver       string   `json:"driver"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
	LinkUp       bool     `json:"link_up"`
	RingBufferSize int    `json:"ring_buffer_size"`
}

// GPUInfo GPU 信息
type GPUInfo struct {
	Name         string `json:"name"`
	Vendor       string `json:"vendor"`
	Memory       int64  `json:"memory"`
	Driver       string `json:"driver"`
	PCIID        string `json:"pci_id"`
	Temperature  float64 `json:"temperature"`
	PowerUsage   int    `json:"power_usage"`
}

// MotherboardInfo 主板信息
type MotherboardInfo struct {
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	BIOSVersion  string `json:"bios_version"`
	Chipset      string `json:"chipset"`
	Slots        int    `json:"slots"`
	SATAPorts    int    `json:"sata_ports"`
	NVMESlots    int    `json:"nvme_slots"`
}

// DriverInfo 驱动信息
type DriverInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Device      string `json:"device"`
	Status      DriverStatus `json:"status"`
	Loaded      bool   `json:"loaded"`
	Kernel      string `json:"kernel"`
}

// DriverStatus 驱动状态
type DriverStatus string

const (
	DriverOK       DriverStatus = "ok"
	DriverMissing  DriverStatus = "missing"
	DriverOutdated DriverStatus = "outdated"
	DriverFailed   DriverStatus = "failed"
)

// CompatRule 兼容性规则
type CompatRule struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Category    CompatCategory `json:"category"`
	Severity    CompatSeverity `json:"severity"`
	Validator   func(*HardwareInfo) *CompatIssue `json:"-"`
}

// CompatCategory 兼容性类别
type CompatCategory string

const (
	CategoryCPU    CompatCategory = "cpu"
	CategoryMemory CompatCategory = "memory"
	CategoryDisk   CompatCategory = "disk"
	CategoryNetwork CompatCategory = "network"
	CategoryGPU    CompatCategory = "gpu"
	CategorySystem CompatCategory = "system"
)

// CompatSeverity 兼容性严重程度
type CompatSeverity string

const (
	SeverityInfo     CompatSeverity = "info"
	SeverityWarning  CompatSeverity = "warning"
	SeverityCritical CompatSeverity = "critical"
	SeverityBlocker  CompatSeverity = "blocker"
)

// CompatIssue 兼容性问题
type CompatIssue struct {
	RuleID      string         `json:"rule_id"`
	RuleName    string         `json:"rule_name"`
	Category    CompatCategory `json:"category"`
	Severity    CompatSeverity `json:"severity"`
	Message     string         `json:"message"`
	Suggestion  string         `json:"suggestion,omitempty"`
	Timestamp   time.Time      `json:"timestamp"`
}

// CompatReport 兼容性报告
type CompatReport struct {
	ID              string          `json:"id"`
	Hardware        *HardwareInfo   `json:"hardware"`
	Score           float64         `json:"score"`        // 0-100
	PerformanceLevel string         `json:"performance_level"`
	Issues          []*CompatIssue  `json:"issues"`
	DriverStatus    map[string]*DriverInfo `json:"driver_status"`
	Temperature     *TempStatus     `json:"temperature"`
	Recommendations []string        `json:"recommendations"`
	Timestamp       time.Time       `json:"timestamp"`
}

// TempMonitor 温度监控器
type TempMonitor struct {
	mu          sync.RWMutex
	sensors     map[string]*TempSensor
	thresholds  *TempThresholds
	alerts      []*TempAlert
}

// TempSensor 温度传感器
type TempSensor struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`   // cpu, disk, gpu, system
	Current     float64   `json:"current"`
	Min         float64   `json:"min"`
	Max         float64   `json:"max"`
	Critical    float64   `json:"critical"`
	Timestamp   time.Time `json:"timestamp"`
}

// TempStatus 温度状态
type TempStatus struct {
	CPU    float64 `json:"cpu"`
	MaxDisk float64 `json:"max_disk"`
	GPU    float64 `json:"gpu"`
	System float64 `json:"system"`
	Status string  `json:"status"`
}

// TempThresholds 温度阈值
type TempThresholds struct {
	CPUWarning    float64 `json:"cpu_warning"`
	CPUCritical   float64 `json:"cpu_critical"`
	DiskWarning   float64 `json:"disk_warning"`
	DiskCritical  float64 `json:"disk_critical"`
	GPUWarning    float64 `json:"gpu_warning"`
	GPUCritical   float64 `json:"gpu_critical"`
}

// TempAlert 温度告警
type TempAlert struct {
	SensorID  string    `json:"sensor_id"`
	SensorName string   `json:"sensor_name"`
	Threshold float64   `json:"threshold"`
	Current   float64   `json:"current"`
	Timestamp time.Time `json:"timestamp"`
}

// NewHWCompatChecker 创建硬件兼容性检测器
func NewHWCompatChecker() *HWCompatChecker {
	checker := &HWCompatChecker{
		drivers:     make(map[string]*DriverInfo),
		compatRules: make([]*CompatRule, 0),
		reports:     make(map[string]*CompatReport),
		stopChan:    make(chan struct{}),
		tempMonitor: &TempMonitor{
			sensors: make(map[string]*TempSensor),
			thresholds: &TempThresholds{
				CPUWarning:   70,
				CPUCritical:  85,
				DiskWarning:  50,
				DiskCritical: 60,
				GPUWarning:   80,
				GPUCritical:  90,
			},
		},
	}

	// 初始化默认兼容性规则
	checker.initDefaultRules()
	return checker
}

// initDefaultRules 初始化默认兼容性规则
func (c *HWCompatChecker) initDefaultRules() {
	c.compatRules = append(c.compatRules,
		&CompatRule{
			ID:          "cpu_cores",
			Name:        "CPU 核心数检查",
			Description: "NAS 建议至少 4 核心",
			Category:    CategoryCPU,
			Severity:    SeverityWarning,
			Validator: func(hw *HardwareInfo) *CompatIssue {
				if hw.CPU != nil && hw.CPU.Cores < 4 {
					return &CompatIssue{
						RuleID:   "cpu_cores",
						RuleName: "CPU 核心数检查",
						Category: CategoryCPU,
						Severity: SeverityWarning,
						Message:  fmt.Sprintf("CPU 核心数不足: %d 核 (建议至少 4 核)", hw.CPU.Cores),
						Suggestion: "建议使用 4 核心以上的 CPU",
					}
				}
				return nil
			},
		},
		&CompatRule{
			ID:          "memory_size",
			Name:        "内存大小检查",
			Description: "NAS 建议至少 8GB 内存",
			Category:    CategoryMemory,
			Severity:    SeverityWarning,
			Validator: func(hw *HardwareInfo) *CompatIssue {
				if hw.Memory != nil && hw.Memory.Total < 8*1024*1024*1024 {
					return &CompatIssue{
						RuleID:   "memory_size",
						RuleName: "内存大小检查",
						Category: CategoryMemory,
						Severity: SeverityWarning,
						Message:  fmt.Sprintf("内存不足: %d GB (建议至少 8 GB)", hw.Memory.Total/1024/1024/1024),
						Suggestion: "建议增加内存到 8GB 以上",
					}
				}
				return nil
			},
		},
		&CompatRule{
			ID:          "disk_count",
			Name:        "磁盘数量检查",
			Description: "NAS 建议至少 2 块磁盘",
			Category:    CategoryDisk,
			Severity:    SeverityInfo,
			Validator: func(hw *HardwareInfo) *CompatIssue {
				if len(hw.Disks) < 2 {
					return &CompatIssue{
						RuleID:   "disk_count",
						RuleName: "磁盘数量检查",
						Category: CategoryDisk,
						Severity: SeverityInfo,
						Message:  fmt.Sprintf("磁盘数量不足: %d 块 (建议至少 2 块)", len(hw.Disks)),
						Suggestion: "建议增加磁盘以实现 RAID 冗余",
					}
				}
				return nil
			},
		},
		&CompatRule{
			ID:          "virtualization",
			Name:        "虚拟化支持检查",
			Description: "NAS 建议启用虚拟化",
			Category:    CategoryCPU,
			Severity:    SeverityWarning,
			Validator: func(hw *HardwareInfo) *CompatIssue {
				if hw.CPU != nil && !hw.CPU.VTEnabled {
					return &CompatIssue{
						RuleID:   "virtualization",
						RuleName: "虚拟化支持检查",
						Category: CategoryCPU,
						Severity: SeverityWarning,
						Message:  "CPU 未启用虚拟化支持",
						Suggestion: "建议在 BIOS 中启用 VT-x/AMD-V",
					}
				}
				return nil
			},
		},
	)
}

// ScanHardware 扫描硬件
func (c *HWCompatChecker) ScanHardware() (*HardwareInfo, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 实际实现需要读取系统信息
	hw := &HardwareInfo{
		Timestamp: time.Now(),
	}

	c.hardware = hw
	return hw, nil
}

// GetHardware 获取硬件信息
func (c *HWCompatChecker) GetHardware() *HardwareInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.hardware
}

// RegisterDriver 注册驱动信息
func (c *HWCompatChecker) RegisterDriver(driver *DriverInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.drivers[driver.Name] = driver
}

// GetDriver 获取驱动信息
func (c *HWCompatChecker) GetDriver(name string) (*DriverInfo, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	driver, exists := c.drivers[name]
	if !exists {
		return nil, fmt.Errorf("驱动 %s 不存在", name)
	}
	return driver, nil
}

// ListDrivers 列出所有驱动
func (c *HWCompatChecker) ListDrivers() []*DriverInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	drivers := make([]*DriverInfo, 0, len(c.drivers))
	for _, d := range c.drivers {
		drivers = append(drivers, d)
	}
	return drivers
}

// CheckDriverStatus 检查驱动状态
func (c *HWCompatChecker) CheckDriverStatus() map[string]*DriverStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	status := make(map[string]*DriverStatus)
	for name, driver := range c.drivers {
		status[name] = &driver.Status
	}
	return status
}

// AddCompatRule 添加兼容性规则
func (c *HWCompatChecker) AddCompatRule(rule *CompatRule) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.compatRules = append(c.compatRules, rule)
}

// ListCompatRules 列出兼容性规则
func (c *HWCompatChecker) ListCompatRules() []*CompatRule {
	c.mu.RLock()
	defer c.mu.RUnlock()

	rules := make([]*CompatRule, len(c.compatRules))
	copy(rules, c.compatRules)
	return rules
}

// RunCompatCheck 运行兼容性检查
func (c *HWCompatChecker) RunCompatCheck() (*CompatReport, error) {
	c.mu.RLock()
	hw := c.hardware
	rules := make([]*CompatRule, len(c.compatRules))
	copy(rules, c.compatRules)
	c.mu.RUnlock()

	if hw == nil {
		return nil, fmt.Errorf("硬件信息未初始化，请先运行 ScanHardware")
	}

	report := &CompatReport{
		ID:        fmt.Sprintf("compat_%d", time.Now().UnixNano()),
		Hardware:  hw,
		Issues:    make([]*CompatIssue, 0),
		Timestamp: time.Now(),
	}

	// 运行所有规则
	for _, rule := range rules {
		if issue := rule.Validator(hw); issue != nil {
			issue.Timestamp = time.Now()
			report.Issues = append(report.Issues, issue)
		}
	}

	// 计算兼容性得分
	report.Score = c.calculateScore(report.Issues)

	// 确定性能等级
	report.PerformanceLevel = c.determinePerformanceLevel(hw)

	// 获取驱动状态
	report.DriverStatus = make(map[string]*DriverInfo)
	c.mu.RLock()
	for name, driver := range c.drivers {
		report.DriverStatus[name] = driver
	}
	c.mu.RUnlock()

	// 生成建议
	report.Recommendations = c.generateRecommendations(report)

	// 保存报告
	c.mu.Lock()
	c.reports[report.ID] = report
	c.mu.Unlock()

	return report, nil
}

// calculateScore 计算兼容性得分
func (c *HWCompatChecker) calculateScore(issues []*CompatIssue) float64 {
	score := 100.0

	for _, issue := range issues {
		switch issue.Severity {
		case SeverityBlocker:
			score -= 30
		case SeverityCritical:
			score -= 20
		case SeverityWarning:
			score -= 10
		case SeverityInfo:
			score -= 5
		}
	}

	if score < 0 {
		score = 0
	}
	return score
}

// determinePerformanceLevel 确定性能等级
func (c *HWCompatChecker) determinePerformanceLevel(hw *HardwareInfo) string {
	score := 0

	// CPU 评分
	if hw.CPU != nil {
		if hw.CPU.Cores >= 8 {
			score += 30
		} else if hw.CPU.Cores >= 4 {
			score += 20
		} else {
			score += 10
		}
	}

	// 内存评分
	if hw.Memory != nil {
		memGB := hw.Memory.Total / 1024 / 1024 / 1024
		if memGB >= 32 {
			score += 30
		} else if memGB >= 16 {
			score += 20
		} else if memGB >= 8 {
			score += 10
		}
	}

	// 磁盘评分
	if len(hw.Disks) >= 4 {
		score += 20
	} else if len(hw.Disks) >= 2 {
		score += 10
	}

	// 网络评分
	if len(hw.Network) > 0 {
		for _, net := range hw.Network {
			if net.Speed >= 10000 {
				score += 20
				break
			} else if net.Speed >= 1000 {
				score += 10
				break
			}
		}
	}

	switch {
	case score >= 80:
		return "excellent"
	case score >= 60:
		return "good"
	case score >= 40:
		return "average"
	default:
		return "basic"
	}
}

// generateRecommendations 生成建议
func (c *HWCompatChecker) generateRecommendations(report *CompatReport) []string {
	recommendations := make([]string, 0)

	// 根据问题生成建议
	for _, issue := range report.Issues {
		if issue.Suggestion != "" {
			recommendations = append(recommendations, issue.Suggestion)
		}
	}

	// 根据性能等级生成建议
	switch report.PerformanceLevel {
	case "basic":
		recommendations = append(recommendations, "硬件配置较低，建议升级以获得更好的性能")
	case "average":
		recommendations = append(recommendations, "硬件配置一般，可以满足基本 NAS 需求")
	}

	return recommendations
}

// GetCompatReport 获取兼容性报告
func (c *HWCompatChecker) GetCompatReport(reportID string) (*CompatReport, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	report, exists := c.reports[reportID]
	if !exists {
		return nil, fmt.Errorf("报告 %s 不存在", reportID)
	}
	return report, nil
}

// ListCompatReports 列出兼容性报告
func (c *HWCompatChecker) ListCompatReports() []*CompatReport {
	c.mu.RLock()
	defer c.mu.RUnlock()

	reports := make([]*CompatReport, 0, len(c.reports))
	for _, r := range c.reports {
		reports = append(reports, r)
	}
	return reports
}

// UpdateTemperature 更新温度数据
func (c *HWCompatChecker) UpdateTemperature(sensor *TempSensor) {
	c.tempMonitor.mu.Lock()
	defer c.tempMonitor.mu.Unlock()

	sensor.Timestamp = time.Now()
	c.tempMonitor.sensors[sensor.ID] = sensor

	// 检查温度告警
	c.checkTempAlert(sensor)
}

// checkTempAlert 检查温度告警
func (c *HWCompatChecker) checkTempAlert(sensor *TempSensor) {
	thresholds := c.tempMonitor.thresholds
	var warningTemp, criticalTemp float64

	switch sensor.Type {
	case "cpu":
		warningTemp = thresholds.CPUWarning
		criticalTemp = thresholds.CPUCritical
	case "disk":
		warningTemp = thresholds.DiskWarning
		criticalTemp = thresholds.DiskCritical
	case "gpu":
		warningTemp = thresholds.GPUWarning
		criticalTemp = thresholds.GPUCritical
	default:
		return
	}

	if sensor.Current >= criticalTemp {
		alert := &TempAlert{
			SensorID:   sensor.ID,
			SensorName: sensor.Name,
			Threshold:  criticalTemp,
			Current:    sensor.Current,
			Timestamp:  time.Now(),
		}
		c.tempMonitor.alerts = append(c.tempMonitor.alerts, alert)
	} else if sensor.Current >= warningTemp {
		alert := &TempAlert{
			SensorID:   sensor.ID,
			SensorName: sensor.Name,
			Threshold:  warningTemp,
			Current:    sensor.Current,
			Timestamp:  time.Now(),
		}
		c.tempMonitor.alerts = append(c.tempMonitor.alerts, alert)
	}
}

// GetTemperatureStatus 获取温度状态
func (c *HWCompatChecker) GetTemperatureStatus() *TempStatus {
	c.tempMonitor.mu.RLock()
	defer c.tempMonitor.mu.RUnlock()

	status := &TempStatus{}
	maxDiskTemp := 0.0

	for _, sensor := range c.tempMonitor.sensors {
		switch sensor.Type {
		case "cpu":
			status.CPU = sensor.Current
		case "disk":
			if sensor.Current > maxDiskTemp {
				maxDiskTemp = sensor.Current
			}
		case "gpu":
			status.GPU = sensor.Current
		case "system":
			status.System = sensor.Current
		}
	}

	status.MaxDisk = maxDiskTemp

	// 判断总体状态
	thresholds := c.tempMonitor.thresholds
	if status.CPU >= thresholds.CPUCritical || status.MaxDisk >= thresholds.DiskCritical {
		status.Status = "critical"
	} else if status.CPU >= thresholds.CPUWarning || status.MaxDisk >= thresholds.DiskWarning {
		status.Status = "warning"
	} else {
		status.Status = "normal"
	}

	return status
}

// GetTempAlerts 获取温度告警
func (c *HWCompatChecker) GetTempAlerts() []*TempAlert {
	c.tempMonitor.mu.RLock()
	defer c.tempMonitor.mu.RUnlock()

	alerts := make([]*TempAlert, len(c.tempMonitor.alerts))
	copy(alerts, c.tempMonitor.alerts)
	return alerts
}

// GenerateHardwareReport 生成硬件报告
func (c *HWCompatChecker) GenerateHardwareReport() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	report := map[string]interface{}{
		"timestamp": time.Now(),
	}

	if c.hardware != nil {
		report["hardware"] = c.hardware
	}

	report["drivers"] = c.drivers
	report["temp_status"] = c.GetTemperatureStatus()
	report["temp_alerts"] = c.GetTempAlerts()

	// 计算驱动状态统计
	driverStats := map[string]int{
		"ok":       0,
		"missing":  0,
		"outdated": 0,
		"failed":   0,
	}
	for _, driver := range c.drivers {
		switch driver.Status {
		case DriverOK:
			driverStats["ok"]++
		case DriverMissing:
			driverStats["missing"]++
		case DriverOutdated:
			driverStats["outdated"]++
		case DriverFailed:
			driverStats["failed"]++
		}
	}
	report["driver_stats"] = driverStats

	return report
}

// Stop 停止硬件兼容性检测器
func (c *HWCompatChecker) Stop() {
	close(c.stopChan)
}
