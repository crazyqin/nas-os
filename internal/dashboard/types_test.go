package dashboard

import (
	"encoding/json"
	"testing"
	"time"
)

func TestWidgetType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		typ      WidgetType
		expected string
	}{
		{"CPU", WidgetTypeCPU, "cpu"},
		{"Memory", WidgetTypeMemory, "memory"},
		{"Disk", WidgetTypeDisk, "disk"},
		{"Network", WidgetTypeNetwork, "network"},
		{"Custom", WidgetTypeCustom, "custom"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.typ) != tt.expected {
				t.Errorf("WidgetType %s = %s, want %s", tt.name, tt.typ, tt.expected)
			}
		})
	}
}

func TestWidgetSize_Constants(t *testing.T) {
	tests := []struct {
		name     string
		size     WidgetSize
		expected string
	}{
		{"Small", WidgetSizeSmall, "small"},
		{"Medium", WidgetSizeMedium, "medium"},
		{"Large", WidgetSizeLarge, "large"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.size) != tt.expected {
				t.Errorf("WidgetSize %s = %s, want %s", tt.name, tt.size, tt.expected)
			}
		})
	}
}

func TestWidgetPosition_JSON(t *testing.T) {
	pos := WidgetPosition{X: 1, Y: 2}
	data, err := json.Marshal(pos)
	if err != nil {
		t.Fatalf("Failed to marshal WidgetPosition: %v", err)
	}

	var unmarshaled WidgetPosition
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal WidgetPosition: %v", err)
	}

	if unmarshaled.X != pos.X || unmarshaled.Y != pos.Y {
		t.Errorf("WidgetPosition mismatch: got %+v, want %+v", unmarshaled, pos)
	}
}

func TestWidget_JSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	widget := &Widget{
		ID:          "test-widget-1",
		Type:        WidgetTypeCPU,
		Title:       "CPU Monitor",
		Size:        WidgetSizeMedium,
		Position:    WidgetPosition{X: 0, Y: 0},
		Config:      WidgetConfig{ShowPerCore: true, WarningThreshold: 70},
		Enabled:     true,
		RefreshRate: 5 * time.Second,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	data, err := json.Marshal(widget)
	if err != nil {
		t.Fatalf("Failed to marshal Widget: %v", err)
	}

	var unmarshaled Widget
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal Widget: %v", err)
	}

	if unmarshaled.ID != widget.ID {
		t.Errorf("ID mismatch: got %s, want %s", unmarshaled.ID, widget.ID)
	}
	if unmarshaled.Type != widget.Type {
		t.Errorf("Type mismatch: got %s, want %s", unmarshaled.Type, widget.Type)
	}
	if unmarshaled.Title != widget.Title {
		t.Errorf("Title mismatch: got %s, want %s", unmarshaled.Title, widget.Title)
	}
	if unmarshaled.Enabled != widget.Enabled {
		t.Errorf("Enabled mismatch: got %v, want %v", unmarshaled.Enabled, widget.Enabled)
	}
}

func TestWidgetConfig_JSON(t *testing.T) {
	config := WidgetConfig{
		ShowPerCore:       true,
		ShowAverage:       false,
		WarningThreshold:  70.5,
		CriticalThreshold: 90.0,
		ShowSwap:          true,
		ShowBuffers:       true,
		MountPoints:       []string{"/", "/home"},
		ShowIOStats:       true,
		Interfaces:        []string{"eth0", "wlan0"},
		ShowPackets:       true,
		ShowErrors:        true,
		MaxDataPoints:     100,
		TimeRange:         "1h",
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal WidgetConfig: %v", err)
	}

	var unmarshaled WidgetConfig
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal WidgetConfig: %v", err)
	}

	if unmarshaled.ShowPerCore != config.ShowPerCore {
		t.Errorf("ShowPerCore mismatch: got %v, want %v", unmarshaled.ShowPerCore, config.ShowPerCore)
	}
	if len(unmarshaled.MountPoints) != len(config.MountPoints) {
		t.Errorf("MountPoints length mismatch: got %d, want %d", len(unmarshaled.MountPoints), len(config.MountPoints))
	}
}

func TestDashboard_JSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	dashboard := &Dashboard{
		ID:          "dash-1",
		Name:        "Main Dashboard",
		Description: "System monitoring dashboard",
		Widgets: []*Widget{
			{
				ID:        "widget-1",
				Type:      WidgetTypeCPU,
				Title:     "CPU",
				Size:      WidgetSizeSmall,
				Enabled:   true,
				CreatedAt: now,
				UpdatedAt: now,
			},
		},
		Layout: Layout{
			Columns: 2,
			Rows:    2,
			Gap:     10,
		},
		IsDefault: true,
		OwnerID:   "user-1",
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(dashboard)
	if err != nil {
		t.Fatalf("Failed to marshal Dashboard: %v", err)
	}

	var unmarshaled Dashboard
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal Dashboard: %v", err)
	}

	if unmarshaled.ID != dashboard.ID {
		t.Errorf("ID mismatch: got %s, want %s", unmarshaled.ID, dashboard.ID)
	}
	if unmarshaled.Name != dashboard.Name {
		t.Errorf("Name mismatch: got %s, want %s", unmarshaled.Name, dashboard.Name)
	}
	if len(unmarshaled.Widgets) != len(dashboard.Widgets) {
		t.Errorf("Widgets length mismatch: got %d, want %d", len(unmarshaled.Widgets), len(dashboard.Widgets))
	}
	if unmarshaled.Layout.Columns != dashboard.Layout.Columns {
		t.Errorf("Layout.Columns mismatch: got %d, want %d", unmarshaled.Layout.Columns, dashboard.Layout.Columns)
	}
}

func TestLayout_JSON(t *testing.T) {
	layout := Layout{
		Columns: 3,
		Rows:    2,
		Gap:     15,
	}

	data, err := json.Marshal(layout)
	if err != nil {
		t.Fatalf("Failed to marshal Layout: %v", err)
	}

	var unmarshaled Layout
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal Layout: %v", err)
	}

	if unmarshaled.Columns != layout.Columns {
		t.Errorf("Columns mismatch: got %d, want %d", unmarshaled.Columns, layout.Columns)
	}
	if unmarshaled.Rows != layout.Rows {
		t.Errorf("Rows mismatch: got %d, want %d", unmarshaled.Rows, layout.Rows)
	}
	if unmarshaled.Gap != layout.Gap {
		t.Errorf("Gap mismatch: got %d, want %d", unmarshaled.Gap, layout.Gap)
	}
}

func TestWidgetData_JSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	widgetData := &WidgetData{
		WidgetID:  "widget-1",
		Type:      WidgetTypeCPU,
		Timestamp: now,
		Data: map[string]interface{}{
			"usage": 45.5,
			"cores": 4,
		},
		Error: "",
	}

	data, err := json.Marshal(widgetData)
	if err != nil {
		t.Fatalf("Failed to marshal WidgetData: %v", err)
	}

	var unmarshaled WidgetData
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal WidgetData: %v", err)
	}

	if unmarshaled.WidgetID != widgetData.WidgetID {
		t.Errorf("WidgetID mismatch: got %s, want %s", unmarshaled.WidgetID, widgetData.WidgetID)
	}
	if unmarshaled.Type != widgetData.Type {
		t.Errorf("Type mismatch: got %s, want %s", unmarshaled.Type, widgetData.Type)
	}
}

func TestCPUWidgetData_JSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	data := &CPUWidgetData{
		Timestamp:    now,
		Usage:        45.5,
		PerCore:      []float64{40.0, 50.0, 45.0, 47.0},
		LoadAvg1:     1.5,
		LoadAvg5:     1.2,
		LoadAvg15:    1.0,
		ProcessCount: 150,
		Trend:        []float64{40.0, 42.0, 45.0, 45.5},
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal CPUWidgetData: %v", err)
	}

	var unmarshaled CPUWidgetData
	if err := json.Unmarshal(bytes, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal CPUWidgetData: %v", err)
	}

	if unmarshaled.Usage != data.Usage {
		t.Errorf("Usage mismatch: got %f, want %f", unmarshaled.Usage, data.Usage)
	}
	if len(unmarshaled.PerCore) != len(data.PerCore) {
		t.Errorf("PerCore length mismatch: got %d, want %d", len(unmarshaled.PerCore), len(data.PerCore))
	}
}

func TestMemoryWidgetData_JSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	data := &MemoryWidgetData{
		Timestamp: now,
		Total:     16 * 1024 * 1024 * 1024, // 16GB
		Used:      8 * 1024 * 1024 * 1024,  // 8GB
		Free:      8 * 1024 * 1024 * 1024,  // 8GB
		Available: 10 * 1024 * 1024 * 1024, // 10GB
		Usage:     50.0,
		SwapTotal: 8 * 1024 * 1024 * 1024,
		SwapUsed:  1 * 1024 * 1024 * 1024,
		SwapFree:  7 * 1024 * 1024 * 1024,
		SwapUsage: 12.5,
		Buffers:   500 * 1024 * 1024,
		Cached:    2 * 1024 * 1024 * 1024,
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal MemoryWidgetData: %v", err)
	}

	var unmarshaled MemoryWidgetData
	if err := json.Unmarshal(bytes, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal MemoryWidgetData: %v", err)
	}

	if unmarshaled.Usage != data.Usage {
		t.Errorf("Usage mismatch: got %f, want %f", unmarshaled.Usage, data.Usage)
	}
	if unmarshaled.Total != data.Total {
		t.Errorf("Total mismatch: got %d, want %d", unmarshaled.Total, data.Total)
	}
}

func TestDiskWidgetData_JSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	data := &DiskWidgetData{
		Timestamp: now,
		Devices: []DiskDeviceData{
			{
				Device:       "/dev/sda1",
				MountPoint:   "/",
				Total:        500 * 1024 * 1024 * 1024,
				Used:         250 * 1024 * 1024 * 1024,
				Free:         250 * 1024 * 1024 * 1024,
				UsagePercent: 50.0,
				FSType:       "ext4",
				ReadBytes:    1024 * 1024 * 1024,
				WriteBytes:   512 * 1024 * 1024,
			},
		},
		Total: DiskSummaryData{
			Total:        500 * 1024 * 1024 * 1024,
			Used:         250 * 1024 * 1024 * 1024,
			Free:         250 * 1024 * 1024 * 1024,
			UsagePercent: 50.0,
		},
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal DiskWidgetData: %v", err)
	}

	var unmarshaled DiskWidgetData
	if err := json.Unmarshal(bytes, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal DiskWidgetData: %v", err)
	}

	if len(unmarshaled.Devices) != len(data.Devices) {
		t.Errorf("Devices length mismatch: got %d, want %d", len(unmarshaled.Devices), len(data.Devices))
	}
	if unmarshaled.Total.UsagePercent != data.Total.UsagePercent {
		t.Errorf("Total.UsagePercent mismatch: got %f, want %f", unmarshaled.Total.UsagePercent, data.Total.UsagePercent)
	}
}

func TestNetworkWidgetData_JSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	data := &NetworkWidgetData{
		Timestamp: now,
		Interfaces: []NetworkInterfaceData{
			{
				Name:      "eth0",
				RXBytes:   1024 * 1024 * 1024,
				TXBytes:   512 * 1024 * 1024,
				RXPackets: 1000000,
				TXPackets: 500000,
				RXErrors:  0,
				TXErrors:  0,
				Speed:     1000,
			},
		},
		Total: NetworkSummaryData{
			RXBytes:   1024 * 1024 * 1024,
			TXBytes:   512 * 1024 * 1024,
			RXPackets: 1000000,
			TXPackets: 500000,
		},
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("Failed to marshal NetworkWidgetData: %v", err)
	}

	var unmarshaled NetworkWidgetData
	if err := json.Unmarshal(bytes, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal NetworkWidgetData: %v", err)
	}

	if len(unmarshaled.Interfaces) != len(data.Interfaces) {
		t.Errorf("Interfaces length mismatch: got %d, want %d", len(unmarshaled.Interfaces), len(data.Interfaces))
	}
	if unmarshaled.Total.RXBytes != data.Total.RXBytes {
		t.Errorf("Total.RXBytes mismatch: got %d, want %d", unmarshaled.Total.RXBytes, data.Total.RXBytes)
	}
}

func TestDashboardState_JSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	state := &DashboardState{
		DashboardID: "dash-1",
		LastUpdate:  now,
		WidgetData: map[string]*WidgetData{
			"widget-1": {
				WidgetID:  "widget-1",
				Type:      WidgetTypeCPU,
				Timestamp: now,
				Data:      map[string]interface{}{"usage": 45.5},
			},
		},
		HealthScore: 85.5,
		Status:      "healthy",
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatalf("Failed to marshal DashboardState: %v", err)
	}

	var unmarshaled DashboardState
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal DashboardState: %v", err)
	}

	if unmarshaled.DashboardID != state.DashboardID {
		t.Errorf("DashboardID mismatch: got %s, want %s", unmarshaled.DashboardID, state.DashboardID)
	}
	if unmarshaled.HealthScore != state.HealthScore {
		t.Errorf("HealthScore mismatch: got %f, want %f", unmarshaled.HealthScore, state.HealthScore)
	}
	if unmarshaled.Status != state.Status {
		t.Errorf("Status mismatch: got %s, want %s", unmarshaled.Status, state.Status)
	}
}

func TestDashboardTemplate_JSON(t *testing.T) {
	template := &DashboardTemplate{
		ID:          "template-1",
		Name:        "Server Monitor",
		Description: "Basic server monitoring template",
		Category:    "infrastructure",
		Widgets: []*WidgetConfig{
			{
				ShowPerCore:  true,
				ShowSwap:     true,
				ShowIOStats:  true,
				ShowPackets:  true,
				MaxDataPoints: 100,
			},
		},
		Layout: Layout{
			Columns: 2,
			Rows:    2,
			Gap:     10,
		},
	}

	data, err := json.Marshal(template)
	if err != nil {
		t.Fatalf("Failed to marshal DashboardTemplate: %v", err)
	}

	var unmarshaled DashboardTemplate
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal DashboardTemplate: %v", err)
	}

	if unmarshaled.ID != template.ID {
		t.Errorf("ID mismatch: got %s, want %s", unmarshaled.ID, template.ID)
	}
	if unmarshaled.Category != template.Category {
		t.Errorf("Category mismatch: got %s, want %s", unmarshaled.Category, template.Category)
	}
}

func TestDashboardEvent_JSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	event := &DashboardEvent{
		Type:        "refresh",
		DashboardID: "dash-1",
		WidgetID:    "widget-1",
		Timestamp:   now,
		Data:        map[string]interface{}{"usage": 45.5},
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("Failed to marshal DashboardEvent: %v", err)
	}

	var unmarshaled DashboardEvent
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal DashboardEvent: %v", err)
	}

	if unmarshaled.Type != event.Type {
		t.Errorf("Type mismatch: got %s, want %s", unmarshaled.Type, event.Type)
	}
	if unmarshaled.DashboardID != event.DashboardID {
		t.Errorf("DashboardID mismatch: got %s, want %s", unmarshaled.DashboardID, event.DashboardID)
	}
}

func TestWidget_Enabled(t *testing.T) {
	enabledWidget := &Widget{
		ID:      "widget-1",
		Type:    WidgetTypeCPU,
		Enabled: true,
	}

	disabledWidget := &Widget{
		ID:      "widget-2",
		Type:    WidgetTypeMemory,
		Enabled: false,
	}

	if !enabledWidget.Enabled {
		t.Error("Expected widget to be enabled")
	}
	if disabledWidget.Enabled {
		t.Error("Expected widget to be disabled")
	}
}

func TestWidget_EmptyFields(t *testing.T) {
	widget := &Widget{
		ID:   "widget-1",
		Type: WidgetTypeCPU,
		// 其他字段为零值
	}

	data, err := json.Marshal(widget)
	if err != nil {
		t.Fatalf("Failed to marshal widget with empty fields: %v", err)
	}

	var unmarshaled Widget
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal widget: %v", err)
	}

	if unmarshaled.ID != widget.ID {
		t.Errorf("ID mismatch: got %s, want %s", unmarshaled.ID, widget.ID)
	}
	if unmarshaled.Enabled != false {
		t.Errorf("Expected Enabled to be false by default")
	}
}

func TestWidgetConfig_DefaultValues(t *testing.T) {
	config := WidgetConfig{}

	if config.ShowPerCore != false {
		t.Error("Expected ShowPerCore to be false by default")
	}
	if config.WarningThreshold != 0 {
		t.Error("Expected WarningThreshold to be 0 by default")
	}
	if len(config.MountPoints) != 0 {
		t.Error("Expected MountPoints to be empty by default")
	}
}