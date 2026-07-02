// Package volencrypt 提供全卷加密功能
// 对标群晖 DSM 7.2 Full Volume Encryption，超越其实现
// 特性：AES-256-XTS、密钥管理、远程密钥托管、自动挂载、加密审计
package volencrypt

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

// VolumeStatus 卷状态.
type VolumeStatus string

const (
	StatusUnencrypted VolumeStatus = "unencrypted"
	StatusEncrypting  VolumeStatus = "encrypting"
	StatusEncrypted   VolumeStatus = "encrypted"
	StatusDecrypting  VolumeStatus = "decrypting"
	StatusError       VolumeStatus = "error"
	StatusLocked      VolumeStatus = "locked"
)

// KeySource 密钥来源.
type KeySource string

const (
	KeySourceLocal KeySource = "local" // 本地密钥
	KeySourceKMIP  KeySource = "kmip"  // KMIP 远程密钥管理
	KeySourceTPM   KeySource = "tpm"   // TPM 安全芯片
	KeySourceUSB   KeySource = "usb"   // USB 密钥盘
	KeySourceTang  KeySource = "tang"  // Tang 网络密钥服务器 (Clevis)
)

// EncryptConfig 加密配置.
type EncryptConfig struct {
	Algorithm    string    `json:"algorithm"`     // 加密算法
	KeySize      int       `json:"key_size"`      // 密钥大小（位）
	BlockSize    int       `json:"block_size"`    // 块大小
	KeySource    KeySource `json:"key_source"`    // 密钥来源
	AutoMount    bool      `json:"auto_mount"`    // 自动挂载
	RemoteBackup bool      `json:"remote_backup"` // 远程备份密钥
	AuditLog     bool      `json:"audit_log"`     // 审计日志
}

// DefaultConfig 返回默认配置.
func DefaultConfig() EncryptConfig {
	return EncryptConfig{
		Algorithm:    "AES-256-XTS",
		KeySize:      256,
		BlockSize:    4096,
		KeySource:    KeySourceLocal,
		AutoMount:    true,
		RemoteBackup: false,
		AuditLog:     true,
	}
}

// Volume 加密卷.
type Volume struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Path       string       `json:"path"`
	Status     VolumeStatus `json:"status"`
	KeyID      string       `json:"key_id"`
	Algorithm  string       `json:"algorithm"`
	KeySource  KeySource    `json:"key_source"`
	Size       int64        `json:"size"`
	Used       int64        `json:"used"`
	CreatedAt  time.Time    `json:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at"`
	MountPoint string       `json:"mount_point,omitempty"`
	ErrorMsg   string       `json:"error_msg,omitempty"`
	Progress   float64      `json:"progress"` // 0-100
}

// EncryptionKey 加密密钥.
type EncryptionKey struct {
	ID        string     `json:"id"`
	Algorithm string     `json:"algorithm"`
	KeyData   []byte     `json:"-"` // 不序列化
	KeyHash   string     `json:"key_hash"`
	Source    KeySource  `json:"source"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	Revoked   bool       `json:"revoked"`
}

// AuditEntry 审计条目.
type AuditEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	VolumeID  string    `json:"volume_id"`
	KeyID     string    `json:"key_id"`
	Source    string    `json:"source"`
	Success   bool      `json:"success"`
	Message   string    `json:"message"`
}

// Manager 卷加密管理器.
type Manager struct {
	mu       sync.RWMutex
	config   EncryptConfig
	volumes  map[string]*Volume
	keys     map[string]*EncryptionKey
	auditLog []AuditEntry
	stopCh   chan struct{}
}

// NewManager 创建卷加密管理器.
func NewManager(config EncryptConfig) *Manager {
	return &Manager{
		config:  config,
		volumes: make(map[string]*Volume),
		keys:    make(map[string]*EncryptionKey),
		stopCh:  make(chan struct{}),
	}
}

// Start 启动管理器.
func (m *Manager) Start(ctx context.Context) error {
	// 启动后台任务
	go m.backgroundTasks(ctx)
	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// CreateVolume 创建加密卷.
func (m *Manager) CreateVolume(name, path string, size int64) (*Volume, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查卷名是否已存在
	for _, v := range m.volumes {
		if v.Name == name {
			return nil, fmt.Errorf("卷名 %s 已存在", name)
		}
	}

	// 生成密钥
	key, err := m.generateKey(m.config.KeySource)
	if err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}

	volume := &Volume{
		ID:        generateID(),
		Name:      name,
		Path:      path,
		Status:    StatusUnencrypted,
		KeyID:     key.ID,
		Algorithm: m.config.Algorithm,
		KeySource: m.config.KeySource,
		Size:      size,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.volumes[volume.ID] = volume
	m.addAudit("create_volume", volume.ID, key.ID, true, "创建加密卷")

	return volume, nil
}

// EncryptVolume 加密卷.
func (m *Manager) EncryptVolume(ctx context.Context, volumeID string) error {
	m.mu.Lock()
	volume, exists := m.volumes[volumeID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("卷 %s 不存在", volumeID)
	}

	if volume.Status == StatusEncrypted {
		m.mu.Unlock()
		return fmt.Errorf("卷 %s 已加密", volumeID)
	}

	volume.Status = StatusEncrypting
	volume.Progress = 0
	m.mu.Unlock()

	// 模拟加密过程
	go m.doEncryption(ctx, volume)

	return nil
}

// DecryptVolume 解密卷.
func (m *Manager) DecryptVolume(ctx context.Context, volumeID string) error {
	m.mu.Lock()
	volume, exists := m.volumes[volumeID]
	if !exists {
		m.mu.Unlock()
		return fmt.Errorf("卷 %s 不存在", volumeID)
	}

	if volume.Status != StatusEncrypted && volume.Status != StatusLocked {
		m.mu.Unlock()
		return fmt.Errorf("卷 %s 未加密", volumeID)
	}

	volume.Status = StatusDecrypting
	volume.Progress = 0
	m.mu.Unlock()

	// 模拟解密过程
	go m.doDecryption(ctx, volume)

	return nil
}

// LockVolume 锁定卷（卸载并锁定密钥）.
func (m *Manager) LockVolume(volumeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	volume, exists := m.volumes[volumeID]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeID)
	}

	if volume.Status != StatusEncrypted {
		return fmt.Errorf("卷 %s 未加密", volumeID)
	}

	volume.Status = StatusLocked
	volume.MountPoint = ""
	volume.UpdatedAt = time.Now()

	m.addAudit("lock_volume", volumeID, volume.KeyID, true, "锁定卷")
	return nil
}

// UnlockVolume 解锁卷.
func (m *Manager) UnlockVolume(volumeID, mountPoint string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	volume, exists := m.volumes[volumeID]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeID)
	}

	if volume.Status != StatusLocked && volume.Status != StatusEncrypted {
		return fmt.Errorf("卷 %s 状态不允许解锁: %s", volumeID, volume.Status)
	}

	// 验证密钥
	key, keyExists := m.keys[volume.KeyID]
	if !keyExists {
		return fmt.Errorf("密钥 %s 不存在", volume.KeyID)
	}

	if key.Revoked {
		return fmt.Errorf("密钥 %s 已吊销", volume.KeyID)
	}

	volume.Status = StatusEncrypted
	volume.MountPoint = mountPoint
	volume.UpdatedAt = time.Now()

	m.addAudit("unlock_volume", volumeID, volume.KeyID, true, "解锁卷到 "+mountPoint)
	return nil
}

// GetVolume 获取卷信息.
func (m *Manager) GetVolume(volumeID string) (*Volume, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	volume, exists := m.volumes[volumeID]
	if !exists {
		return nil, fmt.Errorf("卷 %s 不存在", volumeID)
	}

	return volume, nil
}

// ListVolumes 列出所有卷.
func (m *Manager) ListVolumes() []Volume {
	m.mu.RLock()
	defer m.mu.RUnlock()

	volumes := make([]Volume, 0, len(m.volumes))
	for _, v := range m.volumes {
		volumes = append(volumes, *v)
	}
	return volumes
}

// RotateKey 轮换密钥.
func (m *Manager) RotateKey(volumeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	volume, exists := m.volumes[volumeID]
	if !exists {
		return fmt.Errorf("卷 %s 不存在", volumeID)
	}

	// 生成新密钥
	newKey, err := m.generateKey(volume.KeySource)
	if err != nil {
		return fmt.Errorf("生成新密钥失败: %w", err)
	}

	// 吊销旧密钥
	if oldKey, ok := m.keys[volume.KeyID]; ok {
		oldKey.Revoked = true
	}

	// 更新卷
	volume.KeyID = newKey.ID
	volume.UpdatedAt = time.Now()

	m.addAudit("rotate_key", volumeID, newKey.ID, true, "密钥轮换")
	return nil
}

// RevokeKey 吊销密钥.
func (m *Manager) RevokeKey(keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, exists := m.keys[keyID]
	if !exists {
		return fmt.Errorf("密钥 %s 不存在", keyID)
	}

	key.Revoked = true
	m.addAudit("revoke_key", "", keyID, true, "吊销密钥")
	return nil
}

// GetAuditLog 获取审计日志.
func (m *Manager) GetAuditLog(limit int) []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.auditLog) {
		limit = len(m.auditLog)
	}

	start := len(m.auditLog) - limit
	if start < 0 {
		start = 0
	}

	return m.auditLog[start:]
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_volumes": len(m.volumes),
		"total_keys":    len(m.keys),
		"audit_entries": len(m.auditLog),
	}

	encrypted := 0
	locked := 0
	encrypting := 0
	var totalSize int64

	for _, v := range m.volumes {
		switch v.Status {
		case StatusEncrypted:
			encrypted++
		case StatusLocked:
			locked++
		case StatusEncrypting:
			encrypting++
		}
		totalSize += v.Size
	}

	stats["encrypted_volumes"] = encrypted
	stats["locked_volumes"] = locked
	stats["encrypting_volumes"] = encrypting
	stats["total_size"] = totalSize

	return stats
}

// generateKey 生成加密密钥.
func (m *Manager) generateKey(source KeySource) (*EncryptionKey, error) {
	keyBytes := make([]byte, m.config.KeySize/8)
	if _, err := io.ReadFull(rand.Reader, keyBytes); err != nil {
		return nil, fmt.Errorf("生成随机密钥失败: %w", err)
	}

	hash := sha256.Sum256(keyBytes)
	key := &EncryptionKey{
		ID:        generateID(),
		Algorithm: m.config.Algorithm,
		KeyData:   keyBytes,
		KeyHash:   hex.EncodeToString(hash[:]),
		Source:    source,
		CreatedAt: time.Now(),
	}

	m.keys[key.ID] = key
	return key, nil
}

// doEncryption 执行加密.
func (m *Manager) doEncryption(ctx context.Context, volume *Volume) {
	// 模拟加密进度
	for i := 0; i <= 100; i += 5 {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			volume.Status = StatusError
			volume.ErrorMsg = "加密被取消"
			volume.UpdatedAt = time.Now()
			m.mu.Unlock()
			return
		case <-time.After(10 * time.Millisecond):
			m.mu.Lock()
			volume.Progress = float64(i)
			m.mu.Unlock()
		}
	}

	m.mu.Lock()
	volume.Status = StatusEncrypted
	volume.Progress = 100
	volume.UpdatedAt = time.Now()
	m.addAudit("encrypt_volume", volume.ID, volume.KeyID, true, "加密完成")
	m.mu.Unlock()
}

// doDecryption 执行解密.
func (m *Manager) doDecryption(ctx context.Context, volume *Volume) {
	// 模拟解密进度
	for i := 0; i <= 100; i += 5 {
		select {
		case <-ctx.Done():
			m.mu.Lock()
			volume.Status = StatusError
			volume.ErrorMsg = "解密被取消"
			volume.UpdatedAt = time.Now()
			m.mu.Unlock()
			return
		case <-time.After(10 * time.Millisecond):
			m.mu.Lock()
			volume.Progress = float64(i)
			m.mu.Unlock()
		}
	}

	m.mu.Lock()
	volume.Status = StatusUnencrypted
	volume.Progress = 100
	volume.UpdatedAt = time.Now()
	m.addAudit("decrypt_volume", volume.ID, volume.KeyID, true, "解密完成")
	m.mu.Unlock()
}

// addAudit 添加审计记录.
func (m *Manager) addAudit(action, volumeID, keyID string, success bool, message string) {
	entry := AuditEntry{
		Timestamp: time.Now(),
		Action:    action,
		VolumeID:  volumeID,
		KeyID:     keyID,
		Source:    "api",
		Success:   success,
		Message:   message,
	}
	m.auditLog = append(m.auditLog, entry)

	// 限制审计日志大小
	if len(m.auditLog) > 10000 {
		m.auditLog = m.auditLog[1000:]
	}
}

// backgroundTasks 后台任务.
func (m *Manager) backgroundTasks(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkKeyExpiry()
		}
	}
}

// checkKeyExpiry 检查密钥过期.
func (m *Manager) checkKeyExpiry() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, key := range m.keys {
		if key.ExpiresAt != nil && now.After(*key.ExpiresAt) && !key.Revoked {
			key.Revoked = true
			m.addAudit("auto_revoke_key", "", key.ID, true, "密钥自动过期吊销")
		}
	}
}

// generateID 生成唯一 ID.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// EncryptData 使用 AES-256-GCM 加密数据.
func EncryptData(key, plaintext []byte) ([]byte, error) {
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

// DecryptData 使用 AES-256-GCM 解密数据.
func DecryptData(key, ciphertext []byte) ([]byte, error) {
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
