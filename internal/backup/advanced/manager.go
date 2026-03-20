package advanced

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ========== 备份执行 ==========

// CreateBackup 创建备份
func (m *Manager) CreateBackup(ctx context.Context, backupType BackupType) (*BackupRecord, error) {
	m.mu.Lock()

	// 检查是否有运行中的备份
	for _, record := range m.records {
		if record.Status == StatusRunning {
			m.mu.Unlock()
			return nil, ErrBackupInProgress
		}
	}

	// 创建备份记录
	backupID := generateBackupID()
	record := &BackupRecord{
		ID:          backupID,
		ConfigID:    m.config.ID,
		Name:        m.config.Name,
		Type:        backupType,
		Status:      StatusPending,
		Source:      m.config.Source,
		Destination: filepath.Join(m.storagePath, backupID),
		StartTime:   time.Now(),
	}

	m.records[backupID] = record
	m.mu.Unlock()

	// 执行备份
	var err error
	switch backupType {
	case TypeFull:
		err = m.executeFullBackup(ctx, record)
	case TypeIncremental:
		err = m.executeIncrementalBackup(ctx, record)
	case TypeDifferential:
		err = m.executeDifferentialBackup(ctx, record)
	}

	m.mu.Lock()
	if err != nil {
		record.Status = StatusFailed
		record.Error = err.Error()
	} else {
		record.Status = StatusCompleted
		record.Progress = 100
	}
	record.EndTime = time.Now()
	record.Duration = int64(record.EndTime.Sub(record.StartTime).Seconds())
	m.mu.Unlock()

	// 验证备份
	if m.config.Verification && err == nil {
		m.mu.Lock()
		record.Status = StatusVerifying
		m.mu.Unlock()

		verifier := NewVerifier(m)
		result, verifyErr := verifier.QuickVerify(ctx, backupID)
		m.mu.Lock()
		if verifyErr != nil || result.Status != VerificationValid {
			record.Verified = false
			record.Status = StatusCompleted // 即使验证失败，备份也已完成
			if err == nil {
				record.Error = fmt.Sprintf("verification failed: %v", verifyErr)
			}
		} else {
			record.Verified = true
			record.Status = StatusCompleted // 验证完成，状态改回已完成
			now := time.Now()
			record.VerifiedAt = &now
		}
		m.mu.Unlock()
	}

	return record, err
}

// executeFullBackup 执行完整备份
func (m *Manager) executeFullBackup(ctx context.Context, record *BackupRecord) error {
	m.mu.Lock()
	record.Status = StatusRunning
	m.mu.Unlock()

	// 创建目标目录
	if err := os.MkdirAll(record.Destination, 0755); err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	// 创建清单
	manifest := &BackupManifest{
		ID:          record.ID,
		ConfigID:    record.ConfigID,
		CreatedAt:   time.Now(),
		Type:        TypeFull,
		Files:       make([]FileManifest, 0),
		Chunks:      make([]ChunkManifest, 0),
		Compression: m.compressor.Algorithm(),
		Encrypted:   m.config.Encryption != nil && m.config.Encryption.Enabled,
		Metadata:    make(map[string]interface{}),
	}

	var totalSize int64
	var compressedSize int64
	var fileCount int64

	// 遍历源目录
	err := filepath.Walk(m.config.Source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// 跳过目录
		if info.IsDir() {
			return nil
		}

		// 检查排除模式
		relPath, _ := filepath.Rel(m.config.Source, path)
		if m.shouldExclude(relPath) {
			return nil
		}

		// 处理文件
		fileManifest, compSize, err := m.processFile(path, relPath, info, record)
		if err != nil {
			// 记录错误但继续
			log.Printf("Warning: failed to process %s: %v", path, err)
			return nil
		}

		manifest.Files = append(manifest.Files, fileManifest)
		totalSize += info.Size()
		compressedSize += compSize
		fileCount++

		// 更新进度
		m.mu.Lock()
		record.FileCount = fileCount
		record.Size = totalSize
		record.CompressedSize = compressedSize
		record.Progress = float64(fileCount) / float64(max(fileCount+1, 1)) * 100
		m.mu.Unlock()

		return nil
	})

	if err != nil {
		return err
	}

	manifest.Size = totalSize
	manifest.Compressed = compressedSize

	// 计算整体校验和
	checksum, err := m.calculateManifestChecksum(manifest)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}
	manifest.Checksum = checksum
	record.Checksum = checksum

	// 保存清单
	if err := m.saveManifest(record.ID, manifest); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	m.mu.Lock()
	m.manifests[record.ID] = manifest
	record.FileCount = fileCount
	record.Size = totalSize
	record.CompressedSize = compressedSize
	m.mu.Unlock()

	// 更新增量索引
	m.updateIndex(manifest)

	return nil
}

// executeIncrementalBackup 执行增量备份
func (m *Manager) executeIncrementalBackup(ctx context.Context, record *BackupRecord) error {
	m.mu.Lock()
	record.Status = StatusRunning

	// 查找最近的基础备份
	var baseRecord *BackupRecord
	for _, r := range m.records {
		if r.Type == TypeFull && r.Status == StatusCompleted {
			if baseRecord == nil || r.StartTime.After(baseRecord.StartTime) {
				baseRecord = r
			}
		}
	}
	m.mu.Unlock()

	// 如果没有基础备份，执行完整备份
	if baseRecord == nil {
		record.Type = TypeFull
		return m.executeFullBackup(ctx, record)
	}

	record.BaseBackupID = baseRecord.ID

	// 创建目标目录
	if err := os.MkdirAll(record.Destination, 0755); err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	// 创建清单
	manifest := &BackupManifest{
		ID:           record.ID,
		ConfigID:     record.ConfigID,
		CreatedAt:    time.Now(),
		Type:         TypeIncremental,
		BaseBackupID: baseRecord.ID,
		Files:        make([]FileManifest, 0),
		Chunks:       make([]ChunkManifest, 0),
		Compression:  m.compressor.Algorithm(),
		Encrypted:    m.config.Encryption != nil && m.config.Encryption.Enabled,
		Metadata:     make(map[string]interface{}),
	}

	var totalSize int64
	var compressedSize int64
	var fileCount int64

	// 检测变更
	err := filepath.Walk(m.config.Source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			return nil
		}

		relPath, _ := filepath.Rel(m.config.Source, path)
		if m.shouldExclude(relPath) {
			return nil
		}

		// 检查文件是否变更
		state, exists := m.index.Get(relPath)
		changed := !exists || state.Size != info.Size() || state.ModTime.Before(info.ModTime())

		if !changed {
			return nil
		}

		// 处理变更文件
		fileManifest, compSize, err := m.processFile(path, relPath, info, record)
		if err != nil {
			log.Printf("Warning: failed to process %s: %v", path, err)
			return nil
		}

		manifest.Files = append(manifest.Files, fileManifest)
		totalSize += info.Size()
		compressedSize += compSize
		fileCount++

		m.mu.Lock()
		record.IncrementalFiles = fileCount
		record.Size = totalSize
		record.CompressedSize = compressedSize
		m.mu.Unlock()

		return nil
	})

	if err != nil {
		return err
	}

	manifest.Size = totalSize
	manifest.Compressed = compressedSize

	checksum, err := m.calculateManifestChecksum(manifest)
	if err != nil {
		return fmt.Errorf("failed to calculate checksum: %w", err)
	}
	manifest.Checksum = checksum
	record.Checksum = checksum

	if err := m.saveManifest(record.ID, manifest); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	m.mu.Lock()
	m.manifests[record.ID] = manifest
	record.FileCount = fileCount
	m.mu.Unlock()

	m.updateIndex(manifest)

	return nil
}

// executeDifferentialBackup 执行差异备份
func (m *Manager) executeDifferentialBackup(ctx context.Context, record *BackupRecord) error {
	// 差异备份与增量备份类似，但总是基于完整备份
	return m.executeIncrementalBackup(ctx, record)
}

// processFile 处理单个文件
func (m *Manager) processFile(path, relPath string, info os.FileInfo, record *BackupRecord) (FileManifest, int64, error) {
	manifest := FileManifest{
		Path:    relPath,
		Size:    info.Size(),
		Mode:    info.Mode(),
		ModTime: info.ModTime(),
	}

	// 读取文件
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest, 0, err
	}

	// 计算校验和
	hash := sha256.Sum256(data)
	manifest.Checksum = hex.EncodeToString(hash[:])

	// 压缩
	compressed, err := m.compressor.Compress(data)
	if err != nil {
		return manifest, 0, fmt.Errorf("compression failed: %w", err)
	}

	// 加密
	var finalData []byte
	if m.encryptor != nil {
		encrypted, err := m.encryptor.Encrypt(compressed)
		if err != nil {
			return manifest, 0, fmt.Errorf("encryption failed: %w", err)
		}
		finalData = encrypted
	} else {
		finalData = compressed
	}

	// 保存文件
	destPath := filepath.Join(record.Destination, relPath)
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return manifest, 0, err
	}

	if err := os.WriteFile(destPath, finalData, 0600); err != nil {
		return manifest, 0, err
	}

	return manifest, int64(len(finalData)), nil
}

// shouldExclude 检查是否应该排除
func (m *Manager) shouldExclude(path string) bool {
	for _, pattern := range m.config.ExcludePatterns {
		matched, _ := filepath.Match(pattern, path)
		if matched {
			return true
		}
		if strings.Contains(path, pattern) {
			return true
		}
	}
	return false
}

// calculateManifestChecksum 计算清单校验和
func (m *Manager) calculateManifestChecksum(manifest *BackupManifest) (string, error) {
	data, err := json.Marshal(manifest.Files)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// saveManifest 保存清单
func (m *Manager) saveManifest(backupID string, manifest *BackupManifest) error {
	manifestPath := filepath.Join(m.storagePath, backupID, "manifest.json")
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath, data, 0600)
}

// updateIndex 更新增量索引
func (m *Manager) updateIndex(manifest *BackupManifest) {
	for _, file := range manifest.Files {
		m.index.Update(file.Path, &FileState{
			Path:     file.Path,
			Checksum: file.Checksum,
			Size:     file.Size,
			ModTime:  file.ModTime,
			Mode:     file.Mode,
		})
	}
	m.index.SetBaseID(manifest.ID)
}

// ========== 备份恢复 ==========

// validatePath 安全验证路径，防止路径遍历攻击
func validatePath(basePath, relPath string) (string, error) {
	// 清理路径，移除 . 和 ..
	cleanRelPath := filepath.Clean(relPath)
	// 构建完整路径
	fullPath := filepath.Join(basePath, cleanRelPath)
	// 清理后的完整路径
	cleanFullPath := filepath.Clean(fullPath)
	// 清理后的基础路径
	cleanBasePath := filepath.Clean(basePath)

	// 验证路径在基础目录内
	if !strings.HasPrefix(cleanFullPath, cleanBasePath+string(filepath.Separator)) && cleanFullPath != cleanBasePath {
		return "", fmt.Errorf("检测到路径遍历攻击: %s", relPath)
	}

	return cleanFullPath, nil
}

// RestoreBackup 恢复备份
func (m *Manager) RestoreBackup(ctx context.Context, backupID, targetPath string, overwrite bool) (*BackupRecord, error) {
	m.mu.RLock()
	record, exists := m.records[backupID]
	if !exists {
		m.mu.RUnlock()
		return nil, ErrBackupNotFound
	}
	m.mu.RUnlock()

	manifest, err := m.GetManifest(backupID)
	if err != nil {
		return nil, err
	}

	// 如果是增量备份，先恢复基础备份
	if manifest.Type == TypeIncremental && manifest.BaseBackupID != "" {
		if err := m.restoreBaseBackup(ctx, manifest.BaseBackupID, targetPath, overwrite); err != nil {
			return nil, fmt.Errorf("failed to restore base backup: %w", err)
		}
	}

	// 恢复文件
	for _, file := range manifest.Files {
		select {
		case <-ctx.Done():
			return record, ctx.Err()
		default:
		}

		// 安全验证源路径和目标路径
		srcPath, err := validatePath(record.Destination, file.Path)
		if err != nil {
			log.Printf("Warning: invalid source path %s: %v", file.Path, err)
			continue
		}
		dstPath, err := validatePath(targetPath, file.Path)
		if err != nil {
			log.Printf("Warning: invalid destination path %s: %v", file.Path, err)
			continue
		}

		if !overwrite {
			if _, err := os.Stat(dstPath); err == nil {
				continue // 文件已存在，跳过
			}
		}

		if err := m.restoreFile(srcPath, dstPath, manifest.Encrypted); err != nil {
			log.Printf("Warning: failed to restore %s: %v", file.Path, err)
		}
	}

	return record, nil
}

// restoreBaseBackup 恢复基础备份
func (m *Manager) restoreBaseBackup(ctx context.Context, backupID, targetPath string, overwrite bool) error {
	record, err := m.GetRecord(backupID)
	if err != nil {
		return err
	}

	manifest, err := m.GetManifest(backupID)
	if err != nil {
		return err
	}

	// 递归恢复
	if manifest.Type == TypeIncremental && manifest.BaseBackupID != "" {
		if err := m.restoreBaseBackup(ctx, manifest.BaseBackupID, targetPath, overwrite); err != nil {
			return err
		}
	}

	for _, file := range manifest.Files {
		// 安全验证路径
		srcPath, err := validatePath(record.Destination, file.Path)
		if err != nil {
			log.Printf("Warning: invalid source path %s: %v", file.Path, err)
			continue
		}
		dstPath, err := validatePath(targetPath, file.Path)
		if err != nil {
			log.Printf("Warning: invalid destination path %s: %v", file.Path, err)
			continue
		}

		if !overwrite {
			if _, err := os.Stat(dstPath); err == nil {
				continue
			}
		}

		if err := m.restoreFile(srcPath, dstPath, manifest.Encrypted); err != nil {
			log.Printf("Warning: failed to restore %s: %v", file.Path, err)
		}
	}

	return nil
}

// restoreFile 恢复单个文件
func (m *Manager) restoreFile(srcPath, dstPath string, encrypted bool) error {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// 解密
	if encrypted && m.encryptor != nil {
		decrypted, err := m.encryptor.Decrypt(data)
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
		data = decrypted
	}

	// 解压
	decompressed, err := m.compressor.Decompress(data)
	if err != nil {
		return fmt.Errorf("decompression failed: %w", err)
	}

	// 写入
	if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(dstPath, decompressed, 0600)
}

// ========== 查询方法 ==========

// GetRecord 获取备份记录
func (m *Manager) GetRecord(backupID string) (*BackupRecord, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, exists := m.records[backupID]
	if !exists {
		return nil, ErrBackupNotFound
	}
	return record, nil
}

// GetManifest 获取备份清单
func (m *Manager) GetManifest(backupID string) (*BackupManifest, error) {
	m.mu.RLock()
	manifest, exists := m.manifests[backupID]
	m.mu.RUnlock()

	if exists {
		return manifest, nil
	}

	// 从文件加载
	manifestPath := filepath.Join(m.storagePath, backupID, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("manifest not found: %w", err)
	}

	manifest = &BackupManifest{}
	if err := json.Unmarshal(data, manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	m.mu.Lock()
	m.manifests[backupID] = manifest
	m.mu.Unlock()

	return manifest, nil
}

// ListRecords 列出所有备份记录
func (m *Manager) ListRecords() []*BackupRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records := make([]*BackupRecord, 0, len(m.records))
	for _, record := range m.records {
		records = append(records, record)
	}
	return records
}

// DeleteBackup 删除备份
func (m *Manager) DeleteBackup(backupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	record, exists := m.records[backupID]
	if !exists {
		return ErrBackupNotFound
	}

	// 删除文件
	if err := os.RemoveAll(record.Destination); err != nil {
		return fmt.Errorf("failed to delete backup files: %w", err)
	}

	delete(m.records, backupID)
	delete(m.manifests, backupID)

	return nil
}

// GetProgress 获取备份进度
func (m *Manager) GetProgress(backupID string) (*BackupProgress, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	record, exists := m.records[backupID]
	if !exists {
		return nil, ErrBackupNotFound
	}

	return &BackupProgress{
		BackupID:       backupID,
		Status:         record.Status,
		Progress:       record.Progress,
		FilesProcessed: record.FileCount,
		BytesProcessed: record.Size,
		StartTime:      record.StartTime,
		UpdatedAt:      time.Now(),
	}, nil
}

// ========== 辅助函数 ==========

func generateBackupID() string {
	// 使用纳秒确保唯一性
	return fmt.Sprintf("backup-%s-%d", time.Now().Format("20060102-150405"), time.Now().Nanosecond())
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// BackupStreamReader 备份流读取器
type BackupStreamReader struct {
	reader     io.Reader
	encryptor  Encryptor
	compressor Compressor
	buffer     []byte
}

// NewBackupStreamReader 创建备份流读取器
func NewBackupStreamReader(reader io.Reader, encryptor Encryptor, compressor Compressor) *BackupStreamReader {
	return &BackupStreamReader{
		reader:     reader,
		encryptor:  encryptor,
		compressor: compressor,
		buffer:     make([]byte, 32*1024),
	}
}

// Read 实现 io.Reader
func (r *BackupStreamReader) Read(p []byte) (n int, err error) {
	return r.reader.Read(p)
}

// BackupStreamWriter 备份流写入器
type BackupStreamWriter struct {
	writer     io.Writer
	encryptor  Encryptor
	compressor Compressor
}

// NewBackupStreamWriter 创建备份流写入器
func NewBackupStreamWriter(writer io.Writer, encryptor Encryptor, compressor Compressor) *BackupStreamWriter {
	return &BackupStreamWriter{
		writer:     writer,
		encryptor:  encryptor,
		compressor: compressor,
	}
}

// Write 实现 io.Writer
func (w *BackupStreamWriter) Write(p []byte) (n int, err error) {
	data := p

	// 压缩
	if w.compressor != nil {
		compressed, err := w.compressor.Compress(data)
		if err != nil {
			return 0, err
		}
		data = compressed
	}

	// 加密
	if w.encryptor != nil {
		encrypted, err := w.encryptor.Encrypt(data)
		if err != nil {
			return 0, err
		}
		data = encrypted
	}

	return w.writer.Write(data)
}
