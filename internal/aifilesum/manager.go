// Package aifilesum 提供AI智能文件摘要生成功能
package aifilesum

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

// Manager AI文件摘要管理器
type Manager struct {
	mu         sync.RWMutex
	config     *SummarizerConfig
	summarizer *Summarizer
	tasks      map[string]*SummarizeTask
	cache      map[string]*CacheEntry
	index      map[string]*IndexEntry
	stats      *Stats
}

// NewManager 创建管理器
func NewManager(config *SummarizerConfig) *Manager {
	if config == nil {
		config = &SummarizerConfig{
			AIEndpoint:            "http://localhost:11434",
			AIModel:               "llama3.2",
			MaxConcurrent:         3,
			MaxQueueSize:          100,
			CacheTTL:              3600,
			SupportedLanguages:    []Language{LanguageAuto, LanguageChinese, LanguageEnglish},
			MaxFileSizeMB:         100,
			VideoFrameIntervalSec: 10,
		}
	}

	return &Manager{
		config:     config,
		summarizer: NewSummarizer(config),
		tasks:      make(map[string]*SummarizeTask),
		cache:      make(map[string]*CacheEntry),
		index:      make(map[string]*IndexEntry),
		stats: &Stats{
			LastUpdated: time.Now(),
		},
	}
}

// GetSummarizer 获取摘要引擎
func (m *Manager) GetSummarizer() *Summarizer {
	return m.summarizer
}

// SummarizeFile 单文件摘要
func (m *Manager) SummarizeFile(ctx context.Context, filePath string, opts *SummarizeOptions) (*Summary, error) {
	// 验证文件存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, ErrFileNotFound
	}

	// 检查缓存
	if opts != nil && opts.CacheResults {
		if cached := m.getFromCache(filePath); cached != nil {
			m.updateStats(func(s *Stats) { s.CacheHits++ })
			return cached, nil
		}
		m.updateStats(func(s *Stats) { s.CacheMisses++ })
	}

	// 获取文件类型
	ext := getFileExt(filePath)
	ft := classifyFileType(ext)

	if ft == FileTypeUnknown {
		return nil, ErrUnsupportedFormat
	}

	// 默认选项
	if opts == nil {
		opts = &SummarizeOptions{
			Language:            LanguageAuto,
			MaxSummaryLength:    200,
			ExtractKeywords:     true,
			ExtractTags:         true,
			GenerateDescription: true,
			ExtractKeyFrames:    true,
			CacheResults:        true,
		}
	}

	var summary *Summary
	var err error

	switch ft {
	case FileTypeDocument:
		// 读取文件内容
		content, readErr := readFileContent(filePath)
		if readErr != nil {
			return nil, fmt.Errorf("读取文件失败: %w", readErr)
		}
		summary, err = m.summarizer.SummarizeDocument(ctx, filePath, content, opts)

	case FileTypeImage:
		summary, err = m.summarizer.SummarizeImage(ctx, filePath, opts)

	case FileTypeVideo:
		summary, err = m.summarizer.SummarizeVideo(ctx, filePath, opts)

	default:
		return nil, ErrUnsupportedFormat
	}

	if err != nil {
		return nil, err
	}

	// 缓存结果
	if opts.CacheResults {
		m.addToCache(filePath, summary)
	}

	// 更新索引
	m.addToIndex(filePath, summary)

	// 更新统计
	m.updateStats(func(s *Stats) {
		s.TotalSummaries++
		s.TotalFiles++
		switch ft {
		case FileTypeDocument:
			s.DocumentSummaries++
		case FileTypeImage:
			s.ImageSummaries++
		case FileTypeVideo:
			s.VideoSummaries++
		}
		// 更新平均处理时间
		total := s.TotalSummaries
		s.AvgProcessingTimeMs = (s.AvgProcessingTimeMs*float64(total-1) + float64(summary.Duration)) / float64(total)
	})

	return summary, nil
}

// CreateBatchTask 创建批量处理任务
func (m *Manager) CreateBatchTask(files []string, opts *SummarizeOptions) (*SummarizeTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查队列是否已满
	if len(m.tasks) >= m.config.MaxQueueSize {
		return nil, ErrQueueFull
	}

	// 验证并收集文件信息
	var fileInfos []FileInfo
	for _, filePath := range files {
		info, err := os.Stat(filePath)
		if err != nil {
			continue // 跳过不存在的文件
		}

		ext := getFileExt(filePath)
		ft := classifyFileType(ext)
		if ft == FileTypeUnknown {
			continue // 跳过不支持的格式
		}

		// 检查文件大小
		if info.Size() > int64(m.config.MaxFileSizeMB)*1024*1024 {
			continue // 跳过过大的文件
		}

		fileInfos = append(fileInfos, FileInfo{
			Path:      filePath,
			Name:      info.Name(),
			Size:      info.Size(),
			Extension: ext,
			FileType:  ft,
			MimeType:  getMimeType(ext),
			ModTime:   info.ModTime(),
		})
	}

	if len(fileInfos) == 0 {
		return nil, ErrFileNotFound
	}

	// 设置默认选项
	if opts == nil {
		opts = &SummarizeOptions{
			Language:            LanguageAuto,
			MaxSummaryLength:    200,
			ExtractKeywords:     true,
			ExtractTags:         true,
			GenerateDescription: true,
			ExtractKeyFrames:    true,
			CacheResults:        true,
		}
	}

	task := &SummarizeTask{
		ID:         fmt.Sprintf("task_%d", time.Now().UnixNano()),
		Status:     TaskStatusPending,
		Files:      fileInfos,
		Options:    opts,
		TotalFiles: len(fileInfos),
		Results:    make([]*Summary, 0, len(fileInfos)),
		StartedAt:  time.Now(),
	}

	m.tasks[task.ID] = task
	return task, nil
}

// RunBatchTask 执行批量处理任务
func (m *Manager) RunBatchTask(ctx context.Context, taskID string) error {
	m.mu.Lock()
	task, exists := m.tasks[taskID]
	if !exists {
		m.mu.Unlock()
		return ErrTaskNotFound
	}
	if task.Status == TaskStatusRunning {
		m.mu.Unlock()
		return ErrTaskAlreadyRunning
	}
	task.Status = TaskStatusRunning
	m.mu.Unlock()

	go m.executeBatchTask(ctx, task)
	return nil
}

// executeBatchTask 执行批量任务
func (m *Manager) executeBatchTask(ctx context.Context, task *SummarizeTask) {
	for i, fileInfo := range task.Files {
		m.mu.Lock()
		if task.Status == TaskStatusCancelled {
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()

		// 检查缓存
		if task.Options.CacheResults {
			if cached := m.getFromCache(fileInfo.Path); cached != nil {
				m.mu.Lock()
				task.Results = append(task.Results, cached)
				task.ProcessedFiles++
				task.Progress = float64(i+1) / float64(task.TotalFiles) * 100
				m.mu.Unlock()
				continue
			}
		}

		// 处理文件
		summary, err := m.SummarizeFile(ctx, fileInfo.Path, task.Options)
		if err != nil {
			m.mu.Lock()
			task.FailedFiles++
			task.Errors = append(task.Errors, TaskError{
				FilePath: fileInfo.Path,
				Error:    err.Error(),
			})
			m.mu.Unlock()
			log.Printf("⚠️ 文件摘要失败 %s: %v", fileInfo.Path, err)
			continue
		}

		m.mu.Lock()
		task.Results = append(task.Results, summary)
		task.ProcessedFiles++
		task.Progress = float64(i+1) / float64(task.TotalFiles) * 100
		m.mu.Unlock()
	}

	m.mu.Lock()
	now := time.Now()
	task.CompletedAt = &now
	if task.FailedFiles > 0 && task.ProcessedFiles == 0 {
		task.Status = TaskStatusFailed
	} else {
		task.Status = TaskStatusCompleted
	}
	m.mu.Unlock()

	log.Printf("✅ 批量任务 %s 完成: %d/%d 成功", task.ID, task.ProcessedFiles, task.TotalFiles)
}

// GetTask 获取任务
func (m *Manager) GetTask(taskID string) (*SummarizeTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有任务
func (m *Manager) ListTasks() []*SummarizeTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*SummarizeTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// CancelTask 取消任务
func (m *Manager) CancelTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}
	if task.Status != TaskStatusRunning {
		return ErrTaskNotRunning
	}

	task.Status = TaskStatusCancelled
	return nil
}

// GetSummary 获取摘要
func (m *Manager) GetSummary(summaryID string) (*Summary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 从索引中查找
	for _, entry := range m.index {
		if entry.SummaryID == summaryID {
			if cached, ok := m.cache[entry.FilePath]; ok {
				return cached.Summary, nil
			}
		}
	}

	return nil, ErrSummaryNotFound
}

// GetSummaryByFile 根据文件路径获取摘要
func (m *Manager) GetSummaryByFile(filePath string) (*Summary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if cached, ok := m.cache[filePath]; ok {
		if time.Now().Before(cached.ExpiresAt) {
			cached.AccessCount++
			return cached.Summary, nil
		}
		// 缓存已过期
		delete(m.cache, filePath)
	}

	return nil, ErrSummaryNotFound
}

// SearchByTag 按标签搜索
func (m *Manager) SearchByTag(tag string) []*Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Summary
	for _, entry := range m.index {
		for _, t := range entry.Tags {
			if t == tag {
				if cached, ok := m.cache[entry.FilePath]; ok {
					results = append(results, cached.Summary)
				}
				break
			}
		}
	}

	return results
}

// SearchByKeyword 按关键词搜索
func (m *Manager) SearchByKeyword(keyword string) []*Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []*Summary
	for _, entry := range m.index {
		for _, kw := range entry.Keywords {
			if kw == keyword {
				if cached, ok := m.cache[entry.FilePath]; ok {
					results = append(results, cached.Summary)
				}
				break
			}
		}
	}

	return results
}

// GetStats 获取统计信息
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	stats.LastUpdated = time.Now()
	return &stats
}

// ClearCache 清除缓存
func (m *Manager) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache = make(map[string]*CacheEntry)
	m.index = make(map[string]*IndexEntry)
	log.Println("✅ 摘要缓存已清除")
}

// getFromCache 从缓存获取
func (m *Manager) getFromCache(filePath string) *Summary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, exists := m.cache[filePath]
	if !exists {
		return nil
	}

	if time.Now().After(entry.ExpiresAt) {
		delete(m.cache, filePath)
		return nil
	}

	entry.AccessCount++
	return entry.Summary
}

// addToCache 添加到缓存
func (m *Manager) addToCache(filePath string, summary *Summary) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ttl := time.Duration(m.config.CacheTTL) * time.Second
	m.cache[filePath] = &CacheEntry{
		Summary:     summary,
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(ttl),
		AccessCount: 0,
	}
}

// addToIndex 添加到索引
func (m *Manager) addToIndex(filePath string, summary *Summary) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.index[summary.ID] = &IndexEntry{
		FileID:    summary.FileID,
		FilePath:  filePath,
		SummaryID: summary.ID,
		Tags:      summary.Tags,
		Keywords:  summary.Keywords,
		CreatedAt: time.Now(),
	}
}

// updateStats 更新统计信息
func (m *Manager) updateStats(fn func(*Stats)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	fn(m.stats)
}

// getFileExt 获取文件扩展名
func getFileExt(filePath string) string {
	for i := len(filePath) - 1; i >= 0; i-- {
		if filePath[i] == '.' {
			return filePath[i:]
		}
		if filePath[i] == '/' || filePath[i] == '\\' {
			break
		}
	}
	return ""
}

// readFileContent 读取文件内容
func readFileContent(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	// 限制内容长度（避免过大文件）
	maxLen := 50000 // 约50KB文本
	if len(data) > maxLen {
		data = data[:maxLen]
	}

	return string(data), nil
}
