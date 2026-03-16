package downloader

import (
	"testing"
	"time"
)

func TestDownloadType_Constants(t *testing.T) {
	if TypeBT != "bt" {
		t.Errorf("TypeBT = %s, want bt", TypeBT)
	}
	if TypeMagnet != "magnet" {
		t.Errorf("TypeMagnet = %s, want magnet", TypeMagnet)
	}
	if TypeHTTP != "http" {
		t.Errorf("TypeHTTP = %s, want http", TypeHTTP)
	}
	if TypeFTP != "ftp" {
		t.Errorf("TypeFTP = %s, want ftp", TypeFTP)
	}
	if TypeCloud != "cloud" {
		t.Errorf("TypeCloud = %s, want cloud", TypeCloud)
	}
}

func TestDownloadStatus_Constants(t *testing.T) {
	if StatusWaiting != "waiting" {
		t.Errorf("StatusWaiting = %s, want waiting", StatusWaiting)
	}
	if StatusDownloading != "downloading" {
		t.Errorf("StatusDownloading = %s, want downloading", StatusDownloading)
	}
	if StatusPaused != "paused" {
		t.Errorf("StatusPaused = %s, want paused", StatusPaused)
	}
	if StatusCompleted != "completed" {
		t.Errorf("StatusCompleted = %s, want completed", StatusCompleted)
	}
	if StatusError != "error" {
		t.Errorf("StatusError = %s, want error", StatusError)
	}
	if StatusSeeding != "seeding" {
		t.Errorf("StatusSeeding = %s, want seeding", StatusSeeding)
	}
}

func TestDownloadTask_Fields(t *testing.T) {
	now := time.Now()
	task := &DownloadTask{
		ID:          "abc123",
		Name:        "ubuntu-22.04.iso",
		Type:        TypeHTTP,
		URL:         "https://example.com/ubuntu.iso",
		Status:      StatusDownloading,
		Progress:    45.5,
		TotalSize:   3_221_225_472, // 3 GB
		Downloaded:  1_465_738_849, // ~45%
		Uploaded:    0,
		Speed:       10_485_760, // 10 MB/s
		UploadSpeed: 0,
		Peers:       0,
		Seeds:       0,
		Ratio:       0,
		DestPath:    "/downloads",
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if task.ID != "abc123" {
		t.Errorf("ID = %s, want abc123", task.ID)
	}

	if task.Name != "ubuntu-22.04.iso" {
		t.Errorf("Name = %s, want ubuntu-22.04.iso", task.Name)
	}

	if task.Type != TypeHTTP {
		t.Errorf("Type = %s, want http", task.Type)
	}

	if task.Progress != 45.5 {
		t.Errorf("Progress = %f, want 45.5", task.Progress)
	}

	if task.Speed != 10_485_760 {
		t.Errorf("Speed = %d, want 10485760", task.Speed)
	}
}

func TestDownloadTask_BTFields(t *testing.T) {
	now := time.Now()
	completedAt := now.Add(2 * time.Hour)

	task := &DownloadTask{
		ID:          "bt_456",
		Name:        "debian-12.iso",
		Type:        TypeBT,
		URL:         "magnet:?xt=urn:btih:abc123",
		Status:      StatusSeeding,
		Progress:    100,
		TotalSize:   3_221_225_472,
		Downloaded:  3_221_225_472,
		Uploaded:    6_442_450_944, // 2x upload
		Speed:       0,
		UploadSpeed: 5_242_880, // 5 MB/s
		Peers:       15,
		Seeds:       50,
		Ratio:       2.0,
		DownloadID:  "abc123def456",
		ClientRef:   "transmission:123",
		CreatedAt:   now,
		UpdatedAt:   now,
		CompletedAt: &completedAt,
	}

	if task.Status != StatusSeeding {
		t.Errorf("Status = %s, want seeding", task.Status)
	}

	if task.Peers != 15 {
		t.Errorf("Peers = %d, want 15", task.Peers)
	}

	if task.Ratio != 2.0 {
		t.Errorf("Ratio = %f, want 2.0", task.Ratio)
	}

	if task.CompletedAt == nil {
		t.Error("CompletedAt should not be nil")
	}
}

func TestScheduleConfig_Fields(t *testing.T) {
	config := &ScheduleConfig{
		StartTime: "22:00",
		EndTime:   "08:00",
		Days:      []int{0, 1, 2, 3, 4}, // Sun-Thu
		Enabled:   true,
	}

	if config.StartTime != "22:00" {
		t.Errorf("StartTime = %s, want 22:00", config.StartTime)
	}

	if config.EndTime != "08:00" {
		t.Errorf("EndTime = %s, want 08:00", config.EndTime)
	}

	if len(config.Days) != 5 {
		t.Errorf("Days count = %d, want 5", len(config.Days))
	}

	if !config.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestSpeedLimitConfig_Fields(t *testing.T) {
	config := &SpeedLimitConfig{
		DownloadLimit: 1024, // 1 MB/s
		UploadLimit:   512,  // 512 KB/s
		Enabled:       true,
	}

	if config.DownloadLimit != 1024 {
		t.Errorf("DownloadLimit = %d, want 1024", config.DownloadLimit)
	}

	if config.UploadLimit != 512 {
		t.Errorf("UploadLimit = %d, want 512", config.UploadLimit)
	}

	if !config.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestCreateTaskRequest_Fields(t *testing.T) {
	req := CreateTaskRequest{
		URL:      "https://example.com/file.zip",
		Name:     "file.zip",
		Type:     TypeHTTP,
		DestPath: "/downloads",
	}

	if req.URL != "https://example.com/file.zip" {
		t.Errorf("URL = %s, want https://example.com/file.zip", req.URL)
	}

	if req.Name != "file.zip" {
		t.Errorf("Name = %s, want file.zip", req.Name)
	}
}

func TestUpdateTaskRequest_Fields(t *testing.T) {
	speedLimit := &SpeedLimitConfig{DownloadLimit: 1024}
	req := UpdateTaskRequest{
		Status:     StatusPaused,
		SpeedLimit: speedLimit,
	}

	if req.Status != StatusPaused {
		t.Errorf("Status = %s, want paused", req.Status)
	}

	if req.SpeedLimit.DownloadLimit != 1024 {
		t.Errorf("SpeedLimit.DownloadLimit = %d, want 1024", req.SpeedLimit.DownloadLimit)
	}
}

func TestTaskStats_Fields(t *testing.T) {
	stats := TaskStats{
		TotalTasks:    10,
		Downloading:   3,
		Waiting:       2,
		Paused:        1,
		Completed:     4,
		Seeding:       2,
		TotalSpeed:    20_971_520,     // 20 MB/s
		TotalUploaded: 10_737_418_240, // 10 GB
	}

	if stats.TotalTasks != 10 {
		t.Errorf("TotalTasks = %d, want 10", stats.TotalTasks)
	}

	if stats.Downloading != 3 {
		t.Errorf("Downloading = %d, want 3", stats.Downloading)
	}

	if stats.TotalSpeed != 20_971_520 {
		t.Errorf("TotalSpeed = %d, want 20971520", stats.TotalSpeed)
	}
}

func TestPeerInfo_Fields(t *testing.T) {
	peer := PeerInfo{
		IP:       "192.168.1.100",
		Port:     51413,
		Client:   "Transmission 4.0",
		Progress: 75.5,
		Speed:    1048576, // 1 MB/s
	}

	if peer.IP != "192.168.1.100" {
		t.Errorf("IP = %s, want 192.168.1.100", peer.IP)
	}

	if peer.Port != 51413 {
		t.Errorf("Port = %d, want 51413", peer.Port)
	}

	if peer.Progress != 75.5 {
		t.Errorf("Progress = %f, want 75.5", peer.Progress)
	}
}

func TestTrackerInfo_Fields(t *testing.T) {
	now := time.Now()
	tracker := TrackerInfo{
		URL:        "udp://tracker.example.com:6969/announce",
		Status:     "active",
		Peers:      100,
		Seeds:      50,
		Leechers:   50,
		LastUpdate: now,
	}

	if tracker.URL != "udp://tracker.example.com:6969/announce" {
		t.Errorf("URL = %s, want udp://tracker.example.com:6969/announce", tracker.URL)
	}

	if tracker.Status != "active" {
		t.Errorf("Status = %s, want active", tracker.Status)
	}

	if tracker.Peers != 100 {
		t.Errorf("Peers = %d, want 100", tracker.Peers)
	}
}

func TestDownloadTask_CompletedAt(t *testing.T) {
	// Task without completion time
	task := &DownloadTask{
		Status: StatusDownloading,
	}

	if task.CompletedAt != nil {
		t.Error("CompletedAt should be nil for in-progress task")
	}

	// Completed task
	now := time.Now()
	task = &DownloadTask{
		Status:      StatusCompleted,
		CompletedAt: &now,
	}

	if task.CompletedAt == nil {
		t.Error("CompletedAt should not be nil for completed task")
	}
}

func TestDownloadTask_ErrorMessage(t *testing.T) {
	task := &DownloadTask{
		Status:       StatusError,
		ErrorMessage: "Connection refused",
	}

	if task.ErrorMessage != "Connection refused" {
		t.Errorf("ErrorMessage = %s, want Connection refused", task.ErrorMessage)
	}
}

func TestScheduleConfig_AllDays(t *testing.T) {
	config := &ScheduleConfig{
		Days: []int{0, 1, 2, 3, 4, 5, 6}, // All days
	}

	if len(config.Days) != 7 {
		t.Errorf("Days count = %d, want 7", len(config.Days))
	}
}

func TestSpeedLimitConfig_Disabled(t *testing.T) {
	config := &SpeedLimitConfig{
		DownloadLimit: 0, // Unlimited
		UploadLimit:   0, // Unlimited
		Enabled:       false,
	}

	if config.Enabled {
		t.Error("Enabled should be false")
	}

	if config.DownloadLimit != 0 {
		t.Errorf("DownloadLimit = %d, want 0 (unlimited)", config.DownloadLimit)
	}
}

func TestTaskStats_Consistency(t *testing.T) {
	stats := TaskStats{
		TotalTasks:  10,
		Downloading: 3,
		Waiting:     2,
		Paused:      1,
		Completed:   3,
		Seeding:     1,
	}

	// Total should equal sum of all states
	sum := stats.Downloading + stats.Waiting + stats.Paused + stats.Completed + stats.Seeding
	if sum != stats.TotalTasks {
		t.Errorf("Sum of states (%d) != TotalTasks (%d)", sum, stats.TotalTasks)
	}
}

func TestDownloadTask_Progress(t *testing.T) {
	tests := []struct {
		progress float64
		status   DownloadStatus
	}{
		{0, StatusWaiting},
		{25.5, StatusDownloading},
		{50.0, StatusDownloading},
		{100, StatusCompleted},
	}

	for _, tt := range tests {
		task := &DownloadTask{
			Progress: tt.progress,
			Status:   tt.status,
		}

		if task.Progress != tt.progress {
			t.Errorf("Progress = %f, want %f", task.Progress, tt.progress)
		}

		if task.Status != tt.status {
			t.Errorf("Status = %s, want %s", task.Status, tt.status)
		}
	}
}

func TestDownloadTask_MagnetLink(t *testing.T) {
	task := &DownloadTask{
		URL:  "magnet:?xt=urn:btih:1234567890abcdef&dn=test.iso",
		Type: TypeMagnet,
	}

	if task.Type != TypeMagnet {
		t.Errorf("Type = %s, want magnet", task.Type)
	}

	if !isValidMagnetURL(task.URL) {
		t.Errorf("URL %s is not a valid magnet link", task.URL)
	}
}

// Helper function for testing
func isValidMagnetURL(url string) bool {
	return len(url) > 7 && url[:7] == "magnet:"
}

func TestDownloadTask_ClientRef(t *testing.T) {
	task := &DownloadTask{
		ClientRef: "transmission:12345",
	}

	if task.ClientRef != "transmission:12345" {
		t.Errorf("ClientRef = %s, want transmission:12345", task.ClientRef)
	}
}

func TestCreateTaskRequest_EmptyName(t *testing.T) {
	req := CreateTaskRequest{
		URL:  "https://example.com/file.zip",
		Name: "", // Empty name should be filled from URL
	}

	if req.Name != "" {
		t.Error("Name should be empty when not provided")
	}
}
