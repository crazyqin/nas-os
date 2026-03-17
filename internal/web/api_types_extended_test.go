package web

import (
	"testing"
)

// ========== 网络管理 API 模型测试 ==========

func TestInterfaceConfig_Struct(t *testing.T) {
	config := InterfaceConfig{
		IP:      "192.168.1.100",
		Netmask: "255.255.255.0",
		Gateway: "192.168.1.1",
		DNS:     "8.8.8.8",
		DHCP:    false,
	}

	if config.IP != "192.168.1.100" {
		t.Errorf("Expected IP=192.168.1.100, got %s", config.IP)
	}
	if config.DHCP {
		t.Error("Expected DHCP=false")
	}
}

func TestToggleInterfaceRequest_Struct(t *testing.T) {
	req := ToggleInterfaceRequest{Up: true}
	if !req.Up {
		t.Error("Expected Up=true")
	}
}

func TestDDNSConfig_Struct(t *testing.T) {
	config := DDNSConfig{
		Domain:   "mynas.example.com",
		Provider: "cloudflare",
		Username: "user@example.com",
		Password: "api_token",
		Interval: 300,
		Enabled:  true,
	}

	if config.Domain != "mynas.example.com" {
		t.Errorf("Expected Domain=mynas.example.com, got %s", config.Domain)
	}
	if config.Provider != "cloudflare" {
		t.Errorf("Expected Provider=cloudflare, got %s", config.Provider)
	}
}

func TestPortForward_Struct(t *testing.T) {
	pf := PortForward{
		Name:         "web-server",
		Protocol:     "tcp",
		ExternalIP:   "0.0.0.0",
		ExternalPort: 8080,
		InternalIP:   "192.168.1.100",
		InternalPort: 80,
		Enabled:      true,
	}

	if pf.Name != "web-server" {
		t.Errorf("Expected Name=web-server, got %s", pf.Name)
	}
	if pf.ExternalPort != 8080 {
		t.Errorf("Expected ExternalPort=8080, got %d", pf.ExternalPort)
	}
}

func TestFirewallRule_Struct(t *testing.T) {
	rule := FirewallRule{
		Name:        "allow-ssh",
		Chain:       "INPUT",
		Protocol:    "tcp",
		Source:      "192.168.1.0/24",
		Destination: "0.0.0.0/0",
		Ports:       []string{"22"},
		Action:      "ACCEPT",
		Enabled:     true,
	}

	if rule.Name != "allow-ssh" {
		t.Errorf("Expected Name=allow-ssh, got %s", rule.Name)
	}
	if rule.Action != "ACCEPT" {
		t.Errorf("Expected Action=ACCEPT, got %s", rule.Action)
	}
}

func TestDefaultPolicyRequest_Struct(t *testing.T) {
	req := DefaultPolicyRequest{
		Chain:  "INPUT",
		Policy: "DROP",
	}

	if req.Chain != "INPUT" {
		t.Errorf("Expected Chain=INPUT, got %s", req.Chain)
	}
	if req.Policy != "DROP" {
		t.Errorf("Expected Policy=DROP, got %s", req.Policy)
	}
}

func TestFlushRulesRequest_Struct(t *testing.T) {
	req := FlushRulesRequest{Chain: "INPUT"}
	if req.Chain != "INPUT" {
		t.Errorf("Expected Chain=INPUT, got %s", req.Chain)
	}
}

// ========== 系统信息 API 模型测试 ==========

func TestSystemInfo_Struct(t *testing.T) {
	info := SystemInfo{
		Hostname: "nas-os",
		Version:  "1.0.0",
	}

	if info.Hostname != "nas-os" {
		t.Errorf("Expected Hostname=nas-os, got %s", info.Hostname)
	}
	if info.Version != "1.0.0" {
		t.Errorf("Expected Version=1.0.0, got %s", info.Version)
	}
}

func TestHealthResponse_Struct(t *testing.T) {
	resp := HealthResponse{
		Code:    0,
		Message: "healthy",
	}

	if resp.Code != 0 {
		t.Errorf("Expected Code=0, got %d", resp.Code)
	}
	if resp.Message != "healthy" {
		t.Errorf("Expected Message=healthy, got %s", resp.Message)
	}
}

// ========== 共享管理扩展测试 ==========

func TestSMBShareInput_Struct(t *testing.T) {
	share := SMBShareInput{
		Name:       "documents",
		Path:       "/mnt/data/documents",
		Comment:    "Document Share",
		Browseable: true,
		ReadOnly:   false,
		GuestOK:    false,
		Permissions: map[string]bool{
			"john": true,
		},
	}

	if share.Name != "documents" {
		t.Errorf("Expected Name=documents, got %s", share.Name)
	}
	if share.ReadOnly {
		t.Error("Expected ReadOnly=false")
	}
}

func TestSMBPermissionRequest_Struct(t *testing.T) {
	req := SMBPermissionRequest{
		Username:  "john",
		ReadWrite: true,
	}

	if req.Username != "john" {
		t.Errorf("Expected Username=john, got %s", req.Username)
	}
	if !req.ReadWrite {
		t.Error("Expected ReadWrite=true")
	}
}

func TestNFSExportInput_Struct(t *testing.T) {
	export := NFSExportInput{
		Name:    "media",
		Path:    "/mnt/data/media",
		Clients: []string{"192.168.1.0/24"},
		Options: []string{"rw", "sync"},
	}

	if export.Name != "media" {
		t.Errorf("Expected Name=media, got %s", export.Name)
	}
	if len(export.Clients) != 1 {
		t.Errorf("Expected 1 client, got %d", len(export.Clients))
	}
}
