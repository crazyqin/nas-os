// Package search 提供全局搜索服务
// 包含 API 端点搜索支持
package search

import (
	"strings"
	"sync"
)

// APIEndpoint API端点项.
type APIEndpoint struct {
	ID          string            `json:"id"`          // 唯一标识
	Method      string            `json:"method"`      // HTTP 方法: GET, POST, PUT, DELETE
	Path        string            `json:"path"`        // API 路径
	Summary     string            `json:"summary"`     // 简要描述
	Description string            `json:"description"` // 详细描述
	Tags        []string          `json:"tags"`        // API 标签/分组
	Parameters  []APIParameter    `json:"parameters"`  // 参数列表
	Responses   map[int]APIResponse `json:"responses"`  // 响应定义
	Deprecated  bool              `json:"deprecated"`  // 是否废弃
	Version     string            `json:"version"`     // API 版本
	Keywords    []string          `json:"keywords"`    // 搜索关键词
}

// APIParameter API参数.
type APIParameter struct {
	Name        string `json:"name"`
	In          string `json:"in"` // path, query, header, body
	Required    bool   `json:"required"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// APIResponse API响应.
type APIResponse struct {
	Description string `json:"description"`
	Schema      string `json:"schema,omitempty"`
}

// APIRegistry API注册表.
type APIRegistry struct {
	endpoints []APIEndpoint
	mu        sync.RWMutex
}

// NewAPIRegistry 创建API注册表.
func NewAPIRegistry() *APIRegistry {
	registry := &APIRegistry{
		endpoints: make([]APIEndpoint, 0),
	}
	registry.initDefaultEndpoints()
	return registry
}

// initDefaultEndpoints 初始化默认API端点.
func (r *APIRegistry) initDefaultEndpoints() {
	// 存储 API
	r.Register([]APIEndpoint{
		{
			ID:          "api.storage.pools.list",
			Method:      "GET",
			Path:        "/api/v1/storage/pools",
			Summary:     "获取存储池列表",
			Description: "获取系统中所有存储池的详细信息，包括名称、容量、健康状态等",
			Tags:        []string{"storage", "pools"},
			Keywords:    []string{"存储池", "storage", "pool", "RAID", "列表"},
		},
		{
			ID:          "api.storage.pools.create",
			Method:      "POST",
			Path:        "/api/v1/storage/pools",
			Summary:     "创建存储池",
			Description: "创建新的存储池，配置RAID级别和磁盘成员",
			Tags:        []string{"storage", "pools"},
			Keywords:    []string{"存储池", "创建", "pool", "create", "RAID"},
		},
		{
			ID:          "api.storage.datasets.list",
			Method:      "GET",
			Path:        "/api/v1/storage/datasets",
			Summary:     "获取数据集列表",
			Description: "获取指定存储池下的所有数据集",
			Tags:        []string{"storage", "datasets"},
			Keywords:    []string{"数据集", "dataset", "列表"},
		},
		{
			ID:          "api.storage.snapshots.list",
			Method:      "GET",
			Path:        "/api/v1/storage/snapshots",
			Summary:     "获取快照列表",
			Description: "获取所有存储快照列表",
			Tags:        []string{"storage", "snapshots"},
			Keywords:    []string{"快照", "snapshot", "列表"},
		},
		{
			ID:          "api.storage.snapshots.create",
			Method:      "POST",
			Path:        "/api/v1/storage/snapshots",
			Summary:     "创建快照",
			Description: "为指定数据集创建快照",
			Tags:        []string{"storage", "snapshots"},
			Keywords:    []string{"快照", "创建", "snapshot", "create"},
		},
	}...)

	// 网络 API
	r.Register([]APIEndpoint{
		{
			ID:          "api.network.interfaces.list",
			Method:      "GET",
			Path:        "/api/v1/network/interfaces",
			Summary:     "获取网络接口列表",
			Description: "获取系统所有网络接口的配置和状态",
			Tags:        []string{"network", "interfaces"},
			Keywords:    []string{"网络接口", "interface", "网卡", "列表"},
		},
		{
			ID:          "api.network.interfaces.update",
			Method:      "PUT",
			Path:        "/api/v1/network/interfaces/{id}",
			Summary:     "更新网络接口配置",
			Description: "更新指定网络接口的IP地址、DNS等配置",
			Tags:        []string{"network", "interfaces"},
			Keywords:    []string{"网络接口", "更新", "interface", "IP", "DNS"},
		},
		{
			ID:          "api.network.dns.get",
			Method:      "GET",
			Path:        "/api/v1/network/dns",
			Summary:     "获取DNS配置",
			Description: "获取系统DNS服务器配置",
			Tags:        []string{"network", "dns"},
			Keywords:    []string{"DNS", "域名", "解析"},
		},
	}...)

	// 用户管理 API
	r.Register([]APIEndpoint{
		{
			ID:          "api.users.list",
			Method:      "GET",
			Path:        "/api/v1/users",
			Summary:     "获取用户列表",
			Description: "获取系统所有用户账户列表",
			Tags:        []string{"users", "accounts"},
			Keywords:    []string{"用户", "user", "账户", "列表"},
		},
		{
			ID:          "api.users.create",
			Method:      "POST",
			Path:        "/api/v1/users",
			Summary:     "创建用户",
			Description: "创建新的系统用户账户",
			Tags:        []string{"users", "accounts"},
			Keywords:    []string{"用户", "创建", "user", "create"},
		},
		{
			ID:          "api.users.update",
			Method:      "PUT",
			Path:        "/api/v1/users/{id}",
			Summary:     "更新用户信息",
			Description: "更新指定用户的信息和权限",
			Tags:        []string{"users", "accounts"},
			Keywords:    []string{"用户", "更新", "user", "权限"},
		},
		{
			ID:          "api.users.delete",
			Method:      "DELETE",
			Path:        "/api/v1/users/{id}",
			Summary:     "删除用户",
			Description: "删除指定的用户账户",
			Tags:        []string{"users", "accounts"},
			Keywords:    []string{"用户", "删除", "user", "delete"},
		},
	}...)

	// 容器 API
	r.Register([]APIEndpoint{
		{
			ID:          "api.containers.list",
			Method:      "GET",
			Path:        "/api/v1/containers",
			Summary:     "获取容器列表",
			Description: "获取所有Docker容器的列表和状态",
			Tags:        []string{"containers", "docker"},
			Keywords:    []string{"容器", "container", "Docker", "列表"},
		},
		{
			ID:          "api.containers.start",
			Method:      "POST",
			Path:        "/api/v1/containers/{id}/start",
			Summary:     "启动容器",
			Description: "启动指定的Docker容器",
			Tags:        []string{"containers", "docker"},
			Keywords:    []string{"容器", "启动", "container", "start", "Docker"},
		},
		{
			ID:          "api.containers.stop",
			Method:      "POST",
			Path:        "/api/v1/containers/{id}/stop",
			Summary:     "停止容器",
			Description: "停止指定的Docker容器",
			Tags:        []string{"containers", "docker"},
			Keywords:    []string{"容器", "停止", "container", "stop", "Docker"},
		},
		{
			ID:          "api.containers.logs",
			Method:      "GET",
			Path:        "/api/v1/containers/{id}/logs",
			Summary:     "获取容器日志",
			Description: "获取指定容器的运行日志",
			Tags:        []string{"containers", "docker", "logs"},
			Keywords:    []string{"容器", "日志", "container", "log", "Docker"},
		},
		{
			ID:          "api.images.list",
			Method:      "GET",
			Path:        "/api/v1/images",
			Summary:     "获取镜像列表",
			Description: "获取所有Docker镜像列表",
			Tags:        []string{"containers", "docker", "images"},
			Keywords:    []string{"镜像", "image", "Docker", "列表"},
		},
	}...)

	// 文件 API
	r.Register([]APIEndpoint{
		{
			ID:          "api.files.list",
			Method:      "GET",
			Path:        "/api/v1/files",
			Summary:     "列出文件",
			Description: "列出指定目录下的文件和子目录",
			Tags:        []string{"files", "filesystem"},
			Keywords:    []string{"文件", "列表", "file", "list", "目录"},
		},
		{
			ID:          "api.files.upload",
			Method:      "POST",
			Path:        "/api/v1/files/upload",
			Summary:     "上传文件",
			Description: "上传文件到指定目录",
			Tags:        []string{"files", "filesystem"},
			Keywords:    []string{"文件", "上传", "file", "upload"},
		},
		{
			ID:          "api.files.download",
			Method:      "GET",
			Path:        "/api/v1/files/download",
			Summary:     "下载文件",
			Description: "下载指定的文件",
			Tags:        []string{"files", "filesystem"},
			Keywords:    []string{"文件", "下载", "file", "download"},
		},
		{
			ID:          "api.files.delete",
			Method:      "DELETE",
			Path:        "/api/v1/files",
			Summary:     "删除文件",
			Description: "删除指定的文件或目录",
			Tags:        []string{"files", "filesystem"},
			Keywords:    []string{"文件", "删除", "file", "delete"},
		},
	}...)

	// 备份 API
	r.Register([]APIEndpoint{
		{
			ID:          "api.backup.tasks.list",
			Method:      "GET",
			Path:        "/api/v1/backup/tasks",
			Summary:     "获取备份任务列表",
			Description: "获取所有备份任务的列表和状态",
			Tags:        []string{"backup", "tasks"},
			Keywords:    []string{"备份", "任务", "backup", "task", "列表"},
		},
		{
			ID:          "api.backup.tasks.run",
			Method:      "POST",
			Path:        "/api/v1/backup/tasks/{id}/run",
			Summary:     "执行备份任务",
			Description: "立即执行指定的备份任务",
			Tags:        []string{"backup", "tasks"},
			Keywords:    []string{"备份", "执行", "backup", "run"},
		},
		{
			ID:          "api.cloudsync.tasks.list",
			Method:      "GET",
			Path:        "/api/v1/cloudsync/tasks",
			Summary:     "获取云同步任务列表",
			Description: "获取所有云同步任务的列表",
			Tags:        []string{"cloudsync", "backup"},
			Keywords:    []string{"云同步", "cloudsync", "云存储", "备份"},
		},
	}...)

	// 系统监控 API
	r.Register([]APIEndpoint{
		{
			ID:          "api.system.status",
			Method:      "GET",
			Path:        "/api/v1/system/status",
			Summary:     "获取系统状态",
			Description: "获取系统运行状态、CPU、内存、磁盘使用情况",
			Tags:        []string{"system", "monitoring"},
			Keywords:    []string{"系统", "状态", "system", "status", "监控"},
		},
		{
			ID:          "api.system.info",
			Method:      "GET",
			Path:        "/api/v1/system/info",
			Summary:     "获取系统信息",
			Description: "获取系统版本、硬件信息等",
			Tags:        []string{"system", "info"},
			Keywords:    []string{"系统", "信息", "system", "info", "版本"},
		},
		{
			ID:          "api.monitoring.metrics",
			Method:      "GET",
			Path:        "/api/v1/monitoring/metrics",
			Summary:     "获取监控指标",
			Description: "获取Prometheus格式的监控指标",
			Tags:        []string{"monitoring", "metrics"},
			Keywords:    []string{"监控", "指标", "monitoring", "metrics", "Prometheus"},
		},
		{
			ID:          "api.alerts.list",
			Method:      "GET",
			Path:        "/api/v1/alerts",
			Summary:     "获取告警列表",
			Description: "获取系统告警列表",
			Tags:        []string{"monitoring", "alerts"},
			Keywords:    []string{"告警", "alert", "通知", "列表"},
		},
	}...)

	// 安全 API
	r.Register([]APIEndpoint{
		{
			ID:          "api.auth.login",
			Method:      "POST",
			Path:        "/api/v1/auth/login",
			Summary:     "用户登录",
			Description: "用户登录认证，获取访问令牌",
			Tags:        []string{"auth", "security"},
			Keywords:    []string{"登录", "login", "认证", "auth"},
		},
		{
			ID:          "api.auth.logout",
			Method:      "POST",
			Path:        "/api/v1/auth/logout",
			Summary:     "用户登出",
			Description: "用户登出，注销访问令牌",
			Tags:        []string{"auth", "security"},
			Keywords:    []string{"登出", "logout", "认证"},
		},
		{
			ID:          "api.certificates.list",
			Method:      "GET",
			Path:        "/api/v1/certificates",
			Summary:     "获取证书列表",
			Description: "获取SSL/TLS证书列表",
			Tags:        []string{"security", "certificates"},
			Keywords:    []string{"证书", "certificate", "SSL", "TLS", "HTTPS"},
		},
		{
			ID:          "api.audit.logs",
			Method:      "GET",
			Path:        "/api/v1/audit/logs",
			Summary:     "获取审计日志",
			Description: "获取系统操作审计日志",
			Tags:        []string{"security", "audit"},
			Keywords:    []string{"审计", "日志", "audit", "log", "操作记录"},
		},
	}...)

	// 共享服务 API
	r.Register([]APIEndpoint{
		{
			ID:          "api.shares.smb.list",
			Method:      "GET",
			Path:        "/api/v1/shares/smb",
			Summary:     "获取SMB共享列表",
			Description: "获取所有SMB/CIFS共享配置",
			Tags:        []string{"shares", "smb"},
			Keywords:    []string{"共享", "SMB", "CIFS", "Windows"},
		},
		{
			ID:          "api.shares.nfs.list",
			Method:      "GET",
			Path:        "/api/v1/shares/nfs",
			Summary:     "获取NFS共享列表",
			Description: "获取所有NFS共享配置",
			Tags:        []string{"shares", "nfs"},
			Keywords:    []string{"共享", "NFS", "Linux", "exports"},
		},
	}...)

	// 全局搜索 API
	r.Register([]APIEndpoint{
		{
			ID:          "api.search.global",
			Method:      "POST",
			Path:        "/api/v1/search/global",
			Summary:     "全局搜索",
			Description: "执行全局搜索，支持文件、设置、应用、API等多种类型",
			Tags:        []string{"search"},
			Keywords:    []string{"搜索", "search", "全局", "global"},
		},
		{
			ID:          "api.search.quick",
			Method:      "GET",
			Path:        "/api/v1/search/quick",
			Summary:     "快速搜索",
			Description: "执行快速搜索，用于自动补全和建议",
			Tags:        []string{"search"},
			Keywords:    []string{"搜索", "search", "快速", "quick"},
		},
		{
			ID:          "api.search.suggestions",
			Method:      "GET",
			Path:        "/api/v1/search/suggestions",
			Summary:     "获取搜索建议",
			Description: "基于输入获取搜索建议和补全",
			Tags:        []string{"search"},
			Keywords:    []string{"搜索", "建议", "suggestion", "补全"},
		},
	}...)
}

// Register 注册API端点.
func (r *APIRegistry) Register(endpoints ...APIEndpoint) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.endpoints = append(r.endpoints, endpoints...)
}

// GetAll 获取所有API端点.
func (r *APIRegistry) GetAll() []APIEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]APIEndpoint, len(r.endpoints))
	copy(result, r.endpoints)
	return result
}

// APISearchResult API搜索结果.
type APISearchResult struct {
	Endpoint    APIEndpoint `json:"endpoint"`
	Score       float64     `json:"score"`
	MatchType   string      `json:"matchType"` // path, summary, tag, keyword
	MatchField  string      `json:"matchField"`
	MatchedText string      `json:"matchedText"`
}

// SearchAPIs 搜索API端点.
func (r *APIRegistry) SearchAPIs(query string, limit int) []APISearchResult {
	if query == "" {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	results := make([]APISearchResult, 0)

	for _, endpoint := range r.endpoints {
		score := 0.0
		matchType := ""
		matchField := ""
		matchedText := ""

		// 检查路径匹配
		if strings.Contains(strings.ToLower(endpoint.Path), query) {
			score = 1.0
			matchType = "path"
			matchField = "path"
			matchedText = endpoint.Path
		}

		// 检查摘要匹配
		if score == 0 && strings.Contains(strings.ToLower(endpoint.Summary), query) {
			score = 0.9
			matchType = "summary"
			matchField = "summary"
			matchedText = endpoint.Summary
		}

		// 检查描述匹配
		if score == 0 && strings.Contains(strings.ToLower(endpoint.Description), query) {
			score = 0.7
			matchType = "description"
			matchField = "description"
			matchedText = endpoint.Description
		}

		// 检查关键词匹配
		if score == 0 {
			for _, kw := range endpoint.Keywords {
				if strings.Contains(strings.ToLower(kw), query) {
					score = 0.8
					matchType = "keyword"
					matchField = "keyword"
					matchedText = kw
					break
				}
			}
		}

		// 检查标签匹配
		if score == 0 {
			for _, tag := range endpoint.Tags {
				if strings.Contains(strings.ToLower(tag), query) {
					score = 0.6
					matchType = "tag"
					matchField = "tag"
					matchedText = tag
					break
				}
			}
		}

		// 检查方法匹配
		if score == 0 && strings.ToLower(endpoint.Method) == query {
			score = 0.5
			matchType = "method"
			matchField = "method"
			matchedText = endpoint.Method
		}

		if score > 0 {
			results = append(results, APISearchResult{
				Endpoint:    endpoint,
				Score:       score,
				MatchType:   matchType,
				MatchField:  matchField,
				MatchedText: matchedText,
			})
		}
	}

	// 按分数排序
	sortAPIResults(results)

	// 限制数量
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// GetByMethod 按HTTP方法获取API端点.
func (r *APIRegistry) GetByMethod(method string) []APIEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []APIEndpoint
	for _, ep := range r.endpoints {
		if strings.EqualFold(ep.Method, method) {
			results = append(results, ep)
		}
	}
	return results
}

// GetByTag 按标签获取API端点.
func (r *APIRegistry) GetByTag(tag string) []APIEndpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []APIEndpoint
	tag = strings.ToLower(tag)
	for _, ep := range r.endpoints {
		for _, t := range ep.Tags {
			if strings.ToLower(t) == tag {
				results = append(results, ep)
				break
			}
		}
	}
	return results
}

// GetAPITags 获取所有API标签.
func (r *APIRegistry) GetAPITags() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tagMap := make(map[string]bool)
	for _, ep := range r.endpoints {
		for _, tag := range ep.Tags {
			tagMap[tag] = true
		}
	}

	tags := make([]string, 0, len(tagMap))
	for tag := range tagMap {
		tags = append(tags, tag)
	}
	return tags
}

// sortAPIResults 排序API搜索结果.
func sortAPIResults(results []APISearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}