package web

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestResponse_Fields(t *testing.T) {
	resp := Response{
		Code:    0,
		Message: "success",
		Data:    map[string]string{"key": "value"},
	}

	if resp.Code != 0 {
		t.Error("Code should be 0")
	}
	if resp.Message != "success" {
		t.Error("Message should be success")
	}
	if resp.Data == nil {
		t.Error("Data should not be nil")
	}
}

func TestErrorResponse_Fields(t *testing.T) {
	err := ErrorResponse{
		Code:    400,
		Message: "Bad Request",
	}

	if err.Code != 400 {
		t.Error("Code should be 400")
	}
	if err.Message != "Bad Request" {
		t.Error("Message mismatch")
	}
}

func TestVolumeCreateRequest_Validation(t *testing.T) {
	req := VolumeCreateRequest{
		Name:    "test-vol",
		Devices: []string{"/dev/sda", "/dev/sdb"},
		Profile: "raid1",
	}

	if req.Name != "test-vol" {
		t.Error("Name mismatch")
	}
	if len(req.Devices) != 2 {
		t.Error("Should have 2 devices")
	}
}

func TestVolume_Fields(t *testing.T) {
	vol := Volume{
		Name:       "data",
		UUID:       "uuid-123",
		Mounted:    true,
		MountPoint: "/mnt/data",
		TotalSize:  107374182400,
		UsedSize:   21474836480,
		FreeSize:   85899345920,
		Profile:    "raid1",
		Devices:    []string{"/dev/sda", "/dev/sdb"},
	}

	if vol.Name != "data" {
		t.Error("Name mismatch")
	}
	if !vol.Mounted {
		t.Error("Should be mounted")
	}
	if vol.TotalSize <= vol.UsedSize {
		t.Error("TotalSize should be greater than UsedSize")
	}
}

func TestUserInput_Validation(t *testing.T) {
	user := UserInput{
		Username: "john",
		Password: "secret123",
		Shell:    "/bin/bash",
		HomeDir:  "/home/john",
		Role:     "user",
	}

	if user.Username != "john" {
		t.Error("Username mismatch")
	}
	if user.Password != "secret123" {
		t.Error("Password mismatch")
	}
}

func TestUser_Fields(t *testing.T) {
	user := User{
		Username: "john",
		UID:      1000,
		GID:      1000,
		HomeDir:  "/home/john",
		Shell:    "/bin/bash",
		Disabled: false,
		Role:     "user",
		Groups:   []string{"sudo", "docker"},
	}

	if user.UID != 1000 {
		t.Error("UID should be 1000")
	}
	if len(user.Groups) != 2 {
		t.Error("Should have 2 groups")
	}
}

func TestLoginRequest_Fields(t *testing.T) {
	req := LoginRequest{
		Username: "admin",
		Password: "password",
	}

	if req.Username != "admin" {
		t.Error("Username mismatch")
	}
	if req.Password != "password" {
		t.Error("Password mismatch")
	}
}

func TestLoginResponse_Fields(t *testing.T) {
	resp := LoginResponse{
		Token:     "token123",
		ExpiresAt: "2024-01-02T15:04:05Z",
		User: &User{
			Username: "admin",
		},
	}

	if resp.Token != "token123" {
		t.Error("Token mismatch")
	}
	if resp.User == nil {
		t.Error("User should not be nil")
	}
}

func TestSMBShareInput_Fields(t *testing.T) {
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
		t.Error("Name mismatch")
	}
	if share.ReadOnly {
		t.Error("Should not be read only")
	}
}

func TestNFSExportInput_Fields(t *testing.T) {
	export := NFSExportInput{
		Name:    "media",
		Path:    "/mnt/data/media",
		Clients: []string{"192.168.1.0/24"},
		Options: []string{"rw", "sync"},
	}

	if export.Name != "media" {
		t.Error("Name mismatch")
	}
	if len(export.Clients) != 1 {
		t.Error("Should have 1 client")
	}
}

func TestInterfaceConfig_Fields(t *testing.T) {
	config := InterfaceConfig{
		IP:      "192.168.1.100",
		Netmask: "255.255.255.0",
		Gateway: "192.168.1.1",
		DNS:     "8.8.8.8",
		DHCP:    false,
	}

	if config.IP != "192.168.1.100" {
		t.Error("IP mismatch")
	}
	if config.DHCP {
		t.Error("DHCP should be false")
	}
}

func TestFirewallRule_Fields(t *testing.T) {
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
		t.Error("Name mismatch")
	}
	if rule.Action != "ACCEPT" {
		t.Error("Action should be ACCEPT")
	}
}

func TestHealthResponse_Fields(t *testing.T) {
	resp := HealthResponse{
		Code:    0,
		Message: "healthy",
	}

	if resp.Code != 0 {
		t.Error("Code should be 0")
	}
	if resp.Message != "healthy" {
		t.Error("Message should be healthy")
	}
}

func TestSystemInfo_Fields(t *testing.T) {
	info := SystemInfo{
		Hostname: "nas-os",
		Version:  "1.0.0",
	}

	if info.Hostname != "nas-os" {
		t.Error("Hostname mismatch")
	}
	if info.Version != "1.0.0" {
		t.Error("Version mismatch")
	}
}

func TestSnapshotCreateRequest_Fields(t *testing.T) {
	req := SnapshotCreateRequest{
		SubVolumeName: "documents",
		Name:          "backup-2024",
		ReadOnly:      true,
	}

	if req.SubVolumeName != "documents" {
		t.Error("SubVolumeName mismatch")
	}
	if !req.ReadOnly {
		t.Error("Should be read only")
	}
}

func TestDDNSConfig_Fields(t *testing.T) {
	config := DDNSConfig{
		Domain:   "mynas.example.com",
		Provider: "cloudflare",
		Username: "user@example.com",
		Password: "api_token",
		Interval: 300,
		Enabled:  true,
	}

	if config.Domain != "mynas.example.com" {
		t.Error("Domain mismatch")
	}
	if config.Provider != "cloudflare" {
		t.Error("Provider mismatch")
	}
}

func TestPortForward_Fields(t *testing.T) {
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
		t.Error("Name mismatch")
	}
	if pf.ExternalPort != 8080 {
		t.Error("ExternalPort should be 8080")
	}
}

func TestShareOverview_Fields(t *testing.T) {
	overview := ShareOverview{
		Type:   "smb",
		Name:   "documents",
		Path:   "/mnt/data/documents",
		Config: map[string]string{"comment": "docs"},
	}

	if overview.Type != "smb" {
		t.Error("Type mismatch")
	}
}

// Test helper functions with mock context
func setupTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c
}

func TestRespondSuccess(t *testing.T) {
	c := setupTestContext()
	respondSuccess(c, map[string]string{"key": "value"})
	// Just verify it doesn't panic
}

func TestRespondBadRequest(t *testing.T) {
	c := setupTestContext()
	respondBadRequest(c, "Invalid request")
	// Just verify it doesn't panic
}

func TestRespondNotFound(t *testing.T) {
	c := setupTestContext()
	respondNotFound(c, "Resource not found")
	// Just verify it doesn't panic
}

func TestRespondInternalError(t *testing.T) {
	c := setupTestContext()
	respondInternalError(c, "Internal error")
	// Just verify it doesn't panic
}