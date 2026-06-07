package smartalerttriage

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 智能告警分类管理器.
// 负责告警的分类、去重、聚合、根因分析、升级、抑制和通知.
type Manager struct {
	alerts       map[string]*Alert                // id -> alert
	groups       map[string]*AlertGroup           // 聚合组
	rootCauses   map[string]*RootCause            // 根因索引
	suppressions map[string]*SuppressionRule      // 抑制规则
	policies     []*EscalationPolicy              // 升级策略
	knowledge    map[Category][]KnowledgeEntry    // 知识库
	notifiers    map[NotificationChannel]Notifier // 通知器
	logger       *zap.Logger
	mu           sync.RWMutex
}

// Notifier 通知器接口.
type Notifier interface {
	Send(alert *Alert, message string) error
	Channel() NotificationChannel
}

// NewManager 创建智能告警分类管理器.
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	m := &Manager{
		alerts:       make(map[string]*Alert),
		groups:       make(map[string]*AlertGroup),
		rootCauses:   make(map[string]*RootCause),
		suppressions: make(map[string]*SuppressionRule),
		policies:     make([]*EscalationPolicy, 0),
		knowledge:    make(map[Category][]KnowledgeEntry),
		notifiers:    make(map[NotificationChannel]Notifier),
		logger:       logger,
	}
	m.loadDefaultPolicies()
	m.loadBuiltinKnowledge()
	return m
}

// RegisterNotifier 注册通知器.
func (m *Manager) RegisterNotifier(n Notifier) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifiers[n.Channel()] = n
	m.logger.Info("notifier registered", zap.String("channel", string(n.Channel())))
}

// Ingest 接收并处理一条告警.
// 完整处理流程：分类 → 去重 → 抑制检查 → 关联分析 → 通知.
func (m *Manager) Ingest(title, description, source, resource string, labels map[string]string) *Alert {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成告警指纹
	fp := m.fingerprint(title, source, resource)

	// 去重检查：相同指纹的告警聚合
	for _, a := range m.alerts {
		if a.Fingerprint == fp && a.State != StateResolved {
			a.LastSeen = time.Now()
			a.Count++
			if description != "" {
				a.Description = description
			}
			if labels != nil {
				for k, v := range labels {
					a.Labels[k] = v
				}
			}
			// 更新聚合组
			if a.GroupID != "" {
				if group, ok := m.groups[a.GroupID]; ok {
					group.Count = a.Count
					group.LastSeen = time.Now()
				}
			}
			m.logger.Info("alert deduplicated",
				zap.String("id", a.ID),
				zap.String("fingerprint", fp),
				zap.Int("count", a.Count),
			)
			return a
		}
	}

	// 创建新告警
	alert := &Alert{
		ID:               uuid.New().String(),
		Title:            title,
		Description:      description,
		Priority:         PriorityMedium, // 默认中等，后续AI分类
		OriginalPriority: PriorityMedium,
		Category:         CategoryUnknown,
		State:            StateNew,
		Source:           source,
		Resource:         resource,
		Labels:           labels,
		Fingerprint:      fp,
		FirstSeen:        time.Now(),
		LastSeen:         time.Now(),
		Count:            1,
	}

	// Step 1: AI分类
	m.classify(alert)

	// Step 2: 抑制检查
	if m.isSuppressed(alert) {
		alert.State = StateSuppressed
		m.alerts[alert.ID] = alert
		m.logger.Info("alert suppressed",
			zap.String("id", alert.ID),
			zap.String("title", title),
		)
		return alert
	}

	// Step 3: 聚合
	m.aggregate(alert)

	// Step 4: 根因关联
	m.correlate(alert)

	// Step 5: 匹配知识库
	m.matchKnowledge(alert)

	alert.State = StateClassified
	m.alerts[alert.ID] = alert

	// Step 6: 发送通知
	go m.notify(alert)

	m.logger.Info("alert ingested",
		zap.String("id", alert.ID),
		zap.String("title", title),
		zap.String("priority", string(alert.Priority)),
		zap.String("category", string(alert.Category)),
	)

	return alert
}

// classify AI驱动的告警分类和优先级排序.
func (m *Manager) classify(alert *Alert) {
	title := strings.ToLower(alert.Title)
	desc := strings.ToLower(alert.Description)
	combined := title + " " + desc

	// 分类规则（基于关键词的简单AI分类）
	categoryRules := map[Category][]string{
		CategoryStorage:  {"disk", "磁盘", "存储", "raid", "zfs", "pool", "volume", "空间", "容量", "smart", "硬盘", "坏道"},
		CategoryNetwork:  {"网络", "network", "eth", "网卡", "连接", "timeout", "dns", "ping", "丢包", "延迟", "连接失败"},
		CategorySystem:   {"cpu", "内存", "memory", "负载", "load", "进程", "process", "系统", "温度", "temperature", "高负载"},
		CategorySecurity: {"安全", "security", "入侵", "攻击", "暴力破解", "brute", "登录失败", "异常访问", "恶意"},
		CategoryService:  {"服务", "service", "daemon", "进程停止", "宕机", "down", "failed", "nginx", "docker"},
		CategoryHardware: {"硬件", "hardware", "风扇", "电源", "ups", "pcie", "ecc", "内存错误"},
	}

	// 匹配分类
	bestCategory := CategoryUnknown
	bestScore := 0

	for cat, keywords := range categoryRules {
		score := 0
		for _, kw := range keywords {
			if strings.Contains(combined, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestCategory = cat
		}
	}
	alert.Category = bestCategory

	// 优先级排序（基于关键词严重性）
	priorityRules := map[Priority][]string{
		PriorityCritical: {"critical", "严重", "紧急", "宕机", "down", "故障", "failure", "数据丢失", "raid degraded", "异常", "smart", "检测", "暴力破解", "攻击", "入侵"},
		PriorityHigh:     {"high", "高", "警告", "warning", "阈值", "连接失败", "超时", "timeout", "unreachable"},
		PriorityMedium:   {"medium", "中", "注意", "notice", "负载过高", "使用率"},
		PriorityLow:      {"low", "低", "建议", "suggestion", "优化"},
	}

	bestPriority := PriorityMedium
	bestScore = 0

	for pri, keywords := range priorityRules {
		score := 0
		for _, kw := range keywords {
			if strings.Contains(combined, kw) {
				score++
			}
		}
		if score > bestScore {
			bestScore = score
			bestPriority = pri
		}
	}
	alert.Priority = bestPriority
	alert.OriginalPriority = bestPriority

	m.logger.Debug("alert classified",
		zap.String("id", alert.ID),
		zap.String("category", string(alert.Category)),
		zap.String("priority", string(alert.Priority)),
	)
}

// fingerprint 生成告警指纹（用于去重）.
func (m *Manager) fingerprint(title, source, resource string) string {
	data := fmt.Sprintf("%s:%s:%s", title, source, resource)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash[:8])
}

// aggregate 告警聚合（相似告警合并到同一组）.
func (m *Manager) aggregate(alert *Alert) {
	// 基于 category + source 查找已有聚合组
	for _, group := range m.groups {
		if group.Category == alert.Category && group.Source == alert.Source {
			alert.GroupID = group.ID
			group.AlertIDs = append(group.AlertIDs, alert.ID)
			group.Count = len(group.AlertIDs)
			group.LastSeen = time.Now()
			// 升级聚合组优先级
			if PriorityWeight[alert.Priority] > PriorityWeight[group.Priority] {
				group.Priority = alert.Priority
			}
			return
		}
	}

	// 创建新聚合组
	group := &AlertGroup{
		ID:          uuid.New().String(),
		Fingerprint: alert.Fingerprint,
		AlertIDs:    []string{alert.ID},
		Count:       1,
		FirstSeen:   time.Now(),
		LastSeen:    time.Now(),
		Priority:    alert.Priority,
		Category:    alert.Category,
		Source:      alert.Source,
		Title:       alert.Title,
	}
	alert.GroupID = group.ID
	m.groups[group.ID] = group
}

// correlate 告警关联分析（发现根因）.
func (m *Manager) correlate(alert *Alert) {
	// 查找相同资源的告警
	relatedIDs := make([]string, 0)
	for _, a := range m.alerts {
		if a.ID != alert.ID && a.Resource == alert.Resource && a.State != StateResolved {
			relatedIDs = append(relatedIDs, a.ID)
		}
	}

	if len(relatedIDs) > 0 {
		alert.RelatedIDs = relatedIDs

		// 查找或创建根因
		rootCauseKey := fmt.Sprintf("rc:%s:%s", alert.Resource, alert.Category)
		rc, ok := m.rootCauses[rootCauseKey]
		if !ok {
			rc = &RootCause{
				ID:              rootCauseKey,
				Description:     fmt.Sprintf("资源 %s 相关告警", alert.Resource),
				Category:        alert.Category,
				Confidence:      0.7,
				RelatedAlertIDs: make([]string, 0),
			}
			m.rootCauses[rootCauseKey] = rc
		}

		rc.RelatedAlertIDs = append(rc.RelatedAlertIDs, alert.ID)
		// 包含已关联的告警
		for _, rid := range relatedIDs {
			found := false
			for _, existing := range rc.RelatedAlertIDs {
				if existing == rid {
					found = true
					break
				}
			}
			if !found {
				rc.RelatedAlertIDs = append(rc.RelatedAlertIDs, rid)
			}
		}

		// 置信度随关联告警数增加
		if len(rc.RelatedAlertIDs) > 3 {
			rc.Confidence = 0.9
		} else if len(rc.RelatedAlertIDs) > 1 {
			rc.Confidence = 0.8
		}

		alert.RootCauseID = rc.ID
		alert.State = StateCorrelated

		m.logger.Info("alerts correlated",
			zap.String("alert_id", alert.ID),
			zap.String("root_cause", rc.ID),
			zap.Int("related_count", len(relatedIDs)),
		)
	}
}

// isSuppressed 检查告警是否应被抑制.
func (m *Manager) isSuppressed(alert *Alert) bool {
	now := time.Now()
	for _, rule := range m.suppressions {
		if !rule.Enabled {
			continue
		}
		if now.Before(rule.StartTime) || now.After(rule.EndTime) {
			continue
		}

		// 按分类抑制
		if rule.Category != "" && rule.Category == alert.Category {
			alert.SuppressedBy = rule.ID
			return true
		}

		// 按来源抑制
		if rule.Source != "" && rule.Source == alert.Source {
			alert.SuppressedBy = rule.ID
			return true
		}

		// 按模式匹配抑制
		if rule.Pattern != "" && strings.Contains(strings.ToLower(alert.Title), strings.ToLower(rule.Pattern)) {
			alert.SuppressedBy = rule.ID
			return true
		}
	}
	return false
}

// matchKnowledge 匹配知识库，附加推荐操作.
func (m *Manager) matchKnowledge(alert *Alert) {
	entries, ok := m.knowledge[alert.Category]
	if !ok {
		return
	}

	combined := strings.ToLower(alert.Title + " " + alert.Description)

	for _, entry := range entries {
		matched := false
		for _, kw := range entry.Keywords {
			if strings.Contains(combined, strings.ToLower(kw)) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		alert.RecommendedActions = entry.Actions
		if entry.RootCauseKey != "" && alert.RootCauseID == "" {
			alert.RootCauseID = entry.RootCauseKey
		}

		m.logger.Debug("knowledge matched",
			zap.String("alert_id", alert.ID),
			zap.String("entry_id", entry.ID),
		)
		return
	}
}

// notify 发送多通道通知.
func (m *Manager) notify(alert *Alert) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	message := fmt.Sprintf("[%s] %s\n%s\n来源: %s, 资源: %s",
		alert.Priority, alert.Title, alert.Description, alert.Source, alert.Resource)

	for _, n := range m.notifiers {
		if err := n.Send(alert, message); err != nil {
			m.logger.Error("notification failed",
				zap.String("channel", string(n.Channel())),
				zap.String("alert_id", alert.ID),
				zap.Error(err),
			)
		}
	}
}

// List 获取告警列表，支持筛选.
func (m *Manager) List(q ListQuery) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Alert
	for _, a := range m.alerts {
		if q.Category != "" && a.Category != q.Category {
			continue
		}
		if q.Priority != "" && a.Priority != q.Priority {
			continue
		}
		if q.State != "" && a.State != q.State {
			continue
		}
		if q.Source != "" && a.Source != q.Source {
			continue
		}
		result = append(result, a)
	}

	sort.Slice(result, func(i, j int) bool {
		wi := PriorityWeight[result[i].Priority]
		wj := PriorityWeight[result[j].Priority]
		if wi != wj {
			return wi > wj
		}
		return result[i].LastSeen.After(result[j].LastSeen)
	})

	return result
}

// Get 获取单个告警.
func (m *Manager) Get(id string) (*Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alert, ok := m.alerts[id]
	if !ok {
		return nil, fmt.Errorf("告警不存在: %s", id)
	}
	return alert, nil
}

// Acknowledge 确认告警.
func (m *Manager) Acknowledge(id, operator string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[id]
	if !ok {
		return fmt.Errorf("告警不存在: %s", id)
	}
	if alert.State == StateResolved {
		return fmt.Errorf("告警已解决: %s", id)
	}

	now := time.Now()
	alert.State = StateAcknowledged
	alert.AcknowledgedAt = &now
	alert.AcknowledgedBy = operator

	m.logger.Info("alert acknowledged",
		zap.String("id", id),
		zap.String("operator", operator),
	)
	return nil
}

// Resolve 解决告警.
func (m *Manager) Resolve(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, ok := m.alerts[id]
	if !ok {
		return fmt.Errorf("告警不存在: %s", id)
	}

	now := time.Now()
	alert.State = StateResolved
	alert.Priority = PriorityInfo
	alert.ResolvedAt = &now

	m.logger.Info("alert resolved", zap.String("id", id))
	return nil
}

// AddSuppression 添加抑制规则.
func (m *Manager) AddSuppression(req SuppressRequest) *SuppressionRule {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule := &SuppressionRule{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		Source:      req.Source,
		Pattern:     req.Pattern,
		StartTime:   time.Now(),
		EndTime:     time.Now().Add(time.Duration(req.DurationMin) * time.Minute),
		Reason:      req.Reason,
		CreatedBy:   req.CreatedBy,
		Enabled:     true,
	}

	m.suppressions[rule.ID] = rule
	m.logger.Info("suppression rule added",
		zap.String("id", rule.ID),
		zap.String("name", rule.Name),
		zap.String("reason", rule.Reason),
	)
	return rule
}

// ListSuppressions 列出所有抑制规则.
func (m *Manager) ListSuppressions() []*SuppressionRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SuppressionRule, 0, len(m.suppressions))
	for _, r := range m.suppressions {
		result = append(result, r)
	}
	return result
}

// RemoveSuppression 删除抑制规则.
func (m *Manager) RemoveSuppression(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.suppressions[id]; !ok {
		return fmt.Errorf("抑制规则不存在: %s", id)
	}
	delete(m.suppressions, id)
	return nil
}

// RunEscalation 执行告警升级检查（应定期调用）.
func (m *Manager) RunEscalation() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	escalated := 0
	now := time.Now()

	for _, a := range m.alerts {
		if a.State != StateNew && a.State != StateClassified && a.State != StateCorrelated && a.State != StateEscalated {
			continue
		}

		for _, policy := range m.policies {
			if !policy.Enabled {
				continue
			}
			if policy.Priority != "" && policy.Priority != a.Priority {
				continue
			}

			age := now.Sub(a.LastSeen)
			if age >= policy.UpgradeAfter {
				if PriorityWeight[a.Priority] < PriorityWeight[policy.MaxPriority] {
					a.Priority = escalatePriority(a.Priority)
					a.State = StateEscalated
					nowCopy := now
					a.EscalatedAt = &nowCopy
					escalated++

					m.logger.Warn("alert escalated",
						zap.String("id", a.ID),
						zap.String("new_priority", string(a.Priority)),
						zap.Duration("age", age),
					)
				}
				break
			}
		}
	}

	// 清理过期抑制规则
	for id, r := range m.suppressions {
		if now.After(r.EndTime) {
			delete(m.suppressions, id)
			m.logger.Info("suppression rule expired", zap.String("id", id))
		}
	}

	return escalated
}

// GetStats 获取告警统计.
func (m *Manager) GetStats(hours int) *AlertStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &AlertStats{
		ByCategory: make(map[Category]int),
		BySource:   make(map[string]int),
	}

	var totalResolution time.Duration
	resolvedCount := 0
	acknowledged := 0

	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)

	for _, a := range m.alerts {
		if a.FirstSeen.Before(cutoff) {
			continue
		}

		stats.TotalAlerts++
		if a.State != StateResolved {
			stats.ActiveAlerts++
		}

		switch a.Priority {
		case PriorityCritical:
			stats.CriticalCount++
		case PriorityHigh:
			stats.HighCount++
		case PriorityMedium:
			stats.MediumCount++
		case PriorityLow:
			stats.LowCount++
		case PriorityInfo:
			stats.InfoCount++
		}

		if a.State == StateSuppressed {
			stats.SuppressedCount++
		}

		stats.ByCategory[a.Category]++
		stats.BySource[a.Source]++

		if a.AcknowledgedAt != nil {
			acknowledged++
		}

		if a.ResolvedAt != nil {
			totalResolution += a.ResolvedAt.Sub(a.FirstSeen)
			resolvedCount++
		}
	}

	if stats.TotalAlerts > 0 {
		stats.AcknowledgedPct = float64(acknowledged) / float64(stats.TotalAlerts) * 100
	}
	if resolvedCount > 0 {
		stats.AvgResolution = totalResolution / time.Duration(resolvedCount)
	}

	return stats
}

// GetTrend 获取告警趋势.
func (m *Manager) GetTrend(hours, points int) []TrendPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if points <= 0 {
		points = 24
	}

	now := time.Now()
	start := now.Add(-time.Duration(hours) * time.Hour)
	interval := time.Duration(hours) * time.Duration(time.Hour) / time.Duration(points)

	result := make([]TrendPoint, points)
	for i := 0; i < points; i++ {
		t := start.Add(time.Duration(i) * interval)
		result[i] = TrendPoint{
			Timestamp: t,
		}
	}

	for _, a := range m.alerts {
		for i := 0; i < points; i++ {
			t := result[i].Timestamp
			next := t.Add(interval)
			if a.FirstSeen.After(t) && a.FirstSeen.Before(next) {
				result[i].Count++
				if a.Priority == PriorityCritical {
					result[i].CriticalCount++
				} else if a.Priority == PriorityHigh || a.Priority == PriorityMedium {
					result[i].WarningCount++
				}
			}
		}
	}

	return result
}

// GetRootCause 获取根因信息.
func (m *Manager) GetRootCause(id string) (*RootCause, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rc, ok := m.rootCauses[id]
	if !ok {
		return nil, fmt.Errorf("根因不存在: %s", id)
	}
	return rc, nil
}

// ListGroups 列出所有聚合组.
func (m *Manager) ListGroups() []*AlertGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AlertGroup, 0, len(m.groups))
	for _, g := range m.groups {
		result = append(result, g)
	}
	return result
}

// loadDefaultPolicies 加载默认升级策略.
func (m *Manager) loadDefaultPolicies() {
	m.policies = []*EscalationPolicy{
		{
			ID:           "policy-critical",
			Name:         "紧急告警升级",
			Priority:     PriorityHigh,
			UpgradeAfter: 15 * time.Minute,
			MaxPriority:  PriorityCritical,
			NotifyOnEsc:  true,
			Enabled:      true,
		},
		{
			ID:           "policy-medium",
			Name:         "中等告警升级",
			Priority:     PriorityMedium,
			UpgradeAfter: 30 * time.Minute,
			MaxPriority:  PriorityHigh,
			NotifyOnEsc:  true,
			Enabled:      true,
		},
		{
			ID:           "policy-low",
			Name:         "低优先级告警升级",
			Priority:     PriorityLow,
			UpgradeAfter: 60 * time.Minute,
			MaxPriority:  PriorityMedium,
			NotifyOnEsc:  false,
			Enabled:      true,
		},
	}
}

// loadBuiltinKnowledge 加载预置知识库.
func (m *Manager) loadBuiltinKnowledge() {
	entries := []KnowledgeEntry{
		{
			ID:       "kb-disk-smart",
			Keywords: []string{"smart", "磁盘健康", "硬盘故障", "disk failure", "bad sector", "坏道"},
			Title:    "磁盘SMART检测异常",
			Summary:  "磁盘SMART指标异常，可能即将发生物理故障",
			Category: CategoryStorage,
			Priority: PriorityCritical,
			Actions: []RecommendedAction{
				{ID: "act-1", Name: "查看SMART信息", Description: "获取磁盘详细SMART指标", Command: "smartctl -a /dev/sdX", RiskLevel: "low"},
				{ID: "act-2", Name: "运行磁盘自检", Description: "启动磁盘短时自检", Command: "smartctl -t short /dev/sdX", RiskLevel: "low"},
				{ID: "act-3", Name: "更换磁盘", Description: "标记磁盘待更换", Command: "zpool replace <pool> /dev/sdX /dev/sdY", Automated: false, RiskLevel: "high"},
			},
			References:   []string{"https://www.truenas.com/docs/"},
			RootCauseKey: "disk-hardware-failure",
		},
		{
			ID:       "kb-space-critical",
			Keywords: []string{"空间不足", "disk full", "存储空间", "容量不足", "space", "usage"},
			Title:    "存储空间不足",
			Summary:  "存储空间使用率过高，需要清理数据或扩容",
			Category: CategoryStorage,
			Priority: PriorityHigh,
			Actions: []RecommendedAction{
				{ID: "act-4", Name: "查看磁盘使用率", Description: "确认各分区空间占用", Command: "df -h", RiskLevel: "low"},
				{ID: "act-5", Name: "定位大文件", Description: "找出占用空间最多的目录", Command: "du -sh /* | sort -rh | head -20", RiskLevel: "low"},
				{ID: "act-6", Name: "清理日志", Description: "清理旧日志文件", Command: "journalctl --vacuum-time=7d", RiskLevel: "low"},
			},
			RootCauseKey: "storage-capacity",
		},
		{
			ID:       "kb-cpu-high",
			Keywords: []string{"cpu", "负载", "load average", "高负载"},
			Title:    "CPU负载异常",
			Summary:  "系统CPU使用率过高",
			Category: CategorySystem,
			Priority: PriorityHigh,
			Actions: []RecommendedAction{
				{ID: "act-7", Name: "查看CPU使用率", Description: "确认当前CPU状态", Command: "top -bn1 | head -20", RiskLevel: "low"},
				{ID: "act-8", Name: "定位高CPU进程", Description: "找出CPU占用最高的进程", Command: "ps aux --sort=-%cpu | head -15", RiskLevel: "low"},
			},
			RootCauseKey: "cpu-overload",
		},
		{
			ID:       "kb-network-down",
			Keywords: []string{"网络", "network", "unreachable", "timeout", "连接失败"},
			Title:    "网络连接异常",
			Summary:  "网络连接不稳定或无法到达目标",
			Category: CategoryNetwork,
			Priority: PriorityHigh,
			Actions: []RecommendedAction{
				{ID: "act-9", Name: "检查网卡状态", Description: "确认网络接口状态", Command: "ip addr show", RiskLevel: "low"},
				{ID: "act-10", Name: "测试网关", Description: "测试到网关的连通性", Command: "ping -c 3 $(ip route | grep default | awk '{print $3}')", RiskLevel: "low"},
			},
			RootCauseKey: "network-failure",
		},
		{
			ID:       "kb-security-brute",
			Keywords: []string{"暴力破解", "brute force", "登录失败", "入侵", "攻击"},
			Title:    "检测到暴力破解尝试",
			Summary:  "检测到来自异常IP的大量登录失败尝试",
			Category: CategorySecurity,
			Priority: PriorityCritical,
			Actions: []RecommendedAction{
				{ID: "act-11", Name: "查看失败日志", Description: "检查认证失败记录", Command: "journalctl -u sshd --since '-1h' | grep 'Failed'", RiskLevel: "low"},
				{ID: "act-12", Name: "封禁IP", Description: "将攻击IP加入黑名单", Command: "iptables -I INPUT -s <ip> -j DROP", Automated: false, RiskLevel: "medium"},
			},
			RootCauseKey: "security-attack",
		},
	}

	for _, entry := range entries {
		m.knowledge[entry.Category] = append(m.knowledge[entry.Category], entry)
	}
}

// escalatePriority 将优先级提升一级.
func escalatePriority(p Priority) Priority {
	switch p {
	case PriorityInfo:
		return PriorityLow
	case PriorityLow:
		return PriorityMedium
	case PriorityMedium:
		return PriorityHigh
	case PriorityHigh:
		return PriorityCritical
	default:
		return p
	}
}
