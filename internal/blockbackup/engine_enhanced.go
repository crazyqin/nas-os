package blockbackup

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// ProgressCallback 进度回调函数
type ProgressCallback func(jobID string, progress int, bytesProcessed uint64)

// BackupChain 备份链 (full → incremental → incremental ...)
type BackupChain struct {
	RootSnapshot string        `json:"root_snapshot"` // 全量快照ID
	Chain        []*ChainEntry `json:"chain"`         // 链中各备份条目
	mu           sync.RWMutex
}

// ChainEntry 备份链条目
type ChainEntry struct {
	JobID     string    `json:"job_id"`
	SnapID    string    `json:"snap_id"`
	Type      string    `json:"type"` // full, incremental, differential
	ParentID  string    `json:"parent_id"`
	CreatedAt time.Time `json:"created_at"`
}

// BackupChainManager 备份链管理器
type BackupChainManager struct {
	chains map[string]*BackupChain // key = rootSnapshot ID
	mu     sync.RWMutex
	logger *zap.Logger
}

// BackupScheduler 备份调度器
type BackupScheduler struct {
	cron    *cron.Cron
	engine  *BlockBackupEngine
	logger  *zap.Logger
	entries map[string]cron.EntryID
	mu      sync.RWMutex
	running bool
}

// ScheduledBackup 调度备份配置
type ScheduledBackup struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Source   string `json:"source"`
	Dest     string `json:"dest"`
	Type     string `json:"type"`     // full, incremental, differential
	Schedule string `json:"schedule"` // cron 表达式
	Enabled  bool   `json:"enabled"`
}

// BandwidthLimiter 带宽限速器
type BandwidthLimiter struct {
	limitBytesPerSec int64
	bytesThisSecond  int64
	lastReset        time.Time
	mu               sync.Mutex
}

// ParallelExecutor 并行备份执行器
type ParallelExecutor struct {
	maxParallel int
	semaphore   chan struct{}
	activeJobs  sync.WaitGroup
	logger      *zap.Logger
}

// DifferentialBackupRequest 差异备份请求
type DifferentialBackupRequest struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

// --- BackupChainManager ---

func NewBackupChainManager(logger *zap.Logger) *BackupChainManager {
	return &BackupChainManager{
		chains: make(map[string]*BackupChain),
		logger: logger,
	}
}

// CreateChain 创建新的备份链 (以全量备份为根)
func (bcm *BackupChainManager) CreateChain(rootSnapID, jobID string) *BackupChain {
	bcm.mu.Lock()
	defer bcm.mu.Unlock()

	chain := &BackupChain{
		RootSnapshot: rootSnapID,
		Chain: []*ChainEntry{{
			JobID:     jobID,
			SnapID:    rootSnapID,
			Type:      "full",
			CreatedAt: time.Now(),
		}},
	}
	bcm.chains[rootSnapID] = chain
	bcm.logger.Info("Backup chain created",
		zap.String("root", rootSnapID),
		zap.String("job", jobID))
	return chain
}

// AppendToChain 向备份链追加增量/差异条目
func (bcm *BackupChainManager) AppendToChain(rootSnapID, jobID, snapID, backupType, parentSnapID string) error {
	bcm.mu.Lock()
	defer bcm.mu.Unlock()

	chain, ok := bcm.chains[rootSnapID]
	if !ok {
		return fmt.Errorf("backup chain not found for root snapshot %s", rootSnapID)
	}

	chain.mu.Lock()
	defer chain.mu.Unlock()

	entry := &ChainEntry{
		JobID:     jobID,
		SnapID:    snapID,
		Type:      backupType,
		ParentID:  parentSnapID,
		CreatedAt: time.Now(),
	}
	chain.Chain = append(chain.Chain, entry)

	bcm.logger.Info("Appended to backup chain",
		zap.String("root", rootSnapID),
		zap.String("type", backupType),
		zap.Int("chain_length", len(chain.Chain)))
	return nil
}

// GetChain 获取备份链
func (bcm *BackupChainManager) GetChain(rootSnapID string) *BackupChain {
	bcm.mu.RLock()
	defer bcm.mu.RUnlock()
	return bcm.chains[rootSnapID]
}

// ListChains 列出所有备份链
func (bcm *BackupChainManager) ListChains() []*BackupChain {
	bcm.mu.RLock()
	defer bcm.mu.RUnlock()
	chains := make([]*BackupChain, 0, len(bcm.chains))
	for _, c := range bcm.chains {
		chains = append(chains, c)
	}
	return chains
}

// GetLatestFullSnapshot 获取最近一次全量快照
func (bbe *BlockBackupEngine) GetLatestFullSnapshot() *BlockSnapshot {
	bbe.mu.RLock()
	defer bbe.mu.RUnlock()

	var latest *BlockSnapshot
	for _, snap := range bbe.snapshots {
		if snap.IsBase {
			if latest == nil || snap.CreatedAt.After(latest.CreatedAt) {
				latest = snap
			}
		}
	}
	return latest
}

// --- Differential Backup ---

// CreateDifferentialBackup 创建差异备份 (基于最后一次全量备份)
func (bbe *BlockBackupEngine) CreateDifferentialBackup(ctx context.Context, source, dest string) (*BackupJob, error) {
	// 查找最后一次全量快照
	baseSnap := bbe.GetLatestFullSnapshot()
	if baseSnap == nil {
		return nil, fmt.Errorf("no full backup snapshot found; run a full backup first")
	}

	bbe.mu.Lock()
	job := &BackupJob{
		ID:           fmt.Sprintf("diff-%d", time.Now().UnixNano()),
		Name:         fmt.Sprintf("Differential backup of %s", source),
		Source:       source,
		Destination:  dest,
		Type:         "differential",
		Status:       "pending",
		StartTime:    time.Now(),
		BaseSnapshot: baseSnap.ID,
	}
	bbe.jobs[job.ID] = job
	bbe.mu.Unlock()

	bbe.logger.Info("Starting differential backup",
		zap.String("job", job.ID),
		zap.String("base", baseSnap.ID))

	go bbe.runDifferentialBackup(ctx, job)
	return job, nil
}

func (bbe *BlockBackupEngine) runDifferentialBackup(ctx context.Context, job *BackupJob) {
	bbe.mu.Lock()
	job.Status = "running"
	bbe.mu.Unlock()

	// 使用 rsync 做差异比较，基于最后一次全量备份
	var cmd *exec.Cmd
	switch bbe.config.Compression {
	case "zstd":
		cmd = exec.CommandContext(ctx, "bash", "-c",
			fmt.Sprintf("rsync -av --delete --compare-dest=%s/ %s/ %s/ | zstd > %s.zst",
				job.BaseSnapshot, job.Source, job.Destination, job.Destination))
	default:
		cmd = exec.CommandContext(ctx, "bash", "-c",
			fmt.Sprintf("rsync -av --delete --compare-dest=%s/ %s/ %s/",
				job.BaseSnapshot, job.Source, job.Destination))
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		bbe.mu.Lock()
		job.Status = "failed"
		job.Error = string(output)
		job.EndTime = time.Now()
		bbe.mu.Unlock()
		bbe.logger.Error("Differential backup failed", zap.String("error", string(output)))
		return
	}

	snap := &BlockSnapshot{
		ID:        fmt.Sprintf("snap-%d", time.Now().UnixNano()),
		Volume:    job.Source,
		IsBase:    false,
		CreatedAt: time.Now(),
	}
	bbe.mu.Lock()
	bbe.snapshots[snap.ID] = snap
	job.Status = "completed"
	job.EndTime = time.Now()
	job.Duration = job.EndTime.Sub(job.StartTime)
	job.BaseSnapshot = snap.ID
	bbe.mu.Unlock()

	bbe.logger.Info("Differential backup completed",
		zap.String("job", job.ID),
		zap.Duration("duration", job.Duration))
}

// --- Progress Callback ---

// SetProgressCallback 设置进度回调 (可在任意 job 上使用)
func (bbe *BlockBackupEngine) SetProgressCallback(cb ProgressCallback) {
	bbe.mu.Lock()
	bbe.progressCallback = cb
	bbe.mu.Unlock()
}

// ReportProgress 报告进度
func (bbe *BlockBackupEngine) ReportProgress(jobID string, progress int, bytesProcessed uint64) {
	bbe.mu.RLock()
	cb := bbe.progressCallback
	bbe.mu.RUnlock()

	if cb != nil {
		cb(jobID, progress, bytesProcessed)
	}
}

// --- Bandwidth Limiter ---

func NewBandwidthLimiter(limitMBps int) *BandwidthLimiter {
	return &BandwidthLimiter{
		limitBytesPerSec: int64(limitMBps) * 1024 * 1024,
		lastReset:        time.Now(),
	}
}

// Acquire 带宽限制 - 阻塞直到有可用带宽配额
func (bl *BandwidthLimiter) Acquire(bytes int64) {
	if bl.limitBytesPerSec <= 0 {
		return // 无限制
	}

	for {
		bl.mu.Lock()
		now := time.Now()
		if now.Sub(bl.lastReset) >= time.Second {
			bl.bytesThisSecond = 0
			bl.lastReset = now
		}

		if bl.bytesThisSecond+bytes <= bl.limitBytesPerSec {
			bl.bytesThisSecond += bytes
			bl.mu.Unlock()
			return
		}
		bl.mu.Unlock()
		time.Sleep(10 * time.Millisecond)
	}
}

// --- Parallel Executor ---

func NewParallelExecutor(maxParallel int, logger *zap.Logger) *ParallelExecutor {
	if maxParallel <= 0 {
		maxParallel = 4
	}
	return &ParallelExecutor{
		maxParallel: maxParallel,
		semaphore:   make(chan struct{}, maxParallel),
		logger:      logger,
	}
}

// Submit 提交并行任务
func (pe *ParallelExecutor) Submit(fn func()) {
	pe.activeJobs.Add(1)
	go func() {
		pe.semaphore <- struct{}{} // 获取许可
		defer func() {
			<-pe.semaphore // 释放许可
			pe.activeJobs.Done()
		}()
		fn()
	}()
}

// Wait 等待所有任务完成
func (pe *ParallelExecutor) Wait() {
	pe.activeJobs.Wait()
}

// ActiveCount 当前活跃任务数
func (pe *ParallelExecutor) ActiveCount() int {
	return len(pe.semaphore)
}

// --- Backup Scheduler ---

func NewBackupScheduler(engine *BlockBackupEngine, logger *zap.Logger) *BackupScheduler {
	return &BackupScheduler{
		cron:    cron.New(),
		engine:  engine,
		logger:  logger,
		entries: make(map[string]cron.EntryID),
	}
}

// AddSchedule 添加定时备份
func (bs *BackupScheduler) AddSchedule(cfg ScheduledBackup) error {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if !cfg.Enabled {
		return nil
	}

	entryID, err := bs.cron.AddFunc(cfg.Schedule, func() {
		bs.logger.Info("Scheduled backup triggered",
			zap.String("id", cfg.ID),
			zap.String("type", cfg.Type))

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()

		var job *BackupJob
		var err error

		switch cfg.Type {
		case "full":
			job, err = bs.engine.CreateFullBackup(ctx, cfg.Source, cfg.Dest)
		case "incremental":
			snap := bs.engine.GetLatestFullSnapshot()
			baseSnap := ""
			if snap != nil {
				baseSnap = snap.ID
			}
			job, err = bs.engine.CreateIncrementalBackup(ctx, cfg.Source, cfg.Dest, baseSnap)
		case "differential":
			job, err = bs.engine.CreateDifferentialBackup(ctx, cfg.Source, cfg.Dest)
		default:
			bs.logger.Error("Unknown backup type", zap.String("type", cfg.Type))
			return
		}

		if err != nil {
			bs.logger.Error("Scheduled backup failed", zap.String("id", cfg.ID), zap.Error(err))
		} else {
			bs.logger.Info("Scheduled backup started", zap.String("id", cfg.ID), zap.String("job", job.ID))
		}
	})

	if err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", cfg.Schedule, err)
	}

	bs.entries[cfg.ID] = entryID
	bs.logger.Info("Backup schedule added",
		zap.String("id", cfg.ID),
		zap.String("schedule", cfg.Schedule),
		zap.String("type", cfg.Type))
	return nil
}

// RemoveSchedule 移除调度
func (bs *BackupScheduler) RemoveSchedule(id string) {
	bs.mu.Lock()
	defer bs.mu.Unlock()

	if entryID, ok := bs.entries[id]; ok {
		bs.cron.Remove(entryID)
		delete(bs.entries, id)
		bs.logger.Info("Backup schedule removed", zap.String("id", id))
	}
}

// Start 启动调度器
func (bs *BackupScheduler) Start() {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if bs.running {
		return
	}
	bs.cron.Start()
	bs.running = true
	bs.logger.Info("Backup scheduler started")
}

// Stop 停止调度器
func (bs *BackupScheduler) Stop() {
	bs.mu.Lock()
	defer bs.mu.Unlock()
	if !bs.running {
		return
	}
	bs.cron.Stop()
	bs.running = false
	bs.logger.Info("Backup scheduler stopped")
}

// ListSchedules 列出所有调度
func (bs *BackupScheduler) ListSchedules() []cron.Entry {
	bs.mu.RLock()
	defer bs.mu.RUnlock()
	return bs.cron.Entries()
}

// --- Enhanced Engine with parallel + bandwidth ---

// RunParallelBackups 并行执行多个备份任务
func (bbe *BlockBackupEngine) RunParallelBackups(ctx context.Context, requests []DifferentialBackupRequest, backupType string) ([]*BackupJob, error) {
	executor := NewParallelExecutor(bbe.config.Parallel, bbe.logger)
	results := make([]*BackupJob, len(requests))
	var firstErr atomic.Value
	var mu sync.Mutex

	for i, req := range requests {
		idx := i
		r := req
		executor.Submit(func() {
			var job *BackupJob
			var err error

			switch backupType {
			case "full":
				job, err = bbe.CreateFullBackup(ctx, r.Source, r.Destination)
			case "incremental":
				snap := bbe.GetLatestFullSnapshot()
				baseSnap := ""
				if snap != nil {
					baseSnap = snap.ID
				}
				job, err = bbe.CreateIncrementalBackup(ctx, r.Source, r.Destination, baseSnap)
			case "differential":
				job, err = bbe.CreateDifferentialBackup(ctx, r.Source, r.Destination)
			default:
				err = fmt.Errorf("unknown backup type: %s", backupType)
			}

			mu.Lock()
			if err != nil {
				bbe.logger.Error("Parallel backup failed", zap.Int("index", idx), zap.Error(err))
				firstErr.CompareAndSwap(nil, err)
			} else {
				results[idx] = job
			}
			mu.Unlock()
		})
	}

	executor.Wait()

	if v := firstErr.Load(); v != nil {
		return results, v.(error)
	}
	return results, nil
}

// ListSnapshots 列出所有快照
func (bbe *BlockBackupEngine) ListSnapshots() []*BlockSnapshot {
	bbe.mu.RLock()
	defer bbe.mu.RUnlock()

	snaps := make([]*BlockSnapshot, 0, len(bbe.snapshots))
	for _, s := range bbe.snapshots {
		snaps = append(snaps, s)
	}
	return snaps
}

// --- Helpers ---

// shellQuote 简单 shell 参数转义
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
