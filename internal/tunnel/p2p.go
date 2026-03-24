// Package tunnel 提供内网穿透服务 - P2P 客户端实现
package tunnel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// P2PConfig P2P 配置
type P2PConfig struct {
	// STUN 服务器
	STUNServers []string
	// TURN 服务器
	TURNServers []string
	TURNUser    string
	TURNPass    string
	// 连接超时
	ConnectTimeout time.Duration
	// ICE 候选收集超时
	ICEGatherTimeout time.Duration
	// 打洞重试次数
	HolePunchRetries int
}

// DefaultP2PConfig 默认 P2P 配置
func DefaultP2PConfig() P2PConfig {
	return P2PConfig{
		STUNServers: []string{
			"stun.l.google.com:19302",
			"stun1.l.google.com:19302",
		},
		ConnectTimeout:   30 * time.Second,
		ICEGatherTimeout: 10 * time.Second,
		HolePunchRetries: 5,
	}
}

// ICECandidate ICE 候选
type ICECandidate struct {
	Type        string `json:"type"`     // host, srflx, relay
	Protocol    string `json:"protocol"` // udp, tcp
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	RelatedIP   string `json:"related_ip,omitempty"`
	RelatedPort int    `json:"related_port,omitempty"`
	Priority    uint32 `json:"priority"`
}

// P2PSession P2P 会话信息
type P2PSession struct {
	LocalID          string         `json:"local_id"`
	RemoteID         string         `json:"remote_id"`
	LocalCandidates  []ICECandidate `json:"local_candidates"`
	RemoteCandidates []ICECandidate `json:"remote_candidates"`
	SelectedPair     *ICECandidate  `json:"selected_pair,omitempty"`
	State            string         `json:"state"`
}

// P2PConn P2P 连接
type P2PConn struct {
	config        P2PConfig
	logger        *zap.Logger
	session       *P2PSession
	localConn     *net.UDPConn
	remoteAddr    *net.UDPAddr
	relayConn     *TURNProtocol
	mu            sync.RWMutex
	connected     atomic.Bool
	closed        atomic.Bool
	ctx           context.Context
	cancel        context.CancelFunc
	writeCh       chan []byte
	readCh        chan []byte
	errCh         chan error
	bytesSent     int64
	bytesReceived int64
}

// NewP2PConn 创建 P2P 连接
func NewP2PConn(config P2PConfig, logger *zap.Logger) *P2PConn {
	ctx, cancel := context.WithCancel(context.Background())
	return &P2PConn{
		config:  config,
		logger:  logger,
		session: &P2PSession{State: "new"},
		ctx:     ctx,
		cancel:  cancel,
		writeCh: make(chan []byte, 100),
		readCh:  make(chan []byte, 100),
		errCh:   make(chan error, 1),
	}
}

// GatherCandidates 收集 ICE 候选
func (c *P2PConn) GatherCandidates(ctx context.Context) ([]ICECandidate, error) {
	candidates := make([]ICECandidate, 0)

	// 1. 收集 host 候选（本地地址）
	hostCandidates, err := c.gatherHostCandidates()
	if err != nil {
		c.logger.Debug("failed to gather host candidates", zap.Error(err))
	} else {
		candidates = append(candidates, hostCandidates...)
	}

	// 2. 收集 srflx 候选（通过 STUN）
	srflxCandidates, err := c.gatherSTUNCandidates(ctx)
	if err != nil {
		c.logger.Debug("failed to gather srflx candidates", zap.Error(err))
	} else {
		candidates = append(candidates, srflxCandidates...)
	}

	// 3. 收集 relay 候选（通过 TURN）- 如果配置了 TURN
	if len(c.config.TURNServers) > 0 {
		relayCandidates, err := c.gatherRelayCandidates(ctx)
		if err != nil {
			c.logger.Debug("failed to gather relay candidates", zap.Error(err))
		} else {
			candidates = append(candidates, relayCandidates...)
		}
	}

	c.mu.Lock()
	c.session.LocalCandidates = candidates
	c.mu.Unlock()

	return candidates, nil
}

// gatherHostCandidates 收集本地地址候选
func (c *P2PConn) gatherHostCandidates() ([]ICECandidate, error) {
	candidates := make([]ICECandidate, 0)

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	priority := uint32(2130706431) // 126 << 24
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				candidates = append(candidates, ICECandidate{
					Type:     "host",
					Protocol: "udp",
					IP:       ipnet.IP.String(),
					Port:     0, // 将在绑定时确定
					Priority: priority,
				})
				priority -= 1
			}
		}
	}

	return candidates, nil
}

// gatherSTUNCandidates 收集 STUN 映射地址候选
func (c *P2PConn) gatherSTUNCandidates(ctx context.Context) ([]ICECandidate, error) {
	candidates := make([]ICECandidate, 0)

	for _, server := range c.config.STUNServers {
		stun := NewSTUNProtocol([]string{server}, c.logger)
		result, err := stun.Discover(ctx, server)
		if err != nil {
			continue
		}

		// 获取本地端口
		localPort := 0
		if c.localConn != nil {
			if addr, ok := c.localConn.LocalAddr().(*net.UDPAddr); ok {
				localPort = addr.Port
			}
		}

		candidates = append(candidates, ICECandidate{
			Type:        "srflx",
			Protocol:    "udp",
			IP:          result.PublicIP.String(),
			Port:        result.PublicPort,
			RelatedIP:   "",
			RelatedPort: localPort,
			Priority:    16777215, // 100 << 24
		})
		break // 只需要一次成功的 STUN 查询
	}

	return candidates, nil
}

// gatherRelayCandidates 收集 TURN 中继地址候选
func (c *P2PConn) gatherRelayCandidates(ctx context.Context) ([]ICECandidate, error) {
	if len(c.config.TURNServers) == 0 {
		return nil, errors.New("no TURN servers configured")
	}

	turn := NewTURNProtocol(TURNClientConfig{
		Server:   c.config.TURNServers[0],
		Username: c.config.TURNUser,
		Password: c.config.TURNPass,
	}, c.logger)

	if err := turn.Connect(ctx); err != nil {
		return nil, err
	}

	relayAddr, err := turn.Allocate(ctx)
	if err != nil {
		_ = turn.Close()
		return nil, err
	}

	c.relayConn = turn

	return []ICECandidate{
		{
			Type:     "relay",
			Protocol: "udp",
			IP:       relayAddr.IP.String(),
			Port:     relayAddr.Port,
			Priority: 16777215, // 最低优先级
		},
	}, nil
}

// SetRemoteCandidates 设置对端候选
func (c *P2PConn) SetRemoteCandidates(candidates []ICECandidate) {
	c.mu.Lock()
	c.session.RemoteCandidates = candidates
	c.mu.Unlock()
}

// Connect 尝试连接到对端
func (c *P2PConn) Connect(ctx context.Context) error {
	c.mu.RLock()
	remoteCandidates := c.session.RemoteCandidates
	c.mu.RUnlock()

	if len(remoteCandidates) == 0 {
		return errors.New("no remote candidates available")
	}

	// 按优先级排序候选
	c.sortCandidates(remoteCandidates)

	// 尝试打洞连接
	for _, candidate := range remoteCandidates {
		err := c.tryConnect(ctx, candidate)
		if err == nil {
			c.connected.Store(true)
			c.mu.Lock()
			c.session.State = "connected"
			c.session.SelectedPair = &candidate
			c.mu.Unlock()

			// 启动读写循环
			go c.readLoop()
			go c.writeLoop()

			c.logger.Info("P2P connection established",
				zap.String("type", candidate.Type),
				zap.String("addr", fmt.Sprintf("%s:%d", candidate.IP, candidate.Port)),
			)
			return nil
		}

		c.logger.Debug("connection attempt failed",
			zap.String("type", candidate.Type),
			zap.String("addr", fmt.Sprintf("%s:%d", candidate.IP, candidate.Port)),
			zap.Error(err),
		)
	}

	return errors.New("failed to establish P2P connection")
}

// tryConnect 尝试连接到指定候选地址
func (c *P2PConn) tryConnect(ctx context.Context, candidate ICECandidate) error {
	switch candidate.Type {
	case "host", "srflx":
		return c.holePunch(ctx, candidate)
	case "relay":
		return c.connectViaRelay(ctx, candidate)
	default:
		return fmt.Errorf("unknown candidate type: %s", candidate.Type)
	}
}

// holePunch 实现 UDP 打洞
func (c *P2PConn) holePunch(ctx context.Context, candidate ICECandidate) error {
	// 创建本地 UDP socket
	localAddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return err
	}

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return err
	}

	c.localConn = conn

	// 目标地址
	remoteAddr := &net.UDPAddr{
		IP:   net.ParseIP(candidate.IP),
		Port: candidate.Port,
	}
	c.remoteAddr = remoteAddr

	// 打洞：发送多个探测包
	// 由于 NAT 映射可能尚未建立，需要同时发送
	for i := 0; i < c.config.HolePunchRetries; i++ {
		// 发送打洞包
		holePunchMsg := []byte("HOLEPUNCH")
		if _, err := conn.WriteToUDP(holePunchMsg, remoteAddr); err != nil {
			continue
		}

		// 等待响应
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		buf := make([]byte, 1024)
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		// 验证响应
		if n > 0 && addr.IP.Equal(remoteAddr.IP) && addr.Port == remoteAddr.Port {
			// 连接成功
			_ = conn.SetReadDeadline(time.Time{}) // 清除超时
			return nil
		}
	}

	_ = conn.Close()
	c.localConn = nil
	return errors.New("hole punch failed")
}

// connectViaRelay 通过 TURN 中继连接
func (c *P2PConn) connectViaRelay(ctx context.Context, candidate ICECandidate) error {
	if c.relayConn == nil {
		return errors.New("no relay connection available")
	}

	peerAddr := &net.UDPAddr{
		IP:   net.ParseIP(candidate.IP),
		Port: candidate.Port,
	}

	// 创建权限
	if err := c.relayConn.CreatePermission(ctx, peerAddr); err != nil {
		return err
	}

	c.remoteAddr = peerAddr
	return nil
}

// sortCandidates 按 ICE 优先级排序
func (c *P2PConn) sortCandidates(candidates []ICECandidate) {
	// 简化排序：host > srflx > relay
	typeScore := map[string]int{
		"host":  3,
		"srflx": 2,
		"relay": 1,
	}

	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			if typeScore[candidates[i].Type] < typeScore[candidates[j].Type] {
				candidates[i], candidates[j] = candidates[j], candidates[i]
			}
		}
	}
}

// Send 发送数据
func (c *P2PConn) Send(data []byte) (int, error) {
	if !c.connected.Load() {
		return 0, errors.New("not connected")
	}

	select {
	case c.writeCh <- data:
		return len(data), nil
	default:
		return 0, errors.New("write buffer full")
	}
}

// Receive 接收数据
func (c *P2PConn) Receive() ([]byte, error) {
	select {
	case data := <-c.readCh:
		return data, nil
	case err := <-c.errCh:
		return nil, err
	case <-c.ctx.Done():
		return nil, c.ctx.Err()
	}
}

// Close 关闭连接
func (c *P2PConn) Close() error {
	if c.closed.Swap(true) {
		return nil
	}

	c.cancel()

	if c.localConn != nil {
		_ = c.localConn.Close()
	}
	if c.relayConn != nil {
		_ = c.relayConn.Close()
	}

	close(c.writeCh)
	close(c.readCh)

	c.connected.Store(false)
	c.mu.Lock()
	c.session.State = "closed"
	c.mu.Unlock()

	return nil
}

// IsConnected 检查是否已连接
func (c *P2PConn) IsConnected() bool {
	return c.connected.Load()
}

// GetSession 获取会话信息
func (c *P2PConn) GetSession() *P2PSession {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.session
}

// readLoop 读循环
func (c *P2PConn) readLoop() {
	buf := make([]byte, 65535)
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		var n int
		var err error

		if c.localConn != nil {
			_ = c.localConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
			n, err = c.localConn.Read(buf)
		} else if c.relayConn != nil {
			var data []byte
			data, _, err = c.relayConn.ReceiveFrom(c.ctx)
			if data != nil {
				n = copy(buf, data)
			}
		}

		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case c.errCh <- err:
			default:
			}
			return
		}

		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])
			atomic.AddInt64(&c.bytesReceived, int64(n))

			select {
			case c.readCh <- data:
			case <-c.ctx.Done():
				return
			}
		}
	}
}

// writeLoop 写循环
func (c *P2PConn) writeLoop() {
	for {
		select {
		case data, ok := <-c.writeCh:
			if !ok {
				return
			}

			var n int
			var err error

			if c.localConn != nil && c.remoteAddr != nil {
				n, err = c.localConn.WriteToUDP(data, c.remoteAddr)
			} else if c.relayConn != nil {
				n, err = c.relayConn.SendTo(c.ctx, data, c.remoteAddr)
			}

			if err != nil {
				c.logger.Debug("write error", zap.Error(err))
				continue
			}

			atomic.AddInt64(&c.bytesSent, int64(n))

		case <-c.ctx.Done():
			return
		}
	}
}

// GetStats 获取统计信息
func (c *P2PConn) GetStats() (bytesSent, bytesReceived int64) {
	return atomic.LoadInt64(&c.bytesSent), atomic.LoadInt64(&c.bytesReceived)
}

// SerializeSession 序列化会话信息（用于信令交换）
func (c *P2PConn) SerializeSession() ([]byte, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return json.Marshal(c.session)
}

// DeserializeSession 反序列化会话信息
func (c *P2PConn) DeserializeSession(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return json.Unmarshal(data, c.session)
}
