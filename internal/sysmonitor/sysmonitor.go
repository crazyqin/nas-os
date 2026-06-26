package sysmonitor

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
	"go.uber.org/zap"
)

// Config 系统监控配置
type Config struct {
	Enabled         bool    `json:"enabled"`
	Interval        int     `json:"interval"`
	CPUAlert        float64 `json:"cpu_alert"`
	MemAlert        float64 `json:"mem_alert"`
	DiskAlert       float64 `json:"disk_alert"`
	HistoryMaxSize  int     `json:"history_max_size"`
	TopProcessCount int     `json:"top_process_count"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		Interval:        30,
		CPUAlert:        90.0,
		MemAlert:        90.0,
		DiskAlert:       95.0,
		HistoryMaxSize:  120, // 30s interval * 120 = 1 hour
		TopProcessCount: 10,
	}
}

// Manager 系统监控管理器
type Manager struct {
	mu        sync.RWMutex
	config    *Config
	logger    *zap.Logger
	running   bool
	cancel    context.CancelFunc
	overview  *SystemOverview
	processes []ProcessInfo
	diskUsage []DiskUsageInfo
	network   NetworkInfo
	loadInfo  LoadInfo
	uptime    UptimeInfo
	alerts    []Alert
	history   []HistoryPoint
}

// SystemOverview 系统概览
type SystemOverview struct {
	Hostname    string  `json:"hostname"`
	OS          string  `json:"os"`
	Platform    string  `json:"platform"`
	Kernel      string  `json:"kernel"`
	Arch        string  `json:"arch"`
	CPUCores    int     `json:"cpu_cores"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemTotal    uint64  `json:"mem_total"`
	MemUsed     uint64  `json:"mem_used"`
	MemPercent  float64 `json:"mem_percent"`
	SwapTotal   uint64  `json:"swap_total"`
	SwapUsed    uint64  `json:"swap_used"`
	SwapPercent float64 `json:"swap_percent"`
	Uptime      uint64  `json:"uptime"`
	Load1       float64 `json:"load1"`
	Load5       float64 `json:"load5"`
	Load15      float64 `json:"load15"`
	Timestamp   int64   `json:"timestamp"`
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	User       string  `json:"user"`
	Status     string  `json:"status"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float32 `json:"mem_percent"`
	MemRSS     uint64  `json:"mem_rss"`
	MemVMS     uint64  `json:"mem_vms"`
	CreateTime int64   `json:"create_time"`
	Cmdline    string  `json:"cmdline"`
}

// DiskUsageInfo 磁盘使用信息
type DiskUsageInfo struct {
	MountPoint  string  `json:"mount_point"`
	Device      string  `json:"device"`
	FSType      string  `json:"fs_type"`
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
	InodesTotal uint64  `json:"inodes_total"`
	InodesUsed  uint64  `json:"inodes_used"`
	InodesFree  uint64  `json:"inodes_free"`
}

// NetworkInfo 网络连接信息
type NetworkInfo struct {
	Connections      []ConnectionInfo `json:"connections"`
	TCPCount         int              `json:"tcp_count"`
	UDPCount         int              `json:"udp_count"`
	ListenCount      int              `json:"listen_count"`
	EstablishedCount int              `json:"established_count"`
	BytesSent        uint64           `json:"bytes_sent"`
	BytesRecv        uint64           `json:"bytes_recv"`
	PacketsSent      uint64           `json:"packets_sent"`
	PacketsRecv      uint64           `json:"packets_recv"`
	Timestamp        int64            `json:"timestamp"`
}

// ConnectionInfo 连接信息
type ConnectionInfo struct {
	Family     uint32 `json:"family"`
	Type       uint32 `json:"type"`
	Status     string `json:"status"`
	LocalAddr  string `json:"local_addr"`
	LocalPort  uint32 `json:"local_port"`
	RemoteAddr string `json:"remote_addr"`
	RemotePort uint32 `json:"remote_port"`
	PID        int32  `json:"pid"`
}

// LoadInfo 系统负载信息
type LoadInfo struct {
	Load1     float64 `json:"load1"`
	Load5     float64 `json:"load5"`
	Load15    float64 `json:"load15"`
	CPUCores  int     `json:"cpu_cores"`
	Timestamp int64   `json:"timestamp"`
}

// UptimeInfo 运行时间信息
type UptimeInfo struct {
	Uptime      uint64 `json:"uptime"`
	BootTime    uint64 `json:"boot_time"`
	BootTimeStr string `json:"boot_time_str"`
	UptimeStr   string `json:"uptime_str"`
	NumUsers    int    `json:"num_users"`
	Timestamp   int64  `json:"timestamp"`
}

// UserInfo 用户会话信息
type UserInfo struct {
	User     string `json:"user"`
	Terminal string `json:"terminal"`
	Host     string `json:"host"`
	Started  int    `json:"started"`
}

// Alert 告警信息
type Alert struct {
	Type      string  `json:"type"`
	Level     string  `json:"level"`
	Message   string  `json:"message"`
	Value     float64 `json:"value"`
	Threshold float64 `json:"threshold"`
	Timestamp int64   `json:"timestamp"`
}

// HistoryPoint 历史数据点
type HistoryPoint struct {
	Timestamp   int64   `json:"timestamp"`
	CPUPercent  float64 `json:"cpu_percent"`
	MemPercent  float64 `json:"mem_percent"`
	DiskPercent float64 `json:"disk_percent"`
	Load1       float64 `json:"load1"`
	Load5       float64 `json:"load5"`
	Load15      float64 `json:"load15"`
}

// NewManager 创建监控管理器
func NewManager(cfg *Config, logger *zap.Logger) *Manager {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		config:  cfg,
		logger:  logger,
		history: make([]HistoryPoint, 0, cfg.HistoryMaxSize),
		alerts:  make([]Alert, 0),
	}
}

// Start 启动监控
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.running = true

	// 立即采集一次
	go m.collect(ctx)

	// 启动定时采集
	go m.run(ctx)

	m.logger.Info("系统监控已启动", zap.Int("interval", m.config.Interval))
	return nil
}

// Stop 停止监控
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil
	}

	m.cancel()
	m.running = false
	m.logger.Info("系统监控已停止")
	return nil
}

// IsRunning 运行状态
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// run 定时采集循环
func (m *Manager) run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(m.config.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.collect(ctx)
		}
	}
}

// collect 执行一次完整采集
func (m *Manager) collect(ctx context.Context) {
	var wg sync.WaitGroup

	wg.Add(7)

	go func() { defer wg.Done(); m.collectOverview(ctx) }()
	go func() { defer wg.Done(); m.collectProcesses(ctx) }()
	go func() { defer wg.Done(); m.collectDiskUsage(ctx) }()
	go func() { defer wg.Done(); m.collectNetwork(ctx) }()
	go func() { defer wg.Done(); m.collectLoad(ctx) }()
	go func() { defer wg.Done(); m.collectUptime(ctx) }()
	go func() { defer wg.Done(); m.checkAlerts(ctx) }()

	wg.Wait()

	// 采集完成后记录历史
	m.recordHistory()
}

// collectOverview 采集系统概览
func (m *Manager) collectOverview(ctx context.Context) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		m.logger.Error("获取主机信息失败", zap.Error(err))
		return
	}

	cpuPercent, err := cpu.PercentWithContext(ctx, time.Second, false)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		m.logger.Error("获取CPU使用率失败", zap.Error(err))
		return
	}

	vmem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		m.logger.Error("获取内存信息失败", zap.Error(err))
		return
	}

	swap, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		m.logger.Error("获取交换分区信息失败", zap.Error(err))
		return
	}

	loadAvg, err := load.AvgWithContext(ctx)
	if err != nil {
		m.logger.Error("获取负载信息失败", zap.Error(err))
		return
	}

	cpuCores, _ := cpu.CountsWithContext(ctx, true)

	overview := &SystemOverview{
		Hostname:    info.Hostname,
		OS:          info.OS,
		Platform:    info.Platform,
		Kernel:      info.KernelVersion,
		Arch:        info.KernelArch,
		CPUCores:    cpuCores,
		MemTotal:    vmem.Total,
		MemUsed:     vmem.Used,
		MemPercent:  vmem.UsedPercent,
		SwapTotal:   swap.Total,
		SwapUsed:    swap.Used,
		SwapPercent: swap.UsedPercent,
		Uptime:      info.Uptime,
		Load1:       loadAvg.Load1,
		Load5:       loadAvg.Load5,
		Load15:      loadAvg.Load15,
		Timestamp:   time.Now().Unix(),
	}

	if len(cpuPercent) > 0 {
		overview.CPUPercent = cpuPercent[0]
	}

	m.mu.Lock()
	m.overview = overview
	m.mu.Unlock()
}

// collectProcesses 采集进程信息
func (m *Manager) collectProcesses(ctx context.Context) {
	pids, err := process.PidsWithContext(ctx)
	if err != nil {
		m.logger.Error("获取进程列表失败", zap.Error(err))
		return
	}

	processes := make([]ProcessInfo, 0, len(pids))

	for _, pid := range pids {
		p, err := process.NewProcessWithContext(ctx, pid)
		if err != nil {
			continue
		}

		name, _ := p.NameWithContext(ctx)
		user, _ := p.UsernameWithContext(ctx)
		statusSlice, _ := p.StatusWithContext(ctx)
		cpuPercent, _ := p.CPUPercentWithContext(ctx)
		memPercent, _ := p.MemoryPercentWithContext(ctx)
		memInfo, _ := p.MemoryInfoWithContext(ctx)
		createTime, _ := p.CreateTimeWithContext(ctx)
		cmdline, _ := p.CmdlineWithContext(ctx)

		status := ""
		if len(statusSlice) > 0 {
			status = statusSlice[0]
		}

		var memRSS, memVMS uint64
		if memInfo != nil {
			memRSS = memInfo.RSS
			memVMS = memInfo.VMS
		}

		processes = append(processes, ProcessInfo{
			PID:        pid,
			Name:       name,
			User:       user,
			Status:     status,
			CPUPercent: cpuPercent,
			MemPercent: memPercent,
			MemRSS:     memRSS,
			MemVMS:     memVMS,
			CreateTime: createTime,
			Cmdline:    cmdline,
		})
	}

	// 按 CPU 使用率排序
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].CPUPercent > processes[j].CPUPercent
	})

	// 只保留 top N
	if len(processes) > m.config.TopProcessCount {
		processes = processes[:m.config.TopProcessCount]
	}

	m.mu.Lock()
	m.processes = processes
	m.mu.Unlock()
}

// collectDiskUsage 采集磁盘使用信息
func (m *Manager) collectDiskUsage(ctx context.Context) {
	partitions, err := disk.PartitionsWithContext(ctx, false)
	if err != nil {
		m.logger.Error("获取磁盘分区失败", zap.Error(err))
		return
	}

	diskUsages := make([]DiskUsageInfo, 0, len(partitions))

	for _, p := range partitions {
		usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil {
			m.logger.Warn("获取磁盘使用失败",
				zap.String("mountpoint", p.Mountpoint),
				zap.Error(err))
			continue
		}

		diskUsages = append(diskUsages, DiskUsageInfo{
			MountPoint:  p.Mountpoint,
			Device:      p.Device,
			FSType:      p.Fstype,
			Total:       usage.Total,
			Used:        usage.Used,
			Free:        usage.Free,
			UsedPercent: usage.UsedPercent,
			InodesTotal: usage.InodesTotal,
			InodesUsed:  usage.InodesUsed,
			InodesFree:  usage.InodesFree,
		})
	}

	m.mu.Lock()
	m.diskUsage = diskUsages
	m.mu.Unlock()
}

// collectNetwork 采集网络信息
func (m *Manager) collectNetwork(ctx context.Context) {
	conns, err := net.ConnectionsWithContext(ctx, "all")
	if err != nil {
		m.logger.Error("获取网络连接失败", zap.Error(err))
		return
	}

	connections := make([]ConnectionInfo, 0, len(conns))
	tcpCount, udpCount, listenCount, establishedCount := 0, 0, 0, 0

	for _, c := range conns {
		ci := ConnectionInfo{
			Family: c.Family,
			Type:   c.Type,
			Status: c.Status,
			PID:    c.Pid,
		}

		if c.Laddr.IP != "" {
			ci.LocalAddr = c.Laddr.IP
			ci.LocalPort = c.Laddr.Port
		}
		if c.Raddr.IP != "" {
			ci.RemoteAddr = c.Raddr.IP
			ci.RemotePort = c.Raddr.Port
		}

		connections = append(connections, ci)

		// 统计
		if c.Type == 1 { // TCP
			tcpCount++
		} else if c.Type == 2 { // UDP
			udpCount++
		}
		if c.Status == "LISTEN" {
			listenCount++
		} else if c.Status == "ESTABLISHED" {
			establishedCount++
		}
	}

	// 获取网络 IO 计数器
	var bytesSent, bytesRecv, packetsSent, packetsRecv uint64
	ioCounters, err := net.IOCountersWithContext(ctx, false)
	if err == nil && len(ioCounters) > 0 {
		bytesSent = ioCounters[0].BytesSent
		bytesRecv = ioCounters[0].BytesRecv
		packetsSent = ioCounters[0].PacketsSent
		packetsRecv = ioCounters[0].PacketsRecv
	}

	network := NetworkInfo{
		Connections:      connections,
		TCPCount:         tcpCount,
		UDPCount:         udpCount,
		ListenCount:      listenCount,
		EstablishedCount: establishedCount,
		BytesSent:        bytesSent,
		BytesRecv:        bytesRecv,
		PacketsSent:      packetsSent,
		PacketsRecv:      packetsRecv,
		Timestamp:        time.Now().Unix(),
	}

	m.mu.Lock()
	m.network = network
	m.mu.Unlock()
}

// collectLoad 采集系统负载
func (m *Manager) collectLoad(ctx context.Context) {
	loadAvg, err := load.AvgWithContext(ctx)
	if err != nil {
		m.logger.Error("获取系统负载失败", zap.Error(err))
		return
	}

	cpuCores, _ := cpu.CountsWithContext(ctx, true)

	loadInfo := LoadInfo{
		Load1:     loadAvg.Load1,
		Load5:     loadAvg.Load5,
		Load15:    loadAvg.Load15,
		CPUCores:  cpuCores,
		Timestamp: time.Now().Unix(),
	}

	m.mu.Lock()
	m.loadInfo = loadInfo
	m.mu.Unlock()
}

// collectUptime 采集运行时间信息
func (m *Manager) collectUptime(ctx context.Context) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		m.logger.Error("获取运行时间失败", zap.Error(err))
		return
	}

	users, err := host.UsersWithContext(ctx)
	if err != nil {
		m.logger.Warn("获取用户会话失败", zap.Error(err))
	}

	bootTime := time.Unix(int64(info.BootTime), 0)
	uptimeDuration := time.Duration(info.Uptime) * time.Second

	uptimeInfo := UptimeInfo{
		Uptime:      info.Uptime,
		BootTime:    info.BootTime,
		BootTimeStr: bootTime.Format("2006-01-02 15:04:05"),
		UptimeStr:   formatDuration(uptimeDuration),
		NumUsers:    len(users),
		Timestamp:   time.Now().Unix(),
	}

	m.mu.Lock()
	m.uptime = uptimeInfo
	m.mu.Unlock()
}

// checkAlerts 检查告警
func (m *Manager) checkAlerts(ctx context.Context) {
	var alerts []Alert
	now := time.Now().Unix()

	m.mu.RLock()
	overview := m.overview
	diskUsage := m.diskUsage
	config := m.config
	m.mu.RUnlock()

	if overview == nil {
		return
	}

	// CPU 告警
	if overview.CPUPercent > config.CPUAlert {
		alerts = append(alerts, Alert{
			Type:      "cpu",
			Level:     "warning",
			Message:   fmt.Sprintf("CPU使用率 %.1f%% 超过阈值 %.1f%%", overview.CPUPercent, config.CPUAlert),
			Value:     overview.CPUPercent,
			Threshold: config.CPUAlert,
			Timestamp: now,
		})
	}

	// 内存告警
	if overview.MemPercent > config.MemAlert {
		alerts = append(alerts, Alert{
			Type:      "memory",
			Level:     "warning",
			Message:   fmt.Sprintf("内存使用率 %.1f%% 超过阈值 %.1f%%", overview.MemPercent, config.MemAlert),
			Value:     overview.MemPercent,
			Threshold: config.MemAlert,
			Timestamp: now,
		})
	}

	// 磁盘告警
	for _, du := range diskUsage {
		if du.UsedPercent > config.DiskAlert {
			alerts = append(alerts, Alert{
				Type:      "disk",
				Level:     "critical",
				Message:   fmt.Sprintf("磁盘 %s 使用率 %.1f%% 超过阈值 %.1f%%", du.MountPoint, du.UsedPercent, config.DiskAlert),
				Value:     du.UsedPercent,
				Threshold: config.DiskAlert,
				Timestamp: now,
			})
		}
	}

	m.mu.Lock()
	m.alerts = alerts
	m.mu.Unlock()
}

// recordHistory 记录历史数据点
func (m *Manager) recordHistory() {
	m.mu.RLock()
	overview := m.overview
	loadInfo := m.loadInfo
	maxSize := m.config.HistoryMaxSize
	m.mu.RUnlock()

	if overview == nil {
		return
	}

	// 计算磁盘平均使用率
	var avgDiskPercent float64
	m.mu.RLock()
	if len(m.diskUsage) > 0 {
		for _, du := range m.diskUsage {
			avgDiskPercent += du.UsedPercent
		}
		avgDiskPercent /= float64(len(m.diskUsage))
	}
	m.mu.RUnlock()

	point := HistoryPoint{
		Timestamp:   time.Now().Unix(),
		CPUPercent:  overview.CPUPercent,
		MemPercent:  overview.MemPercent,
		DiskPercent: avgDiskPercent,
		Load1:       loadInfo.Load1,
		Load5:       loadInfo.Load5,
		Load15:      loadInfo.Load15,
	}

	m.mu.Lock()
	m.history = append(m.history, point)
	// 限制历史记录数量
	if len(m.history) > maxSize {
		m.history = m.history[len(m.history)-maxSize:]
	}
	m.mu.Unlock()
}

// GetOverview 获取系统概览
func (m *Manager) GetOverview() *SystemOverview {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.overview
}

// GetProcesses 获取进程列表
func (m *Manager) GetProcesses() []ProcessInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]ProcessInfo, len(m.processes))
	copy(result, m.processes)
	return result
}

// GetDiskUsage 获取磁盘使用
func (m *Manager) GetDiskUsage() []DiskUsageInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]DiskUsageInfo, len(m.diskUsage))
	copy(result, m.diskUsage)
	return result
}

// GetNetwork 获取网络信息
func (m *Manager) GetNetwork() NetworkInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.network
}

// GetLoad 获取系统负载
func (m *Manager) GetLoad() LoadInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.loadInfo
}

// GetUptime 获取运行时间
func (m *Manager) GetUptime() UptimeInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.uptime
}

// GetAlerts 获取当前告警
func (m *Manager) GetAlerts() []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Alert, len(m.alerts))
	copy(result, m.alerts)
	return result
}

// GetHistory 获取历史数据
func (m *Manager) GetHistory() []HistoryPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]HistoryPoint, len(m.history))
	copy(result, m.history)
	return result
}

// formatDuration 格式化时长
func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%d天%d小时%d分钟", days, hours, minutes)
	} else if hours > 0 {
		return fmt.Sprintf("%d小时%d分钟", hours, minutes)
	}
	return fmt.Sprintf("%d分钟", minutes)
}
