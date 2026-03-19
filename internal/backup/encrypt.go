package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/pbkdf2"
)

var (
	// ErrInvalidKey 无效的加密密钥错误
	ErrInvalidKey       = errors.New("invalid encryption key")
	ErrEncryptionFailed = errors.New("encryption failed")
	ErrDecryptionFailed = errors.New("decryption failed")
	ErrKeyNotFound      = errors.New("encryption key not found")
)

// EncryptionManagerConfig 加密管理器配置
type EncryptionManagerConfig struct {
	Enabled       bool   `json:"enabled"`
	Algorithm     string `json:"algorithm"`      // aes-256-gcm, aes-256-cbc
	KeyDerivation string `json:"key_derivation"` // pbkdf2, argon2
	KeyPath       string `json:"key_path"`       // 密钥存储路径
	SaltLength    int    `json:"salt_length"`
	Iterations    int    `json:"iterations"` // PBKDF2 迭代次数
}

// EncryptionManager 加密管理器
type EncryptionManager struct {
	config   *EncryptionManagerConfig
	keys     map[string]*EncryptionKey
	keyStore *KeyStore
	mu       sync.RWMutex
	logger   *zap.Logger
}

// EncryptionKey 加密密钥
type EncryptionKey struct {
	ID        string `json:"id"`
	Key       []byte `json:"key"`
	Salt      []byte `json:"salt"`
	CreatedAt string `json:"created_at"`
	Algorithm string `json:"algorithm"`
}

// KeyStore 密钥存储
type KeyStore struct {
	path string
	keys map[string]*EncryptionKey
	mu   sync.RWMutex
}

// NewKeyStore 创建密钥存储
func NewKeyStore(path string) *KeyStore {
	ks := &KeyStore{
		path: path,
		keys: make(map[string]*EncryptionKey),
	}
	ks.load()
	return ks
}

// load 加载密钥
func (ks *KeyStore) load() {
	// 确保目录存在
	_ = os.MkdirAll(ks.path, 0700)

	_ = filepath.Walk(ks.path, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// 解析密钥（简化实现）
		key := &EncryptionKey{
			ID:  filepath.Base(path),
			Key: data,
		}
		ks.keys[key.ID] = key
		return nil
	})
}

// Store 存储密钥
func (ks *KeyStore) Store(key *EncryptionKey) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	path := filepath.Join(ks.path, key.ID)
	if err := os.WriteFile(path, key.Key, 0600); err != nil {
		return err
	}

	ks.keys[key.ID] = key
	return nil
}

// Get 获取密钥
func (ks *KeyStore) Get(id string) (*EncryptionKey, bool) {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	key, exists := ks.keys[id]
	return key, exists
}

// Delete 删除密钥
func (ks *KeyStore) Delete(id string) error {
	ks.mu.Lock()
	defer ks.mu.Unlock()

	path := filepath.Join(ks.path, id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}

	delete(ks.keys, id)
	return nil
}

// List 列出所有密钥ID
func (ks *KeyStore) List() []string {
	ks.mu.RLock()
	defer ks.mu.RUnlock()

	ids := make([]string, 0, len(ks.keys))
	for id := range ks.keys {
		ids = append(ids, id)
	}
	return ids
}

// NewEncryptionManager 创建加密管理器
func NewEncryptionManager(config *EncryptionManagerConfig, logger *zap.Logger) *EncryptionManager {
	em := &EncryptionManager{
		config:   config,
		keys:     make(map[string]*EncryptionKey),
		keyStore: NewKeyStore(config.KeyPath),
		logger:   logger,
	}

	// 加载已有密钥
	for _, id := range em.keyStore.List() {
		if key, exists := em.keyStore.Get(id); exists {
			em.keys[id] = key
		}
	}

	return em
}

// GenerateKey 生成密钥
func (em *EncryptionManager) GenerateKey(passphrase string) (*EncryptionKey, error) {
	// 生成盐
	salt := make([]byte, em.config.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	// 使用 PBKDF2 派生密钥
	key := pbkdf2.Key([]byte(passphrase), salt, em.config.Iterations, 32, sha256.New)

	encKey := &EncryptionKey{
		ID:        generateKeyID(),
		Key:       key,
		Salt:      salt,
		CreatedAt: currentTime(),
		Algorithm: em.config.Algorithm,
	}

	// 存储密钥
	if err := em.keyStore.Store(encKey); err != nil {
		return nil, err
	}

	em.mu.Lock()
	em.keys[encKey.ID] = encKey
	em.mu.Unlock()

	em.logger.Info("Encryption key generated",
		zap.String("key_id", encKey.ID),
		zap.String("algorithm", encKey.Algorithm),
	)

	return encKey, nil
}

// DeriveKey 从密码派生密钥
func (em *EncryptionManager) DeriveKey(passphrase string, salt []byte) []byte {
	return pbkdf2.Key([]byte(passphrase), salt, em.config.Iterations, 32, sha256.New)
}

// Encrypt 加密数据
func (em *EncryptionManager) Encrypt(plaintext []byte, keyID string) ([]byte, error) {
	em.mu.RLock()
	key, exists := em.keys[keyID]
	em.mu.RUnlock()

	if !exists {
		return nil, ErrKeyNotFound
	}

	switch em.config.Algorithm {
	case "aes-256-gcm":
		return em.encryptGCM(plaintext, key.Key)
	case "aes-256-cbc":
		return em.encryptCBC(plaintext, key.Key)
	default:
		return em.encryptGCM(plaintext, key.Key)
	}
}

// encryptGCM 使用 AES-GCM 加密
func (em *EncryptionManager) encryptGCM(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// 生成随机 nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	// 加密
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// 将 nonce 附加到密文
	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result[:len(nonce)], nonce)
	copy(result[len(nonce):], ciphertext)

	return result, nil
}

// encryptCBC 使用 AES-CBC 加密
func (em *EncryptionManager) encryptCBC(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// PKCS7 填充
	blockSize := block.BlockSize()
	padding := blockSize - len(plaintext)%blockSize
	padded := make([]byte, len(plaintext)+padding)
	copy(padded, plaintext)
	for i := len(plaintext); i < len(padded); i++ {
		padded[i] = byte(padding)
	}

	// 生成随机 IV
	iv := make([]byte, block.BlockSize())
	if _, err := rand.Read(iv); err != nil {
		return nil, err
	}

	// 加密
	ciphertext := make([]byte, len(padded))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, padded)

	// 将 IV 附加到密文
	result := make([]byte, len(iv)+len(ciphertext))
	copy(result[:len(iv)], iv)
	copy(result[len(iv):], ciphertext)

	return result, nil
}

// Decrypt 解密数据
func (em *EncryptionManager) Decrypt(ciphertext []byte, keyID string) ([]byte, error) {
	em.mu.RLock()
	key, exists := em.keys[keyID]
	em.mu.RUnlock()

	if !exists {
		return nil, ErrKeyNotFound
	}

	switch em.config.Algorithm {
	case "aes-256-gcm":
		return em.decryptGCM(ciphertext, key.Key)
	case "aes-256-cbc":
		return em.decryptCBC(ciphertext, key.Key)
	default:
		return em.decryptGCM(ciphertext, key.Key)
	}
}

// decryptGCM 使用 AES-GCM 解密
func (em *EncryptionManager) decryptGCM(ciphertext, key []byte) ([]byte, error) {
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
		return nil, ErrDecryptionFailed
	}

	// 提取 nonce
	nonce := ciphertext[:nonceSize]
	ciphertext = ciphertext[nonceSize:]

	// 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// decryptCBC 使用 AES-CBC 解密
func (em *EncryptionManager) decryptCBC(ciphertext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	if len(ciphertext) < blockSize {
		return nil, ErrDecryptionFailed
	}

	// 提取 IV
	iv := ciphertext[:blockSize]
	ciphertext = ciphertext[blockSize:]

	// 解密
	plaintext := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	// 移除 PKCS7 填充
	padding := int(plaintext[len(plaintext)-1])
	if padding > blockSize || padding > len(plaintext) {
		return nil, ErrDecryptionFailed
	}

	return plaintext[:len(plaintext)-padding], nil
}

// EncryptFile 加密文件
func (em *EncryptionManager) EncryptFile(srcPath, dstPath, keyID string) error {
	// 读取源文件
	plaintext, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// 加密
	ciphertext, err := em.Encrypt(plaintext, keyID)
	if err != nil {
		return err
	}

	// 写入目标文件
	return os.WriteFile(dstPath, ciphertext, 0600)
}

// DecryptFile 解密文件
func (em *EncryptionManager) DecryptFile(srcPath, dstPath, keyID string) error {
	// 读取源文件
	ciphertext, err := os.ReadFile(srcPath)
	if err != nil {
		return err
	}

	// 解密
	plaintext, err := em.Decrypt(ciphertext, keyID)
	if err != nil {
		return err
	}

	// 写入目标文件
	return os.WriteFile(dstPath, plaintext, 0600)
}

// RotateKey 密钥轮换
func (em *EncryptionManager) RotateKey(oldKeyID, newPassphrase string) (*EncryptionKey, error) {
	// 生成新密钥
	newKey, err := em.GenerateKey(newPassphrase)
	if err != nil {
		return nil, err
	}

	em.logger.Info("Key rotated",
		zap.String("old_key_id", oldKeyID),
		zap.String("new_key_id", newKey.ID),
	)

	return newKey, nil
}

// DeleteKey 删除密钥
func (em *EncryptionManager) DeleteKey(keyID string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if err := em.keyStore.Delete(keyID); err != nil {
		return err
	}

	delete(em.keys, keyID)

	em.logger.Info("Key deleted", zap.String("key_id", keyID))
	return nil
}

// ListKeys 列出密钥
func (em *EncryptionManager) ListKeys() []string {
	em.mu.RLock()
	defer em.mu.RUnlock()

	ids := make([]string, 0, len(em.keys))
	for id := range em.keys {
		ids = append(ids, id)
	}
	return ids
}

// GetKeyInfo 获取密钥信息
func (em *EncryptionManager) GetKeyInfo(keyID string) (map[string]interface{}, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	key, exists := em.keys[keyID]
	if !exists {
		return nil, ErrKeyNotFound
	}

	return map[string]interface{}{
		"id":         key.ID,
		"algorithm":  key.Algorithm,
		"created_at": key.CreatedAt,
	}, nil
}

// EncryptStream 加密流
func (em *EncryptionManager) EncryptStream(reader io.Reader, writer io.Writer, keyID string) error {
	em.mu.RLock()
	key, exists := em.keys[keyID]
	em.mu.RUnlock()

	if !exists {
		return ErrKeyNotFound
	}

	// 读取所有数据
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	// 加密
	encrypted, err := em.encryptGCM(data, key.Key)
	if err != nil {
		return err
	}

	// 写入
	_, err = writer.Write(encrypted)
	return err
}

// DecryptStream 解密流
func (em *EncryptionManager) DecryptStream(reader io.Reader, writer io.Writer, keyID string) error {
	em.mu.RLock()
	key, exists := em.keys[keyID]
	em.mu.RUnlock()

	if !exists {
		return ErrKeyNotFound
	}

	// 读取所有数据
	data, err := io.ReadAll(reader)
	if err != nil {
		return err
	}

	// 解密
	decrypted, err := em.decryptGCM(data, key.Key)
	if err != nil {
		return err
	}

	// 写入
	_, err = writer.Write(decrypted)
	return err
}

// ExportKey 导出密钥（Base64编码）
func (em *EncryptionManager) ExportKey(keyID string) (string, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	key, exists := em.keys[keyID]
	if !exists {
		return "", ErrKeyNotFound
	}

	// 将密钥和盐打包
	data := append(key.Salt, key.Key...)
	return base64.StdEncoding.EncodeToString(data), nil
}

// ImportKey 导入密钥
func (em *EncryptionManager) ImportKey(encodedKey, algorithm string) (*EncryptionKey, error) {
	data, err := base64.StdEncoding.DecodeString(encodedKey)
	if err != nil {
		return nil, err
	}

	if len(data) < 32 {
		return nil, ErrInvalidKey
	}

	salt := data[:16]
	key := data[16:]

	encKey := &EncryptionKey{
		ID:        generateKeyID(),
		Key:       key,
		Salt:      salt,
		CreatedAt: currentTime(),
		Algorithm: algorithm,
	}

	if err := em.keyStore.Store(encKey); err != nil {
		return nil, err
	}

	em.mu.Lock()
	em.keys[encKey.ID] = encKey
	em.mu.Unlock()

	return encKey, nil
}

// EncryptionStats 加密统计
type EncryptionStats struct {
	Enabled   bool     `json:"enabled"`
	Algorithm string   `json:"algorithm"`
	KeyCount  int      `json:"key_count"`
	KeyIDs    []string `json:"key_ids"`
}

// GetStats 获取统计
func (em *EncryptionManager) GetStats() EncryptionStats {
	em.mu.RLock()
	defer em.mu.RUnlock()

	ids := make([]string, 0, len(em.keys))
	for id := range em.keys {
		ids = append(ids, id)
	}

	return EncryptionStats{
		Enabled:   em.config.Enabled,
		Algorithm: em.config.Algorithm,
		KeyCount:  len(em.keys),
		KeyIDs:    ids,
	}
}

// generateKeyID 生成密钥ID
func generateKeyID() string {
	return "key-" + randomString(8)
}

// currentTime 获取当前时间字符串
func currentTime() string {
	return timeNow().Format("2006-01-02T15:04:05Z07:00")
}

// 时间函数，便于测试
var timeNow = func() time.Time {
	return time.Now()
}
