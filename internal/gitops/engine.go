// Package gitops provides Git-based infrastructure management and deployment automation.
package gitops

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// GitClient is an interface for Git operations
type GitClient interface {
	// Clone clones a repository
	Clone(ctx context.Context, url, branch, path string, auth GitAuth) error
	// Pull pulls latest changes
	Pull(ctx context.Context, path string) error
	// GetCommitSHA returns the current commit SHA
	GetCommitSHA(ctx context.Context, path string) (string, error)
	// GetFileContent returns file content at a given revision
	GetFileContent(ctx context.Context, path, file, revision string) ([]byte, error)
	// ListFiles lists files in a directory at a given revision
	ListFiles(ctx context.Context, path, dir, revision string) ([]string, error)
}

// K8sClient is an interface for Kubernetes operations
type K8sClient interface {
	// Apply applies a manifest
	Apply(ctx context.Context, manifest []byte, namespace string) error
	// Delete deletes a resource
	Delete(ctx context.Context, kind, name, namespace string) error
	// GetStatus returns the status of a resource
	GetStatus(ctx context.Context, kind, name, namespace string) (string, error)
	// GetResource returns resource manifest
	GetResource(ctx context.Context, kind, name, namespace string) ([]byte, error)
}

// Engine is the main GitOps engine
type Engine struct {
	logger     *zap.Logger
	config     GitOpsConfig
	gitClient  GitClient
	k8sClient  K8sClient
	repos      map[string]*repoState
	deployments map[string]*Deployment
	syncStatus map[string]*SyncStatusDetail
	mu         sync.RWMutex
	stopCh     chan struct{}
}

// repoState tracks the state of a repository
type repoState struct {
	repo       GitRepo
	localPath  string
	lastSync   time.Time
	lastCommit string
	syncing    bool
}

// NewEngine creates a new GitOps engine
func NewEngine(logger *zap.Logger, config GitOpsConfig, gitClient GitClient, k8sClient K8sClient) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Engine{
		logger:      logger,
		config:      config,
		gitClient:   gitClient,
		k8sClient:   k8sClient,
		repos:       make(map[string]*repoState),
		deployments: make(map[string]*Deployment),
		syncStatus:  make(map[string]*SyncStatusDetail),
		stopCh:      make(chan struct{}),
	}
}

// Start starts the GitOps engine
func (e *Engine) Start(ctx context.Context) error {
	e.logger.Info("starting GitOps engine")

	// Initialize repositories
	for _, repo := range e.config.Repos {
		if err := e.initRepo(ctx, repo); err != nil {
			e.logger.Error("failed to init repo",
				zap.String("repo", repo.Name),
				zap.Error(err))
			continue
		}
	}

	// Start sync loop
	go e.syncLoop(ctx)

	return nil
}

// Stop stops the GitOps engine
func (e *Engine) Stop() {
	e.logger.Info("stopping GitOps engine")
	close(e.stopCh)
}

// initRepo initializes a repository
func (e *Engine) initRepo(ctx context.Context, repo GitRepo) error {
	localPath := fmt.Sprintf("/tmp/gitops/%s", repo.ID)

	e.logger.Info("initializing repo",
		zap.String("repo", repo.Name),
		zap.String("path", localPath))

	if err := e.gitClient.Clone(ctx, repo.URL, repo.Branch, localPath, repo.Auth); err != nil {
		return fmt.Errorf("failed to clone repo: %w", err)
	}

	sha, err := e.gitClient.GetCommitSHA(ctx, localPath)
	if err != nil {
		return fmt.Errorf("failed to get commit SHA: %w", err)
	}

	e.mu.Lock()
	e.repos[repo.ID] = &repoState{
		repo:       repo,
		localPath:  localPath,
		lastSync:   time.Now(),
		lastCommit: sha,
	}
	e.mu.Unlock()

	return nil
}

// syncLoop runs periodic syncs
func (e *Engine) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.syncAllRepos(ctx)
		}
	}
}

// syncAllRepos syncs all repositories
func (e *Engine) syncAllRepos(ctx context.Context) {
	e.mu.RLock()
	repoIDs := make([]string, 0, len(e.repos))
	for id := range e.repos {
		repoIDs = append(repoIDs, id)
	}
	e.mu.RUnlock()

	for _, id := range repoIDs {
		state := e.getRepoState(id)
		if state == nil || !state.repo.SyncPolicy.AutoSync {
			continue
		}

		// Check sync interval
		if time.Since(state.lastSync) < state.repo.SyncPolicy.SyncInterval {
			continue
		}

		if err := e.SyncRepo(ctx, id); err != nil {
			e.logger.Error("sync failed",
				zap.String("repo", id),
				zap.Error(err))
		}
	}
}

// SyncRepo syncs a specific repository
func (e *Engine) SyncRepo(ctx context.Context, repoID string) error {
	e.mu.Lock()
	state, exists := e.repos[repoID]
	if !exists {
		e.mu.Unlock()
		return fmt.Errorf("repo %s not found", repoID)
	}
	if state.syncing {
		e.mu.Unlock()
		return fmt.Errorf("repo %s is already syncing", repoID)
	}
	state.syncing = true
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		state.syncing = false
		e.mu.Unlock()
	}()

	e.logger.Info("syncing repo", zap.String("repo", repoID))

	// Pull latest changes
	if err := e.gitClient.Pull(ctx, state.localPath); err != nil {
		return fmt.Errorf("failed to pull: %w", err)
	}

	// Get new commit SHA
	newSHA, err := e.gitClient.GetCommitSHA(ctx, state.localPath)
	if err != nil {
		return fmt.Errorf("failed to get commit SHA: %w", err)
	}

	// Check if there are changes
	if newSHA == state.lastCommit {
		e.logger.Info("no changes detected", zap.String("repo", repoID))
		return nil
	}

	e.logger.Info("changes detected",
		zap.String("repo", repoID),
		zap.String("old_sha", state.lastCommit),
		zap.String("new_sha", newSHA))

	// Deploy to each environment
	startTime := time.Now()
	for _, env := range e.config.Environments {
		if err := e.deployToEnvironment(ctx, repoID, env, newSHA); err != nil {
			e.logger.Error("deployment failed",
				zap.String("repo", repoID),
				zap.String("env", string(env)),
				zap.Error(err))

			e.updateSyncStatus(repoID, env, SyncStatusError, err.Error(), startTime)
			continue
		}

		e.updateSyncStatus(repoID, env, SyncStatusSynced, "", startTime)
	}

	// Update state
	e.mu.Lock()
	state.lastCommit = newSHA
	state.lastSync = time.Now()
	e.mu.Unlock()

	return nil
}

// deployToEnvironment deploys manifests to an environment
func (e *Engine) deployToEnvironment(ctx context.Context, repoID string, env Environment, revision string) error {
	state := e.getRepoState(repoID)
	if state == nil {
		return fmt.Errorf("repo %s not found", repoID)
	}

	// List manifests for this environment
	envPath := fmt.Sprintf("%s/%s", state.repo.Path, env)
	files, err := e.gitClient.ListFiles(ctx, state.localPath, envPath, revision)
	if err != nil {
		// Environment directory might not exist
		e.logger.Warn("no manifests for environment",
			zap.String("env", string(env)),
			zap.Error(err))
		return nil
	}

	deployment := &Deployment{
		ID:          fmt.Sprintf("%s-%s-%s", repoID, env, time.Now().Format("20060102-150405")),
		RepoID:      repoID,
		Environment: env,
		Revision:    revision,
		Status:      DeploymentStatusRunning,
		StartedAt:   time.Now(),
		SyncStatus:  SyncStatusSyncing,
	}

	// Apply each manifest
	for _, file := range files {
		content, err := e.gitClient.GetFileContent(ctx, state.localPath, envPath+"/"+file, revision)
		if err != nil {
			e.logger.Warn("failed to read manifest",
				zap.String("file", file),
				zap.Error(err))
			continue
		}

		if err := e.k8sClient.Apply(ctx, content, string(env)); err != nil {
			deployment.Status = DeploymentStatusFailed
			deployment.Message = fmt.Sprintf("failed to apply %s: %v", file, err)

			e.saveDeployment(deployment)
			return err
		}

		deployment.Resources = append(deployment.Resources, Resource{
			Kind:      "Unknown", // Would parse from manifest
			Name:      file,
			Namespace: string(env),
			Status:    "healthy",
			Synced:    true,
		})
	}

	deployment.Status = DeploymentStatusSucceeded
	now := time.Now()
	deployment.CompletedAt = &now
	deployment.SyncStatus = SyncStatusSynced

	e.saveDeployment(deployment)

	e.logger.Info("deployment completed",
		zap.String("deployment_id", deployment.ID),
		zap.String("env", string(env)))

	return nil
}

// Rollback rolls back a deployment
func (e *Engine) Rollback(ctx context.Context, req RollbackRequest) (*Deployment, error) {
	e.mu.RLock()
	deployment, exists := e.deployments[req.DeploymentID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("deployment %s not found", req.DeploymentID)
	}

	// Get target revision
	targetRevision := req.Revision
	if targetRevision == "" {
		// Rollback to previous deployment
		prevDeployment := e.findPreviousDeployment(deployment.RepoID, deployment.Environment, deployment.ID)
		if prevDeployment == nil {
			return nil, fmt.Errorf("no previous deployment found")
		}
		targetRevision = prevDeployment.Revision
	}

	e.logger.Info("rolling back deployment",
		zap.String("deployment_id", req.DeploymentID),
		zap.String("target_revision", targetRevision))

	// Deploy the target revision
	if err := e.deployToEnvironment(ctx, deployment.RepoID, deployment.Environment, targetRevision); err != nil {
		return nil, fmt.Errorf("rollback failed: %w", err)
	}

	// Update original deployment status
	e.mu.Lock()
	deployment.Status = DeploymentStatusRolledBack
	deployment.RollbackID = req.DeploymentID
	e.mu.Unlock()

	// Get the new deployment
	newDeploymentID := fmt.Sprintf("%s-%s-%s", deployment.RepoID, deployment.Environment, time.Now().Format("20060102-150405"))
	return e.getDeployment(newDeploymentID), nil
}

// GetSyncStatus returns sync status for a repo/environment
func (e *Engine) GetSyncStatus(repoID string, env Environment) *SyncStatusDetail {
	key := fmt.Sprintf("%s/%s", repoID, env)
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.syncStatus[key]
}

// GetDeployment returns a deployment by ID
func (e *Engine) GetDeployment(id string) *Deployment {
	return e.getDeployment(id)
}

// ListDeployments returns deployments for a repo and environment
func (e *Engine) ListDeployments(repoID string, env Environment) []*Deployment {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Deployment
	for _, d := range e.deployments {
		if d.RepoID == repoID && d.Environment == env {
			result = append(result, d)
		}
	}
	return result
}

// GetRepo returns a repository by ID
func (e *Engine) GetRepo(id string) *GitRepo {
	state := e.getRepoState(id)
	if state == nil {
		return nil
	}
	repo := state.repo
	return &repo
}

// ListRepos returns all configured repositories
func (e *Engine) ListRepos() []GitRepo {
	e.mu.RLock()
	defer e.mu.RUnlock()

	repos := make([]GitRepo, 0, len(e.repos))
	for _, state := range e.repos {
		repos = append(repos, state.repo)
	}
	return repos
}

// AddRepo adds a new repository
func (e *Engine) AddRepo(req AddRepoRequest) (*GitRepo, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	repo := GitRepo{
		ID:     fmt.Sprintf("repo_%d", time.Now().UnixNano()),
		Name:   req.Name,
		URL:    req.URL,
		Branch: req.Branch,
		Path:   req.Path,
		Auth:   req.Auth,
		SyncPolicy: DefaultSyncPolicy(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if repo.Branch == "" {
		repo.Branch = "main"
	}

	state := &repoState{
		repo:     repo,
		localPath: fmt.Sprintf("/tmp/gitops/%s", repo.ID),
	}

	e.repos[repo.ID] = state
	return &repo, nil
}

// DetectDrift detects configuration drift for a repo/environment
func (e *Engine) DetectDrift(repoID string, env Environment) (*DriftDetection, error) {
	e.mu.RLock()
	state, exists := e.repos[repoID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("repo %s not found", repoID)
	}

	// Get current sync status
	syncStatus := e.GetSyncStatus(repoID, env)

	detection := &DriftDetection{
		ID:          fmt.Sprintf("drift_%d", time.Now().UnixNano()),
		RepoID:      repoID,
		Environment: env,
		DetectedAt:  time.Now(),
		Drifted:     false,
		Items:       make([]DriftItem, 0),
		Summary:     "No drift detected",
	}

	if syncStatus != nil && syncStatus.DriftDetected {
		detection.Drifted = true
		detection.Items = syncStatus.DriftDetails
		detection.Summary = fmt.Sprintf("Drift detected: %d items", len(detection.Items))
	} else if syncStatus == nil {
		// If no sync status, assume drift since we can't confirm
		detection.Drifted = true
		detection.Summary = "Sync status unknown, drift possible"
	}

	_ = state // use state if needed
	return detection, nil
}

// Helper methods

func (e *Engine) getRepoState(id string) *repoState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.repos[id]
}

func (e *Engine) getDeployment(id string) *Deployment {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.deployments[id]
}

func (e *Engine) saveDeployment(d *Deployment) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.deployments[d.ID] = d
}

func (e *Engine) updateSyncStatus(repoID string, env Environment, status SyncStatus, errMsg string, startTime time.Time) {
	key := fmt.Sprintf("%s/%s", repoID, env)
	state := e.getRepoState(repoID)

	e.mu.Lock()
	defer e.mu.Unlock()

	detail := &SyncStatusDetail{
		RepoID:       repoID,
		Environment:  env,
		Status:       status,
		LastSyncAt:   time.Now(),
		LastCommitSHA: state.lastCommit,
		DesiredSHA:   state.lastCommit,
		SyncDuration: time.Since(startTime),
		Error:        errMsg,
	}
	if state != nil {
		detail.RepoName = state.repo.Name
	}

	e.syncStatus[key] = detail
}

func (e *Engine) findPreviousDeployment(repoID string, env Environment, currentID string) *Deployment {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var prev *Deployment
	for _, d := range e.deployments {
		if d.RepoID == repoID && d.Environment == env && d.ID != currentID && d.Status == DeploymentStatusSucceeded {
			if prev == nil || d.CompletedAt.After(*prev.CompletedAt) {
				prev = d
			}
		}
	}
	return prev
}
