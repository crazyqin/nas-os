package datasecurity

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sync"
	"time"
)

// DataSecurity 数据安全模块.
type DataSecurity struct {
	mu        sync.RWMutex
	keys      map[string]*EncryptionKey
	policies  map[string]*SecurityPolicy
	auditLog  []*AuditEntry
	integrity *IntegrityChecker
	config    *Config
}

// EncryptionKey 加密密钥.
type EncryptionKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Algorithm  string     `json:"algorithm"` // aes-256-gcm, chacha20-poly1305
	KeyData    []byte     `json:"key_data"`
	IsEnabled  bool       `json:"is_enabled"`
	CreatedAt  time.Time  `json:"created_at"`
	ExpiresAt  *time.Time `json:"expires_at"`
	LastUsed   time.Time  `json:"last_used"`
	UsageCount int64      `json:"usage_count"`
}

// SecurityPolicy 安全策略.
type SecurityPolicy struct {
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	Description           string    `json:"description"`
	EncryptionEnabled     bool      `json:"encryption_enabled"`
	EncryptionAlgorithm   string    `json:"encryption_algorithm"`
	CompressionEnabled    bool      `json:"compression_enabled"`
	IntegrityCheckEnabled bool      `json:"integrity_check_enabled"`
	AccessControlEnabled  bool      `json:"access_control_enabled"`
	IsEnabled             bool      `json:"is_enabled"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// AuditEntry 审计条目.
type AuditEntry struct {
	ID        string                 `json:"id"`
	UserID    string                 `json:"user_id"`
	Action    string                 `json:"action"`
	Resource  string                 `json:"resource"`
	Details   map[string]interface{} `json:"details"`
	IPAddress string                 `json:"ip_address"`
	UserAgent string                 `json:"user_agent"`
	Timestamp time.Time              `json:"timestamp"`
}

// IntegrityChecker 完整性检查器.
type IntegrityChecker struct {
	checksums map[string]string
	algorithm string
}

// Config 配置.
type Config struct {
	DefaultAlgorithm      string        `json:"default_algorithm"`
	KeyRotationDays       int           `json:"key_rotation_days"`
	AuditRetentionDays    int           `json:"audit_retention_days"`
	MaxKeyAge             time.Duration `json:"max_key_age"`
	EncryptionEnabled     bool          `json:"encryption_enabled"`
	IntegrityCheckEnabled bool          `json:"integrity_check_enabled"`
	AuditEnabled          bool          `json:"audit_enabled"`
}

// NewDataSecurity 创建数据安全模块.
func NewDataSecurity(config *Config) *DataSecurity {
	return &DataSecurity{
		keys:     make(map[string]*EncryptionKey),
		policies: make(map[string]*SecurityPolicy),
		auditLog: make([]*AuditEntry, 0),
		integrity: &IntegrityChecker{
			checksums: make(map[string]string),
			algorithm: "sha256",
		},
		config: config,
	}
}

// CreateKey 创建加密密钥.
func (ds *DataSecurity) CreateKey(ctx context.Context, key *EncryptionKey) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	// 生成随机密钥
	keyData := make([]byte, 32) // 256 bits
	if _, err := rand.Read(keyData); err != nil {
		return err
	}

	key.KeyData = keyData
	key.CreatedAt = time.Now()
	key.IsEnabled = true
	ds.keys[key.ID] = key

	// 记录审计
	ds.addAudit("create_key", "system", key.ID, map[string]interface{}{
		"algorithm": key.Algorithm,
	})

	return nil
}

// GetKey 获取密钥.
func (ds *DataSecurity) GetKey(ctx context.Context, id string) (*EncryptionKey, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	key, exists := ds.keys[id]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", id)
	}
	return key, nil
}

// ListKeys 列出密钥.
func (ds *DataSecurity) ListKeys(ctx context.Context) []*EncryptionKey {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	var keys []*EncryptionKey
	for _, key := range ds.keys {
		keys = append(keys, key)
	}
	return keys
}

// RotateKey 轮换密钥.
func (ds *DataSecurity) RotateKey(ctx context.Context, keyID string) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	key, exists := ds.keys[keyID]
	if !exists {
		return fmt.Errorf("key not found: %s", keyID)
	}

	// 生成新密钥
	newKeyData := make([]byte, 32)
	if _, err := rand.Read(newKeyData); err != nil {
		return err
	}

	key.KeyData = newKeyData
	key.CreatedAt = time.Now()
	key.UsageCount = 0

	// 记录审计
	ds.addAudit("rotate_key", "system", keyID, nil)

	return nil
}

// Encrypt 加密数据.
func (ds *DataSecurity) Encrypt(ctx context.Context, data []byte, keyID string) ([]byte, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	key, exists := ds.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}

	if !key.IsEnabled {
		return nil, fmt.Errorf("key disabled: %s", keyID)
	}

	// 创建AES cipher
	block, err := aes.NewCipher(key.KeyData)
	if err != nil {
		return nil, err
	}

	// 使用GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// 加密数据
	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	// 更新使用统计
	key.LastUsed = time.Now()
	key.UsageCount++

	return ciphertext, nil
}

// Decrypt 解密数据.
func (ds *DataSecurity) Decrypt(ctx context.Context, ciphertext []byte, keyID string) ([]byte, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	key, exists := ds.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("key not found: %s", keyID)
	}

	if !key.IsEnabled {
		return nil, fmt.Errorf("key disabled: %s", keyID)
	}

	// 创建AES cipher
	block, err := aes.NewCipher(key.KeyData)
	if err != nil {
		return nil, err
	}

	// 使用GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 提取nonce
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// 解密数据
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	// 更新使用统计
	key.LastUsed = time.Now()
	key.UsageCount++

	return plaintext, nil
}

// AddPolicy 添加安全策略.
func (ds *DataSecurity) AddPolicy(ctx context.Context, policy *SecurityPolicy) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()
	ds.policies[policy.ID] = policy
	return nil
}

// GetPolicy 获取策略.
func (ds *DataSecurity) GetPolicy(ctx context.Context, id string) (*SecurityPolicy, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	policy, exists := ds.policies[id]
	if !exists {
		return nil, fmt.Errorf("policy not found: %s", id)
	}
	return policy, nil
}

// ListPolicies 列出策略.
func (ds *DataSecurity) ListPolicies(ctx context.Context) []*SecurityPolicy {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	var policies []*SecurityPolicy
	for _, policy := range ds.policies {
		policies = append(policies, policy)
	}
	return policies
}

// CalculateChecksum 计算校验和.
func (ds *DataSecurity) CalculateChecksum(ctx context.Context, data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// VerifyIntegrity 验证完整性.
func (ds *DataSecurity) VerifyIntegrity(ctx context.Context, filePath string, data []byte) (bool, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	expectedChecksum, exists := ds.integrity.checksums[filePath]
	if !exists {
		return false, fmt.Errorf("checksum not found for file: %s", filePath)
	}

	actualChecksum := ds.CalculateChecksum(ctx, data)
	return expectedChecksum == actualChecksum, nil
}

// StoreChecksum 存储校验和.
func (ds *DataSecurity) StoreChecksum(ctx context.Context, filePath string, data []byte) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	checksum := ds.CalculateChecksum(ctx, data)
	ds.integrity.checksums[filePath] = checksum
}

// AddAudit 添加审计记录.
func (ds *DataSecurity) AddAudit(ctx context.Context, userID, action, resource string, details map[string]interface{}) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	ds.addAudit(action, userID, resource, details)
}

// GetAuditLog 获取审计日志.
func (ds *DataSecurity) GetAuditLog(ctx context.Context, userID string, limit int) []*AuditEntry {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	var entries []*AuditEntry
	for _, entry := range ds.auditLog {
		if userID == "" || entry.UserID == userID {
			entries = append(entries, entry)
		}
	}

	// 按时间倒序排序
	sortAuditByTime(entries)

	if len(entries) > limit {
		return entries[:limit]
	}
	return entries
}

// CleanupAudit 清理审计日志.
func (ds *DataSecurity) CleanupAudit(ctx context.Context) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	retentionDate := time.Now().AddDate(0, 0, -ds.config.AuditRetentionDays)

	var filteredEntries []*AuditEntry
	for _, entry := range ds.auditLog {
		if entry.Timestamp.After(retentionDate) {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	ds.auditLog = filteredEntries
	return nil
}

// GetStats 获取统计信息.
func (ds *DataSecurity) GetStats(ctx context.Context) map[string]interface{} {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	return map[string]interface{}{
		"total_keys":          len(ds.keys),
		"total_policies":      len(ds.policies),
		"total_audit_entries": len(ds.auditLog),
		"total_checksums":     len(ds.integrity.checksums),
	}
}

// 内部方法.
func (ds *DataSecurity) addAudit(action, userID, resource string, details map[string]interface{}) {
	entry := &AuditEntry{
		ID:        generateID(),
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Details:   details,
		Timestamp: time.Now(),
	}
	ds.auditLog = append(ds.auditLog, entry)
}

// 辅助函数.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

func sortAuditByTime(entries []*AuditEntry) {
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[i].Timestamp.Before(entries[j].Timestamp) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}
}
