package dpdkaccel

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPortStates(t *testing.T) {
	assert.Equal(t, "down", string(PortStateDown))
	assert.Equal(t, "up", string(PortStateUp))
	assert.Equal(t, "error", string(PortStateError))
}

func TestRSSModes(t *testing.T) {
	assert.Equal(t, "disabled", string(RSSDisabled))
	assert.Equal(t, "default", string(RSSDefault))
	assert.Equal(t, "symmetric", string(RSSSymmetric))
	assert.Equal(t, "custom", string(RSSCustom))
}

func TestTrafficClasses(t *testing.T) {
	assert.Equal(t, "best_effort", string(TrafficClassBestEffort))
	assert.Equal(t, "bulk", string(TrafficClassBulk))
	assert.Equal(t, "low_latency", string(TrafficClassLowLatency))
	assert.Equal(t, "control", string(TrafficClassControl))
}

func TestRegisterPort(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	port := &Port{
		ID:       "port-0",
		Name:     "eth0",
		PCIeAddr: "0000:01:00.0",
		Speed:    10000,
		MTU:      9000,
		RXQueues: 4,
		TXQueues: 4,
		RSSMode:  RSSSymmetric,
	}

	err := mgr.RegisterPort(port)
	require.NoError(t, err)
	assert.Equal(t, PortStateDown, port.State)

	// Duplicate
	err = mgr.RegisterPort(port)
	assert.ErrorIs(t, err, ErrPortExists)
}

func TestStartStopPort(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	port := &Port{ID: "port-0", Name: "eth0"}
	_ = mgr.RegisterPort(port)

	err := mgr.StartPort("port-0")
	require.NoError(t, err)

	p, _ := mgr.GetPort("port-0")
	assert.Equal(t, PortStateUp, p.State)

	// Already started
	err = mgr.StartPort("port-0")
	assert.ErrorIs(t, err, ErrPortAlreadyStarted)

	err = mgr.StopPort("port-0")
	require.NoError(t, err)

	p, _ = mgr.GetPort("port-0")
	assert.Equal(t, PortStateDown, p.State)
}

func TestFlowRules(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	port := &Port{ID: "port-0", Name: "eth0"}
	_ = mgr.RegisterPort(port)

	rule := &FlowRule{
		ID:           "rule-001",
		PortID:       "port-0",
		Priority:     100,
		TrafficClass: TrafficClassLowLatency,
		DstPort:      443,
		Protocol:     "tcp",
		Action:       "allow",
		QueueID:      0,
	}

	err := mgr.AddFlowRule(rule)
	require.NoError(t, err)

	// Duplicate
	err = mgr.AddFlowRule(rule)
	assert.ErrorIs(t, err, ErrRuleExists)

	rules := mgr.ListFlowRules("port-0")
	assert.Len(t, rules, 1)

	err = mgr.RemoveFlowRule("rule-001")
	assert.NoError(t, err)

	rules = mgr.ListFlowRules("port-0")
	assert.Len(t, rules, 0)
}

func TestGetStats(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	port := &Port{
		ID:   "port-0",
		Name: "eth0",
		Stats: PortStats{
			RXPackets: 1000,
			TXPackets: 500,
		},
	}
	_ = mgr.RegisterPort(port)

	stats, err := mgr.GetStats("port-0")
	require.NoError(t, err)
	assert.Equal(t, uint64(1000), stats.RXPackets)
	assert.Equal(t, uint64(500), stats.TXPackets)
}

func TestManagerClosed(t *testing.T) {
	mgr := NewManager()
	mgr.Close()

	err := mgr.RegisterPort(&Port{ID: "port-0"})
	assert.ErrorIs(t, err, ErrManagerClosed)
}

func TestRemoveNonexistentRule(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	err := mgr.RemoveFlowRule("nonexistent")
	assert.ErrorIs(t, err, ErrRuleNotFound)
}

func TestStartNonexistentPort(t *testing.T) {
	mgr := NewManager()
	defer mgr.Close()

	err := mgr.StartPort("nonexistent")
	assert.ErrorIs(t, err, ErrPortNotFound)
}
