// Package quota 提供存储配额管理功能
package quota

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 配额历史统计 ==========

// HistoryConfig 历史配置
type HistoryConfig struct {
	MaxRecords     int           `json:"max_records"`     // 最大历史记录数
	RetentionDays  int           `json:"retention_days"`  // 保留天数
	CollectEnabled bool          `json:"collect_enabled"` // 是否启用自动采集
	CollectPeriod  time.Duration `json:"collect_period"`  // 采集周期
	PersistPath    string        `json:"persist_path"`    // 持久化路径
}

// DefaultHistoryConfig 默认历史配置
func DefaultHistoryConfig() HistoryConfig {
	return HistoryConfig{
		MaxRecords:     10000,
		RetentionDays:  365,
		CollectEnabled: true,
		CollectPeriod:  5 * time.Minute,
	}
}

// HistoryRecord 配额历史记录详情
type HistoryRecord struct {
	ID           string    `json:"id"`
	QuotaID      string    `json:"quota_id"`
	TargetName   string    `json:"target_name"`
	TargetType   Type      `json:"target_type"`
	VolumeName   string    `json:"volume_name"`
	UsedBytes    uint64    `json:"used_bytes"`
	LimitBytes   uint64    `json:"limit_bytes"`
	UsagePercent float64   `json:"usage_percent"`
	IsOverSoft   bool      `json:"is_over_soft"`
	IsOverHard   bool      `json:"is_over_hard"`
	Timestamp    time.Time `json:"timestamp"`
}

// HistoryStatistics 历史统计结果
type HistoryStatistics struct {
	QuotaID     string    `json:"quota_id"`
	TargetName  string    `json:"target_name"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	RecordCount int       `json:"record_count"`

	// 使用量统计
	MinUsedBytes     uint64  `json:"min_used_bytes"`
	MaxUsedBytes     uint64  `json:"max_used_bytes"`
	AvgUsedBytes     float64 `json:"avg_used_bytes"`
	CurrentUsedBytes uint64  `json:"current_used_bytes"`

	// 使用率统计
	MinUsagePercent     float64 `json:"min_usage_percent"`
	MaxUsagePercent     float64 `json:"max_usage_percent"`
	AvgUsagePercent     float64 `json:"avg_usage_percent"`
	CurrentUsagePercent float64 `json:"current_usage_percent"`

	// 增长统计
	TotalGrowthBytes   uint64  `json:"total_growth_bytes"`
	TotalGrowthPercent float64 `json:"total_growth_percent"`
	DailyGrowthRate    float64 `json:"daily_growth_rate"` // 字节/天

	// 超限统计
	OverSoftCount  int     `json:"over_soft_count"`  // 超软限制次数
	OverHardCount  int     `json:"over_hard_count"`  // 超硬限制次数
	MaxOverPercent float64 `json:"max_over_percent"` // 最大超限百分比
}

// HistoryQuery 历史查询参数
type HistoryQuery struct {
	QuotaID    string     `json:"quota_id,omitempty"`
	VolumeName string     `json:"volume_name,omitempty"`
	StartTime  *time.Time `json:"start_time,omitempty"`
	EndTime    *time.Time `json:"end_time,omitempty"`
	GroupBy    string     `json:"group_by,omitempty"` // hour, day, week, month
	Limit      int        `json:"limit,omitempty"`
}

// HistoryManager 历史数据管理器
type HistoryManager struct {
	mu       sync.RWMutex
	config   HistoryConfig
	records  []*HistoryRecord
	quotaMgr *Manager
	stopChan chan struct{}
	running  bool
}

// NewHistoryManager 创建历史管理器
func NewHistoryManager(quotaMgr *Manager, config HistoryConfig) *HistoryManager {
	return &HistoryManager{
		config:   config,
		records:  make([]*HistoryRecord, 0),
		quotaMgr: quotaMgr,
		stopChan: make(chan struct{}),
	}
}

// Start 启动历史采集
func (m *HistoryManager) Start() {
	if !m.config.CollectEnabled {
		return
	}

	m.mu.Lock()
	m.running = true
	m.mu.Unlock()

	go m.run()
}

// Stop 停止历史采集
func (m *HistoryManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		close(m.stopChan)
		m.running = false
	}

	// 持久化
	if m.config.PersistPath != "" {
		m.persist()
	}
}

// run 运行采集循环
func (m *HistoryManager) run() {
	ticker := time.NewTicker(m.config.CollectPeriod)
	defer ticker.Stop()

	// 首次立即采集
	m.collectAll()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.collectAll()
		}
	}
}

// collectAll 采集所有配额数据
func (m *HistoryManager) collectAll() {
	usages, err := m.quotaMgr.GetAllUsage()
	if err != nil {
		return
	}

	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, usage := range usages {
		record := &HistoryRecord{
			ID:           generateID(),
			QuotaID:      usage.QuotaID,
			TargetName:   usage.TargetName,
			TargetType:   usage.Type,
			VolumeName:   usage.VolumeName,
			UsedBytes:    usage.UsedBytes,
			LimitBytes:   usage.HardLimit,
			UsagePercent: usage.UsagePercent,
			IsOverSoft:   usage.IsOverSoft,
			IsOverHard:   usage.IsOverHard,
			Timestamp:    now,
		}

		m.records = append(m.records, record)
	}

	// 清理过期记录
	m.cleanupRecords()
}

// cleanupRecords 清理过期记录
func (m *HistoryManager) cleanupRecords() {
	// 按数量限制
	if len(m.records) > m.config.MaxRecords {
		m.records = m.records[len(m.records)-m.config.MaxRecords:]
	}

	// 按时间限制
	if m.config.RetentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)
		startIdx := 0
		for i, r := range m.records {
			if r.Timestamp.After(cutoff) {
				startIdx = i
				break
			}
		}
		if startIdx > 0 {
			m.records = m.records[startIdx:]
		}
	}
}

// Record 手动记录数据点
func (m *HistoryManager) Record(record *HistoryRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if record.ID == "" {
		record.ID = generateID()
	}
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	m.records = append(m.records, record)
	m.cleanupRecords()
}

// Query 查询历史数据
func (m *HistoryManager) Query(query HistoryQuery) []*HistoryRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*HistoryRecord, 0)

	for _, r := range m.records {
		// 按配额ID过滤
		if query.QuotaID != "" && r.QuotaID != query.QuotaID {
			continue
		}

		// 按卷名过滤
		if query.VolumeName != "" && r.VolumeName != query.VolumeName {
			continue
		}

		// 按时间过滤
		if query.StartTime != nil && r.Timestamp.Before(*query.StartTime) {
			continue
		}
		if query.EndTime != nil && r.Timestamp.After(*query.EndTime) {
			continue
		}

		result = append(result, r)
	}

	// 限制数量
	if query.Limit > 0 && len(result) > query.Limit {
		result = result[len(result)-query.Limit:]
	}

	return result
}

// GetStatistics 获取历史统计
func (m *HistoryManager) GetStatistics(quotaID string, startTime, endTime time.Time) *HistoryStatistics {
	records := m.Query(HistoryQuery{
		QuotaID:   quotaID,
		StartTime: &startTime,
		EndTime:   &endTime,
	})

	if len(records) == 0 {
		return nil
	}

	stats := &HistoryStatistics{
		QuotaID:     quotaID,
		StartTime:   startTime,
		EndTime:     endTime,
		RecordCount: len(records),
	}

	// 获取配额名称
	m.quotaMgr.mu.RLock()
	if quota, exists := m.quotaMgr.quotas[quotaID]; exists {
		stats.TargetName = quota.TargetName
	}
	m.quotaMgr.mu.RUnlock()

	// 计算统计值
	stats.MinUsedBytes = records[0].UsedBytes
	stats.MaxUsedBytes = records[0].UsedBytes
	stats.MinUsagePercent = records[0].UsagePercent
	stats.MaxUsagePercent = records[0].UsagePercent
	stats.MaxOverPercent = 0

	var totalUsed, totalPercent float64
	for _, r := range records {
		totalUsed += float64(r.UsedBytes)
		totalPercent += r.UsagePercent

		if r.UsedBytes < stats.MinUsedBytes {
			stats.MinUsedBytes = r.UsedBytes
		}
		if r.UsedBytes > stats.MaxUsedBytes {
			stats.MaxUsedBytes = r.UsedBytes
		}
		if r.UsagePercent < stats.MinUsagePercent {
			stats.MinUsagePercent = r.UsagePercent
		}
		if r.UsagePercent > stats.MaxUsagePercent {
			stats.MaxUsagePercent = r.UsagePercent
		}

		if r.IsOverSoft {
			stats.OverSoftCount++
		}
		if r.IsOverHard {
			stats.OverHardCount++
		}
		overPercent := r.UsagePercent - 100
		if overPercent > stats.MaxOverPercent {
			stats.MaxOverPercent = overPercent
		}
	}

	stats.AvgUsedBytes = totalUsed / float64(len(records))
	stats.AvgUsagePercent = totalPercent / float64(len(records))

	// 当前值
	latest := records[len(records)-1]
	stats.CurrentUsedBytes = latest.UsedBytes
	stats.CurrentUsagePercent = latest.UsagePercent

	// 计算增长
	first := records[0]
	stats.TotalGrowthBytes = latest.UsedBytes - first.UsedBytes
	if first.UsedBytes > 0 {
		stats.TotalGrowthPercent = float64(stats.TotalGrowthBytes) / float64(first.UsedBytes) * 100
	}

	// 计算日增长率
	days := latest.Timestamp.Sub(first.Timestamp).Hours() / 24
	if days > 0 {
		stats.DailyGrowthRate = float64(stats.TotalGrowthBytes) / days
	}

	return stats
}

// GetAllStatistics 获取所有配额的历史统计
func (m *HistoryManager) GetAllStatistics(startTime, endTime time.Time) []*HistoryStatistics {
	m.quotaMgr.mu.RLock()
	quotaIDs := make([]string, 0, len(m.quotaMgr.quotas))
	for id := range m.quotaMgr.quotas {
		quotaIDs = append(quotaIDs, id)
	}
	m.quotaMgr.mu.RUnlock()

	stats := make([]*HistoryStatistics, 0, len(quotaIDs))
	for _, id := range quotaIDs {
		if s := m.GetStatistics(id, startTime, endTime); s != nil {
			stats = append(stats, s)
		}
	}

	return stats
}

// persist 持久化历史数据
func (m *HistoryManager) persist() {
	if m.config.PersistPath == "" {
		return
	}

	data := struct {
		Records []*HistoryRecord `json:"records"`
		SavedAt time.Time        `json:"saved_at"`
	}{
		Records: m.records,
		SavedAt: time.Now(),
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	_ = os.MkdirAll(filepath.Dir(m.config.PersistPath), 0750)
	_ = os.WriteFile(m.config.PersistPath, jsonData, 0600)
}

// Load 加载历史数据
func (m *HistoryManager) Load() error {
	if m.config.PersistPath == "" {
		return nil
	}

	if _, err := os.Stat(m.config.PersistPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.config.PersistPath)
	if err != nil {
		return err
	}

	var loaded struct {
		Records []*HistoryRecord `json:"records"`
	}

	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.records = loaded.Records
	return nil
}

// ========== 配额使用图表 API 增强 ==========

// ChartType 图表类型
type ChartType string

// 图表类型常量
const (
	ChartTypeLine    ChartType = "line"    // 折线图
	ChartTypeBar     ChartType = "bar"     // 柱状图
	ChartTypePie     ChartType = "pie"     // 饼图
	ChartTypeArea    ChartType = "area"    // 面积图
	ChartTypeGauge   ChartType = "gauge"   // 仪表盘
	ChartTypeHeatmap ChartType = "heatmap" // 热力图
)

// ChartDataRequest 图表数据请求
type ChartDataRequest struct {
	QuotaID     string    `json:"quota_id,omitempty"`     // 单个配额ID
	VolumeName  string    `json:"volume_name,omitempty"`  // 按卷过滤
	ChartType   ChartType `json:"chart_type"`             // 图表类型
	StartTime   time.Time `json:"start_time"`             // 开始时间
	EndTime     time.Time `json:"end_time"`               // 结束时间
	Granularity string    `json:"granularity,omitempty"`  // 数据粒度: hour, day, week, month
	CompareWith string    `json:"compare_with,omitempty"` // 对比选项: previous_period, same_last_year
}

// ChartDataResponse 图表数据响应
type ChartDataResponse struct {
	ChartType   ChartType     `json:"chart_type"`
	Title       string        `json:"title"`
	GeneratedAt time.Time     `json:"generated_at"`
	Period      ChartPeriod   `json:"period"`
	Series      []ChartSeries `json:"series"`
	Summary     ChartSummary  `json:"summary"`
	Options     ChartOptions  `json:"options,omitempty"`
}

// ChartPeriod 图表时间范围
type ChartPeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Label string    `json:"label"` // 如 "最近7天", "本月" 等
}

// ChartSeries 图表数据系列
type ChartSeries struct {
	Name        string       `json:"name"`
	Type        ChartType    `json:"type,omitempty"`
	Color       string       `json:"color,omitempty"`
	Data        []ChartPoint `json:"data"`
	CompareData []ChartPoint `json:"compare_data,omitempty"` // 对比数据
	Statistics  SeriesStats  `json:"statistics,omitempty"`
}

// ChartPoint 图表数据点
type ChartPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Label     string    `json:"label"`              // X轴标签
	Value     float64   `json:"value"`              // Y轴值
	Value2    float64   `json:"value2,omitempty"`   // 第二Y轴值（如使用量和限制）
	Category  string    `json:"category,omitempty"` // 分类（用于饼图等）
}

// SeriesStats 系列统计
type SeriesStats struct {
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
	Avg    float64 `json:"avg"`
	Sum    float64 `json:"sum"`
	Count  int     `json:"count"`
	Trend  string  `json:"trend"`  // up, down, stable
	Change float64 `json:"change"` // 变化百分比
}

// ChartSummary 图表摘要
type ChartSummary struct {
	TotalUsedBytes   uint64  `json:"total_used_bytes"`
	TotalLimitBytes  uint64  `json:"total_limit_bytes"`
	AvgUsagePercent  float64 `json:"avg_usage_percent"`
	PeakUsagePercent float64 `json:"peak_usage_percent"`
	MinUsagePercent  float64 `json:"min_usage_percent"`
	GrowthPercent    float64 `json:"growth_percent"` // 期间增长百分比
	AlertCount       int     `json:"alert_count"`
	OverLimitCount   int     `json:"over_limit_count"`
}

// ChartOptions 图表选项
type ChartOptions struct {
	YAxisLabel    string `json:"y_axis_label,omitempty"`
	XAxisLabel    string `json:"x_axis_label,omitempty"`
	ShowLegend    bool   `json:"show_legend"`
	ShowGrid      bool   `json:"show_grid"`
	Animated      bool   `json:"animated"`
	FormatBytes   bool   `json:"format_bytes"`   // 是否格式化为字节
	FormatPercent bool   `json:"format_percent"` // 是否格式化为百分比
}

// ChartManager 图表数据管理器
type ChartManager struct {
	quotaMgr   *Manager
	historyMgr *HistoryManager
	trendMgr   *TrendDataManager
}

// NewChartManager 创建图表管理器
func NewChartManager(quotaMgr *Manager, historyMgr *HistoryManager, trendMgr *TrendDataManager) *ChartManager {
	return &ChartManager{
		quotaMgr:   quotaMgr,
		historyMgr: historyMgr,
		trendMgr:   trendMgr,
	}
}

// GetChartData 获取图表数据
func (m *ChartManager) GetChartData(req ChartDataRequest) (*ChartDataResponse, error) {
	response := &ChartDataResponse{
		ChartType:   req.ChartType,
		GeneratedAt: time.Now(),
		Period: ChartPeriod{
			Start: req.StartTime,
			End:   req.EndTime,
		},
		Series: make([]ChartSeries, 0),
	}

	// 设置时间范围
	if req.StartTime.IsZero() {
		req.StartTime = time.Now().AddDate(0, 0, -7)
	}
	if req.EndTime.IsZero() {
		req.EndTime = time.Now()
	}

	response.Period.Start = req.StartTime
	response.Period.End = req.EndTime
	response.Period.Label = formatPeriodLabel(req.StartTime, req.EndTime)

	switch req.ChartType {
	case ChartTypeLine, ChartTypeArea:
		return m.getLineChartData(req, response)
	case ChartTypeBar:
		return m.getBarChartData(req, response)
	case ChartTypePie:
		return m.getPieChartData(req, response)
	case ChartTypeGauge:
		return m.getGaugeChartData(req, response)
	case ChartTypeHeatmap:
		return m.getHeatmapChartData(req, response)
	default:
		return m.getLineChartData(req, response)
	}
}

// getLineChartData 获取折线图数据
func (m *ChartManager) getLineChartData(req ChartDataRequest, response *ChartDataResponse) (*ChartDataResponse, error) {
	response.Title = "配额使用趋势"

	// 获取历史数据
	var records []*HistoryRecord
	if req.QuotaID != "" {
		records = m.historyMgr.Query(HistoryQuery{
			QuotaID:   req.QuotaID,
			StartTime: &req.StartTime,
			EndTime:   &req.EndTime,
		})
	} else {
		records = m.historyMgr.Query(HistoryQuery{
			VolumeName: req.VolumeName,
			StartTime:  &req.StartTime,
			EndTime:    &req.EndTime,
		})
	}

	// 按配额分组
	quotaData := make(map[string][]*HistoryRecord)
	for _, r := range records {
		quotaData[r.QuotaID] = append(quotaData[r.QuotaID], r)
	}

	// 为每个配额创建系列
	for _, data := range quotaData {
		series := ChartSeries{
			Name: data[0].TargetName,
			Type: req.ChartType,
			Data: make([]ChartPoint, 0),
		}

		for _, r := range data {
			point := ChartPoint{
				Timestamp: r.Timestamp,
				Label:     r.Timestamp.Format("01-02 15:04"),
				Value:     r.UsagePercent,
			}
			series.Data = append(series.Data, point)
		}

		// 计算统计
		series.Statistics = m.calculateSeriesStats(series.Data)
		response.Series = append(response.Series, series)
	}

	// 设置摘要
	response.Summary = m.calculateChartSummary(records)

	return response, nil
}

// getBarChartData 获取柱状图数据
func (m *ChartManager) getBarChartData(req ChartDataRequest, response *ChartDataResponse) (*ChartDataResponse, error) {
	response.Title = "配额使用量分布"

	// 获取当前使用情况
	usages, err := m.quotaMgr.GetAllUsage()
	if err != nil {
		return nil, err
	}

	// 过滤
	filtered := make([]*QuotaUsage, 0)
	for _, u := range usages {
		if req.QuotaID != "" && u.QuotaID != req.QuotaID {
			continue
		}
		if req.VolumeName != "" && u.VolumeName != req.VolumeName {
			continue
		}
		filtered = append(filtered, u)
	}

	series := ChartSeries{
		Name: "使用量",
		Type: ChartTypeBar,
		Data: make([]ChartPoint, 0),
	}

	for _, u := range filtered {
		point := ChartPoint{
			Label:    u.TargetName,
			Value:    float64(u.UsedBytes),
			Value2:   float64(u.HardLimit),
			Category: string(u.Type),
		}
		series.Data = append(series.Data, point)
	}

	response.Series = append(response.Series, series)
	response.Options = ChartOptions{
		FormatBytes: true,
		ShowLegend:  true,
	}

	return response, nil
}

// getPieChartData 获取饼图数据
func (m *ChartManager) getPieChartData(req ChartDataRequest, response *ChartDataResponse) (*ChartDataResponse, error) {
	response.Title = "存储使用分布"

	usages, err := m.quotaMgr.GetAllUsage()
	if err != nil {
		return nil, err
	}

	// 按类型分组
	typeData := make(map[Type]uint64)
	for _, u := range usages {
		if req.VolumeName != "" && u.VolumeName != req.VolumeName {
			continue
		}
		typeData[u.Type] += u.UsedBytes
	}

	series := ChartSeries{
		Name: "按类型分布",
		Type: ChartTypePie,
		Data: make([]ChartPoint, 0),
	}

	typeNames := map[Type]string{
		TypeUser:      "用户配额",
		TypeGroup:     "组配额",
		TypeDirectory: "目录配额",
	}

	for t, bytes := range typeData {
		point := ChartPoint{
			Label:    typeNames[t],
			Value:    float64(bytes),
			Category: string(t),
		}
		series.Data = append(series.Data, point)
	}

	response.Series = append(response.Series, series)
	response.Options = ChartOptions{
		FormatBytes: true,
		ShowLegend:  true,
	}

	return response, nil
}

// getGaugeChartData 获取仪表盘数据
func (m *ChartManager) getGaugeChartData(req ChartDataRequest, response *ChartDataResponse) (*ChartDataResponse, error) {
	response.Title = "配额使用率仪表盘"

	usages, err := m.quotaMgr.GetAllUsage()
	if err != nil {
		return nil, err
	}

	var targetUsage *QuotaUsage
	if req.QuotaID != "" {
		for _, u := range usages {
			if u.QuotaID == req.QuotaID {
				targetUsage = u
				break
			}
		}
	} else {
		// 取第一个
		if len(usages) > 0 {
			targetUsage = usages[0]
		}
	}

	if targetUsage == nil {
		return nil, fmt.Errorf("未找到配额数据")
	}

	series := ChartSeries{
		Name: targetUsage.TargetName,
		Type: ChartTypeGauge,
		Data: []ChartPoint{
			{
				Label: targetUsage.TargetName,
				Value: targetUsage.UsagePercent,
			},
		},
	}

	response.Series = append(response.Series, series)
	response.Summary = ChartSummary{
		TotalUsedBytes:  targetUsage.UsedBytes,
		TotalLimitBytes: targetUsage.HardLimit,
		AvgUsagePercent: targetUsage.UsagePercent,
	}

	return response, nil
}

// getHeatmapChartData 获取热力图数据
func (m *ChartManager) getHeatmapChartData(req ChartDataRequest, response *ChartDataResponse) (*ChartDataResponse, error) {
	response.Title = "配额使用热力图"

	records := m.historyMgr.Query(HistoryQuery{
		QuotaID:    req.QuotaID,
		VolumeName: req.VolumeName,
		StartTime:  &req.StartTime,
		EndTime:    &req.EndTime,
	})

	// 按小时和日期聚合
	hourDayData := make(map[string]map[int]float64) // date -> hour -> avg usage

	for _, r := range records {
		date := r.Timestamp.Format("2006-01-02")
		hour := r.Timestamp.Hour()

		if hourDayData[date] == nil {
			hourDayData[date] = make(map[int]float64)
		}
		hourDayData[date][hour] = r.UsagePercent
	}

	series := ChartSeries{
		Name: "使用率热力图",
		Type: ChartTypeHeatmap,
		Data: make([]ChartPoint, 0),
	}

	for date, hours := range hourDayData {
		for hour, usage := range hours {
			point := ChartPoint{
				Label: fmt.Sprintf("%s %d:00", date, hour),
				Value: usage,
			}
			series.Data = append(series.Data, point)
		}
	}

	response.Series = append(response.Series, series)
	return response, nil
}

// calculateSeriesStats 计算系列统计
func (m *ChartManager) calculateSeriesStats(data []ChartPoint) SeriesStats {
	if len(data) == 0 {
		return SeriesStats{}
	}

	stats := SeriesStats{
		Min:   data[0].Value,
		Max:   data[0].Value,
		Count: len(data),
	}

	var sum float64
	for _, p := range data {
		sum += p.Value
		if p.Value < stats.Min {
			stats.Min = p.Value
		}
		if p.Value > stats.Max {
			stats.Max = p.Value
		}
	}

	stats.Avg = sum / float64(len(data))
	stats.Sum = sum

	// 计算趋势
	if len(data) >= 2 {
		first := data[0].Value
		last := data[len(data)-1].Value
		if first > 0 {
			stats.Change = (last - first) / first * 100
		}

		if stats.Change > 5 {
			stats.Trend = "up"
		} else if stats.Change < -5 {
			stats.Trend = "down"
		} else {
			stats.Trend = "stable"
		}
	}

	return stats
}

// calculateChartSummary 计算图表摘要
func (m *ChartManager) calculateChartSummary(records []*HistoryRecord) ChartSummary {
	if len(records) == 0 {
		return ChartSummary{}
	}

	summary := ChartSummary{
		MinUsagePercent: records[0].UsagePercent,
	}

	var totalUsed, totalLimit float64
	var sumPercent float64

	for _, r := range records {
		totalUsed += float64(r.UsedBytes)
		totalLimit += float64(r.LimitBytes)
		sumPercent += r.UsagePercent

		if r.UsagePercent > summary.PeakUsagePercent {
			summary.PeakUsagePercent = r.UsagePercent
		}
		if r.UsagePercent < summary.MinUsagePercent {
			summary.MinUsagePercent = r.UsagePercent
		}

		if r.IsOverSoft || r.IsOverHard {
			summary.OverLimitCount++
		}
	}

	summary.TotalUsedBytes = uint64(totalUsed / float64(len(records)))
	summary.TotalLimitBytes = uint64(totalLimit / float64(len(records)))
	summary.AvgUsagePercent = sumPercent / float64(len(records))

	// 计算增长
	if len(records) >= 2 {
		first := records[0]
		last := records[len(records)-1]
		if first.UsedBytes > 0 {
			summary.GrowthPercent = float64(last.UsedBytes-first.UsedBytes) / float64(first.UsedBytes) * 100
		}
	}

	return summary
}

// formatPeriodLabel 格式化时间范围标签
func formatPeriodLabel(start, end time.Time) string {
	days := int(end.Sub(start).Hours() / 24)
	switch {
	case days <= 1:
		return "今天"
	case days <= 7:
		return "最近7天"
	case days <= 30:
		return "最近30天"
	case days <= 90:
		return "最近3个月"
	case days <= 365:
		return "最近一年"
	default:
		return fmt.Sprintf("%s 至 %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
}

// ========== 预警通知增强 ==========

// NotificationType 通知类型
type NotificationType string

// 通知类型常量
const (
	NotificationEmail    NotificationType = "email"
	NotificationWebhook  NotificationType = "webhook"
	NotificationSlack    NotificationType = "slack"
	NotificationDiscord  NotificationType = "discord"
	NotificationTelegram NotificationType = "telegram"
	NotificationWechat   NotificationType = "wechat"
	NotificationDingtalk NotificationType = "dingtalk"
)

// AlertNotification 预警通知
type AlertNotification struct {
	ID         string                 `json:"id"`
	AlertID    string                 `json:"alert_id"`
	Type       NotificationType       `json:"type"`
	ChannelID  string                 `json:"channel_id"`
	Recipient  string                 `json:"recipient"`
	Subject    string                 `json:"subject"`
	Message    string                 `json:"message"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Status     string                 `json:"status"` // pending, sent, failed
	Error      string                 `json:"error,omitempty"`
	RetryCount int                    `json:"retry_count"`
	SentAt     *time.Time             `json:"sent_at,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// NotificationTemplate 通知模板
type NotificationTemplate struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Type      NotificationType `json:"type"`
	Subject   string           `json:"subject,omitempty"`
	Body      string           `json:"body"`
	Severity  []AlertSeverity  `json:"severity"`
	Variables []string         `json:"variables"`
}

// NotificationManager 通知管理器
type NotificationManager struct {
	mu         sync.RWMutex
	templates  map[string]*NotificationTemplate
	channels   map[string]*NotificationChannel
	history    []*AlertNotification
	maxHistory int
}

// NewNotificationManager 创建通知管理器
func NewNotificationManager() *NotificationManager {
	return &NotificationManager{
		templates:  make(map[string]*NotificationTemplate),
		channels:   make(map[string]*NotificationChannel),
		history:    make([]*AlertNotification, 0),
		maxHistory: 1000,
	}
}

// SendNotification 发送通知
func (m *NotificationManager) SendNotification(alert *Alert, channelID string) (*AlertNotification, error) {
	m.mu.RLock()
	channel, exists := m.channels[channelID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("通知渠道不存在: %s", channelID)
	}

	notification := &AlertNotification{
		ID:        generateID(),
		AlertID:   alert.ID,
		Type:      NotificationType(channel.Type),
		ChannelID: channelID,
		Recipient: getChannelRecipient(channel),
		Subject:   fmt.Sprintf("[NAS-OS] 配额告警 - %s", alert.TargetName),
		Message:   alert.Message,
		Data: map[string]interface{}{
			"alert_id":      alert.ID,
			"quota_id":      alert.QuotaID,
			"target_name":   alert.TargetName,
			"usage_percent": alert.UsagePercent,
			"used_bytes":    alert.UsedBytes,
			"limit_bytes":   alert.LimitBytes,
			"severity":      alert.Severity,
			"created_at":    alert.CreatedAt,
		},
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	// 发送通知
	err := m.dispatch(notification, channel)
	if err != nil {
		notification.Status = "failed"
		notification.Error = err.Error()
	} else {
		notification.Status = "sent"
		now := time.Now()
		notification.SentAt = &now
	}

	// 记录历史
	m.mu.Lock()
	m.history = append(m.history, notification)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
	m.mu.Unlock()

	return notification, err
}

// dispatch 分发通知
func (m *NotificationManager) dispatch(notification *AlertNotification, channel *NotificationChannel) error {
	switch NotificationType(channel.Type) {
	case NotificationWebhook:
		return m.sendWebhook(notification, channel)
	case NotificationEmail:
		return m.sendEmail(notification, channel)
	case NotificationSlack:
		return m.sendSlack(notification, channel)
	case NotificationDiscord:
		return m.sendDiscord(notification, channel)
	case NotificationTelegram:
		return m.sendTelegram(notification, channel)
	default:
		return fmt.Errorf("不支持的通知类型: %s", channel.Type)
	}
}

// sendWebhook 发送 Webhook 通知
func (m *NotificationManager) sendWebhook(notification *AlertNotification, channel *NotificationChannel) error {
	url, ok := channel.Config["url"].(string)
	if !ok {
		return fmt.Errorf("webhook URL 未配置")
	}

	// 简化实现，实际应该发送 HTTP 请求
	fmt.Printf("[quota] 发送 webhook 通知: %s -> %s\n", notification.AlertID, url)
	return nil
}

// sendEmail 发送邮件通知
func (m *NotificationManager) sendEmail(notification *AlertNotification, channel *NotificationChannel) error {
	to, ok := channel.Config["to"].(string)
	if !ok {
		return fmt.Errorf("邮件收件人未配置")
	}

	fmt.Printf("[quota] 发送邮件通知: %s -> %s\n", notification.AlertID, to)
	return nil
}

// sendSlack 发送 Slack 通知
func (m *NotificationManager) sendSlack(notification *AlertNotification, channel *NotificationChannel) error {
	webhook, ok := channel.Config["webhook"].(string)
	if !ok {
		return fmt.Errorf("slack webhook 未配置")
	}

	fmt.Printf("[quota] 发送 Slack 通知: %s -> %s\n", notification.AlertID, webhook)
	return nil
}

// sendDiscord 发送 Discord 通知
func (m *NotificationManager) sendDiscord(notification *AlertNotification, channel *NotificationChannel) error {
	webhook, ok := channel.Config["webhook"].(string)
	if !ok {
		return fmt.Errorf("discord webhook 未配置")
	}

	fmt.Printf("[quota] 发送 Discord 通知: %s -> %s\n", notification.AlertID, webhook)
	return nil
}

// sendTelegram 发送 Telegram 通知
func (m *NotificationManager) sendTelegram(notification *AlertNotification, channel *NotificationChannel) error {
	botToken, ok := channel.Config["bot_token"].(string)
	if !ok || botToken == "" {
		return fmt.Errorf("telegram bot token 未配置")
	}

	// 注意：不记录 bot_token 到日志，避免敏感信息泄露
	fmt.Printf("[quota] 发送 Telegram 通知: alert_id=%s\n", notification.AlertID)
	return nil
}

// getChannelRecipient 获取渠道收件人
func getChannelRecipient(channel *NotificationChannel) string {
	switch NotificationType(channel.Type) {
	case NotificationEmail:
		if to, ok := channel.Config["to"].(string); ok {
			return to
		}
	case NotificationWebhook:
		if url, ok := channel.Config["url"].(string); ok {
			return url
		}
	}
	return ""
}

// AddChannel 添加通知渠道
func (m *NotificationManager) AddChannel(channel *NotificationChannel) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if channel.ID == "" {
		channel.ID = generateID()
	}
	m.channels[channel.ID] = channel
}

// RemoveChannel 移除通知渠道
func (m *NotificationManager) RemoveChannel(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.channels, id)
}

// GetChannels 获取所有通知渠道
func (m *NotificationManager) GetChannels() []*NotificationChannel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*NotificationChannel, 0, len(m.channels))
	for _, c := range m.channels {
		result = append(result, c)
	}
	return result
}

// GetHistory 获取通知历史
func (m *NotificationManager) GetHistory(limit int) []*AlertNotification {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.history) {
		limit = len(m.history)
	}

	result := make([]*AlertNotification, limit)
	copy(result, m.history[len(m.history)-limit:])
	return result
}

// ========== 用户资源使用报告 ==========

// UserResourceReport 用户资源使用报告
type UserResourceReport struct {
	Username        string             `json:"username"`
	GeneratedAt     time.Time          `json:"generated_at"`
	Period          ReportPeriod       `json:"period"`
	Quotas          []UserQuotaDetail  `json:"quotas"`
	Summary         UserReportSummary  `json:"summary"`
	Trend           *UserTrendAnalysis `json:"trend,omitempty"`
	Recommendations []string           `json:"recommendations"`
}

// UserQuotaDetail 用户配额详情
type UserQuotaDetail struct {
	QuotaID        string    `json:"quota_id"`
	VolumeName     string    `json:"volume_name"`
	Path           string    `json:"path"`
	HardLimit      uint64    `json:"hard_limit"`
	SoftLimit      uint64    `json:"soft_limit"`
	UsedBytes      uint64    `json:"used_bytes"`
	AvailableBytes uint64    `json:"available_bytes"`
	UsagePercent   float64   `json:"usage_percent"`
	Status         string    `json:"status"` // normal, warning, critical
	LastUpdated    time.Time `json:"last_updated"`

	// 文件统计
	FileCount      int        `json:"file_count,omitempty"`
	DirectoryCount int        `json:"directory_count,omitempty"`
	TopFiles       []FileInfo `json:"top_files,omitempty"`
}

// FileInfo 文件信息
type FileInfo struct {
	Path    string    `json:"path"`
	Size    uint64    `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Type    string    `json:"type,omitempty"`
}

// UserReportSummary 用户报告摘要
type UserReportSummary struct {
	TotalQuotas     int     `json:"total_quotas"`
	TotalLimitBytes uint64  `json:"total_limit_bytes"`
	TotalUsedBytes  uint64  `json:"total_used_bytes"`
	TotalAvailable  uint64  `json:"total_available"`
	AvgUsagePercent float64 `json:"avg_usage_percent"`
	WarningCount    int     `json:"warning_count"`
	CriticalCount   int     `json:"critical_count"`
}

// UserTrendAnalysis 用户趋势分析
type UserTrendAnalysis struct {
	DailyGrowthBytes    float64   `json:"daily_growth_bytes"`
	GrowthTrend         string    `json:"growth_trend"` // increasing, decreasing, stable
	PredictedDaysToFull int       `json:"predicted_days_to_full,omitempty"`
	WeeklyPattern       []float64 `json:"weekly_pattern,omitempty"` // 每天平均使用率
}

// ReportGeneratorEnhanced 增强的报告生成器
type ReportGeneratorEnhanced struct {
	quotaMgr   *Manager
	historyMgr *HistoryManager
	trendMgr   *TrendDataManager
}

// NewReportGeneratorEnhanced 创建增强报告生成器
func NewReportGeneratorEnhanced(quotaMgr *Manager, historyMgr *HistoryManager, trendMgr *TrendDataManager) *ReportGeneratorEnhanced {
	return &ReportGeneratorEnhanced{
		quotaMgr:   quotaMgr,
		historyMgr: historyMgr,
		trendMgr:   trendMgr,
	}
}

// GenerateUserReport 生成用户资源报告
func (g *ReportGeneratorEnhanced) GenerateUserReport(username string, period ReportPeriod) (*UserResourceReport, error) {
	report := &UserResourceReport{
		Username:        username,
		GeneratedAt:     time.Now(),
		Period:          period,
		Quotas:          make([]UserQuotaDetail, 0),
		Recommendations: make([]string, 0),
	}

	// 获取用户配额
	usages, err := g.quotaMgr.GetUserUsage(username)
	if err != nil {
		return nil, err
	}

	// 构建配额详情
	for _, usage := range usages {
		detail := UserQuotaDetail{
			QuotaID:        usage.QuotaID,
			VolumeName:     usage.VolumeName,
			Path:           usage.Path,
			HardLimit:      usage.HardLimit,
			SoftLimit:      usage.SoftLimit,
			UsedBytes:      usage.UsedBytes,
			AvailableBytes: usage.Available,
			UsagePercent:   usage.UsagePercent,
			LastUpdated:    usage.LastChecked,
		}

		// 确定状态
		if usage.IsOverHard {
			detail.Status = "critical"
		} else if usage.IsOverSoft {
			detail.Status = "warning"
		} else {
			detail.Status = "normal"
		}

		report.Quotas = append(report.Quotas, detail)

		// 更新摘要
		report.Summary.TotalQuotas++
		report.Summary.TotalLimitBytes += usage.HardLimit
		report.Summary.TotalUsedBytes += usage.UsedBytes
		report.Summary.TotalAvailable += usage.Available

		switch detail.Status {
		case "warning":
			report.Summary.WarningCount++
		case "critical":
			report.Summary.CriticalCount++
		}
	}

	// 计算平均使用率
	if report.Summary.TotalLimitBytes > 0 {
		report.Summary.AvgUsagePercent = float64(report.Summary.TotalUsedBytes) / float64(report.Summary.TotalLimitBytes) * 100
	}

	// 生成建议
	report.Recommendations = g.generateUserRecommendations(report)

	return report, nil
}

// generateUserRecommendations 生成用户建议
func (g *ReportGeneratorEnhanced) generateUserRecommendations(report *UserResourceReport) []string {
	recs := make([]string, 0)

	if report.Summary.CriticalCount > 0 {
		recs = append(recs, "您有配额已超过硬限制，请立即清理文件或申请增加配额")
	}

	if report.Summary.WarningCount > 0 {
		recs = append(recs, "部分配额使用率较高，建议检查并清理不需要的文件")
	}

	if report.Summary.AvgUsagePercent > 80 {
		recs = append(recs, "整体存储使用率较高，建议定期清理或申请更多存储空间")
	}

	for _, quota := range report.Quotas {
		if quota.UsagePercent > 90 {
			recs = append(recs, fmt.Sprintf("卷 %s 上的配额使用率已达 %.1f%%，请尽快处理", quota.VolumeName, quota.UsagePercent))
		}
	}

	if len(recs) == 0 {
		recs = append(recs, "您的存储使用情况良好")
	}

	return recs
}

// GenerateSystemReport 生成系统资源报告
func (g *ReportGeneratorEnhanced) GenerateSystemReport(period ReportPeriod) (*SystemResourceReport, error) {
	report := &SystemResourceReport{
		GeneratedAt: time.Now(),
		Period:      period,
	}

	// 获取所有配额使用情况
	usages, err := g.quotaMgr.GetAllUsage()
	if err != nil {
		return nil, err
	}

	// 按卷分组
	volumeMap := make(map[string]*VolumeResourceInfo)
	for _, usage := range usages {
		if _, exists := volumeMap[usage.VolumeName]; !exists {
			volumeMap[usage.VolumeName] = &VolumeResourceInfo{
				VolumeName: usage.VolumeName,
			}
		}

		vol := volumeMap[usage.VolumeName]
		vol.TotalLimit += usage.HardLimit
		vol.TotalUsed += usage.UsedBytes
		vol.TotalAvailable += usage.Available
		vol.QuotaCount++

		switch usage.Type {
		case TypeUser:
			vol.UserCount++
		case TypeGroup:
			vol.GroupCount++
		}

		if usage.IsOverHard {
			vol.OverHardCount++
		} else if usage.IsOverSoft {
			vol.OverSoftCount++
		}
	}

	// 转换为切片
	for _, vol := range volumeMap {
		if vol.TotalLimit > 0 {
			vol.AvgUsagePercent = float64(vol.TotalUsed) / float64(vol.TotalLimit) * 100
		}
		report.Volumes = append(report.Volumes, *vol)
		report.Summary.TotalQuotas += vol.QuotaCount
		report.Summary.TotalUsedBytes += vol.TotalUsed
		report.Summary.TotalLimitBytes += vol.TotalLimit
	}

	// 计算整体统计
	if report.Summary.TotalLimitBytes > 0 {
		report.Summary.AvgUsagePercent = float64(report.Summary.TotalUsedBytes) / float64(report.Summary.TotalLimitBytes) * 100
	}

	// 获取趋势分析
	if g.trendMgr != nil {
		report.Trend = g.generateSystemTrend(usages)
	}

	return report, nil
}

// SystemResourceReport 系统资源报告
type SystemResourceReport struct {
	GeneratedAt time.Time            `json:"generated_at"`
	Period      ReportPeriod         `json:"period"`
	Summary     SystemSummary        `json:"summary"`
	Volumes     []VolumeResourceInfo `json:"volumes"`
	Trend       *SystemTrend         `json:"trend,omitempty"`
	Alerts      []AlertSummary       `json:"alerts,omitempty"`
}

// SystemSummary 系统摘要
type SystemSummary struct {
	TotalQuotas     int     `json:"total_quotas"`
	TotalUsers      int     `json:"total_users"`
	TotalGroups     int     `json:"total_groups"`
	TotalUsedBytes  uint64  `json:"total_used_bytes"`
	TotalLimitBytes uint64  `json:"total_limit_bytes"`
	AvgUsagePercent float64 `json:"avg_usage_percent"`
	OverSoftCount   int     `json:"over_soft_count"`
	OverHardCount   int     `json:"over_hard_count"`
}

// VolumeResourceInfo 卷资源信息
type VolumeResourceInfo struct {
	VolumeName      string  `json:"volume_name"`
	QuotaCount      int     `json:"quota_count"`
	UserCount       int     `json:"user_count"`
	GroupCount      int     `json:"group_count"`
	TotalUsed       uint64  `json:"total_used"`
	TotalLimit      uint64  `json:"total_limit"`
	TotalAvailable  uint64  `json:"total_available"`
	AvgUsagePercent float64 `json:"avg_usage_percent"`
	OverSoftCount   int     `json:"over_soft_count"`
	OverHardCount   int     `json:"over_hard_count"`
}

// SystemTrend 系统趋势
type SystemTrend struct {
	DailyGrowthBytes    float64   `json:"daily_growth_bytes"`
	GrowthTrend         string    `json:"growth_trend"`
	PredictedDaysToFull int       `json:"predicted_days_to_full,omitempty"`
	PeakUsageTime       string    `json:"peak_usage_time,omitempty"`
	WeeklyPattern       []float64 `json:"weekly_pattern,omitempty"`
}

// AlertSummary 告警摘要
type AlertSummary struct {
	TotalAlerts   int `json:"total_alerts"`
	CriticalCount int `json:"critical_count"`
	WarningCount  int `json:"warning_count"`
	ResolvedCount int `json:"resolved_count"`
}

// generateSystemTrend 生成系统趋势
func (g *ReportGeneratorEnhanced) generateSystemTrend(usages []*QuotaUsage) *SystemTrend {
	trend := &SystemTrend{
		GrowthTrend: "stable",
	}

	// 计算整体增长率
	var totalGrowthRate float64
	var count int

	for _, usage := range usages {
		stats := g.trendMgr.GetTrendStats(usage.QuotaID, 24*time.Hour*7)
		if stats != nil && stats.DailyGrowthRate != 0 {
			totalGrowthRate += stats.DailyGrowthRate
			count++
		}
	}

	if count > 0 {
		trend.DailyGrowthBytes = totalGrowthRate / float64(count)

		if trend.DailyGrowthBytes > 0 {
			trend.GrowthTrend = "increasing"
		} else if trend.DailyGrowthBytes < 0 {
			trend.GrowthTrend = "decreasing"
		}
	}

	return trend
}

// ========== 存储使用统计 API ==========

// StorageStats 存储统计
type StorageStats struct {
	VolumeName      string        `json:"volume_name"`
	TotalBytes      uint64        `json:"total_bytes"`
	UsedBytes       uint64        `json:"used_bytes"`
	FreeBytes       uint64        `json:"free_bytes"`
	UsagePercent    float64       `json:"usage_percent"`
	QuotaCount      int           `json:"quota_count"`
	UserQuotas      []QuotaUsage  `json:"user_quotas"`
	GroupQuotas     []QuotaUsage  `json:"group_quotas"`
	DirectoryQuotas []QuotaUsage  `json:"directory_quotas"`
	TopUsers        []QuotaUsage  `json:"top_users"` // 使用量最高的用户
	AlertCount      int           `json:"alert_count"`
	Trend           *StorageTrend `json:"trend,omitempty"`
}

// StorageTrend 存储趋势
type StorageTrend struct {
	DailyGrowthBytes float64 `json:"daily_growth_bytes"`
	GrowthPercent    float64 `json:"growth_percent"`
	DaysToFull       int     `json:"days_to_full,omitempty"`
}

// StorageStatsManager 存储统计管理器
type StorageStatsManager struct {
	quotaMgr   *Manager
	historyMgr *HistoryManager
	trendMgr   *TrendDataManager
}

// NewStorageStatsManager 创建存储统计管理器
func NewStorageStatsManager(quotaMgr *Manager, historyMgr *HistoryManager, trendMgr *TrendDataManager) *StorageStatsManager {
	return &StorageStatsManager{
		quotaMgr:   quotaMgr,
		historyMgr: historyMgr,
		trendMgr:   trendMgr,
	}
}

// GetStorageStats 获取存储统计
func (m *StorageStatsManager) GetStorageStats(volumeName string) (*StorageStats, error) {
	usages, err := m.quotaMgr.GetAllUsage()
	if err != nil {
		return nil, err
	}

	stats := &StorageStats{
		VolumeName:      volumeName,
		UserQuotas:      make([]QuotaUsage, 0),
		GroupQuotas:     make([]QuotaUsage, 0),
		DirectoryQuotas: make([]QuotaUsage, 0),
		TopUsers:        make([]QuotaUsage, 0),
	}

	// 过滤并分类
	for _, usage := range usages {
		if volumeName != "" && usage.VolumeName != volumeName {
			continue
		}

		stats.TotalBytes += usage.HardLimit
		stats.UsedBytes += usage.UsedBytes
		stats.FreeBytes += usage.Available
		stats.QuotaCount++

		switch usage.Type {
		case TypeUser:
			stats.UserQuotas = append(stats.UserQuotas, *usage)
		case TypeGroup:
			stats.GroupQuotas = append(stats.GroupQuotas, *usage)
		case TypeDirectory:
			stats.DirectoryQuotas = append(stats.DirectoryQuotas, *usage)
		}
	}

	// 计算使用率
	if stats.TotalBytes > 0 {
		stats.UsagePercent = float64(stats.UsedBytes) / float64(stats.TotalBytes) * 100
	}

	// 获取使用量最高的用户（前5名）
	stats.TopUsers = m.getTopUsers(stats.UserQuotas, 5)

	// 获取告警数量
	alerts := m.quotaMgr.GetAlerts()
	for _, alert := range alerts {
		if volumeName == "" || alert.VolumeName == volumeName {
			stats.AlertCount++
		}
	}

	// 添加趋势数据
	if m.trendMgr != nil && stats.QuotaCount > 0 {
		stats.Trend = m.calculateStorageTrend(usages)
	}

	return stats, nil
}

// getTopUsers 获取使用量最高的用户
func (m *StorageStatsManager) getTopUsers(users []QuotaUsage, limit int) []QuotaUsage {
	if len(users) == 0 {
		return nil
	}

	// 复制并排序
	sorted := make([]QuotaUsage, len(users))
	copy(sorted, users)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].UsedBytes > sorted[i].UsedBytes {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	if limit > 0 && len(sorted) > limit {
		sorted = sorted[:limit]
	}

	return sorted
}

// calculateStorageTrend 计算存储趋势
func (m *StorageStatsManager) calculateStorageTrend(usages []*QuotaUsage) *StorageTrend {
	trend := &StorageTrend{}

	var totalGrowthRate float64
	var count int

	for _, usage := range usages {
		if m.trendMgr != nil {
			stats := m.trendMgr.GetTrendStats(usage.QuotaID, 24*time.Hour*7)
			if stats != nil && stats.DailyGrowthRate != 0 {
				totalGrowthRate += stats.DailyGrowthRate
				count++
			}
		}
	}

	if count > 0 {
		trend.DailyGrowthBytes = totalGrowthRate / float64(count)
	}

	// 计算预计填满时间
	if trend.DailyGrowthBytes > 0 {
		var totalFree uint64
		for _, usage := range usages {
			totalFree += usage.Available
		}

		if totalFree > 0 {
			trend.DaysToFull = int(float64(totalFree) / trend.DailyGrowthBytes)
		}
	}

	return trend
}

// GetAllVolumesStats 获取所有卷的统计
func (m *StorageStatsManager) GetAllVolumesStats() ([]*StorageStats, error) {
	usages, err := m.quotaMgr.GetAllUsage()
	if err != nil {
		return nil, err
	}

	// 按卷分组
	volumeMap := make(map[string][]*QuotaUsage)
	for _, usage := range usages {
		volumeMap[usage.VolumeName] = append(volumeMap[usage.VolumeName], usage)
	}

	result := make([]*StorageStats, 0, len(volumeMap))
	for volumeName := range volumeMap {
		stats, err := m.GetStorageStats(volumeName)
		if err == nil {
			result = append(result, stats)
		}
	}

	return result, nil
}

// GetGlobalStats 获取全局统计
func (m *StorageStatsManager) GetGlobalStats() (*StorageStats, error) {
	return m.GetStorageStats("")
}

// Helper function
func formatBytesImpl(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
