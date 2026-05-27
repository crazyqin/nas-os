package remotedesktop

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	config := &Config{
		Enabled:          true,
		WebSocketPort:    8080,
		MaxSessions:      10,
		RecordingEnabled: true,
		ClipSyncEnabled:  true,
		DefaultWidth:     1920,
		DefaultHeight:    1080,
		DefaultColor:     32,
		SessionTimeout:   30,
	}
	manager := NewManager(config)
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}
}

func TestCreateSession(t *testing.T) {
	config := &Config{
		Enabled:        true,
		MaxSessions:    5,
		DefaultWidth:   1920,
		DefaultHeight:  1080,
		DefaultColor:   32,
		SessionTimeout: 30,
	}
	manager := NewManager(config)

	host := &Host{
		Name:     "test-server",
		Hostname: "192.168.1.100",
		Protocol: ProtocolVNC,
		Port:     5900,
		Username: "admin",
		Enabled:  true,
	}
	if err := manager.AddHost(host); err != nil {
		t.Fatalf("AddHost failed: %v", err)
	}

	session, err := manager.CreateSession(host.ID, "user1")
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if session.Protocol != ProtocolVNC {
		t.Errorf("expected protocol vnc, got %s", session.Protocol)
	}
	if session.Status != StatusConnecting {
		t.Errorf("expected status connecting, got %s", session.Status)
	}
}

func TestEndSession(t *testing.T) {
	config := &Config{Enabled: true, MaxSessions: 5, DefaultWidth: 1920, DefaultHeight: 1080, DefaultColor: 32}
	manager := NewManager(config)

	host := &Host{Name: "h", Hostname: "1.2.3.4", Protocol: ProtocolRDP, Port: 3389, Enabled: true}
	manager.AddHost(host)

	session, _ := manager.CreateSession(host.ID, "user1")
	if err := manager.EndSession(session.ID); err != nil {
		t.Fatalf("EndSession failed: %v", err)
	}

	s, _ := manager.GetSession(session.ID)
	if s.Status != StatusDisconnected {
		t.Errorf("expected disconnected, got %s", s.Status)
	}
}

func TestListHosts(t *testing.T) {
	config := &Config{Enabled: true, MaxSessions: 5}
	manager := NewManager(config)

	manager.AddHost(&Host{Name: "h1", Hostname: "1.1.1.1", Protocol: ProtocolVNC, Port: 5900, Enabled: true})
	manager.AddHost(&Host{Name: "h2", Hostname: "2.2.2.2", Protocol: ProtocolRDP, Port: 3389, Enabled: true})

	hosts := manager.ListHosts()
	if len(hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(hosts))
	}
}

func TestGetStats(t *testing.T) {
	config := &Config{Enabled: true, MaxSessions: 10}
	manager := NewManager(config)

	stats := manager.GetStats()
	if stats.TotalSessions != 0 {
		t.Errorf("expected 0 sessions, got %d", stats.TotalSessions)
	}
}
