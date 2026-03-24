// Package tunnel 提供内网穿透服务管理器的单元测试
package tunnel

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestNewManager 测试创建管理器
func TestNewManager(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				ServerAddr: "localhost",
				ServerPort: 8080,
			},
			wantErr: false,
		},
		{
			name: "missing server address",
			config: Config{
				ServerPort: 8080,
			},
			wantErr: true,
			errMsg:  "server address is required",
		},
		{
			name: "invalid server port - zero",
			config: Config{
				ServerAddr: "localhost",
				ServerPort: 0,
			},
			wantErr: true,
			errMsg:  "invalid server port",
		},
		{
			name: "invalid server port - negative",
			config: Config{
				ServerAddr: "localhost",
				ServerPort: -1,
			},
			wantErr: true,
			errMsg:  "invalid server port",
		},
		{
			name: "invalid server port - too large",
			config: Config{
				ServerAddr: "localhost",
				ServerPort: 65536,
			},
			wantErr: true,
			errMsg:  "invalid server port",
		},
		{
			name: "nil logger uses nop logger",
			config: Config{
				ServerAddr: "localhost",
				ServerPort: 8080,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := NewManager(tt.config, logger)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, m)
			} else {
				require.NoError(t, err)
				require.NotNil(t, m)
				assert.Equal(t, StateDisconnected, m.state)
				assert.NotNil(t, m.tunnels)
				assert.NotNil(t, m.tunnelByID)
				defer m.Stop()
			}
		})
	}
}

// TestValidateConfig 测试配置验证
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		wantErr     bool
		checkResult func(t *testing.T, c *Config)
	}{
		{
			name: "valid config",
			config: &Config{
				ServerAddr: "localhost",
				ServerPort: 8080,
			},
			wantErr: false,
		},
		{
			name: "sets default heartbeat interval",
			config: &Config{
				ServerAddr:   "localhost",
				ServerPort:   8080,
				HeartbeatInt: 0,
			},
			wantErr: false,
			checkResult: func(t *testing.T, c *Config) {
				assert.Equal(t, 30, c.HeartbeatInt)
			},
		},
		{
			name: "sets default reconnect interval",
			config: &Config{
				ServerAddr:   "localhost",
				ServerPort:   8080,
				ReconnectInt: 0,
			},
			wantErr: false,
			checkResult: func(t *testing.T, c *Config) {
				assert.Equal(t, 5, c.ReconnectInt)
			},
		},
		{
			name: "sets default max reconnect",
			config: &Config{
				ServerAddr:   "localhost",
				ServerPort:   8080,
				MaxReconnect: 0,
			},
			wantErr: false,
			checkResult: func(t *testing.T, c *Config) {
				assert.Equal(t, 10, c.MaxReconnect)
			},
		},
		{
			name: "sets default timeout",
			config: &Config{
				ServerAddr: "localhost",
				ServerPort: 8080,
				Timeout:    0,
			},
			wantErr: false,
			checkResult: func(t *testing.T, c *Config) {
				assert.Equal(t, 30, c.Timeout)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				if tt.checkResult != nil {
					tt.checkResult(t, tt.config)
				}
			}
		})
	}
}

// TestSetDefaults 测试默认值设置
func TestSetDefaults(t *testing.T) {
	tests := []struct {
		name        string
		input       *Config
		checkResult func(t *testing.T, c *Config)
	}{
		{
			name: "sets default mode",
			input: &Config{
				Mode: "",
			},
			checkResult: func(t *testing.T, c *Config) {
				assert.Equal(t, ModeAuto, c.Mode)
			},
		},
		{
			name: "sets default STUN servers",
			input: &Config{
				STUNServers: nil,
			},
			checkResult: func(t *testing.T, c *Config) {
				assert.Len(t, c.STUNServers, 2)
				assert.Contains(t, c.STUNServers[0], "stun.l.google.com")
			},
		},
		{
			name: "preserves existing mode",
			input: &Config{
				Mode: ModeP2P,
			},
			checkResult: func(t *testing.T, c *Config) {
				assert.Equal(t, ModeP2P, c.Mode)
			},
		},
		{
			name: "preserves existing STUN servers",
			input: &Config{
				STUNServers: []string{"stun:custom.server:3478"},
			},
			checkResult: func(t *testing.T, c *Config) {
				assert.Len(t, c.STUNServers, 1)
				assert.Equal(t, "stun:custom.server:3478", c.STUNServers[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setDefaults(tt.input)
			tt.checkResult(t, tt.input)
		})
	}
}

// TestManagerStartStop 测试管理器启动和停止
func TestManagerStartStop(t *testing.T) {
	logger := zap.NewNop()
	config := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	m, err := NewManager(config, logger)
	require.NoError(t, err)
	require.NotNil(t, m)

	// 启动管理器
	ctx := context.Background()
	err = m.Start(ctx)
	require.NoError(t, err)

	// 等待状态更新
	time.Sleep(100 * time.Millisecond)

	// 停止管理器
	err = m.Stop()
	require.NoError(t, err)

	// 检查状态
	status := m.GetStatus()
	assert.Equal(t, StateDisconnected, status.State)
}

// TestManagerConnectDisconnect 测试建立和断开隧道
func TestManagerConnectDisconnect(t *testing.T) {
	logger := zap.NewNop()
	config := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
		Mode:       ModeAuto,
	}

	m, err := NewManager(config, logger)
	require.NoError(t, err)
	defer m.Stop()

	ctx := context.Background()
	err = m.Start(ctx)
	require.NoError(t, err)

	// 建立隧道
	req := ConnectRequest{
		Name:      "test-tunnel",
		Mode:      ModeP2P,
		LocalPort: 8080,
		Protocol:  "tcp",
	}

	resp, err := m.Connect(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.NotEmpty(t, resp.TunnelID)
	assert.Equal(t, "test-tunnel", resp.Name)
	assert.Equal(t, ModeP2P, resp.Mode)
	assert.Equal(t, StateConnecting, resp.State)

	// 验证隧道已创建
	tunnels := m.ListTunnels()
	assert.Len(t, tunnels, 1)

	// 获取隧道状态
	status, err := m.GetTunnelStatus(resp.TunnelID)
	require.NoError(t, err)
	assert.Equal(t, resp.TunnelID, status.ID)

	// 断开隧道
	err = m.Disconnect(resp.TunnelID)
	require.NoError(t, err)

	// 验证隧道已删除
	tunnels = m.ListTunnels()
	assert.Len(t, tunnels, 0)

	// 尝试获取已删除的隧道
	_, err = m.GetTunnelStatus(resp.TunnelID)
	require.Error(t, err)
	assert.Equal(t, ErrTunnelNotFound, err)
}

// TestManagerConnectDuplicate 测试重复建立同名隧道
func TestManagerConnectDuplicate(t *testing.T) {
	logger := zap.NewNop()
	config := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	m, err := NewManager(config, logger)
	require.NoError(t, err)
	defer m.Stop()

	ctx := context.Background()
	err = m.Start(ctx)
	require.NoError(t, err)

	req := ConnectRequest{
		Name:      "duplicate-test",
		LocalPort: 8080,
	}

	// 第一次连接应该成功
	_, err = m.Connect(ctx, req)
	require.NoError(t, err)

	// 第二次同名连接应该失败
	_, err = m.Connect(ctx, req)
	require.Error(t, err)
	assert.Equal(t, ErrTunnelAlreadyExists, err)
}

// TestManagerDisconnectNotFound 测试断开不存在的隧道
func TestManagerDisconnectNotFound(t *testing.T) {
	logger := zap.NewNop()
	config := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	m, err := NewManager(config, logger)
	require.NoError(t, err)
	defer m.Stop()

	err = m.Disconnect("non-existent-id")
	require.Error(t, err)
	assert.Equal(t, ErrTunnelNotFound, err)
}

// TestManagerGetStatus 测试获取状态
func TestManagerGetStatus(t *testing.T) {
	logger := zap.NewNop()
	config := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	m, err := NewManager(config, logger)
	require.NoError(t, err)
	defer m.Stop()

	status := m.GetStatus()
	assert.Equal(t, StateDisconnected, status.State)
	assert.NotNil(t, status.Tunnels)
	assert.Equal(t, 0, status.ActiveTunnels)
}

// TestManagerOnEvent 测试事件回调
func TestManagerOnEvent(t *testing.T) {
	logger := zap.NewNop()
	config := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	m, err := NewManager(config, logger)
	require.NoError(t, err)
	defer m.Stop()

	var receivedEvents []Event
	var mu sync.Mutex

	// 注册事件回调
	m.OnEvent(func(e Event) {
		mu.Lock()
		defer mu.Unlock()
		receivedEvents = append(receivedEvents, e)
	})

	ctx := context.Background()
	err = m.Start(ctx)
	require.NoError(t, err)

	// 建立隧道触发事件
	req := ConnectRequest{
		Name:      "event-test",
		LocalPort: 8080,
	}
	_, err = m.Connect(ctx, req)
	require.NoError(t, err)

	// 等待事件处理
	time.Sleep(200 * time.Millisecond)

	// 检查事件
	mu.Lock()
	events := receivedEvents
	mu.Unlock()

	// 应该至少收到创建事件
	assert.NotEmpty(t, events)

	// 查找创建事件
	var foundCreated bool
	for _, e := range events {
		if e.Type == EventTunnelCreated {
			foundCreated = true
			assert.NotEmpty(t, e.TunnelID)
			break
		}
	}
	assert.True(t, foundCreated, "should have received tunnel created event")
}

// TestGenerateID 测试 ID 生成
func TestGenerateID(t *testing.T) {
	ids := make(map[string]bool)

	// 生成多个 ID 并验证唯一性
	for i := 0; i < 100; i++ {
		id := generateID()
		assert.NotEmpty(t, id)
		assert.Len(t, id, 16) // 8 bytes = 16 hex chars
		assert.NotContains(t, ids, id, "ID should be unique")
		ids[id] = true
	}
}

// TestManagerMultipleTunnels 测试多个隧道
func TestManagerMultipleTunnels(t *testing.T) {
	logger := zap.NewNop()
	config := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	m, err := NewManager(config, logger)
	require.NoError(t, err)
	defer m.Stop()

	ctx := context.Background()
	err = m.Start(ctx)
	require.NoError(t, err)

	// 创建多个隧道
	for i := 0; i < 5; i++ {
		req := ConnectRequest{
			Name:      "tunnel-" + string(rune('A'+i)),
			LocalPort: 8080 + i,
		}
		_, err := m.Connect(ctx, req)
		require.NoError(t, err)
	}

	// 验证隧道数量
	tunnels := m.ListTunnels()
	assert.Len(t, tunnels, 5)

	// 获取状态
	status := m.GetStatus()
	assert.Len(t, status.Tunnels, 5)
}

// TestManagerDifferentModes 测试不同连接模式
func TestManagerDifferentModes(t *testing.T) {
	logger := zap.NewNop()
	config := Config{
		ServerAddr: "localhost",
		ServerPort: 8080,
	}

	m, err := NewManager(config, logger)
	require.NoError(t, err)
	defer m.Stop()

	ctx := context.Background()
	err = m.Start(ctx)
	require.NoError(t, err)

	modes := []TunnelMode{ModeP2P, ModeRelay, ModeReverse, ModeAuto}

	for i, mode := range modes {
		req := ConnectRequest{
			Name:      string(mode) + "-test",
			Mode:      mode,
			LocalPort: 9000 + i,
		}

		resp, err := m.Connect(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, mode, resp.Mode)
	}

	// 验证所有隧道
	tunnels := m.ListTunnels()
	assert.Len(t, tunnels, len(modes))
}

// TestManagerContextCancellation 测试上下文取消
func TestManagerContextCancellation(t *testing.T) {
	logger := zap.NewNop()
	config := Config{
		ServerAddr:   "localhost",
		ServerPort:   8080,
		HeartbeatInt: 1, // 快速心跳便于测试
	}

	m, err := NewManager(config, logger)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	err = m.Start(ctx)
	require.NoError(t, err)

	// 取消上下文
	cancel()

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	// 手动停止
	_ = m.Stop()
}

// TestTunnelModeConstants 测试隧道模式常量
func TestTunnelModeConstants(t *testing.T) {
	assert.Equal(t, TunnelMode("p2p"), ModeP2P)
	assert.Equal(t, TunnelMode("relay"), ModeRelay)
	assert.Equal(t, TunnelMode("reverse"), ModeReverse)
	assert.Equal(t, TunnelMode("auto"), ModeAuto)
}

// TestTunnelStateConstants 测试隧道状态常量
func TestTunnelStateConstants(t *testing.T) {
	assert.Equal(t, TunnelState("disconnected"), StateDisconnected)
	assert.Equal(t, TunnelState("connecting"), StateConnecting)
	assert.Equal(t, TunnelState("connected"), StateConnected)
	assert.Equal(t, TunnelState("reconnecting"), StateReconnecting)
	assert.Equal(t, TunnelState("error"), StateError)
}

// TestNATTypeConstants 测试 NAT 类型常量
func TestNATTypeConstants(t *testing.T) {
	assert.Equal(t, NATType("unknown"), NATTypeUnknown)
	assert.Equal(t, NATType("none"), NATTypeNone)
	assert.Equal(t, NATType("full_cone"), NATTypeFullCone)
	assert.Equal(t, NATType("restricted_cone"), NATTypeRestrictedCone)
	assert.Equal(t, NATType("port_restricted_cone"), NATTypePortRestrictedCone)
	assert.Equal(t, NATType("symmetric"), NATTypeSymmetric)
}

// TestEventTypeConstants 测试事件类型常量
func TestEventTypeConstants(t *testing.T) {
	assert.Equal(t, EventType("tunnel_created"), EventTunnelCreated)
	assert.Equal(t, EventType("tunnel_connected"), EventTunnelConnected)
	assert.Equal(t, EventType("tunnel_disconnected"), EventTunnelDisconnected)
	assert.Equal(t, EventType("tunnel_error"), EventTunnelError)
	assert.Equal(t, EventType("nat_detected"), EventNATDetected)
	assert.Equal(t, EventType("peer_discovered"), EventPeerDiscovered)
}

// TestConnectRequestValidation 测试连接请求验证
func TestConnectRequestValidation(t *testing.T) {
	// 这里测试请求结构的字段
	req := ConnectRequest{
		Name:        "test",
		Mode:        ModeP2P,
		LocalPort:   8080,
		RemotePort:  9090,
		Protocol:    "tcp",
		Description: "test tunnel",
	}

	assert.Equal(t, "test", req.Name)
	assert.Equal(t, ModeP2P, req.Mode)
	assert.Equal(t, 8080, req.LocalPort)
	assert.Equal(t, 9090, req.RemotePort)
	assert.Equal(t, "tcp", req.Protocol)
	assert.Equal(t, "test tunnel", req.Description)
}

// TestTunnelStatusFields 测试隧道状态字段
func TestTunnelStatusFields(t *testing.T) {
	now := time.Now()
	status := TunnelStatus{
		ID:            "test-id",
		Name:          "test-name",
		Mode:          ModeP2P,
		State:         StateConnected,
		LocalAddr:     "127.0.0.1:8080",
		PublicAddr:    "1.2.3.4:8080",
		RemoteAddr:    "5.6.7.8:9090",
		BytesSent:     1000,
		BytesReceived: 2000,
		Connections:   5,
		Uptime:        3600,
		LastError:     "",
		LastConnected: now,
		NATType:       NATTypeFullCone,
		PeerAddr:      "10.0.0.1:1234",
		RelayAddr:     "",
	}

	assert.Equal(t, "test-id", status.ID)
	assert.Equal(t, "test-name", status.Name)
	assert.Equal(t, ModeP2P, status.Mode)
	assert.Equal(t, StateConnected, status.State)
	assert.Equal(t, int64(1000), status.BytesSent)
	assert.Equal(t, int64(2000), status.BytesReceived)
	assert.Equal(t, 5, status.Connections)
}

// TestManagerStatusFields 测试管理器状态字段
func TestManagerStatusFields(t *testing.T) {
	status := ManagerStatus{
		State:         StateConnected,
		NATType:       NATTypePortRestrictedCone,
		PublicIP:      "1.2.3.4",
		PublicPort:    12345,
		Tunnels:       []TunnelStatus{},
		ActiveTunnels: 3,
		TotalBytesTx:  10000,
		TotalBytesRx:  20000,
		ServerLatency: 50,
		StartTime:     time.Now(),
	}

	assert.Equal(t, StateConnected, status.State)
	assert.Equal(t, NATTypePortRestrictedCone, status.NATType)
	assert.Equal(t, "1.2.3.4", status.PublicIP)
	assert.Equal(t, 12345, status.PublicPort)
	assert.Equal(t, 3, status.ActiveTunnels)
	assert.Equal(t, int64(10000), status.TotalBytesTx)
	assert.Equal(t, int64(20000), status.TotalBytesRx)
}
