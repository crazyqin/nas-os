package motionphoto

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDetectVendor_InvalidFile(t *testing.T) {
	_, err := DetectVendor("/nonexistent/path/photo.jpg")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestDetectVendor_RegularJPEG(t *testing.T) {
	// 创建一个空文件模拟 JPEG
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.jpg")
	if err := os.WriteFile(fpath, []byte{0xFF, 0xD8, 0xFF, 0xE0}, 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	vendor, err := DetectVendor(fpath)
	if err != nil {
		t.Fatalf("DetectVendor error: %v", err)
	}
	// 空文件无法识别厂商，应返回 unknown
	if vendor != VendorUnknown {
		t.Errorf("expected VendorUnknown, got %s", vendor)
	}
}

func TestNewParser_DefaultConfig(t *testing.T) {
	p := NewParser(nil)
	if p.config == nil {
		t.Error("expected non-nil config")
	}
	if p.config.MaxFileSize != 200*1024*1024 {
		t.Errorf("expected MaxFileSize=200MB, got %d", p.config.MaxFileSize)
	}
	if !p.config.EnableWebP {
		t.Error("expected EnableWebP=true by default")
	}
	if p.parsed == nil {
		t.Error("expected non-nil parsed map")
	}
}

func TestNewParser_CustomConfig(t *testing.T) {
	cfg := &ParserConfig{
		MaxFileSize: 100 * 1024 * 1024,
		OutputDir:   "/data/motionphoto",
		EnableWebP:  false,
	}
	p := NewParser(cfg)
	if p.config.MaxFileSize != 100*1024*1024 {
		t.Errorf("expected MaxFileSize=100MB, got %d", p.config.MaxFileSize)
	}
	if p.config.EnableWebP {
		t.Error("expected EnableWebP=false")
	}
}

func TestNewParser_NilWebPConfig(t *testing.T) {
	cfg := &ParserConfig{
		MaxFileSize: 50 * 1024 * 1024,
		OutputDir:   "/tmp/mp",
	}
	p := NewParser(cfg)
	if p.config.WebP == nil {
		t.Error("expected WebP config to be auto-created")
	}
	if p.config.WebP.Quality != 85 {
		t.Errorf("expected default WebP quality=85, got %.1f", p.config.WebP.Quality)
	}
}

func TestParser_Parse_UnsupportedVendor(t *testing.T) {
	dir := t.TempDir()
	fpath := filepath.Join(dir, "test.jpg")
	if err := os.WriteFile(fpath, []byte{0xFF, 0xD8, 0xFF, 0xE0}, 0644); err != nil {
		t.Fatalf("create test file: %v", err)
	}

	p := NewParser(&ParserConfig{OutputDir: dir})
	_, err := p.Parse(context.Background(), fpath)
	if err == nil {
		t.Error("expected error for unsupported/unknown vendor")
	}
}

func TestParser_Extract_WebPDisabled(t *testing.T) {
	dir := t.TempDir()
	p := NewParser(&ParserConfig{
		OutputDir:  dir,
		EnableWebP: false,
	})

	mp := &MotionPhoto{
		ID:        "test001",
		FilePath:  filepath.Join(dir, "test.jpg"),
		Vendor:    VendorHuawei,
		PhotoType: "jpeg",
		PhotoSize: 1024,
		VideoSize: 2048,
	}

	result, err := p.Extract(context.Background(), mp)
	if err != nil {
		t.Fatalf("Extract error: %v", err)
	}
	if result.PhotoPath == "" {
		t.Error("expected non-empty PhotoPath")
	}
	if result.VideoPath == "" {
		t.Error("expected non-empty VideoPath")
	}
	// WebP disabled, should be empty
	if result.WebPPath != "" {
		t.Error("expected empty WebPPath when WebP disabled")
	}
}

func TestMotionPhoto_Fields(t *testing.T) {
	now := time.Now()
	mp := &MotionPhoto{
		ID:          "mp001",
		FilePath:    "/photos/motion.jpg",
		Vendor:      VendorXiaomi,
		PhotoSize:   3 * 1024 * 1024,
		VideoSize:   5 * 1024 * 1024,
		VideoOffset: 3 * 1024 * 1024,
		VideoType:   "mp4",
		PhotoType:   "jpeg",
		Width:       4000,
		Height:      3000,
		VideoWidth:  1920,
		VideoHeight: 1080,
		Duration:    3.5,
		CreatedAt:   now,
		Metadata:    map[string]string{"model": "MI 14"},
	}

	if mp.Vendor != VendorXiaomi {
		t.Errorf("expected VendorXiaomi, got %s", mp.Vendor)
	}
	if mp.Width != 4000 {
		t.Errorf("expected Width=4000, got %d", mp.Width)
	}
	if mp.Duration != 3.5 {
		t.Errorf("expected Duration=3.5, got %f", mp.Duration)
	}
	if mp.Metadata["model"] != "MI 14" {
		t.Error("expected metadata model=MI 14")
	}
}

func TestExtensionForType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"jpeg", ".jpg"},
		{"jpg", ".jpg"},
		{"heic", ".heic"},
		{"webp", ".webp"},
		{"unknown", ".jpg"},
	}
	for _, tt := range tests {
		got := extensionForType(tt.input)
		if got != tt.expected {
			t.Errorf("extensionForType(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
