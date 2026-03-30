// Package widgets 提供Dashboard Widget数据提供者
// providers.go - Widget数据获取Provider实现
package widgets

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// SystemLoadProvider 系统负载数据提供者
type SystemLoadProvider struct{}

// NewSystemLoadProvider 创建系统负载Provider
func NewSystemLoadProvider() *SystemLoadProvider {
	return &SystemLoadProvider{}
}

// GetData 获取系统负载数据
func (p *SystemLoadProvider) GetData() (*SystemLoadWidgetData, error) {
	data := &SystemLoadWidgetData{
		Timestamp: time.Now(),
	}

	// 读取负载
	if err := p.readLoadAvg(data); err != nil {
		return nil, err
	}

	// 读取进程数
	if err := p.readProcessCount(data); err != nil {
		// 非关键错误，继续
		data.ProcessCount = 0
	}

	// 计算状态
	data.Status = p.calculateStatus(data)

	return data, nil
}

// readLoadAvg 读取系统负载
func (p *SystemLoadProvider) readLoadAvg(data *SystemLoadWidgetData) error {
	loadavg, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return fmt.Errorf("读取负载失败: %w", err)
	}

	fields := strings.Fields(string(loadavg))
	if len(fields) < 3 {
		return fmt.Errorf("负载格式无效")
	}

	data.LoadAvg1, _ = strconv.ParseFloat(fields[0], 64)
	data.LoadAvg5, _ = strconv.ParseFloat(fields[1], 64)
	data.LoadAvg15, _ = strconv.ParseFloat(fields[2], 64)

	// 获取CPU核心数
	cpuCountFile := "/proc/cpuinfo"
	if content, err := os.ReadFile(cpuCountFile); err == nil {
		data.CPUCount = strings.Count(string(content), "processor")
	}

	return nil
}

// readProcessCount 读取进程数
func (p *SystemLoadProvider) readProcessCount(data *SystemLoadWidgetData) error {
	stat, err := os.ReadFile("/proc/stat")
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(stat)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "procs_") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				key := fields[0]
				value, _ := strconv.Atoi(fields[1])
				switch key {
				case "procs_running":
					data.RunningCount = value
				case "procs_blocked":
					data.BlockedCount = value
				}
			}
		}
	}

	// 总进程数从processes计数获取
	data.ProcessCount = data.RunningCount + data.BlockedCount

	return nil
}

// calculateStatus 计算负载状态
func (p *SystemLoadProvider) calculateStatus(data *SystemLoadWidgetData) string {
	if data.CPUCount == 0 {
		data.CPUCount = 1 // 默认假设单核
	}

	loadPerCore := data.LoadAvg1 / float64(data.CPUCount)

	if loadPerCore >= 1.0 {
		return "critical"
	} else if loadPerCore >= 0.7 {
		return "warning"
	}
	return "normal"
}

// StorageIOProvider 存储IO数据提供者
type StorageIOProvider struct{}

// NewStorageIOProvider 创建存储IO Provider
func NewStorageIOProvider() *StorageIOProvider {
	return &StorageIOProvider{}
}

// GetData 获取存储IO数据
func (p *StorageIOProvider) GetData() (*StorageIOWidgetData, error) {
	data := &StorageIOWidgetData{
		Timestamp: time.Now(),
		Devices:   make([]StorageIODevice, 0),
	}

	// 读取磁盘统计
	diskstats, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return nil, fmt.Errorf("读取磁盘统计失败: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(diskstats)))
	var totalRead, totalWrite uint64

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) < 14 {
			continue
		}

		deviceName := fields[2]
		// 过滤虚拟设备
		if strings.HasPrefix(deviceName, "loop") ||
			strings.HasPrefix(deviceName, "ram") ||
			strings.HasPrefix(deviceName, "sr") {
			continue
		}

		// sectors to bytes (sector = 512 bytes)
		readSectors, _ := strconv.ParseUint(fields[5], 10, 64)
		writeSectors, _ := strconv.ParseUint(fields[9], 10, 64)

		readBytes := readSectors * 512
		writeBytes := writeSectors * 512

		device := StorageIODevice{
			Device:     deviceName,
			ReadBytes:  readBytes,
			WriteBytes: writeBytes,
		}

		data.Devices = append(data.Devices, device)
		totalRead += readBytes
		totalWrite += writeBytes
	}

	data.TotalRead = totalRead
	data.TotalWrite = totalWrite

	// 计算速率（需要两次采样差值，这里返回累计值）
	data.ReadRate = float64(totalRead) / 1024 / 1024   // MB
	data.WriteRate = float64(totalWrite) / 1024 / 1024 // MB

	return data, nil
}

// NetworkTrafficProvider 网络流量数据提供者
type NetworkTrafficProvider struct{}

// NewNetworkTrafficProvider 创建网络流量Provider
func NewNetworkTrafficProvider() *NetworkTrafficProvider {
	return &NetworkTrafficProvider{}
}

// GetData 获取网络流量数据
func (p *NetworkTrafficProvider) GetData() (*NetworkTrafficWidgetData, error) {
	data := &NetworkTrafficWidgetData{
		Timestamp:  time.Now(),
		Interfaces: make([]NetworkTrafficInterface, 0),
	}

	netDev, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil, fmt.Errorf("读取网络统计失败: %w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(netDev)))
	var totalRX, totalTX uint64

	for scanner.Scan() {
		line := scanner.Text()
		// 跳过标题行
		if strings.Contains(line, "|") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 17 {
			continue
		}

		iface := strings.TrimSuffix(fields[0], ":")
		// 跳过回环
		if iface == "lo" {
			continue
		}

		rxBytes, _ := strconv.ParseUint(fields[1], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[9], 10, 64)

		// 获取接口速度
		speed := p.getInterfaceSpeed(iface)

		interfaceData := NetworkTrafficInterface{
			Name:    iface,
			RXBytes: rxBytes,
			TXBytes: txBytes,
			Speed:   speed,
		}

		data.Interfaces = append(data.Interfaces, interfaceData)
		totalRX += rxBytes
		totalTX += txBytes
	}

	data.TotalRX = totalRX
	data.TotalTX = totalTX
	data.RXRate = float64(totalRX) / 1024 / 1024 // MB
	data.TXRate = float64(totalTX) / 1024 / 1024 // MB

	return data, nil
}

// getInterfaceSpeed 获取接口速度
func (p *NetworkTrafficProvider) getInterfaceSpeed(iface string) uint64 {
	speedPath := fmt.Sprintf("/sys/class/net/%s/speed", iface)
	data, err := os.ReadFile(speedPath)
	if err != nil {
		return 0
	}

	speed, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}

	return speed
}

// AlertSummaryProvider 告警汇总数据提供者
type AlertSummaryProvider struct {
	alerts []AlertEntry
}

// NewAlertSummaryProvider 创建告警汇总Provider
func NewAlertSummaryProvider() *AlertSummaryProvider {
	return &AlertSummaryProvider{
		alerts: make([]AlertEntry, 0),
	}
}

// GetData 获取告警汇总数据
func (p *AlertSummaryProvider) GetData() (*AlertSummaryWidgetData, error) {
	data := &AlertSummaryWidgetData{
		Timestamp:    time.Now(),
		ActiveAlerts: make([]AlertEntry, 0),
		RecentAlerts: make([]AlertEntry, 0),
	}

	// 统计各级别告警
	for _, alert := range p.alerts {
		if !alert.Resolved {
			data.ActiveAlerts = append(data.ActiveAlerts, alert)
			switch alert.Level {
			case "critical":
				data.CriticalCount++
			case "warning":
				data.WarningCount++
			case "info":
				data.InfoCount++
			}
		}
	}

	data.TotalAlerts = len(data.ActiveAlerts)

	// 计算状态
	if data.CriticalCount > 0 {
		data.Status = "critical"
	} else if data.WarningCount > 0 {
		data.Status = "warning"
	} else {
		data.Status = "ok"
	}

	// 最近告警
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	for _, alert := range p.alerts {
		if alert.Timestamp.After(dayAgo) {
			data.RecentAlerts = append(data.RecentAlerts, alert)
		}
	}

	// 最后告警时间
	if len(data.ActiveAlerts) > 0 {
		data.LastAlertTime = data.ActiveAlerts[0].Timestamp
	}

	return data, nil
}

// AddAlert 添加告警
func (p *AlertSummaryProvider) AddAlert(alert AlertEntry) {
	p.alerts = append(p.alerts, alert)
}

// ClearAlerts 清除告警
func (p *AlertSummaryProvider) ClearAlerts() {
	p.alerts = make([]AlertEntry, 0)
}

// ResolveAlert 解决告警
func (p *AlertSummaryProvider) ResolveAlert(id string) {
	for i, alert := range p.alerts {
		if alert.ID == id {
			p.alerts[i].Resolved = true
			p.alerts[i].ResolvedAt = time.Now()
			break
		}
	}
}
