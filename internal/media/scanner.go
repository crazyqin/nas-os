package media

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Pre-compiled regex patterns for TV episode parsing
var (
	// S01E01, s01e01, S1E1, s1e1 patterns
	seasonEpisodeRegex = regexp.MustCompile(`(?i)[sS](\d{1,2})[eE](\d{1,3})`)
	// 1x01, 1x1 patterns
	altSeasonEpisodeRegex = regexp.MustCompile(`(?i)(\d{1,2})[xX](\d{1,3})`)
	// Episode 01, Ep.01, E01 patterns (without season)
	episodeOnlyRegex = regexp.MustCompile(`(?i)(?:ep(?:isode)?[.\s]*|e)(\d{1,3})(?:[^0-9]|$)`)
)

// Scanner scans directories for media files
type Scanner struct {
	extensions map[string]bool
	cache      *Cache
}

// NewScanner creates a new media scanner
func NewScanner(cache *Cache) *Scanner {
	return &Scanner{
		extensions: SupportedExtensions,
		cache:      cache,
	}
}

// ScanDirectory scans a directory for video files
func (s *Scanner) ScanDirectory(rootPath string) (*ScanResult, error) {
	start := time.Now()
	result := &ScanResult{
		LibraryID: generateLibraryID(rootPath),
	}

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, ScanError{
				Path:    path,
				Message: err.Error(),
			})
			return nil
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !s.extensions[ext] {
			return nil
		}

		result.TotalFiles++

		// Check if file is already cached
		if s.cache != nil {
			if _, exists := s.cache.GetFile(path); exists {
				return nil
			}
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			result.Errors = append(result.Errors, ScanError{
				Path:    path,
				Message: fmt.Sprintf("failed to get file info: %v", err),
			})
			return nil
		}

		// Create video file record
		videoFile := &VideoFile{
			ID:         uuid.New().String(),
			Path:       path,
			Filename:   d.Name(),
			Size:       info.Size(),
			CreatedAt:  time.Now(),
			ModifiedAt: info.ModTime(),
		}

		// Cache the file
		if s.cache != nil {
			s.cache.SetFile(path, videoFile)
		}

		result.NewFiles++
		return nil
	})

	result.Duration = time.Since(start)
	return result, err
}

// ScanLibrary scans a media library
func (s *Scanner) ScanLibrary(library *MediaLibrary) (*ScanResult, error) {
	return s.ScanDirectory(library.Path)
}

// DetectMediaType tries to detect if the content is a movie or TV show
func (s *Scanner) DetectMediaType(filename string) MediaType {
	filename = strings.ToLower(filename)

	// TV show patterns
	tvPatterns := []string{
		"s01e", "s02e", "s03e", "s04e", "s05e",
		"s1e", "s2e", "s3e", "s4e", "s5e",
		"season", "episode", "ep.", "e0", "e1",
		"720p", "1080p", "2160p", "hdtv", "web-dl",
	}

	for _, pattern := range tvPatterns {
		if strings.Contains(filename, pattern) {
			return MediaTypeTVShow
		}
	}

	// Check for year pattern (common in movies)
	yearPattern := false
	for year := 1920; year <= time.Now().Year()+1; year++ {
		if strings.Contains(filename, fmt.Sprintf("(%d)", year)) ||
			strings.Contains(filename, fmt.Sprintf(".%d.", year)) {
			yearPattern = true
			break
		}
	}

	if yearPattern {
		return MediaTypeMovie
	}

	return MediaTypeUnknown
}

// ParseFilename extracts potential title and year from filename
func (s *Scanner) ParseFilename(filename string) (title string, year int, season int, episode int) {
	// Remove extension
	name := strings.TrimSuffix(filename, filepath.Ext(filename))

	// Try to extract TV show info (S01E01 pattern)
	if s, e, ok := parseTVEpisode(name); ok {
		season = s
		episode = e
		// Remove episode info to get title
		name = removeTVEpisodeInfo(name)
	}

	// Try to extract year
	for year := 1920; year <= time.Now().Year()+1; year++ {
		patterns := []string{
			fmt.Sprintf("(%d)", year),
			fmt.Sprintf(".%d.", year),
			fmt.Sprintf("_%d_", year),
			fmt.Sprintf(" %d ", year),
		}
		for _, p := range patterns {
			if idx := strings.Index(strings.ToLower(name), strings.ToLower(p)); idx > 0 {
				title = strings.TrimSpace(name[:idx])
				title = cleanTitle(title)
				return title, year, season, episode
			}
		}
	}

	// No year found, clean and return
	title = cleanTitle(name)
	return title, 0, season, episode
}

// parseTVEpisode extracts season and episode numbers using regex
func parseTVEpisode(filename string) (season, episode int, ok bool) {
	// Try S01E01 pattern first (most common)
	if matches := seasonEpisodeRegex.FindStringSubmatch(filename); len(matches) == 3 {
		season = parseInt(matches[1])
		episode = parseInt(matches[2])
		if season > 0 && episode > 0 {
			return season, episode, true
		}
	}

	// Try 1x01 pattern
	if matches := altSeasonEpisodeRegex.FindStringSubmatch(filename); len(matches) == 3 {
		season = parseInt(matches[1])
		episode = parseInt(matches[2])
		if season > 0 && episode > 0 {
			return season, episode, true
		}
	}

	// Try episode-only pattern (Ep.01, E01, Episode 01)
	// Note: This assumes season 1 if no season is found
	if matches := episodeOnlyRegex.FindStringSubmatch(filename); len(matches) == 2 {
		episode = parseInt(matches[1])
		if episode > 0 {
			return 1, episode, true // Default to season 1
		}
	}

	return 0, 0, false
}

// parseInt safely parses a string to int, returning 0 on error
func parseInt(s string) int {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result
}

// removeTVEpisodeInfo removes TV episode info from filename
func removeTVEpisodeInfo(filename string) string {
	// Remove S01E01 patterns
	result := seasonEpisodeRegex.ReplaceAllString(filename, "")
	// Remove 1x01 patterns
	result = altSeasonEpisodeRegex.ReplaceAllString(result, "")
	// Remove episode-only patterns
	result = episodeOnlyRegex.ReplaceAllString(result, "")
	// Clean up multiple spaces and trim
	result = strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(result, " "))
	return result
}

// cleanTitle cleans up a title string
func cleanTitle(title string) string {
	// Common replacements
	replacements := map[string]string{
		".":  " ",
		"_":  " ",
		"-":  " ",
		"  ": " ",
	}

	result := title
	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	return strings.TrimSpace(result)
}

func generateLibraryID(path string) string {
	return uuid.NewSHA1(uuid.Nil, []byte(path)).String()[:8]
}

// GetVideoFiles returns all cached video files
func (s *Scanner) GetVideoFiles() []*VideoFile {
	if s.cache == nil {
		return nil
	}
	return s.cache.GetAllFiles()
}

// GetVideoFile returns a specific video file by path
func (s *Scanner) GetVideoFile(path string) (*VideoFile, bool) {
	if s.cache == nil {
		return nil, false
	}
	return s.cache.GetFile(path)
}

// RemoveMissingFiles removes files that no longer exist on disk
func (s *Scanner) RemoveMissingFiles(libraryPath string) (int, error) {
	if s.cache == nil {
		return 0, nil
	}

	files := s.cache.GetAllFiles()
	removed := 0

	for _, file := range files {
		if !strings.HasPrefix(file.Path, libraryPath) {
			continue
		}

		if _, err := os.Stat(file.Path); os.IsNotExist(err) {
			s.cache.RemoveFile(file.Path)
			removed++
		}
	}

	return removed, nil
}
