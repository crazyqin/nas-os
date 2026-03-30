// Package docker Docker Compose 管理模块
package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ComposeProject Docker Compose 项目.
type ComposeProject struct {
	Name        string           `json:"name"`
	ConfigPath  string           `json:"config_path"`
	Services    []ComposeService `json:"services"`
	Networks    []Network        `json:"networks"`
	Volumes     []Volume         `json:"volumes"`
	Status      string           `json:"status"` // running, stopped, error
	CreatedAt   time.Time        `json:"created_at"`
	Description string           `json:"description,omitempty"`
}

// ComposeService Compose 服务.
type ComposeService struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Ports       []PortMapping     `json:"ports"`
	Volumes     []VolumeMount     `json:"volumes"`
	Networks    []string          `json:"networks"`
	Environment map[string]string `json:"environment"`
	Status      string            `json:"status"`
	Health      string            `json:"health"`
	Replicas    int               `json:"replicas,omitempty"`
}

// ComposeTemplate Compose 模板.
type ComposeTemplate struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Category    string             `json:"category"`
	Content     string             `json:"content"`
	Variables   []TemplateVariable `json:"variables,omitempty"`
}

// TemplateVariable 模板变量.
type TemplateVariable struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     string `json:"default"`
	Required    bool   `json:"required"`
}

// ComposeManager Compose 管理器.
type ComposeManager struct {
	templatesDir string
}

// NewComposeManager 创建 Compose 管理器.
func NewComposeManager(templatesDir string) (*ComposeManager, error) {
	if templatesDir == "" {
		templatesDir = "/opt/nas/templates/compose"
	}

	// 确保模板目录存在
	if err := os.MkdirAll(templatesDir, 0755); err != nil {
		return nil, fmt.Errorf("创建模板目录失败: %w", err)
	}

	return &ComposeManager{
		templatesDir: templatesDir,
	}, nil
}

// ListProjects 列出所有 Compose 项目.
func (m *ComposeManager) ListProjects() ([]*ComposeProject, error) {
	// 使用 docker compose ls 命令
	cmd := exec.CommandContext(context.Background(), "docker", "compose", "ls", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		// 如果命令失败，尝试旧版 docker-compose
		cmd = exec.CommandContext(context.Background(), "docker-compose", "ls", "--format", "json")
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("列出Compose项目失败: %w", err)
		}
	}

	// 解析输出
	var rawProjects []struct {
		Name   string `json:"Name"`
		Status string `json:"Status"`
	}

	if err := json.Unmarshal(output, &rawProjects); err != nil {
		// 尝试逐行解析
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			var rp struct {
				Name   string `json:"Name"`
				Status string `json:"Status"`
			}
			if json.Unmarshal([]byte(line), &rp) == nil {
				rawProjects = append(rawProjects, rp)
			}
		}
	}

	var projects []*ComposeProject
	for _, rp := range rawProjects {
		project := &ComposeProject{
			Name:   rp.Name,
			Status: rp.Status,
		}

		// 获取项目详情
		details, err := m.GetProjectDetails(rp.Name)
		if err == nil {
			project.ConfigPath = details.ConfigPath
			project.Services = details.Services
			project.Networks = details.Networks
			project.Volumes = details.Volumes
			project.CreatedAt = details.CreatedAt
		}

		projects = append(projects, project)
	}

	return projects, nil
}

// GetProjectDetails 获取项目详情.
func (m *ComposeManager) GetProjectDetails(name string) (*ComposeProject, error) {
	// 使用 docker compose config 命令获取配置
	cmd := exec.CommandContext(context.Background(), "docker", "compose", "-p", name, "config", "--format", "json")
	output, err := cmd.Output()
	if err != nil {
		cmd = exec.CommandContext(context.Background(), "docker-compose", "-p", name, "config", "--format", "json")
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("获取项目配置失败: %w", err)
		}
	}

	// 解析配置
	var rawConfig struct {
		Services map[string]struct {
			Image       string              `json:"Image"`
			Ports       []map[string]string `json:"Ports"`
			Volumes     []map[string]string `json:"Volumes"`
			Networks    []string            `json:"Networks"`
			Environment map[string]string   `json:"Environment"`
		} `json:"Services"`
		Networks map[string]struct {
			Name   string `json:"Name"`
			Driver string `json:"Driver"`
		} `json:"Networks"`
		Volumes map[string]struct {
			Name string `json:"Name"`
		} `json:"Volumes"`
	}

	if err := json.Unmarshal(output, &rawConfig); err != nil {
		// 返回基本信息
		return &ComposeProject{
			Name:      name,
			CreatedAt: time.Now(),
		}, nil
	}

	project := &ComposeProject{
		Name:      name,
		CreatedAt: time.Now(),
		Services:  make([]ComposeService, 0),
		Networks:  make([]Network, 0),
		Volumes:   make([]Volume, 0),
	}

	// 解析服务
	for svcName, svc := range rawConfig.Services {
		service := ComposeService{
			Name:        svcName,
			Image:       svc.Image,
			Environment: svc.Environment,
			Networks:    svc.Networks,
			Ports:       make([]PortMapping, 0),
			Volumes:     make([]VolumeMount, 0),
		}

		// 解析端口
		for _, port := range svc.Ports {
			pm := PortMapping{Protocol: "tcp"}
			if target, ok := port["Target"]; ok {
				pm.ContainerPort = target
			}
			if published, ok := port["Published"]; ok {
				pm.HostPort = published
			}
			if protocol, ok := port["Protocol"]; ok {
				pm.Protocol = protocol
			}
			if hostIP, ok := port["HostIP"]; ok {
				pm.HostIP = hostIP
			}
			service.Ports = append(service.Ports, pm)
		}

		// 解析卷
		for _, vol := range svc.Volumes {
			vm := VolumeMount{RW: true}
			if source, ok := vol["Source"]; ok {
				vm.Source = source
			}
			if target, ok := vol["Target"]; ok {
				vm.Destination = target
			}
			if mode, ok := vol["Mode"]; ok {
				vm.Mode = mode
				vm.RW = !strings.Contains(mode, "ro")
			}
			service.Volumes = append(service.Volumes, vm)
		}

		// 获取服务状态
		serviceStatus, err := m.GetServiceStatus(name, svcName)
		if err == nil {
			service.Status = serviceStatus.Status
			service.Health = serviceStatus.Health
			service.Replicas = serviceStatus.Replicas
		}

		project.Services = append(project.Services, service)
	}

	// 解析网络
	for netName, net := range rawConfig.Networks {
		project.Networks = append(project.Networks, Network{
			Name:   netName,
			Driver: net.Driver,
		})
	}

	// 解析卷
	for volName := range rawConfig.Volumes {
		project.Volumes = append(project.Volumes, Volume{
			Name: volName,
		})
	}

	return project, nil
}

// ServiceStatus 服务状态.
type ServiceStatus struct {
	Status   string `json:"status"`
	Health   string `json:"health"`
	Replicas int    `json:"replicas"`
}

// GetServiceStatus 获取服务状态.
func (m *ComposeManager) GetServiceStatus(project, service string) (*ServiceStatus, error) {
	cmd := exec.CommandContext(context.Background(), "docker", "compose", "-p", project, "ps", "--format", "json", service)
	output, err := cmd.Output()
	if err != nil {
		cmd = exec.CommandContext(context.Background(), "docker-compose", "-p", project, "ps", "--format", "json", service)
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("获取服务状态失败: %w", err)
		}
	}

	var services []struct {
		Service string `json:"Service"`
		State   string `json:"State"`
		Health  string `json:"Health"`
	}

	if err := json.Unmarshal(output, &services); err != nil {
		scanner := bufio.NewScanner(strings.NewReader(string(output)))
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue
			}
			var s struct {
				Service string `json:"Service"`
				State   string `json:"State"`
				Health  string `json:"Health"`
			}
			if json.Unmarshal([]byte(line), &s) == nil {
				services = append(services, s)
			}
		}
	}

	status := &ServiceStatus{Replicas: 1}
	runningCount := 0
	for _, s := range services {
		if s.State == "running" {
			runningCount++
			status.Health = s.Health
		}
		status.Status = s.State
	}

	if len(services) > 1 {
		status.Replicas = len(services)
		if runningCount == len(services) {
			status.Status = "running"
		} else if runningCount == 0 {
			status.Status = "stopped"
		} else {
			status.Status = "partial"
		}
	}

	return status, nil
}

// Up 启动 Compose 项目.
func (m *ComposeManager) Up(configPath string, opts ComposeUpOptions) (*ComposeProject, error) {
	if configPath == "" {
		return nil, fmt.Errorf("配置文件路径不能为空")
	}

	// 检查文件是否存在
	if _, err := os.Stat(configPath); err != nil {
		return nil, fmt.Errorf("配置文件不存在: %w", err)
	}

	args := []string{"compose", "-f", configPath, "up", "-d"}

	// 添加项目名称
	if opts.Name != "" {
		args = append([]string{"compose", "-p", opts.Name, "-f", configPath, "up", "-d"},
			args[4:]...)
		args = []string{"compose", "-p", opts.Name, "-f", configPath, "up", "-d"}
	}

	// 添加其他选项
	if opts.Build {
		args = append(args, "--build")
	}
	if opts.RemoveOrphans {
		args = append(args, "--remove-orphans")
	}
	if opts.ForceRecreate {
		args = append(args, "--force-recreate")
	}

	cmd := exec.CommandContext(context.Background(), "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// 尝试旧版 docker-compose
		args[0] = "docker-compose"
		args = args[1:] // 移除 "compose"
		cmd = exec.CommandContext(context.Background(), args[0], args[1:]...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("启动Compose项目失败: %w, 输出: %s", err, string(output))
		}
	}

	// 获取项目名称
	projectName := opts.Name
	if projectName == "" {
		// 从配置文件路径提取项目名称
		projectName = filepath.Base(filepath.Dir(configPath))
		if projectName == "." || projectName == "" {
			projectName = strings.TrimSuffix(filepath.Base(configPath), ".yml")
			projectName = strings.TrimSuffix(projectName, ".yaml")
		}
	}

	// 返回项目信息
	return m.GetProjectDetails(projectName)
}

// ComposeUpOptions Compose Up 选项.
type ComposeUpOptions struct {
	Name          string `json:"name"`
	Build         bool   `json:"build"`
	RemoveOrphans bool   `json:"remove_orphans"`
	ForceRecreate bool   `json:"force_recreate"`
	Timeout       int    `json:"timeout"`
}

// Down 停止 Compose 项目.
func (m *ComposeManager) Down(name string, opts ComposeDownOptions) error {
	args := []string{"compose", "-p", name, "down"}

	if opts.RemoveVolumes {
		args = append(args, "-v")
	}
	if opts.RemoveImages {
		args = append(args, "--rmi", "all")
	}
	if opts.Timeout > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", opts.Timeout))
	}

	cmd := exec.CommandContext(context.Background(), "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		args[0] = "docker-compose"
		cmd = exec.CommandContext(context.Background(), args[0], args[1:]...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("停止Compose项目失败: %w, 输出: %s", err, string(output))
		}
	}

	return nil
}

// ComposeDownOptions Compose Down 选项.
type ComposeDownOptions struct {
	RemoveVolumes bool `json:"remove_volumes"`
	RemoveImages  bool `json:"remove_images"`
	Timeout       int  `json:"timeout"`
}

// Restart 重启 Compose 服务.
func (m *ComposeManager) Restart(project string, service string, timeout int) error {
	args := []string{"compose", "-p", project, "restart"}

	if service != "" {
		args = append(args, service)
	}

	if timeout > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", timeout))
	}

	cmd := exec.CommandContext(context.Background(), "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		args[0] = "docker-compose"
		cmd = exec.CommandContext(context.Background(), args[0], args[1:]...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("重启Compose服务失败: %w, 输出: %s", err, string(output))
		}
	}

	return nil
}

// Logs 获取 Compose 日志.
func (m *ComposeManager) Logs(project string, opts ComposeLogsOptions) (string, error) {
	args := []string{"compose", "-p", project, "logs"}

	if opts.Service != "" {
		args = append(args, opts.Service)
	}

	if opts.Tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", opts.Tail))
	}
	if opts.Since != "" {
		args = append(args, "--since", opts.Since)
	}
	if opts.Until != "" {
		args = append(args, "--until", opts.Until)
	}
	if opts.Timestamps {
		args = append(args, "-t")
	}

	cmd := exec.CommandContext(context.Background(), "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		args[0] = "docker-compose"
		cmd = exec.CommandContext(context.Background(), args[0], args[1:]...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("获取Compose日志失败: %w", err)
		}
	}

	return string(output), nil
}

// ComposeLogsOptions Compose 日志选项.
type ComposeLogsOptions struct {
	Service    string `json:"service"`
	Tail       int    `json:"tail"`
	Since      string `json:"since"`
	Until      string `json:"until"`
	Timestamps bool   `json:"timestamps"`
	Follow     bool   `json:"follow"`
}

// Scale 扩缩容 Compose 服务.
func (m *ComposeManager) Scale(project string, service string, replicas int) error {
	if replicas < 0 {
		return fmt.Errorf("副本数不能为负数")
	}

	args := []string{"compose", "-p", project, "up", "-d", "--scale", fmt.Sprintf("%s=%d", service, replicas)}

	cmd := exec.CommandContext(context.Background(), "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		args[0] = "docker-compose"
		cmd = exec.CommandContext(context.Background(), args[0], args[1:]...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("扩缩容Compose服务失败: %w, 输出: %s", err, string(output))
		}
	}

	return nil
}

// GetTemplates 获取所有模板.
func (m *ComposeManager) GetTemplates() ([]*ComposeTemplate, error) {
	files, err := os.ReadDir(m.templatesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*ComposeTemplate{}, nil
		}
		return nil, fmt.Errorf("读取模板目录失败: %w", err)
	}

	var templates []*ComposeTemplate
	for _, file := range files {
		if !file.IsDir() && (strings.HasSuffix(file.Name(), ".yml") || strings.HasSuffix(file.Name(), ".yaml")) {
			template, err := m.LoadTemplate(filepath.Join(m.templatesDir, file.Name()))
			if err == nil {
				templates = append(templates, template)
			}
		}
	}

	// 添加内置模板
	builtin := m.GetBuiltinTemplates()
	templates = append(templates, builtin...)

	return templates, nil
}

// LoadTemplate 加载模板.
func (m *ComposeManager) LoadTemplate(path string) (*ComposeTemplate, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取模板文件失败: %w", err)
	}

	name := strings.TrimSuffix(filepath.Base(path), ".yml")
	name = strings.TrimSuffix(name, ".yaml")

	return &ComposeTemplate{
		Name:    name,
		Content: string(content),
	}, nil
}

// GetBuiltinTemplates 获取内置模板.
func (m *ComposeManager) GetBuiltinTemplates() []*ComposeTemplate {
	return []*ComposeTemplate{
		{
			Name:        "nextcloud",
			Description: "私有云存储和协作平台",
			Category:    "Productivity",
			Variables: []TemplateVariable{
				{Name: "NEXTCLOUD_PORT", Description: "Web界面端口", Default: "8080", Required: false},
				{Name: "NEXTCLOUD_DATA", Description: "数据存储路径", Default: "/opt/nas/data/nextcloud", Required: false},
			},
			Content: `version: '3.8'
services:
  nextcloud:
    image: nextcloud:latest
    container_name: nextcloud
    restart: unless-stopped
    ports:
      - "${NEXTCLOUD_PORT:-8080}:80"
    volumes:
      - "${NEXTCLOUD_DATA:-/opt/nas/data/nextcloud}/data:/var/www/html/data"
      - "${NEXTCLOUD_DATA:-/opt/nas/data/nextcloud}/config:/var/www/html/config"
      - "${NEXTCLOUD_DATA:-/opt/nas/data/nextcloud}/apps:/var/www/html/custom_apps"
    environment:
      - NEXTCLOUD_TRUSTED_DOMAINS=localhost
`,
		},
		{
			Name:        "homeassistant",
			Description: "开源智能家居平台",
			Category:    "Home Automation",
			Variables: []TemplateVariable{
				{Name: "HA_PORT", Description: "Web界面端口", Default: "8123", Required: false},
				{Name: "HA_CONFIG", Description: "配置路径", Default: "/opt/nas/data/homeassistant", Required: false},
			},
			Content: `version: '3.8'
services:
  homeassistant:
    image: homeassistant/home-assistant:stable
    container_name: homeassistant
    restart: unless-stopped
    network_mode: host
    volumes:
      - "${HA_CONFIG:-/opt/nas/data/homeassistant}:/config"
    environment:
      - TZ=Asia/Shanghai
`,
		},
		{
			Name:        "jellyfin",
			Description: "开源媒体服务器",
			Category:    "Media",
			Variables: []TemplateVariable{
				{Name: "JELLYFIN_PORT", Description: "Web界面端口", Default: "8096", Required: false},
				{Name: "JELLYFIN_CONFIG", Description: "配置路径", Default: "/opt/nas/data/jellyfin", Required: false},
				{Name: "JELLYFIN_MEDIA", Description: "媒体路径", Default: "/opt/nas/media", Required: false},
			},
			Content: `version: '3.8'
services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    container_name: jellyfin
    restart: unless-stopped
    ports:
      - "${JELLYFIN_PORT:-8096}:8096"
    volumes:
      - "${JELLYFIN_CONFIG:-/opt/nas/data/jellyfin}/config:/config"
      - "${JELLYFIN_CONFIG:-/opt/nas/data/jellyfin}/cache:/cache"
      - "${JELLYFIN_MEDIA:-/opt/nas/media}:/media"
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
`,
		},
		{
			Name:        "plex",
			Description: "Plex媒体服务器",
			Category:    "Media",
			Variables: []TemplateVariable{
				{Name: "PLEX_PORT", Description: "Web界面端口", Default: "32400", Required: false},
				{Name: "PLEX_CONFIG", Description: "配置路径", Default: "/opt/nas/data/plex", Required: false},
				{Name: "PLEX_MEDIA", Description: "媒体路径", Default: "/opt/nas/media", Required: false},
				{Name: "PLEX_CLAIM", Description: "Plex claim token", Default: "", Required: false},
			},
			Content: `version: '3.8'
services:
  plex:
    image: plexinc/pms-docker:latest
    container_name: plex
    restart: unless-stopped
    ports:
      - "${PLEX_PORT:-32400}:32400"
    volumes:
      - "${PLEX_CONFIG:-/opt/nas/data/plex}/config:/config"
      - "${PLEX_CONFIG:-/opt/nas/data/plex}/transcode:/transcode"
      - "${PLEX_MEDIA:-/opt/nas/media}:/data"
    environment:
      - PLEX_CLAIM=${PLEX_CLAIM}
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
`,
		},
		{
			Name:        "pihole",
			Description: "网络广告拦截DNS服务器",
			Category:    "Network",
			Variables: []TemplateVariable{
				{Name: "PIHOLE_WEBPORT", Description: "Web界面端口", Default: "8080", Required: false},
				{Name: "PIHOLE_DNSPORT", Description: "DNS端口", Default: "53", Required: false},
				{Name: "PIHOLE_CONFIG", Description: "配置路径", Default: "/opt/nas/data/pihole", Required: false},
				{Name: "PIHOLE_PASSWORD", Description: "Web密码", Default: "admin", Required: false},
			},
			Content: `version: '3.8'
services:
  pihole:
    image: pihole/pihole:latest
    container_name: pihole
    restart: unless-stopped
    ports:
      - "${PIHOLE_DNSPORT:-53}:53/tcp"
      - "${PIHOLE_DNSPORT:-53}:53/udp"
      - "${PIHOLE_WEBPORT:-8080}:80"
    volumes:
      - "${PIHOLE_CONFIG:-/opt/nas/data/pihole}/etc:/etc/pihole"
      - "${PIHOLE_CONFIG:-/opt/nas/data/pihole}/dnsmasq:/etc/dnsmasq.d"
    environment:
      - TZ=Asia/Shanghai
      - WEBPASSWORD=${PIHOLE_PASSWORD:-admin}
`,
		},
		{
			Name:        "transmission",
			Description: "BitTorrent下载客户端",
			Category:    "Download",
			Variables: []TemplateVariable{
				{Name: "TRANS_PORT", Description: "Web界面端口", Default: "9091", Required: false},
				{Name: "TRANS_CONFIG", Description: "配置路径", Default: "/opt/nas/data/transmission", Required: false},
				{Name: "TRANS_DOWNLOADS", Description: "下载路径", Default: "/opt/nas/downloads", Required: false},
			},
			Content: `version: '3.8'
services:
  transmission:
    image: linuxserver/transmission:latest
    container_name: transmission
    restart: unless-stopped
    ports:
      - "${TRANS_PORT:-9091}:9091"
      - "51413:51413"
      - "51413:51413/udp"
    volumes:
      - "${TRANS_CONFIG:-/opt/nas/data/transmission}/config:/config"
      - "${TRANS_DOWNLOADS:-/opt/nas/downloads}:/downloads"
      - "${TRANS_DOWNLOADS:-/opt/nas/downloads}/watch:/watch"
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
`,
		},
		{
			Name:        "nginx-proxy",
			Description: "Nginx反向代理管理器",
			Category:    "Network",
			Variables: []TemplateVariable{
				{Name: "NPM_PORT", Description: "Web界面端口", Default: "81", Required: false},
				{Name: "NPM_HTTP_PORT", Description: "HTTP端口", Default: "80", Required: false},
				{Name: "NPM_HTTPS_PORT", Description: "HTTPS端口", Default: "443", Required: false},
				{Name: "NPM_DATA", Description: "数据路径", Default: "/opt/nas/data/nginx-proxy-manager", Required: false},
			},
			Content: `version: '3.8'
services:
  nginx-proxy-manager:
    image: jc21/nginx-proxy-manager:latest
    container_name: nginx-proxy-manager
    restart: unless-stopped
    ports:
      - "${NPM_HTTP_PORT:-80}:80"
      - "${NPM_HTTPS_PORT:-443}:443"
      - "${NPM_PORT:-81}:81"
    volumes:
      - "${NPM_DATA:-/opt/nas/data/nginx-proxy-manager}/data:/data"
      - "${NPM_DATA:-/opt/nas/data/nginx-proxy-manager}/letsencrypt:/etc/letsencrypt"
    environment:
      - DB_SQLITE_FILE="/data/database.sqlite"
`,
		},
		{
			Name:        "vaultwarden",
			Description: "Bitwarden密码管理服务器",
			Category:    "Security",
			Variables: []TemplateVariable{
				{Name: "VW_PORT", Description: "Web界面端口", Default: "8080", Required: false},
				{Name: "VW_DATA", Description: "数据路径", Default: "/opt/nas/data/vaultwarden", Required: false},
			},
			Content: `version: '3.8'
services:
  vaultwarden:
    image: vaultwarden/server:latest
    container_name: vaultwarden
    restart: unless-stopped
    ports:
      - "${VW_PORT:-8080}:80"
    volumes:
      - "${VW_DATA:-/opt/nas/data/vaultwarden}:/data"
    environment:
      - SIGNUPS_ALLOWED=true
      - INVITATIONS_ALLOWED=true
      - TZ=Asia/Shanghai
`,
		},
	}
}

// SaveTemplate 保存模板.
func (m *ComposeManager) SaveTemplate(template *ComposeTemplate) error {
	if template.Name == "" {
		return fmt.Errorf("模板名称不能为空")
	}

	filename := fmt.Sprintf("%s.yml", template.Name)
	path := filepath.Join(m.templatesDir, filename)

	return os.WriteFile(path, []byte(template.Content), 0644)
}

// ExportTemplate 导出模板.
func (m *ComposeManager) ExportTemplate(name string, destPath string) error {
	template, err := m.GetTemplate(name)
	if err != nil {
		return err
	}

	if destPath == "" {
		destPath = fmt.Sprintf("%s.yml", name)
	}

	return os.WriteFile(destPath, []byte(template.Content), 0644)
}

// GetTemplate 获取指定模板.
func (m *ComposeManager) GetTemplate(name string) (*ComposeTemplate, error) {
	// 先检查文件模板
	path := filepath.Join(m.templatesDir, fmt.Sprintf("%s.yml", name))
	if _, err := os.Stat(path); err == nil {
		return m.LoadTemplate(path)
	}

	path = filepath.Join(m.templatesDir, fmt.Sprintf("%s.yaml", name))
	if _, err := os.Stat(path); err == nil {
		return m.LoadTemplate(path)
	}

	// 检查内置模板
	for _, t := range m.GetBuiltinTemplates() {
		if t.Name == name {
			return t, nil
		}
	}

	return nil, fmt.Errorf("模板不存在: %s", name)
}

// ImportTemplate 导入模板.
func (m *ComposeManager) ImportTemplate(name string, sourcePath string) (*ComposeTemplate, error) {
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("读取源文件失败: %w", err)
	}

	template := &ComposeTemplate{
		Name:    name,
		Content: string(content),
	}

	if err := m.SaveTemplate(template); err != nil {
		return nil, err
	}

	return template, nil
}

// DeleteTemplate 删除模板.
func (m *ComposeManager) DeleteTemplate(name string) error {
	// 不能删除内置模板
	for _, t := range m.GetBuiltinTemplates() {
		if t.Name == name {
			return fmt.Errorf("不能删除内置模板")
		}
	}

	path := filepath.Join(m.templatesDir, fmt.Sprintf("%s.yml", name))
	if err := os.Remove(path); err != nil {
		path = filepath.Join(m.templatesDir, fmt.Sprintf("%s.yaml", name))
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("删除模板失败: %w", err)
		}
	}

	return nil
}

// ApplyTemplate 应用模板启动项目.
func (m *ComposeManager) ApplyTemplate(templateName string, variables map[string]string, projectPath string) (*ComposeProject, error) {
	template, err := m.GetTemplate(templateName)
	if err != nil {
		return nil, err
	}

	// 替换变量
	content := template.Content
	for k, v := range variables {
		content = strings.ReplaceAll(content, fmt.Sprintf("${%s}", k), v)
		// 替换带默认值的变量
		pattern := fmt.Sprintf("${%s:-", k)
		if strings.Contains(content, pattern) {
			// 复杂替换：${VAR:-default} -> value
			start := strings.Index(content, pattern)
			if start >= 0 {
				end := strings.Index(content[start:], "}")
				if end >= 0 {
					content = content[:start] + v + content[start+end+1:]
				}
			}
		}
	}

	// 保存配置文件
	configPath := filepath.Join(projectPath, fmt.Sprintf("%s.yml", templateName))
	if err := os.MkdirAll(projectPath, 0755); err != nil {
		return nil, fmt.Errorf("创建项目目录失败: %w", err)
	}

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		return nil, fmt.Errorf("保存配置文件失败: %w", err)
	}

	// 启动项目
	return m.Up(configPath, ComposeUpOptions{
		Name: templateName,
	})
}

// ValidateConfig 验证配置文件.
func (m *ComposeManager) ValidateConfig(configPath string) (string, error) {
	args := []string{"compose", "-f", configPath, "config", "--quiet"}

	cmd := exec.CommandContext(context.Background(), "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		args[0] = "docker-compose"
		cmd = exec.CommandContext(context.Background(), args[0], args[1:]...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return string(output), fmt.Errorf("配置验证失败: %w", err)
		}
	}

	return "配置验证通过", nil
}
