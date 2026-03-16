package system

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"nas-os/pkg/safeguards"
	_ "modernc.org/sqlite"
)

// Monitor 系统监控器
type Monitor struct {
	hostname       string
	db             *sql.DB
	clients        map[string]*websocket.Conn
	clientsMu      sync.RWMutex
	historyMu      sync.RWMutex
	stopChan       chan struct{}
	dataInterval   time.Duration
	historyEnabled bool
}

// SystemStats 系统统计信息
type SystemStats struct {
	CPUUsage      float64   `json:"cpuUsage"`
	CPUCores      int       `json:"cpuCores"`
	CPUTemp       int       `json:"cpuTemp,omitempty"`
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

// DiskStats 磁盘统计信息
type DiskStats struct {
	Device       string  `json:"device"`
	MountPoint   string  `json:"mountPoint"`
	Total        uint64  `json:"total"`
	Used         uint64  `json:"used"`
	Free         uint64  `json:"free"`
	UsagePercent float64 `json:"usagePercent"`
	FSType       string  `json:"fsType"`
	ReadBytes    uint64  `json:"readBytes,omitempty"`
	WriteBytes   uint64  `json:"writeBytes,omitempty"`
	ReadOps      uint64  `json:"readOps,omitempty"`
	WriteOps     uint64  `json:"writeOps,omitempty"`
}

// NetworkStats 网络统计信息
type NetworkStats struct {
	Interface  string    `json:"interface"`
	RXBytes    uint64    `json:"rxBytes"`
	TXBytes    uint64    `json:"txBytes"`
	RXPackets  uint64    `json:"rxPackets"`
	TXPackets  uint64    `json:"txPackets"`
	RXErrors   uint64    `json:"rxErrors"`
	TXErrors   uint64    `json:"txErrors"`
	RXSpeed    uint64    `json:"rxSpeed"` // 实时速度 (bytes/s)
	TXSpeed    uint64    `json:"txSpeed"`
	LastUpdate time.Time `json:"lastUpdate"`
}

// NetworkSpeed 网络速度快照
type NetworkSpeed struct {
	RXBytes uint64    `json:"rxBytes"`
	TXBytes uint64    `json:"txBytes"`
	Time    time.Time `json:"time"`
}

// SMARTInfo SMART 信息
type SMARTInfo struct {
	Device       string `json:"device"`
	Model        string `json:"model"`
	Serial       string `json:"serial"`
	Temperature  int    `json:"temperature"`
	Health       string `json:"health"`
	PowerOnHours uint64 `json:"powerOnHours"`
	ReadErrors   uint64 `json:"readErrors"`
	WriteErrors  uint64 `json:"writeErrors"`
	Reallocated  uint64 `json:"reallocated"`
	Pending      uint64 `json:"pending"`
}

// ProcessInfo 进程信息
type ProcessInfo struct {
	PID      int     `json:"pid"`
	Name     string  `json:"name"`
	CPU      float64 `json:"cpu"`
	Memory   float64 `json:"memory"`
	MemoryMB float64 `json:"memoryMB"`
	User     string  `json:"user"`
	Status   string  `json:"status"`
	CPUTime  string  `json:"cpuTime"`
}

// HistoryData 历史数据点
type HistoryData struct {
	Timestamp time.Time `json:"timestamp"`
	CPU       float64   `json:"cpu"`
	Memory    float64   `json:"memory"`
	NetRX     uint64    `json:"netRX"`
	NetTX     uint64    `json:"netTX"`
}

// Alert 告警信息
type Alert struct {
	ID           string    `json:"id"`
	Type         string    `json:"type"`
	Level        string    `json:"level"`
	Message      string    `json:"message"`
	Source       string    `json:"source"`
	Timestamp    time.Time `json:"timestamp"`
	Acknowledged bool      `json:"acknowledged"`
	Resolved     bool      `json:"resolved"`
}

// RealTimeData 实时数据（WebSocket 推送）
type RealTimeData struct {
	Type      string          `json:"type"`
	System    *SystemStats    `json:"system,omitempty"`
	Disks     []*DiskStats    `json:"disks,omitempty"`
	Network   []*NetworkStats `json:"network,omitempty"`
	Alerts    []*Alert        `json:"alerts,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// NewMonitor 创建监控器
func NewMonitor(dbPath string) (*Monitor, error) {
	hostname, _ := os.Hostname()

	m := &Monitor{
		hostname:       hostname,
		clients:        make(map[string]*websocket.Conn),
		stopChan:       make(chan struct{}),
		dataInterval:   time.Second,
		historyEnabled: true,
	}

	// 初始化数据库
	if err := m.initDB(dbPath); err != nil {
		return nil, fmt.Errorf("初始化数据库失败：%w", err)
	}

	// 启动数据采集协程
	go m.startDataCollection()

	return m, nil
}

// initDB 初始化 SQLite 数据库
func (m *Monitor) initDB(dbPath string) error {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return err
	}

	m.db = db

	// 创建历史数据表
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS system_history (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL,
		cpu_usage REAL,
		memory_usage REAL,
		memory_total INTEGER,
		memory_used INTEGER,
		net_rx_bytes INTEGER,
		net_tx_bytes INTEGER,
		net_rx_speed INTEGER,
		net_tx_speed INTEGER,
		disk_read_bytes INTEGER,
		disk_write_bytes INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_timestamp ON system_history(timestamp);
	
	CREATE TABLE IF NOT EXISTS alerts (
		id TEXT PRIMARY KEY,
		type TEXT,
		level TEXT,
		message TEXT,
		source TEXT,
		timestamp DATETIME,
		acknowledged INTEGER,
		resolved INTEGER
	);
	`

	_, err = db.Exec(createTableSQL)
	return err
}

// RegisterClient 注册 WebSocket 客户端
func (m *Monitor) RegisterClient(id string, conn *websocket.Conn) {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()
	m.clients[id] = conn
}

// UnregisterClient 注销 WebSocket 客户端
func (m *Monitor) UnregisterClient(id string) {
	m.clientsMu.Lock()
	defer m.clientsMu.Unlock()
	delete(m.clients, id)
}

// Broadcast 广播数据到所有客户端
func (m *Monitor) Broadcast(data *RealTimeData) {
	m.clientsMu.RLock()
	defer m.clientsMu.RUnlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}

	for id, conn := range m.clients {
		if err := conn.WriteMessage(websocket.TextMessage, jsonData); err != nil {
			// 连接错误，标记删除
			go func(cid string) {
				m.clientsMu.Lock()
				delete(m.clients, cid)
				m.clientsMu.Unlock()
				conn.Close()
			}(id)
		}
	}
}

// startDataCollection 启动数据采集
func (m *Monitor) startDataCollection() {
	ticker := time.NewTicker(m.dataInterval)
	defer ticker.Stop()

	var prevNetStats map[string]*NetworkSpeed
	prevDiskStats := make(map[string]struct {
		ReadBytes  uint64
		WriteBytes uint64
		ReadOps    uint64
		WriteOps   uint64
		Time       time.Time
	})

	for {
		select {
		case <-ticker.C:
			// 采集系统数据
			systemStats, _ := m.GetSystemStats()
			diskStats, _ := m.GetDiskStats()
			networkStats, _ := m.GetNetworkStats(prevNetStats)

			// 计算磁盘 IO 速度
			m.calculateDiskIO(diskStats, prevDiskStats)

			// 保存历史数据
			if m.historyEnabled {
				m.saveHistoryData(systemStats, networkStats)
			}

			// 广播实时数据
			m.Broadcast(&RealTimeData{
				Type:      "realtime",
				System:    systemStats,
				Disks:     diskStats,
				Network:   networkStats,
				Timestamp: time.Now(),
			})

			// 更新前一次网络统计
			prevNetStats = make(map[string]*NetworkSpeed)
			for _, net := range networkStats {
				prevNetStats[net.Interface] = &NetworkSpeed{
					RXBytes: net.RXBytes,
					TXBytes: net.TXBytes,
					Time:    time.Now(),
				}
			}

		case <-m.stopChan:
			return
		}
	}
}

// calculateDiskIO 计算磁盘 IO 速度
func (m *Monitor) calculateDiskIO(disks []*DiskStats, prev map[string]struct {
	ReadBytes  uint64
	WriteBytes uint64
	ReadOps    uint64
	WriteOps   uint64
	Time       time.Time
}) {
	now := time.Now()

	for _, disk := range disks {
		key := disk.Device
		if prevData, ok := prev[key]; ok {
			elapsed := now.Sub(prevData.Time).Seconds()
			if elapsed > 0 {
				// 这里需要从 /proc/diskstats 获取实际 IO 数据
				// 简化版本，暂时设为 0
				disk.ReadBytes = 0
				disk.WriteBytes = 0
				disk.ReadOps = 0
				disk.WriteOps = 0
			}
		}

		// 更新前一次数据
		prev[key] = struct {
			ReadBytes  uint64
			WriteBytes uint64
			ReadOps    uint64
			WriteOps   uint64
			Time       time.Time
		}{
			Time: now,
		}
	}
}

// GetSystemStats 获取系统统计信息
func (m *Monitor) GetSystemStats() (*SystemStats, error) {
	stats := &SystemStats{
		Timestamp: time.Now(),
		LoadAvg:   make([]float64, 3),
		CPUCores:  runtime.NumCPU(),
	}

	// CPU 使用率
	cpuUsage, err := m.getCPUUsage()
	if err == nil {
		stats.CPUUsage = cpuUsage
	}

	// CPU 温度
	stats.CPUTemp = m.getCPUTemp()

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
func (m *Monitor) GetDiskStats() ([]*DiskStats, error) {
	var stats []*DiskStats

	// 使用 df 命令获取磁盘信息
	cmd := exec.Command("df", "-B1", "--output=source,target,size,used,avail,fstype")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法获取磁盘信息：%w", err)
	}

	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	scanner.Scan() // 跳过标题行

	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		// 跳过虚拟文件系统
		if strings.HasPrefix(fields[0], "tmpfs") || strings.HasPrefix(fields[0], "overlay") {
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
func (m *Monitor) GetNetworkStats(prev map[string]*NetworkSpeed) ([]*NetworkStats, error) {
	var stats []*NetworkStats
	now := time.Now()

	// 读取 /proc/net/dev
	data, err := os.ReadFile("/proc/net/dev")
	if err != nil {
		return nil, fmt.Errorf("无法读取网络统计：%w", err)
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

		netStat := &NetworkStats{
			Interface:  iface,
			RXBytes:    rxBytes,
			TXBytes:    txBytes,
			RXPackets:  rxPackets,
			TXPackets:  txPackets,
			RXErrors:   rxErrors,
			TXErrors:   txErrors,
			LastUpdate: now,
		}

		// 计算实时速度
		if prevData, ok := prev[iface]; ok {
			elapsed := now.Sub(prevData.Time).Seconds()
			if elapsed > 0 {
				if rxBytes >= prevData.RXBytes {
					netStat.RXSpeed = uint64(float64(rxBytes-prevData.RXBytes) / elapsed)
				}
				if txBytes >= prevData.TXBytes {
					netStat.TXSpeed = uint64(float64(txBytes-prevData.TXBytes) / elapsed)
				}
			}
		}

		stats = append(stats, netStat)
	}

	return stats, nil
}

// GetSMARTInfo 获取磁盘 SMART 信息
func (m *Monitor) GetSMARTInfo(device string) (*SMARTInfo, error) {
	info := &SMARTInfo{
		Device: device,
		Health: "UNKNOWN",
	}

	// 检查 smartctl 是否可用
	if _, err := exec.LookPath("smartctl"); err != nil {
		return nil, fmt.Errorf("smartctl 未安装")
	}

	// 获取 SMART 信息
	cmd := exec.Command("smartctl", "-A", "-i", "-H", device)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法获取 SMART 信息：%w", err)
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

		// 解析重映射扇区
		if strings.Contains(line, "Reallocated_Sector_Ct") {
			fields := strings.Fields(line)
			if len(fields) >= 10 {
				count, _ := strconv.ParseUint(fields[9], 10, 64)
				info.Reallocated = count
			}
		}

		// 解析待映射扇区
		if strings.Contains(line, "Current_Pending_Sector") {
			fields := strings.Fields(line)
			if len(fields) >= 10 {
				count, _ := strconv.ParseUint(fields[9], 10, 64)
				info.Pending = count
			}
		}
	}

	return info, nil
}

// CheckAllDisks 检查所有磁盘
func (m *Monitor) CheckAllDisks() ([]*SMARTInfo, error) {
	var results []*SMARTInfo

	// 列出所有块设备
	cmd := exec.Command("lsblk", "-d", "-n", "-o", "NAME")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法列出磁盘：%w", err)
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

// GetTopProcesses 获取资源占用 Top10 进程
func (m *Monitor) GetTopProcesses(limit int) ([]*ProcessInfo, error) {
	if limit <= 0 {
		limit = 10
	}

	cmd := exec.Command("ps", "aux", "--sort=-%cpu")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("无法获取进程列表：%w", err)
	}

	var processes []*ProcessInfo
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	// 跳过标题行
	scanner.Scan()

	count := 0
	for scanner.Scan() && count < limit {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}

		pid, _ := strconv.Atoi(fields[1])
		cpu, _ := strconv.ParseFloat(fields[2], 64)
		mem, _ := strconv.ParseFloat(fields[3], 64)
		memKB, _ := strconv.ParseFloat(fields[4], 64)

		processes = append(processes, &ProcessInfo{
			PID:      pid,
			Name:     fields[10],
			CPU:      cpu,
			Memory:   mem,
			MemoryMB: memKB / 1024,
			User:     fields[0],
			Status:   fields[7],
			CPUTime:  fields[9],
		})
		count++
	}

	return processes, nil
}

// GetHistoryData 获取历史数据
func (m *Monitor) GetHistoryData(duration string, interval string) ([]*HistoryData, error) {
	var timeRange string
	switch duration {
	case "24h":
		timeRange = "24 hours"
	case "7d":
		timeRange = "7 days"
	case "30d":
		timeRange = "30 days"
	default:
		timeRange = "24 hours"
	}

	query := `
	SELECT timestamp, cpu_usage, memory_usage, net_rx_speed, net_tx_speed
	FROM system_history
	WHERE timestamp >= datetime('now', '-' || ?)
	ORDER BY timestamp ASC
	`

	rows, err := m.db.Query(query, timeRange)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var data []*HistoryData
	for rows.Next() {
		var h HistoryData
		var netRX, netTX int64
		err := rows.Scan(&h.Timestamp, &h.CPU, &h.Memory, &netRX, &netTX)
		if err != nil {
			continue
		}
		h.NetRX = uint64(netRX)
		h.NetTX = uint64(netTX)
		data = append(data, &h)
	}

	return data, rows.Err()
}

// GetAlerts 获取告警列表
func (m *Monitor) GetAlerts() ([]*Alert, error) {
	query := `SELECT id, type, level, message, source, timestamp, acknowledged, resolved FROM alerts ORDER BY timestamp DESC`

	rows, err := m.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*Alert
	for rows.Next() {
		var a Alert
		var ack, resolved int
		err := rows.Scan(&a.ID, &a.Type, &a.Level, &a.Message, &a.Source, &a.Timestamp, &ack, &resolved)
		if err != nil {
			continue
		}
		a.Acknowledged = ack == 1
		a.Resolved = resolved == 1
		alerts = append(alerts, &a)
	}

	return alerts, rows.Err()
}

// AddAlert 添加告警
func (m *Monitor) AddAlert(alert *Alert) error {
	query := `INSERT OR REPLACE INTO alerts (id, type, level, message, source, timestamp, acknowledged, resolved) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := m.db.Exec(query, alert.ID, alert.Type, alert.Level, alert.Message, alert.Source,
		alert.Timestamp, boolToInt(alert.Acknowledged), boolToInt(alert.Resolved))
	return err
}

// AcknowledgeAlert 确认告警
func (m *Monitor) AcknowledgeAlert(id string) error {
	query := `UPDATE alerts SET acknowledged = 1 WHERE id = ?`
	_, err := m.db.Exec(query, id)
	return err
}

// saveHistoryData 保存历史数据
func (m *Monitor) saveHistoryData(system *SystemStats, network []*NetworkStats) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()

	var netRX, netTX int64
	for _, n := range network {
		// 使用安全转换避免 integer overflow
		if rx, err := safeguards.SafeUint64ToInt64(n.RXSpeed); err == nil {
			netRX += rx
		}
		if tx, err := safeguards.SafeUint64ToInt64(n.TXSpeed); err == nil {
			netTX += tx
		}
	}

	query := `INSERT INTO system_history (timestamp, cpu_usage, memory_usage, memory_total, memory_used, 
			  net_rx_bytes, net_tx_bytes, net_rx_speed, net_tx_speed) 
			  VALUES (?, ?, ?, ?, ?, 0, 0, ?, ?)`

	_, err := m.db.Exec(query, system.Timestamp, system.CPUUsage, system.MemoryUsage,
		system.MemoryTotal, system.MemoryUsed, netRX, netTX)

	if err != nil {
		// 记录错误但不中断
		return
	}

	// 清理旧数据（保留 90 天）
	m.cleanupOldData()
}

// cleanupOldData 清理旧数据
func (m *Monitor) cleanupOldData() {
	_, err := m.db.Exec(`DELETE FROM system_history WHERE timestamp < datetime('now', '-90 days')`)
	if err != nil {
		return
	}
}

// 辅助函数
func (m *Monitor) getCPUUsage() (float64, error) {
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

	idle, _ := strconv.ParseFloat(fields[4], 64)
	total := 0.0
	for i := 1; i < len(fields) && i <= 7; i++ {
		val, _ := strconv.ParseFloat(fields[i], 64)
		total += val
	}

	if total == 0 {
		return 0, nil
	}

	return (total - idle) / total * 100, nil
}

func (m *Monitor) getCPUTemp() int {
	// 尝试读取 CPU 温度
	paths := []string{
		"/sys/class/thermal/thermal_zone0/temp",
		"/sys/class/hwmon/hwmon0/temp1_input",
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			temp, _ := strconv.Atoi(strings.TrimSpace(string(data)))
			return temp / 1000 // 转换为摄氏度
		}
	}
	return 0
}

func (m *Monitor) getMemoryInfo() (map[string]uint64, error) {
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
				result["Total"] = value * 1024
			case "MemFree":
				result["Free"] = value * 1024
			case "MemAvailable":
				result["Available"] = value * 1024
			}
		}
	}

	return result, nil
}

func (m *Monitor) getSwapInfo() (map[string]uint64, error) {
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

func (m *Monitor) getUptime() (uint64, error) {
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

func (m *Monitor) formatUptime(seconds uint64) string {
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

func (m *Monitor) getLoadAverage() ([]float64, error) {
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

func (m *Monitor) GetHostname() string {
	return m.hostname
}

func (m *Monitor) Close() {
	close(m.stopChan)
	if m.db != nil {
		m.db.Close()
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
