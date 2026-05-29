// Package smartfileorganizer implements AI-powered file organization.
// Inspired by Synology Drive and fnOS AI features, provides intelligent
// file categorization, duplicate detection, naming suggestions, and
// automated organization rules.
package smartfileorganizer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// FileCategory represents a file category.
type FileCategory string

const (
	CategoryDocument FileCategory = "document"
	CategoryImage    FileCategory = "image"
	CategoryVideo    FileCategory = "video"
	CategoryAudio    FileCategory = "audio"
	CategoryArchive  FileCategory = "archive"
	CategoryCode     FileCategory = "code"
	CategoryExec     FileCategory = "executable"
	CategoryData     FileCategory = "data"
	CategoryOther    FileCategory = "other"
)

// SortMode defines how files are sorted.
type SortMode int

const (
	SortByName SortMode = iota
	SortByDate
	SortBySize
	SortByType
	SortByFrequency
)

// FileEntry represents a managed file with metadata.
type FileEntry struct {
	Path       string       `json:"path"`
	Name       string       `json:"name"`
	Size       int64        `json:"size"`
	Category   FileCategory `json:"category"`
	Extension  string       `json:"extension"`
	SHA256     string       `json:"sha256,omitempty"`
	ModTime    time.Time    `json:"modTime"`
	AccessTime time.Time    `json:"accessTime,omitempty"`
	Duplicates []string     `json:"duplicates,omitempty"`
	Tags       []string     `json:"tags,omitempty"`
	Suggested  string       `json:"suggestedName,omitempty"`
}

// OrganizationRule defines an automatic organization rule.
type OrganizationRule struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Enabled     bool         `json:"enabled"`
	Category    FileCategory `json:"category,omitempty"`
	Extensions  []string     `json:"extensions,omitempty"`
	MinSize     int64        `json:"minSize,omitempty"`
	MaxSize     int64        `json:"maxSize,omitempty"`
	TargetDir   string       `json:"targetDir"`
	Dedup       bool         `json:"dedup,omitempty"`
	AutoTag     bool         `json:"autoTag,omitempty"`
	RenamePattern string     `json:"renamePattern,omitempty"`
}

// DuplicateGroup groups duplicate files.
type DuplicateGroup struct {
	Hash  string   `json:"hash"`
	Size  int64    `json:"size"`
	Files []string `json:"files"`
	Count int      `json:"count"`
}

// OrganizationReport summarizes an organization run.
type OrganizationReport struct {
	StartTime       time.Time         `json:"startTime"`
	EndTime         time.Time         `json:"endTime"`
	ScannedFiles    int               `json:"scannedFiles"`
	MovedFiles      int               `json:"movedFiles"`
	RenamedFiles    int               `json:"renamedFiles"`
	DeletedDupes    int               `json:"deletedDupes"`
	SpaceFreedBytes int64             `json:"spaceFreedBytes"`
	CategoryCounts  map[FileCategory]int `json:"categoryCounts"`
	Errors          []string          `json:"errors,omitempty"`
}

// Organizer manages intelligent file organization.
type Organizer struct {
	baseDir string
	rules   []OrganizationRule
	index   map[string]*FileEntry // path -> entry
	catIdx  map[FileCategory][]string
	dupIdx  map[string][]string // hash -> paths
	mu      sync.RWMutex
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewOrganizer creates a new file organizer.
func NewOrganizer(baseDir string) *Organizer {
	ctx, cancel := context.WithCancel(context.Background())
	return &Organizer{
		baseDir: baseDir,
		rules:   make([]OrganizationRule, 0),
		index:   make(map[string]*FileEntry),
		catIdx:  make(map[FileCategory][]string),
		dupIdx:  make(map[string][]string),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// AddRule adds an organization rule.
func (o *Organizer) AddRule(rule OrganizationRule) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.rules = append(o.rules, rule)
}

// RemoveRule removes an organization rule by ID.
func (o *Organizer) RemoveRule(id string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for i, r := range o.rules {
		if r.ID == id {
			o.rules = append(o.rules[:i], o.rules[i+1:]...)
			return true
		}
	}
	return false
}

// GetRules returns all organization rules.
func (o *Organizer) GetRules() []OrganizationRule {
	o.mu.RLock()
	defer o.mu.RUnlock()
	rules := make([]OrganizationRule, len(o.rules))
	copy(rules, o.rules)
	return rules
}

// Scan scans the base directory and builds the file index.
func (o *Organizer) Scan() (*OrganizationReport, error) {
	report := &OrganizationReport{
		StartTime:      time.Now(),
		CategoryCounts: make(map[FileCategory]int),
	}

	err := filepath.Walk(o.baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", path, err))
			return nil
		}
		if info.IsDir() {
			return nil
		}

		select {
		case <-o.ctx.Done():
			return fmt.Errorf("scan cancelled")
		default:
		}

		entry := o.analyzeFile(path, info)

		o.mu.Lock()
		o.index[path] = entry
		o.catIdx[entry.Category] = append(o.catIdx[entry.Category], path)
		o.mu.Unlock()

		report.ScannedFiles++
		report.CategoryCounts[entry.Category]++
		return nil
	})

	report.EndTime = time.Now()
	return report, err
}

// FindDuplicates finds duplicate files by SHA-256 hash.
func (o *Organizer) FindDuplicates() []DuplicateGroup {
	o.mu.RLock()
	defer o.mu.RUnlock()

	// Clear duplicate index
	o.dupIdx = make(map[string][]string)

	// Hash all files
	for path, entry := range o.index {
		if entry.SHA256 == "" {
			hash, err := hashFile(path)
			if err != nil {
				continue
			}
			entry.SHA256 = hash
		}
		o.dupIdx[entry.SHA256] = append(o.dupIdx[entry.SHA256], path)
	}

	// Filter groups with > 1 file
	groups := make([]DuplicateGroup, 0)
	for hash, paths := range o.dupIdx {
		if len(paths) > 1 {
			var size int64
			if entry, ok := o.index[paths[0]]; ok {
				size = entry.Size
			}
			groups = append(groups, DuplicateGroup{
				Hash:  hash,
				Size:  size,
				Files: paths,
				Count: len(paths),
			})
		}
	}

	// Sort by wasted space (descending)
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Size*int64(groups[i].Count-1) > groups[j].Size*int64(groups[j].Count-1)
	})

	return groups
}

// Organize applies rules and organizes files.
func (o *Organizer) Organize(dryRun bool) (*OrganizationReport, error) {
	report := &OrganizationReport{
		StartTime:      time.Now(),
		CategoryCounts: make(map[FileCategory]int),
	}

	o.mu.RLock()
	rules := make([]OrganizationRule, len(o.rules))
	copy(rules, o.rules)
	o.mu.RUnlock()

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		o.mu.RLock()
		paths := o.getMatchingFiles(rule)
		o.mu.RUnlock()

		for _, path := range paths {
			entry, ok := o.index[path]
			if !ok {
				continue
			}

			if rule.Dedup && entry.SHA256 != "" {
				dups := o.dupIdx[entry.SHA256]
				if len(dups) > 1 && dups[0] == path {
					// Keep first, delete rest
					for _, dup := range dups[1:] {
						if !dryRun {
							if err := os.Remove(dup); err == nil {
								report.DeletedDupes++
								report.SpaceFreedBytes += entry.Size
							}
						} else {
							report.DeletedDupes++
							report.SpaceFreedBytes += entry.Size
						}
					}
				}
			}

			// Move to target directory
			if rule.TargetDir != "" {
				newPath := filepath.Join(rule.TargetDir, entry.Name)
				if !dryRun {
					if err := os.Rename(path, newPath); err == nil {
						report.MovedFiles++
						delete(o.index, path)
						o.index[newPath] = entry
						entry.Path = newPath
					} else {
						report.Errors = append(report.Errors, fmt.Sprintf("move %s: %v", path, err))
					}
				} else {
					report.MovedFiles++
				}
			}

			report.CategoryCounts[entry.Category]++
		}
	}

	report.EndTime = time.Now()
	report.ScannedFiles = len(o.index)
	return report, nil
}

// GetByCategory returns all files in a category.
func (o *Organizer) GetByCategory(cat FileCategory) []*FileEntry {
	o.mu.RLock()
	defer o.mu.RUnlock()

	entries := make([]*FileEntry, 0)
	for _, path := range o.catIdx[cat] {
		if entry, ok := o.index[path]; ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetStats returns organization statistics.
func (o *Organizer) GetStats() map[string]interface{} {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var totalSize int64
	for _, entry := range o.index {
		totalSize += entry.Size
	}

	return map[string]interface{}{
		"totalFiles":     len(o.index),
		"totalSize":      totalSize,
		"categories":     len(o.catIdx),
		"rules":          len(o.rules),
		"duplicateGroups": len(o.dupIdx),
		"baseDir":        o.baseDir,
	}
}

// Close stops the organizer.
func (o *Organizer) Close() error {
	o.cancel()
	return nil
}

// analyzeFile analyzes a file and creates a FileEntry.
func (o *Organizer) analyzeFile(path string, info os.FileInfo) *FileEntry {
	ext := strings.ToLower(filepath.Ext(info.Name()))
	category := categorizeByExt(ext)

	return &FileEntry{
		Path:      path,
		Name:      info.Name(),
		Size:      info.Size(),
		Category:  category,
		Extension: ext,
		ModTime:   info.ModTime(),
		Tags:      autoTag(category, ext),
		Suggested: suggestName(info.Name(), category),
	}
}

// getMatchingFiles returns files matching a rule.
func (o *Organizer) getMatchingFiles(rule OrganizationRule) []string {
	var matched []string
	for path, entry := range o.index {
		if rule.Category != "" && entry.Category != rule.Category {
			continue
		}
		if len(rule.Extensions) > 0 {
			found := false
			for _, ext := range rule.Extensions {
				if strings.EqualFold(entry.Extension, ext) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if rule.MinSize > 0 && entry.Size < rule.MinSize {
			continue
		}
		if rule.MaxSize > 0 && entry.Size > rule.MaxSize {
			continue
		}
		matched = append(matched, path)
	}
	return matched
}

// categorizeByExt categorizes a file by its extension.
func categorizeByExt(ext string) FileCategory {
	switch ext {
	case ".doc", ".docx", ".pdf", ".txt", ".rtf", ".odt", ".xls", ".xlsx", ".ppt", ".pptx", ".csv":
		return CategoryDocument
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".svg", ".webp", ".ico", ".tiff":
		return CategoryImage
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v":
		return CategoryVideo
	case ".mp3", ".wav", ".flac", ".aac", ".ogg", ".wma", ".m4a":
		return CategoryAudio
	case ".zip", ".rar", ".7z", ".tar", ".gz", ".bz2", ".xz":
		return CategoryArchive
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb", ".php", ".sh", ".html", ".css":
		return CategoryCode
	case ".exe", ".msi", ".deb", ".rpm", ".dmg", ".app":
		return CategoryExec
	case ".json", ".xml", ".yaml", ".yml", ".toml", ".sql", ".db":
		return CategoryData
	default:
		return CategoryOther
	}
}

// autoTag generates tags for a file based on its category and extension.
func autoTag(cat FileCategory, ext string) []string {
	tags := []string{string(cat)}
	switch cat {
	case CategoryDocument:
		tags = append(tags, "office")
		if ext == ".pdf" {
			tags = append(tags, "pdf")
		}
	case CategoryImage:
		tags = append(tags, "media", "photo")
	case CategoryVideo:
		tags = append(tags, "media", "video")
	case CategoryAudio:
		tags = append(tags, "media", "music")
	}
	return tags
}

// suggestName suggests a cleaner file name.
func suggestName(name string, cat FileCategory) string {
	// Remove common noise patterns
	suggested := name
	suggested = strings.ReplaceAll(suggested, "_", " ")
	suggested = strings.ReplaceAll(suggested, "-", " ")
	// Capitalize first letter
	if len(suggested) > 0 {
		suggested = strings.ToUpper(suggested[:1]) + suggested[1:]
	}
	return suggested
}

// hashFile computes SHA-256 of a file.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
