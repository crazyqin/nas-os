package smbfailover

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// FailoverExecutor executes failover operations
type FailoverExecutor struct {
	mu              sync.RWMutex
	config          FailoverConfig
	logger          *zap.Logger
	sessionManager  *SessionManager
	detector        *FailureDetector
	synchronizer    *StateSynchronizer
	auditLogger     *AuditLogger
	nodes           map[string]*ClusterNode
	localNodeID     string
	activeNodeID    string
	vipManager      *VIPManager
	state           FailoverState
	lastFailover    time.Time
	eventHistory    []FailoverEvent
	metrics         FailoverMetrics
	onFailoverStart func(event *FailoverEvent)
	onFailoverEnd   func(event *FailoverEvent)
}

// FailoverMetrics tracks failover metrics
type FailoverMetrics struct {
	mu               sync.RWMutex
	TotalFailovers   int64           `json:"total_failovers"`
	Successful       int64           `json:"successful"`
	Failed           int64           `json:"failed"`
	AverageDuration  time.Duration   `json:"average_duration"`
	LastFailoverTime time.Time       `json:"last_failover_time"`
	TotalSessions    int64           `json:"total_sessions_transferred"`
	FailoverHistory  []FailoverEvent `json:"failover_history"`
}

// VIPManager manages virtual IP addresses
type VIPManager struct {
	mu            sync.RWMutex
	vips          map[string]*VIPConfig
	logger        *zap.Logger
	interfaceName string
}

// VIPConfig represents a virtual IP configuration
type VIPConfig struct {
	IP          string `json:"ip"`
	Netmask     string `json:"netmask"`
	Gateway     string `json:"gateway"`
	OwnerNodeID string `json:"owner_node_id"`
	Active      bool   `json:"active"`
}

// FailoverStrategy defines failover strategies
type FailoverStrategy string

const (
	StrategyAutomatic FailoverStrategy = "automatic"
	StrategyManual    FailoverStrategy = "manual"
	StrategyQuorum    FailoverStrategy = "quorum"
	StrategyPriority  FailoverStrategy = "priority"
)

// NewFailoverExecutor creates a new failover executor
func NewFailoverExecutor(
	config FailoverConfig,
	sessionManager *SessionManager,
	detector *FailureDetector,
	synchronizer *StateSynchronizer,
	auditLogger *AuditLogger,
	logger *zap.Logger,
) *FailoverExecutor {
	return &FailoverExecutor{
		config:         config,
		logger:         logger,
		sessionManager: sessionManager,
		detector:       detector,
		synchronizer:   synchronizer,
		auditLogger:    auditLogger,
		nodes:          make(map[string]*ClusterNode),
		eventHistory:   make([]FailoverEvent, 0, 100),
		vipManager: &VIPManager{
			vips:   make(map[string]*VIPConfig),
			logger: logger,
		},
	}
}

// SetLocalNode sets the local node ID
func (fe *FailoverExecutor) SetLocalNode(nodeID string) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.localNodeID = nodeID
}

// RegisterNode registers a cluster node
func (fe *FailoverExecutor) RegisterNode(node *ClusterNode) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.nodes[node.ID] = node

	if node.State == NodeStateActive {
		fe.activeNodeID = node.ID
	}

	fe.logger.Info("node registered for failover",
		zap.String("node_id", node.ID),
		zap.String("state", string(node.State)),
		zap.Int("priority", node.Priority))
}

// SetFailoverCallbacks sets failover lifecycle callbacks
func (fe *FailoverExecutor) SetFailoverCallbacks(
	onStart func(event *FailoverEvent),
	onEnd func(event *FailoverEvent),
) {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	fe.onFailoverStart = onStart
	fe.onFailoverEnd = onEnd
}

// ExecuteFailover executes a failover to a target node
func (fe *FailoverExecutor) ExecuteFailover(ctx context.Context, targetNodeID, reason string) error {
	fe.mu.Lock()
	if fe.state != FailoverStateIdle {
		fe.mu.Unlock()
		return fmt.Errorf("failover already in progress: %s", fe.state)
	}

	// Validate target node
	targetNode, ok := fe.nodes[targetNodeID]
	if !ok {
		fe.mu.Unlock()
		return fmt.Errorf("target node %s not found", targetNodeID)
	}

	if targetNode.State != NodeStateActive && targetNode.State != NodeStateStandby {
		fe.mu.Unlock()
		return fmt.Errorf("target node %s not ready: %s", targetNodeID, targetNode.State)
	}

	// Check quorum
	if !fe.detector.HasQuorum() {
		fe.mu.Unlock()
		return fmt.Errorf("failover aborted: no quorum")
	}

	fe.state = FailoverStateDetecting
	startTime := time.Now()

	event := &FailoverEvent{
		ID:        fmt.Sprintf("failover-%d", time.Now().UnixNano()),
		Timestamp: startTime,
		FromNode:  fe.activeNodeID,
		ToNode:    targetNodeID,
		Reason:    reason,
		State:     FailoverStateFailing,
	}
	fe.mu.Unlock()

	// Notify start
	fe.notifyFailoverStart(event)

	// Log to audit
	if fe.auditLogger != nil {
		fe.auditLogger.LogFailoverStart(event)
	}

	fe.logger.Info("failover started",
		zap.String("event_id", event.ID),
		zap.String("from", event.FromNode),
		zap.String("to", event.ToNode),
		zap.String("reason", reason))

	// Execute failover steps
	err := fe.executeFailoverSteps(ctx, event, targetNodeID)

	// Update event
	fe.mu.Lock()
	fe.state = FailoverStateIdle
	event.Duration = time.Since(startTime)

	if err != nil {
		event.State = FailoverStateFailed
		event.Success = false
		fe.metrics.Failed++
	} else {
		event.State = FailoverStateCompleted
		event.Success = true
		fe.activeNodeID = targetNodeID
		fe.metrics.Successful++
	}

	fe.metrics.TotalFailovers++
	fe.metrics.LastFailoverTime = time.Now()
	fe.metrics.FailoverHistory = append(fe.metrics.FailoverHistory, *event)
	fe.eventHistory = append(fe.eventHistory, *event)
	fe.lastFailover = time.Now()
	fe.mu.Unlock()

	// Notify end
	fe.notifyFailoverEnd(event)

	// Log to audit
	if fe.auditLogger != nil {
		fe.auditLogger.LogFailoverEnd(event)
	}

	fe.logger.Info("failover completed",
		zap.String("event_id", event.ID),
		zap.Bool("success", event.Success),
		zap.Duration("duration", event.Duration),
		zap.Error(err))

	return err
}

// executeFailoverSteps executes the failover steps
func (fe *FailoverExecutor) executeFailoverSteps(ctx context.Context, event *FailoverEvent, targetNodeID string) error {
	// Step 1: Freeze session state
	fe.logger.Info("step 1: freezing session state")
	if err := fe.freezeSessionState(); err != nil {
		return fmt.Errorf("failed to freeze sessions: %w", err)
	}

	// Step 2: Synchronize state to target
	fe.logger.Info("step 2: synchronizing state to target")
	if err := fe.synchronizeState(ctx, targetNodeID); err != nil {
		return fmt.Errorf("failed to sync state: %w", err)
	}

	// Step 3: Move VIP
	fe.logger.Info("step 3: moving VIP")
	if err := fe.moveVIP(targetNodeID); err != nil {
		return fmt.Errorf("failed to move VIP: %w", err)
	}

	// Step 4: Update node states
	fe.logger.Info("step 4: updating node states")
	fe.updateNodeStates(targetNodeID)

	// Step 5: Resume sessions on target
	fe.logger.Info("step 5: resuming sessions on target")
	sessionsTransferred, err := fe.resumeSessionsOnTarget(ctx, targetNodeID)
	if err != nil {
		return fmt.Errorf("failed to resume sessions: %w", err)
	}
	event.Sessions = sessionsTransferred

	// Step 6: Send GARP
	fe.logger.Info("step 6: sending GARP")
	if err := fe.sendGARP(targetNodeID); err != nil {
		fe.logger.Warn("GARP failed (non-fatal)", zap.Error(err))
	}

	return nil
}

// freezeSessionState freezes all active sessions
func (fe *FailoverExecutor) freezeSessionState() error {
	if fe.sessionManager == nil {
		return nil
	}

	fe.logger.Info("freezing session state")

	// In production, this would pause all session operations
	// For now, we'll persist current state
	if err := fe.sessionManager.PersistSessions(); err != nil {
		return fmt.Errorf("failed to persist sessions: %w", err)
	}

	return nil
}

// synchronizeState synchronizes state to target node
func (fe *FailoverExecutor) synchronizeState(ctx context.Context, targetNodeID string) error {
	if fe.synchronizer == nil {
		return nil
	}

	fe.logger.Info("synchronizing state", zap.String("target", targetNodeID))

	// Get serialized sessions
	sessions, err := fe.sessionManager.GetSerializedSessions()
	if err != nil {
		return fmt.Errorf("failed to get sessions: %w", err)
	}

	// Send to target
	if err := fe.synchronizer.SyncSessions(ctx, targetNodeID, sessions); err != nil {
		return fmt.Errorf("failed to sync sessions: %w", err)
	}

	return nil
}

// moveVIP moves the virtual IP to the target node
func (fe *FailoverExecutor) moveVIP(targetNodeID string) error {
	fe.vipManager.mu.Lock()
	defer fe.vipManager.mu.Unlock()

	for vipID, vip := range fe.vipManager.vips {
		if vip.OwnerNodeID == fe.localNodeID {
			vip.OwnerNodeID = targetNodeID
			fe.logger.Info("VIP moved",
				zap.String("vip_id", vipID),
				zap.String("from", fe.localNodeID),
				zap.String("to", targetNodeID))
		}
	}

	return nil
}

// updateNodeStates updates node states after failover
func (fe *FailoverExecutor) updateNodeStates(targetNodeID string) {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	// Update old active to standby
	if oldActive, ok := fe.nodes[fe.activeNodeID]; ok {
		oldActive.State = NodeStateStandby
	}

	// Update new active
	if newActive, ok := fe.nodes[targetNodeID]; ok {
		newActive.State = NodeStateActive
	}
}

// resumeSessionsOnTarget resumes sessions on the target node
func (fe *FailoverExecutor) resumeSessionsOnTarget(ctx context.Context, targetNodeID string) (int, error) {
	if fe.sessionManager == nil {
		return 0, nil
	}

	// In production, this would send resume command to target
	// Target would restore sessions from synchronized state
	sessions := fe.sessionManager.GetSessionCount()
	fe.logger.Info("sessions resumed on target",
		zap.String("target", targetNodeID),
		zap.Int("count", sessions))

	return sessions, nil
}

// sendGARP sends Gratuitous ARP to update network caches
func (fe *FailoverExecutor) sendGARP(targetNodeID string) error {
	if !fe.config.EnableGRATARP {
		return nil
	}

	fe.logger.Info("sending GARP", zap.String("target", targetNodeID))

	// In production, this would send actual GARP packets
	// This ensures clients update their ARP cache quickly

	return nil
}

// notifyFailoverStart notifies failover start
func (fe *FailoverExecutor) notifyFailoverStart(event *FailoverEvent) {
	fe.mu.RLock()
	callback := fe.onFailoverStart
	fe.mu.RUnlock()

	if callback != nil {
		go callback(event)
	}
}

// notifyFailoverEnd notifies failover end
func (fe *FailoverExecutor) notifyFailoverEnd(event *FailoverEvent) {
	fe.mu.RLock()
	callback := fe.onFailoverEnd
	fe.mu.RUnlock()

	if callback != nil {
		go callback(event)
	}
}

// HandleNodeFailure handles a node failure event
func (fe *FailoverExecutor) HandleNodeFailure(nodeID, reason string) {
	fe.logger.Error("handling node failure",
		zap.String("node_id", nodeID),
		zap.String("reason", reason))

	// Find best target for failover
	targetID := fe.selectFailoverTarget(nodeID)
	if targetID == "" {
		fe.logger.Error("no suitable failover target found")
		return
	}

	// Execute failover
	ctx, cancel := context.WithTimeout(context.Background(), fe.config.FailoverTimeout)
	defer cancel()

	if err := fe.ExecuteFailover(ctx, targetID, fmt.Sprintf("node failure: %s", reason)); err != nil {
		fe.logger.Error("automatic failover failed", zap.Error(err))
	}
}

// selectFailoverTarget selects the best target for failover
func (fe *FailoverExecutor) selectFailoverTarget(failedNodeID string) string {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	var bestTarget string
	bestPriority := -1

	for id, node := range fe.nodes {
		if id == failedNodeID || id == fe.localNodeID {
			continue
		}

		if !fe.detector.IsNodeHealthy(id) {
			continue
		}

		if node.Priority > bestPriority {
			bestPriority = node.Priority
			bestTarget = id
		}
	}

	// If no other node, check if local can take over
	if bestTarget == "" && fe.localNodeID != failedNodeID {
		if fe.detector.IsNodeHealthy(fe.localNodeID) {
			bestTarget = fe.localNodeID
		}
	}

	return bestTarget
}

// GetFailoverMetrics returns failover metrics
func (fe *FailoverExecutor) GetFailoverMetrics() *FailoverMetrics {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	metrics := &FailoverMetrics{
		TotalFailovers:   fe.metrics.TotalFailovers,
		Successful:       fe.metrics.Successful,
		Failed:           fe.metrics.Failed,
		AverageDuration:  fe.metrics.AverageDuration,
		LastFailoverTime: fe.metrics.LastFailoverTime,
		TotalSessions:    fe.metrics.TotalSessions,
		FailoverHistory:  make([]FailoverEvent, len(fe.eventHistory)),
	}
	copy(metrics.FailoverHistory, fe.eventHistory)

	if metrics.TotalFailovers > 0 {
		totalDuration := time.Duration(0)
		for _, event := range metrics.FailoverHistory {
			totalDuration += event.Duration
		}
		metrics.AverageDuration = totalDuration / time.Duration(metrics.TotalFailovers)
	}

	return metrics
}

// GetFailoverHistory returns recent failover events
func (fe *FailoverExecutor) GetFailoverHistory(limit int) []FailoverEvent {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	if limit <= 0 || limit > len(fe.eventHistory) {
		limit = len(fe.eventHistory)
	}

	start := len(fe.eventHistory) - limit
	return fe.eventHistory[start:]
}

// GetState returns the current failover state
func (fe *FailoverExecutor) GetState() FailoverState {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	return fe.state
}

// GetActiveNodeID returns the active node ID
func (fe *FailoverExecutor) GetActiveNodeID() string {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	return fe.activeNodeID
}

// AddVIP adds a virtual IP configuration
func (fe *FailoverExecutor) AddVIP(vipID string, config *VIPConfig) {
	fe.vipManager.mu.Lock()
	defer fe.vipManager.mu.Unlock()
	fe.vipManager.vips[vipID] = config
	fe.logger.Info("VIP added", zap.String("vip_id", vipID), zap.String("ip", config.IP))
}

// RemoveVIP removes a virtual IP configuration
func (fe *FailoverExecutor) RemoveVIP(vipID string) {
	fe.vipManager.mu.Lock()
	defer fe.vipManager.mu.Unlock()
	delete(fe.vipManager.vips, vipID)
	fe.logger.Info("VIP removed", zap.String("vip_id", vipID))
}

// GetVIPStatus returns status of all VIPs
func (fe *FailoverExecutor) GetVIPStatus() map[string]*VIPConfig {
	fe.vipManager.mu.RLock()
	defer fe.vipManager.mu.RUnlock()

	result := make(map[string]*VIPConfig, len(fe.vipManager.vips))
	for id, vip := range fe.vipManager.vips {
		copy := *vip
		result[id] = &copy
	}
	return result
}

// ManualFailover initiates a manual failover
func (fe *FailoverExecutor) ManualFailover(ctx context.Context, targetNodeID, reason string) error {
	fe.logger.Info("manual failover requested",
		zap.String("target", targetNodeID),
		zap.String("reason", reason))

	return fe.ExecuteFailover(ctx, targetNodeID, fmt.Sprintf("manual: %s", reason))
}

// CanFailover returns true if failover is possible
func (fe *FailoverExecutor) CanFailover() bool {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	if fe.state != FailoverStateIdle {
		return false
	}

	if !fe.detector.HasQuorum() {
		return false
	}

	// Check if there's a healthy target
	for id := range fe.nodes {
		if id != fe.localNodeID && fe.detector.IsNodeHealthy(id) {
			return true
		}
	}

	return false
}

// GetFailoverStatus returns overall failover status
func (fe *FailoverExecutor) GetFailoverStatus() map[string]interface{} {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	metrics := &FailoverMetrics{
		TotalFailovers:   fe.metrics.TotalFailovers,
		Successful:       fe.metrics.Successful,
		Failed:           fe.metrics.Failed,
		AverageDuration:  fe.metrics.AverageDuration,
		LastFailoverTime: fe.metrics.LastFailoverTime,
		TotalSessions:    fe.metrics.TotalSessions,
	}

	return map[string]interface{}{
		"state":         fe.state,
		"active_node":   fe.activeNodeID,
		"local_node":    fe.localNodeID,
		"can_failover":  fe.CanFailover(),
		"last_failover": fe.lastFailover,
		"metrics":       metrics,
	}
}
