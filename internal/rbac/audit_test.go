package rbac

import (
	"sync"
	"testing"
	"time"
)

// ========== 审计日志测试 ==========

func TestNewRBACAuditLogger(t *testing.T) {
	logger := NewRBACAuditLogger(100, nil)
	defer logger.Close()

	if logger == nil {
		t.Fatal("logger is nil")
	}
}

func TestRBACAuditLogger_Log(t *testing.T) {
	var receivedEvent *AuditEvent
	var mu sync.Mutex

	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		mu.Lock()
		receivedEvent = event
		mu.Unlock()
	})
	defer logger.Close()

	event := &AuditEvent{
		Level:    AuditLevelInfo,
		Category: AuditCategoryPermission,
		Event:    "test_event",
		UserID:   "user1",
	}

	logger.Log(event)

	// 等待异步处理
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if receivedEvent == nil {
		t.Error("event not received")
		return
	}

	if receivedEvent.UserID != "user1" {
		t.Errorf("UserID = %s, want user1", receivedEvent.UserID)
	}

	// 验证时间戳自动设置
	if receivedEvent.Timestamp.IsZero() {
		t.Error("Timestamp should be set automatically")
	}
}

func TestRBACAuditLogger_LogSync(t *testing.T) {
	var receivedEvent *AuditEvent

	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	event := &AuditEvent{
		Level:    AuditLevelInfo,
		Category: AuditCategoryPermission,
		Event:    "sync_event",
		UserID:   "user2",
	}

	logger.LogSync(event)

	if receivedEvent == nil {
		t.Fatal("event not received")
	}

	if receivedEvent.UserID != "user2" {
		t.Errorf("UserID = %s, want user2", receivedEvent.UserID)
	}
}

func TestRBACAuditLogger_LogPermissionGrant(t *testing.T) {
	var receivedEvent *AuditEvent

	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	logger.LogSync(&AuditEvent{
		Timestamp:  time.Now(),
		Level:      AuditLevelInfo,
		Category:   AuditCategoryPermission,
		Event:      "permission_grant",
		UserID:     "admin",
		Username:   "管理员",
		TargetID:   "user1",
		Permission: "storage:write",
		Result:     "success",
	})

	if receivedEvent == nil {
		t.Fatal("event not received")
	}

	if receivedEvent.Event != "permission_grant" {
		t.Errorf("Event = %s, want permission_grant", receivedEvent.Event)
	}

	if receivedEvent.Permission != "storage:write" {
		t.Errorf("Permission = %s, want storage:write", receivedEvent.Permission)
	}

	if receivedEvent.Category != AuditCategoryPermission {
		t.Errorf("Category = %s, want permission", receivedEvent.Category)
	}
}

func TestRBACAuditLogger_LogPermissionRevoke(t *testing.T) {
	var receivedEvent *AuditEvent

	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	logger.LogSync(&AuditEvent{
		Level:      AuditLevelInfo,
		Category:   AuditCategoryPermission,
		Event:      "permission_revoke",
		Permission: "storage:write",
	})

	if receivedEvent == nil {
		t.Fatal("event not received")
	}

	if receivedEvent.Event != "permission_revoke" {
		t.Errorf("Event = %s, want permission_revoke", receivedEvent.Event)
	}
}

func TestRBACAuditLogger_LogRoleChange(t *testing.T) {
	var receivedEvent *AuditEvent

	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	logger.LogSync(&AuditEvent{
		Level:    AuditLevelInfo,
		Category: AuditCategoryRole,
		Event:    "role_change",
		UserID:   "admin",
		Username: "管理员",
		TargetID: "user1",
		OldValue: "guest",
		NewValue: "operator",
		Result:   "success",
	})

	if receivedEvent == nil {
		t.Fatal("event not received")
	}

	if receivedEvent.Event != "role_change" {
		t.Errorf("Event = %s, want role_change", receivedEvent.Event)
	}

	if receivedEvent.OldValue != "guest" {
		t.Errorf("OldValue = %v, want guest", receivedEvent.OldValue)
	}

	if receivedEvent.NewValue != "operator" {
		t.Errorf("NewValue = %v, want operator", receivedEvent.NewValue)
	}
}

func TestRBACAuditLogger_LogRoleChange_Admin(t *testing.T) {
	var receivedEvent *AuditEvent

	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	// 提升为管理员应该是警告级别
	logger.LogSync(&AuditEvent{
		Level:    AuditLevelWarning,
		Category: AuditCategoryRole,
		Event:    "role_change",
		OldValue: "operator",
		NewValue: "admin",
	})

	if receivedEvent == nil {
		t.Fatal("event not received")
	}

	if receivedEvent.Level != AuditLevelWarning {
		t.Errorf("Level = %s, want warning for admin role change", receivedEvent.Level)
	}
}

func TestRBACAuditLogger_LogPolicyCreate(t *testing.T) {
	var receivedEvent *AuditEvent

	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	logger.LogPolicyCreate("admin", "管理员", "deny-storage-admin")

	// 等待异步处理
	time.Sleep(100 * time.Millisecond)

	if receivedEvent == nil {
		t.Fatal("event not received")
	}

	if receivedEvent.Event != "policy_create" {
		t.Errorf("Event = %s, want policy_create", receivedEvent.Event)
	}

	if receivedEvent.TargetName != "deny-storage-admin" {
		t.Errorf("TargetName = %s, want deny-storage-admin", receivedEvent.TargetName)
	}
}

func TestRBACAuditLogger_LogPolicyUpdate(t *testing.T) {
	var receivedEvent *AuditEvent

	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	changes := map[string]interface{}{"enabled": false}
	logger.LogPolicyUpdate("admin", "管理员", "policy-123", changes)

	// 等待异步处理
	time.Sleep(100 * time.Millisecond)

	if receivedEvent == nil {
		t.Fatal("event not received")
	}

	if receivedEvent.Event != "policy_update" {
		t.Errorf("Event = %s, want policy_update", receivedEvent.Event)
	}
}

func TestRBACAuditLogger_LogPolicyDelete(t *testing.T) {
	var receivedEvent *AuditEvent

	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	logger.LogPolicyDelete("admin", "管理员", "policy-123")

	// 等待异步处理
	time.Sleep(100 * time.Millisecond)

	if receivedEvent == nil {
		t.Fatal("event not received")
	}

	// 删除策略应该是警告级别
	if receivedEvent.Level != AuditLevelWarning {
		t.Errorf("Level = %s, want warning for policy delete", receivedEvent.Level)
	}
}

func TestRBACAuditLogger_LogAccessCheck(t *testing.T) {
	tests := []struct {
		name       string
		allowed    bool
		wantLevel  AuditLevel
		wantResult string
	}{
		{"allowed", true, AuditLevelInfo, "success"},
		{"denied", false, AuditLevelWarning, "denied"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedEvent *AuditEvent

			logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
				receivedEvent = event
			})
			defer logger.Close()

			logger.LogAccessCheck("user1", "storage", "write", tt.allowed, "test reason")

			// 等待异步处理
			time.Sleep(100 * time.Millisecond)

			if receivedEvent == nil {
				t.Fatal("event not received")
			}

			if receivedEvent.Level != tt.wantLevel {
				t.Errorf("Level = %s, want %s", receivedEvent.Level, tt.wantLevel)
			}

			if receivedEvent.Result != tt.wantResult {
				t.Errorf("Result = %s, want %s", receivedEvent.Result, tt.wantResult)
			}
		})
	}
}

func TestRBACAuditLogger_LogGroupPermissionChange(t *testing.T) {
	var receivedEvent *AuditEvent

	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	logger.LogGroupPermissionChange("admin", "管理员", "group1", []string{"storage:read", "storage:write"})

	// 等待异步处理
	time.Sleep(100 * time.Millisecond)

	if receivedEvent == nil {
		t.Fatal("event not received")
	}

	if receivedEvent.Event != "group_permission_change" {
		t.Errorf("Event = %s, want group_permission_change", receivedEvent.Event)
	}

	if receivedEvent.TargetID != "group1" {
		t.Errorf("TargetID = %s, want group1", receivedEvent.TargetID)
	}
}

func TestRBACAuditLogger_LogShareACLChange(t *testing.T) {
	var receivedEvent *AuditEvent

	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	changes := map[string]interface{}{"access": "read"}
	logger.LogShareACLChange("admin", "管理员", "share1", changes)

	// 等待异步处理
	time.Sleep(100 * time.Millisecond)

	if receivedEvent == nil {
		t.Fatal("event not received")
	}

	if receivedEvent.Event != "share_acl_change" {
		t.Errorf("Event = %s, want share_acl_change", receivedEvent.Event)
	}

	if receivedEvent.TargetName != "share1" {
		t.Errorf("TargetName = %s, want share1", receivedEvent.TargetName)
	}
}

// ========== 审计中间件测试 ==========

func TestNewAuditMiddleware(t *testing.T) {
	logger := NewRBACAuditLogger(100, nil)
	defer logger.Close()

	am := NewAuditMiddleware(logger)
	if am == nil {
		t.Fatal("AuditMiddleware is nil")
	}
}

func TestAuditMiddleware_WrapManager(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	logger := NewRBACAuditLogger(100, nil)
	defer logger.Close()

	am := NewAuditMiddleware(logger)
	auditedMgr := am.WrapManager(m)

	if auditedMgr == nil {
		t.Fatal("AuditedManager is nil")
	}
}

func TestAuditedManager_SetUserRole(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	var receivedEvent *AuditEvent
	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	am := NewAuditMiddleware(logger)
	auditedMgr := am.WrapManager(m)

	err := auditedMgr.SetUserRole("admin", "管理员", "user1", "testuser", RoleOperator)
	if err != nil {
		t.Fatalf("SetUserRole failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if receivedEvent == nil {
		t.Fatal("audit event not received")
	}

	if receivedEvent.Event != "role_change" {
		t.Errorf("Event = %s, want role_change", receivedEvent.Event)
	}
}

func TestAuditedManager_GrantPermissionWithAudit(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	var receivedEvent *AuditEvent
	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	am := NewAuditMiddleware(logger)
	auditedMgr := am.WrapManager(m)

	// 先设置用户角色
	_ = auditedMgr.Manager.SetUserRole("user1", "testuser", RoleReadOnly)

	err := auditedMgr.GrantPermissionWithAudit("admin", "管理员", "user1", "testuser", "storage:write")
	if err != nil {
		t.Fatalf("GrantPermissionWithAudit failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if receivedEvent == nil {
		t.Fatal("audit event not received")
	}

	if receivedEvent.Event != "permission_grant" {
		t.Errorf("Event = %s, want permission_grant", receivedEvent.Event)
	}
}

func TestAuditedManager_RevokePermissionWithAudit(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	var receivedEvent *AuditEvent
	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	am := NewAuditMiddleware(logger)
	auditedMgr := am.WrapManager(m)

	// 先设置用户角色并授予权限
	_ = auditedMgr.Manager.SetUserRole("user1", "testuser", RoleReadOnly)
	_ = auditedMgr.Manager.GrantPermission("user1", "testuser", "storage:write")

	err := auditedMgr.RevokePermissionWithAudit("admin", "管理员", "user1", "storage:write")
	if err != nil {
		t.Fatalf("RevokePermissionWithAudit failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if receivedEvent == nil {
		t.Fatal("audit event not received")
	}

	if receivedEvent.Event != "permission_revoke" {
		t.Errorf("Event = %s, want permission_revoke", receivedEvent.Event)
	}
}

func TestAuditedManager_CreatePolicyWithAudit(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	var receivedEvent *AuditEvent
	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	am := NewAuditMiddleware(logger)
	auditedMgr := am.WrapManager(m)

	policy, err := auditedMgr.CreatePolicyWithAudit(
		"admin", "管理员",
		"test-policy", "测试策略",
		EffectAllow,
		[]string{"user1"},
		[]string{"storage"},
		[]string{"read"},
		50,
	)
	if err != nil {
		t.Fatalf("CreatePolicyWithAudit failed: %v", err)
	}

	if policy == nil {
		t.Fatal("policy is nil")
	}

	time.Sleep(100 * time.Millisecond)

	if receivedEvent == nil {
		t.Fatal("audit event not received")
	}

	if receivedEvent.Event != "policy_create" {
		t.Errorf("Event = %s, want policy_create", receivedEvent.Event)
	}
}

func TestAuditedManager_DeletePolicyWithAudit(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	var receivedEvent *AuditEvent
	logger := NewRBACAuditLogger(100, func(event *AuditEvent) {
		receivedEvent = event
	})
	defer logger.Close()

	am := NewAuditMiddleware(logger)
	auditedMgr := am.WrapManager(m)

	// 先创建策略
	policy, _ := auditedMgr.Manager.CreatePolicy(
		"test-policy", "测试策略",
		EffectAllow,
		[]string{"user1"},
		[]string{"storage"},
		[]string{"read"},
		50,
	)

	err := auditedMgr.DeletePolicyWithAudit("admin", "管理员", policy.ID)
	if err != nil {
		t.Fatalf("DeletePolicyWithAudit failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if receivedEvent == nil {
		t.Fatal("audit event not received")
	}

	if receivedEvent.Event != "policy_delete" {
		t.Errorf("Event = %s, want policy_delete", receivedEvent.Event)
	}
}

// ========== 审计统计测试 ==========

func TestNewAuditStatsCollector(t *testing.T) {
	collector := NewAuditStatsCollector(1000)
	if collector == nil {
		t.Fatal("collector is nil")
	}
}

func TestAuditStatsCollector_Record(t *testing.T) {
	collector := NewAuditStatsCollector(1000)

	event := &AuditEvent{
		Level:    AuditLevelInfo,
		Category: AuditCategoryPermission,
		Event:    "permission_grant",
		UserID:   "user1",
		Username: "testuser",
	}

	collector.Record(event)

	stats := collector.GetStats()
	if stats.TotalEvents != 1 {
		t.Errorf("TotalEvents = %d, want 1", stats.TotalEvents)
	}

	if stats.PermissionGrants != 1 {
		t.Errorf("PermissionGrants = %d, want 1", stats.PermissionGrants)
	}
}

func TestAuditStatsCollector_MultipleEvents(t *testing.T) {
	collector := NewAuditStatsCollector(1000)

	events := []*AuditEvent{
		{Level: AuditLevelInfo, Category: AuditCategoryPermission, Event: "permission_grant", UserID: "user1"},
		{Level: AuditLevelInfo, Category: AuditCategoryPermission, Event: "permission_revoke", UserID: "user1"},
		{Level: AuditLevelWarning, Category: AuditCategoryRole, Event: "role_change", UserID: "admin"},
		{Level: AuditLevelWarning, Category: AuditCategoryAccess, Event: "access_check", Result: "denied"},
	}

	for _, event := range events {
		collector.Record(event)
	}

	stats := collector.GetStats()
	if stats.TotalEvents != 4 {
		t.Errorf("TotalEvents = %d, want 4", stats.TotalEvents)
	}

	if stats.PermissionGrants != 1 {
		t.Errorf("PermissionGrants = %d, want 1", stats.PermissionGrants)
	}

	if stats.PermissionRevokes != 1 {
		t.Errorf("PermissionRevokes = %d, want 1", stats.PermissionRevokes)
	}

	if stats.RoleChanges != 1 {
		t.Errorf("RoleChanges = %d, want 1", stats.RoleChanges)
	}

	if stats.AccessDenials != 1 {
		t.Errorf("AccessDenials = %d, want 1", stats.AccessDenials)
	}

	// 检查分类统计
	if stats.ByCategory[string(AuditCategoryPermission)] != 2 {
		t.Errorf("ByCategory[permission] = %d, want 2", stats.ByCategory[string(AuditCategoryPermission)])
	}

	// 检查级别统计
	if stats.ByLevel[string(AuditLevelInfo)] != 2 {
		t.Errorf("ByLevel[info] = %d, want 2", stats.ByLevel[string(AuditLevelInfo)])
	}
}

func TestAuditStatsCollector_MaxEvents(t *testing.T) {
	collector := NewAuditStatsCollector(10)

	// 添加超过最大限制的事件
	for i := 0; i < 20; i++ {
		collector.Record(&AuditEvent{
			Level:    AuditLevelInfo,
			Category: AuditCategoryPermission,
			Event:    "test",
			UserID:   "user1",
		})
	}

	stats := collector.GetStats()
	if stats.TotalEvents > 10 {
		t.Errorf("TotalEvents = %d, should be <= 10", stats.TotalEvents)
	}
}

func TestAuditStatsCollector_TopOperators(t *testing.T) {
	collector := NewAuditStatsCollector(1000)

	// user1 有 5 次操作
	for i := 0; i < 5; i++ {
		collector.Record(&AuditEvent{
			Level:    AuditLevelInfo,
			Category: AuditCategoryPermission,
			Event:    "test",
			UserID:   "user1",
			Username: "user1",
		})
	}

	// user2 有 3 次操作
	for i := 0; i < 3; i++ {
		collector.Record(&AuditEvent{
			Level:    AuditLevelInfo,
			Category: AuditCategoryPermission,
			Event:    "test",
			UserID:   "user2",
			Username: "user2",
		})
	}

	stats := collector.GetStats()
	if len(stats.TopOperators) != 2 {
		t.Errorf("TopOperators count = %d, want 2", len(stats.TopOperators))
	}
}

// ========== AuditEvent 结构测试 ==========

func TestAuditEvent_Levels(t *testing.T) {
	levels := []AuditLevel{AuditLevelInfo, AuditLevelWarning, AuditLevelError}

	for _, level := range levels {
		event := &AuditEvent{Level: level}
		if event.Level != level {
			t.Errorf("Level = %s, want %s", event.Level, level)
		}
	}
}

func TestAuditEvent_Categories(t *testing.T) {
	categories := []AuditCategory{
		AuditCategoryPermission,
		AuditCategoryRole,
		AuditCategoryPolicy,
		AuditCategoryAccess,
		AuditCategoryGroup,
		AuditCategoryShare,
	}

	for _, category := range categories {
		event := &AuditEvent{Category: category}
		if event.Category != category {
			t.Errorf("Category = %s, want %s", event.Category, category)
		}
	}
}
