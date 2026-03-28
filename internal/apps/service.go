// Package apps 应用中心核心框架
// 提供应用服务管理、目录管理、安装卸载、状态监控等功能
package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nas-os/pkg/app"
)

// Service 应用服务管理器 - 应用中心核心
type Service struct {
	mu sync.RWMutex

	catalog   *Catalog           // 应用目录管理
	installer *Installer         // 应用安装器
	manager   ContainerManager   // 应用生命周期管理
	repo      *Repository        // 应用仓库

	dataDir   string             // 数据目录
	stateFile string             // 状态文件
	installed map[string]*app.InstalledApp // 已安装应用
}

// ContainerManager 容器管理器接口
type ContainerManager interface {
	// 容器操作
	CreateContainer(ctx context.Context, config *app.ContainerConfig) (string, error)
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string, timeout int) error
	RemoveContainer(ctx context.Context, id string, force bool) error
	GetContainerStatus(ctx context.Context, id string) (*app.ContainerStatus, error)
	
	// Compose 操作
	ComposeUp(ctx context.Context, composePath string) error
	ComposeDown(ctx context.Context, composePath string) error
	ComposePS(ctx context.Context, composePath string) ([]app.ComposeService, error)
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	DataDir      string             // 数据目录
	TemplateDir  string             // 模板目录（可选，默认 dataDir/templates）
	InstallDir   string             // 安装目录（可选，默认 dataDir/compose）
	Manager      ContainerManager   // 容器管理器（必须）
}

// NewService 创建应用服务管理器
func NewService(cfg *ServiceConfig) (*Service, error) {
	if cfg.DataDir == "" {
		return nil, fmt.Errorf("数据目录不能为空")
	}
	if cfg.Manager == nil {
		return nil, fmt.Errorf("容器管理器不能为空")
	}

	// 设置默认目录
	templateDir := cfg.TemplateDir
	if templateDir == "" {
		templateDir = filepath.Join(cfg.DataDir, "templates")
	}
	installDir := cfg.InstallDir
	if installDir == "" {
		installDir = filepath.Join(cfg.DataDir, "compose")
	}
	stateFile := filepath.Join(cfg.DataDir, "installed-apps.json")

	// 创建目录
	if err := os.MkdirAll(templateDir, 0750); err != nil {
		return nil, fmt.Errorf("创建模板目录失败: %w", err)
	}
	if err := os.MkdirAll(installDir, 0750); err != nil {
		return nil, fmt.Errorf("创建安装目录失败: %w", err)
	}

	// 创建子模块
	catalog, err := NewCatalog(templateDir)
	if err != nil {
		return nil, fmt.Errorf("创建应用目录失败: %w", err)
	}

	installer, err := NewInstaller(installDir, cfg.Manager)
	if err != nil {
		return nil, fmt.Errorf("创建应用安装器失败: %w", err)
	}

	repo, err := NewRepository(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("创建应用仓库失败: %w", err)
	}

	service := &Service{
		catalog:   catalog,
		installer: installer,
		manager:   cfg.Manager,
		repo:      repo,
		dataDir:   cfg.DataDir,
		stateFile: stateFile,
		installed: make(map[string]*app.InstalledApp),
	}

	// 加载已安装应用状态
	if err := service.loadState(); err != nil {
		// 文件不存在不影响启动
		fmt.Printf("加载已安装应用状态失败: %v\n", err)
	}

	return service, nil
}

// loadState 加载已安装应用状态
func (s *Service) loadState() error {
	data, err := os.ReadFile(s.stateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取状态文件失败: %w", err)
	}

	return json.Unmarshal(data, &s.installed)
}

// saveState 保存已安装应用状态
func (s *Service) saveState() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.installed, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化状态失败: %w", err)
	}

	return os.WriteFile(s.stateFile, data, 0644)
}

// ========== 应用目录操作 ==========

// ListTemplates 列出所有应用模板
func (s *Service) ListTemplates(category string) ([]*app.Template, error) {
	return s.catalog.List(category)
}

// GetTemplate 获取应用模板详情
func (s *Service) GetTemplate(id string) (*app.Template, error) {
	return s.catalog.Get(id)
}

// SearchTemplates 搜索应用模板
func (s *Service) SearchTemplates(query string) ([]*app.Template, error) {
	return s.catalog.Search(query)
}

// GetCategories 获取应用分类列表
func (s *Service) GetCategories() []string {
	return s.catalog.Categories()
}

// ========== 应用安装操作 ==========

// Install 安装应用
func (s *Service) Install(ctx context.Context, templateID string, opts *app.InstallOptions) (*app.InstalledApp, error) {
	// 获取模板
	template, err := s.catalog.Get(templateID)
	if err != nil {
		return nil, fmt.Errorf("获取应用模板失败: %w", err)
	}

	// 检查是否已安装
	s.mu.RLock()
	if _, exists := s.installed[templateID]; exists {
		s.mu.RUnlock()
		return nil, fmt.Errorf("应用 %s 已安装", templateID)
	}
	s.mu.RUnlock()

	// 执行安装
	installed, err := s.installer.Install(ctx, template, opts)
	if err != nil {
		return nil, fmt.Errorf("安装应用失败: %w", err)
	}

	// 记录安装状态
	s.mu.Lock()
	s.installed[templateID] = installed
	s.mu.Unlock()

	// 保存状态
	if err := s.saveState(); err != nil {
		fmt.Printf("保存安装状态失败: %v\n", err)
	}

	return installed, nil
}

// Uninstall 卸载应用
func (s *Service) Uninstall(ctx context.Context, appID string, opts *app.UninstallOptions) error {
	// 检查是否已安装
	s.mu.RLock()
	installed, exists := s.installed[appID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("应用 %s 未安装", appID)
	}

	// 执行卸载
	if err := s.installer.Uninstall(ctx, installed, opts); err != nil {
		return fmt.Errorf("卸载应用失败: %w", err)
	}

	// 移除状态记录
	s.mu.Lock()
	delete(s.installed, appID)
	s.mu.Unlock()

	// 保存状态
	if err := s.saveState(); err != nil {
		fmt.Printf("保存安装状态失败: %v\n", err)
	}

	return nil
}

// ========== 应用状态操作 ==========

// ListInstalled 列出已安装应用
func (s *Service) ListInstalled() []*app.InstalledApp {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*app.InstalledApp, 0, len(s.installed))
	for _, app := range s.installed {
		result = append(result, app)
	}
	return result
}

// GetInstalled 获取已安装应用详情
func (s *Service) GetInstalled(appID string) (*app.InstalledApp, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	installed, exists := s.installed[appID]
	if !exists {
		return nil, fmt.Errorf("应用 %s 未安装", appID)
	}
	return installed, nil
}

// GetAppStatus 获取应用实时状态
func (s *Service) GetAppStatus(ctx context.Context, appID string) (*app.AppStatus, error) {
	s.mu.RLock()
	installed, exists := s.installed[appID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("应用 %s 未安装", appID)
	}

	return s.installer.GetStatus(ctx, installed)
}

// GetAllStatus 获取所有已安装应用状态
func (s *Service) GetAllStatus(ctx context.Context) map[string]*app.AppStatus {
	s.mu.RLock()
	apps := s.installed
	s.mu.RUnlock()

	result := make(map[string]*app.AppStatus)
	for id, installed := range apps {
		status, err := s.installer.GetStatus(ctx, installed)
		if err != nil {
			result[id] = &app.AppStatus{
				State:   app.AppStateError,
				Message: err.Error(),
			}
		} else {
			result[id] = status
		}
	}
	return result
}

// ========== 应用控制操作 ==========

// StartApp 启动应用
func (s *Service) StartApp(ctx context.Context, appID string) error {
	s.mu.RLock()
	installed, exists := s.installed[appID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("应用 %s 未安装", appID)
	}

	return s.installer.Start(ctx, installed)
}

// StopApp 停止应用
func (s *Service) StopApp(ctx context.Context, appID string, timeout int) error {
	s.mu.RLock()
	installed, exists := s.installed[appID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("应用 %s 未安装", appID)
	}

	return s.installer.Stop(ctx, installed, timeout)
}

// RestartApp 重启应用
func (s *Service) RestartApp(ctx context.Context, appID string, timeout int) error {
	s.mu.RLock()
	installed, exists := s.installed[appID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("应用 %s 未安装", appID)
	}

	return s.installer.Restart(ctx, installed, timeout)
}

// ========== 应用配置操作 ==========

// GetAppConfig 获取应用配置
func (s *Service) GetAppConfig(appID string) (map[string]string, error) {
	s.mu.RLock()
	installed, exists := s.installed[appID]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("应用 %s 未安装", appID)
	}

	return s.installer.GetConfig(installed)
}

// UpdateAppConfig 更新应用配置
func (s *Service) UpdateAppConfig(ctx context.Context, appID string, config map[string]string) error {
	s.mu.RLock()
	installed, exists := s.installed[appID]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("应用 %s 未安装", appID)
	}

	if err := s.installer.UpdateConfig(ctx, installed, config); err != nil {
		return fmt.Errorf("更新应用配置失败: %w", err)
	}

	// 更新状态记录
	s.mu.Lock()
	installed.Config = config
	installed.UpdatedAt = time.Now()
	s.mu.Unlock()

	return s.saveState()
}

// ========== 应用仓库操作 ==========

// RefreshCatalog 从远程仓库刷新应用目录
func (s *Service) RefreshCatalog(ctx context.Context) error {
	return s.repo.Refresh(ctx, s.catalog)
}

// AddRemoteRepo 添加远程仓库源
func (s *Service) AddRemoteRepo(url string, name string) error {
	return s.repo.AddSource(url, name)
}

// RemoveRemoteRepo 移除远程仓库源
func (s *Service) RemoveRemoteRepo(name string) error {
	return s.repo.RemoveSource(name)
}

// ListRepos 列出所有仓库源
func (s *Service) ListRepos() []app.RepositorySource {
	return s.repo.ListSources()
}

// ========== 资源清理 ==========

// Close 关闭服务
func (s *Service) Close() error {
	// 保存最终状态
	if err := s.saveState(); err != nil {
		fmt.Printf("保存状态失败: %v\n", err)
	}
	return nil
}