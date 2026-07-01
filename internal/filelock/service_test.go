// Package filelock 提供文件锁定功能。
// 单元测试覆盖服务层与 HTTP handler 层。
package filelock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ===== 测试辅助 =====

func setupTestService(t *testing.T) *Service {
	t.Helper()
	return NewService(nil) // 使用默认配置
}

func setupTestRouter(t *testing.T, s *Service) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := NewHandler(s)
	h.RegisterRoutes(r.Group("/api/v1"))
	return r
}

// ===== 服务层测试 =====

func TestLock(t *testing.T) {
	s := setupTestService(t)

	t.Run("成功获取锁", func(t *testing.T) {
		req := &LockRequest{
			FilePath: "/docs/test.txt",
			UserID:   "user-001",
			UserName: "张三",
		}
		lock, err := s.Lock(req)
		if err != nil {
			t.Fatalf("Lock 失败: %v", err)
		}
		if lock.ID == "" {
			t.Error("锁 ID 不应为空")
		}
		if lock.Status != LockStatusActive {
			t.Errorf("期望 status=active, 实际=%s", lock.Status)
		}
		if lock.FilePath != "/docs/test.txt" {
			t.Errorf("期望 file_path=/docs/test.txt, 实际=%s", lock.FilePath)
		}
		if lock.UserID != "user-001" {
			t.Errorf("期望 user_id=user-001, 实际=%s", lock.UserID)
		}
		if lock.AcquiredAt.IsZero() {
			t.Error("AcquiredAt 不应为零值")
		}
		if lock.ExpiresAt.IsZero() {
			t.Error("ExpiresAt 不应为零值")
		}
		// 默认过期时间应为 30 分钟后
		expectedExpiry := lock.AcquiredAt.Add(30 * time.Minute)
		if lock.ExpiresAt.Sub(expectedExpiry).Abs() > time.Second {
			t.Errorf("期望过期时间约为 30 分钟后, 实际差值=%v", lock.ExpiresAt.Sub(lock.AcquiredAt))
		}
	})

	t.Run("自定义锁定时长", func(t *testing.T) {
		req := &LockRequest{
			FilePath:       "/docs/custom.txt",
			UserID:         "user-002",
			UserName:       "李四",
			DurationMinutes: 60,
		}
		lock, err := s.Lock(req)
		if err != nil {
			t.Fatalf("Lock 失败: %v", err)
		}
		duration := lock.ExpiresAt.Sub(lock.AcquiredAt)
		if duration.Abs()-60*time.Minute > time.Second {
			t.Errorf("期望锁定时长 60 分钟, 实际=%v", duration)
		}
	})

	t.Run("同一用户重复锁定同一文件", func(t *testing.T) {
		req := &LockRequest{
			FilePath: "/docs/repeat.txt",
			UserID:   "user-003",
			UserName: "王五",
		}
		_, err := s.Lock(req)
		if err != nil {
			t.Fatalf("首次 Lock 失败: %v", err)
		}
		_, err = s.Lock(req)
		if err == nil {
			t.Error("同一用户重复锁定应返回错误")
		}
	})

	t.Run("不同用户锁定已被锁定的文件", func(t *testing.T) {
		s2 := setupTestService(t)
		s2.Lock(&LockRequest{
			FilePath: "/docs/conflict.txt",
			UserID:   "user-a",
			UserName: "用户A",
		})
		_, err := s2.Lock(&LockRequest{
			FilePath: "/docs/conflict.txt",
			UserID:   "user-b",
			UserName: "用户B",
		})
		if err == nil {
			t.Error("不同用户锁定已被锁定的文件应返回错误")
		}
	})
}

func TestUnlock(t *testing.T) {
	s := setupTestService(t)

	t.Run("按 lock_id 解锁", func(t *testing.T) {
		lock, _ := s.Lock(&LockRequest{
			FilePath: "/docs/test.txt",
			UserID:   "user-001",
			UserName: "张三",
		})

		count, err := s.Unlock(&UnlockRequest{
			LockID: lock.ID,
			UserID: "user-001",
		})
		if err != nil {
			t.Fatalf("Unlock 失败: %v", err)
		}
		if count != 1 {
			t.Errorf("期望释放 1 个锁, 实际=%d", count)
		}

		// 验证锁已释放
		released, _ := s.GetLock(lock.ID)
		if released.Status != LockStatusReleased {
			t.Errorf("期望 status=released, 实际=%s", released.Status)
		}
		if released.ReleasedAt == nil {
			t.Error("ReleasedAt 不应为 nil")
		}
	})

	t.Run("按 file_path 解锁", func(t *testing.T) {
		s2 := setupTestService(t)
		lock, _ := s2.Lock(&LockRequest{
			FilePath: "/docs/path-test.txt",
			UserID:   "user-001",
			UserName: "张三",
		})

		count, err := s2.Unlock(&UnlockRequest{
			FilePath: lock.FilePath,
			UserID:   "user-001",
		})
		if err != nil {
			t.Fatalf("Unlock 失败: %v", err)
		}
		if count != 1 {
			t.Errorf("期望释放 1 个锁, 实际=%d", count)
		}
	})

	t.Run("无权解锁他人锁", func(t *testing.T) {
		lock, _ := s.Lock(&LockRequest{
			FilePath: "/docs/perm-test.txt",
			UserID:   "user-001",
			UserName: "张三",
		})

		_, err := s.Unlock(&UnlockRequest{
			LockID: lock.ID,
			UserID: "user-002", // 不同用户
		})
		if err == nil {
			t.Error("无权解锁他人锁应返回错误")
		}
	})

	t.Run("解锁不存在的锁", func(t *testing.T) {
		_, err := s.Unlock(&UnlockRequest{
			LockID: "nonexistent",
			UserID: "user-001",
		})
		if err == nil {
			t.Error("解锁不存在的锁应返回错误")
		}
	})

	t.Run("无参数解锁", func(t *testing.T) {
		_, err := s.Unlock(&UnlockRequest{
			UserID: "user-001",
		})
		if err == nil {
			t.Error("无 lock_id 和 file_path 应返回错误")
		}
	})
}

func TestList(t *testing.T) {
	s := setupTestService(t)

	// 初始为空
	resp := s.List()
	if resp.Total != 0 {
		t.Errorf("期望 0 个锁, 实际=%d", resp.Total)
	}

	// 添加锁
	for i := 1; i <= 3; i++ {
		s.Lock(&LockRequest{
			FilePath: fmt.Sprintf("/docs/file%d.txt", i),
			UserID:   fmt.Sprintf("user-%03d", i),
			UserName: fmt.Sprintf("用户%d", i),
		})
	}

	resp = s.List()
	if resp.Total != 3 {
		t.Errorf("期望 3 个锁, 实际=%d", resp.Total)
	}
	if len(resp.Locks) != 3 {
		t.Errorf("期望 3 条锁记录, 实际=%d", len(resp.Locks))
	}
}

func TestListByUser(t *testing.T) {
	s := setupTestService(t)

	s.Lock(&LockRequest{FilePath: "/docs/a.txt", UserID: "user-001", UserName: "张三"})
	s.Lock(&LockRequest{FilePath: "/docs/b.txt", UserID: "user-001", UserName: "张三"})
	s.Lock(&LockRequest{FilePath: "/docs/c.txt", UserID: "user-002", UserName: "李四"})

	locks := s.ListByUser("user-001")
	if len(locks) != 2 {
		t.Errorf("期望 2 个锁, 实际=%d", len(locks))
	}

	locks = s.ListByUser("user-002")
	if len(locks) != 1 {
		t.Errorf("期望 1 个锁, 实际=%d", len(locks))
	}

	locks = s.ListByUser("nonexistent")
	if len(locks) != 0 {
		t.Errorf("期望 0 个锁, 实际=%d", len(locks))
	}
}

func TestListByFile(t *testing.T) {
	s := setupTestService(t)

	s.Lock(&LockRequest{FilePath: "/docs/shared.txt", UserID: "user-001", UserName: "张三"})

	locks := s.ListByFile("/docs/shared.txt")
	if len(locks) != 1 {
		t.Errorf("期望 1 个锁, 实际=%d", len(locks))
	}

	locks = s.ListByFile("/docs/nonexistent.txt")
	if len(locks) != 0 {
		t.Errorf("期望 0 个锁, 实际=%d", len(locks))
	}
}

func TestIsFileLocked(t *testing.T) {
	s := setupTestService(t)

	if s.IsFileLocked("/docs/test.txt") {
		t.Error("未锁定的文件应返回 false")
	}

	s.Lock(&LockRequest{
		FilePath: "/docs/test.txt",
		UserID:   "user-001",
		UserName: "张三",
	})

	if !s.IsFileLocked("/docs/test.txt") {
		t.Error("已锁定的文件应返回 true")
	}
}

func TestGetLock(t *testing.T) {
	s := setupTestService(t)
	lock, _ := s.Lock(&LockRequest{
		FilePath: "/docs/test.txt",
		UserID:   "user-001",
		UserName: "张三",
	})

	t.Run("获取存在的锁", func(t *testing.T) {
		got, err := s.GetLock(lock.ID)
		if err != nil {
			t.Fatalf("GetLock 失败: %v", err)
		}
		if got.ID != lock.ID {
			t.Errorf("期望 ID=%s, 实际=%s", lock.ID, got.ID)
		}
	})

	t.Run("获取不存在的锁", func(t *testing.T) {
		_, err := s.GetLock("nonexistent")
		if err == nil {
			t.Error("不存在的锁应返回错误")
		}
	})
}

func TestMaxLocksPerUser(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxLocksPerUser = 2
	s := NewService(cfg)

	// 获取 2 个锁（达到上限）
	for i := 1; i <= 2; i++ {
		_, err := s.Lock(&LockRequest{
			FilePath: fmt.Sprintf("/docs/file%d.txt", i),
			UserID:   "user-001",
			UserName: "张三",
		})
		if err != nil {
			t.Fatalf("第 %d 个锁获取失败: %v", i, err)
		}
	}

	// 第 3 个应失败
	_, err := s.Lock(&LockRequest{
		FilePath: "/docs/file3.txt",
		UserID:   "user-001",
		UserName: "张三",
	})
	if err == nil {
		t.Error("超过用户锁上限应返回错误")
	}
}

func TestDisabledConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	s := NewService(cfg)

	_, err := s.Lock(&LockRequest{
		FilePath: "/docs/test.txt",
		UserID:   "user-001",
		UserName: "张三",
	})
	if err == nil {
		t.Error("功能禁用时应返回错误")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Enabled {
		t.Error("默认应启用")
	}
	if cfg.DefaultDurationMinutes != 30 {
		t.Errorf("期望默认时长 30 分钟, 实际=%d", cfg.DefaultDurationMinutes)
	}
	if cfg.CleanupIntervalMinutes != 5 {
		t.Errorf("期望清理间隔 5 分钟, 实际=%d", cfg.CleanupIntervalMinutes)
	}
	if cfg.MaxLocksPerUser != 50 {
		t.Errorf("期望每用户最大锁数 50, 实际=%d", cfg.MaxLocksPerUser)
	}
}

func TestConfigUpdate(t *testing.T) {
	s := setupTestService(t)

	cfg := DefaultConfig()
	cfg.MaxLocksPerUser = 5
	s.UpdateConfig(cfg)

	got := s.GetConfig()
	if got.MaxLocksPerUser != 5 {
		t.Errorf("期望 max_locks_per_user=5, 实际=%d", got.MaxLocksPerUser)
	}
}

func TestGetStats(t *testing.T) {
	s := setupTestService(t)

	// 创建锁
	lock1, _ := s.Lock(&LockRequest{FilePath: "/docs/1.txt", UserID: "user-001", UserName: "张三"})
	s.Lock(&LockRequest{FilePath: "/docs/2.txt", UserID: "user-002", UserName: "李四"})

	// 释放一个
	s.Unlock(&UnlockRequest{LockID: lock1.ID, UserID: "user-001"})

	stats := s.GetStats()
	if stats.TotalLocks != 2 {
		t.Errorf("期望总锁数 2, 实际=%d", stats.TotalLocks)
	}
	if stats.ActiveLocks != 1 {
		t.Errorf("期望活跃锁 1, 实际=%d", stats.ActiveLocks)
	}
	if stats.ReleasedLocks != 1 {
		t.Errorf("期望已释放锁 1, 实际=%d", stats.ReleasedLocks)
	}
	if stats.ActiveUsers != 1 {
		t.Errorf("期望活跃用户 1, 实际=%d", stats.ActiveUsers)
	}
	if stats.LockedFiles != 1 {
		t.Errorf("期望锁定文件 1, 实际=%d", stats.LockedFiles)
	}
}

func TestCleanupExpired(t *testing.T) {
	// 使用极短的过期时间测试清理
	cfg := DefaultConfig()
	cfg.DefaultDurationMinutes = 0 // 立即过期（0 分钟）
	// 注意: duration=0 会在 newLockInfo 中被替换为默认值，所以用更直接的方式

	s := setupTestService(t)
	// 手动创建一个已过期的锁
	lock, _ := s.Lock(&LockRequest{
		FilePath: "/docs/expired.txt",
		UserID:   "user-001",
		UserName: "张三",
		DurationMinutes: 1,
	})

	// 手动设置过期时间为过去
	s.mu.Lock()
	lock.ExpiresAt = time.Now().Add(-1 * time.Minute)
	s.mu.Unlock()

	// 手动触发清理
	s.cleanupExpired()

	// 验证锁已过期
	got, _ := s.GetLock(lock.ID)
	if got.Status != LockStatusExpired {
		t.Errorf("期望 status=expired, 实际=%s", got.Status)
	}

	// 验证已从索引中移除
	if s.IsFileLocked("/docs/expired.txt") {
		t.Error("过期后文件不应再被锁定")
	}
}

// ===== HTTP Handler 测试 =====

func TestHandler_Lock(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	body := `{"file_path":"/docs/test.txt","user_id":"user-001","user_name":"张三"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/filelock/lock", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("期望状态码 201, 实际=%d: %s", w.Code, w.Body.String())
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("期望 code=0, 实际=%d", resp.Code)
	}
}

func TestHandler_Lock_InvalidBody(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	body := `{"user_id":"user-001"}` // 缺少 file_path
	req := httptest.NewRequest(http.MethodPost, "/api/v1/filelock/lock", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际=%d", w.Code)
	}
}

func TestHandler_Lock_Conflict(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	// 先锁定文件
	s.Lock(&LockRequest{FilePath: "/docs/test.txt", UserID: "user-001", UserName: "张三"})

	// 再尝试锁定
	body := `{"file_path":"/docs/test.txt","user_id":"user-002","user_name":"李四"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/filelock/lock", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("期望状态码 409, 实际=%d", w.Code)
	}
}

func TestHandler_Unlock(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	// 先获取锁
	lock, _ := s.Lock(&LockRequest{
		FilePath: "/docs/test.txt",
		UserID:   "user-001",
		UserName: "张三",
	})

	body := fmt.Sprintf(`{"lock_id":"%s","user_id":"user-001"}`, lock.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/filelock/unlock", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d: %s", w.Code, w.Body.String())
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("期望 code=0, 实际=%d", resp.Code)
	}
}

func TestHandler_Unlock_NotFound(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	body := `{"lock_id":"nonexistent","user_id":"user-001"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/filelock/unlock", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("期望状态码 400, 实际=%d", w.Code)
	}
}

func TestHandler_List(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	// 添加锁
	s.Lock(&LockRequest{FilePath: "/docs/1.txt", UserID: "user-001", UserName: "张三"})
	s.Lock(&LockRequest{FilePath: "/docs/2.txt", UserID: "user-002", UserName: "李四"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filelock/list", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("期望 code=0, 实际=%d", resp.Code)
	}
}

func TestHandler_List_ByUser(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	s.Lock(&LockRequest{FilePath: "/docs/1.txt", UserID: "user-001", UserName: "张三"})
	s.Lock(&LockRequest{FilePath: "/docs/2.txt", UserID: "user-002", UserName: "李四"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filelock/list?user_id=user-001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}
}

func TestHandler_List_ByFile(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	s.Lock(&LockRequest{FilePath: "/docs/test.txt", UserID: "user-001", UserName: "张三"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filelock/list?file_path=/docs/test.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}
}

func TestHandler_GetLock(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	lock, _ := s.Lock(&LockRequest{
		FilePath: "/docs/test.txt",
		UserID:   "user-001",
		UserName: "张三",
	})

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/filelock/locks/%s", lock.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_GetLock_NotFound(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filelock/locks/nonexistent", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("期望状态码 404, 实际=%d", w.Code)
	}
}

func TestHandler_CheckLock(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	// 检查未锁定的文件
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filelock/check/docs/test.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}

	// 锁定文件后检查
	s.Lock(&LockRequest{FilePath: "/docs/locked.txt", UserID: "user-001", UserName: "张三"})

	req = httptest.NewRequest(http.MethodGet, "/api/v1/filelock/check/docs/locked.txt", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}
}

func TestHandler_Stats(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	s.Lock(&LockRequest{FilePath: "/docs/1.txt", UserID: "user-001", UserName: "张三"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filelock/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}
}

func TestHandler_GetConfig(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filelock/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d", w.Code)
	}
}

func TestHandler_UpdateConfig(t *testing.T) {
	s := setupTestService(t)
	r := setupTestRouter(t, s)

	body := `{"enabled":true,"default_duration_minutes":60,"cleanup_interval_minutes":10,"max_locks_per_user":100}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/filelock/config", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("期望状态码 200, 实际=%d: %s", w.Code, w.Body.String())
	}

	cfg := s.GetConfig()
	if cfg.DefaultDurationMinutes != 60 {
		t.Errorf("期望 default_duration_minutes=60, 实际=%d", cfg.DefaultDurationMinutes)
	}
}

// ===== 类型常量测试 =====

func TestLockStatusValues(t *testing.T) {
	if LockStatusActive != "active" {
		t.Errorf("期望 'active', 实际=%q", LockStatusActive)
	}
	if LockStatusReleased != "released" {
		t.Errorf("期望 'released', 实际=%q", LockStatusReleased)
	}
	if LockStatusExpired != "expired" {
		t.Errorf("期望 'expired', 实际=%q", LockStatusExpired)
	}
}

// ===== 启停测试 =====

func TestStartStop(t *testing.T) {
	s := setupTestService(t)

	// 启动不会 panic
	s.Start()
	s.Start() // 重复启动不报错

	// 停止不会 panic
	s.Stop()
	s.Stop() // 重复停止不报错
}
