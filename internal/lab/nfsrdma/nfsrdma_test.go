package nfsrdma

import (
	"testing"
)

func TestRDMAInterface(t *testing.T) {
	m := NewManager("/tmp/test-rdma")
	iface := &RDMAInterface{
		Name:      "ib0",
		Device:    "mlx5_0",
		Transport: TransportRoCEv2,
		Status:    RDMAStatusUp,
		SpeedGbps: 100,
		Port:      1,
	}
	if err := m.RegisterInterface(iface); err != nil {
		t.Fatalf("RegisterInterface failed: %v", err)
	}
	if iface.MTU != 9000 {
		t.Errorf("expected MTU 9000, got %d", iface.MTU)
	}
	ifaces := m.ListInterfaces()
	if len(ifaces) != 1 {
		t.Errorf("expected 1 interface, got %d", len(ifaces))
	}
}

func TestExport(t *testing.T) {
	m := NewManager("/tmp/test-rdma")
	export := &NFSRDMAExport{
		Path:       "/mnt/pool1/share",
		ExportPath: "/export/mnt/pool1/share",
		AllowedHosts: []string{"192.168.1.0/24"},
	}
	if err := m.AddExport(export); err != nil {
		t.Fatalf("AddExport failed: %v", err)
	}
	if export.ID == "" {
		t.Error("expected export ID to be set")
	}
	exports := m.ListExports()
	if len(exports) != 1 {
		t.Errorf("expected 1 export, got %d", len(exports))
	}
	if err := m.RemoveExport(export.ID); err != nil {
		t.Fatalf("RemoveExport failed: %v", err)
	}
}

func TestHealthCheck(t *testing.T) {
	m := NewManager("/tmp/test-rdma")
	m.RegisterInterface(&RDMAInterface{
		Name: "ib0", Device: "mlx5_0", Transport: TransportIB,
		Status: RDMAStatusUp, SpeedGbps: 100,
	})
	m.RegisterInterface(&RDMAInterface{
		Name: "ib1", Device: "mlx5_1", Transport: TransportRoCEv2,
		Status: RDMAStatusUp, SpeedGbps: 25, ErrorCount: 200,
	})
	health := m.HealthCheck()
	if health["ib1"] != RDMAStatusDegraded {
		t.Errorf("expected ib1 degraded, got %s", health["ib1"])
	}
	if health["ib0"] != RDMAStatusUp {
		t.Errorf("expected ib0 up, got %s", health["ib0"])
	}
}

func TestRecommendExportConfig(t *testing.T) {
	m := NewManager("/tmp/test-rdma")
	rec := m.RecommendExportConfig("/mnt/data")
	if rec == nil {
		t.Fatal("expected recommendation")
	}
	if rec.Squash != "root_squash" {
		t.Errorf("expected root_squash, got %s", rec.Squash)
	}
	if !rec.TransportBoth {
		t.Error("expected TransportBoth true")
	}
}