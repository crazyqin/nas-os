package smb

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"nas-os/internal/users"
)

// ServiceStatus SMB 服务状态
type ServiceStatus struct {
	Running        bool      `json:"running"`
	Status         string    `json:"status"`
	PID            int       `json:"pid,omitempty"`
	Uptime         string    `json:"uptime,omitempty"`
	ConnectedUsers int       `json:"connected_users"`
	OpenFiles      int       `json:"open_files"`
	LastStarted    time.Time `json:"last_started,omitempty"`
	ErrorMessage   string    `json:"error_message,omitempty"`
}

// Connection SMB 连接信息
type Connection struct {
	ID          int       `json:"id"`
	Username    string    `json:"username"`
	ShareName   string    `json:"share_name"`
	ClientIP    string    `json:"client_ip"`
	ClientName  string    `json:"client_name,omitempty"`
	ConnectedAt time.Time `json:"connected_at"`
	Protocol    string    `json:"protocol,omitempty"`
	Encryption  string    `json:"encryption,omitempty"`
	LockedFiles []string  `json:"locked_files,omitempty"`
}

// persistentConfig 持久化配置结构
type persistentConfig struct {
	Config *Config           `json:"config"`
	Shares map[string]*Share `json:"shares"`
}

// Share SMB 共享配置
type Share struct {
	Name               string   `json:"name"`
	Path               string   `json:"path"`
	Comment            string   `json:"comment"`
	Browseable         bool     `json:"browseable"`
	ReadOnly           bool     `json:"read_only"`
	GuestOK            bool     `json:"guest_ok"`
	Users              []string `json:"users,omitempty"`
	ValidUsers         []string `json:"valid_users,omitempty"`
	WriteList          []string `json:"write_list,omitempty"`
	CreateMask         string   `json:"create_mask,omitempty"`
	DirectoryMask      string   `json:"directory_mask,omitempty"`
	GuestAccess        bool     `json:"guest_access"`
	AllowedUsers       []string `json:"allowed_users,omitempty"`
	VetoFiles          []string `json:"veto_files,omitempty"`
	HideFiles          []string `json:"hide_files,omitempty"`
	ForceUser          string   `json:"force_user,omitempty"`
	ForceGroup         string   `json:"force_group,omitempty"`
	StoreDOSAttributes bool     `json:"store_dos_attributes"`
	EASupport          bool     `json:"ea_support"`
	ACLGroupControl    bool     `json:"acl_group_control"`
	InheritACLs        bool     `json:"inherit_acls"`
	InheritOwner       string   `json:"inherit_owner,omitempty"`
	InheritPermissions bool     `json:"inherit_permissions"`
	FollowSymlinks     bool     `json:"follow_symlinks"`
	WideLinks          bool     `json:"wide_links"`
	Available          bool     `json:"available"`
	Printable          bool     `json:"printable"`
}

// ShareInput 创建/更新共享输入
type ShareInput struct {
	Name          string   `json:"name" binding:"required"`
	Path          string   `json:"path" binding:"required"`
	Comment       string   `json:"comment"`
	Browseable    bool     `json:"browseable"`
	ReadOnly      bool     `json:"read_only"`
	GuestOK       bool     `json:"guest_ok"`
	Users         []string `json:"users"`
	ValidUsers    []string `json:"valid_users"`
	WriteList     []string `json:"write_list"`
	CreateMask    string   `json:"create_mask"`
	DirectoryMask string   `json:"directory_mask"`
	GuestAccess   bool     `json:"guest_access"`
	AllowedUsers  []string `json:"allowed_users"`
	VetoFiles     []string `json:"veto_files"`
}

// Config SMB 配置
type Config struct {
	Enabled              bool     `json:"enabled"`
	Workgroup            string   `json:"workgroup"`
	ServerString         string   `json:"server_string"`
	GuestAccess          bool     `json:"guest_access"`
	MinProtocol          string   `json:"min_protocol"`
	MaxProtocol          string   `json:"max_protocol"`
	ServerRole           string   `json:"server_role,omitempty"`
	Security             string   `json:"security,omitempty"`
	LogLevel             string   `json:"log_level,omitempty"`
	MaxLogSize           string   `json:"max_log_size,omitempty"`
	LoadPrinters         bool     `json:"load_printers"`
	Printing             string   `json:"printing,omitempty"`
	PrintcapName         string   `json:"printcap_name,omitempty"`
	GuestAccount         string   `json:"guest_account,omitempty"`
	MapToGuest           string   `json:"map_to_guest,omitempty"`
	UsershareAllowGuests bool     `json:"usershare_allow_guests"`
	UsershareMaxShares   string   `json:"usershare_max_shares,omitempty"`
	UsershareOwnerOnly   bool     `json:"usershare_owner_only"`
	SocketOptions        string   `json:"socket_options,omitempty"`
	DNSProxy             bool     `json:"dns_proxy"`
	BindInterfacesOnly   bool     `json:"bind_interfaces_only"`
	Interfaces           []string `json:"interfaces,omitempty"`
	HostMSDFS            bool     `json:"host_msdfs"`
	PassdbBackend        string   `json:"passdb_backend,omitempty"`
	ObeyPAMRestrictions  bool     `json:"obey_pam_restrictions"`
	UnixPasswordSync     bool     `json:"unix_password_sync"`
	PasswdProgram        string   `json:"passwd_program,omitempty"`
	PasswdChat           string   `json:"passwd_chat,omitempty"`
	PAMPasswordChange    bool     `json:"pam_password_change"`
	UsernameMap          string   `json:"username_map,omitempty"`
	LogFile              string   `json:"log_file,omitempty"`
	ServerMinProtocol    string   `json:"server_min_protocol,omitempty"`
	ClientMinProtocol    string   `json:"client_min_protocol,omitempty"`
	ClientMaxProtocol    string   `json:"client_max_protocol,omitempty"`

	// 性能优化配置（参考 TrueNAS）
	EnableAIO            bool   `json:"enable_aio"`             // 异步 I/O
	AIOReadSize          int    `json:"aio_read_size"`          // 异步读取块大小 (KB)
	AIOWriteSize         int    `json:"aio_write_size"`         // 异步写入块大小 (KB)
	WriteCacheSize       int    `json:"write_cache_size"`       // 写缓存大小 (KB)
	MaxXmit              int    `json:"max_xmit"`               // 最大传输大小 (KB)
	Deadtime             int    `json:"deadtime"`               // 空闲连接超时 (分钟)
	Keepalive            int    `json:"keepalive"`              // 保活间隔 (秒)
	MaxOpenFiles         int    `json:"max_open_files"`         // 最大打开文件数
	UseSendfile          bool   `json:"use_sendfile"`           // 使用 sendfile 优化
	StrictAllocate       bool   `json:"strict_allocate"`        // 严格空间分配
	LargeReadwrite       bool   `json:"large_readwrite"`        // 大文件读写优化
	MinReceivefileSize   int    `json:"min_receivefile_size"`   // 最小接收文件大小 (KB)
	MaxStatCacheSize     int    `json:"max_stat_cache_size"`    // 状态缓存大小
	GetwdCache           bool   `json:"getwd_cache"`            // 工作目录缓存
	KernelOplocks        bool   `json:"kernel_oplocks"`         // 内核 oplock 支持
}

// Manager SMB 管理器
type Manager struct {
	mu          sync.RWMutex
	shares      map[string]*Share
	config      *Config
	userManager *users.Manager
	configPath  string
}

// newDefaultConfig 创建默认配置的副本
func newDefaultConfig() *Config {
	return &Config{
		Enabled:              true,
		Workgroup:            "WORKGROUP",
		ServerString:         "NAS-OS Samba Server",
		GuestAccess:          false,
		MinProtocol:          "SMB2",
		MaxProtocol:          "SMB3",
		Security:             "user",
		MapToGuest:           "Bad User",
		LoadPrinters:         false,
		DNSProxy:             false,
		UsershareAllowGuests: false,
		UsershareOwnerOnly:   true,
		HostMSDFS:            false,
		ObeyPAMRestrictions:  false,
		UnixPasswordSync:     false,
		PAMPasswordChange:    false,
		PassdbBackend:        "tdbsam",
	}
}

// NewManager 创建 SMB 管理器
func NewManager(configPath string) (*Manager, error) {
	m := &Manager{
		shares:     make(map[string]*Share),
		config:     newDefaultConfig(),
		configPath: configPath,
	}

	// 加载现有配置（如果有）
	if err := m.loadConfig(); err != nil {
		return nil, err
	}

	logInfo("SMB管理器已初始化")
	return m, nil
}

// NewManagerWithUserMgr 创建 SMB 管理器（带用户管理器）
func NewManagerWithUserMgr(userMgr *users.Manager, configPath string) (*Manager, error) {
	m := &Manager{
		shares:      make(map[string]*Share),
		config:      newDefaultConfig(),
		userManager: userMgr,
		configPath:  configPath,
	}

	// 加载现有配置（如果有）
	if err := m.loadConfig(); err != nil {
		return nil, err
	}

	logInfo("SMB管理器已初始化（带用户管理器）")
	return m, nil
}

// loadConfig 从文件加载配置
func (m *Manager) loadConfig() error {
	// 检查配置文件是否存在
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		// 配置文件不存在，使用默认配置
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败：%w", err)
	}

	var pc persistentConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return fmt.Errorf("解析配置文件失败：%w", err)
	}

	if pc.Config != nil {
		m.config = pc.Config
	}
	if pc.Shares != nil {
		m.shares = pc.Shares
	}

	logInfo("SMB配置已加载", "shares", len(m.shares))
	return nil
}

// saveConfig 保存配置到文件（线程安全）
func (m *Manager) saveConfig() error {
	m.mu.RLock()
	pc := persistentConfig{
		Config: m.config,
		Shares: m.shares,
	}
	m.mu.RUnlock()

	return writeConfigFile(m.configPath, pc)
}

// saveConfigLocked 保存配置（调用者已持有锁）
func (m *Manager) saveConfigLocked() error {
	pc := persistentConfig{
		Config: m.config,
		Shares: m.shares,
	}
	return writeConfigFile(m.configPath, pc)
}

// writeConfigFile 写入配置文件
func writeConfigFile(configPath string, pc persistentConfig) error {
	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败：%w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败：%w", err)
	}

	if err := os.WriteFile(configPath, data, 0640); err != nil {
		return fmt.Errorf("写入配置文件失败：%w", err)
	}

	return nil
}

// generateSmbConf 生成 Samba 配置文件内容
func (m *Manager) generateSmbConf() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return GenerateSmbConf(m.config, m.shares)
}

// ApplyConfig 应用配置到系统
func (m *Manager) ApplyConfig() error {
	configContent := m.generateSmbConf()

	// 备份现有配置（忽略错误，不影响主流程）
	_ = BackupSmbConf("/etc/samba/smb.conf")

	// 写入 Samba 配置文件
	configPath := "/etc/samba/smb.conf"
	if err := os.WriteFile(configPath, []byte(configContent), 0640); err != nil {
		logError("写入配置文件失败", err)
		return fmt.Errorf("写入配置文件失败：%w", err)
	}

	logInfo("SMB配置已写入", "path", configPath)

	// 重新加载 Samba 配置
	cmd := exec.Command("smbcontrol", "smbd", "reload-config")
	if err := cmd.Run(); err != nil {
		// 如果 smbcontrol 不可用，尝试重启服务
		cmd = exec.Command("systemctl", "restart", "smbd")
		if err := cmd.Run(); err != nil {
			logError("重启 Samba 服务失败", err)
			return fmt.Errorf("重启 Samba 服务失败：%w", err)
		}
	}

	logInfo("SMB配置已应用")
	return nil
}

// CreateShare 创建共享
func (m *Manager) CreateShare(share *Share) error {
	if share == nil {
		return fmt.Errorf("共享配置不能为空")
	}

	// 验证配置
	if err := ValidateShareConfig(share); err != nil {
		return fmt.Errorf("验证共享配置失败：%w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.shares[share.Name]; exists {
		return fmt.Errorf("共享已存在：%s", share.Name)
	}

	// 确保路径存在
	if err := os.MkdirAll(share.Path, 0750); err != nil {
		logError("创建目录失败", err, "path", share.Path)
		return fmt.Errorf("创建目录失败：%w", err)
	}

	// 设置默认值
	if share.CreateMask == "" {
		share.CreateMask = "0644"
	}
	if share.DirectoryMask == "" {
		share.DirectoryMask = "0755"
	}
	if len(share.VetoFiles) == 0 {
		share.VetoFiles = []string{".DS_Store", "Thumbs.db"}
	}
	share.Available = true
	share.FollowSymlinks = true

	m.shares[share.Name] = share

	// 保存配置
	if err := m.saveConfigLocked(); err != nil {
		delete(m.shares, share.Name)
		return fmt.Errorf("保存配置失败：%w", err)
	}

	logInfo("SMB共享已创建", "name", share.Name, "path", share.Path)
	return nil
}

// UpdateShare 更新共享
func (m *Manager) UpdateShare(name string, share *Share) error {
	if share == nil {
		return fmt.Errorf("共享配置不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.shares[name]
	if !exists {
		return fmt.Errorf("共享不存在：%s", name)
	}

	// 更新字段
	if share.Path != "" {
		existing.Path = share.Path
	}
	if share.Comment != "" {
		existing.Comment = share.Comment
	}
	existing.Browseable = share.Browseable
	existing.ReadOnly = share.ReadOnly
	existing.GuestOK = share.GuestOK
	existing.GuestAccess = share.GuestAccess
	if share.Users != nil {
		existing.Users = share.Users
	}
	if share.ValidUsers != nil {
		existing.ValidUsers = share.ValidUsers
	}
	if share.WriteList != nil {
		existing.WriteList = share.WriteList
	}
	if share.CreateMask != "" {
		existing.CreateMask = share.CreateMask
	}
	if share.DirectoryMask != "" {
		existing.DirectoryMask = share.DirectoryMask
	}
	if share.VetoFiles != nil {
		existing.VetoFiles = share.VetoFiles
	}

	// 保存配置
	if err := m.saveConfigLocked(); err != nil {
		return fmt.Errorf("保存配置失败：%w", err)
	}

	logInfo("SMB共享已更新", "name", name)
	return nil
}

// DeleteShare 删除共享
func (m *Manager) DeleteShare(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.shares[name]; !exists {
		return fmt.Errorf("共享不存在：%s", name)
	}

	delete(m.shares, name)

	// 保存配置
	if err := m.saveConfigLocked(); err != nil {
		return fmt.Errorf("保存配置失败：%w", err)
	}

	logInfo("SMB共享已删除", "name", name)
	return nil
}

// ListShares 列出所有 SMB 共享
func (m *Manager) ListShares() ([]*Share, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	shares := make([]*Share, 0, len(m.shares))
	for _, s := range m.shares {
		shares = append(shares, s)
	}
	return shares, nil
}

// GetShare 获取指定 SMB 共享
func (m *Manager) GetShare(name string) (*Share, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	share, exists := m.shares[name]
	if !exists {
		return nil, fmt.Errorf("共享不存在：%s", name)
	}
	return share, nil
}

// Reload 重新加载 SMB 配置
func (m *Manager) Reload() error {
	// 重新加载持久化配置
	if err := m.loadConfig(); err != nil {
		return fmt.Errorf("重新加载配置失败：%w", err)
	}

	// 应用到系统
	if err := m.ApplyConfig(); err != nil {
		return fmt.Errorf("应用配置失败：%w", err)
	}

	logInfo("SMB配置已重新加载")
	return nil
}

// Status 获取 SMB 服务状态
func (m *Manager) Status() (*ServiceStatus, error) {
	status := &ServiceStatus{
		Status:      "stopped",
		Running:     false,
		LastStarted: time.Now(),
	}

	// 检查 smbd 服务状态
	cmd := exec.Command("systemctl", "is-active", "smbd")
	output, err := cmd.Output()
	if err != nil {
		status.Status = "stopped"
		status.ErrorMessage = err.Error()
		return status, nil
	}

	statusStr := strings.TrimSpace(string(output))
	if statusStr == "active" {
		status.Running = true
		status.Status = "running"
	}

	// 获取 PID
	pidCmd := exec.Command("systemctl", "show", "--property=MainPID", "smbd")
	pidOutput, err := pidCmd.Output()
	if err == nil {
		pidStr := strings.TrimPrefix(strings.TrimSpace(string(pidOutput)), "MainPID=")
		if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
			status.PID = pid
		}
	}

	// 获取连接数和打开文件数
	connections, err := m.Connections()
	if err == nil {
		status.ConnectedUsers = len(connections)
		status.OpenFiles = m.countOpenFiles(connections)
	}

	return status, nil
}

// Connections 获取当前 SMB 连接
func (m *Manager) Connections() ([]*Connection, error) {
	var connections []*Connection

	// 使用 smbstatus 获取连接信息
	cmd := exec.Command("smbstatus", "-b")
	output, err := cmd.Output()
	if err != nil {
		logError("获取连接信息失败", err)
		return nil, fmt.Errorf("获取连接信息失败：%w", err)
	}

	// 解析 smbstatus 输出
	lines := strings.Split(string(output), "\n")
	for i, line := range lines {
		if i < 3 { // 跳过头部
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 5 {
			conn := &Connection{
				ID: i - 2,
			}

			// 解析 PID
			if pid, err := strconv.Atoi(fields[0]); err == nil {
				conn.ID = pid
			}

			// 解析其他字段
			if len(fields) >= 2 {
				conn.Username = fields[1]
			}
			if len(fields) >= 3 {
				conn.ClientIP = fields[2]
			}
			if len(fields) >= 4 {
				conn.Protocol = fields[3]
			}
			if len(fields) >= 5 {
				conn.ShareName = fields[4]
			}
			if len(fields) >= 6 {
				conn.ClientName = fields[5]
			}

			conn.ConnectedAt = time.Now() // smbstatus 不提供精确时间
			connections = append(connections, conn)
		}
	}

	return connections, nil
}

// countOpenFiles 统计打开文件数
func (m *Manager) countOpenFiles(connections []*Connection) int {
	// 使用 smbstatus -L 获取锁定文件
	cmd := exec.Command("smbstatus", "-L")
	output, err := cmd.Output()
	if err != nil {
		return 0
	}

	lines := strings.Split(string(output), "\n")
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" && !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "Locked") {
			count++
		}
	}

	return count
}

// GetStatus 获取 Samba 服务状态（兼容旧接口）
func (m *Manager) GetStatus() (bool, error) {
	status, err := m.Status()
	if err != nil {
		return false, err
	}
	return status.Running, nil
}

// Start 启动 Samba 服务
func (m *Manager) Start() error {
	cmd := exec.Command("systemctl", "start", "smbd")
	if err := cmd.Run(); err != nil {
		logError("启动 Samba 服务失败", err)
		return fmt.Errorf("启动 Samba 服务失败：%w", err)
	}
	logInfo("Samba 服务已启动")
	return nil
}

// Stop 停止 Samba 服务
func (m *Manager) Stop() error {
	cmd := exec.Command("systemctl", "stop", "smbd")
	if err := cmd.Run(); err != nil {
		logError("停止 Samba 服务失败", err)
		return fmt.Errorf("停止 Samba 服务失败：%w", err)
	}
	logInfo("Samba 服务已停止")
	return nil
}

// Restart 重启 Samba 服务
func (m *Manager) Restart() error {
	cmd := exec.Command("systemctl", "restart", "smbd")
	if err := cmd.Run(); err != nil {
		logError("重启 Samba 服务失败", err)
		return fmt.Errorf("重启 Samba 服务失败：%w", err)
	}
	logInfo("Samba 服务已重启")
	return nil
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
		if m.userManager == nil {
			// 无用户管理器，返回所有共享
			result = append(result, share)
			continue
		}

		// 管理员可以访问所有共享
		if user, err := m.userManager.GetUser(username); err == nil && user.Role == users.RoleAdmin {
			result = append(result, share)
			continue
		}

		// 检查是否在允许列表中
		if len(share.ValidUsers) == 0 || share.GuestOK || share.GuestAccess {
			// 公开共享或无限制
			result = append(result, share)
		} else {
			for _, allowed := range share.ValidUsers {
				if allowed == username {
					result = append(result, share)
					break
				}
			}
		}
	}
	return result
}

// SetSharePermission 设置共享权限
func (m *Manager) SetSharePermission(shareName, username string, readWrite bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	share, exists := m.shares[shareName]
	if !exists {
		return fmt.Errorf("共享不存在：%s", shareName)
	}

	// 验证用户存在
	if m.userManager != nil {
		if _, err := m.userManager.GetUser(username); err != nil {
			return fmt.Errorf("用户不存在：%w", err)
		}
	}

	// 添加到允许列表
	found := false
	for _, u := range share.ValidUsers {
		if u == username {
			found = true
			break
		}
	}
	if !found {
		share.ValidUsers = append(share.ValidUsers, username)
	}

	// 如果是读写权限，添加到写列表
	if readWrite {
		foundWrite := false
		for _, u := range share.WriteList {
			if u == username {
				foundWrite = true
				break
			}
		}
		if !foundWrite {
			share.WriteList = append(share.WriteList, username)
		}
	}

	// 注意：目录权限保持默认（0755），通过 SMB 配置控制读写权限
	// 不应通过文件系统权限限制目录访问，否则会影响目录的正常使用

	logInfo("共享权限已设置", "share", shareName, "user", username, "readWrite", readWrite)
	return nil
}

// RemoveSharePermission 移除共享权限
func (m *Manager) RemoveSharePermission(shareName, username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	share, exists := m.shares[shareName]
	if !exists {
		return fmt.Errorf("共享不存在：%s", shareName)
	}

	// 从允许列表移除
	newUsers := make([]string, 0, len(share.ValidUsers))
	for _, u := range share.ValidUsers {
		if u != username {
			newUsers = append(newUsers, u)
		}
	}
	share.ValidUsers = newUsers

	// 从写列表移除
	newWriteList := make([]string, 0, len(share.WriteList))
	for _, u := range share.WriteList {
		if u != username {
			newWriteList = append(newWriteList, u)
		}
	}
	share.WriteList = newWriteList

	logInfo("共享权限已移除", "share", shareName, "user", username)
	return nil
}

// CloseShare 关闭共享（禁用但保留配置）
func (m *Manager) CloseShare(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	share, exists := m.shares[name]
	if !exists {
		return fmt.Errorf("共享不存在：%s", name)
	}

	share.Available = false

	if err := m.saveConfigLocked(); err != nil {
		return fmt.Errorf("保存配置失败：%w", err)
	}

	logInfo("SMB共享已关闭", "name", name)
	return nil
}

// OpenShare 打开共享
func (m *Manager) OpenShare(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	share, exists := m.shares[name]
	if !exists {
		return fmt.Errorf("共享不存在：%s", name)
	}

	share.Available = true

	if err := m.saveConfigLocked(); err != nil {
		return fmt.Errorf("保存配置失败：%w", err)
	}

	logInfo("SMB共享已打开", "name", name)
	return nil
}

// GetConfig 获取当前配置
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新全局配置
func (m *Manager) UpdateConfig(config *Config) error {
	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("验证配置失败：%w", err)
	}

	m.mu.Lock()
	m.config = config
	m.mu.Unlock()

	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("保存配置失败：%w", err)
	}

	logInfo("SMB全局配置已更新")
	return nil
}

// CreateShareFromInput 从输入创建共享（兼容旧接口）
func (m *Manager) CreateShareFromInput(input ShareInput) (*Share, error) {
	share := &Share{
		Name:          input.Name,
		Path:          input.Path,
		Comment:       input.Comment,
		Browseable:    input.Browseable,
		ReadOnly:      input.ReadOnly,
		GuestOK:       input.GuestOK,
		GuestAccess:   input.GuestAccess,
		Users:         input.Users,
		ValidUsers:    input.ValidUsers,
		WriteList:     input.WriteList,
		CreateMask:    input.CreateMask,
		DirectoryMask: input.DirectoryMask,
		VetoFiles:     input.VetoFiles,
	}

	if err := m.CreateShare(share); err != nil {
		return nil, err
	}

	return share, nil
}

// UpdateShareFromInput 从输入更新共享（兼容旧接口）
func (m *Manager) UpdateShareFromInput(name string, input ShareInput) (*Share, error) {
	share := &Share{
		Path:          input.Path,
		Comment:       input.Comment,
		Browseable:    input.Browseable,
		ReadOnly:      input.ReadOnly,
		GuestOK:       input.GuestOK,
		GuestAccess:   input.GuestAccess,
		Users:         input.Users,
		ValidUsers:    input.ValidUsers,
		WriteList:     input.WriteList,
		CreateMask:    input.CreateMask,
		DirectoryMask: input.DirectoryMask,
		VetoFiles:     input.VetoFiles,
	}

	if err := m.UpdateShare(name, share); err != nil {
		return nil, err
	}

	return m.GetShare(name)
}
