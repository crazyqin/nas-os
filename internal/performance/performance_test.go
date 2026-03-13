package performance

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestNewManager(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultConfig()

	m := NewManager(logger, config)
	if m == nil {
		t.Fatal("Expected manager to be created")
	}

	if m.collector == nil {
		t.Error("Expected collector to be initialized")
	}

	if m.storage == nil {
		t.Error("Expected storage to be initialized")
	}

	if m.health == nil {
		t.Error("Expected health checker to be initialized")
	}

	if m.alerts == nil {
		t.Error("Expected alert manager to be initialized")
	}
}

func TestSystemCollector(t *testing.T) {
	logger := zap.NewNop()
	collector := NewSystemCollector(logger, 10)

	// Test CPU collection
	cpu := collector.collectCPU()
	if cpu.Timestamp.IsZero() {
		t.Error("Expected CPU timestamp to be set")
	}
	if cpu.Cores <= 0 {
		t.Error("Expected CPU cores to be positive")
	}

	// Test Memory collection
	mem := collector.collectMemory()
	if mem.Timestamp.IsZero() {
		t.Error("Expected memory timestamp to be set")
	}
	if mem.TotalBytes == 0 {
		t.Error("Expected total memory to be non-zero")
	}

	// Test Disk collection
	disks := collector.collectDisks()
	// May be empty if no disks are mounted
	for _, d := range disks {
		if d.Device == "" {
			t.Error("Expected disk device to be set")
		}
	}

	// Test Network collection
	network := collector.collectNetwork()
	// May be empty
	for _, n := range network {
		if n.Interface == "" {
			t.Error("Expected network interface to be set")
		}
	}

	// Test Collect
	summary := collector.Collect()
	if summary == nil {
		t.Fatal("Expected summary to be returned")
	}

	// Test GetUptime
	uptime := collector.getUptime()
	if uptime == 0 {
		t.Error("Expected uptime to be non-zero")
	}
}

func TestHealthChecker(t *testing.T) {
	logger := zap.NewNop()
	collector := NewSystemCollector(logger, 10)
	storage := NewStorageCollector(logger, collector, 10)

	hc := NewHealthChecker(logger, collector, storage)

	// Run health check
	health := hc.Run()
	if health == nil {
		t.Fatal("Expected health result to be returned")
	}

	if health.Status == "" {
		t.Error("Expected health status to be set")
	}

	if health.Score < 0 || health.Score > 100 {
		t.Errorf("Expected health score to be between 0-100, got %d", health.Score)
	}

	if len(health.Checks) == 0 {
		t.Error("Expected health checks to be present")
	}
}

func TestAlertManager(t *testing.T) {
	logger := zap.NewNop()
	collector := NewSystemCollector(logger, 10)
	storage := NewStorageCollector(logger, collector, 10)
	health := NewHealthChecker(logger, collector, storage)

	am := NewAlertManager(logger, collector, storage, health)

	// Test GetRules
	rules := am.GetRules()
	if len(rules) == 0 {
		t.Error("Expected default rules to be present")
	}

	// Test GetAlerts
	alerts := am.GetAlerts()
	// May be empty initially
	_ = alerts

	// Test GetAlertStats
	stats := am.GetAlertStats()
	if stats == nil {
		t.Error("Expected stats to be returned")
	}
}

func TestStorageCollector(t *testing.T) {
	logger := zap.NewNop()
	collector := NewSystemCollector(logger, 10)
	storage := NewStorageCollector(logger, collector, 10)

	// Test Collect
	metrics := storage.Collect()
	if metrics == nil {
		t.Fatal("Expected storage metrics to be returned")
	}

	if metrics.Timestamp.IsZero() {
		t.Error("Expected timestamp to be set")
	}
}

func TestPerformanceMonitor(t *testing.T) {
	logger := zap.NewNop()
	pm := NewPerformanceMonitor(logger)

	// Test RecordAPICall
	pm.RecordAPICall("/api/test", "GET", 100*time.Millisecond, 200)
	pm.RecordAPICall("/api/test", "POST", 200*time.Millisecond, 201)
	pm.RecordAPICall("/api/error", "GET", 50*time.Millisecond, 500)

	// Test GetMetrics
	metrics := pm.GetMetrics()
	if metrics == nil {
		t.Fatal("Expected metrics to be returned")
	}

	if metrics.RequestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", metrics.RequestCount)
	}

	if metrics.SuccessCount != 2 {
		t.Errorf("Expected 2 successful requests, got %d", metrics.SuccessCount)
	}

	if metrics.ErrorCount != 1 {
		t.Errorf("Expected 1 error request, got %d", metrics.ErrorCount)
	}

	// Test Reset
	pm.Reset()
	metrics = pm.GetMetrics()
	if metrics.RequestCount != 0 {
		t.Errorf("Expected 0 requests after reset, got %d", metrics.RequestCount)
	}
}

func TestPrometheusExporter(t *testing.T) {
	logger := zap.NewNop()
	pm := NewPerformanceMonitor(logger)

	// Record some activity
	pm.RecordAPICall("/api/test", "GET", 100*time.Millisecond, 200)

	exporter := NewPrometheusExporter(pm)
	if exporter == nil {
		t.Fatal("Expected exporter to be created")
	}

	// Test extended constructor
	collector := NewSystemCollector(logger, 10)
	storage := NewStorageCollector(logger, collector, 10)
	health := NewHealthChecker(logger, collector, storage)
	alerts := NewAlertManager(logger, collector, storage, health)

	exporterExt := NewPrometheusExporterExtended(pm, collector, storage, health, alerts)
	if exporterExt == nil {
		t.Fatal("Expected extended exporter to be created")
	}
}

func TestAPIHandlers(t *testing.T) {
	logger := zap.NewNop()
	collector := NewSystemCollector(logger, 10)
	storage := NewStorageCollector(logger, collector, 10)
	health := NewHealthChecker(logger, collector, storage)
	alerts := NewAlertManager(logger, collector, storage, health)
	pm := NewPerformanceMonitor(logger)
	prom := NewPrometheusExporterExtended(pm, collector, storage, health, alerts)

	handlers := NewAPIHandlers(logger, collector, storage, health, alerts, prom, pm)
	if handlers == nil {
		t.Fatal("Expected API handlers to be created")
	}
}

func TestHealthThresholds(t *testing.T) {
	thresholds := DefaultHealthThresholds()

	if thresholds.CPUWarningPercent <= 0 {
		t.Error("Expected CPU warning threshold to be positive")
	}

	if thresholds.CPUCriticalPercent <= thresholds.CPUWarningPercent {
		t.Error("Expected CPU critical threshold to be higher than warning")
	}

	if thresholds.MemoryWarningPercent <= 0 {
		t.Error("Expected memory warning threshold to be positive")
	}
}

func TestAlertLevels(t *testing.T) {
	// Test alert level constants
	if AlertLevelInfo != "info" {
		t.Error("Expected info level")
	}
	if AlertLevelWarning != "warning" {
		t.Error("Expected warning level")
	}
	if AlertLevelCritical != "critical" {
		t.Error("Expected critical level")
	}
}

func TestHealthStatus(t *testing.T) {
	// Test health status constants
	if HealthStatusHealthy != "healthy" {
		t.Error("Expected healthy status")
	}
	if HealthStatusDegraded != "degraded" {
		t.Error("Expected degraded status")
	}
	if HealthStatusUnhealthy != "unhealthy" {
		t.Error("Expected unhealthy status")
	}
}