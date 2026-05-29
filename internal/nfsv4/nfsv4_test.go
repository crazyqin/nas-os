package nfsv4

import (
	"testing"
)

func TestNewNFSv4Server(t *testing.T) {
	server := NewNFSv4Server(nil)
	if server == nil {
		t.Fatal("expected non-nil server")
	}

	config := server.GetConfig()
	if config.DefaultVersion != NFSVersion42 {
		t.Errorf("expected default version 4.2, got %v", config.DefaultVersion)
	}
}

func TestAddAndGetExport(t *testing.T) {
	server := NewNFSv4Server(nil)

	export := &NFSExport{
		ID:       "export1",
		Path:     "/data/share",
		Alias:    "share",
		Enabled:  true,
	}

	if err := server.AddExport(export); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, exists := server.GetExport("export1")
	if !exists {
		t.Fatal("expected export to exist")
	}
	if got.Path != "/data/share" {
		t.Errorf("expected path '/data/share', got %q", got.Path)
	}
}

func TestAddDuplicateExport(t *testing.T) {
	server := NewNFSv4Server(nil)

	export1 := &NFSExport{
		ID:   "export1",
		Path: "/data/share",
	}

	export2 := &NFSExport{
		ID:   "export2",
		Path: "/data/share", // 相同路径
	}

	server.AddExport(export1)

	if err := server.AddExport(export2); err == nil {
		t.Error("expected error for duplicate path")
	}
}

func TestDeleteExport(t *testing.T) {
	server := NewNFSv4Server(nil)

	export := &NFSExport{
		ID:   "export1",
		Path: "/data/share",
	}

	server.AddExport(export)

	if err := server.DeleteExport("export1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, exists := server.GetExport("export1")
	if exists {
		t.Error("expected export to be deleted")
	}
}

func TestEnableDisableExport(t *testing.T) {
	server := NewNFSv4Server(nil)

	export := &NFSExport{
		ID:      "export1",
		Path:    "/data/share",
		Enabled: false,
	}

	server.AddExport(export)

	// 启用
	if err := server.EnableExport("export1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := server.GetExport("export1")
	if !got.Enabled {
		t.Error("expected export to be enabled")
	}

	// 禁用
	if err := server.DisableExport("export1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ = server.GetExport("export1")
	if got.Enabled {
		t.Error("expected export to be disabled")
	}
}

func TestAllowedHosts(t *testing.T) {
	server := NewNFSv4Server(nil)

	export := &NFSExport{
		ID:   "export1",
		Path: "/data/share",
	}

	server.AddExport(export)

	// 添加允许的主机
	if err := server.AddAllowedHost("export1", "192.168.1.0/24"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := server.GetExport("export1")
	if len(got.AllowedHosts) != 1 {
		t.Errorf("expected 1 allowed host, got %d", len(got.AllowedHosts))
	}

	// 移除允许的主机
	if err := server.RemoveAllowedHost("export1", "192.168.1.0/24"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ = server.GetExport("export1")
	if len(got.AllowedHosts) != 0 {
		t.Errorf("expected 0 allowed hosts, got %d", len(got.AllowedHosts))
	}
}

func TestGetStats(t *testing.T) {
	server := NewNFSv4Server(nil)

	server.AddExport(&NFSExport{
		ID:      "export1",
		Path:    "/data/share1",
		Enabled: true,
		State:   ExportStateActive,
	})

	server.AddExport(&NFSExport{
		ID:      "export2",
		Path:    "/data/share2",
		Enabled: false,
		State:   ExportStateInactive,
	})

	stats := server.GetStats()
	totalExports := stats["total_exports"].(int)
	if totalExports != 2 {
		t.Errorf("expected 2 exports, got %d", totalExports)
	}

	activeExports := stats["active_exports"].(int)
	if activeExports != 1 {
		t.Errorf("expected 1 active export, got %d", activeExports)
	}
}

func TestNFSVersion42Features(t *testing.T) {
	server := NewNFSv4Server(nil)

	export := &NFSExport{
		ID:         "export1",
		Path:       "/data/share",
		NFSVersion: NFSVersion42,
	}

	server.AddExport(export)

	got, _ := server.GetExport("export1")
	if !got.Options.SupportsCopy {
		t.Error("expected NFSv4.2 to support copy")
	}
	if !got.Options.SupportsClone {
		t.Error("expected NFSv4.2 to support clone")
	}
	if !got.Options.SupportsSeek {
		t.Error("expected NFSv4.2 to support seek")
	}
}
