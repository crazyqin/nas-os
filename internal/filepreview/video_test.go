package filepreview

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewVideoPreviewer(t *testing.T) {
	config := DefaultPreviewConfig()
	previewer := NewVideoPreviewer(config)

	if previewer == nil {
		t.Fatal("NewVideoPreviewer returned nil")
	}

	if previewer.config != config {
		t.Error("Config not set correctly")
	}
}

func TestVideoPreviewer_NilConfig(t *testing.T) {
	previewer := NewVideoPreviewer(nil)

	if previewer == nil {
		t.Fatal("NewVideoPreviewer(nil) returned nil")
	}

	if previewer.config == nil {
		t.Error("Should use default config when nil is passed")
	}
}

func TestVideoPreviewer_Generate_FileNotFound(t *testing.T) {
	previewer := NewVideoPreviewer(nil)
	ctx := context.Background()

	req := &PreviewRequest{
		FilePath: "/nonexistent/video.mp4",
	}

	_, err := previewer.Generate(ctx, req)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestVideoPreviewer_Generate_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.xyz")
	os.WriteFile(tmpFile, []byte("test"), 0o644)

	previewer := NewVideoPreviewer(nil)
	ctx := context.Background()

	req := &PreviewRequest{
		FilePath: tmpFile,
	}

	_, err := previewer.Generate(ctx, req)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestVideoPreviewer_GetVideoInfo_FileNotFound(t *testing.T) {
	previewer := NewVideoPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.GetVideoInfo(ctx, "/nonexistent/video.mp4")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestVideoPreviewer_GetVideoInfo_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.xyz")
	os.WriteFile(tmpFile, []byte("test"), 0o644)

	previewer := NewVideoPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.GetVideoInfo(ctx, tmpFile)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestVideoPreviewer_ExtractKeyFrames_FileNotFound(t *testing.T) {
	previewer := NewVideoPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.ExtractKeyFrames(ctx, "/nonexistent/video.mp4", 10, 320, 180)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestVideoPreviewer_ExtractFrameAtTime_FileNotFound(t *testing.T) {
	previewer := NewVideoPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.ExtractFrameAtTime(ctx, "/nonexistent/video.mp4", 10.0, 320, 180)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestVideoPreviewer_GenerateSpriteSheet_FileNotFound(t *testing.T) {
	previewer := NewVideoPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.GenerateSpriteSheet(ctx, "/nonexistent/video.mp4", 5, 5, 160, 90)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestVideoPreviewer_GetOutputSize(t *testing.T) {
	previewer := NewVideoPreviewer(nil)
	videoInfo := &VideoInfo{
		Width:  1920,
		Height: 1080,
	}

	tests := []struct {
		req        *PreviewRequest
		wantWidth  int
		wantHeight int
	}{
		{&PreviewRequest{Width: 640, Height: 360}, 640, 360},
		{&PreviewRequest{Width: 640}, 640, 360},
		{&PreviewRequest{Height: 360}, 640, 360},
		{&PreviewRequest{}, 640, 360},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			width, height := previewer.getOutputSize(tt.req, videoInfo)
			if width != tt.wantWidth || height != tt.wantHeight {
				t.Errorf("getOutputSize() = (%d, %d), want (%d, %d)",
					width, height, tt.wantWidth, tt.wantHeight)
			}
		})
	}
}

func TestParseFrameRate(t *testing.T) {
	tests := []struct {
		input    string
		expected float64
	}{
		{"30/1", 30.0},
		{"24000/1001", 23.976023976023978},
		{"25/1", 25.0},
		{"60/1", 60.0},
		{"30", 30.0},
		{"", 0.0},
		{"invalid", 0.0},
		{"0/0", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseFrameRate(tt.input)
			if result != tt.expected {
				t.Errorf("parseFrameRate(%q) = %f, want %f", tt.input, result, tt.expected)
			}
		})
	}
}
