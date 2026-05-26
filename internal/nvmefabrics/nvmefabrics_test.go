package nvmefabrics

import (
	"net"
	"testing"
)

func TestFabricManager_CreateTarget(t *testing.T) {
	fm := NewFabricManager()

	target := &NVMeTarget{
		ID:           "tgt-001",
		Name:         "NVMe-TCP-Primary",
		Transport:    TransportTCP,
		IP:           net.ParseIP("192.168.1.100"),
		Port:         4420,
		SubsystemNQN: "nqn.2026-05.com.nasos:primary",
		MaxNamespaces: 16,
	}

	if err := fm.CreateTarget(target); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := fm.GetTarget("tgt-001")
	if !ok {
		t.Fatal("expected target to exist")
	}
	if got.State != TargetStateActive {
		t.Errorf("expected active state, got %q", got.State)
	}
}

func TestFabricManager_CreateDuplicateTarget(t *testing.T) {
	fm := NewFabricManager()

	target := &NVMeTarget{ID: "tgt-001", Name: "Test"}
	fm.CreateTarget(target)

	err := fm.CreateTarget(&NVMeTarget{ID: "tgt-001", Name: "Dup"})
	if err == nil {
		t.Error("expected error for duplicate target")
	}
}

func TestFabricManager_ConnectDisconnect(t *testing.T) {
	fm := NewFabricManager()

	fm.CreateTarget(&NVMeTarget{
		ID:           "tgt-001",
		Transport:    TransportTCP,
		IP:           net.ParseIP("10.0.0.1"),
		MaxNamespaces: 8,
	})

	if err := fm.ConnectHost("tgt-001", "192.168.1.50"); err != nil {
		t.Fatalf("connect error: %v", err)
	}

	conns := fm.GetConnections("tgt-001")
	if len(conns) != 1 {
		t.Errorf("expected 1 connection, got %d", len(conns))
	}

	target, _ := fm.GetTarget("tgt-001")
	if len(target.ConnectedHosts) != 1 {
		t.Errorf("expected 1 connected host, got %d", len(target.ConnectedHosts))
	}

	fm.DisconnectHost("tgt-001", "192.168.1.50")
	conns = fm.GetConnections("tgt-001")
	if len(conns) != 0 {
		t.Errorf("expected 0 connections after disconnect, got %d", len(conns))
	}
}

func TestFabricManager_AddNamespace(t *testing.T) {
	fm := NewFabricManager()

	fm.CreateTarget(&NVMeTarget{
		ID:            "tgt-001",
		MaxNamespaces: 2,
	})

	ns1 := Namespace{ID: 1, DevicePath: "/dev/nvme0n1", SizeBytes: 500 * 1024 * 1024 * 1024, BlockSize: 512, UUID: "uuid-001"}
	if err := fm.AddNamespace("tgt-001", ns1); err != nil {
		t.Fatalf("add namespace error: %v", err)
	}

	ns2 := Namespace{ID: 2, DevicePath: "/dev/nvme0n2", SizeBytes: 1024 * 1024 * 1024 * 1024, BlockSize: 4096, UUID: "uuid-002"}
	if err := fm.AddNamespace("tgt-001", ns2); err != nil {
		t.Fatalf("add namespace 2 error: %v", err)
	}

	ns3 := Namespace{ID: 3}
	err := fm.AddNamespace("tgt-001", ns3)
	if err == nil {
		t.Error("expected error when exceeding max namespaces")
	}
}

func TestFabricManager_ListTargets(t *testing.T) {
	fm := NewFabricManager()

	fm.CreateTarget(&NVMeTarget{ID: "t1", Transport: TransportTCP})
	fm.CreateTarget(&NVMeTarget{ID: "t2", Transport: TransportRDMA})
	fm.CreateTarget(&NVMeTarget{ID: "t3", Transport: TransportTCP})

	tcpTargets := fm.ListTargets(TransportTCP)
	if len(tcpTargets) != 2 {
		t.Errorf("expected 2 TCP targets, got %d", len(tcpTargets))
	}

	allTargets := fm.ListTargets("")
	if len(allTargets) != 3 {
		t.Errorf("expected 3 total targets, got %d", len(allTargets))
	}
}

func TestFabricManager_Stats(t *testing.T) {
	fm := NewFabricManager()

	fm.CreateTarget(&NVMeTarget{ID: "t1", Transport: TransportTCP, MaxNamespaces: 4})
	fm.CreateTarget(&NVMeTarget{ID: "t2", Transport: TransportRDMA, MaxNamespaces: 4})
	fm.ConnectHost("t1", "10.0.0.1")

	stats := fm.GetStats()
	if stats.TotalTargets != 2 {
		t.Errorf("expected 2 targets, got %d", stats.TotalTargets)
	}
	if stats.TCPCount != 1 {
		t.Errorf("expected 1 TCP, got %d", stats.TCPCount)
	}
	if stats.RDMACount != 1 {
		t.Errorf("expected 1 RDMA, got %d", stats.RDMACount)
	}
	if stats.TotalConnections != 1 {
		t.Errorf("expected 1 connection, got %d", stats.TotalConnections)
	}
}
