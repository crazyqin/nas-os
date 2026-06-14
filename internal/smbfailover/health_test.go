package smbfailover

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestHealthChecker_AddNode(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	// Add node
	checker.AddNode("node-1", "server1", "192.168.1.10", 8080)

	// Get node health
	health, ok := checker.GetNodeHealth("node-1")
	assert.True(t, ok)
	assert.NotNil(t, health)
	assert.Equal(t, "node-1", health.NodeID)
	assert.Equal(t, "server1", health.Hostname)
	assert.Equal(t, "192.168.1.10", health.Address)
	assert.True(t, health.Healthy)

	// Remove node
	checker.RemoveNode("node-1")
	_, ok = checker.GetNodeHealth("node-1")
	assert.False(t, ok)
}

func TestHealthChecker_SetLocalNode(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	checker.SetLocalNode("node-local")
	assert.Equal(t, "node-local", checker.localNodeID)
}

func TestHealthChecker_RegisterCheck(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	// Register custom check
	checker.RegisterCheck("custom-check", func(ctx context.Context, node *HealthNode) CheckDetail {
		return CheckDetail{
			Name:    "custom-check",
			Healthy: true,
			Message: "custom check passed",
		}
	})

	// Verify check is registered
	checker.mu.RLock()
	_, ok := checker.checks["custom-check"]
	checker.mu.RUnlock()
	assert.True(t, ok)
}

func TestHealthChecker_StartStop(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	// Start
	err := checker.Start()
	require.NoError(t, err)

	// Try to start again
	err = checker.Start()
	assert.Error(t, err)

	// Stop
	checker.Stop()
}

func TestHealthChecker_IsNodeHealthy(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	checker.AddNode("node-1", "server1", "192.168.1.10", 8080)

	// Initially healthy
	assert.True(t, checker.IsNodeHealthy("node-1"))

	// Mark as unhealthy
	checker.SetNodeHealth("node-1", false)

	assert.False(t, checker.IsNodeHealthy("node-1"))

	// Non-existent node
	assert.False(t, checker.IsNodeHealthy("node-99"))
}

func TestHealthChecker_GetHealthyNodes(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	checker.AddNode("node-1", "server1", "192.168.1.10", 8080)
	checker.AddNode("node-2", "server2", "192.168.1.11", 8080)
	checker.AddNode("node-3", "server3", "192.168.1.12", 8080)

	// Mark one as unhealthy
	checker.SetNodeHealth("node-2", false)

	healthy := checker.GetHealthyNodes()
	assert.Len(t, healthy, 2)
	assert.Contains(t, healthy, "node-1")
	assert.Contains(t, healthy, "node-3")
	assert.NotContains(t, healthy, "node-2")
}

func TestHealthChecker_GetUnhealthyNodes(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	checker.AddNode("node-1", "server1", "192.168.1.10", 8080)
	checker.AddNode("node-2", "server2", "192.168.1.11", 8080)

	// Mark one as unhealthy
	checker.SetNodeHealth("node-2", false)

	unhealthy := checker.GetUnhealthyNodes()
	assert.Len(t, unhealthy, 1)
	assert.Contains(t, unhealthy, "node-2")
}

func TestHealthChecker_GetAllNodeHealth(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	checker.AddNode("node-1", "server1", "192.168.1.10", 8080)
	checker.AddNode("node-2", "server2", "192.168.1.11", 8080)

	allHealth := checker.GetAllNodeHealth()
	assert.Len(t, allHealth, 2)
	assert.Contains(t, allHealth, "node-1")
	assert.Contains(t, allHealth, "node-2")
}

func TestHealthChecker_GetClusterHealth(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	checker.AddNode("node-1", "server1", "192.168.1.10", 8080)
	checker.AddNode("node-2", "server2", "192.168.1.11", 8080)

	// All healthy
	health := checker.GetClusterHealth()
	assert.Equal(t, 2, health["total_nodes"])
	assert.Equal(t, 2, health["healthy_nodes"])
	assert.Equal(t, 0, health["unhealthy_nodes"])
	assert.True(t, health["cluster_healthy"].(bool))

	// Mark one unhealthy
	checker.SetNodeHealth("node-2", false)

	health = checker.GetClusterHealth()
	assert.Equal(t, 1, health["healthy_nodes"])
	assert.Equal(t, 1, health["unhealthy_nodes"])
	assert.False(t, health["cluster_healthy"].(bool))
}

func TestHealthChecker_GetHealthStats(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	checker.AddNode("node-1", "server1", "192.168.1.10", 8080)

	// Register a custom check
	checker.RegisterCheck("test-check", func(ctx context.Context, node *HealthNode) CheckDetail {
		return CheckDetail{Name: "test-check", Healthy: true}
	})

	stats := checker.GetHealthStats()
	assert.Equal(t, int64(0), stats["total_checks"])
	assert.Equal(t, int64(0), stats["total_failures"])
	assert.Equal(t, 1, stats["registered_checks"])
}

func TestHealthChecker_SetHealthChangeCallback(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	// Set callback
	checker.SetHealthChangeCallback(func(nodeID string, healthy bool) {
		// callback set
	})

	// Verify callback is set
	checker.mu.RLock()
	assert.NotNil(t, checker.onHealthChange)
	checker.mu.RUnlock()
}

func TestHealthChecker_PerformImmediateCheck(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	checker.AddNode("node-1", "server1", "192.168.1.10", 8080)

	// Register a custom check
	checker.RegisterCheck("test-check", func(ctx context.Context, node *HealthNode) CheckDetail {
		return CheckDetail{
			Name:    "test-check",
			Healthy: true,
			Latency: 10 * time.Millisecond,
			Message: "check passed",
		}
	})

	// Perform immediate check
	ctx := context.Background()
	result, err := checker.PerformImmediateCheck(ctx, "node-1")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Healthy)
	assert.NotEmpty(t, result.Checks)
}

func TestHealthChecker_GetNodeResult(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	checker.AddNode("node-1", "server1", "192.168.1.10", 8080)

	// No result initially
	_, ok := checker.GetNodeResult("node-1")
	assert.False(t, ok)

	// Perform a check
	ctx := context.Background()
	checker.PerformImmediateCheck(ctx, "node-1")

	// Should have result now
	result, ok := checker.GetNodeResult("node-1")
	assert.True(t, ok)
	assert.NotNil(t, result)
}

func TestHealthChecker_CheckTCP(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test with unreachable address
	detail := checker.checkTCP(ctx, "192.0.2.1", 80)
	assert.False(t, detail.Healthy)
	assert.Error(t, detail.Error)
}

func TestHealthChecker_CheckHTTP(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Test with unreachable address
	detail := checker.checkHTTP(ctx, "192.0.2.1", 8080)
	assert.False(t, detail.Healthy)
	assert.Error(t, detail.Error)
}

func TestHealthChecker_HealthCheckHandler(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	checker := NewHealthChecker(config, logger)

	checker.AddNode("node-1", "server1", "192.168.1.10", 8080)

	handler := checker.HealthCheckHandler()
	assert.NotNil(t, handler)
}

func TestHealthChecker_NodeThresholds(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	config.HealthyThreshold = 2
	config.UnhealthyThreshold = 3
	checker := NewHealthChecker(config, logger)

	checker.AddNode("node-1", "server1", "192.168.1.10", 8080)

	// Register a failing check
	failCount := 0
	checker.RegisterCheck("failing-check", func(ctx context.Context, node *HealthNode) CheckDetail {
		failCount++
		return CheckDetail{
			Name:    "failing-check",
			Healthy: failCount > 5, // Fails first 5 times
			Message: "check failed",
		}
	})

	// Start checker
	err := checker.Start()
	require.NoError(t, err)
	defer checker.Stop()

	// Wait for checks to run
	time.Sleep(500 * time.Millisecond)

	// Node should still be healthy (threshold not reached)
	health, _ := checker.GetNodeHealth("node-1")
	assert.True(t, health.Healthy)
}

func TestHealthChecker_ProcessResult(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultHealthConfig()
	config.HealthyThreshold = 1
	config.UnhealthyThreshold = 1
	checker := NewHealthChecker(config, logger)

	checker.AddNode("node-1", "server1", "192.168.1.10", 8080)

	var changedNode string
	var changedHealthy bool
	checker.SetHealthChangeCallback(func(nodeID string, healthy bool) {
		changedNode = nodeID
		changedHealthy = healthy
	})

	node, _ := checker.GetNodeHealth("node-1")

	// Process healthy result
	result := &HealthResult{
		NodeID:  "node-1",
		Healthy: true,
		Latency: 10 * time.Millisecond,
	}

	checker.processResult(node, result)
	assert.True(t, node.Healthy)
	assert.Equal(t, 1, node.ConsecutiveOK)
	assert.Equal(t, 0, node.ConsecutiveFail)

	// Process unhealthy result
	result = &HealthResult{
		NodeID:  "node-1",
		Healthy: false,
		Latency: 100 * time.Millisecond,
		Message: "connection refused",
	}

	checker.processResult(node, result)
	assert.False(t, node.Healthy)
	assert.Equal(t, 0, node.ConsecutiveOK)
	assert.Equal(t, 1, node.ConsecutiveFail)
	assert.Equal(t, "connection refused", node.FailureReason)

	// Verify callback was called
	assert.Equal(t, "node-1", changedNode)
	assert.False(t, changedHealthy)
}

func TestHealthConfig_Defaults(t *testing.T) {
	config := DefaultHealthConfig()

	assert.Equal(t, 5*time.Second, config.CheckInterval)
	assert.Equal(t, 3*time.Second, config.Timeout)
	assert.Equal(t, 2, config.HealthyThreshold)
	assert.Equal(t, 3, config.UnhealthyThreshold)
	assert.Equal(t, 1*time.Second, config.RetryInterval)
	assert.True(t, config.EnableHTTPCheck)
	assert.Equal(t, "/health", config.HTTPEndpoint)
	assert.Equal(t, 8080, config.HTTPPort)
	assert.True(t, config.EnableTCPCheck)
	assert.Contains(t, config.TCPPorts, 445)
	assert.Contains(t, config.TCPPorts, 139)
	assert.True(t, config.EnableCustomChecks)
}

func TestHealthNode_Fields(t *testing.T) {
	node := &HealthNode{
		NodeID:          "node-1",
		Hostname:        "server1",
		Address:         "192.168.1.10",
		Port:            8080,
		Healthy:         true,
		ConsecutiveOK:   5,
		ConsecutiveFail: 0,
		LastCheck:       time.Now(),
		LastOK:          time.Now(),
		Latency:         10 * time.Millisecond,
		TotalChecks:     100,
		TotalFailures:   2,
	}

	assert.Equal(t, "node-1", node.NodeID)
	assert.Equal(t, "server1", node.Hostname)
	assert.Equal(t, "192.168.1.10", node.Address)
	assert.Equal(t, 8080, node.Port)
	assert.True(t, node.Healthy)
	assert.Equal(t, 5, node.ConsecutiveOK)
	assert.Equal(t, int64(100), node.TotalChecks)
	assert.Equal(t, int64(2), node.TotalFailures)
}

func TestHealthResult_Fields(t *testing.T) {
	result := &HealthResult{
		NodeID:    "node-1",
		Healthy:   true,
		Latency:   50 * time.Millisecond,
		Message:   "all checks passed",
		Timestamp: time.Now(),
		Checks: []CheckDetail{
			{Name: "tcp", Healthy: true, Latency: 10 * time.Millisecond},
			{Name: "http", Healthy: true, Latency: 20 * time.Millisecond},
		},
	}

	assert.Equal(t, "node-1", result.NodeID)
	assert.True(t, result.Healthy)
	assert.Equal(t, 50*time.Millisecond, result.Latency)
	assert.Equal(t, "all checks passed", result.Message)
	assert.Len(t, result.Checks, 2)
}

func TestCheckDetail_Fields(t *testing.T) {
	detail := CheckDetail{
		Name:    "test-check",
		Healthy: true,
		Latency: 25 * time.Millisecond,
		Message: "check passed",
		Error:   nil,
	}

	assert.Equal(t, "test-check", detail.Name)
	assert.True(t, detail.Healthy)
	assert.Equal(t, 25*time.Millisecond, detail.Latency)
	assert.Equal(t, "check passed", detail.Message)
	assert.NoError(t, detail.Error)
}

func TestHealthResponse_Fields(t *testing.T) {
	response := &HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Uptime:    24 * time.Hour,
		Version:   "1.0.0",
		Services: map[string]string{
			"smb": "running",
			"ads": "running",
		},
		Metrics: map[string]interface{}{
			"sessions": 100,
		},
	}

	assert.Equal(t, "healthy", response.Status)
	assert.Equal(t, 24*time.Hour, response.Uptime)
	assert.Equal(t, "1.0.0", response.Version)
	assert.Equal(t, "running", response.Services["smb"])
	assert.Equal(t, 100, response.Metrics["sessions"])
}
