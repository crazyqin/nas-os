// Package service 提供系统服务管理功能
package service

import (
	"fmt"
	"sync"
	"time"
)

// Manager 系统服务管理器
type Manager struct {
	services map[string]*Service
	mu       sync.RWMutex
	backend  ServiceBackend
}

// Service 服务配置
type Service struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Type        string        `json:"type"` // "systemd", "custom"
	Status      ServiceStatus `json:"status"`
	Enabled     bool          `json:"enabled"`
	UnitFile    string        `json:"unitFile,omitempty"`
}

// ServiceStatus 服务状态
type ServiceStatus struct {
	Running   bool          `json:"running"`
	PID       int           `json:"pid,omitempty"`
	Uptime    time.Duration `json:"uptime,omitempty"`
	Memory    uint64        `json:"memory,omitempty"` // bytes
	CPU       float64       `json:"cpu,omitempty"`    // percentage
	LastError string        `json:"lastError,omitempty"`
	StartedAt time.Time     `json:"startedAt,omitempty"`
}

// ServiceBackend 服务后端接口
type ServiceBackend interface {
	// Start 启动服务
	Start(name string) error
	// Stop 停止服务
	Stop(name string) error
	// Restart 重启服务
	Restart(name string) error
	// Status 获取服务状态
	Status(name string) (*ServiceStatus, error)
	// Enable 启用服务开机自启
	Enable(name string) error
	// Disable 禁用服务开机自启
	Disable(name string) error
	// IsEnabled 检查服务是否开机自启
	IsEnabled(name string) (bool, error)
	// IsRunning 检查服务是否运行中
	IsRunning(name string) (bool, error)
	// List 列出所有服务
	List() ([]*Service, error)
	// Get 获取单个服务信息
	Get(name string) (*Service, error)
}

// 预定义的常用服务
var defaultServices = map[string]*Service{
	"smbd": {
		Name:        "smbd",
		Description: "Samba 文件共享服务",
		Type:        "systemd",
		UnitFile:    "smbd.service",
	},
	"nmbd": {
		Name:        "nmbd",
		Description: "Samba NetBIOS 名称服务",
		Type:        "systemd",
		UnitFile:    "nmbd.service",
	},
	"nfs-server": {
		Name:        "nfs-server",
		Description: "NFS 服务器",
		Type:        "systemd",
		UnitFile:    "nfs-server.service",
	},
	"nfs-client.target": {
		Name:        "nfs-client.target",
		Description: "NFS 客户端",
		Type:        "systemd",
		UnitFile:    "nfs-client.target",
	},
	"rpcbind": {
		Name:        "rpcbind",
		Description: "RPC 绑定服务",
		Type:        "systemd",
		UnitFile:    "rpcbind.service",
	},
	"docker": {
		Name:        "docker",
		Description: "Docker 容器引擎",
		Type:        "systemd",
		UnitFile:    "docker.service",
	},
	"sshd": {
		Name:        "sshd",
		Description: "OpenSSH 服务器",
		Type:        "systemd",
		UnitFile:    "sshd.service",
	},
	"nginx": {
		Name:        "nginx",
		Description: "Nginx Web 服务器",
		Type:        "systemd",
		UnitFile:    "nginx.service",
	},
	"mysql": {
		Name:        "mysql",
		Description: "MySQL 数据库",
		Type:        "systemd",
		UnitFile:    "mysql.service",
	},
	"postgresql": {
		Name:        "postgresql",
		Description: "PostgreSQL 数据库",
		Type:        "systemd",
		UnitFile:    "postgresql.service",
	},
	"redis": {
		Name:        "redis",
		Description: "Redis 缓存服务",
		Type:        "systemd",
		UnitFile:    "redis.service",
	},
	"vsftpd": {
		Name:        "vsftpd",
		Description: "VSFTP FTP 服务器",
		Type:        "systemd",
		UnitFile:    "vsftpd.service",
	},
	"proftpd": {
		Name:        "proftpd",
		Description: "ProFTPD FTP 服务器",
		Type:        "systemd",
		UnitFile:    "proftpd.service",
	},
	"syncthing": {
		Name:        "syncthing",
		Description: "Syncthing 同步服务",
		Type:        "systemd",
		UnitFile:    "syncthing.service",
	},
	"transmission": {
		Name:        "transmission",
		Description: "Transmission BT 下载",
		Type:        "systemd",
		UnitFile:    "transmission.service",
	},
}

// NewManager 创建服务管理器
func NewManager() (*Manager, error) {
	backend, err := NewSystemdBackend()
	if err != nil {
		return nil, fmt.Errorf("创建 systemd 后端失败: %w", err)
	}

	m := &Manager{
		services: make(map[string]*Service),
		backend:  backend,
	}

	// 初始化默认服务列表
	for name, svc := range defaultServices {
		m.services[name] = &Service{
			Name:        svc.Name,
			Description: svc.Description,
			Type:        svc.Type,
			UnitFile:    svc.UnitFile,
		}
	}

	// 刷新服务状态
	if err := m.RefreshAll(); err != nil {
		_ = err // 非致命错误，仅记录
	}

	return m, nil
}

// NewManagerWithBackend 使用自定义后端创建管理器（用于测试）
func NewManagerWithBackend(backend ServiceBackend) (*Manager, error) {
	if backend == nil {
		return nil, fmt.Errorf("后端不能为空")
	}

	m := &Manager{
		services: make(map[string]*Service),
		backend:  backend,
	}

	// 初始化默认服务列表
	for name, svc := range defaultServices {
		m.services[name] = &Service{
			Name:        svc.Name,
			Description: svc.Description,
			Type:        svc.Type,
			UnitFile:    svc.UnitFile,
		}
	}

	return m, nil
}

// Start 启动服务
func (m *Manager) Start(name string) error {
	m.mu.RLock()
	_, exists := m.services[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("服务 %s 不存在", name)
	}

	if err := m.backend.Start(name); err != nil {
		return fmt.Errorf("启动服务 %s 失败: %w", name, err)
	}

	// 更新状态（异步刷新，忽略错误）
	go func() { _ = m.Refresh(name) }()

	return nil
}

// Stop 停止服务
func (m *Manager) Stop(name string) error {
	m.mu.RLock()
	_, exists := m.services[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("服务 %s 不存在", name)
	}

	if err := m.backend.Stop(name); err != nil {
		return fmt.Errorf("停止服务 %s 失败: %w", name, err)
	}

	// 更新状态（异步刷新，忽略错误）
	go func() { _ = m.Refresh(name) }()

	return nil
}

// Restart 重启服务
func (m *Manager) Restart(name string) error {
	m.mu.RLock()
	_, exists := m.services[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("服务 %s 不存在", name)
	}

	if err := m.backend.Restart(name); err != nil {
		return fmt.Errorf("重启服务 %s 失败: %w", name, err)
	}

	// 更新状态
	go func() {
		_ = m.Refresh(name)
	}()

	return nil
}

// Status 获取服务状态
func (m *Manager) Status(name string) (*ServiceStatus, error) {
	m.mu.RLock()
	svc, exists := m.services[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("服务 %s 不存在", name)
	}

	// 从后端获取实时状态
	status, err := m.backend.Status(name)
	if err != nil {
		return nil, fmt.Errorf("获取服务 %s 状态失败: %w", name, err)
	}

	// 更新缓存
	m.mu.Lock()
	svc.Status = *status
	m.mu.Unlock()

	return status, nil
}

// Enable 启用服务开机自启
func (m *Manager) Enable(name string) error {
	m.mu.RLock()
	svc, exists := m.services[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("服务 %s 不存在", name)
	}

	if err := m.backend.Enable(name); err != nil {
		return fmt.Errorf("启用服务 %s 开机自启失败: %w", name, err)
	}

	// 更新缓存
	m.mu.Lock()
	svc.Enabled = true
	m.mu.Unlock()

	return nil
}

// Disable 禁用服务开机自启
func (m *Manager) Disable(name string) error {
	m.mu.RLock()
	svc, exists := m.services[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("服务 %s 不存在", name)
	}

	if err := m.backend.Disable(name); err != nil {
		return fmt.Errorf("禁用服务 %s 开机自启失败: %w", name, err)
	}

	// 更新缓存
	m.mu.Lock()
	svc.Enabled = false
	m.mu.Unlock()

	return nil
}

// List 列出所有服务
func (m *Manager) List() ([]*Service, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回缓存的副本
	result := make([]*Service, 0, len(m.services))
	for _, svc := range m.services {
		// 复制服务对象
		svcCopy := *svc
		result = append(result, &svcCopy)
	}

	return result, nil
}

// Get 获取单个服务信息
func (m *Manager) Get(name string) (*Service, error) {
	m.mu.RLock()
	svc, exists := m.services[name]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("服务 %s 不存在", name)
	}

	// 从后端获取最新状态
	status, err := m.backend.Status(name)
	if err == nil {
		m.mu.Lock()
		svc.Status = *status
		m.mu.Unlock()
	}

	enabled, err := m.backend.IsEnabled(name)
	if err == nil {
		m.mu.Lock()
		svc.Enabled = enabled
		m.mu.Unlock()
	}

	// 返回副本
	m.mu.RLock()
	svcCopy := *svc
	m.mu.RUnlock()

	return &svcCopy, nil
}

// Refresh 刷新单个服务状态
func (m *Manager) Refresh(name string) error {
	m.mu.RLock()
	_, exists := m.services[name]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("服务 %s 不存在", name)
	}

	status, err := m.backend.Status(name)
	if err != nil {
		return err
	}

	enabled, err := m.backend.IsEnabled(name)
	if err != nil {
		// 非致命错误
		enabled = false
	}

	m.mu.Lock()
	if svc, ok := m.services[name]; ok {
		svc.Status = *status
		svc.Enabled = enabled
	}
	m.mu.Unlock()

	return nil
}

// RefreshAll 刷新所有服务状态
func (m *Manager) RefreshAll() error {
	m.mu.RLock()
	names := make([]string, 0, len(m.services))
	for name := range m.services {
		names = append(names, name)
	}
	m.mu.RUnlock()

	var lastErr error
	for _, name := range names {
		if err := m.Refresh(name); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Register 注册新服务
func (m *Manager) Register(svc *Service) error {
	if svc == nil {
		return fmt.Errorf("服务不能为空")
	}

	if svc.Name == "" {
		return fmt.Errorf("服务名称不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.services[svc.Name]; exists {
		return fmt.Errorf("服务 %s 已存在", svc.Name)
	}

	// 设置默认类型
	if svc.Type == "" {
		svc.Type = "systemd"
	}

	m.services[svc.Name] = svc

	// 刷新状态（异步刷新，忽略错误）
	go func() { _ = m.Refresh(svc.Name) }()

	return nil
}

// Unregister 注销服务
func (m *Manager) Unregister(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.services[name]; !exists {
		return fmt.Errorf("服务 %s 不存在", name)
	}

	delete(m.services, name)
	return nil
}

// IsRunning 检查服务是否运行中
func (m *Manager) IsRunning(name string) (bool, error) {
	m.mu.RLock()
	_, exists := m.services[name]
	m.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("服务 %s 不存在", name)
	}

	return m.backend.IsRunning(name)
}

// IsEnabled 检查服务是否开机自启
func (m *Manager) IsEnabled(name string) (bool, error) {
	m.mu.RLock()
	_, exists := m.services[name]
	m.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("服务 %s 不存在", name)
	}

	return m.backend.IsEnabled(name)
}

// GetRunningCount 获取运行中的服务数量
func (m *Manager) GetRunningCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, svc := range m.services {
		if svc.Status.Running {
			count++
		}
	}
	return count
}

// GetEnabledCount 获取开机自启的服务数量
func (m *Manager) GetEnabledCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, svc := range m.services {
		if svc.Enabled {
			count++
		}
	}
	return count
}
