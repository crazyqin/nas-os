// Package datainsight 数据分析平台
// 对标群晖数据分析和报表功能
package datainsight

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ReportType 报表类型
type ReportType string

const (
	ReportTypeStorage   ReportType = "storage"   // 存储分析
	ReportTypeUsage     ReportType = "usage"      // 使用分析
	ReportTypePerformance ReportType = "performance" // 性能分析
	ReportTypeSecurity  ReportType = "security"   // 安全分析
	ReportTypeCustom    ReportType = "custom"     // 自定义报表
)

// ChartType 图表类型
type ChartType string

const (
	ChartTypeLine    ChartType = "line"    // 折线图
	ChartTypeBar     ChartType = "bar"     // 柱状图
	ChartTypePie     ChartType = "pie"     // 饼图
	ChartTypeArea    ChartType = "area"    // 面积图
	ChartTypeScatter ChartType = "scatter" // 散点图
	ChartTypeHeatmap ChartType = "heatmap" // 热力图
)

// TimeRange 时间范围
type TimeRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// DataPoint 数据点
type DataPoint struct {
	Timestamp time.Time   `json:"timestamp"`
	Value     interface{} `json:"value"`
	Label     string      `json:"label,omitempty"`
}

// DataSeries 数据系列
type DataSeries struct {
	Name   string      `json:"name"`
	Points []DataPoint `json:"points"`
}

// Query 数据查询
type Query struct {
	Metric    string            `json:"metric"`
	Filters   map[string]string `json:"filters,omitempty"`
	GroupBy   []string          `json:"group_by,omitempty"`
	Aggregation string          `json:"aggregation"` // sum, avg, min, max, count
	TimeRange TimeRange         `json:"time_range"`
	Interval  time.Duration     `json:"interval,omitempty"`
}

// QueryResult 查询结果
type QueryResult struct {
	Query     Query       `json:"query"`
	Series    []DataSeries `json:"series"`
	Total     int64       `json:"total"`
	StartTime time.Time   `json:"start_time"`
	EndTime   time.Time   `json:"end_time"`
}

// Widget 仪表盘组件
type Widget struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Type        ChartType         `json:"type"`
	Query       Query             `json:"query"`
	Position    Position          `json:"position"`
	Size        Size              `json:"size"`
	Options     map[string]interface{} `json:"options,omitempty"`
	RefreshRate time.Duration     `json:"refresh_rate,omitempty"`
}

// Position 位置
type Position struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Size 尺寸
type Size struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Dashboard 仪表盘
type Dashboard struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Widgets     []Widget  `json:"widgets"`
	Layout      string    `json:"layout"` // grid, free
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tags        []string  `json:"tags,omitempty"`
}

// Report 报表定义
type Report struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Type        ReportType `json:"type"`
	Queries     []Query    `json:"queries"`
	Schedule    string     `json:"schedule,omitempty"` // cron expression
	Format      string     `json:"format"`             // pdf, html, csv
	Recipients  []string   `json:"recipients,omitempty"`
	LastRun     *time.Time `json:"last_run,omitempty"`
	NextRun     *time.Time `json:"next_run,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// AlertRule 告警规则
type AlertRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Query       Query       `json:"query"`
	Condition   string      `json:"condition"` // above, below, equal, change
	Threshold   float64     `json:"threshold"`
	Duration    time.Duration `json:"duration"`
	Severity    string      `json:"severity"` // critical, warning, info
	Enabled     bool        `json:"enabled"`
	LastTriggered *time.Time `json:"last_triggered,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
}

// Alert 告警记录
type Alert struct {
	ID          string    `json:"id"`
	RuleID      string    `json:"rule_id"`
	RuleName    string    `json:"rule_name"`
	Severity    string    `json:"severity"`
	Message     string    `json:"message"`
	Value       float64   `json:"value"`
	Threshold   float64   `json:"threshold"`
	Status      string    `json:"status"` // firing, resolved
	StartedAt   time.Time `json:"started_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
}

// MetricCollector 指标收集器
type MetricCollector interface {
	Collect(ctx context.Context, metric string, filters map[string]string) ([]DataPoint, error)
	ListMetrics() []string
}

// Manager 数据分析管理器
type Manager struct {
	mu         sync.RWMutex
	dashboards map[string]*Dashboard
	reports    map[string]*Report
	alertRules map[string]*AlertRule
	alerts     map[string][]*Alert
	collector  MetricCollector
	logger     Logger
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewManager 创建数据分析管理器
func NewManager(collector MetricCollector, logger Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		dashboards: make(map[string]*Dashboard),
		reports:    make(map[string]*Report),
		alertRules: make(map[string]*AlertRule),
		alerts:     make(map[string][]*Alert),
		collector:  collector,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}

	// 启动告警检查循环
	m.wg.Add(1)
	go m.alertCheckLoop()

	// 启动报表调度循环
	m.wg.Add(1)
	go m.reportScheduleLoop()

	return m
}

// CreateDashboard 创建仪表盘
func (m *Manager) CreateDashboard(dashboard *Dashboard) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if dashboard.ID == "" {
		dashboard.ID = generateDashboardID()
	}
	dashboard.CreatedAt = time.Now()
	dashboard.UpdatedAt = time.Now()

	m.dashboards[dashboard.ID] = dashboard
	m.logger.Info("仪表盘创建成功: %s (%s)", dashboard.Name, dashboard.ID)
	return nil
}

// UpdateDashboard 更新仪表盘
func (m *Manager) UpdateDashboard(dashboard *Dashboard) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.dashboards[dashboard.ID]
	if !ok {
		return fmt.Errorf("仪表盘不存在: %s", dashboard.ID)
	}

	dashboard.CreatedAt = existing.CreatedAt
	dashboard.UpdatedAt = time.Now()
	m.dashboards[dashboard.ID] = dashboard
	m.logger.Info("仪表盘更新成功: %s (%s)", dashboard.Name, dashboard.ID)
	return nil
}

// DeleteDashboard 删除仪表盘
func (m *Manager) DeleteDashboard(dashboardID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.dashboards[dashboardID]; !ok {
		return fmt.Errorf("仪表盘不存在: %s", dashboardID)
	}

	delete(m.dashboards, dashboardID)
	m.logger.Info("仪表盘删除成功: %s", dashboardID)
	return nil
}

// GetDashboard 获取仪表盘
func (m *Manager) GetDashboard(dashboardID string) (*Dashboard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboard, ok := m.dashboards[dashboardID]
	if !ok {
		return nil, fmt.Errorf("仪表盘不存在: %s", dashboardID)
	}
	return dashboard, nil
}

// ListDashboards 列出所有仪表盘
func (m *Manager) ListDashboards() []*Dashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboards := make([]*Dashboard, 0, len(m.dashboards))
	for _, dashboard := range m.dashboards {
		dashboards = append(dashboards, dashboard)
	}
	return dashboards
}

// ExecuteQuery 执行数据查询
func (m *Manager) ExecuteQuery(ctx context.Context, query Query) (*QueryResult, error) {
	startTime := time.Now()

	// 收集数据
	points, err := m.collector.Collect(ctx, query.Metric, query.Filters)
	if err != nil {
		return nil, fmt.Errorf("数据收集失败: %v", err)
	}

	// 分组
	grouped := m.groupData(points, query.GroupBy)

	// 聚合
	series := make([]DataSeries, 0, len(grouped))
	for name, groupPoints := range grouped {
		aggregated := m.aggregateData(groupPoints, query.Aggregation, query.TimeRange, query.Interval)
		series = append(series, DataSeries{
			Name:   name,
			Points: aggregated,
		})
	}

	result := &QueryResult{
		Query:     query,
		Series:    series,
		Total:     int64(len(points)),
		StartTime: startTime,
		EndTime:   time.Now(),
	}

	m.logger.Debug("查询执行完成: %s, 数据点: %d", query.Metric, len(points))
	return result, nil
}

// groupData 数据分组
func (m *Manager) groupData(points []DataPoint, groupBy []string) map[string][]DataPoint {
	if len(groupBy) == 0 {
		return map[string][]DataPoint{"default": points}
	}

	grouped := make(map[string][]DataPoint)
	for _, point := range points {
		key := ""
		for _, field := range groupBy {
			if field == "label" {
				key += point.Label
			}
		}
		if key == "" {
			key = "default"
		}
		grouped[key] = append(grouped[key], point)
	}
	return grouped
}

// aggregateData 数据聚合
func (m *Manager) aggregateData(points []DataPoint, aggregation string, timeRange TimeRange, interval time.Duration) []DataPoint {
	if interval == 0 {
		interval = time.Hour
	}

	// 按时间间隔分组
	buckets := make(map[int64][]float64)
	for _, point := range points {
		if point.Timestamp.Before(timeRange.Start) || point.Timestamp.After(timeRange.End) {
			continue
		}

		bucketKey := point.Timestamp.Unix() / int64(interval.Seconds())
		if val, ok := toFloat64(point.Value); ok {
			buckets[bucketKey] = append(buckets[bucketKey], val)
		}
	}

	// 聚合每个桶
	result := make([]DataPoint, 0, len(buckets))
	for bucketKey, values := range buckets {
		timestamp := time.Unix(bucketKey*int64(interval.Seconds()), 0)
		var value float64

		switch aggregation {
		case "sum":
			for _, v := range values {
				value += v
			}
		case "avg":
			for _, v := range values {
				value += v
			}
			if len(values) > 0 {
				value /= float64(len(values))
			}
		case "min":
			value = values[0]
			for _, v := range values[1:] {
				if v < value {
					value = v
				}
			}
		case "max":
			value = values[0]
			for _, v := range values[1:] {
				if v > value {
					value = v
				}
			}
		case "count":
			value = float64(len(values))
		}

		result = append(result, DataPoint{
			Timestamp: timestamp,
			Value:     value,
		})
	}

	return result
}

// toFloat64 转换为 float64
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	default:
		return 0, false
	}
}

// CreateReport 创建报表
func (m *Manager) CreateReport(report *Report) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if report.ID == "" {
		report.ID = generateReportID()
	}
	report.CreatedAt = time.Now()
	report.UpdatedAt = time.Now()

	m.reports[report.ID] = report
	m.logger.Info("报表创建成功: %s (%s)", report.Name, report.ID)
	return nil
}

// GenerateReport 生成报表
func (m *Manager) GenerateReport(ctx context.Context, reportID string) ([]byte, error) {
	m.mu.RLock()
	report, ok := m.reports[reportID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("报表不存在: %s", reportID)
	}

	// 执行所有查询
	results := make([]*QueryResult, 0, len(report.Queries))
	for _, query := range report.Queries {
		result, err := m.ExecuteQuery(ctx, query)
		if err != nil {
			m.logger.Error("报表查询失败: %v", err)
			continue
		}
		results = append(results, result)
	}

	// 根据格式生成报表
	switch report.Format {
	case "json":
		return json.Marshal(results)
	case "csv":
		return m.generateCSV(results)
	default:
		return json.Marshal(results)
	}
}

// generateCSV 生成 CSV 格式
func (m *Manager) generateCSV(results []*QueryResult) ([]byte, error) {
	// 简化实现
	return json.Marshal(results)
}

// CreateAlertRule 创建告警规则
func (m *Manager) CreateAlertRule(rule *AlertRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateAlertRuleID()
	}
	rule.CreatedAt = time.Now()

	m.alertRules[rule.ID] = rule
	m.logger.Info("告警规则创建成功: %s (%s)", rule.Name, rule.ID)
	return nil
}

// alertCheckLoop 告警检查循环
func (m *Manager) alertCheckLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkAlerts()
		}
	}
}

// checkAlerts 检查告警
func (m *Manager) checkAlerts() {
	m.mu.RLock()
	rules := make([]*AlertRule, 0, len(m.alertRules))
	for _, rule := range m.alertRules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	m.mu.RUnlock()

	for _, rule := range rules {
		m.evaluateAlertRule(rule)
	}
}

// evaluateAlertRule 评估告警规则
func (m *Manager) evaluateAlertRule(rule *AlertRule) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := m.ExecuteQuery(ctx, rule.Query)
	if err != nil {
		m.logger.Error("告警查询失败: %v", err)
		return
	}

	// 检查条件
	for _, series := range result.Series {
		for _, point := range series.Points {
			if val, ok := toFloat64(point.Value); ok {
				triggered := false
				switch rule.Condition {
				case "above":
					triggered = val > rule.Threshold
				case "below":
					triggered = val < rule.Threshold
				case "equal":
					triggered = val == rule.Threshold
				}

				if triggered {
					m.fireAlert(rule, val)
				}
			}
		}
	}
}

// fireAlert 触发告警
func (m *Manager) fireAlert(rule *AlertRule, value float64) {
	alert := &Alert{
		ID:        generateAlertID(),
		RuleID:    rule.ID,
		RuleName:  rule.Name,
		Severity:  rule.Severity,
		Message:   fmt.Sprintf("告警: %s, 当前值: %.2f, 阈值: %.2f", rule.Name, value, rule.Threshold),
		Value:     value,
		Threshold: rule.Threshold,
		Status:    "firing",
		StartedAt: time.Now(),
	}

	m.mu.Lock()
	m.alerts[rule.ID] = append(m.alerts[rule.ID], alert)
	now := time.Now()
	rule.LastTriggered = &now
	m.mu.Unlock()

	m.logger.Info("告警触发: %s", alert.Message)
}

// reportScheduleLoop 报表调度循环
func (m *Manager) reportScheduleLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkScheduledReports()
		}
	}
}

// checkScheduledReports 检查定时报表
func (m *Manager) checkScheduledReports() {
	// 简化实现
}

// GetAlerts 获取告警记录
func (m *Manager) GetAlerts(ruleID string) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alerts[ruleID]
}

// ListAlertRules 列出告警规则
func (m *Manager) ListAlertRules() []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*AlertRule, 0, len(m.alertRules))
	for _, rule := range m.alertRules {
		rules = append(rules, rule)
	}
	return rules
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

func generateDashboardID() string {
	return fmt.Sprintf("dashboard_%d", time.Now().UnixNano())
}

func generateReportID() string {
	return fmt.Sprintf("report_%d", time.Now().UnixNano())
}

func generateAlertRuleID() string {
	return fmt.Sprintf("alert_rule_%d", time.Now().UnixNano())
}

func generateAlertID() string {
	return fmt.Sprintf("alert_%d", time.Now().UnixNano())
}

// RegisterHandlers 注册 HTTP 处理器
func (m *Manager) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/dashboards", m.handleDashboards)
	mux.HandleFunc("/api/query", m.handleQuery)
	mux.HandleFunc("/api/reports", m.handleReports)
	mux.HandleFunc("/api/reports/generate", m.handleGenerateReport)
	mux.HandleFunc("/api/alerts/rules", m.handleAlertRules)
	mux.HandleFunc("/api/alerts", m.handleAlerts)
}

func (m *Manager) handleDashboards(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		dashboards := m.ListDashboards()
		writeJSON(w, dashboards)
	case http.MethodPost:
		var dashboard Dashboard
		if err := json.NewDecoder(r.Body).Decode(&dashboard); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateDashboard(&dashboard); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, dashboard)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var query Query
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := m.ExecuteQuery(r.Context(), query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, result)
}

func (m *Manager) handleReports(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		m.mu.RLock()
		reports := make([]*Report, 0, len(m.reports))
		for _, report := range m.reports {
			reports = append(reports, report)
		}
		m.mu.RUnlock()
		writeJSON(w, reports)
	case http.MethodPost:
		var report Report
		if err := json.NewDecoder(r.Body).Decode(&report); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateReport(&report); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, report)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ReportID string `json:"report_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	data, err := m.GenerateReport(r.Context(), req.ReportID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (m *Manager) handleAlertRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules := m.ListAlertRules()
		writeJSON(w, rules)
	case http.MethodPost:
		var rule AlertRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateAlertRule(&rule); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, rule)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ruleID := r.URL.Query().Get("rule_id")
	alerts := m.GetAlerts(ruleID)
	writeJSON(w, alerts)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
