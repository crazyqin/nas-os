package security

import (
	"testing"
	"time"
)

func TestNewAuditManager(t *testing.T) {
	am := NewAuditManager()
	if am == nil {
		t.Fatal("NewAuditManager 返回 nil")
	}

	config := am.GetConfig()
	if !config.Enabled {
		t.Error("审计日志应默认启用")
	}
	if config.MaxLogs != 10000 {
		t.Errorf("默认最大日志数应为 10000，实际为 %d", config.MaxLogs)
	}
	if config.MaxAgeDays != 90 {
		t.Errorf("默认最大保留天数应为 90，实际为 %d", config.MaxAgeDays)
	}
}

func TestAuditManager_SetConfig(t *testing.T) {
	am := NewAuditManager()

	newConfig := AuditConfig{
		Enabled:        true,
		LogPath:        "/tmp/test-audit",
		MaxLogs:        5000,
		MaxAgeDays:     30,
		AutoSave:       false,
		SaveInterval:   time.Minute * 10,
		AlertEnabled:   false,
		AlertThreshold: 5,
	}

	am.SetConfig(newConfig)
	config := am.GetConfig()

	if config.MaxLogs != 5000 {
		t.Errorf("配置更新失败，MaxLogs 应为 5000，实际为 %d", config.MaxLogs)
	}
	if config.AlertEnabled != false {
		t.Error("AlertEnabled 应为 false")
	}
}

func TestAuditManager_Log(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	entry := AuditLogEntry{
		Level:    "info",
		Category: "system",
		Event:    "test_event",
		Username: "testuser",
		IP:       "192.168.1.100",
		Status:   "success",
	}

	am.Log(entry)

	logs := am.GetAuditLogs(100, 0, nil)
	if len(logs) != 1 {
		t.Fatalf("应有 1 条日志，实际为 %d", len(logs))
	}

	if logs[0].Event != "test_event" {
		t.Errorf("事件类型应为 test_event，实际为 %s", logs[0].Event)
	}
	if logs[0].ID == "" {
		t.Error("日志 ID 不应为空")
	}
}

func TestAuditManager_LogDisabled(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      false, // 禁用审计
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	entry := AuditLogEntry{
		Level:    "info",
		Category: "system",
		Event:    "test_event",
	}

	am.Log(entry)

	logs := am.GetAuditLogs(100, 0, nil)
	if len(logs) != 0 {
		t.Errorf("审计禁用时不应记录日志，实际有 %d 条", len(logs))
	}
}

func TestAuditManager_MaxLogs(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      5,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 添加 10 条日志
	for i := 0; i < 10; i++ {
		am.Log(AuditLogEntry{
			Level:    "info",
			Category: "system",
			Event:    "test_event",
		})
	}

	logs := am.GetAuditLogs(100, 0, nil)
	if len(logs) != 5 {
		t.Errorf("应限制为 5 条日志，实际为 %d", len(logs))
	}
}

func TestAuditManager_LogLogin(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 成功登录
	am.LogLogin(LoginLogEntry{
		Username: "admin",
		IP:       "192.168.1.100",
		Status:   "success",
	})

	// 失败登录
	am.LogLogin(LoginLogEntry{
		Username: "admin",
		IP:       "192.168.1.100",
		Status:   "failure",
		Reason:   "密码错误",
	})

	loginLogs := am.GetLoginLogs(100, 0, nil)
	if len(loginLogs) != 2 {
		t.Errorf("应有 2 条登录日志，实际为 %d", len(loginLogs))
	}

	// 检查审计日志（失败登录应记录到审计日志）
	auditLogs := am.GetAuditLogs(100, 0, nil)
	var failureCount int
	for _, log := range auditLogs {
		if log.Event == "login_failure" {
			failureCount++
		}
	}
	if failureCount != 1 {
		t.Errorf("应有 1 条登录失败审计日志，实际为 %d", failureCount)
	}
}

func TestAuditManager_AlertGeneration(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: true,
	})

	// 记录严重事件
	am.Log(AuditLogEntry{
		Level:    "critical",
		Category: "security",
		Event:    "unauthorized_access",
		Username: "unknown",
		IP:       "10.0.0.1",
	})

	alerts := am.GetAlerts(100, 0, nil)
	if len(alerts) == 0 {
		t.Fatal("严重事件应生成告警")
	}

	// unauthorized_access 事件级别为 high
	if alerts[0].Severity != "high" {
		t.Errorf("告警严重级别应为 high，实际为 %s", alerts[0].Severity)
	}
}

func TestAuditLogEntry_Message(t *testing.T) {
	tests := []struct {
		name     string
		entry    AuditLogEntry
		expected string
	}{
		{
			name: "基本消息",
			entry: AuditLogEntry{
				Category: "auth",
				Event:    "login",
			},
			expected: "[auth] login",
		},
		{
			name: "带用户名",
			entry: AuditLogEntry{
				Category: "auth",
				Event:    "login",
				Username: "admin",
			},
			expected: "[auth] login by admin",
		},
		{
			name: "带IP",
			entry: AuditLogEntry{
				Category: "auth",
				Event:    "login",
				Username: "admin",
				IP:       "192.168.1.1",
			},
			expected: "[auth] login by admin from 192.168.1.1",
		},
		{
			name: "完整消息",
			entry: AuditLogEntry{
				Category: "auth",
				Event:    "login",
				Username: "admin",
				IP:       "192.168.1.1",
				Status:   "success",
			},
			expected: "[auth] login by admin from 192.168.1.1 - success",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.entry.Message()
			if msg != tt.expected {
				t.Errorf("消息应为 %q，实际为 %q", tt.expected, msg)
			}
		})
	}
}

func TestNewFirewallManager(t *testing.T) {
	fm := NewFirewallManager()
	if fm == nil {
		t.Fatal("NewFirewallManager 返回 nil")
	}

	config := fm.GetConfig()
	if !config.Enabled {
		t.Error("防火墙应默认启用")
	}
}

func TestNewFail2BanManager(t *testing.T) {
	f2b := NewFail2BanManager()
	if f2b == nil {
		t.Fatal("NewFail2BanManager 返回 nil")
	}
}

func TestNewBaselineManager(t *testing.T) {
	bm := NewBaselineManager()
	if bm == nil {
		t.Fatal("NewBaselineManager 返回 nil")
	}
}

func TestAuditManager_LogAction(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	am.LogAction("user1", "admin", "192.168.1.1", "file", "delete", map[string]interface{}{
		"path": "/tmp/test.txt",
	}, "success")

	logs := am.GetAuditLogs(100, 0, nil)
	if len(logs) != 1 {
		t.Fatalf("应有 1 条日志，实际为 %d", len(logs))
	}

	if logs[0].Event != "delete" {
		t.Errorf("事件应为 delete，实际为 %s", logs[0].Event)
	}
}

func TestAuditManager_GetAlertStats(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: true,
	})

	// 生成一些告警
	am.Log(AuditLogEntry{
		Level:    "critical",
		Category: "security",
		Event:    "unauthorized_access",
	})
	am.Log(AuditLogEntry{
		Level:    "warning",
		Category: "auth",
		Event:    "login_failure_multiple",
	})

	stats := am.GetAlertStats()
	if stats == nil {
		t.Fatal("GetAlertStats 返回 nil")
	}

	total, ok := stats["total"].(int)
	if !ok || total != 2 {
		t.Errorf("总告警数应为 2，实际为 %v", stats["total"])
	}
}

func TestAuditManager_LoginStats(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 添加登录记录
	am.LogLogin(LoginLogEntry{Username: "admin", Status: "success"})
	am.LogLogin(LoginLogEntry{Username: "admin", Status: "success"})
	am.LogLogin(LoginLogEntry{Username: "admin", Status: "failure", Reason: "密码错误"})

	now := time.Now()
	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)

	stats := am.GetLoginStats(start, end)
	if stats == nil {
		t.Fatal("GetLoginStats 返回 nil")
	}
}
