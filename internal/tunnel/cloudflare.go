// Package tunnel 提供 Cloudflare Tunnel 内网穿透服务
// 通过 cloudflared 实现无需开放端口的远程访问
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

// Cloudflare Tunnel 相关错误
var (
	ErrCloudflaredNotFound   = errors.New("cloudflared binary not found")
	ErrTunnelAlreadyRunning  = errors.New("tunnel already running")
	ErrTunnelNotRunning      = errors.New("tunnel not running")
	ErrInvalidCredentials    = errors.New("invalid cloudflare credentials")
	ErrZoneNotFound          = errors.New("zone not found")
	ErrDNSRecordFailed       = errors.New("dns record creation failed")
	ErrTunnelTokenRequired   = errors.New("tunnel token required")
	ErrAPITokenRequired      = errors.New("api token required for managed tunnels")
)

// CloudflareConfig Cloudflare Tunnel 配置
type CloudflareConfig struct {
	// 认证方式
	Token string `json:"token"`           // Tunnel Token (推荐方式)
	APIToken string `json:"api_token"`    // API Token (管理 Tunnel)
	AccountID string `json:"account_id"`  // Account ID

	// Tunnel 配置
	TunnelID string `json:"tunnel_id"`    // 已有 Tunnel ID
	TunnelName string `json:"tunnel_name"` // Tunnel 名称

	// 域名配置
	ZoneID string `json:"zone_id"`        // Zone ID
	ZoneName string `json:"zone_name"`    // Zone 名称 (如 example.com)
	Subdomain string `json:"subdomain"`   // 子域名前缀 (如 nas)

	// 服务配置
	LocalServices []CloudflareService `json:"local_services"`

	// 运行配置
	CloudflaredPath string `json:"cloudflared_path"` // cloudflared 可执行文件路径
	ConfigPath string `json:"config_path"`          // 配置文件路径
	LogPath string `json:"log_path"`               // 日志文件路径
	MetricsPort int `json:"metrics_port"`          // metrics 端口
	NoAutoUpdate bool `json:"no_auto_update"`      // 禁止自动更新

	// 重连配置
	ReconnectInterval int `json:"reconnect_interval"` // 重连间隔(秒)
	MaxReconnectAttempts int `json:"max_reconnect_attempts"` // 最大重连次数
	HeartbeatInterval int `json:"heartbeat_interval"` // 心跳间隔(秒)
}

// CloudflareService 本地服务配置
type CloudflareService struct {
	Name string `json:"name"`              // 服务名称
	Protocol string `json:"protocol"`      // 协议 (http, https, tcp, ssh)
	LocalPort int `json:"local_port"`      // 本地端口
	LocalHost string `json:"local_host"`   // 本地主机 (默认 localhost)
	Path string `json:"path"`              // URL 路径 (HTTP 服务)
	Subdomain string `json:"subdomain"`    // 子域名 (覆盖全局设置)
}

// CloudflareTunnel Cloudflare Tunnel 客户端
type CloudflareTunnel struct {
	config   CloudflareConfig
	logger   *zap.Logger
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	// 运行状态
	running    bool
	process    *exec.Cmd
	tunnelID   string
	publicURL  string
	connStatus CloudflareConnStatus

	// 统计信息
	stats      TunnelStats
	startTime  time.Time
	lastError  string

	// 配置文件路径
	configFile string
	pidFile    string
}

// CloudflareConnStatus Cloudflare 连接状态
type CloudflareConnStatus struct {
	Connected    bool      `json:"connected"`
	TunnelID     string    `json:"tunnel_id"`
	PublicURL    string    `json:"public_url"`
	LatencyMs    int64     `json:"latency_ms"`
	Connections  int       `json:"connections"`
	LastConnect  time.Time `json:"last_connect"`
	LastDisconnect time.Time `json:"last_disconnect"`
	ReconnectCount int     `json:"reconnect_count"`
}

// TunnelStats 隧道统计
type TunnelStats struct {
	BytesSent     int64     `json:"bytes_sent"`
	BytesReceived int64     `json:"bytes_received"`
	Connections   int       `json:"total_connections"`
	RequestCount  int64     `json:"request_count"`
	ErrorCount    int64     `json:"error_count"`
	AvgLatencyMs  int64     `json:"avg_latency_ms"`
	UptimeSeconds int64     `json:"uptime_seconds"`
}

// CloudflareAPI Cloudflare API 客户端
type CloudflareAPI struct {
	apiToken  string
	accountID string
	baseURL   string
	logger    *zap.Logger
}

// NewCloudflareTunnel 创建 Cloudflare Tunnel 客户端
func NewCloudflareTunnel(config CloudflareConfig, logger *zap.Logger) (*CloudflareTunnel, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// 验证配置
	if err := validateCloudflareConfig(&config); err != nil {
		return nil, err
	}

	// 查找 cloudflared
	if config.CloudflaredPath == "" {
		config.CloudflaredPath = findCloudflared()
	}
	if config.CloudflaredPath == "" {
		return nil, ErrCloudflaredNotFound
	}

	// 设置默认值
	setCloudflareDefaults(&config)

	ctx, cancel := context.WithCancel(context.Background())

	tunnel := &CloudflareTunnel{
		config:    config,
		logger:    logger,
		ctx:       ctx,
		cancel:    cancel,
		configFile: config.ConfigPath,
		pidFile:    filepath.Join(config.ConfigPath, "cloudflared.pid"),
	}

	// 确保配置目录存在
	if err := os.MkdirAll(config.ConfigPath, 0750); err != nil {
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}

	return tunnel, nil
}

// validateCloudflareConfig 验证配置
func validateCloudflareConfig(config *CloudflareConfig) error {
	// 必须有 Token 或 API Token
	if config.Token == "" && config.APIToken == "" {
		return ErrTunnelTokenRequired
	}

	// 必须有至少一个本地服务
	if len(config.LocalServices) == 0 {
		return errors.New("至少需要配置一个本地服务")
	}

	// 验证本地服务配置
	for _, svc := range config.LocalServices {
		if svc.LocalPort <= 0 || svc.LocalPort > 65535 {
			return fmt.Errorf("无效端口: %d", svc.LocalPort)
		}
		if svc.Protocol == "" {
			svc.Protocol = "http"
		}
	}

	return nil
}

// setCloudflareDefaults 设置默认值
func setCloudflareDefaults(config *CloudflareConfig) {
	if config.ConfigPath == "" {
		config.ConfigPath = "/etc/nas-os/cloudflare"
	}
	if config.LogPath == "" {
		config.LogPath = "/var/log/nas-os/cloudflared.log"
	}
	if config.MetricsPort == 0 {
		config.MetricsPort = 49133
	}
	if config.ReconnectInterval == 0 {
		config.ReconnectInterval = 5
	}
	if config.MaxReconnectAttempts == 0 {
		config.MaxReconnectAttempts = 10
	}
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 30
	}
	if config.TunnelName == "" {
		config.TunnelName = "nas-tunnel"
	}
}

// findCloudflared 查找 cloudflared 可执行文件
func findCloudflared() string {
	// 常见位置
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
func (t *CloudflareTunnel) Start(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return ErrTunnelAlreadyRunning
	}

	t.logger.Info("启动 Cloudflare Tunnel",
		zap.String("tunnel_name", t.config.TunnelName),
		zap.Int("services", len(t.config.LocalServices)),
	)

	// 生成配置文件
	if err := t.generateConfigFile(); err != nil {
		return fmt.Errorf("生成配置文件失败: %w", err)
	}

	// 构建启动命令
	args := t.buildStartArgs()

	// 启动进程
	t.process = exec.CommandContext(t.ctx, t.config.CloudflaredPath, args...)
	t.process.Stdout = &logWriter{logger: t.logger, prefix: "cloudflared"}
	t.process.Stderr = &logWriter{logger: t.logger, prefix: "cloudflared-error"}

	if err := t.process.Start(); err != nil {
		return fmt.Errorf("启动 cloudflared 失败: %w", err)
	}

	t.running = true
	t.startTime = time.Now()
	t.connStatus.Connected = true
	t.connStatus.LastConnect = time.Now()

	// 启动状态监控
	t.wg.Add(1)
	go t.monitorLoop()

	// 启动重连处理
	t.wg.Add(1)
	go t.reconnectLoop()

	t.logger.Info("Cloudflare Tunnel 已启动",
		zap.Int("pid", t.process.Process.Pid),
	)

	return nil
}

// Stop 停止 Cloudflare Tunnel
func (t *CloudflareTunnel) Stop() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.running {
		return ErrTunnelNotRunning
	}

	t.logger.Info("停止 Cloudflare Tunnel")

	// 取消上下文
	t.cancel()

	// 等待进程结束
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

	// 等待监控线程结束
	t.wg.Wait()

	// 清理配置文件
	if err := t.cleanup(); err != nil {
		t.logger.Warn("清理失败", zap.Error(err))
	}

	t.logger.Info("Cloudflare Tunnel 已停止")

	return nil
}

// generateConfigFile 生成 cloudflared 配置文件
func (t *CloudflareTunnel) generateConfigFile() error {
	config := map[string]interface{}{
		"tunnel": t.config.TunnelID,
		"credentials-file": filepath.Join(t.config.ConfigPath, "credentials.json"),
	}

	// 配置 ingress 规则
	ingress := make([]map[string]interface{}, 0, len(t.config.LocalServices)+1)

	for _, svc := range t.config.LocalServices {
		host := svc.LocalHost
		if host == "" {
			host = "localhost"
		}

		rule := map[string]interface{}{
			"hostname": t.buildHostname(svc),
			"service": map[string]interface{}{
				"protocol": svc.Protocol,
				"address": fmt.Sprintf("%s://%s:%d", svc.Protocol, host, svc.LocalPort),
			},
		}

		if svc.Path != "" {
			rule["path"] = svc.Path
		}

		ingress = append(ingress, rule)
	}

	// 默认规则 (404)
	ingress = append(ingress, map[string]interface{}{
		"service": "http_status:404",
	})

	config["ingress"] = ingress

	// 写入配置文件
	configPath := filepath.Join(t.config.ConfigPath, "config.yml")
	// 使用 YAML 格式 (cloudflared 使用 YAML)
	yamlData := t.jsonToYAML(config)
	return os.WriteFile(configPath, yamlData, 0600)
}

// buildHostname 构建域名
func (t *CloudflareTunnel) buildHostname(svc CloudflareService) string {
	subdomain := svc.Subdomain
	if subdomain == "" {
		subdomain = t.config.Subdomain
	}

	// 如果没有 ZoneName，使用 Tunnel ID 作为标识
	if t.config.ZoneName == "" {
		return fmt.Sprintf("%s-%s.cfargotunnel.com", subdomain, t.tunnelID)
	}

	return fmt.Sprintf("%s.%s", subdomain, t.config.ZoneName)
}

// buildStartArgs 构建启动参数
func (t *CloudflareTunnel) buildStartArgs() []string {
	args := []string{
		"tunnel",
		"--config", filepath.Join(t.config.ConfigPath, "config.yml"),
		"--metrics", fmt.Sprintf("localhost:%d", t.config.MetricsPort),
	}

	if t.config.NoAutoUpdate {
		args = append(args, "--no-autoupdate")
	}

	// 使用 Token 方式
	if t.config.Token != "" {
		args = append(args, "--token", t.config.Token)
		return args
	}

	// 使用 Tunnel ID
	if t.config.TunnelID != "" {
		args = append(args, "run", "--id", t.config.TunnelID)
	} else {
		args = append(args, "run", "--name", t.config.TunnelName)
	}

	return args
}

// monitorLoop 监控循环
func (t *CloudflareTunnel) monitorLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(time.Duration(t.config.HeartbeatInterval) * time.Second)
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

// reconnectLoop 重连循环
func (t *CloudflareTunnel) reconnectLoop() {
	defer t.wg.Done()

	reconnectAttempts := 0

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-time.After(time.Duration(t.config.ReconnectInterval) * time.Second):
			t.mu.RLock()
			running := t.running && t.connStatus.Connected
			t.mu.RUnlock()

			if !running && reconnectAttempts < t.config.MaxReconnectAttempts {
				t.logger.Warn("尝试重新连接",
					zap.Int("attempt", reconnectAttempts+1),
				)

				t.mu.Lock()
				t.connStatus.ReconnectCount++
				t.mu.Unlock()

				if err := t.Start(t.ctx); err != nil {
					t.logger.Error("重连失败", zap.Error(err))
					reconnectAttempts++
				} else {
					reconnectAttempts = 0
				}
			}
		}
	}
}

// checkConnection 检查连接状态
func (t *CloudflareTunnel) checkConnection() {
	// 检查进程是否存活
	if t.process == nil || t.process.Process == nil {
		t.mu.Lock()
		t.connStatus.Connected = false
		t.running = false
		t.mu.Unlock()
		return
	}

	// 通过 metrics API 检查连接
	url := fmt.Sprintf("http://localhost:%d/metrics", t.config.MetricsPort)
	resp, err := http.Get(url)
	if err != nil {
		t.mu.Lock()
		t.connStatus.Connected = false
		t.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	t.mu.Lock()
	t.connStatus.Connected = resp.StatusCode == 200
	t.mu.Unlock()
}

// collectMetrics 收集指标
func (t *CloudflareTunnel) collectMetrics() {
	// 从 metrics 端点获取指标
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

	// 解析 Prometheus 格式的指标
	t.parsePrometheusMetrics(string(body))
}

// parsePrometheusMetrics 解析 Prometheus 指标
func (t *CloudflareTunnel) parsePrometheusMetrics(body string) {
	lines := strings.Split(body, "\n")

	t.mu.Lock()
	defer t.mu.Unlock()

	for _, line := range lines {
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		// 解析指标
		// cloudflared_tunnel_ha_connections_total
		if strings.HasPrefix(line, "cloudflared_tunnel_ha_connections") {
			// 提取连接数
			parts := strings.Split(line, " ")
			if len(parts) >= 2 {
				var count int
				if _, err := fmt.Sscanf(parts[len(parts)-1], "%d", &count); err == nil {
					t.connStatus.Connections = count
				}
			}
		}
	}

	// 计算运行时间
	t.stats.UptimeSeconds = int64(time.Since(t.startTime).Seconds())
}

// GetStatus 获取状态
func (t *CloudflareTunnel) GetStatus() CloudflareTunnelStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return CloudflareTunnelStatus{
		Running:       t.running,
		TunnelID:      t.tunnelID,
		PublicURL:     t.publicURL,
		Connection:    t.connStatus,
		Stats:         t.stats,
		Config:        t.config,
		StartTime:     t.startTime,
		UptimeSeconds: int64(time.Since(t.startTime).Seconds()),
		LastError:     t.lastError,
	}
}

// CloudflareTunnelStatus 隧道状态
type CloudflareTunnelStatus struct {
	Running       bool              `json:"running"`
	TunnelID      string            `json:"tunnel_id"`
	PublicURL     string            `json:"public_url"`
	Connection    CloudflareConnStatus  `json:"connection"`
	Stats         TunnelStats       `json:"stats"`
	Config        CloudflareConfig  `json:"config"`
	StartTime     time.Time         `json:"start_time"`
	UptimeSeconds int64             `json:"uptime_seconds"`
	LastError     string            `json:"last_error"`
}

// Cleanup 清理
func (t *CloudflareTunnel) cleanup() error {
	// 删除配置文件
	configFile := filepath.Join(t.config.ConfigPath, "config.yml")
	if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	// 删除 PID 文件
	if err := os.Remove(t.pidFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// jsonToYAML 转换 JSON 到 YAML (简化实现)
func (t *CloudflareTunnel) jsonToYAML(data map[string]interface{}) []byte {
	var yaml strings.Builder

	for key, value := range data {
		switch v := value.(type) {
		case string:
			yaml.WriteString(fmt.Sprintf("%s: %s\n", key, v))
		case int:
			yaml.WriteString(fmt.Sprintf("%s: %d\n", key, v))
		case bool:
			yaml.WriteString(fmt.Sprintf("%s: %v\n", key, v))
		case []map[string]interface{}:
			yaml.WriteString(fmt.Sprintf("%s:\n", key))
			for _, item := range v {
				yaml.WriteString("  - ")
				for ik, iv := range item {
					switch iiv := iv.(type) {
					case string:
						yaml.WriteString(fmt.Sprintf("%s: %s\n", ik, iiv))
					case map[string]interface{}:
						yaml.WriteString(fmt.Sprintf("%s:\n", ik))
						for sik, siv := range iiv {
							yaml.WriteString(fmt.Sprintf("      %s: %v\n", sik, siv))
						}
					default:
						yaml.WriteString(fmt.Sprintf("%s: %v\n", ik, iiv))
					}
				}
			}
		case map[string]interface{}:
			yaml.WriteString(fmt.Sprintf("%s:\n", key))
			for ik, iv := range v {
				yaml.WriteString(fmt.Sprintf("  %s: %v\n", ik, iv))
			}
		default:
			yaml.WriteString(fmt.Sprintf("%s: %v\n", key, v))
		}
	}

	return []byte(yaml.String())
}

// logWriter 日志写入器
type logWriter struct {
	logger *zap.Logger
	prefix string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.logger.Debug(w.prefix,
		zap.String("output", string(p)),
	)
	return len(p), nil
}

// IsRunning 检查是否运行中
func (t *CloudflareTunnel) IsRunning() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.running && t.connStatus.Connected
}

// ============================================================================
// Cloudflare API 客户端
// ============================================================================

// NewCloudflareAPI 创建 API 客户端
func NewCloudflareAPI(apiToken, accountID string, logger *zap.Logger) *CloudflareAPI {
	return &CloudflareAPI{
		apiToken:  apiToken,
		accountID: accountID,
		baseURL:   "https://api.cloudflare.com/client/v4",
		logger:    logger,
	}
}

// CreateTunnel 创建 Tunnel
func (api *CloudflareAPI) CreateTunnel(name string) (*TunnelInfo, error) {
	url := fmt.Sprintf("%s/accounts/%s/tunnels", api.baseURL, api.accountID)

	body := map[string]interface{}{
		"name": name,
		"tunnel_type": "cloudflared",
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	resp, err := api.request("POST", url, jsonBody)
	if err != nil {
		return nil, err
	}

	var result TunnelResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("创建 Tunnel 失败: %v", result.Errors)
	}

	return &result.Result, nil
}

// DeleteTunnel 删除 Tunnel
func (api *CloudflareAPI) DeleteTunnel(tunnelID string) error {
	url := fmt.Sprintf("%s/accounts/%s/tunnels/%s", api.baseURL, api.accountID, tunnelID)

	resp, err := api.request("DELETE", url, nil)
	if err != nil {
		return err
	}

	var result TunnelResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("删除 Tunnel 失败: %v", result.Errors)
	}

	return nil
}

// GetTunnelToken 获取 Tunnel Token
func (api *CloudflareAPI) GetTunnelToken(tunnelID string) (string, error) {
	url := fmt.Sprintf("%s/accounts/%s/tunnels/%s/token", api.baseURL, api.accountID, tunnelID)

	resp, err := api.request("GET", url, nil)
	if err != nil {
		return "", err
	}

	var result TokenResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}

	if !result.Success {
		return "", fmt.Errorf("获取 Token 失败: %v", result.Errors)
	}

	return result.Result, nil
}

// ListTunnels 列出所有 Tunnel
func (api *CloudflareAPI) ListTunnels() ([]TunnelInfo, error) {
	url := fmt.Sprintf("%s/accounts/%s/tunnels", api.baseURL, api.accountID)

	resp, err := api.request("GET", url, nil)
	if err != nil {
		return nil, err
	}

	var result TunnelsListResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return nil, err
	}

	if !result.Success {
		return nil, fmt.Errorf("列出 Tunnel 失败: %v", result.Errors)
	}

	return result.Result, nil
}

// CreateDNSRecord 创建 DNS 记录
func (api *CloudflareAPI) CreateDNSRecord(zoneID, name, tunnelID string) error {
	url := fmt.Sprintf("%s/zones/%s/dns_records", api.baseURL, zoneID)

	body := map[string]interface{}{
		"type": "CNAME",
		"name": name,
		"content": fmt.Sprintf("%s.cfargotunnel.com", tunnelID),
		"proxied": true,
		"ttl": 1,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}

	resp, err := api.request("POST", url, jsonBody)
	if err != nil {
		return err
	}

	var result DNSResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		return err
	}

	if !result.Success {
		return fmt.Errorf("创建 DNS 记录失败: %v", result.Errors)
	}

	return nil
}

// request 发送 API 请求
func (api *CloudflareAPI) request(method, url string, body []byte) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	var req *http.Request
	var err error

	if body != nil {
		req, err = http.NewRequest(method, url, bytes.NewReader(body))
	} else {
		req, err = http.NewRequest(method, url, nil)
	}

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+api.apiToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

// ============================================================================
// API 响应结构
// ============================================================================

// TunnelInfo Tunnel 信息
type TunnelInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	Conns     int       `json:"conns_active_at,omitempty"`
	TunType   string    `json:"tunnel_type"`
}

// TunnelResponse Tunnel API 响应
type TunnelResponse struct {
	Success bool        `json:"success"`
	Result  TunnelInfo  `json:"result"`
	Errors  []APIError  `json:"errors"`
}

// TunnelsListResponse Tunnel 列表响应
type TunnelsListResponse struct {
	Success bool        `json:"success"`
	Result  []TunnelInfo `json:"result"`
	Errors  []APIError  `json:"errors"`
}

// TokenResponse Token 响应
type TokenResponse struct {
	Success bool   `json:"success"`
	Result  string `json:"result"`
	Errors  []APIError `json:"errors"`
}

// DNSResponse DNS 响应
type DNSResponse struct {
	Success bool        `json:"success"`
	Result  DNSRecord   `json:"result"`
	Errors  []APIError  `json:"errors"`
}

// DNSRecord DNS 记录
type DNSRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
}

// APIError API 错误
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}