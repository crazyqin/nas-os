package files

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNewManager(t *testing.T) {
	config := PreviewConfig{
		ThumbnailSize:    128,
		MaxPreviewSize:   5 * 1024 * 1024,
		CacheDir:         t.TempDir(),
		CacheExpiry:      time.Hour,
		EnableVideoThumb: false,
		EnableDocPreview: true,
	}

	m := NewManager(config)
	require.NotNil(t, m)
	assert.Equal(t, config.ThumbnailSize, m.config.ThumbnailSize)
}

func TestManager_GetFileType(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	tests := []struct {
		name     string
		filename string
		expected FileType
	}{
		{"image jpg", "test.jpg", FileTypeImage},
		{"image jpeg", "test.jpeg", FileTypeImage},
		{"image png", "test.png", FileTypeImage},
		{"image gif", "test.gif", FileTypeImage},
		{"image webp", "test.webp", FileTypeImage},
		{"video mp4", "video.mp4", FileTypeVideo},
		{"video mkv", "video.mkv", FileTypeVideo},
		{"video avi", "video.avi", FileTypeVideo},
		{"video mov", "video.mov", FileTypeVideo},
		{"audio mp3", "audio.mp3", FileTypeAudio},
		{"audio wav", "audio.wav", FileTypeAudio},
		{"audio flac", "audio.flac", FileTypeAudio},
		{"document txt", "doc.txt", FileTypeDocument},
		{"document doc", "doc.doc", FileTypeDocument},
		{"document docx", "doc.docx", FileTypeDocument},
		{"pdf", "file.pdf", FileTypePDF},
		{"code go", "main.go", FileTypeCode},
		{"code js", "app.js", FileTypeCode},
		{"code py", "script.py", FileTypeCode},
		{"archive zip", "file.zip", FileTypeArchive},
		{"archive tar", "file.tar", FileTypeArchive},
		{"archive gz", "file.tar.gz", FileTypeArchive},
		{"other", "unknown.xyz", FileTypeOther},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpFile := filepath.Join(t.TempDir(), tt.filename)
			err := os.WriteFile(tmpFile, []byte("test"), 0644)
			require.NoError(t, err)

			result := m.GetFileType(tmpFile)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestManager_GetMimeType(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{"jpg", "test.jpg", "image/jpeg"},
		{"jpeg", "test.jpeg", "image/jpeg"},
		{"png", "test.png", "image/png"},
		{"gif", "test.gif", "image/gif"},
		{"webp", "test.webp", "image/webp"},
		{"mp4", "video.mp4", "video/mp4"},
		{"mp3", "audio.mp3", "audio/mpeg"},
		{"txt", "doc.txt", "text/plain"},
		{"json", "data.json", "application/json"},
		{"pdf", "file.pdf", "application/pdf"},
		{"zip", "file.zip", "application/zip"},
		{"html", "page.html", "text/html"},
		{"css", "style.css", "text/css"},
		{"js", "app.js", "application/javascript"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(t.TempDir(), tt.filename)
			err := os.WriteFile(tmpFile, []byte("test"), 0644)
			require.NoError(t, err)

			result := m.GetMimeType(tmpFile)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestManager_GetFileContent(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	t.Run("small file", func(t *testing.T) {
		content := "Hello, World!"
		tmpFile := filepath.Join(t.TempDir(), "test.txt")
		err := os.WriteFile(tmpFile, []byte(content), 0644)
		require.NoError(t, err)

		result, err := m.GetFileContent(tmpFile, 1024)
		require.NoError(t, err)
		assert.Equal(t, content, result)
	})

	t.Run("file too large", func(t *testing.T) {
		largeContent := make([]byte, 1000)
		for i := range largeContent {
			largeContent[i] = 'a'
		}
		tmpFile := filepath.Join(t.TempDir(), "large.txt")
		err := os.WriteFile(tmpFile, largeContent, 0644)
		require.NoError(t, err)

		_, err = m.GetFileContent(tmpFile, 100)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "过大")
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, err := m.GetFileContent("/nonexistent/file.txt", 1024)
		assert.Error(t, err)
	})
}

func TestManager_ListFiles(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	tmpDir := t.TempDir()

	// Create test files
	files := []string{"test.txt", "image.jpg", "subdir"}
	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		if f == "subdir" {
			os.Mkdir(path, 0755)
		} else {
			os.WriteFile(path, []byte("test"), 0644)
		}
	}

	result, err := m.ListFiles(tmpDir, false)
	require.NoError(t, err)
	assert.Len(t, result, 3)

	// Check that IsDir is set correctly
	for _, info := range result {
		if info.Name == "subdir" {
			assert.True(t, info.IsDir)
		} else {
			assert.False(t, info.IsDir)
		}
	}
}

func TestGenerateRandomToken(t *testing.T) {
	token1 := generateRandomToken(16)
	token2 := generateRandomToken(16)

	assert.Len(t, token1, 32) // hex encoding doubles length
	assert.Len(t, token2, 32)
	assert.NotEqual(t, token1, token2, "tokens should be unique")
}
