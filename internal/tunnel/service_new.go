// Package tunnel 提供内网穿透服务 - 核心服务实现
// 实现符合任务要求的 TunnelService 接口
package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"
)

// ========== 接口定义 ==========

// TunnelService 内网穿透服务接口
type TunnelServiceInterface interface {
	// Create 创建隧道
	Create(config TunnelCreateConfig) (*TunnelInfo, error)
	// Delete 删除隧道
	Delete(id string) error
	// List 列出所有隧道
	List() ([]*TunnelInfo, error)
	// GetStatus 获取隧道状态
	GetStatus(id string) (*TunnelStatusInfo, error)
	// StreamLogs 流式日志
	StreamLogs(id string) (<-chan LogEntry, error)
}

// TunnelCreateConfig 隧道创建配置
type TunnelCreateConfig struct {
	// 基础配置
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`

	// 隧道类型
	Provider TunnelProvider `json:"provider"` // cloudflare, frp, ngrok

	// 协议配置
	Protocol string `json:"protocol"` // tcp, http, https

	// 本地服务配置
	LocalHost string `json:"local_host"` // 默认 localhost
	LocalPort int    `json:"local_port" binding:"required,min=1,max=65535"`

	// 远程端口（可选）
	RemotePort int `json:"remote_port"`

	// 域名配置（用于 HTTP/HTTPS）
	Subdomain string `json:"subdomain"`
	Domain    string `json:"domain"` // 自定义域名

	// Cloudflare 特定配置
	CloudflareToken    string `json:"cloudflare_token"`
	CloudflareAccount  string `json:"cloudflare_account_id"`
	CloudflareZone     string `json:"cloudflare_zone_id"`
	CloudflareZoneName string `json:"cloudflare_zone_name"`

	// FRP 特定配置
	FRPServerAddr string `json:"frp_server_addr"`
	FRPServerPort int    `json:"frp_server_port"`
	FRPAuthToken  string `json:"frp_auth_token"`

	// 高级配置
	EnableTLS      bool `json:"enable_tls"`
	AutoReconnect  bool `json:"auto_reconnect"`
	MaxConnections int  `json:"max_connections"`
}

// TunnelProvider 隧道提供商
type TunnelProvider string

const (
	ProviderCloudflare TunnelProvider = "cloudflare"
	ProviderFRP        TunnelProvider = "frp"
	ProviderNgrok      TunnelProvider = "ngrok"
	ProviderAuto       TunnelProvider = "auto" // 自动选择
)

// TunnelInfo 隧道信息
type TunnelInfo struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Provider    TunnelProvider `json:"provider"`
	Protocol    string         `json:"protocol"`
	LocalAddr   string         `json:"local_addr"`
	RemoteAddr  string         `json:"remote_addr"`
	PublicURL   string         `json:"public_url"`
	State       TunnelState    `json:"state"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Description string         `json:"description"`
}

// TunnelStatusInfo 隧道详细状态
type TunnelStatusInfo struct {
	*TunnelInfo

	// 连接状态
	Connected     bool      `json:"connected"`
	LastConnected time.Time `json:"last_connected"`
	LastError     string    `json:"last_error,omitempty"`

	// 统计信息
	BytesSent     int64 `json:"bytes_sent"`
	BytesReceived int64 `json:"bytes_received"`
	Connections   int   `json:"connections"`
	RequestCount  int64 `json:"request_count"`
	UptimeSeconds int64 `json:"uptime_seconds"`

	// 性能指标
	LatencyMs     int64 `json:"latency_ms"`
	AvgLatencyMs  int64 `json:"avg_latency_ms"`
	ErrorCount    int64 `json:"error_count"`

	// Cloudflare 特定
	TunnelToken   string `json:"tunnel_token,omitempty"`
	TunnelID      string `json:"tunnel_id,omitempty"` // Cloudflare Tunnel ID
}

// LogEntry 日志条目
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // info, warn, error
	Message   string    `json:"message"`
	Source    string    `json:"source"` // cloudflared, frp, system
	TunnelID  string    `json:"tunnel_id"`
}

// ========== 服务实现 ==========

// TunnelServiceImpl 隧道服务实现
type TunnelServiceImpl struct {
	config    TunnelServiceConfig
	logger    *zap.Logger
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   atomic.Bool

	// 隧道存储
	tunnels    map[string]*managedTunnel
	tunnelByID map[string]*managedTunnel

	// 提供商实例
	cloudflare *CloudflareProvider
	frp        *FRPProvider
}

// TunnelServiceConfig 服务配置
type TunnelServiceConfig struct {
	// 默认提供商
	DefaultProvider TunnelProvider `json:"default_provider"`

	// 存储配置
	ConfigPath string `json:"config_path"`
	DataPath   string `json:"data_path"`

	// Cloudflare 默认配置
	CloudflareToken   string `json:"cloudflare_token"`
	CloudflareAccount string `json:"cloudflare_account_id"`

	// FRP 默认配置
	FRPServerAddr string `json:"frp_server_addr"`
	FRPServerPort int    `json:"frp_server_port"`
	FRPAuthToken  string `json:"frp_auth_token"`

	// 监控配置
	MetricsPort     int `json:"metrics_port"`
	HeartbeatIntSec int `json:"heartbeat_interval_sec"`
	LogRetentionSec int `json:"log_retention_sec"`
}

// managedTunnel 管理的隧道实例
type managedTunnel struct {
	info       *TunnelInfo
	status     *TunnelStatusInfo
	provider   ProviderInstance
	config     TunnelCreateConfig
	logBuffer  *LogBuffer
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	startTime  time.Time
	stats      TunnelStats
}

// ProviderInstance 提供商实例接口
type ProviderInstance interface {
	Start(ctx context.Context) error
	Stop() error
	GetStatus() (*TunnelStatusInfo, error)
	StreamLogs() (<-chan LogEntry, error)
	IsConnected() bool
}

// LogBuffer 日志缓冲
type LogBuffer struct {
	entries []LogEntry
	maxSize int
	mu      sync.RWMutex
	ch      chan LogEntry
}

// NewLogBuffer 创建日志缓冲
func NewLogBuffer(maxSize int) *LogBuffer {
	return &LogBuffer{
		entries: make([]LogEntry, 0, maxSize),
		maxSize: maxSize,
		ch:      make(chan LogEntry, 100),
	}
}

// Add 添加日志
func (b *LogBuffer) Add(entry LogEntry) {
	b.mu.Lock()
	if len(b.entries) >= b.maxSize {
		b.entries = b.entries[1:]
	}
	b.entries = append(b.entries, entry)
	b.mu.Unlock()

	// 发送到通道
	select {
	case b.ch <- entry:
	default:
		// 通道满，丢弃
	}
}

// GetEntries 获取所有日志
func (b *LogBuffer) GetEntries() []LogEntry {
	b.mu.RLock()
	defer b.mu.RUnlock()
	result := make([]LogEntry, len(b.entries))
	copy(result, b.entries)
	return result
}

// Stream 流式获取日志
func (b *LogBuffer) Stream() <-chan LogEntry {
	return b.ch
}

// ========== 服务构造与生命周期 ==========

var (
	ErrServiceNotRunning   = errors.New("tunnel service not running")
	ErrTunnelNotFound      = errors.New("tunnel not found")
	ErrTunnelAlreadyExists = errors.New("tunnel already exists")
	ErrInvalidConfig       = errors.New("invalid tunnel configuration")
	ErrProviderNotReady    = errors.New("provider not ready")
)

// NewTunnelService 创建隧道服务
func NewTunnelService(config TunnelServiceConfig, logger *zap.Logger) (*TunnelServiceImpl, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	// 设置默认值
	setServiceDefaults(&config)

	ctx, cancel := context.WithCancel(context.Background())

	service := &TunnelServiceImpl{
		config:     config,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
		tunnels:    make(map[string]*managedTunnel),
		tunnelByID: make(map[string]*managedTunnel),
	}

	// 初始化提供商
	if err := service.initProviders(); err != nil {
		cancel()
		return nil, fmt.Errorf("初始化提供商失败: %w", err)
	}

	return service, nil
}

// setServiceDefaults 设置默认值
func setServiceDefaults(config *TunnelServiceConfig) {
	if config.DefaultProvider == "" {
		config.DefaultProvider = ProviderCloudflare
	}
	if config.ConfigPath == "" {
		config.ConfigPath = "/etc/nas-os/tunnel"
	}
	if config.DataPath == "" {
		config.DataPath = "/var/lib/nas-os/tunnel"
	}
	if config.MetricsPort == 0 {
		config.MetricsPort = 49133
	}
	if config.HeartbeatIntSec == 0 {
		config.HeartbeatIntSec = 30
	}
	if config.LogRetentionSec == 0 {
		config.LogRetentionSec = 3600
	}
}

// initProviders 初始化提供商
func (s *TunnelServiceImpl) initProviders() error {
	// 初始化 Cloudflare 提供商
	cfConfig := CloudflareProviderConfig{
		Token:      s.config.CloudflareToken,
		AccountID:  s.config.CloudflareAccount,
		ConfigPath: s.config.ConfigPath,
	}
	s.cloudflare = NewCloudflareProvider(cfConfig, s.logger)

	// 初始化 FRP 提供商
	frpConfig := FRPProviderConfig{
		ServerAddr: s.config.FRPServerAddr,
		ServerPort: s.config.FRPServerPort,
		AuthToken:  s.config.FRPAuthToken,
		ConfigPath: s.config.ConfigPath,
	}
	s.frp = NewFRPProvider(frpConfig, s.logger)

	return nil
}

// Start 启动服务
func (s *TunnelServiceImpl) Start(ctx context.Context) error {
	if s.running.Swap(true) {
		return errors.New("service already running")
	}

	s.logger.Info("启动隧道服务")

	// 启动监控循环
	s.wg.Add(1)
	go s.monitorLoop()

	// 启动日志清理
	s.wg.Add(1)
	go s.logCleanupLoop()

	// 加载持久化的隧道配置
	if err := s.loadPersistedTunnels(); err != nil {
		s.logger.Warn("加载隧道配置失败", zap.Error(err))
	}

	s.logger.Info("隧道服务已启动")
	return nil
}

// Stop 停止服务
func (s *TunnelServiceImpl) Stop() error {
	if !s.running.Swap(false) {
		return nil
	}

	s.logger.Info("停止隧道服务")

	s.cancel()

	// 停止所有隧道
	s.mu.Lock()
	for _, t := range s.tunnels {
		if t.provider != nil {
			_ = t.provider.Stop()
		}
		t.cancel()
	}
	s.tunnels = make(map[string]*managedTunnel)
	s.tunnelByID = make(map[string]*managedTunnel)
	s.mu.Unlock()

	// 等待所有线程结束
	s.wg.Wait()

	s.logger.Info("隧道服务已停止")
	return nil
}

// IsRunning 检查是否运行
func (s *TunnelServiceImpl) IsRunning() bool {
	return s.running.Load()
}

// ========== 核心接口实现 ==========

// Create 创建隧道
func (s *TunnelServiceImpl) Create(config TunnelCreateConfig) (*TunnelInfo, error) {
	if !s.running.Load() {
		return nil, ErrServiceNotRunning
	}

	// 验证配置
	if err := s.validateConfig(&config); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查同名隧道
	if _, exists := s.tunnels[config.Name]; exists {
		return nil, ErrTunnelAlreadyExists
	}

	// 生成 ID
	tunnelID := generateTunnelID()

	// 确定提供商
	provider := config.Provider
	if provider == "" || provider == ProviderAuto {
		provider = s.selectProvider(config)
	}

	// 创建提供商实例
	providerInstance, err := s.createProviderInstance(provider, config)
	if err != nil {
		return nil, fmt.Errorf("创建提供商实例失败: %w", err)
	}

	// 创建隧道
	tunnelCtx, tunnelCancel := context.WithCancel(s.ctx)
	tunnel := &managedTunnel{
		info: &TunnelInfo{
			ID:          tunnelID,
			Name:        config.Name,
			Provider:    provider,
			Protocol:    config.Protocol,
			LocalAddr:   fmt.Sprintf("%s:%d", config.LocalHost, config.LocalPort),
			State:       StateConnecting,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Description: config.Description,
		},
		status: &TunnelStatusInfo{
			TunnelInfo: &TunnelInfo{
				ID:       tunnelID,
				Name:     config.Name,
				Provider: provider,
				Protocol: config.Protocol,
				State:    StateConnecting,
			},
			Connected: false,
		},
		provider:  providerInstance,
		config:    config,
		logBuffer: NewLogBuffer(1000),
		ctx:       tunnelCtx,
		cancel:    tunnelCancel,
		startTime: time.Now(),
	}

	// 存储隧道
	s.tunnels[config.Name] = tunnel
	s.tunnelByID[tunnelID] = tunnel

	// 启动隧道
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.runTunnel(tunnel)
	}()

	// 持久化配置
	if err := s.persistTunnelConfig(tunnelID, config); err != nil {
		s.logger.Warn("持久化隧道配置失败", zap.Error(err))
	}

	s.logger.Info("创建隧道",
		zap.String("id", tunnelID),
		zap.String("name", config.Name),
		zap.String("provider", string(provider)),
	)

	return tunnel.info, nil
}

// Delete 删除隧道
func (s *TunnelServiceImpl) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tunnel, exists := s.tunnelByID[id]
	if !exists {
		return ErrTunnelNotFound
	}

	s.logger.Info("删除隧道",
		zap.String("id", id),
		zap.String("name", tunnel.info.Name),
	)

	// 停止提供商
	if tunnel.provider != nil {
		if err := tunnel.provider.Stop(); err != nil {
			s.logger.Warn("停止提供商失败", zap.Error(err))
		}
	}

	// 取消上下文
	tunnel.cancel()

	// 删除存储
	delete(s.tunnels, tunnel.info.Name)
	delete(s.tunnelByID, id)

	// 删除持久化配置
	if err := s.deletePersistedConfig(id); err != nil {
		s.logger.Warn("删除持久化配置失败", zap.Error(err))
	}

	return nil
}

// List 列出所有隧道
func (s *TunnelServiceImpl) List() ([]*TunnelInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*TunnelInfo, 0, len(s.tunnels))
	for _, t := range s.tunnels {
		t.mu.RLock()
		info := *t.info // 复制
		t.mu.RUnlock()
		result = append(result, &info)
	}

	return result, nil
}

// GetStatus 获取隧道状态
func (s *TunnelServiceImpl) GetStatus(id string) (*TunnelStatusInfo, error) {
	s.mu.RLock()
	tunnel, exists := s.tunnelByID[id]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrTunnelNotFound
	}

	// 从提供商获取实时状态
	if tunnel.provider != nil {
		status, err := tunnel.provider.GetStatus()
		if err == nil {
			tunnel.mu.Lock()
			tunnel.status = status
			tunnel.mu.Unlock()
			return status, nil
		}
	}

	tunnel.mu.RLock()
	defer tunnel.mu.RUnlock()
	return tunnel.status, nil
}

// StreamLogs 流式日志
func (s *TunnelServiceImpl) StreamLogs(id string) (<-chan LogEntry, error) {
	s.mu.RLock()
	tunnel, exists := s.tunnelByID[id]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrTunnelNotFound
	}

	// 从提供商获取日志流
	if tunnel.provider != nil {
		ch, err := tunnel.provider.StreamLogs()
		if err == nil {
			return ch, nil
		}
	}

	// 从本地缓冲获取
	return tunnel.logBuffer.Stream(), nil
}

// ========== 辅助方法 ==========

// validateConfig 验证配置
func (s *TunnelServiceImpl) validateConfig(config *TunnelCreateConfig) error {
	if config.Name == "" {
		return errors.New("name is required")
	}
	if config.LocalPort <= 0 || config.LocalPort > 65535 {
		return errors.New("invalid local port")
	}
	if config.Protocol == "" {
		config.Protocol = "tcp"
	}
	if config.LocalHost == "" {
		config.LocalHost = "localhost"
	}

	// 验证提供商特定配置
	switch config.Provider {
	case ProviderCloudflare:
		if config.CloudflareToken == "" && s.config.CloudflareToken == "" {
			return errors.New("cloudflare token is required")
		}
	case ProviderFRP:
		if config.FRPServerAddr == "" && s.config.FRPServerAddr == "" {
			return errors.New("frp server address is required")
		}
	}

	return nil
}

// selectProvider 选择提供商
func (s *TunnelServiceImpl) selectProvider(config TunnelCreateConfig) TunnelProvider {
	// 优先 Cloudflare（免费且稳定）
	if s.config.CloudflareToken != "" || config.CloudflareToken != "" {
		return ProviderCloudflare
	}

	// 其次 FRP
	if s.config.FRPServerAddr != "" || config.FRPServerAddr != "" {
		return ProviderFRP
	}

	// 默认 Cloudflare
	return ProviderCloudflare
}

// createProviderInstance 创建提供商实例
func (s *TunnelServiceImpl) createProviderInstance(provider TunnelProvider, config TunnelCreateConfig) (ProviderInstance, error) {
	switch provider {
	case ProviderCloudflare:
		cfConfig := CloudflareTunnelConfig{
			Token:      config.CloudflareToken,
			AccountID:  config.CloudflareAccount,
			ZoneID:     config.CloudflareZone,
			ZoneName:   config.CloudflareZoneName,
			TunnelName: config.Name,
			Subdomain:  config.Subdomain,
			LocalPort:  config.LocalPort,
			LocalHost:  config.LocalHost,
			Protocol:   config.Protocol,
			ConfigPath: s.config.ConfigPath,
		}
		// 使用默认配置
		if cfConfig.Token == "" {
			cfConfig.Token = s.config.CloudflareToken
		}
		if cfConfig.AccountID == "" {
			cfConfig.AccountID = s.config.CloudflareAccount
		}
		return NewCloudflareTunnelInstance(cfConfig, s.logger)

	case ProviderFRP:
		frpConfig := FRPTunnelConfig{
			ServerAddr: config.FRPServerAddr,
			ServerPort: config.FRPServerPort,
			AuthToken:  config.FRPAuthToken,
			Name:       config.Name,
			LocalPort:  config.LocalPort,
			LocalHost:  config.LocalHost,
			RemotePort: config.RemotePort,
			Protocol:   config.Protocol,
			ConfigPath: s.config.ConfigPath,
		}
		// 使用默认配置
		if frpConfig.ServerAddr == "" {
			frpConfig.ServerAddr = s.config.FRPServerAddr
		}
		if frpConfig.ServerPort == 0 {
			frpConfig.ServerPort = s.config.FRPServerPort
		}
		if frpConfig.AuthToken == "" {
			frpConfig.AuthToken = s.config.FRPAuthToken
		}
		return NewFRPTunnelInstance(frpConfig, s.logger)

	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// runTunnel 运行隧道
func (s *TunnelServiceImpl) runTunnel(tunnel *managedTunnel) {
	// 记录启动日志
	tunnel.logBuffer.Add(LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   fmt.Sprintf("启动隧道 %s (提供商: %s)", tunnel.info.Name, tunnel.info.Provider),
		Source:    "system",
		TunnelID:  tunnel.info.ID,
	})

	// 启动提供商
	err := tunnel.provider.Start(tunnel.ctx)
	if err != nil {
		tunnel.mu.Lock()
		tunnel.info.State = StateError
		tunnel.status.Connected = false
		tunnel.status.LastError = err.Error()
		tunnel.mu.Unlock()

		tunnel.logBuffer.Add(LogEntry{
			Timestamp: time.Now(),
			Level:     "error",
			Message:   fmt.Sprintf("启动失败: %v", err),
			Source:    string(tunnel.info.Provider),
			TunnelID:  tunnel.info.ID,
		})

		return
	}

	// 更新状态为已连接
	tunnel.mu.Lock()
	tunnel.info.State = StateConnected
	tunnel.info.UpdatedAt = time.Now()
	tunnel.status.Connected = true
	tunnel.status.LastConnected = time.Now()
	tunnel.mu.Unlock()

	tunnel.logBuffer.Add(LogEntry{
		Timestamp: time.Now(),
		Level:     "info",
		Message:   "隧道已连接",
		Source:    string(tunnel.info.Provider),
		TunnelID:  tunnel.info.ID,
	})

	// 监控隧道状态
	for {
		select {
		case <-tunnel.ctx.Done():
			return
		case <-time.After(time.Duration(s.config.HeartbeatIntSec) * time.Second):
			s.checkTunnelHealth(tunnel)
		}
	}
}

// checkTunnelHealth 检查隧道健康
func (s *TunnelServiceImpl) checkTunnelHealth(tunnel *managedTunnel) {
	if tunnel.provider == nil {
		return
	}

	connected := tunnel.provider.IsConnected()
	tunnel.mu.Lock()
	defer tunnel.mu.Unlock()

	if !connected && tunnel.info.State == StateConnected {
		tunnel.info.State = StateReconnecting
		tunnel.status.Connected = false
		tunnel.logBuffer.Add(LogEntry{
			Timestamp: time.Now(),
			Level:     "warn",
			Message:   "连接断开，尝试重连",
			Source:    string(tunnel.info.Provider),
			TunnelID:  tunnel.info.ID,
		})
	} else if connected && tunnel.info.State != StateConnected {
		tunnel.info.State = StateConnected
		tunnel.status.Connected = true
		tunnel.status.LastConnected = time.Now()
		tunnel.logBuffer.Add(LogEntry{
			Timestamp: time.Now(),
			Level:     "info",
			Message:   "连接恢复",
			Source:    string(tunnel.info.Provider),
			TunnelID:  tunnel.info.ID,
		})
	}

	// 更新统计
	tunnel.status.UptimeSeconds = int64(time.Since(tunnel.startTime).Seconds())
}

// monitorLoop 监控循环
func (s *TunnelServiceImpl) monitorLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(s.config.HeartbeatIntSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.collectMetrics()
		}
	}
}

// collectMetrics 收集指标
func (s *TunnelServiceImpl) collectMetrics() {
	s.mu.RLock()
	tunnels := make([]*managedTunnel, 0, len(s.tunnels))
	for _, t := range s.tunnels {
		tunnels = append(tunnels, t)
	}
	s.mu.RUnlock()

	for _, t := range tunnels {
		if t.provider == nil {
			continue
		}

		status, err := t.provider.GetStatus()
		if err == nil {
			t.mu.Lock()
			t.status.BytesSent = status.BytesSent
			t.status.BytesReceived = status.BytesReceived
			t.status.Connections = status.Connections
			t.status.RequestCount = status.RequestCount
			t.status.LatencyMs = status.LatencyMs
			t.mu.Unlock()
		}
	}
}

// logCleanupLoop 日志清理循环
func (s *TunnelServiceImpl) logCleanupLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(time.Duration(s.config.LogRetentionSec) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.cleanupLogs()
		}
	}
}

// cleanupLogs 清理过期日志
func (s *TunnelServiceImpl) cleanupLogs() {
	s.mu.RLock()
	tunnels := make([]*managedTunnel, 0, len(s.tunnels))
	for _, t := range s.tunnels {
		tunnels = append(tunnels, t)
	}
	s.mu.RUnlock()

	cutoff := time.Now().Add(-time.Duration(s.config.LogRetentionSec) * time.Second)

	for _, t := range tunnels {
		t.logBuffer.mu.Lock()
		entries := make([]LogEntry, 0, len(t.logBuffer.entries))
		for _, e := range t.logBuffer.entries {
			if e.Timestamp.After(cutoff) {
				entries = append(entries, e)
			}
		}
		t.logBuffer.entries = entries
		t.logBuffer.mu.Unlock()
	}
}

// generateTunnelID 生成隧道 ID
func generateTunnelID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// ========== 持久化相关 ==========

// loadPersistedTunnels 加载持久化的隧道
func (s *TunnelServiceImpl) loadPersistedTunnels() error {
	// 实际实现需要从存储加载
	return nil
}

// persistTunnelConfig 持久化隧道配置
func (s *TunnelServiceImpl) persistTunnelConfig(id string, config TunnelCreateConfig) error {
	// 实际实现需要保存到存储
	return nil
}

// deletePersistedConfig 删除持久化配置
func (s *TunnelServiceImpl) deletePersistedConfig(id string) error {
	// 实际实现需要从存储删除
	return nil
}

// ========== 辅助类型 ==========

// CloudflareProviderConfig Cloudflare 提供商配置
type CloudflareProviderConfig struct {
	Token     string
	AccountID string
	ConfigPath string
}

// CloudflareTunnelConfig Cloudflare 隧道配置
type CloudflareTunnelConfig struct {
	Token      string
	AccountID  string
	ZoneID     string
	ZoneName   string
	TunnelName string
	Subdomain  string
	LocalPort  int
	LocalHost  string
	Protocol   string
	ConfigPath string
	MetricsPort int
}

// FRPProviderConfig FRP 提供商配置
type FRPProviderConfig struct {
	ServerAddr string
	ServerPort int
	AuthToken  string
	ConfigPath string
}

// FRPTunnelConfig FRP 隧道配置
type FRPTunnelConfig struct {
	ServerAddr string
	ServerPort int
	AuthToken  string
	Name       string
	LocalPort  int
	LocalHost  string
	RemotePort int
	Protocol   string
	ConfigPath string
}

// TunnelStats 隧道统计
type TunnelStats struct {
	BytesSent     int64
	BytesReceived int64
	Connections   int
	RequestCount  int64
	ErrorCount    int64
}

// ensure TunnelServiceImpl implements TunnelServiceInterface
var _ TunnelServiceInterface = (*TunnelServiceImpl)(nil)

// StreamLogs 返回实现 io.Reader 的接口
func (s *TunnelServiceImpl) GetLogReader(id string) (io.Reader, error) {
	s.mu.RLock()
	tunnel, exists := s.tunnelByID[id]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrTunnelNotFound
	}

	entries := tunnel.logBuffer.GetEntries()
	var buf []byte
	for _, e := range entries {
		buf = append(buf, []byte(fmt.Sprintf("[%s] %s %s: %s\n",
			e.Timestamp.Format(time.RFC3339),
			e.Level,
			e.Source,
			e.Message))...)
	}

	return &byteReader{data: buf}, nil
}

// byteReader 实现 io.Reader
type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}