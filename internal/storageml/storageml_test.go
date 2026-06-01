package storageml

import (
	"testing"
	"time"
)

func TestNewStorageMLManager(t *testing.T) {
	mgr := NewStorageMLManager()
	if mgr == nil {
		t.Fatal("Expected non-nil manager")
	}
}

func TestAddDataPoint(t *testing.T) {
	mgr := NewStorageMLManager()

	dp := DataPoint{
		Timestamp: time.Now(),
		Value:     100.0,
		Type:      MetricCapacity,
		PoolID:    "test-pool",
	}

	mgr.AddDataPoint(dp)

	points := mgr.GetDataPoints("test-pool")
	if len(points) != 1 {
		t.Fatalf("Expected 1 data point, got %d", len(points))
	}
	if points[0].Value != 100.0 {
		t.Fatalf("Expected value 100.0, got %f", points[0].Value)
	}
}

func TestRegisterPool(t *testing.T) {
	mgr := NewStorageMLManager()

	config := PoolConfig{
		PoolID:        "test-pool",
		Name:          "Test Pool",
		TotalCapacity: 1000.0,
	}

	mgr.RegisterPool(config)

	retrieved, exists := mgr.GetPoolConfig("test-pool")
	if !exists {
		t.Fatal("Expected pool to exist")
	}
	if retrieved.Name != "Test Pool" {
		t.Fatalf("Expected name 'Test Pool', got '%s'", retrieved.Name)
	}
}

func TestLinearRegression(t *testing.T) {
	mgr := NewStorageMLManager()

	// Add increasing data points
	baseTime := time.Now()
	for i := 0; i < 10; i++ {
		mgr.AddDataPoint(DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Value:     float64(i) * 10.0,
			Type:      MetricCapacity,
			PoolID:    "test-pool",
		})
	}

	predictor := mgr.GetPredictor()
	futureDate := baseTime.Add(10 * time.Hour)
	result, err := predictor.Predict("test-pool", MetricCapacity, futureDate)
	if err != nil {
		t.Fatalf("Prediction failed: %v", err)
	}

	if result.TrendDirection != "increasing" {
		t.Fatalf("Expected increasing trend, got %s", result.TrendDirection)
	}

	if result.PredictedValue <= 0 {
		t.Fatalf("Expected positive predicted value, got %f", result.PredictedValue)
	}
}

func TestPredictInsufficientData(t *testing.T) {
	mgr := NewStorageMLManager()

	// Add only one data point
	mgr.AddDataPoint(DataPoint{
		Timestamp: time.Now(),
		Value:     100.0,
		Type:      MetricCapacity,
		PoolID:    "test-pool",
	})

	predictor := mgr.GetPredictor()
	_, err := predictor.Predict("test-pool", MetricCapacity, time.Now().Add(24*time.Hour))
	if err != ErrInsufficientData {
		t.Fatalf("Expected ErrInsufficientData, got %v", err)
	}
}

func TestAnalyzeTrend(t *testing.T) {
	mgr := NewStorageMLManager()

	// Add data points with increasing trend
	baseTime := time.Now().Add(-30 * 24 * time.Hour)
	for i := 0; i < 30; i++ {
		mgr.AddDataPoint(DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * 24 * time.Hour),
			Value:     100.0 + float64(i)*5.0,
			Type:      MetricCapacity,
			PoolID:    "test-pool",
		})
	}

	analyzer := mgr.GetAnalyzer()
	trend, err := analyzer.AnalyzeTrend("test-pool", MetricCapacity, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("Trend analysis failed: %v", err)
	}

	if trend.Direction != "increasing" {
		t.Fatalf("Expected increasing trend, got %s", trend.Direction)
	}
}

func TestDetectAnomalies(t *testing.T) {
	mgr := NewStorageMLManager()

	// Add normal data points
	baseTime := time.Now().Add(-24 * time.Hour)
	for i := 0; i < 20; i++ {
		mgr.AddDataPoint(DataPoint{
			Timestamp: baseTime.Add(time.Duration(i) * time.Hour),
			Value:     100.0 + float64(i%3),
			Type:      MetricCapacity,
			PoolID:    "test-pool",
		})
	}

	// Add anomalous point
	mgr.AddDataPoint(DataPoint{
		Timestamp: time.Now(),
		Value:     500.0, // Way above normal
		Type:      MetricCapacity,
		PoolID:    "test-pool",
	})

	analyzer := mgr.GetAnalyzer()
	anomalies, err := analyzer.DetectAnomalies("test-pool", MetricCapacity)
	if err != nil {
		t.Fatalf("Anomaly detection failed: %v", err)
	}

	if len(anomalies) == 0 {
		t.Fatal("Expected at least one anomaly")
	}
}

func TestGetUsageSummary(t *testing.T) {
	mgr := NewStorageMLManager()

	config := PoolConfig{
		PoolID:        "test-pool",
		Name:          "Test Pool",
		TotalCapacity: 1000.0,
	}
	mgr.RegisterPool(config)

	mgr.AddDataPoint(DataPoint{
		Timestamp: time.Now(),
		Value:     600.0,
		Type:      MetricCapacity,
		PoolID:    "test-pool",
	})

	analyzer := mgr.GetAnalyzer()
	summary := analyzer.GetUsageSummary("test-pool")

	if summary["pool_id"] != "test-pool" {
		t.Fatalf("Expected pool_id 'test-pool', got %v", summary["pool_id"])
	}
	if summary["usage_percent"] != 60.0 {
		t.Fatalf("Expected usage 60%%, got %v", summary["usage_percent"])
	}
}

func TestCleanupOldData(t *testing.T) {
	mgr := NewStorageMLManager()
	mgr.retentionDays = 1

	// Add old data point
	mgr.AddDataPoint(DataPoint{
		Timestamp: time.Now().AddDate(0, 0, -2),
		Value:     100.0,
		Type:      MetricCapacity,
		PoolID:    "test-pool",
	})

	// Add recent data point
	mgr.AddDataPoint(DataPoint{
		Timestamp: time.Now(),
		Value:     200.0,
		Type:      MetricCapacity,
		PoolID:    "test-pool",
	})

	mgr.CleanupOldData()

	points := mgr.GetDataPoints("test-pool")
	if len(points) != 1 {
		t.Fatalf("Expected 1 data point after cleanup, got %d", len(points))
	}
	if points[0].Value != 200.0 {
		t.Fatalf("Expected recent point to remain, got %f", points[0].Value)
	}
}
