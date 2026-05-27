package networkbond

import (
	"testing"
)

func TestCreateBond(t *testing.T) {
	mgr := NewBondManager(nil)
	bond, err := mgr.CreateBond("bond0", IEEE802_3ad, []string{"eth0", "eth1"})
	if err != nil {
		t.Fatalf("CreateBond failed: %v", err)
	}
	if bond.Name != "bond0" {
		t.Errorf("expected name bond0, got %s", bond.Name)
	}
	if bond.Mode != IEEE802_3ad {
		t.Errorf("expected mode IEEE802_3ad, got %d", bond.Mode)
	}
	if len(bond.Interfaces) != 2 {
		t.Errorf("expected 2 interfaces, got %d", len(bond.Interfaces))
	}
}

func TestCreateBondDuplicate(t *testing.T) {
	mgr := NewBondManager(nil)
	mgr.CreateBond("bond0", ActiveBackup, []string{"eth0", "eth1"})
	_, err := mgr.CreateBond("bond0", ActiveBackup, []string{"eth2", "eth3"})
	if err == nil {
		t.Error("expected error for duplicate bond")
	}
}

func TestLACPRequiresTwoInterfaces(t *testing.T) {
	mgr := NewBondManager(nil)
	_, err := mgr.CreateBond("bond0", IEEE802_3ad, []string{"eth0"})
	if err == nil {
		t.Error("expected error for LACP with 1 interface")
	}
}

func TestActivateDeactivate(t *testing.T) {
	mgr := NewBondManager(nil)
	mgr.CreateBond("bond0", ActiveBackup, []string{"eth0", "eth1"})
	mgr.ActivateBond("bond0")
	bond, _ := mgr.GetBond("bond0")
	if bond.State != StateUp {
		t.Errorf("expected up, got %s", bond.State)
	}
	mgr.DeactivateBond("bond0")
	bond, _ = mgr.GetBond("bond0")
	if bond.State != StateDown {
		t.Errorf("expected down, got %s", bond.State)
	}
}

func TestAddRemoveInterface(t *testing.T) {
	mgr := NewBondManager(nil)
	mgr.CreateBond("bond0", BalanceRR, []string{"eth0", "eth1"})
	mgr.AddInterface("bond0", "eth2")
	bond, _ := mgr.GetBond("bond0")
	if len(bond.Interfaces) != 3 {
		t.Errorf("expected 3, got %d", len(bond.Interfaces))
	}
	mgr.RemoveInterface("bond0", "eth1")
	bond, _ = mgr.GetBond("bond0")
	if len(bond.Interfaces) != 2 {
		t.Errorf("expected 2, got %d", len(bond.Interfaces))
	}
}

func TestFailover(t *testing.T) {
	mgr := NewBondManager(nil)
	mgr.CreateBond("bond0", ActiveBackup, []string{"eth0", "eth1"})
	mgr.ActivateBond("bond0")
	mgr.FailoverTrigger("bond0", "eth0")
	bond, _ := mgr.GetBond("bond0")
	if bond.ActiveSlave != "eth1" {
		t.Errorf("expected eth1, got %s", bond.ActiveSlave)
	}
}

func TestGetBondStats(t *testing.T) {
	mgr := NewBondManager(nil)
	mgr.CreateBond("bond0", IEEE802_3ad, []string{"eth0", "eth1"})
	mgr.ActivateBond("bond0")
	stats, err := mgr.GetBondStats("bond0")
	if err != nil {
		t.Fatalf("GetBondStats failed: %v", err)
	}
	if stats.ActiveInterfaces != 2 {
		t.Errorf("expected 2 active, got %d", stats.ActiveInterfaces)
	}
}

func TestGetBondModeName(t *testing.T) {
	tests := []struct {
		mode BondMode
		want string
	}{
		{BalanceRR, "balance-rr"},
		{ActiveBackup, "active-backup"},
		{IEEE802_3ad, "802.3ad"},
	}
	for _, tt := range tests {
		got := GetBondModeName(tt.mode)
		if got != tt.want {
			t.Errorf("GetBondModeName(%d) = %s, want %s", tt.mode, got, tt.want)
		}
	}
}

func TestListBonds(t *testing.T) {
	mgr := NewBondManager(nil)
	mgr.CreateBond("bond0", ActiveBackup, []string{"eth0", "eth1"})
	mgr.CreateBond("bond1", IEEE802_3ad, []string{"eth2", "eth3"})
	bonds := mgr.ListBonds()
	if len(bonds) != 2 {
		t.Errorf("expected 2, got %d", len(bonds))
	}
}
