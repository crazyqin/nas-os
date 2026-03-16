package files

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
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

func TestManager_PreviewFile(t *testing.T) {
	m := NewManager(PreviewConfig{
		CacheDir:       t.TempDir(),
		MaxPreviewSize: 10 * 1024 * 1024,
	})

	t.Run("small file", func(t *testing.T) {
		content := "Hello, World!"
		tmpFile := filepath.Join(t.TempDir(), "test.txt")
		err := os.WriteFile(tmpFile, []byte(content), 0644)
		require.NoError(t, err)

		reader, mimeType, err := m.PreviewFile(tmpFile)
		require.NoError(t, err)
		defer reader.Close()
		assert.Equal(t, "text/plain", mimeType)
	})

	t.Run("file too large", func(t *testing.T) {
		tmpDir := t.TempDir()
		largeFile := filepath.Join(tmpDir, "large.txt")
		largeContent := make([]byte, 20*1024*1024) // 20MB
		err := os.WriteFile(largeFile, largeContent, 0644)
		require.NoError(t, err)

		// Create manager with small limit
		smallMgr := NewManager(PreviewConfig{
			CacheDir:       t.TempDir(),
			MaxPreviewSize: 1 * 1024 * 1024, // 1MB
		})

		_, _, err = smallMgr.PreviewFile(largeFile)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "过大")
	})

	t.Run("non-existent file", func(t *testing.T) {
		_, _, err := m.PreviewFile("/nonexistent/file.txt")
		assert.Error(t, err)
	})
}

func TestManager_DefaultConfig(t *testing.T) {
	// Test that default values are applied
	m := NewManager(PreviewConfig{})

	assert.Equal(t, uint(256), m.config.ThumbnailSize)
	assert.Equal(t, int64(50*1024*1024), m.config.MaxPreviewSize)
	assert.NotEmpty(t, m.config.CacheDir)
	assert.Equal(t, 24*time.Hour, m.config.CacheExpiry)
}

func TestManager_ImageTypes(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg", ".ico", ".tiff", ".tif", ".heic", ".heif"}
	for _, ext := range imageExts {
		assert.True(t, m.imageTypes[ext], "Expected %s to be an image type", ext)
	}
}

func TestManager_VideoTypes(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	videoExts := []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".mpeg", ".mpg", ".3gp"}
	for _, ext := range videoExts {
		assert.True(t, m.videoTypes[ext], "Expected %s to be a video type", ext)
	}
}

func TestManager_AudioTypes(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	audioExts := []string{".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a", ".ape"}
	for _, ext := range audioExts {
		assert.True(t, m.audioTypes[ext], "Expected %s to be an audio type", ext)
	}
}

func TestManager_CodeTypes(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	codeExts := []string{".js", ".ts", ".py", ".go", ".java", ".c", ".cpp", ".h", ".html", ".css", ".json", ".xml", ".yaml", ".yml", ".md", ".sh", ".sql", ".php", ".rb", ".rs"}
	for _, ext := range codeExts {
		assert.True(t, m.codeTypes[ext], "Expected %s to be a code type", ext)
	}
}

func TestManager_ListFiles_EmptyDir(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	emptyDir := t.TempDir()
	result, err := m.ListFiles(emptyDir, false)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestManager_ListFiles_NonExistent(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	_, err := m.ListFiles("/nonexistent/directory", false)
	assert.Error(t, err)
}

func TestManager_GetMimeType_Unknown(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	tmpFile := filepath.Join(t.TempDir(), "unknown.xyz")
	err := os.WriteFile(tmpFile, []byte("test"), 0644)
	require.NoError(t, err)

	result := m.GetMimeType(tmpFile)
	assert.Equal(t, "application/octet-stream", result)
}

func TestNewHandlers(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})
	h := NewHandlers(m)
	require.NotNil(t, h)
	assert.Equal(t, m, h.manager)
}

func TestManager_ArchiveTypes(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	tests := []struct {
		filename string
		expected FileType
	}{
		{"file.zip", FileTypeArchive},
		{"file.rar", FileTypeArchive},
		{"file.7z", FileTypeArchive},
		{"file.tar", FileTypeArchive},
		{"file.gz", FileTypeArchive},
		{"file.tar.gz", FileTypeArchive},
	}

	for _, tt := range tests {
		tmpFile := filepath.Join(t.TempDir(), tt.filename)
		err := os.WriteFile(tmpFile, []byte("test"), 0644)
		require.NoError(t, err)

		result := m.GetFileType(tmpFile)
		assert.Equal(t, tt.expected, result, "filename: %s", tt.filename)
	}
}

func TestManager_GenerateImageThumbnail(t *testing.T) {
	m := NewManager(PreviewConfig{
		CacheDir:       t.TempDir(),
		ThumbnailSize:  128,
	})

	t.Run("jpeg image", func(t *testing.T) {
		// 创建一个简单的测试图片
		tmpFile := filepath.Join(t.TempDir(), "test.jpg")
		imgData := createTestJPEG(200, 200)
		err := os.WriteFile(tmpFile, imgData, 0644)
		require.NoError(t, err)

		thumb, w, h := m.GenerateImageThumbnail(tmpFile)
		assert.NotEmpty(t, thumb)
		assert.Contains(t, thumb, "data:image/jpeg;base64,")
		assert.Equal(t, 200, w)
		assert.Equal(t, 200, h)
	})

	t.Run("png image", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "test.png")
		imgData := createTestPNG(100, 100)
		err := os.WriteFile(tmpFile, imgData, 0644)
		require.NoError(t, err)

		thumb, w, h := m.GenerateImageThumbnail(tmpFile)
		assert.NotEmpty(t, thumb)
		assert.Contains(t, thumb, "data:image/jpeg;base64,")
		assert.Equal(t, 100, w)
		assert.Equal(t, 100, h)
	})

	t.Run("non-existent file", func(t *testing.T) {
		thumb, w, h := m.GenerateImageThumbnail("/nonexistent/image.jpg")
		assert.Empty(t, thumb)
		assert.Equal(t, 0, w)
		assert.Equal(t, 0, h)
	})

	t.Run("cached thumbnail", func(t *testing.T) {
		tmpFile := filepath.Join(t.TempDir(), "cached.jpg")
		imgData := createTestJPEG(50, 50)
		err := os.WriteFile(tmpFile, imgData, 0644)
		require.NoError(t, err)

		// 第一次调用
		thumb1, _, _ := m.GenerateImageThumbnail(tmpFile)
		// 第二次调用应该从缓存获取
		thumb2, _, _ := m.GenerateImageThumbnail(tmpFile)
		assert.Equal(t, thumb1, thumb2)
	})
}

func TestManager_ListFiles_WithSubdirs(t *testing.T) {
	m := NewManager(PreviewConfig{CacheDir: t.TempDir()})

	tmpDir := t.TempDir()

	// 创建目录结构
	os.Mkdir(filepath.Join(tmpDir, "subdir1"), 0755)
	os.Mkdir(filepath.Join(tmpDir, "subdir2"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "subdir1", "file2.txt"), []byte("test"), 0644)

	t.Run("non-recursive", func(t *testing.T) {
		result, err := m.ListFiles(tmpDir, false)
		require.NoError(t, err)
		assert.Len(t, result, 3) // subdir1, subdir2, file1.txt
	})

	t.Run("with thumbnails", func(t *testing.T) {
		// 创建测试图片
		imgFile := filepath.Join(tmpDir, "test.jpg")
		imgData := createTestJPEG(50, 50)
		err := os.WriteFile(imgFile, imgData, 0644)
		require.NoError(t, err)

		result, err := m.ListFiles(tmpDir, true)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(result), 4)

		// 检查图片是否有缩略图
		for _, f := range result {
			if f.Name == "test.jpg" {
				assert.NotEmpty(t, f.Thumbnail)
				assert.Greater(t, f.Width, 0)
				assert.Greater(t, f.Height, 0)
			}
		}
	})
}

func TestShareStore_Operations(t *testing.T) {
	// 清理 share store
	shareStore.Lock()
	shareStore.shares = make(map[string]*ShareInfo)
	shareStore.Unlock()

	// 测试创建分享
	token := generateRandomToken(16)
	share := &ShareInfo{
		Token:         token,
		Path:          "/test/path",
		Password:      "secret",
		Expiry:        time.Now().Add(24 * time.Hour),
		AllowDownload: true,
		CreatedAt:     time.Now(),
		Downloads:     0,
	}

	shareStore.Lock()
	shareStore.shares[token] = share
	shareStore.Unlock()

	// 验证可以读取
	shareStore.RLock()
	retrieved, exists := shareStore.shares[token]
	shareStore.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, "/test/path", retrieved.Path)
	assert.Equal(t, "secret", retrieved.Password)

	// 测试删除
	shareStore.Lock()
	delete(shareStore.shares, token)
	shareStore.Unlock()

	shareStore.RLock()
	_, exists = shareStore.shares[token]
	shareStore.RUnlock()

	assert.False(t, exists)
}

func TestGenerateRandomToken_Uniqueness(t *testing.T) {
	tokens := make(map[string]bool)
	for i := 0; i < 100; i++ {
		token := generateRandomToken(16)
		assert.NotEmpty(t, token)
		assert.False(t, tokens[token], "token should be unique")
		tokens[token] = true
	}
}

func TestGenerateRandomToken_Length(t *testing.T) {
	for length := 1; length <= 32; length++ {
		token := generateRandomToken(length)
		assert.Len(t, token, length*2, "hex encoding doubles length")
	}
}

// createTestJPEG 创建测试用的 JPEG 图片数据
func createTestJPEG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	var buf bytes.Buffer
	jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

// createTestPNG 创建测试用的 PNG 图片数据
func createTestPNG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{R: 0, G: 255, B: 0, A: 255})
		}
	}

	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
