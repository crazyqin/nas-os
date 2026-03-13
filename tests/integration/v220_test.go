// Package integration 提供 NAS-OS v2.2.0 集成测试
package integration

import (
	"os"
	"sync"
	"testing"
	"time"

	"nas-os/internal/quota"
	"nas-os/internal/webdav"
)

// ========== WebDAV 集成测试 ==========

func TestWebDAV_FullIntegration(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "webdav-integration-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建 WebDAV 服务器配置
	config := &webdav.Config{
		Enabled:       true,
		Port:          18081,
		RootPath:      tmpDir,
		AllowGuest:    false,
		MaxUploadSize: 10 * 1024 * 1024,
	}

	// 创建服务器
	srv, err := webdav.NewServer(config)
	if err != nil {
		t.Fatalf("创建 WebDAV 服务器失败: %v", err)
	}

	// 验证初始配置
	gotConfig := srv.GetConfig()
	if gotConfig.Port != 18081 {
		t.Errorf("期望端口 18081, 实际为 %d", gotConfig.Port)
	}

	// 测试锁功能
	t.Run("LockWorkflow", func(t *testing.T) {
		// 创建锁
		lock, err := srv.CreateLock("/test/file.txt", "owner-1", true, 3600)
		if err != nil {
			t.Fatalf("创建锁失败: %v", err)
		}

		// 获取锁
		gotLock, err := srv.GetLock(lock.Token)
		if err != nil {
			t.Fatalf("获取锁失败: %v", err)
		}

		if gotLock.Path != "/test/file.txt" {
			t.Errorf("期望路径 /test/file.txt, 实际为 %s", gotLock.Path)
		}

		// 删除锁
		err = srv.RemoveLock(lock.Token)
		if err != nil {
			t.Fatalf("删除锁失败: %v", err)
		}

		// 验证锁已删除
		_, err = srv.GetLock(lock.Token)
		if err == nil {
			t.Error("期望锁已删除")
		}
	})

	// 测试配置更新
	t.Run("ConfigUpdate", func(t *testing.T) {
		newConfig := &webdav.Config{
			Enabled:       true,
			Port:          19090,
			RootPath:      tmpDir,
			AllowGuest:    true,
			MaxUploadSize: 20 * 1024 * 1024,
		}

		err := srv.UpdateConfig(newConfig)
		if err != nil {
			t.Fatalf("更新配置失败: %v", err)
		}

		updated := srv.GetConfig()
		if updated.Port != 19090 {
			t.Errorf("期望端口 19090, 实际为 %d", updated.Port)
		}

		if !updated.AllowGuest {
			t.Error("期望 AllowGuest=true")
		}
	})

	// 测试状态获取
	t.Run("GetStatus", func(t *testing.T) {
		status := srv.GetStatus()
		if status == nil {
			t.Fatal("状态不应为 nil")
		}

		if _, ok := status["enabled"]; !ok {
			t.Error("状态应该包含 enabled")
		}

		if _, ok := status["running"]; !ok {
			t.Error("状态应该包含 running")
		}
	})
}

func TestWebDAV_ConcurrentLocks(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webdav-concurrent-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := &webdav.Config{
		Enabled:    true,
		Port:       18082,
		RootPath:   tmpDir,
		AllowGuest: false,
	}

	srv, err := webdav.NewServer(config)
	if err != nil {
		t.Fatalf("创建服务器失败: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// 并发创建锁
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			path := "/test/file-" + string(rune('A'+id%26)) + ".txt"
			_, err := srv.CreateLock(path, "owner-"+string(rune('A'+id%26)), true, 3600)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("并发创建锁错误: %v", err)
	}
}

// ========== 配额管理集成测试 ==========

func TestQuota_FullIntegration(t *testing.T) {
	// 创建 Mock 存储
	storage := quota.NewMockStorageProvider()
	storage.volumes["data"] = &quota.VolumeInfo{
		Name:       "data",
		MountPoint: "/mnt/data",
		Size:       1 << 40, // 1TB
		Used:       500 << 30,
		Free:       500 << 30,
	}

	// 创建 Mock 用户
	user := quota.NewMockUserProvider()
	user.AddUser("user1", "/home/user1")
	user.AddUser("user2", "/home/user2")
	user.AddGroup("developers")

	// 创建配额管理器
	mgr, err := quota.NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 测试用户配额流程
	t.Run("UserQuotaWorkflow", func(t *testing.T) {
		// 创建用户配额
		input := quota.QuotaInput{
			Type:       quota.QuotaTypeUser,
			TargetID:   "user1",
			VolumeName: "data",
			HardLimit:  100 << 30, // 100GB
			SoftLimit:  80 << 30,  // 80GB
		}

		q, err := mgr.CreateQuota(input)
		if err != nil {
			t.Fatalf("创建用户配额失败: %v", err)
		}

		// 获取配额
		got, err := mgr.GetQuota(q.ID)
		if err != nil {
			t.Fatalf("获取配额失败: %v", err)
		}

		if got.TargetID != "user1" {
			t.Errorf("期望 TargetID=user1, 实际为 %s", got.TargetID)
		}

		// 更新配额
		updateInput := quota.QuotaInput{
			HardLimit: 200 << 30,
			SoftLimit: 150 << 30,
		}
		updated, err := mgr.UpdateQuota(q.ID, updateInput)
		if err != nil {
			t.Fatalf("更新配额失败: %v", err)
		}

		if updated.HardLimit != 200<<30 {
			t.Errorf("期望 HardLimit=%d, 实际为 %d", 200<<30, updated.HardLimit)
		}

		// 删除配额
		err = mgr.DeleteQuota(q.ID)
		if err != nil {
			t.Fatalf("删除配额失败: %v", err)
		}
	})

	// 测试组配额流程
	t.Run("GroupQuotaWorkflow", func(t *testing.T) {
		input := quota.QuotaInput{
			Type:       quota.QuotaTypeGroup,
			TargetID:   "developers",
			VolumeName: "data",
			HardLimit:  500 << 30,
			SoftLimit:  400 << 30,
		}

		q, err := mgr.CreateQuota(input)
		if err != nil {
			t.Fatalf("创建组配额失败: %v", err)
		}

		if q.Type != quota.QuotaTypeGroup {
			t.Errorf("期望类型为 group, 实际为 %s", q.Type)
		}
	})

	// 测试目录配额流程
	t.Run("DirectoryQuotaWorkflow", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "quota-dir-*")
		if err != nil {
			t.Fatalf("创建临时目录失败: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		input := quota.QuotaInput{
			Type:       quota.QuotaTypeDirectory,
			TargetID:   tmpDir,
			VolumeName: "data",
			Path:       tmpDir,
			HardLimit:  200 << 30,
		}

		q, err := mgr.CreateQuota(input)
		if err != nil {
			t.Fatalf("创建目录配额失败: %v", err)
		}

		if q.Type != quota.QuotaTypeDirectory {
			t.Errorf("期望类型为 directory, 实际为 %s", q.Type)
		}
	})

	// 测试告警配置
	t.Run("AlertConfig", func(t *testing.T) {
		config := quota.AlertConfig{
			Enabled:            true,
			SoftLimitThreshold: 85,
			HardLimitThreshold: 95,
			CheckInterval:      5 * time.Minute,
			NotifyWebhook:      true,
			WebhookURL:         "https://example.com/webhook",
		}

		mgr.SetAlertConfig(config)
		got := mgr.GetAlertConfig()

		if got.SoftLimitThreshold != 85 {
			t.Errorf("期望 SoftLimitThreshold=85, 实际为 %f", got.SoftLimitThreshold)
		}
	})
}

func TestQuota_MultipleQuotaTypes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "quota-multi-*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	storage := quota.NewMockStorageProvider()
	user := quota.NewMockUserProvider()
	user.AddUser("user1", "/home/user1")
	user.AddGroup("developers")

	mgr, err := quota.NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建不同类型的配额
	mgr.CreateQuota(quota.QuotaInput{
		Type:       quota.QuotaTypeUser,
		TargetID:   "user1",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})

	mgr.CreateQuota(quota.QuotaInput{
		Type:       quota.QuotaTypeGroup,
		TargetID:   "developers",
		VolumeName: "data",
		HardLimit:  500 << 30,
	})

	mgr.CreateQuota(quota.QuotaInput{
		Type:       quota.QuotaTypeDirectory,
		TargetID:   tmpDir,
		VolumeName: "data",
		Path:       tmpDir,
		HardLimit:  200 << 30,
	})

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

func TestQuota_CleanupPolicyIntegration(t *testing.T) {
	storage := quota.NewMockStorageProvider()
	storage.volumes["data"] = &quota.VolumeInfo{
		Name:       "data",
		MountPoint: "/mnt/data",
	}

	user := quota.NewMockUserProvider()

	mgr, err := quota.NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	cleanup := quota.NewCleanupManager(mgr)

	// 创建清理策略
	input := quota.CleanupPolicyInput{
		Name:       "清理临时文件",
		VolumeName: "data",
		Path:       "/mnt/data/temp",
		Type:       quota.CleanupPolicyAge,
		Action:     quota.CleanupActionDelete,
		MaxAge:     30,
		Enabled:    true,
	}

	policy, err := cleanup.CreatePolicy(input)
	if err != nil {
		t.Fatalf("创建清理策略失败: %v", err)
	}

	// 验证策略
	if policy.MaxAge != 30 {
		t.Errorf("期望 MaxAge=30, 实际为 %d", policy.MaxAge)
	}

	// 禁用策略
	err = cleanup.EnablePolicy(policy.ID, false)
	if err != nil {
		t.Fatalf("禁用策略失败: %v", err)
	}

	updated, _ := cleanup.GetPolicy(policy.ID)
	if updated.Enabled {
		t.Error("策略应该被禁用")
	}

	// 获取统计
	stats := cleanup.GetCleanupStats()
	if stats["total_policies"].(int) != 1 {
		t.Errorf("期望 1 个策略, 实际为 %d", stats["total_policies"].(int))
	}
}

func TestQuota_ReportGeneration(t *testing.T) {
	storage := quota.NewMockStorageProvider()
	user := quota.NewMockUserProvider()
	user.AddUser("user1", "/home/user1")

	mgr, err := quota.NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	// 创建配额
	mgr.CreateQuota(quota.QuotaInput{
		Type:       quota.QuotaTypeUser,
		TargetID:   "user1",
		VolumeName: "data",
		HardLimit:  100 << 30,
	})

	monitor := quota.NewMonitor(mgr, mgr.alertConfig)
	cleanup := quota.NewCleanupManager(mgr)
	reportGen := quota.NewReportGenerator(mgr, monitor, cleanup)

	// 生成汇总报告
	report, err := reportGen.GenerateReport(quota.ReportRequest{
		Type:   quota.ReportTypeSummary,
		Format: quota.ReportFormatJSON,
	})
	if err != nil {
		t.Fatalf("生成报告失败: %v", err)
	}

	if report.Type != quota.ReportTypeSummary {
		t.Errorf("期望报告类型为 summary, 实际为 %s", report.Type)
	}

	if report.Summary.TotalQuotas != 1 {
		t.Errorf("期望 1 个配额, 实际为 %d", report.Summary.TotalQuotas)
	}

	// 测试导出
	tmpFile := "/tmp/test-report-" + time.Now().Format("20060102150405") + ".json"
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

// ========== 并发集成测试 ==========

func TestQuota_ConcurrentOperations(t *testing.T) {
	storage := quota.NewMockStorageProvider()
	user := quota.NewMockUserProvider()
	user.AddUser("user1", "/home/user1")

	mgr, err := quota.NewManager("", storage, user)
	if err != nil {
		t.Fatalf("创建 Manager 失败: %v", err)
	}

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// 并发创建配额
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			input := quota.QuotaInput{
				Type:       quota.QuotaTypeUser,
				TargetID:   "user1",
				VolumeName: "data",
				HardLimit:  int64(100+id) << 30,
			}
			_, err := mgr.CreateQuota(input)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	// 并发读取配额
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.ListQuotas()
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("并发操作错误: %v", err)
	}
}