package nvme

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Monitor NVMe健康监控服务
// 对标: TrueNAS NVMe S.M.A.R.T.监控、群晖SSD健康监控

// HealthStatus NVMe健康状态
type HealthStatus struct {
	Device         string    // 设备路径 (/dev/nvme0n1)
	Model         string    // 型号
	Serial        string    // 序列号
	Temperature   int       // 温度 (摄氏度)
	PercentUsed   float64   // 已用寿命百分比
	AvailableSpare float64  // 可用备用空间百分比
	CriticalWarning int     // 关 critical警告标志
	PowerCycles   uint64    // 电源循环次数
	PowerOnHours  uint64    // 开机小时数
	DataUnitsRead  uint64   // 读取数据单位
	DataUnitsWrite uint64   // 写入数据单位
	MediaErrors   uint64    // 媒体错误数
	NumErrors     uint64    // 错误计数
	SmartStatus   string    // SMART状态 (healthy/warning/critical)
	LastChecked   time.Time // 最后检查时间
}

// AlertConfig 告警配置
type AlertConfig struct {
	TemperatureThreshold   int     // 温度阈值 (摄氏度)
	PercentUsedThreshold   float64 // 寿命阈值 (百分比)
	AvailableSpareThreshold float64 // 备用空间阈值
	NotifyChannels         []string // 通知渠道 (email/webhook)
}

// DefaultAlertConfig 默认告警配置
func DefaultAlertConfig() AlertConfig {
	return AlertConfig{
		TemperatureThreshold:    70,   // 70度告警
		PercentUsedThreshold:    90,   // 90%寿命告警
		AvailableSpareThreshold: 10,   // 10%备用空间告警
		NotifyChannels:         []string{"webhook"},
	}
}

// NVMeMonitor NVMe监控器
type NVMeMonitor struct {
	config    AlertConfig
	devices   map[string]*HealthStatus
	alertChan chan Alert
	mu        sync.RWMutex
}

// Alert 告警消息
type Alert struct {
	Device    string
	Type      string // temperature/lifespan/spare/media_error
	Severity  string // warning/critical
	Message   string
	Timestamp time.Time
}

// NewNVMeMonitor 创建NVMe监控器
func NewNVMeMonitor(cfg AlertConfig) *NVMeMonitor {
	return &NVMeMonitor{
		config:    cfg,
		devices:   make(map[string]*HealthStatus),
		alertChan: make(chan Alert, 100),
	}
}

// DiscoverDevices 发现所有NVMe设备
func (m *NVMeMonitor) DiscoverDevices(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "nvme", "list")
	output, err := cmd.Output()
	if err != nil {
		// 尝试使用smartctl
		cmd = exec.CommandContext(ctx, "smartctl", "--scan")
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("discover nvme devices: %w", err)
		}
	}
	
	// 解析设备列表
	devices := parseNVMeList(string(output))
	
	// 存储设备信息
	m.mu.Lock()
	for _, dev := range devices {
		m.devices[dev] = nil
	}
	m.mu.Unlock()
	
	return devices, nil
}

// CheckHealth 检查单个设备健康状态
func (m *NVMeMonitor) CheckHealth(ctx context.Context, device string) (*HealthStatus, error) {
	// 使用nvme-cli获取SMART数据
	cmd := exec.CommandContext(ctx, "nvme", "smart-log", device)
	output, err := cmd.Output()
	if err != nil {
		// 尝试smartctl
		cmd = exec.CommandContext(ctx, "smartctl", "-a", device)
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("get smart log for %s: %w", device, err)
		}
	}
	
	status := parseSmartLog(device, string(output))
	status.LastChecked = time.Now()
	
	// 存储状态
	m.mu.Lock()
	m.devices[device] = status
	m.mu.Unlock()
	
	// 检查告警条件
	m.checkAlerts(status)
	
	return status, nil
}

// CheckAllHealth 检查所有设备健康状态
func (m *NVMeMonitor) CheckAllHealth(ctx context.Context) ([]HealthStatus, error) {
	devices, err := m.DiscoverDevices(ctx)
	if err != nil {
		return nil, err
	}
	
	results := make([]HealthStatus, len(devices))
	
	for i, dev := range devices {
		status, err := m.CheckHealth(ctx, dev)
		if err != nil {
			results[i] = HealthStatus{
				Device:     dev,
				SmartStatus: "unknown",
			}
			continue
		}
		results[i] = *status
	}
	
	return results, nil
}

// checkAlerts 检查告警条件
func (m *NVMeMonitor) checkAlerts(status *HealthStatus) {
	// 温度告警
	if status.Temperature >= m.config.TemperatureThreshold {
		severity := "warning"
		if status.Temperature >= 80 {
			severity = "critical"
		}
		m.alertChan <- Alert{
			Device:    status.Device,
			Type:      "temperature",
			Severity:  severity,
			Message:   fmt.Sprintf("NVMe温度过高: %d°C", status.Temperature),
			Timestamp: time.Now(),
		}
	}
	
	// 寿命告警
	if status.PercentUsed >= m.config.PercentUsedThreshold {
		severity := "warning"
		if status.PercentUsed >= 95 {
			severity = "critical"
		}
		m.alertChan <- Alert{
			Device:    status.Device,
			Type:      "lifespan",
			Severity:  severity,
			Message:   fmt.Sprintf("NVMe寿命告警: 已使用%.1f%%", status.PercentUsed),
			Timestamp: time.Now(),
		}
	}
	
	// 备用空间告警
	if status.AvailableSpare <= m.config.AvailableSpareThreshold {
		m.alertChan <- Alert{
			Device:    status.Device,
			Type:      "spare",
			Severity:  "critical",
			Message:   fmt.Sprintf("NVMe备用空间不足: %.1f%%", status.AvailableSpare),
			Timestamp: time.Now(),
		}
	}
	
	// 媒体错误告警
	if status.MediaErrors > 0 {
		m.alertChan <- Alert{
			Device:    status.Device,
			Type:      "media_error",
			Severity:  "warning",
			Message:   fmt.Sprintf("NVMe媒体错误: %d次", status.MediaErrors),
			Timestamp: time.Now(),
		}
	}
	
	// 确定SMART状态
	critical := status.CriticalWarning > 0 || status.PercentUsed >= 95 || status.AvailableSpare <= 5
	warning := status.PercentUsed >= m.config.PercentUsedThreshold || status.Temperature >= m.config.TemperatureThreshold
	
	if critical {
		status.SmartStatus = "critical"
	} else if warning {
		status.SmartStatus = "warning"
	} else {
		status.SmartStatus = "healthy"
	}
}

// Alerts 获取告警通道
func (m *NVMeMonitor) Alerts() <-chan Alert {
	return m.alertChan
}

// GetAllStatus 获取所有设备状态
func (m *NVMeMonitor) GetAllStatus() map[string]*HealthStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.devices
}

// StartMonitoring 启动后台监控
func (m *NVMeMonitor) StartMonitoring(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = m.CheckAllHealth(ctx)
		}
	}
}

// ExportMetrics 导出监控指标 (Prometheus格式)
func (m *NVMeMonitor) ExportMetrics() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var metrics strings.Builder
	for dev, status := range m.devices {
		if status == nil {
			continue
		}
		
		deviceLabel := strings.ReplaceAll(dev, "/", "_")
		
		metrics.WriteString(fmt.Sprintf("nvme_temperature{device=\"%s\"} %d\n", deviceLabel, status.Temperature))
		metrics.WriteString(fmt.Sprintf("nvme_percent_used{device=\"%s\"} %.1f\n", deviceLabel, status.PercentUsed))
		metrics.WriteString(fmt.Sprintf("nvme_available_spare{device=\"%s\"} %.1f\n", deviceLabel, status.AvailableSpare))
		metrics.WriteString(fmt.Sprintf("nvme_power_cycles{device=\"%s\"} %d\n", deviceLabel, status.PowerCycles))
		metrics.WriteString(fmt.Sprintf("nvme_power_on_hours{device=\"%s\"} %d\n", deviceLabel, status.PowerOnHours))
		metrics.WriteString(fmt.Sprintf("nvme_media_errors{device=\"%s\"} %d\n", deviceLabel, status.MediaErrors))
	}
	
	return metrics.String()
}

// parseNVMeList 解析nvme list输出
func parseNVMeList(output string) []string {
	devices := []string{}
	lines := strings.Split(output, "\n")
	
	for _, line := range lines {
		if strings.Contains(line, "/dev/nvme") {
			// 提取设备路径
			re := regexp.MustCompile(`/dev/nvme\d+n\d+`)
			match := re.FindString(line)
			if match != "" {
				devices = append(devices, match)
			}
		}
	}
	
	return devices
}

// parseSmartLog 解析SMART日志
func parseSmartLog(device, output string) *HealthStatus {
	status := &HealthStatus{
		Device: device,
	}
	
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// 温度
		if strings.Contains(line, "temperature") {
			re := regexp.MustCompile(`(\d+)\s*C`)
			match := re.FindString(line)
			if match != "" {
				status.Temperature, _ = strconv.Atoi(strings.TrimSuffix(match, " C"))
			}
		}
		
		// 已用寿命百分比
		if strings.Contains(line, "percentage used") || strings.Contains(line, "Percentage Used") {
			re := regexp.MustCompile(`(\d+\.?\d*)\s*%`)
			match := re.FindString(line)
			if match != "" {
				val := strings.TrimSuffix(match, "%")
				status.PercentUsed, _ = strconv.ParseFloat(val, 64)
			}
		}
		
		// 可用备用空间
		if strings.Contains(line, "available spare") || strings.Contains(line, "Available Spare") {
			re := regexp.MustCompile(`(\d+\.?\d*)\s*%`)
			match := re.FindString(line)
			if match != "" {
				val := strings.TrimSuffix(match, "%")
				status.AvailableSpare, _ = strconv.ParseFloat(val, 64)
			}
		}
		
		// 媒体错误
		if strings.Contains(line, "media errors") || strings.Contains(line, "Media Errors") {
			re := regexp.MustCompile(`(\d+)`)
			match := re.FindString(line)
			if match != "" {
				status.MediaErrors, _ = strconv.ParseUint(match, 10, 64)
			}
		}
	}
	
	return status
}

// DashboardData 存储安全看板数据
type DashboardData struct {
	Devices       []HealthStatus
	CriticalCount int
	WarningCount  int
	HealthyCount  int
	LastUpdate    time.Time
}

// GetDashboard 获取看板数据
func (m *NVMeMonitor) GetDashboard() DashboardData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	data := DashboardData{
		LastUpdate: time.Now(),
	}
	
	for _, status := range m.devices {
		if status == nil {
			continue
		}
		data.Devices = append(data.Devices, *status)
		
		switch status.SmartStatus {
		case "critical":
			data.CriticalCount++
		case "warning":
			data.WarningCount++
		default:
			data.HealthyCount++
		}
	}
	
	return data
}

// ToJSON 导出为JSON
func (s *HealthStatus) ToJSON() string {
	data, _ := json.MarshalIndent(s, "", "  ")
	return string(data)
}