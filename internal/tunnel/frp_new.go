// Package tunnel 提供 FRP 提供商实现
package tunnel

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ========== FRP Tunnel Instance ==========

// FRPTunnelInstance FRP 隧道实例
type FRPTunnelInstance struct {
	config    FRPTunnelConfig
	logger    *zap.Logger
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// 运行状态
	running     bool
	process     *exec.Cmd
	frpcPath    string
	publicAddr  string
	connStatus  FRPConnStatus

	// 统计信息
	stats       TunnelStats
	startTime   time.Time
	lastError   string

	// 日志
	logCh       chan LogEntry

	// 文件路径
	configFile  string
}

// FRPConnStatus FRP 连接状态
type FRPConnStatus struct {
	Connected       bool      `json:"connected"`
	ServerAddr      string    `json:"server_addr"`
	ServerPort      int       `json:"server_port"`
	RemotePort      int       `json:"remote_port"`
	ProxyName       string    `json:"proxy_name"`
	LastConnect     time.Time `json:"last_connect"`
	LastDisconnect  time.Time `json:"last_disconnect"`
	ReconnectCount  int       `json:"reconnect_count"`
}

// NewFRPTunnelInstance 创建 FRP 隧道实例
func NewFRPTunnelInstance(config FRPTunnelConfig, logger *zap.Logger) (*FRPTunnelInstance, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// 验证配置
	if err := validateFRPTunnelConfig(&config); err != nil {
		return nil, err
	}

	// 设置默认值
	setFRPTunnelDefaults(&config)

	ctx, cancel := context.WithCancel(context.Background())

	// 确保配置目录存在
	if err := os.MkdirAll(config.ConfigPath, 0750); err != nil {
		cancel()
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}

	instance := &FRPTunnelInstance{
		config:     config,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		logCh:      make(chan LogEntry, 100),
		configFile: filepath.Join(config.ConfigPath, fmt.Sprintf("frpc_%s.ini", config.Name)),
	}

	return instance, nil
}

// validateFRPTunnelConfig 验证配置
func validateFRPTunnelConfig(config *FRPTunnelConfig) error {
	if config.ServerAddr == "" {
		return errors.New("frp server address is required")
	}
	if config.ServerPort <= 0 || config.ServerPort > 65535 {
		return errors.New("invalid server port")
	}
	if config.Name == "" {
		return errors.New("proxy name is required")
	}
	if config.LocalPort <= 0 || config.LocalPort > 65535 {
		return errors.New("invalid local port")
	}
	return nil
}

// setFRPTunnelDefaults 设置默认值
func setFRPTunnelDefaults(config *FRPTunnelConfig) {
	if config.LocalHost == "" {
		config.LocalHost = "localhost"
	}
	if config.ServerPort == 0 {
		config.ServerPort = 7000
	}
	if config.Protocol == "" {
		config.Protocol = "tcp"
	}
	if config.ConfigPath == "" {
		config.ConfigPath = "/etc/nas-os/frp"
	}
}

// findFRPCBinary 查找 frpc 可执行文件
func findFRPCBinary() string {
	paths := []string{
		"/usr/local/bin/frpc",
		"/usr/bin/frpc",
		"/opt/frp/frpc",
		"/etc/nas-os/frp/frpc",
		"frpc", // PATH 中
	}

	for _, p := range paths {
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
	}

	return ""
}

// Start 启动 FRP 客户端
func (t *FRPTunnelInstance) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return errors.New("tunnel already running")
	}

	t.logger.Info("启动 FRP Tunnel",
		zap.String("name", t.config.Name),
		zap.String("server", t.config.ServerAddr),
	)

	// 查找 frpc
	t.frpcPath = findFRPCBinary()
	if t.frpcPath == "" {
		return errors.New("frpc binary not found")
	}

	// 生成配置文件
	if err := t.generateConfigFile(); err != nil {
		return fmt.Errorf("生成配置文件失败: %w", err)
	}

	// 构建启动参数
	args := []string{
		"-c", t.configFile,
	}

	// 启动进程
	t.process = exec.CommandContext(t.ctx, t.frpcPath, args...)

	// 捕获输出
	outputWriter := &frpLogWriter{
		logger:   t.logger,
		logCh:    t.logCh,
		tunnelID: t.config.Name,
		source:   "frpc",
	}
	t.process.Stdout = outputWriter
	t.process.Stderr = outputWriter

	if err := t.process.Start(); err != nil {
		return fmt.Errorf("启动 frpc 失败: %w", err)
	}

	t.running = true
	t.startTime = time.Now()
	t.connStatus.Connected = true
	t.connStatus.LastConnect = time.Now()
	t.connStatus.ServerAddr = t.config.ServerAddr
	t.connStatus.ServerPort = t.config.ServerPort
	t.connStatus.ProxyName = t.config.Name

	// 计算远程地址
	if t.config.RemotePort > 0 {
		t.publicAddr = fmt.Sprintf("%s:%d", t.config.ServerAddr, t.config.RemotePort)
	} else {
		// FRP 自动分配端口，需要从日志或 API 获取
		t.publicAddr = fmt.Sprintf("%s:?", t.config.ServerAddr)
	}

	// 启动监控
	t.wg.Add(1)
	go t.monitorLoop()

	t.logger.Info("FRP Tunnel 已启动",
		zap.Int("pid", t.process.Process.Pid),
	)

	// 发送启动日志
	t.logCh <- LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   fmt.Sprintf("隧道已启动 (PID: %d)", t.process.Process.Pid),
		Source:    "frpc",
		TunnelID:  t.config.Name,
	}

	return nil
}

// Stop 停止 FRP 客户端
func (t *FRPTunnelInstance) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return nil
	}

	t.logger.Info("停止 FRP Tunnel")

	// 取消上下文
	t.cancel()

	// 停止进程
	if t.process != nil && t.process.Process != nil {
		// 先尝试优雅关闭
		if err := t.process.Process.Signal(os.Interrupt); err != nil {
			// 强制终止
			_ = t.process.Process.Kill()
		}

		// 等待进程结束
		done := make(chan error, 1)
		go func() {
			done <- t.process.Wait()
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = t.process.Process.Kill()
		}
	}

	t.running = false
	t.connStatus.Connected = false
	t.connStatus.LastDisconnect = time.Now()

	// 等待监控线程
	t.wg.Wait()

	// 清理
	if err := t.cleanup(); err != nil {
		t.logger.Warn("清理失败", zap.Error(err))
	}

	t.logger.Info("FRP Tunnel 已停止")

	return nil
}

// GetStatus 获取状态
func (t *FRPTunnelInstance) GetStatus() (*TunnelStatusInfo, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return &TunnelStatusInfo{
		TunnelInfo: &TunnelInfo{
			ID:        t.config.Name,
			Name:      t.config.Name,
			Provider:  ProviderFRP,
			Protocol:  t.config.Protocol,
			LocalAddr: fmt.Sprintf("%s:%d", t.config.LocalHost, t.config.LocalPort),
			RemoteAddr: t.publicAddr,
			State:     t.getState(),
		},
		Connected:       t.connStatus.Connected,
		LastConnected:   t.connStatus.LastConnect,
		LastError:       t.lastError,
		BytesSent:       t.stats.BytesSent,
		BytesReceived:   t.stats.BytesReceived,
		Connections:     t.connStatus.ReconnectCount + 1,
		UptimeSeconds:   int64(time.Since(t.startTime).Seconds()),
	}, nil
}

// getState 获取状态
func (t *FRPTunnelInstance) getState() TunnelState {
	if !t.running {
		return StateDisconnected
	}
	if t.connStatus.Connected {
		return StateConnected
	}
	if t.lastError != "" {
		return StateError
	}
	return StateConnecting
}

// StreamLogs 流式日志
func (t *FRPTunnelInstance) StreamLogs() (<-chan LogEntry, error) {
	return t.logCh, nil
}

// IsConnected 检查连接
func (t *FRPTunnelInstance) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.running && t.connStatus.Connected
}

// generateConfigFile 生成 FRP 配置文件
func (t *FRPTunnelInstance) generateConfigFile() error {
	// INI 格式配置
	configContent := fmt.Sprintf(`
[common]
server_addr = %s
server_port = %d
token = %s

[%s]
type = %s
local_ip = %s
local_port = %d
remote_port = %d
`,
		t.config.ServerAddr,
		t.config.ServerPort,
		t.config.AuthToken,
		t.config.Name,
		t.config.Protocol,
		t.config.LocalHost,
		t.config.LocalPort,
		t.config.RemotePort,
	)

	// 如果远程端口为 0，不指定（让 FRP 自动分配）
	if t.config.RemotePort == 0 {
		configContent = fmt.Sprintf(`
[common]
server_addr = %s
server_port = %d
token = %s

[%s]
type = %s
local_ip = %s
local_port = %d
`,
			t.config.ServerAddr,
			t.config.ServerPort,
			t.config.AuthToken,
			t.config.Name,
			t.config.Protocol,
			t.config.LocalHost,
			t.config.LocalPort,
		)
	}

	return os.WriteFile(t.configFile, []byte(configContent), 0600)
}

// monitorLoop 监控循环
func (t *FRPTunnelInstance) monitorLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.checkConnection()
			t.collectMetrics()
		}
	}
}

// checkConnection 检查连接
func (t *FRPTunnelInstance) checkConnection() {
	// 检查进程是否存活
	if t.process == nil || t.process.Process == nil {
		t.mu.Lock()
		t.connStatus.Connected = false
		t.running = false
		t.mu.Unlock()
		return
	}

	// 检查进程状态
	if t.process.ProcessState != nil && t.process.ProcessState.Exited() {
		t.mu.Lock()
		t.connStatus.Connected = false
		t.running = false
		t.mu.Unlock()
		return
	}

	// 尝试连接 FRP 管理端口检查状态（如果启用了 admin UI）
	// 这里简化处理，假设进程存活即连接
	t.mu.Lock()
	t.connStatus.Connected = true
	t.mu.Unlock()
}

// collectMetrics 收集指标
func (t *FRPTunnelInstance) collectMetrics() {
	// FRP 客户端不直接暴露 metrics，需要通过管理接口获取
	// 这里简化处理，通过检查连接状态来更新

	t.mu.Lock()
	defer t.mu.Unlock()

	t.stats.UptimeSeconds = int64(time.Since(t.startTime).Seconds())

	// 通过连接状态估算流量（简化）
	// 实际实现需要通过 FRP admin API 获取
}

// cleanup 清理
func (t *FRPTunnelInstance) cleanup() error {
	// 删除配置文件
	if err := os.Remove(t.configFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// frpLogWriter FRP 日志写入器
type frpLogWriter struct {
	logger   *zap.Logger
	logCh    chan LogEntry
	tunnelID string
	source   string
}

func (w *frpLogWriter) Write(p []byte) (n int, err error) {
	// 写入日志
	w.logger.Debug("frpc", zap.String("output", string(p)))

	// 解析日志级别
	level := "info"
	if strings.Contains(string(p), "error") || strings.Contains(string(p), "Error") {
		level = "error"
	} else if strings.Contains(string(p), "warn") || strings.Contains(string(p), "Warn") {
		level = "warn"
	}

	// 发送到日志通道
	select {
	case w.logCh <- LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   string(p),
		Source:    w.source,
		TunnelID:  w.tunnelID,
	}:
	default:
		// 通道满，丢弃
	}

	return len(p), nil
}

// ========== FRP Provider 管理器 ==========

// FRPProvider FRP 提供商管理器
type FRPProvider struct {
	config    FRPProviderConfig
	logger    *zap.Logger
}

// NewFRPProvider 创建 FRP 提供商
func NewFRPProvider(config FRPProviderConfig, logger *zap.Logger) *FRPProvider {
	return &FRPProvider{
		config: config,
		logger: logger,
	}
}

// ========== 确保实现接口 ==========

var _ ProviderInstance = (*FRPTunnelInstance)(nil)

// ========== HTTP Proxy 辅助（用于通过 HTTP API 控制 FRP） ==========

// FRPAdminClient FRP 管理客户端
type FRPAdminClient struct {
	adminAddr string
	adminPort int
	adminUser string
	adminPwd  string
}

// NewFRPAdminClient 创建 FRP 管理客户端
func NewFRPAdminClient(adminAddr, adminPort, adminUser, adminPwd string) *FRPAdminClient {
	return &FRPAdminClient{
		adminAddr: adminAddr,
		adminPort: 7500, // 默认管理端口
		adminUser: adminUser,
		adminPwd:  adminPwd,
	}
}

// GetProxyStatus 获取代理状态
func (c *FRPAdminClient) GetProxyStatus(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("http://%s:%d/api/proxy/%s", c.adminAddr, c.adminPort, name)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get proxy status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ListProxies 列出所有代理
func (c *FRPAdminClient) ListProxies() ([]map[string]interface{}, error) {
	url := fmt.Sprintf("http://%s:%d/api/proxy", c.adminAddr, c.adminPort)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to list proxies: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ========== JSON 解析辅助 ==========

// 简化导入，避免循环依赖
var json = jsonEncoder{}

type jsonEncoder struct{}

func (jsonEncoder) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (jsonEncoder) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}