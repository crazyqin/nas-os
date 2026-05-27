package diskspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DiskSpaceManager manages disk space analysis operations
type DiskSpaceManager struct {
	mu           sync.RWMutex
	scanProgress ScanProgress
	scanCancel   context.CancelFunc
	stats        map[string]DiskStats
	growthData   []GrowthTrend
}

// NewDiskSpaceManager creates a new disk space manager
func NewDiskSpaceManager() *DiskSpaceManager {
	return &DiskSpaceManager{
		stats:      make(map[string]DiskStats),
		growthData: generateMockGrowthData(),
	}
}

// StartScan starts a disk scan operation
func (m *DiskSpaceManager) StartScan(ctx context.Context, config ScanConfig) error {
	m.mu.Lock()
	
	// Cancel any existing scan
	if m.scanCancel != nil {
		m.scanCancel()
	}
	
	scanCtx, cancel := context.WithCancel(ctx)
	m.scanCancel = cancel
	
	m.scanProgress = ScanProgress{
		Status: "running",
	}
	m.mu.Unlock()
	
	go m.runScan(scanCtx, config)
	
	return nil
}

// runScan performs the actual scan operation
func (m *DiskSpaceManager) runScan(ctx context.Context, config ScanConfig) {
	startTime := time.Now()
	
	m.mu.Lock()
	m.scanProgress.Status = "running"
	m.mu.Unlock()
	
	// Simulate scanning with mock data
	paths := []string{
		"/home", "/var", "/usr", "/tmp", "/opt",
	}
	
	for i, path := range paths {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			m.scanProgress.Status = "cancelled"
			m.mu.Unlock()
			return
		default:
		}
		
		m.mu.Lock()
		m.scanProgress.CurrentPath = path
		m.scanProgress.ScannedDirs = (i + 1) * 100
		m.scanProgress.ScannedFiles = (i + 1) * 500
		m.scanProgress.Percent = float64(i+1) / float64(len(paths)) * 100
		m.scanProgress.Elapsed = time.Since(startTime).Seconds()
		m.mu.Unlock()
		
		time.Sleep(100 * time.Millisecond)
	}
	
	// Generate mock results
	m.mu.Lock()
	m.scanProgress.Status = "completed"
	m.scanProgress.Percent = 100
	m.scanProgress.Elapsed = time.Since(startTime).Seconds()
	
	// Store results for each path
	for _, path := range paths {
		m.stats[path] = m.generateMockStats(path)
	}
	m.mu.Unlock()
}

// GetScanProgress returns the current scan progress
func (m *DiskSpaceManager) GetScanProgress() ScanProgress {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.scanProgress
}

// GetDiskUsage returns disk usage for a path
func (m *DiskSpaceManager) GetDiskUsage(path string) DiskUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Try to get real disk usage
	usage, err := getRealDiskUsage(path)
	if err == nil {
		return usage
	}
	
	// Return mock data
	return DiskUsage{
		Total:        1024 * 1024 * 1024 * 100, // 100GB
		Used:         1024 * 1024 * 1024 * 65,  // 65GB
		Free:         1024 * 1024 * 1024 * 35,  // 35GB
		UsagePercent: 65.0,
		InodeTotal:   1000000,
		InodeUsed:    650000,
		InodeFree:    350000,
		InodePercent: 65.0,
	}
}

// GetDirectoryTree returns the directory tree for a path
func (m *DiskSpaceManager) GetDirectoryTree(path string, maxDepth int) DirectoryNode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.buildDirectoryTree(path, 0, maxDepth)
}

// buildDirectoryTree recursively builds the directory tree
func (m *DiskSpaceManager) buildDirectoryTree(path string, depth, maxDepth int) DirectoryNode {
	name := filepath.Base(path)
	if path == "/" {
		name = "/"
	}
	
	node := DirectoryNode{
		Path:       path,
		Name:       name,
		Depth:      depth,
		ParentPath: filepath.Dir(path),
	}
	
	if depth >= maxDepth {
		// Return leaf node with mock data
		node.Size = 1024 * 1024 * 100 // 100MB
		node.FileCount = 50
		node.DirCount = 3
		return node
	}
	
	// Generate mock children
	mockDirs := []string{"documents", "pictures", "videos", "music", "downloads"}
	for _, dir := range mockDirs {
		childPath := filepath.Join(path, dir)
		child := m.buildDirectoryTree(childPath, depth+1, maxDepth)
		node.Children = append(node.Children, child)
		node.Size += child.Size
		node.FileCount += child.FileCount
		node.DirCount += child.DirCount + 1
	}
	
	return node
}

// GetFileTypeStats returns file type statistics for a path
func (m *DiskSpaceManager) GetFileTypeStats(path string) []FileTypeStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Mock file type statistics
	stats := []FileTypeStats{
		{Extension: ".mp4", Count: 150, TotalSize: 1024 * 1024 * 1024 * 50, Percentage: 45.0},
		{Extension: ".jpg", Count: 5000, TotalSize: 1024 * 1024 * 1024 * 20, Percentage: 18.0},
		{Extension: ".pdf", Count: 200, TotalSize: 1024 * 1024 * 1024 * 5, Percentage: 4.5},
		{Extension: ".docx", Count: 150, TotalSize: 1024 * 1024 * 1024 * 2, Percentage: 1.8},
		{Extension: ".zip", Count: 50, TotalSize: 1024 * 1024 * 1024 * 10, Percentage: 9.0},
		{Extension: ".mp3", Count: 1000, TotalSize: 1024 * 1024 * 1024 * 15, Percentage: 13.5},
		{Extension: "other", Count: 3000, TotalSize: 1024 * 1024 * 1024 * 9, Percentage: 8.2},
	}
	
	return stats
}

// FindLargeFiles finds large files in a path
func (m *DiskSpaceManager) FindLargeFiles(path string, minSize int64, limit int) []LargeFileInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Mock large files
	files := []LargeFileInfo{
		{
			Path:       filepath.Join(path, "videos/movie.mp4"),
			Size:       1024 * 1024 * 1024 * 4,
			ModifiedAt: time.Now().Add(-24 * time.Hour),
			Owner:      "user",
			Extension:  ".mp4",
		},
		{
			Path:       filepath.Join(path, "backups/backup.tar.gz"),
			Size:       1024 * 1024 * 1024 * 2,
			ModifiedAt: time.Now().Add(-48 * time.Hour),
			Owner:      "root",
			Extension:  ".gz",
		},
		{
			Path:       filepath.Join(path, "downloads/largefile.iso"),
			Size:       1024 * 1024 * 500,
			ModifiedAt: time.Now().Add(-72 * time.Hour),
			Owner:      "user",
			Extension:  ".iso",
		},
		{
			Path:       filepath.Join(path, "documents/archive.zip"),
			Size:       1024 * 1024 * 200,
			ModifiedAt: time.Now().Add(-96 * time.Hour),
			Owner:      "user",
			Extension:  ".zip",
		},
		{
			Path:       filepath.Join(path, "pictures/raw/photo.raw"),
			Size:       1024 * 1024 * 50,
			ModifiedAt: time.Now().Add(-120 * time.Hour),
			Owner:      "user",
			Extension:  ".raw",
		},
	}
	
	// Filter by minSize
	filtered := make([]LargeFileInfo, 0)
	for _, f := range files {
		if f.Size >= minSize {
			filtered = append(filtered, f)
		}
	}
	
	// Apply limit
	if limit > 0 && limit < len(filtered) {
		filtered = filtered[:limit]
	}
	
	return filtered
}

// FindDuplicates finds duplicate files in a path
func (m *DiskSpaceManager) FindDuplicates(ctx context.Context, path string) ([]DuplicateFile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// Mock duplicate files
	duplicates := []DuplicateFile{
		{
			Hash: "abc123def456",
			Files: []FileInfo{
				{Path: filepath.Join(path, "photos/image1.jpg"), Size: 1024 * 1024 * 5},
				{Path: filepath.Join(path, "backup/image1.jpg"), Size: 1024 * 1024 * 5},
				{Path: filepath.Join(path, "downloads/image1.jpg"), Size: 1024 * 1024 * 5},
			},
			TotalWastedSpace: 1024 * 1024 * 10,
		},
		{
			Hash: "789xyz000abc",
			Files: []FileInfo{
				{Path: filepath.Join(path, "documents/report.pdf"), Size: 1024 * 1024 * 2},
				{Path: filepath.Join(path, "backup/report.pdf"), Size: 1024 * 1024 * 2},
			},
			TotalWastedSpace: 1024 * 1024 * 2,
		},
		{
			Hash: "def456ghi789",
			Files: []FileInfo{
				{Path: filepath.Join(path, "music/song.mp3"), Size: 1024 * 1024 * 8},
				{Path: filepath.Join(path, "backup/song.mp3"), Size: 1024 * 1024 * 8},
			},
			TotalWastedSpace: 1024 * 1024 * 8,
		},
	}
	
	return duplicates, nil
}

// GetTreemapData returns treemap visualization data
func (m *DiskSpaceManager) GetTreemapData(path string, maxDepth int) TreemapData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.buildTreemap(path, 0, maxDepth)
}

// buildTreemap recursively builds treemap data
func (m *DiskSpaceManager) buildTreemap(path string, depth, maxDepth int) TreemapData {
	name := filepath.Base(path)
	if path == "/" {
		name = "root"
	}
	
	node := TreemapData{
		Name: name,
		Path: path,
	}
	
	if depth >= maxDepth {
		node.Size = 1024 * 1024 * 500 // 500MB
		node.Color = getColorForDepth(depth)
		return node
	}
	
	// Generate mock children
	mockDirs := []string{"documents", "pictures", "videos", "music", "downloads", "backups"}
	for _, dir := range mockDirs {
		childPath := filepath.Join(path, dir)
		child := m.buildTreemap(childPath, depth+1, maxDepth)
		node.Children = append(node.Children, child)
		node.Size += child.Size
	}
	
	node.Color = getColorForDepth(depth)
	
	return node
}

// GetGrowthTrend returns disk usage growth trend
func (m *DiskSpaceManager) GetGrowthTrend(days int) []GrowthTrend {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	if days <= 0 {
		days = 30
	}
	
	if days > len(m.growthData) {
		days = len(m.growthData)
	}
	
	return m.growthData[:days]
}

// CleanupCache cleans up cached data
func (m *DiskSpaceManager) CleanupCache() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.stats = make(map[string]DiskStats)
	m.scanProgress = ScanProgress{}
}

// ExportReport exports a disk usage report
func (m *DiskSpaceManager) ExportReport(format string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	report := map[string]interface{}{
		"generated_at": time.Now(),
		"format":       format,
		"summary": map[string]interface{}{
			"total_space":    1024 * 1024 * 1024 * 100,
			"used_space":     1024 * 1024 * 1024 * 65,
			"free_space":     1024 * 1024 * 1024 * 35,
			"usage_percent":  65.0,
			"total_files":    15000,
			"total_dirs":     500,
		},
		"file_types": m.GetFileTypeStats("/"),
		"top_dirs": []map[string]interface{}{
			{"path": "/home", "size": 1024 * 1024 * 1024 * 30},
			{"path": "/var", "size": 1024 * 1024 * 1024 * 15},
			{"path": "/usr", "size": 1024 * 1024 * 1024 * 10},
		},
	}
	
	switch format {
	case "json":
		return json.MarshalIndent(report, "", "  ")
	case "text":
		return []byte(formatTextReport(report)), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// formatTextReport formats the report as text
func formatTextReport(report map[string]interface{}) string {
	var sb strings.Builder
	
	sb.WriteString("=== Disk Usage Report ===\n")
	sb.WriteString(fmt.Sprintf("Generated: %v\n\n", report["generated_at"]))
	
	if summary, ok := report["summary"].(map[string]interface{}); ok {
		sb.WriteString("--- Summary ---\n")
		sb.WriteString(fmt.Sprintf("Total Space: %s\n", formatBytes(toInt64(summary["total_space"]))))
		sb.WriteString(fmt.Sprintf("Used Space: %s\n", formatBytes(toInt64(summary["used_space"]))))
		sb.WriteString(fmt.Sprintf("Free Space: %s\n", formatBytes(toInt64(summary["free_space"]))))
		sb.WriteString(fmt.Sprintf("Usage: %.1f%%\n", toFloat64(summary["usage_percent"])))
	}
	
	return sb.String()
}

// toInt64 converts interface{} to int64
func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int:
		return int64(val)
	case int64:
		return val
	case float64:
		return int64(val)
	default:
		return 0
	}
}

// toFloat64 converts interface{} to float64
func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

// formatBytes formats bytes to human readable string
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// getColorForDepth returns a color based on depth
func getColorForDepth(depth int) string {
	colors := []string{
		"#4CAF50", // Green
		"#2196F3", // Blue
		"#FF9800", // Orange
		"#9C27B0", // Purple
		"#F44336", // Red
		"#00BCD4", // Cyan
	}
	return colors[depth%len(colors)]
}

// generateMockGrowthData generates mock growth trend data
func generateMockGrowthData() []GrowthTrend {
	data := make([]GrowthTrend, 30)
	baseSpace := int64(1024 * 1024 * 1024 * 50) // 50GB
	baseFiles := 10000
	
	for i := 0; i < 30; i++ {
		data[i] = GrowthTrend{
			Date:       time.Now().AddDate(0, 0, -30+i),
			UsedSpace:  baseSpace + int64(i*1024*1024*500), // +500MB per day
			FileCount:  baseFiles + i*100,
		}
	}
	
	return data
}

// generateMockStats generates mock disk statistics
func (m *DiskSpaceManager) generateMockStats(path string) DiskStats {
	return DiskStats{
		DiskUsage: m.GetDiskUsage(path),
		TopDirectories: []DirectoryNode{
			{Path: filepath.Join(path, "videos"), Name: "videos", Size: 1024 * 1024 * 1024 * 30, FileCount: 150, DirCount: 10},
			{Path: filepath.Join(path, "pictures"), Name: "pictures", Size: 1024 * 1024 * 1024 * 20, FileCount: 5000, DirCount: 50},
			{Path: filepath.Join(path, "documents"), Name: "documents", Size: 1024 * 1024 * 1024 * 5, FileCount: 200, DirCount: 20},
		},
		FileTypeDistribution: m.GetFileTypeStats(path),
		LargestFiles:         m.FindLargeFiles(path, 0, 10),
		GrowthTrend:          m.GetGrowthTrend(30),
	}
}

// getRealDiskUsage attempts to get real disk usage using syscall
func getRealDiskUsage(path string) (DiskUsage, error) {
	// This would use syscall.Statfs in production
	// For now, return error to use mock data
	return DiskUsage{}, fmt.Errorf("not implemented")
}

// calculateFileHash calculates SHA256 hash of a file
func calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// getFileInfo gets file information
func getFileInfo(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

// isHidden checks if a file is hidden
func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

// getFileExtension gets file extension
func getFileExtension(path string) string {
	return strings.ToLower(filepath.Ext(path))
}

// sortFilesBySize sorts files by size (largest first)
func sortFilesBySize(files []LargeFileInfo) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].Size > files[j].Size
	})
}
