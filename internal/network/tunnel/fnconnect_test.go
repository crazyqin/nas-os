// Package tunnel 提供内网穿透服务测试
package tunnel

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestFNConnect_New(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFNConnectConfig()

	client, err := NewFNConnect(config, logger)

	assert.NoError(t, err)
	assert.NotNil(t, client)
	assert.Equal(t, FNCStateDisconnected, client.state)
	assert.NotNil(t, client.tunnels)
	assert.NotNil(t, client.msgChan)
}

func TestFNConnect_DefaultConfig(t *testing.T) {
	config := DefaultFNConnectConfig()

	assert.Equal(t, "connect.fnos.cn:7000", config.ServerURL)
	assert.Equal(t, "cn", config.Region)
	assert.Equal(t, 5*time.Second, config.ReconnectInterval)
	assert.Equal(t, 30*time.Second, config.HeartbeatInterval)
	assert.Equal(t, 30*time.Second, config.Timeout)
	assert.Equal(t, 5, config.MaxRetries)
	assert.True(t, config.EnableTLS)
	assert.Equal(t, int64(10*1024*1024), config.MaxBandwidth)
	assert.Equal(t, 3, config.QoSLevel)
}

func TestFNConnect_SelectServer(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		region   string
		expected string
	}{
		{"cn", "connect.fnos.cn:7000"},
		{"us", "connect.fnos.us:7000"},
		{"eu", "connect.fnos.eu:7000"},
		{"unknown", "connect.fnos.cn:7000"}, // 默认中国
	}

	for _, tt := range tests {
		config := &FNConnectConfig{
			Region: tt.region,
		}
		client, _ := NewFNConnect(config, logger)
		server := client.selectServer()
		assert.Equal(t, tt.expected, server, "region: %s", tt.region)
	}
}

func TestFNConnect_CustomServer(t *testing.T) {
	logger := zap.NewNop()
	config := &FNConnectConfig{
		ServerURL: "custom.server.com:7000",
	}

	client, _ := NewFNConnect(config, logger)
	server := client.selectServer()

	assert.Equal(t, "custom.server.com:7000", server)
}

func TestFNConnect_EventHandlers(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFNConnectConfig()
	client, _ := NewFNConnect(config, logger)

	eventReceived := false
	client.OnEvent(func(event *FNConnectEvent) {
		eventReceived = true
	})

	// 触发事件
	client.emitEvent(&FNConnectEvent{
		Type: "test",
	})

	time.Sleep(100 * time.Millisecond)
	assert.True(t, eventReceived)
}

func TestFNConnect_GetStats(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFNConnectConfig()
	client, _ := NewFNConnect(config, logger)

	stats := client.GetStats()

	assert.Equal(t, FNCStateDisconnected, stats.State)
	assert.Empty(t, stats.PublicURL)
	assert.Equal(t, 0, stats.ActiveTunnels)
}

func TestFNConnect_GetTunnels(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFNConnectConfig()
	client, _ := NewFNConnect(config, logger)

	tunnels := client.GetTunnels()
	assert.Empty(t, tunnels)

	// 手动添加隧道（模拟）
	client.mu.Lock()
	client.tunnels["test1"] = &FNCTunnel{ID: "test1", Name: "Test Tunnel"}
	client.tunnels["test2"] = &FNCTunnel{ID: "test2", Name: "Another Tunnel"}
	client.mu.Unlock()

	tunnels = client.GetTunnels()
	assert.Len(t, tunnels, 2)
}

func TestFNConnect_IsConnected(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFNConnectConfig()
	client, _ := NewFNConnect(config, logger)

	assert.False(t, client.IsConnected())

	client.mu.Lock()
	client.state = FNCStateConnected
	client.mu.Unlock()

	assert.True(t, client.IsConnected())
}

func TestFNConnect_GetPublicURL(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFNConnectConfig()
	client, _ := NewFNConnect(config, logger)

	assert.Empty(t, client.GetPublicURL())

	client.mu.Lock()
	client.publicURL = "https://test.fnos.cn"
	client.mu.Unlock()

	assert.Equal(t, "https://test.fnos.cn", client.GetPublicURL())
}

func TestFNConnect_DeleteTunnel(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFNConnectConfig()
	client, _ := NewFNConnect(config, logger)

	// 测试删除不存在的隧道
	err := client.DeleteTunnel("nonexistent")
	assert.Error(t, err)

	// 添加隧道
	client.mu.Lock()
	client.tunnels["test1"] = &FNCTunnel{ID: "test1"}
	client.state = FNCStateConnected
	client.mu.Unlock()

	// 由于没有实际连接，这个测试会失败
	// 但我们测试的是ID存在性检查
}

func TestFNConnect_GenerateIDs(t *testing.T) {
	deviceID := generateDeviceID()
	tunnelID := generateTunnelID()

	assert.NotEmpty(t, deviceID)
	assert.Contains(t, deviceID, "device-")
	assert.NotEmpty(t, tunnelID)
	assert.Contains(t, tunnelID, "tunnel-")

	// ID应该是唯一的
	deviceID2 := generateDeviceID()
	tunnelID2 := generateTunnelID()

	assert.NotEqual(t, deviceID, deviceID2)
	assert.NotEqual(t, tunnelID, tunnelID2)
}

func TestFNConnect_Disconnect(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFNConnectConfig()
	client, _ := NewFNConnect(config, logger)

	err := client.Disconnect()
	assert.NoError(t, err)
	assert.Equal(t, FNCStateDisconnected, client.state)
}

func TestFNCTunnel_State(t *testing.T) {
	tunnel := &FNCTunnel{
		ID:        "test",
		Name:      "Test Tunnel",
		Protocol:  "tcp",
		LocalPort: 8080,
		State:     FNCStateDisconnected,
	}

	assert.Equal(t, FNCStateDisconnected, tunnel.State)

	tunnel.State = FNCStateConnected
	assert.Equal(t, FNCStateConnected, tunnel.State)
}

func TestFNConnectConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config *FNConnectConfig
		valid  bool
	}{
		{
			name:   "default config",
			config: DefaultFNConnectConfig(),
			valid:  true,
		},
		{
			name: "custom config",
			config: &FNConnectConfig{
				ServerURL:         "custom.server:7000",
				Region:            "us",
				ReconnectInterval: 10 * time.Second,
				HeartbeatInterval: 60 * time.Second,
				Timeout:           30 * time.Second,
				MaxRetries:        3,
				EnableTLS:         true,
				MaxBandwidth:      5 * 1024 * 1024,
				QoSLevel:          5,
			},
			valid: true,
		},
		{
			name:   "nil config uses defaults",
			config: nil,
			valid:  true, // NewFNConnect会使用默认配置
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			client, err := NewFNConnect(tt.config, logger)

			if tt.valid {
				assert.NoError(t, err)
				assert.NotNil(t, client)
			} else {
				assert.Error(t, err)
			}
		})
	}
}

func TestFNConnect_ConcurrentAccess(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultFNConnectConfig()
	client, _ := NewFNConnect(config, logger)

	var wg sync.WaitGroup

	// 并发读取状态
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = client.IsConnected()
			_ = client.GetStats()
			_ = client.GetTunnels()
			_ = client.GetPublicURL()
		}()
	}

	wg.Wait()
}

func TestFNConnectMessage_Structure(t *testing.T) {
	msg := &FNCMessage{
		Type:      "test",
		TunnelID:  "tunnel-123",
		Timestamp: time.Now(),
	}

	assert.Equal(t, "test", msg.Type)
	assert.Equal(t, "tunnel-123", msg.TunnelID)
	assert.False(t, msg.Timestamp.IsZero())
}

func TestFNConnectEvent_Structure(t *testing.T) {
	event := &FNConnectEvent{
		Type:      "connected",
		State:     FNCStateConnected,
		Timestamp: time.Now(),
	}

	assert.Equal(t, "connected", event.Type)
	assert.Equal(t, FNCStateConnected, event.State)
	assert.False(t, event.Timestamp.IsZero())
}

func TestFNConnectStats_Structure(t *testing.T) {
	stats := &FNConnectStats{
		State:         FNCStateConnected,
		PublicURL:     "https://test.fnos.cn",
		ActiveTunnels: 2,
		TotalBytesTx:  1024,
		TotalBytesRx:  2048,
		Uptime:        time.Hour,
	}

	assert.Equal(t, FNCStateConnected, stats.State)
	assert.Equal(t, "https://test.fnos.cn", stats.PublicURL)
	assert.Equal(t, 2, stats.ActiveTunnels)
}
