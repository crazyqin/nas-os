package smb

import (
	"testing"
	"time"
)

func TestNewSecurityAuditManager(t *testing.T) {
	sam := NewSecurityAuditManager()
	if sam == nil {
		t.Fatal("NewSecurityAuditManager 返回 nil")
	}

	config := sam.GetConfig()
	if !config.Enabled {
		t.Error("安全审计应默认启用")
	}
	if !config.FileAccessAudit {
		t.Error("文件访问审计应默认启用")
	}
	if !config.AnomalyDetection {
		t.Error("异常检测应默认启用")
	}
	if !config.RansomwareDetection {
		t.Error("勒索软件检测应默认启用")
	}
	if config.MaxRecords != 50000 {
		t.Errorf("默认最大记录数应为 50000，实际为 %d", config.MaxRecords)
	}
	if config.RetentionDays != 90 {
		t.Errorf("默认保留天数应为 90，实际为 %d", config.RetentionDays)
	}
}

func TestSecurityAuditManager_SetConfig(t *testing.T) {
	sam := NewSecurityAuditManager()

	newConfig := SecurityAuditConfig{
		Enabled:             true,
		FileAccessAudit:     true,
		AuthAudit:           true,
		AnomalyDetection:    false,
		RansomwareDetection: false,
		MaxRecords:          10000,
		RetentionDays:       30,
		AlertThreshold:      5,
		AlertChannels:       []string{"email"},
		WhitelistedIPs:      []string{"192.168.1.1"},
		WhitelistedUsers:    []string{"admin"},
	}

	sam.SetConfig(newConfig)
	config := sam.GetConfig()

	if config.MaxRecords != 10000 {
		t.Errorf("配置更新失败，MaxRecords 应为 10000，实际为 %d", config.MaxRecords)
	}
	if config.AnomalyDetection != false {
		t.Error("AnomalyDetection 应为 false")
	}
	if len(config.WhitelistedIPs) != 1 {
		t.Errorf("白名单 IP 数量应为 1，实际为 %d", len(config.WhitelistedIPs))
	}
	if len(config.WhitelistedUsers) != 1 {
		t.Errorf("白名单用户数量应为 1，实际为 %d", len(config.WhitelistedUsers))
	}
}

func TestSecurityAuditManager_LogFileAccess(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: false,
		RansomwareDetection: false,
		MaxRecords:      100,
	})

	record := FileAccessRecord{
		ShareName: "documents",
		FilePath:  "/documents/report.pdf",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		Action:    "read",
		Status:    "success",
	}

	sam.LogFileAccess(record)

	records := sam.GetFileAccessRecords(100, 0, nil)
	if len(records) != 1 {
		t.Fatalf("应有 1 条记录，实际为 %d", len(records))
	}

	if records[0].FilePath != "/documents/report.pdf" {
		t.Errorf("文件路径应为 /documents/report.pdf，实际为 %s", records[0].FilePath)
	}
	if records[0].ID == "" {
		t.Error("记录 ID 不应为空")
	}
	if records[0].Timestamp.IsZero() {
		t.Error("时间戳不应为零")
	}
}

func TestSecurityAuditManager_LogFileAccessDisabled(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         false, // 禁用审计
		FileAccessAudit: false,
		MaxRecords:      100,
	})

	record := FileAccessRecord{
		ShareName: "documents",
		FilePath:  "/documents/report.pdf",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		Action:    "read",
	}

	sam.LogFileAccess(record)

	records := sam.GetFileAccessRecords(100, 0, nil)
	if len(records) != 0 {
		t.Errorf("审计禁用时不应记录，实际有 %d 条", len(records))
	}
}

func TestSecurityAuditManager_LogSecurityEvent(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:    true,
		MaxRecords: 100,
	})

	event := SecurityEvent{
		EventType: "access",
		Severity:  "high",
		ShareName: "documents",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		FilePath:  "/documents/sensitive.pdf",
		Action:    "read",
		Status:    "detected",
	}

	sam.LogSecurityEvent(event)

	events := sam.GetSecurityEvents(100, 0, nil)
	if len(events) != 1 {
		t.Fatalf("应有 1 条事件，实际为 %d", len(events))
	}

	if events[0].EventType != "access" {
		t.Errorf("事件类型应为 access，实际为 %s", events[0].EventType)
	}
	if events[0].ID == "" {
		t.Error("事件 ID 不应为空")
	}
}

func TestSecurityAuditManager_MaxRecords(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: false,
		RansomwareDetection: false,
		MaxRecords:      5,
	})

	// 添加 10 条记录
	for i := 0; i < 10; i++ {
		sam.LogFileAccess(FileAccessRecord{
			ShareName: "documents",
			FilePath:  "/documents/file" + string(rune(i)),
			Username:  "testuser",
			ClientIP:  "192.168.1.100",
			Action:    "read",
		})
	}

	records := sam.GetFileAccessRecords(100, 0, nil)
	if len(records) != 5 {
		t.Errorf("应限制为 5 条记录，实际为 %d", len(records))
	}
}

func TestSecurityAuditManager_FileFilters(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: false,
		RansomwareDetection: false,
		MaxRecords:      100,
	})

	// 添加多条记录
	sam.LogFileAccess(FileAccessRecord{ShareName: "documents", Username: "user1", Action: "read", ClientIP: "192.168.1.1"})
	sam.LogFileAccess(FileAccessRecord{ShareName: "documents", Username: "user2", Action: "write", ClientIP: "192.168.1.2"})
	sam.LogFileAccess(FileAccessRecord{ShareName: "backup", Username: "user1", Action: "delete", ClientIP: "192.168.1.1"})
	sam.LogFileAccess(FileAccessRecord{ShareName: "documents", Username: "user1", Action: "delete", ClientIP: "192.168.1.1"})

	// 按用户筛选
	filters := map[string]string{"username": "user1"}
	records := sam.GetFileAccessRecords(100, 0, filters)
	if len(records) != 3 {
		t.Errorf("user1 应有 3 条记录，实际为 %d", len(records))
	}

	// 按共享名筛选
	filters = map[string]string{"share_name": "documents"}
	records = sam.GetFileAccessRecords(100, 0, filters)
	if len(records) != 3 {
		t.Errorf("documents 共享应有 3 条记录，实际为 %d", len(records))
	}

	// 按操作筛选
	filters = map[string]string{"action": "delete"}
	records = sam.GetFileAccessRecords(100, 0, filters)
	if len(records) != 2 {
		t.Errorf("delete 操作应有 2 条记录，实际为 %d", len(records))
	}

	// 按 IP 筛选
	filters = map[string]string{"client_ip": "192.168.1.1"}
	records = sam.GetFileAccessRecords(100, 0, filters)
	if len(records) != 3 {
		t.Errorf("IP 192.168.1.1 应有 3 条记录，实际为 %d", len(records))
	}
}

func TestSecurityAuditManager_EventFilters(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:    true,
		MaxRecords: 100,
	})

	// 添加多个事件
	sam.LogSecurityEvent(SecurityEvent{EventType: "access", Severity: "high", Username: "user1"})
	sam.LogSecurityEvent(SecurityEvent{EventType: "anomaly", Severity: "critical", Username: "user2"})
	sam.LogSecurityEvent(SecurityEvent{EventType: "access", Severity: "low", Username: "user1"})
	sam.LogSecurityEvent(SecurityEvent{EventType: "ransomware", Severity: "critical", Username: "user3"})

	// 按类型筛选
	filters := map[string]string{"event_type": "access"}
	events := sam.GetSecurityEvents(100, 0, filters)
	if len(events) != 2 {
		t.Errorf("access 类型应有 2 条事件，实际为 %d", len(events))
	}

	// 按严重级别筛选
	filters = map[string]string{"severity": "critical"}
	events = sam.GetSecurityEvents(100, 0, filters)
	if len(events) != 2 {
		t.Errorf("critical 级别应有 2 条事件，实际为 %d", len(events))
	}
}

func TestSecurityAuditManager_UserStats(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: false,
		RansomwareDetection: false,
		MaxRecords:      100,
	})

	// 添加多条记录
	sam.LogFileAccess(FileAccessRecord{Username: "testuser", Action: "read", FilePath: "/file1"})
	sam.LogFileAccess(FileAccessRecord{Username: "testuser", Action: "write", FilePath: "/file2"})
	sam.LogFileAccess(FileAccessRecord{Username: "testuser", Action: "delete", FilePath: "/file3"})
	sam.LogFileAccess(FileAccessRecord{Username: "otheruser", Action: "read", FilePath: "/file4"})

	stats, exists := sam.GetUserAccessStats("testuser")
	if !exists {
		t.Fatal("testuser 统计应存在")
	}

	if stats.TotalAccess != 3 {
		t.Errorf("总访问次数应为 3，实际为 %d", stats.TotalAccess)
	}
	if stats.ReadCount != 1 {
		t.Errorf("读取次数应为 1，实际为 %d", stats.ReadCount)
	}
	if stats.WriteCount != 1 {
		t.Errorf("写入次数应为 1，实际为 %d", stats.WriteCount)
	}
	if stats.DeleteCount != 1 {
		t.Errorf("删除次数应为 1，实际为 %d", stats.DeleteCount)
	}
	if len(stats.RecentFiles) != 3 {
		t.Errorf("最近文件数应为 3，实际为 %d", len(stats.RecentFiles))
	}

	// 检查不存在用户
	_, exists = sam.GetUserAccessStats("nonexistent")
	if exists {
		t.Error("不存在用户不应有统计")
	}
}

func TestSecurityAuditManager_IPStats(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: false,
		RansomwareDetection: false,
		MaxRecords:      100,
	})

	// 添加多条记录
	sam.LogFileAccess(FileAccessRecord{ClientIP: "192.168.1.100", Username: "user1", Action: "read"})
	sam.LogFileAccess(FileAccessRecord{ClientIP: "192.168.1.100", Username: "user2", Action: "write"})
	sam.LogFileAccess(FileAccessRecord{ClientIP: "192.168.1.100", Username: "user1", Action: "delete"})
	sam.LogFileAccess(FileAccessRecord{ClientIP: "192.168.1.200", Username: "user3", Action: "read"})

	stats, exists := sam.GetIPAccessStats("192.168.1.100")
	if !exists {
		t.Fatal("192.168.1.100 统计应存在")
	}

	if stats.TotalAccess != 3 {
		t.Errorf("总访问次数应为 3，实际为 %d", stats.TotalAccess)
	}
	if len(stats.ConnectedUsers) != 2 {
		t.Errorf("连接用户数应为 2，实际为 %d", len(stats.ConnectedUsers))
	}

	// 检查不存在 IP
	_, exists = sam.GetIPAccessStats("10.0.0.1")
	if exists {
		t.Error("不存在 IP 不应有统计")
	}
}

func TestSecurityAuditManager_AnomalyRules(t *testing.T) {
	sam := NewSecurityAuditManager()

	// 获取默认规则
	rules := sam.GetAnomalyRules()
	if len(rules) == 0 {
		t.Error("应有默认异常检测规则")
	}

	// 验证默认规则
	var rapidDeletionFound bool
	for _, rule := range rules {
		if rule.ID == "rapid_deletion" {
			rapidDeletionFound = true
			if !rule.Enabled {
				t.Error("rapid_deletion 规则应默认启用")
			}
			if rule.Severity != "high" {
				t.Errorf("rapid_deletion 严重级别应为 high，实际为 %s", rule.Severity)
			}
		}
	}
	if !rapidDeletionFound {
		t.Error("应存在 rapid_deletion 规则")
	}

	// 添加新规则
	newRule := AnomalyRule{
		ID:          "test_rule",
		Name:        "测试规则",
		Description: "用于测试",
		Enabled:     true,
		Severity:    "medium",
		Category:    "file_operation",
		Threshold:   5,
		TimeWindow:  time.Minute,
	}

	sam.AddAnomalyRule(newRule)

	rules = sam.GetAnomalyRules()
	var testRuleFound bool
	for _, rule := range rules {
		if rule.ID == "test_rule" {
			testRuleFound = true
		}
	}
	if !testRuleFound {
		t.Error("添加的规则应存在")
	}

	// 更新规则
	updatedRule := AnomalyRule{
		ID:          "test_rule",
		Name:        "更新测试规则",
		Description: "已更新",
		Enabled:     true,
		Severity:    "high",
		Category:    "file_operation",
		Threshold:   10,
		TimeWindow:  time.Minute * 5,
	}

	err := sam.UpdateAnomalyRule("test_rule", updatedRule)
	if err != nil {
		t.Errorf("更新规则失败: %v", err)
	}

	// 删除规则
	err = sam.DeleteAnomalyRule("test_rule")
	if err != nil {
		t.Errorf("删除规则失败: %v", err)
	}

	rules = sam.GetAnomalyRules()
	for _, rule := range rules {
		if rule.ID == "test_rule" {
			t.Error("规则应已删除")
		}
	}

	// 删除不存在规则
	err = sam.DeleteAnomalyRule("nonexistent")
	if err == nil {
		t.Error("删除不存在规则应返回错误")
	}
}

func TestSecurityAuditManager_RansomwareIndicators(t *testing.T) {
	sam := NewSecurityAuditManager()

	// 获取默认指标
	indicators := sam.GetRansomwareIndicators()
	if len(indicators) == 0 {
		t.Error("应有默认勒索软件检测指标")
	}

	// 验证默认指标
	var ransomwareExtFound bool
	for _, ind := range indicators {
		if ind.ID == "ransomware_extensions" {
			ransomwareExtFound = true
			if !ind.Enabled {
				t.Error("ransomware_extensions 指标应默认启用")
			}
			if len(ind.FileExtensions) == 0 {
				t.Error("应有可疑扩展名")
			}
			if len(ind.SuspiciousPatterns) == 0 {
				t.Error("应有可疑文件名模式")
			}
		}
	}
	if !ransomwareExtFound {
		t.Error("应存在 ransomware_extensions 指标")
	}
}

func TestSecurityAuditManager_SecurityStats(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: false,
		RansomwareDetection: false,
		MaxRecords:      100,
	})

	// 添加一些记录和事件
	for i := 0; i < 10; i++ {
		sam.LogFileAccess(FileAccessRecord{
			ShareName: "documents",
			Username:  "user" + string(rune(i)),
			Action:    "read",
			ClientIP:  "192.168.1." + string(rune(i)),
		})
	}
	sam.LogSecurityEvent(SecurityEvent{EventType: "access", Severity: "high"})
	sam.LogSecurityEvent(SecurityEvent{EventType: "anomaly", Severity: "critical"})
	sam.LogSecurityEvent(SecurityEvent{EventType: "ransomware", Severity: "critical"})

	stats := sam.GetSecurityStats()
	if stats == nil {
		t.Fatal("GetSecurityStats 返回 nil")
	}

	totalEvents, ok := stats["total_events"].(int)
	if !ok || totalEvents != 3 {
		t.Errorf("总事件数应为 3，实际为 %v", stats["total_events"])
	}

	totalAccess, ok := stats["total_file_access"].(int)
	if !ok || totalAccess != 10 {
		t.Errorf("总文件访问数应为 10，实际为 %v", stats["total_file_access"])
	}

	uniqueUsers, ok := stats["unique_users"].(int)
	if !ok || uniqueUsers == 0 {
		t.Error("应有唯一用户")
	}

	uniqueIPs, ok := stats["unique_ips"].(int)
	if !ok || uniqueIPs == 0 {
		t.Error("应有唯一 IP")
	}
}

func TestSecurityAuditManager_AlertCallback(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:    true,
		MaxRecords: 100,
	})

	alertReceived := false
	var receivedEvent SecurityEvent

	sam.SetAlertCallback(func(event SecurityEvent) {
		alertReceived = true
		receivedEvent = event
	})

	// 记录高严重级别事件（应触发告警）
	sam.LogSecurityEvent(SecurityEvent{
		EventType: "anomaly",
		Severity:  "high",
		Username:  "testuser",
	})

	// 等待回调执行
	time.Sleep(100 * time.Millisecond)

	if !alertReceived {
		t.Error("高严重级别事件应触发告警回调")
	}
	if receivedEvent.EventType != "anomaly" {
		t.Errorf("接收的事件类型应为 anomaly，实际为 %s", receivedEvent.EventType)
	}
}

func TestSecurityAuditManager_Whitelist(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: true,
		RansomwareDetection: false,
		MaxRecords:      100,
		WhitelistedIPs:  []string{"192.168.1.1"},
		WhitelistedUsers: []string{"admin"},
	})

	// 白名单用户的大量删除（不应触发异常）
	for i := 0; i < 20; i++ {
		sam.LogFileAccess(FileAccessRecord{
			Username:  "admin",
			ClientIP:  "192.168.1.1",
			Action:    "delete",
			FilePath:  "/file" + string(rune(i)),
		})
	}

	// 验证无异常事件（白名单用户）
	events := sam.GetSecurityEvents(100, 0, map[string]string{"username": "admin"})
	for _, event := range events {
		if event.EventType == "anomaly" {
			t.Error("白名单用户不应触发异常告警")
		}
	}
}

func TestSecurityAuditManager_CleanupOldRecords(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: false,
		RansomwareDetection: false,
		MaxRecords:      100,
		RetentionDays:   1,
	})

	// 添加旧记录
	oldRecord := FileAccessRecord{
		ShareName: "documents",
		FilePath:  "/documents/old.pdf",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		Action:    "read",
	}
	oldRecord.Timestamp = time.Now().AddDate(0, 0, -2) // 2 天前
	sam.LogFileAccess(oldRecord)

	// 添加新记录
	sam.LogFileAccess(FileAccessRecord{
		ShareName: "documents",
		FilePath:  "/documents/new.pdf",
		Username:  "testuser",
		ClientIP:  "192.168.1.100",
		Action:    "read",
	})

	// 清理旧记录
	sam.CleanupOldRecords()

	records := sam.GetFileAccessRecords(100, 0, nil)
	if len(records) != 1 {
		t.Errorf("清理后应有 1 条记录，实际为 %d", len(records))
	}

	if len(records) > 0 && records[0].FilePath != "/documents/new.pdf" {
		t.Errorf("保留的记录应为新记录，实际为 %s", records[0].FilePath)
	}
}

func TestSecurityAuditManager_ResetRecentStats(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: false,
		RansomwareDetection: false,
		MaxRecords:      100,
	})

	// 添加删除记录
	for i := 0; i < 10; i++ {
		sam.LogFileAccess(FileAccessRecord{
			Username: "testuser",
			ClientIP: "192.168.1.100",
			Action:   "delete",
		})
	}

	stats, _ := sam.GetUserAccessStats("testuser")
	if stats.RecentDeleteCount != 10 {
		t.Errorf("近期删除次数应为 10，实际为 %d", stats.RecentDeleteCount)
	}

	// 重置统计
	sam.ResetRecentStats()

	stats, _ = sam.GetUserAccessStats("testuser")
	if stats.RecentDeleteCount != 0 {
		t.Errorf("重置后近期删除次数应为 0，实际为 %d", stats.RecentDeleteCount)
	}
}

func TestGetFileExtension(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/documents/report.pdf", ".pdf"},
		{"/documents/report.encrypted", ".encrypted"},
		{"/documents/README_FOR_DECRYPT.txt", ".txt"},
		{"/documents/file", ""},
		{"/documents/.hidden", ".hidden"},
	}

	for _, tt := range tests {
		ext := getFileExtension(tt.path)
		if ext != tt.expected {
			t.Errorf("getFileExtension(%q) = %q，期望 %q", tt.path, ext, tt.expected)
		}
	}
}

func TestGenerateEventID(t *testing.T) {
	id1 := generateEventID()
	id2 := generateEventID()

	if id1 == "" {
		t.Error("事件 ID 不应为空")
	}
	if id1 == id2 {
		t.Error("两次生成的 ID 应不同")
	}
	if !startsWith(id1, "evt-") {
		t.Errorf("事件 ID 应以 evt- 开头，实际为 %s", id1)
	}
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

// 测试异常检测触发
func TestSecurityAuditManager_AnomalyDetection_RapidDeletion(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: true,
		RansomwareDetection: false,
		MaxRecords:      1000,
		WhitelistedIPs:  []string{},
		WhitelistedUsers: []string{},
	})

	alertReceived := false
	sam.SetAlertCallback(func(event SecurityEvent) {
		alertReceived = true
	})

	// 快速添加大量删除记录
	for i := 0; i < 15; i++ {
		sam.LogFileAccess(FileAccessRecord{
			Username:  "attacker",
			ClientIP:  "10.0.0.1",
			Action:    "delete",
			FilePath:  "/documents/file" + string(rune(i)),
			ShareName: "documents",
		})
	}

	time.Sleep(200 * time.Millisecond)

	events := sam.GetSecurityEvents(100, 0, map[string]string{"event_type": "anomaly"})
	if len(events) == 0 {
		t.Error("快速删除应触发异常告警")
	}

	// 验证告警详情
	for _, event := range events {
		if event.Severity != "high" {
			t.Errorf("快速删除告警严重级别应为 high，实际为 %s", event.Severity)
		}
		if event.DetectedBy != "anomaly_detector" {
			t.Errorf("检测来源应为 anomaly_detector，实际为 %s", event.DetectedBy)
		}
	}
}

// 测试勒索软件检测
func TestSecurityAuditManager_RansomwareDetection_SuspiciousExtension(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: false,
		RansomwareDetection: true,
		MaxRecords:      100,
		WhitelistedIPs:  []string{},
		WhitelistedUsers: []string{},
	})

	alertReceived := false
	sam.SetAlertCallback(func(event SecurityEvent) {
		alertReceived = true
	})

	// 记录可疑扩展名文件写入
	sam.LogFileAccess(FileAccessRecord{
		Username:  "attacker",
		ClientIP:  "10.0.0.1",
		Action:    "write",
		FilePath:  "/documents/report.encrypted",
		ShareName: "documents",
	})

	time.Sleep(200 * time.Millisecond)

	events := sam.GetSecurityEvents(100, 0, map[string]string{"event_type": "ransomware"})
	if len(events) == 0 {
		t.Error("可疑扩展名应触发勒索软件告警")
	}

	for _, event := range events {
		if event.Severity != "critical" {
			t.Errorf("勒索软件告警严重级别应为 critical，实际为 %s", event.Severity)
		}
		if event.DetectedBy != "ransomware_detector" {
			t.Errorf("检测来源应为 ransomware_detector，实际为 %s", event.DetectedBy)
		}
	}
}

// 测试勒索软件检测 - 可疑文件名模式
func TestSecurityAuditManager_RansomwareDetection_SuspiciousPattern(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: false,
		RansomwareDetection: true,
		MaxRecords:      100,
		WhitelistedIPs:  []string{},
		WhitelistedUsers: []string{},
	})

	// 记录可疑文件名模式
	sam.LogFileAccess(FileAccessRecord{
		Username:  "attacker",
		ClientIP:  "10.0.0.1",
		Action:    "create",
		FilePath:  "/documents/README_FOR_DECRYPT.txt",
		ShareName: "documents",
	})

	time.Sleep(200 * time.Millisecond)

	events := sam.GetSecurityEvents(100, 0, map[string]string{"event_type": "ransomware"})
	if len(events) == 0 {
		t.Error("可疑文件名模式应触发勒索软件告警")
	}
}

// 测试分页功能
func TestSecurityAuditManager_Pagination(t *testing.T) {
	sam := NewSecurityAuditManager()
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: false,
		RansomwareDetection: false,
		MaxRecords:      100,
	})

	// 添加 20 条记录
	for i := 0; i < 20; i++ {
		sam.LogFileAccess(FileAccessRecord{
			ShareName: "documents",
			FilePath:  "/documents/file" + string(rune(i)),
			Username:  "testuser",
			ClientIP:  "192.168.1.100",
			Action:    "read",
		})
	}

	// 测试第一页
	page1 := sam.GetFileAccessRecords(5, 0, nil)
	if len(page1) != 5 {
		t.Errorf("第一页应有 5 条记录，实际为 %d", len(page1))
	}

	// 测试第二页
	page2 := sam.GetFileAccessRecords(5, 5, nil)
	if len(page2) != 5 {
		t.Errorf("第二页应有 5 条记录，实际为 %d", len(page2))
	}

	// 测试超出范围的偏移
	pageInvalid := sam.GetFileAccessRecords(5, 100, nil)
	if len(pageInvalid) != 0 {
		t.Errorf("超出范围应返回空，实际为 %d", len(pageInvalid))
	}
}

// 测试事件类型的详细信息
func TestSecurityAuditManager_EventDetails(t *testing.T) {
	sam := NewSecurityAuditManager()

	// 触发异常
	sam.SetConfig(SecurityAuditConfig{
		Enabled:         true,
		FileAccessAudit: true,
		AnomalyDetection: true,
		RansomwareDetection: false,
		MaxRecords:      1000,
	})

	for i := 0; i < 15; i++ {
		sam.LogFileAccess(FileAccessRecord{
			Username:  "testuser",
			ClientIP:  "192.168.1.100",
			Action:    "delete",
			FilePath:  "/file" + string(rune(i)),
			ShareName: "documents",
		})
	}

	events := sam.GetSecurityEvents(100, 0, map[string]string{"event_type": "anomaly"})
	if len(events) == 0 {
		t.Fatal("应有异常事件")
	}

	// 验证详细信息
	details := events[0].Details
	if details == nil {
		t.Fatal("异常事件应有详细信息")
	}

	ruleID, ok := details["rule_id"].(string)
	if !ok || ruleID == "" {
		t.Error("详细信息应包含 rule_id")
	}

	ruleName, ok := details["rule_name"].(string)
	if !ok || ruleName == "" {
		t.Error("详细信息应包含 rule_name")
	}

	count, ok := details["count"].(int)
	if !ok || count == 0 {
		t.Error("详细信息应包含 count")
	}
}