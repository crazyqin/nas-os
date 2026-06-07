// Package appwizard 提供智能应用安装向导功能
// 包括一键部署、依赖检测、配置推荐、版本管理
package appwizard

import (
	"errors"
	"sync"
	"time"
)

// ========== 应用状态 ==========

// AppStatus 应用状态
type AppStatus string

const (
	StatusAvailable  AppStatus = "available"  // 可用
	StatusInstalling AppStatus = "installing" // 安装中
	StatusInstalled  AppStatus = "installed"  // 已安装
	StatusUpdating   AppStatus = "updating"   // 更新中
	StatusRemoving   AppStatus = "removing"   // 卸载中
	StatusError      AppStatus = "error"      // 错误
)

// ========== 应用来源 ==========

// AppSource 应用来源
type AppSource string

const (
	SourceOfficial  AppSource = "official"  // 官方仓库
	SourceCommunity AppSource = "community" // 社区仓库
	SourceCustom    AppSource = "custom"    // 自定义仓库
	SourceDockerHub AppSource = "dockerhub" // Docker Hub
)

// ========== 应用分类 ==========

// AppCategory 应用分类
type AppCategory string

const (
	CategoryMedia        AppCategory = "media"        // 媒体
	CategoryDownload     AppCategory = "download"     // 下载
	CategoryProductivity AppCategory = "productivity" // 效率
	CategoryDevelopment  AppCategory = "development"  // 开发
	CategoryNetwork      AppCategory = "network"      // 网络
	CategorySecurity     AppCategory = "security"     // 安全
	CategoryDatabase     AppCategory = "database"     // 数据库
	CategoryStorage      AppCategory = "storage"      // 存储
	CategoryMonitoring   AppCategory = "monitoring"   // 监控
	CategoryOther        AppCategory = "other"        // 其他
)

// ========== 应用元数据 ==========

// AppMeta 应用元数据
type AppMeta struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	DisplayName   string      `json:"display_name"`
	Description   string      `json:"description"`
	Version       string      `json:"version"`
	LatestVersion string      `json:"latest_version"`
	Category      AppCategory `json:"category"`
	Source        AppSource   `json:"source"`
	Icon          string      `json:"icon"`
	Screenshots   []string    `json:"screenshots,omitempty"`
	Author        string      `json:"author"`
	License       string      `json:"license"`
	Website       string      `json:"website,omitempty"`
	Repository    string      `json:"repository,omitempty"`
	Stars         int         `json:"stars"`     // 社区评分
	Downloads     int64       `json:"downloads"` // 下载次数
	Tags          []string    `json:"tags"`
	MinCPU        int         `json:"min_cpu"`       // 最低CPU核心
	MinMemoryMB   int         `json:"min_memory_mb"` // 最低内存(MB)
	MinDiskGB     int         `json:"min_disk_gb"`   // 最低磁盘(GB)
	Ports         []PortMap   `json:"ports"`         // 端口映射
	Volumes       []string    `json:"volumes"`       // 持久化卷
	EnvVars       []EnvVar    `json:"env_vars"`      // 环境变量
	Dependencies  []string    `json:"dependencies"`  // 依赖应用ID
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// PortMap 端口映射
type PortMap struct {
	Name      string `json:"name"`
	Container int    `json:"container"`
	Host      int    `json:"host"`
	Protocol  string `json:"protocol"` // tcp, udp
}

// EnvVar 环境变量
type EnvVar struct {
	Name        string   `json:"name"`
	Default     string   `json:"default"`
	Description string   `json:"description"`
	Required    bool     `json:"required"`
	Secret      bool     `json:"secret"`
	Type        string   `json:"type"`              // string, number, boolean, password, select
	Options     []string `json:"options,omitempty"` // type=select 时可选值
}

// ========== 安装任务 ==========

// InstallTask 安装任务
type InstallTask struct {
	ID         string            `json:"id"`
	AppID      string            `json:"app_id"`
	Version    string            `json:"version"`
	Status     AppStatus         `json:"status"`
	Progress   int               `json:"progress"` // 0-100
	Step       string            `json:"step"`     // 当前步骤描述
	Config     map[string]string `json:"config"`
	Error      string            `json:"error,omitempty"`
	StartedAt  time.Time         `json:"started_at"`
	FinishedAt *time.Time        `json:"finished_at,omitempty"`
}

// ========== 应用实例 ==========

// AppInstance 应用实例
type AppInstance struct {
	ID          string            `json:"id"`
	AppID       string            `json:"app_id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Status      AppStatus         `json:"status"`
	Port        int               `json:"port"`
	URL         string            `json:"url"`
	Config      map[string]string `json:"config"`
	HealthCheck string            `json:"health_check"`
	InstalledAt time.Time         `json:"installed_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	CPUUsage    float64           `json:"cpu_usage"`
	MemUsageMB  float64           `json:"mem_usage_mb"`
	DiskUsageGB float64           `json:"disk_usage_gb"`
}

// ========== 配置推荐 ==========

// ConfigRecommendation 配置推荐
type ConfigRecommendation struct {
	AppID       string            `json:"app_id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Config      map[string]string `json:"config"`
	Score       float64           `json:"score"` // 推荐分数
	Reason      string            `json:"reason"`
	Template    string            `json:"template"` // 推荐模板名
}

// ========== 应用向导引擎 ==========

// AppWizardEngine 应用向导引擎
type AppWizardEngine struct {
	mu          sync.RWMutex
	catalog     map[string]*AppMeta
	instances   map[string]*AppInstance
	tasks       map[string]*InstallTask
	repos       []AppRepo
	suggestions []ConfigRecommendation
}

// AppRepo 应用仓库
type AppRepo struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	URL      string    `json:"url"`
	Source   AppSource `json:"source"`
	Enabled  bool      `json:"enabled"`
	Priority int       `json:"priority"`
	LastSync time.Time `json:"last_sync"`
}

// EngineOption 引擎配置选项
type EngineOption func(*AppWizardEngine)

// NewAppWizardEngine 创建应用向导引擎
func NewAppWizardEngine(opts ...EngineOption) *AppWizardEngine {
	e := &AppWizardEngine{
		catalog:   make(map[string]*AppMeta),
		instances: make(map[string]*AppInstance),
		tasks:     make(map[string]*InstallTask),
	}
	for _, opt := range opts {
		opt(e)
	}
	e.initBuiltInCatalog()
	return e
}

// ========== 应用目录 ==========

// GetCatalog 获取应用目录
func (e *AppWizardEngine) GetCatalog() []*AppMeta {
	e.mu.RLock()
	defer e.mu.RUnlock()
	apps := make([]*AppMeta, 0, len(e.catalog))
	for _, app := range e.catalog {
		apps = append(apps, app)
	}
	return apps
}

// GetApp 获取应用详情
func (e *AppWizardEngine) GetApp(appID string) (*AppMeta, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	app, ok := e.catalog[appID]
	if !ok {
		return nil, errors.New("app not found")
	}
	return app, nil
}

// SearchApps 搜索应用
func (e *AppWizardEngine) SearchApps(query string, category AppCategory) []*AppMeta {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var results []*AppMeta
	for _, app := range e.catalog {
		if category != "" && app.Category != category {
			continue
		}
		if query != "" && !containsIgnoreCase(app.Name, query) && !containsIgnoreCase(app.Description, query) {
			continue
		}
		results = append(results, app)
	}
	return results
}

// ListByCategory 按分类列出应用
func (e *AppWizardEngine) ListByCategory(category AppCategory) []*AppMeta {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var apps []*AppMeta
	for _, app := range e.catalog {
		if app.Category == category {
			apps = append(apps, app)
		}
	}
	return apps
}

// ========== 安装管理 ==========

// Install 安装应用
func (e *AppWizardEngine) Install(appID string, config map[string]string) (*InstallTask, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	app, ok := e.catalog[appID]
	if !ok {
		return nil, errors.New("app not found")
	}

	// 检查依赖
	for _, dep := range app.Dependencies {
		if inst, ok := e.instances[dep]; !ok || inst.Status != StatusInstalled {
			return nil, errors.New("dependency not installed: " + dep)
		}
	}

	// 检查端口冲突
	for _, port := range app.Ports {
		for _, inst := range e.instances {
			if inst.Port == port.Host && inst.Status == StatusInstalled {
				return nil, errors.New("port conflict")
			}
		}
	}

	task := &InstallTask{
		ID:        generateID(),
		AppID:     appID,
		Version:   app.LatestVersion,
		Status:    StatusInstalling,
		Progress:  0,
		Step:      "准备安装",
		Config:    config,
		StartedAt: time.Now(),
	}

	e.tasks[task.ID] = task
	go e.installWorker(task)
	return task, nil
}

// Uninstall 卸载应用
func (e *AppWizardEngine) Uninstall(instanceID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	inst, ok := e.instances[instanceID]
	if !ok {
		return errors.New("instance not found")
	}

	// 检查是否被其他应用依赖
	for _, other := range e.instances {
		app := e.catalog[other.AppID]
		if app != nil {
			for _, dep := range app.Dependencies {
				if dep == inst.AppID {
					return errors.New("cannot uninstall: other app depends on this")
				}
			}
		}
	}

	inst.Status = StatusRemoving
	go func() {
		time.Sleep(2 * time.Second)
		e.mu.Lock()
		delete(e.instances, instanceID)
		e.mu.Unlock()
	}()

	return nil
}

// GetInstallTask 获取安装任务状态
func (e *AppWizardEngine) GetInstallTask(taskID string) (*InstallTask, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	task, ok := e.tasks[taskID]
	if !ok {
		return nil, errors.New("task not found")
	}
	return task, nil
}

// ========== 实例管理 ==========

// ListInstances 列出已安装应用
func (e *AppWizardEngine) ListInstances() []*AppInstance {
	e.mu.RLock()
	defer e.mu.RUnlock()
	instances := make([]*AppInstance, 0, len(e.instances))
	for _, inst := range e.instances {
		instances = append(instances, inst)
	}
	return instances
}

// GetInstance 获取实例详情
func (e *AppWizardEngine) GetInstance(instanceID string) (*AppInstance, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	inst, ok := e.instances[instanceID]
	if !ok {
		return nil, errors.New("instance not found")
	}
	return inst, nil
}

// ========== 配置推荐 ==========

// RecommendConfig 推荐配置
func (e *AppWizardEngine) RecommendConfig(appID string) []ConfigRecommendation {
	e.mu.RLock()
	defer e.mu.RUnlock()

	app, ok := e.catalog[appID]
	if !ok {
		return nil
	}

	var recs []ConfigRecommendation

	// 默认配置推荐
	defaultConfig := make(map[string]string)
	for _, env := range app.EnvVars {
		if env.Default != "" {
			defaultConfig[env.Name] = env.Default
		}
	}
	recs = append(recs, ConfigRecommendation{
		AppID:       appID,
		Name:        "默认配置",
		Description: "适合大多数场景的默认配置",
		Config:      defaultConfig,
		Score:       0.8,
		Reason:      "使用应用推荐的默认参数",
		Template:    "default",
	})

	return recs
}

// ========== 仓库管理 ==========

// AddRepo 添加应用仓库
func (e *AppWizardEngine) AddRepo(repo AppRepo) error {
	if repo.ID == "" {
		return errors.New("repo ID cannot be empty")
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.repos = append(e.repos, repo)
	return nil
}

// ListRepos 列出仓库
func (e *AppWizardEngine) ListRepos() []AppRepo {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.repos
}

// ========== 内部方法 ==========

func (e *AppWizardEngine) installWorker(task *InstallTask) {
	steps := []struct {
		name     string
		progress int
	}{
		{"检查环境", 10},
		{"拉取镜像", 30},
		{"配置网络", 50},
		{"创建容器", 70},
		{"初始化服务", 85},
		{"健康检查", 95},
		{"安装完成", 100},
	}

	for _, step := range steps {
		e.mu.Lock()
		task.Step = step.name
		task.Progress = step.progress
		e.mu.Unlock()
		time.Sleep(500 * time.Millisecond)
	}

	e.mu.Lock()
	task.Status = StatusInstalled
	now := time.Now()
	task.FinishedAt = &now

	app := e.catalog[task.AppID]
	port := 8080
	if app != nil && len(app.Ports) > 0 {
		port = app.Ports[0].Host
	}

	e.instances[task.ID] = &AppInstance{
		ID:          task.ID,
		AppID:       task.AppID,
		Name:        app.Name,
		Version:     task.Version,
		Status:      StatusInstalled,
		Port:        port,
		URL:         "http://localhost",
		Config:      task.Config,
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}
	e.mu.Unlock()
}

func (e *AppWizardEngine) initBuiltInCatalog() {
	builtinApps := []*AppMeta{
		{
			ID: "jellyfin", Name: "jellyfin", DisplayName: "Jellyfin",
			Description: "免费开源媒体服务器", Category: CategoryMedia, Source: SourceOfficial,
			Version: "10.9.0", LatestVersion: "10.9.0", Stars: 45000,
			Ports:  []PortMap{{Name: "web", Container: 8096, Host: 8096, Protocol: "tcp"}},
			MinCPU: 2, MinMemoryMB: 2048, MinDiskGB: 10,
			Tags: []string{"media", "video", "music"},
		},
		{
			ID: "nextcloud", Name: "nextcloud", DisplayName: "Nextcloud",
			Description: "私有云文件同步与共享", Category: CategoryProductivity, Source: SourceOfficial,
			Version: "29.0.0", LatestVersion: "29.0.0", Stars: 28000,
			Ports:  []PortMap{{Name: "web", Container: 80, Host: 8080, Protocol: "tcp"}},
			MinCPU: 2, MinMemoryMB: 2048, MinDiskGB: 20,
			Tags: []string{"cloud", "files", "sync"},
		},
		{
			ID: "homeassistant", Name: "homeassistant", DisplayName: "Home Assistant",
			Description: "智能家居自动化平台", Category: CategoryOther, Source: SourceOfficial,
			Version: "2026.6.0", LatestVersion: "2026.6.0", Stars: 72000,
			Ports:  []PortMap{{Name: "web", Container: 8123, Host: 8123, Protocol: "tcp"}},
			MinCPU: 1, MinMemoryMB: 1024, MinDiskGB: 5,
			Tags: []string{"iot", "smart-home", "automation"},
		},
		{
			ID: "traefik", Name: "traefik", DisplayName: "Traefik",
			Description: "云原生反向代理和负载均衡", Category: CategoryNetwork, Source: SourceOfficial,
			Version: "v3.0", LatestVersion: "v3.0", Stars: 52000,
			Ports:  []PortMap{{Name: "web", Container: 80, Host: 80, Protocol: "tcp"}, {Name: "api", Container: 8080, Host: 8443, Protocol: "tcp"}},
			MinCPU: 1, MinMemoryMB: 512, MinDiskGB: 1,
			Tags: []string{"proxy", "reverse-proxy", "load-balancer"},
		},
		{
			ID: "prometheus", Name: "prometheus", DisplayName: "Prometheus",
			Description: "监控系统和时间序列数据库", Category: CategoryMonitoring, Source: SourceOfficial,
			Version: "v2.52", LatestVersion: "v2.52", Stars: 56000,
			Ports:  []PortMap{{Name: "web", Container: 9090, Host: 9090, Protocol: "tcp"}},
			MinCPU: 2, MinMemoryMB: 2048, MinDiskGB: 20,
			Tags: []string{"monitoring", "metrics", "database"},
		},
		{
			ID: "vaultwarden", Name: "vaultwarden", DisplayName: "Vaultwarden",
			Description: "轻量级密码管理器（Bitwarden 兼容）", Category: CategorySecurity, Source: SourceOfficial,
			Version: "1.31.0", LatestVersion: "1.31.0", Stars: 35000,
			Ports:  []PortMap{{Name: "web", Container: 80, Host: 8880, Protocol: "tcp"}},
			MinCPU: 1, MinMemoryMB: 256, MinDiskGB: 1,
			Tags: []string{"password", "security", "vault"},
		},
	}

	for _, app := range builtinApps {
		app.CreatedAt = time.Now()
		app.UpdatedAt = time.Now()
		e.catalog[app.ID] = app
	}
}

func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			tc := substr[j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			if tc >= 'A' && tc <= 'Z' {
				tc += 32
			}
			if sc != tc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-app"
}
