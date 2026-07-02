package presto

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"go.uber.org/zap"
)

// Client Presto 客户端.
type Client struct {
	config     *Config
	logger     *zap.Logger
	conn       *quic.Conn
	stream     *quic.Stream
	mu         sync.RWMutex
	connected  bool
	clientID   string
	serverAddr string
}

// NewClient 创建客户端.
func NewClient(cfg *Config, logger *zap.Logger) *Client {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Client{
		config: cfg,
		logger: logger,
	}
}

// Connect 连接到服务端.
func (c *Client) Connect(ctx context.Context, serverAddr string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("已经连接到服务端")
	}

	// 配置 TLS
	tlsConfig, err := c.buildTLSConfig()
	if err != nil {
		return fmt.Errorf("配置 TLS 失败: %w", err)
	}

	// 配置 QUIC
	quicConfig := &quic.Config{
		MaxIdleTimeout:  5 * time.Minute,
		KeepAlivePeriod: 30 * time.Second,
		EnableDatagrams: true,
	}

	// 连接到服务端
	conn, err := quic.DialAddr(ctx, serverAddr, tlsConfig, quicConfig)
	if err != nil {
		return fmt.Errorf("连接服务端失败: %w", err)
	}

	c.conn = conn
	c.serverAddr = serverAddr
	c.connected = true

	// 执行握手
	if err := c.handshake(); err != nil {
		conn.CloseWithError(0, "握手失败")
		c.connected = false
		return fmt.Errorf("握手失败: %w", err)
	}

	c.logger.Info("已连接到 Presto 服务端", zap.String("addr", serverAddr))
	return nil
}

// Disconnect 断开连接.
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected {
		return nil
	}

	if c.stream != nil {
		c.stream.Close()
	}

	if c.conn != nil {
		c.conn.CloseWithError(0, "客户端断开")
	}

	c.connected = false
	c.logger.Info("已断开 Presto 服务端连接")
	return nil
}

func (c *Client) buildTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		NextProtos: []string{"presto/1.0"},
		ServerName: "localhost",
	}

	// 如果配置了 CA 证书，验证服务端证书
	if c.config.ClientCAFile != "" {
		caCert, err := os.ReadFile(c.config.ClientCAFile)
		if err != nil {
			return nil, fmt.Errorf("读取 CA 证书失败: %w", err)
		}
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("解析 CA 证书失败")
		}
		tlsConfig.RootCAs = caCertPool
	} else {
		// 开发模式：跳过证书验证
		tlsConfig.InsecureSkipVerify = true
	}

	// mTLS 客户端证书
	if c.config.TLSCertFile != "" && c.config.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(c.config.TLSCertFile, c.config.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("加载客户端证书失败: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	return tlsConfig, nil
}

func (c *Client) handshake() error {
	// 创建流
	stream, err := c.conn.OpenStreamSync(context.Background())
	if err != nil {
		return fmt.Errorf("创建流失败: %w", err)
	}

	c.stream = stream

	// 发送握手
	payload := HandshakePayload{
		Version:    "1.0",
		ClientID:   c.clientID,
		Compress:   c.config.EnableCompression,
		Encrypt:    c.config.EnableEncryption,
		SpeedLimit: c.config.SpeedLimit,
	}

	if err := c.sendMessage(MsgTypeHandshake, payload); err != nil {
		return fmt.Errorf("发送握手消息失败: %w", err)
	}

	// 接收握手响应
	msg, err := c.receiveMessage()
	if err != nil {
		return fmt.Errorf("接收握手响应失败: %w", err)
	}

	if msg.Type == MsgTypeError {
		var errPayload ErrorPayload
		json.Unmarshal(msg.Payload, &errPayload)
		return fmt.Errorf("握手失败: %s", errPayload.Message)
	}

	if msg.Type != MsgTypeHandshake {
		return fmt.Errorf("意外的消息类型: %s", msg.Type)
	}

	return nil
}

// SendFile 发送文件.
func (c *Client) SendFile(ctx context.Context, filePath string, destPath string) (*Transfer, error) {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return nil, fmt.Errorf("未连接到服务端")
	}
	c.mu.RUnlock()

	// 为文件传输创建新流
	stream, err := c.conn.OpenStreamSync(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建传输流失败: %w", err)
	}
	c.mu.Lock()
	c.stream = stream
	c.mu.Unlock()

	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 计算文件校验和
	checksum, err := ComputeFileChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("计算文件校验和失败: %w", err)
	}

	// 计算分块
	chunkSize := c.config.ChunkSize
	chunks := CalculateChunks(fileInfo.Size(), chunkSize)

	// 发送文件元数据
	fileName := filepath.Base(filePath)
	metaPayload := FileMetaPayload{
		FileName:    fileName,
		FilePath:    destPath,
		FileSize:    fileInfo.Size(),
		ChunkSize:   chunkSize,
		ChunkCount:  len(chunks),
		Checksum:    checksum,
		ModTime:     fileInfo.ModTime().Unix(),
		Permissions: uint32(fileInfo.Mode()),
	}

	if err := c.sendMessage(MsgTypeFileMeta, metaPayload); err != nil {
		return nil, fmt.Errorf("发送文件元数据失败: %w", err)
	}

	// 接收确认
	msg, err := c.receiveMessage()
	if err != nil {
		return nil, fmt.Errorf("接收确认失败: %w", err)
	}

	if msg.Type == MsgTypeError {
		var errPayload ErrorPayload
		json.Unmarshal(msg.Payload, &errPayload)
		return nil, fmt.Errorf("服务端拒绝: %s", errPayload.Message)
	}

	var ackPayload map[string]interface{}
	json.Unmarshal(msg.Payload, &ackPayload)

	transferID, _ := ackPayload["transfer_id"].(string)

	ctx, cancel := context.WithTimeout(ctx, c.config.TransferTimeout)

	// 创建本地传输记录
	transfer := &Transfer{
		ID:           transferID,
		Name:         fileName,
		SourcePath:   filePath,
		DestPath:     destPath,
		Mode:         ModeSend,
		Status:       StatusRunning,
		TotalBytes:   fileInfo.Size(),
		ChunkCount:   len(chunks),
		Compressed:   c.config.EnableCompression,
		Encrypted:    c.config.EnableEncryption,
		FileChecksum: checksum,
		StartedAt:    time.Now(),
		ctx:          ctx,
		cancel:       cancel,
		chunks:       chunks,
		checksums:    make(map[int]string),
	}

	// 启动发送协程
	go c.sendChunks(ctx, filePath, transfer)

	return transfer, nil
}

func (c *Client) sendChunks(ctx context.Context, filePath string, transfer *Transfer) {
	file, err := os.Open(filePath)
	if err != nil {
		transfer.mu.Lock()
		transfer.Status = StatusFailed
		transfer.ErrorMsg = "打开文件失败: " + err.Error()
		now := time.Now()
		transfer.CompletedAt = &now
		transfer.Elapsed = now.Sub(transfer.StartedAt)
		transfer.mu.Unlock()
		return
	}
	defer file.Close()

	startTime := time.Now()

	for i, chunk := range transfer.chunks {
		select {
		case <-ctx.Done():
			transfer.mu.Lock()
			transfer.Status = StatusCancelled
			transfer.ErrorMsg = "传输已取消"
			now := time.Now()
			transfer.CompletedAt = &now
			transfer.Elapsed = now.Sub(transfer.StartedAt)
			transfer.mu.Unlock()
			return
		case <-transfer.ctx.Done():
			return
		default:
		}

		// 读取数据块
		chunkData := make([]byte, chunk.Size)
		_, err := file.ReadAt(chunkData, chunk.Offset)
		if err != nil && err != io.EOF {
			c.logger.Error("读取数据块失败", zap.Int("chunk", i), zap.Error(err))
			continue
		}

		// 计算校验和
		checksum := ComputeChecksum(chunkData)

		// 压缩
		compressed := false
		if c.config.EnableCompression {
			compressedData, err := compressData(chunkData, c.config.CompressionLevel)
			if err == nil && len(compressedData) < len(chunkData) {
				chunkData = compressedData
				compressed = true
			}
		}

		// 加密
		encrypted := false
		if c.config.EnableEncryption && len(c.config.EncryptionKey) > 0 {
			encryptedData, err := Encrypt(chunkData, c.config.EncryptionKey)
			if err != nil {
				c.logger.Error("加密数据块失败", zap.Int("chunk", i), zap.Error(err))
				continue
			}
			chunkData = encryptedData
			encrypted = true
		}

		// 发送数据块
		payload := ChunkDataPayload{
			TransferID: transfer.ID,
			ChunkIndex: i,
			Offset:     chunk.Offset,
			Size:       chunk.Size,
			Data:       chunkData,
			Checksum:   checksum,
			Compressed: compressed,
			Encrypted:  encrypted,
		}

		if err := c.sendMessage(MsgTypeChunkData, payload); err != nil {
			c.logger.Error("发送数据块失败", zap.Int("chunk", i), zap.Error(err))
			continue
		}

		// 等待 ACK
		ackMsg, err := c.receiveMessage()
		if err != nil {
			c.logger.Error("接收 ACK 失败", zap.Int("chunk", i), zap.Error(err))
			continue
		}

		if ackMsg.Type == MsgTypeChunkAck {
			var ackPayload ChunkAckPayload
			json.Unmarshal(ackMsg.Payload, &ackPayload)

			if ackPayload.Status == "ok" {
				transfer.mu.Lock()
				transfer.chunks[i].Status = "done"
				transfer.chunks[i].Checksum = ackPayload.Checksum
				transfer.Transferred += chunk.Size
				transfer.ChunksDone++

				// 计算速度
				elapsed := time.Since(startTime).Seconds()
				if elapsed > 0 {
					transfer.SpeedBps = float64(transfer.Transferred) / elapsed
				}
				transfer.mu.Unlock()
			} else {
				c.logger.Error("数据块传输失败",
					zap.Int("chunk", i),
					zap.String("error", ackPayload.Error),
				)
			}
		}

		// 速度限制
		if c.config.SpeedLimit > 0 {
			expectedTime := time.Duration(float64(chunk.Size) / float64(c.config.SpeedLimit) * float64(time.Second))
			actualTime := time.Since(startTime)
			if actualTime < expectedTime {
				time.Sleep(expectedTime - actualTime)
			}
		}
	}

	// 发送完成消息
	completePayload := CompletePayload{
		TransferID: transfer.ID,
		Checksum:   transfer.FileChecksum,
		Size:       transfer.TotalBytes,
		Elapsed:    time.Since(startTime).Milliseconds(),
		SpeedBps:   transfer.SpeedBps,
	}

	c.sendMessage(MsgTypeComplete, completePayload)

	// 更新状态
	transfer.mu.Lock()
	transfer.Status = StatusCompleted
	now := time.Now()
	transfer.CompletedAt = &now
	transfer.Elapsed = now.Sub(transfer.StartedAt)
	transfer.mu.Unlock()

	c.logger.Info("文件发送完成",
		zap.String("file", filePath),
		zap.Duration("elapsed", transfer.Elapsed),
		zap.String("speed", formatSpeed(transfer.SpeedBps)),
	)
}

// RequestResume 请求断点续传.
func (c *Client) RequestResume(ctx context.Context, transferID string, fileName string, fileSize int64, checksum string) (*ResumeResponsePayload, error) {
	c.mu.RLock()
	if !c.connected {
		c.mu.RUnlock()
		return nil, fmt.Errorf("未连接到服务端")
	}
	c.mu.RUnlock()

	// 发送续传请求
	payload := ResumeRequestPayload{
		TransferID: transferID,
		FileName:   fileName,
		FileSize:   fileSize,
		Checksum:   checksum,
	}

	if err := c.sendMessage(MsgTypeResumeReq, payload); err != nil {
		return nil, fmt.Errorf("发送续传请求失败: %w", err)
	}

	// 接收响应
	msg, err := c.receiveMessage()
	if err != nil {
		return nil, fmt.Errorf("接收续传响应失败: %w", err)
	}

	if msg.Type == MsgTypeError {
		var errPayload ErrorPayload
		json.Unmarshal(msg.Payload, &errPayload)
		return nil, fmt.Errorf("续传请求失败: %s", errPayload.Message)
	}

	var respPayload ResumeResponsePayload
	if err := json.Unmarshal(msg.Payload, &respPayload); err != nil {
		return nil, fmt.Errorf("解析续传响应失败: %w", err)
	}

	return &respPayload, nil
}

// SendFileWithResume 发送文件（支持断点续传）.
func (c *Client) SendFileWithResume(ctx context.Context, filePath string, destPath string, stateDir string) (*Transfer, error) {
	// 获取文件信息
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 计算文件校验和
	checksum, err := ComputeFileChecksum(filePath)
	if err != nil {
		return nil, fmt.Errorf("计算文件校验和失败: %w", err)
	}

	// 尝试加载本地状态
	fileName := filepath.Base(filePath)
	var transferID string
	var doneChunks map[int]bool

	// 查找匹配的传输状态
	entries, _ := os.ReadDir(stateDir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		state, err := LoadTransferState(stateDir, entry.Name()[:len(entry.Name())-6]) // 去掉 .state
		if err != nil {
			continue
		}

		if state.SourcePath == filePath && state.TotalBytes == fileInfo.Size() && state.FileChecksum == checksum {
			transferID = state.ID
			doneChunks = make(map[int]bool)
			for _, chunk := range state.chunks {
				if chunk.Status == "done" {
					doneChunks[chunk.Index] = true
				}
			}
			break
		}
	}

	// 如果找到匹配的状态，请求续传
	if transferID != "" {
		resp, err := c.RequestResume(ctx, transferID, fileName, fileInfo.Size(), checksum)
		if err != nil {
			c.logger.Warn("续传请求失败，将重新开始传输", zap.Error(err))
			transferID = ""
		} else if resp.CanResume {
			// 使用续传模式
			c.logger.Info("使用断点续传模式",
				zap.String("transfer_id", transferID),
				zap.Int("done_chunks", len(resp.DoneChunks)),
			)

			// 合并已完成的块
			for _, idx := range resp.DoneChunks {
				doneChunks[idx] = true
			}

			return c.resumeSendFile(ctx, filePath, destPath, transferID, doneChunks)
		}
	}

	// 普通传输模式
	return c.SendFile(ctx, filePath, destPath)
}

func (c *Client) resumeSendFile(ctx context.Context, filePath string, destPath string, transferID string, doneChunks map[int]bool) (*Transfer, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	fileInfo, _ := file.Stat()
	chunkSize := c.config.ChunkSize
	chunks := CalculateChunks(fileInfo.Size(), chunkSize)

	transfer := &Transfer{
		ID:         transferID,
		Name:       filepath.Base(filePath),
		SourcePath: filePath,
		DestPath:   destPath,
		Mode:       ModeSend,
		Status:     StatusRunning,
		TotalBytes: fileInfo.Size(),
		ChunkCount: len(chunks),
		StartedAt:  time.Now(),
		chunks:     chunks,
		checksums:  make(map[int]string),
	}

	// 计算已完成的字节数
	var transferred int64
	doneCount := 0
	for i, chunk := range chunks {
		if doneChunks[i] {
			chunk.Status = "done"
			chunks[i] = chunk
			transferred += chunk.Size
			doneCount++
		}
	}

	transfer.Transferred = transferred
	transfer.ChunksDone = doneCount

	startTime := time.Now()

	// 发送未完成的块
	for i, chunk := range chunks {
		if doneChunks[i] {
			continue
		}

		select {
		case <-ctx.Done():
			transfer.mu.Lock()
			transfer.Status = StatusCancelled
			now := time.Now()
			transfer.CompletedAt = &now
			transfer.Elapsed = now.Sub(transfer.StartedAt)
			transfer.mu.Unlock()

			// 保存状态
			c.saveTransferState(transfer)
			return transfer, nil
		default:
		}

		// 读取数据块
		chunkData := make([]byte, chunk.Size)
		_, err := file.ReadAt(chunkData, chunk.Offset)
		if err != nil && err != io.EOF {
			continue
		}

		// 计算校验和
		checksum := ComputeChecksum(chunkData)

		// 压缩
		compressed := false
		if c.config.EnableCompression {
			compressedData, err := compressData(chunkData, c.config.CompressionLevel)
			if err == nil && len(compressedData) < len(chunkData) {
				chunkData = compressedData
				compressed = true
			}
		}

		// 加密
		encrypted := false
		if c.config.EnableEncryption && len(c.config.EncryptionKey) > 0 {
			encryptedData, err := Encrypt(chunkData, c.config.EncryptionKey)
			if err != nil {
				continue
			}
			chunkData = encryptedData
			encrypted = true
		}

		// 发送数据块
		payload := ChunkDataPayload{
			TransferID: transferID,
			ChunkIndex: i,
			Offset:     chunk.Offset,
			Size:       chunk.Size,
			Data:       chunkData,
			Checksum:   checksum,
			Compressed: compressed,
			Encrypted:  encrypted,
		}

		if err := c.sendMessage(MsgTypeChunkData, payload); err != nil {
			continue
		}

		// 等待 ACK
		ackMsg, err := c.receiveMessage()
		if err != nil {
			continue
		}

		if ackMsg.Type == MsgTypeChunkAck {
			var ackPayload ChunkAckPayload
			json.Unmarshal(ackMsg.Payload, &ackPayload)

			if ackPayload.Status == "ok" {
				transfer.mu.Lock()
				chunks[i].Status = "done"
				transfer.chunks[i] = chunks[i]
				transfer.Transferred += chunk.Size
				transfer.ChunksDone++

				elapsed := time.Since(startTime).Seconds()
				if elapsed > 0 {
					transfer.SpeedBps = float64(transfer.Transferred) / elapsed
				}
				transfer.mu.Unlock()
			}
		}
	}

	// 发送完成消息
	completePayload := CompletePayload{
		TransferID: transferID,
		Checksum:   transfer.FileChecksum,
		Size:       transfer.TotalBytes,
		Elapsed:    time.Since(startTime).Milliseconds(),
		SpeedBps:   transfer.SpeedBps,
	}

	c.sendMessage(MsgTypeComplete, completePayload)

	transfer.mu.Lock()
	transfer.Status = StatusCompleted
	now := time.Now()
	transfer.CompletedAt = &now
	transfer.Elapsed = now.Sub(transfer.StartedAt)
	transfer.mu.Unlock()

	return transfer, nil
}

func (c *Client) saveTransferState(transfer *Transfer) {
	stateDir := filepath.Join(c.config.TempDir, "states")
	os.MkdirAll(stateDir, 0755)
	transfer.SaveTransferState(stateDir)
}

func (c *Client) sendMessage(msgType string, payload interface{}) error {
	msg, err := NewMessage(msgType, payload)
	if err != nil {
		return err
	}

	data, err := EncodeMessage(msg)
	if err != nil {
		return err
	}

	_, err = c.stream.Write(data)
	return err
}

func (c *Client) receiveMessage() (*Message, error) {
	return DecodeMessage(c.stream)
}
