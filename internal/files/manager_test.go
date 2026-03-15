package files

import (
	"testing"
	"time"
)

func TestFileType_Constants(t *testing.T) {
	tests := []struct {
		fileType FileType
		expected string
	}{
		{FileTypeImage, "image"},
		{FileTypeVideo, "video"},
		{FileTypeAudio, "audio"},
		{FileTypeDocument, "document"},
		{FileTypePDF, "pdf"},
		{FileTypeCode, "code"},
		{FileTypeArchive, "archive"},
		{FileTypeOther, "other"},
	}

	for _, tt := range tests {
		if string(tt.fileType) != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, string(tt.fileType))
		}
	}
}

func TestFileInfo_Fields(t *testing.T) {
	info := FileInfo{
		Name:     "test.txt",
		Path:     "/path/to/test.txt",
		Size:     1024,
		Mode:     "-rw-r--r--",
		ModTime:  "2024-01-01",
		IsDir:    false,
		Type:     FileTypeDocument,
		MimeType: "text/plain",
	}

	if info.Name != "test.txt" {
		t.Error("Name mismatch")
	}
	if info.Size != 1024 {
		t.Error("Size mismatch")
	}
	if info.IsDir {
		t.Error("IsDir should be false")
	}
	if info.Type != FileTypeDocument {
		t.Error("Type mismatch")
	}
}

func TestPreviewConfig_Defaults(t *testing.T) {
	config := PreviewConfig{
		ThumbnailSize:    256,
		MaxPreviewSize:   10 * 1024 * 1024,
		CacheDir:         "/tmp/cache",
		CacheExpiry:      24 * time.Hour,
		EnableVideoThumb: true,
		EnableDocPreview: true,
	}

	if config.ThumbnailSize != 256 {
		t.Error("ThumbnailSize mismatch")
	}
	if config.MaxPreviewSize <= 0 {
		t.Error("MaxPreviewSize should be positive")
	}
	if config.CacheDir == "" {
		t.Error("CacheDir should not be empty")
	}
}

func TestShareInfo_Fields(t *testing.T) {
	now := time.Now()
	share := ShareInfo{
		Token:         "abc123",
		Path:          "/shared/file.txt",
		Password:      "secret",
		Expiry:        now.Add(24 * time.Hour),
		AllowDownload: true,
		CreatedAt:     now,
		Downloads:     0,
	}

	if share.Token != "abc123" {
		t.Error("Token mismatch")
	}
	if share.Path != "/shared/file.txt" {
		t.Error("Path mismatch")
	}
	if !share.AllowDownload {
		t.Error("AllowDownload should be true")
	}
}