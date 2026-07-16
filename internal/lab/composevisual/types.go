// Package composevisual 提供 Docker Compose 可视化编排功能
package composevisual

import (
	"sync"
	"time"
)

// Manager 管理所有 Compose 项目.
type Manager struct {
	mu        sync.RWMutex
	projects  map[string]*ComposeProject
	templates map[string]*ComposeTemplate
}

// ComposeProject Compose 项目.
type ComposeProject struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Services    map[string]*ServiceNode   `json:"services"`
	Networks    map[string]*NetworkConfig `json:"networks"`
	Volumes     map[string]*VolumeConfig  `json:"volumes"`
	EnvVars     map[string]string         `json:"envVars"`
	Layout      *VisualLayout             `json:"layout"`
	Status      ProjectStatus             `json:"status"`
	CreatedAt   time.Time                 `json:"createdAt"`
	UpdatedAt   time.Time                 `json:"updatedAt"`
	DeployedAt  *time.Time                `json:"deployedAt,omitempty"`
	ComposePath string                    `json:"composePath,omitempty"`
	Tags        []string                  `json:"tags"`
}

// ProjectStatus 项目状态.
type ProjectStatus string

const (
	StatusDraft    ProjectStatus = "draft"
	StatusReady    ProjectStatus = "ready"
	StatusDeployed ProjectStatus = "deployed"
	StatusError    ProjectStatus = "error"
)

// ServiceNode 服务节点.
type ServiceNode struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	ContainerName string            `json:"containerName"`
	Image         string            `json:"image"`
	Ports         []PortMapping     `json:"ports"`
	Volumes       []VolumeMapping   `json:"volumes"`
	Environment   map[string]string `json:"environment"`
	DependsOn     []string          `json:"dependsOn"`
	Resources     *ResourceLimits   `json:"resources,omitempty"`
	HealthCheck   *HealthCheck      `json:"healthCheck,omitempty"`
	Restart       string            `json:"restart"`
	Command       []string          `json:"command,omitempty"`
	EntryPoint    []string          `json:"entrypoint,omitempty"`
	WorkingDir    string            `json:"workingDir,omitempty"`
	Networks      []string          `json:"networks,omitempty"`
	Labels        map[string]string `json:"labels,omitempty"`
	Position      *NodePosition     `json:"position"`
	Status        ServiceStatus     `json:"status"`
}

// ServiceStatus 服务状态.
type ServiceStatus string

const (
	ServiceDraft   ServiceStatus = "draft"
	ServiceRunning ServiceStatus = "running"
	ServiceStopped ServiceStatus = "stopped"
	ServiceError   ServiceStatus = "error"
)

// PortMapping 端口映射.
type PortMapping struct {
	HostPort      int    `json:"hostPort"`
	ContainerPort int    `json:"containerPort"`
	Protocol      string `json:"protocol"`
	IP            string `json:"ip,omitempty"`
}

// VolumeMapping 卷映射.
type VolumeMapping struct {
	Type     string `json:"type"`
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"readOnly"`
}

// ResourceLimits 资源限制.
type ResourceLimits struct {
	CPUs         string               `json:"cpus,omitempty"`
	Memory       string               `json:"memory,omitempty"`
	MemorySwap   string               `json:"memorySwap,omitempty"`
	Reservations *ResourceReservation `json:"reservations,omitempty"`
}

// ResourceReservation 资源预留.
type ResourceReservation struct {
	CPUs   string `json:"cpus,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// HealthCheck 健康检查.
type HealthCheck struct {
	Test        []string `json:"test"`
	Interval    string   `json:"interval"`
	Timeout     string   `json:"timeout"`
	Retries     int      `json:"retries"`
	StartPeriod string   `json:"startPeriod"`
}

// NetworkConfig 网络配置.
type NetworkConfig struct {
	Driver     string            `json:"driver"`
	DriverOpts map[string]string `json:"driverOpts,omitempty"`
	IPAM       *IPAMConfig       `json:"ipam,omitempty"`
	External   bool              `json:"external"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// IPAMConfig IPAM 配置.
type IPAMConfig struct {
	Driver string     `json:"driver"`
	Config []IPAMPool `json:"config,omitempty"`
}

// IPAMPool IPAM 地址池.
type IPAMPool struct {
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway,omitempty"`
	IPRange string `json:"ipRange,omitempty"`
}

// VolumeConfig 卷配置.
type VolumeConfig struct {
	Driver     string            `json:"driver"`
	DriverOpts map[string]string `json:"driverOpts,omitempty"`
	External   bool              `json:"external"`
	Labels     map[string]string `json:"labels,omitempty"`
}

// VisualLayout 可视化布局.
type VisualLayout struct {
	CanvasWidth  int                      `json:"canvasWidth"`
	CanvasHeight int                      `json:"canvasHeight"`
	Nodes        map[string]*NodePosition `json:"nodes"`
	Connections  []Connection             `json:"connections"`
}

// NodePosition 节点位置.
type NodePosition struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Connection 连接关系.
type Connection struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Type     string `json:"type"`
	Label    string `json:"label,omitempty"`
	Animated bool   `json:"animated"`
}

// TopologyData 拓扑图数据.
type TopologyData struct {
	Nodes  []TopologyNode  `json:"nodes"`
	Edges  []TopologyEdge  `json:"edges"`
	Groups []TopologyGroup `json:"groups"`
	Layers []string        `json:"layers"`
}

// TopologyNode 拓扑节点.
type TopologyNode struct {
	ID       string        `json:"id"`
	Label    string        `json:"label"`
	Type     string        `json:"type"`
	Image    string        `json:"image"`
	Status   ServiceStatus `json:"status"`
	Position NodePosition  `json:"position"`
	Ports    []PortMapping `json:"ports"`
	Tier     int           `json:"tier"`
}

// TopologyEdge 拓扑边.
type TopologyEdge struct {
	ID     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"`
	Label  string `json:"label,omitempty"`
}

// TopologyGroup 拓扑分组.
type TopologyGroup struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	NodeIDs []string `json:"nodeIds"`
}

// ComposeTemplate Compose 模板.
type ComposeTemplate struct {
	ID          string                    `json:"id"`
	Name        string                    `json:"name"`
	Description string                    `json:"description"`
	Category    string                    `json:"category"`
	Icon        string                    `json:"icon"`
	Tags        []string                  `json:"tags"`
	Rating      float64                   `json:"rating"`
	Downloads   int                       `json:"downloads"`
	Author      string                    `json:"author"`
	Version     string                    `json:"version"`
	Services    map[string]*ServiceNode   `json:"services"`
	Networks    map[string]*NetworkConfig `json:"networks"`
	Volumes     map[string]*VolumeConfig  `json:"volumes"`
	EnvVars     map[string]string         `json:"envVars"`
	EnvExample  map[string]string         `json:"envExample"`
	Readme      string                    `json:"readme"`
	CreatedAt   time.Time                 `json:"createdAt"`
}

// TemplateCategory 模板分类.
type TemplateCategory string

const (
	CategoryWeb       TemplateCategory = "web"
	CategoryDatabase  TemplateCategory = "database"
	CategoryMedia     TemplateCategory = "media"
	CategoryDevOps    TemplateCategory = "devops"
	CategorySmartHome TemplateCategory = "smart_home"
	CategoryStorage   TemplateCategory = "storage"
	CategoryNetwork   TemplateCategory = "network"
	CategoryAI        TemplateCategory = "ai"
	CategoryCMS       TemplateCategory = "cms"
	CategoryAll       TemplateCategory = "all"
)

// ========== 请求/响应结构体 ==========

// CreateProjectRequest 创建项目请求.
type CreateProjectRequest struct {
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	EnvVars     map[string]string `json:"envVars"`
	Tags        []string          `json:"tags"`
}

// UpdateProjectRequest 更新项目请求.
type UpdateProjectRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	EnvVars     map[string]string `json:"envVars"`
	Tags        []string          `json:"tags"`
}

// AddServiceRequest 添加服务请求.
type AddServiceRequest struct {
	Name          string            `json:"name" binding:"required"`
	Image         string            `json:"image" binding:"required"`
	ContainerName string            `json:"containerName"`
	Ports         []PortMapping     `json:"ports"`
	Volumes       []VolumeMapping   `json:"volumes"`
	Environment   map[string]string `json:"environment"`
	DependsOn     []string          `json:"dependsOn"`
	Command       []string          `json:"command"`
	EntryPoint    []string          `json:"entrypoint"`
	WorkingDir    string            `json:"workingDir"`
	Restart       string            `json:"restart"`
}

// UpdateServiceRequest 更新服务请求.
type UpdateServiceRequest struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	ContainerName string            `json:"containerName"`
	Ports         []PortMapping     `json:"ports"`
	Volumes       []VolumeMapping   `json:"volumes"`
	Environment   map[string]string `json:"environment"`
	DependsOn     []string          `json:"dependsOn"`
	Command       []string          `json:"command"`
	EntryPoint    []string          `json:"entrypoint"`
	WorkingDir    string            `json:"workingDir"`
	Restart       string            `json:"restart"`
	Resources     *ResourceLimits   `json:"resources"`
	HealthCheck   *HealthCheck      `json:"healthCheck"`
}

// ConnectServicesRequest 连接服务请求.
type ConnectServicesRequest struct {
	From string `json:"from" binding:"required"`
	To   string `json:"to" binding:"required"`
	Type string `json:"type"`
}

// ImportComposeRequest 导入 Compose 请求.
type ImportComposeRequest struct {
	Content string `json:"content" binding:"required"`
	Name    string `json:"name"`
}

// InstantiateTemplateRequest 从模板创建请求.
type InstantiateTemplateRequest struct {
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	EnvVars     map[string]string `json:"envVars"`
}

// ExportComposeResponse 导出 Compose 响应.
type ExportComposeResponse struct {
	Content string `json:"content"`
}

// TopologyResponse 拓扑图响应.
type TopologyResponse struct {
	Topology    *TopologyData `json:"topology"`
	StartOrder  [][]string    `json:"startOrder"`
	TotalMemory string        `json:"totalMemory"`
	TotalCPU    string        `json:"totalCPU"`
}

// DeployResult 部署结果.
type DeployResult struct {
	ProjectID string   `json:"projectId"`
	Status    string   `json:"status"`
	Services  []string `json:"services"`
	Output    string   `json:"output"`
	Error     string   `json:"error,omitempty"`
}

// TemplateSearchResponse 模板搜索响应.
type TemplateSearchResponse struct {
	Templates []*ComposeTemplate `json:"templates"`
	Total     int                `json:"total"`
}
