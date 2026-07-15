// Package deployorch 提供部署编排管理器
// 对标 Synology CMS (Central Management System) 和 TrueNAS 多节点部署
package deployorch

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// NodeRole 节点角色
type NodeRole string

const (
	RoleMaster  NodeRole = "master"
	RoleWorker  NodeRole = "worker"
	RoleStorage NodeRole = "storage"
	RoleEdge    NodeRole = "edge"
)

// NodeStatus 节点状态
type NodeStatus string

const (
	StatusOnline    NodeStatus = "online"
	StatusOffline   NodeStatus = "offline"
	StatusProvision NodeStatus = "provisioning"
	StatusDraining  NodeStatus = "draining"
	StatusFailed    NodeStatus = "failed"
)

// Node 节点信息
type Node struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Role          NodeRole          `json:"role"`
	Status        NodeStatus        `json:"status"`
	Address       string            `json:"address"`
	SSHPort       int               `json:"ssh_port"`
	OSType        string            `json:"os_type"`
	StorageTB     float64           `json:"storage_tb"`
	MemoryGB      int               `json:"memory_gb"`
	CPUCores      int               `json:"cpu_cores"`
	Labels        map[string]string `json:"labels,omitempty"`
	JoinedAt      time.Time         `json:"joined_at"`
	LastHeartbeat time.Time         `json:"last_heartbeat"`
}

// DeployTemplate 部署模板
type DeployTemplate struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Services     []*ServiceDef     `json:"services"`
	NodeSelector map[string]string `json:"node_selector,omitempty"`
	Variables    map[string]string `json:"variables,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// ServiceDef 服务定义
type ServiceDef struct {
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	Replicas    int               `json:"replicas"`
	Ports       []PortMapping     `json:"ports,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	Volumes     []VolumeMount     `json:"volumes,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
	HealthCheck *HealthCheck      `json:"health_check,omitempty"`
}

// PortMapping 端口映射
type PortMapping struct {
	Host      int    `json:"host"`
	Container int    `json:"container"`
	Protocol  string `json:"protocol"`
}

// VolumeMount 卷挂载
type VolumeMount struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Mode   string `json:"mode"`
}

// HealthCheck 健康检查
type HealthCheck struct {
	Type     string `json:"type"`
	Endpoint string `json:"endpoint,omitempty"`
	Interval int    `json:"interval_seconds"`
	Timeout  int    `json:"timeout_seconds"`
	Retries  int    `json:"retries"`
}

// Deployment 部署
type Deployment struct {
	ID         string             `json:"id"`
	TemplateID string             `json:"template_id"`
	Name       string             `json:"name"`
	Status     DeployStatus       `json:"status"`
	NodeID     string             `json:"node_id"`
	Services   []*ServiceInstance `json:"services"`
	StartedAt  time.Time          `json:"started_at"`
	FinishedAt time.Time          `json:"finished_at,omitempty"`
	Error      string             `json:"error,omitempty"`
}

// DeployStatus 部署状态
type DeployStatus string

const (
	DeployPending   DeployStatus = "pending"
	DeployRunning   DeployStatus = "running"
	DeploySuccess   DeployStatus = "success"
	DeployFailed    DeployStatus = "failed"
	DeployRolling   DeployStatus = "rolling_back"
	DeployCancelled DeployStatus = "cancelled"
)

// ServiceInstance 服务实例
type ServiceInstance struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Replicas int    `json:"replicas"`
	Healthy  int    `json:"healthy"`
	Message  string `json:"message,omitempty"`
}

// Orchestrator 编排器
type Orchestrator struct {
	mu          sync.RWMutex
	nodes       map[string]*Node
	templates   map[string]*DeployTemplate
	deployments map[string]*Deployment
}

// NewOrchestrator 创建编排器
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		nodes:       make(map[string]*Node),
		templates:   make(map[string]*DeployTemplate),
		deployments: make(map[string]*Deployment),
	}
}

// AddNode 添加节点
func (o *Orchestrator) AddNode(node *Node) {
	o.mu.Lock()
	defer o.mu.Unlock()
	node.JoinedAt = time.Now()
	node.LastHeartbeat = time.Now()
	o.nodes[node.ID] = node
}

// RemoveNode 移除节点
func (o *Orchestrator) RemoveNode(nodeID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	node, exists := o.nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}
	if node.Status == StatusOnline {
		node.Status = StatusDraining
	}
	delete(o.nodes, nodeID)
	return nil
}

// Heartbeat 心跳更新
func (o *Orchestrator) Heartbeat(nodeID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	node, exists := o.nodes[nodeID]
	if !exists {
		return fmt.Errorf("节点 %s 不存在", nodeID)
	}
	node.LastHeartbeat = time.Now()
	node.Status = StatusOnline
	return nil
}

// GetNodes 获取所有节点
func (o *Orchestrator) GetNodes() []*Node {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var nodes []*Node
	for _, node := range o.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})
	return nodes
}

// GetHealthyNodes 获取健康节点
func (o *Orchestrator) GetHealthyNodes() []*Node {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var nodes []*Node
	maxAge := 5 * time.Minute
	for _, node := range o.nodes {
		if node.Status == StatusOnline && time.Since(node.LastHeartbeat) < maxAge {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// SaveTemplate 保存模板
func (o *Orchestrator) SaveTemplate(tmpl *DeployTemplate) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if tmpl.CreatedAt.IsZero() {
		tmpl.CreatedAt = time.Now()
	}
	o.templates[tmpl.ID] = tmpl
}

// Deploy 部署服务
func (o *Orchestrator) Deploy(templateID, nodeID string) (*Deployment, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	tmpl, exists := o.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("模板 %s 不存在", templateID)
	}

	node, exists := o.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("节点 %s 不存在", nodeID)
	}

	if node.Status != StatusOnline {
		return nil, fmt.Errorf("节点 %s 状态异常: %s", nodeID, node.Status)
	}

	dep := &Deployment{
		ID:         fmt.Sprintf("deploy-%d", time.Now().UnixNano()),
		TemplateID: templateID,
		Name:       tmpl.Name,
		Status:     DeployRunning,
		NodeID:     nodeID,
		StartedAt:  time.Now(),
	}
	for _, svc := range tmpl.Services {
		dep.Services = append(dep.Services, &ServiceInstance{
			Name:     svc.Name,
			Status:   "starting",
			Replicas: svc.Replicas,
			Healthy:  0,
		})
	}

	// 模拟部署
	go func(d *Deployment) {
		time.Sleep(1 * time.Second)
		o.mu.Lock()
		defer o.mu.Unlock()
		if d.Status != DeployRunning {
			return
		}
		for _, svc := range d.Services {
			svc.Status = "running"
			svc.Healthy = svc.Replicas
		}
		d.Status = DeploySuccess
		d.FinishedAt = time.Now()
	}(dep)

	o.deployments[dep.ID] = dep
	return dep, nil
}

// GetDeployment 获取部署
func (o *Orchestrator) GetDeployment(depID string) (*Deployment, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	dep, exists := o.deployments[depID]
	if !exists {
		return nil, fmt.Errorf("部署 %s 不存在", depID)
	}
	return dep, nil
}

// ListDeployments 列出部署
func (o *Orchestrator) ListDeployments() []*Deployment {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var deps []*Deployment
	for _, dep := range o.deployments {
		deps = append(deps, dep)
	}
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].StartedAt.After(deps[j].StartedAt)
	})
	return deps
}

// Rollback 回滚部署
func (o *Orchestrator) Rollback(depID string) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	dep, exists := o.deployments[depID]
	if !exists {
		return fmt.Errorf("部署 %s 不存在", depID)
	}

	dep.Status = DeployRolling
	for _, svc := range dep.Services {
		svc.Status = "stopping"
		svc.Healthy = 0
	}

	go func(d *Deployment) {
		time.Sleep(500 * time.Millisecond)
		o.mu.Lock()
		d.Status = DeployCancelled
		d.FinishedAt = time.Now()
		o.mu.Unlock()
	}(dep)

	return nil
}

// FormatNodeList 格式化节点列表
func (o *Orchestrator) FormatNodeList() string {
	nodes := o.GetNodes()
	if len(nodes) == 0 {
		return "无注册节点"
	}

	var sb strings.Builder
	sb.WriteString("集群节点:\n")
	sb.WriteString(strings.Repeat("═", 60) + "\n")
	for _, node := range nodes {
		sb.WriteString(fmt.Sprintf("  %s [%s] %s | %s | %d核/%dGB/%.1fTB\n",
			node.Name, node.Role, node.Status, node.Address,
			node.CPUCores, node.MemoryGB, node.StorageTB))
	}

	healthy := o.GetHealthyNodes()
	sb.WriteString(fmt.Sprintf("\n健康节点: %d/%d\n", len(healthy), len(nodes)))
	return sb.String()
}

// FormatDeployment 格式化部署信息
func (o *Orchestrator) FormatDeployment(dep *Deployment) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("部署 %s [%s]:\n", dep.Name, dep.Status))
	sb.WriteString(strings.Repeat("─", 40) + "\n")
	for _, svc := range dep.Services {
		sb.WriteString(fmt.Sprintf("  %s: %s (%d/%d 健康副本)\n",
			svc.Name, svc.Status, svc.Healthy, svc.Replicas))
	}
	return sb.String()
}
