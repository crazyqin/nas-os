// Package containerorch 提供 K3s 轻量级容器编排功能
package containerorch

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager 容器编排管理器.
type Manager struct {
	mu          sync.RWMutex
	containers  map[string]*Container  // id -> container
	pods        map[string]*Pod        // id -> pod
	deployments map[string]*Deployment // id -> deployment
	services    map[string]*Service    // id -> service
	stats       ClusterStats
	scheduler   *Scheduler
	healthCheck *HealthChecker
	nodeID      string // 当前节点 ID
}

// NewManager 创建容器编排管理器.
func NewManager(nodeID string) *Manager {
	m := &Manager{
		containers:  make(map[string]*Container),
		pods:        make(map[string]*Pod),
		deployments: make(map[string]*Deployment),
		services:    make(map[string]*Service),
		nodeID:      nodeID,
	}

	// 初始化调度器
	m.scheduler = NewScheduler(m)

	// 初始化健康检查器
	m.healthCheck = NewHealthChecker(m)

	return m
}

// Start 启动管理器.
func (m *Manager) Start(ctx context.Context) error {
	log.Printf("[ContainerOrch] 启动容器编排管理器，节点: %s", m.nodeID)

	// 启动健康检查
	go m.healthCheck.Start(ctx)

	// 启动调度器
	go m.scheduler.Start(ctx)

	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	log.Printf("[ContainerOrch] 停止容器编排管理器")
	m.healthCheck.Stop()
	m.scheduler.Stop()
}

// ==================== 容器生命周期管理 ====================

// CreateContainer 创建容器.
func (m *Manager) CreateContainer(req *CreateContainerRequest) (*Container, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成容器 ID
	containerID := generateID("container")

	// 验证镜像
	if req.Image == "" {
		return nil, fmt.Errorf("image is required")
	}

	// 创建容器
	container := &Container{
		ID:            containerID,
		Name:          req.Name,
		PodID:         req.PodID,
		Image:         req.Image,
		Command:       req.Command,
		Args:          req.Args,
		WorkingDir:    req.WorkingDir,
		State:         StateCreated,
		Status:        "Container created",
		Resources:     req.Resources,
		Ports:         req.Ports,
		Volumes:       req.Volumes,
		Env:           req.Env,
		LivenessProbe: req.LivenessProbe,
		ReadinessProbe: req.ReadinessProbe,
		StartupProbe:  req.StartupProbe,
		RestartPolicy: req.RestartPolicy,
		CreatedAt:     time.Now(),
		LogPath:       fmt.Sprintf("/var/log/container/%s.log", containerID),
	}

	// 存储容器
	m.containers[containerID] = container

	log.Printf("[ContainerOrch] 容器已创建: %s (%s)", container.Name, containerID)
	return container, nil
}

// StartContainer 启动容器.
func (m *Manager) StartContainer(containerID string) error {
	m.mu.Lock()
	container, ok := m.containers[containerID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("container not found: %s", containerID)
	}

	if container.State == StateRunning {
		m.mu.Unlock()
		return fmt.Errorf("container is already running: %s", containerID)
	}
	m.mu.Unlock()

	// 模拟启动容器
	container.mu.Lock()
	container.State = StateRunning
	container.Status = "Container started"
	now := time.Now()
	container.StartedAt = &now
	container.mu.Unlock()

	log.Printf("[ContainerOrch] 容器已启动: %s (%s)", container.Name, containerID)
	return nil
}

// StopContainer 停止容器.
func (m *Manager) StopContainer(containerID string, timeout *int) error {
	m.mu.Lock()
	container, ok := m.containers[containerID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("container not found: %s", containerID)
	}

	if container.State != StateRunning {
		m.mu.Unlock()
		return fmt.Errorf("container is not running: %s", containerID)
	}
	m.mu.Unlock()

	// 模拟停止容器
	container.mu.Lock()
	container.State = StateStopped
	container.Status = "Container stopped"
	now := time.Now()
	container.FinishedAt = &now
	container.mu.Unlock()

	log.Printf("[ContainerOrch] 容器已停止: %s (%s)", container.Name, containerID)
	return nil
}

// RestartContainer 重启容器.
func (m *Manager) RestartContainer(containerID string, timeout *int) error {
	// 停止容器
	if err := m.StopContainer(containerID, timeout); err != nil {
		// 如果容器未运行，忽略错误
		if container, ok := m.containers[containerID]; ok && container.State == StateStopped {
			// 继续启动
		} else {
			return err
		}
	}

	// 启动容器
	if err := m.StartContainer(containerID); err != nil {
		return err
	}

	// 增加重启计数
	m.mu.Lock()
	if container, ok := m.containers[containerID]; ok {
		container.RestartCount++
	}
	m.mu.Unlock()

	return nil
}

// RemoveContainer 删除容器.
func (m *Manager) RemoveContainer(containerID string, force bool) error {
	m.mu.Lock()
	container, ok := m.containers[containerID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("container not found: %s", containerID)
	}

	// 如果容器正在运行且未强制删除，报错
	if container.State == StateRunning && !force {
		m.mu.Unlock()
		return fmt.Errorf("container is running, use force to remove: %s", containerID)
	}
	m.mu.Unlock()

	// 如果正在运行，先停止
	if container.State == StateRunning {
		if err := m.StopContainer(containerID, nil); err != nil && !force {
			return err
		}
	}

	// 删除容器
	m.mu.Lock()
	delete(m.containers, containerID)
	m.mu.Unlock()

	log.Printf("[ContainerOrch] 容器已删除: %s (%s)", container.Name, containerID)
	return nil
}

// GetContainer 获取容器.
func (m *Manager) GetContainer(containerID string) (*Container, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	container, ok := m.containers[containerID]
	return container, ok
}

// ListContainers 列出所有容器.
func (m *Manager) ListContainers() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	containers := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		containers = append(containers, c)
	}
	return containers
}

// GetContainerLogs 获取容器日志.
func (m *Manager) GetContainerLogs(containerID string, opts *LogOptions) (string, error) {
	m.mu.RLock()
	container, ok := m.containers[containerID]
	m.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("container not found: %s", containerID)
	}

	// 模拟返回日志
	log.Printf("[ContainerOrch] 获取容器日志: %s", containerID)
	return fmt.Sprintf("[%s] Container %s logs\n", time.Now().Format(time.RFC3339), container.Name), nil
}

// ==================== Pod 管理 ====================

// CreatePod 创建 Pod.
func (m *Manager) CreatePod(req *CreatePodRequest) (*Pod, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成 Pod ID
	podID := generateID("pod")

	// 创建 Pod
	pod := &Pod{
		ID:           podID,
		Name:         req.Name,
		Namespace:    req.Namespace,
		DeploymentID: req.DeploymentID,
		Spec:         req.Spec,
		Phase:        PodPending,
		Labels:       req.Labels,
		Annotations:  req.Annotations,
		Containers:   make([]*Container, 0),
		CreatedAt:    time.Now(),
	}

	// 设置默认命名空间
	if pod.Namespace == "" {
		pod.Namespace = "default"
	}

	// 存储 Pod
	m.pods[podID] = pod

	// 更新统计
	m.stats.mu.Lock()
	m.stats.TotalPods++
	m.stats.PendingPods++
	m.stats.LastUpdated = time.Now()
	m.stats.mu.Unlock()

	log.Printf("[ContainerOrch] Pod 已创建: %s/%s (%s)", pod.Namespace, pod.Name, podID)
	return pod, nil
}

// StartPod 启动 Pod.
func (m *Manager) StartPod(podID string) error {
	m.mu.Lock()
	pod, ok := m.pods[podID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("pod not found: %s", podID)
	}

	if pod.Phase == PodRunning {
		m.mu.Unlock()
		return fmt.Errorf("pod is already running: %s", podID)
	}
	m.mu.Unlock()

	// 调度 Pod 到节点
	nodeID, err := m.scheduler.SchedulePod(pod)
	if err != nil {
		return fmt.Errorf("failed to schedule pod: %w", err)
	}

	// 启动 Pod 中的所有容器
	pod.mu.Lock()
	pod.NodeID = nodeID
	pod.Phase = PodRunning
	pod.HostIP = fmt.Sprintf("192.168.1.%d", hashString(nodeID)%254+1)
	pod.PodIP = fmt.Sprintf("10.42.%d.%d", hashString(podID)%254+1, hashString(podID+"pod")%254+1)
	now := time.Now()
	pod.StartedAt = &now
	pod.mu.Unlock()

	// 启动所有容器
	for _, container := range pod.Containers {
		if err := m.StartContainer(container.ID); err != nil {
			log.Printf("[ContainerOrch] 启动容器失败: %s, 错误: %v", container.ID, err)
		}
	}

	// 更新统计
	m.stats.mu.Lock()
	m.stats.PendingPods--
	m.stats.RunningPods++
	m.stats.LastUpdated = time.Now()
	m.stats.mu.Unlock()

	log.Printf("[ContainerOrch] Pod 已启动: %s/%s (%s), 节点: %s", pod.Namespace, pod.Name, podID, nodeID)
	return nil
}

// StopPod 停止 Pod.
func (m *Manager) StopPod(podID string) error {
	m.mu.Lock()
	pod, ok := m.pods[podID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("pod not found: %s", podID)
	}

	if pod.Phase != PodRunning {
		m.mu.Unlock()
		return fmt.Errorf("pod is not running: %s", podID)
	}
	m.mu.Unlock()

	// 停止所有容器
	for _, container := range pod.Containers {
		if err := m.StopContainer(container.ID, nil); err != nil {
			log.Printf("[ContainerOrch] 停止容器失败: %s, 错误: %v", container.ID, err)
		}
	}

	// 更新 Pod 状态
	pod.mu.Lock()
	pod.Phase = PodSucceeded
	now := time.Now()
	pod.FinishedAt = &now
	pod.mu.Unlock()

	// 更新统计
	m.stats.mu.Lock()
	m.stats.RunningPods--
	m.stats.SucceededPods++
	m.stats.LastUpdated = time.Now()
	m.stats.mu.Unlock()

	log.Printf("[ContainerOrch] Pod 已停止: %s/%s (%s)", pod.Namespace, pod.Name, podID)
	return nil
}

// RemovePod 删除 Pod.
func (m *Manager) RemovePod(podID string, force bool) error {
	m.mu.Lock()
	pod, ok := m.pods[podID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("pod not found: %s", podID)
	}

	// 如果 Pod 正在运行且未强制删除，报错
	if pod.Phase == PodRunning && !force {
		m.mu.Unlock()
		return fmt.Errorf("pod is running, use force to remove: %s", podID)
	}
	m.mu.Unlock()

	// 如果正在运行，先停止
	if pod.Phase == PodRunning {
		if err := m.StopPod(podID); err != nil && !force {
			return err
		}
	}

	// 删除所有容器
	for _, container := range pod.Containers {
		if err := m.RemoveContainer(container.ID, force); err != nil {
			log.Printf("[ContainerOrch] 删除容器失败: %s, 错误: %v", container.ID, err)
		}
	}

	// 删除 Pod
	m.mu.Lock()
	delete(m.pods, podID)
	m.stats.mu.Lock()
	m.stats.TotalPods--
	m.stats.mu.Unlock()
	m.stats.LastUpdated = time.Now()
	m.mu.Unlock()

	log.Printf("[ContainerOrch] Pod 已删除: %s/%s (%s)", pod.Namespace, pod.Name, podID)
	return nil
}

// GetPod 获取 Pod.
func (m *Manager) GetPod(podID string) (*Pod, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pod, ok := m.pods[podID]
	return pod, ok
}

// ListPods 列出所有 Pod.
func (m *Manager) ListPods(namespace string) []*Pod {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pods := make([]*Pod, 0)
	for _, p := range m.pods {
		if namespace == "" || p.Namespace == namespace {
			pods = append(pods, p)
		}
	}
	return pods
}

// ==================== Deployment 管理 ====================

// CreateDeployment 创建 Deployment.
func (m *Manager) CreateDeployment(req *CreateDeploymentRequest) (*Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成 Deployment ID
	deploymentID := generateID("deploy")

	// 创建 Deployment
	deployment := &Deployment{
		ID:        deploymentID,
		Name:      req.Name,
		Namespace: req.Namespace,
		Spec:      req.Spec,
		Status: DeploymentStatus{
			Replicas: req.Spec.Replicas,
		},
		Labels:      req.Labels,
		Annotations: req.Annotations,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置默认命名空间
	if deployment.Namespace == "" {
		deployment.Namespace = "default"
	}

	// 设置默认副本数
	if deployment.Spec.Replicas <= 0 {
		deployment.Spec.Replicas = 1
	}

	// 存储 Deployment
	m.deployments[deploymentID] = deployment

	// 更新统计
	m.stats.mu.Lock()
	m.stats.TotalDeployments++
	m.stats.LastUpdated = time.Now()
	m.stats.mu.Unlock()

	log.Printf("[ContainerOrch] Deployment 已创建: %s/%s (%s)", deployment.Namespace, deployment.Name, deploymentID)
	return deployment, nil
}

// ScaleDeployment 扩缩容 Deployment.
func (m *Manager) ScaleDeployment(deploymentID string, replicas int) error {
	if replicas < 0 {
		return fmt.Errorf("replicas cannot be negative")
	}

	m.mu.Lock()
	deployment, ok := m.deployments[deploymentID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("deployment not found: %s", deploymentID)
	}

	oldReplicas := deployment.Spec.Replicas
	deployment.Spec.Replicas = replicas
	deployment.UpdatedAt = time.Now()
	m.mu.Unlock()

	log.Printf("[ContainerOrch] Deployment 扩缩容: %s, %d -> %d", deploymentID, oldReplicas, replicas)

	// 根据副本数调整 Pod
	go m.reconcileDeployment(deploymentID)

	return nil
}

// DeleteDeployment 删除 Deployment.
func (m *Manager) DeleteDeployment(deploymentID string) error {
	m.mu.Lock()
	deployment, ok := m.deployments[deploymentID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("deployment not found: %s", deploymentID)
	}
	m.mu.Unlock()

	// 删除所有关联的 Pod
	for _, pod := range m.pods {
		if pod.DeploymentID == deploymentID {
			if err := m.RemovePod(pod.ID, true); err != nil {
				log.Printf("[ContainerOrch] 删除 Pod 失败: %s, 错误: %v", pod.ID, err)
			}
		}
	}

	// 删除 Deployment
	m.mu.Lock()
	delete(m.deployments, deploymentID)
	m.stats.mu.Lock()
	m.stats.TotalDeployments--
	m.stats.mu.Unlock()
	m.stats.LastUpdated = time.Now()
	m.mu.Unlock()

	log.Printf("[ContainerOrch] Deployment 已删除: %s/%s (%s)", deployment.Namespace, deployment.Name, deploymentID)
	return nil
}

// GetDeployment 获取 Deployment.
func (m *Manager) GetDeployment(deploymentID string) (*Deployment, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	deployment, ok := m.deployments[deploymentID]
	return deployment, ok
}

// ListDeployments 列出所有 Deployment.
func (m *Manager) ListDeployments(namespace string) []*Deployment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	deployments := make([]*Deployment, 0)
	for _, d := range m.deployments {
		if namespace == "" || d.Namespace == namespace {
			deployments = append(deployments, d)
		}
	}
	return deployments
}

// reconcileDeployment 协调 Deployment 状态.
func (m *Manager) reconcileDeployment(deploymentID string) {
	m.mu.RLock()
	deployment, ok := m.deployments[deploymentID]
	m.mu.RUnlock()

	if !ok {
		return
	}

	// 获取当前关联的 Pod
	currentPods := make([]*Pod, 0)
	for _, pod := range m.pods {
		if pod.DeploymentID == deploymentID {
			currentPods = append(currentPods, pod)
		}
	}

	targetReplicas := deployment.Spec.Replicas
	currentReplicas := len(currentPods)

	if currentReplicas < targetReplicas {
		// 需要扩容
		for i := 0; i < targetReplicas-currentReplicas; i++ {
			podReq := &CreatePodRequest{
				Name:         fmt.Sprintf("%s-pod-%d", deployment.Name, currentReplicas+i+1),
				Namespace:    deployment.Namespace,
				DeploymentID: deploymentID,
				Spec:         deployment.Spec.Template.Spec,
				Labels:       deployment.Spec.Template.Metadata.Labels,
				Annotations:  deployment.Spec.Template.Metadata.Annotations,
			}

			pod, err := m.CreatePod(podReq)
			if err != nil {
				log.Printf("[ContainerOrch] 创建 Pod 失败: %v", err)
				continue
			}

			// 启动 Pod
			if err := m.StartPod(pod.ID); err != nil {
				log.Printf("[ContainerOrch] 启动 Pod 失败: %v", err)
			}
		}
	} else if currentReplicas > targetReplicas {
		// 需要缩容
		for i := 0; i < currentReplicas-targetReplicas; i++ {
			if i < len(currentPods) {
				if err := m.RemovePod(currentPods[i].ID, true); err != nil {
					log.Printf("[ContainerOrch] 删除 Pod 失败: %v", err)
				}
			}
		}
	}

	// 更新 Deployment 状态
	m.mu.Lock()
	if dep, ok := m.deployments[deploymentID]; ok {
		dep.Status.Replicas = targetReplicas
		dep.Status.ReadyReplicas = targetReplicas // 简化处理
		dep.Status.AvailableReplicas = targetReplicas
		dep.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
}

// ==================== Service 管理 ====================

// CreateService 创建 Service.
func (m *Manager) CreateService(req *CreateServiceRequest) (*Service, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 生成 Service ID
	serviceID := generateID("svc")

	// 创建 Service
	service := &Service{
		ID:        serviceID,
		Name:      req.Name,
		Namespace: req.Namespace,
		Spec:      req.Spec,
		Labels:    req.Labels,
		Annotations: req.Annotations,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 设置默认命名空间
	if service.Namespace == "" {
		service.Namespace = "default"
	}

	// 设置默认类型
	if service.Spec.Type == "" {
		service.Spec.Type = ServiceClusterIP
	}

	// 分配 ClusterIP
	if service.Spec.ClusterIP == "" {
		service.Spec.ClusterIP = fmt.Sprintf("10.43.%d.%d", hashString(serviceID)%254+1, hashString(serviceID+"svc")%254+1)
	}

	// 存储 Service
	m.services[serviceID] = service

	// 更新统计
	m.stats.mu.Lock()
	m.stats.TotalServices++
	m.stats.LastUpdated = time.Now()
	m.stats.mu.Unlock()

	log.Printf("[ContainerOrch] Service 已创建: %s/%s (%s), 类型: %s, ClusterIP: %s",
		service.Namespace, service.Name, serviceID, service.Spec.Type, service.Spec.ClusterIP)
	return service, nil
}

// DeleteService 删除 Service.
func (m *Manager) DeleteService(serviceID string) error {
	m.mu.Lock()
	service, ok := m.services[serviceID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("service not found: %s", serviceID)
	}

	delete(m.services, serviceID)
	m.stats.mu.Lock()
	m.stats.TotalServices--
	m.stats.mu.Unlock()
	m.stats.LastUpdated = time.Now()
	m.mu.Unlock()

	log.Printf("[ContainerOrch] Service 已删除: %s/%s (%s)", service.Namespace, service.Name, serviceID)
	return nil
}

// GetService 获取 Service.
func (m *Manager) GetService(serviceID string) (*Service, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	service, ok := m.services[serviceID]
	return service, ok
}

// ListServices 列出所有 Service.
func (m *Manager) ListServices(namespace string) []*Service {
	m.mu.RLock()
	defer m.mu.RUnlock()
	services := make([]*Service, 0)
	for _, s := range m.services {
		if namespace == "" || s.Namespace == namespace {
			services = append(services, s)
		}
	}
	return services
}

// ==================== 统计 ====================

// GetStats 获取集群统计.
func (m *Manager) GetStats() *ClusterStats {
	return m.stats.GetSnapshot()
}

// ==================== 辅助函数 ====================

// CreateContainerRequest 创建容器请求.
type CreateContainerRequest struct {
	Name           string               `json:"name"`
	PodID          string               `json:"podId"`
	Image          string               `json:"image"`
	Command        []string             `json:"command"`
	Args           []string             `json:"args"`
	WorkingDir     string               `json:"workingDir"`
	Resources      ResourceRequirements `json:"resources"`
	Ports          []ContainerPort      `json:"ports"`
	Volumes        []VolumeMount        `json:"volumes"`
	Env            []EnvVar             `json:"env"`
	LivenessProbe  *HealthCheck         `json:"livenessProbe,omitempty"`
	ReadinessProbe *HealthCheck         `json:"readinessProbe,omitempty"`
	StartupProbe   *HealthCheck         `json:"startupProbe,omitempty"`
	RestartPolicy  RestartPolicy        `json:"restartPolicy"`
}

// CreatePodRequest 创建 Pod 请求.
type CreatePodRequest struct {
	Name         string            `json:"name"`
	Namespace    string            `json:"namespace"`
	DeploymentID string            `json:"deploymentId"`
	Spec         PodSpec           `json:"spec"`
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
}

// CreateDeploymentRequest 创建 Deployment 请求.
type CreateDeploymentRequest struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Spec        DeploymentSpec    `json:"spec"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// CreateServiceRequest 创建 Service 请求.
type CreateServiceRequest struct {
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Spec        ServiceSpec       `json:"spec"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
}

// LogOptions 日志选项.
type LogOptions struct {
	Follow     bool  `json:"follow"`     // 是否跟踪
	Tail       int   `json:"tail"`       // 返回最后 N 行
	Timestamps bool  `json:"timestamps"` // 是否显示时间戳
	SinceTime  *time.Time `json:"sinceTime"` // 起始时间
}

// generateID 生成唯一 ID.
func generateID(prefix string) string {
	return fmt.Sprintf("%s-%d-%s", prefix, time.Now().UnixNano(), randomString(6))
}

// randomString 生成随机字符串.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}

// hashString 字符串哈希.
func hashString(s string) int {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}
