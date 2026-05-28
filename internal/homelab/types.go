// Package homelab 家庭实验室管理器 - 统一管理Docker/VM/服务
package homelab

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"
)

// ServiceType 服务类型
type ServiceType string

const (
	ServiceDocker  ServiceType = "docker"
	ServiceVM      ServiceType = "vm"
	ServiceCompose ServiceType = "compose"
	ServiceK3s     ServiceType = "k3s"
	ServiceSystemd ServiceType = "systemd"
)

// ServiceStatus 服务状态
type ServiceStatus string

const (
	StatusRunning  ServiceStatus = "running"
	StatusStopped  ServiceStatus = "stopped"
	StatusError    ServiceStatus = "error"
	StatusStarting ServiceStatus = "starting"
	StatusStopping ServiceStatus = "stopping"
)

// Service 服务定义
type Service struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Type        ServiceType   `json:"type"`
	Status      ServiceStatus `json:"status"`
	Image       string        `json:"image,omitempty"`
	Port        int           `json:"port,omitempty"`
	Ports       []string      `json:"ports,omitempty"`
	Volumes     []string      `json:"volumes,omitempty"`
	Env         []string      `json:"env,omitempty"`
	CPUUsage    float64       `json:"cpu_usage"`
	MemUsage    int64         `json:"mem_usage"`
	MemLimit    int64         `json:"mem_limit"`
	NetRx       int64         `json:"net_rx"`
	NetTx       int64         `json:"net_tx"`
	Uptime      int64         `json:"uptime"`
	RestartCount int          `json:"restart_count"`
	HealthCheck string        `json:"health_check,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// Stack 编排栈
type Stack struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Services  []string          `json:"services"`
	Status    ServiceStatus     `json:"status"`
	Env       map[string]string `json:"env,omitempty"`
	Networks  []string          `json:"networks,omitempty"`
	Volumes   []string          `json:"volumes,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Template 服务模板
type Template struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Image       string            `json:"image"`
	Ports       []string          `json:"ports"`
	Volumes     []string          `json:"volumes"`
	Env         map[string]string `json:"env"`
	Icon        string            `json:"icon,omitempty"`
	Downloads   int               `json:"downloads"`
	Rating      float64           `json:"rating"`
}

// Config 配置
type Config struct {
	DockerHost     string `json:"docker_host"`
	K3sConfig      string `json:"k3s_config"`
	AutoRestart    bool   `json:"auto_restart"`
	HealthInterval int    `json:"health_interval"`
	MaxServices    int    `json:"max_services"`
}

// Manager 管理器
type Manager struct {
	mu        sync.RWMutex
	services  map[string]*Service
	stacks    map[string]*Stack
	templates map[string]*Template
	config    *Config
	dataFile  string
}

var (
	ErrServiceNotFound = errors.New("service not found")
	ErrStackNotFound   = errors.New("stack not found")
	ErrServiceExists   = errors.New("service already exists")
	ErrMaxServices     = errors.New("max services reached")
	ErrInvalidType     = errors.New("invalid service type")
)

// NewManager 创建管理器
func NewManager(dataFile string) *Manager {
	return &Manager{
		services:  make(map[string]*Service),
		stacks:    make(map[string]*Stack),
		templates: make(map[string]*Template),
		config: &Config{
			DockerHost:     "unix:///var/run/docker.sock",
			AutoRestart:    true,
			HealthInterval: 30,
			MaxServices:    200,
		},
		dataFile: dataFile,
	}
}

// Initialize 初始化
func (m *Manager) Initialize() error {
	m.loadDefaultTemplates()
	return m.load()
}

func (m *Manager) loadDefaultTemplates() {
	templates := []Template{
		{ID: "nextcloud", Name: "Nextcloud", Description: "私有云盘", Category: "存储", Image: "nextcloud:latest", Ports: []string{"8080:80"}, Downloads: 50000, Rating: 4.5},
		{ID: "jellyfin", Name: "Jellyfin", Description: "媒体服务器", Category: "媒体", Image: "jellyfin/jellyfin:latest", Ports: []string{"8096:8096"}, Downloads: 40000, Rating: 4.6},
		{ID: "homeassistant", Name: "Home Assistant", Description: "智能家居", Category: "智能家居", Image: "homeassistant/home-assistant:stable", Ports: []string{"8123:8123"}, Downloads: 60000, Rating: 4.8},
		{ID: "vaultwarden", Name: "Vaultwarden", Description: "密码管理", Category: "安全", Image: "vaultwarden/server:latest", Ports: []string{"8880:80"}, Downloads: 30000, Rating: 4.7},
		{ID: "pihole", Name: "Pi-hole", Description: "DNS广告过滤", Category: "网络", Image: "pihole/pihole:latest", Ports: []string{"53:53/tcp", "53:53/udp", "8053:80"}, Downloads: 45000, Rating: 4.4},
		{ID: "portainer", Name: "Portainer", Description: "容器管理UI", Category: "运维", Image: "portainer/portainer-ce:latest", Ports: []string{"9000:9000"}, Downloads: 70000, Rating: 4.3},
		{ID: "immich", Name: "Immich", Description: "自托管照片备份", Category: "媒体", Image: "ghcr.io/immich-app/immich-server:release", Ports: []string{"2283:2283"}, Downloads: 35000, Rating: 4.7},
		{ID: "n8n", Name: "n8n", Description: "工作流自动化", Category: "自动化", Image: "n8nio/n8n:latest", Ports: []string{"5678:5678"}, Downloads: 25000, Rating: 4.5},
	}
	for i := range templates {
		m.templates[templates[i].ID] = &templates[i]
	}
}

// CreateService 创建服务
func (m *Manager) CreateService(svc *Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.services[svc.ID]; exists {
		return ErrServiceExists
	}
	if len(m.services) >= m.config.MaxServices {
		return ErrMaxServices
	}
	if !isValidType(svc.Type) {
		return ErrInvalidType
	}

	svc.Status = StatusStopped
	svc.CreatedAt = time.Now()
	svc.UpdatedAt = time.Now()
	m.services[svc.ID] = svc
	return m.save()
}

// StartService 启动服务
func (m *Manager) StartService(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, ok := m.services[id]
	if !ok {
		return ErrServiceNotFound
	}
	svc.Status = StatusRunning
	svc.UpdatedAt = time.Now()
	return m.save()
}

// StopService 停止服务
func (m *Manager) StopService(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, ok := m.services[id]
	if !ok {
		return ErrServiceNotFound
	}
	svc.Status = StatusStopped
	svc.UpdatedAt = time.Now()
	return m.save()
}

// RestartService 重启服务
func (m *Manager) RestartService(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	svc, ok := m.services[id]
	if !ok {
		return ErrServiceNotFound
	}
	svc.Status = StatusStarting
	svc.RestartCount++
	svc.UpdatedAt = time.Now()
	// 模拟重启
	svc.Status = StatusRunning
	return m.save()
}

// DeleteService 删除服务
func (m *Manager) DeleteService(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.services[id]; !ok {
		return ErrServiceNotFound
	}
	delete(m.services, id)
	return m.save()
}

// GetService 获取服务
func (m *Manager) GetService(id string) (*Service, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	svc, ok := m.services[id]
	if !ok {
		return nil, ErrServiceNotFound
	}
	return svc, nil
}

// ListServices 列出服务
func (m *Manager) ListServices(svcType ServiceType, status ServiceStatus) []*Service {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Service
	for _, svc := range m.services {
		if svcType != "" && svc.Type != svcType {
			continue
		}
		if status != "" && svc.Status != status {
			continue
		}
		result = append(result, svc)
	}
	return result
}

// CreateStack 创建编排栈
func (m *Manager) CreateStack(stack *Stack) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stack.Status = StatusStopped
	stack.UpdatedAt = time.Now()
	m.stacks[stack.ID] = stack
	return m.save()
}

// StartStack 启动编排栈
func (m *Manager) StartStack(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stack, ok := m.stacks[id]
	if !ok {
		return ErrStackNotFound
	}
	stack.Status = StatusRunning
	for _, svcID := range stack.Services {
		if svc, ok := m.services[svcID]; ok {
			svc.Status = StatusRunning
		}
	}
	stack.UpdatedAt = time.Now()
	return m.save()
}

// StopStack 停止编排栈
func (m *Manager) StopStack(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stack, ok := m.stacks[id]
	if !ok {
		return ErrStackNotFound
	}
	stack.Status = StatusStopped
	for _, svcID := range stack.Services {
		if svc, ok := m.services[svcID]; ok {
			svc.Status = StatusStopped
		}
	}
	stack.UpdatedAt = time.Now()
	return m.save()
}

// GetStack 获取编排栈
func (m *Manager) GetStack(id string) (*Stack, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stack, ok := m.stacks[id]
	if !ok {
		return nil, ErrStackNotFound
	}
	return stack, nil
}

// ListStacks 列出编排栈
func (m *Manager) ListStacks() []*Stack {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Stack
	for _, stack := range m.stacks {
		result = append(result, stack)
	}
	return result
}

// ListTemplates 列出模板
func (m *Manager) ListTemplates(category string) []*Template {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Template
	for _, tpl := range m.templates {
		if category != "" && tpl.Category != category {
			continue
		}
		result = append(result, tpl)
	}
	return result
}

// DeployFromTemplate 从模板部署
func (m *Manager) DeployFromTemplate(templateID string, name string, env map[string]string) (*Service, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tpl, ok := m.templates[templateID]
	if !ok {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	svc := &Service{
		ID:      fmt.Sprintf("%s-%d", name, time.Now().Unix()),
		Name:    name,
		Type:    ServiceDocker,
		Image:   tpl.Image,
		Ports:   tpl.Ports,
		Labels:  map[string]string{"template": templateID},
	}

	svc.Status = StatusRunning
	svc.CreatedAt = time.Now()
	svc.UpdatedAt = time.Now()
	m.services[svc.ID] = svc

	tpl.Downloads++
	return svc, m.save()
}

// GetStats 获取统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	running := 0
	stopped := 0
	errors := 0
	for _, svc := range m.services {
		switch svc.Status {
		case StatusRunning:
			running++
		case StatusStopped:
			stopped++
		case StatusError:
			errors++
		}
	}

	return map[string]interface{}{
		"total_services":  len(m.services),
		"running":         running,
		"stopped":         stopped,
		"errors":          errors,
		"total_stacks":    len(m.stacks),
		"total_templates": len(m.templates),
	}
}

func (m *Manager) load() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var stored struct {
		Services  map[string]*Service `json:"services"`
		Stacks    map[string]*Stack   `json:"stacks"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	if stored.Services != nil {
		m.services = stored.Services
	}
	if stored.Stacks != nil {
		m.stacks = stored.Stacks
	}
	return nil
}

func (m *Manager) save() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := json.MarshalIndent(struct {
		Services map[string]*Service `json:"services"`
		Stacks   map[string]*Stack   `json:"stacks"`
	}{m.services, m.stacks}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.dataFile, data, 0644)
}

func isValidType(t ServiceType) bool {
	switch t {
	case ServiceDocker, ServiceVM, ServiceCompose, ServiceK3s, ServiceSystemd:
		return true
	}
	return false
}
