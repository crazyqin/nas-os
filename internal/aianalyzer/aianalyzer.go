package aianalyzer

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ContentAnalysis represents analysis results
type ContentAnalysis struct {
	ID           string    `json:"id"`
	FilePath     string    `json:"file_path"`
	Category     string    `json:"category"`
	Tags         []string  `json:"tags"`
	Summary      string    `json:"summary,omitempty"`
	Sentiment    string    `json:"sentiment,omitempty"`
	Language     string    `json:"language,omitempty"`
	Confidence   float64   `json:"confidence"`
	AnalyzedAt   time.Time `json:"analyzed_at"`
	ProcessingMs int64     `json:"processing_ms"`
}

// AnalysisConfig defines analysis behavior
type AnalysisConfig struct {
	EnableOCR       bool    `json:"enable_ocr"`
	EnableSummary   bool    `json:"enable_summary"`
	EnableTags      bool    `json:"enable_tags"`
	EnableSentiment bool    `json:"enable_sentiment"`
	MinConfidence   float64 `json:"min_confidence"`
	MaxFileSize     int64   `json:"max_file_size"`
}

// AIContentAnalyzer provides AI-powered content analysis
// Inspired by Synology Photos AI and AI Office
type AIContentAnalyzer struct {
	mu       sync.RWMutex
	analyses map[string]*ContentAnalysis
	config   AnalysisConfig
	running  bool
	stopCh   chan struct{}
	workers  int
	queue    chan string
}

// NewAIContentAnalyzer creates a new AIContentAnalyzer instance
func NewAIContentAnalyzer(workers int) *AIContentAnalyzer {
	if workers <= 0 {
		workers = 4
	}
	return &AIContentAnalyzer{
		analyses: make(map[string]*ContentAnalysis),
		config: AnalysisConfig{
			EnableOCR:       true,
			EnableSummary:   true,
			EnableTags:      true,
			EnableSentiment: false,
			MinConfidence:   0.7,
			MaxFileSize:     100 * 1024 * 1024, // 100MB
		},
		stopCh:  make(chan struct{}),
		workers: workers,
		queue:   make(chan string, 1000),
	}
}

// SetConfig updates the analysis configuration
func (aca *AIContentAnalyzer) SetConfig(config AnalysisConfig) {
	aca.mu.Lock()
	defer aca.mu.Unlock()
	aca.config = config
}

// AnalyzeFile queues a file for analysis
func (aca *AIContentAnalyzer) AnalyzeFile(ctx context.Context, filePath string) error {
	aca.mu.RLock()
	if !aca.running {
		aca.mu.RUnlock()
		return fmt.Errorf("analyzer not running")
	}
	aca.mu.RUnlock()

	select {
	case aca.queue <- filePath:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// GetAnalysis returns the analysis result for a file
func (aca *AIContentAnalyzer) GetAnalysis(filePath string) (*ContentAnalysis, error) {
	aca.mu.RLock()
	defer aca.mu.RUnlock()

	analysis, exists := aca.analyses[filePath]
	if !exists {
		return nil, fmt.Errorf("analysis not found for: %s", filePath)
	}
	return analysis, nil
}

// ListAnalyses returns all analysis results
func (aca *AIContentAnalyzer) ListAnalyses() []*ContentAnalysis {
	aca.mu.RLock()
	defer aca.mu.RUnlock()

	results := make([]*ContentAnalysis, 0, len(aca.analyses))
	for _, a := range aca.analyses {
		results = append(results, a)
	}
	return results
}

// SearchByTag searches analyses by tag
func (aca *AIContentAnalyzer) SearchByTag(tag string) []*ContentAnalysis {
	aca.mu.RLock()
	defer aca.mu.RUnlock()

	var results []*ContentAnalysis
	for _, a := range aca.analyses {
		for _, t := range a.Tags {
			if t == tag {
				results = append(results, a)
				break
			}
		}
	}
	return results
}

// SearchByCategory searches analyses by category
func (aca *AIContentAnalyzer) SearchByCategory(category string) []*ContentAnalysis {
	aca.mu.RLock()
	defer aca.mu.RUnlock()

	var results []*ContentAnalysis
	for _, a := range aca.analyses {
		if a.Category == category {
			results = append(results, a)
		}
	}
	return results
}

// Start begins the analysis workers
func (aca *AIContentAnalyzer) Start(ctx context.Context) error {
	aca.mu.Lock()
	if aca.running {
		aca.mu.Unlock()
		return fmt.Errorf("already running")
	}
	aca.running = true
	aca.mu.Unlock()

	for i := 0; i < aca.workers; i++ {
		go aca.worker(ctx, i)
	}
	return nil
}

// Stop stops the analysis workers
func (aca *AIContentAnalyzer) Stop() {
	aca.mu.Lock()
	defer aca.mu.Unlock()
	if aca.running {
		close(aca.stopCh)
		aca.running = false
	}
}

// GetStats returns analyzer statistics
func (aca *AIContentAnalyzer) GetStats() map[string]interface{} {
	aca.mu.RLock()
	defer aca.mu.RUnlock()

	return map[string]interface{}{
		"total_analyses": len(aca.analyses),
		"queue_length":   len(aca.queue),
		"workers":        aca.workers,
		"running":        aca.running,
	}
}

func (aca *AIContentAnalyzer) worker(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-aca.stopCh:
			return
		case filePath, ok := <-aca.queue:
			if !ok {
				return
			}
			aca.processFile(ctx, filePath)
		}
	}
}

func (aca *AIContentAnalyzer) processFile(ctx context.Context, filePath string) {
	start := time.Now()

	analysis := &ContentAnalysis{
		ID:       fmt.Sprintf("analysis-%d", time.Now().UnixNano()),
		FilePath: filePath,
	}

	// Simulate analysis
	time.Sleep(100 * time.Millisecond)

	analysis.Category = "document"
	analysis.Tags = []string{"processed"}
	analysis.Confidence = 0.95
	analysis.AnalyzedAt = time.Now()
	analysis.ProcessingMs = time.Since(start).Milliseconds()

	aca.mu.Lock()
	aca.analyses[filePath] = analysis
	aca.mu.Unlock()
}
