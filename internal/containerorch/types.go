// Package containerorch 提供 K3s 轻量级容器编排功能
// 支持容器、Pod、Deployment、Service 的生命周期管理和资源调度
package containerorch

import (
	"context"
	"sync"
	"time"
)

// ContainerState 容器状态.
type ContainerState string

const (
	StateCreated  ContainerState = "created"  // 已创建
	StateRunning  ContainerState = "running"  // 运行中
	StateStopped  ContainerState = "stopped"  // 已停止
	StatePaused   ContainerState = "paused"   // 已暂停
	StateFailed   ContainerState = "failed"   // 失败
	StateRemoving ContainerState = "removing" // 删除中
)

// PodPhase Pod 阶段.
type PodPhase string

const (
	PodPending   PodPhase = "pending"   // 等待中
	PodRunning   PodPhase = "running"   // 运行中
	PodSucceeded PodPhase = "succeeded" // 成功完成
	PodFailed    PodPhase = "failed"    // 失败
	PodUnknown   PodPhase = "unknown"   // 未知
)

// ServiceType 服务类型.
type ServiceType string

const (
	ServiceClusterIP ServiceType = "ClusterIP" // 集群内部访问
	ServiceNodePort  ServiceType = "NodePort"  // 节点端口暴露
	ServiceLoadBalancer ServiceType = "LoadBalancer" // 负载均衡器
)

// HealthCheckType 健康检查类型.
type HealthCheckType string

const (
	HealthCheckHTTP     HealthCheckType = "http"     // HTTP 检查
	HealthCheckTCP      HealthCheckType = "tcp"      // TCP 检查
	HealthCheckExec     HealthCheckType = "exec"     // 命令执行检查
	HealthCheckGRPC     HealthCheckType = "grpc"     // gRPC 检查
)

// RestartPolicy 重启策略.
type RestartPolicy string

const (
	RestartAlways    RestartPolicy = "always"    // 总是重启
	RestartOnFailure RestartPolicy = "onFailure" // 失败时重启
	RestartNever     RestartPolicy = "never"     // 从不重启
)

// Container 容器定义.
type Container struct {
	mu sync.RWMutex `json:"-"`

	// 基本信息
	ID          string         `json:"id"`          // 容器唯一标识
	Name        string         `json:"name"`        // 容器名称
	PodID       string         `json:"podId"`       // 所属 Pod ID
	Image       string         `json:"image"`       // 镜像名称
	ImageID     string         `json:"imageId"`     // 镜像 ID
	Command     []string       `json:"command"`     // 启动命令
	Args        []string       `json:"args"`        // 命令参数
	WorkingDir  string         `json:"workingDir"`  // 工作目录
	State       ContainerState `json:"state"`       // 容器状态
	Status      string         `json:"status"`      // 状态描述

	// 资源配置
	Resources   ResourceRequirements `json:"resources"`   // 资源需求

	// 网络配置
	Ports       []ContainerPort      `json:"ports"`       // 端口映射

	// 存储配置
	Volumes     []VolumeMount        `json:"volumes"`     // 卷挂载

	// 环境变量
	Env         []EnvVar             `json:"env"`         // 环境变量

	// 健康检查
	LivenessProbe   *HealthCheck `json:"livenessProbe,omitempty"`   // 存活探针
	ReadinessProbe  *HealthCheck `json:"readinessProbe,omitempty"`  // 就绪探针
	StartupProbe    *HealthCheck `json:"startupProbe,omitempty"`    // 启动探针

	// 生命周期钩子
	PostStart   *LifecycleHook `json:"postStart,omitempty"`   // 启动后钩子
	PreStop     *LifecycleHook `json:"preStop,omitempty"`     // 停止前钩子

	// 重启策略
	RestartPolicy RestartPolicy `json:"restartPolicy"` // 重启策略

	// 时间信息
	CreatedAt   time.Time  `json:"createdAt"`   // 创建时间
	StartedAt   *time.Time `json:"startedAt"`   // 启动时间
	FinishedAt  *time.Time `json:"finishedAt"`  // 结束时间
	RestartCount int       `json:"restartCount"` // 重启次数

	// 日志
	LogPath     string `json:"logPath"` // 日志文件路径
}

// ResourceRequirements 资源需求.
type ResourceRequirements struct {
	// 请求资源（保证分配）
	Requests ResourceList `json:"requests"` // 请求资源

	// 限制资源（最大可用）
	Limits   ResourceList `json:"limits"`   // 限制资源
}

// ResourceList 资源列表.
type ResourceList struct {
	CPU    string `json:"cpu"`    // CPU 资源（如 "100m", "1"）
	Memory string `json:"memory"` // 内存资源（如 "128Mi", "1Gi"）
	GPU    string `json:"gpu"`    // GPU 资源（如 "1"）
}

// ContainerPort 容器端口.
type ContainerPort struct {
	Name          string `json:"name"`          // 端口名称
	ContainerPort int    `json:"containerPort"` // 容器端口
	HostPort      int    `json:"hostPort"`      // 主机端口
	Protocol      string `json:"protocol"`      // 协议（TCP/UDP）
}

// VolumeMount 卷挂载.
type VolumeMount struct {
	Name      string `json:"name"`      // 卷名称
	MountPath string `json:"mountPath"` // 挂载路径
	ReadOnly  bool   `json:"readOnly"`  // 只读
	SubPath   string `json:"subPath"`   // 子路径
}

// EnvVar 环境变量.
type EnvVar struct {
	Name  string `json:"name"`  // 变量名
	Value string `json:"value"` // 变量值
}

// HealthCheck 健康检查配置.
type HealthCheck struct {
	Type               HealthCheckType `json:"type"`               // 检查类型
	Path               string          `json:"path,omitempty"`     // HTTP 检查路径
	Port               int             `json:"port,omitempty"`     // 检查端口
	Command            []string        `json:"command,omitempty"`  // 执行命令
	InitialDelaySeconds int            `json:"initialDelaySeconds"` // 初始延迟（秒）
	PeriodSeconds      int             `json:"periodSeconds"`      // 检查周期（秒）
	TimeoutSeconds     int             `json:"timeoutSeconds"`     // 超时时间（秒）
	FailureThreshold   int             `json:"failureThreshold"`   // 失败阈值
	SuccessThreshold   int             `json:"successThreshold"`   // 成功阈值
}

// LifecycleHook 生命周期钩子.
type LifecycleHook struct {
	Exec *ExecAction `json:"exec,omitempty"` // 执行命令
	HTTP *HTTPAction  `json:"http,omitempty"` // HTTP 请求
}

// ExecAction 执行命令动作.
type ExecAction struct {
	Command []string `json:"command"` // 命令
}

// HTTPAction HTTP 动作.
type HTTPAction struct {
	Path   string            `json:"path"`   // 路径
	Port   int               `json:"port"`   // 端口
	Host   string            `json:"host"`   // 主机
	Headers map[string]string `json:"headers,omitempty"` // 请求头
}

// Pod Pod 定义（一组共享资源的容器）.
type Pod struct {
	mu sync.RWMutex `json:"-"`

	// 基本信息
	ID           string   `json:"id"`           // Pod 唯一标识
	Name         string   `json:"name"`         // Pod 名称
	Namespace    string   `json:"namespace"`    // 命名空间
	DeploymentID string   `json:"deploymentId"` // 所属 Deployment ID
	NodeID       string   `json:"nodeId"`       // 调度到的节点 ID

	// 规格
	Spec         PodSpec  `json:"spec"`         // Pod 规格

	// 状态
	Phase        PodPhase `json:"phase"`        // Pod 阶段
	Conditions   []PodCondition `json:"conditions"` // Pod 条件
	Message      string   `json:"message"`      // 状态消息
	Reason       string   `json:"reason"`       // 状态原因

	// 容器列表
	Containers   []*Container `json:"containers"` // 容器列表

	// 网络
	PodIP        string   `json:"podIp"`        // Pod IP 地址
	HostIP       string   `json:"hostIp"`       // 主机 IP 地址

	// 标签和注解
	Labels       map[string]string `json:"labels"`       // 标签
	Annotations  map[string]string `json:"annotations"`  // 注解

	// 时间信息
	CreatedAt    time.Time  `json:"createdAt"`    // 创建时间
	StartedAt    *time.Time `json:"startedAt"`    // 启动时间
	FinishedAt   *time.Time `json:"finishedAt"`   // 结束时间
}

// PodSpec Pod 规格.
type PodSpec struct {
	RestartPolicy   RestartPolicy      `json:"restartPolicy"`   // 重启策略
	NodeSelector    map[string]string  `json:"nodeSelector"`    // 节点选择器
	Tolerations     []Toleration       `json:"tolerations"`     // 容忍度
	Affinity        *Affinity          `json:"affinity"`        // 亲和性
	HostNetwork     bool               `json:"hostNetwork"`     // 使用主机网络
	DNSPolicy       string             `json:"dnsPolicy"`       // DNS 策略
	ServiceAccount  string             `json:"serviceAccount"`  // 服务账号
}

// Toleration 容忍度.
type Toleration struct {
	Key      string `json:"key"`      // 键
	Operator string `json:"operator"` // 操作符
	Value    string `json:"value"`    // 值
	Effect   string `json:"effect"`   // 效果
}

// Affinity 亲和性配置.
type Affinity struct {
	NodeAffinity    *NodeAffinity    `json:"nodeAffinity,omitempty"`    // 节点亲和性
	PodAffinity     *PodAffinity     `json:"podAffinity,omitempty"`     // Pod 亲和性
	PodAntiAffinity *PodAntiAffinity `json:"podAntiAffinity,omitempty"` // Pod 反亲和性
}

// NodeAffinity 节点亲和性.
type NodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelector `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// NodeSelector 节点选择器.
type NodeSelector struct {
	NodeSelectorTerms []NodeSelectorTerm `json:"nodeSelectorTerms"`
}

// NodeSelectorTerm 节点选择器条件.
type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement `json:"matchExpressions,omitempty"`
	MatchFields      []NodeSelectorRequirement `json:"matchFields,omitempty"`
}

// NodeSelectorRequirement 节点选择器需求.
type NodeSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// PreferredSchedulingTerm 首选调度条件.
type PreferredSchedulingTerm struct {
	Weight     int              `json:"weight"`
	Preference NodeSelectorTerm `json:"preference"`
}

// PodAffinity Pod 亲和性.
type PodAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAntiAffinity Pod 反亲和性.
type PodAntiAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  []PodAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

// PodAffinityTerm Pod 亲和性条件.
type PodAffinityTerm struct {
	LabelSelector *LabelSelector `json:"labelSelector,omitempty"`
	TopologyKey   string         `json:"topologyKey"`
}

// WeightedPodAffinityTerm 加权 Pod 亲和性条件.
type WeightedPodAffinityTerm struct {
	Weight          int            `json:"weight"`
	PodAffinityTerm PodAffinityTerm `json:"podAffinityTerm"`
}

// LabelSelector 标签选择器.
type LabelSelector struct {
	MatchLabels      map[string]string        `json:"matchLabels,omitempty"`
	MatchExpressions []LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

// LabelSelectorRequirement 标签选择器需求.
type LabelSelectorRequirement struct {
	Key      string   `json:"key"`
	Operator string   `json:"operator"`
	Values   []string `json:"values,omitempty"`
}

// PodCondition Pod 条件.
type PodCondition struct {
	Type               string    `json:"type"`               // 条件类型
	Status             string    `json:"status"`             // 状态
	LastProbeTime      time.Time `json:"lastProbeTime"`      // 最后探测时间
	LastTransitionTime time.Time `json:"lastTransitionTime"` // 最后转换时间
	Reason             string    `json:"reason"`             // 原因
	Message            string    `json:"message"`            // 消息
}

// Deployment Deployment 定义.
type Deployment struct {
	mu sync.RWMutex `json:"-"`

	// 基本信息
	ID        string `json:"id"`        // Deployment 唯一标识
	Name      string `json:"name"`      // Deployment 名称
	Namespace string `json:"namespace"` // 命名空间

	// 规格
	Spec DeploymentSpec `json:"spec"` // Deployment 规格

	// 状态
	Status DeploymentStatus `json:"status"` // Deployment 状态

	// 标签和注解
	Labels      map[string]string `json:"labels"`      // 标签
	Annotations map[string]string `json:"annotations"` // 注解

	// 时间信息
	CreatedAt   time.Time `json:"createdAt"` // 创建时间
	UpdatedAt   time.Time `json:"updatedAt"` // 更新时间
}

// DeploymentSpec Deployment 规格.
type DeploymentSpec struct {
	Replicas        int              `json:"replicas"`        // 副本数
	Selector        *LabelSelector   `json:"selector"`        // 选择器
	Template        PodTemplateSpec  `json:"template"`        // Pod 模板
	Strategy        DeploymentStrategy `json:"strategy"`      // 部署策略
	MinReadySeconds int              `json:"minReadySeconds"` // 最小就绪时间（秒）
}

// DeploymentStrategy 部署策略.
type DeploymentStrategy struct {
	Type          string              `json:"type"`          // 策略类型（RollingUpdate/Recreate）
	RollingUpdate *RollingUpdateDeployment `json:"rollingUpdate,omitempty"` // 滚动更新配置
}

// RollingUpdateDeployment 滚动更新配置.
type RollingUpdateDeployment struct {
	MaxUnavailable string `json:"maxUnavailable"` // 最大不可用数
	MaxSurge       string `json:"maxSurge"`       // 最大超出数
}

// PodTemplateSpec Pod 模板规格.
type PodTemplateSpec struct {
	Metadata ObjectMeta `json:"metadata"` // 元数据
	Spec     PodSpec    `json:"spec"`     // Pod 规格
}

// ObjectMeta 对象元数据.
type ObjectMeta struct {
	Labels      map[string]string `json:"labels,omitempty"`      // 标签
	Annotations map[string]string `json:"annotations,omitempty"` // 注解
}

// DeploymentStatus Deployment 状态.
type DeploymentStatus struct {
	Replicas            int `json:"replicas"`            // 总副本数
	ReadyReplicas       int `json:"readyReplicas"`       // 就绪副本数
	AvailableReplicas   int `json:"availableReplicas"`   // 可用副本数
	UnavailableReplicas int `json:"unavailableReplicas"` // 不可用副本数
	UpdatedReplicas     int `json:"updatedReplicas"`     // 已更新副本数
	ObservedGeneration  int `json:"observedGeneration"`  // 观察到的代数
}

// Service Service 定义.
type Service struct {
	mu sync.RWMutex `json:"-"`

	// 基本信息
	ID        string      `json:"id"`        // Service 唯一标识
	Name      string      `json:"name"`      // Service 名称
	Namespace string      `json:"namespace"` // 命名空间

	// 规格
	Spec ServiceSpec `json:"spec"` // Service 规格

	// 状态
	Status ServiceStatus `json:"status"` // Service 状态

	// 标签和注解
	Labels      map[string]string `json:"labels"`      // 标签
	Annotations map[string]string `json:"annotations"` // 注解

	// 时间信息
	CreatedAt   time.Time `json:"createdAt"` // 创建时间
	UpdatedAt   time.Time `json:"updatedAt"` // 更新时间
}

// ServiceSpec Service 规格.
type ServiceSpec struct {
	Type       ServiceType      `json:"type"`       // 服务类型
	Selector   map[string]string `json:"selector"`  // 选择器
	Ports      []ServicePort    `json:"ports"`      // 端口列表
	ClusterIP  string           `json:"clusterIp"`  // 集群 IP
	ExternalIPs []string        `json:"externalIps"` // 外部 IP
	SessionAffinity string      `json:"sessionAffinity"` // 会话亲和性
}

// ServicePort Service 端口.
type ServicePort struct {
	Name       string `json:"name"`       // 端口名称
	Protocol   string `json:"protocol"`   // 协议
	Port       int    `json:"port"`       // 服务端口
	TargetPort int    `json:"targetPort"` // 目标端口
	NodePort   int    `json:"nodePort"`   // 节点端口
}

// ServiceStatus Service 状态.
type ServiceStatus struct {
	LoadBalancer LoadBalancerStatus `json:"loadBalancer"` // 负载均衡状态
}

// LoadBalancerStatus 负载均衡状态.
type LoadBalancerStatus struct {
	Ingress []LoadBalancerIngress `json:"ingress"` // 入口列表
}

// LoadBalancerIngress 负载均衡入口.
type LoadBalancerIngress struct {
	IP       string `json:"ip"`       // IP 地址
	Hostname string `json:"hostname"` // 主机名
}

// ClusterStats 集群统计.
type ClusterStats struct {
	mu sync.RWMutex `json:"-"`

	// 节点统计
	TotalNodes     int `json:"totalNodes"`     // 总节点数
	ReadyNodes     int `json:"readyNodes"`     // 就绪节点数
	NotReadyNodes  int `json:"notReadyNodes"`  // 未就绪节点数

	// Pod 统计
	TotalPods      int `json:"totalPods"`      // 总 Pod 数
	RunningPods    int `json:"runningPods"`    // 运行中 Pod 数
	PendingPods    int `json:"pendingPods"`    // 等待中 Pod 数
	FailedPods     int `json:"failedPods"`     // 失败 Pod 数
	SucceededPods  int `json:"succeededPods"`  // 成功 Pod 数

	// 资源统计
	TotalCPU       string `json:"totalCpu"`       // 总 CPU
	UsedCPU        string `json:"usedCpu"`        // 已用 CPU
	TotalMemory    string `json:"totalMemory"`    // 总内存
	UsedMemory     string `json:"usedMemory"`     // 已用内存

	// Deployment 统计
	TotalDeployments     int `json:"totalDeployments"`     // 总 Deployment 数
	AvailableDeployments int `json:"availableDeployments"` // 可用 Deployment 数

	// Service 统计
	TotalServices int `json:"totalServices"` // 总 Service 数

	// 时间
	LastUpdated time.Time `json:"lastUpdated"` // 最后更新时间
}

// GetSnapshot 获取统计快照（线程安全）.
func (s *ClusterStats) GetSnapshot() *ClusterStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &ClusterStats{
		TotalNodes:           s.TotalNodes,
		ReadyNodes:           s.ReadyNodes,
		NotReadyNodes:        s.NotReadyNodes,
		TotalPods:            s.TotalPods,
		RunningPods:          s.RunningPods,
		PendingPods:          s.PendingPods,
		FailedPods:           s.FailedPods,
		SucceededPods:        s.SucceededPods,
		TotalCPU:             s.TotalCPU,
		UsedCPU:              s.UsedCPU,
		TotalMemory:          s.TotalMemory,
		UsedMemory:           s.UsedMemory,
		TotalDeployments:     s.TotalDeployments,
		AvailableDeployments: s.AvailableDeployments,
		TotalServices:        s.TotalServices,
		LastUpdated:          s.LastUpdated,
	}
}

// HealthChecker 容器健康检查器
type HealthChecker struct {
	manager *Manager
	stopCh  chan struct{}
}

// NewHealthChecker 创建健康检查器
func NewHealthChecker(manager *Manager) *HealthChecker {
	return &HealthChecker{
		manager: manager,
		stopCh:  make(chan struct{}),
	}
}

// Start 启动健康检查
func (h *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-h.stopCh:
			return
		case <-ticker.C:
			h.check()
		}
	}
}

// Stop 停止健康检查
func (h *HealthChecker) Stop() {
	close(h.stopCh)
}

// check 执行一次健康检查
func (h *HealthChecker) check() {
	// 健康检查逻辑
}
