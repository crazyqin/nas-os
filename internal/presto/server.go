package presto

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/quic-go/quic-go"
	"go.uber.org/zap"
)

// Server Presto 服务端
type Server struct {
	config    *Config
	manager   *Manager
	logger    *zap.Logger
	listener  *quic.Listener
	clients   sync.Map
	running   atomic.Bool
	startTime time.Time
}

// ClientConnection 客户端连接
type ClientConnection struct {
	conn      *quic.Conn
	stream    *quic.Stream
	clientID  string
	addr      string
	connected time.Time
	lastPing  time.Time
}

// NewServer 创建服务端
func NewServer(cfg *Config, manager *Manager, logger *zap.Logger) (*Server, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if manager == nil {
		manager = NewManager(cfg, logger)
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	s := &Server{
		config:  cfg,
		manager: manager,
		logger:  logger,
	}

	return s, nil
}

// Start 启动服务端
func (s *Server) Start(ctx context.Context) error {
	if s.running.Load() {
		return fmt.Errorf("服务端已在运行")
	}

	// 配置 TLS
	tlsConfig, err := s.buildTLSConfig()
	if err != nil {
		return fmt.Errorf("配置 TLS 失败: %w", err)
	}

	// 配置 QUIC
	quicConfig := &quic.Config{
		MaxIdleTimeout:        5 * time.Minute,
		KeepAlivePeriod:       30 * time.Second,
		MaxIncomingStreams:    100,
		MaxIncomingUniStreams: 100,
		EnableDatagrams:       true,
	}

	// 创建监听器
	listener, err := quic.ListenAddr(s.config.ListenAddr, tlsConfig, quicConfig)
	if err != nil {
		return fmt.Errorf("创建 QUIC 监听器失败: %w", err)
	}

	s.listener = listener
	s.running.Store(true)
	s.startTime = time.Now()

	s.logger.Info("Presto 服务端启动",
		zap.String("addr", s.config.ListenAddr),
		zap.Bool("compression", s.config.EnableCompression),
		zap.Bool("encryption", s.config.EnableEncryption),
	)

	// 接受连接
	go s.acceptConnections(ctx)

	return nil
}

// Stop 停止服务端
func (s *Server) Stop() error {
	if !s.running.Load() {
		return nil
	}

	s.running.Store(false)
	if s.listener != nil {
		s.listener.Close()
	}

	// 关闭所有客户端连接
	s.clients.Range(func(key, value interface{}) bool {
		if cc, ok := value.(*ClientConnection); ok {
			cc.conn.CloseWithError(0, "服务端关闭")
		}
		return true
	})

	s.logger.Info("Presto 服务端已停止")
	return nil
}

func (s *Server) buildTLSConfig() (*tls.Config, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS13,
		NextProtos: []string{"presto/1.0"},
	}

	// 加载服务端证书
	if s.config.TLSCertFile != "" && s.config.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(s.config.TLSCertFile, s.config.TLSKeyFile)
		if err != nil {
			return nil, fmt.Errorf("加载 TLS 证书失败: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	} else {
		// 生成自签名证书（开发模式）
		cert, err := generateSelfSignedCert()
		if err != nil {
			return nil, fmt.Errorf("生成自签名证书失败: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{*cert}
	}

	// mTLS 配置
	if s.config.EnableMTLS {
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		if s.config.ClientCAFile != "" {
			caCert, err := os.ReadFile(s.config.ClientCAFile)
			if err != nil {
				return nil, fmt.Errorf("读取客户端 CA 证书失败: %w", err)
			}
			caCertPool := x509.NewCertPool()
			if !caCertPool.AppendCertsFromPEM(caCert) {
				return nil, fmt.Errorf("解析客户端 CA 证书失败")
			}
			tlsConfig.ClientCAs = caCertPool
		}
	}

	return tlsConfig, nil
}

func (s *Server) acceptConnections(ctx context.Context) {
	for s.running.Load() {
		conn, err := s.listener.Accept(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			s.logger.Error("接受连接失败", zap.Error(err))
			continue
		}

		go s.handleConnection(ctx, conn)
	}
}

func (s *Server) handleConnection(ctx context.Context, conn *quic.Conn) {
	remoteAddr := conn.RemoteAddr().String()
	s.logger.Info("新客户端连接", zap.String("addr", remoteAddr))

	cc := &ClientConnection{
		conn:      conn,
		addr:      remoteAddr,
		connected: time.Now(),
		lastPing:  time.Now(),
	}

	// 接受流
	for {
		stream, err := conn.AcceptStream(ctx)
		if err != nil {
			s.logger.Info("客户端断开", zap.String("addr", remoteAddr), zap.Error(err))
			break
		}

		go s.handleStream(ctx, stream, cc)
	}
}

func (s *Server) handleStream(ctx context.Context, stream *quic.Stream, cc *ClientConnection) {
	defer stream.Close()

	// 读取消息
	msg, err := DecodeMessage(stream)
	if err != nil {
		s.logger.Error("读取消息失败", zap.Error(err))
		return
	}

	// 处理消息
	switch msg.Type {
	case MsgTypeHandshake:
		s.handleHandshake(stream, msg, cc)
	case MsgTypeFileMeta:
		s.handleFileMeta(ctx, stream, msg, cc)
	case MsgTypeResumeReq:
		s.handleResumeRequest(stream, msg)
	case MsgTypeChunkReq:
		s.handleChunkRequest(ctx, stream, msg)
	case MsgTypeHeartbeat:
		s.handleHeartbeat(stream, msg, cc)
	default:
		s.sendError(stream, 400, "未知消息类型: "+msg.Type)
	}
}

func (s *Server) handleHandshake(stream *quic.Stream, msg *Message, cc *ClientConnection) {
	var payload HandshakePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.sendError(stream, 400, "无效的握手载荷")
		return
	}

	cc.clientID = payload.ClientID
	s.clients.Store(payload.ClientID, cc)

	s.logger.Info("客户端握手成功",
		zap.String("client_id", payload.ClientID),
		zap.String("addr", cc.addr),
	)

	// 发送握手响应
	response := HandshakePayload{
		Version:  "1.0",
		ClientID: "server",
		Compress: s.config.EnableCompression,
		Encrypt:  s.config.EnableEncryption,
	}

	s.sendMessage(stream, MsgTypeHandshake, response)
}

func (s *Server) handleFileMeta(ctx context.Context, stream *quic.Stream, msg *Message, cc *ClientConnection) {
	var payload FileMetaPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.sendError(stream, 400, "无效的文件元数据")
		return
	}

	s.logger.Info("接收文件元数据",
		zap.String("file_name", payload.FileName),
		zap.Int64("file_size", payload.FileSize),
		zap.Int("chunks", payload.ChunkCount),
	)

	// 创建接收传输任务
	transfer, err := s.manager.CreateTransfer(
		payload.FileName,
		"",
		filepath.Join(s.config.StorageRoot, payload.FileName),
		ModeRecv,
	)
	if err != nil {
		s.sendError(stream, 500, "创建传输任务失败: "+err.Error())
		return
	}

	transfer.mu.Lock()
	transfer.TotalBytes = payload.FileSize
	transfer.ChunkCount = payload.ChunkCount
	transfer.chunks = CalculateChunks(payload.FileSize, s.config.ChunkSize)
	transfer.ClientAddr = cc.addr
	transfer.Status = StatusRunning
	transfer.mu.Unlock()

	// 发送确认
	ackPayload := map[string]interface{}{
		"transfer_id": transfer.ID,
		"chunk_size":  s.config.ChunkSize,
		"accepted":    true,
	}

	s.sendMessage(stream, MsgTypeChunkAck, ackPayload)

	// 启动接收数据块
	go s.receiveChunks(ctx, stream, transfer)
}

func (s *Server) handleResumeRequest(stream *quic.Stream, msg *Message) {
	var payload ResumeRequestPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.sendError(stream, 400, "无效的续传请求")
		return
	}

	// 尝试加载传输状态
	stateDir := filepath.Join(s.config.TempDir, "states")
	transfer, err := LoadTransferState(stateDir, payload.TransferID)
	if err != nil {
		// 无法恢复，返回不支持续传
		s.sendMessage(stream, MsgTypeResumeResp, ResumeResponsePayload{
			TransferID: payload.TransferID,
			CanResume:  false,
		})
		return
	}

	// 检查文件是否匹配
	if transfer.TotalBytes != payload.FileSize {
		s.sendMessage(stream, MsgTypeResumeResp, ResumeResponsePayload{
			TransferID: payload.TransferID,
			CanResume:  false,
		})
		return
	}

	// 收集已完成的块
	doneChunks := make([]int, 0)
	for _, chunk := range transfer.chunks {
		if chunk.Status == "done" {
			doneChunks = append(doneChunks, chunk.Index)
		}
	}

	s.sendMessage(stream, MsgTypeResumeResp, ResumeResponsePayload{
		TransferID:  payload.TransferID,
		CanResume:   true,
		DoneChunks:  doneChunks,
		ChunkStates: transfer.chunks,
	})
}

func (s *Server) handleChunkRequest(ctx context.Context, stream *quic.Stream, msg *Message) {
	var payload ChunkRequestPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.sendError(stream, 400, "无效的数据块请求")
		return
	}

	// 获取传输任务
	transfer, err := s.manager.GetTransfer(payload.TransferID)
	if err != nil {
		s.sendError(stream, 404, "传输任务不存在")
		return
	}

	// 打开源文件
	file, err := os.Open(transfer.SourcePath)
	if err != nil {
		s.sendError(stream, 500, "打开文件失败: "+err.Error())
		return
	}
	defer file.Close()

	// 读取数据块
	chunkData := make([]byte, payload.Size)
	_, err = file.ReadAt(chunkData, payload.Offset)
	if err != nil && err != io.EOF {
		s.sendError(stream, 500, "读取数据块失败: "+err.Error())
		return
	}

	// 计算校验和
	checksum := ComputeChecksum(chunkData)

	// 压缩
	compressed := false
	if s.config.EnableCompression {
		compressedData, err := compressData(chunkData, s.config.CompressionLevel)
		if err == nil && len(compressedData) < len(chunkData) {
			chunkData = compressedData
			compressed = true
		}
	}

	// 加密
	encrypted := false
	if s.config.EnableEncryption && len(s.config.EncryptionKey) > 0 {
		encryptedData, err := Encrypt(chunkData, s.config.EncryptionKey)
		if err != nil {
			s.sendError(stream, 500, "加密数据块失败: "+err.Error())
			return
		}
		chunkData = encryptedData
		encrypted = true
	}

	// 发送数据块
	s.sendMessage(stream, MsgTypeChunkData, ChunkDataPayload{
		TransferID: payload.TransferID,
		ChunkIndex: payload.ChunkIndex,
		Offset:     payload.Offset,
		Size:       payload.Size,
		Data:       chunkData,
		Checksum:   checksum,
		Compressed: compressed,
		Encrypted:  encrypted,
	})
}

func (s *Server) handleHeartbeat(stream *quic.Stream, msg *Message, cc *ClientConnection) {
	cc.lastPing = time.Now()
	s.sendMessage(stream, MsgTypeHeartbeat, map[string]string{"status": "pong"})
}

func (s *Server) receiveChunks(ctx context.Context, stream *quic.Stream, transfer *Transfer) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-transfer.ctx.Done():
			return
		default:
		}

		// 读取数据块消息
		msg, err := DecodeMessage(stream)
		if err != nil {
			if err == io.EOF {
				break
			}
			s.logger.Error("读取数据块失败", zap.Error(err))
			continue
		}

		if msg.Type == MsgTypeComplete {
			s.handleTransferComplete(transfer, msg)
			return
		}

		if msg.Type == MsgTypeError {
			s.handleTransferError(transfer, msg)
			return
		}

		if msg.Type != MsgTypeChunkData {
			continue
		}

		var payload ChunkDataPayload
		if err := json.Unmarshal(msg.Payload, &payload); err != nil {
			s.logger.Error("解析数据块失败", zap.Error(err))
			continue
		}

		// 处理数据块
		if err := s.processChunk(transfer, &payload); err != nil {
			s.logger.Error("处理数据块失败",
				zap.Int("chunk", payload.ChunkIndex),
				zap.Error(err),
			)

			// 发送 NACK
			s.sendMessage(stream, MsgTypeChunkAck, ChunkAckPayload{
				TransferID: transfer.ID,
				ChunkIndex: payload.ChunkIndex,
				Status:     "error",
				Error:      err.Error(),
			})
			continue
		}

		// 发送 ACK
		s.sendMessage(stream, MsgTypeChunkAck, ChunkAckPayload{
			TransferID: transfer.ID,
			ChunkIndex: payload.ChunkIndex,
			Status:     "ok",
			Checksum:   payload.Checksum,
		})

		// 更新进度
		transfer.mu.Lock()
		transfer.Transferred += payload.Size
		transfer.ChunksDone++
		elapsed := time.Since(transfer.StartedAt).Seconds()
		if elapsed > 0 {
			transfer.SpeedBps = float64(transfer.Transferred) / elapsed
		}

		// 保存状态（每10块保存一次）
		if transfer.ChunksDone%10 == 0 {
			stateDir := filepath.Join(s.config.TempDir, "states")
			os.MkdirAll(stateDir, 0755)
			transfer.SaveTransferState(stateDir)
		}
		transfer.mu.Unlock()
	}
}

func (s *Server) processChunk(transfer *Transfer, payload *ChunkDataPayload) error {
	// 解密
	data := payload.Data
	if payload.Encrypted && len(s.config.EncryptionKey) > 0 {
		decrypted, err := Decrypt(data, s.config.EncryptionKey)
		if err != nil {
			return fmt.Errorf("解密数据块失败: %w", err)
		}
		data = decrypted
	}

	// 解压
	if payload.Compressed {
		decompressed, err := decompressData(data)
		if err != nil {
			return fmt.Errorf("解压数据块失败: %w", err)
		}
		data = decompressed
	}

	// 校验和验证
	actualChecksum := ComputeChecksum(data)
	if actualChecksum != payload.Checksum {
		return ErrChecksumMismatch
	}

	// 写入文件
	destPath := transfer.DestPath

	// 确保目录存在
	dir := filepath.Dir(destPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 打开或创建目标文件
	file, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("打开目标文件失败: %w", err)
	}
	defer file.Close()

	// 写入数据
	if _, err := file.WriteAt(data, payload.Offset); err != nil {
		return fmt.Errorf("写入数据失败: %w", err)
	}

	// 更新块状态
	transfer.mu.Lock()
	if payload.ChunkIndex < len(transfer.chunks) {
		transfer.chunks[payload.ChunkIndex].Status = "done"
		transfer.chunks[payload.ChunkIndex].Checksum = actualChecksum
	}
	transfer.checksums[payload.ChunkIndex] = actualChecksum
	transfer.mu.Unlock()

	return nil
}

func (s *Server) handleTransferComplete(transfer *Transfer, msg *Message) {
	var payload CompletePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		s.logger.Error("解析完成消息失败", zap.Error(err))
		return
	}

	transfer.mu.Lock()
	transfer.Status = StatusCompleted
	now := time.Now()
	transfer.CompletedAt = &now
	transfer.Elapsed = now.Sub(transfer.StartedAt)
	transfer.FileChecksum = payload.Checksum
	transfer.mu.Unlock()

	s.logger.Info("传输完成",
		zap.String("id", transfer.ID),
		zap.String("checksum", payload.Checksum),
		zap.Duration("elapsed", transfer.Elapsed),
	)

	// 清理状态文件
	stateFile := filepath.Join(s.config.TempDir, "states", transfer.ID+".state")
	os.Remove(stateFile)
}

func (s *Server) handleTransferError(transfer *Transfer, msg *Message) {
	var payload ErrorPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return
	}

	transfer.mu.Lock()
	transfer.Status = StatusFailed
	transfer.ErrorMsg = payload.Message
	now := time.Now()
	transfer.CompletedAt = &now
	transfer.Elapsed = now.Sub(transfer.StartedAt)
	transfer.mu.Unlock()

	s.logger.Error("传输失败",
		zap.String("id", transfer.ID),
		zap.String("error", payload.Message),
	)
}

func (s *Server) sendMessage(stream *quic.Stream, msgType string, payload interface{}) {
	msg, err := NewMessage(msgType, payload)
	if err != nil {
		s.logger.Error("创建消息失败", zap.Error(err))
		return
	}

	data, err := EncodeMessage(msg)
	if err != nil {
		s.logger.Error("编码消息失败", zap.Error(err))
		return
	}

	if _, err := stream.Write(data); err != nil {
		s.logger.Error("发送消息失败", zap.Error(err))
	}
}

func (s *Server) sendError(stream *quic.Stream, code int, message string) {
	s.sendMessage(stream, MsgTypeError, ErrorPayload{
		Code:    code,
		Message: message,
	})
}

// generateSelfSignedCert 生成自签名证书
func generateSelfSignedCert() (*tls.Certificate, error) {
	// 生成 RSA 私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("生成私钥失败: %w", err)
	}

	// 创建证书模板
	now := time.Now()
	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("生成序列号失败: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"NAS-OS"},
			CommonName:   "localhost",
		},
		NotBefore:             now,
		NotAfter:              now.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
		DNSNames:              []string{"localhost"},
	}

	// 自签名证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, fmt.Errorf("创建证书失败: %w", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return nil, fmt.Errorf("解析证书失败: %w", err)
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  privateKey,
	}

	return tlsCert, nil
}

// 压缩解压工具函数（简化版，实际应使用 zstd 或 lz4）
func compressData(data []byte, level int) ([]byte, error) {
	// TODO: 实现 zstd 压缩
	return data, nil
}

func decompressData(data []byte) ([]byte, error) {
	// TODO: 实现 zstd 解压
	return data, nil
}
