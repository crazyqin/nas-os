package containerhealthmon

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), t.TempDir())
}

func TestRegisterContainer(t *testing.T) {
	mgr := setupTestManager(t)
	c := mgr.RegisterContainer("nginx", "nginx:latest")
	assert.Equal(t, "nginx", c.Name)
	assert.Equal(t, HealthUnknown, c.Health)
}

func TestUpdateHealth(t *testing.T) {
	mgr := setupTestManager(t)
	c := mgr.RegisterContainer("app", "app:v1")
	require.NoError(t, mgr.UpdateHealth(c.ID, HealthOK, 25.5, 512))
	updated := mgr.ListContainers("")[0]
	assert.Equal(t, HealthOK, updated.Health)
	assert.Equal(t, 25.5, updated.CPUPercent)
}

func TestIncrementRestart(t *testing.T) {
	mgr := setupTestManager(t)
	c := mgr.RegisterContainer("app", "app:v1")
	require.NoError(t, mgr.IncrementRestart(c.ID))
	updated := mgr.ListContainers("")[0]
	assert.Equal(t, 1, updated.RestartCount)
}

func TestListContainers(t *testing.T) {
	mgr := setupTestManager(t)
	_ = mgr.RegisterContainer("a", "img")
	_ = mgr.RegisterContainer("b", "img")
	require.NoError(t, mgr.UpdateHealth(mgr.ListContainers("")[0].ID, HealthOK, 0, 0))
	list := mgr.ListContainers(HealthOK)
	assert.Len(t, list, 1)
}

func TestCreateRule(t *testing.T) {
	mgr := setupTestManager(t)
	r := mgr.CreateRule("default", 80.0, 1024, 3)
	assert.Equal(t, "default", r.Name)
	assert.True(t, r.Enabled)
	assert.True(t, r.AutoRestart)
}

func TestGetEvents(t *testing.T) {
	mgr := setupTestManager(t)
	c := mgr.RegisterContainer("app", "img")
	require.NoError(t, mgr.UpdateHealth(c.ID, HealthCritical, 95.0, 2048))
	events := mgr.GetEvents(10)
	assert.True(t, len(events) >= 1)
}

func TestGetStats(t *testing.T) {
	mgr := setupTestManager(t)
	_ = mgr.RegisterContainer("a", "img")
	stats := mgr.GetStats()
	assert.Equal(t, 1, stats["total"])
}

func TestContainerNotFound(t *testing.T) {
	mgr := setupTestManager(t)
	assert.Error(t, mgr.UpdateHealth("nonexistent", HealthOK, 0, 0))
	assert.Error(t, mgr.IncrementRestart("nonexistent"))
}
