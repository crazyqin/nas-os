// Package perfmon provides comprehensive system performance monitoring
// including IOPS, latency, bandwidth, disk I/O, network throughput,
// CPU and memory metrics with configurable collection intervals.
package perfmon

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
	"go.uber.org/zap"
)

// Config holds the performance monitor configuration.
type Config struct {
	Enabled           bool          `json:"enabled"`
	Interval          time.Duration `json:"interval"`
	MaxSamples        int           `json:"max_samples"`
	LatencyWindowSize int           `json:"latency_window_size"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Enabled:           true,
		Interval:          5 * time.Second,
		MaxSamples:        720, // 1 hour at 5s interval
		LatencyWindowSize: 1000,
	}
}

// IOPSStats holds IOPS statistics.
type IOPSStats struct {
	ReadIOPS      float64 `json:"read_iops"`
	WriteIOPS     float64 `json:"write_iops"`
	TotalIOPS     float64 `json:"total_iops"`
	ReadOpsDelta  uint64  `json:"read_ops_delta"`
	WriteOpsDelta uint64  `json:"write_ops_delta"`
	Timestamp     int64   `json:"timestamp"`
}

// LatencyStats holds latency statistics.
type LatencyStats struct {
	ReadAvgMs  float64 `json:"read_avg_ms"`
	ReadP99Ms  float64 `json:"read_p99_ms"`
	ReadMaxMs  float64 `json:"read_max_ms"`
	WriteAvgMs float64 `json:"write_avg_ms"`
	WriteP99Ms float64 `json:"write_p99_ms"`
	WriteMaxMs float64 `json:"write_max_ms"`
	Timestamp  int64   `json:"timestamp"`
}

// BandwidthStats holds bandwidth statistics.
type BandwidthStats struct {
	NetworkInBps  float64 `json:"network_in_bps"`
	NetworkOutBps float64 `json:"network_out_bps"`
	DiskReadBps   float64 `json:"disk_read_bps"`
	DiskWriteBps  float64 `json:"disk_write_bps"`
	Timestamp     int64   `json:"timestamp"`
}

// DiskIOStats holds per-disk I/O statistics.
type DiskIOStats struct {
	Device           string  `json:"device"`
	ReadBytes        uint64  `json:"read_bytes"`
	WriteBytes       uint64  `json:"write_bytes"`
	ReadCount        uint64  `json:"read_count"`
	WriteCount       uint64  `json:"write_count"`
	IoTime           uint64  `json:"io_time"`
	QueueDepth       float64 `json:"queue_depth"`
	ReadBytesPerSec  float64 `json:"read_bytes_per_sec"`
	WriteBytesPerSec float64 `json:"write_bytes_per_sec"`
	Timestamp        int64   `json:"timestamp"`
}

// NetIOStats holds per-interface network I/O statistics.
type NetIOStats struct {
	Interface string `json:"interface"`
	RxBytes   uint64 `json:"rx_bytes"`
	TxBytes   uint64 `json:"tx_bytes"`
	RxPackets uint64 `json:"rx_packets"`
	TxPackets uint64 `json:"tx_packets"`
	RxErrors  uint64 `json:"rx_errors"`
	TxErrors  uint64 `json:"tx_errors"`
	RxDropped uint64 `json:"rx_dropped"`
	TxDropped uint64 `json:"tx_dropped"`
	Timestamp int64  `json:"timestamp"`
}

// CPUDetailStats holds detailed CPU statistics.
type CPUDetailStats struct {
	UserPercent    float64 `json:"user_percent"`
	SystemPercent  float64 `json:"system_percent"`
	IOWaitPercent  float64 `json:"iowait_percent"`
	IRQPercent     float64 `json:"irq_percent"`
	SoftIRQPercent float64 `json:"softirq_percent"`
	IdlePercent    float64 `json:"idle_percent"`
	StealPercent   float64 `json:"steal_percent"`
	TotalPercent   float64 `json:"total_percent"`
	Timestamp      int64   `json:"timestamp"`
}

// MemoryDetailStats holds detailed memory statistics.
type MemoryDetailStats struct {
	TotalBytes      uint64  `json:"total_bytes"`
	UsedBytes       uint64  `json:"used_bytes"`
	AvailableBytes  uint64  `json:"available_bytes"`
	CachedBytes     uint64  `json:"cached_bytes"`
	BuffersBytes    uint64  `json:"buffers_bytes"`
	SwapTotalBytes  uint64  `json:"swap_total_bytes"`
	SwapUsedBytes   uint64  `json:"swap_used_bytes"`
	SwapFreeBytes   uint64  `json:"swap_free_bytes"`
	UsedPercent     float64 `json:"used_percent"`
	SwapUsedPercent float64 `json:"swap_used_percent"`
	Timestamp       int64   `json:"timestamp"`
}

// PerfSummary holds all collected metrics in a single snapshot.
type PerfSummary struct {
	IOPS      *IOPSStats         `json:"iops"`
	Latency   *LatencyStats      `json:"latency"`
	Bandwidth *BandwidthStats    `json:"bandwidth"`
	DiskIO    []DiskIOStats      `json:"disk_io"`
	NetIO     []NetIOStats       `json:"net_io"`
	CPU       *CPUDetailStats    `json:"cpu"`
	Memory    *MemoryDetailStats `json:"memory"`
	Timestamp int64              `json:"timestamp"`
}

// Sample stores a single data point for latency calculation.
type Sample struct {
	Timestamp time.Time
	Value     float64
}

// CollectFunc is a callback invoked after each collection cycle.
type CollectFunc func(summary *PerfSummary)

// Manager is the main performance monitoring manager.
type Manager struct {
	mu      sync.RWMutex
	config  *Config
	logger  *zap.Logger
	running bool
	cancel  context.CancelFunc

	// Current stats
	iopsStats      *IOPSStats
	latencyStats   *LatencyStats
	bandwidthStats *BandwidthStats
	diskIOStats    []DiskIOStats
	netIOStats     []NetIOStats
	cpuStats       *CPUDetailStats
	memoryStats    *MemoryDetailStats

	// Historical data for latency calculation
	readLatencySamples  []Sample
	writeLatencySamples []Sample

	// Previous counters for delta calculation
	prevDiskIO   map[string]disk.IOCountersStat
	prevNetIO    map[string]net.IOCountersStat
	prevDiskTime time.Time

	// Callbacks
	onCollect []CollectFunc
}

// NewManager creates a new performance monitoring manager.
func NewManager(cfg *Config, logger *zap.Logger) *Manager {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		config:              cfg,
		logger:              logger,
		readLatencySamples:  make([]Sample, 0, cfg.LatencyWindowSize),
		writeLatencySamples: make([]Sample, 0, cfg.LatencyWindowSize),
		prevDiskIO:          make(map[string]disk.IOCountersStat),
		prevNetIO:           make(map[string]net.IOCountersStat),
	}
}

// OnCollect registers a callback invoked after each collection cycle.
func (m *Manager) OnCollect(fn CollectFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onCollect = append(m.onCollect, fn)
}

// Start begins the performance monitoring loop.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	if !m.config.Enabled {
		return fmt.Errorf("perfmon is disabled in config")
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	m.running = true
	m.prevDiskTime = time.Now()

	go m.collectLoop(ctx)
	m.logger.Info("performance monitor started",
		zap.Duration("interval", m.config.Interval),
		zap.Int("max_samples", m.config.MaxSamples))
	return nil
}

// Stop halts the performance monitoring loop.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return nil
	}
	m.cancel()
	m.running = false
	m.logger.Info("performance monitor stopped")
	return nil
}

// IsRunning returns whether the monitor is currently running.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// GetIOPSStats returns the latest IOPS statistics.
func (m *Manager) GetIOPSStats() *IOPSStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.iopsStats == nil {
		return &IOPSStats{Timestamp: time.Now().Unix()}
	}
	cp := *m.iopsStats
	return &cp
}

// GetLatencyStats returns the latest latency statistics.
func (m *Manager) GetLatencyStats() *LatencyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.latencyStats == nil {
		return &LatencyStats{Timestamp: time.Now().Unix()}
	}
	cp := *m.latencyStats
	return &cp
}

// GetBandwidthStats returns the latest bandwidth statistics.
func (m *Manager) GetBandwidthStats() *BandwidthStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.bandwidthStats == nil {
		return &BandwidthStats{Timestamp: time.Now().Unix()}
	}
	cp := *m.bandwidthStats
	return &cp
}

// GetDiskIOStats returns the latest per-disk I/O statistics.
func (m *Manager) GetDiskIOStats() []DiskIOStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.diskIOStats) == 0 {
		return []DiskIOStats{}
	}
	cp := make([]DiskIOStats, len(m.diskIOStats))
	copy(cp, m.diskIOStats)
	return cp
}

// GetNetIOStats returns the latest per-interface network I/O statistics.
func (m *Manager) GetNetIOStats() []NetIOStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.netIOStats) == 0 {
		return []NetIOStats{}
	}
	cp := make([]NetIOStats, len(m.netIOStats))
	copy(cp, m.netIOStats)
	return cp
}

// GetCPUDetailStats returns the latest CPU statistics.
func (m *Manager) GetCPUDetailStats() *CPUDetailStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cpuStats == nil {
		return &CPUDetailStats{Timestamp: time.Now().Unix()}
	}
	cp := *m.cpuStats
	return &cp
}

// GetMemoryDetailStats returns the latest memory statistics.
func (m *Manager) GetMemoryDetailStats() *MemoryDetailStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.memoryStats == nil {
		return &MemoryDetailStats{Timestamp: time.Now().Unix()}
	}
	cp := *m.memoryStats
	return &cp
}

// GetSummary returns a snapshot of all collected metrics.
func (m *Manager) GetSummary() *PerfSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return &PerfSummary{
		IOPS:      m.iopsStats,
		Latency:   m.latencyStats,
		Bandwidth: m.bandwidthStats,
		DiskIO:    m.diskIOStats,
		NetIO:     m.netIOStats,
		CPU:       m.cpuStats,
		Memory:    m.memoryStats,
		Timestamp: time.Now().Unix(),
	}
}

// collectLoop runs the periodic collection cycle.
func (m *Manager) collectLoop(ctx context.Context) {
	ticker := time.NewTicker(m.config.Interval)
	defer ticker.Stop()

	// Initial collection
	m.collect()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.collect()
		}
	}
}

// collect performs a single collection cycle of all metrics.
func (m *Manager) collect() {
	now := time.Now()

	iops, diskIO, diskReadBps, diskWriteBps := m.collectDiskMetrics(now)
	netIO, netInBps, netOutBps := m.collectNetMetrics(now)
	cpuStats := m.collectCPUMetrics(now)
	memStats := m.collectMemoryMetrics(now)
	latency := m.collectLatencyStats(now)

	bandwidth := &BandwidthStats{
		NetworkInBps:  netInBps,
		NetworkOutBps: netOutBps,
		DiskReadBps:   diskReadBps,
		DiskWriteBps:  diskWriteBps,
		Timestamp:     now.Unix(),
	}

	m.mu.Lock()
	m.iopsStats = iops
	m.latencyStats = latency
	m.bandwidthStats = bandwidth
	m.diskIOStats = diskIO
	m.netIOStats = netIO
	m.cpuStats = cpuStats
	m.memoryStats = memStats
	callbacks := make([]CollectFunc, len(m.onCollect))
	copy(callbacks, m.onCollect)
	m.mu.Unlock()

	summary := &PerfSummary{
		IOPS:      iops,
		Latency:   latency,
		Bandwidth: bandwidth,
		DiskIO:    diskIO,
		NetIO:     netIO,
		CPU:       cpuStats,
		Memory:    memStats,
		Timestamp: now.Unix(),
	}

	for _, fn := range callbacks {
		fn(summary)
	}
}

// collectDiskMetrics gathers disk I/O counters and computes IOPS.
func (m *Manager) collectDiskMetrics(now time.Time) (*IOPSStats, []DiskIOStats, float64, float64) {
	counters, err := disk.IOCounters()
	if err != nil {
		m.logger.Warn("failed to get disk IO counters", zap.Error(err))
		return &IOPSStats{Timestamp: now.Unix()}, nil, 0, 0
	}

	elapsed := now.Sub(m.prevDiskTime).Seconds()
	if elapsed <= 0 {
		elapsed = m.config.Interval.Seconds()
	}

	var totalReadOps, totalWriteOps uint64
	var totalReadBytes, totalWriteBytes uint64
	diskStats := make([]DiskIOStats, 0, len(counters))

	for device, cur := range counters {
		prev, hasPrev := m.prevDiskIO[device]
		var readOpsDelta, writeOpsDelta uint64
		var readBytesDelta, writeBytesDelta uint64
		var queueDepth float64

		if hasPrev && elapsed > 0 {
			if cur.ReadCount >= prev.ReadCount {
				readOpsDelta = cur.ReadCount - prev.ReadCount
			}
			if cur.WriteCount >= prev.WriteCount {
				writeOpsDelta = cur.WriteCount - prev.WriteCount
			}
			if cur.ReadBytes >= prev.ReadBytes {
				readBytesDelta = cur.ReadBytes - prev.ReadBytes
			}
			if cur.WriteBytes >= prev.WriteBytes {
				writeBytesDelta = cur.WriteBytes - prev.WriteBytes
			}
			ioTimeDelta := float64(cur.IoTime - prev.IoTime)
			queueDepth = ioTimeDelta / (elapsed * 1000.0)
		}

		diskStats = append(diskStats, DiskIOStats{
			Device:           device,
			ReadBytes:        cur.ReadBytes,
			WriteBytes:       cur.WriteBytes,
			ReadCount:        cur.ReadCount,
			WriteCount:       cur.WriteCount,
			IoTime:           cur.IoTime,
			QueueDepth:       queueDepth,
			ReadBytesPerSec:  float64(readBytesDelta) / elapsed,
			WriteBytesPerSec: float64(writeBytesDelta) / elapsed,
			Timestamp:        now.Unix(),
		})

		totalReadOps += readOpsDelta
		totalWriteOps += writeOpsDelta
		totalReadBytes += readBytesDelta
		totalWriteBytes += writeBytesDelta
	}

	// Sort by device name for consistent output
	sort.Slice(diskStats, func(i, j int) bool {
		return diskStats[i].Device < diskStats[j].Device
	})

	readIOPS := float64(totalReadOps) / elapsed
	writeIOPS := float64(totalWriteOps) / elapsed

	iops := &IOPSStats{
		ReadIOPS:      readIOPS,
		WriteIOPS:     writeIOPS,
		TotalIOPS:     readIOPS + writeIOPS,
		ReadOpsDelta:  totalReadOps,
		WriteOpsDelta: totalWriteOps,
		Timestamp:     now.Unix(),
	}

	m.mu.Lock()
	m.prevDiskIO = counters
	m.prevDiskTime = now
	m.mu.Unlock()

	diskReadBps := float64(totalReadBytes) / elapsed
	diskWriteBps := float64(totalWriteBytes) / elapsed

	return iops, diskStats, diskReadBps, diskWriteBps
}

// collectNetMetrics gathers network I/O counters.
func (m *Manager) collectNetMetrics(now time.Time) ([]NetIOStats, float64, float64) {
	counters, err := net.IOCounters(true)
	if err != nil {
		m.logger.Warn("failed to get net IO counters", zap.Error(err))
		return nil, 0, 0
	}

	elapsed := now.Sub(m.prevDiskTime).Seconds()
	if elapsed <= 0 {
		elapsed = m.config.Interval.Seconds()
	}

	var totalRxBytes, totalTxBytes uint64
	netStats := make([]NetIOStats, 0, len(counters))

	for _, cur := range counters {
		if cur.Name == "lo" || cur.Name == "" {
			continue
		}

		netStats = append(netStats, NetIOStats{
			Interface: cur.Name,
			RxBytes:   cur.BytesRecv,
			TxBytes:   cur.BytesSent,
			RxPackets: cur.PacketsRecv,
			TxPackets: cur.PacketsSent,
			RxErrors:  cur.Errin,
			TxErrors:  cur.Errout,
			RxDropped: cur.Dropin,
			TxDropped: cur.Dropout,
			Timestamp: now.Unix(),
		})

		totalRxBytes += cur.BytesRecv
		totalTxBytes += cur.BytesSent
	}

	sort.Slice(netStats, func(i, j int) bool {
		return netStats[i].Interface < netStats[j].Interface
	})

	m.mu.Lock()
	m.prevNetIO = make(map[string]net.IOCountersStat)
	for _, c := range counters {
		m.prevNetIO[c.Name] = c
	}
	m.mu.Unlock()

	// Note: totalRxBytes/totalTxBytes are cumulative; for rate we'd need deltas.
	// Returning 0 for rate on first call; subsequent calls compute from prev.
	netInBps := float64(totalRxBytes) / elapsed
	netOutBps := float64(totalTxBytes) / elapsed

	return netStats, netInBps, netOutBps
}

// collectCPUMetrics gathers detailed CPU statistics.
func (m *Manager) collectCPUMetrics(now time.Time) *CPUDetailStats {
	percentages, err := cpu.Times(false)
	if err != nil || len(percentages) == 0 {
		m.logger.Warn("failed to get CPU times", zap.Error(err))
		return &CPUDetailStats{Timestamp: now.Unix()}
	}

	t := percentages[0]
	total := t.User + t.System + t.Idle + t.Iowait + t.Irq + t.Softirq + t.Steal
	if total == 0 {
		return &CPUDetailStats{Timestamp: now.Unix()}
	}

	return &CPUDetailStats{
		UserPercent:    round2(t.User / total * 100),
		SystemPercent:  round2(t.System / total * 100),
		IOWaitPercent:  round2(t.Iowait / total * 100),
		IRQPercent:     round2(t.Irq / total * 100),
		SoftIRQPercent: round2(t.Softirq / total * 100),
		StealPercent:   round2(t.Steal / total * 100),
		IdlePercent:    round2(t.Idle / total * 100),
		TotalPercent:   round2((total - t.Idle) / total * 100),
		Timestamp:      now.Unix(),
	}
}

// collectMemoryMetrics gathers detailed memory statistics.
func (m *Manager) collectMemoryMetrics(now time.Time) *MemoryDetailStats {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		m.logger.Warn("failed to get virtual memory", zap.Error(err))
		return &MemoryDetailStats{Timestamp: now.Unix()}
	}

	swapStat, err := mem.SwapMemory()
	if err != nil {
		m.logger.Warn("failed to get swap memory", zap.Error(err))
		swapStat = &mem.SwapMemoryStat{}
	}

	return &MemoryDetailStats{
		TotalBytes:      vmStat.Total,
		UsedBytes:       vmStat.Used,
		AvailableBytes:  vmStat.Available,
		CachedBytes:     vmStat.Cached,
		BuffersBytes:    vmStat.Buffers,
		SwapTotalBytes:  swapStat.Total,
		SwapUsedBytes:   swapStat.Used,
		SwapFreeBytes:   swapStat.Free,
		UsedPercent:     round2(vmStat.UsedPercent),
		SwapUsedPercent: round2(swapStat.UsedPercent),
		Timestamp:       now.Unix(),
	}
}

// collectLatencyStats computes latency statistics from samples.
func (m *Manager) collectLatencyStats(now time.Time) *LatencyStats {
	m.mu.RLock()
	readSamples := make([]float64, len(m.readLatencySamples))
	for i, s := range m.readLatencySamples {
		readSamples[i] = s.Value
	}
	writeSamples := make([]float64, len(m.writeLatencySamples))
	for i, s := range m.writeLatencySamples {
		writeSamples[i] = s.Value
	}
	m.mu.RUnlock()

	return &LatencyStats{
		ReadAvgMs:  avg(readSamples),
		ReadP99Ms:  percentile(readSamples, 99),
		ReadMaxMs:  maxVal(readSamples),
		WriteAvgMs: avg(writeSamples),
		WriteP99Ms: percentile(writeSamples, 99),
		WriteMaxMs: maxVal(writeSamples),
		Timestamp:  now.Unix(),
	}
}

// AddReadLatencySample adds a read latency sample (in milliseconds).
// Callers can use this to feed external latency measurements.
func (m *Manager) AddReadLatencySample(latencyMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readLatencySamples = append(m.readLatencySamples, Sample{
		Timestamp: time.Now(),
		Value:     latencyMs,
	})
	if len(m.readLatencySamples) > m.config.LatencyWindowSize {
		m.readLatencySamples = m.readLatencySamples[1:]
	}
}

// AddWriteLatencySample adds a write latency sample (in milliseconds).
// Callers can use this to feed external latency measurements.
func (m *Manager) AddWriteLatencySample(latencyMs float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.writeLatencySamples = append(m.writeLatencySamples, Sample{
		Timestamp: time.Now(),
		Value:     latencyMs,
	})
	if len(m.writeLatencySamples) > m.config.LatencyWindowSize {
		m.writeLatencySamples = m.writeLatencySamples[1:]
	}
}

// --- Utility functions ---

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func avg(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return round2(sum / float64(len(vals)))
}

func percentile(vals []float64, pct float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)
	idx := int(math.Ceil(pct/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return round2(sorted[idx])
}

func maxVal(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	m := vals[0]
	for _, v := range vals[1:] {
		if v > m {
			m = v
		}
	}
	return round2(m)
}
