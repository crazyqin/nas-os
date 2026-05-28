// Package containerwizard 提供容器一键部署向导功能
// 模板市场、一键安装、配置推荐、依赖自动解析
package containerwizard

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 核心类型 ==========

// Template 容器部署模板
type Template struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Slug        string            `json:"slug"`
	Description string            `json:"description"`
	Category    TemplateCategory  `json:"category"`
	Icon        string            `json:"icon,omitempty"`
	Author      string            `json:"author"`
	Version     string            `json:"version"`
	Image       string            `json:"image"`
	Tag         string            `json:"tag"`
	Ports       []PortDef         `json:"ports,omitempty"`
	Volumes     []VolumeDef       `json:"volumes,omitempty"`
	EnvVars     []EnvVarDef       `json:"envVars,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	MinCPU      float64           `json:"minCpu,omitempty"`
	MinMemory   int64             `json:"minMemory,omitempty"` // bytes
	MinDisk     int64             `json:"minDisk,omitempty"`   // bytes
	Requires    []string          `json:"requires,omitempty"`  // 依赖的模板 slug
	Tags        []string          `json:"tags,omitempty"`
	Featured    bool              `json:"featured"`
	Downloads   int64             `json:"downloads"`
	Rating      float64           `json:"rating"`
	Docs        string            `json:"docs,omitempty"`
	ConfigJSON  string            `json:"configJson,omitempty"` // 默认配置 JSON
	HealthCheck *HealthCheckDef   `json:"healthCheck,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// TemplateCategory 模板分类
type TemplateCategory string

const (
	CategoryMedia      TemplateCategory = "media"
	CategoryDatabase   TemplateCategory = "database"
	CategoryWeb        TemplateCategory = "web"
	CategoryDevOps     TemplateCategory = "devops"
	CategoryNetwork    TemplateCategory = "network"
	CategoryStorage    TemplateCategory = "storage"
	CategoryAI         TemplateCategory = "ai"
	CategoryMonitoring TemplateCategory = "monitoring"
	CategorySecurity   TemplateCategory = "security"
	CategoryProductivity TemplateCategory = "productivity"
	CategoryGame       TemplateCategory = "game"
	CategoryOther      TemplateCategory = "other"
)

// PortDef 端口定义
type PortDef struct {
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol,omitempty"`
	Desc      string `json:"desc,omitempty"`
}

// VolumeDef 卷定义
type VolumeDef struct {
	Host      string `json:"host"`
	Container string `json:"container"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
	Desc      string `json:"desc,omitempty"`
	Default   string `json:"default,omitempty"`
}

// EnvVarDef 环境变量定义
type EnvVarDef struct {
	Name        string `json:"name"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
}

// HealthCheckDef 健康检查定义
type HealthCheckDef struct {
	Test     []string `json:"test"`
	Interval int      `json:"interval"` // seconds
	Timeout  int      `json:"timeout"`  // seconds
	Retries  int      `json:"retries"`
}

// DeployTask 部署任务
type DeployTask struct {
	ID          string            `json:"id"`
	TemplateID  string            `json:"templateId"`
	Name        string            `json:"name"`
	Status      DeployStatus      `json:"status"`
	Overrides   map[string]string `json:"overrides,omitempty"`
	ContainerID string            `json:"containerId,omitempty"`
	Logs        []string          `json:"logs,omitempty"`
	Error       string            `json:"error,omitempty"`
	StartedAt   time.Time         `json:"startedAt"`
	CompletedAt *time.Time        `json:"completedAt,omitempty"`
}

// DeployStatus 部署状态
type DeployStatus string

const (
	DeployStatusPending   DeployStatus = "pending"
	DeployStatusPulling   DeployStatus = "pulling"
	DeployStatusConfiguring DeployStatus = "configuring"
	DeployStatusStarting  DeployStatus = "starting"
	DeployStatusRunning   DeployStatus = "running"
	DeployStatusFailed    DeployStatus = "failed"
	DeployStatusStopped   DeployStatus = "stopped"
)

// ResourceRecommend 资源推荐
type ResourceRecommend struct {
	CPU       float64 `json:"cpu"`
	Memory    int64   `json:"memory"` // bytes
	Disk      int64   `json:"disk"`   // bytes
	Reason    string  `json:"reason"`
}

// StackTemplate 组合模板（多容器一键部署）
type StackTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    TemplateCategory  `json:"category"`
	Templates   []StackItem       `json:"templates"` // 包含的模板
	Tags        []string          `json:"tags,omitempty"`
	Featured    bool              `json:"featured"`
}

// StackItem 组合模板中的单个服务
type StackItem struct {
	TemplateSlug string            `json:"templateSlug"`
	Alias        string            `json:"alias"`
	Overrides    map[string]string `json:"overrides,omitempty"`
	DependsOn    []string          `json:"dependsOn,omitempty"`
}

// ========== Manager ==========

// Manager 容器向导管理器
type Manager struct {
	mu         sync.RWMutex
	templates  map[string]*Template
	stacks     map[string]*StackTemplate
	tasks      map[string]*DeployTask
	installed  map[string]*DeployTask // templateSlug -> task
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		templates: make(map[string]*Template),
		stacks:    make(map[string]*StackTemplate),
		tasks:     make(map[string]*DeployTask),
		installed: make(map[string]*DeployTask),
	}
	m.initDefaultTemplates()
	return m
}

// initDefaultTemplates 初始化默认模板
func (m *Manager) initDefaultTemplates() {
	defaults := []*Template{
		{
			Name: "Jellyfin", Slug: "jellyfin", Category: CategoryMedia,
			Description: "免费开源媒体服务器，支持视频/音乐/图片流媒体播放",
			Image: "jellyfin/jellyfin", Tag: "latest",
			Ports: []PortDef{{Host: 8096, Container: 8096, Desc: "Web UI"}},
			Volumes: []VolumeDef{
				{Host: "/data/jellyfin/config", Container: "/config", Desc: "配置"},
				{Host: "/data/media", Container: "/media", Desc: "媒体库"},
			},
			MinCPU: 1, MinMemory: 512 * 1024 * 1024, Featured: true, Rating: 4.8,
			Tags: []string{"媒体", "视频", "流媒体"},
		},
		{
			Name: "Nextcloud", Slug: "nextcloud", Category: CategoryStorage,
			Description: "私有云盘，文件同步、日历、联系人、协作办公",
			Image: "nextcloud", Tag: "latest",
			Ports: []PortDef{{Host: 8080, Container: 80, Desc: "Web UI"}},
			Volumes: []VolumeDef{
				{Host: "/data/nextcloud/html", Container: "/var/www/html", Desc: "应用"},
				{Host: "/data/nextcloud/data", Container: "/var/www/html/data", Desc: "数据"},
			},
			MinCPU: 1, MinMemory: 1024 * 1024 * 1024, Featured: true, Rating: 4.7,
			Tags: []string{"云盘", "同步", "协作"},
		},
		{
			Name: "PostgreSQL", Slug: "postgres", Category: CategoryDatabase,
			Description: "高级开源关系型数据库",
			Image: "postgres", Tag: "16-alpine",
			Ports: []PortDef{{Host: 5432, Container: 5432, Desc: "数据库端口"}},
			Volumes: []VolumeDef{
				{Host: "/data/postgres", Container: "/var/lib/postgresql/data", Desc: "数据"},
			},
			EnvVars: []EnvVarDef{
				{Name: "POSTGRES_PASSWORD", Required: true, Secret: true, Description: "管理员密码"},
				{Name: "POSTGRES_DB", Default: "appdb", Description: "默认数据库"},
			},
			MinCPU: 0.5, MinMemory: 256 * 1024 * 1024, Rating: 4.9,
			Tags: []string{"数据库", "SQL", "关系型"},
		},
		{
			Name: "Redis", Slug: "redis", Category: CategoryDatabase,
			Description: "高性能内存缓存数据库",
			Image: "redis", Tag: "alpine",
			Ports: []PortDef{{Host: 6379, Container: 6379, Desc: "Redis 端口"}},
			MinCPU: 0.5, MinMemory: 128 * 1024 * 1024, Rating: 4.9,
			Tags: []string{"缓存", "数据库", "内存"},
		},
		{
			Name: "Home Assistant", Slug: "homeassistant", Category: CategoryProductivity,
			Description: "开源智能家居平台，支持 3000+ 设备集成",
			Image: "homeassistant/home-assistant", Tag: "stable",
			Ports: []PortDef{{Host: 8123, Container: 8123, Desc: "Web UI"}},
			Volumes: []VolumeDef{
				{Host: "/data/homeassistant", Container: "/config", Desc: "配置"},
			},
			MinCPU: 1, MinMemory: 512 * 1024 * 1024, Featured: true, Rating: 4.8,
			Tags: []string{"智能家居", "自动化", "IoT"},
		},
		{
			Name: "Uptime Kuma", Slug: "uptimekuma", Category: CategoryMonitoring,
			Description: "美观的自托管监控工具",
			Image: "louislam/uptime-kuma", Tag: "1",
			Ports: []PortDef{{Host: 3001, Container: 3001, Desc: "Web UI"}},
			Volumes: []VolumeDef{
				{Host: "/data/uptimekuma", Container: "/app/data", Desc: "数据"},
			},
			MinCPU: 0.5, MinMemory: 128 * 1024 * 1024, Rating: 4.9,
			Tags: []string{"监控", "可用性", "告警"},
		},
		{
			Name: "Grafana", Slug: "grafana", Category: CategoryMonitoring,
			Description: "数据可视化和监控仪表盘",
			Image: "grafana/grafana", Tag: "latest",
			Ports: []PortDef{{Host: 3000, Container: 3000, Desc: "Web UI"}},
			Volumes: []VolumeDef{
				{Host: "/data/grafana", Container: "/var/lib/grafana", Desc: "数据"},
			},
			MinCPU: 0.5, MinMemory: 256 * 1024 * 1024, Rating: 4.8,
			Tags: []string{"监控", "可视化", "仪表盘"},
		},
		{
			Name: "Vaultwarden", Slug: "vaultwarden", Category: CategorySecurity,
			Description: "Bitwarden 兼容密码管理器",
			Image: "vaultwarden/server", Tag: "latest",
			Ports: []PortDef{{Host: 8222, Container: 80, Desc: "Web UI"}},
			Volumes: []VolumeDef{
				{Host: "/data/vaultwarden", Container: "/data", Desc: "数据"},
			},
			MinCPU: 0.5, MinMemory: 128 * 1024 * 1024, Featured: true, Rating: 4.9,
			Tags: []string{"密码", "安全", "加密"},
		},
		{
			Name: "Gitea", Slug: "gitea", Category: CategoryDevOps,
			Description: "轻量级自托管 Git 服务",
			Image: "gitea/gitea", Tag: "latest",
			Ports: []PortDef{{Host: 3000, Container: 3000, Desc: "Web UI"}, {Host: 2222, Container: 22, Desc: "SSH"}},
			Volumes: []VolumeDef{
				{Host: "/data/gitea", Container: "/data", Desc: "数据"},
			},
			MinCPU: 0.5, MinMemory: 256 * 1024 * 1024, Rating: 4.7,
			Tags: []string{"Git", "代码", "CI/CD"},
		},
		{
			Name: "Nginx Proxy Manager", Slug: "nginx-proxy", Category: CategoryNetwork,
			Description: "反向代理管理，自动 HTTPS 证书",
			Image: "jc21/nginx-proxy-manager", Tag: "latest",
			Ports: []PortDef{
				{Host: 80, Container: 80, Desc: "HTTP"},
				{Host: 443, Container: 443, Desc: "HTTPS"},
				{Host: 81, Container: 81, Desc: "管理面板"},
			},
			Volumes: []VolumeDef{
				{Host: "/data/nginx-proxy/data", Container: "/data", Desc: "数据"},
				{Host: "/data/nginx-proxy/letsencrypt", Container: "/etc/letsencrypt", Desc: "证书"},
			},
			MinCPU: 0.5, MinMemory: 128 * 1024 * 1024, Rating: 4.8,
			Tags: []string{"代理", "HTTPS", "反向代理"},
		},
	}

	for _, t := range defaults {
		t.ID = uuid.New().String()
		t.CreatedAt = time.Now()
		t.UpdatedAt = time.Now()
		m.templates[t.Slug] = t
	}

	// 初始化组合模板
	m.stacks["fullstack-web"] = &StackTemplate{
		ID:          uuid.New().String(),
		Name:        "全栈 Web 开发环境",
		Description: "Nginx + PostgreSQL + Redis + Gitea 一站式 Web 开发环境",
		Category:    CategoryDevOps,
		Templates: []StackItem{
			{TemplateSlug: "postgres", Alias: "db"},
			{TemplateSlug: "redis", Alias: "cache"},
			{TemplateSlug: "gitea", Alias: "git", DependsOn: []string{"db"}},
			{TemplateSlug: "nginx-proxy", Alias: "proxy", DependsOn: []string{"git"}},
		},
		Featured: true,
	}

	m.stacks["smart-home"] = &StackTemplate{
		ID:          uuid.New().String(),
		Name:        "智能家居套件",
		Description: "Home Assistant + Grafana + Uptime Kuma 智能家居监控方案",
		Category:    CategoryProductivity,
		Templates: []StackItem{
			{TemplateSlug: "homeassistant", Alias: "hass"},
			{TemplateSlug: "grafana", Alias: "dashboard"},
			{TemplateSlug: "uptimekuma", Alias: "monitor"},
		},
		Featured: true,
	}
}

// ========== 模板管理 ==========

// GetTemplate 获取模板
func (m *Manager) GetTemplate(slug string) (*Template, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.templates[slug]
	if !ok {
		return nil, fmt.Errorf("template %s not found", slug)
	}
	return t, nil
}

// ListTemplates 列出模板
func (m *Manager) ListTemplates(category TemplateCategory, search string) []*Template {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Template
	for _, t := range m.templates {
		if category != "" && t.Category != category {
			continue
		}
		if search != "" {
			searchLower := strings.ToLower(search)
			if !strings.Contains(strings.ToLower(t.Name), searchLower) &&
				!strings.Contains(strings.ToLower(t.Description), searchLower) {
				continue
			}
		}
		result = append(result, t)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Featured != result[j].Featured {
			return result[i].Featured
		}
		return result[i].Rating > result[j].Rating
	})

	return result
}

// ListFeatured 列出推荐模板
func (m *Manager) ListFeatured() []*Template {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var featured []*Template
	for _, t := range m.templates {
		if t.Featured {
			featured = append(featured, t)
		}
	}

	sort.Slice(featured, func(i, j int) bool {
		return featured[i].Rating > featured[j].Rating
	})

	return featured
}

// ========== 部署 ==========

// Deploy 部署模板
func (m *Manager) Deploy(slug string, name string, overrides map[string]string) (*DeployTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.templates[slug]
	if !ok {
		return nil, fmt.Errorf("template %s not found", slug)
	}

	// 检查是否已安装
	if existing, exists := m.installed[slug]; exists && existing.Status == DeployStatusRunning {
		return nil, fmt.Errorf("template %s already deployed", slug)
	}

	task := &DeployTask{
		ID:         uuid.New().String(),
		TemplateID: t.ID,
		Name:       name,
		Status:     DeployStatusPending,
		Overrides:  overrides,
		StartedAt:  time.Now(),
	}

	m.tasks[task.ID] = task
	m.installed[slug] = task

	// 模拟异步部署
	go m.executeDeploy(task, t)

	log.Printf("[容器向导] 开始部署: %s (%s)", name, slug)
	return task, nil
}

// executeDeploy 执行部署
func (m *Manager) executeDeploy(task *DeployTask, tmpl *Template) {
	m.updateTaskStatus(task.ID, DeployStatusPulling, "正在拉取镜像...")
	time.Sleep(100 * time.Millisecond)

	m.updateTaskStatus(task.ID, DeployStatusConfiguring, "正在配置...")
	time.Sleep(100 * time.Millisecond)

	m.updateTaskStatus(task.ID, DeployStatusStarting, "正在启动...")
	time.Sleep(100 * time.Millisecond)

	// 模拟成功
	m.mu.Lock()
	task.Status = DeployStatusRunning
	task.ContainerID = uuid.New().String()[:12]
	task.Logs = append(task.Logs, "容器启动成功")
	now := time.Now()
	task.CompletedAt = &now
	m.mu.Unlock()

	log.Printf("[容器向导] 部署完成: %s, 容器: %s", task.Name, task.ContainerID)
}

func (m *Manager) updateTaskStatus(taskID string, status DeployStatus, logMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task, ok := m.tasks[taskID]; ok {
		task.Status = status
		task.Logs = append(task.Logs, fmt.Sprintf("[%s] %s", status, logMsg))
	}
}

// GetDeployTask 获取部署任务
func (m *Manager) GetDeployTask(id string) (*DeployTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %s not found", id)
	}
	return task, nil
}

// ListDeployTasks 列出部署任务
func (m *Manager) ListDeployTasks() []*DeployTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*DeployTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].StartedAt.After(tasks[j].StartedAt)
	})

	return tasks
}

// GetInstalled 获取已安装列表
func (m *Manager) GetInstalled() []*DeployTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var installed []*DeployTask
	for _, task := range m.installed {
		installed = append(installed, task)
	}
	return installed
}

// ========== 组合模板 ==========

// GetStackTemplate 获取组合模板
func (m *Manager) GetStackTemplate(id string) (*StackTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stack, ok := m.stacks[id]
	if !ok {
		return nil, fmt.Errorf("stack template %s not found", id)
	}
	return stack, nil
}

// ListStackTemplates 列出组合模板
func (m *Manager) ListStackTemplates() []*StackTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stacks := make([]*StackTemplate, 0, len(m.stacks))
	for _, s := range m.stacks {
		stacks = append(stacks, s)
	}
	return stacks
}

// DeployStack 部署组合模板
func (m *Manager) DeployStack(stackID string, overrides map[string]map[string]string) ([]*DeployTask, error) {
	m.mu.RLock()
	stack, ok := m.stacks[stackID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("stack template %s not found", stackID)
	}

	var tasks []*DeployTask
	for _, item := range stack.Templates {
		ovr := overrides[item.Alias]
		task, err := m.Deploy(item.TemplateSlug, item.Alias, ovr)
		if err != nil {
			log.Printf("[容器向导] 部署 %s 失败: %v", item.Alias, err)
			continue
		}
		tasks = append(tasks, task)
	}

	return tasks, nil
}

// ========== 资源推荐 ==========

// GetResourceRecommend 获取资源推荐
func (m *Manager) GetResourceRecommend(slug string) (*ResourceRecommend, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.templates[slug]
	if !ok {
		return nil, fmt.Errorf("template %s not found", slug)
	}

	rec := &ResourceRecommend{
		CPU:    t.MinCPU * 1.5,
		Memory: t.MinMemory * 2,
		Disk:   t.MinDisk,
		Reason: "基于模板最低要求的 1.5-2 倍推荐",
	}

	if rec.Disk == 0 {
		rec.Disk = 10 * 1024 * 1024 * 1024 // 默认 10GB
	}

	return rec, nil
}

// ========== 统计 ==========

// GetStats 获取统计
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	categories := make(map[TemplateCategory]int)
	for _, t := range m.templates {
		categories[t.Category]++
	}

	running := 0
	for _, t := range m.tasks {
		if t.Status == DeployStatusRunning {
			running++
		}
	}

	return map[string]interface{}{
		"totalTemplates":  len(m.templates),
		"totalStacks":     len(m.stacks),
		"totalDeployed":   len(m.installed),
		"runningServices": running,
		"categories":      categories,
	}
}
