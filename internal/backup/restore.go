package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RestoreOptionsExtended 扩展恢复选项（restore 包内部使用）
type RestoreOptionsExtended struct {
	BackupID     string
	TargetPath   string
	Overwrite    bool
	Decrypt      bool
	Password     string
	Files        []string
	DryRun       bool
	VerifyAfter  bool
}

// RestoreManager 恢复管理器
type RestoreManager struct {
	backupDir  string
	storageDir string
}

// NewRestoreManager 创建恢复管理器
func NewRestoreManager(backupDir, storageDir string) *RestoreManager {
	return &RestoreManager{
		backupDir:  backupDir,
		storageDir: storageDir,
	}
}



// RestoreResult 恢复结果
type RestoreResult struct {
	Success      bool          `json:"success"`
	BackupInfo   *BackupInfo   `json:"backupInfo"`
	TargetPath   string        `json:"targetPath"`
	TotalFiles   int           `json:"totalFiles"`
	RestoredFiles int          `json:"restoredFiles"`
	TotalSize    int64         `json:"totalSize"`
	Duration     time.Duration `json:"duration"`
	Errors       []string      `json:"errors,omitempty"`
	Warnings     []string      `json:"warnings,omitempty"`
}

// Restore 一键恢复
func (rm *RestoreManager) Restore(opts RestoreOptionsExtended) (*RestoreResult, error) {
	startTime := time.Now()
	result := &RestoreResult{
		Success:    false,
		TargetPath: opts.TargetPath,
	}

	// 1. 查找备份
	backupInfo, err := rm.findBackup(opts.BackupID)
	if err != nil {
		return nil, fmt.Errorf("查找备份失败：%w", err)
	}
	result.BackupInfo = backupInfo

	// 2. 验证目标路径
	if err := rm.validateTargetPath(opts.TargetPath, opts.Overwrite); err != nil {
		return nil, err
	}

	// 3. 准备恢复
	backupPath := backupInfo.Path
	
	// 如果需要解密
	if opts.Decrypt {
		decryptedPath, err := rm.decryptBackup(backupPath, opts.Password)
		if err != nil {
			return nil, fmt.Errorf("解密失败：%w", err)
		}
		defer os.RemoveAll(decryptedPath)
		backupPath = decryptedPath
	}

	// 4. 执行恢复
	if opts.DryRun {
		// 仅预览
		preview, err := rm.previewRestore(backupPath, opts.Files)
		if err != nil {
			return nil, err
		}
		result.Warnings = append(result.Warnings, fmt.Sprintf("预览模式：将恢复 %d 个文件，总大小 %s", 
			preview.fileCount, formatSize(preview.totalSize)))
		result.Success = true
		return result, nil
	}

	// 实际恢复
	if err := rm.executeRestore(backupPath, opts.TargetPath, opts.Files, opts.Overwrite); err != nil {
		return nil, fmt.Errorf("恢复失败：%w", err)
	}

	// 5. 验证（可选）
	if opts.VerifyAfter {
		if err := rm.verifyRestore(backupPath, opts.TargetPath); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("验证警告：%v", err))
		}
	}

	// 统计信息
	stats, err := rm.getRestoreStats(opts.TargetPath)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("统计失败：%v", err))
	} else {
		result.TotalFiles = stats.fileCount
		result.RestoredFiles = stats.fileCount
		result.TotalSize = stats.totalSize
	}

	result.Duration = time.Since(startTime)
	result.Success = true

	return result, nil
}

// backupPreview 恢复预览
type backupPreview struct {
	fileCount int
	totalSize int64
	files     []string
}

// previewRestore 预览恢复
func (rm *RestoreManager) previewRestore(backupPath string, files []string) (*backupPreview, error) {
	preview := &backupPreview{}

	if len(files) > 0 {
		// 恢复指定文件
		for _, file := range files {
			fullPath := filepath.Join(backupPath, file)
			info, err := os.Stat(fullPath)
			if err != nil {
				preview.files = append(preview.files, file+" (不存在)")
				continue
			}
			if !info.IsDir() {
				preview.fileCount++
				preview.totalSize += info.Size()
				preview.files = append(preview.files, file)
			}
		}
	} else {
		// 恢复全部
		err := filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() {
				preview.fileCount++
				preview.totalSize += info.Size()
				relPath, _ := filepath.Rel(backupPath, path)
				preview.files = append(preview.files, relPath)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return preview, nil
}

// executeRestore 执行恢复
func (rm *RestoreManager) executeRestore(backupPath, targetPath string, files []string, overwrite bool) error {
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return err
	}

	if len(files) > 0 {
		// 恢复指定文件
		for _, file := range files {
			srcPath := filepath.Join(backupPath, file)
			dstPath := filepath.Join(targetPath, file)

			if err := rm.copyFileOrDir(srcPath, dstPath, overwrite); err != nil {
				return fmt.Errorf("恢复文件 %s 失败：%w", file, err)
			}
		}
	} else {
		// 恢复全部 - 使用 rsync
		args := []string{"-av", "--progress"}
		if !overwrite {
			args = append(args, "--ignore-existing")
		}
		args = append(args, backupPath+"/", targetPath+"/")

		cmd := exec.Command("rsync", args...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("rsync 失败：%w, output: %s", err, string(output))
		}
	}

	return nil
}

// copyFileOrDir 复制文件或目录
func (rm *RestoreManager) copyFileOrDir(src, dst string, overwrite bool) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	if info.IsDir() {
		if err := os.MkdirAll(dst, info.Mode()); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := rm.copyFileOrDir(
				filepath.Join(src, entry.Name()),
				filepath.Join(dst, entry.Name()),
				overwrite,
			); err != nil {
				return err
			}
		}
	} else {
		if !overwrite {
			if _, err := os.Stat(dst); err == nil {
				return nil // 跳过已存在的文件
			}
		}

		data, err := os.ReadFile(src)
		if err != nil {
			return err
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}

		return os.WriteFile(dst, data, info.Mode())
	}

	return nil
}

// verifyRestore 验证恢复结果
func (rm *RestoreManager) verifyRestore(backupPath, targetPath string) error {
	// 简单验证：检查文件数量是否一致
	var backupFiles, targetFiles int

	filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			backupFiles++
		}
		return nil
	})

	filepath.Walk(targetPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			targetFiles++
		}
		return nil
	})

	if backupFiles != targetFiles {
		return fmt.Errorf("文件数量不匹配：备份 %d 个，恢复 %d 个", backupFiles, targetFiles)
	}

	return nil
}

// decryptBackup 解密备份
func (rm *RestoreManager) decryptBackup(backupPath, password string) (string, error) {
	// 创建临时目录
	tempDir, err := os.MkdirTemp("", "backup-decrypt-*")
	if err != nil {
		return "", err
	}

	encryptor, err := NewEncryptor(password)
	if err != nil {
		return "", err
	}

	// 判断是文件还是目录
	info, err := os.Stat(backupPath)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		// 目录：直接复制（假设已解密）
		destPath := filepath.Join(tempDir, "restored")
		cmd := exec.Command("cp", "-r", backupPath, destPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("复制失败：%w, output: %s", err, string(output))
		}
		return destPath, nil
	} else {
		// 文件：解密
		decryptedPath := filepath.Join(tempDir, "decrypted")
		if err := encryptor.DecryptFile(backupPath, decryptedPath); err != nil {
			return "", err
		}
		return decryptedPath, nil
	}
}

// findBackup 查找备份
func (rm *RestoreManager) findBackup(backupID string) (*BackupInfo, error) {
	// 1. 如果是绝对路径，直接使用
	if filepath.IsAbs(backupID) {
		if _, err := os.Stat(backupID); err != nil {
			return nil, fmt.Errorf("备份路径不存在：%s", backupID)
		}
		return rm.loadBackupInfo(backupID)
	}

	// 2. 在备份目录中查找
	parts := strings.Split(backupID, "/")
	if len(parts) == 2 {
		// 格式：backupName/timestamp
		backupName := parts[0]
		timestamp := parts[1]
		backupPath := filepath.Join(rm.backupDir, backupName, timestamp)
		
		if _, err := os.Stat(backupPath); err == nil {
			return rm.loadBackupInfo(backupPath)
		}
	}

	// 3. 尝试作为备份名称查找最新备份
	backupPath := filepath.Join(rm.backupDir, backupID, "latest")
	if info, err := os.Readlink(backupPath); err == nil {
		return rm.loadBackupInfo(filepath.Join(rm.backupDir, backupID, info))
	}

	// 4. 列出所有备份供用户选择
	allBackups, err := rm.ListAllBackups()
	if err != nil {
		return nil, fmt.Errorf("备份不存在且无法列出：%s", backupID)
	}

	return nil, fmt.Errorf("备份不存在：%s，可用备份：%+v", backupID, allBackups)
}

// loadBackupInfo 加载备份信息
func (rm *RestoreManager) loadBackupInfo(backupPath string) (*BackupInfo, error) {
	metaPath := filepath.Join(backupPath, ".backup-meta.json")
	
	info := &BackupInfo{
		Path: backupPath,
		Name: filepath.Base(backupPath),
	}

	if data, err := os.ReadFile(metaPath); err == nil {
		json.Unmarshal(data, &info.Metadata)
	} else {
		// 从文件系统获取基本信息
		if stat, err := os.Stat(backupPath); err == nil {
			info.Metadata = &BackupMetadata{
				Timestamp: stat.ModTime().Format("20060102_150405"),
				Size:      stat.Size(),
			}
		}
	}

	return info, nil
}

// validateTargetPath 验证目标路径
func (rm *RestoreManager) validateTargetPath(targetPath string, overwrite bool) error {
	if targetPath == "" {
		return fmt.Errorf("目标路径不能为空")
	}

	// 检查磁盘空间
	if err := rm.checkDiskSpace(targetPath); err != nil {
		return err
	}

	// 检查是否覆盖
	if !overwrite {
		if _, err := os.Stat(targetPath); err == nil {
			// 目标已存在，检查是否为空目录
			entries, err := os.ReadDir(targetPath)
			if err == nil && len(entries) > 0 {
				return fmt.Errorf("目标目录非空，请使用 overwrite=true 或指定空目录")
			}
		}
	}

	return nil
}

// checkDiskSpace 检查磁盘空间
func (rm *RestoreManager) checkDiskSpace(path string) error {
	// 简化实现：仅检查路径是否可写
	testFile := filepath.Join(path, ".space-test-"+fmt.Sprintf("%d", time.Now().UnixNano()))
	if err := os.WriteFile(testFile, []byte(""), 0644); err != nil {
		return fmt.Errorf("磁盘空间不足或无写入权限：%w", err)
	}
	os.Remove(testFile)
	return nil
}

// getRestoreStats 获取恢复统计
func (rm *RestoreManager) getRestoreStats(path string) (*backupPreview, error) {
	stats := &backupPreview{}

	err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			stats.fileCount++
			stats.totalSize += info.Size()
		}
		return nil
	})

	return stats, err
}

// ListAllBackups 列出所有可用备份
func (rm *RestoreManager) ListAllBackups() ([]BackupInfo, error) {
	var allBackups []BackupInfo

	entries, err := os.ReadDir(rm.backupDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		backupName := entry.Name()
		ib := NewIncrementalBackup(rm.backupDir)
		backups, err := ib.ListBackups(backupName)
		if err != nil {
			continue
		}

		allBackups = append(allBackups, backups...)
	}

	// 按时间排序
	sort.Slice(allBackups, func(i, j int) bool {
		return allBackups[i].Metadata.Timestamp > allBackups[j].Metadata.Timestamp
	})

	return allBackups, nil
}

// QuickRestore 快速恢复到最新备份
func (rm *RestoreManager) QuickRestore(backupName, targetPath string) (*RestoreResult, error) {
	return rm.Restore(RestoreOptionsExtended{
		BackupID:    backupName,
		TargetPath:  targetPath,
		Overwrite:   true,
		DryRun:      false,
		VerifyAfter: true,
	})
}

// RestoreSingleFile 恢复单个文件
func (rm *RestoreManager) RestoreSingleFile(backupID, filePath, targetPath string) (*RestoreResult, error) {
	return rm.Restore(RestoreOptionsExtended{
		BackupID:   backupID,
		TargetPath: targetPath,
		Files:      []string{filePath},
		Overwrite:  true,
	})
}

// 辅助函数

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}
