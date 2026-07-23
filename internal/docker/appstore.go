package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AppTemplate 应用模板.
type AppTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Icon        string            `json:"icon"`
	Version     string            `json:"version"`
	Image       string            `json:"image"`
	Ports       []PortConfig      `json:"ports"`
	Volumes     []VolumeConfig    `json:"volumes"`
	Environment map[string]string `json:"environment"`
	Compose     string            `json:"compose,omitempty"` // Docker Compose 模板
	Notes       string            `json:"notes"`
	Website     string            `json:"website"`
	Source      string            `json:"source"`
}

// PortConfig 端口配置.
type PortConfig struct {
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	Description string `json:"description"`
	Default     int    `json:"default"` // 默认主机端口
}

// VolumeConfig 卷配置.
type VolumeConfig struct {
	ContainerPath string `json:"containerPath"`
	Description   string `json:"description"`
	Default       string `json:"default"` // 默认主机路径
}

// InstalledApp 已安装应用.
type InstalledApp struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName"`
	TemplateID  string            `json:"templateId"`
	Version     string            `json:"version"`
	Status      string            `json:"status"`
	InstallTime time.Time         `json:"installTime"`
	Ports       map[int]int       `json:"ports"`   // 容器端口 -> 主机端口
	Volumes     map[string]string `json:"volumes"` // 容器路径 -> 主机路径
	Environment map[string]string `json:"environment"`
	ContainerID string            `json:"containerId"`
	ComposePath string            `json:"composePath"`
}

// AppStore 应用商店.
type AppStore struct {
	mu          sync.RWMutex
	manager     *Manager
	templateDir string
	installDir  string
	dataFile    string
	templates   map[string]*AppTemplate
	installed   map[string]*InstalledApp
}

// NewAppStore 创建应用商店.
func NewAppStore(mgr *Manager, dataDir string) (*AppStore, error) {
	templateDir := filepath.Join(dataDir, "app-templates")
	installDir := filepath.Join(dataDir, "apps")
	dataFile := filepath.Join(dataDir, "installed-apps.json")

	store := &AppStore{
		manager:     mgr,
		templateDir: templateDir,
		installDir:  installDir,
		dataFile:    dataFile,
		templates:   make(map[string]*AppTemplate),
		installed:   make(map[string]*InstalledApp),
	}

	// 创建目录
	if err := os.MkdirAll(templateDir, 0750); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(installDir, 0750); err != nil {
		return nil, err
	}

	// 加载内置模板
	store.loadBuiltinTemplates()

	// 加载已安装应用
	if err := store.loadInstalled(); err != nil {
		// 文件不存在不影响启动
		fmt.Printf("加载已安装应用列表失败: %v\n", err)
	}

	return store, nil
}

// loadBuiltinTemplates 加载内置模板.
// loadBuiltinTemplates lives in appstore_templates.go
func (s *AppStore) loadInstalled() error {
	data, err := os.ReadFile(s.dataFile)
	if err != nil {
		return err
	}

	var apps []*InstalledApp
	if err := json.Unmarshal(data, &apps); err != nil {
		return err
	}

	for _, app := range apps {
		s.installed[app.ID] = app
	}
	return nil
}

// saveInstalled 保存已安装应用.
func (s *AppStore) saveInstalled() error {
	apps := make([]*InstalledApp, 0, len(s.installed))
	for _, app := range s.installed {
		apps = append(apps, app)
	}

	data, err := json.MarshalIndent(apps, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.dataFile, data, 0640)
}

// ListTemplates 列出所有模板.
func (s *AppStore) ListTemplates() []*AppTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*AppTemplate, 0, len(s.templates))
	for _, t := range s.templates {
		result = append(result, t)
	}
	return result
}

// GetTemplate 获取模板.
func (s *AppStore) GetTemplate(id string) *AppTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.templates[id]
}

// ListInstalled 列出已安装应用.
func (s *AppStore) ListInstalled() []*InstalledApp {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*InstalledApp, 0, len(s.installed))
	for _, app := range s.installed {
		// 更新状态
		if container, err := s.manager.GetContainer(app.Name); err == nil {
			app.Status = container.State
		} else {
			app.Status = "stopped"
		}
		result = append(result, app)
	}
	return result
}

// GetInstalled 获取已安装应用.
func (s *AppStore) GetInstalled(id string) *InstalledApp {
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, ok := s.installed[id]
	if !ok {
		return nil
	}
	// 更新状态
	if container, err := s.manager.GetContainer(app.Name); err == nil {
		app.Status = container.State
	}
	return app
}

// InstallApp 安装应用.
func (s *AppStore) InstallApp(templateID string, config map[string]interface{}) (*InstalledApp, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	template, ok := s.templates[templateID]
	if !ok {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}

	// 检查是否已安装
	if _, exists := s.installed[templateID]; exists {
		return nil, fmt.Errorf("应用已安装: %s", template.DisplayName)
	}

	appDir := filepath.Join(s.installDir, template.Name)
	if err := os.MkdirAll(appDir, 0750); err != nil {
		return nil, err
	}

	// 准备配置
	ports := make(map[int]int)
	volumes := make(map[string]string)
	env := make(map[string]string)

	// 从配置中提取端口映射
	for _, port := range template.Ports {
		key := fmt.Sprintf("port_%d", port.Port)
		if val, ok := config[key].(float64); ok {
			ports[port.Port] = int(val)
		} else {
			ports[port.Port] = port.Default
		}
	}

	// 从配置中提取卷映射
	for _, vol := range template.Volumes {
		key := fmt.Sprintf("vol_%s", strings.ReplaceAll(vol.ContainerPath, "/", "_"))
		if val, ok := config[key].(string); ok && val != "" {
			volumes[vol.ContainerPath] = val
		} else {
			volumes[vol.ContainerPath] = vol.Default
		}
	}

	// 复制环境变量
	for k, v := range template.Environment {
		env[k] = v
	}

	// 从配置中提取自定义环境变量
	if customEnv, ok := config["environment"].(map[string]interface{}); ok {
		for k, v := range customEnv {
			if str, ok := v.(string); ok {
				env[k] = str
			}
		}
	}

	// 生成 Docker Compose 文件
	composeContent := s.renderCompose(template, config)
	composePath := filepath.Join(appDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0640); err != nil {
		return nil, err
	}

	// 使用 docker-compose 启动
	cmd := exec.CommandContext(context.Background(), "docker-compose", "-f", composePath, "up", "-d")
	cmd.Dir = appDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("启动失败: %w, %s", err, string(output))
	}

	// 获取容器 ID
	containers, _ := s.manager.ListContainers(false)
	var containerID string
	for _, c := range containers {
		if c.Name == template.Name {
			containerID = c.ID
			break
		}
	}

	// 记录安装信息
	app := &InstalledApp{
		ID:          templateID,
		Name:        template.Name,
		DisplayName: template.DisplayName,
		TemplateID:  templateID,
		Version:     template.Version,
		Status:      "running",
		InstallTime: time.Now(),
		Ports:       ports,
		Volumes:     volumes,
		Environment: env,
		ContainerID: containerID,
		ComposePath: composePath,
	}

	s.installed[templateID] = app
	if err := s.saveInstalled(); err != nil {
		fmt.Printf("保存安装信息失败: %v\n", err)
	}

	return app, nil
}

// renderCompose 渲染 Docker Compose 模板.
func (s *AppStore) renderCompose(template *AppTemplate, config map[string]interface{}) string {
	compose := template.Compose
	if compose == "" {
		// 生成默认 compose
		compose = s.generateDefaultCompose(template, config)
	}

	// 替换变量
	for key, val := range config {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		compose = strings.ReplaceAll(compose, placeholder, fmt.Sprintf("%v", val))
	}

	// 替换端口
	for _, port := range template.Ports {
		key := fmt.Sprintf("port_%d", port.Port)
		hostPort := port.Default
		if val, ok := config[key].(float64); ok {
			hostPort = int(val)
		}
		compose = strings.ReplaceAll(compose, "{{.WebPort}}", fmt.Sprintf("%d", hostPort))
	}

	// 替换路径
	for _, vol := range template.Volumes {
		key := fmt.Sprintf("vol_%s", strings.ReplaceAll(vol.ContainerPath, "/", "_"))
		hostPath := vol.Default
		if val, ok := config[key].(string); ok && val != "" {
			hostPath = val
		}
		// 根据 key 生成占位符名
		name := strings.TrimPrefix(vol.ContainerPath, "/")
		name = strings.ReplaceAll(name, "/", "")
		compose = strings.ReplaceAll(compose, fmt.Sprintf("{{.%sDir}}", name), hostPath)
	}

	return compose
}

// generateDefaultCompose 生成默认 compose.
func (s *AppStore) generateDefaultCompose(template *AppTemplate, config map[string]interface{}) string {
	ports := make([]string, 0, len(template.Ports))
	for _, port := range template.Ports {
		hostPort := port.Default
		if val, ok := config[fmt.Sprintf("port_%d", port.Port)].(float64); ok {
			hostPort = int(val)
		}
		ports = append(ports, fmt.Sprintf("      - \"%d:%d\"", hostPort, port.Port))
	}

	volumes := make([]string, 0, len(template.Volumes))
	for _, vol := range template.Volumes {
		hostPath := vol.Default
		if val, ok := config[fmt.Sprintf("vol_%s", strings.ReplaceAll(vol.ContainerPath, "/", "_"))].(string); ok && val != "" {
			hostPath = val
		}
		volumes = append(volumes, fmt.Sprintf("      - %s:%s", hostPath, vol.ContainerPath))
	}

	env := make([]string, 0, len(template.Environment))
	for k, v := range template.Environment {
		env = append(env, fmt.Sprintf("      - %s=%s", k, v))
	}

	return fmt.Sprintf(`version: '3'
services:
  %s:
    image: %s
    container_name: %s
    restart: unless-stopped
    ports:
%s
    volumes:
%s
    environment:
%s
`, template.Name, template.Image, template.Name, strings.Join(ports, "\n"), strings.Join(volumes, "\n"), strings.Join(env, "\n"))
}

// UninstallApp 卸载应用.
func (s *AppStore) UninstallApp(id string, removeData bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	// 停止并删除容器
	if app.ComposePath != "" {
		cmd := exec.CommandContext(context.Background(), "docker-compose", "-f", app.ComposePath, "down")
		if err := cmd.Run(); err != nil {
			log.Printf("docker-compose down 失败: %v", err)
		}
	} else if app.ContainerID != "" {
		if err := s.manager.RemoveContainer(app.ContainerID, true); err != nil {
			log.Printf("移除容器失败: %v", err)
		}
	}

	// 删除数据
	if removeData {
		appDir := filepath.Join(s.installDir, app.Name)
		_ = os.RemoveAll(appDir)
	}

	// 从记录中删除
	delete(s.installed, id)
	if err := s.saveInstalled(); err != nil {
		return err
	}

	return nil
}

// StartApp 启动应用.
func (s *AppStore) StartApp(id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	if app.ComposePath != "" {
		cmd := exec.CommandContext(context.Background(), "docker-compose", "-f", app.ComposePath, "start")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("启动失败: %w, %s", err, string(output))
		}
	} else {
		return s.manager.StartContainer(app.ContainerID)
	}

	app.Status = "running"
	if err := s.saveInstalled(); err != nil {
		log.Printf("保存安装状态失败: %v", err)
	}
	return nil
}

// StopApp 停止应用.
func (s *AppStore) StopApp(id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	if app.ComposePath != "" {
		cmd := exec.CommandContext(context.Background(), "docker-compose", "-f", app.ComposePath, "stop")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("停止失败: %w, %s", err, string(output))
		}
	} else {
		return s.manager.StopContainer(app.ContainerID, 10)
	}

	app.Status = "stopped"
	if err := s.saveInstalled(); err != nil {
		log.Printf("保存安装状态失败: %v", err)
	}
	return nil
}

// RestartApp 重启应用.
func (s *AppStore) RestartApp(id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	if app.ComposePath != "" {
		cmd := exec.CommandContext(context.Background(), "docker-compose", "-f", app.ComposePath, "restart")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("重启失败: %w, %s", err, string(output))
		}
	} else {
		return s.manager.RestartContainer(app.ContainerID, 10)
	}

	return nil
}

// UpdateApp 更新应用.
func (s *AppStore) UpdateApp(id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	template, ok := s.templates[id]
	if !ok {
		return fmt.Errorf("模板不存在: %s", id)
	}

	// 拉取最新镜像
	if err := s.manager.PullImage(template.Image); err != nil {
		return fmt.Errorf("拉取镜像失败: %w", err)
	}

	// 重新创建容器
	if app.ComposePath != "" {
		cmd := exec.CommandContext(context.Background(), "docker-compose", "-f", app.ComposePath, "up", "-d")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("更新失败: %w, %s", err, string(output))
		}
	}

	return nil
}

// GetAppStats 获取应用统计.
func (s *AppStore) GetAppStats(id string) (map[string]interface{}, error) {
	app, ok := s.installed[id]
	if !ok {
		return nil, fmt.Errorf("应用未安装: %s", id)
	}

	if app.ContainerID == "" {
		return nil, fmt.Errorf("容器 ID 为空")
	}

	stats, err := s.manager.GetContainerStats(app.ContainerID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"cpuUsage":   stats.CPUUsage,
		"memUsage":   stats.MemUsage,
		"memLimit":   stats.MemLimit,
		"netRx":      stats.NetRX,
		"netTx":      stats.NetTX,
		"blockRead":  stats.BlockRead,
		"blockWrite": stats.BlockWrite,
	}, nil
}

// =============================================================================
// 模板版本管理（参考飞牛fnOS应用市场设计）
// =============================================================================

// TemplateVersion 模板版本信息.
type TemplateVersion struct {
	ID           string            `json:"id"`
	TemplateID   string            `json:"templateId"`
	Version      string            `json:"version"`
	ImageTag     string            `json:"imageTag"`
	Compose      string            `json:"compose,omitempty"`
	ReleaseNotes string            `json:"releaseNotes"`
	PublishedAt  time.Time         `json:"publishedAt"`
	Digest       string            `json:"digest"`
	Deprecated   bool              `json:"deprecated"`
	Environment  map[string]string `json:"environment,omitempty"`
	MinVersion   string            `json:"minVersion,omitempty"` // 最低系统版本要求
}

