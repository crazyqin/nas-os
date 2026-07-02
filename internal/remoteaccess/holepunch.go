// Package remoteaccess 提供 UDP 打洞 (UDP Hole Punching) 实现
// 实现 NAT 穿透的核心算法，支持各种 NAT 类型
package remoteaccess

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// 打洞消息类型.
const (
	HolePunchMsgPing      uint8 = 0x01
	HolePunchMsgPong      uint8 = 0x02
	HolePunchMsgData      uint8 = 0x03
	HolePunchMsgKeepAlive uint8 = 0x04
	HolePunchMsgClose     uint8 = 0x05
)

// HolePunchConn UDP 打洞连接.
type HolePunchConn struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	conn        *net.UDPConn
	localID     string
	remoteID    string
	localAddr   *net.UDPAddr
	remoteAddr  *net.UDPAddr
	established int32 // atomic
	lastPing    time.Time
	lastPong    time.Time
	rtt         time.Duration
	bytesSent   int64 // atomic
	bytesRecv   int64 // atomic
	encrypted   bool
	sharedKey   []byte
	onData      func([]byte)
	ctx         context.Context
	cancel      context.CancelFunc
	done        chan struct{}
}

// HolePunchConfig 打洞配置.
type HolePunchConfig struct {
	LocalID       string
	RemoteID      string
	LocalAddr     *net.UDPAddr
	RemoteAddr    *net.UDPAddr
	Encrypted     bool
	SharedKey     []byte
	MaxRetries    int
	RetryInterval time.Duration
	PingInterval  time.Duration
	Timeout       time.Duration
}

// HolePunchManager UDP 打洞管理器.
type HolePunchManager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	conns     map[string]*HolePunchConn
	localID   string
	stunPool  *STUNServerPool
	localAddr *net.UDPAddr
}

// NewHolePunchManager 创建打洞管理器.
func NewHolePunchManager(logger *zap.Logger, localID string) *HolePunchManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &HolePunchManager{
		logger:   logger,
		conns:    make(map[string]*HolePunchConn),
		localID:  localID,
		stunPool: NewSTUNServerPool(logger),
	}
}

// SetLocalAddr 设置本地监听地址.
func (m *HolePunchManager) SetLocalAddr(addr *net.UDPAddr) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.localAddr = addr
}

// GetExternalAddr 通过 STUN 获取外部地址.
func (m *HolePunchManager) GetExternalAddr(ctx context.Context) (*net.UDPAddr, error) {
	result, err := m.stunPool.QueryBest(ctx)
	if err != nil {
		return nil, err
	}
	return &net.UDPAddr{
		IP:   result.MappedIP,
		Port: result.MappedPort,
	}, nil
}

// PunchHole 执行 UDP 打洞.
func (m *HolePunchManager) PunchHole(ctx context.Context, config HolePunchConfig) (*HolePunchConn, error) {
	// 设置默认值
	if config.MaxRetries <= 0 {
		config.MaxRetries = 20
	}
	if config.RetryInterval <= 0 {
		config.RetryInterval = 500 * time.Millisecond
	}
	if config.PingInterval <= 0 {
		config.PingInterval = 5 * time.Second
	}
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(ctx, config.Timeout)
	defer cancel()

	// 创建或复用 UDP 连接
	var conn *net.UDPConn
	var err error

	m.mu.RLock()
	localAddr := m.localAddr
	m.mu.RUnlock()

	if localAddr != nil {
		conn, err = net.ListenUDP("udp", localAddr)
	} else {
		conn, err = net.ListenUDP("udp", &net.UDPAddr{Port: 0})
	}
	if err != nil {
		return nil, fmt.Errorf("创建 UDP 连接失败: %w", err)
	}

	// 创建打洞连接
	hpConn := &HolePunchConn{
		logger:     m.logger,
		conn:       conn,
		localID:    config.LocalID,
		remoteID:   config.RemoteID,
		localAddr:  conn.LocalAddr().(*net.UDPAddr),
		remoteAddr: config.RemoteAddr,
		encrypted:  config.Encrypted,
		sharedKey:  config.SharedKey,
		ctx:        ctx,
		cancel:     cancel,
		done:       make(chan struct{}),
	}

	// 启动打洞循环
	go hpConn.punchLoop(config.MaxRetries, config.RetryInterval, config.PingInterval)

	// 等待连接建立
	select {
	case <-hpConn.establishedChan():
		m.logger.Info("UDP 打洞成功",
			zap.String("local_id", config.LocalID),
			zap.String("remote_id", config.RemoteID),
			zap.String("remote_addr", config.RemoteAddr.String()),
		)
		return hpConn, nil
	case <-ctx.Done():
		conn.Close()
		return nil, fmt.Errorf("UDP 打洞超时")
	}
}

// establishedChan 返回连接建立信号通道.
func (c *HolePunchConn) establishedChan() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		for {
			if atomic.LoadInt32(&c.established) == 1 {
				close(ch)
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()
	return ch
}

// punchLoop 打洞主循环.
func (c *HolePunchConn) punchLoop(maxRetries int, retryInterval, pingInterval time.Duration) {
	defer close(c.done)

	// 发送打洞 ping
	go func() {
		ticker := time.NewTicker(retryInterval)
		defer ticker.Stop()

		for i := 0; i < maxRetries; i++ {
			select {
			case <-c.ctx.Done():
				return
			default:
			}

			c.sendPing()
			<-ticker.C
		}
	}()

	// 读取循环
	buf := make([]byte, 65536)
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		c.conn.SetReadDeadline(time.Now().Add(pingInterval))
		n, remoteAddr, err := c.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			c.logger.Debug("读取 UDP 数据错误", zap.Error(err))
			continue
		}

		if n < 1 {
			continue
		}

		msgType := buf[0]
		data := buf[1:n]

		switch msgType {
		case HolePunchMsgPing:
			// 收到 ping，发送 pong
			c.mu.Lock()
			c.remoteAddr = remoteAddr
			c.mu.Unlock()
			c.sendPong()
			c.logger.Debug("收到打洞 ping",
				zap.String("from", remoteAddr.String()),
			)

		case HolePunchMsgPong:
			// 收到 pong，打洞成功
			if atomic.CompareAndSwapInt32(&c.established, 0, 1) {
				c.mu.Lock()
				c.remoteAddr = remoteAddr
				c.lastPong = time.Now()
				c.mu.Unlock()

				c.logger.Info("收到打洞 pong，连接建立",
					zap.String("remote", remoteAddr.String()),
				)
			}

		case HolePunchMsgData:
			// 数据包
			atomic.AddInt64(&c.bytesRecv, int64(len(data)))
			if c.onData != nil {
				if c.encrypted && len(c.sharedKey) > 0 {
					decrypted, err := c.decrypt(data)
					if err != nil {
						c.logger.Warn("解密数据失败", zap.Error(err))
						continue
					}
					data = decrypted
				}
				c.onData(data)
			}

		case HolePunchMsgKeepAlive:
			// 保活包
			c.mu.Lock()
			c.lastPong = time.Now()
			c.mu.Unlock()

		case HolePunchMsgClose:
			// 关闭信号
			c.cancel()
			return
		}
	}
}

// sendPing 发送打洞 ping.
func (c *HolePunchConn) sendPing() {
	c.mu.RLock()
	remoteAddr := c.remoteAddr
	c.mu.RUnlock()

	if remoteAddr == nil {
		return
	}

	// 构建 ping 消息: [type(1)] + [timestamp(8)]
	msg := make([]byte, 9)
	msg[0] = HolePunchMsgPing
	binary.BigEndian.PutUint64(msg[1:8], uint64(time.Now().UnixNano()))
	msg[8] = 0 // padding

	c.conn.WriteToUDP(msg, remoteAddr)
	atomic.AddInt64(&c.bytesSent, int64(len(msg)))

	c.mu.Lock()
	c.lastPing = time.Now()
	c.mu.Unlock()
}

// sendPong 发送打洞 pong.
func (c *HolePunchConn) sendPong() {
	c.mu.RLock()
	remoteAddr := c.remoteAddr
	c.mu.RUnlock()

	if remoteAddr == nil {
		return
	}

	msg := make([]byte, 9)
	msg[0] = HolePunchMsgPong
	binary.BigEndian.PutUint64(msg[1:8], uint64(time.Now().UnixNano()))
	msg[8] = 0

	c.conn.WriteToUDP(msg, remoteAddr)
	atomic.AddInt64(&c.bytesSent, int64(len(msg)))
}

// Send 发送数据.
func (c *HolePunchConn) Send(data []byte) error {
	if atomic.LoadInt32(&c.established) != 1 {
		return fmt.Errorf("连接未建立")
	}

	c.mu.RLock()
	remoteAddr := c.remoteAddr
	c.mu.RUnlock()

	if remoteAddr == nil {
		return fmt.Errorf("远程地址未知")
	}

	payload := data
	if c.encrypted && len(c.sharedKey) > 0 {
		encrypted, err := c.encrypt(data)
		if err != nil {
			return fmt.Errorf("加密数据失败: %w", err)
		}
		payload = encrypted
	}

	msg := make([]byte, 1+len(payload))
	msg[0] = HolePunchMsgData
	copy(msg[1:], payload)

	_, err := c.conn.WriteToUDP(msg, remoteAddr)
	if err != nil {
		return err
	}

	atomic.AddInt64(&c.bytesSent, int64(len(msg)))
	return nil
}

// SetOnData 设置数据回调.
func (c *HolePunchConn) SetOnData(fn func([]byte)) {
	c.onData = fn
}

// Close 关闭连接.
func (c *HolePunchConn) Close() error {
	c.mu.RLock()
	remoteAddr := c.remoteAddr
	c.mu.RUnlock()

	// 发送关闭消息
	if remoteAddr != nil {
		msg := []byte{HolePunchMsgClose}
		c.conn.WriteToUDP(msg, remoteAddr)
	}

	c.cancel()
	<-c.done
	return c.conn.Close()
}

// GetStats 获取连接统计.
func (c *HolePunchConn) GetStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"local_addr":  c.localAddr.String(),
		"remote_addr": c.remoteAddr.String(),
		"established": atomic.LoadInt32(&c.established) == 1,
		"bytes_sent":  atomic.LoadInt64(&c.bytesSent),
		"bytes_recv":  atomic.LoadInt64(&c.bytesRecv),
		"rtt":         c.rtt,
		"encrypted":   c.encrypted,
		"last_ping":   c.lastPing,
		"last_pong":   c.lastPong,
	}
}

// encrypt 使用 AES-GCM 加密数据.
func (c *HolePunchConn) encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.sharedKey[:32])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// decrypt 使用 AES-GCM 解密数据.
func (c *HolePunchConn) decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(c.sharedKey[:32])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("加密数据太短")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
