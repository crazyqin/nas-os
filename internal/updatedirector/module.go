// Package updatedirector orchestrates multi-node update coordination, rolling
// update rollouts, rollback management, and update window scheduling. Inspired
// by TrueNAS update management, it ensures updates are applied safely across
// a cluster with minimal downtime and automatic recovery on failure.
package updatedirector

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// -------------------- Domain Types --------------------

// UpdateDirector is the central coordinator for cluster updates.
type UpdateDirector struct {
	mu         sync.RWMutex
	nodes      map[string]*NodeState
	rollouts   map[string]*RolloutPlan
	rollbacks  map[string]*RollbackState
	windows    map[string]*UpdateWindow
	version    string
}

// NodeState tracks the update status of a single node.
type NodeState struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Role        string    `json:"role"`        // "controller" | "worker" | "storage"
	Version     string    `json:"version"`     // current running version
	Status      NodeStatus `json:"status"`
	LastUpdated time.Time `json:"last_updated"`
	Error       string    `json:"error,omitempty"`
}

// NodeStatus represents the update lifecycle state of a node.
type NodeStatus string

const (
	NodeStatusIdle       NodeStatus = "idle"
	NodeStatusPreparing  NodeStatus = "preparing"
	NodeStatusUpdating   NodeStatus = "updating"
	NodeStatusRebooting  NodeStatus = "rebooting"
	NodeStatusVerifying  NodeStatus = "verifying"
	NodeStatusComplete   NodeStatus = "complete"
	NodeStatusFailed     NodeStatus = "failed"
	NodeStatusRolledBack NodeStatus = "rolled_back"
)

// RolloutPlan describes a multi-node update rollout strategy.
type RolloutPlan struct {
	ID           string         `json:"id"`
	FromVersion  string         `json:"from_version"`
	ToVersion    string         `json:"to_version"`
	Strategy     RolloutStrategy `json:"strategy"`
	BatchSize    int            `json:"batch_size"`    // nodes per batch (for rolling)
	Batches      [][]string     `json:"batches"`       // node IDs per batch
	CurrentBatch int            `json:"current_batch"`
	Status       RolloutStatus  `json:"status"`
	StartTime    *time.Time     `json:"start_time,omitempty"`
	EndTime      *time.Time     `json:"end_time,omitempty"`
	AutoRollback bool           `json:"auto_rollback"`
	CreatedAt    time.Time      `json:"created_at"`
}

// RolloutStrategy defines how nodes are updated.
type RolloutStrategy string

const (
	StrategyRolling  RolloutStrategy = "rolling"   // one batch at a time
	StrategyAllAtOnce RolloutStrategy = "all_at_once"
	StrategyCanary   RolloutStrategy = "canary"    // single node first, then rest
	StrategyBlueGreen RolloutStrategy = "blue_green"
)

// RolloutStatus tracks overall progress.
type RolloutStatus string

const (
	RolloutStatusPending   RolloutStatus = "pending"
	RolloutStatusRunning    RolloutStatus = "running"
	RolloutStatusPaused    RolloutStatus = "paused"
	RolloutStatusCompleted RolloutStatus = "completed"
	RolloutStatusFailed    RolloutStatus = "failed"
	RolloutStatusRolledBack RolloutStatus = "rolled_back"
)

// RollbackState tracks the state of a rollback operation.
type RollbackState struct {
	ID            string         `json:"id"`
	RolloutID     string         `json:"rollout_id"`
	Reason        string         `json:"reason"`
	FromVersion   string         `json:"to_version"`  // what we tried to upgrade to
	ToVersion     string         `json:"from_version"` // what we're going back to
	CompletedNodes []string      `json:"completed_nodes"`
	Status        RolloutStatus  `json:"status"`
	StartedAt     time.Time      `json:"started_at"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty"`
}

// UpdateWindow defines a scheduled maintenance window for updates.
type UpdateWindow struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	StartTime   time.Time    `json:"start_time"`
	EndTime     time.Time    `json:"end_time"`
	Recurring   bool         `json:"recurring"`
	Recurrence  string      `json:"recurrence,omitempty"` // "daily" | "weekly" | "monthly"
	Weekdays    []time.Weekday `json:"weekdays,omitempty"`
	MaxNodes    int          `json:"max_nodes"`    // max concurrent updates during window
	AutoStart    bool         `json:"auto_start"`   // auto-trigger pending rollouts
	Enabled     bool         `json:"enabled"`
	CreatedAt   time.Time    `json:"created_at"`
}

// -------------------- Constructor --------------------

// NewUpdateDirector creates a new director with the given cluster version.
func NewUpdateDirector(currentVersion string) *UpdateDirector {
	return &UpdateDirector{
		nodes:     make(map[string]*NodeState),
		rollouts:  make(map[string]*RolloutPlan),
		rollbacks: make(map[string]*RollbackState),
		windows:   make(map[string]*UpdateWindow),
		version:   currentVersion,
	}
}

// RegisterNode adds a node to the director's management scope.
func (d *UpdateDirector) RegisterNode(node NodeState) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nodes[node.ID] = &node
}

// GetNode returns the state of a specific node.
func (d *UpdateDirector) GetNode(id string) (*NodeState, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	node, ok := d.nodes[id]
	return node, ok
}

// ListNodes returns all registered nodes.
func (d *UpdateDirector) ListNodes() []NodeState {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]NodeState, 0, len(d.nodes))
	for _, n := range d.nodes {
		result = append(result, *n)
	}
	return result
}

// -------------------- Core Methods --------------------

// PlanRollout creates a rollout plan for upgrading nodes from current to target version.
func (d *UpdateDirector) PlanRollout(ctx context.Context, targetVersion string, strategy RolloutStrategy, batchSize int) (*RolloutPlan, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	plan := &RolloutPlan{
		ID:          fmt.Sprintf("rollout-%d", time.Now().UnixMilli()),
		FromVersion: d.version,
		ToVersion:   targetVersion,
		Strategy:    strategy,
		BatchSize:   batchSize,
		Status:      RolloutStatusPending,
		AutoRollback: true,
		CreatedAt:   time.Now(),
	}

	// Collect node IDs and organize into batches.
	nodeIDs := make([]string, 0, len(d.nodes))
	for id := range d.nodes {
		nodeIDs = append(nodeIDs, id)
	}

	switch strategy {
	case StrategyAllAtOnce:
		plan.Batches = [][]string{nodeIDs}
	case StrategyCanary:
		if len(nodeIDs) > 0 {
			plan.Batches = [][]string{{nodeIDs[0]}, nodeIDs[1:]}
		}
	case StrategyBlueGreen:
		mid := len(nodeIDs) / 2
		plan.Batches = [][]string{nodeIDs[:mid], nodeIDs[mid:]}
	default: // rolling
		plan.Batches = d.buildBatches(nodeIDs, batchSize)
	}

	d.rollouts[plan.ID] = plan
	return plan, nil
}

// ExecuteUpdate starts or resumes a rollout plan, updating nodes batch by batch.
func (d *UpdateDirector) ExecuteUpdate(ctx context.Context, rolloutID string) error {
	d.mu.Lock()
	plan, ok := d.rollouts[rolloutID]
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("rollout %s not found", rolloutID)
	}
	plan.Status = RolloutStatusRunning
	now := time.Now()
	plan.StartTime = &now
	d.mu.Unlock()

	for i := plan.CurrentBatch; i < len(plan.Batches); i++ {
		batch := plan.Batches[i]
		// Update each node in the batch.
		for _, nodeID := range batch {
			if err := d.updateNode(ctx, plan, nodeID); err != nil {
				if plan.AutoRollback {
					_ = d.Rollback(ctx, rolloutID, "update failed on "+nodeID+": "+err.Error())
				}
				return fmt.Errorf("batch %d failed on node %s: %w", i, nodeID, err)
			}
		}
		d.mu.Lock()
		plan.CurrentBatch = i + 1
		d.mu.Unlock()
	}

	d.mu.Lock()
	plan.Status = RolloutStatusCompleted
	end := time.Now()
	plan.EndTime = &end
	d.version = plan.ToVersion
	d.mu.Unlock()

	return nil
}

// Rollback initiates a rollback of a completed or in-progress rollout.
func (d *UpdateDirector) Rollback(ctx context.Context, rolloutID, reason string) error {
	d.mu.Lock()
	plan, ok := d.rollouts[rolloutID]
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("rollout %s not found", rolloutID)
	}

	rb := &RollbackState{
		ID:           fmt.Sprintf("rollback-%d", time.Now().UnixMilli()),
		RolloutID:    rolloutID,
		Reason:       reason,
		FromVersion:  plan.ToVersion,
		ToVersion:    plan.FromVersion,
		CompletedNodes: make([]string, 0),
		Status:       RolloutStatusRunning,
		StartedAt:    time.Now(),
	}
	d.rollbacks[rb.ID] = rb
	plan.Status = RolloutStatusRolledBack
	d.mu.Unlock()

	// Roll back each updated node.
	for _, batch := range plan.Batches[:plan.CurrentBatch] {
		for _, nodeID := range batch {
			if err := d.rollbackNode(ctx, rb, plan, nodeID); err != nil {
				rb.Status = RolloutStatusFailed
				return fmt.Errorf("rollback failed on node %s: %w", nodeID, err)
			}
		}
	}

	d.mu.Lock()
	rb.Status = RolloutStatusCompleted
	now := time.Now()
	rb.CompletedAt = &now
	d.mu.Unlock()

	return nil
}

// ScheduleWindow creates or updates a maintenance window for updates.
func (d *UpdateDirector) ScheduleWindow(ctx context.Context, window UpdateWindow) (*UpdateWindow, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if window.ID == "" {
		window.ID = fmt.Sprintf("window-%d", time.Now().UnixMilli())
	}
	if window.CreatedAt.IsZero() {
		window.CreatedAt = time.Now()
	}
	d.windows[window.ID] = &window
	return &window, nil
}

// GetRollout retrieves a rollout plan by ID.
func (d *UpdateDirector) GetRollout(id string) (*RolloutPlan, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	r, ok := d.rollouts[id]
	return r, ok
}

// ListWindows returns all configured update windows.
func (d *UpdateDirector) ListWindows() []UpdateWindow {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]UpdateWindow, 0, len(d.windows))
	for _, w := range d.windows {
		result = append(result, *w)
	}
	return result
}

// IsWindowActive checks whether any maintenance window is currently active.
func (d *UpdateDirector) IsWindowActive() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	now := time.Now()
	for _, w := range d.windows {
		if !w.Enabled {
			continue
		}
		if now.After(w.StartTime) && now.Before(w.EndTime) {
			return true
		}
	}
	return false
}

// -------------------- Internal Helpers --------------------

// updateNode simulates updating a single node through its lifecycle.
func (d *UpdateDirector) updateNode(ctx context.Context, plan *RolloutPlan, nodeID string) error {
	d.mu.Lock()
	node, ok := d.nodes[nodeID]
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("node %s not registered", nodeID)
	}
	node.Status = NodeStatusPreparing
	d.mu.Unlock()

	// Simulate update phases.
	d.setNodeStatus(nodeID, NodeStatusUpdating)
	d.setNodeStatus(nodeID, NodeStatusRebooting)
	d.setNodeStatus(nodeID, NodeStatusVerifying)

	d.mu.Lock()
	node.Version = plan.ToVersion
	node.Status = NodeStatusComplete
	node.LastUpdated = time.Now()
	d.mu.Unlock()

	return nil
}

// rollbackNode reverts a single node to the previous version.
func (d *UpdateDirector) rollbackNode(ctx context.Context, rb *RollbackState, plan *RolloutPlan, nodeID string) error {
	d.mu.Lock()
	node, ok := d.nodes[nodeID]
	if !ok {
		d.mu.Unlock()
		return fmt.Errorf("node %s not registered", nodeID)
	}
	node.Version = rb.ToVersion
	node.Status = NodeStatusRolledBack
	d.mu.Unlock()

	d.mu.Lock()
	rb.CompletedNodes = append(rb.CompletedNodes, nodeID)
	d.mu.Unlock()

	return nil
}

func (d *UpdateDirector) setNodeStatus(nodeID string, status NodeStatus) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if node, ok := d.nodes[nodeID]; ok {
		node.Status = status
	}
}

// buildBatches splits node IDs into batches of the given size.
func (d *UpdateDirector) buildBatches(nodeIDs []string, batchSize int) [][]string {
	if batchSize <= 0 {
		batchSize = 1
	}
	var batches [][]string
	for i := 0; i < len(nodeIDs); i += batchSize {
		end := i + batchSize
		if end > len(nodeIDs) {
			end = len(nodeIDs)
		}
		batches = append(batches, nodeIDs[i:end])
	}
	return batches
}