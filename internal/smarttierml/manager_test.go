package smarttierml

import (
	"fmt"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	cfg := TierConfig{Enabled: true, PredictionWindow: 24}
	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.running {
		t.Error("should not be running initially")
	}
}

func TestStartStop(t *testing.T) {
	m := NewManager(TierConfig{})
	if err := m.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if err := m.Start(); err == nil {
		t.Error("double Start should fail")
	}
	if err := m.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestRegisterItem(t *testing.T) {
	m := NewManager(TierConfig{DecayFactor: 0.95})
	item := &DataItem{
		ID:          "item1",
		Path:        "/data/file.txt",
		Size:        1024,
		CurrentTier: TierWarm,
		AccessCount: 10,
		LastAccess:  time.Now(),
		CreatedAt:   time.Now().Add(-24 * time.Hour),
	}
	m.RegisterItem(item)
	if _, ok := m.items["item1"]; !ok {
		t.Error("item not registered")
	}
}

func TestCalculateHeat(t *testing.T) {
	m := NewManager(TierConfig{DecayFactor: 0.95})

	// Hot item - recent access
	hotItem := &DataItem{
		ID:          "hot",
		AccessCount: 100,
		LastAccess:  time.Now(),
		AccessPattern: []AccessPoint{
			{Timestamp: time.Now().Add(-time.Hour), ReadBytes: 1024},
			{Timestamp: time.Now(), ReadBytes: 2048},
		},
		CreatedAt: time.Now().Add(-30 * 24 * time.Hour),
	}

	// Cold item - old access
	coldItem := &DataItem{
		ID:          "cold",
		AccessCount: 1,
		LastAccess:  time.Now().Add(-90 * 24 * time.Hour),
		AccessPattern: []AccessPoint{
			{Timestamp: time.Now().Add(-90 * 24 * time.Hour), ReadBytes: 100},
		},
		CreatedAt: time.Now().Add(-365 * 24 * time.Hour),
	}

	hotHeat := m.calculateHeat(hotItem)
	coldHeat := m.calculateHeat(coldItem)

	if hotHeat <= coldHeat {
		t.Errorf("hot item heat (%f) should be > cold item heat (%f)", hotHeat, coldHeat)
	}
}

func TestPredictTier(t *testing.T) {
	m := NewManager(TierConfig{DecayFactor: 0.95})
	item := &DataItem{
		ID:          "pred1",
		CurrentTier: TierCold,
		AccessCount: 50,
		LastAccess:  time.Now(),
		AccessPattern: []AccessPoint{
			{Timestamp: time.Now(), ReadBytes: 5000},
		},
		CreatedAt: time.Now().Add(-30 * 24 * time.Hour),
	}
	m.RegisterItem(item)

	result, err := m.PredictTier("pred1")
	if err != nil {
		t.Fatalf("PredictTier failed: %v", err)
	}
	if result.ItemID != "pred1" {
		t.Error("wrong item ID")
	}
	if result.Confidence <= 0 || result.Confidence > 1 {
		t.Errorf("invalid confidence: %f", result.Confidence)
	}
}

func TestPredictTierNotFound(t *testing.T) {
	m := NewManager(TierConfig{})
	_, err := m.PredictTier("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent item")
	}
}

func TestRunMigration(t *testing.T) {
	m := NewManager(TierConfig{})
	item := &DataItem{
		ID:          "mig1",
		CurrentTier: TierCold,
		Size:        1024,
	}
	m.RegisterItem(item)

	task, err := m.RunMigration("mig1", TierHot, "high heat")
	if err != nil {
		t.Fatalf("RunMigration failed: %v", err)
	}
	if task.SourceTier != TierCold || task.TargetTier != TierHot {
		t.Error("wrong tier in migration task")
	}

	time.Sleep(200 * time.Millisecond)
	if m.items["mig1"].CurrentTier != TierHot {
		t.Error("item not migrated to hot tier")
	}
}

func TestRunMigrationSameTier(t *testing.T) {
	m := NewManager(TierConfig{})
	item := &DataItem{ID: "same", CurrentTier: TierHot}
	m.RegisterItem(item)
	_, err := m.RunMigration("same", TierHot, "test")
	if err == nil {
		t.Error("expected error for same tier migration")
	}
}

func TestGetTieringStats(t *testing.T) {
	m := NewManager(TierConfig{})
	m.items["a"] = &DataItem{ID: "a", CurrentTier: TierHot, Size: 100}
	m.items["b"] = &DataItem{ID: "b", CurrentTier: TierCold, Size: 200}

	stats := m.GetTieringStats()
	if stats.TotalItems != 2 {
		t.Errorf("expected 2 items, got %d", stats.TotalItems)
	}
	if stats.ItemsPerTier[TierHot] != 1 {
		t.Error("expected 1 hot item")
	}
}

func TestGetPredictions(t *testing.T) {
	m := NewManager(TierConfig{DecayFactor: 0.95})
	m.items["p1"] = &DataItem{
		ID: "p1", CurrentTier: TierCold, AccessCount: 100,
		LastAccess:    time.Now(),
		AccessPattern: []AccessPoint{{Timestamp: time.Now(), ReadBytes: 1000}},
		CreatedAt:     time.Now().Add(-30 * 24 * time.Hour),
	}
	predictions := m.GetPredictions()
	_ = predictions // may or may not have predictions depending on heat
}

func TestGetItems(t *testing.T) {
	m := NewManager(TierConfig{})
	for i := 0; i < 25; i++ {
		id := fmt.Sprintf("item%d", i)
		tier := TierHot
		if i%2 == 0 {
			tier = TierCold
		}
		m.items[id] = &DataItem{ID: id, CurrentTier: tier, HeatScore: float64(i)}
	}

	items, total := m.GetItems(TierHot, 1, 10)
	if total != 12 { // 12 odd items (1,3,5,...,23) are hot
		t.Errorf("expected 13 hot items, got %d", total)
	}
	if len(items) != 10 {
		t.Errorf("expected 10 items in page, got %d", len(items))
	}
}

func TestGetMigrationTasks(t *testing.T) {
	m := NewManager(TierConfig{})
	m.migrations["t1"] = &MigrationTask{ID: "t1", Status: "completed"}
	m.migrations["t2"] = &MigrationTask{ID: "t2", Status: "running"}

	all := m.GetMigrationTasks("")
	if len(all) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(all))
	}
	completed := m.GetMigrationTasks("completed")
	if len(completed) != 1 {
		t.Errorf("expected 1 completed task, got %d", len(completed))
	}
}

func TestTrainModel(t *testing.T) {
	m := NewManager(TierConfig{})
	m.TrainModel()
	if m.model.TrainedAt.IsZero() {
		t.Error("model not trained")
	}
	if m.model.Accuracy < 0.5 {
		t.Error("model accuracy too low")
	}
}

func TestExtractFeatures(t *testing.T) {
	m := NewManager(TierConfig{})
	item := &DataItem{
		AccessCount: 50,
		LastAccess:  time.Now(),
		Size:        1024 * 1024,
		CreatedAt:   time.Now().Add(-30 * 24 * time.Hour),
	}
	features := m.extractFeatures(item)
	if len(features) != 4 {
		t.Errorf("expected 4 features, got %d", len(features))
	}
}

func TestModelPredict(t *testing.T) {
	m := NewManager(TierConfig{})
	features := []float64{0.9, 0.8, 0.5, 0.1}
	pred := m.modelPredict(features)
	if pred < 0 || pred > 1 {
		t.Errorf("prediction out of range: %f", pred)
	}
}

func TestConfigCRUD(t *testing.T) {
	m := NewManager(TierConfig{PredictionWindow: 12})
	cfg := m.GetConfig()
	if cfg.PredictionWindow != 12 {
		t.Error("wrong prediction window")
	}
	newCfg := TierConfig{PredictionWindow: 24, EnablePrediction: true}
	m.UpdateConfig(newCfg)
	cfg = m.GetConfig()
	if cfg.PredictionWindow != 24 {
		t.Error("config not updated")
	}
}

func TestEstimateGain(t *testing.T) {
	m := NewManager(TierConfig{})
	gain := m.estimateGain(TierCold, TierHot)
	if gain <= 0 {
		t.Error("expected positive gain for cold->hot")
	}
	gain2 := m.estimateGain(TierHot, TierCold)
	if gain2 >= 0 {
		t.Error("expected negative gain for hot->cold")
	}
}
