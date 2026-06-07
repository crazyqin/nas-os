package stateful

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SessionStateRegistry 会话状态注册表（支持节点归属查询）
type SessionStateRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*SessionState
}

// SessionState SMB会话状态（用于跨节点迁移）
type SessionState struct {
	SessionID    string            `json:"session_id"`
	ClientIP     string            `json:"client_ip"`
	ClientName   string            `json:"client_name"`
	Username     string            `json:"username"`
	ShareName    string            `json:"share_name"`
	Protocol     string            `json:"protocol"`
	NodeID       string            `json:"node_id"` // 当前归属节点
	OpenedFiles  []string          `json:"opened_files"`
	FileLocks    []FileLockState   `json:"file_locks"`
	OplockLevel  string            `json:"oplock_level"`
	ConnectedAt  time.Time         `json:"connected_at"`
	LastActivity time.Time         `json:"last_activity"`
	RegisteredAt time.Time         `json:"registered_at"`
	ExpiresAt    time.Time         `json:"expires_at"`
	MigrationSeq int               `json:"migration_seq"` // 迁移序列号
	MigratedAt   time.Time         `json:"migrated_at,omitempty"`
	Encrypted    bool              `json:"encrypted"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// FileLockState 文件锁状态
type FileLockState struct {
	FilePath   string    `json:"file_path"`
	LockType   string    `json:"lock_type"` // "read" | "write" | "read-write"
	PID        int       `json:"pid"`
	Offset     int64     `json:"offset"`
	Length     int64     `json:"length"`
	AcquiredAt time.Time `json:"acquired_at"`
}

// NewSessionStateRegistry 创建会话注册表
func NewSessionStateRegistry() *SessionStateRegistry {
	return &SessionStateRegistry{
		sessions: make(map[string]*SessionState),
	}
}

// Add 添加会话
func (r *SessionStateRegistry) Add(session *SessionState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.SessionID] = session
}

// Remove 移除会话
func (r *SessionStateRegistry) Remove(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.sessions, sessionID)
}

// Get 获取会话
func (r *SessionStateRegistry) Get(sessionID string) *SessionState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.sessions[sessionID]
}

// GetByNode 获取指定节点的所有会话
func (r *SessionStateRegistry) GetByNode(nodeID string) []*SessionState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*SessionState
	for _, s := range r.sessions {
		if s.NodeID == nodeID {
			result = append(result, s)
		}
	}
	return result
}

// GetByClient 获取客户端的所有会话
func (r *SessionStateRegistry) GetByClient(clientIP string) []*SessionState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*SessionState
	for _, s := range r.sessions {
		if s.ClientIP == clientIP {
			result = append(result, s)
		}
	}
	return result
}

// GetByShare 获取共享的所有会话
func (r *SessionStateRegistry) GetByShare(shareName string) []*SessionState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []*SessionState
	for _, s := range r.sessions {
		if s.ShareName == shareName {
			result = append(result, s)
		}
	}
	return result
}

// ListAll 列出所有会话
func (r *SessionStateRegistry) ListAll() []*SessionState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]*SessionState, 0, len(r.sessions))
	for _, s := range r.sessions {
		result = append(result, s)
	}
	return result
}

// Size 返回会话总数
func (r *SessionStateRegistry) Size() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.sessions)
}

// CleanupExpired 清理过期会话
func (r *SessionStateRegistry) CleanupExpired(maxAge time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	count := 0
	cutoff := time.Now().Add(-maxAge)
	for id, s := range r.sessions {
		if s.LastActivity.Before(cutoff) {
			delete(r.sessions, id)
			count++
		}
	}
	return count
}

// MarshalJSON 序列化
func (r *SessionStateRegistry) MarshalJSON() ([]byte, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return json.Marshal(r.sessions)
}

// ValidateSessionState 验证会话状态完整性
func ValidateSessionState(s *SessionState) error {
	if s.SessionID == "" {
		return fmt.Errorf("SessionID不能为空")
	}
	if s.ClientIP == "" {
		return fmt.Errorf("ClientIP不能为空")
	}
	if s.NodeID == "" {
		return fmt.Errorf("NodeID不能为空")
	}
	return nil
}
