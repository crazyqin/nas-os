// Package auth 提供认证授权功能
package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// BackupCodeManager 备份码管理器.
type BackupCodeManager struct {
	mu    sync.RWMutex
	codes map[string]map[string]*BackupCode // userID -> code -> BackupCode
}

// NewBackupCodeManager 创建备份码管理器.
func NewBackupCodeManager() *BackupCodeManager {
	return &BackupCodeManager{
		codes: make(map[string]map[string]*BackupCode),
	}
}

// generateBackupCode 生成单个备份码.
func generateBackupCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// 转换为易读的格式：XXXX-XXXX
	hexStr := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s", hexStr[:8], hexStr[8:]), nil
}

// GenerateBackupCodes 生成备份码（返回未加密的明文，用于展示给用户）.
func (m *BackupCodeManager) GenerateBackupCodes(userID string, count int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if count <= 0 {
		count = 10 // 默认生成 10 个备份码
	}
	if count > 20 {
		count = 20 // 最多 20 个
	}

	// 生成备份码
	plainCodes := make([]string, count)
	m.codes[userID] = make(map[string]*BackupCode)

	for i := 0; i < count; i++ {
		code, err := generateBackupCode()
		if err != nil {
			return nil, err
		}
		plainCodes[i] = code
		m.codes[userID][code] = &BackupCode{
			Code: code,
			Used: false,
		}
	}

	return plainCodes, nil
}

// VerifyBackupCode 验证备份码.
func (m *BackupCodeManager) VerifyBackupCode(userID, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	userCodes, ok := m.codes[userID]
	if !ok {
		return fmt.Errorf("备份码不存在")
	}

	backupCode, ok := userCodes[code]
	if !ok {
		return fmt.Errorf("%s", ErrBackupCodeInvalid)
	}

	if backupCode.Used {
		return fmt.Errorf("%s", ErrBackupCodeUsed)
	}

	// 标记为已使用
	backupCode.Used = true
	now := time.Now()
	backupCode.UsedAt = &now

	return nil
}

// GetUnusedCount 获取未使用的备份码数量.
func (m *BackupCodeManager) GetUnusedCount(userID string) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userCodes, ok := m.codes[userID]
	if !ok {
		return 0
	}

	count := 0
	for _, code := range userCodes {
		if !code.Used {
			count++
		}
	}
	return count
}

// InvalidateAll 使所有备份码失效（用户重新生成时调用）.
func (m *BackupCodeManager) InvalidateAll(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.codes, userID)
}

// ListUsedCodes 列出已使用的备份码（用于审计）.
func (m *BackupCodeManager) ListUsedCodes(userID string) []BackupCode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userCodes, ok := m.codes[userID]
	if !ok {
		return nil
	}

	used := make([]BackupCode, 0)
	for _, code := range userCodes {
		if code.Used {
			used = append(used, *code)
		}
	}
	return used
}
