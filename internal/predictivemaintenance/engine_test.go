package predictivemaintenance

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEngine(t *testing.T) {
	e := New(Config{Enabled: true})
	assert.NotNil(t, e)
	assert.Equal(t, 30, e.cfg.CheckIntervalMin)
	assert.Equal(t, 30.0, e.cfg.AlertThreshold)
}

func TestRegisterComponent(t *testing.T) {
	e := New(Config{Enabled: true})
	e.RegisterComponent("cpu-0", ComponentCPU, "主CPU")

	comp, err := e.GetHealth("cpu-0")
	require.NoError(t, err)
	assert.Equal(t, "cpu-0", comp.ID)
	assert.Equal(t, ComponentCPU, comp.Type)
	assert.Equal(t, StatusHealthy, comp.Status)
	assert.Equal(t, 100.0, comp.HealthScore)
}

func TestGetHealth_NotFound(t *testing.T) {
	e := New(Config{Enabled: true})
	_, err := e.GetHealth("nonexistent")
	assert.Error(t, err)
}

func TestListComponents(t *testing.T) {
	e := New(Config{Enabled: true})
	e.RegisterComponent("cpu-0", ComponentCPU, "CPU")
	e.RegisterComponent("mem-0", ComponentMemory, "内存")

	comps := e.ListComponents()
	assert.Len(t, comps, 2)
}

func TestRecordMetric(t *testing.T) {
	e := New(Config{Enabled: true})
	e.RegisterComponent("cpu-0", ComponentCPU, "CPU")

	for i := 0; i < 15; i++ {
		e.RecordMetric("cpu-0", float64(50+i))
	}

	e.mu.RLock()
	history := e.history["cpu-0"]
	e.mu.RUnlock()
	assert.Len(t, history, 15)
}

func TestPredict(t *testing.T) {
	e := New(Config{Enabled: true})
	e.RegisterComponent("cpu-0", ComponentCPU, "CPU")

	// 记录线性增长数据
	for i := 0; i < 20; i++ {
		e.RecordMetric("cpu-0", float64(40+i*2))
	}

	ctx := context.Background()
	pred, err := e.Predict(ctx, "cpu-0")
	require.NoError(t, err)
	assert.NotEmpty(t, pred.ComponentID)
	assert.True(t, pred.Confidence > 0.9, "confidence should be high for linear data")
	assert.Equal(t, "rising", pred.Trend)
	assert.True(t, pred.DaysToFailure > 0)
}

func TestPredict_InsufficientData(t *testing.T) {
	e := New(Config{Enabled: true})
	e.RegisterComponent("cpu-0", ComponentCPU, "CPU")

	for i := 0; i < 5; i++ {
		e.RecordMetric("cpu-0", 50)
	}

	ctx := context.Background()
	_, err := e.Predict(ctx, "cpu-0")
	assert.Error(t, err)
}

func TestCreateSchedule(t *testing.T) {
	e := New(Config{Enabled: true})
	e.RegisterComponent("disk-0", ComponentDisk, "主盘")

	sched, err := e.CreateSchedule("disk-0", "preventive", "更换硬盘", "SMART预警，建议更换", 1)
	require.NoError(t, err)
	assert.NotEmpty(t, sched.ID)
	assert.Equal(t, "pending", sched.Status)
}

func TestCreateSchedule_ComponentNotFound(t *testing.T) {
	e := New(Config{Enabled: true})
	_, err := e.CreateSchedule("nonexistent", "preventive", "test", "test", 1)
	assert.Error(t, err)
}

func TestListSchedules(t *testing.T) {
	e := New(Config{Enabled: true})
	e.RegisterComponent("disk-0", ComponentDisk, "主盘")
	e.CreateSchedule("disk-0", "preventive", "维护1", "desc", 1)
	e.CreateSchedule("disk-0", "predictive", "维护2", "desc", 2)

	schedules := e.ListSchedules()
	assert.Len(t, schedules, 2)
}

func TestCheckAll(t *testing.T) {
	e := New(Config{Enabled: true})
	e.RegisterComponent("cpu-0", ComponentCPU, "CPU")
	e.RegisterComponent("mem-0", ComponentMemory, "内存")

	// 给 CPU 记录数据以便预测
	for i := 0; i < 15; i++ {
		e.RecordMetric("cpu-0", float64(50+i))
	}

	ctx := context.Background()
	results := e.CheckAll(ctx)
	assert.Len(t, results, 2)
}

func TestLinearRegression(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5}
	slope, intercept := linearRegression(data)
	assert.InDelta(t, 1.0, slope, 0.1)
	assert.InDelta(t, 1.0, intercept, 0.1)
}

func TestCalculateConfidence(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5}
	slope, intercept := linearRegression(data)
	conf := calculateConfidence(data, slope, intercept)
	assert.InDelta(t, 1.0, conf, 0.01)
}
