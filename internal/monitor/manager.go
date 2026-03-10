package monitor

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Manager 监控管理器
type Manager struct {
	hostname string
}

// SystemStats 系统统计信息
type SystemStats struct {
	CPUUsage     float64   `json:"cpuUsage"`
	MemoryUsage  float64   `json:"memoryUsage"`
	MemoryTotal  uint64    `json:"memoryTotal"`
	MemoryUsed   uint64    `json:"memoryUsed"`
	MemoryFree   uint64    `json:"memoryFree"`
	SwapUsage    float64   `json:"swapUsage"`
	SwapTotal    uint64    `json:"swapTotal"`
	SwapUsed     uint64    `json:"swapUsed"`
	Uptime       string    `json:"uptime"`
	UptimeSeconds uint64   `json:"uptimeSeconds"`
	LoadAvg      []float64 `json:"loadAvg"`
	Processes    int       `json:"processes"`
	Timestamp    time.Time `json:"timestamp"`
}

// DiskStats 磁盘统计信息
type DiskStats struct {
	Device      string  `json:"device"`
	MountPoint  string  `json:"mountPoint"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsagePercent float64 `json:"usagePercent"`
	FSType      string  `json:"fsType"`
}

// NetworkStats 网络统计信息
type NetworkStats struct {
	Interface string `json:"interface"`
	RXBytes   uint64 `json:"rxBytes"`
	TXBytes   uint64 `json:"txBytes"`
	RXPackets uint64 `json:"rxPackets"`
	TXPackets uint64 `json:"txPackets"`
	RXErrors  uint64 `json:"rxErrors"`
	TXErrors  uint64 `json:"txErrors"`
}

// SMARTInfo SMART 信息
type SMARTInfo struct {
	Device     string `json:"device"`
	Model      string `json:"model"`
	Serial     string `json:"serial"`
	Temperature int   `json:"temperature"`
	Health     string `json:"health"`
	PowerOnHours uint64 `json:"powerOnHours"`
	ReadErrors  uint64 `json:"readErrors"`
	WriteErrors uint64 `json:"writeErrors"`
}

// Alert 告警信息
type Alert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // cpu, memory, disk, smart
	Level     string    `json:"level"` // warning, critical
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	Timestamp time.Time `json:"timestamp"`
	Acknowledged bool   `json:"acknowledged"`
}

// AlertRule 告警规则
type AlertRule struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Threshold float64 `json:"threshold"`
	Level     string  `json:"level"`
	Enabled   bool    `json:"enabled"`
}

// NewManager 创建监控管理器
func NewManager() (*Manager, error) {
	hostname, _ := os.Hostname()
	return &Manager{
		hostname: hostname,
	}, nil
}

// GetSystemStats 获取系统统计信息
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

// GetDiskStats 获取磁盘统计信息
func (m *Manager) GetDiskStats() ([]*DiskStats, error) {
	var stats []*DiskStats

	// 使用 df 命令获取磁盘信息
	cmd := exec.Command("df", "-B1", "--output=source,target,size,used,avail,fstype")
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

// GetNetworkStats 获取网络统计信息
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

// GetSMARTInfo 获取磁盘 SMART 信息
func (m *Manager) GetSMARTInfo(device string) (*SMARTInfo, error) {
	info := &SMARTInfo{
		Device: device,
	}

	// 检查 smartctl 是否可用
	if _, err := exec.LookPath("smartctl"); err != nil {
		return nil, fmt.Errorf("smartctl 未安装")
	}

	// 获取 SMART 信息
	cmd := exec.Command("smartctl", "-A", "-i", "-H", device)
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

// CheckDisks 检查所有磁盘
func (m *Manager) CheckDisks() ([]*SMARTInfo, error) {
	var results []*SMARTInfo

	// 列出所有块设备
	cmd := exec.Command("lsblk", "-d", "-n", "-o", "NAME")
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

// getCPUUsage 获取 CPU 使用率
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

// getMemoryInfo 获取内存信息
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

// getSwapInfo 获取 Swap 信息
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

// getUptime 获取运行时间
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

// formatUptime 格式化运行时间
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

// getLoadAverage 获取负载均衡
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

// GetHostname 获取主机名
func (m *Manager) GetHostname() string {
	return m.hostname
}