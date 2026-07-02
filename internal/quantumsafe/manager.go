// Package quantumsafe 提供抗量子加密模块核心管理逻辑
package quantumsafe

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 量子安全加密管理器.
type Manager struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	config     *QuantumSafeConfig
	keys       map[string]*QuantumKey
	ciphers    map[string]*HybridCipher
	migrations map[string]*MigrationPlan
	auditLog   []*CryptoAudit
	stats      *CryptoStats
}

// NewManager 创建量子安全加密管理器.
func NewManager(logger *zap.Logger, config *QuantumSafeConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultQuantumSafeConfig()
	}

	return &Manager{
		logger:     logger,
		config:     config,
		keys:       make(map[string]*QuantumKey),
		ciphers:    make(map[string]*HybridCipher),
		migrations: make(map[string]*MigrationPlan),
		auditLog:   make([]*CryptoAudit, 0),
		stats: &CryptoStats{
			ByAlgorithm: make(map[Algorithm]int64),
			ByMode:      make(map[CipherMode]int64),
		},
	}
}

// generateID 生成唯一 ID.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateKey 生成量子安全密钥.
func (m *Manager) GenerateKey(name string, algo Algorithm, level SecurityLevel) (*QuantumKey, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("quantum safe module is disabled")
	}

	start := time.Now()

	// 确定算法
	if algo == "" {
		algo = m.config.DefaultAlgorithm
	}
	if level == 0 {
		level = m.config.DefaultSecurityLevel
	}

	// 获取算法信息
	algoInfo := GetAlgorithmInfo(algo)
	if !algoInfo.IsQuantumSafe && !m.config.HybridMode {
		return nil, fmt.Errorf("algorithm %s is not quantum-safe", algo)
	}

	// 生成密钥对（模拟）
	keySize := algoInfo.KeySize
	publicKey := make([]byte, keySize)
	privateKey := make([]byte, keySize*2)
	rand.Read(publicKey)
	rand.Read(privateKey)

	key := &QuantumKey{
		ID:            generateID(),
		Name:          name,
		Algorithm:     algo,
		SecurityLevel: level,
		Status:        KeyStatusActive,
		PublicKey:     publicKey,
		PrivateKey:    privateKey,
		KeySize:       keySize,
		IsHybrid:      m.config.HybridMode,
		ExpiresAt:     time.Now().AddDate(0, 0, m.config.KeyRotationDays),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		UsageCount:    0,
		MaxUsage:      m.config.MaxKeyUsage,
	}

	if m.config.HybridMode && algoInfo.IsHybridReady {
		key.AlgorithmPair = &AlgorithmPair{
			PostQuantum: algo,
			Classical:   m.config.ClassicalAlgorithm,
		}
	}

	m.mu.Lock()
	m.keys[key.ID] = key
	m.stats.TotalKeys++
	m.stats.ActiveKeys++
	m.stats.ByAlgorithm[algo]++
	m.mu.Unlock()

	// 审计
	m.addAudit(&CryptoAudit{
		ID:        generateID(),
		Action:    AuditKeyGenerate,
		KeyID:     key.ID,
		Algorithm: algo,
		Success:   true,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Details: map[string]interface{}{
			"key_size":       keySize,
			"security_level": level,
			"is_hybrid":      key.IsHybrid,
		},
	})

	m.logger.Info("quantum key generated",
		zap.String("key_id", key.ID),
		zap.String("algorithm", string(algo)),
		zap.Int("security_level", int(level)))

	return key, nil
}

// EncryptHybrid 混合加密.
func (m *Manager) EncryptHybrid(req *EncryptRequest) (*EncryptResponse, error) {
	if !m.config.Enabled {
		return nil, fmt.Errorf("quantum safe module is disabled")
	}

	start := time.Now()

	// 获取密钥
	m.mu.RLock()
	key, ok := m.keys[req.KeyID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key not found: %s", req.KeyID)
	}

	if key.Status != KeyStatusActive {
		return nil, fmt.Errorf("key is not active: %s", key.Status)
	}

	// 检查密钥是否过期
	if key.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("key has expired")
	}

	// 检查使用次数
	if key.MaxUsage > 0 && key.UsageCount >= key.MaxUsage {
		return nil, fmt.Errorf("key usage limit exceeded")
	}

	// 确定加密模式
	mode := req.Mode
	if mode == "" {
		if m.config.HybridMode {
			mode = ModeHybrid
		} else {
			mode = ModePostQuantum
		}
	}

	algo := req.Algorithm
	if algo == "" {
		algo = key.Algorithm
	}

	// 生成 IV
	iv := make([]byte, 16)
	rand.Read(iv)

	// 执行加密（使用经典 AES 作为模拟实现）
	ciphertext, tag, err := m.encryptData(key, req.Plaintext, iv, req.AAD)
	if err != nil {
		m.addAudit(&CryptoAudit{
			ID:        generateID(),
			Action:    AuditEncrypt,
			KeyID:     key.ID,
			Algorithm: algo,
			Success:   false,
			Error:     err.Error(),
			Timestamp: time.Now(),
			Duration:  time.Since(start),
		})
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	// 更新密钥使用次数
	m.mu.Lock()
	key.UsageCount++
	m.stats.TotalOperations++
	m.stats.EncryptOps++
	m.stats.ByMode[mode]++
	m.mu.Unlock()

	// 审计
	m.addAudit(&CryptoAudit{
		ID:        generateID(),
		Action:    AuditEncrypt,
		KeyID:     key.ID,
		Algorithm: algo,
		Success:   true,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Details: map[string]interface{}{
			"data_size": len(req.Plaintext),
			"mode":      string(mode),
		},
	})

	return &EncryptResponse{
		Ciphertext: ciphertext,
		IV:         iv,
		Tag:        tag,
		KeyID:      key.ID,
		Algorithm:  algo,
		Mode:       mode,
	}, nil
}

// MigrateKeys 迁移密钥.
func (m *Manager) MigrateKeys(sourceKeyID string, targetAlgorithm Algorithm) (*MigrationPlan, error) {
	if !m.config.MigrationEnabled {
		return nil, fmt.Errorf("migration is disabled")
	}

	start := time.Now()

	// 获取源密钥
	m.mu.RLock()
	sourceKey, ok := m.keys[sourceKeyID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("source key not found: %s", sourceKeyID)
	}

	// 生成新密钥
	newKey, err := m.GenerateKey(
		fmt.Sprintf("%s-migrated", sourceKey.Name),
		targetAlgorithm,
		sourceKey.SecurityLevel,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate target key: %w", err)
	}

	// 创建迁移计划
	now := time.Now()
	plan := &MigrationPlan{
		ID:              generateID(),
		Name:            fmt.Sprintf("Migration from %s to %s", sourceKey.Algorithm, targetAlgorithm),
		Description:     fmt.Sprintf("Migrate key %s to post-quantum algorithm %s", sourceKeyID, targetAlgorithm),
		Status:          MigrationInProgress,
		SourceAlgorithm: sourceKey.Algorithm,
		TargetAlgorithm: targetAlgorithm,
		SourceKeyID:     sourceKeyID,
		TargetKeyID:     newKey.ID,
		TotalResources:  1, // 简化：假设只有1个资源（密钥本身）
		Progress:        0,
		StartedAt:       &now,
		Resources:       make([]MigrationResource, 0),
		Errors:          make([]MigrationError, 0),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	// 标记源密钥为轮换中
	m.mu.Lock()
	sourceKey.Status = KeyStatusRotating
	sourceKey.UpdatedAt = time.Now()

	plan.MigratedCount = 1
	plan.Progress = 100
	completedAt := time.Now()
	plan.CompletedAt = &completedAt
	plan.Status = MigrationCompleted

	m.migrations[plan.ID] = plan
	m.stats.MigrationsTotal++
	m.mu.Unlock()

	// 标记源密钥为已弃用
	m.mu.Lock()
	sourceKey.Status = KeyStatusDeprecated
	m.mu.Unlock()

	// 审计
	m.addAudit(&CryptoAudit{
		ID:        generateID(),
		Action:    AuditMigrate,
		KeyID:     sourceKeyID,
		Algorithm: targetAlgorithm,
		Success:   true,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
		Details: map[string]interface{}{
			"source_algorithm": string(sourceKey.Algorithm),
			"target_algorithm": string(targetAlgorithm),
			"new_key_id":       newKey.ID,
		},
	})

	m.logger.Info("key migration completed",
		zap.String("plan_id", plan.ID),
		zap.String("source_key", sourceKeyID),
		zap.String("target_key", newKey.ID),
		zap.String("target_algorithm", string(targetAlgorithm)))

	return plan, nil
}

// AuditCrypto 加密审计.
func (m *Manager) AuditCrypto(action AuditAction, keyID string, details map[string]interface{}) *CryptoAudit {
	m.mu.RLock()
	key, ok := m.keys[keyID]
	m.mu.RUnlock()

	algo := Algorithm("")
	if ok {
		algo = key.Algorithm
	}

	audit := &CryptoAudit{
		ID:        generateID(),
		Action:    action,
		KeyID:     keyID,
		Algorithm: algo,
		Success:   true,
		Timestamp: time.Now(),
		Details:   details,
	}

	m.addAudit(audit)
	return audit
}

// encryptData 执行加密（使用 AES-GCM 作为模拟）.
func (m *Manager) encryptData(key *QuantumKey, plaintext, iv, aad []byte) ([]byte, []byte, error) {
	// 使用 SHA-256 哈希密钥的前32字节作为 AES 密钥
	hash := sha256.Sum256(key.PrivateKey[:32])

	block, err := aes.NewCipher(hash[:])
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// 使用 IV 作为 nonce
	nonce := iv[:gcm.NonceSize()]

	ciphertext := gcm.Seal(nil, nonce, plaintext, aad)

	// 提取 tag（GCM Seal 已包含 tag）
	tagSize := gcm.Overhead()
	var tag []byte
	if len(ciphertext) > tagSize {
		tag = ciphertext[len(ciphertext)-tagSize:]
		ciphertext = ciphertext[:len(ciphertext)-tagSize]
	}

	return ciphertext, tag, nil
}

// addAudit 添加审计日志.
func (m *Manager) addAudit(audit *CryptoAudit) {
	if !m.config.AuditEnabled {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.auditLog = append(m.auditLog, audit)

	// 限制审计日志大小
	if len(m.auditLog) > 10000 {
		m.auditLog = m.auditLog[len(m.auditLog)-10000:]
	}
}

// CreateCipher 创建混合加密器.
func (m *Manager) CreateCipher(req *HybridCipher) (*HybridCipher, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.ID == "" {
		req.ID = generateID()
	}
	req.IsActive = true
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	// 验证密钥存在
	if _, ok := m.keys[req.KeyID]; !ok {
		return nil, fmt.Errorf("key not found: %s", req.KeyID)
	}

	m.ciphers[req.ID] = req
	return req, nil
}

// GetCipher 获取加密器.
func (m *Manager) GetCipher(id string) (*HybridCipher, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.ciphers[id]
	if !ok {
		return nil, fmt.Errorf("cipher not found: %s", id)
	}
	return c, nil
}

// ListCiphers 列出所有加密器.
func (m *Manager) ListCiphers() []*HybridCipher {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ciphers := make([]*HybridCipher, 0, len(m.ciphers))
	for _, c := range m.ciphers {
		ciphers = append(ciphers, c)
	}
	return ciphers
}

// RotateKey 轮换密钥.
func (m *Manager) RotateKey(req *KeyRotationRequest) (*QuantumKey, error) {
	m.mu.RLock()
	oldKey, ok := m.keys[req.KeyID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("key not found: %s", req.KeyID)
	}

	// 确定新算法
	newAlgo := req.NewAlgorithm
	if newAlgo == "" {
		newAlgo = oldKey.Algorithm
	}

	// 生成新密钥
	newKey, err := m.GenerateKey(
		fmt.Sprintf("%s-rotated", oldKey.Name),
		newAlgo,
		oldKey.SecurityLevel,
	)
	if err != nil {
		return nil, err
	}

	newKey.RotatedFrom = oldKey.ID

	// 标记旧密钥
	m.mu.Lock()
	if req.RetainOldKey {
		oldKey.Status = KeyStatusDeprecated
	} else {
		oldKey.Status = KeyStatusRevoked
	}
	oldKey.UpdatedAt = time.Now()
	m.stats.ActiveKeys--
	m.mu.Unlock()

	m.logger.Info("key rotated",
		zap.String("old_key_id", req.KeyID),
		zap.String("new_key_id", newKey.ID),
		zap.String("new_algorithm", string(newAlgo)))

	return newKey, nil
}

// GetKey 获取密钥.
func (m *Manager) GetKey(id string) (*QuantumKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.keys[id]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", id)
	}
	return key, nil
}

// ListKeys 列出所有密钥.
func (m *Manager) ListKeys() []*QuantumKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]*QuantumKey, 0, len(m.keys))
	for _, k := range m.keys {
		keys = append(keys, k)
	}
	return keys
}

// RevokeKey 吊销密钥.
func (m *Manager) RevokeKey(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.keys[id]
	if !ok {
		return fmt.Errorf("key not found: %s", id)
	}

	key.Status = KeyStatusRevoked
	key.UpdatedAt = time.Now()
	m.stats.ActiveKeys--

	// 审计
	m.addAudit(&CryptoAudit{
		ID:        generateID(),
		Action:    AuditKeyRevoke,
		KeyID:     id,
		Algorithm: key.Algorithm,
		Success:   true,
		Timestamp: time.Now(),
	})

	return nil
}

// GetMigration 获取迁移计划.
func (m *Manager) GetMigration(id string) (*MigrationPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.migrations[id]
	if !ok {
		return nil, fmt.Errorf("migration plan not found: %s", id)
	}
	return plan, nil
}

// ListMigrations 列出所有迁移计划.
func (m *Manager) ListMigrations() []*MigrationPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plans := make([]*MigrationPlan, 0, len(m.migrations))
	for _, p := range m.migrations {
		plans = append(plans, p)
	}
	return plans
}

// GetAuditLog 获取审计日志.
func (m *Manager) GetAuditLog(limit int) []*CryptoAudit {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.auditLog) {
		limit = len(m.auditLog)
	}

	start := len(m.auditLog) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*CryptoAudit, limit)
	copy(result, m.auditLog[start:])
	return result
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() *CryptoStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	stats.TotalKeys = int64(len(m.keys))
	stats.MigrationsActive = 0

	for _, plan := range m.migrations {
		if plan.Status == MigrationInProgress {
			stats.MigrationsActive++
		}
	}

	return &stats
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *QuantumSafeConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg *QuantumSafeConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}

// GetAlgorithmInfo 获取算法信息.
func (m *Manager) GetAlgorithmInfo(algo Algorithm) *AlgorithmInfo {
	return GetAlgorithmInfo(algo)
}

// ListAlgorithms 列出支持的算法.
func (m *Manager) ListAlgorithms() []AlgorithmInfo {
	algos := SupportedAlgorithms()
	result := make([]AlgorithmInfo, 0, len(algos))
	for _, a := range algos {
		info := GetAlgorithmInfo(a)
		result = append(result, *info)
	}
	return result
}
