// Package plugin 提供插件系统支持
//
// 支持动态加载 Go 插件（.so 文件），提供完整的插件生命周期管理。
// 插件可以实现扩展功能、主题定制、文件管理器增强等。
package plugin

import (
	"encoding/json"
	"time"
)

// Plugin 接口定义 - 所有插件必须实现
type Plugin interface {
	// Info 返回插件基本信息
	Info() PluginInfo

	// Init 初始化插件（加载配置、建立连接等）
	Init(config map[string]interface{}) error

	// Start 启动插件
	Start() error

	// Stop 停止插件
	Stop() error

	// Destroy 销毁插件（清理资源）
	Destroy() error
}

// PluginInfo 插件元信息
type PluginInfo struct {
	// 基本信息
	ID          string   `json:"id"`                    // 插件唯一标识（如 com.nas-os.filemanager-enhance）
	Name        string   `json:"name"`                  // 插件显示名称
	Version     string   `json:"version"`               // 版本号（语义化版本）
	Author      string   `json:"author"`                 // 作者
	Description string   `json:"description"`            // 描述
	Category    Category `json:"category"`               // 分类
	Tags        []string `json:"tags,omitempty"`         // 标签

	// 技术信息
	Entrypoint string `json:"entrypoint"`              // 入口函数名
	MainFile   string `json:"mainFile"`                // 主文件路径

	// 依赖信息
	Dependencies []Dependency `json:"dependencies,omitempty"`
	NASVersion   string       `json:"nasVersion,omitempty"`   // 兼容的 NAS-OS 版本
	GoVersion    string       `json:"goVersion,omitempty"`    // 编译的 Go 版本

	// 权限声明
	Permissions []Permission `json:"permissions,omitempty"`

	// 扩展点
	ExtensionPoints []string `json:"extensionPoints,omitempty"`

	// 配置
	ConfigSchema *ConfigSchema `json:"configSchema,omitempty"`

	// UI 相关
	Icon        string `json:"icon,omitempty"`         // 图标（SVG/PNG base64）
	Screenshots []string `json:"screenshots,omitempty"` // 截图

	// 市场信息
	Homepage    string `json:"homepage,omitempty"`
	Repository  string `json:"repository,omitempty"`
	License     string `json:"license,omitempty"`
	Price       string `json:"price,omitempty"`        // "free" 或价格
}

// Category 插件分类
type Category string

const (
	CategoryStorage       Category = "storage"        // 存储管理
	CategoryFileManager   Category = "file-manager"   // 文件管理
	CategoryNetwork       Category = "network"         // 网络工具
	CategorySystem        Category = "system"         // 系统工具
	CategorySecurity      Category = "security"       // 安全工具
	CategoryMedia         Category = "media"          // 多媒体
	CategoryBackup        Category = "backup"         // 备份同步
	CategoryTheme         Category = "theme"          // 主题外观
	CategoryIntegration   Category = "integration"    // 第三方集成
	CategoryDeveloper     Category = "developer"      // 开发工具
	CategoryProductivity  Category = "productivity"    // 生产力
	CategoryOther         Category = "other"          // 其他
)

// Dependency 插件依赖
type Dependency struct {
	ID      string `json:"id"`      // 依赖的插件 ID
	Version string `json:"version"` // 版本要求（如 ">=1.0.0"）
	Optional bool   `json:"optional,omitempty"`
}

// Permission 权限声明
type Permission struct {
	Name        string `json:"name"`                  // 权限名称
	Description string `json:"description,omitempty"` // 权限描述
}

// ConfigSchema 配置模式定义
type ConfigSchema struct {
	Properties map[string]Property `json:"properties"`
	Required   []string             `json:"required,omitempty"`
}

// Property 配置属性
type Property struct {
	Type        string      `json:"type"`                  // 类型：string, number, boolean, array, object
	Title       string      `json:"title,omitempty"`       // 显示名称
	Description string      `json:"description,omitempty"` // 描述
	Default     interface{} `json:"default,omitempty"`     // 默认值
	Enum        []string    `json:"enum,omitempty"`        // 枚举值
	Minimum     *float64    `json:"minimum,omitempty"`     // 最小值
	Maximum     *float64    `json:"maximum,omitempty"`     // 最大值
	MinLength   *int        `json:"minLength,omitempty"`  // 最小长度
	MaxLength   *int        `json:"maxLength,omitempty"`  // 最大长度
}

// PluginState 插件状态
type PluginState struct {
	ID          string      `json:"id"`
	Enabled     bool        `json:"enabled"`
	Running     bool        `json:"running"`
	Installed   bool        `json:"installed"`
	Version     string      `json:"version"`
	InstalledAt time.Time   `json:"installedAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	Config      json.RawMessage `json:"config,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// PluginInstance 运行时插件实例
type PluginInstance struct {
	Info    PluginInfo
	Plugin  Plugin
	State   PluginState
	Path    string            // 插件 .so 文件路径
	Enabled bool              // 是否启用
	Running bool              // 是否运行中
}

// ExtensionPoint 扩展点定义
type ExtensionPoint struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Extensions  []*Extension    `json:"extensions"`
}

// Extension 扩展实现
type Extension struct {
	PluginID string      `json:"pluginId"`
	PointID  string      `json:"pointId"`
	Priority int         `json:"priority"`
	Config   interface{} `json:"config"`
}

// Hook 钩子函数类型
type Hook func(ctx HookContext) error

// HookContext 钩子上下文
type HookContext struct {
	Event     string
	Data      interface{}
	PluginID  string
	Timestamp time.Time
}

// HookType 钩子类型
type HookType string

const (
	HookBeforeMount    HookType = "beforeMount"
	HookAfterMount     HookType = "afterMount"
	HookBeforeUnmount  HookType = "beforeUnmount"
	HookAfterUnmount   HookType = "afterUnmount"
	HookBeforeCreate   HookType = "beforeCreate"
	HookAfterCreate    HookType = "afterCreate"
	HookBeforeDelete   HookType = "beforeDelete"
	HookAfterDelete    HookType = "afterDelete"
	HookBeforeStart    HookType = "beforeStart"
	HookAfterStart     HookType = "afterStart"
	HookBeforeStop     HookType = "beforeStop"
	HookAfterStop      HookType = "afterStop"
)