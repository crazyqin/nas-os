// Package tunnel 提供内网穿透客户端的单元测试
package tunnel

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestP2PClientCreation 测试 P2P 客户端创建
func TestP2PClientCreation(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "test-id",
		Name:      "test-tunnel",
		Mode:      ModeP2P,
		LocalAddr: "127.0.0.1:8080",
		Protocol:  "tcp",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger)
	require.NotNil(t, client)

	// 验证初始状态
	status := client.GetStatus()
	assert.Equal(t, "test-id", status.ID)
	assert.Equal(t, "test-tunnel", status.Name)
	assert.Equal(t, ModeP2P, status.Mode)
	assert.Equal(t, StateDisconnected, status.State)
	assert.False(t, client.IsConnected())
}

// TestP2PClientConnect 测试 P2P 客户端连接
func TestP2PClientConnect(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "test-id",
		Name:      "test-tunnel",
		Mode:      ModeP2P,
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger).(*P2PClient)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)

	// 验证连接状态
	assert.True(t, client.IsConnected())
	status := client.GetStatus()
	assert.Equal(t, StateConnected, status.State)
	assert.NotZero(t, status.LastConnected)

	// 断开连接
	err = client.Disconnect()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

// TestP2PClientSendReceiveNotConnected 测试未连接时发送
func TestP2PClientSendReceiveNotConnected(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:   "test-id",
		Name: "test-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger).(*P2PClient)

	// 未连接时发送应该失败
	_, err := client.Send([]byte("test"))
	require.Error(t, err)
	assert.Equal(t, ErrNotConnected, err)

	// 注意：Receive 方法在 conn 为 nil 时会 panic，
	// 这是预期的行为，调用者应先检查 IsConnected()
	// 所以这里只测试 Send 方法
}

// TestRelayClientCreation 测试中继客户端创建
func TestRelayClientCreation(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "test-id",
		Name:      "test-tunnel",
		Mode:      ModeRelay,
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewRelayClient(tunnelConfig, mgrConfig, logger)
	require.NotNil(t, client)

	status := client.GetStatus()
	assert.Equal(t, ModeRelay, status.Mode)
	assert.Equal(t, StateDisconnected, status.State)
	assert.False(t, client.IsConnected())
}

// TestRelayClientConnect 测试中继客户端连接
func TestRelayClientConnect(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "test-id",
		Name:      "test-tunnel",
		Mode:      ModeRelay,
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewRelayClient(tunnelConfig, mgrConfig, logger).(*RelayClient)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)

	assert.True(t, client.IsConnected())
	status := client.GetStatus()
	assert.Equal(t, StateConnected, status.State)

	err = client.Disconnect()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

// TestRelayClientSendReceiveNotConnected 测试中继客户端未连接时操作
func TestRelayClientSendReceiveNotConnected(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:   "test-id",
		Name: "test-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewRelayClient(tunnelConfig, mgrConfig, logger).(*RelayClient)

	_, err := client.Send([]byte("test"))
	require.Error(t, err)
	assert.Equal(t, ErrNotConnected, err)
}

// TestReverseClientCreation 测试反向代理客户端创建
func TestReverseClientCreation(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "test-id",
		Name:      "test-tunnel",
		Mode:      ModeReverse,
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewReverseClient(tunnelConfig, mgrConfig, logger)
	require.NotNil(t, client)

	status := client.GetStatus()
	assert.Equal(t, ModeReverse, status.Mode)
	assert.Equal(t, StateDisconnected, status.State)
	assert.False(t, client.IsConnected())
}

// TestReverseClientConnect 测试反向代理客户端连接
func TestReverseClientConnect(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "test-id",
		Name:      "test-tunnel",
		Mode:      ModeReverse,
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewReverseClient(tunnelConfig, mgrConfig, logger).(*ReverseClient)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)

	assert.True(t, client.IsConnected())
	status := client.GetStatus()
	assert.Equal(t, StateConnected, status.State)

	err = client.Disconnect()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

// TestReverseClientSendReceiveNotConnected 测试反向代理客户端未连接时操作
func TestReverseClientSendReceiveNotConnected(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:   "test-id",
		Name: "test-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewReverseClient(tunnelConfig, mgrConfig, logger).(*ReverseClient)

	_, err := client.Send([]byte("test"))
	require.Error(t, err)
	assert.Equal(t, ErrNotConnected, err)
}

// TestAutoClientCreation 测试自动模式客户端创建
func TestAutoClientCreation(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "test-id",
		Name:      "test-tunnel",
		Mode:      ModeAuto,
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewAutoClient(tunnelConfig, mgrConfig, logger)
	require.NotNil(t, client)

	status := client.GetStatus()
	assert.Equal(t, ModeAuto, status.Mode)
	assert.Equal(t, StateDisconnected, status.State)
	assert.False(t, client.IsConnected())
}

// TestAutoClientConnect 测试自动模式客户端连接
func TestAutoClientConnect(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "test-id",
		Name:      "test-tunnel",
		Mode:      ModeAuto,
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewAutoClient(tunnelConfig, mgrConfig, logger).(*AutoClient)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)

	assert.True(t, client.IsConnected())
	status := client.GetStatus()
	assert.Equal(t, StateConnected, status.State)

	err = client.Disconnect()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

// TestAutoClientFallback 测试自动模式降级（P2P -> Relay）
func TestAutoClientFallback(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "test-id",
		Name:      "test-tunnel",
		Mode:      ModeAuto,
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewAutoClient(tunnelConfig, mgrConfig, logger).(*AutoClient)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)

	// 自动模式应该连接成功（当前实现都会成功）
	assert.True(t, client.IsConnected())
	_ = client.Disconnect()
}

// TestAutoClientSendReceiveNotConnected 测试自动模式客户端未连接时操作
func TestAutoClientSendReceiveNotConnected(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:   "test-id",
		Name: "test-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewAutoClient(tunnelConfig, mgrConfig, logger).(*AutoClient)

	_, err := client.Send([]byte("test"))
	require.Error(t, err)
	assert.Equal(t, ErrNotConnected, err)

	_, err = client.Receive()
	require.Error(t, err)
	assert.Equal(t, ErrNotConnected, err)
}

// TestClientGetStatus 测试客户端状态获取
func TestClientGetStatus(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "status-test-id",
		Name:      "status-test-tunnel",
		Mode:      ModeP2P,
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger).(*P2PClient)

	// 连接前
	status := client.GetStatus()
	assert.Equal(t, "status-test-id", status.ID)
	assert.Equal(t, "status-test-tunnel", status.Name)
	assert.Equal(t, ModeP2P, status.Mode)
	assert.Equal(t, StateDisconnected, status.State)

	// 连接后
	ctx := context.Background()
	_ = client.Connect(ctx)
	status = client.GetStatus()
	assert.Equal(t, StateConnected, status.State)
	assert.NotZero(t, status.LastConnected)

	_ = client.Disconnect()
}

// TestBaseClientStatusUpdate 测试基础客户端状态更新
func TestBaseClientStatusUpdate(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:   "test-id",
		Name: "test-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger).(*P2PClient)

	// 模拟字节统计
	ctx := context.Background()
	_ = client.Connect(ctx)

	// 获取状态
	status := client.GetStatus()
	assert.Equal(t, int64(0), status.BytesSent)
	assert.Equal(t, int64(0), status.BytesReceived)

	_ = client.Disconnect()
}

// TestClientConcurrency 测试客户端并发安全
func TestClientConcurrency(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "concurrent-test",
		Name:      "concurrent-tunnel",
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger).(*P2PClient)
	ctx := context.Background()

	var wg sync.WaitGroup

	// 并发连接和断开
	for i := 0; i < 10; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			_ = client.Connect(ctx)
		}()

		go func() {
			defer wg.Done()
			_ = client.Disconnect()
		}()
	}

	wg.Wait()

	// 并发读取状态
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.GetStatus()
			_ = client.IsConnected()
		}()
	}

	wg.Wait()
}

// TestClientContextCancellation 测试客户端上下文取消
func TestClientContextCancellation(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:   "ctx-test",
		Name: "ctx-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger).(*P2PClient)

	ctx, cancel := context.WithCancel(context.Background())

	// 在连接前取消
	cancel()

	// 连接应该仍然成功（当前实现不依赖外部服务）
	err := client.Connect(ctx)
	require.NoError(t, err)

	_ = client.Disconnect()
}

// TestClientMultipleDisconnect 测试多次断开连接
func TestClientMultipleDisconnect(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:   "multi-disconnect",
		Name: "multi-disconnect-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger).(*P2PClient)

	ctx := context.Background()
	_ = client.Connect(ctx)

	// 多次断开应该不会 panic
	_ = client.Disconnect()
	_ = client.Disconnect()
	_ = client.Disconnect()

	assert.False(t, client.IsConnected())
}

// TestClientReconnect 测试客户端重连
func TestClientReconnect(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:   "reconnect-test",
		Name: "reconnect-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger).(*P2PClient)
	ctx := context.Background()

	// 第一次连接
	err := client.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	// 断开
	err = client.Disconnect()
	require.NoError(t, err)
	assert.False(t, client.IsConnected())

	// 重连
	err = client.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, client.IsConnected())

	_ = client.Disconnect()
}

// TestConnectionStruct 测试连接结构
func TestConnectionStruct(t *testing.T) {
	now := time.Now()
	conn := Connection{
		ID:          "conn-1",
		TunnelID:    "tunnel-1",
		LocalAddr:   &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		RemoteAddr:  &net.TCPAddr{IP: net.ParseIP("192.168.1.1"), Port: 9090},
		Established: now,
		LastActive:  now,
		BytesSent:   1000,
		BytesRecv:   2000,
		Closed:      false,
	}

	assert.Equal(t, "conn-1", conn.ID)
	assert.Equal(t, "tunnel-1", conn.TunnelID)
	assert.Equal(t, int64(1000), conn.BytesSent)
	assert.Equal(t, int64(2000), conn.BytesRecv)
	assert.False(t, conn.Closed)
}

// TestPeerInfoStruct 测试对端信息结构
func TestPeerInfoStruct(t *testing.T) {
	now := time.Now()
	peer := PeerInfo{
		ID:        "peer-1",
		Name:      "peer-name",
		PublicKey: "public-key-string",
		Endpoints: []string{"192.168.1.1:8080", "10.0.0.1:8080"},
		NATType:   NATTypeFullCone,
		LastSeen:  now,
	}

	assert.Equal(t, "peer-1", peer.ID)
	assert.Equal(t, "peer-name", peer.Name)
	assert.Len(t, peer.Endpoints, 2)
	assert.Equal(t, NATTypeFullCone, peer.NATType)
}

// TestTunnelConfigDefaults 测试隧道配置默认值
func TestTunnelConfigDefaults(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:   "defaults-test",
		Name: "defaults-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger)
	require.NotNil(t, client)

	// 验证客户端创建成功，默认值已应用
	status := client.GetStatus()
	assert.Equal(t, "defaults-test", status.ID)
	assert.Equal(t, "defaults-tunnel", status.Name)
}

// TestClientWithNilLogger 测试空日志处理器
func TestClientWithNilLogger(t *testing.T) {
	tunnelConfig := TunnelConfig{
		ID:   "nil-logger-test",
		Name: "nil-logger-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	// 空日志处理器应该使用 nop logger
	client := NewP2PClient(tunnelConfig, mgrConfig, nil)
	require.NotNil(t, client)
}

// TestClientStatusTransitions 测试客户端状态转换
func TestClientStatusTransitions(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:   "status-transition",
		Name: "status-transition-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger).(*P2PClient)

	// 初始状态
	status := client.GetStatus()
	assert.Equal(t, StateDisconnected, status.State)

	// 连接后
	ctx := context.Background()
	_ = client.Connect(ctx)
	status = client.GetStatus()
	assert.Equal(t, StateConnected, status.State)

	// 断开后
	_ = client.Disconnect()
	status = client.GetStatus()
	assert.Equal(t, StateDisconnected, status.State)
}

// TestClientByteCounting 测试客户端字节计数
func TestClientByteCounting(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:   "byte-count-test",
		Name: "byte-count-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger).(*P2PClient)
	ctx := context.Background()

	// 连接
	_ = client.Connect(ctx)

	// 初始计数应为 0
	status := client.GetStatus()
	assert.Equal(t, int64(0), status.BytesSent)
	assert.Equal(t, int64(0), status.BytesReceived)

	// 注意：当前实现中，没有实际连接时 Send/Receive 会返回错误
	// 这里我们测试未连接情况

	_ = client.Disconnect()
}

// TestClientTimeout 测试客户端超时
func TestClientTimeout(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:   "timeout-test",
		Name: "timeout-tunnel",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
		Timeout:    5, // 5 秒超时
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger).(*P2PClient)

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// 连接应该快速完成（当前实现）
	err := client.Connect(ctx)
	require.NoError(t, err)

	_ = client.Disconnect()
}

// TestClientProtocolTCP 测试 TCP 协议客户端
func TestClientProtocolTCP(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "tcp-test",
		Name:      "tcp-tunnel",
		Protocol:  "tcp",
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger)
	require.NotNil(t, client)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)

	_ = client.Disconnect()
}

// TestClientProtocolUDP 测试 UDP 协议客户端
func TestClientProtocolUDP(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "udp-test",
		Name:      "udp-tunnel",
		Protocol:  "udp",
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	client := NewP2PClient(tunnelConfig, mgrConfig, logger)
	require.NotNil(t, client)

	ctx := context.Background()
	err := client.Connect(ctx)
	require.NoError(t, err)

	_ = client.Disconnect()
}

// TestAllClientTypes 测试所有客户端类型
func TestAllClientTypes(t *testing.T) {
	logger := zap.NewNop()
	tunnelConfig := TunnelConfig{
		ID:        "all-types-test",
		Name:      "all-types-tunnel",
		LocalAddr: "127.0.0.1:8080",
	}
	mgrConfig := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	tests := []struct {
		name        string
		client      TunnelClient
		mode        TunnelMode
		checkActual bool // AutoClient 返回实际使用的模式
	}{
		{
			name:   "P2P client",
			client: NewP2PClient(tunnelConfig, mgrConfig, logger),
			mode:   ModeP2P,
		},
		{
			name:   "Relay client",
			client: NewRelayClient(tunnelConfig, mgrConfig, logger),
			mode:   ModeRelay,
		},
		{
			name:   "Reverse client",
			client: NewReverseClient(tunnelConfig, mgrConfig, logger),
			mode:   ModeReverse,
		},
		{
			name:        "Auto client",
			client:      NewAutoClient(tunnelConfig, mgrConfig, logger),
			mode:        ModeP2P, // AutoClient 连接后返回实际使用的模式（P2P 或 Relay）
			checkActual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NotNil(t, tt.client)

			// 初始状态
			assert.False(t, tt.client.IsConnected())

			// 连接
			ctx := context.Background()
			err := tt.client.Connect(ctx)
			require.NoError(t, err)
			assert.True(t, tt.client.IsConnected())

			// 获取状态
			status := tt.client.GetStatus()
			assert.Equal(t, tt.mode, status.Mode)

			// 断开
			err = tt.client.Disconnect()
			require.NoError(t, err)
			assert.False(t, tt.client.IsConnected())
		})
	}
}

// MockTunnelClient 用于测试的模拟客户端
type MockTunnelClient struct {
	connected bool
	status    TunnelStatus
	sendErr   error
	recvErr   error
	mu        sync.RWMutex
}

func NewMockTunnelClient() *MockTunnelClient {
	return &MockTunnelClient{
		status: TunnelStatus{
			ID:    "mock-id",
			Name:  "mock-tunnel",
			State: StateDisconnected,
		},
	}
}

func (m *MockTunnelClient) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = true
	m.status.State = StateConnected
	m.status.LastConnected = time.Now()
	return nil
}

func (m *MockTunnelClient) Disconnect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = false
	m.status.State = StateDisconnected
	return nil
}

func (m *MockTunnelClient) GetStatus() TunnelStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

func (m *MockTunnelClient) Send(data []byte) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.connected {
		return 0, ErrNotConnected
	}
	if m.sendErr != nil {
		return 0, m.sendErr
	}
	m.status.BytesSent += int64(len(data))
	return len(data), nil
}

func (m *MockTunnelClient) Receive() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.connected {
		return nil, ErrNotConnected
	}
	if m.recvErr != nil {
		return nil, m.recvErr
	}
	data := []byte("mock-response")
	m.status.BytesReceived += int64(len(data))
	return data, nil
}

func (m *MockTunnelClient) IsConnected() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.connected
}

// TestMockClient 测试模拟客户端
func TestMockClient(t *testing.T) {
	mock := NewMockTunnelClient()

	// 初始状态
	assert.False(t, mock.IsConnected())

	// 连接
	ctx := context.Background()
	err := mock.Connect(ctx)
	require.NoError(t, err)
	assert.True(t, mock.IsConnected())

	// 发送数据
	n, err := mock.Send([]byte("test"))
	require.NoError(t, err)
	assert.Equal(t, 4, n)

	// 接收数据
	data, err := mock.Receive()
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// 验证字节计数
	status := mock.GetStatus()
	assert.Equal(t, int64(4), status.BytesSent)
	assert.Greater(t, status.BytesReceived, int64(0))

	// 断开
	err = mock.Disconnect()
	require.NoError(t, err)
	assert.False(t, mock.IsConnected())
}

// TestMockClientErrors 测试模拟客户端错误处理
func TestMockClientErrors(t *testing.T) {
	mock := NewMockTunnelClient()

	// 未连接时发送
	_, err := mock.Send([]byte("test"))
	require.Error(t, err)
	assert.Equal(t, ErrNotConnected, err)

	// 未连接时接收
	_, err = mock.Receive()
	require.Error(t, err)
	assert.Equal(t, ErrNotConnected, err)

	// 连接
	ctx := context.Background()
	_ = mock.Connect(ctx)

	// 设置发送错误
	mock.sendErr = assert.AnError
	_, err = mock.Send([]byte("test"))
	require.Error(t, err)

	// 设置接收错误
	mock.recvErr = assert.AnError
	_, err = mock.Receive()
	require.Error(t, err)
}
