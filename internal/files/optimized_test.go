package files

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestFileListCache_Basic(t *testing.T) {
	cache := NewFileListCache(10, time.Minute)

	files := []FileInfo{
		{Name: "test.txt", Path: "/test/test.txt", Size: 100},
		{Name: "image.jpg", Path: "/test/image.jpg", Size: 500},
	}

	// Test Set and Get
	cache.Set("/test", files)

	result, ok := cache.Get("/test")
	require.True(t, ok)
	assert.Len(t, result.Files, 2)
	assert.Equal(t, "/test", result.Path)

	// Test cache miss
	_, ok = cache.Get("/nonexistent")
	assert.False(t, ok)
}

func TestFileListCache_Expiry(t *testing.T) {
	cache := NewFileListCache(10, 50*time.Millisecond)

	files := []FileInfo{{Name: "test.txt"}}
	cache.Set("/test", files)

	// Should exist immediately
	_, ok := cache.Get("/test")
	assert.True(t, ok)

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, ok = cache.Get("/test")
	assert.False(t, ok)
}

func TestFileListCache_Delete(t *testing.T) {
	cache := NewFileListCache(10, time.Minute)

	files := []FileInfo{{Name: "test.txt"}}
	cache.Set("/test", files)

	cache.Delete("/test")

	_, ok := cache.Get("/test")
	assert.False(t, ok)
}

func TestFileListCache_Invalidate(t *testing.T) {
	cache := NewFileListCache(10, time.Minute)

	// Add multiple entries
	cache.Set("/parent", []FileInfo{{Name: "parent.txt"}})
	cache.Set("/parent/child1", []FileInfo{{Name: "child1.txt"}})
	cache.Set("/parent/child2", []FileInfo{{Name: "child2.txt"}})
	cache.Set("/other", []FileInfo{{Name: "other.txt"}})

	// Invalidate parent and children
	cache.Invalidate("/parent")

	_, ok := cache.Get("/parent")
	assert.False(t, ok)
	_, ok = cache.Get("/parent/child1")
	assert.False(t, ok)
	_, ok = cache.Get("/parent/child2")
	assert.False(t, ok)

	// Other should still exist
	_, ok = cache.Get("/other")
	assert.True(t, ok)
}

func TestFileListCache_Stats(t *testing.T) {
	cache := NewFileListCache(10, time.Minute)

	// Miss
	_, ok := cache.Get("/test")
	assert.False(t, ok)

	// Set
	cache.Set("/test", []FileInfo{{Name: "test.txt"}})

	// Hit
	_, ok = cache.Get("/test")
	assert.True(t, ok)

	hits, misses, evictions, size := cache.Stats()
	assert.Equal(t, int64(1), hits)
	assert.Equal(t, int64(1), misses)
	assert.Equal(t, int64(0), evictions)
	assert.Equal(t, 1, size)
}

func TestFileListCache_Eviction(t *testing.T) {
	cache := NewFileListCache(3, time.Minute)

	// Add more than capacity
	cache.Set("/test1", []FileInfo{{Name: "test1.txt"}})
	cache.Set("/test2", []FileInfo{{Name: "test2.txt"}})
	cache.Set("/test3", []FileInfo{{Name: "test3.txt"}})
	cache.Set("/test4", []FileInfo{{Name: "test4.txt"}})

	_, _, evictions, size := cache.Stats()
	assert.Equal(t, int64(1), evictions)
	assert.Equal(t, 3, size)
}

func TestThumbnailCache_Basic(t *testing.T) {
	cache := NewThumbnailCache(10, 1024*1024)

	tmpFile := filepath.Join(t.TempDir(), "test.jpg")
	err := os.WriteFile(tmpFile, []byte("test"), 0644)
	require.NoError(t, err)
	info, err := os.Stat(tmpFile)
	require.NoError(t, err)

	// Test Set and Get
	cache.Set(tmpFile, int(info.Size()), info.ModTime(), "base64thumb", 100, 100)

	entry, ok := cache.Get(tmpFile, int(info.Size()), info.ModTime())
	require.True(t, ok)
	assert.Equal(t, "base64thumb", entry.Thumbnail)
	assert.Equal(t, 100, entry.Width)
	assert.Equal(t, 100, entry.Height)
}

func TestThumbnailCache_Miss(t *testing.T) {
	cache := NewThumbnailCache(10, 1024*1024)

	_, ok := cache.Get("/nonexistent", 100, time.Now())
	assert.False(t, ok)
}

func TestThumbnailCache_ModTimeChange(t *testing.T) {
	cache := NewThumbnailCache(10, 1024*1024)

	// Set with one modtime
	cache.Set("/test", 100, time.Now(), "thumb", 50, 50)

	// Get with different modtime should miss
	_, ok := cache.Get("/test", 100, time.Now().Add(time.Hour))
	assert.False(t, ok)
}

func TestThumbnailCache_SizeChange(t *testing.T) {
	cache := NewThumbnailCache(10, 1024*1024)

	modTime := time.Now()
	cache.Set("/test", 100, modTime, "thumb", 50, 50)

	// Get with different size should miss
	_, ok := cache.Get("/test", 200, modTime)
	assert.False(t, ok)
}

func TestThumbnailCache_Stats(t *testing.T) {
	cache := NewThumbnailCache(10, 1024*1024)

	// Miss
	cache.Get("/test", 100, time.Now())

	// Set - use same modTime for consistency
	modTime := time.Now()
	cache.Set("/test", 100, modTime, "thumb", 50, 50)

	// Hit - must use same modTime to match
	cache.Get("/test", 100, modTime)

	hits, misses, evictions, size, bytes := cache.Stats()
	assert.Equal(t, int64(1), hits)
	assert.Equal(t, int64(1), misses)
	assert.Equal(t, int64(0), evictions)
	assert.Equal(t, 1, size)
	assert.Greater(t, bytes, int64(0))
}

func TestThumbnailCache_ByteLimit(t *testing.T) {
	// Very small byte limit
	cache := NewThumbnailCache(10, 20)

	// Add entries that exceed byte limit
	cache.Set("/test1", 100, time.Now(), "thumb12345678901234567890", 50, 50)
	cache.Set("/test2", 100, time.Now(), "thumb12345678901234567890", 50, 50)

	stats := cache.Stats()
	assert.Greater(t, stats.Evictions, int64(0))
}

func TestOptimizedManager_New(t *testing.T) {
	config := PreviewConfig{CacheDir: t.TempDir()}
	logger := zap.NewNop()

	om := NewOptimizedManager(config, logger)
	require.NotNil(t, om)
	assert.NotNil(t, om.fileListCache)
	assert.NotNil(t, om.thumbnailCache)

	// Cleanup
	om.Close()
}

func TestOptimizedManager_ListFilesCached(t *testing.T) {
	config := PreviewConfig{CacheDir: t.TempDir()}
	logger := zap.NewNop()
	om := NewOptimizedManager(config, logger)
	defer om.Close()

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)

	// First call
	files, err := om.ListFilesCached(tmpDir, false)
	require.NoError(t, err)
	assert.Len(t, files, 1)

	// Second call should use cache
	files2, err := om.ListFilesCached(tmpDir, false)
	require.NoError(t, err)
	assert.Equal(t, files, files2)
}

func TestOptimizedManager_InvalidateCache(t *testing.T) {
	config := PreviewConfig{CacheDir: t.TempDir()}
	logger := zap.NewNop()
	om := NewOptimizedManager(config, logger)
	defer om.Close()

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)

	// Cache it
	om.ListFilesCached(tmpDir, false)

	// Invalidate
	om.InvalidateCache(tmpDir)

	// Check cache is empty
	_, ok := om.fileListCache.Get(tmpDir)
	assert.False(t, ok)
}

func TestOptimizedManager_GetCacheStats(t *testing.T) {
	config := PreviewConfig{CacheDir: t.TempDir()}
	logger := zap.NewNop()
	om := NewOptimizedManager(config, logger)
	defer om.Close()

	stats := om.GetCacheStats()
	require.NotNil(t, stats)

	_, ok := stats["fileList"].(map[string]interface{})
	assert.True(t, ok)

	_, ok = stats["thumbnail"].(map[string]interface{})
	assert.True(t, ok)
}

func TestSearchCache_Basic(t *testing.T) {
	cache := NewSearchCache(10, time.Minute)

	result := map[string]string{"file": "test.txt"}

	// Set and Get
	cache.Set("test query", result)

	retrieved, ok := cache.Get("test query")
	require.True(t, ok)
	assert.Equal(t, result, retrieved)
}

func TestSearchCache_Miss(t *testing.T) {
	cache := NewSearchCache(10, time.Minute)

	_, ok := cache.Get("nonexistent")
	assert.False(t, ok)
}

func TestSearchCache_Expiry(t *testing.T) {
	cache := NewSearchCache(10, 50*time.Millisecond)

	cache.Set("query", "result")

	// Should exist
	_, ok := cache.Get("query")
	assert.True(t, ok)

	// Wait for expiry
	time.Sleep(100 * time.Millisecond)

	// Should be expired
	_, ok = cache.Get("query")
	assert.False(t, ok)
}

func TestSearchCache_Eviction(t *testing.T) {
	cache := NewSearchCache(3, time.Minute)

	// Add more than capacity
	cache.Set("query1", "result1")
	cache.Set("query2", "result2")
	cache.Set("query3", "result3")
	cache.Set("query4", "result4")

	// Oldest should be evicted
	_, ok := cache.Get("query1")
	assert.False(t, ok)

	// Newest should exist
	_, ok = cache.Get("query4")
	assert.True(t, ok)
}

func TestHashQuery(t *testing.T) {
	hash1 := hashQuery("test query")
	hash2 := hashQuery("test query")
	hash3 := hashQuery("different query")

	// Same query should produce same hash
	assert.Equal(t, hash1, hash2)
	// Different queries should produce different hashes
	assert.NotEqual(t, hash1, hash3)
	// Hash should be 16 chars (truncated SHA256)
	assert.Len(t, hash1, 16)
}

func TestBatchDelete(t *testing.T) {
	config := PreviewConfig{CacheDir: t.TempDir()}
	logger := zap.NewNop()
	om := NewOptimizedManager(config, logger)
	defer om.Close()

	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	file3 := filepath.Join(tmpDir, "file3.txt")
	os.WriteFile(file1, []byte("test"), 0644)
	os.WriteFile(file2, []byte("test"), 0644)
	// file3 doesn't exist

	result := om.BatchDelete([]string{file1, file2, file3})

	assert.Len(t, result.Success, 2)
	assert.Len(t, result.Failed, 1)
	assert.Equal(t, file3, result.Failed[0].Path)
}

func TestBatchRename(t *testing.T) {
	config := PreviewConfig{CacheDir: t.TempDir()}
	logger := zap.NewNop()
	om := NewOptimizedManager(config, logger)
	defer om.Close()

	tmpDir := t.TempDir()

	// Create test files
	file1 := filepath.Join(tmpDir, "old1.txt")
	file2 := filepath.Join(tmpDir, "old2.txt")
	os.WriteFile(file1, []byte("test"), 0644)
	os.WriteFile(file2, []byte("test"), 0644)

	renames := map[string]string{
		file1: filepath.Join(tmpDir, "new1.txt"),
		file2: filepath.Join(tmpDir, "new2.txt"),
	}

	result := om.BatchRename(renames)

	assert.Len(t, result.Success, 2)
	assert.Len(t, result.Failed, 0)

	// Verify files were renamed
	assert.FileExists(t, filepath.Join(tmpDir, "new1.txt"))
	assert.FileExists(t, filepath.Join(tmpDir, "new2.txt"))
	assert.NoFileExists(t, file1)
	assert.NoFileExists(t, file2)
}

func TestSortFiles(t *testing.T) {
	files := []FileInfo{
		{Name: "c.txt", Size: 100, ModTime: "2024-01-01", Type: FileTypeDocument},
		{Name: "a.txt", Size: 300, ModTime: "2024-01-03", Type: FileTypeCode},
		{Name: "b.txt", Size: 200, ModTime: "2024-01-02", Type: FileTypeImage},
	}

	// Sort by name
	SortFiles(files, "name", false)
	assert.Equal(t, "a.txt", files[0].Name)
	assert.Equal(t, "b.txt", files[1].Name)
	assert.Equal(t, "c.txt", files[2].Name)

	// Sort by name descending
	SortFiles(files, "name", true)
	assert.Equal(t, "c.txt", files[0].Name)

	// Sort by size
	SortFiles(files, "size", false)
	assert.Equal(t, "c.txt", files[0].Name)
	assert.Equal(t, "b.txt", files[1].Name)
	assert.Equal(t, "a.txt", files[2].Name)

	// Sort by modTime
	SortFiles(files, "modTime", false)
	assert.Equal(t, "c.txt", files[0].Name)
	assert.Equal(t, "b.txt", files[1].Name)
	assert.Equal(t, "a.txt", files[2].Name)

	// Sort by type
	SortFiles(files, "type", false)
	assert.Equal(t, FileTypeCode, files[0].Type)
	assert.Equal(t, FileTypeDocument, files[1].Type)
	assert.Equal(t, FileTypeImage, files[2].Type)
}

func TestSortFiles_DirectoriesFirst(t *testing.T) {
	files := []FileInfo{
		{Name: "file.txt", IsDir: false},
		{Name: "subdir", IsDir: true},
		{Name: "another", IsDir: false},
	}

	SortFiles(files, "", false)

	// Directories should come first
	assert.True(t, files[0].IsDir)
}

func TestFilterFiles(t *testing.T) {
	files := []FileInfo{
		{Name: "image.jpg", Type: FileTypeImage, Size: 100},
		{Name: "doc.txt", Type: FileTypeDocument, Size: 50},
		{Name: "video.mp4", Type: FileTypeVideo, Size: 1000},
		{Name: "large.bin", Type: FileTypeOther, Size: 500},
	}

	// Filter by type
	result := FilterFiles(files, FileFilter{Types: []string{"image"}})
	assert.Len(t, result, 1)
	assert.Equal(t, "image.jpg", result[0].Name)

	// Filter by size range
	result = FilterFiles(files, FileFilter{MinSize: 100, MaxSize: 500})
	assert.Len(t, result, 2)

	// Filter by name pattern
	result = FilterFiles(files, FileFilter{NamePattern: "*.jpg"})
	assert.Len(t, result, 1)
	assert.Equal(t, "image.jpg", result[0].Name)

	// Combined filter
	result = FilterFiles(files, FileFilter{
		Types:   []string{"image", "video"},
		MinSize: 100,
	})
	assert.Len(t, result, 2)
}

func TestFileFilter_Match(t *testing.T) {
	tests := []struct {
		name   string
		filter FileFilter
		file   FileInfo
		want   bool
	}{
		{
			name:   "match type",
			filter: FileFilter{Types: []string{"image"}},
			file:   FileInfo{Type: FileTypeImage},
			want:   true,
		},
		{
			name:   "no match type",
			filter: FileFilter{Types: []string{"image"}},
			file:   FileInfo{Type: FileTypeVideo},
			want:   false,
		},
		{
			name:   "match name pattern",
			filter: FileFilter{NamePattern: "*.txt"},
			file:   FileInfo{Name: "file.txt"},
			want:   true,
		},
		{
			name:   "no match name pattern",
			filter: FileFilter{NamePattern: "*.txt"},
			file:   FileInfo{Name: "file.jpg"},
			want:   false,
		},
		{
			name:   "match min size",
			filter: FileFilter{MinSize: 100},
			file:   FileInfo{Size: 200},
			want:   true,
		},
		{
			name:   "below min size",
			filter: FileFilter{MinSize: 100},
			file:   FileInfo{Size: 50},
			want:   false,
		},
		{
			name:   "match max size",
			filter: FileFilter{MaxSize: 100},
			file:   FileInfo{Size: 50},
			want:   true,
		},
		{
			name:   "above max size",
			filter: FileFilter{MaxSize: 100},
			file:   FileInfo{Size: 200},
			want:   false,
		},
		{
			name:   "empty filter matches all",
			filter: FileFilter{},
			file:   FileInfo{Name: "anything", Type: FileTypeOther, Size: 999},
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.filter.Match(tt.file)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFileInfo_ToJSON(t *testing.T) {
	info := FileInfo{
		Name:     "test.txt",
		Path:     "/test/test.txt",
		Size:     1024,
		IsDir:    false,
		Type:     FileTypeDocument,
		MimeType: "text/plain",
	}

	json := info.ToJSON()
	assert.Contains(t, json, `"name": "test.txt"`)
	assert.Contains(t, json, `"path": "/test/test.txt"`)
	assert.Contains(t, json, `"size": 1024`)
}

func TestFileInfo_FromJSON(t *testing.T) {
	json := `{
		"name": "test.txt",
		"path": "/test/test.txt",
		"size": 1024,
		"isDir": false,
		"type": "document",
		"mimeType": "text/plain"
	}`

	info, err := FileInfoFromJSON(json)
	require.NoError(t, err)
	assert.Equal(t, "test.txt", info.Name)
	assert.Equal(t, "/test/test.txt", info.Path)
	assert.Equal(t, int64(1024), info.Size)
	assert.Equal(t, FileTypeDocument, info.Type)
}

func TestFileInfo_FromJSON_Invalid(t *testing.T) {
	_, err := FileInfoFromJSON("invalid json")
	assert.Error(t, err)
}

func TestGetThumbnailCached(t *testing.T) {
	config := PreviewConfig{
		CacheDir:      t.TempDir(),
		ThumbnailSize: 128,
	}
	logger := zap.NewNop()
	om := NewOptimizedManager(config, logger)
	defer om.Close()

	// Create test image
	tmpFile := filepath.Join(t.TempDir(), "test.jpg")
	imgData := createTestJPEG(100, 100)
	err := os.WriteFile(tmpFile, imgData, 0644)
	require.NoError(t, err)

	// First call generates thumbnail
	thumb1, w1, h1 := om.GetThumbnailCached(tmpFile, 128)
	assert.NotEmpty(t, thumb1)
	assert.Equal(t, 100, w1)
	assert.Equal(t, 100, h1)

	// Second call should use cache
	thumb2, w2, h2 := om.GetThumbnailCached(tmpFile, 128)
	assert.Equal(t, thumb1, thumb2)
	assert.Equal(t, w1, w2)
	assert.Equal(t, h1, h2)
}
