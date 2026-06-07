// Package teamworkspace 提供团队工作区功能
// 团队/项目空间管理、任务看板、团队日历、文件共享、成员管理、活动流
package teamworkspace

import (
	"fmt"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// MemberRole 成员角色
type MemberRole string

const (
	RoleAdmin  MemberRole = "admin"  // 管理员
	RoleMember MemberRole = "member" // 成员
	RoleGuest  MemberRole = "guest"  // 访客
)

// TaskPriority 任务优先级
type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"
	PriorityMedium TaskPriority = "medium"
	PriorityHigh   TaskPriority = "high"
	PriorityUrgent TaskPriority = "urgent"
)

// Workspace 团队工作区
type Workspace struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Owner       string    `json:"owner"`
	MemberCount int       `json:"memberCount"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Board 看板
type Board struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Columns []string `json:"columns"` // 列名列表
}

// Task 任务
type Task struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Assignee    string       `json:"assignee,omitempty"`
	Priority    TaskPriority `json:"priority"`
	DueDate     time.Time    `json:"dueDate,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	ColumnID    string       `json:"columnId"`
	BoardID     string       `json:"boardId"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// TaskComment 任务评论
type TaskComment struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"taskId"`
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
}

// CalendarEvent 日历事件
type CalendarEvent struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	StartTime    time.Time `json:"startTime"`
	EndTime      time.Time `json:"endTime"`
	Participants []string  `json:"participants,omitempty"`
}

// SharedFile 共享文件
type SharedFile struct {
	ID        string    `json:"id"`
	FileName  string    `json:"fileName"`
	Size      int64     `json:"size"`
	Uploader  string    `json:"uploader"`
	Scope     string    `json:"scope"` // 共享范围
	CreatedAt time.Time `json:"createdAt"`
}

// Activity 活动记录
type Activity struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	UserID      string    `json:"userId"`
	Description string    `json:"description"`
	Timestamp   time.Time `json:"timestamp"`
}

// Member 工作区成员
type Member struct {
	UserID   string     `json:"userId"`
	Role     MemberRole `json:"role"`
	JoinedAt time.Time  `json:"joinedAt"`
}

// ========== Manager ==========

// Manager 团队工作区管理器
type Manager struct {
	mu           sync.RWMutex
	workspaces   map[string]*Workspace
	members      map[string][]Member        // wsID -> members
	boards       map[string][]*Board        // wsID -> boards
	tasks        map[string]*Task           // taskID -> task
	taskComments map[string][]TaskComment   // taskID -> comments
	events       map[string][]CalendarEvent // wsID -> events
	files        map[string][]SharedFile    // wsID -> files
	activities   map[string][]Activity      // wsID -> activities
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		workspaces:   make(map[string]*Workspace),
		members:      make(map[string][]Member),
		boards:       make(map[string][]*Board),
		tasks:        make(map[string]*Task),
		taskComments: make(map[string][]TaskComment),
		events:       make(map[string][]CalendarEvent),
		files:        make(map[string][]SharedFile),
		activities:   make(map[string][]Activity),
	}
}

// addActivity 记录活动
func (m *Manager) addActivity(wsID, actType, userID, desc string) {
	m.activities[wsID] = append(m.activities[wsID], Activity{
		ID:          fmt.Sprintf("act-%d", len(m.activities[wsID])+1),
		Type:        actType,
		UserID:      userID,
		Description: desc,
		Timestamp:   time.Now(),
	})
}

// ========== 工作区管理 ==========

// CreateWorkspace 创建工作区
func (m *Manager) CreateWorkspace(ws *Workspace) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if ws.ID == "" {
		return fmt.Errorf("workspace ID is required")
	}
	if _, exists := m.workspaces[ws.ID]; exists {
		return fmt.Errorf("workspace %s already exists", ws.ID)
	}

	ws.CreatedAt = time.Now()
	ws.MemberCount = 1
	m.workspaces[ws.ID] = ws

	// 创建者自动成为管理员
	m.members[ws.ID] = []Member{{
		UserID: ws.Owner, Role: RoleAdmin, JoinedAt: time.Now(),
	}}

	// 默认看板
	m.boards[ws.ID] = []*Board{{
		ID: fmt.Sprintf("%s-board-default", ws.ID), Name: "默认看板",
		Columns: []string{"待办", "进行中", "已完成"},
	}}

	m.addActivity(ws.ID, "workspace_created", ws.Owner, fmt.Sprintf("创建工作区 '%s'", ws.Name))
	return nil
}

// GetWorkspace 获取工作区
func (m *Manager) GetWorkspace(id string) *Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.workspaces[id]
}

// ListWorkspaces 列出所有工作区
func (m *Manager) ListWorkspaces() []Workspace {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wsList := make([]Workspace, 0, len(m.workspaces))
	for _, ws := range m.workspaces {
		wsList = append(wsList, *ws)
	}
	return wsList
}

// DeleteWorkspace 删除工作区
func (m *Manager) DeleteWorkspace(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workspaces[id]; !ok {
		return fmt.Errorf("workspace %s not found", id)
	}

	delete(m.workspaces, id)
	delete(m.members, id)
	delete(m.boards, id)
	delete(m.events, id)
	delete(m.files, id)
	delete(m.activities, id)
	return nil
}

// ========== 成员管理 ==========

// AddMember 添加成员
func (m *Manager) AddMember(wsID, userID string, role string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workspaces[wsID]; !ok {
		return fmt.Errorf("workspace %s not found", wsID)
	}

	memberRole := MemberRole(role)
	if memberRole != RoleAdmin && memberRole != RoleMember && memberRole != RoleGuest {
		return fmt.Errorf("invalid role: %s", role)
	}

	// 检查是否已存在
	for _, mem := range m.members[wsID] {
		if mem.UserID == userID {
			return fmt.Errorf("member %s already exists in workspace %s", userID, wsID)
		}
	}

	m.members[wsID] = append(m.members[wsID], Member{
		UserID: userID, Role: memberRole, JoinedAt: time.Now(),
	})
	m.workspaces[wsID].MemberCount = len(m.members[wsID])

	m.addActivity(wsID, "member_added", userID, fmt.Sprintf("成员 %s 加入 (角色: %s)", userID, role))
	return nil
}

// RemoveMember 移除成员
func (m *Manager) RemoveMember(wsID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	members := m.members[wsID]
	for i, mem := range members {
		if mem.UserID == userID {
			m.members[wsID] = append(members[:i], members[i+1:]...)
			if ws, ok := m.workspaces[wsID]; ok {
				ws.MemberCount = len(m.members[wsID])
			}
			m.addActivity(wsID, "member_removed", userID, fmt.Sprintf("成员 %s 离开", userID))
			return nil
		}
	}
	return fmt.Errorf("member %s not found in workspace %s", userID, wsID)
}

// ========== 看板管理 ==========

// CreateBoard 创建看板
func (m *Manager) CreateBoard(wsID string, board *Board) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workspaces[wsID]; !ok {
		return fmt.Errorf("workspace %s not found", wsID)
	}

	if board.ID == "" {
		board.ID = fmt.Sprintf("%s-board-%d", wsID, len(m.boards[wsID])+1)
	}

	m.boards[wsID] = append(m.boards[wsID], board)
	return nil
}

// GetBoard 获取看板
func (m *Manager) GetBoard(wsID, boardID string) *Board {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, board := range m.boards[wsID] {
		if board.ID == boardID {
			return board
		}
	}
	return nil
}

// ========== 任务管理 ==========

// CreateTask 创建任务
func (m *Manager) CreateTask(wsID, boardID string, task *Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workspaces[wsID]; !ok {
		return fmt.Errorf("workspace %s not found", wsID)
	}

	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}

	task.BoardID = boardID
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	m.tasks[task.ID] = task

	m.addActivity(wsID, "task_created", task.Assignee, fmt.Sprintf("创建任务 '%s'", task.Title))
	return nil
}

// MoveTask 移动任务到指定列
func (m *Manager) MoveTask(wsID, taskID, targetCol string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	oldCol := task.ColumnID
	task.ColumnID = targetCol
	task.UpdatedAt = time.Now()

	m.addActivity(wsID, "task_moved", task.Assignee,
		fmt.Sprintf("任务 '%s' 从 '%s' 移动到 '%s'", task.Title, oldCol, targetCol))
	return nil
}

// AssignTask 分配任务
func (m *Manager) AssignTask(taskID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Assignee = userID
	task.UpdatedAt = time.Now()
	return nil
}

// GetTasks 获取任务列表（支持过滤）
func (m *Manager) GetTasks(wsID string, filters map[string]string) []Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Task
	for _, task := range m.tasks {
		if task.BoardID == "" {
			continue
		}

		// 简单过滤逻辑
		match := true
		for key, val := range filters {
			switch key {
			case "column":
				if task.ColumnID != val {
					match = false
				}
			case "assignee":
				if task.Assignee != val {
					match = false
				}
			case "priority":
				if string(task.Priority) != val {
					match = false
				}
			case "tag":
				found := false
				for _, tag := range task.Tags {
					if tag == val {
						found = true
						break
					}
				}
				if !found {
					match = false
				}
			}
		}

		if match {
			result = append(result, *task)
		}
	}
	return result
}

// ========== 日历 ==========

// AddEvent 添加日历事件
func (m *Manager) AddEvent(wsID string, event *CalendarEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workspaces[wsID]; !ok {
		return fmt.Errorf("workspace %s not found", wsID)
	}

	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%d", len(m.events[wsID])+1)
	}

	m.events[wsID] = append(m.events[wsID], *event)
	m.addActivity(wsID, "event_created", "", fmt.Sprintf("创建事件 '%s'", event.Title))
	return nil
}

// GetEvents 获取指定时间范围内的事件
func (m *Manager) GetEvents(wsID string, start, end time.Time) []CalendarEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []CalendarEvent
	for _, evt := range m.events[wsID] {
		if evt.EndTime.After(start) && evt.StartTime.Before(end) {
			result = append(result, evt)
		}
	}
	return result
}

// ========== 文件共享 ==========

// ShareFile 共享文件
func (m *Manager) ShareFile(wsID string, file *SharedFile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workspaces[wsID]; !ok {
		return fmt.Errorf("workspace %s not found", wsID)
	}

	if file.ID == "" {
		file.ID = fmt.Sprintf("file-%d", len(m.files[wsID])+1)
	}
	file.CreatedAt = time.Now()

	m.files[wsID] = append(m.files[wsID], *file)
	m.addActivity(wsID, "file_shared", file.Uploader, fmt.Sprintf("共享文件 '%s'", file.FileName))
	return nil
}

// ========== 活动流 ==========

// GetActivities 获取活动记录
func (m *Manager) GetActivities(wsID string, limit int) []Activity {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activities := m.activities[wsID]
	if limit > 0 && limit < len(activities) {
		// 返回最近的记录
		return activities[len(activities)-limit:]
	}
	return activities
}
