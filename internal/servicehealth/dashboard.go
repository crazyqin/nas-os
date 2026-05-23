// Package servicehealth - 服务仪表盘
// 提供服务拓扑图、状态概览、历史可用性统计、SLA 计算等功能
package servicehealth

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// ============================================================
// 仪表盘数据类型
// ============================================================

// ServiceTopologyNode 服务拓扑节点
type ServiceTopologyNode struct {
	Name        string   `json:"name"`        // 服务名称
	DisplayName string   `json:"display_name"` // 显示名称
	Status      ServiceStatus `json:"status"`  // 当前状态
	Score       float64  `json:"score"`       // 健康评分
	Level       int      `json:"level"`       // 拓扑层级 (0=入口, 1=中间件, 2=存储等)
	Tags        []string `json:"tags"`        // 标签
}

// ServiceTopologyEdge 服务拓扑边（依赖关系）
type ServiceTopologyEdge struct {
	Source string `json:"source"` // 依赖方
	Target string `json:"target"` // 被依赖方
	Type   string `json:"type"`   // 依赖类型: "http", "tcp", "rpc" 等
}

// ServiceTopology 服务拓扑图
type ServiceTopology struct {
	Nodes []ServiceTopologyNode `json:"nodes"` // 节点列表
	Edges []ServiceTopologyEdge `json:"edges"` // 边列表
}

// StatusOverview 状态概览
type StatusOverview struct {
	Total     int     `json:"total"`      // 总服务数
	Healthy   int     `json:"healthy"`    // 正常服务数
	Warning   int     `json:"warning"`    // 警告服务数
	Critical  int     `json:"critical"`   // 故障服务数
	Unknown   int     `json:"unknown"`    // 未知服务数
	AvgScore  float64 `json:"avg_score"`  // 平均健康评分
	UpdatedAt time.Time `json:"updated_at"` // 更新时间
}

// AvailabilityRecord 可用性记录
type AvailabilityRecord struct {
	Timestamp  time.Time     `json:"timestamp"`   // 记录时间
	Status     ServiceStatus `json:"status"`      // 状态
	Score      float64       `json:"score"`       // 评分
	Uptime     float64       `json:"uptime"`      // 可用性百分比
	ResponseTime time.Duration `json:"response_time"` // 响应时间
}

// ServiceAvailabilityStats 服务可用性统计
type ServiceAvailabilityStats struct {
	ServiceName       string                `json:"service_name"`       // 服务名称
	Period            string                `json:"period"`             // 统计周期
	TotalChecks       int                   `json:"total_checks"`       // 总检查次数
	SuccessfulChecks  int                   `json:"successful_checks"`  // 成功次数
	FailedChecks      int                   `json:"failed_checks"`      // 失败次数
	UptimePercent     float64               `json:"uptime_percent"`     // 可用性百分比
	AvgResponseTime   time.Duration         `json:"avg_response_time"`  // 平均响应时间
	MaxResponseTime   time.Duration         `json:"max_response_time"`  // 最大响应时间
	MinResponseTime   time.Duration         `json:"min_response_time"`  // 最小响应时间
	LastDowntime      *time.Time            `json:"last_downtime"`      // 最后一次故障时间
	TotalDowntime     time.Duration         `json:"total_downtime"`     // 总故障时间
	Records           []AvailabilityRecord  `json:"records"`            // 详细记录
	UpdatedAt         time.Time             `json:"updated_at"`         // 更新时间
}

// DashboardSummary 仪表盘汇总
type DashboardSummary struct {
	Overview     StatusOverview                  `json:"overview"`      // 状态概览
	Topology     ServiceTopology                 `json:"topology"`      // 服务拓扑
	Services     []*ServiceHealth                `json:"services"`      // 服务列表
	Availability map[string]*ServiceAvailabilityStats `json:"availability"` // 可用性统计
	Alerts       []DashboardAlert                `json:"alerts"`        // 告警列表
	UpdatedAt    time.Time                       `json:"updated_at"`    // 更新时间
}

// DashboardAlert 仪表盘告警
type DashboardAlert struct {
	ServiceName string        `json:"service_name"` // 服务名称
	Level       ServiceStatus `json:"level"`        // 告警级别
	Message     string        `json:"message"`      // 告警消息
	Timestamp   time.Time     `json:"timestamp"`    // 告警时间
	Score       float64       `json:"score"`        // 当前评分
}

// ============================================================
// 服务仪表盘
// ============================================================

// ServiceDashboard 服务仪表盘
type ServiceDashboard struct {
	mu sync.RWMutex

	// 依赖
	manager *ServiceHealthManager

	// 拓扑数据
	topology ServiceTopology

	// 可用性历史
	availabilityHistory map[string][]AvailabilityRecord // 服务名 -> 历史记录
	maxHistoryLen       int                            // 最大历史记录数
}

// NewServiceDashboard 创建服务仪表盘
func NewServiceDashboard(manager *ServiceHealthManager) *ServiceDashboard {
	return &ServiceDashboard{
		manager:             manager,
		topology:            ServiceTopology{},
		availabilityHistory: make(map[string][]AvailabilityRecord),
		maxHistoryLen:       2880, // 24小时，每分钟一条记录
	}
}

// ============================================================
// 拓扑管理
// ============================================================

// UpdateTopology 更新服务拓扑
func (d *ServiceDashboard) UpdateTopology(topology ServiceTopology) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.topology = topology
}

// GetTopology 获取服务拓扑
func (d *ServiceDashboard) GetTopology() ServiceTopology {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// 更新节点状态
	services := d.manager.ListServices()
	statusMap := make(map[string]*ServiceHealth)
	for _, s := range services {
		statusMap[s.Config.Name] = s
	}

	result := d.topology
	for i, node := range result.Nodes {
		if s, exists := statusMap[node.Name]; exists {
			result.Nodes[i].Status = s.Status
			result.Nodes[i].Score = s.Score
		}
	}

	return result
}

// AddTopologyNode 添加拓扑节点
func (d *ServiceDashboard) AddTopologyNode(node ServiceTopologyNode) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 检查是否已存在
	for i, n := range d.topology.Nodes {
		if n.Name == node.Name {
			d.topology.Nodes[i] = node
			return
		}
	}

	d.topology.Nodes = append(d.topology.Nodes, node)
}

// AddTopologyEdge 添加拓扑边（依赖关系）
func (d *ServiceDashboard) AddTopologyEdge(edge ServiceTopologyEdge) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 检查是否已存在
	for _, e := range d.topology.Edges {
		if e.Source == edge.Source && e.Target == edge.Target {
			return
		}
	}

	d.topology.Edges = append(d.topology.Edges, edge)
}

// RemoveTopologyNode 移除拓扑节点
func (d *ServiceDashboard) RemoveTopologyNode(name string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 移除节点
	for i, n := range d.topology.Nodes {
		if n.Name == name {
			d.topology.Nodes = append(d.topology.Nodes[:i], d.topology.Nodes[i+1:]...)
			break
		}
	}

	// 移除相关边
	edges := make([]ServiceTopologyEdge, 0)
	for _, e := range d.topology.Edges {
		if e.Source != name && e.Target != name {
			edges = append(edges, e)
		}
	}
	d.topology.Edges = edges
}

// ============================================================
// 状态概览
// ============================================================

// GetStatusOverview 获取状态概览
func (d *ServiceDashboard) GetStatusOverview() StatusOverview {
	services := d.manager.ListServices()

	overview := StatusOverview{
		UpdatedAt: time.Now(),
	}

	if len(services) == 0 {
		return overview
	}

	var totalScore float64
	for _, s := range services {
		overview.Total++
		totalScore += s.Score

		switch s.Status {
		case StatusHealthy:
			overview.Healthy++
		case StatusWarning:
			overview.Warning++
		case StatusCritical:
			overview.Critical++
		case StatusUnknown:
			overview.Unknown++
		}
	}

	overview.AvgScore = totalScore / float64(len(services))
	overview.AvgScore = float64(int(overview.AvgScore*100)) / 100 // 保留两位小数

	return overview
}

// GetStatusOverviewByTag 按标签获取状态概览
func (d *ServiceDashboard) GetStatusOverviewByTag(tagKey, tagValue string) StatusOverview {
	services := d.manager.ListServices()

	overview := StatusOverview{
		UpdatedAt: time.Now(),
	}

	var totalScore float64
	for _, s := range services {
		// 检查标签匹配
		if tagValue != "" {
			if v, ok := s.Config.Tags[tagKey]; !ok || v != tagValue {
				continue
			}
		} else {
			if _, ok := s.Config.Tags[tagKey]; !ok {
				continue
			}
		}

		overview.Total++
		totalScore += s.Score

		switch s.Status {
		case StatusHealthy:
			overview.Healthy++
		case StatusWarning:
			overview.Warning++
		case StatusCritical:
			overview.Critical++
		case StatusUnknown:
			overview.Unknown++
		}
	}

	if overview.Total > 0 {
		overview.AvgScore = totalScore / float64(overview.Total)
		overview.AvgScore = float64(int(overview.AvgScore*100)) / 100
	}

	return overview
}

// ============================================================
// 历史可用性统计
// ============================================================

// RecordAvailability 记录可用性数据
func (d *ServiceDashboard) RecordAvailability(name string, record AvailabilityRecord) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.availabilityHistory[name] = append(d.availabilityHistory[name], record)

	// 裁剪历史
	if len(d.availabilityHistory[name]) > d.maxHistoryLen {
		d.availabilityHistory[name] = d.availabilityHistory[name][len(d.availabilityHistory[name])-d.maxHistoryLen:]
	}
}

// GetAvailabilityStats 获取服务可用性统计
func (d *ServiceDashboard) GetAvailabilityStats(name string, period time.Duration) (*ServiceAvailabilityStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	records, exists := d.availabilityHistory[name]
	if !exists {
		return nil, fmt.Errorf("服务 %s 无可用性记录", name)
	}

	// 过滤指定时间段的记录
	cutoff := time.Now().Add(-period)
	filtered := make([]AvailabilityRecord, 0)
	for _, r := range records {
		if r.Timestamp.After(cutoff) {
			filtered = append(filtered, r)
		}
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("服务 %s 在指定时间段内无记录", name)
	}

	stats := &ServiceAvailabilityStats{
		ServiceName: name,
		Period:      formatDuration(period),
		TotalChecks: len(filtered),
		UpdatedAt:   time.Now(),
	}

	var totalResponseTime time.Duration
	var totalDowntime time.Duration
	var lastDowntime *time.Time
	successCount := 0

	for i, r := range filtered {
		if r.Status == StatusHealthy || r.Status == StatusWarning {
			successCount++
		} else if r.Status == StatusCritical {
			stats.FailedChecks++
			lt := r.Timestamp
			lastDowntime = &lt

			// 计算故障时长（假设每次故障持续到下次检查）
			if i+1 < len(filtered) {
				totalDowntime += filtered[i+1].Timestamp.Sub(r.Timestamp)
			}
		}

		totalResponseTime += r.ResponseTime

		if i == 0 || r.ResponseTime > stats.MaxResponseTime {
			stats.MaxResponseTime = r.ResponseTime
		}
		if i == 0 || r.ResponseTime < stats.MinResponseTime {
			stats.MinResponseTime = r.ResponseTime
		}
	}

	stats.SuccessfulChecks = successCount
	if stats.TotalChecks > 0 {
		stats.UptimePercent = float64(successCount) / float64(stats.TotalChecks) * 100
		stats.UptimePercent = float64(int(stats.UptimePercent*100)) / 100
	}
	if len(filtered) > 0 {
		stats.AvgResponseTime = totalResponseTime / time.Duration(len(filtered))
	}
	stats.LastDowntime = lastDowntime
	stats.TotalDowntime = totalDowntime
	stats.Records = filtered

	return stats, nil
}

// GetAllAvailabilityStats 获取所有服务的可用性统计
func (d *ServiceDashboard) GetAllAvailabilityStats(period time.Duration) map[string]*ServiceAvailabilityStats {
	d.mu.RLock()
	names := make([]string, 0, len(d.availabilityHistory))
	for name := range d.availabilityHistory {
		names = append(names, name)
	}
	d.mu.RUnlock()

	result := make(map[string]*ServiceAvailabilityStats)
	for _, name := range names {
		stats, err := d.GetAvailabilityStats(name, period)
		if err == nil {
			result[name] = stats
		}
	}
	return result
}

// ============================================================
// 仪表盘汇总
// ============================================================

// GetDashboardSummary 获取仪表盘汇总数据
func (d *ServiceDashboard) GetDashboardSummary(period time.Duration) DashboardSummary {
	summary := DashboardSummary{
		Overview:     d.GetStatusOverview(),
		Topology:     d.GetTopology(),
		Services:     d.manager.ListServices(),
		Availability: d.GetAllAvailabilityStats(period),
		Alerts:       d.GetActiveAlerts(),
		UpdatedAt:    time.Now(),
	}

	return summary
}

// GetActiveAlerts 获取活跃告警
func (d *ServiceDashboard) GetActiveAlerts() []DashboardAlert {
	services := d.manager.ListServices()
	alerts := make([]DashboardAlert, 0)

	for _, s := range services {
		if s.Status == StatusCritical || s.Status == StatusWarning {
			alert := DashboardAlert{
				ServiceName: s.Config.Name,
				Level:       s.Status,
				Score:       s.Score,
				Timestamp:   s.UpdatedAt,
			}

			switch s.Status {
			case StatusCritical:
				alert.Message = fmt.Sprintf("服务 %s 故障，健康评分 %.1f", s.Config.DisplayName, s.Score)
			case StatusWarning:
				alert.Message = fmt.Sprintf("服务 %s 异常，健康评分 %.1f", s.Config.DisplayName, s.Score)
			}

			if s.LastCheck != nil {
				alert.Message += fmt.Sprintf("，最后检查: %s", s.LastCheck.Message)
			}

			alerts = append(alerts, alert)
		}
	}

	// 按评分排序（低分优先）
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].Score < alerts[j].Score
	})

	return alerts
}

// GetServiceRanking 获取服务健康排名
func (d *ServiceDashboard) GetServiceRanking(limit int) []*ServiceHealth {
	services := d.manager.ListServices()

	// 按评分排序
	sort.Slice(services, func(i, j int) bool {
		return services[i].Score > services[j].Score
	})

	if limit > 0 && limit < len(services) {
		services = services[:limit]
	}

	return services
}

// ============================================================
// SLA 计算（99.9% 可用性目标）
// ============================================================

// SLATarget SLA 目标配置
type SLATarget struct {
	Name           string  `json:"name"`            // SLA 名称
	TargetUptime   float64 `json:"target_uptime"`   // 目标可用性百分比，如 99.9
	PeriodDays     int     `json:"period_days"`     // 统计周期天数
	MaxDowntimeMin float64 `json:"max_downtime_min"` // 最大允许故障时间（分钟）
}

// SLAResult SLA 计算结果
type SLAResult struct {
	ServiceName    string    `json:"service_name"`     // 服务名称
	SLATarget      SLATarget `json:"sla_target"`       // SLA 目标
	ActualUptime   float64   `json:"actual_uptime"`    // 实际可用性
	TotalChecks    int       `json:"total_checks"`     // 总检查次数
	FailedChecks   int       `json:"failed_checks"`    // 失败次数
	TotalDowntime  time.Duration `json:"total_downtime"` // 总故障时间
	MaxDowntime    time.Duration `json:"max_downtime"`   // 最长单次故障
	IsCompliant    bool      `json:"is_compliant"`     // 是否达标
	GapPercent     float64   `json:"gap_percent"`      // 与目标的差距
	ReportTime     time.Time `json:"report_time"`      // 报告时间
}

// DefaultSLATargets 默认 SLA 目标
func DefaultSLATargets() []SLATarget {
	return []SLATarget{
		{Name: "critical", TargetUptime: 99.99, PeriodDays: 30, MaxDowntimeMin: 4.32},
		{Name: "high", TargetUptime: 99.9, PeriodDays: 30, MaxDowntimeMin: 43.2},
		{Name: "standard", TargetUptime: 99.5, PeriodDays: 30, MaxDowntimeMin: 216},
		{Name: "basic", TargetUptime: 99.0, PeriodDays: 30, MaxDowntimeMin: 432},
	}
}

// CalculateSLA 计算服务 SLA
func (d *ServiceDashboard) CalculateSLA(name string, target SLATarget) (*SLAResult, error) {
	period := time.Duration(target.PeriodDays) * 24 * time.Hour
	stats, err := d.GetAvailabilityStats(name, period)
	if err != nil {
		return nil, fmt.Errorf("获取可用性统计失败: %w", err)
	}

	result := &SLAResult{
		ServiceName:   name,
		SLATarget:     target,
		ActualUptime:  stats.UptimePercent,
		TotalChecks:   stats.TotalChecks,
		FailedChecks:  stats.FailedChecks,
		TotalDowntime: stats.TotalDowntime,
		ReportTime:    time.Now(),
	}

	// 计算最长单次故障
	if len(stats.Records) > 0 {
		currentDowntime := time.Duration(0)
		maxDowntime := time.Duration(0)

		for _, r := range stats.Records {
			if r.Status == StatusCritical {
				currentDowntime += time.Minute // 假设检查间隔约1分钟
			} else {
				if currentDowntime > maxDowntime {
					maxDowntime = currentDowntime
				}
				currentDowntime = 0
			}
		}
		if currentDowntime > maxDowntime {
			maxDowntime = currentDowntime
		}
		result.MaxDowntime = maxDowntime
	}

	// 判断是否达标
	result.IsCompliant = stats.UptimePercent >= target.TargetUptime
	result.GapPercent = stats.UptimePercent - target.TargetUptime

	return result, nil
}

// CalculateAllSLA 计算所有服务的 SLA
func (d *ServiceDashboard) CalculateAllSLA(target SLATarget) map[string]*SLAResult {
	services := d.manager.ListServices()
	results := make(map[string]*SLAResult)

	for _, s := range services {
		result, err := d.CalculateSLA(s.Config.Name, target)
		if err == nil {
			results[s.Config.Name] = result
		}
	}

	return results
}

// ============================================================
// 辅助函数
// ============================================================

// formatDuration 格式化时间段
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}
