// 单元测试
package smartlifebackup

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ============================================================================
// 类型测试
// ============================================================================

func TestDefaultBackupPolicy(t *testing.T) {
	policy := DefaultBackupPolicy()

	if policy.ID != "default" {
		t.Errorf("期望ID为 'default'，实际为 '%s'", policy.ID)
	}

	if !policy.Enabled {
		t.Error("期望策略默认启用")
	}

	if len(policy.RetentionRules) != 4 {
		t.Errorf("期望4条保留规则，实际为 %d", len(policy.RetentionRules))
	}

	if policy.CompressionType != CompressionGzip {
		t.Errorf("期望压缩类型为 gzip，实际为 %s", policy.CompressionType)
	}

	if !policy.Deduplication {
		t.Error("期望启用去重")
	}
}

func TestDefaultStorageCost(t *testing.T) {
	cost := DefaultStorageCost()

	if cost.HotCostPerGB <= 0 {
		t.Error("热存储成本应大于0")
	}

	if cost.WarmCostPerGB >= cost.HotCostPerGB {
		t.Error("温存储成本应低于热存储")
	}

	if cost.ColdCostPerGB >= cost.WarmCostPerGB {
		t.Error("冷存储成本应低于温存储")
	}

	if cost.ArchiveCostPerGB >= cost.ColdCostPerGB {
		t.Error("归档存储成本应低于冷存储")
	}
}

func TestDefaultScheduleConfig(t *testing.T) {
	config := DefaultScheduleConfig()

	if len(config.PeakHours) == 0 {
		t.Error("应配置高峰时段")
	}

	if len(config.AllowedWindows) == 0 {
		t.Error("应配置允许窗口")
	}

	if config.MaxConcurrent <= 0 {
		t.Error("最大并发数应大于0")
	}
}

// ============================================================================
// 管理器测试
// ============================================================================

func TestManagerInitialize(t *testing.T) {
	// 使用临时目录
	tmpDir := t.TempDir()

	manager := NewManager(tmpDir, tmpDir+"/storage")
	if err := manager.Initialize(); err != nil {
		t.Fatalf("初始化失败：%v", err)
	}

	// 验证默认策略已创建
	policies := manager.ListPolicies()
	if len(policies) == 0 {
		t.Error("应自动创建默认策略")
	}

	// 验证活跃策略
	policy, err := manager.GetActivePolicy()
	if err != nil {
		t.Fatalf("获取活跃策略失败：%v", err)
	}

	if policy.ID != "default" {
		t.Errorf("期望活跃策略ID为 'default'，实际为 '%s'", policy.ID)
	}
}

func TestManagerPolicyCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, tmpDir+"/storage")
	if err := manager.Initialize(); err != nil {
		t.Fatalf("初始化失败：%v", err)
	}

	// 创建策略
	policy := &BackupPolicy{
		ID:              "test-policy",
		Name:            "测试策略",
		Enabled:         true,
		CompressionType: CompressionGzip,
		RetentionRules: []RetentionRule{
			{
				Name:        "每日保留",
				RetainDays:  7,
				KeepCount:   7,
				Interval:    TimeIntervalDaily,
				StorageTier: StorageTierHot,
			},
		},
	}

	if err := manager.CreatePolicy(policy); err != nil {
		t.Fatalf("创建策略失败：%v", err)
	}

	// 获取策略
	retrieved, err := manager.GetPolicy("test-policy")
	if err != nil {
		t.Fatalf("获取策略失败：%v", err)
	}

	if retrieved.Name != "测试策略" {
		t.Errorf("策略名称不匹配：期望 '测试策略'，实际 '%s'", retrieved.Name)
	}

	// 更新策略
	policy.Name = "更新后的策略"
	if err := manager.UpdatePolicy("test-policy", policy); err != nil {
		t.Fatalf("更新策略失败：%v", err)
	}

	retrieved, _ = manager.GetPolicy("test-policy")
	if retrieved.Name != "更新后的策略" {
		t.Errorf("策略名称未更新：期望 '更新后的策略'，实际 '%s'", retrieved.Name)
	}

	// 删除策略
	if err := manager.DeletePolicy("test-policy"); err != nil {
		t.Fatalf("删除策略失败：%v", err)
	}

	_, err = manager.GetPolicy("test-policy")
	if err == nil {
		t.Error("策略应已被删除")
	}
}

func TestManagerBackupItem(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, tmpDir+"/storage")
	if err := manager.Initialize(); err != nil {
		t.Fatalf("初始化失败：%v", err)
	}

	// 注册备份项
	item := &BackupItem{
		ID:         "backup-1",
		Name:       "测试备份",
		SourcePath: "/data/test",
		BackupPath: "/backups/test",
		Size:       1024 * 1024 * 100, // 100MB
		Tier:       StorageTierHot,
		Checksum:   "abc123",
	}

	if err := manager.RegisterBackup(item); err != nil {
		t.Fatalf("注册备份项失败：%v", err)
	}

	// 获取备份项
	retrieved, err := manager.GetBackup("backup-1")
	if err != nil {
		t.Fatalf("获取备份项失败：%v", err)
	}

	if retrieved.Name != "测试备份" {
		t.Errorf("备份名称不匹配：期望 '测试备份'，实际 '%s'", retrieved.Name)
	}

	// 列出备份
	backups := manager.ListBackups()
	if len(backups) != 1 {
		t.Errorf("期望1个备份，实际 %d 个", len(backups))
	}

	// 删除备份
	if err := manager.DeleteBackup("backup-1"); err != nil {
		t.Fatalf("删除备份项失败：%v", err)
	}

	backups = manager.ListBackups()
	if len(backups) != 0 {
		t.Errorf("期望0个备份，实际 %d 个", len(backups))
	}
}

// ============================================================================
// 成本计算器测试
// ============================================================================

func TestCostCalculator(t *testing.T) {
	calc := NewCostCalculator(DefaultStorageCost())

	// 测试热存储成本
	hotCost := calc.CalculateCost(StorageTierHot, 10) // 10GB
	if hotCost <= 0 {
		t.Error("热存储成本应大于0")
	}

	// 测试冷存储成本应低于热存储
	coldCost := calc.CalculateCost(StorageTierCold, 10)
	if coldCost >= hotCost {
		t.Error("冷存储成本应低于热存储")
	}

	// 测试传输成本
	transferCost := calc.CalculateTransferCost(5) // 5GB
	if transferCost <= 0 {
		t.Error("传输成本应大于0")
	}
}

func TestCostCalculatorReport(t *testing.T) {
	calc := NewCostCalculator(DefaultStorageCost())

	// 创建测试数据
	backups := map[string]*BackupItem{
		"1": {
			ID:        "1",
			Name:      "热备份",
			Size:      1024 * 1024 * 1024, // 1GB
			Tier:      StorageTierHot,
			CreatedAt: time.Now().AddDate(0, 0, -3),
		},
		"2": {
			ID:        "2",
			Name:      "冷备份",
			Size:      2 * 1024 * 1024 * 1024, // 2GB
			Tier:      StorageTierCold,
			CreatedAt: time.Now().AddDate(0, -2, 0),
		},
	}

	report := calc.GenerateReport(backups)

	if report.TotalStorageGB < 2.9 || report.TotalStorageGB > 3.1 {
		t.Errorf("总存储应约为3GB，实际 %.2f GB", report.TotalStorageGB)
	}

	if report.TotalCost <= 0 {
		t.Error("总成本应大于0")
	}

	if len(report.TierBreakdown) != 4 {
		t.Errorf("应有4个层级分解，实际 %d 个", len(report.TierBreakdown))
	}
}

// ============================================================================
// 调度器测试
// ============================================================================

func TestSchedulerTimeWindow(t *testing.T) {
	config := &ScheduleConfig{
		PeakHours: []PeakHour{
			{
				StartHour: 9,
				EndHour:   18,
				Days:      []int{1, 2, 3, 4, 5}, // 工作日
			},
		},
		AllowedWindows: []TimeWindow{
			{
				StartHour: 22,
				EndHour:   6,
				Priority:  1,
			},
		},
		MaxConcurrent: 3,
	}

	scheduler := NewScheduler(config)
	defer scheduler.Stop()

	// 测试时间范围检查
	if !scheduler.isInTimeRange(23, 22, 6) {
		t.Error("23点应在22-6范围内")
	}

	if !scheduler.isInTimeRange(2, 22, 6) {
		t.Error("2点应在22-6范围内")
	}

	if scheduler.isInTimeRange(10, 22, 6) {
		t.Error("10点不应在22-6范围内")
	}

	if !scheduler.isInTimeRange(10, 9, 18) {
		t.Error("10点应在9-18范围内")
	}
}

func TestSchedulerParseSchedule(t *testing.T) {
	config := DefaultScheduleConfig()
	scheduler := NewScheduler(config)
	defer scheduler.Stop()

	// 测试 @hourly
	nextRun, err := scheduler.parseSchedule("@hourly")
	if err != nil {
		t.Fatalf("@hourly 解析失败：%v", err)
	}
	if nextRun.Before(time.Now()) {
		t.Error("@hourly 的下次运行时间应在未来")
	}

	// 测试 @daily
	nextRun, err = scheduler.parseSchedule("@daily")
	if err != nil {
		t.Fatalf("@daily 解析失败：%v", err)
	}
	if nextRun.Before(time.Now()) {
		t.Error("@daily 的下次运行时间应在未来")
	}

	// 测试具体时间
	nextRun, err = scheduler.parseSchedule("02:30")
	if err != nil {
		t.Fatalf("02:30 解析失败：%v", err)
	}
	if nextRun.Hour() != 2 || nextRun.Minute() != 30 {
		t.Errorf("期望时间 02:30，实际 %02d:%02d", nextRun.Hour(), nextRun.Minute())
	}
}

func TestSchedulerTaskManagement(t *testing.T) {
	config := DefaultScheduleConfig()
	scheduler := NewScheduler(config)
	defer scheduler.Stop()

	// 调度任务
	err := scheduler.ScheduleTask("@hourly", func() {
		// 任务函数
	})
	if err != nil {
		t.Fatalf("调度任务失败：%v", err)
	}

	tasks := scheduler.GetScheduledTasks()
	if len(tasks) != 1 {
		t.Errorf("期望1个任务，实际 %d 个", len(tasks))
	}

	// 获取任务ID
	var taskID string
	for id := range tasks {
		taskID = id
	}

	// 禁用任务
	if err := scheduler.DisableTask(taskID); err != nil {
		t.Fatalf("禁用任务失败：%v", err)
	}

	// 启用任务
	if err := scheduler.EnableTask(taskID); err != nil {
		t.Fatalf("启用任务失败：%v", err)
	}

	// 取消任务
	scheduler.UnscheduleTask(taskID)
	tasks = scheduler.GetScheduledTasks()
	if len(tasks) != 0 {
		t.Errorf("期望0个任务，实际 %d 个", len(tasks))
	}
}

func TestSchedulerStats(t *testing.T) {
	config := DefaultScheduleConfig()
	scheduler := NewScheduler(config)
	defer scheduler.Stop()

	stats := scheduler.GetStats()

	if stats["total_tasks"] != 0 {
		t.Errorf("期望0个任务，实际 %v", stats["total_tasks"])
	}

	if stats["enabled_tasks"] != 0 {
		t.Errorf("期望0个启用任务，实际 %v", stats["enabled_tasks"])
	}
}

// ============================================================================
// HTTP Handlers 测试
// ============================================================================

func TestHandlersHealth(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, tmpDir+"/storage")
	if err := manager.Initialize(); err != nil {
		t.Fatalf("初始化失败：%v", err)
	}

	handlers := NewHandlers(manager)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backup/lifecycle/health", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "healthy") {
		t.Error("响应应包含 'healthy'")
	}
}

func TestHandlersGetStats(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, tmpDir+"/storage")
	if err := manager.Initialize(); err != nil {
		t.Fatalf("初始化失败：%v", err)
	}

	handlers := NewHandlers(manager)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backup/lifecycle/stats", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "total_backups") {
		t.Error("响应应包含 'total_backups'")
	}
}

func TestHandlersGetPolicies(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, tmpDir+"/storage")
	if err := manager.Initialize(); err != nil {
		t.Fatalf("初始化失败：%v", err)
	}

	handlers := NewHandlers(manager)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backup/lifecycle/policies", nil)
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200，实际 %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "default") {
		t.Error("响应应包含默认策略")
	}
}

func TestHandlersCreatePolicy(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, tmpDir+"/storage")
	if err := manager.Initialize(); err != nil {
		t.Fatalf("初始化失败：%v", err)
	}

	handlers := NewHandlers(manager)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	policyJSON := `{
		"id": "new-policy",
		"name": "新策略",
		"enabled": true,
		"compression_type": "gzip",
		"retention_rules": [
			{
				"name": "每日保留",
				"retain_days": 7,
				"keep_count": 7,
				"interval": "daily",
				"storage_tier": "hot"
			}
		]
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backup/lifecycle/policies", strings.NewReader(policyJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 201，实际 %d", w.Code)
	}

	// 验证策略已创建
	policy, err := manager.GetPolicy("new-policy")
	if err != nil {
		t.Fatalf("策略创建失败：%v", err)
	}

	if policy.Name != "新策略" {
		t.Errorf("策略名称不匹配：期望 '新策略'，实际 '%s'", policy.Name)
	}
}

// ============================================================================
// 集成测试
// ============================================================================

func TestIntegrationLifecycleExecution(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, tmpDir+"/storage")
	if err := manager.Initialize(); err != nil {
		t.Fatalf("初始化失败：%v", err)
	}

	// 注册一些测试备份
	for i := 0; i < 10; i++ {
		item := &BackupItem{
			ID:         generateID(),
			Name:       "备份",
			Size:       1024 * 1024 * 100,
			Tier:       StorageTierHot,
			CreatedAt:  time.Now().AddDate(0, 0, -i*2), // 每个备份间隔2天
			Compressed: false,
		}
		if err := manager.RegisterBackup(item); err != nil {
			t.Fatalf("注册备份失败：%v", err)
		}
	}

	// 执行生命周期管理（dry run）
	task, err := manager.ExecuteLifecycle(nil, &ExecuteOptions{DryRun: true})
	if err != nil {
		t.Fatalf("执行生命周期失败：%v", err)
	}

	// 等待任务完成
	time.Sleep(100 * time.Millisecond)

	retrieved, err := manager.GetTask(task.ID)
	if err != nil {
		t.Fatalf("获取任务失败：%v", err)
	}

	if retrieved.Status != TaskStatusCompleted {
		t.Errorf("任务状态应为 completed，实际 %s", retrieved.Status)
	}
}

func TestIntegrationCostReport(t *testing.T) {
	tmpDir := t.TempDir()
	manager := NewManager(tmpDir, tmpDir+"/storage")
	if err := manager.Initialize(); err != nil {
		t.Fatalf("初始化失败：%v", err)
	}

	// 注册不同层级的备份
	backups := []*BackupItem{
		{
			ID:   "hot-1",
			Name: "热备份",
			Size: 1024 * 1024 * 1024,
			Tier: StorageTierHot,
		},
		{
			ID:   "cold-1",
			Name: "冷备份",
			Size: 2 * 1024 * 1024 * 1024,
			Tier: StorageTierCold,
		},
		{
			ID:   "archive-1",
			Name: "归档备份",
			Size: 5 * 1024 * 1024 * 1024,
			Tier: StorageTierArchive,
		},
	}

	for _, item := range backups {
		if err := manager.RegisterBackup(item); err != nil {
			t.Fatalf("注册备份失败：%v", err)
		}
	}

	report := manager.GetCostReport()

	if report.TotalStorageGB < 7.9 || report.TotalStorageGB > 8.1 {
		t.Errorf("总存储应约为8GB，实际 %.2f GB", report.TotalStorageGB)
	}

	if report.TotalCost <= 0 {
		t.Error("总成本应大于0")
	}
}

// ============================================================================
// 基准测试
// ============================================================================

func BenchmarkCostCalculation(b *testing.B) {
	calc := NewCostCalculator(DefaultStorageCost())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.CalculateCost(StorageTierHot, 100)
	}
}

func BenchmarkManagerGetStats(b *testing.B) {
	tmpDir := b.TempDir()
	manager := NewManager(tmpDir, tmpDir+"/storage")
	if err := manager.Initialize(); err != nil {
		b.Fatalf("初始化失败：%v", err)
	}

	// 添加一些数据
	for i := 0; i < 100; i++ {
		item := &BackupItem{
			ID:   generateID(),
			Name: "备份",
			Size: 1024 * 1024 * 100,
			Tier: StorageTierHot,
		}
		manager.RegisterBackup(item)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetStats()
	}
}
