package dashboardwidget

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

func TestCreateWidget(t *testing.T) {
	mgr := setupTestManager(t)
	w := mgr.CreateWidget("CPU", WidgetTypeGauge, map[string]string{"source": "cpu"})
	assert.Equal(t, "CPU", w.Name)
	assert.Equal(t, WidgetTypeGauge, w.Type)
	assert.Equal(t, 30, w.RefreshSec)
}

func TestUpdateWidget(t *testing.T) {
	mgr := setupTestManager(t)
	w := mgr.CreateWidget("Test", WidgetTypeStat, nil)
	err := mgr.UpdateWidget(w.ID, &Position{X: 5, Y: 10}, &Size{Width: 8, Height: 6})
	require.NoError(t, err)
	updated := mgr.widgets[w.ID]
	assert.Equal(t, 5, updated.Position.X)
	assert.Equal(t, 8, updated.Size.Width)
}

func TestDeleteWidget(t *testing.T) {
	mgr := setupTestManager(t)
	w := mgr.CreateWidget("Del", WidgetTypeList, nil)
	d := mgr.CreateDashboard("D", "", "grid")
	_ = mgr.AddWidgetToDashboard(d.ID, w.ID)

	require.NoError(t, mgr.DeleteWidget(w.ID))
	assert.Nil(t, mgr.widgets[w.ID])
	assert.Len(t, mgr.dashboards[d.ID].Widgets, 0)
}

func TestDashboardOperations(t *testing.T) {
	mgr := setupTestManager(t)
	d := mgr.CreateDashboard("Main", "Main dashboard", "grid")
	assert.Equal(t, "Main", d.Name)

	w := mgr.CreateWidget("W1", WidgetTypeChart, nil)
	require.NoError(t, mgr.AddWidgetToDashboard(d.ID, w.ID))

	detail, widgets, err := mgr.GetDashboard(d.ID)
	require.NoError(t, err)
	assert.Len(t, detail.Widgets, 1)
	assert.Contains(t, widgets, w.ID)
}

func TestListDashboards(t *testing.T) {
	mgr := setupTestManager(t)
	_ = mgr.CreateDashboard("A", "", "grid")
	_ = mgr.CreateDashboard("B", "", "free")
	list := mgr.ListDashboards()
	assert.Len(t, list, 2)
}

func TestSystemWidgets(t *testing.T) {
	mgr := setupTestManager(t)
	widgets := mgr.GetSystemWidgets()
	assert.True(t, len(widgets) >= 6)
}

func TestWidgetNotFound(t *testing.T) {
	mgr := setupTestManager(t)
	assert.Error(t, mgr.UpdateWidget("nonexistent", nil, nil))
	assert.Error(t, mgr.DeleteWidget("nonexistent"))
	assert.Error(t, mgr.AddWidgetToDashboard("nonexistent", "x"))
}
