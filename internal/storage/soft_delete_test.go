package storage

import (
	"testing"
	"time"
)

func TestSoftDetach_RestoreWithinGrace(t *testing.T) {
	m := &Manager{volumes: map[string]*Volume{
		"tank": {Name: "tank", UUID: "u1", Devices: []string{"/dev/sda"}},
	}}
	m.SetDeleteGracePeriod(time.Hour)
	defer m.StopSoftDeleteReaper()

	if err := m.DeleteVolumeConfirmed("tank", DeleteVolumeOptions{
		ConfirmName: "tank",
		AllowWipe:   false,
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.volumes["tank"]; ok {
		t.Fatal("tank should not be active after soft delete")
	}
	pending := m.ListPendingDeletions()
	if len(pending) != 1 || pending[0].Volume == nil || pending[0].Volume.Name != "tank" {
		t.Fatalf("want 1 pending tank, got %+v", pending)
	}
	if err := m.RestorePending("tank"); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.volumes["tank"]; !ok {
		t.Fatal("tank should be restored")
	}
	if len(m.ListPendingDeletions()) != 0 {
		t.Fatal("pending should be empty after restore")
	}
}

func TestSoftDetach_PurgePending(t *testing.T) {
	m := &Manager{volumes: map[string]*Volume{
		"tank": {Name: "tank"},
	}}
	defer m.StopSoftDeleteReaper()
	if err := m.SoftDetachVolume("tank", false, false, time.Hour); err != nil {
		t.Fatal(err)
	}
	if err := m.PurgePending("tank"); err != nil {
		t.Fatal(err)
	}
	if len(m.ListPendingDeletions()) != 0 {
		t.Fatal("pending should be empty")
	}
	if err := m.RestorePending("tank"); err == nil {
		t.Fatal("restore after purge should fail")
	}
}

func TestSoftDetach_SkipGraceImmediate(t *testing.T) {
	m := &Manager{volumes: map[string]*Volume{
		"tank": {Name: "tank"},
		"data": {Name: "data"},
	}}
	if err := m.DeleteVolumeConfirmed("tank", DeleteVolumeOptions{
		ConfirmName: "tank",
		SkipGrace:   true,
	}); err != nil {
		t.Fatal(err)
	}
	if _, ok := m.volumes["tank"]; ok {
		t.Fatal("tank should be gone")
	}
	if len(m.ListPendingDeletions()) != 0 {
		t.Fatal("skip_grace should not create pending")
	}
	if _, ok := m.volumes["data"]; !ok {
		t.Fatal("data should remain")
	}
}

func TestProductRegistry_styleGraceDefault(t *testing.T) {
	m := &Manager{volumes: map[string]*Volume{}}
	if m.DeleteGracePeriod() != DefaultDeleteGracePeriod {
		t.Fatalf("default grace %v", m.DeleteGracePeriod())
	}
	m.SetDeleteGracePeriod(2 * time.Hour)
	if m.DeleteGracePeriod() != 2*time.Hour {
		t.Fatal("set grace")
	}
}
