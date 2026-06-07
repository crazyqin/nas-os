// Package cloudflare 提供Cloudflare Tunnel集成
// 实现无端口远程访问，类似飞牛fnOS FN Connect
package cloudflare

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TunnelManager Cloudflare Tunnel管理器
type TunnelManager struct {
	config     *TunnelConfig
	logger     *zap.Logger
	httpClient *http.Client

	// Tunnel状态
	tunnelID  string
	publicURL string
	state     TunnelState
	connected bool

	// Tunnel进程
	cloudflaredPath string
	cloudflaredCmd  *exec.Cmd
	cloudflaredMu   sync.Mutex

	// 事件处理
	eventHandlers []TunnelEventHandler
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
}

// TunnelConfig 配置
type TunnelConfig struct {
	// 认证配置
	APIToken    string `json:"api_token"`    // Cloudflare API Token
	CertPath    string `json:"cert_path"`    // 证书路径(.cert文件)
	TunnelToken string `json:"tunnel_token"` // Tunnel Token (快速配置方式)

	// Tunnel配置
	TunnelName string            `json:"tunnel_name"` // Tunnel名称
	AccountID  string            `json:"account_id"`  // Cloudflare Account ID
	ZoneID     string            `json:"zone_id"`     // Zone ID
	Domain     string            `json:"domain"`      // 绑定域名
	Subdomain  string            `json:"subdomain"`   // 子域名
	Origins    map[string]string `json:"origins"`     // 路由配置: hostname -> origin URL

	// 运行配置
	AutoStart    bool          `json:"auto_start"`    // 自动启动
	Reconnect    bool          `json:"reconnect"`     // 断线重连
	ReconnectInt time.Duration `json:"reconnect_int"` // 重连间隔
	MaxRetries   int           `json:"max_retries"`   // 最大重试次数
	Timeout      time.Duration `json:"timeout"`       // 操作超时

	// 监控配置
	MetricsPort int  `json:"metrics_port"` // Metrics端口
	HealthCheck bool `json:"health_check"` // 健康检查
}

// TunnelState 状态
type TunnelState string

const (
	TunnelStateDisconnected TunnelState = "disconnected"
	TunnelStateConnecting   TunnelState = "connecting"
	TunnelStateConnected    TunnelState = "connected"
	TunnelStateReconnecting TunnelState = "reconnecting"
	TunnelStateError        TunnelState = "error"
	TunnelStateStopped      TunnelState = "stopped"
)

// TunnelEvent 事件
type TunnelEvent struct {
	Type      string      `json:"type"`
	State     TunnelState `json:"state,omitempty"`
	TunnelID  string      `json:"tunnel_id,omitempty"`
	PublicURL string      `json:"public_url,omitempty"`
	Error     error       `json:"error,omitempty"`
	Message   string      `json:"message,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// TunnelEventHandler 事件处理器
type TunnelEventHandler func(event *TunnelEvent)

// TunnelStats 统计信息
type TunnelStats struct {
	State         TunnelState   `json:"state"`
	TunnelID      string        `json:"tunnel_id"`
	PublicURL     string        `json:"public_url"`
	Connections   int           `json:"connections"`
	BytesTx       int64         `json:"bytes_tx"`
	BytesRx       int64         `json:"bytes_rx"`
	Uptime        time.Duration `json:"uptime"`
	LastConnected time.Time     `json:"last_connected"`
	Reconnects    int           `json:"reconnects"`
	Errors        int           `json:"errors"`
}

// TunnelRoute 路由配置
type TunnelRoute struct {
	Hostname string `json:"hostname"` // 公网域名
	Path     string `json:"path"`     // URL路径（可选）
	Service  string `json:"service"`  // 本地服务地址 (http://localhost:port)
}

// TunnelInfo Tunnel信息
type TunnelInfo struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	CreatedAt   time.Time     `json:"created_at"`
	Status      string        `json:"status"`
	Connections []Connection  `json:"connections,omitempty"`
	Routes      []TunnelRoute `json:"routes"`
}

// Connection 连接信息
type Connection struct {
	ID          string    `json:"id"`
	ColoID      string    `json:"colo_id"`   // 数据中心ID
	ColoName    string    `json:"colo_name"` // 数据中心名称
	OriginIP    string    `json:"origin_ip"` // 源IP
	ConnectedAt time.Time `json:"connected_at"`
	IsHealthy   bool      `json:"is_healthy"`
}

var (
	ErrNotInstalled     = errors.New("cloudflared not installed")
	ErrNotConnected     = errors.New("tunnel not connected")
	ErrAlreadyConnected = errors.New("tunnel already connected")
	ErrAuthFailed       = errors.New("authentication failed")
	ErrCreateFailed     = errors.New("failed to create tunnel")
	ErrConfigFailed     = errors.New("failed to configure tunnel")
	ErrStartFailed      = errors.New("failed to start tunnel")
)

// NewTunnelManager 创建Tunnel管理器
func NewTunnelManager(config *TunnelConfig, logger *zap.Logger) (*TunnelManager, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())

	mgr := &TunnelManager{
		config:     config,
		logger:     logger,
		httpClient: httpClient,
		state:      TunnelStateDisconnected,
		ctx:        ctx,
		cancel:     cancel,
	}

	// 检查cloudflared安装
	if err := mgr.checkInstallation(); err != nil {
		return nil, err
	}

	return mgr, nil
}

// DefaultTunnelConfig 默认配置
func DefaultTunnelConfig() *TunnelConfig {
	return &TunnelConfig{
		AutoStart:    false,
		Reconnect:    true,
		ReconnectInt: 5 * time.Second,
		MaxRetries:   10,
		Timeout:      30 * time.Second,
		MetricsPort:  50000,
		HealthCheck:  true,
		Origins:      make(map[string]string),
	}
}

// checkInstallation 检查cloudflared安装
func (tm *TunnelManager) checkInstallation() error {
	// 检查常见安装路径
	paths := []string{
		"/usr/local/bin/cloudflared",
		"/usr/bin/cloudflared",
		"/opt/cloudflared/cloudflared",
		"/home/linuxbrew/.linuxbrew/bin/cloudflared",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			tm.cloudflaredPath = p
			return nil
		}
	}

	// 检查PATH
	if path, err := exec.LookPath("cloudflared"); err == nil {
		tm.cloudflaredPath = path
		return nil
	}

	return ErrNotInstalled
}

// Install 安装cloudflared
func (tm *TunnelManager) Install(ctx context.Context) error {
	// 检查是否已安装
	if tm.cloudflaredPath != "" {
		return nil
	}

	tm.logger.Info("安装cloudflared")

	// 使用Cloudflare官方安装脚本
	cmd := exec.CommandContext(ctx, "bash", "-c",
		"curl -L https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-arm64 -o /tmp/cloudflared && "+
			"chmod +x /tmp/cloudflared && "+
			"sudo mv /tmp/cloudflared /usr/local/bin/cloudflared")

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("安装失败: %w, output: %s", err, output)
	}

	// 验证安装
	if err := tm.checkInstallation(); err != nil {
		return err
	}

	tm.logger.Info("cloudflared安装成功", zap.String("path", tm.cloudflaredPath))
	return nil
}

// Authenticate 认证
func (tm *TunnelManager) Authenticate(ctx context.Context) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// 如果已有Tunnel Token，跳过认证
	if tm.config.TunnelToken != "" {
		return nil
	}

	// 如果已有证书，跳过认证
	if tm.config.CertPath != "" {
		if _, err := os.Stat(tm.config.CertPath); err == nil {
			return nil
		}
	}

	tm.logger.Info("开始Cloudflare认证")

	// 使用cloudflared login
	cmd := exec.CommandContext(ctx, tm.cloudflaredPath, "tunnel", "login")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return ErrAuthFailed
	}

	// 查找生成的证书
	homeDir, _ := os.UserHomeDir()
	certPath := filepath.Join(homeDir, ".cloudflared", "cert.pem")
	if _, err := os.Stat(certPath); err == nil {
		tm.config.CertPath = certPath
	}

	return nil
}

// CreateTunnel 创建Tunnel
func (tm *TunnelManager) CreateTunnel(ctx context.Context, name string) (*TunnelInfo, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.config.CertPath == "" && tm.config.APIToken == "" && tm.config.TunnelToken == "" {
		return nil, ErrAuthFailed
	}

	tm.logger.Info("创建Tunnel", zap.String("name", name))

	args := []string{"tunnel", "create", name}

	// 使用API Token
	if tm.config.APIToken != "" {
		args = []string{"tunnel", "--api-token", tm.config.APIToken, "create", name}
	}

	cmd := exec.CommandContext(ctx, tm.cloudflaredPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrCreateFailed, err.Error())
	}

	// 解析输出获取Tunnel ID
	// 输出格式: "Created tunnel <name> with id <id>"
	outputStr := string(output)
	parts := strings.Split(outputStr, " ")
	var tunnelID string
	for i, p := range parts {
		if p == "id" && i+1 < len(parts) {
			tunnelID = strings.TrimSpace(parts[i+1])
			break
		}
	}

	if tunnelID == "" {
		return nil, ErrCreateFailed
	}

	tm.tunnelID = tunnelID
	tm.config.TunnelName = name

	// 创建配置文件
	if err := tm.createConfigFile(); err != nil {
		return nil, err
	}

	return &TunnelInfo{
		ID:        tunnelID,
		Name:      name,
		CreatedAt: time.Now(),
		Status:    "created",
	}, nil
}

// createConfigFile 创建配置文件
func (tm *TunnelManager) createConfigFile() error {
	if tm.tunnelID == "" {
		return ErrConfigFailed
	}

	homeDir, _ := os.UserHomeDir()
	configDir := filepath.Join(homeDir, ".cloudflared")
	configPath := filepath.Join(configDir, fmt.Sprintf("%s.yml", tm.tunnelID))

	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// 构建配置
	config := map[string]interface{}{
		"tunnel":           tm.tunnelID,
		"credentials-file": filepath.Join(configDir, fmt.Sprintf("%s.json", tm.tunnelID)),
	}

	// 添加路由配置
	if len(tm.config.Origins) > 0 {
		routes := make([]map[string]string, 0)
		for hostname, origin := range tm.config.Origins {
			routes = append(routes, map[string]string{
				"hostname": hostname,
				"service":  origin,
			})
		}
		config["ingress"] = routes
	}

	// 拒绝其他请求
	if len(tm.config.Origins) > 0 {
		config["ingress"] = append(config["ingress"].([]map[string]string), map[string]string{
			"service": "http_status:404",
		})
	}

	data, err := yamlMarshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// yamlMarshal 简单YAML序列化
func yamlMarshal(v interface{}) ([]byte, error) {
	// 简化实现，实际应使用yaml库
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	// 转换为YAML风格（简化处理）
	return data, nil
}

// Start 启动Tunnel
func (tm *TunnelManager) Start(ctx context.Context) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.connected {
		return ErrAlreadyConnected
	}

	// 使用Tunnel Token启动（最简单方式）
	if tm.config.TunnelToken != "" {
		return tm.startWithToken(ctx)
	}

	// 使用Tunnel ID启动
	if tm.tunnelID != "" {
		return tm.startWithTunnelID(ctx)
	}

	return ErrNotConnected
}

// startWithToken 使用Token启动
func (tm *TunnelManager) startWithToken(ctx context.Context) error {
	tm.logger.Info("使用Tunnel Token启动")

	args := []string{"tunnel", "--token", tm.config.TunnelToken, "run"}

	cmd := exec.CommandContext(ctx, tm.cloudflaredPath, args...)
	cmd.Stdout = &tunnelLogWriter{tm: tm, prefix: "OUT"}
	cmd.Stderr = &tunnelLogWriter{tm: tm, prefix: "ERR"}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %s", ErrStartFailed, err.Error())
	}

	tm.cloudflaredCmd = cmd
	tm.state = TunnelStateConnecting

	// 监控进程
	go tm.monitorProcess()

	// 等待连接
	select {
	case <-time.After(tm.config.Timeout):
		// 检查是否已连接
		if tm.state == TunnelStateConnected {
			return nil
		}
		cmd.Process.Kill()
		return ErrStartFailed
	case <-tm.ctx.Done():
		return tm.ctx.Err()
	}
}

// startWithTunnelID 使用Tunnel ID启动
func (tm *TunnelManager) startWithTunnelID(ctx context.Context) error {
	tm.logger.Info("使用Tunnel ID启动", zap.String("id", tm.tunnelID))

	args := []string{"tunnel", "run", tm.tunnelID}

	if tm.config.APIToken != "" {
		args = []string{"tunnel", "--api-token", tm.config.APIToken, "run", tm.tunnelID}
	}

	// 指定配置文件
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".cloudflared", fmt.Sprintf("%s.yml", tm.tunnelID))
	if _, err := os.Stat(configPath); err == nil {
		args = append([]string{"--config", configPath}, args...)
	}

	cmd := exec.CommandContext(ctx, tm.cloudflaredPath, args...)
	cmd.Stdout = &tunnelLogWriter{tm: tm, prefix: "OUT"}
	cmd.Stderr = &tunnelLogWriter{tm: tm, prefix: "ERR"}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%w: %s", ErrStartFailed, err.Error())
	}

	tm.cloudflaredCmd = cmd
	tm.state = TunnelStateConnecting

	go tm.monitorProcess()

	select {
	case <-time.After(tm.config.Timeout):
		if tm.state == TunnelStateConnected {
			return nil
		}
		cmd.Process.Kill()
		return ErrStartFailed
	case <-tm.ctx.Done():
		return tm.ctx.Err()
	}
}

// monitorProcess 监控进程
func (tm *TunnelManager) monitorProcess() {
	for {
		select {
		case <-tm.ctx.Done():
			return
		case <-time.After(5 * time.Second):
			tm.cloudflaredMu.Lock()
			if tm.cloudflaredCmd != nil && tm.cloudflaredCmd.Process != nil {
				// 检查进程状态
				if err := tm.cloudflaredCmd.Process.Signal(os.Signal(nil)); err != nil {
					tm.logger.Warn("cloudflared进程异常", zap.Error(err))
					tm.handleDisconnect()
				} else if tm.state == TunnelStateConnecting {
					// 检查是否已连接（通过metrics）
					tm.checkConnection()
				}
			}
			tm.cloudflaredMu.Unlock()
		}
	}
}

// checkConnection 检查连接状态
func (tm *TunnelManager) checkConnection() {
	if tm.config.MetricsPort == 0 {
		return
	}

	metricsURL := fmt.Sprintf("http://localhost:%d/metrics", tm.config.MetricsPort)
	resp, err := tm.httpClient.Get(metricsURL)
	if err == nil && resp.StatusCode == 200 {
		_ = resp.Body.Close() // 关闭响应体
		tm.mu.Lock()
		tm.state = TunnelStateConnected
		tm.connected = true
		tm.mu.Unlock()

		tm.emitEvent(&TunnelEvent{
			Type:  "connected",
			State: TunnelStateConnected,
		})
	}
}

// handleDisconnect 处理断开
func (tm *TunnelManager) handleDisconnect() {
	tm.mu.Lock()
	tm.state = TunnelStateDisconnected
	tm.connected = false
	tm.mu.Unlock()

	tm.emitEvent(&TunnelEvent{
		Type:  "disconnected",
		State: TunnelStateDisconnected,
	})

	// 自动重连
	if tm.config.Reconnect {
		go tm.reconnect()
	}
}

// reconnect 重连
func (tm *TunnelManager) reconnect() {
	for retry := 0; retry < tm.config.MaxRetries; retry++ {
		tm.mu.Lock()
		tm.state = TunnelStateReconnecting
		tm.mu.Unlock()

		tm.emitEvent(&TunnelEvent{
			Type:    "reconnecting",
			State:   TunnelStateReconnecting,
			Message: fmt.Sprintf("重连尝试 %d/%d", retry+1, tm.config.MaxRetries),
		})

		select {
		case <-tm.ctx.Done():
			return
		case <-time.After(tm.config.ReconnectInt):
			if err := tm.Start(tm.ctx); err == nil {
				tm.mu.Lock()
				tm.state = TunnelStateConnected
				tm.connected = true
				tm.mu.Unlock()
				return
			}
		}
	}

	tm.mu.Lock()
	tm.state = TunnelStateError
	tm.mu.Unlock()

	tm.emitEvent(&TunnelEvent{
		Type:    "error",
		State:   TunnelStateError,
		Error:   errors.New("reconnect failed"),
		Message: "达到最大重试次数",
	})
}

// Stop 停止Tunnel
func (tm *TunnelManager) Stop() error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.cancel()

	tm.cloudflaredMu.Lock()
	if tm.cloudflaredCmd != nil && tm.cloudflaredCmd.Process != nil {
		tm.cloudflaredCmd.Process.Kill()
		tm.cloudflaredCmd.Wait()
		tm.cloudflaredCmd = nil
	}
	tm.cloudflaredMu.Unlock()

	tm.state = TunnelStateStopped
	tm.connected = false

	tm.emitEvent(&TunnelEvent{
		Type:  "stopped",
		State: TunnelStateStopped,
	})

	return nil
}

// AddRoute 添加路由
func (tm *TunnelManager) AddRoute(hostname, service string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if tm.config.Origins == nil {
		tm.config.Origins = make(map[string]string)
	}
	tm.config.Origins[hostname] = service

	// 更新配置文件
	if tm.tunnelID != "" {
		return tm.createConfigFile()
	}

	return nil
}

// RemoveRoute 移除路由
func (tm *TunnelManager) RemoveRoute(hostname string) error {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	delete(tm.config.Origins, hostname)

	if tm.tunnelID != "" {
		return tm.createConfigFile()
	}

	return nil
}

// GetPublicURL 获取公网地址
func (tm *TunnelManager) GetPublicURL() string {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.publicURL
}

// GetState 获取状态
func (tm *TunnelManager) GetState() TunnelState {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.state
}

// GetStats 获取统计
func (tm *TunnelManager) GetStats() *TunnelStats {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	return &TunnelStats{
		State:     tm.state,
		TunnelID:  tm.tunnelID,
		PublicURL: tm.publicURL,
	}
}

// IsConnected 是否已连接
func (tm *TunnelManager) IsConnected() bool {
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	return tm.connected
}

// ListTunnels 列出所有Tunnel
func (tm *TunnelManager) ListTunnels(ctx context.Context) ([]TunnelInfo, error) {
	args := []string{"tunnel", "list"}

	if tm.config.APIToken != "" {
		args = []string{"tunnel", "--api-token", tm.config.APIToken, "list"}
	}

	cmd := exec.CommandContext(ctx, tm.cloudflaredPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// 解析输出
	lines := strings.Split(string(output), "\n")
	tunnels := make([]TunnelInfo, 0)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "ID") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 3 {
			tunnels = append(tunnels, TunnelInfo{
				ID:        fields[0],
				Name:      fields[1],
				CreatedAt: time.Now(),
				Status:    fields[2],
			})
		}
	}

	return tunnels, nil
}

// DeleteTunnel 删除Tunnel
func (tm *TunnelManager) DeleteTunnel(ctx context.Context, tunnelID string) error {
	args := []string{"tunnel", "delete", tunnelID}

	if tm.config.APIToken != "" {
		args = []string{"tunnel", "--api-token", tm.config.APIToken, "delete", tunnelID}
	}

	cmd := exec.CommandContext(ctx, tm.cloudflaredPath, args...)
	return cmd.Run()
}

// OnEvent 注册事件处理器
func (tm *TunnelManager) OnEvent(handler TunnelEventHandler) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.eventHandlers = append(tm.eventHandlers, handler)
}

// emitEvent 发送事件
func (tm *TunnelManager) emitEvent(event *TunnelEvent) {
	event.Timestamp = time.Now()
	for _, handler := range tm.eventHandlers {
		go handler(event)
	}
}

// tunnelLogWriter 日志写入器
type tunnelLogWriter struct {
	tm     *TunnelManager
	prefix string
}

func (w *tunnelLogWriter) Write(p []byte) (n int, err error) {
	w.tm.logger.Debug("cloudflared",
		zap.String("prefix", w.prefix),
		zap.String("output", string(p)))

	// 检测连接成功消息
	if strings.Contains(string(p), "Connection registered") ||
		strings.Contains(string(p), "Tunnel established") {
		w.tm.mu.Lock()
		w.tm.state = TunnelStateConnected
		w.tm.connected = true
		w.tm.mu.Unlock()

		w.tm.emitEvent(&TunnelEvent{
			Type:  "connected",
			State: TunnelStateConnected,
		})
	}

	return len(p), nil
}
