package storagereporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	return NewManager(zap.NewNop(), t.TempDir())
}

func TestTakeSnapshot(t *testing.T) {
	mgr := setupTestManager(t)
	snap := mgr.TakeSnapshot(500*1024*1024*1024, 200*1024*1024*1024, map[string]int64{"photos": 50*1024*1024*1024})
	assert.True(t, snap.TotalBytes > 0)
	assert.True(t, snap.FreeBytes > 0)
	assert.Equal(t, int64(50*1024*1024*1024), snap.ByCategory["photos"])
}

func TestGetTrend(t *testing.T) {
	mgr := setupTestManager(t)
	_ = mgr.TakeSnapshot(100, 50, nil)
	report := mgr.GetTrend(7)
	assert.Equal(t, "7 days", report.Period)
	assert.Equal(t, "数据不足", report.Prediction)
}

func TestGetLatest(t *testing.T) {
	mgr := setupTestManager(t)
	assert.Nil(t, mgr.GetLatest())
	_ = mgr.TakeSnapshot(100, 50, nil)
	assert.NotNil(t, mgr.GetLatest())
}

func TestGetHistory(t *testing.T) {
	mgr := setupTestManager(t)
	for i := 0; i < 5; i++ {
		_ = mgr.TakeSnapshot(100, int64(50+i), nil)
	}
	history := mgr.GetHistory(3)
	assert.Len(t, history, 3)
}

func TestGenerateReport(t *testing.T) {
	mgr := setupTestManager(t)
	_ = mgr.TakeSnapshot(100, 50, nil)
	report := mgr.GenerateReport()
	assert.Contains(t, report, "current")
	assert.Contains(t, report, "trend_7d")
	assert.Contains(t, report, "trend_30d")
}

func TestAvgMaxMin(t *testing.T) {
	assert.Equal(t, 2.0, avg([]float64{1, 2, 3}))
	assert.Equal(t, 3.0, maxVal([]float64{1, 2, 3}))
	assert.Equal(t, 1.0, minVal([]float64{1, 2, 3}))
	assert.Equal(t, 0.0, avg(nil))
	assert.Equal(t, 0.0, maxVal(nil))
	assert.Equal(t, 0.0, minVal(nil))
}
