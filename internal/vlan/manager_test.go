package vlan

import (
	"testing"
)

func TestCreateVLAN(t *testing.T) {
	m := NewManager()

	vlan, err := m.Create("eth0", 100, "192.168.100.1", "255.255.255.0", "192.168.100.254", 1500)
	if err != nil {
		t.Fatalf("创建 VLAN 失败: %v", err)
	}

	if vlan.ID != 100 {
		t.Errorf("期望 VLAN ID 100，实际 %d", vlan.ID)
	}

	if vlan.Name != "eth0.100" {
		t.Errorf("期望接口名 eth0.100，实际 %s", vlan.Name)
	}

	if vlan.IPAddr != "192.168.100.1" {
		t.Errorf("期望 IP 192.168.100.1，实际 %s", vlan.IPAddr)
	}

	if vlan.Status != "down" {
		t.Errorf("期望状态 down，实际 %s", vlan.Status)
	}
}

func TestCreateDuplicateVLAN(t *testing.T) {
	m := NewManager()

	_, err := m.Create("eth0", 100, "192.168.100.1", "255.255.255.0", "", 1500)
	if err != nil {
		t.Fatalf("创建 VLAN 失败: %v", err)
	}

	_, err = m.Create("eth0", 100, "192.168.100.2", "255.255.255.0", "", 1500)
	if err == nil {
		t.Fatal("期望创建重复 VLAN 失败，但成功了")
	}
}

func TestInvalidVLANID(t *testing.T) {
	m := NewManager()

	_, err := m.Create("eth0", 0, "192.168.1.1", "255.255.255.0", "", 1500)
	if err == nil {
		t.Fatal("期望 VLAN ID 0 失败，但成功了")
	}

	_, err = m.Create("eth0", 4095, "192.168.1.1", "255.255.255.0", "", 1500)
	if err == nil {
		t.Fatal("期望 VLAN ID 4095 失败，但成功了")
	}
}

func TestInvalidIP(t *testing.T) {
	m := NewManager()

	_, err := m.Create("eth0", 100, "invalid-ip", "255.255.255.0", "", 1500)
	if err == nil {
		t.Fatal("期望无效 IP 失败，但成功了")
	}
}

func TestDeleteVLAN(t *testing.T) {
	m := NewManager()

	_, err := m.Create("eth0", 100, "192.168.100.1", "255.255.255.0", "", 1500)
	if err != nil {
		t.Fatalf("创建 VLAN 失败: %v", err)
	}

	err = m.Delete(100)
	if err != nil {
		t.Fatalf("删除 VLAN 失败: %v", err)
	}

	_, err = m.Get(100)
	if err == nil {
		t.Fatal("期望获取已删除 VLAN 失败，但成功了")
	}
}

func TestUpdateVLAN(t *testing.T) {
	m := NewManager()

	_, err := m.Create("eth0", 100, "192.168.100.1", "255.255.255.0", "", 1500)
	if err != nil {
		t.Fatalf("创建 VLAN 失败: %v", err)
	}

	vlan, err := m.Update(100, "192.168.100.2", "", "192.168.100.254", 9000, []string{"production"})
	if err != nil {
		t.Fatalf("更新 VLAN 失败: %v", err)
	}

	if vlan.IPAddr != "192.168.100.2" {
		t.Errorf("期望 IP 192.168.100.2，实际 %s", vlan.IPAddr)
	}

	if vlan.MTU != 9000 {
		t.Errorf("期望 MTU 9000，实际 %d", vlan.MTU)
	}

	if len(vlan.Tags) != 1 || vlan.Tags[0] != "production" {
		t.Errorf("期望标签 [production]，实际 %v", vlan.Tags)
	}
}

func TestEnableDisable(t *testing.T) {
	m := NewManager()

	_, err := m.Create("eth0", 100, "192.168.100.1", "255.255.255.0", "", 1500)
	if err != nil {
		t.Fatalf("创建 VLAN 失败: %v", err)
	}

	err = m.Enable(100)
	if err != nil {
		t.Fatalf("启用 VLAN 失败: %v", err)
	}

	vlan, _ := m.Get(100)
	if vlan.Status != "up" {
		t.Errorf("期望状态 up，实际 %s", vlan.Status)
	}

	err = m.Disable(100)
	if err != nil {
		t.Fatalf("禁用 VLAN 失败: %v", err)
	}

	vlan, _ = m.Get(100)
	if vlan.Status != "down" {
		t.Errorf("期望状态 down，实际 %s", vlan.Status)
	}
}

func TestListVLANs(t *testing.T) {
	m := NewManager()

	m.Create("eth0", 100, "192.168.100.1", "255.255.255.0", "", 1500)
	m.Create("eth0", 200, "192.168.200.1", "255.255.255.0", "", 1500)
	m.Create("eth1", 300, "10.0.0.1", "255.255.255.0", "", 1500)

	vlans := m.List()
	if len(vlans) != 3 {
		t.Errorf("期望 3 个 VLAN，实际 %d", len(vlans))
	}
}

func TestGetByParent(t *testing.T) {
	m := NewManager()

	m.Create("eth0", 100, "192.168.100.1", "255.255.255.0", "", 1500)
	m.Create("eth0", 200, "192.168.200.1", "255.255.255.0", "", 1500)
	m.Create("eth1", 300, "10.0.0.1", "255.255.255.0", "", 1500)

	eth0VLANs := m.GetByParent("eth0")
	if len(eth0VLANs) != 2 {
		t.Errorf("期望 eth0 有 2 个 VLAN，实际 %d", len(eth0VLANs))
	}
}

func TestDefaultMTU(t *testing.T) {
	m := NewManager()

	vlan, err := m.Create("eth0", 100, "192.168.100.1", "255.255.255.0", "", 0)
	if err != nil {
		t.Fatalf("创建 VLAN 失败: %v", err)
	}

	if vlan.MTU != 1500 {
		t.Errorf("期望默认 MTU 1500，实际 %d", vlan.MTU)
	}
}
