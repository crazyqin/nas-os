package webdav

import (
	"testing"
	"time"
)

func TestNewLockManager(t *testing.T) {
	lm := NewLockManager()
	if lm == nil {
		t.Fatal("LockManager 不应为 nil")
	}
	if lm.locks == nil {
		t.Error("locks map 应该被初始化")
	}
	if lm.paths == nil {
		t.Error("paths map 应该被初始化")
	}
}

func TestCreateLock(t *testing.T) {
	lm := NewLockManager()

	// 测试创建排他锁
	lock, err := lm.CreateLock("/test/path", "owner1", 0, "exclusive", 0)
	if err != nil {
		t.Fatalf("创建锁失败: %v", err)
	}
	if lock == nil {
		t.Fatal("锁不应为 nil")
	}
	if lock.Path != "/test/path" {
		t.Errorf("期望路径 /test/path，实际为 %s", lock.Path)
	}
	if lock.Owner != "owner1" {
		t.Errorf("期望所有者 owner1，实际为 %s", lock.Owner)
	}
	if lock.Scope != "exclusive" {
		t.Errorf("期望范围 exclusive，实际为 %s", lock.Scope)
	}
	if lock.Type != "write" {
		t.Errorf("期望类型 write，实际为 %s", lock.Type)
	}
}

func TestCreateLockWithTimeout(t *testing.T) {
	lm := NewLockManager()

	// 创建带超时的锁
	lock, err := lm.CreateLock("/test/timeout", "owner1", 0, "exclusive", 60)
	if err != nil {
		t.Fatalf("创建锁失败: %v", err)
	}

	if lock.Timeout.IsZero() {
		t.Error("锁应该有超时时间")
	}

	// 验证超时时间大约是 60 秒后
	expectedTimeout := time.Now().Add(60 * time.Second)
	diff := lock.Timeout.Sub(expectedTimeout)
	if diff < -time.Second || diff > time.Second {
		t.Errorf("超时时间不正确，期望约 %v，实际为 %v", expectedTimeout, lock.Timeout)
	}
}

func TestCreateExclusiveLockConflict(t *testing.T) {
	lm := NewLockManager()

	// 创建第一个排他锁
	_, err := lm.CreateLock("/test/conflict", "owner1", 0, "exclusive", 0)
	if err != nil {
		t.Fatalf("创建第一个锁失败: %v", err)
	}

	// 尝试在同一路径创建另一个排他锁
	_, err = lm.CreateLock("/test/conflict", "owner2", 0, "exclusive", 0)
	if err != ErrLocked {
		t.Errorf("期望错误 ErrLocked，实际为 %v", err)
	}
}

func TestCreateSharedLock(t *testing.T) {
	lm := NewLockManager()

	// 创建第一个共享锁
	lock1, err := lm.CreateLock("/test/shared", "owner1", 0, "shared", 0)
	if err != nil {
		t.Fatalf("创建第一个共享锁失败: %v", err)
	}

	// 在同一路径尝试创建排他锁应该失败
	_, err = lm.CreateLock("/test/shared", "owner2", 0, "exclusive", 0)
	if err != ErrLocked {
		t.Errorf("期望错误 ErrLocked，实际为 %v", err)
	}

	// 验证第一个锁仍然存在
	_, exists := lm.GetLock(lock1.Token)
	if !exists {
		t.Error("第一个锁应该仍然存在")
	}
}

func TestGetLock(t *testing.T) {
	lm := NewLockManager()

	// 创建锁
	lock, err := lm.CreateLock("/test/get", "owner1", 0, "exclusive", 0)
	if err != nil {
		t.Fatalf("创建锁失败: %v", err)
	}

	// 获取锁
	gotLock, exists := lm.GetLock(lock.Token)
	if !exists {
		t.Fatal("锁应该存在")
	}
	if gotLock.Path != lock.Path {
		t.Errorf("期望路径 %s，实际为 %s", lock.Path, gotLock.Path)
	}

	// 获取不存在的锁
	_, exists = lm.GetLock("nonexistent-token")
	if exists {
		t.Error("不存在的锁应该返回 false")
	}
}

func TestGetLockByPath(t *testing.T) {
	lm := NewLockManager()

	// 创建锁
	lock, err := lm.CreateLock("/test/path", "owner1", 0, "exclusive", 0)
	if err != nil {
		t.Fatalf("创建锁失败: %v", err)
	}

	// 通过路径获取锁
	gotLock, exists := lm.GetLockByPath("/test/path")
	if !exists {
		t.Fatal("锁应该存在")
	}
	if gotLock.Token != lock.Token {
		t.Error("令牌应该匹配")
	}

	// 获取不存在路径的锁
	_, exists = lm.GetLockByPath("/nonexistent/path")
	if exists {
		t.Error("不存在的路径应该返回 false")
	}
}

func TestGetLockExpired(t *testing.T) {
	lm := NewLockManager()

	// 创建一个立即过期的锁
	lock, err := lm.CreateLock("/test/expired", "owner1", 0, "exclusive", -1)
	if err != nil {
		t.Fatalf("创建锁失败: %v", err)
	}

	// 手动设置过期时间
	lm.mu.Lock()
	lm.locks[lock.Token].Timeout = time.Now().Add(-time.Second)
	lm.mu.Unlock()

	// 获取过期的锁
	_, exists := lm.GetLock(lock.Token)
	if exists {
		t.Error("过期的锁应该返回 false")
	}
}

func TestRefreshLock(t *testing.T) {
	lm := NewLockManager()

	// 创建锁
	lock, err := lm.CreateLock("/test/refresh", "owner1", 0, "exclusive", 0)
	if err != nil {
		t.Fatalf("创建锁失败: %v", err)
	}

	// 刷新锁
	refreshed, err := lm.RefreshLock(lock.Token, 120)
	if err != nil {
		t.Fatalf("刷新锁失败: %v", err)
	}

	if refreshed.Timeout.IsZero() {
		t.Error("刷新后的锁应该有超时时间")
	}
}

func TestRefreshLockNotFound(t *testing.T) {
	lm := NewLockManager()

	// 刷新不存在的锁
	_, err := lm.RefreshLock("nonexistent-token", 60)
	if err != ErrLockNotFound {
		t.Errorf("期望错误 ErrLockNotFound，实际为 %v", err)
	}
}

func TestRemoveLock(t *testing.T) {
	lm := NewLockManager()

	// 创建锁
	lock, err := lm.CreateLock("/test/remove", "owner1", 0, "exclusive", 0)
	if err != nil {
		t.Fatalf("创建锁失败: %v", err)
	}

	// 移除锁
	if err := lm.RemoveLock(lock.Token); err != nil {
		t.Fatalf("移除锁失败: %v", err)
	}

	// 验证锁已被移除
	_, exists := lm.GetLock(lock.Token)
	if exists {
		t.Error("锁应该已被移除")
	}

	// 验证路径映射也被移除
	_, exists = lm.GetLockByPath("/test/remove")
	if exists {
		t.Error("路径映射应该已被移除")
	}
}

func TestRemoveLockNotFound(t *testing.T) {
	lm := NewLockManager()

	// 移除不存在的锁
	err := lm.RemoveLock("nonexistent-token")
	if err != ErrLockNotFound {
		t.Errorf("期望错误 ErrLockNotFound，实际为 %v", err)
	}
}

func TestIsLocked(t *testing.T) {
	lm := NewLockManager()

	// 创建锁前
	if lm.IsLocked("/test/check") {
		t.Error("路径应该未被锁定")
	}

	// 创建锁
	_, err := lm.CreateLock("/test/check", "owner1", 0, "exclusive", 0)
	if err != nil {
		t.Fatalf("创建锁失败: %v", err)
	}

	// 创建锁后
	if !lm.IsLocked("/test/check") {
		t.Error("路径应该被锁定")
	}
}

func TestValidateToken(t *testing.T) {
	lm := NewLockManager()

	// 创建锁
	lock, err := lm.CreateLock("/test/validate", "owner1", 0, "exclusive", 0)
	if err != nil {
		t.Fatalf("创建锁失败: %v", err)
	}

	// 验证正确的令牌和路径
	if !lm.ValidateToken("/test/validate", lock.Token) {
		t.Error("令牌验证应该成功")
	}

	// 验证错误的路径
	if lm.ValidateToken("/wrong/path", lock.Token) {
		t.Error("错误路径的验证应该失败")
	}

	// 验证错误的令牌
	if lm.ValidateToken("/test/validate", "wrong-token") {
		t.Error("错误令牌的验证应该失败")
	}
}

func TestCleanupExpired(t *testing.T) {
	lm := NewLockManager()

	// 创建两个锁，一个过期一个不过期
	lock1, _ := lm.CreateLock("/test/expired", "owner1", 0, "exclusive", 0)
	lock2, _ := lm.CreateLock("/test/active", "owner2", 0, "exclusive", 0)

	// 手动设置过期时间
	lm.mu.Lock()
	lm.locks[lock1.Token].Timeout = time.Now().Add(-time.Second)
	lm.locks[lock2.Token].Timeout = time.Now().Add(time.Hour)
	lm.mu.Unlock()

	// 清理过期锁
	count := lm.CleanupExpired()
	if count != 1 {
		t.Errorf("期望清理 1 个锁，实际清理了 %d 个", count)
	}

	// 验证过期的锁已被移除
	_, exists := lm.GetLock(lock1.Token)
	if exists {
		t.Error("过期的锁应该已被清理")
	}

	// 验证活跃的锁仍然存在
	_, exists = lm.GetLock(lock2.Token)
	if !exists {
		t.Error("活跃的锁应该仍然存在")
	}
}

func TestGenerateLockToken(t *testing.T) {
	token1, err := generateLockToken()
	if err != nil {
		t.Fatalf("生成令牌失败: %v", err)
	}
	if token1 == "" {
		t.Error("令牌不应为空")
	}
	if len(token1) < 10 {
		t.Error("令牌应该足够长")
	}

	// 生成第二个令牌验证唯一性
	token2, err := generateLockToken()
	if err != nil {
		t.Fatalf("生成第二个令牌失败: %v", err)
	}
	if token1 == token2 {
		t.Error("两个令牌应该不同")
	}
}