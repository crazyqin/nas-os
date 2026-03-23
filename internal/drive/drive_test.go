package drive

import (
	"context"
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	cfg := &Config{
		RootPath:    "/tmp/drive-test",
		SyncInterval: 300,
		MaxVersions: 32,
		LockTimeout: 3600,
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}

	if svc == nil {
		t.Fatal("服务不应为 nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.SyncInterval != 300 {
		t.Errorf("预期 SyncInterval=300, 实际=%d", cfg.SyncInterval)
	}

	if cfg.MaxVersions != 32 {
		t.Errorf("预期 MaxVersions=32, 实际=%d", cfg.MaxVersions)
	}
}

func TestFileLocker(t *testing.T) {
	locker := NewFileLocker(time.Hour)

	// 测试锁定
	err := locker.Lock("/test/file.txt", "user1")
	if err != nil {
		t.Fatalf("锁定失败: %v", err)
	}

	// 测试重复锁定
	err = locker.Lock("/test/file.txt", "user2")
	if err == nil {
		t.Fatal("应该返回锁定错误")
	}

	// 测试解锁
	err = locker.Unlock("/test/file.txt", "user1")
	if err != nil {
		t.Fatalf("解锁失败: %v", err)
	}

	// 测试解锁后重新锁定
	err = locker.Lock("/test/file.txt", "user2")
	if err != nil {
		t.Fatalf("重新锁定失败: %v", err)
	}
}

func TestFileLockerExpiration(t *testing.T) {
	locker := NewFileLocker(100 * time.Millisecond)

	locker.Lock("/test/file.txt", "user1")

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 应该可以重新锁定
	err := locker.Lock("/test/file.txt", "user2")
	if err != nil {
		t.Fatalf("过期后应可重新锁定: %v", err)
	}
}

func TestVersionManager(t *testing.T) {
	vm := NewVersionManager(10)

	// 测试创建版本
	v1, err := vm.CreateVersion("/test/file.txt", "hash1", 100, time.Now())
	if err != nil {
		t.Fatalf("创建版本失败: %v", err)
	}

	if v1.Version != 1 {
		t.Errorf("预期版本号=1, 实际=%d", v1.Version)
	}

	// 测试版本列表
	versions, err := vm.ListVersions("/test/file.txt")
	if err != nil {
		t.Fatalf("获取版本列表失败: %v", err)
	}

	if len(versions) != 1 {
		t.Errorf("预期版本数=1, 实际=%d", len(versions))
	}
}

func TestAuditLogger(t *testing.T) {
	logger := NewAuditLogger(true)

	// 测试日志记录
	logger.Log(AuditActionSync, "/test/file.txt", "test sync")

	// 测试带用户信息的日志
	logger.LogWithUser(AuditActionLock, "/test/file.txt", "user1", "Test User", "127.0.0.1", "test lock")

	// 测试错误日志
	logger.LogError(AuditActionDelete, "/test/file.txt", "permission denied")
}

func TestServiceStartStop(t *testing.T) {
	cfg := &Config{
		RootPath:     "/tmp/drive-test-startstop",
		SyncInterval: 300,
		MaxVersions:  32,
		LockTimeout:  3600,
	}

	svc, err := NewService(cfg)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}

	ctx := context.Background()

	// 启动
	err = svc.Start(ctx)
	if err != nil {
		t.Fatalf("启动服务失败: %v", err)
	}

	// 停止
	err = svc.Stop()
	if err != nil {
		t.Fatalf("停止服务失败: %v", err)
	}
}