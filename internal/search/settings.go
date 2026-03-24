package search

import (
	"strings"
	"sync"
)

// SettingItem 设置项
type SettingItem struct {
	ID          string   `json:"id"`          // 唯一标识
	Name        string   `json:"name"`        // 设置名称
	Description string   `json:"description"` // 设置描述
	Category    string   `json:"category"`    // 分类: storage, network, system, security, etc.
	SubCategory string   `json:"subCategory"` // 子分类
	Path        string   `json:"path"`        // 设置页面路径
	Keywords    []string `json:"keywords"`    // 搜索关键词
	Icon        string   `json:"icon"`        // 图标
	Section     string   `json:"section"`     // 设置区块
}

// SettingsRegistry 设置注册表
type SettingsRegistry struct {
	items []SettingItem
	mu    sync.RWMutex
}

// NewSettingsRegistry 创建设置注册表
func NewSettingsRegistry() *SettingsRegistry {
	registry := &SettingsRegistry{
		items: make([]SettingItem, 0),
	}
	// 初始化默认设置项
	registry.initDefaultSettings()
	return registry
}

// initDefaultSettings 初始化默认设置项
func (r *SettingsRegistry) initDefaultSettings() {
	// 存储设置
	r.Register([]SettingItem{
		{
			ID:          "storage.pools",
			Name:        "存储池管理",
			Description: "创建、编辑和删除存储池，配置RAID级别",
			Category:    "storage",
			SubCategory: "pools",
			Path:        "/storage/pools",
			Keywords:    []string{"存储", "池", "RAID", "磁盘", "storage", "pool"},
			Icon:        "database",
			Section:     "存储",
		},
		{
			ID:          "storage.datasets",
			Name:        "数据集管理",
			Description: "创建和管理数据集，配置配额和压缩",
			Category:    "storage",
			SubCategory: "datasets",
			Path:        "/storage/datasets",
			Keywords:    []string{"数据集", "dataset", "配额", "压缩", "快照"},
			Icon:        "folder",
			Section:     "存储",
		},
		{
			ID:          "storage.snapshots",
			Name:        "快照管理",
			Description: "创建、恢复和删除存储快照",
			Category:    "storage",
			SubCategory: "snapshots",
			Path:        "/storage/snapshots",
			Keywords:    []string{"快照", "snapshot", "备份", "恢复"},
			Icon:        "camera",
			Section:     "存储",
		},
		{
			ID:          "storage.shares",
			Name:        "共享管理",
			Description: "配置SMB、NFS、WebDAV等共享服务",
			Category:    "storage",
			SubCategory: "shares",
			Path:        "/storage/shares",
			Keywords:    []string{"共享", "share", "SMB", "NFS", "WebDAV", "CIFS"},
			Icon:        "share-alt",
			Section:     "存储",
		},
		{
			ID:          "storage.quota",
			Name:        "配额管理",
			Description: "设置用户和组的存储配额限制",
			Category:    "storage",
			SubCategory: "quota",
			Path:        "/storage/quota",
			Keywords:    []string{"配额", "quota", "限制", "容量"},
			Icon:        "pie-chart",
			Section:     "存储",
		},
	}...)

	// 网络设置
	r.Register([]SettingItem{
		{
			ID:          "network.interfaces",
			Name:        "网络接口",
			Description: "配置网络接口、IP地址和DNS",
			Category:    "network",
			SubCategory: "interfaces",
			Path:        "/network/interfaces",
			Keywords:    []string{"网络", "接口", "IP", "DNS", "网卡", "interface"},
			Icon:        "network",
			Section:     "网络",
		},
		{
			ID:          "network.static-routes",
			Name:        "静态路由",
			Description: "配置静态路由表",
			Category:    "network",
			SubCategory: "routes",
			Path:        "/network/routes",
			Keywords:    []string{"路由", "route", "网关", "gateway"},
			Icon:        "route",
			Section:     "网络",
		},
		{
			ID:          "network.dns",
			Name:        "DNS设置",
			Description: "配置DNS服务器和域名解析",
			Category:    "network",
			SubCategory: "dns",
			Path:        "/network/dns",
			Keywords:    []string{"DNS", "域名", "解析", "nameserver"},
			Icon:        "globe",
			Section:     "网络",
		},
		{
			ID:          "network.proxy",
			Name:        "代理设置",
			Description: "配置HTTP/HTTPS代理服务器",
			Category:    "network",
			SubCategory: "proxy",
			Path:        "/network/proxy",
			Keywords:    []string{"代理", "proxy", "HTTP", "HTTPS"},
			Icon:        "shield",
			Section:     "网络",
		},
		{
			ID:          "network.firewall",
			Name:        "防火墙",
			Description: "配置防火墙规则和端口转发",
			Category:    "network",
			SubCategory: "firewall",
			Path:        "/network/firewall",
			Keywords:    []string{"防火墙", "firewall", "端口", "规则", "iptables"},
			Icon:        "fire",
			Section:     "网络",
		},
	}...)

	// 系统设置
	r.Register([]SettingItem{
		{
			ID:          "system.general",
			Name:        "常规设置",
			Description: "系统名称、时区、语言等基本设置",
			Category:    "system",
			SubCategory: "general",
			Path:        "/system/general",
			Keywords:    []string{"常规", "general", "时区", "语言", "主机名", "hostname"},
			Icon:        "cog",
			Section:     "系统",
		},
		{
			ID:          "system.users",
			Name:        "用户管理",
			Description: "创建和管理系统用户账户",
			Category:    "system",
			SubCategory: "users",
			Path:        "/system/users",
			Keywords:    []string{"用户", "user", "账户", "account", "权限"},
			Icon:        "user",
			Section:     "系统",
		},
		{
			ID:          "system.groups",
			Name:        "用户组管理",
			Description: "创建和管理用户组",
			Category:    "system",
			SubCategory: "groups",
			Path:        "/system/groups",
			Keywords:    []string{"组", "group", "用户组", "权限组"},
			Icon:        "users",
			Section:     "系统",
		},
		{
			ID:          "system.datetime",
			Name:        "日期时间",
			Description: "设置系统时间和NTP服务器",
			Category:    "system",
			SubCategory: "datetime",
			Path:        "/system/datetime",
			Keywords:    []string{"时间", "日期", "NTP", "时区", "timezone"},
			Icon:        "clock",
			Section:     "系统",
		},
		{
			ID:          "system.cron",
			Name:        "计划任务",
			Description: "创建和管理定时任务",
			Category:    "system",
			SubCategory: "cron",
			Path:        "/system/cron",
			Keywords:    []string{"定时", "cron", "任务", "schedule", "计划"},
			Icon:        "calendar",
			Section:     "系统",
		},
		{
			ID:          "system.alerts",
			Name:        "告警设置",
			Description: "配置系统告警和通知",
			Category:    "system",
			SubCategory: "alerts",
			Path:        "/system/alerts",
			Keywords:    []string{"告警", "alert", "通知", "notification", "邮件"},
			Icon:        "bell",
			Section:     "系统",
		},
		{
			ID:          "system.logs",
			Name:        "系统日志",
			Description: "查看和管理系统日志",
			Category:    "system",
			SubCategory: "logs",
			Path:        "/system/logs",
			Keywords:    []string{"日志", "log", "系统日志", "audit"},
			Icon:        "file-text",
			Section:     "系统",
		},
	}...)

	// 安全设置
	r.Register([]SettingItem{
		{
			ID:          "security.ssh",
			Name:        "SSH设置",
			Description: "配置SSH服务和密钥认证",
			Category:    "security",
			SubCategory: "ssh",
			Path:        "/security/ssh",
			Keywords:    []string{"SSH", "密钥", "key", "远程", "remote", "终端"},
			Icon:        "terminal",
			Section:     "安全",
		},
		{
			ID:          "security.ssl",
			Name:        "SSL证书",
			Description: "管理SSL/TLS证书",
			Category:    "security",
			SubCategory: "ssl",
			Path:        "/security/certificates",
			Keywords:    []string{"SSL", "TLS", "证书", "certificate", "HTTPS", "加密"},
			Icon:        "lock",
			Section:     "安全",
		},
		{
			ID:          "security.2fa",
			Name:        "双因素认证",
			Description: "配置两步验证和TOTP",
			Category:    "security",
			SubCategory: "2fa",
			Path:        "/security/2fa",
			Keywords:    []string{"双因素", "2FA", "TOTP", "验证", "认证", "MFA"},
			Icon:        "mobile",
			Section:     "安全",
		},
		{
			ID:          "security.audit",
			Name:        "审计日志",
			Description: "查看系统操作审计记录",
			Category:    "security",
			SubCategory: "audit",
			Path:        "/security/audit",
			Keywords:    []string{"审计", "audit", "日志", "操作记录"},
			Icon:        "eye",
			Section:     "安全",
		},
		{
			ID:          "security.rbac",
			Name:        "权限管理",
			Description: "配置基于角色的访问控制",
			Category:    "security",
			SubCategory: "rbac",
			Path:        "/security/rbac",
			Keywords:    []string{"权限", "RBAC", "角色", "role", "访问控制"},
			Icon:        "key",
			Section:     "安全",
		},
	}...)

	// 服务设置
	r.Register([]SettingItem{
		{
			ID:          "services.smb",
			Name:        "SMB/CIFS服务",
			Description: "配置SMB文件共享服务",
			Category:    "services",
			SubCategory: "smb",
			Path:        "/services/smb",
			Keywords:    []string{"SMB", "CIFS", "Windows", "共享", "share"},
			Icon:        "windows",
			Section:     "服务",
		},
		{
			ID:          "services.nfs",
			Name:        "NFS服务",
			Description: "配置NFS网络文件系统服务",
			Category:    "services",
			SubCategory: "nfs",
			Path:        "/services/nfs",
			Keywords:    []string{"NFS", "Linux", "共享", "exports"},
			Icon:        "linux",
			Section:     "服务",
		},
		{
			ID:          "services.webdav",
			Name:        "WebDAV服务",
			Description: "配置WebDAV文件服务",
			Category:    "services",
			SubCategory: "webdav",
			Path:        "/services/webdav",
			Keywords:    []string{"WebDAV", "HTTP", "文件", "共享"},
			Icon:        "cloud",
			Section:     "服务",
		},
		{
			ID:          "services.ftp",
			Name:        "FTP服务",
			Description: "配置FTP文件传输服务",
			Category:    "services",
			SubCategory: "ftp",
			Path:        "/services/ftp",
			Keywords:    []string{"FTP", "SFTP", "文件传输", "传输"},
			Icon:        "upload",
			Section:     "服务",
		},
		{
			ID:          "services.iscsi",
			Name:        "iSCSI服务",
			Description: "配置iSCSI块存储服务",
			Category:    "services",
			SubCategory: "iscsi",
			Path:        "/services/iscsi",
			Keywords:    []string{"iSCSI", "块存储", "SAN", "Target"},
			Icon:        "database",
			Section:     "服务",
		},
	}...)

	// 容器/虚拟化设置
	r.Register([]SettingItem{
		{
			ID:          "containers.docker",
			Name:        "Docker容器",
			Description: "管理Docker容器和镜像",
			Category:    "containers",
			SubCategory: "docker",
			Path:        "/containers/docker",
			Keywords:    []string{"Docker", "容器", "container", "镜像", "image"},
			Icon:        "docker",
			Section:     "容器",
		},
		{
			ID:          "containers.compose",
			Name:        "Docker Compose",
			Description: "管理Docker Compose项目",
			Category:    "containers",
			SubCategory: "compose",
			Path:        "/containers/compose",
			Keywords:    []string{"Compose", "Docker", "编排", "stack"},
			Icon:        "layers",
			Section:     "容器",
		},
		{
			ID:          "containers.registry",
			Name:        "镜像仓库",
			Description: "管理容器镜像仓库",
			Category:    "containers",
			SubCategory: "registry",
			Path:        "/containers/registry",
			Keywords:    []string{"Registry", "镜像", "仓库", "repository"},
			Icon:        "archive",
			Section:     "容器",
		},
		{
			ID:          "containers.networks",
			Name:        "容器网络",
			Description: "管理Docker网络",
			Category:    "containers",
			SubCategory: "networks",
			Path:        "/containers/networks",
			Keywords:    []string{"Docker", "网络", "network", "bridge", "overlay"},
			Icon:        "project-diagram",
			Section:     "容器",
		},
	}...)

	// 数据保护
	r.Register([]SettingItem{
		{
			ID:          "protection.backup",
			Name:        "备份任务",
			Description: "配置数据备份任务",
			Category:    "protection",
			SubCategory: "backup",
			Path:        "/protection/backup",
			Keywords:    []string{"备份", "backup", "数据保护", "恢复"},
			Icon:        "save",
			Section:     "数据保护",
		},
		{
			ID:          "protection.replication",
			Name:        "复制任务",
			Description: "配置数据远程复制",
			Category:    "protection",
			SubCategory: "replication",
			Path:        "/protection/replication",
			Keywords:    []string{"复制", "replication", "同步", "sync", "远程"},
			Icon:        "sync",
			Section:     "数据保护",
		},
		{
			ID:          "protection.cloudsync",
			Name:        "云同步",
			Description: "配置云存储同步任务",
			Category:    "protection",
			SubCategory: "cloudsync",
			Path:        "/protection/cloudsync",
			Keywords:    []string{"云同步", "cloudsync", "云存储", "S3", "OSS"},
			Icon:        "cloud-upload-alt",
			Section:     "数据保护",
		},
		{
			ID:          "protection.scrub",
			Name:        "数据校验",
			Description: "配置存储池数据校验任务",
			Category:    "protection",
			SubCategory: "scrub",
			Path:        "/protection/scrub",
			Keywords:    []string{"校验", "scrub", "数据完整性", "ZFS"},
			Icon:        "check-circle",
			Section:     "数据保护",
		},
	}...)

	// 应用设置
	r.Register([]SettingItem{
		{
			ID:          "apps.store",
			Name:        "应用商店",
			Description: "浏览和安装应用",
			Category:    "apps",
			SubCategory: "store",
			Path:        "/apps/store",
			Keywords:    []string{"应用", "商店", "app", "store", "安装"},
			Icon:        "store",
			Section:     "应用",
		},
		{
			ID:          "apps.installed",
			Name:        "已安装应用",
			Description: "管理已安装的应用",
			Category:    "apps",
			SubCategory: "installed",
			Path:        "/apps/installed",
			Keywords:    []string{"应用", "installed", "已安装", "app"},
			Icon:        "box",
			Section:     "应用",
		},
		{
			ID:          "apps.templates",
			Name:        "应用模板",
			Description: "管理应用部署模板",
			Category:    "apps",
			SubCategory: "templates",
			Path:        "/apps/templates",
			Keywords:    []string{"模板", "template", "应用", "部署"},
			Icon:        "file-alt",
			Section:     "应用",
		},
	}...)

	// 监控设置
	r.Register([]SettingItem{
		{
			ID:          "monitoring.dashboard",
			Name:        "监控仪表板",
			Description: "查看系统资源监控",
			Category:    "monitoring",
			SubCategory: "dashboard",
			Path:        "/monitoring/dashboard",
			Keywords:    []string{"监控", "dashboard", "仪表板", "资源", "CPU", "内存"},
			Icon:        "tachometer-alt",
			Section:     "监控",
		},
		{
			ID:          "monitoring.reporting",
			Name:        "报表中心",
			Description: "查看和导出系统报表",
			Category:    "monitoring",
			SubCategory: "reporting",
			Path:        "/monitoring/reports",
			Keywords:    []string{"报表", "report", "统计", "分析"},
			Icon:        "chart-bar",
			Section:     "监控",
		},
		{
			ID:          "monitoring.alerts",
			Name:        "告警中心",
			Description: "查看和管理系统告警",
			Category:    "monitoring",
			SubCategory: "alerts",
			Path:        "/monitoring/alerts",
			Keywords:    []string{"告警", "alert", "警告", "通知"},
			Icon:        "exclamation-triangle",
			Section:     "监控",
		},
	}...)
}

// Register 注册设置项
func (r *SettingsRegistry) Register(items ...SettingItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, items...)
}

// GetAll 获取所有设置项
func (r *SettingsRegistry) GetAll() []SettingItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]SettingItem, len(r.items))
	copy(result, r.items)
	return result
}

// SettingsSearchResult 设置搜索结果
type SettingsSearchResult struct {
	Setting     SettingItem `json:"setting"`
	Score       float64     `json:"score"`
	MatchType   string      `json:"matchType"` // name, description, keyword
	MatchField  string      `json:"matchField"`
	MatchedText string      `json:"matchedText"`
}

// SearchSettings 搜索设置
func (r *SettingsRegistry) SearchSettings(query string, limit int) []SettingsSearchResult {
	if query == "" {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	results := make([]SettingsSearchResult, 0)

	for _, item := range r.items {
		score := 0.0
		matchType := ""
		matchField := ""
		matchedText := ""

		// 检查名称匹配
		if strings.Contains(strings.ToLower(item.Name), query) {
			score = 1.0
			matchType = "name"
			matchField = "name"
			matchedText = item.Name
		}

		// 检查ID匹配
		if score == 0 && strings.Contains(strings.ToLower(item.ID), query) {
			score = 0.9
			matchType = "id"
			matchField = "id"
			matchedText = item.ID
		}

		// 检查关键词匹配
		if score == 0 {
			for _, kw := range item.Keywords {
				if strings.Contains(strings.ToLower(kw), query) {
					score = 0.8
					matchType = "keyword"
					matchField = "keyword"
					matchedText = kw
					break
				}
			}
		}

		// 检查描述匹配
		if score == 0 && strings.Contains(strings.ToLower(item.Description), query) {
			score = 0.7
			matchType = "description"
			matchField = "description"
			matchedText = item.Description
		}

		// 检查分类匹配
		if score == 0 && strings.Contains(strings.ToLower(item.Category), query) {
			score = 0.6
			matchType = "category"
			matchField = "category"
			matchedText = item.Category
		}

		if score > 0 {
			results = append(results, SettingsSearchResult{
				Setting:     item,
				Score:       score,
				MatchType:   matchType,
				MatchField:  matchField,
				MatchedText: matchedText,
			})
		}
	}

	// 按分数排序
	sortSettingsResults(results)

	// 限制数量
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// GetByCategory 按分类获取设置项
func (r *SettingsRegistry) GetByCategory(category string) []SettingItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []SettingItem
	for _, item := range r.items {
		if item.Category == category {
			results = append(results, item)
		}
	}
	return results
}

// GetByPath 按路径获取设置项
func (r *SettingsRegistry) GetByPath(path string) *SettingItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for i := range r.items {
		if r.items[i].Path == path {
			return &r.items[i]
		}
	}
	return nil
}

// GetCategories 获取所有分类
func (r *SettingsRegistry) GetCategories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	categoryMap := make(map[string]bool)
	for _, item := range r.items {
		categoryMap[item.Category] = true
	}

	categories := make([]string, 0, len(categoryMap))
	for cat := range categoryMap {
		categories = append(categories, cat)
	}
	return categories
}

// sortSettingsResults 排序设置搜索结果
func sortSettingsResults(results []SettingsSearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}
