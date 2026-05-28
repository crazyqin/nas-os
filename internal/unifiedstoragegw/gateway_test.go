package unifiedstoragegw

import (
	"testing"
)

func TestNewGateway(t *testing.T) {
	gw := NewGateway()
	if gw == nil {
		t.Fatal("expected non-nil gateway")
	}

	eps := gw.ListEndpoints()
	if len(eps) < 4 {
		t.Errorf("expected at least 4 endpoints, got %d", len(eps))
	}
}

func TestStartStopProtocol(t *testing.T) {
	gw := NewGateway()

	err := gw.StartProtocol(ProtocolNFS, "0.0.0.0", 2049)
	if err != nil {
		t.Fatalf("StartProtocol failed: %v", err)
	}

	ep, err := gw.GetProtocolStatus(ProtocolNFS)
	if err != nil {
		t.Fatalf("GetProtocolStatus failed: %v", err)
	}
	if ep.Status != StatusRunning {
		t.Errorf("expected running, got %s", ep.Status)
	}

	err = gw.StopProtocol(ProtocolNFS)
	if err != nil {
		t.Fatalf("StopProtocol failed: %v", err)
	}

	ep, _ = gw.GetProtocolStatus(ProtocolNFS)
	if ep.Status != StatusStopped {
		t.Errorf("expected stopped, got %s", ep.Status)
	}
}

func TestCreateAndListShares(t *testing.T) {
	gw := NewGateway()

	share := &ShareDefinition{
		ID:        "share-1",
		Name:      "documents",
		Path:      "/data/documents",
		Protocols: []Protocol{ProtocolSMB, ProtocolNFS},
		ReadOnly:  false,
	}

	err := gw.CreateShare(share)
	if err != nil {
		t.Fatalf("CreateShare failed: %v", err)
	}

	shares := gw.ListShares()
	if len(shares) != 1 {
		t.Fatalf("expected 1 share, got %d", len(shares))
	}
	if shares[0].Name != "documents" {
		t.Errorf("expected 'documents', got '%s'", shares[0].Name)
	}
}

func TestDuplicateShare(t *testing.T) {
	gw := NewGateway()

	share := &ShareDefinition{ID: "dup", Name: "test", Path: "/tmp"}
	_ = gw.CreateShare(share)

	err := gw.CreateShare(share)
	if err == nil {
		t.Error("expected error for duplicate share")
	}
}

func TestDeleteShare(t *testing.T) {
	gw := NewGateway()

	_ = gw.CreateShare(&ShareDefinition{ID: "del-1", Name: "test", Path: "/tmp"})

	err := gw.DeleteShare("del-1")
	if err != nil {
		t.Fatalf("DeleteShare failed: %v", err)
	}

	shares := gw.ListShares()
	if len(shares) != 0 {
		t.Errorf("expected 0 shares, got %d", len(shares))
	}
}

func TestConnectionManagement(t *testing.T) {
	gw := NewGateway()
	_ = gw.StartProtocol(ProtocolSMB, "0.0.0.0", 445)

	conn := &ClientConnection{
		Protocol:   ProtocolSMB,
		ClientAddr: "192.168.1.100",
		ShareName:  "documents",
		UserName:   "admin",
	}

	gw.RegisterConnection(conn)

	conns := gw.GetActiveConnections()
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}

	ep, _ := gw.GetProtocolStatus(ProtocolSMB)
	if ep.Connections != 1 {
		t.Errorf("expected 1 connection on endpoint, got %d", ep.Connections)
	}

	gw.UnregisterConnection(conn.ID)

	conns = gw.GetActiveConnections()
	if len(conns) != 0 {
		t.Errorf("expected 0 connections, got %d", len(conns))
	}
}

func TestNegotiateProtocol(t *testing.T) {
	gw := NewGateway()
	_ = gw.StartProtocol(ProtocolNFS, "0.0.0.0", 2049)
	_ = gw.StartProtocol(ProtocolSMB, "0.0.0.0", 445)

	// 客户端支持 NFS 和 SMB
	protocol := gw.NegotiateProtocol("192.168.1.1", []Protocol{ProtocolSMB, ProtocolNFS})
	if protocol != ProtocolNFS {
		t.Errorf("expected NFS (higher priority), got %s", protocol)
	}
}

func TestGetBestProtocolForUseCase(t *testing.T) {
	tests := []struct {
		useCase  string
		expected Protocol
	}{
		{"vm_storage", ProtocolISCSI},
		{"file_sharing", ProtocolSMB},
		{"linux_nfs", ProtocolNFS},
		{"object_storage", ProtocolS3},
		{"unknown", ProtocolSMB},
	}

	for _, tt := range tests {
		result := GetBestProtocolForUseCase(tt.useCase)
		if result != tt.expected {
			t.Errorf("use case '%s': expected %s, got %s", tt.useCase, tt.expected, result)
		}
	}
}

func TestIsProtocolSupported(t *testing.T) {
	if !IsProtocolSupported(ProtocolNFS) {
		t.Error("NFS should be supported")
	}
	if IsProtocolSupported("ftp") {
		t.Error("FTP should not be supported")
	}
}

func TestGetGatewayStatus(t *testing.T) {
	gw := NewGateway()
	_ = gw.StartProtocol(ProtocolSMB, "0.0.0.0", 445)
	_ = gw.CreateShare(&ShareDefinition{ID: "s1", Name: "test", Path: "/tmp"})

	status := gw.GetGatewayStatus()
	if status.Shares != 1 {
		t.Errorf("expected 1 share, got %d", status.Shares)
	}
	if len(status.Protocols) != 1 {
		t.Errorf("expected 1 active protocol, got %d", len(status.Protocols))
	}
}

func TestUpdateShareACL(t *testing.T) {
	gw := NewGateway()
	_ = gw.CreateShare(&ShareDefinition{ID: "acl-test", Name: "test", Path: "/tmp"})

	acl := []ACLEntry{
		{Principal: "user:admin", Permission: "admin"},
		{Principal: "group:users", Permission: "read"},
	}

	err := gw.UpdateShareACL("acl-test", acl)
	if err != nil {
		t.Fatalf("UpdateShareACL failed: %v", err)
	}

	share, _ := gw.GetShare("acl-test")
	if len(share.ACL) != 2 {
		t.Errorf("expected 2 ACL entries, got %d", len(share.ACL))
	}
}

func TestProtocolStats(t *testing.T) {
	gw := NewGateway()
	_ = gw.StartProtocol(ProtocolNFS, "0.0.0.0", 2049)

	gw.RegisterConnection(&ClientConnection{
		Protocol:   ProtocolNFS,
		ClientAddr: "10.0.0.1",
		ShareName:  "data",
		BytesRead:  1024,
		BytesWrite: 512,
	})

	stats := gw.GetProtocolStats(ProtocolNFS)
	if stats == nil {
		t.Fatal("expected non-nil stats")
	}
	if stats.ActiveConns != 1 {
		t.Errorf("expected 1 active conn, got %d", stats.ActiveConns)
	}
	if stats.BytesRead != 1024 {
		t.Errorf("expected 1024 bytes read, got %d", stats.BytesRead)
	}
}
