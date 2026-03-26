// Package search 提供全局搜索服务
// 包含文档搜索支持
package search

import (
	"strings"
	"sync"
	"time"
)

// DocumentItem 文档项.
type DocumentItem struct {
	ID          string            `json:"id"`          // 唯一标识
	Title       string            `json:"title"`       // 文档标题
	Content     string            `json:"content"`     // 文档内容（摘要）
	Path        string            `json:"path"`        // 文档路径/URL
	Type        string            `json:"type"`        // 文档类型: guide, api, faq, changelog, tutorial
	Category    string            `json:"category"`    // 文档分类
	Tags        []string          `json:"tags"`        // 文档标签
	Keywords    []string          `json:"keywords"`    // 搜索关键词
	Icon        string            `json:"icon"`        // 图标
	Locale      string            `json:"locale"`      // 语言环境
	Version     string            `json:"version"`     // 文档版本
	UpdatedAt   time.Time         `json:"updatedAt"`   // 更新时间
	Section     string            `json:"section"`     // 文档区块
	Order       int               `json:"order"`       // 排序顺序
	Metadata    map[string]string `json:"metadata"`    // 元数据
}

// DocRegistry 文档注册表.
type DocRegistry struct {
	docs []DocumentItem
	mu   sync.RWMutex
}

// NewDocRegistry 创建文档注册表.
func NewDocRegistry() *DocRegistry {
	registry := &DocRegistry{
		docs: make([]DocumentItem, 0),
	}
	registry.initDefaultDocs()
	return registry
}

// initDefaultDocs 初始化默认文档.
func (r *DocRegistry) initDefaultDocs() {
	// 快速入门文档
	r.Register([]DocumentItem{
		{
			ID:        "doc.quickstart.getting-started",
			Title:     "快速入门",
			Content:   "了解如何在5分钟内完成NAS系统的基本配置，包括初始化设置、创建存储池、配置网络等。",
			Path:      "/docs/getting-started",
			Type:      "guide",
			Category:  "quickstart",
			Tags:      []string{"入门", "快速开始", "初始化"},
			Keywords:  []string{"入门", "快速", "开始", "getting started", "初始化", "配置"},
			Icon:      "rocket",
			Locale:    "zh-CN",
			Section:   "快速入门",
			Order:     1,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.quickstart.first-pool",
			Title:     "创建第一个存储池",
			Content:   "详细指南：如何创建你的第一个存储池，选择合适的RAID级别，优化存储性能。",
			Path:      "/docs/getting-started/first-pool",
			Type:      "tutorial",
			Category:  "quickstart",
			Tags:      []string{"存储池", "RAID", "创建"},
			Keywords:  []string{"存储池", "RAID", "创建", "pool", "磁盘"},
			Icon:      "database",
			Locale:    "zh-CN",
			Section:   "快速入门",
			Order:     2,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.quickstart.first-share",
			Title:     "创建第一个共享",
			Content:   "学习如何创建SMB/NFS共享，让其他设备可以访问NAS上的文件。",
			Path:      "/docs/getting-started/first-share",
			Type:      "tutorial",
			Category:  "quickstart",
			Tags:      []string{"共享", "SMB", "NFS"},
			Keywords:  []string{"共享", "SMB", "NFS", "share", "文件共享"},
			Icon:      "share-alt",
			Locale:    "zh-CN",
			Section:   "快速入门",
			Order:     3,
			UpdatedAt: time.Now(),
		},
	}...)

	// 存储文档
	r.Register([]DocumentItem{
		{
			ID:        "doc.storage.pools",
			Title:     "存储池管理",
			Content:   "全面了解存储池的概念、创建、管理和维护。包括RAID级别选择、存储池扩展、性能优化等。",
			Path:      "/docs/storage/pools",
			Type:      "guide",
			Category:  "storage",
			Tags:      []string{"存储池", "RAID", "ZFS"},
			Keywords:  []string{"存储池", "pool", "RAID", "ZFS", "磁盘管理"},
			Icon:      "database",
			Locale:    "zh-CN",
			Section:   "存储管理",
			Order:     1,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.storage.datasets",
			Title:     "数据集管理",
			Content:   "学习如何使用数据集组织数据，配置压缩、配额、快照策略等高级功能。",
			Path:      "/docs/storage/datasets",
			Type:      "guide",
			Category:  "storage",
			Tags:      []string{"数据集", "配额", "压缩"},
			Keywords:  []string{"数据集", "dataset", "配额", "压缩", "快照"},
			Icon:      "folder",
			Locale:    "zh-CN",
			Section:   "存储管理",
			Order:     2,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.storage.snapshots",
			Title:     "快照管理",
			Content:   "了解快照的工作原理，如何创建、恢复和管理快照，以及快照策略的最佳实践。",
			Path:      "/docs/storage/snapshots",
			Type:      "guide",
			Category:  "storage",
			Tags:      []string{"快照", "备份", "恢复"},
			Keywords:  []string{"快照", "snapshot", "备份", "恢复", "数据保护"},
			Icon:      "camera",
			Locale:    "zh-CN",
			Section:   "存储管理",
			Order:     3,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.storage.raid",
			Title:     "RAID级别详解",
			Content:   "详细比较各种RAID级别的特点、性能、可靠性，帮助你选择最适合的RAID配置。",
			Path:      "/docs/storage/raid-levels",
			Type:      "guide",
			Category:  "storage",
			Tags:      []string{"RAID", "冗余", "性能"},
			Keywords:  []string{"RAID", "RAID0", "RAID1", "RAID5", "RAID6", "RAID10", "RAIDZ"},
			Icon:      "layer-group",
			Locale:    "zh-CN",
			Section:   "存储管理",
			Order:     4,
			UpdatedAt: time.Now(),
		},
	}...)

	// 网络文档
	r.Register([]DocumentItem{
		{
			ID:        "doc.network.interfaces",
			Title:     "网络接口配置",
			Content:   "配置网络接口、IP地址、网关、DNS等网络设置，支持静态IP和DHCP。",
			Path:      "/docs/network/interfaces",
			Type:      "guide",
			Category:  "network",
			Tags:      []string{"网络", "IP", "DNS"},
			Keywords:  []string{"网络", "interface", "IP", "DNS", "网关", "DHCP"},
			Icon:      "network-wired",
			Locale:    "zh-CN",
			Section:   "网络配置",
			Order:     1,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.network.firewall",
			Title:     "防火墙配置",
			Content:   "配置防火墙规则，管理端口开放，保护系统安全。",
			Path:      "/docs/network/firewall",
			Type:      "guide",
			Category:  "network",
			Tags:      []string{"防火墙", "安全", "端口"},
			Keywords:  []string{"防火墙", "firewall", "端口", "安全", "iptables"},
			Icon:      "shield-alt",
			Locale:    "zh-CN",
			Section:   "网络配置",
			Order:     2,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.network.remote-access",
			Title:     "远程访问配置",
			Content:   "配置远程访问方式，包括VPN、内网穿透、DDNS等。",
			Path:      "/docs/network/remote-access",
			Type:      "guide",
			Category:  "network",
			Tags:      []string{"远程访问", "VPN", "穿透"},
			Keywords:  []string{"远程访问", "VPN", "内网穿透", "DDNS", "远程"},
			Icon:      "globe",
			Locale:    "zh-CN",
			Section:   "网络配置",
			Order:     3,
			UpdatedAt: time.Now(),
		},
	}...)

	// 用户权限文档
	r.Register([]DocumentItem{
		{
			ID:        "doc.users.management",
			Title:     "用户管理",
			Content:   "创建和管理用户账户，配置用户权限和访问控制。",
			Path:      "/docs/users/management",
			Type:      "guide",
			Category:  "users",
			Tags:      []string{"用户", "账户", "权限"},
			Keywords:  []string{"用户", "user", "账户", "权限", "管理"},
			Icon:      "users",
			Locale:    "zh-CN",
			Section:   "用户与权限",
			Order:     1,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.users.groups",
			Title:     "用户组管理",
			Content:   "创建和管理用户组，简化权限管理。",
			Path:      "/docs/users/groups",
			Type:      "guide",
			Category:  "users",
			Tags:      []string{"用户组", "权限", "共享"},
			Keywords:  []string{"用户组", "group", "权限组", "权限管理"},
			Icon:      "users-cog",
			Locale:    "zh-CN",
			Section:   "用户与权限",
			Order:     2,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.users.rbac",
			Title:     "基于角色的访问控制",
			Content:   "了解RBAC概念，配置角色和权限，实现精细化的访问控制。",
			Path:      "/docs/users/rbac",
			Type:      "guide",
			Category:  "users",
			Tags:      []string{"RBAC", "角色", "权限"},
			Keywords:  []string{"RBAC", "角色", "role", "访问控制", "权限"},
			Icon:      "key",
			Locale:    "zh-CN",
			Section:   "用户与权限",
			Order:     3,
			UpdatedAt: time.Now(),
		},
	}...)

	// 容器文档
	r.Register([]DocumentItem{
		{
			ID:        "doc.containers.docker",
			Title:     "Docker容器管理",
			Content:   "全面了解Docker容器的使用，包括容器创建、镜像管理、网络配置等。",
			Path:      "/docs/containers/docker",
			Type:      "guide",
			Category:  "containers",
			Tags:      []string{"Docker", "容器", "镜像"},
			Keywords:  []string{"Docker", "容器", "container", "镜像", "image"},
			Icon:      "docker",
			Locale:    "zh-CN",
			Section:   "容器与虚拟化",
			Order:     1,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.containers.compose",
			Title:     "Docker Compose使用",
			Content:   "学习使用Docker Compose编排多容器应用，简化部署流程。",
			Path:      "/docs/containers/compose",
			Type:      "guide",
			Category:  "containers",
			Tags:      []string{"Compose", "编排", "部署"},
			Keywords:  []string{"Compose", "Docker Compose", "编排", "stack"},
			Icon:      "layer-group",
			Locale:    "zh-CN",
			Section:   "容器与虚拟化",
			Order:     2,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.containers.registry",
			Title:     "镜像仓库管理",
			Content:   "配置和管理私有镜像仓库，推送和拉取镜像。",
			Path:      "/docs/containers/registry",
			Type:      "guide",
			Category:  "containers",
			Tags:      []string{"镜像仓库", "Registry", "私有"},
			Keywords:  []string{"镜像仓库", "Registry", "私有仓库", "镜像"},
			Icon:      "archive",
			Locale:    "zh-CN",
			Section:   "容器与虚拟化",
			Order:     3,
			UpdatedAt: time.Now(),
		},
	}...)

	// 备份文档
	r.Register([]DocumentItem{
		{
			ID:        "doc.backup.tasks",
			Title:     "备份任务配置",
			Content:   "创建和管理备份任务，设置备份策略和计划。",
			Path:      "/docs/backup/tasks",
			Type:      "guide",
			Category:  "backup",
			Tags:      []string{"备份", "任务", "计划"},
			Keywords:  []string{"备份", "backup", "任务", "计划", "数据保护"},
			Icon:      "save",
			Locale:    "zh-CN",
			Section:   "数据保护",
			Order:     1,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.backup.cloudsync",
			Title:     "云同步配置",
			Content:   "配置云存储同步，支持多种云服务商，实现混合云备份。",
			Path:      "/docs/backup/cloudsync",
			Type:      "guide",
			Category:  "backup",
			Tags:      []string{"云同步", "云存储", "S3"},
			Keywords:  []string{"云同步", "cloudsync", "云存储", "S3", "OSS", "混合云"},
			Icon:      "cloud",
			Locale:    "zh-CN",
			Section:   "数据保护",
			Order:     2,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.backup.replication",
			Title:     "数据复制",
			Content:   "配置远程复制任务，实现异地数据同步和灾备。",
			Path:      "/docs/backup/replication",
			Type:      "guide",
			Category:  "backup",
			Tags:      []string{"复制", "同步", "灾备"},
			Keywords:  []string{"复制", "replication", "同步", "灾备", "远程复制"},
			Icon:      "sync",
			Locale:    "zh-CN",
			Section:   "数据保护",
			Order:     3,
			UpdatedAt: time.Now(),
		},
	}...)

	// 安全文档
	r.Register([]DocumentItem{
		{
			ID:        "doc.security.ssh",
			Title:     "SSH安全配置",
			Content:   "配置SSH服务，设置密钥认证，提高系统安全性。",
			Path:      "/docs/security/ssh",
			Type:      "guide",
			Category:  "security",
			Tags:      []string{"SSH", "安全", "密钥"},
			Keywords:  []string{"SSH", "安全", "密钥", "key", "远程"},
			Icon:      "terminal",
			Locale:    "zh-CN",
			Section:   "安全配置",
			Order:     1,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.security.ssl",
			Title:     "SSL证书管理",
			Content:   "管理SSL/TLS证书，配置HTTPS，申请和续期证书。",
			Path:      "/docs/security/ssl",
			Type:      "guide",
			Category:  "security",
			Tags:      []string{"SSL", "证书", "HTTPS"},
			Keywords:  []string{"SSL", "TLS", "证书", "certificate", "HTTPS"},
			Icon:      "lock",
			Locale:    "zh-CN",
			Section:   "安全配置",
			Order:     2,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.security.2fa",
			Title:     "双因素认证",
			Content:   "配置双因素认证(2FA)，增强账户安全性。",
			Path:      "/docs/security/2fa",
			Type:      "guide",
			Category:  "security",
			Tags:      []string{"双因素", "2FA", "TOTP"},
			Keywords:  []string{"双因素", "2FA", "TOTP", "认证", "安全"},
			Icon:      "mobile-alt",
			Locale:    "zh-CN",
			Section:   "安全配置",
			Order:     3,
			UpdatedAt: time.Now(),
		},
	}...)

	// API 文档
	r.Register([]DocumentItem{
		{
			ID:        "doc.api.reference",
			Title:     "API参考文档",
			Content:   "完整的API参考文档，包括所有端点的详细说明和示例。",
			Path:      "/docs/api/reference",
			Type:      "api",
			Category:  "api",
			Tags:      []string{"API", "REST", "接口"},
			Keywords:  []string{"API", "REST", "接口", "文档", "endpoint"},
			Icon:      "code",
			Locale:    "zh-CN",
			Section:   "开发者文档",
			Order:     1,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.api.authentication",
			Title:     "API认证",
			Content:   "了解API认证方式，获取和使用访问令牌。",
			Path:      "/docs/api/authentication",
			Type:      "api",
			Category:  "api",
			Tags:      []string{"API", "认证", "Token"},
			Keywords:  []string{"API", "认证", "auth", "token", "JWT"},
			Icon:      "key",
			Locale:    "zh-CN",
			Section:   "开发者文档",
			Order:     2,
			UpdatedAt: time.Now(),
		},
	}...)

	// FAQ
	r.Register([]DocumentItem{
		{
			ID:        "doc.faq.common",
			Title:     "常见问题",
			Content:   "常见问题解答，快速找到问题解决方案。",
			Path:      "/docs/faq",
			Type:      "faq",
			Category:  "faq",
			Tags:      []string{"FAQ", "问题", "解答"},
			Keywords:  []string{"FAQ", "常见问题", "问题", "解答", "帮助"},
			Icon:      "question-circle",
			Locale:    "zh-CN",
			Section:   "帮助",
			Order:     1,
			UpdatedAt: time.Now(),
		},
		{
			ID:        "doc.faq.troubleshooting",
			Title:     "故障排查指南",
			Content:   "常见故障的诊断和解决方法。",
			Path:      "/docs/troubleshooting",
			Type:      "faq",
			Category:  "faq",
			Tags:      []string{"故障", "排查", "诊断"},
			Keywords:  []string{"故障", "排查", "troubleshooting", "诊断", "问题解决"},
			Icon:      "tools",
			Locale:    "zh-CN",
			Section:   "帮助",
			Order:     2,
			UpdatedAt: time.Now(),
		},
	}...)

	// 变更日志
	r.Register([]DocumentItem{
		{
			ID:        "doc.changelog.latest",
			Title:     "更新日志",
			Content:   "查看最新版本的更新内容和历史变更记录。",
			Path:      "/docs/changelog",
			Type:      "changelog",
			Category:  "changelog",
			Tags:      []string{"更新", "版本", "变更"},
			Keywords:  []string{"更新日志", "changelog", "版本", "更新", "新功能"},
			Icon:      "history",
			Locale:    "zh-CN",
			Section:   "更新",
			Order:     1,
			UpdatedAt: time.Now(),
		},
	}...)
}

// Register 注册文档.
func (r *DocRegistry) Register(docs ...DocumentItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.docs = append(r.docs, docs...)
}

// GetAll 获取所有文档.
func (r *DocRegistry) GetAll() []DocumentItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]DocumentItem, len(r.docs))
	copy(result, r.docs)
	return result
}

// DocSearchResult 文档搜索结果.
type DocSearchResult struct {
	Doc         DocumentItem `json:"doc"`
	Score       float64      `json:"score"`
	MatchType   string       `json:"matchType"` // title, content, tag, keyword
	MatchField  string       `json:"matchField"`
	MatchedText string       `json:"matchedText"`
}

// SearchDocs 搜索文档.
func (r *DocRegistry) SearchDocs(query string, limit int) []DocSearchResult {
	if query == "" {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	results := make([]DocSearchResult, 0)

	for _, doc := range r.docs {
		score := 0.0
		matchType := ""
		matchField := ""
		matchedText := ""

		// 检查标题匹配
		if strings.Contains(strings.ToLower(doc.Title), query) {
			score = 1.0
			matchType = "title"
			matchField = "title"
			matchedText = doc.Title
		}

		// 检查关键词匹配
		if score == 0 {
			for _, kw := range doc.Keywords {
				if strings.Contains(strings.ToLower(kw), query) {
					score = 0.9
					matchType = "keyword"
					matchField = "keyword"
					matchedText = kw
					break
				}
			}
		}

		// 检查内容匹配
		if score == 0 && strings.Contains(strings.ToLower(doc.Content), query) {
			score = 0.7
			matchType = "content"
			matchField = "content"
			matchedText = truncateText(doc.Content, 100)
		}

		// 检查标签匹配
		if score == 0 {
			for _, tag := range doc.Tags {
				if strings.Contains(strings.ToLower(tag), query) {
					score = 0.6
					matchType = "tag"
					matchField = "tag"
					matchedText = tag
					break
				}
			}
		}

		// 检查分类匹配
		if score == 0 && strings.Contains(strings.ToLower(doc.Category), query) {
			score = 0.5
			matchType = "category"
			matchField = "category"
			matchedText = doc.Category
		}

		// 检查区块匹配
		if score == 0 && strings.Contains(strings.ToLower(doc.Section), query) {
			score = 0.4
			matchType = "section"
			matchField = "section"
			matchedText = doc.Section
		}

		if score > 0 {
			results = append(results, DocSearchResult{
				Doc:         doc,
				Score:       score,
				MatchType:   matchType,
				MatchField:  matchField,
				MatchedText: matchedText,
			})
		}
	}

	// 按分数排序
	sortDocResults(results)

	// 限制数量
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// GetByType 按类型获取文档.
func (r *DocRegistry) GetByType(docType string) []DocumentItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []DocumentItem
	for _, doc := range r.docs {
		if doc.Type == docType {
			results = append(results, doc)
		}
	}
	return results
}

// GetByCategory 按分类获取文档.
func (r *DocRegistry) GetByCategory(category string) []DocumentItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []DocumentItem
	for _, doc := range r.docs {
		if doc.Category == category {
			results = append(results, doc)
		}
	}
	return results
}

// GetBySection 按区块获取文档.
func (r *DocRegistry) GetBySection(section string) []DocumentItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []DocumentItem
	for _, doc := range r.docs {
		if doc.Section == section {
			results = append(results, doc)
		}
	}
	return results
}

// GetDocCategories 获取文档分类.
func (r *DocRegistry) GetDocCategories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	categoryMap := make(map[string]bool)
	for _, doc := range r.docs {
		categoryMap[doc.Category] = true
	}

	categories := make([]string, 0, len(categoryMap))
	for cat := range categoryMap {
		categories = append(categories, cat)
	}
	return categories
}

// GetDocSections 获取文档区块.
func (r *DocRegistry) GetDocSections() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	sectionMap := make(map[string]bool)
	for _, doc := range r.docs {
		sectionMap[doc.Section] = true
	}

	sections := make([]string, 0, len(sectionMap))
	for sec := range sectionMap {
		sections = append(sections, sec)
	}
	return sections
}

// sortDocResults 排序文档搜索结果.
func sortDocResults(results []DocSearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// truncateText 截断文本.
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}