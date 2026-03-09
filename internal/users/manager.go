package users

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Role 用户角色
type Role string

const (
	RoleAdmin  Role = "admin"  // 管理员：全部权限
	RoleUser   Role = "user"   // 普通用户：受限访问
	RoleGuest  Role = "guest"  // 访客：只读访问
)

// User 用户信息
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"` // 不序列化
	Role         Role      `json:"role"`
	Email        string    `json:"email,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Disabled     bool      `json:"disabled"`
	HomeDir      string    `json:"home_dir,omitempty"` // 用户主目录
}

// UserInput 创建/更新用户输入
type UserInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=6"`
	Role     Role   `json:"role"`
	Email    string `json:"email"`
	HomeDir  string `json:"home_dir"`
}

// Token 会话令牌
type Token struct {
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Manager 用户管理器
type Manager struct {
	mu       sync.RWMutex
	users    map[string]*User   // username -> User
	tokens   map[string]*Token  // token -> Token
	mountBase string
}

var (
	ErrUserNotFound     = errors.New("用户不存在")
	ErrUserExists       = errors.New("用户已存在")
	ErrInvalidPassword  = errors.New("密码错误")
	ErrTokenInvalid     = errors.New("令牌无效或已过期")
	ErrAdminCannotDelete = errors.New("不能删除管理员账户")
)

// NewManager 创建用户管理器
func NewManager(mountBase string) (*Manager, error) {
	m := &Manager{
		users:     make(map[string]*User),
		tokens:    make(map[string]*Token),
		mountBase: mountBase,
	}

	// 创建默认管理员账户
	adminUser := &User{
		ID:       "admin",
		Username: "admin",
		Role:     RoleAdmin,
		HomeDir:  mountBase + "/admin",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	// 默认密码：admin123（首次登录后应修改）
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	adminUser.PasswordHash = string(hash)
	m.users["admin"] = adminUser

	return m, nil
}

// generateID 生成用户 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateToken 生成会话令牌
func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

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
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.users[input.Username] = user
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
	user.UpdatedAt = time.Now()

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
		return ErrAdminCannotDelete
	}

	delete(m.users, username)
	// 清理相关令牌
	for token, t := range m.tokens {
		if t.UserID == user.ID {
			delete(m.tokens, token)
		}
	}
	return nil
}

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
	}
	m.tokens[token.Token] = token

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
}

// DisableUser 禁用/启用用户
func (m *Manager) DisableUser(username string, disabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[username]
	if !exists {
		return ErrUserNotFound
	}

	if user.Role == RoleAdmin && disabled {
		return errors.New("不能禁用管理员账户")
	}

	user.Disabled = disabled
	user.UpdatedAt = time.Now()
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
	return nil
}

// HasPermission 检查用户是否有指定权限
func (m *Manager) HasPermission(user *User, action string) bool {
	if user == nil {
		return false
	}

	switch user.Role {
	case RoleAdmin:
		return true // 管理员拥有全部权限
	case RoleUser:
		// 普通用户不能执行管理操作
		return action != "admin"
	case RoleGuest:
		// 访客只能读取
		return action == "read"
	default:
		return false
	}
}
