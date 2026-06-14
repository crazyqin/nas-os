package filepreview

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewPreviewCache(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024, // 1MB
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)

	if cache == nil {
		t.Fatal("NewPreviewCache returned nil")
	}

	if cache.config.CacheDir != tmpDir {
		t.Errorf("CacheDir = %q, want %q", cache.config.CacheDir, tmpDir)
	}

	cache.Close()
}

func TestPreviewCache_SetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)
	defer cache.Close()

	// 创建测试文件.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0o644)

	// 设置缓存.
	entry := &CacheEntry{
		Key:           "test_key",
		FilePath:      testFile,
		SourcePath:    "/source/file.txt",
		FileSize:      12,
		ContentType:   "text/plain",
		SourceModTime: time.Now(),
	}

	if err := cache.Set("test_key", entry); err != nil {
		t.Fatalf("Set() failed: %v", err)
	}

	// 获取缓存.
	got, err := cache.Get("test_key")
	if err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	if got.Key != "test_key" {
		t.Errorf("Key = %q, want %q", got.Key, "test_key")
	}

	if got.FilePath != testFile {
		t.Errorf("FilePath = %q, want %q", got.FilePath, testFile)
	}
}

func TestPreviewCache_Get_Miss(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)
	defer cache.Close()

	_, err := cache.Get("nonexistent_key")
	if err == nil {
		t.Error("Expected error for nonexistent key")
	}
}

func TestPreviewCache_Get_Expired(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)
	defer cache.Close()

	// 创建测试文件.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	// 设置已过期的缓存.
	expiredTime := time.Now().Add(-time.Hour)
	entry := &CacheEntry{
		Key:           "expired_key",
		FilePath:      testFile,
		SourcePath:    "/source/file.txt",
		FileSize:      4,
		ContentType:   "text/plain",
		ExpiresAt:     &expiredTime,
		SourceModTime: time.Now().Add(-2 * time.Hour),
	}

	cache.Set("expired_key", entry)

	_, err := cache.Get("expired_key")
	if err == nil {
		t.Error("Expected error for expired key")
	}
}

func TestPreviewCache_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)

	// 创建测试文件.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	entry := &CacheEntry{
		Key:           "delete_key",
		FilePath:      testFile,
		SourcePath:    "/source/file.txt",
		FileSize:      4,
		ContentType:   "text/plain",
		SourceModTime: time.Now(),
	}

	cache.Set("delete_key", entry)

	// 等待异步保存完成.
	time.Sleep(100 * time.Millisecond)

	// 删除.
	if err := cache.Delete("delete_key"); err != nil {
		t.Fatalf("Delete() failed: %v", err)
	}

	// 验证已删除.
	_, err := cache.Get("delete_key")
	if err == nil {
		t.Error("Expected error for deleted key")
	}

	// 关闭缓存并手动清理.
	cache.Close()
	os.RemoveAll(tmpDir)
}

func TestPreviewCache_DeleteBySource(t *testing.T) {
	tmpDir := t.TempDir()
	cacheDir := filepath.Join(tmpDir, "cache")
	os.MkdirAll(cacheDir, 0o755)

	config := CacheConfig{
		CacheDir:        cacheDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)

	// 创建测试文件.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	sourcePath := "/source/file.txt"

	// 设置多个缓存条目.
	for i := 0; i < 3; i++ {
		entry := &CacheEntry{
			Key:           "key_" + string(rune('a'+i)),
			FilePath:      testFile,
			SourcePath:    sourcePath,
			FileSize:      4,
			ContentType:   "text/plain",
			SourceModTime: time.Now(),
		}
		cache.Set(entry.Key, entry)
	}

	// 等待异步保存完成.
	time.Sleep(100 * time.Millisecond)

	// 删除源文件的所有缓存.
	if err := cache.DeleteBySource(sourcePath); err != nil {
		t.Fatalf("DeleteBySource() failed: %v", err)
	}

	// 验证已删除.
	stats := cache.GetStats()
	if stats.TotalEntries != 0 {
		t.Errorf("TotalEntries = %d, want 0", stats.TotalEntries)
	}

	// 关闭缓存并手动清理.
	cache.Close()
	os.RemoveAll(tmpDir)
}

func TestPreviewCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)
	defer cache.Close()

	// 创建测试文件.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	// 设置多个缓存.
	for i := 0; i < 5; i++ {
		entry := &CacheEntry{
			Key:           "key_" + string(rune('a'+i)),
			FilePath:      testFile,
			SourcePath:    "/source/file.txt",
			FileSize:      4,
			ContentType:   "text/plain",
			SourceModTime: time.Now(),
		}
		cache.Set(entry.Key, entry)
	}

	// 清空.
	if err := cache.Clear(); err != nil {
		t.Fatalf("Clear() failed: %v", err)
	}

	// 验证已清空.
	stats := cache.GetStats()
	if stats.TotalEntries != 0 {
		t.Errorf("TotalEntries = %d, want 0", stats.TotalEntries)
	}
}

func TestPreviewCache_GetStats(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)

	// 创建测试文件.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	entry := &CacheEntry{
		Key:           "stats_key",
		FilePath:      testFile,
		SourcePath:    "/source/file.txt",
		FileSize:      4,
		ContentType:   "text/plain",
		SourceModTime: time.Now(),
	}

	cache.Set("stats_key", entry)

	// 等待异步保存完成.
	time.Sleep(100 * time.Millisecond)

	stats := cache.GetStats()

	if stats.TotalEntries != 1 {
		t.Errorf("TotalEntries = %d, want 1", stats.TotalEntries)
	}

	if stats.TotalSize != 4 {
		t.Errorf("TotalSize = %d, want 4", stats.TotalSize)
	}

	if stats.MaxSize != 1024*1024 {
		t.Errorf("MaxSize = %d, want %d", stats.MaxSize, 1024*1024)
	}

	// 关闭缓存并手动清理.
	cache.Close()
	os.RemoveAll(tmpDir)
}

func TestPreviewCache_Has(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)
	defer cache.Close()

	// 创建测试文件.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	entry := &CacheEntry{
		Key:           "has_key",
		FilePath:      testFile,
		SourcePath:    "/source/file.txt",
		FileSize:      4,
		ContentType:   "text/plain",
		SourceModTime: time.Now(),
	}

	cache.Set("has_key", entry)

	if !cache.Has("has_key") {
		t.Error("Has() should return true for existing key")
	}

	if cache.Has("nonexistent") {
		t.Error("Has() should return false for nonexistent key")
	}
}

func TestPreviewCache_Count(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)

	if cache.Count() != 0 {
		t.Errorf("Count() = %d, want 0", cache.Count())
	}

	// 添加条目.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	entry := &CacheEntry{
		Key:           "count_key",
		FilePath:      testFile,
		SourcePath:    "/source/file.txt",
		FileSize:      4,
		ContentType:   "text/plain",
		SourceModTime: time.Now(),
	}

	cache.Set("count_key", entry)

	// 等待异步保存完成.
	time.Sleep(100 * time.Millisecond)

	if cache.Count() != 1 {
		t.Errorf("Count() = %d, want 1", cache.Count())
	}

	// 关闭缓存并手动清理.
	cache.Close()
	os.RemoveAll(tmpDir)
}

func TestPreviewCache_Size(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)
	defer cache.Close()

	if cache.Size() != 0 {
		t.Errorf("Size() = %d, want 0", cache.Size())
	}

	// 添加条目.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test content"), 0o644)

	entry := &CacheEntry{
		Key:           "size_key",
		FilePath:      testFile,
		SourcePath:    "/source/file.txt",
		FileSize:      12,
		ContentType:   "text/plain",
		SourceModTime: time.Now(),
	}

	cache.Set("size_key", entry)

	if cache.Size() != 12 {
		t.Errorf("Size() = %d, want 12", cache.Size())
	}
}

func TestPreviewCache_InvalidateSource(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)
	defer cache.Close()

	// 创建测试文件.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	sourcePath := "/source/file.txt"

	// 设置缓存.
	entry := &CacheEntry{
		Key:           "invalidate_key",
		FilePath:      testFile,
		SourcePath:    sourcePath,
		FileSize:      4,
		ContentType:   "text/plain",
		SourceModTime: time.Now(),
	}

	cache.Set("invalidate_key", entry)

	// 使失效.
	count := cache.InvalidateSource(sourcePath)
	if count != 1 {
		t.Errorf("InvalidateSource() count = %d, want 1", count)
	}

	// 验证已失效.
	if cache.Has("invalidate_key") {
		t.Error("Key should be invalidated")
	}
}

func TestPreviewCache_SetWithTTL(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)
	defer cache.Close()

	// 创建测试文件.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	entry := &CacheEntry{
		Key:           "ttl_key",
		FilePath:      testFile,
		SourcePath:    "/source/file.txt",
		FileSize:      4,
		ContentType:   "text/plain",
		SourceModTime: time.Now(),
	}

	// 设置短 TTL.
	if err := cache.SetWithTTL("ttl_key", entry, time.Second); err != nil {
		t.Fatalf("SetWithTTL() failed: %v", err)
	}

	// 立即获取应该成功.
	if _, err := cache.Get("ttl_key"); err != nil {
		t.Fatalf("Get() failed: %v", err)
	}

	// 等待过期.
	time.Sleep(2 * time.Second)

	_, err := cache.Get("ttl_key")
	if err == nil {
		t.Error("Expected error for expired key")
	}
}

func TestPreviewCache_Touch(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)
	defer cache.Close()

	// 创建测试文件.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	entry := &CacheEntry{
		Key:           "touch_key",
		FilePath:      testFile,
		SourcePath:    "/source/file.txt",
		FileSize:      4,
		ContentType:   "text/plain",
		SourceModTime: time.Now(),
	}

	cache.Set("touch_key", entry)

	// 获取初始访问时间.
	got, _ := cache.Get("touch_key")
	initialAccess := got.AccessedAt

	// 等待一小段时间.
	time.Sleep(10 * time.Millisecond)

	// Touch.
	if err := cache.Touch("touch_key"); err != nil {
		t.Fatalf("Touch() failed: %v", err)
	}

	// 验证访问时间已更新.
	got, _ = cache.Get("touch_key")
	if !got.AccessedAt.After(initialAccess) {
		t.Error("Touch() should update AccessedAt")
	}
}

func TestPreviewCache_ListEntries(t *testing.T) {
	tmpDir := t.TempDir()
	config := CacheConfig{
		CacheDir:        tmpDir,
		MaxSize:         1024 * 1024,
		MaxEntries:      100,
		DefaultTTL:      time.Hour,
		CleanupInterval: time.Minute,
	}

	cache := NewPreviewCache(config)

	// 创建测试文件.
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0o644)

	// 设置多个缓存.
	for i := 0; i < 3; i++ {
		entry := &CacheEntry{
			Key:           "list_key_" + string(rune('a'+i)),
			FilePath:      testFile,
			SourcePath:    "/source/file.txt",
			FileSize:      4,
			ContentType:   "text/plain",
			SourceModTime: time.Now(),
		}
		cache.Set(entry.Key, entry)
	}

	// 等待异步保存完成.
	time.Sleep(100 * time.Millisecond)

	entries := cache.ListEntries()
	if len(entries) != 3 {
		t.Errorf("ListEntries() length = %d, want 3", len(entries))
	}

	// 关闭缓存并手动清理.
	cache.Close()
	os.RemoveAll(tmpDir)
}
