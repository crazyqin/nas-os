package filelock

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), nil)
}

func setupTestRouter(t *testing.T, m *Manager) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api/v1")
	h := NewHandlers(m)
	h.RegisterRoutes(rg)
	return r
}

func TestAcquireLock(t *testing.T) {
	m := setupTestManager(t)

	tests := []struct {
		name    string
		req     *AcquireRequest
		wantErr bool
	}{
		{
			name: "独占锁",
			req: &AcquireRequest{
				FilePath: "/documents/test.txt",
				UserID:   "user-001",
				UserName: "张三",
				LockType: LockTypeExclusive,
			},
			wantErr: false,
		},
		{
			name: "共享锁",
			req: &AcquireRequest{
				FilePath: "/documents/shared.txt",
				UserID:   "user-002",
				UserName: "李四",
				LockType: LockTypeShared,
			},
			wantErr: false,
		},
		{
			name: "默认锁类型",
			req: &AcquireRequest{
				FilePath: "/documents/default.txt",
				UserID:   "user-003",
				UserName: "王五",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lock, err := m.AcquireLock(tt.req)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if lock.ID == "" {
				t.Error("expected non-empty lock ID")
			}
			if lock.Status != LockStatusActive {
				t.Errorf("expected status active, got %s", lock.Status)
			}
		})
	}
}

func TestAcquireLockConflict(t *testing.T) {
	m := setupTestManager(t)

	// 第一个用户获取独占锁
	_, err := m.AcquireLock(&AcquireRequest{
		FilePath: "/documents/test.txt",
		UserID:   "user-001",
		UserName: "张三",
		LockType: LockTypeExclusive,
	})
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}

	// 第二个用户尝试获取同一文件的独占锁
	_, err = m.AcquireLock(&AcquireRequest{
		FilePath: "/documents/test.txt",
		UserID:   "user-002",
		UserName: "李四",
		LockType: LockTypeExclusive,
	})
	if err == nil {
		t.Error("expected conflict error, got nil")
	}
}

func TestAcquireLockSharedMultiple(t *testing.T) {
	m := setupTestManager(t)

	// 多个用户获取共享锁
	for i := 1; i <= 3; i++ {
		_, err := m.AcquireLock(&AcquireRequest{
			FilePath: "/documents/shared.txt",
			UserID:   fmt.Sprintf("user-%03d", i),
			UserName: fmt.Sprintf("用户%d", i),
			LockType: LockTypeShared,
		})
		if err != nil {
			t.Fatalf("acquire shared lock %d failed: %v", i, err)
		}
	}

	// 验证文件有3个共享锁
	locks := m.GetFileLocks("/documents/shared.txt")
	if len(locks) != 3 {
		t.Errorf("expected 3 shared locks, got %d", len(locks))
	}
}

func TestAcquireLockExclusiveWhenSharedExists(t *testing.T) {
	m := setupTestManager(t)

	// 先获取共享锁
	_, err := m.AcquireLock(&AcquireRequest{
		FilePath: "/documents/test.txt",
		UserID:   "user-001",
		UserName: "张三",
		LockType: LockTypeShared,
	})
	if err != nil {
		t.Fatalf("acquire shared lock failed: %v", err)
	}

	// 尝试获取独占锁
	_, err = m.AcquireLock(&AcquireRequest{
		FilePath: "/documents/test.txt",
		UserID:   "user-002",
		UserName: "李四",
		LockType: LockTypeExclusive,
	})
	if err == nil {
		t.Error("expected error when acquiring exclusive lock with shared locks existing")
	}
}

func TestReleaseLock(t *testing.T) {
	m := setupTestManager(t)

	// 获取锁
	lock, err := m.AcquireLock(&AcquireRequest{
		FilePath: "/documents/test.txt",
		UserID:   "user-001",
		UserName: "张三",
		LockType: LockTypeExclusive,
	})
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// 释放锁
	err = m.ReleaseLock(&ReleaseRequest{
		LockID: lock.ID,
		UserID: "user-001",
	})
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}

	// 验证锁已释放
	lock, err = m.GetLock(lock.ID)
	if err != nil {
		t.Fatalf("get lock failed: %v", err)
	}
	if lock.Status != LockStatusReleased {
		t.Errorf("expected status released, got %s", lock.Status)
	}
}

func TestReleaseLockPermission(t *testing.T) {
	m := setupTestManager(t)

	// 用户1获取锁
	lock, err := m.AcquireLock(&AcquireRequest{
		FilePath: "/documents/test.txt",
		UserID:   "user-001",
		UserName: "张三",
		LockType: LockTypeExclusive,
	})
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// 用户2尝试释放用户1的锁
	err = m.ReleaseLock(&ReleaseRequest{
		LockID: lock.ID,
		UserID: "user-002",
	})
	if err == nil {
		t.Error("expected permission error, got nil")
	}
}

func TestRenewLock(t *testing.T) {
	m := setupTestManager(t)

	// 获取锁
	lock, err := m.AcquireLock(&AcquireRequest{
		FilePath: "/documents/test.txt",
		UserID:   "user-001",
		UserName: "张三",
		LockType: LockTypeExclusive,
		Duration: 30,
	})
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	originalExpiry := lock.ExpiresAt

	// 续期
	lock, err = m.RenewLock(&RenewRequest{
		LockID:   lock.ID,
		UserID:   "user-001",
		Duration: 60,
	})
	if err != nil {
		t.Fatalf("renew failed: %v", err)
	}

	// 验证过期时间已更新
	if !lock.ExpiresAt.After(originalExpiry) {
		t.Error("expected expiry to be extended")
	}
}

func TestForceReleaseLock(t *testing.T) {
	m := setupTestManager(t)

	// 获取锁
	lock, err := m.AcquireLock(&AcquireRequest{
		FilePath: "/documents/test.txt",
		UserID:   "user-001",
		UserName: "张三",
		LockType: LockTypeExclusive,
	})
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// 管理员强制释放
	err = m.ForceReleaseLock(&ForceReleaseRequest{
		LockID:  lock.ID,
		AdminID: "admin-001",
		Reason:  "紧急维护",
	})
	if err != nil {
		t.Fatalf("force release failed: %v", err)
	}

	// 验证锁已强制释放
	lock, err = m.GetLock(lock.ID)
	if err != nil {
		t.Fatalf("get lock failed: %v", err)
	}
	if lock.Status != LockStatusForceReleased {
		t.Errorf("expected status force_released, got %s", lock.Status)
	}
	if lock.ReleasedBy != "admin-001" {
		t.Errorf("expected released by admin-001, got %s", lock.ReleasedBy)
	}
}

func TestIsFileLocked(t *testing.T) {
	m := setupTestManager(t)

	// 文件未锁定
	if m.IsFileLocked("/documents/test.txt") {
		t.Error("expected file not locked")
	}

	// 获取锁
	_, err := m.AcquireLock(&AcquireRequest{
		FilePath: "/documents/test.txt",
		UserID:   "user-001",
		UserName: "张三",
	})
	if err != nil {
		t.Fatalf("acquire failed: %v", err)
	}

	// 文件已锁定
	if !m.IsFileLocked("/documents/test.txt") {
		t.Error("expected file locked")
	}
}

func TestListLocks(t *testing.T) {
	m := setupTestManager(t)

	// 创建多个锁
	for i := 1; i <= 5; i++ {
		_, err := m.AcquireLock(&AcquireRequest{
			FilePath: fmt.Sprintf("/documents/file%d.txt", i),
			UserID:   "user-001",
			UserName: "张三",
		})
		if err != nil {
			t.Fatalf("acquire %d failed: %v", i, err)
		}
	}

	// 列出所有锁
	locks, total := m.ListLocks(&ListLocksRequest{})
	if total != 5 {
		t.Errorf("expected 5 locks, got %d", total)
	}
	if len(locks) != 5 {
		t.Errorf("expected 5 locks in result, got %d", len(locks))
	}

	// 按用户过滤
	locks, total = m.ListLocks(&ListLocksRequest{UserID: "user-001"})
	if total != 5 {
		t.Errorf("expected 5 locks for user-001, got %d", total)
	}

	// 分页
	locks, total = m.ListLocks(&ListLocksRequest{Page: 1, PageSize: 2})
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(locks) != 2 {
		t.Errorf("expected 2 locks per page, got %d", len(locks))
	}
}

func TestGetStats(t *testing.T) {
	m := setupTestManager(t)

	// 创建不同类型的锁
	m.AcquireLock(&AcquireRequest{
		FilePath: "/docs/1.txt",
		UserID:   "user-001",
		UserName: "张三",
		LockType: LockTypeExclusive,
	})
	m.AcquireLock(&AcquireRequest{
		FilePath: "/docs/2.txt",
		UserID:   "user-001",
		UserName: "张三",
		LockType: LockTypeShared,
	})
	m.AcquireLock(&AcquireRequest{
		FilePath: "/docs/2.txt",
		UserID:   "user-002",
		UserName: "李四",
		LockType: LockTypeShared,
	})

	stats := m.GetStats()
	if stats.ActiveLocks != 3 {
		t.Errorf("expected 3 active locks, got %d", stats.ActiveLocks)
	}
	if stats.ExclusiveLocks != 1 {
		t.Errorf("expected 1 exclusive lock, got %d", stats.ExclusiveLocks)
	}
	if stats.SharedLocks != 2 {
		t.Errorf("expected 2 shared locks, got %d", stats.SharedLocks)
	}
}

func TestGetHistory(t *testing.T) {
	m := setupTestManager(t)

	// 创建并释放锁
	lock, _ := m.AcquireLock(&AcquireRequest{
		FilePath: "/docs/test.txt",
		UserID:   "user-001",
		UserName: "张三",
	})
	m.ReleaseLock(&ReleaseRequest{
		LockID: lock.ID,
		UserID: "user-001",
	})

	history := m.GetHistory(10)
	if len(history) < 2 {
		t.Errorf("expected at least 2 history entries, got %d", len(history))
	}
}

func TestPolicyManagement(t *testing.T) {
	m := setupTestManager(t)

	// 获取默认策略
	policy := m.GetPolicy()
	if !policy.Enabled {
		t.Error("expected policy to be enabled")
	}

	// 更新策略
	newPolicy := DefaultLockPolicy()
	newPolicy.MaxLocksPerUser = 5
	m.UpdatePolicy(newPolicy)

	policy = m.GetPolicy()
	if policy.MaxLocksPerUser != 5 {
		t.Errorf("expected max locks per user 5, got %d", policy.MaxLocksPerUser)
	}
}

func TestMaxLocksPerUser(t *testing.T) {
	policy := DefaultLockPolicy()
	policy.MaxLocksPerUser = 2
	m := NewManager(zap.NewNop(), policy)

	// 获取2个锁
	for i := 1; i <= 2; i++ {
		_, err := m.AcquireLock(&AcquireRequest{
			FilePath: fmt.Sprintf("/docs/%d.txt", i),
			UserID:   "user-001",
			UserName: "张三",
		})
		if err != nil {
			t.Fatalf("acquire %d failed: %v", i, err)
		}
	}

	// 第3个应该失败
	_, err := m.AcquireLock(&AcquireRequest{
		FilePath: "/docs/3.txt",
		UserID:   "user-001",
		UserName: "张三",
	})
	if err == nil {
		t.Error("expected error when exceeding max locks per user")
	}
}

func TestDisabledPolicy(t *testing.T) {
	policy := DefaultLockPolicy()
	policy.Enabled = false
	m := NewManager(zap.NewNop(), policy)

	_, err := m.AcquireLock(&AcquireRequest{
		FilePath: "/docs/test.txt",
		UserID:   "user-001",
		UserName: "张三",
	})
	if err == nil {
		t.Error("expected error when policy disabled")
	}
}

func TestDefaultLockPolicy(t *testing.T) {
	policy := DefaultLockPolicy()

	if !policy.Enabled {
		t.Error("expected policy to be enabled")
	}
	if policy.DefaultLockType != LockTypeExclusive {
		t.Errorf("expected default lock type exclusive, got %s", policy.DefaultLockType)
	}
	if policy.ExclusiveLockMaxDuration != 480 {
		t.Errorf("expected exclusive max duration 480, got %d", policy.ExclusiveLockMaxDuration)
	}
	if policy.SharedLockMaxDuration != 1440 {
		t.Errorf("expected shared max duration 1440, got %d", policy.SharedLockMaxDuration)
	}
}

func TestHandler_AcquireLock(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	body := `{"file_path":"/docs/test.txt","user_id":"user-001","user_name":"张三","lock_type":"exclusive"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/locks/acquire", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp response
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if resp.Code != 0 {
		t.Errorf("expected code 0, got %d", resp.Code)
	}
}

func TestHandler_ListLocks(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/locks", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_ReleaseLock(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 先获取锁
	lock, _ := m.AcquireLock(&AcquireRequest{
		FilePath: "/docs/test.txt",
		UserID:   "user-001",
		UserName: "张三",
	})

	body := fmt.Sprintf(`{"lock_id":"%s","user_id":"user-001"}`, lock.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/locks/release", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_RenewLock(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 先获取锁
	lock, _ := m.AcquireLock(&AcquireRequest{
		FilePath: "/docs/test.txt",
		UserID:   "user-001",
		UserName: "张三",
	})

	body := fmt.Sprintf(`{"lock_id":"%s","user_id":"user-001","duration":60}`, lock.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/locks/renew", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_ForceReleaseLock(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 先获取锁
	lock, _ := m.AcquireLock(&AcquireRequest{
		FilePath: "/docs/test.txt",
		UserID:   "user-001",
		UserName: "张三",
	})

	body := fmt.Sprintf(`{"lock_id":"%s","admin_id":"admin-001","reason":"紧急维护"}`, lock.ID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/locks/force-release", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandler_CheckFileLock(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/locks/check/docs/test.txt", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_GetFileLocks(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 先获取锁
	m.AcquireLock(&AcquireRequest{
		FilePath: "/docs/test.txt",
		UserID:   "user-001",
		UserName: "张三",
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/locks/file/docs/test.txt", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Stats(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/locks/stats", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_History(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/locks/history?limit=10", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestHandler_Policy(t *testing.T) {
	m := setupTestManager(t)
	r := setupTestRouter(t, m)

	// 获取策略
	req := httptest.NewRequest(http.MethodGet, "/api/v1/locks/policy", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// 更新策略
	body := `{"enabled":true,"default_lock_type":"exclusive","exclusive_lock_max_duration":480}`
	req = httptest.NewRequest(http.MethodPut, "/api/v1/locks/policy", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}
