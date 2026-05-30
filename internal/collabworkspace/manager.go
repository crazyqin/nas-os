// Package collabworkspace - Manager 管理器（handlers_test.go 兼容）
package collabworkspace

import (
	"fmt"
	"sync"
	"time"
)

// ==================== Manager 专用类型 ====================

// CollabDocument 协作文档
type CollabDocument struct {
	ID          string         `json:"id"`
	WorkspaceID string         `json:"workspaceId"`
	Title       string         `json:"title"`
	Content     string         `json:"content"`
	ContentType string         `json:"contentType"`
	Version     int            `json:"version"`
	Editors     []ActiveEditor `json:"editors"`
	Comments    []Comment      `json:"comments"`
	CreatedBy   string         `json:"createdBy"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
	Tags        []string       `json:"tags"`
	Locked      bool           `json:"locked"`
	LockedBy    string         `json:"lockedBy,omitempty"`
	Permission  string         `json:"permission"`
}

// ActiveEditor 活跃编辑者
type ActiveEditor struct {
	UserID    string    `json:"userId"`
	UserName  string    `json:"userName"`
	CursorPos int       `json:"cursorPos"`
	Section   string    `json:"section"`
	JoinedAt  time.Time `json:"joinedAt"`
	Color     string    `json:"color"`
}

// Whiteboard 白板
type Whiteboard struct {
	ID            string      `json:"id"`
	WorkspaceID   string      `json:"workspaceId"`
	Title         string      `json:"title"`
	Elements      []WbElement `json:"elements"`
	Collaborators []string    `json:"collaborators"`
	CreatedBy     string      `json:"createdBy"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	Width         int         `json:"width"`
	Height        int         `json:"height"`
	Background    string      `json:"background"`
}

// WbElement 白板元素
type WbElement struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	X       float64           `json:"x"`
	Y       float64           `json:"y"`
	Width   float64           `json:"width"`
	Height  float64           `json:"height"`
	Content string            `json:"content"`
	Color   string            `json:"color"`
	Style   map[string]string `json:"style"`
	Author  string            `json:"author"`
	ZIndex  int               `json:"zIndex"`
}

// WorkspaceStats 工作空间统计
type WorkspaceStats struct {
	TotalWorkspaces  int            `json:"totalWorkspaces"`
	TotalDocuments   int            `json:"totalDocuments"`
	TotalTasks       int            `json:"totalTasks"`
	TotalWhiteboards int            `json:"totalWhiteboards"`
	ActiveUsers      int            `json:"activeUsers"`
	TasksByStatus    map[string]int `json:"tasksByStatus"`
	RecentActivity   []Activity     `json:"recentActivity"`
}

// Manager 自包含管理器
type Manager struct {
	mu          sync.RWMutex
	workspaces  map[string]*Workspace
	documents   map[string]*CollabDocument
	tasks       map[string]*Task
	whiteboards map[string]*Whiteboard
	activities  []Activity
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		workspaces:  make(map[string]*Workspace),
		documents:   make(map[string]*CollabDocument),
		tasks:       make(map[string]*Task),
		whiteboards: make(map[string]*Whiteboard),
	}
}

// CreateWorkspace 创建工作空间
func (m *Manager) CreateWorkspace(ws *Workspace) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ws.ID = fmt.Sprintf("ws-%d", time.Now().UnixNano())
	ws.CreatedAt = time.Now()
	ws.UpdatedAt = time.Now()
	m.workspaces[ws.ID] = ws
	return nil
}

// GetWorkspace 获取工作空间
func (m *Manager) GetWorkspace(id string) (*Workspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ws, ok := m.workspaces[id]
	if !ok {
		return nil, fmt.Errorf("workspace not found: %s", id)
	}
	return ws, nil
}

// ListWorkspaces 列出工作空间
func (m *Manager) ListWorkspaces(userID string, page, pageSize int) ([]Workspace, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Workspace
	for _, ws := range m.workspaces {
		if userID == "" || ws.OwnerID == userID {
			result = append(result, *ws)
		}
	}
	total := len(result)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return result[start:end], total
}

// CreateDocument 创建文档
func (m *Manager) CreateDocument(doc *CollabDocument) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc.ID = fmt.Sprintf("doc-%d", time.Now().UnixNano())
	doc.Version = 1
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = time.Now()
	m.documents[doc.ID] = doc
	return nil
}

// GetDocument 获取文档
func (m *Manager) GetDocument(id string) (*CollabDocument, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	doc, ok := m.documents[id]
	if !ok {
		return nil, fmt.Errorf("document not found: %s", id)
	}
	return doc, nil
}

// UpdateDocument 更新文档
func (m *Manager) UpdateDocument(id, userID, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, ok := m.documents[id]
	if !ok {
		return fmt.Errorf("document not found: %s", id)
	}
	if doc.Locked && doc.LockedBy != userID {
		return fmt.Errorf("document locked by %s", doc.LockedBy)
	}
	doc.Content = content
	doc.Version++
	doc.UpdatedAt = time.Now()
	return nil
}

// ListDocuments 列出文档
func (m *Manager) ListDocuments(workspaceID string, page, pageSize int) ([]CollabDocument, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []CollabDocument
	for _, doc := range m.documents {
		if workspaceID == "" || doc.WorkspaceID == workspaceID {
			result = append(result, *doc)
		}
	}
	total := len(result)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return result[start:end], total
}

// AddComment 添加评论
func (m *Manager) AddComment(docID string, comment Comment) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, ok := m.documents[docID]
	if !ok {
		return fmt.Errorf("document not found: %s", docID)
	}
	comment.ID = fmt.Sprintf("cmt-%d", time.Now().UnixNano())
	comment.CreatedAt = time.Now()
	if doc.Comments == nil {
		doc.Comments = make([]Comment, 0)
	}
	doc.Comments = append(doc.Comments, comment)
	return nil
}

// LockDocument 锁定文档
func (m *Manager) LockDocument(id, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, ok := m.documents[id]
	if !ok {
		return fmt.Errorf("document not found: %s", id)
	}
	if doc.Locked {
		return fmt.Errorf("already locked by %s", doc.LockedBy)
	}
	doc.Locked = true
	doc.LockedBy = userID
	return nil
}

// UnlockDocument 解锁文档
func (m *Manager) UnlockDocument(id, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, ok := m.documents[id]
	if !ok {
		return fmt.Errorf("document not found: %s", id)
	}
	if doc.Locked && doc.LockedBy != userID {
		return fmt.Errorf("locked by another user")
	}
	doc.Locked = false
	doc.LockedBy = ""
	return nil
}

// CreateTask 创建任务
func (m *Manager) CreateTask(task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task.ID = fmt.Sprintf("task-%d", time.Now().UnixNano())
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	if task.Status == "" {
		task.Status = TaskStatusTodo
	}
	m.tasks[task.ID] = task
	return nil
}

// UpdateTask 更新任务
func (m *Manager) UpdateTask(id string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return fmt.Errorf("task not found: %s", id)
	}
	if s, ok := updates["status"].(string); ok {
		task.Status = TaskStatus(s)
		if s == "done" {
			now := time.Now()
			task.CompletedAt = &now
		}
	}
	if p, ok := updates["priority"].(float64); ok {
		task.Priority = int(p)
	}
	if a, ok := updates["assigneeId"].(string); ok {
		task.AssigneeID = a
	}
	task.UpdatedAt = time.Now()
	return nil
}

// ListTasks 列出任务
func (m *Manager) ListTasks(workspaceID, status string, page, pageSize int) ([]Task, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []Task
	for _, task := range m.tasks {
		if (workspaceID == "" || task.WorkspaceID == workspaceID) &&
			(status == "" || string(task.Status) == status) {
			result = append(result, *task)
		}
	}
	total := len(result)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}
	return result[start:end], total
}

// CreateWhiteboard 创建白板
func (m *Manager) CreateWhiteboard(wb *Whiteboard) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	wb.ID = fmt.Sprintf("wb-%d", time.Now().UnixNano())
	wb.CreatedAt = time.Now()
	wb.UpdatedAt = time.Now()
	if wb.Width == 0 {
		wb.Width = 1920
	}
	if wb.Height == 0 {
		wb.Height = 1080
	}
	m.whiteboards[wb.ID] = wb
	return nil
}

// GetWhiteboard 获取白板
func (m *Manager) GetWhiteboard(id string) (*Whiteboard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	wb, ok := m.whiteboards[id]
	if !ok {
		return nil, fmt.Errorf("whiteboard not found: %s", id)
	}
	return wb, nil
}

// AddWhiteboardElement 添加白板元素
func (m *Manager) AddWhiteboardElement(wbID string, elem WbElement) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	wb, ok := m.whiteboards[wbID]
	if !ok {
		return fmt.Errorf("whiteboard not found: %s", wbID)
	}
	elem.ID = fmt.Sprintf("elem-%d", time.Now().UnixNano())
	wb.Elements = append(wb.Elements, elem)
	wb.UpdatedAt = time.Now()
	return nil
}

// GetStats 获取统计
func (m *Manager) GetStats() WorkspaceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tasksByStatus := make(map[string]int)
	for _, task := range m.tasks {
		tasksByStatus[string(task.Status)]++
	}
	return WorkspaceStats{
		TotalWorkspaces:  len(m.workspaces),
		TotalDocuments:   len(m.documents),
		TotalTasks:       len(m.tasks),
		TotalWhiteboards: len(m.whiteboards),
		TasksByStatus:    tasksByStatus,
	}
}
