// Package team 协同编辑功能
package team

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CollabManager 协同编辑管理器
type CollabManager struct {
	mu            sync.RWMutex
	sessions      map[string]*EditSession       // sessionID -> EditSession
	resourceIndex map[string]map[string]bool    // resourceID -> sessionID set
	operations    map[string][]*EditOperation   // sessionID -> operations
	cursors       map[string]map[string]*CursorPosition // resourceID -> userID -> CursorPosition
	versions      map[string]int64              // resourceID -> version
	configPath    string
	manager       *Manager
	notifier      *Notifier
}

// NewCollabManager 创建协同编辑管理器
func NewCollabManager(configPath string, manager *Manager) *CollabManager {
	cm := &CollabManager{
		sessions:      make(map[string]*EditSession),
		resourceIndex: make(map[string]map[string]bool),
		operations:    make(map[string][]*EditOperation),
		cursors:       make(map[string]map[string]*CursorPosition),
		versions:      make(map[string]int64),
		configPath:    configPath,
		manager:       manager,
		notifier:      NewNotifier(),
	}
	
	// 加载配置
	if configPath != "" {
		cm.loadConfig()
	}
	
	return cm
}

// loadConfig 加载配置
func (cm *CollabManager) loadConfig() error {
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		return nil
	}
	
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return err
	}
	
	var config struct {
		Sessions map[string]*EditSession `json:"sessions"`
		Versions map[string]int64        `json:"versions"`
	}
	
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	
	cm.sessions = config.Sessions
	cm.versions = config.Versions
	
	// 重建索引
	for id, session := range cm.sessions {
		if cm.resourceIndex[session.ResourceID] == nil {
			cm.resourceIndex[session.ResourceID] = make(map[string]bool)
		}
		cm.resourceIndex[session.ResourceID][id] = true
	}
	
	return nil
}

// saveConfig 保存配置
func (cm *CollabManager) saveConfig() error {
	if cm.configPath == "" {
		return nil
	}
	
	cm.mu.RLock()
	config := struct {
		Sessions map[string]*EditSession `json:"sessions"`
		Versions map[string]int64        `json:"versions"`
	}{
		Sessions: cm.sessions,
		Versions: cm.versions,
	}
	cm.mu.RUnlock()
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	if err := os.MkdirAll(filepath.Dir(cm.configPath), 0750); err != nil {
		return err
	}
	
	return os.WriteFile(cm.configPath, data, 0600)
}

// StartSession 开始编辑会话
func (cm *CollabManager) StartSession(resourceType ResourceType, resourceID, resourcePath, userID, username string) (*EditSession, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	// 检查是否已有活跃会话
	if sessions, ok := cm.resourceIndex[resourceID]; ok {
		for sessionID := range sessions {
			if session, ok := cm.sessions[sessionID]; ok && session.IsActive {
				// 复用现有会话
				return session, nil
			}
		}
	}
	
	session := &EditSession{
		ID:           generateID(),
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ResourcePath: resourcePath,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		IsActive:     true,
	}
	
	cm.sessions[session.ID] = session
	cm.operations[session.ID] = make([]*EditOperation, 0)
	
	// 建立资源索引
	if cm.resourceIndex[resourceID] == nil {
		cm.resourceIndex[resourceID] = make(map[string]bool)
	}
	cm.resourceIndex[resourceID][session.ID] = true
	
	// 初始化版本
	if _, ok := cm.versions[resourceID]; !ok {
		cm.versions[resourceID] = 1
	}
	
	// 记录审计日志
	if cm.manager != nil && cm.manager.audit != nil {
		cm.manager.audit.Log(&TeamAuditLog{
			UserID:       userID,
			Username:     username,
			Action:       AuditEditStart,
			ResourceType: string(resourceType),
			ResourceID:   resourceID,
			ResourcePath: resourcePath,
		})
	}
	
	cm.saveConfig()
	return session, nil
}

// EndSession 结束编辑会话
func (cm *CollabManager) EndSession(sessionID, userID, username string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	session, ok := cm.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	
	session.IsActive = false
	session.UpdatedAt = time.Now()
	
	// 清理光标
	if cursors, ok := cm.cursors[session.ResourceID]; ok {
		delete(cursors, userID)
	}
	
	// 记录审计日志
	if cm.manager != nil && cm.manager.audit != nil {
		cm.manager.audit.Log(&TeamAuditLog{
			UserID:       userID,
			Username:     username,
			Action:       AuditEditEnd,
			ResourceType: string(session.ResourceType),
			ResourceID:   session.ResourceID,
			ResourcePath: session.ResourcePath,
		})
	}
	
	cm.saveConfig()
	return nil
}

// ApplyOperation 应用编辑操作
func (cm *CollabManager) ApplyOperation(sessionID, userID, username string, op EditOperation) (*EditOperation, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	session, ok := cm.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	
	if !session.IsActive {
		return nil, &TeamError{Code: 400, Message: "编辑会话已结束"}
	}
	
	// 检查版本号（乐观锁）
	currentVersion := cm.versions[session.ResourceID]
	if op.Version > 0 && op.Version != currentVersion {
		// 版本冲突
		if cm.manager != nil && cm.manager.audit != nil {
			cm.manager.audit.Log(&TeamAuditLog{
				UserID:       userID,
				Username:     username,
				Action:       AuditEditConflict,
				ResourceType: string(session.ResourceType),
				ResourceID:   session.ResourceID,
				Details: map[string]interface{}{
					"client_version": op.Version,
					"server_version": currentVersion,
				},
			})
		}
		
		return nil, ErrEditConflict
	}
	
	// 设置操作属性
	op.ID = generateID()
	op.SessionID = sessionID
	op.UserID = userID
	op.Username = username
	op.Timestamp = time.Now()
	op.Version = currentVersion + 1
	
	// 更新版本
	cm.versions[session.ResourceID] = op.Version
	session.UpdatedAt = time.Now()
	
	// 存储操作
	cm.operations[sessionID] = append(cm.operations[sessionID], &op)

	return &op, nil
}

// UpdateCursor 更新光标位置
func (cm *CollabManager) UpdateCursor(sessionID, userID, username string, position int64, selection *Selection) (*CursorPosition, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	session, ok := cm.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	
	if !session.IsActive {
		return nil, &TeamError{Code: 400, Message: "编辑会话已结束"}
	}
	
	// 初始化光标映射
	if cm.cursors[session.ResourceID] == nil {
		cm.cursors[session.ResourceID] = make(map[string]*CursorPosition)
	}
	
	cursor := &CursorPosition{
		SessionID:  sessionID,
		UserID:     userID,
		Username:   username,
		ResourceID: session.ResourceID,
		Position:   position,
		Selection:  selection,
		UpdatedAt:  time.Now(),
	}
	
	cm.cursors[session.ResourceID][userID] = cursor
	
	return cursor, nil
}

// GetCursors 获取资源上所有用户的光标位置
func (cm *CollabManager) GetCursors(resourceID string) []*CursorPosition {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	cursors := cm.cursors[resourceID]
	if cursors == nil {
		return []*CursorPosition{}
	}
	
	result := make([]*CursorPosition, 0, len(cursors))
	for _, cursor := range cursors {
		result = append(result, cursor)
	}
	
	return result
}

// GetActiveEditors 获取资源的活跃编辑者
func (cm *CollabManager) GetActiveEditors(resourceID string) []string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	editors := make([]string, 0)
	
	sessions := cm.resourceIndex[resourceID]
	for sessionID := range sessions {
		if session, ok := cm.sessions[sessionID]; ok && session.IsActive {
			// 从操作中获取编辑者
			for _, op := range cm.operations[sessionID] {
				found := false
				for _, e := range editors {
					if e == op.Username {
						found = true
						break
					}
				}
				if !found {
					editors = append(editors, op.Username)
				}
			}
		}
	}
	
	return editors
}

// GetSession 获取编辑会话
func (cm *CollabManager) GetSession(sessionID string) (*EditSession, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	session, ok := cm.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

// GetResourceVersion 获取资源当前版本
func (cm *CollabManager) GetResourceVersion(resourceID string) int64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	return cm.versions[resourceID]
}

// SyncDocument 同步文档状态
func (cm *CollabManager) SyncDocument(sessionID, userID, username string) (map[string]interface{}, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	session, ok := cm.sessions[sessionID]
	if !ok {
		return nil, ErrSessionNotFound
	}
	
	// 获取当前版本和操作
	version := cm.versions[session.ResourceID]
	ops := cm.operations[sessionID]
	
	// 获取光标位置
	cursors := cm.cursors[session.ResourceID]
	cursorList := make([]*CursorPosition, 0)
	for _, c := range cursors {
		cursorList = append(cursorList, c)
	}
	
	return map[string]interface{}{
		"version":   version,
		"operations": ops,
		"cursors":   cursorList,
		"is_active": session.IsActive,
	}, nil
}

// SaveDocument 保存文档
func (cm *CollabManager) SaveDocument(sessionID, userID, username string, content []byte) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	session, ok := cm.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	
	// 实际保存逻辑需要调用文件系统
	// 这里只记录审计日志
	if cm.manager != nil && cm.manager.audit != nil {
		cm.manager.audit.Log(&TeamAuditLog{
			UserID:       userID,
			Username:     username,
			Action:       AuditEditSave,
			ResourceType: string(session.ResourceType),
			ResourceID:   session.ResourceID,
			ResourcePath: session.ResourcePath,
			Details: map[string]interface{}{
				"size": len(content),
			},
		})
	}
	
	session.UpdatedAt = time.Now()
	cm.saveConfig()
	
	return nil
}

// ResolveConflict 解决冲突
func (cm *CollabManager) ResolveConflict(sessionID, userID, username string, resolution string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	
	session, ok := cm.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	
	// 记录冲突解决
	if cm.manager != nil && cm.manager.audit != nil {
		cm.manager.audit.Log(&TeamAuditLog{
			UserID:       userID,
			Username:     username,
			Action:       AuditEditConflict,
			ResourceType: string(session.ResourceType),
			ResourceID:   session.ResourceID,
			Details: map[string]interface{}{
				"resolution": resolution,
			},
		})
	}
	
	return nil
}

// GetStats 获取协同编辑统计
func (cm *CollabManager) GetStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	
	activeSessions := 0
	totalOperations := 0
	
	for _, session := range cm.sessions {
		if session.IsActive {
			activeSessions++
		}
	}
	
	for _, ops := range cm.operations {
		totalOperations += len(ops)
	}
	
	return map[string]interface{}{
		"total_sessions":   len(cm.sessions),
		"active_sessions":  activeSessions,
		"total_operations": totalOperations,
		"total_resources":  len(cm.versions),
	}
}

// BroadcastEdit 广播编辑操作（用于WebSocket）
func (cm *CollabManager) BroadcastEdit(sessionID string, op *EditOperation) {
	if cm.notifier != nil {
		cm.notifier.BroadcastToResource(sessionID, &WSMessage{
			Type:      string(WSEventEdit),
			Data:      op,
			Timestamp: time.Now(),
		})
	}
}

// BroadcastCursor 广播光标位置（用于WebSocket）
func (cm *CollabManager) BroadcastCursor(sessionID string, cursor *CursorPosition) {
	if cm.notifier != nil {
		cm.notifier.BroadcastToResource(cursor.ResourceID, &WSMessage{
			Type:      string(WSEventCursor),
			Data:      cursor,
			Timestamp: time.Now(),
		})
	}
}

// CleanupInactiveSessions 清理不活跃会话
func (cm *CollabManager) CleanupInactiveSessions(timeout time.Duration) int {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	now := time.Now()
	count := 0

	for id, session := range cm.sessions {
		if session.IsActive && now.Sub(session.UpdatedAt) > timeout {
			session.IsActive = false
			count++
			// 使用 id 进行清理记录
			cm.logSessionCleanup(id)
		}
	}

	if count > 0 {
		cm.saveConfig()
	}

	return count
}

// logSessionCleanup 记录会话清理
func (cm *CollabManager) logSessionCleanup(sessionID string) {
	// 记录清理日志
}