package teamfile

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// NewTeamFileManager 创建团队文件管理器
func NewTeamFileManager(cfg ManagerConfig) *TeamFileManager {
	return &TeamFileManager{
		folders: make(map[string]*TeamFolder),
		members: make(map[string][]*FolderMember),
		links:   make(map[string]*ShareLink),
		config:  cfg,
	}
}

// CreateFolder 创建团队文件夹
func (m *TeamFileManager) CreateFolder(name, description, ownerTeam, path string) (*TeamFolder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.folders) >= m.config.MaxFolders {
		return nil, ErrMaxFoldersReached
	}

	folderID := fmt.Sprintf("tf-%d", time.Now().UnixNano())
	now := time.Now()

	folder := &TeamFolder{
		ID:          folderID,
		Name:        name,
		Description: description,
		Path:        path,
		OwnerTeam:   ownerTeam,
		CreatedAt:   now,
		UpdatedAt:   now,
		IsActive:    true,
	}

	m.folders[folderID] = folder

	// 添加创建者为所有者
	m.members[folderID] = []*FolderMember{{
		UserID:     ownerTeam,
		Role:       RoleOwner,
		Permission: PermOwner,
		JoinedAt:   now,
		AddedBy:    "system",
	}}

	m.addAuditLog(folderID, ownerTeam, "create_folder", name, "创建团队文件夹")

	return folder, nil
}

// DeleteFolder 删除团队文件夹
func (m *TeamFileManager) DeleteFolder(folderID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.folders[folderID]; !exists {
		return ErrFolderNotFound
	}

	delete(m.folders, folderID)
	delete(m.members, folderID)
	return nil
}

// GetFolder 获取团队文件夹
func (m *TeamFileManager) GetFolder(folderID string) (*TeamFolder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	folder, exists := m.folders[folderID]
	if !exists {
		return nil, ErrFolderNotFound
	}
	return folder, nil
}

// ListFolders 列出所有团队文件夹
func (m *TeamFileManager) ListFolders() []*TeamFolder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TeamFolder, 0, len(m.folders))
	for _, f := range m.folders {
		result = append(result, f)
	}
	return result
}

// AddMember 添加成员
func (m *TeamFileManager) AddMember(folderID, userID string, role MemberRole, perm FolderPermission) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.folders[folderID]; !exists {
		return ErrFolderNotFound
	}

	members := m.members[folderID]
	if len(members) >= m.config.MaxMembersPerFolder {
		return ErrMaxMembersReached
	}

	// 检查是否已存在
	for _, member := range members {
		if member.UserID == userID {
			return ErrMemberExists
		}
	}

	m.members[folderID] = append(members, &FolderMember{
		UserID:     userID,
		Role:       role,
		Permission: perm,
		JoinedAt:   time.Now(),
	})

	m.addAuditLog(folderID, userID, "add_member", userID, fmt.Sprintf("添加成员，角色: %s", role))
	return nil
}

// RemoveMember 移除成员
func (m *TeamFileManager) RemoveMember(folderID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	members, exists := m.members[folderID]
	if !exists {
		return ErrFolderNotFound
	}

	for i, member := range members {
		if member.UserID == userID {
			m.members[folderID] = append(members[:i], members[i+1:]...)
			m.addAuditLog(folderID, userID, "remove_member", userID, "移除成员")
			return nil
		}
	}
	return ErrMemberNotFound
}

// GetMembers 获取文件夹成员
func (m *TeamFileManager) GetMembers(folderID string) ([]*FolderMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	members, exists := m.members[folderID]
	if !exists {
		return nil, ErrFolderNotFound
	}
	return members, nil
}

// CreateShareLink 创建分享链接
func (m *TeamFileManager) CreateShareLink(folderID, createdBy string, perm FolderPermission, expiryDays int) (*ShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.folders[folderID]; !exists {
		return nil, ErrFolderNotFound
	}

	token, _ := generateToken(16)
	linkID := fmt.Sprintf("link-%d", time.Now().UnixNano())
	now := time.Now()
	expires := now.AddDate(0, 0, expiryDays)

	link := &ShareLink{
		ID:         linkID,
		FolderID:   folderID,
		Token:      token,
		Permission: perm,
		ExpiresAt:  &expires,
		CreatedBy:  createdBy,
		CreatedAt:  now,
		IsActive:   true,
	}

	m.links[linkID] = link
	m.addAuditLog(folderID, createdBy, "create_link", linkID, "创建分享链接")
	return link, nil
}

// ValidateShareLink 验证分享链接
func (m *TeamFileManager) ValidateShareLink(token string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, link := range m.links {
		if link.Token == token && link.IsActive {
			if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
				return nil, ErrLinkExpired
			}
			link.DownloadCount++
			return link, nil
		}
	}
	return nil, ErrLinkNotFound
}

// GetAuditLog 获取审计日志
func (m *TeamFileManager) GetAuditLog(folderID string) []*AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AuditLog, 0)
	for _, log := range m.auditLog {
		if folderID == "" || log.FolderID == folderID {
			result = append(result, log)
		}
	}
	return result
}

func (m *TeamFileManager) addAuditLog(folderID, userID, action, target, details string) {
	if !m.config.AuditEnabled {
		return
	}
	m.auditLog = append(m.auditLog, &AuditLog{
		ID:        fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		FolderID:  folderID,
		UserID:    userID,
		Action:    action,
		Target:    target,
		Details:   details,
		Timestamp: time.Now(),
	})
}

func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
