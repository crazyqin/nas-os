package nfs

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Export NFS 导出配置
type Export struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Comment     string   `json:"comment"`
	ReadOnly    bool     `json:"read_only"`
	NoSubtreeCheck bool  `json:"no_subtree_check"`
	Sync        bool     `json:"sync"`
	AllowedNetworks []string `json:"allowed_networks"` // CIDR 格式
	AllowedHosts []string    `json:"allowed_hosts"`    // 单个 IP
}

// ExportInput 创建/更新导出输入
type ExportInput struct {
	Name        string   `json:"name" binding:"required"`
	Path        string   `json:"path" binding:"required"`
	Comment     string   `json:"comment"`
	ReadOnly    bool     `json:"read_only"`
	AllowedNetworks []string `json:"allowed_networks"`
	AllowedHosts []string    `json:"allowed_hosts"`
}

// Config NFS 配置
type Config struct {
	Enabled       bool     `json:"enabled"`
	Threads       int      `json:"threads"`
	GracePeriod   int      `json:"grace_period"` // 秒
	LeaseTime     int      `json:"lease_time"`   // 秒
}

// Manager NFS 管理器
type Manager struct {
	mu         sync.RWMutex
	exports    map[string]*Export
	config     *Config
	configPath string
}

var defaultConfig = &Config{
	Enabled:     true,
	Threads:     8,
	GracePeriod: 90,
	LeaseTime:   90,
}

// NewManager 创建 NFS 管理器
func NewManager(configPath string) (*Manager, error) {
	m := &Manager{
		exports:    make(map[string]*Export),
		config:     defaultConfig,
		configPath: configPath,
	}
	return m, nil
}

// generateExports 生成 /etc/exports 内容
func (m *Manager) generateExports() string {
	var sb strings.Builder

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, exp := range m.exports {
		options := []string{"rw"}
		if exp.ReadOnly {
			options = []string{"ro"}
		}
		if exp.NoSubtreeCheck {
			options = append(options, "no_subtree_check")
		}
		if exp.Sync {
			options = append(options, "sync")
		} else {
			options = append(options, "async")
		}
		options = append(options, "no_root_squash")

		// 构建客户端列表
		clients := exp.AllowedNetworks
		clients = append(clients, exp.AllowedHosts...)
		if len(clients) == 0 {
			clients = []string{"*(rw,no_root_squash)"}
		}

		clientStr := strings.Join(clients, "("+strings.Join(options, ",")+"),")
		sb.WriteString(fmt.Sprintf("%s\t%s(%s)\n", exp.Path, clientStr, strings.Join(options, ",")))
	}

	return sb.String()
}

// ApplyConfig 应用配置
func (m *Manager) ApplyConfig() error {
	m.mu.RLock()
	exportsContent := m.generateExports()
	m.mu.RUnlock()

	// 写入 exports 文件
	if err := os.WriteFile("/etc/exports", []byte(exportsContent), 0644); err != nil {
		return fmt.Errorf("写入 exports 失败：%w", err)
	}

	// 重新导出
	cmd := exec.Command("exportfs", "-ra")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("重新导出失败：%w", err)
	}

	return nil
}

// CreateExport 创建导出
func (m *Manager) CreateExport(input ExportInput) (*Export, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.exports[input.Name]; exists {
		return nil, fmt.Errorf("导出已存在")
	}

	// 确保路径存在
	if err := os.MkdirAll(input.Path, 0755); err != nil {
		return nil, fmt.Errorf("创建目录失败：%w", err)
	}

	exp := &Export{
		Name:        input.Name,
		Path:        input.Path,
		Comment:     input.Comment,
		ReadOnly:    input.ReadOnly,
		NoSubtreeCheck: true,
		Sync:        false,
		AllowedNetworks: input.AllowedNetworks,
		AllowedHosts: input.AllowedHosts,
	}

	m.exports[input.Name] = exp
	return exp, nil
}

// GetExport 获取导出
func (m *Manager) GetExport(name string) (*Export, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exp, exists := m.exports[name]
	if !exists {
		return nil, fmt.Errorf("导出不存在")
	}
	return exp, nil
}

// ListExports 获取导出列表
func (m *Manager) ListExports() []*Export {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exports := make([]*Export, 0, len(m.exports))
	for _, e := range m.exports {
		exports = append(exports, e)
	}
	return exports
}

// UpdateExport 更新导出
func (m *Manager) UpdateExport(name string, input ExportInput) (*Export, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, exists := m.exports[name]
	if !exists {
		return nil, fmt.Errorf("导出不存在")
	}

	exp.Comment = input.Comment
	exp.ReadOnly = input.ReadOnly
	exp.AllowedNetworks = input.AllowedNetworks
	exp.AllowedHosts = input.AllowedHosts

	return exp, nil
}

// DeleteExport 删除导出
func (m *Manager) DeleteExport(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.exports[name]; !exists {
		return fmt.Errorf("导出不存在")
	}

	delete(m.exports, name)
	return nil
}

// GetStatus 获取 NFS 服务状态
func (m *Manager) GetStatus() (running bool, err error) {
	cmd := exec.Command("systemctl", "is-active", "nfs-kernel-server")
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return strings.TrimSpace(string(output)) == "active", nil
}

// Start 启动 NFS 服务
func (m *Manager) Start() error {
	cmd := exec.Command("systemctl", "start", "nfs-kernel-server")
	return cmd.Run()
}

// Stop 停止 NFS 服务
func (m *Manager) Stop() error {
	cmd := exec.Command("systemctl", "stop", "nfs-kernel-server")
	return cmd.Run()
}

// Restart 重启 NFS 服务
func (m *Manager) Restart() error {
	cmd := exec.Command("systemctl", "restart", "nfs-kernel-server")
	return cmd.Run()
}

// GetClientInfo 获取连接的客户端信息
func (m *Manager) GetClientInfo() ([]string, error) {
	cmd := exec.Command("showmount", "-a", "localhost")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	return lines, nil
}

// ExportPath 获取导出路径
func (m *Manager) ExportPath(name string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if exp, exists := m.exports[name]; exists {
		return exp.Path
	}
	return ""
}
