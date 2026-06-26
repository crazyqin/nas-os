// Package teamfile 提供团队文件协作功能。
// 支持团队创建/成员管理、角色权限（管理员/编辑/只读）、文件共享与锁定。
// 参考飞 fnOS 团队文件（成员权限精细化管理、多人协作）设计。
package teamfile

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

// 常见错误
var (
	ErrTeamNotFound     = errors.New("team not found")
	ErrMemberNotFound   = errors.New("member not found")
	ErrFileNotFound     = errors.New("file not found")
	ErrPermissionDenied = errors.New("permission denied")
	ErrFileLocked       = errors.New("file is locked by another member")
	ErrAlreadyMember    = errors.New("user is already a team member")
	ErrInvalidInput     = errors.New("invalid input")
	ErrRoleNotAllowed   = errors.New("role not allowed for this operation")
)

// Role 成员角色
type Role string

const (
	RoleAdmin  Role = "admin"  // 管理员：完全控制
	RoleEditor Role = "editor" // 编辑：可读写文件
	RoleViewer Role = "viewer" // 只读：仅查看
)

// CanEdit 判断角色是否有编辑权限
func (r Role) CanEdit() bool {
	return r == RoleAdmin || r == RoleEditor
}

// CanAdmin 判断角色是否有管理员权限
func (r Role) CanAdmin() bool {
	return r == RoleAdmin
}

// Member 团队成员
type Member struct {
	UserID    string    `json:"user_id"`
	Role      Role      `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
	InvitedBy string    `json:"invited_by,omitempty"`
}

// FileLock 文件锁定信息
type FileLock struct {
	LockedBy  string     `json:"locked_by"`
	LockedAt  time.Time  `json:"locked_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// SharedFile 共享文件
type SharedFile struct {
	ID          string    `json:"id"`
	TeamID      string    `json:"team_id"`
	Path        string    `json:"path"`
	SharedBy    string    `json:"shared_by"`
	SharedAt    time.Time `json:"shared_at"`
	Lock        *FileLock `json:"lock,omitempty"`
	Description string    `json:"description,omitempty"`
}

// Team 团队
type Team struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	OwnerID     string       `json:"owner_id"`
	Members     []Member     `json:"members"`
	Files       []SharedFile `json:"files,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// CreateTeamRequest 创建团队请求
type CreateTeamRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description,omitempty"`
}

// AddMemberRequest 添加成员请求
type AddMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   Role   `json:"role" binding:"required"`
}

// ShareFileRequest 共享文件请求
type ShareFileRequest struct {
	Path        string `json:"path" binding:"required"`
	Description string `json:"description,omitempty"`
}

// LockFileRequest 锁定文件请求
type LockFileRequest struct {
	Duration *time.Duration `json:"duration,omitempty"` // 锁定持续时间
}

// Manager 团队文件管理器
type Manager struct {
	mu    sync.RWMutex
	teams map[string]*Team // teamID -> Team
	// TODO: 替换为持久化存储
}

// NewManager 创建团队文件管理器
func NewManager() *Manager {
	return &Manager{
		teams: make(map[string]*Team),
	}
}

// CreateTeam 创建团队
func (m *Manager) CreateTeam(ownerID string, req *CreateTeamRequest) (*Team, error) {
	if ownerID == "" || req.Name == "" {
		return nil, ErrInvalidInput
	}

	now := time.Now()
	// TODO: 使用 uuid 生成 ID
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	id := "team_" + hex.EncodeToString(raw)

	team := &Team{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     ownerID,
		Members: []Member{
			{UserID: ownerID, Role: RoleAdmin, JoinedAt: now},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	m.mu.Lock()
	m.teams[id] = team
	m.mu.Unlock()

	return team, nil
}

// AddMember 添加团队成员（需要管理员权限）
func (m *Manager) AddMember(operatorID, teamID string, req *AddMemberRequest) error {
	if req.UserID == "" {
		return ErrInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return ErrTeamNotFound
	}

	if !m.hasAdminRole(team, operatorID) {
		return ErrPermissionDenied
	}

	// 检查是否已是成员
	for _, m := range team.Members {
		if m.UserID == req.UserID {
			return ErrAlreadyMember
		}
	}

	// 验证角色
	if req.Role != RoleAdmin && req.Role != RoleEditor && req.Role != RoleViewer {
		return ErrInvalidInput
	}

	team.Members = append(team.Members, Member{
		UserID:    req.UserID,
		Role:      req.Role,
		JoinedAt:  time.Now(),
		InvitedBy: operatorID,
	})
	team.UpdatedAt = time.Now()

	return nil
}

// RemoveMember 移除团队成员（需要管理员权限）
func (m *Manager) RemoveMember(operatorID, teamID, memberUserID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return ErrTeamNotFound
	}

	if !m.hasAdminRole(team, operatorID) {
		return ErrPermissionDenied
	}

	if memberUserID == team.OwnerID {
		return ErrRoleNotAllowed // 不能移除团队所有者
	}

	for i, member := range team.Members {
		if member.UserID == memberUserID {
			team.Members = append(team.Members[:i], team.Members[i+1:]...)
			team.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrMemberNotFound
}

// UpdateMemberRole 更新成员角色（需要管理员权限）
func (m *Manager) UpdateMemberRole(operatorID, teamID, memberUserID string, newRole Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return ErrTeamNotFound
	}

	if !m.hasAdminRole(team, operatorID) {
		return ErrPermissionDenied
	}

	if newRole != RoleAdmin && newRole != RoleEditor && newRole != RoleViewer {
		return ErrInvalidInput
	}

	for i, member := range team.Members {
		if member.UserID == memberUserID {
			team.Members[i].Role = newRole
			team.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrMemberNotFound
}

// ShareFile 在团队中共享文件（需要编辑权限）
func (m *Manager) ShareFile(operatorID, teamID string, req *ShareFileRequest) (*SharedFile, error) {
	if req.Path == "" {
		return nil, ErrInvalidInput
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return nil, ErrTeamNotFound
	}

	if !m.hasEditRole(team, operatorID) {
		return nil, ErrPermissionDenied
	}

	// TODO: 使用 uuid 生成 ID
	now := time.Now()
	raw := make([]byte, 8)
	_, _ = rand.Read(raw)
	fileID := "file_" + hex.EncodeToString(raw)

	sf := SharedFile{
		ID:          fileID,
		TeamID:      teamID,
		Path:        req.Path,
		SharedBy:    operatorID,
		SharedAt:    now,
		Description: req.Description,
	}

	team.Files = append(team.Files, sf)
	team.UpdatedAt = now

	// TODO: 通知团队成员有新文件共享

	return &sf, nil
}

// LockFile 锁定文件（需要编辑权限）
func (m *Manager) LockFile(operatorID, teamID, fileID string, req *LockFileRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return ErrTeamNotFound
	}

	if !m.hasEditRole(team, operatorID) {
		return ErrPermissionDenied
	}

	for i, f := range team.Files {
		if f.ID == fileID {
			if f.Lock != nil {
				// 检查是否过期
				if f.Lock.ExpiresAt != nil && time.Now().After(*f.Lock.ExpiresAt) {
					// 锁已过期，允许重新锁定
				} else if f.Lock.LockedBy != operatorID {
					return ErrFileLocked
				}
			}
			now := time.Now()
			lock := &FileLock{
				LockedBy: operatorID,
				LockedAt: now,
			}
			if req != nil && req.Duration != nil {
				expires := now.Add(*req.Duration)
				lock.ExpiresAt = &expires
			}
			team.Files[i].Lock = lock
			team.UpdatedAt = now
			return nil
		}
	}

	return ErrFileNotFound
}

// UnlockFile 解锁文件
func (m *Manager) UnlockFile(operatorID, teamID, fileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return ErrTeamNotFound
	}

	if !m.hasEditRole(team, operatorID) {
		return ErrPermissionDenied
	}

	for i, f := range team.Files {
		if f.ID == fileID {
			if f.Lock != nil && f.Lock.LockedBy != operatorID && !m.hasAdminRole(team, operatorID) {
				return ErrFileLocked // 只有锁定者或管理员可以解锁
			}
			team.Files[i].Lock = nil
			team.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrFileNotFound
}

// GetTeam 获取团队详情
func (m *Manager) GetTeam(teamID string) (*Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	team, ok := m.teams[teamID]
	if !ok {
		return nil, ErrTeamNotFound
	}
	return team, nil
}

// ListUserTeams 列出用户参与的所有团队
func (m *Manager) ListUserTeams(userID string) []*Team {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Team
	for _, team := range m.teams {
		for _, member := range team.Members {
			if member.UserID == userID {
				result = append(result, team)
				break
			}
		}
	}
	return result
}

// hasAdminRole 检查用户是否是团队管理员
func (m *Manager) hasAdminRole(team *Team, userID string) bool {
	for _, member := range team.Members {
		if member.UserID == userID && member.Role.CanAdmin() {
			return true
		}
	}
	return false
}

// hasEditRole 检查用户是否有编辑权限
func (m *Manager) hasEditRole(team *Team, userID string) bool {
	for _, member := range team.Members {
		if member.UserID == userID && member.Role.CanEdit() {
			return true
		}
	}
	return false
}
