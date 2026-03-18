package backup

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// CredentialStore 安全凭证存储
type CredentialStore struct {
	keyPath string
	key     []byte
	mu      sync.RWMutex
}

// NewCredentialStore 创建凭证存储
func NewCredentialStore(keyPath string) (*CredentialStore, error) {
	cs := &CredentialStore{
		keyPath: keyPath,
	}

	// 加载或生成密钥
	if err := cs.loadOrGenerateKey(); err != nil {
		return nil, err
	}

	return cs, nil
}

// loadOrGenerateKey 加载或生成加密密钥
func (cs *CredentialStore) loadOrGenerateKey() error {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(cs.keyPath), 0700); err != nil {
		return err
	}

	// 尝试加载现有密钥
	if keyData, err := os.ReadFile(cs.keyPath); err == nil {
		if len(keyData) == 32 {
			cs.key = keyData
			return nil
		}
	}

	// 生成新密钥
	cs.key = make([]byte, 32)
	if _, err := rand.Read(cs.key); err != nil {
		return err
	}

	return os.WriteFile(cs.keyPath, cs.key, 0600)
}

// Encrypt 加密敏感数据
func (cs *CredentialStore) Encrypt(plaintext string) (string, error) {
	if plaintext == "" {
		return "", nil
	}

	cs.mu.RLock()
	defer cs.mu.RUnlock()

	block, err := aes.NewCipher(cs.key)
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

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt 解密敏感数据
func (cs *CredentialStore) Decrypt(ciphertext string) (string, error) {
	if ciphertext == "" {
		return "", nil
	}

	cs.mu.RLock()
	defer cs.mu.RUnlock()

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(cs.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, cipherData := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// SecureJobConfig 安全的作业配置（包含加密凭证）
type SecureJobConfig struct {
	*JobConfig

	// 加密的敏感字段
	EncryptedRemotePassword string `json:"encryptedRemotePassword,omitempty"`
	EncryptedEncryptionKey  string `json:"encryptedEncryptionKey,omitempty"`
	EncryptedCloudSecret    string `json:"encryptedCloudSecret,omitempty"`
}

// EncryptCredentials 加密配置中的敏感信息
func (cs *CredentialStore) EncryptCredentials(config *JobConfig) (*SecureJobConfig, error) {
	secure := &SecureJobConfig{JobConfig: config}

	var err error
	if config.RemotePassword != "" {
		secure.EncryptedRemotePassword, err = cs.Encrypt(config.RemotePassword)
		if err != nil {
			return nil, err
		}
	}

	if config.EncryptionKey != "" {
		secure.EncryptedEncryptionKey, err = cs.Encrypt(config.EncryptionKey)
		if err != nil {
			return nil, err
		}
	}

	if config.CloudConfig != nil && config.CloudConfig.SecretKey != "" {
		secure.EncryptedCloudSecret, err = cs.Encrypt(config.CloudConfig.SecretKey)
		if err != nil {
			return nil, err
		}
	}

	return secure, nil
}

// DecryptCredentials 解密配置中的敏感信息
func (cs *CredentialStore) DecryptCredentials(secure *SecureJobConfig) (*JobConfig, error) {
	config := secure.JobConfig

	var err error
	if secure.EncryptedRemotePassword != "" {
		config.RemotePassword, err = cs.Decrypt(secure.EncryptedRemotePassword)
		if err != nil {
			return nil, err
		}
	}

	if secure.EncryptedEncryptionKey != "" {
		config.EncryptionKey, err = cs.Decrypt(secure.EncryptedEncryptionKey)
		if err != nil {
			return nil, err
		}
	}

	if secure.EncryptedCloudSecret != "" && config.CloudConfig != nil {
		config.CloudConfig.SecretKey, err = cs.Decrypt(secure.EncryptedCloudSecret)
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}

// SecureBackupTarget 安全的备份目标配置
type SecureBackupTarget struct {
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	Path           string            `json:"path"`
	Enabled        bool              `json:"enabled"`
	EncryptedCreds map[string]string `json:"encryptedCreds,omitempty"`
}

// EncryptTargetCredentials 加密目标配置中的凭证
func (cs *CredentialStore) EncryptTargetCredentials(target *BackupTarget) (*SecureBackupTarget, error) {
	secure := &SecureBackupTarget{
		Name:    target.Name,
		Type:    target.Type,
		Path:    target.Path,
		Enabled: target.Enabled,
	}

	if len(target.Credentials) > 0 {
		secure.EncryptedCreds = make(map[string]string)
		for k, v := range target.Credentials {
			encrypted, err := cs.Encrypt(v)
			if err != nil {
				return nil, err
			}
			secure.EncryptedCreds[k] = encrypted
		}
	}

	return secure, nil
}

// DecryptTargetCredentials 解密目标配置中的凭证
func (cs *CredentialStore) DecryptTargetCredentials(secure *SecureBackupTarget) (*BackupTarget, error) {
	target := &BackupTarget{
		Name:    secure.Name,
		Type:    secure.Type,
		Path:    secure.Path,
		Enabled: secure.Enabled,
	}

	if len(secure.EncryptedCreds) > 0 {
		target.Credentials = make(map[string]string)
		for k, v := range secure.EncryptedCreds {
			decrypted, err := cs.Decrypt(v)
			if err != nil {
				return nil, err
			}
			target.Credentials[k] = decrypted
		}
	}

	return target, nil
}

// SaveSecureConfig 安全保存配置到文件
func (cs *CredentialStore) SaveSecureConfig(config *JobConfig, path string) error {
	secure, err := cs.EncryptCredentials(config)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(secure, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// LoadSecureConfig 安全加载配置从文件
func (cs *CredentialStore) LoadSecureConfig(path string) (*JobConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var secure SecureJobConfig
	if err := json.Unmarshal(data, &secure); err != nil {
		return nil, err
	}

	return cs.DecryptCredentials(&secure)
}
