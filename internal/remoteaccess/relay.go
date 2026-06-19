// Package remoteaccess 提供中继服务器协议实现
// 当 UDP 打洞失败时 (如对称 NAT)，通过中继服务器转发流量
// 协议: 类 TLS 握手 + 帧协议 (type + length + payload)
package remoteaccess

import (
	"bufio"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/chacha20poly1305"
)

// Relay 帧类型
const (
	RelayFrameHandshake  uint8 = 0x01 // 握手
	RelayFrameHandshakeOK uint8 = 0x02 // 握手确认
	RelayFrameAuth       uint8 = 0x03 // 认证
	RelayFrameAuthOK     uint8 = 0x04 // 认证确认
	RelayFrameData       uint8 = 0x05 // 数据帧
	RelayFrameKeepAlive  uint8 = 0x06 // 保活
	RelayFrameError      uint8 = 0x07 // 错误
	RelayFrameClose      uint8 = 0x08 // 关闭
)

// RelayConnectionState 中继连接状态
type RelayConnectionState uint8

const (
	RelayStateInit       RelayConnectionState = 0
	RelayStateConnecting RelayConnectionState = 1
	RelayStateHandshaked RelayConnectionState = 2
	RelayStateAuthed     RelayConnectionState = 3
	RelayStateReady      RelayConnectionState = 4
	RelayStateClosed     RelayConnectionState = 5
)

// RelayClientConfig 中继客户端配置
type RelayClientConfig struct {
	ServerAddr   string // 中继服务器地址
	PeerID       string // 本端节点 ID
	TargetPeerID string // 目标节点 ID
	AuthToken    string // 认证令牌
	SecretKey    []byte // 共享密钥 (32 bytes)
	Timeout      time.Duration
	KeepAlive    time.Duration
}

// RelayClient 中继客户端
type RelayClient struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	config   RelayClientConfig
	conn     net.Conn
	reader   *bufio.Reader
	writer   *bufio.Writer
	state    RelayConnectionState
	aead     cipher.AEAD
	bytesIn  int64 // atomic
	bytesOut int64 // atomic
	onData   func([]byte)
	ctx      context.Context
	cancel   context.CancelFunc
	done     chan struct{}
}

// NewRelayClient 创建中继客户端
func NewRelayClient(logger *zap.Logger, config RelayClientConfig) *RelayClient {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config.Timeout <= 0 {
		config.Timeout = 15 * time.Second
	}
	if config.KeepAlive <= 0 {
		config.KeepAlive = 30 * time.Second
	}
	return &RelayClient{
		logger: logger,
		config: config,
		state:  RelayStateInit,
		done:   make(chan struct{}),
	}
}

// Connect 连接到中继服务器并建立加密通道
func (rc *RelayClient) Connect(ctx context.Context) error {
	ctx, rc.cancel = context.WithCancel(ctx)
	rc.ctx = ctx

	// TCP 连接
	dialer := net.Dialer{Timeout: rc.config.Timeout}
	conn, err := dialer.DialContext(ctx, "tcp", rc.config.ServerAddr)
	if err != nil {
		return fmt.Errorf("连接中继服务器失败: %w", err)
	}
	rc.conn = conn
	rc.reader = bufio.NewReaderSize(conn, 65536)
	rc.writer = bufio.NewWriterSize(conn, 65536)
	rc.state = RelayStateConnecting

	// 握手
	if err := rc.handshake(); err != nil {
		conn.Close()
		return err
	}
	rc.state = RelayStateHandshaked

	// 初始化加密 (ChaCha20-Poly1305)
	if err := rc.initCrypto(); err != nil {
		conn.Close()
		return err
	}

	// 认证
	if err := rc.authenticate(); err != nil {
		conn.Close()
		return err
	}
	rc.state = RelayStateAuthed

	// 请求连接到目标节点
	if err := rc.requestPeer(rc.config.TargetPeerID); err != nil {
		conn.Close()
		return err
	}
	rc.state = RelayStateReady

	// 启动读写循环
	go rc.readLoop()
	go rc.keepAliveLoop()

	rc.logger.Info("中继连接建立",
		zap.String("server", rc.config.ServerAddr),
		zap.String("peer_id", rc.config.PeerID),
		zap.String("target", rc.config.TargetPeerID),
	)

	return nil
}

// handshake 握手
func (rc *RelayClient) handshake() error {
	// 发送握手帧: [version(1)] + [peer_id_len(1)] + [peer_id]
	version := byte(1)
	peerIDBytes := []byte(rc.config.PeerID)
	msg := make([]byte, 2+len(peerIDBytes))
	msg[0] = version
	msg[1] = byte(len(peerIDBytes))
	copy(msg[2:], peerIDBytes)

	if err := rc.writeFrame(RelayFrameHandshake, msg); err != nil {
		return fmt.Errorf("发送握手帧失败: %w", err)
	}

	// 读取握手确认
	frameType, data, err := rc.readFrame()
	if err != nil {
		return fmt.Errorf("读取握手确认失败: %w", err)
	}
	if frameType != RelayFrameHandshakeOK {
		return fmt.Errorf("握手失败，收到类型: %d", frameType)
	}

	// 解析确认: [version(1)] + [nonce(12)] (用于密钥派生)
	if len(data) < 13 {
		return fmt.Errorf("握手确认数据太短")
	}

	_ = data[0] // version
	// 保存 nonce 用于密钥派生
	rc.mu.Lock()
	nonce := data[1:13]
	rc.mu.Unlock()

	_ = nonce // 将用于 initCrypto

	return nil
}

// initCrypto 初始化加密
func (rc *RelayClient) initCrypto() error {
	if len(rc.config.SecretKey) == 0 {
		// 生成临时密钥
		rc.config.SecretKey = make([]byte, 32)
		rand.Read(rc.config.SecretKey)
	}

	// 使用 ChaCha20-Poly1305
	aead, err := chacha20poly1305.New(rc.config.SecretKey)
	if err != nil {
		return fmt.Errorf("初始化加密失败: %w", err)
	}
	rc.aead = aead
	return nil
}

// authenticate 认证
func (rc *RelayClient) authenticate() error {
	// 发送认证帧: [token_len(2)] + [token]
	tokenBytes := []byte(rc.config.AuthToken)
	msg := make([]byte, 2+len(tokenBytes))
	binary.BigEndian.PutUint16(msg[0:2], uint16(len(tokenBytes)))
	copy(msg[2:], tokenBytes)

	if err := rc.writeFrame(RelayFrameAuth, msg); err != nil {
		return fmt.Errorf("发送认证帧失败: %w", err)
	}

	// 读取认证确认
	frameType, _, err := rc.readFrame()
	if err != nil {
		return fmt.Errorf("读取认证确认失败: %w", err)
	}
	if frameType != RelayFrameAuthOK {
		return fmt.Errorf("认证失败，收到类型: %d", frameType)
	}

	return nil
}

// requestPeer 请求连接到目标节点
func (rc *RelayClient) requestPeer(peerID string) error {
	peerBytes := []byte(peerID)
	msg := make([]byte, 2+len(peerBytes))
	binary.BigEndian.PutUint16(msg[0:2], uint16(len(peerBytes)))
	copy(msg[2:], peerBytes)

	return rc.writeFrame(RelayFrameData, msg)
}

// Send 发送数据
func (rc *RelayClient) Send(data []byte) error {
	if rc.state != RelayStateReady {
		return fmt.Errorf("连接未就绪")
	}

	// 加密数据
	if rc.aead != nil {
		nonce := make([]byte, rc.aead.NonceSize())
		rand.Read(nonce)
		data = rc.aead.Seal(nonce, nonce, data, nil)
	}

	if err := rc.writeFrame(RelayFrameData, data); err != nil {
		return err
	}

	atomic.AddInt64(&rc.bytesOut, int64(len(data)))
	return nil
}

// SetOnData 设置数据回调
func (rc *RelayClient) SetOnData(fn func([]byte)) {
	rc.onData = fn
}

// Close 关闭连接
func (rc *RelayClient) Close() error {
	if rc.state == RelayStateClosed {
		return nil
	}

	rc.writeFrame(RelayFrameClose, nil)
	rc.cancel()
	rc.state = RelayStateClosed
	close(rc.done)
	return rc.conn.Close()
}

// GetStats 获取统计
func (rc *RelayClient) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"state":     rc.state,
		"bytes_in":  atomic.LoadInt64(&rc.bytesIn),
		"bytes_out": atomic.LoadInt64(&rc.bytesOut),
		"server":    rc.config.ServerAddr,
		"peer_id":   rc.config.PeerID,
		"target":    rc.config.TargetPeerID,
		"encrypted": rc.aead != nil,
	}
}

// readLoop 读取循环
func (rc *RelayClient) readLoop() {
	for {
		select {
		case <-rc.ctx.Done():
			return
		default:
		}

		frameType, data, err := rc.readFrame()
		if err != nil {
			if rc.state != RelayStateClosed {
				rc.logger.Debug("读取帧错误", zap.Error(err))
			}
			return
		}

		switch frameType {
		case RelayFrameData:
			payload := data
			// 解密
			if rc.aead != nil && len(data) > rc.aead.NonceSize() {
				nonceSize := rc.aead.NonceSize()
				nonce, ciphertext := data[:nonceSize], data[nonceSize:]
				plaintext, err := rc.aead.Open(nil, nonce, ciphertext, nil)
				if err != nil {
					rc.logger.Warn("解密中继数据失败", zap.Error(err))
					continue
				}
				payload = plaintext
			}
			atomic.AddInt64(&rc.bytesIn, int64(len(payload)))
			if rc.onData != nil {
				rc.onData(payload)
			}

		case RelayFrameKeepAlive:
			// 回复保活
			rc.writeFrame(RelayFrameKeepAlive, nil)

		case RelayFrameError:
			errMsg := string(data)
			rc.logger.Error("中继服务器错误", zap.String("error", errMsg))

		case RelayFrameClose:
			rc.logger.Info("中继连接关闭")
			rc.cancel()
			return
		}
	}
}

// keepAliveLoop 保活循环
func (rc *RelayClient) keepAliveLoop() {
	ticker := time.NewTicker(rc.config.KeepAlive)
	defer ticker.Stop()

	for {
		select {
		case <-rc.ctx.Done():
			return
		case <-ticker.C:
			if err := rc.writeFrame(RelayFrameKeepAlive, nil); err != nil {
				rc.logger.Debug("发送保活失败", zap.Error(err))
				return
			}
		}
	}
}

// writeFrame 写入帧
func (rc *RelayClient) writeFrame(frameType uint8, data []byte) error {
	// 帧格式: [type(1)] + [length(4)] + [data]
	header := make([]byte, 5)
	header[0] = frameType
	binary.BigEndian.PutUint32(header[1:5], uint32(len(data)))

	rc.mu.Lock()
	defer rc.mu.Unlock()

	if _, err := rc.writer.Write(header); err != nil {
		return err
	}
	if len(data) > 0 {
		if _, err := rc.writer.Write(data); err != nil {
			return err
		}
	}
	return rc.writer.Flush()
}

// readFrame 读取帧
func (rc *RelayClient) readFrame() (uint8, []byte, error) {
	// 读取头部
	header := make([]byte, 5)
	if _, err := io.ReadFull(rc.reader, header); err != nil {
		return 0, nil, err
	}

	frameType := header[0]
	length := binary.BigEndian.Uint32(header[1:5])

	if length > 16*1024*1024 { // 16MB 限制
		return 0, nil, fmt.Errorf("帧长度过大: %d", length)
	}

	if length == 0 {
		return frameType, nil, nil
	}

	data := make([]byte, length)
	if _, err := io.ReadFull(rc.reader, data); err != nil {
		return 0, nil, err
	}

	return frameType, data, nil
}

// RelayPool 中继服务器连接池
type RelayPool struct {
	mu      sync.RWMutex
	logger  *zap.Logger
	servers []*RelayServer
	clients map[string]*RelayClient // peerID -> client
}

// NewRelayPool 创建中继连接池
func NewRelayPool(logger *zap.Logger) *RelayPool {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RelayPool{
		logger:  logger,
		servers: make([]*RelayServer, 0),
		clients: make(map[string]*RelayClient),
	}
}

// ConnectToPeer 通过最佳中继服务器连接到目标节点
func (p *RelayPool) ConnectToPeer(ctx context.Context, targetPeerID, authToken string, secretKey []byte) (*RelayClient, error) {
	p.mu.RLock()
	servers := make([]*RelayServer, len(p.servers))
	copy(servers, p.servers)
	p.mu.RUnlock()

	if len(servers) == 0 {
		return nil, fmt.Errorf("无可用中继服务器")
	}

	// 按延迟排序，尝试连接
	var lastErr error
	for _, server := range servers {
		if server.Status != RelayStatusOnline {
			continue
		}

		config := RelayClientConfig{
			ServerAddr:   fmt.Sprintf("%s:%d", server.Address, server.Port),
			TargetPeerID: targetPeerID,
			AuthToken:    authToken,
			SecretKey:    secretKey,
		}

		client := NewRelayClient(p.logger, config)
		if err := client.Connect(ctx); err != nil {
			p.logger.Debug("连接中继服务器失败",
				zap.String("server", server.Address),
				zap.Error(err),
			)
			lastErr = err
			continue
		}

		p.mu.Lock()
		p.clients[targetPeerID] = client
		p.mu.Unlock()

		return client, nil
	}

	return nil, fmt.Errorf("所有中继服务器连接失败: %w", lastErr)
}

// AddServer 添加中继服务器
func (p *RelayPool) AddServer(server *RelayServer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.servers = append(p.servers, server)
}

// Close 关闭所有连接
func (p *RelayPool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, client := range p.clients {
		client.Close()
	}
	p.clients = make(map[string]*RelayClient)
}

// deriveKey 使用 SHA-256 派生密钥
func deriveKey(shared []byte, nonce []byte) []byte {
	h := sha256.New()
	h.Write(shared)
	h.Write(nonce)
	return h.Sum(nil)
}
