package filepreview

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewAudioPreviewer(t *testing.T) {
	config := DefaultPreviewConfig()
	previewer := NewAudioPreviewer(config)

	if previewer == nil {
		t.Fatal("NewAudioPreviewer returned nil")
	}

	if previewer.config != config {
		t.Error("Config not set correctly")
	}
}

func TestAudioPreviewer_NilConfig(t *testing.T) {
	previewer := NewAudioPreviewer(nil)

	if previewer == nil {
		t.Fatal("NewAudioPreviewer(nil) returned nil")
	}

	if previewer.config == nil {
		t.Error("Should use default config when nil is passed")
	}
}

func TestAudioPreviewer_Generate_FileNotFound(t *testing.T) {
	previewer := NewAudioPreviewer(nil)
	ctx := context.Background()

	req := &PreviewRequest{
		FilePath: "/nonexistent/audio.mp3",
	}

	_, err := previewer.Generate(ctx, req)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestAudioPreviewer_Generate_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.xyz")
	os.WriteFile(tmpFile, []byte("test"), 0o644)

	previewer := NewAudioPreviewer(nil)
	ctx := context.Background()

	req := &PreviewRequest{
		FilePath: tmpFile,
	}

	_, err := previewer.Generate(ctx, req)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestAudioPreviewer_GetAudioInfo_FileNotFound(t *testing.T) {
	previewer := NewAudioPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.GetAudioInfo(ctx, "/nonexistent/audio.mp3")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestAudioPreviewer_GetAudioInfo_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.xyz")
	os.WriteFile(tmpFile, []byte("test"), 0o644)

	previewer := NewAudioPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.GetAudioInfo(ctx, tmpFile)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestAudioPreviewer_ExtractWaveformData_FileNotFound(t *testing.T) {
	previewer := NewAudioPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.ExtractWaveformData(ctx, "/nonexistent/audio.mp3", 1000)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestAudioPreviewer_GenerateSpectrogram_FileNotFound(t *testing.T) {
	previewer := NewAudioPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.GenerateSpectrogram(ctx, "/nonexistent/audio.mp3", 1200, 400)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestAudioPreviewer_ExtractAudioCover_FileNotFound(t *testing.T) {
	previewer := NewAudioPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.ExtractAudioCover(ctx, "/nonexistent/audio.mp3")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestAudioPreviewer_GetAudioMetadata_FileNotFound(t *testing.T) {
	previewer := NewAudioPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.GetAudioMetadata(ctx, "/nonexistent/audio.mp3")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestAudioPreviewer_Downsample(t *testing.T) {
	previewer := NewAudioPreviewer(nil)

	tests := []struct {
		name       string
		data       []float64
		targetLen  int
		wantLen    int
	}{
		{
			name:      "downsample",
			data:      []float64{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8, 0.9, 1.0},
			targetLen: 5,
			wantLen:   5,
		},
		{
			name:      "no downsample needed",
			data:      []float64{0.1, 0.2, 0.3},
			targetLen: 5,
			wantLen:   3,
		},
		{
			name:      "empty data",
			data:      []float64{},
			targetLen: 5,
			wantLen:   0,
		},
		{
			name:      "single element",
			data:      []float64{0.5},
			targetLen: 5,
			wantLen:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := previewer.downsample(tt.data, tt.targetLen)
			if len(result) != tt.wantLen {
				t.Errorf("downsample() length = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestAudioPreviewer_CalculatePeaks(t *testing.T) {
	previewer := NewAudioPreviewer(nil)

	data := []float64{0.1, -0.9, 0.5, -0.3, 0.8, -0.2, 0.6, -0.7}
	peaks := previewer.calculatePeaks(data, 4)

	if len(peaks) != 4 {
		t.Errorf("calculatePeaks() length = %d, want 4", len(peaks))
	}

	// 检查峰值是否正确.
	expected := []float64{0.9, 0.5, 0.8, 0.7}
	for i, peak := range peaks {
		if peak != expected[i] {
			t.Errorf("peaks[%d] = %f, want %f", i, peak, expected[i])
		}
	}
}

func TestAudioPreviewer_CalculatePeaks_Empty(t *testing.T) {
	previewer := NewAudioPreviewer(nil)

	peaks := previewer.calculatePeaks([]float64{}, 5)
	if len(peaks) != 0 {
		t.Errorf("calculatePeaks(empty) length = %d, want 0", len(peaks))
	}
}

func TestAudioPreviewer_Downsample_EdgeCases(t *testing.T) {
	previewer := NewAudioPreviewer(nil)

	// 测试单元素数据.
	data := []float64{0.5}
	result := previewer.downsample(data, 1)
	if len(result) != 1 {
		t.Errorf("downsample single element: length = %d, want 1", len(result))
	}
	if result[0] != 0.5 {
		t.Errorf("downsample single element: value = %f, want 0.5", result[0])
	}

	// 测试目标长度为 0.
	result = previewer.downsample(data, 0)
	if len(result) != 1 {
		t.Errorf("downsample to 0: length = %d, want 1 (returns original)", len(result))
	}
}
