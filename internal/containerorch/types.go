package containerorch

import (
	"fmt"
	"sync"
	"time"
)

// ContainerOrchManager 容器编排管理器
type ContainerOrchManager struct {
	mu         sync.RWMutex
	containers map[string]*Container
	networks   map[string]*Network
	volumes    map[string]*Volume
	stacks     map[string]*Stack
	config     *OrchConfig
}

type OrchConfig struct {
	DefaultRegistry string `json:"default_registry"`
	AutoRestart     bool   `json:"auto_restart"`
	MaxContainers   int    `json:"max_containers"`
}

type Container struct {
	ID             string               `json:"id"`
	Name           string               `json:"name"`
	PodID          string               `json:"podId,omitempty"`
	Image          string               `json:"image"`
	Command        []string             `json:"command,omitempty"`
	Args           []string             `json:"args,omitempty"`
	WorkingDir     string               `json:"workingDir,omitempty"`
	Status         ContainerStatus      `json:"status"`
	State          string               `json:"state"`
	Ports          []PortMapping        `json:"ports"`
	Env            map[string]string    `json:"env"`
	Volumes        []string             `json:"volumes"`
	Networks       []string             `json:"networks"`
	Labels         map[string]string    `json:"labels"`
	CreatedAt      time.Time            `json:"created_at"`
	StartedAt      *time.Time           `json:"started_at,omitempty"`
	FinishedAt     *time.Time           `json:"finished_at,omitempty"`
	RestartCount   int                  `json:"restart_count"`
	Health         string               `json:"health"`
	Resources      ResourceLimits       `json:"resources"`
	LivenessProbe  *HealthCheck         `json:"livenessProbe,omitempty"`
	ReadinessProbe *HealthCheck         `json:"readinessProbe,omitempty"`
	StartupProbe   *HealthCheck         `json:"startupProbe,omitempty"`
	RestartPolicy  string               `json:"restartPolicy,omitempty"`
	LogPath        string               `json:"logPath,omitempty"`
}

type ContainerStatus string
const (
	StatusCreated  ContainerStatus = "created"
	StatusRunning  ContainerStatus = "running"
	StatusStopped  ContainerStatus = "stopped"
	StatusPaused   ContainerStatus = "paused"
	StatusFailed   ContainerStatus = "failed"
	StateCreated   string = "created"
	StateRunning   string = "running"
	StateStopped   string = "stopped"
)

type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
	HostIP        string `json:"host_ip"`
}

type ResourceLimits struct {
	CPUShares  int         `json:"cpu_shares"`
	MemoryMB   int         `json:"memory_mb"`
	MemorySwap int         `json:"memory_swap_mb"`
	Requests   ResourceList `json:"requests,omitempty"`
}

// RestartPolicy 重启策略
type RestartPolicy string

const (
	RestartPolicyAlways    RestartPolicy = "Always"
	RestartPolicyOnFailure RestartPolicy = "OnFailure"
	RestartPolicyNever     RestartPolicy = "Never"
)

// PodSpec Pod规格 (用于请求)
type PodSpec struct {
	Containers   []ContainerSpec    `json:"containers"`
	NodeSelector map[string]string  `json:"nodeSelector,omitempty"`
}

// DeploymentSpec 部署规格
type DeploymentSpec struct {
	Replicas int        `json:"replicas"`
	Template PodTemplate `json:"template"`
}

// ServiceSpec 服务规格 (用于请求)
type ServiceSpec struct {
	Type      ServiceType       `json:"type"`
	ClusterIP string            `json:"clusterIp,omitempty"`
	Ports     []ServicePort     `json:"ports"`
	Selector  map[string]string `json:"selector"`
}

type Network struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Driver   string            `json:"driver"`
	Subnet   string            `json:"subnet"`
	Gateway  string            `json:"gateway"`
	Labels   map[string]string `json:"labels"`
	CreatedAt time.Time        `json:"created_at"`
}

type Volume struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Driver    string            `json:"driver"`
	Mountpoint string           `json:"mountpoint"`
	Labels    map[string]string `json:"labels"`
	Size      int64             `json:"size_bytes"`
	CreatedAt time.Time         `json:"created_at"`
}

type Stack struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Services   []StackService    `json:"services"`
	Networks   []string          `json:"networks"`
	Volumes    []string          `json:"volumes"`
	Status     string            `json:"status"`
	Labels     map[string]string `json:"labels"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

type StackService struct {
	Name      string            `json:"name"`
	Image     string            `json:"image"`
	Replicas  int               `json:"replicas"`
	Ports     []PortMapping     `json:"ports"`
	Env       map[string]string `json:"env"`
	Volumes   []string          `json:"volumes"`
	Networks  []string          `json:"networks"`
}

func NewContainerOrchManager(config *OrchConfig) *ContainerOrchManager {
	if config == nil {
		config = &OrchConfig{
			DefaultRegistry: "docker.io",
			AutoRestart:     true,
			MaxContainers:   100,
		}
	}
	return &ContainerOrchManager{
		containers: make(map[string]*Container),
		networks:   make(map[string]*Network),
		volumes:    make(map[string]*Volume),
		stacks:     make(map[string]*Stack),
		config:     config,
	}
}

func (m *ContainerOrchManager) CreateContainer(req *Container) (*Container, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("容器名称不能为空")
	}
	if req.Image == "" {
		return nil, fmt.Errorf("镜像不能为空")
	}

	if len(m.containers) >= m.config.MaxContainers {
		return nil, fmt.Errorf("已达到最大容器数量限制: %d", m.config.MaxContainers)
	}

	// 检查名称唯一性
	for _, c := range m.containers {
		if c.Name == req.Name {
			return nil, fmt.Errorf("容器名称已存在: %s", req.Name)
		}
	}

	req.ID = fmt.Sprintf("ctr_%d", time.Now().UnixNano())
	req.Status = StatusCreated
	req.State = "created"
	req.CreatedAt = time.Now()
	if req.Env == nil {
		req.Env = make(map[string]string)
	}
	if req.Labels == nil {
		req.Labels = make(map[string]string)
	}

	m.containers[req.ID] = req
	return req, nil
}

func (m *ContainerOrchManager) StartContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctr, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("容器不存在: %s", id)
	}

	if ctr.Status == StatusRunning {
		return fmt.Errorf("容器已在运行")
	}

	ctr.Status = StatusRunning
	ctr.State = "running"
	now := time.Now()
	ctr.StartedAt = &now
	return nil
}

func (m *ContainerOrchManager) StopContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctr, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("容器不存在: %s", id)
	}

	if ctr.Status == StatusStopped {
		return fmt.Errorf("容器已停止")
	}

	ctr.Status = StatusStopped
	ctr.State = "stopped"
	now := time.Now()
	ctr.FinishedAt = &now
	return nil
}

func (m *ContainerOrchManager) RestartContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctr, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("容器不存在: %s", id)
	}

	ctr.Status = StatusRunning
	ctr.State = "running"
	ctr.RestartCount++
	now := time.Now()
	ctr.StartedAt = &now
	return nil
}

func (m *ContainerOrchManager) RemoveContainer(id string, force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	ctr, exists := m.containers[id]
	if !exists {
		return fmt.Errorf("容器不存在: %s", id)
	}

	if ctr.Status == StatusRunning && !force {
		return fmt.Errorf("容器正在运行中，使用 force=true 强制删除")
	}

	delete(m.containers, id)
	return nil
}

func (m *ContainerOrchManager) GetContainer(id string) (*Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctr, exists := m.containers[id]
	if !exists {
		return nil, fmt.Errorf("容器不存在: %s", id)
	}
	return ctr, nil
}

func (m *ContainerOrchManager) ListContainers() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ctrs := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		ctrs = append(ctrs, c)
	}
	return ctrs
}

func (m *ContainerOrchManager) CreateNetwork(net *Network) (*Network, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if net.Name == "" {
		return nil, fmt.Errorf("网络名称不能为空")
	}

	for _, n := range m.networks {
		if n.Name == net.Name {
			return nil, fmt.Errorf("网络名称已存在: %s", net.Name)
		}
	}

	net.ID = fmt.Sprintf("net_%d", time.Now().UnixNano())
	net.CreatedAt = time.Now()
	if net.Labels == nil {
		net.Labels = make(map[string]string)
	}

	m.networks[net.ID] = net
	return net, nil
}

func (m *ContainerOrchManager) ListNetworks() []*Network {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nets := make([]*Network, 0, len(m.networks))
	for _, n := range m.networks {
		nets = append(nets, n)
	}
	return nets
}

func (m *ContainerOrchManager) RemoveNetwork(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.networks[id]; !exists {
		return fmt.Errorf("网络不存在: %s", id)
	}

	delete(m.networks, id)
	return nil
}

func (m *ContainerOrchManager) CreateVolume(vol *Volume) (*Volume, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if vol.Name == "" {
		return nil, fmt.Errorf("卷名称不能为空")
	}

	for _, v := range m.volumes {
		if v.Name == vol.Name {
			return nil, fmt.Errorf("卷名称已存在: %s", vol.Name)
		}
	}

	vol.ID = fmt.Sprintf("vol_%d", time.Now().UnixNano())
	vol.CreatedAt = time.Now()
	if vol.Labels == nil {
		vol.Labels = make(map[string]string)
	}

	m.volumes[vol.ID] = vol
	return vol, nil
}

func (m *ContainerOrchManager) ListVolumes() []*Volume {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vols := make([]*Volume, 0, len(m.volumes))
	for _, v := range m.volumes {
		vols = append(vols, v)
	}
	return vols
}

func (m *ContainerOrchManager) RemoveVolume(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.volumes[id]; !exists {
		return fmt.Errorf("卷不存在: %s", id)
	}

	delete(m.volumes, id)
	return nil
}

func (m *ContainerOrchManager) DeployStack(stack *Stack) (*Stack, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if stack.Name == "" {
		return nil, fmt.Errorf("栈名称不能为空")
	}

	for _, s := range m.stacks {
		if s.Name == stack.Name {
			return nil, fmt.Errorf("栈名称已存在: %s", stack.Name)
		}
	}

	stack.ID = fmt.Sprintf("stack_%d", time.Now().UnixNano())
	stack.Status = "deploying"
	stack.CreatedAt = time.Now()
	stack.UpdatedAt = time.Now()
	if stack.Labels == nil {
		stack.Labels = make(map[string]string)
	}

	m.stacks[stack.ID] = stack

	// 模拟部署完成
	go func() {
		time.Sleep(1 * time.Second)
		m.mu.Lock()
		defer m.mu.Unlock()
		stack.Status = "running"
		stack.UpdatedAt = time.Now()
	}()

	return stack, nil
}

func (m *ContainerOrchManager) ListStacks() []*Stack {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stacks := make([]*Stack, 0, len(m.stacks))
	for _, s := range m.stacks {
		stacks = append(stacks, s)
	}
	return stacks
}

func (m *ContainerOrchManager) RemoveStack(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stack, exists := m.stacks[id]
	if !exists {
		return fmt.Errorf("栈不存在: %s", id)
	}

	if stack.Status == "running" {
		return fmt.Errorf("栈正在运行中，请先停止")
	}

	delete(m.stacks, id)
	return nil
}

func (m *ContainerOrchManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	running := 0
	stopped := 0
	for _, c := range m.containers {
		if c.Status == StatusRunning {
			running++
		} else {
			stopped++
		}
	}

	return map[string]interface{}{
		"total_containers": len(m.containers),
		"running":          running,
		"stopped":          stopped,
		"networks":         len(m.networks),
		"volumes":          len(m.volumes),
		"stacks":           len(m.stacks),
	}
}

// ========== Kubernetes 风格类型 ==========

// Pod Pod定义
type Pod struct {
	mu           sync.RWMutex    `json:"-"`
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	DeploymentID string            `json:"deploymentId,omitempty"`
	NodeID       string            `json:"nodeId,omitempty"`
	HostIP       string            `json:"hostIp,omitempty"`
	PodIP        string            `json:"podIp,omitempty"`
	Phase        PodPhase          `json:"phase"`
	Spec         PodSpec           `json:"spec"`
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	Containers   []*Container      `json:"containers"`
	CreatedAt    time.Time         `json:"created_at"`
	StartedAt    *time.Time        `json:"started_at,omitempty"`
	FinishedAt   *time.Time        `json:"finished_at,omitempty"`
	UpdatedAt    time.Time         `json:"updated_at"`
}

// PodPhase Pod阶段
type PodPhase string

const (
	PodPending   PodPhase = "pending"
	PodRunning   PodPhase = "running"
	PodSucceeded PodPhase = "succeeded"
	PodFailed    PodPhase = "failed"
	PodUnknown   PodPhase = "unknown"
)

// PodStatus Pod状态 (兼容)
type PodStatus = PodPhase

// Deployment 部署定义
type Deployment struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Namespace   string               `json:"namespace"`
	Spec        DeploymentSpecData   `json:"spec"`
	Labels      map[string]string    `json:"labels,omitempty"`
	Annotations map[string]string    `json:"annotations,omitempty"`
	Status      DeploymentStatusData `json:"status"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

// DeploymentSpecData 部署规格数据
type DeploymentSpecData struct {
	Replicas int                  `json:"replicas"`
	Selector map[string]string    `json:"selector,omitempty"`
	Template DeploymentTemplate   `json:"template"`
}

// DeploymentTemplate 部署模板
type DeploymentTemplate struct {
	Metadata TemplateMetadata `json:"metadata"`
	Spec     PodSpecData      `json:"spec"`
}

// TemplateMetadata 模板元数据
type TemplateMetadata struct {
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// PodSpecData Pod规格数据
type PodSpecData struct {
	Containers []ContainerSpec `json:"containers"`
}

// DeploymentStatusData 部署状态数据
type DeploymentStatusData struct {
	Replicas           int    `json:"replicas"`
	ReadyReplicas      int    `json:"readyReplicas,omitempty"`
	AvailableReplicas  int    `json:"availableReplicas,omitempty"`
	Status             string `json:"status,omitempty"`
}

// DeploymentStatus 部署状态 (兼容)
type DeploymentStatus = DeploymentStatusData

const (
	DeploymentStatusProgressing = "progressing"
	DeploymentStatusAvailable   = "available"
	DeploymentStatusFailed      = "failed"
)

// PodTemplate Pod模板
type PodTemplate struct {
	Metadata TemplateMetadata `json:"metadata"`
	Spec     PodSpecData      `json:"spec"`
}

// ContainerSpec 容器规格
type ContainerSpec struct {
	Name      string               `json:"name"`
	Image     string               `json:"image"`
	Ports     []ContainerPort      `json:"ports,omitempty"`
	Env       []EnvVar             `json:"env,omitempty"`
	Resources *ResourceRequirements `json:"resources,omitempty"`
	Volumes   []VolumeMount        `json:"volumes,omitempty"`
	HealthCheck *HealthCheck       `json:"health_check,omitempty"`
}

// Service 服务定义
type Service struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Spec        ServiceSpecData   `json:"spec"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Status      ServiceStatus     `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// ServiceSpecData 服务规格数据
type ServiceSpecData struct {
	Type      ServiceType   `json:"type"`
	ClusterIP string        `json:"clusterIp,omitempty"`
	Ports     []ServicePort `json:"ports"`
	Selector  map[string]string `json:"selector"`
}

// ServiceType 服务类型
type ServiceType string

const (
	ServiceTypeClusterIP    ServiceType = "ClusterIP"
	ServiceTypeNodePort     ServiceType = "NodePort"
	ServiceTypeLoadBalancer ServiceType = "LoadBalancer"
	ServiceClusterIP        = ServiceTypeClusterIP
)

// ServiceStatus 服务状态
type ServiceStatus string

const (
	ServiceStatusActive  ServiceStatus = "active"
	ServiceStatusPending ServiceStatus = "pending"
	ServiceStatusFailed  ServiceStatus = "failed"
)

// ServicePort 服务端口
type ServicePort struct {
	Name       string `json:"name"`
	Port       int    `json:"port"`
	TargetPort int    `json:"target_port"`
	NodePort   int    `json:"node_port,omitempty"`
	Protocol   string `json:"protocol"`
}

// ContainerPort 容器端口
type ContainerPort struct {
	Name          string `json:"name,omitempty"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol,omitempty"`
}

// EnvVar 环境变量
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ResourceRequirements 资源需求
type ResourceRequirements struct {
	Limits   ResourceList `json:"limits,omitempty"`
	Requests ResourceList `json:"requests,omitempty"`
}

// ResourceList 资源列表
type ResourceList struct {
	CPU    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// VolumeMount 卷挂载
type VolumeMount struct {
	Name      string `json:"name"`
	MountPath string `json:"mount_path"`
	ReadOnly  bool   `json:"read_only,omitempty"`
}

// HealthCheck 健康检查
type HealthCheck struct {
	Type     string `json:"type"`     // http, tcp, exec
	Path     string `json:"path,omitempty"`
	Port     int    `json:"port,omitempty"`
	Command  string `json:"command,omitempty"`
	Interval int    `json:"interval_seconds,omitempty"`
	Timeout  int    `json:"timeout_seconds,omitempty"`
}

// ClusterStats 集群统计
type ClusterStats struct {
	mu               sync.RWMutex `json:"-"`
	TotalPods        int          `json:"total_pods"`
	PendingPods      int          `json:"pending_pods"`
	RunningPods      int          `json:"running_pods"`
	SucceededPods    int          `json:"succeeded_pods"`
	FailedPods       int          `json:"failed_pods"`
	TotalServices    int          `json:"total_services"`
	TotalDeployments int          `json:"total_deployments"`
	TotalContainers  int          `json:"total_containers"`
	LastUpdated      time.Time    `json:"last_updated"`
}

// GetSnapshot 获取统计快照
func (s *ClusterStats) GetSnapshot() *ClusterStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return &ClusterStats{
		TotalPods:        s.TotalPods,
		PendingPods:      s.PendingPods,
		RunningPods:      s.RunningPods,
		SucceededPods:    s.SucceededPods,
		FailedPods:       s.FailedPods,
		TotalServices:    s.TotalServices,
		TotalDeployments: s.TotalDeployments,
		TotalContainers:  s.TotalContainers,
		LastUpdated:      s.LastUpdated,
	}
}

// ==================== 类型转换函数 ====================

// podSpecToDeploymentTemplate 将 PodSpec 转换为 DeploymentTemplate
func podSpecToDeploymentTemplate(spec PodSpec) DeploymentTemplate {
	containers := make([]ContainerSpec, len(spec.Containers))
	copy(containers, spec.Containers)
	return DeploymentTemplate{
		Spec: PodSpecData{
			Containers: containers,
		},
	}
}

// deploymentSpecToData 将 DeploymentSpec 转换为 DeploymentSpecData
func deploymentSpecToData(spec DeploymentSpec) DeploymentSpecData {
	return DeploymentSpecData{
		Replicas: spec.Replicas,
		Template: DeploymentTemplate{
			Metadata: spec.Template.Metadata,
			Spec:     spec.Template.Spec,
		},
	}
}

// podSpecDataToPodSpec 将 PodSpecData 转换为 PodSpec
func podSpecDataToPodSpec(data PodSpecData) PodSpec {
	containers := make([]ContainerSpec, len(data.Containers))
	copy(containers, data.Containers)
	return PodSpec{
		Containers: containers,
	}
}

// serviceSpecToData 将 ServiceSpec 转换为 ServiceSpecData
func serviceSpecToData(spec ServiceSpec) ServiceSpecData {
	ports := make([]ServicePort, len(spec.Ports))
	copy(ports, spec.Ports)
	selector := make(map[string]string)
	for k, v := range spec.Selector {
		selector[k] = v
	}
	return ServiceSpecData{
		Type:      spec.Type,
		ClusterIP: spec.ClusterIP,
		Ports:     ports,
		Selector:  selector,
	}
}
