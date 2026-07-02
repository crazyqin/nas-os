// Package sync 全局文件锁
// 跨节点文件锁定机制，冲突检测和自动解决
// Hybrid Share 全局文件锁，对标群晖 Hybrid Share
package sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 常量 ==========

const (
	// LockDefaultTTL 默认锁过期时间.
	LockDefaultTTL = 30 * time.Minute
	// LockMaxTTL 最大锁持有时间.
	LockMaxTTL = 24 * time.Hour
	// LockRenewInterval 锁续期间隔.
	LockRenewInterval = 10 * time.Minute
	// ConflictDetectionWindow 冲突检测窗口.
	ConflictDetectionWindow = 5 * time.Minute
)

// ========== 类型 ==========

// LockState 锁状态.
type LockState string

const (
	LockStateLocked   LockState = "locked"
	LockStateExpired  LockState = "expired"
	LockStateReleased LockState = "released"
	LockStateConflict LockState = "conflict"
)

// LockType 锁类型.
type LockType string

const (
	LockTypeExclusive LockType = "exclusive" // 独占锁（写锁）
	LockTypeShared    LockType = "shared"    // 共享锁（读锁）
)

// ConflictResolution 冲突解决策略.
type ConflictResolution string

const (
	ConflictLastWriterWins  ConflictResolution = "last_writer_wins"
	ConflictFirstWriterWins ConflictResolution = "first_writer_wins"
	ConflictManual          ConflictResolution = "manual"
	ConflictAutoRename      ConflictResolution = "auto_rename"
)

// FileLock 文件锁定义.
type FileLock struct {
	ID          string            `json:"id"`
	FilePath    string            `json:"file_path"`
	NodeID      string            `json:"node_id"`
	NodeName    string            `json:"node_name"`
	LockType    LockType          `json:"lock_type"`
	State       LockState         `json:"state"`
	Token       string            `json:"token"`
	TTL         time.Duration     `json:"ttl"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   time.Time         `json:"expires_at"`
	LastRenewAt time.Time         `json:"last_renew_at"`
	ReleasedAt  *time.Time        `json:"released_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ConflictRecord 冲突记录.
type ConflictRecord struct {
	ID           string             `json:"id"`
	FilePath     string             `json:"file_path"`
	ConflictType string             `json:"conflict_type"` // "write_write", "write_read", "concurrent_edit"
	LockA        string             `json:"lock_a"`        // 锁ID
	LockB        string             `json:"lock_b"`        // 锁ID
	Resolution   ConflictResolution `json:"resolution"`
	ResolvedBy   string             `json:"resolved_by"`
	ResolvedAt   *time.Time         `json:"resolved_at,omitempty"`
	CreatedAt    time.Time          `json:"created_at"`
	Description  string             `json:"description"`
}

// LockRequest 锁请求.
type LockRequest struct {
	FilePath string            `json:"file_path"`
	NodeID   string            `json:"node_id"`
	NodeName string            `json:"node_name"`
	LockType LockType          `json:"lock_type"`
	TTL      time.Duration     `json:"ttl"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NodeInfo 节点信息.
type NodeInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Address   string    `json:"address"`
	Status    string    `json:"status"`
	LastSeen  time.Time `json:"last_seen"`
	LockCount int       `json:"lock_count"`
}

// GlobalFileLockManager 全局文件锁管理器.
type GlobalFileLockManager struct {
	mu               sync.RWMutex
	locks            map[string]*FileLock // lockID -> FileLock
	fileLocks        map[string][]string  // filePath -> []lockID
	nodeLocks        map[string][]string  // nodeID -> []lockID
	conflicts        map[string]*ConflictRecord
	conflictStrategy ConflictResolution
	nodes            map[string]*NodeInfo
	baseDir          string
	stopCh           chan struct{}
}

// NewGlobalFileLockManager 创建全局文件锁管理器.
func NewGlobalFileLockManager(baseDir string, conflictStrategy ConflictResolution) (*GlobalFileLockManager, error) {
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return nil, fmt.Errorf("创建锁目录失败: %w", err)
	}

	if conflictStrategy == "" {
		conflictStrategy = ConflictLastWriterWins
	}

	mgr := &GlobalFileLockManager{
		locks:            make(map[string]*FileLock),
		fileLocks:        make(map[string][]string),
		nodeLocks:        make(map[string][]string),
		conflicts:        make(map[string]*ConflictRecord),
		conflictStrategy: conflictStrategy,
		nodes:            make(map[string]*NodeInfo),
		baseDir:          baseDir,
		stopCh:           make(chan struct{}),
	}

	// 加载已有锁
	if err := mgr.loadState(); err != nil {
		return nil, err
	}

	// 启动过期清理和续约检查
	go mgr.cleanupLoop()

	return mgr, nil
}

// AcquireLock 获取文件锁.
func (m *GlobalFileLockManager) AcquireLock(req LockRequest) (*FileLock, error) {
	if req.FilePath == "" {
		return nil, fmt.Errorf("文件路径不能为空")
	}
	if req.NodeID == "" {
		return nil, fmt.Errorf("节点ID不能为空")
	}
	if req.TTL == 0 {
		req.TTL = LockDefaultTTL
	}
	if req.TTL > LockMaxTTL {
		req.TTL = LockMaxTTL
	}
	if req.LockType == "" {
		req.LockType = LockTypeExclusive
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查冲突
	existingLocks := m.fileLocks[req.FilePath]
	for _, lockID := range existingLocks {
		existing := m.locks[lockID]
		if existing == nil || existing.State != LockStateLocked {
			continue
		}
		if existing.ExpiresAt.Before(time.Now()) {
			continue
		}

		// 检查是否是同一节点的同一文件（续期）
		if existing.NodeID == req.NodeID {
			existing.TTL = req.TTL
			existing.ExpiresAt = time.Now().Add(req.TTL)
			existing.LastRenewAt = time.Now()
			return existing, nil
		}

		// 写写冲突
		if req.LockType == LockTypeExclusive || existing.LockType == LockTypeExclusive {
			conflict := m.createConflict(req.FilePath, lockID, "write_write")
			switch m.conflictStrategy {
			case ConflictLastWriterWins:
				// 释放旧锁
				existing.State = LockStateReleased
				now := time.Now()
				existing.ReleasedAt = &now
				conflict.Resolution = ConflictLastWriterWins
				conflict.ResolvedBy = req.NodeID
				conflict.ResolvedAt = &now
			case ConflictFirstWriterWins:
				return nil, fmt.Errorf("文件 '%s' 已被节点 '%s' 锁定 (冲突: %s)", req.FilePath, existing.NodeName, conflict.ID)
			case ConflictAutoRename:
				// 自动重命名后允许访问
				conflict.Resolution = ConflictAutoRename
				now := time.Now()
				conflict.ResolvedAt = &now
			case ConflictManual:
				return nil, fmt.Errorf("文件 '%s' 存在冲突，请手动解决 (冲突ID: %s)", req.FilePath, conflict.ID)
			}
		}
		// 读读兼容
		if req.LockType == LockTypeShared && existing.LockType == LockTypeShared {
			// 允许多个共享锁
		}
	}

	// 创建新锁
	lock := &FileLock{
		ID:          uuid.New().String(),
		FilePath:    req.FilePath,
		NodeID:      req.NodeID,
		NodeName:    req.NodeName,
		LockType:    req.LockType,
		State:       LockStateLocked,
		Token:       uuid.New().String(),
		TTL:         req.TTL,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(req.TTL),
		LastRenewAt: time.Now(),
		Metadata:    req.Metadata,
	}

	m.locks[lock.ID] = lock
	m.fileLocks[req.FilePath] = append(m.fileLocks[req.FilePath], lock.ID)
	m.nodeLocks[req.NodeID] = append(m.nodeLocks[req.NodeID], lock.ID)

	// 注册节点
	if _, ok := m.nodes[req.NodeID]; !ok {
		m.nodes[req.NodeID] = &NodeInfo{
			ID:       req.NodeID,
			Name:     req.NodeName,
			Status:   "online",
			LastSeen: time.Now(),
		}
	}
	m.nodes[req.NodeID].LockCount++
	m.nodes[req.NodeID].LastSeen = time.Now()

	m.saveState()

	return lock, nil
}

// ReleaseLock 释放文件锁.
func (m *GlobalFileLockManager) ReleaseLock(lockID, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, ok := m.locks[lockID]
	if !ok {
		return fmt.Errorf("锁 '%s' 不存在", lockID)
	}
	if lock.Token != token {
		return fmt.Errorf("令牌无效")
	}
	if lock.State != LockStateLocked {
		return fmt.Errorf("锁已经释放")
	}

	lock.State = LockStateReleased
	now := time.Now()
	lock.ReleasedAt = &now

	// 更新节点锁计数
	if node, ok := m.nodes[lock.NodeID]; ok {
		node.LockCount--
		if node.LockCount < 0 {
			node.LockCount = 0
		}
	}

	m.saveState()
	return nil
}

// RenewLock 续期文件锁.
func (m *GlobalFileLockManager) RenewLock(lockID, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	lock, ok := m.locks[lockID]
	if !ok {
		return fmt.Errorf("锁不存在")
	}
	if lock.Token != token {
		return fmt.Errorf("令牌无效")
	}
	if lock.State != LockStateLocked {
		return fmt.Errorf("锁已释放")
	}

	lock.ExpiresAt = time.Now().Add(lock.TTL)
	lock.LastRenewAt = time.Now()

	m.saveState()
	return nil
}

// GetFileLocks 获取文件的所有锁.
func (m *GlobalFileLockManager) GetFileLocks(filePath string) []*FileLock {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*FileLock
	for _, lockID := range m.fileLocks[filePath] {
		if lock := m.locks[lockID]; lock != nil && lock.State == LockStateLocked {
			result = append(result, lock)
		}
	}
	return result
}

// GetNodeLocks 获取节点的所有锁.
func (m *GlobalFileLockManager) GetNodeLocks(nodeID string) []*FileLock {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*FileLock
	for _, lockID := range m.nodeLocks[nodeID] {
		if lock := m.locks[lockID]; lock != nil && lock.State == LockStateLocked {
			result = append(result, lock)
		}
	}
	return result
}

// ListConflicts 列出所有冲突.
func (m *GlobalFileLockManager) ListConflicts() []*ConflictRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ConflictRecord, 0, len(m.conflicts))
	for _, c := range m.conflicts {
		result = append(result, c)
	}
	return result
}

// ResolveConflict 手动解决冲突.
func (m *GlobalFileLockManager) ResolveConflict(conflictID, resolvedBy string, keepLockID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	conflict, ok := m.conflicts[conflictID]
	if !ok {
		return fmt.Errorf("冲突不存在")
	}

	now := time.Now()
	conflict.ResolvedAt = &now
	conflict.ResolvedBy = resolvedBy
	conflict.Resolution = ConflictManual

	// 释放被丢弃的锁
	if keepLockID != "" {
		for _, lockID := range []string{conflict.LockA, conflict.LockB} {
			if lockID != keepLockID {
				if lock := m.locks[lockID]; lock != nil {
					lock.State = LockStateReleased
					lock.ReleasedAt = &now
				}
			}
		}
	}

	m.saveState()
	return nil
}

// RegisterNode 注册节点.
func (m *GlobalFileLockManager) RegisterNode(id, name, address string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nodes[id] = &NodeInfo{
		ID:       id,
		Name:     name,
		Address:  address,
		Status:   "online",
		LastSeen: time.Now(),
	}
}

// GetNodes 获取所有节点.
func (m *GlobalFileLockManager) GetNodes() []*NodeInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*NodeInfo, 0, len(m.nodes))
	for _, n := range m.nodes {
		result = append(result, n)
	}
	return result
}

// GetStats 获取统计信息.
func (m *GlobalFileLockManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeLocks := 0
	expiredLocks := 0
	for _, l := range m.locks {
		switch l.State {
		case LockStateLocked:
			if l.ExpiresAt.Before(time.Now()) {
				expiredLocks++
			} else {
				activeLocks++
			}
		}
	}

	unresolved := 0
	for _, c := range m.conflicts {
		if c.ResolvedAt == nil {
			unresolved++
		}
	}

	return map[string]interface{}{
		"total_locks":       len(m.locks),
		"active_locks":      activeLocks,
		"expired_locks":     expiredLocks,
		"total_conflicts":   len(m.conflicts),
		"unresolved":        unresolved,
		"nodes":             len(m.nodes),
		"conflict_strategy": m.conflictStrategy,
	}
}

// Stop 停止管理器.
func (m *GlobalFileLockManager) Stop() {
	close(m.stopCh)
}

// ========== 辅助函数 ==========

func (m *GlobalFileLockManager) createConflict(filePath, lockAID, conflictType string) *ConflictRecord {
	lockBID := ""
	for _, lockID := range m.fileLocks[filePath] {
		if lockID != lockAID {
			if lock := m.locks[lockID]; lock != nil && lock.State == LockStateLocked {
				lockBID = lockID
				break
			}
		}
	}

	conflict := &ConflictRecord{
		ID:           uuid.New().String(),
		FilePath:     filePath,
		ConflictType: conflictType,
		LockA:        lockAID,
		LockB:        lockBID,
		CreatedAt:    time.Now(),
		Description:  fmt.Sprintf("%s 冲突: %s", conflictType, filePath),
	}
	m.conflicts[conflict.ID] = conflict
	return conflict
}

func (m *GlobalFileLockManager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpired()
		case <-m.stopCh:
			return
		}
	}
}

func (m *GlobalFileLockManager) cleanupExpired() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, lock := range m.locks {
		if lock.State == LockStateLocked && lock.ExpiresAt.Before(now) {
			lock.State = LockStateExpired
			// 更新节点计数
			if node, ok := m.nodes[lock.NodeID]; ok {
				node.LockCount--
				if node.LockCount < 0 {
					node.LockCount = 0
				}
			}
		}
	}

	// 检测节点离线
	for _, node := range m.nodes {
		if node.Status == "online" && now.Sub(node.LastSeen) > 5*time.Minute {
			node.Status = "offline"
		}
	}
}

func (m *GlobalFileLockManager) saveState() {
	data, _ := json.MarshalIndent(m.locks, "", "  ")
	os.WriteFile(filepath.Join(m.baseDir, "locks.json"), data, 0644)

	data, _ = json.MarshalIndent(m.conflicts, "", "  ")
	os.WriteFile(filepath.Join(m.baseDir, "conflicts.json"), data, 0644)

	data, _ = json.MarshalIndent(m.nodes, "", "  ")
	os.WriteFile(filepath.Join(m.baseDir, "nodes.json"), data, 0644)
}

func (m *GlobalFileLockManager) loadState() error {
	for _, f := range []struct {
		name string
		dst  interface{}
	}{
		{"locks.json", &m.locks},
		{"conflicts.json", &m.conflicts},
		{"nodes.json", &m.nodes},
	} {
		path := filepath.Join(m.baseDir, f.name)
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		json.Unmarshal(data, f.dst)
	}

	// 重建索引
	for _, lock := range m.locks {
		if lock.State == LockStateLocked {
			m.fileLocks[lock.FilePath] = append(m.fileLocks[lock.FilePath], lock.ID)
			m.nodeLocks[lock.NodeID] = append(m.nodeLocks[lock.NodeID], lock.ID)
		}
	}

	return nil
}

// ========== HTTP Handlers ==========

// GlobalFileLockHandlers 全局文件锁HTTP处理器.
type GlobalFileLockHandlers struct {
	manager *GlobalFileLockManager
}

func NewGlobalFileLockHandlers(manager *GlobalFileLockManager) *GlobalFileLockHandlers {
	return &GlobalFileLockHandlers{manager: manager}
}

func (h *GlobalFileLockHandlers) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/locks", h.handleLocks)
	mux.HandleFunc(prefix+"/locks/", h.handleLockByID)
	mux.HandleFunc(prefix+"/files/", h.handleFileLocks)
	mux.HandleFunc(prefix+"/conflicts", h.handleConflicts)
	mux.HandleFunc(prefix+"/conflicts/", h.handleConflictByID)
	mux.HandleFunc(prefix+"/nodes", h.handleNodes)
	mux.HandleFunc(prefix+"/stats", h.handleStats)
}

func (h *GlobalFileLockHandlers) handleLocks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req LockRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		lock, err := h.manager.AcquireLock(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(lock)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *GlobalFileLockHandlers) handleLockByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/filelock/locks/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing lock ID", http.StatusBadRequest)
		return
	}
	lockID := parts[0]

	if len(parts) > 1 {
		action := parts[1]
		switch action {
		case "release":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				Token string `json:"token"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if err := h.manager.ReleaseLock(lockID, req.Token); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "released"})
		case "renew":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				Token string `json:"token"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if err := h.manager.RenewLock(lockID, req.Token); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "renewed"})
		default:
			http.Error(w, "Unknown action", http.StatusBadRequest)
		}
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (h *GlobalFileLockHandlers) handleFileLocks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	filePath := strings.TrimPrefix(r.URL.Path, "/api/v1/filelock/files/")
	locks := h.manager.GetFileLocks(filePath)
	json.NewEncoder(w).Encode(map[string]interface{}{"locks": locks, "file": filePath})
}

func (h *GlobalFileLockHandlers) handleConflicts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	conflicts := h.manager.ListConflicts()
	json.NewEncoder(w).Encode(map[string]interface{}{"conflicts": conflicts})
}

func (h *GlobalFileLockHandlers) handleConflictByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	conflictID := strings.TrimPrefix(r.URL.Path, "/api/v1/filelock/conflicts/")
	var req struct {
		ResolvedBy string `json:"resolved_by"`
		KeepLockID string `json:"keep_lock_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if err := h.manager.ResolveConflict(conflictID, req.ResolvedBy, req.KeepLockID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "resolved"})
}

func (h *GlobalFileLockHandlers) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	nodes := h.manager.GetNodes()
	json.NewEncoder(w).Encode(map[string]interface{}{"nodes": nodes})
}

func (h *GlobalFileLockHandlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.manager.GetStats())
}
