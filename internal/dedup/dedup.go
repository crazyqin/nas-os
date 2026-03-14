// Package dedup 数据去重核心功能
package dedup

import (
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
	"sync/atomic"
	"time"
)

// ========== 数据结构 ==========

// Chunk 表示一个数据块
type Chunk struct {
	Hash       string            `json:"hash"`
	Size       int64             `json:"size"`
	Offset     int64             `json:"offset"`
	RefCount   int32             `json:"refCount"`
	Users      map[string]bool   `json:"users,omitempty"`
	StorePath  string            `json:"storePath,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
	AccessedAt time.Time         `json:"accessedAt"`
}

// ChunkIndex 块索引
type ChunkIndex struct {
	mu     sync.RWMutex
	chunks map[string]*Chunk // hash -> chunk
}

// NewChunkIndex 创建块索引
func NewChunkIndex() *ChunkIndex {
	return &ChunkIndex{
		chunks: make(map[string]*Chunk),
	}
}

// FileRecord 文件记录
type FileRecord struct {
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	Checksum    string    `json:"checksum"`
	ChunkHashes []string  `json:"chunkHashes"`
	User        string    `json:"user,omitempty"`
	Shared      bool      `json:"shared,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	ModifiedAt  time.Time `json:"modifiedAt"`
	AccessedAt  time.Time `json:"accessedAt"`
}

// DuplicateGroup 重复文件组
type DuplicateGroup struct {
	Checksum  string              `json:"checksum"`
	Size      int64               `json:"size"`
	Files     []string            `json:"files"`
	Users     []string            `json:"users,omitempty"`
	Savings   int64               `json:"savings"`
	UserFiles map[string][]string `json:"userFiles,omitempty"`
}

// DedupStats 去重统计
type DedupStats struct {
	mu sync.RWMutex

	// 文件统计
	TotalFiles     int64 `json:"totalFiles"`
	TotalSize      int64 `json:"totalSize"`
	DuplicateFiles int64 `json:"duplicateFiles"`
	DuplicateSize  int64 `json:"duplicateSize"`

	// 空间统计
	SavingsPotential int64 `json:"savingsPotential"`
	SavingsActual    int64 `json:"savingsActual"`

	// 块统计
	ChunksStored  int   `json:"chunksStored"`
	ChunkDataSize int64 `json:"chunkDataSize"`

	// 跨用户统计
	SharedChunks     int   `json:"sharedChunks"`
	SharedDataSize   int64 `json:"sharedDataSize"`
	CrossUserSavings int64 `json:"crossUserSavings"`
	UserCount        int   `json:"userCount"`

	// 时间统计
	LastScanTime  time.Time     `json:"lastScanTime"`
	LastDedupTime time.Time     `json:"lastDedupTime"`
	TotalScanTime time.Duration `json:"totalScanTime"`
}

// GetValues 获取统计值（线程安全）
func (s *DedupStats) GetValues() (totalFiles, totalSize, duplicateFiles, duplicateSize, savingsPotential, savingsActual int64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.TotalFiles, s.TotalSize, s.DuplicateFiles, s.DuplicateSize, s.SavingsPotential, s.SavingsActual
}

// Update 更新统计（线程安全）
func (s *DedupStats) Update(fn func(*DedupStats)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(s)
}

// Clone 克隆统计
func (s *DedupStats) Clone() DedupStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return *s
}

// Progress 进度信息
type Progress struct {
	Phase      string    `json:"phase"`   // 当前阶段
	Current    int64     `json:"current"` // 当前进度
	Total      int64     `json:"total"`   // 总数
	Percent    float64   `json:"percent"` // 百分比
	Speed      float64   `json:"speed"`   // 速度 (MB/s)
	ETA        int       `json:"eta"`     // 预计剩余时间 (秒)
	Message    string    `json:"message"` // 进度消息
	StartTime  time.Time `json:"startTime"`
	LastUpdate time.Time `json:"lastUpdate"`
}

// ProgressCallback 进度回调函数
type ProgressCallback func(progress *Progress)

// ========== 块指纹计算 ==========

// ChunkFingerprint 计算文件块指纹
func ChunkFingerprint(filePath string, chunkSize int64) ([]*Chunk, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	if info.Size() == 0 {
		return nil, nil
	}

	var chunks []*Chunk
	buf := make([]byte, chunkSize)
	offset := int64(0)

	for {
		n, err := file.Read(buf)
		if n == 0 {
			break
		}

		data := buf[:n]
		hash := sha256.Sum256(data)
		hashStr := hex.EncodeToString(hash[:])

		chunk := &Chunk{
			Hash:       hashStr,
			Size:       int64(n),
			Offset:     offset,
			RefCount:   1,
			CreatedAt:  time.Now(),
			AccessedAt: time.Now(),
		}

		chunks = append(chunks, chunk)
		offset += int64(n)

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取文件失败: %w", err)
		}
	}

	return chunks, nil
}

// ChunkFingerprintWithProgress 带进度的块指纹计算
func ChunkFingerprintWithProgress(filePath string, chunkSize int64, callback ProgressCallback) ([]*Chunk, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	if info.Size() == 0 {
		return nil, nil
	}

	progress := &Progress{
		Phase:      "fingerprint",
		Total:      info.Size(),
		StartTime:  time.Now(),
		LastUpdate: time.Now(),
	}

	var chunks []*Chunk
	buf := make([]byte, chunkSize)
	offset := int64(0)

	for {
		n, err := file.Read(buf)
		if n == 0 {
			break
		}

		data := buf[:n]
		hash := sha256.Sum256(data)
		hashStr := hex.EncodeToString(hash[:])

		chunk := &Chunk{
			Hash:       hashStr,
			Size:       int64(n),
			Offset:     offset,
			RefCount:   1,
			CreatedAt:  time.Now(),
			AccessedAt: time.Now(),
		}

		chunks = append(chunks, chunk)
		offset += int64(n)

		// 更新进度
		progress.Current = offset
		progress.Percent = float64(offset) * 100 / float64(progress.Total)
		elapsed := time.Since(progress.StartTime).Seconds()
		if elapsed > 0 {
			progress.Speed = float64(offset) / 1024 / 1024 / elapsed
			if progress.Speed > 0 {
				remaining := float64(progress.Total-offset) / 1024 / 1024 / progress.Speed
				progress.ETA = int(remaining)
			}
		}
		progress.LastUpdate = time.Now()

		if callback != nil {
			callback(progress)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("读取文件失败: %w", err)
		}
	}

	return chunks, nil
}

// FileChecksum 计算文件整体校验和
func FileChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
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

// ========== 查找重复块 ==========

// FindDuplicatesResult 查找重复结果
type FindDuplicatesResult struct {
	Groups           []*DuplicateGroup `json:"groups"`
	TotalFiles       int               `json:"totalFiles"`
	TotalSize        int64             `json:"totalSize"`
	DuplicateGroups  int               `json:"duplicateGroups"`
	DuplicatesFound  int               `json:"duplicatesFound"`
	SavingsPotential int64             `json:"savingsPotential"`
	CrossUserGroups  int               `json:"crossUserGroups"`
	CrossUserSavings int64             `json:"crossUserSavings"`
	Errors           []ScanError       `json:"errors"`
	Duration         time.Duration     `json:"duration"`
}

// ScanError 扫描错误
type ScanError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// FindDuplicates 查找重复文件
func FindDuplicates(paths []string, config *Config, callback ProgressCallback) (*FindDuplicatesResult, error) {
	if config == nil {
		config = DefaultConfig()
	}

	start := time.Now()
	result := &FindDuplicatesResult{
		Groups: make([]*DuplicateGroup, 0),
		Errors: make([]ScanError, 0),
	}

	// 文件校验和索引
	checksums := make(map[string][]*FileRecord)
	fileRecords := make(map[string]*FileRecord)
	userIndexes := make(map[string]map[string]string) // user -> path -> checksum

	progress := &Progress{
		Phase:      "scan",
		StartTime:  time.Now(),
		LastUpdate: time.Now(),
	}

	// 扫描文件
	for _, rootPath := range paths {
		err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				result.Errors = append(result.Errors, ScanError{
					Path:    path,
					Message: err.Error(),
				})
				return nil
			}

			if info.IsDir() {
				// 检查排除路径
				for _, exclude := range config.ExcludePaths {
					if strings.HasPrefix(path, exclude) {
						return filepath.SkipDir
					}
				}
				return nil
			}

			// 检查最小大小
			if info.Size() < config.MinFileSize {
				return nil
			}

			// 检查排除模式
			for _, pattern := range config.ExcludePatterns {
				matched, _ := filepath.Match(pattern, filepath.Base(path))
				if matched {
					return nil
				}
			}

			// 计算校验和
			checksum, err := FileChecksum(path)
			if err != nil {
				result.Errors = append(result.Errors, ScanError{
					Path:    path,
					Message: err.Error(),
				})
				return nil
			}

			// 提取用户信息
			user := extractUserFromPath(path)

			record := &FileRecord{
				Path:       path,
				Size:       info.Size(),
				Checksum:   checksum,
				User:       user,
				CreatedAt:  time.Now(),
				ModifiedAt: info.ModTime(),
			}

			fileRecords[path] = record
			checksums[checksum] = append(checksums[checksum], record)

			if user != "" {
				if userIndexes[user] == nil {
					userIndexes[user] = make(map[string]string)
				}
				userIndexes[user][path] = checksum
			}

			result.TotalFiles++
			result.TotalSize += info.Size()

			// 更新进度
			progress.Current = int64(result.TotalFiles)
			progress.Percent = float64(progress.Current) / float64(progress.Current+100) * 100
			progress.Message = path
			progress.LastUpdate = time.Now()

			if callback != nil {
				callback(progress)
			}

			return nil
		})

		if err != nil {
			result.Errors = append(result.Errors, ScanError{
				Path:    rootPath,
				Message: err.Error(),
			})
		}
	}

	// 分析重复
	progress.Phase = "analyze"
	progress.Current = 0
	progress.Total = int64(len(checksums))

	for checksum, records := range checksums {
		if len(records) > 1 {
			files := make([]string, len(records))
			userFiles := make(map[string][]string)
			users := make(map[string]bool)

			for i, r := range records {
				files[i] = r.Path
				if r.User != "" {
					users[r.User] = true
					userFiles[r.User] = append(userFiles[r.User], r.Path)
				}
			}

			size := records[0].Size
			savings := size * int64(len(records)-1)

			group := &DuplicateGroup{
				Checksum:  checksum,
				Size:      size,
				Files:     files,
				Savings:   savings,
				UserFiles: userFiles,
			}

			for u := range users {
				group.Users = append(group.Users, u)
			}

			result.Groups = append(result.Groups, group)
			result.DuplicatesFound += len(records) - 1
			result.SavingsPotential += savings

			if len(users) > 1 {
				result.CrossUserGroups++
				result.CrossUserSavings += savings
			}
		}

		progress.Current++
		if callback != nil {
			progress.Percent = float64(progress.Current) * 100 / float64(progress.Total)
			callback(progress)
		}
	}

	// 按节省空间排序
	sort.Slice(result.Groups, func(i, j int) bool {
		return result.Groups[i].Savings > result.Groups[j].Savings
	})

	result.DuplicateGroups = len(result.Groups)
	result.Duration = time.Since(start)

	return result, nil
}

// FindDuplicateChunks 查找重复块
func FindDuplicateChunks(filePath string, chunkSize int64, index *ChunkIndex) ([]*Chunk, error) {
	chunks, err := ChunkFingerprint(filePath, chunkSize)
	if err != nil {
		return nil, err
	}

	index.mu.RLock()
	defer index.mu.RUnlock()

	var duplicates []*Chunk
	for _, chunk := range chunks {
		if existing, ok := index.chunks[chunk.Hash]; ok {
			duplicates = append(duplicates, existing)
		}
	}

	return duplicates, nil
}

// ========== 执行去重 ==========

// DeduplicateResult 去重结果
type DeduplicateResult struct {
	Success        bool          `json:"success"`
	ProcessedFiles int           `json:"processedFiles"`
	SavedBytes     int64         `json:"savedBytes"`
	Groups         []GroupResult `json:"groups"`
	Errors         []DedupError  `json:"errors"`
	Duration       time.Duration `json:"duration"`
	DryRun         bool          `json:"dryRun"`
}

// GroupResult 组处理结果
type GroupResult struct {
	Checksum  string   `json:"checksum"`
	KeepPath  string   `json:"keepPath"`
	Processed []string `json:"processed"`
	Saved     int64    `json:"saved"`
	Skipped   []string `json:"skipped,omitempty"`
}

// DedupError 去重错误
type DedupError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// Deduplicate 执行去重
func Deduplicate(groups []*DuplicateGroup, policy *DedupPolicy, callback ProgressCallback) (*DeduplicateResult, error) {
	if policy == nil {
		policy = DefaultPolicy()
	}

	start := time.Now()
	result := &DeduplicateResult{
		DryRun: policy.DryRun,
		Groups: make([]GroupResult, 0),
		Errors: make([]DedupError, 0),
	}

	progress := &Progress{
		Phase:      "deduplicate",
		Total:      int64(len(groups)),
		StartTime:  time.Now(),
		LastUpdate: time.Now(),
	}

	for i, group := range groups {
		if len(group.Files) < 2 {
			continue
		}

		// 选择保留的文件
		keepPath, err := selectKeepFile(group, policy.Retention)
		if err != nil {
			result.Errors = append(result.Errors, DedupError{
				Path:  group.Files[0],
				Error: err.Error(),
			})
			continue
		}

		groupResult := GroupResult{
			Checksum:  group.Checksum,
			KeepPath:  keepPath,
			Processed: make([]string, 0),
			Skipped:   make([]string, 0),
		}

		// 处理重复文件
		for _, filePath := range group.Files {
			if filePath == keepPath {
				continue
			}

			if policy.DryRun {
				groupResult.Processed = append(groupResult.Processed, filePath)
				groupResult.Saved += group.Size
				result.SavedBytes += group.Size
				result.ProcessedFiles++
				continue
			}

			// 执行去重操作
			err := executeDedupAction(filePath, keepPath, policy.Action, policy.PreserveAttrs)
			if err != nil {
				result.Errors = append(result.Errors, DedupError{
					Path:  filePath,
					Error: err.Error(),
				})
				groupResult.Skipped = append(groupResult.Skipped, filePath)
				continue
			}

			groupResult.Processed = append(groupResult.Processed, filePath)
			groupResult.Saved += group.Size
			result.SavedBytes += group.Size
			result.ProcessedFiles++
		}

		result.Groups = append(result.Groups, groupResult)

		// 更新进度
		progress.Current = int64(i + 1)
		progress.Percent = float64(progress.Current) * 100 / float64(progress.Total)
		progress.Message = keepPath
		progress.LastUpdate = time.Now()

		if callback != nil {
			callback(progress)
		}
	}

	result.Success = len(result.Errors) == 0
	result.Duration = time.Since(start)

	return result, nil
}

// selectKeepFile 选择保留的文件
func selectKeepFile(group *DuplicateGroup, retention RetentionPolicy) (string, error) {
	if len(group.Files) == 0 {
		return "", fmt.Errorf("空重复组")
	}

	if len(group.Files) == 1 {
		return group.Files[0], nil
	}

	switch retention {
	case RetentionKeepFirst:
		return group.Files[0], nil

	case RetentionKeepOldest, RetentionKeepNewest:
		// 需要获取文件修改时间
		var selectedPath string
		var selectedTime time.Time

		for i, path := range group.Files {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}

			if i == 0 {
				selectedPath = path
				selectedTime = info.ModTime()
				continue
			}

			if retention == RetentionKeepOldest && info.ModTime().Before(selectedTime) {
				selectedPath = path
				selectedTime = info.ModTime()
			} else if retention == RetentionKeepNewest && info.ModTime().After(selectedTime) {
				selectedPath = path
				selectedTime = info.ModTime()
			}
		}

		if selectedPath != "" {
			return selectedPath, nil
		}

	case RetentionKeepLargest:
		var selectedPath string
		var selectedSize int64

		for i, path := range group.Files {
			info, err := os.Stat(path)
			if err != nil {
				continue
			}

			if i == 0 || info.Size() > selectedSize {
				selectedPath = path
				selectedSize = info.Size()
			}
		}

		if selectedPath != "" {
			return selectedPath, nil
		}

	case RetentionManual:
		// 手动选择需要外部处理，默认返回第一个
		return group.Files[0], nil
	}

	// 默认返回第一个
	return group.Files[0], nil
}

// executeDedupAction 执行去重操作
func executeDedupAction(targetPath, keepPath string, action DedupAction, preserveAttrs bool) error {
	switch action {
	case ActionReport:
		// 仅报告，不做任何操作
		return nil

	case ActionSoftlink:
		// 保存属性
		var mode os.FileMode
		var modTime time.Time
		if preserveAttrs {
			info, err := os.Lstat(targetPath)
			if err == nil {
				mode = info.Mode()
				modTime = info.ModTime()
			}
		}

		// 删除目标文件
		if err := os.Remove(targetPath); err != nil {
			return fmt.Errorf("删除文件失败: %w", err)
		}

		// 创建软链接
		if err := os.Symlink(keepPath, targetPath); err != nil {
			return fmt.Errorf("创建软链接失败: %w", err)
		}

		// 恢复属性
		if preserveAttrs {
			if err := os.Chmod(targetPath, mode); err == nil {
				os.Chtimes(targetPath, modTime, modTime)
			}
		}

		return nil

	case ActionHardlink:
		// 保存属性
		var mode os.FileMode
		var modTime time.Time
		if preserveAttrs {
			info, err := os.Lstat(targetPath)
			if err == nil {
				mode = info.Mode()
				modTime = info.ModTime()
			}
		}

		// 删除目标文件
		if err := os.Remove(targetPath); err != nil {
			return fmt.Errorf("删除文件失败: %w", err)
		}

		// 创建硬链接
		if err := os.Link(keepPath, targetPath); err != nil {
			return fmt.Errorf("创建硬链接失败: %w", err)
		}

		// 恢复属性
		if preserveAttrs {
			if err := os.Chmod(targetPath, mode); err == nil {
				os.Chtimes(targetPath, modTime, modTime)
			}
		}

		return nil

	case ActionRemove:
		// 直接删除
		return os.Remove(targetPath)

	default:
		return fmt.Errorf("未知的去重操作: %s", action)
	}
}

// ========== 去重统计 ==========

// StatsCollector 统计收集器
type StatsCollector struct {
	mu    sync.RWMutex
	stats DedupStats
}

// NewStatsCollector 创建统计收集器
func NewStatsCollector() *StatsCollector {
	return &StatsCollector{}
}

// GetStats 获取统计
func (s *StatsCollector) GetStats() DedupStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// IncrementFiles 增加文件计数
func (s *StatsCollector) IncrementFiles(delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.TotalFiles += delta
}

// IncrementSize 增加大小
func (s *StatsCollector) IncrementSize(delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.TotalSize += delta
}

// AddSavings 添加节省空间
func (s *StatsCollector) AddSavings(delta int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.SavingsActual += delta
}

// RecordScan 记录扫描时间
func (s *StatsCollector) RecordScan(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.LastScanTime = time.Now()
	s.stats.TotalScanTime += d
}

// RecordDedup 记录去重时间
func (s *StatsCollector) RecordDedup() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats.LastDedupTime = time.Now()
}

// Reset 重置统计
func (s *StatsCollector) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stats = DedupStats{}
}

// ToJSON 转换为 JSON
func (s *StatsCollector) ToJSON() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, _ := json.MarshalIndent(s.stats, "", "  ")
	return string(data)
}

// ========== 辅助函数 ==========

// extractUserFromPath 从路径提取用户
func extractUserFromPath(path string) string {
	patterns := []string{
		"/home/",
		"/Users/",
		"/data/users/",
	}

	for _, pattern := range patterns {
		idx := strings.Index(path, pattern)
		if idx != -1 {
			afterPattern := path[idx+len(pattern):]
			parts := strings.SplitN(afterPattern, "/", 2)
			if len(parts) > 0 && parts[0] != "" {
				return parts[0]
			}
		}
	}

	return ""
}

// atomicCounter 原子计数器
type atomicCounter struct {
	value int64
}

func (c *atomicCounter) Add(delta int64) int64 {
	return atomic.AddInt64(&c.value, delta)
}

func (c *atomicCounter) Get() int64 {
	return atomic.LoadInt64(&c.value)
}

func (c *atomicCounter) Reset() {
	atomic.StoreInt64(&c.value, 0)
}
