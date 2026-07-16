package gitops

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// MockGitClient implements GitClient for testing.
type MockGitClient struct {
	mock.Mock
}

func (m *MockGitClient) Clone(ctx context.Context, url, branch, path string, auth GitAuth) error {
	args := m.Called(ctx, url, branch, path, auth)
	return args.Error(0)
}

func (m *MockGitClient) Pull(ctx context.Context, path string) error {
	args := m.Called(ctx, path)
	return args.Error(0)
}

func (m *MockGitClient) GetCommitSHA(ctx context.Context, path string) (string, error) {
	args := m.Called(ctx, path)
	return args.String(0), args.Error(1)
}

func (m *MockGitClient) GetFileContent(ctx context.Context, path, file, revision string) ([]byte, error) {
	args := m.Called(ctx, path, file, revision)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockGitClient) ListFiles(ctx context.Context, path, dir, revision string) ([]string, error) {
	args := m.Called(ctx, path, dir, revision)
	return args.Get(0).([]string), args.Error(1)
}

// MockK8sClient implements K8sClient for testing.
type MockK8sClient struct {
	mock.Mock
}

func (m *MockK8sClient) Apply(ctx context.Context, manifest []byte, namespace string) error {
	args := m.Called(ctx, manifest, namespace)
	return args.Error(0)
}

func (m *MockK8sClient) Delete(ctx context.Context, kind, name, namespace string) error {
	args := m.Called(ctx, kind, name, namespace)
	return args.Error(0)
}

func (m *MockK8sClient) GetStatus(ctx context.Context, kind, name, namespace string) (string, error) {
	args := m.Called(ctx, kind, name, namespace)
	return args.String(0), args.Error(1)
}

func (m *MockK8sClient) GetResource(ctx context.Context, kind, name, namespace string) ([]byte, error) {
	args := m.Called(ctx, kind, name, namespace)
	return args.Get(0).([]byte), args.Error(1)
}

func TestEngine_Start(t *testing.T) {
	logger := zap.NewNop()
	gitClient := new(MockGitClient)
	k8sClient := new(MockK8sClient)

	config := GitOpsConfig{
		Repos: []GitRepo{
			{
				ID:     "test-repo",
				Name:   "Test Repo",
				URL:    "https://github.com/test/repo.git",
				Branch: "main",
				Path:   "manifests",
			},
		},
		Environments: []Environment{EnvDev, EnvStaging},
	}

	// Mock git clone
	gitClient.On("Clone", mock.Anything, "https://github.com/test/repo.git", "main", "/tmp/gitops/test-repo", mock.Anything).Return(nil)
	gitClient.On("GetCommitSHA", mock.Anything, "/tmp/gitops/test-repo").Return("abc123", nil)

	engine := NewEngine(logger, config, gitClient, k8sClient)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := engine.Start(ctx)
	require.NoError(t, err)

	// Verify repo was initialized
	repo := engine.GetRepo("test-repo")
	assert.NotNil(t, repo)
	assert.Equal(t, "Test Repo", repo.Name)

	engine.Stop()
	gitClient.AssertExpectations(t)
}

func TestEngine_SyncRepo(t *testing.T) {
	logger := zap.NewNop()
	gitClient := new(MockGitClient)
	k8sClient := new(MockK8sClient)

	config := GitOpsConfig{
		Repos: []GitRepo{
			{
				ID:     "test-repo",
				Name:   "Test Repo",
				URL:    "https://github.com/test/repo.git",
				Branch: "main",
				Path:   "manifests",
			},
		},
		Environments: []Environment{EnvDev},
	}

	// Mock git operations
	gitClient.On("Clone", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	gitClient.On("GetCommitSHA", mock.Anything, mock.Anything).Return("abc123", nil).Once()
	gitClient.On("Pull", mock.Anything, mock.Anything).Return(nil)
	gitClient.On("GetCommitSHA", mock.Anything, mock.Anything).Return("def456", nil).Once()
	gitClient.On("ListFiles", mock.Anything, mock.Anything, "manifests/dev", "def456").Return([]string{"deployment.yaml"}, nil)
	gitClient.On("GetFileContent", mock.Anything, mock.Anything, "manifests/dev/deployment.yaml", "def456").Return([]byte("apiVersion: v1"), nil)

	// Mock k8s apply
	k8sClient.On("Apply", mock.Anything, mock.Anything, "dev").Return(nil)

	engine := NewEngine(logger, config, gitClient, k8sClient)

	ctx := context.Background()
	err := engine.Start(ctx)
	require.NoError(t, err)

	// Trigger sync
	err = engine.SyncRepo(ctx, "test-repo")
	require.NoError(t, err)

	// Verify deployment was created
	deployments := engine.ListDeployments("test-repo", EnvDev)
	assert.Len(t, deployments, 1)
	assert.Equal(t, DeploymentStatusSucceeded, deployments[0].Status)

	engine.Stop()
	gitClient.AssertExpectations(t)
	k8sClient.AssertExpectations(t)
}

func TestEngine_GetSyncStatus(t *testing.T) {
	logger := zap.NewNop()
	engine := NewEngine(logger, GitOpsConfig{}, nil, nil)

	// Initially no status
	status := engine.GetSyncStatus("repo1", EnvDev)
	assert.Nil(t, status)

	// Manually set status
	engine.mu.Lock()
	engine.syncStatus["repo1/dev"] = &SyncStatusDetail{
		RepoID:      "repo1",
		Environment: EnvDev,
		Status:      SyncStatusSynced,
	}
	engine.mu.Unlock()

	status = engine.GetSyncStatus("repo1", EnvDev)
	assert.NotNil(t, status)
	assert.Equal(t, SyncStatusSynced, status.Status)
}

func TestEngine_Rollback(t *testing.T) {
	logger := zap.NewNop()
	gitClient := new(MockGitClient)
	k8sClient := new(MockK8sClient)

	config := GitOpsConfig{
		Repos: []GitRepo{
			{
				ID:   "test-repo",
				Name: "Test Repo",
				Path: "manifests",
			},
		},
		Environments: []Environment{EnvDev},
	}

	engine := NewEngine(logger, config, gitClient, k8sClient)

	// Add a previous successful deployment
	engine.mu.Lock()
	engine.deployments["old-deployment"] = &Deployment{
		ID:          "old-deployment",
		RepoID:      "test-repo",
		Environment: EnvDev,
		Revision:    "old-sha",
		Status:      DeploymentStatusSucceeded,
		CompletedAt: timePtr(time.Now().Add(-1 * time.Hour)),
	}
	engine.deployments["current-deployment"] = &Deployment{
		ID:          "current-deployment",
		RepoID:      "test-repo",
		Environment: EnvDev,
		Revision:    "new-sha",
		Status:      DeploymentStatusSucceeded,
		CompletedAt: timePtr(time.Now()),
	}
	engine.repos["test-repo"] = &repoState{
		repo:       config.Repos[0],
		localPath:  "/tmp/gitops/test-repo",
		lastCommit: "new-sha",
	}
	engine.mu.Unlock()

	// Mock for rollback deployment
	gitClient.On("ListFiles", mock.Anything, mock.Anything, "manifests/dev", "old-sha").Return([]string{"deployment.yaml"}, nil)
	gitClient.On("GetFileContent", mock.Anything, mock.Anything, "manifests/dev/deployment.yaml", "old-sha").Return([]byte("apiVersion: v1"), nil)
	k8sClient.On("Apply", mock.Anything, mock.Anything, "dev").Return(nil)

	ctx := context.Background()
	deployment, err := engine.Rollback(ctx, RollbackRequest{
		DeploymentID: "current-deployment",
	})

	require.NoError(t, err)
	assert.NotNil(t, deployment)
	assert.Equal(t, DeploymentStatusSucceeded, deployment.Status)
	assert.Equal(t, "old-sha", deployment.Revision)

	// Verify old deployment was marked as rolled back
	old := engine.GetDeployment("current-deployment")
	assert.Equal(t, DeploymentStatusRolledBack, old.Status)
}

func TestReconciler_DetectDrift(t *testing.T) {
	logger := zap.NewNop()
	engine := NewEngine(logger, GitOpsConfig{}, nil, nil)
	reconciler := NewReconciler(logger, engine, time.Minute)

	// Test with missing desired resources
	desired := []desiredResource{
		{Kind: "Deployment", Name: "app", Namespace: "dev", Content: []byte("spec")},
	}
	actual := []actualResource{}

	drifts := reconciler.detectDrift(desired, actual)
	assert.Len(t, drifts, 1)
	assert.Equal(t, "create", drifts[0].Action)

	// Test with extra actual resources
	desired = []desiredResource{}
	actual = []actualResource{
		{Kind: "Service", Name: "old-svc", Namespace: "dev"},
	}

	drifts = reconciler.detectDrift(desired, actual)
	assert.Len(t, drifts, 1)
	assert.Equal(t, "delete", drifts[0].Action)

	// Test with modified resources
	desired = []desiredResource{
		{Kind: "Deployment", Name: "app", Namespace: "dev", Content: []byte("new-spec")},
	}
	actual = []actualResource{
		{Kind: "Deployment", Name: "app", Namespace: "dev", Content: []byte("old-spec")},
	}

	drifts = reconciler.detectDrift(desired, actual)
	assert.Len(t, drifts, 1)
	assert.Equal(t, "update", drifts[0].Action)

	// Test with no drift
	desired = []desiredResource{
		{Kind: "Deployment", Name: "app", Namespace: "dev", Content: []byte("spec")},
	}
	actual = []actualResource{
		{Kind: "Deployment", Name: "app", Namespace: "dev", Content: []byte("spec")},
	}

	drifts = reconciler.detectDrift(desired, actual)
	assert.Len(t, drifts, 0)
}

func TestDefaultSyncPolicy(t *testing.T) {
	policy := DefaultSyncPolicy()

	assert.True(t, policy.AutoSync)
	assert.Equal(t, 5*time.Minute, policy.SyncInterval)
	assert.True(t, policy.Prune)
	assert.True(t, policy.SelfHeal)
	assert.Equal(t, 3, policy.RetryLimit)
}

func TestEngine_ListRepos(t *testing.T) {
	logger := zap.NewNop()
	engine := NewEngine(logger, GitOpsConfig{}, nil, nil)

	engine.mu.Lock()
	engine.repos["repo1"] = &repoState{
		repo: GitRepo{ID: "repo1", Name: "Repo 1"},
	}
	engine.repos["repo2"] = &repoState{
		repo: GitRepo{ID: "repo2", Name: "Repo 2"},
	}
	engine.mu.Unlock()

	repos := engine.ListRepos()
	assert.Len(t, repos, 2)
}

func TestEngine_ListDeployments(t *testing.T) {
	logger := zap.NewNop()
	engine := NewEngine(logger, GitOpsConfig{}, nil, nil)

	engine.mu.Lock()
	engine.deployments["d1"] = &Deployment{
		ID:          "d1",
		RepoID:      "repo1",
		Environment: EnvDev,
		Status:      DeploymentStatusSucceeded,
	}
	engine.deployments["d2"] = &Deployment{
		ID:          "d2",
		RepoID:      "repo1",
		Environment: EnvProd,
		Status:      DeploymentStatusRunning,
	}
	engine.mu.Unlock()

	deployments := engine.ListDeployments("repo1", EnvDev)
	assert.Len(t, deployments, 1)
	assert.Equal(t, "d1", deployments[0].ID)
}

func TestReconciler_GetDriftSummary(t *testing.T) {
	logger := zap.NewNop()
	engine := NewEngine(logger, GitOpsConfig{}, nil, nil)
	reconciler := NewReconciler(logger, engine, time.Minute)

	engine.mu.Lock()
	engine.syncStatus["repo1/dev"] = &SyncStatusDetail{
		RepoID:        "repo1",
		Environment:   EnvDev,
		DriftDetected: true,
		DriftDetails:  []DriftItem{{ResourceKind: "Deployment"}, {ResourceKind: "Service"}},
	}
	engine.mu.Unlock()

	summary := reconciler.GetDriftSummary()
	assert.Len(t, summary, 1)
	assert.Equal(t, 2, summary["repo1/dev"])
}

func timePtr(t time.Time) *time.Time {
	return &t
}
