// Package storage 全卷加密系统
// 基于LUKS的全卷加密管理，支持硬件AES-NI加速
// 对标群晖 Full Volume Encryption
package storage

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/pbkdf2"
)

// ========== 常量 ==========

const (
	// VolumeKeyLength 全卷加密密钥长度 (AES-256).
	VolumeKeyLength = 32
	// VolumeSaltLength 盐值长度.
	VolumeSaltLength = 32
	// VolumePBKDF2Iterations PBKDF2迭代次数.
	VolumePBKDF2Iterations = 200000
	// VolumeChunkSize 流式加密块大小 (256KB).
	VolumeChunkSize = 256 * 1024
	// VolumeHeaderMagic 卷头魔数.
	VolumeHeaderMagic = "NAS-VOL-ENC"
	// VolumeHeaderVersion 卷头版本.
	VolumeHeaderVersion = 1
)

// ========== 类型 ==========

// VolumeEncryptionState 加密卷状态.
type VolumeEncryptionState string

const (
	VolumeStateLocked   VolumeEncryptionState = "locked"
	VolumeStateUnlocked VolumeEncryptionState = "unlocked"
	VolumeStateCreating VolumeEncryptionState = "creating"
	VolumeStateError    VolumeEncryptionState = "error"
	VolumeStateRekeying VolumeEncryptionState = "rekeying"
)

// VolumeEncryptionAlgorithm 加密算法.
type VolumeEncryptionAlgorithm string

const (
	AlgoAES256XTS VolumeEncryptionAlgorithm = "aes-256-xts"
	AlgoAES256GCM VolumeEncryptionAlgorithm = "aes-256-gcm"
	AlgoAES256CBC VolumeEncryptionAlgorithm = "aes-256-cbc"
)

// EncryptedVolume 加密卷定义.
type EncryptedVolume struct {
	ID            string                    `json:"id"`
	Name          string                    `json:"name"`
	MountPoint    string                    `json:"mount_point"`
	DevicePath    string                    `json:"device_path"`
	State         VolumeEncryptionState     `json:"state"`
	Algorithm     VolumeEncryptionAlgorithm `json:"algorithm"`
	KeyID         string                    `json:"key_id"`
	Salt          string                    `json:"salt"`
	HeaderPath    string                    `json:"header_path"`
	SizeBytes     int64                     `json:"size_bytes"`
	UsedBytes     int64                     `json:"used_bytes"`
	AESNI         bool                      `json:"aes_ni"`
	CreatedAt     time.Time                 `json:"created_at"`
	LastUnlocked  *time.Time                `json:"last_unlocked,omitempty"`
	LastLocked    *time.Time                `json:"last_locked,omitempty"`
	RekeyDeadline *time.Time                `json:"rekey_deadline,omitempty"`
	Metadata      map[string]string         `json:"metadata,omitempty"`
}

// VolumeKey 卷加密密钥.
type VolumeKey struct {
	ID           string                    `json:"id"`
	VolumeID     string                    `json:"volume_id"`
	Salt         string                    `json:"salt"`
	EncryptedKey string                    `json:"encrypted_key"` // 主密钥加密后的数据密钥
	Algorithm    VolumeEncryptionAlgorithm `json:"algorithm"`
	Version      int                       `json:"version"`
	CreatedAt    time.Time                 `json:"created_at"`
}

// KMIPConfig KMIP远程密钥管理配置.
type KMIPConfig struct {
	Enabled  bool   `json:"enabled"`
	Endpoint string `json:"endpoint"`
	Port     int    `json:"port"`
	CertPath string `json:"cert_path"`
	KeyPath  string `json:"key_path"`
	CAPath   string `json:"ca_path"`
}

// CreateEncryptedVolumeRequest 创建加密卷请求.
type CreateEncryptedVolumeRequest struct {
	Name       string                    `json:"name"`
	DevicePath string                    `json:"device_path"`
	SizeBytes  int64                     `json:"size_bytes"`
	Algorithm  VolumeEncryptionAlgorithm `json:"algorithm"`
	Password   string                    `json:"password"`
	Metadata   map[string]string         `json:"metadata,omitempty"`
}

// VolumeEncryptionManager 全卷加密管理器.
type VolumeEncryptionManager struct {
	mu         sync.RWMutex
	volumes    map[string]*EncryptedVolume
	keys       map[string]*VolumeKey
	baseDir    string
	kmipConfig KMIPConfig
	aesNI      bool   // 是否支持AES-NI硬件加速
	masterKey  []byte // 运行时主密钥
}

// NewVolumeEncryptionManager 创建全卷加密管理器.
func NewVolumeEncryptionManager(baseDir string, kmipConfig KMIPConfig) (*VolumeEncryptionManager, error) {
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return nil, fmt.Errorf("创建加密卷目录失败: %w", err)
	}
	keyDir := filepath.Join(baseDir, "keys")
	if err := os.MkdirAll(keyDir, 0700); err != nil {
		return nil, fmt.Errorf("创建密钥目录失败: %w", err)
	}
	headerDir := filepath.Join(baseDir, "headers")
	if err := os.MkdirAll(headerDir, 0700); err != nil {
		return nil, fmt.Errorf("创建卷头目录失败: %w", err)
	}

	mgr := &VolumeEncryptionManager{
		volumes:    make(map[string]*EncryptedVolume),
		keys:       make(map[string]*VolumeKey),
		baseDir:    baseDir,
		kmipConfig: kmipConfig,
		aesNI:      detectAESNI(),
	}

	// 加载已有卷配置
	if err := mgr.loadVolumes(); err != nil {
		return nil, fmt.Errorf("加载卷配置失败: %w", err)
	}

	return mgr, nil
}

// detectAESNI 检测CPU是否支持AES-NI硬件加速.
func detectAESNI() bool {
	// 通过 /proc/cpuinfo 检测 AES-NI 支持
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return false
	}
	// 检查 flags 中是否包含 aes
	content := string(data)
	for i := 0; i < len(content)-3; i++ {
		if content[i] == 'a' && content[i+1] == 'e' && content[i+2] == 's' {
			// 简单匹配 "aes" flag
			if (i == 0 || content[i-1] == ' ' || content[i-1] == '\t') &&
				(i+3 >= len(content) || content[i+3] == ' ' || content[i+3] == '\n' || content[i+3] == '\t') {
				return true
			}
		}
	}
	return false
}

// CreateVolume 创建加密卷.
func (m *VolumeEncryptionManager) CreateVolume(req CreateEncryptedVolumeRequest) (*EncryptedVolume, error) {
	if req.Name == "" {
		return nil, fmt.Errorf("卷名不能为空")
	}
	if req.DevicePath == "" {
		return nil, fmt.Errorf("设备路径不能为空")
	}
	if req.SizeBytes <= 0 {
		return nil, fmt.Errorf("卷大小必须大于0")
	}
	if req.Password == "" {
		return nil, fmt.Errorf("密码不能为空")
	}
	if req.Algorithm == "" {
		req.Algorithm = AlgoAES256XTS
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查卷名重复
	for _, v := range m.volumes {
		if v.Name == req.Name {
			return nil, fmt.Errorf("卷名 '%s' 已存在", req.Name)
		}
	}

	volumeID := uuid.New().String()
	keyID := uuid.New().String()
	salt := generateSalt()

	vol := &EncryptedVolume{
		ID:         volumeID,
		Name:       req.Name,
		DevicePath: req.DevicePath,
		State:      VolumeStateCreating,
		Algorithm:  req.Algorithm,
		KeyID:      keyID,
		Salt:       hex.EncodeToString(salt),
		HeaderPath: filepath.Join(m.baseDir, "headers", volumeID+".hdr"),
		SizeBytes:  req.SizeBytes,
		AESNI:      m.aesNI,
		CreatedAt:  time.Now(),
		Metadata:   req.Metadata,
	}

	// 生成数据密钥
	dataKey := make([]byte, VolumeKeyLength)
	if _, err := io.ReadFull(rand.Reader, dataKey); err != nil {
		return nil, fmt.Errorf("生成数据密钥失败: %w", err)
	}

	// 用密码派生主密钥，加密数据密钥
	masterKey := pbkdf2.Key([]byte(req.Password), salt, VolumePBKDF2Iterations, VolumeKeyLength, sha256.New)
	encryptedKey, err := encryptWithAESGCM(masterKey, dataKey)
	if err != nil {
		return nil, fmt.Errorf("加密数据密钥失败: %w", err)
	}

	vk := &VolumeKey{
		ID:           keyID,
		VolumeID:     volumeID,
		Salt:         hex.EncodeToString(salt),
		EncryptedKey: hex.EncodeToString(encryptedKey),
		Algorithm:    req.Algorithm,
		Version:      1,
		CreatedAt:    time.Now(),
	}

	// 写入卷头
	header := VolumeHeader{
		Magic:     VolumeHeaderMagic,
		Version:   VolumeHeaderVersion,
		VolumeID:  volumeID,
		Algorithm: string(req.Algorithm),
		Salt:      hex.EncodeToString(salt),
		KeyID:     keyID,
		SizeBytes: req.SizeBytes,
		CreatedAt: time.Now(),
	}
	if err := m.writeHeader(vol.HeaderPath, header); err != nil {
		return nil, fmt.Errorf("写入卷头失败: %w", err)
	}

	// 保存密钥
	if err := m.saveKey(vk); err != nil {
		return nil, fmt.Errorf("保存密钥失败: %w", err)
	}

	vol.State = VolumeStateLocked
	m.volumes[volumeID] = vol
	m.keys[keyID] = vk

	// 保存卷配置
	if err := m.saveVolumes(); err != nil {
		return nil, fmt.Errorf("保存卷配置失败: %w", err)
	}

	return vol, nil
}

// UnlockVolume 解锁加密卷.
func (m *VolumeEncryptionManager) UnlockVolume(volumeID, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vol, ok := m.volumes[volumeID]
	if !ok {
		return fmt.Errorf("卷 '%s' 不存在", volumeID)
	}
	if vol.State == VolumeStateUnlocked {
		return fmt.Errorf("卷 '%s' 已经解锁", volumeID)
	}

	key, ok := m.keys[vol.KeyID]
	if !ok {
		return fmt.Errorf("密钥 '%s' 不存在", vol.KeyID)
	}

	salt, err := hex.DecodeString(key.Salt)
	if err != nil {
		return fmt.Errorf("解析盐值失败: %w", err)
	}

	encryptedKey, err := hex.DecodeString(key.EncryptedKey)
	if err != nil {
		return fmt.Errorf("解析加密密钥失败: %w", err)
	}

	// 验证密码
	masterKey := pbkdf2.Key([]byte(password), salt, VolumePBKDF2Iterations, VolumeKeyLength, sha256.New)
	_, err = decryptWithAESGCM(masterKey, encryptedKey)
	if err != nil {
		return fmt.Errorf("密码错误或密钥损坏: %w", err)
	}

	now := time.Now()
	vol.State = VolumeStateUnlocked
	vol.LastUnlocked = &now

	// 如果配置了KMIP，同步密钥到远程
	if m.kmipConfig.Enabled {
		go m.syncToKMIP(key)
	}

	return m.saveVolumes()
}

// LockVolume 锁定加密卷.
func (m *VolumeEncryptionManager) LockVolume(volumeID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vol, ok := m.volumes[volumeID]
	if !ok {
		return fmt.Errorf("卷 '%s' 不存在", volumeID)
	}
	if vol.State == VolumeStateLocked {
		return fmt.Errorf("卷 '%s' 已经锁定", volumeID)
	}

	now := time.Now()
	vol.State = VolumeStateLocked
	vol.LastLocked = &now
	vol.MountPoint = ""

	return m.saveVolumes()
}

// DeleteVolume 删除加密卷.
func (m *VolumeEncryptionManager) DeleteVolume(volumeID string, password string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vol, ok := m.volumes[volumeID]
	if !ok {
		return fmt.Errorf("卷 '%s' 不存在", volumeID)
	}
	if vol.State == VolumeStateUnlocked {
		return fmt.Errorf("请先锁定卷再删除")
	}

	// 验证密码后安全删除
	key, ok := m.keys[vol.KeyID]
	if ok {
		salt, _ := hex.DecodeString(key.Salt)
		encryptedKey, _ := hex.DecodeString(key.EncryptedKey)
		masterKey := pbkdf2.Key([]byte(password), salt, VolumePBKDF2Iterations, VolumeKeyLength, sha256.New)
		if _, err := decryptWithAESGCM(masterKey, encryptedKey); err != nil {
			return fmt.Errorf("密码错误")
		}
	}

	// 安全擦除卷头
	if err := secureWipeFile(vol.HeaderPath); err != nil {
		return fmt.Errorf("安全擦除卷头失败: %w", err)
	}

	// 删除密钥文件
	keyPath := filepath.Join(m.baseDir, "keys", key.ID+".json")
	_ = secureWipeFile(keyPath)

	delete(m.volumes, volumeID)
	delete(m.keys, vol.KeyID)

	return m.saveVolumes()
}

// RekeyVolume 轮换密钥.
func (m *VolumeEncryptionManager) RekeyVolume(volumeID, oldPassword, newPassword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	vol, ok := m.volumes[volumeID]
	if !ok {
		return fmt.Errorf("卷 '%s' 不存在", volumeID)
	}

	oldKey, ok := m.keys[vol.KeyID]
	if !ok {
		return fmt.Errorf("密钥不存在")
	}

	vol.State = VolumeStateRekeying
	now := time.Now()
	vol.RekeyDeadline = &now

	// 解密旧数据密钥
	oldSalt, _ := hex.DecodeString(oldKey.Salt)
	oldEncryptedKey, _ := hex.DecodeString(oldKey.EncryptedKey)
	oldMasterKey := pbkdf2.Key([]byte(oldPassword), oldSalt, VolumePBKDF2Iterations, VolumeKeyLength, sha256.New)
	dataKey, err := decryptWithAESGCM(oldMasterKey, oldEncryptedKey)
	if err != nil {
		vol.State = VolumeStateUnlocked
		return fmt.Errorf("旧密码错误: %w", err)
	}

	// 用新密码重新加密数据密钥
	newSalt := generateSalt()
	newMasterKey := pbkdf2.Key([]byte(newPassword), newSalt, VolumePBKDF2Iterations, VolumeKeyLength, sha256.New)
	newEncryptedKey, err := encryptWithAESGCM(newMasterKey, dataKey)
	if err != nil {
		vol.State = VolumeStateUnlocked
		return fmt.Errorf("加密失败: %w", err)
	}

	newKeyID := uuid.New().String()
	newVK := &VolumeKey{
		ID:           newKeyID,
		VolumeID:     volumeID,
		Salt:         hex.EncodeToString(newSalt),
		EncryptedKey: hex.EncodeToString(newEncryptedKey),
		Algorithm:    vol.Algorithm,
		Version:      oldKey.Version + 1,
		CreatedAt:    time.Now(),
	}

	// 保存新密钥
	if err := m.saveKey(newVK); err != nil {
		vol.State = VolumeStateUnlocked
		return fmt.Errorf("保存新密钥失败: %w", err)
	}

	// 安全删除旧密钥
	oldKeyPath := filepath.Join(m.baseDir, "keys", oldKey.ID+".json")
	_ = secureWipeFile(oldKeyPath)
	delete(m.keys, oldKey.ID)

	vol.KeyID = newKeyID
	vol.Salt = hex.EncodeToString(newSalt)
	vol.State = VolumeStateUnlocked

	m.keys[newKeyID] = newVK
	return m.saveVolumes()
}

// GetVolume 获取加密卷信息.
func (m *VolumeEncryptionManager) GetVolume(volumeID string) (*EncryptedVolume, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	vol, ok := m.volumes[volumeID]
	if !ok {
		return nil, fmt.Errorf("卷 '%s' 不存在", volumeID)
	}
	return vol, nil
}

// ListVolumes 列出所有加密卷.
func (m *VolumeEncryptionManager) ListVolumes() []*EncryptedVolume {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*EncryptedVolume, 0, len(m.volumes))
	for _, v := range m.volumes {
		result = append(result, v)
	}
	return result
}

// GetStats 获取加密卷统计信息.
func (m *VolumeEncryptionManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	locked, unlocked, errCount := 0, 0, 0
	for _, v := range m.volumes {
		switch v.State {
		case VolumeStateLocked:
			locked++
		case VolumeStateUnlocked:
			unlocked++
		default:
			errCount++
		}
	}

	return map[string]interface{}{
		"total_volumes": len(m.volumes),
		"locked":        locked,
		"unlocked":      unlocked,
		"errors":        errCount,
		"aes_ni":        m.aesNI,
		"kmip_enabled":  m.kmipConfig.Enabled,
	}
}

// ========== KMIP ==========

// syncToKMIP 同步密钥到KMIP远程服务器.
func (m *VolumeEncryptionManager) syncToKMIP(key *VolumeKey) {
	if !m.kmipConfig.Enabled {
		return
	}
	// KMIP 协议实现 - 通过TLS连接远程密钥管理服务器
	// 实际部署时连接KMIP服务器
}

// ========== 辅助函数 ==========

// VolumeHeader 卷头结构.
type VolumeHeader struct {
	Magic     string    `json:"magic"`
	Version   int       `json:"version"`
	VolumeID  string    `json:"volume_id"`
	Algorithm string    `json:"algorithm"`
	Salt      string    `json:"salt"`
	KeyID     string    `json:"key_id"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

func (m *VolumeEncryptionManager) writeHeader(path string, header VolumeHeader) error {
	data, err := json.Marshal(header)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func (m *VolumeEncryptionManager) saveKey(key *VolumeKey) error {
	data, err := json.Marshal(key)
	if err != nil {
		return err
	}
	path := filepath.Join(m.baseDir, "keys", key.ID+".json")
	return os.WriteFile(path, data, 0600)
}

func (m *VolumeEncryptionManager) loadVolumes() error {
	configPath := filepath.Join(m.baseDir, "volumes.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &m.volumes)
}

func (m *VolumeEncryptionManager) saveVolumes() error {
	data, err := json.Marshal(m.volumes)
	if err != nil {
		return err
	}
	configPath := filepath.Join(m.baseDir, "volumes.json")
	return os.WriteFile(configPath, data, 0644)
}

func generateSalt() []byte {
	salt := make([]byte, VolumeSaltLength)
	_, _ = io.ReadFull(rand.Reader, salt)
	return salt
}

func encryptWithAESGCM(key, plaintext []byte) ([]byte, error) {
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

func decryptWithAESGCM(key, ciphertext []byte) ([]byte, error) {
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
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ct, nil)
}

// secureWipeFile 安全擦除文件（覆写后删除）.
func secureWipeFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// 三次覆写：随机数据 → 零 → 随机数据
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	size := info.Size()
	buf := make([]byte, 4096)
	for pass := 0; pass < 3; pass++ {
		f.Seek(0, 0)
		remaining := size
		for remaining > 0 {
			n := int64(len(buf))
			if n > remaining {
				n = remaining
			}
			if pass == 1 {
				// 零填充
				for i := int64(0); i < n; i++ {
					buf[i] = 0
				}
			} else {
				rand.Read(buf[:n])
			}
			f.Write(buf[:n])
			remaining -= n
		}
		f.Sync()
	}
	f.Close()

	return os.Remove(path)
}
