package aianalyzer

import (
	"context"
	"testing"
	"time"
)

func TestNewAIContentAnalyzer(t *testing.T) {
	aca := NewAIContentAnalyzer(2)
	if aca == nil {
		t.Fatal("expected non-nil AIContentAnalyzer")
	}
	if aca.workers != 2 {
		t.Errorf("expected 2 workers, got %d", aca.workers)
	}
}

func TestSetConfig(t *testing.T) {
	aca := NewAIContentAnalyzer(1)

	config := AnalysisConfig{
		EnableOCR:     false,
		EnableSummary: true,
		MinConfidence: 0.8,
	}

	aca.SetConfig(config)
	// No error expected
}

func TestAnalyzeFileNotRunning(t *testing.T) {
	aca := NewAIContentAnalyzer(1)

	err := aca.AnalyzeFile(context.Background(), "/test/file.txt")
	if err == nil {
		t.Error("expected error when not running")
	}
}

func TestStartStop(t *testing.T) {
	aca := NewAIContentAnalyzer(1)
	ctx := context.Background()

	err := aca.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	err = aca.Start(ctx)
	if err == nil {
		t.Error("expected error on double start")
	}

	aca.Stop()
}

func TestAnalyzeFile(t *testing.T) {
	aca := NewAIContentAnalyzer(1)
	ctx := context.Background()

	aca.Start(ctx)
	defer aca.Stop()

	err := aca.AnalyzeFile(ctx, "/test/file.txt")
	if err != nil {
		t.Fatal(err)
	}

	// Wait for processing
	time.Sleep(500 * time.Millisecond)

	analysis, err := aca.GetAnalysis("/test/file.txt")
	if err != nil {
		t.Fatal(err)
	}

	if analysis.Category != "document" {
		t.Errorf("expected category 'document', got '%s'", analysis.Category)
	}
}

func TestSearchByTag(t *testing.T) {
	aca := NewAIContentAnalyzer(1)

	aca.analyses["/test/file.txt"] = &ContentAnalysis{
		ID:   "test",
		Tags: []string{"photo", "vacation"},
	}

	results := aca.SearchByTag("photo")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestSearchByCategory(t *testing.T) {
	aca := NewAIContentAnalyzer(1)

	aca.analyses["/test/file.txt"] = &ContentAnalysis{
		ID:       "test",
		Category: "image",
	}

	results := aca.SearchByCategory("image")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestGetStats(t *testing.T) {
	aca := NewAIContentAnalyzer(2)

	stats := aca.GetStats()
	if stats["workers"] != 2 {
		t.Errorf("expected 2 workers, got %v", stats["workers"])
	}
}
