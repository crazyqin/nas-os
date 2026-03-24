// Package container provides Docker container management functionality.
package container

import (
	"testing"
)

// TestBatchOperationType tests batch operation type constants.
func TestBatchOperationType(t *testing.T) {
	tests := []struct {
		name     string
		op       BatchOperationType
		expected string
	}{
		{"start", BatchStart, "start"},
		{"stop", BatchStop, "stop"},
		{"restart", BatchRestart, "restart"},
		{"remove", BatchRemove, "remove"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.op) != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.op)
			}
		})
	}
}

// TestBatchOperationResult tests BatchOperationResult struct.
func TestBatchOperationResult(t *testing.T) {
	result := BatchOperationResult{
		ContainerID:   "abc123",
		ContainerName: "test-container",
		Operation:     BatchStart,
		Success:       true,
		Duration:      1000000000, // 1 second
	}

	if result.ContainerID != "abc123" {
		t.Errorf("expected ContainerID abc123, got %s", result.ContainerID)
	}
	if !result.Success {
		t.Error("expected Success to be true")
	}
	if result.Operation != BatchStart {
		t.Errorf("expected Operation BatchStart, got %s", result.Operation)
	}
}

// TestBatchOperationRequest tests BatchOperationRequest struct.
func TestBatchOperationRequest(t *testing.T) {
	req := BatchOperationRequest{
		ContainerIDs:  []string{"id1", "id2", "id3"},
		Operation:     BatchStop,
		Timeout:       30,
		Force:         true,
		RemoveVolumes: false,
	}

	if len(req.ContainerIDs) != 3 {
		t.Errorf("expected 3 container IDs, got %d", len(req.ContainerIDs))
	}
	if req.Operation != BatchStop {
		t.Errorf("expected BatchStop, got %s", req.Operation)
	}
	if req.Timeout != 30 {
		t.Errorf("expected timeout 30, got %d", req.Timeout)
	}
}

// TestBatchOperationResponse tests BatchOperationResponse struct.
func TestBatchOperationResponse(t *testing.T) {
	response := BatchOperationResponse{
		Total:     5,
		Succeeded: 3,
		Failed:    2,
		Results: []BatchOperationResult{
			{ContainerID: "id1", Success: true},
			{ContainerID: "id2", Success: true},
			{ContainerID: "id3", Success: true},
			{ContainerID: "id4", Success: false, Error: "error"},
			{ContainerID: "id5", Success: false, Error: "error"},
		},
	}

	if response.Total != 5 {
		t.Errorf("expected Total 5, got %d", response.Total)
	}
	if response.Succeeded != 3 {
		t.Errorf("expected Succeeded 3, got %d", response.Succeeded)
	}
	if response.Failed != 2 {
		t.Errorf("expected Failed 2, got %d", response.Failed)
	}
	if len(response.Results) != 5 {
		t.Errorf("expected 5 results, got %d", len(response.Results))
	}
}

// TestContainerFilter tests ContainerFilter struct.
func TestContainerFilter(t *testing.T) {
	filter := ContainerFilter{
		State:   "running",
		Image:   "nginx:latest",
		Label:   map[string]string{"app": "web"},
		Name:    "nginx-server",
		Network: "bridge",
	}

	if filter.State != "running" {
		t.Errorf("expected State running, got %s", filter.State)
	}
	if filter.Image != "nginx:latest" {
		t.Errorf("expected Image nginx:latest, got %s", filter.Image)
	}
	if filter.Label["app"] != "web" {
		t.Errorf("expected Label app=web, got %s", filter.Label["app"])
	}
}

// TestContainerFilter_Match tests ContainerFilter.Match method.
func TestContainerFilter_Match(t *testing.T) {
	tests := []struct {
		name     string
		filter   ContainerFilter
		container *Container
		expected bool
	}{
		{
			name:   "match all empty filter",
			filter: ContainerFilter{},
			container: &Container{
				State: "running",
				Name:  "test",
			},
			expected: true,
		},
		{
			name: "match state",
			filter: ContainerFilter{State: "running"},
			container: &Container{State: "running"},
			expected: true,
		},
		{
			name: "no match state",
			filter: ContainerFilter{State: "running"},
			container: &Container{State: "exited"},
			expected: false,
		},
		{
			name: "match image",
			filter: ContainerFilter{Image: "nginx:latest"},
			container: &Container{Image: "nginx:latest"},
			expected: true,
		},
		{
			name: "no match image",
			filter: ContainerFilter{Image: "nginx:latest"},
			container: &Container{Image: "redis:latest"},
			expected: false,
		},
		{
			name: "match name",
			filter: ContainerFilter{Name: "nginx-server"},
			container: &Container{Name: "nginx-server"},
			expected: true,
		},
		{
			name: "no match name",
			filter: ContainerFilter{Name: "nginx-server"},
			container: &Container{Name: "redis-server"},
			expected: false,
		},
		{
			name: "match label",
			filter: ContainerFilter{Label: map[string]string{"app": "web"}},
			container: &Container{Labels: map[string]string{"app": "web"}},
			expected: true,
		},
		{
			name: "no match label missing",
			filter: ContainerFilter{Label: map[string]string{"app": "web"}},
			container: &Container{Labels: map[string]string{}},
			expected: false,
		},
		{
			name: "no match label different value",
			filter: ContainerFilter{Label: map[string]string{"app": "web"}},
			container: &Container{Labels: map[string]string{"app": "api"}},
			expected: false,
		},
		{
			name: "match network",
			filter: ContainerFilter{Network: "bridge"},
			container: &Container{Networks: []string{"bridge", "host"}},
			expected: true,
		},
		{
			name: "no match network",
			filter: ContainerFilter{Network: "custom"},
			container: &Container{Networks: []string{"bridge", "host"}},
			expected: false,
		},
		{
			name: "match multiple criteria",
			filter: ContainerFilter{
				State: "running",
				Image: "nginx:latest",
			},
			container: &Container{
				State: "running",
				Image: "nginx:latest",
			},
			expected: true,
		},
		{
			name: "no match one of multiple criteria",
			filter: ContainerFilter{
				State: "running",
				Image: "nginx:latest",
			},
			container: &Container{
				State: "running",
				Image: "redis:latest",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.filter.Match(tt.container)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// TestBatchProgress tests BatchProgress struct.
func TestBatchProgress(t *testing.T) {
	progress := &BatchProgress{
		Total:       10,
		Completed:   5,
		Succeeded:   4,
		Failed:      1,
		CurrentID:   "container-123",
		CurrentName: "nginx-server",
	}

	p := progress.GetProgress()

	if p["total"].(int32) != 10 {
		t.Errorf("expected total 10, got %v", p["total"])
	}
	if p["completed"].(int32) != 5 {
		t.Errorf("expected completed 5, got %v", p["completed"])
	}
	if p["succeeded"].(int32) != 4 {
		t.Errorf("expected succeeded 4, got %v", p["succeeded"])
	}
	if p["failed"].(int32) != 1 {
		t.Errorf("expected failed 1, got %v", p["failed"])
	}
}

// TestStreamConfig tests StreamConfig struct.
func TestStreamConfig(t *testing.T) {
	config := StreamConfig{
		Follow:     true,
		Tail:       200,
		Since:      "1h",
		Timestamps: true,
		Stdout:     true,
		Stderr:     true,
	}

	if !config.Follow {
		t.Error("expected Follow to be true")
	}
	if config.Tail != 200 {
		t.Errorf("expected Tail 200, got %d", config.Tail)
	}
}

// TestDefaultStreamConfig tests DefaultStreamConfig function.
func TestDefaultStreamConfig(t *testing.T) {
	config := DefaultStreamConfig()

	if !config.Follow {
		t.Error("expected default Follow to be true")
	}
	if config.Tail != 100 {
		t.Errorf("expected default Tail 100, got %d", config.Tail)
	}
	if !config.Timestamps {
		t.Error("expected default Timestamps to be true")
	}
}

// TestLogMessage tests LogMessage struct.
func TestLogMessage(t *testing.T) {
	msg := LogMessage{
		Line:      "test log message",
		Source:    "stdout",
		Container: "nginx-server",
	}

	if msg.Source != "stdout" {
		t.Errorf("expected Source stdout, got %s", msg.Source)
	}
	if msg.Line != "test log message" {
		t.Errorf("expected Line 'test log message', got %s", msg.Line)
	}
}

// TestStatsMessage tests StatsMessage struct.
func TestStatsMessage(t *testing.T) {
	stats := StatsMessage{
		CPUUsage:    25.5,
		MemUsage:    512 * 1024 * 1024, // 512MB
		MemLimit:    1024 * 1024 * 1024, // 1GB
		MemPercent:  50.0,
		NetRX:       1024000,
		NetTX:       2048000,
		BlockRead:   4096,
		BlockWrite:  8192,
		PIDs:        5,
		ContainerID: "abc123",
	}

	if stats.CPUUsage != 25.5 {
		t.Errorf("expected CPUUsage 25.5, got %f", stats.CPUUsage)
	}
	if stats.MemPercent != 50.0 {
		t.Errorf("expected MemPercent 50.0, got %f", stats.MemPercent)
	}
	if stats.PIDs != 5 {
		t.Errorf("expected PIDs 5, got %d", stats.PIDs)
	}
}

// TestPruneResult tests PruneResult struct.
func TestPruneResult(t *testing.T) {
	result := &PruneResult{
		ContainersDeleted: []string{"id1", "id2", "id3"},
		SpaceReclaimed:    1024 * 1024 * 100, // 100MB
	}

	if len(result.ContainersDeleted) != 3 {
		t.Errorf("expected 3 deleted containers, got %d", len(result.ContainersDeleted))
	}
}