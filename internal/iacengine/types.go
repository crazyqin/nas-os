// Package iacengine provides Infrastructure as Code capabilities for managing
// NAS resources and services through declarative configuration templates.
package iacengine

import (
	"time"
)

// StackStatus represents the state of a stack
type StackStatus string

const (
	StackStatusCreating  StackStatus = "creating"
	StackStatusActive    StackStatus = "active"
	StackStatusUpdating  StackStatus = "updating"
	StackStatusDeleting  StackStatus = "deleting"
	StackStatusFailed    StackStatus = "failed"
	StackStatusDrifted   StackStatus = "drifted"
)

// ResourceStatus represents the state of a resource
type ResourceStatus string

const (
	ResourceStatusPending  ResourceStatus = "pending"
	ResourceStatusCreating ResourceStatus = "creating"
	ResourceStatusActive   ResourceStatus = "active"
	ResourceStatusUpdating ResourceStatus = "updating"
	ResourceStatusDeleting ResourceStatus = "deleting"
	ResourceStatusFailed   ResourceStatus = "failed"
	ResourceStatusDrifted  ResourceStatus = "drifted"
)

// DriftStatus represents drift detection result
type DriftStatus string

const (
	DriftStatusNone     DriftStatus = "none"
	DriftStatusDrifted  DriftStatus = "drifted"
	DriftStatusDeleted  DriftStatus = "deleted"
	DriftStatusModified DriftStatus = "modified"
)

// IaCTemplate represents an Infrastructure as Code template
type IaCTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Version     string            `json:"version"`
	Content     string            `json:"content"` // YAML/JSON template content
	Variables   map[string]string `json:"variables,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Resource represents a managed infrastructure resource
type Resource struct {
	ID         string            `json:"id"`
	StackID    string            `json:"stack_id"`
	Kind       string            `json:"kind"`       // volume, share, container, network, etc.
	Name       string            `json:"name"`
	Status     ResourceStatus    `json:"status"`
	Config     map[string]string `json:"config"`
	Properties map[string]string `json:"properties,omitempty"`
	Error      string            `json:"error,omitempty"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// Stack represents a deployed infrastructure stack
type Stack struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	TemplateID  string            `json:"template_id"`
	Status      StackStatus       `json:"status"`
	Variables   map[string]string `json:"variables,omitempty"`
	Resources   []Resource        `json:"resources"`
	Outputs     map[string]string `json:"outputs,omitempty"`
	DriftStatus DriftStatus       `json:"drift_status"`
	Error       string            `json:"error,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	DeployedAt  *time.Time        `json:"deployed_at,omitempty"`
	DestroyedAt *time.Time        `json:"destroyed_at,omitempty"`
}

// DriftReport represents a drift detection report
type DriftReport struct {
	ID         string         `json:"id"`
	StackID    string         `json:"stack_id"`
	StackName  string         `json:"stack_name"`
	CheckedAt  time.Time      `json:"checked_at"`
	HasDrift   bool           `json:"has_drift"`
	Drifts     []ResourceDrift `json:"drifts"`
	Summary    DriftSummary   `json:"summary"`
}

// ResourceDrift represents drift in a single resource
type ResourceDrift struct {
	ResourceID   string     `json:"resource_id"`
	ResourceName string     `json:"resource_name"`
	ResourceKind string     `json:"resource_kind"`
	Status       DriftStatus `json:"status"`
	Expected     string     `json:"expected"`
	Actual       string     `json:"actual,omitempty"`
	Diff         string     `json:"diff,omitempty"`
}

// DriftSummary provides a summary of drift detection
type DriftSummary struct {
	TotalResources    int `json:"total_resources"`
	DriftedResources  int `json:"drifted_resources"`
	DeletedResources  int `json:"deleted_resources"`
	ModifiedResources int `json:"modified_resources"`
	UnchangedResources int `json:"unchanged_resources"`
}

// DeployStackRequest is the request for deploying a stack
type DeployStackRequest struct {
	Name       string            `json:"name" binding:"required"`
	TemplateID string            `json:"template_id" binding:"required"`
	Variables  map[string]string `json:"variables,omitempty"`
}

// UpdateStackRequest is the request for updating a stack
type UpdateStackRequest struct {
	Variables map[string]string `json:"variables,omitempty"`
}
