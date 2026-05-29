// Package ransomwarecanary 测试
package ransomwarecanary

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("manager should not be nil")
	}

	cfg := m.GetConfig()
	if !cfg.Enabled {
		t.Error("config should be enabled by default")
	}
	if cfg.AutoLockEnabled {
		t.Error("auto lock should be disabled by default")
	}
}

func TestDeployCanary(t *testing.T) {
	m := NewManager()

	// 创建临时目录用于测试
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test_canary.txt")

	canary, err := m.DeployCanary(DeployCanaryRequest{
		Name:      "test_canary",
		ShareName: "test_share",
		FilePath:  filePath,
	})
	if err != nil {
		t.Fatalf("deploy canary failed: %v", err)
	}

	if canary == nil {
		t.Fatal("canary should not be nil")
	}
	if canary.ID == "" {
		t.Error("canary should have an ID")
	}
	if canary.Name != "test_canary" {
		t.Errorf("expected name test_canary, got %s", canary.Name)
	}
	if canary.Status != "active" {
		t.Errorf("expected status active, got %s", canary.Status)
	}
	if canary.ContentHash == "" {
		t.Error("canary should have a content hash")
	}

	// 验证文件已创建
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("canary file should exist on disk")
	}
}

func TestDeployCanaryDuplicatePath(t *testing.T) {
	m := NewManager()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "dup_canary.txt")

	_, err := m.DeployCanary(DeployCanaryRequest{
		Name:      "canary1",
		ShareName: "share1",
		FilePath:  filePath,
	})
	if err != nil {
		t.Fatalf("first deploy failed: %v", err)
	}

	// 同路径再次部署应失败
	_, err = m.DeployCanary(DeployCanaryRequest{
		Name:      "canary2",
		ShareName: "share1",
		FilePath:  filePath,
	})
	if err == nil {
		t.Error("expected error for duplicate path")
	}
}

func TestRemoveCanary(t *testing.T) {
	m := NewManager()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "remove_canary.txt")

	canary, err := m.DeployCanary(DeployCanaryRequest{
		Name:      "to_remove",
		ShareName: "share1",
		FilePath:  filePath,
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	if err := m.RemoveCanary(canary.ID); err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	// 验证文件已删除
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("canary file should have been deleted")
	}

	// 验证列表中已无该金丝雀
	canaries := m.ListCanaries()
	for _, c := range canaries {
		if c.ID == canary.ID {
			t.Error("canary should not be in list after removal")
		}
	}
}

func TestRemoveCanaryNotFound(t *testing.T) {
	m := NewManager()

	err := m.RemoveCanary("nonexistent-id")
	if err == nil {
		t.Error("expected error for nonexistent canary")
	}
}

func TestDisableCanary(t *testing.T) {
	m := NewManager()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "disable_canary.txt")

	canary, err := m.DeployCanary(DeployCanaryRequest{
		Name:      "to_disable",
		ShareName: "share1",
		FilePath:  filePath,
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	if err := m.DisableCanary(canary.ID); err != nil {
		t.Fatalf("disable failed: %v", err)
	}

	// 验证状态已改变
	canaries := m.ListCanaries()
	for _, c := range canaries {
		if c.ID == canary.ID {
			if c.Status != "disabled" {
				t.Errorf("expected status disabled, got %s", c.Status)
			}
		}
	}
}

func TestListCanaries(t *testing.T) {
	m := NewManager()

	if len(m.ListCanaries()) != 0 {
		t.Error("should start with no canaries")
	}

	tmpDir := t.TempDir()
	for i := 0; i < 3; i++ {
		_, err := m.DeployCanary(DeployCanaryRequest{
			Name:      "canary",
			ShareName: "share1",
			FilePath:  filepath.Join(tmpDir, "canary_"+string(rune('a'+i))+".txt"),
		})
		if err != nil {
			t.Fatalf("deploy failed: %v", err)
		}
	}

	canaries := m.ListCanaries()
	if len(canaries) != 3 {
		t.Errorf("expected 3 canaries, got %d", len(canaries))
	}
}

func TestMonitorCanaries(t *testing.T) {
	m := NewManager()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "monitor_canary.txt")

	_, err := m.DeployCanary(DeployCanaryRequest{
		Name:      "monitor_test",
		ShareName: "share1",
		FilePath:  filePath,
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	// 正常检测应无告警
	result, err := m.MonitorCanaries()
	if err != nil {
		t.Fatalf("monitor failed: %v", err)
	}

	if result.TotalChecked != 1 {
		t.Errorf("expected 1 checked, got %d", result.TotalChecked)
	}
	if result.AlertCount != 0 {
		t.Errorf("expected 0 alerts, got %d", result.AlertCount)
	}
	if result.Duration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestMonitorCanariesDetectDeletion(t *testing.T) {
	m := NewManager()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "deletion_canary.txt")

	_, err := m.DeployCanary(DeployCanaryRequest{
		Name:      "deletion_test",
		ShareName: "share1",
		FilePath:  filePath,
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	// 删除文件模拟勒索软件
	os.Remove(filePath)

	result, err := m.MonitorCanaries()
	if err != nil {
		t.Fatalf("monitor failed: %v", err)
	}

	if result.AlertCount != 1 {
		t.Errorf("expected 1 alert for deletion, got %d", result.AlertCount)
	}

	if len(result.Alerts) > 0 {
		alert := result.Alerts[0]
		if alert.AlertType != "deleted" {
			t.Errorf("expected alert type deleted, got %s", alert.AlertType)
		}
		if alert.Severity != "critical" {
			t.Errorf("expected severity critical, got %s", alert.Severity)
		}
	}
}

func TestMonitorCanariesDetectModification(t *testing.T) {
	m := NewManager()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "modify_canary.txt")

	_, err := m.DeployCanary(DeployCanaryRequest{
		Name:      "modify_test",
		ShareName: "share1",
		FilePath:  filePath,
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	// 修改文件内容模拟加密攻击（保持相同大小，仅改变内容）
	original, _ := os.ReadFile(filePath)
	modified := make([]byte, len(original))
	for i := range modified {
		modified[i] = original[len(original)-1-i] // 反转内容
	}
	os.WriteFile(filePath, modified, 0644)

	result, err := m.MonitorCanaries()
	if err != nil {
		t.Fatalf("monitor failed: %v", err)
	}

	if result.AlertCount != 1 {
		t.Errorf("expected 1 alert for modification, got %d", result.AlertCount)
	}

	if len(result.Alerts) > 0 {
		alert := result.Alerts[0]
		if alert.AlertType != "encrypted" {
			t.Errorf("expected alert type encrypted, got %s", alert.AlertType)
		}
	}
}

func TestMonitorCanariesSizeChange(t *testing.T) {
	m := NewManager()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "size_canary.txt")

	_, err := m.DeployCanary(DeployCanaryRequest{
		Name:      "size_test",
		ShareName: "share1",
		FilePath:  filePath,
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	// 写入不同大小的内容（保持前1024字节相同但追加数据）
	original, _ := os.ReadFile(filePath)
	newContent := append(original, []byte("EXTRA_DATA")...)
	os.WriteFile(filePath, newContent, 0644)

	result, err := m.MonitorCanaries()
	if err != nil {
		t.Fatalf("monitor failed: %v", err)
	}

	if result.AlertCount != 1 {
		t.Errorf("expected 1 alert for size change, got %d", result.AlertCount)
	}

	if len(result.Alerts) > 0 {
		alert := result.Alerts[0]
		if alert.AlertType != "modified" {
			t.Errorf("expected alert type modified, got %s", alert.AlertType)
		}
	}
}

func TestTriggerAlert(t *testing.T) {
	m := NewManager()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "trigger_canary.txt")

	canary, err := m.DeployCanary(DeployCanaryRequest{
		Name:      "trigger_test",
		ShareName: "share1",
		FilePath:  filePath,
	})
	if err != nil {
		t.Fatalf("deploy failed: %v", err)
	}

	alert, err := m.TriggerAlert(canary.ID, "modified", "high", "manual trigger test")
	if err != nil {
		t.Fatalf("trigger alert failed: %v", err)
	}

	if alert.CanaryID != canary.ID {
		t.Errorf("expected canary ID %s, got %s", canary.ID, alert.CanaryID)
	}
	if alert.AlertType != "modified" {
		t.Errorf("expected alert type modified, got %s", alert.AlertType)
	}
	if alert.Severity != "high" {
		t.Errorf("expected severity high, got %s", alert.Severity)
	}
}

func TestTriggerAlertNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.TriggerAlert("nonexistent", "modified", "high", "test")
	if err == nil {
		t.Error("expected error for nonexistent canary")
	}
}

func TestGetAlerts(t *testing.T) {
	m := NewManager()

	// 无告警时
	alerts := m.GetAlerts(10)
	if len(alerts) != 0 {
		t.Error("should have no alerts initially")
	}

	// 触发一些告警
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "alerts_canary.txt")

	canary, _ := m.DeployCanary(DeployCanaryRequest{
		Name:      "alerts_test",
		ShareName: "share1",
		FilePath:  filePath,
	})

	m.TriggerAlert(canary.ID, "modified", "high", "alert 1")
	m.TriggerAlert(canary.ID, "deleted", "critical", "alert 2")
	m.TriggerAlert(canary.ID, "encrypted", "critical", "alert 3")

	alerts = m.GetAlerts(2)
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts, got %d", len(alerts))
	}

	all := m.GetAlerts(0)
	if len(all) != 3 {
		t.Errorf("expected 3 alerts, got %d", len(all))
	}
}

func TestClearAlerts(t *testing.T) {
	m := NewManager()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "clear_canary.txt")

	canary, _ := m.DeployCanary(DeployCanaryRequest{
		Name:      "clear_test",
		ShareName: "share1",
		FilePath:  filePath,
	})

	m.TriggerAlert(canary.ID, "modified", "high", "test")
	if len(m.GetAlerts(0)) != 1 {
		t.Error("should have 1 alert")
	}

	m.ClearAlerts()
	if len(m.GetAlerts(0)) != 0 {
		t.Error("should have 0 alerts after clear")
	}
}

func TestLockUnlockShare(t *testing.T) {
	m := NewManager()

	locked, err := m.AutoLockShare("test_share", "test reason")
	if err != nil {
		t.Fatalf("lock failed: %v", err)
	}
	if !locked {
		t.Error("share should be locked")
	}

	// 再次锁定应返回 false（已锁定）
	locked, err = m.AutoLockShare("test_share", "duplicate lock")
	if err != nil {
		t.Fatalf("duplicate lock failed: %v", err)
	}
	if locked {
		t.Error("should return false for already locked share")
	}

	// 验证锁定列表
	lockedShares := m.GetLockedShares()
	if _, ok := lockedShares["test_share"]; !ok {
		t.Error("test_share should be in locked shares")
	}

	// 解锁
	if err := m.UnlockShare("test_share"); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}

	lockedShares = m.GetLockedShares()
	if _, ok := lockedShares["test_share"]; ok {
		t.Error("test_share should not be in locked shares after unlock")
	}
}

func TestUnlockShareNotLocked(t *testing.T) {
	m := NewManager()

	err := m.UnlockShare("nonexistent_share")
	if err == nil {
		t.Error("expected error for unlocking non-locked share")
	}
}

func TestGetStatus(t *testing.T) {
	m := NewManager()

	tmpDir := t.TempDir()
	for i := 0; i < 3; i++ {
		filePath := filepath.Join(tmpDir, "status_canary_"+string(rune('a'+i))+".txt")
		m.DeployCanary(DeployCanaryRequest{
			Name:      "status_test",
			ShareName: "share1",
			FilePath:  filePath,
		})
	}

	status := m.GetStatus()
	if status.TotalCanaries != 3 {
		t.Errorf("expected 3 canaries, got %d", status.TotalCanaries)
	}
	if status.ActiveCount != 3 {
		t.Errorf("expected 3 active, got %d", status.ActiveCount)
	}
	if status.TriggeredCount != 0 {
		t.Errorf("expected 0 triggered, got %d", status.TriggeredCount)
	}
}

func TestUpdateConfig(t *testing.T) {
	m := NewManager()

	cfg := m.GetConfig()
	if cfg.CheckIntervalSec != 60 {
		t.Errorf("default interval should be 60, got %d", cfg.CheckIntervalSec)
	}

	newCfg := CanaryConfig{
		Enabled:          true,
		CheckIntervalSec: 30,
		MonitoredPaths:   []string{"/shares/photos", "/shares/documents"},
		AutoLockEnabled:  true,
		AlertWebhookURL:  "https://example.com/alert",
		MaxAlertsPerHour: 100,
	}
	m.UpdateConfig(newCfg)

	updated := m.GetConfig()
	if updated.CheckIntervalSec != 30 {
		t.Errorf("expected interval 30, got %d", updated.CheckIntervalSec)
	}
	if !updated.AutoLockEnabled {
		t.Error("auto lock should be enabled")
	}
	if updated.AlertWebhookURL != "https://example.com/alert" {
		t.Errorf("expected webhook URL, got %s", updated.AlertWebhookURL)
	}
}

func TestMonitorNoCanaries(t *testing.T) {
	m := NewManager()

	result, err := m.MonitorCanaries()
	if err != nil {
		t.Fatalf("monitor failed: %v", err)
	}
	if result.TotalChecked != 0 {
		t.Errorf("expected 0 checked, got %d", result.TotalChecked)
	}
	if result.AlertCount != 0 {
		t.Errorf("expected 0 alerts, got %d", result.AlertCount)
	}
}

func TestMonitorSkipsDisabled(t *testing.T) {
	m := NewManager()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "disabled_canary.txt")

	canary, _ := m.DeployCanary(DeployCanaryRequest{
		Name:      "disabled_test",
		ShareName: "share1",
		FilePath:  filePath,
	})

	m.DisableCanary(canary.ID)

	// 删除文件 — 但金丝雀已禁用，不应产生告警
	os.Remove(filePath)

	result, err := m.MonitorCanaries()
	if err != nil {
		t.Fatalf("monitor failed: %v", err)
	}
	if result.TotalChecked != 0 {
		t.Errorf("expected 0 checked (disabled), got %d", result.TotalChecked)
	}
	if result.AlertCount != 0 {
		t.Errorf("expected 0 alerts (disabled), got %d", result.AlertCount)
	}
}

func TestAutoLockWithMonitor(t *testing.T) {
	m := NewManager()

	// 启用自动锁定
	cfg := m.GetConfig()
	cfg.AutoLockEnabled = true
	m.UpdateConfig(cfg)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "autolock_canary.txt")

	m.DeployCanary(DeployCanaryRequest{
		Name:      "autolock_test",
		ShareName: "critical_share",
		FilePath:  filePath,
	})

	// 删除文件触发告警
	os.Remove(filePath)

	result, _ := m.MonitorCanaries()
	if result.AlertCount != 1 {
		t.Fatalf("expected 1 alert, got %d", result.AlertCount)
	}

	// 验证共享被自动锁定
	locked := m.GetLockedShares()
	if _, ok := locked["critical_share"]; !ok {
		t.Error("critical_share should be auto-locked after critical alert")
	}

	if !result.Alerts[0].ShareLocked {
		t.Error("alert should indicate share was locked")
	}
}

func TestComputeSHA256(t *testing.T) {
	data := []byte("hello world")
	hash := computeSHA256(data)
	if hash == "" {
		t.Error("hash should not be empty")
	}
	if len(hash) != 64 {
		t.Errorf("SHA256 hash should be 64 hex chars, got %d", len(hash))
	}

	// 相同数据应产生相同哈希
	hash2 := computeSHA256(data)
	if hash != hash2 {
		t.Error("same data should produce same hash")
	}

	// 不同数据应产生不同哈希
	hash3 := computeSHA256([]byte("hello world!"))
	if hash == hash3 {
		t.Error("different data should produce different hash")
	}
}

func TestGenerateCanaryContent(t *testing.T) {
	content := generateCanaryContent("test.xlsx")
	if len(content) == 0 {
		t.Error("content should not be empty")
	}
	if len(content) < 1024 {
		t.Errorf("content should be at least 1024 bytes, got %d", len(content))
	}
}
