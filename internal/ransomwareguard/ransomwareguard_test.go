package ransomwareguard

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()

	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if !m.enabled {
		t.Error("expected enabled=true by default")
	}
	if len(m.monitoredPaths) != 2 {
		t.Errorf("expected 2 monitored paths, got %d", len(m.monitoredPaths))
	}
	if m.monitoredPaths[0] != "/data" || m.monitoredPaths[1] != "/shared" {
		t.Errorf("unexpected default paths: %v", m.monitoredPaths)
	}
	if m.windowSec != 60 {
		t.Errorf("expected windowSec=60, got %d", m.windowSec)
	}
	if m.threshold != 50 {
		t.Errorf("expected threshold=50, got %d", m.threshold)
	}
	if m.honeypots == nil || m.alerts == nil || m.recentEvents == nil {
		t.Error("maps/slices should be initialized")
	}
}

func TestEnableDisable(t *testing.T) {
	m := NewManager()

	// 初始状态已启用
	status := m.GetStatus()
	if !status.Enabled {
		t.Error("expected enabled on creation")
	}

	// 禁用
	m.Disable()
	status = m.GetStatus()
	if status.Enabled {
		t.Error("expected disabled after Disable()")
	}

	// 重新启用
	m.Enable()
	status = m.GetStatus()
	if !status.Enabled {
		t.Error("expected enabled after Enable()")
	}
	if status.ProtectionSince == nil {
		t.Error("expected ProtectionSince to be set after Enable()")
	}
}

func TestAddMonitoredPath(t *testing.T) {
	m := NewManager()
	initial := len(m.monitoredPaths)

	// 添加新路径
	m.AddMonitoredPath("/backup")
	if len(m.monitoredPaths) != initial+1 {
		t.Errorf("expected %d paths, got %d", initial+1, len(m.monitoredPaths))
	}

	// 重复添加不生效
	m.AddMonitoredPath("/backup")
	if len(m.monitoredPaths) != initial+1 {
		t.Errorf("duplicate add should be ignored, got %d paths", len(m.monitoredPaths))
	}
}

func TestRemoveMonitoredPath(t *testing.T) {
	m := NewManager()

	// 移除存在的路径
	m.RemoveMonitoredPath("/data")
	for _, p := range m.monitoredPaths {
		if p == "/data" {
			t.Error("/data should have been removed")
		}
	}

	// 移除不存在的路径不报错
	m.RemoveMonitoredPath("/nonexistent")
	if len(m.monitoredPaths) != 1 {
		t.Errorf("expected 1 path, got %d", len(m.monitoredPaths))
	}
}

func TestDeployHoneypots(t *testing.T) {
	m := NewManager()

	// 部署蜜罐
	deployed := m.DeployHoneypots("/test", 5)
	if deployed != 5 {
		t.Errorf("expected 5 deployed, got %d", deployed)
	}

	honeypots := m.GetHoneypots()
	if len(honeypots) != 5 {
		t.Errorf("expected 5 honeypots, got %d", len(honeypots))
	}

	// 重复部署不覆盖
	deployed = m.DeployHoneypots("/test", 5)
	if deployed != 0 {
		t.Errorf("expected 0 new deployed, got %d", deployed)
	}

	// 部署更多
	deployed = m.DeployHoneypots("/test2", 3)
	if deployed != 3 {
		t.Errorf("expected 3 deployed, got %d", deployed)
	}

	// 限制最大数量
	deployed = m.DeployHoneypots("/test3", 20)
	if deployed > 10 {
		t.Errorf("should not deploy more than available names, got %d", deployed)
	}
}

func TestProcessEvent_HoneypotTrigger(t *testing.T) {
	m := NewManager()
	m.DeployHoneypots("/test", 1)

	honeypots := m.GetHoneypots()
	if len(honeypots) == 0 {
		t.Fatal("no honeypots deployed")
	}

	// 触发蜜罐
	event := FileEvent{
		Path:      honeypots[0].Path,
		Operation: "modify",
		Timestamp: time.Now(),
	}

	alert := m.ProcessEvent(event)
	if alert == nil {
		t.Fatal("expected alert from honeypot trigger")
	}
	if alert.Type != AlertHoneypotHit {
		t.Errorf("expected AlertHoneypotHit, got %s", alert.Type)
	}
	if alert.Level != ThreatCritical {
		t.Errorf("expected ThreatCritical, got %s", alert.Level)
	}

	// 重新获取蜜罐检查触发状态
	honeypots = m.GetHoneypots()
	if !honeypots[0].Triggered {
		t.Error("honeypot should be marked as triggered")
	}

	// 再次触发同一个蜜罐不产生新告警
	alert2 := m.ProcessEvent(event)
	if alert2 != nil {
		t.Error("second trigger of same honeypot should not create new alert")
	}
}

func TestProcessEvent_SuspiciousExt(t *testing.T) {
	m := NewManager()

	tests := []struct {
		path      string
		operation string
		wantAlert bool
	}{
		{"/test/file.encrypted", "rename", true},
		{"/test/file.locked", "create", true},
		{"/test/file.wncry", "rename", true},
		{"/test/file.cerber", "create", true},
		{"/test/file.txt", "rename", false},
		{"/test/file.encrypted", "modify", false}, // modify 不检测
		{"/test/file.normal", "create", false},
	}

	for _, tt := range tests {
		alert := m.ProcessEvent(FileEvent{
			Path:      tt.path,
			Operation: tt.operation,
			Timestamp: time.Now(),
		})
		if tt.wantAlert && alert == nil {
			t.Errorf("expected alert for %s/%s", tt.path, tt.operation)
		}
		if !tt.wantAlert && alert != nil && alert.Type == AlertSuspiciousExt {
			t.Errorf("unexpected alert for %s/%s", tt.path, tt.operation)
		}
	}
}

func TestProcessEvent_MassDelete(t *testing.T) {
	m := NewManager()

	// 生成大量删除事件
	now := time.Now()
	for i := 0; i < 55; i++ {
		m.ProcessEvent(FileEvent{
			Path:      fmt.Sprintf("/test/file_%d.txt", i),
			Operation: "delete",
			Timestamp: now,
		})
	}

	// 检查是否产生批量删除告警
	alerts := m.GetAlerts(false)
	found := false
	for _, a := range alerts {
		if a.Type == AlertMassDelete {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mass delete alert after 55 deletes")
	}
}

func TestProcessEvent_MassRename(t *testing.T) {
	m := NewManager()

	// 生成大量重命名事件
	now := time.Now()
	for i := 0; i < 55; i++ {
		m.ProcessEvent(FileEvent{
			Path:      fmt.Sprintf("/test/renamed_%d.txt", i),
			Operation: "rename",
			Timestamp: now,
		})
	}

	// 检查是否产生批量重命名告警
	alerts := m.GetAlerts(false)
	found := false
	for _, a := range alerts {
		if a.Type == AlertMassRename {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mass rename alert after 55 renames")
	}
}

func TestProcessEvent_Disabled(t *testing.T) {
	m := NewManager()
	m.Disable()

	alert := m.ProcessEvent(FileEvent{
		Path:      "/test/file.txt",
		Operation: "delete",
		Timestamp: time.Now(),
	})

	if alert != nil {
		t.Error("should not generate alert when disabled")
	}
}

func TestGetStatus(t *testing.T) {
	m := NewManager()

	status := m.GetStatus()
	if !status.Enabled {
		t.Error("expected enabled")
	}
	if status.HoneypotCount != 0 {
		t.Error("expected 0 honeypots")
	}
	if status.ActiveAlerts != 0 {
		t.Error("expected 0 active alerts")
	}

	// 添加蜜罐和告警
	m.DeployHoneypots("/test", 3)
	m.ProcessEvent(FileEvent{
		Path:      "/test/file.encrypted",
		Operation: "rename",
		Timestamp: time.Now(),
	})

	status = m.GetStatus()
	if status.HoneypotCount != 3 {
		t.Errorf("expected 3 honeypots, got %d", status.HoneypotCount)
	}
	if status.ActiveAlerts != 1 {
		t.Errorf("expected 1 active alert, got %d", status.ActiveAlerts)
	}
	if status.TotalBlocked != 1 {
		t.Errorf("expected 1 total blocked, got %d", status.TotalBlocked)
	}
}

func TestGetAlerts(t *testing.T) {
	m := NewManager()

	// 初始无告警
	alerts := m.GetAlerts(false)
	if len(alerts) != 0 {
		t.Error("expected 0 alerts initially")
	}

	// 产生告警
	m.ProcessEvent(FileEvent{
		Path:      "/test/file.encrypted",
		Operation: "rename",
		Timestamp: time.Now(),
	})

	// 获取未解决告警
	alerts = m.GetAlerts(false)
	if len(alerts) != 1 {
		t.Errorf("expected 1 unresolved alert, got %d", len(alerts))
	}

	// 获取已解决告警
	alerts = m.GetAlerts(true)
	if len(alerts) != 0 {
		t.Error("expected 0 resolved alerts")
	}
}

func TestResolveAlert(t *testing.T) {
	m := NewManager()

	// 产生告警
	m.ProcessEvent(FileEvent{
		Path:      "/test/file.encrypted",
		Operation: "rename",
		Timestamp: time.Now(),
	})

	alerts := m.GetAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("no alerts to resolve")
	}

	// 解决告警
	err := m.ResolveAlert(alerts[0].ID)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// 验证已解决
	unresolved := m.GetAlerts(false)
	resolved := m.GetAlerts(true)
	if len(unresolved) != 0 {
		t.Errorf("expected 0 unresolved, got %d", len(unresolved))
	}
	if len(resolved) != 1 {
		t.Errorf("expected 1 resolved, got %d", len(resolved))
	}

	// 解决不存在的告警
	err = m.ResolveAlert("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestGetHoneypots(t *testing.T) {
	m := NewManager()

	// 初始无蜜罐
	honeypots := m.GetHoneypots()
	if len(honeypots) != 0 {
		t.Error("expected 0 honeypots initially")
	}

	// 部署蜜罐
	m.DeployHoneypots("/test", 3)
	honeypots = m.GetHoneypots()
	if len(honeypots) != 3 {
		t.Errorf("expected 3 honeypots, got %d", len(honeypots))
	}

	// 验证蜜罐属性
	for _, hp := range honeypots {
		if hp.Path == "" {
			t.Error("honeypot path should not be empty")
		}
		if hp.RealPath == "" {
			t.Error("honeypot real path should not be empty")
		}
		if hp.Triggered {
			t.Error("new honeypot should not be triggered")
		}
	}
}

func TestResetHoneypots(t *testing.T) {
	m := NewManager()
	m.DeployHoneypots("/test", 1)

	// 触发蜜罐
	honeypots := m.GetHoneypots()
	m.ProcessEvent(FileEvent{
		Path:      honeypots[0].Path,
		Operation: "modify",
		Timestamp: time.Now(),
	})

	// 验证已触发
	honeypots = m.GetHoneypots()
	if !honeypots[0].Triggered {
		t.Error("honeypot should be triggered")
	}

	// 重置
	m.ResetHoneypots()
	honeypots = m.GetHoneypots()
	if honeypots[0].Triggered {
		t.Error("honeypot should be reset")
	}
}

func TestConcurrency(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup

	// 并发读写
	for i := 0; i < 100; i++ {
		wg.Add(3)

		go func() {
			defer wg.Done()
			m.GetStatus()
		}()

		go func(n int) {
			defer wg.Done()
			m.ProcessEvent(FileEvent{
				Path:      fmt.Sprintf("/test/file_%d.txt", n),
				Operation: "create",
				Timestamp: time.Now(),
			})
		}(i)

		go func(n int) {
			defer wg.Done()
			if n%2 == 0 {
				m.Enable()
			} else {
				m.Disable()
			}
		}(i)
	}

	wg.Wait()

	// 验证没有 panic
	status := m.GetStatus()
	t.Logf("Final status: enabled=%v, alerts=%d", status.Enabled, status.ActiveAlerts)
}

func TestConcurrentHoneypotAccess(t *testing.T) {
	m := NewManager()
	m.DeployHoneypots("/test", 5)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			m.GetHoneypots()
		}()

		go func(n int) {
			defer wg.Done()
			honeypots := m.GetHoneypots()
			if len(honeypots) > 0 {
				m.ProcessEvent(FileEvent{
					Path:      honeypots[0].Path,
					Operation: "modify",
					Timestamp: time.Now(),
				})
			}
			if n%10 == 0 {
				m.ResetHoneypots()
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentPathOperations(t *testing.T) {
	m := NewManager()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(2)

		go func(n int) {
			defer wg.Done()
			m.AddMonitoredPath(fmt.Sprintf("/path_%d", n))
		}(i)

		go func(n int) {
			defer wg.Done()
			m.RemoveMonitoredPath(fmt.Sprintf("/path_%d", n))
		}(i)
	}

	wg.Wait()
}

func TestEventWindowExpiry(t *testing.T) {
	m := NewManager()

	// 添加旧事件（超过时间窗口）
	oldTime := time.Now().Add(-2 * time.Duration(m.windowSec) * time.Second)
	for i := 0; i < 60; i++ {
		m.ProcessEvent(FileEvent{
			Path:      fmt.Sprintf("/old/file_%d.txt", i),
			Operation: "delete",
			Timestamp: oldTime,
		})
	}

	// 检查告警（旧事件不应触发）
	alerts := m.GetAlerts(false)
	for _, a := range alerts {
		if a.Type == AlertMassDelete {
			t.Error("old events should not trigger mass delete alert")
		}
	}
}

func TestAlertIDUniqueness(t *testing.T) {
	m := NewManager()

	// 快速创建多个告警
	for i := 0; i < 10; i++ {
		m.ProcessEvent(FileEvent{
			Path:      fmt.Sprintf("/test/file_%d.encrypted", i),
			Operation: "rename",
			Timestamp: time.Now(),
		})
		time.Sleep(time.Nanosecond) // 确保时间戳不同
	}

	alerts := m.GetAlerts(false)
	ids := make(map[string]bool)
	for _, a := range alerts {
		if ids[a.ID] {
			t.Errorf("duplicate alert ID: %s", a.ID)
		}
		ids[a.ID] = true
	}
}

func TestDefaultPaths(t *testing.T) {
	m := NewManager()

	// 验证默认路径存在
	found := map[string]bool{}
	for _, p := range m.monitoredPaths {
		found[p] = true
	}

	if !found["/data"] {
		t.Error("default path /data missing")
	}
	if !found["/shared"] {
		t.Error("default path /shared missing")
	}
}

func TestHoneypotPaths(t *testing.T) {
	m := NewManager()
	m.DeployHoneypots("/nas/documents", 3)

	honeypots := m.GetHoneypots()
	for _, hp := range honeypots {
		if hp.Path == "" || hp.RealPath == "" {
			t.Error("honeypot paths should not be empty")
		}
		if hp.Path == hp.RealPath {
			t.Error("path and real_path should be different")
		}
	}
}
