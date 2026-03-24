// Package tiering provides hot/cold data tiering for NAS storage optimization.
// This file implements file system scanning for tiering analysis.
package tiering

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Scanner scans file systems for tiering analysis
type Scanner struct {
	mu       sync.RWMutex
	workers  int
	progress ScanProgress
}

// ScanProgress tracks scanning progress
type ScanProgress struct {
	TotalFiles    int64     `json:"total_files"`
	TotalSize     int64     `json:"total_size"`
	ScannedFiles  int64     `json:"scanned_files"`
	ScannedSize   int64     `json:"scanned_size"`
	CurrentPath   string    `json:"current_path"`
	StartTime     time.Time `json:"start_time"`
	EstimatedTime string    `json:"estimated_time,omitempty"`
	Running       bool      `json:"running"`
}

// ScanResult contains scan results
type ScanResult struct {
	Files      []FileInfo       `json:"files"`
	ByTier     map[Tier]int64   `json:"by_tier"`
	SizeByTier map[Tier]int64   `json:"size_by_tier"`
	Progress   ScanProgress     `json:"progress"`
	Duration   time.Duration    `json:"duration"`
	Errors     []ScanError      `json:"errors,omitempty"`
}

// ScanError represents a scanning error
type ScanError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// ScanOptions configures scanning behavior
type ScanOptions struct {
	RootPath       string
	MaxDepth       int
	MinSize        int64
	MaxSize        int64
	FollowSymlinks bool
	ExcludeHidden  bool
	ExcludePaths   []string
	IncludeExts    []string
	ExcludeExts    []string
	Concurrency    int
}

// NewScanner creates a new scanner
func NewScanner(workers int) *Scanner {
	if workers <= 0 {
		workers = 4
	}
	return &Scanner{
		workers: workers,
	}
}

// Scan scans a path for file information
func (s *Scanner) Scan(ctx context.Context, opts ScanOptions) (*ScanResult, error) {
	if opts.RootPath == "" {
		opts.RootPath = "/"
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = s.workers
	}

	result := &ScanResult{
		Files:      make([]FileInfo, 0),
		ByTier:     make(map[Tier]int64),
		SizeByTier: make(map[Tier]int64),
		Errors:     make([]ScanError, 0),
	}

	s.mu.Lock()
	s.progress = ScanProgress{
		StartTime: time.Now(),
		Running:   true,
	}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.progress.Running = false
		s.mu.Unlock()
	}()

	// Channel for files to process
	fileChan := make(chan string, 1000)
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Start worker goroutines
	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range fileChan {
				select {
				case <-ctx.Done():
					return
				default:
					info, err := s.scanFile(path, opts)
					if err != nil {
						mu.Lock()
						result.Errors = append(result.Errors, ScanError{
							Path:    path,
							Message: err.Error(),
						})
						mu.Unlock()
						continue
					}
					if info != nil {
						mu.Lock()
						result.Files = append(result.Files, *info)
						result.ByTier[info.CurrentTier]++
						result.SizeByTier[info.CurrentTier] += info.Size
						s.progress.ScannedFiles++
						s.progress.ScannedSize += info.Size
						mu.Unlock()
					}
				}
			}
		}()
	}

	// Walk the file system
	err := filepath.Walk(opts.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check exclusions
		if s.shouldSkip(path, info, opts) {
			return nil
		}

		s.mu.Lock()
		s.progress.TotalFiles++
		s.progress.TotalSize += info.Size()
		s.progress.CurrentPath = path
		s.mu.Unlock()

		fileChan <- path
		return nil
	})

	close(fileChan)
	wg.Wait()

	if err != nil && err != context.Canceled {
		return result, err
	}

	result.Progress = s.progress
	result.Duration = time.Since(result.Progress.StartTime)

	return result, nil
}

// scanFile scans a single file
func (s *Scanner) scanFile(path string, opts ScanOptions) (*FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// Check size constraints
	if opts.MinSize > 0 && info.Size() < opts.MinSize {
		return nil, nil
	}
	if opts.MaxSize > 0 && info.Size() > opts.MaxSize {
		return nil, nil
	}

	// Get access time (platform-specific)
	atime := s.getAccessTime(info)

	// Determine initial tier based on access patterns
	tier := s.determineInitialTier(info, atime)

	return &FileInfo{
		Path:        path,
		Size:        info.Size(),
		CurrentTier: tier,
		LastAccess:  atime,
		LastModified: info.ModTime(),
		CreatedAt:   info.ModTime(), // Approximation
		AccessCount: 0,               // Would need extended attributes or tracking
	}, nil
}

// shouldSkip determines if a path should be skipped
func (s *Scanner) shouldSkip(path string, info os.FileInfo, opts ScanOptions) bool {
	// Skip hidden files
	if opts.ExcludeHidden {
		name := filepath.Base(path)
		if len(name) > 0 && name[0] == '.' {
			return true
		}
	}

	// Check excluded paths
	for _, exclude := range opts.ExcludePaths {
		if matched, _ := filepath.Match(exclude, path); matched {
			return true
		}
	}

	// Check extension filters
	ext := filepath.Ext(path)
	if len(opts.IncludeExts) > 0 {
		found := false
		for _, include := range opts.IncludeExts {
			if ext == include {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}

	for _, exclude := range opts.ExcludeExts {
		if ext == exclude {
			return true
		}
	}

	return false
}

// determineInitialTier determines the initial tier for a file
func (s *Scanner) determineInitialTier(info os.FileInfo, atime time.Time) Tier {
	// New files start in warm tier
	if time.Since(info.ModTime()) < 24*time.Hour {
		return TierWarm
	}

	// Recently accessed files are warm
	if time.Since(atime) < 7*24*time.Hour {
		return TierWarm
	}

	// Files not accessed in 30+ days are cold
	if time.Since(atime) > 30*24*time.Hour {
		return TierCold
	}

	return TierWarm
}

// getAccessTime extracts access time from file info
func (s *Scanner) getAccessTime(info os.FileInfo) time.Time {
	// Use ModTime as fallback; on Linux, we can use Sys().(*syscall.Stat_t)
	return info.ModTime()
}

// GetProgress returns current scan progress
func (s *Scanner) GetProgress() ScanProgress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.progress
}

// Stop stops the current scan
func (s *Scanner) Stop() {
	s.mu.Lock()
	s.progress.Running = false
	s.mu.Unlock()
}