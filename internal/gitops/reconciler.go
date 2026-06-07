// Package gitops provides Git-based infrastructure management and deployment automation.
package gitops

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Reconciler compares desired state (Git) with actual state (cluster)
type Reconciler struct {
	logger     *zap.Logger
	engine     *Engine
	interval   time.Duration
	mu         sync.RWMutex
	driftItems []DriftItem
	stopCh     chan struct{}
}

// NewReconciler creates a new reconciler
func NewReconciler(logger *zap.Logger, engine *Engine, interval time.Duration) *Reconciler {
	if logger == nil {
		logger = zap.NewNop()
	}
	if interval == 0 {
		interval = 3 * time.Minute
	}
	return &Reconciler{
		logger:   logger,
		engine:   engine,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start starts the reconciler
func (r *Reconciler) Start(ctx context.Context) {
	r.logger.Info("starting reconciler")
	go r.reconcileLoop(ctx)
}

// Stop stops the reconciler
func (r *Reconciler) Stop() {
	r.logger.Info("stopping reconciler")
	close(r.stopCh)
}

// reconcileLoop runs periodic reconciliation
func (r *Reconciler) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.ReconcileAll(ctx)
		}
	}
}

// ReconcileAll reconciles all repositories and environments
func (r *Reconciler) ReconcileAll(ctx context.Context) {
	repos := r.engine.ListRepos()

	for _, repo := range repos {
		for _, env := range r.engine.config.Environments {
			if err := r.Reconcile(ctx, repo.ID, env); err != nil {
				r.logger.Error("reconciliation failed",
					zap.String("repo", repo.ID),
					zap.String("env", string(env)),
					zap.Error(err))
			}
		}
	}
}

// Reconcile checks for drift between desired and actual state
func (r *Reconciler) Reconcile(ctx context.Context, repoID string, env Environment) error {
	r.logger.Info("reconciling",
		zap.String("repo", repoID),
		zap.String("env", string(env)))

	repo := r.engine.GetRepo(repoID)
	if repo == nil {
		return fmt.Errorf("repo %s not found", repoID)
	}

	// Get desired state from Git
	desiredResources, err := r.getDesiredState(ctx, repo, env)
	if err != nil {
		return fmt.Errorf("failed to get desired state: %w", err)
	}

	// Get actual state from cluster
	actualResources, err := r.getActualState(ctx, env)
	if err != nil {
		return fmt.Errorf("failed to get actual state: %w", err)
	}

	// Compare and detect drift
	drifts := r.detectDrift(desiredResources, actualResources)

	r.mu.Lock()
	r.driftItems = drifts
	r.mu.Unlock()

	if len(drifts) > 0 {
		r.logger.Warn("drift detected",
			zap.String("repo", repoID),
			zap.String("env", string(env)),
			zap.Int("drift_count", len(drifts)))

		// Update sync status
		r.updateDriftStatus(repoID, env, drifts)

		// Auto-heal if enabled
		if repo.SyncPolicy.SelfHeal {
			r.logger.Info("auto-healing drift",
				zap.String("repo", repoID),
				zap.String("env", string(env)))
			return r.engine.SyncRepo(ctx, repoID)
		}
	} else {
		r.logger.Debug("no drift detected",
			zap.String("repo", repoID),
			zap.String("env", string(env)))
	}

	return nil
}

// desiredResource represents a resource from Git
type desiredResource struct {
	Kind      string
	Name      string
	Namespace string
	Content   []byte
}

// actualResource represents a resource from the cluster
type actualResource struct {
	Kind      string
	Name      string
	Namespace string
	Content   []byte
}

// getDesiredState reads desired state from Git
func (r *Reconciler) getDesiredState(ctx context.Context, repo *GitRepo, env Environment) ([]desiredResource, error) {
	state := r.engine.getRepoState(repo.ID)
	if state == nil {
		return nil, fmt.Errorf("repo state not found")
	}

	envPath := fmt.Sprintf("%s/%s", repo.Path, env)
	files, err := r.engine.gitClient.ListFiles(ctx, state.localPath, envPath, state.lastCommit)
	if err != nil {
		return nil, err
	}

	var resources []desiredResource
	for _, file := range files {
		content, err := r.engine.gitClient.GetFileContent(ctx, state.localPath, envPath+"/"+file, state.lastCommit)
		if err != nil {
			r.logger.Warn("failed to read file",
				zap.String("file", file),
				zap.Error(err))
			continue
		}

		// Parse manifest (simplified)
		resources = append(resources, desiredResource{
			Kind:      "Unknown",
			Name:      file,
			Namespace: string(env),
			Content:   content,
		})
	}

	return resources, nil
}

// getActualState reads actual state from cluster
func (r *Reconciler) getActualState(ctx context.Context, env Environment) ([]actualResource, error) {
	// In production, this would list all resources in the namespace
	// For now, return empty (no drift)
	return []actualResource{}, nil
}

// detectDrift compares desired and actual state
func (r *Reconciler) detectDrift(desired []desiredResource, actual []actualResource) []DriftItem {
	var drifts []DriftItem

	// Build map of actual resources
	actualMap := make(map[string]actualResource)
	for _, a := range actual {
		key := fmt.Sprintf("%s/%s/%s", a.Namespace, a.Kind, a.Name)
		actualMap[key] = a
	}

	// Check desired resources
	for _, d := range desired {
		key := fmt.Sprintf("%s/%s/%s", d.Namespace, d.Kind, d.Name)
		actual, exists := actualMap[key]

		if !exists {
			// Resource missing - needs creation
			drifts = append(drifts, DriftItem{
				ResourceKind: d.Kind,
				ResourceName: d.Name,
				Field:        "existence",
				DesiredValue: "exists",
				ActualValue:  "missing",
				Action:       "create",
			})
			continue
		}

		// Compare content (simplified)
		if string(d.Content) != string(actual.Content) {
			drifts = append(drifts, DriftItem{
				ResourceKind: d.Kind,
				ResourceName: d.Name,
				Field:        "spec",
				DesiredValue: "modified",
				ActualValue:  "different",
				Action:       "update",
			})
		}

		// Remove from map to track extras
		delete(actualMap, key)
	}

	// Remaining actual resources should be deleted
	for _, a := range actualMap {
		drifts = append(drifts, DriftItem{
			ResourceKind: a.Kind,
			ResourceName: a.Name,
			Field:        "existence",
			DesiredValue: "missing",
			ActualValue:  "exists",
			Action:       "delete",
		})
	}

	return drifts
}

// updateDriftStatus updates sync status with drift information
func (r *Reconciler) updateDriftStatus(repoID string, env Environment, drifts []DriftItem) {
	key := fmt.Sprintf("%s/%s", repoID, env)
	state := r.engine.getRepoState(repoID)

	r.engine.mu.Lock()
	defer r.engine.mu.Unlock()

	detail := &SyncStatusDetail{
		RepoID:        repoID,
		Environment:   env,
		Status:        SyncStatusOutOfSync,
		LastSyncAt:    time.Now(),
		DriftDetected: true,
		DriftDetails:  drifts,
	}
	if state != nil {
		detail.RepoName = state.repo.Name
		detail.LastCommitSHA = state.lastCommit
		detail.DesiredSHA = state.lastCommit
	}

	r.engine.syncStatus[key] = detail
}

// GetDriftItems returns current drift items
func (r *Reconciler) GetDriftItems() []DriftItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.driftItems
}

// GetDriftSummary returns a summary of drift for all repos/envs
func (r *Reconciler) GetDriftSummary() map[string]int {
	r.engine.mu.RLock()
	defer r.engine.mu.RUnlock()

	summary := make(map[string]int)
	for key, status := range r.engine.syncStatus {
		if status.DriftDetected {
			summary[key] = len(status.DriftDetails)
		}
	}
	return summary
}
