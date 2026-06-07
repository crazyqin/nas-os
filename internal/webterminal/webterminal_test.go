// webterminal_test.go - Web 终端测试
package webterminal

import (
	"fmt"
	"testing"
	"time"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager()
}

func TestNewManager(t *testing.T) {
	m := newTestManager(t)
	if m == nil {
		t.Fatal("NewManager 返回 nil")
	}
	if m.config.MaxSessions != 10 {
		t.Errorf("MaxSessions: 期望 10, 实际 %d", m.config.MaxSessions)
	}
}

func TestDefaultConfig(t *testing.T) {
	m := newTestManager(t)
	config := m.GetConfig()

	if config.DefaultShell != "/bin/bash" {
		t.Errorf("DefaultShell: 期望 /bin/bash, 实际 %s", config.DefaultShell)
	}
	if config.IdleTimeout == 0 {
		t.Error("IdleTimeout 不应为 0")
	}
}

func TestUpdateConfig(t *testing.T) {
	m := newTestManager(t)

	config := m.GetConfig()
	config.MaxSessions = 20
	config.AllowRoot = true
	m.UpdateConfig(config)

	got := m.GetConfig()
	if got.MaxSessions != 20 {
		t.Errorf("MaxSessions: 期望 20, 实际 %d", got.MaxSessions)
	}
	if !got.AllowRoot {
		t.Error("AllowRoot 应为 true")
	}
}

func TestListSessionsEmpty(t *testing.T) {
	m := newTestManager(t)
	sessions := m.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("初始应无会话, 实际 %d", len(sessions))
	}
}

func TestActiveCount(t *testing.T) {
	m := newTestManager(t)
	if m.ActiveCount() != 0 {
		t.Error("初始活跃会话数应为 0")
	}
}

func TestGetSessionNotFound(t *testing.T) {
	m := newTestManager(t)
	_, err := m.GetSession("nonexistent")
	if err == nil {
		t.Error("不存在的会话应返回错误")
	}
}

func TestCloseSessionNotFound(t *testing.T) {
	m := newTestManager(t)
	err := m.CloseSession("nonexistent")
	if err == nil {
		t.Error("关闭不存在的会话应返回错误")
	}
}

func TestManualSessionLifecycle(t *testing.T) {
	m := newTestManager(t)

	// 手动添加会话
	m.mu.Lock()
	m.sessions["test-1"] = &TerminalSession{
		ID:     "test-1",
		User:   "admin",
		Status: "active",
		Cols:   80,
		Rows:   24,
	}
	m.mu.Unlock()

	// 列出会话
	sessions := m.ListSessions()
	if len(sessions) != 1 {
		t.Fatalf("期望 1 个会话, 实际 %d", len(sessions))
	}
	if sessions[0].User != "admin" {
		t.Errorf("用户: 期望 admin, 实际 %s", sessions[0].User)
	}

	// 活跃数
	if m.ActiveCount() != 1 {
		t.Errorf("活跃数: 期望 1, 实际 %d", m.ActiveCount())
	}

	// 获取会话
	s, err := m.GetSession("test-1")
	if err != nil {
		t.Fatalf("GetSession 失败: %v", err)
	}
	if s.ID != "test-1" {
		t.Errorf("ID: 期望 test-1, 实际 %s", s.ID)
	}

	// 关闭会话
	err = m.CloseSession("test-1")
	if err != nil {
		t.Fatalf("CloseSession 失败: %v", err)
	}
	if m.ActiveCount() != 0 {
		t.Error("关闭后活跃数应为 0")
	}
}

func TestCleanupIdle(t *testing.T) {
	m := newTestManager(t)

	// 设置极短超时
	m.config.IdleTimeout = 0

	m.mu.Lock()
	m.sessions["idle-1"] = &TerminalSession{
		ID:         "idle-1",
		Status:     "active",
		LastActive: time.Now().Add(-time.Hour),
	}
	m.mu.Unlock()

	cleaned := m.CleanupIdle()
	if cleaned != 1 {
		t.Errorf("清理数: 期望 1, 实际 %d", cleaned)
	}
	if m.ActiveCount() != 0 {
		t.Error("清理后活跃数应为 0")
	}
}

func TestMultipleSessions(t *testing.T) {
	m := newTestManager(t)

	m.mu.Lock()
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("term-%d", i)
		m.sessions[id] = &TerminalSession{
			ID:     id,
			Status: "active",
			User:   fmt.Sprintf("user%d", i),
		}
	}
	m.mu.Unlock()

	if m.ActiveCount() != 5 {
		t.Errorf("活跃数: 期望 5, 实际 %d", m.ActiveCount())
	}

	sessions := m.ListSessions()
	if len(sessions) != 5 {
		t.Errorf("会话数: 期望 5, 实际 %d", len(sessions))
	}
}
