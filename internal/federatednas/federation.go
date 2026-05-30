package federatednas

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

var (
	ErrNodeNotFound      = errors.New("node not found")
	ErrNodeAlreadyExists = errors.New("node already exists")
	ErrSyncInProgress    = errors.New("sync already in progress")
	ErrConflictNotResolved = errors.New("conflict not resolved")
	ErrInvalidNodeID     = errors.New("invalid node ID")
	ErrNodeOffline       = errors.New("node is offline")
)

// Federation manages multiple NAS devices in a federation.
type Federation struct {
	mu          sync.RWMutex
	nodes       map[string]*FederationNode
	syncJobs    map[string]*SyncJob
	conflicts   map[string]*ConflictRecord
	policies    map[string]*FederationPolicy
	namespace   map[string]*DistributedNamespace
	health      map[string]*NodeHealth
	activeSyncs map[string]bool
}

// NewFederation creates a new Federation instance.
func NewFederation() *Federation {
	return &Federation{
		nodes:       make(map[string]*FederationNode),
		syncJobs:    make(map[string]*SyncJob),
		conflicts:   make(map[string]*ConflictRecord),
		policies:    make(map[string]*FederationPolicy),
		namespace:   make(map[string]*DistributedNamespace),
		health:      make(map[string]*NodeHealth),
		activeSyncs: make(map[string]bool),
	}
}

// RegisterNode registers a new NAS device to the federation.
func (f *Federation) RegisterNode(node *FederationNode) error {
	if node.ID == "" {
		return ErrInvalidNodeID
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.nodes[node.ID]; exists {
		return ErrNodeAlreadyExists
	}

	node.RegisteredAt = time.Now()
	node.LastSeen = time.Now()
	node.Status = NodeStatusOnline
	node.SyncVersion = 0

	f.nodes[node.ID] = node
	log.Printf("Node registered: %s (%s)", node.ID, node.Name)
	return nil
}

// GetNode returns a federation node by ID.
func (f *Federation) GetNode(nodeID string) (*FederationNode, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	node, exists := f.nodes[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}
	return node, nil
}

// ListNodes returns all registered federation nodes.
func (f *Federation) ListNodes() []*FederationNode {
	f.mu.RLock()
	defer f.mu.RUnlock()

	nodes := make([]*FederationNode, 0, len(f.nodes))
	for _, node := range f.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// RemoveNode removes a node from the federation.
func (f *Federation) RemoveNode(nodeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.nodes[nodeID]; !exists {
		return ErrNodeNotFound
	}

	delete(f.nodes, nodeID)
	delete(f.health, nodeID)
	log.Printf("Node removed: %s", nodeID)
	return nil
}

// StartSync initiates a synchronization job between two nodes.
func (f *Federation) StartSync(sourceNodeID, targetNodeID string, incremental bool) (*SyncJob, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	sourceNode, exists := f.nodes[sourceNodeID]
	if !exists {
		return nil, fmt.Errorf("source node: %w", ErrNodeNotFound)
	}

	targetNode, exists := f.nodes[targetNodeID]
	if !exists {
		return nil, fmt.Errorf("target node: %w", ErrNodeNotFound)
	}

	if sourceNode.Status == NodeStatusOffline {
		return nil, fmt.Errorf("source node: %w", ErrNodeOffline)
	}

	if targetNode.Status == NodeStatusOffline {
		return nil, fmt.Errorf("target node: %w", ErrNodeOffline)
	}

	syncKey := sourceNodeID + ":" + targetNodeID
	if f.activeSyncs[syncKey] {
		return nil, ErrSyncInProgress
	}

	jobID := fmt.Sprintf("sync-%s-%s-%d", sourceNodeID, targetNodeID, time.Now().UnixNano())
	job := &SyncJob{
		ID:           jobID,
		SourceNodeID: sourceNodeID,
		TargetNodeID: targetNodeID,
		Status:       "running",
		StartedAt:    time.Now(),
		Incremental:  incremental,
		Resumable:    true,
	}

	f.syncJobs[jobID] = job
	f.activeSyncs[syncKey] = true

	sourceNode.UpdateStatus(NodeStatusSyncing)
	targetNode.UpdateStatus(NodeStatusSyncing)

	go f.executeSync(job)

	log.Printf("Sync started: %s -> %s (job: %s)", sourceNodeID, targetNodeID, jobID)
	return job, nil
}

// executeSync simulates sync execution.
func (f *Federation) executeSync(job *SyncJob) {
	defer func() {
		f.mu.Lock()
		syncKey := job.SourceNodeID + ":" + job.TargetNodeID
		delete(f.activeSyncs, syncKey)
		f.mu.Unlock()

		sourceNode, _ := f.GetNode(job.SourceNodeID)
		targetNode, _ := f.GetNode(job.TargetNodeID)
		if sourceNode != nil {
			sourceNode.UpdateStatus(NodeStatusOnline)
		}
		if targetNode != nil {
			targetNode.UpdateStatus(NodeStatusOnline)
		}
	}()

	// Simulate sync progress
	job.mu.Lock()
	job.TotalFiles = 100
	job.BytesTotal = 1024 * 1024 * 100 // 100MB
	job.mu.Unlock()

	for i := 0; i < 100; i++ {
		time.Sleep(10 * time.Millisecond)
		job.mu.Lock()
		job.SyncedFiles = i + 1
		job.BytesSynced = int64(i+1) * 1024 * 1024
		job.mu.Unlock()
	}

	now := time.Now()
	job.mu.Lock()
	job.Status = "completed"
	job.CompletedAt = &now
	job.mu.Unlock()

	log.Printf("Sync completed: %s", job.ID)
}

// GetSyncJob returns a sync job by ID.
func (f *Federation) GetSyncJob(jobID string) (*SyncJob, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	job, exists := f.syncJobs[jobID]
	if !exists {
		return nil, fmt.Errorf("sync job not found: %s", jobID)
	}
	return job, nil
}

// ListSyncJobs returns all sync jobs.
func (f *Federation) ListSyncJobs() []*SyncJob {
	f.mu.RLock()
	defer f.mu.RUnlock()

	jobs := make([]*SyncJob, 0, len(f.syncJobs))
	for _, job := range f.syncJobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// ResolveConflict resolves a conflict record.
func (f *Federation) ResolveConflict(conflictID, resolution, resolvedBy string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	conflict, exists := f.conflicts[conflictID]
	if !exists {
		return fmt.Errorf("conflict not found: %s", conflictID)
	}

	now := time.Now()
	conflict.Resolution = resolution
	conflict.ResolvedAt = &now
	conflict.ResolvedBy = resolvedBy

	log.Printf("Conflict resolved: %s by %s", conflictID, resolvedBy)
	return nil
}

// GetConflict returns a conflict record by ID.
func (f *Federation) GetConflict(conflictID string) (*ConflictRecord, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	conflict, exists := f.conflicts[conflictID]
	if !exists {
		return nil, fmt.Errorf("conflict not found: %s", conflictID)
	}
	return conflict, nil
}

// ListConflicts returns all conflict records.
func (f *Federation) ListConflicts() []*ConflictRecord {
	f.mu.RLock()
	defer f.mu.RUnlock()

	conflicts := make([]*ConflictRecord, 0, len(f.conflicts))
	for _, conflict := range f.conflicts {
		conflicts = append(conflicts, conflict)
	}
	return conflicts
}

// GetNodeStatus returns the current status of a node.
func (f *Federation) GetNodeStatus(nodeID string) (*NodeHealth, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	health, exists := f.health[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}
	return health, nil
}

// UpdateNodeHealth updates health metrics for a node.
func (f *Federation) UpdateNodeHealth(nodeID string, health *NodeHealth) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, exists := f.nodes[nodeID]; !exists {
		return ErrNodeNotFound
	}

	health.NodeID = nodeID
	health.LastCheckTime = time.Now()
	f.health[nodeID] = health
	return nil
}

// PropagateChange propagates a file change to all relevant nodes.
func (f *Federation) PropagateChange(nodeID, filePath string, isDelete bool) error {
	f.mu.RLock()
	defer f.mu.RUnlock()

	sourceNode, exists := f.nodes[nodeID]
	if !exists {
		return ErrNodeNotFound
	}

	// Update namespace
	nsKey := nodeID + ":" + filePath
	if isDelete {
		delete(f.namespace, nsKey)
	} else {
		f.namespace[nsKey] = &DistributedNamespace{
			Path:    filePath,
			NodeID:  nodeID,
			ModTime: time.Now(),
		}
	}

	// Propagate to other nodes (simulated)
	for _, node := range f.nodes {
		if node.ID == nodeID {
			continue
		}
		if node.Status == NodeStatusOnline {
			log.Printf("Propagating change to node %s: %s", node.ID, filePath)
		}
	}

	_ = sourceNode // Use sourceNode to avoid unused warning
	return nil
}

// AddConflict adds a new conflict record.
func (f *Federation) AddConflict(conflict *ConflictRecord) {
	f.mu.Lock()
	defer f.mu.Unlock()

	conflict.ID = fmt.Sprintf("conflict-%s-%d", conflict.FilePath, time.Now().UnixNano())
	conflict.CreatedAt = time.Now()
	f.conflicts[conflict.ID] = conflict
}

// AddPolicy adds or updates a federation policy.
func (f *Federation) AddPolicy(policy *FederationPolicy) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if policy.ID == "" {
		policy.ID = fmt.Sprintf("policy-%d", time.Now().UnixNano())
	}
	policy.UpdatedAt = time.Now()
	if policy.CreatedAt.IsZero() {
		policy.CreatedAt = time.Now()
	}
	f.policies[policy.ID] = policy
}

// GetPolicy returns a policy by ID.
func (f *Federation) GetPolicy(policyID string) (*FederationPolicy, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	policy, exists := f.policies[policyID]
	if !exists {
		return nil, fmt.Errorf("policy not found: %s", policyID)
	}
	return policy, nil
}

// ListPolicies returns all federation policies.
func (f *Federation) ListPolicies() []*FederationPolicy {
	f.mu.RLock()
	defer f.mu.RUnlock()

	policies := make([]*FederationPolicy, 0, len(f.policies))
	for _, policy := range f.policies {
		policies = append(policies, policy)
	}
	return policies
}

// GetFederationStatus returns the overall federation status.
func (f *Federation) GetFederationStatus() *FederationStatus {
	f.mu.RLock()
	defer f.mu.RUnlock()

	status := &FederationStatus{
		TotalNodes: len(f.nodes),
	}

	for _, node := range f.nodes {
		switch node.Status {
		case NodeStatusOnline:
			status.OnlineNodes++
		case NodeStatusOffline:
			status.OfflineNodes++
		case NodeStatusSyncing:
			status.SyncingNodes++
		case NodeStatusError:
			status.ErrorNodes++
		}
		status.TotalCapacity += node.Capacity
		status.UsedSpace += node.UsedSpace
	}

	for _, job := range f.syncJobs {
		if job.Status == "running" {
			status.ActiveJobs++
		}
	}

	for _, conflict := range f.conflicts {
		if conflict.ResolvedAt == nil {
			status.Conflicts++
		}
	}

	return status
}

// GetNamespace returns the distributed namespace for a path.
func (f *Federation) GetNamespace(path string) []DistributedNamespace {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var results []DistributedNamespace
	for _, ns := range f.namespace {
		if ns.Path == path || (len(ns.Path) > len(path) && ns.Path[:len(path)] == path) {
			results = append(results, *ns)
		}
	}
	return results
}

// generateID generates a unique ID using SHA256.
func generateID(prefix string) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())))
	return fmt.Sprintf("%s-%x", prefix, hash[:8])
}
