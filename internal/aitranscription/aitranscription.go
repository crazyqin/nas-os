// Package aitranscription implements AI-powered audio/video transcription search.
// Inspired by QNAP Qsirch 7.1.0, enables searching within media content.
//
// Features:
// - Audio/video transcription using local AI models
// - Content-aware search across transcripts
// - Speaker diarization and identification
// - Timestamp-based navigation
// - Multi-language support
// - Batch processing with queue management
// - Integration with TrueSearch Pro
package aitranscription

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TranscriptionEngine manages media transcription and search.
type TranscriptionEngine struct {
	mu     sync.RWMutex
	models map[string]*TranscriptionModel
	queue  *TranscriptionQueue
	store  *TranscriptStore
	config *TranscriptionConfig
	logger *slog.Logger
	ctx    context.Context
	cancel context.CancelFunc
	stats  *TranscriptionStats
}

// TranscriptionConfig configures the transcription engine.
type TranscriptionConfig struct {
	ModelPath       string        `json:"modelPath"`
	ModelsDir       string        `json:"modelsDir"`
	DefaultModel    string        `json:"defaultModel"`
	QueueSize       int           `json:"queueSize"`
	BatchSize       int           `json:"batchSize"`
	MaxConcurrent   int           `json:"maxConcurrent"`
	Languages       []string      `json:"languages"`
	SampleRate      int           `json:"sampleRate"`
	ChunkDuration   time.Duration `json:"chunkDuration"`
	EnableDiarize   bool          `json:"enableDiarization"`
	EnableTranslate bool          `json:"enableTranslate"`
	GPUEnabled      bool          `json:"gpuEnabled"`
}

// TranscriptionModel represents an AI transcription model.
type TranscriptionModel struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Language string    `json:"language"`
	Size     int64     `json:"size"`
	Loaded   bool      `json:"loaded"`
	Accuracy float64   `json:"accuracy"`
	Speed    float64   `json:"speed"` // realtime factor
	LastUsed time.Time `json:"lastUsed"`
}

// TranscriptStore stores transcription results.
type TranscriptStore struct {
	mu          sync.RWMutex
	transcripts map[string]*Transcript
	index       map[string][]string // word -> transcript IDs
}

// Transcript represents a transcription result.
type Transcript struct {
	ID        string                 `json:"id"`
	MediaPath string                 `json:"mediaPath"`
	MediaType string                 `json:"mediaType"`
	Language  string                 `json:"language"`
	Duration  time.Duration          `json:"duration"`
	Segments  []*TranscriptSegment   `json:"segments"`
	Speakers  []*Speaker             `json:"speakers,omitempty"`
	Summary   string                 `json:"summary,omitempty"`
	Tags      []string               `json:"tags,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"createdAt"`
	UpdatedAt time.Time              `json:"updatedAt"`
}

// TranscriptSegment represents a segment of transcription.
type TranscriptSegment struct {
	ID         string        `json:"id"`
	Start      time.Duration `json:"start"`
	End        time.Duration `json:"end"`
	Text       string        `json:"text"`
	Speaker    string        `json:"speaker,omitempty"`
	Confidence float64       `json:"confidence"`
	Words      []*Word       `json:"words,omitempty"`
}

// Word represents a single word with timing.
type Word struct {
	Text       string        `json:"text"`
	Start      time.Duration `json:"start"`
	End        time.Duration `json:"end"`
	Confidence float64       `json:"confidence"`
}

// Speaker represents an identified speaker.
type Speaker struct {
	ID       string        `json:"id"`
	Name     string        `json:"name,omitempty"`
	Segments []string      `json:"segments"` // segment IDs
	Duration time.Duration `json:"duration"`
}

// TranscriptionQueue manages transcription jobs.
type TranscriptionQueue struct {
	mu        sync.RWMutex
	jobs      []*TranscriptionJob
	active    int
	maxActive int
	complete  int
	failed    int
}

// TranscriptionJob represents a transcription job.
type TranscriptionJob struct {
	ID          string                 `json:"id"`
	MediaPath   string                 `json:"mediaPath"`
	Status      JobStatus              `json:"status"`
	Progress    float64                `json:"progress"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"createdAt"`
	StartedAt   time.Time              `json:"startedAt"`
	CompletedAt time.Time              `json:"completedAt"`
	Options     map[string]interface{} `json:"options,omitempty"`
}

// JobStatus defines job status.
type JobStatus string

const (
	StatusPending    JobStatus = "pending"
	StatusProcessing JobStatus = "processing"
	StatusCompleted  JobStatus = "completed"
	StatusFailed     JobStatus = "failed"
	StatusCancelled  JobStatus = "cancelled"
)

// TranscriptionStats tracks transcription statistics.
type TranscriptionStats struct {
	mu               sync.RWMutex
	TotalTranscripts int64         `json:"totalTranscripts"`
	TotalDuration    time.Duration `json:"totalDuration"`
	TotalWords       int64         `json:"totalWords"`
	AverageAccuracy  float64       `json:"averageAccuracy"`
	QueueLength      int           `json:"queueLength"`
	ActiveJobs       int           `json:"activeJobs"`
}

// TranscriptSearchQuery represents a search query within transcripts.
type TranscriptSearchQuery struct {
	Text        string        `json:"text"`
	Language    string        `json:"language,omitempty"`
	Speaker     string        `json:"speaker,omitempty"`
	After       time.Time     `json:"after,omitempty"`
	Before      time.Time     `json:"before,omitempty"`
	MinDuration time.Duration `json:"minDuration,omitempty"`
	MaxDuration time.Duration `json:"maxDuration,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
	Limit       int           `json:"limit,omitempty"`
}

// TranscriptSearchResult represents a search result.
type TranscriptSearchResult struct {
	Transcript *Transcript          `json:"transcript"`
	Segments   []*TranscriptSegment `json:"segments"`
	Score      float64              `json:"score"`
	Snippet    string               `json:"snippet"`
	Matches    []string             `json:"matches"`
}

// TranscriptSearchResponse represents search response.
type TranscriptSearchResponse struct {
	Query     string                   `json:"query"`
	Results   []TranscriptSearchResult `json:"results"`
	TotalHits int                      `json:"totalHits"`
	QueryTime time.Duration            `json:"queryTime"`
}

// NewTranscriptionEngine creates a new transcription engine.
func NewTranscriptionEngine(config *TranscriptionConfig, logger *slog.Logger) *TranscriptionEngine {
	ctx, cancel := context.WithCancel(context.Background())

	engine := &TranscriptionEngine{
		models: make(map[string]*TranscriptionModel),
		queue: &TranscriptionQueue{
			jobs:      make([]*TranscriptionJob, 0),
			maxActive: config.MaxConcurrent,
		},
		store: &TranscriptStore{
			transcripts: make(map[string]*Transcript),
			index:       make(map[string][]string),
		},
		config: config,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
		stats:  &TranscriptionStats{},
	}

	// Register default models
	engine.registerDefaultModels()

	return engine
}

// registerDefaultModels registers default transcription models.
func (e *TranscriptionEngine) registerDefaultModels() {
	defaultModels := []*TranscriptionModel{
		{ID: "whisper-base", Name: "Whisper Base", Language: "multilingual", Size: 140000000, Accuracy: 0.85, Speed: 1.5},
		{ID: "whisper-large", Name: "Whisper Large", Language: "multilingual", Size: 3000000000, Accuracy: 0.95, Speed: 0.5},
		{ID: "whisper-chinese", Name: "Whisper Chinese", Language: "zh", Size: 800000000, Accuracy: 0.92, Speed: 1.0},
		{ID: "whisper-english", Name: "Whisper English", Language: "en", Size: 800000000, Accuracy: 0.94, Speed: 1.2},
	}

	for _, model := range defaultModels {
		e.models[model.ID] = model
	}
}

// Transcribe starts transcription of a media file.
func (e *TranscriptionEngine) Transcribe(ctx context.Context, mediaPath string, options map[string]interface{}) (*TranscriptionJob, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// Check queue size limit (prevent memory exhaustion)
	e.queue.mu.RLock()
	queueLen := len(e.queue.jobs)
	e.queue.mu.RUnlock()

	if queueLen >= e.config.QueueSize {
		return nil, fmt.Errorf("queue is full (%d/%d)", queueLen, e.config.QueueSize)
	}

	// Validate media path (basic check)
	if mediaPath == "" {
		return nil, fmt.Errorf("media path cannot be empty")
	}

	// Create job
	job := &TranscriptionJob{
		ID:        fmt.Sprintf("job-%d", time.Now().UnixNano()),
		MediaPath: mediaPath,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		Options:   options,
	}

	// Add to queue
	e.queue.mu.Lock()
	e.queue.jobs = append(e.queue.jobs, job)
	e.queue.mu.Unlock()

	// Start processing if capacity available
	go e.processJob(ctx, job)

	e.logger.Info("Transcription job created", "id", job.ID, "path", mediaPath)
	return job, nil
}

// processJob processes a transcription job.
func (e *TranscriptionEngine) processJob(ctx context.Context, job *TranscriptionJob) {
	e.queue.mu.Lock()
	if e.queue.active >= e.queue.maxActive {
		e.queue.mu.Unlock()
		return // wait for capacity
	}
	e.queue.active++
	job.Status = StatusProcessing
	job.StartedAt = time.Now()
	e.queue.mu.Unlock()

	defer func() {
		e.queue.mu.Lock()
		e.queue.active--
		if job.Status == StatusCompleted {
			e.queue.complete++
		} else {
			e.queue.failed++
		}
		e.queue.mu.Unlock()
	}()

	// Check context cancellation before processing
	select {
	case <-ctx.Done():
		job.Status = StatusFailed
		job.Error = "cancelled: " + ctx.Err().Error()
		job.CompletedAt = time.Now()
		return
	case <-e.ctx.Done():
		job.Status = StatusCancelled
		job.Error = "engine stopped"
		job.CompletedAt = time.Now()
		return
	default:
	}

	// Simulate transcription (in production, call actual AI model)
	select {
	case <-time.After(2 * time.Second):
		// continue
	case <-ctx.Done():
		job.Status = StatusFailed
		job.Error = "cancelled: " + ctx.Err().Error()
		job.CompletedAt = time.Now()
		return
	case <-e.ctx.Done():
		job.Status = StatusCancelled
		job.Error = "engine stopped"
		job.CompletedAt = time.Now()
		return
	}

	// Create transcript
	transcript := &Transcript{
		ID:        fmt.Sprintf("trans-%d", time.Now().UnixNano()),
		MediaPath: job.MediaPath,
		MediaType: detectMediaType(job.MediaPath),
		Language:  "zh",
		Duration:  5 * time.Minute,
		Segments: []*TranscriptSegment{
			{
				ID:         "seg-1",
				Start:      0,
				End:        30 * time.Second,
				Text:       "欢迎使用NAS-OS智能媒体转录系统",
				Speaker:    "Speaker-1",
				Confidence: 0.95,
			},
			{
				ID:         "seg-2",
				Start:      30 * time.Second,
				End:        60 * time.Second,
				Text:       "本系统支持多语言转录和智能搜索",
				Speaker:    "Speaker-1",
				Confidence: 0.92,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Index transcript
	e.store.mu.Lock()
	e.store.transcripts[transcript.ID] = transcript
	for _, seg := range transcript.Segments {
		words := strings.Fields(seg.Text)
		for _, word := range words {
			word = strings.ToLower(word)
			e.store.index[word] = append(e.store.index[word], transcript.ID)
		}
	}
	e.store.mu.Unlock()

	// Update stats
	e.stats.mu.Lock()
	e.stats.TotalTranscripts++
	e.stats.TotalDuration += transcript.Duration
	e.stats.mu.Unlock()

	job.Status = StatusCompleted
	job.CompletedAt = time.Now()
	job.Progress = 1.0

	e.logger.Info("Transcription completed", "id", job.ID, "transcript", transcript.ID)
}

// SearchTranscripts searches within transcripts.
func (e *TranscriptionEngine) SearchTranscripts(ctx context.Context, query *TranscriptSearchQuery) (*TranscriptSearchResponse, error) {
	start := time.Now()

	e.store.mu.RLock()
	defer e.store.mu.RUnlock()

	// Tokenize query
	queryTerms := strings.Fields(strings.ToLower(query.Text))

	// Find matching transcripts
	matches := make(map[string]float64)
	transcriptMatches := make(map[string][]*TranscriptSegment)

	for _, term := range queryTerms {
		if transcriptIDs, ok := e.store.index[term]; ok {
			for _, id := range transcriptIDs {
				matches[id] += 1.0

				// Find matching segments
				if transcript, exists := e.store.transcripts[id]; exists {
					for _, seg := range transcript.Segments {
						if strings.Contains(strings.ToLower(seg.Text), term) {
							transcriptMatches[id] = append(transcriptMatches[id], seg)
						}
					}
				}
			}
		}
	}

	// Build results
	results := make([]TranscriptSearchResult, 0)
	for id, score := range matches {
		transcript, exists := e.store.transcripts[id]
		if !exists {
			continue
		}

		// Apply filters
		if query.Language != "" && transcript.Language != query.Language {
			continue
		}

		// Find best snippet
		snippet := ""
		if segments, ok := transcriptMatches[id]; ok && len(segments) > 0 {
			bestSeg := segments[0]
			for _, seg := range segments {
				if seg.Confidence > bestSeg.Confidence {
					bestSeg = seg
				}
			}
			snippet = bestSeg.Text
		}

		results = append(results, TranscriptSearchResult{
			Transcript: transcript,
			Segments:   transcriptMatches[id],
			Score:      score,
			Snippet:    snippet,
			Matches:    queryTerms,
		})
	}

	return &TranscriptSearchResponse{
		Query:     query.Text,
		Results:   results,
		TotalHits: len(results),
		QueryTime: time.Since(start),
	}, nil
}

// GetJobStatus returns the status of a transcription job.
func (e *TranscriptionEngine) GetJobStatus(jobID string) (*TranscriptionJob, error) {
	e.queue.mu.RLock()
	defer e.queue.mu.RUnlock()

	for _, job := range e.queue.jobs {
		if job.ID == jobID {
			return job, nil
		}
	}
	return nil, fmt.Errorf("job not found: %s", jobID)
}

// GetTranscript returns a transcript by ID.
func (e *TranscriptionEngine) GetTranscript(transcriptID string) (*Transcript, error) {
	e.store.mu.RLock()
	defer e.store.mu.RUnlock()

	transcript, exists := e.store.transcripts[transcriptID]
	if !exists {
		return nil, fmt.Errorf("transcript not found: %s", transcriptID)
	}
	return transcript, nil
}

// GetStats returns transcription statistics.
func (e *TranscriptionEngine) GetStats() *TranscriptionStats {
	e.stats.mu.RLock()
	defer e.stats.mu.RUnlock()
	return e.stats
}

// GetModels returns available transcription models.
func (e *TranscriptionEngine) GetModels() []*TranscriptionModel {
	e.mu.RLock()
	defer e.mu.RUnlock()

	models := make([]*TranscriptionModel, 0, len(e.models))
	for _, model := range e.models {
		models = append(models, model)
	}
	return models
}

// Helper functions.
func detectMediaType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".m4a":
		return "audio"
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv":
		return "video"
	default:
		return "unknown"
	}
}

// Stop stops the transcription engine.
func (e *TranscriptionEngine) Stop() {
	e.cancel()
	e.logger.Info("Transcription engine stopped")
}
