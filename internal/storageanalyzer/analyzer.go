package storageanalyzer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

// NewAnalyzer creates a new storage analyzer instance
func NewAnalyzer(config AnalysisConfig) *Analyzer {
	return &Analyzer{
		config:  config,
		reports: make(map[string]*StorageReport),
		jobs:    make(map[string]*AnalysisJob),
		history: make([]Snapshot, 0),
		stopCh:  make(chan struct{}),
	}
}

// Start starts the analyzer background tasks
func (a *Analyzer) Start(ctx context.Context) {
	log.Println("Storage analyzer started")
	<-ctx.Done()
	close(a.stopCh)
}

// RunAnalysis runs a complete storage analysis
func (a *Analyzer) RunAnalysis(ctx context.Context) (*StorageReport, error) {
	jobID := fmt.Sprintf("analysis-%d", time.Now().UnixNano())
	job := &AnalysisJob{
		ID:        jobID,
		Status:    "running",
		StartedAt: time.Now(),
		Progress:  0,
	}

	a.mu.Lock()
	a.jobs[jobID] = job
	a.mu.Unlock()

	report := &StorageReport{
		ID:          jobID,
		GeneratedAt: time.Now(),
	}

	// Step 1: Scan directories
	job.Progress = 0.1
	dirUsage, err := a.scanDirectories(ctx)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return nil, err
	}
	report.ByDirectory = dirUsage

	// Step 2: Scan file types
	job.Progress = 0.2
	fileTypes, totalSize, fileCount, err := a.scanFileTypes(ctx)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return nil, err
	}
	report.ByFileType = fileTypes
	report.UsedSpace = totalSize

	// Step 3: Get filesystem stats
	job.Progress = 0.3
	totalSpace, freeSpace, err := a.getFSStats()
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return nil, err
	}
	report.TotalSpace = totalSpace
	report.FreeSpace = freeSpace
	report.UsagePercent = float64(totalSize) / float64(totalSpace) * 100

	// Step 4: Scan users
	job.Progress = 0.4
	userUsage, err := a.scanUsers(ctx)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return nil, err
	}
	report.ByUser = userUsage

	// Step 5: Scan by time
	job.Progress = 0.5
	timeUsage, err := a.scanByTime(ctx)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return nil, err
	}
	report.ByTime = timeUsage

	// Step 6: Find duplicates
	job.Progress = 0.6
	duplicates, err := a.findDuplicates(ctx)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return nil, err
	}
	report.Duplicates = duplicates

	// Step 7: Find big files
	job.Progress = 0.7
	bigFiles, err := a.findBigFiles(ctx)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return nil, err
	}
	report.BigFiles = bigFiles

	// Step 8: Analyze growth trend
	job.Progress = 0.8
	growth, err := a.analyzeGrowth(totalSize)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return nil, err
	}
	report.Growth = growth

	// Step 9: Generate suggestions
	job.Progress = 0.9
	suggestions := a.generateSuggestions(report)
	report.Suggestions = suggestions

	// Step 10: Generate heatmap
	heatmap, err := a.generateHeatmap(ctx)
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return nil, err
	}
	report.Heatmap = heatmap

	// Step 11: Analyze snapshots
	snapshotUsage, err := a.analyzeSnapshots()
	if err != nil {
		job.Status = "failed"
		job.Error = err.Error()
		return nil, err
	}
	report.Snapshots = snapshotUsage

	// Record snapshot for history
	a.recordSnapshot(totalSize, freeSpace, fileCount)

	job.Progress = 1.0
	job.Status = "completed"
	now := time.Now()
	job.EndedAt = &now
	job.Report = report

	a.mu.Lock()
	a.reports[jobID] = report
	a.mu.Unlock()

	return report, nil
}

// scanDirectories scans storage usage by directory
func (a *Analyzer) scanDirectories(ctx context.Context) ([]DirUsage, error) {
	usageMap := make(map[string]*DirUsage)

	for _, root := range a.config.ScanPaths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if a.isExcluded(path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !info.IsDir() {
				dir := filepath.Dir(path)
				if u, ok := usageMap[dir]; ok {
					u.Size += info.Size()
					u.FileCount++
				} else {
					usageMap[dir] = &DirUsage{
						Path:       dir,
						Size:       info.Size(),
						FileCount:  1,
						LastAccess: info.ModTime(),
					}
				}
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Calculate percentages
	var totalSize int64
	for _, u := range usageMap {
		totalSize += u.Size
	}

	result := make([]DirUsage, 0, len(usageMap))
	for _, u := range usageMap {
		if totalSize > 0 {
			u.Percent = float64(u.Size) / float64(totalSize) * 100
		}
		result = append(result, *u)
	}

	// Sort by size descending
	sort.Slice(result, func(i, j int) bool {
		return result[i].Size > result[j].Size
	})

	// Return top 50
	if len(result) > 50 {
		result = result[:50]
	}

	return result, nil
}

// scanFileTypes scans storage usage by file type
func (a *Analyzer) scanFileTypes(ctx context.Context) ([]FileTypeUsage, int64, int, error) {
	typeMap := make(map[string]*FileTypeUsage)
	var totalSize int64
	var fileCount int

	for _, root := range a.config.ScanPaths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if a.isExcluded(path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !info.IsDir() {
				ext := strings.ToLower(filepath.Ext(path))
				if ext == "" {
					ext = "(no extension)"
				}

				category := "other"
				if cat, ok := FileTypeCategory[ext]; ok {
					category = cat
				}

				if u, ok := typeMap[ext]; ok {
					u.Size += info.Size()
					u.Count++
				} else {
					typeMap[ext] = &FileTypeUsage{
						Extension: ext,
						Size:      info.Size(),
						Count:     1,
						Category:  category,
					}
				}

				totalSize += info.Size()
				fileCount++
			}

			return nil
		})
		if err != nil {
			return nil, 0, 0, err
		}
	}

	result := make([]FileTypeUsage, 0, len(typeMap))
	for _, u := range typeMap {
		if totalSize > 0 {
			u.Percent = float64(u.Size) / float64(totalSize) * 100
		}
		result = append(result, *u)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Size > result[j].Size
	})

	return result, totalSize, fileCount, nil
}

// scanUsers scans storage usage by user
func (a *Analyzer) scanUsers(ctx context.Context) ([]UserUsage, error) {
	userMap := make(map[int]*UserUsage)

	for _, root := range a.config.ScanPaths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if a.isExcluded(path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !info.IsDir() {
				uid := 0 // Default user
				if stat, ok := info.Sys().(*syscall.Stat_t); ok {
					uid = int(stat.Uid)
				}

				if u, ok := userMap[uid]; ok {
					u.Size += info.Size()
					u.FileCount++
				} else {
					userMap[uid] = &UserUsage{
						UserID:    uid,
						Username:  fmt.Sprintf("uid_%d", uid),
						Size:      info.Size(),
						FileCount: 1,
					}
				}
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	var totalSize int64
	for _, u := range userMap {
		totalSize += u.Size
	}

	result := make([]UserUsage, 0, len(userMap))
	for _, u := range userMap {
		if totalSize > 0 {
			u.Percent = float64(u.Size) / float64(totalSize) * 100
		}
		result = append(result, *u)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Size > result[j].Size
	})

	return result, nil
}

// scanByTime scans storage usage by time period
func (a *Analyzer) scanByTime(ctx context.Context) ([]TimeUsage, error) {
	now := time.Now()
	periods := []struct {
		name  string
		start time.Time
	}{
		{"last_24h", now.Add(-24 * time.Hour)},
		{"last_7d", now.Add(-7 * 24 * time.Hour)},
		{"last_30d", now.Add(-30 * 24 * time.Hour)},
		{"last_90d", now.Add(-90 * 24 * time.Hour)},
		{"last_year", now.Add(-365 * 24 * time.Hour)},
		{"older", time.Time{}},
	}

	usageMap := make(map[string]*TimeUsage)
	for _, p := range periods {
		usageMap[p.name] = &TimeUsage{
			Period:    p.name,
			StartTime: p.start,
			EndTime:   now,
		}
	}

	for _, root := range a.config.ScanPaths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if a.isExcluded(path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !info.IsDir() {
				modTime := info.ModTime()
				assigned := false
				for _, p := range periods {
					if modTime.After(p.start) {
						usageMap[p.name].Size += info.Size()
						usageMap[p.name].FileCount++
						assigned = true
						break
					}
				}
				if !assigned {
					usageMap["older"].Size += info.Size()
					usageMap["older"].FileCount++
				}
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	result := make([]TimeUsage, 0, len(usageMap))
	for _, u := range usageMap {
		result = append(result, *u)
	}

	return result, nil
}

// findDuplicates finds duplicate files
func (a *Analyzer) findDuplicates(ctx context.Context) ([]DuplicateGroup, error) {
	sizeMap := make(map[int64][]string)

	// First pass: group by size
	for _, root := range a.config.ScanPaths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if a.isExcluded(path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !info.IsDir() && info.Size() > 0 {
				sizeMap[info.Size()] = append(sizeMap[info.Size()], path)
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Second pass: hash files with same size
	hashMap := make(map[string][]FileInfo)
	for size, paths := range sizeMap {
		if len(paths) < 2 {
			continue
		}

		for _, path := range paths {
			hash, err := a.hashFile(path)
			if err != nil {
				continue
			}

			key := fmt.Sprintf("%s:%d", hash, size)
			info, err := os.Stat(path)
			if err != nil {
				continue
			}

			hashMap[key] = append(hashMap[key], FileInfo{
				Path:      path,
				Name:      info.Name(),
				Size:      info.Size(),
				ModTime:   info.ModTime(),
				Extension: filepath.Ext(path),
			})
		}
	}

	result := make([]DuplicateGroup, 0)
	for key, files := range hashMap {
		if len(files) < 2 {
			continue
		}

		parts := strings.SplitN(key, ":", 2)
		hash := parts[0]
		size := files[0].Size

		result = append(result, DuplicateGroup{
			Hash:      hash,
			Size:      size,
			Count:     len(files),
			Wasted:    int64(len(files)-1) * size,
			Files:     files,
			Algorithm: "sha256",
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Wasted > result[j].Wasted
	})

	if len(result) > 100 {
		result = result[:100]
	}

	return result, nil
}

// findBigFiles finds the largest files
func (a *Analyzer) findBigFiles(ctx context.Context) ([]FileInfo, error) {
	var bigFiles []FileInfo

	for _, root := range a.config.ScanPaths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if a.isExcluded(path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !info.IsDir() && info.Size() > 100*1024*1024 { // > 100MB
				bigFiles = append(bigFiles, FileInfo{
					Path:      path,
					Name:      info.Name(),
					Size:      info.Size(),
					ModTime:   info.ModTime(),
					Extension: filepath.Ext(path),
				})
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	sort.Slice(bigFiles, func(i, j int) bool {
		return bigFiles[i].Size > bigFiles[j].Size
	})

	if len(bigFiles) > 100 {
		bigFiles = bigFiles[:100]
	}

	return bigFiles, nil
}

// analyzeGrowth analyzes storage growth trend
func (a *Analyzer) analyzeGrowth(currentSize int64) (GrowthTrend, error) {
	a.mu.RLock()
	history := make([]Snapshot, len(a.history))
	copy(history, a.history)
	a.mu.RUnlock()

	trend := GrowthTrend{
		History: history,
	}

	if len(history) < 2 {
		trend.Predicted = Prediction{
			Assumptions: "Insufficient data for prediction",
		}
		return trend, nil
	}

	// Calculate growth rates
	var totalGrowth int64
	var days float64
	for i := 1; i < len(history); i++ {
		growth := history[i].UsedSpace - history[i-1].UsedSpace
		duration := history[i].Timestamp.Sub(history[i-1].Timestamp).Hours() / 24
		if duration > 0 {
			totalGrowth += growth
			days += duration
		}
	}

	if days > 0 {
		trend.DailyAvg = int64(float64(totalGrowth) / days)
		trend.WeeklyAvg = trend.DailyAvg * 7
		trend.MonthlyAvg = trend.DailyAvg * 30
	}

	// Predict full date
	if trend.DailyAvg > 0 {
		freeSpace := history[len(history)-1].FreeSpace
		daysRemaining := float64(freeSpace) / float64(trend.DailyAvg)
		trend.Predicted = Prediction{
			FullDate:      time.Now().AddDate(0, 0, int(daysRemaining)),
			DaysRemaining: int(daysRemaining),
			Confidence:    0.7,
			Assumptions:   "Based on average daily growth",
		}
	}

	return trend, nil
}

// generateSuggestions generates cleanup suggestions
func (a *Analyzer) generateSuggestions(report *StorageReport) []CleanSuggestion {
	suggestions := make([]CleanSuggestion, 0)

	// Suggest cleaning old temporary files
	for _, tu := range report.ByTime {
		if tu.Period == "older" && tu.Size > 1024*1024*1024 { // > 1GB
			suggestions = append(suggestions, CleanSuggestion{
				ID:          fmt.Sprintf("old-files-%d", time.Now().UnixNano()),
				Type:        "old_files",
				Path:        "/",
				Size:        tu.Size,
				Reason:      "Files older than 1 year",
				Priority:    2,
				Safe:        false,
				Description: fmt.Sprintf("%.2f GB of files older than 1 year", float64(tu.Size)/1024/1024/1024),
			})
		}
	}

	// Suggest removing duplicates
	for _, dup := range report.Duplicates {
		if dup.Wasted > 100*1024*1024 { // > 100MB
			suggestions = append(suggestions, CleanSuggestion{
				ID:          fmt.Sprintf("duplicates-%s", dup.Hash[:8]),
				Type:        "duplicates",
				Path:        dup.Files[0].Path,
				Size:        dup.Wasted,
				Reason:      fmt.Sprintf("%d duplicate files", dup.Count),
				Priority:    1,
				Safe:        true,
				Description: fmt.Sprintf("%.2f MB wasted by %d duplicates", float64(dup.Wasted)/1024/1024, dup.Count),
			})
		}
	}

	// Suggest cleaning large log files
	for _, ft := range report.ByFileType {
		if ft.Extension == ".log" && ft.Size > 500*1024*1024 { // > 500MB
			suggestions = append(suggestions, CleanSuggestion{
				ID:          fmt.Sprintf("logs-%d", time.Now().UnixNano()),
				Type:        "log_files",
				Path:        "/var/log",
				Size:        ft.Size,
				Reason:      "Large log files",
				Priority:    3,
				Safe:        true,
				Description: fmt.Sprintf("%.2f MB of log files", float64(ft.Size)/1024/1024),
			})
		}
	}

	// Suggest cleaning cache
	for _, ft := range report.ByFileType {
		if ft.Category == "system" && ft.Size > 1024*1024*1024 { // > 1GB
			suggestions = append(suggestions, CleanSuggestion{
				ID:          fmt.Sprintf("cache-%d", time.Now().UnixNano()),
				Type:        "cache_files",
				Path:        "/",
				Size:        ft.Size,
				Reason:      "System cache files",
				Priority:    4,
				Safe:        true,
				Description: fmt.Sprintf("%.2f GB of cache/temp files", float64(ft.Size)/1024/1024/1024),
			})
		}
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Priority < suggestions[j].Priority
	})

	return suggestions
}

// generateHeatmap generates storage access heatmap
func (a *Analyzer) generateHeatmap(ctx context.Context) ([]HeatmapEntry, error) {
	entries := make([]HeatmapEntry, 0)

	for _, root := range a.config.ScanPaths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if a.isExcluded(path) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			if !info.IsDir() {
				hotLevel := 1
				age := time.Since(info.ModTime())
				if age < 24*time.Hour {
					hotLevel = 5
				} else if age < 7*24*time.Hour {
					hotLevel = 4
				} else if age < 30*24*time.Hour {
					hotLevel = 3
				} else if age < 90*24*time.Hour {
					hotLevel = 2
				}

				entries = append(entries, HeatmapEntry{
					Path:       filepath.Dir(path),
					LastAccess: info.ModTime(),
					Size:       info.Size(),
					HotLevel:   hotLevel,
				})
			}

			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	// Aggregate by directory
	dirMap := make(map[string]*HeatmapEntry)
	for _, e := range entries {
		if existing, ok := dirMap[e.Path]; ok {
			existing.Size += e.Size
			existing.AccessCount++
			if e.HotLevel > existing.HotLevel {
				existing.HotLevel = e.HotLevel
			}
			if e.LastAccess.After(existing.LastAccess) {
				existing.LastAccess = e.LastAccess
			}
		} else {
			dirMap[e.Path] = &HeatmapEntry{
				Path:        e.Path,
				AccessCount: 1,
				LastAccess:  e.LastAccess,
				Size:        e.Size,
				HotLevel:    e.HotLevel,
			}
		}
	}

	result := make([]HeatmapEntry, 0, len(dirMap))
	for _, e := range dirMap {
		result = append(result, *e)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].HotLevel > result[j].HotLevel
	})

	if len(result) > 100 {
		result = result[:100]
	}

	return result, nil
}

// analyzeSnapshots analyzes snapshot storage usage
func (a *Analyzer) analyzeSnapshots() (SnapshotUsage, error) {
	usage := SnapshotUsage{
		ByDataset: make([]DatasetSnapshot, 0),
	}

	// Try to read ZFS/Btrfs snapshots
	snapshotDirs := []string{
		"/.zfs/snapshot",
		"/var/lib/docker/btrfs",
	}

	for _, dir := range snapshotDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		var totalSize int64
		var oldest, newest time.Time

		for _, entry := range entries {
			if entry.IsDir() {
				info, err := entry.Info()
				if err != nil {
					continue
				}

				totalSize += info.Size()
				usage.SnapshotCount++

				if oldest.IsZero() || info.ModTime().Before(oldest) {
					oldest = info.ModTime()
				}
				if newest.IsZero() || info.ModTime().After(newest) {
					newest = info.ModTime()
				}
			}
		}

		usage.TotalSize += totalSize
		usage.Oldest = oldest
		usage.Newest = newest

		if len(entries) > 0 {
			usage.ByDataset = append(usage.ByDataset, DatasetSnapshot{
				Dataset: dir,
				Count:   len(entries),
				Size:    totalSize,
			})
		}
	}

	return usage, nil
}

// getFSStats gets filesystem statistics
func (a *Analyzer) getFSStats() (int64, int64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return 0, 0, err
	}

	total := int64(stat.Blocks) * int64(stat.Bsize)
	free := int64(stat.Bavail) * int64(stat.Bsize)

	return total, free, nil
}

// hashFile calculates SHA256 hash of a file
func (a *Analyzer) hashFile(path string) (string, error) {
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

// isExcluded checks if a path is excluded
func (a *Analyzer) isExcluded(path string) bool {
	for _, exclude := range a.config.ExcludePaths {
		if strings.HasPrefix(path, exclude) {
			return true
		}
	}
	return false
}

// recordSnapshot records a storage snapshot for history
func (a *Analyzer) recordSnapshot(used, free int64, fileCount int) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.history = append(a.history, Snapshot{
		Timestamp: time.Now(),
		UsedSpace: used,
		FreeSpace: free,
		FileCount: fileCount,
	})

	// Keep only last 90 days
	cutoff := time.Now().AddDate(0, 0, -90)
	for i, s := range a.history {
		if s.Timestamp.After(cutoff) {
			a.history = a.history[i:]
			break
		}
	}
}

// GetReport returns a stored report by ID
func (a *Analyzer) GetReport(id string) (*StorageReport, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	report, ok := a.reports[id]
	return report, ok
}

// GetJob returns a job by ID
func (a *Analyzer) GetJob(id string) (*AnalysisJob, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	job, ok := a.jobs[id]
	return job, ok
}

// GetReports returns all stored reports
func (a *Analyzer) GetReports() []*StorageReport {
	a.mu.RLock()
	defer a.mu.RUnlock()

	reports := make([]*StorageReport, 0, len(a.reports))
	for _, r := range a.reports {
		reports = append(reports, r)
	}
	return reports
}

// GetHistory returns storage history
func (a *Analyzer) GetHistory() []Snapshot {
	a.mu.RLock()
	defer a.mu.RUnlock()

	history := make([]Snapshot, len(a.history))
	copy(history, a.history)
	return history
}
