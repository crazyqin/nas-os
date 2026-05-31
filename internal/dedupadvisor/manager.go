// Package dedupadvisor 去重建议引擎
// 扫描存储池，分析文件重复情况，提供智能去重建议
package dedupadvisor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// FileType 文件类型
type FileType string

const (
	FileTypeDocument FileType = "document"
	FileTypeImage    FileType = "image"
	FileTypeVideo    FileType = "video"
	FileTypeAudio    FileType = "audio"
	FileTypeArchive  FileType = "archive"
	FileTypeOther    FileType = "other"
)

// DedupCandidate 去重候选
type DedupCandidate struct {
	Hash         string    `json:"hash"`
	Size         int64     `json:"size"`
	FileType     FileType  `json:"file_type"`
	Count        int       `json:"count"`          // 重复次数
	TotalSize    int64     `json:"total_size"`      // 总占用空间
	PotentialSave int64    `json:"potential_save"`   // 可节省空间
	Files        []FileInfo `json:"files"`           // 重复文件列表
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
}

// FileInfo 文件信息
type FileInfo struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	IsOriginal   bool      `json:"is_original"`   // 是否为原始文件
	AccessCount  int       `json:"access_count"`
	LastAccessed time.Time `json:"last_accessed"`
}

// ScanResult 扫描结果
type ScanResult struct {
	ScanID        string           `json:"scan_id"`
	StartTime     time.Time        `json:"start_time"`
	EndTime       time.Time        `json:"end_time"`
	Duration      time.Duration    `json:"duration"`
	TotalFiles    int              `json:"total_files"`
	TotalSize     int64            `json:"total_size"`
	DuplicateFiles int             `json:"duplicate_files"`
	DuplicateSize  int64           `json:"duplicate_size"`
	SaveableSize   int64           `json:"saveable_size"`
	DedupRatio     float64         `json:"dedup_ratio"`    // 去重率
	Candidates     []DedupCandidate `json:"candidates"`
	TopDuplicates  []DedupCandidate `json:"top_duplicates"` // 按节省空间排序的前N个
	Recommendations []Recommendation `json:"recommendations"`
}

// Recommendation 建议
type Recommendation struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`     // dedup, compress, archive
	Priority    string   `json:"priority"` // high, medium, low
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Saveable    int64    `json:"saveable"`
	Files       []string `json:"files"`
}

// ScanConfig 扫描配置
type ScanConfig struct {
	Paths           []string   `json:"paths"`
	ExcludePatterns []string   `json:"exclude_patterns"`
	MinFileSize     int64      `json:"min_file_size"`     // 最小文件大小
	MaxFileSize     int64      `json:"max_file_size"`     // 最大文件大小`
	FileTypes       []FileType `json:"file_types"`
	MaxDepth        int        `json:"max_depth"`
	FollowSymlinks  bool       `json:"follow_symlinks"`
}

// Advisor 去重建议器
type Advisor struct {
	mu       sync.RWMutex
	results  map[string]*ScanResult
	lastScan *ScanResult
	config   ScanConfig
}

// NewAdvisor 创建建议器
func NewAdvisor() *Advisor {
	return &Advisor{
		results: make(map[string]*ScanResult),
		config: ScanConfig{
			MinFileSize: 1024,      // 1KB
			MaxFileSize: 1073741824, // 1GB
			MaxDepth:    10,
		},
	}
}

// SetConfig 设置扫描配置
func (a *Advisor) SetConfig(config ScanConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.config = config
}

// Scan 扫描目录
func (a *Advisor) Scan(paths []string) (*ScanResult, error) {
	startTime := time.Now()
	scanID := fmt.Sprintf("scan-%d", startTime.Unix())

	result := &ScanResult{
		ScanID:    scanID,
		StartTime: startTime,
	}

	// 用于按哈希分组文件
	hashMap := make(map[string][]FileInfo)
	var totalSize int64
	var totalFiles int

	// 扫描每个路径
	for _, path := range paths {
		err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // 跳过无法访问的文件
			}

			// 跳过目录
			if info.IsDir() {
				return nil
			}

			// 检查文件大小
			if info.Size() < a.config.MinFileSize || info.Size() > a.config.MaxFileSize {
				return nil
			}

			// 检查排除模式
			if a.isExcluded(filePath) {
				return nil
			}

			// 计算文件哈希
			hash, err := a.calculateHash(filePath)
			if err != nil {
				return nil // 跳过无法读取的文件
			}

			// 记录文件信息
			fileInfo := FileInfo{
				Path:    filePath,
				Size:    info.Size(),
				ModTime: info.ModTime(),
			}

			hashMap[hash] = append(hashMap[hash], fileInfo)
			totalSize += info.Size()
			totalFiles++

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("扫描路径 %s 失败: %v", path, err)
		}
	}

	// 分析重复文件
	var candidates []DedupCandidate
	var duplicateFiles int
	var duplicateSize int64
	var saveableSize int64

	for hash, files := range hashMap {
		if len(files) < 2 {
			continue // 不是重复文件
		}

		// 按修改时间排序，最早的作为原始文件
		sort.Slice(files, func(i, j int) bool {
			return files[i].ModTime.Before(files[j].ModTime)
		})
		files[0].IsOriginal = true

		// 计算重复统计
		fileSize := files[0].Size
		count := len(files)
		totalDupSize := fileSize * int64(count)
		potentialSave := fileSize * int64(count-1)

		candidate := DedupCandidate{
			Hash:         hash,
			Size:         fileSize,
			FileType:     a.detectFileType(files[0].Path),
			Count:        count,
			TotalSize:    totalDupSize,
			PotentialSave: potentialSave,
			Files:        files,
			FirstSeen:    files[0].ModTime,
			LastSeen:     files[len(files)-1].ModTime,
		}

		candidates = append(candidates, candidate)
		duplicateFiles += count - 1
		duplicateSize += potentialSave
		saveableSize += potentialSave
	}

	// 按可节省空间排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].PotentialSave > candidates[j].PotentialSave
	})

	// 生成建议
	recommendations := a.generateRecommendations(candidates)

	// 取前10个作为TopDuplicates
	topCount := 10
	if len(candidates) < topCount {
		topCount = len(candidates)
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)
	result.TotalFiles = totalFiles
	result.TotalSize = totalSize
	result.DuplicateFiles = duplicateFiles
	result.DuplicateSize = duplicateSize
	result.SaveableSize = saveableSize
	if totalFiles > 0 {
		result.DedupRatio = float64(duplicateFiles) / float64(totalFiles)
	}
	result.Candidates = candidates
	result.TopDuplicates = candidates[:topCount]
	result.Recommendations = recommendations

	// 保存结果
	a.mu.Lock()
	a.results[scanID] = result
	a.lastScan = result
	a.mu.Unlock()

	return result, nil
}

// calculateHash 计算文件哈希
func (a *Advisor) calculateHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 对于大文件，只读取前64KB和后64KB
	info, err := file.Stat()
	if err != nil {
		return "", err
	}

	hasher := sha256.New()

	if info.Size() > 131072 { // > 128KB
		// 读取前64KB
		buf := make([]byte, 65536)
		n, err := io.ReadFull(file, buf)
		if err != nil && err != io.ErrUnexpectedEOF {
			return "", err
		}
		hasher.Write(buf[:n])

		// 跳到末尾读取后64KB
		file.Seek(-65536, io.SeekEnd)
		n, err = io.ReadFull(file, buf)
		if err != nil && err != io.ErrUnexpectedEOF {
			return "", err
		}
		hasher.Write(buf[:n])
	} else {
		// 小文件全部读取
		if _, err := io.Copy(hasher, file); err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// detectFileType 检测文件类型
func (a *Advisor) detectFileType(path string) FileType {
	ext := filepath.Ext(path)
	switch ext {
	case ".doc", ".docx", ".pdf", ".txt", ".md", ".xls", ".xlsx", ".ppt", ".pptx":
		return FileTypeDocument
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".heic":
		return FileTypeImage
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv":
		return FileTypeVideo
	case ".mp3", ".wav", ".flac", ".aac", ".ogg":
		return FileTypeAudio
	case ".zip", ".tar", ".gz", ".7z", ".rar":
		return FileTypeArchive
	default:
		return FileTypeOther
	}
}

// isExcluded 检查是否排除
func (a *Advisor) isExcluded(path string) bool {
	for _, pattern := range a.config.ExcludePatterns {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
	}
	return false
}

// generateRecommendations 生成建议
func (a *Advisor) generateRecommendations(candidates []DedupCandidate) []Recommendation {
	var recs []Recommendation

	// 高优先级：大文件重复
	for i, c := range candidates {
		if i >= 5 { // 最多5个高优先级
			break
		}
		if c.PotentialSave > 100*1024*1024 { // > 100MB
			recs = append(recs, Recommendation{
				ID:       fmt.Sprintf("dedup-high-%d", i),
				Type:     "dedup",
				Priority: "high",
				Title:    fmt.Sprintf("去重: %s (节省 %s)", c.Files[0].Path, formatSize(c.PotentialSave)),
				Description: fmt.Sprintf("发现 %d 个相同文件，可节省 %s 空间", c.Count, formatSize(c.PotentialSave)),
				Saveable: c.PotentialSave,
			})
		}
	}

	// 中优先级：多次重复的文件
	dedupCount := 0
	for _, c := range candidates {
		if c.Count >= 3 && dedupCount < 5 {
			recs = append(recs, Recommendation{
				ID:       fmt.Sprintf("dedup-med-%d", dedupCount),
				Type:     "dedup",
				Priority: "medium",
				Title:    fmt.Sprintf("去重: %d 个副本 - %s", c.Count, filepath.Base(c.Files[0].Path)),
				Description: fmt.Sprintf("文件有 %d 个副本，建议保留1个", c.Count),
				Saveable: c.PotentialSave,
			})
			dedupCount++
		}
	}

	return recs
}

// GetLastScan 获取上次扫描结果
func (a *Advisor) GetLastScan() *ScanResult {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.lastScan
}

// GetScanResult 获取指定扫描结果
func (a *Advisor) GetScanResult(scanID string) (*ScanResult, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result, ok := a.results[scanID]
	if !ok {
		return nil, fmt.Errorf("扫描结果 %s 不存在", scanID)
	}

	return result, nil
}

// ListScans 列出所有扫描
func (a *Advisor) ListScans() []*ScanResult {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var results []*ScanResult
	for _, r := range a.results {
		results = append(results, r)
	}

	return results
}

// formatSize 格式化文件大小
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
