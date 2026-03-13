package ftp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.False(t, config.Enabled)
	assert.Equal(t, 21, config.Port)
	assert.Equal(t, 50000, config.PasvPortStart)
	assert.Equal(t, 51000, config.PasvPortEnd)
	assert.Equal(t, "/data/ftp", config.RootPath)
	assert.False(t, config.AllowAnonymous)
	assert.Equal(t, 100, config.MaxConnections)
}

func TestNewServer(t *testing.T) {
	t.Run("with default config", func(t *testing.T) {
		server, err := NewServer(nil)
		assert.NoError(t, err)
		assert.NotNil(t, server)
		assert.NotNil(t, server.config)
		assert.False(t, server.running)
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &Config{
			Enabled:       false,
			Port:          2121,
			RootPath:      "/tmp/ftp-test",
			MaxConnections: 50,
		}
		server, err := NewServer(config)
		assert.NoError(t, err)
		assert.NotNil(t, server)
		assert.Equal(t, 2121, server.config.Port)
		assert.Equal(t, "/tmp/ftp-test", server.config.RootPath)
		assert.Equal(t, 50, server.config.MaxConnections)
	})
}

func TestServerLifecycle(t *testing.T) {
	config := &Config{
		Enabled:        false,
		Port:           2121,
		PasvPortStart:  60000,
		PasvPortEnd:    60100,
		RootPath:       t.TempDir(),
		AllowAnonymous: false,
		MaxConnections: 10,
	}

	server, err := NewServer(config)
	assert.NoError(t, err)
	assert.NotNil(t, server)

	// 初始状态应为未运行
	assert.False(t, server.IsRunning())

	// 启动服务器
	config.Enabled = true
	err = server.Start()
	assert.NoError(t, err)
	assert.True(t, server.IsRunning())

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 停止服务器
	err = server.Stop()
	assert.NoError(t, err)
	assert.False(t, server.IsRunning())
}

func TestUpdateConfig(t *testing.T) {
	config := DefaultConfig()
	config.Port = 2121
	config.RootPath = t.TempDir()
	config.Enabled = false

	server, err := NewServer(config)
	assert.NoError(t, err)

	// 更新配置
	newConfig := &Config{
		Enabled:        false,
		Port:           2222,
		RootPath:       t.TempDir(),
		AllowAnonymous: true,
		MaxConnections: 200,
	}

	err = server.UpdateConfig(newConfig)
	assert.NoError(t, err)

	updatedConfig := server.GetConfig()
	assert.Equal(t, 2222, updatedConfig.Port)
	assert.True(t, updatedConfig.AllowAnonymous)
	assert.Equal(t, 200, updatedConfig.MaxConnections)
}

func TestSetAuthFunc(t *testing.T) {
	server, err := NewServer(nil)
	assert.NoError(t, err)

	called := false
	server.SetAuthFunc(func(username, password string) bool {
		called = true
		return username == "test" && password == "pass"
	})

	// 验证认证函数已设置
	assert.NotNil(t, server.authFunc)
	assert.True(t, server.authFunc("test", "pass"))
	assert.True(t, called)
}

func TestSetGetUserHome(t *testing.T) {
	server, err := NewServer(nil)
	assert.NoError(t, err)

	server.SetGetUserHome(func(username string) string {
		return "/home/" + username
	})

	// 验证函数已设置
	assert.NotNil(t, server.getUserHome)
	assert.Equal(t, "/home/testuser", server.getUserHome("testuser"))
}

func TestBandwidthConfig(t *testing.T) {
	config := &Config{
		Enabled:  false,
		Port:     2121,
		RootPath: t.TempDir(),
		BandwidthLimit: BandwidthConfig{
			Enabled:      true,
			DownloadKBps: 1024,
			UploadKBps:   512,
			PerUser:      true,
		},
	}

	server, err := NewServer(config)
	assert.NoError(t, err)

	bwLimit := server.GetConfig().BandwidthLimit
	assert.True(t, bwLimit.Enabled)
	assert.Equal(t, int64(1024), bwLimit.DownloadKBps)
	assert.Equal(t, int64(512), bwLimit.UploadKBps)
	assert.True(t, bwLimit.PerUser)
}

func TestVirtualDirs(t *testing.T) {
	config := &Config{
		Enabled:  false,
		Port:     2121,
		RootPath: t.TempDir(),
		VirtualDirs: map[string]string{
			"/share":  "/mnt/share",
			"/backup": "/mnt/backup",
		},
	}

	server, err := NewServer(config)
	assert.NoError(t, err)

	virtualDirs := server.GetConfig().VirtualDirs
	assert.Equal(t, "/mnt/share", virtualDirs["/share"])
	assert.Equal(t, "/mnt/backup", virtualDirs["/backup"])
}

func TestGetStatus(t *testing.T) {
	config := DefaultConfig()
	config.RootPath = t.TempDir()

	server, err := NewServer(config)
	assert.NoError(t, err)

	status := server.GetStatus()
	assert.NotNil(t, status)
	assert.False(t, status["running"].(bool))
	assert.Equal(t, 0, status["connections"].(int))
}

func TestConnectionLimit(t *testing.T) {
	config := &Config{
		Enabled:        false,
		Port:           2121,
		RootPath:       t.TempDir(),
		MaxConnections: 5,
	}

	server, err := NewServer(config)
	assert.NoError(t, err)
	assert.NotNil(t, server.connSem)
	assert.Equal(t, 5, cap(server.connSem))
}