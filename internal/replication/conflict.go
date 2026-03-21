package replication

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ConflictStrategy 冲突解决策略
type ConflictStrategy string

// 冲突解决策略常量
const (
	ConflictSourceWins ConflictStrategy = "source_wins" // 源端优先
	ConflictTargetWins ConflictStrategy = "target_wins" // 目标端优先
	ConflictNewerWins  ConflictStrategy = "newer_wins"  // 较新优先
	ConflictLargerWins ConflictStrategy = "larger_wins" // 较大优先
	ConflictRename     ConflictStrategy = "rename"      // 重命名保留
	ConflictSkip       ConflictStrategy = "skip"        // 跳过冲突文件
	ConflictManual     ConflictStrategy = "manual"      // 手动解决
)

// ConflictInfo 冲突信息
type ConflictInfo struct {
	ID             string           `json:"id"`
	TaskID         string           `json:"task_id"`
	RelativePath   string           `json:"relative_path"`
	SourcePath     string           `json:"source_path"`
	TargetPath     string           `json:"target_path"`
	SourceSize     int64            `json:"source_size"`
	TargetSize     int64            `json:"target_size"`
	SourceModTime  time.Time        `json:"source_mod_time"`
	TargetModTime  time.Time        `json:"target_mod_time"`
	SourceHash     string           `json:"source_hash"`
	TargetHash     string           `json:"target_hash"`
	Strategy       ConflictStrategy `json:"strategy"`
	Resolved       bool             `json:"resolved"`
	ResolutionPath string           `json:"resolution_path,omitempty"`
	CreatedAt      time.Time        `json:"created_at"`
	ResolvedAt     time.Time        `json:"resolved_at,omitempty"`
}

// ConflictDetector 冲突检测器
type ConflictDetector struct {
	defaultStrategy ConflictStrategy
	conflicts       map[string]*ConflictInfo
}

// NewConflictDetector 创建冲突检测器
func NewConflictDetector(strategy ConflictStrategy) *ConflictDetector {
	return &ConflictDetector{
		defaultStrategy: strategy,
		conflicts:       make(map[string]*ConflictInfo),
	}
}

// DetectConflict 检测文件冲突
func (d *ConflictDetector) DetectConflict(task *Task, relativePath string) (*ConflictInfo, error) {
	sourcePath := filepath.Join(task.SourcePath, relativePath)
	targetPath := filepath.Join(task.TargetPath, relativePath)

	// 如果目标端不存在，无冲突
	if _, err := os.Stat(targetPath); os.IsNotExist(err) {
		return nil, nil
	}

	// 获取源文件信息
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("获取源文件信息失败：%w", err)
	}

	// 获取目标文件信息
	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return nil, fmt.Errorf("获取目标文件信息失败：%w", err)
	}

	// 如果都是目录，无冲突
	if sourceInfo.IsDir() && targetInfo.IsDir() {
		return nil, nil
	}

	// 类型不同，产生冲突
	if sourceInfo.IsDir() != targetInfo.IsDir() {
		return &ConflictInfo{
			ID:            generateConflictID(),
			TaskID:        task.ID,
			RelativePath:  relativePath,
			SourcePath:    sourcePath,
			TargetPath:    targetPath,
			SourceSize:    sourceInfo.Size(),
			TargetSize:    targetInfo.Size(),
			SourceModTime: sourceInfo.ModTime(),
			TargetModTime: targetInfo.ModTime(),
			Strategy:      d.defaultStrategy,
			CreatedAt:     time.Now(),
		}, nil
	}

	// 比较修改时间和大小
	sourceMod := sourceInfo.ModTime()
	targetMod := targetInfo.ModTime()
	sourceSize := sourceInfo.Size()
	targetSize := targetInfo.Size()

	// 如果修改时间和大小都相同，计算哈希确认
	if sourceMod.Equal(targetMod) && sourceSize == targetSize {
		sourceHash, err := d.calculateFileHash(sourcePath)
		if err != nil {
			return nil, err
		}
		targetHash, err := d.calculateFileHash(targetPath)
		if err != nil {
			return nil, err
		}

		// 内容相同，无冲突
		if sourceHash == targetHash {
			return nil, nil
		}

		// 内容不同，产生冲突
		return &ConflictInfo{
			ID:            generateConflictID(),
			TaskID:        task.ID,
			RelativePath:  relativePath,
			SourcePath:    sourcePath,
			TargetPath:    targetPath,
			SourceSize:    sourceSize,
			TargetSize:    targetSize,
			SourceModTime: sourceMod,
			TargetModTime: targetMod,
			SourceHash:    sourceHash,
			TargetHash:    targetHash,
			Strategy:      d.defaultStrategy,
			CreatedAt:     time.Now(),
		}, nil
	}

	// 修改时间或大小不同，计算哈希确认内容差异
	sourceHash, err := d.calculateFileHash(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("计算源文件哈希失败：%w", err)
	}
	targetHash, err := d.calculateFileHash(targetPath)
	if err != nil {
		return nil, fmt.Errorf("计算目标文件哈希失败：%w", err)
	}

	// 内容相同，无冲突
	if sourceHash == targetHash {
		return nil, nil
	}

	// 内容不同，产生冲突
	return &ConflictInfo{
		ID:            generateConflictID(),
		TaskID:        task.ID,
		RelativePath:  relativePath,
		SourcePath:    sourcePath,
		TargetPath:    targetPath,
		SourceSize:    sourceSize,
		TargetSize:    targetSize,
		SourceModTime: sourceMod,
		TargetModTime: targetMod,
		SourceHash:    sourceHash,
		TargetHash:    targetHash,
		Strategy:      d.defaultStrategy,
		CreatedAt:     time.Now(),
	}, nil
}

// ResolveConflict 解决冲突
func (d *ConflictDetector) ResolveConflict(conflict *ConflictInfo) error {
	switch conflict.Strategy {
	case ConflictSourceWins:
		return d.resolveSourceWins(conflict)
	case ConflictTargetWins:
		return d.resolveTargetWins(conflict)
	case ConflictNewerWins:
		if conflict.SourceModTime.After(conflict.TargetModTime) {
			return d.resolveSourceWins(conflict)
		}
		return d.resolveTargetWins(conflict)
	case ConflictLargerWins:
		if conflict.SourceSize > conflict.TargetSize {
			return d.resolveSourceWins(conflict)
		}
		return d.resolveTargetWins(conflict)
	case ConflictRename:
		return d.resolveRename(conflict)
	case ConflictSkip:
		conflict.Resolved = true
		conflict.ResolvedAt = time.Now()
		return nil
	case ConflictManual:
		// 手动解决需要用户干预，标记为未解决
		return fmt.Errorf("冲突需要手动解决")
	default:
		return fmt.Errorf("未知的冲突解决策略：%s", conflict.Strategy)
	}
}

// resolveSourceWins 源端优先解决
func (d *ConflictDetector) resolveSourceWins(conflict *ConflictInfo) error {
	// 备份目标文件（可选）
	if conflict.Strategy == ConflictRename {
		backupPath := conflict.TargetPath + ".conflict." + time.Now().Format("20060102-150405")
		if err := os.Rename(conflict.TargetPath, backupPath); err != nil {
			return fmt.Errorf("备份目标文件失败：%w", err)
		}
		conflict.ResolutionPath = backupPath
	}

	// 复制源文件到目标
	if err := copyFile(conflict.SourcePath, conflict.TargetPath); err != nil {
		return fmt.Errorf("复制源文件失败：%w", err)
	}

	conflict.Resolved = true
	conflict.ResolvedAt = time.Now()
	return nil
}

// resolveTargetWins 目标端优先解决
func (d *ConflictDetector) resolveTargetWins(conflict *ConflictInfo) error {
	conflict.Resolved = true
	conflict.ResolvedAt = time.Now()
	return nil
}

// resolveRename 重命名解决
func (d *ConflictDetector) resolveRename(conflict *ConflictInfo) error {
	// 重命名目标文件
	ext := filepath.Ext(conflict.TargetPath)
	base := strings.TrimSuffix(filepath.Base(conflict.TargetPath), ext)
	dir := filepath.Dir(conflict.TargetPath)
	newName := fmt.Sprintf("%s_conflict_%s%s", base, time.Now().Format("20060102-150405"), ext)
	newPath := filepath.Join(dir, newName)

	if err := os.Rename(conflict.TargetPath, newPath); err != nil {
		return fmt.Errorf("重命名目标文件失败：%w", err)
	}

	conflict.ResolutionPath = newPath

	// 复制源文件
	if err := copyFile(conflict.SourcePath, conflict.TargetPath); err != nil {
		return fmt.Errorf("复制源文件失败：%w", err)
	}

	conflict.Resolved = true
	conflict.ResolvedAt = time.Now()
	return nil
}

// calculateFileHash 计算文件哈希（使用 SHA256）
func (d *ConflictDetector) calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
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

// GetConflicts 获取所有冲突
func (d *ConflictDetector) GetConflicts(taskID string) []*ConflictInfo {
	var conflicts []*ConflictInfo
	for _, c := range d.conflicts {
		if taskID == "" || c.TaskID == taskID {
			conflicts = append(conflicts, c)
		}
	}
	return conflicts
}

// AddConflict 添加冲突
func (d *ConflictDetector) AddConflict(conflict *ConflictInfo) {
	d.conflicts[conflict.ID] = conflict
}

// generateConflictID 生成冲突 ID
func generateConflictID() string {
	return fmt.Sprintf("conflict-%d", time.Now().UnixNano())
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sourceFile.Close() }()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	// 复制权限
	sourceInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.Chmod(dst, sourceInfo.Mode())
}
