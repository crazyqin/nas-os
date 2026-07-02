// Package k3s 提供 K3s 容器编排核心业务逻辑
package k3s

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager K3s 集群管理器.
type Manager struct {
	cluster     *ClusterInfo
	nodes       map[string]*NodeInfo
	releases    map[string]*HelmRelease
	deployments map[string]*DeploymentInfo
	services    map[string]*ServiceInfo
	pods        map[string]*PodInfo
	hpas        map[string]*HPAConfig
	quotas      map[string]*ResourceQuota
	events      []*ClusterEvent
	meshConfig  *ServiceMeshConfig
	mu          sync.RWMutex
}

// NewManager 创建 K3s 管理器.
func NewManager() *Manager {
	return &Manager{
		cluster: &ClusterInfo{
			Name:        "nas-os-cluster",
			Version:     "v1.31.4+k3s1",
			Status:      ClusterStatusRunning,
			APIEndpoint: "https://127.0.0.1:6443",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		nodes:       make(map[string]*NodeInfo),
		releases:    make(map[string]*HelmRelease),
		deployments: make(map[string]*DeploymentInfo),
		services:    make(map[string]*ServiceInfo),
		pods:        make(map[string]*PodInfo),
		hpas:        make(map[string]*HPAConfig),
		quotas:      make(map[string]*ResourceQuota),
		events:      make([]*ClusterEvent, 0),
		meshConfig: &ServiceMeshConfig{
			Enabled:   false,
			Type:      ServiceMeshNone,
			UpdatedAt: time.Now(),
		},
	}
}

// ========== 集群管理 ==========

// GetClusterInfo 获取集群信息.
func (m *Manager) GetClusterInfo() *ClusterInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info := *m.cluster
	info.NodeCount = len(m.nodes)
	info.PodCount = len(m.pods)
	info.UpdatedAt = time.Now()
	return &info
}

// GetClusterHealth 获取集群健康状态.
func (m *Manager) GetClusterHealth() *ClusterHealth {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health := &ClusterHealth{
		Components: []ComponentHealth{
			{Name: "apiserver", Status: "healthy", Message: "API 服务正常"},
			{Name: "etcd", Status: "healthy", Message: "数据存储正常"},
			{Name: "scheduler", Status: "healthy", Message: "调度器正常"},
			{Name: "controller-manager", Status: "healthy", Message: "控制器管理器正常"},
			{Name: "kube-proxy", Status: "healthy", Message: "网络代理正常"},
		},
		CheckedAt: time.Now(),
	}

	// 检查节点健康
	warningCount := 0
	for _, node := range m.nodes {
		nh := NodeHealth{
			Name:  node.Name,
			Ready: node.Status == NodeStatusReady,
		}
		for _, cond := range node.Conditions {
			switch cond.Type {
			case "DiskPressure":
				nh.DiskPressure = cond.Status == "True"
			case "MemoryPressure":
				nh.MemoryPressure = cond.Status == "True"
			case "PIDPressure":
				nh.PIDPressure = cond.Status == "True"
			}
		}
		health.Nodes = append(health.Nodes, nh)
		if !nh.Ready || nh.DiskPressure || nh.MemoryPressure {
			warningCount++
		}
	}

	// 整体状态判定
	if warningCount == 0 {
		health.Status = "healthy"
	} else if warningCount < len(m.nodes) {
		health.Status = "warning"
	} else {
		health.Status = "critical"
	}

	return health
}

// ========== 节点管理 ==========

// AddNode 添加节点.
func (m *Manager) AddNode(info *NodeInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	info.CreatedAt = time.Now()
	info.UpdatedAt = time.Now()
	m.nodes[info.Name] = info

	m.addEvent("Normal", "Node", info.Name, "default", "NodeAdded",
		fmt.Sprintf("节点 %s 加入集群", info.Name))
}

// GetNode 获取节点信息.
func (m *Manager) GetNode(name string) (*NodeInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[name]
	if !ok {
		return nil, ErrNodeNotFound
	}
	return node, nil
}

// ListNodes 列出所有节点.
func (m *Manager) ListNodes() []*NodeInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*NodeInfo, 0, len(m.nodes))
	for _, n := range m.nodes {
		nodes = append(nodes, n)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})
	return nodes
}

// UpdateNodeStatus 更新节点状态.
func (m *Manager) UpdateNodeStatus(name string, status NodeStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[name]
	if !ok {
		return ErrNodeNotFound
	}

	oldStatus := node.Status
	node.Status = status
	node.UpdatedAt = time.Now()

	if oldStatus != status {
		severity := "Normal"
		if status == NodeStatusNotReady {
			severity = "Warning"
		}
		m.addEvent(severity, "Node", name, "default", "NodeStatusChanged",
			fmt.Sprintf("节点 %s 状态从 %s 变为 %s", name, oldStatus, status))
	}
	return nil
}

// RemoveNode 移除节点.
func (m *Manager) RemoveNode(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nodes[name]; !ok {
		return ErrNodeNotFound
	}
	delete(m.nodes, name)

	m.addEvent("Normal", "Node", name, "default", "NodeRemoved",
		fmt.Sprintf("节点 %s 已从集群移除", name))
	return nil
}

// ========== Helm Chart 管理 ==========

// DeployChart 部署 Helm Chart.
func (m *Manager) DeployChart(req DeployChartRequest) (*HelmRelease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在同名 Release
	key := req.Namespace + "/" + req.Name
	if _, exists := m.releases[key]; exists {
		return nil, ErrHelmReleaseExists
	}

	now := time.Now()
	release := &HelmRelease{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Namespace:   req.Namespace,
		Chart:       req.Chart,
		ChartVer:    req.Version,
		Status:      HelmStatusDeployed,
		Revision:    1,
		Values:      req.Values,
		Description: req.Description,
		DeployedAt:  now,
		UpdatedAt:   now,
	}

	if release.Values == nil {
		release.Values = make(map[string]interface{})
	}

	m.releases[key] = release

	m.addEvent("Normal", "HelmRelease", req.Name, req.Namespace, "Deployed",
		fmt.Sprintf("Chart %s 已部署到 %s 命名空间", req.Chart, req.Namespace))

	return release, nil
}

// GetHelmRelease 获取 Helm Release.
func (m *Manager) GetHelmRelease(namespace, name string) (*HelmRelease, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := namespace + "/" + name
	release, ok := m.releases[key]
	if !ok {
		return nil, ErrHelmReleaseNotFound
	}
	return release, nil
}

// ListHelmReleases 列出所有 Helm Release.
func (m *Manager) ListHelmReleases(namespace string) []*HelmRelease {
	m.mu.RLock()
	defer m.mu.RUnlock()

	releases := make([]*HelmRelease, 0)
	for _, r := range m.releases {
		if namespace == "" || r.Namespace == namespace {
			releases = append(releases, r)
		}
	}
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].DeployedAt.After(releases[j].DeployedAt)
	})
	return releases
}

// UpgradeHelmRelease 升级 Helm Release.
func (m *Manager) UpgradeHelmRelease(namespace, name string, req UpgradeChartRequest) (*HelmRelease, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := namespace + "/" + name
	release, ok := m.releases[key]
	if !ok {
		return nil, ErrHelmReleaseNotFound
	}

	if req.Version != "" {
		release.ChartVer = req.Version
	}
	if !req.ResetValues && req.Values != nil {
		for k, v := range req.Values {
			release.Values[k] = v
		}
	} else if req.ResetValues && req.Values != nil {
		release.Values = req.Values
	}
	if req.Description != "" {
		release.Description = req.Description
	}

	release.Revision++
	release.Status = HelmStatusDeployed
	release.UpdatedAt = time.Now()

	m.addEvent("Normal", "HelmRelease", name, namespace, "Upgraded",
		fmt.Sprintf("Release %s 升级到修订版本 %d", name, release.Revision))

	return release, nil
}

// RollbackHelmRelease 回滚 Helm Release.
func (m *Manager) RollbackHelmRelease(namespace, name string, req RollbackChartRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := namespace + "/" + name
	release, ok := m.releases[key]
	if !ok {
		return ErrHelmReleaseNotFound
	}

	if req.Revision <= 0 || req.Revision >= release.Revision {
		return ErrRollbackFailed
	}

	release.Revision++
	release.Status = HelmStatusDeployed
	release.UpdatedAt = time.Now()
	release.Description = fmt.Sprintf("回滚到修订版本 %d", req.Revision)

	m.addEvent("Normal", "HelmRelease", name, namespace, "RolledBack",
		fmt.Sprintf("Release %s 回滚到修订版本 %d (新修订版本: %d)", name, req.Revision, release.Revision))

	return nil
}

// UninstallHelmRelease 卸载 Helm Release.
func (m *Manager) UninstallHelmRelease(namespace, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := namespace + "/" + name
	if _, ok := m.releases[key]; !ok {
		return ErrHelmReleaseNotFound
	}

	delete(m.releases, key)

	m.addEvent("Normal", "HelmRelease", name, namespace, "Uninstalled",
		fmt.Sprintf("Release %s 已从 %s 卸载", name, namespace))

	return nil
}

// ========== 工作负载管理 ==========

// AddDeployment 添加 Deployment 信息.
func (m *Manager) AddDeployment(dep *DeploymentInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	dep.CreatedAt = time.Now()
	m.deployments[dep.Namespace+"/"+dep.Name] = dep
}

// ListDeployments 列出 Deployment.
func (m *Manager) ListDeployments(namespace string) []*DeploymentInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	deps := make([]*DeploymentInfo, 0)
	for _, d := range m.deployments {
		if namespace == "" || d.Namespace == namespace {
			deps = append(deps, d)
		}
	}
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Name < deps[j].Name
	})
	return deps
}

// GetDeployment 获取 Deployment.
func (m *Manager) GetDeployment(namespace, name string) (*DeploymentInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := namespace + "/" + name
	dep, ok := m.deployments[key]
	if !ok {
		return nil, ErrWorkloadNotFound
	}
	return dep, nil
}

// AddService 添加 Service 信息.
func (m *Manager) AddService(svc *ServiceInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	svc.CreatedAt = time.Now()
	m.services[svc.Namespace+"/"+svc.Name] = svc
}

// ListServices 列出 Service.
func (m *Manager) ListServices(namespace string) []*ServiceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	svcs := make([]*ServiceInfo, 0)
	for _, s := range m.services {
		if namespace == "" || s.Namespace == namespace {
			svcs = append(svcs, s)
		}
	}
	sort.Slice(svcs, func(i, j int) bool {
		return svcs[i].Name < svcs[j].Name
	})
	return svcs
}

// GetService 获取 Service.
func (m *Manager) GetService(namespace, name string) (*ServiceInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := namespace + "/" + name
	svc, ok := m.services[key]
	if !ok {
		return nil, ErrWorkloadNotFound
	}
	return svc, nil
}

// AddPod 添加 Pod 信息.
func (m *Manager) AddPod(pod *PodInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pod.CreatedAt = time.Now()
	m.pods[pod.Namespace+"/"+pod.Name] = pod
}

// ListPods 列出 Pod.
func (m *Manager) ListPods(namespace string) []*PodInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pods := make([]*PodInfo, 0)
	for _, p := range m.pods {
		if namespace == "" || p.Namespace == namespace {
			pods = append(pods, p)
		}
	}
	sort.Slice(pods, func(i, j int) bool {
		return pods[i].Name < pods[j].Name
	})
	return pods
}

// GetPod 获取 Pod.
func (m *Manager) GetPod(namespace, name string) (*PodInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := namespace + "/" + name
	pod, ok := m.pods[key]
	if !ok {
		return nil, ErrPodNotFound
	}
	return pod, nil
}

// GetPodLogs 获取 Pod 日志.
func (m *Manager) GetPodLogs(req PodLogRequest) (*PodLogResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := req.Namespace + "/" + req.PodName
	pod, ok := m.pods[key]
	if !ok {
		return nil, ErrPodNotFound
	}

	// 模拟日志输出
	container := req.Container
	if container == "" && len(pod.Containers) > 0 {
		container = pod.Containers[0].Name
	}

	tailLines := req.TailLines
	if tailLines <= 0 {
		tailLines = 100
	}

	lines := make([]string, 0, tailLines)
	for i := 0; i < tailLines; i++ {
		lines = append(lines, fmt.Sprintf("[%s] %s: 日志行 %d - 系统运行正常",
			time.Now().Add(-time.Duration(tailLines-i)*time.Second).Format(time.RFC3339),
			container, i+1))
	}

	return &PodLogResult{
		PodName:   req.PodName,
		Container: container,
		Lines:     lines,
	}, nil
}

// ========== 服务网格管理 ==========

// GetServiceMeshConfig 获取服务网格配置.
func (m *Manager) GetServiceMeshConfig() *ServiceMeshConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.meshConfig
	return &cfg
}

// EnableServiceMesh 启用服务网格.
func (m *Manager) EnableServiceMesh(meshType ServiceMeshType, namespace string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.meshConfig.Enabled = true
	m.meshConfig.Type = meshType
	m.meshConfig.Namespace = namespace
	m.meshConfig.Version = "latest"
	m.meshConfig.UpdatedAt = time.Now()

	m.addEvent("Normal", "ServiceMesh", string(meshType), namespace, "Enabled",
		fmt.Sprintf("已启用 %s 服务网格", meshType))
	return nil
}

// DisableServiceMesh 禁用服务网格.
func (m *Manager) DisableServiceMesh() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	meshType := m.meshConfig.Type
	m.meshConfig.Enabled = false
	m.meshConfig.Type = ServiceMeshNone
	m.meshConfig.UpdatedAt = time.Now()

	m.addEvent("Normal", "ServiceMesh", string(meshType), "", "Disabled",
		fmt.Sprintf("已禁用 %s 服务网格", meshType))
	return nil
}

// UpdateServiceMeshConfig 更新服务网格配置.
func (m *Manager) UpdateServiceMeshConfig(cfg ServiceMeshConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.meshConfig.Enabled {
		return ErrServiceMeshNotEnabled
	}

	if cfg.MTLS {
		m.meshConfig.MTLS = cfg.MTLS
	}
	if cfg.Tracing {
		m.meshConfig.Tracing = cfg.Tracing
		m.meshConfig.TracingURL = cfg.TracingURL
	}
	if cfg.AccessLog {
		m.meshConfig.AccessLog = cfg.AccessLog
	}
	m.meshConfig.UpdatedAt = time.Now()

	return nil
}

// ========== HPA 自动扩缩容管理 ==========

// CreateHPA 创建 HPA 配置.
func (m *Manager) CreateHPA(req CreateHPARequest) *HPAConfig {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	hpa := &HPAConfig{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Namespace:   req.Namespace,
		TargetKind:  req.TargetKind,
		TargetName:  req.TargetName,
		MinReplicas: req.MinReplicas,
		MaxReplicas: req.MaxReplicas,
		Metrics:     req.Metrics,
		Behavior:    req.Behavior,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 默认值
	if hpa.TargetKind == "" {
		hpa.TargetKind = "Deployment"
	}
	if hpa.MinReplicas <= 0 {
		hpa.MinReplicas = 1
	}
	if len(hpa.Metrics) == 0 {
		hpa.Metrics = []HPAMetric{
			{Type: "Resource", Resource: "cpu", Target: "Utilization", Value: 80},
		}
	}

	m.hpas[req.Namespace+"/"+req.Name] = hpa

	m.addEvent("Normal", "HPA", req.Name, req.Namespace, "Created",
		fmt.Sprintf("HPA %s 已创建，副本范围 %d-%d", req.Name, hpa.MinReplicas, hpa.MaxReplicas))

	return hpa
}

// GetHPA 获取 HPA 配置.
func (m *Manager) GetHPA(namespace, name string) (*HPAConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := namespace + "/" + name
	hpa, ok := m.hpas[key]
	if !ok {
		return nil, ErrHPANotFound
	}
	return hpa, nil
}

// ListHPAs 列出 HPA 配置.
func (m *Manager) ListHPAs(namespace string) []*HPAConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hpas := make([]*HPAConfig, 0)
	for _, h := range m.hpas {
		if namespace == "" || h.Namespace == namespace {
			hpas = append(hpas, h)
		}
	}
	sort.Slice(hpas, func(i, j int) bool {
		return hpas[i].Name < hpas[j].Name
	})
	return hpas
}

// UpdateHPA 更新 HPA 配置.
func (m *Manager) UpdateHPA(namespace, name string, req UpdateHPARequest) (*HPAConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := namespace + "/" + name
	hpa, ok := m.hpas[key]
	if !ok {
		return nil, ErrHPANotFound
	}

	if req.MinReplicas != nil {
		hpa.MinReplicas = *req.MinReplicas
	}
	if req.MaxReplicas != nil {
		hpa.MaxReplicas = *req.MaxReplicas
	}
	if req.Metrics != nil {
		hpa.Metrics = req.Metrics
	}
	if req.Behavior != nil {
		hpa.Behavior = req.Behavior
	}
	hpa.UpdatedAt = time.Now()

	return hpa, nil
}

// DeleteHPA 删除 HPA 配置.
func (m *Manager) DeleteHPA(namespace, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := namespace + "/" + name
	if _, ok := m.hpas[key]; !ok {
		return ErrHPANotFound
	}
	delete(m.hpas, key)

	m.addEvent("Normal", "HPA", name, namespace, "Deleted",
		fmt.Sprintf("HPA %s 已删除", name))
	return nil
}

// ========== 应用商店集成 ==========

// DeployFromAppStore 从应用商店部署应用
// 对接 appstore 模块获取 Chart 信息，然后通过 Helm 部署.
func (m *Manager) DeployFromAppStore(req AppStoreDeployRequest) (*HelmRelease, error) {
	// 查询应用商店获取 Chart 信息
	app := m.getAppStoreApp(req.AppID)
	if app == nil {
		return nil, fmt.Errorf("应用商店中未找到应用 %s", req.AppID)
	}

	releaseName := req.ReleaseName
	if releaseName == "" {
		releaseName = strings.ReplaceAll(req.AppID, "_", "-")
	}

	chartRef := app.ChartRepo + "/" + app.ChartName
	deployReq := DeployChartRequest{
		Name:      releaseName,
		Namespace: req.Namespace,
		Chart:     chartRef,
		Version:   app.Version,
		Values:    req.Values,
		Wait:      req.Wait,
	}

	return m.DeployChart(deployReq)
}

// ListAppStoreApps 列出可部署的应用商店应用.
func (m *Manager) ListAppStoreApps() []*AppStoreApp {
	// 返回预置的可部署应用列表
	return []*AppStoreApp{
		{ID: "plex", Name: "Plex", Description: "媒体服务器", Category: "media", Version: "1.40.0", ChartRepo: "stable", ChartName: "plex"},
		{ID: "nextcloud", Name: "Nextcloud", Description: "私有云盘", Category: "storage", Version: "28.0.0", ChartRepo: "stable", ChartName: "nextcloud"},
		{ID: "homeassistant", Name: "Home Assistant", Description: "智能家居", Category: "iot", Version: "2024.1", ChartRepo: "stable", ChartName: "home-assistant"},
		{ID: "grafana", Name: "Grafana", Description: "可视化面板", Category: "monitoring", Version: "10.3.0", ChartRepo: "stable", ChartName: "grafana"},
		{ID: "prometheus", Name: "Prometheus", Description: "监控系统", Category: "monitoring", Version: "2.49.0", ChartRepo: "stable", ChartName: "prometheus"},
		{ID: "traefik", Name: "Traefik", Description: "反向代理", Category: "networking", Version: "2.11.0", ChartRepo: "stable", ChartName: "traefik"},
	}
}

// getAppStoreApp 从应用商店获取应用信息.
func (m *Manager) getAppStoreApp(appID string) *AppStoreApp {
	apps := m.ListAppStoreApps()
	for _, app := range apps {
		if app.ID == appID {
			// 标记已安装
			for _, rel := range m.releases {
				if strings.Contains(rel.Chart, app.ChartName) {
					app.Installed = true
					break
				}
			}
			return app
		}
	}
	return nil
}

// ========== 资源配额管理 ==========

// CreateQuota 创建资源配额.
func (m *Manager) CreateQuota(req CreateQuotaRequest) *ResourceQuota {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	quota := &ResourceQuota{
		ID:        uuid.New().String(),
		Namespace: req.Namespace,
		Name:      req.Name,
		Hard:      req.Hard,
		Used:      make(map[string]string),
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 初始化使用量为 0
	for k := range req.Hard {
		quota.Used[k] = "0"
	}

	m.quotas[req.Namespace+"/"+req.Name] = quota

	m.addEvent("Normal", "ResourceQuota", req.Name, req.Namespace, "Created",
		fmt.Sprintf("资源配额 %s 已创建", req.Name))

	return quota
}

// GetQuota 获取资源配额.
func (m *Manager) GetQuota(namespace, name string) (*ResourceQuota, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := namespace + "/" + name
	quota, ok := m.quotas[key]
	if !ok {
		return nil, ErrQuotaNotFound
	}
	return quota, nil
}

// ListQuotas 列出资源配额.
func (m *Manager) ListQuotas(namespace string) []*ResourceQuota {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quotas := make([]*ResourceQuota, 0)
	for _, q := range m.quotas {
		if namespace == "" || q.Namespace == namespace {
			quotas = append(quotas, q)
		}
	}
	sort.Slice(quotas, func(i, j int) bool {
		return quotas[i].Name < quotas[j].Name
	})
	return quotas
}

// UpdateQuota 更新资源配额.
func (m *Manager) UpdateQuota(namespace, name string, req UpdateQuotaRequest) (*ResourceQuota, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := namespace + "/" + name
	quota, ok := m.quotas[key]
	if !ok {
		return nil, ErrQuotaNotFound
	}

	quota.Hard = req.Hard
	quota.UpdatedAt = time.Now()

	return quota, nil
}

// DeleteQuota 删除资源配额.
func (m *Manager) DeleteQuota(namespace, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := namespace + "/" + name
	if _, ok := m.quotas[key]; !ok {
		return ErrQuotaNotFound
	}
	delete(m.quotas, key)

	m.addEvent("Normal", "ResourceQuota", name, namespace, "Deleted",
		fmt.Sprintf("资源配额 %s 已删除", name))
	return nil
}

// ========== 集群事件监控 ==========

// ListEvents 列出集群事件.
func (m *Manager) ListEvents(namespace string, limit int) []*ClusterEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]*ClusterEvent, 0)
	for _, e := range m.events {
		if namespace == "" || e.Namespace == namespace {
			events = append(events, e)
		}
	}

	// 按时间倒序
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTime.After(events[j].LastTime)
	})

	if limit > 0 && limit < len(events) {
		events = events[:limit]
	}
	return events
}

// GetEventsBySeverity 按严重级别过滤事件.
func (m *Manager) GetEventsBySeverity(severity EventSeverity) []*ClusterEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]*ClusterEvent, 0)
	for _, e := range m.events {
		if e.Severity == severity {
			events = append(events, e)
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].LastTime.After(events[j].LastTime)
	})
	return events
}

// ClearEvents 清除事件.
func (m *Manager) ClearEvents(namespace string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	before := len(m.events)
	if namespace == "" {
		m.events = make([]*ClusterEvent, 0)
		return before
	}

	filtered := make([]*ClusterEvent, 0)
	for _, e := range m.events {
		if e.Namespace != namespace {
			filtered = append(filtered, e)
		}
	}
	m.events = filtered
	return before - len(filtered)
}

// addEvent 添加集群事件（内部方法，调用前需持有写锁）.
func (m *Manager) addEvent(severity, kind, name, namespace, reason, message string) {
	event := &ClusterEvent{
		ID:        uuid.New().String(),
		Namespace: namespace,
		Kind:      kind,
		Name:      name,
		Reason:    reason,
		Message:   message,
		Severity:  EventSeverity(severity),
		Source:    "k3s-manager",
		Count:     1,
		FirstTime: time.Now(),
		LastTime:  time.Now(),
	}

	// 检查是否已有相同事件，如果有则增加计数
	for _, e := range m.events {
		if e.Kind == kind && e.Name == name && e.Namespace == namespace && e.Reason == reason {
			e.Count++
			e.LastTime = time.Now()
			return
		}
	}

	m.events = append(m.events, event)

	// 保留最近 1000 条事件
	if len(m.events) > 1000 {
		m.events = m.events[len(m.events)-1000:]
	}
}
