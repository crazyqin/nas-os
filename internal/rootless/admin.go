package rootless

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"sync"
	"time"
)

// RootlessAdminManager 无Root管理员管理器
// 对标 TrueNAS SCALE 25.10 的 Rootless Admin 功能
// 允许管理员在不使用 root 权限的情况下执行管理操作
type RootlessAdminManager struct {
	mu       sync.RWMutex
	config   *RootlessConfig
	admins   map[string]*AdminProfile
	auditLog *AuditLogger
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// RootlessConfig 无Root管理配置
type RootlessConfig struct {
	Enabled         bool     `json:"enabled"`
	AdminGroup      string   `json:"admin_group"`       // 管理员组名
	AllowedCommands []string `json:"allowed_commands"`   // 允许的命令白名单
	DeniedPaths     []string `json:"denied_paths"`       // 禁止访问的路径
	MaxSessionTime  int      `json:"max_session_time"`   // 最大会话时间(分钟)
	AuditEnabled    bool     `json:"audit_enabled"`
	SudoersDir      string   `json:"sudoers_dir"`
}

// AdminProfile 管理员配置文件
type AdminProfile struct {
	UserID       string          `json:"user_id"`
	Username     string          `json:"username"`
	Group        string          `json:"group"`
	Privileges   []Privilege     `json:"privileges"`
	IsActive     bool            `json:"is_active"`
	LastLogin    time.Time       `json:"last_login"`
	LoginCount   int             `json:"login_count"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	MaxSessions  int             `json:"max_sessions"`
	CurrSessions int             `json:"current_sessions"`
}

// Privilege 权限
type Privilege struct {
	Resource   string   `json:"resource"`   // 资源类型: storage, network, docker, etc.
	Actions    []string `json:"actions"`    // 允许的操作: read, write, delete, admin
	Exceptions []string `json:"exceptions"` // 例外情况
}

// AuditLogEntry 审计日志条目
type AuditLogEntry struct {
	Timestamp   time.Time `json:"timestamp"`
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	Action      string    `json:"action"`
	Resource    string    `json:"resource"`
	Command     string    `json:"command"`
	Success     bool      `json:"success"`
	ErrorMsg    string    `json:"error_msg,omitempty"`
	IPAddress   string    `json:"ip_address"`
	SessionID   string    `json:"session_id"`
}

// AuditLogger 审计日志记录器
type AuditLogger struct {
	mu       sync.Mutex
	logFile  string
	entries  []AuditLogEntry
	maxSize  int
}

// NewRootlessAdminManager 创建无Root管理员管理器
func NewRootlessAdminManager(cfg *RootlessConfig) *RootlessAdminManager {
	if cfg == nil {
		cfg = &RootlessConfig{
			Enabled:    true,
			AdminGroup: "nas-admins",
			AllowedCommands: []string{
				"/usr/bin/systemctl",
				"/usr/bin/docker",
				"/usr/sbin/btrfs",
				"/usr/sbin/smbcontrol",
				"/usr/bin/journalctl",
				"/usr/bin/top",
				"/usr/bin/htop",
			},
			DeniedPaths: []string{
				"/etc/shadow",
				"/etc/gshadow",
				"/root/.ssh",
			},
			MaxSessionTime: 480, // 8 hours
			AuditEnabled:   true,
			SudoersDir:     "/etc/sudoers.d",
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &RootlessAdminManager{
		config: cfg,
		admins: make(map[string]*AdminProfile),
		auditLog: &AuditLogger{
			entries: make([]AuditLogEntry, 0),
			maxSize: 10000,
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动管理器
func (m *RootlessAdminManager) Start() error {
	if !m.config.Enabled {
		return nil
	}

	// 确保管理员组存在
	if err := m.ensureAdminGroup(); err != nil {
		return fmt.Errorf("创建管理员组失败: %w", err)
	}

	// 配置 sudoers 规则
	if err := m.configureSudoers(); err != nil {
		return fmt.Errorf("配置 sudoers 失败: %w", err)
	}

	m.wg.Add(1)
	go m.sessionMonitor()

	return nil
}

// Stop 停止管理器
func (m *RootlessAdminManager) Stop() error {
	m.cancel()
	m.wg.Wait()
	return nil
}

// RegisterAdmin 注册管理员
func (m *RootlessAdminManager) RegisterAdmin(username string, privileges []Privilege) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查用户是否存在
	u, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("用户 %s 不存在: %w", username, err)
	}

	// 添加到管理员组
	if err := m.addToAdminGroup(u); err != nil {
		return fmt.Errorf("添加到管理员组失败: %w", err)
	}

	profile := &AdminProfile{
		UserID:     u.Uid,
		Username:   username,
		Group:      m.config.AdminGroup,
		Privileges: privileges,
		IsActive:   true,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		MaxSessions: 3,
	}

	m.admins[username] = profile

	if m.config.AuditEnabled {
		m.auditLog.Log(AuditLogEntry{
			Timestamp: time.Now(),
			UserID:    u.Uid,
			Username:  username,
			Action:    "register_admin",
			Resource:  "rootless_admin",
			Success:   true,
		})
	}

	return nil
}

// RevokeAdmin 撤销管理员权限
func (m *RootlessAdminManager) RevokeAdmin(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, exists := m.admins[username]
	if !exists {
		return fmt.Errorf("管理员 %s 不存在", username)
	}

	// 从管理员组移除
	u, err := user.Lookup(username)
	if err == nil {
		m.removeFromAdminGroup(u)
	}

	profile.IsActive = false
	profile.UpdatedAt = time.Now()

	if m.config.AuditEnabled {
		m.auditLog.Log(AuditLogEntry{
			Timestamp: time.Now(),
			UserID:    profile.UserID,
			Username:  username,
			Action:    "revoke_admin",
			Resource:  "rootless_admin",
			Success:   true,
		})
	}

	return nil
}

// CheckPrivilege 检查用户是否有特定权限
func (m *RootlessAdminManager) CheckPrivilege(username, resource, action string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, exists := m.admins[username]
	if !exists || !profile.IsActive {
		return false
	}

	for _, priv := range profile.Privileges {
		if priv.Resource == resource {
			for _, a := range priv.Actions {
				if a == action || a == "admin" {
					return true
				}
			}
		}
	}

	return false
}

// ExecuteCommand 以无root方式执行管理命令
func (m *RootlessAdminManager) ExecuteCommand(username string, cmd string, args ...string) (string, error) {
	m.mu.RLock()
	profile, exists := m.admins[username]
	m.mu.RUnlock()

	if !exists || !profile.IsActive {
		return "", fmt.Errorf("用户 %s 不是有效管理员", username)
	}

	// 检查命令白名单
	if !m.isCommandAllowed(cmd) {
		if m.config.AuditEnabled {
			m.auditLog.Log(AuditLogEntry{
				Timestamp: time.Now(),
				UserID:    profile.UserID,
				Username:  username,
				Action:    "execute_command",
				Resource:  "command",
				Command:   cmd,
				Success:   false,
				ErrorMsg:  "命令不在白名单中",
			})
		}
		return "", fmt.Errorf("命令 %s 不在允许列表中", cmd)
	}

	// 检查会话数限制
	if profile.CurrSessions >= profile.MaxSessions {
		return "", fmt.Errorf("已达到最大并发会话数 %d", profile.MaxSessions)
	}

	profile.CurrSessions++
	defer func() { profile.CurrSessions-- }()

	// 通过 sudo 以管理员组权限执行
	ctx, cancel := context.WithTimeout(m.ctx, 30*time.Second)
	defer cancel()

	execCmd := exec.CommandContext(ctx, "sudo", append([]string{"-g", m.config.AdminGroup, cmd}, args...)...)
	output, err := execCmd.CombinedOutput()

	if m.config.AuditEnabled {
		m.auditLog.Log(AuditLogEntry{
			Timestamp: time.Now(),
			UserID:    profile.UserID,
			Username:  username,
			Action:    "execute_command",
			Resource:  "command",
			Command:   cmd,
			Success:   err == nil,
			ErrorMsg:  func() string { if err != nil { return err.Error() }; return "" }(),
		})
	}

	return string(output), err
}

// GetAuditLog 获取审计日志
func (m *RootlessAdminManager) GetAuditLog(limit int) []AuditLogEntry {
	m.auditLog.mu.Lock()
	defer m.auditLog.mu.Unlock()

	if limit <= 0 || limit > len(m.auditLog.entries) {
		limit = len(m.auditLog.entries)
	}

	start := len(m.auditLog.entries) - limit
	result := make([]AuditLogEntry, limit)
	copy(result, m.auditLog.entries[start:])
	return result
}

func (m *RootlessAdminManager) isCommandAllowed(cmd string) bool {
	for _, allowed := range m.config.AllowedCommands {
		if cmd == allowed {
			return true
		}
	}
	return false
}

func (m *RootlessAdminManager) ensureAdminGroup() error {
	_, err := user.LookupGroup(m.config.AdminGroup)
	if err != nil {
		cmd := exec.Command("groupadd", m.config.AdminGroup)
		return cmd.Run()
	}
	return nil
}

func (m *RootlessAdminManager) configureSudoers() error {
	sudoersContent := fmt.Sprintf(`# NAS-OS Rootless Admin Configuration
# Generated by RootlessAdminManager
%%%-s ALL=(ALL) NOPASSWD: %s
Defaults:%-s !requiretty
`, m.config.AdminGroup, m.joinCommands(), m.config.AdminGroup)

	sudoersFile := m.config.SudoersDir + "/nas-os-rootless"
	return os.WriteFile(sudoersFile, []byte(sudoersContent), 0440)
}

func (m *RootlessAdminManager) joinCommands() string {
	result := ""
	for i, cmd := range m.config.AllowedCommands {
		if i > 0 {
			result += ", "
		}
		result += cmd
	}
	return result
}

func (m *RootlessAdminManager) addToAdminGroup(u *user.User) error {
	cmd := exec.Command("usermod", "-a", "-G", m.config.AdminGroup, u.Username)
	return cmd.Run()
}

func (m *RootlessAdminManager) removeFromAdminGroup(u *user.User) error {
	cmd := exec.Command("gpasswd", "-d", u.Username, m.config.AdminGroup)
	return cmd.Run()
}

func (m *RootlessAdminManager) sessionMonitor() {
	defer m.wg.Done()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanupExpiredSessions()
		}
	}
}

func (m *RootlessAdminManager) cleanupExpiredSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, profile := range m.admins {
		if profile.CurrSessions > 0 && time.Since(profile.LastLogin) > time.Duration(m.config.MaxSessionTime)*time.Minute {
			profile.CurrSessions = 0
		}
	}
}

func (l *AuditLogger) Log(entry AuditLogEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, entry)
	if len(l.entries) > l.maxSize {
		l.entries = l.entries[len(l.entries)-l.maxSize:]
	}
}

// GetAdminCount 获取管理员数量
func (m *RootlessAdminManager) GetAdminCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	count := 0
	for _, p := range m.admins {
		if p.IsActive {
			count++
		}
	}
	return count
}

// ListAdmins 列出所有管理员
func (m *RootlessAdminManager) ListAdmins() []*AdminProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*AdminProfile, 0, len(m.admins))
	for _, p := range m.admins {
		if p.IsActive {
			result = append(result, p)
		}
	}
	return result
}

func uidStrToInt(uid string) int {
	v, _ := strconv.Atoi(uid)
	return v
}
