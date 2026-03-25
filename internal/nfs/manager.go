// Package nfs 提供NFS共享服务管理功能
package nfs

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"nas-os/internal/logging"
)

// ServiceStatus NFS服务状态.
type ServiceStatus struct {
	Running     bool   `json:"running"`
	Status      string `json:"status"`
	Version     string `json:"version,omitempty"`
	Connections int    `json:"connections,omitempty"`
	Exports     int    `json:"exports"`
}

// Manager NFS服务管理器.
type Manager struct {
	configPath string
	exports    map[string]*Export
	config     *Config
	mu         sync.RWMutex
	logger     *logging.Logger
}

// Export NFS导出配置.
type Export struct {
	Path    string        `json:"path"`
	Clients []Client      `json:"clients"`
	Options ExportOptions `json:"options"`
	FSID    int           `json:"fsid"`
	Comment string        `json:"comment,omitempty"`
}

// Client NFS客户端配置.
type Client struct {
	Host    string   `json:"host"`
	Options []string `json:"options,omitempty"`
}

// ExportOptions NFS导出选项.
type ExportOptions struct {
	Ro           bool `json:"ro"`
	Rw           bool `json:"rw"`
	NoRootSquash bool `json:"no_root_squash"`
	Async        bool `json:"async"`
	Secure       bool `json:"secure"`
	SubtreeCheck bool `json:"subtree_check"`
}

// Config NFS 全局配置.
type Config struct {
	Enabled        bool     `json:"enabled"`
	Version        string   `json:"version"`         // NFS版本: "4", "3", "4.2"
	Threads        int      `json:"threads"`         // 工作线程数
	UDPPort        int      `json:"udp_port"`        // UDP端口
	TCPPort        int      `json:"tcp_port"`        // TCP端口
	MountdPort     int      `json:"mountd_port"`     // mountd端口
	StatdPort      int      `json:"statd_port"`      // statd端口
	LockdPort      int      `json:"lockd_port"`      // lockd端口
	AllowedHosts   []string `json:"allowed_hosts"`   // 允许的主机
	BlockedHosts   []string `json:"blocked_hosts"`   // 禁止的主机
	MaxConnections int      `json:"max_connections"` // 最大连接数
}

// persistentConfig 持久化配置结构.
type persistentConfig struct {
	Config  *Config            `json:"config"`
	Exports map[string]*Export `json:"exports"`
}

// newDefaultConfig 创建默认配置.
func newDefaultConfig() *Config {
	return &Config{
		Enabled:        true,
		Version:        "4",
		Threads:        8,
		TCPPort:        2049,
		UDPPort:        2049,
		MountdPort:     0, // 自动分配
		StatdPort:      0,
		LockdPort:      0,
		AllowedHosts:   []string{},
		BlockedHosts:   []string{},
		MaxConnections: 0, // 无限制
	}
}

// NewManager 创建NFS管理器.
func NewManager(configPath string) (*Manager, error) {
	logger := logging.NewLogger(nil).WithSource("nfs")

	m := &Manager{
		configPath: configPath,
		exports:    make(map[string]*Export),
		config:     newDefaultConfig(),
		logger:     logger,
	}

	// 加载现有配置
	if err := m.loadConfig(); err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	logger.Info("NFS管理器初始化完成")
	return m, nil
}

// loadConfig 从文件加载配置.
func (m *Manager) loadConfig() error {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		m.logger.Debug("配置文件不存在，使用空配置")
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var pc persistentConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	if pc.Exports != nil {
		m.exports = pc.Exports
	}
	if pc.Config != nil {
		m.config = pc.Config
	}

	m.logger.Infof("加载了 %d 个导出配置", len(m.exports))
	return nil
}

// saveConfig 保存配置到文件.
func (m *Manager) saveConfig() error {
	m.mu.RLock()
	pc := persistentConfig{
		Config:  m.config,
		Exports: m.exports,
	}
	m.mu.RUnlock()

	return m.writeConfigFile(pc)
}

// saveConfigLocked 保存配置（调用者已持有锁）.
func (m *Manager) saveConfigLocked() error {
	pc := persistentConfig{
		Config:  m.config,
		Exports: m.exports,
	}
	return m.writeConfigFile(pc)
}

// writeConfigFile 写入配置文件.
func (m *Manager) writeConfigFile(pc persistentConfig) error {
	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	if err := os.WriteFile(m.configPath, data, 0640); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	m.logger.Debug("配置文件已保存")
	return nil
}

// CreateExport 创建NFS导出.
func (m *Manager) CreateExport(export *Export) error {
	if export == nil {
		return fmt.Errorf("导出配置不能为空")
	}

	if export.Path == "" {
		return fmt.Errorf("导出路径不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	if _, exists := m.exports[export.Path]; exists {
		m.logger.Warnf("导出已存在: %s", export.Path)
		return fmt.Errorf("导出已存在: %s", export.Path)
	}

	// 确保路径存在
	if err := os.MkdirAll(export.Path, 0750); err != nil {
		m.logger.Errorf("创建导出目录失败: %s - %v", export.Path, err)
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 设置默认选项
	m.applyDefaultOptions(export)

	// 添加到内存
	m.exports[export.Path] = export

	// 保存配置
	if err := m.saveConfigLocked(); err != nil {
		delete(m.exports, export.Path)
		return fmt.Errorf("保存配置失败: %w", err)
	}

	m.logger.Infof("创建NFS导出: %s", export.Path)
	return nil
}

// applyDefaultOptions 应用默认选项.
func (m *Manager) applyDefaultOptions(export *Export) {
	// 如果没有客户端，添加默认允许所有
	if len(export.Clients) == 0 {
		export.Clients = []Client{
			{Host: "*", Options: []string{}},
		}
	}

	// 设置默认权限（如果没有指定读写权限）
	if !export.Options.Ro && !export.Options.Rw {
		export.Options.Rw = true
	}
}

// UpdateExport 更新NFS导出.
func (m *Manager) UpdateExport(path string, export *Export) error {
	if export == nil {
		return fmt.Errorf("导出配置不能为空")
	}

	if path == "" {
		return fmt.Errorf("导出路径不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否存在
	if _, exists := m.exports[path]; !exists {
		m.logger.Warnf("导出不存在: %s", path)
		return fmt.Errorf("导出不存在: %s", path)
	}

	// 如果路径变了，需要删除旧的
	if export.Path != "" && export.Path != path {
		delete(m.exports, path)
		path = export.Path
	}

	// 确保路径存在
	targetPath := export.Path
	if targetPath == "" {
		targetPath = path
	}
	if err := os.MkdirAll(targetPath, 0750); err != nil {
		m.logger.Errorf("创建导出目录失败: %s - %v", targetPath, err)
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 应用默认选项
	m.applyDefaultOptions(export)

	// 更新
	m.exports[path] = export

	// 保存配置
	if err := m.saveConfigLocked(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	m.logger.Infof("更新NFS导出: %s", path)
	return nil
}

// DeleteExport 删除NFS导出.
func (m *Manager) DeleteExport(path string) error {
	if path == "" {
		return fmt.Errorf("导出路径不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.exports[path]; !exists {
		m.logger.Warnf("导出不存在: %s", path)
		return fmt.Errorf("导出不存在: %s", path)
	}

	delete(m.exports, path)

	// 保存配置
	if err := m.saveConfigLocked(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	m.logger.Infof("删除NFS导出: %s", path)
	return nil
}

// ListExports 列出所有NFS导出.
func (m *Manager) ListExports() ([]*Export, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exports := make([]*Export, 0, len(m.exports))
	for _, e := range m.exports {
		exports = append(exports, e)
	}

	m.logger.Debugf("列出 %d 个导出", len(exports))
	return exports, nil
}

// GetExport 获取指定NFS导出.
func (m *Manager) GetExport(path string) (*Export, error) {
	if path == "" {
		return nil, fmt.Errorf("导出路径不能为空")
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	export, exists := m.exports[path]
	if !exists {
		return nil, fmt.Errorf("导出不存在: %s", path)
	}

	return export, nil
}

// Reload 重新加载NFS配置.
func (m *Manager) Reload() error {
	m.logger.Info("重新加载NFS配置")

	// 生成exports文件内容
	content, err := m.GenerateExportsFile()
	if err != nil {
		return fmt.Errorf("生成配置失败: %w", err)
	}

	// 写入/etc/exports
	if err := os.WriteFile("/etc/exports", []byte(content), 0640); err != nil {
		m.logger.Errorf("写入exports文件失败: %v", err)
		return fmt.Errorf("写入exports文件失败: %w", err)
	}

	// 重新导出 NFS 配置
	cmd := exec.CommandContext(context.Background(), "exportfs", "-ra") //nolint:misspell
	if output, err := cmd.CombinedOutput(); err != nil {
		m.logger.Errorf("执行 exportfs 失败: %s - %v", string(output), err) //nolint:misspell
		return fmt.Errorf("重新导出失败: %w - %s", err, string(output))
	}

	m.logger.Info("NFS配置重新加载成功")
	return nil
}

// Status 获取NFS服务状态.
func (m *Manager) Status() (*ServiceStatus, error) {
	m.mu.RLock()
	exportsCount := len(m.exports)
	m.mu.RUnlock()

	status := &ServiceStatus{
		Exports: exportsCount,
	}

	// 检查服务运行状态
	cmd := exec.CommandContext(context.Background(), "systemctl", "is-active", "nfs-kernel-server")
	output, err := cmd.Output()
	if err != nil {
		status.Running = false
		status.Status = "stopped"
		return status, nil
	}

	statusStr := strings.TrimSpace(string(output))
	status.Running = statusStr == "active"
	status.Status = statusStr

	// 获取版本信息
	cmd = exec.CommandContext(context.Background(), "nfsstat", "-v")
	if output, err := cmd.Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) > 0 {
			status.Version = strings.TrimSpace(lines[0])
		}
	}

	// 获取连接数 - 通过解析 /proc/fs/nfsd/clients/ 目录
	status.Connections = m.getConnectionsCount()

	m.logger.Debugf("NFS服务状态: running=%v, exports=%d", status.Running, status.Exports)
	return status, nil
}

// getConnectionsCount 获取NFS连接数
// 通过解析 /proc/fs/nfsd/clients/ 目录统计活跃连接
// 这是最可靠的方法，因为数据直接来自内核.
func (m *Manager) getConnectionsCount() int {
	clientsDir := "/proc/fs/nfsd/clients"

	entries, err := os.ReadDir(clientsDir)
	if err != nil {
		m.logger.Debugf("无法读取NFS客户端目录 %s: %v", clientsDir, err)
		return 0
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			// 每个子目录代表一个客户端连接
			count++
		}
	}

	return count
}

// GenerateExportsFile 生成/etc/exports文件内容.
func (m *Manager) GenerateExportsFile() (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var sb strings.Builder

	for path, export := range m.exports {
		if len(export.Clients) == 0 {
			// 默认允许所有客户端
			fmt.Fprintf(&sb, "%s *(%s)\n", path, m.optionsToString(&export.Options))
		} else {
			// 为每个客户端生成配置
			for _, client := range export.Clients {
				opts := m.optionsToString(&export.Options)
				if len(client.Options) > 0 {
					opts = opts + "," + strings.Join(client.Options, ",")
				}
				fmt.Fprintf(&sb, "%s %s(%s)\n", path, client.Host, opts)
			}
		}
	}

	return sb.String(), nil
}

// optionsToString 将导出选项转换为字符串.
func (m *Manager) optionsToString(opts *ExportOptions) string {
	var parts []string

	if opts.Ro {
		parts = append(parts, "ro")
	} else if opts.Rw || (!opts.Ro && !opts.Rw) {
		parts = append(parts, "rw")
	}

	if opts.NoRootSquash {
		parts = append(parts, "no_root_squash")
	} else {
		parts = append(parts, "root_squash")
	}

	if opts.Async {
		parts = append(parts, "async")
	} else {
		parts = append(parts, "sync")
	}

	if opts.Secure {
		parts = append(parts, "secure")
	}

	if opts.SubtreeCheck {
		parts = append(parts, "subtree_check")
	} else {
		parts = append(parts, "no_subtree_check")
	}

	return strings.Join(parts, ",")
}

// Start 启动NFS服务.
func (m *Manager) Start() error {
	m.logger.Info("启动NFS服务")

	cmd := exec.CommandContext(context.Background(), "systemctl", "start", "nfs-kernel-server")
	if output, err := cmd.CombinedOutput(); err != nil {
		m.logger.Errorf("启动NFS服务失败: %s - %v", string(output), err)
		return fmt.Errorf("启动失败: %w - %s", err, string(output))
	}

	m.logger.Info("NFS服务启动成功")
	return nil
}

// Stop 停止NFS服务.
func (m *Manager) Stop() error {
	m.logger.Info("停止NFS服务")

	cmd := exec.CommandContext(context.Background(), "systemctl", "stop", "nfs-kernel-server")
	if output, err := cmd.CombinedOutput(); err != nil {
		m.logger.Errorf("停止NFS服务失败: %s - %v", string(output), err)
		return fmt.Errorf("停止失败: %w - %s", err, string(output))
	}

	m.logger.Info("NFS服务已停止")
	return nil
}

// Restart 重启NFS服务.
func (m *Manager) Restart() error {
	m.logger.Info("重启NFS服务")

	cmd := exec.CommandContext(context.Background(), "systemctl", "restart", "nfs-kernel-server")
	if output, err := cmd.CombinedOutput(); err != nil {
		m.logger.Errorf("重启NFS服务失败: %s - %v", string(output), err)
		return fmt.Errorf("重启失败: %w - %s", err, string(output))
	}

	m.logger.Info("NFS服务重启成功")
	return nil
}

// GetClients 获取连接的客户端信息.
func (m *Manager) GetClients() ([]map[string]string, error) {
	cmd := exec.CommandContext(context.Background(), "showmount", "-a", "localhost")
	output, err := cmd.Output()
	if err != nil {
		m.logger.Errorf("获取客户端信息失败: %v", err)
		return nil, fmt.Errorf("获取客户端信息失败: %w", err)
	}

	var clients []map[string]string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	// 跳过标题行
	for i, line := range lines {
		if i == 0 || line == "" {
			continue
		}

		// 格式: hostname:path
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			clients = append(clients, map[string]string{
				"host": parts[0],
				"path": parts[1],
			})
		}
	}

	return clients, nil
}

// ValidateExport 验证导出配置.
func (m *Manager) ValidateExport(export *Export) error {
	if export == nil {
		return fmt.Errorf("导出配置不能为空")
	}

	if export.Path == "" {
		return fmt.Errorf("导出路径不能为空")
	}

	// 检查路径格式
	if !filepath.IsAbs(export.Path) {
		return fmt.Errorf("导出路径必须是绝对路径")
	}

	// 验证客户端配置
	for _, client := range export.Clients {
		if client.Host == "" {
			return fmt.Errorf("客户端主机不能为空")
		}
	}

	// 检查选项冲突
	if export.Options.Ro && export.Options.Rw {
		return fmt.Errorf("不能同时设置只读和读写选项")
	}

	return nil
}

// GetConfig 获取全局配置.
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新全局配置.
func (m *Manager) UpdateConfig(config *Config) error {
	if err := ValidateConfig(config); err != nil {
		return fmt.Errorf("验证配置失败: %w", err)
	}

	m.mu.Lock()
	m.config = config
	m.mu.Unlock()

	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	m.logger.Info("NFS全局配置已更新")
	return nil
}

// ValidateConfig 验证全局配置.
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("配置不能为空")
	}

	// 验证 NFS 版本
	validVersions := map[string]bool{
		"3":   true,
		"4":   true,
		"4.1": true,
		"4.2": true,
	}
	if config.Version != "" && !validVersions[config.Version] {
		return fmt.Errorf("无效的NFS版本: %s", config.Version)
	}

	// 验证端口范围
	if config.TCPPort != 0 && (config.TCPPort < 1 || config.TCPPort > 65535) {
		return fmt.Errorf("tcp端口超出有效范围: %d", config.TCPPort)
	}
	if config.UDPPort != 0 && (config.UDPPort < 1 || config.UDPPort > 65535) {
		return fmt.Errorf("udp端口超出有效范围: %d", config.UDPPort)
	}
	if config.MountdPort != 0 && (config.MountdPort < 1 || config.MountdPort > 65535) {
		return fmt.Errorf("mountd端口超出有效范围: %d", config.MountdPort)
	}
	if config.StatdPort != 0 && (config.StatdPort < 1 || config.StatdPort > 65535) {
		return fmt.Errorf("statd端口超出有效范围: %d", config.StatdPort)
	}
	if config.LockdPort != 0 && (config.LockdPort < 1 || config.LockdPort > 65535) {
		return fmt.Errorf("lockd端口超出有效范围: %d", config.LockdPort)
	}

	// 验证线程数
	if config.Threads < 0 {
		return fmt.Errorf("线程数不能为负数: %d", config.Threads)
	}

	// 验证最大连接数
	if config.MaxConnections < 0 {
		return fmt.Errorf("最大连接数不能为负数: %d", config.MaxConnections)
	}

	return nil
}
