package smartdedup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Scanner 文件扫描器。
// 扫描指定目录，收集文件信息并按哈希分组。
type Scanner struct {
	config    *Config
	hasher    *Hasher
	mu        sync.Mutex
	errors    []ScanError
	files     []*FileInfo
	fileChan  chan *FileInfo
	errChan   chan ScanError
}

// NewScanner 创建新的文件扫描器。
func NewScanner(config *Config) *Scanner {
	return &Scanner{
		config: config,
		hasher: NewHasher(0),
	}
}

// Scan 扫描配置中的所有路径，返回扫描结果。
func (s *Scanner) Scan() (*ScanResult, error) {
	if len(s.config.ScanPaths) == 0 {
		return nil, fmt.Errorf("no scan paths configured")
	}

	startTime := time.Now()
	s.mu.Lock()
	s.errors = make([]ScanError, 0)
	s.files = make([]*FileInfo, 0)
	s.mu.Unlock()

	// 收集所有文件
	allFiles := make([]string, 0)
	for _, scanPath := range s.config.ScanPaths {
		if err := s.walkDir(scanPath, &allFiles); err != nil {
			s.addError(scanPath, err.Error())
		}
	}

	// 并发计算哈希
	results := s.computeHashes(allFiles)

	endTime := time.Now()

	// 构建结果
	result := &ScanResult{
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime),
		TotalFiles: len(results),
		Errors:     s.getErrors(),
	}

	// 按内容哈希分组
	hashGroups := make(map[string][]*FileInfo)
	for _, fi := range results {
		hashGroups[fi.ContentHash] = append(hashGroups[fi.ContentHash], fi)
		result.TotalSize += fi.Size
	}

	// 构建重复组
	result.DuplicateGroups = make([]*DuplicateGroup, 0)
	for hash, group := range hashGroups {
		if len(group) < 2 {
			continue
		}
		dg := &DuplicateGroup{
			ContentHash: hash,
			Files:       group,
			TotalSize:   int64(len(group)) * group[0].Size,
			SavedSize:   int64(len(group)-1) * group[0].Size,
		}
		result.DuplicateGroups = append(result.DuplicateGroups, dg)
		result.DuplicateCount += len(group) - 1
		result.DuplicateSize += dg.SavedSize
	}

	// 按感知哈希分组（相似文件检测）
	if s.config.PerceptualEnabled {
		result.SimilarGroups = s.findSimilarGroups(results)
	}

	return result, nil
}

// walkDir 递归遍历目录，收集文件路径。
func (s *Scanner) walkDir(dir string, files *[]string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			s.addError(path, err.Error())
			return nil // 跳过错误文件，继续扫描
		}

		// 跳过目录
		if info.IsDir() {
			if s.shouldExcludeDir(path) {
				return filepath.SkipDir
			}
			return nil
		}

		// 检查是否应排除
		if s.shouldExclude(path) {
			return nil
		}

		// 检查文件大小范围
		if !s.checkFileSize(info.Size()) {
			return nil
		}

		*files = append(*files, path)
		return nil
	})
}

// shouldExcludeDir 检查目录是否应排除。
func (s *Scanner) shouldExcludeDir(dir string) bool {
	dirName := filepath.Base(dir)
	for _, pattern := range s.config.ExcludePatterns {
		if strings.Contains(dirName, pattern) {
			return true
		}
	}
	for _, excludePath := range s.config.ExcludePaths {
		excluded := filepath.Clean(excludePath)
		d := filepath.Clean(dir)
		if d == excluded || strings.HasPrefix(d, excluded+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// shouldExclude 检查文件是否应排除。
func (s *Scanner) shouldExclude(path string) bool {
	base := filepath.Base(path)
	for _, pattern := range s.config.ExcludePatterns {
		if strings.Contains(base, pattern) {
			return true
		}
	}
	for _, excludePath := range s.config.ExcludePaths {
		excluded := filepath.Clean(excludePath)
		p := filepath.Clean(path)
		if p == excluded || strings.HasPrefix(p, excluded+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

// checkFileSize 检查文件大小是否在配置范围内。
func (s *Scanner) checkFileSize(size int64) bool {
	if s.config.MinFileSize > 0 && size < s.config.MinFileSize {
		return false
	}
	if s.config.MaxFileSize > 0 && size > s.config.MaxFileSize {
		return false
	}
	return true
}

// computeHashes 并发计算文件哈希。
func (s *Scanner) computeHashes(files []string) []*FileInfo {
	workers := s.config.MaxWorkers
	if workers <= 0 {
		workers = 4
	}

	fileChan := make(chan string, workers*2)
	resultChan := make(chan *FileInfo, len(files))
	errChan := make(chan ScanError, len(files))
	var wg sync.WaitGroup

	// 启动 worker
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range fileChan {
				fi, err := s.hasher.ComputeFileInfo(path)
				if err != nil {
					errChan <- ScanError{Path: path, Error: err.Error()}
					continue
				}
				resultChan <- fi
			}
		}()
	}

	// 分发任务
	go func() {
		for _, path := range files {
			fileChan <- path
		}
		close(fileChan)
	}()

	// 等待完成
	go func() {
		wg.Wait()
		close(resultChan)
		close(errChan)
	}()

	// 收集结果
	results := make([]*FileInfo, 0, len(files))
	for fi := range resultChan {
		results = append(results, fi)
	}
	for scanErr := range errChan {
		s.addError(scanErr.Path, scanErr.Error)
	}

	return results
}

// findSimilarGroups 查找相似文件组。
func (s *Scanner) findSimilarGroups(files []*FileInfo) []*SimilarGroup {
	// 按感知哈希分组
	hashGroups := make(map[string][]*FileInfo)
	for _, fi := range files {
		if fi.PerceptHash == "" {
			continue
		}
		hashGroups[fi.PerceptHash] = append(hashGroups[fi.PerceptHash], fi)
	}

	groups := make([]*SimilarGroup, 0)
	groupID := 0
	for hash, group := range hashGroups {
		if len(group) < 2 {
			continue
		}
		groupID++
		groups = append(groups, &SimilarGroup{
			GroupID:    fmt.Sprintf("sim-%d", groupID),
			HashValue:  hash,
			Files:      group,
			Threshold:  s.config.PerceptThreshold,
			Similarity: 1.0, // 同哈希视为完全相似
		})
	}

	return groups
}

// addError 添加扫描错误。
func (s *Scanner) addError(path, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errors = append(s.errors, ScanError{Path: path, Error: errMsg})
}

// getErrors 获取所有扫描错误。
func (s *Scanner) getErrors() []ScanError {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.errors) == 0 {
		return nil
	}
	errs := make([]ScanError, len(s.errors))
	copy(errs, s.errors)
	return errs
}

// ScanSingle 扫描单个文件。
func (s *Scanner) ScanSingle(filePath string) (*FileInfo, error) {
	return s.hasher.ComputeFileInfo(filePath)
}
