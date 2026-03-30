package security

import (
	"sync"
	"time"
)

// Manager 安全管理器（统一入口）.
type Manager struct {
	firewall   *FirewallManager
	fail2ban   *Fail2BanManager
	audit      *AuditManager
	baseline   *BaselineManager
	config     Config
	notifyFunc func(alert Alert)
	mu         sync.RWMutex
}

// NewManager 创建安全管理器.
func NewManager() *Manager {
	sm := &Manager{
		firewall: NewFirewallManager(),
		fail2ban: NewFail2BanManager(),
		audit:    NewAuditManager(),
		baseline: NewBaselineManager(),
		config: Config{
			Firewall: FirewallConfig{
				Enabled:       true,
				DefaultPolicy: "deny",
				IPv6Enabled:   true,
				LogDropped:    true,
			},
			Fail2Ban: Fail2BanConfig{
				Enabled:            true,
				MaxAttempts:        5,
				WindowMinutes:      10,
				BanDurationMinutes: 60,
				AutoUnban:          true,
				NotifyOnBan:        true,
			},
			AuditEnabled: true,
			AlertEnabled: true,
		},
	}

	// 设置通知回调
	sm.fail2ban.SetNotifyFunc(sm.handleAlert)

	// 启动清理例程
	sm.firewall.StartCleanupRoutine(time.Minute * 5)
	sm.fail2ban.StartCleanupRoutine(time.Minute * 5)
	sm.audit.StartCleanupRoutine(time.Hour * 24)

	return sm
}

// handleAlert 处理安全告警.
func (sm *Manager) handleAlert(alert Alert) {
	sm.mu.RLock()
	notifyFunc := sm.notifyFunc
	sm.mu.RUnlock()

	// 添加到审计日志
	sm.audit.mu.Lock()
	sm.audit.alerts = append(sm.audit.alerts, &alert)
	sm.audit.mu.Unlock()

	// 调用外部通知函数
	if notifyFunc != nil {
		go notifyFunc(alert)
	}
}

// SetNotifyFunc 设置通知回调.
func (sm *Manager) SetNotifyFunc(notifyFn func(alert Alert)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.notifyFunc = notifyFn
}

// GetFirewallManager 获取防火墙管理器.
func (sm *Manager) GetFirewallManager() *FirewallManager {
	return sm.firewall
}

// GetFail2BanManager 获取失败登录保护管理器.
func (sm *Manager) GetFail2BanManager() *Fail2BanManager {
	return sm.fail2ban
}

// GetAuditManager 获取审计管理器.
func (sm *Manager) GetAuditManager() *AuditManager {
	return sm.audit
}

// GetBaselineManager 获取基线检查管理器.
func (sm *Manager) GetBaselineManager() *BaselineManager {
	return sm.baseline
}

// GetConfig 获取安全配置.
func (sm *Manager) GetConfig() Config {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.config
}

// UpdateConfig 更新安全配置.
func (sm *Manager) UpdateConfig(config Config) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.config = config

	// 应用防火墙配置
	if err := sm.firewall.UpdateConfig(config.Firewall); err != nil {
		return err
	}

	// 应用 fail2ban 配置
	if err := sm.fail2ban.UpdateConfig(config.Fail2Ban); err != nil {
		return err
	}

	// 应用审计配置
	sm.audit.SetConfig(AuditConfig{
		Enabled:      config.AuditEnabled,
		AlertEnabled: config.AlertEnabled,
	})

	return nil
}

// RecordFailedLogin 记录失败登录（统一入口）.
func (sm *Manager) RecordFailedLogin(ip, username, userAgent, reason string) error {
	// 记录到审计日志
	sm.audit.LogLogin(LoginLogEntry{
		Username:  username,
		IP:        ip,
		UserAgent: userAgent,
		Status:    "failure",
		Reason:    reason,
	})

	// 记录到 fail2ban
	return sm.fail2ban.RecordFailedLogin(ip, username, userAgent, reason)
}

// RecordSuccessfulLogin 记录成功登录.
func (sm *Manager) RecordSuccessfulLogin(ip, username, userAgent, mfaMethod string) {
	// 记录到审计日志
	sm.audit.LogLogin(LoginLogEntry{
		Username:  username,
		IP:        ip,
		UserAgent: userAgent,
		Status:    "success",
		MFAMethod: mfaMethod,
	})

	// 清除 fail2ban 记录
	sm.fail2ban.RecordSuccessfulLogin(ip, username)
}

// RecordAction 记录操作日志.
func (sm *Manager) RecordAction(userID, username, ip, resource, action string, details map[string]interface{}, status string) {
	sm.audit.LogAction(userID, username, ip, resource, action, details, status)
}

// IsAccessAllowed 检查访问是否允许.
func (sm *Manager) IsAccessAllowed(ip string) bool {
	// 检查是否在白名单
	if sm.firewall.IsWhitelisted(ip) {
		return true
	}

	// 检查是否在黑名单
	if sm.firewall.IsBlacklisted(ip) {
		return false
	}

	// 检查是否被封禁
	if sm.fail2ban.IsBanned(ip) {
		return false
	}

	return true
}

// BanIP 封禁 IP.
func (sm *Manager) BanIP(ip, reason string, durationMinutes int) error {
	return sm.firewall.AddToBlacklist(ip, reason, durationMinutes)
}

// UnbanIP 解封 IP.
func (sm *Manager) UnbanIP(ip string) error {
	// 从防火墙黑名单移除
	if err := sm.firewall.RemoveFromBlacklist(ip); err != nil {
		// 尝试从 fail2ban 解封
		return sm.fail2ban.UnbanIP(ip)
	}
	return nil
}

// GetSecurityStatus 获取安全状态概览.
func (sm *Manager) GetSecurityStatus() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	firewallRules := len(sm.firewall.ListRules())
	bannedIPs := len(sm.fail2ban.GetBannedIPs())
	alertStats := sm.audit.GetAlertStats()

	return map[string]interface{}{
		"firewall_enabled": sm.config.Firewall.Enabled,
		"firewall_rules":   firewallRules,
		"fail2ban_enabled": sm.config.Fail2Ban.Enabled,
		"banned_ips":       bannedIPs,
		"alert_stats":      alertStats,
		"audit_enabled":    sm.config.AuditEnabled,
	}
}

// RunBaselineCheck 运行基线检查.
func (sm *Manager) RunBaselineCheck() BaselineReport {
	return sm.baseline.RunAllChecks()
}

// GetDashboard 获取安全仪表板数据.
func (sm *Manager) GetDashboard() map[string]interface{} {
	now := time.Now()
	startTime := now.Add(-24 * time.Hour)

	loginStats := sm.audit.GetLoginStats(startTime, now)
	baselineReport := sm.baseline.RunAllChecks()

	return map[string]interface{}{
		"timestamp":             now,
		"security_status":       sm.GetSecurityStatus(),
		"login_stats_24h":       loginStats,
		"baseline_score":        baselineReport.OverallScore,
		"baseline_failed":       baselineReport.Failed,
		"unacknowledged_alerts": sm.getUnacknowledgedAlertsCount(),
	}
}

func (sm *Manager) getUnacknowledgedAlertsCount() int {
	alerts := sm.audit.GetAlerts(1000, 0, func() *bool { b := false; return &b }())
	return len(alerts)
}
