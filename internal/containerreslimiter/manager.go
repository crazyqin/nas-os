// Package containerreslimiter 提供智能容器资源限制管理
// 基于历史使用模式自动调整 CPU/内存限制，避免资源浪费并确保公平分配
package containerreslimiter

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"time"
)

var (
	ErrContainerNotFound = errors.New("container not found")
	ErrInvalidLimit      = errors.New("invalid resource limit")
	ErrInsufficientData  = errors.New("insufficient usage data")
)

// ResourceLimits 资源限制定义
type ResourceLimits struct {
	CPUMilliCores int64  `json:"cpuMilliCores"` // CPU限制（毫核）
	MemoryBytes   int64  `json:"memoryBytes"`   // 内存限制（字节）
	IOReadBPS     int64  `json:"ioReadBps"`     // IO读取速率限制
	IOWriteBPS    int64  `json:"ioWriteBps"`    // IO写入速率限制
	NetUploadBPS  int64  `json:"netUploadBps"`  // 网络上传速率限制
	NetDownloadBPS int64 `json:"netDownloadBps"` // 网络下载速率限制
}

// UsageSample 使用量采样
type UsageSample struct {
	Timestamp      time.Time `json:"timestamp"`
	CPUMilliCores  float64   `json:"cpuMilliCores"`
	MemoryBytes    int64     `json:"memoryBytes"`
	IOReadBytesPS  int64     `json:"ioReadBytesPs"`
	IOWriteBytesPS int64     `json:"ioWriteBytesPs"`
	NetRxBytesPS   int64     `json:"netRxBytesPs"`
	NetTxBytesPS   int64     `json:"netTxBytesPs"`
}

// ContainerProfile 容器资源画像
type ContainerProfile struct {
	ContainerID    string         `json:"containerId"`
	ContainerName  string         `json:"containerName"`
	ImageName      string         `json:"imageName"`
	CurrentLimits  ResourceLimits `json:"currentLimits"`
	RecommendedLimits ResourceLimits `json:"recommendedLimits"`
	Samples        []UsageSample  `json:"-"`
	LastUpdated    time.Time      `json:"lastUpdated"`
	Strategy       LimitStrategy  `json:"strategy"`
}

// LimitStrategy 限制策略
type LimitStrategy string

const (
	StrategyConservative LimitStrategy = "conservative" // 保守策略：限制为峰值的150%
	StrategyBalanced     LimitStrategy = "balanced"     // 平衡策略：限制为P95的120%
	StrategyAggressive   LimitStrategy = "aggressive"   // 激进策略：限制为P90的110%
	StrategyManual       LimitStrategy = "manual"       // 手动策略：不自动调整
)

// PercentileResult 百分位数结果
type PercentileResult struct {
	P50  float64 `json:"p50"`
	P90  float64 `json:"p90"`
	P95  float64 `json:"p95"`
	P99  float64 `json:"p99"`
	Max  float64 `json:"max"`
	Mean float64 `json:"mean"`
}

// ResourceAnalysis 资源分析结果
type ResourceAnalysis struct {
	ContainerID       string           `json:"containerId"`
	ContainerName     string           `json:"containerName"`
	CPUPercentile     PercentileResult `json:"cpuPercentile"`
	MemoryPercentile  PercentileResult `json:"memoryPercentile"`
	IOReadPercentile  PercentileResult `json:"ioReadPercentile"`
	IOWritePercentile PercentileResult `json:"ioWritePercentile"`
	AnalysisPeriod    time.Duration    `json:"analysisPeriod"`
	SampleCount       int              `json:"sampleCount"`
	Recommendation    string           `json:"recommendation"`
}

// AdjustmentRecord 调整记录
type AdjustmentRecord struct {
	Timestamp    time.Time     `json:"timestamp"`
	ContainerID  string        `json:"containerId"`
	OldLimits    ResourceLimits `json:"oldLimits"`
	NewLimits    ResourceLimits `json:"newLimits"`
	Reason       string        `json:"reason"`
	Strategy     LimitStrategy `json:"strategy"`
	Applied      bool          `json:"applied"`
}

// Manager 智能容器资源限制管理器
type Manager struct {
	mu           sync.RWMutex
	config       *Config
	containers   map[string]*ContainerProfile
	adjustments  []AdjustmentRecord
	running      bool
	stopCh       chan struct{}
	nowFunc      func() time.Time
}

// Config 配置
type Config struct {
	Enabled             bool          `json:"enabled"`
	DefaultStrategy     LimitStrategy `json:"defaultStrategy"`
	SampleInterval      time.Duration `json:"sampleInterval"`
	MinSamplesRequired  int           `json:"minSamplesRequired"`
	AnalysisWindow      time.Duration `json:"analysisWindow"`
	MaxAdjustmentPerDay int           `json:"maxAdjustmentPerDay"`
	CPUBufferPercent    float64       `json:"cpuBufferPercent"`
	MemoryBufferPercent float64       `json:"memoryBufferPercent"`
	AutoApply           bool          `json:"autoApply"`
}

// NewManager 创建管理器
func NewManager(config *Config) *Manager {
	if config == nil {
		config = &Config{
			Enabled:             true,
			DefaultStrategy:     StrategyBalanced,
			SampleInterval:      time.Minute * 5,
			MinSamplesRequired:  20,
			AnalysisWindow:      time.Hour * 24 * 7, // 7天
			MaxAdjustmentPerDay: 3,
			CPUBufferPercent:    20,
			MemoryBufferPercent: 15,
			AutoApply:           false,
		}
	}
	return &Manager{
		config:      config,
		containers:  make(map[string]*ContainerProfile),
		adjustments: make([]AdjustmentRecord, 0),
		stopCh:      make(chan struct{}),
		nowFunc:     time.Now,
	}
}

// RegisterContainer 注册容器
func (m *Manager) RegisterContainer(id, name, image string, limits ResourceLimits) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "" {
		return fmt.Errorf("container id is required")
	}

	m.containers[id] = &ContainerProfile{
		ContainerID:       id,
		ContainerName:     name,
		ImageName:         image,
		CurrentLimits:     limits,
		RecommendedLimits: limits,
		Samples:           make([]UsageSample, 0),
		LastUpdated:       m.nowFunc(),
		Strategy:          m.config.DefaultStrategy,
	}
	return nil
}

// RecordUsage 记录使用量
func (m *Manager) RecordUsage(containerID string, sample UsageSample) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, ok := m.containers[containerID]
	if !ok {
		return ErrContainerNotFound
	}

	profile.Samples = append(profile.Samples, sample)
	profile.LastUpdated = m.nowFunc()

	// 保留最近的采样数据
	maxSamples := int(m.config.AnalysisWindow / m.config.SampleInterval)
	if maxSamples > 0 && len(profile.Samples) > maxSamples {
		profile.Samples = profile.Samples[len(profile.Samples)-maxSamples:]
	}

	return nil
}

// AnalyzeContainer 分析容器资源使用
func (m *Manager) AnalyzeContainer(containerID string) (*ResourceAnalysis, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, ok := m.containers[containerID]
	if !ok {
		return nil, ErrContainerNotFound
	}

	if len(profile.Samples) < m.config.MinSamplesRequired {
		return nil, ErrInsufficientData
	}

	// 提取各指标数据
	cpuValues := make([]float64, len(profile.Samples))
	memValues := make([]float64, len(profile.Samples))
	ioReadValues := make([]float64, len(profile.Samples))
	ioWriteValues := make([]float64, len(profile.Samples))

	for i, s := range profile.Samples {
		cpuValues[i] = s.CPUMilliCores
		memValues[i] = float64(s.MemoryBytes)
		ioReadValues[i] = float64(s.IOReadBytesPS)
		ioWriteValues[i] = float64(s.IOWriteBytesPS)
	}

	analysis := &ResourceAnalysis{
		ContainerID:       containerID,
		ContainerName:     profile.ContainerName,
		CPUPercentile:     calculatePercentiles(cpuValues),
		MemoryPercentile:  calculatePercentiles(memValues),
		IOReadPercentile:  calculatePercentiles(ioReadValues),
		IOWritePercentile: calculatePercentiles(ioWriteValues),
		AnalysisPeriod:    m.config.AnalysisWindow,
		SampleCount:       len(profile.Samples),
	}

	// 生成建议
	analysis.Recommendation = m.generateRecommendation(profile, analysis)

	return analysis, nil
}

// CalculateRecommendedLimits 计算推荐限制
func (m *Manager) CalculateRecommendedLimits(containerID string) (*ResourceLimits, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, ok := m.containers[containerID]
	if !ok {
		return nil, ErrContainerNotFound
	}

	if len(profile.Samples) < m.config.MinSamplesRequired {
		return nil, ErrInsufficientData
	}

	// 提取数据
	cpuValues := make([]float64, len(profile.Samples))
	memValues := make([]float64, len(profile.Samples))

	for i, s := range profile.Samples {
		cpuValues[i] = s.CPUMilliCores
		memValues[i] = float64(s.MemoryBytes)
	}

	limits := m.calculateLimitsByStrategy(profile.Strategy, cpuValues, memValues)

	// 更新推荐值
	profile.RecommendedLimits = *limits

	return limits, nil
}

// AutoAdjust 自动调整所有容器
func (m *Manager) AutoAdjust() ([]AdjustmentRecord, error) {
	if !m.config.AutoApply {
		return nil, fmt.Errorf("auto-apply is disabled")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	results := make([]AdjustmentRecord, 0)

	for id, profile := range m.containers {
		if profile.Strategy == StrategyManual {
			continue
		}

		if len(profile.Samples) < m.config.MinSamplesRequired {
			continue
		}

		// 检查今日调整次数
		today := m.nowFunc().Format("2006-01-02")
		adjustCount := 0
		for _, adj := range m.adjustments {
			if adj.ContainerID == id && adj.Timestamp.Format("2006-01-02") == today {
				adjustCount++
			}
		}
		if adjustCount >= m.config.MaxAdjustmentPerDay {
			continue
		}

		// 计算新限制
		cpuValues := make([]float64, len(profile.Samples))
		memValues := make([]float64, len(profile.Samples))
		for i, s := range profile.Samples {
			cpuValues[i] = s.CPUMilliCores
			memValues[i] = float64(s.MemoryBytes)
		}

		newLimits := m.calculateLimitsByStrategy(profile.Strategy, cpuValues, memValues)

		// 检查是否有显著变化（超过10%）
		if !m.isSignificantChange(profile.CurrentLimits, *newLimits) {
			continue
		}

		record := AdjustmentRecord{
			Timestamp:   m.nowFunc(),
			ContainerID: id,
			OldLimits:   profile.CurrentLimits,
			NewLimits:   *newLimits,
			Reason:      fmt.Sprintf("Auto-adjusted based on %s strategy", profile.Strategy),
			Strategy:    profile.Strategy,
			Applied:     true,
		}

		profile.CurrentLimits = *newLimits
		m.adjustments = append(m.adjustments, record)
		results = append(results, record)
	}

	return results, nil
}

// GetContainer 获取容器信息
func (m *Manager) GetContainer(containerID string) (*ContainerProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, ok := m.containers[containerID]
	if !ok {
		return nil, ErrContainerNotFound
	}

	// 返回副本，不暴露采样数据
	copy := *profile
	copy.Samples = nil
	return &copy, nil
}

// GetAllContainers 获取所有容器
func (m *Manager) GetAllContainers() []*ContainerProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ContainerProfile, 0, len(m.containers))
	for _, p := range m.containers {
		copy := *p
		copy.Samples = nil
		result = append(result, &copy)
	}
	return result
}

// SetStrategy 设置容器策略
func (m *Manager) SetStrategy(containerID string, strategy LimitStrategy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, ok := m.containers[containerID]
	if !ok {
		return ErrContainerNotFound
	}

	profile.Strategy = strategy
	return nil
}

// GetAdjustmentHistory 获取调整历史
func (m *Manager) GetAdjustmentHistory(containerID string, limit int) []AdjustmentRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]AdjustmentRecord, 0)
	for i := len(m.adjustments) - 1; i >= 0; i-- {
		if containerID != "" && m.adjustments[i].ContainerID != containerID {
			continue
		}
		result = append(result, m.adjustments[i])
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// GetDashboard 获取仪表板数据
func (m *Manager) GetDashboard() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalContainers := len(m.containers)
	autoManaged := 0
	overProvisioned := 0
	underProvisioned := 0

	for _, p := range m.containers {
		if p.Strategy != StrategyManual {
			autoManaged++
		}
		// 简单判断：当前限制 > 推荐限制的150%视为过度配置
		if p.CurrentLimits.CPUMilliCores > 0 && p.RecommendedLimits.CPUMilliCores > 0 {
			ratio := float64(p.CurrentLimits.CPUMilliCores) / float64(p.RecommendedLimits.CPUMilliCores)
			if ratio > 1.5 {
				overProvisioned++
			} else if ratio < 0.8 {
				underProvisioned++
			}
		}
	}

	return map[string]interface{}{
		"totalContainers":   totalContainers,
		"autoManaged":       autoManaged,
		"overProvisioned":   overProvisioned,
		"underProvisioned":  underProvisioned,
		"totalAdjustments":  len(m.adjustments),
		"defaultStrategy":   string(m.config.DefaultStrategy),
		"autoApply":         m.config.AutoApply,
	}
}

// 内部方法

func (m *Manager) calculateLimitsByStrategy(strategy LimitStrategy, cpuValues, memValues []float64) *ResourceLimits {
	cpuPercentiles := calculatePercentiles(cpuValues)
	memPercentiles := calculatePercentiles(memValues)

	var cpuLimit, memLimit float64

	switch strategy {
	case StrategyConservative:
		cpuLimit = cpuPercentiles.Max * 1.5
		memLimit = memPercentiles.Max * 1.5
	case StrategyBalanced:
		cpuLimit = cpuPercentiles.P95 * 1.2
		memLimit = memPercentiles.P95 * 1.2
	case StrategyAggressive:
		cpuLimit = cpuPercentiles.P90 * 1.1
		memLimit = memPercentiles.P90 * 1.1
	default:
		cpuLimit = cpuPercentiles.P95 * 1.2
		memLimit = memPercentiles.P95 * 1.2
	}

	// 应用缓冲区
	cpuLimit *= (1 + m.config.CPUBufferPercent/100)
	memLimit *= (1 + m.config.MemoryBufferPercent/100)

	// 最小值保证
	if cpuLimit < 100 { // 最少100毫核
		cpuLimit = 100
	}
	if memLimit < 64*1024*1024 { // 最少64MB
		memLimit = 64 * 1024 * 1024
	}

	return &ResourceLimits{
		CPUMilliCores: int64(math.Ceil(cpuLimit)),
		MemoryBytes:   int64(math.Ceil(memLimit)),
	}
}

func (m *Manager) isSignificantChange(old, new ResourceLimits) bool {
	cpuChange := math.Abs(float64(new.CPUMilliCores-old.CPUMilliCores)) / float64(old.CPUMilliCores)
	memChange := math.Abs(float64(new.MemoryBytes-old.MemoryBytes)) / float64(old.MemoryBytes)
	return cpuChange > 0.1 || memChange > 0.1
}

func (m *Manager) generateRecommendation(profile *ContainerProfile, analysis *ResourceAnalysis) string {
	cpuUtil := 0.0
	if profile.CurrentLimits.CPUMilliCores > 0 {
		cpuUtil = analysis.CPUPercentile.P95 / float64(profile.CurrentLimits.CPUMilliCores) * 100
	}
	memUtil := 0.0
	if profile.CurrentLimits.MemoryBytes > 0 {
		memUtil = analysis.MemoryPercentile.P95 / float64(profile.CurrentLimits.MemoryBytes) * 100
	}

	if cpuUtil < 30 && memUtil < 30 {
		return "资源严重过度配置，建议降低限制以节省资源"
	}
	if cpuUtil < 50 && memUtil < 50 {
		return "资源过度配置，可适当降低限制"
	}
	if cpuUtil > 90 || memUtil > 90 {
		return "资源接近上限，建议增加限制以避免性能问题"
	}
	if cpuUtil > 80 || memUtil > 80 {
		return "资源使用较高，建议关注"
	}
	return "资源配置合理"
}

func calculatePercentiles(values []float64) PercentileResult {
	if len(values) == 0 {
		return PercentileResult{}
	}

	// 排序
	sorted := make([]float64, len(values))
	copy(sorted, values)
	sortFloat64s(sorted)

	sum := 0.0
	for _, v := range sorted {
		sum += v
	}

	return PercentileResult{
		P50:  percentile(sorted, 50),
		P90:  percentile(sorted, 90),
		P95:  percentile(sorted, 95),
		P99:  percentile(sorted, 99),
		Max:  sorted[len(sorted)-1],
		Mean: sum / float64(len(sorted)),
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	index := (p / 100) * float64(len(sorted)-1)
	lower := int(math.Floor(index))
	upper := int(math.Ceil(index))
	if lower == upper {
		return sorted[lower]
	}
	fraction := index - float64(lower)
	return sorted[lower]*(1-fraction) + sorted[upper]*fraction
}

func sortFloat64s(data []float64) {
	n := len(data)
	for i := 1; i < n; i++ {
		key := data[i]
		j := i - 1
		for j >= 0 && data[j] > key {
			data[j+1] = data[j]
			j--
		}
		data[j+1] = key
	}
}
