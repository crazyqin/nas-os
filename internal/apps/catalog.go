// Package apps 应用目录管理
// 管理应用模板的加载、存储、搜索和分类
package apps

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"nas-os/pkg/app"
)

// Catalog 应用目录管理器
type Catalog struct {
	mu sync.RWMutex

	templateDir string                // 模板目录
	templates   map[string]*app.Template // 模板缓存
	categories  map[string]int        // 分类统计
}

// NewCatalog 创建应用目录管理器
func NewCatalog(templateDir string) (*Catalog, error) {
	if err := os.MkdirAll(templateDir, 0750); err != nil {
		return nil, fmt.Errorf("创建模板目录失败: %w", err)
	}

	catalog := &Catalog{
		templateDir: templateDir,
		templates:   make(map[string]*app.Template),
		categories:  make(map[string]int),
	}

	// 加载内置模板
	catalog.loadBuiltinTemplates()

	// 加载目录中的模板文件
	if err := catalog.loadTemplatesFromDir(); err != nil {
		fmt.Printf("加载模板文件失败: %v\n", err)
	}

	return catalog, nil
}

// loadBuiltinTemplates 加载内置应用模板
func (c *Catalog) loadBuiltinTemplates() {
	builtin := []*app.Template{
		// ===== 媒体类 =====
		{
			ID:          "jellyfin",
			Name:        "jellyfin",
			DisplayName: "Jellyfin",
			Description: "开源媒体服务器，支持电影、电视剧、音乐播放",
			Category:    app.CategoryMedia,
			Icon:        "🎬",
			Version:     "latest",
			Author:      "Jellyfin Team",
			Website:     "https://jellyfin.org",
			Source:      "https://github.com/jellyfin/jellyfin",
			License:     "GPL-2.0",
			Containers: []app.ContainerSpec{
				{
					Name:  "jellyfin",
					Image: "jellyfin/jellyfin:latest",
					Ports: []app.PortSpec{
						{Name: "web", ContainerPort: 8096, Protocol: "tcp", DefaultHostPort: 8096, Description: "Web界面"},
					},
					Volumes: []app.VolumeSpec{
						{Name: "config", ContainerPath: "/config", DefaultHostPath: "/opt/nas/apps/jellyfin/config", Description: "配置目录"},
						{Name: "cache", ContainerPath: "/cache", DefaultHostPath: "/opt/nas/apps/jellyfin/cache", Description: "缓存目录"},
						{Name: "media", ContainerPath: "/media", DefaultHostPath: "/opt/nas/media", Description: "媒体目录"},
					},
					Environment: map[string]string{
						"PUID": "1000",
						"PGID": "1000",
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Notes: "首次启动需要设置媒体库",
		},
		{
			ID:          "plex",
			Name:        "plex",
			DisplayName: "Plex",
			Description: "流行媒体服务器，支持电影、电视剧、音乐播放",
			Category:    app.CategoryMedia,
			Icon:        "🎥",
			Version:     "latest",
			Author:      "Plex Inc",
			Website:     "https://www.plex.tv",
			Source:      "https://github.com/plexinc",
			License:     "Proprietary",
			Containers: []app.ContainerSpec{
				{
					Name:  "plex",
					Image: "plexinc/pms-docker:latest",
					Ports: []app.PortSpec{
						{Name: "web", ContainerPort: 32400, Protocol: "tcp", DefaultHostPort: 32400, Description: "Web界面"},
					},
					Volumes: []app.VolumeSpec{
						{Name: "config", ContainerPath: "/config", DefaultHostPath: "/opt/nas/apps/plex/config", Description: "配置目录"},
						{Name: "transcode", ContainerPath: "/transcode", DefaultHostPath: "/opt/nas/apps/plex/transcode", Description: "转码目录"},
						{Name: "media", ContainerPath: "/data", DefaultHostPath: "/opt/nas/media", Description: "媒体目录"},
					},
					Environment: map[string]string{
						"PUID": "1000",
						"PGID": "1000",
						"TZ":   "Asia/Shanghai",
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Notes: "需要 Plex Pass 订阅获取完整功能",
		},
		// ===== 生产力类 =====
		{
			ID:          "nextcloud",
			Name:        "nextcloud",
			DisplayName: "Nextcloud",
			Description: "私有云存储服务，支持文件同步、分享、在线办公",
			Category:    app.CategoryProductivity,
			Icon:        "☁️",
			Version:     "latest",
			Author:      "Nextcloud GmbH",
			Website:     "https://nextcloud.com",
			Source:      "https://github.com/nextcloud",
			License:     "AGPL-3.0",
			Containers: []app.ContainerSpec{
				{
					Name:  "nextcloud",
					Image: "nextcloud:latest",
					Ports: []app.PortSpec{
						{Name: "web", ContainerPort: 80, Protocol: "tcp", DefaultHostPort: 8080, Description: "Web界面"},
					},
					Volumes: []app.VolumeSpec{
						{Name: "data", ContainerPath: "/var/www/html", DefaultHostPath: "/opt/nas/apps/nextcloud/data", Description: "数据目录"},
					},
					Environment: map[string]string{
						"TZ": "Asia/Shanghai",
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Notes: "首次访问需要创建管理员账户",
		},
		{
			ID:          "syncthing",
			Name:        "syncthing",
			DisplayName: "Syncthing",
			Description: "开源文件同步工具，支持多设备间实时同步",
			Category:    app.CategoryProductivity,
			Icon:        "🔄",
			Version:     "latest",
			Author:      "Syncthing Foundation",
			Website:     "https://syncthing.net",
			Source:      "https://github.com/syncthing/syncthing",
			License:     "MPL-2.0",
			Containers: []app.ContainerSpec{
				{
					Name:     "syncthing",
					Image:    "syncthing/syncthing:latest",
					Hostname: "nas-syncthing",
					Ports: []app.PortSpec{
						{Name: "web", ContainerPort: 8384, Protocol: "tcp", DefaultHostPort: 8384, Description: "Web界面"},
						{Name: "sync", ContainerPort: 22000, Protocol: "tcp", DefaultHostPort: 22000, Description: "同步端口"},
					},
					Volumes: []app.VolumeSpec{
						{Name: "data", ContainerPath: "/var/syncthing", DefaultHostPath: "/opt/nas/apps/syncthing/data", Description: "数据目录"},
					},
					Environment: map[string]string{
						"PUID": "1000",
						"PGID": "1000",
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Notes: "首次访问需要设置用户名密码",
		},
		// ===== 智能家居类 =====
		{
			ID:          "homeassistant",
			Name:        "homeassistant",
			DisplayName: "Home Assistant",
			Description: "开源智能家居平台，支持数千种设备集成",
			Category:    app.CategorySmartHome,
			Icon:        "🏠",
			Version:     "stable",
			Author:      "Home Assistant",
			Website:     "https://www.home-assistant.io",
			Source:      "https://github.com/home-assistant/core",
			License:     "Apache-2.0",
			Containers: []app.ContainerSpec{
				{
					Name:       "homeassistant",
					Image:      "homeassistant/home-assistant:stable",
					Privileged: true,
					NetworkMode: "host",
					Volumes: []app.VolumeSpec{
						{Name: "config", ContainerPath: "/config", DefaultHostPath: "/opt/nas/apps/homeassistant/config", Description: "配置目录"},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Notes: "使用 host 网络模式以支持设备发现",
		},
		// ===== 下载类 =====
		{
			ID:          "transmission",
			Name:        "transmission",
			DisplayName: "Transmission",
			Description: "轻量级 BitTorrent 客户端，简洁高效",
			Category:    app.CategoryDownload,
			Icon:        "📥",
			Version:     "latest",
			Author:      "LinuxServer.io",
			Website:     "https://transmissionbt.com",
			Source:      "https://github.com/transmission/transmission",
			License:     "GPL-2.0",
			Containers: []app.ContainerSpec{
				{
					Name:  "transmission",
					Image: "linuxserver/transmission:latest",
					Ports: []app.PortSpec{
						{Name: "web", ContainerPort: 9091, Protocol: "tcp", DefaultHostPort: 9091, Description: "Web界面"},
						{Name: "bt", ContainerPort: 51413, Protocol: "tcp", DefaultHostPort: 51413, Description: "BT端口"},
					},
					Volumes: []app.VolumeSpec{
						{Name: "config", ContainerPath: "/config", DefaultHostPath: "/opt/nas/apps/transmission/config", Description: "配置目录"},
						{Name: "downloads", ContainerPath: "/downloads", DefaultHostPath: "/opt/nas/downloads", Description: "下载目录"},
					},
					Environment: map[string]string{
						"PUID": "1000",
						"PGID": "1000",
						"TZ":   "Asia/Shanghai",
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Notes: "默认用户名密码: admin/admin",
		},
		// ===== 网络类 =====
		{
			ID:          "pihole",
			Name:        "pihole",
			DisplayName: "Pi-hole",
			Description: "网络级广告拦截器，DNS服务器",
			Category:    app.CategoryNetwork,
			Icon:        "🛡️",
			Version:     "latest",
			Author:      "Pi-hole LLC",
			Website:     "https://pi-hole.net",
			Source:      "https://github.com/pi-hole/pi-hole",
			License:     "EUPL-1.2",
			Containers: []app.ContainerSpec{
				{
					Name:  "pihole",
					Image: "pihole/pihole:latest",
					Ports: []app.PortSpec{
						{Name: "dns-tcp", ContainerPort: 53, Protocol: "tcp", DefaultHostPort: 53, Description: "DNS TCP"},
						{Name: "dns-udp", ContainerPort: 53, Protocol: "udp", DefaultHostPort: 53, Description: "DNS UDP"},
						{Name: "web", ContainerPort: 80, Protocol: "tcp", DefaultHostPort: 8081, Description: "Web界面"},
					},
					Volumes: []app.VolumeSpec{
						{Name: "config", ContainerPath: "/etc/pihole", DefaultHostPath: "/opt/nas/apps/pihole/etc", Description: "配置目录"},
						{Name: "dnsmasq", ContainerPath: "/etc/dnsmasq.d", DefaultHostPath: "/opt/nas/apps/pihole/dnsmasq", Description: "DNS配置"},
					},
					Environment: map[string]string{
						"TZ": "Asia/Shanghai",
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Notes: "设置路由器DNS指向Pi-hole以全局拦截广告",
		},
		// ===== 数据库类 =====
		{
			ID:          "postgres",
			Name:        "postgres",
			DisplayName: "PostgreSQL",
			Description: "强大的开源关系型数据库",
			Category:    app.CategoryDatabase,
			Icon:        "🗄️",
			Version:     "15",
			Author:      "PostgreSQL Global Development Group",
			Website:     "https://www.postgresql.org",
			Source:      "https://github.com/postgres/postgres",
			License:     "PostgreSQL",
			Containers: []app.ContainerSpec{
				{
					Name:  "postgres",
					Image: "postgres:15",
					Ports: []app.PortSpec{
						{Name: "db", ContainerPort: 5432, Protocol: "tcp", DefaultHostPort: 5432, Description: "数据库端口"},
					},
					Volumes: []app.VolumeSpec{
						{Name: "data", ContainerPath: "/var/lib/postgresql/data", DefaultHostPath: "/opt/nas/apps/postgres/data", Description: "数据目录"},
					},
					Environment: map[string]string{
						"POSTGRES_PASSWORD": "nas123456",
						"TZ":                "Asia/Shanghai",
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Notes: "默认密码: nas123456，请及时修改",
		},
		{
			ID:          "redis",
			Name:        "redis",
			DisplayName: "Redis",
			Description: "高性能内存数据库和缓存服务器",
			Category:    app.CategoryDatabase,
			Icon:        "⚡",
			Version:     "latest",
			Author:      "Redis Ltd",
			Website:     "https://redis.io",
			Source:      "https://github.com/redis/redis",
			License:     "BSD-3-Clause",
			Containers: []app.ContainerSpec{
				{
					Name:  "redis",
					Image: "redis:latest",
					Ports: []app.PortSpec{
						{Name: "db", ContainerPort: 6379, Protocol: "tcp", DefaultHostPort: 6379, Description: "数据库端口"},
					},
					Volumes: []app.VolumeSpec{
						{Name: "data", ContainerPath: "/data", DefaultHostPath: "/opt/nas/apps/redis/data", Description: "数据目录"},
					},
					Command: []string{"redis-server", "--appendonly yes"},
					RestartPolicy: "unless-stopped",
				},
			},
			Notes: "默认启用AOF持久化",
		},
		// ===== AI类 =====
		{
			ID:          "qdrant",
			Name:        "qdrant",
			DisplayName: "Qdrant",
			Description: "高性能向量数据库，用于AI应用",
			Category:    app.CategoryAI,
			Icon:        "🧠",
			Version:     "latest",
			Author:      "Qdrant",
			Website:     "https://qdrant.tech",
			Source:      "https://github.com/qdrant/qdrant",
			License:     "Apache-2.0",
			Containers: []app.ContainerSpec{
				{
					Name:  "qdrant",
					Image: "qdrant/qdrant:latest",
					Ports: []app.PortSpec{
						{Name: "web", ContainerPort: 6333, Protocol: "tcp", DefaultHostPort: 6333, Description: "Web界面"},
						{Name: "grpc", ContainerPort: 6334, Protocol: "tcp", DefaultHostPort: 6334, Description: "GRPC端口"},
					},
					Volumes: []app.VolumeSpec{
						{Name: "data", ContainerPath: "/qdrant/storage", DefaultHostPath: "/opt/nas/apps/qdrant/data", Description: "数据目录"},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Notes: "用于向量搜索和AI应用",
		},
		// ===== 网络服务类 =====
		{
			ID:          "nginx",
			Name:        "nginx",
			DisplayName: "Nginx",
			Description: "高性能Web服务器和反向代理",
			Category:    app.CategoryNetwork,
			Icon:        "🌐",
			Version:     "latest",
			Author:      "NGINX Inc",
			Website:     "https://nginx.org",
			Source:      "https://github.com/nginx/nginx",
			License:     "BSD-2-Clause",
			Containers: []app.ContainerSpec{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					Ports: []app.PortSpec{
						{Name: "http", ContainerPort: 80, Protocol: "tcp", DefaultHostPort: 8888, Description: "HTTP端口"},
						{Name: "https", ContainerPort: 443, Protocol: "tcp", DefaultHostPort: 8443, Description: "HTTPS端口"},
					},
					Volumes: []app.VolumeSpec{
						{Name: "config", ContainerPath: "/etc/nginx", DefaultHostPath: "/opt/nas/apps/nginx/config", Description: "配置目录"},
						{Name: "html", ContainerPath: "/usr/share/nginx/html", DefaultHostPath: "/opt/nas/apps/nginx/html", Description: "网站目录"},
						{Name: "logs", ContainerPath: "/var/log/nginx", DefaultHostPath: "/opt/nas/apps/nginx/logs", Description: "日志目录"},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Notes: "默认配置为反向代理模式",
		},
	}

	for _, t := range builtin {
		c.templates[t.ID] = t
		c.categories[t.Category]++
	}
}

// loadTemplatesFromDir 从目录加载模板文件
func (c *Catalog) loadTemplatesFromDir() error {
	files, err := os.ReadDir(c.templateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取模板目录失败: %w", err)
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			path := filepath.Join(c.templateDir, file.Name())
			if err := c.loadTemplateFile(path); err != nil {
				fmt.Printf("加载模板文件 %s 失败: %v\n", file.Name(), err)
			}
		}
	}

	return nil
}

// loadTemplateFile 加载单个模板文件
func (c *Catalog) loadTemplateFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	template := &app.Template{}
	if err := json.Unmarshal(data, template); err != nil {
		return fmt.Errorf("解析模板失败: %w", err)
	}

	if template.ID == "" {
		return fmt.Errorf("模板ID不能为空")
	}

	c.mu.Lock()
	c.templates[template.ID] = template
	c.categories[template.Category]++
	c.mu.Unlock()

	return nil
}

// List 列出模板（可选按分类过滤）
func (c *Catalog) List(category string) ([]*app.Template, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*app.Template, 0)
	for _, t := range c.templates {
		if category == "" || t.Category == category {
			result = append(result, t)
		}
	}

	// 按名称排序
	sortTemplates(result)

	return result, nil
}

// Get 获取指定模板
func (c *Catalog) Get(id string) (*app.Template, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	template, exists := c.templates[id]
	if !exists {
		return nil, fmt.Errorf("模板 %s 不存在", id)
	}

	return template, nil
}

// Search 搜索模板
func (c *Catalog) Search(query string) ([]*app.Template, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	query = strings.ToLower(query)
	result := make([]*app.Template, 0)

	for _, t := range c.templates {
		// 匹配名称、显示名、描述
		if strings.Contains(strings.ToLower(t.Name), query) ||
			strings.Contains(strings.ToLower(t.DisplayName), query) ||
			strings.Contains(strings.ToLower(t.Description), query) ||
			strings.Contains(strings.ToLower(t.Category), query) {
			result = append(result, t)
		}
	}

	sortTemplates(result)
	return result, nil
}

// Categories 获取分类列表
func (c *Catalog) Categories() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cats := make([]string, 0, len(c.categories))
	for cat := range c.categories {
		cats = append(cats, cat)
	}

	// 按分类名称排序
	for i := 0; i < len(cats)-1; i++ {
		for j := i + 1; j < len(cats); j++ {
			if cats[i] > cats[j] {
				cats[i], cats[j] = cats[j], cats[i]
			}
		}
	}

	return cats
}

// Add 添加新模板
func (c *Catalog) Add(template *app.Template) error {
	if template.ID == "" {
		return fmt.Errorf("模板ID不能为空")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// 检查是否已存在
	if _, exists := c.templates[template.ID]; exists {
		return fmt.Errorf("模板 %s 已存在", template.ID)
	}

	c.templates[template.ID] = template
	c.categories[template.Category]++

	// 保存到文件
	path := filepath.Join(c.templateDir, template.ID+".json")
	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化模板失败: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Update 更新模板
func (c *Catalog) Update(template *app.Template) error {
	if template.ID == "" {
		return fmt.Errorf("模板ID不能为空")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	old, exists := c.templates[template.ID]
	if !exists {
		return fmt.Errorf("模板 %s 不存在", template.ID)
	}

	// 更新分类计数
	if old.Category != template.Category {
		c.categories[old.Category]--
		c.categories[template.Category]++
	}

	c.templates[template.ID] = template

	// 保存到文件
	path := filepath.Join(c.templateDir, template.ID+".json")
	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化模板失败: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}

// Remove 移除模板
func (c *Catalog) Remove(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	template, exists := c.templates[id]
	if !exists {
		return fmt.Errorf("模板 %s 不存在", id)
	}

	c.categories[template.Category]--
	delete(c.templates, id)

	// 删除文件
	path := filepath.Join(c.templateDir, id+".json")
	return os.Remove(path)
}

// sortTemplates 模板排序辅助函数
func sortTemplates(templates []*app.Template) {
	for i := 0; i < len(templates)-1; i++ {
		for j := i + 1; j < len(templates); j++ {
			if templates[i].DisplayName > templates[j].DisplayName {
				templates[i], templates[j] = templates[j], templates[i]
			}
		}
	}
}