package webapphost

import (
	"fmt"
	"log"
	"sort"
	"time"
)

// CreateApp 创建 Web 应用
func (m *WebAppManager) CreateApp(config *DeployConfig) (*WebApp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查应用数量限制
	if len(m.apps) >= m.config.MaxApps {
		return nil, fmt.Errorf("maximum number of apps (%d) reached", m.config.MaxApps)
	}

	// 检查名称唯一性
	for _, app := range m.apps {
		if app.Name == config.AppName {
			return nil, fmt.Errorf("app name already exists: %s", config.AppName)
		}
	}

	// 验证部署配置
	if err := validateDeployConfig(config); err != nil {
		return nil, fmt.Errorf("invalid deploy config: %w", err)
	}

	// 创建应用
	app := &WebApp{
		ID:          GenerateID("app"),
		Name:        config.AppName,
		DisplayName: config.AppName,
		Description: "",
		Version:     config.Version,
		Type:        config.Type,
		Status:      "stopped",
		Domain:      config.Domain,
		Path:        config.Path,
		Port:        config.Port,
		SSLEnabled:  config.SSLEnabled,
		TemplateID:  config.TemplateID,
		Image:       config.Image,
		EnvVars:     config.EnvVars,
		Volumes:     config.Volumes,
		Ports:       config.Ports,
		Resources:   config.Resources,
		Labels:      config.Labels,
		Config:      config.Config,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.apps[app.ID] = app

	// 配置域名路由
	if config.Domain != "" {
		domainConfig := &DomainConfig{
			Domain:        config.Domain,
			AppID:         app.ID,
			SSLEnabled:    config.SSLEnabled,
			RedirectHTTPS: config.SSLEnabled,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		m.domains[config.Domain] = domainConfig
	}

	// 创建路由规则
	if config.Path != "" {
		routeID := GenerateID("route")
		route := &RouteRule{
			ID:        routeID,
			Domain:    config.Domain,
			Path:      config.Path,
			AppID:     app.ID,
			Priority:  100,
			StripPath: false,
			CreatedAt: time.Now(),
		}
		m.routes[routeID] = route
	}

	// 申请 SSL 证书
	if config.SSLEnabled && config.Domain != "" && m.config.EnableSSL {
		log.Printf("Requesting SSL certificate for domain: %s", config.Domain)
		// SSL 证书申请异步执行
	}

	return app, nil
}

// GetApp 获取应用
func (m *WebAppManager) GetApp(id string) (*WebApp, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, exists := m.apps[id]
	if !exists {
		return nil, fmt.Errorf("app not found: %s", id)
	}
	return app, nil
}

// GetAppByName 根据名称获取应用
func (m *WebAppManager) GetAppByName(name string) (*WebApp, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, app := range m.apps {
		if app.Name == name {
			return app, nil
		}
	}
	return nil, fmt.Errorf("app not found: %s", name)
}

// ListApps 列出所有应用
func (m *WebAppManager) ListApps(opts *ListOptions) []*WebApp {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]*WebApp, 0, len(m.apps))
	for _, app := range m.apps {
		if opts != nil && opts.Status != "" && app.Status != opts.Status {
			continue
		}
		if opts != nil && opts.Type != "" && app.Type != opts.Type {
			continue
		}
		apps = append(apps, app)
	}

	// 排序
	if opts != nil && opts.SortBy != "" {
		sortApps(apps, opts.SortBy, opts.SortDesc)
	}

	// 分页
	if opts != nil && opts.Limit > 0 {
		start := opts.Offset
		if start >= len(apps) {
			return []*WebApp{}
		}
		end := start + opts.Limit
		if end > len(apps) {
			end = len(apps)
		}
		return apps[start:end]
	}

	return apps
}

// ListOptions 列表选项
type ListOptions struct {
	Status   string `json:"status,omitempty"`
	Type     string `json:"type,omitempty"`
	SortBy   string `json:"sort_by,omitempty"` // name, created_at, status
	SortDesc bool   `json:"sort_desc,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

// sortApps 排序应用列表
func sortApps(apps []*WebApp, sortBy string, desc bool) {
	sort.Slice(apps, func(i, j int) bool {
		switch sortBy {
		case "name":
			if desc {
				return apps[i].Name > apps[j].Name
			}
			return apps[i].Name < apps[j].Name
		case "created_at":
			if desc {
				return apps[i].CreatedAt.After(apps[j].CreatedAt)
			}
			return apps[i].CreatedAt.Before(apps[j].CreatedAt)
		case "status":
			if desc {
				return apps[i].Status > apps[j].Status
			}
			return apps[i].Status < apps[j].Status
		default:
			return apps[i].CreatedAt.Before(apps[j].CreatedAt)
		}
	})
}

// UpdateApp 更新应用
func (m *WebAppManager) UpdateApp(id string, updates *UpdateAppRequest) (*WebApp, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.apps[id]
	if !exists {
		return nil, fmt.Errorf("app not found: %s", id)
	}

	if updates.DisplayName != nil {
		app.DisplayName = *updates.DisplayName
	}
	if updates.Description != nil {
		app.Description = *updates.Description
	}
	if updates.Domain != nil {
		app.Domain = *updates.Domain
	}
	if updates.Path != nil {
		app.Path = *updates.Path
	}
	if updates.Port > 0 {
		app.Port = updates.Port
	}
	if updates.SSLEnabled != nil {
		app.SSLEnabled = *updates.SSLEnabled
	}
	if updates.EnvVars != nil {
		app.EnvVars = updates.EnvVars
	}
	if updates.Volumes != nil {
		app.Volumes = updates.Volumes
	}
	if updates.Resources != nil {
		app.Resources = *updates.Resources
	}
	if updates.Labels != nil {
		app.Labels = updates.Labels
	}
	if updates.Config != nil {
		app.Config = updates.Config
	}
	if updates.Tags != nil {
		app.Tags = updates.Tags
	}

	app.UpdatedAt = time.Now()
	return app, nil
}

// UpdateAppRequest 更新应用请求
type UpdateAppRequest struct {
	DisplayName *string           `json:"display_name,omitempty"`
	Description *string           `json:"description,omitempty"`
	Domain      *string           `json:"domain,omitempty"`
	Path        *string           `json:"path,omitempty"`
	Port        int               `json:"port,omitempty"`
	SSLEnabled  *bool             `json:"ssl_enabled,omitempty"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Volumes     []VolumeMount     `json:"volumes,omitempty"`
	Resources   *ResourceLimit    `json:"resources,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Config      map[string]string `json:"config,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
}

// DeleteApp 删除应用
func (m *WebAppManager) DeleteApp(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.apps[id]
	if !exists {
		return fmt.Errorf("app not found: %s", id)
	}

	// 如果正在运行，先停止
	if app.Status == "running" {
		app.Status = "stopping"
		// 实际停止逻辑
		app.Status = "stopped"
	}

	// 清理域名配置
	for domain, dc := range m.domains {
		if dc.AppID == id {
			delete(m.domains, domain)
		}
	}

	// 清理路由规则
	for routeID, route := range m.routes {
		if route.AppID == id {
			delete(m.routes, routeID)
		}
	}

	// 清理告警规则
	for alertID, alert := range m.alerts {
		if alert.AppID == id {
			delete(m.alerts, alertID)
		}
	}

	delete(m.apps, id)
	return nil
}

// StartApp 启动应用
func (m *WebAppManager) StartApp(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.apps[id]
	if !exists {
		return fmt.Errorf("app not found: %s", id)
	}

	if app.Status == "running" {
		return fmt.Errorf("app is already running")
	}

	app.Status = "starting"
	app.UpdatedAt = time.Now()

	// 根据应用类型启动
	switch app.Type {
	case "docker":
		return m.startDockerApp(app)
	case "static":
		return m.startStaticApp(app)
	case "proxy":
		return m.startProxyApp(app)
	default:
		app.Status = "error"
		return fmt.Errorf("unsupported app type: %s", app.Type)
	}
}

// startDockerApp 启动 Docker 应用
func (m *WebAppManager) startDockerApp(app *WebApp) error {
	// 模拟 Docker 容器启动
	log.Printf("Starting Docker app: %s (image: %s)", app.Name, app.Image)
	app.Status = "running"
	now := time.Now()
	app.StartedAt = &now
	return nil
}

// startStaticApp 启动静态应用
func (m *WebAppManager) startStaticApp(app *WebApp) error {
	log.Printf("Starting static app: %s", app.Name)
	app.Status = "running"
	now := time.Now()
	app.StartedAt = &now
	return nil
}

// startProxyApp 启动代理应用
func (m *WebAppManager) startProxyApp(app *WebApp) error {
	log.Printf("Starting proxy app: %s", app.Name)
	app.Status = "running"
	now := time.Now()
	app.StartedAt = &now
	return nil
}

// StopApp 停止应用
func (m *WebAppManager) StopApp(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, exists := m.apps[id]
	if !exists {
		return fmt.Errorf("app not found: %s", id)
	}

	if app.Status == "stopped" {
		return fmt.Errorf("app is already stopped")
	}

	app.Status = "stopping"
	app.UpdatedAt = time.Now()

	// 模拟停止
	app.Status = "stopped"
	app.StartedAt = nil
	return nil
}

// RestartApp 重启应用
func (m *WebAppManager) RestartApp(id string) error {
	if err := m.StopApp(id); err != nil && err.Error() != "app is already stopped" {
		return err
	}
	return m.StartApp(id)
}

// GetAppStatus 获取应用状态
func (m *WebAppManager) GetAppStatus(id string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	app, exists := m.apps[id]
	if !exists {
		return "", fmt.Errorf("app not found: %s", id)
	}
	return app.Status, nil
}

// GetAppCount 获取应用数量
func (m *WebAppManager) GetAppCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.apps)
}

// GetRunningAppCount 获取运行中应用数量
func (m *WebAppManager) GetRunningAppCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, app := range m.apps {
		if app.Status == "running" {
			count++
		}
	}
	return count
}

// validateDeployConfig 验证部署配置
func validateDeployConfig(config *DeployConfig) error {
	if config.AppName == "" {
		return fmt.Errorf("app name is required")
	}
	if config.Type == "" {
		return fmt.Errorf("app type is required")
	}
	if config.Type != "docker" && config.Type != "static" && config.Type != "proxy" {
		return fmt.Errorf("invalid app type: %s", config.Type)
	}
	if config.Type == "docker" && config.Image == "" {
		return fmt.Errorf("docker image is required for docker apps")
	}
	if config.Path == "" {
		config.Path = "/"
	}
	if config.Version == "" {
		config.Version = "latest"
	}
	return nil
}
