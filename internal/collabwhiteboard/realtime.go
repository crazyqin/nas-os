// Package collabwhiteboard 提供协作白板功能
package collabwhiteboard

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// RealtimeEngine 实时协作引擎.
type RealtimeEngine struct {
	mu         sync.RWMutex
	engine     *Engine
	cursors    map[string]map[string]*Cursor // boardID -> userID -> cursor
	clients    map[string]map[string]bool    // boardID -> userID -> connected
	listeners  map[string][]chan Operation   // boardID -> operation channels
	otBuffer   map[string][]OTOperation     // boardID -> OT operations
}

// OTOperation OT操作.
type OTOperation struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // insert, delete, update
	ElementID string    `json:"element_id"`
	Position  int       `json:"position,omitempty"`
	Data      []byte    `json:"data,omitempty"`
	Version   int       `json:"version"`
	UserID    string    `json:"user_id"`
	Timestamp time.Time `json:"timestamp"`
}

// OTTransformResult OT转换结果.
type OTTransformResult struct {
	Transformed OTOperation `json:"transformed"`
	Applied     bool        `json:"applied"`
}

// SyncMessage 同步消息.
type SyncMessage struct {
	Type      string      `json:"type"` // operation, cursor, join, leave, sync
	BoardID   string      `json:"board_id"`
	UserID    string      `json:"user_id"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// SyncState 同步状态.
type SyncState struct {
	BoardID       string `json:"board_id"`
	Version       int    `json:"version"`
	ConnectedUsers int   `json:"connected_users"`
	LastOperation time.Time `json:"last_operation"`
}

// NewRealtimeEngine 创建实时协作引擎.
func NewRealtimeEngine(engine *Engine) *RealtimeEngine {
	return &RealtimeEngine{
		engine:    engine,
		cursors:   make(map[string]map[string]*Cursor),
		clients:   make(map[string]map[string]bool),
		listeners: make(map[string][]chan Operation),
		otBuffer:  make(map[string][]OTOperation),
	}
}

// JoinBoard 加入白板.
func (re *RealtimeEngine) JoinBoard(boardID, userID string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	// 验证白板存在
	if _, err := re.engine.GetBoard(boardID); err != nil {
		return err
	}

	// 初始化
	if re.clients[boardID] == nil {
		re.clients[boardID] = make(map[string]bool)
	}
	if re.cursors[boardID] == nil {
		re.cursors[boardID] = make(map[string]*Cursor)
	}
	if re.listeners[boardID] == nil {
		re.listeners[boardID] = make([]chan Operation, 0)
	}
	if re.otBuffer[boardID] == nil {
		re.otBuffer[boardID] = make([]OTOperation, 0)
	}

	re.clients[boardID][userID] = true

	return nil
}

// LeaveBoard 离开白板.
func (re *RealtimeEngine) LeaveBoard(boardID, userID string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	if re.clients[boardID] != nil {
		delete(re.clients[boardID], userID)
	}

	if re.cursors[boardID] != nil {
		delete(re.cursors[boardID], userID)
	}

	return nil
}

// UpdateCursor 更新光标位置.
func (re *RealtimeEngine) UpdateCursor(boardID string, update CursorUpdate) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	if re.cursors[boardID] == nil {
		re.cursors[boardID] = make(map[string]*Cursor)
	}

	re.cursors[boardID][update.UserID] = &Cursor{
		UserID:    update.UserID,
		Username:  update.Username,
		X:         update.X,
		Y:         update.Y,
		Color:     update.Color,
		UpdatedAt: time.Now(),
	}

	return nil
}

// GetCursors 获取所有光标.
func (re *RealtimeEngine) GetCursors(boardID string) ([]*Cursor, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	cursors, ok := re.cursors[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	result := make([]*Cursor, 0, len(cursors))
	for _, cursor := range cursors {
		result = append(result, cursor)
	}

	return result, nil
}

// GetConnectedUsers 获取在线用户.
func (re *RealtimeEngine) GetConnectedUsers(boardID string) ([]string, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	clients, ok := re.clients[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	users := make([]string, 0, len(clients))
	for userID := range clients {
		users = append(users, userID)
	}

	return users, nil
}

// BroadcastOperation 广播操作.
func (re *RealtimeEngine) BroadcastOperation(boardID string, op Operation) error {
	re.mu.RLock()
	defer re.mu.RUnlock()

	listeners, ok := re.listeners[boardID]
	if !ok {
		return nil
	}

	for _, ch := range listeners {
		select {
		case ch <- op:
		default:
			// 跳过满的通道
		}
	}

	return nil
}

// Subscribe 订阅操作.
func (re *RealtimeEngine) Subscribe(boardID string) (<-chan Operation, error) {
	re.mu.Lock()
	defer re.mu.Unlock()

	if re.listeners[boardID] == nil {
		re.listeners[boardID] = make([]chan Operation, 0)
	}

	ch := make(chan Operation, 100)
	re.listeners[boardID] = append(re.listeners[boardID], ch)

	return ch, nil
}

// Unsubscribe 取消订阅.
func (re *RealtimeEngine) Unsubscribe(boardID string, ch <-chan Operation) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	listeners, ok := re.listeners[boardID]
	if !ok {
		return nil
	}

	// 找到并移除对应的channel
	newListeners := make([]chan Operation, 0)
	for _, listener := range listeners {
		if listener != ch {
			newListeners = append(newListeners, listener)
		} else {
			close(listener)
		}
	}
	re.listeners[boardID] = newListeners

	return nil
}

// ApplyOT 应用OT操作.
func (re *RealtimeEngine) ApplyOT(boardID string, op OTOperation) (*OTTransformResult, error) {
	re.mu.Lock()
	defer re.mu.Unlock()

	if re.otBuffer[boardID] == nil {
		re.otBuffer[boardID] = make([]OTOperation, 0)
	}

	// 获取当前版本
	currentVersion := len(re.otBuffer[boardID])

	// 如果版本不匹配，需要转换
	if op.Version != currentVersion {
		transformed := re.transformOperation(op, currentVersion)
		re.otBuffer[boardID] = append(re.otBuffer[boardID], transformed)

		return &OTTransformResult{
			Transformed: transformed,
			Applied:     true,
		}, nil
	}

	// 版本匹配，直接应用
	re.otBuffer[boardID] = append(re.otBuffer[boardID], op)

	return &OTTransformResult{
		Transformed: op,
		Applied:     true,
	}, nil
}

// transformOperation 转换操作.
func (re *RealtimeEngine) transformOperation(op OTOperation, targetVersion int) OTOperation {
	transformed := op
	transformed.Version = targetVersion
	transformed.Timestamp = time.Now()

	// 简单的OT转换逻辑
	// 实际应用中需要更复杂的转换算法
	buffer := re.otBuffer[op.ElementID]
	for i := op.Version; i < len(buffer) && i < targetVersion; i++ {
		existing := buffer[i]
		if existing.Type == "insert" && existing.Position <= transformed.Position {
			transformed.Position++
		} else if existing.Type == "delete" && existing.Position < transformed.Position {
			transformed.Position--
		}
	}

	return transformed
}

// GetOTBuffer 获取OT缓冲.
func (re *RealtimeEngine) GetOTBuffer(boardID string) ([]OTOperation, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	buffer, ok := re.otBuffer[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	result := make([]OTOperation, len(buffer))
	copy(result, buffer)

	return result, nil
}

// SyncState 获取同步状态.
func (re *RealtimeEngine) SyncState(boardID string) (*SyncState, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	if _, ok := re.clients[boardID]; !ok {
		return nil, errors.New("白板不存在")
	}

	version := 0
	if buffer, ok := re.otBuffer[boardID]; ok {
		version = len(buffer)
	}

	connectedUsers := 0
	if clients, ok := re.clients[boardID]; ok {
		connectedUsers = len(clients)
	}

	lastOp := time.Time{}
	if operations, err := re.engine.GetOperations(boardID, 1); err == nil && len(operations) > 0 {
		lastOp = operations[len(operations)-1].Timestamp
	}

	return &SyncState{
		BoardID:        boardID,
		Version:        version,
		ConnectedUsers: connectedUsers,
		LastOperation:  lastOp,
	}, nil
}

// SyncFull 全量同步.
func (re *RealtimeEngine) SyncFull(boardID string) ([]byte, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	board, err := re.engine.GetBoard(boardID)
	if err != nil {
		return nil, err
	}

	cursors := make([]*Cursor, 0)
	if boardCursors, ok := re.cursors[boardID]; ok {
		for _, cursor := range boardCursors {
			cursors = append(cursors, cursor)
		}
	}

	users := make([]string, 0)
	if clients, ok := re.clients[boardID]; ok {
		for userID := range clients {
			users = append(users, userID)
		}
	}

	syncData := map[string]interface{}{
		"board":    board,
		"cursors":  cursors,
		"users":    users,
		"version":  0,
	}

	if buffer, ok := re.otBuffer[boardID]; ok {
		syncData["version"] = len(buffer)
	}

	return json.Marshal(syncData)
}

// ResolveConflict 解决冲突.
func (re *RealtimeEngine) ResolveConflict(boardID string, ops []OTOperation) ([]OTOperation, error) {
	re.mu.Lock()
	defer re.mu.Unlock()

	if re.otBuffer[boardID] == nil {
		return nil, errors.New("白板不存在")
	}

	resolved := make([]OTOperation, 0, len(ops))

	for _, op := range ops {
		// 检查是否与现有操作冲突
		conflict := false
		for _, existing := range re.otBuffer[boardID] {
			if existing.ElementID == op.ElementID &&
				existing.Version == op.Version &&
				existing.UserID != op.UserID {
				// 存在冲突，需要转换
				conflict = true
				transformed := re.transformOperation(op, len(re.otBuffer[boardID]))
				resolved = append(resolved, transformed)
				break
			}
		}

		if !conflict {
			resolved = append(resolved, op)
		}
	}

	// 应用所有解决后的操作
	for _, op := range resolved {
		re.otBuffer[boardID] = append(re.otBuffer[boardID], op)
	}

	return resolved, nil
}

// GetConflictInfo 获取冲突信息.
func (re *RealtimeEngine) GetConflictInfo(boardID string) (int, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	if re.otBuffer[boardID] == nil {
		return 0, errors.New("白板不存在")
	}

	// 统计潜在冲突
	conflicts := 0
	buffer := re.otBuffer[boardID]
	for i := 0; i < len(buffer); i++ {
		for j := i + 1; j < len(buffer); j++ {
			if buffer[i].ElementID == buffer[j].ElementID &&
				buffer[i].UserID != buffer[j].UserID &&
				buffer[i].Version == buffer[j].Version {
				conflicts++
			}
		}
	}

	return conflicts, nil
}

// CleanupStaleCursors 清理过期光标.
func (re *RealtimeEngine) CleanupStaleCursors(boardID string, maxAge time.Duration) int {
	re.mu.Lock()
	defer re.mu.Unlock()

	cursors, ok := re.cursors[boardID]
	if !ok {
		return 0
	}

	removed := 0
	now := time.Now()
	for userID, cursor := range cursors {
		if now.Sub(cursor.UpdatedAt) > maxAge {
			delete(cursors, userID)
			removed++
		}
	}

	return removed
}

// GetSyncMessage 获取同步消息.
func (re *RealtimeEngine) GetSyncMessage(boardID, userID string) (*SyncMessage, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	board, err := re.engine.GetBoard(boardID)
	if err != nil {
		return nil, err
	}

	return &SyncMessage{
		Type:      "sync",
		BoardID:   boardID,
		UserID:    userID,
		Data:      board,
		Timestamp: time.Now(),
	}, nil
}

// MarshalSyncMessage 序列化同步消息.
func (re *RealtimeEngine) MarshalSyncMessage(msg SyncMessage) ([]byte, error) {
	return json.Marshal(msg)
}

// UnmarshalSyncMessage 反序列化同步消息.
func (re *RealtimeEngine) UnmarshalSyncMessage(data []byte) (*SyncMessage, error) {
	var msg SyncMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
