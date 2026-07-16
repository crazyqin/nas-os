// Package opscenter 单元测试
package opscenter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 30, cfg.HeartbeatSec)
	assert.Equal(t, 100, cfg.MaxNodes)
}

func TestOpsCenter_RegisterNode(t *testing.T) {
	oc := New(DefaultConfig())
	node := &NASNode{
		ID:       "node-1",
		Hostname: "nas-primary",
		IP:       "192.168.1.100",
		Version:  "v2.506.0",
		Status:   NodeOnline,
	}
	err := oc.RegisterNode(node)
	assert.NoError(t, err)

	nodes := oc.GetNodes()
	assert.Len(t, nodes, 1)
	assert.Equal(t, "nas-primary", nodes[0].Hostname)
}

func TestOpsCenter_MaxNodes(t *testing.T) {
	cfg := DefaultConfig()
	cfg.MaxNodes = 2
	oc := New(cfg)

	oc.RegisterNode(&NASNode{ID: "n1", Hostname: "h1"})
	oc.RegisterNode(&NASNode{ID: "n2", Hostname: "h2"})
	err := oc.RegisterNode(&NASNode{ID: "n3", Hostname: "h3"})
	assert.Error(t, err)
}

func TestOpsCenter_Heartbeat(t *testing.T) {
	oc := New(DefaultConfig())
	oc.RegisterNode(&NASNode{ID: "node-1", Hostname: "h1", Status: NodeOffline})

	err := oc.Heartbeat("node-1")
	assert.NoError(t, err)

	nodes := oc.GetNodes()
	assert.Equal(t, NodeOnline, nodes[0].Status)
}

func TestOpsCenter_UpdateNodeStatus_TriggersAlerts(t *testing.T) {
	oc := New(DefaultConfig())
	oc.RegisterNode(&NASNode{ID: "node-1", Hostname: "h1", Status: NodeOnline})

	oc.UpdateNodeStatus("node-1", 95, 92, 85)

	alerts := oc.GetAlerts("", false)
	assert.True(t, len(alerts) >= 3) // CPU, Memory, Temperature
}

func TestOpsCenter_Dashboard(t *testing.T) {
	oc := New(DefaultConfig())
	oc.RegisterNode(&NASNode{ID: "n1", Hostname: "h1", Status: NodeOnline, CPUPercent: 50, MemPercent: 60})
	oc.RegisterNode(&NASNode{ID: "n2", Hostname: "h2", Status: NodeOffline})

	dash := oc.GetDashboard()
	assert.Equal(t, 2, dash.TotalNodes)
	assert.Equal(t, 1, dash.OnlineNodes)
	assert.Equal(t, 1, dash.OfflineNodes)
	assert.Equal(t, 25.0, dash.AvgCPU) // (50 + 0) / 2
}

func TestOpsCenter_AcknowledgeAlert(t *testing.T) {
	oc := New(DefaultConfig())
	oc.RegisterNode(&NASNode{ID: "n1", Hostname: "h1", Status: NodeOnline})
	oc.UpdateNodeStatus("n1", 95, 50, 40) // triggers CPU alert

	alerts := oc.GetAlerts("", false)
	assert.True(t, len(alerts) > 0)

	err := oc.AcknowledgeAlert(alerts[0].ID, "admin")
	assert.NoError(t, err)
}

func TestOpsCenter_ResolveAlert(t *testing.T) {
	oc := New(DefaultConfig())
	oc.RegisterNode(&NASNode{ID: "n1", Hostname: "h1", Status: NodeOnline})
	oc.UpdateNodeStatus("n1", 95, 50, 40)

	alerts := oc.GetAlerts("", false)
	err := oc.ResolveAlert(alerts[0].ID)
	assert.NoError(t, err)
}

func TestNodeStatus_Constants(t *testing.T) {
	assert.Equal(t, NodeStatus("online"), NodeOnline)
	assert.Equal(t, NodeStatus("offline"), NodeOffline)
	assert.Equal(t, NodeStatus("degraded"), NodeDegraded)
}

func TestSeverity_Constants(t *testing.T) {
	assert.Equal(t, Severity("info"), SeverityInfo)
	assert.Equal(t, Severity("critical"), SeverityCritical)
}

func TestHealthCheck_Fields(t *testing.T) {
	hc := &HealthCheck{
		NodeID: "node-1",
		Score:  95.0,
		Checks: []CheckResult{
			{Name: "cpu", Status: "ok", Value: 30, Threshold: 90},
		},
		CheckedAt: time.Now(),
	}
	assert.Equal(t, 95.0, hc.Score)
	assert.Len(t, hc.Checks, 1)
}
