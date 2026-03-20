package ftp

import (
	"bufio"
	"bytes"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockConn 模拟网络连接
type mockConn struct {
	buf    bytes.Buffer
	closed bool
}

func (m *mockConn) Read(b []byte) (n int, err error)  { return 0, nil }
func (m *mockConn) Write(b []byte) (n int, err error) { return m.buf.Write(b) }
func (m *mockConn) Close() error                      { m.closed = true; return nil }
func (m *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 21}
}
func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
}
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

// TestHandleUSER 测试 USER 命令处理
func TestHandleUSER(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantCode int
	}{
		{"有效用户名", "testuser", 331},
		{"空用户名", "", 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
			conn := &mockConn{}
			client := &clientConn{
				id:     1,
				conn:   conn,
				server: server,
				writer: bufio.NewWriter(conn),
			}

			client.handleUSER(tt.username)

			// 验证用户名已设置
			if tt.username != "" {
				assert.Equal(t, tt.username, client.user)
			}
		})
	}
}

// TestHandlePASS 测试 PASS 命令处理
func TestHandlePASS(t *testing.T) {
	t.Run("匿名登录成功", func(t *testing.T) {
		config := &Config{
			Enabled:        false,
			RootPath:       t.TempDir(),
			AllowAnonymous: true,
		}
		server, _ := NewServer(config)
		conn := &mockConn{}
		client := &clientConn{
			id:      1,
			conn:    conn,
			server:  server,
			writer:  bufio.NewWriter(conn),
			user:    "anonymous",
			homeDir: config.RootPath,
		}

		client.handlePASS("test@example.com")

		assert.True(t, client.loggedIn)
	})

	t.Run("匿名登录被拒绝", func(t *testing.T) {
		config := &Config{
			Enabled:        false,
			RootPath:       t.TempDir(),
			AllowAnonymous: false,
		}
		server, _ := NewServer(config)
		conn := &mockConn{}
		client := &clientConn{
			id:     1,
			conn:   conn,
			server: server,
			writer: bufio.NewWriter(conn),
			user:   "anonymous",
		}

		client.handlePASS("test@example.com")

		assert.False(t, client.loggedIn)
	})

	t.Run("用户认证成功", func(t *testing.T) {
		config := &Config{
			Enabled:        false,
			RootPath:       t.TempDir(),
			AllowAnonymous: false,
		}
		server, _ := NewServer(config)
		server.SetAuthFunc(func(username, password string) bool {
			return username == "admin" && password == "secret"
		})
		server.SetGetUserHome(func(username string) string {
			return "/home/" + username
		})

		conn := &mockConn{}
		client := &clientConn{
			id:     1,
			conn:   conn,
			server: server,
			writer: bufio.NewWriter(conn),
			user:   "admin",
		}

		client.handlePASS("secret")

		assert.True(t, client.loggedIn)
		assert.Equal(t, "/home/admin", client.homeDir)
	})

	t.Run("用户认证失败", func(t *testing.T) {
		config := &Config{
			Enabled:        false,
			RootPath:       t.TempDir(),
			AllowAnonymous: false,
		}
		server, _ := NewServer(config)
		server.SetAuthFunc(func(username, password string) bool {
			return false
		})

		conn := &mockConn{}
		client := &clientConn{
			id:     1,
			conn:   conn,
			server: server,
			writer: bufio.NewWriter(conn),
			user:   "admin",
		}

		client.handlePASS("wrongpassword")

		assert.False(t, client.loggedIn)
	})

	t.Run("未先执行 USER", func(t *testing.T) {
		server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
		conn := &mockConn{}
		client := &clientConn{
			id:     1,
			conn:   conn,
			server: server,
			writer: bufio.NewWriter(conn),
		}

		client.handlePASS("password")

		assert.False(t, client.loggedIn)
	})
}

// TestHandleFEAT 测试 FEAT 命令处理
func TestHandleFEAT(t *testing.T) {
	server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
	conn := &mockConn{}
	client := &clientConn{
		id:     1,
		conn:   conn,
		server: server,
		writer: bufio.NewWriter(conn),
	}

	client.handleFEAT()

	// 验证输出包含特性列表
	output := conn.buf.String()
	assert.Contains(t, output, "PASV")
	assert.Contains(t, output, "UTF8")
}

// TestHandlePWD 测试 PWD 命令处理
func TestHandlePWD(t *testing.T) {
	server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
	conn := &mockConn{}
	client := &clientConn{
		id:         1,
		conn:       conn,
		server:     server,
		writer:     bufio.NewWriter(conn),
		currentDir: "/test",
	}

	client.handlePWD()

	output := conn.buf.String()
	assert.Contains(t, output, "257")
	assert.Contains(t, output, "/test")
}

// TestHandleCDUP 测试 CDUP 命令处理
func TestHandleCDUP(t *testing.T) {
	tests := []struct {
		name        string
		currentDir  string
		expectedDir string
	}{
		{"从子目录返回", "/data/files", "/data"},
		{"从根目录", "/", "/"},
		{"从一级目录", "/data", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
			conn := &mockConn{}
			client := &clientConn{
				id:         1,
				conn:       conn,
				server:     server,
				writer:     bufio.NewWriter(conn),
				currentDir: tt.currentDir,
			}

			client.handleCDUP()

			assert.Equal(t, tt.expectedDir, client.currentDir)
		})
	}
}

// TestHandleTYPE 测试 TYPE 命令处理
func TestHandleTYPE(t *testing.T) {
	tests := []struct {
		typ        string
		binaryMode bool
	}{
		{"A", false}, // ASCII
		{"I", true},  // Binary/Image
		{"a", false}, // ASCII (小写)
		{"i", true},  // Binary (小写)
	}

	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
			conn := &mockConn{}
			client := &clientConn{
				id:     1,
				conn:   conn,
				server: server,
				writer: bufio.NewWriter(conn),
			}

			client.handleTYPE(tt.typ)

			assert.Equal(t, tt.binaryMode, client.binaryMode)
		})
	}
}

// TestHandleREST 测试 REST 命令处理
func TestHandleREST(t *testing.T) {
	server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
	conn := &mockConn{}
	client := &clientConn{
		id:     1,
		conn:   conn,
		server: server,
		writer: bufio.NewWriter(conn),
	}

	client.handleREST("1024")

	assert.Equal(t, int64(1024), client.restOffset)
}

// TestHandleABOR 测试 ABOR 命令处理
func TestHandleABOR(t *testing.T) {
	server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
	conn := &mockConn{}
	client := &clientConn{
		id:         1,
		conn:       conn,
		server:     server,
		writer:     bufio.NewWriter(conn),
		restOffset: 1024,
	}

	client.handleABOR()

	assert.Equal(t, int64(0), client.restOffset)
}

// TestHandleCommand 测试命令路由
func TestHandleCommand(t *testing.T) {
	tests := []struct {
		cmd     string
		args    string
		wantErr bool
	}{
		{"SYST", "", false},
		{"NOOP", "", false},
		{"QUIT", "", false},
		{"PWD", "", false},
		{"CDUP", "", false},
		{"TYPE", "I", false},
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
			conn := &mockConn{}
			client := &clientConn{
				id:       1,
				conn:     conn,
				server:   server,
				writer:   bufio.NewWriter(conn),
				loggedIn: true,
				homeDir:  t.TempDir(),
			}

			// 不会 panic 就算成功
			client.handleCommand(tt.cmd, tt.args)
		})
	}
}

// TestHandleCommand_Unknown 测试未知命令
func TestHandleCommand_Unknown(t *testing.T) {
	server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
	conn := &mockConn{}
	client := &clientConn{
		id:       1,
		conn:     conn,
		server:   server,
		writer:   bufio.NewWriter(conn),
		loggedIn: true,
	}

	client.handleCommand("UNKNOWN", "")

	output := conn.buf.String()
	assert.Contains(t, output, "500")
}

// TestHandleCommand_NotLoggedIn 测试未登录时拒绝命令
func TestHandleCommand_NotLoggedIn(t *testing.T) {
	server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
	conn := &mockConn{}
	client := &clientConn{
		id:       1,
		conn:     conn,
		server:   server,
		writer:   bufio.NewWriter(conn),
		loggedIn: false,
	}

	// 除了 USER, PASS, QUIT, SYST, FEAT 外，其他命令应被拒绝
	client.handleCommand("LIST", "")

	output := conn.buf.String()
	assert.Contains(t, output, "530")
}

// TestNormalizePath 测试路径标准化
func TestNormalizePath(t *testing.T) {
	server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
	conn := &mockConn{}
	client := &clientConn{
		id:         1,
		conn:       conn,
		server:     server,
		currentDir: "/home/user",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "/home/user/relative/path"},
		{"./current", "/home/user/current"},
		{"../parent", "/home/parent"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := client.normalizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestResolvePath 测试路径解析
func TestResolvePath(t *testing.T) {
	tempDir := t.TempDir()
	server, _ := NewServer(&Config{
		Enabled:  false,
		RootPath: tempDir,
		VirtualDirs: map[string]string{
			"/share": "/mnt/share",
		},
	})
	conn := &mockConn{}
	client := &clientConn{
		id:         1,
		conn:       conn,
		server:     server,
		currentDir: "/",
		homeDir:    tempDir,
	}

	tests := []struct {
		name     string
		path     string
		contains string
	}{
		{"根路径", "/", tempDir},
		{"虚拟目录", "/share/file.txt", "/mnt/share"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.resolvePath(tt.path)
			assert.Contains(t, result, tt.contains)
		})
	}
}

// TestFormatFileInfo 测试文件信息格式化
func TestFormatFileInfo(t *testing.T) {
	server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
	conn := &mockConn{}
	client := &clientConn{
		id:     1,
		conn:   conn,
		server: server,
	}

	// 测试目录
	dirInfo, _ := requireOpenDir(t.TempDir())
	dirLine := client.formatFileInfo("testdir", dirInfo)
	assert.True(t, strings.HasPrefix(dirLine, "d"))

	// 测试文件
	fileInfo, _ := requireCreateFile(t.TempDir()+"/testfile.txt", "content")
	fileLine := client.formatFileInfo("testfile.txt", fileInfo)
	assert.True(t, strings.HasPrefix(fileLine, "-"))
	assert.Contains(t, fileLine, "testfile.txt")
}

// 辅助函数：创建目录并返回 FileInfo
func requireOpenDir(path string) (info os.FileInfo, err error) {
	return os.Stat(path)
}

// 辅助函数：创建文件并返回 FileInfo
func requireCreateFile(path, content string) (info os.FileInfo, err error) {
	if err = os.WriteFile(path, []byte(content), 0644); err != nil {
		return
	}
	return os.Stat(path)
}

// TestGetStatus_Running 测试运行状态
func TestGetStatus_Running(t *testing.T) {
	config := &Config{
		Enabled:        false,
		Port:           2121,
		PasvPortStart:  60000,
		PasvPortEnd:    60100,
		RootPath:       t.TempDir(),
		AllowAnonymous: false,
		MaxConnections: 10,
	}

	server, _ := NewServer(config)

	// 未启动时的状态
	status := server.GetStatus()
	assert.False(t, status["running"].(bool))
	assert.Equal(t, 0, status["connections"].(int))
}

// TestErrServerErrors 测试错误定义
func TestErrServerErrors(t *testing.T) {
	assert.Equal(t, "服务器未运行", ErrServerNotRunning.Error())
	assert.Equal(t, "服务器已在运行", ErrServerRunning.Error())
	assert.Equal(t, "连接数超过限制", ErrTooManyConns.Error())
	assert.Equal(t, "登录失败", ErrLoginFailed.Error())
	assert.Equal(t, "权限被拒绝", ErrPermissionDenied.Error())
}

// TestClientConn_Close 测试客户端连接关闭
func TestClientConn_Close(t *testing.T) {
	server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
	conn := &mockConn{}
	client := &clientConn{
		id:     1,
		conn:   conn,
		server: server,
	}

	// 第一次关闭
	client.close()
	assert.True(t, client.closed)

	// 再次关闭不应 panic
	client.close()
}

// TestWriteResponse 测试响应写入
func TestWriteResponse(t *testing.T) {
	server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
	conn := &mockConn{}
	client := &clientConn{
		id:     1,
		conn:   conn,
		server: server,
		writer: bufio.NewWriter(conn),
	}

	err := client.writeResponse(220, "Welcome")
	require.NoError(t, err)

	output := conn.buf.String()
	assert.Contains(t, output, "220")
	assert.Contains(t, output, "Welcome")
}

// TestHandlePORT 测试 PORT 命令处理
func TestHandlePORT(t *testing.T) {
	tests := []struct {
		name   string
		args   string
		wantOK bool
	}{
		{"有效地址", "127,0,0,1,19,136", true}, // 127.0.0.1:5000
		{"无效格式-参数不足", "127,0,0,1", false},
		{"无效格式-空", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, _ := NewServer(&Config{Enabled: false, RootPath: t.TempDir()})
			conn := &mockConn{}
			client := &clientConn{
				id:     1,
				conn:   conn,
				server: server,
				writer: bufio.NewWriter(conn),
			}

			client.handlePORT(tt.args)

			output := conn.buf.String()
			if tt.wantOK {
				assert.Contains(t, output, "200")
			} else {
				assert.Contains(t, output, "500")
			}
		})
	}
}

// TestHandleSIZE 测试 SIZE 命令处理
func TestHandleSIZE(t *testing.T) {
	tempDir := t.TempDir()
	testFile := tempDir + "/test.txt"
	require.NoError(t, os.WriteFile(testFile, []byte("hello world"), 0644))

	server, _ := NewServer(&Config{Enabled: false, RootPath: tempDir})
	conn := &mockConn{}
	client := &clientConn{
		id:         1,
		conn:       conn,
		server:     server,
		writer:     bufio.NewWriter(conn),
		homeDir:    tempDir,
		currentDir: "/",
		loggedIn:   true,
	}

	client.handleSIZE("test.txt")

	output := conn.buf.String()
	assert.Contains(t, output, "213")
	assert.Contains(t, output, "11") // 文件大小
}

// TestHandleSIZE_Directory 测试 SIZE 对目录的处理
func TestHandleSIZE_Directory(t *testing.T) {
	tempDir := t.TempDir()

	server, _ := NewServer(&Config{Enabled: false, RootPath: tempDir})
	conn := &mockConn{}
	client := &clientConn{
		id:         1,
		conn:       conn,
		server:     server,
		writer:     bufio.NewWriter(conn),
		homeDir:    tempDir,
		currentDir: "/",
		loggedIn:   true,
	}

	client.handleSIZE("/") // 目录

	output := conn.buf.String()
	assert.Contains(t, output, "550") // 不是文件
}

// TestHandleMKD 测试 MKD 命令处理
func TestHandleMKD(t *testing.T) {
	tempDir := t.TempDir()

	server, _ := NewServer(&Config{Enabled: false, RootPath: tempDir})
	conn := &mockConn{}
	client := &clientConn{
		id:         1,
		conn:       conn,
		server:     server,
		writer:     bufio.NewWriter(conn),
		homeDir:    tempDir,
		currentDir: "/",
		loggedIn:   true,
	}

	client.handleMKD("newdir")

	output := conn.buf.String()
	assert.Contains(t, output, "257")

	// 验证目录已创建
	_, err := os.Stat(tempDir + "/newdir")
	assert.NoError(t, err)
}

// TestHandleRMD 测试 RMD 命令处理
func TestHandleRMD(t *testing.T) {
	tempDir := t.TempDir()
	testDir := tempDir + "/testdir"
	require.NoError(t, os.Mkdir(testDir, 0755))

	server, _ := NewServer(&Config{Enabled: false, RootPath: tempDir})
	conn := &mockConn{}
	client := &clientConn{
		id:         1,
		conn:       conn,
		server:     server,
		writer:     bufio.NewWriter(conn),
		homeDir:    tempDir,
		currentDir: "/",
		loggedIn:   true,
	}

	client.handleRMD("testdir")

	output := conn.buf.String()
	assert.Contains(t, output, "250")

	// 验证目录已删除
	_, err := os.Stat(testDir)
	assert.True(t, os.IsNotExist(err))
}

// TestHandleDELE 测试 DELE 命令处理
func TestHandleDELE(t *testing.T) {
	tempDir := t.TempDir()
	testFile := tempDir + "/testfile.txt"
	require.NoError(t, os.WriteFile(testFile, []byte("test"), 0644))

	server, _ := NewServer(&Config{Enabled: false, RootPath: tempDir})
	conn := &mockConn{}
	client := &clientConn{
		id:         1,
		conn:       conn,
		server:     server,
		writer:     bufio.NewWriter(conn),
		homeDir:    tempDir,
		currentDir: "/",
		loggedIn:   true,
	}

	client.handleDELE("testfile.txt")

	output := conn.buf.String()
	assert.Contains(t, output, "250")

	// 验证文件已删除
	_, err := os.Stat(testFile)
	assert.True(t, os.IsNotExist(err))
}

// TestHandleRNFR_RNTO 测试重命名命令
func TestHandleRNFR_RNTO(t *testing.T) {
	tempDir := t.TempDir()
	oldFile := tempDir + "/old.txt"
	require.NoError(t, os.WriteFile(oldFile, []byte("test"), 0644))

	server, _ := NewServer(&Config{Enabled: false, RootPath: tempDir})
	conn := &mockConn{}
	client := &clientConn{
		id:         1,
		conn:       conn,
		server:     server,
		writer:     bufio.NewWriter(conn),
		homeDir:    tempDir,
		currentDir: "/",
		loggedIn:   true,
	}

	// 先发送 RNFR
	client.handleRNFR("old.txt")
	output1 := conn.buf.String()
	assert.Contains(t, output1, "350")

	// 再发送 RNTO
	conn.buf.Reset()
	client.handleRNTO("new.txt")
	output2 := conn.buf.String()
	assert.Contains(t, output2, "250")

	// 验证重命名成功
	_, err := os.Stat(tempDir + "/new.txt")
	assert.NoError(t, err)
}
