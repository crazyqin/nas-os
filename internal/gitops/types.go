// Package gitops provides Git-based infrastructure management and deployment automation.
package gitops

import (
	"time"
)

// Environment represents a deployment environment
type Environment string

const (
	EnvDev     Environment = "dev"
	EnvStaging Environment = "staging"
	EnvProd    Environment = "prod"
)

// SyncStatus represents the synchronization state
type SyncStatus string

const (
	SyncStatusSynced    SyncStatus = "synced"
	SyncStatusOutOfSync SyncStatus = "out_of_sync"
	SyncStatusUnknown   SyncStatus = "unknown"
	SyncStatusError     SyncStatus = "error"
	SyncStatusSyncing   SyncStatus = "syncing"
)

// DeploymentStatus represents the state of a deployment
type DeploymentStatus string

const (
	DeploymentStatusPending    DeploymentStatus = "pending"
	DeploymentStatusRunning    DeploymentStatus = "running"
	DeploymentStatusSucceeded  DeploymentStatus = "succeeded"
	DeploymentStatusFailed     DeploymentStatus = "failed"
	DeploymentStatusRolledBack DeploymentStatus = "rolled_back"
)

// GitRepo represents a Git repository configuration
type GitRepo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	URL        string     `json:"url"`
	Branch     string     `json:"branch"` // default: main
	Path       string     `json:"path"`   // path within repo
	Auth       GitAuth    `json:"auth"`
	SyncPolicy SyncPolicy `json:"sync_policy"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// GitAuth contains authentication credentials for Git
type GitAuth struct {
	Type     string `json:"type"` // ssh, token, basic
	Token    string `json:"token,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	SSHKey   string `json:"ssh_key,omitempty"`
}

// SyncPolicy defines how and when to sync
type SyncPolicy struct {
	AutoSync     bool          `json:"auto_sync"`
	SyncInterval time.Duration `json:"sync_interval"` // e.g. 5m
	Prune        bool          `json:"prune"`         // delete resources not in git
	SelfHeal     bool          `json:"self_heal"`     // auto-fix drift
	RetryLimit   int           `json:"retry_limit"`
}

// DefaultSyncPolicy returns a sensible default sync policy
func DefaultSyncPolicy() SyncPolicy {
	return SyncPolicy{
		AutoSync:     true,
		SyncInterval: 5 * time.Minute,
		Prune:        true,
		SelfHeal:     true,
		RetryLimit:   3,
	}
}

// Deployment represents a deployment to an environment
type Deployment struct {
	ID          string           `json:"id"`
	RepoID      string           `json:"repo_id"`
	Environment Environment      `json:"environment"`
	Revision    string           `json:"revision"` // git commit SHA
	Status      DeploymentStatus `json:"status"`
	Resources   []Resource       `json:"resources"`
	SyncStatus  SyncStatus       `json:"sync_status"`
	StartedAt   time.Time        `json:"started_at"`
	CompletedAt *time.Time       `json:"completed_at,omitempty"`
	Message     string           `json:"message"`
	RollbackID  string           `json:"rollback_id,omitempty"` // ID of deployment to rollback to
}

// Resource represents a deployed resource
type Resource struct {
	Kind      string `json:"kind"` // Deployment, Service, ConfigMap, etc.
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Status    string `json:"status"` // healthy, degraded, missing
	Synced    bool   `json:"synced"`
}

// SyncStatusDetail contains detailed sync information
type SyncStatusDetail struct {
	RepoID        string        `json:"repo_id"`
	RepoName      string        `json:"repo_name"`
	Environment   Environment   `json:"environment"`
	Status        SyncStatus    `json:"status"`
	LastSyncAt    time.Time     `json:"last_sync_at"`
	LastCommitSHA string        `json:"last_commit_sha"`
	DesiredSHA    string        `json:"desired_sha"` // target commit
	DriftDetected bool          `json:"drift_detected"`
	DriftDetails  []DriftItem   `json:"drift_details,omitempty"`
	SyncDuration  time.Duration `json:"sync_duration"`
	Error         string        `json:"error,omitempty"`
}

// DriftItem represents a detected drift between desired and actual state
type DriftItem struct {
	ResourceKind string `json:"resource_kind"`
	ResourceName string `json:"resource_name"`
	Field        string `json:"field"`
	DesiredValue string `json:"desired_value"`
	ActualValue  string `json:"actual_value"`
	Action       string `json:"action"` // create, update, delete
}

// GitOpsConfig is the main configuration for the GitOps engine
type GitOpsConfig struct {
	Repos         []GitRepo          `json:"repos"`
	Environments  []Environment      `json:"environments"`
	DefaultPolicy SyncPolicy         `json:"default_policy"`
	Webhooks      []Webhook          `json:"webhooks,omitempty"`
	Notifications NotificationConfig `json:"notifications"`
}

// Webhook configuration for Git events
type Webhook struct {
	ID      string   `json:"id"`
	URL     string   `json:"url"`
	Secret  string   `json:"secret,omitempty"`
	Events  []string `json:"events"` // push, tag, pr
	Enabled bool     `json:"enabled"`
}

// NotificationConfig defines how to send notifications
type NotificationConfig struct {
	Enabled   bool     `json:"enabled"`
	Channels  []string `json:"channels"` // slack, email, webhook
	OnError   bool     `json:"on_error"`
	OnSuccess bool     `json:"on_success"`
	OnDrift   bool     `json:"on_drift"`
}

// RollbackRequest is the API request for rolling back a deployment
type RollbackRequest struct {
	DeploymentID string `json:"deployment_id" binding:"required"`
	Revision     string `json:"revision,omitempty"` // specific revision, empty = previous
}

// SyncRequest is the API request for triggering a sync
type SyncRequest struct {
	RepoID      string      `json:"repo_id" binding:"required"`
	Environment Environment `json:"environment" binding:"required"`
	Force       bool        `json:"force"`              // force sync even if no changes
	Revision    string      `json:"revision,omitempty"` // specific revision
}

// ========== GitOps 增强类型 ==========

// DriftDetection 漂移检测结果
type DriftDetection struct {
	ID          string      `json:"id"`
	RepoID      string      `json:"repo_id"`
	Environment Environment `json:"environment"`
	DetectedAt  time.Time   `json:"detected_at"`
	Drifted     bool        `json:"drifted"`
	Items       []DriftItem `json:"items,omitempty"`
	Summary     string      `json:"summary"`
}

// RollbackRecord 回滚记录
type RollbackRecord struct {
	ID           string           `json:"id"`
	DeploymentID string           `json:"deployment_id"`
	RepoID       string           `json:"repo_id"`
	Environment  Environment      `json:"environment"`
	FromRevision string           `json:"from_revision"`
	ToRevision   string           `json:"to_revision"`
	Status       DeploymentStatus `json:"status"`
	Reason       string           `json:"reason,omitempty"`
	RolledBackAt time.Time        `json:"rolled_back_at"`
	CompletedAt  *time.Time       `json:"completed_at,omitempty"`
}

// AddRepoRequest 添加仓库请求
type AddRepoRequest struct {
	Name   string  `json:"name" binding:"required"`
	URL    string  `json:"url" binding:"required"`
	Branch string  `json:"branch"`
	Path   string  `json:"path"`
	Auth   GitAuth `json:"auth,omitempty"`
}

// DriftDetectionRequest 漂移检测请求
type DriftDetectionRequest struct {
	RepoID      string      `json:"repo_id" binding:"required"`
	Environment Environment `json:"environment" binding:"required"`
}
