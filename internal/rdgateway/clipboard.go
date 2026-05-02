// Package rdgateway 提供剪贴板双向同步功能
package rdgateway

import (
	"fmt"
	"sync"
	"time"
)

// ClipboardSync 剪贴板同步管理器.
type ClipboardSync struct {
	mu        sync.RWMutex
	clipboards map[string]*ClipboardState // sessionID -> state
	history    []ClipboardEntry
	maxHistory int
}

// ClipboardState 会话剪贴板状态.
type ClipboardState struct {
	SessionID string    `json:"session_id"`
	Content   string    `json:"content"`
	Format    string    `json:"format"` // text, html, image
	UpdatedAt time.Time `json:"updated_at"`
	Source    string    `json:"source"` // local, remote
}

// ClipboardEntry 剪贴板历史条目.
type ClipboardEntry struct {
	SessionID string    `json:"session_id"`
	Format    string    `json:"format"`
	Source    string    `json:"source"`
	Length    int       `json:"length"`
	Timestamp time.Time `json:"timestamp"`
}

// NewClipboardSync 创建剪贴板同步管理器.
func NewClipboardSync(maxHistory int) *ClipboardSync {
	if maxHistory <= 0 {
		maxHistory = 100
	}
	return &ClipboardSync{
		clipboards: make(map[string]*ClipboardState),
		history:    make([]ClipboardEntry, 0),
		maxHistory: maxHistory,
	}
}

// UpdateClipboard 更新会话剪贴板内容.
func (cs *ClipboardSync) UpdateClipboard(sessionID, content, format, source string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.clipboards[sessionID] = &ClipboardState{
		SessionID: sessionID,
		Content:   content,
		Format:    format,
		UpdatedAt: time.Now(),
		Source:    source,
	}

	entry := ClipboardEntry{
		SessionID: sessionID,
		Format:    format,
		Source:    source,
		Length:    len(content),
		Timestamp: time.Now(),
	}

	cs.history = append(cs.history, entry)
	if len(cs.history) > cs.maxHistory {
		cs.history = cs.history[1:]
	}
}

// GetClipboard 获取会话剪贴板内容.
func (cs *ClipboardSync) GetClipboard(sessionID string) (*ClipboardState, error) {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	state, ok := cs.clipboards[sessionID]
	if !ok {
		return nil, fmt.Errorf("clipboard not found for session %q", sessionID)
	}
	return state, nil
}

// GetHistory 获取剪贴板历史.
func (cs *ClipboardSync) GetHistory(sessionID string, limit int) []ClipboardEntry {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	var result []ClipboardEntry
	for i := len(cs.history) - 1; i >= 0; i-- {
		entry := cs.history[i]
		if sessionID != "" && entry.SessionID != sessionID {
			continue
		}
		result = append(result, entry)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// ClearClipboard 清除会话剪贴板.
func (cs *ClipboardSync) ClearClipboard(sessionID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.clipboards, sessionID)
}
