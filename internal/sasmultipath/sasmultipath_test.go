// Package sasmultipath 单元测试
package sasmultipath

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPathStatus_Constants(t *testing.T) {
	assert.Equal(t, PathStatus("active"), PathActive)
	assert.Equal(t, PathStatus("standby"), PathStandby)
	assert.Equal(t, PathStatus("failed"), PathFailed)
	assert.Equal(t, PathStatus("removed"), PathRemoved)
}

func TestLoadBalancePolicy_Constants(t *testing.T) {
	assert.Equal(t, LoadBalancePolicy("round-robin"), PolicyRoundRobin)
	assert.Equal(t, LoadBalancePolicy("least-pending"), PolicyLeastPending)
}

func TestDeviceStatus_Constants(t *testing.T) {
	assert.Equal(t, DeviceStatus("healthy"), DeviceHealthy)
	assert.Equal(t, DeviceStatus("degraded"), DeviceDegraded)
	assert.Equal(t, DeviceStatus("failed"), DeviceFailed)
}

func TestSASDevice_Fields(t *testing.T) {
	now := time.Now()
	device := SASDevice{
		WWN:    "5000000000000001",
		Model:  "Samsung PM1643",
		Serial: "SAMSUNG123456",
		Paths:  []*Path{},
		Policy: PolicyRoundRobin,
		Status: DeviceHealthy,
		LastFailover: &now,
	}

	assert.Equal(t, "5000000000000001", device.WWN)
	assert.Equal(t, "Samsung PM1643", device.Model)
	assert.Equal(t, PolicyRoundRobin, device.Policy)
	assert.Equal(t, DeviceHealthy, device.Status)
	assert.NotNil(t, device.LastFailover)
}

func TestPath_Fields(t *testing.T) {
	now := time.Now()
	path := Path{
		ID:              "0:0:0:0",
		HostAdapter:     0,
		Channel:         0,
		TargetID:        0,
		LUN:             0,
		DeviceNode:      "/dev/sda",
		SASAddress:      "5000000000000001",
		Controller:      "ctrl0",
		Status:          PathActive,
		IOPs:            1500,
		PendingIOs:      10,
		LatencyMs:       0.5,
		ErrorCount:      0,
		LastHealthCheck: now,
	}

	assert.Equal(t, "0:0:0:0", path.ID)
	assert.Equal(t, 0, path.HostAdapter)
	assert.Equal(t, "/dev/sda", path.DeviceNode)
	assert.Equal(t, PathActive, path.Status)
	assert.Equal(t, int64(1500), path.IOPs)
	assert.Equal(t, 0.5, path.LatencyMs)
}

func TestFailoverEvent_Fields(t *testing.T) {
	now := time.Now()
	event := FailoverEvent{
		Timestamp:   now,
		DeviceWWN:   "5000000000000001",
		FromPath:    "0:0:0:0",
		ToPath:      "1:0:0:0",
		Reason:      "path failed",
		DurationMs:  15,
	}

	assert.Equal(t, "5000000000000001", event.DeviceWWN)
	assert.Equal(t, "0:0:0:0", event.FromPath)
	assert.Equal(t, "1:0:0:0", event.ToPath)
	assert.Equal(t, "path failed", event.Reason)
	assert.Equal(t, int64(15), event.DurationMs)
}

func TestHealthCheckResult_Fields(t *testing.T) {
	now := time.Now()
	result := HealthCheckResult{
		PathID:    "0:0:0:0",
		Healthy:   true,
		LatencyMs: 0.3,
		Error:     "",
		CheckedAt: now,
	}

	assert.Equal(t, "0:0:0:0", result.PathID)
	assert.True(t, result.Healthy)
	assert.Equal(t, 0.3, result.LatencyMs)
	assert.Empty(t, result.Error)
}

func TestHealthCheckResult_Unhealthy(t *testing.T) {
	result := HealthCheckResult{
		PathID:    "0:0:0:0",
		Healthy:   false,
		LatencyMs: 0,
		Error:     "connection timeout",
		CheckedAt: time.Now(),
	}

	assert.False(t, result.Healthy)
	assert.Equal(t, "connection timeout", result.Error)
}

func TestManualFailoverRequest_Fields(t *testing.T) {
	req := ManualFailoverRequest{
		DeviceWWN:    "5000000000000001",
		TargetPathID: "1:0:0:0",
	}

	assert.Equal(t, "5000000000000001", req.DeviceWWN)
	assert.Equal(t, "1:0:0:0", req.TargetPathID)
}

func TestPolicyUpdateRequest_Fields(t *testing.T) {
	req := PolicyUpdateRequest{
		DeviceWWN: "5000000000000001",
		Policy:    PolicyLeastPending,
	}

	assert.Equal(t, "5000000000000001", req.DeviceWWN)
	assert.Equal(t, PolicyLeastPending, req.Policy)
}

func TestSASDevice_MultiplePaths(t *testing.T) {
	device := SASDevice{
		WWN: "5000000000000001",
		Paths: []*Path{
			{ID: "0:0:0:0", Status: PathActive, DeviceNode: "/dev/sda"},
			{ID: "1:0:0:0", Status: PathStandby, DeviceNode: "/dev/sdb"},
			{ID: "2:0:0:0", Status: PathFailed, DeviceNode: "/dev/sdc"},
		},
		ActivePath: &Path{ID: "0:0:0:0", Status: PathActive},
		Status:     DeviceDegraded,
	}

	assert.Len(t, device.Paths, 3)
	assert.Equal(t, DeviceDegraded, device.Status)
	assert.NotNil(t, device.ActivePath)
	assert.Equal(t, PathActive, device.ActivePath.Status)
}

func TestSASDevice_AllPathsFailed(t *testing.T) {
	device := SASDevice{
		WWN: "5000000000000001",
		Paths: []*Path{
			{ID: "0:0:0:0", Status: PathFailed},
			{ID: "1:0:0:0", Status: PathFailed},
		},
		Status: DeviceFailed,
	}

	assert.Len(t, device.Paths, 2)
	assert.Equal(t, DeviceFailed, device.Status)

	// 检查所有路径是否都失败
	allFailed := true
	for _, p := range device.Paths {
		if p.Status != PathFailed {
			allFailed = false
			break
		}
	}
	assert.True(t, allFailed)
}
