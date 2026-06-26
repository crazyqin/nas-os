// Package backup 块级备份引擎
// 块级别增量备份，去重+压缩，对标群晖 Hyper Backup
package backup

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 常量 ==========

const (
	// BlockSize 默认块大小 (4MB).
	BlockSize = 4 * 1024 * 1024
	// MaxConcurrentBlocks 最大并发块处理数.
	MaxConcurrentBlocks = 8
	// CompressionThreshold 压缩阈值 (1KB以下不压缩).
	CompressionThreshold = 1024
	// DedupIndexFile 去重索引文件名.
	DedupIndexFile = "dedup-index.json"
	// ManifestFile 备份清单文件名.
	ManifestFile = "manifest.json"
)

// ========== 类型 ==========

// BlockBackupStatus 备份任务状态
type BlockBackupStatus string

const (
	BlockStatusPending  BlockBackupStatus = "pending"
	BlockStatusRunning  BlockBackupStatus = "running"
	BlockStatusPaused   BlockBackupStatus = "paused"
	BlockStatusComplete BlockBackupStatus = "complete"
	BlockStatusFailed   BlockBackupStatus = "failed"
	BlockStatusCanceled BlockBackupStatus = "canceled"
)

// BlockBackupJob 块级备份任务
type BlockBackupJob struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	SourcePaths   []string          `json:"source_paths"`
	DestPath      string            `json:"dest_path"`
	Status        BlockBackupStatus `json:"status"`
	BlockSize     int               `json:"block_size"`
	Compression   bool              `json:"compression"`
	Encryption    bool              `json:"encryption"`
	EncryptionKey string            `json:"encryption_key,omitempty"`
	Schedule      string            `json:"schedule,omitempty"` // cron表达式
	MaxVersions   int               `json:"max_versions"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	LastRunAt     *time.Time        `json:"last_run_at,omitempty"`
	NextRunAt     *time.Time        `json:"next_run_at,omitempty"`
}

// BlockBackupRun 备份执行记录
type BlockBackupRun struct {
	ID          string            `json:"id"`
	JobID       string            `json:"job_id"`
	Version     int               `json:"version"`
	Status      BlockBackupStatus `json:"status"`
	TotalFiles  int               `json:"total_files"`
	TotalBlocks int               `json:"total_blocks"`
	TotalBytes  int64             `json:"total_bytes"`
	DoneFiles   int               `json:"done_files"`
	DoneBlocks  int               `json:"done_blocks"`
	DoneBytes   int64             `json:"done_bytes"`
	DedupSaved  int64             `json:"dedup_saved"` // 去重节省的字节
	CompSaved   int64             `json:"comp_saved"`  // 压缩节省的字节
	Error       string            `json:"error,omitempty"`
	StartedAt   time.Time         `json:"started_at"`
	EndedAt     *time.Time        `json:"ended_at,omitempty"`
}

// BlockInfo 块信息
type BlockInfo struct {
	Hash       string `json:"hash"`
	Offset     int64  `json:"offset"`
	Size       int    `json:"size"`
	CompSize   int    `json:"comp_size"`
	Compressed bool   `json:"compressed"`
	RefCount   int    `json:"ref_count"`
}

// DedupEntry 去重索引条目
type DedupEntry struct {
	Hash     string `json:"hash"`
	Size     int    `json:"size"`
	Path     string `json:"path"`
	RefCount int    `json:"ref_count"`
}

// Manifest 备份清单
type Manifest struct {
	JobID     string       `json:"job_id"`
	Version   int          `json:"version"`
	RunID     string       `json:"run_id"`
	Files     []FileRecord `json:"files"`
	Blocks    []BlockInfo  `json:"blocks"`
	TotalSize int64        `json:"total_size"`
	CreatedAt time.Time    `json:"created_at"`
}

// FileRecord 文件记录
type FileRecord struct {
	Path       string    `json:"path"`
	Size       int64     `json:"size"`
	ModTime    time.Time `json:"mod_time"`
	Permission string    `json:"permission"`
	BlockRefs  []string  `json:"block_refs"` // 块哈希列表
	Checksum   string    `json:"checksum"`
}

// BlockBackupEngine 块级备份引擎
type BlockBackupEngine struct {
	mu          sync.RWMutex
	jobs        map[string]*BlockBackupJob
	runs        map[string]*BlockBackupRun
	dedupIndex  map[string]*DedupEntry
	baseDir     string
	blockDir    string
	manifestDir string
	runCtx      context.Context
	runCancel   context.CancelFunc
	progressCh  chan ProgressEvent
}

// ProgressEvent 进度事件
type ProgressEvent struct {
	RunID    string  `json:"run_id"`
	JobID    string  `json:"job_id"`
	Type     string  `json:"type"`
	Progress float64 `json:"progress"`
	Message  string  `json:"message"`
}

// NewBlockBackupEngine 创建块级备份引擎
func NewBlockBackupEngine(baseDir string) (*BlockBackupEngine, error) {
	blockDir := filepath.Join(baseDir, "blocks")
	manifestDir := filepath.Join(baseDir, "manifests")

	for _, dir := range []string{baseDir, blockDir, manifestDir} {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())

	engine := &BlockBackupEngine{
		jobs:        make(map[string]*BlockBackupJob),
		runs:        make(map[string]*BlockBackupRun),
		dedupIndex:  make(map[string]*DedupEntry),
		baseDir:     baseDir,
		blockDir:    blockDir,
		manifestDir: manifestDir,
		runCtx:      ctx,
		runCancel:   cancel,
		progressCh:  make(chan ProgressEvent, 100),
	}

	// 加载已有任务
	if err := engine.loadJobs(); err != nil {
		return nil, err
	}

	// 加载去重索引
	if err := engine.loadDedupIndex(); err != nil {
		return nil, err
	}

	return engine, nil
}

// CreateJob 创建备份任务
func (e *BlockBackupEngine) CreateJob(name string, sourcePaths []string, destPath string, opts ...JobOption) (*BlockBackupJob, error) {
	if name == "" {
		return nil, fmt.Errorf("任务名不能为空")
	}
	if len(sourcePaths) == 0 {
		return nil, fmt.Errorf("源路径不能为空")
	}

	job := &BlockBackupJob{
		ID:          uuid.New().String(),
		Name:        name,
		SourcePaths: sourcePaths,
		DestPath:    destPath,
		Status:      BlockStatusPending,
		BlockSize:   BlockSize,
		Compression: true,
		MaxVersions: 10,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	for _, opt := range opts {
		opt(job)
	}

	e.mu.Lock()
	e.jobs[job.ID] = job
	e.mu.Unlock()

	if err := e.saveJobs(); err != nil {
		return nil, err
	}

	return job, nil
}

// JobOption 任务选项
type JobOption func(*BlockBackupJob)

func WithBlockSize(size int) JobOption {
	return func(j *BlockBackupJob) { j.BlockSize = size }
}

func WithCompression(enabled bool) JobOption {
	return func(j *BlockBackupJob) { j.Compression = enabled }
}

func WithEncryption(key string) JobOption {
	return func(j *BlockBackupJob) {
		j.Encryption = true
		j.EncryptionKey = key
	}
}

func WithSchedule(cron string) JobOption {
	return func(j *BlockBackupJob) { j.Schedule = cron }
}

func WithMaxVersions(n int) JobOption {
	return func(j *BlockBackupJob) { j.MaxVersions = n }
}

// RunBackup 执行备份
func (e *BlockBackupEngine) RunBackup(ctx context.Context, jobID string) (*BlockBackupRun, error) {
	e.mu.Lock()
	job, ok := e.jobs[jobID]
	if !ok {
		e.mu.Unlock()
		return nil, fmt.Errorf("任务 '%s' 不存在", jobID)
	}

	if job.Status == BlockStatusRunning {
		e.mu.Unlock()
		return nil, fmt.Errorf("任务 '%s' 正在运行", jobID)
	}

	run := &BlockBackupRun{
		ID:        uuid.New().String(),
		JobID:     jobID,
		Status:    BlockStatusRunning,
		StartedAt: time.Now(),
	}

	e.runs[run.ID] = run
	job.Status = BlockStatusRunning
	now := time.Now()
	job.LastRunAt = &now
	job.UpdatedAt = now
	e.mu.Unlock()

	// 异步执行备份
	go e.executeBackup(ctx, job, run)

	return run, nil
}

// executeBackup 执行备份逻辑
func (e *BlockBackupEngine) executeBackup(ctx context.Context, job *BlockBackupJob, run *BlockBackupRun) {
	defer func() {
		e.mu.Lock()
		if r := recover(); r != nil {
			run.Status = BlockStatusFailed
			run.Error = fmt.Sprintf("panic: %v", r)
		}
		now := time.Now()
		run.EndedAt = &now
		job.Status = BlockStatusPending
		job.UpdatedAt = now
		e.mu.Unlock()
		e.saveJobs()
	}()

	// 版本号递增
	e.mu.Lock()
	run.Version = e.getNextVersion(job.ID)
	e.mu.Unlock()

	// 创建版本目录
	versionDir := filepath.Join(job.DestPath, fmt.Sprintf("v%04d", run.Version))
	if err := os.MkdirAll(versionDir, 0750); err != nil {
		run.Status = BlockStatusFailed
		run.Error = fmt.Sprintf("创建版本目录失败: %v", err)
		return
	}

	manifest := Manifest{
		JobID:     job.ID,
		Version:   run.Version,
		RunID:     run.ID,
		Files:     make([]FileRecord, 0),
		Blocks:    make([]BlockInfo, 0),
		CreatedAt: time.Now(),
	}

	// 收集所有源文件
	var allFiles []string
	for _, src := range job.SourcePaths {
		filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() {
				allFiles = append(allFiles, path)
			}
			return nil
		})
	}

	run.TotalFiles = len(allFiles)
	e.emitProgress(run, "collect", 0, fmt.Sprintf("收集到 %d 个文件", len(allFiles)))

	// 处理每个文件
	for i, filePath := range allFiles {
		select {
		case <-ctx.Done():
			run.Status = BlockStatusCanceled
			run.Error = "用户取消"
			return
		default:
		}

		record, err := e.processFile(filePath, job, run, &manifest, versionDir)
		if err != nil {
			// 记录错误但继续
			e.emitProgress(run, "error", float64(i)/float64(len(allFiles)),
				fmt.Sprintf("处理文件失败 %s: %v", filePath, err))
			continue
		}

		if record != nil {
			manifest.Files = append(manifest.Files, *record)
			manifest.TotalSize += record.Size
		}

		run.DoneFiles = i + 1
		e.emitProgress(run, "backup", float64(i+1)/float64(len(allFiles)),
			fmt.Sprintf("进度: %d/%d 文件", i+1, len(allFiles)))
	}

	// 写入清单
	manifest.Blocks = append(manifest.Blocks, e.collectBlocks(&manifest)...)
	manifestPath := filepath.Join(versionDir, ManifestFile)
	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(manifestPath, data, 0644)

	// 清理旧版本
	e.cleanupOldVersions(job)

	run.Status = BlockStatusComplete
	e.emitProgress(run, "complete", 1.0, "备份完成")
}

// processFile 处理单个文件的块级备份
func (e *BlockBackupEngine) processFile(filePath string, job *BlockBackupJob, run *BlockBackupRun, manifest *Manifest, versionDir string) (*FileRecord, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}

	record := &FileRecord{
		Path:       filePath,
		Size:       info.Size(),
		ModTime:    info.ModTime(),
		Permission: fmt.Sprintf("%04o", info.Mode().Perm()),
		BlockRefs:  make([]string, 0),
	}

	checksum := sha256.New()
	buf := make([]byte, job.BlockSize)
	blockIndex := 0

	for {
		n, err := f.Read(buf)
		if n == 0 {
			break
		}

		blockData := buf[:n]
		checksum.Write(blockData)

		// 计算块哈希
		hash := sha256.Sum256(blockData)
		hashStr := hex.EncodeToString(hash[:])

		e.mu.RLock()
		dedup, exists := e.dedupIndex[hashStr]
		e.mu.RUnlock()

		if exists {
			// 去重命中
			dedup.RefCount++
			run.DedupSaved += int64(n)
		} else {
			// 新块，存储
			blockPath := filepath.Join(versionDir, "blocks", hashStr[:2], hashStr)
			os.MkdirAll(filepath.Dir(blockPath), 0750)

			writeData := blockData
			compressed := false

			// 压缩
			if job.Compression && n >= CompressionThreshold {
				writeData = compressBlock(blockData)
				compressed = true
				run.CompSaved += int64(n - len(writeData))
			}

			if err := os.WriteFile(blockPath, writeData, 0640); err != nil {
				return nil, err
			}

			e.mu.Lock()
			e.dedupIndex[hashStr] = &DedupEntry{
				Hash:     hashStr,
				Size:     n,
				Path:     blockPath,
				RefCount: 1,
			}
			e.mu.Unlock()

			manifest.Blocks = append(manifest.Blocks, BlockInfo{
				Hash:       hashStr,
				Offset:     int64(blockIndex * job.BlockSize),
				Size:       n,
				CompSize:   len(writeData),
				Compressed: compressed,
				RefCount:   1,
			})
		}

		record.BlockRefs = append(record.BlockRefs, hashStr)
		blockIndex++
		run.DoneBlocks++
		run.TotalBytes += int64(n)

		if err != nil && err != io.EOF {
			break
		}
	}

	record.Checksum = hex.EncodeToString(checksum.Sum(nil))
	return record, nil
}

// RestoreVersion 恢复指定版本
func (e *BlockBackupEngine) RestoreVersion(ctx context.Context, jobID string, version int, destPath string) error {
	e.mu.RLock()
	job, ok := e.jobs[jobID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("任务 '%s' 不存在", jobID)
	}

	versionDir := filepath.Join(job.DestPath, fmt.Sprintf("v%04d", version))
	manifestPath := filepath.Join(versionDir, ManifestFile)

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("读取清单失败: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return fmt.Errorf("解析清单失败: %w", err)
	}

	// 恢复每个文件
	for _, file := range manifest.Files {
		restorePath := filepath.Join(destPath, file.Path)
		os.MkdirAll(filepath.Dir(restorePath), 0750)

		outFile, err := os.Create(restorePath)
		if err != nil {
			return err
		}

		for _, blockHash := range file.BlockRefs {
			blockData, err := e.readBlock(versionDir, blockHash)
			if err != nil {
				outFile.Close()
				return err
			}
			outFile.Write(blockData)
		}
		outFile.Close()
		os.Chmod(restorePath, parsePermission(file.Permission))
	}

	return nil
}

// GetJob 获取备份任务
func (e *BlockBackupEngine) GetJob(jobID string) (*BlockBackupJob, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	job, ok := e.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("任务不存在")
	}
	return job, nil
}

// ListJobs 列出所有任务
func (e *BlockBackupEngine) ListJobs() []*BlockBackupJob {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*BlockBackupJob, 0, len(e.jobs))
	for _, j := range e.jobs {
		result = append(result, j)
	}
	sort.Slice(result, func(i, k int) bool {
		return result[i].CreatedAt.After(result[k].CreatedAt)
	})
	return result
}

// GetRun 获取运行记录
func (e *BlockBackupEngine) GetRun(runID string) (*BlockBackupRun, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	run, ok := e.runs[runID]
	if !ok {
		return nil, fmt.Errorf("运行记录不存在")
	}
	return run, nil
}

// PauseBackup 暂停备份
func (e *BlockBackupEngine) PauseBackup(runID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	run, ok := e.runs[runID]
	if !ok {
		return fmt.Errorf("运行记录不存在")
	}
	run.Status = BlockStatusPaused
	return nil
}

// DeleteJob 删除备份任务
func (e *BlockBackupEngine) DeleteJob(jobID string) error {
	e.mu.Lock()
	job, ok := e.jobs[jobID]
	if !ok {
		e.mu.Unlock()
		return fmt.Errorf("任务不存在")
	}
	if job.Status == BlockStatusRunning {
		e.mu.Unlock()
		return fmt.Errorf("不能删除运行中的任务")
	}
	delete(e.jobs, jobID)
	e.mu.Unlock()
	return e.saveJobs()
}

// GetStats 获取备份统计
func (e *BlockBackupEngine) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	totalDedupSaved := int64(0)
	totalCompSaved := int64(0)
	for _, r := range e.runs {
		totalDedupSaved += r.DedupSaved
		totalCompSaved += r.CompSaved
	}

	return map[string]interface{}{
		"total_jobs":        len(e.jobs),
		"total_runs":        len(e.runs),
		"dedup_index_size":  len(e.dedupIndex),
		"total_dedup_saved": totalDedupSaved,
		"total_comp_saved":  totalCompSaved,
	}
}

// ProgressChan 返回进度事件通道
func (e *BlockBackupEngine) ProgressChan() <-chan ProgressEvent {
	return e.progressCh
}

// Stop 停止引擎
func (e *BlockBackupEngine) Stop() {
	e.runCancel()
	close(e.progressCh)
}

// ========== 辅助函数 ==========

func (e *BlockBackupEngine) emitProgress(run *BlockBackupRun, typ string, progress float64, message string) {
	select {
	case e.progressCh <- ProgressEvent{
		RunID:    run.ID,
		JobID:    run.JobID,
		Type:     typ,
		Progress: progress,
		Message:  message,
	}:
	default:
	}
}

func (e *BlockBackupEngine) readBlock(versionDir, hash string) ([]byte, error) {
	blockPath := filepath.Join(versionDir, "blocks", hash[:2], hash)
	data, err := os.ReadFile(blockPath)
	if err != nil {
		// 尝试从全局块目录读取
		globalPath := filepath.Join(e.blockDir, hash[:2], hash)
		data, err = os.ReadFile(globalPath)
		if err != nil {
			return nil, fmt.Errorf("块 %s 不存在", hash)
		}
	}

	// 尝试解压
	if isCompressed(data) {
		return decompressBlock(data)
	}
	return data, nil
}

func (e *BlockBackupEngine) collectBlocks(manifest *Manifest) []BlockInfo {
	seen := make(map[string]bool)
	var blocks []BlockInfo
	for _, b := range manifest.Blocks {
		if !seen[b.Hash] {
			blocks = append(blocks, b)
			seen[b.Hash] = true
		}
	}
	return blocks
}

func (e *BlockBackupEngine) getNextVersion(jobID string) int {
	maxVer := 0
	for _, r := range e.runs {
		if r.JobID == jobID && r.Version > maxVer {
			maxVer = r.Version
		}
	}
	return maxVer + 1
}

func (e *BlockBackupEngine) cleanupOldVersions(job *BlockBackupJob) {
	versions := make([]int, 0)
	for _, r := range e.runs {
		if r.JobID == job.ID && r.Status == BlockStatusComplete {
			versions = append(versions, r.Version)
		}
	}
	sort.Ints(versions)

	for len(versions) > job.MaxVersions {
		oldVer := versions[0]
		versionDir := filepath.Join(job.DestPath, fmt.Sprintf("v%04d", oldVer))
		os.RemoveAll(versionDir)
		versions = versions[1:]
	}
}

func (e *BlockBackupEngine) saveJobs() error {
	data, err := json.MarshalIndent(e.jobs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.baseDir, "jobs.json"), data, 0644)
}

func (e *BlockBackupEngine) loadJobs() error {
	path := filepath.Join(e.baseDir, "jobs.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &e.jobs)
}

func (e *BlockBackupEngine) loadDedupIndex() error {
	path := filepath.Join(e.blockDir, DedupIndexFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &e.dedupIndex)
}

func (e *BlockBackupEngine) saveDedupIndex() error {
	data, err := json.MarshalIndent(e.dedupIndex, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(e.blockDir, DedupIndexFile), data, 0644)
}

func compressBlock(data []byte) []byte {
	if len(data) < CompressionThreshold {
		return data
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return data
	}
	if err := zw.Close(); err != nil {
		return data
	}
	compressed := buf.Bytes()
	if len(compressed) >= len(data) {
		return data
	}
	return compressed
}

func decompressBlock(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return io.ReadAll(zr)
}

func isCompressed(data []byte) bool {
	return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}

func parsePermission(perm string) os.FileMode {
	var mode uint32
	fmt.Sscanf(perm, "%o", &mode)
	return os.FileMode(mode)
}

// BlockBackupHandlers 块级备份HTTP处理器
type BlockBackupHandlers struct {
	engine *BlockBackupEngine
}

func NewBlockBackupHandlers(engine *BlockBackupEngine) *BlockBackupHandlers {
	return &BlockBackupHandlers{engine: engine}
}

func (h *BlockBackupHandlers) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/jobs", h.handleJobs)
	mux.HandleFunc(prefix+"/jobs/", h.handleJobByID)
	mux.HandleFunc(prefix+"/stats", h.handleStats)
}

func (h *BlockBackupHandlers) handleJobs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		jobs := h.engine.ListJobs()
		json.NewEncoder(w).Encode(map[string]interface{}{"jobs": jobs})
	case http.MethodPost:
		var req struct {
			Name        string   `json:"name"`
			SourcePaths []string `json:"source_paths"`
			DestPath    string   `json:"dest_path"`
			Compression bool     `json:"compression"`
			MaxVersions int      `json:"max_versions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		opts := []JobOption{
			WithCompression(req.Compression),
		}
		if req.MaxVersions > 0 {
			opts = append(opts, WithMaxVersions(req.MaxVersions))
		}
		job, err := h.engine.CreateJob(req.Name, req.SourcePaths, req.DestPath, opts...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(job)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *BlockBackupHandlers) handleJobByID(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/backup/jobs/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing job ID", http.StatusBadRequest)
		return
	}
	jobID := parts[0]

	if len(parts) > 1 {
		action := parts[1]
		switch action {
		case "run":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			run, err := h.engine.RunBackup(r.Context(), jobID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(run)
		case "restore":
			if r.Method != http.MethodPost {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			var req struct {
				Version  int    `json:"version"`
				DestPath string `json:"dest_path"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			if err := h.engine.RestoreVersion(r.Context(), jobID, req.Version, req.DestPath); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			json.NewEncoder(w).Encode(map[string]string{"status": "restored"})
		default:
			http.Error(w, "Unknown action", http.StatusBadRequest)
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		job, err := h.engine.GetJob(jobID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(job)
	case http.MethodDelete:
		if err := h.engine.DeleteJob(jobID); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *BlockBackupHandlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.engine.GetStats())
}
