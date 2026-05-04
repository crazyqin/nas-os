package smartalert

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Engine 智能告警引擎.
// 负责告警的创建、知识库匹配、关联分析、升级和静默管理.
type Engine struct {
	alerts     map[string]*SmartAlert    // id -> alert
	knowledge  map[Category][]KnowledgeEntry // 知识库：按分类索引
	rootCauses map[string]*RootCause     // 根因索引
	silences   map[string]*SilenceRule   // 静默规则
	escPolicy  EscalationPolicy         // 升级策略
	logger     *zap.Logger
	mu         sync.RWMutex
}

// KnowledgeEntry 知识库条目.
type KnowledgeEntry struct {
	ID              string             `json:"id"`
	Keywords        []string           `json:"keywords"`         // 匹配关键词
	Title           string             `json:"title"`
	Summary         string             `json:"summary"`          // 根因概述
	Category        Category           `json:"category"`
	Severity        Severity           `json:"severity"`
	Steps           []TroubleshootStep `json:"steps"`            // 排查步骤
	FixCommands     []FixCommand       `json:"fix_commands"`     // 修复命令
	References      []string           `json:"references"`       // 参考链接
	RootCauseKey    string             `json:"root_cause_key"`   // 用于关联分析的根因标识
}

// NewEngine 创建智能告警引擎.
func NewEngine(logger *zap.Logger) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	e := &Engine{
		alerts:     make(map[string]*SmartAlert),
		knowledge:  make(map[Category][]KnowledgeEntry),
		rootCauses: make(map[string]*RootCause),
		silences:   make(map[string]*SilenceRule),
		escPolicy: EscalationPolicy{
			UpgradeAfter: 30 * time.Minute,
			MaxSeverity:  SeverityCritical,
		},
		logger: logger,
	}
	e.loadBuiltinKnowledge()
	return e
}

// Ingest 创建或更新一条告警.
// 会自动匹配知识库、附加排查步骤，并进行关联分析.
func (e *Engine) Ingest(title, description string, category Category, severity Severity, source, resource string, labels map[string]string) *SmartAlert {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 检查是否已存在相同告警（基于 title + resource 去重）
	for _, a := range e.alerts {
		if a.Title == title && a.Resource == resource && a.State != StateResolved {
			a.LastSeen = time.Now()
			a.Description = description
			if labels != nil {
				for k, v := range labels {
					a.Labels[k] = v
				}
			}
			e.logger.Info("alert updated", zap.String("id", a.ID), zap.String("title", title))
			return a
		}
	}

	alert := &SmartAlert{
		ID:               uuid.New().String(),
		Title:            title,
		Description:      description,
		Severity:         severity,
		OriginalSeverity: severity,
		Category:         category,
		State:            StateActive,
		Source:           source,
		Resource:         resource,
		Labels:           labels,
		FirstSeen:        time.Now(),
		LastSeen:         time.Now(),
	}

	// 匹配知识库
	e.matchKnowledge(alert)
	// 关联分析
	e.correlate(alert)

	e.alerts[alert.ID] = alert
	e.logger.Info("alert created",
		zap.String("id", alert.ID),
		zap.String("title", title),
		zap.String("severity", string(severity)),
		zap.String("category", string(category)),
	)
	return alert
}

// List 获取告警列表，支持筛选.
func (e *Engine) List(q ListQuery) []*SmartAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*SmartAlert
	for _, a := range e.alerts {
		if q.Category != "" && a.Category != q.Category {
			continue
		}
		if q.Severity != "" && a.Severity != q.Severity {
			continue
		}
		if q.State != "" && a.State != q.State {
			continue
		}
		result = append(result, a)
	}

	sort.Slice(result, func(i, j int) bool {
		wi := SeverityWeight[result[i].Severity]
		wj := SeverityWeight[result[j].Severity]
		if wi != wj {
			return wi > wj
		}
		return result[i].LastSeen.After(result[j].LastSeen)
	})

	return result
}

// GetGuide 获取告警的完整处置引导.
func (e *Engine) GetGuide(id string) (*Guide, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	alert, ok := e.alerts[id]
	if !ok {
		return nil, fmt.Errorf("alert %q not found", id)
	}

	guide := &Guide{
		Alert: alert,
		Summary: fmt.Sprintf("告警 [%s] %s: %s",
			alert.Severity, alert.Title, alert.Description),
	}

	// 附加关联信息
	if alert.RootCauseID != "" {
		rc, ok := e.rootCauses[alert.RootCauseID]
		if ok {
			guide.Correlation = &CorrelationInfo{
				RootCauseID:     rc.ID,
				Description:     rc.Description,
				RelatedAlertIDs: rc.RelatedAlertIDs,
			}
		}
	}

	return guide, nil
}

// Acknowledge 确认告警.
func (e *Engine) Acknowledge(id, operator string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	alert, ok := e.alerts[id]
	if !ok {
		return fmt.Errorf("alert %q not found", id)
	}
	if alert.State == StateResolved {
		return fmt.Errorf("alert %q already resolved", id)
	}

	now := time.Now()
	alert.State = StateAcknowledged
	alert.AcknowledgedAt = &now
	alert.AcknowledgedBy = operator

	e.logger.Info("alert acknowledged",
		zap.String("id", id),
		zap.String("operator", operator),
	)
	return nil
}

// AddSilence 添加静默规则.
func (e *Engine) AddSilence(req SilenceRequest) *SilenceRule {
	e.mu.Lock()
	defer e.mu.Unlock()

	rule := &SilenceRule{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
		AlertID:     req.AlertID,
		StartTime:   time.Now(),
		EndTime:     time.Now().Add(time.Duration(req.DurationMin) * time.Minute),
		CreatedBy:   req.CreatedBy,
		CreatedAt:   time.Now(),
		Enabled:     true,
	}

	e.silences[rule.ID] = rule
	e.logger.Info("silence rule added",
		zap.String("id", rule.ID),
		zap.String("name", rule.Name),
		zap.Time("end_time", rule.EndTime),
	)
	return rule
}

// ListSilences 列出所有静默规则.
func (e *Engine) ListSilences() []*SilenceRule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*SilenceRule, 0, len(e.silences))
	for _, r := range e.silences {
		result = append(result, r)
	}
	return result
}

// RemoveSilence 删除静默规则.
func (e *Engine) RemoveSilence(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.silences[id]; !ok {
		return fmt.Errorf("silence rule %q not found", id)
	}
	delete(e.silences, id)
	return nil
}

// RunEscalation 执行告警升级检查（应定期调用）.
// 对超过阈值时间未处理的活跃告警自动升级严重等级.
func (e *Engine) RunEscalation() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	escalated := 0
	now := time.Now()

	for _, a := range e.alerts {
		if a.State != StateActive && a.State != StateEscalated {
			continue
		}
		if a.Severity == e.escPolicy.MaxSeverity {
			continue
		}

		age := now.Sub(a.LastSeen)
		if age >= e.escPolicy.UpgradeAfter {
			a.Severity = escalateSeverity(a.Severity)
			a.State = StateEscalated
			nowCopy := now
			a.EscalatedAt = &nowCopy
			escalated++

			e.logger.Warn("alert escalated",
				zap.String("id", a.ID),
				zap.String("new_severity", string(a.Severity)),
				zap.Duration("age", age),
			)
		}
	}

	// 清理过期静默规则
	for id, r := range e.silences {
		if now.After(r.EndTime) {
			delete(e.silences, id)
			e.logger.Info("silence rule expired", zap.String("id", id))
		}
	}

	return escalated
}

// Resolve 手动解决告警.
func (e *Engine) Resolve(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	alert, ok := e.alerts[id]
	if !ok {
		return fmt.Errorf("alert %q not found", id)
	}

	now := time.Now()
	alert.State = StateResolved
	alert.Severity = SeverityResolved
	alert.ResolvedAt = &now

	e.logger.Info("alert resolved", zap.String("id", id))
	return nil
}

// matchKnowledge 为告警匹配知识库条目，附加排查步骤.
func (e *Engine) matchKnowledge(alert *SmartAlert) {
	entries, ok := e.knowledge[alert.Category]
	if !ok {
		return
	}

	title := strings.ToLower(alert.Title)
	desc := strings.ToLower(alert.Description)
	combined := title + " " + desc

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

		// 填充排查步骤和修复命令
		if len(alert.TroubleshootSteps) == 0 {
			alert.TroubleshootSteps = entry.Steps
		}
		if len(alert.FixCommands) == 0 {
			alert.FixCommands = entry.FixCommands
		}
		if len(alert.References) == 0 {
			alert.References = entry.References
		}
		if alert.RootCauseID == "" && entry.RootCauseKey != "" {
			alert.RootCauseID = entry.RootCauseKey
		}

		e.logger.Debug("knowledge matched",
			zap.String("alert_id", alert.ID),
			zap.String("entry_id", entry.ID),
		)
		return // 匹配第一个即可
	}
}

// correlate 进行告警关联分析.
// 多条告警可能有相同根因.
func (e *Engine) correlate(alert *SmartAlert) {
	if alert.RootCauseID == "" {
		return
	}

	rc, ok := e.rootCauses[alert.RootCauseID]
	if !ok {
		rc = &RootCause{
			ID:              alert.RootCauseID,
			Description:     fmt.Sprintf("根因: %s", alert.Title),
			Category:        alert.Category,
			RelatedAlertIDs: []string{},
		}
		e.rootCauses[rc.ID] = rc
	}

	rc.RelatedAlertIDs = append(rc.RelatedAlertIDs, alert.ID)

	if len(rc.RelatedAlertIDs) > 1 {
		e.logger.Info("correlated alerts detected",
			zap.String("root_cause", rc.ID),
			zap.Int("count", len(rc.RelatedAlertIDs)),
		)
	}
}

// loadBuiltinKnowledge 加载预置知识库.
func (e *Engine) loadBuiltinKnowledge() {
	entries := []KnowledgeEntry{
		// 磁盘故障
		{
			ID:       "disk-smart-failure",
			Keywords: []string{"smart", "磁盘健康", "硬盘故障", "disk failure", "bad sector", "坏道"},
			Title:    "磁盘SMART检测异常",
			Summary:  "磁盘SMART指标异常，可能即将发生物理故障，建议尽快备份数据并更换磁盘",
			Category: CategoryDisk,
			Severity: SeverityCritical,
			Steps: []TroubleshootStep{
				{Order: 1, Title: "查看SMART信息", Description: "获取磁盘详细SMART指标", Command: "smartctl -a /dev/sdX", Expected: "所有指标正常，无重分配扇区"},
				{Order: 2, Title: "检查健康状态", Description: "查看SMART健康评估", Command: "smartctl -H /dev/sdX", Expected: "PASSED"},
				{Order: 3, Title: "查看错误日志", Description: "检查磁盘错误日志", Command: "smartctl -l error /dev/sdX", Expected: "无错误记录"},
				{Order: 4, Title: "运行短时自检", Description: "启动磁盘短时自检", Command: "smartctl -t short /dev/sdX"},
			},
			FixCommands: []FixCommand{
				{ID: "smart-selftest", Name: "磁盘自检", Description: "运行磁盘短时自检", Command: "smartctl -t short /dev/sdX"},
				{ID: "disk-replace", Name: "更换磁盘", Description: "标记磁盘待更换，执行数据迁移", Command: "zpool replace <pool> /dev/sdX /dev/sdY", Destructive: true, RequiresConfirm: true},
			},
			References:   []string{"https://www.truenas.com/docs/scale/scaleuireference/storage/disks/disksscreens/", "https://wiki.archlinux.org/title/S.M.A.R.T."},
			RootCauseKey: "disk-hardware-failure",
		},
		// 空间不足
		{
			ID:       "space-critical",
			Keywords: []string{"空间不足", "disk full", "存储空间", "容量不足", "space", "usage", "使用率"},
			Title:    "存储空间不足",
			Summary:  "存储空间使用率过高，需要清理数据或扩容",
			Category: CategorySpace,
			Severity: SeverityWarning,
			Steps: []TroubleshootStep{
				{Order: 1, Title: "查看磁盘使用率", Description: "确认各分区空间占用", Command: "df -h", Expected: "使用率 < 80%"},
				{Order: 2, Title: "定位大文件目录", Description: "找出占用空间最多的目录", Command: "du -sh /* 2>/dev/null | sort -rh | head -20"},
				{Order: 3, Title: "检查日志文件", Description: "查看是否有过大的日志", Command: "find /var/log -type f -size +100M -exec ls -lh {} \\;"},
				{Order: 4, Title: "检查ZFS快照", Description: "查看存储池快照占用", Command: "zfs list -t snapshot -o name,used | sort -k2 -rh | head -20"},
			},
			FixCommands: []FixCommand{
				{ID: "cleanup-logs", Name: "清理日志", Description: "清理7天前的日志文件", Command: "find /var/log -type f -mtime +7 -name '*.gz' -delete && journalctl --vacuum-time=7d"},
				{ID: "cleanup-tmp", Name: "清理临时文件", Description: "清理临时目录", Command: "find /tmp -type f -mtime +3 -delete"},
				{ID: "prune-snapshots", Name: "清理快照", Description: "删除旧的ZFS快照", Command: "zfs destroy pool/auto@%7d", Destructive: true, RequiresConfirm: true},
			},
			References:   []string{"https://www.truenas.com/docs/scale/scaleuireference/storage/"},
			RootCauseKey: "storage-capacity",
		},
		// 性能异常 - CPU
		{
			ID:       "perf-high-cpu",
			Keywords: []string{"cpu", "负载", "load average", "高负载", "性能"},
			Title:    "CPU负载异常",
			Summary:  "系统CPU使用率过高，可能影响服务响应速度",
			Category: CategoryPerf,
			Severity: SeverityWarning,
			Steps: []TroubleshootStep{
				{Order: 1, Title: "查看CPU使用率", Description: "确认当前CPU和负载状态", Command: "top -bn1 | head -20", Expected: "load average < CPU核心数"},
				{Order: 2, Title: "定位高CPU进程", Description: "找出CPU占用最高的进程", Command: "ps aux --sort=-%cpu | head -15"},
				{Order: 3, Title: "检查IO等待", Description: "确认是否有IO瓶颈", Command: "iostat -x 1 3", Expected: "iowait < 10%"},
			},
			FixCommands: []FixCommand{
				{ID: "restart-heavy", Name: "重启高负载服务", Description: "重启CPU占用过高的服务", Command: "systemctl restart <service>"},
			},
			References:   []string{"https://www.truenas.com/docs/scale/scaleuireference/systemsettings/advanced/"},
			RootCauseKey: "cpu-overload",
		},
		// 性能异常 - 内存
		{
			ID:       "perf-memory",
			Keywords: []string{"内存", "oom", "memory", "swap", "交换分区"},
			Title:    "内存不足",
			Summary:  "系统内存使用率过高或发生OOM",
			Category: CategoryPerf,
			Severity: SeverityCritical,
			Steps: []TroubleshootStep{
				{Order: 1, Title: "查看内存使用", Description: "确认当前内存和swap状态", Command: "free -h", Expected: "可用内存 > 20%"},
				{Order: 2, Title: "定位内存大户", Description: "找出内存占用最高的进程", Command: "ps aux --sort=-%mem | head -15"},
				{Order: 3, Title: "检查OOM日志", Description: "查看内核OOM日志", Command: "dmesg | grep -i 'oom' | tail -20"},
			},
			FixCommands: []FixCommand{
				{ID: "restart-mem-heavy", Name: "重启内存大户", Description: "重启内存占用过高的服务", Command: "systemctl restart <service>"},
			},
			References:   []string{"https://www.truenas.com/docs/scale/scaleuireference/systemsettings/advanced/"},
			RootCauseKey: "memory-exhaustion",
		},
		// 网络问题
		{
			ID:       "network-unreachable",
			Keywords: []string{"网络", "network", "unreachable", "timeout", "连接失败", "dns"},
			Title:    "网络连接异常",
			Summary:  "网络连接不稳定或无法到达目标",
			Category: CategoryNetwork,
			Severity: SeverityWarning,
			Steps: []TroubleshootStep{
				{Order: 1, Title: "检查网卡状态", Description: "确认网络接口是否正常", Command: "ip addr show", Expected: "接口UP"},
				{Order: 2, Title: "测试网关", Description: "测试到网关的连通性", Command: "ping -c 3 $(ip route | grep default | awk '{print $3}')"},
				{Order: 3, Title: "检查DNS", Description: "确认DNS解析是否正常", Command: "nslookup google.com"},
				{Order: 4, Title: "检查防火墙", Description: "查看防火墙规则", Command: "iptables -L -n 2>/dev/null || nft list ruleset 2>/dev/null"},
			},
			FixCommands: []FixCommand{
				{ID: "restart-net", Name: "重启网络", Description: "重启网络管理服务", Command: "systemctl restart systemd-networkd || systemctl restart NetworkManager", RequiresConfirm: true},
			},
			References:   []string{"https://www.truenas.com/docs/scale/scaleuireference/network/"},
			RootCauseKey: "network-failure",
		},
		// 安全威胁
		{
			ID:       "security-bruteforce",
			Keywords: []string{"暴力破解", "brute force", "登录失败", "failed login", "入侵", "攻击", "ssh"},
			Title:    "检测到暴力破解尝试",
			Summary:  "检测到来自异常IP的大量登录失败尝试",
			Category: CategorySecurity,
			Severity: SeverityCritical,
			Steps: []TroubleshootStep{
				{Order: 1, Title: "查看失败日志", Description: "检查认证失败记录", Command: "journalctl -u sshd --since '-1h' | grep 'Failed' | tail -30"},
				{Order: 2, Title: "统计攻击IP", Description: "汇总攻击来源IP", Command: "journalctl -u sshd | grep 'Failed' | awk '{print $(NF-3)}' | sort | uniq -c | sort -rn | head -10"},
				{Order: 3, Title: "检查当前连接", Description: "查看当前SSH连接", Command: "ss -tnp | grep :22"},
				{Order: 4, Title: "检查授权密钥", Description: "确认authorized_keys未被篡改", Command: "cat ~/.ssh/authorized_keys"},
			},
			FixCommands: []FixCommand{
				{ID: "block-ip", Name: "封禁IP", Description: "将攻击IP加入防火墙黑名单", Command: "iptables -I INPUT -s <attacker_ip> -j DROP", RequiresConfirm: true},
				{ID: "fail2ban", Name: "启用fail2ban", Description: "启用fail2ban自动封禁", Command: "apt install -y fail2ban && systemctl enable --now fail2ban"},
			},
			References:   []string{"https://www.truenas.com/docs/scale/scaleuireference/credentials/"},
			RootCauseKey: "security-attack",
		},
		// 服务故障
		{
			ID:       "service-down",
			Keywords: []string{"服务停止", "service down", "service failed", "宕机", "停止运行"},
			Title:    "关键服务停止",
			Summary:  "系统关键服务停止运行",
			Category: CategoryService,
			Severity: SeverityCritical,
			Steps: []TroubleshootStep{
				{Order: 1, Title: "检查服务状态", Description: "确认服务运行状态", Command: "systemctl status <service>"},
				{Order: 2, Title: "查看错误日志", Description: "检查服务错误日志", Command: "journalctl -u <service> --since '-1h' --no-pager | tail -50"},
				{Order: 3, Title: "检查配置", Description: "验证服务配置", Command: "<service> -t 2>/dev/null || echo 'no config test available'"},
			},
			FixCommands: []FixCommand{
				{ID: "restart-svc", Name: "重启服务", Description: "尝试重启已停止的服务", Command: "systemctl restart <service>"},
			},
			References:   []string{"https://www.truenas.com/docs/scale/scaleuireference/systemsettings/services/"},
			RootCauseKey: "service-crash",
		},
	}

	for _, entry := range entries {
		e.knowledge[entry.Category] = append(e.knowledge[entry.Category], entry)
	}

	total := 0
	for _, v := range e.knowledge {
		total += len(v)
	}
	e.logger.Info("builtin knowledge loaded", zap.Int("entries", total))
}

// escalateSeverity 将严重等级提升一级.
func escalateSeverity(s Severity) Severity {
	switch s {
	case SeverityInfo:
		return SeverityWarning
	case SeverityWarning:
		return SeverityCritical
	default:
		return s
	}
}
