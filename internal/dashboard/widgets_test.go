package dashboard

import (
	"testing"
	"time"

	"nas-os/internal/monitor"
)

func TestCPUWidgetProvider_GetType(t *testing.T) {
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Skipf("Failed to create monitor manager: %v", err)
	}
	provider := NewCPUWidgetProvider(monMgr)

	if provider.GetType() != WidgetTypeCPU {
		t.Errorf("Expected type %s, got %s", WidgetTypeCPU, provider.GetType())
	}
}

func TestCPUWidgetProvider_GetData(t *testing.T) {
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Skipf("Failed to create monitor manager: %v", err)
	}
	provider := NewCPUWidgetProvider(monMgr)

	widget := &Widget{
		ID:      "cpu-1",
		Type:    WidgetTypeCPU,
		Enabled: true,
		Config: WidgetConfig{
			ShowPerCore: false,
		},
	}

	data, err := provider.GetData(widget)
	if err != nil {
		t.Fatalf("Failed to get data: %v", err)
	}

	if data.WidgetID != widget.ID {
		t.Errorf("WidgetID mismatch: got %s, want %s", data.WidgetID, widget.ID)
	}
	if data.Type != WidgetTypeCPU {
		t.Errorf("Type mismatch: got %s, want %s", data.Type, WidgetTypeCPU)
	}

	cpuData, ok := data.Data.(*CPUWidgetData)
	if !ok {
		t.Fatal("Expected CPUWidgetData type")
	}

	if cpuData.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestMemoryWidgetProvider_GetType(t *testing.T) {
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Skipf("Failed to create monitor manager: %v", err)
	}
	provider := NewMemoryWidgetProvider(monMgr)

	if provider.GetType() != WidgetTypeMemory {
		t.Errorf("Expected type %s, got %s", WidgetTypeMemory, provider.GetType())
	}
}

func TestMemoryWidgetProvider_GetData(t *testing.T) {
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Skipf("Failed to create monitor manager: %v", err)
	}
	provider := NewMemoryWidgetProvider(monMgr)

	widget := &Widget{
		ID:      "mem-1",
		Type:    WidgetTypeMemory,
		Enabled: true,
		Config: WidgetConfig{
			ShowSwap:    true,
			ShowBuffers: true,
		},
	}

	data, err := provider.GetData(widget)
	if err != nil {
		t.Fatalf("Failed to get data: %v", err)
	}

	if data.WidgetID != widget.ID {
		t.Errorf("WidgetID mismatch: got %s, want %s", data.WidgetID, widget.ID)
	}

	memData := data.Data.(*MemoryWidgetData)
	if memData.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestDiskWidgetProvider_GetType(t *testing.T) {
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Skipf("Failed to create monitor manager: %v", err)
	}
	provider := NewDiskWidgetProvider(monMgr)

	if provider.GetType() != WidgetTypeDisk {
		t.Errorf("Expected type %s, got %s", WidgetTypeDisk, provider.GetType())
	}
}

func TestDiskWidgetProvider_GetData(t *testing.T) {
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Skipf("Failed to create monitor manager: %v", err)
	}
	provider := NewDiskWidgetProvider(monMgr)

	widget := &Widget{
		ID:      "disk-1",
		Type:    WidgetTypeDisk,
		Enabled: true,
		Config: WidgetConfig{
			ShowIOStats: true,
		},
	}

	data, err := provider.GetData(widget)
	if err != nil {
		t.Fatalf("Failed to get data: %v", err)
	}

	if data.WidgetID != widget.ID {
		t.Errorf("WidgetID mismatch: got %s, want %s", data.WidgetID, widget.ID)
	}

	diskData := data.Data.(*DiskWidgetData)
	if diskData.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestNetworkWidgetProvider_GetType(t *testing.T) {
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Skipf("Failed to create monitor manager: %v", err)
	}
	provider := NewNetworkWidgetProvider(monMgr)

	if provider.GetType() != WidgetTypeNetwork {
		t.Errorf("Expected type %s, got %s", WidgetTypeNetwork, provider.GetType())
	}
}

func TestNetworkWidgetProvider_GetData(t *testing.T) {
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Skipf("Failed to create monitor manager: %v", err)
	}
	provider := NewNetworkWidgetProvider(monMgr)

	widget := &Widget{
		ID:      "net-1",
		Type:    WidgetTypeNetwork,
		Enabled: true,
		Config: WidgetConfig{
			ShowPackets: true,
			ShowErrors:  true,
		},
	}

	data, err := provider.GetData(widget)
	if err != nil {
		t.Fatalf("Failed to get data: %v", err)
	}

	if data.WidgetID != widget.ID {
		t.Errorf("WidgetID mismatch: got %s, want %s", data.WidgetID, widget.ID)
	}

	netData := data.Data.(*NetworkWidgetData)
	if netData.Timestamp.IsZero() {
		t.Error("Expected non-zero timestamp")
	}
}

func TestWidgetRegistry_Register(t *testing.T) {
	registry := NewWidgetRegistry()

	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Skipf("Failed to create monitor manager: %v", err)
	}
	provider := NewCPUWidgetProvider(monMgr)
	registry.Register(provider)

	types := registry.GetAvailableTypes()
	if len(types) != 1 {
		t.Errorf("Expected 1 type, got %d", len(types))
	}
}

func TestWidgetRegistry_Get(t *testing.T) {
	registry := NewWidgetRegistry()
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Skipf("Failed to create monitor manager: %v", err)
	}
	provider := NewCPUWidgetProvider(monMgr)
	registry.Register(provider)

	retrieved, ok := registry.Get(WidgetTypeCPU)
	if !ok {
		t.Fatal("Expected to find CPU provider")
	}
	if retrieved.GetType() != WidgetTypeCPU {
		t.Errorf("Type mismatch: got %s, want %s", retrieved.GetType(), WidgetTypeCPU)
	}
}

func TestWidgetRegistry_Get_NotFound(t *testing.T) {
	registry := NewWidgetRegistry()

	_, ok := registry.Get(WidgetTypeCPU)
	if ok {
		t.Error("Expected not to find CPU provider")
	}
}

func TestDefaultWidgetRegistry(t *testing.T) {
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Skipf("Failed to create monitor manager: %v", err)
	}
	registry := DefaultWidgetRegistry(monMgr)

	types := registry.GetAvailableTypes()
	if len(types) != 4 {
		t.Errorf("Expected 4 types, got %d", len(types))
	}

	_, ok := registry.Get(WidgetTypeCPU)
	if !ok {
		t.Error("Expected CPU provider")
	}
	_, ok = registry.Get(WidgetTypeMemory)
	if !ok {
		t.Error("Expected Memory provider")
	}
	_, ok = registry.Get(WidgetTypeDisk)
	if !ok {
		t.Error("Expected Disk provider")
	}
	_, ok = registry.Get(WidgetTypeNetwork)
	if !ok {
		t.Error("Expected Network provider")
	}
}

func TestCreateDefaultWidgets(t *testing.T) {
	widgets := CreateDefaultWidgets()

	if len(widgets) != 4 {
		t.Errorf("Expected 4 default widgets, got %d", len(widgets))
	}

	types := make(map[WidgetType]bool)
	for _, w := range widgets {
		types[w.Type] = true
	}

	if !types[WidgetTypeCPU] {
		t.Error("Missing CPU widget")
	}
	if !types[WidgetTypeMemory] {
		t.Error("Missing Memory widget")
	}
	if !types[WidgetTypeDisk] {
		t.Error("Missing Disk widget")
	}
	if !types[WidgetTypeNetwork] {
		t.Error("Missing Network widget")
	}

	for _, w := range widgets {
		if !w.Enabled {
			t.Errorf("Widget %s should be enabled", w.Type)
		}
	}
}

func TestGetWidgetStatus_CPU(t *testing.T) {
	tests := []struct {
		name      string
		usage     float64
		threshold WidgetConfig
		expected  string
	}{
		{"Healthy", 50.0, WidgetConfig{WarningThreshold: 70, CriticalThreshold: 90}, "healthy"},
		{"Warning", 75.0, WidgetConfig{WarningThreshold: 70, CriticalThreshold: 90}, "warning"},
		{"Critical", 95.0, WidgetConfig{WarningThreshold: 70, CriticalThreshold: 90}, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &WidgetData{
				Type: WidgetTypeCPU,
				Data: &CPUWidgetData{Usage: tt.usage},
			}

			status := GetWidgetStatus(data, tt.threshold)
			if status != tt.expected {
				t.Errorf("Status mismatch: got %s, want %s", status, tt.expected)
			}
		})
	}
}

func TestGetWidgetStatus_Memory(t *testing.T) {
	tests := []struct {
		name      string
		usage     float64
		threshold WidgetConfig
		expected  string
	}{
		{"Healthy", 60.0, WidgetConfig{WarningThreshold: 80, CriticalThreshold: 95}, "healthy"},
		{"Warning", 85.0, WidgetConfig{WarningThreshold: 80, CriticalThreshold: 95}, "warning"},
		{"Critical", 96.0, WidgetConfig{WarningThreshold: 80, CriticalThreshold: 95}, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &WidgetData{
				Type: WidgetTypeMemory,
				Data: &MemoryWidgetData{Usage: tt.usage},
			}

			status := GetWidgetStatus(data, tt.threshold)
			if status != tt.expected {
				t.Errorf("Status mismatch: got %s, want %s", status, tt.expected)
			}
		})
	}
}

func TestGetWidgetStatus_Disk(t *testing.T) {
	tests := []struct {
		name     string
		devices  []DiskDeviceData
		expected string
	}{
		{
			name: "Healthy",
			devices: []DiskDeviceData{
				{UsagePercent: 50.0},
			},
			expected: "healthy",
		},
		{
			name: "Warning",
			devices: []DiskDeviceData{
				{UsagePercent: 85.0},
			},
			expected: "warning",
		},
		{
			name: "Critical",
			devices: []DiskDeviceData{
				{UsagePercent: 96.0},
			},
			expected: "critical",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &WidgetData{
				Type: WidgetTypeDisk,
				Data: &DiskWidgetData{Devices: tt.devices},
			}

			status := GetWidgetStatus(data, WidgetConfig{})
			if status != tt.expected {
				t.Errorf("Status mismatch: got %s, want %s", status, tt.expected)
			}
		})
	}
}

func TestGetWidgetStatus_Unknown(t *testing.T) {
	data := &WidgetData{
		Type: WidgetTypeCustom,
		Data: "some string",
	}

	status := GetWidgetStatus(data, WidgetConfig{})
	if status != "unknown" {
		t.Errorf("Status mismatch: got %s, want unknown", status)
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0 B"},
		{512, "512.00 B"},
		{1024, "1.00 KB"},
		{1024 * 1024, "1.00 MB"},
		{1024 * 1024 * 1024, "1.00 GB"},
		{1024 * 1024 * 1024 * 1024, "1.00 TB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
		}
	}
}

func TestFormatRate(t *testing.T) {
	tests := []struct {
		bytesPerSec uint64
		expected    string
	}{
		{1024, "1.00 KB/s"},
		{1024 * 1024, "1.00 MB/s"},
	}

	for _, tt := range tests {
		result := FormatRate(tt.bytesPerSec)
		if result != tt.expected {
			t.Errorf("FormatRate(%d) = %s, want %s", tt.bytesPerSec, result, tt.expected)
		}
	}
}

func TestCalculateUsagePercent(t *testing.T) {
	tests := []struct {
		used     uint64
		total    uint64
		expected float64
	}{
		{50, 100, 50.0},
		{1, 3, 33.333333333333336},
		{0, 100, 0.0},
		{100, 0, 0.0},
		{0, 0, 0.0},
	}

	for _, tt := range tests {
		result := calculateUsagePercent(tt.used, tt.total)
		if result != tt.expected {
			t.Errorf("calculateUsagePercent(%d, %d) = %f, want %f", tt.used, tt.total, result, tt.expected)
		}
	}
}

func TestWidgetData_Timestamp(t *testing.T) {
	monMgr, err := monitor.NewManager()
	if err != nil {
		t.Skipf("Failed to create monitor manager: %v", err)
	}
	provider := NewCPUWidgetProvider(monMgr)

	before := time.Now()
	widget := &Widget{
		ID:      "cpu-1",
		Type:    WidgetTypeCPU,
		Enabled: true,
	}
	data, _ := provider.GetData(widget)
	after := time.Now()

	if data.Timestamp.Before(before) || data.Timestamp.After(after) {
		t.Error("Timestamp should be within test execution time")
	}
}
