// Package smartbackupverify 提供备份智能验证功能
package smartbackupverify

import (
	"testing"
	"time"
)

// ========== Manager 基础测试 ==========

func TestNewManager(t *testing.T) {
	mgr := NewManager(nil)
	if mgr == nil {
		t.Fatal("Manager should not be nil")
	}
}

// ========== 备份管理测试 ==========

func TestRegisterBackup(t *testing.T) {
	mgr := NewManager(nil)

	req := BackupRegisterRequest{
		TaskID:   "task-001",
		Source:   "/data/documents",
		DestPath: "/backup/documents-20240101",
		Size:     1024 * 1024 * 100, // 100MB
		Checksum: "abc123def456",
	}

	backup, err := mgr.RegisterBackup(req)
	if err != nil {
		t.Fatalf("RegisterBackup failed: %v", err)
	}
	if backup.ID == "" {
		t.Error("backup ID should not be empty")
	}
	if backup.TaskID != "task-001" {
		t.Errorf("expected task_id task-001, got %s", backup.TaskID)
	}
	if backup.Source != "/data/documents" {
		t.Errorf("expected source /data/documents, got %s", backup.Source)
	}
}

func TestGetBackup(t *testing.T) {
	mgr := NewManager(nil)

	backup, _ := mgr.RegisterBackup(BackupRegisterRequest{
		TaskID:   "task-001",
		Source:   "/data/test",
		DestPath: "/backup/test",
	})

	got, err := mgr.GetBackup(backup.ID)
	if err != nil {
		t.Fatalf("GetBackup failed: %v", err)
	}
	if got.ID != backup.ID {
		t.Errorf("expected ID %s, got %s", backup.ID, got.ID)
	}
}

func TestGetBackup_NotFound(t *testing.T) {
	mgr := NewManager(nil)

	_, err := mgr.GetBackup("nonexistent")
	if err != ErrBackupNotFound {
		t.Errorf("expected ErrBackupNotFound, got: %v", err)
	}
}

func TestListBackups(t *testing.T) {
	mgr := NewManager(nil)

	mgr.RegisterBackup(BackupRegisterRequest{TaskID: "t1", Source: "/data1", DestPath: "/backup1"})
	mgr.RegisterBackup(BackupRegisterRequest{TaskID: "t2", Source: "/data2", DestPath: "/backup2"})

	backups := mgr.ListBackups()
	if len(backups) != 2 {
		t.Errorf("expected 2 backups, got %d", len(backups))
	}
}

// ========== 验证任务测试 ==========

func TestRunVerification(t *testing.T) {
	mgr := NewManager(nil)

	backup, _ := mgr.RegisterBackup(BackupRegisterRequest{
		TaskID:   "task-001",
		Source:   "/data/test",
		DestPath: "/backup/test",
		Checksum: "abc123",
	})

	req := VerifyRequest{
		BackupID:       backup.ID,
		RunRestoreTest: false,
	}

	task, err := mgr.RunVerification(req)
	if err != nil {
		t.Fatalf("RunVerification failed: %v", err)
	}
	if task.ID == "" {
		t.Error("task ID should not be empty")
	}
	if task.BackupID != backup.ID {
		t.Errorf("expected backup_id %s, got %s", backup.ID, task.BackupID)
	}

	// 等待异步验证完成
	time.Sleep(500 * time.Millisecond)

	// 重新获取任务状态
	mgr.mu.RLock()
	updatedTask := mgr.verifyTasks[task.ID]
	mgr.mu.RUnlock()

	if updatedTask.Status != VerifyStatusPassed {
		t.Errorf("expected status passed, got %s", updatedTask.Status)
	}
	if len(updatedTask.Checks) < 4 {
		t.Errorf("expected at least 4 checks, got %d", len(updatedTask.Checks))
	}
}

func TestRunVerification_WithRestoreTest(t *testing.T) {
	mgr := NewManager(nil)

	backup, _ := mgr.RegisterBackup(BackupRegisterRequest{
		TaskID:   "task-002",
		Source:   "/data/test",
		DestPath: "/backup/test",
		Checksum: "abc123",
	})

	task, err := mgr.RunVerification(VerifyRequest{
		BackupID:       backup.ID,
		RunRestoreTest: true,
	})
	if err != nil {
		t.Fatalf("RunVerification failed: %v", err)
	}

	// 等待异步验证完成
	time.Sleep(500 * time.Millisecond)

	mgr.mu.RLock()
	updatedTask := mgr.verifyTasks[task.ID]
	mgr.mu.RUnlock()

	// 应该包含恢复测试检查项
	hasRestoreTest := false
	for _, check := range updatedTask.Checks {
		if check.Name == "恢复测试" {
			hasRestoreTest = true
			break
		}
	}
	if !hasRestoreTest {
		t.Error("expected restore test check item")
	}
}

func TestRunVerification_BackupNotFound(t *testing.T) {
	mgr := NewManager(nil)

	_, err := mgr.RunVerification(VerifyRequest{
		BackupID: "nonexistent",
	})
	if err != ErrBackupNotFound {
		t.Errorf("expected ErrBackupNotFound, got: %v", err)
	}
}

// ========== 恢复测试测试 ==========

func TestGetRestoreTest(t *testing.T) {
	mgr := NewManager(nil)

	backup, _ := mgr.RegisterBackup(BackupRegisterRequest{
		TaskID:   "task-001",
		Source:   "/data/test",
		DestPath: "/backup/test",
	})

	// 触发带恢复测试的验证
	mgr.RunVerification(VerifyRequest{
		BackupID:       backup.ID,
		RunRestoreTest: true,
	})

	// 等待完成
	time.Sleep(500 * time.Millisecond)

	// 获取恢复测试结果
	mgr.mu.RLock()
	var testID string
	for _, t := range mgr.restoreTests {
		testID = t.ID
		break
	}
	mgr.mu.RUnlock()

	if testID == "" {
		t.Skip("no restore test found")
	}

	result, err := mgr.GetRestoreTest(testID)
	if err != nil {
		t.Fatalf("GetRestoreTest failed: %v", err)
	}
	if !result.Success {
		t.Error("expected restore test to succeed")
	}
}

// ========== 健康度评分测试 ==========

func TestGetHealthScore(t *testing.T) {
	mgr := NewManager(nil)

	backup, _ := mgr.RegisterBackup(BackupRegisterRequest{
		TaskID:   "task-001",
		Source:   "/data/test",
		DestPath: "/backup/test",
		Checksum: "abc123",
	})

	// 运行验证以生成健康度评分
	mgr.RunVerification(VerifyRequest{BackupID: backup.ID})
	time.Sleep(500 * time.Millisecond)

	score, err := mgr.GetHealthScore(backup.ID)
	if err != nil {
		t.Fatalf("GetHealthScore failed: %v", err)
	}
	if score.Score < 0 || score.Score > 100 {
		t.Errorf("expected score between 0-100, got %d", score.Score)
	}
	if score.Level == "" {
		t.Error("health level should not be empty")
	}
}

func TestGetHealthScore_NotFound(t *testing.T) {
	mgr := NewManager(nil)

	_, err := mgr.GetHealthScore("nonexistent")
	if err != ErrBackupNotFound {
		t.Errorf("expected ErrBackupNotFound, got: %v", err)
	}
}

// ========== 报告测试 ==========

func TestGetReport(t *testing.T) {
	mgr := NewManager(nil)

	backup, _ := mgr.RegisterBackup(BackupRegisterRequest{
		TaskID:   "task-001",
		Source:   "/data/test",
		DestPath: "/backup/test",
		Checksum: "abc123",
	})

	mgr.RunVerification(VerifyRequest{BackupID: backup.ID})
	time.Sleep(500 * time.Millisecond)

	reports := mgr.ListReports()
	if len(reports) == 0 {
		t.Skip("no reports generated")
	}

	report, err := mgr.GetReport(reports[0].ID)
	if err != nil {
		t.Fatalf("GetReport failed: %v", err)
	}
	if report.Summary.TotalChecks == 0 {
		t.Error("expected total checks > 0")
	}
}

func TestListReports(t *testing.T) {
	mgr := NewManager(nil)

	backup, _ := mgr.RegisterBackup(BackupRegisterRequest{
		TaskID:   "task-001",
		Source:   "/data/test",
		DestPath: "/backup/test",
		Checksum: "abc123",
	})

	mgr.RunVerification(VerifyRequest{BackupID: backup.ID})
	time.Sleep(500 * time.Millisecond)

	reports := mgr.ListReports()
	if len(reports) < 1 {
		t.Errorf("expected at least 1 report, got %d", len(reports))
	}
}

func TestGetReport_NotFound(t *testing.T) {
	mgr := NewManager(nil)

	_, err := mgr.GetReport("nonexistent")
	if err != ErrReportNotFound {
		t.Errorf("expected ErrReportNotFound, got: %v", err)
	}
}

// ========== 告警测试 ==========

func TestCreateAlert(t *testing.T) {
	mgr := NewManager(nil)

	alert := mgr.CreateAlert("backup-001", AlertSeverityWarning, "测试告警", "这是一条测试告警")
	if alert.ID == "" {
		t.Error("alert ID should not be empty")
	}
	if alert.Severity != AlertSeverityWarning {
		t.Errorf("expected severity warning, got %s", alert.Severity)
	}
	if alert.Resolved {
		t.Error("new alert should not be resolved")
	}
}

func TestListAlerts(t *testing.T) {
	mgr := NewManager(nil)

	mgr.CreateAlert("b1", AlertSeverityInfo, "告警1", "消息1")
	mgr.CreateAlert("b2", AlertSeverityError, "告警2", "消息2")

	alerts := mgr.ListAlerts()
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts, got %d", len(alerts))
	}
}

func TestResolveAlert(t *testing.T) {
	mgr := NewManager(nil)

	alert := mgr.CreateAlert("backup-001", AlertSeverityWarning, "测试告警", "消息")

	err := mgr.ResolveAlert(alert.ID)
	if err != nil {
		t.Fatalf("ResolveAlert failed: %v", err)
	}

	alerts := mgr.ListAlerts()
	for _, a := range alerts {
		if a.ID == alert.ID {
			if !a.Resolved {
				t.Error("alert should be resolved")
			}
		}
	}
}

func TestResolveAlert_NotFound(t *testing.T) {
	mgr := NewManager(nil)

	err := mgr.ResolveAlert("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	mgr := NewManager(nil)

	mgr.RegisterBackup(BackupRegisterRequest{TaskID: "t1", Source: "/data1", DestPath: "/backup1", Checksum: "abc"})
	mgr.RegisterBackup(BackupRegisterRequest{TaskID: "t2", Source: "/data2", DestPath: "/backup2", Checksum: "def"})

	// 运行验证
	backups := mgr.ListBackups()
	for _, b := range backups {
		mgr.RunVerification(VerifyRequest{BackupID: b.ID})
	}

	time.Sleep(500 * time.Millisecond)

	stats := mgr.GetStats()
	if stats.TotalBackups != 2 {
		t.Errorf("expected 2 total backups, got %d", stats.TotalBackups)
	}
	if stats.VerifiedBackups != 2 {
		t.Errorf("expected 2 verified backups, got %d", stats.VerifiedBackups)
	}
}
