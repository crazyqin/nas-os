// Package app 应用类型定义
// 定义应用中心使用的核心数据类型
package app

import (
	"fmt"
	"time"
)

// ========== 应用模板类型 ==========

// Template 应用模板 - 定义应用的基础配置
type Template struct {
	ID          string            `json:"id"`                    // 模板唯一标识
	Name        string            `json:"name"`                  // 应用名称
	DisplayName string            `json:"displayName"`           // 显示名称
	Description string            `json:"description"`           // 应用描述
	Category    string            `json:"category"`              // 分类
	Icon        string            `json:"icon"`                  // 图标（emoji或URL）
	Version     string            `json:"version"`               // 模板版本
	Author      string            `json:"author"`                // 作者
	Website     string            `json:"website"`               // 官网链接
	Source      string            `json:"source"`                // 源码链接
	License     string            `json:"license"`               // 许可协议
	Containers  []ContainerSpec   `json:"containers"`            // 容器规格列表
	Notes       string            `json:"notes"`                 // 使用说明
	Tags        []string          `json:"tags"`                  // 标签
	Rating      float64           `json:"rating"`                // 评分
	Downloads   int64             `json:"downloads"`             // 下载次数
}

// ContainerSpec 容器规格 - 定义单个容器的配置
type ContainerSpec struct {
	Name          string            `json:"name"`                  // 容器名称
	Image         string            `json:"image"`                 // 镜像名称
	Hostname      string            `json:"hostname"`              // 主机名
	Ports         []PortSpec        `json:"ports"`                 // 端口配置
	Volumes       []VolumeSpec      `json:"volumes"`               // 卷配置
	Environment   map[string]string `json:"environment"`           // 环境变量
	Command       []string          `json:"command"`               // 启动命令
	Privileged    bool              `json:"privileged"`            // 特权模式
	NetworkMode   string            `json:"networkMode"`           // 网络模式
	RestartPolicy string            `json:"restartPolicy"`         // 重启策略
	HealthCheck   *HealthCheckSpec  `json:"healthCheck"`           // 健康检查
	ComposeTemplate string          `json:"composeTemplate"`       // 自定义Compose模板（可选）
}

// PortSpec 端口规格
type PortSpec struct {
	Name           string `json:"name"`           // 端口名称
	ContainerPort  int    `json:"containerPort"`  // 容器端口
	Protocol       string `json:"protocol"`       // 协议(tcp/udp)
	DefaultHostPort int   `json:"defaultHostPort"`// 默认主机端口
	Description    string `json:"description"`    // 端口说明
	Required       bool   `json:"required"`       // 是否必须映射
}

// VolumeSpec 卷规格
type VolumeSpec struct {
	Name           string `json:"name"`           // 卷名称
	ContainerPath  string `json:"containerPath"`  // 容器路径
	DefaultHostPath string `json:"defaultHostPath"`// 默认主机路径
	Description    string `json:"description"`    // 卷说明
	ReadOnly       bool   `json:"readOnly"`       // 只读模式
	Required       bool   `json:"required"`       // 是否必须挂载
}

// HealthCheckSpec 健康检查规格
type HealthCheckSpec struct {
	Test        []string `json:"test"`        // 检查命令
	Interval    int      `json:"interval"`    // 检查间隔(秒)
	Timeout     int      `json:"timeout"`     // 超时时间(秒)
	StartPeriod int      `json:"startPeriod"` // 启动等待(秒)
	Retries     int      `json:"retries"`     // 重试次数
}

// Validate 验证模板
func (t *Template) Validate() error {
	if t.ID == "" {
		return fmt.Errorf("模板ID不能为空")
	}
	if t.Name == "" {
		return fmt.Errorf("应用名称不能为空")
	}
	if len(t.Containers) == 0 {
		return fmt.Errorf("至少需要定义一个容器")
	}
	for _, c := range t.Containers {
		if c.Image == "" {
			return fmt.Errorf("容器 %s 镜像不能为空", c.Name)
		}
	}
	return nil
}

// ========== 应用分类常量 ==========

const (
	CategoryMedia       = "Media"        // 媒体
	CategoryProductivity = "Productivity" // 生产力
	CategorySmartHome   = "Smart Home"   // 智能家居
	CategoryDownload    = "Download"     // 下载
	CategoryNetwork     = "Network"      // 网络
	CategoryDatabase    = "Database"     // 数据库
	CategoryAI          = "AI"           // AI
	CategoryDevelopment = "Development"  // 开发
	CategorySecurity    = "Security"     // 安全
	CategoryMonitoring  = "Monitoring"   // 监控
	CategoryOther       = "Other"        // 其他
)

// ========== 安装配置类型 ==========

// InstallOptions 安装选项 - 用户安装应用时的自定义配置
type InstallOptions struct {
	InstanceName  string            `json:"instanceName"`  // 实例名称（多实例安装）
	PortMappings  map[string]int    `json:"portMappings"`  // 端口映射（按名称）
	VolumePaths   map[string]string `json:"volumePaths"`   // 卷路径（按名称）
	Env           map[string]string `json:"env"`           // 环境变量覆盖
	Network       string            `json:"network"`       // 自定义网络
	CPULimit      string            `json:"cpuLimit"`      // CPU限制
	MemoryLimit   string            `json:"memoryLimit"`   // 内存限制
	SkipStart     bool              `json:"skipStart"`     // 安装后不启动
}

// UninstallOptions 卸载选项
type UninstallOptions struct {
	Force         bool `json:"force"`         // 强制卸载（忽略错误）
	RemoveVolumes bool `json:"removeVolumes"` // 删除数据卷
	RemoveConfig  bool `json:"removeConfig"`  // 删除配置文件
}

// InstallConfig 安装配置记录（保存到文件）
type InstallConfig struct {
	TemplateID  string          `json:"templateId"`
	Options     *InstallOptions `json:"options"`
	InstalledAt time.Time       `json:"installedAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// ========== 已安装应用类型 ==========

// InstalledApp 已安装应用记录
type InstalledApp struct {
	ID              string           `json:"id"`              // 应用ID
	Name            string           `json:"name"`            // 应用名称
	DisplayName     string           `json:"displayName"`     // 显示名称
	TemplateID      string           `json:"templateId"`      // 模板ID
	Version         string           `json:"version"`         // 安装版本
	Status          string           `json:"status"`          // 当前状态
	InstalledAt     time.Time        `json:"installedAt"`     // 安装时间
	UpdatedAt       time.Time        `json:"updatedAt"`       // 更新时间
	ComposePath     string           `json:"composePath"`     // Compose文件路径
	ConfigPath      string           `json:"configPath"`      // 配置文件路径
	Config          map[string]string `json:"config"`         // 当前配置
	PortMappings    []PortMapping    `json:"portMappings"`    // 端口映射
	VolumeMappings  []VolumeMapping  `json:"volumeMappings"`  // 卷映射
	Services        []ComposeService `json:"services"`        // 服务列表
}

// PortMapping 端口映射记录
type PortMapping struct {
	Name          string `json:"name"`          // 端口名称
	HostPort      int    `json:"hostPort"`      // 主机端口
	ContainerPort int    `json:"containerPort"` // 容器端口
	Protocol      string `json:"protocol"`      // 协议
	Description   string `json:"description"`   // 说明
}

// VolumeMapping 卷映射记录
type VolumeMapping struct {
	Name          string `json:"name"`          // 卷名称
	HostPath      string `json:"hostPath"`      // 主机路径
	ContainerPath string `json:"containerPath"` // 容器路径
	Description   string `json:"description"`   // 说明
	ReadOnly      bool   `json:"readOnly"`      // 只读
}

// ========== 应用状态类型 ==========

// AppState 应用状态常量
const (
	AppStateRunning  = "running"  // 运行中
	AppStateStopped  = "stopped"  // 已停止
	AppStateStarting = "starting" // 启动中
	AppStateStopping = "stopping" // 停止中
	AppStateError    = "error"    // 错误
	AppStateUnknown  = "unknown"  // 未知
)

// AppStatus 应用状态
type AppStatus struct {
	State      string           `json:"state"`      // 状态
	Message    string           `json:"message"`    // 状态消息
	Services   []ComposeService `json:"services"`   // 服务列表
	UpdatedAt  time.Time        `json:"updatedAt"`  // 更新时间
}

// AppStatusRunning 运行状态常量
const (
	AppStatusRunning   = "running"
	AppStatusStopped   = "stopped"
	AppStatusPartial   = "partial"  // 部分运行
	AppStatusError     = "error"
)

// ComposeService Compose 服务状态
type ComposeService struct {
	Name    string `json:"name"`    // 服务名称
	State   string `json:"state"`   // 状态
	Status  string `json:"status"`  // 状态描述
	Image   string `json:"image"`   // 镜像
	Ports   string `json:"ports"`   // 端口信息
	Health  string `json:"health"`  // 健康状态
	Running bool   `json:"running"` // 是否运行
}

// ========== 容器操作类型 ==========

// ContainerConfig 容器创建配置
type ContainerConfig struct {
	Name        string            `json:"name"`        // 容器名称
	Image       string            `json:"image"`       // 镜像
	Command     []string          `json:"command"`     // 启动命令
	Ports       []string          `json:"ports"`       // 端口映射
	Volumes     []string          `json:"volumes"`     // 卷挂载
	Environment map[string]string `json:"environment"` // 环境变量
	Network     string            `json:"network"`     // 网络
	Restart     string            `json:"restart"`     // 重启策略
	CPULimit    string            `json:"cpuLimit"`    // CPU限制
	MemLimit    string            `json:"memLimit"`    // 内存限制
	Labels      map[string]string `json:"labels"`      // 标签
	Privileged  bool              `json:"privileged"`  // 特权模式
}

// ContainerStatus 容器状态
type ContainerStatus struct {
	ID        string            `json:"id"`        // 容器ID
	Name      string            `json:"name"`      // 容器名称
	State     string            `json:"state"`     // 状态
	Status    string            `json:"status"`    // 状态描述
	Image     string            `json:"image"`     // 镜像
	Running   bool              `json:"running"`   // 是否运行
	CPUUsage  float64           `json:"cpuUsage"`  // CPU使用率
	MemUsage  uint64            `json:"memUsage"`  // 内存使用
	MemLimit  uint64            `json:"memLimit"`  // 内存限制
	NetRx     uint64            `json:"netRx"`     // 网络接收
	NetTx     uint64            `json:"netTx"`     // 网络发送
	Ports     []string          `json:"ports"`     // 端口列表
	Volumes   []string          `json:"volumes"`   // 卷列表
	Labels    map[string]string `json:"labels"`    // 标签
	Created   time.Time         `json:"created"`   // 创建时间
}

// ========== 仓库类型 ==========

// RepositorySource 仓库源
type RepositorySource struct {
	Name      string `json:"name"`      // 仓库名称
	URL       string `json:"url"`       // 仓库URL
	Type      string `json:"type"`      // 类型(local/remote)
	Enabled   bool   `json:"enabled"`   // 是否启用
	Priority  int    `json:"priority"`  // 优先级
	UpdatedAt string `json:"updatedAt"` // 更新时间
}

// RepositoryConfig 仓库配置
type RepositoryConfig struct {
	Sources []RepositorySource `json:"sources"` // 仓库源列表
}