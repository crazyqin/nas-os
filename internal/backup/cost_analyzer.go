// Package backup 提供备份成本分析功能
// 支持多种存储后端的成本计算、趋势分析和优化建议
package backup

import (
	"fmt"
	"math"
	"sync"
	"time"
)

// ============================================================
// 成本数据模型
// ============================================================

// StorageCostConfig 存储成本配置
// 定义不同存储后端的定价参数
type StorageCostConfig struct {
	// 存储提供商类型
	Provider CloudProvider `json:"provider"`

	// 存储成本（元/GB/月）
	StoragePricePerGB float64 `json:"storagePricePerGB"`

	// 下载流量成本（元/GB）
	DownloadPricePerGB float64 `json:"downloadPricePerGB"`

	// 上传流量成本（元/GB，通常为 0）
	UploadPricePerGB float64 `json:"uploadPricePerGB"`

	// 请求成本（元/万次）
	RequestPricePer10K float64 `json:"requestPricePer10K"`

	// 最低存储期限（天）
	MinimumStorageDays int `json:"minimumStorageDays"`

	// 可用性 SLA（百分比）
	AvailabilitySLA float64 `json:"availabilitySLA"`
}

// DefaultStorageCostConfigs 默认存储成本配置
func DefaultStorageCostConfigs() map[CloudProvider]*StorageCostConfig {
	return map[CloudProvider]*StorageCostConfig{
		"local": {
			Provider:           "local",
			StoragePricePerGB:  0.0,
			DownloadPricePerGB: 0.0,
			UploadPricePerGB:   0.0,
			RequestPricePer10K: 0.0,
			AvailabilitySLA:    99.9,
		},
		CloudProviderS3: {
			Provider:           CloudProviderS3,
			StoragePricePerGB:  0.12,
			DownloadPricePerGB: 0.5,
			UploadPricePerGB:   0.0,
			RequestPricePer10K: 0.01,
			AvailabilitySLA:    99.99,
		},
		CloudProviderAliyun: {
			Provider:           CloudProviderAliyun,
			StoragePricePerGB:  0.12,
			DownloadPricePerGB: 0.5,
			UploadPricePerGB:   0.0,
			RequestPricePer10K: 0.01,
			AvailabilitySLA:    99.995,
		},
		CloudProviderWebDAV: {
			Provider:           CloudProviderWebDAV,
			StoragePricePerGB:  0.05,
			DownloadPricePerGB: 0.1,
			UploadPricePerGB:   0.0,
			RequestPricePer10K: 0.0,
			AvailabilitySLA:    99.5,
		},
	}
}

// BackupCostRecord 备份成本记录
type BackupCostRecord struct {
	BackupID         string        `json:"backupId"`
	ConfigID         string        `json:"configId"`
	BackupName       string        `json:"backupName"`
	Provider         CloudProvider `json:"provider"`
	Timestamp        time.Time     `json:"timestamp"`
	StorageCost      float64       `json:"storageCost"`
	UploadCost       float64       `json:"uploadCost"`
	DownloadCost     float64       `json:"downloadCost"`
	RequestCost      float64       `json:"requestCost"`
	TotalCost        float64       `json:"totalCost"`
	OriginalSize     int64         `json:"originalSize"`
	StoredSize       int64         `json:"storedSize"`
	CompressionRatio float64       `json:"compressionRatio"`
	UploadBytes      int64         `json:"uploadBytes"`
	DownloadBytes    int64         `json:"downloadBytes"`
	RequestCount     int64         `json:"requestCount"`
	BackupType       BackupType    `json:"backupType"`
	Incremental      bool          `json:"incremental"`
	Duration         time.Duration `json:"duration"`
}

// CostTrendData 成本趋势数据
type CostTrendData struct {
	Timestamp           time.Time `json:"timestamp"`
	StorageCost         float64   `json:"storageCost"`
	TrafficCost         float64   `json:"trafficCost"`
	TotalCost           float64   `json:"totalCost"`
	TotalStorage        int64     `json:"totalStorage"`
	BackupCount         int       `json:"backupCount"`
	AvgCompressionRatio float64   `json:"avgCompressionRatio"`
}

// CostReport 成本分析报告
type CostReport struct {
	GeneratedAt     time.Time            `json:"generatedAt"`
	Period          ReportPeriod         `json:"period"`
	StartTime       time.Time            `json:"startTime"`
	EndTime         time.Time            `json:"endTime"`
	Summary         CostSummary          `json:"summary"`
	CostByProvider  []ProviderCost       `json:"costByProvider"`
	Trend           []*CostTrendData     `json:"trend"`
	Recommendations []CostRecommendation `json:"recommendations"`
	Forecast        *CostForecast        `json:"forecast,omitempty"`
	Alerts          []CostAlert          `json:"alerts,omitempty"`
}

// ReportPeriod 报告周期类型
type ReportPeriod string

const (
	PeriodDaily   ReportPeriod = "daily"
	PeriodWeekly  ReportPeriod = "weekly"
	PeriodMonthly ReportPeriod = "monthly"
	PeriodYearly  ReportPeriod = "yearly"
)

// CostSummary 成本汇总
type CostSummary struct {
	TotalCost            float64 `json:"totalCost"`
	StorageCost          float64 `json:"storageCost"`
	UploadCost           float64 `json:"uploadCost"`
	DownloadCost         float64 `json:"downloadCost"`
	RequestCost          float64 `json:"requestCost"`
	TotalStorage         int64   `json:"totalStorage"`
	TotalStorageHuman    string  `json:"totalStorageHuman"`
	BackupCount          int     `json:"backupCount"`
	RestoreCount         int     `json:"restoreCount"`
	AvgCompressionRatio  float64 `json:"avgCompressionRatio"`
	CostChangeRate       float64 `json:"costChangeRate"`
	EstimatedMonthlyCost float64 `json:"estimatedMonthlyCost"`
}

// ProviderCost 按提供商分类的成本
type ProviderCost struct {
	Provider            CloudProvider `json:"provider"`
	StorageCost         float64       `json:"storageCost"`
	TrafficCost         float64       `json:"trafficCost"`
	TotalCost           float64       `json:"totalCost"`
	Storage             int64         `json:"storage"`
	StoragePercentage   float64       `json:"storagePercentage"`
	CostPercentage      float64       `json:"costPercentage"`
	AvgCompressionRatio float64       `json:"avgCompressionRatio"`
	BackupCount         int           `json:"backupCount"`
}

// CostRecommendation 成本优化建议
type CostRecommendation struct {
	Type             RecommendationType `json:"type"`
	Priority         int                `json:"priority"`
	Title            string             `json:"title"`
	Description      string             `json:"description"`
	PotentialSavings float64            `json:"potentialSavings"`
	AffectedConfigs  []string           `json:"affectedConfigs,omitempty"`
	Steps            []string           `json:"steps,omitempty"`
	Risk             string             `json:"risk,omitempty"`
}

// RecommendationType 建议类型
type RecommendationType string

const (
	RecommendationCompression   RecommendationType = "compression"
	RecommendationStorageTier   RecommendationType = "storage_tier"
	RecommendationIncremental   RecommendationType = "incremental"
	RecommendationRetention     RecommendationType = "retention"
	RecommendationLocation      RecommendationType = "location"
	RecommendationDeduplication RecommendationType = "deduplication"
)

// CostForecast 成本预测
type CostForecast struct {
	Date                 time.Time `json:"date"`
	ProjectedStorage     int64     `json:"projectedStorage"`
	ProjectedDailyCost   float64   `json:"projectedDailyCost"`
	ProjectedMonthlyCost float64   `json:"projectedMonthlyCost"`
	Confidence           float64   `json:"confidence"`
	Basis                string    `json:"basis"`
}

// CostAlert 成本告警
type CostAlert struct {
	Level        AlertLevel `json:"level"`
	Type         string     `json:"type"`
	Message      string     `json:"message"`
	CurrentValue float64    `json:"currentValue"`
	Threshold    float64    `json:"threshold"`
	Timestamp    time.Time  `json:"timestamp"`
}

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// CostAlertThresholds 成本告警阈值配置
type CostAlertThresholds struct {
	MonthlyCostWarning    float64 `json:"monthlyCostWarning"`
	MonthlyCostCritical   float64 `json:"monthlyCostCritical"`
	SingleBackupWarning   float64 `json:"singleBackupWarning"`
	SingleBackupCritical  float64 `json:"singleBackupCritical"`
	StorageGrowthWarning  float64 `json:"storageGrowthWarning"`
	StorageGrowthCritical float64 `json:"storageGrowthCritical"`
	MinCompressionRatio   float64 `json:"minCompressionRatio"`
}

// DefaultCostAlertThresholds 默认告警阈值
func DefaultCostAlertThresholds() *CostAlertThresholds {
	return &CostAlertThresholds{
		MonthlyCostWarning:    100.0,
		MonthlyCostCritical:   500.0,
		SingleBackupWarning:   10.0,
		SingleBackupCritical:  50.0,
		StorageGrowthWarning:  20.0,
		StorageGrowthCritical: 50.0,
		MinCompressionRatio:   30.0,
	}
}

// CostAnalyzer 备份成本分析器
type CostAnalyzer struct {
	mu              sync.RWMutex
	records         []*BackupCostRecord
	costConfigs     map[CloudProvider]*StorageCostConfig
	trendData       []*CostTrendData
	alertThresholds *CostAlertThresholds
	manager         *Manager
}

// NewCostAnalyzer 创建成本分析器
func NewCostAnalyzer(manager *Manager) *CostAnalyzer {
	return &CostAnalyzer{
		records:         make([]*BackupCostRecord, 0),
		costConfigs:     DefaultStorageCostConfigs(),
		trendData:       make([]*CostTrendData, 0),
		alertThresholds: DefaultCostAlertThresholds(),
		manager:         manager,
	}
}

// CalculateBackupCost 计算单次备份成本
func (ca *CostAnalyzer) CalculateBackupCost(
	config *JobConfig,
	originalSize int64,
	storedSize int64,
	uploadBytes int64,
	requestCount int64,
	duration time.Duration,
) *BackupCostRecord {
	// 获取存储成本配置（需要读锁）
	ca.mu.RLock()
	provider := ca.getProviderFromConfig(config)
	costConfig, ok := ca.costConfigs[provider]
	if !ok {
		costConfig = ca.costConfigs[CloudProviderS3]
	}
	ca.mu.RUnlock()

	storedGB := float64(storedSize) / (1024 * 1024 * 1024)
	storageCost := storedGB * costConfig.StoragePricePerGB / 30

	uploadGB := float64(uploadBytes) / (1024 * 1024 * 1024)
	uploadCost := uploadGB * costConfig.UploadPricePerGB

	requestCost := float64(requestCount) / 10000 * costConfig.RequestPricePer10K

	totalCost := storageCost + uploadCost + requestCost

	compressionRatio := 0.0
	if originalSize > 0 {
		compressionRatio = (1 - float64(storedSize)/float64(originalSize)) * 100
	}

	record := &BackupCostRecord{
		BackupID:         generateID(),
		ConfigID:         config.ID,
		BackupName:       config.Name,
		Provider:         provider,
		Timestamp:        time.Now(),
		StorageCost:      storageCost,
		UploadCost:       uploadCost,
		DownloadCost:     0,
		RequestCost:      requestCost,
		TotalCost:        totalCost,
		OriginalSize:     originalSize,
		StoredSize:       storedSize,
		CompressionRatio: compressionRatio,
		UploadBytes:      uploadBytes,
		DownloadBytes:    0,
		RequestCount:     requestCount,
		BackupType:       config.Type,
		Incremental:      false,
		Duration:         duration,
	}

	ca.mu.Lock()
	ca.records = append(ca.records, record)
	ca.mu.Unlock()

	return record
}

// CalculateRestoreCost 计算恢复操作成本
func (ca *CostAnalyzer) CalculateRestoreCost(
	config *JobConfig,
	downloadBytes int64,
	requestCount int64,
) *BackupCostRecord {
	// 获取存储成本配置（需要读锁）
	ca.mu.RLock()
	provider := ca.getProviderFromConfig(config)
	costConfig, ok := ca.costConfigs[provider]
	if !ok {
		costConfig = ca.costConfigs[CloudProviderS3]
	}
	ca.mu.RUnlock()

	downloadGB := float64(downloadBytes) / (1024 * 1024 * 1024)
	downloadCost := downloadGB * costConfig.DownloadPricePerGB

	requestCost := float64(requestCount) / 10000 * costConfig.RequestPricePer10K

	totalCost := downloadCost + requestCost

	record := &BackupCostRecord{
		BackupID:      generateID(),
		ConfigID:      config.ID,
		BackupName:    config.Name + " (恢复)",
		Provider:      provider,
		Timestamp:     time.Now(),
		StorageCost:   0,
		UploadCost:    0,
		DownloadCost:  downloadCost,
		RequestCost:   requestCost,
		TotalCost:     totalCost,
		DownloadBytes: downloadBytes,
		RequestCount:  requestCount,
		BackupType:    config.Type,
	}

	ca.mu.Lock()
	ca.records = append(ca.records, record)
	ca.mu.Unlock()

	return record
}

func (ca *CostAnalyzer) getProviderFromConfig(config *JobConfig) CloudProvider {
	if config == nil {
		return "local"
	}
	if config.CloudBackup && config.CloudConfig != nil {
		return config.CloudConfig.Provider
	}
	switch config.Type {
	case BackupTypeRemote:
		return CloudProviderS3
	case BackupTypeRsync:
		return "local"
	default:
		return "local"
	}
}

// GetCostTrend 获取成本趋势数据
func (ca *CostAnalyzer) GetCostTrend(days int, period ReportPeriod) ([]*CostTrendData, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	if len(ca.records) == 0 {
		return []*CostTrendData{}, nil
	}

	endTime := time.Now()
	startTime := endTime.AddDate(0, 0, -days)

	aggregated := make(map[string]*CostTrendData)

	for _, record := range ca.records {
		if record.Timestamp.Before(startTime) || record.Timestamp.After(endTime) {
			continue
		}

		key := ca.getAggregationKey(record.Timestamp, period)

		if _, ok := aggregated[key]; !ok {
			aggregated[key] = &CostTrendData{
				Timestamp: record.Timestamp,
			}
		}

		data := aggregated[key]
		data.StorageCost += record.StorageCost
		data.TrafficCost += record.UploadCost + record.DownloadCost
		data.TotalCost += record.TotalCost
		data.TotalStorage += record.StoredSize
		data.BackupCount++
		data.AvgCompressionRatio += record.CompressionRatio
	}

	for _, data := range aggregated {
		if data.BackupCount > 0 {
			data.AvgCompressionRatio /= float64(data.BackupCount)
		}
	}

	result := make([]*CostTrendData, 0, len(aggregated))
	for _, data := range aggregated {
		result = append(result, data)
	}

	ca.sortTrendData(result)

	return result, nil
}

func (ca *CostAnalyzer) getAggregationKey(t time.Time, period ReportPeriod) string {
	switch period {
	case PeriodDaily:
		return t.Format("2006-01-02")
	case PeriodWeekly:
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case PeriodMonthly:
		return t.Format("2006-01")
	case PeriodYearly:
		return t.Format("2006")
	default:
		return t.Format("2006-01-02")
	}
}

func (ca *CostAnalyzer) sortTrendData(data []*CostTrendData) {
	for i := 0; i < len(data)-1; i++ {
		for j := i + 1; j < len(data); j++ {
			if data[i].Timestamp.After(data[j].Timestamp) {
				data[i], data[j] = data[j], data[i]
			}
		}
	}
}

// GenerateCostReport 生成成本分析报告
func (ca *CostAnalyzer) GenerateCostReport(period ReportPeriod) (*CostReport, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	now := time.Now()
	startTime, endTime := ca.calculatePeriodRange(now, period)

	report := &CostReport{
		GeneratedAt: now,
		Period:      period,
		StartTime:   startTime,
		EndTime:     endTime,
	}

	report.Summary = ca.calculateSummary(startTime, endTime)
	report.CostByProvider = ca.calculateCostByProvider(startTime, endTime)

	trendDays := ca.getTrendDays(period)
	trend, _ := ca.GetCostTrend(trendDays, PeriodDaily)
	report.Trend = trend

	report.Recommendations = ca.generateRecommendations(report)
	report.Forecast = ca.generateForecast()
	report.Alerts = ca.checkAlerts(&report.Summary)

	return report, nil
}

func (ca *CostAnalyzer) calculatePeriodRange(now time.Time, period ReportPeriod) (start, end time.Time) {
	end = now
	switch period {
	case PeriodDaily:
		start = now.AddDate(0, 0, -1)
	case PeriodWeekly:
		start = now.AddDate(0, 0, -7)
	case PeriodMonthly:
		start = now.AddDate(0, -1, 0)
	case PeriodYearly:
		start = now.AddDate(-1, 0, 0)
	default:
		start = now.AddDate(0, 0, -7)
	}
	return start, end
}

func (ca *CostAnalyzer) getTrendDays(period ReportPeriod) int {
	switch period {
	case PeriodDaily:
		return 7
	case PeriodWeekly:
		return 14
	case PeriodMonthly:
		return 30
	case PeriodYearly:
		return 365
	default:
		return 30
	}
}

func (ca *CostAnalyzer) calculateSummary(startTime, endTime time.Time) CostSummary {
	summary := CostSummary{}

	for _, record := range ca.records {
		if record.Timestamp.Before(startTime) || record.Timestamp.After(endTime) {
			continue
		}

		summary.TotalCost += record.TotalCost
		summary.StorageCost += record.StorageCost
		summary.UploadCost += record.UploadCost
		summary.DownloadCost += record.DownloadCost
		summary.RequestCost += record.RequestCost
		summary.TotalStorage += record.StoredSize
		summary.BackupCount++

		if record.DownloadBytes > 0 {
			summary.RestoreCount++
		}

		summary.AvgCompressionRatio += record.CompressionRatio
	}

	if summary.BackupCount > 0 {
		summary.AvgCompressionRatio /= float64(summary.BackupCount)
	}

	summary.TotalStorageHuman = humanReadableSize(summary.TotalStorage)

	days := endTime.Sub(startTime).Hours() / 24
	if days > 0 {
		summary.EstimatedMonthlyCost = summary.TotalCost / days * 30
	}

	return summary
}

func (ca *CostAnalyzer) calculateCostByProvider(startTime, endTime time.Time) []ProviderCost {
	providerMap := make(map[CloudProvider]*ProviderCost)

	for _, record := range ca.records {
		if record.Timestamp.Before(startTime) || record.Timestamp.After(endTime) {
			continue
		}

		if _, ok := providerMap[record.Provider]; !ok {
			providerMap[record.Provider] = &ProviderCost{
				Provider: record.Provider,
			}
		}

		pc := providerMap[record.Provider]
		pc.StorageCost += record.StorageCost
		pc.TrafficCost += record.UploadCost + record.DownloadCost
		pc.TotalCost += record.TotalCost
		pc.Storage += record.StoredSize
		pc.BackupCount++
		pc.AvgCompressionRatio += record.CompressionRatio
	}

	var totalCost float64
	var totalStorage int64
	for _, pc := range providerMap {
		totalCost += pc.TotalCost
		totalStorage += pc.Storage
		if pc.BackupCount > 0 {
			pc.AvgCompressionRatio /= float64(pc.BackupCount)
		}
	}

	result := make([]ProviderCost, 0, len(providerMap))
	for _, pc := range providerMap {
		if totalCost > 0 {
			pc.CostPercentage = pc.TotalCost / totalCost * 100
		}
		if totalStorage > 0 {
			pc.StoragePercentage = float64(pc.Storage) / float64(totalStorage) * 100
		}
		result = append(result, *pc)
	}

	return result
}

func (ca *CostAnalyzer) generateRecommendations(report *CostReport) []CostRecommendation {
	recommendations := make([]CostRecommendation, 0)

	if report.Summary.AvgCompressionRatio < ca.alertThresholds.MinCompressionRatio {
		savings := ca.calculateCompressionSavings(report)
		recommendations = append(recommendations, CostRecommendation{
			Type:             RecommendationCompression,
			Priority:         1,
			Title:            "提高压缩效率",
			Description:      fmt.Sprintf("当前平均压缩率 %.1f%%，低于建议值 %.1f%%", report.Summary.AvgCompressionRatio, ca.alertThresholds.MinCompressionRatio),
			PotentialSavings: savings,
			Steps: []string{
				"评估当前压缩算法效率",
				"考虑使用 zstd 算法",
				"调整压缩级别",
			},
			Risk: "更高压缩级别会增加 CPU 使用率",
		})
	}

	fullBackupRatio := ca.calculateFullBackupRatio()
	if fullBackupRatio > 0.5 {
		savings := ca.calculateIncrementalSavings(report, fullBackupRatio)
		recommendations = append(recommendations, CostRecommendation{
			Type:             RecommendationIncremental,
			Priority:         1,
			Title:            "启用增量备份",
			Description:      fmt.Sprintf("当前完整备份占比 %.1f%%", fullBackupRatio*100),
			PotentialSavings: savings,
			Steps: []string{
				"为频繁备份的配置启用增量备份",
				"设置合理的全量备份间隔",
			},
			Risk: "增量备份恢复时间可能较长",
		})
	}

	retentionSavings := ca.calculateRetentionSavings(report)
	if retentionSavings > 0 {
		recommendations = append(recommendations, CostRecommendation{
			Type:             RecommendationRetention,
			Priority:         2,
			Title:            "优化备份保留策略",
			Description:      "部分备份保留时间过长",
			PotentialSavings: retentionSavings,
			Steps: []string{
				"分析备份数据的访问频率",
				"对长期不访问的备份使用归档存储",
			},
			Risk: "可能丢失历史数据",
		})
	}

	for _, pc := range report.CostByProvider {
		if pc.Provider == CloudProviderS3 && pc.CostPercentage > 50 {
			recommendations = append(recommendations, CostRecommendation{
				Type:             RecommendationLocation,
				Priority:         3,
				Title:            "考虑使用低成本存储类型",
				Description:      fmt.Sprintf("%s 存储成本占总成本的 %.1f%%", pc.Provider, pc.CostPercentage),
				PotentialSavings: pc.StorageCost * 0.6,
				Steps: []string{
					"识别不常访问的备份数据",
					"将冷数据迁移到归档存储类",
				},
				Risk: "归档数据恢复时间较长",
			})
		}
	}

	ca.sortRecommendations(recommendations)

	return recommendations
}

func (ca *CostAnalyzer) calculateCompressionSavings(report *CostReport) float64 {
	targetRatio := 50.0
	if report.Summary.AvgCompressionRatio >= targetRatio {
		return 0
	}
	improvement := targetRatio - report.Summary.AvgCompressionRatio
	return report.Summary.StorageCost * improvement / 100
}

func (ca *CostAnalyzer) calculateFullBackupRatio() float64 {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	if len(ca.records) == 0 {
		return 0
	}

	var fullBackupCount int
	for _, record := range ca.records {
		if !record.Incremental {
			fullBackupCount++
		}
	}

	return float64(fullBackupCount) / float64(len(ca.records))
}

func (ca *CostAnalyzer) calculateIncrementalSavings(report *CostReport, fullBackupRatio float64) float64 {
	savingsRatio := 0.7
	excessFullBackups := fullBackupRatio - 0.2
	return report.Summary.StorageCost * excessFullBackups * savingsRatio
}

func (ca *CostAnalyzer) calculateRetentionSavings(report *CostReport) float64 {
	return report.Summary.StorageCost * 0.2
}

func (ca *CostAnalyzer) sortRecommendations(recs []CostRecommendation) {
	for i := 0; i < len(recs)-1; i++ {
		for j := i + 1; j < len(recs); j++ {
			if recs[i].Priority > recs[j].Priority {
				recs[i], recs[j] = recs[j], recs[i]
			}
		}
	}
}

func (ca *CostAnalyzer) generateForecast() *CostForecast {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	if len(ca.records) < 7 {
		return nil
	}

	var totalStorage int64
	var totalCost float64
	var count int

	for _, record := range ca.records {
		totalStorage += record.StoredSize
		totalCost += record.TotalCost
		count++
	}

	avgDailyStorage := float64(totalStorage) / float64(count)
	avgDailyCost := totalCost / float64(count)

	growthRate := 1.1
	monthlyGrowthRate := math.Pow(growthRate, 1.0/30.0)

	projectedStorage := int64(avgDailyStorage * math.Pow(monthlyGrowthRate, 30))
	projectedDailyCost := avgDailyCost * math.Pow(monthlyGrowthRate, 30)
	projectedMonthlyCost := projectedDailyCost * 30

	confidence := math.Min(float64(count)/30.0*100, 85.0)

	return &CostForecast{
		Date:                 time.Now().AddDate(0, 0, 30),
		ProjectedStorage:     projectedStorage,
		ProjectedDailyCost:   projectedDailyCost,
		ProjectedMonthlyCost: projectedMonthlyCost,
		Confidence:           confidence,
		Basis:                "基于过去 30 天数据的线性预测",
	}
}

func (ca *CostAnalyzer) checkAlerts(summary *CostSummary) []CostAlert {
	alerts := make([]CostAlert, 0)
	now := time.Now()

	if summary.EstimatedMonthlyCost >= ca.alertThresholds.MonthlyCostCritical {
		alerts = append(alerts, CostAlert{
			Level:        AlertLevelCritical,
			Type:         "monthly_cost",
			Message:      fmt.Sprintf("预计月度成本 %.2f 元，超过严重阈值 %.2f 元", summary.EstimatedMonthlyCost, ca.alertThresholds.MonthlyCostCritical),
			CurrentValue: summary.EstimatedMonthlyCost,
			Threshold:    ca.alertThresholds.MonthlyCostCritical,
			Timestamp:    now,
		})
	} else if summary.EstimatedMonthlyCost >= ca.alertThresholds.MonthlyCostWarning {
		alerts = append(alerts, CostAlert{
			Level:        AlertLevelWarning,
			Type:         "monthly_cost",
			Message:      fmt.Sprintf("预计月度成本 %.2f 元，超过警告阈值 %.2f 元", summary.EstimatedMonthlyCost, ca.alertThresholds.MonthlyCostWarning),
			CurrentValue: summary.EstimatedMonthlyCost,
			Threshold:    ca.alertThresholds.MonthlyCostWarning,
			Timestamp:    now,
		})
	}

	for _, record := range ca.records {
		if record.Timestamp.After(now.Add(-24 * time.Hour)) {
			if record.TotalCost >= ca.alertThresholds.SingleBackupCritical {
				alerts = append(alerts, CostAlert{
					Level:        AlertLevelCritical,
					Type:         "single_backup_cost",
					Message:      fmt.Sprintf("备份 %s 成本 %.2f 元，超过严重阈值", record.BackupName, record.TotalCost),
					CurrentValue: record.TotalCost,
					Threshold:    ca.alertThresholds.SingleBackupCritical,
					Timestamp:    record.Timestamp,
				})
			} else if record.TotalCost >= ca.alertThresholds.SingleBackupWarning {
				alerts = append(alerts, CostAlert{
					Level:        AlertLevelWarning,
					Type:         "single_backup_cost",
					Message:      fmt.Sprintf("备份 %s 成本 %.2f 元，超过警告阈值", record.BackupName, record.TotalCost),
					CurrentValue: record.TotalCost,
					Threshold:    ca.alertThresholds.SingleBackupWarning,
					Timestamp:    record.Timestamp,
				})
			}
		}
	}

	if summary.AvgCompressionRatio < ca.alertThresholds.MinCompressionRatio {
		alerts = append(alerts, CostAlert{
			Level:        AlertLevelWarning,
			Type:         "compression_ratio",
			Message:      fmt.Sprintf("平均压缩率 %.1f%%，低于建议值 %.1f%%", summary.AvgCompressionRatio, ca.alertThresholds.MinCompressionRatio),
			CurrentValue: summary.AvgCompressionRatio,
			Threshold:    ca.alertThresholds.MinCompressionRatio,
			Timestamp:    now,
		})
	}

	return alerts
}

// SetCostConfig 设置存储成本配置
func (ca *CostAnalyzer) SetCostConfig(provider CloudProvider, config *StorageCostConfig) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.costConfigs[provider] = config
}

// SetAlertThresholds 设置告警阈值
func (ca *CostAnalyzer) SetAlertThresholds(thresholds *CostAlertThresholds) {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.alertThresholds = thresholds
}

// GetRecords 获取成本记录
func (ca *CostAnalyzer) GetRecords(limit int) []*BackupCostRecord {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	if limit <= 0 || limit > len(ca.records) {
		limit = len(ca.records)
	}

	start := len(ca.records) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*BackupCostRecord, limit)
	copy(result, ca.records[start:])
	return result
}

// GetCurrentStorageCost 获取当前存储成本（按月）
func (ca *CostAnalyzer) GetCurrentStorageCost() float64 {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	var totalStorage int64
	for _, record := range ca.records {
		if record.Timestamp.After(time.Now().AddDate(0, 0, -30)) {
			totalStorage += record.StoredSize
		}
	}

	config := ca.costConfigs[CloudProviderS3]
	storedGB := float64(totalStorage) / (1024 * 1024 * 1024)
	return storedGB * config.StoragePricePerGB
}

// OptimizeRequest 优化建议请求参数
type OptimizeRequest struct {
	ConfigIDs    []string `json:"configIds,omitempty"`
	IncludeSteps bool     `json:"includeSteps"`
	BudgetLimit  float64  `json:"budgetLimit,omitempty"`
	OptimizeGoal string   `json:"optimizeGoal"`
}

// OptimizeResponse 优化建议响应
type OptimizeResponse struct {
	CurrentCost         float64              `json:"currentCost"`
	OptimizedCost       float64              `json:"optimizedCost"`
	PotentialSavings    float64              `json:"potentialSavings"`
	SavingsRatio        float64              `json:"savingsRatio"`
	Recommendations     []CostRecommendation `json:"recommendations"`
	ImplementationOrder []string             `json:"implementationOrder"`
}

// GetOptimizationSuggestions 获取优化建议
func (ca *CostAnalyzer) GetOptimizationSuggestions(req *OptimizeRequest) (*OptimizeResponse, error) {
	report, err := ca.GenerateCostReport(PeriodMonthly)
	if err != nil {
		return nil, err
	}

	response := &OptimizeResponse{
		CurrentCost:     report.Summary.TotalCost,
		Recommendations: report.Recommendations,
	}

	var totalSavings float64
	for _, rec := range report.Recommendations {
		totalSavings += rec.PotentialSavings
	}

	response.PotentialSavings = totalSavings
	response.OptimizedCost = response.CurrentCost - totalSavings
	if response.CurrentCost > 0 {
		response.SavingsRatio = totalSavings / response.CurrentCost * 100
	}

	response.ImplementationOrder = []string{
		"1. 启用/优化增量备份策略",
		"2. 调整压缩算法和级别",
		"3. 实施分层保留策略",
		"4. 迁移冷数据到低成本存储",
		"5. 定期审查和清理无用备份",
	}

	if req.BudgetLimit > 0 && response.OptimizedCost > req.BudgetLimit {
		response.Recommendations = append(response.Recommendations, CostRecommendation{
			Type:             RecommendationRetention,
			Priority:         0,
			Title:            "预算超支警告",
			Description:      fmt.Sprintf("优化后成本 %.2f 元仍超过预算 %.2f 元", response.OptimizedCost, req.BudgetLimit),
			PotentialSavings: response.OptimizedCost - req.BudgetLimit,
			Risk:             "可能需要删除部分历史备份或降低备份频率",
		})
	}

	return response, nil
}
