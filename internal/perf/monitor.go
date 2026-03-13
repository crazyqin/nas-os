package perf

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ResourceMonitor provides real-time resource monitoring
type ResourceMonitor struct {
	interval    time.Duration
	historySize int

	cpuHistory    []float64
	memHistory    []float64
	diskIOHistory []DiskIOStats
	netIOHistory  []NetIOStats

	mu       sync.RWMutex
	stopChan chan struct{}
	running  bool

	// Callbacks
	onHighCPU    func(usage float64)
	onHighMemory func(usage float64)

	logger *zap.Logger
}

// CPUStats holds CPU statistics
type CPUStats struct {
	UsagePercent  float64   `json:"usage_percent"`
	UserPercent   float64   `json:"user_percent"`
	SystemPercent float64   `json:"system_percent"`
	IdlePercent   float64   `json:"idle_percent"`
	Cores         int       `json:"cores"`
	Temperature   int       `json:"temperature,omitempty"`
	LoadAvg1      float64   `json:"load_avg_1"`
	LoadAvg5      float64   `json:"load_avg_5"`
	LoadAvg15     float64   `json:"load_avg_15"`
	Timestamp     time.Time `json:"timestamp"`
}

// MemoryStats holds memory statistics
type MemoryStats struct {
	Total        uint64    `json:"total"`
	Used         uint64    `json:"used"`
	Free         uint64    `json:"free"`
	Available    uint64    `json:"available"`
	UsagePercent float64   `json:"usage_percent"`
	SwapTotal    uint64    `json:"swap_total"`
	SwapUsed     uint64    `json:"swap_used"`
	SwapFree     uint64    `json:"swap_free"`
	SwapPercent  float64   `json:"swap_percent"`
	Cached       uint64    `json:"cached"`
	Buffers      uint64    `json:"buffers"`
	Timestamp    time.Time `json:"timestamp"`
}

// DiskIOStats holds disk I/O statistics
type DiskIOStats struct {
	Device     string    `json:"device"`
	ReadBytes  uint64    `json:"read_bytes"`
	WriteBytes uint64    `json:"write_bytes"`
	ReadOps    uint64    `json:"read_ops"`
	WriteOps   uint64    `json:"write_ops"`
	ReadSpeed  uint64    `json:"read_speed"` // bytes/s
	WriteSpeed uint64    `json:"write_speed"`
	Timestamp  time.Time `json:"timestamp"`
}

// NetIOStats holds network I/O statistics
type NetIOStats struct {
	Interface string    `json:"interface"`
	RXBytes   uint64    `json:"rx_bytes"`
	TXBytes   uint64    `json:"tx_bytes"`
	RXPackets uint64    `json:"rx_packets"`
	TXPackets uint64    `json:"tx_packets"`
	RXErrors  uint64    `json:"rx_errors"`
	TXErrors  uint64    `json:"tx_errors"`
	RXSpeed   uint64    `json:"rx_speed"` // bytes/s
	TXSpeed   uint64    `json:"tx_speed"`
	Timestamp time.Time `json:"timestamp"`
}

// ProcessStats holds process statistics
type ProcessStats struct {
	PID           int       `json:"pid"`
	Name          string    `json:"name"`
	CPUUsage      float64   `json:"cpu_usage"`
	MemoryUsage   uint64    `json:"memory_usage"`
	MemoryPercent float64   `json:"memory_percent"`
	Status        string    `json:"status"`
	NumThreads    int       `json:"num_threads"`
	Timestamp     time.Time `json:"timestamp"`
}

// SystemHealth holds overall system health
type SystemHealth struct {
	CPUHealthy     bool     `json:"cpu_healthy"`
	MemoryHealthy  bool     `json:"memory_healthy"`
	DiskHealthy    bool     `json:"disk_healthy"`
	NetworkHealthy bool     `json:"network_healthy"`
	OverallScore   float64  `json:"overall_score"` // 0-100
	Issues         []string `json:"issues,omitempty"`
}

// NewResourceMonitor creates a new resource monitor
func NewResourceMonitor(interval time.Duration, historySize int, logger *zap.Logger) *ResourceMonitor {
	return &ResourceMonitor{
		interval:      interval,
		historySize:   historySize,
		cpuHistory:    make([]float64, 0, historySize),
		memHistory:    make([]float64, 0, historySize),
		diskIOHistory: make([]DiskIOStats, 0, historySize),
		netIOHistory:  make([]NetIOStats, 0, historySize),
		logger:        logger,
	}
}

// Start starts the monitoring loop
func (m *ResourceMonitor) Start() {
	if m.running {
		return
	}

	m.running = true
	m.stopChan = make(chan struct{})

	go m.monitorLoop()
	m.logger.Info("Resource monitor started")
}

// Stop stops the monitoring loop
func (m *ResourceMonitor) Stop() {
	if !m.running {
		return
	}

	close(m.stopChan)
	m.running = false
	m.logger.Info("Resource monitor stopped")
}

// monitorLoop is the main monitoring loop
func (m *ResourceMonitor) monitorLoop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	prevDiskIO := make(map[string]DiskIOStats)
	prevNetIO := make(map[string]NetIOStats)

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			// Collect metrics
			cpuStats := m.getCPUStats()
			memStats := m.getMemoryStats()
			diskIOStats := m.getDiskIOStats(prevDiskIO)
			netIOStats := m.getNetIOStats(prevNetIO)

			// Update history
			m.mu.Lock()
			m.addHistory(cpuStats.UsagePercent, memStats.UsagePercent, diskIOStats, netIOStats)
			m.mu.Unlock()

			// Update previous stats for speed calculation
			for _, d := range diskIOStats {
				prevDiskIO[d.Device] = d
			}
			for _, n := range netIOStats {
				prevNetIO[n.Interface] = n
			}

			// Check thresholds
			m.checkThresholds(cpuStats, memStats)
		}
	}
}

// addHistory adds stats to history
func (m *ResourceMonitor) addHistory(cpu, mem float64, diskIO []DiskIOStats, netIO []NetIOStats) {
	m.cpuHistory = append(m.cpuHistory, cpu)
	m.memHistory = append(m.memHistory, mem)

	if len(m.cpuHistory) > m.historySize {
		m.cpuHistory = m.cpuHistory[1:]
	}
	if len(m.memHistory) > m.historySize {
		m.memHistory = m.memHistory[1:]
	}

	m.diskIOHistory = append(m.diskIOHistory, diskIO...)
	m.netIOHistory = append(m.netIOHistory, netIO...)

	if len(m.diskIOHistory) > m.historySize {
		m.diskIOHistory = m.diskIOHistory[len(m.diskIOHistory)-m.historySize:]
	}
	if len(m.netIOHistory) > m.historySize {
		m.netIOHistory = m.netIOHistory[len(m.netIOHistory)-m.historySize:]
	}
}

// checkThresholds checks resource thresholds and triggers callbacks
func (m *ResourceMonitor) checkThresholds(cpu CPUStats, mem MemoryStats) {
	if cpu.UsagePercent > 90 && m.onHighCPU != nil {
		m.onHighCPU(cpu.UsagePercent)
	}

	if mem.UsagePercent > 90 && m.onHighMemory != nil {
		m.onHighMemory(mem.UsagePercent)
	}
}

// SetHighCPUCallback sets callback for high CPU usage
func (m *ResourceMonitor) SetHighCPUCallback(callback func(float64)) {
	m.onHighCPU = callback
}

// SetHighMemoryCallback sets callback for high memory usage
func (m *ResourceMonitor) SetHighMemoryCallback(callback func(float64)) {
	m.onHighMemory = callback
}

// GetCPUStats returns current CPU stats
func (m *ResourceMonitor) GetCPUStats() CPUStats {
	return m.getCPUStats()
}

// GetMemoryStats returns current memory stats
func (m *ResourceMonitor) GetMemoryStats() MemoryStats {
	return m.getMemoryStats()
}

// GetDiskIOStats returns current disk I/O stats
func (m *ResourceMonitor) GetDiskIOStats() []DiskIOStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.diskIOHistory) == 0 {
		return m.getDiskIOStats(nil)
	}
	return []DiskIOStats{m.diskIOHistory[len(m.diskIOHistory)-1]}
}

// GetNetIOStats returns current network I/O stats
func (m *ResourceMonitor) GetNetIOStats() []NetIOStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.netIOHistory) == 0 {
		return m.getNetIOStats(nil)
	}
	return []NetIOStats{m.netIOHistory[len(m.netIOHistory)-1]}
}

// GetHistory returns historical data
func (m *ResourceMonitor) GetHistory() (cpu, mem []float64, diskIO []DiskIOStats, netIO []NetIOStats) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cpu = make([]float64, len(m.cpuHistory))
	copy(cpu, m.cpuHistory)

	mem = make([]float64, len(m.memHistory))
	copy(mem, m.memHistory)

	diskIO = make([]DiskIOStats, len(m.diskIOHistory))
	copy(diskIO, m.diskIOHistory)

	netIO = make([]NetIOStats, len(m.netIOHistory))
	copy(netIO, m.netIOHistory)

	return
}

// GetHealth returns system health status
func (m *ResourceMonitor) GetHealth() SystemHealth {
	cpu := m.getCPUStats()
	mem := m.getMemoryStats()

	health := SystemHealth{
		CPUHealthy:     cpu.UsagePercent < 90,
		MemoryHealthy:  mem.UsagePercent < 90,
		DiskHealthy:    true,
		NetworkHealthy: true,
	}

	score := 100.0
	var issues []string

	if !health.CPUHealthy {
		score -= 25
		issues = append(issues, fmt.Sprintf("High CPU usage: %.1f%%", cpu.UsagePercent))
	}

	if !health.MemoryHealthy {
		score -= 25
		issues = append(issues, fmt.Sprintf("High memory usage: %.1f%%", mem.UsagePercent))
	}

	health.OverallScore = score
	health.Issues = issues

	return health
}

// getCPUStats collects CPU statistics
func (m *ResourceMonitor) getCPUStats() CPUStats {
	stats := CPUStats{
		Cores:     runtime.NumCPU(),
		Timestamp: time.Now(),
	}

	// Read /proc/stat
	cpuFile, err := os.Open("/proc/stat")
	if err == nil {
		defer cpuFile.Close()

		scanner := bufio.NewScanner(cpuFile)
		if scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 5 && fields[0] == "cpu" {
				user, _ := strconv.ParseFloat(fields[1], 64)
				nice, _ := strconv.ParseFloat(fields[2], 64)
				system, _ := strconv.ParseFloat(fields[3], 64)
				idle, _ := strconv.ParseFloat(fields[4], 64)

				total := user + nice + system + idle
				if total > 0 {
					stats.UserPercent = user / total * 100
					stats.SystemPercent = system / total * 100
					stats.IdlePercent = idle / total * 100
					stats.UsagePercent = 100 - stats.IdlePercent
				}
			}
		}
	}

	// Read load average
	if loadFile, err := os.Open("/proc/loadavg"); err == nil {
		defer loadFile.Close()

		scanner := bufio.NewScanner(loadFile)
		if scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 3 {
				stats.LoadAvg1, _ = strconv.ParseFloat(fields[0], 64)
				stats.LoadAvg5, _ = strconv.ParseFloat(fields[1], 64)
				stats.LoadAvg15, _ = strconv.ParseFloat(fields[2], 64)
			}
		}
	}

	// Try to get CPU temperature
	if temp, err := m.getCPUTemperature(); err == nil {
		stats.Temperature = temp
	}

	return stats
}

// getMemoryStats collects memory statistics
func (m *ResourceMonitor) getMemoryStats() MemoryStats {
	stats := MemoryStats{
		Timestamp: time.Now(),
	}

	memFile, err := os.Open("/proc/meminfo")
	if err != nil {
		return stats
	}
	defer memFile.Close()

	scanner := bufio.NewScanner(memFile)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}

		value, _ := strconv.ParseUint(fields[1], 10, 64)
		value *= 1024 // Convert from KB to bytes

		switch fields[0] {
		case "MemTotal:":
			stats.Total = value
		case "MemFree:":
			stats.Free = value
		case "MemAvailable:":
			stats.Available = value
		case "Buffers:":
			stats.Buffers = value
		case "Cached:":
			stats.Cached = value
		case "SwapTotal:":
			stats.SwapTotal = value
		case "SwapFree:":
			stats.SwapFree = value
		}
	}

	stats.Used = stats.Total - stats.Available
	stats.SwapUsed = stats.SwapTotal - stats.SwapFree

	if stats.Total > 0 {
		stats.UsagePercent = float64(stats.Used) / float64(stats.Total) * 100
	}
	if stats.SwapTotal > 0 {
		stats.SwapPercent = float64(stats.SwapUsed) / float64(stats.SwapTotal) * 100
	}

	return stats
}

// getDiskIOStats collects disk I/O statistics
func (m *ResourceMonitor) getDiskIOStats(prev map[string]DiskIOStats) []DiskIOStats {
	var stats []DiskIOStats

	ioFile, err := os.Open("/proc/diskstats")
	if err != nil {
		return stats
	}
	defer ioFile.Close()

	now := time.Now()
	scanner := bufio.NewScanner(ioFile)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 14 {
			continue
		}

		device := fields[2]
		// Skip non-disk devices
		if !strings.HasPrefix(device, "sd") && !strings.HasPrefix(device, "nvme") && !strings.HasPrefix(device, "vd") {
			continue
		}

		readSectors, _ := strconv.ParseUint(fields[5], 10, 64)
		writeSectors, _ := strconv.ParseUint(fields[9], 10, 64)
		readOps, _ := strconv.ParseUint(fields[3], 10, 64)
		writeOps, _ := strconv.ParseUint(fields[7], 10, 64)

		readBytes := readSectors * 512
		writeBytes := writeSectors * 512

		diskStat := DiskIOStats{
			Device:     device,
			ReadBytes:  readBytes,
			WriteBytes: writeBytes,
			ReadOps:    readOps,
			WriteOps:   writeOps,
			Timestamp:  now,
		}

		// Calculate speed if we have previous stats
		if prevStat, ok := prev[device]; ok {
			elapsed := now.Sub(prevStat.Timestamp).Seconds()
			if elapsed > 0 {
				diskStat.ReadSpeed = uint64(float64(readBytes-prevStat.ReadBytes) / elapsed)
				diskStat.WriteSpeed = uint64(float64(writeBytes-prevStat.WriteBytes) / elapsed)
			}
		}

		stats = append(stats, diskStat)
	}

	return stats
}

// getNetIOStats collects network I/O statistics
func (m *ResourceMonitor) getNetIOStats(prev map[string]NetIOStats) []NetIOStats {
	var stats []NetIOStats

	netFile, err := os.Open("/proc/net/dev")
	if err != nil {
		return stats
	}
	defer netFile.Close()

	now := time.Now()
	scanner := bufio.NewScanner(netFile)

	// Skip header lines
	scanner.Scan()
	scanner.Scan()

	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		iface := strings.TrimSpace(parts[0])
		fields := strings.Fields(parts[1])

		if len(fields) < 10 {
			continue
		}

		rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
		rxPackets, _ := strconv.ParseUint(fields[1], 10, 64)
		rxErrors, _ := strconv.ParseUint(fields[2], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[8], 10, 64)
		txPackets, _ := strconv.ParseUint(fields[9], 10, 64)
		txErrors, _ := strconv.ParseUint(fields[10], 10, 64)

		netStat := NetIOStats{
			Interface: iface,
			RXBytes:   rxBytes,
			TXBytes:   txBytes,
			RXPackets: rxPackets,
			TXPackets: txPackets,
			RXErrors:  rxErrors,
			TXErrors:  txErrors,
			Timestamp: now,
		}

		// Calculate speed if we have previous stats
		if prevStat, ok := prev[iface]; ok {
			elapsed := now.Sub(prevStat.Timestamp).Seconds()
			if elapsed > 0 {
				netStat.RXSpeed = uint64(float64(rxBytes-prevStat.RXBytes) / elapsed)
				netStat.TXSpeed = uint64(float64(txBytes-prevStat.TXBytes) / elapsed)
			}
		}

		stats = append(stats, netStat)
	}

	return stats
}

// getCPUTemperature tries to read CPU temperature
func (m *ResourceMonitor) getCPUTemperature() (int, error) {
	// Try thermal zones
	if content, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp"); err == nil {
		temp, _ := strconv.Atoi(strings.TrimSpace(string(content)))
		return temp / 1000, nil
	}

	// Try hwmon
	if content, err := os.ReadFile("/sys/class/hwmon/hwmon0/temp1_input"); err == nil {
		temp, _ := strconv.Atoi(strings.TrimSpace(string(content)))
		return temp / 1000, nil
	}

	return 0, fmt.Errorf("temperature not available")
}

// GetTopProcesses returns top processes by CPU/memory usage
func (m *ResourceMonitor) GetTopProcesses(n int) []ProcessStats {
	cmd := exec.Command("ps", "aux", "--sort=-%cpu")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	var processes []ProcessStats
	scanner := bufio.NewScanner(strings.NewReader(string(output)))

	// Skip header
	scanner.Scan()

	count := 0
	for scanner.Scan() && count < n {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 11 {
			continue
		}

		pid, _ := strconv.Atoi(fields[1])
		cpu, _ := strconv.ParseFloat(fields[2], 64)
		mem, _ := strconv.ParseFloat(fields[3], 64)

		processes = append(processes, ProcessStats{
			PID:           pid,
			Name:          fields[10],
			CPUUsage:      cpu,
			MemoryPercent: mem,
			Status:        "running",
			Timestamp:     time.Now(),
		})
		count++
	}

	return processes
}
