package auth

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

	"golang.org/x/crypto/pbkdf2"
)

// SecretEncryption 敏感数据加密器
// 用于加密存储 TOTP Secret、备份码等敏感数据
type SecretEncryption struct {
	mu       sync.RWMutex
	key      []byte
	keyPath  string
	initialized bool
}

var (
	ErrEncryptionNotInitialized = errors.New("加密器未初始化")
	ErrEncryptionFailed         = errors.New("加密失败")
	ErrDecryptionFailed         = errors.New("解密失败")
)

// NewSecretEncryption 创建敏感数据加密器
func NewSecretEncryption(keyPath string) *SecretEncryption {
	se := &SecretEncryption{
		keyPath: keyPath,
	}
	
	// 尝试加载已有密钥
	if key, err := se.loadKey(); err == nil {
		se.key = key
		se.initialized = true
	}
	
	return se
}

// Initialize 初始化加密器（生成新密钥或加载已有密钥）
func (se *SecretEncryption) Initialize(passphrase string) error {
	se.mu.Lock()
	defer se.mu.Unlock()
	
	// 尝试加载已有密钥
	if key, err := se.loadKey(); err == nil {
		se.key = key
		se.initialized = true
		return nil
	}
	
	// 生成新密钥
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	
	// 使用 PBKDF2 派生密钥
	key := pbkdf2.Key([]byte(passphrase), salt, 100000, 32, sha256.New)
	
	// 存储密钥（盐 + 密钥）
	data := append(salt, key...)
	if err := os.MkdirAll(filepath.Dir(se.keyPath), 0700); err != nil {
		return err
	}
	if err := os.WriteFile(se.keyPath, data, 0600); err != nil {
		return err
	}
	
	se.key = key
	se.initialized = true
	return nil
}

// loadKey 加载已存储的密钥
func (se *SecretEncryption) loadKey() ([]byte, error) {
	if se.keyPath == "" {
		return nil, errors.New("密钥路径未设置")
	}
	
	data, err := os.ReadFile(se.keyPath)
	if err != nil {
		return nil, err
	}
	
	if len(data) < 48 { // 16 salt + 32 key
		return nil, errors.New("无效的密钥文件")
	}
	
	return data[16:], nil
}

// Encrypt 加密数据
func (se *SecretEncryption) Encrypt(plaintext string) (string, error) {
	se.mu.RLock()
	defer se.mu.RUnlock()
	
	if !se.initialized {
		return "", ErrEncryptionNotInitialized
	}
	
	block, err := aes.NewCipher(se.key)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	
	// 将 nonce 附加到密文
	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result[:len(nonce)], nonce)
	copy(result[len(nonce):], ciphertext)
	
	return base64.StdEncoding.EncodeToString(result), nil
}

// Decrypt 解密数据
func (se *SecretEncryption) Decrypt(ciphertext string) (string, error) {
	se.mu.RLock()
	defer se.mu.RUnlock()
	
	if !se.initialized {
		return "", ErrEncryptionNotInitialized
	}
	
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}
	
	block, err := aes.NewCipher(se.key)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", ErrDecryptionFailed
	}
	
	nonce := data[:nonceSize]
	ciphertextBytes := data[nonceSize:]
	
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}
	
	return string(plaintext), nil
}

// IsInitialized 检查是否已初始化
func (se *SecretEncryption) IsInitialized() bool {
	se.mu.RLock()
	defer se.mu.RUnlock()
	return se.initialized
}

// EncryptBytes 加密字节数据
func (se *SecretEncryption) EncryptBytes(plaintext []byte) (string, error) {
	se.mu.RLock()
	defer se.mu.RUnlock()
	
	if !se.initialized {
		return "", ErrEncryptionNotInitialized
	}
	
	block, err := aes.NewCipher(se.key)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	result := append(nonce, ciphertext...)
	
	return base64.StdEncoding.EncodeToString(result), nil
}

// DecryptBytes 解密字节数据
func (se *SecretEncryption) DecryptBytes(ciphertext string) ([]byte, error) {
	se.mu.RLock()
	defer se.mu.RUnlock()
	
	if !se.initialized {
		return nil, ErrEncryptionNotInitialized
	}
	
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, err
	}
	
	block, err := aes.NewCipher(se.key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, ErrDecryptionFailed
	}
	
	nonce := data[:nonceSize]
	ciphertextBytes := data[nonceSize:]
	
	return gcm.Open(nil, nonce, ciphertextBytes, nil)
}