// Package security 提供安全审计功能测试
package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditManager_AcknowledgeAlert(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: true,
	})

	// 生成一个告警
	am.Log(AuditLogEntry{
		Level:    "critical",
		Category: "security",
		Event:    "unauthorized_access",
		Username: "hacker",
		IP:       "10.0.0.1",
	})

	alerts := am.GetAlerts(100, 0, nil)
	require.Len(t, alerts, 1)

	// 确认告警
	err := am.AcknowledgeAlert(alerts[0].ID, "admin")
	require.NoError(t, err)

	// 验证状态更新
	alerts = am.GetAlerts(100, 0, nil)
	assert.True(t, alerts[0].Acknowledged)
	assert.Equal(t, "admin", alerts[0].AckedBy)
	assert.NotNil(t, alerts[0].AckedAt)
}

func TestAuditManager_AcknowledgeAlert_NotFound(t *testing.T) {
	am := NewAuditManager()

	err := am.AcknowledgeAlert("nonexistent", "admin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "告警不存在")
}

func TestAuditManager_GetAlerts_FilterByAcknowledged(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: true,
	})

	// 生成多个告警
	for i := 0; i < 3; i++ {
		am.Log(AuditLogEntry{
			Level:    "critical",
			Category: "security",
			Event:    "unauthorized_access",
		})
	}

	alerts := am.GetAlerts(100, 0, nil)
	require.Len(t, alerts, 3)

	// 确认第一个告警
	am.AcknowledgeAlert(alerts[0].ID, "admin")

	// 只获取未确认的告警
	unacked := bool(false)
	unacknowledgedAlerts := am.GetAlerts(100, 0, &unacked)
	assert.Len(t, unacknowledgedAlerts, 2)

	// 只获取已确认的告警
	acked := bool(true)
	acknowledgedAlerts := am.GetAlerts(100, 0, &acked)
	assert.Len(t, acknowledgedAlerts, 1)
}

func TestAuditManager_GetAlerts_Pagination(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: true,
	})

	// 生成多个告警
	for i := 0; i < 10; i++ {
		am.Log(AuditLogEntry{
			Level:    "warning",
			Category: "auth",
			Event:    "login_failure_multiple",
		})
	}

	// 第一页
	page1 := am.GetAlerts(5, 0, nil)
	assert.Len(t, page1, 5)

	// 第二页
	page2 := am.GetAlerts(5, 5, nil)
	assert.Len(t, page2, 5)

	// 超出范围的偏移
	empty := am.GetAlerts(5, 100, nil)
	assert.Empty(t, empty)
}

func TestAuditManager_GetAuditLogs_Filters(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 添加不同类型的日志
	am.Log(AuditLogEntry{Level: "info", Category: "system", Event: "startup", Username: "system"})
	am.Log(AuditLogEntry{Level: "warning", Category: "auth", Event: "login_failure", Username: "user1"})
	am.Log(AuditLogEntry{Level: "error", Category: "file", Event: "delete", Username: "user2"})
	am.Log(AuditLogEntry{Level: "info", Category: "auth", Event: "login_success", Username: "user1"})

	tests := []struct {
		name     string
		filters  map[string]string
		expected int
	}{
		{
			name:     "按类别过滤",
			filters:  map[string]string{"category": "auth"},
			expected: 2,
		},
		{
			name:     "按级别过滤",
			filters:  map[string]string{"level": "warning"},
			expected: 1,
		},
		{
			name:     "按用户名过滤",
			filters:  map[string]string{"username": "user1"},
			expected: 2,
		},
		{
			name:     "按事件过滤",
			filters:  map[string]string{"event": "startup"},
			expected: 1,
		},
		{
			name:     "组合过滤",
			filters:  map[string]string{"category": "auth", "username": "user1"},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := am.GetAuditLogs(100, 0, tt.filters)
			assert.Len(t, logs, tt.expected)
		})
	}
}

func TestAuditManager_GetLoginLogs_Filters(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 添加不同类型的登录日志
	am.LogLogin(LoginLogEntry{Username: "admin", IP: "192.168.1.1", Status: "success"})
	am.LogLogin(LoginLogEntry{Username: "admin", IP: "192.168.1.1", Status: "failure", Reason: "密码错误"})
	am.LogLogin(LoginLogEntry{Username: "user1", IP: "10.0.0.1", Status: "success"})

	tests := []struct {
		name     string
		filters  map[string]string
		expected int
	}{
		{
			name:     "按用户名过滤",
			filters:  map[string]string{"username": "admin"},
			expected: 2,
		},
		{
			name:     "按状态过滤",
			filters:  map[string]string{"status": "failure"},
			expected: 1,
		},
		{
			name:     "按IP过滤",
			filters:  map[string]string{"ip": "10.0.0.1"},
			expected: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := am.GetLoginLogs(100, 0, tt.filters)
			assert.Len(t, logs, tt.expected)
		})
	}
}

func TestAuditManager_CleanupOldLogs(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		MaxAgeDays:   1,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 手动添加旧日志
	oldTime := time.Now().Add(-48 * time.Hour)
	am.logs = append(am.logs, &AuditLogEntry{
		ID:        "old-log",
		Timestamp: oldTime,
		Level:     "info",
		Category:  "system",
		Event:     "old_event",
	})

	// 添加新日志
	am.Log(AuditLogEntry{
		Level:    "info",
		Category: "system",
		Event:    "new_event",
	})

	// 清理旧日志
	am.CleanupOldLogs()

	logs := am.GetAuditLogs(100, 0, nil)
	assert.Len(t, logs, 1)
	assert.Equal(t, "new_event", logs[0].Event)
}

func TestAuditManager_CleanupOldLoginLogs(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		MaxAgeDays:   1,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 手动添加旧登录日志
	oldTime := time.Now().Add(-48 * time.Hour)
	am.loginLogs = append(am.loginLogs, &LoginLogEntry{
		ID:        "old-login",
		Timestamp: oldTime,
		Username:  "olduser",
	})

	// 添加新登录日志
	am.LogLogin(LoginLogEntry{
		Username: "newuser",
		Status:   "success",
	})

	am.CleanupOldLogs()

	logs := am.GetLoginLogs(100, 0, nil)
	assert.Len(t, logs, 1)
	assert.Equal(t, "newuser", logs[0].Username)
}

func TestAuditManager_CleanupOldAlerts(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		MaxAgeDays:   1,
		AutoSave:     false,
		AlertEnabled: true,
	})

	// 手动添加旧告警
	oldTime := time.Now().Add(-48 * time.Hour)
	am.alerts = append(am.alerts, &Alert{
		ID:        "old-alert",
		Timestamp: oldTime,
		Severity:  "high",
		Type:      "old_type",
	})

	// 生成新告警
	am.Log(AuditLogEntry{
		Level:    "critical",
		Category: "security",
		Event:    "unauthorized_access",
	})

	am.CleanupOldLogs()

	alerts := am.GetAlerts(100, 0, nil)
	assert.Len(t, alerts, 1)
}

func TestAuditManager_ExportLogs_JSON(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	am.Log(AuditLogEntry{Level: "info", Category: "system", Event: "test1"})
	am.Log(AuditLogEntry{Level: "warning", Category: "auth", Event: "test2"})

	now := time.Now()
	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)

	data, err := am.ExportLogs(start, end, "json")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "test1")
	assert.Contains(t, string(data), "test2")
}

func TestAuditManager_ExportLogs_CSV(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	am.Log(AuditLogEntry{
		Level:    "info",
		Category: "system",
		Event:    "test_event",
		Username: "testuser",
		IP:       "192.168.1.1",
		Status:   "success",
	})

	now := time.Now()
	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)

	data, err := am.ExportLogs(start, end, "csv")
	require.NoError(t, err)
	assert.NotEmpty(t, data)
	assert.Contains(t, string(data), "Timestamp,Level,Category,Event,Username,IP,Status")
	assert.Contains(t, string(data), "test_event")
}

func TestAuditManager_ExportLogs_TimeRange(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	am.Log(AuditLogEntry{Level: "info", Category: "system", Event: "recent_event"})

	// 导出时间范围在过去（不包含当前日志）
	now := time.Now()
	start := now.Add(-2 * time.Hour)
	end := now.Add(-time.Hour)

	data, err := am.ExportLogs(start, end, "json")
	require.NoError(t, err)
	// 应该是空数组或没有数据
	assert.NotContains(t, string(data), "recent_event")
}

func TestAuditManager_GetLoginStats_TimeRange(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 添加登录记录
	am.LogLogin(LoginLogEntry{Username: "admin", IP: "192.168.1.1", Status: "success"})
	am.LogLogin(LoginLogEntry{Username: "admin", IP: "192.168.1.1", Status: "failure", Reason: "密码错误"})
	am.LogLogin(LoginLogEntry{Username: "user1", IP: "10.0.0.1", Status: "success"})

	now := time.Now()
	start := now.Add(-time.Hour)
	end := now.Add(time.Hour)

	stats := am.GetLoginStats(start, end)

	require.NotNil(t, stats)
	assert.Equal(t, 3, stats["total"])
	assert.Equal(t, 2, stats["success"])
	assert.Equal(t, 1, stats["failure"])

	byUser := stats["by_user"].(map[string]int)
	assert.Equal(t, 2, byUser["admin"])
	assert.Equal(t, 1, byUser["user1"])

	byIP := stats["by_ip"].(map[string]int)
	assert.Equal(t, 2, byIP["192.168.1.1"])
	assert.Equal(t, 1, byIP["10.0.0.1"])
}

func TestAuditManager_GetLoginStats_OutsideTimeRange(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	am.LogLogin(LoginLogEntry{Username: "admin", Status: "success"})

	// 时间范围在未来
	now := time.Now()
	start := now.Add(time.Hour)
	end := now.Add(2 * time.Hour)

	stats := am.GetLoginStats(start, end)

	assert.Equal(t, 0, stats["total"])
}

func TestAuditManager_LogAction_Failure(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	am.LogAction("user1", "admin", "192.168.1.1", "file", "delete", nil, "failure")

	logs := am.GetAuditLogs(100, 0, nil)
	require.Len(t, logs, 1)

	assert.Equal(t, "warning", logs[0].Level) // 失败应该是warning级别
	assert.Equal(t, "delete", logs[0].Event)
	assert.Equal(t, "failure", logs[0].Status)
}

func TestAuditManager_LogLogin_Disabled(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      false,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	am.LogLogin(LoginLogEntry{Username: "admin", Status: "success"})

	logs := am.GetLoginLogs(100, 0, nil)
	assert.Empty(t, logs)
}

func TestAuditManager_CheckAlertCondition_Levels(t *testing.T) {
	tests := []struct {
		level    string
		event    string
		expected string
	}{
		{"critical", "data_breach", "critical"},
		{"error", "system_error", "high"},
		{"warning", "config_change", "medium"},
		{"info", "normal_event", ""}, // info不会触发告警
	}

	for _, tt := range tests {
		t.Run(tt.level+"_"+tt.event, func(t *testing.T) {
			am := NewAuditManager()
			am.SetConfig(AuditConfig{
				Enabled:      true,
				MaxLogs:      100,
				AutoSave:     false,
				AlertEnabled: true,
			})

			am.Log(AuditLogEntry{
				Level:    tt.level,
				Category: "security",
				Event:    tt.event,
			})

			alerts := am.GetAlerts(100, 0, nil)
			if tt.expected == "" {
				assert.Empty(t, alerts)
			} else {
				require.Len(t, alerts, 1)
				assert.Equal(t, tt.expected, alerts[0].Severity)
			}
		})
	}
}

func TestAuditManager_CheckAlertCondition_SpecialEvents(t *testing.T) {
	specialEvents := []string{
		"login_failure_multiple",
		"firewall_rule_violation",
		"unauthorized_access",
	}

	for _, event := range specialEvents {
		t.Run(event, func(t *testing.T) {
			am := NewAuditManager()
			am.SetConfig(AuditConfig{
				Enabled:      true,
				MaxLogs:      100,
				AutoSave:     false,
				AlertEnabled: true,
			})

			am.Log(AuditLogEntry{
				Level:    "info", // 即使是info级别
				Category: "security",
				Event:    event,
			})

			alerts := am.GetAlerts(100, 0, nil)
			require.Len(t, alerts, 1)
			assert.Equal(t, "high", alerts[0].Severity)
		})
	}
}

func TestAuditConfig_Defaults(t *testing.T) {
	am := NewAuditManager()
	config := am.GetConfig()

	assert.True(t, config.Enabled)
	assert.Equal(t, "/var/log/nas-os/audit", config.LogPath)
	assert.Equal(t, 10000, config.MaxLogs)
	assert.Equal(t, 90, config.MaxAgeDays)
	assert.True(t, config.AutoSave)
	assert.Equal(t, time.Minute*5, config.SaveInterval)
	assert.True(t, config.AlertEnabled)
	assert.Equal(t, 10, config.AlertThreshold)
}

func TestAlert_Fields(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: true,
	})

	am.Log(AuditLogEntry{
		Level:    "critical",
		Category: "security",
		Event:    "data_breach",
		Username: "hacker",
		IP:       "10.0.0.1",
		Details: map[string]interface{}{
			"attempted_table": "users",
		},
	})

	alerts := am.GetAlerts(100, 0, nil)
	require.Len(t, alerts, 1)

	alert := alerts[0]
	assert.NotEmpty(t, alert.ID)
	assert.NotEmpty(t, alert.Title)
	assert.Contains(t, alert.Description, "data_breach")
	assert.Equal(t, "10.0.0.1", alert.SourceIP)
	assert.Equal(t, "hacker", alert.Username)
	assert.NotNil(t, alert.Details)
}

func TestAuditManager_GetAuditLogs_Pagination(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 添加10条日志
	for i := 0; i < 10; i++ {
		am.Log(AuditLogEntry{
			Level:    "info",
			Category: "system",
			Event:    "test_event",
		})
	}

	// 第一页
	page1 := am.GetAuditLogs(5, 0, nil)
	assert.Len(t, page1, 5)

	// 第二页
	page2 := am.GetAuditLogs(5, 5, nil)
	assert.Len(t, page2, 5)

	// 超出范围
	empty := am.GetAuditLogs(5, 100, nil)
	assert.Empty(t, empty)
}

func TestAuditManager_GetLoginLogs_Pagination(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 添加登录记录
	for i := 0; i < 10; i++ {
		am.LogLogin(LoginLogEntry{
			Username: "admin",
			Status:   "success",
		})
	}

	page1 := am.GetLoginLogs(5, 0, nil)
	assert.Len(t, page1, 5)

	page2 := am.GetLoginLogs(5, 5, nil)
	assert.Len(t, page2, 5)
}

func TestAuditManager_MaxLoginLogs(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      5,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 添加10条登录日志
	for i := 0; i < 10; i++ {
		am.LogLogin(LoginLogEntry{
			Username: "admin",
			Status:   "success",
		})
	}

	logs := am.GetLoginLogs(100, 0, nil)
	assert.Len(t, logs, 5)
}

func TestAuditManager_StartCleanupRoutine(t *testing.T) {
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		MaxAgeDays:   90,
		AutoSave:     false,
		AlertEnabled: false,
	})

	// 启动清理例程（很短的间隔用于测试）
	am.StartCleanupRoutine(100 * time.Millisecond)

	// 添加日志
	am.Log(AuditLogEntry{Level: "info", Category: "system", Event: "test"})

	// 等待一次清理周期
	time.Sleep(150 * time.Millisecond)

	// 日志应该还存在（因为不是旧日志）
	logs := am.GetAuditLogs(100, 0, nil)
	assert.Len(t, logs, 1)
}
