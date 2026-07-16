package mediaindex

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

var (
	// ErrFileNotFound 文件未找到.
	ErrFileNotFound = errors.New("file not found")
	// ErrTagNotFound 标签未找到.
	ErrTagNotFound = errors.New("tag not found")
	// ErrCollectionNotFound 合集未找到.
	ErrCollectionNotFound = errors.New("collection not found")
	// ErrUnsupportedType 不支持的媒体类型.
	ErrUnsupportedType = errors.New("unsupported media type")
)

// Indexer 媒体索引器.
type Indexer struct {
	mu          sync.RWMutex
	files       map[string]*MediaFile
	tags        map[string]*MediaTag
	collections map[string]*MediaCollection
	indexedDirs []string
	checksums   map[string]string // checksum -> fileID for dedup
}

// NewIndexer 创建索引器.
func NewIndexer() *Indexer {
	return &Indexer{
		files:       make(map[string]*MediaFile),
		tags:        make(map[string]*MediaTag),
		collections: make(map[string]*MediaCollection),
		checksums:   make(map[string]string),
	}
}

// IndexFile 索引单个文件.
func (ix *Indexer) IndexFile(path string) (*MediaFile, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	mt := detectMediaType(path)
	if mt == "" {
		return nil, ErrUnsupportedType
	}

	checksum, err := computeChecksum(path)
	if err != nil {
		return nil, err
	}

	mf := &MediaFile{
		ID:         "media-" + time.Now().Format("20060102150405") + "-" + checksum[:8],
		Path:       path,
		Name:       filepath.Base(path),
		Type:       mt,
		MIMEType:   detectMIME(path),
		Size:       info.Size(),
		Checksum:   checksum,
		ModifiedAt: info.ModTime(),
		IndexedAt:  time.Now(),
		EXIF:       extractEXIF(path),
		Tags:       []string{},
	}

	// 尝试从EXIF提取拍摄时间
	if takenAt, ok := mf.EXIF["DateTimeOriginal"]; ok {
		if t, err := time.Parse("2006:01:02 15:04:05", takenAt); err == nil {
			mf.TakenAt = &t
		}
	}

	// 提取GPS
	if lat, ok := mf.EXIF["GPSLatitude"]; ok {
		if lon, ok2 := mf.EXIF["GPSLongitude"]; ok2 {
			mf.GPS = &GPSInfo{
				Latitude:  parseFloat(lat),
				Longitude: parseFloat(lon),
			}
		}
	}

	// 重复检测
	ix.mu.Lock()
	if existingID, exists := ix.checksums[checksum]; exists {
		mf.IsDuplicate = true
		mf.DuplicateOf = existingID
	}
	ix.checksums[checksum] = mf.ID
	ix.files[mf.ID] = mf
	ix.mu.Unlock()

	return mf, nil
}

// IndexDirectory 索引目录.
func (ix *Indexer) IndexDirectory(dir string) (int, error) {
	count := 0
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
		}
		if info.IsDir() {
			return nil
		}
		if detectMediaType(path) != "" {
			if _, err := ix.IndexFile(path); err == nil {
				count++
			}
		}
		return nil
	})
	if err != nil {
		return count, err
	}

	ix.mu.Lock()
	ix.indexedDirs = append(ix.indexedDirs, dir)
	ix.mu.Unlock()

	return count, nil
}

// Get 获取媒体文件.
func (ix *Indexer) Get(id string) (*MediaFile, error) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	mf, ok := ix.files[id]
	if !ok {
		return nil, ErrFileNotFound
	}
	return mf, nil
}

// Delete 删除索引.
func (ix *Indexer) Delete(id string) error {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	mf, ok := ix.files[id]
	if !ok {
		return ErrFileNotFound
	}
	delete(ix.checksums, mf.Checksum)
	delete(ix.files, id)
	return nil
}

// GetStats 获取索引统计.
func (ix *Indexer) GetStats() *MediaIndex {
	ix.mu.RLock()
	defer ix.mu.RUnlock()

	stats := &MediaIndex{
		ByType:      make(map[string]int),
		IndexedDirs: ix.indexedDirs,
	}
	for _, f := range ix.files {
		stats.TotalFiles++
		stats.TotalSize += f.Size
		stats.ByType[string(f.Type)]++
		if f.IsDuplicate {
			stats.DuplicateCount++
		}
	}
	return stats
}

// GetTimeline 获取时间线.
func (ix *Indexer) GetTimeline() []*TimelineGroup {
	ix.mu.RLock()
	defer ix.mu.RUnlock()

	groups := make(map[string][]*MediaFile)
	for _, f := range ix.files {
		date := ""
		if f.TakenAt != nil {
			date = f.TakenAt.Format("2006-01-02")
		} else {
			date = f.IndexedAt.Format("2006-01-02")
		}
		groups[date] = append(groups[date], f)
	}

	result := make([]*TimelineGroup, 0, len(groups))
	for date, files := range groups {
		sort.Slice(files, func(i, j int) bool {
			return files[i].Name < files[j].Name
		})
		result = append(result, &TimelineGroup{
			Date:  date,
			Count: len(files),
			Files: files,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Date > result[j].Date
	})
	return result
}

// AddTag 添加标签.
func (ix *Indexer) AddTag(name string) *MediaTag {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	tag := &MediaTag{
		ID:       "tag-" + strings.ToLower(strings.ReplaceAll(name, " ", "-")),
		Name:     name,
		CreateAt: time.Now(),
	}
	ix.tags[tag.ID] = tag
	return tag
}

// GetTags 获取所有标签.
func (ix *Indexer) GetTags() []*MediaTag {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	result := make([]*MediaTag, 0, len(ix.tags))
	for _, t := range ix.tags {
		result = append(result, t)
	}
	return result
}

// TagFile 给文件打标签.
func (ix *Indexer) TagFile(fileID, tagID string) error {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	f, ok := ix.files[fileID]
	if !ok {
		return ErrFileNotFound
	}
	if _, ok := ix.tags[tagID]; !ok {
		return ErrTagNotFound
	}
	for _, t := range f.Tags {
		if t == tagID {
			return nil // 已有标签
		}
	}
	f.Tags = append(f.Tags, tagID)
	ix.tags[tagID].FileIDs = append(ix.tags[tagID].FileIDs, fileID)
	return nil
}

// CreateCollection 创建合集.
func (ix *Indexer) CreateCollection(name, desc string) *MediaCollection {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	c := &MediaCollection{
		ID:          "col-" + time.Now().Format("20060102150405"),
		Name:        name,
		Description: desc,
		FileIDs:     []string{},
		CreateAt:    time.Now(),
		UpdatedAt:   time.Now(),
	}
	ix.collections[c.ID] = c
	return c
}

// GetCollections 获取所有合集.
func (ix *Indexer) GetCollections() []*MediaCollection {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	result := make([]*MediaCollection, 0, len(ix.collections))
	for _, c := range ix.collections {
		result = append(result, c)
	}
	return result
}

// AddToCollection 添加文件到合集.
func (ix *Indexer) AddToCollection(colID, fileID string) error {
	ix.mu.Lock()
	defer ix.mu.Unlock()
	c, ok := ix.collections[colID]
	if !ok {
		return ErrCollectionNotFound
	}
	if _, ok := ix.files[fileID]; !ok {
		return ErrFileNotFound
	}
	for _, id := range c.FileIDs {
		if id == fileID {
			return nil
		}
	}
	c.FileIDs = append(c.FileIDs, fileID)
	c.UpdatedAt = time.Now()
	return nil
}

// detectMediaType 检测媒体类型.
func detectMediaType(path string) MediaType {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp", ".svg", ".tiff", ".heic", ".heif":
		return MediaTypeImage
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv", ".flv", ".webm", ".m4v", ".ts":
		return MediaTypeVideo
	case ".mp3", ".flac", ".wav", ".aac", ".ogg", ".wma", ".m4a", ".opus":
		return MediaTypeAudio
	default:
		return ""
	}
}

// detectMIME 检测MIME类型.
func detectMIME(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	mimes := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png",
		".gif": "image/gif", ".webp": "image/webp", ".svg": "image/svg+xml",
		".mp4": "video/mp4", ".avi": "video/x-msvideo", ".mkv": "video/x-matroska",
		".mov": "video/quicktime", ".webm": "video/webm",
		".mp3": "audio/mpeg", ".flac": "audio/flac", ".wav": "audio/wav",
		".ogg": "audio/ogg", ".m4a": "audio/mp4", ".aac": "audio/aac",
	}
	if m, ok := mimes[ext]; ok {
		return m
	}
	return "application/octet-stream"
}

// computeChecksum 计算文件SHA256.
func computeChecksum(path string) (string, error) {
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

// extractEXIF 提取EXIF信息（简化实现）.
func extractEXIF(path string) map[string]string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".tiff" {
		return nil
	}
	// 简化实现：返回空map，实际应使用exif库
	return make(map[string]string)
}

// parseFloat 解析浮点数字符串.
func parseFloat(s string) float64 {
	var f float64
	for _, c := range s {
		if c >= '0' && c <= '9' || c == '.' || c == '-' {
			// 简化实现
		}
	}
	_ = f
	return 0.0
}
