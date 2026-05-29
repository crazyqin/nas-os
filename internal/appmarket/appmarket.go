// Package appmarket 应用市场 - 一键部署应用
// 对标TrueCharts应用商店
package appmarket

import (
	"errors"
	"sync"
	"time"
)

// AppCategory 应用分类
type AppCategory string

const (
	CategoryMedia      AppCategory = "media"
	CategoryDownload   AppCategory = "download"
	CategoryNetwork    AppCategory = "network"
	CategoryDatabase   AppCategory = "database"
	CategoryDevOps     AppCategory = "devops"
	CategoryProductivity AppCategory = "productivity"
	CategorySecurity   AppCategory = "security"
	CategoryStorage    AppCategory = "storage"
	CategoryUtility    AppCategory = "utility"
	CategoryOther      AppCategory = "other"
)

// AppStatus 应用状态
type AppStatus string

const (
	AppStatusAvailable AppStatus = "available"
	AppStatusInstalling AppStatus = "installing"
	AppStatusInstalled  AppStatus = "installed"
	AppStatusUpdating   AppStatus = "updating"
	AppStatusError      AppStatus = "error"
	AppStatusDisabled   AppStatus = "disabled"
)

// App 应用定义
type App struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Category    AppCategory `json:"category"`
	Version     string      `json:"version"`
	LatestVersion string   `json:"latest_version"`
	Author      string      `json:"author"`
	Website     string      `json:"website,omitempty"`
	Icon        string      `json:"icon,omitempty"`
	Screenshot  string      `json:"screenshot,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	
	// 部署配置
	Image       string            `json:"image"`
	ImageTag    string            `json:"image_tag"`
	Ports       []PortMapping     `json:"ports,omitempty"`
	Volumes     []VolumeMount     `json:"volumes,omitempty"`
	EnvVars     []EnvVariable     `json:"env_vars,omitempty"`
	Requirements AppRequirements  `json:"requirements"`
	
	// 状态
	Status      AppStatus   `json:"status"`
	InstalledAt *time.Time  `json:"installed_at,omitempty"`
	UpdatedAt   time.Time   `json:"updated_at"`
	CreatedAt   time.Time   `json:"created_at"`
	
	// 统计
	Downloads   int       `json:"downloads"`
	Rating      float64   `json:"rating"`
	Reviews     int       `json:"reviews"`
}

// PortMapping 端口映射
type PortMapping struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port,omitempty"`
	Protocol      string `json:"protocol"` // tcp, udp
}

// VolumeMount 卷挂载
type VolumeMount struct {
	Name          string `json:"name"`
	ContainerPath string `json:"container_path"`
	HostPath      string `json:"host_path,omitempty"`
	ReadOnly      bool   `json:"read_only,omitempty"`
	Type          string `json:"type"` // hostPath, pvc, emptyDir
}

// EnvVariable 环境变量
type EnvVariable struct {
	Name        string `json:"name"`
	Value       string `json:"value,omitempty"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
}

// AppRequirements 应用需求
type AppRequirements struct {
	MinCPU       float64 `json:"min_cpu,omitempty"`       // 核数
	MinMemory    int64   `json:"min_memory,omitempty"`    // MB
	MinDisk      int64   `json:"min_disk,omitempty"`      // MB
	GPU          bool    `json:"gpu,omitempty"`
	Privileged   bool    `json:"privileged,omitempty"`
	NetworkMode  string  `json:"network_mode,omitempty"` // bridge, host, none
}

// InstalledApp 已安装应用
type InstalledApp struct {
	AppID       string            `json:"app_id"`
	Name        string            `json:"name"`
	ContainerID string            `json:"container_id,omitempty"`
	Status      AppStatus         `json:"status"`
	Version     string            `json:"version"`
	Config      map[string]string `json:"config,omitempty"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	StoppedAt   *time.Time        `json:"stopped_at,omitempty"`
	ErrorMsg     string            `json:"error_msg,omitempty"`
	ResourceUsage ResourceUsage   `json:"resource_usage"`
}

// ResourceUsage 资源使用
type ResourceUsage struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   int64   `json:"memory_usage"`   // bytes
	MemoryLimit   int64   `json:"memory_limit"`   // bytes
	DiskUsage     int64   `json:"disk_usage"`     // bytes
	NetworkRx     int64   `json:"network_rx"`     // bytes
	NetworkTx     int64   `json:"network_tx"`     // bytes
}

// AppMarket 应用市场
type AppMarket struct {
	mu           sync.RWMutex
	apps         map[string]*App
	installed    map[string]*InstalledApp
	categories   []AppCategory
	config       *AppMarketConfig
}

// AppMarketConfig 应用市场配置
type AppMarketConfig struct {
	RegistryURL    string   `json:"registry_url"`
	AutoUpdate     bool     `json:"auto_update"`
	UpdateInterval int      `json:"update_interval"` // 小时
	MaxConcurrent  int      `json:"max_concurrent"`
	DataDir        string   `json:"data_dir"`
	AllowedRegistries []string `json:"allowed_registries"`
}

// DefaultAppMarketConfig 默认配置
func DefaultAppMarketConfig() *AppMarketConfig {
	return &AppMarketConfig{
		RegistryURL:    "https://ghcr.io",
		AutoUpdate:     true,
		UpdateInterval: 24,
		MaxConcurrent:  3,
		DataDir:        "/var/lib/nas-os/apps",
	}
}

// NewAppMarket 创建应用市场
func NewAppMarket(config *AppMarketConfig) *AppMarket {
	if config == nil {
		config = DefaultAppMarketConfig()
	}

	return &AppMarket{
		apps:       make(map[string]*App),
		installed:  make(map[string]*InstalledApp),
		categories: []AppCategory{
			CategoryMedia, CategoryDownload, CategoryNetwork,
			CategoryDatabase, CategoryDevOps, CategoryProductivity,
			CategorySecurity, CategoryStorage, CategoryUtility,
		},
		config: config,
	}
}

// RegisterApp 注册应用
func (m *AppMarket) RegisterApp(app *App) error {
	if app == nil {
		return errors.New("app is nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	if _, exists := m.apps[app.ID]; exists {
		return errors.New("app already exists: " + app.ID)
	}

	// 设置默认值
	now := time.Now()
	if app.CreatedAt.IsZero() {
		app.CreatedAt = now
	}
	app.UpdatedAt = now
	app.Status = AppStatusAvailable

	if app.LatestVersion == "" {
		app.LatestVersion = app.Version
	}

	m.apps[app.ID] = app

	return nil
}

// GetApp 获取应用
func (m *AppMarket) GetApp(id string) (*App, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, exists := m.apps[id]
	return app, exists
}

// UpdateApp 更新应用
func (m *AppMarket) UpdateApp(id string, update func(*App)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.apps[id]
	if !exists {
		return errors.New("app not found: " + id)
	}

	update(app)
	app.UpdatedAt = time.Now()

	return nil
}

// UnregisterApp 注销应用
func (m *AppMarket) UnregisterApp(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.apps[id]; !exists {
		return errors.New("app not found: " + id)
	}

	// 检查是否已安装
	if _, installed := m.installed[id]; installed {
		return errors.New("app is installed, uninstall first")
	}

	delete(m.apps, id)
	return nil
}

// ListApps 列出应用
func (m *AppMarket) ListApps(category *AppCategory) []*App {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]*App, 0)

	for _, app := range m.apps {
		if category != nil && app.Category != *category {
			continue
		}
		apps = append(apps, app)
	}

	return apps
}

// SearchApps 搜索应用
func (m *AppMarket) SearchApps(query string) []*App {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]*App, 0)

	for _, app := range m.apps {
		if matchesSearch(app, query) {
			apps = append(apps, app)
		}
	}

	return apps
}

// matchesSearch 检查是否匹配搜索
func matchesSearch(app *App, query string) bool {
	if query == "" {
		return true
	}

	// 检查名称、标题、描述、标签
	if containsIgnoreCase(app.Name, query) ||
		containsIgnoreCase(app.Title, query) ||
		containsIgnoreCase(app.Description, query) {
		return true
	}

	for _, tag := range app.Tags {
		if containsIgnoreCase(tag, query) {
			return true
		}
	}

	return false
}

// containsIgnoreCase 忽略大小写包含检查
func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) == 0 {
		return false
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 32
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 32
			}
			if c1 != c2 {
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

// InstallApp 安装应用
func (m *AppMarket) InstallApp(appID string, config map[string]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.apps[appID]
	if !exists {
		return errors.New("app not found: " + appID)
	}

	// 检查是否已安装
	if _, installed := m.installed[appID]; installed {
		return errors.New("app already installed")
	}

	// 创建已安装记录
	now := time.Now()
	installed := &InstalledApp{
		AppID:     appID,
		Name:      app.Name,
		Status:    AppStatusInstalling,
		Version:   app.Version,
		Config:    config,
		StartedAt: &now,
	}

	m.installed[appID] = installed
	app.Status = AppStatusInstalling
	app.InstalledAt = &now
	app.UpdatedAt = now

	return nil
}

// UninstallApp 卸载应用
func (m *AppMarket) UninstallApp(appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.installed[appID]; !exists {
		return errors.New("app not installed: " + appID)
	}

	app, exists := m.apps[appID]
	if exists {
		app.Status = AppStatusAvailable
		app.InstalledAt = nil
		app.UpdatedAt = time.Now()
	}

	delete(m.installed, appID)

	return nil
}

// GetInstalledApp 获取已安装应用
func (m *AppMarket) GetInstalledApp(appID string) (*InstalledApp, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, exists := m.installed[appID]
	return app, exists
}

// ListInstalledApps 列出已安装应用
func (m *AppMarket) ListInstalledApps() []*InstalledApp {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]*InstalledApp, 0, len(m.installed))
	for _, app := range m.installed {
		apps = append(apps, app)
	}

	return apps
}

// StartApp 启动应用
func (m *AppMarket) StartApp(appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, exists := m.installed[appID]
	if !exists {
		return errors.New("app not installed: " + appID)
	}

	installed.Status = AppStatusInstalled
	now := time.Now()
	installed.StartedAt = &now
	installed.StoppedAt = nil
	installed.ErrorMsg = ""

	app, exists := m.apps[appID]
	if exists {
		app.Status = AppStatusInstalled
	}

	return nil
}

// StopApp 停止应用
func (m *AppMarket) StopApp(appID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	installed, exists := m.installed[appID]
	if !exists {
		return errors.New("app not installed: " + appID)
	}

	installed.Status = AppStatusDisabled
	now := time.Now()
	installed.StoppedAt = &now

	app, exists := m.apps[appID]
	if exists {
		app.Status = AppStatusDisabled
	}

	return nil
}

// GetCategories 获取分类列表
func (m *AppMarket) GetCategories() []AppCategory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.categories
}

// GetStats 获取统计信息
func (m *AppMarket) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_apps":      len(m.apps),
		"installed_apps":  len(m.installed),
		"by_category":     make(map[AppCategory]int),
		"by_status":       make(map[AppStatus]int),
	}

	byCategory := stats["by_category"].(map[AppCategory]int)
	byStatus := stats["by_status"].(map[AppStatus]int)

	for _, app := range m.apps {
		byCategory[app.Category]++
		byStatus[app.Status]++
	}

	return stats
}

// GetTopApps 获取热门应用
func (m *AppMarket) GetTopApps(limit int) []*App {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]*App, 0, len(m.apps))
	for _, app := range m.apps {
		apps = append(apps, app)
	}

	// 按下载量排序
	for i := 0; i < len(apps); i++ {
		for j := i + 1; j < len(apps); j++ {
			if apps[j].Downloads > apps[i].Downloads {
				apps[i], apps[j] = apps[j], apps[i]
			}
		}
	}

	if limit > 0 && limit < len(apps) {
		apps = apps[:limit]
	}

	return apps
}

// GetRecentApps 获取最新应用
func (m *AppMarket) GetRecentApps(limit int) []*App {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]*App, 0, len(m.apps))
	for _, app := range m.apps {
		apps = append(apps, app)
	}

	// 按创建时间排序
	for i := 0; i < len(apps); i++ {
		for j := i + 1; j < len(apps); j++ {
			if apps[j].CreatedAt.After(apps[i].CreatedAt) {
				apps[i], apps[j] = apps[j], apps[i]
			}
		}
	}

	if limit > 0 && limit < len(apps) {
		apps = apps[:limit]
	}

	return apps
}
