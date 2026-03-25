// Package natpierce 提供内网穿透服务
// UDP打洞实现
package natpierce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"
)

// 打洞相关错误
var (
	ErrHolePunchFailed  = errors.New("hole punch failed")
	ErrNoPublicAddr     = errors.New("no public address discovered")
	ErrPeerUnreachable  = errors.New("peer is unreachable")
	ErrSymmetricNAT     = errors.New("symmetric NAT detected, hole punch may not work")
	ErrHandshakeTimeout = errors.New("handshake timeout")
	ErrConnectionClosed = errors.New("connection closed")
	ErrInvalidPeerInfo  = errors.New("invalid peer info")
)

// HolePunchConfig 打洞配置
type HolePunchConfig struct {
	// LocalPort 本地端口（0表示随机）
	LocalPort int `json:"localPort"`

	// HandshakeTimeout 握手超时
	HandshakeTimeout time.Duration `json:"handshakeTimeout"`

	// PunchInterval 打洞尝试间隔
	PunchInterval time.Duration `json:"punchInterval"`

	// MaxPunchAttempts 最大打洞尝试次数
	MaxPunchAttempts int `json:"maxPunchAttempts"`

	// KeepAliveInterval 保活间隔
	KeepAliveInterval time.Duration `json:"keepAliveInterval"`

	// EnablePortPrediction 是否启用端口预测
	EnablePortPrediction bool `json:"enablePortPrediction"`

	// STUNServers STUN服务器列表
	STUNServers []string `json:"stunServers"`

	// RelayServer 中继服务器地址（打洞失败时使用）
	RelayServer string `json:"relayServer"`
}

// DefaultHolePunchConfig 默认打洞配置
var DefaultHolePunchConfig = HolePunchConfig{
	LocalPort:            0,
	HandshakeTimeout:     10 * time.Second,
	PunchInterval:        100 * time.Millisecond,
	MaxPunchAttempts:     50,
	KeepAliveInterval:    30 * time.Second,
	EnablePortPrediction: true,
	STUNServers: []string{
		"stun:stun.l.google.com:19302",
		"stun:stun1.l.google.com:19302",
		"stun:stun2.l.google.com:19302",
	},
}

// PeerInfo 对端信息
type PeerInfo struct {
	ID         string    `json:"id"`
	PublicIP   net.IP    `json:"publicIP"`
	PublicPort int       `json:"publicPort"`
	LocalIP    net.IP    `json:"localIP"`
	LocalPort  int       `json:"localPort"`
	NATType    NATType   `json:"natType"`
	LastSeen   time.Time `json:"lastSeen"`
}

// HolePuncher UDP打洞器
type HolePuncher struct {
	config       HolePunchConfig
	stunClient   *STUNClient
	localAddr    *net.UDPAddr
	publicAddr   *net.UDPAddr
	conn         *net.UDPConn
	peers        map[string]*PeerInfo
	connections  map[string]*P2PConnection
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	onConnect    func(peerID string, conn *P2PConnection)
	onDisconnect func(peerID string, reason error)
	onError      func(peerID string, err error)
}

// P2PConnection P2P连接
type P2PConnection struct {
	PeerID       string
	LocalAddr    *net.UDPAddr
	RemoteAddr   *net.UDPAddr
	Established  time.Time
	LastActivity time.Time
	BytesSent    uint64
	BytesRecv    uint64
	closed       bool
	mu           sync.RWMutex
}

// HolePunchResult 打洞结果
type HolePunchResult struct {
	Success     bool          `json:"success"`
	Method      string        `json:"method"` // "direct", "predicted", "relay"
	PeerID      string        `json:"peerId"`
	LocalAddr   *net.UDPAddr  `json:"localAddr"`
	RemoteAddr  *net.UDPAddr  `json:"remoteAddr"`
	NATType     NATType       `json:"natType"`
	PeerNATType NATType       `json:"peerNATType"`
	RTT         time.Duration `json:"rtt"`
	Error       string        `json:"error,omitempty"`
}

// NewHolePuncher 创建UDP打洞器
func NewHolePuncher(config HolePunchConfig) *HolePuncher {
	ctx, cancel := context.WithCancel(context.Background())
	return &HolePuncher{
		config:      config,
		stunClient:  NewSTUNClient().WithTimeout(5 * time.Second),
		peers:       make(map[string]*PeerInfo),
		connections: make(map[string]*P2PConnection),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动打洞器
func (h *HolePuncher) Start() error {
	// 创建UDP socket
	var localAddr *net.UDPAddr
	if h.config.LocalPort > 0 {
		localAddr = &net.UDPAddr{Port: h.config.LocalPort}
	} else {
		localAddr = &net.UDPAddr{Port: 0}
	}

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return fmt.Errorf("listen UDP: %w", err)
	}

	h.conn = conn
	h.localAddr, _ = conn.LocalAddr().(*net.UDPAddr)

	// 通过STUN获取公网地址
	if len(h.config.STUNServers) > 0 {
		result, err := h.stunClient.Discover(h.config.STUNServers[0])
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("STUN discovery: %w", err)
		}
		h.publicAddr = result.ServerReflexive
	} else {
		// 没有STUN服务器，使用本地地址
		h.publicAddr = h.localAddr
	}

	// 启动接收goroutine
	go h.receiveLoop()
	go h.keepAliveLoop()

	return nil
}

// Stop 停止打洞器
func (h *HolePuncher) Stop() error {
	h.cancel()

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.conn != nil {
		return h.conn.Close()
	}
	return nil
}

// GetPublicAddr 获取公网地址
func (h *HolePuncher) GetPublicAddr() *net.UDPAddr {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.publicAddr
}

// GetLocalAddr 获取本地地址
func (h *HolePuncher) GetLocalAddr() *net.UDPAddr {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.localAddr
}

// AddPeer 添加对端信息
func (h *HolePuncher) AddPeer(info *PeerInfo) error {
	if info == nil || info.ID == "" {
		return ErrInvalidPeerInfo
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	info.LastSeen = time.Now()
	h.peers[info.ID] = info
	return nil
}

// RemovePeer 移除对端
func (h *HolePuncher) RemovePeer(peerID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.peers, peerID)
	if conn, ok := h.connections[peerID]; ok {
		conn.closed = true
		delete(h.connections, peerID)
	}
}

// ConnectToPeer 连接到对端（执行打洞）
func (h *HolePuncher) ConnectToPeer(peerID string) (*HolePunchResult, error) {
	h.mu.RLock()
	peer, exists := h.peers[peerID]
	h.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%w: peer %s not found", ErrInvalidPeerInfo, peerID)
	}

	if h.publicAddr == nil {
		return nil, ErrNoPublicAddr
	}

	start := time.Now()

	// 构建对端地址
	peerAddr := &net.UDPAddr{
		IP:   peer.PublicIP,
		Port: peer.PublicPort,
	}

	// 执行打洞
	result, err := h.punchHole(peerID, peerAddr, peer)
	if err != nil {
		// 打洞失败，尝试端口预测
		if h.config.EnablePortPrediction && peer.NATType != NATTypeSymmetric {
			result, err = h.punchWithPrediction(peerID, peer)
			if err == nil {
				return result, nil
			}
		}

		// 仍然失败，返回错误
		return &HolePunchResult{
			Success: false,
			PeerID:  peerID,
			Error:   err.Error(),
		}, err
	}

	result.RTT = time.Since(start)
	return result, nil
}

// punchHole 执行UDP打洞
func (h *HolePuncher) punchHole(peerID string, peerAddr *net.UDPAddr, peer *PeerInfo) (*HolePunchResult, error) {
	ctx, cancel := context.WithTimeout(h.ctx, h.config.HandshakeTimeout)
	defer cancel()

	// 创建握手消息
	handshake := HandshakeMessage{
		Type:      "punch",
		PeerID:    peerID,
		Timestamp: time.Now().UnixNano(),
	}

	handshakeData, err := json.Marshal(handshake)
	if err != nil {
		return nil, fmt.Errorf("marshal handshake: %w", err)
	}

	// 同时发送和接收
	// 发送打洞包
	punchDone := make(chan struct{})
	var punchErr error

	go func() {
		defer close(punchDone)

		for i := 0; i < h.config.MaxPunchAttempts; i++ {
			select {
			case <-ctx.Done():
				return
			default:
			}

			_, err := h.conn.WriteToUDP(handshakeData, peerAddr)
			if err != nil {
				punchErr = err
				return
			}

			time.Sleep(h.config.PunchInterval)
		}
	}()

	// 等待对端响应
	result := &HolePunchResult{
		Method:     "direct",
		PeerID:     peerID,
		LocalAddr:  h.localAddr,
		RemoteAddr: peerAddr,
	}

	for {
		select {
		case <-ctx.Done():
			<-punchDone
			if punchErr != nil {
				return nil, punchErr
			}
			return nil, ErrHandshakeTimeout

		case <-punchDone:
			if punchErr != nil {
				return nil, punchErr
			}
		default:
		}

		// 非阻塞检查是否已建立连接
		h.mu.RLock()
		conn, exists := h.connections[peerID]
		h.mu.RUnlock()

		if exists && !conn.closed {
			result.Success = true
			return result, nil
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// punchWithPrediction 使用端口预测进行打洞
func (h *HolePuncher) punchWithPrediction(peerID string, peer *PeerInfo) (*HolePunchResult, error) {
	// 端口预测：假设NAT按顺序分配端口
	// 尝试预测对端的下一个端口

	peerAddr := &net.UDPAddr{
		IP:   peer.PublicIP,
		Port: peer.PublicPort,
	}

	// 尝试预测端口（前后各尝试几个）
	predictPorts := []int{
		peer.PublicPort,
		peer.PublicPort + 1,
		peer.PublicPort + 2,
		peer.PublicPort - 1,
		peer.PublicPort - 2,
	}

	ctx, cancel := context.WithTimeout(h.ctx, h.config.HandshakeTimeout)
	defer cancel()

	handshake := HandshakeMessage{
		Type:      "predicted_punch",
		PeerID:    peerID,
		Timestamp: time.Now().UnixNano(),
	}
	handshakeData, _ := json.Marshal(handshake)

	for _, port := range predictPorts {
		select {
		case <-ctx.Done():
			return nil, ErrHandshakeTimeout
		default:
		}

		if port <= 0 || port > 65535 {
			continue
		}

		peerAddr.Port = port

		// 发送打洞包
		for i := 0; i < 5; i++ {
			_, _ = h.conn.WriteToUDP(handshakeData, peerAddr)
			time.Sleep(h.config.PunchInterval)
		}

		// 检查连接
		time.Sleep(100 * time.Millisecond)
		h.mu.RLock()
		conn, exists := h.connections[peerID]
		h.mu.RUnlock()

		if exists && !conn.closed {
			return &HolePunchResult{
				Success:    true,
				Method:     "predicted",
				PeerID:     peerID,
				LocalAddr:  h.localAddr,
				RemoteAddr: peerAddr,
			}, nil
		}
	}

	return nil, ErrHolePunchFailed
}

// Send 发送数据到对端
func (h *HolePuncher) Send(peerID string, data []byte) (int, error) {
	h.mu.RLock()
	conn, exists := h.connections[peerID]
	if !exists {
		h.mu.RUnlock()
		return 0, fmt.Errorf("no connection to peer %s", peerID)
	}
	remoteAddr := conn.RemoteAddr
	h.mu.RUnlock()

	n, err := h.conn.WriteToUDP(data, remoteAddr)
	if err != nil {
		return 0, err
	}

	conn.mu.Lock()
	conn.BytesSent += uint64(n)
	conn.LastActivity = time.Now()
	conn.mu.Unlock()

	return n, nil
}

// Broadcast 广播数据到所有连接的对端
func (h *HolePuncher) Broadcast(data []byte) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for peerID, conn := range h.connections {
		if conn.closed {
			continue
		}
		_, err := h.conn.WriteToUDP(data, conn.RemoteAddr)
		if err != nil && h.onError != nil {
			h.onError(peerID, err)
		}
	}
	return nil
}

// receiveLoop 接收循环
func (h *HolePuncher) receiveLoop() {
	buf := make([]byte, 65535)

	for {
		select {
		case <-h.ctx.Done():
			return
		default:
		}

		_ = h.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, remoteAddr, err := h.conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			return
		}

		data := make([]byte, n)
		copy(data, buf[:n])

		// 解析消息
		go h.handleMessage(data, remoteAddr)
	}
}

// handleMessage 处理接收到的消息
func (h *HolePuncher) handleMessage(data []byte, remoteAddr *net.UDPAddr) {
	// 尝试解析为握手消息
	var handshake HandshakeMessage
	if err := json.Unmarshal(data, &handshake); err == nil {
		h.handleHandshake(&handshake, remoteAddr)
		return
	}

	// 尝试解析为数据消息
	var msg DataMessage
	if err := json.Unmarshal(data, &msg); err == nil {
		h.handleDataMessage(&msg, remoteAddr)
		return
	}

	// 尝试解析为心跳消息
	var ping PingMessage
	if err := json.Unmarshal(data, &ping); err == nil && ping.Type == "ping" {
		h.handlePing(&ping, remoteAddr)
		return
	}
}

// handleHandshake 处理握手消息
func (h *HolePuncher) handleHandshake(msg *HandshakeMessage, remoteAddr *net.UDPAddr) {
	// 更新或创建连接
	h.mu.Lock()
	defer h.mu.Unlock()

	conn, exists := h.connections[msg.PeerID]
	if !exists {
		conn = &P2PConnection{
			PeerID:      msg.PeerID,
			LocalAddr:   h.localAddr,
			RemoteAddr:  remoteAddr,
			Established: time.Now(),
		}
		h.connections[msg.PeerID] = conn
	} else {
		conn.RemoteAddr = remoteAddr
		conn.LastActivity = time.Now()
	}

	// 回复确认
	ack := HandshakeMessage{
		Type:      "ack",
		PeerID:    msg.PeerID,
		Timestamp: time.Now().UnixNano(),
	}
	ackData, _ := json.Marshal(ack)
	_, _ = h.conn.WriteToUDP(ackData, remoteAddr)

	// 触发连接回调
	if h.onConnect != nil {
		go h.onConnect(msg.PeerID, conn)
	}
}

// handleDataMessage 处理数据消息
func (h *HolePuncher) handleDataMessage(msg *DataMessage, remoteAddr *net.UDPAddr) {
	h.mu.Lock()
	conn, exists := h.connections[msg.PeerID]
	if exists {
		conn.LastActivity = time.Now()
		conn.BytesRecv += uint64(len(msg.Data))
	}
	h.mu.Unlock()

	// TODO: 将数据传递给上层应用
}

// handlePing 处理心跳
func (h *HolePuncher) handlePing(msg *PingMessage, remoteAddr *net.UDPAddr) {
	pong := PingMessage{
		Type:      "pong",
		Timestamp: time.Now().UnixNano(),
	}
	pongData, _ := json.Marshal(pong)
	_, _ = h.conn.WriteToUDP(pongData, remoteAddr)
}

// keepAliveLoop 保活循环
func (h *HolePuncher) keepAliveLoop() {
	ticker := time.NewTicker(h.config.KeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case <-ticker.C:
			h.sendKeepAlive()
		}
	}
}

// sendKeepAlive 发送保活包
func (h *HolePuncher) sendKeepAlive() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	ping := PingMessage{
		Type:      "ping",
		Timestamp: time.Now().UnixNano(),
	}
	pingData, _ := json.Marshal(ping)

	for _, conn := range h.connections {
		if conn.closed {
			continue
		}
		_, _ = h.conn.WriteToUDP(pingData, conn.RemoteAddr)
	}
}

// SetOnConnect 设置连接回调
func (h *HolePuncher) SetOnConnect(callback func(peerID string, conn *P2PConnection)) {
	h.onConnect = callback
}

// SetOnDisconnect 设置断开回调
func (h *HolePuncher) SetOnDisconnect(callback func(peerID string, reason error)) {
	h.onDisconnect = callback
}

// SetOnError 设置错误回调
func (h *HolePuncher) SetOnError(callback func(peerID string, err error)) {
	h.onError = callback
}

// GetConnections 获取所有连接
func (h *HolePuncher) GetConnections() map[string]*P2PConnection {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make(map[string]*P2PConnection)
	for k, v := range h.connections {
		result[k] = v
	}
	return result
}

// GetConnection 获取指定连接
func (h *HolePuncher) GetConnection(peerID string) (*P2PConnection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, exists := h.connections[peerID]
	return conn, exists
}

// IsConnected 检查是否已连接
func (h *HolePuncher) IsConnected(peerID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, exists := h.connections[peerID]
	return exists && !conn.closed
}

// CloseConnection 关闭指定连接
func (h *HolePuncher) CloseConnection(peerID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	conn, exists := h.connections[peerID]
	if !exists {
		return nil
	}

	conn.closed = true
	delete(h.connections, peerID)

	if h.onDisconnect != nil {
		go h.onDisconnect(peerID, ErrConnectionClosed)
	}

	return nil
}

// 消息类型定义
type HandshakeMessage struct {
	Type      string `json:"type"`
	PeerID    string `json:"peerId"`
	Timestamp int64  `json:"timestamp"`
}

type DataMessage struct {
	Type      string `json:"type"`
	PeerID    string `json:"peerId"`
	Data      []byte `json:"data"`
	Timestamp int64  `json:"timestamp"`
}

type PingMessage struct {
	Type      string `json:"type"`
	Timestamp int64  `json:"timestamp"`
}
