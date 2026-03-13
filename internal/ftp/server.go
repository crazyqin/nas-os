package ftp

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrServerNotRunning = errors.New("服务器未运行")
	ErrServerRunning    = errors.New("服务器已在运行")
	ErrTooManyConns     = errors.New("连接数超过限制")
	ErrLoginFailed      = errors.New("登录失败")
	ErrPermissionDenied = errors.New("权限被拒绝")
)

// Server FTP 服务器
type Server struct {
	mu            sync.RWMutex
	config        *Config
	listener      net.Listener
	clients       map[int]*clientConn
	nextClientID  int
	running       bool
	authFunc      func(username, password string) bool
	getUserHome   func(username string) string
	ctx           context.Context
	cancel        context.CancelFunc
	connSem       chan struct{} // 连接数限制信号量
	pasvListeners map[int]net.Listener // 被动模式监听器池
	pasvPortMutex sync.Mutex
}

// clientConn 客户端连接
type clientConn struct {
	id          int
	conn        net.Conn
	server      *Server
	remoteAddr  string
	user        string
	homeDir     string
	currentDir  string
	loggedIn    bool
	binaryMode  bool
	pasvListener net.Listener
	pasvPort    int
	pasvHost    string
	restOffset  int64
	lastCmd     time.Time
	reader      *bufio.Reader
	writer      *bufio.Writer
	closed      bool
}

// NewServer 创建 FTP 服务器
func NewServer(config *Config) (*Server, error) {
	if config == nil {
		config = DefaultConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	s := &Server{
		config:        config,
		clients:       make(map[int]*clientConn),
		ctx:           ctx,
		cancel:        cancel,
		pasvListeners: make(map[int]net.Listener),
	}

	// 初始化连接数限制
	if config.MaxConnections > 0 {
		s.connSem = make(chan struct{}, config.MaxConnections)
	}

	return s, nil
}

// SetAuthFunc 设置认证函数
func (s *Server) SetAuthFunc(fn func(username, password string) bool) {
	s.authFunc = fn
}

// SetGetUserHome 设置获取用户主目录函数
func (s *Server) SetGetUserHome(fn func(username string) string) {
	s.getUserHome = fn
}

// Start 启动 FTP 服务器
func (s *Server) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return ErrServerRunning
	}

	if !s.config.Enabled {
		return nil
	}

	// 确保根目录存在
	if err := os.MkdirAll(s.config.RootPath, 0755); err != nil {
		return fmt.Errorf("创建根目录失败：%w", err)
	}

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

// Stop 停止 FTP 服务器
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return nil
	}

	s.cancel()

	// 关闭所有客户端连接
	for _, client := range s.clients {
		client.close()
	}
	s.clients = make(map[int]*clientConn)

	// 关闭被动模式监听器
	for port, ln := range s.pasvListeners {
		ln.Close()
		delete(s.pasvListeners, port)
	}

	// 关闭主监听器
	if s.listener != nil {
		s.listener.Close()
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

// startInternal 内部启动方法（需要持有锁）
func (s *Server) startInternal() error {
	if s.running {
		return nil
	}

	// 确保根目录存在
	if err := os.MkdirAll(s.config.RootPath, 0755); err != nil {
		return fmt.Errorf("创建根目录失败：%w", err)
	}

	// 启动监听
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

// stopInternal 内部停止方法（需要持有锁）
func (s *Server) stopInternal() {
	if !s.running {
		return
	}

	s.cancel()

	for _, client := range s.clients {
		client.close()
	}
	s.clients = make(map[int]*clientConn)

	for port, ln := range s.pasvListeners {
		ln.Close()
		delete(s.pasvListeners, port)
	}

	if s.listener != nil {
		s.listener.Close()
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
				return // 服务器已停止
			}
			continue
		}

		// 检查连接数限制
		if s.connSem != nil {
			select {
			case s.connSem <- struct{}{}:
				// 获得槽位
			default:
				_, _ = conn.Write([]byte("421 Too many connections\r\n"))
				conn.Close()
				continue
			}
		}

		// 创建客户端连接
		go s.handleConnection(conn)
	}
}

// handleConnection 处理客户端连接
func (s *Server) handleConnection(conn net.Conn) {
	s.mu.Lock()
	s.nextClientID++
	client := &clientConn{
		id:         s.nextClientID,
		conn:       conn,
		server:     s,
		remoteAddr: conn.RemoteAddr().String(),
		currentDir: "/",
		binaryMode: false,
		lastCmd:    time.Now(),
		reader:     bufio.NewReader(conn),
		writer:     bufio.NewWriter(conn),
	}
	s.clients[client.id] = client
	s.mu.Unlock()

	// 清理
	defer func() {
		s.mu.Lock()
		delete(s.clients, client.id)
		if s.connSem != nil {
			<-s.connSem // 释放槽位
		}
		s.mu.Unlock()
		client.close()
	}()

	// 发送欢迎消息
	_ = client.writeResponse(220, "NAS-OS FTP Server Ready")

	// 命令处理循环
	client.handleCommands()
}

// close 关闭客户端连接
func (c *clientConn) close() {
	if c.closed {
		return
	}
	c.closed = true
	if c.pasvListener != nil {
		c.pasvListener.Close()
	}
	if c.conn != nil {
		c.conn.Close()
	}
}

// writeResponse 发送响应
func (c *clientConn) writeResponse(code int, message string) error {
	line := fmt.Sprintf("%d %s\r\n", code, message)
	_, err := c.writer.WriteString(line)
	if err != nil {
		return err
	}
	return c.writer.Flush()
}

// handleCommands 命令处理循环
func (c *clientConn) handleCommands() {
	for {
		select {
		case <-c.server.ctx.Done():
			return
		default:
		}

		// 设置读取超时
		_ = c.conn.SetReadDeadline(time.Now().Add(5 * time.Minute))

		line, err := c.reader.ReadString('\n')
		if err != nil {
			return // 连接关闭或错误
		}

		c.lastCmd = time.Now()

		// 解析命令
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		cmd := strings.ToUpper(parts[0])
		var args string
		if len(parts) > 1 {
			args = parts[1]
		}

		// 处理命令
		c.handleCommand(cmd, args)
	}
}

// handleCommand 处理单个命令
func (c *clientConn) handleCommand(cmd, args string) {
	// 未登录时只允许 USER, PASS, QUIT
	if !c.loggedIn && cmd != "USER" && cmd != "PASS" && cmd != "QUIT" && cmd != "SYST" && cmd != "FEAT" {
		_ = c.writeResponse(530, "Please login with USER and PASS")
		return
	}

	switch cmd {
	case "USER":
		c.handleUSER(args)
	case "PASS":
		c.handlePASS(args)
	case "QUIT":
		c.handleQUIT()
	case "SYST":
		_ = c.writeResponse(215, "UNIX Type: L8")
	case "FEAT":
		c.handleFEAT()
	case "PWD", "XPWD":
		c.handlePWD()
	case "CWD":
		c.handleCWD(args)
	case "CDUP":
		c.handleCDUP()
	case "MKD", "XMKD":
		c.handleMKD(args)
	case "RMD", "XRMD":
		c.handleRMD(args)
	case "DELE":
		c.handleDELE(args)
	case "RNFR":
		c.handleRNFR(args)
	case "RNTO":
		c.handleRNTO(args)
	case "LIST", "NLST":
		c.handleLIST(args)
	case "TYPE":
		c.handleTYPE(args)
	case "PASV":
		c.handlePASV()
	case "PORT":
		c.handlePORT(args)
	case "RETR":
		c.handleRETR(args)
	case "STOR":
		c.handleSTOR(args)
	case "REST":
		c.handleREST(args)
	case "SIZE":
		c.handleSIZE(args)
	case "ABOR":
		c.handleABOR()
	case "NOOP":
		_ = c.writeResponse(200, "OK")
	default:
		_ = c.writeResponse(500, fmt.Sprintf("Unknown command: %s", cmd))
	}
}

// handleUSER 处理 USER 命令
func (c *clientConn) handleUSER(username string) {
	if username == "" {
		_ = c.writeResponse(500, "USER requires a username")
		return
	}
	c.user = username
	_ = c.writeResponse(331, "Password required")
}

// handlePASS 处理 PASS 命令
func (c *clientConn) handlePASS(password string) {
	if c.user == "" {
		_ = c.writeResponse(503, "Login with USER first")
		return
	}

	// 匿名登录
	if c.user == "anonymous" || c.user == "ftp" {
		if c.server.config.AllowAnonymous {
			c.loggedIn = true
			c.homeDir = c.server.config.RootPath
			c.currentDir = "/"
			_ = c.writeResponse(230, "Anonymous login successful")
			return
		}
		_ = c.writeResponse(530, "Anonymous login not allowed")
		return
	}

	// 用户认证
	if c.server.authFunc != nil && c.server.authFunc(c.user, password) {
		c.loggedIn = true
		// 获取用户主目录
		if c.server.getUserHome != nil {
			c.homeDir = c.server.getUserHome(c.user)
		}
		if c.homeDir == "" {
			c.homeDir = c.server.config.RootPath
		}
		c.currentDir = "/"
		_ = c.writeResponse(230, "Login successful")
		return
	}

	_ = c.writeResponse(530, "Login failed")
	c.user = ""
}

// handleQUIT 处理 QUIT 命令
func (c *clientConn) handleQUIT() {
	_ = c.writeResponse(221, "Goodbye")
	c.close()
}

// handleFEAT 处理 FEAT 命令
func (c *clientConn) handleFEAT() {
	features := []string{
		" PASV",
		" PORT",
		" TYPE A I",
		" REST STREAM",
		" SIZE",
		" UTF8",
	}
	_ = c.writeResponse(211, "Features:")
	for _, f := range features {
		_ = c.writeResponse(0, f)
	}
	_ = c.writeResponse(211, "End")
}

// handlePWD 处理 PWD 命令
func (c *clientConn) handlePWD() {
	_ = c.writeResponse(257, fmt.Sprintf(`"%s" is current directory`, c.currentDir))
}

// handleCWD 处理 CWD 命令
func (c *clientConn) handleCWD(path string) {
	realPath := c.resolvePath(path)

	info, err := os.Stat(realPath)
	if err != nil || !info.IsDir() {
		_ = c.writeResponse(550, "Directory not found")
		return
	}

	c.currentDir = c.normalizePath(path)
	_ = c.writeResponse(250, "Directory changed")
}

// handleCDUP 处理 CDUP 命令
func (c *clientConn) handleCDUP() {
	if c.currentDir == "/" {
		_ = c.writeResponse(250, "Directory not changed")
		return
	}
	c.currentDir = filepath.Dir(c.currentDir)
	if c.currentDir == "." {
		c.currentDir = "/"
	}
	_ = c.writeResponse(250, "Directory changed")
}

// handleMKD 处理 MKD 命令
func (c *clientConn) handleMKD(path string) {
	realPath := c.resolvePath(path)

	if err := os.MkdirAll(realPath, 0755); err != nil {
		_ = c.writeResponse(550, fmt.Sprintf("Failed to create directory: %v", err))
		return
	}

	_ = c.writeResponse(257, fmt.Sprintf(`"%s" created`, path))
}

// handleRMD 处理 RMD 命令
func (c *clientConn) handleRMD(path string) {
	realPath := c.resolvePath(path)

	if err := os.RemoveAll(realPath); err != nil {
		_ = c.writeResponse(550, fmt.Sprintf("Failed to remove directory: %v", err))
		return
	}

	_ = c.writeResponse(250, "Directory removed")
}

// handleDELE 处理 DELE 命令
func (c *clientConn) handleDELE(path string) {
	realPath := c.resolvePath(path)

	if err := os.Remove(realPath); err != nil {
		_ = c.writeResponse(550, fmt.Sprintf("Failed to delete file: %v", err))
		return
	}

	_ = c.writeResponse(250, "File deleted")
}

var renameFrom string

// handleRNFR 处理 RNFR 命令
func (c *clientConn) handleRNFR(path string) {
	realPath := c.resolvePath(path)

	if _, err := os.Stat(realPath); err != nil {
		_ = c.writeResponse(550, "File not found")
		return
	}

	renameFrom = realPath
	_ = c.writeResponse(350, "Ready for RNTO")
}

// handleRNTO 处理 RNTO 命令
func (c *clientConn) handleRNTO(path string) {
	if renameFrom == "" {
		_ = c.writeResponse(503, "RNFR required first")
		return
	}

	realPath := c.resolvePath(path)

	if err := os.Rename(renameFrom, realPath); err != nil {
		_ = c.writeResponse(550, fmt.Sprintf("Failed to rename: %v", err))
		renameFrom = ""
		return
	}

	renameFrom = ""
	_ = c.writeResponse(250, "Rename successful")
}

// handleLIST 处理 LIST 命令
func (c *clientConn) handleLIST(path string) {
	realPath := c.resolvePath(path)
	if path == "" {
		realPath = c.resolvePath(c.currentDir)
	}

	// 获取数据连接
	dataConn, err := c.getDataConnection()
	if err != nil {
		_ = c.writeResponse(425, fmt.Sprintf("Failed to establish data connection: %v", err))
		return
	}
	defer dataConn.Close()

	_ = c.writeResponse(150, "Opening data connection for listing")

	// 列出目录
	entries, err := os.ReadDir(realPath)
	if err != nil {
		_ = c.writeResponse(550, fmt.Sprintf("Failed to list directory: %v", err))
		return
	}

	writer := bufio.NewWriter(dataConn)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Unix 风格的列表格式
		line := c.formatFileInfo(entry.Name(), info)
		writer.WriteString(line + "\r\n")
	}
	writer.Flush()

	_ = c.writeResponse(226, "Transfer complete")
}

// formatFileInfo 格式化文件信息
func (c *clientConn) formatFileInfo(name string, info os.FileInfo) string {
	// 类似 ls -l 的格式
	_ = info.Mode() // 用于将来可能的权限格式化
	var perm string
	if info.IsDir() {
		perm = "drwxr-xr-x"
	} else {
		perm = "-rw-r--r--"
	}

	// 大小
	size := info.Size()

	// 修改时间
	modTime := info.ModTime().Format("Jan _2 15:04")

	return fmt.Sprintf("%s 1 ftp ftp %12d %s %s", perm, size, modTime, name)
}

// handleTYPE 处理 TYPE 命令
func (c *clientConn) handleTYPE(typ string) {
	switch strings.ToUpper(typ) {
	case "A":
		c.binaryMode = false
		_ = c.writeResponse(200, "Type set to ASCII")
	case "I":
		c.binaryMode = true
		_ = c.writeResponse(200, "Type set to Binary")
	default:
		_ = c.writeResponse(500, fmt.Sprintf("Unsupported type: %s", typ))
	}
}

// handlePASV 处理 PASV 命令（被动模式）
func (c *clientConn) handlePASV() {
	port, listener, err := c.allocatePasvPort()
	if err != nil {
		_ = c.writeResponse(425, fmt.Sprintf("Failed to enter passive mode: %v", err))
		return
	}

	c.pasvListener = listener
	c.pasvPort = port

	// 获取被动模式 IP
	pasvHost := c.server.config.PasvHost
	if pasvHost == "" {
		// 从控制连接获取本地 IP
		localAddr := c.conn.LocalAddr().String()
		if idx := strings.LastIndex(localAddr, ":"); idx != -1 {
			pasvHost = localAddr[:idx]
		} else {
			pasvHost = "127.0.0.1"
		}
	}

	// 格式化 IP 和端口
	ip := net.ParseIP(pasvHost)
	if ip == nil {
		ip = net.ParseIP("127.0.0.1")
	}

	// IPv4 模式
	ip = ip.To4()
	if ip == nil {
		ip = net.ParseIP("127.0.0.1").To4()
	}

	p1 := port / 256
	p2 := port % 256

	response := fmt.Sprintf("Entering Passive Mode (%d,%d,%d,%d,%d,%d)",
		ip[0], ip[1], ip[2], ip[3], p1, p2)

	_ = c.writeResponse(227, response)
}

// allocatePasvPort 分配被动模式端口
func (c *clientConn) allocatePasvPort() (int, net.Listener, error) {
	c.server.pasvPortMutex.Lock()
	defer c.server.pasvPortMutex.Unlock()

	start := c.server.config.PasvPortStart
	end := c.server.config.PasvPortEnd

	for port := start; port <= end; port++ {
		if _, used := c.server.pasvListeners[port]; used {
			continue
		}

		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			continue
		}

		c.server.pasvListeners[port] = listener
		return port, listener, nil
	}

	return 0, nil, errors.New("no available passive port")
}

// handlePORT 处理 PORT 命令（主动模式）
func (c *clientConn) handlePORT(args string) {
	// 解析 PORT 参数: h1,h2,h3,h4,p1,p2
	parts := strings.Split(args, ",")
	if len(parts) != 6 {
		_ = c.writeResponse(500, "Invalid PORT format")
		return
	}

	var bytes []byte
	for _, p := range parts {
		b, err := strconv.Atoi(p)
		if err != nil {
			_ = c.writeResponse(500, "Invalid PORT format")
			return
		}
		bytes = append(bytes, byte(b))
	}

	ip := fmt.Sprintf("%d.%d.%d.%d", bytes[0], bytes[1], bytes[2], bytes[3])
	port := int(bytes[4])*256 + int(bytes[5])

	// 关闭之前的被动模式监听器
	if c.pasvListener != nil {
		c.pasvListener.Close()
		c.pasvListener = nil
	}

	// 存储主动模式地址
	c.pasvPort = -port // 负数表示主动模式
	c.pasvHost = ip

	_ = c.writeResponse(200, "PORT command successful")
}

// handleRETR 处理 RETR 命令（下载文件）
func (c *clientConn) handleRETR(path string) {
	realPath := c.resolvePath(path)

	// 检查文件是否存在
	info, err := os.Stat(realPath)
	if err != nil {
		_ = c.writeResponse(550, "File not found")
		return
	}

	if info.IsDir() {
		_ = c.writeResponse(550, "Not a file")
		return
	}

	// 获取数据连接
	dataConn, err := c.getDataConnection()
	if err != nil {
		_ = c.writeResponse(425, fmt.Sprintf("Failed to establish data connection: %v", err))
		return
	}
	defer dataConn.Close()

	_ = c.writeResponse(150, "Opening data connection for file transfer")

	// 打开文件
	file, err := os.Open(realPath)
	if err != nil {
		_ = c.writeResponse(550, fmt.Sprintf("Failed to open file: %v", err))
		return
	}
	defer file.Close()

	// 断点续传
	if c.restOffset > 0 {
		_, _ = file.Seek(c.restOffset, io.SeekStart)
		c.restOffset = 0
	}

	// 带限速的复制
	written, err := c.copyWithLimit(dataConn, file, false)
	if err != nil {
		_ = c.writeResponse(426, fmt.Sprintf("Transfer aborted: %v", err))
		return
	}

	_ = written
	_ = c.writeResponse(226, "Transfer complete")
}

// handleSTOR 处理 STOR 命令（上传文件）
func (c *clientConn) handleSTOR(path string) {
	realPath := c.resolvePath(path)

	// 获取数据连接
	dataConn, err := c.getDataConnection()
	if err != nil {
		_ = c.writeResponse(425, fmt.Sprintf("Failed to establish data connection: %v", err))
		return
	}
	defer dataConn.Close()

	_ = c.writeResponse(150, "Opening data connection for file transfer")

	// 创建文件
	file, err := os.Create(realPath)
	if err != nil {
		_ = c.writeResponse(550, fmt.Sprintf("Failed to create file: %v", err))
		return
	}
	defer file.Close()

	// 断点续传
	if c.restOffset > 0 {
		_, _ = file.Seek(c.restOffset, io.SeekStart)
		c.restOffset = 0
	}

	// 带限速的复制
	written, err := c.copyWithLimit(file, dataConn, true)
	if err != nil {
		_ = c.writeResponse(426, fmt.Sprintf("Transfer aborted: %v", err))
		return
	}

	_ = written
	_ = c.writeResponse(226, "Transfer complete")
}

// handleREST 处理 REST 命令（断点续传）
func (c *clientConn) handleREST(offset string) {
	o, err := strconv.ParseInt(offset, 10, 64)
	if err != nil {
		_ = c.writeResponse(500, "Invalid offset")
		return
	}
	c.restOffset = o
	_ = c.writeResponse(350, fmt.Sprintf("Restarting at %d", o))
}

// handleSIZE 处理 SIZE 命令
func (c *clientConn) handleSIZE(path string) {
	realPath := c.resolvePath(path)

	info, err := os.Stat(realPath)
	if err != nil {
		_ = c.writeResponse(550, "File not found")
		return
	}

	if info.IsDir() {
		_ = c.writeResponse(550, "Not a file")
		return
	}

	_ = c.writeResponse(213, fmt.Sprintf("%d", info.Size()))
}

// handleABOR 处理 ABOR 命令
func (c *clientConn) handleABOR() {
	c.restOffset = 0
	_ = c.writeResponse(226, "Abort successful")
}

// getDataConnection 获取数据连接
func (c *clientConn) getDataConnection() (net.Conn, error) {
	if c.pasvListener != nil {
		// 被动模式：等待客户端连接
		_ = c.pasvListener.(*net.TCPListener).SetDeadline(time.Now().Add(30 * time.Second))
		conn, err := c.pasvListener.Accept()
		if err != nil {
			return nil, err
		}

		// 释放端口
		c.server.pasvPortMutex.Lock()
		delete(c.server.pasvListeners, c.pasvPort)
		c.server.pasvPortMutex.Unlock()
		c.pasvListener = nil

		return conn, nil
	}

	if c.pasvPort < 0 {
		// 主动模式：连接到客户端
		port := -c.pasvPort
		addr := net.JoinHostPort(c.pasvHost, strconv.Itoa(port))
		conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
		if err != nil {
			return nil, err
		}
		return conn, nil
	}

	return nil, errors.New("no data connection available")
}

// resolvePath 解析路径（支持虚拟目录）
func (c *clientConn) resolvePath(path string) string {
	// 标准化路径
	if !strings.HasPrefix(path, "/") {
		path = filepath.Join(c.currentDir, path)
	}

	// 检查虚拟目录映射
	c.server.mu.RLock()
	virtualDirs := c.server.config.VirtualDirs
	c.server.mu.RUnlock()

	for virtual, real := range virtualDirs {
		if strings.HasPrefix(path, virtual) {
			relPath := strings.TrimPrefix(path, virtual)
			return filepath.Join(real, relPath)
		}
	}

	// 默认映射到用户主目录
	return filepath.Join(c.homeDir, path)
}

// normalizePath 标准化路径
func (c *clientConn) normalizePath(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = filepath.Join(c.currentDir, path)
	}
	return filepath.Clean(path)
}

// copyWithLimit 带限速的复制
func (c *clientConn) copyWithLimit(dst io.Writer, src io.Reader, isUpload bool) (int64, error) {
	c.server.mu.RLock()
	bwLimit := c.server.config.BandwidthLimit
	c.server.mu.RUnlock()

	if !bwLimit.Enabled {
		return io.Copy(dst, src)
	}

	var limit int64
	if isUpload {
		limit = bwLimit.UploadKBps
	} else {
		limit = bwLimit.DownloadKBps
	}

	if limit <= 0 {
		return io.Copy(dst, src)
	}

	// 限速复制
	buf := make([]byte, 32*1024)
	total := int64(0)
	start := time.Now()

	for {
		n, err := src.Read(buf)
		if n > 0 {
			written, err2 := dst.Write(buf[:n])
			if err2 != nil {
				return total + int64(written), err2
			}
			total += int64(written)

			// 计算等待时间
			expected := float64(total) / float64(limit*1024) // 秒
			elapsed := time.Since(start).Seconds()
			if expected > elapsed {
				time.Sleep(time.Duration((expected - elapsed) * float64(time.Second)))
			}
		}
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			return total, err
		}
	}
}

// GetStatus 获取服务器状态
func (s *Server) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := map[string]interface{}{
		"running":      s.running,
		"port":         s.config.Port,
		"connections":  len(s.clients),
		"max_connections": s.config.MaxConnections,
	}

	if s.running {
		status["uptime"] = time.Since(s.clients[0].lastCmd).String() // 简化
	}

	return status
}