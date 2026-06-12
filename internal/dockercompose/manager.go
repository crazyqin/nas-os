// Package dockercompose 提供 Docker Compose 可视化编排功能
// 支持 Compose 文件解析、服务管理和状态监控
package dockercompose

import (
	"fmt"
	"sync"
	"time"
)

// Project 表示一个 Compose 项目
type Project struct {
	Name        string             `json:"name"`         // 项目名称
	Path        string             `json:"path"`         // compose.yml 路径
	Services    map[string]*Service `json:"services"`    // 服务列表
	Networks    map[string]*Network `json:"networks"`    // 网络列表
	Volumes     map[string]*Volume  `json:"volumes"`     // 卷列表
	Status      string             `json:"status"`      // running/stopped/partial
	CreatedAt   int64              `json:"created_at"`
	UpdatedAt   int64              `json:"updated_at"`
}

// Service 表示一个 Compose 服务
type Service struct {
	Name        string            `json:"name"`         // 服务名称
	Image       string            `json:"image"`        // 镜像
	Ports       []string          `json:"ports"`        // 端口映射
	Volumes     []string          `json:"volumes"`      // 卷映射
	Env         map[string]string `json:"env"`          // 环境变量
	Networks    []string          `json:"networks"`     // 所属网络
	DependsOn   []string          `json:"depends_on"`   // 依赖服务
	Replicas    int               `json:"replicas"`     // 副本数
	Status      string            `json:"status"`       // running/stopped/building
	ContainerID string            `json:"container_id"` // 容器 ID
	CPU         float64           `json:"cpu"`          // CPU 使用率
	Memory      int64             `json:"memory"`       // 内存使用 (bytes)
	RestartPolicy string          `json:"restart_policy"` // 重启策略
	HealthCheck   *HealthCheck    `json:"health_check"`   // 健康检查
}

// HealthCheck 表示健康检查配置
type HealthCheck struct {
	Test        []string `json:"test"`         // 检查命令
	Interval    string   `json:"interval"`     // 检查间隔
	Timeout     string   `json:"timeout"`      // 超时时间
	Retries     int      `json:"retries"`      // 重试次数
	StartPeriod string   `json:"start_period"` // 启动等待时间
}

// Network 表示一个 Compose 网络
type Network struct {
	Name       string            `json:"name"`        // 网络名称
	Driver     string            `json:"driver"`      // 驱动 (bridge/overlay)
	External   bool              `json:"external"`    // 是否外部网络
	Labels     map[string]string `json:"labels"`      // 标签
	Subnet     string            `json:"subnet"`      // 子网
	Gateway    string            `json:"gateway"`     // 网关
}

// Volume 表示一个 Compose 卷
type Volume struct {
	Name       string            `json:"name"`        // 卷名称
	Driver     string            `json:"driver"`      // 驱动
	External   bool              `json:"external"`    // 是否外部卷
	Labels     map[string]string `json:"labels"`      // 标签
	MountPoint string            `json:"mount_point"` // 挂载点
	Size       int64             `json:"size"`        // 大小 (bytes)
}

// Manager 管理 Docker Compose 项目
type Manager struct {
	mu       sync.RWMutex
	projects map[string]*Project
}

// NewManager 创建 Compose 管理器
func NewManager() *Manager {
	return &Manager{
		projects: make(map[string]*Project),
	}
}

// CreateProject 创建 Compose 项目
func (m *Manager) CreateProject(name, path string) (*Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		return nil, fmt.Errorf("项目名称不能为空")
	}

	if _, exists := m.projects[name]; exists {
		return nil, fmt.Errorf("项目 %s 已存在", name)
	}

	project := &Project{
		Name:      name,
		Path:      path,
		Services:  make(map[string]*Service),
		Networks:  make(map[string]*Network),
		Volumes:   make(map[string]*Volume),
		Status:    "stopped",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
	}

	// 添加默认网络
	project.Networks["default"] = &Network{
		Name:   fmt.Sprintf("%s_default", name),
		Driver: "bridge",
	}

	m.projects[name] = project

	return project, nil
}

// DeleteProject 删除项目
func (m *Manager) DeleteProject(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projects[name]; !exists {
		return fmt.Errorf("项目 %s 不存在", name)
	}

	delete(m.projects, name)

	return nil
}

// GetProject 获取项目
func (m *Manager) GetProject(name string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, exists := m.projects[name]
	if !exists {
		return nil, fmt.Errorf("项目 %s 不存在", name)
	}

	return project, nil
}

// ListProjects 列出所有项目
func (m *Manager) ListProjects() []*Project {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Project, 0, len(m.projects))
	for _, project := range m.projects {
		result = append(result, project)
	}
	return result
}

// AddService 添加服务
func (m *Manager) AddService(projectName string, service *Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectName]
	if !exists {
		return fmt.Errorf("项目 %s 不存在", projectName)
	}

	if service.Name == "" {
		return fmt.Errorf("服务名称不能为空")
	}

	if _, exists := project.Services[service.Name]; exists {
		return fmt.Errorf("服务 %s 已存在", service.Name)
	}

	if service.Replicas <= 0 {
		service.Replicas = 1
	}

	if service.RestartPolicy == "" {
		service.RestartPolicy = "unless-stopped"
	}

	service.Status = "stopped"
	project.Services[service.Name] = service
	project.UpdatedAt = time.Now().Unix()

	return nil
}

// RemoveService 移除服务
func (m *Manager) RemoveService(projectName, serviceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectName]
	if !exists {
		return fmt.Errorf("项目 %s 不存在", projectName)
	}

	if _, exists := project.Services[serviceName]; !exists {
		return fmt.Errorf("服务 %s 不存在", serviceName)
	}

	delete(project.Services, serviceName)
	project.UpdatedAt = time.Now().Unix()

	return nil
}

// StartProject 启动项目
func (m *Manager) StartProject(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[name]
	if !exists {
		return fmt.Errorf("项目 %s 不存在", name)
	}

	project.Status = "running"
	for _, service := range project.Services {
		service.Status = "running"
	}
	project.UpdatedAt = time.Now().Unix()

	return nil
}

// StopProject 停止项目
func (m *Manager) StopProject(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[name]
	if !exists {
		return fmt.Errorf("项目 %s 不存在", name)
	}

	project.Status = "stopped"
	for _, service := range project.Services {
		service.Status = "stopped"
	}
	project.UpdatedAt = time.Now().Unix()

	return nil
}

// RestartProject 重启项目
func (m *Manager) RestartProject(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[name]
	if !exists {
		return fmt.Errorf("项目 %s 不存在", name)
	}

	// 先停止
	project.Status = "stopped"
	for _, service := range project.Services {
		service.Status = "stopped"
	}

	// 模拟重启
	go func() {
		time.Sleep(1 * time.Second)
		m.mu.Lock()
		project.Status = "running"
		for _, service := range project.Services {
			service.Status = "running"
		}
		m.mu.Unlock()
	}()

	return nil
}

// StartService 启动单个服务
func (m *Manager) StartService(projectName, serviceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectName]
	if !exists {
		return fmt.Errorf("项目 %s 不存在", projectName)
	}

	service, exists := project.Services[serviceName]
	if !exists {
		return fmt.Errorf("服务 %s 不存在", serviceName)
	}

	service.Status = "running"
	project.UpdatedAt = time.Now().Unix()

	return nil
}

// StopService 停止单个服务
func (m *Manager) StopService(projectName, serviceName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectName]
	if !exists {
		return fmt.Errorf("项目 %s 不存在", projectName)
	}

	service, exists := project.Services[serviceName]
	if !exists {
		return fmt.Errorf("服务 %s 不存在", serviceName)
	}

	service.Status = "stopped"
	project.UpdatedAt = time.Now().Unix()

	return nil
}

// ScaleService 扩缩容服务
func (m *Manager) ScaleService(projectName, serviceName string, replicas int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectName]
	if !exists {
		return fmt.Errorf("项目 %s 不存在", projectName)
	}

	service, exists := project.Services[serviceName]
	if !exists {
		return fmt.Errorf("服务 %s 不存在", serviceName)
	}

	if replicas < 0 {
		return fmt.Errorf("副本数不能为负数")
	}

	service.Replicas = replicas
	project.UpdatedAt = time.Now().Unix()

	return nil
}

// GetProjectStats 获取项目统计信息
func (m *Manager) GetProjectStats(projectName string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, exists := m.projects[projectName]
	if !exists {
		return nil, fmt.Errorf("项目 %s 不存在", projectName)
	}

	runningCount := 0
	for _, service := range project.Services {
		if service.Status == "running" {
			runningCount++
		}
	}

	return map[string]interface{}{
		"total_services":   len(project.Services),
		"running_services": runningCount,
		"total_networks":   len(project.Networks),
		"total_volumes":    len(project.Volumes),
		"status":           project.Status,
	}, nil
}
