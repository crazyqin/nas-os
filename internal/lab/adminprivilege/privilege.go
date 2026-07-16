package adminprivilege

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// PrivilegeLevel 权限级别.
type PrivilegeLevel int

const (
	PrivilegeViewer     PrivilegeLevel = 1 // 只读查看
	PrivilegeOperator   PrivilegeLevel = 2 // 基本操作
	PrivilegeAdmin      PrivilegeLevel = 3 // 管理员
	PrivilegeSuperAdmin PrivilegeLevel = 4 // 超级管理员
)

// Permission 权限.
type Permission string

const (
	PermViewDashboard  Permission = "VIEW_DASHBOARD"
	PermViewStorage    Permission = "VIEW_STORAGE"
	PermManageStorage  Permission = "MANAGE_STORAGE"
	PermViewUsers      Permission = "VIEW_USERS"
	PermManageUsers    Permission = "MANAGE_USERS"
	PermViewNetwork    Permission = "VIEW_NETWORK"
	PermManageNetwork  Permission = "MANAGE_NETWORK"
	PermViewDocker     Permission = "VIEW_DOCKER"
	PermManageDocker   Permission = "MANAGE_DOCKER"
	PermViewAudit      Permission = "VIEW_AUDIT"
	PermManageAudit    Permission = "MANAGE_AUDIT"
	PermSystemConfig   Permission = "SYSTEM_CONFIG"
	PermBackup         Permission = "BACKUP"
	PermRestore        Permission = "RESTORE"
	PermViewLogs       Permission = "VIEW_LOGS"
	PermManageSecurity Permission = "MANAGE_SECURITY"
)

// AdminUser 管理员用户.
type AdminUser struct {
	ID           string         `json:"id"`
	Username     string         `json:"username"`
	DisplayName  string         `json:"display_name"`
	Level        PrivilegeLevel `json:"level"`
	Permissions  []Permission   `json:"permissions"`
	Enabled      bool           `json:"enabled"`
	CreatedAt    time.Time      `json:"created_at"`
	LastLogin    *time.Time     `json:"last_login,omitempty"`
	FailedLogins int            `json:"failed_logins"`
	TwoFactor    bool           `json:"two_factor"`
	IPWhitelist  []string       `json:"ip_whitelist,omitempty"`
}

// AuditAction 审计动作.
type AuditAction struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Details   string    `json:"details"`
	IP        string    `json:"ip"`
	Timestamp time.Time `json:"timestamp"`
	Success   bool      `json:"success"`
}

// RoleTemplate 角色模板.
type RoleTemplate struct {
	Name        string         `json:"name"`
	Level       PrivilegeLevel `json:"level"`
	Permissions []Permission   `json:"permissions"`
	Description string         `json:"description"`
}

// AdminPrivilegeManager 权限管理器.
type AdminPrivilegeManager struct {
	users    map[string]*AdminUser
	actions  []*AuditAction
	roles    []*RoleTemplate
	dataPath string
	mu       sync.RWMutex
}

// NewAdminPrivilegeManager 创建权限管理器.
func NewAdminPrivilegeManager(dataPath string) *AdminPrivilegeManager {
	os.MkdirAll(dataPath, 0755)
	m := &AdminPrivilegeManager{
		users:    make(map[string]*AdminUser),
		dataPath: dataPath,
	}
	m.loadState()
	m.initRoles()
	return m
}

// CreateUser 创建用户.
func (m *AdminPrivilegeManager) CreateUser(username, displayName string, level PrivilegeLevel) (*AdminUser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, u := range m.users {
		if u.Username == username {
			return nil, fmt.Errorf("user %s already exists", username)
		}
	}
	user := &AdminUser{
		ID:          fmt.Sprintf("admin-%d", time.Now().UnixNano()),
		Username:    username,
		DisplayName: displayName,
		Level:       level,
		Permissions: m.getPermissionsForLevel(level),
		Enabled:     true,
		CreatedAt:   time.Now(),
	}
	m.users[user.ID] = user
	m.saveState()
	return user, nil
}

// UpdateUserLevel 更新用户级别.
func (m *AdminPrivilegeManager) UpdateUserLevel(userID string, level PrivilegeLevel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}
	user.Level = level
	user.Permissions = m.getPermissionsForLevel(level)
	m.saveState()
	return nil
}

// GrantPermission 授予权限.
func (m *AdminPrivilegeManager) GrantPermission(userID string, perm Permission) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}
	for _, p := range user.Permissions {
		if p == perm {
			return nil
		}
	}
	user.Permissions = append(user.Permissions, perm)
	m.saveState()
	return nil
}

// RevokePermission 撤销权限.
func (m *AdminPrivilegeManager) RevokePermission(userID string, perm Permission) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}
	for i, p := range user.Permissions {
		if p == perm {
			user.Permissions = append(user.Permissions[:i], user.Permissions[i+1:]...)
			break
		}
	}
	m.saveState()
	return nil
}

// CheckPermission 检查权限.
func (m *AdminPrivilegeManager) CheckPermission(userID string, perm Permission) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	user, ok := m.users[userID]
	if !ok || !user.Enabled {
		return false
	}
	for _, p := range user.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

// DisableUser 禁用用户.
func (m *AdminPrivilegeManager) DisableUser(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}
	user.Enabled = false
	m.saveState()
	return nil
}

// EnableUser 启用用户.
func (m *AdminPrivilegeManager) EnableUser(userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.users[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}
	user.Enabled = true
	user.FailedLogins = 0
	m.saveState()
	return nil
}

// RecordLogin 记录登录.
func (m *AdminPrivilegeManager) RecordLogin(userID string, success bool, ip string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	user, ok := m.users[userID]
	if !ok {
		return
	}
	if success {
		now := time.Now()
		user.LastLogin = &now
		user.FailedLogins = 0
	} else {
		user.FailedLogins++
		if user.FailedLogins >= 5 {
			user.Enabled = false
		}
	}
	m.logAction(userID, "LOGIN", "", fmt.Sprintf("ip=%s success=%v", ip, success), ip, success)
}

// LogAction 记录操作.
func (m *AdminPrivilegeManager) LogAction(userID, action, resource, details, ip string, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logAction(userID, action, resource, details, ip, success)
}

// GetUsers 获取用户列表.
func (m *AdminPrivilegeManager) GetUsers() []*AdminUser {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var users []*AdminUser
	for _, u := range m.users {
		users = append(users, u)
	}
	return users
}

// GetUser 获取用户.
func (m *AdminPrivilegeManager) GetUser(userID string) (*AdminUser, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[userID]
	return u, ok
}

// GetAuditLog 获取审计日志.
func (m *AdminPrivilegeManager) GetAuditLog(limit int) []*AuditAction {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if limit <= 0 || limit > len(m.actions) {
		limit = len(m.actions)
	}
	start := len(m.actions) - limit
	if start < 0 {
		start = 0
	}
	return m.actions[start:]
}

// GetRoles 获取角色模板.
func (m *AdminPrivilegeManager) GetRoles() []*RoleTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.roles
}

func (m *AdminPrivilegeManager) logAction(userID, action, resource, details, ip string, success bool) {
	m.actions = append(m.actions, &AuditAction{
		ID:        fmt.Sprintf("act-%d", time.Now().UnixNano()),
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Details:   details,
		IP:        ip,
		Timestamp: time.Now(),
		Success:   success,
	})
	if len(m.actions) > 10000 {
		m.actions = m.actions[len(m.actions)-10000:]
	}
}

func (m *AdminPrivilegeManager) getPermissionsForLevel(level PrivilegeLevel) []Permission {
	switch level {
	case PrivilegeViewer:
		return []Permission{PermViewDashboard, PermViewStorage, PermViewNetwork, PermViewLogs}
	case PrivilegeOperator:
		return []Permission{PermViewDashboard, PermViewStorage, PermManageStorage, PermViewNetwork, PermViewDocker, PermViewLogs, PermBackup}
	case PrivilegeAdmin:
		return []Permission{PermViewDashboard, PermViewStorage, PermManageStorage, PermViewUsers, PermManageUsers, PermViewNetwork, PermManageNetwork, PermViewDocker, PermManageDocker, PermViewAudit, PermManageAudit, PermViewLogs, PermBackup, PermRestore, PermManageSecurity}
	case PrivilegeSuperAdmin:
		return []Permission{PermViewDashboard, PermViewStorage, PermManageStorage, PermViewUsers, PermManageUsers, PermViewNetwork, PermManageNetwork, PermViewDocker, PermManageDocker, PermViewAudit, PermManageAudit, PermSystemConfig, PermBackup, PermRestore, PermViewLogs, PermManageSecurity}
	default:
		return []Permission{PermViewDashboard}
	}
}

func (m *AdminPrivilegeManager) initRoles() {
	if len(m.roles) > 0 {
		return
	}
	m.roles = []*RoleTemplate{
		{Name: "查看者", Level: PrivilegeViewer, Permissions: m.getPermissionsForLevel(PrivilegeViewer), Description: "只读访问仪表盘和基本状态"},
		{Name: "操作员", Level: PrivilegeOperator, Permissions: m.getPermissionsForLevel(PrivilegeOperator), Description: "基本存储操作和备份"},
		{Name: "管理员", Level: PrivilegeAdmin, Permissions: m.getPermissionsForLevel(PrivilegeAdmin), Description: "完整管理权限"},
		{Name: "超级管理员", Level: PrivilegeSuperAdmin, Permissions: m.getPermissionsForLevel(PrivilegeSuperAdmin), Description: "系统级完全控制"},
	}
}

func (m *AdminPrivilegeManager) saveState() {
	state := struct {
		Users   map[string]*AdminUser `json:"users"`
		Actions []*AuditAction        `json:"actions"`
	}{m.users, m.actions}
	data, _ := json.MarshalIndent(state, "", "  ")
	os.WriteFile(m.dataPath+"/state.json", data, 0644)
}

func (m *AdminPrivilegeManager) loadState() {
	data, err := os.ReadFile(m.dataPath + "/state.json")
	if err != nil {
		return
	}
	var state struct {
		Users   map[string]*AdminUser `json:"users"`
		Actions []*AuditAction        `json:"actions"`
	}
	json.Unmarshal(data, &state)
	if state.Users != nil {
		m.users = state.Users
	}
	if state.Actions != nil {
		m.actions = state.Actions
	}
}
