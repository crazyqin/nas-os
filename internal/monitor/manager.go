package monitor

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Manager 监控管理器.
type Manager struct {
	hostname          string
	diskHealthMonitor *DiskHealthMonitor
	alertingManager   *AlertingManager
	backupStats       *BackupStats
}

// SystemStats 系统统计信息.
type SystemStats struct {
	CPUUsage      float64   `json:"cpuUsage"`
	MemoryUsage   float64   `json:"memoryUsage"`
	MemoryTotal   uint64    `json:"memoryTotal"`
	MemoryUsed    uint64    `json:"memoryUsed"`
	MemoryFree    uint64    `json:"memoryFree"`
	SwapUsage     float64   `json:"swapUsage"`
	SwapTotal     uint64    `json:"swapTotal"`
	SwapUsed      uint64    `json:"swapUsed"`
	Uptime        string    `json:"uptime"`
	UptimeSeconds uint64    `json:"uptimeSeconds"`
	LoadAvg       []float64 `json:"loadAvg"`
	Processes     int       `json:"processes"`
	Timestamp     time.Time `json:"timestamp"`
}

// DiskStats 磁盘统计信息.
type DiskStats struct {
	Device       string  `json:"device"`
	MountPoint   string  `json:"mountPoint"`
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usagePercent"`
	FSType       string  `json:"fsType"`
}

// NetworkStats 网络统计信息.
type NetworkStats struct {
	Interface string `json:"interface"`
	RXBytes   uint64 `json:"rxBytes"`
	TXBytes   uint64 `json:"txBytes"`
	RXPackets uint64 `json:"rxPackets"`
	TXPackets uint64 `json:"txPackets"`
	RXErrors  uint64 `json:"rxErrors"`
	TXErrors  uint64 `json:"txErrors"`
}

// SMARTInfo SMART 信息.
type SMARTInfo struct {
	Device       string `json:"device"`
	Model        string `json:"model"`
	Serial       string `json:"serial"`
	Temperature  int    `json:"temperature"`
	Health       string `json:"health"`
	PowerOnHours uint64 `json:"powerOnHours"`
	ReadErrors   uint64 `json:"readErrors"`
	WriteErrors  uint64 `json:"writeErrors"`
}

// Alert 告警信息.
type Alert struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`  // cpu, memory, disk, smart
	Level        string    `json:"level"` // warning, critical
	Message      string    `json:"message"`
	Source       string    `json:"source"`
	Timestamp    time.Time `json:"timestamp"`
	Acknowledged bool      `json:"acknowledged"`
}

// AlertRule 告警规则.
type AlertRule struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Threshold float64 `json:"threshold"`
	Level     string  `json:"level"`
	Enabled   bool    `json:"enabled"`
}

// NewManager 创建监控管理器.
func NewManager() (*Manager, error) {
	hostname, _ := os.Hostname()
	return &Manager{
		hostname: hostname,
	}, nil
}

// NewManagerWithComponents 创建带组件的监控管理器 (v2.59.0).
func NewManagerWithComponents(diskMonitor *DiskHealthMonitor, alertMgr *AlertingManager) (*Manager, error) {
	hostname, _ := os.Hostname()
	return &Manager{
		hostname:          hostname,
		diskHealthMonitor: diskMonitor,
		alertingManager:   alertMgr,
	}, nil
}

// SetDiskHealthMonitor 设置磁盘健康监控器 (v2.59.0).
func (m *Manager) SetDiskHealthMonitor(monitor *DiskHealthMonitor) {
	m.diskHealthMonitor = monitor
}

// SetAlertingManager 设置告警管理器 (v2.59.0).
func (m *Manager) SetAlertingManager(mgr *AlertingManager) {
	m.alertingManager = mgr
}

// GetDiskHealthMonitor 获取磁盘健康监控器 (v2.59.0).
func (m *Manager) GetDiskHealthMonitor() *DiskHealthMonitor {
	return m.diskHealthMonitor
}

// GetAlertingManager 获取告警管理器 (v2.59.0).
func (m *Manager) GetAlertingManager() *AlertingManager {
	return m.alertingManager
}

// GetBackupStats 获取备份统计 (v2.59.0).
func (m *Manager) GetBackupStats() (*BackupStats, error) {
	// 如果有缓存的备份数据，返回它
	if m.backupStats != nil {
		return m.backupStats, nil
	}

	// 否则返回一个空的统计结构
	// 实际实现应该从备份服务或存储中获取数据
	return nil, fmt.Errorf("备份统计未初始化")
}

// SetBackupStats 设置备份统计 (v2.59.0).
func (m *Manager) SetBackupStats(stats *BackupStats) {
	m.backupStats = stats
}

// UpdateBackupStats 更新备份统计 (v2.59.0).
func (m *Manager) UpdateBackupStats(totalCount, fullCount, incrementalCount, databaseCount, configCount int, totalSize, spaceUsed, spaceTotal, spaceAvailable uint64) {
	m.backupStats = &BackupStats{
		TotalCount:       totalCount,
		FullCount:        fullCount,
		IncrementalCount: incrementalCount,
		DatabaseCount:    databaseCount,
		ConfigCount:      configCount,
		TotalSize:        totalSize,
		SpaceUsed:        spaceUsed,
		SpaceTotal:       spaceTotal,
		SpaceAvailable:   spaceAvailable,
	}
}

// GetSystemStats 获取系统统计信息.
func (m *Manager) GetSystemStats() (*SystemStats, error) {
	stats := &SystemStats{
		Timestamp: time.Now(),
		LoadAvg:   make([]float64, 3),
	}

	// CPU 使用率
	cpuUsage, err := m.getCPUUsage()
	if err == nil {
		stats.CPUUsage = cpuUsage
	}

	// 内存使用
	if memInfo, err := m.getMemoryInfo(); err == nil {
		stats.MemoryTotal = memInfo["Total"]
		stats.MemoryFree = memInfo["Free"]
		stats.MemoryUsed = memInfo["Total"] - memInfo["Free"]
		if stats.MemoryTotal > 0 {
			stats.MemoryUsage = float64(stats.MemoryUsed) / float64(stats.MemoryTotal) * 100
		}
	}

	// Swap 使用
	if swapInfo, err := m.getSwapInfo(); err == nil {
		stats.SwapTotal = swapInfo["Total"]
		stats.SwapUsed = swapInfo["Used"]
		if stats.SwapTotal > 0 {
			stats.SwapUsage = float64(stats.SwapUsed) / float64(stats.SwapTotal) * 100
		}
	}

	// 运行时间
	if uptime, err := m.getUptime(); err == nil {
		stats.UptimeSeconds = uptime
		stats.Uptime = m.formatUptime(uptime)
	}

	// 负载均衡
	if loadAvg, err := m.getLoadAverage(); err == nil {
		stats.LoadAvg = loadAvg
	}

	// 进程数
	stats.Processes = runtime.NumGoroutine()

	return stats, nil
}

// GetDiskStats 获取磁盘统计信息.
func (m *Manager) GetDiskStats() ([]*DiskStats, error) {
	var stats []*DiskStats

	// 使用 df 命令获取磁盘信息
	cmd := exec.CommandContext(context.Background(), "df", "-B1", "--output=source,target,size,used,avail,fstype")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法获取磁盘信息: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	scanner.Scan() // 跳过标题行

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		total, _ := strconv.ParseUint(fields[2], 10, 64)
		used, _ := strconv.ParseUint(fields[3], 10, 64)
		free, _ := strconv.ParseUint(fields[4], 10, 64)

		var usagePercent float64
		if total > 0 {
			usagePercent = float64(used) / float64(total) * 100
		}

		stats = append(stats, &DiskStats{
			Device:       fields[0],
			MountPoint:   fields[1],
			Total:        total,
			Used:         used,
			Free:         free,
			UsagePercent: usagePercent,
			FSType:       fields[5],
		})
	}

	return stats, nil
}

// GetNetworkStats 获取网络统计信息.
func (m *Manager) GetNetworkStats() ([]*NetworkStats, error) {
	var stats []*NetworkStats

	// 读取 /proc/net/dev
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil, fmt.Errorf("无法读取网络统计: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		iface := strings.TrimSpace(parts[0])

		// 跳过 lo 接口
		if iface == "lo" {
			continue
		}

		fields := strings.Fields(strings.TrimSpace(parts[1]))
		if len(fields) < 16 {
			continue
		}

		rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
		rxPackets, _ := strconv.ParseUint(fields[1], 10, 64)
		rxErrors, _ := strconv.ParseUint(fields[2], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[8], 10, 64)
		txPackets, _ := strconv.ParseUint(fields[9], 10, 64)
		txErrors, _ := strconv.ParseUint(fields[10], 10, 64)

		stats = append(stats, &NetworkStats{
			Interface: iface,
			RXBytes:   rxBytes,
			TXBytes:   txBytes,
			RXPackets: rxPackets,
			TXPackets: txPackets,
			RXErrors:  rxErrors,
			TXErrors:  txErrors,
		})
	}

	return stats, nil
}

// GetSMARTInfo 获取磁盘 SMART 信息.
func (m *Manager) GetSMARTInfo(device string) (*SMARTInfo, error) {
	info := &SMARTInfo{
		Device: device,
	}

	// 检查 smartctl 是否可用
	if _, err := exec.LookPath("smartctl"); err != nil {
		return nil, fmt.Errorf("smartctl 未安装")
	}

	// 获取 SMART 信息
	cmd := exec.CommandContext(context.Background(), "smartctl", "-A", "-i", "-H", device)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法获取 SMART 信息: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()

		// 解析温度
		if strings.Contains(line, "Temperature_Celsius") || strings.Contains(line, "Temperature:") {
			fields := strings.Fields(line)
			for i, f := range fields {
				if f == "Temperature_Celsius" || f == "Temperature:" {
					if i+1 < len(fields) {
						temp, _ := strconv.Atoi(fields[i+1])
						info.Temperature = temp
					}
				}
			}
		}

		// 解析健康状态
		if strings.Contains(line, "SMART overall-health self-assessment test result:") {
			if strings.Contains(line, "PASSED") {
				info.Health = "PASSED"
			} else {
				info.Health = "FAILED"
			}
		}

		// 解析型号
		if strings.HasPrefix(line, "Device Model:") {
			info.Model = strings.TrimSpace(strings.TrimPrefix(line, "Device Model:"))
		}

		// 解析序列号
		if strings.HasPrefix(line, "Serial Number:") {
			info.Serial = strings.TrimSpace(strings.TrimPrefix(line, "Serial Number:"))
		}

		// 解析通电时间
		if strings.Contains(line, "Power_On_Hours") {
			fields := strings.Fields(line)
			if len(fields) >= 10 {
				hours, _ := strconv.ParseUint(fields[9], 10, 64)
				info.PowerOnHours = hours
			}
		}
	}

	return info, nil
}

// CheckDisks 检查所有磁盘.
func (m *Manager) CheckDisks() ([]*SMARTInfo, error) {
	var results []*SMARTInfo

	// 列出所有块设备
	cmd := exec.CommandContext(context.Background(), "lsblk", "-d", "-n", "-o", "NAME")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法列出磁盘: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		device := "/dev/" + strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(device, "/dev/sd") || strings.HasPrefix(device, "/dev/nvme") {
			info, err := m.GetSMARTInfo(device)
			if err == nil {
				results = append(results, info)
			}
		}
	}

	return results, nil
}

// getCPUUsage 获取 CPU 使用率.
func (m *Manager) getCPUUsage() (float64, error) {
	// 读取 /proc/stat
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) == 0 {
		return 0, fmt.Errorf("无法读取 CPU 统计")
	}

	fields := strings.Fields(lines[0])
	if len(fields) < 5 {
		return 0, fmt.Errorf("无效的 CPU 统计格式")
	}

	// 计算 CPU 使用率
	idle, _ := strconv.ParseFloat(fields[4], 64)
	total := 0.0
	for i := 1; i < len(fields) && i <= 7; i++ {
		val, _ := strconv.ParseFloat(fields[i], 64)
		total += val
	}

	if total == 0 {
		return 0, nil
	}

	usage := (total - idle) / total * 100
	return usage, nil
}

// getMemoryInfo 获取内存信息.
func (m *Manager) getMemoryInfo() (map[string]uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, err
	}

	result := make(map[string]uint64)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) >= 2 {
			key := strings.TrimSuffix(fields[0], ":")
			value, _ := strconv.ParseUint(fields[1], 10, 64)

			switch key {
			case "MemTotal":
				result["Total"] = value * 1024 // 转换为字节
			case "MemFree":
				result["Free"] = value * 1024
			}
		}
	}

	return result, nil
}

// getSwapInfo 获取 Swap 信息.
func (m *Manager) getSwapInfo() (map[string]uint64, error) {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return nil, err
	}

	result := make(map[string]uint64)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) >= 2 {
			key := strings.TrimSuffix(fields[0], ":")
			value, _ := strconv.ParseUint(fields[1], 10, 64)

			switch key {
			case "SwapTotal":
				result["Total"] = value * 1024
			case "SwapFree":
				result["Free"] = value * 1024
				result["Used"] = result["Total"] - result["Free"]
			}
		}
	}

	return result, nil
}

// getUptime 获取运行时间.
func (m *Manager) getUptime() (uint64, error) {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}

	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, fmt.Errorf("无效的 uptime 格式")
	}

	uptime, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, err
	}

	return uint64(uptime), nil
}

// formatUptime 格式化运行时间.
func (m *Manager) formatUptime(seconds uint64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	mins := (seconds % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%d天 %d小时 %d分钟", days, hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%d小时 %d分钟", hours, mins)
	}
	return fmt.Sprintf("%d分钟", mins)
}

// getLoadAverage 获取负载均衡.
func (m *Manager) getLoadAverage() ([]float64, error) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return nil, err
	}

	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return nil, fmt.Errorf("无效的负载格式")
	}

	loadAvg := make([]float64, 3)
	for i := 0; i < 3; i++ {
		loadAvg[i], _ = strconv.ParseFloat(fields[i], 64)
	}

	return loadAvg, nil
}

// GetHostname 获取主机名.
func (m *Manager) GetHostname() string {
	return m.hostname
}

// --- 告警增强系统集成 (Round228) ---

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"nas-os/internal/monitor/alerting"
)

// AlertManager 增强版告警管理器（兼容原接口）
type AlertManager struct {
	// 原有的 AlertingManager
	legacy *AlertingManager
	// 新的增强管理器
	enhanced *alerting.Manager
	hostname string
	hostIP   string
}

// NewAlertManager 创建增强告警管理器
func NewAlertManager() (*AlertManager, error) {
	hostname, _ := os.Hostname()
	hostIP := getLocalIP()

	// 初始化增强系统
	cfg := alerting.DefaultManagerConfig()
	cfg.EnableAggregation = true
	cfg.EnableAggregation = true
	cfg.AggregationWindow = 5 * time.Minute
	cfg.EnableRouting = true
	cfg.EnableTemplates = true

	enhanced := alerting.NewManager(cfg)
	enhanced.Start()

	// 设置默认渠道和规则
	setupDefaultRoutes(enhanced)

	am := &AlertManager{
		legacy:   NewAlertingManager(),
		enhanced: enhanced,
		hostname: hostname,
		hostIP:   hostIP,
	}

	// 设置增强系统发送回调
	enhanced.SetOnSend(func(channelID string, vars *alerting.AlertVars) error {
		fmt.Printf("[AlertManager] 发送告警到 %s: %s\n", channelID, vars.AlertName)
		return nil
	})

	return am, nil
}

// setupDefaultRoutes 设置默认路由规则
func setupDefaultRoutes(m *alerting.Manager) {
	router := m.GetRouter()

	// 添加渠道
	channels := []*alerting.Channel{
		{
			ID:       "email-default",
			Name:     "默认邮件",
			Type:     alerting.ChannelEmail,
			Target:   "admin@nas-os.local",
			Template: "email_html_default",
			Enabled:  true,
		},
		{
			ID:       "webhook-default",
			Name:     "默认Webhook",
			Type:     alerting.ChannelWebhook,
			Target:   "http://localhost:8080/webhooks/alerts",
			Template: "webhook_default",
			Enabled:  true,
		},
	}

	for _, ch := range channels {
		_ = router.AddChannel(ch)
	}

	// 添加路由规则
	rules := []*alerting.RouteRule{
		{
			ID:                "critical-all",
			Name:              "严重告警全部通知",
			Priority:          1,
			Enabled:           true,
			Levels:            []alerting.AlertLevel{alerting.AlertLevelCritical},
			Channels:          []string{"email-default", "webhook-default"},
			SuppressionWindow: 10 * time.Minute,
		},
		{
			ID:                "warning-storage",
			Name:              "存储警告通知",
			Priority:          2,
			Enabled:           true,
			Levels:            []alerting.AlertLevel{alerting.AlertLevelWarning},
			ServiceTypes:      []string{"storage", "disk"},
			Channels:          []string{"email-default"},
			SuppressionWindow: 30 * time.Minute,
		},
		{
			ID:                "info-brief",
			Name:              "信息告警简化通知",
			Priority:          3,
			Enabled:           true,
			Levels:            []alerting.AlertLevel{alerting.AlertLevelInfo},
			Channels:          []string{"webhook-default"},
			SuppressionWindow: 1 * time.Hour,
		},
	}

	for _, rule := range rules {
		_ = router.AddRule(rule)
	}
}

// SendAlert 发送告警（兼容原接口）
func (am *AlertManager) SendAlert(ctx context.Context, alertType, level, message, source string, extra map[string]interface{}) error {
	vars := &alerting.AlertVars{
		AlertID:    fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		AlertName:  alertType,
		HostName:   am.hostname,
		HostIP:     am.hostIP,
		Level:      alerting.AlertLevel(level),
		Message:    message,
		Source:    source,
		Timestamp: time.Now(),
		Tags:      make(map[string]string),
		Extra:     extra,
	}

	// 设置额外字段
	if extra != nil {
		if metric, ok := extra["metric"].(string); ok {
			vars.Metric = metric
		}
		if value, ok := extra["value"].(float64); ok {
			vars.Value = value
		}
		if threshold, ok := extra["threshold"].(float64); ok {
			vars.Threshold = threshold
		}
		if unit, ok := extra["unit"].(string); ok {
			vars.Unit = unit
		}
		if serviceType, ok := extra["serviceType"].(string); ok {
			vars.ServiceType = serviceType
		}
	}

	return am.enhanced.ProcessAlert(ctx, vars)
}

// SendCriticalAlert 发送严重告警
func (am *AlertManager) SendCriticalAlert(ctx context.Context, alertType, message, source string) error {
	return am.SendAlert(ctx, alertType, "critical", message, source, nil)
}

// SendWarningAlert 发送警告告警
func (am *AlertManager) SendWarningAlert(ctx context.Context, alertType, message, source string) error {
	return am.SendAlert(ctx, alertType, "warning", message, source, nil)
}

// SendInfoAlert 发送信息告警
func (am *AlertManager) SendInfoAlert(ctx context.Context, alertType, message, source string) error {
	return am.SendAlert(ctx, alertType, "info", message, source, nil)
}

// QuickNotify 快速通知（直接发送，不经过聚合）
func (am *AlertManager) QuickNotify(ctx context.Context, chType alerting.ChannelType, target, templateID string, level alerting.AlertLevel, alertName, message string) error {
	vars := &alerting.AlertVars{
		AlertID:   fmt.Sprintf("quick-%d", time.Now().UnixNano()),
		AlertName: alertName,
		HostName:  am.hostname,
		HostIP:    am.hostIP,
		Level:     level,
		Message:   message,
		Source:    "nas-os",
		Timestamp: time.Now(),
		Tags:      make(map[string]string),
	}

	return am.enhanced.QuickSend(ctx, chType, target, templateID, vars)
}

// GetAlertingStatus 获取告警系统状态
func (am *AlertManager) GetAlertingStatus() map[string]interface{} {
	status := am.enhanced.GetStatus()
	status["hostname"] = am.hostname
	status["hostIP"] = am.hostIP
	status["legacy"] = map[string]interface{}{
		"totalAlerts":   len(am.legacy.GetAlerts(0, 0, nil)),
		"activeAlerts":  len(am.legacy.GetActiveAlerts()),
		"subscribers":   len(am.legacy.GetSubscribers()),
		"rules":         len(am.legacy.GetRules()),
	}
	return status
}

// Stop 停止告警管理器
func (am *AlertManager) Stop() {
	am.enhanced.Stop()
}

// getLocalIP 获取本机IP
func getLocalIP() string {
	// 简单实现，返回第一个非lo的IP
	cmd := exec.Command("hostname", "-I")
	output, err := cmd.Output()
	if err != nil {
		return "127.0.0.1"
	}
	ips := strings.Fields(string(output))
	if len(ips) > 0 {
		return ips[0]
	}
	return "127.0.0.1"
}
