// Package composeinclude 提供 Docker Compose Include 支持，
// 允许自定义应用 Compose 文件引用外部 Compose 文件并合并服务定义。
// 对标 TrueNAS 26 的 Docker Compose include 功能。
package composeinclude

import "time"

// IncludeDirective Compose 文件中的 include 指令
type IncludeDirective struct {
	// 引用的外部 Compose 文件路径列表
	Paths []string `json:"paths"`
}

// ComposeFile Compose 文件结构（简化）
type ComposeFile struct {
	// 服务定义
	Services map[string]ServiceDefinition `json:"services"`
	// include 指令
	Include []IncludeDirective `json:"include,omitempty"`
	// 卷定义
	Volumes map[string]interface{} `json:"volumes,omitempty"`
	// 网络定义
	Networks map[string]interface{} `json:"networks,omitempty"`
}

// ServiceDefinition 服务定义
type ServiceDefinition struct {
	// 镜像
	Image string `json:"image,omitempty"`
	// 构建上下文
	Build string `json:"build,omitempty"`
	// 端口映射
	Ports []string `json:"ports,omitempty"`
	// 环境变量
	Environment map[string]string `json:"environment,omitempty"`
	// 挂载卷
	Volumes []string `json:"volumes,omitempty"`
	// 依赖服务
	DependsOn []string `json:"depends_on,omitempty"`
	// 重启策略
	Restart string `json:"restart,omitempty"`
	// 网络配置
	Networks []string `json:"networks,omitempty"`
}

// ParseResult 解析结果
type ParseResult struct {
	// 解析结果唯一标识
	ID string `json:"id"`
	// 主 Compose 文件路径
	SourceFile string `json:"source_file"`
	// 合并后的所有服务
	MergedServices map[string]ServiceDefinition `json:"merged_services"`
	// 引用的外部文件列表
	IncludePaths []string `json:"include_paths"`
	// 缺失的引用文件列表
	MissingFiles []string `json:"missing_files,omitempty"`
	// 是否所有引用文件存在
	AllFilesExist bool `json:"all_files_exist"`
	// 服务总数
	ServiceCount int `json:"service_count"`
	// 解析时间
	ParsedAt time.Time `json:"parsed_at"`
}

// ParseRequest 解析请求
type ParseRequest struct {
	// Compose 文件内容
	Content string `json:"content"`
	// Compose 文件所在目录（用于解析相对路径）
	BaseDir string `json:"base_dir,omitempty"`
}