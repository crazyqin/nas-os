package media

import (
	"testing"
	"time"
)

func TestDefaultThumbnailConfig(t *testing.T) {
	config := DefaultThumbnailConfig()

	if config.Width != 320 {
		t.Errorf("Width = %d, want 320", config.Width)
	}

	if config.Height != 180 {
		t.Errorf("Height = %d, want 180", config.Height)
	}

	if config.Format != "jpg" {
		t.Errorf("Format = %s, want jpg", config.Format)
	}

	if config.Quality != 85 {
		t.Errorf("Quality = %d, want 85", config.Quality)
	}
}

func TestNewThumbnailGenerator(t *testing.T) {
	gen := NewThumbnailGenerator("/usr/bin/ffmpeg", "/usr/bin/ffprobe")
	if gen == nil {
		t.Fatal("NewThumbnailGenerator returned nil")
	}

	if gen.ffmpegPath != "/usr/bin/ffmpeg" {
		t.Errorf("ffmpegPath = %s, want /usr/bin/ffmpeg", gen.ffmpegPath)
	}
}

func TestNewThumbnailGenerator_DefaultPaths(t *testing.T) {
	gen := NewThumbnailGenerator("", "")

	if gen.ffmpegPath != "ffmpeg" {
		t.Errorf("Default ffmpegPath = %s, want ffmpeg", gen.ffmpegPath)
	}

	if gen.ffprobePath != "ffprobe" {
		t.Errorf("Default ffprobePath = %s, want ffprobe", gen.ffprobePath)
	}
}

func TestThumbnailConfig_Fields(t *testing.T) {
	config := ThumbnailConfig{
		Width:           640,
		Height:          360,
		Format:          "png",
		Quality:         90,
		Timestamp:       10.5,
		KeepAspectRatio: false,
	}

	if config.Width != 640 {
		t.Errorf("Width = %d, want 640", config.Width)
	}

	if config.Height != 360 {
		t.Errorf("Height = %d, want 360", config.Height)
	}

	if config.Format != "png" {
		t.Errorf("Format = %s, want png", config.Format)
	}

	if config.Timestamp != 10.5 {
		t.Errorf("Timestamp = %f, want 10.5", config.Timestamp)
	}

	if config.KeepAspectRatio {
		t.Error("KeepAspectRatio should be false")
	}
}

func TestThumbnailGenerator_BuildThumbnailArgs(t *testing.T) {
	gen := NewThumbnailGenerator("ffmpeg", "ffprobe")

	config := ThumbnailConfig{
		Width:           320,
		Height:          180,
		Format:          "jpg",
		Quality:         85,
		Timestamp:       5.0,
		KeepAspectRatio: true,
	}

	args := gen.buildThumbnailArgs("/input.mp4", "/output.jpg", config)

	// Check for essential arguments
	hasInput := false
	hasOutput := false
	hasTimestamp := false

	for i, arg := range args {
		if arg == "-i" && i+1 < len(args) && args[i+1] == "/input.mp4" {
			hasInput = true
		}
		if arg == "/output.jpg" {
			hasOutput = true
		}
		if arg == "-ss" && i+1 < len(args) {
			hasTimestamp = true
		}
	}

	if !hasInput {
		t.Error("Missing input argument")
	}

	if !hasOutput {
		t.Error("Missing output argument")
	}

	if !hasTimestamp {
		t.Error("Missing timestamp argument")
	}
}

func TestThumbnailGenerator_BuildThumbnailArgs_Formats(t *testing.T) {
	gen := NewThumbnailGenerator("ffmpeg", "ffprobe")

	tests := []struct {
		format      string
		expectedArg string
	}{
		{"jpg", "-c:v"},
		{"png", "-c:v"},
		{"webp", "-c:v"},
	}

	for _, tt := range tests {
		config := ThumbnailConfig{
			Format:    tt.format,
			Width:     320,
			Height:    180,
			Quality:   85,
			Timestamp: 0,
		}

		args := gen.buildThumbnailArgs("/input.mp4", "/output."+tt.format, config)

		found := false
		for _, arg := range args {
			if arg == tt.expectedArg {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Format %s: expected argument %s not found", tt.format, tt.expectedArg)
		}
	}
}

func TestThumbnailInfo_Fields(t *testing.T) {
	info := &ThumbnailInfo{
		Path:      "/path/to/thumb.jpg",
		Width:     320,
		Height:    180,
		Format:    "jpeg",
		Size:      15000,
		CreatedAt: time.Now(),
	}

	if info.Width != 320 {
		t.Errorf("Width = %d, want 320", info.Width)
	}

	if info.Height != 180 {
		t.Errorf("Height = %d, want 180", info.Height)
	}

	if info.Size != 15000 {
		t.Errorf("Size = %d, want 15000", info.Size)
	}
}

func TestSpriteInfo_Fields(t *testing.T) {
	info := &SpriteInfo{
		Path:       "/path/to/sprite.jpg",
		Width:      1920,
		Height:     1080,
		Cols:       10,
		Rows:       10,
		Timestamps: []float64{1.0, 2.0, 3.0},
		Duration:   100.0,
	}

	if info.Cols != 10 {
		t.Errorf("Cols = %d, want 10", info.Cols)
	}

	if info.Rows != 10 {
		t.Errorf("Rows = %d, want 10", info.Rows)
	}

	if info.Duration != 100.0 {
		t.Errorf("Duration = %f, want 100.0", info.Duration)
	}

	if len(info.Timestamps) != 3 {
		t.Errorf("Timestamps count = %d, want 3", len(info.Timestamps))
	}
}

func TestThumbnailGenerator_GenerateFromVideo_FileNotFound(t *testing.T) {
	gen := NewThumbnailGenerator("ffmpeg", "ffprobe")
	config := DefaultThumbnailConfig()

	err := gen.GenerateFromVideo("/nonexistent/video.mp4", "/tmp/thumb.jpg", config)
	if err == nil {
		t.Error("GenerateFromVideo should return error for non-existent file")
	}
}

func TestThumbnailGenerator_GenerateMultiple(t *testing.T) {
	// This test verifies the function signature and basic behavior
	// Without ffprobe, duration detection will fail
	gen := NewThumbnailGenerator("ffmpeg", "ffprobe")

	// Create temp dir for output
	tmpDir := t.TempDir()

	// This will fail without a real video file, but we verify the function doesn't panic
	_, err := gen.GenerateMultiple("/nonexistent.mp4", tmpDir, 5, DefaultThumbnailConfig())
	if err == nil {
		t.Error("GenerateMultiple should return error for non-existent file")
	}
}

func TestThumbnailGenerator_GenerateSprite(t *testing.T) {
	gen := NewThumbnailGenerator("ffmpeg", "ffprobe")

	tmpDir := t.TempDir()
	outputPath := tmpDir + "/sprite.jpg"

	// Without real ffmpeg/ffprobe, this will fail
	_, err := gen.GenerateSprite("/nonexistent.mp4", outputPath, 4, 4, DefaultThumbnailConfig())
	if err == nil {
		t.Error("GenerateSprite should return error for non-existent file")
	}
}

func TestThumbnailGenerator_GeneratePreviewGif(t *testing.T) {
	gen := NewThumbnailGenerator("ffmpeg", "ffprobe")

	tmpDir := t.TempDir()
	outputPath := tmpDir + "/preview.gif"
	config := DefaultThumbnailConfig()

	err := gen.GeneratePreviewGif("/nonexistent.mp4", outputPath, 5.0, config)
	if err == nil {
		t.Error("GeneratePreviewGif should return error for non-existent file")
	}
}

func TestThumbnailGenerator_BatchGenerate(t *testing.T) {
	gen := NewThumbnailGenerator("ffmpeg", "ffprobe")

	tmpDir := t.TempDir()
	config := DefaultThumbnailConfig()

	videos := []string{"/nonexistent1.mp4", "/nonexistent2.mp4"}
	results := gen.BatchGenerate(videos, tmpDir, config, 2)

	if len(results) != 2 {
		t.Errorf("BatchGenerate results count = %d, want 2", len(results))
	}

	// All should have errors
	for _, video := range videos {
		if results[video] == nil {
			t.Errorf("BatchGenerate should have error for %s", video)
		}
	}
}
