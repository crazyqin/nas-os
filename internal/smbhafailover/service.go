// Package smbhafailover 提供 SMB 有状态高可用故障转移功能。
package smbhafailover

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// FailoverManager SMB HA 故障转移管理器
type FailoverManager struct {
	mu        sync.RWMutex
	sessions  map[string]*SessionState
	snapshots map[string]*Snapshot
	state     *FailoverState
}

// NewFailoverManager 创建故障转移管理器
func NewFailoverManager() *FailoverManager {
	return &FailoverManager{
		sessions:  make(map[string]*SessionState),
		snapshots: make(map[string]*Snapshot),
		state: &FailoverState{
			Status:     FailoverStatusIdle,
			ActiveNode: "node-1",
		},
	}
}

// RegisterSession 注册新会话
func (m *FailoverManager) RegisterSession(s *SessionState) error {
	if s == nil {
		return fmt.Errorf("session is nil")
	}
	if s.SessionID == "" {
		return fmt.Errorf("session ID is required")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s.LastAccess = time.Now()
	m.sessions[s.SessionID] = s
	return nil
}

// GetSession 获取会话
func (m *FailoverManager) GetSession(sessionID string) (*SessionState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}
	return s, nil
}

// ListSessions 列出所有会话
func (m *FailoverManager) ListSessions() []*SessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*SessionState, 0, len(m.sessions))
	for _, s := range m.sessions {
		result = append(result, s)
	}
	return result
}

// UpdateSessionAccess 更新会话最后访问时间
func (m *FailoverManager) UpdateSessionAccess(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[sessionID]
	if !ok {
		return fmt.Errorf("session not found: %s", sessionID)
	}
	s.LastAccess = time.Now()
	return nil
}

// RemoveSession 移除会话
func (m *FailoverManager) RemoveSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionID)
}

// CreateSnapshot 创建会话状态快照
func (m *FailoverManager) CreateSnapshot(sourceNode string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.Status = FailoverStatusCapture

	sessions := make([]*SessionState, 0, len(m.sessions))
	for _, s := range m.sessions {
		copy := *s
		sessions = append(sessions, &copy)
	}

	snap := &Snapshot{
		ID:         generateID(),
		CreatedAt:  time.Now(),
		Sessions:   sessions,
		Status:     SnapshotStatusActive,
		SourceNode: sourceNode,
	}
	m.snapshots[snap.ID] = snap
	m.state.Status = FailoverStatusIdle
	m.state.SnapshotID = snap.ID
	return snap, nil
}

// GetSnapshot 获取快照
func (m *FailoverManager) GetSnapshot(id string) (*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", id)
	}
	return s, nil
}

// RestoreSnapshot 从快照恢复会话
func (m *FailoverManager) RestoreSnapshot(snapshotID string) ([]*SessionState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[snapshotID]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	m.state.Status = FailoverStatusRestore
	restored := make([]*SessionState, 0, len(snap.Sessions))
	for _, s := range snap.Sessions {
		copy := *s
		copy.LastAccess = time.Now()
		m.sessions[s.SessionID] = &copy
		restored = append(restored, &copy)
	}

	snap.Status = SnapshotStatusRestored
	m.state.Status = FailoverStatusCompleted
	now := time.Now()
	m.state.LastFail = &now
	return restored, nil
}

// GetState 获取故障转移状态
func (m *FailoverManager) GetState() *FailoverState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	copy := *m.state
	return &copy
}

// SetActiveNode 设置活跃节点
func (m *FailoverManager) SetActiveNode(node string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.ActiveNode = node
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}