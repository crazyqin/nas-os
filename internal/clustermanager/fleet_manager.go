package clustermanager

import (
	"fmt"
	"sync"
	"time"
)

// FleetDeployStatus represents the status of a mass deployment.
type FleetDeployStatus string

const (
	FleetDeployPending    FleetDeployStatus = "pending"
	FleetDeployInProgress FleetDeployStatus = "in_progress"
	FleetDeployCompleted  FleetDeployStatus = "completed"
	FleetDeployFailed     FleetDeployStatus = "failed"
	FleetDeployPartial    FleetDeployStatus = "partial" // some nodes succeeded
)

// FleetTemplate defines a provisioning template for mass deployment.
type FleetTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Config      map[string]string `json:"config"`      // configuration key-value pairs
	Packages    []string          `json:"packages"`     // packages to install
	Services    []string          `json:"services"`     // services to enable
	Scripts     []string          `json:"scripts"`      // post-provision scripts
	CreatedAt   time.Time         `json:"created_at"`
	Version     string            `json:"version"`
}

// FleetDeployment tracks a mass deployment operation.
type FleetDeployment struct {
	ID          string                   `json:"id"`
	TemplateID  string                   `json:"template_id"`
	TargetNodes []string                 `json:"target_nodes"`
	Status      FleetDeployStatus         `json:"status"`
	StartTime   time.Time                `json:"start_time"`
	EndTime     *time.Time               `json:"end_time,omitempty"`
	Results     map[string]*FleetDeployResult `json:"results"` // nodeID -> result
	TotalNodes  int                      `json:"total_nodes"`
	SuccessCount int                     `json:"success_count"`
	FailCount    int                     `json:"fail_count"`
}

// FleetDeployResult represents the deployment result for a single node.
type FleetDeployResult struct {
	NodeID    string           `json:"node_id"`
	Status    FleetDeployStatus `json:"status"`
	StartTime time.Time        `json:"start_time"`
	EndTime   time.Time        `json:"end_time"`
	Error     string           `json:"error,omitempty"`
	Logs      []string         `json:"logs,omitempty"`
}

// FleetManager manages fleet-wide provisioning and configuration.
type FleetManager struct {
	mu          sync.RWMutex
	templates   map[string]*FleetTemplate
	deployments []*FleetDeployment
	nodes       map[string]*FleetNode
	groups      map[string]*FleetGroup
}

// FleetNode represents a managed node in the fleet.
type FleetNode struct {
	ID           string            `json:"id"`
	Hostname     string            `json:"hostname"`
	IP           string            `json:"ip"`
	Status       NodeStatus        `json:"status"`
	Version      string            `json:"version"`
	LastSeen     time.Time         `json:"last_seen"`
	GroupID      string            `json:"group_id"`
	Tags         map[string]string `json:"tags"`
	TemplateID   string            `json:"template_id"`
	ProvisionedAt *time.Time       `json:"provisioned_at,omitempty"`
}

// FleetGroup represents a group of nodes.
type FleetGroup struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	NodeIDs     []string `json:"node_ids"`
	TemplateID  string   `json:"template_id"`
}

// NewFleetManager creates a new fleet manager.
func NewFleetManager() *FleetManager {
	return &FleetManager{
		templates:   make(map[string]*FleetTemplate),
		deployments: make([]*FleetDeployment, 0),
		nodes:       make(map[string]*FleetNode),
		groups:      make(map[string]*FleetGroup),
	}
}

// CreateTemplate creates a provisioning template.
func (fm *FleetManager) CreateTemplate(template FleetTemplate) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if template.ID == "" {
		return fmt.Errorf("template ID cannot be empty")
	}
	if _, exists := fm.templates[template.ID]; exists {
		return fmt.Errorf("template already exists: %s", template.ID)
	}
	template.CreatedAt = time.Now()
	fm.templates[template.ID] = &template
	return nil
}

// RegisterNode registers a managed node.
func (fm *FleetManager) RegisterNode(node FleetNode) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if node.ID == "" {
		return fmt.Errorf("node ID cannot be empty")
	}
	node.LastSeen = time.Now()
	fm.nodes[node.ID] = &node
	return nil
}

// RemoveNode removes a node from fleet management.
func (fm *FleetManager) RemoveNode(nodeID string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if _, exists := fm.nodes[nodeID]; !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}
	delete(fm.nodes, nodeID)
	return nil
}

// CreateGroup creates a node group.
func (fm *FleetManager) CreateGroup(group FleetGroup) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	if group.ID == "" {
		return fmt.Errorf("group ID cannot be empty")
	}
	fm.groups[group.ID] = &group
	return nil
}

// AddNodeToGroup adds a node to a group.
func (fm *FleetManager) AddNodeToGroup(nodeID, groupID string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	node, exists := fm.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node not found: %s", nodeID)
	}
	group, exists := fm.groups[groupID]
	if !exists {
		return fmt.Errorf("group not found: %s", groupID)
	}

	node.GroupID = groupID
	group.NodeIDs = append(group.NodeIDs, nodeID)
	return nil
}

// MassDeploy deploys a template to multiple nodes.
func (fm *FleetManager) MassDeploy(templateID string, nodeIDs []string) (*FleetDeployment, error) {
	fm.mu.Lock()

	if _, exists := fm.templates[templateID]; !exists {
		fm.mu.Unlock()
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	deployment := &FleetDeployment{
		ID:          fmt.Sprintf("deploy-%d", time.Now().UnixNano()),
		TemplateID:  templateID,
		TargetNodes: nodeIDs,
		Status:      FleetDeployInProgress,
		StartTime:   time.Now(),
		Results:     make(map[string]*FleetDeployResult),
		TotalNodes:  len(nodeIDs),
	}

	fm.deployments = append(fm.deployments, deployment)
	fm.mu.Unlock()

	// Simulate deployment (in real implementation, this would use SSH/API)
	for _, nodeID := range nodeIDs {
		result := &FleetDeployResult{
			NodeID:    nodeID,
			StartTime: time.Now(),
		}

		fm.mu.RLock()
		_, exists := fm.nodes[nodeID]
		fm.mu.RUnlock()

		if !exists {
			result.Status = FleetDeployFailed
			result.Error = "node not found"
			result.EndTime = time.Now()
			deployment.FailCount++
		} else {
			result.Status = FleetDeployCompleted
			result.EndTime = time.Now()
			deployment.SuccessCount++
		}

		deployment.Results[nodeID] = result
	}

	endTime := time.Now()
	deployment.EndTime = &endTime
	if deployment.FailCount == 0 {
		deployment.Status = FleetDeployCompleted
	} else if deployment.SuccessCount > 0 {
		deployment.Status = FleetDeployPartial
	} else {
		deployment.Status = FleetDeployFailed
	}

	return deployment, nil
}

// DeployToGroup deploys a template to all nodes in a group.
func (fm *FleetManager) DeployToGroup(templateID, groupID string) (*FleetDeployment, error) {
	fm.mu.RLock()
	group, exists := fm.groups[groupID]
	if !exists {
		fm.mu.RUnlock()
		return nil, fmt.Errorf("group not found: %s", groupID)
	}
	nodeIDs := make([]string, len(group.NodeIDs))
	copy(nodeIDs, group.NodeIDs)
	fm.mu.RUnlock()

	return fm.MassDeploy(templateID, nodeIDs)
}

// GetDeployment returns a deployment by ID.
func (fm *FleetManager) GetDeployment(deploymentID string) (*FleetDeployment, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	for _, d := range fm.deployments {
		if d.ID == deploymentID {
			return d, nil
		}
	}
	return nil, fmt.Errorf("deployment not found: %s", deploymentID)
}

// ListDeployments returns all deployments.
func (fm *FleetManager) ListDeployments() []*FleetDeployment {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	result := make([]*FleetDeployment, len(fm.deployments))
	copy(result, fm.deployments)
	return result
}

// ListNodes returns all managed nodes.
func (fm *FleetManager) ListNodes() []FleetNode {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	result := make([]FleetNode, 0, len(fm.nodes))
	for _, n := range fm.nodes {
		result = append(result, *n)
	}
	return result
}

// GetFleetStats returns fleet-wide statistics.
func (fm *FleetManager) GetFleetStats() map[string]int {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	stats := map[string]int{
		"total_nodes":  len(fm.nodes),
		"total_groups": len(fm.groups),
		"deployments":  len(fm.deployments),
	}

	for _, n := range fm.nodes {
		switch n.Status {
		case NodeStatusOnline:
			stats["online"]++
		case NodeStatusOffline:
			stats["offline"]++
		case NodeStatusError:
			stats["error"]++
		}
	}
	return stats
}
