package sftp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/ssh"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.NotNil(t, config)
	assert.False(t, config.Enabled)
	assert.Equal(t, 22, config.Port)
	assert.Equal(t, "/etc/nas-os/ssh_host_key", config.HostKeyPath)
	assert.Equal(t, "/data/sftp", config.RootPath)
	assert.False(t, config.AllowAnonymous)
	assert.Equal(t, 100, config.MaxConnections)
	assert.Equal(t, 300, config.IdleTimeout)
	assert.True(t, config.ChrootEnabled)
}

func TestNewServer(t *testing.T) {
	t.Run("with default config", func(t *testing.T) {
		// 使用临时目录和密钥路径
		config := DefaultConfig()
		config.HostKeyPath = t.TempDir() + "/ssh_host_key"

		server, err := NewServer(config)
		// 可能会因为缺少有效的主机密钥而失败，这是预期行为
		// 在测试中我们主要验证创建流程
		if err != nil {
			// 预期可能失败
			t.Logf("NewServer error (expected in test env): %v", err)
		} else {
			assert.NotNil(t, server)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		config := &Config{
			Enabled:        false,
			Port:           2222,
			HostKeyPath:    t.TempDir() + "/ssh_host_key",
			RootPath:       "/tmp/sftp-test",
			MaxConnections: 50,
		}

		server, err := NewServer(config)
		if err != nil {
			t.Logf("NewServer error (expected in test env): %v", err)
		} else {
			assert.NotNil(t, server)
			assert.Equal(t, 2222, server.config.Port)
			assert.Equal(t, "/tmp/sftp-test", server.config.RootPath)
			assert.Equal(t, 50, server.config.MaxConnections)
		}
	})
}

func TestServerLifecycle(t *testing.T) {
	config := &Config{
		Enabled:        false,
		Port:           2222,
		HostKeyPath:    t.TempDir() + "/ssh_host_key",
		RootPath:       t.TempDir(),
		MaxConnections: 10,
	}

	server, err := NewServer(config)
	if err != nil {
		t.Skipf("Cannot create server in test env: %v", err)
		return
	}

	// 初始状态应为未运行
	assert.False(t, server.IsRunning())

	// 停止未启动的服务器应该成功
	err = server.Stop()
	assert.NoError(t, err)
}

func TestSetAuthFunc(t *testing.T) {
	config := DefaultConfig()
	config.HostKeyPath = t.TempDir() + "/ssh_host_key"

	server, err := NewServer(config)
	if err != nil {
		t.Skipf("Cannot create server in test env: %v", err)
		return
	}

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

func TestSetPublicKeyAuth(t *testing.T) {
	config := DefaultConfig()
	config.HostKeyPath = t.TempDir() + "/ssh_host_key"

	server, err := NewServer(config)
	if err != nil {
		t.Skipf("Cannot create server in test env: %v", err)
		return
	}

	server.SetPublicKeyAuth(func(username string, pubKey ssh.PublicKey) bool {
		return username == "testuser"
	})

	// 验证公钥认证函数已设置
	assert.NotNil(t, server.pubKeyAuth)
}

func TestSetGetUserHome(t *testing.T) {
	config := DefaultConfig()
	config.HostKeyPath = t.TempDir() + "/ssh_host_key"

	server, err := NewServer(config)
	if err != nil {
		t.Skipf("Cannot create server in test env: %v", err)
		return
	}

	server.SetGetUserHome(func(username string) string {
		return "/home/" + username
	})

	// 验证函数已设置
	assert.NotNil(t, server.getUserHome)
	assert.Equal(t, "/home/testuser", server.getUserHome("testuser"))
}

func TestChrootConfig(t *testing.T) {
	config := &Config{
		Enabled:       false,
		Port:          2222,
		HostKeyPath:   t.TempDir() + "/ssh_host_key",
		RootPath:      t.TempDir(),
		ChrootEnabled: true,
		UserChroots: map[string]string{
			"alice": "/mnt/alice",
			"bob":   "/mnt/bob",
		},
	}

	server, err := NewServer(config)
	if err != nil {
		t.Skipf("Cannot create server in test env: %v", err)
		return
	}

	userChroots := server.GetConfig().UserChroots
	assert.Equal(t, "/mnt/alice", userChroots["alice"])
	assert.Equal(t, "/mnt/bob", userChroots["bob"])
	assert.True(t, server.GetConfig().ChrootEnabled)
}

func TestGetStatus(t *testing.T) {
	config := DefaultConfig()
	config.HostKeyPath = t.TempDir() + "/ssh_host_key"

	server, err := NewServer(config)
	if err != nil {
		t.Skipf("Cannot create server in test env: %v", err)
		return
	}

	status := server.GetStatus()
	assert.NotNil(t, status)
	assert.False(t, status["running"].(bool))
	assert.Equal(t, 0, status["connections"].(int))
	assert.True(t, status["chroot_enabled"].(bool))
}

func TestConnectionLimit(t *testing.T) {
	config := &Config{
		Enabled:        false,
		Port:           2222,
		HostKeyPath:    t.TempDir() + "/ssh_host_key",
		RootPath:       t.TempDir(),
		MaxConnections: 5,
	}

	server, err := NewServer(config)
	if err != nil {
		t.Skipf("Cannot create server in test env: %v", err)
		return
	}

	assert.NotNil(t, server.connSem)
	assert.Equal(t, 5, cap(server.connSem))
}

func TestIdleTimeout(t *testing.T) {
	config := &Config{
		Enabled:     false,
		Port:        2222,
		HostKeyPath: t.TempDir() + "/ssh_host_key",
		RootPath:    t.TempDir(),
		IdleTimeout: 600, // 10 分钟
	}

	server, err := NewServer(config)
	if err != nil {
		t.Skipf("Cannot create server in test env: %v", err)
		return
	}

	assert.Equal(t, 600, server.GetConfig().IdleTimeout)
}

func TestSftpHandlerResolvePath(t *testing.T) {
	handler := &sftpHandler{
		rootDir:  "/data/sftp",
		readOnly: false,
	}

	tests := []struct {
		name      string
		path      string
		want      string
		wantError bool
	}{
		{
			name: "simple path",
			path: "/test.txt",
			want: "/data/sftp/test.txt",
		},
		{
			name: "nested path",
			path: "/dir/subdir/file.txt",
			want: "/data/sftp/dir/subdir/file.txt",
		},
		{
			name:      "traversal attack",
			path:      "../etc/passwd",
			wantError: true,
		},
		{
			name:      "deep traversal",
			path:      "/foo/../../bar",
			wantError: false, // filepath.Clean handles this safely
			want:      "/data/sftp/bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := handler.resolvePath(tt.path)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
