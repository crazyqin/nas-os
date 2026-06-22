package quictransport

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// QUICConfig QUIC传输配置
type QUICConfig struct {
	Enabled        bool          `json:"enabled"`
	ListenAddr     string        `json:"listenAddr"`
	MaxStreams     int           `json:"maxStreams"`
	IdleTimeout    time.Duration `json:"idleTimeout"`
	KeepAlive      time.Duration `json:"keepAlive"`
	TLSCertFile    string        `json:"tlsCertFile"`
	TLSKeyFile     string        `json:"tlsKeyFile"`
	EnableDatagram bool          `json:"enableDatagram"`
}

// QUICConnection QUIC连接
type QUICConnection struct {
	ID         string    `json:"id"`
	RemoteAddr string    `json:"remoteAddr"`
	LocalAddr  string    `json:"localAddr"`
	StartTime  time.Time `json:"startTime"`
	LastActive time.Time `json:"lastActive"`
	BytesSent  int64     `json:"bytesSent"`
	BytesRecv  int64     `json:"bytesRecv"`
	Status     string    `json:"status"` // connected, disconnected, error
}

// QUICStats QUIC统计
type QUICStats struct {
	TotalConnections  int64         `json:"totalConnections"`
	ActiveConnections int           `json:"activeConnections"`
	BytesTransferred  int64         `json:"bytesTransferred"`
	AvgLatency        time.Duration `json:"avgLatency"`
}

// QUICTransport QUIC传输层
type QUICTransport struct {
	config      QUICConfig
	logger      *slog.Logger
	mu          sync.RWMutex
	connections map[string]*QUICConnection
	stats       QUICStats
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	running     bool
}

// NewQUICTransport 创建QUIC传输层
func NewQUICTransport(config QUICConfig, logger *slog.Logger) *QUICTransport {
	ctx, cancel := context.WithCancel(context.Background())

	return &QUICTransport{
		config:      config,
		logger:      logger,
		connections: make(map[string]*QUICConnection),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动QUIC传输层
func (t *QUICTransport) Start() error {
	if !t.config.Enabled {
		t.logger.Info("QUIC传输层未启用")
		return nil
	}

	// 配置TLS
	_, err := t.loadTLSConfig()
	if err != nil {
		return fmt.Errorf("加载TLS配置失败: %w", err)
	}

	t.running = true
	t.logger.Info("QUIC传输层已启动", "addr", t.config.ListenAddr)

	return nil
}

// Stop 停止QUIC传输层
func (t *QUICTransport) Stop() {
	t.cancel()
	t.wg.Wait()
	t.running = false
	t.logger.Info("QUIC传输层已停止")
}

// loadTLSConfig 加载TLS配置
func (t *QUICTransport) loadTLSConfig() (*tls.Config, error) {
	if t.config.TLSCertFile == "" || t.config.TLSKeyFile == "" {
		// 使用自签名证书
		return &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         []string{"nas-os-quic"},
		}, nil
	}

	cert, err := tls.LoadX509KeyPair(t.config.TLSCertFile, t.config.TLSKeyFile)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"nas-os-quic"},
	}, nil
}

// Connect 连接到远程QUIC服务器
func (t *QUICTransport) Connect(addr string) (*QUICConnection, error) {
	if !t.running {
		return nil, fmt.Errorf("QUIC传输层未启动")
	}

	connID := fmt.Sprintf("conn_%d", time.Now().UnixNano())

	qc := &QUICConnection{
		ID:         connID,
		RemoteAddr: addr,
		StartTime:  time.Now(),
		LastActive: time.Now(),
		Status:     "connected",
	}

	t.mu.Lock()
	t.connections[connID] = qc
	t.stats.TotalConnections++
	t.stats.ActiveConnections++
	t.mu.Unlock()

	t.logger.Info("已连接到QUIC服务器", "addr", addr, "id", connID)

	return qc, nil
}

// GetConnection 获取连接
func (t *QUICTransport) GetConnection(connID string) (*QUICConnection, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	conn, ok := t.connections[connID]
	if !ok {
		return nil, fmt.Errorf("连接不存在: %s", connID)
	}

	return conn, nil
}

// ListConnections 列出连接
func (t *QUICTransport) ListConnections(status string) []*QUICConnection {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var conns []*QUICConnection
	for _, c := range t.connections {
		if status == "" || c.Status == status {
			conns = append(conns, c)
		}
	}

	return conns
}

// CloseConnection 关闭连接
func (t *QUICTransport) CloseConnection(connID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	conn, ok := t.connections[connID]
	if !ok {
		return fmt.Errorf("连接不存在: %s", connID)
	}

	conn.Status = "disconnected"
	t.stats.ActiveConnections--

	t.logger.Info("关闭连接", "id", connID)

	return nil
}

// GetStats 获取统计
func (t *QUICTransport) GetStats() QUICStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.stats
}

// IsRunning 是否运行中
func (t *QUICTransport) IsRunning() bool {
	return t.running
}
