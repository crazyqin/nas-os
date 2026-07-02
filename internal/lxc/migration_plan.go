package lxc

import (
	"fmt"
	"time"
)

// MigrationStep describes an ordered LXC migration action.
type MigrationStep struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Duration    time.Duration `json:"duration"`
	Required    bool          `json:"required"`
}

// MigrationPlan is a safe, reviewable plan for moving workloads between nodes.
type MigrationPlan struct {
	ContainerID string          `json:"container_id"`
	SourceNode  string          `json:"source_node"`
	TargetNode  string          `json:"target_node"`
	Mode        string          `json:"mode"` // cold, warm, live
	Steps       []MigrationStep `json:"steps"`
	Rollback    []MigrationStep `json:"rollback"`
	CreatedAt   time.Time       `json:"created_at"`
}

// BuildMigrationPlan creates a deterministic migration plan without executing it.
func BuildMigrationPlan(container *Container, sourceNode, targetNode string, live bool) (*MigrationPlan, error) {
	if container == nil {
		return nil, fmt.Errorf("container cannot be nil")
	}
	if sourceNode == "" || targetNode == "" {
		return nil, fmt.Errorf("source and target nodes are required")
	}
	mode := "warm"
	if live && container.Status == StatusRunning {
		mode = "live"
	} else if container.Status != StatusRunning {
		mode = "cold"
	}

	steps := []MigrationStep{
		{Name: "preflight", Description: "verify template, storage quota, network bridge, and target node health", Duration: time.Minute, Required: true},
		{Name: "snapshot", Description: "create consistent container snapshot before transfer", Duration: 2 * time.Minute, Required: true},
		{Name: "sync", Description: "transfer rootfs, volumes, metadata, and HA policy to target", Duration: 5 * time.Minute, Required: true},
		{Name: "validate", Description: "validate network, ports, health checks, and resource limits", Duration: time.Minute, Required: true},
	}
	if mode == "live" {
		steps = append(steps, MigrationStep{Name: "cutover", Description: "freeze briefly, sync dirty pages, then resume on target", Duration: 30 * time.Second, Required: true})
	} else {
		steps = append(steps, MigrationStep{Name: "start", Description: "start container on target after final sync", Duration: time.Minute, Required: true})
	}

	return &MigrationPlan{
		ContainerID: container.ID,
		SourceNode:  sourceNode,
		TargetNode:  targetNode,
		Mode:        mode,
		Steps:       steps,
		Rollback: []MigrationStep{
			{Name: "restore-source", Description: "keep source snapshot until target health check passes", Duration: time.Minute, Required: true},
			{Name: "dns-revert", Description: "restore previous service discovery records if cutover fails", Duration: 30 * time.Second, Required: false},
		},
		CreatedAt: time.Now(),
	}, nil
}
