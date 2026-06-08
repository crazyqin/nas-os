package featureroadmap

import (
	"testing"
)

func TestNewFeatureRoadmap(t *testing.T) {
	rm := NewFeatureRoadmap()
	if rm == nil {
		t.Fatal("NewFeatureRoadmap returned nil")
	}
}

func TestAddFeature(t *testing.T) {
	rm := NewFeatureRoadmap()
	feat := rm.AddFeature(Feature{
		Title:       "智能分层引擎",
		Description: "统一存储分层",
		Category:    "storage",
		Priority:    "high",
		Assignee:    "兵部",
	})
	
	if feat.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if feat.Status != "planned" {
		t.Errorf("expected status 'planned', got %s", feat.Status)
	}
	
	features := rm.GetFeatures()
	if len(features) != 1 {
		t.Fatalf("expected 1 feature, got %d", len(features))
	}
}

func TestUpdateFeature(t *testing.T) {
	rm := NewFeatureRoadmap()
	feat := rm.AddFeature(Feature{
		Title:    "测试功能",
		Priority: "medium",
	})
	
	ok := rm.UpdateFeature(feat.ID, map[string]interface{}{
		"status":   "in_progress",
		"progress": 50.0,
	})
	if !ok {
		t.Fatal("expected true")
	}
	
	features := rm.GetFeatures()
	if features[0].Status != "in_progress" {
		t.Errorf("expected status 'in_progress', got %s", features[0].Status)
	}
	if features[0].Progress != 50 {
		t.Errorf("expected progress 50, got %d", features[0].Progress)
	}
}

func TestAddMilestone(t *testing.T) {
	rm := NewFeatureRoadmap()
	ms := rm.AddMilestone(Milestone{
		Name:    "v2.577.0",
		Version: "v2.577.0",
	})
	if ms.ID == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestGetStats(t *testing.T) {
	rm := NewFeatureRoadmap()
	rm.AddFeature(Feature{Title: "A", Priority: "high", Status: "planned"})
	rm.AddFeature(Feature{Title: "B", Priority: "low", Status: "released"})
	
	features := rm.GetFeatures()
	if len(features) != 2 {
		t.Fatalf("expected 2 features, got %d", len(features))
	}
	
	stats := rm.GetStats()
	if stats.TotalFeatures != 2 {
		t.Errorf("expected 2 features, got %d", stats.TotalFeatures)
	}
}
