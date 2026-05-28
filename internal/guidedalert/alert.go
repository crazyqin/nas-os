// Package guidedalert 提供引导式告警系统
package guidedalert

import (
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Manager 引导式告警管理器
type Manager struct {
	alerts  map[string]*GuidedAlert
	rules   map[string]*AlertRule
	mu      sync.RWMutex
	counter int64
	logger  *slog.Logger
}

// NewManager 创建管理器
func NewManager(logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	m := &Manager{
		alerts: make(map[string]*GuidedAlert),
		rules:  make(map[string]*AlertRule),
		logger: logger,
	}
	m.registerBuiltinRules()
	return m
}

// RegisterRule 注册告警规则
func (m *Manager) RegisterRule(rule *AlertRule) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rules[rule.Name] = rule
	m.logger.Info("registered alert rule", "name", rule.Name)
}

// Fire 触发告警
func (m *Manager) Fire(ruleName, message string) *GuidedAlert {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[ruleName]
	if !exists {
		m.logger.Warn("unknown rule, creating basic alert", "rule", ruleName)
		m.counter++
		alert := &GuidedAlert{
			ID:        fmt.Sprintf("GA-%d", m.counter),
			Title:     ruleName,
			Message:   message,
			Severity:  SeverityWarning,
			Category:  CategoryStorage,
			CreatedAt: time.Now(),
		}
		m.alerts[alert.ID] = alert
		return alert
	}

	m.counter++
	alert := &GuidedAlert{
		ID:                   fmt.Sprintf("GA-%d", m.counter),
		Title:                rule.Name,
		Message:              message,
		Severity:             rule.Severity,
		Category:             rule.Category,
		CreatedAt:            time.Now(),
		TroubleshootingGuide: rule.TroubleshootingGuide,
	}
	m.alerts[alert.ID] = alert

	// 告警关联分析
	m.correlateAlerts(alert)

	m.logger.Info("alert fired",
		"id", alert.ID,
		"rule", ruleName,
		"severity", alert.Severity,
		"category", alert.Category,
	)
	return alert
}

// correlateAlerts 关联分析：同类别未确认告警合并
func (m *Manager) correlateAlerts(newAlert *GuidedAlert) {
	for _, existing := range m.alerts {
		if existing.ID == newAlert.ID {
			continue
		}
		if existing.Category == newAlert.Category && !existing.Acknowledged {
			newAlert.RelatedAlertIDs = append(newAlert.RelatedAlertIDs, existing.ID)
			existing.RelatedAlertIDs = append(existing.RelatedAlertIDs, newAlert.ID)
		}
	}
}

// Acknowledge 确认告警
func (m *Manager) Acknowledge(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	alert, ok := m.alerts[id]
	if !ok {
		return fmt.Errorf("alert %s not found", id)
	}
	alert.Acknowledged = true
	m.logger.Info("alert acknowledged", "id", id)
	return nil
}

// Silence 静音告警
func (m *Manager) Silence(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	alert, ok := m.alerts[id]
	if !ok {
		return fmt.Errorf("alert %s not found", id)
	}
	alert.Silenced = true
	m.logger.Info("alert silenced", "id", id)
	return nil
}

// Get 获取单个告警
func (m *Manager) Get(id string) (*GuidedAlert, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	alert, ok := m.alerts[id]
	return alert, ok
}

// List 获取告警列表
func (m *Manager) List() []*GuidedAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*GuidedAlert, 0, len(m.alerts))
	for _, alert := range m.alerts {
		result = append(result, alert)
	}
	return result
}

// Summary 获取告警汇总
func (m *Manager) Summary() *AlertSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	summary := &AlertSummary{
		ByCategory: make(map[Category]int),
		BySeverity: make(map[Severity]int),
	}
	for _, alert := range m.alerts {
		summary.Total++
		summary.ByCategory[alert.Category]++
		summary.BySeverity[alert.Severity]++
	}
	return summary
}

// GetRules 获取所有规则
func (m *Manager) GetRules() []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*AlertRule, 0, len(m.rules))
	for _, rule := range m.rules {
		result = append(result, rule)
	}
	return result
}

// registerBuiltinRules 注册内置告警规则
func (m *Manager) registerBuiltinRules() {
	m.rules["smart_warning"] = &AlertRule{
		Name:     "smart_warning",
		Condition: "disk SMART health check failed",
		Severity: SeverityWarning,
		Category: CategoryHardware,
		TroubleshootingGuide: &TroubleshootingGuide{
			Title:       "磁盘SMART健康告警排查",
			Description: "磁盘SMART检测到潜在问题，需要及时处理",
			Steps: []TroubleshootingStep{
				{Order: 1, Description: "查看磁盘SMART详情", Command: "smartctl -a /dev/sdX", ExpectedResult: "检查Reallocated_Sector_Ct等指标"},
				{Order: 2, Description: "运行磁盘短自检", Command: "smartctl -t short /dev/sdX", ExpectedResult: "自检应在2分钟内完成"},
				{Order: 3, Description: "检查系统日志", Command: "dmesg | grep sdX", ExpectedResult: "无I/O error记录"},
				{Order: 4, Description: "备份重要数据", ExpectedResult: "数据已安全备份"},
			},
			DocsURL: "https://docs.nas-os.com/guides/smart-monitoring",
		},
	}

	m.rules["pool_degraded"] = &AlertRule{
		Name:     "pool_degraded",
		Condition: "storage pool status is DEGRADED",
		Severity: SeverityCritical,
		Category: CategoryStorage,
		TroubleshootingGuide: &TroubleshootingGuide{
			Title:       "存储池降级排查",
			Description: "存储池处于降级状态，数据冗余受损",
			Steps: []TroubleshootingStep{
				{Order: 1, Description: "检查池状态", Command: "zpool status tank", ExpectedResult: "查看DEGRADED设备"},
				{Order: 2, Description: "替换故障设备", Command: "zpool replace tank old new", ExpectedResult: "池开始resilver"},
				{Order: 3, Description: "监控重建进度", Command: "zpool status tank", ExpectedResult: "resilver完成后状态ONLINE"},
			},
		},
	}

	m.rules["disk_space_low"] = &AlertRule{
		Name:     "disk_space_low",
		Condition: "disk usage exceeds 90%",
		Severity: SeverityWarning,
		Category: CategoryStorage,
		TroubleshootingGuide: &TroubleshootingGuide{
			Title:       "磁盘空间不足排查",
			Description: "存储空间即将耗尽，需要清理或扩容",
			Steps: []TroubleshootingStep{
				{Order: 1, Description: "查看空间使用", Command: "df -h", ExpectedResult: "定位高占用分区"},
				{Order: 2, Description: "查找大文件", Command: "du -sh /* | sort -rh | head", ExpectedResult: "找到占用空间最大的目录"},
				{Order: 3, Description: "清理临时文件", Command: "rm -rf /tmp/* /var/tmp/*", ExpectedResult: "释放临时空间"},
			},
		},
	}

	m.rules["network_down"] = &AlertRule{
		Name:     "network_down",
		Condition: "network interface is down",
		Severity: SeverityCritical,
		Category: CategoryNetwork,
		TroubleshootingGuide: &TroubleshootingGuide{
			Title:       "网络连接故障排查",
			Description: "网络接口断开，需要检查物理连接和配置",
			Steps: []TroubleshootingStep{
				{Order: 1, Description: "检查接口状态", Command: "ip link show", ExpectedResult: "查看接口是否UP"},
				{Order: 2, Description: "检查网线连接", ExpectedResult: "确认网线插好"},
				{Order: 3, Description: "重启网络服务", Command: "systemctl restart networking", ExpectedResult: "服务重启成功"},
			},
		},
	}

	m.rules["high_cpu"] = &AlertRule{
		Name:     "high_cpu",
		Condition: "CPU usage exceeds 90% for 5 minutes",
		Severity: SeverityWarning,
		Category: CategoryPerformance,
		TroubleshootingGuide: &TroubleshootingGuide{
			Title:       "CPU高负载排查",
			Description: "CPU持续高负载，需要排查占用进程",
			Steps: []TroubleshootingStep{
				{Order: 1, Description: "查看CPU占用", Command: "top -bn1 | head -20", ExpectedResult: "找到高占用进程"},
				{Order: 2, Description: "检查进程详情", Command: "ps aux --sort=-%cpu | head", ExpectedResult: "确认进程是否正常"},
			},
		},
	}

	m.rules["security_breach"] = &AlertRule{
		Name:     "security_breach",
		Condition: "suspicious login attempts detected",
		Severity: SeverityCritical,
		Category: CategorySecurity,
		TroubleshootingGuide: &TroubleshootingGuide{
			Title:       "安全告警排查",
			Description: "检测到可疑登录尝试，需要检查安全状态",
			Steps: []TroubleshootingStep{
				{Order: 1, Description: "查看登录日志", Command: "journalctl -u sshd | grep Failed", ExpectedResult: "分析失败登录来源"},
				{Order: 2, Description: "检查防火墙", Command: "iptables -L -n", ExpectedResult: "确认规则正确"},
				{Order: 3, Description: "封禁可疑IP", Command: "iptables -A INPUT -s x.x.x.x -j DROP", ExpectedResult: "阻止恶意访问"},
			},
		},
	}
}
