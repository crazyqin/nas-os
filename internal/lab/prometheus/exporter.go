// Package prometheus 提供 NAS-OS Prometheus 指标导出器
// 实现 Collector 接口，采集系统级监控指标供 Prometheus 拉取。
package prometheus

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	psnet "github.com/shirou/gopsutil/v4/net"
)

// ---------------------------------------------------------------------------
// 数据结构
// ---------------------------------------------------------------------------

// DiskMetric 磁盘使用量指标.
type DiskMetric struct {
	MountPoint string
	Used       uint64
	Total      uint64
}

// NetworkMetric 网络流量指标.
type NetworkMetric struct {
	Interface string
	RXBytes   uint64
	TXBytes   uint64
}

// StoragePoolMetric 存储池状态指标.
type StoragePoolMetric struct {
	Pool   string
	Status int // 0=healthy, 1=degraded, 2=faulted
}

// DiskTempMetric 磁盘温度指标.
type DiskTempMetric struct {
	Device      string
	Temperature float64
}

// MetricsProvider 系统指标数据源接口.
// 测试时可注入 mock 实现。
type MetricsProvider interface {
	GetCPUUsage() (float64, error)
	GetMemoryUsage() (float64, error)
	GetDiskStats() ([]DiskMetric, error)
	GetNetworkStats() ([]NetworkMetric, error)
	GetStoragePoolStatus() ([]StoragePoolMetric, error)
	GetDiskTemperatures() ([]DiskTempMetric, error)
	GetActiveConnections() (float64, error)
	GetSMBSessionCount() (float64, error)
}

// ---------------------------------------------------------------------------
// 系统数据源实现
// ---------------------------------------------------------------------------

// SystemProvider 从操作系统采集真实指标.
type SystemProvider struct{}

// NewSystemProvider 创建系统数据源.
func NewSystemProvider() *SystemProvider {
	return &SystemProvider{}
}

// GetCPUUsage 获取 CPU 使用率（百分比）.
func (p *SystemProvider) GetCPUUsage() (float64, error) {
	percentages, err := cpu.Percent(0, false)
	if err != nil {
		return 0, fmt.Errorf("获取 CPU 使用率失败：%w", err)
	}
	if len(percentages) == 0 {
		return 0, nil
	}
	return percentages[0], nil
}

// GetMemoryUsage 获取内存使用率（百分比）.
func (p *SystemProvider) GetMemoryUsage() (float64, error) {
	vm, err := mem.VirtualMemory()
	if err != nil {
		return 0, fmt.Errorf("获取内存信息失败：%w", err)
	}
	return vm.UsedPercent, nil
}

// GetDiskStats 获取各挂载点的磁盘使用量.
func (p *SystemProvider) GetDiskStats() ([]DiskMetric, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, fmt.Errorf("获取磁盘分区失败：%w", err)
	}

	var metrics []DiskMetric
	for _, part := range partitions {
		// 跳过虚拟文件系统
		if part.Fstype == "tmpfs" || part.Fstype == "overlay" || part.Fstype == "devtmpfs" ||
			part.Fstype == "sysfs" || part.Fstype == "proc" || part.Fstype == "devpts" {
			continue
		}

		usage, err := disk.Usage(part.Mountpoint)
		if err != nil {
			continue
		}

		metrics = append(metrics, DiskMetric{
			MountPoint: part.Mountpoint,
			Used:       usage.Used,
			Total:      usage.Total,
		})
	}

	return metrics, nil
}

// GetNetworkStats 获取各网卡的收发字节数.
func (p *SystemProvider) GetNetworkStats() ([]NetworkMetric, error) {
	counters, err := psnet.IOCounters(true)
	if err != nil {
		return nil, fmt.Errorf("获取网络统计失败：%w", err)
	}

	var metrics []NetworkMetric
	for _, c := range counters {
		if c.Name == "lo" {
			continue
		}
		metrics = append(metrics, NetworkMetric{
			Interface: c.Name,
			RXBytes:   c.BytesRecv,
			TXBytes:   c.BytesSent,
		})
	}
	return metrics, nil
}

// GetStoragePoolStatus 获取存储池状态.
// 尝试通过 zpool 命令获取；若不可用则返回空切片。
func (p *SystemProvider) GetStoragePoolStatus() ([]StoragePoolMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()

	//nolint:gosec // zpool 为系统命令，无注入风险
	cmd := exec.CommandContext(ctx, "zpool", "list", "-H", "-o", "name,health")
	output, err := cmd.Output()
	if err != nil {
		// zpool 未安装或无存储池，返回空
		return nil, nil //nolint:nilerr
	}

	var pools []StoragePoolMetric
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		status := parsePoolHealth(fields[1])
		pools = append(pools, StoragePoolMetric{
			Pool:   fields[0],
			Status: status,
		})
	}
	return pools, nil
}

// parsePoolHealth 将 zpool 健康状态映射为数值.
func parsePoolHealth(health string) int {
	switch strings.ToUpper(health) {
	case "ONLINE":
		return 0 // healthy
	case "DEGRADED":
		return 1
	case "FAULTED", "UNAVAIL", "REMOVED":
		return 2
	default:
		return 2
	}
}

// GetDiskTemperatures 获取磁盘温度.
// 尝试通过 smartctl 获取；若不可用则返回空切片。
func (p *SystemProvider) GetDiskTemperatures() ([]DiskTempMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10)
	defer cancel()

	// 列出所有块设备
	lsblk, err := exec.CommandContext(ctx, "lsblk", "-d", "-n", "-o", "NAME").Output()
	if err != nil {
		return nil, nil //nolint:nilerr
	}

	var temps []DiskTempMetric
	scanner := bufio.NewScanner(strings.NewReader(string(lsblk)))
	for scanner.Scan() {
		device := "/dev/" + strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(device, "/dev/sd") && !strings.HasPrefix(device, "/dev/nvme") {
			continue
		}

		temp, ok := queryDiskTemp(ctx, device)
		if ok {
			temps = append(temps, DiskTempMetric{
				Device:      device,
				Temperature: temp,
			})
		}
	}
	return temps, nil
}

// queryDiskTemp 通过 smartctl 查询单块磁盘温度.
func queryDiskTemp(ctx context.Context, device string) (float64, bool) {
	cmd := exec.CommandContext(ctx, "smartctl", "-A", device) //nolint:gosec
	output, err := cmd.Output()
	if err != nil {
		return 0, false
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "Temperature_Celsius") || strings.Contains(line, "Temperature_Internal") {
			fields := strings.Fields(line)
			for _, f := range fields {
				if f == "Temperature_Celsius" || f == "Temperature_Internal" {
					// SMART 属性值通常在第 9 列（index 9），也可能在 VALUE 列（index 3）
					for _, idx := range []int{9, 3, 10} {
						if idx < len(fields) {
							if v, err := strconv.ParseFloat(fields[idx], 64); err == nil && v > 0 && v < 200 {
								return v, true
							}
						}
					}
				}
			}
		}
	}
	return 0, false
}

// GetActiveConnections 获取活跃 TCP 连接数.
func (p *SystemProvider) GetActiveConnections() (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()

	// 使用 ss 获取已建立连接数
	cmd := exec.CommandContext(ctx, "ss", "-t", "-a", "state", "established")
	output, err := cmd.Output()
	if err != nil {
		// 回退：读 /proc/net/tcp
		return countProcTCP()
	}

	count := 0
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ESTAB") || strings.Contains(line, "ESTAB") {
			count++
		}
	}
	return float64(count), nil
}

// countProcTCP 通过 /proc/net/tcp 统计已建立连接数.
func countProcTCP() (float64, error) {
	data, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		return 0, fmt.Errorf("读取 /proc/net/tcp 失败：%w", err)
	}

	count := 0
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, " 01 ") { // state=ESTABLISHED
			count++
		}
	}
	return float64(count), nil
}

// GetSMBSessionCount 获取 SMB 会话数.
func (p *SystemProvider) GetSMBSessionCount() (float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5)
	defer cancel()

	cmd := exec.CommandContext(ctx, "smbstatus", "--brief")
	output, err := cmd.Output()
	if err != nil {
		// smb 未运行或未安装
		return 0, nil //nolint:nilerr
	}

	count := 0
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	inSessionSection := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "---") && !inSessionSection {
			inSessionSection = true
			continue
		}
		if inSessionSection && line != "" && !strings.HasPrefix(line, "Pid") {
			count++
		}
	}
	return float64(count), nil
}

// ---------------------------------------------------------------------------
// Prometheus Exporter（Collector 实现）
// ---------------------------------------------------------------------------

// Exporter Prometheus 指标导出器.
type Exporter struct {
	provider MetricsProvider
	mu       sync.RWMutex

	// 指标描述符
	cpuUsage          *promclient.Desc
	memoryUsage       *promclient.Desc
	diskUsageBytes    *promclient.Desc
	diskTotalBytes    *promclient.Desc
	networkRXBytes    *promclient.Desc
	networkTXBytes    *promclient.Desc
	storagePoolStatus *promclient.Desc
	diskTemperature   *promclient.Desc
	activeConnections *promclient.Desc
	smbSessions       *promclient.Desc
}

// NewExporter 创建指标导出器.
func NewExporter(provider MetricsProvider) *Exporter {
	return &Exporter{
		provider: provider,

		cpuUsage: promclient.NewDesc(
			"nas_os_cpu_usage",
			"CPU 使用率（百分比）",
			nil, nil,
		),
		memoryUsage: promclient.NewDesc(
			"nas_os_memory_usage",
			"内存使用率（百分比）",
			nil, nil,
		),
		diskUsageBytes: promclient.NewDesc(
			"nas_os_disk_usage_bytes",
			"磁盘已使用空间（字节）",
			[]string{"mountpoint"}, nil,
		),
		diskTotalBytes: promclient.NewDesc(
			"nas_os_disk_total_bytes",
			"磁盘总容量（字节）",
			[]string{"mountpoint"}, nil,
		),
		networkRXBytes: promclient.NewDesc(
			"nas_os_network_rx_bytes",
			"网络接收字节数",
			[]string{"interface"}, nil,
		),
		networkTXBytes: promclient.NewDesc(
			"nas_os_network_tx_bytes",
			"网络发送字节数",
			[]string{"interface"}, nil,
		),
		storagePoolStatus: promclient.NewDesc(
			"nas_os_storage_pool_status",
			"存储池状态（0=healthy, 1=degraded, 2=faulted）",
			[]string{"pool"}, nil,
		),
		diskTemperature: promclient.NewDesc(
			"nas_os_disk_temperature",
			"磁盘温度（摄氏度）",
			[]string{"device"}, nil,
		),
		activeConnections: promclient.NewDesc(
			"nas_os_active_connections",
			"活跃 TCP 连接数",
			nil, nil,
		),
		smbSessions: promclient.NewDesc(
			"nas_os_smb_sessions",
			"SMB 会话数",
			nil, nil,
		),
	}
}

// Describe 实现 prometheus.Collector 接口.
func (e *Exporter) Describe(ch chan<- *promclient.Desc) {
	ch <- e.cpuUsage
	ch <- e.memoryUsage
	ch <- e.diskUsageBytes
	ch <- e.diskTotalBytes
	ch <- e.networkRXBytes
	ch <- e.networkTXBytes
	ch <- e.storagePoolStatus
	ch <- e.diskTemperature
	ch <- e.activeConnections
	ch <- e.smbSessions
}

// Collect 实现 prometheus.Collector 接口，每次 scrape 时采集最新数据.
func (e *Exporter) Collect(ch chan<- promclient.Metric) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// CPU
	if val, err := e.provider.GetCPUUsage(); err == nil {
		ch <- promclient.MustNewConstMetric(e.cpuUsage, promclient.GaugeValue, val)
	}

	// Memory
	if val, err := e.provider.GetMemoryUsage(); err == nil {
		ch <- promclient.MustNewConstMetric(e.memoryUsage, promclient.GaugeValue, val)
	}

	// Disks
	if disks, err := e.provider.GetDiskStats(); err == nil {
		for _, d := range disks {
			ch <- promclient.MustNewConstMetric(e.diskUsageBytes, promclient.GaugeValue, float64(d.Used), d.MountPoint)
			ch <- promclient.MustNewConstMetric(e.diskTotalBytes, promclient.GaugeValue, float64(d.Total), d.MountPoint)
		}
	}

	// Network
	if nets, err := e.provider.GetNetworkStats(); err == nil {
		for _, n := range nets {
			ch <- promclient.MustNewConstMetric(e.networkRXBytes, promclient.GaugeValue, float64(n.RXBytes), n.Interface)
			ch <- promclient.MustNewConstMetric(e.networkTXBytes, promclient.GaugeValue, float64(n.TXBytes), n.Interface)
		}
	}

	// Storage pools
	if pools, err := e.provider.GetStoragePoolStatus(); err == nil {
		for _, p := range pools {
			ch <- promclient.MustNewConstMetric(e.storagePoolStatus, promclient.GaugeValue, float64(p.Status), p.Pool)
		}
	}

	// Disk temperatures
	if temps, err := e.provider.GetDiskTemperatures(); err == nil {
		for _, t := range temps {
			ch <- promclient.MustNewConstMetric(e.diskTemperature, promclient.GaugeValue, t.Temperature, t.Device)
		}
	}

	// Active connections
	if val, err := e.provider.GetActiveConnections(); err == nil {
		ch <- promclient.MustNewConstMetric(e.activeConnections, promclient.GaugeValue, val)
	}

	// SMB sessions
	if val, err := e.provider.GetSMBSessionCount(); err == nil {
		ch <- promclient.MustNewConstMetric(e.smbSessions, promclient.GaugeValue, val)
	}
}
