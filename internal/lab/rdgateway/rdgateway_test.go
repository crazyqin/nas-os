// Package rdgateway 测试
package rdgateway

import (
	"testing"
)

func TestCreateSession(t *testing.T) {
	sm := NewSessionManager(nil)
	session, err := sm.CreateSession(CreateSessionRequest{
		UserID:   "user1",
		Protocol: ProtocolRDP,
		Host:     "192.168.1.100",
		Port:     3389,
	})
	if err != nil {
		t.Fatalf("创建会话失败: %v", err)
	}
	if session.UserID != "user1" {
		t.Errorf("用户ID不匹配: %s", session.UserID)
	}
	if session.Protocol != ProtocolRDP {
		t.Errorf("协议不匹配: %s", session.Protocol)
	}
	if session.State != StateConnecting {
		t.Errorf("状态应为connecting: %s", session.State)
	}
	if len(session.Displays) != 1 {
		t.Errorf("默认显示器数应为1: %d", len(session.Displays))
	}
}

func TestCreateSessionDefaultPort(t *testing.T) {
	sm := NewSessionManager(nil)

	// RDP 默认端口
	s1, _ := sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolRDP, Host: "h"})
	if s1.Port != 3389 {
		t.Errorf("RDP默认端口应为3389: %d", s1.Port)
	}

	// VNC 默认端口
	s2, _ := sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolVNC, Host: "h"})
	if s2.Port != 5900 {
		t.Errorf("VNC默认端口应为5900: %d", s2.Port)
	}
}

func TestCreateSessionInvalidProtocol(t *testing.T) {
	sm := NewSessionManager(nil)
	_, err := sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: "ssh", Host: "h"})
	if err == nil {
		t.Error("无效协议应返回错误")
	}
}

func TestCreateSessionEmptyHost(t *testing.T) {
	sm := NewSessionManager(nil)
	_, err := sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolRDP, Host: ""})
	if err == nil {
		t.Error("空主机应返回错误")
	}
}

func TestGetSession(t *testing.T) {
	sm := NewSessionManager(nil)
	s, _ := sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolVNC, Host: "host"})

	got, err := sm.GetSession(s.ID)
	if err != nil {
		t.Fatalf("获取会话失败: %v", err)
	}
	if got.ID != s.ID {
		t.Errorf("会话ID不匹配")
	}
}

func TestGetSessionNotFound(t *testing.T) {
	sm := NewSessionManager(nil)
	_, err := sm.GetSession("nonexistent")
	if err == nil {
		t.Error("不存在的会话应返回错误")
	}
}

func TestListSessions(t *testing.T) {
	sm := NewSessionManager(nil)
	sm.CreateSession(CreateSessionRequest{UserID: "u1", Protocol: ProtocolRDP, Host: "h1"})
	sm.CreateSession(CreateSessionRequest{UserID: "u2", Protocol: ProtocolVNC, Host: "h2"})
	sm.CreateSession(CreateSessionRequest{UserID: "u1", Protocol: ProtocolRDP, Host: "h3"})

	all := sm.ListSessions("")
	if len(all) != 3 {
		t.Errorf("应有3个会话: %d", len(all))
	}

	u1 := sm.ListSessions("u1")
	if len(u1) != 2 {
		t.Errorf("u1应有2个会话: %d", len(u1))
	}
}

func TestUpdateSessionState(t *testing.T) {
	sm := NewSessionManager(nil)
	s, _ := sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolRDP, Host: "h"})

	err := sm.UpdateSessionState(s.ID, StateConnected)
	if err != nil {
		t.Fatalf("更新状态失败: %v", err)
	}

	got, _ := sm.GetSession(s.ID)
	if got.State != StateConnected {
		t.Errorf("状态不匹配: %s", got.State)
	}
	if got.ConnectedAt == nil {
		t.Error("连接时间应被设置")
	}
}

func TestDisconnectSession(t *testing.T) {
	sm := NewSessionManager(nil)
	s, _ := sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolRDP, Host: "h"})

	err := sm.DisconnectSession(s.ID)
	if err != nil {
		t.Fatalf("断开会话失败: %v", err)
	}

	got, _ := sm.GetSession(s.ID)
	if got.State != StateDisconnected {
		t.Errorf("状态应为disconnected: %s", got.State)
	}
}

func TestReconnectSession(t *testing.T) {
	sm := NewSessionManager(nil)
	s, _ := sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolRDP, Host: "h"})
	sm.DisconnectSession(s.ID)

	err := sm.ReconnectSession(s.ID)
	if err != nil {
		t.Fatalf("重连失败: %v", err)
	}

	got, _ := sm.GetSession(s.ID)
	if got.State != StateReconnecting {
		t.Errorf("状态应为reconnecting: %s", got.State)
	}
}

func TestReconnectSessionInvalidState(t *testing.T) {
	sm := NewSessionManager(nil)
	s, _ := sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolRDP, Host: "h"})

	// 连接中不允许重连
	err := sm.ReconnectSession(s.ID)
	if err == nil {
		t.Error("非断开状态应返回错误")
	}
}

func TestDeleteSession(t *testing.T) {
	sm := NewSessionManager(nil)
	s, _ := sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolRDP, Host: "h"})

	err := sm.DeleteSession(s.ID)
	if err != nil {
		t.Fatalf("删除会话失败: %v", err)
	}

	_, err = sm.GetSession(s.ID)
	if err == nil {
		t.Error("已删除的会话不应存在")
	}
}

func TestAuditLog(t *testing.T) {
	sm := NewSessionManager(nil)
	s, _ := sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolRDP, Host: "h"})
	sm.UpdateSessionState(s.ID, StateConnected)
	sm.DisconnectSession(s.ID)

	log := sm.GetAuditLog("", 0)
	if len(log) < 3 {
		t.Errorf("应至少有3条审计记录: %d", len(log))
	}

	// 按会话过滤
	sessionLog := sm.GetAuditLog(s.ID, 0)
	for _, entry := range sessionLog {
		if entry.SessionID != s.ID {
			t.Errorf("审计条目会话ID不匹配: %s", entry.SessionID)
		}
	}
}

func TestSessionCount(t *testing.T) {
	sm := NewSessionManager(nil)
	if sm.SessionCount() != 0 {
		t.Errorf("初始会话数应为0: %d", sm.SessionCount())
	}

	sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolRDP, Host: "h"})
	sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolVNC, Host: "h2"})
	if sm.SessionCount() != 2 {
		t.Errorf("会话数应为2: %d", sm.SessionCount())
	}
}

func TestCustomDisplays(t *testing.T) {
	sm := NewSessionManager(nil)
	displays := []DisplayInfo{
		{ID: 0, Width: 1920, Height: 1080, IsPrimary: true},
		{ID: 1, Width: 2560, Height: 1440, IsPrimary: false},
	}
	s, _ := sm.CreateSession(CreateSessionRequest{
		UserID:   "u",
		Protocol: ProtocolRDP,
		Host:     "h",
		Displays: displays,
	})
	if len(s.Displays) != 2 {
		t.Errorf("显示器数应为2: %d", len(s.Displays))
	}
}

func TestTLSEnabled(t *testing.T) {
	sm := NewSessionManager(nil)
	s, _ := sm.CreateSession(CreateSessionRequest{
		UserID:     "u",
		Protocol:   ProtocolRDP,
		Host:       "h",
		TLSEnabled: true,
	})
	if !s.TLSEnabled {
		t.Error("TLS应启用")
	}
}

func TestSessionManagerAuditLimit(t *testing.T) {
	sm := NewSessionManager(nil)
	sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolRDP, Host: "h"})
	sm.CreateSession(CreateSessionRequest{UserID: "u", Protocol: ProtocolVNC, Host: "h2"})

	log := sm.GetAuditLog("", 1)
	if len(log) != 1 {
		t.Errorf("限制应返回1条: %d", len(log))
	}
}
