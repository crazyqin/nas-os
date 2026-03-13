package trash

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestManager(t *testing.T) (*Manager, string, func()) {
	t.Helper()

	// 创建临时目录
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	trashRoot := filepath.Join(tmpDir, "trash")

	// 创建管理器
	config := &Config{
		Enabled:       true,
		RetentionDays: 30,
		MaxSize:       1024 * 1024 * 100, // 100MB
		AutoEmpty:     false,
	}

	mgr, err := NewManager(configPath, trashRoot, config)
	if err != nil {
		t.Fatalf("创建管理器失败：%v", err)
	}

	cleanup := func() {
		mgr.Empty()
	}

	return mgr, tmpDir, cleanup
}

func TestMoveToTrash(t *testing.T) {
	mgr, _, cleanup := setupTestManager(t)
	defer cleanup()

	// 创建测试文件
	tmpFile := filepath.Join(t.TempDir(), "testfile.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("创建测试文件失败：%v", err)
	}

	// 移动到回收站
	item, err := mgr.MoveToTrash(tmpFile, "user1")
	if err != nil {
		t.Fatalf("MoveToTrash 失败：%v", err)
	}

	if item.Name != "testfile.txt" {
		t.Errorf("期望文件名 testfile.txt, 得到 %s", item.Name)
	}

	if item.DeletedBy != "user1" {
		t.Errorf("期望删除者 user1, 得到 %s", item.DeletedBy)
	}

	// 验证原文件已不存在
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("原文件应该不存在")
	}

	// 验证回收站文件存在
	if _, err := os.Stat(item.TrashPath); err != nil {
		t.Errorf("回收站文件应该存在：%v", err)
	}
}

func TestRestore(t *testing.T) {
	mgr, _, cleanup := setupTestManager(t)
	defer cleanup()

	// 创建测试文件
	tmpFile := filepath.Join(t.TempDir(), "testfile.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("创建测试文件失败：%v", err)
	}

	// 移动到回收站
	item, err := mgr.MoveToTrash(tmpFile, "user1")
	if err != nil {
		t.Fatalf("MoveToTrash 失败：%v", err)
	}

	// 恢复到原始路径
	if err := mgr.Restore(item.ID); err != nil {
		t.Fatalf("Restore 失败：%v", err)
	}

	// 验证文件已恢复到原位置
	if _, err := os.Stat(tmpFile); err != nil {
		t.Errorf("文件应该恢复到原位置：%v", err)
	}

	// 验证回收站文件已不存在
	if _, err := os.Stat(item.TrashPath); !os.IsNotExist(err) {
		t.Error("回收站文件应该不存在")
	}

	// 验证回收站列表为空
	items := mgr.List()
	if len(items) != 0 {
		t.Errorf("期望回收站为空，得到 %d 个项目", len(items))
	}
}

func TestRestoreTo(t *testing.T) {
	mgr, _, cleanup := setupTestManager(t)
	defer cleanup()

	// 创建测试文件
	originalPath := filepath.Join(t.TempDir(), "testfile.txt")
	if err := os.WriteFile(originalPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("创建测试文件失败：%v", err)
	}

	// 移动到回收站
	item, err := mgr.MoveToTrash(originalPath, "user1")
	if err != nil {
		t.Fatalf("MoveToTrash 失败：%v", err)
	}

	// 恢复到新路径
	newPath := filepath.Join(t.TempDir(), "restored", "newfile.txt")
	if err := mgr.RestoreTo(item.ID, newPath); err != nil {
		t.Fatalf("RestoreTo 失败：%v", err)
	}

	// 验证文件已恢复到新位置
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("文件应该恢复到新位置：%v", err)
	}

	// 验证原位置文件不存在
	if _, err := os.Stat(originalPath); !os.IsNotExist(err) {
		t.Error("原位置文件应该不存在")
	}

	// 验证回收站文件已不存在
	if _, err := os.Stat(item.TrashPath); !os.IsNotExist(err) {
		t.Error("回收站文件应该不存在")
	}
}

func TestRestoreTo_OverwriteProtection(t *testing.T) {
	mgr, _, cleanup := setupTestManager(t)
	defer cleanup()

	// 创建测试文件
	originalPath := filepath.Join(t.TempDir(), "testfile.txt")
	if err := os.WriteFile(originalPath, []byte("test content"), 0644); err != nil {
		t.Fatalf("创建测试文件失败：%v", err)
	}

	// 移动到回收站
	item, err := mgr.MoveToTrash(originalPath, "user1")
	if err != nil {
		t.Fatalf("MoveToTrash 失败：%v", err)
	}

	// 创建目标位置的文件
	targetPath := filepath.Join(t.TempDir(), "existing.txt")
	if err := os.WriteFile(targetPath, []byte("existing"), 0644); err != nil {
		t.Fatalf("创建目标文件失败：%v", err)
	}

	// 尝试恢复到已存在的位置（应该失败）
	err = mgr.RestoreTo(item.ID, targetPath)
	if err == nil {
		t.Error("期望恢复到已存在位置失败")
	}

	// 验证回收站文件仍然存在
	if _, err := os.Stat(item.TrashPath); err != nil {
		t.Error("回收站文件应该仍然存在")
	}
}

func TestDeletePermanently(t *testing.T) {
	mgr, _, cleanup := setupTestManager(t)
	defer cleanup()

	// 创建测试文件
	tmpFile := filepath.Join(t.TempDir(), "testfile.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("创建测试文件失败：%v", err)
	}

	// 移动到回收站
	item, err := mgr.MoveToTrash(tmpFile, "user1")
	if err != nil {
		t.Fatalf("MoveToTrash 失败：%v", err)
	}

	// 永久删除
	if err := mgr.DeletePermanently(item.ID); err != nil {
		t.Fatalf("DeletePermanently 失败：%v", err)
	}

	// 验证回收站文件已删除
	if _, err := os.Stat(item.TrashPath); !os.IsNotExist(err) {
		t.Error("回收站文件应该被永久删除")
	}

	// 验证回收站列表为空
	items := mgr.List()
	if len(items) != 0 {
		t.Errorf("期望回收站为空，得到 %d 个项目", len(items))
	}
}

func TestEmpty(t *testing.T) {
	mgr, _, cleanup := setupTestManager(t)
	defer cleanup()

	// 创建多个测试文件
	tmpDir := t.TempDir()
	for i := 0; i < 3; i++ {
		tmpFile := filepath.Join(tmpDir, "testfile"+string(rune('0'+i))+".txt")
		if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
			t.Fatalf("创建测试文件失败：%v", err)
		}

		_, err := mgr.MoveToTrash(tmpFile, "user1")
		if err != nil {
			t.Fatalf("MoveToTrash 失败：%v", err)
		}
	}

	// 验证有 3 个项目
	items := mgr.List()
	if len(items) != 3 {
		t.Fatalf("期望 3 个项目，得到 %d", len(items))
	}

	// 清空回收站
	if err := mgr.Empty(); err != nil {
		t.Fatalf("Empty 失败：%v", err)
	}

	// 验证回收站为空
	items = mgr.List()
	if len(items) != 0 {
		t.Errorf("期望回收站为空，得到 %d 个项目", len(items))
	}
}

func TestGetStats(t *testing.T) {
	mgr, _, cleanup := setupTestManager(t)
	defer cleanup()

	// 创建测试文件
	tmpFile := filepath.Join(t.TempDir(), "testfile.txt")
	content := []byte("test content for stats")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("创建测试文件失败：%v", err)
	}

	// 移动到回收站
	_, err := mgr.MoveToTrash(tmpFile, "user1")
	if err != nil {
		t.Fatalf("MoveToTrash 失败：%v", err)
	}

	// 获取统计
	stats := mgr.GetStats()

	if stats["total_items"] != 1 {
		t.Errorf("期望 1 个项目，得到 %v", stats["total_items"])
	}

	if stats["total_size"] != int64(len(content)) {
		t.Errorf("期望大小 %d, 得到 %v", len(content), stats["total_size"])
	}

	if stats["enabled"] != true {
		t.Error("期望回收站启用")
	}
}

func TestCleanupExpired(t *testing.T) {
	mgr, _, cleanup := setupTestManager(t)
	defer cleanup()

	// 创建测试文件
	tmpFile := filepath.Join(t.TempDir(), "testfile.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("创建测试文件失败：%v", err)
	}

	// 移动到回收站
	item, err := mgr.MoveToTrash(tmpFile, "user1")
	if err != nil {
		t.Fatalf("MoveToTrash 失败：%v", err)
	}

	// 手动设置过期时间（已过期）
	mgr.mu.Lock()
	item.ExpiresAt = time.Now().Add(-24 * time.Hour)
	mgr.mu.Unlock()

	// 清理过期项目
	mgr.cleanupExpired()

	// 验证项目已被清理
	items := mgr.List()
	if len(items) != 0 {
		t.Errorf("期望过期项目被清理，得到 %d 个项目", len(items))
	}
}

func TestConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")
	trashRoot := filepath.Join(tmpDir, "trash")

	// 创建第一个管理器
	config1 := &Config{
		Enabled:       true,
		RetentionDays: 15,
		MaxSize:       50 * 1024 * 1024,
		AutoEmpty:     true,
	}

	mgr1, err := NewManager(configPath, trashRoot, config1)
	if err != nil {
		t.Fatalf("创建管理器 1 失败：%v", err)
	}

	// 创建第二个管理器（应该加载相同的配置）
	mgr2, err := NewManager(configPath, trashRoot, nil)
	if err != nil {
		t.Fatalf("创建管理器 2 失败：%v", err)
	}

	config2 := mgr2.GetConfig()

	if config2.RetentionDays != 15 {
		t.Errorf("期望 RetentionDays=15, 得到 %d", config2.RetentionDays)
	}

	if config2.MaxSize != 50*1024*1024 {
		t.Errorf("期望 MaxSize=50MB, 得到 %d", config2.MaxSize)
	}

	_ = mgr1
}

func TestGet(t *testing.T) {
	mgr, _, cleanup := setupTestManager(t)
	defer cleanup()

	// 创建测试文件
	tmpFile := filepath.Join(t.TempDir(), "testfile.txt")
	if err := os.WriteFile(tmpFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("创建测试文件失败：%v", err)
	}

	// 移动到回收站
	item, err := mgr.MoveToTrash(tmpFile, "user1")
	if err != nil {
		t.Fatalf("MoveToTrash 失败：%v", err)
	}

	// 获取项目
	gotItem, err := mgr.Get(item.ID)
	if err != nil {
		t.Fatalf("Get 失败：%v", err)
	}

	if gotItem.ID != item.ID {
		t.Errorf("期望 ID=%s, 得到 %s", item.ID, gotItem.ID)
	}

	if gotItem.Name != "testfile.txt" {
		t.Errorf("期望 Name=testfile.txt, 得到 %s", gotItem.Name)
	}

	// 获取不存在的项目
	_, err = mgr.Get("nonexistent")
	if err == nil {
		t.Error("期望获取不存在的项目失败")
	}
}
