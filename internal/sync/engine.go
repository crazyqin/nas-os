package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Engine 同步核心引擎.
// 整合 delta 计算、冲突检测、版本管理、文件传输等模块.
type Engine struct {
	mu       sync.RWMutex
	task     *Task
	store    *StateStore
	versions *VersionManager
	notifier *Notifier
	watcher  *Watcher

	// 并发控制
	semaphore chan struct{}

	// 运行时状态
	progress *Progress
	running  bool
	ctx      context.Context
	cancel   context.CancelFunc
	pauseCh  chan struct{}
	resumeCh chan struct{}
}

// EngineConfig 引擎配置.
type EngineConfig struct {
	StateDir   string
	VersionDir string
	MaxKeep    int
	Workers    int // 并发 worker 数
}

// NewEngine 创建同步引擎.
func NewEngine(task *Task, provider Provider, cfg EngineConfig) (*Engine, error) {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.MaxKeep <= 0 {
		cfg.MaxKeep = 5
	}

	store := NewStateStore(cfg.StateDir)
	if err := store.Load(); err != nil {
		return nil, fmt.Errorf("load state: %w", err)
	}

	vm := NewVersionManager(cfg.VersionDir, cfg.MaxKeep)

	e := &Engine{
		task:      task,
		store:     store,
		versions:  vm,
		notifier:  NewNotifier(),
		semaphore: make(chan struct{}, cfg.Workers),
		pauseCh:   make(chan struct{}),
		resumeCh:  make(chan struct{}),
		progress: &Progress{
			TaskID:    task.ID,
			Direction: task.Direction,
			State:     "idle",
		},
	}

	// 创建 watcher
	w, err := NewWatcher(task.ID, task, provider, store)
	if err != nil {
		return nil, fmt.Errorf("create watcher: %w", err)
	}
	e.watcher = w

	return e, nil
}

// Start 启动引擎（启动 watcher）.
func (e *Engine) Start(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("engine already running")
	}

	e.running = true
	e.notifier.Start(ctx)

	// 启动 watcher，设置回调触发同步
	e.watcher.SetOnChanges(func(delta *Delta) {
		// 通知上层有变化（外部可决定何时触发同步）
		e.notifier.Emit(&Event{
			Type:    EventScanComplete,
			TaskID:  e.task.ID,
			Message: "watcher detected changes",
			Extra:   delta,
		})
	})

	if err := e.watcher.Start(ctx); err != nil {
		return fmt.Errorf("start watcher: %w", err)
	}

	e.notifier.Emit(&Event{
		Type:    EventWatchStarted,
		TaskID:  e.task.ID,
		Message: "sync engine started",
	})

	return nil
}

// Stop 停止引擎.
func (e *Engine) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return nil
	}

	e.running = false
	e.watcher.Stop()
	e.notifier.Stop()

	e.notifier.Emit(&Event{
		Type:    EventWatchStopped,
		TaskID:  e.task.ID,
		Message: "sync engine stopped",
	})

	return nil
}

// Sync 执行一次完整同步.
func (e *Engine) Sync(ctx context.Context, provider Provider) error {
	e.mu.Lock()

	if e.progress.State == "running" {
		e.mu.Unlock()
		return fmt.Errorf("sync already in progress")
	}

	e.ctx, e.cancel = context.WithCancel(ctx)
	e.progress.State = "running"
	e.progress.StartTime = time.Now()
	e.progress.TaskID = e.task.ID

	e.mu.Unlock()

	e.notifier.EmitMigrationStart(e.task.ID, e.task.Direction)

	// 捕获异常
	var syncErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				syncErr = fmt.Errorf("sync panic: %v", r)
			}
		}()
		syncErr = e.runSync(ctx, provider)
	}()

	e.mu.Lock()
	e.progress.EndTime = ptrTime(time.Now())
	if syncErr != nil {
		e.progress.State = "failed"
		e.mu.Unlock()
		e.notifier.EmitMigrationFailed(e.task.ID, syncErr, e.progress)
		return syncErr
	}
	e.progress.State = "completed"
	e.mu.Unlock()

	// 保存状态
	if err := e.store.Save(); err != nil {
		e.notifier.EmitError(e.task.ID, err, "failed to save state")
	}

	e.notifier.EmitMigrationComplete(e.task.ID, e.progress)
	return nil
}

// runSync 执行核心同步逻辑.
func (e *Engine) runSync(ctx context.Context, provider Provider) error {
	// 1. 扫描本地当前快照
	e.notifier.Emit(&Event{
		Type:    EventScanStart,
		TaskID:  e.task.ID,
		Message: "scanning local files",
	})

	localScanner := NewSnapshotScanner()
	localScanner.SetExcludePatterns(e.task.ExcludePatterns)
	localScanner.SetMaxSize(e.task.MaxFileSize)
	localScanner.SetChecksum(e.task.ChecksumVerify)

	localSnap, err := localScanner.Scan(e.task.LocalPath, 0)
	if err != nil {
		return fmt.Errorf("scan local: %w", err)
	}

	e.notifier.Emit(&Event{
		Type:    EventScanComplete,
		TaskID:  e.task.ID,
		Message: fmt.Sprintf("local scan done: %d files", len(localSnap.Entries)),
	})

	// 2. 扫描远程当前快照
	remoteScanner := NewRemoteScanner(provider)
	remoteSnap, err := remoteScanner.Scan(ctx, e.task.RemotePath, 0)
	if err != nil {
		return fmt.Errorf("scan remote: %w", err)
	}

	e.notifier.Emit(&Event{
		Type:    EventScanComplete,
		TaskID:  e.task.ID,
		Message: fmt.Sprintf("remote scan done: %d files", len(remoteSnap.Entries)),
	})

	// 3. 计算 delta（相对于上次同步状态）
	localDelta := e.store.GetDeltaFromLastSync(e.task.ID, localSnap)

	var oldRemoteSnap *Snapshot
	if ts := e.store.GetTaskState(e.task.ID); ts != nil {
		oldRemoteSnap = ts.RemoteSnapshot
	}
	remoteDelta := ComputeDelta(oldRemoteSnap, remoteSnap)

	e.mu.Lock()
	e.progress.TotalFiles = localDelta.TotalFilesDelta() + remoteDelta.TotalFilesDelta()
	e.mu.Unlock()

	// 4. 冲突检测（仅双向同步）
	var conflicts []*Conflict
	if e.task.Direction == DirectionBidirectional {
		detector := NewConflictDetector(e.task.ConflictStrategy)
		ts := e.store.GetTaskState(e.task.ID)
		conflicts = detector.DetectAll(e.task.ID, localDelta, remoteDelta, ts.FileStates)
		for _, c := range conflicts {
			e.notifier.EmitConflict(c)
			e.mu.Lock()
			e.progress.Conflicts++
			e.mu.Unlock()
		}
	}

	// 5. 确定操作计划
	plan := e.buildPlan(localDelta, remoteDelta, conflicts)

	// 6. 执行操作
	return e.executePlan(ctx, plan, provider)
}

// buildPlan 根据 delta 和冲突构建同步计划.
func (e *Engine) buildPlan(localDelta, remoteDelta *Delta, conflicts []*Conflict) *SyncPlan {
	plan := &SyncPlan{}

	conflictMap := make(map[string]*Conflict)
	for _, c := range conflicts {
		conflictMap[c.RelPath] = c
	}

	switch e.task.Direction {
	case DirectionUpload:
		for _, item := range localDelta.Adds {
			plan.ops = append(plan.ops, &SyncOp{
				Type:    SyncOpUpload,
				RelPath: item.RelPath,
				SrcPath: filepath.Join(e.task.LocalPath, item.RelPath),
				DstPath: filepath.Join(e.task.RemotePath, item.RelPath),
				Size:    item.NewEntry.Size,
			})
		}
		for _, item := range localDelta.Mods {
			plan.ops = append(plan.ops, &SyncOp{
				Type:    SyncOpUpload,
				RelPath: item.RelPath,
				SrcPath: filepath.Join(e.task.LocalPath, item.RelPath),
				DstPath: filepath.Join(e.task.RemotePath, item.RelPath),
				Size:    item.NewEntry.Size,
			})
		}
		if e.task.DeleteOrphan {
			for _, item := range remoteDelta.Dels {
				plan.ops = append(plan.ops, &SyncOp{
					Type:    SyncOpDelete,
					RelPath: item.RelPath,
					DstPath: filepath.Join(e.task.RemotePath, item.RelPath),
				})
			}
		}

	case DirectionDownload:
		for _, item := range localDelta.Adds {
			plan.ops = append(plan.ops, &SyncOp{
				Type:    SyncOpDownload,
				RelPath: item.RelPath,
				SrcPath: filepath.Join(e.task.RemotePath, item.RelPath),
				DstPath: filepath.Join(e.task.LocalPath, item.RelPath),
				Size:    item.NewEntry.Size,
			})
		}
		for _, item := range localDelta.Mods {
			plan.ops = append(plan.ops, &SyncOp{
				Type:    SyncOpDownload,
				RelPath: item.RelPath,
				SrcPath: filepath.Join(e.task.RemotePath, item.RelPath),
				DstPath: filepath.Join(e.task.LocalPath, item.RelPath),
				Size:    item.NewEntry.Size,
			})
		}
		if e.task.DeleteOrphan {
			for _, item := range localDelta.Dels {
				plan.ops = append(plan.ops, &SyncOp{
					Type:    SyncOpDelete,
					RelPath: item.RelPath,
					DstPath: filepath.Join(e.task.LocalPath, item.RelPath),
				})
			}
		}

	case DirectionBidirectional:
		for _, item := range localDelta.Adds {
			op := &SyncOp{
				RelPath: item.RelPath,
				SrcPath: filepath.Join(e.task.LocalPath, item.RelPath),
				DstPath: filepath.Join(e.task.RemotePath, item.RelPath),
				Size:    item.NewEntry.Size,
			}
			if _, conflict := conflictMap[item.RelPath]; conflict {
				op.Type = SyncOpConflict
			} else {
				op.Type = SyncOpUpload
			}
			plan.ops = append(plan.ops, op)
		}
		for _, item := range localDelta.Mods {
			op := &SyncOp{
				RelPath: item.RelPath,
				SrcPath: filepath.Join(e.task.LocalPath, item.RelPath),
				DstPath: filepath.Join(e.task.RemotePath, item.RelPath),
				Size:    item.NewEntry.Size,
			}
			if _, conflict := conflictMap[item.RelPath]; conflict {
				op.Type = SyncOpConflict
			} else {
				op.Type = SyncOpUpload
			}
			plan.ops = append(plan.ops, op)
		}
		for _, item := range localDelta.Dels {
			op := &SyncOp{
				RelPath: item.RelPath,
				DstPath: filepath.Join(e.task.RemotePath, item.RelPath),
			}
			if _, conflict := conflictMap[item.RelPath]; conflict {
				op.Type = SyncOpConflict
			} else {
				op.Type = SyncOpDelete
			}
			plan.ops = append(plan.ops, op)
		}
		// 远程独有
		for _, item := range remoteDelta.Adds {
			if _, conflict := conflictMap[item.RelPath]; !conflict {
				plan.ops = append(plan.ops, &SyncOp{
					Type:    SyncOpDownload,
					RelPath: item.RelPath,
					SrcPath: filepath.Join(e.task.RemotePath, item.RelPath),
					DstPath: filepath.Join(e.task.LocalPath, item.RelPath),
					Size:    item.NewEntry.Size,
				})
			}
		}
		for _, item := range remoteDelta.Mods {
			if _, conflict := conflictMap[item.RelPath]; !conflict {
				plan.ops = append(plan.ops, &SyncOp{
					Type:    SyncOpDownload,
					RelPath: item.RelPath,
					SrcPath: filepath.Join(e.task.RemotePath, item.RelPath),
					DstPath: filepath.Join(e.task.LocalPath, item.RelPath),
					Size:    item.NewEntry.Size,
				})
			}
		}
	}

	return plan
}

// executePlan 执行同步计划.
func (e *Engine) executePlan(ctx context.Context, plan *SyncPlan, provider Provider) error {
	totalOps := int64(len(plan.ops))
	e.mu.Lock()
	e.progress.TotalFiles = totalOps
	e.mu.Unlock()

	var wg sync.WaitGroup

	for i, op := range plan.ops {
		// 暂停检查
		select {
		case <-e.pauseCh:
			e.mu.Lock()
			e.progress.State = "paused"
			e.mu.Unlock()
			<-e.resumeCh
			e.mu.Lock()
			e.progress.State = "running"
			e.mu.Unlock()
		default:
		}

		// 取消检查
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		wg.Add(1)
		go func(idx int, op *SyncOp) {
			defer wg.Done()

			// 信号量获取
			e.semaphore <- struct{}{}
			defer func() { <-e.semaphore }()

			e.mu.Lock()
			e.progress.CurrentFile = op.RelPath
			e.progress.CurrentOp = string(op.Type)
			e.mu.Unlock()

			var err error
			switch op.Type {
			case SyncOpUpload:
				// 版本管理
				if e.task.VersionKeep > 0 {
					_ = e.versions.StoreVersion(e.task.ID, op.RelPath, op.SrcPath, int64(i))
					_ = e.versions.PruneVersions(e.task.ID, op.RelPath)
				}
				err = provider.Upload(ctx, op.SrcPath, op.DstPath)
				e.mu.Lock()
				if err == nil {
					e.progress.UploadedFiles++
				}
				e.mu.Unlock()

			case SyncOpDownload:
				// 确保目录存在
				_ = os.MkdirAll(filepath.Dir(op.DstPath), 0750)
				// 版本管理
				if e.task.VersionKeep > 0 {
					_ = e.versions.StoreVersion(e.task.ID, op.RelPath, op.DstPath, int64(i))
					_ = e.versions.PruneVersions(e.task.ID, op.RelPath)
				}
				err = provider.Download(ctx, op.SrcPath, op.DstPath)
				e.mu.Lock()
				if err == nil {
					e.progress.DownloadedFiles++
					if e.task.PreserveModTime {
						_ = os.Chtimes(op.DstPath, time.Now(), time.Now())
					}
				}
				e.mu.Unlock()

			case SyncOpDelete:
				if strings.HasPrefix(op.DstPath, e.task.LocalPath) {
					err = os.Remove(op.DstPath)
				} else {
					err = provider.Delete(ctx, op.DstPath)
				}
				e.mu.Lock()
				if err == nil {
					e.progress.DeletedFiles++
				}
				e.mu.Unlock()

			case SyncOpRename:
				// rename 冲突处理：保留双方
				e.mu.RLock()
				c := &Conflict{
					RelPath: op.RelPath,
					TaskID:  e.task.ID,
				}
				e.mu.RUnlock()
				detector := NewConflictDetector(e.task.ConflictStrategy)
				_, _, err = detector.ResolveRename(c, e.task.LocalPath, e.task.RemotePath, provider)
				e.mu.Lock()
				if err == nil {
					e.progress.UploadedFiles++
				}
				e.mu.Unlock()

			case SyncOpConflict:
				e.mu.Lock()
				e.progress.Conflicts++
				e.mu.Unlock()
				// 冲突文件暂不操作，等待用户处理

			default:
				e.mu.Lock()
				e.progress.SkippedFiles++
				e.mu.Unlock()
			}

			e.mu.Lock()
			e.progress.ProcessedFiles++
			if err != nil {
				e.progress.ErrorCount++
				e.notifier.EmitError(e.task.ID, err, fmt.Sprintf("sync op failed: %s %s", op.Type, op.RelPath))
			}
			e.progress.TransferredBytes += op.Size
			if totalOps > 0 {
				e.progress.ProgressPct = float64(e.progress.ProcessedFiles) / float64(totalOps) * 100
			}
			e.mu.Unlock()

			// 每 10 个文件或进度变化时发送事件
			if e.progress.ProcessedFiles%10 == 0 || e.progress.ProcessedFiles == totalOps {
				e.notifier.EmitProgress(e.task.ID, e.progress)
			}

		}(i, op)
	}

	wg.Wait()
	return nil
}

// Pause 暂停同步.
func (e *Engine) Pause() {
	e.pauseCh <- struct{}{}
}

// Resume 恢复同步.
func (e *Engine) Resume() {
	e.resumeCh <- struct{}{}
}

// Cancel 取消同步.
func (e *Engine) Cancel() {
	if e.cancel != nil {
		e.cancel()
	}
}

// GetProgress 返回当前同步进度.
func (e *Engine) GetProgress() *Progress {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.progress
}

// GetVersionManager 返回版本管理器.
func (e *Engine) GetVersionManager() *VersionManager {
	return e.versions
}

// GetNotifier 返回通知器.
func (e *Engine) GetNotifier() *Notifier {
	return e.notifier
}

// GetWatcher 返回 watcher.
func (e *Engine) GetWatcher() *Watcher {
	return e.watcher
}

// SyncPlan 同步计划.
type SyncPlan struct {
	ops []*SyncOp
}

// SyncOp 单个同步操作.
type SyncOp struct {
	Type    SyncOpType
	RelPath string
	SrcPath string // 源路径（绝对）
	DstPath string // 目标路径（绝对）
	Size    int64
}

func ptrTime(t time.Time) *time.Time { return &t }
