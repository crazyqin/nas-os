package wanoptimize

import (
	"testing"
)

func TestManager_CreateTunnel(t *testing.T) {
	m := NewManager()

	req := CreateTunnelRequest{
		Name:       "测试隧道",
		LocalAddr:  "192.168.1.100",
		RemoteAddr: "10.0.0.1",
		Port:       8443,
		Compress:   CompressLZ4,
		Encrypt:    true,
	}

	tunnel, err := m.CreateTunnel(req)
	if err != nil {
		t.Fatalf("创建隧道失败: %v", err)
	}
	if tunnel.Name != "测试隧道" {
		t.Errorf("期望名称 '测试隧道', 得到 '%s'", tunnel.Name)
	}
	if tunnel.Status != TunnelStatusInactive {
		t.Errorf("期望状态 inactive, 得到 %s", tunnel.Status)
	}
}

func TestManager_CreateTunnel_DefaultCompress(t *testing.T) {
	m := NewManager()

	req := CreateTunnelRequest{
		Name:       "默认压缩",
		LocalAddr:  "192.168.1.100",
		RemoteAddr: "10.0.0.1",
	}

	tunnel, err := m.CreateTunnel(req)
	if err != nil {
		t.Fatalf("创建隧道失败: %v", err)
	}
	if tunnel.Compress != CompressLZ4 {
		t.Errorf("期望默认压缩 LZ4, 得到 %s", tunnel.Compress)
	}
}

func TestManager_GetTunnel(t *testing.T) {
	m := NewManager()

	req := CreateTunnelRequest{Name: "获取测试", LocalAddr: "a", RemoteAddr: "b"}
	tunnel, _ := m.CreateTunnel(req)

	got, err := m.GetTunnel(tunnel.ID)
	if err != nil {
		t.Fatalf("获取隧道失败: %v", err)
	}
	if got.ID != tunnel.ID {
		t.Errorf("ID不匹配")
	}
}

func TestManager_GetTunnel_NotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetTunnel("nonexistent")
	if err != ErrTunnelNotFound {
		t.Errorf("期望 ErrTunnelNotFound, 得到 %v", err)
	}
}

func TestManager_ListTunnels(t *testing.T) {
	m := NewManager()

	m.CreateTunnel(CreateTunnelRequest{Name: "t1", LocalAddr: "a", RemoteAddr: "b"})
	m.CreateTunnel(CreateTunnelRequest{Name: "t2", LocalAddr: "c", RemoteAddr: "d"})

	tunnels := m.ListTunnels()
	if len(tunnels) != 2 {
		t.Errorf("期望2个隧道, 得到 %d", len(tunnels))
	}
}

func TestManager_DeleteTunnel(t *testing.T) {
	m := NewManager()

	tunnel, _ := m.CreateTunnel(CreateTunnelRequest{Name: "删除测试", LocalAddr: "a", RemoteAddr: "b"})

	if err := m.DeleteTunnel(tunnel.ID); err != nil {
		t.Fatalf("删除失败: %v", err)
	}
}

func TestManager_DeleteTunnel_Active(t *testing.T) {
	m := NewManager()

	tunnel, _ := m.CreateTunnel(CreateTunnelRequest{Name: "活跃测试", LocalAddr: "a", RemoteAddr: "b"})
	m.ConnectTunnel(tunnel.ID)

	err := m.DeleteTunnel(tunnel.ID)
	if err != ErrTunnelActive {
		t.Errorf("期望 ErrTunnelActive, 得到 %v", err)
	}
}

func TestManager_ConnectDisconnect(t *testing.T) {
	m := NewManager()

	tunnel, _ := m.CreateTunnel(CreateTunnelRequest{Name: "连接测试", LocalAddr: "a", RemoteAddr: "b"})

	if err := m.ConnectTunnel(tunnel.ID); err != nil {
		t.Fatalf("连接失败: %v", err)
	}

	got, _ := m.GetTunnel(tunnel.ID)
	if got.Status != TunnelStatusActive {
		t.Errorf("期望 active, 得到 %s", got.Status)
	}

	m.DisconnectTunnel(tunnel.ID)
	got, _ = m.GetTunnel(tunnel.ID)
	if got.Status != TunnelStatusInactive {
		t.Errorf("期望 inactive, 得到 %s", got.Status)
	}
}

func TestManager_GetStats(t *testing.T) {
	m := NewManager()

	m.CreateTunnel(CreateTunnelRequest{Name: "s1", LocalAddr: "a", RemoteAddr: "b"})
	m.CreateTunnel(CreateTunnelRequest{Name: "s2", LocalAddr: "c", RemoteAddr: "d"})

	stats := m.GetStats()
	if stats.TotalTunnels != 2 {
		t.Errorf("期望2个隧道, 得到 %d", stats.TotalTunnels)
	}
}
