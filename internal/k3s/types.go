// Package k3s 提供 K3s 容器编排管理功能
// 集群管理、Helm Chart 部署、工作负载管理、服务网格、自动扩缩容、应用商店集成
package k3s

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNodeNotFound 节点不存在
	ErrNodeNotFound = errors.New("节点不存在")
	// ErrClusterNotReady 集群未就绪
	ErrClusterNotReady = errors.New("集群未就绪")
	// ErrHelmReleaseNotFound Helm Release 不存在
	ErrHelmReleaseNotFound = errors.New("Helm Release 不存在")
	// ErrHelmReleaseExists Helm Release 已存在
	ErrHelmReleaseExists = errors.New("Helm Release 已存在")
	// ErrChartNotFound Chart 不存在
	ErrChartNotFound = errors.New("Chart 不存在")
	// ErrWorkloadNotFound 工作负载不存在
	ErrWorkloadNotFound = errors.New("工作负载不存在")
	// ErrPodNotFound Pod 不存在
	ErrPodNotFound = errors.New("Pod 不存在")
	// ErrServiceMeshNotEnabled 服务网格未启用
	ErrServiceMeshNotEnabled = errors.New("服务网格未启用")
	// ErrHPANotFound HPA 配置不存在
	ErrHPANotFound = errors.New("HPA 配置不存在")
	// ErrQuotaNotFound 资源配额不存在
	ErrQuotaNotFound = errors.New("资源配额不存在")
	// ErrQuotaExceeded 资源配额超限
	ErrQuotaExceeded = errors.New("资源配额超限")
	// ErrInvalidNamespace 命名空间无效
	ErrInvalidNamespace = errors.New("命名空间无效")
	// ErrRollbackFailed 回滚失败
	ErrRollbackFailed = errors.New("回滚失败")
)

// ========== 集群类型 ==========

// ClusterStatus 集群状态
type ClusterStatus string

const (
	ClusterStatusRunning  ClusterStatus = "running"  // 运行中
	ClusterStatusDegraded ClusterStatus = "degraded"  // 降级
	ClusterStatusStopped  ClusterStatus = "stopped"   // 已停止
	ClusterStatusUnknown  ClusterStatus = "unknown"   // 未知
)

// ClusterInfo 集群基本信息
type ClusterInfo struct {
	Name        string        `json:"name"`         // 集群名称
	Version     string        `json:"version"`      // K3s 版本
	Status      ClusterStatus `json:"status"`       // 集群状态
	NodeCount   int           `json:"node_count"`   // 节点数
	PodCount    int           `json:"pod_count"`    // Pod 数量
	Namespaces  int           `json:"namespaces"`   // 命名空间数
	Uptime      string        `json:"uptime"`       // 运行时长
	APIEndpoint string        `json:"api_endpoint"` // API 端点
	CreatedAt   time.Time     `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time     `json:"updated_at"`   // 更新时间
}

// ========== 节点类型 ==========

// NodeRole 节点角色
type NodeRole string

const (
	NodeRoleMaster NodeRole = "master" // 主节点
	NodeRoleWorker NodeRole = "worker" // 工作节点
)

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeStatusReady    NodeStatus = "ready"    // 就绪
	NodeStatusNotReady NodeStatus = "not_ready" // 未就绪
	NodeStatusScheduling NodeStatus = "scheduling_disabled" // 调度禁用
	NodeStatusUnknown  NodeStatus = "unknown"  // 未知
)

// NodeInfo 节点信息
type NodeInfo struct {
	Name       string     `json:"name"`        // 节点名称
	Role       NodeRole   `json:"role"`        // 节点角色
	Status     NodeStatus `json:"status"`      // 节点状态
	IP         string     `json:"ip"`          // 节点 IP
	OS         string     `json:"os"`          // 操作系统
	Arch       string     `json:"arch"`        // 架构
	KubeletVer string     `json:"kubelet_ver"` // Kubelet 版本
	CPUCores   int        `json:"cpu_cores"`   // CPU 核数
	MemoryGB   float64    `json:"memory_gb"`   // 内存 (GB)
	DiskGB     float64    `json:"disk_gb"`     // 磁盘 (GB)
	PodCount   int        `json:"pod_count"`   // Pod 数量
	Labels     map[string]string `json:"labels,omitempty"` // 标签
	Taints     []string   `json:"taints,omitempty"` // 污点
	Conditions []NodeCondition `json:"conditions,omitempty"` // 节点条件
	CreatedAt  time.Time  `json:"created_at"`  // 加入时间
	UpdatedAt  time.Time  `json:"updated_at"`  // 更新时间
}

// NodeCondition 节点条件
type NodeCondition struct {
	Type    string    `json:"type"`    // 条件类型
	Status  string    `json:"status"`  // 状态
	Reason  string    `json:"reason"`  // 原因
	Message string    `json:"message"` // 消息
	LastTime time.Time `json:"last_time"` // 最后更新时间
}

// ========== 集群健康检查 ==========

// HealthCheckType 健康检查类型
type HealthCheckType string

const (
	HealthCheckComponent  HealthCheckType = "component"  // 组件健康
	HealthCheckNode       HealthCheckType = "node"        // 节点健康
	HealthCheckWorkload   HealthCheckType = "workload"    // 工作负载健康
	HealthCheckCluster    HealthCheckType = "cluster"     // 集群整体健康
)

// ClusterHealth 集群健康状态
type ClusterHealth struct {
	Status     string           `json:"status"`      // 整体状态: healthy, warning, critical
	Components []ComponentHealth `json:"components"`  // 组件健康
	Nodes      []NodeHealth     `json:"nodes"`       // 节点健康
	CheckedAt  time.Time        `json:"checked_at"`  // 检查时间
}

// ComponentHealth 组件健康
type ComponentHealth struct {
	Name    string `json:"name"`    // 组件名称
	Status  string `json:"status"`  // 状态
	Message string `json:"message"` // 描述
}

// NodeHealth 节点健康
type NodeHealth struct {
	Name        string `json:"name"`         // 节点名称
	Ready       bool   `json:"ready"`        // 是否就绪
	DiskPressure bool  `json:"disk_pressure"` // 磁盘压力
	MemoryPressure bool `json:"memory_pressure"` // 内存压力
	PIDPressure  bool  `json:"pid_pressure"` // PID 压力
}

// ========== Helm Chart 类型 ==========

// HelmReleaseStatus Helm Release 状态
type HelmReleaseStatus string

const (
	HelmStatusDeployed  HelmReleaseStatus = "deployed"  // 已部署
	HelmStatusFailed    HelmReleaseStatus = "failed"    // 失败
	HelmStatusPending   HelmReleaseStatus = "pending"   // 等待中
	HelmStatusSuperseded HelmReleaseStatus = "superseded" // 已替代
	HelmStatusUninstalled HelmReleaseStatus = "uninstalled" // 已卸载
)

// HelmRelease Helm Release 信息
type HelmRelease struct {
	ID          string            `json:"id"`           // 唯一标识
	Name        string            `json:"name"`         // Release 名称
	Namespace   string            `json:"namespace"`    // 命名空间
	Chart       string            `json:"chart"`        // Chart 名称
	ChartVer    string            `json:"chart_ver"`    // Chart 版本
	AppVer      string            `json:"app_ver"`      // 应用版本
	Status      HelmReleaseStatus `json:"status"`       // 状态
	Revision    int               `json:"revision"`     // 修订版本
	Values      map[string]interface{} `json:"values,omitempty"` // 配置值
	Description string            `json:"description,omitempty"` // 描述
	Notes       string            `json:"notes,omitempty"` // 部署备注
	DeployedAt  time.Time         `json:"deployed_at"`  // 部署时间
	UpdatedAt   time.Time         `json:"updated_at"`   // 更新时间
}

// HelmChartInfo Helm Chart 信息（仓库索引）
type HelmChartInfo struct {
	Name        string   `json:"name"`         // Chart 名称
	Version     string   `json:"version"`      // 版本
	AppVersion  string   `json:"app_version"`  // 应用版本
	Description string   `json:"description"`  // 描述
	Repository  string   `json:"repository"`   // 所属仓库
	Home        string   `json:"home"`         // 主页
	Keywords    []string `json:"keywords,omitempty"` // 关键词
	Maintainers []string `json:"maintainers,omitempty"` // 维护者
}

// DeployChartRequest 部署 Chart 请求
type DeployChartRequest struct {
	Name      string                 `json:"name" binding:"required"`      // Release 名称
	Namespace string                 `json:"namespace" binding:"required"` // 命名空间
	Chart     string                 `json:"chart" binding:"required"`     // Chart 名称 (repo/chart)
	Version   string                 `json:"version"`                       // Chart 版本，空则最新
	Values    map[string]interface{} `json:"values,omitempty"`             // 配置值
	Wait      bool                   `json:"wait"`                          // 是否等待就绪
	Timeout   int                    `json:"timeout"`                       // 超时秒数
	Description string               `json:"description,omitempty"`        // 描述
}

// UpgradeChartRequest 升级 Chart 请求
type UpgradeChartRequest struct {
	Version   string                 `json:"version"`               // 新版本
	Values    map[string]interface{} `json:"values,omitempty"`     // 新配置值
	Wait      bool                   `json:"wait"`                  // 是否等待就绪
	Timeout   int                    `json:"timeout"`               // 超时秒数
	ResetValues bool                 `json:"reset_values"`         // 是否重置值
	Description string               `json:"description,omitempty"` // 描述
}

// RollbackChartRequest 回滚 Chart 请求
type RollbackChartRequest struct {
	Revision int  `json:"revision" binding:"required"` // 回滚到的修订版本
	Wait     bool `json:"wait"`                        // 是否等待就绪
	Timeout  int  `json:"timeout"`                     // 超时秒数
}

// ========== 工作负载类型 ==========

// WorkloadType 工作负载类型
type WorkloadType string

const (
	WorkloadDeployment  WorkloadType = "deployment"  // Deployment
	WorkloadStatefulSet WorkloadType = "statefulset" // StatefulSet
	WorkloadDaemonSet   WorkloadType = "daemonset"   // DaemonSet
	WorkloadJob         WorkloadType = "job"         // Job
	WorkloadCronJob     WorkloadType = "cronjob"     // CronJob
)

// DeploymentInfo Deployment 信息
type DeploymentInfo struct {
	Name      string `json:"name"`       // 名称
	Namespace string `json:"namespace"`  // 命名空间
	Ready     int    `json:"ready"`      // 就绪副本数
	Desired   int    `json:"desired"`    // 期望副本数
	Updated   int    `json:"updated"`    // 已更新副本数
	Available int    `json:"available"`  // 可用副本数
	Strategy  string `json:"strategy"`   // 更新策略
	Image     string `json:"image"`      // 镜像
	Labels    map[string]string `json:"labels,omitempty"` // 标签
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// ServiceInfo Service 信息
type ServiceInfo struct {
	Name       string            `json:"name"`        // 名称
	Namespace  string            `json:"namespace"`   // 命名空间
	Type       string            `json:"type"`        // 类型 (ClusterIP, NodePort, LoadBalancer)
	ClusterIP  string            `json:"cluster_ip"`  // Cluster IP
	Ports      []ServicePort     `json:"ports"`       // 端口列表
	Selector   map[string]string `json:"selector,omitempty"` // 选择器
	Labels     map[string]string `json:"labels,omitempty"`   // 标签
	CreatedAt  time.Time         `json:"created_at"`  // 创建时间
}

// ServicePort Service 端口
type ServicePort struct {
	Name       string `json:"name"`        // 端口名称
	Port       int    `json:"port"`        // 服务端口
	TargetPort int    `json:"target_port"` // 目标端口
	NodePort   int    `json:"node_port"`   // 节点端口
	Protocol   string `json:"protocol"`    // 协议
}

// PodInfo Pod 信息
type PodInfo struct {
	Name      string            `json:"name"`       // 名称
	Namespace string            `json:"namespace"`  // 命名空间
	Status    string            `json:"status"`     // 状态
	IP        string            `json:"ip"`         // Pod IP
	Node      string            `json:"node"`       // 所在节点
	Restarts  int               `json:"restarts"`   // 重启次数
	Labels    map[string]string `json:"labels,omitempty"` // 标签
	Containers []ContainerInfo  `json:"containers"` // 容器列表
	CreatedAt time.Time         `json:"created_at"` // 创建时间
}

// ContainerInfo 容器信息
type ContainerInfo struct {
	Name         string `json:"name"`          // 容器名
	Image        string `json:"image"`         // 镜像
	Ready        bool   `json:"ready"`         // 是否就绪
	RestartCount int    `json:"restart_count"` // 重启次数
	State        string `json:"state"`         // 状态
}

// PodLogRequest Pod 日志请求
type PodLogRequest struct {
	Namespace string `json:"namespace" binding:"required"` // 命名空间
	PodName   string `json:"pod_name" binding:"required"`  // Pod 名称
	Container string `json:"container"`                     // 容器名（可选）
	TailLines int    `json:"tail_lines"`                   // 尾行数（默认 100）
	Follow    bool   `json:"follow"`                        // 是否跟踪
	SinceSec  int    `json:"since_sec"`                     // 最近 N 秒
}

// PodLogResult Pod 日志结果
type PodLogResult struct {
	PodName   string   `json:"pod_name"`
	Container string   `json:"container"`
	Lines     []string `json:"lines"`
}

// ========== 服务网格类型 ==========

// ServiceMeshType 服务网格类型
type ServiceMeshType string

const (
	ServiceMeshNone    ServiceMeshType = "none"    // 未启用
	ServiceMeshIstio   ServiceMeshType = "istio"   // Istio
	ServiceMeshLinkerd ServiceMeshType = "linkerd" // Linkerd
)

// ServiceMeshConfig 服务网格配置
type ServiceMeshConfig struct {
	Enabled     bool              `json:"enabled"`      // 是否启用
	Type        ServiceMeshType   `json:"type"`         // 网格类型
	Version     string            `json:"version"`      // 版本
	Namespace   string            `json:"namespace"`    // 控制面命名空间
	MTLS        bool              `json:"mtls"`         // 是否启用 mTLS
	Tracing     bool              `json:"tracing"`      // 是否启用链路追踪
	TracingURL  string            `json:"tracing_url"`  // 链路追踪地址
	AccessLog   bool              `json:"access_log"`   // 是否启用访问日志
	UpdatedAt   time.Time         `json:"updated_at"`   // 更新时间
}

// ========== 自动扩缩容类型 ==========

// HPAConfig HPA 配置
type HPAConfig struct {
	ID          string       `json:"id"`           // 唯一标识
	Name        string       `json:"name"`         // HPA 名称
	Namespace   string       `json:"namespace"`    // 命名空间
	TargetKind  string       `json:"target_kind"`  // 目标类型 (Deployment/StatefulSet)
	TargetName  string       `json:"target_name"`  // 目标名称
	MinReplicas int          `json:"min_replicas"` // 最小副本数
	MaxReplicas int          `json:"max_replicas"` // 最大副本数
	Metrics     []HPAMetric  `json:"metrics"`      // 扩缩指标
	Behavior    *HPABehavior `json:"behavior,omitempty"` // 扩缩行为
	CreatedAt   time.Time    `json:"created_at"`   // 创建时间
	UpdatedAt   time.Time    `json:"updated_at"`   // 更新时间
}

// HPAMetric HPA 指标
type HPAMetric struct {
	Type     string `json:"type"`      // 指标类型: Resource, Pods, Object, External
	Resource string `json:"resource"`  // 资源名: cpu, memory
	Target   string `json:"target"`    // 目标类型: Utilization, AverageValue
	Value    int    `json:"value"`     // 目标值
}

// HPABehavior HPA 扩缩行为
type HPABehavior struct {
	ScaleUp   *HPAScalingRule `json:"scale_up,omitempty"`   // 扩容规则
	ScaleDown *HPAScalingRule `json:"scale_down,omitempty"` // 缩容规则
}

// HPAScalingRule HPA 扩缩规则
type HPAScalingRule struct {
	StabilizationWindow int              `json:"stabilization_window"` // 稳定窗口 (秒)
	Policies            []HPAPolicy      `json:"policies"`             // 策略列表
	SelectPolicy        string           `json:"select_policy"`        // 选择策略: Max, Min, Disabled
}

// HPAPolicy HPA 策略
type HPAPolicy struct {
	Type          string `json:"type"`           // 策略类型: Pods, Percent
	Value         int    `json:"value"`          // 值
	PeriodSeconds int    `json:"period_seconds"` // 周期 (秒)
}

// CreateHPARequest 创建 HPA 请求
type CreateHPARequest struct {
	Name        string       `json:"name" binding:"required"`
	Namespace   string       `json:"namespace" binding:"required"`
	TargetKind  string       `json:"target_kind"`
	TargetName  string       `json:"target_name" binding:"required"`
	MinReplicas int          `json:"min_replicas"`
	MaxReplicas int          `json:"max_replicas" binding:"required"`
	Metrics     []HPAMetric  `json:"metrics"`
	Behavior    *HPABehavior `json:"behavior,omitempty"`
}

// UpdateHPARequest 更新 HPA 请求
type UpdateHPARequest struct {
	MinReplicas *int          `json:"min_replicas,omitempty"`
	MaxReplicas *int          `json:"max_replicas,omitempty"`
	Metrics     []HPAMetric   `json:"metrics,omitempty"`
	Behavior    *HPABehavior  `json:"behavior,omitempty"`
}

// ========== 应用商店集成 ==========

// AppStoreDeployRequest 从应用商店部署请求
type AppStoreDeployRequest struct {
	AppID      string                 `json:"app_id" binding:"required"`     // 应用商店的应用 ID
	Namespace  string                 `json:"namespace" binding:"required"`  // 部署命名空间
	ReleaseName string                `json:"release_name"`                  // Release 名称（可选，默认用 app_id）
	Values     map[string]interface{} `json:"values,omitempty"`             // 覆盖配置值
	Wait       bool                   `json:"wait"`                          // 等待就绪
}

// AppStoreApp 应用商店应用摘要（对接 appstore 模块）
type AppStoreApp struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Version     string `json:"version"`
	ChartRepo   string `json:"chart_repo"`
	ChartName   string `json:"chart_name"`
	Installed   bool   `json:"installed"`
}

// ========== 资源配额类型 ==========

// ResourceQuota 资源配额
type ResourceQuota struct {
	ID        string             `json:"id"`         // 唯一标识
	Namespace string             `json:"namespace"`  // 命名空间
	Name      string             `json:"name"`       // 配额名称
	Hard      map[string]string  `json:"hard"`       // 硬限制
	Used      map[string]string  `json:"used"`       // 已使用
	CreatedAt time.Time          `json:"created_at"` // 创建时间
	UpdatedAt time.Time          `json:"updated_at"` // 更新时间
}

// CreateQuotaRequest 创建配额请求
type CreateQuotaRequest struct {
	Namespace string            `json:"namespace" binding:"required"` // 命名空间
	Name      string            `json:"name" binding:"required"`      // 配额名
	Hard      map[string]string `json:"hard" binding:"required"`      // 硬限制
}

// UpdateQuotaRequest 更新配额请求
type UpdateQuotaRequest struct {
	Hard map[string]string `json:"hard" binding:"required"` // 硬限制
}

// ========== 集群事件类型 ==========

// EventSeverity 事件严重级别
type EventSeverity string

const (
	EventSeverityNormal  EventSeverity = "normal"  // 正常
	EventSeverityWarning EventSeverity = "warning" // 警告
	EventSeverityError   EventSeverity = "error"   // 错误
)

// ClusterEvent 集群事件
type ClusterEvent struct {
	ID          string        `json:"id"`           // 唯一标识
	Namespace   string        `json:"namespace"`    // 命名空间
	Kind        string        `json:"kind"`         // 资源类型
	Name        string        `json:"name"`         // 资源名称
	Reason      string        `json:"reason"`       // 原因
	Message     string        `json:"message"`      // 消息
	Severity    EventSeverity `json:"severity"`     // 严重级别
	Source      string        `json:"source"`       // 来源
	Count       int           `json:"count"`        // 发生次数
	FirstTime   time.Time     `json:"first_time"`   // 首次发生
	LastTime    time.Time     `json:"last_time"`    // 最后发生
}

// ========== 通用查询参数 ==========

// NamespaceFilter 命名空间过滤
type NamespaceFilter struct {
	Namespace string `form:"namespace"` // 命名空间，空表示全部
}

// ListOptions 列表选项
type ListOptions struct {
	Namespace string `form:"namespace"`  // 命名空间
	LabelSel  string `form:"label_selector"` // 标签选择器
	Limit     int    `form:"limit"`      // 返回数量限制
	Offset    int    `form:"offset"`     // 偏移量
}

// ========== 标准 API 响应 ==========

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
