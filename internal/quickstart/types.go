// Package quickstart 社区应用快速部署模板
// 提供一键部署热门 Docker Compose 应用的能力
package quickstart

import (
	"fmt"
	"time"
)

// ========== 分类定义 ==========

// Category 模板分类
type Category string

const (
	CategoryMedia     Category = "media"     // 媒体中心
	CategoryDownload  Category = "download"  // 下载工具
	CategoryCloud     Category = "cloud"     // 网盘同步
	CategoryPhoto     Category = "photo"     // 照片管理
	CategoryDevTools  Category = "devtools"  // 开发工具
	CategoryAIML      Category = "aiml"      // AI/ML
	CategoryDatabase  Category = "database"  // 数据库
	CategoryMonitor   Category = "monitor"   // 监控
)

// CategoryInfo 分类信息
type CategoryInfo struct {
	ID          Category `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Icon        string   `json:"icon"`
}

// DefaultCategories 预置分类列表
var DefaultCategories = []CategoryInfo{
	{ID: CategoryMedia, Name: "媒体中心", Description: "视频/音乐串流服务", Icon: "🎬"},
	{ID: CategoryDownload, Name: "下载工具", Description: "BT/HTTP 下载管理", Icon: "⬇️"},
	{ID: CategoryCloud, Name: "网盘同步", Description: "私有云盘与文件同步", Icon: "☁️"},
	{ID: CategoryPhoto, Name: "照片管理", Description: "照片存储与AI整理", Icon: "📷"},
	{ID: CategoryDevTools, Name: "开发工具", Description: "Git、CI/CD、容器管理", Icon: "🛠️"},
	{ID: CategoryAIML, Name: "AI/ML", Description: "大模型推理与图像生成", Icon: "🤖"},
	{ID: CategoryDatabase, Name: "数据库", Description: "关系型与缓存数据库", Icon: "🗄️"},
	{ID: CategoryMonitor, Name: "监控", Description: "指标采集与可视化", Icon: "📊"},
}

// ========== 参数定义 ==========

// ParamType 参数类型
type ParamType string

const (
	ParamTypeString  ParamType = "string"
	ParamTypeInt     ParamType = "int"
	ParamTypeBool    ParamType = "bool"
	ParamTypePath    ParamType = "path"
	ParamTypePort    ParamType = "port"
	ParamTypeSelect  ParamType = "select"
)

// TemplateParam 模板可配置参数
type TemplateParam struct {
	Key          string    `json:"key"`
	Label        string    `json:"label"`
	Description  string    `json:"description,omitempty"`
	Type         ParamType `json:"type"`
	DefaultValue string    `json:"default_value"`
	Required     bool      `json:"required"`
	Options      []string  `json:"options,omitempty"`   // 用于 select 类型
	Min          *int      `json:"min,omitempty"`       // 用于 int/port 类型最小值
	Max          *int      `json:"max,omitempty"`       // 用于 int/port 类型最大值
	Placeholder  string    `json:"placeholder,omitempty"`
}

// Validate 校验参数值
func (p *TemplateParam) Validate(value string) error {
	if p.Required && value == "" {
		return fmt.Errorf("参数 %s (%s) 为必填项", p.Key, p.Label)
	}
	if value == "" {
		return nil // 非必填且为空，跳过
	}
	switch p.Type {
	case ParamTypePort, ParamTypeInt:
		var n int
		if _, err := fmt.Sscanf(value, "%d", &n); err != nil {
			return fmt.Errorf("参数 %s 需要整数值", p.Key)
		}
		if p.Min != nil && n < *p.Min {
			return fmt.Errorf("参数 %s 最小值为 %d", p.Key, *p.Min)
		}
		if p.Max != nil && n > *p.Max {
			return fmt.Errorf("参数 %s 最大值为 %d", p.Key, *p.Max)
		}
	case ParamTypeBool:
		if value != "true" && value != "false" {
			return fmt.Errorf("参数 %s 需要 true 或 false", p.Key)
		}
	case ParamTypeSelect:
		found := false
		for _, opt := range p.Options {
			if value == opt {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("参数 %s 的值不在可选范围内", p.Key)
		}
	}
	return nil
}

// ========== 模板定义 ==========

// Template 一键部署模板
type Template struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Category     Category        `json:"category"`
	Tags         []string        `json:"tags"`
	Icon         string          `json:"icon,omitempty"`
	Version      string          `json:"version"`
	Author       string          `json:"author,omitempty"`
	Homepage     string          `json:"homepage,omitempty"`
	ComposeYAML  string          `json:"compose_yaml"`
	Params       []TemplateParam `json:"params"`
	Requirements Requirements    `json:"requirements"`
	MinVersion   string          `json:"min_version,omitempty"` // 最低 NAS-OS 版本
	UpdatedAt    time.Time       `json:"updated_at"`
}

// GetParam 获取参数定义
func (t *Template) GetParam(key string) *TemplateParam {
	for i := range t.Params {
		if t.Params[i].Key == key {
			return &t.Params[i]
		}
	}
	return nil
}

// ValidateParams 校验所有参数值
func (t *Template) ValidateParams(params map[string]string) error {
	for _, p := range t.Params {
		val := params[p.Key]
		if val == "" {
			val = p.DefaultValue
		}
		if err := p.Validate(val); err != nil {
			return err
		}
	}
	return nil
}

// ========== 环境需求 ==========

// Requirements 部署环境需求
type Requirements struct {
	MinDiskGB   int    `json:"min_disk_gb,omitempty"`   // 最小磁盘空间 GB
	MinMemoryMB int    `json:"min_memory_mb,omitempty"` // 最小内存 MB
	MinCPU      int    `json:"min_cpu,omitempty"`       // 最小CPU核心数
	GPU         bool   `json:"gpu,omitempty"`           // 是否需要GPU
	DockerMin   string `json:"docker_min,omitempty"`    // 最低Docker版本
	Ports       []int  `json:"ports,omitempty"`         // 需要的端口
}

// ========== 部署定义 ==========

// DeployStatus 部署状态
type DeployStatus string

const (
	DeployStatusDeploying DeployStatus = "deploying"  // 部署中
	DeployStatusRunning   DeployStatus = "running"    // 运行中
	DeployStatusStopped   DeployStatus = "stopped"    // 已停止
	DeployStatusFailed    DeployStatus = "failed"     // 失败
	DeployStatusStarting  DeployStatus = "starting"   // 启动中
	DeployStatusStopping  DeployStatus = "stopping"   // 停止中
)

// Deployment 部署实例
type Deployment struct {
	ID          string                 `json:"id"`
	TemplateID  string                 `json:"template_id"`
	Name        string                 `json:"name"`
	Status      DeployStatus           `json:"status"`
	Params      map[string]string      `json:"params"`
	ComposeYAML string                 `json:"compose_yaml"` // 渲染后的 YAML
	ComposeDir  string                 `json:"compose_dir"`
	Error       string                 `json:"error,omitempty"`
	Services    []ServiceInfo          `json:"services,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	StoppedAt   *time.Time             `json:"stopped_at,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ServiceInfo 容器服务信息
type ServiceInfo struct {
	Name        string `json:"name"`
	ContainerID string `json:"container_id,omitempty"`
	Status      string `json:"status,omitempty"`
	Image       string `json:"image,omitempty"`
	Ports       string `json:"ports,omitempty"`
}

// ========== API 请求/响应 ==========

// ListTemplatesRequest 列表查询请求
type ListTemplatesRequest struct {
	Category string   `form:"category"`
	Tags     []string `form:"tags"`
	Search   string   `form:"search"`
}

// DeployRequest 部署请求
type DeployRequest struct {
	Name   string            `json:"name" binding:"required"`
	Params map[string]string `json:"params"`
}

// APIResponse 通用 API 响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ListResponse 列表响应
type ListResponse struct {
	Items interface{} `json:"items"`
	Total int         `json:"total"`
}
