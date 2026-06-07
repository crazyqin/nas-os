// Package collabworkspace 协作工作空间模块
// 对标群晖 Synology Office，提供文档协作、任务看板、权限控制等功能
package collabworkspace

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// PermissionLevel 权限级别
type PermissionLevel int

const (
	PermissionNone   PermissionLevel = iota // 无权限
	PermissionRead                          // 只读
	PermissionWrite                         // 读写
	PermissionManage                        // 管理
)

// String 权限级别字符串
func (p PermissionLevel) String() string {
	switch p {
	case PermissionRead:
		return "read"
	case PermissionWrite:
		return "write"
	case PermissionManage:
		return "manage"
	default:
		return "none"
	}
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusReview     TaskStatus = "review"
	TaskStatusDone       TaskStatus = "done"
)

// ActivityType 活动类型
type ActivityType string

const (
	ActivityWorkspaceCreate  ActivityType = "workspace_create"
	ActivityWorkspaceDelete  ActivityType = "workspace_delete"
	ActivityDocumentCreate   ActivityType = "document_create"
	ActivityDocumentEdit     ActivityType = "document_edit"
	ActivityDocumentDelete   ActivityType = "document_delete"
	ActivityCommentAdd       ActivityType = "comment_add"
	ActivityCommentDelete    ActivityType = "comment_delete"
	ActivityTaskCreate       ActivityType = "task_create"
	ActivityTaskUpdate       ActivityType = "task_update"
	ActivityTaskDelete       ActivityType = "task_delete"
	ActivityMemberAdd        ActivityType = "member_add"
	ActivityMemberRemove     ActivityType = "member_remove"
	ActivityPermissionChange ActivityType = "permission_change"
)

// Permission 权限配置
type Permission struct {
	UserID string          `json:"user_id"`
	Level  PermissionLevel `json:"level"`
}

// Member 工作空间成员
type Member struct {
	UserID   string    `json:"user_id"`
	Username string    `json:"username"`
	Role     string    `json:"role"`
	JoinedAt time.Time `json:"joined_at"`
}

// Workspace 工作空间
type Workspace struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	OwnerID     string       `json:"owner_id"`
	Members     []Member     `json:"members"`
	Permissions []Permission `json:"permissions"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	IsArchived  bool         `json:"is_archived"`
}

// DocumentVersion 文档版本
type DocumentVersion struct {
	Version   int       `json:"version"`
	Content   string    `json:"content"`
	EditorID  string    `json:"editor_id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// Document 协作文档
type Document struct {
	ID          string            `json:"id"`
	WorkspaceID string            `json:"workspace_id"`
	Title       string            `json:"title"`
	Content     string            `json:"content"`
	CreatorID   string            `json:"creator_id"`
	Editors     []string          `json:"editors"`
	Versions    []DocumentVersion `json:"versions"`
	Version     int               `json:"version"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	IsLocked    bool              `json:"is_locked"`
	LockedBy    string            `json:"locked_by,omitempty"`
}

// Comment 文档评论
type Comment struct {
	ID        string    `json:"id"`
	DocID     string    `json:"doc_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"content"`
	ParentID  string    `json:"parent_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	IsEdited  bool      `json:"is_edited"`
}

// Tag 任务标签
type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Task 任务
type Task struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspace_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
	AssigneeID  string     `json:"assignee_id,omitempty"`
	CreatorID   string     `json:"creator_id"`
	Tags        []Tag      `json:"tags"`
	Priority    int        `json:"priority"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// Activity 活动记录
type Activity struct {
	ID          string       `json:"id"`
	WorkspaceID string       `json:"workspace_id"`
	UserID      string       `json:"user_id"`
	Username    string       `json:"username"`
	Type        ActivityType `json:"type"`
	TargetID    string       `json:"target_id"`
	TargetName  string       `json:"target_name"`
	Detail      string       `json:"detail"`
	CreatedAt   time.Time    `json:"created_at"`
}

// CollaborativeWorkspace 协作工作空间管理器
type CollaborativeWorkspace struct {
	mu         sync.RWMutex
	workspaces map[string]*Workspace
	documents  map[string]*Document
	comments   map[string]*Comment
	tasks      map[string]*Task
	activities []Activity
	idCounter  int64
}

// NewCollaborativeWorkspace 创建协作工作空间管理器
func NewCollaborativeWorkspace() *CollaborativeWorkspace {
	return &CollaborativeWorkspace{
		workspaces: make(map[string]*Workspace),
		documents:  make(map[string]*Document),
		comments:   make(map[string]*Comment),
		tasks:      make(map[string]*Task),
		activities: make([]Activity, 0),
	}
}

// 生成唯一ID
func (cw *CollaborativeWorkspace) generateID(prefix string) string {
	cw.idCounter++
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), cw.idCounter)
}

// addActivity 添加活动记录（调用者需持有写锁）
func (cw *CollaborativeWorkspace) addActivity(workspaceID, userID, username string, actType ActivityType, targetID, targetName, detail string) {
	activity := Activity{
		ID:          cw.generateID("act"),
		WorkspaceID: workspaceID,
		UserID:      userID,
		Username:    username,
		Type:        actType,
		TargetID:    targetID,
		TargetName:  targetName,
		Detail:      detail,
		CreatedAt:   time.Now(),
	}
	cw.activities = append(cw.activities, activity)
}

// ==================== 工作空间管理 ====================

// CreateWorkspace 创建工作空间
func (cw *CollaborativeWorkspace) CreateWorkspace(name, description, ownerID string) (*Workspace, error) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if name == "" {
		return nil, errors.New("workspace name is required")
	}
	if ownerID == "" {
		return nil, errors.New("owner ID is required")
	}

	now := time.Now()
	ws := &Workspace{
		ID:          cw.generateID("ws"),
		Name:        name,
		Description: description,
		OwnerID:     ownerID,
		Members: []Member{
			{
				UserID:   ownerID,
				Username: ownerID,
				Role:     "owner",
				JoinedAt: now,
			},
		},
		Permissions: []Permission{
			{UserID: ownerID, Level: PermissionManage},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	cw.workspaces[ws.ID] = ws
	cw.addActivity(ws.ID, ownerID, ownerID, ActivityWorkspaceCreate, ws.ID, name, "创建工作空间")

	return ws, nil
}

// DeleteWorkspace 删除工作空间
func (cw *CollaborativeWorkspace) DeleteWorkspace(workspaceID, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	ws, ok := cw.workspaces[workspaceID]
	if !ok {
		return errors.New("workspace not found")
	}

	if !cw.hasPermission(workspaceID, userID, PermissionManage) {
		return errors.New("permission denied: manage permission required")
	}

	// 删除工作空间下的所有文档及评论
	for docID, doc := range cw.documents {
		if doc.WorkspaceID == workspaceID {
			delete(cw.documents, docID)
		}
	}
	for commentID, comment := range cw.comments {
		if _, exists := cw.documents[comment.DocID]; !exists {
			delete(cw.comments, commentID)
		}
	}

	// 删除工作空间下的所有任务
	for taskID, task := range cw.tasks {
		if task.WorkspaceID == workspaceID {
			delete(cw.tasks, taskID)
		}
	}

	cw.addActivity(workspaceID, userID, userID, ActivityWorkspaceDelete, workspaceID, ws.Name, "删除工作空间")
	delete(cw.workspaces, workspaceID)

	return nil
}

// ListWorkspaces 列出用户可见的工作空间
func (cw *CollaborativeWorkspace) ListWorkspaces(userID string) []*Workspace {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	result := make([]*Workspace, 0)
	for _, ws := range cw.workspaces {
		if ws.IsArchived {
			continue
		}
		for _, member := range ws.Members {
			if member.UserID == userID {
				result = append(result, ws)
				break
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// GetWorkspace 获取工作空间详情
func (cw *CollaborativeWorkspace) GetWorkspace(workspaceID string) (*Workspace, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	ws, ok := cw.workspaces[workspaceID]
	if !ok {
		return nil, errors.New("workspace not found")
	}
	return ws, nil
}

// ArchiveWorkspace 归档工作空间
func (cw *CollaborativeWorkspace) ArchiveWorkspace(workspaceID, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	ws, ok := cw.workspaces[workspaceID]
	if !ok {
		return errors.New("workspace not found")
	}

	if !cw.hasPermission(workspaceID, userID, PermissionManage) {
		return errors.New("permission denied: manage permission required")
	}

	ws.IsArchived = true
	ws.UpdatedAt = time.Now()

	return nil
}

// ==================== 成员管理 ====================

// AddMember 添加成员
func (cw *CollaborativeWorkspace) AddMember(workspaceID, userID, username, role, operatorID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	ws, ok := cw.workspaces[workspaceID]
	if !ok {
		return errors.New("workspace not found")
	}

	if !cw.hasPermission(workspaceID, operatorID, PermissionManage) {
		return errors.New("permission denied: manage permission required")
	}

	for _, member := range ws.Members {
		if member.UserID == userID {
			return errors.New("member already exists")
		}
	}

	ws.Members = append(ws.Members, Member{
		UserID:   userID,
		Username: username,
		Role:     role,
		JoinedAt: time.Now(),
	})

	ws.Permissions = append(ws.Permissions, Permission{
		UserID: userID,
		Level:  PermissionWrite,
	})

	ws.UpdatedAt = time.Now()
	cw.addActivity(workspaceID, operatorID, operatorID, ActivityMemberAdd, userID, username, fmt.Sprintf("添加成员 %s", username))

	return nil
}

// RemoveMember 移除成员
func (cw *CollaborativeWorkspace) RemoveMember(workspaceID, userID, operatorID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	ws, ok := cw.workspaces[workspaceID]
	if !ok {
		return errors.New("workspace not found")
	}

	if !cw.hasPermission(workspaceID, operatorID, PermissionManage) {
		return errors.New("permission denied: manage permission required")
	}

	if userID == ws.OwnerID {
		return errors.New("cannot remove workspace owner")
	}

	found := false
	for i, member := range ws.Members {
		if member.UserID == userID {
			ws.Members = append(ws.Members[:i], ws.Members[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return errors.New("member not found")
	}

	for i, perm := range ws.Permissions {
		if perm.UserID == userID {
			ws.Permissions = append(ws.Permissions[:i], ws.Permissions[i+1:]...)
			break
		}
	}

	ws.UpdatedAt = time.Now()
	cw.addActivity(workspaceID, operatorID, operatorID, ActivityMemberRemove, userID, userID, fmt.Sprintf("移除成员 %s", userID))

	return nil
}

// ==================== 权限控制 ====================

// SetPermission 设置用户权限
func (cw *CollaborativeWorkspace) SetPermission(workspaceID, targetUserID string, level PermissionLevel, operatorID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	ws, ok := cw.workspaces[workspaceID]
	if !ok {
		return errors.New("workspace not found")
	}

	if !cw.hasPermission(workspaceID, operatorID, PermissionManage) {
		return errors.New("permission denied: manage permission required")
	}

	found := false
	for i, perm := range ws.Permissions {
		if perm.UserID == targetUserID {
			ws.Permissions[i].Level = level
			found = true
			break
		}
	}

	if !found {
		ws.Permissions = append(ws.Permissions, Permission{
			UserID: targetUserID,
			Level:  level,
		})
	}

	ws.UpdatedAt = time.Now()
	cw.addActivity(workspaceID, operatorID, operatorID, ActivityPermissionChange, targetUserID, targetUserID,
		fmt.Sprintf("设置权限为 %s", level.String()))

	return nil
}

// CheckPermission 检查用户权限
func (cw *CollaborativeWorkspace) CheckPermission(workspaceID, userID string, requiredLevel PermissionLevel) bool {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	return cw.hasPermission(workspaceID, userID, requiredLevel)
}

// GetPermission 获取用户权限级别
func (cw *CollaborativeWorkspace) GetPermission(workspaceID, userID string) PermissionLevel {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	ws, ok := cw.workspaces[workspaceID]
	if !ok {
		return PermissionNone
	}

	for _, perm := range ws.Permissions {
		if perm.UserID == userID {
			return perm.Level
		}
	}

	return PermissionNone
}

// hasPermission 内部方法：检查权限（调用者需持有锁）
func (cw *CollaborativeWorkspace) hasPermission(workspaceID, userID string, requiredLevel PermissionLevel) bool {
	ws, ok := cw.workspaces[workspaceID]
	if !ok {
		return false
	}

	if ws.OwnerID == userID {
		return true
	}

	for _, perm := range ws.Permissions {
		if perm.UserID == userID {
			return perm.Level >= requiredLevel
		}
	}

	return false
}

// ==================== 文档协作 ====================

// CreateDocument 创建文档
func (cw *CollaborativeWorkspace) CreateDocument(workspaceID, title, content, creatorID string) (*Document, error) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if !cw.hasPermission(workspaceID, creatorID, PermissionWrite) {
		return nil, errors.New("permission denied: write permission required")
	}

	now := time.Now()
	doc := &Document{
		ID:          cw.generateID("doc"),
		WorkspaceID: workspaceID,
		Title:       title,
		Content:     content,
		CreatorID:   creatorID,
		Editors:     make([]string, 0),
		Versions: []DocumentVersion{
			{
				Version:   1,
				Content:   content,
				EditorID:  creatorID,
				Message:   "初始版本",
				CreatedAt: now,
			},
		},
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	cw.documents[doc.ID] = doc
	cw.addActivity(workspaceID, creatorID, creatorID, ActivityDocumentCreate, doc.ID, title, "创建文档")

	return doc, nil
}

// GetDocument 获取文档
func (cw *CollaborativeWorkspace) GetDocument(docID, userID string) (*Document, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	doc, ok := cw.documents[docID]
	if !ok {
		return nil, errors.New("document not found")
	}

	if !cw.hasPermission(doc.WorkspaceID, userID, PermissionRead) {
		return nil, errors.New("permission denied: read permission required")
	}

	return doc, nil
}

// EditDocument 编辑文档
func (cw *CollaborativeWorkspace) EditDocument(docID, content, editorID, message string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	doc, ok := cw.documents[docID]
	if !ok {
		return errors.New("document not found")
	}

	if !cw.hasPermission(doc.WorkspaceID, editorID, PermissionWrite) {
		return errors.New("permission denied: write permission required")
	}

	if doc.IsLocked && doc.LockedBy != editorID {
		return fmt.Errorf("document is locked by %s", doc.LockedBy)
	}

	doc.Content = content
	doc.Version++
	doc.UpdatedAt = time.Now()

	doc.Versions = append(doc.Versions, DocumentVersion{
		Version:   doc.Version,
		Content:   content,
		EditorID:  editorID,
		Message:   message,
		CreatedAt: time.Now(),
	})

	cw.addActivity(doc.WorkspaceID, editorID, editorID, ActivityDocumentEdit, docID, doc.Title, message)

	return nil
}

// DeleteDocument 删除文档
func (cw *CollaborativeWorkspace) DeleteDocument(docID, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	doc, ok := cw.documents[docID]
	if !ok {
		return errors.New("document not found")
	}

	if !cw.hasPermission(doc.WorkspaceID, userID, PermissionWrite) {
		return errors.New("permission denied: write permission required")
	}

	for commentID, comment := range cw.comments {
		if comment.DocID == docID {
			delete(cw.comments, commentID)
		}
	}

	cw.addActivity(doc.WorkspaceID, userID, userID, ActivityDocumentDelete, docID, doc.Title, "删除文档")
	delete(cw.documents, docID)

	return nil
}

// ListDocuments 列出工作空间文档
func (cw *CollaborativeWorkspace) ListDocuments(workspaceID, userID string) ([]*Document, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	if !cw.hasPermission(workspaceID, userID, PermissionRead) {
		return nil, errors.New("permission denied: read permission required")
	}

	result := make([]*Document, 0)
	for _, doc := range cw.documents {
		if doc.WorkspaceID == workspaceID {
			result = append(result, doc)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	return result, nil
}

// LockDocument 锁定文档（防止并发编辑冲突）
func (cw *CollaborativeWorkspace) LockDocument(docID, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	doc, ok := cw.documents[docID]
	if !ok {
		return errors.New("document not found")
	}

	if !cw.hasPermission(doc.WorkspaceID, userID, PermissionWrite) {
		return errors.New("permission denied: write permission required")
	}

	if doc.IsLocked && doc.LockedBy != userID {
		return fmt.Errorf("document is already locked by %s", doc.LockedBy)
	}

	doc.IsLocked = true
	doc.LockedBy = userID
	doc.UpdatedAt = time.Now()

	return nil
}

// UnlockDocument 解锁文档
func (cw *CollaborativeWorkspace) UnlockDocument(docID, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	doc, ok := cw.documents[docID]
	if !ok {
		return errors.New("document not found")
	}

	if doc.LockedBy != userID {
		return errors.New("only the lock owner can unlock the document")
	}

	doc.IsLocked = false
	doc.LockedBy = ""
	doc.UpdatedAt = time.Now()

	return nil
}

// JoinEditing 加入编辑（实时协作）
func (cw *CollaborativeWorkspace) JoinEditing(docID, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	doc, ok := cw.documents[docID]
	if !ok {
		return errors.New("document not found")
	}

	if !cw.hasPermission(doc.WorkspaceID, userID, PermissionWrite) {
		return errors.New("permission denied: write permission required")
	}

	for _, editor := range doc.Editors {
		if editor == userID {
			return nil
		}
	}

	doc.Editors = append(doc.Editors, userID)
	doc.UpdatedAt = time.Now()

	return nil
}

// LeaveEditing 离开编辑
func (cw *CollaborativeWorkspace) LeaveEditing(docID, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	doc, ok := cw.documents[docID]
	if !ok {
		return errors.New("document not found")
	}

	for i, editor := range doc.Editors {
		if editor == userID {
			doc.Editors = append(doc.Editors[:i], doc.Editors[i+1:]...)
			doc.UpdatedAt = time.Now()
			return nil
		}
	}

	return nil
}

// GetDocumentVersions 获取文档版本历史
func (cw *CollaborativeWorkspace) GetDocumentVersions(docID, userID string) ([]DocumentVersion, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	doc, ok := cw.documents[docID]
	if !ok {
		return nil, errors.New("document not found")
	}

	if !cw.hasPermission(doc.WorkspaceID, userID, PermissionRead) {
		return nil, errors.New("permission denied: read permission required")
	}

	return doc.Versions, nil
}

// RestoreDocumentVersion 恢复文档版本
func (cw *CollaborativeWorkspace) RestoreDocumentVersion(docID string, version int, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	doc, ok := cw.documents[docID]
	if !ok {
		return errors.New("document not found")
	}

	if !cw.hasPermission(doc.WorkspaceID, userID, PermissionWrite) {
		return errors.New("permission denied: write permission required")
	}

	if doc.IsLocked && doc.LockedBy != userID {
		return fmt.Errorf("document is locked by %s", doc.LockedBy)
	}

	var targetVersion *DocumentVersion
	for _, v := range doc.Versions {
		if v.Version == version {
			vCopy := v
			targetVersion = &vCopy
			break
		}
	}

	if targetVersion == nil {
		return fmt.Errorf("version %d not found", version)
	}

	doc.Content = targetVersion.Content
	doc.Version++
	doc.UpdatedAt = time.Now()

	doc.Versions = append(doc.Versions, DocumentVersion{
		Version:   doc.Version,
		Content:   targetVersion.Content,
		EditorID:  userID,
		Message:   fmt.Sprintf("恢复到版本 %d", version),
		CreatedAt: time.Now(),
	})

	cw.addActivity(doc.WorkspaceID, userID, userID, ActivityDocumentEdit, docID, doc.Title,
		fmt.Sprintf("恢复到版本 %d", version))

	return nil
}

// ==================== 评论系统 ====================

// AddComment 添加评论
func (cw *CollaborativeWorkspace) AddComment(docID, userID, username, content, parentID string) (*Comment, error) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	doc, ok := cw.documents[docID]
	if !ok {
		return nil, errors.New("document not found")
	}

	if !cw.hasPermission(doc.WorkspaceID, userID, PermissionRead) {
		return nil, errors.New("permission denied: read permission required")
	}

	if content == "" {
		return nil, errors.New("comment content is required")
	}

	if parentID != "" {
		if _, exists := cw.comments[parentID]; !exists {
			return nil, errors.New("parent comment not found")
		}
	}

	now := time.Now()
	comment := &Comment{
		ID:        cw.generateID("cmt"),
		DocID:     docID,
		UserID:    userID,
		Username:  username,
		Content:   content,
		ParentID:  parentID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	cw.comments[comment.ID] = comment
	cw.addActivity(doc.WorkspaceID, userID, username, ActivityCommentAdd, docID, doc.Title, "添加评论")

	return comment, nil
}

// GetComments 获取文档评论
func (cw *CollaborativeWorkspace) GetComments(docID, userID string) ([]*Comment, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	doc, ok := cw.documents[docID]
	if !ok {
		return nil, errors.New("document not found")
	}

	if !cw.hasPermission(doc.WorkspaceID, userID, PermissionRead) {
		return nil, errors.New("permission denied: read permission required")
	}

	result := make([]*Comment, 0)
	for _, comment := range cw.comments {
		if comment.DocID == docID {
			result = append(result, comment)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result, nil
}

// EditComment 编辑评论
func (cw *CollaborativeWorkspace) EditComment(commentID, userID, content string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	comment, ok := cw.comments[commentID]
	if !ok {
		return errors.New("comment not found")
	}

	if comment.UserID != userID {
		return errors.New("only comment author can edit")
	}

	comment.Content = content
	comment.UpdatedAt = time.Now()
	comment.IsEdited = true

	return nil
}

// DeleteComment 删除评论
func (cw *CollaborativeWorkspace) DeleteComment(commentID, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	comment, ok := cw.comments[commentID]
	if !ok {
		return errors.New("comment not found")
	}

	doc, docExists := cw.documents[comment.DocID]
	if !docExists {
		return errors.New("associated document not found")
	}

	if comment.UserID != userID && !cw.hasPermission(doc.WorkspaceID, userID, PermissionManage) {
		return errors.New("permission denied")
	}

	cw.addActivity(doc.WorkspaceID, userID, userID, ActivityCommentDelete, commentID, "", "删除评论")
	delete(cw.comments, commentID)

	return nil
}

// ==================== 任务看板 ====================

// CreateTask 创建任务
func (cw *CollaborativeWorkspace) CreateTask(workspaceID, title, description, creatorID string, priority int, dueDate *time.Time) (*Task, error) {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	if !cw.hasPermission(workspaceID, creatorID, PermissionWrite) {
		return nil, errors.New("permission denied: write permission required")
	}

	if title == "" {
		return nil, errors.New("task title is required")
	}

	if priority < 1 || priority > 5 {
		priority = 3
	}

	now := time.Now()
	task := &Task{
		ID:          cw.generateID("task"),
		WorkspaceID: workspaceID,
		Title:       title,
		Description: description,
		Status:      TaskStatusTodo,
		CreatorID:   creatorID,
		Tags:        make([]Tag, 0),
		Priority:    priority,
		DueDate:     dueDate,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	cw.tasks[task.ID] = task
	cw.addActivity(workspaceID, creatorID, creatorID, ActivityTaskCreate, task.ID, title, "创建任务")

	return task, nil
}

// GetTask 获取任务详情
func (cw *CollaborativeWorkspace) GetTask(taskID, userID string) (*Task, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	task, ok := cw.tasks[taskID]
	if !ok {
		return nil, errors.New("task not found")
	}

	if !cw.hasPermission(task.WorkspaceID, userID, PermissionRead) {
		return nil, errors.New("permission denied: read permission required")
	}

	return task, nil
}

// ListTasks 列出工作空间任务
func (cw *CollaborativeWorkspace) ListTasks(workspaceID, userID string, status TaskStatus) ([]*Task, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	if !cw.hasPermission(workspaceID, userID, PermissionRead) {
		return nil, errors.New("permission denied: read permission required")
	}

	result := make([]*Task, 0)
	for _, task := range cw.tasks {
		if task.WorkspaceID == workspaceID {
			if status == "" || task.Status == status {
				result = append(result, task)
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	return result, nil
}

// UpdateTaskStatus 更新任务状态
func (cw *CollaborativeWorkspace) UpdateTaskStatus(taskID string, status TaskStatus, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	task, ok := cw.tasks[taskID]
	if !ok {
		return errors.New("task not found")
	}

	if !cw.hasPermission(task.WorkspaceID, userID, PermissionWrite) {
		return errors.New("permission denied: write permission required")
	}

	oldStatus := task.Status
	task.Status = status
	task.UpdatedAt = time.Now()

	if status == TaskStatusDone {
		now := time.Now()
		task.CompletedAt = &now
	}

	cw.addActivity(task.WorkspaceID, userID, userID, ActivityTaskUpdate, taskID, task.Title,
		fmt.Sprintf("状态从 %s 变为 %s", oldStatus, status))

	return nil
}

// AssignTask 分配任务
func (cw *CollaborativeWorkspace) AssignTask(taskID, assigneeID, operatorID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	task, ok := cw.tasks[taskID]
	if !ok {
		return errors.New("task not found")
	}

	if !cw.hasPermission(task.WorkspaceID, operatorID, PermissionWrite) {
		return errors.New("permission denied: write permission required")
	}

	task.AssigneeID = assigneeID
	task.UpdatedAt = time.Now()

	cw.addActivity(task.WorkspaceID, operatorID, operatorID, ActivityTaskUpdate, taskID, task.Title,
		fmt.Sprintf("分配给 %s", assigneeID))

	return nil
}

// UpdateTask 更新任务信息
func (cw *CollaborativeWorkspace) UpdateTask(taskID, userID, title, description string, priority int, dueDate *time.Time) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	task, ok := cw.tasks[taskID]
	if !ok {
		return errors.New("task not found")
	}

	if !cw.hasPermission(task.WorkspaceID, userID, PermissionWrite) {
		return errors.New("permission denied: write permission required")
	}

	if title != "" {
		task.Title = title
	}
	if description != "" {
		task.Description = description
	}
	if priority >= 1 && priority <= 5 {
		task.Priority = priority
	}
	if dueDate != nil {
		task.DueDate = dueDate
	}

	task.UpdatedAt = time.Now()
	cw.addActivity(task.WorkspaceID, userID, userID, ActivityTaskUpdate, taskID, task.Title, "更新任务")

	return nil
}

// DeleteTask 删除任务
func (cw *CollaborativeWorkspace) DeleteTask(taskID, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	task, ok := cw.tasks[taskID]
	if !ok {
		return errors.New("task not found")
	}

	if !cw.hasPermission(task.WorkspaceID, userID, PermissionWrite) {
		return errors.New("permission denied: write permission required")
	}

	cw.addActivity(task.WorkspaceID, userID, userID, ActivityTaskDelete, taskID, task.Title, "删除任务")
	delete(cw.tasks, taskID)

	return nil
}

// AddTaskTag 添加任务标签
func (cw *CollaborativeWorkspace) AddTaskTag(taskID string, tag Tag, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	task, ok := cw.tasks[taskID]
	if !ok {
		return errors.New("task not found")
	}

	if !cw.hasPermission(task.WorkspaceID, userID, PermissionWrite) {
		return errors.New("permission denied: write permission required")
	}

	for _, existingTag := range task.Tags {
		if existingTag.ID == tag.ID {
			return nil
		}
	}

	if tag.ID == "" {
		tag.ID = cw.generateID("tag")
	}

	task.Tags = append(task.Tags, tag)
	task.UpdatedAt = time.Now()

	return nil
}

// RemoveTaskTag 移除任务标签
func (cw *CollaborativeWorkspace) RemoveTaskTag(taskID, tagID, userID string) error {
	cw.mu.Lock()
	defer cw.mu.Unlock()

	task, ok := cw.tasks[taskID]
	if !ok {
		return errors.New("task not found")
	}

	if !cw.hasPermission(task.WorkspaceID, userID, PermissionWrite) {
		return errors.New("permission denied: write permission required")
	}

	for i, tag := range task.Tags {
		if tag.ID == tagID {
			task.Tags = append(task.Tags[:i], task.Tags[i+1:]...)
			task.UpdatedAt = time.Now()
			return nil
		}
	}

	return errors.New("tag not found")
}

// GetTasksByAssignee 获取分配给指定用户的任务
func (cw *CollaborativeWorkspace) GetTasksByAssignee(workspaceID, assigneeID, userID string) ([]*Task, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	if !cw.hasPermission(workspaceID, userID, PermissionRead) {
		return nil, errors.New("permission denied: read permission required")
	}

	result := make([]*Task, 0)
	for _, task := range cw.tasks {
		if task.WorkspaceID == workspaceID && task.AssigneeID == assigneeID {
			result = append(result, task)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority
	})

	return result, nil
}

// GetTasksByTag 获取带有指定标签的任务
func (cw *CollaborativeWorkspace) GetTasksByTag(workspaceID, tagID, userID string) ([]*Task, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	if !cw.hasPermission(workspaceID, userID, PermissionRead) {
		return nil, errors.New("permission denied: read permission required")
	}

	result := make([]*Task, 0)
	for _, task := range cw.tasks {
		if task.WorkspaceID != workspaceID {
			continue
		}
		for _, tag := range task.Tags {
			if tag.ID == tagID {
				result = append(result, task)
				break
			}
		}
	}

	return result, nil
}

// ==================== 活动记录 ====================

// GetActivities 获取活动记录
func (cw *CollaborativeWorkspace) GetActivities(workspaceID, userID string, limit int) ([]Activity, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	if !cw.hasPermission(workspaceID, userID, PermissionRead) {
		return nil, errors.New("permission denied: read permission required")
	}

	result := make([]Activity, 0)
	for _, activity := range cw.activities {
		if activity.WorkspaceID == workspaceID {
			result = append(result, activity)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}

	return result, nil
}

// GetActivitiesByType 按类型获取活动记录
func (cw *CollaborativeWorkspace) GetActivitiesByType(workspaceID, userID string, actType ActivityType) ([]Activity, error) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()

	if !cw.hasPermission(workspaceID, userID, PermissionRead) {
		return nil, errors.New("permission denied: read permission required")
	}

	result := make([]Activity, 0)
	for _, activity := range cw.activities {
		if activity.WorkspaceID == workspaceID && activity.Type == actType {
			result = append(result, activity)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result, nil
}
