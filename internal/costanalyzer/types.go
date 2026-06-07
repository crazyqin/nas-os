// Package costanalyzer 存储成本分析器 - 计算、分析和优化存储成本
package costanalyzer

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sync"
	"time"
)

// StorageType 存储类型
type StorageType string

const (
	StorageSSD    StorageType = "ssd"
	StorageHDD    StorageType = "hdd"
	StorageNVMe   StorageType = "nvme"
	StorageHybrid StorageType = "hybrid"
)

// CostUnit 成本单位（元/TB/月）
type CostUnit struct {
	Type     StorageType `json:"type"`
	PriceTB  float64     `json:"price_per_tb_month"` // 每TB每月成本（元）
	PowerKWH float64     `json:"power_kwh_month"`    // 每TB每月功耗成本
	Lifespan int         `json:"lifespan_years"`     // 使用寿命（年）
}

// StoragePool 存储池
type StoragePool struct {
	ID       string      `json:"id"`
	Name     string      `json:"name"`
	Type     StorageType `json:"type"`
	TotalTB  float64     `json:"total_tb"`
	UsedTB   float64     `json:"used_tb"`
	UnitCost float64     `json:"unit_cost"` // 当前每TB月成本
	IsHot    bool        `json:"is_hot"`    // 是否热数据层
}

// CostRecord 成本记录
type CostRecord struct {
	Timestamp   time.Time `json:"timestamp"`
	Period      string    `json:"period"` // "2024-01", "2024-Q1"
	TotalCost   float64   `json:"total_cost"`
	StorageCost float64   `json:"storage_cost"`
	PowerCost   float64   `json:"power_cost"`
	MaintCost   float64   `json:"maintenance_cost"`
	UsedTB      float64   `json:"used_tb"`
	UnitCostTB  float64   `json:"unit_cost_per_tb"`
}

// OptimizationSuggestion 优化建议
type OptimizationSuggestion struct {
	ID          string  `json:"id"`
	Type        string  `json:"type"` // "cold_migration", "dedup", "compress", "tiering"
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Savings     float64 `json:"estimated_savings"` // 预计节省金额（元/月）
	Effort      string  `json:"effort"`            // "low", "medium", "high"
	Priority    int     `json:"priority"`          // 1-5
}

// ROIReport 投资回报报告
type ROIReport struct {
	Investment     float64 `json:"investment"`      // 总投资
	MonthlySavings float64 `json:"monthly_savings"` // 月节省
	AnnualSavings  float64 `json:"annual_savings"`  // 年节省
	PaybackMonths  float64 `json:"payback_months"`  // 回本周期（月）
	ThreeYearROI   float64 `json:"three_year_roi"`  // 3年ROI百分比
	FiveYearROI    float64 `json:"five_year_roi"`   // 5年ROI百分比
}

// CloudComparison 云存储对比
type CloudComparison struct {
	LocalCostTB    float64            `json:"local_cost_per_tb"`
	CloudCosts     map[string]float64 `json:"cloud_costs"` // provider -> cost/tb/month
	Savings        map[string]float64 `json:"savings"`     // provider -> savings/tb/month
	Recommendation string             `json:"recommendation"`
}

// CostReport 综合成本报告
type CostReport struct {
	ID            string                    `json:"id"`
	GeneratedAt   time.Time                 `json:"generated_at"`
	TotalMonthly  float64                   `json:"total_monthly_cost"`
	TotalCapacity float64                   `json:"total_capacity_tb"`
	UsedCapacity  float64                   `json:"used_capacity_tb"`
	Utilization   float64                   `json:"utilization_rate"`
	CostPerTB     float64                   `json:"cost_per_tb"`
	Trends        []*CostRecord             `json:"trends,omitempty"`
	Suggestions   []*OptimizationSuggestion `json:"suggestions,omitempty"`
	ROI           *ROIReport                `json:"roi,omitempty"`
	CloudCompare  *CloudComparison          `json:"cloud_comparison,omitempty"`
	Pools         []*StoragePool            `json:"pools"`
}

// Config 分析器配置
type Config struct {
	MaintCostPerTB   float64            `json:"maint_cost_per_tb"`  // 每TB每月维护成本
	PowerCostKWH     float64            `json:"power_cost_kwh"`     // 每度电成本
	CloudPrices      map[string]float64 `json:"cloud_prices"`       // 云存储价格
	HotColdThreshold int                `json:"hot_cold_threshold"` // 热/冷数据天数阈值
}

// Manager 成本分析管理器
type Manager struct {
	mu       sync.RWMutex
	config   *Config
	pools    []*StoragePool
	records  []*CostRecord
	report   *CostReport
	dataFile string
}

var (
	// ErrPoolNotFound 存储池未找到
	ErrPoolNotFound = errors.New("storage pool not found")
	// ErrNoData 无数据
	ErrNoData = errors.New("no data available")
)

// NewManager 创建成本分析管理器
func NewManager(dataFile string) *Manager {
	return &Manager{
		config: &Config{
			MaintCostPerTB: 5.0,
			PowerCostKWH:   0.6,
			CloudPrices: map[string]float64{
				"aliyun_oss":  0.12,
				"tencent_cos": 0.119,
				"aws_s3":      0.17,
				"azure_blob":  0.15,
			},
			HotColdThreshold: 90,
		},
		dataFile: dataFile,
	}
}

// Initialize 初始化管理器
func (m *Manager) Initialize() error {
	if err := m.load(); err != nil {
		return err
	}
	if len(m.pools) == 0 {
		m.pools = m.defaultPools()
	}
	return nil
}

func (m *Manager) defaultPools() []*StoragePool {
	return []*StoragePool{
		{ID: "pool-ssd-01", Name: "SSD热数据池", Type: StorageSSD, TotalTB: 4, UsedTB: 2.8, UnitCost: 150, IsHot: true},
		{ID: "pool-hdd-01", Name: "HDD存储池", Type: StorageHDD, TotalTB: 16, UsedTB: 12.5, UnitCost: 40, IsHot: false},
		{ID: "pool-nvme-01", Name: "NVMe缓存池", Type: StorageNVMe, TotalTB: 1, UsedTB: 0.6, UnitCost: 300, IsHot: true},
	}
}

// AnalyzeCost 执行成本分析
func (m *Manager) AnalyzeCost() *CostReport {
	m.mu.Lock()
	defer m.mu.Unlock()

	totalCost := 0.0
	totalCap := 0.0
	totalUsed := 0.0

	for _, p := range m.pools {
		poolCost := p.UsedTB * p.UnitCost
		poolPower := p.UsedTB * m.estimatePower(p.Type)
		poolMaint := p.UsedTB * m.config.MaintCostPerTB
		totalCost += poolCost + poolPower + poolMaint
		totalCap += p.TotalTB
		totalUsed += p.UsedTB
	}

	utilization := 0.0
	if totalCap > 0 {
		utilization = totalUsed / totalCap
	}

	costPerTB := 0.0
	if totalUsed > 0 {
		costPerTB = totalCost / totalUsed
	}

	// 记录本月成本
	record := &CostRecord{
		Timestamp:   time.Now(),
		Period:      time.Now().Format("2006-01"),
		TotalCost:   totalCost,
		StorageCost: totalCost * 0.7,
		PowerCost:   totalCost * 0.15,
		MaintCost:   totalCost * 0.15,
		UsedTB:      totalUsed,
		UnitCostTB:  costPerTB,
	}
	m.records = append(m.records, record)

	report := &CostReport{
		ID:            fmt.Sprintf("cost-%d", time.Now().UnixNano()),
		GeneratedAt:   time.Now(),
		TotalMonthly:  math.Round(totalCost*100) / 100,
		TotalCapacity: totalCap,
		UsedCapacity:  totalUsed,
		Utilization:   math.Round(utilization*10000) / 100,
		CostPerTB:     math.Round(costPerTB*100) / 100,
		Trends:        m.getTrends(),
		Suggestions:   m.generateSuggestions(),
		ROI:           m.calculateROI(),
		CloudCompare:  m.compareCloud(),
		Pools:         m.pools,
	}

	m.report = report
	m.save()
	return report
}

// GetTrends 获取成本趋势
func (m *Manager) GetTrends(period string) []*CostRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if period == "quarterly" {
		return m.aggregateQuarterly()
	}
	return m.records
}

// GetROI 计算ROI
func (m *Manager) GetROI(investment float64) *ROIReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	monthlySavings := 0.0
	for _, s := range m.generateSuggestions() {
		monthlySavings += s.Savings
	}

	return m.computeROI(investment, monthlySavings)
}

// CompareCloud 对比云存储成本
func (m *Manager) CompareCloud() *CloudComparison {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.compareCloud()
}

// AddPool 添加存储池
func (m *Manager) AddPool(pool *StoragePool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pools = append(m.pools, pool)
	return m.save()
}

// RemovePool 移除存储池
func (m *Manager) RemovePool(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, p := range m.pools {
		if p.ID == id {
			m.pools = append(m.pools[:i], m.pools[i+1:]...)
			return m.save()
		}
	}
	return ErrPoolNotFound
}

// GetPools 获取所有存储池
func (m *Manager) GetPools() []*StoragePool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pools
}

// GetLatestReport 获取最新报告
func (m *Manager) GetLatestReport() (*CostReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.report == nil {
		return nil, ErrNoData
	}
	return m.report, nil
}

// GetStats 获取统计概览
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalCost := 0.0
	totalUsed := 0.0
	for _, p := range m.pools {
		totalCost += p.UsedTB * p.UnitCost
		totalUsed += p.UsedTB
	}

	return map[string]interface{}{
		"pools":          len(m.pools),
		"total_capacity": m.totalCapacity(),
		"used_capacity":  totalUsed,
		"monthly_cost":   totalCost,
		"cost_per_tb":    safeDiv(totalCost, totalUsed),
		"records":        len(m.records),
	}
}

// UpdatePoolUsage 更新存储池使用量
func (m *Manager) UpdatePoolUsage(id string, usedTB float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, p := range m.pools {
		if p.ID == id {
			p.UsedTB = usedTB
			return m.save()
		}
	}
	return ErrPoolNotFound
}

// 内部方法

func (m *Manager) estimatePower(t StorageType) float64 {
	switch t {
	case StorageSSD:
		return 0.5 * m.config.PowerCostKWH
	case StorageNVMe:
		return 0.8 * m.config.PowerCostKWH
	case StorageHDD:
		return 1.2 * m.config.PowerCostKWH
	default:
		return 1.0 * m.config.PowerCostKWH
	}
}

func (m *Manager) getTrends() []*CostRecord {
	if len(m.records) > 12 {
		return m.records[len(m.records)-12:]
	}
	return m.records
}

func (m *Manager) aggregateQuarterly() []*CostRecord {
	quarterMap := make(map[string]*CostRecord)
	for _, r := range m.records {
		year := r.Timestamp.Year()
		quarter := (r.Timestamp.Month()-1)/3 + 1
		key := fmt.Sprintf("%d-Q%d", year, quarter)
		if existing, ok := quarterMap[key]; ok {
			existing.TotalCost += r.TotalCost
			existing.StorageCost += r.StorageCost
			existing.PowerCost += r.PowerCost
			existing.MaintCost += r.MaintCost
			existing.UsedTB = r.UsedTB // 取最新值
		} else {
			quarterMap[key] = &CostRecord{
				Timestamp:   r.Timestamp,
				Period:      key,
				TotalCost:   r.TotalCost,
				StorageCost: r.StorageCost,
				PowerCost:   r.PowerCost,
				MaintCost:   r.MaintCost,
				UsedTB:      r.UsedTB,
			}
		}
	}

	var result []*CostRecord
	for _, v := range quarterMap {
		v.UnitCostTB = safeDiv(v.TotalCost, v.UsedTB)
		result = append(result, v)
	}
	return result
}

func (m *Manager) generateSuggestions() []*OptimizationSuggestion {
	var suggestions []*OptimizationSuggestion
	id := 0

	for _, p := range m.pools {
		// 低利用率建议
		if p.TotalTB > 0 && p.UsedTB/p.TotalTB < 0.3 {
			id++
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:          fmt.Sprintf("opt-%d", id),
				Type:        "consolidation",
				Title:       fmt.Sprintf("合并低利用率存储池: %s", p.Name),
				Description: fmt.Sprintf("利用率仅%.1f%%，建议合并到其他存储池", p.UsedTB/p.TotalTB*100),
				Savings:     (p.TotalTB - p.UsedTB) * p.UnitCost * 0.3,
				Effort:      "medium",
				Priority:    3,
			})
		}

		// HDD冷数据迁移
		if p.Type == StorageHDD && p.IsHot == false && p.UsedTB > 10 {
			id++
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:          fmt.Sprintf("opt-%d", id),
				Type:        "cold_migration",
				Title:       "冷数据归档",
				Description: fmt.Sprintf("%s 中 %.1fTB 数据超过 %d 天未访问，建议归档", p.Name, p.UsedTB*0.3, m.config.HotColdThreshold),
				Savings:     p.UsedTB * 0.3 * p.UnitCost * 0.5,
				Effort:      "low",
				Priority:    2,
			})
		}

		// 去重建议
		if p.UsedTB > 5 {
			id++
			dedupRatio := 0.15 // 预估15%去重率
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:          fmt.Sprintf("opt-%d", id),
				Type:        "dedup",
				Title:       fmt.Sprintf("启用去重: %s", p.Name),
				Description: fmt.Sprintf("预计可节省 %.1fTB 空间（约15%%去重率）", p.UsedTB*dedupRatio),
				Savings:     p.UsedTB * dedupRatio * p.UnitCost,
				Effort:      "medium",
				Priority:    3,
			})
		}

		// 压缩建议
		if p.UsedTB > 2 {
			id++
			compressRatio := 0.2 // 预估20%压缩率
			suggestions = append(suggestions, &OptimizationSuggestion{
				ID:          fmt.Sprintf("opt-%d", id),
				Type:        "compress",
				Title:       fmt.Sprintf("启用压缩: %s", p.Name),
				Description: fmt.Sprintf("预计可节省 %.1fTB 空间（约20%%压缩率）", p.UsedTB*compressRatio),
				Savings:     p.UsedTB * compressRatio * p.UnitCost,
				Effort:      "low",
				Priority:    2,
			})
		}
	}

	// 分层建议
	if len(m.pools) > 1 {
		id++
		suggestions = append(suggestions, &OptimizationSuggestion{
			ID:          fmt.Sprintf("opt-%d", id),
			Type:        "tiering",
			Title:       "优化数据分层策略",
			Description: "建议将热数据放在SSD/NVMe，温数据放HDD，冷数据归档",
			Savings:     m.totalCapacity() * 5,
			Effort:      "high",
			Priority:    4,
		})
	}

	return suggestions
}

func (m *Manager) calculateROI() *ROIReport {
	// 估算当前投资（硬件 + 3年运营）
	hardwareCost := 0.0
	for _, p := range m.pools {
		hardwareCost += p.TotalTB * p.UnitCost * 12 * 3 // 3年的存储成本
	}

	monthlySavings := 0.0
	for _, s := range m.generateSuggestions() {
		monthlySavings += s.Savings
	}

	return m.computeROI(hardwareCost, monthlySavings)
}

func (m *Manager) computeROI(investment, monthlySavings float64) *ROIReport {
	annualSavings := monthlySavings * 12
	payback := safeDiv(investment, monthlySavings)
	threeYearROI := safeDiv((annualSavings*3-investment), investment) * 100
	fiveYearROI := safeDiv((annualSavings*5-investment), investment) * 100

	return &ROIReport{
		Investment:     math.Round(investment*100) / 100,
		MonthlySavings: math.Round(monthlySavings*100) / 100,
		AnnualSavings:  math.Round(annualSavings*100) / 100,
		PaybackMonths:  math.Round(payback*10) / 10,
		ThreeYearROI:   math.Round(threeYearROI*100) / 100,
		FiveYearROI:    math.Round(fiveYearROI*100) / 100,
	}
}

func (m *Manager) compareCloud() *CloudComparison {
	localCost := 0.0
	totalUsed := 0.0
	for _, p := range m.pools {
		localCost += p.UsedTB * p.UnitCost
		totalUsed += p.UsedTB
	}
	localPerTB := safeDiv(localCost, totalUsed)

	cloudCosts := make(map[string]float64)
	savings := make(map[string]float64)
	for provider, price := range m.config.CloudPrices {
		cloudCosts[provider] = price
		savings[provider] = localPerTB - price
	}

	recommendation := "本地存储"
	for provider, saving := range savings {
		if saving > 0 {
			recommendation = fmt.Sprintf("本地存储更划算，比 %s 每TB每月节省 %.2f 元", provider, saving)
			break
		}
	}

	return &CloudComparison{
		LocalCostTB:    localPerTB,
		CloudCosts:     cloudCosts,
		Savings:        savings,
		Recommendation: recommendation,
	}
}

func (m *Manager) totalCapacity() float64 {
	total := 0.0
	for _, p := range m.pools {
		total += p.TotalTB
	}
	return total
}

func (m *Manager) load() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m.records)
}

func (m *Manager) save() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := json.MarshalIndent(m.records, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.dataFile, data, 0644)
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
