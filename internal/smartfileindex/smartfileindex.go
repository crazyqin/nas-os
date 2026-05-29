package smartfileindex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// IndexEntry represents a file index entry
type IndexEntry struct {
	Path        string    `json:"path"`
	Name        string    `json:"name"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
	ContentType string    `json:"content_type"`
	Tags        []string  `json:"tags"`
	Thumbnail   string    `json:"thumbnail,omitempty"`
}

// SearchQuery represents a search query
type SearchQuery struct {
	Keyword     string    `json:"keyword"`
	ContentType string    `json:"content_type,omitempty"`
	MinSize     int64     `json:"min_size,omitempty"`
	MaxSize     int64     `json:"max_size,omitempty"`
	After       time.Time `json:"after,omitempty"`
	Before      time.Time `json:"before,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	Limit       int       `json:"limit,omitempty"`
}

// SmartFileIndex provides intelligent file indexing and full-text search
// Inspired by TrueNAS SMB Spotlight
type SmartFileIndex struct {
	mu          sync.RWMutex
	entries     map[string]*IndexEntry
	indexPaths  []string
	maxEntries  int
	running     bool
	stopCh      chan struct{}
}

// NewSmartFileIndex creates a new SmartFileIndex instance
func NewSmartFileIndex(indexPaths []string, maxEntries int) *SmartFileIndex {
	return &SmartFileIndex{
		entries:    make(map[string]*IndexEntry),
		indexPaths: indexPaths,
		maxEntries: maxEntries,
		stopCh:     make(chan struct{}),
	}
}

// Start begins the indexing process
func (sfi *SmartFileIndex) Start(ctx context.Context) error {
	sfi.mu.Lock()
	if sfi.running {
		sfi.mu.Unlock()
		return fmt.Errorf("index already running")
	}
	sfi.running = true
	sfi.mu.Unlock()

	go sfi.indexLoop(ctx)
	return nil
}

// Stop stops the indexing process
func (sfi *SmartFileIndex) Stop() {
	sfi.mu.Lock()
	defer sfi.mu.Unlock()
	if sfi.running {
		close(sfi.stopCh)
		sfi.running = false
	}
}

// Search performs a search query
func (sfi *SmartFileIndex) Search(query SearchQuery) ([]*IndexEntry, error) {
	sfi.mu.RLock()
	defer sfi.mu.RUnlock()

	var results []*IndexEntry
	keyword := strings.ToLower(query.Keyword)

	for _, entry := range sfi.entries {
		if !sfi.matchesQuery(entry, query, keyword) {
			continue
		}
		results = append(results, entry)
		if query.Limit > 0 && len(results) >= query.Limit {
			break
		}
	}

	return results, nil
}

// GetStats returns indexing statistics
func (sfi *SmartFileIndex) GetStats() map[string]interface{} {
	sfi.mu.RLock()
	defer sfi.mu.RUnlock()

	return map[string]interface{}{
		"total_entries": len(sfi.entries),
		"max_entries":   sfi.maxEntries,
		"running":       sfi.running,
		"index_paths":   sfi.indexPaths,
	}
}

func (sfi *SmartFileIndex) indexLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	// Initial index
	sfi.buildIndex(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-sfi.stopCh:
			return
		case <-ticker.C:
			sfi.buildIndex(ctx)
		}
	}
}

func (sfi *SmartFileIndex) buildIndex(ctx context.Context) {
	for _, root := range sfi.indexPaths {
		filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if info.IsDir() {
				return nil
			}

			entry := &IndexEntry{
				Path:    path,
				Name:    info.Name(),
				Size:    info.Size(),
				ModTime: info.ModTime(),
			}

			sfi.mu.Lock()
			if len(sfi.entries) < sfi.maxEntries {
				sfi.entries[path] = entry
			}
			sfi.mu.Unlock()

			return nil
		})
	}
}

func (sfi *SmartFileIndex) matchesQuery(entry *IndexEntry, query SearchQuery, keyword string) bool {
	if keyword != "" && !strings.Contains(strings.ToLower(entry.Name), keyword) {
		return false
	}
	if query.ContentType != "" && entry.ContentType != query.ContentType {
		return false
	}
	if query.MinSize > 0 && entry.Size < query.MinSize {
		return false
	}
	if query.MaxSize > 0 && entry.Size > query.MaxSize {
		return false
	}
	if !query.After.IsZero() && entry.ModTime.Before(query.After) {
		return false
	}
	if !query.Before.IsZero() && entry.ModTime.After(query.Before) {
		return false
	}
	return true
}
