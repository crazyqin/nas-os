// Package tunnel 提供内网穿透服务 - 增强配置测试
package tunnel

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultEnhancedConfig 测试默认增强配置
func TestDefaultEnhancedConfig(t *testing.T) {
	config := DefaultEnhancedConfig()

	require.NotNil(t, config)
	assert.NotNil(t, config.Config)
	assert.Equal(t, ModeAuto, config.Mode)
	assert.Equal(t, 30, config.HeartbeatInt)
	assert.Equal(t, 5, config.ReconnectInt)
	assert.Equal(t, 10, config.MaxReconnect)
	assert.Equal(t, 30, config.Timeout)
}

// TestEnhancedConfigValidation 测试增强配置验证
func TestEnhancedConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *EnhancedConfig
		wantErr bool
		errMsg  string
	}{
		{
			name:   "valid config",
			config: createValidTestConfig(),
			wantErr: false,
		},
		{
			name: "missing server address",
			config: &EnhancedConfig{
				Config: &Config{
					ServerPort: 8080,
				},
			},
			wantErr: true,
			errMsg:  "server address is required",
		},
		{
			name: "invalid server port",
			config: &EnhancedConfig{
				Config: &Config{
					ServerAddr: "localhost",
					ServerPort: 70000,
				},
			},
			wantErr: true,
			errMsg:  "invalid server port",
		},
		{
			name: "negative upload limit",
			config: &EnhancedConfig{
				Config: &Config{
					ServerAddr: "localhost",
					ServerPort: 8080,
				},
				Bandwidth: BandwidthConfig{
					UploadLimit: -100,
				},
			},
			wantErr: true,
			errMsg:  "upload limit cannot be negative",
		},
		{
			name: "negative download limit",
			config: &EnhancedConfig{
				Config: &Config{
					ServerAddr: "localhost",
					ServerPort: 8080,
				},
				Bandwidth: BandwidthConfig{
					DownloadLimit: -100,
				},
			},
			wantErr: true,
			errMsg:  "download limit cannot be negative",
		},
		{
			name: "invalid switch threshold",
			config: &EnhancedConfig{
				Config: &Config{
					ServerAddr: "localhost",
					ServerPort: 8080,
				},
				Optimization: OptimizationConfig{
					SwitchThreshold: 150,
				},
			},
			wantErr: true,
			errMsg:  "switch threshold must be between 0 and 100",
		},
		{
			name: "invalid cipher algorithm",
			config: &EnhancedConfig{
				Config: &Config{
					ServerAddr: "localhost",
					ServerPort: 8080,
				},
				Security: SecurityConfig{
					CipherAlgorithm: "invalid-cipher",
				},
			},
			wantErr: true,
			errMsg:  "invalid cipher algorithm",
		},
		{
			name: "invalid MTU - too low",
			config: &EnhancedConfig{
				Config: &Config{
					ServerAddr: "localhost",
					ServerPort: 8080,
				},
				Security: SecurityConfig{
					CipherAlgorithm: "chacha20-poly1305",
				},
				Advanced: AdvancedConfig{
					MTU: 100,
				},
			},
			wantErr: true,
			errMsg:  "MTU must be between 576 and 9000",
		},
		{
			name: "invalid max connections",
			config: &EnhancedConfig{
				Config: &Config{
					ServerAddr: "localhost",
					ServerPort: 8080,
				},
				Security: SecurityConfig{
					CipherAlgorithm: "chacha20-poly1305",
				},
				Advanced: AdvancedConfig{
					MTU:            1400,
					MaxConnections: 0,
				},
			},
			wantErr: true,
			errMsg:  "max connections must be at least 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEnhancedConfig(tt.config)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestApplyProfile 测试应用配置模板
func TestApplyProfile(t *testing.T) {
	tests := []struct {
		name          string
		profile       ConfigProfile
		checkResult   func(t *testing.T, c *EnhancedConfig)
	}{
		{
			name:    "performance profile",
			profile: ProfilePerformance,
			checkResult: func(t *testing.T, c *EnhancedConfig) {
				assert.True(t, c.Optimization.P2PPriority)
				assert.True(t, c.Advanced.EnableMultiplexing)
				assert.Equal(t, int64(0), c.Bandwidth.UploadLimit)
			},
		},
		{
			name:    "reliable profile",
			profile: ProfileReliable,
			checkResult: func(t *testing.T, c *EnhancedConfig) {
				assert.True(t, c.Optimization.AutoModeSwitch)
				assert.Equal(t, 5, c.Optimization.ConnectRetries)
			},
		},
		{
			name:    "low latency profile",
			profile: ProfileLowLatency,
			checkResult: func(t *testing.T, c *EnhancedConfig) {
				assert.True(t, c.Optimization.P2PPriority)
				assert.Equal(t, int64(30), c.Quality.Thresholds.ExcellentLatency)
				assert.Equal(t, int64(50), c.Quality.Thresholds.GoodLatency)
			},
		},
		{
			name:    "secure profile",
			profile: ProfileSecure,
			checkResult: func(t *testing.T, c *EnhancedConfig) {
				assert.True(t, c.Security.EnableEncryption)
				assert.Equal(t, "aes-256-gcm", c.Security.CipherAlgorithm)
				assert.Equal(t, 3, c.Security.MaxAuthFailures)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := DefaultEnhancedConfig()
			config.ServerAddr = "localhost"
			config.ServerPort = 8080

			config.ApplyProfile(tt.profile)
			tt.checkResult(t, config)
		})
	}
}

// TestSaveAndLoadEnhancedConfig 测试保存和加载配置
func TestSaveAndLoadEnhancedConfig(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "tunnel-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// 创建并保存配置
	originalConfig := createValidTestConfig()
	err = SaveEnhancedConfig(configPath, originalConfig)
	require.NoError(t, err)

	// 加载配置
	loadedConfig, err := LoadEnhancedConfig(configPath)
	require.NoError(t, err)

	// 验证
	assert.Equal(t, originalConfig.ServerAddr, loadedConfig.ServerAddr)
	assert.Equal(t, originalConfig.ServerPort, loadedConfig.ServerPort)
	assert.Equal(t, originalConfig.Mode, loadedConfig.Mode)
}

// TestCompareConfigs 测试配置比较
func TestCompareConfigs(t *testing.T) {
	oldConfig := createValidTestConfig()
	newConfig := createValidTestConfig()

	// 无差异
	diffs := CompareConfigs(oldConfig, newConfig)
	assert.Len(t, diffs, 0)

	// 修改配置
	newConfig.ServerAddr = "new-server"
	newConfig.ServerPort = 9090
	newConfig.Mode = ModeP2P
	newConfig.Bandwidth.UploadLimit = 1000000

	diffs = CompareConfigs(oldConfig, newConfig)
	assert.Len(t, diffs, 4)
}

// TestConfigManager 测试配置管理器
func TestConfigManager(t *testing.T) {
	// 创建临时目录
	tmpDir, err := os.MkdirTemp("", "tunnel-config-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "config.json")

	// 创建管理器
	manager := NewConfigManager(configPath)

	// 加载配置（文件不存在，使用默认配置）
	err = manager.Load()
	require.NoError(t, err)

	config := manager.Get()
	assert.NotNil(t, config)

	// 修改并保存
	config.ServerAddr = "test-server"
	config.ServerPort = 8080

	err = manager.Set(config)
	require.NoError(t, err)

	err = manager.Save()
	require.NoError(t, err)

	// 重新加载
	err = manager.Reload()
	require.NoError(t, err)

	loaded := manager.Get()
	assert.Equal(t, "test-server", loaded.ServerAddr)
	assert.Equal(t, 8080, loaded.ServerPort)
}

// TestOptimizationConfig 测试优化配置
func TestOptimizationConfig(t *testing.T) {
	config := OptimizationConfig{
		AutoModeSwitch:    true,
		P2PPriority:       true,
		SwitchThreshold:   50,
		ConnectRetries:    3,
		RetryInterval:     time.Second,
		KeepaliveInterval: 25 * time.Second,
		ConnectTimeout:    10 * time.Second,
		IdleTimeout:       5 * time.Minute,
	}

	assert.True(t, config.AutoModeSwitch)
	assert.True(t, config.P2PPriority)
	assert.Equal(t, 50, config.SwitchThreshold)
	assert.Equal(t, 3, config.ConnectRetries)
	assert.Equal(t, time.Second, config.RetryInterval)
}

// TestSecurityConfig 测试安全配置
func TestSecurityConfig(t *testing.T) {
	config := SecurityConfig{
		EnableEncryption: true,
		CipherAlgorithm:  "chacha20-poly1305",
		KeyExchange:      "x25519",
		AuthTimeout:      10 * time.Second,
		AllowedIPs:       []string{"192.168.1.0/24"},
		MaxAuthFailures:  5,
		AuthLockDuration: 5 * time.Minute,
	}

	assert.True(t, config.EnableEncryption)
	assert.Equal(t, "chacha20-poly1305", config.CipherAlgorithm)
	assert.Equal(t, "x25519", config.KeyExchange)
	assert.Equal(t, 10*time.Second, config.AuthTimeout)
	assert.Len(t, config.AllowedIPs, 1)
}

// TestAdvancedConfig 测试高级配置
func TestAdvancedConfig(t *testing.T) {
	config := AdvancedConfig{
		MTU:                1400,
		SendBuffer:         1024 * 1024,
		RecvBuffer:         1024 * 1024,
		MaxConnections:     100,
		ConnectionQueue:    128,
		EnableMultiplexing: true,
		MuxStreams:         16,
		EnableCompression:  false,
		CompressionLevel:   6,
		EnableTCPFastOpen:  true,
	}

	assert.Equal(t, 1400, config.MTU)
	assert.Equal(t, 1024*1024, config.SendBuffer)
	assert.Equal(t, 100, config.MaxConnections)
	assert.True(t, config.EnableMultiplexing)
	assert.True(t, config.EnableTCPFastOpen)
}

// TestLoggingConfig 测试日志配置
func TestLoggingConfig(t *testing.T) {
	config := LoggingConfig{
		Level:      "info",
		Format:     "json",
		OutputPath: "/var/log/tunnel.log",
		MaxSize:    100,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   true,
	}

	assert.Equal(t, "info", config.Level)
	assert.Equal(t, "json", config.Format)
	assert.Equal(t, 100, config.MaxSize)
	assert.True(t, config.Compress)
}

// TestBandwidthConfig 测试带宽配置
func TestBandwidthConfig(t *testing.T) {
	config := BandwidthConfig{
		UploadLimit:     1000000,
		DownloadLimit:   2000000,
		StatsInterval:   time.Second,
		BucketMultiplier: 2,
	}

	assert.Equal(t, int64(1000000), config.UploadLimit)
	assert.Equal(t, int64(2000000), config.DownloadLimit)
	assert.Equal(t, time.Second, config.StatsInterval)
}

// 辅助函数：创建有效的测试配置
func createValidTestConfig() *EnhancedConfig {
	config := DefaultEnhancedConfig()
	config.ServerAddr = "localhost"
	config.ServerPort = 8080
	return config
}