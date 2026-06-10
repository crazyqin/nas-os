package smartcam

import (
	"testing"
	"time"
)

// TestCameraStatus_Constants 测试摄像头状态常量定义.
func TestCameraStatus_Constants(t *testing.T) {
	tests := []struct {
		name   string
		status CameraStatus
		want   string
	}{
		{"online", CameraStatusOnline, "online"},
		{"offline", CameraStatusOffline, "offline"},
		{"error", CameraStatusError, "error"},
		{"disabled", CameraStatusDisabled, "disabled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("CameraStatus = %v, want %v", tt.status, tt.want)
			}
		})
	}
}

// TestRecordingMode_Constants 测试录像模式常量定义.
func TestRecordingMode_Constants(t *testing.T) {
	tests := []struct {
		name string
		mode RecordingMode
		want string
	}{
		{"continuous", RecordingModeContinuous, "continuous"},
		{"motion", RecordingModeMotion, "motion"},
		{"schedule", RecordingModeSchedule, "schedule"},
		{"manual", RecordingModeManual, "manual"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.mode) != tt.want {
				t.Errorf("RecordingMode = %v, want %v", tt.mode, tt.want)
			}
		})
	}
}

// TestStreamProtocol_Constants 测试视频流协议常量定义.
func TestStreamProtocol_Constants(t *testing.T) {
	tests := []struct {
		name     string
		protocol StreamProtocol
		want     string
	}{
		{"rtsp", ProtocolRTSP, "rtsp"},
		{"onvif", ProtocolONVIF, "onvif"},
		{"http", ProtocolHTTP, "http"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.protocol) != tt.want {
				t.Errorf("StreamProtocol = %v, want %v", tt.protocol, tt.want)
			}
		})
	}
}

// TestMotionSensitivity_Constants 测试移动侦测灵敏度常量定义.
func TestMotionSensitivity_Constants(t *testing.T) {
	tests := []struct {
		name        string
		sensitivity MotionSensitivity
		want        string
	}{
		{"low", SensitivityLow, "low"},
		{"medium", SensitivityMedium, "medium"},
		{"high", SensitivityHigh, "high"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.sensitivity) != tt.want {
				t.Errorf("MotionSensitivity = %v, want %v", tt.sensitivity, tt.want)
			}
		})
	}
}

// TestCamera_Creation 测试摄像头结构体创建.
func TestCamera_Creation(t *testing.T) {
	now := time.Now()
	camera := Camera{
		ID:           "cam-001",
		Name:         "前门摄像头",
		Location:     "大门入口",
		IPAddress:    "192.168.1.100",
		Port:         554,
		Protocol:     ProtocolRTSP,
		StreamURL:    "rtsp://192.168.1.100:554/stream",
		Username:     "admin",
		Password:     "password123",
		Model:        "DS-2CD2143G2-I",
		Manufacturer: "海康威视",
		Firmware:     "V5.6.0",
		Status:       CameraStatusOnline,
		Resolution:   "1080p",
		FrameRate:    25,
		Enabled:      true,
		Recording: RecordingConfig{
			Enabled:       true,
			Mode:          RecordingModeContinuous,
			Quality:       "high",
			PreRecord:     5,
			PostRecord:    10,
			MaxDuration:   3600,
			RetentionDays: 30,
		},
		Motion: MotionConfig{
			Enabled:     true,
			Sensitivity: SensitivityMedium,
			Threshold:   50.0,
			Zones: []MotionZone{
				{ID: "zone-1", Name: "入口区域", X: 0, Y: 0, Width: 1920, Height: 1080},
			},
			Actions: []MotionAction{
				{Type: "email", Target: "admin@example.com", Enabled: true},
				{Type: "snapshot", Target: "/snapshots", Enabled: true},
			},
			CooldownSec: 30,
		},
		Tags:      []string{"室外", "前门", "24小时"},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 验证基本字段
	if camera.ID != "cam-001" {
		t.Errorf("Camera.ID = %v, want cam-001", camera.ID)
	}
	if camera.Name != "前门摄像头" {
		t.Errorf("Camera.Name = %v, want 前门摄像头", camera.Name)
	}
	if camera.Status != CameraStatusOnline {
		t.Errorf("Camera.Status = %v, want online", camera.Status)
	}
	if camera.Port != 554 {
		t.Errorf("Camera.Port = %v, want 554", camera.Port)
	}
	if !camera.Enabled {
		t.Error("Camera.Enabled should be true")
	}
}

// TestRecordingConfig 测试录像配置.
func TestRecordingConfig(t *testing.T) {
	config := RecordingConfig{
		Enabled:       true,
		Mode:          RecordingModeSchedule,
		Quality:       "medium",
		PreRecord:     10,
		PostRecord:    30,
		MaxDuration:   7200,
		RetentionDays: 90,
		Schedule: []TimeSlot{
			{DayOfWeek: 1, StartTime: "08:00", EndTime: "18:00"},
			{DayOfWeek: 2, StartTime: "08:00", EndTime: "18:00"},
			{DayOfWeek: 3, StartTime: "08:00", EndTime: "18:00"},
			{DayOfWeek: 4, StartTime: "08:00", EndTime: "18:00"},
			{DayOfWeek: 5, StartTime: "08:00", EndTime: "18:00"},
		},
	}

	if !config.Enabled {
		t.Error("RecordingConfig.Enabled should be true")
	}
	if config.Mode != RecordingModeSchedule {
		t.Errorf("RecordingConfig.Mode = %v, want schedule", config.Mode)
	}
	if config.RetentionDays != 90 {
		t.Errorf("RecordingConfig.RetentionDays = %v, want 90", config.RetentionDays)
	}
	if len(config.Schedule) != 5 {
		t.Errorf("len(RecordingConfig.Schedule) = %v, want 5", len(config.Schedule))
	}
}

// TestMotionConfig 测试移动侦测配置.
func TestMotionConfig(t *testing.T) {
	config := MotionConfig{
		Enabled:     true,
		Sensitivity: SensitivityHigh,
		Threshold:   75.0,
		Zones: []MotionZone{
			{ID: "z1", Name: "主区域", X: 100, Y: 100, Width: 500, Height: 400},
			{ID: "z2", Name: "副区域", X: 600, Y: 200, Width: 300, Height: 300},
		},
		Actions: []MotionAction{
			{Type: "webhook", Target: "http://notify.example.com", Enabled: true},
			{Type: "record", Target: "", Enabled: true},
		},
		CooldownSec: 60,
	}

	if !config.Enabled {
		t.Error("MotionConfig.Enabled should be true")
	}
	if config.Sensitivity != SensitivityHigh {
		t.Errorf("MotionConfig.Sensitivity = %v, want high", config.Sensitivity)
	}
	if config.Threshold != 75.0 {
		t.Errorf("MotionConfig.Threshold = %v, want 75.0", config.Threshold)
	}
	if len(config.Zones) != 2 {
		t.Errorf("len(MotionConfig.Zones) = %v, want 2", len(config.Zones))
	}
	if len(config.Actions) != 2 {
		t.Errorf("len(MotionConfig.Actions) = %v, want 2", len(config.Actions))
	}
}

// TestRecording_Creation 测试录像记录创建.
func TestRecording_Creation(t *testing.T) {
	now := time.Now()
	recording := Recording{
		ID:           "rec-001",
		CameraID:     "cam-001",
		CameraName:   "前门摄像头",
		StartTime:    now.Add(-1 * time.Hour),
		EndTime:      now,
		Duration:     1 * time.Hour,
		FileSize:     1024 * 1024 * 500, // 500MB
		FilePath:     "/recordings/cam-001/2024-01-01/rec-001.mp4",
		Thumbnail:    "/thumbnails/cam-001/rec-001.jpg",
		Trigger:      "motion",
		MotionEvents: 3,
		Tags:         []string{"移动侦测", "前门"},
	}

	if recording.ID != "rec-001" {
		t.Errorf("Recording.ID = %v, want rec-001", recording.ID)
	}
	if recording.CameraID != "cam-001" {
		t.Errorf("Recording.CameraID = %v, want cam-001", recording.CameraID)
	}
	if recording.Trigger != "motion" {
		t.Errorf("Recording.Trigger = %v, want motion", recording.Trigger)
	}
	if recording.MotionEvents != 3 {
		t.Errorf("Recording.MotionEvents = %v, want 3", recording.MotionEvents)
	}
	if recording.FileSize != 1024*1024*500 {
		t.Errorf("Recording.FileSize = %v, want %v", recording.FileSize, 1024*1024*500)
	}
}

// TestMotionEvent_Creation 测试移动侦测事件创建.
func TestMotionEvent_Creation(t *testing.T) {
	now := time.Now()
	event := MotionEvent{
		ID:         "evt-001",
		CameraID:   "cam-001",
		CameraName: "前门摄像头",
		Timestamp:  now,
		ZoneID:     "zone-1",
		ZoneName:   "入口区域",
		Confidence: 0.85,
		BoundingBox: &BoundingBox{
			X:      200,
			Y:      150,
			Width:  300,
			Height: 250,
		},
		Snapshot: "/snapshots/evt-001.jpg",
		Handled:  false,
	}

	if event.ID != "evt-001" {
		t.Errorf("MotionEvent.ID = %v, want evt-001", event.ID)
	}
	if event.Confidence != 0.85 {
		t.Errorf("MotionEvent.Confidence = %v, want 0.85", event.Confidence)
	}
	if event.BoundingBox == nil {
		t.Fatal("MotionEvent.BoundingBox should not be nil")
	}
	if event.BoundingBox.X != 200 {
		t.Errorf("BoundingBox.X = %v, want 200", event.BoundingBox.X)
	}
	if event.Handled {
		t.Error("MotionEvent.Handled should be false")
	}
}

// TestAddCameraRequest 测试添加摄像头请求.
func TestAddCameraRequest(t *testing.T) {
	req := AddCameraRequest{
		Name:       "后门摄像头",
		Location:   "后门出口",
		IPAddress:  "192.168.1.101",
		Port:       554,
		Protocol:   ProtocolRTSP,
		Username:   "admin",
		Password:   "pass123",
		Resolution: "4K",
		FrameRate:  30,
		Tags:       []string{"室外", "后门"},
	}

	if req.Name != "后门摄像头" {
		t.Errorf("AddCameraRequest.Name = %v, want 后门摄像头", req.Name)
	}
	if req.IPAddress != "192.168.1.101" {
		t.Errorf("AddCameraRequest.IPAddress = %v, want 192.168.1.101", req.IPAddress)
	}
	if req.Protocol != ProtocolRTSP {
		t.Errorf("AddCameraRequest.Protocol = %v, want rtsp", req.Protocol)
	}
}

// TestSystemStatus 测试系统状态结构体.
func TestSystemStatus(t *testing.T) {
	status := SystemStatus{
		TotalCameras:    10,
		OnlineCameras:   8,
		OfflineCameras:  2,
		RecordingCount:  5,
		TotalRecordings: 1000,
		StorageUsed:     1024 * 1024 * 1024 * 100, // 100GB
		StorageTotal:    1024 * 1024 * 1024 * 500, // 500GB
		MotionEvents24h: 25,
		Uptime:          "72h30m",
	}

	if status.TotalCameras != 10 {
		t.Errorf("SystemStatus.TotalCameras = %v, want 10", status.TotalCameras)
	}
	if status.OnlineCameras+status.OfflineCameras != status.TotalCameras {
		t.Error("Online + Offline should equal Total")
	}
	if status.StorageUsed > status.StorageTotal {
		t.Error("StorageUsed should not exceed StorageTotal")
	}
}

// TestDiscoverResult 测试发现结果结构体.
func TestDiscoverResult(t *testing.T) {
	result := DiscoverResult{
		Found: 3,
		Cameras: []Camera{
			{ID: "cam-001", Name: "Camera 1", Status: CameraStatusOnline},
			{ID: "cam-002", Name: "Camera 2", Status: CameraStatusOnline},
			{ID: "cam-003", Name: "Camera 3", Status: CameraStatusOffline},
		},
		ScannedIPs: 254,
		Elapsed:    "5.2s",
	}

	if result.Found != 3 {
		t.Errorf("DiscoverResult.Found = %v, want 3", result.Found)
	}
	if len(result.Cameras) != 3 {
		t.Errorf("len(DiscoverResult.Cameras) = %v, want 3", len(result.Cameras))
	}
}

// TestErrorConstants 测试错误常量定义.
func TestErrorConstants(t *testing.T) {
	tests := []struct {
		name string
		err  string
		want string
	}{
		{"camera not found", ErrCameraNotFound, "摄像头不存在"},
		{"camera offline", ErrCameraOffline, "摄像头离线"},
		{"invalid config", ErrInvalidConfig, "无效的摄像头配置"},
		{"duplicate camera", ErrDuplicateCamera, "摄像头已存在"},
		{"recording in progress", ErrRecordingInProgress, "录像正在进行中"},
		{"recording not found", ErrRecordingNotFound, "录像不存在"},
		{"motion zone invalid", ErrMotionZoneInvalid, "无效的移动侦测区域"},
		{"schedule conflict", ErrScheduleConflict, "录像计划冲突"},
		{"storage full", ErrStorageFull, "存储空间已满"},
		{"stream failed", ErrStreamFailed, "视频流连接失败"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err != tt.want {
				t.Errorf("Error constant = %v, want %v", tt.err, tt.want)
			}
		})
	}
}
