package snmpagent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgent_DefaultConfig(t *testing.T) {
	a := NewAgent(nil)
	assert.NotNil(t, a)
	assert.Equal(t, ":161", a.config.ListenAddr)
	assert.Equal(t, "public", a.config.Community)
	assert.Equal(t, "2c", a.config.Version)
	assert.True(t, a.config.Enabled)
}

func TestNewAgent_CustomConfig(t *testing.T) {
	cfg := &SNMPConfig{
		ListenAddr: ":1161",
		Community:  "private",
		Version:    "3",
		Username:   "admin",
		AuthKey:    "authkey123",
		PrivKey:    "privkey456",
		Enabled:    false,
	}
	a := NewAgent(cfg)
	assert.Equal(t, ":1161", a.config.ListenAddr)
	assert.Equal(t, "private", a.config.Community)
	assert.Equal(t, "3", a.config.Version)
	assert.Equal(t, "admin", a.config.Username)
}

func TestNewAgent_DefaultMetricsRegistered(t *testing.T) {
	a := NewAgent(nil)
	metrics := a.ListMetrics()
	assert.GreaterOrEqual(t, len(metrics), 10, "should have at least 10 default metrics")

	oidMap := make(map[string]bool)
	for _, m := range metrics {
		oidMap[m.OID] = true
	}
	assert.True(t, oidMap["1.3.6.1.4.1.2021.11.9.0"], "CPU user metric should be registered")
	assert.True(t, oidMap["1.3.6.1.4.1.2021.4.5.0"], "mem_total metric should be registered")
}

func TestStartStop(t *testing.T) {
	a := NewAgent(nil)

	err := a.Start()
	require.NoError(t, err)
	assert.True(t, a.running)

	// Starting again should fail
	err = a.Start()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	err = a.Stop()
	require.NoError(t, err)
	assert.False(t, a.running)

	// Stopping again should fail
	err = a.Stop()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not running")
}

func TestRegisterAndGetMetric(t *testing.T) {
	a := NewAgent(nil)

	err := a.RegisterMetric("1.3.6.1.99.1.0", "test_metric", 42, "gauge", map[string]string{"env": "test"})
	require.NoError(t, err)

	m, err := a.GetMetric("1.3.6.1.99.1.0")
	require.NoError(t, err)
	assert.Equal(t, "test_metric", m.Name)
	assert.Equal(t, 42, m.Value)
	assert.Equal(t, "gauge", m.Type)
	assert.Equal(t, "test", m.Labels["env"])
}

func TestRegisterDuplicateMetric(t *testing.T) {
	a := NewAgent(nil)

	err := a.RegisterMetric("1.3.6.1.99.2.0", "dup_metric", 1, "gauge", nil)
	require.NoError(t, err)

	err = a.RegisterMetric("1.3.6.1.99.2.0", "dup_metric2", 2, "counter", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestRegisterInvalidMetricType(t *testing.T) {
	a := NewAgent(nil)
	err := a.RegisterMetric("1.3.6.1.99.3.0", "bad_type", 1, "histogram", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid metric type")
}

func TestUpdateMetric(t *testing.T) {
	a := NewAgent(nil)

	err := a.RegisterMetric("1.3.6.1.99.4.0", "update_test", 10, "gauge", nil)
	require.NoError(t, err)

	err = a.UpdateMetric("1.3.6.1.99.4.0", 99)
	require.NoError(t, err)

	m, err := a.GetMetric("1.3.6.1.99.4.0")
	require.NoError(t, err)
	assert.Equal(t, 99, m.Value)

	// Update non-existent metric
	err = a.UpdateMetric("1.3.6.1.99.999.0", 1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestUnregisterMetric(t *testing.T) {
	a := NewAgent(nil)

	err := a.RegisterMetric("1.3.6.1.99.5.0", "temp_metric", 0, "gauge", nil)
	require.NoError(t, err)

	err = a.UnregisterMetric("1.3.6.1.99.5.0")
	require.NoError(t, err)

	_, err = a.GetMetric("1.3.6.1.99.5.0")
	assert.Error(t, err)

	// Unregister non-existent
	err = a.UnregisterMetric("1.3.6.1.99.5.0")
	assert.Error(t, err)
}

func TestGetStatus(t *testing.T) {
	a := NewAgent(nil)
	status := a.GetStatus()

	assert.Equal(t, false, status["running"])
	assert.Equal(t, ":161", status["listen_addr"])
	assert.Equal(t, "public", status["community"])
	assert.Equal(t, "2c", status["version"])
	assert.GreaterOrEqual(t, status["metric_count"].(int), 10)
}

func TestGetConfigUpdateConfig(t *testing.T) {
	a := NewAgent(nil)
	cfg := a.GetConfig()
	assert.Equal(t, "public", cfg.Community)

	newCfg := SNMPConfig{
		ListenAddr: ":162",
		Community:  "newpublic",
		Version:    "3",
		Enabled:    true,
	}
	a.UpdateConfig(newCfg)

	cfg = a.GetConfig()
	assert.Equal(t, ":162", cfg.ListenAddr)
	assert.Equal(t, "newpublic", cfg.Community)
}

func TestGetMetricNotFound(t *testing.T) {
	a := NewAgent(nil)
	_, err := a.GetMetric("1.3.6.1.999.0")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegisterEmptyOID(t *testing.T) {
	a := NewAgent(nil)
	err := a.RegisterMetric("", "empty", 0, "gauge", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "OID cannot be empty")
}
