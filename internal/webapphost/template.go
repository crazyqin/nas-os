package webapphost

import (
	"fmt"
	"log"
	"sort"
	"sync"
)

// TemplateManager 模板管理器.
type TemplateManager struct {
	mu         sync.RWMutex
	templates  map[string]*AppTemplate
	categories map[string]*MarketCategory
}

// NewTemplateManager 创建模板管理器.
func NewTemplateManager() *TemplateManager {
	tm := &TemplateManager{
		templates:  make(map[string]*AppTemplate),
		categories: make(map[string]*MarketCategory),
	}

	// 注册默认分类
	tm.registerDefaultCategories()

	// 注册内置模板
	tm.registerBuiltinTemplates()

	return tm
}

// registerDefaultCategories 注册默认分类.
func (tm *TemplateManager) registerDefaultCategories() {
	categories := []*MarketCategory{
		{ID: "web", Name: "Web 服务", Description: "Web 应用和服务", Icon: "web", AppCount: 0},
		{ID: "database", Name: "数据库", Description: "数据库服务", Icon: "database", AppCount: 0},
		{ID: "media", Name: "媒体", Description: "媒体服务", Icon: "media", AppCount: 0},
		{ID: "devops", Name: "DevOps", Description: "开发运维工具", Icon: "devops", AppCount: 0},
		{ID: "productivity", Name: "效率", Description: "生产力工具", Icon: "productivity", AppCount: 0},
		{ID: "network", Name: "网络", Description: "网络服务", Icon: "network", AppCount: 0},
		{ID: "security", Name: "安全", Description: "安全工具", Icon: "security", AppCount: 0},
		{ID: "storage", Name: "存储", Description: "存储服务", Icon: "storage", AppCount: 0},
	}

	for _, cat := range categories {
		tm.categories[cat.ID] = cat
	}
}

// registerBuiltinTemplates 注册内置模板.
func (tm *TemplateManager) registerBuiltinTemplates() {
	templates := []*AppTemplate{
		// Web 服务
		{
			ID:          "wordpress",
			Name:        "wordpress",
			DisplayName: "WordPress",
			Description: "世界上最流行的博客和 CMS 平台",
			Category:    "web",
			Icon:        "wordpress",
			Version:     "6.4",
			Author:      "WordPress Foundation",
			Type:        "docker",
			Image:       "wordpress:latest",
			Tags:        []string{"blog", "cms", "php"},
			EnvVars: []EnvVarDef{
				{Name: "WORDPRESS_DB_HOST", Description: "数据库主机", Default: "db:3306", Required: true},
				{Name: "WORDPRESS_DB_USER", Description: "数据库用户", Default: "wordpress", Required: true},
				{Name: "WORDPRESS_DB_PASSWORD", Description: "数据库密码", Required: true, Secret: true},
				{Name: "WORDPRESS_DB_NAME", Description: "数据库名", Default: "wordpress", Required: true},
			},
			Volumes: []VolumeDef{
				{Name: "data", ContainerPath: "/var/www/html", Description: "WordPress 数据", Required: true},
			},
			Ports: []PortDef{
				{Name: "http", ContainerPort: 80, Protocol: "tcp", Description: "HTTP 端口", Required: true},
			},
			MinMemory: 256,
			MinCPU:    0.5,
			MinDisk:   1024,
			Rating:    4.5,
			Downloads: 100000,
			Official:  true,
			Featured:  true,
		},
		{
			ID:          "gitea",
			Name:        "gitea",
			DisplayName: "Gitea",
			Description: "轻量级 Git 托管服务",
			Category:    "devops",
			Icon:        "gitea",
			Version:     "1.21",
			Author:      "Gitea",
			Type:        "docker",
			Image:       "gitea/gitea:latest",
			Tags:        []string{"git", "code", "devops"},
			EnvVars: []EnvVarDef{
				{Name: "GITEA__database__DB_TYPE", Description: "数据库类型", Default: "sqlite3", Options: []string{"sqlite3", "mysql", "postgres"}},
			},
			Volumes: []VolumeDef{
				{Name: "data", ContainerPath: "/data", Description: "Gitea 数据", Required: true},
			},
			Ports: []PortDef{
				{Name: "http", ContainerPort: 3000, Protocol: "tcp", Description: "HTTP 端口", Required: true},
				{Name: "ssh", ContainerPort: 22, Protocol: "tcp", Description: "SSH 端口", Required: false},
			},
			MinMemory: 256,
			MinCPU:    0.5,
			MinDisk:   1024,
			Rating:    4.8,
			Downloads: 50000,
			Official:  true,
			Featured:  true,
		},
		{
			ID:          "nextcloud",
			Name:        "nextcloud",
			DisplayName: "Nextcloud",
			Description: "私有云存储和协作平台",
			Category:    "storage",
			Icon:        "nextcloud",
			Version:     "28",
			Author:      "Nextcloud GmbH",
			Type:        "docker",
			Image:       "nextcloud:latest",
			Tags:        []string{"cloud", "storage", "sync"},
			EnvVars: []EnvVarDef{
				{Name: "MYSQL_HOST", Description: "MySQL 主机", Default: "db:3306", Required: true},
				{Name: "MYSQL_DATABASE", Description: "数据库名", Default: "nextcloud", Required: true},
				{Name: "MYSQL_USER", Description: "数据库用户", Default: "nextcloud", Required: true},
				{Name: "MYSQL_PASSWORD", Description: "数据库密码", Required: true, Secret: true},
			},
			Volumes: []VolumeDef{
				{Name: "data", ContainerPath: "/var/www/html", Description: "Nextcloud 数据", Required: true},
				{Name: "apps", ContainerPath: "/var/www/html/custom_apps", Description: "应用目录"},
			},
			Ports: []PortDef{
				{Name: "http", ContainerPort: 80, Protocol: "tcp", Description: "HTTP 端口", Required: true},
			},
			MinMemory: 512,
			MinCPU:    1.0,
			MinDisk:   2048,
			Rating:    4.7,
			Downloads: 80000,
			Official:  true,
			Featured:  true,
		},
		// 数据库
		{
			ID:          "mysql",
			Name:        "mysql",
			DisplayName: "MySQL",
			Description: "世界上最流行的开源关系型数据库",
			Category:    "database",
			Icon:        "mysql",
			Version:     "8.0",
			Author:      "Oracle",
			Type:        "docker",
			Image:       "mysql:8.0",
			Tags:        []string{"database", "sql", "relational"},
			EnvVars: []EnvVarDef{
				{Name: "MYSQL_ROOT_PASSWORD", Description: "Root 密码", Required: true, Secret: true},
				{Name: "MYSQL_DATABASE", Description: "默认数据库"},
				{Name: "MYSQL_USER", Description: "默认用户"},
				{Name: "MYSQL_PASSWORD", Description: "默认用户密码", Secret: true},
			},
			Volumes: []VolumeDef{
				{Name: "data", ContainerPath: "/var/lib/mysql", Description: "MySQL 数据", Required: true},
			},
			Ports: []PortDef{
				{Name: "mysql", ContainerPort: 3306, Protocol: "tcp", Description: "MySQL 端口", Required: true},
			},
			MinMemory: 512,
			MinCPU:    1.0,
			MinDisk:   1024,
			Rating:    4.6,
			Downloads: 200000,
			Official:  true,
		},
		{
			ID:          "postgresql",
			Name:        "postgresql",
			DisplayName: "PostgreSQL",
			Description: "功能强大的开源对象关系型数据库",
			Category:    "database",
			Icon:        "postgresql",
			Version:     "16",
			Author:      "PostgreSQL Global Development Group",
			Type:        "docker",
			Image:       "postgres:16-alpine",
			Tags:        []string{"database", "sql", "relational"},
			EnvVars: []EnvVarDef{
				{Name: "POSTGRES_PASSWORD", Description: "PostgreSQL 密码", Required: true, Secret: true},
				{Name: "POSTGRES_USER", Description: "PostgreSQL 用户", Default: "postgres"},
				{Name: "POSTGRES_DB", Description: "默认数据库"},
			},
			Volumes: []VolumeDef{
				{Name: "data", ContainerPath: "/var/lib/postgresql/data", Description: "PostgreSQL 数据", Required: true},
			},
			Ports: []PortDef{
				{Name: "postgres", ContainerPort: 5432, Protocol: "tcp", Description: "PostgreSQL 端口", Required: true},
			},
			MinMemory: 256,
			MinCPU:    0.5,
			MinDisk:   1024,
			Rating:    4.7,
			Downloads: 150000,
			Official:  true,
		},
		// 媒体
		{
			ID:          "jellyfin",
			Name:        "jellyfin",
			DisplayName: "Jellyfin",
			Description: "免费的媒体服务器",
			Category:    "media",
			Icon:        "jellyfin",
			Version:     "10.8",
			Author:      "Jellyfin Project",
			Type:        "docker",
			Image:       "jellyfin/jellyfin:latest",
			Tags:        []string{"media", "video", "music"},
			Volumes: []VolumeDef{
				{Name: "config", ContainerPath: "/config", Description: "配置数据", Required: true},
				{Name: "media", ContainerPath: "/media", Description: "媒体文件"},
				{Name: "cache", ContainerPath: "/cache", Description: "缓存"},
			},
			Ports: []PortDef{
				{Name: "http", ContainerPort: 8096, Protocol: "tcp", Description: "HTTP 端口", Required: true},
			},
			MinMemory: 512,
			MinCPU:    1.0,
			MinDisk:   1024,
			Rating:    4.8,
			Downloads: 60000,
			Official:  true,
			Featured:  true,
		},
		// 效率
		{
			ID:          "vaultwarden",
			Name:        "vaultwarden",
			DisplayName: "Vaultwarden",
			Description: "Bitwarden 兼容的密码管理器",
			Category:    "security",
			Icon:        "vaultwarden",
			Version:     "1.30",
			Author:      "Daniel García",
			Type:        "docker",
			Image:       "vaultwarden/server:latest",
			Tags:        []string{"password", "security", "vault"},
			EnvVars: []EnvVarDef{
				{Name: "ADMIN_TOKEN", Description: "管理后台 Token", Secret: true},
				{Name: "SIGNUPS_ALLOWED", Description: "允许注册", Default: "true", Type: "boolean"},
			},
			Volumes: []VolumeDef{
				{Name: "data", ContainerPath: "/data", Description: "Vaultwarden 数据", Required: true},
			},
			Ports: []PortDef{
				{Name: "http", ContainerPort: 80, Protocol: "tcp", Description: "HTTP 端口", Required: true},
			},
			MinMemory: 128,
			MinCPU:    0.25,
			MinDisk:   512,
			Rating:    4.9,
			Downloads: 40000,
			Official:  true,
		},
	}

	for _, tmpl := range templates {
		tm.templates[tmpl.ID] = tmpl
		// 更新分类计数
		if cat, exists := tm.categories[tmpl.Category]; exists {
			cat.AppCount++
		}
	}
}

// GetTemplate 获取模板.
func (tm *TemplateManager) GetTemplate(id string) (*AppTemplate, error) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tmpl, exists := tm.templates[id]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", id)
	}
	return tmpl, nil
}

// ListTemplates 列出所有模板.
func (tm *TemplateManager) ListTemplates(opts *TemplateListOptions) []*AppTemplate {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	templates := make([]*AppTemplate, 0, len(tm.templates))
	for _, tmpl := range tm.templates {
		if opts != nil {
			if opts.Category != "" && tmpl.Category != opts.Category {
				continue
			}
			if opts.Type != "" && tmpl.Type != opts.Type {
				continue
			}
			if opts.Official && !tmpl.Official {
				continue
			}
			if opts.Featured && !tmpl.Featured {
				continue
			}
			if opts.Search != "" {
				if !contains(tmpl.Name, opts.Search) && !contains(tmpl.DisplayName, opts.Search) && !contains(tmpl.Description, opts.Search) {
					continue
				}
			}
		}
		templates = append(templates, tmpl)
	}

	// 排序
	if opts != nil && opts.SortBy != "" {
		sortTemplates(templates, opts.SortBy, opts.SortDesc)
	} else {
		sortTemplates(templates, "downloads", true)
	}

	// 分页
	if opts != nil && opts.Limit > 0 {
		start := opts.Offset
		if start >= len(templates) {
			return []*AppTemplate{}
		}
		end := start + opts.Limit
		if end > len(templates) {
			end = len(templates)
		}
		return templates[start:end]
	}

	return templates
}

// TemplateListOptions 模板列表选项.
type TemplateListOptions struct {
	Category string `json:"category,omitempty"`
	Type     string `json:"type,omitempty"`
	Official bool   `json:"official,omitempty"`
	Featured bool   `json:"featured,omitempty"`
	Search   string `json:"search,omitempty"`
	SortBy   string `json:"sort_by,omitempty"` // name, downloads, rating
	SortDesc bool   `json:"sort_desc,omitempty"`
	Limit    int    `json:"limit,omitempty"`
	Offset   int    `json:"offset,omitempty"`
}

// sortTemplates 排序模板.
func sortTemplates(templates []*AppTemplate, sortBy string, desc bool) {
	sort.Slice(templates, func(i, j int) bool {
		switch sortBy {
		case "name":
			if desc {
				return templates[i].DisplayName > templates[j].DisplayName
			}
			return templates[i].DisplayName < templates[j].DisplayName
		case "downloads":
			if desc {
				return templates[i].Downloads > templates[j].Downloads
			}
			return templates[i].Downloads < templates[j].Downloads
		case "rating":
			if desc {
				return templates[i].Rating > templates[j].Rating
			}
			return templates[i].Rating < templates[j].Rating
		default:
			return templates[i].Downloads > templates[j].Downloads
		}
	})
}

// contains 检查字符串是否包含子串.
func contains(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && (s == substr || len(s) > 0 && containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	// 简单实现，实际应使用 strings.Contains
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(s[i+j]) != toLower(substr[j]) {
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

func toLower(b byte) byte {
	if b >= 'A' && b <= 'Z' {
		return b + 32
	}
	return b
}

// ListCategories 列出所有分类.
func (tm *TemplateManager) ListCategories() []*MarketCategory {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	categories := make([]*MarketCategory, 0, len(tm.categories))
	for _, cat := range tm.categories {
		categories = append(categories, cat)
	}

	sort.Slice(categories, func(i, j int) bool {
		return categories[i].ID < categories[j].ID
	})

	return categories
}

// RegisterTemplate 注册自定义模板.
func (tm *TemplateManager) RegisterTemplate(tmpl *AppTemplate) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tmpl.ID == "" {
		return fmt.Errorf("template ID is required")
	}

	if _, exists := tm.templates[tmpl.ID]; exists {
		return fmt.Errorf("template already exists: %s", tmpl.ID)
	}

	tm.templates[tmpl.ID] = tmpl

	// 更新分类计数
	if cat, exists := tm.categories[tmpl.Category]; exists {
		cat.AppCount++
	}

	log.Printf("Template registered: %s (%s)", tmpl.ID, tmpl.DisplayName)
	return nil
}

// UnregisterTemplate 注销模板.
func (tm *TemplateManager) UnregisterTemplate(id string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tmpl, exists := tm.templates[id]
	if !exists {
		return fmt.Errorf("template not found: %s", id)
	}

	// 更新分类计数
	if cat, exists := tm.categories[tmpl.Category]; exists && cat.AppCount > 0 {
		cat.AppCount--
	}

	delete(tm.templates, id)
	log.Printf("Template unregistered: %s", id)
	return nil
}

// GetTemplateCount 获取模板数量.
func (tm *TemplateManager) GetTemplateCount() int {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return len(tm.templates)
}

// SearchTemplates 搜索模板.
func (tm *TemplateManager) SearchTemplates(query string) []*AppTemplate {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	results := make([]*AppTemplate, 0)
	query = toLowerStr(query)

	for _, tmpl := range tm.templates {
		if containsIgnoreCase(tmpl.Name, query) ||
			containsIgnoreCase(tmpl.DisplayName, query) ||
			containsIgnoreCase(tmpl.Description, query) {
			results = append(results, tmpl)
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Downloads > results[j].Downloads
	})

	return results
}

func toLowerStr(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		result[i] = toLower(s[i])
	}
	return string(result)
}
