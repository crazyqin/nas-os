// Package presto 高速文件传输模块
// 对标群晖 Presto File Server，基于 QUIC 协议实现高速、加密、可断点续传的文件传输
package presto

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// 传输状态常量
const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusPaused    = "paused"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
	StatusCancelled = "cancelled"
)

// 传输模式
const (
	ModeSend = "send"
	ModeRecv = "recv"
)

// 错误定义
var (
	ErrTransferNotFound  = errors.New("传输任务不存在")
	ErrTransferCancelled = errors.New("传输任务已取消")
	ErrTransferFailed    = errors.New("传输任务失败")
	ErrInvalidChunk      = errors.New("无效的数据块")
	ErrChecksumMismatch  = errors.New("校验和不匹配")
	ErrMaxConcurrent     = errors.New("已达最大并发传输数")
	ErrFileNotFound      = errors.New("文件不存在")
	ErrPermissionDenied  = errors.New("权限不足")
	ErrEncryptionFailed  = errors.New("加密失败")
	ErrDecryptionFailed  = errors.New("解密失败")
)

// Config Presto 配置
type Config struct {
	// 服务端监听地址
	ListenAddr string `json:"listen_addr" yaml:"listen_addr"`
	// 最大并发传输数
	MaxConcurrent int `json:"max_concurrent" yaml:"max_concurrent"`
	// 数据块大小（字节）
	ChunkSize int `json:"chunk_size" yaml:"chunk_size"`
	// 是否启用压缩
	EnableCompression bool `json:"enable_compression" yaml:"enable_compression"`
	// 压缩级别 1-9
	CompressionLevel int `json:"compression_level" yaml:"compression_level"`
	// 是否启用加密
	EnableEncryption bool `json:"enable_encryption" yaml:"enable_encryption"`
	// 加密密钥（32字节 AES-256）
	EncryptionKey []byte `json:"-" yaml:"-"`
	// 传输超时时间
	TransferTimeout time.Duration `json:"transfer_timeout" yaml:"transfer_timeout"`
	// 速度限制（字节/秒），0 表示不限速
	SpeedLimit int64 `json:"speed_limit" yaml:"speed_limit"`
	// 存储根目录
	StorageRoot string `json:"storage_root" yaml:"storage_root"`
	// 临时文件目录
	TempDir string `json:"temp_dir" yaml:"temp_dir"`
	// TLS 证书文件
	TLSCertFile string `json:"tls_cert_file" yaml:"tls_cert_file"`
	// TLS 密钥文件
	TLSKeyFile string `json:"tls_key_file" yaml:"tls_key_file"`
	// 是否启用 mTLS（双向认证）
	EnableMTLS bool `json:"enable_mtls" yaml:"enable_mtls"`
	// 客户端 CA 证书
	ClientCAFile string `json:"client_ca_file" yaml:"client_ca_file"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		ListenAddr:        ":9443",
		MaxConcurrent:     8,
		ChunkSize:         4 * 1024 * 1024, // 4MB
		EnableCompression: true,
		CompressionLevel:  6,
		EnableEncryption:  true,
		TransferTimeout:   30 * time.Minute,
		StorageRoot:       "/mnt/presto",
		TempDir:           "/tmp/presto",
	}
}

// Transfer 传输任务
type Transfer struct {
	mu sync.RWMutex `json:"-"`

	ID           string        `json:"id"`
	Name         string        `json:"name"`
	SourcePath   string        `json:"source_path"`
	DestPath     string        `json:"dest_path"`
	Mode         string        `json:"mode"` // send/recv
	Status       string        `json:"status"`
	TotalBytes   int64         `json:"total_bytes"`
	Transferred  int64         `json:"transferred"`
	ChunkCount   int           `json:"chunk_count"`
	ChunksDone   int           `json:"chunks_done"`
	SpeedBps     float64       `json:"speed_bps"`
	Compressed   bool          `json:"compressed"`
	Encrypted    bool          `json:"encrypted"`
	FileChecksum string        `json:"file_checksum"`
	ErrorMsg     string        `json:"error_msg,omitempty"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	Elapsed      time.Duration `json:"elapsed"`
	ClientAddr   string        `json:"client_addr,omitempty"`
	RemotePath   string        `json:"remote_path,omitempty"`

	// 内部状态
	ctx       context.Context    `json:"-"`
	cancel    context.CancelFunc `json:"-"`
	chunks    []ChunkState       `json:"-"`
	checksums map[int]string     `json:"-"`
}

// ChunkState 数据块状态
type ChunkState struct {
	Index       int    `json:"index"`
	Offset      int64  `json:"offset"`
	Size        int64  `json:"size"`
	Status      string `json:"status"` // pending/sending/done/failed
	Checksum    string `json:"checksum"`
	Transferred int64  `json:"transferred"`
}

// TransferInfo 传输信息（用于 API 响应）
type TransferInfo struct {
	ID           string        `json:"id"`
	Name         string        `json:"name"`
	SourcePath   string        `json:"source_path"`
	DestPath     string        `json:"dest_path"`
	Mode         string        `json:"mode"`
	Status       string        `json:"status"`
	TotalBytes   int64         `json:"total_bytes"`
	Transferred  int64         `json:"transferred"`
	Progress     float64       `json:"progress"`
	ChunkCount   int           `json:"chunk_count"`
	ChunksDone   int           `json:"chunks_done"`
	SpeedBps     float64       `json:"speed_bps"`
	SpeedHuman   string        `json:"speed_human"`
	Compressed   bool          `json:"compressed"`
	Encrypted    bool          `json:"encrypted"`
	FileChecksum string        `json:"file_checksum"`
	ErrorMsg     string        `json:"error_msg,omitempty"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  *time.Time    `json:"completed_at,omitempty"`
	Elapsed      time.Duration `json:"elapsed"`
	ElapsedHuman string        `json:"elapsed_human"`
	ClientAddr   string        `json:"client_addr,omitempty"`
	ETA          string        `json:"eta,omitempty"`
}

// Stats 传输统计
type Stats struct {
	TotalTransfers   int64   `json:"total_transfers"`
	ActiveTransfers  int     `json:"active_transfers"`
	CompletedCount   int     `json:"completed_count"`
	FailedCount      int     `json:"failed_count"`
	TotalBytes       int64   `json:"total_bytes"`
	TotalTransferred int64   `json:"total_transferred"`
	AvgSpeedBps      float64 `json:"avg_speed_bps"`
	AvgSpeedHuman    string  `json:"avg_speed_human"`
}

// Manager Presto 传输管理器
type Manager struct {
	mu        sync.RWMutex
	config    *Config
	transfers map[string]*Transfer
	logger    *zap.Logger
	running   int32

	// 统计
	totalTransfers   int64
	completedCount   int64
	failedCount      int64
	totalBytes       int64
	totalTransferred int64
}

// NewManager 创建传输管理器
func NewManager(cfg *Config, logger *zap.Logger) *Manager {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	// 确保目录存在
	os.MkdirAll(cfg.StorageRoot, 0755)
	os.MkdirAll(cfg.TempDir, 0755)

	return &Manager{
		config:    cfg,
		transfers: make(map[string]*Transfer),
		logger:    logger,
	}
}

// CreateTransfer 创建新的传输任务
func (m *Manager) CreateTransfer(name, sourcePath, destPath, mode string) (*Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查并发数
	active := 0
	for _, t := range m.transfers {
		if t.Status == StatusRunning || t.Status == StatusPending {
			active++
		}
	}
	if active >= m.config.MaxConcurrent {
		return nil, ErrMaxConcurrent
	}

	// 验证源文件（发送模式）
	if mode == ModeSend {
		info, err := os.Stat(sourcePath)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrFileNotFound, err)
		}
		if info.IsDir() {
			return nil, fmt.Errorf("不支持目录传输，请使用打包功能")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), m.config.TransferTimeout)

	t := &Transfer{
		ID:         uuid.New().String(),
		Name:       name,
		SourcePath: sourcePath,
		DestPath:   destPath,
		Mode:       mode,
		Status:     StatusPending,
		Compressed: m.config.EnableCompression,
		Encrypted:  m.config.EnableEncryption,
		StartedAt:  time.Now(),
		ctx:        ctx,
		cancel:     cancel,
		chunks:     make([]ChunkState, 0),
		checksums:  make(map[int]string),
	}

	m.transfers[t.ID] = t
	atomic.AddInt64(&m.totalTransfers, 1)

	m.logger.Info("创建传输任务",
		zap.String("id", t.ID),
		zap.String("name", name),
		zap.String("source", sourcePath),
		zap.String("dest", destPath),
		zap.String("mode", mode),
	)

	return t, nil
}

// GetTransfer 获取传输任务
func (m *Manager) GetTransfer(id string) (*Transfer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.transfers[id]
	if !ok {
		return nil, ErrTransferNotFound
	}
	return t, nil
}

// ListTransfers 列出所有传输任务
func (m *Manager) ListTransfers() []*Transfer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Transfer, 0, len(m.transfers))
	for _, t := range m.transfers {
		result = append(result, t)
	}
	return result
}

// CancelTransfer 取消传输任务
func (m *Manager) CancelTransfer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.transfers[id]
	if !ok {
		return ErrTransferNotFound
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Status != StatusRunning && t.Status != StatusPending && t.Status != StatusPaused {
		return fmt.Errorf("无法取消状态为 %s 的传输任务", t.Status)
	}

	t.cancel()
	t.Status = StatusCancelled
	now := time.Now()
	t.CompletedAt = &now
	t.Elapsed = now.Sub(t.StartedAt)

	m.logger.Info("取消传输任务", zap.String("id", id))
	return nil
}

// PauseTransfer 暂停传输任务
func (m *Manager) PauseTransfer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.transfers[id]
	if !ok {
		return ErrTransferNotFound
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Status != StatusRunning {
		return fmt.Errorf("只能暂停运行中的传输任务")
	}

	t.Status = StatusPaused
	m.logger.Info("暂停传输任务", zap.String("id", id))
	return nil
}

// ResumeTransfer 恢复传输任务
func (m *Manager) ResumeTransfer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.transfers[id]
	if !ok {
		return ErrTransferNotFound
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Status != StatusPaused {
		return fmt.Errorf("只能恢复暂停的传输任务")
	}

	t.Status = StatusRunning
	m.logger.Info("恢复传输任务", zap.String("id", id))
	return nil
}

// GetStats 获取传输统计
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		TotalTransfers: atomic.LoadInt64(&m.totalTransfers),
	}

	var totalTransferred int64
	for _, t := range m.transfers {
		switch t.Status {
		case StatusRunning, StatusPending:
			stats.ActiveTransfers++
		case StatusCompleted:
			stats.CompletedCount++
		case StatusFailed:
			stats.FailedCount++
		}
		totalTransferred += t.Transferred
	}

	stats.TotalTransferred = totalTransferred

	if stats.TotalTransferred > 0 {
		elapsed := time.Since(m.getEarliestStart())
		if elapsed > 0 {
			stats.AvgSpeedBps = float64(stats.TotalTransferred) / elapsed.Seconds()
			stats.AvgSpeedHuman = formatSpeed(stats.AvgSpeedBps)
		}
	}

	return stats
}

func (m *Manager) getEarliestStart() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	earliest := time.Now()
	for _, t := range m.transfers {
		if t.StartedAt.Before(earliest) {
			earliest = t.StartedAt
		}
	}
	return earliest
}

// Cleanup 清理已完成的任务
func (m *Manager) Cleanup(olderThan time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	cutoff := time.Now().Add(-olderThan)
	for id, t := range m.transfers {
		if (t.Status == StatusCompleted || t.Status == StatusFailed || t.Status == StatusCancelled) &&
			t.CompletedAt != nil && t.CompletedAt.Before(cutoff) {
			delete(m.transfers, id)
			count++
		}
	}

	if count > 0 {
		m.logger.Info("清理已完成的传输任务", zap.Int("count", count))
	}
	return count
}

// GetTransferInfo 获取传输任务信息
func (t *Transfer) GetTransferInfo() *TransferInfo {
	t.mu.RLock()
	defer t.mu.RUnlock()

	info := &TransferInfo{
		ID:           t.ID,
		Name:         t.Name,
		SourcePath:   t.SourcePath,
		DestPath:     t.DestPath,
		Mode:         t.Mode,
		Status:       t.Status,
		TotalBytes:   t.TotalBytes,
		Transferred:  t.Transferred,
		ChunkCount:   t.ChunkCount,
		ChunksDone:   t.ChunksDone,
		SpeedBps:     t.SpeedBps,
		SpeedHuman:   formatSpeed(t.SpeedBps),
		Compressed:   t.Compressed,
		Encrypted:    t.Encrypted,
		FileChecksum: t.FileChecksum,
		ErrorMsg:     t.ErrorMsg,
		StartedAt:    t.StartedAt,
		CompletedAt:  t.CompletedAt,
		Elapsed:      t.Elapsed,
		ElapsedHuman: formatDuration(t.Elapsed),
		ClientAddr:   t.ClientAddr,
	}

	if t.TotalBytes > 0 {
		info.Progress = float64(t.Transferred) / float64(t.TotalBytes) * 100
	}

	// 计算 ETA
	if t.SpeedBps > 0 && t.TotalBytes > t.Transferred {
		remaining := float64(t.TotalBytes - t.Transferred)
		eta := time.Duration(remaining/t.SpeedBps) * time.Second
		info.ETA = formatDuration(eta)
	}

	return info
}

// 加密相关函数

// Encrypt 使用 AES-256-GCM 加密数据
func Encrypt(data []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("密钥长度必须为 32 字节")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	// nonce + ciphertext + tag
	ciphertext := aesGCM.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// Decrypt 使用 AES-256-GCM 解密数据
func Decrypt(data []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("密钥长度必须为 32 字节")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("%w: 数据太短", ErrDecryptionFailed)
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return plaintext, nil
}

// ComputeChecksum 计算 SHA-256 校验和
func ComputeChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

// ComputeFileChecksum 计算文件的 SHA-256 校验和
func ComputeFileChecksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// 分块相关函数

// CalculateChunks 计算文件分块信息
func CalculateChunks(fileSize int64, chunkSize int) []ChunkState {
	if chunkSize <= 0 {
		chunkSize = 4 * 1024 * 1024 // 默认 4MB
	}

	chunks := make([]ChunkState, 0)
	offset := int64(0)
	index := 0

	for offset < fileSize {
		size := int64(chunkSize)
		if offset+size > fileSize {
			size = fileSize - offset
		}

		chunks = append(chunks, ChunkState{
			Index:  index,
			Offset: offset,
			Size:   size,
			Status: "pending",
		})

		offset += size
		index++
	}

	return chunks
}

// 协议消息定义

// Message 类型常量
const (
	MsgTypeHandshake  = "handshake"
	MsgTypeFileMeta   = "file_meta"
	MsgTypeChunkReq   = "chunk_req"
	MsgTypeChunkData  = "chunk_data"
	MsgTypeChunkAck   = "chunk_ack"
	MsgTypeComplete   = "complete"
	MsgTypeError      = "error"
	MsgTypeResumeReq  = "resume_req"
	MsgTypeResumeResp = "resume_resp"
	MsgTypeHeartbeat  = "heartbeat"
)

// Message 协议消息
type Message struct {
	Type      string          `json:"type"`
	ID        string          `json:"id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
	Checksum  string          `json:"checksum,omitempty"`
}

// HandshakePayload 握手载荷
type HandshakePayload struct {
	Version    string `json:"version"`
	ClientID   string `json:"client_id"`
	AuthToken  string `json:"auth_token,omitempty"`
	Compress   bool   `json:"compress"`
	Encrypt    bool   `json:"encrypt"`
	SpeedLimit int64  `json:"speed_limit,omitempty"`
}

// FileMetaPayload 文件元数据载荷
type FileMetaPayload struct {
	TransferID  string `json:"transfer_id"`
	FileName    string `json:"file_name"`
	FilePath    string `json:"file_path"`
	FileSize    int64  `json:"file_size"`
	ChunkSize   int    `json:"chunk_size"`
	ChunkCount  int    `json:"chunk_count"`
	Checksum    string `json:"checksum"`
	ModTime     int64  `json:"mod_time"`
	Permissions uint32 `json:"permissions"`
}

// ChunkRequestPayload 数据块请求载荷
type ChunkRequestPayload struct {
	TransferID string `json:"transfer_id"`
	ChunkIndex int    `json:"chunk_index"`
	Offset     int64  `json:"offset"`
	Size       int64  `json:"size"`
}

// ChunkDataPayload 数据块数据载荷
type ChunkDataPayload struct {
	TransferID string `json:"transfer_id"`
	ChunkIndex int    `json:"chunk_index"`
	Offset     int64  `json:"offset"`
	Size       int64  `json:"size"`
	Data       []byte `json:"data"`
	Checksum   string `json:"checksum"`
	Compressed bool   `json:"compressed"`
	Encrypted  bool   `json:"encrypted"`
}

// ChunkAckPayload 数据块确认载荷
type ChunkAckPayload struct {
	TransferID string `json:"transfer_id"`
	ChunkIndex int    `json:"chunk_index"`
	Status     string `json:"status"` // ok/error
	Checksum   string `json:"checksum,omitempty"`
	Error      string `json:"error,omitempty"`
}

// ResumeRequestPayload 断点续传请求载荷
type ResumeRequestPayload struct {
	TransferID string `json:"transfer_id"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	Checksum   string `json:"checksum"`
}

// ResumeResponsePayload 断点续传响应载荷
type ResumeResponsePayload struct {
	TransferID  string       `json:"transfer_id"`
	CanResume   bool         `json:"can_resume"`
	DoneChunks  []int        `json:"done_chunks,omitempty"`
	ChunkStates []ChunkState `json:"chunk_states,omitempty"`
}

// ErrorPayload 错误载荷
type ErrorPayload struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// CompletePayload 完成载荷
type CompletePayload struct {
	TransferID string  `json:"transfer_id"`
	Checksum   string  `json:"checksum"`
	Size       int64   `json:"size"`
	Elapsed    int64   `json:"elapsed_ms"`
	SpeedBps   float64 `json:"speed_bps"`
}

// NewMessage 创建新消息
func NewMessage(msgType string, payload interface{}) (*Message, error) {
	var payloadBytes json.RawMessage
	if payload != nil {
		var err error
		payloadBytes, err = json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("序列化载荷失败: %w", err)
		}
	}

	return &Message{
		Type:      msgType,
		ID:        uuid.New().String(),
		Payload:   payloadBytes,
		Timestamp: time.Now(),
	}, nil
}

// EncodeMessage 编码消息为字节（长度前缀 + JSON）
func EncodeMessage(msg *Message) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	// 4字节长度前缀
	buf := make([]byte, 4+len(data))
	binary.BigEndian.PutUint32(buf[:4], uint32(len(data)))
	copy(buf[4:], data)

	return buf, nil
}

// DecodeMessage 从字节解码消息
func DecodeMessage(r io.Reader) (*Message, error) {
	// 读取长度前缀
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lenBuf); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(lenBuf)
	if length > 100*1024*1024 { // 100MB 限制
		return nil, fmt.Errorf("消息太大: %d bytes", length)
	}

	// 读取消息体
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}

	return &msg, nil
}

// 工具函数

func formatSpeed(bps float64) string {
	if bps <= 0 {
		return "0 B/s"
	}

	units := []string{"B/s", "KB/s", "MB/s", "GB/s", "TB/s"}
	unit := 0
	speed := bps

	for speed >= 1024 && unit < len(units)-1 {
		speed /= 1024
		unit++
	}

	return fmt.Sprintf("%.2f %s", speed, units[unit])
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "0s"
	}

	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm%ds", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}

// GenerateEncryptionKey 生成随机加密密钥
func GenerateEncryptionKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}

// SaveTransferState 保存传输状态到文件（用于断点续传）
func (t *Transfer) SaveTransferState(dir string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	state := struct {
		ID           string       `json:"id"`
		Name         string       `json:"name"`
		SourcePath   string       `json:"source_path"`
		DestPath     string       `json:"dest_path"`
		Mode         string       `json:"mode"`
		TotalBytes   int64        `json:"total_bytes"`
		Transferred  int64        `json:"transferred"`
		ChunkCount   int          `json:"chunk_count"`
		ChunksDone   int          `json:"chunks_done"`
		FileChecksum string       `json:"file_checksum"`
		Chunks       []ChunkState `json:"chunks"`
		SavedAt      time.Time    `json:"saved_at"`
	}{
		ID:           t.ID,
		Name:         t.Name,
		SourcePath:   t.SourcePath,
		DestPath:     t.DestPath,
		Mode:         t.Mode,
		TotalBytes:   t.TotalBytes,
		Transferred:  t.Transferred,
		ChunkCount:   t.ChunkCount,
		ChunksDone:   t.ChunksDone,
		FileChecksum: t.FileChecksum,
		Chunks:       t.chunks,
		SavedAt:      time.Now(),
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	stateFile := filepath.Join(dir, t.ID+".state")
	return os.WriteFile(stateFile, data, 0644)
}

// LoadTransferState 从文件加载传输状态
func LoadTransferState(dir, id string) (*Transfer, error) {
	stateFile := filepath.Join(dir, id+".state")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		return nil, err
	}

	var state struct {
		ID           string       `json:"id"`
		Name         string       `json:"name"`
		SourcePath   string       `json:"source_path"`
		DestPath     string       `json:"dest_path"`
		Mode         string       `json:"mode"`
		TotalBytes   int64        `json:"total_bytes"`
		Transferred  int64        `json:"transferred"`
		ChunkCount   int          `json:"chunk_count"`
		ChunksDone   int          `json:"chunks_done"`
		FileChecksum string       `json:"file_checksum"`
		Chunks       []ChunkState `json:"chunks"`
	}

	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)

	t := &Transfer{
		ID:           state.ID,
		Name:         state.Name,
		SourcePath:   state.SourcePath,
		DestPath:     state.DestPath,
		Mode:         state.Mode,
		Status:       StatusPaused,
		TotalBytes:   state.TotalBytes,
		Transferred:  state.Transferred,
		ChunkCount:   state.ChunkCount,
		ChunksDone:   state.ChunksDone,
		FileChecksum: state.FileChecksum,
		StartedAt:    time.Now(),
		ctx:          ctx,
		cancel:       cancel,
		chunks:       state.Chunks,
		checksums:    make(map[int]string),
	}

	return t, nil
}
