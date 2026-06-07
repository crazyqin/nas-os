// Package cost provides storage efficiency statistics
// 对标群晖DSM存储效率分析 + TrueNAS成本管理
package cost

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"
	"time"
)

// ========== 存储效率类型定义 ==========

// EfficiencyType 效率类型
type EfficiencyType string

const (
	EfficiencyTypeDedup       EfficiencyType = "dedup"       // 去重效率
	EfficiencyTypeCompression EfficiencyType = "compression" // 压缩效率
	EfficiencyTypeThinProv    EfficiencyType = "thin_prov"   // 精简配置效率
	EfficiencyTypeTiering     EfficiencyType = "tiering"     // 分层存储效率
	EfficiencyTypeOverall     EfficiencyType = "overall"     // 综合效率
)

// StorageEfficiencyStats 存储效率统计（对标群晖）
type StorageEfficiencyStats struct {
	// 统计时间
	CollectedAt time.Time `json:"collectedAt"`

	// 资源名称
	ResourceName string `json:"resourceName"`

	// 资源类型
	ResourceType string `json:"resourceType"` // volume/pool/dataset

	// ========== 基础容量 ==========

	// 物理总容量（字节）
	PhysicalCapacityTotal uint64 `json:"physicalCapacityTotal"`

	// 物理已用容量（字节）
	PhysicalCapacityUsed uint64 `json:"physicalCapacityUsed"`

	// 物理可用容量（字节）
	PhysicalCapacityFree uint64 `json:"physicalCapacityFree"`

	// 逻辑总容量（字节）- 精简配置后的逻辑容量
	LogicalCapacityTotal uint64 `json:"logicalCapacityTotal"`

	// 逻辑已用容量（字节）
	LogicalCapacityUsed uint64 `json:"logicalCapacityUsed"`

	// ========== 去重统计 ==========

	// 去重比率（如1.5x表示节省33%）
	DedupRatio float64 `json:"dedupRatio"`

	// 去重节省空间（字节）
	DedupSavedBytes uint64 `json:"dedupSavedBytes"`

	// 去重节省百分比
	DedupSavedPercent float64 `json:"dedupSavedPercent"`

	// 去重表大小（字节）
	DedupTableSize uint64 `json:"dedupTableSize"`

	// 去重块数
	DedupBlockCount uint64 `json:"dedupBlockCount"`

	// ========== 压缩统计 ==========

	// 压缩比率
	CompressionRatio float64 `json:"compressionRatio"`

	// 压缩节省空间（字节）
	CompressionSavedBytes uint64 `json:"compressionSavedBytes"`

	// 压缩节省百分比
	CompressionSavedPercent float64 `json:"compressionSavedPercent"`

	// 压缩算法
	CompressionAlgorithm string `json:"compressionAlgorithm"`

	// ========== 精简配置统计 ==========

	// 精简配置比率
	ThinProvRatio float64 `json:"thinProvRatio"`

	// 精简配置超额分配（字节）
	ThinProvOvercommit uint64 `json:"thinProvOvercommit"`

	// 精简配置使用率（实际使用/逻辑容量）
	ThinProvUtilization float64 `json:"thinProvUtilization"`

	// ========== 分层存储统计 ==========

	// 热层使用率（%）
	HotTierUsagePercent float64 `json:"hotTierUsagePercent"`

	// 冷层使用率（%）
	ColdTierUsagePercent float64 `json:"coldTierUsagePercent"`

	// 热层命中率（%）
	HotTierHitRate float64 `json:"hotTierHitRate"`

	// 分层迁移次数
	TierMigrationCount uint64 `json:"tierMigrationCount"`

	// ========== 综合效率 ==========

	// 综合效率评分（0-100）
	OverallEfficiencyScore float64 `json:"overallEfficiencyScore"`

	// 综合节省空间（字节）
	TotalSavedBytes uint64 `json:"totalSavedBytes"`

	// 综合节省百分比
	TotalSavedPercent float64 `json:"totalSavedPercent"`

	// 物理到逻辑比率（物理/逻辑）
	PhysicalToLogicalRatio float64 `json:"physicalToLogicalRatio"`

	// ========== 成本影响 ==========

	// 单位成本（元/GB）
	CostPerGB float64 `json:"costPerGB"`

	// 有效成本（考虑去重压缩后）
	EffectiveCostPerGB float64 `json:"effectiveCostPerGB"`

	// 成本节省（元/月）
	CostSavedMonthly float64 `json:"costSavedMonthly"`

	// ========== 趋势 ==========

	// 效率趋势（improving/stable/degrading）
	EfficiencyTrend string `json:"efficiencyTrend"`

	// 趋势变化率（%）
	TrendChangeRate float64 `json:"trendChangeRate"`

	// 历史数据点（最近30天）
	HistoricalData []EfficiencyHistoryPoint `json:"historicalData"`
}

// EfficiencyHistoryPoint 效率历史数据点
type EfficiencyHistoryPoint struct {
	Timestamp         time.Time `json:"timestamp"`
	OverallEfficiency float64   `json:"overallEfficiency"`
	DedupRatio        float64   `json:"dedupRatio"`
	CompressionRatio  float64   `json:"compressionRatio"`
	PhysicalUsedGB    float64   `json:"physicalUsedGB"`
	LogicalUsedGB     float64   `json:"logicalUsedGB"`
	CostPerGB         float64   `json:"costPerGB"`
}

// EfficiencyRecommendation 效率优化建议
type EfficiencyRecommendation struct {
	// 建议类型
	Type EfficiencyType `json:"type"`

	// 优先级（1-5，越高越重要）
	Priority int `json:"priority"`

	// 建议内容
	Suggestion string `json:"suggestion"`

	// 预期收益（字节节省或成本节省）
	ExpectedBenefit uint64 `json:"expectedBenefit"`

	// 收益描述
	BenefitDescription string `json:"benefitDescription"`

	// 实施难度（easy/medium/hard）
	ImplementationDifficulty string `json:"implementationDifficulty"`

	// 实施步骤
	ImplementationSteps []string `json:"implementationSteps"`
}

// ========== 效率监控服务 ==========

// EfficiencyMonitorConfig 监控配置
type EfficiencyMonitorConfig struct {
	// 监控间隔（分钟）
	MonitorIntervalMinutes int `json:"monitorIntervalMinutes"`

	// 历史数据保留天数
	HistoryRetentionDays int `json:"historyRetentionDays"`

	// 低效率阈值（%）
	LowEfficiencyThreshold float64 `json:"lowEfficiencyThreshold"`

	// 去重低效率阈值（比率）
	LowDedupThreshold float64 `json:"lowDedupThreshold"`

	// 压缩低效率阈值（比率）
	LowCompressionThreshold float64 `json:"lowCompressionThreshold"`

	// 告警启用
	AlertingEnabled bool `json:"alertingEnabled"`

	// 报告生成间隔（小时）
	ReportIntervalHours int `json:"reportIntervalHours"`

	// 成本单价（元/GB/月）
	CostPerGBMonthly float64 `json:"costPerGBMonthly"`
}

// DefaultEfficiencyMonitorConfig 默认配置
var DefaultEfficiencyMonitorConfig = EfficiencyMonitorConfig{
	MonitorIntervalMinutes:  15,
	HistoryRetentionDays:    30,
	LowEfficiencyThreshold:  50,
	LowDedupThreshold:       1.1,
	LowCompressionThreshold: 1.2,
	AlertingEnabled:         true,
	ReportIntervalHours:     24,
	CostPerGBMonthly:        0.1,
}

// EfficiencyMonitorService 效率监控服务
type EfficiencyMonitorService struct {
	config     EfficiencyMonitorConfig
	stats      map[string]*StorageEfficiencyStats
	history    map[string][]EfficiencyHistoryPoint
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	logger     interface{} // 使用interface避免依赖
	configPath string
}

// NewEfficiencyMonitorService 创建效率监控服务
func NewEfficiencyMonitorService(config EfficiencyMonitorConfig) *EfficiencyMonitorService {
	ctx, cancel := context.WithCancel(context.Background())

	return &EfficiencyMonitorService{
		config:     config,
		stats:      make(map[string]*StorageEfficiencyStats),
		history:    make(map[string][]EfficiencyHistoryPoint),
		ctx:        ctx,
		cancel:     cancel,
		configPath: "/var/lib/nas-os/efficiency",
	}
}

// Start 启动监控服务
func (s *EfficiencyMonitorService) Start() error {
	// 确保数据目录存在
	_ = os.MkdirAll(s.configPath, 0750)

	// 加载历史数据
	_ = s.loadHistory()

	// 启动监控循环
	s.wg.Add(1)
	go s.monitorLoop()

	// 启动报告生成循环
	s.wg.Add(1)
	go s.reportLoop()

	return nil
}

// Stop 停止监控服务
func (s *EfficiencyMonitorService) Stop() {
	s.cancel()
	s.wg.Wait()
	_ = s.saveHistory()
}

// ========== 核心API ==========

// GetEfficiencyStats 获取效率统计
func (s *EfficiencyMonitorService) GetEfficiencyStats(resourceName string) (*StorageEfficiencyStats, error) {
	s.mu.RLock()
	stats, exists := s.stats[resourceName]
	s.mu.RUnlock()

	if !exists {
		// 实时采集
		stats, err := s.collectEfficiency(resourceName)
		if err != nil {
			return nil, err
		}
		return stats, nil
	}

	return stats, nil
}

// GetAllEfficiencyStats 获取所有资源效率统计
func (s *EfficiencyMonitorService) GetAllEfficiencyStats() map[string]*StorageEfficiencyStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*StorageEfficiencyStats)
	for name, stats := range s.stats {
		result[name] = stats
	}

	return result
}

// GetRecommendations 获取优化建议
func (s *EfficiencyMonitorService) GetRecommendations(resourceName string) []EfficiencyRecommendation {
	stats, err := s.GetEfficiencyStats(resourceName)
	if err != nil {
		return nil
	}

	return s.generateRecommendations(stats)
}

// ========== 内部方法 ==========

// monitorLoop 监控循环
func (s *EfficiencyMonitorService) monitorLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(s.config.MonitorIntervalMinutes) * time.Minute)
	defer ticker.Stop()

	// 启动后立即采集一次
	s.collectAll()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.collectAll()
		}
	}
}

// reportLoop 报告生成循环
func (s *EfficiencyMonitorService) reportLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(s.config.ReportIntervalHours) * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.generateReport()
		}
	}
}

// collectAll 采集所有资源效率
func (s *EfficiencyMonitorService) collectAll() {
	// 获取存储资源列表（这里简化实现）
	// 实际应从存储管理器获取
	resourceNames := []string{"default-pool", "tank", "backup-pool"}

	for _, name := range resourceNames {
		stats, err := s.collectEfficiency(name)
		if err != nil {
			continue
		}

		s.mu.Lock()
		s.stats[name] = stats

		// 添加历史数据点
		historyPoint := EfficiencyHistoryPoint{
			Timestamp:         time.Now(),
			OverallEfficiency: stats.OverallEfficiencyScore,
			DedupRatio:        stats.DedupRatio,
			CompressionRatio:  stats.CompressionRatio,
			PhysicalUsedGB:    float64(stats.PhysicalCapacityUsed) / 1024 / 1024 / 1024,
			LogicalUsedGB:     float64(stats.LogicalCapacityUsed) / 1024 / 1024 / 1024,
			CostPerGB:         stats.CostPerGB,
		}

		s.history[name] = append(s.history[name], historyPoint)

		// 限制历史数据长度
		if len(s.history[name]) > s.config.HistoryRetentionDays*24*60/s.config.MonitorIntervalMinutes {
			s.history[name] = s.history[name][:100]
		}

		s.mu.Unlock()
	}
}

// collectEfficiency 采集单个资源效率
func (s *EfficiencyMonitorService) collectEfficiency(resourceName string) (*StorageEfficiencyStats, error) {
	stats := &StorageEfficiencyStats{
		CollectedAt:            time.Now(),
		ResourceName:           resourceName,
		ResourceType:           "volume",
		DedupRatio:             1.0,
		CompressionRatio:       1.0,
		ThinProvRatio:          1.0,
		OverallEfficiencyScore: 100,
	}

	// 模拟数据采集（实际应调用btrfs/zfs命令）
	// 这里使用模拟数据进行演示
	stats.PhysicalCapacityTotal = 10 * 1024 * 1024 * 1024 * 1024 // 10TB
	stats.PhysicalCapacityUsed = 3 * 1024 * 1024 * 1024 * 1024   // 3TB
	stats.PhysicalCapacityFree = stats.PhysicalCapacityTotal - stats.PhysicalCapacityUsed

	stats.DedupRatio = 1.5 // 1.5x去重比率
	stats.DedupSavedBytes = uint64(float64(stats.PhysicalCapacityUsed) * (stats.DedupRatio - 1) / stats.DedupRatio)
	stats.DedupSavedPercent = (stats.DedupRatio - 1) / stats.DedupRatio * 100

	stats.CompressionRatio = 1.8 // 1.8x压缩比率
	stats.CompressionSavedBytes = uint64(float64(stats.PhysicalCapacityUsed) * (stats.CompressionRatio - 1) / stats.CompressionRatio)
	stats.CompressionSavedPercent = (stats.CompressionRatio - 1) / stats.CompressionRatio * 100

	// 计算逻辑容量
	stats.LogicalCapacityUsed = stats.PhysicalCapacityUsed + stats.DedupSavedBytes + stats.CompressionSavedBytes
	stats.LogicalCapacityTotal = stats.PhysicalCapacityTotal + uint64(float64(stats.PhysicalCapacityTotal)*(stats.DedupRatio*stats.CompressionRatio-1)/(stats.DedupRatio*stats.CompressionRatio))

	// 综合效率计算
	physicalGB := float64(stats.PhysicalCapacityUsed) / 1024 / 1024 / 1024
	logicalGB := float64(stats.LogicalCapacityUsed) / 1024 / 1024 / 1024

	stats.PhysicalToLogicalRatio = physicalGB / logicalGB
	stats.TotalSavedBytes = stats.DedupSavedBytes + stats.CompressionSavedBytes
	stats.TotalSavedPercent = stats.DedupSavedPercent + stats.CompressionSavedPercent

	// 效率评分计算（基于多个指标）
	stats.OverallEfficiencyScore = s.calculateEfficiencyScore(stats)

	// 成本计算
	stats.CostPerGB = s.config.CostPerGBMonthly
	stats.EffectiveCostPerGB = stats.CostPerGB * stats.PhysicalToLogicalRatio
	stats.CostSavedMonthly = float64(stats.TotalSavedBytes) / 1024 / 1024 / 1024 * s.config.CostPerGBMonthly

	// 趋势分析
	stats.EfficiencyTrend = s.analyzeTrend(resourceName)
	stats.HistoricalData = s.history[resourceName]

	return stats, nil
}

// calculateEfficiencyScore 计算效率评分
func (s *EfficiencyMonitorService) calculateEfficiencyScore(stats *StorageEfficiencyStats) float64 {
	// 基于多个指标加权计算
	score := 100.0

	// 去重效率评分（权重30%）
	if stats.DedupRatio < s.config.LowDedupThreshold {
		score -= (s.config.LowDedupThreshold - stats.DedupRatio) * 20
	} else {
		score += (stats.DedupRatio - 1) * 5 // 最高加25分
	}

	// 压缩效率评分（权重30%）
	if stats.CompressionRatio < s.config.LowCompressionThreshold {
		score -= (s.config.LowCompressionThreshold - stats.CompressionRatio) * 15
	} else {
		score += (stats.CompressionRatio - 1) * 3 // 最高加15分
	}

	// 物理使用率评分（权重20%）
	usagePercent := float64(stats.PhysicalCapacityUsed) / float64(stats.PhysicalCapacityTotal) * 100
	if usagePercent > 80 {
		score -= (usagePercent - 80) * 0.5 // 使用率超过80%扣分
	}

	// 分层存储效率评分（权重20%）
	if stats.HotTierHitRate > 50 {
		score += stats.HotTierHitRate * 0.2 // 热层命中率加分
	}

	// 确保评分在0-100范围内
	return math.Max(0, math.Min(100, score))
}

// analyzeTrend 分析效率趋势
func (s *EfficiencyMonitorService) analyzeTrend(resourceName string) string {
	s.mu.RLock()
	history := s.history[resourceName]
	s.mu.RUnlock()

	if len(history) < 2 {
		return "stable"
	}

	// 比较最近两个数据点
	latest := history[len(history)-1]
	previous := history[len(history)-2]

	changeRate := (latest.OverallEfficiency - previous.OverallEfficiency) / previous.OverallEfficiency * 100

	if changeRate > 5 {
		return "improving"
	} else if changeRate < -5 {
		return "degrading"
	}

	return "stable"
}

// generateRecommendations 生成优化建议
func (s *EfficiencyMonitorService) generateRecommendations(stats *StorageEfficiencyStats) []EfficiencyRecommendation {
	recommendations := []EfficiencyRecommendation{}

	// 去重优化建议
	if stats.DedupRatio < s.config.LowDedupThreshold {
		rec := EfficiencyRecommendation{
			Type:                     EfficiencyTypeDedup,
			Priority:                 3,
			Suggestion:               "考虑启用数据去重以提高存储效率",
			ExpectedBenefit:          uint64(float64(stats.PhysicalCapacityUsed) * 0.2),
			BenefitDescription:       "预计可节省约20%存储空间",
			ImplementationDifficulty: "medium",
			ImplementationSteps: []string{
				"评估数据类型是否适合去重（虚拟化镜像、备份库收益最高）",
				"在数据集启用dedup=on选项",
				"监控内存使用（Fast Dedup降低内存需求）",
			},
		}
		recommendations = append(recommendations, rec)
	}

	// 压缩优化建议
	if stats.CompressionRatio < s.config.LowCompressionThreshold {
		rec := EfficiencyRecommendation{
			Type:                     EfficiencyTypeCompression,
			Priority:                 4,
			Suggestion:               "考虑启用更高压缩率算法（如zstd）",
			ExpectedBenefit:          uint64(float64(stats.PhysicalCapacityUsed) * 0.15),
			BenefitDescription:       "预计可节省约15%存储空间",
			ImplementationDifficulty: "easy",
			ImplementationSteps: []string{
				"设置compression=zstd",
				"新数据自动压缩",
				"现有数据可通过rewrite压缩",
			},
		}
		recommendations = append(recommendations, rec)
	}

	// 分层存储建议
	if stats.HotTierHitRate < 30 && stats.ColdTierUsagePercent > 50 {
		rec := EfficiencyRecommendation{
			Type:                     EfficiencyTypeTiering,
			Priority:                 2,
			Suggestion:               "优化分层存储策略，提高热层命中率",
			ExpectedBenefit:          0, // 性能收益而非容量收益
			BenefitDescription:       "提高热层命中率可显著改善读写性能",
			ImplementationDifficulty: "medium",
			ImplementationSteps: []string{
				"分析热点数据访问模式",
				"调整迁移策略阈值",
				"增加SSD热层容量",
			},
		}
		recommendations = append(recommendations, rec)
	}

	// 容量预警
	usagePercent := float64(stats.PhysicalCapacityUsed) / float64(stats.PhysicalCapacityTotal) * 100
	if usagePercent > 80 {
		rec := EfficiencyRecommendation{
			Type:                     EfficiencyTypeOverall,
			Priority:                 5,
			Suggestion:               "存储使用率超过80%，建议扩容或清理",
			ExpectedBenefit:          0,
			BenefitDescription:       fmt.Sprintf("当前使用率%.1f%%，存在容量风险", usagePercent),
			ImplementationDifficulty: "medium",
			ImplementationSteps: []string{
				"检查大文件和日志",
				"清理过期快照",
				"规划容量扩容",
			},
		}
		recommendations = append(recommendations, rec)
	}

	return recommendations
}

// generateReport 生成报告
func (s *EfficiencyMonitorService) generateReport() {
	report := s.generateEfficiencyReport()

	// 保存报告
	reportPath := fmt.Sprintf("%s/efficiency-report-%s.json", s.configPath, time.Now().Format("20060102-150405"))
	data, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(reportPath, data, 0644)
}

// EfficiencyReport 效率报告
type EfficiencyReport struct {
	GeneratedAt        time.Time                          `json:"generatedAt"`
	ResourceStats      map[string]*StorageEfficiencyStats `json:"resourceStats"`
	TopRecommendations []EfficiencyRecommendation         `json:"topRecommendations"`
	Summary            EfficiencySummary                  `json:"summary"`
}

// EfficiencySummary 效率汇总
type EfficiencySummary struct {
	TotalPhysicalUsedGB     float64 `json:"totalPhysicalUsedGB"`
	TotalLogicalUsedGB      float64 `json:"totalLogicalUsedGB"`
	OverallEfficiency       float64 `json:"overallEfficiency"`
	TotalCostSavedMonthly   float64 `json:"totalCostSavedMonthly"`
	AverageDedupRatio       float64 `json:"averageDedupRatio"`
	AverageCompressionRatio float64 `json:"averageCompressionRatio"`
	ResourceCount           int     `json:"resourceCount"`
	ImprovingCount          int     `json:"improvingCount"`
	DegradingCount          int     `json:"degradingCount"`
}

// generateEfficiencyReport 生成完整效率报告
func (s *EfficiencyMonitorService) generateEfficiencyReport() *EfficiencyReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report := &EfficiencyReport{
		GeneratedAt:        time.Now(),
		ResourceStats:      make(map[string]*StorageEfficiencyStats),
		TopRecommendations: []EfficiencyRecommendation{},
	}

	// 复制统计数据
	for name, stats := range s.stats {
		report.ResourceStats[name] = stats
	}

	// 生成汇总
	summary := EfficiencySummary{}
	for _, stats := range s.stats {
		summary.TotalPhysicalUsedGB += float64(stats.PhysicalCapacityUsed) / 1024 / 1024 / 1024
		summary.TotalLogicalUsedGB += float64(stats.LogicalCapacityUsed) / 1024 / 1024 / 1024
		summary.TotalCostSavedMonthly += stats.CostSavedMonthly
		summary.AverageDedupRatio += stats.DedupRatio
		summary.AverageCompressionRatio += stats.CompressionRatio
		summary.ResourceCount++

		if stats.EfficiencyTrend == "improving" {
			summary.ImprovingCount++
		} else if stats.EfficiencyTrend == "degrading" {
			summary.DegradingCount++
		}
	}

	if summary.ResourceCount > 0 {
		summary.AverageDedupRatio /= float64(summary.ResourceCount)
		summary.AverageCompressionRatio /= float64(summary.ResourceCount)
	}

	summary.OverallEfficiency = summary.TotalLogicalUsedGB / summary.TotalPhysicalUsedGB

	report.Summary = summary

	// 收集所有建议并按优先级排序
	for _, stats := range s.stats {
		for _, rec := range s.generateRecommendations(stats) {
			report.TopRecommendations = append(report.TopRecommendations, rec)
		}
	}

	return report
}

// loadHistory 加载历史数据
func (s *EfficiencyMonitorService) loadHistory() error {
	data, err := os.ReadFile(s.configPath + "/history.json")
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &s.history)
}

// saveHistory 保存历史数据
func (s *EfficiencyMonitorService) saveHistory() error {
	data, err := json.MarshalIndent(s.history, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.configPath+"/history.json", data, 0644)
}
