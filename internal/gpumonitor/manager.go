package gpumonitor

import (
	"log"
	"sort"
	"sync"
	"time"
)

// Monitor 是 Manager 的别名，用于向后兼容.
type Monitor = Manager

// Manager GPU 监控管理器.
type Manager struct {
	mu      sync.RWMutex
	config  GPUConfig
	gpus    map[string]*GPU
	metrics []GPUMetrics
	alerts  []GPUAlert
	running bool
	stopCh  chan struct{}
}

// NewManager 创建管理器.
func NewManager(cfg GPUConfig) *Manager {
	if cfg.MonitorInterval == 0 {
		cfg.MonitorInterval = 10 * time.Second
	}
	if cfg.TempWarning == 0 {
		cfg.TempWarning = 80
	}
	if cfg.TempCritical == 0 {
		cfg.TempCritical = 90
	}
	if cfg.PowerWarning == 0 {
		cfg.PowerWarning = 90
	}
	if cfg.VRAMWarning == 0 {
		cfg.VRAMWarning = 80
	}
	if cfg.RetentionDays == 0 {
		cfg.RetentionDays = 30
	}
	return &Manager{
		config: cfg,
		gpus:   make(map[string]*GPU),
		stopCh: make(chan struct{}),
	}
}

// Start 启动监控.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	m.stopCh = make(chan struct{})
	go m.monitorLoop()
	log.Println("[GPUMonitor] GPU 监控已启动")
	return nil
}

// Stop 停止监控.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	log.Println("[GPUMonitor] GPU 监控已停止")
}

// RegisterGPU 注册 GPU.
func (m *Manager) RegisterGPU(gpu *GPU) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	gpu.LastUpdated = time.Now()
	gpu.State = StateIdle
	m.gpus[gpu.ID] = gpu
	return nil
}

// UnregisterGPU 注销 GPU.
func (m *Manager) UnregisterGPU(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.gpus[id]; !ok {
		return ErrGPUNotFound
	}
	delete(m.gpus, id)
	return nil
}

// GetGPU 获取 GPU 信息.
func (m *Manager) GetGPU(id string) (*GPU, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	gpu, ok := m.gpus[id]
	if !ok {
		return nil, ErrGPUNotFound
	}
	return gpu, nil
}

// ListGPUs 列出所有 GPU.
func (m *Manager) ListGPUs() []*GPU {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []*GPU
	for _, g := range m.gpus {
		result = append(result, g)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// UpdateGPU 更新 GPU 状态.
func (m *Manager) UpdateGPU(id string, update *GPU) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	gpu, ok := m.gpus[id]
	if !ok {
		return ErrGPUNotFound
	}
	if update.Temperature > 0 {
		gpu.Temperature = update.Temperature
	}
	if update.VRAMUsed > 0 {
		gpu.VRAMUsed = update.VRAMUsed
	}
	if update.UtilizationGPU >= 0 {
		gpu.UtilizationGPU = update.UtilizationGPU
	}
	if update.PowerDraw > 0 {
		gpu.PowerDraw = update.PowerDraw
	}
	if update.FanSpeed >= 0 {
		gpu.FanSpeed = update.FanSpeed
	}
	gpu.LastUpdated = time.Now()
	return nil
}

// GetMetrics 获取历史指标.
func (m *Manager) GetMetrics(gpuID string, since time.Time) []GPUMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []GPUMetrics
	for _, met := range m.metrics {
		if met.GPUID == gpuID && met.Timestamp.After(since) {
			result = append(result, met)
		}
	}
	return result
}

// GetAlerts 获取告警.
func (m *Manager) GetAlerts(gpuID string, limit int) []GPUAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []GPUAlert
	for i := len(m.alerts) - 1; i >= 0; i-- {
		alert := m.alerts[i]
		if gpuID != "" && alert.GPUID != gpuID {
			continue
		}
		result = append(result, alert)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// GetStats 获取统计.
func (m *Manager) GetStats() GPUStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	stats := GPUStats{
		VendorDist: make(map[string]int),
	}
	var totalTemp, totalUtil float64
	for _, g := range m.gpus {
		stats.TotalGPUs++
		stats.TotalVRAM += g.VRAMTotal
		stats.UsedVRAM += g.VRAMUsed
		stats.TotalPower += g.PowerDraw
		stats.VendorDist[string(g.Vendor)]++
		if g.State != StateOffline {
			stats.OnlineGPUs++
			totalTemp += g.Temperature
			totalUtil += float64(g.UtilizationGPU)
		}
	}
	if stats.OnlineGPUs > 0 {
		stats.AvgTemp = totalTemp / float64(stats.OnlineGPUs)
		stats.AvgUtil = totalUtil / float64(stats.OnlineGPUs)
	}
	return stats
}

// GetTopProcesses 获取显存占用最多的进程.
func (m *Manager) GetTopProcesses(limit int) []GPUProcess {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var all []GPUProcess
	for _, g := range m.gpus {
		all = append(all, g.Processes...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].VRAMUsed > all[j].VRAMUsed
	})
	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all
}

func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(m.config.MonitorInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.collectMetrics()
			m.checkAlerts()
			m.cleanup()
		}
	}
}

func (m *Manager) collectMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for _, g := range m.gpus {
		if g.State == StateOffline {
			continue
		}
		m.metrics = append(m.metrics, GPUMetrics{
			GPUID:     g.ID,
			Timestamp: now,
			Temp:      g.Temperature,
			Power:     g.PowerDraw,
			VRAMUsed:  g.VRAMUsed,
			GPUUtil:   g.UtilizationGPU,
		})
	}
}

func (m *Manager) checkAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for _, g := range m.gpus {
		if g.State == StateOffline {
			continue
		}
		// 温度告警
		if g.Temperature >= m.config.TempCritical {
			m.alerts = append(m.alerts, GPUAlert{
				GPUID: g.ID, Level: AlertCritical,
				Message: "GPU 温度严重过高", Value: g.Temperature, Threshold: m.config.TempCritical, Timestamp: now,
			})
		} else if g.Temperature >= m.config.TempWarning {
			m.alerts = append(m.alerts, GPUAlert{
				GPUID: g.ID, Level: AlertWarning,
				Message: "GPU 温度过高", Value: g.Temperature, Threshold: m.config.TempWarning, Timestamp: now,
			})
		}
		// 显存告警
		if g.VRAMTotal > 0 {
			vramPercent := float64(g.VRAMUsed) / float64(g.VRAMTotal) * 100
			if vramPercent >= m.config.VRAMWarning {
				m.alerts = append(m.alerts, GPUAlert{
					GPUID: g.ID, Level: AlertWarning,
					Message: "显存使用率过高", Value: vramPercent, Threshold: m.config.VRAMWarning, Timestamp: now,
				})
			}
		}
		// 功耗告警
		if g.PowerLimit > 0 {
			powerPercent := g.PowerDraw / g.PowerLimit * 100
			if powerPercent >= m.config.PowerWarning {
				m.alerts = append(m.alerts, GPUAlert{
					GPUID: g.ID, Level: AlertWarning,
					Message: "GPU 功耗接近限制", Value: powerPercent, Threshold: m.config.PowerWarning, Timestamp: now,
				})
			}
		}
	}
}

func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()
	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)
	start := 0
	for start < len(m.metrics) && m.metrics[start].Timestamp.Before(cutoff) {
		start++
	}
	if start > 0 {
		m.metrics = m.metrics[start:]
	}
	alertCutoff := time.Now().Add(-24 * time.Hour)
	start = 0
	for start < len(m.alerts) && m.alerts[start].Timestamp.Before(alertCutoff) {
		start++
	}
	if start > 0 {
		m.alerts = m.alerts[start:]
	}
}
