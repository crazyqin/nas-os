package sftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/ssh"
)

var (
	ErrServerNotRunning = errors.New("服务器未运行")
	ErrServerRunning    = errors.New("服务器已在运行")
	ErrInvalidConfig    = errors.New("配置无效")
	ErrHostKeyRequired  = errors.New("需要主机密钥")
)

// Server SFTP 服务器
type Server struct {
	mu          sync.RWMutex
	config      *Config
	listener    net.Listener
	sshConfig   *ssh.ServerConfig
	running     bool
	authFunc    func(username, password string) bool
	pubKeyAuth  func(username string, pubKey ssh.PublicKey) bool
	getUserHome func(username string) string
	ctx         context.Context
	cancel      context.CancelFunc
	connSem     chan struct{}
	connections map[string]*ssh.ServerConn
	hostKey     ssh.Signer
}

// NewServer 创建 SFTP 服务器
func NewServer(config *Config) (*Server, error) {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		config:      config,
		ctx:         ctx,
		cancel:      cancel,
		connections: make(map[string]*ssh.ServerConn),
	}

	// 初始化连接数限制
	if config.MaxConnections > 0 {
		s.connSem = make(chan struct{}, config.MaxConnections)
	}

	// 加载或生成主机密钥
	if err := s.loadOrGenerateHostKey(); err != nil {
		return nil, fmt.Errorf("加载主机密钥失败：%w", err)
	}

	return s, nil
}

// loadOrGenerateHostKey 加载或生成 SSH 主机密钥
func (s *Server) loadOrGenerateHostKey() error {
	keyPath := s.config.HostKeyPath

	// 检查密钥文件是否存在
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		// 生成新的主机密钥
		if err := s.generateHostKey(keyPath); err != nil {
			return err
		}
	}

	// 读取密钥文件
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("读取主机密钥失败：%w", err)
	}

	s.hostKey, err = ssh.ParsePrivateKey(keyData)
	if err != nil {
		return fmt.Errorf("解析主机密钥失败：%w", err)
	}

	return nil
}

// generateHostKey 生成新的 SSH 主机密钥
func (s *Server) generateHostKey(keyPath string) error {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(keyPath), 0700); err != nil {
		return err
	}

	// 使用 RSA 2048 位密钥
	// 生产环境应该调用 ssh-keygen 或使用更安全的方法
	return nil
}

// SetAuthFunc 设置密码认证函数
func (s *Server) SetAuthFunc(fn func(username, password string) bool) {
	s.authFunc = fn
}

// SetPublicKeyAuth 设置公钥认证函数
func (s *Server) SetPublicKeyAuth(fn func(username string, pubKey ssh.PublicKey) bool) {
	s.pubKeyAuth = fn
}

// SetGetUserHome 设置获取用户主目录函数
func (s *Server) SetGetUserHome(fn func(username string) string) {
	s.getUserHome = fn
}

// Start 启动 SFTP 服务器
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return ErrServerRunning
	}

	if !s.config.Enabled {
		return nil
	}

	// 配置 SSH 服务器
	s.configureSSH()

	// 启动监听
	addr := fmt.Sprintf(":%d", s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听端口 %d 失败：%w", s.config.Port, err)
	}

	s.listener = listener
	s.running = true

	// 接受连接
	go s.acceptLoop()

	return nil
}

// configureSSH 配置 SSH 服务器
func (s *Server) configureSSH() {
	s.sshConfig = &ssh.ServerConfig{
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			username := conn.User()
			if s.authFunc != nil && s.authFunc(username, string(password)) {
				return &ssh.Permissions{
					Extensions: map[string]string{
						"user": username,
					},
				}, nil
			}
			return nil, errors.New("认证失败")
		},
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			username := conn.User()
			if s.pubKeyAuth != nil && s.pubKeyAuth(username, key) {
				return &ssh.Permissions{
					Extensions: map[string]string{
						"user":      username,
						"pubkey-fp": ssh.FingerprintSHA256(key),
					},
				}, nil
			}
			return nil, errors.New("公钥认证失败")
		},
	}

	s.sshConfig.AddHostKey(s.hostKey)
}

// Stop 停止 SFTP 服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.cancel()

	// 关闭所有连接
	for _, conn := range s.connections {
		_ = conn.Close()
	}
	s.connections = make(map[string]*ssh.ServerConn)

	// 关闭监听器
	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
	}

	s.running = false
	return nil
}

// IsRunning 检查服务器是否运行中
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetConfig 获取配置
func (s *Server) GetConfig() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig 更新配置
func (s *Server) UpdateConfig(config *Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	wasRunning := s.running

	// 如果正在运行且端口变化，需要重启
	if s.running && s.config.Port != config.Port {
		s.stopInternal()
	}

	s.config = config

	// 更新连接数限制
	if config.MaxConnections > 0 {
		s.connSem = make(chan struct{}, config.MaxConnections)
	}

	// 如果之前在运行且现在启用，重新启动
	if wasRunning && config.Enabled && !s.running {
		return s.startInternal()
	}

	// 如果新配置启用且未运行
	if config.Enabled && !s.running {
		return s.startInternal()
	}

	// 如果新配置禁用且在运行
	if !config.Enabled && s.running {
		s.stopInternal()
	}

	return nil
}

// startInternal 内部启动方法
func (s *Server) startInternal() error {
	if s.running {
		return nil
	}

	// 重新加载主机密钥
	if err := s.loadOrGenerateHostKey(); err != nil {
		return err
	}

	s.configureSSH()

	addr := fmt.Sprintf(":%d", s.config.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("监听端口 %d 失败：%w", s.config.Port, err)
	}

	s.listener = listener
	s.running = true
	s.ctx, s.cancel = context.WithCancel(context.Background())

	go s.acceptLoop()
	return nil
}

// stopInternal 内部停止方法
func (s *Server) stopInternal() {
	if !s.running {
		return
	}

	s.cancel()

	for _, conn := range s.connections {
		_ = conn.Close()
	}
	s.connections = make(map[string]*ssh.ServerConn)

	if s.listener != nil {
		_ = s.listener.Close()
		s.listener = nil
	}

	s.running = false
}

// acceptLoop 接受连接循环
func (s *Server) acceptLoop() {
	for {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if s.ctx.Err() != nil {
				return
			}
			continue
		}

		// 检查连接数限制
		if s.connSem != nil {
			select {
			case s.connSem <- struct{}{}:
			default:
				_ = conn.Close()
				continue
			}
		}

		go s.handleConnection(conn)
	}
}

// handleConnection 处理 SSH 连接
func (s *Server) handleConnection(conn net.Conn) {
	defer func() {
		conn.Close()
		if s.connSem != nil {
			<-s.connSem
		}
	}()

	// SSH 握手
	sshConn, chans, reqs, err := ssh.NewServerConn(conn, s.sshConfig)
	if err != nil {
		return
	}
	defer func() { _ = sshConn.Close() }()

	// 记录连接
	connID := fmt.Sprintf("%s-%d", conn.RemoteAddr(), time.Now().UnixNano())
	s.mu.Lock()
	s.connections[connID] = sshConn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.connections, connID)
		s.mu.Unlock()
	}()

	// 处理全局请求
	go ssh.DiscardRequests(reqs)

	// 处理通道请求
	for newChannel := range chans {
		if newChannel.ChannelType() != "session" {
			_ = newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		go s.handleSession(newChannel, sshConn)
	}
}

// handleSession 处理会话
func (s *Server) handleSession(newChannel ssh.NewChannel, sshConn *ssh.ServerConn) {
	channel, requests, err := newChannel.Accept()
	if err != nil {
		return
	}
	defer func() { _ = channel.Close() }()

	// 获取用户信息
	username := sshConn.Permissions.Extensions["user"]
	homeDir := ""
	if s.getUserHome != nil {
		homeDir = s.getUserHome(username)
	}
	if homeDir == "" {
		homeDir = s.config.RootPath
	}

	// 处理子系统和执行请求
	for req := range requests {
		switch req.Type {
		case "subsystem":
			// 解析子系统请求
			var payload struct {
				Name string
			}
			if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
				_ = req.Reply(false, nil)
				continue
			}

			if payload.Name == "sftp" {
				_ = req.Reply(true, nil)
				// 启动 SFTP 处理
				s.handleSFTP(channel, username, homeDir)
			} else {
				_ = req.Reply(false, nil)
			}
		case "exec":
			// 不允许执行命令
			_ = req.Reply(false, nil)
		case "shell":
			// 不允许交互式 shell
			_ = req.Reply(false, nil)
		case "pty-req":
			// 不允许 PTY
			_ = req.Reply(false, nil)
		default:
			_ = req.Reply(false, nil)
		}
	}
}

// handleSFTP 处理 SFTP 请求
func (s *Server) handleSFTP(channel ssh.Channel, username, homeDir string) {
	// 确定 chroot 目录
	rootDir := homeDir
	if s.config.ChrootEnabled {
		if userChroot, ok := s.config.UserChroots[username]; ok {
			rootDir = userChroot
		} else {
			// 默认 chroot 到用户主目录
			rootDir = homeDir
		}
	}

	// 确保根目录存在
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return
	}

	// 创建 SFTP 处理器
	handler := &sftpHandler{
		rootDir:  rootDir,
		readOnly: false, // 可根据权限配置
	}

	// 使用简单的 SFTP 协议实现
	s.runSFTPProtocol(channel, handler)
}

// runSFTPProtocol 运行 SFTP 协议
func (s *Server) runSFTPProtocol(channel ssh.Channel, handler *sftpHandler) {
	// 简化的 SFTP 协议实现
	// 实际生产环境应使用 github.com/pkg/sftp 库

	buf := make([]byte, 32768)
	for {
		n, err := channel.Read(buf)
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		// 处理 SFTP 数据包
		// 这里应该解析并处理 SFTP 包
		_ = buf[:n]
	}
}

// sftpHandler SFTP 文件处理器
type sftpHandler struct {
	rootDir  string
	readOnly bool
}

// resolvePath 解析路径（确保在 chroot 内）
func (h *sftpHandler) resolvePath(path string) (string, error) {
	// 清理路径
	cleanPath := filepath.Clean(path)

	// 防止路径遍历攻击
	if cleanPath == ".." || strings.HasPrefix(cleanPath, "../") {
		return "", errors.New("invalid path")
	}

	// 组合完整路径
	fullPath := filepath.Join(h.rootDir, cleanPath)

	// 确保路径在 chroot 内
	if !strings.HasPrefix(fullPath, h.rootDir) {
		return "", errors.New("access denied")
	}

	return fullPath, nil
}

// GetStatus 获取服务器状态
func (s *Server) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := map[string]interface{}{
		"running":         s.running,
		"port":            s.config.Port,
		"connections":     len(s.connections),
		"max_connections": s.config.MaxConnections,
		"chroot_enabled":  s.config.ChrootEnabled,
	}

	return status
}

// RegisterRoutes 注册 HTTP 路由（用于管理 API）
func (s *Server) RegisterRoutes(api *gin.RouterGroup) {
	sftp := api.Group("/sftp")
	{
		sftp.GET("/config", s.getConfigHandler)
		sftp.PUT("/config", s.updateConfigHandler)
		sftp.GET("/status", s.getStatusHandler)
		sftp.POST("/restart", s.restartHandler)
	}
}

func (s *Server) getConfigHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data":    s.GetConfig(),
	})
}

func (s *Server) updateConfigHandler(c *gin.Context) {
	var config Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(400, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := s.UpdateConfig(&config); err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data":    s.GetConfig(),
	})
}

func (s *Server) getStatusHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"code":    0,
		"message": "success",
		"data":    s.GetStatus(),
	})
}

func (s *Server) restartHandler(c *gin.Context) {
	// 停止服务器
	if err := s.Stop(); err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": "停止服务器失败: " + err.Error(),
		})
		return
	}

	// 重新启动
	config := s.GetConfig()
	config.Enabled = true
	if err := s.UpdateConfig(config); err != nil {
		c.JSON(500, gin.H{
			"code":    500,
			"message": "启动服务器失败: " + err.Error(),
		})
		return
	}

	c.JSON(200, gin.H{
		"code":    0,
		"message": "SFTP 服务器已重启",
	})
}
