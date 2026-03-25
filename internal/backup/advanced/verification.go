package advanced

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 验证器 ==========

// Verifier 备份验证器.
type Verifier struct {
	manager     *Manager
	checksumMap map[string]string // 文件路径 -> 校验和
	_           sync.RWMutex      // 保留字段以备将来使用
}

// NewVerifier 创建验证器.
func NewVerifier(manager *Manager) *Verifier {
	return &Verifier{
		manager:     manager,
		checksumMap: make(map[string]string),
	}
}

// VerifyBackup 验证备份完整性.
func (v *Verifier) VerifyBackup(ctx context.Context, backupID string) (*VerificationResult, error) {
	startTime := time.Now()

	result := &VerificationResult{
		BackupID:   backupID,
		CheckedAt:  startTime,
		TotalFiles: 0,
		ValidFiles: 0,
	}

	// 获取备份记录
	record, err := v.manager.GetRecord(backupID)
	if err != nil {
		result.Status = VerificationInvalid
		result.Error = err.Error()
		return result, err
	}

	// 获取备份清单
	manifest, err := v.manager.GetManifest(backupID)
	if err != nil {
		result.Status = VerificationInvalid
		result.Error = fmt.Sprintf("manifest not found: %v", err)
		return result, err
	}

	result.TotalFiles = int64(len(manifest.Files))

	// 验证每个文件
	var invalidFiles []FileError
	for _, file := range manifest.Files {
		select {
		case <-ctx.Done():
			result.Status = VerificationPartial
			result.Error = ctx.Err().Error()
			return result, ctx.Err()
		default:
		}

		fullPath := filepath.Join(record.Destination, file.Path)
		valid, err := v.verifyFile(fullPath, file)
		if !valid {
			invalidFiles = append(invalidFiles, FileError{
				Path:  file.Path,
				Error: fmt.Sprintf("%v", err),
				Type:  v.getErrorType(err),
			})
		} else {
			result.ValidFiles++
		}
	}

	result.InvalidFiles = invalidFiles

	// 验证整体校验和
	if record.Checksum != "" {
		actualChecksum, err := v.calculateBackupChecksum(record.Destination)
		if err != nil {
			result.Status = VerificationInvalid
			result.Error = fmt.Sprintf("checksum calculation failed: %v", err)
			return result, err
		}
		result.ChecksumMatch = (actualChecksum == record.Checksum)
	} else {
		result.ChecksumMatch = true // 没有记录校验和，跳过
	}

	// 确定状态
	if len(invalidFiles) == 0 && result.ChecksumMatch {
		result.Status = VerificationValid
	} else if result.ValidFiles > 0 {
		result.Status = VerificationPartial
	} else {
		result.Status = VerificationInvalid
	}

	result.Duration = time.Since(startTime)

	// 更新记录
	v.manager.mu.Lock()
	if record, exists := v.manager.records[backupID]; exists {
		record.Verified = (result.Status == VerificationValid)
		now := time.Now()
		record.VerifiedAt = &now
	}
	v.manager.mu.Unlock()

	return result, nil
}

// verifyFile 验证单个文件.
func (v *Verifier) verifyFile(path string, manifest FileManifest) (bool, error) {
	// 检查文件是否存在
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("file not found")
		}
		return false, err
	}

	// 检查大小
	if info.Size() != manifest.Size {
		return false, fmt.Errorf("size mismatch: expected %d, got %d", manifest.Size, info.Size())
	}

	// 计算校验和
	if manifest.Checksum != "" {
		file, err := os.Open(path)
		if err != nil {
			return false, err
		}
		defer func() { _ = file.Close() }()

		hash := sha256.New()
		if _, err := io.Copy(hash, file); err != nil {
			return false, err
		}

		actualChecksum := hex.EncodeToString(hash.Sum(nil))
		if actualChecksum != manifest.Checksum {
			return false, fmt.Errorf("checksum mismatch: expected %s, got %s", manifest.Checksum, actualChecksum)
		}
	}

	return true, nil
}

// getErrorType 获取错误类型.
func (v *Verifier) getErrorType(err error) string {
	if err == nil {
		return ""
	}
	errStr := err.Error()
	switch {
	case containsString(errStr, "not found"):
		return "missing"
	case containsString(errStr, "checksum"):
		return "checksum"
	case containsString(errStr, "size"):
		return "corrupted"
	default:
		return "unknown"
	}
}

// calculateBackupChecksum 计算备份整体校验和.
func (v *Verifier) calculateBackupChecksum(backupPath string) (string, error) {
	hash := sha256.New()

	err := filepath.Walk(backupPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()

		if _, err := io.Copy(hash, file); err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// QuickVerify 快速验证（仅检查文件是否存在和大小）.
func (v *Verifier) QuickVerify(ctx context.Context, backupID string) (*VerificationResult, error) {
	startTime := time.Now()

	result := &VerificationResult{
		BackupID:  backupID,
		CheckedAt: startTime,
	}

	record, err := v.manager.GetRecord(backupID)
	if err != nil {
		result.Status = VerificationInvalid
		result.Error = err.Error()
		return result, err
	}

	manifest, err := v.manager.GetManifest(backupID)
	if err != nil {
		result.Status = VerificationInvalid
		result.Error = fmt.Sprintf("manifest not found: %v", err)
		return result, err
	}

	result.TotalFiles = int64(len(manifest.Files))

	var invalidFiles []FileError
	for _, file := range manifest.Files {
		select {
		case <-ctx.Done():
			result.Status = VerificationPartial
			return result, ctx.Err()
		default:
		}

		fullPath := filepath.Join(record.Destination, file.Path)
		info, err := os.Stat(fullPath)
		if err != nil {
			invalidFiles = append(invalidFiles, FileError{
				Path:  file.Path,
				Error: err.Error(),
				Type:  "missing",
			})
			continue
		}

		if info.Size() != file.Size {
			invalidFiles = append(invalidFiles, FileError{
				Path:  file.Path,
				Error: "size mismatch",
				Type:  "corrupted",
			})
			continue
		}

		result.ValidFiles++
	}

	result.InvalidFiles = invalidFiles
	result.Duration = time.Since(startTime)

	if len(invalidFiles) == 0 {
		result.Status = VerificationValid
	} else if result.ValidFiles > 0 {
		result.Status = VerificationPartial
	} else {
		result.Status = VerificationInvalid
	}

	return result, nil
}

// VerifyChecksum 验证文件校验和.
func (v *Verifier) VerifyChecksum(path, expectedChecksum string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false, err
	}

	actualChecksum := hex.EncodeToString(hash.Sum(nil))
	return actualChecksum == expectedChecksum, nil
}

// ========== 完整性检查器 ==========

// IntegrityChecker 完整性检查器.
type IntegrityChecker struct {
	verifier  *Verifier
	encryptor Encryptor
}

// NewIntegrityChecker 创建完整性检查器.
func NewIntegrityChecker(verifier *Verifier) *IntegrityChecker {
	return &IntegrityChecker{
		verifier: verifier,
	}
}

// SetEncryptor 设置加密器.
func (ic *IntegrityChecker) SetEncryptor(encryptor Encryptor) {
	ic.encryptor = encryptor
}

// CheckManifest 检查清单完整性.
func (ic *IntegrityChecker) CheckManifest(manifestPath string) (*BackupManifest, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	// 尝试解析以检查是否加密
	var tempManifest struct {
		Encrypted bool `json:"encrypted"`
	}
	if err := json.Unmarshal(data, &tempManifest); err == nil && tempManifest.Encrypted {
		// 清单已加密，需要解密
		if ic.encryptor == nil {
			return nil, fmt.Errorf("manifest is encrypted but no encryptor available")
		}
		data, err = ic.encryptor.Decrypt(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt manifest: %w", err)
		}
	}

	var manifest BackupManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	// 验证清单完整性
	if manifest.ID == "" {
		return nil, fmt.Errorf("manifest missing ID")
	}
	if manifest.CreatedAt.IsZero() {
		return nil, fmt.Errorf("manifest missing creation time")
	}

	return &manifest, nil
}

// CheckBackupChain 检查备份链完整性.
func (ic *IntegrityChecker) CheckBackupChain(ctx context.Context, backupIDs []string) ([]*VerificationResult, error) {
	var results []*VerificationResult

	for _, id := range backupIDs {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		result, err := ic.verifier.QuickVerify(ctx, id)
		if err != nil {
			result = &VerificationResult{
				BackupID: id,
				Status:   VerificationInvalid,
				Error:    err.Error(),
			}
		}
		results = append(results, result)
	}

	return results, nil
}

// ValidateIncrementalChain 验证增量备份链.
func (ic *IntegrityChecker) ValidateIncrementalChain(ctx context.Context, baseID string, incrementalIDs []string) error {
	// 验证基础备份
	result, err := ic.verifier.QuickVerify(ctx, baseID)
	if err != nil {
		return fmt.Errorf("base backup verification failed: %w", err)
	}
	if result.Status != VerificationValid {
		return fmt.Errorf("base backup is invalid")
	}

	// 验证每个增量备份
	for i, incID := range incrementalIDs {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		result, err := ic.verifier.QuickVerify(ctx, incID)
		if err != nil {
			return fmt.Errorf("incremental backup %s verification failed: %w", incID, err)
		}
		if result.Status != VerificationValid {
			return fmt.Errorf("incremental backup %s is invalid", incID)
		}

		// 验证增量备份链
		if i > 0 {
			record, err := ic.verifier.manager.GetRecord(incID)
			if err != nil {
				return fmt.Errorf("failed to get record for %s: %w", incID, err)
			}
			if record.BaseBackupID != incrementalIDs[i-1] {
				return fmt.Errorf("broken incremental chain at %s", incID)
			}
		}
	}

	return nil
}

// ========== 辅助函数 ==========

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr
}
