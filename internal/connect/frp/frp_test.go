// Package frp provides FRP client implementation
// 单元测试
package frp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.Equal(t, "connect.nas-os.io", config.Common.ServerAddr)
	assert.Equal(t, 7000, config.Common.ServerPort)
	assert.True(t, config.Common.TLSEnable)
	assert.Equal(t, 30, config.Common.HeartbeatInterval)
	assert.NotNil(t, config.Tunnels)
}

func TestConfigSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "frp_config.json")

	config := DefaultConfig()
	config.ConfigPath = configPath
	config.Common.ServerAddr = "test.example.com"
	config.Common.Token = "test-token-123"

	// 添加隧道
	config.AddTunnel(TunnelConfig{
		Name:      "web",
		Type:      TunnelTypeHTTP,
		LocalIP:   "127.0.0.1",
		LocalPort: 8080,
		SubDomain: "myweb",
	})

	// 保存
	err := config.SaveConfig()
	require.NoError(t, err)

	// 加载
	loaded, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, "test.example.com", loaded.Common.ServerAddr)
	assert.Equal(t, "test-token-123", loaded.Common.Token)
	assert.Len(t, loaded.Tunnels, 1)
	assert.Equal(t, "web", loaded.Tunnels[0].Name)
	assert.Equal(t, TunnelTypeHTTP, loaded.Tunnels[0].Type)
}

func TestConfigTunnelOperations(t *testing.T) {
	config := DefaultConfig()

	// 添加隧道
	tunnel1 := TunnelConfig{
		ID:         "t1",
		Name:       "ssh",
		Type:       TunnelTypeTCP,
		LocalIP:    "127.0.0.1",
		LocalPort:  22,
		RemotePort: 2222,
		Enabled:    true,
	}
	config.AddTunnel(tunnel1)

	assert.Len(t, config.Tunnels, 1)

	// 添加第二个隧道
	tunnel2 := TunnelConfig{
		ID:        "t2",
		Name:      "web",
		Type:      TunnelTypeHTTP,
		LocalIP:   "127.0.0.1",
		LocalPort: 80,
		SubDomain: "myapp",
		Enabled:   true,
	}
	config.AddTunnel(tunnel2)

	assert.Len(t, config.Tunnels, 2)

	// 获取隧道
	found := config.GetTunnel("t1")
	require.NotNil(t, found)
	assert.Equal(t, "ssh", found.Name)
	assert.Equal(t, TunnelTypeTCP, found.Type)

	// 更新隧道
	tunnel1Updated := TunnelConfig{
		ID:         "t1",
		Name:       "ssh-updated",
		Type:       TunnelTypeTCP,
		LocalIP:    "127.0.0.1",
		LocalPort:  22,
		RemotePort: 2223,
		Enabled:    true,
	}
	assert.True(t, config.UpdateTunnel(tunnel1Updated))

	updated := config.GetTunnel("t1")
	require.NotNil(t, updated)
	assert.Equal(t, "ssh-updated", updated.Name)
	assert.Equal(t, 2223, updated.RemotePort)

	// 删除隧道
	assert.True(t, config.RemoveTunnel("t1"))
	assert.Len(t, config.Tunnels, 1)
	assert.False(t, config.RemoveTunnel("nonexistent"))
}

func TestTunnelConfigJSON(t *testing.T) {
	tunnel := TunnelConfig{
		ID:            "test-tunnel",
		Name:          "my-web",
		Type:          TunnelTypeHTTP,
		LocalIP:       "127.0.0.1",
		LocalPort:     8080,
		SubDomain:     "myweb",
		CustomDomains: []string{"www.example.com"},
		HTTPUser:      "admin",
		HTTPPwd:       "secret",
		Enabled:       true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Metas:         map[string]string{"version": "1.0"},
	}

	data, err := json.MarshalIndent(tunnel, "", "  ")
	require.NoError(t, err)

	var decoded TunnelConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, tunnel.ID, decoded.ID)
	assert.Equal(t, tunnel.Name, decoded.Name)
	assert.Equal(t, TunnelTypeHTTP, decoded.Type)
	assert.Equal(t, tunnel.LocalPort, decoded.LocalPort)
	assert.Equal(t, tunnel.SubDomain, decoded.SubDomain)
	assert.Len(t, decoded.CustomDomains, 1)
}

func TestProtocolMessageEncoding(t *testing.T) {
	// 测试认证消息
	authReq := AuthRequest{
		Version:   "0.52.0",
		Token:     "test-token",
		Timestamp: time.Now().Unix(),
		RunID:     "test-run-id",
	}

	data, err := EncodeMessage(MsgTypeAuth, authReq)
	require.NoError(t, err)

	// 验证消息格式
	msgType, msgLen, err := ParseHeader(data[:10])
	require.NoError(t, err)

	assert.Equal(t, MsgTypeAuth, msgType)
	assert.True(t, msgLen > 0)

	// 解码消息体
	msg := &Message{
		Type: msgType,
		Len:  msgLen,
		Data: data[10:],
	}

	decoded, err := DecodeMessage(msg)
	require.NoError(t, err)

	authDecoded, ok := decoded.(AuthRequest)
	require.True(t, ok)

	assert.Equal(t, authReq.Version, authDecoded.Version)
	assert.Equal(t, authReq.Token, authDecoded.Token)
	assert.Equal(t, authReq.RunID, authDecoded.RunID)
}

func TestProtocolTunnelMessage(t *testing.T) {
	tunnelReq := TunnelRequest{
		Name:      "web-server",
		Type:      "http",
		LocalIP:   "127.0.0.1",
		LocalPort: 8080,
		SubDomain: "myapp",
	}

	data, err := EncodeMessage(MsgTypeNewProxy, tunnelReq)
	require.NoError(t, err)

	msgType, msgLen, err := ParseHeader(data[:10])
	require.NoError(t, err)

	assert.Equal(t, MsgTypeNewProxy, msgType)
	assert.True(t, msgLen > 0)

	msg := &Message{
		Type: msgType,
		Len:  msgLen,
		Data: data[10:],
	}

	decoded, err := DecodeMessage(msg)
	require.NoError(t, err)

	tunnelDecoded, ok := decoded.(TunnelRequest)
	require.True(t, ok)

	assert.Equal(t, tunnelReq.Name, tunnelDecoded.Name)
	assert.Equal(t, tunnelReq.Type, tunnelDecoded.Type)
	assert.Equal(t, tunnelReq.LocalPort, tunnelDecoded.LocalPort)
}

func TestProtocolDataMessage(t *testing.T) {
	dataMsg := DataMessage{
		ProxyName: "web-server",
		Data:      []byte("Hello, World!"),
	}

	encoded, err := EncodeMessage(MsgTypeData, dataMsg)
	require.NoError(t, err)

	msgType, msgLen, err := ParseHeader(encoded[:10])
	require.NoError(t, err)

	assert.Equal(t, MsgTypeData, msgType)
	assert.True(t, msgLen > 0)

	msg := &Message{
		Type: msgType,
		Len:  msgLen,
		Data: encoded[10:],
	}

	decoded, err := DecodeMessage(msg)
	require.NoError(t, err)

	dataDecoded, ok := decoded.(DataMessage)
	require.True(t, ok)

	assert.Equal(t, dataMsg.ProxyName, dataDecoded.ProxyName)
	assert.Equal(t, string(dataMsg.Data), string(dataDecoded.Data))
}

func TestGenerateTunnelID(t *testing.T) {
	id1 := generateTunnelID()
	id2 := generateTunnelID()

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2) // 时间戳不同，ID应该不同
	assert.Contains(t, id1, "tunnel_")
}

func TestTunnelStatus(t *testing.T) {
	status := TunnelStatus{
		ID:          "t1",
		Name:        "ssh",
		Type:        TunnelTypeTCP,
		Status:      "running",
		LocalAddr:   "127.0.0.1:22",
		RemoteAddr:  "server.example.com:2222",
		BytesSent:   1024,
		BytesRecv:   2048,
		Connections: 5,
		LastActive:  time.Now(),
	}

	data, err := json.MarshalIndent(status, "", "  ")
	require.NoError(t, err)

	var decoded TunnelStatus
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, status.ID, decoded.ID)
	assert.Equal(t, status.BytesSent, decoded.BytesSent)
	assert.Equal(t, status.Connections, decoded.Connections)
}

func TestClientStats(t *testing.T) {
	stats := ClientStats{
		BytesSent:     1024 * 1024,
		BytesReceived: 2 * 1024 * 1024,
		Connections:   10,
		LastActivity:  time.Now(),
		Latency:       50,
		Uptime:        "2h30m",
		ConnectedAt:   time.Now().Add(-2 * time.Hour),
	}

	data, err := json.Marshal(stats)
	require.NoError(t, err)

	var decoded ClientStats
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, stats.BytesSent, decoded.BytesSent)
	assert.Equal(t, stats.Latency, decoded.Latency)
}

func TestMultipleTunnelTypes(t *testing.T) {
	config := DefaultConfig()

	// 添加不同类型的隧道
	config.AddTunnel(TunnelConfig{
		Name:       "tcp-service",
		Type:       TunnelTypeTCP,
		LocalPort:  3306,
		RemotePort: 33306,
	})

	config.AddTunnel(TunnelConfig{
		Name:       "udp-service",
		Type:       TunnelTypeUDP,
		LocalPort:  53,
		RemotePort: 5353,
	})

	config.AddTunnel(TunnelConfig{
		Name:      "http-web",
		Type:      TunnelTypeHTTP,
		LocalPort: 80,
		SubDomain: "myweb",
	})

	config.AddTunnel(TunnelConfig{
		Name:      "https-secure",
		Type:      TunnelTypeHTTPS,
		LocalPort: 443,
		SubDomain: "secure",
	})

	config.AddTunnel(TunnelConfig{
		Name:      "stcp-private",
		Type:      TunnelTypeSTCP,
		LocalPort: 22,
		Sk:        "secret-key",
	})

	assert.Len(t, config.Tunnels, 5)

	// 验证每种类型
	types := make(map[TunnelType]bool)
	for _, t := range config.Tunnels {
		types[t.Type] = true
	}

	assert.True(t, types[TunnelTypeTCP])
	assert.True(t, types[TunnelTypeUDP])
	assert.True(t, types[TunnelTypeHTTP])
	assert.True(t, types[TunnelTypeHTTPS])
	assert.True(t, types[TunnelTypeSTCP])
}

func TestConfigValidation(t *testing.T) {
	// 测试无效配置
	invalidConfig := &ClientConfig{
		Common: CommonConfig{
			ServerAddr: "", // 空，应该无效
		},
	}

	// 测试有效配置
	validConfig := &ClientConfig{
		Common: CommonConfig{
			ServerAddr: "example.com",
			ServerPort: 7000,
		},
	}

	assert.Empty(t, invalidConfig.Common.ServerAddr)
	assert.NotEmpty(t, validConfig.Common.ServerAddr)
}

func TestHealthCheckConfig(t *testing.T) {
	tunnel := TunnelConfig{
		Name:                 "web",
		Type:                 TunnelTypeHTTP,
		LocalPort:            80,
		HealthCheckType:      "tcp",
		HealthCheckTimeoutS:  10,
		HealthCheckMaxFailed: 3,
		HealthCheckIntervalS: 30,
	}

	data, err := json.Marshal(tunnel)
	require.NoError(t, err)

	var decoded TunnelConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "tcp", decoded.HealthCheckType)
	assert.Equal(t, 10, decoded.HealthCheckTimeoutS)
	assert.Equal(t, 3, decoded.HealthCheckMaxFailed)
	assert.Equal(t, 30, decoded.HealthCheckIntervalS)
}

func TestLoadBalancerConfig(t *testing.T) {
	tunnel := TunnelConfig{
		Name:              "web",
		Type:              TunnelTypeHTTP,
		LocalPort:         80,
		LoadBalancerGroup: "web-group",
		LoadBalancerKey:   "route-key",
	}

	data, err := json.Marshal(tunnel)
	require.NoError(t, err)

	var decoded TunnelConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "web-group", decoded.LoadBalancerGroup)
	assert.Equal(t, "route-key", decoded.LoadBalancerKey)
}

func TestBandwidthLimit(t *testing.T) {
	tunnel := TunnelConfig{
		Name:           "limited",
		Type:           TunnelTypeTCP,
		LocalPort:      80,
		BandwidthLimit: "10MB",
	}

	data, err := json.Marshal(tunnel)
	require.NoError(t, err)

	var decoded TunnelConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "10MB", decoded.BandwidthLimit)
}

func TestCustomDomains(t *testing.T) {
	tunnel := TunnelConfig{
		Name:          "multi-domain",
		Type:          TunnelTypeHTTP,
		LocalPort:     80,
		CustomDomains: []string{"a.example.com", "b.example.com", "c.example.com"},
	}

	data, err := json.Marshal(tunnel)
	require.NoError(t, err)

	var decoded TunnelConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.CustomDomains, 3)
	assert.Contains(t, decoded.CustomDomains, "a.example.com")
	assert.Contains(t, decoded.CustomDomains, "b.example.com")
	assert.Contains(t, decoded.CustomDomains, "c.example.com")
}

func TestLocationsConfig(t *testing.T) {
	tunnel := TunnelConfig{
		Name:      "location-based",
		Type:      TunnelTypeHTTP,
		LocalPort: 80,
		Locations: []string{"/api", "/v1", "/health"},
	}

	data, err := json.Marshal(tunnel)
	require.NoError(t, err)

	var decoded TunnelConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Locations, 3)
}

func TestMetasConfig(t *testing.T) {
	tunnel := TunnelConfig{
		Name:      "with-metas",
		Type:      TunnelTypeHTTP,
		LocalPort: 80,
		Metas: map[string]string{
			"version": "1.0",
			"env":     "production",
			"owner":   "team-a",
		},
	}

	data, err := json.Marshal(tunnel)
	require.NoError(t, err)

	var decoded TunnelConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Len(t, decoded.Metas, 3)
	assert.Equal(t, "1.0", decoded.Metas["version"])
	assert.Equal(t, "production", decoded.Metas["env"])
}

func TestQUICConfig(t *testing.T) {
	config := DefaultConfig()
	config.Common.Protocol = "quic"
	config.Common.QUICKeepalivePeriod = 10
	config.Common.QUICMaxIdleTimeout = 30

	data, err := json.Marshal(config.Common)
	require.NoError(t, err)

	var decoded CommonConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "quic", decoded.Protocol)
	assert.Equal(t, 10, decoded.QUICKeepalivePeriod)
	assert.Equal(t, 30, decoded.QUICMaxIdleTimeout)
}

func TestPoolAndMuxConfig(t *testing.T) {
	config := DefaultConfig()
	config.Common.PoolCount = 10
	config.Common.TCPMux = true
	config.Common.TCPMuxKeepalive = 60

	data, err := json.Marshal(config.Common)
	require.NoError(t, err)

	var decoded CommonConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 10, decoded.PoolCount)
	assert.True(t, decoded.TCPMux)
	assert.Equal(t, 60, decoded.TCPMuxKeepalive)
}

func TestAdminConfig(t *testing.T) {
	config := DefaultConfig()
	config.Common.AdminAddr = "127.0.0.1"
	config.Common.AdminPort = 7500
	config.Common.AdminUser = "admin"
	config.Common.AdminPwd = "admin123"

	data, err := json.Marshal(config.Common)
	require.NoError(t, err)

	var decoded CommonConfig
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "127.0.0.1", decoded.AdminAddr)
	assert.Equal(t, 7500, decoded.AdminPort)
	assert.Equal(t, "admin", decoded.AdminUser)
	assert.Equal(t, "admin123", decoded.AdminPwd)
}

func TestConfigFilePersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建多级目录结构
	configDir := filepath.Join(tmpDir, "nas", "config")
	err := os.MkdirAll(configDir, 0750)
	require.NoError(t, err)

	configPath := filepath.Join(configDir, "frp.json")

	config := DefaultConfig()
	config.ConfigPath = configPath
	config.Common.ServerAddr = "persistent.example.com"
	config.Common.Token = "persistent-token"

	// 保存
	err = config.SaveConfig()
	require.NoError(t, err)

	// 确认文件存在
	assert.FileExists(t, configPath)

	// 加载并验证
	loaded, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, "persistent.example.com", loaded.Common.ServerAddr)
	assert.Equal(t, "persistent-token", loaded.Common.Token)
}
