// Package globalfilelock 测试
package globalfilelock

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// ============================================================
// 类型测试
// ============================================================

func TestLockType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		lockType LockType
		want     bool
	}{
		{"read lock", LockTypeRead, true},
		{"write lock", LockTypeWrite, true},
		{"invalid lock", LockType("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.lockType.IsValid(); got != tt.want {
				t.Errorf("LockType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLockScope_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		lockScope LockScope
		want      bool
	}{
		{"local scope", LockScopeLocal, true},
		{"global scope", LockScopeGlobal, true},
		{"site scope", LockScopeSite, true},
		{"invalid scope", LockScope("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.lockScope.IsValid(); got != tt.want {
				t.Errorf("LockScope.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultLockPolicy(t *testing.T) {
	policy := DefaultLockPolicy()

	if !policy.Enabled {
		t.Error("默认策略应该启用")
	}
	if policy.DefaultLockType != LockTypeWrite {
		t.Errorf("默认锁类型应该是 write，得到 %s", policy.DefaultLockType)
	}
	if policy.DefaultLockScope != LockScopeGlobal {
		t.Errorf("默认锁范围应该是 global，得到 %s", policy.DefaultLockScope)
	}
	if policy.ReadLockMaxDuration != 1440 {
		t.Errorf("读锁最大时长应该是 1440 分钟，得到 %d", policy.ReadLockMaxDuration)
	}
	if policy.WriteLockMaxDuration != 480 {
		t.Errorf("写锁最大时长应该是 480 分钟，得到 %d", policy.WriteLockMaxDuration)
	}
}

func TestValidateAcquireRequest(t *testing.T) {
	tests := []struct {
		name    string
		req     *AcquireLockRequest
		wantErr bool
	}{
		{
			name: "valid request",
			req: &AcquireLockRequest{
				FilePath:   "/test/file.txt",
				HolderID:   "user1",
				HolderName: "User 1",
				LockType:   LockTypeWrite,
				LockScope:  LockScopeGlobal,
			},
			wantErr: false,
		},
		{
			name: "missing file path",
			req: &AcquireLockRequest{
				HolderID:   "user1",
				HolderName: "User 1",
			},
			wantErr: true,
		},
		{
			name: "missing holder ID",
			req: &AcquireLockRequest{
				FilePath:   "/test/file.txt",
				HolderName: "User 1",
			},
			wantErr: true,
		},
		{
			name: "missing holder name",
			req: &AcquireLockRequest{
				FilePath: "/test/file.txt",
				HolderID: "user1",
			},
			wantErr: true,
		},
		{
			name: "site scope without site ID",
			req: &AcquireLockRequest{
				FilePath:   "/test/file.txt",
				HolderID:   "user1",
				HolderName: "User 1",
				LockScope:  LockScopeSite,
			},
			wantErr: true,
		},
		{
			name: "invalid lock type",
			req: &AcquireLockRequest{
				FilePath:   "/test/file.txt",
				HolderID:   "user1",
				HolderName: "User 1",
				LockType:   LockType("invalid"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateAcquireRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateAcquireRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ============================================================
// 管理器测试
// ============================================================

func TestLockManager_AcquireLock(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	req := &AcquireLockRequest{
		FilePath:   "/test/file.txt",
		HolderID:   "user1",
		HolderName: "User 1",
		LockType:   LockTypeWrite,
		LockScope:  LockScopeGlobal,
		Duration:   60,
	}

	lock, err := manager.AcquireLock(req)
	if err != nil {
		t.Fatalf("获取锁失败: %v", err)
	}

	if lock.FilePath != req.FilePath {
		t.Errorf("文件路径不匹配: got %s, want %s", lock.FilePath, req.FilePath)
	}
	if lock.HolderID != req.HolderID {
		t.Errorf("持有者ID不匹配: got %s, want %s", lock.HolderID, req.HolderID)
	}
	if lock.LockType != LockTypeWrite {
		t.Errorf("锁类型不匹配: got %s, want %s", lock.LockType, LockTypeWrite)
	}
	if lock.Status != LockStatusActive {
		t.Errorf("锁状态应该是 active，得到 %s", lock.Status)
	}
}

func TestLockManager_ConflictDetection(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	// 第一个用户获取写锁
	req1 := &AcquireLockRequest{
		FilePath:   "/test/file.txt",
		HolderID:   "user1",
		HolderName: "User 1",
		LockType:   LockTypeWrite,
		Duration:   60,
	}

	_, err := manager.AcquireLock(req1)
	if err != nil {
		t.Fatalf("第一个用户获取锁失败: %v", err)
	}

	// 第二个用户尝试获取同一文件的锁（应该失败）
	req2 := &AcquireLockRequest{
		FilePath:   "/test/file.txt",
		HolderID:   "user2",
		HolderName: "User 2",
		LockType:   LockTypeWrite,
		Duration:   60,
	}

	_, err = manager.AcquireLock(req2)
	if err == nil {
		t.Error("应该检测到锁冲突")
	}
}

func TestLockManager_ReleaseLock(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	// 获取锁
	acqReq := &AcquireLockRequest{
		FilePath:   "/test/file.txt",
		HolderID:   "user1",
		HolderName: "User 1",
		LockType:   LockTypeWrite,
		Duration:   60,
	}

	lock, err := manager.AcquireLock(acqReq)
	if err != nil {
		t.Fatalf("获取锁失败: %v", err)
	}

	// 释放锁
	relReq := &ReleaseLockRequest{
		LockID:   lock.ID,
		HolderID: "user1",
	}

	err = manager.ReleaseLock(relReq)
	if err != nil {
		t.Fatalf("释放锁失败: %v", err)
	}

	// 验证锁已释放
	releasedLock, _ := manager.GetLock(lock.ID)
	if releasedLock.Status != LockStatusReleased {
		t.Errorf("锁状态应该是 released，得到 %s", releasedLock.Status)
	}
}

func TestLockManager_RenewLock(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	// 获取锁
	acqReq := &AcquireLockRequest{
		FilePath:   "/test/file.txt",
		HolderID:   "user1",
		HolderName: "User 1",
		LockType:   LockTypeWrite,
		Duration:   30,
	}

	lock, err := manager.AcquireLock(acqReq)
	if err != nil {
		t.Fatalf("获取锁失败: %v", err)
	}

	originalExpiry := lock.ExpiresAt

	// 续期
	renewReq := &RenewLockRequest{
		LockID:   lock.ID,
		HolderID: "user1",
		Duration: 60,
	}

	renewedLock, err := manager.RenewLock(renewReq)
	if err != nil {
		t.Fatalf("续期失败: %v", err)
	}

	// 验证过期时间已更新
	if !renewedLock.ExpiresAt.After(originalExpiry) {
		t.Error("续期后过期时间应该延长")
	}
}

func TestLockManager_UpgradeDowngrade(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	// 获取读锁
	acqReq := &AcquireLockRequest{
		FilePath:   "/test/file.txt",
		HolderID:   "user1",
		HolderName: "User 1",
		LockType:   LockTypeRead,
		Duration:   60,
	}

	lock, err := manager.AcquireLock(acqReq)
	if err != nil {
		t.Fatalf("获取锁失败: %v", err)
	}

	// 升级为写锁
	upReq := &UpgradeLockRequest{
		LockID:   lock.ID,
		HolderID: "user1",
	}

	upgradedLock, err := manager.UpgradeLock(upReq)
	if err != nil {
		t.Fatalf("升级失败: %v", err)
	}

	if upgradedLock.LockType != LockTypeWrite {
		t.Errorf("升级后应该是写锁，得到 %s", upgradedLock.LockType)
	}

	// 降级为读锁
	downReq := &DowngradeLockRequest{
		LockID:   lock.ID,
		HolderID: "user1",
	}

	downgradedLock, err := manager.DowngradeLock(downReq)
	if err != nil {
		t.Fatalf("降级失败: %v", err)
	}

	if downgradedLock.LockType != LockTypeRead {
		t.Errorf("降级后应该是读锁，得到 %s", downgradedLock.LockType)
	}
}

func TestLockManager_ReadLockSharing(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	// 用户1获取读锁
	req1 := &AcquireLockRequest{
		FilePath:   "/test/file.txt",
		HolderID:   "user1",
		HolderName: "User 1",
		LockType:   LockTypeRead,
		Duration:   60,
	}

	_, err := manager.AcquireLock(req1)
	if err != nil {
		t.Fatalf("用户1获取读锁失败: %v", err)
	}

	// 用户2获取同一文件的读锁（应该成功）
	req2 := &AcquireLockRequest{
		FilePath:   "/test/file.txt",
		HolderID:   "user2",
		HolderName: "User 2",
		LockType:   LockTypeRead,
		Duration:   60,
	}

	_, err = manager.AcquireLock(req2)
	if err != nil {
		t.Fatalf("用户2获取读锁失败: %v", err)
	}

	// 验证文件有两个活跃锁
	locks := manager.GetFileLocks("/test/file.txt")
	if len(locks) != 2 {
		t.Errorf("应该有2个活跃锁，得到 %d", len(locks))
	}
}

func TestLockManager_GetStatistics(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	// 添加一些锁
	for i := 0; i < 5; i++ {
		req := &AcquireLockRequest{
			FilePath:   "/test/file" + string(rune('0'+i)) + ".txt",
			HolderID:   "user1",
			HolderName: "User 1",
			LockType:   LockTypeWrite,
			LockScope:  LockScopeGlobal,
			Duration:   60,
		}
		manager.AcquireLock(req)
	}

	stats := manager.GetStatistics()

	if stats.ActiveLocks != 5 {
		t.Errorf("活跃锁数量应该是 5，得到 %d", stats.ActiveLocks)
	}
	if stats.WriteLocks != 5 {
		t.Errorf("写锁数量应该是 5，得到 %d", stats.WriteLocks)
	}
	if stats.GlobalLocks != 5 {
		t.Errorf("全局锁数量应该是 5，得到 %d", stats.GlobalLocks)
	}
}

func TestLockManager_SiteRegistration(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	site := &SiteInfo{
		ID:      "site1",
		Name:    "Site 1",
		Address: "192.168.1.100",
		Online:  true,
	}

	manager.RegisterSite(site)

	sites := manager.GetSites()
	if len(sites) != 1 {
		t.Fatalf("站点数量应该是 1，得到 %d", len(sites))
	}

	if sites[0].ID != "site1" {
		t.Errorf("站点ID不匹配: got %s, want %s", sites[0].ID, "site1")
	}

	// 测试心跳
	err := manager.Heartbeat("site1")
	if err != nil {
		t.Fatalf("心跳更新失败: %v", err)
	}
}

// ============================================================
// 冲突解决器测试
// ============================================================

func TestConflictResolver_LastWriteWins(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	resolver := NewConflictResolver(manager)

	// 创建冲突场景
	lock1 := &FileLock{
		ID:         "lock1",
		FilePath:   "/test/file.txt",
		HolderID:   "user1",
		HolderName: "User 1",
		LockType:   LockTypeWrite,
		Status:     LockStatusActive,
		AcquiredAt: time.Now().Add(-10 * time.Minute),
	}
	lock2 := &FileLock{
		ID:         "lock2",
		FilePath:   "/test/file.txt",
		HolderID:   "user2",
		HolderName: "User 2",
		LockType:   LockTypeWrite,
		Status:     LockStatusActive,
		AcquiredAt: time.Now(),
	}

	conflict := &LockConflict{
		ID:               "conflict1",
		FilePath:         "/test/file.txt",
		ConflictingLocks: []*FileLock{lock1, lock2},
		DetectedAt:       time.Now(),
		Resolved:         false,
	}

	// 模拟冲突存在
	manager.mu.Lock()
	manager.conflicts = append(manager.conflicts, conflict)
	manager.mu.Unlock()

	// 解决冲突
	req := &ResolveConflictRequest{
		ConflictID: "conflict1",
		Resolution: ResolutionLastWriteWins,
		ResolvedBy: "admin",
	}

	record, err := resolver.ResolveConflict(req)
	if err != nil {
		t.Fatalf("解决冲突失败: %v", err)
	}

	if record.Result != "resolved" {
		t.Errorf("解决结果应该是 resolved，得到 %s", record.Result)
	}

	if record.PreferredLockID != "lock2" {
		t.Errorf("应该保留 lock2，得到 %s", record.PreferredLockID)
	}
}

func TestConflictResolver_ManualResolution(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	resolver := NewConflictResolver(manager)

	lock1 := &FileLock{
		ID:         "lock1",
		FilePath:   "/test/file.txt",
		HolderID:   "user1",
		HolderName: "User 1",
		LockType:   LockTypeWrite,
		Status:     LockStatusActive,
		AcquiredAt: time.Now(),
	}
	lock2 := &FileLock{
		ID:         "lock2",
		FilePath:   "/test/file.txt",
		HolderID:   "user2",
		HolderName: "User 2",
		LockType:   LockTypeWrite,
		Status:     LockStatusActive,
		AcquiredAt: time.Now(),
	}

	conflict := &LockConflict{
		ID:               "conflict1",
		FilePath:         "/test/file.txt",
		ConflictingLocks: []*FileLock{lock1, lock2},
		DetectedAt:       time.Now(),
		Resolved:         false,
	}

	manager.mu.Lock()
	manager.conflicts = append(manager.conflicts, conflict)
	manager.mu.Unlock()

	req := &ResolveConflictRequest{
		ConflictID:      "conflict1",
		Resolution:      ResolutionManual,
		ResolvedBy:      "admin",
		PreferredLockID: "lock1",
		Detail:          "用户1有更高优先级",
	}

	record, err := resolver.ResolveConflict(req)
	if err != nil {
		t.Fatalf("手动解决冲突失败: %v", err)
	}

	if record.PreferredLockID != "lock1" {
		t.Errorf("应该保留 lock1，得到 %s", record.PreferredLockID)
	}
}

func TestConflictResolver_AutoMerge(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	resolver := NewConflictResolver(manager)

	// 两个读锁可以自动合并
	lock1 := &FileLock{
		ID:         "lock1",
		FilePath:   "/test/file.txt",
		HolderID:   "user1",
		HolderName: "User 1",
		LockType:   LockTypeRead,
		Status:     LockStatusActive,
		AcquiredAt: time.Now(),
	}
	lock2 := &FileLock{
		ID:         "lock2",
		FilePath:   "/test/file.txt",
		HolderID:   "user2",
		HolderName: "User 2",
		LockType:   LockTypeRead,
		Status:     LockStatusActive,
		AcquiredAt: time.Now(),
	}

	conflict := &LockConflict{
		ID:               "conflict1",
		FilePath:         "/test/file.txt",
		ConflictingLocks: []*FileLock{lock1, lock2},
		DetectedAt:       time.Now(),
		Resolved:         false,
	}

	manager.mu.Lock()
	manager.conflicts = append(manager.conflicts, conflict)
	manager.mu.Unlock()

	req := &ResolveConflictRequest{
		ConflictID: "conflict1",
		Resolution: ResolutionAutoMerge,
		ResolvedBy: "system",
	}

	record, err := resolver.ResolveConflict(req)
	if err != nil {
		t.Fatalf("自动合并失败: %v", err)
	}

	if record.Result != "merged" {
		t.Errorf("合并结果应该是 merged，得到 %s", record.Result)
	}
}

// ============================================================
// HTTP API 测试
// ============================================================

func TestHandlers_LockEndpoint(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	handlers := NewHandlers(manager)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// 测试获取锁
	lockReq := AcquireLockRequest{
		FilePath:   "/test/api/file.txt",
		HolderID:   "api_user",
		HolderName: "API User",
		LockType:   LockTypeWrite,
		LockScope:  LockScopeGlobal,
		Duration:   60,
	}

	body, _ := json.Marshal(lockReq)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/filelock/lock", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("预期状态码 201，得到 %d", w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Code != 0 {
		t.Errorf("预期响应码 0，得到 %d", resp.Code)
	}

	// 解析返回的锁数据
	lockData, _ := json.Marshal(resp.Data)
	var lock FileLock
	json.Unmarshal(lockData, &lock)

	if lock.FilePath != "/test/api/file.txt" {
		t.Errorf("文件路径不匹配: got %s", lock.FilePath)
	}
}

func TestHandlers_UnlockEndpoint(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	handlers := NewHandlers(manager)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// 先获取锁
	lockReq := AcquireLockRequest{
		FilePath:   "/test/api/file.txt",
		HolderID:   "api_user",
		HolderName: "API User",
		LockType:   LockTypeWrite,
		Duration:   60,
	}

	lockBody, _ := json.Marshal(lockReq)
	lockHTTPReq := httptest.NewRequest(http.MethodPost, "/api/v1/filelock/lock", bytes.NewReader(lockBody))
	lockHTTPReq.Header.Set("Content-Type", "application/json")
	lockW := httptest.NewRecorder()
	mux.ServeHTTP(lockW, lockHTTPReq)

	var lockResp APIResponse
	json.NewDecoder(lockW.Body).Decode(&lockResp)

	lockData, _ := json.Marshal(lockResp.Data)
	var lock FileLock
	json.Unmarshal(lockData, &lock)

	// 释放锁
	unlockReq := ReleaseLockRequest{
		LockID:   lock.ID,
		HolderID: "api_user",
	}

	unlockBody, _ := json.Marshal(unlockReq)
	unlockHTTPReq := httptest.NewRequest(http.MethodPost, "/api/v1/filelock/unlock", bytes.NewReader(unlockBody))
	unlockHTTPReq.Header.Set("Content-Type", "application/json")
	unlockW := httptest.NewRecorder()
	mux.ServeHTTP(unlockW, unlockHTTPReq)

	if unlockW.Code != http.StatusOK {
		t.Errorf("预期状态码 200，得到 %d", unlockW.Code)
	}
}

func TestHandlers_StatusEndpoint(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	handlers := NewHandlers(manager)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	// 测试查询文件锁状态
	req := httptest.NewRequest(http.MethodGet, "/api/v1/filelock/status?file_path=/test/file.txt", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("预期状态码 200，得到 %d", w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Code != 0 {
		t.Errorf("预期响应码 0，得到 %d", resp.Code)
	}
}

func TestHandlers_StatsEndpoint(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	handlers := NewHandlers(manager)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filelock/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("预期状态码 200，得到 %d", w.Code)
	}

	var resp APIResponse
	json.NewDecoder(w.Body).Decode(&resp)

	if resp.Code != 0 {
		t.Errorf("预期响应码 0，得到 %d", resp.Code)
	}
}

func TestHandlers_ConflictsEndpoint(t *testing.T) {
	manager := NewLockManager(nil)
	manager.Start()
	defer manager.Stop()

	handlers := NewHandlers(manager)
	mux := http.NewServeMux()
	handlers.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filelock/conflicts", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("预期状态码 200，得到 %d", w.Code)
	}
}

// ============================================================
// 辅助函数测试
// ============================================================

func TestContainsPath(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		prefix string
		want   bool
	}{
		{"exact match", "/test/file.txt", "/test/file.txt", true},
		{"prefix match", "/test/file.txt", "/test", true},
		{"no match", "/other/file.txt", "/test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsPath(tt.path, tt.prefix); got != tt.want {
				t.Errorf("ContainsPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCountWriteLocks(t *testing.T) {
	locks := []*FileLock{
		{LockType: LockTypeRead},
		{LockType: LockTypeWrite},
		{LockType: LockTypeRead},
		{LockType: LockTypeWrite},
	}

	count := countWriteLocks(locks)
	if count != 2 {
		t.Errorf("预期写锁数量 2，得到 %d", count)
	}
}
