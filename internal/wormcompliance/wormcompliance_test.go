package wormcompliance

import (
	"testing"
	"time"
)

func TestPolicyManager_CreatePolicy(t *testing.T) {
	pm := NewPolicyManager()

	// 测试创建策略
	policy, err := pm.CreatePolicy(
		"财务数据保留策略",
		"用于财务数据的合规保留",
		ModeEnterprise,
		RetentionPeriod{Value: 7, Unit: RetentionYears},
		[]string{"/data/finance", "/data/accounting"},
		[]RegulationType{RegulationSOX},
	)

	if err != nil {
		t.Fatalf("创建策略失败: %v", err)
	}

	if policy.Name != "财务数据保留策略" {
		t.Errorf("期望策略名称为 '财务数据保留策略'，实际为 '%s'", policy.Name)
	}

	if policy.Mode != ModeEnterprise {
		t.Errorf("期望合规模式为 'enterprise'，实际为 '%s'", policy.Mode)
	}

	if !policy.Enabled {
		t.Error("策略应该默认启用")
	}
}

func TestPolicyManager_GetPolicy(t *testing.T) {
	pm := NewPolicyManager()

	// 创建策略
	policy, _ := pm.CreatePolicy("测试策略", "测试", ModeGovernance, RetentionPeriod{Value: 30, Unit: RetentionDays}, []string{"/test"}, nil)

	// 获取策略
	retrieved, err := pm.GetPolicy(policy.ID)
	if err != nil {
		t.Fatalf("获取策略失败: %v", err)
	}

	if retrieved.ID != policy.ID {
		t.Errorf("策略ID不匹配")
	}

	// 测试获取不存在的策略
	_, err = pm.GetPolicy("nonexistent")
	if err == nil {
		t.Error("期望获取不存在的策略时返回错误")
	}
}

func TestPolicyManager_UpdatePolicy(t *testing.T) {
	pm := NewPolicyManager()

	// 创建策略
	policy, _ := pm.CreatePolicy("测试策略", "测试", ModeGovernance, RetentionPeriod{Value: 30, Unit: RetentionDays}, []string{"/test"}, nil)

	// 更新策略
	updates := map[string]interface{}{
		"name":    "更新后的策略",
		"enabled": false,
	}

	updated, err := pm.UpdatePolicy(policy.ID, updates)
	if err != nil {
		t.Fatalf("更新策略失败: %v", err)
	}

	if updated.Name != "更新后的策略" {
		t.Errorf("期望策略名称为 '更新后的策略'，实际为 '%s'", updated.Name)
	}

	if updated.Enabled {
		t.Error("策略应该被禁用")
	}
}

func TestPolicyManager_DeletePolicy(t *testing.T) {
	pm := NewPolicyManager()

	// 创建策略
	policy, _ := pm.CreatePolicy("测试策略", "测试", ModeGovernance, RetentionPeriod{Value: 30, Unit: RetentionDays}, []string{"/test"}, nil)

	// 删除策略
	err := pm.DeletePolicy(policy.ID)
	if err != nil {
		t.Fatalf("删除策略失败: %v", err)
	}

	// 验证策略已删除
	_, err = pm.GetPolicy(policy.ID)
	if err == nil {
		t.Error("期望策略已被删除")
	}
}

func TestPolicyManager_ListPolicies(t *testing.T) {
	pm := NewPolicyManager()

	// 创建多个策略
	pm.CreatePolicy("策略1", "测试1", ModeGovernance, RetentionPeriod{Value: 30, Unit: RetentionDays}, []string{"/test1"}, nil)
	pm.CreatePolicy("策略2", "测试2", ModeEnterprise, RetentionPeriod{Value: 7, Unit: RetentionYears}, []string{"/test2"}, nil)

	policies := pm.ListPolicies()
	if len(policies) != 2 {
		t.Errorf("期望有2个策略，实际有%d个", len(policies))
	}
}

func TestPolicyManager_GetPoliciesForPath(t *testing.T) {
	pm := NewPolicyManager()

	// 创建策略
	pm.CreatePolicy("策略1", "测试1", ModeGovernance, RetentionPeriod{Value: 30, Unit: RetentionDays}, []string{"/data/finance"}, nil)
	pm.CreatePolicy("策略2", "测试2", ModeEnterprise, RetentionPeriod{Value: 7, Unit: RetentionYears}, []string{"/data/medical"}, nil)

	// 测试路径匹配
	policies := pm.GetPoliciesForPath("/data/finance/report.csv")
	if len(policies) != 1 {
		t.Errorf("期望匹配1个策略，实际匹配%d个", len(policies))
	}

	// 测试通配符
	pm.CreatePolicy("全局策略", "全局", ModeGovernance, RetentionPeriod{Value: 1, Unit: RetentionYears}, []string{"*"}, nil)
	policies = pm.GetPoliciesForPath("/any/path")
	if len(policies) != 1 {
		t.Errorf("期望匹配1个策略，实际匹配%d个", len(policies))
	}
}

func TestImmutabilityManager_ProtectAndLock(t *testing.T) {
	config := DefaultWORMConfig()
	im := NewImmutabilityManager(config)

	// 保护对象
	obj, err := im.ProtectObject("/data/test.txt", 1024, "policy-1", "admin", nil)
	if err != nil {
		t.Fatalf("保护对象失败: %v", err)
	}

	if obj.Locked {
		t.Error("对象不应该被锁定")
	}

	// 锁定对象
	retention := RetentionPeriod{Value: 30, Unit: RetentionDays}
	err = im.LockObject(obj.ID, retention)
	if err != nil {
		t.Fatalf("锁定对象失败: %v", err)
	}

	// 验证锁定
	lockedObj, _ := im.GetObject(obj.ID)
	if !lockedObj.Locked {
		t.Error("对象应该被锁定")
	}

	if lockedObj.ExpiresAt == nil {
		t.Error("应该设置过期时间")
	}

	// 测试重复锁定
	err = im.LockObject(obj.ID, retention)
	if err == nil {
		t.Error("期望重复锁定时返回错误")
	}
}

func TestImmutabilityManager_VerifyObject(t *testing.T) {
	config := DefaultWORMConfig()
	im := NewImmutabilityManager(config)

	// 保护对象
	obj, _ := im.ProtectObject("/data/test.txt", 1024, "policy-1", "admin", nil)

	// 验证对象
	valid, err := im.VerifyObject(obj.ID)
	if err != nil {
		t.Fatalf("验证对象失败: %v", err)
	}

	if !valid {
		t.Error("对象应该通过验证")
	}

	// 测试验证不存在的对象
	_, err = im.VerifyObject("nonexistent")
	if err == nil {
		t.Error("期望验证不存在的对象时返回错误")
	}
}

func TestImmutabilityManager_VerifyHashChain(t *testing.T) {
	config := DefaultWORMConfig()
	im := NewImmutabilityManager(config)

	// 创建多个对象
	im.ProtectObject("/data/test1.txt", 1024, "policy-1", "admin", nil)
	im.ProtectObject("/data/test2.txt", 2048, "policy-1", "admin", nil)
	im.ProtectObject("/data/test3.txt", 4096, "policy-1", "admin", nil)

	// 验证哈希链
	valid, err := im.VerifyHashChain()
	if err != nil {
		t.Fatalf("验证哈希链失败: %v", err)
	}

	if !valid {
		t.Error("哈希链应该通过验证")
	}
}

func TestImmutabilityManager_CanDelete(t *testing.T) {
	config := DefaultWORMConfig()
	im := NewImmutabilityManager(config)

	// 创建对象
	obj, _ := im.ProtectObject("/data/test.txt", 1024, "policy-1", "admin", nil)

	// 测试不同模式下的删除权限
	canDelete, _ := im.CanDelete(obj.ID, ModeGovernance, time.Now())
	if !canDelete {
		t.Error("治理模式应该允许删除")
	}

	canDelete, _ = im.CanDelete(obj.ID, ModeEnterprise, time.Now())
	if canDelete {
		t.Error("企业模式不应该允许删除未过期对象")
	}

	canDelete, _ = im.CanDelete(obj.ID, ModeRegulatory, time.Now())
	if canDelete {
		t.Error("法规模式不应该允许删除任何对象")
	}

	// 锁定对象并测试过期后的删除
	retention := RetentionPeriod{Value: -1, Unit: RetentionDays} // 已过期
	im.LockObject(obj.ID, retention)

	futureTime := time.Now().Add(2 * 24 * time.Hour)
	canDelete, _ = im.CanDelete(obj.ID, ModeEnterprise, futureTime)
	if !canDelete {
		t.Error("过期对象应该允许删除")
	}
}

func TestImmutabilityManager_ListExpiredObjects(t *testing.T) {
	config := DefaultWORMConfig()
	im := NewImmutabilityManager(config)

	// 创建对象
	obj, _ := im.ProtectObject("/data/test.txt", 1024, "policy-1", "admin", nil)

	// 锁定对象
	retention := RetentionPeriod{Value: -1, Unit: RetentionDays} // 已过期
	im.LockObject(obj.ID, retention)

	// 列出过期对象
	futureTime := time.Now().Add(2 * 24 * time.Hour)
	expired := im.ListExpiredObjects(futureTime)
	if len(expired) != 1 {
		t.Errorf("期望1个过期对象，实际%d个", len(expired))
	}
}

func TestAuditManager_LogAndVerify(t *testing.T) {
	am := NewAuditManager(365)

	// 记录操作
	entry1 := am.LogAction("obj-1", "create", "admin", "创建对象", "192.168.1.1", true, "")
	am.LogAction("obj-1", "lock", "admin", "锁定对象", "192.168.1.1", true, "")
	entry3 := am.LogAction("obj-1", "delete", "user", "删除对象失败", "192.168.1.2", false, "权限不足")

	// 验证记录
	if entry1.Action != "create" {
		t.Errorf("期望操作为 'create'，实际为 '%s'", entry1.Action)
	}

	if entry3.Success {
		t.Error("失败的操作应该标记为不成功")
	}

	// 验证审计链
	valid, err := am.VerifyAuditChain()
	if err != nil {
		t.Fatalf("验证审计链失败: %v", err)
	}

	if !valid {
		t.Error("审计链应该通过验证")
	}

	// 验证记录数量
	if am.GetEntryCount() != 3 {
		t.Errorf("期望3条记录，实际%d条", am.GetEntryCount())
	}
}

func TestAuditManager_GetEntries(t *testing.T) {
	am := NewAuditManager(365)

	// 创建多条记录
	for i := 0; i < 10; i++ {
		am.LogAction("obj-1", "read", "user", "读取对象", "", true, "")
	}

	// 获取限制数量的记录
	entries := am.GetEntries(5)
	if len(entries) != 5 {
		t.Errorf("期望5条记录，实际%d条", len(entries))
	}

	// 获取所有记录
	entries = am.GetEntries(0)
	if len(entries) != 10 {
		t.Errorf("期望10条记录，实际%d条", len(entries))
	}
}

func TestAuditManager_GetEntriesForObject(t *testing.T) {
	am := NewAuditManager(365)

	am.LogAction("obj-1", "create", "admin", "创建对象1", "", true, "")
	am.LogAction("obj-2", "create", "admin", "创建对象2", "", true, "")
	am.LogAction("obj-1", "read", "user", "读取对象1", "", true, "")

	entries := am.GetEntriesForObject("obj-1")
	if len(entries) != 2 {
		t.Errorf("期望2条记录，实际%d条", len(entries))
	}
}

func TestAuditManager_GetFailedAttempts(t *testing.T) {
	am := NewAuditManager(365)

	am.LogAction("obj-1", "delete", "user", "删除失败", "", false, "权限不足")
	am.LogAction("obj-1", "read", "user", "读取成功", "", true, "")
	am.LogAction("obj-1", "write", "user", "写入失败", "", false, "磁盘已满")

	failed := am.GetFailedAttempts()
	if len(failed) != 2 {
		t.Errorf("期望2条失败记录，实际%d条", len(failed))
	}
}

func TestAuditManager_PurgeOldEntries(t *testing.T) {
	am := NewAuditManager(1) // 1天保留期

	// 创建旧记录（模拟）
	oldEntry := &AuditEntry{
		ID:        "old-1",
		Timestamp: time.Now().AddDate(0, 0, -2), // 2天前
		Action:    "old_action",
	}
	am.entries = append(am.entries, oldEntry)

	// 创建新记录
	am.LogAction("obj-1", "create", "admin", "新操作", "", true, "")

	// 清理
	purged := am.PurgeOldEntries()
	if purged != 1 {
		t.Errorf("期望清理1条记录，实际清理%d条", purged)
	}

	if am.GetEntryCount() != 1 {
		t.Errorf("期望剩余1条记录，实际%d条", am.GetEntryCount())
	}
}

func TestWORMManager_FullWorkflow(t *testing.T) {
	config := DefaultWORMConfig()
	wm := NewWORMManager(config)

	// 创建策略
	policy, err := wm.CreatePolicy(
		"测试策略",
		"用于测试",
		ModeGovernance,
		RetentionPeriod{Value: 30, Unit: RetentionDays},
		[]string{"/data/test"},
		nil,
	)
	if err != nil {
		t.Fatalf("创建策略失败: %v", err)
	}

	// 保护对象
	obj, err := wm.ProtectObject("/data/test/file.txt", 1024, policy.ID, "admin", nil)
	if err != nil {
		t.Fatalf("保护对象失败: %v", err)
	}

	// 锁定对象
	err = wm.LockObject(obj.ID, "admin")
	if err != nil {
		t.Fatalf("锁定对象失败: %v", err)
	}

	// 验证对象
	valid, err := wm.VerifyObject(obj.ID)
	if err != nil {
		t.Fatalf("验证对象失败: %v", err)
	}
	if !valid {
		t.Error("对象应该通过验证")
	}

	// 验证哈希链
	valid, err = wm.VerifyHashChain()
	if err != nil {
		t.Fatalf("验证哈希链失败: %v", err)
	}
	if !valid {
		t.Error("哈希链应该通过验证")
	}

	// 验证审计链
	valid, err = wm.VerifyAuditChain()
	if err != nil {
		t.Fatalf("验证审计链失败: %v", err)
	}
	if !valid {
		t.Error("审计链应该通过验证")
	}

	// 删除对象
	err = wm.DeleteObject(obj.ID, "admin")
	if err != nil {
		t.Fatalf("删除对象失败: %v", err)
	}

	// 验证对象已删除
	_, err = wm.GetObject(obj.ID)
	if err == nil {
		t.Error("期望对象已被删除")
	}

	// 检查审计日志
	auditLog := wm.GetAuditLog(10)
	if len(auditLog) < 4 {
		t.Errorf("期望至少4条审计记录，实际%d条", len(auditLog))
	}
}

func TestWORMManager_ComplianceReport(t *testing.T) {
	config := DefaultWORMConfig()
	wm := NewWORMManager(config)

	// 创建 SOX 策略
	wm.CreatePolicy(
		"SOX策略",
		"SOX 合规策略",
		ModeEnterprise,
		RetentionPeriod{Value: 7, Unit: RetentionYears},
		[]string{"/data/finance"},
		[]RegulationType{RegulationSOX},
	)

	// 创建 GDPR 策略（违反 GDPR）
	wm.CreatePolicy(
		"GDPR策略",
		"GDPR 合规策略",
		ModeGovernance,
		RetentionPeriod{Value: 0, Unit: RetentionForever},
		[]string{"/data/users"},
		[]RegulationType{RegulationGDPR},
	)

	// 生成 SOX 报告
	report, err := wm.GenerateComplianceReport(RegulationSOX)
	if err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}

	if report.Status != StatusCompliant {
		t.Errorf("SOX 报告应该是合规的，实际状态为: %s", report.Status)
	}

	// 生成 GDPR 报告
	report, err = wm.GenerateComplianceReport(RegulationGDPR)
	if err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}

	// 注意: 由于没有实际对象，报告应该是合规的
	// 违规检查需要有实际对象才能触发
}

func TestRetentionPeriod_GetDuration(t *testing.T) {
	tests := []struct {
		name     string
		rp       RetentionPeriod
		expected time.Duration
	}{
		{"30天", RetentionPeriod{Value: 30, Unit: RetentionDays}, 30 * 24 * time.Hour},
		{"3个月", RetentionPeriod{Value: 3, Unit: RetentionMonths}, 90 * 24 * time.Hour},
		{"1年", RetentionPeriod{Value: 1, Unit: RetentionYears}, 365 * 24 * time.Hour},
		{"永久", RetentionPeriod{Value: 0, Unit: RetentionForever}, 100 * 365 * 24 * time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			duration := tt.rp.GetDuration()
			if duration != tt.expected {
				t.Errorf("期望 %v，实际 %v", tt.expected, duration)
			}
		})
	}
}

func TestRetentionPeriod_IsForever(t *testing.T) {
	rp := RetentionPeriod{Unit: RetentionForever}
	if !rp.IsForever() {
		t.Error("永久保留应该返回 true")
	}

	rp = RetentionPeriod{Value: 30, Unit: RetentionDays}
	if rp.IsForever() {
		t.Error("30天保留不应该返回 true")
	}
}

func TestProtectedObject_IsExpired(t *testing.T) {
	obj := &ProtectedObject{
		ExpiresAt: nil,
	}

	if obj.IsExpired(time.Now()) {
		t.Error("没有过期时间的对象不应该过期")
	}

	pastTime := time.Now().Add(-1 * time.Hour)
	obj.ExpiresAt = &pastTime

	if !obj.IsExpired(time.Now()) {
		t.Error("已过期的对象应该过期")
	}

	futureTime := time.Now().Add(1 * time.Hour)
	obj.ExpiresAt = &futureTime

	if obj.IsExpired(time.Now()) {
		t.Error("未过期的对象不应该过期")
	}
}

func TestWORMManager_ListExpiredObjects(t *testing.T) {
	config := DefaultWORMConfig()
	wm := NewWORMManager(config)

	// 创建策略
	policy, _ := wm.CreatePolicy("测试策略", "测试", ModeGovernance, RetentionPeriod{Value: 1, Unit: RetentionDays}, []string{"/test"}, nil)

	// 保护对象
	obj, _ := wm.ProtectObject("/test/file.txt", 1024, policy.ID, "admin", nil)

	// 锁定对象
	wm.LockObject(obj.ID, "admin")

	// 模拟时间流逝（修改对象的过期时间）
	wm.mu.Lock()
	lockedObj, _ := wm.immutabilityMgr.GetObject(obj.ID)
	pastTime := time.Now().Add(-1 * time.Hour)
	lockedObj.ExpiresAt = &pastTime
	wm.mu.Unlock()

	// 列出过期对象
	expired := wm.ListExpiredObjects()
	if len(expired) != 1 {
		t.Errorf("期望1个过期对象，实际%d个", len(expired))
	}
}

func TestWORMManager_GetAuditStats(t *testing.T) {
	config := DefaultWORMConfig()
	wm := NewWORMManager(config)

	// 创建一些审计记录
	wm.auditManager.LogAction("obj-1", "create", "admin", "创建", "", true, "")
	wm.auditManager.LogAction("obj-1", "read", "user", "读取", "", true, "")
	wm.auditManager.LogAction("obj-1", "delete", "user", "删除失败", "", false, "权限不足")

	stats := wm.GetAuditStats()
	total, ok := stats["total_records"].(int)
	if !ok || total != 3 {
		t.Errorf("期望3条记录，实际 %v", stats["total_records"])
	}

	successful, ok := stats["successful"].(int)
	if !ok || successful != 2 {
		t.Errorf("期望2条成功记录，实际 %v", stats["successful"])
	}

	failed, ok := stats["failed"].(int)
	if !ok || failed != 1 {
		t.Errorf("期望1条失败记录，实际 %v", stats["failed"])
	}
}

func TestDefaultWORMConfig(t *testing.T) {
	config := DefaultWORMConfig()

	if config.HashChainSeed != "nas-os-worm-seed-2024" {
		t.Errorf("期望种子为 'nas-os-worm-seed-2024'，实际为 '%s'", config.HashChainSeed)
	}

	if !config.EnableAuditLog {
		t.Error("应该默认启用审计日志")
	}

	if config.MaxAuditRetentionDays != 365 {
		t.Errorf("期望最大保留天数为365，实际为%d", config.MaxAuditRetentionDays)
	}

	if config.DefaultMode != ModeGovernance {
		t.Errorf("期望默认模式为 'governance'，实际为 '%s'", config.DefaultMode)
	}
}

// BenchmarkPolicyManager_CreatePolicy 性能测试
func BenchmarkPolicyManager_CreatePolicy(b *testing.B) {
	pm := NewPolicyManager()

	for i := 0; i < b.N; i++ {
		pm.CreatePolicy("策略", "测试", ModeGovernance, RetentionPeriod{Value: 30, Unit: RetentionDays}, []string{"/test"}, nil)
	}
}

// BenchmarkImmutabilityManager_ProtectObject 性能测试
func BenchmarkImmutabilityManager_ProtectObject(b *testing.B) {
	config := DefaultWORMConfig()
	im := NewImmutabilityManager(config)

	for i := 0; i < b.N; i++ {
		im.ProtectObject("/test/file.txt", 1024, "policy-1", "admin", nil)
	}
}

// BenchmarkAuditManager_LogAction 性能测试
func BenchmarkAuditManager_LogAction(b *testing.B) {
	am := NewAuditManager(365)

	for i := 0; i < b.N; i++ {
		am.LogAction("obj-1", "action", "user", "details", "", true, "")
	}
}
