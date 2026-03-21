// Package advanced 提供高级备份功能：增量备份、压缩算法选择、AES-256加密、完整性验证
package advanced

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/pbkdf2"
)

// ========== 错误定义 ==========

// ErrInvalidConfig 表示配置无效的错误。
var ErrInvalidConfig = errors.New("invalid backup configuration")

// ErrBackupInProgress 表示备份已在进行中的错误。
var ErrBackupInProgress = errors.New("backup already in progress")

// ErrBackupNotFound 表示备份未找到的错误。
var ErrBackupNotFound = errors.New("backup not found")

// ErrVerificationFailed 表示备份验证失败。
var ErrVerificationFailed = errors.New("backup verification failed")

// ErrDecryptionFailed 表示解密失败。
var ErrDecryptionFailed = errors.New("decryption failed")

// ErrUnsupportedAlgorithm 表示不支持的压缩算法。
var ErrUnsupportedAlgorithm = errors.New("unsupported compression algorithm")

// ErrInvalidKey 表示无效的加密密钥。
var ErrInvalidKey = errors.New("invalid encryption key")

// ========== 压缩算法 ==========

// CompressionAlgorithm 压缩算法类型
type CompressionAlgorithm string

// CompressionNone 表示不压缩。
const CompressionNone CompressionAlgorithm = "none"

// CompressionGzip 表示使用 Gzip 压缩。
const CompressionGzip CompressionAlgorithm = "gzip"

// CompressionZstd 表示使用 Zstd 压缩。
const CompressionZstd CompressionAlgorithm = "zstd"

// CompressionLz4 表示使用 LZ4 压缩。
const CompressionLz4 CompressionAlgorithm = "lz4"

// CompressionBzip2 表示使用 Bzip2 压缩。
const CompressionBzip2 CompressionAlgorithm = "bzip2"

// CompressionXz 表示使用 XZ 压缩。
const CompressionXz CompressionAlgorithm = "xz"

// CompressionConfig 压缩配置
type CompressionConfig struct {
	Algorithm CompressionAlgorithm `json:"algorithm"`
	Level     int                  `json:"level"` // 1-9, 1=fastest, 9=best compression
}

// DefaultCompressionConfig 默认压缩配置
func DefaultCompressionConfig() *CompressionConfig {
	return &CompressionConfig{
		Algorithm: CompressionGzip,
		Level:     6,
	}
}

// ========== 加密配置 ==========

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	Enabled    bool   `json:"enabled"`
	Algorithm  string `json:"algorithm"` // AES-256-GCM, AES-256-CBC
	Key        string `json:"key,omitempty"`
	KeyFile    string `json:"keyFile,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

// DefaultEncryptionConfig 默认加密配置
func DefaultEncryptionConfig() *EncryptionConfig {
	return &EncryptionConfig{
		Enabled:   false,
		Algorithm: "AES-256-GCM",
	}
}

// ========== 备份配置 ==========

// BackupConfig 高级备份配置
type BackupConfig struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	Source           string             `json:"source"`
	Destination      string             `json:"destination"`
	Schedule         string             `json:"schedule"`
	Retention        int                `json:"retention"`
	Enabled          bool               `json:"enabled"`
	Incremental      bool               `json:"incremental"`
	Compression      *CompressionConfig `json:"compression"`
	Encryption       *EncryptionConfig  `json:"encryption"`
	Verification     bool               `json:"verification"`
	ExcludePatterns  []string           `json:"excludePatterns"`
	IncludePatterns  []string           `json:"includePatterns"`
	PreBackupScript  string             `json:"preBackupScript"`
	PostBackupScript string             `json:"postBackupScript"`
	MaxSize          int64              `json:"maxSize"` // 最大备份大小（字节）
	CreatedAt        time.Time          `json:"createdAt"`
	UpdatedAt        time.Time          `json:"updatedAt"`
}

// DefaultBackupConfig 默认备份配置
func DefaultBackupConfig() *BackupConfig {
	return &BackupConfig{
		Retention:    7,
		Enabled:      true,
		Incremental:  true,
		Compression:  DefaultCompressionConfig(),
		Encryption:   DefaultEncryptionConfig(),
		Verification: true,
	}
}

// ========== 备份状态 ==========

// BackupStatus 备份状态
type BackupStatus string

// StatusPending 表示待处理状态。
const StatusPending BackupStatus = "pending"

// StatusRunning 表示运行中状态。
const StatusRunning BackupStatus = "running"

// StatusCompleted 表示已完成状态。
const StatusCompleted BackupStatus = "completed"

// StatusFailed 表示失败状态。
const StatusFailed BackupStatus = "failed"

// StatusCancelled 表示已取消状态。
const StatusCancelled BackupStatus = "cancelled"

// StatusVerifying 表示验证中状态。
const StatusVerifying BackupStatus = "verifying"

// BackupType 备份类型
type BackupType string

// TypeFull 表示完整备份。
const TypeFull BackupType = "full"

// TypeIncremental 表示增量备份。
const TypeIncremental BackupType = "incremental"

// TypeDifferential 表示差异备份。
const TypeDifferential BackupType = "differential"

// ========== 备份记录 ==========

// BackupRecord 备份记录
type BackupRecord struct {
	ID               string            `json:"id"`
	ConfigID         string            `json:"configId"`
	Name             string            `json:"name"`
	Type             BackupType        `json:"type"`
	Status           BackupStatus      `json:"status"`
	Source           string            `json:"source"`
	Destination      string            `json:"destination"`
	Size             int64             `json:"size"`
	CompressedSize   int64             `json:"compressedSize"`
	FileCount        int64             `json:"fileCount"`
	StartTime        time.Time         `json:"startTime"`
	EndTime          time.Time         `json:"endTime"`
	Duration         int64             `json:"duration"` // 秒
	Progress         float64           `json:"progress"`
	Error            string            `json:"error,omitempty"`
	Checksum         string            `json:"checksum"`
	Verified         bool              `json:"verified"`
	VerifiedAt       *time.Time        `json:"verifiedAt,omitempty"`
	BaseBackupID     string            `json:"baseBackupId,omitempty"`
	IncrementalFiles int64             `json:"incrementalFiles"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// BackupManifest 备份清单
type BackupManifest struct {
	ID           string                 `json:"id"`
	ConfigID     string                 `json:"configId"`
	CreatedAt    time.Time              `json:"createdAt"`
	Type         BackupType             `json:"type"`
	Files        []FileManifest         `json:"files"`
	Chunks       []ChunkManifest        `json:"chunks"`
	Checksum     string                 `json:"checksum"`
	Size         int64                  `json:"size"`
	Compressed   int64                  `json:"compressed"`
	Encrypted    bool                   `json:"encrypted"`
	Compression  CompressionAlgorithm   `json:"compression"`
	BaseBackupID string                 `json:"baseBackupId,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// FileManifest 文件清单
type FileManifest struct {
	Path     string      `json:"path"`
	Size     int64       `json:"size"`
	Mode     os.FileMode `json:"mode"`
	ModTime  time.Time   `json:"modTime"`
	Checksum string      `json:"checksum"`
	Chunks   []string    `json:"chunks,omitempty"`
}

// ChunkManifest 块清单
type ChunkManifest struct {
	ID       string `json:"id"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
	Offset   int64  `json:"offset"`
}

// ========== 验证结果 ==========

// VerificationResult 验证结果
type VerificationResult struct {
	BackupID      string             `json:"backupId"`
	Status        VerificationStatus `json:"status"`
	CheckedAt     time.Time          `json:"checkedAt"`
	TotalFiles    int64              `json:"totalFiles"`
	ValidFiles    int64              `json:"validFiles"`
	InvalidFiles  []FileError        `json:"invalidFiles,omitempty"`
	ChecksumMatch bool               `json:"checksumMatch"`
	Duration      time.Duration      `json:"duration"`
	Error         string             `json:"error,omitempty"`
}

// VerificationStatus 验证状态
type VerificationStatus string

// VerificationValid 表示验证通过。
const VerificationValid VerificationStatus = "valid"

// VerificationInvalid 表示验证失败。
const VerificationInvalid VerificationStatus = "invalid"

// VerificationPartial 表示部分验证通过。
const VerificationPartial VerificationStatus = "partial"

// FileError 文件错误
type FileError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
	Type  string `json:"type"` // checksum, missing, corrupted
}

// ========== 增量备份索引 ==========

// IncrementalIndex 增量备份索引
type IncrementalIndex struct {
	mu         sync.RWMutex
	fileStates map[string]*FileState
	baseID     string
	updatedAt  time.Time
}

// FileState 文件状态
type FileState struct {
	Path     string      `json:"path"`
	Checksum string      `json:"checksum"`
	Size     int64       `json:"size"`
	ModTime  time.Time   `json:"modTime"`
	Mode     os.FileMode `json:"mode"`
	Deleted  bool        `json:"deleted"`
}

// NewIncrementalIndex 创建增量索引
func NewIncrementalIndex() *IncrementalIndex {
	return &IncrementalIndex{
		fileStates: make(map[string]*FileState),
	}
}

// Update 更新文件状态
func (idx *IncrementalIndex) Update(path string, state *FileState) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.fileStates[path] = state
	idx.updatedAt = time.Now()
}

// Get 获取文件状态
func (idx *IncrementalIndex) Get(path string) (*FileState, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	state, exists := idx.fileStates[path]
	return state, exists
}

// Delete 标记文件删除
func (idx *IncrementalIndex) Delete(path string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	if state, exists := idx.fileStates[path]; exists {
		state.Deleted = true
	}
	idx.updatedAt = time.Now()
}

// GetAll 获取所有文件状态
func (idx *IncrementalIndex) GetAll() map[string]*FileState {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	result := make(map[string]*FileState, len(idx.fileStates))
	for k, v := range idx.fileStates {
		result[k] = v
	}
	return result
}

// SetBaseID 设置基础备份ID
func (idx *IncrementalIndex) SetBaseID(id string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.baseID = id
}

// GetBaseID 获取基础备份ID
func (idx *IncrementalIndex) GetBaseID() string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.baseID
}

// ========== 备份进度 ==========

// BackupProgress 备份进度
type BackupProgress struct {
	BackupID       string       `json:"backupId"`
	Status         BackupStatus `json:"status"`
	Progress       float64      `json:"progress"`
	CurrentFile    string       `json:"currentFile,omitempty"`
	FilesProcessed int64        `json:"filesProcessed"`
	FilesTotal     int64        `json:"filesTotal"`
	BytesProcessed int64        `json:"bytesProcessed"`
	BytesTotal     int64        `json:"bytesTotal"`
	Speed          float64      `json:"speed"` // MB/s
	ETA            int64        `json:"eta"`   // 秒
	StartTime      time.Time    `json:"startTime"`
	UpdatedAt      time.Time    `json:"updatedAt"`
}

// ========== 加密器 ==========

// Encryptor 加密器接口
type Encryptor interface {
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
	SetKey(key []byte) error
}

// AES256Encryptor AES-256加密器
type AES256Encryptor struct {
	key []byte
	gcm cipher.AEAD
	mu  sync.Mutex
}

// NewAES256Encryptor 创建AES-256加密器
func NewAES256Encryptor(key []byte) (*AES256Encryptor, error) {
	if len(key) != 32 {
		return nil, ErrInvalidKey
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &AES256Encryptor{
		key: key,
		gcm: gcm,
	}, nil
}

// Encrypt 加密数据
func (e *AES256Encryptor) Encrypt(data []byte) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	nonce := make([]byte, e.gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := e.gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// Decrypt 解密数据
func (e *AES256Encryptor) Decrypt(data []byte) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	nonceSize := e.gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, ErrDecryptionFailed
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := e.gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// SetKey 设置密钥
func (e *AES256Encryptor) SetKey(key []byte) error {
	if len(key) != 32 {
		return ErrInvalidKey
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create GCM: %w", err)
	}

	e.mu.Lock()
	e.key = key
	e.gcm = gcm
	e.mu.Unlock()

	return nil
}

// DeriveKey 从密码派生密钥（使用 PBKDF2 进行安全密钥派生）
func DeriveKey(password string, salt []byte) []byte {
	// 使用 PBKDF2，迭代次数 100,000 次，防止暴力破解
	return pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
}

// GenerateKey 生成随机密钥
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// ========== 压缩器 ==========

// Compressor 压缩器接口
type Compressor interface {
	Compress(data []byte) ([]byte, error)
	Decompress(data []byte) ([]byte, error)
	Algorithm() CompressionAlgorithm
}

// ========== 校验和计算 ==========

// CalculateChecksum 计算文件校验和
func CalculateChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// CalculateChecksumBytes 计算字节校验和
func CalculateChecksumBytes(data []byte) string {
	hash := sha256.New()
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}

// ========== 高级备份管理器 ==========

// Manager 高级备份管理器
type Manager struct {
	config      *BackupConfig
	encryptor   Encryptor
	compressor  Compressor
	records     map[string]*BackupRecord
	manifests   map[string]*BackupManifest
	index       *IncrementalIndex
	progress    map[string]*BackupProgress
	activeJobs  map[string]context.CancelFunc
	mu          sync.RWMutex
	storagePath string
	indexPath   string
}

// NewManager 创建高级备份管理器
func NewManager(config *BackupConfig, storagePath string) (*Manager, error) {
	if config == nil {
		config = DefaultBackupConfig()
	}

	indexPath := filepath.Join(storagePath, "index")
	if err := os.MkdirAll(indexPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create index directory: %w", err)
	}

	mgr := &Manager{
		config:      config,
		records:     make(map[string]*BackupRecord),
		manifests:   make(map[string]*BackupManifest),
		index:       NewIncrementalIndex(),
		progress:    make(map[string]*BackupProgress),
		activeJobs:  make(map[string]context.CancelFunc),
		storagePath: storagePath,
		indexPath:   indexPath,
	}

	// 初始化加密器
	if config.Encryption != nil && config.Encryption.Enabled {
		key, err := mgr.getEncryptionKey()
		if err != nil {
			return nil, fmt.Errorf("failed to initialize encryptor: %w", err)
		}
		encryptor, err := NewAES256Encryptor(key)
		if err != nil {
			return nil, fmt.Errorf("failed to create encryptor: %w", err)
		}
		mgr.encryptor = encryptor
	}

	// 初始化压缩器
	mgr.compressor = NewDefaultCompressor(config.Compression)

	return mgr, nil
}

// getEncryptionKey 获取加密密钥
func (m *Manager) getEncryptionKey() ([]byte, error) {
	if m.config.Encryption.Key != "" {
		key := []byte(m.config.Encryption.Key)
		if len(key) < 32 {
			// 填充到32字节
			padded := make([]byte, 32)
			copy(padded, key)
			key = padded
		}
		return key[:32], nil
	}

	if m.config.Encryption.KeyFile != "" {
		data, err := os.ReadFile(m.config.Encryption.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read key file: %w", err)
		}
		if len(data) < 32 {
			return nil, ErrInvalidKey
		}
		return data[:32], nil
	}

	if m.config.Encryption.Passphrase != "" {
		salt := []byte("nas-os-backup-salt")
		return DeriveKey(m.config.Encryption.Passphrase, salt), nil
	}

	return nil, ErrInvalidKey
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *BackupConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(config *BackupConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = config
	m.config.UpdatedAt = time.Now()
	return nil
}
