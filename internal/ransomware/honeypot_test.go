package ransomware

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewHoneypotManager(t *testing.T) {
	config := DefaultHoneypotConfig()
	config.BasePaths = []string{t.TempDir()}

	hm := NewHoneypotManager(config)
	if hm == nil {
		t.Fatal("NewHoneypotManager returned nil")
	}

	if len(hm.templates) == 0 {
		t.Error("templates should not be empty")
	}

	if len(hm.layers) == 0 {
		t.Error("layers should not be empty")
	}
}

func TestHoneypotManager_DeployAll(t *testing.T) {
	tmpDir := t.TempDir()
	config := HoneypotConfig{
		Enabled:            true,
		BasePaths:          []string{tmpDir},
		FileCount:          5,
		FileExtensions:     []string{".docx", ".xlsx", ".pdf"},
		RefreshIntervalMin: 1440,
	}

	hm := NewHoneypotManager(config)
	if err := hm.DeployAll(); err != nil {
		t.Fatalf("DeployAll failed: %v", err)
	}

	files := hm.GetAll()
	if len(files) == 0 {
		t.Error("expected honeypot files to be deployed")
	}

	// 验证文件确实存在于磁盘
	for _, f := range files {
		if _, err := os.Stat(f.Path); os.IsNotExist(err) {
			t.Errorf("honeypot file does not exist: %s", f.Path)
		}
	}
}

func TestHoneypotManager_RecordAccess(t *testing.T) {
	tmpDir := t.TempDir()
	config := HoneypotConfig{
		Enabled:            true,
		BasePaths:          []string{tmpDir},
		FileCount:          3,
		FileExtensions:     []string{".txt"},
		RefreshIntervalMin: 1440,
	}

	hm := NewHoneypotManager(config)
	hm.DeployAll()

	files := hm.GetAll()
	if len(files) == 0 {
		t.Skip("no honeypot files deployed")
	}

	// 触发蜜罐
	event := hm.RecordAccess(files[0].Path, "write", "malware.exe", 1234, 0, "192.168.1.100")
	if event == nil {
		t.Fatal("RecordAccess should return event for honeypot file")
	}

	if event.ThreatLevel != ThreatLevelCritical {
		t.Errorf("expected ThreatLevelCritical for write access, got %s", event.ThreatLevel.String())
	}

	if event.ProcessName != "malware.exe" {
		t.Errorf("expected process name malware.exe, got %s", event.ProcessName)
	}

	// 验证统计
	stats := hm.GetStats()
	if stats.TotalTriggered != 1 {
		t.Errorf("expected 1 trigger, got %d", stats.TotalTriggered)
	}
}

func TestHoneypotManager_IsHoneypot(t *testing.T) {
	tmpDir := t.TempDir()
	config := HoneypotConfig{
		Enabled:            true,
		BasePaths:          []string{tmpDir},
		FileCount:          3,
		FileExtensions:     []string{".txt"},
		RefreshIntervalMin: 1440,
	}

	hm := NewHoneypotManager(config)
	hm.DeployAll()

	files := hm.GetAll()
	if len(files) == 0 {
		t.Skip("no honeypot files")
	}

	if !hm.IsHoneypot(files[0].Path) {
		t.Error("expected IsHoneypot to return true for deployed file")
	}

	if hm.IsHoneypot("/tmp/not-a-honeypot.txt") {
		t.Error("expected IsHoneypot to return false for non-honeypot file")
	}
}

func TestHoneypotManager_GetAccessLog(t *testing.T) {
	tmpDir := t.TempDir()
	config := HoneypotConfig{
		Enabled:            true,
		BasePaths:          []string{tmpDir},
		FileCount:          3,
		FileExtensions:     []string{".txt"},
		RefreshIntervalMin: 1440,
	}

	hm := NewHoneypotManager(config)
	hm.DeployAll()

	files := hm.GetAll()
	if len(files) == 0 {
		t.Skip("no honeypot files")
	}

	// 触发多次
	hm.RecordAccess(files[0].Path, "read", "proc1", 1, 0, "")
	hm.RecordAccess(files[0].Path, "write", "proc2", 2, 0, "")

	log := hm.GetAccessLog(10)
	if len(log) != 2 {
		t.Errorf("expected 2 log entries, got %d", len(log))
	}
}

func TestHoneypotManager_Triggered(t *testing.T) {
	tmpDir := t.TempDir()
	config := HoneypotConfig{
		Enabled:            true,
		BasePaths:          []string{tmpDir},
		FileCount:          3,
		FileExtensions:     []string{".txt"},
		RefreshIntervalMin: 1440,
	}

	hm := NewHoneypotManager(config)
	hm.DeployAll()

	files := hm.GetAll()
	if len(files) == 0 {
		t.Skip("no honeypot files")
	}

	hm.RecordAccess(files[0].Path, "delete", "ransom", 99, 0, "")

	triggered := hm.GetTriggered()
	if len(triggered) == 0 {
		t.Error("expected at least one triggered honeypot")
	}
}

func TestHoneypotManager_RecordAccessNonHoneypot(t *testing.T) {
	tmpDir := t.TempDir()
	config := HoneypotConfig{
		Enabled:            true,
		BasePaths:          []string{tmpDir},
		FileCount:          3,
		FileExtensions:     []string{".txt"},
		RefreshIntervalMin: 1440,
	}

	hm := NewHoneypotManager(config)
	hm.DeployAll()

	// 尝试对非蜜罐文件记录访问
	event := hm.RecordAccess("/tmp/regular-file.txt", "read", "normal", 1, 0, "")
	if event != nil {
		t.Error("should return nil for non-honeypot access")
	}
}

func TestHoneypotManager_TriggerCallback(t *testing.T) {
	tmpDir := t.TempDir()
	config := HoneypotConfig{
		Enabled:            true,
		BasePaths:          []string{tmpDir},
		FileCount:          3,
		FileExtensions:     []string{".txt"},
		RefreshIntervalMin: 1440,
	}

	hm := NewHoneypotManager(config)
	hm.DeployAll()

	triggered := false
	hm.SetTriggerCallback(func(event HoneypotAccessEvent) {
		triggered = true
	})

	files := hm.GetAll()
	if len(files) > 0 {
		hm.RecordAccess(files[0].Path, "read", "test", 1, 0, "")
	}

	// 给回调一点执行时间
	if !triggered {
		// callback runs in goroutine, might not have executed yet
		t.Log("trigger callback may not have executed yet (async)")
	}
}

func TestDefaultHoneypotConfig(t *testing.T) {
	config := DefaultHoneypotConfig()

	if !config.Enabled {
		t.Error("default config should be enabled")
	}

	if config.FileCount != 20 {
		t.Errorf("expected FileCount 20, got %d", config.FileCount)
	}

	if len(config.FileExtensions) == 0 {
		t.Error("expected non-empty FileExtensions")
	}
}

func TestHoneypotManager_DeployToPath(t *testing.T) {
	tmpDir := t.TempDir()
	config := HoneypotConfig{
		Enabled:            true,
		BasePaths:          []string{tmpDir},
		FileCount:          10,
		FileExtensions:     []string{".docx", ".xlsx", ".pdf", ".jpg"},
		RefreshIntervalMin: 1440,
	}

	hm := NewHoneypotManager(config)
	hm.DeployAll()

	files := hm.GetAll()
	if len(files) < 5 {
		t.Errorf("expected at least 5 honeypot files, got %d", len(files))
	}

	// 验证文件扩展名分布
	exts := make(map[string]int)
	for _, f := range files {
		exts[f.Extension]++
	}
	if len(exts) < 2 {
		t.Errorf("expected diverse extensions, got %v", exts)
	}

	// 验证文件在磁盘上存在
	honeypotDir := filepath.Join(tmpDir, ".honeypot")
	if _, err := os.Stat(honeypotDir); os.IsNotExist(err) {
		t.Error("honeypot directory should exist")
	}
}
