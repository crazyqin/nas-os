// Package vaultencryption - 保险库加密管理器
// 实现密钥管理、卷解锁、自动锁定逻辑
package vaultencryption

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// VaultEncryptionManager 保险库加密管理器
type VaultEncryptionManager struct {
	mu           sync.RWMutex
	config       VaultConfig
	keys         map[string]*VaultKey         // 密钥ID -> 密钥
	volumes      map[string]*EncryptedVolume  // 卷ID -> 加密卷
	auditLogs    []AuditLog                   // 审计日志
	stats        VaultStats                   // 统计信息
	lockTimers   map[string]*time.Timer       // 自动锁定定时器
	retryCounts  map[string]int               // 重试计数
	lockoutUntil map[string]time.Time         // 锁定截止时间
}

// NewVaultEncryptionManager 创建管理器
func NewVaultEncryptionManager() *VaultEncryptionManager {
	return &VaultEncryptionManager{
		config:       DefaultVaultConfig(),
		keys:         make(map[string]*VaultKey),
		volumes:      make(map[string]*EncryptedVolume),
		auditLogs:    make([]AuditLog, 0),
		lockTimers:   make(map[string]*time.Timer),
		retryCounts:  make(map[string]int),
		lockoutUntil: make(map[string]time.Time),
	}
}

// SetConfig 设置配置
func (m *VaultEncryptionManager) SetConfig(config VaultConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
}

// GetConfig 获取配置
func (m *VaultEncryptionManager) GetConfig() VaultConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// ============================================================
// 密钥管理
// ============================================================

// CreateKey 创建保险库密钥
func (m *VaultEncryptionManager) CreateKey(req CreateKeyRequest) (*VaultKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证请求
	if req.Name == "" {
		return nil, fmt.Errorf("密钥名称不能为空")
	}

	if req.Password == "" {
		return nil, fmt.Errorf("密码不能为空")
	}

	if len(req.Password) < 8 {
		return nil, fmt.Errorf("密码长度不能少于8位")
	}

	// 生成盐值
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, fmt.Errorf("生成盐值失败: %w", err)
	}

	// 派生密钥哈希（简化版，实际应使用 argon2/scrypt）
	keyHash := m.deriveKeyHash(req.Password, salt)

	// 计算过期时间
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().AddDate(0, 0, req.ExpiresIn)
		expiresAt = &t
	}

	// 创建密钥
	key := &VaultKey{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		KeyHash:     hex.EncodeToString(keyHash),
		Salt:        hex.EncodeToString(salt),
		Algorithm:   m.config.KeyDerivation,
		CreatedAt:   time.Now(),
		IsActive:    true,
		ExpiresAt:   expiresAt,
	}

	m.keys[key.ID] = key

	// 记录审计日志
	m.addAuditLog(AuditActionCreateKey, "", key.ID, "", true, "密钥创建成功", "")

	return key, nil
}

// DeleteKey 删除密钥
func (m *VaultEncryptionManager) DeleteKey(keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return fmt.Errorf("密钥不存在: %s", keyID)
	}

	// 检查是否有卷使用此密钥
	for _, vol := range m.volumes {
		if vol.KeyID == keyID && !vol.IsLocked {
			return fmt.Errorf("密钥正在被卷 %s 使用，请先锁定卷", vol.Name)
		}
	}

	key.IsActive = false

	// 记录审计日志
	m.addAuditLog(AuditActionDeleteKey, "", keyID, "", true, "密钥已删除", "")

	return nil
}

// GetKey 获取密钥信息
func (m *VaultEncryptionManager) GetKey(keyID string) (*VaultKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, exists := m.keys[keyID]
	if !exists {
		return nil, fmt.Errorf("密钥不存在: %s", keyID)
	}

	return key, nil
}

// ListKeys 列出所有密钥
func (m *VaultEncryptionManager) ListKeys() []VaultKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]VaultKey, 0, len(m.keys))
	for _, key := range m.keys {
		keys = append(keys, *key)
	}
	return keys
}

// ChangePassword 修改密钥密码
func (m *VaultEncryptionManager) ChangePassword(req ChangePasswordRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[req.KeyID]
	if !exists {
		return fmt.Errorf("密钥不存在: %s", req.KeyID)
	}

	// 验证旧密码
	salt, _ := hex.DecodeString(key.Salt)
	oldHash := m.deriveKeyHash(req.OldPassword, salt)
	if hex.EncodeToString(oldHash) != key.KeyHash {
		m.addAuditLog(AuditActionChangePass, "", req.KeyID, "", false, "旧密码验证失败", "")
		return fmt.Errorf("旧密码不正确")
	}

	// 验证新密码
	if len(req.NewPassword) < 8 {
		return fmt.Errorf("新密码长度不能少于8位")
	}

	// 生成新的盐值和哈希
	newSalt := make([]byte, 32)
	if _, err := rand.Read(newSalt); err != nil {
		return fmt.Errorf("生成盐值失败: %w", err)
	}

	newHash := m.deriveKeyHash(req.NewPassword, newSalt)

	// 更新密钥
	key.KeyHash = hex.EncodeToString(newHash)
	key.Salt = hex.EncodeToString(newSalt)
	key.LastUsedAt = time.Now()
	key.UsageCount++

	m.addAuditLog(AuditActionChangePass, "", req.KeyID, "", true, "密码修改成功", "")

	return nil
}

// ============================================================
// 卷管理
// ============================================================

// RegisterVolume 注册加密卷
func (m *VaultEncryptionManager) RegisterVolume(vol *EncryptedVolume) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if vol.ID == "" {
		vol.ID = uuid.New().String()
	}

	if vol.Name == "" {
		return fmt.Errorf("卷名称不能为空")
	}

	// 设置默认值
	if vol.EncryptionAlgo == "" {
		vol.EncryptionAlgo = string(AlgoAES256XTS)
	}

	vol.IsLocked = true
	vol.CreatedAt = time.Now()
	vol.UpdatedAt = time.Now()

	m.volumes[vol.ID] = vol
	m.updateStats()

	return nil
}

// UnregisterVolume 注销加密卷
func (m *VaultEncryptionManager) UnregisterVolume(volumeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vol, exists := m.volumes[volumeID]
	if !exists {
		return fmt.Errorf("卷不存在: %s", volumeID)
	}

	if !vol.IsLocked {
		return fmt.Errorf("卷未锁定，请先锁定后再注销")
	}

	delete(m.volumes, volumeID)
	m.updateStats()

	return nil
}

// GetVolume 获取卷信息
func (m *VaultEncryptionManager) GetVolume(volumeID string) (*EncryptedVolume, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vol, exists := m.volumes[volumeID]
	if !exists {
		return nil, fmt.Errorf("卷不存在: %s", volumeID)
	}

	return vol, nil
}

// ListVolumes 列出所有卷
func (m *VaultEncryptionManager) ListVolumes() []EncryptedVolume {
	m.mu.RLock()
	defer m.mu.RUnlock()

	volumes := make([]EncryptedVolume, 0, len(m.volumes))
	for _, vol := range m.volumes {
		volumes = append(volumes, *vol)
	}
	return volumes
}

// ============================================================
// 解锁/锁定逻辑
// ============================================================

// UnlockVolume 解锁加密卷
func (m *VaultEncryptionManager) UnlockVolume(req UnlockRequest) (*UnlockResponse, error) {
	m.mu.Lock()

	// 检查卷是否存在
	vol, exists := m.volumes[req.VolumeID]
	if !exists {
		m.mu.Unlock()
		return &UnlockResponse{
			Success:  false,
			VolumeID: req.VolumeID,
			Message:  "卷不存在",
		}, fmt.Errorf("卷不存在: %s", req.VolumeID)
	}

	// 检查是否已解锁
	if !vol.IsLocked {
		m.mu.Unlock()
		return &UnlockResponse{
			Success:    true,
			VolumeID:   req.VolumeID,
			MountPoint: vol.MountPoint,
			Message:    "卷已经解锁",
		}, nil
	}

	// 检查锁定状态
	lockout, isLockedOut := m.lockoutUntil[req.VolumeID]
	if isLockedOut && time.Now().Before(lockout) {
		remaining := time.Until(lockout)
		m.mu.Unlock()
		return &UnlockResponse{
			Success:  false,
			VolumeID: req.VolumeID,
			Message:  fmt.Sprintf("由于多次失败，已锁定，请在 %v 后重试", remaining.Round(time.Minute)),
		}, fmt.Errorf("卷已锁定")
	}

	// 获取关联的密钥
	key, keyExists := m.keys[vol.KeyID]
	if !keyExists {
		m.mu.Unlock()
		return &UnlockResponse{
			Success:  false,
			VolumeID: req.VolumeID,
			Message:  "关联密钥不存在",
		}, fmt.Errorf("关联密钥不存在: %s", vol.KeyID)
	}

	// 验证密码
	salt, _ := hex.DecodeString(key.Salt)
	keyHash := m.deriveKeyHash(req.Password, salt)
	if hex.EncodeToString(keyHash) != key.KeyHash {
		// 增加重试计数
		m.retryCounts[req.VolumeID]++
		retryCount := m.retryCounts[req.VolumeID]

		// 检查是否超过最大重试次数
		if retryCount >= m.config.MaxRetryAttempts {
			lockoutTime := time.Now().Add(m.config.RetryLockout)
			m.lockoutUntil[req.VolumeID] = lockoutTime
			m.stats.FailedAttempts++
			m.addAuditLog(ActionUnlock, req.VolumeID, vol.KeyID, "", false,
				fmt.Sprintf("超过最大重试次数，锁定至 %v", lockoutTime), "")
			m.mu.Unlock()
			return &UnlockResponse{
				Success:  false,
				VolumeID: req.VolumeID,
				Message:  "超过最大重试次数，卷已临时锁定",
			}, fmt.Errorf("超过最大重试次数")
		}

		m.stats.FailedAttempts++
		m.addAuditLog(ActionUnlock, req.VolumeID, vol.KeyID, "", false,
			fmt.Sprintf("密码错误，已尝试 %d/%d 次", retryCount, m.config.MaxRetryAttempts), "")
		m.mu.Unlock()
		return &UnlockResponse{
			Success:  false,
			VolumeID: req.VolumeID,
			Message:  fmt.Sprintf("密码错误，已尝试 %d/%d 次", retryCount, m.config.MaxRetryAttempts),
		}, fmt.Errorf("密码错误")
	}

	// 解锁成功
	now := time.Now()
	vol.IsLocked = false
	vol.UnlockedAt = &now
	vol.LockedAt = nil
	vol.UpdatedAt = now

	// 更新密钥使用信息
	key.LastUsedAt = now
	key.UsageCount++

	// 重置重试计数
	delete(m.retryCounts, req.VolumeID)
	delete(m.lockoutUntil, req.VolumeID)

	// 启动自动锁定定时器
	m.startAutoLockTimer(req.VolumeID)

	m.stats.LastUnlockTime = now
	m.updateStats()
	m.addAuditLog(ActionUnlock, req.VolumeID, vol.KeyID, "", true, "解锁成功", "")
	m.mu.Unlock()

	return &UnlockResponse{
		Success:    true,
		VolumeID:   req.VolumeID,
		MountPoint: vol.MountPoint,
		Message:    "解锁成功",
	}, nil
}

// LockVolume 锁定加密卷
func (m *VaultEncryptionManager) LockVolume(req LockRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vol, exists := m.volumes[req.VolumeID]
	if !exists {
		return fmt.Errorf("卷不存在: %s", req.VolumeID)
	}

	if vol.IsLocked {
		return fmt.Errorf("卷已经锁定")
	}

	// 停止自动锁定定时器
	if timer, ok := m.lockTimers[req.VolumeID]; ok {
		timer.Stop()
		delete(m.lockTimers, req.VolumeID)
	}

	// 锁定卷
	now := time.Now()
	vol.IsLocked = true
	vol.LockedAt = &now
	vol.UnlockedAt = nil
	vol.UpdatedAt = now

	m.updateStats()
	m.addAuditLog(ActionLock, req.VolumeID, vol.KeyID, "", true, "锁定成功", "")

	return nil
}

// AutoLockVolume 自动锁定卷（内部方法）
func (m *VaultEncryptionManager) AutoLockVolume(volumeID string) {
	m.LockVolume(LockRequest{
		VolumeID: volumeID,
		Force:    true,
	})
}

// ============================================================
// 审计日志
// ============================================================

// GetAuditLogs 获取审计日志
func (m *VaultEncryptionManager) GetAuditLogs(limit int) []AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.auditLogs) {
		limit = len(m.auditLogs)
	}

	// 返回最新的日志
	start := len(m.auditLogs) - limit
	if start < 0 {
		start = 0
	}

	logs := make([]AuditLog, limit)
	copy(logs, m.auditLogs[start:])
	return logs
}

// ============================================================
// 统计
// ============================================================

// GetStats 获取统计信息
func (m *VaultEncryptionManager) GetStats() VaultStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// ============================================================
// 内部方法
// ============================================================

// deriveKeyHash 派生密钥哈希
func (m *VaultEncryptionManager) deriveKeyHash(password string, salt []byte) []byte {
	// 简化版：使用 SHA-256(password + salt)
	// 实际应使用 argon2.IDKey 或 scrypt.Key
	h := sha256.New()
	h.Write([]byte(password))
	h.Write(salt)
	return h.Sum(nil)
}

// startAutoLockTimer 启动自动锁定定时器
func (m *VaultEncryptionManager) startAutoLockTimer(volumeID string) {
	// 停止现有定时器
	if timer, ok := m.lockTimers[volumeID]; ok {
		timer.Stop()
	}

	// 创建新定时器
	m.lockTimers[volumeID] = time.AfterFunc(m.config.AutoLockTimeout, func() {
		m.AutoLockVolume(volumeID)
	})
}

// addAuditLog 添加审计日志
func (m *VaultEncryptionManager) addAuditLog(action AuditAction, volumeID, keyID, userID string, success bool, message, ipAddr string) {
	log := AuditLog{
		ID:        uuid.New().String(),
		Action:    action,
		VolumeID:  volumeID,
		KeyID:     keyID,
		UserID:    userID,
		Success:   success,
		Message:   message,
		IPAddress: ipAddr,
		Timestamp: time.Now(),
	}

	m.auditLogs = append(m.auditLogs, log)

	// 限制日志数量
	maxLogs := 1000
	if len(m.auditLogs) > maxLogs {
		m.auditLogs = m.auditLogs[len(m.auditLogs)-maxLogs:]
	}
}

// updateStats 更新统计信息
func (m *VaultEncryptionManager) updateStats() {
	m.stats.TotalKeys = 0
	m.stats.ActiveKeys = 0
	m.stats.ExpiredKeys = 0

	for _, key := range m.keys {
		m.stats.TotalKeys++
		if key.IsActive {
			m.stats.ActiveKeys++
			if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
				m.stats.ExpiredKeys++
			}
		}
	}

	m.stats.TotalVolumes = 0
	m.stats.LockedVolumes = 0
	m.stats.UnlockedVolumes = 0

	for _, vol := range m.volumes {
		m.stats.TotalVolumes++
		if vol.IsLocked {
			m.stats.LockedVolumes++
		} else {
			m.stats.UnlockedVolumes++
		}
	}
}
