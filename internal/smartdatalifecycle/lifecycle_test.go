package smartdatalifecycle

import (
	"context"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if !config.Enabled {
		t.Error("expected config to be enabled by default")
	}

	if config.ScanIntervalSec != 3600 {
		t.Errorf("expected ScanIntervalSec to be 3600, got %d", config.ScanIntervalSec)
	}

	if config.MaxConcurrentOps != 4 {
		t.Errorf("expected MaxConcurrentOps to be 4, got %d", config.MaxConcurrentOps)
	}

	if !config.Archive.Enabled {
		t.Error("expected archive to be enabled by default")
	}

	if config.Archive.MinIdleDays != 30 {
		t.Errorf("expected Archive.MinIdleDays to be 30, got %d", config.Archive.MinIdleDays)
	}

	if !config.Cleanup.Enabled {
		t.Error("expected cleanup to be enabled by default")
	}

	if config.Cleanup.TrashRetentionDays != 30 {
		t.Errorf("expected Cleanup.TrashRetentionDays to be 30, got %d", config.Cleanup.TrashRetentionDays)
	}

	if !config.Migration.Enabled {
		t.Error("expected migration to be enabled by default")
	}

	if config.Migration.WarmAfterDays != 7 {
		t.Errorf("expected Migration.WarmAfterDays to be 7, got %d", config.Migration.WarmAfterDays)
	}

	if config.Migration.ColdAfterDays != 30 {
		t.Errorf("expected Migration.ColdAfterDays to be 30, got %d", config.Migration.ColdAfterDays)
	}

	if config.Migration.ArchiveAfterDays != 90 {
		t.Errorf("expected Migration.ArchiveAfterDays to be 90, got %d", config.Migration.ArchiveAfterDays)
	}

	if !config.Retention.Enabled {
		t.Error("expected retention to be enabled by default")
	}

	if config.Retention.GracePeriodDays != 7 {
		t.Errorf("expected Retention.GracePeriodDays to be 7, got %d", config.Retention.GracePeriodDays)
	}

	if !config.Dedup.Enabled {
		t.Error("expected dedup to be enabled by default")
	}

	if config.Dedup.Algorithm != "xxhash" {
		t.Errorf("expected Dedup.Algorithm to be xxhash, got %s", config.Dedup.Algorithm)
	}
}

func TestManagerCreate(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	if manager == nil {
		t.Fatal("expected manager to be created")
	}

	if manager.IsRunning() {
		t.Error("expected manager to not be running initially")
	}
}

func TestManagerStartStop(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	ctx := context.Background()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("failed to start manager: %v", err)
	}

	if !manager.IsRunning() {
		t.Error("expected manager to be running")
	}

	// 不能重复启动
	if err := manager.Start(ctx); err == nil {
		t.Error("expected error on double start")
	}

	manager.Stop()

	if manager.IsRunning() {
		t.Error("expected manager to be stopped")
	}
}

func TestRegisterAndGetItem(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	item := &DataItem{
		ID:   "test-item-1",
		Path: "/data/test.txt",
		Name: "test.txt",
		Size: 1024,
	}

	if err := manager.RegisterItem(item); err != nil {
		t.Fatalf("failed to register item: %v", err)
	}

	got, ok := manager.GetItem("test-item-1")
	if !ok {
		t.Fatal("expected to get item")
	}

	if got.ID != item.ID {
		t.Errorf("expected ID %s, got %s", item.ID, got.ID)
	}

	if got.Path != item.Path {
		t.Errorf("expected path %s, got %s", item.Path, got.Path)
	}

	if got.Stage != StageActive {
		t.Errorf("expected stage to be active, got %s", got.Stage)
	}
}

func TestUpdateItemStage(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	item := &DataItem{
		ID:   "test-item-2",
		Path: "/data/test2.txt",
		Name: "test2.txt",
		Size: 2048,
	}

	if err := manager.RegisterItem(item); err != nil {
		t.Fatalf("failed to register item: %v", err)
	}

	// 更新阶段
	if err := manager.UpdateItemStage("test-item-2", StageWarm, "test"); err != nil {
		t.Fatalf("failed to update stage: %v", err)
	}

	got, _ := manager.GetItem("test-item-2")
	if got.Stage != StageWarm {
		t.Errorf("expected stage warm, got %s", got.Stage)
	}
}

func TestLegalHoldProtection(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	item := &DataItem{
		ID:        "test-item-3",
		Path:      "/data/legal.txt",
		Name:      "legal.txt",
		Size:      4096,
		LegalHold: true,
	}

	if err := manager.RegisterItem(item); err != nil {
		t.Fatalf("failed to register item: %v", err)
	}

	// 尝试删除法律冻结的数据
	err := manager.UpdateItemStage("test-item-3", StageDeleted, "test")
	if err == nil {
		t.Error("expected error when deleting legal hold item")
	}
}

func TestRecordAccess(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	item := &DataItem{
		ID:   "test-item-4",
		Path: "/data/access.txt",
		Name: "access.txt",
		Size: 512,
	}

	if err := manager.RegisterItem(item); err != nil {
		t.Fatalf("failed to register item: %v", err)
	}

	// 记录访问
	if err := manager.RecordAccess("test-item-4", "read"); err != nil {
		t.Fatalf("failed to record access: %v", err)
	}

	if err := manager.RecordAccess("test-item-4", "read"); err != nil {
		t.Fatalf("failed to record access: %v", err)
	}

	got, _ := manager.GetItem("test-item-4")
	if got.AccessCount != 2 {
		t.Errorf("expected access count 2, got %d", got.AccessCount)
	}
	if got.ReadCount != 2 {
		t.Errorf("expected read count 2, got %d", got.ReadCount)
	}
}

func TestRetentionPolicy(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	policy := &RetentionPolicy{
		ID:              "test-policy-1",
		Name:            "Test Policy",
		RetentionDays:   365,
		ExpirationAction: ActionArchive,
	}

	manager.AddRetentionPolicy(policy)

	got, ok := manager.GetRetentionPolicy("test-policy-1")
	if !ok {
		t.Fatal("expected to get policy")
	}

	if got.Name != "Test Policy" {
		t.Errorf("expected name 'Test Policy', got %s", got.Name)
	}

	if got.RetentionDays != 365 {
		t.Errorf("expected retention days 365, got %d", got.RetentionDays)
	}

	// 列出策略
	policies := manager.ListRetentionPolicies()
	if len(policies) != 1 {
		t.Errorf("expected 1 policy, got %d", len(policies))
	}
}

func TestArchivePolicy(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	policy := &ArchivePolicy{
		ID:              "test-archive-1",
		Name:            "Old Files Archive",
		Enabled:         true,
		Trigger:         TriggerLastAccessTime,
		DaysSinceAccess: 30,
		TargetStage:     StageArchive,
	}

	manager.AddArchivePolicy(policy)

	got, ok := manager.GetArchivePolicy("test-archive-1")
	if !ok {
		t.Fatal("expected to get archive policy")
	}

	if got.Trigger != TriggerLastAccessTime {
		t.Errorf("expected trigger last_access_time, got %s", got.Trigger)
	}
}

func TestCleanupRule(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	rule := &CleanupRule{
		ID:       "test-rule-1",
		Name:     "Temp Files Cleanup",
		Enabled:  true,
		RuleType: RuleTypeTempFiles,
	}

	manager.AddCleanupRule(rule)

	got, ok := manager.GetCleanupRule("test-rule-1")
	if !ok {
		t.Fatal("expected to get cleanup rule")
	}

	if got.RuleType != RuleTypeTempFiles {
		t.Errorf("expected rule type temp_files, got %s", got.RuleType)
	}
}

func TestMigrationTask(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	task := &MigrationTask{
		ID:          "test-migration-1",
		SourcePath:  "/data/hot/file.txt",
		TargetPath:  "/data/cold/file.txt",
		SourceStage: StageActive,
		TargetStage: StageCold,
		Size:        1024,
		Reason:      "test migration",
	}

	manager.AddMigrationTask(task)

	got, ok := manager.GetMigrationTask("test-migration-1")
	if !ok {
		t.Fatal("expected to get migration task")
	}

	if got.Status != MigrationPending {
		t.Errorf("expected status pending, got %s", got.Status)
	}
}

func TestListItemsByStage(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	// 注册多个不同阶段的数据项
	items := []*DataItem{
		{ID: "active-1", Path: "/data/a1.txt", Size: 100, Stage: StageActive},
		{ID: "active-2", Path: "/data/a2.txt", Size: 200, Stage: StageActive},
		{ID: "warm-1", Path: "/data/w1.txt", Size: 300, Stage: StageWarm},
		{ID: "cold-1", Path: "/data/c1.txt", Size: 400, Stage: StageCold},
	}

	for _, item := range items {
		if err := manager.RegisterItem(item); err != nil {
			t.Fatalf("failed to register item %s: %v", item.ID, err)
		}
	}

	// 测试按阶段列出
	activeItems := manager.ListItems(StageActive, 0, 0)
	if len(activeItems) != 2 {
		t.Errorf("expected 2 active items, got %d", len(activeItems))
	}

	warmItems := manager.ListItems(StageWarm, 0, 0)
	if len(warmItems) != 1 {
		t.Errorf("expected 1 warm item, got %d", len(warmItems))
	}

	coldItems := manager.ListItems(StageCold, 0, 0)
	if len(coldItems) != 1 {
		t.Errorf("expected 1 cold item, got %d", len(coldItems))
	}

	// 测试列出全部
	allItems := manager.ListItems("", 0, 0)
	if len(allItems) != 4 {
		t.Errorf("expected 4 total items, got %d", len(allItems))
	}
}

func TestEvents(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	item := &DataItem{
		ID:   "event-item-1",
		Path: "/data/event.txt",
		Name: "event.txt",
		Size: 1024,
	}

	if err := manager.RegisterItem(item); err != nil {
		t.Fatalf("failed to register item: %v", err)
	}

	// 更新阶段会产生事件
	if err := manager.UpdateItemStage("event-item-1", StageWarm, "test"); err != nil {
		t.Fatalf("failed to update stage: %v", err)
	}

	events := manager.GetEvents(10, 0)
	if len(events) == 0 {
		t.Error("expected at least one event")
	}
}

func TestStats(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	// 注册一些数据项
	for i := 0; i < 5; i++ {
		item := &DataItem{
			ID:    "stat-item-" + string(rune('a'+i)),
			Path:  "/data/stat" + string(rune('a'+i)) + ".txt",
			Size:  int64((i + 1) * 1000),
			Stage: LifecycleStage([]string{"active", "warm", "cold", "archive", "active"}[i]),
		}
		manager.RegisterItem(item)
	}

	// 手动更新统计（因为后台更新器未启动）
	manager.updateStats()

	stats := manager.GetStats()
	if stats == nil {
		t.Fatal("expected stats to be non-nil")
	}

	if stats.TotalItems != 5 {
		t.Errorf("expected 5 total items, got %d", stats.TotalItems)
	}
}

func TestLifecycleStages(t *testing.T) {
	tests := []struct {
		stage LifecycleStage
		str   string
	}{
		{StageActive, "active"},
		{StageWarm, "warm"},
		{StageCold, "cold"},
		{StageArchive, "archive"},
		{StageExpired, "expired"},
		{StageDeleted, "deleted"},
	}

	for _, tt := range tests {
		if tt.stage.String() != tt.str {
			t.Errorf("expected %s, got %s", tt.str, tt.stage.String())
		}

		parsed := ParseStage(tt.str)
		if parsed != tt.stage {
			t.Errorf("ParseStage(%s) = %v, want %v", tt.str, parsed, tt.stage)
		}
	}
}

func TestRetentionPolicyValidation(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	rm := manager.retentionMgr

	// 测试有效策略
	validPolicy := &RetentionPolicy{
		RetentionDays:    365,
		ExpirationAction: ActionArchive,
	}
	warnings := rm.ValidateRetentionPolicy(validPolicy)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for valid policy, got %d", len(warnings))
	}

	// 测试负数保留天数
	negativePolicy := &RetentionPolicy{
		RetentionDays: -1,
	}
	warnings = rm.ValidateRetentionPolicy(negativePolicy)
	if len(warnings) == 0 {
		t.Error("expected warning for negative retention days")
	}

	// 测试超长保留期
	longPolicy := &RetentionPolicy{
		RetentionDays: 5000,
	}
	warnings = rm.ValidateRetentionPolicy(longPolicy)
	if len(warnings) == 0 {
		t.Error("expected warning for very long retention period")
	}
}

func TestPatternMatching(t *testing.T) {
	// 测试简单匹配
	if !simpleMatch("test.txt", "*") {
		t.Error("expected * to match anything")
	}

	if !simpleMatch("/data/test.txt", "/data/*") {
		t.Error("expected /data/* to match /data/test.txt")
	}

	if !simpleMatch("test.log", "*.log") {
		t.Error("expected *.log to match test.log")
	}

	if simpleMatch("test.txt", "*.log") {
		t.Error("expected *.log to not match test.txt")
	}

	// 测试 matchesPattern
	if !matchesPattern("/data/test.txt", nil, nil) {
		t.Error("expected empty patterns to match anything")
	}

	if !matchesPattern("/data/test.txt", []string{"/data/"}, nil) {
		t.Error("expected prefix /data/ to match /data/test.txt")
	}

	if !matchesPattern("/data/test.txt", nil, []string{"/data/*"}) {
		t.Error("expected pattern /data/* to match /data/test.txt")
	}
}

func TestDeduplicator(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	ctx := context.Background()

	// 注册一些数据项（设置相同的 ContentHash 来模拟重复）
	item1 := &DataItem{
		ID:          "dup-item-1",
		Path:        "/data/original.txt",
		Size:        1024,
		ContentHash: "abc123hash",
	}
	item2 := &DataItem{
		ID:          "dup-item-2",
		Path:        "/data/copy1.txt",
		Size:        1024,
		ContentHash: "abc123hash",
	}
	item3 := &DataItem{
		ID:          "dup-item-3",
		Path:        "/data/copy2.txt",
		Size:        1024,
		ContentHash: "abc123hash",
	}
	item4 := &DataItem{
		ID:          "unique-item-1",
		Path:        "/data/unique.txt",
		Size:        2048,
		ContentHash: "unique456hash",
	}

	manager.RegisterItem(item1)
	manager.RegisterItem(item2)
	manager.RegisterItem(item3)
	manager.RegisterItem(item4)

	// 执行扫描
	result, err := manager.deduplicator.Scan(ctx)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if result.ScannedItems != 4 {
		t.Errorf("expected 4 scanned items, got %d", result.ScannedItems)
	}

	if result.DuplicateGroups != 1 {
		t.Errorf("expected 1 duplicate group, got %d", result.DuplicateGroups)
	}

	if result.WastedSpace != 2048 {
		t.Errorf("expected 2048 bytes wasted, got %d", result.WastedSpace)
	}
}

func TestRetentionManager(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	ctx := context.Background()

	// 注册一个已过期的数据项
	expiredTime := time.Now().Add(-24 * time.Hour)
	item := &DataItem{
		ID:        "expired-item-1",
		Path:      "/data/expired.txt",
		Size:      1024,
		ExpiresAt: &expiredTime,
	}

	manager.RegisterItem(item)

	// 运行保留检查
	if err := manager.retentionMgr.Run(ctx); err != nil {
		t.Fatalf("retention check failed: %v", err)
	}
}

func TestArchiverEvaluation(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	// 注册一个长期未访问的数据项
	oldAccess := time.Now().Add(-100 * 24 * time.Hour)
	item := &DataItem{
		ID:         "old-item-1",
		Path:       "/data/old.txt",
		Size:       1024,
		AccessedAt: oldAccess,
	}

	manager.RegisterItem(item)

	// 添加归档策略
	policy := &ArchivePolicy{
		ID:              "archive-old",
		Name:            "Archive Old Files",
		Enabled:         true,
		Trigger:         TriggerLastAccessTime,
		DaysSinceAccess: 30,
		TargetStage:     StageArchive,
	}
	manager.AddArchivePolicy(policy)

	// 评估归档候选
	candidates := manager.archiver.GetArchiveCandidates(10)
	if len(candidates) == 0 {
		t.Error("expected at least one archive candidate")
	}
}

func TestConcurrencySafety(t *testing.T) {
	config := DefaultConfig()
	manager := NewManager(config, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动管理器
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("failed to start manager: %v", err)
	}
	defer manager.Stop()

	// 并发注册数据项
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			item := &DataItem{
				ID:   "concurrent-item-" + string(rune('a'+idx)),
				Path: "/data/concurrent" + string(rune('a'+idx)) + ".txt",
				Size: int64((idx + 1) * 100),
			}
			manager.RegisterItem(item)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证所有项都已注册
	items := manager.ListItems("", 0, 0)
	if len(items) != 10 {
		t.Errorf("expected 10 items, got %d", len(items))
	}
}
