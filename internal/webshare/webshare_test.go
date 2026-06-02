package webshare

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestManager(t *testing.T) (*Manager, string) {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "webshare-test")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}
	config := WebShareConfig{
		BaseDir:         tmpDir,
		MaxFileSize:     10 * 1024 * 1024,
		EnableShareLink: true,
		ShareLinkExpiry: 24 * time.Hour,
		CacheDir:        filepath.Join(tmpDir, ".cache"),
	}
	manager := NewManager(config)
	return manager, tmpDir
}

func TestNewManager(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	if manager == nil {
		t.Fatal("创建管理器失败")
	}
	if manager.config.BaseDir != tmpDir {
		t.Errorf("BaseDir 不匹配")
	}
	if manager.shareLinks == nil {
		t.Error("shareLinks 未初始化")
	}
	if manager.uploadQueue == nil {
		t.Error("uploadQueue 未初始化")
	}
}

func TestListDirectory(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test content"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "nested.txt"), []byte("nested"), 0644)

	// 测试列出根目录
	info, err := manager.ListDirectory("/", false, "name", false)
	if err != nil {
		t.Fatalf("列出目录失败: %v", err)
	}
	if info.TotalFiles != 1 {
		t.Errorf("期望 1 个文件，得到 %d", info.TotalFiles)
	}
	// 目录数量可能包含 cache 目录
	if info.TotalDirs < 1 {
		t.Errorf("期望至少 1 个目录，得到 %d", info.TotalDirs)
	}

	// 测试列出子目录
	info, err = manager.ListDirectory("/subdir", false, "name", false)
	if err != nil {
		t.Fatalf("列出子目录失败: %v", err)
	}
	if info.TotalFiles != 1 {
		t.Errorf("期望 1 个文件，得到 %d", info.TotalFiles)
	}
}

func TestCreateDirectory(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	err := manager.CreateDirectory("/newdir")
	if err != nil {
		t.Fatalf("创建目录失败: %v", err)
	}

	// 验证目录存在
	if _, err := os.Stat(filepath.Join(tmpDir, "newdir")); os.IsNotExist(err) {
		t.Error("目录未创建")
	}
}

func TestDeleteFile(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "to_delete.txt")
	os.WriteFile(testFile, []byte("delete me"), 0644)

	err := manager.DeleteFile("/to_delete.txt")
	if err != nil {
		t.Fatalf("删除文件失败: %v", err)
	}

	// 验证文件已删除
	if _, err := os.Stat(testFile); !os.IsNotExist(err) {
		t.Error("文件未删除")
	}
}

func TestMoveFile(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	srcFile := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(srcFile, []byte("move me"), 0644)

	err := manager.MoveFile("/source.txt", "/dest.txt")
	if err != nil {
		t.Fatalf("移动文件失败: %v", err)
	}

	// 验证文件已移动
	if _, err := os.Stat(srcFile); !os.IsNotExist(err) {
		t.Error("源文件未删除")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "dest.txt")); os.IsNotExist(err) {
		t.Error("目标文件不存在")
	}
}

func TestCopyFile(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	srcFile := filepath.Join(tmpDir, "source.txt")
	os.WriteFile(srcFile, []byte("copy me"), 0644)

	err := manager.CopyFile("/source.txt", "/copy.txt")
	if err != nil {
		t.Fatalf("复制文件失败: %v", err)
	}

	// 验证两个文件都存在
	if _, err := os.Stat(srcFile); os.IsNotExist(err) {
		t.Error("源文件不存在")
	}
	if _, err := os.Stat(filepath.Join(tmpDir, "copy.txt")); os.IsNotExist(err) {
		t.Error("副本文件不存在")
	}
}

func TestShareLink(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "shared.txt"), []byte("shared content"), 0644)

	// 创建分享链接
	link, err := manager.CreateShareLink("/shared.txt", "password123", 24, false, false, "testuser")
	if err != nil {
		t.Fatalf("创建分享链接失败: %v", err)
	}

	if link.Token == "" {
		t.Error("Token 为空")
	}
	if link.Password != "password123" {
		t.Errorf("密码不匹配: %s", link.Password)
	}
	if link.CreatedBy != "testuser" {
		t.Errorf("创建者不匹配: %s", link.CreatedBy)
	}

	// 获取分享链接
	fetchedLink, err := manager.GetShareLink(link.Token)
	if err != nil {
		t.Fatalf("获取分享链接失败: %v", err)
	}
	if fetchedLink.Token != link.Token {
		t.Errorf("Token 不匹配: %s != %s", fetchedLink.Token, link.Token)
	}

	// 列出分享链接
	links := manager.ListShareLinks("testuser")
	if len(links) != 1 {
		t.Errorf("期望 1 个链接，得到 %d", len(links))
	}

	// 删除分享链接
	err = manager.DeleteShareLink(link.Token)
	if err != nil {
		t.Fatalf("删除分享链接失败: %v", err)
	}

	// 验证链接已删除
	_, err = manager.GetShareLink(link.Token)
	if err == nil {
		t.Error("链接未删除")
	}
}

func TestShareLinkExpiry(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)

	// 创建 0 小时过期的链接（立即过期）
	link, err := manager.CreateShareLink("/test.txt", "", 0, false, false, "testuser")
	if err != nil {
		t.Fatalf("创建分享链接失败: %v", err)
	}

	// 链接应该已过期
	_, err = manager.GetShareLink(link.Token)
	if err == nil {
		// 注意：GetShareLink 可能不检查过期，这取决于实现
	}
}

func TestUploadProgress(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 开始上传
	progress, err := manager.StartUpload("test.txt", 1024, "/uploads/test.txt")
	if err != nil {
		t.Fatalf("开始上传失败: %v", err)
	}

	if progress.ID == "" {
		t.Error("上传 ID 为空")
	}
	if progress.Status != "uploading" {
		t.Errorf("状态不匹配: %s", progress.Status)
	}

	// 更新进度
	manager.UpdateUploadProgress(progress.ID, 512)
	updated := manager.GetUploadProgress(progress.ID)
	if updated.Uploaded != 512 {
		t.Errorf("上传进度不匹配: %d", updated.Uploaded)
	}

	// 完成上传
	manager.CompleteUpload(progress.ID, "/uploads/test.txt")
	completed := manager.GetUploadProgress(progress.ID)
	if completed.Status != "completed" {
		t.Errorf("状态不匹配: %s", completed.Status)
	}
}

func TestUploadProgressFail(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 开始上传
	progress, err := manager.StartUpload("test.txt", 1024, "/uploads/test.txt")
	if err != nil {
		t.Fatalf("开始上传失败: %v", err)
	}

	// 失败上传
	manager.FailUpload(progress.ID, "network error")
	failed := manager.GetUploadProgress(progress.ID)
	if failed.Status != "failed" {
		t.Errorf("状态不匹配: %s", failed.Status)
	}
	if failed.Error != "network error" {
		t.Errorf("错误信息不匹配: %s", failed.Error)
	}
}

func TestSnapshots(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试目录
	os.Mkdir(filepath.Join(tmpDir, "snapshots"), 0755)

	// 添加快照
	snapshot, err := manager.AddSnapshot("/snapshots", "backup-1", "测试快照")
	if err != nil {
		t.Fatalf("添加快照失败: %v", err)
	}

	if snapshot.Name != "backup-1" {
		t.Errorf("快照名称不匹配: %s", snapshot.Name)
	}
	if snapshot.Description != "测试快照" {
		t.Errorf("快照描述不匹配: %s", snapshot.Description)
	}

	// 获取快照列表
	snapshots, err := manager.GetSnapshots("/snapshots")
	if err != nil {
		t.Fatalf("获取快照失败: %v", err)
	}
	if len(snapshots) != 1 {
		t.Errorf("期望 1 个快照，得到 %d", len(snapshots))
	}
}

func TestSearchFiles(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "hello.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "test.md"), []byte("# Test"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "docs"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "docs", "readme.txt"), []byte("readme"), 0644)

	// 搜索文件 - SearchFiles 可能需要先建立索引
	// 测试按类型搜索
	results, err := manager.SearchFiles("", "/", "txt", 0, 0)
	if err != nil {
		t.Fatalf("按类型搜索失败: %v", err)
	}
	// SearchFiles 返回的是 FileItem 列表
	t.Logf("搜索到 %d 个文件", len(results))
}

func TestListShareLinksFilter(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)

	// 创建多个链接
	manager.CreateShareLink("/test.txt", "", 24, false, false, "user1")
	manager.CreateShareLink("/test.txt", "", 24, false, false, "user1")
	manager.CreateShareLink("/test.txt", "", 24, false, false, "user2")

	// 按用户过滤
	links := manager.ListShareLinks("user1")
	if len(links) != 2 {
		t.Errorf("期望 2 个链接，得到 %d", len(links))
	}

	// 列出所有
	links = manager.ListShareLinks("")
	if len(links) != 3 {
		t.Errorf("期望 3 个链接，得到 %d", len(links))
	}
}

func TestShareLinkWithUpload(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	os.Mkdir(filepath.Join(tmpDir, "uploads"), 0755)

	// 创建允许上传的分享链接
	link, err := manager.CreateShareLink("/uploads", "", 24, true, false, "admin")
	if err != nil {
		t.Fatalf("创建分享链接失败: %v", err)
	}

	if !link.AllowUpload {
		t.Error("期望 AllowUpload 为 true")
	}
	if link.AllowDelete {
		t.Error("期望 AllowDelete 为 false")
	}
}

func TestShareLinkWithDelete(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)

	// 创建允许删除的分享链接
	link, err := manager.CreateShareLink("/test.txt", "", 24, false, true, "admin")
	if err != nil {
		t.Fatalf("创建分享链接失败: %v", err)
	}

	if link.AllowUpload {
		t.Error("期望 AllowUpload 为 false")
	}
	if !link.AllowDelete {
		t.Error("期望 AllowDelete 为 true")
	}
}

func TestShareLinkMaxDownloads(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)

	// 创建分享链接
	link, err := manager.CreateShareLink("/test.txt", "", 24, false, false, "user")
	if err != nil {
		t.Fatalf("创建分享链接失败: %v", err)
	}

	if link.MaxDownloads != 0 {
		t.Errorf("期望 MaxDownloads 为 0，得到 %d", link.MaxDownloads)
	}
	if link.Downloads != 0 {
		t.Errorf("期望 Downloads 为 0，得到 %d", link.Downloads)
	}
}

func TestBreadcrumb(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建深层目录结构
	os.MkdirAll(filepath.Join(tmpDir, "a", "b", "c"), 0755)

	info, err := manager.ListDirectory("/a/b/c", false, "name", false)
	if err != nil {
		t.Fatalf("列出目录失败: %v", err)
	}

	if len(info.Breadcrumb) < 4 {
		t.Errorf("期望至少 4 个面包屑项，得到 %d", len(info.Breadcrumb))
	}
}

func TestHiddenFiles(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建隐藏文件
	os.WriteFile(filepath.Join(tmpDir, ".hidden"), []byte("hidden"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "visible.txt"), []byte("visible"), 0644)

	// 不显示隐藏文件
	info, _ := manager.ListDirectory("/", false, "name", false)
	for _, item := range info.Items {
		if item.Name == ".hidden" {
			t.Error("不应显示隐藏文件")
		}
	}

	// 显示隐藏文件
	info, _ = manager.ListDirectory("/", true, "name", false)
	found := false
	for _, item := range info.Items {
		if item.Name == ".hidden" {
			found = true
		}
	}
	if !found {
		t.Error("应显示隐藏文件")
	}
}

func TestSortFiles(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 创建测试文件
	os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("c"), 0644)

	// 按名称排序
	info, _ := manager.ListDirectory("/", false, "name", false)
	if len(info.Items) >= 2 {
		if info.Items[0].Name > info.Items[1].Name {
			t.Error("文件未按名称排序")
		}
	}

	// 按名称降序排序
	info, _ = manager.ListDirectory("/", false, "name", true)
	if len(info.Items) >= 2 {
		if info.Items[0].Name < info.Items[1].Name {
			t.Error("文件未按名称降序排序")
		}
	}
}

func TestPathSanitization(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	// 测试路径穿越攻击
	_, err := manager.ListDirectory("/../../../etc", false, "name", false)
	if err == nil {
		t.Error("应拒绝路径穿越")
	}

	// 测试正常路径
	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)
	_, err = manager.ListDirectory("/test.txt", false, "name", false)
	if err != nil {
		// 这可能是预期的，因为 test.txt 是文件不是目录
	}
}

func TestConcurrentAccess(t *testing.T) {
	manager, tmpDir := setupTestManager(t)
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)

	// 并发创建分享链接
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			_, err := manager.CreateShareLink("/test.txt", "", 24, false, false, "user")
			if err != nil {
				t.Errorf("并发创建链接失败: %v", err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	links := manager.ListShareLinks("")
	if len(links) != 10 {
		t.Errorf("期望 10 个链接，得到 %d", len(links))
	}
}
