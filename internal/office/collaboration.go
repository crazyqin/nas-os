// Package office 协作编辑引擎
// 实现OT (Operational Transformation) 算法简化版，支持多用户同时编辑
// WebSocket实时同步、文档版本历史、评论和批注
package office

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// ========== OT 操作类型 ==========

// OpType 操作类型.
type OpType string

const (
	OpInsert OpType = "insert" // 插入
	OpDelete OpType = "delete" // 删除
	OpRetain OpType = "retain" // 保留（光标移动）
)

// Operation OT操作.
type Operation struct {
	Type     OpType `json:"type"`             // 操作类型
	Position int    `json:"position"`         // 操作位置
	Text     string `json:"text,omitempty"`   // 插入的文本
	Length   int    `json:"length,omitempty"` // 删除/保留的长度
	UserID   string `json:"user_id"`          // 操作用户ID
	Seq      int64  `json:"seq"`              // 操作序列号
	Parent   int64  `json:"parent"`           // 父操作序列号（用于冲突解决）
}

// OperationResult 操作结果.
type OperationResult struct {
	Applied     bool       `json:"applied"`               // 是否成功应用
	Op          *Operation `json:"op"`                    // 原始操作
	Transformed *Operation `json:"transformed,omitempty"` // 变换后的操作
	Error       string     `json:"error,omitempty"`       // 错误信息
}

// ========== 协作引擎 ==========

// CollabEngine 协作编辑引擎.
type CollabEngine struct {
	mu sync.RWMutex

	// 文档状态
	documents map[string]*CollabDocument // docID -> Document

	// WebSocket 连接
	clients map[string]*WSClient // clientID -> WSClient

	// 配置
	config CollabEngineConfig
}

// CollabEngineConfig 协作引擎配置.
type CollabEngineConfig struct {
	MaxClientsPerDoc int           `json:"max_clients_per_doc"` // 每文档最大客户端数
	AutoSaveInterval time.Duration `json:"auto_save_interval"`  // 自动保存间隔
	MaxVersions      int           `json:"max_versions"`        // 最大版本数
	OpBufferSize     int           `json:"op_buffer_size"`      // 操作缓冲区大小
}

// DefaultCollabEngineConfig 默认配置.
func DefaultCollabEngineConfig() CollabEngineConfig {
	return CollabEngineConfig{
		MaxClientsPerDoc: 50,
		AutoSaveInterval: 30 * time.Second,
		MaxVersions:      100,
		OpBufferSize:     1000,
	}
}

// CollabDocument 协作文档.
type CollabDocument struct {
	DocID     string                `json:"doc_id"`
	Content   string                `json:"content"`
	Version   int64                 `json:"version"`  // 当前文档版本号
	Seq       int64                 `json:"seq"`      // 全局操作序列号
	OpLog     []*Operation          `json:"-"`        // 操作日志
	Versions  []*DocVersionSnapshot `json:"-"`        // 版本快照列表
	Comments  []*DocComment         `json:"comments"` // 评论列表
	CreatedAt time.Time             `json:"created_at"`
	UpdatedAt time.Time             `json:"updated_at"`
	mu        sync.RWMutex          `json:"-"`
}

// DocVersionSnapshot 文档版本快照.
type DocVersionSnapshot struct {
	Version   int64     `json:"version"`
	Content   string    `json:"content"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	Message   string    `json:"message,omitempty"` // 版本备注
	CreatedAt time.Time `json:"created_at"`
}

// DocComment 文档评论.
type DocComment struct {
	CommentID string        `json:"comment_id"`
	DocID     string        `json:"doc_id"`
	UserID    string        `json:"user_id"`
	UserName  string        `json:"user_name"`
	Content   string        `json:"content"`
	Range     *CommentRange `json:"range,omitempty"` // 选区范围
	Resolved  bool          `json:"resolved"`
	Replies   []*DocReply   `json:"replies"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// CommentRange 评论选区.
type CommentRange struct {
	StartOffset int `json:"start_offset"`
	EndOffset   int `json:"end_offset"`
}

// DocReply 评论回复.
type DocReply struct {
	ReplyID   string    `json:"reply_id"`
	UserID    string    `json:"user_id"`
	UserName  string    `json:"user_name"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// ========== WebSocket 客户端 ==========

// WSClient WebSocket客户端.
type WSClient struct {
	ID         string          `json:"id"`
	UserID     string          `json:"user_id"`
	UserName   string          `json:"user_name"`
	DocID      string          `json:"doc_id"`
	Conn       *websocket.Conn `json:"-"`
	RemoteAddr string          `json:"remote_addr"`
	JoinedAt   time.Time       `json:"joined_at"`
	LastPing   time.Time       `json:"last_ping"`
	mu         sync.Mutex      `json:"-"`
}

// WSMessage WebSocket消息.
type WSMessage struct {
	Type    string          `json:"type"`    // 消息类型
	Payload json.RawMessage `json:"payload"` // 消息体
	DocID   string          `json:"doc_id"`  // 文档ID
	From    string          `json:"from"`    // 发送者ID
}

// WSMessageType WebSocket消息类型.
const (
	WSTypeOp          = "operation"    // OT操作
	WSTypeCursor      = "cursor"       // 光标移动
	WSTypeJoin        = "join"         // 加入文档
	WSTypeLeave       = "leave"        // 离开文档
	WSTypeSync        = "sync"         // 同步文档内容
	WSTypeAck         = "ack"          // 操作确认
	WSTypeError       = "error"        // 错误
	WSTypeUserList    = "user_list"    // 用户列表
	WSTypeComment     = "comment"      // 评论
	WSTypeVersionSave = "version_save" // 保存版本
	WSTypePing        = "ping"         // 心跳
	WSTypePong        = "pong"         // 心跳响应
)

// ========== 协作引擎方法 ==========

// NewCollabEngine 创建协作引擎.
func NewCollabEngine(config CollabEngineConfig) *CollabEngine {
	engine := &CollabEngine{
		documents: make(map[string]*CollabDocument),
		clients:   make(map[string]*WSClient),
		config:    config,
	}
	return engine
}

// OpenDocument 打开文档进行协作.
func (e *CollabEngine) OpenDocument(docID, initialContent string) *CollabDocument {
	e.mu.Lock()
	defer e.mu.Unlock()

	if doc, exists := e.documents[docID]; exists {
		return doc
	}

	doc := &CollabDocument{
		DocID:     docID,
		Content:   initialContent,
		Version:   1,
		Seq:       0,
		OpLog:     make([]*Operation, 0, e.config.OpBufferSize),
		Versions:  make([]*DocVersionSnapshot, 0),
		Comments:  make([]*DocComment, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 创建初始版本快照
	doc.Versions = append(doc.Versions, &DocVersionSnapshot{
		Version:   1,
		Content:   initialContent,
		UserID:    "system",
		UserName:  "系统",
		Message:   "初始版本",
		CreatedAt: time.Now(),
	})

	e.documents[docID] = doc
	return doc
}

// GetDocument 获取协作文档.
func (e *CollabEngine) GetDocument(docID string) (*CollabDocument, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	doc, exists := e.documents[docID]
	if !exists {
		return nil, errors.New("文档不存在")
	}
	return doc, nil
}

// CloseDocument 关闭文档.
func (e *CollabEngine) CloseDocument(docID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.documents[docID]; !exists {
		return errors.New("文档不存在")
	}

	// 断开所有连接到此文档的客户端
	for _, client := range e.clients {
		if client.DocID == docID {
			client.Conn.Close()
			delete(e.clients, client.ID)
		}
	}

	delete(e.documents, docID)
	return nil
}

// ========== OT 操作处理 ==========

// ApplyOperation 应用OT操作.
func (e *CollabEngine) ApplyOperation(docID string, op *Operation) (*OperationResult, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, exists := e.documents[docID]
	if !exists {
		return nil, errors.New("文档不存在")
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()

	// 验证操作
	if err := e.validateOperation(doc, op); err != nil {
		return &OperationResult{
			Applied: false,
			Op:      op,
			Error:   err.Error(),
		}, nil
	}

	// 变换操作（处理并发冲突）
	transformed := e.transformOperation(doc, op)

	// 应用操作到文档内容
	newContent, err := e.applyOp(doc.Content, transformed)
	if err != nil {
		return &OperationResult{
			Applied: false,
			Op:      op,
			Error:   err.Error(),
		}, nil
	}

	// 更新文档状态
	doc.Seq++
	transformed.Seq = doc.Seq
	doc.Content = newContent
	doc.OpLog = append(doc.OpLog, transformed)
	doc.UpdatedAt = time.Now()

	// 广播操作到其他客户端
	e.broadcastOperation(docID, transformed, op.UserID)

	return &OperationResult{
		Applied:     true,
		Op:          op,
		Transformed: transformed,
	}, nil
}

// validateOperation 验证操作.
func (e *CollabEngine) validateOperation(doc *CollabDocument, op *Operation) error {
	contentLen := len([]rune(doc.Content))

	switch op.Type {
	case OpInsert:
		if op.Text == "" {
			return errors.New("插入操作的文本不能为空")
		}
		if op.Position < 0 || op.Position > contentLen {
			return fmt.Errorf("插入位置 %d 超出范围 [0, %d]", op.Position, contentLen)
		}
	case OpDelete:
		if op.Length <= 0 {
			return errors.New("删除长度必须大于 0")
		}
		if op.Position < 0 || op.Position >= contentLen {
			return fmt.Errorf("删除位置 %d 超出范围 [0, %d)", op.Position, contentLen)
		}
		if op.Position+op.Length > contentLen {
			return fmt.Errorf("删除范围 [%d, %d) 超出文档长度 %d", op.Position, op.Position+op.Length, contentLen)
		}
	case OpRetain:
		if op.Position < 0 || op.Position > contentLen {
			return fmt.Errorf("保留位置 %d 超出范围 [0, %d]", op.Position, contentLen)
		}
	default:
		return fmt.Errorf("未知的操作类型: %s", op.Type)
	}

	return nil
}

// transformOperation OT变换操作.
// 简化版OT：当两个操作冲突时，根据序列号优先级进行变换.
func (e *CollabEngine) transformOperation(doc *CollabDocument, op *Operation) *Operation {
	if len(doc.OpLog) == 0 {
		return op
	}

	transformed := &Operation{
		Type:     op.Type,
		Position: op.Position,
		Text:     op.Text,
		Length:   op.Length,
		UserID:   op.UserID,
		Parent:   doc.Seq,
	}

	// 遍历日志中在 parent 之后的操作进行变换
	for _, logOp := range doc.OpLog {
		if logOp.Seq <= op.Parent {
			continue
		}
		if logOp.UserID == op.UserID {
			continue // 同一用户的操作不需要变换
		}

		transformed = transformPair(transformed, logOp)
	}

	return transformed
}

// transformPair 变换一对操作.
func transformPair(op, against *Operation) *Operation {
	result := &Operation{
		Type:     op.Type,
		Position: op.Position,
		Text:     op.Text,
		Length:   op.Length,
		UserID:   op.UserID,
		Parent:   op.Parent,
	}

	switch {
	case op.Type == OpInsert && against.Type == OpInsert:
		// 两个插入：如果 against 在 op 之前，op 位置需要后移
		if against.Position <= op.Position {
			result.Position += len([]rune(against.Text))
		}

	case op.Type == OpInsert && against.Type == OpDelete:
		// 插入 vs 删除
		if against.Position < op.Position {
			deleted := against.Length
			if against.Position+deleted > op.Position {
				deleted = op.Position - against.Position
			}
			result.Position -= deleted
		}

	case op.Type == OpDelete && against.Type == OpInsert:
		// 删除 vs 插入
		if against.Position <= op.Position {
			result.Position += len([]rune(against.Text))
		}

	case op.Type == OpDelete && against.Type == OpDelete:
		// 两个删除
		if against.Position < op.Position {
			if against.Position+against.Length <= op.Position {
				result.Position -= against.Length
			} else {
				overlap := (against.Position + against.Length) - op.Position
				result.Position = against.Position
				result.Length -= overlap
				if result.Length < 0 {
					result.Length = 0
				}
			}
		}
	}

	return result
}

// applyOp 应用单个操作到内容.
func (e *CollabEngine) applyOp(content string, op *Operation) (string, error) {
	runes := []rune(content)

	switch op.Type {
	case OpInsert:
		text := []rune(op.Text)
		pos := op.Position
		if pos > len(runes) {
			pos = len(runes)
		}
		newRunes := make([]rune, 0, len(runes)+len(text))
		newRunes = append(newRunes, runes[:pos]...)
		newRunes = append(newRunes, text...)
		newRunes = append(newRunes, runes[pos:]...)
		return string(newRunes), nil

	case OpDelete:
		start := op.Position
		end := start + op.Length
		if start < 0 {
			start = 0
		}
		if end > len(runes) {
			end = len(runes)
		}
		newRunes := make([]rune, 0, len(runes)-(end-start))
		newRunes = append(newRunes, runes[:start]...)
		newRunes = append(newRunes, runes[end:]...)
		return string(newRunes), nil

	case OpRetain:
		return content, nil // 不改变内容

	default:
		return content, fmt.Errorf("未知的操作类型: %s", op.Type)
	}
}

// ========== 版本历史 ==========

// SaveVersion 保存当前版本快照.
func (e *CollabEngine) SaveVersion(docID, userID, userName, message string) (*DocVersionSnapshot, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, exists := e.documents[docID]
	if !exists {
		return nil, errors.New("文档不存在")
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()

	doc.Version++
	snapshot := &DocVersionSnapshot{
		Version:   doc.Version,
		Content:   doc.Content,
		UserID:    userID,
		UserName:  userName,
		Message:   message,
		CreatedAt: time.Now(),
	}

	doc.Versions = append(doc.Versions, snapshot)

	// 限制版本数
	if len(doc.Versions) > e.config.MaxVersions {
		doc.Versions = doc.Versions[len(doc.Versions)-e.config.MaxVersions:]
	}

	return snapshot, nil
}

// GetVersionHistory 获取版本历史.
func (e *CollabEngine) GetVersionHistory(docID string, limit, offset int) ([]*DocVersionSnapshot, int, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	doc, exists := e.documents[docID]
	if !exists {
		return nil, 0, errors.New("文档不存在")
	}

	doc.mu.RLock()
	defer doc.mu.RUnlock()

	total := len(doc.Versions)
	if offset >= total {
		return []*DocVersionSnapshot{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	// 返回最新的版本在前
	result := make([]*DocVersionSnapshot, 0, end-offset)
	for i := total - 1 - offset; i >= total-1-(end-1) && i >= 0; i-- {
		result = append(result, doc.Versions[i])
	}

	return result, total, nil
}

// GetVersion 获取指定版本.
func (e *CollabEngine) GetVersion(docID string, version int64) (*DocVersionSnapshot, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	doc, exists := e.documents[docID]
	if !exists {
		return nil, errors.New("文档不存在")
	}

	doc.mu.RLock()
	defer doc.mu.RUnlock()

	for _, v := range doc.Versions {
		if v.Version == version {
			return v, nil
		}
	}

	return nil, errors.New("版本不存在")
}

// RestoreVersion 恢复到指定版本.
func (e *CollabEngine) RestoreVersion(docID string, version int64, userID, userName string) error {
	e.mu.Lock()

	doc, exists := e.documents[docID]
	if !exists {
		e.mu.Unlock()
		return errors.New("文档不存在")
	}

	doc.mu.Lock()

	var target *DocVersionSnapshot
	for _, v := range doc.Versions {
		if v.Version == version {
			target = v
			break
		}
	}

	if target == nil {
		doc.mu.Unlock()
		e.mu.Unlock()
		return errors.New("版本不存在")
	}

	// 恢复内容并创建新版本
	doc.Content = target.Content
	doc.Version++
	snapshot := &DocVersionSnapshot{
		Version:   doc.Version,
		Content:   doc.Content,
		UserID:    userID,
		UserName:  userName,
		Message:   fmt.Sprintf("恢复到版本 %d", version),
		CreatedAt: time.Now(),
	}
	doc.Versions = append(doc.Versions, snapshot)
	doc.UpdatedAt = time.Now()

	// 读取需要广播的数据
	broadcastContent := doc.Content
	broadcastVersion := doc.Version

	doc.mu.Unlock()
	e.mu.Unlock()

	// 广播同步（不持有锁）
	e.broadcastSyncContent(docID, broadcastContent, broadcastVersion)

	return nil
}

// ========== 评论和批注 ==========

// AddComment 添加评论.
func (e *CollabEngine) AddComment(docID, userID, userName, content string, rng *CommentRange) (*DocComment, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, exists := e.documents[docID]
	if !exists {
		return nil, errors.New("文档不存在")
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()

	comment := &DocComment{
		CommentID: uuid.New().String(),
		DocID:     docID,
		UserID:    userID,
		UserName:  userName,
		Content:   content,
		Range:     rng,
		Resolved:  false,
		Replies:   make([]*DocReply, 0),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	doc.Comments = append(doc.Comments, comment)

	// 广播评论
	e.broadcastComment(docID, comment)

	return comment, nil
}

// GetComments 获取文档评论.
func (e *CollabEngine) GetComments(docID string) ([]*DocComment, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	doc, exists := e.documents[docID]
	if !exists {
		return nil, errors.New("文档不存在")
	}

	doc.mu.RLock()
	defer doc.mu.RUnlock()

	comments := make([]*DocComment, len(doc.Comments))
	copy(comments, doc.Comments)
	return comments, nil
}

// ResolveComment 解决评论.
func (e *CollabEngine) ResolveComment(docID, commentID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, exists := e.documents[docID]
	if !exists {
		return errors.New("文档不存在")
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()

	for _, c := range doc.Comments {
		if c.CommentID == commentID {
			c.Resolved = true
			c.UpdatedAt = time.Now()
			return nil
		}
	}

	return errors.New("评论不存在")
}

// ReplyComment 回复评论.
func (e *CollabEngine) ReplyComment(docID, commentID, userID, userName, content string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, exists := e.documents[docID]
	if !exists {
		return errors.New("文档不存在")
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()

	for _, c := range doc.Comments {
		if c.CommentID == commentID {
			reply := &DocReply{
				ReplyID:   uuid.New().String(),
				UserID:    userID,
				UserName:  userName,
				Content:   content,
				CreatedAt: time.Now(),
			}
			c.Replies = append(c.Replies, reply)
			c.UpdatedAt = time.Now()
			return nil
		}
	}

	return errors.New("评论不存在")
}

// DeleteComment 删除评论.
func (e *CollabEngine) DeleteComment(docID, commentID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, exists := e.documents[docID]
	if !exists {
		return errors.New("文档不存在")
	}

	doc.mu.Lock()
	defer doc.mu.Unlock()

	for i, c := range doc.Comments {
		if c.CommentID == commentID {
			doc.Comments = append(doc.Comments[:i], doc.Comments[i+1:]...)
			return nil
		}
	}

	return errors.New("评论不存在")
}

// ========== WebSocket 管理 ==========

// AddClient 添加 WebSocket 客户端.
func (e *CollabEngine) AddClient(client *WSClient) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	doc, exists := e.documents[client.DocID]
	if !exists {
		return errors.New("文档不存在")
	}

	// 检查客户端数限制
	count := 0
	for _, c := range e.clients {
		if c.DocID == client.DocID {
			count++
		}
	}
	if count >= e.config.MaxClientsPerDoc {
		return errors.New("已达到文档最大客户端数限制")
	}

	e.clients[client.ID] = client

	// 更新最后活动
	doc.mu.Lock()
	doc.UpdatedAt = time.Now()
	doc.mu.Unlock()

	// 通知其他客户端
	e.broadcastUserList(client.DocID)

	return nil
}

// RemoveClient 移除 WebSocket 客户端.
func (e *CollabEngine) RemoveClient(clientID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	client, exists := e.clients[clientID]
	if !exists {
		return nil
	}

	docID := client.DocID
	delete(e.clients, clientID)

	// 关闭连接
	client.Conn.Close()

	// 通知其他客户端
	e.broadcastUserList(docID)

	return nil
}

// GetDocClients 获取文档的所有客户端.
func (e *CollabEngine) GetDocClients(docID string) []*WSClient {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var clients []*WSClient
	for _, c := range e.clients {
		if c.DocID == docID {
			clients = append(clients, c)
		}
	}
	return clients
}

// GetOnlineUsers 获取在线用户列表.
func (e *CollabEngine) GetOnlineUsers(docID string) []map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var users []map[string]interface{}
	for _, c := range e.clients {
		if c.DocID == docID {
			users = append(users, map[string]interface{}{
				"user_id":   c.UserID,
				"user_name": c.UserName,
				"client_id": c.ID,
				"joined_at": c.JoinedAt,
				"last_ping": c.LastPing,
			})
		}
	}
	return users
}

// ========== 广播方法 ==========

// broadcastOperation 广播操作到其他客户端.
func (e *CollabEngine) broadcastOperation(docID string, op *Operation, excludeUserID string) {
	msg := WSMessage{
		Type:  WSTypeOp,
		DocID: docID,
		From:  op.UserID,
	}

	payload, _ := json.Marshal(op)
	msg.Payload = payload

	data, _ := json.Marshal(msg)

	for _, client := range e.clients {
		if client.DocID == docID && client.UserID != excludeUserID {
			client.mu.Lock()
			_ = client.Conn.WriteMessage(websocket.TextMessage, data)
			client.mu.Unlock()
		}
	}
}

// broadcastSyncContent 广播同步到所有客户端（使用已有数据，不获取文档锁）.
func (e *CollabEngine) broadcastSyncContent(docID string, content string, version int64) {
	syncData := map[string]interface{}{
		"content": content,
		"version": version,
	}

	msg := WSMessage{
		Type:  WSTypeSync,
		DocID: docID,
	}
	payload, _ := json.Marshal(syncData)
	msg.Payload = payload
	data, _ := json.Marshal(msg)

	for _, client := range e.clients {
		if client.DocID == docID {
			client.mu.Lock()
			_ = client.Conn.WriteMessage(websocket.TextMessage, data)
			client.mu.Unlock()
		}
	}
}

// broadcastUserList 广播用户列表.
func (e *CollabEngine) broadcastUserList(docID string) {
	users := e.getOnlineUsersUnsafe(docID)

	msg := WSMessage{
		Type:  WSTypeUserList,
		DocID: docID,
	}
	payload, _ := json.Marshal(users)
	msg.Payload = payload
	data, _ := json.Marshal(msg)

	for _, client := range e.clients {
		if client.DocID == docID {
			client.mu.Lock()
			_ = client.Conn.WriteMessage(websocket.TextMessage, data)
			client.mu.Unlock()
		}
	}
}

// broadcastComment 广播评论.
func (e *CollabEngine) broadcastComment(docID string, comment *DocComment) {
	msg := WSMessage{
		Type:  WSTypeComment,
		DocID: docID,
		From:  comment.UserID,
	}
	payload, _ := json.Marshal(comment)
	msg.Payload = payload
	data, _ := json.Marshal(msg)

	for _, client := range e.clients {
		if client.DocID == docID && client.UserID != comment.UserID {
			client.mu.Lock()
			_ = client.Conn.WriteMessage(websocket.TextMessage, data)
			client.mu.Unlock()
		}
	}
}

// getOnlineUsersUnsafe 获取在线用户（不加锁，调用者需持有锁）.
func (e *CollabEngine) getOnlineUsersUnsafe(docID string) []map[string]interface{} {
	var users []map[string]interface{}
	for _, c := range e.clients {
		if c.DocID == docID {
			users = append(users, map[string]interface{}{
				"user_id":   c.UserID,
				"user_name": c.UserName,
				"client_id": c.ID,
				"joined_at": c.JoinedAt,
			})
		}
	}
	return users
}

// SendMessage 向客户端发送消息.
func (c *WSClient) SendMessage(msgType string, payload interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	msg := WSMessage{
		Type:  msgType,
		DocID: c.DocID,
		From:  c.UserID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}
	msg.Payload = data

	msgData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	return c.Conn.WriteMessage(websocket.TextMessage, msgData)
}

// SendError 发送错误消息.
func (c *WSClient) SendError(errMsg string) {
	c.SendMessage(WSTypeError, map[string]string{"error": errMsg})
}

// ========== 文档统计 ==========

// CollabStats 协作统计.
type CollabStats struct {
	DocID           int   `json:"doc_id"`
	OnlineUsers     int   `json:"online_users"`
	Version         int64 `json:"version"`
	OpCount         int   `json:"op_count"`
	CommentCount    int   `json:"comment_count"`
	UnresolvedCount int   `json:"unresolved_comments"`
	ContentLength   int   `json:"content_length"`
}

// GetStats 获取协作文档统计.
func (e *CollabEngine) GetStats(docID string) (*CollabStats, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	doc, exists := e.documents[docID]
	if !exists {
		return nil, errors.New("文档不存在")
	}

	doc.mu.RLock()
	defer doc.mu.RUnlock()

	unresolved := 0
	for _, c := range doc.Comments {
		if !c.Resolved {
			unresolved++
		}
	}

	onlineCount := 0
	for _, c := range e.clients {
		if c.DocID == docID {
			onlineCount++
		}
	}

	return &CollabStats{
		OnlineUsers:     onlineCount,
		Version:         doc.Version,
		OpCount:         len(doc.OpLog),
		CommentCount:    len(doc.Comments),
		UnresolvedCount: unresolved,
		ContentLength:   len([]rune(doc.Content)),
	}, nil
}
