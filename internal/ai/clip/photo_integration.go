// Package clip - Integration with photos module for AI album text search
package clip

import (
	"context"
	"fmt"
	"sync"
	"time"

	"nas-os/internal/ai/photos"
)

// PhotoSearchIndexer integrates CLIP with the photos module
type PhotoSearchIndexer struct {
	service TextSearchService
	loader  photos.ImageLoader
	storage photos.Storage
	config  *IndexerConfig
	queue   chan *indexJob
	workers int
	running bool
	stats   IndexerStats
	mu      sync.RWMutex
}

// IndexerConfig holds indexer configuration
type IndexerConfig struct {
	BatchSize    int           `json:"batch_size"`
	Workers      int           `json:"workers"`
	QueueSize    int           `json:"queue_size"`
	RetryCount   int           `json:"retry_count"`
	RetryDelay   time.Duration `json:"retry_delay"`
	SaveInterval time.Duration `json:"save_interval"`
}

// DefaultIndexerConfig returns defaults
func DefaultIndexerConfig() *IndexerConfig {
	return &IndexerConfig{
		BatchSize:    50,
		Workers:      4,
		QueueSize:    1000,
		RetryCount:   3,
		RetryDelay:   5 * time.Second,
		SaveInterval: 5 * time.Minute,
	}
}

// IndexerStats holds indexer statistics
type IndexerStats struct {
	TotalProcessed  int64         `json:"total_processed"`
	TotalFailed     int64         `json:"total_failed"`
	QueueSize       int           `json:"queue_size"`
	ProcessingSpeed float64       `json:"processing_speed"` // photos/sec
	LastProcessed   time.Time     `json:"last_processed"`
	AverageTime     time.Duration `json:"average_time"`
}

type indexJob struct {
	photo    *photos.Photo
	priority int
	retry    int
}

// NewPhotoSearchIndexer creates a new photo search indexer
func NewPhotoSearchIndexer(
	service TextSearchService,
	loader photos.ImageLoader,
	storage photos.Storage,
	config *IndexerConfig,
) *PhotoSearchIndexer {
	if config == nil {
		config = DefaultIndexerConfig()
	}

	return &PhotoSearchIndexer{
		service: service,
		loader:  loader,
		storage: storage,
		config:  config,
		queue:   make(chan *indexJob, config.QueueSize),
		workers: config.Workers,
	}
}

// Start starts the indexing workers
func (idx *PhotoSearchIndexer) Start(ctx context.Context) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if idx.running {
		return nil
	}

	idx.running = true

	// Start worker goroutines
	for i := 0; i < idx.workers; i++ {
		go idx.worker(ctx, i)
	}

	// Start periodic save goroutine
	go idx.periodicSave(ctx)

	return nil
}

// Stop stops the indexer
func (idx *PhotoSearchIndexer) Stop() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.running = false
	close(idx.queue)

	return nil
}

// QueuePhoto queues a photo for indexing
func (idx *PhotoSearchIndexer) QueuePhoto(photo *photos.Photo, priority int) error {
	idx.mu.RLock()
	running := idx.running
	idx.mu.RUnlock()

	if !running {
		return fmt.Errorf("indexer not running")
	}

	job := &indexJob{
		photo:    photo,
		priority: priority,
		retry:    0,
	}

	select {
	case idx.queue <- job:
		idx.stats.QueueSize = len(idx.queue)
		return nil
	default:
		return fmt.Errorf("queue full")
	}
}

// QueueBatch queues multiple photos for indexing
func (idx *PhotoSearchIndexer) QueueBatch(photos []*photos.Photo, priority int) error {
	for _, p := range photos {
		if err := idx.QueuePhoto(p, priority); err != nil {
			return err
		}
	}
	return nil
}

// GetStats returns indexer statistics
func (idx *PhotoSearchIndexer) GetStats() IndexerStats {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.stats
}

// worker processes indexing jobs
func (idx *PhotoSearchIndexer) worker(ctx context.Context, id int) {
	for job := range idx.queue {
		if ctx.Err() != nil {
			return
		}

		idx.processJob(ctx, job)
	}
}

// processJob processes a single indexing job
func (idx *PhotoSearchIndexer) processJob(ctx context.Context, job *indexJob) {
	start := time.Now()

	// Index the photo
	_, err := idx.service.Index(ctx, &IndexRequest{
		PhotoID: job.photo.ID,
		Path:    job.photo.Path,
		Tags:    job.photo.Tags,
	})

	if err != nil {
		// Retry on failure
		if job.retry < idx.config.RetryCount {
			job.retry++
			time.Sleep(idx.config.RetryDelay)

			// Re-queue with lower priority
			select {
			case idx.queue <- job:
			default:
				// Queue full, give up
				idx.recordFailure()
			}
			return
		}

		idx.recordFailure()
		return
	}

	// Update statistics
	elapsed := time.Since(start)
	idx.mu.Lock()
	idx.stats.TotalProcessed++
	idx.stats.LastProcessed = time.Now()

	// Update average processing time
	if idx.stats.AverageTime == 0 {
		idx.stats.AverageTime = elapsed
	} else {
		idx.stats.AverageTime = (idx.stats.AverageTime + elapsed) / 2
	}

	// Calculate processing speed
	if elapsed > 0 {
		speed := 1.0 / elapsed.Seconds()
		idx.stats.ProcessingSpeed = (idx.stats.ProcessingSpeed + speed) / 2
	}
	idx.mu.Unlock()
}

// recordFailure records a failed indexing attempt
func (idx *PhotoSearchIndexer) recordFailure() {
	idx.mu.Lock()
	idx.stats.TotalFailed++
	idx.mu.Unlock()
}

// periodicSave periodically saves the index
func (idx *PhotoSearchIndexer) periodicSave(ctx context.Context) {
	ticker := time.NewTicker(idx.config.SaveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Save index (if service supports it)
			if svc, ok := idx.service.(*TextSearchServiceImpl); ok {
				_ = svc.Close() // This will save
			}
		}
	}
}

// --- Photo Search Interface Implementation ---

// PhotoSearchService implements advanced photo search using CLIP
type PhotoSearchService struct {
	clipService TextSearchService
	photoStore  photos.Storage
	tagIndex    *TagIndex
}

// NewPhotoSearchService creates a new photo search service
func NewPhotoSearchService(clipService TextSearchService, store photos.Storage) *PhotoSearchService {
	return &PhotoSearchService{
		clipService: clipService,
		photoStore:  store,
		tagIndex:    NewTagIndex(),
	}
}

// SearchByText searches photos using natural language
func (s *PhotoSearchService) SearchByText(ctx context.Context, query string, topK int) (*photos.SearchResult, error) {
	// Use CLIP for semantic search
	resp, err := s.clipService.Search(ctx, &SearchRequest{
		Query: query,
		TopK:  topK,
	})
	if err != nil {
		return nil, err
	}

	// Convert results
	results := make([]*photos.Photo, 0, len(resp.Results))
	for _, r := range resp.Results {
		photo, err := s.photoStore.GetPhoto(ctx, r.PhotoID)
		if err != nil {
			continue
		}
		results = append(results, photo)
	}

	return &photos.SearchResult{
		Photos:    results,
		Total:     resp.Total,
		QueryTime: resp.QueryTime,
	}, nil
}

// SearchByTags searches photos by tags
func (s *PhotoSearchService) SearchByTags(ctx context.Context, tags []string, matchAll bool) ([]*photos.Photo, error) {
	photoIDs := s.tagIndex.Search(tags, matchAll)
	photos := make([]*photos.Photo, 0, len(photoIDs))

	for _, id := range photoIDs {
		photo, err := s.photoStore.GetPhoto(ctx, id)
		if err != nil {
			continue
		}
		photos = append(photos, photo)
	}

	return photos, nil
}

// SearchByPerson searches photos by person
func (s *PhotoSearchService) SearchByPerson(ctx context.Context, personID string) ([]*photos.Photo, error) {
	// Combine CLIP semantic search with person filter
	query := "person face portrait"
	resp, err := s.clipService.Search(ctx, &SearchRequest{
		Query: query,
		TopK:  100,
		Filters: &Filters{
			Tags: []string{personID},
		},
	})
	if err != nil {
		return nil, err
	}

	results := make([]*photos.Photo, 0, len(resp.Results))
	for _, r := range resp.Results {
		photo, err := s.photoStore.GetPhoto(ctx, r.PhotoID)
		if err != nil {
			continue
		}
		results = append(results, photo)
	}

	return results, nil
}

// SearchByLocation searches photos by location description
func (s *PhotoSearchService) SearchByLocation(ctx context.Context, location string) ([]*photos.Photo, error) {
	query := fmt.Sprintf("photo at %s", location)
	resp, err := s.clipService.Search(ctx, &SearchRequest{
		Query: query,
		TopK:  100,
	})
	if err != nil {
		return nil, err
	}

	results := make([]*photos.Photo, 0, len(resp.Results))
	for _, r := range resp.Results {
		photo, err := s.photoStore.GetPhoto(ctx, r.PhotoID)
		if err != nil {
			continue
		}
		results = append(results, photo)
	}

	return results, nil
}

// SearchByScene searches photos by scene description
func (s *PhotoSearchService) SearchByScene(ctx context.Context, scene string) ([]*photos.Photo, error) {
	resp, err := s.clipService.Search(ctx, &SearchRequest{
		Query: scene,
		TopK:  100,
	})
	if err != nil {
		return nil, err
	}

	results := make([]*photos.Photo, 0, len(resp.Results))
	for _, r := range resp.Results {
		photo, err := s.photoStore.GetPhoto(ctx, r.PhotoID)
		if err != nil {
			continue
		}
		results = append(results, photo)
	}

	return results, nil
}

// HybridSearch combines multiple search methods
func (s *PhotoSearchService) HybridSearch(ctx context.Context, query *HybridSearchQuery) (*photos.SearchResult, error) {
	// Build combined query
	textQuery := query.Text

	// Add scene hints
	if query.SceneHint != "" {
		textQuery = fmt.Sprintf("%s %s", textQuery, query.SceneHint)
	}

	// Use CLIP search
	resp, err := s.clipService.Search(ctx, &SearchRequest{
		Query:    textQuery,
		TopK:     query.Limit * 2, // Get more for filtering
		MinScore: query.MinScore,
	})
	if err != nil {
		return nil, err
	}

	// Apply additional filters
	results := make([]*photos.Photo, 0)
	for _, r := range resp.Results {
		photo, err := s.photoStore.GetPhoto(ctx, r.PhotoID)
		if err != nil {
			continue
		}

		// Apply date filter
		if !query.DateFrom.IsZero() && photo.TakenAt != nil {
			if photo.TakenAt.Before(query.DateFrom) {
				continue
			}
		}
		if !query.DateTo.IsZero() && photo.TakenAt != nil {
			if photo.TakenAt.After(query.DateTo) {
				continue
			}
		}

		// Apply tag filter
		if len(query.RequiredTags) > 0 {
			if !s.hasAllTags(photo, query.RequiredTags) {
				continue
			}
		}

		results = append(results, photo)

		if len(results) >= query.Limit {
			break
		}
	}

	return &photos.SearchResult{
		Photos:    results,
		Total:     len(results),
		QueryTime: resp.QueryTime,
	}, nil
}

func (s *PhotoSearchService) hasAllTags(photo *photos.Photo, tags []string) bool {
	tagSet := make(map[string]bool)
	for _, t := range photo.Tags {
		tagSet[t] = true
	}

	for _, t := range tags {
		if !tagSet[t] {
			return false
		}
	}

	return true
}

// HybridSearchQuery represents a hybrid search query
type HybridSearchQuery struct {
	Text         string    `json:"text"`
	SceneHint    string    `json:"scene_hint,omitempty"`
	DateFrom     time.Time `json:"date_from,omitempty"`
	DateTo       time.Time `json:"date_to,omitempty"`
	RequiredTags []string  `json:"required_tags,omitempty"`
	MinScore     float64   `json:"min_score,omitempty"`
	Limit        int       `json:"limit,omitempty"`
}

// --- Tag Index ---

// TagIndex provides tag-based photo lookup
type TagIndex struct {
	tags map[string]map[string]bool // tag -> photo IDs
	mu   sync.RWMutex
}

// NewTagIndex creates a new tag index
func NewTagIndex() *TagIndex {
	return &TagIndex{
		tags: make(map[string]map[string]bool),
	}
}

// Add adds tags for a photo
func (ti *TagIndex) Add(photoID string, tags []string) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	for _, tag := range tags {
		if ti.tags[tag] == nil {
			ti.tags[tag] = make(map[string]bool)
		}
		ti.tags[tag][photoID] = true
	}
}

// Remove removes a photo from the index
func (ti *TagIndex) Remove(photoID string) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	for tag := range ti.tags {
		delete(ti.tags[tag], photoID)
	}
}

// Search finds photos matching tags
func (ti *TagIndex) Search(tags []string, matchAll bool) []string {
	ti.mu.RLock()
	defer ti.mu.RUnlock()

	if len(tags) == 0 {
		return []string{}
	}

	if matchAll {
		// Intersection of all tag sets
		result := ti.copySet(ti.tags[tags[0]])
		for _, tag := range tags[1:] {
			result = ti.intersect(result, ti.tags[tag])
		}
		return ti.setToSlice(result)
	}

	// Union of all tag sets
	result := make(map[string]bool)
	for _, tag := range tags {
		for photoID := range ti.tags[tag] {
			result[photoID] = true
		}
	}
	return ti.setToSlice(result)
}

func (ti *TagIndex) copySet(s map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for k, v := range s {
		result[k] = v
	}
	return result
}

func (ti *TagIndex) intersect(a, b map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for k := range a {
		if b[k] {
			result[k] = true
		}
	}
	return result
}

func (ti *TagIndex) setToSlice(s map[string]bool) []string {
	result := make([]string, 0, len(s))
	for k := range s {
		result = append(result, k)
	}
	return result
}
