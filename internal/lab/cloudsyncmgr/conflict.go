package cloudsyncmgr

import (
	"fmt"
	"path/filepath"
	"time"
)

// ConflictResolver 冲突检测和解决.
type ConflictResolver struct {
	policy ConflictPolicy
}

// NewConflictResolver 创建冲突解决器.
func NewConflictResolver(policy ConflictPolicy) *ConflictResolver {
	return &ConflictResolver{policy: policy}
}

// Detect 检测两个文件是否冲突.
func (cr *ConflictResolver) Detect(local, remote *FileInfo) *ConflictInfo {
	if local == nil || remote == nil {
		return nil
	}

	// 同路径、同时修改 = 冲突
	if local.ModTime.After(time.Time{}) && remote.ModTime.After(time.Time{}) {
		// 双方都有修改才算冲突 (简化: 比较 mod time 不同)
		if local.ModTime != remote.ModTime {
			// 如果文件大小和修改时间都不同，则存在冲突
			if local.Size != remote.Size {
				return &ConflictInfo{
					LocalPath:     local.Path,
					RemotePath:    remote.Path,
					LocalModTime:  local.ModTime,
					RemoteModTime: remote.ModTime,
					LocalSize:     local.Size,
					RemoteSize:    remote.Size,
					DetectedAt:    time.Now(),
				}
			}
		}
	}
	return nil
}

// Resolve 根据策略解决冲突，返回应采用的文件和操作.
func (cr *ConflictResolver) Resolve(conflict *ConflictInfo) (useLocal bool, rename string, err error) {
	switch cr.policy {
	case ConflictLocalWins:
		return true, "", nil
	case ConflictRemoteWins:
		return false, "", nil
	case ConflictNewest:
		if conflict.LocalModTime.After(conflict.RemoteModTime) {
			return true, "", nil
		}
		return false, "", nil
	case ConflictRename:
		ts := time.Now().Format("20060102_150405")
		ext := filepath.Ext(conflict.LocalPath)
		base := conflict.LocalPath[:len(conflict.LocalPath)-len(ext)]
		suggestedName := fmt.Sprintf("%s_conflict_%s%s", base, ts, ext)
		return true, suggestedName, nil
	case ConflictManual:
		return false, "", fmt.Errorf("冲突需要手动解决: %s vs %s", conflict.LocalPath, conflict.RemotePath)
	default:
		return false, "", fmt.Errorf("未知冲突策略: %s", cr.policy)
	}
}

// ResolveBatch 批量解决冲突.
func (cr *ConflictResolver) ResolveBatch(conflicts []*ConflictInfo) []ResolvedConflict {
	results := make([]ResolvedConflict, 0, len(conflicts))
	for _, c := range conflicts {
		useLocal, rename, err := cr.Resolve(c)
		results = append(results, ResolvedConflict{
			Conflict: c,
			UseLocal: useLocal,
			RenameTo: rename,
			Error:    err,
		})
	}
	return results
}

// ResolvedConflict 冲突解决结果.
type ResolvedConflict struct {
	Conflict *ConflictInfo `json:"conflict"`
	UseLocal bool          `json:"use_local"`
	RenameTo string        `json:"rename_to,omitempty"`
	Error    error         `json:"error,omitempty"`
}

// String 返回冲突解决描述.
func (rc ResolvedConflict) String() string {
	if rc.Error != nil {
		return fmt.Sprintf("未解决: %v", rc.Error)
	}
	if rc.RenameTo != "" {
		return fmt.Sprintf("重命名为: %s", rc.RenameTo)
	}
	if rc.UseLocal {
		return "使用本地版本"
	}
	return "使用云端版本"
}
