package smb

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"nas-os/internal/users"
)

// Share SMB 共享配置
type Share struct {
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	Comment     string            `json:"comment"`
	ReadOnly    bool              `json:"read_only"`
	GuestAccess bool              `json:"guest_access"`
	AllowedUsers []string         `json:"allowed_users"`
	Browseable  bool              `json:"browseable"`
	VetoFiles   []string          `json:"veto_files"` // 隐藏的文件类型
}

// ShareInput 创建/更新共享输入
type ShareInput struct {
	Name        string   `json:"name" binding:"required"`
	Path        string   `json:"path" binding:"required"`
	Comment     string   `json:"comment"`
	ReadOnly    bool     `json:"read_only"`
	GuestAccess bool     `json:"guest_access"`
	AllowedUsers []string `json:"allowed_users"`
	Browseable  bool     `json:"browseable"`
}

// Config SMB 配置
type Config struct {
	Enabled      bool     `json:"enabled"`
	Workgroup    string   `json:"workgroup"`
	ServerString string   `json:"server_string"`
	GuestAccess  bool     `json:"guest_access"`
	MinProtocol  string   `json:"min_protocol"` // SMB2 或 SMB3
	MaxProtocol  string   `json:"max_protocol"`
}

// Manager SMB 管理器
type Manager struct {
	mu          sync.RWMutex
	shares      map[string]*Share
	config      *Config
	userManager *users.Manager
	configPath  string
}

var (
	defaultConfig = &Config{
		Enabled:      true,
		Workgroup:    "WORKGROUP",
		ServerString: "NAS-OS Samba Server",
		GuestAccess:  false,
		MinProtocol:  "SMB2",
		MaxProtocol:  "SMB3",
	}
)

// NewManager 创建 SMB 管理器
func NewManager(userMgr *users.Manager, configPath string) (*Manager, error) {
	m := &Manager{
		shares:      make(map[string]*Share),
		config:      defaultConfig,
		userManager: userMgr,
		configPath:  configPath,
	}

	// 加载现有配置（如果有）
	if err := m.loadConfig(); err != nil {
		return nil, err
	}

	return m, nil
}

// loadConfig 从文件加载配置
func (m *Manager) loadConfig() error {
	// TODO: 从配置文件加载
	// 目前使用默认配置
	return nil
}

// saveConfig 保存配置到文件
func (m *Manager) saveConfig() error {
	// TODO: 保存到配置文件
	return nil
}

// generateSmbConf 生成 Samba 配置文件内容
func (m *Manager) generateSmbConf() string {
	var sb strings.Builder

	sb.WriteString("[global]\n")
	sb.WriteString(fmt.Sprintf("    workgroup = %s\n", m.config.Workgroup))
	sb.WriteString(fmt.Sprintf("    server string = %s\n", m.config.ServerString))
	sb.WriteString("    security = user\n")
	sb.WriteString("    map to guest = Bad User\n")
	sb.WriteString(fmt.Sprintf("    min protocol = %s\n", m.config.MinProtocol))
	sb.WriteString(fmt.Sprintf("    max protocol = %s\n", m.config.MaxProtocol))
	sb.WriteString("    local master = yes\n")
	sb.WriteString("    dns proxy = no\n")
	sb.WriteString("\n")

	// 生成共享配置
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, share := range m.shares {
		sb.WriteString(fmt.Sprintf("[%s]\n", share.Name))
		sb.WriteString(fmt.Sprintf("    path = %s\n", share.Path))
		sb.WriteString(fmt.Sprintf("    comment = %s\n", share.Comment))
		sb.WriteString(fmt.Sprintf("    browseable = %t\n", share.Browseable))
		sb.WriteString(fmt.Sprintf("    read only = %t\n", share.ReadOnly))
		sb.WriteString(fmt.Sprintf("    guest ok = %t\n", share.GuestAccess))

		if len(share.AllowedUsers) > 0 {
			sb.WriteString(fmt.Sprintf("    valid users = %s\n", strings.Join(share.AllowedUsers, ", ")))
		}

		if len(share.VetoFiles) > 0 {
			sb.WriteString(fmt.Sprintf("    veto files = /%s/\n", strings.Join(share.VetoFiles, "/")))
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// ApplyConfig 应用配置到系统
func (m *Manager) ApplyConfig() error {
	m.mu.RLock()
	configContent := m.generateSmbConf()
	m.mu.RUnlock()

	// 写入 Samba 配置文件
	configPath := "/etc/samba/smb.conf"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("写入配置文件失败：%w", err)
	}

	// 重新加载 Samba 配置
	cmd := exec.Command("smbcontrol", "smbd", "reload-config")
	if err := cmd.Run(); err != nil {
		// 如果 smbcontrol 不可用，尝试重启服务
		cmd = exec.Command("systemctl", "restart", "smbd")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("重启 Samba 服务失败：%w", err)
		}
	}

	return nil
}

// CreateShare 创建共享
func (m *Manager) CreateShare(input ShareInput) (*Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.shares[input.Name]; exists {
		return nil, fmt.Errorf("共享已存在")
	}

	// 确保路径存在
	if err := os.MkdirAll(input.Path, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败：%w", err)
	}

	share := &Share{
		Name:        input.Name,
		Path:        input.Path,
		Comment:     input.Comment,
		ReadOnly:    input.ReadOnly,
		GuestAccess: input.GuestAccess,
		AllowedUsers: input.AllowedUsers,
		Browseable:  input.Browseable,
		VetoFiles:   []string{".DS_Store", "Thumbs.db"}, // 默认隐藏系统文件
	}

	m.shares[input.Name] = share
	return share, nil
}

// GetShare 获取共享
func (m *Manager) GetShare(name string) (*Share, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	share, exists := m.shares[name]
	if !exists {
		return nil, fmt.Errorf("共享不存在")
	}
	return share, nil
}

// ListShares 获取共享列表
func (m *Manager) ListShares() []*Share {
	m.mu.RLock()
	defer m.mu.RUnlock()

	shares := make([]*Share, 0, len(m.shares))
	for _, s := range m.shares {
		shares = append(shares, s)
	}
	return shares
}

// UpdateShare 更新共享
func (m *Manager) UpdateShare(name string, input ShareInput) (*Share, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	share, exists := m.shares[name]
	if !exists {
		return nil, fmt.Errorf("共享不存在")
	}

	share.Comment = input.Comment
	share.ReadOnly = input.ReadOnly
	share.GuestAccess = input.GuestAccess
	share.AllowedUsers = input.AllowedUsers
	share.Browseable = input.Browseable

	return share, nil
}

// DeleteShare 删除共享
func (m *Manager) DeleteShare(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.shares[name]; !exists {
		return fmt.Errorf("共享不存在")
	}

	delete(m.shares, name)
	return nil
}

// SetSharePermission 设置共享权限
func (m *Manager) SetSharePermission(shareName, username string, readWrite bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	share, exists := m.shares[shareName]
	if !exists {
		return fmt.Errorf("共享不存在")
	}

	// 验证用户存在
	if _, err := m.userManager.GetUser(username); err != nil {
		return fmt.Errorf("用户不存在：%w", err)
	}

	// 添加到允许列表
	found := false
	for _, u := range share.AllowedUsers {
		if u == username {
			found = true
			break
		}
	}
	if !found {
		share.AllowedUsers = append(share.AllowedUsers, username)
	}

	// 设置系统文件权限
	if err := os.Chown(share.Path, -1, -1); err != nil {
		return fmt.Errorf("设置权限失败：%w", err)
	}

	mode := os.FileMode(0755)
	if !readWrite {
		mode = 0555 // 只读
	}
	if err := os.Chmod(share.Path, mode); err != nil {
		return fmt.Errorf("设置权限失败：%w", err)
	}

	return nil
}

// RemoveSharePermission 移除共享权限
func (m *Manager) RemoveSharePermission(shareName, username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	share, exists := m.shares[shareName]
	if !exists {
		return fmt.Errorf("共享不存在")
	}

	// 从允许列表移除
	newUsers := make([]string, 0, len(share.AllowedUsers))
	for _, u := range share.AllowedUsers {
		if u != username {
			newUsers = append(newUsers, u)
		}
	}
	share.AllowedUsers = newUsers

	return nil
}

// GetStatus 获取 Samba 服务状态
func (m *Manager) GetStatus() (running bool, err error) {
	cmd := exec.Command("systemctl", "is-active", "smbd")
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(output)) == "active", nil
}

// Start 启动 Samba 服务
func (m *Manager) Start() error {
	cmd := exec.Command("systemctl", "start", "smbd")
	return cmd.Run()
}

// Stop 停止 Samba 服务
func (m *Manager) Stop() error {
	cmd := exec.Command("systemctl", "stop", "smbd")
	return cmd.Run()
}

// Restart 重启 Samba 服务
func (m *Manager) Restart() error {
	cmd := exec.Command("systemctl", "restart", "smbd")
	return cmd.Run()
}

// TestConfig 测试配置文件语法
func (m *Manager) TestConfig() (bool, string, error) {
	cmd := exec.Command("testparm", "-s")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, string(output), err
	}
	return true, string(output), nil
}

// GetSharePath 获取共享路径（用于权限检查）
func (m *Manager) GetSharePath(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if share, exists := m.shares[name]; exists {
		return share.Path
	}
	return ""
}

// GetUserShares 获取用户有权访问的共享列表
func (m *Manager) GetUserShares(username string) []*Share {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Share
	for _, share := range m.shares {
		// 管理员可以访问所有共享
		if user, err := m.userManager.GetUser(username); err == nil && user.Role == users.RoleAdmin {
			result = append(result, share)
			continue
		}

		// 检查是否在允许列表中
		if len(share.AllowedUsers) == 0 || share.GuestAccess {
			// 公开共享或无限制
			result = append(result, share)
		} else {
			for _, allowed := range share.AllowedUsers {
				if allowed == username {
					result = append(result, share)
					break
				}
			}
		}
	}
	return result
}
