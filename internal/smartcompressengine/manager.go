package smartcompressengine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// NewManager 创建智能压缩引擎管理器.
func NewManager(cfg *EngineConfig) (*Manager, error) {
	if cfg == nil {
		cfg = &EngineConfig{
			DefaultAlgorithm: AlgorithmZstd,
			DefaultLevel:     LevelBalanced,
			MaxConcurrency:   4,
			EnableAI:         true,
			MinFileSize:      1024,
			SkipEncrypted:    true,
		}
	}

	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 4
	}

	m := &Manager{
		config:     *cfg,
		tasks:      make(map[string]*CompressTask),
		stopChan:   make(chan struct{}),
		workerPool: make(chan struct{}, cfg.MaxConcurrency),
	}

	if cfg.DataDir != "" {
		if err := os.MkdirAll(cfg.DataDir, 0750); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}

	return m, nil
}

// Start 启动管理器.
func (m *Manager) Start() {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopChan)
}

// AnalyzeFile 分析文件并推荐压缩算法.
func (m *Manager) AnalyzeFile(ctx context.Context, filePath string) (*FileAnalysis, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	analysis := &FileAnalysis{
		FilePath: filePath,
		FileSize: info.Size(),
	}

	// 检测文件类型
	analysis.FileType = m.detectFileType(filePath)

	// 计算熵值
	analysis.Entropy, err = m.calculateEntropy(filePath)
	if err != nil {
		return nil, fmt.Errorf("计算熵值失败: %w", err)
	}

	// 判断是否可压缩
	analysis.Compressible = analysis.Entropy < 7.0 && info.Size() >= m.config.MinFileSize

	// AI推荐算法
	if m.config.EnableAI && analysis.Compressible {
		analysis.Recommended = m.recommendAlgorithm(analysis)
		analysis.EstRatio = m.estimateRatio(analysis)
	} else {
		analysis.Recommended = m.config.DefaultAlgorithm
		analysis.EstRatio = 1.0
	}

	return analysis, nil
}

// CompressFile 压缩文件.
func (m *Manager) CompressFile(ctx context.Context, sourcePath, destPath string, algorithm CompressionAlgorithm, level CompressionLevel) (*CompressTask, error) {
	task := &CompressTask{
		ID:         fmt.Sprintf("compress_%d", time.Now().UnixNano()),
		SourcePath: sourcePath,
		DestPath:   destPath,
		Algorithm:  algorithm,
		Level:      level,
		Status:     StatusPending,
		StartTime:  time.Now(),
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 异步执行压缩
	go m.executeCompression(ctx, task)

	return task, nil
}

// GetTask 获取任务状态.
func (m *Manager) GetTask(taskID string) (*CompressTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}
	return task, nil
}

// GetStats 获取压缩统计.
func (m *Manager) GetStats() CompressionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// ListTasks 列出所有任务.
func (m *Manager) ListTasks() []*CompressTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*CompressTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// CompressDirectory 压缩整个目录.
func (m *Manager) CompressDirectory(ctx context.Context, dirPath string, recursive bool) ([]*CompressTask, error) {
	var tasks []*CompressTask

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && !recursive && path != dirPath {
			return filepath.SkipDir
		}

		if info.IsDir() {
			return nil
		}

		// 分析文件
		analysis, err := m.AnalyzeFile(ctx, path)
		if err != nil {
			return err
		}

		if !analysis.Compressible {
			return nil
		}

		// 生成目标路径
		destPath := path + "." + string(analysis.Recommended)

		task, err := m.CompressFile(ctx, path, destPath, analysis.Recommended, m.config.DefaultLevel)
		if err != nil {
			return err
		}

		tasks = append(tasks, task)
		return nil
	})

	if err != nil {
		return nil, err
	}

	return tasks, nil
}

// 内部方法

func (m *Manager) detectFileType(filePath string) FileType {
	ext := filepath.Ext(filePath)
	switch ext {
	case ".txt", ".log", ".csv", ".json", ".xml", ".yaml", ".yml", ".toml", ".ini", ".conf":
		return FileTypeText
	case ".mp4", ".avi", ".mkv", ".mov", ".mp3", ".wav", ".flac", ".jpg", ".jpeg", ".png", ".gif", ".webp":
		return FileTypeMedia
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar":
		return FileTypeArchive
	case ".db", ".sqlite", ".mdb":
		return FileTypeDatabase
	case ".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb", ".php":
		return FileTypeCode
	case ".doc", ".docx", ".pdf", ".md", ".rtf":
		return FileTypeDoc
	default:
		return FileTypeBinary
	}
}

func (m *Manager) calculateEntropy(filePath string) (float64, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}

	if len(data) == 0 {
		return 0, nil
	}

	// 计算字节频率
	var freq [256]int64
	for _, b := range data {
		freq[b]++
	}

	// 计算熵
	var entropy float64
	size := float64(len(data))
	for _, f := range freq {
		if f > 0 {
			p := float64(f) / size
			entropy -= p * log2(p)
		}
	}

	return entropy, nil
}

func log2(x float64) float64 {
	// 简单的log2实现
	if x <= 0 {
		return 0
	}
	result := 0.0
	for x < 0.5 {
		x *= 2
		result--
	}
	for x > 1 {
		x /= 2
		result++
	}
	return result
}

func (m *Manager) recommendAlgorithm(analysis *FileAnalysis) CompressionAlgorithm {
	switch analysis.FileType {
	case FileTypeText, FileTypeCode, FileTypeDoc:
		if analysis.Entropy < 4.0 {
			return AlgorithmBrotli
		}
		return AlgorithmZstd
	case FileTypeLog:
		return AlgorithmZstd
	case FileTypeDatabase:
		return AlgorithmLZ4
	case FileTypeMedia:
		return AlgorithmSnappy
	case FileTypeBinary:
		if analysis.FileSize > 100*1024*1024 {
			return AlgorithmLZ4
		}
		return AlgorithmZstd
	default:
		return m.config.DefaultAlgorithm
	}
}

func (m *Manager) estimateRatio(analysis *FileAnalysis) float64 {
	switch analysis.FileType {
	case FileTypeText, FileTypeCode, FileTypeDoc:
		if analysis.Entropy < 4.0 {
			return 0.3
		}
		return 0.5
	case FileTypeLog:
		return 0.2
	case FileTypeDatabase:
		return 0.6
	case FileTypeMedia:
		return 0.95
	case FileTypeBinary:
		return 0.7
	default:
		return 0.8
	}
}

func (m *Manager) executeCompression(ctx context.Context, task *CompressTask) {
	m.mu.Lock()
	task.Status = StatusRunning
	m.mu.Unlock()

	// 模拟压缩过程
	select {
	case <-ctx.Done():
		task.Status = StatusFailed
		task.Error = "任务被取消"
		return
	case <-time.After(time.Duration(100+task.Level*50) * time.Millisecond):
		// 压缩完成
	}

	// 获取原始文件大小
	info, err := os.Stat(task.SourcePath)
	if err != nil {
		task.Status = StatusFailed
		task.Error = fmt.Sprintf("获取文件信息失败: %v", err)
		return
	}

	task.OriginalSize = info.Size()
	task.CompressedSize = int64(float64(task.OriginalSize) * 0.5) // 模拟压缩比
	task.Ratio = float64(task.CompressedSize) / float64(task.OriginalSize)
	task.Status = StatusCompleted
	task.EndTime = time.Now()

	// 更新统计
	m.mu.Lock()
	m.stats.TotalTasks++
	m.stats.CompletedTasks++
	m.stats.TotalOriginal += task.OriginalSize
	m.stats.TotalCompressed += task.CompressedSize
	if m.stats.TotalOriginal > 0 {
		m.stats.AvgRatio = float64(m.stats.TotalCompressed) / float64(m.stats.TotalOriginal)
	}
	m.mu.Unlock()
}
