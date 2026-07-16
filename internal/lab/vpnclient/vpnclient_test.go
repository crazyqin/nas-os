package vpnclient

import (
	"testing"
	"time"
)

func TestCreateProfileRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     CreateProfileRequest
		wantErr bool
	}{
		{
			name: "valid openvpn profile",
			req: CreateProfileRequest{
				Name:       "test-openvpn",
				Protocol:   ProtocolOpenVPN,
				ServerAddr: "vpn.example.com",
				ServerPort: 1194,
			},
			wantErr: false,
		},
		{
			name: "valid wireguard profile",
			req: CreateProfileRequest{
				Name:       "test-wg",
				Protocol:   ProtocolWireGuard,
				ServerAddr: "wg.example.com",
				ServerPort: 51820,
			},
			wantErr: false,
		},
		{
			name: "valid l2tp profile",
			req: CreateProfileRequest{
				Name:       "test-l2tp",
				Protocol:   ProtocolL2TP,
				ServerAddr: "l2tp.example.com",
				ServerPort: 1701,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			req: CreateProfileRequest{
				Protocol:   ProtocolOpenVPN,
				ServerAddr: "vpn.example.com",
				ServerPort: 1194,
			},
			wantErr: true,
		},
		{
			name: "missing protocol",
			req: CreateProfileRequest{
				Name:       "test",
				ServerAddr: "vpn.example.com",
				ServerPort: 1194,
			},
			wantErr: true,
		},
		{
			name: "missing server address",
			req: CreateProfileRequest{
				Name:       "test",
				Protocol:   ProtocolOpenVPN,
				ServerPort: 1194,
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			req: CreateProfileRequest{
				Name:       "test",
				Protocol:   ProtocolOpenVPN,
				ServerAddr: "vpn.example.com",
				ServerPort: 0,
			},
			wantErr: true,
		},
		{
			name: "unsupported protocol",
			req: CreateProfileRequest{
				Name:       "test",
				Protocol:   "ikev2",
				ServerAddr: "vpn.example.com",
				ServerPort: 500,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.req.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_CreateProfile(t *testing.T) {
	m := NewManager()

	req := CreateProfileRequest{
		Name:       "test-profile",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn.example.com",
		ServerPort: 1194,
	}

	profile, err := m.CreateProfile(req)
	if err != nil {
		t.Fatalf("CreateProfile() error = %v", err)
	}

	if profile.Name != req.Name {
		t.Errorf("Name = %v, want %v", profile.Name, req.Name)
	}
	if profile.Protocol != req.Protocol {
		t.Errorf("Protocol = %v, want %v", profile.Protocol, req.Protocol)
	}
	if profile.ServerAddr != req.ServerAddr {
		t.Errorf("ServerAddr = %v, want %v", profile.ServerAddr, req.ServerAddr)
	}
	if !profile.Enabled {
		t.Error("Expected profile to be enabled")
	}
}

func TestManager_CreateDuplicateProfile(t *testing.T) {
	m := NewManager()

	req := CreateProfileRequest{
		Name:       "test-profile",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn.example.com",
		ServerPort: 1194,
	}

	_, err := m.CreateProfile(req)
	if err != nil {
		t.Fatalf("CreateProfile() error = %v", err)
	}

	_, err = m.CreateProfile(req)
	if err == nil {
		t.Error("Expected error for duplicate profile")
	}
}

func TestManager_GetProfile(t *testing.T) {
	m := NewManager()

	req := CreateProfileRequest{
		Name:       "test-profile",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn.example.com",
		ServerPort: 1194,
	}

	created, _ := m.CreateProfile(req)

	fetched, err := m.GetProfile(created.ID)
	if err != nil {
		t.Fatalf("GetProfile() error = %v", err)
	}

	if fetched.ID != created.ID {
		t.Errorf("ID = %v, want %v", fetched.ID, created.ID)
	}
}

func TestManager_GetProfileNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetProfile("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent profile")
	}
}

func TestManager_ListProfiles(t *testing.T) {
	m := NewManager()

	req1 := CreateProfileRequest{
		Name:       "profile-1",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn1.example.com",
		ServerPort: 1194,
	}
	req2 := CreateProfileRequest{
		Name:       "profile-2",
		Protocol:   ProtocolWireGuard,
		ServerAddr: "vpn2.example.com",
		ServerPort: 51820,
	}

	m.CreateProfile(req1)
	m.CreateProfile(req2)

	profiles := m.ListProfiles()
	if len(profiles) != 2 {
		t.Errorf("ListProfiles() count = %v, want 2", len(profiles))
	}
}

func TestManager_UpdateProfile(t *testing.T) {
	m := NewManager()

	req := CreateProfileRequest{
		Name:       "test-profile",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn.example.com",
		ServerPort: 1194,
	}

	created, _ := m.CreateProfile(req)

	newName := "updated-profile"
	updateReq := UpdateProfileRequest{
		Name: &newName,
	}

	updated, err := m.UpdateProfile(created.ID, updateReq)
	if err != nil {
		t.Fatalf("UpdateProfile() error = %v", err)
	}

	if updated.Name != newName {
		t.Errorf("Name = %v, want %v", updated.Name, newName)
	}
}

func TestManager_DeleteProfile(t *testing.T) {
	m := NewManager()

	req := CreateProfileRequest{
		Name:       "test-profile",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn.example.com",
		ServerPort: 1194,
	}

	created, _ := m.CreateProfile(req)

	err := m.DeleteProfile(created.ID)
	if err != nil {
		t.Fatalf("DeleteProfile() error = %v", err)
	}

	_, err = m.GetProfile(created.ID)
	if err == nil {
		t.Error("Expected error for deleted profile")
	}
}

func TestManager_SetDefaultProfile(t *testing.T) {
	m := NewManager()

	req := CreateProfileRequest{
		Name:       "test-profile",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn.example.com",
		ServerPort: 1194,
	}

	created, _ := m.CreateProfile(req)

	err := m.SetDefaultProfile(created.ID)
	if err != nil {
		t.Fatalf("SetDefaultProfile() error = %v", err)
	}

	defaultProfile, err := m.GetDefaultProfile()
	if err != nil {
		t.Fatalf("GetDefaultProfile() error = %v", err)
	}

	if defaultProfile.ID != created.ID {
		t.Errorf("Default profile ID = %v, want %v", defaultProfile.ID, created.ID)
	}
}

func TestManager_Connect(t *testing.T) {
	m := NewManager()

	req := CreateProfileRequest{
		Name:       "test-profile",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn.example.com",
		ServerPort: 1194,
	}

	profile, _ := m.CreateProfile(req)

	// Wait for connection to establish
	time.Sleep(600 * time.Millisecond)

	connReq := ConnectRequest{
		ProfileID: profile.ID,
	}

	conn, err := m.Connect(connReq)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	if conn.ProfileID != profile.ID {
		t.Errorf("ProfileID = %v, want %v", conn.ProfileID, profile.ID)
	}
	if conn.Protocol != ProtocolOpenVPN {
		t.Errorf("Protocol = %v, want %v", conn.Protocol, ProtocolOpenVPN)
	}

	// Wait for connection to establish
	time.Sleep(600 * time.Millisecond)

	// Verify connection status
	fetched, err := m.GetConnection(conn.ID)
	if err != nil {
		t.Fatalf("GetConnection() error = %v", err)
	}
	if fetched.Status != StatusConnected {
		t.Errorf("Status = %v, want %v", fetched.Status, StatusConnected)
	}
}

func TestManager_Disconnect(t *testing.T) {
	m := NewManager()

	req := CreateProfileRequest{
		Name:       "test-profile",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn.example.com",
		ServerPort: 1194,
	}

	profile, _ := m.CreateProfile(req)

	// Wait for connection to establish
	time.Sleep(600 * time.Millisecond)

	connReq := ConnectRequest{
		ProfileID: profile.ID,
	}

	conn, _ := m.Connect(connReq)
	time.Sleep(600 * time.Millisecond)

	err := m.Disconnect(DisconnectRequest{ConnectionID: conn.ID})
	if err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}

	_, err = m.GetConnection(conn.ID)
	if err == nil {
		t.Error("Expected error for disconnected connection")
	}
}

func TestManager_ListConnections(t *testing.T) {
	m := NewManager()

	req := CreateProfileRequest{
		Name:       "test-profile",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn.example.com",
		ServerPort: 1194,
	}

	profile, _ := m.CreateProfile(req)
	time.Sleep(600 * time.Millisecond)

	m.Connect(ConnectRequest{ProfileID: profile.ID})
	time.Sleep(600 * time.Millisecond)

	conns := m.ListConnections()
	if len(conns) != 1 {
		t.Errorf("ListConnections() count = %v, want 1", len(conns))
	}
}

func TestManager_GetStatus(t *testing.T) {
	m := NewManager()

	status := m.GetStatus()
	if status.TotalProfiles != 0 {
		t.Errorf("TotalProfiles = %v, want 0", status.TotalProfiles)
	}
	if status.ActiveConnections != 0 {
		t.Errorf("ActiveConnections = %v, want 0", status.ActiveConnections)
	}
	if !status.AutoReconnect {
		t.Error("Expected AutoReconnect to be true by default")
	}
}

func TestManager_SetAutoReconnect(t *testing.T) {
	m := NewManager()

	if !m.IsAutoReconnectEnabled() {
		t.Error("Expected AutoReconnect to be true by default")
	}

	m.SetAutoReconnect(false)
	if m.IsAutoReconnectEnabled() {
		t.Error("Expected AutoReconnect to be false")
	}
}

func TestManager_SetFailoverEnabled(t *testing.T) {
	m := NewManager()

	if m.IsFailoverEnabled() {
		t.Error("Expected Failover to be false by default")
	}

	m.SetFailoverEnabled(true)
	if !m.IsFailoverEnabled() {
		t.Error("Expected Failover to be true")
	}
}

func TestOpenVPNClient_ImportConfig(t *testing.T) {
	c := NewOpenVPNClient()

	configContent := `client
dev tun
proto udp
remote vpn.example.com 1194
resolv-retry infinite
nobind
persist-key
persist-tun
cipher AES-256-GCM
auth SHA256
verb 3`

	config, err := c.ImportConfig("profile1", configContent)
	if err != nil {
		t.Fatalf("ImportConfig() error = %v", err)
	}

	if config.RemoteAddr != "vpn.example.com" {
		t.Errorf("RemoteAddr = %v, want vpn.example.com", config.RemoteAddr)
	}
	if config.RemotePort != 1194 {
		t.Errorf("RemotePort = %v, want 1194", config.RemotePort)
	}
	if config.Protocol != "udp" {
		t.Errorf("Protocol = %v, want udp", config.Protocol)
	}
	if config.Cipher != "AES-256-GCM" {
		t.Errorf("Cipher = %v, want AES-256-GCM", config.Cipher)
	}
}

func TestOpenVPNClient_ExportConfig(t *testing.T) {
	c := NewOpenVPNClient()

	configContent := `client
dev tun
proto udp
remote vpn.example.com 1194
cipher AES-256-GCM`

	c.ImportConfig("profile1", configContent)

	exported, err := c.ExportConfig("profile1")
	if err != nil {
		t.Fatalf("ExportConfig() error = %v", err)
	}

	if exported == "" {
		t.Error("Exported config is empty")
	}
}

func TestOpenVPNClient_ConnectDisconnect(t *testing.T) {
	c := NewOpenVPNClient()

	profile := &VPNProfile{
		ID:         "profile1",
		Name:       "test",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn.example.com",
		ServerPort: 1194,
		Enabled:    true,
	}

	conn, err := c.Connect(profile)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	time.Sleep(600 * time.Millisecond)

	fetched, err := c.GetConnection(conn.ID)
	if err != nil {
		t.Fatalf("GetConnection() error = %v", err)
	}
	if fetched.Status != StatusConnected {
		t.Errorf("Status = %v, want %v", fetched.Status, StatusConnected)
	}

	err = c.Disconnect(conn.ID)
	if err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
}

func TestWireGuardClient_GenerateKeyPair(t *testing.T) {
	c := NewWireGuardClient()

	kp, err := c.GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	if kp.PublicKey == "" {
		t.Error("PublicKey is empty")
	}
	if kp.PrivateKey == "" {
		t.Error("PrivateKey is empty")
	}
	if kp.PublicKey == kp.PrivateKey {
		t.Error("PublicKey and PrivateKey should be different")
	}
}

func TestWireGuardClient_GeneratePresharedKey(t *testing.T) {
	c := NewWireGuardClient()

	psk, err := c.GeneratePresharedKey()
	if err != nil {
		t.Fatalf("GeneratePresharedKey() error = %v", err)
	}

	if psk == "" {
		t.Error("PresharedKey is empty")
	}
}

func TestWireGuardClient_ConnectDisconnect(t *testing.T) {
	c := NewWireGuardClient()

	profile := &VPNProfile{
		ID:         "profile1",
		Name:       "test",
		Protocol:   ProtocolWireGuard,
		ServerAddr: "wg.example.com",
		ServerPort: 51820,
		Enabled:    true,
	}

	conn, err := c.Connect(profile)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	fetched, err := c.GetConnection(conn.ID)
	if err != nil {
		t.Fatalf("GetConnection() error = %v", err)
	}
	if fetched.Status != StatusConnected {
		t.Errorf("Status = %v, want %v", fetched.Status, StatusConnected)
	}

	err = c.Disconnect(conn.ID)
	if err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
}

func TestL2TPClient_ConnectDisconnect(t *testing.T) {
	c := NewL2TPClient()

	profile := &VPNProfile{
		ID:         "profile1",
		Name:       "test",
		Protocol:   ProtocolL2TP,
		ServerAddr: "l2tp.example.com",
		ServerPort: 1701,
		Username:   "user",
		Password:   "pass",
		Enabled:    true,
	}

	config := &L2TPConfig{
		ServerAddr:  "l2tp.example.com",
		ServerPort:  1701,
		Username:    "user",
		Password:    "pass",
		PSK:         "test-psk",
		PPPAuthType: "mschap-v2",
		MTU:         1400,
		MRU:         1400,
		IPSecProto:  "ikev2",
	}

	err := c.ImportConfig("profile1", config)
	if err != nil {
		t.Fatalf("ImportConfig() error = %v", err)
	}

	conn, err := c.Connect(profile)
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	time.Sleep(1100 * time.Millisecond)

	fetched, err := c.GetConnection(conn.ID)
	if err != nil {
		t.Fatalf("GetConnection() error = %v", err)
	}
	if fetched.Status != StatusConnected {
		t.Errorf("Status = %v, want %v", fetched.Status, StatusConnected)
	}

	err = c.Disconnect(conn.ID)
	if err != nil {
		t.Fatalf("Disconnect() error = %v", err)
	}
}

func TestL2TPClient_SetPSK(t *testing.T) {
	c := NewL2TPClient()

	err := c.SetPSK("profile1", "my-secret-psk")
	if err != nil {
		t.Fatalf("SetPSK() error = %v", err)
	}

	config, err := c.GetConfig("profile1")
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if config.PSK != "my-secret-psk" {
		t.Errorf("PSK = %v, want my-secret-psk", config.PSK)
	}
}

func TestL2TPClient_ValidateConfig(t *testing.T) {
	c := NewL2TPClient()

	tests := []struct {
		name    string
		config  *L2TPConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &L2TPConfig{
				ServerAddr:  "l2tp.example.com",
				Username:    "user",
				Password:    "pass",
				PSK:         "psk",
				PPPAuthType: "mschap-v2",
				IPSecProto:  "ikev2",
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			config:  nil,
			wantErr: true,
		},
		{
			name: "missing server",
			config: &L2TPConfig{
				Username:    "user",
				Password:    "pass",
				PSK:         "psk",
				PPPAuthType: "mschap-v2",
				IPSecProto:  "ikev2",
			},
			wantErr: true,
		},
		{
			name: "missing username",
			config: &L2TPConfig{
				ServerAddr:  "l2tp.example.com",
				Password:    "pass",
				PSK:         "psk",
				PPPAuthType: "mschap-v2",
				IPSecProto:  "ikev2",
			},
			wantErr: true,
		},
		{
			name: "missing password",
			config: &L2TPConfig{
				ServerAddr:  "l2tp.example.com",
				Username:    "user",
				PSK:         "psk",
				PPPAuthType: "mschap-v2",
				IPSecProto:  "ikev2",
			},
			wantErr: true,
		},
		{
			name: "missing psk",
			config: &L2TPConfig{
				ServerAddr:  "l2tp.example.com",
				Username:    "user",
				Password:    "pass",
				PPPAuthType: "mschap-v2",
				IPSecProto:  "ikev2",
			},
			wantErr: true,
		},
		{
			name: "invalid auth type",
			config: &L2TPConfig{
				ServerAddr:  "l2tp.example.com",
				Username:    "user",
				Password:    "pass",
				PSK:         "psk",
				PPPAuthType: "invalid",
				IPSecProto:  "ikev2",
			},
			wantErr: true,
		},
		{
			name: "invalid ipsec proto",
			config: &L2TPConfig{
				ServerAddr:  "l2tp.example.com",
				Username:    "user",
				Password:    "pass",
				PSK:         "psk",
				PPPAuthType: "mschap-v2",
				IPSecProto:  "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := c.ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTrafficMonitor_RecordTraffic(t *testing.T) {
	m := NewTrafficMonitor()

	m.RecordTraffic("conn1", 1024, 512)

	stats, err := m.GetTrafficStats("conn1")
	if err != nil {
		t.Fatalf("GetTrafficStats() error = %v", err)
	}

	if stats.RxBytes != 1024 {
		t.Errorf("RxBytes = %v, want 1024", stats.RxBytes)
	}
	if stats.TxBytes != 512 {
		t.Errorf("TxBytes = %v, want 512", stats.TxBytes)
	}
}

func TestTrafficMonitor_GetTotalTraffic(t *testing.T) {
	m := NewTrafficMonitor()

	m.RecordTraffic("conn1", 1024, 512)
	m.RecordTraffic("conn2", 2048, 1024)

	total := m.GetTotalTraffic()
	if total.RxBytes != 3072 {
		t.Errorf("Total RxBytes = %v, want 3072", total.RxBytes)
	}
	if total.TxBytes != 1536 {
		t.Errorf("Total TxBytes = %v, want 1536", total.TxBytes)
	}
}

func TestTrafficMonitor_SetTrafficLimit(t *testing.T) {
	m := NewTrafficMonitor()

	m.SetTrafficLimit("profile1", 1024*1024) // 1MB

	limit := m.GetTrafficLimit("profile1")
	if limit != 1024*1024 {
		t.Errorf("Limit = %v, want %v", limit, 1024*1024)
	}
}

func TestTrafficMonitor_CheckTrafficLimit(t *testing.T) {
	m := NewTrafficMonitor()

	m.SetTrafficLimit("profile1", 1000)

	// Record some traffic
	m.TakeSnapshot("conn1", "profile1")
	m.RecordTraffic("conn1", 500, 300)
	m.TakeSnapshot("conn1", "profile1")

	exceeded, usage, limit := m.CheckTrafficLimit("profile1")
	if exceeded {
		t.Error("Expected traffic not to exceed limit")
	}
	if usage != 800 {
		t.Errorf("Usage = %v, want 800", usage)
	}
	if limit != 1000 {
		t.Errorf("Limit = %v, want 1000", limit)
	}
}

func TestTrafficMonitor_CreateAlert(t *testing.T) {
	m := NewTrafficMonitor()

	alert := m.CreateAlert("alert1", "profile1", 1024, "both", "day")
	if alert == nil {
		t.Fatal("CreateAlert() returned nil")
	}
	if alert.ID != "alert1" {
		t.Errorf("ID = %v, want alert1", alert.ID)
	}
	if alert.Threshold != 1024 {
		t.Errorf("Threshold = %v, want 1024", alert.Threshold)
	}
}

func TestTrafficMonitor_ListAlerts(t *testing.T) {
	m := NewTrafficMonitor()

	m.CreateAlert("alert1", "profile1", 1024, "rx", "day")
	m.CreateAlert("alert2", "profile2", 2048, "tx", "hour")

	alerts := m.ListAlerts()
	if len(alerts) != 2 {
		t.Errorf("ListAlerts() count = %v, want 2", len(alerts))
	}
}

func TestTrafficMonitor_DeleteAlert(t *testing.T) {
	m := NewTrafficMonitor()

	m.CreateAlert("alert1", "profile1", 1024, "rx", "day")

	err := m.DeleteAlert("alert1")
	if err != nil {
		t.Fatalf("DeleteAlert() error = %v", err)
	}

	_, err = m.GetAlert("alert1")
	if err == nil {
		t.Error("Expected error for deleted alert")
	}
}

func TestTrafficMonitor_ResetStats(t *testing.T) {
	m := NewTrafficMonitor()

	m.RecordTraffic("conn1", 1024, 512)
	m.ResetStats("conn1")

	stats, err := m.GetTrafficStats("conn1")
	if err != nil {
		t.Fatalf("GetTrafficStats() error = %v", err)
	}

	if stats.RxBytes != 0 {
		t.Errorf("RxBytes = %v, want 0", stats.RxBytes)
	}
}

func TestTrafficMonitor_GetUptime(t *testing.T) {
	m := NewTrafficMonitor()

	time.Sleep(100 * time.Millisecond)

	uptime := m.GetUptime()
	if uptime < 100*time.Millisecond {
		t.Errorf("Uptime = %v, expected >= 100ms", uptime)
	}
}

func TestManager_GetTrafficMonitor(t *testing.T) {
	m := NewManager()

	monitor := m.GetTrafficMonitor()
	if monitor == nil {
		t.Fatal("GetTrafficMonitor() returned nil")
	}
}

func TestManager_GetProtocolClients(t *testing.T) {
	m := NewManager()

	if m.GetOpenVPNClient() == nil {
		t.Error("GetOpenVPNClient() returned nil")
	}
	if m.GetWireGuardClient() == nil {
		t.Error("GetWireGuardClient() returned nil")
	}
	if m.GetL2TPClient() == nil {
		t.Error("GetL2TPClient() returned nil")
	}
}

func TestManager_GetFailoverProfile(t *testing.T) {
	m := NewManager()

	req1 := CreateProfileRequest{
		Name:       "primary",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn1.example.com",
		ServerPort: 1194,
	}
	req2 := CreateProfileRequest{
		Name:       "backup",
		Protocol:   ProtocolOpenVPN,
		ServerAddr: "vpn2.example.com",
		ServerPort: 1194,
	}

	p1, _ := m.CreateProfile(req1)
	m.CreateProfile(req2)

	failover, err := m.GetFailoverProfile(p1.ID)
	if err != nil {
		t.Fatalf("GetFailoverProfile() error = %v", err)
	}

	if failover.ID == p1.ID {
		t.Error("Failover profile should be different from primary")
	}
}

func TestParseOpenVPNConfig(t *testing.T) {
	content := `client
dev tun
proto tcp
remote 1.2.3.4 443
cipher AES-128-CBC
auth SHA1
verb 5
comp-lzo`

	config, err := parseOpenVPNConfig(content)
	if err != nil {
		t.Fatalf("parseOpenVPNConfig() error = %v", err)
	}

	if config.RemoteAddr != "1.2.3.4" {
		t.Errorf("RemoteAddr = %v, want 1.2.3.4", config.RemoteAddr)
	}
	if config.RemotePort != 443 {
		t.Errorf("RemotePort = %v, want 443", config.RemotePort)
	}
	if config.Protocol != "tcp" {
		t.Errorf("Protocol = %v, want tcp", config.Protocol)
	}
	if config.Cipher != "AES-128-CBC" {
		t.Errorf("Cipher = %v, want AES-128-CBC", config.Cipher)
	}
	if !config.CompLZO {
		t.Error("CompLZO should be true")
	}
}

func TestParseWireGuardConfig(t *testing.T) {
	content := `[Interface]
PrivateKey = abc123
Address = 10.0.0.2/32
DNS = 1.1.1.1
MTU = 1420

[Peer]
PublicKey = def456
Endpoint = wg.example.com:51820
AllowedIPs = 0.0.0.0/0
PersistentKeepalive = 25`

	config, err := parseWireGuardConfig(content)
	if err != nil {
		t.Fatalf("parseWireGuardConfig() error = %v", err)
	}

	if config.PrivateKey != "abc123" {
		t.Errorf("PrivateKey = %v, want abc123", config.PrivateKey)
	}
	if config.Address != "10.0.0.2/32" {
		t.Errorf("Address = %v, want 10.0.0.2/32", config.Address)
	}
	if config.PublicKey != "def456" {
		t.Errorf("PublicKey = %v, want def456", config.PublicKey)
	}
	if config.Endpoint != "wg.example.com:51820" {
		t.Errorf("Endpoint = %v, want wg.example.com:51820", config.Endpoint)
	}
	if config.MTU != 1420 {
		t.Errorf("MTU = %v, want 1420", config.MTU)
	}
	if config.Keepalive != 25 {
		t.Errorf("Keepalive = %v, want 25", config.Keepalive)
	}
}
