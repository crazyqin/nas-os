// Package gitopsmgr provides GitOps management with repository connection,
// config synchronization, drift detection, rollback, and deployment history.
package gitopsmgr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager manages GitOps repositories, sync operations, and deployments
type Manager struct {
	logger      *zap.Logger
	repos       map[string]*GitRepo
	repoMu      sync.RWMutex
	deployments map[string]*DeploymentState
	deployMu    sync.RWMutex
	drifts      map[string][]*DriftDetection
	driftMu     sync.RWMutex
	history     map[string][]*DeploymentState
	historyMu   sync.RWMutex
	stopCh      chan struct{}
}

// NewManager creates a new GitOps manager
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		logger:      logger,
		repos:       make(map[string]*GitRepo),
		deployments: make(map[string]*DeploymentState),
		drifts:      make(map[string][]*DriftDetection),
		history:     make(map[string][]*DeploymentState),
		stopCh:      make(chan struct{}),
	}
}

// Start begins the background sync loop
func (m *Manager) Start(ctx context.Context) {
	go m.runSyncLoop(ctx)
	m.logger.Info("gitops manager started")
}

// Stop stops the background sync loop
func (m *Manager) Stop() {
	close(m.stopCh)
	m.logger.Info("gitops manager stopped")
}

// ConnectRepo connects a new Git repository
func (m *Manager) ConnectRepo(repo *GitRepo) error {
	if repo.ID == "" {
		return fmt.Errorf("repo ID is required")
	}
	if repo.URL == "" {
		return fmt.Errorf("repo URL is required")
	}

	m.repoMu.Lock()
	defer m.repoMu.Unlock()

	if _, exists := m.repos[repo.ID]; exists {
		return fmt.Errorf("repo %s already connected", repo.ID)
	}

	now := time.Now()
	repo.Connected = true
	repo.CreatedAt = now
	repo.UpdatedAt = now
	if repo.Branch == "" {
		repo.Branch = "main"
	}
	if repo.SyncPolicy.Interval == 0 {
		repo.SyncPolicy = DefaultSyncPolicy()
	}

	m.repos[repo.ID] = repo

	m.logger.Info("repo connected",
		zap.String("repo_id", repo.ID),
		zap.String("url", repo.URL),
		zap.String("branch", repo.Branch))

	return nil
}

// SyncConfig synchronizes configuration from a repository
func (m *Manager) SyncConfig(ctx context.Context, repoID string, force bool) (*DeploymentState, error) {
	m.repoMu.RLock()
	repo, exists := m.repos[repoID]
	m.repoMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("repo %s not found", repoID)
	}

	// Simulate sync operation
	deployment := &DeploymentState{
		ID:        fmt.Sprintf("deploy-%s-%d", repoID, time.Now().UnixNano()),
		RepoID:    repoID,
		CommitSHA: fmt.Sprintf("abc%d", time.Now().UnixNano()%10000),
		Version:   time.Now().Format("20060102-150405"),
		Status:    DeploymentStatusRunning,
		SyncedAt:  time.Now(),
		Message:   "sync in progress",
	}

	// Store deployment
	m.deployMu.Lock()
	m.deployments[deployment.ID] = deployment
	m.deployMu.Unlock()

	// Simulate completion
	go func() {
		time.Sleep(100 * time.Millisecond)
		deployment.Status = DeploymentStatusSucceeded
		deployment.Message = "sync completed"

		// Update last sync
		m.repoMu.Lock()
		if r, ok := m.repos[repoID]; ok {
			now := time.Now()
			r.LastSync = &now
			r.UpdatedAt = now
		}
		m.repoMu.Unlock()

		// Add to history
		m.historyMu.Lock()
		m.history[repoID] = append(m.history[repoID], deployment)
		m.historyMu.Unlock()

		m.logger.Info("sync completed",
			zap.String("repo_id", repoID),
			zap.String("commit", deployment.CommitSHA))
	}()

	_ = repo
	return deployment, nil
}

// DetectDrift detects configuration drift for a repository
func (m *Manager) DetectDrift(ctx context.Context, repoID string) ([]*DriftDetection, error) {
	m.repoMu.RLock()
	_, exists := m.repos[repoID]
	m.repoMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("repo %s not found", repoID)
	}

	// Simulate drift detection
	drifts := make([]*DriftDetection, 0)

	// Check if we have previous drifts
	m.driftMu.RLock()
	existing := m.drifts[repoID]
	m.driftMu.RUnlock()

	// Return existing unresolved drifts
	for _, d := range existing {
		if d.ResolvedAt == nil {
			drifts = append(drifts, d)
		}
	}

	m.logger.Info("drift detection completed",
		zap.String("repo_id", repoID),
		zap.Int("drifts_found", len(drifts)))

	return drifts, nil
}

// Rollback rolls back to a previous deployment
func (m *Manager) Rollback(ctx context.Context, req RollbackRequest) (*DeploymentState, error) {
	m.deployMu.RLock()
	original, exists := m.deployments[req.DeploymentID]
	m.deployMu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("deployment %s not found", req.DeploymentID)
	}

	// Create rollback deployment
	rollback := &DeploymentState{
		ID:           fmt.Sprintf("rollback-%s-%d", req.DeploymentID, time.Now().UnixNano()),
		RepoID:       original.RepoID,
		CommitSHA:    original.CommitSHA,
		Version:      original.Version,
		Status:       DeploymentStatusRunning,
		Resources:    original.Resources,
		SyncedAt:     time.Now(),
		RollbackFrom: req.DeploymentID,
		Message:      fmt.Sprintf("rolling back: %s", req.Reason),
	}

	// Mark original as rolled back
	m.deployMu.Lock()
	original.Status = DeploymentStatusRolledBack
	now := time.Now()
	original.RolledBackAt = &now
	m.deployments[rollback.ID] = rollback
	m.deployMu.Unlock()

	// Simulate rollback completion
	go func() {
		time.Sleep(100 * time.Millisecond)
		rollback.Status = DeploymentStatusSucceeded
		rollback.Message = "rollback completed"

		m.historyMu.Lock()
		m.history[original.RepoID] = append(m.history[original.RepoID], rollback)
		m.historyMu.Unlock()

		m.logger.Info("rollback completed",
			zap.String("deployment_id", req.DeploymentID),
			zap.String("rollback_id", rollback.ID))
	}()

	return rollback, nil
}

// GetHistory returns deployment history for a repository
func (m *Manager) GetHistory(repoID string, limit int) []*DeploymentState {
	m.historyMu.RLock()
	defer m.historyMu.RUnlock()

	deployments := m.history[repoID]
	if limit > 0 && limit < len(deployments) {
		return deployments[len(deployments)-limit:]
	}
	return deployments
}

// GetRepo returns a repository by ID
func (m *Manager) GetRepo(id string) *GitRepo {
	m.repoMu.RLock()
	defer m.repoMu.RUnlock()
	return m.repos[id]
}

// ListRepos returns all connected repositories
func (m *Manager) ListRepos() []*GitRepo {
	m.repoMu.RLock()
	defer m.repoMu.RUnlock()

	repos := make([]*GitRepo, 0, len(m.repos))
	for _, r := range m.repos {
		repos = append(repos, r)
	}
	return repos
}

// DeleteRepo disconnects a repository
func (m *Manager) DeleteRepo(id string) bool {
	m.repoMu.Lock()
	defer m.repoMu.Unlock()

	if _, exists := m.repos[id]; !exists {
		return false
	}
	delete(m.repos, id)

	m.logger.Info("repo disconnected", zap.String("repo_id", id))
	return true
}

// GetDeployment returns a deployment by ID
func (m *Manager) GetDeployment(id string) *DeploymentState {
	m.deployMu.RLock()
	defer m.deployMu.RUnlock()
	return m.deployments[id]
}

// ListDeployments returns all deployments
func (m *Manager) ListDeployments() []*DeploymentState {
	m.deployMu.RLock()
	defer m.deployMu.RUnlock()

	deployments := make([]*DeploymentState, 0, len(m.deployments))
	for _, d := range m.deployments {
		deployments = append(deployments, d)
	}
	return deployments
}

// AddDrift manually adds a drift detection entry
func (m *Manager) AddDrift(drift *DriftDetection) {
	m.driftMu.Lock()
	defer m.driftMu.Unlock()

	drift.DetectedAt = time.Now()
	m.drifts[drift.RepoID] = append(m.drifts[drift.RepoID], drift)

	m.logger.Info("drift added",
		zap.String("repo_id", drift.RepoID),
		zap.String("resource", drift.ResourceName),
		zap.String("severity", string(drift.Severity)))
}

// ResolveDrift marks a drift as resolved
func (m *Manager) ResolveDrift(repoID, driftID string) bool {
	m.driftMu.Lock()
	defer m.driftMu.Unlock()

	drifts := m.drifts[repoID]
	for _, d := range drifts {
		if d.ID == driftID {
			now := time.Now()
			d.ResolvedAt = &now
			return true
		}
	}
	return false
}

// runSyncLoop periodically syncs repositories with auto-sync enabled
func (m *Manager) runSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.runAutoSync(ctx)
		}
	}
}

// runAutoSync syncs all repos with auto-sync enabled
func (m *Manager) runAutoSync(ctx context.Context) {
	m.repoMu.RLock()
	var autoSyncRepos []*GitRepo
	for _, repo := range m.repos {
		if repo.Connected && repo.SyncPolicy.AutoSync {
			autoSyncRepos = append(autoSyncRepos, repo)
		}
	}
	m.repoMu.RUnlock()

	for _, repo := range autoSyncRepos {
		if repo.LastSync != nil && time.Since(*repo.LastSync) < repo.SyncPolicy.Interval {
			continue
		}

		m.logger.Info("auto-sync triggered", zap.String("repo_id", repo.ID))
		if _, err := m.SyncConfig(ctx, repo.ID, false); err != nil {
			m.logger.Error("auto-sync failed",
				zap.String("repo_id", repo.ID),
				zap.Error(err))
		}
	}
}
