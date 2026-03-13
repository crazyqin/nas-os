package tiering

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Migrator 数据迁移调度器
type Migrator struct {
	mu sync.RWMutex

	config PolicyEngineConfig

	// 运行状态
	running bool
	stopCh  chan struct{}

	// 并发控制
	sem chan struct{}

	// 云同步回调（可选）
	cloudSyncFunc func(sourcePath, targetPath string) error
}

// NewMigrator 创建迁移调度器
func NewMigrator(config PolicyEngineConfig) *Migrator {
	return &Migrator{
		config: config,
		stopCh: make(chan struct{}),
		sem:    make(chan struct{}, config.MaxConcurrent),
	}
}

// Start 启动迁移器
func (m *Migrator) Start() {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()
}

// Stop 停止迁移器
func (m *Migrator) Stop() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	close(m.stopCh)
}

// SetCloudSyncFunc 设置云同步回调
func (m *Migrator) SetCloudSyncFunc(fn func(sourcePath, targetPath string) error) {
	m.cloudSyncFunc = fn
}

// ==================== 文件迁移 ====================

// MigrateFile 迁移单个文件
func (m *Migrator) MigrateFile(file *MigrateFile, policy *Policy) error {
	// 获取并发槽
	m.sem <- struct{}{}
	defer func() { <-m.sem }()

	// 检查试运行模式
	if policy != nil && policy.DryRun {
		file.SourcePath = file.Path
		file.TargetPath = m.getTargetPath(file.Path, policy.TargetTier)
		file.Status = "dry_run"
		return nil
	}

	// 根据动作执行迁移
	switch policy.Action {
	case PolicyActionMove:
		return m.moveFile(file, policy)
	case PolicyActionCopy:
		return m.copyFile(file, policy)
	case PolicyActionArchive:
		return m.archiveFile(file, policy)
	case PolicyActionDelete:
		return m.deleteFile(file, policy)
	default:
		return m.moveFile(file, policy)
	}
}

// moveFile 移动文件
func (m *Migrator) moveFile(file *MigrateFile, policy *Policy) error {
	// 确定目标路径
	targetPath := m.getTargetPath(file.Path, policy.TargetTier)

	// 保留原路径信息
	file.SourcePath = file.Path
	file.TargetPath = targetPath

	// 确保目标目录存在
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 如果保留原文件，则复制
	if policy.PreserveOrigin {
		return m.copyFileInternal(file.Path, targetPath, policy.VerifyAfter)
	}

	// 移动文件
	if err := os.Rename(file.Path, targetPath); err != nil {
		// 跨文件系统移动失败时使用复制+删除
		if err := m.copyFileInternal(file.Path, targetPath, policy.VerifyAfter); err != nil {
			return err
		}
		return os.Remove(file.Path)
	}

	return nil
}

// copyFile 复制文件
func (m *Migrator) copyFile(file *MigrateFile, policy *Policy) error {
	targetPath := m.getTargetPath(file.Path, policy.TargetTier)

	file.SourcePath = file.Path
	file.TargetPath = targetPath

	return m.copyFileInternal(file.Path, targetPath, policy.VerifyAfter)
}

// copyFileInternal 内部复制实现
func (m *Migrator) copyFileInternal(sourcePath, targetPath string, verify bool) error {
	// 确保目标目录存在
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 打开源文件
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer sourceFile.Close()

	// 创建目标文件
	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer targetFile.Close()

	// 复制数据
	if _, err := io.Copy(targetFile, sourceFile); err != nil {
		return fmt.Errorf("复制文件失败: %w", err)
	}

	// 同步到磁盘
	if err := targetFile.Sync(); err != nil {
		return fmt.Errorf("同步文件失败: %w", err)
	}

	// 复制文件属性
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("获取源文件信息失败: %w", err)
	}

	// 设置修改时间
	if err := os.Chtimes(targetPath, time.Now(), sourceInfo.ModTime()); err != nil {
		return fmt.Errorf("设置修改时间失败: %w", err)
	}

	// 设置权限
	if err := os.Chmod(targetPath, sourceInfo.Mode()); err != nil {
		return fmt.Errorf("设置权限失败: %w", err)
	}

	// 验证
	if verify {
		if err := m.verifyFile(sourcePath, targetPath); err != nil {
			// 验证失败，删除目标文件
			os.Remove(targetPath)
			return fmt.Errorf("文件验证失败: %w", err)
		}
	}

	return nil
}

// archiveFile 归档文件
func (m *Migrator) archiveFile(file *MigrateFile, policy *Policy) error {
	// 如果有云同步回调，使用云同步
	if m.cloudSyncFunc != nil {
		targetPath := m.getTargetPath(file.Path, policy.TargetTier)
		file.SourcePath = file.Path
		file.TargetPath = targetPath

		if err := m.cloudSyncFunc(file.Path, targetPath); err != nil {
			return fmt.Errorf("云同步失败: %w", err)
		}

		// 归档成功后可选删除本地文件
		if !policy.PreserveOrigin {
			return os.Remove(file.Path)
		}

		return nil
	}

	// 否则使用复制
	return m.copyFile(file, policy)
}

// deleteFile 删除文件
func (m *Migrator) deleteFile(file *MigrateFile, policy *Policy) error {
	file.SourcePath = file.Path

	// 试运行模式不实际删除
	if policy.DryRun {
		file.Status = "dry_run"
		return nil
	}

	return os.Remove(file.Path)
}

// ==================== 辅助函数 ====================

// getTargetPath 获取目标路径
func (m *Migrator) getTargetPath(sourcePath string, targetTier TierType) string {
	// 基于源路径生成目标路径
	// 实际实现应根据存储层配置
	switch targetTier {
	case TierTypeSSD:
		return filepath.Join("/mnt/ssd", filepath.Base(sourcePath))
	case TierTypeHDD:
		return filepath.Join("/mnt/hdd", filepath.Base(sourcePath))
	case TierTypeCloud:
		return filepath.Join("/mnt/cloud", filepath.Base(sourcePath))
	default:
		return sourcePath
	}
}

// verifyFile 验证文件
func (m *Migrator) verifyFile(sourcePath, targetPath string) error {
	// 比较文件大小
	sourceInfo, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	targetInfo, err := os.Stat(targetPath)
	if err != nil {
		return err
	}

	if sourceInfo.Size() != targetInfo.Size() {
		return fmt.Errorf("文件大小不匹配: 源 %d, 目标 %d", sourceInfo.Size(), targetInfo.Size())
	}

	// TODO: 可以添加校验和验证

	return nil
}

// ==================== 批量迁移 ====================

// BatchMigrate 批量迁移
func (m *Migrator) BatchMigrate(files []MigrateFile, policy *Policy, progress func(int, int)) error {
	var wg sync.WaitGroup
	errCh := make(chan error, len(files))
	completed := 0
	var mu sync.Mutex

	for i := range files {
		wg.Add(1)
		go func(file *MigrateFile) {
			defer wg.Done()

			if err := m.MigrateFile(file, policy); err != nil {
				errCh <- err
				return
			}

			mu.Lock()
			completed++
			if progress != nil {
				progress(completed, len(files))
			}
			mu.Unlock()
		}(&files[i])
	}

	wg.Wait()
	close(errCh)

	// 收集错误
	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("%d 个文件迁移失败", len(errors))
	}

	return nil
}

// ==================== 估算 ====================

// EstimateMigration 估算迁移
func (m *Migrator) EstimateMigration(files []MigrateFile, targetTier TierType) (totalSize int64, fileCount int) {
	for _, file := range files {
		totalSize += file.Size
		fileCount++
	}
	return
}
