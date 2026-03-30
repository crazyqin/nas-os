// Package webshare WebShare 后端测试
package webshare

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	// 创建临时测试目录
	tmpDir, err := os.MkdirTemp("", "webshare-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := WebShareConfig{
		BaseDir:       tmpDir,
		MaxFileSize:   100 * 1024 * 1024,
		CacheDir:      filepath.Join(tmpDir, "cache"),
		ShareLinkExpiry: 24 * time.Hour,
	}

	manager := NewManager(config)
	if manager == nil {
		t.Fatal("创建管理器失败")
	}

	// 验证配置
	if manager.config.BaseDir != tmpDir {
		t.Errorf("BaseDir 不匹配")
	}

	if manager.shareLinks == nil {
		t.Error("shareLinks 未初始化")
	}
}

func TestListDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webshare-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件结构
	os.Mkdir(filepath.Join(tmpDir, "subdir1"), 0755)
	os.Mkdir(filepath.Join(tmpDir, "subdir2"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.jpg"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	manager := NewManager(config)

	// 测试列表（不显示隐藏文件）
	info, err := manager.ListDirectory("/", false, "name", false)
	if err != nil {
		t.Fatalf("列出目录失败: %v", err)
	}

	// 应有 2 个目录 + 2 个文件 + cache 目录 = 5 个项（不含隐藏文件）
	// cache 目录会被 NewManager 自动创建
	expectedItems := 5 // subdir1, subdir2, file1.txt, file2.jpg, cache
	if len(info.Items) != expectedItems {
		t.Errorf("项目数量不匹配: got %d, want %d", len(info.Items), expectedItems)
	}

	// 测试列表（显示隐藏文件）
	infoHidden, err := manager.ListDirectory("/", true, "name", false)
	if err != nil {
		t.Fatalf("列出目录失败: %v", err)
	}

	// 应有 2 个目录 + 3 个文件 + cache 目录 = 6 个项（含隐藏文件）
	expectedHidden := 6 // subdir1, subdir2, file1.txt, file2.jpg, .hidden, cache
	if len(infoHidden.Items) != expectedHidden {
		t.Errorf("项目数量不匹配: got %d, want %d", len(infoHidden.Items), expectedHidden)
	}

	// 验证面包屑
	if len(info.Breadcrumb) != 1 {
		t.Errorf("面包屑数量不匹配: got %d, want 1", len(info.Breadcrumb))
	}
}

func TestCreateDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webshare-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	manager := NewManager(config)

	// 创建目录
	err = manager.CreateDirectory("/newdir")
	if err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}

	// 验证目录存在
	if _, err := os.Stat(filepath.Join(tmpDir, "newdir")); err != nil {
		t.Errorf("目录不存在: %v", err)
	}
}

func TestDeleteFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webshare-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	manager := NewManager(config)

	// 删除文件
	err = manager.DeleteFile("/test.txt")
	if err != nil {
		t.Fatalf("删除文件失败: %v", err)
	}

	// 验证文件不存在
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("文件应该已删除")
	}
}

func TestShareLink(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webshare-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "share.txt"), []byte("share content"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
		ShareLinkExpiry: 24 * time.Hour,
	}

	manager := NewManager(config)

	// 创建分享链接
	link, err := manager.CreateShareLink("/share.txt", "", 24, false, false, "testuser")
	if err != nil {
		t.Fatalf("创建分享链接失败: %v", err)
	}

	// 验证链接
	if link.Token == "" {
		t.Error("Token 应不为空")
	}

	if link.CreatedBy != "testuser" {
		t.Errorf("CreatedBy 不匹配: got %s, want testuser", link.CreatedBy)
	}

	// 获取分享链接
	retrieved, err := manager.GetShareLink(link.Token)
	if err != nil {
		t.Fatalf("获取分享链接失败: %v", err)
	}

	if retrieved.Token != link.Token {
		t.Error("Token 不匹配")
	}

	// 删除分享链接
	err = manager.DeleteShareLink(link.Token)
	if err != nil {
		t.Fatalf("删除分享链接失败: %v", err)
	}

	// 验证已删除
	_, err = manager.GetShareLink(link.Token)
	if err == nil {
		t.Error("链接应该已删除")
	}
}

func TestShareLinkWithPassword(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webshare-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "protected.txt"), []byte("protected"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	manager := NewManager(config)

	// 创建带密码的分享链接
	link, err := manager.CreateShareLink("/protected.txt", "secret123", 24, false, false, "testuser")
	if err != nil {
		t.Fatalf("创建分享链接失败: %v", err)
	}

	if link.Password != "secret123" {
		t.Error("密码不匹配")
	}

	if link.IsPublic {
		t.Error("带密码的链接不应是公开的")
	}
}

func TestMoveFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webshare-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "source.txt"), []byte("content"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "dest"), 0755)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	manager := NewManager(config)

	// 移动文件
	err = manager.MoveFile("/source.txt", "/dest/moved.txt")
	if err != nil {
		t.Fatalf("移动文件失败: %v", err)
	}

	// 验证源文件不存在
	if _, err := os.Stat(filepath.Join(tmpDir, "source.txt")); !os.IsNotExist(err) {
		t.Error("源文件应该已移动")
	}

	// 验证目标文件存在
	if _, err := os.Stat(filepath.Join(tmpDir, "dest", "moved.txt")); err != nil {
		t.Error("目标文件应存在")
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webshare-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "original.txt"), []byte("content"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	manager := NewManager(config)

	// 复制文件
	err = manager.CopyFile("/original.txt", "/copy.txt")
	if err != nil {
		t.Fatalf("复制文件失败: %v", err)
	}

	// 验证源文件仍存在
	if _, err := os.Stat(filepath.Join(tmpDir, "original.txt")); err != nil {
		t.Error("源文件应仍存在")
	}

	// 验证副本存在
	if _, err := os.Stat(filepath.Join(tmpDir, "copy.txt")); err != nil {
		t.Error("副本应存在")
	}
}

func TestSnapshot(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webshare-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "snap.txt"), []byte("snapshot test"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	manager := NewManager(config)

	// 添加快照
	snap, err := manager.AddSnapshot("/", "initial", "初始快照")
	if err != nil {
		t.Fatalf("添加快照失败: %v", err)
	}

	if snap.Name != "initial" {
		t.Errorf("快照名称不匹配")
	}

	// 获取快照列表
	snaps, err := manager.GetSnapshots("/")
	if err != nil {
		t.Fatalf("获取快照列表失败: %v", err)
	}

	if len(snaps) != 1 {
		t.Errorf("快照数量不匹配: got %d, want 1", len(snaps))
	}
}

func TestSanitizePath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "webshare-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	config := WebShareConfig{
		BaseDir:  tmpDir,
	}

	manager := NewManager(config)

	// 正常路径
	normal, err := manager.sanitizePath("/subdir/file.txt")
	if err != nil {
		t.Errorf("正常路径验证失败: %v", err)
	}

	if !filepath.IsAbs(normal) {
		t.Error("结果应该是绝对路径")
	}

	// 路径遍历攻击
	_, err = manager.sanitizePath("../../../etc/passwd")
	if err == nil {
		t.Error("路径遍历攻击应该被检测")
	}
}

func TestFileManager_GetMimeType(t *testing.T) {
	fm := NewFileManager(WebShareConfig{})

	tests := []struct {
		filename string
		expected string
	}{
		{"test.jpg", "image/jpeg"},
		{"test.png", "image/png"},
		{"test.mp4", "video/mp4"},
		{"test.mp3", "audio/mpeg"},
		{"test.pdf", "application/pdf"},
		{"test.txt", "text/plain"},
		{"unknown.xyz", "application/octet-stream"},
	}

	for _, test := range tests {
		result := fm.GetMimeType(test.filename)
		if result != test.expected {
			t.Errorf("MIME 类型不匹配: %s -> got %s, want %s", test.filename, result, test.expected)
		}
	}
}

func TestFileManager_IsImage(t *testing.T) {
	fm := NewFileManager(WebShareConfig{})

	if !fm.IsImage("photo.jpg") {
		t.Error("jpg 应识别为图片")
	}

	if fm.IsImage("video.mp4") {
		t.Error("mp4 不应识别为图片")
	}
}

func TestFileManager_GetFileType(t *testing.T) {
	fm := NewFileManager(WebShareConfig{})

	tests := []struct {
		filename string
		expected string
	}{
		{"photo.jpg", "image"},
		{"video.mp4", "video"},
		{"song.mp3", "audio"},
		{"doc.pdf", "document"},
		{"script.py", "code"},
		{"archive.zip", "archive"},
		{"unknown.xyz", "other"},
	}

	for _, test := range tests {
		result := fm.GetFileType(test.filename)
		if result != test.expected {
			t.Errorf("文件类型不匹配: %s -> got %s, want %s", test.filename, result, test.expected)
		}
	}
}

func TestSearchIndex_BuildIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "search-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "document.pdf"), []byte("pdf content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "photo.jpg"), []byte("photo"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "video.mp4"), []byte("video"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "readme.md"), []byte("readme"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	si := NewSearchIndex(config)

	// 构建索引
	err = si.BuildIndex(tmpDir)
	if err != nil {
		t.Fatalf("构建索引失败: %v", err)
	}

	stats := si.GetStats()
	totalFiles := stats["totalFiles"].(int)

	if totalFiles < 4 {
		t.Errorf("索引文件数量不匹配: got %d, want >= 4", totalFiles)
	}
}

func TestSearchIndex_Search(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "search-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "document.pdf"), []byte("pdf content"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "photo.jpg"), []byte("photo"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "video.mp4"), []byte("video"), 0644)

	config := WebShareConfig{
		BaseDir:  tmpDir,
		CacheDir: filepath.Join(tmpDir, "cache"),
	}

	si := NewSearchIndex(config)
	si.BuildIndex(tmpDir)

	// 搜索 "document"
	results, err := si.Search("document", "", "", 0, 0)
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("搜索结果数量不匹配: got %d, want 1", len(results))
	}

	// 搜索类型过滤
	imageResults, err := si.Search("", "", "image", 0, 0)
	if err != nil {
		t.Fatalf("类型搜索失败: %v", err)
	}

	if len(imageResults) != 1 {
		t.Errorf("图片搜索结果数量不匹配: got %d, want 1", len(imageResults))
	}
}

func TestBreadcrumb(t *testing.T) {
	manager := NewManager(WebShareConfig{BaseDir: "/tmp"})

	breadcrumb := manager.buildBreadcrumb("/data/photos/2024")

	expected := []struct {
		name string
		path string
	}{
		{"根目录", "/"},
		{"data", "/data"},
		{"photos", "/data/photos"},
		{"2024", "/data/photos/2024"},
	}

	if len(breadcrumb) != len(expected) {
		t.Errorf("面包屑数量不匹配: got %d, want %d", len(breadcrumb), len(expected))
		return
	}

	for i, item := range breadcrumb {
		if item.Name != expected[i].name {
			t.Errorf("名称不匹配[%d]: got %s, want %s", i, item.Name, expected[i].name)
		}
		if item.Path != expected[i].path {
			t.Errorf("路径不匹配[%d]: got %s, want %s", i, item.Path, expected[i].path)
		}
	}
}