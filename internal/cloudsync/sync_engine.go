package cloudsync

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SyncEngine 同步引擎.
type SyncEngine struct {
	mu         sync.RWMutex
	provider   Provider
	task       *SyncTask
	status     *SyncStatus
	cancelFunc context.CancelFunc
	pauseChan  chan struct{}
	resumeChan chan struct{}

	// 回调函数
	onProgress func(status *SyncStatus)
	onComplete func(status *SyncStatus)
	onError    func(err error, path string)
	onConflict func(conflict *ConflictInfo) ConflictStrategy
}

// NewSyncEngine 创建同步引擎.
func NewSyncEngine(provider Provider, task *SyncTask) *SyncEngine {
	return &SyncEngine{
		provider:   provider,
		task:       task,
		pauseChan:  make(chan struct{}),
		resumeChan: make(chan struct{}),
		status: &SyncStatus{
			TaskID: task.ID,
			Status: TaskStatusIdle,
		},
	}
}

// SetCallbacks 设置回调函数.
func (e *SyncEngine) SetCallbacks(
	onProgress func(status *SyncStatus),
	onComplete func(status *SyncStatus),
	onError func(err error, path string),
	onConflict func(conflict *ConflictInfo) ConflictStrategy,
) {
	e.onProgress = onProgress
	e.onComplete = onComplete
	e.onError = onError
	e.onConflict = onConflict
}

// Run 执行同步.
func (e *SyncEngine) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	e.cancelFunc = cancel
	defer cancel()

	e.mu.Lock()
	e.status.Status = TaskStatusRunning
	e.status.StartTime = time.Now()
	e.status.ProcessedFiles = 0
	e.status.UploadedFiles = 0
	e.status.DownloadedFiles = 0
	e.status.SkippedFiles = 0
	e.status.FailedFiles = 0
	e.status.DeletedFiles = 0
	e.status.Errors = nil
	e.status.Conflicts = nil
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.status.EndTime = time.Now()
		if e.status.Status == TaskStatusRunning {
			e.status.Status = TaskStatusCompleted
		}
		e.mu.Unlock()

		if e.onComplete != nil {
			e.onComplete(e.status)
		}
	}()

	// 根据同步方向执行
	switch e.task.Direction {
	case SyncDirectionUpload:
		return e.syncUpload(ctx)
	case SyncDirectionDownload:
		return e.syncDownload(ctx)
	case SyncDirectionBidirect:
		return e.syncBidirectional(ctx)
	default:
		return fmt.Errorf("未知的同步方向: %s", e.task.Direction)
	}
}

// syncUpload 上传同步（本地 → 云端）.
func (e *SyncEngine) syncUpload(ctx context.Context) error {
	// 收集本地文件
	localFiles, err := e.collectLocalFiles(e.task.LocalPath)
	if err != nil {
		return fmt.Errorf("收集本地文件失败: %w", err)
	}

	// 收集远程文件
	remoteFiles, err := e.provider.List(ctx, e.task.RemotePath, true)
	if err != nil {
		// 远程目录可能不存在，创建它
		if err := e.provider.CreateDir(ctx, e.task.RemotePath); err != nil {
			return fmt.Errorf("创建远程目录失败: %w", err)
		}
		remoteFiles = []FileInfo{}
	}

	// 构建远程文件索引
	remoteIndex := make(map[string]FileInfo)
	for _, f := range remoteFiles {
		relPath := strings.TrimPrefix(f.Path, e.task.RemotePath)
		remoteIndex[relPath] = f
	}

	e.mu.Lock()
	e.status.TotalFiles = int64(len(localFiles))
	e.mu.Unlock()

	// 遍历本地文件
	for _, localFile := range localFiles {
		// 检查是否暂停
		if err := e.checkPause(ctx); err != nil {
			return err
		}

		// 检查是否取消
		select {
		case <-ctx.Done():
			e.mu.Lock()
			e.status.Status = TaskStatusCancelled
			e.mu.Unlock()
			return ctx.Err()
		default:
		}

		relPath, err := filepath.Rel(e.task.LocalPath, localFile.Path)
		if err != nil {
			continue
		}

		// 检查过滤规则
		if !e.shouldSync(relPath) {
			e.mu.Lock()
			e.status.SkippedFiles++
			e.mu.Unlock()
			continue
		}

		remotePath := filepath.Join(e.task.RemotePath, relPath)

		e.mu.Lock()
		e.status.CurrentFile = relPath
		e.status.CurrentAction = "upload"
		e.mu.Unlock()

		// 检查是否需要上传
		remoteFile, exists := remoteIndex[relPath]
		needUpload := true

		if exists {
			if e.task.ChecksumVerify {
				// 比较文件内容
				localHash, _ := e.calculateFileHash(localFile.Path)
				if localHash != "" && localHash == remoteFile.Hash {
					needUpload = false
				}
			} else {
				// 比较修改时间和大小
				if localFile.ModTime.Before(remoteFile.ModTime) && localFile.Size == remoteFile.Size {
					needUpload = false
				}
			}
		}

		if needUpload {
			startTime := time.Now()
			err := e.provider.Upload(ctx, localFile.Path, remotePath)
			duration := time.Since(startTime)

			e.mu.Lock()
			e.status.ProcessedFiles++
			if err != nil {
				e.status.FailedFiles++
				e.status.Errors = append(e.status.Errors, SyncError{
					Time:   time.Now(),
					Path:   relPath,
					Action: "upload",
					Error:  err.Error(),
				})
				if e.onError != nil {
					e.onError(err, localFile.Path)
				}
			} else {
				e.status.UploadedFiles++
				e.status.TransferredBytes += localFile.Size
				if duration.Seconds() > 0 {
					e.status.Speed = localFile.Size / int64(duration.Seconds()) / 1024
				}
			}
			e.status.Progress = float64(e.status.ProcessedFiles) / float64(e.status.TotalFiles) * 100
			e.mu.Unlock()
		} else {
			e.mu.Lock()
			e.status.SkippedFiles++
			e.status.ProcessedFiles++
			e.status.Progress = float64(e.status.ProcessedFiles) / float64(e.status.TotalFiles) * 100
			e.mu.Unlock()
		}

		if e.onProgress != nil {
			e.onProgress(e.status)
		}
	}

	// 处理需要删除的远程文件
	if e.task.DeleteRemote {
		for relPath, remoteFile := range remoteIndex {
			if remoteFile.IsDir {
				continue
			}

			localPath := filepath.Join(e.task.LocalPath, relPath)
			if _, err := os.Stat(localPath); os.IsNotExist(err) {
				// 本地不存在，删除远程
				err := e.provider.Delete(ctx, remoteFile.Path)
				e.mu.Lock()
				if err != nil {
					e.status.Errors = append(e.status.Errors, SyncError{
						Time:   time.Now(),
						Path:   relPath,
						Action: "delete_remote",
						Error:  err.Error(),
					})
				} else {
					e.status.DeletedFiles++
				}
				e.mu.Unlock()
			}
		}
	}

	return nil
}

// syncDownload 下载同步（云端 → 本地）.
func (e *SyncEngine) syncDownload(ctx context.Context) error {
	// 收集远程文件
	remoteFiles, err := e.provider.List(ctx, e.task.RemotePath, true)
	if err != nil {
		return fmt.Errorf("列出远程文件失败: %w", err)
	}

	// 收集本地文件
	localFiles, err := e.collectLocalFiles(e.task.LocalPath)
	if err != nil {
		localFiles = []FileInfo{}
	}

	// 构建本地文件索引
	localIndex := make(map[string]FileInfo)
	for _, f := range localFiles {
		relPath, _ := filepath.Rel(e.task.LocalPath, f.Path)
		localIndex[relPath] = f
	}

	e.mu.Lock()
	e.status.TotalFiles = int64(len(remoteFiles))
	e.mu.Unlock()

	// 遍历远程文件
	for _, remoteFile := range remoteFiles {
		// 检查是否暂停
		if err := e.checkPause(ctx); err != nil {
			return err
		}

		// 检查是否取消
		select {
		case <-ctx.Done():
			e.mu.Lock()
			e.status.Status = TaskStatusCancelled
			e.mu.Unlock()
			return ctx.Err()
		default:
		}

		if remoteFile.IsDir {
			continue
		}

		relPath := strings.TrimPrefix(remoteFile.Path, e.task.RemotePath)

		// 检查过滤规则
		if !e.shouldSync(relPath) {
			e.mu.Lock()
			e.status.SkippedFiles++
			e.mu.Unlock()
			continue
		}

		localPath := filepath.Join(e.task.LocalPath, relPath)

		e.mu.Lock()
		e.status.CurrentFile = relPath
		e.status.CurrentAction = "download"
		e.mu.Unlock()

		// 检查是否需要下载
		localFile, exists := localIndex[relPath]
		needDownload := true

		if exists {
			if e.task.ChecksumVerify {
				localHash, _ := e.calculateFileHash(localPath)
				if localHash != "" && localHash == remoteFile.Hash {
					needDownload = false
				}
			} else {
				// 远程较新或大小不同则下载
				if remoteFile.ModTime.Before(localFile.ModTime) && localFile.Size == remoteFile.Size {
					needDownload = false
				}
			}
		}

		if needDownload {
			startTime := time.Now()
			err := e.provider.Download(ctx, remoteFile.Path, localPath)
			duration := time.Since(startTime)

			e.mu.Lock()
			e.status.ProcessedFiles++
			if err != nil {
				e.status.FailedFiles++
				e.status.Errors = append(e.status.Errors, SyncError{
					Time:   time.Now(),
					Path:   relPath,
					Action: "download",
					Error:  err.Error(),
				})
				if e.onError != nil {
					e.onError(err, localPath)
				}
			} else {
				e.status.DownloadedFiles++
				e.status.TransferredBytes += remoteFile.Size
				if duration.Seconds() > 0 {
					e.status.Speed = remoteFile.Size / int64(duration.Seconds()) / 1024
				}
				// 保留修改时间
				if e.task.PreserveModTime {
					_ = os.Chtimes(localPath, remoteFile.ModTime, remoteFile.ModTime)
				}
			}
			e.status.Progress = float64(e.status.ProcessedFiles) / float64(e.status.TotalFiles) * 100
			e.mu.Unlock()
		} else {
			e.mu.Lock()
			e.status.SkippedFiles++
			e.status.ProcessedFiles++
			e.status.Progress = float64(e.status.ProcessedFiles) / float64(e.status.TotalFiles) * 100
			e.mu.Unlock()
		}

		if e.onProgress != nil {
			e.onProgress(e.status)
		}
	}

	// 处理需要删除的本地文件
	if e.task.DeleteLocal {
		for relPath, localFile := range localIndex {
			remotePath := filepath.Join(e.task.RemotePath, relPath)
			found := false
			for _, rf := range remoteFiles {
				if rf.Path == remotePath {
					found = true
					break
				}
			}

			if !found {
				// 远程不存在，删除本地
				err := os.Remove(localFile.Path)
				e.mu.Lock()
				if err != nil {
					e.status.Errors = append(e.status.Errors, SyncError{
						Time:   time.Now(),
						Path:   relPath,
						Action: "delete_local",
						Error:  err.Error(),
					})
				} else {
					e.status.DeletedFiles++
				}
				e.mu.Unlock()
			}
		}
	}

	return nil
}

// syncBidirectional 双向同步.
func (e *SyncEngine) syncBidirectional(ctx context.Context) error {
	// 收集本地文件
	localFiles, err := e.collectLocalFiles(e.task.LocalPath)
	if err != nil {
		return fmt.Errorf("收集本地文件失败: %w", err)
	}

	// 收集远程文件
	remoteFiles, err := e.provider.List(ctx, e.task.RemotePath, true)
	if err != nil {
		if err := e.provider.CreateDir(ctx, e.task.RemotePath); err != nil {
			return fmt.Errorf("创建远程目录失败: %w", err)
		}
		remoteFiles = []FileInfo{}
	}

	// 构建索引
	localIndex := make(map[string]FileInfo)
	for _, f := range localFiles {
		relPath, _ := filepath.Rel(e.task.LocalPath, f.Path)
		localIndex[relPath] = f
	}

	remoteIndex := make(map[string]FileInfo)
	for _, f := range remoteFiles {
		relPath := strings.TrimPrefix(f.Path, e.task.RemotePath)
		remoteIndex[relPath] = f
	}

	// 收集所有文件路径
	allPaths := make(map[string]bool)
	for path := range localIndex {
		allPaths[path] = true
	}
	for path := range remoteIndex {
		allPaths[path] = true
	}

	e.mu.Lock()
	e.status.TotalFiles = int64(len(allPaths))
	e.mu.Unlock()

	// 处理每个文件
	for relPath := range allPaths {
		// 检查是否暂停
		if err := e.checkPause(ctx); err != nil {
			return err
		}

		// 检查是否取消
		select {
		case <-ctx.Done():
			e.mu.Lock()
			e.status.Status = TaskStatusCancelled
			e.mu.Unlock()
			return ctx.Err()
		default:
		}

		// 检查过滤规则
		if !e.shouldSync(relPath) {
			e.mu.Lock()
			e.status.SkippedFiles++
			e.status.ProcessedFiles++
			e.status.Progress = float64(e.status.ProcessedFiles) / float64(e.status.TotalFiles) * 100
			e.mu.Unlock()
			continue
		}

		localFile, hasLocal := localIndex[relPath]
		remoteFile, hasRemote := remoteIndex[relPath]

		e.mu.Lock()
		e.status.CurrentFile = relPath
		e.mu.Unlock()

		var err error
		action := ""

		switch {
		case hasLocal && !hasRemote:
			// 只存在于本地，上传
			action = "upload"
			e.mu.Lock()
			e.status.CurrentAction = action
			e.mu.Unlock()

			remotePath := filepath.Join(e.task.RemotePath, relPath)
			err = e.provider.Upload(ctx, localFile.Path, remotePath)

			e.mu.Lock()
			if err != nil {
				e.status.FailedFiles++
			} else {
				e.status.UploadedFiles++
				e.status.TransferredBytes += localFile.Size
			}

		case !hasLocal && hasRemote:
			// 只存在于远程，下载
			action = "download"
			e.mu.Lock()
			e.status.CurrentAction = action
			e.mu.Unlock()

			localPath := filepath.Join(e.task.LocalPath, relPath)
			err = e.provider.Download(ctx, remoteFile.Path, localPath)

			e.mu.Lock()
			if err != nil {
				e.status.FailedFiles++
			} else {
				e.status.DownloadedFiles++
				e.status.TransferredBytes += remoteFile.Size
				if e.task.PreserveModTime {
					_ = os.Chtimes(localPath, remoteFile.ModTime, remoteFile.ModTime)
				}
			}

		case hasLocal && hasRemote:
			// 两边都存在，检测冲突
			localHash, _ := e.calculateFileHash(localFile.Path)
			needSync := false
			action = "skip"

			if e.task.ChecksumVerify {
				if localHash != remoteFile.Hash {
					needSync = true
				}
			} else {
				// 比较修改时间
				localModTime := localFile.ModTime.Truncate(time.Second)
				remoteModTime := remoteFile.ModTime.Truncate(time.Second)

				if !localModTime.Equal(remoteModTime) || localFile.Size != remoteFile.Size {
					needSync = true
				}
			}

			if needSync {
				// 检测冲突
				conflict := &ConflictInfo{
					Path:          relPath,
					LocalModTime:  localFile.ModTime,
					LocalSize:     localFile.Size,
					LocalHash:     localHash,
					RemoteModTime: remoteFile.ModTime,
					RemoteSize:    remoteFile.Size,
					RemoteHash:    remoteFile.Hash,
				}

				strategy := e.task.ConflictStrategy
				if e.onConflict != nil {
					strategy = e.onConflict(conflict)
				}

				e.mu.Lock()
				e.status.Conflicts = append(e.status.Conflicts, *conflict)
				e.mu.Unlock()

				switch strategy {
				case ConflictStrategyLocal:
					// 本地优先
					action = "upload"
					e.mu.Lock()
					e.status.CurrentAction = action
					e.mu.Unlock()
					remotePath := filepath.Join(e.task.RemotePath, relPath)
					err = e.provider.Upload(ctx, localFile.Path, remotePath)
					e.mu.Lock()
					if err != nil {
						e.status.FailedFiles++
					} else {
						e.status.UploadedFiles++
						e.status.TransferredBytes += localFile.Size
					}

				case ConflictStrategyRemote:
					// 远程优先
					action = "download"
					e.mu.Lock()
					e.status.CurrentAction = action
					e.mu.Unlock()
					localPath := filepath.Join(e.task.LocalPath, relPath)
					err = e.provider.Download(ctx, remoteFile.Path, localPath)
					e.mu.Lock()
					if err != nil {
						e.status.FailedFiles++
					} else {
						e.status.DownloadedFiles++
						e.status.TransferredBytes += remoteFile.Size
					}

				case ConflictStrategyNewer:
					// 较新的优先
					if localFile.ModTime.After(remoteFile.ModTime) {
						action = "upload"
						e.mu.Lock()
						e.status.CurrentAction = action
						e.mu.Unlock()
						remotePath := filepath.Join(e.task.RemotePath, relPath)
						err = e.provider.Upload(ctx, localFile.Path, remotePath)
						e.mu.Lock()
						if err != nil {
							e.status.FailedFiles++
						} else {
							e.status.UploadedFiles++
							e.status.TransferredBytes += localFile.Size
						}
					} else {
						action = "download"
						e.mu.Lock()
						e.status.CurrentAction = action
						e.mu.Unlock()
						localPath := filepath.Join(e.task.LocalPath, relPath)
						err = e.provider.Download(ctx, remoteFile.Path, localPath)
						e.mu.Lock()
						if err != nil {
							e.status.FailedFiles++
						} else {
							e.status.DownloadedFiles++
							e.status.TransferredBytes += remoteFile.Size
						}
					}

				case ConflictStrategySkip:
					// 跳过
					e.mu.Lock()
					e.status.SkippedFiles++
					e.mu.Unlock()

				default:
					// 默认跳过
					e.mu.Lock()
					e.status.SkippedFiles++
					e.mu.Unlock()
				}
			} else {
				e.mu.Lock()
				e.status.SkippedFiles++
			}
		}

		e.mu.Lock()
		e.status.ProcessedFiles++
		if err != nil {
			e.status.Errors = append(e.status.Errors, SyncError{
				Time:   time.Now(),
				Path:   relPath,
				Action: action,
				Error:  err.Error(),
			})
		}
		e.status.Progress = float64(e.status.ProcessedFiles) / float64(e.status.TotalFiles) * 100
		e.mu.Unlock()

		if e.onProgress != nil {
			e.onProgress(e.status)
		}
	}

	return nil
}

// Pause 暂停同步.
func (e *SyncEngine) Pause() {
	e.pauseChan <- struct{}{}
}

// Resume 恢复同步.
func (e *SyncEngine) Resume() {
	e.resumeChan <- struct{}{}
}

// Cancel 取消同步.
func (e *SyncEngine) Cancel() {
	if e.cancelFunc != nil {
		e.cancelFunc()
	}
}

// GetStatus 获取状态.
func (e *SyncEngine) GetStatus() *SyncStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.status
}

// checkPause 检查暂停状态.
func (e *SyncEngine) checkPause(ctx context.Context) error {
	select {
	case <-e.pauseChan:
		e.mu.Lock()
		e.status.Status = TaskStatusPaused
		e.mu.Unlock()

		// 等待恢复
		<-e.resumeChan

		e.mu.Lock()
		e.status.Status = TaskStatusRunning
		e.mu.Unlock()

	case <-ctx.Done():
		return ctx.Err()

	default:
	}
	return nil
}

// collectLocalFiles 收集本地文件.
func (e *SyncEngine) collectLocalFiles(rootPath string) ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// 检查文件大小限制
		if e.task.MaxFileSize > 0 && info.Size() > e.task.MaxFileSize {
			return nil
		}

		files = append(files, FileInfo{
			Path:    path,
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   false,
		})

		return nil
	})

	return files, err
}

// shouldSync 检查文件是否应该同步.
func (e *SyncEngine) shouldSync(relPath string) bool {
	// 检查排除规则
	for _, pattern := range e.task.ExcludePatterns {
		matched, _ := filepath.Match(pattern, relPath)
		if matched {
			return false
		}
		// 支持目录模式
		if strings.HasSuffix(pattern, "/") {
			if strings.HasPrefix(relPath, pattern) {
				return false
			}
		}
	}

	// 检查包含规则
	if len(e.task.IncludePatterns) > 0 {
		for _, pattern := range e.task.IncludePatterns {
			matched, _ := filepath.Match(pattern, relPath)
			if matched {
				return true
			}
		}
		return false
	}

	return true
}

// calculateFileHash 计算文件哈希（使用 SHA256）.
func (e *SyncEngine) calculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
