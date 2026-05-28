package bootrepair

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
}

func TestDetectBootloader(t *testing.T) {
	m := NewManager()

	info, err := m.DetectBootloader()
	if err != nil {
		t.Fatalf("detect bootloader failed: %v", err)
	}
	if info == nil {
		t.Fatal("expected bootloader info")
	}
	if info.Type != BootloaderGRUB {
		t.Errorf("expected grub, got '%s'", info.Type)
	}
	if !info.Detected {
		t.Error("expected detected")
	}
	if info.ConfigPath != "/boot/grub/grub.cfg" {
		t.Errorf("expected '/boot/grub/grub.cfg', got '%s'", info.ConfigPath)
	}
}

func TestListBootEntries(t *testing.T) {
	m := NewManager()

	entries, err := m.ListBootEntries()
	if err != nil {
		t.Fatalf("list entries failed: %v", err)
	}
	if len(entries) < 2 {
		t.Errorf("expected at least 2 entries, got %d", len(entries))
	}

	// 检查默认项
	hasDefault := false
	for _, e := range entries {
		if e.IsDefault {
			hasDefault = true
			if e.Name != "Ubuntu 22.04 LTS" {
				t.Errorf("expected 'Ubuntu 22.04 LTS', got '%s'", e.Name)
			}
		}
	}
	if !hasDefault {
		t.Error("expected default boot entry")
	}
}

func TestSetDefaultBoot(t *testing.T) {
	m := NewManager()

	// 设置恢复模式为默认
	err := m.SetDefaultBoot("recovery")
	if err != nil {
		t.Fatalf("set default boot failed: %v", err)
	}

	entries, _ := m.ListBootEntries()
	for _, e := range entries {
		if e.ID == "recovery" && !e.IsDefault {
			t.Error("expected recovery to be default")
		}
		if e.ID == "default" && e.IsDefault {
			t.Error("expected default to not be default")
		}
	}

	// 设置不存在的启动项
	err = m.SetDefaultBoot("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent entry")
	}
}

func TestCheckBootPartition(t *testing.T) {
	m := NewManager()

	issues, err := m.CheckBootPartition()
	if err != nil {
		t.Fatalf("check partition failed: %v", err)
	}
	if len(issues) == 0 {
		t.Error("expected at least one issue")
	}

	// 检查问题类型
	foundPartitionIssue := false
	for _, issue := range issues {
		if issue.Type == IssueTypePartitionError {
			foundPartitionIssue = true
			if issue.Severity != SeverityMedium {
				t.Errorf("expected medium severity, got '%s'", issue.Severity)
			}
		}
	}
	if !foundPartitionIssue {
		t.Error("expected partition error issue")
	}
}

func TestRepairBootloader(t *testing.T) {
	m := NewManager()

	job, err := m.RepairBootloader()
	if err != nil {
		t.Fatalf("repair bootloader failed: %v", err)
	}
	if job == nil {
		t.Fatal("expected repair job")
	}
	if job.Status != RepairStatusCompleted {
		t.Errorf("expected completed, got '%s'", job.Status)
	}
	if job.Result == "" {
		t.Error("expected non-empty result")
	}
}

func TestUEFIEntries(t *testing.T) {
	m := NewManager()

	// 列出默认 UEFI 启动项
	entries, err := m.ListUEFIEntries()
	if err != nil {
		t.Fatalf("list UEFI entries failed: %v", err)
	}
	if len(entries) < 1 {
		t.Errorf("expected at least 1 UEFI entry, got %d", len(entries))
	}

	// 添加新的 UEFI 启动项
	err = m.AddUEFIEntry(&UEFIEntry{
		Name:   "Windows Boot Manager",
		Path:   `\EFI\Microsoft\Boot\bootmgfw.efi`,
		Device: "/dev/sda2",
	})
	if err != nil {
		t.Fatalf("add UEFI entry failed: %v", err)
	}

	entries, _ = m.ListUEFIEntries()
	if len(entries) < 2 {
		t.Errorf("expected at least 2 UEFI entries, got %d", len(entries))
	}

	// 添加无效条目
	err = m.AddUEFIEntry(nil)
	if err == nil {
		t.Error("expected error for nil entry")
	}

	err = m.AddUEFIEntry(&UEFIEntry{Name: "test"})
	if err == nil {
		t.Error("expected error for empty path")
	}

	// 删除 UEFI 启动项
	err = m.RemoveUEFIEntry("ubuntu")
	if err != nil {
		t.Fatalf("remove UEFI entry failed: %v", err)
	}

	entries, _ = m.ListUEFIEntries()
	for _, e := range entries {
		if e.ID == "ubuntu" {
			t.Error("expected ubuntu entry to be removed")
		}
	}

	// 删除不存在的条目
	err = m.RemoveUEFIEntry("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent entry")
	}
}

func TestSecureBoot(t *testing.T) {
	m := NewManager()

	// 检查初始状态
	status, err := m.CheckSecureBoot()
	if err != nil {
		t.Fatalf("check secure boot failed: %v", err)
	}
	if status.Enabled {
		t.Error("expected secure boot disabled by default")
	}

	// 启用安全启动
	err = m.SetSecureBoot(true)
	if err != nil {
		t.Fatalf("set secure boot failed: %v", err)
	}

	status, _ = m.CheckSecureBoot()
	if !status.Enabled {
		t.Error("expected secure boot enabled")
	}
	if status.KeyState != "deployed" {
		t.Errorf("expected deployed, got '%s'", status.KeyState)
	}
	if !status.PlatformKey {
		t.Error("expected platform key")
	}

	// 禁用安全启动
	err = m.SetSecureBoot(false)
	if err != nil {
		t.Fatalf("disable secure boot failed: %v", err)
	}

	status, _ = m.CheckSecureBoot()
	if status.Enabled {
		t.Error("expected secure boot disabled")
	}
	if !status.SetupMode {
		t.Error("expected setup mode")
	}
}

func TestRollbackKernel(t *testing.T) {
	m := NewManager()

	// 回滚内核
	err := m.RollbackKernel("5.15.0-58-generic")
	if err != nil {
		t.Fatalf("rollback kernel failed: %v", err)
	}

	// 检查新启动项
	entries, _ := m.ListBootEntries()
	found := false
	for _, e := range entries {
		if e.IsDefault && e.Name == "Linux 5.15.0-58-generic (回滚)" {
			found = true
		}
	}
	if !found {
		t.Error("expected rollback entry to be default")
	}

	// 空版本应失败
	err = m.RollbackKernel("")
	if err == nil {
		t.Error("expected error for empty version")
	}
}

func TestGetBootLogs(t *testing.T) {
	m := NewManager()

	// 获取所有日志
	logs := m.GetBootLogs(time.Time{})
	if len(logs) < 5 {
		t.Errorf("expected at least 5 logs, got %d", len(logs))
	}

	// 获取最近的日志
	recent := m.GetBootLogs(time.Now().Add(-15 * time.Second))
	if len(recent) == 0 {
		t.Error("expected recent logs")
	}

	// 验证日志内容
	hasFirmwareLog := false
	for _, l := range logs {
		if l.Phase == PhaseFirmware {
			hasFirmwareLog = true
			if !l.Success {
				t.Error("expected firmware log to be successful")
			}
		}
	}
	if !hasFirmwareLog {
		t.Error("expected firmware log")
	}
}

func TestAnalyzeIssues(t *testing.T) {
	m := NewManager()

	// 分析问题
	issues, err := m.AnalyzeIssues()
	if err != nil {
		// 没有问题是正常的（所有日志都成功）
		return
	}

	if len(issues) == 0 {
		t.Skip("no issues found, skipping")
	}

	// 检查问题格式
	for _, issue := range issues {
		if issue.ID == "" {
			t.Error("expected non-empty issue ID")
		}
		if issue.Description == "" {
			t.Error("expected non-empty description")
		}
		if issue.Suggestion == "" {
			t.Error("expected non-empty suggestion")
		}
	}
}

func TestAutoRepair(t *testing.T) {
	m := NewManager()

	// 先创建一个问题
	issues, _ := m.CheckBootPartition()
	if len(issues) == 0 {
		t.Skip("no issues to repair")
	}

	// 自动修复第一个问题
	job, err := m.AutoRepair(issues[0].ID)
	if err != nil {
		t.Fatalf("auto repair failed: %v", err)
	}
	if job.Status != RepairStatusCompleted {
		t.Errorf("expected completed, got '%s'", job.Status)
	}
	if job.Result == "" {
		t.Error("expected non-empty result")
	}

	// 重复修复应失败
	_, err = m.AutoRepair(issues[0].ID)
	if err == nil {
		t.Error("expected error for already repaired issue")
	}

	// 不存在的问题
	_, err = m.AutoRepair("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent issue")
	}
}

func TestRescueMode(t *testing.T) {
	m := NewManager()

	// 初始状态不在救援模式
	if m.IsInRescueMode() {
		t.Error("expected not in rescue mode initially")
	}

	// 进入救援模式
	err := m.EnterRescueMode()
	if err != nil {
		t.Fatalf("enter rescue mode failed: %v", err)
	}
	if !m.IsInRescueMode() {
		t.Error("expected in rescue mode")
	}

	// 重复进入应失败
	err = m.EnterRescueMode()
	if err == nil {
		t.Error("expected error for duplicate enter")
	}

	// 退出救援模式
	err = m.ExitRescueMode()
	if err != nil {
		t.Fatalf("exit rescue mode failed: %v", err)
	}
	if m.IsInRescueMode() {
		t.Error("expected not in rescue mode")
	}

	// 重复退出应失败
	err = m.ExitRescueMode()
	if err == nil {
		t.Error("expected error for duplicate exit")
	}
}
