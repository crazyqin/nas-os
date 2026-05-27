// Package appcenter - 应用商店模块
// 对标群晖 Package Center / 飞牛应用市场，支持应用安装、更新、配置、依赖管理
package appcenter

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// App 应用信息
type App struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	Category    string            `json:"category"` // media, tools, development, security, network, storage
	Icon        string            `json:"icon"`
	Homepage    string            `json:"homepage"`
	License     string            `json:"license"`
	Size        int64             `json:"size_bytes"`
	MinVersion  string            `json:"min_version"` // 最低系统版本
	Dependencies []string         `json:"dependencies,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Rating      float64           `json:"rating"`     // 0-5
	Downloads   int64             `json:"downloads"`
	Installed   bool              `json:"installed"`
	Enabled     bool              `json:"enabled"`
	Config      map[string]string `json:"config,omitempty"`
	Ports       []PortMapping     `json:"ports,omitempty"`
	Volumes     []VolumeMount     `json:"volumes,omitempty"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Status      string            `json:"status"` // available, installed, running, stopped, error
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"` // tcp, udp
}

// VolumeMount 卷挂载
type VolumeMount struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
}

// AppCategory 应用分类
type AppCategory struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Icon        string `json:"icon"`
	AppCount    int    `json:"app_count"`
}

// AppReview 应用评价
type AppReview struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Rating    int       `json:"rating"` // 1-5
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// InstallLog 安装日志
type InstallLog struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	Action    string    `json:"action"` // install, update, uninstall, enable, disable
	Status    string    `json:"status"` // success, failed, in_progress
	Message   string    `json:"message"`
	Duration  int       `json:"duration_sec"`
	CreatedAt time.Time `json:"created_at"`
}

// AppStore 应用商店管理器
type AppStore struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	storagePath string

	apps       map[string]*App
	categories map[string]*AppCategory
	reviews    map[string][]*AppReview // appID -> reviews
	logs       []*InstallLog

	// 仓库配置
	repoURL   string
	repoToken string
}

// NewAppStore 创建应用商店
func NewAppStore(logger *zap.Logger, storagePath string) *AppStore {
	store := &AppStore{
		logger:      logger,
		storagePath: storagePath,
		apps:        make(map[string]*App),
		categories:  make(map[string]*AppCategory),
		reviews:     make(map[string][]*AppReview),
		repoURL:     "https://repo.nas-os.com/apps",
	}
	store.initCategories()
	return store
}

// initCategories 初始化默认分类
func (as *AppStore) initCategories() {
	categories := []AppCategory{
		{ID: "media", Name: "影音媒体", Description: "视频、音乐、照片管理", Icon: "🎬"},
		{ID: "tools", Name: "实用工具", Description: "系统工具和实用程序", Icon: "🔧"},
		{ID: "development", Name: "开发工具", Description: "编程和开发环境", Icon: "💻"},
		{ID: "security", Name: "安全防护", Description: "杀毒、防火墙、加密", Icon: "🛡️"},
		{ID: "network", Name: "网络服务", Description: "VPN、代理、DNS", Icon: "🌐"},
		{ID: "storage", Name: "存储管理", Description: "备份、同步、云存储", Icon: "💾"},
		{ID: "productivity", Name: "办公效率", Description: "文档、协作、项目管理", Icon: "📊"},
		{ID: "home", Name: "智能家居", Description: "IoT、自动化、监控", Icon: "🏠"},
	}
	for i := range categories {
		as.categories[categories[i].ID] = &categories[i]
	}
}

// RegisterApp 注册应用
func (as *AppStore) RegisterApp(ctx context.Context, app *App) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if app.ID == "" {
		app.ID = fmt.Sprintf("app-%d", time.Now().UnixNano())
	}
	if _, exists := as.apps[app.ID]; exists {
		return fmt.Errorf("app %s already exists", app.ID)
	}

	app.Status = "available"
	app.CreatedAt = time.Now()
	app.UpdatedAt = time.Now()
	as.apps[app.ID] = app

	// 更新分类计数
	if cat, ok := as.categories[app.Category]; ok {
		cat.AppCount++
	}

	as.logger.Info("应用已注册", zap.String("id", app.ID), zap.String("name", app.Name))
	return nil
}

// UpdateApp 更新应用信息
func (as *AppStore) UpdateApp(ctx context.Context, app *App) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	existing, exists := as.apps[app.ID]
	if !exists {
		return fmt.Errorf("app %s not found", app.ID)
	}

	app.CreatedAt = existing.CreatedAt
	app.UpdatedAt = time.Now()
	as.apps[app.ID] = app
	return nil
}

// RemoveApp 移除应用
func (as *AppStore) RemoveApp(ctx context.Context, appID string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	app, exists := as.apps[appID]
	if !exists {
		return fmt.Errorf("app %s not found", appID)
	}

	if app.Status == "running" {
		return fmt.Errorf("app %s is running, stop it first", appID)
	}

	if cat, ok := as.categories[app.Category]; ok {
		cat.AppCount--
	}

	delete(as.apps, appID)
	delete(as.reviews, appID)

	as.logger.Info("应用已移除", zap.String("id", appID))
	return nil
}

// GetApp 获取应用信息
func (as *AppStore) GetApp(ctx context.Context, appID string) (*App, error) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	app, exists := as.apps[appID]
	if !exists {
		return nil, fmt.Errorf("app %s not found", appID)
	}
	return app, nil
}

// ListApps 列出所有应用
func (as *AppStore) ListApps(ctx context.Context, category string, installedOnly bool) []*App {
	as.mu.RLock()
	defer as.mu.RUnlock()

	var apps []*App
	for _, app := range as.apps {
		if category != "" && app.Category != category {
			continue
		}
		if installedOnly && !app.Installed {
			continue
		}
		apps = append(apps, app)
	}
	return apps
}

// InstallApp 安装应用
func (as *AppStore) InstallApp(ctx context.Context, appID string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	app, exists := as.apps[appID]
	if !exists {
		return fmt.Errorf("app %s not found", appID)
	}

	if app.Installed {
		return fmt.Errorf("app %s already installed", appID)
	}

	// 检查依赖
	for _, dep := range app.Dependencies {
		depApp, ok := as.apps[dep]
		if !ok || !depApp.Installed {
			return fmt.Errorf("dependency %s not installed", dep)
		}
	}

	app.Installed = true
	app.Enabled = true
	app.Status = "installed"
	app.UpdatedAt = time.Now()

	log := &InstallLog{
		ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
		AppID:     appID,
		Action:    "install",
		Status:    "success",
		Message:   fmt.Sprintf("应用 %s 安装成功", app.Name),
		CreatedAt: time.Now(),
	}
	as.logs = append(as.logs, log)
	app.Downloads++

	as.logger.Info("应用已安装", zap.String("id", appID), zap.String("name", app.Name))
	return nil
}

// UninstallApp 卸载应用
func (as *AppStore) UninstallApp(ctx context.Context, appID string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	app, exists := as.apps[appID]
	if !exists {
		return fmt.Errorf("app %s not found", appID)
	}

	if app.Status == "running" {
		return fmt.Errorf("app %s is running, stop it first", appID)
	}

	// 检查是否被其他应用依赖
	for _, other := range as.apps {
		for _, dep := range other.Dependencies {
			if dep == appID && other.Installed {
				return fmt.Errorf("app %s is required by %s", appID, other.Name)
			}
		}
	}

	app.Installed = false
	app.Enabled = false
	app.Status = "available"
	app.UpdatedAt = time.Now()

	log := &InstallLog{
		ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
		AppID:     appID,
		Action:    "uninstall",
		Status:    "success",
		Message:   fmt.Sprintf("应用 %s 已卸载", app.Name),
		CreatedAt: time.Now(),
	}
	as.logs = append(as.logs, log)

	as.logger.Info("应用已卸载", zap.String("id", appID))
	return nil
}

// StartApp 启动应用
func (as *AppStore) StartApp(ctx context.Context, appID string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	app, exists := as.apps[appID]
	if !exists {
		return fmt.Errorf("app %s not found", appID)
	}

	if !app.Installed {
		return fmt.Errorf("app %s not installed", appID)
	}

	app.Status = "running"
	app.UpdatedAt = time.Now()

	as.logger.Info("应用已启动", zap.String("id", appID))
	return nil
}

// StopApp 停止应用
func (as *AppStore) StopApp(ctx context.Context, appID string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	app, exists := as.apps[appID]
	if !exists {
		return fmt.Errorf("app %s not found", appID)
	}

	app.Status = "installed"
	app.UpdatedAt = time.Now()

	as.logger.Info("应用已停止", zap.String("id", appID))
	return nil
}

// EnableApp 启用应用
func (as *AppStore) EnableApp(ctx context.Context, appID string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	app, exists := as.apps[appID]
	if !exists {
		return fmt.Errorf("app %s not found", appID)
	}

	app.Enabled = true
	app.UpdatedAt = time.Now()
	return nil
}

// DisableApp 禁用应用
func (as *AppStore) DisableApp(ctx context.Context, appID string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	app, exists := as.apps[appID]
	if !exists {
		return fmt.Errorf("app %s not found", appID)
	}

	app.Enabled = false
	app.Status = "stopped"
	app.UpdatedAt = time.Now()
	return nil
}

// UpdateAppVersion 更新应用版本
func (as *AppStore) UpdateAppVersion(ctx context.Context, appID, newVersion string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	app, exists := as.apps[appID]
	if !exists {
		return fmt.Errorf("app %s not found", appID)
	}

	oldVersion := app.Version
	app.Version = newVersion
	app.UpdatedAt = time.Now()

	log := &InstallLog{
		ID:        fmt.Sprintf("log-%d", time.Now().UnixNano()),
		AppID:     appID,
		Action:    "update",
		Status:    "success",
		Message:   fmt.Sprintf("应用 %s 从 %s 更新到 %s", app.Name, oldVersion, newVersion),
		CreatedAt: time.Now(),
	}
	as.logs = append(as.logs, log)

	as.logger.Info("应用已更新", zap.String("id", appID), zap.String("version", newVersion))
	return nil
}

// SetAppConfig 设置应用配置
func (as *AppStore) SetAppConfig(ctx context.Context, appID string, config map[string]string) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	app, exists := as.apps[appID]
	if !exists {
		return fmt.Errorf("app %s not found", appID)
	}

	if app.Config == nil {
		app.Config = make(map[string]string)
	}
	for k, v := range config {
		app.Config[k] = v
	}
	app.UpdatedAt = time.Now()
	return nil
}

// GetCategories 获取应用分类
func (as *AppStore) GetCategories(ctx context.Context) []*AppCategory {
	as.mu.RLock()
	defer as.mu.RUnlock()

	categories := make([]*AppCategory, 0, len(as.categories))
	for _, cat := range as.categories {
		categories = append(categories, cat)
	}
	return categories
}

// AddReview 添加评价
func (as *AppStore) AddReview(ctx context.Context, review *AppReview) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if _, exists := as.apps[review.AppID]; !exists {
		return fmt.Errorf("app %s not found", review.AppID)
	}

	if review.ID == "" {
		review.ID = fmt.Sprintf("review-%d", time.Now().UnixNano())
	}
	review.CreatedAt = time.Now()

	as.reviews[review.AppID] = append(as.reviews[review.AppID], review)

	// 更新评分
	reviews := as.reviews[review.AppID]
	total := 0
	for _, r := range reviews {
		total += r.Rating
	}
	as.apps[review.AppID].Rating = float64(total) / float64(len(reviews))

	return nil
}

// GetReviews 获取应用评价
func (as *AppStore) GetReviews(ctx context.Context, appID string) []*AppReview {
	as.mu.RLock()
	defer as.mu.RUnlock()

	return as.reviews[appID]
}

// GetInstallLogs 获取安装日志
func (as *AppStore) GetInstallLogs(ctx context.Context, appID string) []*InstallLog {
	as.mu.RLock()
	defer as.mu.RUnlock()

	if appID == "" {
		return as.logs
	}

	var logs []*InstallLog
	for _, log := range as.logs {
		if log.AppID == appID {
			logs = append(logs, log)
		}
	}
	return logs
}

// SearchApps 搜索应用
func (as *AppStore) SearchApps(ctx context.Context, query string) []*App {
	as.mu.RLock()
	defer as.mu.RUnlock()

	var results []*App
	for _, app := range as.apps {
		if containsIgnoreCase(app.Name, query) ||
			containsIgnoreCase(app.Description, query) ||
			containsTag(app.Tags, query) {
			results = append(results, app)
		}
	}
	return results
}

// GetInstalledApps 获取已安装应用
func (as *AppStore) GetInstalledApps(ctx context.Context) []*App {
	return as.ListApps(ctx, "", true)
}

// CheckUpdates 检查更新
func (as *AppStore) CheckUpdates(ctx context.Context) []*App {
	as.mu.RLock()
	defer as.mu.RUnlock()

	var updates []*App
	for _, app := range as.apps {
		if app.Installed && app.Version < app.MinVersion {
			updates = append(updates, app)
		}
	}
	return updates
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

func containsTag(tags []string, query string) bool {
	for _, tag := range tags {
		if containsIgnoreCase(tag, query) {
			return true
		}
	}
	return false
}
