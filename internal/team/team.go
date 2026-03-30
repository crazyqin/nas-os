// Package team 团队管理核心功能
package team

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager 团队管理器
type Manager struct {
	mu         sync.RWMutex
	teams      map[string]*Team                  // teamID -> Team
	members    map[string]map[string]*TeamMember // teamID -> userID -> TeamMember
	folders    map[string]map[string]*TeamFolder // teamID -> folderID -> TeamFolder
	userTeams  map[string]map[string]bool        // userID -> teamID set
	configPath string
	dataDir    string
	audit      *AuditLogger
	notifier   *Notifier
}

// NewManager 创建团队管理器
func NewManager(configPath, dataDir string) (*Manager, error) {
	m := &Manager{
		teams:      make(map[string]*Team),
		members:    make(map[string]map[string]*TeamMember),
		folders:    make(map[string]map[string]*TeamFolder),
		userTeams:  make(map[string]map[string]bool),
		configPath: configPath,
		dataDir:    dataDir,
	}

	// 初始化审计日志
	m.audit = NewAuditLogger(filepath.Join(dataDir, "audit"))

	// 初始化通知器
	m.notifier = NewNotifier()

	// 加载配置
	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			log.Printf("加载团队配置失败: %v", err)
		}
	}

	return m, nil
}

// loadConfig 加载配置
func (m *Manager) loadConfig() error {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置失败: %w", err)
	}

	var config struct {
		Teams   map[string]*Team                  `json:"teams"`
		Members map[string]map[string]*TeamMember `json:"members"`
		Folders map[string]map[string]*TeamFolder `json:"folders"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	m.teams = config.Teams
	m.members = config.Members
	m.folders = config.Folders

	// 重建用户-团队索引
	for teamID, members := range m.members {
		for userID := range members {
			if m.userTeams[userID] == nil {
				m.userTeams[userID] = make(map[string]bool)
			}
			m.userTeams[userID][teamID] = true
		}
	}

	return nil
}

// saveConfig 保存配置
func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	config := struct {
		Teams   map[string]*Team                  `json:"teams"`
		Members map[string]map[string]*TeamMember `json:"members"`
		Folders map[string]map[string]*TeamFolder `json:"folders"`
	}{
		Teams:   m.teams,
		Members: m.members,
		Folders: m.folders,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0600)
}

// generateID 生成唯一ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ========== 团队管理 ==========

// CreateTeam 创建团队
func (m *Manager) CreateTeam(input TeamInput, ownerID, username string) (*Team, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查团队名是否已存在
	for _, t := range m.teams {
		if t.Name == input.Name {
			return nil, ErrTeamExists
		}
	}

	team := &Team{
		ID:          generateID(),
		Name:        input.Name,
		Description: input.Description,
		OwnerID:     ownerID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Settings:    input.Settings,
	}

	// 设置默认值
	if team.Settings.MaxMembers == 0 {
		team.Settings.MaxMembers = 100
	}

	m.teams[team.ID] = team

	// 初始化成员列表
	m.members[team.ID] = make(map[string]*TeamMember)

	// 添加所有者为成员
	m.members[team.ID][ownerID] = &TeamMember{
		TeamID:   team.ID,
		UserID:   ownerID,
		Username: username,
		Role:     RoleOwner,
		JoinedAt: time.Now(),
	}

	// 更新用户-团队索引
	if m.userTeams[ownerID] == nil {
		m.userTeams[ownerID] = make(map[string]bool)
	}
	m.userTeams[ownerID][team.ID] = true

	// 初始化文件夹列表
	m.folders[team.ID] = make(map[string]*TeamFolder)

	// 创建默认团队文件夹
	defaultFolder := &TeamFolder{
		ID:        generateID(),
		TeamID:    team.ID,
		Name:      "共享文件",
		Path:      filepath.Join(m.dataDir, "teams", team.ID, "shared"),
		CreatedBy: ownerID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Permissions: FolderPermissions{
			DefaultRole: RoleEditor,
		},
	}
	m.folders[team.ID][defaultFolder.ID] = defaultFolder

	// 创建物理目录
	os.MkdirAll(defaultFolder.Path, 0750)

	// 记录审计日志
	m.audit.Log(&TeamAuditLog{
		TeamID:   team.ID,
		UserID:   ownerID,
		Username: username,
		Action:   AuditTeamCreate,
		Details:  map[string]interface{}{"name": team.Name},
	})

	m.saveConfig()

	return team, nil
}

// GetTeam 获取团队
func (m *Manager) GetTeam(teamID string) (*Team, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	team, ok := m.teams[teamID]
	if !ok {
		return nil, ErrTeamNotFound
	}
	return team, nil
}

// ListTeams 列出所有团队
func (m *Manager) ListTeams() []*Team {
	m.mu.RLock()
	defer m.mu.RUnlock()

	teams := make([]*Team, 0, len(m.teams))
	for _, t := range m.teams {
		teams = append(teams, t)
	}
	return teams
}

// ListUserTeams 列出用户所属团队
func (m *Manager) ListUserTeams(userID string) []*Team {
	m.mu.RLock()
	defer m.mu.RUnlock()

	teams := make([]*Team, 0)
	for teamID := range m.userTeams[userID] {
		if team, ok := m.teams[teamID]; ok {
			teams = append(teams, team)
		}
	}
	return teams
}

// UpdateTeam 更新团队
func (m *Manager) UpdateTeam(teamID string, input TeamInput, userID string) (*Team, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return nil, ErrTeamNotFound
	}

	// 检查权限
	if !m.hasPermission(teamID, userID, RoleAdmin) {
		return nil, ErrNoPermission
	}

	if input.Name != "" {
		team.Name = input.Name
	}
	if input.Description != "" {
		team.Description = input.Description
	}
	team.Settings = input.Settings
	team.UpdatedAt = time.Now()

	m.audit.Log(&TeamAuditLog{
		TeamID:  teamID,
		UserID:  userID,
		Action:  AuditTeamUpdate,
		Details: map[string]interface{}{"name": team.Name},
	})

	m.saveConfig()
	return team, nil
}

// DeleteTeam 删除团队
func (m *Manager) DeleteTeam(teamID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return ErrTeamNotFound
	}

	// 只有所有者可以删除团队
	if team.OwnerID != userID {
		return ErrNoPermission
	}

	// 删除团队文件夹
	for _, folder := range m.folders[teamID] {
		os.RemoveAll(folder.Path)
	}

	// 清理索引
	for memberID := range m.members[teamID] {
		delete(m.userTeams[memberID], teamID)
	}

	delete(m.teams, teamID)
	delete(m.members, teamID)
	delete(m.folders, teamID)

	m.audit.Log(&TeamAuditLog{
		TeamID:  teamID,
		UserID:  userID,
		Action:  AuditTeamDelete,
		Details: map[string]interface{}{"name": team.Name},
	})

	m.saveConfig()
	return nil
}

// ========== 成员管理 ==========

// AddMember 添加成员
func (m *Manager) AddMember(teamID string, input MemberInput, inviterID, inviterName string) (*TeamMember, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return nil, ErrTeamNotFound
	}

	// 检查权限
	if !m.hasPermission(teamID, inviterID, RoleAdmin) {
		return nil, ErrNoPermission
	}

	// 检查成员数限制
	if len(m.members[teamID]) >= team.Settings.MaxMembers {
		return nil, &TeamError{Code: 400, Message: "团队成员数已达上限"}
	}

	// 检查是否已是成员
	if _, ok := m.members[teamID][input.UserID]; ok {
		return nil, ErrMemberExists
	}

	role := input.Role
	if role == "" {
		role = RoleViewer
	}

	member := &TeamMember{
		TeamID:    teamID,
		UserID:    input.UserID,
		Role:      role,
		JoinedAt:  time.Now(),
		InvitedBy: inviterID,
	}

	m.members[teamID][input.UserID] = member

	// 更新索引
	if m.userTeams[input.UserID] == nil {
		m.userTeams[input.UserID] = make(map[string]bool)
	}
	m.userTeams[input.UserID][teamID] = true

	// 发送通知
	m.notifier.Notify(&Notification{
		Type:     NotifyTeamMemberAdd,
		UserID:   input.UserID,
		FromUser: inviterName,
		Title:    "加入团队",
		Content:  fmt.Sprintf("您已被邀请加入团队: %s", team.Name),
		Data:     map[string]interface{}{"team_id": teamID, "role": role},
	})

	m.audit.Log(&TeamAuditLog{
		TeamID:   teamID,
		UserID:   inviterID,
		Username: inviterName,
		Action:   AuditTeamMemberAdd,
		Details:  map[string]interface{}{"member_id": input.UserID, "role": role},
	})

	m.saveConfig()
	return member, nil
}

// RemoveMember 移除成员
func (m *Manager) RemoveMember(teamID, memberID, operatorID, operatorName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return ErrTeamNotFound
	}

	// 检查权限
	if !m.hasPermission(teamID, operatorID, RoleAdmin) {
		return nil
	}

	// 不能移除所有者
	if memberID == team.OwnerID {
		return &TeamError{Code: 400, Message: "不能移除团队所有者"}
	}

	if _, ok := m.members[teamID][memberID]; !ok {
		return ErrMemberNotFound
	}

	delete(m.members[teamID], memberID)
	delete(m.userTeams[memberID], teamID)

	m.audit.Log(&TeamAuditLog{
		TeamID:   teamID,
		UserID:   operatorID,
		Username: operatorName,
		Action:   AuditTeamMemberRemove,
		Details:  map[string]interface{}{"member_id": memberID},
	})

	m.saveConfig()
	return nil
}

// UpdateMemberRole 更新成员角色
func (m *Manager) UpdateMemberRole(teamID, memberID string, role MemberRole, operatorID, operatorName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	team, ok := m.teams[teamID]
	if !ok {
		return ErrTeamNotFound
	}

	// 检查权限
	if !m.hasPermission(teamID, operatorID, RoleAdmin) {
		return ErrNoPermission
	}

	// 不能修改所有者角色
	if memberID == team.OwnerID {
		return &TeamError{Code: 400, Message: "不能修改所有者角色"}
	}

	member, ok := m.members[teamID][memberID]
	if !ok {
		return ErrMemberNotFound
	}

	member.Role = role

	m.audit.Log(&TeamAuditLog{
		TeamID:   teamID,
		UserID:   operatorID,
		Username: operatorName,
		Action:   AuditTeamMemberRole,
		Details:  map[string]interface{}{"member_id": memberID, "role": role},
	})

	m.saveConfig()
	return nil
}

// GetMembers 获取团队成员列表
func (m *Manager) GetMembers(teamID string) ([]*TeamMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.teams[teamID]; !ok {
		return nil, ErrTeamNotFound
	}

	members := make([]*TeamMember, 0, len(m.members[teamID]))
	for _, member := range m.members[teamID] {
		members = append(members, member)
	}
	return members, nil
}

// GetMemberRole 获取成员角色
func (m *Manager) GetMemberRole(teamID, userID string) (MemberRole, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	member, ok := m.members[teamID][userID]
	if !ok {
		return "", ErrMemberNotFound
	}
	return member.Role, nil
}

// hasPermission 检查权限
func (m *Manager) hasPermission(teamID, userID string, requiredRole MemberRole) bool {
	member, ok := m.members[teamID][userID]
	if !ok {
		return false
	}

	switch requiredRole {
	case RoleOwner:
		return member.Role == RoleOwner
	case RoleAdmin:
		return member.Role == RoleOwner || member.Role == RoleAdmin
	case RoleEditor:
		return member.Role == RoleOwner || member.Role == RoleAdmin || member.Role == RoleEditor
	case RoleViewer:
		return true
	default:
		return false
	}
}

// ========== 文件夹管理 ==========

// CreateFolder 创建团队文件夹
func (m *Manager) CreateFolder(teamID string, input FolderInput, userID, username string) (*TeamFolder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.teams[teamID]; !ok {
		return nil, ErrTeamNotFound
	}

	// 检查权限
	if !m.hasPermission(teamID, userID, RoleEditor) {
		return nil, ErrNoPermission
	}

	// 检查文件夹名是否已存在
	for _, f := range m.folders[teamID] {
		if f.Name == input.Name && f.ParentID == input.ParentID {
			return nil, ErrFolderExists
		}
	}

	path := input.Path
	if path == "" {
		path = filepath.Join(m.dataDir, "teams", teamID, input.Name)
	}

	folder := &TeamFolder{
		ID:          generateID(),
		TeamID:      teamID,
		Name:        input.Name,
		Path:        path,
		ParentID:    input.ParentID,
		CreatedBy:   userID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Permissions: input.Permissions,
		Quota:       input.Quota,
	}

	if folder.Permissions.DefaultRole == "" {
		folder.Permissions.DefaultRole = RoleEditor
	}

	m.folders[teamID][folder.ID] = folder

	// 创建物理目录
	os.MkdirAll(folder.Path, 0750)

	m.audit.Log(&TeamAuditLog{
		TeamID:       teamID,
		UserID:       userID,
		Username:     username,
		Action:       AuditFolderCreate,
		ResourceID:   folder.ID,
		ResourcePath: folder.Path,
		Details:      map[string]interface{}{"name": folder.Name},
	})

	m.saveConfig()
	return folder, nil
}

// GetFolder 获取团队文件夹
func (m *Manager) GetFolder(teamID, folderID string) (*TeamFolder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	folders, ok := m.folders[teamID]
	if !ok {
		return nil, ErrTeamNotFound
	}

	folder, ok := folders[folderID]
	if !ok {
		return nil, ErrFolderNotFound
	}
	return folder, nil
}

// ListFolders 列出团队文件夹
func (m *Manager) ListFolders(teamID string) ([]*TeamFolder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	folders, ok := m.folders[teamID]
	if !ok {
		return nil, ErrTeamNotFound
	}

	result := make([]*TeamFolder, 0, len(folders))
	for _, f := range folders {
		result = append(result, f)
	}
	return result, nil
}

// UpdateFolder 更新文件夹
func (m *Manager) UpdateFolder(teamID, folderID string, input FolderInput, userID, username string) (*TeamFolder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.hasPermission(teamID, userID, RoleEditor) {
		return nil, ErrNoPermission
	}

	folder, ok := m.folders[teamID][folderID]
	if !ok {
		return nil, ErrFolderNotFound
	}

	if input.Name != "" {
		folder.Name = input.Name
	}
	if input.Permissions.DefaultRole != "" {
		folder.Permissions = input.Permissions
	}
	if input.Quota > 0 {
		folder.Quota = input.Quota
	}
	folder.UpdatedAt = time.Now()

	m.audit.Log(&TeamAuditLog{
		TeamID:       teamID,
		UserID:       userID,
		Username:     username,
		Action:       AuditFolderUpdate,
		ResourceID:   folderID,
		ResourcePath: folder.Path,
	})

	m.saveConfig()
	return folder, nil
}

// DeleteFolder 删除文件夹
func (m *Manager) DeleteFolder(teamID, folderID, userID, username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.hasPermission(teamID, userID, RoleAdmin) {
		return ErrNoPermission
	}

	folder, ok := m.folders[teamID][folderID]
	if !ok {
		return ErrFolderNotFound
	}

	// 删除物理目录
	os.RemoveAll(folder.Path)

	delete(m.folders[teamID], folderID)

	m.audit.Log(&TeamAuditLog{
		TeamID:       teamID,
		UserID:       userID,
		Username:     username,
		Action:       AuditFolderDelete,
		ResourceID:   folderID,
		ResourcePath: folder.Path,
	})

	m.saveConfig()
	return nil
}

// GetTeamStats 获取团队统计
func (m *Manager) GetTeamStats(teamID string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.teams[teamID]; !ok {
		return nil, ErrTeamNotFound
	}

	stats := map[string]interface{}{
		"member_count": len(m.members[teamID]),
		"folder_count": len(m.folders[teamID]),
	}

	// 计算总使用量
	var totalSize int64
	for _, folder := range m.folders[teamID] {
		totalSize += folder.UsedSize
	}
	stats["total_size"] = totalSize

	return stats, nil
}
