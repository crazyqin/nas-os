package docker

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// AppTemplate 应用模板
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

// PortConfig 端口配置
type PortConfig struct {
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	Description string `json:"description"`
	Default     int    `json:"default"` // 默认主机端口
}

// VolumeConfig 卷配置
type VolumeConfig struct {
	ContainerPath string `json:"containerPath"`
	Description   string `json:"description"`
	Default       string `json:"default"` // 默认主机路径
}

// InstalledApp 已安装应用
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

// AppStore 应用商店
type AppStore struct {
	mu          sync.RWMutex
	manager     *Manager
	templateDir string
	installDir  string
	dataFile    string
	templates   map[string]*AppTemplate
	installed   map[string]*InstalledApp
}

// NewAppStore 创建应用商店
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

// loadBuiltinTemplates 加载内置模板
func (s *AppStore) loadBuiltinTemplates() {
	templates := []*AppTemplate{
		{
			ID:          "nextcloud",
			Name:        "nextcloud",
			DisplayName: "Nextcloud",
			Description: "私有云存储服务，支持文件同步、分享、在线办公",
			Category:    "Productivity",
			Icon:        "☁️",
			Version:     "latest",
			Image:       "nextcloud:latest",
			Ports: []PortConfig{
				{Port: 80, Protocol: "tcp", Description: "Web 界面", Default: 8080},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/var/www/html", Description: "数据目录", Default: "/opt/nas/apps/nextcloud/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  nextcloud:
    image: nextcloud:latest
    container_name: nextcloud
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:80"
    volumes:
      - {{.DataDir}}:/var/www/html
    environment:
      - NEXTCLOUD_TRUSTED_DOMAINS={{.TrustedDomains}}
`,
			Notes:   "首次访问需要创建管理员账户",
			Website: "https://nextcloud.com",
			Source:  "https://github.com/nextcloud",
		},
		{
			ID:          "jellyfin",
			Name:        "jellyfin",
			DisplayName: "Jellyfin",
			Description: "开源媒体服务器，支持电影、电视剧、音乐播放",
			Category:    "Media",
			Icon:        "🎬",
			Version:     "latest",
			Image:       "jellyfin/jellyfin:latest",
			Ports: []PortConfig{
				{Port: 8096, Protocol: "tcp", Description: "Web 界面", Default: 8096},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/jellyfin/config"},
				{ContainerPath: "/cache", Description: "缓存目录", Default: "/opt/nas/apps/jellyfin/cache"},
				{ContainerPath: "/media", Description: "媒体目录", Default: "/opt/nas/media"},
			},
			Environment: map[string]string{
				"PUID": "1000",
				"PGID": "1000",
			},
			Compose: `version: '3'
services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    container_name: jellyfin
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:8096"
    volumes:
      - {{.ConfigDir}}:/config
      - {{.CacheDir}}:/cache
      - {{.MediaDir}}:/media
    environment:
      - PUID=1000
      - PGID=1000
`,
			Notes:   "首次启动需要设置媒体库",
			Website: "https://jellyfin.org",
			Source:  "https://github.com/jellyfin/jellyfin",
		},
		{
			ID:          "homeassistant",
			Name:        "homeassistant",
			DisplayName: "Home Assistant",
			Description: "开源智能家居平台，支持数千种设备集成",
			Category:    "Smart Home",
			Icon:        "🏠",
			Version:     "stable",
			Image:       "homeassistant/home-assistant:stable",
			Ports: []PortConfig{
				{Port: 8123, Protocol: "tcp", Description: "Web 界面", Default: 8123},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/homeassistant/config"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  homeassistant:
    image: homeassistant/home-assistant:stable
    container_name: homeassistant
    restart: unless-stopped
    privileged: true
    network_mode: host
    volumes:
      - {{.ConfigDir}}:/config
      - /etc/localtime:/etc/localtime:ro
`,
			Notes:   "使用 host 网络模式以支持设备发现",
			Website: "https://www.home-assistant.io",
			Source:  "https://github.com/home-assistant/core",
		},
		{
			ID:          "pihole",
			Name:        "pihole",
			DisplayName: "Pi-hole",
			Description: "网络级广告拦截器，DNS 服务器",
			Category:    "Network",
			Icon:        "🛡️",
			Version:     "latest",
			Image:       "pihole/pihole:latest",
			Ports: []PortConfig{
				{Port: 53, Protocol: "tcp", Description: "DNS (TCP)", Default: 53},
				{Port: 53, Protocol: "udp", Description: "DNS (UDP)", Default: 53},
				{Port: 80, Protocol: "tcp", Description: "Web 界面", Default: 8081},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/etc/pihole", Description: "配置目录", Default: "/opt/nas/apps/pihole/etc"},
				{ContainerPath: "/etc/dnsmasq.d", Description: "DNS 配置", Default: "/opt/nas/apps/pihole/dnsmasq"},
			},
			Environment: map[string]string{
				"TZ": "Asia/Shanghai",
			},
			Compose: `version: '3'
services:
  pihole:
    image: pihole/pihole:latest
    container_name: pihole
    restart: unless-stopped
    ports:
      - "{{.DNSPort}}:53/tcp"
      - "{{.DNSPort}}:53/udp"
      - "{{.WebPort}}:80"
    volumes:
      - {{.ConfigDir}}:/etc/pihole
      - {{.DnsmasqDir}}:/etc/dnsmasq.d
    environment:
      - TZ=Asia/Shanghai
      - WEBPASSWORD={{.WebPassword}}
`,
			Notes:   "设置路由器 DNS 指向 Pi-hole 以全局拦截广告",
			Website: "https://pi-hole.net",
			Source:  "https://github.com/pi-hole/pi-hole",
		},
		{
			ID:          "transmission",
			Name:        "transmission",
			DisplayName: "Transmission",
			Description: "轻量级 BitTorrent 客户端",
			Category:    "Download",
			Icon:        "📥",
			Version:     "latest",
			Image:       "linuxserver/transmission:latest",
			Ports: []PortConfig{
				{Port: 9091, Protocol: "tcp", Description: "Web 界面", Default: 9091},
				{Port: 51413, Protocol: "tcp", Description: "BT 端口", Default: 51413},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/transmission/config"},
				{ContainerPath: "/downloads", Description: "下载目录", Default: "/opt/nas/downloads"},
			},
			Environment: map[string]string{
				"PUID": "1000",
				"PGID": "1000",
				"TZ":   "Asia/Shanghai",
			},
			Compose: `version: '3'
services:
  transmission:
    image: linuxserver/transmission:latest
    container_name: transmission
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:9091"
      - "{{.BTPort}}:51413"
      - "{{.BTPort}}:51413/udp"
    volumes:
      - {{.ConfigDir}}:/config
      - {{.DownloadDir}}:/downloads
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
`,
			Notes:   "默认用户名/密码: admin/admin",
			Website: "https://transmissionbt.com",
			Source:  "https://github.com/transmission/transmission",
		},
		{
			ID:          "syncthing",
			Name:        "syncthing",
			DisplayName: "Syncthing",
			Description: "开源文件同步工具，支持多设备同步",
			Category:    "Productivity",
			Icon:        "🔄",
			Version:     "latest",
			Image:       "syncthing/syncthing:latest",
			Ports: []PortConfig{
				{Port: 8384, Protocol: "tcp", Description: "Web 界面", Default: 8384},
				{Port: 22000, Protocol: "tcp", Description: "同步端口", Default: 22000},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/var/syncthing", Description: "数据目录", Default: "/opt/nas/apps/syncthing/data"},
			},
			Environment: map[string]string{
				"PUID": "1000",
				"PGID": "1000",
			},
			Compose: `version: '3'
services:
  syncthing:
    image: syncthing/syncthing:latest
    container_name: syncthing
    restart: unless-stopped
    hostname: nas-syncthing
    ports:
      - "{{.WebPort}}:8384"
      - "{{.SyncPort}}:22000/tcp"
      - "{{.SyncPort}}:22000/udp"
    volumes:
      - {{.DataDir}}:/var/syncthing
    environment:
      - PUID=1000
      - PGID=1000
`,
			Notes:   "首次访问需要设置用户名密码",
			Website: "https://syncthing.net",
			Source:  "https://github.com/syncthing/syncthing",
		},
		{
			ID:          "gitea",
			Name:        "gitea",
			DisplayName: "Gitea",
			Description: "轻量级 Git 服务，自建代码仓库",
			Category:    "Development",
			Icon:        "🐙",
			Version:     "latest",
			Image:       "gitea/gitea:latest",
			Ports: []PortConfig{
				{Port: 3000, Protocol: "tcp", Description: "Web 界面", Default: 3000},
				{Port: 22, Protocol: "tcp", Description: "SSH", Default: 2222},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/gitea/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  gitea:
    image: gitea/gitea:latest
    container_name: gitea
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:3000"
      - "{{.SSHPort}}:22"
    volumes:
      - {{.DataDir}}:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
`,
			Notes:   "首次访问需要完成安装向导",
			Website: "https://gitea.io",
			Source:  "https://github.com/go-gitea/gitea",
		},
		{
			ID:          "vaultwarden",
			Name:        "vaultwarden",
			DisplayName: "Vaultwarden",
			Description: "Bitwarden 密码管理器服务端，管理所有密码",
			Category:    "Security",
			Icon:        "🔐",
			Version:     "latest",
			Image:       "vaultwarden/server:latest",
			Ports: []PortConfig{
				{Port: 80, Protocol: "tcp", Description: "Web 界面", Default: 8082},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/vaultwarden/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  vaultwarden:
    image: vaultwarden/server:latest
    container_name: vaultwarden
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:80"
    volumes:
      - {{.DataDir}}:/data
    environment:
      - SIGNUPS_ALLOWED=true
`,
			Notes:   "建议配合 HTTPS 使用，可搭配 Bitwarden 客户端",
			Website: "https://github.com/dani-garcia/vaultwarden",
			Source:  "https://github.com/dani-garcia/vaultwarden",
		},
		{
			ID:          "immich",
			Name:        "immich",
			DisplayName: "Immich",
			Description: "自托管照片和视频备份方案，类似 Google Photos",
			Category:    "Media",
			Icon:        "📸",
			Version:     "latest",
			Image:       "ghcr.io/immich-app/immich-server:latest",
			Ports: []PortConfig{
				{Port: 2283, Protocol: "tcp", Description: "Web 界面", Default: 2283},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/usr/src/app/upload", Description: "上传目录", Default: "/opt/nas/apps/immich/upload"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  immich-server:
    image: ghcr.io/immich-app/immich-server:latest
    container_name: immich-server
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:3001"
    volumes:
      - {{.UploadDir}}:/usr/src/app/upload
      - /etc/localtime:/etc/localtime:ro
    environment:
      - DB_HOSTNAME=immich-db
      - DB_USERNAME=postgres
      - DB_PASSWORD=postgres
      - DB_DATABASE_NAME=immich
      - REDIS_HOSTNAME=immich-redis
    depends_on:
      - immich-db
      - immich-redis

  immich-db:
    image: tensorchord/pgvecto-rs:pg14-v0.2.0
    container_name: immich-db
    restart: unless-stopped
    volumes:
      - /opt/nas/apps/immich/db:/var/lib/postgresql/data
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=immich

  immich-redis:
    image: redis:6.2-alpine
    container_name: immich-redis
    restart: unless-stopped
`,
			Notes:   "首次访问需要创建管理员账户，需要较多资源",
			Website: "https://immich.app",
			Source:  "https://github.com/immich-app/immich",
		},
		{
			ID:          "nginxproxymanager",
			Name:        "nginxproxymanager",
			DisplayName: "Nginx Proxy Manager",
			Description: "反向代理管理界面，SSL 证书自动申请",
			Category:    "Network",
			Icon:        "🌐",
			Version:     "latest",
			Image:       "jc21/nginx-proxy-manager:latest",
			Ports: []PortConfig{
				{Port: 80, Protocol: "tcp", Description: "HTTP", Default: 80},
				{Port: 443, Protocol: "tcp", Description: "HTTPS", Default: 443},
				{Port: 81, Protocol: "tcp", Description: "管理界面", Default: 8181},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/npm/data"},
				{ContainerPath: "/etc/letsencrypt", Description: "证书目录", Default: "/opt/nas/apps/npm/letsencrypt"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  nginx-proxy-manager:
    image: jc21/nginx-proxy-manager:latest
    container_name: nginx-proxy-manager
    restart: unless-stopped
    ports:
      - "{{.HTTPPort}}:80"
      - "{{.HTTPSPort}}:443"
      - "{{.WebPort}}:81"
    volumes:
      - {{.DataDir}}:/data
      - {{.CertsDir}}:/etc/letsencrypt
`,
			Notes:   "默认登录: admin@example.com / changeme",
			Website: "https://nginxproxymanager.com",
			Source:  "https://github.com/NginxProxyManager/nginx-proxy-manager",
		},
		{
			ID:          "portainer",
			Name:        "portainer",
			DisplayName: "Portainer",
			Description: "Docker 容器管理界面，可视化管理容器",
			Category:    "Development",
			Icon:        "🐳",
			Version:     "latest",
			Image:       "portainer/portainer-ce:latest",
			Ports: []PortConfig{
				{Port: 9443, Protocol: "tcp", Description: "Web 界面", Default: 9443},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/portainer/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  portainer:
    image: portainer/portainer-ce:latest
    container_name: portainer
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:9443"
    volumes:
      - {{.DataDir}}:/data
      - /var/run/docker.sock:/var/run/docker.sock
`,
			Notes:   "首次访问需要设置管理员密码",
			Website: "https://www.portainer.io",
			Source:  "https://github.com/portainer/portainer",
		},
	}

	for _, t := range templates {
		s.templates[t.ID] = t
	}
}

// loadInstalled 加载已安装应用
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

// saveInstalled 保存已安装应用
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

// ListTemplates 列出所有模板
func (s *AppStore) ListTemplates() []*AppTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]*AppTemplate, 0, len(s.templates))
	for _, t := range s.templates {
		result = append(result, t)
	}
	return result
}

// GetTemplate 获取模板
func (s *AppStore) GetTemplate(id string) *AppTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.templates[id]
}

// ListInstalled 列出已安装应用
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

// GetInstalled 获取已安装应用
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

// InstallApp 安装应用
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
	cmd := exec.Command("docker-compose", "-f", composePath, "up", "-d")
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

// renderCompose 渲染 Docker Compose 模板
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

// generateDefaultCompose 生成默认 compose
func (s *AppStore) generateDefaultCompose(template *AppTemplate, config map[string]interface{}) string {
	var ports []string
	for _, port := range template.Ports {
		hostPort := port.Default
		if val, ok := config[fmt.Sprintf("port_%d", port.Port)].(float64); ok {
			hostPort = int(val)
		}
		ports = append(ports, fmt.Sprintf("      - \"%d:%d\"", hostPort, port.Port))
	}

	var volumes []string
	for _, vol := range template.Volumes {
		hostPath := vol.Default
		if val, ok := config[fmt.Sprintf("vol_%s", strings.ReplaceAll(vol.ContainerPath, "/", "_"))].(string); ok && val != "" {
			hostPath = val
		}
		volumes = append(volumes, fmt.Sprintf("      - %s:%s", hostPath, vol.ContainerPath))
	}

	var env []string
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

// UninstallApp 卸载应用
func (s *AppStore) UninstallApp(id string, removeData bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	// 停止并删除容器
	if app.ComposePath != "" {
		cmd := exec.Command("docker-compose", "-f", app.ComposePath, "down")
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

// StartApp 启动应用
func (s *AppStore) StartApp(id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	if app.ComposePath != "" {
		cmd := exec.Command("docker-compose", "-f", app.ComposePath, "start")
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

// StopApp 停止应用
func (s *AppStore) StopApp(id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	if app.ComposePath != "" {
		cmd := exec.Command("docker-compose", "-f", app.ComposePath, "stop")
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

// RestartApp 重启应用
func (s *AppStore) RestartApp(id string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	if app.ComposePath != "" {
		cmd := exec.Command("docker-compose", "-f", app.ComposePath, "restart")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("重启失败: %w, %s", err, string(output))
		}
	} else {
		return s.manager.RestartContainer(app.ContainerID, 10)
	}

	return nil
}

// UpdateApp 更新应用
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
		cmd := exec.Command("docker-compose", "-f", app.ComposePath, "up", "-d")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("更新失败: %w, %s", err, string(output))
		}
	}

	return nil
}

// GetAppStats 获取应用统计
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

// TemplateVersion 模板版本信息
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

// TemplateVersionManager 模板版本管理器
type TemplateVersionManager struct {
	mu       sync.RWMutex
	store    *AppStore
	dataDir  string
	versions map[string][]*TemplateVersion // templateID -> versions
}

// NewTemplateVersionManager 创建模板版本管理器
func NewTemplateVersionManager(store *AppStore, dataDir string) (*TemplateVersionManager, error) {
	tvm := &TemplateVersionManager{
		store:    store,
		dataDir:  dataDir,
		versions: make(map[string][]*TemplateVersion),
	}

	// 加载版本数据
	if err := tvm.load(); err != nil {
		fmt.Printf("加载模板版本数据失败: %v\n", err)
	}

	return tvm, nil
}

// load 加载版本数据
func (tvm *TemplateVersionManager) load() error {
	dataFile := filepath.Join(tvm.dataDir, "template-versions.json")
	data, err := os.ReadFile(dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在是正常的
		}
		return err
	}

	return json.Unmarshal(data, &tvm.versions)
}

// save 保存版本数据
func (tvm *TemplateVersionManager) save() error {
	dataFile := filepath.Join(tvm.dataDir, "template-versions.json")
	data, err := json.MarshalIndent(tvm.versions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(dataFile, data, 0640)
}

// AddVersion 添加模板版本
func (tvm *TemplateVersionManager) AddVersion(templateID string, version *TemplateVersion) error {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	version.TemplateID = templateID
	version.ID = fmt.Sprintf("%s-%s", templateID, version.Version)

	// 检查版本是否已存在
	for _, v := range tvm.versions[templateID] {
		if v.Version == version.Version {
			return fmt.Errorf("版本已存在: %s", version.Version)
		}
	}

	tvm.versions[templateID] = append(tvm.versions[templateID], version)
	return tvm.save()
}

// GetVersions 获取模板的所有版本
func (tvm *TemplateVersionManager) GetVersions(templateID string) []*TemplateVersion {
	tvm.mu.RLock()
	defer tvm.mu.RUnlock()

	versions := tvm.versions[templateID]
	if versions == nil {
		return []*TemplateVersion{}
	}

	// 返回副本
	result := make([]*TemplateVersion, len(versions))
	copy(result, versions)
	return result
}

// GetLatestVersion 获取最新版本
func (tvm *TemplateVersionManager) GetLatestVersion(templateID string) *TemplateVersion {
	tvm.mu.RLock()
	defer tvm.mu.RUnlock()

	versions := tvm.versions[templateID]
	if len(versions) == 0 {
		return nil
	}

	// 返回最新的非弃用版本
	for i := len(versions) - 1; i >= 0; i-- {
		if !versions[i].Deprecated {
			return versions[i]
		}
	}

	return versions[len(versions)-1]
}

// GetVersion 获取指定版本
func (tvm *TemplateVersionManager) GetVersion(templateID, version string) *TemplateVersion {
	tvm.mu.RLock()
	defer tvm.mu.RUnlock()

	for _, v := range tvm.versions[templateID] {
		if v.Version == version {
			return v
		}
	}

	return nil
}

// DeprecateVersion 标记版本为弃用
func (tvm *TemplateVersionManager) DeprecateVersion(templateID, version string) error {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	for _, v := range tvm.versions[templateID] {
		if v.Version == version {
			v.Deprecated = true
			return tvm.save()
		}
	}

	return fmt.Errorf("版本不存在: %s", version)
}

// RemoveVersion 移除版本
func (tvm *TemplateVersionManager) RemoveVersion(templateID, version string) error {
	tvm.mu.Lock()
	defer tvm.mu.Unlock()

	versions := tvm.versions[templateID]
	for i, v := range versions {
		if v.Version == version {
			tvm.versions[templateID] = append(versions[:i], versions[i+1:]...)
			return tvm.save()
		}
	}

	return fmt.Errorf("版本不存在: %s", version)
}

// =============================================================================
// 应用更新检测增强
// =============================================================================

// UpdateInfo 更新信息
type UpdateInfo struct {
	AppID           string    `json:"appId"`
	AppName         string    `json:"appName"`
	CurrentVersion  string    `json:"currentVersion"`
	LatestVersion   string    `json:"latestVersion"`
	CurrentDigest   string    `json:"currentDigest"`
	LatestDigest    string    `json:"latestDigest"`
	HasUpdate       bool      `json:"hasUpdate"`
	ReleaseNotes    string    `json:"releaseNotes"`
	PublishedAt     time.Time `json:"publishedAt"`
	ImageSize       int64     `json:"imageSize"`
	CheckTime       time.Time `json:"checkTime"`
	AutoUpdate      bool      `json:"autoUpdate"`
	IgnoreUntil     time.Time `json:"ignoreUntil"`
}

// UpdateChecker 更新检测器
type UpdateChecker struct {
	mu          sync.RWMutex
	store       *AppStore
	httpClient  *http.Client
	registry    string // Docker Registry 地址
	credentials map[string]RegistryCredential
}

// RegistryCredential Registry 凭证
type RegistryCredential struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// NewUpdateChecker 创建更新检测器
func NewUpdateChecker(store *AppStore) *UpdateChecker {
	return &UpdateChecker{
		store: store,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		registry:    "https://registry.hub.docker.com",
		credentials: make(map[string]RegistryCredential),
	}
}

// SetRegistry 设置 Registry 地址
func (uc *UpdateChecker) SetRegistry(registry string) {
	uc.registry = registry
}

// SetCredential 设置 Registry 凭证
func (uc *UpdateChecker) SetCredential(registry string, cred RegistryCredential) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.credentials[registry] = cred
}

// CheckAppUpdate 检查单个应用的更新
func (uc *UpdateChecker) CheckAppUpdate(appID string) (*UpdateInfo, error) {
	app := uc.store.GetInstalled(appID)
	if app == nil {
		return nil, fmt.Errorf("应用未安装: %s", appID)
	}

	template := uc.store.GetTemplate(app.TemplateID)
	if template == nil {
		return nil, fmt.Errorf("模板不存在: %s", app.TemplateID)
	}

	return uc.CheckImageUpdate(template.Image, app.Version)
}

// CheckImageUpdate 检查镜像更新
func (uc *UpdateChecker) CheckImageUpdate(image, currentTag string) (*UpdateInfo, error) {
	// 解析镜像名称
	imageName, tag := parseImageName(image)
	if tag != "" && tag != "latest" {
		currentTag = tag
	}

	// 获取镜像信息
	imageInfo, err := uc.fetchImageInfo(imageName)
	if err != nil {
		return nil, fmt.Errorf("获取镜像信息失败: %w", err)
	}

	// 查找当前版本
	var currentDigest string
	for _, t := range imageInfo.Tags {
		if t.Name == currentTag {
			currentDigest = t.Digest
			break
		}
	}

	// 查找最新版本
	latestTag := findLatestStableTag(imageInfo.Tags)
	if latestTag == nil && len(imageInfo.Tags) > 0 {
		latestTag = &imageInfo.Tags[0]
	}

	if latestTag == nil {
		return nil, fmt.Errorf("未找到可用版本")
	}

	// 构建更新信息
	info := &UpdateInfo{
		CurrentVersion: currentTag,
		LatestVersion:  latestTag.Name,
		CurrentDigest:  currentDigest,
		LatestDigest:   latestTag.Digest,
		HasUpdate:      currentDigest != "" && currentDigest != latestTag.Digest,
		ReleaseNotes:   latestTag.ReleaseNotes,
		PublishedAt:    latestTag.LastUpdated,
		ImageSize:      latestTag.FullSize,
		CheckTime:      time.Now(),
	}

	return info, nil
}

// ImageInfo 镜像信息
type ImageInfo struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Tags        []TagInfo  `json:"tags"`
}

// TagInfo 标签信息
type TagInfo struct {
	Name          string    `json:"name"`
	Digest        string    `json:"digest"`
	FullSize      int64     `json:"full_size"`
	LastUpdated   time.Time `json:"last_updated"`
	ReleaseNotes  string    `json:"release_notes"`
}

// fetchImageInfo 获取镜像信息
func (uc *UpdateChecker) fetchImageInfo(imageName string) (*ImageInfo, error) {
	namespace, name := parseImageNamespace(imageName)

	// 构建 Docker Hub API URL
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags/?page_size=100", namespace, name)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 添加认证（如果有）
	if cred, ok := uc.credentials["docker.io"]; ok {
		req.SetBasicAuth(cred.Username, cred.Password)
	}

	resp, err := uc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry 返回 %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Name        string    `json:"name"`
			FullSize    int64     `json:"full_size"`
			LastUpdated string    `json:"last_updated"`
			Images      []struct {
				Digest string `json:"digest"`
			} `json:"images"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// 转换结果
	info := &ImageInfo{
		Name:  imageName,
		Tags:  make([]TagInfo, 0, len(result.Results)),
	}

	for _, r := range result.Results {
		var digest string
		if len(r.Images) > 0 {
			digest = r.Images[0].Digest
		}

		lastUpdated, _ := time.Parse(time.RFC3339, r.LastUpdated)

		info.Tags = append(info.Tags, TagInfo{
			Name:        r.Name,
			Digest:      digest,
			FullSize:    r.FullSize,
			LastUpdated: lastUpdated,
		})
	}

	return info, nil
}

// parseImageName 解析镜像名称
func parseImageName(image string) (string, string) {
	parts := strings.SplitN(image, ":", 2)
	if len(parts) == 1 {
		return parts[0], "latest"
	}
	return parts[0], parts[1]
}

// parseImageNamespace 解析镜像命名空间
func parseImageNamespace(imageName string) (string, string) {
	if strings.Contains(imageName, "/") {
		parts := strings.SplitN(imageName, "/", 2)
		return parts[0], parts[1]
	}
	return "library", imageName
}

// findLatestStableTag 查找最新稳定版本标签
func findLatestStableTag(tags []TagInfo) *TagInfo {
	// 优先选择非 latest、非预发布版本
	for _, t := range tags {
		if t.Name == "latest" {
			continue
		}
		// 跳过预发布版本（包含 -alpha, -beta, -rc 等）
		if strings.Contains(t.Name, "-alpha") ||
			strings.Contains(t.Name, "-beta") ||
			strings.Contains(t.Name, "-rc") ||
			strings.Contains(t.Name, "-preview") {
			continue
		}
		return &t
	}

	// 如果没有稳定版本，返回第一个非 latest 版本
	for _, t := range tags {
		if t.Name != "latest" {
			return &t
		}
	}

	return nil
}

// =============================================================================
// 应用备份与恢复
// =============================================================================

// BackupInfo 备份信息
type BackupInfo struct {
	ID          string            `json:"id"`
	AppID       string            `json:"appId"`
	AppName     string            `json:"appName"`
	Version     string            `json:"version"`
	CreateTime  time.Time         `json:"createTime"`
	Size        int64             `json:"size"`
	Notes       string            `json:"notes"`
	Includes    []string          `json:"includes"` // 包含的内容: config, data, env
	Checksum    string            `json:"checksum"`
	Labels      map[string]string `json:"labels"`
}

// BackupManager 备份管理器
type BackupManager struct {
	mu        sync.RWMutex
	store     *AppStore
	backupDir string
}

// NewBackupManager 创建备份管理器
func NewBackupManager(store *AppStore, dataDir string) (*BackupManager, error) {
	backupDir := filepath.Join(dataDir, "app-backups")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return nil, err
	}

	return &BackupManager{
		store:     store,
		backupDir: backupDir,
	}, nil
}

// BackupApp 备份应用
func (bm *BackupManager) BackupApp(appID string, opts BackupOptions) (*BackupInfo, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	app := bm.store.GetInstalled(appID)
	if app == nil {
		return nil, fmt.Errorf("应用未安装: %s", appID)
	}

	// 创建备份目录
	backupID := fmt.Sprintf("%s-%d", appID, time.Now().Unix())
	backupPath := filepath.Join(bm.backupDir, backupID)
	if err := os.MkdirAll(backupPath, 0750); err != nil {
		return nil, err
	}

	includes := []string{}
	var totalSize int64

	// 备份配置
	if opts.IncludeConfig {
		configPath := filepath.Join(backupPath, "config.json")
		configData := map[string]interface{}{
			"appId":       app.ID,
			"name":        app.Name,
			"displayName": app.DisplayName,
			"templateId":  app.TemplateID,
			"version":     app.Version,
			"ports":       app.Ports,
			"volumes":     app.Volumes,
			"environment": app.Environment,
			"installTime": app.InstallTime,
		}

		data, err := json.MarshalIndent(configData, "", "  ")
		if err != nil {
			_ = os.RemoveAll(backupPath)
			return nil, err
		}

		if err := os.WriteFile(configPath, data, 0640); err != nil {
			_ = os.RemoveAll(backupPath)
			return nil, err
		}

		includes = append(includes, "config")
		totalSize += int64(len(data))
	}

	// 备份 Docker Compose 文件
	if app.ComposePath != "" && opts.IncludeCompose {
		composeData, err := os.ReadFile(app.ComposePath)
		if err == nil {
			composeBackup := filepath.Join(backupPath, "docker-compose.yml")
			if err := os.WriteFile(composeBackup, composeData, 0640); err == nil {
				includes = append(includes, "compose")
				totalSize += int64(len(composeData))
			}
		}
	}

	// 备份数据卷（可选）
	if opts.IncludeData && len(app.Volumes) > 0 {
		dataDir := filepath.Join(backupPath, "volumes")
		if err := os.MkdirAll(dataDir, 0750); err == nil {
			for containerPath, hostPath := range app.Volumes {
				if _, err := os.Stat(hostPath); err == nil {
					// 使用 tar 打包数据
					archiveName := strings.ReplaceAll(strings.TrimPrefix(containerPath, "/"), "/", "_") + ".tar.gz"
					archivePath := filepath.Join(dataDir, archiveName)

					cmd := exec.Command("tar", "-czf", archivePath, "-C", hostPath, ".")
					if err := cmd.Run(); err == nil {
						if fi, err := os.Stat(archivePath); err == nil {
							totalSize += fi.Size()
						}
					}
				}
			}
			includes = append(includes, "data")
		}
	}

	// 计算校验和
	checksum, err := bm.calculateChecksum(backupPath)
	if err != nil {
		_ = os.RemoveAll(backupPath)
		return nil, err
	}

	// 保存备份信息
	info := &BackupInfo{
		ID:         backupID,
		AppID:      appID,
		AppName:    app.DisplayName,
		Version:    app.Version,
		CreateTime: time.Now(),
		Size:       totalSize,
		Notes:      opts.Notes,
		Includes:   includes,
		Checksum:   checksum,
		Labels:     opts.Labels,
	}

	infoPath := filepath.Join(backupPath, "backup.json")
	infoData, _ := json.MarshalIndent(info, "", "  ")
	_ = os.WriteFile(infoPath, infoData, 0640)

	return info, nil
}

// BackupOptions 备份选项
type BackupOptions struct {
	IncludeConfig bool              `json:"includeConfig"`
	IncludeCompose bool             `json:"includeCompose"`
	IncludeData   bool              `json:"includeData"`
	Notes         string            `json:"notes"`
	Labels        map[string]string `json:"labels"`
}

// RestoreApp 恢复应用
func (bm *BackupManager) RestoreApp(backupID string, opts RestoreOptions) (*InstalledApp, error) {
	bm.mu.Lock()
	defer bm.mu.Unlock()

	backupPath := filepath.Join(bm.backupDir, backupID)
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("备份不存在: %s", backupID)
	}

	// 读取备份信息
	infoPath := filepath.Join(backupPath, "backup.json")
	infoData, err := os.ReadFile(infoPath)
	if err != nil {
		return nil, err
	}

	var info BackupInfo
	if err := json.Unmarshal(infoData, &info); err != nil {
		return nil, err
	}

	// 验证校验和
	checksum, err := bm.calculateChecksum(backupPath)
	if err != nil {
		return nil, err
	}

	if checksum != info.Checksum {
		return nil, fmt.Errorf("备份校验失败，文件可能已损坏")
	}

	// 读取配置
	configPath := filepath.Join(backupPath, "config.json")
	configData, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config struct {
		AppID       string            `json:"appId"`
		Name        string            `json:"name"`
		DisplayName string            `json:"displayName"`
		TemplateID  string            `json:"templateId"`
		Version     string            `json:"version"`
		Ports       map[int]int       `json:"ports"`
		Volumes     map[string]string `json:"volumes"`
		Environment map[string]string `json:"environment"`
	}

	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, err
	}

	// 检查应用是否已存在
	if !opts.ForceRestore {
		if existing := bm.store.GetInstalled(config.AppID); existing != nil {
			return nil, fmt.Errorf("应用已存在，使用 forceRestore 强制恢复")
		}
	}

	// 恢复数据卷
	if opts.RestoreData && containsStr(info.Includes, "data") {
		dataDir := filepath.Join(backupPath, "volumes")
		if _, err := os.Stat(dataDir); err == nil {
			for containerPath, hostPath := range config.Volumes {
				archiveName := strings.ReplaceAll(strings.TrimPrefix(containerPath, "/"), "/", "_") + ".tar.gz"
				archivePath := filepath.Join(dataDir, archiveName)

				if _, err := os.Stat(archivePath); err == nil {
					// 创建目标目录
					_ = os.MkdirAll(hostPath, 0750)

					// 解压数据
					cmd := exec.Command("tar", "-xzf", archivePath, "-C", hostPath)
					_ = cmd.Run()
				}
			}
		}
	}

	// 恢复 Docker Compose 文件
	composeBackup := filepath.Join(backupPath, "docker-compose.yml")
	var composePath string

	if composeData, err := os.ReadFile(composeBackup); err == nil {
		appDir := filepath.Join(bm.store.installDir, config.Name)
		_ = os.MkdirAll(appDir, 0750)
		composePath = filepath.Join(appDir, "docker-compose.yml")
		_ = os.WriteFile(composePath, composeData, 0640)

		// 启动容器
		if opts.StartAfterRestore {
			cmd := exec.Command("docker-compose", "-f", composePath, "up", "-d")
			_ = cmd.Run()
		}
	}

	// 创建安装记录
	app := &InstalledApp{
		ID:          config.AppID,
		Name:        config.Name,
		DisplayName: config.DisplayName,
		TemplateID:  config.TemplateID,
		Version:     config.Version,
		Status:      "restored",
		InstallTime: time.Now(),
		Ports:       config.Ports,
		Volumes:     config.Volumes,
		Environment: config.Environment,
		ComposePath: composePath,
	}

	// 保存到安装列表
	bm.store.mu.Lock()
	bm.store.installed[config.AppID] = app
	_ = bm.store.saveInstalled()
	bm.store.mu.Unlock()

	return app, nil
}

// RestoreOptions 恢复选项
type RestoreOptions struct {
	ForceRestore    bool `json:"forceRestore"`
	RestoreData     bool `json:"restoreData"`
	StartAfterRestore bool `json:"startAfterRestore"`
}

// ListBackups 列出所有备份
func (bm *BackupManager) ListBackups(appID string) ([]*BackupInfo, error) {
	bm.mu.RLock()
	defer bm.mu.RUnlock()

	var backups []*BackupInfo

	entries, err := os.ReadDir(bm.backupDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		infoPath := filepath.Join(bm.backupDir, entry.Name(), "backup.json")
		data, err := os.ReadFile(infoPath)
		if err != nil {
			continue
		}

		var info BackupInfo
		if err := json.Unmarshal(data, &info); err != nil {
			continue
		}

		if appID == "" || info.AppID == appID {
			backups = append(backups, &info)
		}
	}

	// 按时间排序（最新的在前）
	sortBackupsByTime(backups)

	return backups, nil
}

// GetBackup 获取备份信息
func (bm *BackupManager) GetBackup(backupID string) (*BackupInfo, error) {
	infoPath := filepath.Join(bm.backupDir, backupID, "backup.json")
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return nil, err
	}

	var info BackupInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}

	return &info, nil
}

// DeleteBackup 删除备份
func (bm *BackupManager) DeleteBackup(backupID string) error {
	backupPath := filepath.Join(bm.backupDir, backupID)
	return os.RemoveAll(backupPath)
}

// calculateChecksum 计算备份目录校验和
func (bm *BackupManager) calculateChecksum(backupPath string) (string, error) {
	// 简单实现：遍历文件计算 MD5
	cmd := exec.Command("sh", "-c", fmt.Sprintf("find %s -type f -exec md5sum {} \\; | sort | md5sum | cut -d' ' -f1", backupPath))
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// sortBackupsByTime 按时间排序备份
func sortBackupsByTime(backups []*BackupInfo) {
	for i := 0; i < len(backups); i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[i].CreateTime.Before(backups[j].CreateTime) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}
}

// containsStr 检查字符串是否在切片中
func containsStr(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// =============================================================================
// 应用健康检查
// =============================================================================

// HealthStatus 健康状态
type HealthStatus struct {
	AppID         string          `json:"appId"`
	AppName       string          `json:"appName"`
	Status        string          `json:"status"` // healthy, unhealthy, starting, stopped
	LastCheck     time.Time       `json:"lastCheck"`
	Checks        []HealthCheck   `json:"checks"`
	Uptime        time.Duration   `json:"uptime"`
	RestartCount  int             `json:"restartCount"`
	LastRestart   time.Time       `json:"lastRestart"`
	Message       string          `json:"message"`
}

// HealthCheck 健康检查项
type HealthCheck struct {
	Name     string        `json:"name"`
	Type     string        `json:"type"` // http, tcp, exec, container
	Status   string        `json:"status"`
	Message  string        `json:"message"`
	Latency  time.Duration `json:"latency"`
	Endpoint string        `json:"endpoint,omitempty"`
}

// HealthChecker 健康检查器
type HealthChecker struct {
	mu      sync.RWMutex
	store   *AppStore
	manager *Manager
	config  HealthCheckConfig
}

// HealthCheckConfig 健康检查配置
type HealthCheckConfig struct {
	CheckInterval   time.Duration `json:"checkInterval"`
	Timeout         time.Duration `json:"timeout"`
	HealthyThreshold int          `json:"healthyThreshold"`
	UnhealthyThreshold int        `json:"unhealthyThreshold"`
}

// DefaultHealthCheckConfig 默认健康检查配置
var DefaultHealthCheckConfig = HealthCheckConfig{
	CheckInterval:     30 * time.Second,
	Timeout:           10 * time.Second,
	HealthyThreshold:  2,
	UnhealthyThreshold: 3,
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(store *AppStore, mgr *Manager, config HealthCheckConfig) *HealthChecker {
	if config.CheckInterval == 0 {
		config = DefaultHealthCheckConfig
	}

	return &HealthChecker{
		store:   store,
		manager: mgr,
		config:  config,
	}
}

// CheckHealth 检查应用健康状态
func (hc *HealthChecker) CheckHealth(appID string) (*HealthStatus, error) {
	app := hc.store.GetInstalled(appID)
	if app == nil {
		return nil, fmt.Errorf("应用未安装: %s", appID)
	}

	status := &HealthStatus{
		AppID:     appID,
		AppName:   app.DisplayName,
		Status:    "unknown",
		LastCheck: time.Now(),
		Checks:    []HealthCheck{},
	}

	// 获取容器信息
	container, err := hc.manager.GetContainer(app.Name)
	if err != nil {
		status.Status = "stopped"
		status.Message = "容器不存在或已停止"
		return status, nil
	}

	// 更新运行时间
	if !container.Created.IsZero() {
		status.Uptime = time.Since(container.Created)
	}

	// 1. 容器状态检查
	containerCheck := hc.checkContainerStatus(container)
	status.Checks = append(status.Checks, containerCheck)

	// 2. 端口检查
	if len(app.Ports) > 0 {
		portChecks := hc.checkPorts(app.Ports)
		status.Checks = append(status.Checks, portChecks...)
	}

	// 3. HTTP 健康检查（如果有 Web 端口）
	if webCheck := hc.checkHTTPEndpoint(app); webCheck != nil {
		status.Checks = append(status.Checks, *webCheck)
	}

	// 4. 资源检查
	resourceCheck := hc.checkResources(container)
	status.Checks = append(status.Checks, resourceCheck)

	// 汇总状态
	status.Status = hc.aggregateStatus(status.Checks)
	status.Message = hc.getStatusMessage(status.Checks)

	return status, nil
}

// checkContainerStatus 检查容器状态
func (hc *HealthChecker) checkContainerStatus(container *Container) HealthCheck {
	check := HealthCheck{
		Name: "容器状态",
		Type: "container",
	}

	if container.State == "running" {
		check.Status = "healthy"
		check.Message = fmt.Sprintf("容器运行中，状态: %s", container.Status)
	} else {
		check.Status = "unhealthy"
		check.Message = fmt.Sprintf("容器未运行，状态: %s", container.State)
	}

	return check
}

// checkPorts 检查端口
func (hc *HealthChecker) checkPorts(ports map[int]int) []HealthCheck {
	var checks []HealthCheck

	for _, hostPort := range ports {
		check := HealthCheck{
			Name:     fmt.Sprintf("端口 %d", hostPort),
			Type:     "tcp",
			Endpoint: fmt.Sprintf("127.0.0.1:%d", hostPort),
		}

		start := time.Now()

		// 尝试连接端口
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", hostPort), hc.config.Timeout)
		check.Latency = time.Since(start)

		if err != nil {
			check.Status = "unhealthy"
			check.Message = fmt.Sprintf("端口 %d 无法连接", hostPort)
		} else {
			check.Status = "healthy"
			check.Message = fmt.Sprintf("端口 %d 正常响应 (%.0fms)", hostPort, float64(check.Latency.Milliseconds()))
			_ = conn.Close()
		}

		checks = append(checks, check)
	}

	return checks
}

// checkHTTPEndpoint 检查 HTTP 端点
func (hc *HealthChecker) checkHTTPEndpoint(app *InstalledApp) *HealthCheck {
	// 查找可能的 Web 端口
	var webPort int
	for containerPort, hostPort := range app.Ports {
		if containerPort == 80 || containerPort == 443 || containerPort == 8080 ||
			containerPort == 3000 || containerPort == 8000 || containerPort == 5000 {
			webPort = hostPort
			break
		}
	}

	if webPort == 0 {
		return nil
	}

	check := HealthCheck{
		Name:     "HTTP 健康检查",
		Type:     "http",
		Endpoint: fmt.Sprintf("http://127.0.0.1:%d/", webPort),
	}

	start := time.Now()

	client := &http.Client{
		Timeout: hc.config.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Get(check.Endpoint)
	check.Latency = time.Since(start)

	if err != nil {
		check.Status = "unhealthy"
		check.Message = fmt.Sprintf("HTTP 请求失败: %v", err)
	} else {
		_ = resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 400 {
			check.Status = "healthy"
			check.Message = fmt.Sprintf("HTTP %d (%.0fms)", resp.StatusCode, float64(check.Latency.Milliseconds()))
		} else {
			check.Status = "degraded"
			check.Message = fmt.Sprintf("HTTP %d 响应异常", resp.StatusCode)
		}
	}

	return &check
}

// checkResources 检查资源使用
func (hc *HealthChecker) checkResources(container *Container) HealthCheck {
	check := HealthCheck{
		Name: "资源使用",
		Type: "resource",
	}

	// CPU 使用率检查
	if container.CPUUsage > 80 {
		check.Status = "degraded"
		check.Message = fmt.Sprintf("CPU 使用率过高: %.1f%%", container.CPUUsage)
		return check
	}

	// 内存使用检查
	if container.MemLimit > 0 {
		memUsagePercent := float64(container.MemUsage) / float64(container.MemLimit) * 100
		if memUsagePercent > 90 {
			check.Status = "degraded"
			check.Message = fmt.Sprintf("内存使用率过高: %.1f%%", memUsagePercent)
			return check
		}
	}

	check.Status = "healthy"
	check.Message = fmt.Sprintf("CPU: %.1f%%, 内存: %s/%s",
		container.CPUUsage,
		formatBytes(container.MemUsage),
		formatBytes(container.MemLimit))

	return check
}

// aggregateStatus 汇总状态
func (hc *HealthChecker) aggregateStatus(checks []HealthCheck) string {
	hasUnhealthy := false
	hasDegraded := false

	for _, check := range checks {
		switch check.Status {
		case "unhealthy":
			hasUnhealthy = true
		case "degraded":
			hasDegraded = true
		}
	}

	if hasUnhealthy {
		return "unhealthy"
	}
	if hasDegraded {
		return "degraded"
	}
	return "healthy"
}

// getStatusMessage 获取状态消息
func (hc *HealthChecker) getStatusMessage(checks []HealthCheck) string {
	var unhealthy []string
	for _, check := range checks {
		if check.Status == "unhealthy" {
			unhealthy = append(unhealthy, check.Name)
		}
	}

	if len(unhealthy) == 0 {
		return "所有检查项正常"
	}

	return fmt.Sprintf("异常检查项: %s", strings.Join(unhealthy, ", "))
}

// formatBytes 格式化字节
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// StartPeriodicCheck 启动定期健康检查
func (hc *HealthChecker) StartPeriodicCheck() chan *HealthStatus {
	results := make(chan *HealthStatus, 100)

	go func() {
		ticker := time.NewTicker(hc.config.CheckInterval)
		defer ticker.Stop()

		for range ticker.C {
			installed := hc.store.ListInstalled()
			for _, app := range installed {
				if status, err := hc.CheckHealth(app.ID); err == nil {
					select {
					case results <- status:
					default:
						// 通道满了，跳过
					}
				}
			}
		}
	}()

	return results
}
