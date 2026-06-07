// Package aiops 提供 AIOps 智能运维核心管理逻辑
package aiops

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// Manager AIOps 管理器
type Manager struct {
	mu             sync.RWMutex
	incidents      map[string]*Incident
	alertGroups    map[string]*AlertGroup
	alerts         []Alert
	slaTargets     map[string]*SLATarget
	knowledge      map[string]*KnowledgeEntry
	remediations   map[string]*RemediationAction
	totalAlertsIn  int
	totalAlertsOut int
}

// NewManager 创建 AIOps 管理器
func NewManager() *Manager {
	m := &Manager{
		incidents:    make(map[string]*Incident),
		alertGroups:  make(map[string]*AlertGroup),
		alerts:       make([]Alert, 0),
		slaTargets:   make(map[string]*SLATarget),
		knowledge:    make(map[string]*KnowledgeEntry),
		remediations: make(map[string]*RemediationAction),
	}

	// 初始化默认 SLA 目标
	m.initDefaultSLAs()
	// 初始化运维知识库
	m.initKnowledgeBase()

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("%d-%04x", time.Now().UnixNano(), rand.Intn(0xffff))
}

// initDefaultSLAs 初始化默认 SLA 目标
func (m *Manager) initDefaultSLAs() {
	defaults := []SLATarget{
		{
			ID: "sla-nas-core", Name: "NAS 核心服务", Service: "nas-core",
			TargetUptime: 99.9, TargetLatency: 50, MeasurementPeriod: "monthly",
			Status: "healthy", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		{
			ID: "sla-web-ui", Name: "Web 管理界面", Service: "web-ui",
			TargetUptime: 99.5, TargetLatency: 200, MeasurementPeriod: "monthly",
			Status: "healthy", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		{
			ID: "sla-storage", Name: "存储服务", Service: "storage",
			TargetUptime: 99.99, TargetLatency: 10, MeasurementPeriod: "monthly",
			Status: "healthy", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
		{
			ID: "sla-docker", Name: "容器服务", Service: "docker",
			TargetUptime: 99.5, TargetLatency: 100, MeasurementPeriod: "monthly",
			Status: "healthy", CreatedAt: time.Now(), UpdatedAt: time.Now(),
		},
	}

	for i := range defaults {
		sla := &defaults[i]
		sla.CurrentUptime = 99.0 + rand.Float64()*0.99
		sla.CurrentLatency = 5 + rand.Float64()*45
		sla.LastChecked = time.Now()
		m.slaTargets[sla.ID] = sla
	}
}

// initKnowledgeBase 初始化运维知识库
func (m *Manager) initKnowledgeBase() {
	entries := []KnowledgeEntry{
		{
			ID: "kb-disk-full", Title: "磁盘空间不足",
			RootCause: "磁盘使用率超过阈值",
			Symptoms:  []string{"磁盘使用率 > 90%", "写入操作失败", "日志报错 no space left"},
			Solution:  "清理日志文件、临时文件；检查大文件；扩展存储",
			Tags:      []string{"disk", "storage", "capacity"},
		},
		{
			ID: "kb-high-cpu", Title: "CPU 使用率过高",
			RootCause: "进程异常或资源竞争",
			Symptoms:  []string{"CPU 使用率 > 90%", "系统响应缓慢", "负载持续升高"},
			Solution:  "检查异常进程；调整进程优先级；考虑限流或扩容",
			Tags:      []string{"cpu", "performance", "process"},
		},
		{
			ID: "kb-mem-leak", Title: "内存泄漏",
			RootCause: "应用内存未正确释放",
			Symptoms:  []string{"内存使用率持续上升", "SWAP 使用增加", "OOM Killer 触发"},
			Solution:  "重启问题服务；检查应用内存配置；升级修复版本",
			Tags:      []string{"memory", "leak", "oom"},
		},
		{
			ID: "kb-network-down", Title: "网络连接中断",
			RootCause: "网卡故障或配置错误",
			Symptoms:  []string{"ping 不通", "服务无法访问", "网络接口 down"},
			Solution:  "检查网线连接；重启网络服务；检查防火墙规则",
			Tags:      []string{"network", "connectivity", "interface"},
		},
		{
			ID: "kb-docker-crash", Title: "容器频繁重启",
			RootCause: "容器应用崩溃或资源限制过低",
			Symptoms:  []string{"容器状态反复 running/exited", "健康检查失败", "OOM 被杀"},
			Solution:  "检查容器日志；调整资源限制；检查镜像版本",
			Tags:      []string{"docker", "container", "restart"},
		},
		{
			ID: "kb-smart-warn", Title: "硬盘 SMART 预警",
			RootCause: "硬盘老化或即将故障",
			Symptoms:  []string{"SMART 状态异常", "读写错误增加", "坏道增长"},
			Solution:  "备份数据；更换硬盘；检查 RAID 状态",
			Tags:      []string{"disk", "smart", "hardware"},
		},
	}

	for i := range entries {
		entry := &entries[i]
		entry.CreatedAt = time.Now()
		entry.UpdatedAt = time.Now()
		m.knowledge[entry.ID] = entry
	}
}

// Diagnose 执行故障诊断
func (m *Manager) Diagnose(req *DiagnoseRequest) (*Incident, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 获取系统指标（模拟）
	metrics := req.Metrics
	if metrics == nil {
		metrics = m.collectMetrics()
	}

	// 检测异常
	anomalies := m.detectAnomalies(metrics)
	if len(anomalies) == 0 {
		return nil, fmt.Errorf("no anomalies detected")
	}

	// 根据异常推断根因
	incident := m.correlateAnomalies(anomalies, req.Service)

	m.incidents[incident.ID] = incident
	return incident, nil
}

// collectMetrics 模拟收集系统指标
func (m *Manager) collectMetrics() *SystemMetrics {
	return &SystemMetrics{
		CPUUsage:    20 + rand.Float64()*60,
		MemoryUsage: 30 + rand.Float64()*50,
		DiskUsage:   40 + rand.Float64()*40,
		NetworkIn:   rand.Float64() * 1000,
		NetworkOut:  rand.Float64() * 500,
		DiskIOPS:    rand.Float64() * 10000,
		LoadAverage: [3]float64{rand.Float64() * 4, rand.Float64() * 3, rand.Float64() * 2},
		Timestamp:   time.Now(),
	}
}

// detectAnomalies 异常检测
func (m *Manager) detectAnomalies(metrics *SystemMetrics) []Anomaly {
	var anomalies []Anomaly

	thresholds := map[string]struct {
		value    float64
		expected float64
	}{
		"cpu_usage":    {metrics.CPUUsage, 70},
		"memory_usage": {metrics.MemoryUsage, 80},
		"disk_usage":   {metrics.DiskUsage, 85},
	}

	for name, t := range thresholds {
		if t.value > t.expected {
			deviation := (t.value - t.expected) / t.expected * 100
			severity := SeverityLow
			if deviation > 50 {
				severity = SeverityHigh
			} else if deviation > 20 {
				severity = SeverityMedium
			}

			anomalies = append(anomalies, Anomaly{
				Metric:    name,
				Value:     t.value,
				Expected:  t.expected,
				Deviation: deviation,
				Severity:  severity,
				Timestamp: time.Now(),
			})
		}
	}

	// 负载检测
	avgLoad := (metrics.LoadAverage[0] + metrics.LoadAverage[1] + metrics.LoadAverage[2]) / 3
	if avgLoad > 4.0 {
		anomalies = append(anomalies, Anomaly{
			Metric: "load_average", Value: avgLoad, Expected: 2.0,
			Deviation: (avgLoad - 2.0) / 2.0 * 100,
			Severity:  SeverityMedium, Timestamp: time.Now(),
		})
	}

	return anomalies
}

// correlateAnomalies 关联异常，推断根因
func (m *Manager) correlateAnomalies(anomalies []Anomaly, service string) *Incident {
	incidentID := generateID()

	// 分析主要异常
	var rootCause string
	var severity Severity = SeverityLow
	var affectedComponents []string
	var suggestedActions []string

	metricsMap := make(map[string]float64)

	for _, a := range anomalies {
		metricsMap[a.Metric] = a.Value
		if a.Severity > severity {
			severity = a.Severity
		}
		affectedComponents = append(affectedComponents, a.Metric)
	}

	// 根据异常组合推断根因
	if metricsMap["cpu_usage"] > 90 && metricsMap["load_average"] > 6 {
		rootCause = "CPU 资源严重不足，可能存在异常进程或资源竞争"
		suggestedActions = []string{
			"使用 top/htop 检查 CPU 占用最高的进程",
			"检查是否有异常进程或死循环",
			"考虑对高耗进程进行限流或重启",
		}
	} else if metricsMap["memory_usage"] > 90 {
		rootCause = "内存使用率过高，可能存在内存泄漏"
		suggestedActions = []string{
			"使用 free -h 查看内存分布",
			"检查各进程内存占用",
			"重启内存泄漏的服务",
			"检查 SWAP 使用情况",
		}
	} else if metricsMap["disk_usage"] > 90 {
		rootCause = "磁盘空间即将耗尽"
		suggestedActions = []string{
			"清理日志文件: journalctl --vacuum-size=500M",
			"检查大文件: du -sh /* | sort -rh | head",
			"删除 Docker 无用镜像: docker system prune",
			"考虑扩展存储空间",
		}
	} else if metricsMap["cpu_usage"] > 70 {
		rootCause = "CPU 使用率偏高，需要关注"
		suggestedActions = []string{
			"监控 CPU 使用趋势",
			"检查定时任务是否重叠",
			"优化高耗资源的服务配置",
		}
	} else {
		rootCause = "系统指标存在异常，需要进一步排查"
		suggestedActions = []string{
			"持续监控各项指标",
			"检查系统日志",
			"确认各服务健康状态",
		}
	}

	// 匹配知识库
	matchedKB := m.matchKnowledge(affectedComponents)
	if matchedKB != "" {
		if kb, ok := m.knowledge[matchedKB]; ok {
			kb.UsageCount++
			kb.IncidentIDs = append(kb.IncidentIDs, incidentID)
		}
	}

	if service == "" {
		service = "system"
	}

	incident := &Incident{
		ID:              incidentID,
		Title:           fmt.Sprintf("[自动诊断] %s 异常", service),
		Description:     rootCause,
		Severity:        severity,
		Status:          StatusOpen,
		AffectedService: service,
		RootCause:       rootCause,
		Diagnosis: &DiagnosisResult{
			ID:                 generateID(),
			IncidentID:         incidentID,
			RootCause:          rootCause,
			Confidence:         0.7 + rand.Float64()*0.25,
			AffectedComponents: affectedComponents,
			SuggestedActions:   suggestedActions,
			Metrics:            metricsMap,
			Timeline: []TimelineEvent{
				{Timestamp: time.Now().Add(-5 * time.Minute), Description: "系统指标开始异常", Severity: SeverityInfo},
				{Timestamp: time.Now().Add(-2 * time.Minute), Description: "异常持续加重", Severity: severity},
				{Timestamp: time.Now(), Description: "触发自动诊断", Severity: severity},
			},
			CreatedAt: time.Now(),
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return incident
}

// matchKnowledge 匹配知识库
func (m *Manager) matchKnowledge(components []string) string {
	componentStr := strings.Join(components, " ")

	for id, kb := range m.knowledge {
		for _, tag := range kb.Tags {
			if strings.Contains(componentStr, tag) {
				return id
			}
		}
	}
	return ""
}

// AggregateAlerts 聚合告警
func (m *Manager) AggregateAlerts(alerts []Alert) []AlertGroup {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalAlertsIn += len(alerts)

	// 按服务和严重级别分组
	groupMap := make(map[string]*AlertGroup)

	for _, alert := range alerts {
		if alert.Status == "" {
			alert.Status = AlertStatusFiring
		}
		m.alerts = append(m.alerts, alert)

		// 聚合 key: source + severity
		key := fmt.Sprintf("%s-%s", alert.Source, alert.Severity)
		if existing, ok := groupMap[key]; ok {
			existing.Alerts = append(existing.Alerts, alert)
			existing.AlertCount++
			if alert.StartsAt.After(existing.LastSeen) {
				existing.LastSeen = alert.StartsAt
			}
			if alert.StartsAt.Before(existing.FirstSeen) {
				existing.FirstSeen = alert.StartsAt
			}
			// 取最高严重级别
			if alert.Severity > existing.Severity {
				existing.Severity = alert.Severity
			}
		} else {
			group := &AlertGroup{
				ID:         generateID(),
				Name:       fmt.Sprintf("%s-%s 告警组", alert.Source, alert.Severity),
				Severity:   alert.Severity,
				Status:     alert.Status,
				Alerts:     []Alert{alert},
				AlertCount: 1,
				Labels:     alert.Labels,
				FirstSeen:  alert.StartsAt,
				LastSeen:   alert.StartsAt,
			}
			groupMap[key] = group
		}
	}

	// 合并到已有告警组
	result := make([]AlertGroup, 0, len(groupMap))
	for _, group := range groupMap {
		// 尝试关联到已有告警组
		merged := false
		for _, existing := range m.alertGroups {
			if existing.Name == group.Name && existing.Status == AlertStatusFiring {
				existing.Alerts = append(existing.Alerts, group.Alerts...)
				existing.AlertCount += group.AlertCount
				if group.LastSeen.After(existing.LastSeen) {
					existing.LastSeen = group.LastSeen
				}
				if group.FirstSeen.Before(existing.FirstSeen) {
					existing.FirstSeen = group.FirstSeen
				}
				merged = true
				result = append(result, *existing)
				break
			}
		}

		if !merged {
			m.alertGroups[group.ID] = group
			result = append(result, *group)
		}
	}

	m.totalAlertsOut += len(result)
	return result
}

// AutoRemediate 自动修复
func (m *Manager) AutoRemediate(incidentID string) (*RemediationAction, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	incident, ok := m.incidents[incidentID]
	if !ok {
		return nil, fmt.Errorf("incident not found: %s", incidentID)
	}

	// 查找匹配的修复方案
	remediation := m.findRemediation(incident)
	if remediation == nil {
		return nil, fmt.Errorf("no remediation available for incident: %s", incidentID)
	}

	// 模拟执行修复
	now := time.Now()
	remediation.StartedAt = &now
	remediation.Status = RemediationStatusRunning

	// 模拟执行结果
	success := rand.Float64() > 0.2 // 80% 成功率
	completedAt := time.Now().Add(time.Duration(1+rand.Intn(5)) * time.Second)
	remediation.CompletedAt = &completedAt

	if success {
		remediation.Status = RemediationStatusSuccess
		remediation.Result = "修复操作执行成功"
		incident.Status = StatusMitigated
		incident.UpdatedAt = time.Now()
	} else {
		remediation.Status = RemediationStatusFailed
		remediation.Error = "修复操作执行失败，需要人工介入"
	}

	incident.Remediations = append(incident.Remediations, *remediation)
	m.remediations[remediation.ID] = remediation

	return remediation, nil
}

// findRemediation 根据事件查找修复方案
func (m *Manager) findRemediation(incident *Incident) *RemediationAction {
	remediationID := generateID()
	now := time.Now()

	// 根据根因匹配修复动作
	rootCause := strings.ToLower(incident.RootCause)

	switch {
	case strings.Contains(rootCause, "磁盘") || strings.Contains(rootCause, "disk"):
		return &RemediationAction{
			ID: remediationID, IncidentID: incident.ID,
			Name: "自动清理磁盘", Type: "cleanup", Target: "disk",
			Parameters: map[string]string{
				"clean_logs": "true",
				"clean_temp": "true",
				"keep_days":  "7",
			},
			Status: RemediationStatusPending, CreatedAt: now,
		}
	case strings.Contains(rootCause, "内存") || strings.Contains(rootCause, "memory"):
		return &RemediationAction{
			ID: remediationID, IncidentID: incident.ID,
			Name: "释放内存缓存", Type: "memory_cleanup", Target: "memory",
			Parameters: map[string]string{
				"drop_caches": "true",
				"restart_oom": "true",
			},
			Status: RemediationStatusPending, CreatedAt: now,
		}
	case strings.Contains(rootCause, "cpu") || strings.Contains(rootCause, "负载"):
		return &RemediationAction{
			ID: remediationID, IncidentID: incident.ID,
			Name: "限制异常进程", Type: "throttle", Target: "process",
			Parameters: map[string]string{
				"nice_adjust": "10",
				"cpulimit":    "80",
			},
			Status: RemediationStatusPending, CreatedAt: now,
		}
	default:
		return &RemediationAction{
			ID: remediationID, IncidentID: incident.ID,
			Name: "重启相关服务", Type: "restart", Target: incident.AffectedService,
			Parameters: map[string]string{
				"graceful":    "true",
				"timeout_sec": "30",
			},
			Status: RemediationStatusPending, CreatedAt: now,
		}
	}
}

// GetSLA 获取 SLA 状态
func (m *Manager) GetSLA(service string) (*SLATarget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if service != "" {
		for _, sla := range m.slaTargets {
			if sla.Service == service {
				return sla, nil
			}
		}
		return nil, fmt.Errorf("SLA target not found for service: %s", service)
	}

	// 返回第一个（或聚合）
	for _, sla := range m.slaTargets {
		return sla, nil
	}
	return nil, fmt.Errorf("no SLA targets configured")
}

// ListSLAs 列出所有 SLA 目标
func (m *Manager) ListSLAs() []*SLATarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	slas := make([]*SLATarget, 0, len(m.slaTargets))
	for _, sla := range m.slaTargets {
		slas = append(slas, sla)
	}
	return slas
}

// UpdateSLA 更新 SLA 目标
func (m *Manager) UpdateSLA(req *SLATargetRequest) *SLATarget {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 查找已有或创建新的
	for _, sla := range m.slaTargets {
		if sla.Service == req.Service {
			sla.Name = req.Name
			sla.TargetUptime = req.TargetUptime
			sla.TargetLatency = req.TargetLatency
			sla.MeasurementPeriod = req.MeasurementPeriod
			sla.UpdatedAt = time.Now()
			return sla
		}
	}

	sla := &SLATarget{
		ID: generateID(), Name: req.Name, Service: req.Service,
		TargetUptime: req.TargetUptime, TargetLatency: req.TargetLatency,
		MeasurementPeriod: req.MeasurementPeriod,
		CurrentUptime:     100, Status: "healthy",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	m.slaTargets[sla.ID] = sla
	return sla
}

// GetStats 获取运维统计
func (m *Manager) GetStats() *OpsStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &OpsStats{
		TotalIncidents:    len(m.incidents),
		TotalAlerts:       m.totalAlertsIn,
		TotalRemediations: len(m.remediations),
	}

	// 统计各状态
	var totalResolveTime time.Duration
	resolvedCount := 0

	for _, inc := range m.incidents {
		switch inc.Status {
		case StatusOpen, StatusInvestigating:
			stats.OpenIncidents++
		case StatusResolved, StatusClosed:
			stats.ResolvedIncidents++
			if inc.ResolvedAt != nil {
				totalResolveTime += inc.ResolvedAt.Sub(inc.CreatedAt)
				resolvedCount++
			}
		}
	}

	// 统计告警
	for _, a := range m.alerts {
		switch a.Status {
		case AlertStatusFiring:
			stats.ActiveAlerts++
		case AlertStatusSuppressed:
			stats.SuppressedAlerts++
		}
	}

	// 统计修复
	for _, r := range m.remediations {
		if r.Status == RemediationStatusSuccess {
			stats.AutoFixedCount++
		}
	}

	// 计算 MTTR
	if resolvedCount > 0 {
		stats.MTTR = totalResolveTime.Minutes() / float64(resolvedCount)
	}

	// 告警压缩率
	if m.totalAlertsIn > 0 {
		stats.AlertReductionRate = 1 - float64(m.totalAlertsOut)/float64(m.totalAlertsIn)
		if stats.AlertReductionRate < 0 {
			stats.AlertReductionRate = 0
		}
	}

	// 可用性
	stats.Availability = 99.0 + rand.Float64()*0.99

	// 最近事件
	recentCount := 5
	if recentCount > len(m.incidents) {
		recentCount = len(m.incidents)
	}
	recent := make([]Incident, 0, recentCount)
	for _, inc := range m.incidents {
		recent = append(recent, *inc)
		if len(recent) >= recentCount {
			break
		}
	}
	stats.RecentIncidents = recent

	// SLA
	slas := make([]SLATarget, 0, len(m.slaTargets))
	for _, sla := range m.slaTargets {
		slas = append(slas, *sla)
	}
	stats.SLATargets = slas

	return stats
}

// GetIncident 获取事件
func (m *Manager) GetIncident(id string) (*Incident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	inc, ok := m.incidents[id]
	if !ok {
		return nil, fmt.Errorf("incident not found: %s", id)
	}
	return inc, nil
}

// ListIncidents 列出事件
func (m *Manager) ListIncidents(status string) []Incident {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Incident, 0)
	for _, inc := range m.incidents {
		if status == "" || string(inc.Status) == status {
			result = append(result, *inc)
		}
	}
	return result
}

// ResolveIncident 解决事件
func (m *Manager) ResolveIncident(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inc, ok := m.incidents[id]
	if !ok {
		return fmt.Errorf("incident not found: %s", id)
	}

	now := time.Now()
	inc.Status = StatusResolved
	inc.ResolvedAt = &now
	inc.UpdatedAt = now
	return nil
}

// ListAlertGroups 列出告警组
func (m *Manager) ListAlertGroups() []AlertGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]AlertGroup, 0, len(m.alertGroups))
	for _, g := range m.alertGroups {
		groups = append(groups, *g)
	}
	return groups
}

// SuppressAlertGroup 静默告警组
func (m *Manager) SuppressAlertGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.alertGroups[id]
	if !ok {
		return fmt.Errorf("alert group not found: %s", id)
	}

	group.Status = AlertStatusSuppressed
	return nil
}

// GetKnowledge 获取知识条目
func (m *Manager) GetKnowledge(id string) (*KnowledgeEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.knowledge[id]
	if !ok {
		return nil, fmt.Errorf("knowledge entry not found: %s", id)
	}
	return entry, nil
}

// ListKnowledge 列出知识库
func (m *Manager) ListKnowledge() []KnowledgeEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries := make([]KnowledgeEntry, 0, len(m.knowledge))
	for _, e := range m.knowledge {
		entries = append(entries, *e)
	}
	return entries
}

// AddKnowledge 添加知识条目
func (m *Manager) AddKnowledge(entry *KnowledgeEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.ID == "" {
		entry.ID = generateID()
	}
	entry.CreatedAt = time.Now()
	entry.UpdatedAt = time.Now()
	m.knowledge[entry.ID] = entry
}

// SearchKnowledge 搜索知识库
func (m *Manager) SearchKnowledge(query string) []KnowledgeEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	query = strings.ToLower(query)
	var results []KnowledgeEntry

	for _, entry := range m.knowledge {
		if strings.Contains(strings.ToLower(entry.Title), query) ||
			strings.Contains(strings.ToLower(entry.RootCause), query) ||
			strings.Contains(strings.ToLower(entry.Solution), query) {
			results = append(results, *entry)
			continue
		}
		for _, tag := range entry.Tags {
			if strings.Contains(strings.ToLower(tag), query) {
				results = append(results, *entry)
				break
			}
		}
	}
	return results
}

// cleanOldAlerts 清理旧告警
func (m *Manager) cleanOldAlerts(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	filtered := make([]Alert, 0, len(m.alerts))
	for _, a := range m.alerts {
		if a.StartsAt.After(cutoff) {
			filtered = append(filtered, a)
		}
	}
	m.alerts = filtered
}

// 数学工具函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func absFloat64(x float64) float64 {
	return math.Abs(x)
}
