// Package tunnel 提供 Cloudflare Tunnel 提供商实现
package tunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ========== Cloudflare Provider ==========

// CloudflareProvider Cloudflare 提供商管理器
type CloudflareProvider struct {
	config    CloudflareProviderConfig
	logger    *zap.Logger
	api       *CloudflareAPI
}

// NewCloudflareProvider 创建 Cloudflare 提供商
func NewCloudflareProvider(config CloudflareProviderConfig, logger *zap.Logger) *CloudflareProvider {
	api := NewCloudflareAPI(config.Token, config.AccountID, logger)
	return &CloudflareProvider{
		config: config,
		logger: logger,
		api:    api,
	}
}

// ========== Cloudflare Tunnel Instance ==========

// CloudflareTunnelInstance 单个 Cloudflare 隧道实例
type CloudflareTunnelInstance struct {
	config    CloudflareTunnelConfig
	logger    *zap.Logger
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup

	// 运行状态
	running      bool
	process      *exec.Cmd
	tunnelID     string
	publicURL    string
	connStatus   CloudflareConnStatus

	// 统计信息
	stats        TunnelStats
	startTime    time.Time
	lastError    string

	// 日志
	logCh        chan LogEntry

	// 文件路径
	configFile   string
	pidFile      string
	credentialsFile string
}

// NewCloudflareTunnelInstance 创建 Cloudflare 隧道实例
func NewCloudflareTunnelInstance(config CloudflareTunnelConfig, logger *zap.Logger) (*CloudflareTunnelInstance, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// 验证配置
	if err := validateCloudflareTunnelConfig(&config); err != nil {
		return nil, err
	}

	// 设置默认值
	setCloudflareTunnelDefaults(&config)

	ctx, cancel := context.WithCancel(context.Background())

	// 确保配置目录存在
	if err := os.MkdirAll(config.ConfigPath, 0750); err != nil {
		cancel()
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}

	instance := &CloudflareTunnelInstance{
		config:       config,
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
		logCh:        make(chan LogEntry, 100),
		configFile:   filepath.Join(config.ConfigPath, fmt.Sprintf("%s.yml", config.TunnelName)),
		pidFile:      filepath.Join(config.ConfigPath, fmt.Sprintf("%s.pid", config.TunnelName)),
		credentialsFile: filepath.Join(config.ConfigPath, "credentials.json"),
	}

	return instance, nil
}

// validateCloudflareTunnelConfig 验证配置
func validateCloudflareTunnelConfig(config *CloudflareTunnelConfig) error {
	// Token 方式最简单
	if config.Token != "" {
		return nil
	}

	// 必须有至少一个本地服务配置
	if config.LocalPort <= 0 || config.LocalPort > 65535 {
		return errors.New("invalid local port")
	}

	return nil
}

// setCloudflareTunnelDefaults 设置默认值
func setCloudflareTunnelDefaults(config *CloudflareTunnelConfig) {
	if config.LocalHost == "" {
		config.LocalHost = "localhost"
	}
	if config.Protocol == "" {
		config.Protocol = "http"
	}
	if config.MetricsPort == 0 {
		config.MetricsPort = 49133
	}
	if config.ConfigPath == "" {
		config.ConfigPath = "/etc/nas-os/cloudflare"
	}
}

// findCloudflaredBinary 查找 cloudflared 可执行文件
func findCloudflaredBinary() string {
	paths := []string{
		"/usr/local/bin/cloudflared",
		"/usr/bin/cloudflared",
		"/opt/cloudflared/cloudflared",
		"/etc/nas-os/cloudflared",
		"cloudflared", // PATH 中
	}

	for _, p := range paths {
		if _, err := exec.LookPath(p); err == nil {
			return p
		}
	}

	return ""
}

// Start 启动 Cloudflare Tunnel
func (t *CloudflareTunnelInstance) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return errors.New("tunnel already running")
	}

	t.logger.Info("启动 Cloudflare Tunnel",
		zap.String("tunnel_name", t.config.TunnelName),
		zap.Int("local_port", t.config.LocalPort),
	)

	// 查找 cloudflared
	cloudflaredPath := findCloudflaredBinary()
	if cloudflaredPath == "" {
		return errors.New("cloudflared binary not found")
	}

	// 生成配置文件
	if err := t.generateConfigFile(); err != nil {
		return fmt.Errorf("生成配置文件失败: %w", err)
	}

	// 构建启动参数
	args := t.buildStartArgs()

	// 启动进程
	t.process = exec.CommandContext(t.ctx, cloudflaredPath, args...)

	// 捕获输出
	outputWriter := &tunnelLogWriter{
		logger:  t.logger,
		logCh:   t.logCh,
		tunnelID: t.tunnelID,
		source:  "cloudflared",
	}
	t.process.Stdout = outputWriter
	t.process.Stderr = outputWriter

	if err := t.process.Start(); err != nil {
		return fmt.Errorf("启动 cloudflared 失败: %w", err)
	}

	t.running = true
	t.startTime = time.Now()
	t.connStatus.Connected = true
	t.connStatus.LastConnect = time.Now()

	// 启动监控
	t.wg.Add(1)
	go t.monitorLoop()

	t.logger.Info("Cloudflare Tunnel 已启动",
		zap.Int("pid", t.process.Process.Pid),
	)

	// 发送启动日志
	t.logCh <- LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   fmt.Sprintf("隧道已启动 (PID: %d)", t.process.Process.Pid),
		Source:    "cloudflared",
		TunnelID:  t.tunnelID,
	}

	return nil
}

// Stop 停止 Cloudflare Tunnel
func (t *CloudflareTunnelInstance) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return nil
	}

	t.logger.Info("停止 Cloudflare Tunnel")

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

	t.logger.Info("Cloudflare Tunnel 已停止")

	return nil
}

// GetStatus 获取状态
func (t *CloudflareTunnelInstance) GetStatus() (*TunnelStatusInfo, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return &TunnelStatusInfo{
		TunnelInfo: &TunnelInfo{
			ID:        t.tunnelID,
			Name:      t.config.TunnelName,
			Provider:  ProviderCloudflare,
			Protocol:  t.config.Protocol,
			LocalAddr: fmt.Sprintf("%s:%d", t.config.LocalHost, t.config.LocalPort),
			PublicURL: t.publicURL,
			State:     t.getState(),
		},
		Connected:       t.connStatus.Connected,
		LastConnected:   t.connStatus.LastConnect,
		LastError:       t.lastError,
		BytesSent:       t.stats.BytesSent,
		BytesReceived:   t.stats.BytesReceived,
		Connections:     t.connStatus.Connections,
		UptimeSeconds:   int64(time.Since(t.startTime).Seconds()),
		LatencyMs:       t.connStatus.LatencyMs,
		TunnelToken:     t.config.Token,
		TunnelID:        t.tunnelID,
	}, nil
}

// getState 获取状态
func (t *CloudflareTunnelInstance) getState() TunnelState {
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
func (t *CloudflareTunnelInstance) StreamLogs() (<-chan LogEntry, error) {
	return t.logCh, nil
}

// IsConnected 检查连接
func (t *CloudflareTunnelInstance) IsConnected() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.running && t.connStatus.Connected
}

// generateConfigFile 生成配置文件
func (t *CloudflareTunnelInstance) generateConfigFile() error {
	// 如果使用 Token，不需要配置文件
	if t.config.Token != "" {
		return nil
	}

	// 构建 YAML 配置
	yamlContent := fmt.Sprintf(`
tunnel: %s
credentials-file: %s

ingress:
  - hostname: %s
    service: %s://%s:%d
  - service: http_status:404

metrics: localhost:%d
`,
		t.tunnelID,
		t.credentialsFile,
		t.buildHostname(),
		t.config.Protocol,
		t.config.LocalHost,
		t.config.LocalPort,
		t.config.MetricsPort,
	)

	return os.WriteFile(t.configFile, []byte(yamlContent), 0600)
}

// buildHostname 构建域名
func (t *CloudflareTunnelInstance) buildHostname() string {
	subdomain := t.config.Subdomain
	if subdomain == "" {
		subdomain = t.config.TunnelName
	}

	// 使用 Quick Tunnel（临时域名）
	if t.config.Token == "" && t.config.ZoneName == "" {
		return fmt.Sprintf("%s.trycloudflare.com", subdomain)
	}

	// 使用自定义域名
	if t.config.ZoneName != "" {
		return fmt.Sprintf("%s.%s", subdomain, t.config.ZoneName)
	}

	// 使用 Cloudflare Tunnel 域名
	return fmt.Sprintf("%s.cfargotunnel.com", t.tunnelID)
}

// buildStartArgs 构建启动参数
func (t *CloudflareTunnelInstance) buildStartArgs() []string {
	// 使用 Token 方式最简单
	if t.config.Token != "" {
		return []string{
			"tunnel",
			"--token", t.config.Token,
			"run",
		}
	}

	// 使用 Quick Tunnel（临时域名）
	if t.config.ZoneName == "" && t.tunnelID == "" {
		return []string{
			"tunnel",
			"--url", fmt.Sprintf("%s://%s:%d", t.config.Protocol, t.config.LocalHost, t.config.LocalPort),
		}
	}

	// 使用配置文件
	return []string{
		"tunnel",
		"--config", t.configFile,
		"run",
	}
}

// monitorLoop 监控循环
func (t *CloudflareTunnelInstance) monitorLoop() {
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
func (t *CloudflareTunnelInstance) checkConnection() {
	// 检查进程是否存活
	if t.process == nil || t.process.Process == nil {
		t.mu.Lock()
		t.connStatus.Connected = false
		t.running = false
		t.mu.Unlock()
		return
	}

	// 通过 metrics API 检查
	url := fmt.Sprintf("http://localhost:%d/metrics", t.config.MetricsPort)
	resp, err := http.Get(url)
	if err != nil {
		t.mu.Lock()
		t.connStatus.Connected = false
		t.lastError = err.Error()
		t.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	t.mu.Lock()
	t.connStatus.Connected = resp.StatusCode == 200
	t.mu.Unlock()
}

// collectMetrics 收集指标
func (t *CloudflareTunnelInstance) collectMetrics() {
	url := fmt.Sprintf("http://localhost:%d/metrics", t.config.MetricsPort)

	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	t.parsePrometheusMetrics(string(body))
}

// parsePrometheusMetrics 解析 Prometheus 指标
func (t *CloudflareTunnelInstance) parsePrometheusMetrics(body string) {
	lines := strings.Split(body, "\n")

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, line := range lines {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// 解析关键指标
		if strings.HasPrefix(line, "cloudflared_tunnel_ha_connections") {
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				var count int
				if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &count); err == nil {
					t.connStatus.Connections = count
				}
			}
		}

		// 发送字节数
		if strings.Contains(line, "cloudflared_tunnel_bytes_tx") {
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				var bytes int64
				if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &bytes); err == nil {
					t.stats.BytesSent = bytes
				}
			}
		}

		// 接收字节数
		if strings.Contains(line, "cloudflared_tunnel_bytes_rx") {
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				var bytes int64
				if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &bytes); err == nil {
					t.stats.BytesReceived = bytes
				}
			}
		}

		// 请求计数
		if strings.Contains(line, "cloudflared_tunnel_requests") {
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				var count int64
				if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &count); err == nil {
					t.stats.RequestCount = count
				}
			}
		}
	}

	// 计算运行时间
	t.stats.UptimeSeconds = int64(time.Since(t.startTime).Seconds())
}

// cleanup 清理
func (t *CloudflareTunnelInstance) cleanup() error {
	// 删除配置文件
	if err := os.Remove(t.configFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	// 删除 PID 文件
	if err := os.Remove(t.pidFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// tunnelLogWriter 隧道日志写入器
type tunnelLogWriter struct {
	logger   *zap.Logger
	logCh    chan LogEntry
	tunnelID string
	source   string
}

func (w *tunnelLogWriter) Write(p []byte) (n int, err error) {
	// 写入日志
	w.logger.Debug("cloudflared", zap.String("output", string(p)))

	// 发送到日志通道
	level := "info"
	if strings.Contains(string(p), "error") || strings.Contains(string(p), "Error") {
		level = "error"
	} else if strings.Contains(string(p), "warn") || strings.Contains(string(p), "Warn") {
		level = "warn"
	}

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

// ========== Cloudflare API 辅助 ==========

// CreateTunnelViaAPI 通过 API 创建 Tunnel
func (t *CloudflareTunnelInstance) CreateTunnelViaAPI(name string) error {
	if t.api == nil {
		return errors.New("API client not initialized")
	}

	info, err := t.api.CreateTunnel(name)
	if err != nil {
		return err
	}

	t.mu.Lock()
	t.tunnelID = info.ID
	t.mu.Unlock()

	return nil
}

// GetTunnelTokenViaAPI 通过 API 获取 Tunnel Token
func (t *CloudflareTunnelInstance) GetTunnelTokenViaAPI() (string, error) {
	if t.api == nil {
		return "", errors.New("API client not initialized")
	}

	return t.api.GetTunnelToken(t.tunnelID)
}

// ========== 确保实现接口 ==========

var _ ProviderInstance = (*CloudflareTunnelInstance)(nil)