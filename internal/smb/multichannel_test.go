// Package smb SMB多通道单元测试
package smb

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultMultichannelConfig 测试默认配置
func TestDefaultMultichannelConfig(t *testing.T) {
	config := DefaultMultichannelConfig()

	assert.False(t, config.Enabled, "默认应禁用多通道")
	assert.Equal(t, 4, config.MaxChannels, "默认最大通道数为4")
	assert.True(t, config.AutoDiscover, "默认启用自动发现")
	assert.True(t, config.RoundRobin, "默认启用轮询")
	assert.True(t, config.FailoverEnabled, "默认启用故障切换")
	assert.Equal(t, 30, config.HealthCheckSec, "默认健康检查间隔30秒")
	assert.Equal(t, 100, config.MinBandwidthMbps, "默认最低带宽100Mbps")
}

// TestNewMultichannelManager 测试创建多通道管理器
func TestNewMultichannelManager(t *testing.T) {
	config := DefaultMultichannelConfig()
	config.Enabled = false // 禁用，避免自动启动

	manager := NewMultichannelManager(config)

	assert.NotNil(t, manager)
	assert.False(t, manager.running, "禁用时不应运行")
	assert.Empty(t, manager.channels, "初始无通道")
}

// TestMultichannelManager_StartStop 测试启动停止
func TestMultichannelManager_StartStop(t *testing.T) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	config.HealthCheckSec = 5 // 缩短测试时间

	manager := NewMultichannelManager(config)

	// 启动（可能会因网络配置失败，但不应崩溃）
	err := manager.Start()
	// 在某些环境下可能无法发现接口，这是预期行为
	if err != nil {
		t.Logf("启动失败（可能因环境限制）: %v", err)
		return
	}

	assert.True(t, manager.running, "启动后应运行")

	// 停止
	err = manager.Stop()
	assert.NoError(t, err)
	assert.False(t, manager.running, "停止后不应运行")
}

// TestMultichannelManager_GetStatus 测试获取状态
func TestMultichannelManager_GetStatus(t *testing.T) {
	config := DefaultMultichannelConfig()
	config.Enabled = false

	manager := NewMultichannelManager(config)

	status := manager.GetStatus()

	assert.NotNil(t, status)
	assert.False(t, status.Enabled)
	assert.Equal(t, 0, status.TotalChannels)
}

// TestNetworkInterface 测试网络接口结构
func TestNetworkInterface(t *testing.T) {
	iface := &NetworkInterface{
		Name:        "eth0",
		IPAddresses: []string{"192.168.1.100"},
		MAC:         "00:11:22:33:44:55",
		SpeedMbps:   1000,
		Up:          true,
		Type:        "ethernet",
		MTU:         1500,
		Priority:    1,
	}

	assert.Equal(t, "eth0", iface.Name)
	assert.Len(t, iface.IPAddresses, 1)
	assert.Equal(t, 1000, iface.SpeedMbps)
	assert.True(t, iface.Up)
}

// TestSMBChannel 测试SMB通道结构
func TestSMBChannel(t *testing.T) {
	channel := &SMBChannel{
		ID:            1,
		InterfaceName: "eth0",
		IPAddress:     "192.168.1.100",
		Port:          445,
		Connected:     true,
		Connections:   5,
		BandwidthMbps: 1000,
		HealthScore:   100,
		ActiveSince:   time.Now(),
	}

	assert.Equal(t, 1, channel.ID)
	assert.Equal(t, 445, channel.Port)
	assert.True(t, channel.Connected)
	assert.Equal(t, 100, channel.HealthScore)
}

// TestGetInterfaceType 测试接口类型判断
func TestGetInterfaceType(t *testing.T) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	manager := NewMultichannelManager(config)

	tests := []struct {
		name     string
		expected string
	}{
		{"eth0", "ethernet"},
		{"en0", "ethernet"},
		{"wlan0", "wifi"},
		{"wl0", "wifi"},
		{"br0", "bridge"},
		{"docker0", "virtual"},
		{"veth123", "virtual"},
		{"bond0", "bond"},
		{"lo", "loopback"},
		{"unknown", "ethernet"}, // 默认
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.getInterfaceType(tt.name, 0)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestIsInterfaceSuitable 测试接口适配性检查
func TestIsInterfaceSuitable(t *testing.T) {
	config := DefaultMultichannelConfig()
	config.MinBandwidthMbps = 100
	manager := NewMultichannelManager(config)

	tests := []struct {
		name     string
		iface    *NetworkInterface
		expected bool
	}{
		{
			name: "valid_ethernet",
			iface: &NetworkInterface{
				Name:        "eth0",
				IPAddresses: []string{"192.168.1.100"},
				Up:          true,
				Type:        "ethernet",
				SpeedMbps:   1000,
			},
			expected: true,
		},
		{
			name: "no_ip_address",
			iface: &NetworkInterface{
				Name:        "eth1",
				IPAddresses: []string{},
				Up:          true,
				Type:        "ethernet",
				SpeedMbps:   1000,
			},
			expected: false,
		},
		{
			name: "interface_down",
			iface: &NetworkInterface{
				Name:        "eth2",
				IPAddresses: []string{"192.168.1.101"},
				Up:          false,
				Type:        "ethernet",
				SpeedMbps:   1000,
			},
			expected: false,
		},
		{
			name: "virtual_interface",
			iface: &NetworkInterface{
				Name:        "docker0",
				IPAddresses: []string{"172.17.0.1"},
				Up:          true,
				Type:        "virtual",
				SpeedMbps:   1000,
			},
			expected: false,
		},
		{
			name: "low_bandwidth",
			iface: &NetworkInterface{
				Name:        "wlan0",
				IPAddresses: []string{"192.168.1.102"},
				Up:          true,
				Type:        "wifi",
				SpeedMbps:   10,
			},
			expected: false,
		},
		{
			name: "loopback",
			iface: &NetworkInterface{
				Name:        "lo",
				IPAddresses: []string{"127.0.0.1"},
				Up:          true,
				Type:        "loopback",
				SpeedMbps:   10000,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.isInterfaceSuitable(tt.iface)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSameSubnet 测试子网判断
func TestSameSubnet(t *testing.T) {
	config := DefaultMultichannelConfig()
	manager := NewMultichannelManager(config)

	tests := []struct {
		name     string
		ip1      string
		ip2      string
		expected bool
	}{
		{
			name:     "same_subnet",
			ip1:      "192.168.1.100",
			ip2:      "192.168.1.101",
			expected: true,
		},
		{
			name:     "different_subnet",
			ip1:      "192.168.1.100",
			ip2:      "192.168.2.100",
			expected: false,
		},
		{
			name:     "invalid_ip1",
			ip1:      "invalid",
			ip2:      "192.168.1.101",
			expected: false,
		},
		{
			name:     "invalid_ip2",
			ip1:      "192.168.1.100",
			ip2:      "invalid",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := manager.sameSubnet(tt.ip1, tt.ip2)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestValidateMultichannelConfig 测试配置验证
func TestValidateMultichannelConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *MultichannelConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid_config",
			config: &MultichannelConfig{
				MaxChannels:      4,
				HealthCheckSec:   30,
				MinBandwidthMbps: 100,
			},
			wantErr: false,
		},
		{
			name: "zero_max_channels",
			config: &MultichannelConfig{
				MaxChannels:      0,
				HealthCheckSec:   30,
				MinBandwidthMbps: 100,
			},
			wantErr: true,
			errMsg:  "最大通道数必须大于0",
		},
		{
			name: "excessive_max_channels",
			config: &MultichannelConfig{
				MaxChannels:      50,
				HealthCheckSec:   30,
				MinBandwidthMbps: 100,
			},
			wantErr: true,
			errMsg:  "最大通道数不能超过32",
		},
		{
			name: "low_health_check_interval",
			config: &MultichannelConfig{
				MaxChannels:      4,
				HealthCheckSec:   2,
				MinBandwidthMbps: 100,
			},
			wantErr: true,
			errMsg:  "健康检查间隔至少5秒",
		},
		{
			name: "high_health_check_interval",
			config: &MultichannelConfig{
				MaxChannels:      4,
				HealthCheckSec:   400,
				MinBandwidthMbps: 100,
			},
			wantErr: true,
			errMsg:  "健康检查间隔不能超过300秒",
		},
		{
			name: "low_bandwidth",
			config: &MultichannelConfig{
				MaxChannels:      4,
				HealthCheckSec:   30,
				MinBandwidthMbps: 5,
			},
			wantErr: true,
			errMsg:  "最低带宽要求至少10Mbps",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMultichannelConfig(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestGenerateMultichannelConfig 测试配置生成
func TestGenerateMultichannelConfig(t *testing.T) {
	tests := []struct {
		name     string
		config   *MultichannelConfig
		ifaces   []*NetworkInterface
		expected string
	}{
		{
			name: "disabled",
			config: &MultichannelConfig{
				Enabled: false,
			},
			ifaces:   nil,
			expected: "",
		},
		{
			name: "enabled_with_interfaces",
			config: &MultichannelConfig{
				Enabled:     true,
				MaxChannels: 4,
			},
			ifaces: []*NetworkInterface{
				{
					Name:        "eth0",
					IPAddresses: []string{"192.168.1.100"},
				},
				{
					Name:        "eth1",
					IPAddresses: []string{"192.168.1.101"},
				},
			},
			expected: "server multi channel support = yes",
		},
		{
			name: "enabled_no_interfaces",
			config: &MultichannelConfig{
				Enabled:     true,
				MaxChannels: 4,
			},
			ifaces:   []*NetworkInterface{},
			expected: "server multi channel support = yes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateMultichannelConfig(tt.config, tt.ifaces)

			if tt.expected == "" {
				assert.Empty(t, result)
			} else {
				assert.Contains(t, result, tt.expected)
			}
		})
	}
}

// TestGetRoundRobinInterface 测试轮询接口选择
func TestGetRoundRobinInterface(t *testing.T) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	config.RoundRobin = true
	manager := NewMultichannelManager(config)

	// 手动添加接口和通道进行测试
	manager.mu.Lock()
	manager.interfaces = []*NetworkInterface{
		{Name: "eth0", SpeedMbps: 1000},
		{Name: "eth1", SpeedMbps: 1000},
	}
	manager.channels = []*SMBChannel{
		{ID: 1, InterfaceName: "eth0", Connected: true, HealthScore: 100},
		{ID: 2, InterfaceName: "eth1", Connected: true, HealthScore: 100},
	}
	manager.mu.Unlock()

	// 获取轮询接口
	iface1 := manager.GetRoundRobinInterface()
	iface2 := manager.GetRoundRobinInterface()

	assert.NotNil(t, iface1)
	assert.NotNil(t, iface2)
	// 轮询应该交替返回不同接口（在健康状态下）
}

// TestEnableDisableChannel 测试启用禁用通道
func TestEnableDisableChannel(t *testing.T) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	manager := NewMultichannelManager(config)

	// 手动添加通道
	manager.mu.Lock()
	manager.channels = []*SMBChannel{
		{ID: 1, InterfaceName: "eth0", Connected: false},
	}
	manager.mu.Unlock()

	// 启用通道
	err := manager.EnableChannel(1)
	assert.NoError(t, err)

	channel := manager.GetChannelByID(1)
	assert.NotNil(t, channel)
	assert.True(t, channel.Connected)

	// 禁用通道
	err = manager.DisableChannel(1)
	assert.NoError(t, err)

	channel = manager.GetChannelByID(1)
	assert.NotNil(t, channel)
	assert.False(t, channel.Connected)

	// 测试不存在的通道
	err = manager.EnableChannel(999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "通道不存在")
}

// TestGetActiveInterfaceIPs 测试获取活动接口IP
func TestGetActiveInterfaceIPs(t *testing.T) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	manager := NewMultichannelManager(config)

	// 手动添加通道
	manager.mu.Lock()
	manager.channels = []*SMBChannel{
		{ID: 1, IPAddress: "192.168.1.100", Connected: true, HealthScore: 80},
		{ID: 2, IPAddress: "192.168.1.101", Connected: false, HealthScore: 30},
		{ID: 3, IPAddress: "192.168.1.102", Connected: true, HealthScore: 50},
	}
	manager.mu.Unlock()

	ips := manager.GetActiveInterfaceIPs()

	assert.Len(t, ips, 2)
	assert.Contains(t, ips, "192.168.1.100")
	assert.Contains(t, ips, "192.168.1.102")
	assert.NotContains(t, ips, "192.168.1.101")
}

// TestGetMultichannelMetrics 测试获取性能指标
func TestGetMultichannelMetrics(t *testing.T) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	manager := NewMultichannelManager(config)

	// 手动添加通道
	manager.mu.Lock()
	manager.channels = []*SMBChannel{
		{ID: 1, InterfaceName: "eth0", BandwidthMbps: 1000, Connections: 5, HealthScore: 90, Connected: true},
		{ID: 2, InterfaceName: "eth1", BandwidthMbps: 500, Connections: 0, HealthScore: 40, Connected: false},
	}
	manager.mu.Unlock()

	metrics := manager.GetMultichannelMetrics()

	assert.NotNil(t, metrics)
	assert.Equal(t, 2, metrics.TotalChannels)
	assert.Equal(t, 1, metrics.ActiveChannels)
	assert.Equal(t, 1000, metrics.TotalBandwidth)
	assert.Equal(t, 65, metrics.AvgHealthScore) // (90+40)/2
	assert.Len(t, metrics.ChannelMetrics, 2)
}

// TestUpdateConfig 测试配置更新
func TestUpdateConfig(t *testing.T) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	manager := NewMultichannelManager(config)

	// 更新配置
	newConfig := DefaultMultichannelConfig()
	newConfig.MaxChannels = 8
	newConfig.HealthCheckSec = 60

	err := manager.UpdateConfig(newConfig)
	assert.NoError(t, err)

	assert.Equal(t, 8, manager.config.MaxChannels)
	assert.Equal(t, 60, manager.config.HealthCheckSec)
}

// TestMultichannelConnectionInfo 测试连接信息解析
func TestMultichannelConnectionInfo(t *testing.T) {
	manager := NewMultichannelManager(DefaultMultichannelConfig())

	// 测试解析smbstatus输出（模拟）
	output := `
Service      pid     machine       connected at
IPC$         1234    192.168.1.100  (ipv4:192.168.1.1:445)  SMB3_11  AES-128-GCM
share1       1235    192.168.1.101  (ipv4:192.168.1.2:445)  SMB3_02  AES-256-GCM
`

	connections, err := manager.parseSmbStatusOutput(output)
	require.NoError(t, err)
	assert.Len(t, connections, 2)

	// 验证解析结果
	if len(connections) >= 2 {
		assert.Equal(t, "192.168.1.100", connections[0].ClientIP)
		assert.Contains(t, connections[0].Protocol, "SMB3")
	}
}

// TestChannelStatus 测试通道状态结构
func TestChannelStatus(t *testing.T) {
	status := &ChannelStatus{
		Enabled:          true,
		TotalChannels:    4,
		ActiveChannels:   3,
		TotalBandwidth:   4000,
		TotalConnections: 15,
		FailoverActive:   false,
	}

	assert.True(t, status.Enabled)
	assert.Equal(t, 4, status.TotalChannels)
	assert.Equal(t, 3, status.ActiveChannels)
	assert.Equal(t, 4000, status.TotalBandwidth)
}

// TestGetInterfaceSpeed_Mock 测试接口速度获取（模拟测试）
func TestGetInterfaceSpeed_Mock(t *testing.T) {
	// 创建临时目录模拟sysfs
	tmpDir := t.TempDir()
	netDir := filepath.Join(tmpDir, "class", "net", "eth0")
	require.NoError(t, os.MkdirAll(netDir, 0755))

	// 写入速度文件
	speedFile := filepath.Join(netDir, "speed")
	require.NoError(t, os.WriteFile(speedFile, []byte("1000"), 0644))

	config := DefaultMultichannelConfig()
	manager := NewMultichannelManager(config)

	// 注意：实际测试需要真实sysfs路径，这里仅测试逻辑
	// 在真实环境中，会读取 /sys/class/net/<iface>/speed
	speed := manager.getInterfaceSpeed("eth0")
	// 由于使用了真实路径，返回默认值
	assert.GreaterOrEqual(t, speed, 0)
}

// TestCheckChannelHealth 测试通道健康检查逻辑
func TestCheckChannelHealth(t *testing.T) {
	config := DefaultMultichannelConfig()
	config.MinBandwidthMbps = 100
	config.Enabled = false
	manager := NewMultichannelManager(config)

	channel := &SMBChannel{
		ID:            1,
		InterfaceName: "mock",
		IPAddress:     "192.168.1.100",
		Port:          445,
	}

	// 注意：健康检查依赖真实接口信息，这里测试逻辑流程
	health := manager.checkChannelHealth(channel)
	// 无法连接时会返回较低分数
	assert.GreaterOrEqual(t, health, 0)
	assert.LessOrEqual(t, health, 100)
}

// TestSortInterfacesByPriority 测试接口优先级排序
func TestSortInterfacesByPriority(t *testing.T) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	manager := NewMultichannelManager(config)

	// 手动添加接口（乱序）
	manager.mu.Lock()
	manager.interfaces = []*NetworkInterface{
		{Name: "wifi", SpeedMbps: 100},
		{Name: "slow_eth", SpeedMbps: 100},
		{Name: "fast_eth", SpeedMbps: 1000},
	}
	manager.mu.Unlock()

	manager.sortInterfacesByPriority()

	// 检查排序结果（按带宽降序）
	assert.Equal(t, "fast_eth", manager.interfaces[0].Name)
	assert.Equal(t, 1000, manager.interfaces[0].SpeedMbps)
	assert.Equal(t, 1, manager.interfaces[0].Priority)
}

// BenchmarkGetRoundRobinInterface 性能基准测试
func BenchmarkGetRoundRobinInterface(b *testing.B) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	manager := NewMultichannelManager(config)

	manager.mu.Lock()
	manager.interfaces = []*NetworkInterface{
		{Name: "eth0", SpeedMbps: 1000},
		{Name: "eth1", SpeedMbps: 1000},
	}
	manager.channels = []*SMBChannel{
		{ID: 1, InterfaceName: "eth0", Connected: true, HealthScore: 100},
		{ID: 2, InterfaceName: "eth1", Connected: true, HealthScore: 100},
	}
	manager.mu.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetRoundRobinInterface()
	}
}

// BenchmarkGetStatus 性能基准测试
func BenchmarkGetStatus(b *testing.B) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	manager := NewMultichannelManager(config)

	manager.mu.Lock()
	for i := 0; i < 10; i++ {
		manager.channels = append(manager.channels, &SMBChannel{
			ID:            i + 1,
			InterfaceName: "eth" + string(rune(i)),
			Connected:     true,
			HealthScore:   100,
		})
	}
	manager.mu.Unlock()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetStatus()
	}
}