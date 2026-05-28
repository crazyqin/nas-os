// Package privacyvault - crypto.go 实现加密引擎，包括 AES-256-GCM 加密解密、
// PBKDF2 密钥派生、密钥轮换和数据完整性校验。
package privacyvault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

const (
	// DefaultPBKDF2Iterations PBKDF2 默认迭代次数
	DefaultPBKDF2Iterations = 100000
	// DefaultSaltSize 默认盐值长度（字节）
	DefaultSaltSize = 32
	// DefaultKeySize 默认密钥长度（字节）
	DefaultKeySize = 32
)

// CryptoEngine 加密引擎
type CryptoEngine struct {
	algorithm  EncryptionAlgorithm
	keySize    int
	saltSize   int
	iterations int
}

// NewCryptoEngine 创建加密引擎
func NewCryptoEngine(algorithm EncryptionAlgorithm) *CryptoEngine {
	return &CryptoEngine{
		algorithm:  algorithm,
		keySize:    DefaultKeySize,
		saltSize:   DefaultSaltSize,
		iterations: DefaultPBKDF2Iterations,
	}
}

// DeriveKey 使用 PBKDF2-SHA256 从密码派生密钥
// 返回派生密钥和盐值
func (ce *CryptoEngine) DeriveKey(passphrase string) ([]byte, []byte, error) {
	salt := make([]byte, ce.saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, nil, NewPrivacyVaultError("KEY_DERIVATION_FAILED", "生成盐值失败", err)
	}

	key := ce.deriveKeyWithSalt(passphrase, salt)
	return key, salt, nil
}

// DeriveKeyWithSalt 使用指定盐值派生密钥
func (ce *CryptoEngine) DeriveKeyWithSalt(passphrase string, salt []byte) []byte {
	return ce.deriveKeyWithSalt(passphrase, salt)
}

func (ce *CryptoEngine) deriveKeyWithSalt(passphrase string, salt []byte) []byte {
	return pbkdf2([]byte(passphrase), salt, ce.iterations, ce.keySize)
}

// pbkdf2 PBKDF2-SHA256 密钥派生实现
func pbkdf2(password, salt []byte, iterations, keyLen int) []byte {
	key := make([]byte, keyLen)
	blockCount := (keyLen + sha256.Size - 1) / sha256.Size

	for i := 1; i <= blockCount; i++ {
		// U1 = HMAC(password, salt || INT(i))
		mac := hmac.New(sha256.New, password)
		mac.Write(salt)
		mac.Write([]byte{byte(i >> 24), byte(i >> 16), byte(i >> 8), byte(i)})
		U := mac.Sum(nil)
		T := make([]byte, len(U))
		copy(T, U)

		// U2..Uc
		for j := 1; j < iterations; j++ {
			mac = hmac.New(sha256.New, password)
			mac.Write(U)
			U = mac.Sum(nil)
			for k := range T {
				T[k] ^= U[k]
			}
		}

		offset := (i - 1) * sha256.Size
		end := offset + sha256.Size
		if end > keyLen {
			end = keyLen
		}
		copy(key[offset:end], T[:end-offset])
	}

	return key
}

// Encrypt 使用 AES-256-GCM 加密数据
func (ce *CryptoEngine) Encrypt(key, plaintext []byte) ([]byte, error) {
	if len(key) != ce.keySize {
		return nil, NewPrivacyVaultError("INVALID_KEY_SIZE", "密钥长度不正确", nil)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, NewPrivacyVaultError("CIPHER_CREATION_FAILED", "创建加密块失败", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, NewPrivacyVaultError("GCM_CREATION_FAILED", "创建 GCM 失败", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, NewPrivacyVaultError("NONCE_GENERATION_FAILED", "生成随机数失败", err)
	}

	// nonce || ciphertext || tag
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt 使用 AES-256-GCM 解密数据
func (ce *CryptoEngine) Decrypt(key, ciphertext []byte) ([]byte, error) {
	if len(key) != ce.keySize {
		return nil, NewPrivacyVaultError("INVALID_KEY_SIZE", "密钥长度不正确", nil)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, NewPrivacyVaultError("CIPHER_CREATION_FAILED", "创建加密块失败", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, NewPrivacyVaultError("GCM_CREATION_FAILED", "创建 GCM 失败", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, NewPrivacyVaultError("CIPHERTEXT_TOO_SHORT", "密文长度不足", nil)
	}

	nonce, encryptedData := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return nil, NewPrivacyVaultError("DECRYPTION_FAILED", "解密失败", err)
	}

	return plaintext, nil
}

// RotateKey 轮换密钥：使用旧密钥解密，新密钥加密
func (ce *CryptoEngine) RotateKey(oldKey, newKey, ciphertext []byte) ([]byte, error) {
	plaintext, err := ce.Decrypt(oldKey, ciphertext)
	if err != nil {
		return nil, NewPrivacyVaultError("KEY_ROTATION_DECRYPT_FAILED", "密钥轮换解密失败", err)
	}

	newCiphertext, err := ce.Encrypt(newKey, plaintext)
	if err != nil {
		return nil, NewPrivacyVaultError("KEY_ROTATION_ENCRYPT_FAILED", "密钥轮换加密失败", err)
	}

	return newCiphertext, nil
}

// ComputeHash 计算数据的 SHA-256 哈希
func ComputeHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// VerifyHash 验证数据的 SHA-256 哈希
func VerifyHash(data []byte, expectedHash string) bool {
	actualHash := ComputeHash(data)
	return hmac.Equal([]byte(actualHash), []byte(expectedHash))
}

// GenerateToken 生成随机令牌
func GenerateToken(length int) (string, error) {
	if length <= 0 {
		length = 32
	}
	bytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", NewPrivacyVaultError("TOKEN_GENERATION_FAILED", "生成令牌失败", err)
	}
	return hex.EncodeToString(bytes), nil
}

// GenerateVerificationToken 生成密码验证令牌
func GenerateVerificationToken(passphrase string, salt []byte) string {
	key := sha256.Sum256(append(salt, []byte(passphrase)...))
	return hex.EncodeToString(key[:])
}

// VerifyPassphrase 验证密码是否匹配
func VerifyPassphrase(passphrase string, salt []byte, expectedToken string) bool {
	actualToken := GenerateVerificationToken(passphrase, salt)
	return hmac.Equal([]byte(actualToken), []byte(expectedToken))
}

// FormatKeyID 格式化密钥 ID
func FormatKeyID(vaultID string, index int) string {
	return fmt.Sprintf("key-%s-%d", vaultID, index)
}

// KeyRotationInfo 密钥轮换信息
type KeyRotationInfo struct {
	VaultID      string    `json:"vault_id"`
	OldKeyID     string    `json:"old_key_id"`
	NewKeyID     string    `json:"new_key_id"`
	RotatedAt    time.Time `json:"rotated_at"`
	SecretsCount int       `json:"secrets_count"`
}
