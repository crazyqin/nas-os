// Package gitopsmgr provides GitOps management with repository connection,
// config synchronization, drift detection, rollback, and deployment history.
package gitopsmgr

import (
	"time"
)

// SyncState represents the synchronization state
type SyncState string

const (
	SyncStateSynced    SyncState = "synced"
	SyncStateOutOfSync SyncState = "out_of_sync"
	SyncStateSyncing   SyncState = "syncing"
	SyncStateFailed    SyncState = "failed"
	SyncStateUnknown   SyncState = "unknown"
)

// DriftSeverity represents the severity of configuration drift
type DriftSeverity string

const (
	DriftSeverityNone     DriftSeverity = "none"
	DriftSeverityLow      DriftSeverity = "low"
	DriftSeverityMedium   DriftSeverity = "medium"
	DriftSeverityHigh     DriftSeverity = "high"
	DriftSeverityCritical DriftSeverity = "critical"
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

// GitRepo represents a connected Git repository
type GitRepo struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	URL        string     `json:"url"`
	Branch     string     `json:"branch"`
	Path       string     `json:"path"`
	AuthType   string     `json:"auth_type"` // ssh, token, basic
	Connected  bool       `json:"connected"`
	LastSync   *time.Time `json:"last_sync,omitempty"`
	SyncPolicy SyncPolicy `json:"sync_policy"`
	Labels     []string   `json:"labels,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// SyncPolicy defines synchronization behavior
type SyncPolicy struct {
	AutoSync    bool          `json:"auto_sync"`
	Interval    time.Duration `json:"interval"`
	Prune       bool          `json:"prune"`
	SelfHeal    bool          `json:"self_heal"`
	RetryLimit  int           `json:"retry_limit"`
	Timeout     time.Duration `json:"timeout"`
	IgnoreDiffs []string      `json:"ignore_diffs,omitempty"`
}

// DriftDetection represents a detected configuration drift
type DriftDetection struct {
	ID           string        `json:"id"`
	RepoID       string        `json:"repo_id"`
	ResourceType string        `json:"resource_type"`
	ResourceName string        `json:"resource_name"`
	Severity     DriftSeverity `json:"severity"`
	Expected     string        `json:"expected"`
	Actual       string        `json:"actual"`
	Diff         string        `json:"diff"`
	DetectedAt   time.Time     `json:"detected_at"`
	ResolvedAt   *time.Time    `json:"resolved_at,omitempty"`
}

// DeploymentState represents a deployment record
type DeploymentState struct {
	ID           string           `json:"id"`
	RepoID       string           `json:"repo_id"`
	CommitSHA    string           `json:"commit_sha"`
	Version      string           `json:"version"`
	Status       DeploymentStatus `json:"status"`
	Resources    []ResourceRef    `json:"resources"`
	SyncedAt     time.Time        `json:"synced_at"`
	RolledBackAt *time.Time       `json:"rolled_back_at,omitempty"`
	RollbackFrom string           `json:"rollback_from,omitempty"`
	Message      string           `json:"message,omitempty"`
}

// ResourceRef references a deployed resource
type ResourceRef struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
	NS   string `json:"namespace,omitempty"`
}

// SyncRequest is the request body for triggering sync
type SyncRequest struct {
	RepoID string `json:"repo_id" binding:"required"`
	Force  bool   `json:"force"`
	DryRun bool   `json:"dry_run"`
}

// RollbackRequest is the request body for rollback
type RollbackRequest struct {
	DeploymentID string `json:"deployment_id" binding:"required"`
	Reason       string `json:"reason"`
}

// DefaultSyncPolicy returns a sensible default sync policy
func DefaultSyncPolicy() SyncPolicy {
	return SyncPolicy{
		AutoSync:   true,
		Interval:   5 * time.Minute,
		Prune:      true,
		SelfHeal:   true,
		RetryLimit: 3,
		Timeout:    2 * time.Minute,
	}
}
