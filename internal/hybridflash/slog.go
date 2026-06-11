// Package hybridflash 提供 SSD/HDD 智能混合分层存储管理.
//
// SLOGManager: SLOG/ZIL 管理器，优化同步写入性能.
// MetadataOptimizer: 元数据优化器，小文件优先 flash.
// CostAnalyzer: 成本分析器，对比不同分层方案的成本效益.
package hybridflash

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// SLOGManager SLOG/ZIL 管理器.
//
// 管理 ZFS 意图日志（ZIL）和独立日志设备（SLOG），
// 优化同步写入性能，降低写延迟.
type SLOGManager struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	devices      map[string]*SLOGDevice
	config       *SLOGConfig
	writeQueue   chan *SLOGWrite
	stats        *SLOGStats
}

// SLOGDevice SLOG 设备.
type SLOGDevice struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Path         string    `json:"path"`
	Type         FlashType `json:"type"`
	Capacity     int64     `json:"capacity"`     // 容量 (字节)
	Used         int64     `json:"used"`         // 已用 (字节)
	WriteLatency float64   `json:"writeLatency"` // 写延迟 (μs)
	IOPS         int64     `json:"iops"`         // 写 IOPS
	Health       float64   `json:"health"`       // 健康度 (0-100)
	Enabled      bool      `json:"enabled"`
	Role         FlashRole `json:"role"`         // zil 或 slog
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// SLOGConfig SLOG 配置.
type SLOGConfig struct {
	// Enabled 启用 SLOG 管理.
	Enabled bool `json:"enabled"`
	// MaxWriteQueueSize 最大写入队列大小.
	MaxWriteQueueSize int `json:"maxWriteQueueSize"`
	// SyncMode 同步模式: always/standard/disabled.
	SyncMode string `json:"syncMode"`
	// FlushInterval 刷新间隔.
	FlushInterval time.Duration `json:"flushInterval"`
	// EnableRedundancy 启用 SLOG 冗余.
	EnableRedundancy bool `json:"enableRedundancy"`
	// MinDevices 最小设备数（冗余模式）.
	MinDevices int `json:"minDevices"`
	// WriteBufferSize 写缓冲大小 (字节).
	WriteBufferSize int64 `json:"writeBufferSize"`
	// EnableCompression 启用 SLOG 压缩.
	EnableCompression bool `json:"enableCompression"`
}

// DefaultSLOGConfig 默认 SLOG 配置.
func DefaultSLOGConfig() *SLOGConfig {
	return &SLOGConfig{
		Enabled:           true,
		MaxWriteQueueSize: 10000,
		SyncMode:          "standard",
		FlushInterval:     5 * time.Second,
		EnableRedundancy:  true,
		MinDevices:        2,
		WriteBufferSize:   64 * 1024 * 1024, // 64MB
		EnableCompression: true,
	}
}

// SLOGWrite SLOG 写入请求.
type SLOGWrite struct {
	ID        string    `json:"id"`
	PoolID    string    `json:"poolId"`
	Offset    int64     `json:"offset"`
	Size      int64     `json:"size"`
	Data      []byte    `json:"data,omitempty"` // 不序列化实际数据
	SyncMode  string    `json:"syncMode"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
}

// SLOGStats SLOG 统计.
type SLOGStats struct {
	mu              sync.RWMutex
	TotalWrites     int64   `json:"totalWrites"`
	TotalBytes      int64   `json:"totalBytes"`
	AvgLatency      float64 `json:"avgLatency"`      // μs
	P99Latency      float64 `json:"p99Latency"`      // μs
	WriteIOPS       int64   `json:"writeIops"`
	QueueDepth      int     `json:"queueDepth"`
	HitRate         float64 `json:"hitRate"`         // SLOG 命中率
	CompressionRatio float64 `json:"compressionRatio"`
}

// NewSLOGManager 创建 SLOG 管理器.
func NewSLOGManager(logger *zap.Logger, config *SLOGConfig) *SLOGManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultSLOGConfig()
	}

	return &SLOGManager{
		logger:     logger,
		devices:    make(map[string]*SLOGDevice),
		config:     config,
		writeQueue: make(chan *SLOGWrite, config.MaxWriteQueueSize),
		stats:      &SLOGStats{},
	}
}

// RegisterDevice 注册 SLOG 设备.
func (s *SLOGManager) RegisterDevice(device *SLOGDevice) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.devices[device.ID]; exists {
		return fmt.Errorf("设备 %s 已注册", device.ID)
	}

	device.Enabled = true
	device.CreatedAt = time.Now()
	device.UpdatedAt = time.Now()

	s.devices[device.ID] = device

	s.logger.Info("SLOG 设备已注册",
		zap.String("deviceId", device.ID),
		zap.String("type", string(device.Type)),
		zap.Int64("capacity", device.Capacity),
	)

	return nil
}

// UnregisterDevice 注销 SLOG 设备.
func (s *SLOGManager) UnregisterDevice(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.devices[deviceID]; !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	delete(s.devices, deviceID)

	s.logger.Info("SLOG 设备已注销", zap.String("deviceId", deviceID))

	return nil
}

// Write 写入 SLOG.
func (s *SLOGManager) Write(poolID string, offset, size int64, data []byte, syncMode string) (*SLOGWrite, error) {
	if !s.config.Enabled {
		return nil, fmt.Errorf("SLOG 未启用")
	}

	// 选择最佳设备
	device, err := s.selectDevice()
	if err != nil {
		return nil, err
	}

	write := &SLOGWrite{
		ID:        fmt.Sprintf("slog-%d", time.Now().UnixNano()),
		PoolID:    poolID,
		Offset:    offset,
		Size:      size,
		SyncMode:  syncMode,
		Timestamp: time.Now(),
		Status:    "pending",
	}

	// 提交到队列
	select {
	case s.writeQueue <- write:
		// 成功入队
	default:
		return nil, fmt.Errorf("写入队列已满")
	}

	// 更新统计
	s.stats.mu.Lock()
	s.stats.TotalWrites++
	s.stats.TotalBytes += size
	s.stats.WriteIOPS++
	s.stats.mu.Unlock()

	// 更新设备使用量
	s.mu.Lock()
	device.Used += size
	device.UpdatedAt = time.Now()
	s.mu.Unlock()

	s.logger.Debug("SLOG 写入",
		zap.String("writeId", write.ID),
		zap.String("deviceId", device.ID),
		zap.Int64("size", size),
	)

	return write, nil
}

// selectDevice 选择最佳 SLOG 设备.
func (s *SLOGManager) selectDevice() (*SLOGDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var bestDevice *SLOGDevice
	bestScore := -1.0

	for _, device := range s.devices {
		if !device.Enabled {
			continue
		}

		// 计算设备分数（基于可用空间和延迟）
		availableRatio := float64(device.Capacity-device.Used) / float64(device.Capacity)
		latencyScore := 1.0 / (1.0 + device.WriteLatency/1000.0)
		healthScore := device.Health / 100.0

		score := availableRatio*0.4 + latencyScore*0.4 + healthScore*0.2

		if score > bestScore {
			bestScore = score
			bestDevice = device
		}
	}

	if bestDevice == nil {
		return nil, fmt.Errorf("无可用 SLOG 设备")
	}

	return bestDevice, nil
}

// GetStats 获取 SLOG 统计.
func (s *SLOGManager) GetStats() *SLOGStats {
	s.stats.mu.RLock()
	defer s.stats.mu.RUnlock()

	return &SLOGStats{
		TotalWrites:      s.stats.TotalWrites,
		TotalBytes:       s.stats.TotalBytes,
		AvgLatency:       s.stats.AvgLatency,
		P99Latency:       s.stats.P99Latency,
		WriteIOPS:        s.stats.WriteIOPS,
		QueueDepth:       len(s.writeQueue),
		HitRate:          s.stats.HitRate,
		CompressionRatio: s.stats.CompressionRatio,
	}
}

// GetDevices 获取所有 SLOG 设备.
func (s *SLOGManager) GetDevices() []*SLOGDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	devices := make([]*SLOGDevice, 0, len(s.devices))
	for _, d := range s.devices {
		devices = append(devices, d)
	}

	return devices
}

// CheckHealth 检查 SLOG 健康状态.
func (s *SLOGManager) CheckHealth() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	healthyCount := 0
	degradedCount := 0
	failedCount := 0

	for _, device := range s.devices {
		if device.Health >= 80 {
			healthyCount++
		} else if device.Health >= 50 {
			degradedCount++
		} else {
			failedCount++
		}
	}

	status := "healthy"
	if failedCount > 0 {
		status = "critical"
	} else if degradedCount > 0 {
		status = "degraded"
	}

	return map[string]interface{}{
		"status":         status,
		"totalDevices":   len(s.devices),
		"healthyDevices": healthyCount,
		"degradedDevices": degradedCount,
		"failedDevices":  failedCount,
		"redundancyOK":   healthyCount >= s.config.MinDevices,
	}
}

// MetadataOptimizer 元数据优化器.
//
// 优化元数据和小文件的存储位置，优先使用 flash 层.
type MetadataOptimizer struct {
	mu           sync.RWMutex
	logger       *zap.Logger
	config       *MetadataConfig
	metadataCache map[string]*MetadataEntry
	stats        *MetadataStats
}

// MetadataConfig 元数据优化配置.
type MetadataConfig struct {
	// Enabled 启用元数据优化.
	Enabled bool `json:"enabled"`
	// SmallFileThreshold 小文件阈值 (字节).
	SmallFileThreshold int64 `json:"smallFileThreshold"`
	// MetadataPreference 元数据存储偏好: flash/hdd/auto.
	MetadataPreference string `json:"metadataPreference"`
	// EnableMetadataDedup 启用元数据去重.
	EnableMetadataDedup bool `json:"enableMetadataDedup"`
	// MaxMetadataCacheSize 最大元数据缓存条目数.
	MaxMetadataCacheSize int `json:"maxMetadataCacheSize"`
	// PrefetchMetadata 启用元数据预取.
	PrefetchMetadata bool `json:"prefetchMetadata"`
}

// DefaultMetadataConfig 默认元数据优化配置.
func DefaultMetadataConfig() *MetadataConfig {
	return &MetadataConfig{
		Enabled:              true,
		SmallFileThreshold:   1024 * 1024, // 1MB
		MetadataPreference:   "flash",
		EnableMetadataDedup:  true,
		MaxMetadataCacheSize: 100000,
		PrefetchMetadata:     true,
	}
}

// MetadataEntry 元数据条目.
type MetadataEntry struct {
	ID          string    `json:"id"`
	FilePath    string    `json:"filePath"`
	FileSize    int64     `json:"fileSize"`
	IsMetadata  bool      `json:"isMetadata"`
	IsSmallFile bool      `json:"isSmallFile"`
	Tier        FlashType `json:"tier"`
	CachedAt    time.Time `json:"cachedAt"`
	AccessCount int64     `json:"accessCount"`
}

// MetadataStats 元数据统计.
type MetadataStats struct {
	mu                sync.RWMutex
	TotalEntries      int64   `json:"totalEntries"`
	CachedEntries     int64   `json:"cachedEntries"`
	HitRate           float64 `json:"hitRate"`
	SmallFileCount    int64   `json:"smallFileCount"`
	MetadataFileCount int64   `json:"metadataFileCount"`
	FlashUsage        int64   `json:"flashUsage"`
}

// NewMetadataOptimizer 创建元数据优化器.
func NewMetadataOptimizer(logger *zap.Logger, config *MetadataConfig) *MetadataOptimizer {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultMetadataConfig()
	}

	return &MetadataOptimizer{
		logger:        logger,
		config:        config,
		metadataCache: make(map[string]*MetadataEntry),
		stats:         &MetadataStats{},
	}
}

// ShouldUseFlash 判断是否应该使用 flash 层.
func (o *MetadataOptimizer) ShouldUseFlash(filePath string, fileSize int64, isMetadata bool) bool {
	if !o.config.Enabled {
		return false
	}

	// 元数据优先使用 flash
	if isMetadata && o.config.MetadataPreference == "flash" {
		return true
	}

	// 小文件优先使用 flash
	if fileSize <= o.config.SmallFileThreshold {
		return true
	}

	return false
}

// RecordAccess 记录文件访问.
func (o *MetadataOptimizer) RecordAccess(filePath string, fileSize int64, isMetadata bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	entry, exists := o.metadataCache[filePath]
	if !exists {
		entry = &MetadataEntry{
			ID:          filePath,
			FilePath:    filePath,
			FileSize:    fileSize,
			IsMetadata:  isMetadata,
			IsSmallFile: fileSize <= o.config.SmallFileThreshold,
			CachedAt:    time.Now(),
		}
		o.metadataCache[filePath] = entry

		// 检查缓存大小限制
		if len(o.metadataCache) > o.config.MaxMetadataCacheSize {
			o.evictOldest()
		}
	}

	entry.AccessCount++

	// 更新统计
	o.stats.mu.Lock()
	o.stats.TotalEntries++
	if entry.IsSmallFile {
		o.stats.SmallFileCount++
	}
	if entry.IsMetadata {
		o.stats.MetadataFileCount++
	}
	o.stats.mu.Unlock()
}

// evictOldest 淘汰最旧的缓存条目.
func (o *MetadataOptimizer) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range o.metadataCache {
		if oldestKey == "" || entry.CachedAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.CachedAt
		}
	}

	if oldestKey != "" {
		delete(o.metadataCache, oldestKey)
	}
}

// GetRecommendation 获取存储推荐.
func (o *MetadataOptimizer) GetRecommendation(filePath string, fileSize int64, isMetadata bool) FlashType {
	if o.ShouldUseFlash(filePath, fileSize, isMetadata) {
		return FlashTypeNVMe
	}
	return FlashTypeHDD
}

// GetStats 获取元数据统计.
func (o *MetadataOptimizer) GetStats() *MetadataStats {
	o.stats.mu.RLock()
	defer o.stats.mu.RUnlock()

	return &MetadataStats{
		TotalEntries:      o.stats.TotalEntries,
		CachedEntries:     int64(len(o.metadataCache)),
		HitRate:           o.stats.HitRate,
		SmallFileCount:    o.stats.SmallFileCount,
		MetadataFileCount: o.stats.MetadataFileCount,
		FlashUsage:        o.stats.FlashUsage,
	}
}

// CostAnalyzer 成本分析器.
//
// 对比不同分层方案的成本效益，提供优化建议.
type CostAnalyzer struct {
	mu     sync.RWMutex
	logger *zap.Logger
	config *CostConfig
}

// CostConfig 成本分析配置.
type CostConfig struct {
	// Enabled 启用成本分析.
	Enabled bool `json:"enabled"`
	// NVMeCostPerTB NVMe 每 TB 成本 ($).
	NVMeCostPerTB float64 `json:"nvmeCostPerTB"`
	// SSDCostPerTB SSD 每 TB 成本 ($).
	SSDCostPerTB float64 `json:"ssdCostPerTB"`
	// HDDCostPerTB HDD 每 TB 成本 ($).
	HDDCostPerTB float64 `json:"hddCostPerTB"`
	// PowerCostPerKWh 电费 ($/kWh).
	PowerCostPerKWh float64 `json:"powerCostPerKWh"`
	// NVMePowerWatts NVMe 功耗 (W/TB).
	NVMePowerWatts float64 `json:"nvmePowerWatts"`
	// SSDPowerWatts SSD 功耗 (W/TB).
	SSDPowerWatts float64 `json:"ssdPowerWatts"`
	// HDDPowerWatts HDD 功耗 (W/TB).
	HDDPowerWatts float64 `json:"hddPowerWatts"`
	// AnalysisPeriod 分析周期 (年).
	AnalysisPeriod float64 `json:"analysisPeriod"`
}

// DefaultCostConfig 默认成本配置.
func DefaultCostConfig() *CostConfig {
	return &CostConfig{
		Enabled:         true,
		NVMeCostPerTB:   100.0,  // $100/TB
		SSDCostPerTB:    60.0,   // $60/TB
		HDDCostPerTB:    15.0,   // $15/TB
		PowerCostPerKWh: 0.12,   // $0.12/kWh
		NVMePowerWatts:  3.0,    // 3W/TB
		SSDPowerWatts:   2.0,    // 2W/TB
		HDDPowerWatts:   8.0,    // 8W/TB
		AnalysisPeriod:  3.0,    // 3年
	}
}

// CostAnalysisResult 成本分析结果.
type CostAnalysisResult struct {
	Scenario         string           `json:"scenario"`
	TotalCost        float64          `json:"totalCost"`        // 总成本 ($)
	StorageCost      float64          `json:"storageCost"`      // 存储成本
	PowerCost        float64          `json:"powerCost"`        // 电力成本
	CostPerTB        float64          `json:"costPerTB"`        // 每 TB 成本
	Performance      *PerformanceEst  `json:"performance"`      // 性能估算
	Recommendations  []string         `json:"recommendations"`
	Breakdown        []*CostBreakdown `json:"breakdown"`
}

// PerformanceEst 性能估算.
type PerformanceEst struct {
	AvgLatency  float64 `json:"avgLatency"`  // ms
	IOPS        int64   `json:"iops"`
	Throughput   int64   `json:"throughput"`  // MB/s
	HitRate     float64 `json:"hitRate"`
}

// CostBreakdown 成本明细.
type CostBreakdown struct {
	Item     string  `json:"item"`
	Capacity float64 `json:"capacity"` // TB
	UnitCost float64 `json:"unitCost"` // $/TB
	Total    float64 `json:"total"`
}

// NewCostAnalyzer 创建成本分析器.
func NewCostAnalyzer(logger *zap.Logger, config *CostConfig) *CostAnalyzer {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultCostConfig()
	}

	return &CostAnalyzer{
		logger: logger,
		config: config,
	}
}

// AnalyzeTieringSchemes 对比不同分层方案.
func (a *CostAnalyzer) AnalyzeTieringSchemes(totalCapacityTB float64, hotDataRatio float64) []*CostAnalysisResult {
	if !a.config.Enabled {
		return nil
	}

	results := make([]*CostAnalysisResult, 0)

	// 方案 1: 全 NVMe
	results = append(results, a.analyzeAllNVMe(totalCapacityTB))

	// 方案 2: 全 SSD
	results = append(results, a.analyzeAllSSD(totalCapacityTB))

	// 方案 3: 全 HDD
	results = append(results, a.analyzeAllHDD(totalCapacityTB))

	// 方案 4: 混合方案 (NVMe + HDD)
	results = append(results, a.analyzeHybrid(totalCapacityTB, hotDataRatio))

	// 方案 5: 混合方案 (SSD + HDD)
	results = append(results, a.analyzeHybridSSD(totalCapacityTB, hotDataRatio))

	// 标记最优方案
	a.markOptimal(results)

	return results
}

// analyzeAllNVMe 分析全 NVMe 方案.
func (a *CostAnalyzer) analyzeAllNVMe(capacityTB float64) *CostAnalysisResult {
	storageCost := capacityTB * a.config.NVMeCostPerTB
	powerCost := capacityTB * a.config.NVMePowerWatts * 24 * 365 * a.config.AnalysisPeriod * a.config.PowerCostPerKWh / 1000
	totalCost := storageCost + powerCost

	return &CostAnalysisResult{
		Scenario:    "全 NVMe",
		TotalCost:   totalCost,
		StorageCost: storageCost,
		PowerCost:   powerCost,
		CostPerTB:   totalCost / capacityTB,
		Performance: &PerformanceEst{
			AvgLatency: 0.1,
			IOPS:       1000000,
			Throughput:  7000,
			HitRate:    1.0,
		},
		Breakdown: []*CostBreakdown{
			{Item: "NVMe 存储", Capacity: capacityTB, UnitCost: a.config.NVMeCostPerTB, Total: storageCost},
			{Item: "电力成本", Capacity: capacityTB, UnitCost: powerCost / capacityTB, Total: powerCost},
		},
	}
}

// analyzeAllSSD 分析全 SSD 方案.
func (a *CostAnalyzer) analyzeAllSSD(capacityTB float64) *CostAnalysisResult {
	storageCost := capacityTB * a.config.SSDCostPerTB
	powerCost := capacityTB * a.config.SSDPowerWatts * 24 * 365 * a.config.AnalysisPeriod * a.config.PowerCostPerKWh / 1000
	totalCost := storageCost + powerCost

	return &CostAnalysisResult{
		Scenario:    "全 SSD",
		TotalCost:   totalCost,
		StorageCost: storageCost,
		PowerCost:   powerCost,
		CostPerTB:   totalCost / capacityTB,
		Performance: &PerformanceEst{
			AvgLatency: 0.2,
			IOPS:       500000,
			Throughput:  3000,
			HitRate:    1.0,
		},
		Breakdown: []*CostBreakdown{
			{Item: "SSD 存储", Capacity: capacityTB, UnitCost: a.config.SSDCostPerTB, Total: storageCost},
			{Item: "电力成本", Capacity: capacityTB, UnitCost: powerCost / capacityTB, Total: powerCost},
		},
	}
}

// analyzeAllHDD 分析全 HDD 方案.
func (a *CostAnalyzer) analyzeAllHDD(capacityTB float64) *CostAnalysisResult {
	storageCost := capacityTB * a.config.HDDCostPerTB
	powerCost := capacityTB * a.config.HDDPowerWatts * 24 * 365 * a.config.AnalysisPeriod * a.config.PowerCostPerKWh / 1000
	totalCost := storageCost + powerCost

	return &CostAnalysisResult{
		Scenario:    "全 HDD",
		TotalCost:   totalCost,
		StorageCost: storageCost,
		PowerCost:   powerCost,
		CostPerTB:   totalCost / capacityTB,
		Performance: &PerformanceEst{
			AvgLatency: 5.0,
			IOPS:       200,
			Throughput:  200,
			HitRate:    1.0,
		},
		Breakdown: []*CostBreakdown{
			{Item: "HDD 存储", Capacity: capacityTB, UnitCost: a.config.HDDCostPerTB, Total: storageCost},
			{Item: "电力成本", Capacity: capacityTB, UnitCost: powerCost / capacityTB, Total: powerCost},
		},
	}
}

// analyzeHybrid 分析 NVMe + HDD 混合方案.
func (a *CostAnalyzer) analyzeHybrid(totalTB, hotRatio float64) *CostAnalysisResult {
	nvmeTB := totalTB * hotRatio
	hddTB := totalTB * (1 - hotRatio)

	nvmeStorageCost := nvmeTB * a.config.NVMeCostPerTB
	hddStorageCost := hddTB * a.config.HDDCostPerTB
	storageCost := nvmeStorageCost + hddStorageCost

	powerCost := (nvmeTB*a.config.NVMePowerWatts + hddTB*a.config.HDDPowerWatts) * 24 * 365 * a.config.AnalysisPeriod * a.config.PowerCostPerKWh / 1000
	totalCost := storageCost + powerCost

	// 混合性能估算
	avgLatency := 0.1*hotRatio + 5.0*(1-hotRatio)
	hitRate := hotRatio*1.0 + (1-hotRatio)*0.1

	return &CostAnalysisResult{
		Scenario:    "NVMe + HDD 混合",
		TotalCost:   totalCost,
		StorageCost: storageCost,
		PowerCost:   powerCost,
		CostPerTB:   totalCost / totalTB,
		Performance: &PerformanceEst{
			AvgLatency: avgLatency,
			IOPS:       int64(1000000*hotRatio + 200*(1-hotRatio)),
			Throughput:  int64(7000*hotRatio + 200*(1-hotRatio)),
			HitRate:    hitRate,
		},
		Breakdown: []*CostBreakdown{
			{Item: "NVMe 存储", Capacity: nvmeTB, UnitCost: a.config.NVMeCostPerTB, Total: nvmeStorageCost},
			{Item: "HDD 存储", Capacity: hddTB, UnitCost: a.config.HDDCostPerTB, Total: hddStorageCost},
			{Item: "电力成本", Capacity: totalTB, UnitCost: powerCost / totalTB, Total: powerCost},
		},
	}
}

// analyzeHybridSSD 分析 SSD + HDD 混合方案.
func (a *CostAnalyzer) analyzeHybridSSD(totalTB, hotRatio float64) *CostAnalysisResult {
	ssdTB := totalTB * hotRatio
	hddTB := totalTB * (1 - hotRatio)

	ssdStorageCost := ssdTB * a.config.SSDCostPerTB
	hddStorageCost := hddTB * a.config.HDDCostPerTB
	storageCost := ssdStorageCost + hddStorageCost

	powerCost := (ssdTB*a.config.SSDPowerWatts + hddTB*a.config.HDDPowerWatts) * 24 * 365 * a.config.AnalysisPeriod * a.config.PowerCostPerKWh / 1000
	totalCost := storageCost + powerCost

	// 混合性能估算
	avgLatency := 0.2*hotRatio + 5.0*(1-hotRatio)
	hitRate := hotRatio*1.0 + (1-hotRatio)*0.1

	return &CostAnalysisResult{
		Scenario:    "SSD + HDD 混合",
		TotalCost:   totalCost,
		StorageCost: storageCost,
		PowerCost:   powerCost,
		CostPerTB:   totalCost / totalTB,
		Performance: &PerformanceEst{
			AvgLatency: avgLatency,
			IOPS:       int64(500000*hotRatio + 200*(1-hotRatio)),
			Throughput:  int64(3000*hotRatio + 200*(1-hotRatio)),
			HitRate:    hitRate,
		},
		Breakdown: []*CostBreakdown{
			{Item: "SSD 存储", Capacity: ssdTB, UnitCost: a.config.SSDCostPerTB, Total: ssdStorageCost},
			{Item: "HDD 存储", Capacity: hddTB, UnitCost: a.config.HDDCostPerTB, Total: hddStorageCost},
			{Item: "电力成本", Capacity: totalTB, UnitCost: powerCost / totalTB, Total: powerCost},
		},
	}
}

// markOptimal 标记最优方案.
func (a *CostAnalyzer) markOptimal(results []*CostAnalysisResult) {
	if len(results) == 0 {
		return
	}

	// 计算性价比 (性能/成本)
	bestIndex := 0
	bestValue := 0.0

	for i, r := range results {
		if r.TotalCost == 0 {
			continue
		}

		// 性价比 = IOPS / 成本
		value := float64(r.Performance.IOPS) / r.TotalCost

		if value > bestValue {
			bestValue = value
			bestIndex = i
		}
	}

	// 标记最优方案
	results[bestIndex].Recommendations = append(
		results[bestIndex].Recommendations,
		"★ 推荐方案: 性价比最高",
	)
}

// EstimateCostSavings 估算成本节省.
func (a *CostAnalyzer) EstimateCostSavings(currentScheme, optimalScheme *CostAnalysisResult) map[string]interface{} {
	if currentScheme == nil || optimalScheme == nil {
		return nil
	}

	savings := currentScheme.TotalCost - optimalScheme.TotalCost
	savingsPercent := 0.0
	if currentScheme.TotalCost > 0 {
		savingsPercent = (savings / currentScheme.TotalCost) * 100
	}

	performanceGain := 0.0
	if currentScheme.Performance.AvgLatency > 0 {
		performanceGain = ((currentScheme.Performance.AvgLatency - optimalScheme.Performance.AvgLatency) /
			currentScheme.Performance.AvgLatency) * 100
	}

	return map[string]interface{}{
		"currentCost":      currentScheme.TotalCost,
		"optimalCost":      optimalScheme.TotalCost,
		"savings":          savings,
		"savingsPercent":   savingsPercent,
		"performanceGain":  performanceGain,
		"currentScenario":  currentScheme.Scenario,
		"optimalScenario":  optimalScheme.Scenario,
	}
}
