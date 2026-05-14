package sshmanager

import (
	"testing"
)

func TestAddAndGetKey(t *testing.T) {
	mgr := NewManager()
	key := &SSHKey{ID: "k1", Name: "my-key", Type: "ed25519", PublicKey: "ssh-ed25519 AAAA..."}
	mgr.AddKey(key)
	got, ok := mgr.GetKey("k1")
	if !ok {
		t.Fatal("expected key to exist")
	}
	if got.Name != "my-key" {
		t.Errorf("expected my-key, got %s", got.Name)
	}
}

func TestListKeys(t *testing.T) {
	mgr := NewManager()
	mgr.AddKey(&SSHKey{ID: "k1", Name: "a", Type: "rsa"})
	mgr.AddKey(&SSHKey{ID: "k2", Name: "b", Type: "ed25519"})
	keys := mgr.ListKeys()
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestDeleteKey(t *testing.T) {
	mgr := NewManager()
	mgr.AddKey(&SSHKey{ID: "k1", Name: "a", Type: "rsa"})
	if !mgr.DeleteKey("k1") {
		t.Error("expected delete to succeed")
	}
	if _, ok := mgr.GetKey("k1"); ok {
		t.Error("expected key to be deleted")
	}
}

func TestDeleteKeyNotFound(t *testing.T) {
	mgr := NewManager()
	if mgr.DeleteKey("nonexistent") {
		t.Error("expected false")
	}
}

func TestCreateSession(t *testing.T) {
	mgr := NewManager()
	sess, err := mgr.CreateSession("192.168.1.1", 22, "root", "k1")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.Host != "192.168.1.1" {
		t.Errorf("expected 192.168.1.1, got %s", sess.Host)
	}
	if sess.Status != StatusActive {
		t.Errorf("expected active, got %s", sess.Status)
	}
}

func TestMaxSessions(t *testing.T) {
	mgr := NewManager()
	mgr.config.MaxSessions = 2
	mgr.CreateSession("h1", 22, "u", "k1")
	mgr.CreateSession("h2", 22, "u", "k1")
	_, err := mgr.CreateSession("h3", 22, "u", "k1")
	if err != ErrMaxSessionsReached {
		t.Errorf("expected ErrMaxSessionsReached, got %v", err)
	}
}

func TestCloseSession(t *testing.T) {
	mgr := NewManager()
	sess, _ := mgr.CreateSession("h1", 22, "u", "k1")
	if !mgr.CloseSession(sess.ID) {
		t.Error("expected close to succeed")
	}
	got, _ := mgr.GetSession(sess.ID)
	if got.Status != StatusClosed {
		t.Errorf("expected closed, got %s", got.Status)
	}
	if got.ClosedAt == nil {
		t.Error("expected ClosedAt to be set")
	}
}

func TestListSessionsActiveOnly(t *testing.T) {
	mgr := NewManager()
	s1, _ := mgr.CreateSession("h1", 22, "u", "k1")
	mgr.CreateSession("h2", 22, "u", "k1")
	mgr.CloseSession(s1.ID)
	active := mgr.ListSessions(true)
	if len(active) != 1 {
		t.Errorf("expected 1 active, got %d", len(active))
	}
}

func TestCreateTunnel(t *testing.T) {
	mgr := NewManager()
	sess, _ := mgr.CreateSession("h1", 22, "u", "k1")
	tun := mgr.CreateTunnel("my-tunnel", sess.ID, "localhost:8080", "remote:80")
	if tun.Name != "my-tunnel" {
		t.Errorf("expected my-tunnel, got %s", tun.Name)
	}
	if !tun.Enabled {
		t.Error("expected tunnel to be enabled")
	}
}

func TestListAndDeleteTunnel(t *testing.T) {
	mgr := NewManager()
	mgr.CreateTunnel("t1", "s1", "local:80", "remote:80")
	mgr.CreateTunnel("t2", "s1", "local:443", "remote:443")
	tunnels := mgr.ListTunnels()
	if len(tunnels) != 2 {
		t.Errorf("expected 2 tunnels, got %d", len(tunnels))
	}
	mgr.DeleteTunnel(tunnels[0].ID)
	if len(mgr.ListTunnels()) != 1 {
		t.Error("expected 1 tunnel after delete")
	}
}

func TestConfig(t *testing.T) {
	mgr := NewManager()
	cfg := mgr.GetConfig()
	if cfg.DefaultPort != 22 {
		t.Errorf("expected 22, got %d", cfg.DefaultPort)
	}
	cfg.DefaultPort = 2222
	mgr.UpdateConfig(cfg)
	if mgr.GetConfig().DefaultPort != 2222 {
		t.Errorf("expected 2222, got %d", mgr.GetConfig().DefaultPort)
	}
}

func TestStats(t *testing.T) {
	mgr := NewManager()
	mgr.AddKey(&SSHKey{ID: "k1", Name: "a", Type: "rsa"})
	mgr.CreateSession("h1", 22, "u", "k1")
	stats := mgr.GetStats()
	if stats["total_keys"] != 1 {
		t.Errorf("expected 1 key, got %v", stats["total_keys"])
	}
	if stats["active_sessions"] != 1 {
		t.Errorf("expected 1 active, got %v", stats["active_sessions"])
	}
}
