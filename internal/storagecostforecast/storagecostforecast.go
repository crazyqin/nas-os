// Package storagecostforecast 提供存储成本预测引擎
// 基于历史增长趋势预测未来存储成本，支持多云成本对比、TCO计算、
// 成本优化建议、预算告警、趋势图表数据生成和ROI分析
package storagecostforecast

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

// CloudProvider 云服务提供商类型.
type CloudProvider string

const (
	// ProviderAWS 亚马逊云 S3.
	ProviderAWS CloudProvider = "aws_s3"
	// ProviderAzure 微软云 Azure Blob Storage.
	ProviderAzure CloudProvider = "azure_blob"
	// ProviderGCP 谷歌云 Cloud Storage.
	ProviderGCP CloudProvider = "google_cloud_storage"
	// ProviderAlibaba 阿里云 OSS.
	ProviderAlibaba CloudProvider = "alibaba_oss"
	// ProviderLocal 本地存储.
	ProviderLocal CloudProvider = "local_storage"
)

// StorageTier 存储层级.
type StorageTier string

const (
	// TierHot 热存储 - 频繁访问.
	TierHot StorageTier = "hot"
	// TierWarm 温存储 - 偶尔访问.
	TierWarm StorageTier = "warm"
	// TierCold 冷存储 - 归档.
	TierCold StorageTier = "cold"
	// TierArchive 归档存储 - 极少访问.
	TierArchive StorageTier = "archive"
)

// CostRecord 历史成本记录.
type CostRecord struct {
	Timestamp    time.Time     `json:"timestamp"`     // 记录时间戳
	StorageGB    float64       `json:"storage_gb"`    // 存储容量(GB)
	MonthlyCost  float64       `json:"monthly_cost"`  // 月度成本(元)
	Provider     CloudProvider `json:"provider"`      // 云服务商
	Tier         StorageTier   `json:"tier"`          // 存储层级
	BandwidthGB  float64       `json:"bandwidth_gb"`  // 带宽使用(GB)
	RequestCount int64         `json:"request_count"` // 请求次数
}

// ForecastResult 预测结果.
type ForecastResult struct {
	Timestamp       time.Time     `json:"timestamp"`        // 预测时间点
	PredictedGB     float64       `json:"predicted_gb"`     // 预测存储容量(GB)
	PredictedCost   float64       `json:"predicted_cost"`   // 预测月度成本(元)
	ConfidenceLevel float64       `json:"confidence_level"` // 置信度(0-1)
	Provider        CloudProvider `json:"provider"`         // 云服务商
	GrowthRate      float64       `json:"growth_rate"`      // 月增长率
}

// CloudCostComparison 多云成本对比结果.
type CloudCostComparison struct {
	Provider      CloudProvider `json:"provider"`       // 云服务商
	StorageCost   float64       `json:"storage_cost"`   // 存储成本(元)
	BandwidthCost float64       `json:"bandwidth_cost"` // 带宽成本(元)
	RequestCost   float64       `json:"request_cost"`   // 请求成本(元)
	TotalCost     float64       `json:"total_cost"`     // 总成本(元)
	Rank          int           `json:"rank"`           // 排名(1最便宜)
}

// TCOResult 总拥有成本计算结果.
type TCOResult struct {
	HardwareCost    float64 `json:"hardware_cost"`    // 硬件成本(元)
	ElectricityCost float64 `json:"electricity_cost"` // 电力成本(元)
	MaintenanceCost float64 `json:"maintenance_cost"` // 运维成本(元)
	BandwidthCost   float64 `json:"bandwidth_cost"`   // 带宽成本(元)
	TotalCost       float64 `json:"total_cost"`       // 总成本(元)
	MonthlyCost     float64 `json:"monthly_cost"`     // 月均成本(元)
	DurationMonths  int     `json:"duration_months"`  // 预测周期(月)
}

// OptimizationAdvice 成本优化建议.
type OptimizationAdvice struct {
	Category    string  `json:"category"`    // 建议类别
	Description string  `json:"description"` // 建议描述
	Savings     float64 `json:"savings"`     // 预计节省(元/月)
	Priority    int     `json:"priority"`    // 优先级(1最高)
}

// BudgetAlert 预算告警.
type BudgetAlert struct {
	Timestamp   time.Time `json:"timestamp"`    // 告警时间
	BudgetLimit float64   `json:"budget_limit"` // 预算上限(元)
	Predicted   float64   `json:"predicted"`    // 预测成本(元)
	Excess      float64   `json:"excess"`       // 超出金额(元)
	Level       string    `json:"level"`        // 告警级别: warning/critical
	Message     string    `json:"message"`      // 告警消息
}

// TrendDataPoint 趋势图表数据点.
type TrendDataPoint struct {
	Timestamp time.Time `json:"timestamp"`  // 时间点
	Actual    float64   `json:"actual"`     // 实际成本(元)
	Predicted float64   `json:"predicted"`  // 预测成本(元)
	StorageGB float64   `json:"storage_gb"` // 存储容量(GB)
}

// ROIResult 投资回报率分析结果.
type ROIResult struct {
	InvestmentCost   float64 `json:"investment_cost"`    // 投资成本(元)
	AnnualSavings    float64 `json:"annual_savings"`     // 年度节省(元)
	ROI              float64 `json:"roi"`                // 投资回报率(%)
	PaybackMonths    float64 `json:"payback_months"`     // 回本周期(月)
	ThreeYearSavings float64 `json:"three_year_savings"` // 三年累计节省(元)
}

// PriceConfig 云服务价格配置.
type PriceConfig struct {
	StoragePerGB   float64 // 每GB存储月费(元)
	BandwidthPerGB float64 // 每GB带宽费(元)
	RequestPer10K  float64 // 每万次请求费(元)
	Tier           StorageTier
}

// CostForecastEngine 存储成本预测引擎
// 提供存储成本预测、多云对比、TCO计算、优化建议等核心功能.
type CostForecastEngine struct {
	mu               sync.Mutex                                    // 并发保护锁
	records          []CostRecord                                  // 历史成本记录
	budgetLimit      float64                                       // 月度预算上限(元)
	budgetAlerts     []BudgetAlert                                 // 预算告警历史
	priceConfigs     map[CloudProvider]map[StorageTier]PriceConfig // 云服务价格配置
	running          bool                                          // 引擎运行状态
	cancel           context.CancelFunc                            // 取消函数
	predictionMonths int                                           // 预测月数
	alertCallback    func(BudgetAlert)                             // 告警回调函数
}

// init 模块初始化，注册存储成本预测引擎.
func init() {
	log.Println("[storagecostforecast] 存储成本预测引擎模块已加载")
}

// New 创建新的存储成本预测引擎实例
// 返回初始化完成的 CostForecastEngine 指针.
func New() *CostForecastEngine {
	engine := &CostForecastEngine{
		records:          make([]CostRecord, 0),
		budgetAlerts:     make([]BudgetAlert, 0),
		priceConfigs:     initDefaultPrices(),
		predictionMonths: 12, // 默认预测12个月
	}
	return engine
}

// initDefaultPrices 初始化默认云服务价格配置
// 返回各云服务商各存储层级的价格配置映射.
func initDefaultPrices() map[CloudProvider]map[StorageTier]PriceConfig {
	configs := make(map[CloudProvider]map[StorageTier]PriceConfig)

	// AWS S3 价格配置 (人民币估算)
	configs[ProviderAWS] = map[StorageTier]PriceConfig{
		TierHot:     {StoragePerGB: 0.18, BandwidthPerGB: 0.72, RequestPer10K: 0.036},
		TierWarm:    {StoragePerGB: 0.10, BandwidthPerGB: 0.72, RequestPer10K: 0.08},
		TierCold:    {StoragePerGB: 0.03, BandwidthPerGB: 0.72, RequestPer10K: 0.08},
		TierArchive: {StoragePerGB: 0.015, BandwidthPerGB: 0.55, RequestPer10K: 0.25},
	}

	// Azure Blob Storage 价格配置
	configs[ProviderAzure] = map[StorageTier]PriceConfig{
		TierHot:     {StoragePerGB: 0.15, BandwidthPerGB: 0.65, RequestPer10K: 0.035},
		TierWarm:    {StoragePerGB: 0.08, BandwidthPerGB: 0.65, RequestPer10K: 0.07},
		TierCold:    {StoragePerGB: 0.02, BandwidthPerGB: 0.65, RequestPer10K: 0.08},
		TierArchive: {StoragePerGB: 0.012, BandwidthPerGB: 0.50, RequestPer10K: 0.22},
	}

	// Google Cloud Storage 价格配置
	configs[ProviderGCP] = map[StorageTier]PriceConfig{
		TierHot:     {StoragePerGB: 0.17, BandwidthPerGB: 0.70, RequestPer10K: 0.035},
		TierWarm:    {StoragePerGB: 0.09, BandwidthPerGB: 0.70, RequestPer10K: 0.075},
		TierCold:    {StoragePerGB: 0.025, BandwidthPerGB: 0.70, RequestPer10K: 0.08},
		TierArchive: {StoragePerGB: 0.013, BandwidthPerGB: 0.50, RequestPer10K: 0.23},
	}

	// 阿里云 OSS 价格配置
	configs[ProviderAlibaba] = map[StorageTier]PriceConfig{
		TierHot:     {StoragePerGB: 0.12, BandwidthPerGB: 0.50, RequestPer10K: 0.01},
		TierWarm:    {StoragePerGB: 0.08, BandwidthPerGB: 0.50, RequestPer10K: 0.02},
		TierCold:    {StoragePerGB: 0.033, BandwidthPerGB: 0.50, RequestPer10K: 0.05},
		TierArchive: {StoragePerGB: 0.015, BandwidthPerGB: 0.50, RequestPer10K: 0.10},
	}

	return configs
}

// SetBudgetLimit 设置月度预算上限
// limit: 预算金额(元)
func (e *CostForecastEngine) SetBudgetLimit(limit float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.budgetLimit = limit
	log.Printf("[storagecostforecast] 预算上限已设置: %.2f 元/月\n", limit)
}

// SetAlertCallback 设置告警回调函数
// callback: 当触发预算告警时调用的回调函数
func (e *CostForecastEngine) SetAlertCallback(callback func(BudgetAlert)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.alertCallback = callback
}

// SetPredictionMonths 设置预测月数
// months: 预测月数(1-60)
func (e *CostForecastEngine) SetPredictionMonths(months int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if months < 1 {
		months = 1
	}
	if months > 60 {
		months = 60
	}
	e.predictionMonths = months
	log.Printf("[storagecostforecast] 预测周期已设置: %d 个月\n", months)
}

// AddRecord 添加历史成本记录
// record: 成本记录数据
func (e *CostForecastEngine) AddRecord(record CostRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.records = append(e.records, record)
	log.Printf("[storagecostforecast] 添加成本记录: %s %.2fGB %.2f元\n",
		record.Provider, record.StorageGB, record.MonthlyCost)
}

// AddRecords 批量添加历史成本记录
// records: 成本记录切片
func (e *CostForecastEngine) AddRecords(records []CostRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.records = append(e.records, records...)
	log.Printf("[storagecostforecast] 批量添加 %d 条成本记录\n", len(records))
}

// Start 启动成本预测引擎
// 启动后台预测任务和告警监控.
func (e *CostForecastEngine) Start() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		log.Println("[storagecostforecast] 引擎已在运行中")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.running = true

	// 启动后台任务
	go e.runPredictionLoop(ctx)

	log.Println("[storagecostforecast] 成本预测引擎已启动")
}

// Stop 停止成本预测引擎.
func (e *CostForecastEngine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		log.Println("[storagecostforecast] 引擎未运行")
		return
	}

	if e.cancel != nil {
		e.cancel()
	}
	e.running = false
	log.Println("[storagecostforecast] 成本预测引擎已停止")
}

// IsRunning 检查引擎是否运行中.
func (e *CostForecastEngine) IsRunning() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.running
}

// GetForecast 获取存储成本预测
// provider: 云服务商
// tier: 存储层级
// 返回预测结果切片，按时间排序.
func (e *CostForecastEngine) GetForecast(provider CloudProvider, tier StorageTier) []ForecastResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 过滤指定云服务商和存储层级的记录
	filtered := e.filterRecords(provider, tier)
	if len(filtered) < 2 {
		log.Println("[storagecostforecast] 历史数据不足，无法进行预测")
		return []ForecastResult{}
	}

	// 计算线性回归参数
	growthRate, baseCost, baseGB := e.calculateLinearRegression(filtered)

	// 生成预测结果
	results := make([]ForecastResult, 0, e.predictionMonths)
	now := time.Now()

	for i := 1; i <= e.predictionMonths; i++ {
		futureTime := now.AddDate(0, i, 0)
		monthsAhead := float64(i)

		predictedGB := baseGB * math.Pow(1+growthRate, monthsAhead)
		predictedCost := baseCost * math.Pow(1+growthRate, monthsAhead)

		// 置信度随预测时间衰减
		confidence := math.Max(0.5, 1.0-float64(i)*0.03)

		results = append(results, ForecastResult{
			Timestamp:       futureTime,
			PredictedGB:     math.Round(predictedGB*100) / 100,
			PredictedCost:   math.Round(predictedCost*100) / 100,
			ConfidenceLevel: math.Round(confidence*100) / 100,
			Provider:        provider,
			GrowthRate:      math.Round(growthRate*10000) / 100,
		})
	}

	log.Printf("[storagecostforecast] 生成 %s %s 预测结果 %d 条\n", provider, tier, len(results))
	return results
}

// GetMultiCloudComparison 多云成本对比
// storageGB: 当前存储容量(GB)
// bandwidthGB: 月带宽使用量(GB)
// requestCount: 月请求次数
// tier: 存储层级
// 返回各云服务商成本对比结果，按总成本升序排列.
func (e *CostForecastEngine) GetMultiCloudComparison(storageGB, bandwidthGB float64, requestCount int64, tier StorageTier) []CloudCostComparison {
	e.mu.Lock()
	defer e.mu.Unlock()

	comparisons := make([]CloudCostComparison, 0)
	providers := []CloudProvider{ProviderAWS, ProviderAzure, ProviderGCP, ProviderAlibaba}

	for _, provider := range providers {
		config, exists := e.priceConfigs[provider][tier]
		if !exists {
			continue
		}

		storageCost := storageGB * config.StoragePerGB
		bandwidthCost := bandwidthGB * config.BandwidthPerGB
		requestCost := float64(requestCount) / 10000 * config.RequestPer10K
		totalCost := storageCost + bandwidthCost + requestCost

		comparisons = append(comparisons, CloudCostComparison{
			Provider:      provider,
			StorageCost:   math.Round(storageCost*100) / 100,
			BandwidthCost: math.Round(bandwidthCost*100) / 100,
			RequestCost:   math.Round(requestCost*100) / 100,
			TotalCost:     math.Round(totalCost*100) / 100,
		})
	}

	// 按总成本排序
	sort.Slice(comparisons, func(i, j int) bool {
		return comparisons[i].TotalCost < comparisons[j].TotalCost
	})

	// 设置排名
	for i := range comparisons {
		comparisons[i].Rank = i + 1
	}

	log.Printf("[storagecostforecast] 生成多云对比结果 %d 条\n", len(comparisons))
	return comparisons
}

// CalculateTCO 计算总拥有成本
// storageGB: 存储容量(GB)
// durationMonths: 预测周期(月)
// 返回本地存储的TCO计算结果.
func (e *CostForecastEngine) CalculateTCO(storageGB float64, durationMonths int) TCOResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 硬件成本估算: 每TB约2000元
	tbSize := storageGB / 1024
	hardwareCost := tbSize * 2000

	// 电力成本: 每TB每月约50元
	electricityCost := tbSize * 50 * float64(durationMonths)

	// 运维成本: 固定每月500元
	maintenanceCost := 500 * float64(durationMonths)

	// 带宽成本: 假设每月传输存储量10%的数据，每GB 0.5元
	bandwidthMonthly := storageGB * 0.1 * 0.5
	bandwidthCost := bandwidthMonthly * float64(durationMonths)

	totalCost := hardwareCost + electricityCost + maintenanceCost + bandwidthCost
	monthlyCost := totalCost / float64(durationMonths)

	result := TCOResult{
		HardwareCost:    math.Round(hardwareCost*100) / 100,
		ElectricityCost: math.Round(electricityCost*100) / 100,
		MaintenanceCost: math.Round(maintenanceCost*100) / 100,
		BandwidthCost:   math.Round(bandwidthCost*100) / 100,
		TotalCost:       math.Round(totalCost*100) / 100,
		MonthlyCost:     math.Round(monthlyCost*100) / 100,
		DurationMonths:  durationMonths,
	}

	log.Printf("[storagecostforecast] TCO计算完成: %.2fGB %d个月 总计%.2f元\n",
		storageGB, durationMonths, totalCost)
	return result
}

// GetOptimizationAdvice 获取成本优化建议
// provider: 当前使用的云服务商
// tier: 当前存储层级
// storageGB: 当前存储容量(GB)
// 返回优化建议列表，按优先级排序.
func (e *CostForecastEngine) GetOptimizationAdvice(provider CloudProvider, tier StorageTier, storageGB float64) []OptimizationAdvice {
	e.mu.Lock()
	defer e.mu.Unlock()

	advice := make([]OptimizationAdvice, 0)

	// 建议1: 存储层级优化
	if tier == TierHot && storageGB > 500 {
		savings := storageGB * 0.05 * 30 // 假设迁移到Warm节省5分/GB
		advice = append(advice, OptimizationAdvice{
			Category:    "存储层级优化",
			Description: fmt.Sprintf("建议将 %.0fGB 数据从热存储迁移到温存储，可显著降低成本", storageGB*0.3),
			Savings:     math.Round(savings*100) / 100,
			Priority:    1,
		})
	}

	if tier != TierArchive && storageGB > 1000 {
		coldSavings := storageGB * 0.10 * 30
		advice = append(advice, OptimizationAdvice{
			Category:    "冷数据归档",
			Description: "超过1TB的数据中，建议将不常访问的数据归档到冷存储",
			Savings:     math.Round(coldSavings*100) / 100,
			Priority:    2,
		})
	}

	// 建议2: 多云对比
	cheapestProvider, cheapestPrice := e.findCheapestProvider(tier)
	if cheapestProvider != provider && cheapestProvider != "" {
		currentPrice := e.getProviderPrice(provider, tier)
		savings := (currentPrice - cheapestPrice) * storageGB
		if savings > 0 {
			advice = append(advice, OptimizationAdvice{
				Category:    "云服务商切换",
				Description: fmt.Sprintf("切换到 %s 可降低存储单价", cheapestProvider),
				Savings:     math.Round(savings*100) / 100,
				Priority:    3,
			})
		}
	}

	// 建议3: 数据压缩和去重
	if storageGB > 100 {
		compressionSavings := storageGB * 0.15 * e.getProviderPrice(provider, tier) // 假设15%压缩率
		advice = append(advice, OptimizationAdvice{
			Category:    "数据压缩去重",
			Description: "启用数据压缩和去重功能，可减少约15%的存储占用",
			Savings:     math.Round(compressionSavings*100) / 100,
			Priority:    4,
		})
	}

	// 建议4: 生命周期管理
	advice = append(advice, OptimizationAdvice{
		Category:    "生命周期管理",
		Description: "配置自动生命周期策略，定期将旧数据迁移到低成本存储层",
		Savings:     math.Round(storageGB*0.02*30*100) / 100,
		Priority:    5,
	})

	// 建议5: 预留容量
	if storageGB > 5000 {
		reservedSavings := storageGB * 0.03 * 30 // 预留容量折扣约30%
		advice = append(advice, OptimizationAdvice{
			Category:    "预留容量购买",
			Description: "购买预留容量计划可获得显著折扣，适合稳定增长的存储需求",
			Savings:     math.Round(reservedSavings*100) / 100,
			Priority:    6,
		})
	}

	// 按优先级排序
	sort.Slice(advice, func(i, j int) bool {
		return advice[i].Priority < advice[j].Priority
	})

	log.Printf("[storagecostforecast] 生成优化建议 %d 条\n", len(advice))
	return advice
}

// GetBudgetAlerts 获取预算告警历史
// 返回告警列表，按时间降序排列.
func (e *CostForecastEngine) GetBudgetAlerts() []BudgetAlert {
	e.mu.Lock()
	defer e.mu.Unlock()

	alerts := make([]BudgetAlert, len(e.budgetAlerts))
	copy(alerts, e.budgetAlerts)

	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].Timestamp.After(alerts[j].Timestamp)
	})

	return alerts
}

// GenerateTrendData 生成成本趋势图表数据
// provider: 云服务商
// tier: 存储层级
// 返回趋势数据点列表.
func (e *CostForecastEngine) GenerateTrendData(provider CloudProvider, tier StorageTier) []TrendDataPoint {
	e.mu.Lock()
	defer e.mu.Unlock()

	dataPoints := make([]TrendDataPoint, 0)

	// 添加历史数据点
	for _, record := range e.records {
		if record.Provider == provider && record.Tier == tier {
			dataPoints = append(dataPoints, TrendDataPoint{
				Timestamp: record.Timestamp,
				Actual:    record.MonthlyCost,
				StorageGB: record.StorageGB,
			})
		}
	}

	// 生成预测数据点
	filtered := e.filterRecords(provider, tier)
	if len(filtered) >= 2 {
		growthRate, baseCost, baseGB := e.calculateLinearRegression(filtered)
		lastRecord := filtered[len(filtered)-1]

		for i := 1; i <= e.predictionMonths; i++ {
			futureTime := lastRecord.Timestamp.AddDate(0, i, 0)
			monthsAhead := float64(i)

			predictedCost := baseCost * math.Pow(1+growthRate, monthsAhead)
			predictedGB := baseGB * math.Pow(1+growthRate, monthsAhead)

			dataPoints = append(dataPoints, TrendDataPoint{
				Timestamp: futureTime,
				Predicted: math.Round(predictedCost*100) / 100,
				StorageGB: math.Round(predictedGB*100) / 100,
			})
		}
	}

	// 按时间排序
	sort.Slice(dataPoints, func(i, j int) bool {
		return dataPoints[i].Timestamp.Before(dataPoints[j].Timestamp)
	})

	log.Printf("[storagecostforecast] 生成趋势数据点 %d 个\n", len(dataPoints))
	return dataPoints
}

// CalculateROI 计算投资回报率
// currentMonthlyCost: 当前月成本(元)
// optimizedMonthlyCost: 优化后月成本(元)
// investmentCost: 优化投资成本(元)
// 返回ROI分析结果.
func (e *CostForecastEngine) CalculateROI(currentMonthlyCost, optimizedMonthlyCost, investmentCost float64) ROIResult {
	e.mu.Lock()
	defer e.mu.Unlock()

	monthlySavings := currentMonthlyCost - optimizedMonthlyCost
	annualSavings := monthlySavings * 12
	roi := (annualSavings / investmentCost) * 100
	paybackMonths := investmentCost / monthlySavings
	threeYearSavings := annualSavings*3 - investmentCost

	result := ROIResult{
		InvestmentCost:   math.Round(investmentCost*100) / 100,
		AnnualSavings:    math.Round(annualSavings*100) / 100,
		ROI:              math.Round(roi*100) / 100,
		PaybackMonths:    math.Round(paybackMonths*10) / 10,
		ThreeYearSavings: math.Round(threeYearSavings*100) / 100,
	}

	log.Printf("[storagecostforecast] ROI分析完成: 投资回报率 %.2f%%, 回本周期 %.1f 月\n",
		roi, paybackMonths)
	return result
}

// filterRecords 过滤指定条件的历史记录
// provider: 云服务商
// tier: 存储层级
// 返回过滤后的记录切片.
func (e *CostForecastEngine) filterRecords(provider CloudProvider, tier StorageTier) []CostRecord {
	filtered := make([]CostRecord, 0)
	for _, record := range e.records {
		if record.Provider == provider && record.Tier == tier {
			filtered = append(filtered, record)
		}
	}

	// 按时间排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.Before(filtered[j].Timestamp)
	})

	return filtered
}

// calculateLinearRegression 计算线性回归参数
// records: 历史记录（已按时间排序）
// 返回增长率、基础成本、基础存储容量.
func (e *CostForecastEngine) calculateLinearRegression(records []CostRecord) (growthRate, baseCost, baseGB float64) {
	n := float64(len(records))
	if n < 2 {
		return 0, 0, 0
	}

	// 使用对数线性回归计算增长率
	var sumX, sumY, sumXY, sumX2 float64
	baseTime := records[0].Timestamp

	for i, record := range records {
		x := record.Timestamp.Sub(baseTime).Hours() / 720 // 以月为单位
		y := math.Log(record.MonthlyCost)

		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x

		if i == 0 {
			baseCost = record.MonthlyCost
			baseGB = record.StorageGB
		}
	}

	// 计算斜率（月增长率）
	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	growthRate = math.Exp(slope) - 1

	return growthRate, baseCost, baseGB
}

// findCheapestProvider 找到指定存储层级最便宜的云服务商
// tier: 存储层级
// 返回最便宜的云服务商和每GB存储价格.
func (e *CostForecastEngine) findCheapestProvider(tier StorageTier) (CloudProvider, float64) {
	var cheapestProvider CloudProvider
	var cheapestPrice = math.MaxFloat64

	for provider, tierConfigs := range e.priceConfigs {
		if config, exists := tierConfigs[tier]; exists {
			if config.StoragePerGB < cheapestPrice {
				cheapestPrice = config.StoragePerGB
				cheapestProvider = provider
			}
		}
	}

	return cheapestProvider, cheapestPrice
}

// getProviderPrice 获取指定云服务商存储层级的价格
// provider: 云服务商
// tier: 存储层级
// 返回每GB存储价格.
func (e *CostForecastEngine) getProviderPrice(provider CloudProvider, tier StorageTier) float64 {
	if tierConfigs, exists := e.priceConfigs[provider]; exists {
		if config, exists := tierConfigs[tier]; exists {
			return config.StoragePerGB
		}
	}
	return 0
}

// checkBudgetAlert 检查预算告警
// predictedCost: 预测成本(元).
func (e *CostForecastEngine) checkBudgetAlert(predictedCost float64) {
	if e.budgetLimit <= 0 {
		return
	}

	excess := predictedCost - e.budgetLimit
	if excess > 0 {
		level := "warning"
		if excess > e.budgetLimit*0.2 {
			level = "critical"
		}

		alert := BudgetAlert{
			Timestamp:   time.Now(),
			BudgetLimit: e.budgetLimit,
			Predicted:   predictedCost,
			Excess:      math.Round(excess*100) / 100,
			Level:       level,
			Message:     fmt.Sprintf("预测月度成本 %.2f 元超出预算 %.2f 元，超出 %.2f 元", predictedCost, e.budgetLimit, excess),
		}

		e.budgetAlerts = append(e.budgetAlerts, alert)
		log.Printf("[storagecostforecast] 预算告警: %s\n", alert.Message)

		// 调用告警回调
		if e.alertCallback != nil {
			go e.alertCallback(alert)
		}
	}
}

// runPredictionLoop 后台预测任务循环
// ctx: 上下文用于取消控制
func (e *CostForecastEngine) runPredictionLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour) // 每小时执行一次预测检查
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[storagecostforecast] 预测任务循环已停止")
			return
		case <-ticker.C:
			e.runBudgetCheck()
		}
	}
}

// runBudgetCheck 执行预算检查
// 扫描所有云服务商的预测成本，检查是否超出预算.
func (e *CostForecastEngine) runBudgetCheck() {
	e.mu.Lock()
	defer e.mu.Unlock()

	providers := []CloudProvider{ProviderAWS, ProviderAzure, ProviderGCP, ProviderAlibaba, ProviderLocal}
	tiers := []StorageTier{TierHot, TierWarm, TierCold, TierArchive}

	for _, provider := range providers {
		for _, tier := range tiers {
			filtered := e.filterRecords(provider, tier)
			if len(filtered) < 2 {
				continue
			}

			growthRate, baseCost, _ := e.calculateLinearRegression(filtered)
			predictedCost := baseCost * (1 + growthRate) // 下个月预测

			if predictedCost > e.budgetLimit {
				e.checkBudgetAlert(predictedCost)
			}
		}
	}
}

// GetRecordCount 获取历史记录数量.
func (e *CostForecastEngine) GetRecordCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.records)
}

// ClearRecords 清空历史记录.
func (e *CostForecastEngine) ClearRecords() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.records = make([]CostRecord, 0)
	e.budgetAlerts = make([]BudgetAlert, 0)
	log.Println("[storagecostforecast] 历史记录已清空")
}

// UpdatePriceConfig 更新云服务商价格配置
// provider: 云服务商
// tier: 存储层级
// config: 新的价格配置
func (e *CostForecastEngine) UpdatePriceConfig(provider CloudProvider, tier StorageTier, config PriceConfig) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.priceConfigs[provider]; !exists {
		e.priceConfigs[provider] = make(map[StorageTier]PriceConfig)
	}
	e.priceConfigs[provider][tier] = config
	log.Printf("[storagecostforecast] 更新价格配置: %s %s\n", provider, tier)
}

// GetPriceConfig 获取云服务商价格配置
// provider: 云服务商
// tier: 存储层级
// 返回价格配置和是否存在标志.
func (e *CostForecastEngine) GetPriceConfig(provider CloudProvider, tier StorageTier) (PriceConfig, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if tierConfigs, exists := e.priceConfigs[provider]; exists {
		if config, exists := tierConfigs[tier]; exists {
			return config, true
		}
	}
	return PriceConfig{}, false
}
