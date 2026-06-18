package cloudconnect

import (
	"testing"
	"time"
)

func TestNewCloudManager(t *testing.T) {
	cm := NewCloudManager()
	if cm == nil {
		t.Fatal("NewCloudManager returned nil")
	}

	if len(cm.clouds) != 0 {
		t.Errorf("Expected 0 clouds, got %d", len(cm.clouds))
	}
	if len(cm.devices) != 0 {
		t.Errorf("Expected 0 devices, got %d", len(cm.devices))
	}
}

func TestAddCloudConnection(t *testing.T) {
	cm := NewCloudManager()

	config := &CloudConfig{
		ID:       "aws-1",
		Name:     "AWS Main",
		Provider: ProviderAWS,
		Region:   "us-east-1",
		AccessKey: "AKIAIOSFODNN7EXAMPLE",
		SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
	}

	err := cm.AddCloudConnection(config)
	if err != nil {
		t.Fatalf("AddCloudConnection failed: %v", err)
	}

	if config.Status != StatusDisconnected {
		t.Errorf("Expected status disconnected, got %s", config.Status)
	}

	// Verify cloud exists
	clouds := cm.ListClouds()
	if len(clouds) != 1 {
		t.Errorf("Expected 1 cloud, got %d", len(clouds))
	}
}

func TestConnectCloud(t *testing.T) {
	cm := NewCloudManager()

	config := &CloudConfig{
		ID:       "azure-1",
		Name:     "Azure",
		Provider: ProviderAzure,
		Region:   "eastus",
	}
	cm.AddCloudConnection(config)

	err := cm.ConnectCloud("azure-1")
	if err != nil {
		t.Fatalf("ConnectCloud failed: %v", err)
	}

	// Verify connected
	status, err := cm.GetCloudStatus("azure-1")
	if err != nil {
		t.Fatalf("GetCloudStatus failed: %v", err)
	}
	if status.Status != StatusConnected {
		t.Errorf("Expected status connected, got %s", status.Status)
	}
	if status.ConnectedAt == nil {
		t.Error("ConnectedAt not set")
	}
}

func TestDisconnectCloud(t *testing.T) {
	cm := NewCloudManager()

	config := &CloudConfig{
		ID:       "gcp-1",
		Name:     "GCP",
		Provider: ProviderGCP,
	}
	cm.AddCloudConnection(config)
	cm.ConnectCloud("gcp-1")

	err := cm.DisconnectCloud("gcp-1")
	if err != nil {
		t.Fatalf("DisconnectCloud failed: %v", err)
	}

	status, _ := cm.GetCloudStatus("gcp-1")
	if status.Status != StatusDisconnected {
		t.Errorf("Expected status disconnected, got %s", status.Status)
	}
}

func TestRegisterDevice(t *testing.T) {
	cm := NewCloudManager()

	device := &RemoteDevice{
		ID:       "nas-remote-1",
		Name:     "Remote NAS",
		Type:     DeviceTypeNAS,
		Hostname: "nas.example.com",
		Port:     22,
	}

	err := cm.RegisterDevice(device)
	if err != nil {
		t.Fatalf("RegisterDevice failed: %v", err)
	}

	if device.Status != StatusDisconnected {
		t.Errorf("Expected status disconnected, got %s", device.Status)
	}

	// Verify device exists
	devices := cm.ListDevices()
	if len(devices) != 1 {
		t.Errorf("Expected 1 device, got %d", len(devices))
	}
}

func TestConnectDevice(t *testing.T) {
	cm := NewCloudManager()

	device := &RemoteDevice{
		ID:       "server-1",
		Name:     "Remote Server",
		Type:     DeviceTypeServer,
		Hostname: "server.example.com",
		Port:     22,
	}
	cm.RegisterDevice(device)

	err := cm.ConnectDevice("server-1")
	if err != nil {
		t.Fatalf("ConnectDevice failed: %v", err)
	}

	status, _ := cm.GetDeviceStatus("server-1")
	if status.Status != StatusConnected {
		t.Errorf("Expected status connected, got %s", status.Status)
	}
}

func TestSendCommand(t *testing.T) {
	cm := NewCloudManager()

	device := &RemoteDevice{
		ID:       "nas-1",
		Name:     "NAS",
		Type:     DeviceTypeNAS,
		Hostname: "nas.local",
		Port:     22,
	}
	cm.RegisterDevice(device)
	cm.ConnectDevice("nas-1")

	cmd, err := cm.SendCommand("nas-1", "uptime")
	if err != nil {
		t.Fatalf("SendCommand failed: %v", err)
	}

	// Wait for command completion
	time.Sleep(time.Second * 3)

	status, err := cm.GetCommandStatus(cmd.ID)
	if err != nil {
		t.Fatalf("GetCommandStatus failed: %v", err)
	}

	if status.Status != "completed" {
		t.Errorf("Expected status completed, got %s", status.Status)
	}
	if status.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", status.ExitCode)
	}
}

func TestCreateSyncJob(t *testing.T) {
	cm := NewCloudManager()

	job, err := cm.CreateSyncJob("/data/local", "s3://bucket/remote")
	if err != nil {
		t.Fatalf("CreateSyncJob failed: %v", err)
	}

	// Wait for sync completion
	time.Sleep(time.Second * 6)

	status, err := cm.GetSyncJobStatus(job.ID)
	if err != nil {
		t.Fatalf("GetSyncJobStatus failed: %v", err)
	}

	if status.Status != "completed" {
		t.Errorf("Expected status completed, got %s", status.Status)
	}
	if status.Progress != 100 {
		t.Errorf("Expected progress 100, got %f", status.Progress)
	}
}

func TestGetStats(t *testing.T) {
	cm := NewCloudManager()

	// Add cloud connections
	cm.AddCloudConnection(&CloudConfig{ID: "aws-1", Name: "AWS", Provider: ProviderAWS})
	cm.AddCloudConnection(&CloudConfig{ID: "azure-1", Name: "Azure", Provider: ProviderAzure})
	cm.ConnectCloud("aws-1")

	// Add devices
	cm.RegisterDevice(&RemoteDevice{ID: "nas-1", Name: "NAS", Type: DeviceTypeNAS})
	cm.RegisterDevice(&RemoteDevice{ID: "server-1", Name: "Server", Type: DeviceTypeServer})
	cm.ConnectDevice("nas-1")

	stats := cm.GetStats()

	if stats["total_clouds"] != 2 {
		t.Errorf("Expected 2 clouds, got %v", stats["total_clouds"])
	}
	if stats["connected_clouds"] != 1 {
		t.Errorf("Expected 1 connected cloud, got %v", stats["connected_clouds"])
	}
	if stats["total_devices"] != 2 {
		t.Errorf("Expected 2 devices, got %v", stats["total_devices"])
	}
	if stats["connected_devices"] != 1 {
		t.Errorf("Expected 1 connected device, got %v", stats["connected_devices"])
	}
}

func TestWebhookNotification(t *testing.T) {
	cm := NewCloudManager()

	received := make(chan string, 1)
	cm.AddWebhook(func(event string, data interface{}) {
		received <- event
	})

	device := &RemoteDevice{ID: "test-1", Name: "Test", Type: DeviceTypeNAS}
	cm.RegisterDevice(device)
	cm.ConnectDevice("test-1")

	select {
	case event := <-received:
		if event != "device.connected" {
			t.Errorf("Expected 'device.connected', got '%s'", event)
		}
	case <-time.After(time.Second * 3):
		t.Error("Webhook notification timeout")
	}
}
