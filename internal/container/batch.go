// Package container provides Docker container management functionality
// File: batch.go - Batch operations for containers
package container

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// BatchOperationType defines the type of batch operation
type BatchOperationType string

const (
	BatchStart   BatchOperationType = "start"
	BatchStop    BatchOperationType = "stop"
	BatchRestart BatchOperationType = "restart"
	BatchRemove  BatchOperationType = "remove"
)

// BatchOperationResult represents the result of a single container operation
type BatchOperationResult struct {
	ContainerID   string             `json:"containerId"`
	ContainerName string             `json:"containerName"`
	Operation     BatchOperationType `json:"operation"`
	Success       bool               `json:"success"`
	Error         string             `json:"error,omitempty"`
	Duration      time.Duration      `json:"duration"`
}

// BatchOperationRequest represents a batch operation request
type BatchOperationRequest struct {
	ContainerIDs []string           `json:"containerIds"`
	Operation    BatchOperationType `json:"operation"`
	Timeout      int                `json:"timeout,omitempty"` // seconds, for stop/restart
	Force        bool               `json:"force,omitempty"`  // for remove
	RemoveVolumes bool              `json:"removeVolumes,omitempty"` // for remove
}

// BatchOperationResponse represents the overall batch operation result
type BatchOperationResponse struct {
	Total     int                     `json:"total"`
	Succeeded int                     `json:"succeeded"`
	Failed    int                     `json:"failed"`
	Results   []BatchOperationResult  `json:"results"`
	Duration  time.Duration           `json:"duration"`
}

// BatchManager handles batch operations on containers
type BatchManager struct {
	manager *Manager
}

// NewBatchManager creates a new batch manager
func NewBatchManager(manager *Manager) *BatchManager {
	return &BatchManager{
		manager: manager,
	}
}

// Execute performs a batch operation on multiple containers
func (b *BatchManager) Execute(ctx context.Context, req BatchOperationRequest) (*BatchOperationResponse, error) {
	if len(req.ContainerIDs) == 0 {
		return nil, fmt.Errorf("no containers specified")
	}

	startTime := time.Now()
	response := &BatchOperationResponse{
		Total:   len(req.ContainerIDs),
		Results: make([]BatchOperationResult, 0, len(req.ContainerIDs)),
	}

	// Use a channel to collect results
	resultChan := make(chan BatchOperationResult, len(req.ContainerIDs))
	var wg sync.WaitGroup

	// Process containers concurrently (with limit)
	maxConcurrency := 5
	semaphore := make(chan struct{}, maxConcurrency)

	for _, containerID := range req.ContainerIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			result := b.executeSingle(ctx, id, req)
			resultChan <- result
		}(containerID)
	}

	// Wait for all operations to complete
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		response.Results = append(response.Results, result)
		if result.Success {
			response.Succeeded++
		} else {
			response.Failed++
		}
	}

	response.Duration = time.Since(startTime)
	return response, nil
}

// executeSingle executes a single container operation
func (b *BatchManager) executeSingle(ctx context.Context, containerID string, req BatchOperationRequest) BatchOperationResult {
	startTime := time.Now()
	result := BatchOperationResult{
		ContainerID: containerID,
		Operation:   req.Operation,
	}

	// Get container name for better error messages
	if container, err := b.manager.GetContainer(containerID); err == nil {
		result.ContainerName = container.Name
	} else {
		result.ContainerName = containerID
	}

	var err error
	switch req.Operation {
	case BatchStart:
		err = b.manager.StartContainer(containerID)
	case BatchStop:
		timeout := req.Timeout
		if timeout <= 0 {
			timeout = DefaultStopTimeout
		}
		err = b.manager.StopContainer(containerID, timeout)
	case BatchRestart:
		timeout := req.Timeout
		if timeout <= 0 {
			timeout = DefaultStopTimeout
		}
		err = b.manager.RestartContainer(containerID, timeout)
	case BatchRemove:
		err = b.manager.RemoveContainer(containerID, req.Force, req.RemoveVolumes)
	default:
		err = fmt.Errorf("unknown operation: %s", req.Operation)
	}

	result.Duration = time.Since(startTime)
	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
	}

	return result
}

// StartBatch starts multiple containers
func (b *BatchManager) StartBatch(ctx context.Context, containerIDs []string) (*BatchOperationResponse, error) {
	return b.Execute(ctx, BatchOperationRequest{
		ContainerIDs: containerIDs,
		Operation:    BatchStart,
	})
}

// StopBatch stops multiple containers
func (b *BatchManager) StopBatch(ctx context.Context, containerIDs []string, timeout int) (*BatchOperationResponse, error) {
	return b.Execute(ctx, BatchOperationRequest{
		ContainerIDs: containerIDs,
		Operation:    BatchStop,
		Timeout:      timeout,
	})
}

// RestartBatch restarts multiple containers
func (b *BatchManager) RestartBatch(ctx context.Context, containerIDs []string, timeout int) (*BatchOperationResponse, error) {
	return b.Execute(ctx, BatchOperationRequest{
		ContainerIDs: containerIDs,
		Operation:    BatchRestart,
		Timeout:      timeout,
	})
}

// RemoveBatch removes multiple containers
func (b *BatchManager) RemoveBatch(ctx context.Context, containerIDs []string, force bool, removeVolumes bool) (*BatchOperationResponse, error) {
	return b.Execute(ctx, BatchOperationRequest{
		ContainerIDs:   containerIDs,
		Operation:      BatchRemove,
		Force:          force,
		RemoveVolumes:  removeVolumes,
	})
}

// SelectByFilter selects containers matching a filter for batch operations
func (b *BatchManager) SelectByFilter(filter ContainerFilter) ([]string, error) {
	containers, err := b.manager.ListContainers(true)
	if err != nil {
		return nil, err
	}

	var ids []string
	for _, c := range containers {
		if filter.Match(c) {
			ids = append(ids, c.ID)
		}
	}

	return ids, nil
}

// ContainerFilter defines criteria for selecting containers
type ContainerFilter struct {
	State   string            `json:"state,omitempty"`   // "running", "exited", "paused", etc.
	Image   string            `json:"image,omitempty"`   // Image name pattern
	Label   map[string]string `json:"label,omitempty"`   // Label key-value pairs
	Name    string            `json:"name,omitempty"`    // Container name pattern
	Network string            `json:"network,omitempty"` // Network name
}

// Match checks if a container matches the filter criteria
func (f *ContainerFilter) Match(c *Container) bool {
	// Check state
	if f.State != "" && c.State != f.State {
		return false
	}

	// Check image
	if f.Image != "" && c.Image != f.Image {
		return false
	}

	// Check name
	if f.Name != "" && c.Name != f.Name {
		return false
	}

	// Check labels
	for k, v := range f.Label {
		if c.Labels == nil {
			return false
		}
		if cv, ok := c.Labels[k]; !ok || cv != v {
			return false
		}
	}

	// Check network
	if f.Network != "" {
		found := false
		for _, n := range c.Networks {
			if n == f.Network {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// PruneResult represents the result of a prune operation
type PruneResult struct {
	ContainersDeleted []string `json:"containersDeleted"`
	SpaceReclaimed    uint64   `json:"spaceReclaimed"`
}

// PruneContainers removes all stopped containers
func (m *Manager) PruneContainers() (*PruneResult, error) {
	// Get all stopped containers first
	containers, err := m.ListContainers(true)
	if err != nil {
		return nil, err
	}

	var stoppedIDs []string
	for _, c := range containers {
		if c.State == "exited" || c.State == "created" || c.State == "dead" {
			stoppedIDs = append(stoppedIDs, c.ID)
		}
	}

	if len(stoppedIDs) == 0 {
		return &PruneResult{}, nil
	}

	// Use batch manager to remove
	batch := NewBatchManager(m)
	ctx := context.Background()
	response, err := batch.RemoveBatch(ctx, stoppedIDs, false, false)
	if err != nil {
		return nil, err
	}

	result := &PruneResult{}
	for _, r := range response.Results {
		if r.Success {
			result.ContainersDeleted = append(result.ContainersDeleted, r.ContainerID)
		}
	}

	return result, nil
}

// BatchProgress tracks progress of batch operations for WebSocket streaming
type BatchProgress struct {
	Total       int32
	Completed   int32
	Succeeded   int32
	Failed      int32
	CurrentID   string
	CurrentName string
}

// GetProgress returns current progress
func (p *BatchProgress) GetProgress() map[string]interface{} {
	return map[string]interface{}{
		"total":       atomic.LoadInt32(&p.Total),
		"completed":   atomic.LoadInt32(&p.Completed),
		"succeeded":   atomic.LoadInt32(&p.Succeeded),
		"failed":      atomic.LoadInt32(&p.Failed),
		"currentId":   p.CurrentID,
		"currentName": p.CurrentName,
	}
}

// ExecuteWithProgress performs batch operation with progress updates
func (b *BatchManager) ExecuteWithProgress(ctx context.Context, req BatchOperationRequest, progressChan chan<- BatchProgress) (*BatchOperationResponse, error) {
	if len(req.ContainerIDs) == 0 {
		return nil, fmt.Errorf("no containers specified")
	}

	progress := &BatchProgress{
		Total: int32(len(req.ContainerIDs)),
	}

	startTime := time.Now()
	response := &BatchOperationResponse{
		Total:   len(req.ContainerIDs),
		Results: make([]BatchOperationResult, 0, len(req.ContainerIDs)),
	}

	resultChan := make(chan BatchOperationResult, len(req.ContainerIDs))
	var wg sync.WaitGroup

	// Send initial progress
	if progressChan != nil {
		select {
		case progressChan <- *progress:
		default:
		}
	}

	for _, containerID := range req.ContainerIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			progress.CurrentID = id
			result := b.executeSingle(ctx, id, req)

			atomic.AddInt32(&progress.Completed, 1)
			if result.Success {
				atomic.AddInt32(&progress.Succeeded, 1)
			} else {
				atomic.AddInt32(&progress.Failed, 1)
			}

			// Send progress update
			if progressChan != nil {
				select {
				case progressChan <- *progress:
				default:
				}
			}

			resultChan <- result
		}(containerID)
	}

	// Wait for completion
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		response.Results = append(response.Results, result)
		if result.Success {
			response.Succeeded++
		} else {
			response.Failed++
		}
	}

	response.Duration = time.Since(startTime)
	return response, nil
}