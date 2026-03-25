// Package natpierce 提供内网穿透服务
// 支持P2P直连和中继模式
package natpierce

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"sync"
	"time"
)

// PierceMode 穿透模式
type PierceMode string

const (
	ModeP2P   PierceMode = "p2p"   // P2P直连
	ModeRelay PierceMode = "relay" // 中继模式
	ModeAuto  PierceMode = "auto"  // 自动选择
)

// Config 穿透配置
type Config struct {
	Enabled     bool       `json:"enabled"`
	Mode        PierceMode `json:"mode"`
	ServerAddr  string     `json:"serverAddr"`  // 中继服务器地址
	ServerPort  int        `json:"serverPort"`  // 中继服务器端口
	LocalPort   int        `json:"localPort"`   // 本地服务端口
	Token       string     `json:"token"`       // 认证令牌
	STUNServers []string   `json:"stunServers"` // STUN服务器列表
	TLSEnabled  bool       `json:"tlsEnabled"`  // 是否启用TLS
	Timeout     int        `json:"timeout"`     // 连接超时(秒)
}

// PierceClient 穿透客户端
type PierceClient struct {
	config   *Config
	conn     net.Conn
	peerAddr *net.UDPAddr
	status   ConnectionStatus
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	onStatus func(status ConnectionStatus)
}

// ConnectionStatus 连接状态
type ConnectionStatus struct {
	Connected    bool       `json:"connected"`
	Mode         PierceMode `json:"mode"`
	PublicIP     string     `json:"publicIP"`
	PublicPort   int        `json:"publicPort"`
	PeerID       string     `json:"peerId"`
	Latency      int        `json:"latency"` // ms
	LastPing     time.Time  `json:"lastPing"`
	ErrorMessage string     `json:"errorMessage,omitempty"`
}

// NewPierceClient 创建穿透客户端
func NewPierceClient(cfg *Config) *PierceClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &PierceClient{
		config: cfg,
		status: ConnectionStatus{},
		ctx:    ctx,
		cancel: cancel,
	}
}

// Start 启动穿透服务
func (pc *PierceClient) Start() error {
	if !pc.config.Enabled {
		return fmt.Errorf("natpierce is disabled")
	}

	// 根据模式选择连接方式
	switch pc.config.Mode {
	case ModeP2P:
		return pc.startP2P()
	case ModeRelay:
		return pc.startRelay()
	case ModeAuto:
		// 优先尝试P2P，失败则回退到中继
		if err := pc.startP2P(); err != nil {
			return pc.startRelay()
		}
		return nil
	default:
		return fmt.Errorf("unknown pierce mode: %s", pc.config.Mode)
	}
}

// startP2P 启动P2P直连
func (pc *PierceClient) startP2P() error {
	// 1. 通过STUN获取公网地址
	publicAddr, err := pc.discoverPublicAddress()
	if err != nil {
		return fmt.Errorf("STUN discovery failed: %w", err)
	}

	pc.mu.Lock()
	pc.status.PublicIP = publicAddr.IP.String()
	pc.status.PublicPort = publicAddr.Port
	pc.status.Mode = ModeP2P
	pc.mu.Unlock()

	// 2. 尝试打洞
	// TODO: 实现UDP打洞逻辑

	return nil
}

// startRelay 启动中继模式
func (pc *PierceClient) startRelay() error {
	// 连接中继服务器 - 使用 net.JoinHostPort 正确处理 IPv6
	addr := net.JoinHostPort(pc.config.ServerAddr, strconv.Itoa(pc.config.ServerPort))

	var conn net.Conn
	var err error

	if pc.config.TLSEnabled {
		conn, err = tls.Dial("tcp", addr, &tls.Config{
			InsecureSkipVerify: false,
		})
	} else {
		conn, err = net.Dial("tcp", addr)
	}

	if err != nil {
		return fmt.Errorf("failed to connect relay server: %w", err)
	}

	pc.mu.Lock()
	pc.conn = conn
	pc.status.Connected = true
	pc.status.Mode = ModeRelay
	pc.mu.Unlock()

	// 发送认证
	auth := map[string]string{
		"token": pc.config.Token,
		"type":  "auth",
	}
	authData, _ := json.Marshal(auth)
	_, err = conn.Write(authData)
	if err != nil {
		return fmt.Errorf("auth failed: %w", err)
	}

	// 启动心跳
	go pc.heartbeat()

	return nil
}

// discoverPublicAddress 通过STUN发现公网地址
func (pc *PierceClient) discoverPublicAddress() (*net.UDPAddr, error) {
	if len(pc.config.STUNServers) == 0 {
		return nil, fmt.Errorf("no STUN servers configured")
	}

	// 尝试每个STUN服务器
	for _, stunServer := range pc.config.STUNServers {
		addr, err := pc.querySTUN(stunServer)
		if err == nil {
			return addr, nil
		}
	}

	return nil, fmt.Errorf("all STUN servers failed")
}

// querySTUN 查询单个STUN服务器
func (pc *PierceClient) querySTUN(server string) (*net.UDPAddr, error) {
	// TODO: 实现完整的STUN协议
	// 这里是简化版本
	return nil, fmt.Errorf("STUN query not implemented")
}

// heartbeat 心跳保活
func (pc *PierceClient) heartbeat() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-pc.ctx.Done():
			return
		case <-ticker.C:
			pc.mu.RLock()
			conn := pc.conn
			pc.mu.RUnlock()

			if conn == nil {
				continue
			}

			// 发送心跳
			ping := map[string]string{"type": "ping"}
			pingData, _ := json.Marshal(ping)
			_, err := conn.Write(pingData)
			if err != nil {
				pc.updateStatus(func(s *ConnectionStatus) {
					s.Connected = false
					s.ErrorMessage = err.Error()
				})
			} else {
				pc.updateStatus(func(s *ConnectionStatus) {
					s.LastPing = time.Now()
				})
			}
		}
	}
}

// Stop 停止穿透服务
func (pc *PierceClient) Stop() error {
	pc.cancel()

	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.conn != nil {
		return pc.conn.Close()
	}
	return nil
}

// GetStatus 获取连接状态
func (pc *PierceClient) GetStatus() ConnectionStatus {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.status
}

// SetStatusCallback 设置状态回调
func (pc *PierceClient) SetStatusCallback(callback func(status ConnectionStatus)) {
	pc.onStatus = callback
}

// updateStatus 更新状态
func (pc *PierceClient) updateStatus(update func(*ConnectionStatus)) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	update(&pc.status)
	if pc.onStatus != nil {
		go pc.onStatus(pc.status)
	}
}
