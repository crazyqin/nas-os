package media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDefaultStreamConfig(t *testing.T) {
	config := DefaultStreamConfig()

	if config.VideoBitrate != "2M" {
		t.Errorf("VideoBitrate = %s, want 2M", config.VideoBitrate)
	}

	if config.AudioBitrate != "128k" {
		t.Errorf("AudioBitrate = %s, want 128k", config.AudioBitrate)
	}

	if config.Resolution != "1280x720" {
		t.Errorf("Resolution = %s, want 1280x720", config.Resolution)
	}

	if config.HLSSegmentDuration != 6 {
		t.Errorf("HLSSegmentDuration = %d, want 6", config.HLSSegmentDuration)
	}
}

func TestNewStreamServer(t *testing.T) {
	config := DefaultStreamConfig()
	server := NewStreamServer(config, "/usr/bin/ffmpeg", "/usr/bin/ffprobe")

	if server == nil {
		t.Fatal("NewStreamServer returned nil")
	}

	if server.ffmpegPath != "/usr/bin/ffmpeg" {
		t.Errorf("ffmpegPath = %s, want /usr/bin/ffmpeg", server.ffmpegPath)
	}
}

func TestNewStreamServer_DefaultPaths(t *testing.T) {
	config := DefaultStreamConfig()
	server := NewStreamServer(config, "", "")

	if server.ffmpegPath != "ffmpeg" {
		t.Errorf("Default ffmpegPath = %s, want ffmpeg", server.ffmpegPath)
	}

	if server.ffprobePath != "ffprobe" {
		t.Errorf("Default ffprobePath = %s, want ffprobe", server.ffprobePath)
	}
}

func TestStreamServer_GetSession(t *testing.T) {
	config := DefaultStreamConfig()
	server := NewStreamServer(config, "ffmpeg", "ffprobe")

	// Test non-existent session
	session := server.GetSession("nonexistent")
	if session != nil {
		t.Error("GetSession should return nil for non-existent session")
	}
}

func TestStreamServer_ListSessions(t *testing.T) {
	config := DefaultStreamConfig()
	server := NewStreamServer(config, "ffmpeg", "ffprobe")

	sessions := server.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("ListSessions should return empty slice initially, got %d", len(sessions))
	}
}

func TestStreamServer_StopSession_NonExistent(t *testing.T) {
	config := DefaultStreamConfig()
	server := NewStreamServer(config, "ffmpeg", "ffprobe")

	err := server.StopSession("nonexistent")
	if err == nil {
		t.Error("StopSession should return error for non-existent session")
	}
}

func TestStreamServer_DeleteSession_NonExistent(t *testing.T) {
	config := DefaultStreamConfig()
	server := NewStreamServer(config, "ffmpeg", "ffprobe")

	err := server.DeleteSession("nonexistent")
	if err == nil {
		t.Error("DeleteSession should return error for non-existent session")
	}
}

func TestStreamConfig_Fields(t *testing.T) {
	config := StreamConfig{
		VideoBitrate:       "5M",
		AudioBitrate:       "256k",
		Resolution:         "1920x1080",
		Framerate:          60,
		HLSSegmentDuration: 10,
		HLSListSize:        6,
		HWAccel:            "cuda",
	}

	if config.VideoBitrate != "5M" {
		t.Errorf("VideoBitrate = %s, want 5M", config.VideoBitrate)
	}

	if config.Framerate != 60 {
		t.Errorf("Framerate = %d, want 60", config.Framerate)
	}

	if config.HWAccel != "cuda" {
		t.Errorf("HWAccel = %s, want cuda", config.HWAccel)
	}
}

func TestStreamSession_Fields(t *testing.T) {
	now := time.Now()
	session := &StreamSession{
		ID:          "hls_123",
		SourcePath:  "/path/to/video.mp4",
		Type:        "hls",
		Status:      "running",
		StartTime:   now,
		Viewers:     5,
		OutputDir:   "/tmp/hls_output",
		ManifestURL: "/stream/hls_123/stream.m3u8",
	}

	if session.ID != "hls_123" {
		t.Errorf("ID = %s, want hls_123", session.ID)
	}

	if session.Type != "hls" {
		t.Errorf("Type = %s, want hls", session.Type)
	}

	if session.Status != "running" {
		t.Errorf("Status = %s, want running", session.Status)
	}

	if session.Viewers != 5 {
		t.Errorf("Viewers = %d, want 5", session.Viewers)
	}
}

func TestStreamServer_GetContentType(t *testing.T) {
	config := DefaultStreamConfig()
	server := NewStreamServer(config, "ffmpeg", "ffprobe")

	tests := []struct {
		filename string
		expected string
	}{
		{"video.mp4", "video/mp4"},
		{"video.mkv", "video/x-matroska"},
		{"video.webm", "video/webm"},
		{"video.avi", "video/x-msvideo"},
		{"video.mov", "video/quicktime"},
		{"playlist.m3u8", "application/vnd.apple.mpegurl"},
		{"segment.ts", "video/mp2t"},
		{"audio.mp3", "audio/mpeg"},
		{"audio.m4a", "audio/mp4"},
		{"audio.flac", "audio/flac"},
		{"audio.wav", "audio/wav"},
		{"unknown.xyz", "application/octet-stream"},
	}

	for _, tt := range tests {
		result := server.getContentType(tt.filename)
		if result != tt.expected {
			t.Errorf("getContentType(%s) = %s, want %s", tt.filename, result, tt.expected)
		}
	}
}

func TestStreamServer_BuildHLSArgs(t *testing.T) {
	config := StreamConfig{
		VideoBitrate:       "2M",
		AudioBitrate:       "128k",
		Resolution:         "1280x720",
		Framerate:          30,
		HLSSegmentDuration: 6,
		HLSListSize:        10,
		HWAccel:            "none",
	}

	server := NewStreamServer(config, "ffmpeg", "ffprobe")
	args := server.buildHLSArgs("/input/video.mp4", "/output/stream.m3u8", "/output/segment_%03d.ts")

	// Check essential arguments are present
	hasInput := false
	hasOutput := false
	for _, arg := range args {
		if arg == "/input/video.mp4" {
			hasInput = true
		}
		if arg == "/output/stream.m3u8" {
			hasOutput = true
		}
	}

	if !hasInput {
		t.Error("HLS args missing input file")
	}

	if !hasOutput {
		t.Error("HLS args missing output file")
	}
}

func TestStreamServer_BuildHLSArgs_HWAccel(t *testing.T) {
	tests := []struct {
		hwaccel    string
		expectedVc string
	}{
		{"cuda", "h264_nvenc"},
		{"nvenc", "h264_nvenc"},
		{"qsv", "h264_qsv"},
		{"vaapi", "h264_vaapi"},
		{"none", "libx264"},
	}

	for _, tt := range tests {
		config := StreamConfig{
			HWAccel:            tt.hwaccel,
			HLSSegmentDuration: 6,
			HLSListSize:        10,
		}

		server := NewStreamServer(config, "ffmpeg", "ffprobe")
		args := server.buildHLSArgs("/input.mp4", "/out.m3u8", "/out_%03d.ts")

		found := false
		for i, arg := range args {
			if arg == "-c:v" && i+1 < len(args) && args[i+1] == tt.expectedVc {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("HWAccel %s: expected video codec %s not found in args", tt.hwaccel, tt.expectedVc)
		}
	}
}

func TestStreamServer_ParseBitrate(t *testing.T) {
	config := DefaultStreamConfig()
	server := NewStreamServer(config, "ffmpeg", "ffprobe")

	tests := []struct {
		input    string
		expected int
	}{
		{"2M", 2000000},
		{"5M", 5000000},
		{"128k", 128000},
		{"256K", 256000},
		{"1000", 1000},
	}

	for _, tt := range tests {
		result := server.parseBitrate(tt.input)
		if result != tt.expected {
			t.Errorf("parseBitrate(%s) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestStreamServer_IncrementViewer(t *testing.T) {
	config := DefaultStreamConfig()
	server := NewStreamServer(config, "ffmpeg", "ffprobe")

	// Create a test session
	session := &StreamSession{
		ID:        "test_session",
		Status:    "running",
		Viewers:   0,
		StartTime: time.Now(),
	}
	server.mu.Lock()
	server.sessions["test_session"] = session
	server.mu.Unlock()

	server.IncrementViewer("test_session")

	if session.Viewers != 1 {
		t.Errorf("Viewers = %d, want 1", session.Viewers)
	}

	// Increment again
	server.IncrementViewer("test_session")

	if session.Viewers != 2 {
		t.Errorf("Viewers = %d, want 2", session.Viewers)
	}
}

func TestStreamServer_DecrementViewer(t *testing.T) {
	config := DefaultStreamConfig()
	server := NewStreamServer(config, "ffmpeg", "ffprobe")

	session := &StreamSession{
		ID:        "test_session",
		Status:    "running",
		Viewers:   5,
		StartTime: time.Now(),
	}
	server.mu.Lock()
	server.sessions["test_session"] = session
	server.mu.Unlock()

	server.DecrementViewer("test_session")

	if session.Viewers != 4 {
		t.Errorf("Viewers = %d, want 4", session.Viewers)
	}

	// Decrement below zero should not go negative
	session.Viewers = 0
	server.DecrementViewer("test_session")

	if session.Viewers != 0 {
		t.Errorf("Viewers = %d, want 0 (should not go negative)", session.Viewers)
	}
}

func TestStreamServer_CleanupStaleSessions(t *testing.T) {
	config := DefaultStreamConfig()
	server := NewStreamServer(config, "ffmpeg", "ffprobe")

	// Add completed session
	oldSession := &StreamSession{
		ID:        "old_session",
		Status:    "completed",
		StartTime: time.Now().Add(-2 * time.Hour),
	}

	// Add running session
	runningSession := &StreamSession{
		ID:        "running_session",
		Status:    "running",
		StartTime: time.Now(),
	}

	server.mu.Lock()
	server.sessions["old_session"] = oldSession
	server.sessions["running_session"] = runningSession
	server.mu.Unlock()

	count := server.CleanupStaleSessions(1 * time.Hour)

	if count != 1 {
		t.Errorf("CleanupStaleSessions removed %d sessions, want 1", count)
	}

	// Running session should still exist
	if server.GetSession("running_session") == nil {
		t.Error("Running session should not be cleaned up")
	}

	// Old session should be removed
	if server.GetSession("old_session") != nil {
		t.Error("Old session should be cleaned up")
	}
}

func TestAdaptiveStream_Fields(t *testing.T) {
	stream := AdaptiveStream{
		Quality:    "1080p",
		Resolution: "1920x1080",
		Bitrate:    "5M",
	}

	if stream.Quality != "1080p" {
		t.Errorf("Quality = %s, want 1080p", stream.Quality)
	}

	if stream.Resolution != "1920x1080" {
		t.Errorf("Resolution = %s, want 1920x1080", stream.Resolution)
	}
}

func TestStreamServer_StreamFile(t *testing.T) {
	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("Hello, World!")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := DefaultStreamConfig()
	server := NewStreamServer(config, "ffmpeg", "ffprobe")

	req := httptest.NewRequest("GET", "/stream", nil)
	w := httptest.NewRecorder()

	err := server.StreamFile(w, req, testFile)
	if err != nil {
		t.Fatalf("StreamFile returned error: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("Status = %d, want 200", w.Code)
	}

	if w.Body.String() != string(content) {
		t.Errorf("Body = %s, want %s", w.Body.String(), string(content))
	}
}

func TestStreamServer_StreamFile_Range(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	content := []byte("Hello, World! This is a test file.")
	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	config := DefaultStreamConfig()
	server := NewStreamServer(config, "ffmpeg", "ffprobe")

	req := httptest.NewRequest("GET", "/stream", nil)
	req.Header.Set("Range", "bytes=0-4")
	w := httptest.NewRecorder()

	err := server.StreamFile(w, req, testFile)
	if err != nil {
		t.Fatalf("StreamFile returned error: %v", err)
	}

	if w.Code != http.StatusPartialContent {
		t.Errorf("Status = %d, want 206", w.Code)
	}

	if w.Body.String() != "Hello" {
		t.Errorf("Body = %s, want Hello", w.Body.String())
	}
}

func TestStreamServer_TranscodeStream(t *testing.T) {
	// This test verifies the function exists and accepts correct parameters
	// Without actual ffmpeg, it will fail, so we just verify the structure
	config := DefaultStreamConfig()
	server := NewStreamServer(config, "ffmpeg", "ffprobe")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Try to transcode - this will fail without ffmpeg but shouldn't panic
	_ = server.TranscodeStream(ctx, "/nonexistent.mp4", config, nil)
}
