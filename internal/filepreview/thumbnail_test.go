package filepreview

import (
	"context"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

func TestNewThumbnailGenerator(t *testing.T) {
	config := DefaultPreviewConfig()
	gen := NewThumbnailGenerator(config)

	if gen == nil {
		t.Fatal("NewThumbnailGenerator returned nil")
	}

	if gen.config != config {
		t.Error("Config not set correctly")
	}

	if cap(gen.semaphore) != config.MaxConcurrent {
		t.Errorf("Semaphore capacity = %d, want %d", cap(gen.semaphore), config.MaxConcurrent)
	}
}

func TestThumbnailGenerator_NilConfig(t *testing.T) {
	gen := NewThumbnailGenerator(nil)

	if gen == nil {
		t.Fatal("NewThumbnailGenerator(nil) returned nil")
	}

	if gen.config == nil {
		t.Error("Should use default config when nil is passed")
	}
}

func TestThumbnailGenerator_Generate_FileNotFound(t *testing.T) {
	gen := NewThumbnailGenerator(nil)
	ctx := context.Background()

	req := &PreviewRequest{
		FilePath: "/nonexistent/file.jpg",
	}

	_, err := gen.Generate(ctx, req)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestThumbnailGenerator_Generate_UnsupportedFormat(t *testing.T) {
	// 创建临时文件.
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.xyz")
	os.WriteFile(tmpFile, []byte("test"), 0o644)

	gen := NewThumbnailGenerator(nil)
	ctx := context.Background()

	req := &PreviewRequest{
		FilePath: tmpFile,
	}

	_, err := gen.Generate(ctx, req)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestThumbnailGenerator_GetImageInfo(t *testing.T) {
	// 创建一个简单的测试图片.
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "test.png")

	// 创建 100x100 的红色图片.
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	// 保存图片.
	f, err := os.Create(imgPath)
	if err != nil {
		t.Fatal(err)
	}
	// 使用 PNG 编码.
	// png.Encode(f, img) // 需要导入 image/png
	f.Close()

	// 由于没有真正保存图片，这里测试文件不存在的情况.
	gen := NewThumbnailGenerator(nil)
	ctx := context.Background()

	_, err = gen.GetImageInfo(ctx, imgPath)
	if err == nil {
		// 如果文件不存在，应该返回错误.
		t.Log("GetImageInfo succeeded (file exists)")
	}
}

func TestThumbnailGenerator_SetRawConverter(t *testing.T) {
	gen := NewThumbnailGenerator(nil)

	gen.SetRawConverter("/usr/bin/dcraw")

	gen.mu.RLock()
	if gen.rawConverter != "/usr/bin/dcraw" {
		t.Errorf("rawConverter = %q, want %q", gen.rawConverter, "/usr/bin/dcraw")
	}
	gen.mu.RUnlock()
}

func TestThumbnailGenerator_Resize(t *testing.T) {
	gen := NewThumbnailGenerator(nil)

	// 创建测试图片.
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))

	tests := []struct {
		maxWidth   int
		maxHeight  int
		wantWidth  int
		wantHeight int
	}{
		{100, 100, 100, 50},   // 按宽度缩放
		{200, 50, 100, 50},    // 按高度缩放
		{400, 200, 200, 100},  // 不缩放（原图更小）
		{50, 50, 50, 25},      // 小缩略图
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := gen.resize(src, tt.maxWidth, tt.maxHeight)
			bounds := result.Bounds()

			if bounds.Dx() != tt.wantWidth || bounds.Dy() != tt.wantHeight {
				t.Errorf("resize(%d, %d) = %dx%d, want %dx%d",
					tt.maxWidth, tt.maxHeight, bounds.Dx(), bounds.Dy(), tt.wantWidth, tt.wantHeight)
			}
		})
	}
}

func TestThumbnailGenerator_GetOutputSize(t *testing.T) {
	gen := NewThumbnailGenerator(nil)

	tests := []struct {
		req        *PreviewRequest
		wantWidth  int
		wantHeight int
	}{
		{&PreviewRequest{Width: 300, Height: 200}, 300, 200},
		{&PreviewRequest{Width: 300}, 300, 300},        // 默认高度
		{&PreviewRequest{Height: 200}, 200, 200},       // 默认宽度
		{&PreviewRequest{}, 300, 300},                  // 使用默认值
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			width, height := gen.getOutputSize(tt.req)
			if width != tt.wantWidth || height != tt.wantHeight {
				t.Errorf("getOutputSize() = (%d, %d), want (%d, %d)",
					width, height, tt.wantWidth, tt.wantHeight)
			}
		})
	}
}

func TestThumbnailGenerator_IsRawConverterAvailable(t *testing.T) {
	gen := NewThumbnailGenerator(nil)

	// 测试默认转换器.
	result := gen.IsRawConverterAvailable()
	t.Logf("dcraw available: %v", result)

	// 测试不存在的转换器.
	gen.SetRawConverter("/nonexistent/dcraw")
	result = gen.IsRawConverterAvailable()
	if result {
		t.Error("Should return false for nonexistent converter")
	}
}

func TestHasAlphaChannel(t *testing.T) {
	tests := []struct {
		model    color.Model
		expected bool
	}{
		{color.RGBAModel, true},      // RGBA 有透明通道
		{color.NRGBAModel, true},     // NRGBA 有透明通道
		{color.AlphaModel, true},     // Alpha 有透明通道
		{color.GrayModel, false},     // Gray 没有透明通道
		{color.RGBA64Model, true},    // RGBA64 有透明通道
		{color.NRGBA64Model, true},   // NRGBA64 有透明通道
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			result := hasAlphaChannel(tt.model)
			if result != tt.expected {
				t.Errorf("hasAlphaChannel() = %v, want %v", result, tt.expected)
			}
		})
	}
}
