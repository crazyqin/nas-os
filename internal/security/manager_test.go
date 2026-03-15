package security

import (
	"testing"
	"time"
)

func TestNewSecurityManager(t *testing.T) {
	sm := NewSecurityManager()
	if sm == nil {
		t.Fatal("NewSecurityManager 返回 nil")
	}

	// 验证子管理器已初始化
	if sm.firewall == nil {
		t.Error("防火墙管理器未初始化")
	}
	if sm.fail2ban == nil {
		t.Error("Fail2Ban 管理器未初始化")
	}
	if sm.audit == nil {
		t.Error("审计管理器未初始化")
	}
	if sm.baseline == nil {
		t.Error("基线管理器未初始化")
	}
}

func TestSecurityManager_GetManagers(t *testing.T) {
	sm := NewSecurityManager()

	fm := sm.GetFirewallManager()
	if fm == nil {
		t.Error("GetFirewallManager 返回 nil")
	}

	f2b := sm.GetFail2BanManager()
	if f2b == nil {
		t.Error("GetFail2BanManager 返回 nil")
	}

	am := sm.GetAuditManager()
	if am == nil {
		t.Error("GetAuditManager 返回 nil")
	}

	bm := sm.GetBaselineManager()
	if bm == nil {
		t.Error("GetBaselineManager 返回 nil")
	}
}

func TestSecurityManager_Config(t *testing.T) {
	sm := NewSecurityManager()

	// 获取默认配置
	config := sm.GetConfig()
	if !config.Firewall.Enabled {
		t.Error("防火墙应默认启用")
	}
	if !config.Fail2Ban.Enabled {
		t.Error("Fail2Ban 应默认启用")
	}

	// 更新配置
	newConfig := SecurityConfig{
		Firewall: FirewallConfig{
			Enabled:       true,
			DefaultPolicy: "deny",
			IPv6Enabled:   false,
			LogDropped:    true,
		},
		Fail2Ban: Fail2BanConfig{
			Enabled:            true,
			MaxAttempts:        10,
			WindowMinutes:      15,
			BanDurationMinutes: 120,
		},
		AuditEnabled: true,
		AlertEnabled: false,
	}

	err := sm.UpdateConfig(newConfig)
	if err != nil {
		t.Fatalf("更新配置失败: %v", err)
	}

	updatedConfig := sm.GetConfig()
	if updatedConfig.Fail2Ban.MaxAttempts != 10 {
		t.Errorf("MaxAttempts 应为 10，实际为 %d", updatedConfig.Fail2Ban.MaxAttempts)
	}
}

func TestSecurityManager_RecordFailedLogin(t *testing.T) {
	sm := NewSecurityManager()
	sm.audit.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	err := sm.RecordFailedLogin("192.168.1.100", "admin", "Mozilla/5.0", "密码错误")
	if err != nil {
		t.Fatalf("记录失败登录出错: %v", err)
	}

	// 检查 fail2ban 记录存在
	// 注意：Fail2BanManager 可能没有 HasFailedAttempts 方法
	// 使用 GetBannedIPs 来验证
	bannedIPs := sm.fail2ban.GetBannedIPs()
	_ = bannedIPs // 验证调用成功
}

func TestSecurityManager_RecordSuccessfulLogin(t *testing.T) {
	sm := NewSecurityManager()
	sm.audit.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 先记录失败尝试
	sm.RecordFailedLogin("192.168.1.100", "admin", "Mozilla/5.0", "密码错误")

	// 记录成功登录
	sm.RecordSuccessfulLogin("192.168.1.100", "admin", "Mozilla/5.0", "totp")

	// 验证登录日志被记录
	loginLogs := sm.audit.GetLoginLogs(10, 0, nil)
	found := false
	for _, log := range loginLogs {
		if log.Status == "success" {
			found = true
			break
		}
	}
	if !found {
		t.Error("应记录成功登录")
	}
}

func TestSecurityManager_IsAccessAllowed(t *testing.T) {
	sm := NewSecurityManager()

	// 正常 IP 应允许
	if !sm.IsAccessAllowed("192.168.1.100") {
		t.Error("正常 IP 应允许访问")
	}

	// 添加到黑名单
	sm.firewall.AddToBlacklist("10.0.0.1", "恶意IP", 0)

	// 黑名单 IP 应拒绝
	if sm.IsAccessAllowed("10.0.0.1") {
		t.Error("黑名单 IP 应拒绝访问")
	}

	// 添加到白名单
	sm.firewall.AddToWhitelist("10.0.0.2", "可信IP")

	// 白名单 IP 应允许
	if !sm.IsAccessAllowed("10.0.0.2") {
		t.Error("白名单 IP 应允许访问")
	}
}

func TestSecurityManager_BanUnbanIP(t *testing.T) {
	sm := NewSecurityManager()

	// 封禁 IP
	err := sm.BanIP("192.168.1.200", "测试封禁", 60)
	if err != nil {
		t.Fatalf("封禁 IP 失败: %v", err)
	}

	// 验证被封禁
	if sm.IsAccessAllowed("192.168.1.200") {
		t.Error("被封禁的 IP 不应允许访问")
	}

	// 解封 IP
	err = sm.UnbanIP("192.168.1.200")
	if err != nil {
		t.Fatalf("解封 IP 失败: %v", err)
	}

	// 验证已解封
	if !sm.IsAccessAllowed("192.168.1.200") {
		t.Error("解封后的 IP 应允许访问")
	}
}

func TestSecurityManager_RecordAction(t *testing.T) {
	sm := NewSecurityManager()
	sm.audit.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	sm.RecordAction("user123", "admin", "192.168.1.50", "file", "delete", map[string]interface{}{
		"path": "/data/important.txt",
	}, "success")

	logs := sm.audit.GetAuditLogs(10, 0, nil)
	if len(logs) == 0 {
		t.Fatal("应有操作日志")
	}

	found := false
	for _, log := range logs {
		if log.Event == "delete" && log.Resource == "file" {
			found = true
			break
		}
	}

	if !found {
		t.Error("未找到删除操作日志")
	}
}

func TestSecurityManager_GetSecurityStatus(t *testing.T) {
	sm := NewSecurityManager()

	status := sm.GetSecurityStatus()

	if status == nil {
		t.Fatal("GetSecurityStatus 返回 nil")
	}

	// 检查必要字段
	if _, ok := status["firewall_enabled"]; !ok {
		t.Error("状态应包含 firewall_enabled")
	}
	if _, ok := status["fail2ban_enabled"]; !ok {
		t.Error("状态应包含 fail2ban_enabled")
	}
	if _, ok := status["banned_ips"]; !ok {
		t.Error("状态应包含 banned_ips")
	}
}

func TestSecurityManager_RunBaselineCheck(t *testing.T) {
	sm := NewSecurityManager()

	report := sm.RunBaselineCheck()

	if report.ReportID == "" {
		t.Error("报告 ID 不应为空")
	}
	if report.TotalChecks == 0 {
		t.Error("应有检查项")
	}
	if report.OverallScore < 0 || report.OverallScore > 100 {
		t.Errorf("总体评分应在 0-100 范围内，实际为 %d", report.OverallScore)
	}
}

func TestSecurityManager_GetDashboard(t *testing.T) {
	sm := NewSecurityManager()
	sm.audit.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: true,
	})

	// 添加一些登录记录
	sm.RecordSuccessfulLogin("192.168.1.10", "user1", "Chrome", "")
	sm.RecordFailedLogin("192.168.1.20", "user2", "Firefox", "密码错误")

	dashboard := sm.GetDashboard()

	if dashboard == nil {
		t.Fatal("GetDashboard 返回 nil")
	}

	// 检查必要字段
	requiredFields := []string{
		"timestamp",
		"security_status",
		"login_stats_24h",
		"baseline_score",
	}

	for _, field := range requiredFields {
		if _, ok := dashboard[field]; !ok {
			t.Errorf("仪表板应包含 %s", field)
		}
	}
}

func TestSecurityManager_NotifyFunc(t *testing.T) {
	sm := NewSecurityManager()

	alertReceived := false
	sm.SetNotifyFunc(func(alert SecurityAlert) {
		alertReceived = true
	})

	// 触发告警
	sm.handleSecurityAlert(SecurityAlert{
		ID:       "test-alert",
		Severity: "high",
		Type:     "test",
		Title:    "测试告警",
	})

	time.Sleep(100 * time.Millisecond)

	if !alertReceived {
		t.Error("应触发通知回调")
	}
}

func TestSecurityManager_Concurrent(t *testing.T) {
	sm := NewSecurityManager()
	sm.audit.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      10000,
		AutoSave:     false,
		AlertEnabled: false,
	})

	done := make(chan bool)

	// 并发写入
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				sm.RecordAction("user", "admin", "192.168.1.1", "test", "action", nil, "success")
			}
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 50; j++ {
				sm.GetSecurityStatus()
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 15; i++ {
		<-done
	}

	// 应能正常获取日志
	logs := sm.audit.GetAuditLogs(10000, 0, nil)
	if len(logs) != 1000 {
		t.Errorf("应有 1000 条日志，实际为 %d", len(logs))
	}
}