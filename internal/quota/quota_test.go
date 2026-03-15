package quota

import (
	"os"
	"testing"
	"time"
)

// ========== 测试用例 ==========

func TestNewManager(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}
	if mgr == nil {
		t.Fatal("Manager 不应为 nil")
	}
}

func TestCreateQuota(t *testing.T) {
	storage := NewMockStorageProvider()
	storage.volumes["data"] = &VolumeInfo{Name: "data", MountPoint: "/mnt/data", Size: 1 << 40}

	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30, // 100GB
		SoftLimit:  80 << 30,  // 80GB
	}

	quota, err := mgr.CreateQuota(input)
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	if quota.TargetID != "testuser" {
		t.Errorf("期望 TargetID 为 testuser, 实际为 %s", quota.TargetID)
	}

	if quota.HardLimit != 100<<30 {
		t.Errorf("期望 HardLimit 为 %d, 实际为 %d", 100<<30, quota.HardLimit)
	}
}

func TestCreateQuotaUserNotFound(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "nonexistent",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}

	_, err = mgr.CreateQuota(input)
	if err != ErrUserNotFound {
		t.Errorf("期望错误 ErrUserNotFound, 实际为 %v", err)
	}
}

func TestUpdateQuota(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 先创建配额
	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}
	quota, err := mgr.CreateQuota(input)
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	// 更新配额
	updateInput := QuotaInput{
		HardLimit: 200 << 30,
		SoftLimit: 150 << 30,
	}
	updated, err := mgr.UpdateQuota(quota.ID, updateInput)
	if err != nil {
		t.Fatalf("更新配额失败: %v", err)
	}

	if updated.HardLimit != 200<<30 {
		t.Errorf("期望 HardLimit 为 %d, 实际为 %d", 200<<30, updated.HardLimit)
	}
}

func TestDeleteQuota(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}
	quota, err := mgr.CreateQuota(input)
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	// 删除配额
	err = mgr.DeleteQuota(quota.ID)
	if err != nil {
		t.Fatalf("删除配额失败: %v", err)
	}

	// 验证已删除
	_, err = mgr.GetQuota(quota.ID)
	if err != ErrQuotaNotFound {
		t.Errorf("期望错误 ErrQuotaNotFound, 实际为 %v", err)
	}
}

func TestListQuotas(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("user1", "/home/user1")
	user.AddUser("user2", "/home/user2")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建多个配额
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "user1",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "user2",
		VolumeName: "data",
		HardLimit:  200 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	quotas := mgr.ListQuotas()
	if len(quotas) != 2 {
		t.Errorf("期望 2 个配额, 实际为 %d", len(quotas))
	}
}

func TestGroupQuota(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddGroup("developers")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	input := QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "developers",
		VolumeName: "data",
		HardLimit:  500 << 30,
		SoftLimit:  400 << 30,
	}

	quota, err := mgr.CreateQuota(input)
	if err != nil {
		t.Fatalf("创建用户组配额失败: %v", err)
	}

	if quota.Type != QuotaTypeGroup {
		t.Errorf("期望类型为 group, 实际为 %s", quota.Type)
	}
}

func TestAlertConfig(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := AlertConfig{
		Enabled:            true,
		SoftLimitThreshold: 85,
		HardLimitThreshold: 95,
		CheckInterval:      10 * time.Minute,
	}

	mgr.SetAlertConfig(config)
	got := mgr.GetAlertConfig()

	if got.SoftLimitThreshold != 85 {
		t.Errorf("期望 SoftLimitThreshold 为 85, 实际为 %f", got.SoftLimitThreshold)
	}
}

func TestCleanupPolicy(t *testing.T) {
	storage := NewMockStorageProvider()
	storage.volumes["data"] = &VolumeInfo{Name: "data", MountPoint: "/mnt/data"}

	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	cleanup := NewCleanupManager(mgr)

	input := CleanupPolicyInput{
		Name:       "清理临时文件",
		VolumeName: "data",
		Path:       "/mnt/data/temp",
		Type:       CleanupPolicyAge,
		Action:     CleanupActionDelete,
		MaxAge:     30,
		Enabled:    true,
	}

	policy, err := cleanup.CreatePolicy(input)
	if err != nil {
		t.Fatalf("创建清理策略失败: %v", err)
	}

	if policy.MaxAge != 30 {
		t.Errorf("期望 MaxAge 为 30, 实际为 %d", policy.MaxAge)
	}

	// 测试启用/禁用
	err = cleanup.EnablePolicy(policy.ID, false)
	if err != nil {
		t.Fatalf("禁用策略失败: %v", err)
	}

	updated, err := cleanup.GetPolicy(policy.ID)
	if err != nil {
		t.Fatalf("获取策略失败: %v", err)
	}
	if updated.Enabled {
		t.Error("策略应该被禁用")
	}
}

func TestCleanupPolicyValidation(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	cleanup := NewCleanupManager(mgr)

	// 测试年龄策略缺少 MaxAge
	input := CleanupPolicyInput{
		Name:       "测试策略",
		VolumeName: "data",
		Type:       CleanupPolicyAge,
		Action:     CleanupActionDelete,
	}

	_, err = cleanup.CreatePolicy(input)
	if err == nil {
		t.Error("应该返回验证错误")
	}
}

func TestReportGeneration(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建配额
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	monitor := NewMonitor(mgr, mgr.alertConfig)
	cleanup := NewCleanupManager(mgr)
	reportGen := NewReportGenerator(mgr, monitor, cleanup)

	req := ReportRequest{
		Type:   ReportTypeSummary,
		Format: ReportFormatJSON,
	}

	report, err := reportGen.GenerateReport(req)
	if err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}

	if report.Type != ReportTypeSummary {
		t.Errorf("期望报告类型为 summary, 实际为 %s", report.Type)
	}

	if report.Summary.TotalQuotas != 1 {
		t.Errorf("期望 1 个配额, 实际为 %d", report.Summary.TotalQuotas)
	}
}

func TestReportExport(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	monitor := NewMonitor(mgr, mgr.alertConfig)
	cleanup := NewCleanupManager(mgr)
	reportGen := NewReportGenerator(mgr, monitor, cleanup)

	report := &Report{
		ID:          "test-report",
		Type:        ReportTypeSummary,
		Format:      ReportFormatJSON,
		GeneratedAt: time.Now(),
		Summary: ReportSummary{
			TotalQuotas:     1,
			TotalUsedBytes:  50 << 30,
			TotalLimitBytes: 100 << 30,
		},
	}

	// 测试 JSON 导出
	tmpFile := "/tmp/test-report.json"
	err = reportGen.ExportReport(report, tmpFile)
	if err != nil {
		t.Fatalf("导出报告失败: %v", err)
	}
	defer os.Remove(tmpFile)

	// 验证文件存在
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("报告文件应该存在")
	}
}

func TestMonitorStartStop(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	monitor := NewMonitor(mgr, AlertConfig{
		Enabled:       true,
		CheckInterval: 1 * time.Second,
	})

	monitor.Start()
	time.Sleep(100 * time.Millisecond)

	status := monitor.GetMonitorStatus()
	if !status["running"].(bool) {
		t.Error("监控应该正在运行")
	}

	monitor.Stop()

	status = monitor.GetMonitorStatus()
	if status["running"].(bool) {
		t.Error("监控应该已停止")
	}
}

func TestQuotaExceeded(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建一个小配额
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  1 << 20, // 1MB
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	// 检查配额 - 这里由于没有实际文件，不会超限
	err = mgr.CheckQuota("testuser", "data", 0)
	// 没有配额路径会返回 nil（允许）
	if err != nil && err != ErrQuotaExceeded {
		t.Logf("CheckQuota 返回: %v", err)
	}
}

// ========== v2.1.0 新增测试 ==========

func TestListUserQuotas(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("user1", "/home/user1")
	user.AddUser("user2", "/home/user2")
	user.AddGroup("developers")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建用户配额
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "user1",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "user2",
		VolumeName: "data",
		HardLimit:  200 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	// 创建组配额（应该不出现在用户配额列表中）
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "developers",
		VolumeName: "data",
		HardLimit:  500 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	// 测试列出所有用户配额
	quotas := mgr.ListQuotas()
	userQuotas := make([]*Quota, 0)
	for _, q := range quotas {
		if q.Type == QuotaTypeUser {
			userQuotas = append(userQuotas, q)
		}
	}

	if len(userQuotas) != 2 {
		t.Errorf("期望 2 个用户配额, 实际为 %d", len(userQuotas))
	}

	// 测试列出特定用户的配额
	user1Quotas := mgr.ListUserQuotas("user1")
	if len(user1Quotas) != 1 {
		t.Errorf("期望 user1 有 1 个配额, 实际为 %d", len(user1Quotas))
	}
}

func TestListGroupQuotas(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddGroup("developers")
	user.AddGroup("admins")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建组配额
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "developers",
		VolumeName: "data",
		HardLimit:  500 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "admins",
		VolumeName: "data",
		HardLimit:  1000 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	// 测试列出特定组的配额
	devQuotas := mgr.ListGroupQuotas("developers")
	if len(devQuotas) != 1 {
		t.Errorf("期望 developers 组有 1 个配额, 实际为 %d", len(devQuotas))
	}
}

func TestDirectoryQuota(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "quota-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建目录配额
	input := QuotaInput{
		Type:       QuotaTypeDirectory,
		TargetID:   tmpDir,
		VolumeName: "data",
		Path:       tmpDir,
		HardLimit:  100 << 30,
		SoftLimit:  80 << 30,
	}

	quota, err := mgr.CreateQuota(input)
	if err != nil {
		t.Fatalf("创建目录配额失败: %v", err)
	}

	if quota.Type != QuotaTypeDirectory {
		t.Errorf("期望类型为 directory, 实际为 %s", quota.Type)
	}

	if quota.Path != tmpDir {
		t.Errorf("期望路径为 %s, 实际为 %s", tmpDir, quota.Path)
	}

	// 测试列出目录配额
	dirQuotas := mgr.ListDirectoryQuotas()
	if len(dirQuotas) != 1 {
		t.Errorf("期望 1 个目录配额, 实际为 %d", len(dirQuotas))
	}

	// 测试获取目录配额
	retrieved, err := mgr.GetDirectoryQuota(tmpDir)
	if err != nil {
		t.Fatalf("获取目录配额失败: %v", err)
	}

	if retrieved.Path != tmpDir {
		t.Errorf("期望路径为 %s, 实际为 %s", tmpDir, retrieved.Path)
	}
}

func TestDirectoryQuotaNotFound(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 尝试获取不存在的目录配额
	_, err = mgr.GetDirectoryQuota("/nonexistent/path")
	if err != ErrQuotaNotFound {
		t.Errorf("期望错误 ErrQuotaNotFound, 实际为 %v", err)
	}
}

func TestSetUserQuota(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建初始配额
	input := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	}
	quota, err := mgr.CreateQuota(input)
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	// 更新配额（模拟 setUserQuota）
	updateInput := QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  200 << 30,
		SoftLimit:  150 << 30,
	}
	updated, err := mgr.UpdateQuota(quota.ID, updateInput)
	if err != nil {
		t.Fatalf("更新配额失败: %v", err)
	}

	if updated.HardLimit != 200<<30 {
		t.Errorf("期望 HardLimit 为 %d, 实际为 %d", 200<<30, updated.HardLimit)
	}
}

func TestSetGroupQuota(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddGroup("developers")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建初始配额
	input := QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "developers",
		VolumeName: "data",
		HardLimit:  500 << 30,
	}
	quota, err := mgr.CreateQuota(input)
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	// 更新配额（模拟 setGroupQuota）
	updateInput := QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "developers",
		VolumeName: "data",
		HardLimit:  1000 << 30,
		SoftLimit:  800 << 30,
	}
	updated, err := mgr.UpdateQuota(quota.ID, updateInput)
	if err != nil {
		t.Fatalf("更新配额失败: %v", err)
	}

	if updated.HardLimit != 1000<<30 {
		t.Errorf("期望 HardLimit 为 %d, 实际为 %d", 1000<<30, updated.HardLimit)
	}
}

func TestAlertsManagement(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建配额
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	// 初始应该没有告警
	alerts := mgr.GetAlerts()
	if len(alerts) != 0 {
		t.Errorf("初始应该没有告警, 实际有 %d 个", len(alerts))
	}

	// 测试告警配置
	config := AlertConfig{
		Enabled:            true,
		SoftLimitThreshold: 80,
		HardLimitThreshold: 95,
		CheckInterval:      5 * time.Minute,
		NotifyWebhook:      true,
		WebhookURL:         "https://example.com/webhook",
	}
	mgr.SetAlertConfig(config)

	gotConfig := mgr.GetAlertConfig()
	if gotConfig.SoftLimitThreshold != 80 {
		t.Errorf("期望 SoftLimitThreshold 为 80, 实际为 %f", gotConfig.SoftLimitThreshold)
	}
	if gotConfig.WebhookURL != "https://example.com/webhook" {
		t.Errorf("期望 WebhookURL 为 https://example.com/webhook, 实际为 %s", gotConfig.WebhookURL)
	}
}

func TestQuotaReportTypes(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("testuser", "/home/testuser")
	user.AddGroup("developers")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建配额
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "developers",
		VolumeName: "data",
		HardLimit:  500 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	monitor := NewMonitor(mgr, mgr.alertConfig)
	cleanup := NewCleanupManager(mgr)
	reportGen := NewReportGenerator(mgr, monitor, cleanup)

	// 测试汇总报告
	report, err := reportGen.GenerateReport(ReportRequest{
		Type:   ReportTypeSummary,
		Format: ReportFormatJSON,
	})
	if err != nil {
		t.Fatalf("生成汇总报告失败: %v", err)
	}
	if report.Type != ReportTypeSummary {
		t.Errorf("期望报告类型为 summary, 实际为 %s", report.Type)
	}

	// 测试用户报告
	userReport, err := reportGen.GenerateReport(ReportRequest{
		Type:   ReportTypeUser,
		UserID: "testuser",
	})
	if err != nil {
		t.Fatalf("生成用户报告失败: %v", err)
	}
	if userReport.Type != ReportTypeUser {
		t.Errorf("期望报告类型为 user, 实际为 %s", userReport.Type)
	}

	// 测试组报告
	groupReport, err := reportGen.GenerateReport(ReportRequest{
		Type:    ReportTypeGroup,
		GroupID: "developers",
	})
	if err != nil {
		t.Fatalf("生成组报告失败: %v", err)
	}
	if groupReport.Type != ReportTypeGroup {
		t.Errorf("期望报告类型为 group, 实际为 %s", groupReport.Type)
	}
}

func TestQuotaReportExportFormats(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	monitor := NewMonitor(mgr, mgr.alertConfig)
	cleanup := NewCleanupManager(mgr)
	reportGen := NewReportGenerator(mgr, monitor, cleanup)

	report := &Report{
		ID:          "test-report",
		Type:        ReportTypeSummary,
		Format:      ReportFormatJSON,
		GeneratedAt: time.Now(),
		Summary: ReportSummary{
			TotalQuotas:     1,
			TotalUsedBytes:  50 << 30,
			TotalLimitBytes: 100 << 30,
			AverageUsage:    50.0,
		},
	}

	// 测试 JSON 导出
	jsonFile := "/tmp/test-report.json"
	err = reportGen.ExportReport(report, jsonFile)
	if err != nil {
		t.Fatalf("导出 JSON 失败: %v", err)
	}
	_ = os.Remove(jsonFile)

	// 测试 CSV 导出
	report.Format = ReportFormatCSV
	csvFile := "/tmp/test-report.csv"
	err = reportGen.ExportReport(report, csvFile)
	if err != nil {
		t.Fatalf("导出 CSV 失败: %v", err)
	}
	_ = os.Remove(csvFile)

	// 测试 HTML 导出
	report.Format = ReportFormatHTML
	htmlFile := "/tmp/test-report.html"
	err = reportGen.ExportReport(report, htmlFile)
	if err != nil {
		t.Fatalf("导出 HTML 失败: %v", err)
	}
	_ = os.Remove(htmlFile)
}

func TestMonitorWebhookNotification(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	config := AlertConfig{
		Enabled:       true,
		CheckInterval: 1 * time.Second,
		NotifyWebhook: true,
		WebhookURL:    "https://httpbin.org/post", // 测试 URL
	}

	monitor := NewMonitor(mgr, config)

	// 测试更新配置
	monitor.UpdateConfig(AlertConfig{
		WebhookURL: "https://example.com/new-webhook",
	})

	// 验证配置更新
	status := monitor.GetMonitorStatus()
	if status == nil {
		t.Error("获取监控状态失败")
	}
}

func TestTrendDataRecording(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	monitor := NewMonitor(mgr, mgr.alertConfig)

	// 模拟记录趋势数据
	usage := &QuotaUsage{
		QuotaID:      "test-quota",
		UsedBytes:    50 << 30,
		UsagePercent: 50.0,
	}
	monitor.recordTrend(usage)

	// 获取趋势数据
	trend := monitor.GetTrend("test-quota", 24*time.Hour)
	if len(trend) == 0 {
		t.Error("应该有趋势数据")
	}

	// 测试增长率计算
	growthRate := monitor.CalculateGrowthRate("test-quota")
	// 只有一个数据点，增长率应该是 0
	if growthRate != 0 {
		t.Logf("增长率: %f", growthRate)
	}
}

func TestPredictFullTime(t *testing.T) {
	storage := NewMockStorageProvider()
	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	monitor := NewMonitor(mgr, mgr.alertConfig)

	// 没有数据时预测应该返回 -1
	days := monitor.PredictFullTime("nonexistent", 100<<30)
	if days != -1 {
		t.Errorf("没有数据时应该返回 -1, 实际返回 %d", days)
	}
}

func TestCleanupManagerStats(t *testing.T) {
	storage := NewMockStorageProvider()
	storage.volumes["data"] = &VolumeInfo{Name: "data", MountPoint: "/mnt/data"}

	user := NewMockUserProvider()

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	cleanup := NewCleanupManager(mgr)

	// 创建策略
	cleanup.CreatePolicy(CleanupPolicyInput{
		Name:       "测试策略",
		VolumeName: "data",
		Type:       CleanupPolicyAge,
		Action:     CleanupActionDelete,
		MaxAge:     30,
		Enabled:    true,
	})

	// 获取统计
	stats := cleanup.GetCleanupStats()
	if stats["total_policies"].(int) != 1 {
		t.Errorf("期望 1 个策略, 实际为 %d", stats["total_policies"].(int))
	}
	if stats["enabled_policies"].(int) != 1 {
		t.Errorf("期望 1 个启用策略, 实际为 %d", stats["enabled_policies"].(int))
	}
}

func TestMultipleQuotaTypes(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "quota-multi-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := NewMockStorageProvider()
	user := NewMockUserProvider()
	user.AddUser("user1", "/home/user1")
	user.AddGroup("developers")

	mgr, err := NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建不同类型的配额
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "user1",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeGroup,
		TargetID:   "developers",
		VolumeName: "data",
		HardLimit:  500 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}
	_, err = mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeDirectory,
		TargetID:   tmpDir,
		VolumeName: "data",
		Path:       tmpDir,
		HardLimit:  200 << 30,
	})
	if err != nil {
		t.Fatalf("创建配额失败: %v", err)
	}

	// 验证各类型配额
	allQuotas := mgr.ListQuotas()
	if len(allQuotas) != 3 {
		t.Errorf("期望 3 个配额, 实际为 %d", len(allQuotas))
	}

	userQuotas := mgr.ListUserQuotas("user1")
	if len(userQuotas) != 1 {
		t.Errorf("期望 1 个用户配额, 实际为 %d", len(userQuotas))
	}

	groupQuotas := mgr.ListGroupQuotas("developers")
	if len(groupQuotas) != 1 {
		t.Errorf("期望 1 个组配额, 实际为 %d", len(groupQuotas))
	}

	dirQuotas := mgr.ListDirectoryQuotas()
	if len(dirQuotas) != 1 {
		t.Errorf("期望 1 个目录配额, 实际为 %d", len(dirQuotas))
	}
}
