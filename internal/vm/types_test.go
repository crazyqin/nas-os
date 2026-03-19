package vm

import (
	"testing"
	"time"
)

func TestStatus_Constants(t *testing.T) {
	tests := []struct {
		status   Status
		expected string
	}{
		{StatusRunning, "running"},
		{StatusStopped, "stopped"},
		{StatusPaused, "paused"},
		{StatusCreating, "creating"},
		{StatusDeleting, "deleting"},
		{StatusSnapshot, "snapshotting"},
		{StatusRestoring, "restoring"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.status))
		}
	}
}

func TestType_Constants(t *testing.T) {
	tests := []struct {
		vmType   Type
		expected string
	}{
		{TypeLinux, "linux"},
		{TypeWindows, "windows"},
		{TypeOther, "other"},
	}

	for _, tt := range tests {
		if string(tt.vmType) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.vmType))
		}
	}
}

func TestVM_Fields(t *testing.T) {
	now := time.Now()
	vm := VM{
		ID:          "vm-001",
		Name:        "test-vm",
		Description: "Test VM",
		Type:        TypeLinux,
		Status:      StatusRunning,
		CreatedAt:   now,
		UpdatedAt:   now,
		CPU:         4,
		Memory:      8192,
		DiskSize:    100,
		Network:     "bridge",
		VNCPort:     5900,
		VNCEnabled:  true,
		Tags:        map[string]string{"env": "test"},
	}

	if vm.ID != "vm-001" {
		t.Error("ID mismatch")
	}
	if vm.CPU != 4 {
		t.Error("CPU mismatch")
	}
	if vm.Memory != 8192 {
		t.Error("Memory mismatch")
	}
	if vm.Status != StatusRunning {
		t.Error("Status mismatch")
	}
}

func TestConfig_Fields(t *testing.T) {
	config := Config{
		Name:        "new-vm",
		Description: "New VM",
		Type:        TypeLinux,
		CPU:         2,
		Memory:      4096,
		DiskSize:    50,
		Network:     "nat",
		VNCEnabled:  true,
		Tags:        map[string]string{"env": "dev"},
	}

	if config.Name != "new-vm" {
		t.Error("Name mismatch")
	}
	if config.CPU != 2 {
		t.Error("CPU mismatch")
	}
	if config.Network != "nat" {
		t.Error("Network mismatch")
	}
}

func TestStats_Fields(t *testing.T) {
	stats := Stats{
		CPUUsage:    45.5,
		MemoryUsage: 2048,
		DiskRead:    1024000,
		DiskWrite:   512000,
		NetRX:       2048000,
		NetTX:       1024000,
	}

	if stats.CPUUsage != 45.5 {
		t.Error("CPUUsage mismatch")
	}
	if stats.MemoryUsage != 2048 {
		t.Error("MemoryUsage mismatch")
	}
}

func TestISOImage_Fields(t *testing.T) {
	now := time.Now()
	iso := ISOImage{
		ID:         "iso-001",
		Name:       "ubuntu-22.04.iso",
		Path:       "/isos/ubuntu-22.04.iso",
		Size:       3221225472, // 3GB
		CreatedAt:  now,
		UpdatedAt:  now,
		IsUploaded: true,
		OS:         "linux",
	}

	if iso.ID != "iso-001" {
		t.Error("ID mismatch")
	}
	if iso.Size != 3221225472 {
		t.Error("Size mismatch")
	}
	if !iso.IsUploaded {
		t.Error("IsUploaded should be true")
	}
}

func TestSnapshot_Fields(t *testing.T) {
	now := time.Now()
	snapshot := Snapshot{
		ID:          "snap-001",
		VMID:        "vm-001",
		Name:        "before-update",
		Description: "Snapshot before system update",
		CreatedAt:   now,
		Size:        1073741824, // 1GB
		Status:      "ready",
	}

	if snapshot.ID != "snap-001" {
		t.Error("ID mismatch")
	}
	if snapshot.VMID != "vm-001" {
		t.Error("VMID mismatch")
	}
	if snapshot.Status != "ready" {
		t.Error("Status mismatch")
	}
}

func TestTemplate_Fields(t *testing.T) {
	now := time.Now()
	template := Template{
		ID:          "tpl-001",
		Name:        "Ubuntu Server",
		Description: "Ubuntu 22.04 LTS Server",
		Type:        TypeLinux,
		CPU:         2,
		Memory:      4096,
		DiskSize:    20,
		Network:     "bridge",
		OS:          "ubuntu-22.04",
		CreatedAt:   now,
		UpdatedAt:   now,
		Tags:        map[string]string{"os": "linux"},
	}

	if template.ID != "tpl-001" {
		t.Error("ID mismatch")
	}
	if template.OS != "ubuntu-22.04" {
		t.Error("OS mismatch")
	}
}

func TestVNCConnection_Fields(t *testing.T) {
	conn := VNCConnection{
		Host:     "192.168.1.100",
		Port:     5900,
		Password: "secret",
		Token:    "token123",
	}

	if conn.Host != "192.168.1.100" {
		t.Error("Host mismatch")
	}
	if conn.Port != 5900 {
		t.Error("Port mismatch")
	}
}

func TestUSBDevice_Fields(t *testing.T) {
	device := USBDevice{
		ID:           "usb-001",
		VendorID:     "1234",
		ProductID:    "5678",
		Manufacturer: "Test Manufacturer",
		Product:      "Test Device",
		InUse:        false,
	}

	if device.ID != "usb-001" {
		t.Error("ID mismatch")
	}
	if device.InUse {
		t.Error("InUse should be false")
	}
}

func TestPCIDevice_Fields(t *testing.T) {
	device := PCIDevice{
		ID:       "pci-001",
		BDF:      "0000:01:00.0",
		VendorID: "10de",
		DeviceID: "1b80",
		Name:     "NVIDIA GPU",
		InUse:    false,
		Driver:   "nvidia",
	}

	if device.ID != "pci-001" {
		t.Error("ID mismatch")
	}
	if device.BDF != "0000:01:00.0" {
		t.Error("BDF mismatch")
	}
}
