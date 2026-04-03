// Package cloudsync OAuth安全配置和凭证保护
package cloudsync

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"
)

// OAuthSecurityManager OAuth凭证安全管理器
type OAuthSecurityManager struct {
	config      OAuthSecurityConfig
	masterKey   []byte
	tokenStore  map[string]*SecureToken
	auditLogger OAuthAuditLogger
	mu          sync.RWMutex
}

// OAuthSecurityConfig OAuth安全配置
type OAuthSecurityConfig struct {
	Enabled            bool          `json:"enabled"`
	TokenEncrypt       bool          `json:"token_encrypt"`       // Token加密存储
	KeyRotationDays    int           `json:"key_rotation_days"`   // 密钥轮转周期(天)
	TokenExpiryMargin  time.Duration `json:"token_expiry_margin"` // Token过期提前刷新时间
	MaxTokenAge        time.Duration `json:"max_token_age"`       // Token最大有效期
	SecureStoragePath  string        `json:"secure_storage_path"` // 安全存储路径
	KeyDerivationAlg   string        `json:"key_derivation_alg"`  // 密钥派生算法
	AutoRotate         bool          `json:"auto_rotate"`         // 自动密钥轮转
	AuditEnabled       bool          `json:"audit_enabled"`       // 审计日志
}

// SecureToken 安全存储的Token
type SecureToken struct {
	ID           string    `json:"id"`
	ProviderID   string    `json:"provider_id"`
	ProviderType ProviderType `json:"provider_type"`
	TokenType    string    `json:"token_type"` // access_token, refresh_token, api_key
	EncryptedData []byte   `json:"encrypted_data"`
	Nonce        []byte    `json:"nonce"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	KeyVersion   int       `json:"key_version"` // 密钥版本号
	LastUsed     time.Time `json:"last_used"`
	UsageCount   int64     `json:"usage_count"`
}

// OAuthToken OAuth Token结构
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresIn    int       `json:"expires_in,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	Scope        string    `json:"scope,omitempty"`
}

// APIKeyRecord API密钥记录
type APIKeyRecord struct {
	KeyID       string    `json:"key_id"`
	ProviderID  string    `json:"provider_id"`
	KeyType     string    `json:"key_type"` // access_key, secret_key, api_key
	EncryptedKey []byte   `json:"encrypted_key"`
	Nonce       []byte    `json:"nonce"`
	KeyVersion  int       `json:"key_version"`
	CreatedAt   time.Time `json:"created_at"`
	RotatedAt   time.Time `json:"rotated_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	RotationDue time.Time `json:"rotation_due"`
	LastUsed    time.Time `json:"last_used"`
	Status      string    `json:"status"` // active, rotated, expired, revoked
}

// KeyRotationRecord 密钥轮转记录
type KeyRotationRecord struct {
	ID          string    `json:"id"`
	KeyID       string    `json:"key_id"`
	OldVersion  int       `json:"old_version"`
	NewVersion  int       `json:"new_version"`
	RotatedAt   time.Time `json:"rotated_at"`
	Reason      string    `json:"reason"`
	InitiatedBy string    `json:"initiated_by"`
}

// OAuthAuditLogger OAuth审计日志接口
type OAuthAuditLogger interface {
	LogTokenOperation(op OAuthAuditOperation, details map[string]interface{})
	LogKeyRotation(record *KeyRotationRecord)
	LogSecurityEvent(event string, severity string, details map[string]interface{})
}

// OAuthAuditOperation OAuth审计操作类型
type OAuthAuditOperation string

const (
	OAuthOpTokenStore    OAuthAuditOperation = "token_store"
	OAuthOpTokenRetrieve OAuthAuditOperation = "token_retrieve"
	OAuthOpTokenRefresh  OAuthAuditOperation = "token_refresh"
	OAuthOpTokenExpire   OAuthAuditOperation = "token_expire"
	OAuthOpTokenRevoke   OAuthAuditOperation = "token_revoke"
	OAuthOpKeyRotate     OAuthAuditOperation = "key_rotate"
	OAuthOpKeyCreate     OAuthAuditOperation = "key_create"
	OAuthOpKeyDelete     OAuthAuditOperation = "key_delete"
)

// DefaultOAuthSecurityConfig 默认OAuth安全配置
func DefaultOAuthSecurityConfig() OAuthSecurityConfig {
	return OAuthSecurityConfig{
		Enabled:           true,
		TokenEncrypt:      true,
		KeyRotationDays:   90,
		TokenExpiryMargin: 10 * time.Minute,
		MaxTokenAge:       30 * 24 * time.Hour,
		SecureStoragePath: "/var/lib/nas-os/cloudsync/oauth_tokens",
		KeyDerivationAlg:  "argon2id",
		AutoRotate:        true,
		AuditEnabled:      true,
	}
}

// NewOAuthSecurityManager 创建OAuth安全管理器
func NewOAuthSecurityManager(config OAuthSecurityConfig, auditLogger OAuthAuditLogger) (*OAuthSecurityManager, error) {
	// 确保存储目录存在
	if err := os.MkdirAll(config.SecureStoragePath, 0700); err != nil {
		return nil, fmt.Errorf("创建安全存储目录失败: %w", err)
	}

	manager := &OAuthSecurityManager{
		config:      config,
		tokenStore:  make(map[string]*SecureToken),
		auditLogger: auditLogger,
	}

	// 初始化或加载主密钥
	if err := manager.initMasterKey(); err != nil {
		return nil, fmt.Errorf("初始化主密钥失败: %w", err)
	}

	// 加载现有Token
	if err := manager.loadTokens(); err != nil {
		// 首次启动时可能没有Token，忽略错误
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("加载Token失败: %w", err)
		}
	}

	// 启动自动轮转检查
	if config.AutoRotate {
		go manager.rotationCheckLoop()
	}

	return manager, nil
}

// initMasterKey 初始化主密钥
func (m *OAuthSecurityManager) initMasterKey() error {
	keyPath := filepath.Join(m.config.SecureStoragePath, "master.key")
	saltPath := filepath.Join(m.config.SecureStoragePath, "salt")

	// 加载或生成盐值
	salt, err := m.loadOrGenerateFile(saltPath, 16)
	if err != nil {
		return err
	}

	// 检查是否已有主密钥
	if data, err := os.ReadFile(keyPath); err == nil {
		// 从文件加载加密的主密钥，使用系统密钥解密
		systemKey := m.deriveSystemKey(salt)
		m.masterKey, err = m.decryptData(data, systemKey)
		if err != nil {
			return fmt.Errorf("解密主密钥失败: %w", err)
		}
		return nil
	}

	// 生成新的主密钥 (AES-256 需要 32字节)
	m.masterKey = make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, m.masterKey); err != nil {
		return fmt.Errorf("生成主密钥失败: %w", err)
	}

	// 加密并保存主密钥
	systemKey := m.deriveSystemKey(salt)
	encryptedKey, err := m.encryptData(m.masterKey, systemKey)
	if err != nil {
		return fmt.Errorf("加密主密钥失败: %w", err)
	}

	return os.WriteFile(keyPath, encryptedKey, 0600)
}

// deriveSystemKey 从系统信息派生密钥
func (m *OAuthSecurityManager) deriveSystemKey(salt []byte) []byte {
	// 使用机器ID等系统信息作为密钥材料
	systemInfo := fmt.Sprintf("%s-%d", m.config.SecureStoragePath, time.Now().UnixNano())
	
	// Argon2id 参数
	timeCost := uint32(3)
	memory := uint32(64 * 1024) // 64MB
	threads := uint8(4)
	keyLen := uint32(32)

	return argon2.IDKey([]byte(systemInfo), salt, timeCost, memory, threads, keyLen)
}

// loadOrGenerateFile 加载或生成文件
func (m *OAuthSecurityManager) loadOrGenerateFile(path string, size int) ([]byte, error) {
	if data, err := os.ReadFile(path); err == nil {
		return data, nil
	}

	data := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, data); err != nil {
		return nil, err
	}

	return data, os.WriteFile(path, data, 0600)
}

// ==================== Token 存储 ====================

// StoreToken 安全存储OAuth Token
func (m *OAuthSecurityManager) StoreToken(providerID string, providerType ProviderType, token *OAuthToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.Enabled || !m.config.TokenEncrypt {
		return fmt.Errorf("Token加密存储未启用")
	}

	// 序列化Token
	tokenData, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("序列化Token失败: %w", err)
	}

	// 加密Token
	encryptedData, nonce, err := m.encryptWithNonce(tokenData)
	if err != nil {
		return fmt.Errorf("加密Token失败: %w", err)
	}

	// 创建安全Token记录
	secureToken := &SecureToken{
		ID:            generateTokenID(providerID, "access_token"),
		ProviderID:    providerID,
		ProviderType:  providerType,
		TokenType:     "access_token",
		EncryptedData: encryptedData,
		Nonce:         nonce,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ExpiresAt:     &token.ExpiresAt,
		KeyVersion:    1,
	}

	// 存储Refresh Token
	if token.RefreshToken != "" {
		refreshData, _ := json.Marshal(map[string]string{"refresh_token": token.RefreshToken})
		encryptedRefresh, refreshNonce, err := m.encryptWithNonce(refreshData)
		if err == nil {
			refreshToken := &SecureToken{
				ID:            generateTokenID(providerID, "refresh_token"),
				ProviderID:    providerID,
				ProviderType:  providerType,
				TokenType:     "refresh_token",
				EncryptedData: encryptedRefresh,
				Nonce:         refreshNonce,
				CreatedAt:     time.Now(),
				KeyVersion:    1,
			}
			m.tokenStore[refreshToken.ID] = refreshToken
		}
	}

	m.tokenStore[secureToken.ID] = secureToken

	// 保存到文件
	if err := m.saveTokensLocked(); err != nil {
		return err
	}

	// 审计日志
	if m.config.AuditEnabled && m.auditLogger != nil {
		m.auditLogger.LogTokenOperation(OAuthOpTokenStore, map[string]interface{}{
			"provider_id":   providerID,
			"provider_type": providerType,
			"token_type":    "access_token",
			"expires_at":    token.ExpiresAt,
		})
	}

	return nil
}

// RetrieveToken 获取OAuth Token
func (m *OAuthSecurityManager) RetrieveToken(providerID string) (*OAuthToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tokenID := generateTokenID(providerID, "access_token")
	secureToken, exists := m.tokenStore[tokenID]
	if !exists {
		return nil, fmt.Errorf("Token不存在: %s", providerID)
	}

	// 解密Token
	tokenData, err := m.decryptWithNonce(secureToken.EncryptedData, secureToken.Nonce)
	if err != nil {
		return nil, fmt.Errorf("解密Token失败: %w", err)
	}

	var token OAuthToken
	if err := json.Unmarshal(tokenData, &token); err != nil {
		return nil, fmt.Errorf("解析Token失败: %w", err)
	}

	// 检查是否过期
	if secureToken.ExpiresAt != nil && time.Now().After(*secureToken.ExpiresAt) {
		if m.config.AuditEnabled && m.auditLogger != nil {
			m.auditLogger.LogTokenOperation(OAuthOpTokenExpire, map[string]interface{}{
				"provider_id": providerID,
				"expires_at":  *secureToken.ExpiresAt,
			})
		}
		return nil, fmt.Errorf("Token已过期")
	}

	// 更新使用记录
	m.mu.Lock()
	secureToken.LastUsed = time.Now()
	secureToken.UsageCount++
	m.mu.Unlock()

	// 审计日志
	if m.config.AuditEnabled && m.auditLogger != nil {
		m.auditLogger.LogTokenOperation(OAuthOpTokenRetrieve, map[string]interface{}{
			"provider_id": providerID,
			"usage_count": secureToken.UsageCount,
		})
	}

	return &token, nil
}

// RetrieveRefreshToken 获取Refresh Token
func (m *OAuthSecurityManager) RetrieveRefreshToken(providerID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tokenID := generateTokenID(providerID, "refresh_token")
	secureToken, exists := m.tokenStore[tokenID]
	if !exists {
		return "", fmt.Errorf("Refresh Token不存在")
	}

	tokenData, err := m.decryptWithNonce(secureToken.EncryptedData, secureToken.Nonce)
	if err != nil {
		return "", fmt.Errorf("解密Refresh Token失败: %w", err)
	}

	var data map[string]string
	if err := json.Unmarshal(tokenData, &data); err != nil {
		return "", err
	}

	return data["refresh_token"], nil
}

// DeleteToken 删除Token
func (m *OAuthSecurityManager) DeleteToken(providerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	accessID := generateTokenID(providerID, "access_token")
	refreshID := generateTokenID(providerID, "refresh_token")

	delete(m.tokenStore, accessID)
	delete(m.tokenStore, refreshID)

	if err := m.saveTokensLocked(); err != nil {
		return err
	}

	if m.config.AuditEnabled && m.auditLogger != nil {
		m.auditLogger.LogTokenOperation(OAuthOpTokenRevoke, map[string]interface{}{
			"provider_id": providerID,
		})
	}

	return nil
}

// ==================== API密钥存储 ====================

// StoreAPIKey 安全存储API密钥
func (m *OAuthSecurityManager) StoreAPIKey(providerID string, keyType string, keyValue string) (*APIKeyRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 加密密钥
	encryptedKey, nonce, err := m.encryptWithNonce([]byte(keyValue))
	if err != nil {
		return nil, fmt.Errorf("加密API密钥失败: %w", err)
	}

	keyID := generateKeyID(providerID, keyType)
	rotationDue := time.Now().AddDate(0, 0, m.config.KeyRotationDays)

	record := &APIKeyRecord{
		KeyID:        keyID,
		ProviderID:   providerID,
		KeyType:      keyType,
		EncryptedKey: encryptedKey,
		Nonce:        nonce,
		KeyVersion:   1,
		CreatedAt:    time.Now(),
		RotatedAt:    time.Now(),
		RotationDue:  rotationDue,
		Status:       "active",
	}

	// 保存记录
	if err := m.saveAPIKeyRecord(record); err != nil {
		return nil, err
	}

	if m.config.AuditEnabled && m.auditLogger != nil {
		m.auditLogger.LogTokenOperation(OAuthOpKeyCreate, map[string]interface{}{
			"provider_id": providerID,
			"key_type":    keyType,
			"key_id":      keyID,
		})
	}

	return record, nil
}

// RetrieveAPIKey 获取API密钥
func (m *OAuthSecurityManager) RetrieveAPIKey(providerID string, keyType string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keyID := generateKeyID(providerID, keyType)
	record, err := m.loadAPIKeyRecord(keyID)
	if err != nil {
		return "", err
	}

	// 检查密钥状态
	if record.Status != "active" {
		return "", fmt.Errorf("密钥状态异常: %s", record.Status)
	}

	// 检查是否需要轮转
	if time.Now().After(record.RotationDue) {
		return "", fmt.Errorf("密钥已过期需要轮转")
	}

	// 解密密钥
	decrypted, err := m.decryptWithNonce(record.EncryptedKey, record.Nonce)
	if err != nil {
		return "", fmt.Errorf("解密API密钥失败: %w", err)
	}

	// 更新使用记录
	record.LastUsed = time.Now()
	_ = m.saveAPIKeyRecord(record)

	return string(decrypted), nil
}

// RotateAPIKey 轮转API密钥
func (m *OAuthSecurityManager) RotateAPIKey(providerID string, keyType string, newKeyValue string, reason string) (*KeyRotationRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	keyID := generateKeyID(providerID, keyType)
	oldRecord, err := m.loadAPIKeyRecord(keyID)
	if err != nil {
		return nil, err
	}

	oldVersion := oldRecord.KeyVersion
	newVersion := oldVersion + 1

	// 加密新密钥
	encryptedKey, nonce, err := m.encryptWithNonce([]byte(newKeyValue))
	if err != nil {
		return nil, fmt.Errorf("加密新密钥失败: %w", err)
	}

	// 更新记录
	oldRecord.EncryptedKey = encryptedKey
	oldRecord.Nonce = nonce
	oldRecord.KeyVersion = newVersion
	oldRecord.RotatedAt = time.Now()
	oldRecord.RotationDue = time.Now().AddDate(0, 0, m.config.KeyRotationDays)
	oldRecord.Status = "active"

	if err := m.saveAPIKeyRecord(oldRecord); err != nil {
		return nil, err
	}

	// 记录轮转历史
	rotationRecord := &KeyRotationRecord{
		ID:          fmt.Sprintf("rotate_%d", time.Now().Unix()),
		KeyID:       keyID,
		OldVersion:  oldVersion,
		NewVersion:  newVersion,
		RotatedAt:   time.Now(),
		Reason:      reason,
		InitiatedBy: "system",
	}

	if err := m.saveRotationRecord(rotationRecord); err != nil {
		return nil, err
	}

	if m.config.AuditEnabled && m.auditLogger != nil {
		m.auditLogger.LogKeyRotation(rotationRecord)
		m.auditLogger.LogTokenOperation(OAuthOpKeyRotate, map[string]interface{}{
			"provider_id":  providerID,
			"key_type":     keyType,
			"old_version":  oldVersion,
			"new_version":  newVersion,
			"reason":       reason,
		})
	}

	return rotationRecord, nil
}

// ==================== 加密解密 ====================

// encryptWithNonce 使用AES-256-GCM加密并返回nonce
func (m *OAuthSecurityManager) encryptWithNonce(plaintext []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(m.masterKey)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// decryptWithNonce 使用AES-256-GCM解密
func (m *OAuthSecurityManager) decryptWithNonce(ciphertext []byte, nonce []byte) ([]byte, error) {
	block, err := aes.NewCipher(m.masterKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// encryptData 加密数据(无nonce返回)
func (m *OAuthSecurityManager) encryptData(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// decryptData 解密数据
func (m *OAuthSecurityManager) decryptData(ciphertext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("密文太短")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// ==================== 存储管理 ====================

// loadTokens 加载Token存储
func (m *OAuthSecurityManager) loadTokens() error {
	tokensPath := filepath.Join(m.config.SecureStoragePath, "tokens.json")
	data, err := os.ReadFile(tokensPath)
	if err != nil {
		return err
	}

	var tokens []*SecureToken
	if err := json.Unmarshal(data, &tokens); err != nil {
		return err
	}

	for _, token := range tokens {
		m.tokenStore[token.ID] = token
	}

	return nil
}

// saveTokensLocked 保存Token(已持锁)
func (m *OAuthSecurityManager) saveTokensLocked() error {
	tokens := make([]*SecureToken, 0, len(m.tokenStore))
	for _, token := range m.tokenStore {
		tokens = append(tokens, token)
	}

	data, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return err
	}

	tokensPath := filepath.Join(m.config.SecureStoragePath, "tokens.json")
	return os.WriteFile(tokensPath, data, 0600)
}

// saveAPIKeyRecord 保存API密钥记录
func (m *OAuthSecurityManager) saveAPIKeyRecord(record *APIKeyRecord) error {
	keyPath := filepath.Join(m.config.SecureStoragePath, "api_keys", record.KeyID+".json")
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(keyPath, data, 0600)
}

// loadAPIKeyRecord 加载API密钥记录
func (m *OAuthSecurityManager) loadAPIKeyRecord(keyID string) (*APIKeyRecord, error) {
	keyPath := filepath.Join(m.config.SecureStoragePath, "api_keys", keyID+".json")
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	var record APIKeyRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

// saveRotationRecord 保存轮转记录
func (m *OAuthSecurityManager) saveRotationRecord(record *KeyRotationRecord) error {
	historyPath := filepath.Join(m.config.SecureStoragePath, "rotation_history.json")
	
	var history []*KeyRotationRecord
	if data, err := os.ReadFile(historyPath); err == nil {
		_ = json.Unmarshal(data, &history)
	}

	history = append(history, record)

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(historyPath, data, 0600)
}

// ==================== 自动轮转 ====================

// rotationCheckLoop 定时检查密钥轮转
func (m *OAuthSecurityManager) rotationCheckLoop() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		m.checkKeyRotation()
		m.checkTokenExpiry()
	}
}

// checkKeyRotation 检查需要轮转的密钥
func (m *OAuthSecurityManager) checkKeyRotation() {
	keysDir := filepath.Join(m.config.SecureStoragePath, "api_keys")
	files, err := os.ReadDir(keysDir)
	if err != nil {
		return
	}

	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			keyID := strings.TrimSuffix(file.Name(), ".json")
			record, err := m.loadAPIKeyRecord(keyID)
			if err != nil {
				continue
			}

			// 检查是否需要轮转提醒
			if time.Now().After(record.RotationDue.Add(-7 * 24 * time.Hour)) {
				if m.config.AuditEnabled && m.auditLogger != nil {
					m.auditLogger.LogSecurityEvent("key_rotation_due", "warning", map[string]interface{}{
						"key_id":       keyID,
						"provider_id":  record.ProviderID,
						"rotation_due": record.RotationDue,
					})
				}
			}
		}
	}
}

// checkTokenExpiry 检查即将过期的Token
func (m *OAuthSecurityManager) checkTokenExpiry() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	for _, token := range m.tokenStore {
		if token.ExpiresAt != nil {
			// Token即将过期
			if now.After(token.ExpiresAt.Add(-m.config.TokenExpiryMargin)) {
				if m.config.AuditEnabled && m.auditLogger != nil {
					m.auditLogger.LogSecurityEvent("token_expiry_warning", "warning", map[string]interface{}{
						"provider_id":  token.ProviderID,
						"expires_at":   *token.ExpiresAt,
						"token_type":   token.TokenType,
					})
				}
			}
		}
	}
}

// ==================== 辅助函数 ====================

func generateTokenID(providerID, tokenType string) string {
	return fmt.Sprintf("%s_%s", providerID, tokenType)
}

func generateKeyID(providerID, keyType string) string {
	return fmt.Sprintf("%s_%s", providerID, keyType)
}

// GetConfig 获取配置
func (m *OAuthSecurityManager) GetConfig() OAuthSecurityConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *OAuthSecurityManager) UpdateConfig(config OAuthSecurityConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	return nil
}

// Close 关闭管理器
func (m *OAuthSecurityManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveTokensLocked()
}

// GetKeyRotationHistory 获取密钥轮转历史
func (m *OAuthSecurityManager) GetKeyRotationHistory() ([]*KeyRotationRecord, error) {
	historyPath := filepath.Join(m.config.SecureStoragePath, "rotation_history.json")
	data, err := os.ReadFile(historyPath)
	if err != nil {
		return nil, err
	}

	var history []*KeyRotationRecord
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}

	return history, nil
}