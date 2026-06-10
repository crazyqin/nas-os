package lxc

import (
	"fmt"
	"sync"
	"time"
)

// TemplateCategory 模板分类
type TemplateCategory string

const (
	CategoryBase       TemplateCategory = "base"
	CategoryWeb        TemplateCategory = "web"
	CategoryDatabase   TemplateCategory = "database"
	CategoryMonitoring TemplateCategory = "monitoring"
	CategoryDev        TemplateCategory = "dev"
	CategoryStorage    TemplateCategory = "storage"
	CategoryCustom     TemplateCategory = "custom"
)

// Template 容器模板
type Template struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Distro      string            `json:"distro"`
	Version     string            `json:"version"`
	Category    TemplateCategory  `json:"category"`
	Description string            `json:"description"`
	ImageURL    string            `json:"imageURL"`
	Packages    []string          `json:"packages"`
	DefaultRes  ResourceLimit     `json:"defaultResources"`
	DefaultNet  NetworkConfig     `json:"defaultNetwork"`
	DefaultVol  []VolumeMount     `json:"defaultVolumes"`
	Metadata    map[string]string `json:"metadata"`
	SizeMB      int               `json:"sizeMB"`
	IsBuiltin   bool              `json:"isBuiltin"`
	Tags        []string          `json:"tags"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// TemplateManager 模板管理器
type TemplateManager struct {
	mu        sync.RWMutex
	templates map[string]*Template
}

// NewTemplateManager 创建模板管理器
func NewTemplateManager() *TemplateManager {
	tm := &TemplateManager{
		templates: make(map[string]*Template),
	}
	tm.loadBuiltinTemplates()
	return tm
}

// loadBuiltinTemplates 加载内置模板
func (tm *TemplateManager) loadBuiltinTemplates() {
	builtins := []*Template{
		// 基础系统模板
		{
			Name:        "ubuntu-24.04",
			Distro:      "ubuntu",
			Version:     "24.04",
			Category:    CategoryBase,
			Description: "Ubuntu 24.04 LTS - 通用服务器环境",
			Packages:    []string{"curl", "wget", "vim", "git", "openssh-server"},
			DefaultRes:  ResourceLimit{CPUCores: 1, MemoryMB: 512, DiskGB: 10},
			DefaultNet:  NetworkConfig{Mode: NetworkBridge},
			SizeMB:      300,
			Tags:        []string{"lts", "server", "popular"},
		},
		{
			Name:        "debian-12",
			Distro:      "debian",
			Version:     "12",
			Category:    CategoryBase,
			Description: "Debian 12 (Bookworm) - 稳定服务器环境",
			Packages:    []string{"curl", "wget", "vim", "openssh-server"},
			DefaultRes:  ResourceLimit{CPUCores: 1, MemoryMB: 512, DiskGB: 10},
			DefaultNet:  NetworkConfig{Mode: NetworkBridge},
			SizeMB:      250,
			Tags:        []string{"stable", "server"},
		},
		{
			Name:        "alpine-3.20",
			Distro:      "alpine",
			Version:     "3.20",
			Category:    CategoryBase,
			Description: "Alpine Linux 3.20 - 轻量级容器环境",
			Packages:    []string{"busybox", "curl", "openssl"},
			DefaultRes:  ResourceLimit{CPUCores: 1, MemoryMB: 128, DiskGB: 2},
			DefaultNet:  NetworkConfig{Mode: NetworkBridge},
			SizeMB:      50,
			Tags:        []string{"lightweight", "minimal", "fast"},
		},
		{
			Name:        "centos-stream-9",
			Distro:      "centos",
			Version:     "stream-9",
			Category:    CategoryBase,
			Description: "CentOS Stream 9 - 企业级服务器环境",
			Packages:    []string{"curl", "wget", "vim", "openssh-server"},
			DefaultRes:  ResourceLimit{CPUCores: 1, MemoryMB: 512, DiskGB: 10},
			DefaultNet:  NetworkConfig{Mode: NetworkBridge},
			SizeMB:      320,
			Tags:        []string{"enterprise", "rhel"},
		},

		// Web 服务模板
		{
			Name:        "nginx-web",
			Distro:      "debian",
			Version:     "12",
			Category:    CategoryWeb,
			Description: "Nginx Web 服务器 - 预装配置优化",
			Packages:    []string{"nginx", "certbot", "openssl"},
			DefaultRes:  ResourceLimit{CPUCores: 1, MemoryMB: 256, DiskGB: 5},
			DefaultNet:  NetworkConfig{Mode: NetworkBridge, Ports: []PortMap{{HostPort: 80, ContainerPort: 80, Protocol: "tcp"}, {HostPort: 443, ContainerPort: 443, Protocol: "tcp"}}},
			DefaultVol:  []VolumeMount{{Source: "/var/www", Destination: "/var/www/html"}, {Source: "/etc/nginx", Destination: "/etc/nginx"}},
			SizeMB:      280,
			Tags:        []string{"web", "http", "reverse-proxy"},
		},
		{
			Name:        "nodejs-app",
			Distro:      "debian",
			Version:     "12",
			Category:    CategoryWeb,
			Description: "Node.js 应用服务器 - 预装 Node.js 20 LTS",
			Packages:    []string{"nodejs", "npm", "git", "openssh-server"},
			DefaultRes:  ResourceLimit{CPUCores: 1, MemoryMB: 512, DiskGB: 10},
			DefaultNet:  NetworkConfig{Mode: NetworkBridge, Ports: []PortMap{{HostPort: 3000, ContainerPort: 3000, Protocol: "tcp"}}},
			SizeMB:      350,
			Tags:        []string{"node", "javascript", "app"},
		},

		// 数据库模板
		{
			Name:        "mysql-8",
			Distro:      "debian",
			Version:     "12",
			Category:    CategoryDatabase,
			Description: "MySQL 8.0 数据库服务器",
			Packages:    []string{"mysql-server", "mysql-client"},
			DefaultRes:  ResourceLimit{CPUCores: 2, MemoryMB: 1024, DiskGB: 50},
			DefaultNet:  NetworkConfig{Mode: NetworkBridge, Ports: []PortMap{{HostPort: 3306, ContainerPort: 3306, Protocol: "tcp"}}},
			DefaultVol:  []VolumeMount{{Source: "/var/lib/mysql", Destination: "/var/lib/mysql"}},
			SizeMB:      400,
			Tags:        []string{"database", "mysql", "sql"},
		},
		{
			Name:        "postgresql-16",
			Distro:      "debian",
			Version:     "12",
			Category:    CategoryDatabase,
			Description: "PostgreSQL 16 数据库服务器",
			Packages:    []string{"postgresql", "postgresql-client"},
			DefaultRes:  ResourceLimit{CPUCores: 2, MemoryMB: 1024, DiskGB: 50},
			DefaultNet:  NetworkConfig{Mode: NetworkBridge, Ports: []PortMap{{HostPort: 5432, ContainerPort: 5432, Protocol: "tcp"}}},
			DefaultVol:  []VolumeMount{{Source: "/var/lib/postgresql", Destination: "/var/lib/postgresql"}},
			SizeMB:      380,
			Tags:        []string{"database", "postgresql", "sql"},
		},
		{
			Name:        "redis-7",
			Distro:      "alpine",
			Version:     "3.20",
			Category:    CategoryDatabase,
			Description: "Redis 7 内存数据库",
			Packages:    []string{"redis"},
			DefaultRes:  ResourceLimit{CPUCores: 1, MemoryMB: 256, DiskGB: 5},
			DefaultNet:  NetworkConfig{Mode: NetworkBridge, Ports: []PortMap{{HostPort: 6379, ContainerPort: 6379, Protocol: "tcp"}}},
			SizeMB:      60,
			Tags:        []string{"database", "redis", "cache"},
		},

		// 监控模板
		{
			Name:        "prometheus",
			Distro:      "alpine",
			Version:     "3.20",
			Category:    CategoryMonitoring,
			Description: "Prometheus 监控服务器",
			Packages:    []string{"prometheus", "grafana"},
			DefaultRes:  ResourceLimit{CPUCores: 1, MemoryMB: 512, DiskGB: 20},
			DefaultNet:  NetworkConfig{Mode: NetworkBridge, Ports: []PortMap{{HostPort: 9090, ContainerPort: 9090, Protocol: "tcp"}, {HostPort: 3000, ContainerPort: 3000, Protocol: "tcp"}}},
			DefaultVol:  []VolumeMount{{Source: "/var/lib/prometheus", Destination: "/var/lib/prometheus"}},
			SizeMB:      200,
			Tags:        []string{"monitoring", "prometheus", "grafana"},
		},

		// 开发环境模板
		{
			Name:        "golang-dev",
			Distro:      "ubuntu",
			Version:     "24.04",
			Category:    CategoryDev,
			Description: "Go 语言开发环境 - 预装 Go 1.22",
			Packages:    []string{"golang", "git", "vim", "make", "gcc"},
			DefaultRes:  ResourceLimit{CPUCores: 2, MemoryMB: 1024, DiskGB: 20},
			DefaultNet:  NetworkConfig{Mode: NetworkBridge},
			SizeMB:      500,
			Tags:        []string{"dev", "golang", "go"},
		},
		{
			Name:        "python-dev",
			Distro:      "ubuntu",
			Version:     "24.04",
			Category:    CategoryDev,
			Description: "Python 开发环境 - 预装 Python 3.12",
			Packages:    []string{"python3", "python3-pip", "python3-venv", "git", "vim"},
			DefaultRes:  ResourceLimit{CPUCores: 1, MemoryMB: 512, DiskGB: 15},
			DefaultNet:  NetworkConfig{Mode: NetworkBridge},
			SizeMB:      450,
			Tags:        []string{"dev", "python", "python3"},
		},
	}

	now := time.Now()
	for _, t := range builtins {
		t.ID = fmt.Sprintf("tpl-%s-%s", t.Distro, t.Version)
		t.IsBuiltin = true
		t.CreatedAt = now
		t.UpdatedAt = now
		tm.templates[t.Name] = t
	}
}

// Register 注册新模板
func (tm *TemplateManager) Register(t *Template) error {
	if t.Name == "" {
		return fmt.Errorf("模板名称不能为空")
	}
	if t.Distro == "" {
		return fmt.Errorf("发行版不能为空")
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if _, exists := tm.templates[t.Name]; exists {
		return fmt.Errorf("模板 %s 已存在", t.Name)
	}

	now := time.Now()
	if t.ID == "" {
		t.ID = fmt.Sprintf("tpl-custom-%s", t.Name)
	}
	t.IsBuiltin = false
	t.CreatedAt = now
	t.UpdatedAt = now

	tm.templates[t.Name] = t
	return nil
}

// Get 获取模板
func (tm *TemplateManager) Get(name string) (*Template, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, ok := tm.templates[name]
	if !ok {
		return nil, fmt.Errorf("模板 %s 不存在", name)
	}
	return t, nil
}

// List 列出所有模板
func (tm *TemplateManager) List() []*Template {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	result := make([]*Template, 0, len(tm.templates))
	for _, t := range tm.templates {
		result = append(result, t)
	}
	return result
}

// ListByCategory 按分类列出模板
func (tm *TemplateManager) ListByCategory(category TemplateCategory) []*Template {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []*Template
	for _, t := range tm.templates {
		if t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// ListByDistro 按发行版列出模板
func (tm *TemplateManager) ListByDistro(distro string) []*Template {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var result []*Template
	for _, t := range tm.templates {
		if t.Distro == distro {
			result = append(result, t)
		}
	}
	return result
}

// Update 更新模板
func (tm *TemplateManager) Update(name string, t *Template) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	existing, ok := tm.templates[name]
	if !ok {
		return fmt.Errorf("模板 %s 不存在", name)
	}
	if existing.IsBuiltin {
		return fmt.Errorf("内置模板 %s 不允许修改", name)
	}

	t.ID = existing.ID
	t.Name = name
	t.IsBuiltin = false
	t.CreatedAt = existing.CreatedAt
	t.UpdatedAt = time.Now()

	tm.templates[name] = t
	return nil
}

// Delete 删除模板
func (tm *TemplateManager) Delete(name string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	t, ok := tm.templates[name]
	if !ok {
		return fmt.Errorf("模板 %s 不存在", name)
	}
	if t.IsBuiltin {
		return fmt.Errorf("内置模板 %s 不允许删除", name)
	}

	delete(tm.templates, name)
	return nil
}

// Exists 检查模板是否存在
func (tm *TemplateManager) Exists(name string) bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	_, ok := tm.templates[name]
	return ok
}

// Count 返回模板数量
func (tm *TemplateManager) Count() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return len(tm.templates)
}

// GetDefaultResources 获取模板默认资源限制
func (tm *TemplateManager) GetDefaultResources(name string) (ResourceLimit, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	t, ok := tm.templates[name]
	if !ok {
		return ResourceLimit{}, fmt.Errorf("模板 %s 不存在", name)
	}
	return t.DefaultRes, nil
}
