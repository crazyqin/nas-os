// Package rbac 提供共享访问控制列表
// 支持 SMB/NFS 共享权限管理
package rbac

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 共享访问控制类型 ==========

// ShareType 共享类型.
type ShareType string

const (
	// ShareTypeSMB SMB 共享.
	ShareTypeSMB ShareType = "smb"
	// ShareTypeNFS NFS 共享.
	ShareTypeNFS ShareType = "nfs"
	// ShareTypeFTP FTP 共享.
	ShareTypeFTP ShareType = "ftp"
	// ShareTypeWebDAV WebDAV 共享.
	ShareTypeWebDAV ShareType = "webdav"
)

// AccessLevel 访问级别.
type AccessLevel string

const (
	// AccessNone 无权限.
	AccessNone AccessLevel = "none"
	// AccessRead 只读.
	AccessRead AccessLevel = "read"
	// AccessWrite 读写.
	AccessWrite AccessLevel = "write"
	// AccessFull 完全控制.
	AccessFull AccessLevel = "full"
	// AccessCustom 自定义权限.
	AccessCustom AccessLevel = "custom"
)

// ShareACL 共享访问控制条目.
type ShareACL struct {
	ID           string      `json:"id"`
	ShareName    string      `json:"share_name"`
	ShareType    ShareType   `json:"share_type"`
	Path         string      `json:"path"`
	Description  string      `json:"description"`
	Entries      []*ACLEntry `json:"entries"`
	DefaultLevel AccessLevel `json:"default_level"` // 默认访问级别
	Enabled      bool        `json:"enabled"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// ACLEntry 访问控制条目.
type ACLEntry struct {
	ID            string        `json:"id"`
	PrincipalType PrincipalType `json:"principal_type"` // user, group, everyone
	PrincipalID   string        `json:"principal_id"`   // 用户ID或组ID
	PrincipalName string        `json:"principal_name"` // 用户名或组名
	AccessLevel   AccessLevel   `json:"access_level"`
	Permissions   []string      `json:"permissions,omitempty"` // 自定义权限列表
	Inherited     bool          `json:"inherited"`             // 是否继承
	Disabled      bool          `json:"disabled"`              // 是否禁用
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
	CreatedBy     string        `json:"created_by"` // 创建者用户ID
}

// PrincipalType 主体类型.
type PrincipalType string

const (
	// PrincipalUser 用户主体.
	PrincipalUser PrincipalType = "user"
	// PrincipalGroup 用户组主体.
	PrincipalGroup PrincipalType = "group"
	// PrincipalEveryone 所有人主体.
	PrincipalEveryone PrincipalType = "everyone"
)

// SharePermission 共享权限检查结果.
type SharePermission struct {
	ShareName   string      `json:"share_name"`
	UserID      string      `json:"user_id"`
	AccessLevel AccessLevel `json:"access_level"`
	Permissions []string    `json:"permissions"`
	Source      string      `json:"source"` // 来源: direct, group, default
	Inherited   bool        `json:"inherited"`
}

// ShareACLConfig 共享 ACL 配置.
type ShareACLConfig struct {
	ConfigPath string `json:"config_path"`
}

// ShareACLManager 共享 ACL 管理器.
type ShareACLManager struct {
	mu     sync.RWMutex
	config ShareACLConfig
	acls   map[string]*ShareACL // shareName -> ACL
	rb     *Manager             // RBAC 管理器
}

// 错误定义.
var (
	ErrShareNotFound      = errors.New("共享不存在")
	ErrACLEntryExists     = errors.New("ACL 条目已存在")
	ErrACLEntryNotFound   = errors.New("ACL 条目不存在")
	ErrInvalidAccessLevel = errors.New("无效的访问级别")
)

// NewShareACLManager 创建共享 ACL 管理器.
func NewShareACLManager(config ShareACLConfig, rbacManager *Manager) (*ShareACLManager, error) {
	m := &ShareACLManager{
		config: config,
		acls:   make(map[string]*ShareACL),
		rb:     rbacManager,
	}

	if config.ConfigPath != "" {
		if err := m.load(); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	return m, nil
}

// ========== 共享 ACL 管理 ==========

// CreateShareACL 创建共享 ACL.
func (m *ShareACLManager) CreateShareACL(shareName string, shareType ShareType, path, description string, defaultLevel AccessLevel) (*ShareACL, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.acls[shareName]; exists {
		return nil, fmt.Errorf("共享 ACL 已存在: %s", shareName)
	}

	now := time.Now()
	acl := &ShareACL{
		ID:           generateACLID(),
		ShareName:    shareName,
		ShareType:    shareType,
		Path:         path,
		Description:  description,
		Entries:      make([]*ACLEntry, 0),
		DefaultLevel: defaultLevel,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// 添加默认条目：所有人（使用默认访问级别）
	if defaultLevel != AccessNone {
		acl.Entries = append(acl.Entries, &ACLEntry{
			ID:            generateACLEntryID(),
			PrincipalType: PrincipalEveryone,
			PrincipalID:   "everyone",
			PrincipalName: "所有人",
			AccessLevel:   defaultLevel,
			Inherited:     false,
			Disabled:      false,
			CreatedAt:     now,
		})
	}

	m.acls[shareName] = acl
	if err := m.save(); err != nil {
		return nil, fmt.Errorf("保存 ACL 配置失败: %w", err)
	}

	return acl, nil
}

// GetShareACL 获取共享 ACL.
func (m *ShareACLManager) GetShareACL(shareName string) (*ShareACL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	acl, exists := m.acls[shareName]
	if !exists {
		return nil, ErrShareNotFound
	}

	return acl, nil
}

// UpdateShareACL 更新共享 ACL 基本信息.
func (m *ShareACLManager) UpdateShareACL(shareName, description string, defaultLevel AccessLevel, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	acl, exists := m.acls[shareName]
	if !exists {
		return ErrShareNotFound
	}

	acl.Description = description
	acl.DefaultLevel = defaultLevel
	acl.Enabled = enabled
	acl.UpdatedAt = time.Now()

	if err := m.save(); err != nil {
		return fmt.Errorf("保存 ACL 配置失败: %w", err)
	}
	return nil
}

// DeleteShareACL 删除共享 ACL.
func (m *ShareACLManager) DeleteShareACL(shareName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.acls[shareName]; !exists {
		return ErrShareNotFound
	}

	delete(m.acls, shareName)
	if err := m.save(); err != nil {
		return fmt.Errorf("保存 ACL 配置失败: %w", err)
	}
	return nil
}

// ListShareACLs 列出所有共享 ACL.
func (m *ShareACLManager) ListShareACLs() []*ShareACL {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ShareACL, 0, len(m.acls))
	for _, acl := range m.acls {
		result = append(result, acl)
	}
	return result
}

// ========== ACL 条目管理 ==========

// AddACLEntry 添加 ACL 条目.
func (m *ShareACLManager) AddACLEntry(shareName string, principalType PrincipalType, principalID, principalName string, accessLevel AccessLevel, createdBy string) (*ACLEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	acl, exists := m.acls[shareName]
	if !exists {
		return nil, ErrShareNotFound
	}

	// 检查是否已存在
	for _, entry := range acl.Entries {
		if entry.PrincipalType == principalType && entry.PrincipalID == principalID {
			if !entry.Disabled {
				return nil, ErrACLEntryExists
			}
			// 如果已禁用，重新启用
			entry.Disabled = false
			entry.AccessLevel = accessLevel
			entry.UpdatedAt = time.Now()
			if err := m.save(); err != nil {
				return nil, fmt.Errorf("保存 ACL 配置失败: %w", err)
			}
			return entry, nil
		}
	}

	now := time.Now()
	entry := &ACLEntry{
		ID:            generateACLEntryID(),
		PrincipalType: principalType,
		PrincipalID:   principalID,
		PrincipalName: principalName,
		AccessLevel:   accessLevel,
		Inherited:     false,
		Disabled:      false,
		CreatedAt:     now,
		CreatedBy:     createdBy,
	}

	acl.Entries = append(acl.Entries, entry)
	acl.UpdatedAt = now

	if err := m.save(); err != nil {
		return nil, fmt.Errorf("保存 ACL 配置失败: %w", err)
	}
	return entry, nil
}

// UpdateACLEntry 更新 ACL 条目.
func (m *ShareACLManager) UpdateACLEntry(shareName, entryID string, accessLevel AccessLevel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	acl, exists := m.acls[shareName]
	if !exists {
		return ErrShareNotFound
	}

	for _, entry := range acl.Entries {
		if entry.ID == entryID {
			entry.AccessLevel = accessLevel
			acl.UpdatedAt = time.Now()
			if err := m.save(); err != nil {
				return fmt.Errorf("保存 ACL 配置失败: %w", err)
			}
			return nil
		}
	}

	return ErrACLEntryNotFound
}

// RemoveACLEntry 移除 ACL 条目.
func (m *ShareACLManager) RemoveACLEntry(shareName, entryID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	acl, exists := m.acls[shareName]
	if !exists {
		return ErrShareNotFound
	}

	newEntries := make([]*ACLEntry, 0, len(acl.Entries))
	for _, entry := range acl.Entries {
		if entry.ID != entryID {
			newEntries = append(newEntries, entry)
		}
	}

	if len(newEntries) == len(acl.Entries) {
		return ErrACLEntryNotFound
	}

	acl.Entries = newEntries
	acl.UpdatedAt = time.Now()
	if err := m.save(); err != nil {
		return fmt.Errorf("保存 ACL 配置失败: %w", err)
	}
	return nil
}

// DisableACLEntry 禁用 ACL 条目.
func (m *ShareACLManager) DisableACLEntry(shareName, entryID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	acl, exists := m.acls[shareName]
	if !exists {
		return ErrShareNotFound
	}

	for _, entry := range acl.Entries {
		if entry.ID == entryID {
			entry.Disabled = true
			acl.UpdatedAt = time.Now()
			if err := m.save(); err != nil {
				return fmt.Errorf("保存 ACL 配置失败: %w", err)
			}
			return nil
		}
	}

	return ErrACLEntryNotFound
}

// ========== 权限检查 ==========

// CheckShareAccess 检查用户对共享的访问权限.
func (m *ShareACLManager) CheckShareAccess(userID, username, shareName string) *SharePermission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := &SharePermission{
		ShareName:   shareName,
		UserID:      userID,
		AccessLevel: AccessNone,
		Permissions: []string{},
		Source:      "default",
		Inherited:   false,
	}

	acl, exists := m.acls[shareName]
	if !exists || !acl.Enabled {
		result.AccessLevel = AccessNone
		return result
	}

	// 获取用户所属组
	var groups []string
	if m.rb != nil {
		if up, err := m.rb.GetUserPermissions(userID); err == nil {
			for _, gm := range up.GroupMemberships {
				groups = append(groups, gm.GroupID, gm.GroupName)
			}
		}
	}

	// 检查 ACL 条目（优先级：用户 > 组 > 所有人）
	// 先检查用户条目
	for _, entry := range acl.Entries {
		if entry.Disabled {
			continue
		}
		if entry.PrincipalType == PrincipalUser && entry.PrincipalID == userID {
			result.AccessLevel = entry.AccessLevel
			result.Permissions = entry.Permissions
			result.Source = "direct"
			result.Inherited = entry.Inherited
			return result
		}
		if entry.PrincipalType == PrincipalUser && entry.PrincipalName == username {
			result.AccessLevel = entry.AccessLevel
			result.Permissions = entry.Permissions
			result.Source = "direct"
			result.Inherited = entry.Inherited
			return result
		}
	}

	// 检查组条目
	for _, entry := range acl.Entries {
		if entry.Disabled {
			continue
		}
		if entry.PrincipalType == PrincipalGroup {
			for _, g := range groups {
				if entry.PrincipalID == g || entry.PrincipalName == g {
					// 取最高权限
					if compareAccessLevel(entry.AccessLevel, result.AccessLevel) > 0 {
						result.AccessLevel = entry.AccessLevel
						result.Permissions = entry.Permissions
						result.Source = "group"
						result.Inherited = entry.Inherited
					}
				}
			}
		}
	}

	// 如果用户条目和组条目都没找到，使用默认（所有人）条目
	if result.AccessLevel == AccessNone {
		for _, entry := range acl.Entries {
			if entry.Disabled {
				continue
			}
			if entry.PrincipalType == PrincipalEveryone {
				result.AccessLevel = entry.AccessLevel
				result.Permissions = entry.Permissions
				result.Source = "default"
				result.Inherited = entry.Inherited
				break
			}
		}
	}

	// 如果还是没有，使用 ACL 默认级别
	if result.AccessLevel == AccessNone {
		result.AccessLevel = acl.DefaultLevel
	}

	return result
}

// GetUserShares 获取用户可访问的共享列表.
func (m *ShareACLManager) GetUserShares(userID, username string) []*SharePermission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SharePermission, 0)

	for shareName, acl := range m.acls {
		if !acl.Enabled {
			continue
		}

		perm := m.CheckShareAccess(userID, username, shareName)
		if perm.AccessLevel != AccessNone {
			result = append(result, perm)
		}
	}

	return result
}

// CanRead 检查读权限.
func (m *ShareACLManager) CanRead(userID, username, shareName string) bool {
	perm := m.CheckShareAccess(userID, username, shareName)
	return perm.AccessLevel == AccessRead ||
		perm.AccessLevel == AccessWrite ||
		perm.AccessLevel == AccessFull
}

// CanWrite 检查写权限.
func (m *ShareACLManager) CanWrite(userID, username, shareName string) bool {
	perm := m.CheckShareAccess(userID, username, shareName)
	return perm.AccessLevel == AccessWrite || perm.AccessLevel == AccessFull
}

// CanFullControl 检查完全控制权限.
func (m *ShareACLManager) CanFullControl(userID, username, shareName string) bool {
	perm := m.CheckShareAccess(userID, username, shareName)
	return perm.AccessLevel == AccessFull
}

// ========== 辅助函数 ==========

func generateACLID() string {
	return fmt.Sprintf("acl-%d", time.Now().UnixNano())
}

func generateACLEntryID() string {
	return fmt.Sprintf("entry-%d", time.Now().UnixNano())
}

func compareAccessLevel(a, b AccessLevel) int {
	levels := map[AccessLevel]int{
		AccessNone:   0,
		AccessRead:   1,
		AccessWrite:  2,
		AccessFull:   3,
		AccessCustom: 4,
	}

	return levels[a] - levels[b]
}

// ========== 持久化 ==========

func (m *ShareACLManager) load() error {
	if m.config.ConfigPath == "" {
		return nil
	}

	data, err := os.ReadFile(m.config.ConfigPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.acls)
}

func (m *ShareACLManager) save() error {
	if m.config.ConfigPath == "" {
		return nil
	}

	data, err := json.MarshalIndent(m.acls, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(m.config.ConfigPath), 0750); err != nil {
		return err
	}

	return os.WriteFile(m.config.ConfigPath, data, 0600)
}

// ========== SMB 兼容层 ==========

// ToSMBACL 转换为 SMB ACL 格式.
func (acl *ShareACL) ToSMBACL() map[string]interface{} {
	result := map[string]interface{}{
		"share_name":  acl.ShareName,
		"path":        acl.Path,
		"comment":     acl.Description,
		"browseable":  true,
		"guest_ok":    acl.DefaultLevel == AccessRead,
		"valid_users": []string{},
		"write_list":  []string{},
		"read_list":   []string{},
	}

	validUsers := make([]string, 0)
	writeList := make([]string, 0)
	readList := make([]string, 0)

	for _, entry := range acl.Entries {
		if entry.Disabled {
			continue
		}

		name := entry.PrincipalName
		if entry.PrincipalType == PrincipalGroup {
			name = "@" + name
		} else if entry.PrincipalType == PrincipalEveryone {
			continue // 所有人通过 guest_ok 处理
		}

		validUsers = append(validUsers, name)

		switch entry.AccessLevel {
		case AccessRead:
			readList = append(readList, name)
		case AccessWrite, AccessFull:
			writeList = append(writeList, name)
		}
	}

	result["valid_users"] = validUsers
	result["write_list"] = writeList
	result["read_list"] = readList

	return result
}

// ========== NFS 兼容层 ==========

// ToNFSExports 转换为 NFS exports 格式.
func (acl *ShareACL) ToNFSExports() []map[string]interface{} {
	result := make([]map[string]interface{}, 0)

	for _, entry := range acl.Entries {
		if entry.Disabled || entry.PrincipalType == PrincipalEveryone {
			continue
		}

		export := map[string]interface{}{
			"path":    acl.Path,
			"client":  entry.PrincipalName, // NFS 使用 IP/主机名
			"options": []string{},
		}

		options := []string{"rw", "sync", "no_subtree_check"}
		if entry.AccessLevel == AccessRead {
			options = []string{"ro", "sync", "no_subtree_check"}
		}

		export["options"] = options
		result = append(result, export)
	}

	return result
}
