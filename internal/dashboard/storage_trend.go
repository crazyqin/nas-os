// Package dashboard 提供存储容量趋势分析功能
// 对标TrueNAS Dashboard的存储容量趋势可视化
package dashboard

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ========== 存储趋势数据结构 ==========

// StorageTrendData 存储趋势数据
type StorageTrendData struct {
	Timestamp    time.Time `json:"timestamp"`
	TotalSize    uint64    `json:"totalSize"`
	UsedSize     uint64    `json:"usedSize"`
	FreeSize     uint64    `json:"freeSize"`
	UsagePercent float64   `json:"usagePercent"`
	GrowthRate   float64   `json:"growthRate"` // 每日增长率 (GB/day)
}

// StoragePoolTrend 存储池趋势
type StoragePoolTrend struct {
	PoolID     string             `json:"poolId"`
	PoolName   string             `json:"poolName"`
	TrendData  []StorageTrendData `json:"trendData"`
	Prediction *StoragePrediction `json:"prediction,omitempty"`
}

// StoragePrediction 存储容量预测
type StoragePrediction struct {
	FullDate          time.Time `json:"fullDate"`          // 预测满盘日期
	DaysUntilFull     int       `json:"daysUntilFull"`     // 距离满盘天数
	GrowthRateDaily   float64   `json:"growthRateDaily"`   // 每日增长率 (GB)
	GrowthRateWeekly  float64   `json:"growthRateWeekly"`  // 每周增长率 (GB)
	GrowthRateMonthly float64   `json:"growthRateMonthly"` // 每月增长率 (GB)
	Confidence        float64   `json:"confidence"`        // 预测置信度 (0-1)
	PredictedUsage30d float64   `json:"predictedUsage30d"` // 30天后预测使用率
	PredictedUsage60d float64   `json:"predictedUsage60d"` // 60天后预测使用率
	PredictedUsage90d float64   `json:"predictedUsage90d"` // 90天后预测使用率
}

// TrendTimeRange 时间范围
type TrendTimeRange string

const (
	TrendRange7Days   TrendTimeRange = "7d"
	TrendRange30Days  TrendTimeRange = "30d"
	TrendRange90Days  TrendTimeRange = "90d"
	TrendRange180Days TrendTimeRange = "180d"
	TrendRange365Days TrendTimeRange = "365d"
)

// TrendAggregation 聚合方式
type TrendAggregation string

const (
	TrendAggHourly  TrendAggregation = "hourly"
	TrendAggDaily   TrendAggregation = "daily"
	TrendAggWeekly  TrendAggregation = "weekly"
	TrendAggMonthly TrendAggregation = "monthly"
)

// StorageTrendQuery 趋势查询参数
type StorageTrendQuery struct {
	PoolID       string           `json:"poolId,omitempty"`
	TimeRange    TrendTimeRange   `json:"timeRange"`
	Aggregation  TrendAggregation `json:"aggregation"`
	IncludeShare bool             `json:"includeShare"` // 是否包含共享文件夹分组
}

// StorageHistoryRecord 历史记录
type StorageHistoryRecord struct {
	ID           string    `json:"id"`
	PoolID       string    `json:"poolId"`
	PoolName     string    `json:"poolName"`
	Timestamp    time.Time `json:"timestamp"`
	TotalSize    uint64    `json:"totalSize"`
	UsedSize     uint64    `json:"usedSize"`
	FreeSize     uint64    `json:"freeSize"`
	UsagePercent float64   `json:"usagePercent"`
	ShareID      string    `json:"shareId,omitempty"`   // 共享文件夹ID
	ShareName    string    `json:"shareName,omitempty"` // 共享文件夹名称
}

// ShareTrendData 共享文件夹趋势
type ShareTrendData struct {
	ShareID    string             `json:"shareId"`
	ShareName  string             `json:"shareName"`
	TrendData  []StorageTrendData `json:"trendData"`
	Prediction *StoragePrediction `json:"prediction,omitempty"`
}

// ========== 存储趋势管理器 ==========

// StorageTrendManager 存储趋势管理器
type StorageTrendManager struct {
	mu                 sync.RWMutex
	metricsDir         string                            // 指标存储目录
	collectionInterval time.Duration                     // 采集间隔
	historyRecords     map[string][]StorageHistoryRecord // 历史记录缓存
	lastCollection     time.Time                         // 上次采集时间
}

// StorageTrendConfig 配置
type StorageTrendConfig struct {
	MetricsDir         string        `json:"metricsDir"`
	CollectionInterval time.Duration `json:"collectionInterval"`
	MaxHistoryDays     int           `json:"maxHistoryDays"`
}

// NewStorageTrendManager 创建存储趋势管理器
func NewStorageTrendManager(config *StorageTrendConfig) (*StorageTrendManager, error) {
	if config == nil {
		config = &StorageTrendConfig{
			MetricsDir:         "/var/lib/nas-os/metrics",
			CollectionInterval: 1 * time.Hour,
			MaxHistoryDays:     365,
		}
	}

	// 创建指标目录
	if err := os.MkdirAll(config.MetricsDir, 0750); err != nil {
		return nil, fmt.Errorf("创建指标目录失败: %w", err)
	}

	manager := &StorageTrendManager{
		metricsDir:         config.MetricsDir,
		collectionInterval: config.CollectionInterval,
		historyRecords:     make(map[string][]StorageHistoryRecord),
	}

	// 加载历史数据
	if err := manager.loadHistory(); err != nil {
		// 非致命错误，仅记录
		fmt.Printf("加载历史数据失败: %v\n", err)
	}

	return manager, nil
}

// ========== 数据采集 ==========

// CollectSnapshot 采集存储容量快照
func (m *StorageTrendManager) CollectSnapshot(pools []StoragePoolInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for _, pool := range pools {
		record := StorageHistoryRecord{
			ID:           fmt.Sprintf("%s-%d", pool.PoolID, now.Unix()),
			PoolID:       pool.PoolID,
			PoolName:     pool.PoolName,
			Timestamp:    now,
			TotalSize:    pool.TotalSize,
			UsedSize:     pool.UsedSize,
			FreeSize:     pool.FreeSize,
			UsagePercent: pool.UsagePercent,
		}

		// 添加到缓存
		poolKey := pool.PoolID
		m.historyRecords[poolKey] = append(m.historyRecords[poolKey], record)

		// 保存到文件
		if err := m.saveRecordToFile(&record); err != nil {
			fmt.Printf("保存记录失败 [%s]: %v\n", pool.PoolID, err)
		}
	}

	m.lastCollection = now
	return nil
}

// StoragePoolInfo 存储池信息
type StoragePoolInfo struct {
	PoolID       string  `json:"poolId"`
	PoolName     string  `json:"poolName"`
	TotalSize    uint64  `json:"totalSize"`
	UsedSize     uint64  `json:"usedSize"`
	FreeSize     uint64  `json:"freeSize"`
	UsagePercent float64 `json:"usagePercent"`
}

// saveRecordToFile 保存记录到文件
func (m *StorageTrendManager) saveRecordToFile(record *StorageHistoryRecord) error {
	dateDir := filepath.Join(m.metricsDir, "storage", record.Timestamp.Format("2006-01-02"))
	if err := os.MkdirAll(dateDir, 0750); err != nil {
		return err
	}

	filename := filepath.Join(dateDir, fmt.Sprintf("%s.json", record.PoolID))

	// 读取现有记录
	var records []StorageHistoryRecord
	if data, err := os.ReadFile(filename); err == nil {
		if err := json.Unmarshal(data, &records); err != nil {
			// 解析失败，重新创建
			records = []StorageHistoryRecord{}
		}
	}

	// 添加新记录
	records = append(records, *record)

	// 写入文件
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// loadHistory 加载历史数据
func (m *StorageTrendManager) loadHistory() error {
	storageDir := filepath.Join(m.metricsDir, "storage")

	// 遍历日期目录
	entries, err := os.ReadDir(storageDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		dateDir := filepath.Join(storageDir, entry.Name())
		files, err := os.ReadDir(dateDir)
		if err != nil {
			continue
		}

		for _, file := range files {
			if !strings.HasSuffix(file.Name(), ".json") {
				continue
			}

			poolID := strings.TrimSuffix(file.Name(), ".json")
			filePath := filepath.Join(dateDir, file.Name())

			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}

			var records []StorageHistoryRecord
			if err := json.Unmarshal(data, &records); err != nil {
				continue
			}

			m.historyRecords[poolID] = append(m.historyRecords[poolID], records...)
		}
	}

	// 按时间排序
	for poolID := range m.historyRecords {
		sortRecordsByTime(m.historyRecords[poolID])
	}

	return nil
}

// sortRecordsByTime 按时间排序记录
func sortRecordsByTime(records []StorageHistoryRecord) {
	for i := 0; i < len(records)-1; i++ {
		for j := i + 1; j < len(records); j++ {
			if records[i].Timestamp.After(records[j].Timestamp) {
				records[i], records[j] = records[j], records[i]
			}
		}
	}
}

// ========== 趋势数据API ==========

// GetStorageTrend 获取存储趋势数据
func (m *StorageTrendManager) GetStorageTrend(query *StorageTrendQuery) ([]StoragePoolTrend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	startTime := m.calculateStartTime(query.TimeRange, now)

	var trends []StoragePoolTrend

	// 确定查询的存储池
	poolIDs := []string{query.PoolID}
	if query.PoolID == "" {
		// 获取所有存储池
		for poolID := range m.historyRecords {
			poolIDs = append(poolIDs, poolID)
		}
	}

	for _, poolID := range poolIDs {
		if poolID == "" {
			continue
		}

		records := m.historyRecords[poolID]
		if len(records) == 0 {
			continue
		}

		// 过滤时间范围
		filtered := m.filterRecordsByTime(records, startTime, now)

		// 聚合数据
		aggregated := m.aggregateRecords(filtered, query.Aggregation)

		// 计算增长率
		trendData := m.calculateGrowthRate(aggregated)

		// 创建趋势数据
		poolName := ""
		if len(records) > 0 {
			poolName = records[0].PoolName
		}

		poolTrend := StoragePoolTrend{
			PoolID:    poolID,
			PoolName:  poolName,
			TrendData: trendData,
		}

		// 计算预测
		if len(trendData) >= 7 {
			prediction := m.calculatePrediction(trendData)
			poolTrend.Prediction = &prediction
		}

		trends = append(trends, poolTrend)
	}

	return trends, nil
}

// calculateStartTime 计算起始时间
func (m *StorageTrendManager) calculateStartTime(rangeType TrendTimeRange, now time.Time) time.Time {
	switch rangeType {
	case TrendRange7Days:
		return now.AddDate(0, 0, -7)
	case TrendRange30Days:
		return now.AddDate(0, 0, -30)
	case TrendRange90Days:
		return now.AddDate(0, 0, -90)
	case TrendRange180Days:
		return now.AddDate(0, 0, -180)
	case TrendRange365Days:
		return now.AddDate(0, 0, -365)
	default:
		return now.AddDate(0, 0, -30)
	}
}

// filterRecordsByTime 过滤时间范围内的记录
func (m *StorageTrendManager) filterRecordsByTime(records []StorageHistoryRecord, start, end time.Time) []StorageHistoryRecord {
	var filtered []StorageHistoryRecord
	for _, r := range records {
		if r.Timestamp.After(start) && r.Timestamp.Before(end) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// aggregateRecords 聚合记录
func (m *StorageTrendManager) aggregateRecords(records []StorageHistoryRecord, agg TrendAggregation) []StorageHistoryRecord {
	if len(records) == 0 {
		return records
	}

	switch agg {
	case TrendAggHourly:
		// 不聚合，返回原始数据
		return records
	case TrendAggDaily:
		return m.aggregateByDay(records)
	case TrendAggWeekly:
		return m.aggregateByWeek(records)
	case TrendAggMonthly:
		return m.aggregateByMonth(records)
	default:
		return records
	}
}

// aggregateByDay 按天聚合
func (m *StorageTrendManager) aggregateByDay(records []StorageHistoryRecord) []StorageHistoryRecord {
	dayMap := make(map[string][]StorageHistoryRecord)

	for _, r := range records {
		dayKey := r.Timestamp.Format("2006-01-02")
		dayMap[dayKey] = append(dayMap[dayKey], r)
	}

	var aggregated []StorageHistoryRecord
	for dayKey, dayRecords := range dayMap {
		// 取每天最后一条记录作为代表
		if len(dayRecords) > 0 {
			// 或计算平均值
			var totalUsed, totalFree uint64
			var avgUsage float64
			for _, r := range dayRecords {
				totalUsed += r.UsedSize
				totalFree += r.FreeSize
				avgUsage += r.UsagePercent
			}
			avgUsage /= float64(len(dayRecords))

			// 使用最后一条记录的时间戳
			lastRecord := dayRecords[len(dayRecords)-1]
			aggRecord := StorageHistoryRecord{
				ID:           fmt.Sprintf("agg-%s", dayKey),
				PoolID:       lastRecord.PoolID,
				PoolName:     lastRecord.PoolName,
				Timestamp:    lastRecord.Timestamp,
				TotalSize:    lastRecord.TotalSize,
				UsedSize:     totalUsed / uint64(len(dayRecords)),
				FreeSize:     totalFree / uint64(len(dayRecords)),
				UsagePercent: avgUsage,
			}
			aggregated = append(aggregated, aggRecord)
		}
	}

	// 按时间排序
	sortRecordsByTime(aggregated)
	return aggregated
}

// aggregateByWeek 按周聚合
func (m *StorageTrendManager) aggregateByWeek(records []StorageHistoryRecord) []StorageHistoryRecord {
	weekMap := make(map[int][]StorageHistoryRecord)

	for _, r := range records {
		year, week := r.Timestamp.ISOWeek()
		weekKey := year*100 + week
		weekMap[weekKey] = append(weekMap[weekKey], r)
	}

	var aggregated []StorageHistoryRecord
	for _, weekRecords := range weekMap {
		if len(weekRecords) > 0 {
			lastRecord := weekRecords[len(weekRecords)-1]
			var avgUsage float64
			var avgUsed, avgFree uint64
			for _, r := range weekRecords {
				avgUsage += r.UsagePercent
				avgUsed += r.UsedSize
				avgFree += r.FreeSize
			}
			n := float64(len(weekRecords))
			_, weekNum := lastRecord.Timestamp.ISOWeek()
			aggRecord := StorageHistoryRecord{
				ID:           fmt.Sprintf("agg-week-%d", weekNum),
				PoolID:       lastRecord.PoolID,
				PoolName:     lastRecord.PoolName,
				Timestamp:    lastRecord.Timestamp,
				TotalSize:    lastRecord.TotalSize,
				UsedSize:     uint64(float64(avgUsed) / n),
				FreeSize:     uint64(float64(avgFree) / n),
				UsagePercent: avgUsage / n,
			}
			aggregated = append(aggregated, aggRecord)
		}
	}

	sortRecordsByTime(aggregated)
	return aggregated
}

// aggregateByMonth 按月聚合
func (m *StorageTrendManager) aggregateByMonth(records []StorageHistoryRecord) []StorageHistoryRecord {
	monthMap := make(map[string][]StorageHistoryRecord)

	for _, r := range records {
		monthKey := r.Timestamp.Format("2006-01")
		monthMap[monthKey] = append(monthMap[monthKey], r)
	}

	var aggregated []StorageHistoryRecord
	for _, monthRecords := range monthMap {
		if len(monthRecords) > 0 {
			lastRecord := monthRecords[len(monthRecords)-1]
			var avgUsage float64
			var avgUsed, avgFree uint64
			for _, r := range monthRecords {
				avgUsage += r.UsagePercent
				avgUsed += r.UsedSize
				avgFree += r.FreeSize
			}
			n := float64(len(monthRecords))
			aggRecord := StorageHistoryRecord{
				ID:           fmt.Sprintf("agg-%s", lastRecord.Timestamp.Format("2006-01")),
				PoolID:       lastRecord.PoolID,
				PoolName:     lastRecord.PoolName,
				Timestamp:    lastRecord.Timestamp,
				TotalSize:    lastRecord.TotalSize,
				UsedSize:     uint64(float64(avgUsed) / n),
				FreeSize:     uint64(float64(avgFree) / n),
				UsagePercent: avgUsage / n,
			}
			aggregated = append(aggregated, aggRecord)
		}
	}

	sortRecordsByTime(aggregated)
	return aggregated
}

// ========== 增长率计算 ==========

// calculateGrowthRate 计算增长率
func (m *StorageTrendManager) calculateGrowthRate(records []StorageHistoryRecord) []StorageTrendData {
	var trendData []StorageTrendData

	for i, r := range records {
		data := StorageTrendData{
			Timestamp:    r.Timestamp,
			TotalSize:    r.TotalSize,
			UsedSize:     r.UsedSize,
			FreeSize:     r.FreeSize,
			UsagePercent: r.UsagePercent,
			GrowthRate:   0,
		}

		// 计算增长率 (基于前一天)
		if i > 0 {
			prev := records[i-1]
			timeDiff := r.Timestamp.Sub(prev.Timestamp).Hours()
			if timeDiff > 0 {
				usedDiff := float64(r.UsedSize-prev.UsedSize) / (1024 * 1024 * 1024) // GB
				days := timeDiff / 24
				if days > 0 {
					data.GrowthRate = usedDiff / days // GB/day
				}
			}
		}

		trendData = append(trendData, data)
	}

	return trendData
}

// ========== 容量预测 ==========

// GetStoragePrediction 获取存储容量预测
func (m *StorageTrendManager) GetStoragePrediction(poolID string) (*StoragePrediction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := m.historyRecords[poolID]
	if len(records) < 7 {
		return nil, fmt.Errorf("历史数据不足，至少需要7天数据")
	}

	// 转换为趋势数据
	trendData := m.calculateGrowthRate(records)

	prediction := m.calculatePrediction(trendData)

	return &prediction, nil
}

// calculatePrediction 计算预测
func (m *StorageTrendManager) calculatePrediction(trendData []StorageTrendData) StoragePrediction {
	prediction := StoragePrediction{}

	if len(trendData) < 2 {
		return prediction
	}

	// 线性回归计算增长趋势
	// 使用最近30天数据（或全部数据如果少于30天）
	dataPoints := trendData
	if len(trendData) > 30 {
		dataPoints = trendData[len(trendData)-30:]
	}

	// 计算平均增长率
	var totalGrowth float64
	var growthCount int
	for i := 1; i < len(dataPoints); i++ {
		if dataPoints[i].GrowthRate > 0 {
			totalGrowth += dataPoints[i].GrowthRate
			growthCount++
		}
	}

	if growthCount > 0 {
		avgGrowth := totalGrowth / float64(growthCount)
		prediction.GrowthRateDaily = avgGrowth
		prediction.GrowthRateWeekly = avgGrowth * 7
		prediction.GrowthRateMonthly = avgGrowth * 30

		// 计算满盘时间
		currentData := dataPoints[len(dataPoints)-1]
		freeSpaceGB := float64(currentData.FreeSize) / (1024 * 1024 * 1024)
		totalSpaceGB := float64(currentData.TotalSize) / (1024 * 1024 * 1024)

		if avgGrowth > 0 && freeSpaceGB > 0 {
			daysUntilFull := int(freeSpaceGB / avgGrowth)
			prediction.DaysUntilFull = daysUntilFull
			prediction.FullDate = currentData.Timestamp.AddDate(0, 0, daysUntilFull)

			// 计算未来使用率
			currentUsage := currentData.UsagePercent
			prediction.PredictedUsage30d = math.Min(100, currentUsage+(avgGrowth*30/totalSpaceGB*100))
			prediction.PredictedUsage60d = math.Min(100, currentUsage+(avgGrowth*60/totalSpaceGB*100))
			prediction.PredictedUsage90d = math.Min(100, currentUsage+(avgGrowth*90/totalSpaceGB*100))

			// 计算置信度 (基于数据量和一致性)
			prediction.Confidence = m.calculateConfidence(dataPoints, avgGrowth)
		}
	}

	return prediction
}

// calculateConfidence 计算预测置信度
func (m *StorageTrendManager) calculateConfidence(dataPoints []StorageTrendData, avgGrowth float64) float64 {
	// 置信度因素：
	// 1. 数据量：越多数据置信度越高
	// 2. 增长一致性：增长越稳定置信度越高

	// 数据量因素
	dataFactor := math.Min(1.0, float64(len(dataPoints))/30.0)

	// 增长一致性因素
	var variance float64
	var count int
	for i := 1; i < len(dataPoints); i++ {
		if dataPoints[i].GrowthRate > 0 {
			diff := dataPoints[i].GrowthRate - avgGrowth
			variance += diff * diff
			count++
		}
	}

	if count > 0 {
		stdDev := math.Sqrt(variance / float64(count))
		// 标准差越小，一致性越高
		consistencyFactor := math.Max(0, 1.0-stdDev/(avgGrowth+1))
		return (dataFactor * 0.3) + (consistencyFactor * 0.7)
	}

	return dataFactor * 0.5
}

// ========== 历史数据API ==========

// GetStorageHistory 获取存储历史数据
func (m *StorageTrendManager) GetStorageHistory(poolID string, days int) ([]StorageHistoryRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := m.historyRecords[poolID]
	if len(records) == 0 {
		return nil, fmt.Errorf("存储池 %s 无历史数据", poolID)
	}

	// 过滤时间范围
	now := time.Now()
	startTime := now.AddDate(0, 0, -days)

	var history []StorageHistoryRecord
	for _, r := range records {
		if r.Timestamp.After(startTime) && r.Timestamp.Before(now) {
			history = append(history, r)
		}
	}

	return history, nil
}

// GetShareHistory 获取共享文件夹历史数据
func (m *StorageTrendManager) GetShareHistory(poolID, shareID string, days int) ([]StorageHistoryRecord, error) {
	// TODO: 实现共享文件夹级别的历史数据
	// 需要从存储模块获取共享文件夹容量信息
	return nil, fmt.Errorf("共享文件夹历史数据功能待实现")
}

// ========== 图表数据格式化 ==========

// ToChartData 转换为图表数据格式 (ECharts)
func (t *StoragePoolTrend) ToChartData() *EChartsLineData {
	chartData := &EChartsLineData{
		Title:  fmt.Sprintf("%s 存储趋势", t.PoolName),
		XAxis:  make([]string, 0),
		Series: make([]*EChartsSeries, 0),
	}

	// X轴：时间
	for _, d := range t.TrendData {
		chartData.XAxis = append(chartData.XAxis, d.Timestamp.Format("MM-DD"))
	}

	// Y轴系列：使用量、使用率、增长率
	usageSeries := &EChartsSeries{
		Name: "使用量 (GB)",
		Type: "line",
		Data: make([]float64, 0),
	}
	usagePercentSeries := &EChartsSeries{
		Name: "使用率 (%)",
		Type: "line",
		Data: make([]float64, 0),
	}
	growthSeries := &EChartsSeries{
		Name: "增长率 (GB/day)",
		Type: "line",
		Data: make([]float64, 0),
	}

	for _, d := range t.TrendData {
		usageGB := float64(d.UsedSize) / (1024 * 1024 * 1024)
		usageSeries.Data = append(usageSeries.Data, usageGB)
		usagePercentSeries.Data = append(usagePercentSeries.Data, d.UsagePercent)
		growthSeries.Data = append(growthSeries.Data, d.GrowthRate)
	}

	chartData.Series = append(chartData.Series, usageSeries, usagePercentSeries, growthSeries)

	return chartData
}

// EChartsLineData ECharts折线图数据
type EChartsLineData struct {
	Title  string           `json:"title"`
	XAxis  []string         `json:"xAxis"`
	Series []*EChartsSeries `json:"series"`
}

// EChartsSeries ECharts系列
type EChartsSeries struct {
	Name string    `json:"name"`
	Type string    `json:"type"`
	Data []float64 `json:"data"`
}

// ToChartJSData 转换为Chart.js数据格式
func (t *StoragePoolTrend) ToChartJSData() *ChartJSLineData {
	chartData := &ChartJSLineData{
		Labels:   make([]string, 0),
		Datasets: make([]*ChartJSDataset, 0),
	}

	// 标签：时间
	for _, d := range t.TrendData {
		chartData.Labels = append(chartData.Labels, d.Timestamp.Format("MM-DD"))
	}

	// 数据集
	usageDataset := &ChartJSDataset{
		Label:           "使用量 (GB)",
		Data:            make([]float64, 0),
		BorderColor:     "#4CAF50",
		BackgroundColor: "#4CAF50",
		Fill:            false,
	}
	usagePercentDataset := &ChartJSDataset{
		Label:           "使用率 (%)",
		Data:            make([]float64, 0),
		BorderColor:     "#2196F3",
		BackgroundColor: "#2196F3",
		Fill:            false,
	}

	for _, d := range t.TrendData {
		usageGB := float64(d.UsedSize) / (1024 * 1024 * 1024)
		usageDataset.Data = append(usageDataset.Data, usageGB)
		usagePercentDataset.Data = append(usagePercentDataset.Data, d.UsagePercent)
	}

	chartData.Datasets = append(chartData.Datasets, usageDataset, usagePercentDataset)

	return chartData
}

// ChartJSLineData Chart.js折线图数据
type ChartJSLineData struct {
	Labels   []string          `json:"labels"`
	Datasets []*ChartJSDataset `json:"datasets"`
}

// ChartJSDataset Chart.js数据集
type ChartJSDataset struct {
	Label           string    `json:"label"`
	Data            []float64 `json:"data"`
	BorderColor     string    `json:"borderColor"`
	BackgroundColor string    `json:"backgroundColor"`
	Fill            bool      `json:"fill"`
}

// ========== 清理和维护 ==========

// CleanupOldRecords 清理过期记录
func (m *StorageTrendManager) CleanupOldRecords(maxDays int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoffTime := time.Now().AddDate(0, 0, -maxDays)

	for poolID, records := range m.historyRecords {
		var filtered []StorageHistoryRecord
		for _, r := range records {
			if r.Timestamp.After(cutoffTime) {
				filtered = append(filtered, r)
			}
		}
		m.historyRecords[poolID] = filtered
	}

	// 清理文件
	storageDir := filepath.Join(m.metricsDir, "storage")
	entries, err := os.ReadDir(storageDir)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// 解析日期
		date, err := time.Parse("2006-01-02", entry.Name())
		if err != nil {
			continue
		}

		if date.Before(cutoffTime) {
			dateDir := filepath.Join(storageDir, entry.Name())
			os.RemoveAll(dateDir)
		}
	}

	return nil
}

// GetCollectionStatus 获取采集状态
func (m *StorageTrendManager) GetCollectionStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalRecords := 0
	for _, records := range m.historyRecords {
		totalRecords += len(records)
	}

	return map[string]interface{}{
		"lastCollection": m.lastCollection,
		"totalRecords":   totalRecords,
		"poolCount":      len(m.historyRecords),
		"interval":       m.collectionInterval.String(),
	}
}

// ========== 单元测试辅助 ==========

// GenerateMockHistory 生成模拟历史数据
func GenerateMockHistory(poolID, poolName string, days int) []StorageHistoryRecord {
	var records []StorageHistoryRecord

	now := time.Now()
	startSize := uint64(100 * 1024 * 1024 * 1024)  // 100GB
	growthPerDay := uint64(2 * 1024 * 1024 * 1024) // 2GB/day

	for i := days; i >= 0; i-- {
		timestamp := now.AddDate(0, 0, -i)
		used := startSize + uint64(days-i)*growthPerDay
		total := uint64(500 * 1024 * 1024 * 1024) // 500GB total
		free := total - used
		usage := float64(used) / float64(total) * 100

		record := StorageHistoryRecord{
			ID:           fmt.Sprintf("%s-%d", poolID, timestamp.Unix()),
			PoolID:       poolID,
			PoolName:     poolName,
			Timestamp:    timestamp,
			TotalSize:    total,
			UsedSize:     used,
			FreeSize:     free,
			UsagePercent: usage,
		}

		records = append(records, record)
	}

	return records
}
