package users

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Role 用户角色
type Role string

const (
	// RoleAdmin 管理员角色，拥有全部权限
	RoleAdmin Role = "admin"
	// RoleUser 普通用户角色，受限访问
	RoleUser Role = "user"
	// RoleGuest 访客角色，只读访问
	RoleGuest Role = "guest"
)

// User 用户信息
type User struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"` // 不序列化
	Role         Role       `json:"role"`
	Email        string     `json:"email,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	Disabled     bool       `json:"disabled"`
	HomeDir      string     `json:"home_dir,omitempty"` // 用户主目录
	Groups       []string   `json:"groups,omitempty"`   // 所属用户组
	Config       UserConfig `json:"config,omitempty"`   // 用户配置
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	LastLoginIP  string     `json:"last_login_ip,omitempty"`
}

// UserConfig 用户配置
type UserConfig struct {
	Language         string            `json:"language,omitempty"`          // 界面语言
	Timezone         string            `json:"timezone,omitempty"`          // 时区
	Theme            string            `json:"theme,omitempty"`             // 界面主题
	Notifications    NotificationPrefs `json:"notifications,omitempty"`     // 通知偏好
	StorageQuota     int64             `json:"storage_quota,omitempty"`     // 存储配额 (字节)
	AllowedServices  []string          `json:"allowed_services,omitempty"`  // 允许访问的服务
	DeniedServices   []string          `json:"denied_services,omitempty"`   // 禁止访问的服务
	CustomAttributes map[string]string `json:"custom_attributes,omitempty"` // 自定义属性
}

// NotificationPrefs 通知偏好配置
type NotificationPrefs struct {
	EmailEnabled  bool `json:"email_enabled"`   // 邮件通知
	PushEnabled   bool `json:"push_enabled"`    // 推送通知
	SMSEnabled    bool `json:"sms_enabled"`     // 短信通知
	SystemEnabled bool `json:"system_enabled"`  // 系统通知
	AlertOnLogin  bool `json:"alert_on_login"`  // 登录提醒
	AlertOnChange bool `json:"alert_on_change"` // 变更提醒
}

// UserInput 创建/更新用户输入
type UserInput struct {
	Username string   `json:"username" binding:"required"`
	Password string   `json:"password" binding:"required,min=6"`
	Role     Role     `json:"role"`
	Email    string   `json:"email"`
	HomeDir  string   `json:"home_dir"`
	Groups   []string `json:"groups"`
}

// Group 用户组
type Group struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Members     []string  `json:"members"` // 用户名列表
	CreatedAt   time.Time `json:"created_at"`
}

// GroupInput 创建/更新用户组输入
type GroupInput struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
}

// Token 会话令牌
type Token struct {
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Permission 权限定义
type Permission struct {
	Resource string `json:"resource"` // 资源类型：volume, share, user, system
	Action   string `json:"action"`   // 操作：read, write, delete, admin
}

// persistentConfig 持久化配置
type persistentConfig struct {
	Users  map[string]*User  `json:"users"`
	Groups map[string]*Group `json:"groups"`
	Tokens map[string]*Token `json:"tokens"`
}

// Manager 用户管理器
type Manager struct {
	mu         sync.RWMutex
	users      map[string]*User  // username -> User
	groups     map[string]*Group // group name -> Group
	tokens     map[string]*Token // token -> Token
	mountBase  string
	configPath string
}

var (
	// ErrUserNotFound 用户不存在错误
	ErrUserNotFound = errors.New("用户不存在")
	// ErrUserExists 用户已存在错误
	ErrUserExists = errors.New("用户已存在")
	// ErrInvalidPassword 密码错误
	ErrInvalidPassword = errors.New("密码错误")
	// ErrTokenInvalid 令牌无效或已过期
	ErrTokenInvalid = errors.New("令牌无效或已过期")
	// ErrAdminCannotDelete 不能删除管理员账户
	ErrAdminCannotDelete = errors.New("不能删除管理员账户")
	// ErrGroupNotFound 用户组不存在
	ErrGroupNotFound = errors.New("用户组不存在")
	// ErrGroupExists 用户组已存在
	ErrGroupExists = errors.New("用户组已存在")
	// ErrLastAdmin 系统必须保留至少一个管理员
	ErrLastAdmin = errors.New("系统必须保留至少一个管理员")
)

// NewManager 创建用户管理器
func NewManager(mountBase string) (*Manager, error) {
	return NewManagerWithConfig(mountBase, "")
}

// NewManagerWithConfig 创建用户管理器（带配置文件路径）
func NewManagerWithConfig(mountBase, configPath string) (*Manager, error) {
	m := &Manager{
		users:      make(map[string]*User),
		groups:     make(map[string]*Group),
		tokens:     make(map[string]*Token),
		mountBase:  mountBase,
		configPath: configPath,
	}

	// 尝试加载配置
	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			return nil, fmt.Errorf("加载配置失败：%w", err)
		}
	}

	// 如果没有用户，创建默认管理员
	if len(m.users) == 0 {
		adminUser := &User{
			ID:        generateID(),
			Username:  "admin",
			Role:      RoleAdmin,
			HomeDir:   mountBase + "/admin",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		// 生成随机默认密码（首次登录后应修改）
		// 安全改进：不再使用硬编码密码
		defaultPassword := generateRandomPassword(16)
		hash, err := bcrypt.GenerateFromPassword([]byte(defaultPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		adminUser.PasswordHash = string(hash)
		m.users["admin"] = adminUser

		// 将初始密码输出到控制台（仅首次启动）
		// 生产环境建议：通过邮件或安全渠道发送初始密码
		// 注意：密码仅输出到 stdout，不记录到日志文件
		fmt.Println("========================================")
		fmt.Println("⚠️  首次启动：默认管理员账号已创建")
		fmt.Println("   用户名: admin")
		fmt.Printf("   密码: %s\n", defaultPassword)
		fmt.Println("   请立即登录并修改密码！")
		fmt.Println("========================================")
	}

	return m, nil
}

// loadConfig 从文件加载配置
func (m *Manager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil // 配置文件不存在，使用默认配置
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败：%w", err)
	}

	var pc persistentConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return fmt.Errorf("解析配置文件失败：%w", err)
	}

	if pc.Users != nil {
		m.users = pc.Users
	}
	if pc.Groups != nil {
		m.groups = pc.Groups
	}
	if pc.Tokens != nil {
		// 清理过期令牌
		now := time.Now()
		for token, t := range pc.Tokens {
			if now.After(t.ExpiresAt) {
				delete(pc.Tokens, token)
			}
		}
		m.tokens = pc.Tokens
	}

	return nil
}

// saveConfig 保存配置到文件
// 注意：调用者必须持有 m.mu 锁
func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	pc := persistentConfig{
		Users:  m.users,
		Groups: m.groups,
		Tokens: m.tokens,
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败：%w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败：%w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0600); err != nil {
		return fmt.Errorf("写入配置文件失败：%w", err)
	}

	return nil
}

// generateID 生成用户 ID
func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// generateToken 生成会话令牌
func generateToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// ========== 用户管理 ==========

// CreateUser 创建用户
func (m *Manager) CreateUser(input UserInput) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.users[input.Username]; exists {
		return nil, ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	role := input.Role
	if role == "" {
		role = RoleUser
	}

	homeDir := input.HomeDir
	if homeDir == "" {
		homeDir = m.mountBase + "/" + input.Username
	}

	user := &User{
		ID:           generateID(),
		Username:     input.Username,
		PasswordHash: string(hash),
		Role:         role,
		Email:        input.Email,
		HomeDir:      homeDir,
		Groups:       input.Groups,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.users[input.Username] = user

	// 保存配置
	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}

	return user, nil
}

// GetUser 获取用户
func (m *Manager) GetUser(username string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[username]
	if !exists {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// GetUserByID 通过 ID 获取用户
func (m *Manager) GetUserByID(id string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, user := range m.users {
		if user.ID == id {
			return user, nil
		}
	}
	return nil, ErrUserNotFound
}

// ListUsers 获取用户列表
func (m *Manager) ListUsers() []*User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	users := make([]*User, 0, len(m.users))
	for _, u := range m.users {
		// 不返回密码哈希
		userCopy := &User{
			ID:        u.ID,
			Username:  u.Username,
			Role:      u.Role,
			Email:     u.Email,
			CreatedAt: u.CreatedAt,
			UpdatedAt: u.UpdatedAt,
			Disabled:  u.Disabled,
			HomeDir:   u.HomeDir,
			Groups:    u.Groups,
		}
		users = append(users, userCopy)
	}
	return users
}

// UpdateUser 更新用户信息
func (m *Manager) UpdateUser(username string, input UserInput) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return nil, ErrUserNotFound
	}

	if input.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		user.PasswordHash = string(hash)
	}

	if input.Role != "" {
		user.Role = input.Role
	}
	if input.Email != "" {
		user.Email = input.Email
	}
	if input.HomeDir != "" {
		user.HomeDir = input.HomeDir
	}
	if input.Groups != nil {
		user.Groups = input.Groups
	}
	user.UpdatedAt = time.Now()

	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return user, nil
}

// DeleteUser 删除用户
func (m *Manager) DeleteUser(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	if user.Role == RoleAdmin {
		// 检查是否是最后一个管理员
		adminCount := 0
		for _, u := range m.users {
			if u.Role == RoleAdmin {
				adminCount++
			}
		}
		if adminCount <= 1 {
			return ErrLastAdmin
		}
	}

	delete(m.users, username)
	// 清理相关令牌
	for token, t := range m.tokens {
		if t.UserID == user.ID {
			delete(m.tokens, token)
		}
	}
	// 从用户组中移除
	for _, group := range m.groups {
		m.removeMemberFromGroup(group, username)
	}

	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// ========== 用户组管理 ==========

// CreateGroup 创建用户组
func (m *Manager) CreateGroup(input GroupInput) (*Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.groups[input.Name]; exists {
		return nil, ErrGroupExists
	}

	group := &Group{
		ID:          generateID(),
		Name:        input.Name,
		Description: input.Description,
		Members:     input.Members,
		CreatedAt:   time.Now(),
	}

	m.groups[input.Name] = group

	// 更新用户的组信息
	for _, username := range input.Members {
		if user, exists := m.users[username]; exists {
			user.Groups = append(user.Groups, input.Name)
		}
	}

	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return group, nil
}

// GetGroup 获取用户组
func (m *Manager) GetGroup(name string) (*Group, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, exists := m.groups[name]
	if !exists {
		return nil, ErrGroupNotFound
	}
	return group, nil
}

// ListGroups 获取用户组列表
func (m *Manager) ListGroups() []*Group {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*Group, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	return groups
}

// UpdateGroup 更新用户组
func (m *Manager) UpdateGroup(name string, input GroupInput) (*Group, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, exists := m.groups[name]
	if !exists {
		return nil, ErrGroupNotFound
	}

	group.Description = input.Description

	// 更新成员
	oldMembers := make(map[string]bool)
	for _, m := range group.Members {
		oldMembers[m] = true
	}

	newMembers := make(map[string]bool)
	for _, m := range input.Members {
		newMembers[m] = true
	}

	// 移除旧成员
	for username := range oldMembers {
		if !newMembers[username] {
			m.removeMemberFromGroup(group, username)
			if user, exists := m.users[username]; exists {
				user.Groups = m.removeString(user.Groups, name)
			}
		}
	}

	// 添加新成员
	for username := range newMembers {
		if !oldMembers[username] {
			group.Members = append(group.Members, username)
			if user, exists := m.users[username]; exists {
				user.Groups = append(user.Groups, name)
			}
		}
	}

	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return group, nil
}

// DeleteGroup 删除用户组
func (m *Manager) DeleteGroup(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, exists := m.groups[name]
	if !exists {
		return ErrGroupNotFound
	}

	// 从用户中移除组信息
	for _, username := range group.Members {
		if user, exists := m.users[username]; exists {
			user.Groups = m.removeString(user.Groups, name)
		}
	}

	delete(m.groups, name)
	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// AddUserToGroup 将用户添加到组
func (m *Manager) AddUserToGroup(username, groupName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	group, exists := m.groups[groupName]
	if !exists {
		return ErrGroupNotFound
	}

	// 检查是否已在组中
	for _, g := range user.Groups {
		if g == groupName {
			return nil // 已在组中
		}
	}

	user.Groups = append(user.Groups, groupName)
	group.Members = append(group.Members, username)

	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// RemoveUserFromGroup 从组中移除用户
func (m *Manager) RemoveUserFromGroup(username, groupName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	group, exists := m.groups[groupName]
	if !exists {
		return ErrGroupNotFound
	}

	user.Groups = m.removeString(user.Groups, groupName)
	group.Members = m.removeString(group.Members, username)

	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

func (m *Manager) removeMemberFromGroup(group *Group, username string) {
	group.Members = m.removeString(group.Members, username)
}

func (m *Manager) removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

// ========== 认证管理 ==========

// Authenticate 验证用户登录
func (m *Manager) Authenticate(username, password string) (*Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return nil, ErrUserNotFound
	}

	if user.Disabled {
		return nil, errors.New("账户已禁用")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidPassword
	}

	// 创建令牌（24 小时有效期）
	token := &Token{
		UserID:    user.ID,
		Token:     generateToken(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	m.tokens[token.Token] = token

	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return token, nil
}

// ValidateToken 验证令牌
func (m *Manager) ValidateToken(tokenStr string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	token, exists := m.tokens[tokenStr]
	if !exists {
		return nil, ErrTokenInvalid
	}

	if time.Now().After(token.ExpiresAt) {
		return nil, ErrTokenInvalid
	}

	// 查找用户
	for _, user := range m.users {
		if user.ID == token.UserID {
			if user.Disabled {
				return nil, errors.New("账户已禁用")
			}
			return user, nil
		}
	}

	return nil, ErrTokenInvalid
}

// Logout 登出（使令牌失效）
func (m *Manager) Logout(tokenStr string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, tokenStr)
	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
}

// RefreshToken 刷新令牌
func (m *Manager) RefreshToken(tokenStr string) (*Token, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	token, exists := m.tokens[tokenStr]
	if !exists {
		return nil, ErrTokenInvalid
	}

	// 延长有效期
	token.ExpiresAt = time.Now().Add(24 * time.Hour)
	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return token, nil
}

// ========== 用户状态管理 ==========

// DisableUser 禁用/启用用户
func (m *Manager) DisableUser(username string, disabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	if user.Role == RoleAdmin && disabled {
		// 检查是否是最后一个启用的管理员
		activeAdmins := 0
		for _, u := range m.users {
			if u.Role == RoleAdmin && !u.Disabled {
				activeAdmins++
			}
		}
		if activeAdmins <= 1 {
			return errors.New("不能禁用最后一个管理员账户")
		}
	}

	user.Disabled = disabled
	user.UpdatedAt = time.Now()
	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// ChangePassword 修改密码
func (m *Manager) ChangePassword(username, oldPassword, newPassword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	// 验证旧密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)); err != nil {
		return ErrInvalidPassword
	}

	// 设置新密码
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	user.UpdatedAt = time.Now()
	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// ResetPassword 重置密码（管理员操作）
func (m *Manager) ResetPassword(username, newPassword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	user.UpdatedAt = time.Now()
	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// ========== 权限管理 ==========

// HasPermission 检查用户是否有指定权限
func (m *Manager) HasPermission(user *User, resource, action string) bool {
	if user == nil {
		return false
	}

	switch user.Role {
	case RoleAdmin:
		return true // 管理员拥有全部权限
	case RoleUser:
		// 普通用户不能执行管理操作
		if action == "admin" {
			return false
		}
		return action == "read" || action == "write"
	case RoleGuest:
		// 访客只能读取
		return action == "read"
	default:
		return false
	}
}

// CheckPermission 检查用户权限（通过用户名）
func (m *Manager) CheckPermission(username, resource, action string) (bool, error) {
	user, err := m.GetUser(username)
	if err != nil {
		return false, err
	}
	return m.HasPermission(user, resource, action), nil
}

// GetUsersByRole 获取指定角色的用户列表
func (m *Manager) GetUsersByRole(role Role) []*User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	users := make([]*User, 0)
	for _, u := range m.users {
		if u.Role == role {
			users = append(users, u)
		}
	}
	return users
}

// GetUsersInGroup 获取用户组中的用户列表
func (m *Manager) GetUsersInGroup(groupName string) ([]*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, exists := m.groups[groupName]
	if !exists {
		return nil, ErrGroupNotFound
	}

	users := make([]*User, 0, len(group.Members))
	for _, username := range group.Members {
		if user, exists := m.users[username]; exists {
			users = append(users, user)
		}
	}
	return users, nil
}

// SetUserRole 设置用户角色
func (m *Manager) SetUserRole(username string, role Role) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	// 如果要移除管理员权限，检查是否是最后一个管理员
	if user.Role == RoleAdmin && role != RoleAdmin {
		adminCount := 0
		for _, u := range m.users {
			if u.Role == RoleAdmin && !u.Disabled {
				adminCount++
			}
		}
		if adminCount <= 1 {
			return ErrLastAdmin
		}
	}

	user.Role = role
	user.UpdatedAt = time.Now()
	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// ========== 安全辅助函数 ==========

// generateRandomPassword 生成随机密码
func generateRandomPassword(length int) string {
	const (
		uppercase = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		lowercase = "abcdefghijklmnopqrstuvwxyz"
		digits    = "0123456789"
		special   = "!@#$%^&*"
	)
	allChars := uppercase + lowercase + digits + special

	// 确保密码包含各类字符
	password := make([]byte, length)

	// 至少一个大写字母
	password[0] = uppercase[mustRandomInt(len(uppercase))]
	// 至少一个小写字母
	password[1] = lowercase[mustRandomInt(len(lowercase))]
	// 至少一个数字
	password[2] = digits[mustRandomInt(len(digits))]
	// 至少一个特殊字符
	password[3] = special[mustRandomInt(len(special))]

	// 填充其余字符
	for i := 4; i < length; i++ {
		password[i] = allChars[mustRandomInt(len(allChars))]
	}

	// 打乱顺序
	for i := range password {
		j := mustRandomInt(len(password))
		password[i], password[j] = password[j], password[i]
	}

	return string(password)
}

// mustRandomInt 生成随机整数
func mustRandomInt(max int) int {
	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic(err) // 如果系统随机数生成器失败，应该立即终止
	}
	return int(n.Int64())
}

// ========== 用户配置管理 ==========

// GetUserConfig 获取用户配置
func (m *Manager) GetUserConfig(username string) (*UserConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[username]
	if !exists {
		return nil, ErrUserNotFound
	}

	// 返回配置副本
	config := user.Config
	return &config, nil
}

// UpdateUserConfig 更新用户配置
func (m *Manager) UpdateUserConfig(username string, config UserConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	user.Config = config
	user.UpdatedAt = time.Now()

	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// PatchUserConfig 部分更新用户配置
func (m *Manager) PatchUserConfig(username string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	// 应用更新
	for key, value := range updates {
		switch key {
		case "language":
			if v, ok := value.(string); ok {
				user.Config.Language = v
			}
		case "timezone":
			if v, ok := value.(string); ok {
				user.Config.Timezone = v
			}
		case "theme":
			if v, ok := value.(string); ok {
				user.Config.Theme = v
			}
		case "notifications":
			if v, ok := value.(NotificationPrefs); ok {
				user.Config.Notifications = v
			}
		case "storage_quota":
			if v, ok := value.(int64); ok {
				user.Config.StorageQuota = v
			}
		case "allowed_services":
			if v, ok := value.([]string); ok {
				user.Config.AllowedServices = v
			}
		case "denied_services":
			if v, ok := value.([]string); ok {
				user.Config.DeniedServices = v
			}
		case "custom_attributes":
			if v, ok := value.(map[string]string); ok {
				user.Config.CustomAttributes = v
			}
		}
	}

	user.UpdatedAt = time.Now()
	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// SetUserLanguage 设置用户语言
func (m *Manager) SetUserLanguage(username, language string) error {
	return m.PatchUserConfig(username, map[string]interface{}{"language": language})
}

// SetUserTimezone 设置用户时区
func (m *Manager) SetUserTimezone(username, timezone string) error {
	return m.PatchUserConfig(username, map[string]interface{}{"timezone": timezone})
}

// SetUserTheme 设置用户主题
func (m *Manager) SetUserTheme(username, theme string) error {
	return m.PatchUserConfig(username, map[string]interface{}{"theme": theme})
}

// SetUserStorageQuota 设置用户存储配额
func (m *Manager) SetUserStorageQuota(username string, quota int64) error {
	return m.PatchUserConfig(username, map[string]interface{}{"storage_quota": quota})
}

// GetUserStorageQuota 获取用户存储配额
func (m *Manager) GetUserStorageQuota(username string) (int64, error) {
	config, err := m.GetUserConfig(username)
	if err != nil {
		return 0, err
	}
	return config.StorageQuota, nil
}

// SetUserNotificationPrefs 设置用户通知偏好
func (m *Manager) SetUserNotificationPrefs(username string, prefs NotificationPrefs) error {
	return m.PatchUserConfig(username, map[string]interface{}{"notifications": prefs})
}

// AllowService 允许用户访问指定服务
func (m *Manager) AllowService(username, service string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	// 从禁止列表移除
	user.Config.DeniedServices = m.removeString(user.Config.DeniedServices, service)

	// 添加到允许列表
	for _, s := range user.Config.AllowedServices {
		if s == service {
			return nil // 已在允许列表
		}
	}
	user.Config.AllowedServices = append(user.Config.AllowedServices, service)

	user.UpdatedAt = time.Now()
	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// DenyService 禁止用户访问指定服务
func (m *Manager) DenyService(username, service string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	// 从允许列表移除
	user.Config.AllowedServices = m.removeString(user.Config.AllowedServices, service)

	// 添加到禁止列表
	for _, s := range user.Config.DeniedServices {
		if s == service {
			return nil // 已在禁止列表
		}
	}
	user.Config.DeniedServices = append(user.Config.DeniedServices, service)

	user.UpdatedAt = time.Now()
	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// SetCustomAttribute 设置用户自定义属性
func (m *Manager) SetCustomAttribute(username, key, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	if user.Config.CustomAttributes == nil {
		user.Config.CustomAttributes = make(map[string]string)
	}
	user.Config.CustomAttributes[key] = value
	user.UpdatedAt = time.Now()

	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// GetCustomAttribute 获取用户自定义属性
func (m *Manager) GetCustomAttribute(username, key string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	user, exists := m.users[username]
	if !exists {
		return "", ErrUserNotFound
	}

	if user.Config.CustomAttributes == nil {
		return "", nil
	}
	return user.Config.CustomAttributes[key], nil
}

// ========== 用户统计与审计 ==========

// UpdateLastLogin 更新用户最后登录信息
func (m *Manager) UpdateLastLogin(username, ip string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	now := time.Now()
	user.LastLoginAt = &now
	user.LastLoginIP = ip
	user.UpdatedAt = now

	if err := m.saveConfig(); err != nil {
		log.Printf("保存配置失败: %v", err)
	}
	return nil
}

// GetUserStats 获取用户统计信息
func (m *Manager) GetUserStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_users":  len(m.users),
		"total_groups": len(m.groups),
		"total_tokens": len(m.tokens),
		"by_role":      make(map[Role]int),
		"by_status": map[string]int{
			"active":   0,
			"disabled": 0,
		},
	}

	roleStats, ok := stats["by_role"].(map[Role]int)
	if !ok {
		roleStats = make(map[Role]int)
		stats["by_role"] = roleStats
	}
	statusStats, ok := stats["by_status"].(map[string]int)
	if !ok {
		statusStats = make(map[string]int)
		stats["by_status"] = statusStats
	}

	for _, user := range m.users {
		roleStats[user.Role]++
		if user.Disabled {
			statusStats["disabled"]++
		} else {
			statusStats["active"]++
		}
	}

	return stats
}

// GetActiveUsers 获取活跃用户列表（指定时间内登录过）
func (m *Manager) GetActiveUsers(since time.Time) []*User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	users := make([]*User, 0)
	for _, u := range m.users {
		if u.LastLoginAt != nil && u.LastLoginAt.After(since) {
			users = append(users, u)
		}
	}
	return users
}

// GetInactiveUsers 获取不活跃用户列表（超过指定时间未登录）
func (m *Manager) GetInactiveUsers(duration time.Duration) []*User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	users := make([]*User, 0)
	for _, u := range m.users {
		if u.LastLoginAt == nil || u.LastLoginAt.Before(cutoff) {
			users = append(users, u)
		}
	}
	return users
}

// ========== 批量操作 ==========

// BatchCreateUsers 批量创建用户
func (m *Manager) BatchCreateUsers(inputs []UserInput) ([]*User, []error) {
	results := make([]*User, 0, len(inputs))
	errs := make([]error, 0)

	for _, input := range inputs {
		user, err := m.CreateUser(input)
		if err != nil {
			errs = append(errs, fmt.Errorf("创建用户 %s 失败: %w", input.Username, err))
		} else {
			results = append(results, user)
		}
	}

	return results, errs
}

// BatchDeleteUsers 批量删除用户
func (m *Manager) BatchDeleteUsers(usernames []string) []error {
	errs := make([]error, 0)
	for _, username := range usernames {
		if err := m.DeleteUser(username); err != nil {
			errs = append(errs, fmt.Errorf("删除用户 %s 失败: %w", username, err))
		}
	}
	return errs
}

// BatchAddToGroup 批量将用户添加到组
func (m *Manager) BatchAddToGroup(usernames []string, groupName string) []error {
	errs := make([]error, 0)
	for _, username := range usernames {
		if err := m.AddUserToGroup(username, groupName); err != nil {
			errs = append(errs, fmt.Errorf("添加用户 %s 到组 %s 失败: %w", username, groupName, err))
		}
	}
	return errs
}
