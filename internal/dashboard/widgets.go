package dashboard

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"nas-os/internal/monitor"
)

// WidgetProvider 小组件数据提供者接口.
type WidgetProvider interface {
	GetData(widget *Widget) (*WidgetData, error)
	GetType() WidgetType
}

// CPUWidgetProvider CPU 小组件提供者.
type CPUWidgetProvider struct {
	monitorManager *monitor.Manager
}

// NewCPUWidgetProvider 创建 CPU 小组件提供者.
func NewCPUWidgetProvider(mgr *monitor.Manager) *CPUWidgetProvider {
	return &CPUWidgetProvider{monitorManager: mgr}
}

// GetType 获取类型.
func (p *CPUWidgetProvider) GetType() WidgetType {
	return WidgetTypeCPU
}

// GetData 获取数据.
func (p *CPUWidgetProvider) GetData(widget *Widget) (*WidgetData, error) {
	data := &CPUWidgetData{
		Timestamp: time.Now(),
	}

	stats, err := p.monitorManager.GetSystemStats()
	if err != nil {
		return nil, fmt.Errorf("获取系统统计失败: %w", err)
	}

	data.Usage = stats.CPUUsage
	if len(stats.LoadAvg) >= 3 {
		data.LoadAvg1 = stats.LoadAvg[0]
		data.LoadAvg5 = stats.LoadAvg[1]
		data.LoadAvg15 = stats.LoadAvg[2]
	}
	data.ProcessCount = stats.Processes

	// 获取每核心使用率
	if widget.Config.ShowPerCore {
		perCore, err := p.getPerCoreUsage()
		if err == nil {
			data.PerCore = perCore
		}
	}

	return &WidgetData{
		WidgetID:  widget.ID,
		Type:      WidgetTypeCPU,
		Timestamp: time.Now(),
		Data:      data,
	}, nil
}

// getPerCoreUsage 获取每核心 CPU 使用率.
func (p *CPUWidgetProvider) getPerCoreUsage() ([]float64, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return nil, err
	}

	var cores []float64
	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		if !strings.HasPrefix(line, "cpu") || line == "cpu " {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		user, _ := strconv.ParseFloat(fields[1], 64)
		nice, _ := strconv.ParseFloat(fields[2], 64)
		system, _ := strconv.ParseFloat(fields[3], 64)
		idle, _ := strconv.ParseFloat(fields[4], 64)

		total := user + nice + system + idle
		if total > 0 {
			usage := (total - idle) / total * 100
			cores = append(cores, usage)
		}
	}

	return cores, nil
}

// MemoryWidgetProvider 内存小组件提供者.
type MemoryWidgetProvider struct {
	monitorManager *monitor.Manager
}

// NewMemoryWidgetProvider 创建内存小组件提供者.
func NewMemoryWidgetProvider(mgr *monitor.Manager) *MemoryWidgetProvider {
	return &MemoryWidgetProvider{monitorManager: mgr}
}

// GetType 获取类型.
func (p *MemoryWidgetProvider) GetType() WidgetType {
	return WidgetTypeMemory
}

// GetData 获取数据.
func (p *MemoryWidgetProvider) GetData(widget *Widget) (*WidgetData, error) {
	data := &MemoryWidgetData{
		Timestamp: time.Now(),
	}

	stats, err := p.monitorManager.GetSystemStats()
	if err != nil {
		return nil, fmt.Errorf("获取系统统计失败: %w", err)
	}

	data.Total = stats.MemoryTotal
	data.Used = stats.MemoryUsed
	data.Free = stats.MemoryFree
	data.Usage = stats.MemoryUsage

	if widget.Config.ShowSwap {
		data.SwapTotal = stats.SwapTotal
		data.SwapUsed = stats.SwapUsed
		data.SwapFree = stats.SwapTotal - stats.SwapUsed
		data.SwapUsage = stats.SwapUsage
	}

	if widget.Config.ShowBuffers {
		p.getBufferStats(data)
	}

	return &WidgetData{
		WidgetID:  widget.ID,
		Type:      WidgetTypeMemory,
		Timestamp: time.Now(),
		Data:      data,
	}, nil
}

// getBufferStats 获取缓冲区统计.
func (p *MemoryWidgetProvider) getBufferStats(data *MemoryWidgetData) {
	memInfo, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(memInfo)))
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)

		if len(fields) >= 2 {
			key := strings.TrimSuffix(fields[0], ":")
			value, _ := strconv.ParseUint(fields[1], 10, 64)
			value *= 1024 // KB to bytes

			switch key {
			case "Buffers":
				data.Buffers = value
			case "Cached":
				data.Cached = value
			}
		}
	}
}

// DiskWidgetProvider 磁盘小组件提供者.
type DiskWidgetProvider struct {
	monitorManager *monitor.Manager
}

// NewDiskWidgetProvider 创建磁盘小组件提供者.
func NewDiskWidgetProvider(mgr *monitor.Manager) *DiskWidgetProvider {
	return &DiskWidgetProvider{monitorManager: mgr}
}

// GetType 获取类型.
func (p *DiskWidgetProvider) GetType() WidgetType {
	return WidgetTypeDisk
}

// GetData 获取数据.
func (p *DiskWidgetProvider) GetData(widget *Widget) (*WidgetData, error) {
	data := &DiskWidgetData{
		Timestamp: time.Now(),
		Devices:   make([]DiskDeviceData, 0),
	}

	diskStats, err := p.monitorManager.GetDiskStats()
	if err != nil {
		return nil, fmt.Errorf("获取磁盘统计失败: %w", err)
	}

	// 过滤挂载点
	mountFilter := make(map[string]bool)
	if len(widget.Config.MountPoints) > 0 {
		for _, mp := range widget.Config.MountPoints {
			mountFilter[mp] = true
		}
	}

	var totalSize, totalUsed, totalFree uint64

	for _, d := range diskStats {
		// 跳过特殊文件系统
		if d.FSType == "tmpfs" || d.FSType == "devtmpfs" || d.FSType == "squashfs" {
			continue
		}

		// 应用挂载点过滤
		if len(mountFilter) > 0 && !mountFilter[d.MountPoint] {
			continue
		}

		device := DiskDeviceData{
			Device:       d.Device,
			MountPoint:   d.MountPoint,
			Total:        d.Total,
			Used:         d.Used,
			Free:         d.Free,
			UsagePercent: d.UsagePercent,
			FSType:       d.FSType,
		}

		// 获取 IO 统计
		if widget.Config.ShowIOStats {
			p.getDiskIOStats(d.Device, &device)
		}

		data.Devices = append(data.Devices, device)
		totalSize += d.Total
		totalUsed += d.Used
		totalFree += d.Free
	}

	data.Total = DiskSummaryData{
		Total:        totalSize,
		Used:         totalUsed,
		Free:         totalFree,
		UsagePercent: calculateUsagePercent(totalUsed, totalSize),
	}

	return &WidgetData{
		WidgetID:  widget.ID,
		Type:      WidgetTypeDisk,
		Timestamp: time.Now(),
		Data:      data,
	}, nil
}

// getDiskIOStats 获取磁盘 IO 统计.
func (p *DiskWidgetProvider) getDiskIOStats(device string, data *DiskDeviceData) {
	// 提取设备名
	devName := strings.TrimPrefix(device, "/dev/")
	if strings.HasPrefix(devName, "sd") || strings.HasPrefix(devName, "hd") {
		devName = devName[:3] // sda, sdb, etc.
	} else if strings.HasPrefix(devName, "nvme") {
		devName = devName[:5] // nvme0, nvme1, etc.
	}

	diskstats, err := os.ReadFile("/proc/diskstats")
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(strings.NewReader(string(diskstats)))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, devName) {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 10 {
			// sectors to bytes (sector = 512 bytes)
			readSectors, _ := strconv.ParseUint(fields[5], 10, 64)
			writeSectors, _ := strconv.ParseUint(fields[9], 10, 64)
			data.ReadBytes = readSectors * 512
			data.WriteBytes = writeSectors * 512
		}
		break
	}
}

// NetworkWidgetProvider 网络小组件提供者.
type NetworkWidgetProvider struct {
	monitorManager *monitor.Manager
}

// NewNetworkWidgetProvider 创建网络小组件提供者.
func NewNetworkWidgetProvider(mgr *monitor.Manager) *NetworkWidgetProvider {
	return &NetworkWidgetProvider{monitorManager: mgr}
}

// GetType 获取类型.
func (p *NetworkWidgetProvider) GetType() WidgetType {
	return WidgetTypeNetwork
}

// GetData 获取数据.
func (p *NetworkWidgetProvider) GetData(widget *Widget) (*WidgetData, error) {
	data := &NetworkWidgetData{
		Timestamp:  time.Now(),
		Interfaces: make([]NetworkInterfaceData, 0),
	}

	netStats, err := p.monitorManager.GetNetworkStats()
	if err != nil {
		return nil, fmt.Errorf("获取网络统计失败: %w", err)
	}

	// 接口过滤
	ifaceFilter := make(map[string]bool)
	if len(widget.Config.Interfaces) > 0 {
		for _, iface := range widget.Config.Interfaces {
			ifaceFilter[iface] = true
		}
	}

	var totalRX, totalTX, totalRXPkts, totalTXPkts uint64

	for _, n := range netStats {
		// 跳过回环接口
		if n.Interface == "lo" {
			continue
		}

		// 应用接口过滤
		if len(ifaceFilter) > 0 && !ifaceFilter[n.Interface] {
			continue
		}

		iface := NetworkInterfaceData{
			Name:    n.Interface,
			RXBytes: n.RXBytes,
			TXBytes: n.TXBytes,
		}

		if widget.Config.ShowPackets {
			iface.RXPackets = n.RXPackets
			iface.TXPackets = n.TXPackets
		}

		if widget.Config.ShowErrors {
			iface.RXErrors = n.RXErrors
			iface.TXErrors = n.TXErrors
		}

		// 获取接口速度
		iface.Speed = p.getInterfaceSpeed(n.Interface)

		data.Interfaces = append(data.Interfaces, iface)
		totalRX += n.RXBytes
		totalTX += n.TXBytes
		totalRXPkts += n.RXPackets
		totalTXPkts += n.TXPackets
	}

	data.Total = NetworkSummaryData{
		RXBytes:   totalRX,
		TXBytes:   totalTX,
		RXPackets: totalRXPkts,
		TXPackets: totalTXPkts,
	}

	return &WidgetData{
		WidgetID:  widget.ID,
		Type:      WidgetTypeNetwork,
		Timestamp: time.Now(),
		Data:      data,
	}, nil
}

// getInterfaceSpeed 获取接口速度.
func (p *NetworkWidgetProvider) getInterfaceSpeed(iface string) uint64 {
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

// calculateUsagePercent 计算使用百分比.
func calculateUsagePercent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

// WidgetRegistry 小组件注册表.
type WidgetRegistry struct {
	providers map[WidgetType]WidgetProvider
}

// NewWidgetRegistry 创建小组件注册表.
func NewWidgetRegistry() *WidgetRegistry {
	return &WidgetRegistry{
		providers: make(map[WidgetType]WidgetProvider),
	}
}

// Register 注册小组件提供者.
func (r *WidgetRegistry) Register(provider WidgetProvider) {
	r.providers[provider.GetType()] = provider
}

// Get 获取小组件提供者.
func (r *WidgetRegistry) Get(widgetType WidgetType) (WidgetProvider, bool) {
	provider, ok := r.providers[widgetType]
	return provider, ok
}

// GetAvailableTypes 获取可用类型.
func (r *WidgetRegistry) GetAvailableTypes() []WidgetType {
	types := make([]WidgetType, 0, len(r.providers))
	for t := range r.providers {
		types = append(types, t)
	}
	return types
}

// DefaultWidgetRegistry 默认小组件注册表.
func DefaultWidgetRegistry(mgr *monitor.Manager) *WidgetRegistry {
	registry := NewWidgetRegistry()
	registry.Register(NewCPUWidgetProvider(mgr))
	registry.Register(NewMemoryWidgetProvider(mgr))
	registry.Register(NewDiskWidgetProvider(mgr))
	registry.Register(NewNetworkWidgetProvider(mgr))
	return registry
}

// CreateDefaultWidgets 创建默认小组件.
func CreateDefaultWidgets() []*Widget {
	now := time.Now()
	return []*Widget{
		{
			ID:          "cpu-default",
			Type:        WidgetTypeCPU,
			Title:       "CPU 监控",
			Size:        WidgetSizeMedium,
			Position:    WidgetPosition{X: 0, Y: 0},
			Enabled:     true,
			RefreshRate: 5 * time.Second,
			CreatedAt:   now,
			UpdatedAt:   now,
			Config: WidgetConfig{
				ShowPerCore:       true,
				WarningThreshold:  70,
				CriticalThreshold: 90,
			},
		},
		{
			ID:          "memory-default",
			Type:        WidgetTypeMemory,
			Title:       "内存监控",
			Size:        WidgetSizeMedium,
			Position:    WidgetPosition{X: 1, Y: 0},
			Enabled:     true,
			RefreshRate: 5 * time.Second,
			CreatedAt:   now,
			UpdatedAt:   now,
			Config: WidgetConfig{
				ShowSwap:          true,
				ShowBuffers:       true,
				WarningThreshold:  80,
				CriticalThreshold: 95,
			},
		},
		{
			ID:          "disk-default",
			Type:        WidgetTypeDisk,
			Title:       "磁盘监控",
			Size:        WidgetSizeLarge,
			Position:    WidgetPosition{X: 0, Y: 1},
			Enabled:     true,
			RefreshRate: 30 * time.Second,
			CreatedAt:   now,
			UpdatedAt:   now,
			Config: WidgetConfig{
				ShowIOStats: true,
			},
		},
		{
			ID:          "network-default",
			Type:        WidgetTypeNetwork,
			Title:       "网络监控",
			Size:        WidgetSizeMedium,
			Position:    WidgetPosition{X: 1, Y: 1},
			Enabled:     true,
			RefreshRate: 5 * time.Second,
			CreatedAt:   now,
			UpdatedAt:   now,
			Config: WidgetConfig{
				ShowPackets: true,
				ShowErrors:  true,
			},
		},
	}
}

// GetWidgetStatus 获取小组件状态.
func GetWidgetStatus(data *WidgetData, config WidgetConfig) string {
	switch d := data.Data.(type) {
	case *CPUWidgetData:
		if d.Usage >= config.CriticalThreshold {
			return "critical"
		} else if d.Usage >= config.WarningThreshold {
			return "warning"
		}
		return "healthy"
	case *MemoryWidgetData:
		if d.Usage >= config.CriticalThreshold {
			return "critical"
		} else if d.Usage >= config.WarningThreshold {
			return "warning"
		}
		return "healthy"
	case *DiskWidgetData:
		for _, dev := range d.Devices {
			if dev.UsagePercent >= 95 {
				return "critical"
			} else if dev.UsagePercent >= 80 {
				return "warning"
			}
		}
		return "healthy"
	default:
		return "unknown"
	}
}

// FormatBytes 格式化字节数.
func FormatBytes(bytes uint64) string {
	const unit = 1024
	sizes := []string{"B", "KB", "MB", "GB", "TB", "PB"}

	if bytes == 0 {
		return "0 B"
	}

	i := 0
	fb := float64(bytes)
	for fb >= unit && i < len(sizes)-1 {
		fb /= unit
		i++
	}

	return fmt.Sprintf("%.2f %s", fb, sizes[i])
}

// FormatRate 格式化速率.
func FormatRate(bytesPerSec uint64) string {
	return FormatBytes(bytesPerSec) + "/s"
}

// ExecuteCommand 执行系统命令.
func ExecuteCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
