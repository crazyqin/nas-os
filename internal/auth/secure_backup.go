package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// SecureBackupCodeManager 安全备份码管理器
// 使用 bcrypt 哈希存储备份码，防止明文泄露
type SecureBackupCodeManager struct {
	mu         sync.RWMutex
	codes      map[string][]*HashedBackupCode // userID -> hashed codes
	configPath string
	encryption *SecretEncryption
}

// HashedBackupCode 哈希存储的备份码
type HashedBackupCode struct {
	Hash      string     `json:"hash"` // bcrypt 哈希
	CreatedAt time.Time  `json:"created_at"`
	Used      bool       `json:"used"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}

// NewSecureBackupCodeManager 创建安全备份码管理器
func NewSecureBackupCodeManager(configPath string, encryption *SecretEncryption) *SecureBackupCodeManager {
	m := &SecureBackupCodeManager{
		codes:      make(map[string][]*HashedBackupCode),
		configPath: configPath,
		encryption: encryption,
	}
	m.load()
	return m
}

// load 加载已存储的备份码
func (m *SecureBackupCodeManager) load() {
	if m.configPath == "" {
		return
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return
	}

	var stored map[string][]*HashedBackupCode
	if err := json.Unmarshal(data, &stored); err != nil {
		return
	}

	m.codes = stored
}

// save 保存备份码
func (m *SecureBackupCodeManager) save() error {
	if m.configPath == "" {
		return nil
	}

	data, err := json.MarshalIndent(m.codes, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0700); err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0600)
}

// generateBackupCode 生成备份码
func generateSecureBackupCode() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	hexStr := hex.EncodeToString(b)
	return fmt.Sprintf("%s-%s", hexStr[:8], hexStr[8:]), nil
}

// hashBackupCode 使用 bcrypt 哈希备份码
func hashBackupCode(code string) (string, error) {
	// 先 SHA256 减少长度，再 bcrypt
	hash := sha256.Sum256([]byte(code))
	result, err := bcrypt.GenerateFromPassword([]byte(hex.EncodeToString(hash[:])), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(result), nil
}

// verifyBackupCodeHash 验证备份码哈希
func verifyBackupCodeHash(code, hash string) bool {
	codeHash := sha256.Sum256([]byte(code))
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(hex.EncodeToString(codeHash[:])))
	return err == nil
}

// GenerateBackupCodes 生成备份码
// 返回明文备份码（仅此一次），存储哈希值
func (m *SecureBackupCodeManager) GenerateBackupCodes(userID string, count int) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if count <= 0 {
		count = 10
	}
	if count > 20 {
		count = 20
	}

	plainCodes := make([]string, count)
	m.codes[userID] = make([]*HashedBackupCode, 0, count)

	now := time.Now()
	for i := 0; i < count; i++ {
		code, err := generateSecureBackupCode()
		if err != nil {
			return nil, err
		}

		hash, err := hashBackupCode(code)
		if err != nil {
			return nil, err
		}

		plainCodes[i] = code
		m.codes[userID] = append(m.codes[userID], &HashedBackupCode{
			Hash:      hash,
			CreatedAt: now,
			Used:      false,
		})
	}

	if err := m.save(); err != nil {
		return nil, err
	}

	return plainCodes, nil
}

// VerifyBackupCode 验证备份码
func (m *SecureBackupCodeManager) VerifyBackupCode(userID, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	userCodes, ok := m.codes[userID]
	if !ok {
		return fmt.Errorf("%s", ErrBackupCodeInvalid)
	}

	for _, backupCode := range userCodes {
		if backupCode.Used {
			continue
		}

		if verifyBackupCodeHash(code, backupCode.Hash) {
			backupCode.Used = true
			now := time.Now()
			backupCode.UsedAt = &now
			m.save()
			return nil
		}
	}

	return fmt.Errorf("%s", ErrBackupCodeInvalid)
}

// GetUnusedCount 获取未使用的备份码数量
func (m *SecureBackupCodeManager) GetUnusedCount(userID string) int {
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

// InvalidateAll 使所有备份码失效
func (m *SecureBackupCodeManager) InvalidateAll(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.codes, userID)
	m.save()
}

// ListUsedCodes 列出已使用的备份码（用于审计）
func (m *SecureBackupCodeManager) ListUsedCodes(userID string) []HashedBackupCode {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userCodes, ok := m.codes[userID]
	if !ok {
		return nil
	}

	used := make([]HashedBackupCode, 0)
	for _, code := range userCodes {
		if code.Used {
			used = append(used, *code)
		}
	}
	return used
}

// GetStats 获取统计信息
func (m *SecureBackupCodeManager) GetStats(userID string) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	userCodes, ok := m.codes[userID]
	if !ok {
		return map[string]interface{}{
			"total":  0,
			"used":   0,
			"unused": 0,
		}
	}

	total := len(userCodes)
	used := 0
	for _, code := range userCodes {
		if code.Used {
			used++
		}
	}

	return map[string]interface{}{
		"total":  total,
		"used":   used,
		"unused": total - used,
	}
}
