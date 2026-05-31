package smartresource

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ResourceType represents resource types
type ResourceType string

const (
	ResourceCPU     ResourceType = "cpu"
	ResourceMemory  ResourceType = "memory"
	ResourceDisk    ResourceType = "disk"
	ResourceNetwork ResourceType = "network"
	ResourceGPU     ResourceType = "gpu"
)

// AllocationStatus represents allocation status
type AllocationStatus string

const (
	StatusPending   AllocationStatus = "pending"
	StatusAllocated AllocationStatus = "allocated"
	StatusReleased  AllocationStatus = "released"
	StatusFailed    AllocationStatus = "failed"
)

// Resource represents a system resource
type Resource struct {
	ID        string       `json:"id"`
	Type      ResourceType `json:"type"`
	Name      string       `json:"name"`
	Total     float64      `json:"total"`
	Used      float64      `json:"used"`
	Available float64      `json:"available"`
	Unit      string       `json:"unit"`
	Status    string       `json:"status"`
	NodeID    string       `json:"node_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Allocation represents a resource allocation
type Allocation struct {
	ID        string           `json:"id"`
	ResourceID string          `json:"resource_id"`
	Service   string           `json:"service"`
	Amount    float64          `json:"amount"`
	Priority  int              `json:"priority"`
	Status    AllocationStatus `json:"status"`
	CreatedAt time.Time        `json:"created_at"`
	ExpiresAt *time.Time       `json:"expires_at,omitempty"`
	ReleasedAt *time.Time      `json:"released_at,omitempty"`
}

// Prediction represents resource usage prediction
type Prediction struct {
	ResourceType ResourceType `json:"resource_type"`
	Current     float64      `json:"current"`
	Predicted   float64      `json:"predicted"`
	TimeWindow  string       `json:"time_window"`
	Confidence  float64      `json:"confidence"`
	Trend       string       `json:"trend"`
	GeneratedAt time.Time    `json:"generated_at"`
}

// Optimization represents a resource optimization suggestion
type Optimization struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	ResourceType ResourceType `json:"resource_type"`
	Savings     float64   `json:"savings"`
	Impact      string    `json:"impact"`
	Effort      string    `json:"effort"`
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
}

// Node represents a cluster node
type Node struct {
	ID        string              `json:"id"`
	Name      string              `json:"name"`
	Host      string              `json:"host"`
	Resources map[ResourceType]*Resource `json:"resources"`
	Status    string              `json:"status"`
	Labels    map[string]string   `json:"labels,omitempty"`
	LastSeen  time.Time           `json:"last_seen"`
}

// Manager manages resource scheduling
type Manager struct {
	mu          sync.RWMutex
	resources   map[string]*Resource
	allocations map[string]*Allocation
	nodes       map[string]*Node
	predictions []*Prediction
	config      *Config
}

// Config represents manager configuration
type Config struct {
	PredictionWindow time.Duration `json:"prediction_window"`
	MaxAllocations   int           `json:"max_allocations"`
	AutoScale        bool          `json:"auto_scale"`
	ScaleThreshold   float64       `json:"scale_threshold"`
}

// DefaultConfig returns default configuration
func DefaultConfig() *Config {
	return &Config{
		PredictionWindow: 24 * time.Hour,
		MaxAllocations:   1000,
		AutoScale:        true,
		ScaleThreshold:   0.8,
	}
}

// NewManager creates a new resource manager
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}

	return &Manager{
		resources:   make(map[string]*Resource),
		allocations: make(map[string]*Allocation),
		nodes:       make(map[string]*Node),
		config:      config,
	}
}

// RegisterNode registers a cluster node
func (m *Manager) RegisterNode(node *Node) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if node.ID == "" {
		return fmt.Errorf("node ID is required")
	}

	node.LastSeen = time.Now()
	node.Status = "online"
	m.nodes[node.ID] = node

	// Register node resources
	for _, res := range node.Resources {
		res.NodeID = node.ID
		m.resources[res.ID] = res
	}

	return nil
}

// UnregisterNode unregisters a node
func (m *Manager) UnregisterNode(nodeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.nodes[nodeID]; !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	// Release all allocations for this node
	for _, alloc := range m.allocations {
		if res, exists := m.resources[alloc.ResourceID]; exists && res.NodeID == nodeID {
			now := time.Now()
			alloc.Status = StatusReleased
			alloc.ReleasedAt = &now
		}
	}

	delete(m.nodes, nodeID)
	return nil
}

// AddResource adds a resource
func (m *Manager) AddResource(resource *Resource) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if resource.ID == "" {
		return fmt.Errorf("resource ID is required")
	}

	resource.Available = resource.Total - resource.Used
	m.resources[resource.ID] = resource
	return nil
}

// GetResource gets a resource by ID
func (m *Manager) GetResource(id string) (*Resource, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	resource, exists := m.resources[id]
	if !exists {
		return nil, fmt.Errorf("resource %s not found", id)
	}

	return resource, nil
}

// ListResources lists all resources
func (m *Manager) ListResources(resourceType ResourceType) []*Resource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	resources := make([]*Resource, 0)
	for _, res := range m.resources {
		if resourceType != "" && res.Type != resourceType {
			continue
		}
		resources = append(resources, res)
	}

	return resources
}

// AllocateResource allocates resources
func (m *Manager) AllocateResource(ctx context.Context, resourceID, service string, amount float64, priority int) (*Allocation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	resource, exists := m.resources[resourceID]
	if !exists {
		return nil, fmt.Errorf("resource %s not found", resourceID)
	}

	if resource.Available < amount {
		return nil, fmt.Errorf("insufficient resources: available %.2f, requested %.2f", resource.Available, amount)
	}

	allocation := &Allocation{
		ID:         fmt.Sprintf("alloc-%d", time.Now().UnixNano()),
		ResourceID: resourceID,
		Service:    service,
		Amount:     amount,
		Priority:   priority,
		Status:     StatusAllocated,
		CreatedAt:  time.Now(),
	}

	resource.Used += amount
	resource.Available -= amount
	m.allocations[allocation.ID] = allocation

	return allocation, nil
}

// ReleaseAllocation releases an allocation
func (m *Manager) ReleaseAllocation(allocationID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	allocation, exists := m.allocations[allocationID]
	if !exists {
		return fmt.Errorf("allocation %s not found", allocationID)
	}

	if allocation.Status != StatusAllocated {
		return fmt.Errorf("allocation is not in allocated status")
	}

	resource, exists := m.resources[allocation.ResourceID]
	if exists {
		resource.Used -= allocation.Amount
		resource.Available += allocation.Amount
	}

	now := time.Now()
	allocation.Status = StatusReleased
	allocation.ReleasedAt = &now

	return nil
}

// GetAllocations gets allocations for a service
func (m *Manager) GetAllocations(service string) []*Allocation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	allocations := make([]*Allocation, 0)
	for _, alloc := range m.allocations {
		if service != "" && alloc.Service != service {
			continue
		}
		allocations = append(allocations, alloc)
	}

	return allocations
}

// PredictUsage predicts resource usage
func (m *Manager) PredictUsage(ctx context.Context, resourceType ResourceType) (*Prediction, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Simple prediction based on current usage
	var totalUsed, totalCapacity float64
	for _, res := range m.resources {
		if res.Type == resourceType {
			totalUsed += res.Used
			totalCapacity += res.Total
		}
	}

	if totalCapacity == 0 {
		return nil, fmt.Errorf("no resources of type %s found", resourceType)
	}

	currentPct := totalUsed / totalCapacity
	predictedPct := currentPct * 1.1 // 10% growth prediction

	trend := "stable"
	if predictedPct > currentPct*1.05 {
		trend = "increasing"
	} else if predictedPct < currentPct*0.95 {
		trend = "decreasing"
	}

	prediction := &Prediction{
		ResourceType: resourceType,
		Current:      currentPct * 100,
		Predicted:    predictedPct * 100,
		TimeWindow:   m.config.PredictionWindow.String(),
		Confidence:   0.85,
		Trend:        trend,
		GeneratedAt:  time.Now(),
	}

	return prediction, nil
}

// GetOptimizations gets optimization suggestions
func (m *Manager) GetOptimizations(ctx context.Context) ([]*Optimization, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	optimizations := make([]*Optimization, 0)

	// Analyze resources for optimization opportunities
	for _, res := range m.resources {
		usagePct := res.Used / res.Total

		if usagePct < 0.3 {
			optimizations = append(optimizations, &Optimization{
				ID:           fmt.Sprintf("opt-%s-%d", res.ID, time.Now().UnixNano()),
				Title:        fmt.Sprintf("Underutilized %s", res.Name),
				Description:  fmt.Sprintf("%s is only %.0f%% utilized. Consider consolidating workloads.", res.Name, usagePct*100),
				ResourceType: res.Type,
				Savings:      res.Total * 0.3,
				Impact:       "low",
				Effort:       "medium",
				Priority:     2,
				CreatedAt:    time.Now(),
			})
		} else if usagePct > 0.9 {
			optimizations = append(optimizations, &Optimization{
				ID:           fmt.Sprintf("opt-%s-%d", res.ID, time.Now().UnixNano()),
				Title:        fmt.Sprintf("High %s usage", res.Name),
				Description:  fmt.Sprintf("%s is %.0f%% utilized. Consider scaling up.", res.Name, usagePct*100),
				ResourceType: res.Type,
				Savings:      0,
				Impact:       "high",
				Effort:       "high",
				Priority:     1,
				CreatedAt:    time.Now(),
			})
		}
	}

	return optimizations, nil
}

// GetNodes gets all nodes
func (m *Manager) GetNodes() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*Node, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}

	return nodes
}

// GetClusterStats gets cluster statistics
func (m *Manager) GetClusterStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_nodes":      len(m.nodes),
		"online_nodes":     0,
		"total_resources":  len(m.resources),
		"total_allocations": len(m.allocations),
	}

	for _, node := range m.nodes {
		if node.Status == "online" {
			stats["online_nodes"] = stats["online_nodes"].(int) + 1
		}
	}

	return stats
}

// HandleHTTP registers HTTP handlers
func (m *Manager) HandleHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/resource/resources", m.handleResources)
	mux.HandleFunc("/api/v1/resource/allocate", m.handleAllocate)
	mux.HandleFunc("/api/v1/resource/release", m.handleRelease)
	mux.HandleFunc("/api/v1/resource/allocations", m.handleAllocations)
	mux.HandleFunc("/api/v1/resource/predict", m.handlePredict)
	mux.HandleFunc("/api/v1/resource/optimizations", m.handleOptimizations)
	mux.HandleFunc("/api/v1/resource/nodes", m.handleNodes)
	mux.HandleFunc("/api/v1/resource/stats", m.handleStats)
}

func (m *Manager) handleResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resourceType := ResourceType(r.URL.Query().Get("type"))
	resources := m.ListResources(resourceType)
	json.NewEncoder(w).Encode(resources)
}

func (m *Manager) handleAllocate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ResourceID string  `json:"resource_id"`
		Service    string  `json:"service"`
		Amount     float64 `json:"amount"`
		Priority   int     `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	allocation, err := m.AllocateResource(r.Context(), req.ResourceID, req.Service, req.Amount, req.Priority)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(allocation)
}

func (m *Manager) handleRelease(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		AllocationID string `json:"allocation_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.ReleaseAllocation(req.AllocationID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "released"})
}

func (m *Manager) handleAllocations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	service := r.URL.Query().Get("service")
	allocations := m.GetAllocations(service)
	json.NewEncoder(w).Encode(allocations)
}

func (m *Manager) handlePredict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resourceType := ResourceType(r.URL.Query().Get("type"))
	prediction, err := m.PredictUsage(r.Context(), resourceType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(prediction)
}

func (m *Manager) handleOptimizations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	opts, _ := m.GetOptimizations(r.Context())
	json.NewEncoder(w).Encode(opts)
}

func (m *Manager) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodes := m.GetNodes()
	json.NewEncoder(w).Encode(nodes)
}

func (m *Manager) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := m.GetClusterStats()
	json.NewEncoder(w).Encode(stats)
}
