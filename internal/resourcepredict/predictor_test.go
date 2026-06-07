package resourcepredict

import (
	"testing"
	"time"
)

func TestDefaultPredictorConfig(t *testing.T) {
	cfg := DefaultPredictorConfig()
	if cfg.Thresholds.WarningDays != 30 {
		t.Errorf("expected warning 30 days, got %d", cfg.Thresholds.WarningDays)
	}
	if cfg.Thresholds.CriticalDays != 14 {
		t.Errorf("expected critical 14 days, got %d", cfg.Thresholds.CriticalDays)
	}
	if cfg.Thresholds.UrgentDays != 7 {
		t.Errorf("expected urgent 7 days, got %d", cfg.Thresholds.UrgentDays)
	}
	if cfg.RetentionDays != 90 {
		t.Errorf("expected 90 retention days, got %d", cfg.RetentionDays)
	}
}

func TestRegisterAndRecord(t *testing.T) {
	cfg := DefaultPredictorConfig()
	cfg.SamplingInterval = time.Hour
	rp := NewResourcePredictor(cfg)

	rp.RegisterResource(ResourceDisk, "/dev/sda1", "bytes", 100*1024*1024*1024)

	rp.RecordValue(ResourceDisk, 50*1024*1024*1024) // 50%
	rp.RecordValue(ResourceDisk, 55*1024*1024*1024) // 55%

	metrics := rp.GetMetrics()
	disk, ok := metrics[ResourceDisk]
	if !ok {
		t.Fatal("expected disk metric to exist")
	}
	if disk.CurrentValue != 55*1024*1024*1024 {
		t.Errorf("expected current value 55GB, got %f", disk.CurrentValue)
	}
	if len(disk.Points) != 2 {
		t.Errorf("expected 2 data points, got %d", len(disk.Points))
	}
}

func TestPredictionInsufficientData(t *testing.T) {
	cfg := DefaultPredictorConfig()
	cfg.SamplingInterval = time.Hour
	cfg.Thresholds.MinDataPoints = 5
	rp := NewResourcePredictor(cfg)

	rp.RegisterResource(ResourceMemory, "system", "bytes", 8*1024*1024*1024)

	// Only 3 data points (below minimum of 5)
	for i := 0; i < 3; i++ {
		rp.RecordValue(ResourceMemory, float64(4+i)*1024*1024*1024)
	}

	report := rp.PredictNow()
	if len(report.Predictions) != 1 {
		t.Fatalf("expected 1 prediction, got %d", len(report.Predictions))
	}
	if report.Predictions[0].AlertMessage == "" {
		t.Error("expected alert message about insufficient data")
	}
}

func TestPredictionWithIncreasingTrend(t *testing.T) {
	cfg := DefaultPredictorConfig()
	cfg.SamplingInterval = time.Hour
	cfg.Thresholds.MinDataPoints = 5
	rp := NewResourcePredictor(cfg)

	rp.RegisterResource(ResourceDisk, "/data", "bytes", 1000)

	// Simulate steady increase: 100, 200, 300, ..., 700
	baseTime := time.Now().Add(-6 * time.Hour)
	for i := 0; i < 7; i++ {
		rp.mu.Lock()
		metric := rp.metrics[ResourceDisk]
		metric.Points = append(metric.Points, DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Value:     float64(100 + i*100),
			Total:     1000,
		})
		metric.CurrentValue = 700
		rp.mu.Unlock()
	}

	report := rp.PredictNow()
	if len(report.Predictions) != 1 {
		t.Fatalf("expected 1 prediction, got %d", len(report.Predictions))
	}

	pred := report.Predictions[0]
	if !pred.IsIncreasing {
		t.Error("expected increasing trend")
	}
	if pred.DaysUntilFull <= 0 {
		t.Errorf("expected positive days until full, got %f", pred.DaysUntilFull)
	}
	if pred.TrendR2 < 0.5 {
		t.Errorf("expected R² > 0.5 for linear data, got %f", pred.TrendR2)
	}
}

func TestAlertLevelDetermination(t *testing.T) {
	cfg := DefaultPredictorConfig()
	rp := NewResourcePredictor(cfg)

	tests := []struct {
		days     float64
		usage    float64
		expected AlertLevel
	}{
		{100, 50, AlertInfo},
		{20, 60, AlertWarning},
		{10, 70, AlertCritical},
		{5, 80, AlertUrgent},
		{100, 96, AlertUrgent}, // >95% usage triggers urgent regardless
	}

	for _, tt := range tests {
		level := rp.determineAlertLevel(tt.days, tt.usage)
		if level != tt.expected {
			t.Errorf("days=%f usage=%f: expected %s, got %s", tt.days, tt.usage, tt.expected, level)
		}
	}
}

func TestPredictionWithDecreasingTrend(t *testing.T) {
	cfg := DefaultPredictorConfig()
	cfg.SamplingInterval = time.Hour
	cfg.Thresholds.MinDataPoints = 5
	rp := NewResourcePredictor(cfg)

	rp.RegisterResource(ResourceDisk, "/data", "bytes", 1000)

	// Simulate decreasing usage
	baseTime := time.Now().Add(-6 * time.Hour)
	for i := 0; i < 7; i++ {
		rp.mu.Lock()
		metric := rp.metrics[ResourceDisk]
		metric.Points = append(metric.Points, DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Value:     float64(700 - i*50),
			Total:     1000,
		})
		metric.CurrentValue = 400
		rp.mu.Unlock()
	}

	report := rp.PredictNow()
	pred := report.Predictions[0]
	if pred.IsIncreasing {
		t.Error("expected decreasing trend")
	}
	if pred.AlertLevel != AlertInfo {
		t.Errorf("expected info alert for decreasing trend, got %s", pred.AlertLevel)
	}
}

func TestConfidenceScore(t *testing.T) {
	cfg := DefaultPredictorConfig()
	cfg.SamplingInterval = time.Hour
	cfg.Thresholds.MinDataPoints = 5
	rp := NewResourcePredictor(cfg)

	rp.RegisterResource(ResourceDisk, "/data", "bytes", 1000)

	// Perfect linear data = high confidence
	baseTime := time.Now().Add(-20 * time.Hour)
	for i := 0; i < 20; i++ {
		rp.mu.Lock()
		metric := rp.metrics[ResourceDisk]
		metric.Points = append(metric.Points, DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Value:     float64(100 + i*10), // perfect linear
			Total:     1000,
		})
		metric.CurrentValue = 290
		rp.mu.Unlock()
	}

	report := rp.PredictNow()
	pred := report.Predictions[0]
	if pred.Confidence < 0.7 {
		t.Errorf("expected high confidence for linear data, got %f", pred.Confidence)
	}
}

func TestSummaryGeneration(t *testing.T) {
	cfg := DefaultPredictorConfig()
	rp := NewResourcePredictor(cfg)

	tests := []struct {
		predictions []Prediction
		contains    string
	}{
		{
			[]Prediction{{AlertLevel: AlertInfo}, {AlertLevel: AlertInfo}},
			"正常",
		},
		{
			[]Prediction{{AlertLevel: AlertWarning}},
			"警告",
		},
		{
			[]Prediction{{AlertLevel: AlertCritical}, {AlertLevel: AlertWarning}},
			"严重",
		},
		{
			[]Prediction{{AlertLevel: AlertUrgent}},
			"紧急",
		},
	}

	for _, tt := range tests {
		summary := rp.generateSummary(tt.predictions)
		if len(summary) == 0 {
			t.Error("expected non-empty summary")
		}
	}
}

func TestStartStop(t *testing.T) {
	cfg := DefaultPredictorConfig()
	cfg.SamplingInterval = 50 * time.Millisecond
	rp := NewResourcePredictor(cfg)

	rp.RegisterResource(ResourceCPU, "system", "percent", 100)

	rp.Start()
	if !rp.running {
		t.Error("expected predictor to be running")
	}

	// Double start should be no-op
	rp.Start()

	rp.Stop()
	if rp.running {
		t.Error("expected predictor to be stopped")
	}
}

func TestAlertCallback(t *testing.T) {
	cfg := DefaultPredictorConfig()
	cfg.SamplingInterval = time.Hour
	cfg.Thresholds.MinDataPoints = 3
	rp := NewResourcePredictor(cfg)

	alertReceived := false
	rp.SetAlertCallback(func(p Prediction) {
		alertReceived = true
	})

	rp.RegisterResource(ResourceDisk, "/data", "bytes", 1000)

	// Fill with data that shows increasing usage approaching capacity
	// This creates a trend from 900 to 960 over 5 hours, which will eventually exceed 1000
	baseTime := time.Now().Add(-5 * time.Hour)
	for i := 0; i < 5; i++ {
		rp.mu.Lock()
		metric := rp.metrics[ResourceDisk]
		metric.Points = append(metric.Points, DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Value:     float64(900 + i*15),
			Total:     1000,
		})
		metric.CurrentValue = 960
		rp.mu.Unlock()
	}

	rp.PredictNow()
	if !alertReceived {
		t.Error("expected alert callback to be triggered")
	}
}
