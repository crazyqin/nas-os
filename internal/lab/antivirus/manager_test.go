// Package antivirus 测试
package antivirus

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(DefaultClamAVConfig(), tmpDir)
	if mgr == nil {
		t.Fatal("管理器不应为nil")
	}
}

func TestCreateScan(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(DefaultClamAVConfig(), tmpDir)

	task, err := mgr.CreateScan(CreateScanRequest{
		Name:  "测试扫描",
		Type:  ScanTypeQuick,
		Paths: []string{"/tmp"},
	})
	if err != nil {
		t.Fatalf("创建扫描失败: %v", err)
	}
	if task.Name != "测试扫描" {
		t.Errorf("名称不匹配: %s", task.Name)
	}
	if task.Type != ScanTypeQuick {
		t.Errorf("类型不匹配: %s", task.Type)
	}
}

func TestGetScan(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(DefaultClamAVConfig(), tmpDir)

	task, _ := mgr.CreateScan(CreateScanRequest{
		Name:  "test",
		Type:  ScanTypeFull,
		Paths: []string{"/tmp"},
	})

	got, err := mgr.GetScan(task.ID)
	if err != nil {
		t.Fatalf("获取扫描失败: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("ID不匹配")
	}
}

func TestGetScanNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(DefaultClamAVConfig(), tmpDir)

	_, err := mgr.GetScan("nonexistent")
	if err == nil {
		t.Error("不存在的任务应返回错误")
	}
}

func TestWhitelistOperations(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(DefaultClamAVConfig(), tmpDir)

	entry := mgr.AddWhitelist(WhitelistAddRequest{
		Path:   "/safe/file.txt",
		Reason: "已确认安全",
	})
	if entry.Path != "/safe/file.txt" {
		t.Errorf("路径不匹配")
	}

	list := mgr.ListWhitelist()
	if len(list) != 1 {
		t.Errorf("白名单应有1条，实际 %d", len(list))
	}

	err := mgr.RemoveWhitelist(entry.ID)
	if err != nil {
		t.Fatalf("移除白名单失败: %v", err)
	}

	list = mgr.ListWhitelist()
	if len(list) != 0 {
		t.Errorf("白名单应为空")
	}
}

func TestQuarantineEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(DefaultClamAVConfig(), tmpDir)

	list := mgr.GetQuarantineList()
	if len(list) != 0 {
		t.Errorf("初始隔离区应为空")
	}
}

func TestMonitorConfig(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(DefaultClamAVConfig(), tmpDir)

	cfg := mgr.GetMonitorConfig()
	if cfg.Enabled {
		t.Error("初始应禁用")
	}

	enabled := true
	mgr.UpdateMonitorConfig(UpdateMonitorConfigRequest{
		Enabled:    &enabled,
		WatchPaths: []string{"/mnt/data"},
	})

	cfg = mgr.GetMonitorConfig()
	if !cfg.Enabled {
		t.Error("应已启用")
	}
	if len(cfg.WatchPaths) != 1 {
		t.Errorf("监控路径数量不匹配")
	}
}

func TestStats(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(DefaultClamAVConfig(), tmpDir)

	stats := mgr.GetStats()
	if stats.TotalScans != 0 {
		t.Errorf("初始扫描数应为0")
	}
}

func TestVirusDBStatus(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(DefaultClamAVConfig(), tmpDir)

	status := mgr.GetVirusDBStatus()
	if status == nil {
		t.Fatal("病毒库状态不应为nil")
	}
}

func TestScanTypes(t *testing.T) {
	types := []ScanType{ScanTypeFull, ScanTypeQuick, ScanTypeCustom}
	for _, st := range types {
		if st == "" {
			t.Error("扫描类型不应为空")
		}
	}
}
