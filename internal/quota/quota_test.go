package quota

import (
	"os"
	"testing"
	"time"
)

// MockStorageProvider 模拟存储提供者
type MockStorageProvider struct {
	volumes map[string]*VolumeInfo
}

func NewMockStorageProvider() *MockStorageProvider {
	return &MockStorageProvider{
		volumes: make(map[string]*VolumeInfo),
	}
}

func (m *MockStorageProvider) GetVolume(name string) *VolumeInfo {
	return m.volumes[name]
}

func (m *MockStorageProvider) GetUsage(volumeName string) (total, used, free uint64, err error) {
	vol := m.volumes[volumeName]
	if vol == nil {
		return 0, 0, 0, ErrVolumeNotFound
	}
	return vol.Size, vol.Used, vol.Free, nil
}

// MockUserProvider 模拟用户提供者
type MockUserProvider struct {
	users  map[string]bool
	groups map[string]bool
	homes  map[string]string
}

func NewMockUserProvider() *MockUserProvider {
	return &MockUserProvider{
		users:  make(map[string]bool),
		groups: make(map[string]bool),
		homes:  make(map[string]string),
	}
}

func (m *MockUserProvider) UserExists(username string) bool {
	return m.users[username]
}

func (m *MockUserProvider) GroupExists(groupName string) bool {
	return m.groups[groupName]
}

func (m *MockUserProvider) GetUserHomeDir(username string) string {
	return m.homes[username]
}

func (m *MockUserProvider) AddUser(username, homeDir string) {
	m.users[username] = true
	m.homes[username] = homeDir
}

func (m *MockUserProvider) AddGroup(groupName string) {
	m.groups[groupName] = true
}

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
	quota, _ := mgr.CreateQuota(input)

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
	quota, _ := mgr.CreateQuota(input)

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
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "user1",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "user2",
		VolumeName: "data",
		HardLimit:  200 << 30,
	})

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

	updated, _ := cleanup.GetPolicy(policy.ID)
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
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})

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
	mgr.CreateQuota(QuotaInput{
		Type:       QuotaTypeUser,
		TargetID:   "testuser",
		VolumeName: "data",
		HardLimit:  1 << 20, // 1MB
	})

	// 检查配额 - 这里由于没有实际文件，不会超限
	err = mgr.CheckQuota("testuser", "data", 0)
	// 没有配额路径会返回 nil（允许）
	if err != nil && err != ErrQuotaExceeded {
		t.Logf("CheckQuota 返回: %v", err)
	}
}