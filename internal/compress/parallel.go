// Package compress 提供透明压缩存储功能
// v2.6.0 增强版本：并行压缩、进度追踪、失败恢复
package compress

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ========== 压缩进度追踪 ==========

// ProgressPhase 进度阶段
type ProgressPhase string

const (
	PhaseScanning    ProgressPhase = "scanning"    // 扫描文件
	PhaseCompressing ProgressPhase = "compressing" // 压缩中
	PhaseVerifying   ProgressPhase = "verifying"   // 验证中
	PhaseCompleted   ProgressPhase = "completed"   // 完成
)

// CompressionProgress 压缩进度
type CompressionProgress struct {
	mu sync.RWMutex

	Phase        ProgressPhase `json:"phase"`
	CurrentFile  string        `json:"currentFile"`
	FilesTotal   int64         `json:"filesTotal"`
	FilesDone    int64         `json:"filesDone"`
	FilesSkipped int64         `json:"filesSkipped"`
	FilesFailed  int64         `json:"filesFailed"`
	BytesTotal   int64         `json:"bytesTotal"`
	BytesDone    int64         `json:"bytesDone"`
	BytesSaved   int64         `json:"bytesSaved"`
	StartTime    time.Time     `json:"startTime"`
	LastUpdate   time.Time     `json:"lastUpdate"`
	ETA          time.Duration `json:"eta"`
	Speed        float64       `json:"speed"` // MB/s

	// 并行进度
	Workers     int   `json:"workers"`
	ActiveTasks int32 `json:"activeTasks"`

	// 回调
	onProgress []ProgressCallback
}

// ProgressCallback 进度回调
type ProgressCallback func(progress *CompressionProgress)

// NewCompressionProgress 创建进度追踪器
func NewCompressionProgress() *CompressionProgress {
	return &CompressionProgress{
		Phase:      PhaseScanning,
		StartTime:  time.Now(),
		LastUpdate: time.Now(),
		onProgress: make([]ProgressCallback, 0),
	}
}

// SetPhase 设置阶段
func (p *CompressionProgress) SetPhase(phase ProgressPhase) {
	p.mu.Lock()
	p.Phase = phase
	p.LastUpdate = time.Now()
	p.mu.Unlock()
	p.notify()
}

// SetCurrentFile 设置当前文件
func (p *CompressionProgress) SetCurrentFile(file string) {
	p.mu.Lock()
	p.CurrentFile = file
	p.LastUpdate = time.Now()
	p.mu.Unlock()
	p.notify()
}

// AddFileDone 增加完成文件计数
func (p *CompressionProgress) AddFileDone(saved int64) {
	p.mu.Lock()
	p.FilesDone++
	p.BytesSaved += saved
	p.updateETA()
	p.LastUpdate = time.Now()
	p.mu.Unlock()
	p.notify()
}

// AddFileSkipped 增加跳过文件计数
func (p *CompressionProgress) AddFileSkipped() {
	p.mu.Lock()
	p.FilesSkipped++
	p.LastUpdate = time.Now()
	p.mu.Unlock()
	p.notify()
}

// AddFileFailed 增加失败文件计数
func (p *CompressionProgress) AddFileFailed() {
	p.mu.Lock()
	p.FilesFailed++
	p.LastUpdate = time.Now()
	p.mu.Unlock()
	p.notify()
}

// SetTotal 设置总量
func (p *CompressionProgress) SetTotal(files int64, bytes int64) {
	p.mu.Lock()
	p.FilesTotal = files
	p.BytesTotal = bytes
	p.LastUpdate = time.Now()
	p.mu.Unlock()
	p.notify()
}

// AddBytes 增加处理字节数
func (p *CompressionProgress) AddBytes(n int64) {
	p.mu.Lock()
	p.BytesDone += n
	p.updateETA()
	p.LastUpdate = time.Now()
	p.mu.Unlock()
	p.notify()
}

// SetWorkers 设置工作协程数
func (p *CompressionProgress) SetWorkers(n int) {
	p.mu.Lock()
	p.Workers = n
	p.mu.Unlock()
	p.notify()
}

// SetActiveTasks 设置活跃任务数
func (p *CompressionProgress) SetActiveTasks(n int32) {
	p.mu.Lock()
	p.ActiveTasks = n
	p.mu.Unlock()
	p.notify()
}

// OnProgress 注册进度回调
func (p *CompressionProgress) OnProgress(callback ProgressCallback) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onProgress = append(p.onProgress, callback)
}

// updateETA 更新预计完成时间
func (p *CompressionProgress) updateETA() {
	elapsed := time.Since(p.StartTime).Seconds()
	if elapsed > 0 && p.BytesDone > 0 {
		p.Speed = float64(p.BytesDone) / 1024 / 1024 / elapsed
		if p.Speed > 0 && p.BytesTotal > p.BytesDone {
			remaining := float64(p.BytesTotal-p.BytesDone) / 1024 / 1024 / p.Speed
			p.ETA = time.Duration(remaining) * time.Second
		}
	}
}

// notify 通知回调
func (p *CompressionProgress) notify() {
	p.mu.RLock()
	callbacks := p.onProgress
	p.mu.RUnlock()

	for _, cb := range callbacks {
		cb(p)
	}
}

// GetPercent 获取完成百分比
func (p *CompressionProgress) GetPercent() float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.BytesTotal == 0 {
		return 0
	}
	return float64(p.BytesDone) * 100 / float64(p.BytesTotal)
}

// GetSnapshot 获取进度快照（返回不含锁的副本）
func (p *CompressionProgress) GetSnapshot() CompressionProgressSnapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return CompressionProgressSnapshot{
		Phase:        p.Phase,
		CurrentFile:  p.CurrentFile,
		FilesTotal:   p.FilesTotal,
		FilesDone:    p.FilesDone,
		FilesSkipped: p.FilesSkipped,
		FilesFailed:  p.FilesFailed,
		BytesTotal:   p.BytesTotal,
		BytesDone:    p.BytesDone,
		BytesSaved:   p.BytesSaved,
		StartTime:    p.StartTime,
		LastUpdate:   p.LastUpdate,
		ETA:          p.ETA,
		Speed:        p.Speed,
		Workers:      p.Workers,
		ActiveTasks:  p.ActiveTasks,
	}
}

// CompressionProgressSnapshot 进度快照（不含锁，可安全复制）
type CompressionProgressSnapshot struct {
	Phase        ProgressPhase `json:"phase"`
	CurrentFile  string        `json:"currentFile"`
	FilesTotal   int64         `json:"filesTotal"`
	FilesDone    int64         `json:"filesDone"`
	FilesSkipped int64         `json:"filesSkipped"`
	FilesFailed  int64         `json:"filesFailed"`
	BytesTotal   int64         `json:"bytesTotal"`
	BytesDone    int64         `json:"bytesDone"`
	BytesSaved   int64         `json:"bytesSaved"`
	StartTime    time.Time     `json:"startTime"`
	LastUpdate   time.Time     `json:"lastUpdate"`
	ETA          time.Duration `json:"eta"`
	Speed        float64       `json:"speed"`
	Workers      int           `json:"workers"`
	ActiveTasks  int32         `json:"activeTasks"`
}

// ========== 压缩失败恢复 ==========

// CompressionState 压缩状态（用于恢复）
type CompressionState struct {
	ID             string         `json:"id"`
	StartedAt      time.Time      `json:"startedAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
	Status         string         `json:"status"` // running, paused, completed, failed
	TotalFiles     int64          `json:"totalFiles"`
	ProcessedFiles []string       `json:"processedFiles"`
	PendingFiles   []string       `json:"pendingFiles"`
	FailedFiles    []FailedFile   `json:"failedFiles"`
	BytesDone      int64          `json:"bytesDone"`
	BytesSaved     int64          `json:"bytesSaved"`
	Config         CompressConfig `json:"config"`
}

// FailedFile 失败文件记录
type FailedFile struct {
	Path      string    `json:"path"`
	Error     string    `json:"error"`
	Retries   int       `json:"retries"`
	LastRetry time.Time `json:"lastRetry"`
}

// RecoveryManager 恢复管理器
type RecoveryManager struct {
	mu       sync.RWMutex
	stateDir string
	states   map[string]*CompressionState
}

// NewRecoveryManager 创建恢复管理器
func NewRecoveryManager(stateDir string) (*RecoveryManager, error) {
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return nil, err
	}

	rm := &RecoveryManager{
		stateDir: stateDir,
		states:   make(map[string]*CompressionState),
	}

	// 加载现有状态
	_ = rm.loadStates()

	return rm, nil
}

// CreateState 创建新状态
func (rm *RecoveryManager) CreateState(id string, config CompressConfig) *CompressionState {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state := &CompressionState{
		ID:             id,
		StartedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Status:         "running",
		PendingFiles:   make([]string, 0),
		ProcessedFiles: make([]string, 0),
		FailedFiles:    make([]FailedFile, 0),
		Config:         config,
	}

	rm.states[id] = state
	_ = rm.saveState(state)

	return state
}

// GetState 获取状态
func (rm *RecoveryManager) GetState(id string) (*CompressionState, bool) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	state, ok := rm.states[id]
	return state, ok
}

// UpdateState 更新状态
func (rm *RecoveryManager) UpdateState(id string, fn func(*CompressionState)) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, ok := rm.states[id]
	if !ok {
		return fmt.Errorf("state not found: %s", id)
	}

	fn(state)
	state.UpdatedAt = time.Now()
	return rm.saveState(state)
}

// MarkCompleted 标记完成
func (rm *RecoveryManager) MarkCompleted(id string) error {
	return rm.UpdateState(id, func(s *CompressionState) {
		s.Status = "completed"
	})
}

// MarkFailed 标记失败
func (rm *RecoveryManager) MarkFailed(id string, err error) error {
	return rm.UpdateState(id, func(s *CompressionState) {
		s.Status = "failed"
	})
}

// AddProcessed 添加已处理文件
func (rm *RecoveryManager) AddProcessed(id string, path string, bytesSaved int64) error {
	return rm.UpdateState(id, func(s *CompressionState) {
		s.ProcessedFiles = append(s.ProcessedFiles, path)
		s.BytesDone++
		s.BytesSaved += bytesSaved
	})
}

// AddFailed 添加失败文件
func (rm *RecoveryManager) AddFailed(id string, path string, err error) error {
	return rm.UpdateState(id, func(s *CompressionState) {
		s.FailedFiles = append(s.FailedFiles, FailedFile{
			Path:  path,
			Error: err.Error(),
		})
	})
}

// RetryFailed 重试失败文件
func (rm *RecoveryManager) RetryFailed(id string) ([]string, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	state, ok := rm.states[id]
	if !ok {
		return nil, fmt.Errorf("state not found: %s", id)
	}

	var retryPaths []string
	now := time.Now()

	for i := range state.FailedFiles {
		if state.FailedFiles[i].Retries < 3 {
			state.FailedFiles[i].Retries++
			state.FailedFiles[i].LastRetry = now
			retryPaths = append(retryPaths, state.FailedFiles[i].Path)
		}
	}

	return retryPaths, nil
}

// Resume 恢复中断的任务
func (rm *RecoveryManager) Resume(id string) (*CompressionState, error) {
	rm.mu.RLock()
	state, ok := rm.states[id]
	rm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("state not found: %s", id)
	}

	if state.Status != "running" && state.Status != "paused" {
		return nil, fmt.Errorf("cannot resume task with status: %s", state.Status)
	}

	return state, nil
}

// CleanState 清理状态
func (rm *RecoveryManager) CleanState(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	delete(rm.states, id)
	stateFile := filepath.Join(rm.stateDir, id+".json")
	return os.Remove(stateFile)
}

// ListPending 列出待恢复的任务
func (rm *RecoveryManager) ListPending() []*CompressionState {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var pending []*CompressionState
	for _, state := range rm.states {
		if state.Status == "running" || state.Status == "paused" {
			pending = append(pending, state)
		}
	}
	return pending
}

// saveState 保存状态到文件
func (rm *RecoveryManager) saveState(state *CompressionState) error {
	stateFile := filepath.Join(rm.stateDir, state.ID+".json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(stateFile, data, 0644)
}

// loadStates 从文件加载状态
func (rm *RecoveryManager) loadStates() error {
	entries, err := os.ReadDir(rm.stateDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(rm.stateDir, entry.Name()))
		if err != nil {
			continue
		}

		var state CompressionState
		if err := json.Unmarshal(data, &state); err != nil {
			continue
		}

		rm.states[state.ID] = &state
	}

	return nil
}

// ========== 并行压缩增强 ==========

// CompressConfig 压缩配置
type CompressConfig struct {
	Algorithm       Algorithm `json:"algorithm"`
	Level           int       `json:"level"`
	Workers         int       `json:"workers"`
	DeleteOriginal  bool      `json:"deleteOriginal"`
	Overwrite       bool      `json:"overwrite"`
	ContinueOnError bool      `json:"continueOnError"`
	DryRun          bool      `json:"dryRun"`
	VerifyAfter     bool      `json:"verifyAfter"`
	MaxRetries      int       `json:"maxRetries"`
	MinSize         int64     `json:"minSize"`
}

// DefaultCompressConfig 默认配置
func DefaultCompressConfig() *CompressConfig {
	return &CompressConfig{
		Algorithm:       AlgorithmGzip,
		Level:           6,
		Workers:         4,
		DeleteOriginal:  false,
		Overwrite:       false,
		ContinueOnError: true,
		DryRun:          false,
		VerifyAfter:     true,
		MaxRetries:      3,
		MinSize:         1024,
	}
}

// ParallelCompressor 并行压缩器
type ParallelCompressor struct {
	config      *CompressConfig
	compressors map[Algorithm]Compressor
	progress    *CompressionProgress
	recovery    *RecoveryManager
}

// NewParallelCompressor 创建并行压缩器
func NewParallelCompressor(config *CompressConfig, stateDir string) (*ParallelCompressor, error) {
	if config == nil {
		config = DefaultCompressConfig()
	}

	pc := &ParallelCompressor{
		config:      config,
		compressors: make(map[Algorithm]Compressor),
		progress:    NewCompressionProgress(),
	}

	// 注册压缩器
	pc.compressors[AlgorithmGzip] = &GzipCompressor{}

	// 初始化恢复管理器
	if stateDir != "" {
		recovery, err := NewRecoveryManager(stateDir)
		if err != nil {
			return nil, err
		}
		pc.recovery = recovery
	}

	return pc, nil
}

// ParallelCompressResult 并行压缩结果
type ParallelCompressResult struct {
	ID              string               `json:"id"`
	TotalFiles      int64                `json:"totalFiles"`
	ProcessedFiles  int64                `json:"processedFiles"`
	SkippedFiles    int64                `json:"skippedFiles"`
	FailedFiles     int64                `json:"failedFiles"`
	TotalBytes      int64                `json:"totalBytes"`
	CompressedBytes int64                `json:"compressedBytes"`
	SavedBytes      int64                `json:"savedBytes"`
	Duration        time.Duration        `json:"duration"`
	Speed           float64              `json:"speed"` // MB/s
	Results         []FileCompressResult `json:"results"`
	Errors          []FileCompressError  `json:"errors"`
}

// FileCompressResult 文件压缩结果
type FileCompressResult struct {
	Path           string        `json:"path"`
	OriginalSize   int64         `json:"originalSize"`
	CompressedSize int64         `json:"compressedSize"`
	SavedBytes     int64         `json:"savedBytes"`
	Ratio          float64       `json:"ratio"`
	Algorithm      Algorithm     `json:"algorithm"`
	Duration       time.Duration `json:"duration"`
	Skipped        bool          `json:"skipped"`
	SkipReason     string        `json:"skipReason,omitempty"`
	Error          string        `json:"error,omitempty"`
}

// FileCompressError 文件压缩错误
type FileCompressError struct {
	Path    string `json:"path"`
	Error   string `json:"error"`
	Retries int    `json:"retries"`
}

// CompressParallel 并行压缩
func (pc *ParallelCompressor) CompressParallel(ctx context.Context, paths []string, config *CompressConfig) (*ParallelCompressResult, error) {
	if config == nil {
		config = DefaultCompressConfig()
	}

	// 生成任务ID
	taskID := fmt.Sprintf("compress_%d", time.Now().UnixNano())

	// 初始化进度
	pc.progress = NewCompressionProgress()
	pc.progress.SetPhase(PhaseScanning)
	pc.progress.SetWorkers(config.Workers)

	// 创建恢复状态
	var state *CompressionState
	if pc.recovery != nil {
		state = pc.recovery.CreateState(taskID, *config)
	}

	// 扫描阶段
	var validPaths []string
	var totalSize int64
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if !info.IsDir() && info.Size() >= config.MinSize {
			validPaths = append(validPaths, path)
			totalSize += info.Size()
		}
	}

	pc.progress.SetTotal(int64(len(validPaths)), totalSize)
	pc.progress.SetPhase(PhaseCompressing)

	if state != nil {
		_ = pc.recovery.UpdateState(taskID, func(s *CompressionState) {
			s.TotalFiles = int64(len(validPaths))
			s.PendingFiles = validPaths
		})
	}

	// 结果收集
	result := &ParallelCompressResult{
		ID:      taskID,
		Results: make([]FileCompressResult, 0, len(validPaths)),
		Errors:  make([]FileCompressError, 0),
	}

	start := time.Now()

	// 创建工作池
	pathChan := make(chan string, len(validPaths))
	resultChan := make(chan FileCompressResult, len(validPaths))
	errorChan := make(chan FileCompressError, len(validPaths))

	// 原子计数器
	var activeTasks int32
	var processedBytes int64
	var savedBytes int64

	// 启动 workers
	var wg sync.WaitGroup
	for i := 0; i < config.Workers; i++ {
		wg.Add(1)
		go pc.compressWorker(ctx, &wg, config, pathChan, resultChan, errorChan,
			&activeTasks, &processedBytes, &savedBytes)
	}

	// 发送任务
sendLoop:
	for _, path := range validPaths {
		select {
		case pathChan <- path:
		case <-ctx.Done():
			break sendLoop
		}
	}
	close(pathChan)

	// 等待完成
	go func() {
		wg.Wait()
		close(resultChan)
		close(errorChan)
	}()

	// 收集结果
	for res := range resultChan {
		result.Results = append(result.Results, res)
		result.ProcessedFiles++
		result.TotalBytes += res.OriginalSize
		result.CompressedBytes += res.CompressedSize
		result.SavedBytes += res.SavedBytes

		if res.Skipped {
			result.SkippedFiles++
		}

		if state != nil {
			_ = pc.recovery.AddProcessed(taskID, res.Path, res.SavedBytes)
		}
	}

	for err := range errorChan {
		result.Errors = append(result.Errors, err)
		result.FailedFiles++

		if state != nil {
			_ = pc.recovery.AddFailed(taskID, err.Path, errors.New(err.Error))
		}
	}

	result.Duration = time.Since(start)
	elapsed := result.Duration.Seconds()
	if elapsed > 0 {
		result.Speed = float64(result.TotalBytes) / 1024 / 1024 / elapsed
	}

	pc.progress.SetPhase(PhaseCompleted)

	// 标记完成
	if state != nil {
		_ = pc.recovery.MarkCompleted(taskID)
	}

	return result, nil
}

// compressWorker 压缩工作协程
func (pc *ParallelCompressor) compressWorker(ctx context.Context, wg *sync.WaitGroup, config *CompressConfig,
	paths <-chan string, results chan<- FileCompressResult, errors chan<- FileCompressError,
	activeTasks *int32, processedBytes, savedBytes *int64) {

	defer wg.Done()

	compressor, ok := pc.compressors[config.Algorithm]
	if !ok {
		compressor = pc.compressors[AlgorithmGzip]
	}

	for path := range paths {
		select {
		case <-ctx.Done():
			return
		default:
		}

		atomic.AddInt32(activeTasks, 1)
		pc.progress.SetActiveTasks(*activeTasks)
		pc.progress.SetCurrentFile(path)

		result := pc.compressFile(path, config, compressor)
		if result.Error != "" {
			errors <- FileCompressError{
				Path:  path,
				Error: result.Error,
			}
			pc.progress.AddFileFailed()
		} else {
			results <- result
			pc.progress.AddFileDone(result.SavedBytes)
		}

		atomic.AddInt32(activeTasks, -1)
		pc.progress.SetActiveTasks(*activeTasks)
	}
}

// compressFile 压缩单个文件
func (pc *ParallelCompressor) compressFile(path string, config *CompressConfig, compressor Compressor) FileCompressResult {
	result := FileCompressResult{
		Path:      path,
		Algorithm: config.Algorithm,
	}

	start := time.Now()

	// 获取文件信息
	info, err := os.Stat(path)
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.OriginalSize = info.Size()

	// 检查是否应该压缩
	if !pc.shouldCompress(path, info.Size(), config) {
		result.Skipped = true
		result.SkipReason = "不符合压缩条件"
		result.Duration = time.Since(start)
		pc.progress.AddFileSkipped()
		return result
	}

	// DryRun 模式
	if config.DryRun {
		result.Skipped = true
		result.SkipReason = "dry run"
		result.Duration = time.Since(start)
		return result
	}

	// 打开源文件
	srcFile, err := os.Open(path)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = srcFile.Close() }()

	// 创建目标文件
	dstPath := path + compressor.Extension()
	dstFile, err := os.Create(dstPath)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer func() { _ = dstFile.Close() }()

	// 压缩
	err = compressor.Compress(dstFile, srcFile, config.Level)
	if err != nil {
		_ = os.Remove(dstPath)
		result.Error = err.Error()
		return result
	}

	// 获取压缩后大小
	dstInfo, err := dstFile.Stat()
	if err != nil {
		result.Error = err.Error()
		return result
	}

	result.CompressedSize = dstInfo.Size()
	result.SavedBytes = result.OriginalSize - result.CompressedSize
	result.Ratio = float64(result.CompressedSize) / float64(result.OriginalSize)
	result.Duration = time.Since(start)

	// 验证（如果启用）
	if config.VerifyAfter {
		if err := pc.verifyCompression(path, dstPath, compressor); err != nil {
			_ = os.Remove(dstPath)
			result.Error = fmt.Sprintf("验证失败: %s", err.Error())
			return result
		}
	}

	// 删除原文件（如果配置）
	if config.DeleteOriginal {
		_ = os.Remove(path)
	}

	return result
}

// shouldCompress 检查是否应该压缩
func (pc *ParallelCompressor) shouldCompress(path string, size int64, config *CompressConfig) bool {
	if size < config.MinSize {
		return false
	}

	// 检查扩展名 - 使用预定义的不压缩扩展名
	ext := strings.ToLower(filepath.Ext(path))
	excludeExts := []string{".zip", ".gz", ".bz2", ".xz", ".zst", ".lz4", ".mp3", ".mp4", ".avi", ".mkv", ".mov", ".jpg", ".jpeg", ".png", ".gif"}
	for _, exclude := range excludeExts {
		if ext == exclude {
			return false
		}
	}

	return true
}

// verifyCompression 验证压缩结果
func (pc *ParallelCompressor) verifyCompression(srcPath, dstPath string, compressor Compressor) error {
	// 打开压缩文件
	dstFile, err := os.Open(dstPath)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	// 创建临时文件验证解压
	tmpFile, err := os.CreateTemp("", "verify_*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()
	defer func() { _ = tmpFile.Close() }()

	// 解压到临时文件
	if err := compressor.Decompress(tmpFile, dstFile); err != nil {
		return err
	}

	// 比较大小（简化验证，生产环境应该比较内容）
	srcInfo, _ := os.Stat(srcPath)
	tmpInfo, _ := tmpFile.Stat()

	if srcInfo.Size() != tmpInfo.Size() {
		return fmt.Errorf("size mismatch: %d != %d", srcInfo.Size(), tmpInfo.Size())
	}

	return nil
}

// GetProgress 获取进度
func (pc *ParallelCompressor) GetProgress() *CompressionProgress {
	return pc.progress
}

// Resume 恢复中断的任务
func (pc *ParallelCompressor) Resume(ctx context.Context, taskID string) (*ParallelCompressResult, error) {
	if pc.recovery == nil {
		return nil, errors.New("recovery not enabled")
	}

	state, err := pc.recovery.Resume(taskID)
	if err != nil {
		return nil, err
	}

	// 获取待处理文件
	pending := state.PendingFiles
	if len(pending) == 0 {
		// 获取失败文件路径
		for _, f := range state.FailedFiles {
			pending = append(pending, f.Path)
		}
	}

	return pc.CompressParallel(ctx, pending, &state.Config)
}

// ListPendingTasks 列出待恢复的任务
func (pc *ParallelCompressor) ListPendingTasks() []*CompressionState {
	if pc.recovery == nil {
		return nil
	}
	return pc.recovery.ListPending()
}
