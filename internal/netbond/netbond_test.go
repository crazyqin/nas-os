// Package netbond 网络绑定 - 测试
package netbond

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if len(m.ListBonds()) != 0 {
		t.Error("expected 0 bonds")
	}
}

func TestCreateBond(t *testing.T) {
	m := NewManager()

	if err := m.CreateBond("bond0", BondMode8023AD, []string{"eth0", "eth1"}); err != nil {
		t.Fatalf("CreateBond failed: %v", err)
	}

	// Duplicate
	if err := m.CreateBond("bond0", BondModeActiveBackup, []string{"eth2", "eth3"}); err == nil {
		t.Error("expected error on duplicate bond")
	}

	// Too few slaves
	if err := m.CreateBond("bond1", BondModeActiveBackup, []string{"eth0"}); err == nil {
		t.Error("expected error with < 2 slaves")
	}

	bond, err := m.GetBond("bond0")
	if err != nil {
		t.Fatalf("GetBond failed: %v", err)
	}
	if bond.Mode != BondMode8023AD {
		t.Errorf("expected 802.3ad, got %s", bond.Mode)
	}
	if len(bond.Slaves) != 2 {
		t.Errorf("expected 2 slaves, got %d", len(bond.Slaves))
	}
	if bond.State != BondStateDown {
		t.Errorf("expected down, got %s", bond.State)
	}
}

func TestBondUp(t *testing.T) {
	m := NewManager()
	m.CreateBond("bond0", BondModeActiveBackup, []string{"eth0", "eth1"})

	ip := IPConfig{
		IPv4:    "192.168.1.100",
		Netmask: "255.255.255.0",
		Gateway: "192.168.1.1",
		DNS:     []string{"8.8.8.8"},
	}

	if err := m.UpBond("bond0", ip); err != nil {
		t.Fatalf("UpBond failed: %v", err)
	}

	bond, _ := m.GetBond("bond0")
	if bond.State != BondStateUp {
		t.Errorf("expected up, got %s", bond.State)
	}
	if bond.IP.IPv4 != "192.168.1.100" {
		t.Errorf("expected 192.168.1.100, got %s", bond.IP.IPv4)
	}
	if bond.ActiveSlave == "" {
		t.Error("expected active slave")
	}

	for _, s := range bond.Slaves {
		if s.State != SlaveStateActive {
			t.Errorf("slave %s should be active, got %s", s.Name, s.State)
		}
	}
}

func TestBondDown(t *testing.T) {
	m := NewManager()
	m.CreateBond("bond0", BondModeActiveBackup, []string{"eth0", "eth1"})
	m.UpBond("bond0", IPConfig{IPv4: "10.0.0.1", Netmask: "255.255.255.0"})

	if err := m.DownBond("bond0"); err != nil {
		t.Fatalf("DownBond failed: %v", err)
	}

	bond, _ := m.GetBond("bond0")
	if bond.State != BondStateDown {
		t.Errorf("expected down, got %s", bond.State)
	}
	if bond.ActiveSlave != "" {
		t.Errorf("expected no active slave, got %s", bond.ActiveSlave)
	}
}

func TestDeleteBond(t *testing.T) {
	m := NewManager()
	m.CreateBond("bond0", BondModeActiveBackup, []string{"eth0", "eth1"})

	// Can't delete while up
	m.UpBond("bond0", IPConfig{IPv4: "10.0.0.1", Netmask: "255.255.255.0"})
	if err := m.DeleteBond("bond0"); err == nil {
		t.Error("expected error deleting active bond")
	}

	m.DownBond("bond0")
	if err := m.DeleteBond("bond0"); err != nil {
		t.Fatalf("DeleteBond failed: %v", err)
	}

	if err := m.DeleteBond("nonexistent"); err == nil {
		t.Error("expected error on nonexistent bond")
	}
}

func TestAddRemoveSlave(t *testing.T) {
	m := NewManager()
	m.CreateBond("bond0", BondModeActiveBackup, []string{"eth0", "eth1"})

	// Add
	if err := m.AddSlave("bond0", "eth2"); err != nil {
		t.Fatalf("AddSlave failed: %v", err)
	}
	bond, _ := m.GetBond("bond0")
	if len(bond.Slaves) != 3 {
		t.Errorf("expected 3 slaves, got %d", len(bond.Slaves))
	}

	// Duplicate
	if err := m.AddSlave("bond0", "eth2"); err == nil {
		t.Error("expected error on duplicate slave")
	}

	// Remove
	if err := m.RemoveSlave("bond0", "eth2"); err != nil {
		t.Fatalf("RemoveSlave failed: %v", err)
	}
	bond, _ = m.GetBond("bond0")
	if len(bond.Slaves) != 2 {
		t.Errorf("expected 2 slaves, got %d", len(bond.Slaves))
	}

	// Can't remove below 2
	if err := m.RemoveSlave("bond0", "eth1"); err == nil {
		t.Error("expected error removing below 2 slaves")
	}
}

func TestSetMTU(t *testing.T) {
	m := NewManager()
	m.CreateBond("bond0", BondModeActiveBackup, []string{"eth0", "eth1"})

	if err := m.SetMTU("bond0", 9000); err != nil {
		t.Fatalf("SetMTU failed: %v", err)
	}
	bond, _ := m.GetBond("bond0")
	if bond.MTU != 9000 {
		t.Errorf("expected 9000, got %d", bond.MTU)
	}

	// Invalid MTU
	if err := m.SetMTU("bond0", 50); err == nil {
		t.Error("expected error on invalid MTU")
	}
	if err := m.SetMTU("bond0", 10000); err == nil {
		t.Error("expected error on MTU > 9216")
	}
}

func TestVLANs(t *testing.T) {
	m := NewManager()

	config := VLANConfig{
		ID:     100,
		Parent: "bond0",
		IP:     IPConfig{IPv4: "10.100.0.1", Netmask: "255.255.255.0"},
		Enabled: true,
	}

	if err := m.CreateVLAN(config); err != nil {
		t.Fatalf("CreateVLAN failed: %v", err)
	}

	vlans := m.ListVLANs()
	if len(vlans) != 1 {
		t.Errorf("expected 1 VLAN, got %d", len(vlans))
	}
	if vlans[0].Name != "bond0.100" {
		t.Errorf("expected 'bond0.100', got %q", vlans[0].Name)
	}

	// Duplicate
	if err := m.CreateVLAN(config); err == nil {
		t.Error("expected error on duplicate VLAN")
	}

	// Delete
	if err := m.DeleteVLAN("bond0.100"); err != nil {
		t.Fatalf("DeleteVLAN failed: %v", err)
	}
	if err := m.DeleteVLAN("nonexistent"); err == nil {
		t.Error("expected error on nonexistent VLAN")
	}
}

func TestStats(t *testing.T) {
	m := NewManager()

	stats := NetworkStats{
		Interface: "eth0",
		RxBytes:   1024,
		TxBytes:   512,
		Speed:     1000,
	}

	m.UpdateStats("eth0", stats)
	got, err := m.GetStats("eth0")
	if err != nil {
		t.Fatalf("GetStats failed: %v", err)
	}
	if got.RxBytes != 1024 {
		t.Errorf("expected 1024, got %d", got.RxBytes)
	}
	if got.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}

	all := m.GetAllStats()
	if len(all) != 1 {
		t.Errorf("expected 1 stat, got %d", len(all))
	}

	if _, err := m.GetStats("nonexistent"); err == nil {
		t.Error("expected error on nonexistent stats")
	}
}

func TestBondModes(t *testing.T) {
	modes := map[BondMode]string{
		BondModeRoundRobin:   "round_robin",
		BondModeActiveBackup: "active_backup",
		BondModeXOR:          "xor",
		BondModeBroadcast:    "broadcast",
		BondMode8023AD:       "802.3ad",
		BondModeBalanceTLB:   "balance_tlb",
		BondModeBalanceALB:   "balance_alb",
	}
	for mode, expected := range modes {
		if string(mode) != expected {
			t.Errorf("mode %v != %q", mode, expected)
		}
	}
}

func TestExportImport(t *testing.T) {
	m := NewManager()
	m.CreateBond("bond0", BondMode8023AD, []string{"eth0", "eth1"})
	m.CreateVLAN(VLANConfig{ID: 100, Parent: "bond0", Enabled: true})

	data, err := m.ExportConfig()
	if err != nil {
		t.Fatalf("ExportConfig failed: %v", err)
	}

	m2 := NewManager()
	if err := m2.ImportConfig(data); err != nil {
		t.Fatalf("ImportConfig failed: %v", err)
	}

	bonds := m2.ListBonds()
	if len(bonds) != 1 {
		t.Errorf("expected 1 bond, got %d", len(bonds))
	}
	vlans := m2.ListVLANs()
	if len(vlans) != 1 {
		t.Errorf("expected 1 VLAN, got %d", len(vlans))
	}
}

func TestCallback(t *testing.T) {
	m := NewManager()
	var lastEvent string
	m.SetOnChangeCallback(func(name, event string) {
		lastEvent = event
	})

	m.CreateBond("bond0", BondModeActiveBackup, []string{"eth0", "eth1"})
	m.UpBond("bond0", IPConfig{IPv4: "10.0.0.1", Netmask: "255.255.255.0"})
	time.Sleep(10 * time.Millisecond)
	if lastEvent != "up" {
		t.Errorf("expected 'up', got %q", lastEvent)
	}
}
