// Package cloudsync provides cloud storage synchronization
// This file implements advanced conflict resolution with rename strategy
package cloudsync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ConflictResolver 冲突解决器.
type ConflictResolver struct {
	task      *SyncTask
	provider  Provider
	renames   map[string]string // 记录重命名映射
	onResolve func(original, renamed string)
}

// NewConflictResolver 创建冲突解决器.
func NewConflictResolver(task *SyncTask, provider Provider) *ConflictResolver {
	return &ConflictResolver{
		task:     task,
		provider: provider,
		renames:  make(map[string]string),
	}
}

// Resolve 解决文件冲突.
func (r *ConflictResolver) Resolve(ctx context.Context, conflict *ConflictInfo) (ResolutionResult, error) {
	strategy := r.task.ConflictStrategy

	result := ResolutionResult{
		OriginalPath: conflict.Path,
		Action:       SyncOpSkip,
	}

	switch strategy {
	case ConflictStrategySkip:
		result.Action = SyncOpSkip
		result.Message = "跳过冲突文件"

	case ConflictStrategyLocal:
		result.Action = SyncOpUpload
		result.Message = "本地优先，上传覆盖远程"

	case ConflictStrategyRemote:
		result.Action = SyncOpDownload
		result.Message = "远程优先，下载覆盖本地"

	case ConflictStrategyNewer:
		if conflict.LocalModTime.After(conflict.RemoteModTime) {
			result.Action = SyncOpUpload
			result.Message = "本地较新，上传覆盖远程"
		} else if conflict.RemoteModTime.After(conflict.LocalModTime) {
			result.Action = SyncOpDownload
			result.Message = "远程较新，下载覆盖本地"
		} else {
			// 时间相同，比较大小
			if conflict.LocalSize >= conflict.RemoteSize {
				result.Action = SyncOpUpload
				result.Message = "时间相同，本地较大，上传覆盖远程"
			} else {
				result.Action = SyncOpDownload
				result.Message = "时间相同，远程较大，下载覆盖本地"
			}
		}

	case ConflictStrategyRename:
		// 重命名冲突文件
		renamedPath, err := r.generateRenamePath(ctx, conflict)
		if err != nil {
			return result, fmt.Errorf("生成重命名路径失败: %w", err)
		}
		result.RenamedPath = renamedPath
		result.Action = SyncOpConflict
		result.Message = fmt.Sprintf("重命名冲突文件: %s -> %s", conflict.Path, renamedPath)
		r.renames[conflict.Path] = renamedPath
		if r.onResolve != nil {
			r.onResolve(conflict.Path, renamedPath)
		}

	case ConflictStrategyAsk:
		// 询问策略需要外部回调处理
		result.Action = SyncOpConflict
		result.Message = "等待用户决定"
		result.NeedUserInput = true

	default:
		result.Action = SyncOpSkip
		result.Message = "未知策略，跳过"
	}

	return result, nil
}

// generateRenamePath 生成重命名路径.
func (r *ConflictResolver) generateRenamePath(ctx context.Context, conflict *ConflictInfo) (string, error) {
	ext := filepath.Ext(conflict.Path)
	base := strings.TrimSuffix(conflict.Path, ext)

	// 使用时间戳或版本号作为后缀
	var suffix string
	if !conflict.LocalModTime.IsZero() {
		suffix = conflict.LocalModTime.Format("_conflict_20060102_150405")
	} else {
		suffix = fmt.Sprintf("_conflict_%d", time.Now().Unix())
	}

	// 生成本地重命名路径
	localRenamed := base + suffix + "_local" + ext

	// 生成远程重命名路径
	remoteRenamed := base + suffix + "_remote" + ext

	// 检查重命名路径是否已存在
	localPath := filepath.Join(r.task.LocalPath, localRenamed)
	if _, err := os.Stat(localPath); err == nil {
		// 已存在，添加序号
		for i := 1; i < 100; i++ {
			localRenamed = base + suffix + fmt.Sprintf("_local_%d", i) + ext
			localPath = filepath.Join(r.task.LocalPath, localRenamed)
			if _, err := os.Stat(localPath); os.IsNotExist(err) {
				break
			}
		}
	}

	remotePath := filepath.Join(r.task.RemotePath, remoteRenamed)
	info, err := r.provider.Stat(ctx, remotePath)
	if err == nil && info != nil {
		// 已存在，添加序号
		for i := 1; i < 100; i++ {
			remoteRenamed = base + suffix + fmt.Sprintf("_remote_%d", i) + ext
			remotePath = filepath.Join(r.task.RemotePath, remoteRenamed)
			info, err = r.provider.Stat(ctx, remotePath)
			if err != nil || info == nil {
				break
			}
		}
	}

	// 返回两个重命名路径（用于保留两个版本）
	return localRenamed + "|" + remoteRenamed, nil
}

// ExecuteRename 执行重命名操作，保留两个版本.
func (r *ConflictResolver) ExecuteRename(ctx context.Context, conflict *ConflictInfo, renamedPaths string) error {
	paths := strings.Split(renamedPaths, "|")
	if len(paths) != 2 {
		return fmt.Errorf("无效的重命名路径格式")
	}

	localRenamed := paths[0]
	remoteRenamed := paths[1]

	// 1. 保留本地版本：重命名本地文件
	localOriginal := filepath.Join(r.task.LocalPath, conflict.Path)
	localNew := filepath.Join(r.task.LocalPath, localRenamed)

	if err := os.Rename(localOriginal, localNew); err != nil {
		return fmt.Errorf("重命名本地文件失败: %w", err)
	}

	// 2. 下载远程版本到新位置
	remoteOriginal := filepath.Join(r.task.RemotePath, conflict.Path)
	_ = filepath.Join(r.task.RemotePath, remoteRenamed) // remoteNew: remote path for renamed version (用于后续同步)

	// 先下载远程版本到临时位置
	tempPath := filepath.Join(r.task.LocalPath, ".temp_"+remoteRenamed)
	if err := r.provider.Download(ctx, remoteOriginal, tempPath); err != nil {
		// 回滚本地重命名
		_ = os.Rename(localNew, localOriginal)
		return fmt.Errorf("下载远程版本失败: %w", err)
	}

	// 移动到最终位置
	if err := os.Rename(tempPath, filepath.Join(r.task.LocalPath, remoteRenamed)); err != nil {
		_ = os.Remove(tempPath)
		_ = os.Rename(localNew, localOriginal)
		return fmt.Errorf("移动远程版本失败: %w", err)
	}

	// 3. 在远程也创建两个版本（可选）
	// 上传本地重命名版本到远程
	if err := r.provider.Upload(ctx, localNew, filepath.Join(r.task.RemotePath, localRenamed)); err != nil {
		// 本地已经有两个版本，只是远程同步失败，不回滚
		return fmt.Errorf("上传本地重命名版本到远程失败: %w", err)
	}

	return nil
}

// GetRenames 获取所有重命名映射.
func (r *ConflictResolver) GetRenames() map[string]string {
	return r.renames
}

// SetOnResolveCallback 设置解决回调.
func (r *ConflictResolver) SetOnResolveCallback(callback func(original, renamed string)) {
	r.onResolve = callback
}

// ResolutionResult 冲突解决结果.
type ResolutionResult struct {
	OriginalPath  string     `json:"originalPath"`
	RenamedPath   string     `json:"renamedPath,omitempty"`
	Action        SyncOpType `json:"action"`
	Message       string     `json:"message"`
	NeedUserInput bool       `json:"needUserInput"`
}

// ConflictHistory 冲突历史记录.
type ConflictHistory struct {
	TaskID      string             `json:"taskId"`
	Conflicts   []ConflictInfo     `json:"conflicts"`
	Resolutions []ResolutionResult `json:"resolutions"`
	Timestamp   time.Time          `json:"timestamp"`
	TotalFiles  int                `json:"totalFiles"`
	Resolved    int                `json:"resolved"`
	Skipped     int                `json:"skipped"`
}

// NewConflictHistory 创建冲突历史.
func NewConflictHistory(taskID string) *ConflictHistory {
	return &ConflictHistory{
		TaskID:    taskID,
		Timestamp: time.Now(),
	}
}

// AddConflict 添加冲突记录.
func (h *ConflictHistory) AddConflict(conflict ConflictInfo, result ResolutionResult) {
	h.Conflicts = append(h.Conflicts, conflict)
	h.Resolutions = append(h.Resolutions, result)
	h.TotalFiles++

	switch result.Action {
	case SyncOpSkip:
		h.Skipped++
	default:
		h.Resolved++
	}
}

// Summary 生成摘要.
func (h *ConflictHistory) Summary() string {
	return fmt.Sprintf("任务 %s 冲突处理完成: 共 %d 个冲突, 已解决 %d, 已跳过 %d",
		h.TaskID, h.TotalFiles, h.Resolved, h.Skipped)
}

// MultiCloudConflictResolver 多云冲突解决器.
type MultiCloudConflictResolver struct {
	resolvers map[string]*ConflictResolver // providerID -> resolver
	history   *ConflictHistory
}

// NewMultiCloudConflictResolver 创建多云冲突解决器.
func NewMultiCloudConflictResolver() *MultiCloudConflictResolver {
	return &MultiCloudConflictResolver{
		resolvers: make(map[string]*ConflictResolver),
		history:   NewConflictHistory("multi-cloud"),
	}
}

// AddResolver 添加单个云的解决器.
func (m *MultiCloudConflictResolver) AddResolver(providerID string, task *SyncTask, provider Provider) {
	m.resolvers[providerID] = NewConflictResolver(task, provider)
}

// ResolveAll 解决所有云的冲突.
func (m *MultiCloudConflictResolver) ResolveAll(ctx context.Context, conflicts map[string][]ConflictInfo) (map[string][]ResolutionResult, error) {
	results := make(map[string][]ResolutionResult)

	for providerID, conflictList := range conflicts {
		resolver, ok := m.resolvers[providerID]
		if !ok {
			continue
		}

		providerResults := make([]ResolutionResult, 0, len(conflictList))
		for _, conflict := range conflictList {
			result, err := resolver.Resolve(ctx, &conflict)
			if err != nil {
				result = ResolutionResult{
					OriginalPath: conflict.Path,
					Action:       SyncOpSkip,
					Message:      fmt.Sprintf("解决失败: %v", err),
				}
			}
			providerResults = append(providerResults, result)
			m.history.AddConflict(conflict, result)
		}

		results[providerID] = providerResults
	}

	return results, nil
}

// GetHistory 获取冲突历史.
func (m *MultiCloudConflictResolver) GetHistory() *ConflictHistory {
	return m.history
}
