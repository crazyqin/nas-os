// Package apps Docker Compose Web UI管理器
// 对标群晖Container Manager，YAML可视化编辑+模板市场
package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// ========== 常量 ==========

const (
	ComposeDir     = "/opt/nas-os/compose"
	TemplatesDir   = "/opt/nas-os/compose/templates"
	ComposeBinPath = "/usr/bin/docker-compose"
)

// ========== 类型 ==========

// ComposeProject Docker Compose项目
type ComposeProject struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	YAMLContent string            `json:"yaml_content"`
	FilePath    string            `json:"file_path"`
	Status      ComposeStatus     `json:"status"`
	Services    []ComposeServiceInfo  `json:"services"`
	EnvVars     map[string]string `json:"env_vars,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	IsTemplate  bool              `json:"is_template"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ComposeStatus 项目状态
type ComposeStatus string

const (
	ComposeStatusStopped  ComposeStatus = "stopped"
	ComposeStatusRunning  ComposeStatus = "running"
	ComposeStatusStarting ComposeStatus = "starting"
	ComposeStatusStopping ComposeStatus = "stopping"
	ComposeStatusError    ComposeStatus = "error"
	ComposeStatusPartial  ComposeStatus = "partial" // 部分容器运行中
)

// ComposeServiceInfo 单个服务的信息
type ComposeServiceInfo struct {
	Name         string            `json:"name"`
	Image        string            `json:"image"`
	Status       string            `json:"status"`
	Ports        []string          `json:"ports,omitempty"`
	Volumes      []string          `json:"volumes,omitempty"`
	Environment  map[string]string `json:"environment,omitempty"`
	HealthStatus string            `json:"health_status,omitempty"`
	ContainerID  string            `json:"container_id,omitempty"`
	RestartCount int               `json:"restart_count"`
	Logs         string            `json:"logs,omitempty"`
}

// ComposeTemplate Compose模板
type ComposeTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Icon        string   `json:"icon,omitempty"`
	YAMLContent string   `json:"yaml_content"`
	Variables   []TemplateVar `json:"variables,omitempty"`
	Tags        []string  `json:"tags"`
	Downloads   int      `json:"downloads"`
	Rating      float64  `json:"rating"`
	Author      string   `json:"author"`
	Version     string   `json:"version"`
}

// TemplateVar 模板变量
type TemplateVar struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	Type         string `json:"type"` // string, int, bool, select
	DefaultValue string `json:"default_value,omitempty"`
	Required     bool   `json:"required"`
	Options      []string `json:"options,omitempty"` // 用于select类型
}

// ComposeManager Docker Compose管理器
type ComposeManager struct {
	mu        sync.RWMutex
	projects  map[string]*ComposeProject
	templates map[string]*ComposeTemplate
	baseDir   string
	dockerCmd string
}

// NewComposeManager 创建Compose管理器
func NewComposeManager(baseDir string) (*ComposeManager, error) {
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return nil, fmt.Errorf("创建Compose目录失败: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(baseDir, "templates"), 0750); err != nil {
		return nil, fmt.Errorf("创建模板目录失败: %w", err)
	}

	mgr := &ComposeManager{
		projects:  make(map[string]*ComposeProject),
		templates: make(map[string]*ComposeTemplate),
		baseDir:   baseDir,
		dockerCmd: "docker",
	}

	// 检测docker-compose命令
	if _, err := exec.LookPath("docker-compose"); err == nil {
		mgr.dockerCmd = "docker-compose"
	} else if _, err := exec.LookPath("docker"); err == nil {
		mgr.dockerCmd = "docker"
	}

	// 加载已有项目
	if err := mgr.loadProjects(); err != nil {
		return nil, err
	}

	// 初始化内置模板
	mgr.initBuiltinTemplates()

	return mgr, nil
}

// CreateProject 创建Compose项目
func (m *ComposeManager) CreateProject(ctx context.Context, name, yamlContent string, envVars map[string]string) (*ComposeProject, error) {
	if name == "" {
		return nil, fmt.Errorf("项目名不能为空")
	}
	if yamlContent == "" {
		return nil, fmt.Errorf("YAML内容不能为空")
	}

	// 验证YAML格式
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &parsed); err != nil {
		return nil, fmt.Errorf("YAML格式错误: %w", err)
	}

	projectID := uuid.New().String()
	filePath := filepath.Join(m.baseDir, projectID, "docker-compose.yml")

	// 创建项目目录
	if err := os.MkdirAll(filepath.Dir(filePath), 0750); err != nil {
		return nil, err
	}

	// 写入YAML文件
	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		return nil, err
	}

	// 写入环境变量文件
	if len(envVars) > 0 {
		envContent := ""
		for k, v := range envVars {
			envContent += fmt.Sprintf("%s=%s\n", k, v)
		}
		envPath := filepath.Join(filepath.Dir(filePath), ".env")
		os.WriteFile(envPath, []byte(envContent), 0600)
	}

	// 解析服务列表
	services := m.parseServices(parsed)

	project := &ComposeProject{
		ID:          projectID,
		Name:        name,
		YAMLContent: yamlContent,
		FilePath:    filePath,
		Status:      ComposeStatusStopped,
		Services:    services,
		EnvVars:     envVars,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.mu.Lock()
	m.projects[projectID] = project
	m.mu.Unlock()

	if err := m.saveProjects(); err != nil {
		return nil, err
	}

	return project, nil
}

// Up 启动项目
func (m *ComposeManager) Up(ctx context.Context, projectID string, detach bool) error {
	project, err := m.getProject(projectID)
	if err != nil {
		return err
	}

	project.Status = ComposeStatusStarting
	m.saveProjects()

	args := []string{"-f", project.FilePath, "up"}
	if detach {
		args = append(args, "-d")
	}

	cmd := exec.CommandContext(ctx, m.dockerCmd, args...)
	cmd.Dir = filepath.Dir(project.FilePath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		project.Status = ComposeStatusError
		m.saveProjects()
		return fmt.Errorf("启动失败: %s\n%s", err, string(output))
	}

	project.Status = ComposeStatusRunning
	m.saveProjects()

	// 刷新服务状态
	go m.refreshServices(ctx, projectID)

	return nil
}

// Down 停止项目
func (m *ComposeManager) Down(ctx context.Context, projectID string) error {
	project, err := m.getProject(projectID)
	if err != nil {
		return err
	}

	project.Status = ComposeStatusStopping
	m.saveProjects()

	cmd := exec.CommandContext(ctx, m.dockerCmd, "-f", project.FilePath, "down")
	output, err := cmd.CombinedOutput()
	if err != nil {
		project.Status = ComposeStatusError
		m.saveProjects()
		return fmt.Errorf("停止失败: %s\n%s", err, string(output))
	}

	project.Status = ComposeStatusStopped
	m.saveProjects()

	return nil
}

// Restart 重启项目
func (m *ComposeManager) Restart(ctx context.Context, projectID string) error {
	if err := m.Down(ctx, projectID); err != nil {
		return err
	}
	return m.Up(ctx, projectID, true)
}

// GetLogs 获取项目日志
func (m *ComposeManager) GetLogs(ctx context.Context, projectID string, tail int) (string, error) {
	project, err := m.getProject(projectID)
	if err != nil {
		return "", err
	}

	args := []string{"-f", project.FilePath, "logs"}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}

	cmd := exec.CommandContext(ctx, m.dockerCmd, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("获取日志失败: %s", err)
	}

	return string(output), nil
}

// GetServiceLogs 获取单个服务日志
func (m *ComposeManager) GetServiceLogs(ctx context.Context, projectID, serviceName string, tail int) (string, error) {
	project, err := m.getProject(projectID)
	if err != nil {
		return "", err
	}

	args := []string{"-f", project.FilePath, "logs"}
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}
	args = append(args, serviceName)

	cmd := exec.CommandContext(ctx, m.dockerCmd, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("获取服务日志失败: %s", err)
	}

	return string(output), nil
}

// UpdateYAML 更新项目YAML内容
func (m *ComposeManager) UpdateYAML(ctx context.Context, projectID, newYAML string) error {
	m.mu.Lock()
	project, ok := m.projects[projectID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("项目不存在")
	}

	// 验证YAML
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(newYAML), &parsed); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("YAML格式错误: %w", err)
	}

	project.YAMLContent = newYAML
	project.Services = m.parseServices(parsed)
	project.UpdatedAt = time.Now()
	m.mu.Unlock()

	// 写入文件
	return os.WriteFile(project.FilePath, []byte(newYAML), 0644)
}

// ImportFromTemplate 从模板创建项目
func (m *ComposeManager) ImportFromTemplate(ctx context.Context, templateID, projectName string, vars map[string]string) (*ComposeProject, error) {
	m.mu.RLock()
	tmpl, ok := m.templates[templateID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("模板 '%s' 不存在", templateID)
	}

	// 替换模板变量
	yamlContent := tmpl.YAMLContent
	for k, v := range vars {
		yamlContent = strings.ReplaceAll(yamlContent, fmt.Sprintf("{{.%s}}", k), v)
	}

	// 增加下载计数
	m.mu.Lock()
	tmpl.Downloads++
	m.mu.Unlock()

	return m.CreateProject(ctx, projectName, yamlContent, vars)
}

// ListProjects 列出所有项目
func (m *ComposeManager) ListProjects() []*ComposeProject {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ComposeProject, 0, len(m.projects))
	for _, p := range m.projects {
		result = append(result, p)
	}
	return result
}

// GetProject 获取项目信息
func (m *ComposeManager) GetProject(projectID string) (*ComposeProject, error) {
	return m.getProject(projectID)
}

// DeleteProject 删除项目
func (m *ComposeManager) DeleteProject(ctx context.Context, projectID string) error {
	m.mu.Lock()
	project, ok := m.projects[projectID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("项目不存在")
	}

	// 先停止
	if project.Status == ComposeStatusRunning {
		m.mu.Unlock()
		m.Down(ctx, projectID)
		m.mu.Lock()
	}

	// 删除项目目录
	projectDir := filepath.Dir(project.FilePath)
	os.RemoveAll(projectDir)

	delete(m.projects, projectID)
	m.mu.Unlock()

	return m.saveProjects()
}

// ListTemplates 列出所有模板
func (m *ComposeManager) ListTemplates() []*ComposeTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ComposeTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		result = append(result, t)
	}
	return result
}

// GetStats 获取统计信息
func (m *ComposeManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	running, stopped, errored := 0, 0, 0
	for _, p := range m.projects {
		switch p.Status {
		case ComposeStatusRunning:
			running++
		case ComposeStatusStopped:
			stopped++
		default:
			errored++
		}
	}

	return map[string]interface{}{
		"total_projects": len(m.projects),
		"running":        running,
		"stopped":        stopped,
		"errored":        errored,
		"templates":      len(m.templates),
	}
}

// ========== 辅助函数 ==========

func (m *ComposeManager) getProject(projectID string) (*ComposeProject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	project, ok := m.projects[projectID]
	if !ok {
		return nil, fmt.Errorf("项目 '%s' 不存在", projectID)
	}
	return project, nil
}

func (m *ComposeManager) parseServices(parsed map[string]interface{}) []ComposeServiceInfo {
	var services []ComposeServiceInfo
	servicesRaw, ok := parsed["services"].(map[string]interface{})
	if !ok {
		return services
	}
	for name, svc := range servicesRaw {
		svcMap, ok := svc.(map[string]interface{})
		if !ok {
			continue
		}
		s := ComposeServiceInfo{Name: name}
		if img, ok := svcMap["image"].(string); ok {
			s.Image = img
		}
		if ports, ok := svcMap["ports"].([]interface{}); ok {
			for _, p := range ports {
				s.Ports = append(s.Ports, fmt.Sprintf("%v", p))
			}
		}
		if vols, ok := svcMap["volumes"].([]interface{}); ok {
			for _, v := range vols {
				s.Volumes = append(s.Volumes, fmt.Sprintf("%v", v))
			}
		}
		services = append(services, s)
	}
	return services
}

func (m *ComposeManager) refreshServices(ctx context.Context, projectID string) {
	project, err := m.getProject(projectID)
	if err != nil {
		return
	}

	cmd := exec.CommandContext(ctx, m.dockerCmd, "-f", project.FilePath, "ps", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return
	}

	// 解析docker-compose ps输出，更新服务状态
	m.mu.Lock()
	for i := range project.Services {
		// 简化：检查输出中是否包含服务名
		if strings.Contains(string(output), project.Services[i].Name) {
			project.Services[i].Status = "running"
		} else {
			project.Services[i].Status = "stopped"
		}
	}
	m.mu.Unlock()
}

func (m *ComposeManager) saveProjects() error {
	m.mu.RLock()
	data, err := json.MarshalIndent(m.projects, "", "  ")
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(m.baseDir, "projects.json"), data, 0644)
}

func (m *ComposeManager) loadProjects() error {
	path := filepath.Join(m.baseDir, "projects.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m.projects)
}

func (m *ComposeManager) initBuiltinTemplates() {
	templates := []*ComposeTemplate{
		{
			ID:          "nextcloud",
			Name:        "Nextcloud",
			Description: "私有云盘，文件同步与共享",
			Category:    "productivity",
			Icon:        "☁️",
			YAMLContent: `version: "3"
services:
  nextcloud:
    image: nextcloud:latest
    ports:
      - "8080:80"
    volumes:
      - nextcloud_data:/var/www/html
    environment:
      - MYSQL_HOST=db
      - MYSQL_DATABASE=nextcloud
      - MYSQL_USER=nextcloud
      - MYSQL_PASSWORD={{.db_password}}
    restart: unless-stopped
  db:
    image: mariadb:10
    volumes:
      - db_data:/var/lib/mysql
    environment:
      - MYSQL_ROOT_PASSWORD={{.db_root_password}}
      - MYSQL_DATABASE=nextcloud
      - MYSQL_USER=nextcloud
      - MYSQL_PASSWORD={{.db_password}}
    restart: unless-stopped
volumes:
  nextcloud_data:
  db_data:`,
			Variables: []TemplateVar{
				{Name: "db_password", Description: "数据库密码", Type: "string", Required: true},
				{Name: "db_root_password", Description: "数据库root密码", Type: "string", Required: true},
			},
			Tags:  []string{"cloud", "storage", "sync"},
			Author: "nas-os",
			Version: "1.0.0",
		},
		{
			ID:          "jellyfin",
			Name:        "Jellyfin",
			Description: "开源媒体服务器，支持影视、音乐、有声书",
			Category:    "media",
			Icon:        "🎬",
			YAMLContent: `version: "3"
services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    ports:
      - "8096:8096"
    volumes:
      - jellyfin_config:/config
      - /mnt/media:/media
    restart: unless-stopped
volumes:
  jellyfin_config:`,
			Tags:  []string{"media", "streaming", "video"},
			Author: "nas-os",
			Version: "1.0.0",
		},
		{
			ID:          "homeassistant",
			Name:        "Home Assistant",
			Description: "智能家居平台，支持数千种设备",
			Category:    "iot",
			Icon:        "🏠",
			YAMLContent: `version: "3"
services:
  homeassistant:
    image: ghcr.io/home-assistant/home-assistant:stable
    ports:
      - "8123:8123"
    volumes:
      - ha_config:/config
    environment:
      - TZ={{.timezone}}
    network_mode: host
    restart: unless-stopped
volumes:
  ha_config:`,
			Variables: []TemplateVar{
				{Name: "timezone", Description: "时区", Type: "string", DefaultValue: "Asia/Shanghai", Required: false},
			},
			Tags:  []string{"iot", "smart-home", "automation"},
			Author: "nas-os",
			Version: "1.0.0",
		},
		{
			ID:          "immich",
			Name:        "Immich",
			Description: "自托管照片和视频备份方案，Google Photos替代",
			Category:    "media",
			Icon:        "📸",
			YAMLContent: `version: "3"
services:
  immich-server:
    image: ghcr.io/immich-app/immich-server:release
    ports:
      - "2283:2283"
    volumes:
      - immich_upload:/usr/src/app/upload
    environment:
      - DB_HOSTNAME=database
      - DB_USERNAME=postgres
      - DB_PASSWORD={{.db_password}}
      - DB_DATABASE_NAME=immich
    restart: unless-stopped
  database:
    image: tensorchord/pgvecto-rs:pg16-v0.2.0
    volumes:
      - db_data:/var/lib/postgresql/data
    environment:
      - POSTGRES_PASSWORD={{.db_password}}
      - POSTGRES_USER=postgres
      - POSTGRES_DB=immich
    restart: unless-stopped
volumes:
  immich_upload:
  db_data:`,
			Variables: []TemplateVar{
				{Name: "db_password", Description: "数据库密码", Type: "string", Required: true},
			},
			Tags:  []string{"photos", "backup", "ai"},
			Author: "nas-os",
			Version: "1.0.0",
		},
		{
			ID:          "vaultwarden",
			Name:        "Vaultwarden",
			Description: "Bitwarden兼容密码管理器，自托管",
			Category:    "security",
			Icon:        "🔐",
			YAMLContent: `version: "3"
services:
  vaultwarden:
    image: vaultwarden/server:latest
    ports:
      - "8880:80"
    volumes:
      - vw_data:/data
    environment:
      - ADMIN_TOKEN={{.admin_token}}
      - SIGNUPS_ALLOWED=false
    restart: unless-stopped
volumes:
  vw_data:`,
			Variables: []TemplateVar{
				{Name: "admin_token", Description: "管理Token", Type: "string", Required: true},
			},
			Tags:  []string{"password", "security", "self-hosted"},
			Author: "nas-os",
			Version: "1.0.0",
		},
	}

	for _, t := range templates {
		m.templates[t.ID] = t
	}
}

// ComposeHandlers Compose管理HTTP处理器
type ComposeHandlers struct {
	manager *ComposeManager
}

func NewComposeHandlers(manager *ComposeManager) *ComposeHandlers {
	return &ComposeHandlers{manager: manager}
}

func (h *ComposeHandlers) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/projects", h.handleProjects)
	mux.HandleFunc(prefix+"/projects/", h.handleProjectByID)
	mux.HandleFunc(prefix+"/templates", h.handleTemplates)
	mux.HandleFunc(prefix+"/templates/", h.handleTemplateByID)
	mux.HandleFunc(prefix+"/stats", h.handleStats)
}

func (h *ComposeHandlers) handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		projects := h.manager.ListProjects()
		json.NewEncoder(w).Encode(map[string]interface{}{"projects": projects})
	case http.MethodPost:
		var req struct {
			Name        string            `json:"name"`
			YAMLContent string            `json:"yaml_content"`
			EnvVars     map[string]string `json:"env_vars,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		project, err := h.manager.CreateProject(r.Context(), req.Name, req.YAMLContent, req.EnvVars)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(project)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ComposeHandlers) handleProjectByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/compose/projects/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing project ID", http.StatusBadRequest)
		return
	}
	projectID := parts[0]

	if len(parts) > 1 {
		action := parts[1]
		switch action {
		case "up":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := h.manager.Up(r.Context(), projectID, true); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "started"})
		case "down":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := h.manager.Down(r.Context(), projectID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
		case "restart":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if err := h.manager.Restart(r.Context(), projectID); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "restarted"})
		case "logs":
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			logs, err := h.manager.GetLogs(r.Context(), projectID, 100)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"logs": logs})
		default:
			http.Error(w, "Unknown action", http.StatusBadRequest)
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		project, err := h.manager.GetProject(projectID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(project)
	case http.MethodDelete:
		if err := h.manager.DeleteProject(r.Context(), projectID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *ComposeHandlers) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	templates := h.manager.ListTemplates()
	json.NewEncoder(w).Encode(map[string]interface{}{"templates": templates})
}

func (h *ComposeHandlers) handleTemplateByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/compose/templates/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing template ID", http.StatusBadRequest)
		return
	}
	templateID := parts[0]

	if len(parts) > 1 && parts[1] == "deploy" {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			ProjectName string            `json:"project_name"`
			Vars        map[string]string `json:"vars"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		project, err := h.manager.ImportFromTemplate(r.Context(), templateID, req.ProjectName, req.Vars)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(project)
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

func (h *ComposeHandlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.manager.GetStats())
}
