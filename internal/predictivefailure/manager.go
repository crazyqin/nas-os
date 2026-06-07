package predictivefailure

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

// Manager 预测性故障分析管理器.
type Manager struct {
	config      Config
	mu          sync.RWMutex
	running     bool
	disks       map[string]*DiskHealthData
	resources   *SystemResourceData
	predictions []*PredictionRecord
	history     map[string]*DiskHistory
	alerts      []*Alert
	stats       PredictionStats
	stopCh      chan struct{}
	doneCh      chan struct{}
}

// NewManager 创建新的预测性故障分析管理器.
func NewManager(config *Config) *Manager {
	cfg := DefaultConfig()
	if config != nil {
		cfg = *config
	}
	return &Manager{
		config:      cfg,
		disks:       make(map[string]*DiskHealthData),
		history:     make(map[string]*DiskHistory),
		predictions: make([]*PredictionRecord, 0),
		alerts:      make([]*Alert, 0),
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return ErrAlreadyRunning
	}
	if !m.config.Enabled {
		return ErrInvalidConfig
	}

	m.running = true
	m.stopCh = make(chan struct{})
	m.doneCh = make(chan struct{})

	go m.scanLoop()
	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return ErrNotRunning
	}

	close(m.stopCh)
	m.running = false
	m.mu.Unlock()

	<-m.doneCh

	m.mu.Lock()
	return nil
}

// scanLoop 定期扫描循环.
func (m *Manager) scanLoop() {
	defer close(m.doneCh)

	interval := time.Duration(m.config.ScanIntervalMinutes) * time.Minute
	if interval < time.Minute {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			_, _ = m.RunFullScan()
		}
	}
}

// ScanDisk 扫描指定磁盘健康状态.
func (m *Manager) ScanDisk(diskID string) (*DiskHealthData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil, ErrNotRunning
	}

	health := simulateDiskHealth(diskID)
	m.disks[diskID] = health

	// 更新历史数据
	m.appendDiskHistory(diskID, health)

	return health, nil
}

// ScanMemory 扫描内存健康状态.
func (m *Manager) ScanMemory() (*SystemResourceData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil, ErrNotRunning
	}

	data := simulateSystemResources()
	m.resources = data

	return data, nil
}

// ScanCPU 扫描 CPU 健康状态.
func (m *Manager) ScanCPU() (*SystemResourceData, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return nil, ErrNotRunning
	}

	data := simulateSystemResources()
	m.resources = data

	return data, nil
}

// RunFullScan 执行完整扫描.
func (m *Manager) RunFullScan() (*ScanResult, error) {
	m.mu.Lock()
	if !m.running {
		m.mu.Unlock()
		return nil, ErrNotRunning
	}
	m.mu.Unlock()

	start := time.Now()

	// 扫描所有已知磁盘
	m.mu.RLock()
	diskIDs := make([]string, 0, len(m.disks))
	for id := range m.disks {
		diskIDs = append(diskIDs, id)
	}
	m.mu.RUnlock()

	// 默认扫描 sda 如果没有已知磁盘
	if len(diskIDs) == 0 {
		diskIDs = []string{"/dev/sda"}
	}

	scanAlerts := make([]Alert, 0)
	diskPredictions := make([]PredictionRecord, 0)

	for _, id := range diskIDs {
		health, err := m.ScanDisk(id)
		if err != nil {
			continue
		}
		pred := m.buildDiskPrediction(id, health)
		diskPredictions = append(diskPredictions, *pred)
		scanAlerts = append(scanAlerts, m.generateAlerts(pred)...)
	}

	// 扫描内存和CPU
	resources, _ := m.ScanMemory()

	var memPred, cpuPred *PredictionRecord
	if resources != nil {
		memPred = m.buildMemoryPrediction(resources)
		cpuPred = m.buildCPUPrediction(resources)
		scanAlerts = append(scanAlerts, m.generateAlerts(memPred)...)
		scanAlerts = append(scanAlerts, m.generateAlerts(cpuPred)...)
	}

	// 计算整体风险
	overallScore := calculateOverallRisk(diskPredictions, memPred, cpuPred)

	scanResult := &ScanResult{
		ID:               fmt.Sprintf("scan-%d", time.Now().UnixMilli()),
		ScanTime:         start,
		Duration:         time.Since(start),
		DiskPredictions:  diskPredictions,
		MemoryPrediction: memPred,
		CPUPrediction:    cpuPred,
		Alerts:           scanAlerts,
		OverallRiskScore: overallScore,
		OverallRiskLevel: ScoreToRiskLevel(overallScore),
	}

	// 存储预测和告警
	m.mu.Lock()
	for i := range diskPredictions {
		m.predictions = append(m.predictions, &diskPredictions[i])
	}
	if memPred != nil {
		m.predictions = append(m.predictions, memPred)
	}
	if cpuPred != nil {
		m.predictions = append(m.predictions, cpuPred)
	}
	for i := range scanAlerts {
		m.alerts = append(m.alerts, &scanAlerts[i])
	}
	m.stats.LastScanTime = start
	m.stats.ScansTotal++
	m.updateStats()
	m.mu.Unlock()

	return scanResult, nil
}

// PredictFailure 预测指定组件的故障.
func (m *Manager) PredictFailure(componentID string) (*PredictionRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running {
		return nil, ErrNotRunning
	}

	// 检查是否是已知磁盘
	if disk, ok := m.disks[componentID]; ok {
		return m.buildDiskPrediction(componentID, disk), nil
	}

	// 检查是否是内存或CPU
	switch componentID {
	case "memory", "cpu":
		if m.resources == nil {
			return nil, fmt.Errorf("no resource data available, run a scan first")
		}
		if componentID == "memory" {
			return m.buildMemoryPrediction(m.resources), nil
		}
		return m.buildCPUPrediction(m.resources), nil
	}

	return nil, ErrPredictionNotFound
}

// GetDashboard 获取仪表盘统计数据.
func (m *Manager) GetDashboard() *PredictionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := m.stats
	stats.CurrentAlerts = 0
	for _, a := range m.alerts {
		if !a.Acknowledged {
			stats.CurrentAlerts++
		}
	}
	return &stats
}

// ListPredictions 列出所有预测记录.
func (m *Manager) ListPredictions() []*PredictionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*PredictionRecord, len(m.predictions))
	copy(result, m.predictions)
	return result
}

// GetMaintenanceSuggestions 获取维护建议.
func (m *Manager) GetMaintenanceSuggestions() []*Recommendation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	suggestions := make([]*Recommendation, 0)

	// 基于磁盘状态生成建议
	for id, disk := range m.disks {
		if disk.ReallocatedSectors > 0 {
			suggestions = append(suggestions, &Recommendation{
				Priority:    1,
				Title:       fmt.Sprintf("磁盘 %s 存在重分配扇区", id),
				Description: fmt.Sprintf("检测到 %d 个重分配扇区，建议备份数据并准备更换磁盘", disk.ReallocatedSectors),
				Action:      "backup_and_replace",
				Urgency:     "immediate",
			})
		}
		if disk.Temperature > m.config.TemperatureWarnThreshold {
			suggestions = append(suggestions, &Recommendation{
				Priority:    2,
				Title:       fmt.Sprintf("磁盘 %s 温度过高", id),
				Description: fmt.Sprintf("当前温度 %.1f°C 超过告警阈值 %.1f°C", disk.Temperature, m.config.TemperatureWarnThreshold),
				Action:      "check_cooling",
				Urgency:     "soon",
			})
		}
		if disk.CurrentPendingSectors > 0 {
			suggestions = append(suggestions, &Recommendation{
				Priority:    2,
				Title:       fmt.Sprintf("磁盘 %s 存在待处理扇区", id),
				Description: fmt.Sprintf("检测到 %d 个待处理扇区", disk.CurrentPendingSectors),
				Action:      "run_extended_test",
				Urgency:     "soon",
			})
		}
	}

	// 基于系统资源生成建议
	if m.resources != nil {
		if m.resources.CPUUsagePercent > m.config.CPUPercentWarnThreshold {
			suggestions = append(suggestions, &Recommendation{
				Priority:    3,
				Title:       "CPU 使用率过高",
				Description: fmt.Sprintf("当前 CPU 使用率 %.1f%% 超过阈值 %.1f%%", m.resources.CPUUsagePercent, m.config.CPUPercentWarnThreshold),
				Action:      "check_processes",
				Urgency:     "soon",
			})
		}
		if m.resources.MemoryUsagePercent > m.config.MemoryPercentWarnThreshold {
			suggestions = append(suggestions, &Recommendation{
				Priority:    3,
				Title:       "内存使用率过高",
				Description: fmt.Sprintf("当前内存使用率 %.1f%% 超过阈值 %.1f%%", m.resources.MemoryUsagePercent, m.config.MemoryPercentWarnThreshold),
				Action:      "check_memory_leak",
				Urgency:     "soon",
			})
		}
	}

	return suggestions
}

// ========== 内部方法 ==========

// appendDiskHistory 追加磁盘历史数据.
func (m *Manager) appendDiskHistory(device string, health *DiskHealthData) {
	h, ok := m.history[device]
	if !ok {
		h = &DiskHistory{Device: device}
		m.history[device] = h
	}

	now := time.Now()
	h.TemperatureHistory = append(h.TemperatureHistory, DataPoint{Timestamp: now, Value: health.Temperature})
	h.ReadErrorRateHistory = append(h.ReadErrorRateHistory, DataPoint{Timestamp: now, Value: health.ReadErrorRate})
	h.ReallocatedSectorHistory = append(h.ReallocatedSectorHistory, DataPoint{Timestamp: now, Value: float64(health.ReallocatedSectors)})
	h.PendingSectorHistory = append(h.PendingSectorHistory, DataPoint{Timestamp: now, Value: float64(health.CurrentPendingSectors)})
}

// buildDiskPrediction 构建磁盘故障预测.
func (m *Manager) buildDiskPrediction(device string, health *DiskHealthData) *PredictionRecord {
	factors := make([]RiskFactor, 0)
	totalWeight := 0.0
	weightedScore := 0.0

	// 重分配扇区因素
	if health.ReallocatedSectors > 0 {
		score := math.Min(float64(health.ReallocatedSectors)*10, 100)
		f := RiskFactor{
			Name:        "reallocated_sectors",
			Description: fmt.Sprintf("重分配扇区数: %d", health.ReallocatedSectors),
			Weight:      0.3,
			Score:       score,
			Trend:       "degrading",
		}
		factors = append(factors, f)
		weightedScore += f.Weight * f.Score
		totalWeight += f.Weight
	}

	// 待处理扇区因素
	if health.CurrentPendingSectors > 0 {
		score := math.Min(float64(health.CurrentPendingSectors)*15, 100)
		f := RiskFactor{
			Name:        "pending_sectors",
			Description: fmt.Sprintf("待处理扇区数: %d", health.CurrentPendingSectors),
			Weight:      0.25,
			Score:       score,
			Trend:       "degrading",
		}
		factors = append(factors, f)
		weightedScore += f.Weight * f.Score
		totalWeight += f.Weight
	}

	// 温度因素
	tempScore := 0.0
	if health.Temperature > m.config.TemperatureCriticalThreshold {
		tempScore = 80 + (health.Temperature-m.config.TemperatureCriticalThreshold)*5
	} else if health.Temperature > m.config.TemperatureWarnThreshold {
		tempScore = 30 + (health.Temperature-m.config.TemperatureWarnThreshold)*10
	}
	if tempScore > 0 {
		f := RiskFactor{
			Name:        "temperature",
			Description: fmt.Sprintf("磁盘温度: %.1f°C", health.Temperature),
			Weight:      0.2,
			Score:       math.Min(tempScore, 100),
			Trend:       "stable",
		}
		factors = append(factors, f)
		weightedScore += f.Weight * f.Score
		totalWeight += f.Weight
	}

	// 读取错误率因素
	if health.ReadErrorRate > 0 {
		score := math.Min(health.ReadErrorRate*20, 100)
		f := RiskFactor{
			Name:        "read_error_rate",
			Description: fmt.Sprintf("读取错误率: %.2f", health.ReadErrorRate),
			Weight:      0.15,
			Score:       score,
			Trend:       "stable",
		}
		factors = append(factors, f)
		weightedScore += f.Weight * f.Score
		totalWeight += f.Weight
	}

	// 通电时长因素
	pohScore := math.Min(float64(health.PowerOnHours)/100, 100)
	if pohScore > 10 {
		f := RiskFactor{
			Name:        "power_on_hours",
			Description: fmt.Sprintf("通电时长: %d 小时", health.PowerOnHours),
			Weight:      0.1,
			Score:       pohScore,
			Trend:       "degrading",
		}
		factors = append(factors, f)
		weightedScore += f.Weight * f.Score
		totalWeight += f.Weight
	}

	// 计算风险评分
	riskScore := 5.0 // 基础风险
	if totalWeight > 0 {
		riskScore = weightedScore / totalWeight
	}

	// 计算故障概率
	failureProb := riskScore / 100.0

	// 估算故障日期
	var predictedDate *time.Time
	if failureProb > 0.3 {
		daysToFailure := int((1.0 - failureProb) * 365)
		if daysToFailure < 7 {
			daysToFailure = 7
		}
		t := time.Now().AddDate(0, 0, daysToFailure)
		predictedDate = &t
	}

	recs := buildDiskRecommendations(health, m.config)

	return &PredictionRecord{
		ID:                   fmt.Sprintf("pred-%s-%d", device, time.Now().UnixMilli()),
		ComponentType:        ComponentDisk,
		ComponentID:          device,
		RiskScore:            riskScore,
		RiskLevel:            ScoreToRiskLevel(riskScore),
		FailureProbability:   failureProb,
		PredictedFailureDate: predictedDate,
		Factors:              factors,
		Recommendations:      recs,
		PredictedAt:          time.Now(),
	}
}

// buildMemoryPrediction 构建内存故障预测.
func (m *Manager) buildMemoryPrediction(resources *SystemResourceData) *PredictionRecord {
	factors := make([]RiskFactor, 0)
	totalWeight := 0.0
	weightedScore := 0.0

	// 内存使用率
	if resources.MemoryUsagePercent > m.config.MemoryPercentWarnThreshold {
		score := (resources.MemoryUsagePercent - m.config.MemoryPercentWarnThreshold) * 5
		f := RiskFactor{
			Name:        "memory_usage",
			Description: fmt.Sprintf("内存使用率: %.1f%%", resources.MemoryUsagePercent),
			Weight:      0.5,
			Score:       math.Min(score, 100),
			Trend:       "stable",
		}
		factors = append(factors, f)
		weightedScore += f.Weight * f.Score
		totalWeight += f.Weight
	}

	// Swap 使用率
	if resources.SwapTotalMB > 0 {
		swapPercent := resources.SwapUsedMB / resources.SwapTotalMB * 100
		if swapPercent > 50 {
			score := (swapPercent - 50) * 2
			f := RiskFactor{
				Name:        "swap_usage",
				Description: fmt.Sprintf("Swap 使用率: %.1f%%", swapPercent),
				Weight:      0.3,
				Score:       math.Min(score, 100),
				Trend:       "stable",
			}
			factors = append(factors, f)
			weightedScore += f.Weight * f.Score
			totalWeight += f.Weight
		}
	}

	riskScore := 5.0
	if totalWeight > 0 {
		riskScore = weightedScore / totalWeight
	}

	return &PredictionRecord{
		ID:                 fmt.Sprintf("pred-memory-%d", time.Now().UnixMilli()),
		ComponentType:      ComponentMemory,
		ComponentID:        "memory",
		RiskScore:          riskScore,
		RiskLevel:          ScoreToRiskLevel(riskScore),
		FailureProbability: riskScore / 100.0,
		Factors:            factors,
		Recommendations:    buildResourceRecommendations("memory", resources, m.config),
		PredictedAt:        time.Now(),
	}
}

// buildCPUPrediction 构建 CPU 故障预测.
func (m *Manager) buildCPUPrediction(resources *SystemResourceData) *PredictionRecord {
	factors := make([]RiskFactor, 0)
	totalWeight := 0.0
	weightedScore := 0.0

	// CPU 使用率
	if resources.CPUUsagePercent > m.config.CPUPercentWarnThreshold {
		score := (resources.CPUUsagePercent - m.config.CPUPercentWarnThreshold) * 5
		f := RiskFactor{
			Name:        "cpu_usage",
			Description: fmt.Sprintf("CPU 使用率: %.1f%%", resources.CPUUsagePercent),
			Weight:      0.4,
			Score:       math.Min(score, 100),
			Trend:       "stable",
		}
		factors = append(factors, f)
		weightedScore += f.Weight * f.Score
		totalWeight += f.Weight
	}

	// CPU 温度
	if resources.CPUTemperature > m.config.TemperatureWarnThreshold {
		score := (resources.CPUTemperature - m.config.TemperatureWarnThreshold) * 5
		f := RiskFactor{
			Name:        "cpu_temperature",
			Description: fmt.Sprintf("CPU 温度: %.1f°C", resources.CPUTemperature),
			Weight:      0.4,
			Score:       math.Min(score, 100),
			Trend:       "stable",
		}
		factors = append(factors, f)
		weightedScore += f.Weight * f.Score
		totalWeight += f.Weight
	}

	// 负载均值
	if resources.LoadAverage1Min > 4.0 {
		score := math.Min((resources.LoadAverage1Min-4.0)*20, 100)
		f := RiskFactor{
			Name:        "load_average",
			Description: fmt.Sprintf("1分钟负载均值: %.2f", resources.LoadAverage1Min),
			Weight:      0.2,
			Score:       score,
			Trend:       "stable",
		}
		factors = append(factors, f)
		weightedScore += f.Weight * f.Score
		totalWeight += f.Weight
	}

	riskScore := 3.0
	if totalWeight > 0 {
		riskScore = weightedScore / totalWeight
	}

	return &PredictionRecord{
		ID:                 fmt.Sprintf("pred-cpu-%d", time.Now().UnixMilli()),
		ComponentType:      ComponentCPU,
		ComponentID:        "cpu",
		RiskScore:          riskScore,
		RiskLevel:          ScoreToRiskLevel(riskScore),
		FailureProbability: riskScore / 100.0,
		Factors:            factors,
		Recommendations:    buildResourceRecommendations("cpu", resources, m.config),
		PredictedAt:        time.Now(),
	}
}

// generateAlerts 根据预测生成告警.
func (m *Manager) generateAlerts(pred *PredictionRecord) []Alert {
	alerts := make([]Alert, 0)
	if pred.RiskScore >= m.config.AlertThreshold {
		alerts = append(alerts, Alert{
			ID:            fmt.Sprintf("alert-%s-%d", pred.ComponentID, time.Now().UnixMilli()),
			ComponentType: pred.ComponentType,
			ComponentID:   pred.ComponentID,
			Level:         pred.RiskLevel,
			Title:         fmt.Sprintf("%s 风险告警", pred.ComponentID),
			Message:       fmt.Sprintf("风险评分 %.1f, 等级: %s", pred.RiskScore, pred.RiskLevel),
			RiskScore:     pred.RiskScore,
			CreatedAt:     time.Now(),
		})
	}
	return alerts
}

// updateStats 更新统计数据（调用时需持有写锁）.
func (m *Manager) updateStats() {
	m.stats.TotalPredictions = len(m.predictions)
	m.stats.CriticalDisks = 0
	totalScore := 0.0
	for _, p := range m.predictions {
		totalScore += p.RiskScore
		if p.ComponentType == ComponentDisk && p.RiskLevel == RiskCritical {
			m.stats.CriticalDisks++
		}
	}
	if m.stats.TotalPredictions > 0 {
		m.stats.AverageRiskScore = totalScore / float64(m.stats.TotalPredictions)
	}
}

// ========== 辅助函数 ==========

func buildDiskRecommendations(health *DiskHealthData, config Config) []Recommendation {
	recs := make([]Recommendation, 0)
	if health.ReallocatedSectors > 0 {
		recs = append(recs, Recommendation{
			Priority:    1,
			Title:       "立即备份数据",
			Description: fmt.Sprintf("检测到 %d 个重分配扇区，磁盘可能出现物理损坏", health.ReallocatedSectors),
			Action:      "backup_data",
			Urgency:     "immediate",
		})
	}
	if health.Temperature > config.TemperatureCriticalThreshold {
		recs = append(recs, Recommendation{
			Priority:    1,
			Title:       "改善散热",
			Description: fmt.Sprintf("磁盘温度 %.1f°C 已超过严重阈值 %.1f°C", health.Temperature, config.TemperatureCriticalThreshold),
			Action:      "improve_cooling",
			Urgency:     "immediate",
		})
	} else if health.Temperature > config.TemperatureWarnThreshold {
		recs = append(recs, Recommendation{
			Priority:    2,
			Title:       "检查散热系统",
			Description: fmt.Sprintf("磁盘温度 %.1f°C 超过告警阈值", health.Temperature),
			Action:      "check_cooling",
			Urgency:     "soon",
		})
	}
	if health.CurrentPendingSectors > 0 {
		recs = append(recs, Recommendation{
			Priority:    2,
			Title:       "运行扩展自检",
			Description: "存在待处理扇区，建议运行磁盘扩展自检",
			Action:      "run_extended_selftest",
			Urgency:     "soon",
		})
	}
	return recs
}

func buildResourceRecommendations(component string, resources *SystemResourceData, config Config) []Recommendation {
	recs := make([]Recommendation, 0)
	switch component {
	case "memory":
		if resources.MemoryUsagePercent > config.MemoryPercentWarnThreshold {
			recs = append(recs, Recommendation{
				Priority:    2,
				Title:       "检查内存使用",
				Description: fmt.Sprintf("内存使用率 %.1f%% 偏高", resources.MemoryUsagePercent),
				Action:      "check_memory_usage",
				Urgency:     "soon",
			})
		}
	case "cpu":
		if resources.CPUUsagePercent > config.CPUPercentWarnThreshold {
			recs = append(recs, Recommendation{
				Priority:    2,
				Title:       "检查 CPU 负载",
				Description: fmt.Sprintf("CPU 使用率 %.1f%% 偏高", resources.CPUUsagePercent),
				Action:      "check_cpu_load",
				Urgency:     "soon",
			})
		}
	}
	return recs
}

func calculateOverallRisk(disks []PredictionRecord, mem, cpu *PredictionRecord) float64 {
	scores := make([]float64, 0)
	for _, d := range disks {
		scores = append(scores, d.RiskScore)
	}
	if mem != nil {
		scores = append(scores, mem.RiskScore)
	}
	if cpu != nil {
		scores = append(scores, cpu.RiskScore)
	}
	if len(scores) == 0 {
		return 0
	}
	// 整体风险取最高分的 60% + 平均分的 40%
	maxScore := 0.0
	sum := 0.0
	for _, s := range scores {
		if s > maxScore {
			maxScore = s
		}
		sum += s
	}
	avg := sum / float64(len(scores))
	return maxScore*0.6 + avg*0.4
}

// ========== 模拟数据生成 ==========

func simulateDiskHealth(device string) *DiskHealthData {
	now := time.Now()
	r := rand.New(rand.NewSource(now.UnixNano() + int64(len(device))))

	reallocated := int64(0)
	if r.Float64() < 0.1 {
		reallocated = int64(r.Intn(50) + 1)
	}

	pending := int64(0)
	if r.Float64() < 0.08 {
		pending = int64(r.Intn(20) + 1)
	}

	return &DiskHealthData{
		Device:                device,
		Model:                 fmt.Sprintf("Simulated-Disk-%s", device[len(device)-1:]),
		Serial:                fmt.Sprintf("SIM%08d", r.Intn(100000000)),
		CapacityGB:            float64(r.Intn(8000) + 1000),
		Temperature:           25 + r.Float64()*40,
		PowerOnHours:          int64(r.Intn(50000)),
		ReallocatedSectors:    reallocated,
		CurrentPendingSectors: pending,
		OfflineUncorrectable:  int64(r.Intn(3)),
		UDMAErrors:            int64(r.Intn(10)),
		ReadErrorRate:         r.Float64() * 5,
		SeekErrorRate:         r.Float64() * 3,
		SpinRetryCount:        int64(r.Intn(2)),
		HealthStatus:          "OK",
		CollectedAt:           now,
	}
}

func simulateSystemResources() *SystemResourceData {
	now := time.Now()
	r := rand.New(rand.NewSource(now.UnixNano()))

	totalMem := float64(16384)
	usedMem := totalMem * (0.3 + r.Float64()*0.5)

	return &SystemResourceData{
		CPUUsagePercent:    10 + r.Float64()*70,
		CPUTemperature:     35 + r.Float64()*40,
		MemoryTotalMB:      totalMem,
		MemoryUsedMB:       usedMem,
		MemoryUsagePercent: usedMem / totalMem * 100,
		SwapTotalMB:        4096,
		SwapUsedMB:         r.Float64() * 1000,
		LoadAverage1Min:    r.Float64() * 8,
		LoadAverage5Min:    r.Float64() * 6,
		LoadAverage15Min:   r.Float64() * 5,
		CollectedAt:        now,
	}
}
