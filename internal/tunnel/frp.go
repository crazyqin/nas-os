// Package tunnel 提供 FRP 内网穿透集成
// 实现零配置远程访问，参考飞牛fnOS FN Connect
package tunnel

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// FRPConfig FRP配置
type FRPConfig struct {
	Enabled       bool     `json:"enabled"`       // 是否启用
	ServerAddr    string   `json:"serverAddr"`    // FRP服务器地址
	ServerPort    int      `json:"serverPort"`    // FRP服务器端口
	Token         string   `json:"token"`         // 认证令牌
	DeviceID      string   `json:"deviceId"`      // 设备ID
	DeviceName    string   `json:"deviceName"`    // 设备名称
	STUNServers   []string `json:"stunServers"`   // STUN服务器列表
	AutoReconnect bool     `json:"autoReconnect"` // 自动重连
	LogLevel      string   `json:"logLevel"`      // 日志级别
}

// FRPProxyConfig 代理配置
type FRPProxyConfig struct {
	Name          string            `json:"name"`          // 代理名称
	Type          string            `json:"type"`          // 类型: tcp, udp, http, https
	LocalIP       string            `json:"localIp"`       // 本地IP
	LocalPort     int               `json:"localPort"`     // 本地端口
	RemotePort    int               `json:"remotePort"`    // 远程端口（tcp/udp）
	CustomDomains []string          `json:"customDomains"` // 自定义域名（http/https）
	Subdomain     string            `json:"subdomain"`     // 子域名
	Headers       map[string]string `json:"headers"`       // HTTP头
	EnableTLS     bool              `json:"enableTls"`     // 启用TLS
}

// FRPStatus FRP状态
type FRPStatus struct {
	Connected     bool             `json:"connected"`     // 是否已连接
	ServerAddr    string           `json:"serverAddr"`    // 服务器地址
	DeviceID      string           `json:"deviceId"`      // 设备ID
	PublicURL     string           `json:"publicUrl"`     // 公网访问地址
	Uptime        time.Duration    `json:"uptime"`        // 运行时间
	ProxyCount    int              `json:"proxyCount"`    // 代理数量
	LastConnected time.Time        `json:"lastConnected"` // 最后连接时间
	ErrorMessage  string           `json:"errorMessage"`  // 错误信息
	Proxies       []FRPProxyStatus `json:"proxies"`       // 代理状态列表
}

// FRPProxyStatus 代理状态
type FRPProxyStatus struct {
	Name       string    `json:"name"`       // 代理名称
	Type       string    `json:"type"`       // 类型
	Status     string    `json:"status"`     // 状态: running, offline, error
	LocalAddr  string    `json:"localAddr"`  // 本地地址
	RemoteAddr string    `json:"remoteAddr"` // 远程地址
	TrafficIn  uint64    `json:"trafficIn"`  // 入流量
	TrafficOut uint64    `json:"trafficOut"` // 出流量
	LastActive time.Time `json:"lastActive"` // 最后活动时间
}

// FRPManager FRP管理器
type FRPManager struct {
	config         *FRPConfig
	proxyConfigs   map[string]*FRPProxyConfig
	status         FRPStatus
	cmd            *exec.Cmd
	configPath     string
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
	logger         *zap.Logger
	onStatusChange func(status FRPStatus)
}

// NewFRPManager 创建FRP管理器
func NewFRPManager(config *FRPConfig, logger *zap.Logger) *FRPManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &FRPManager{
		config:       config,
		proxyConfigs: make(map[string]*FRPProxyConfig),
		status: FRPStatus{
			Connected: false,
		},
		ctx:    ctx,
		cancel: cancel,
		logger: logger,
	}
}

// Start 启动FRP客户端
func (m *FRPManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status.Connected {
		return fmt.Errorf("FRP客户端已在运行")
	}

	// 生成配置文件
	if err := m.generateConfig(); err != nil {
		return fmt.Errorf("生成配置文件失败: %w", err)
	}

	// 启动frpc进程
	m.cmd = exec.CommandContext(m.ctx, "frpc", "-c", m.configPath)
	m.cmd.Stdout = &frpLogWriter{logger: m.logger}
	m.cmd.Stderr = &frpLogWriter{logger: m.logger}

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("启动FRP客户端失败: %w", err)
	}

	m.status.Connected = true
	m.status.LastConnected = time.Now()
	m.status.DeviceID = m.config.DeviceID
	m.status.ServerAddr = m.config.ServerAddr

	// 启动状态监控
	go m.monitorStatus()

	m.logger.Info("FRP客户端已启动",
		zap.String("device", m.config.DeviceID),
		zap.String("server", m.config.ServerAddr))

	return nil
}

// Stop 停止FRP客户端
func (m *FRPManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.status.Connected {
		return nil
	}

	if m.cmd != nil && m.cmd.Process != nil {
		if err := m.cmd.Process.Signal(os.Interrupt); err != nil {
			_ = m.cmd.Process.Kill()
		}
	}

	m.status.Connected = false
	m.logger.Info("FRP客户端已停止")

	return nil
}

// Restart 重启FRP客户端
func (m *FRPManager) Restart() error {
	if err := m.Stop(); err != nil {
		return err
	}
	time.Sleep(1 * time.Second)
	return m.Start()
}

// AddProxy 添加代理
func (m *FRPManager) AddProxy(proxy *FRPProxyConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.proxyConfigs[proxy.Name]; exists {
		return fmt.Errorf("代理 %s 已存在", proxy.Name)
	}

	m.proxyConfigs[proxy.Name] = proxy
	m.status.ProxyCount = len(m.proxyConfigs)

	// 如果已连接，重新生成配置并重启
	if m.status.Connected {
		if err := m.generateConfig(); err != nil {
			return err
		}
		go func() { _ = m.Restart() }()
	}

	m.logger.Info("添加代理", zap.String("name", proxy.Name))
	return nil
}

// RemoveProxy 移除代理
func (m *FRPManager) RemoveProxy(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.proxyConfigs[name]; !exists {
		return fmt.Errorf("代理 %s 不存在", name)
	}

	delete(m.proxyConfigs, name)
	m.status.ProxyCount = len(m.proxyConfigs)

	// 如果已连接，重新生成配置并重启
	if m.status.Connected {
		if err := m.generateConfig(); err != nil {
			return err
		}
		go func() { _ = m.Restart() }()
	}

	m.logger.Info("移除代理", zap.String("name", name))
	return nil
}

// ListProxies 列出代理
func (m *FRPManager) ListProxies() []*FRPProxyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*FRPProxyConfig, 0, len(m.proxyConfigs))
	for _, p := range m.proxyConfigs {
		result = append(result, p)
	}
	return result
}

// GetStatus 获取状态
func (m *FRPManager) GetStatus() FRPStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status
}

// SetStatusCallback 设置状态回调
func (m *FRPManager) SetStatusCallback(callback func(status FRPStatus)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onStatusChange = callback
}

// generateConfig 生成FRP配置文件
func (m *FRPManager) generateConfig() error {
	// 确保配置目录存在
	configDir := "/var/lib/nas-os/frp"
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return err
	}
	m.configPath = filepath.Join(configDir, "frpc.toml")

	// 构建配置
	config := m.buildTOMLConfig()

	// 写入文件
	if err := os.WriteFile(m.configPath, []byte(config), 0600); err != nil {
		return err
	}

	return nil
}

// buildTOMLConfig 构建TOML配置
func (m *FRPManager) buildTOMLConfig() string {
	var sb strings.Builder

	// 基础配置
	sb.WriteString("# NAS-OS FRP Client Configuration\n")
	sb.WriteString("# Auto-generated, do not edit manually\n\n")

	sb.WriteString("[common]\n")
	fmt.Fprintf(&sb, "serverAddr = \"%s\"\n", m.config.ServerAddr)
	fmt.Fprintf(&sb, "serverPort = %d\n", m.config.ServerPort)

	if m.config.Token != "" {
		fmt.Fprintf(&sb, "auth.token = \"%s\"\n", m.config.Token)
	}

	if m.config.DeviceID != "" {
		fmt.Fprintf(&sb, "user = \"%s\"\n", m.config.DeviceID)
	}

	fmt.Fprintf(&sb, "log.level = \"%s\"\n", m.config.LogLevel)
	sb.WriteString("transport.tls.disableCustomTLSFirstByte = false\n\n")

	// 代理配置
	for name, proxy := range m.proxyConfigs {
		sb.WriteString("[[proxies]]\n")
		fmt.Fprintf(&sb, "name = \"%s\"\n", name)
		fmt.Fprintf(&sb, "type = \"%s\"\n", proxy.Type)
		fmt.Fprintf(&sb, "localIP = \"%s\"\n", proxy.LocalIP)
		fmt.Fprintf(&sb, "localPort = %d\n", proxy.LocalPort)

		switch proxy.Type {
		case "tcp", "udp":
			fmt.Fprintf(&sb, "remotePort = %d\n", proxy.RemotePort)
		case "http", "https":
			if proxy.Subdomain != "" {
				fmt.Fprintf(&sb, "subdomain = \"%s\"\n", proxy.Subdomain)
			}
			for _, domain := range proxy.CustomDomains {
				fmt.Fprintf(&sb, "customDomains = [\"%s\"]\n", domain)
			}
			if proxy.EnableTLS {
				sb.WriteString("transport.tls.enable = true\n")
			}
		}

		// HTTP头
		for k, v := range proxy.Headers {
			fmt.Fprintf(&sb, "proxyProtocol.header.%s = \"%s\"\n", k, v)
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// monitorStatus 监控状态
func (m *FRPManager) monitorStatus() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkConnection()
		}
	}
}

// checkConnection 检查连接状态
func (m *FRPManager) checkConnection() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查进程是否存活
	if m.cmd == nil || m.cmd.Process == nil {
		m.status.Connected = false
		m.status.ErrorMessage = "进程未运行"
		return
	}

	// 检查进程状态
	if err := m.cmd.Process.Signal(syscall.Signal(0)); err != nil {
		m.status.Connected = false
		m.status.ErrorMessage = "进程已退出"

		// 自动重连
		if m.config.AutoReconnect {
			go m.reconnect()
		}
		return
	}

	// 更新运行时间
	if m.status.Connected {
		m.status.Uptime = time.Since(m.status.LastConnected)
	}

	// 通知状态变化
	if m.onStatusChange != nil {
		go m.onStatusChange(m.status)
	}
}

// reconnect 重连
func (m *FRPManager) reconnect() {
	for i := 0; i < 5; i++ {
		m.logger.Info("尝试重新连接FRP服务器", zap.Int("attempt", i+1))

		if err := m.Start(); err == nil {
			m.logger.Info("FRP服务器重连成功")
			return
		}

		time.Sleep(time.Duration(i+1) * 5 * time.Second)
	}

	m.logger.Error("FRP服务器重连失败")
}

// ========== 零配置API ==========

// QuickConnect 零配置快速连接
func (m *FRPManager) QuickConnect(localPort int, serviceName string) (*QuickConnectResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 自动生成代理名称
	proxyName := fmt.Sprintf("%s-%d-%s", m.config.DeviceID, localPort, serviceName)

	// 自动分配远程端口
	remotePort := 10000 + (localPort % 55535)

	proxy := &FRPProxyConfig{
		Name:       proxyName,
		Type:       "tcp",
		LocalIP:    "127.0.0.1",
		LocalPort:  localPort,
		RemotePort: remotePort,
	}

	m.proxyConfigs[proxyName] = proxy
	m.status.ProxyCount = len(m.proxyConfigs)

	// 生成配置并重启
	if err := m.generateConfig(); err != nil {
		return nil, err
	}

	if m.status.Connected {
		go func() { _ = m.Restart() }()
	}

	// 构建公网地址
	publicURL := fmt.Sprintf("%s:%d", m.config.ServerAddr, remotePort)

	return &QuickConnectResult{
		ProxyName:  proxyName,
		PublicURL:  publicURL,
		LocalPort:  localPort,
		RemotePort: remotePort,
	}, nil
}

// QuickConnectResult 快速连接结果
type QuickConnectResult struct {
	ProxyName  string `json:"proxyName"`  // 代理名称
	PublicURL  string `json:"publicUrl"`  // 公网访问地址
	LocalPort  int    `json:"localPort"`  // 本地端口
	RemotePort int    `json:"remotePort"` // 远程端口
}

// ========== Web Dashboard API ==========

// GetDashboardData 获取仪表盘数据
func (m *FRPManager) GetDashboardData() *FRPDashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proxies := make([]FRPProxyStatus, 0)
	for name, cfg := range m.proxyConfigs {
		proxies = append(proxies, FRPProxyStatus{
			Name:       name,
			Type:       cfg.Type,
			Status:     "running",
			LocalAddr:  fmt.Sprintf("%s:%d", cfg.LocalIP, cfg.LocalPort),
			RemoteAddr: fmt.Sprintf("%s:%d", m.config.ServerAddr, cfg.RemotePort),
		})
	}

	return &FRPDashboard{
		Status:       m.status,
		Proxies:      proxies,
		ProxyCount:   len(proxies),
		TotalTraffic: m.calculateTotalTraffic(),
	}
}

// FRPDashboard FRP仪表盘
type FRPDashboard struct {
	Status       FRPStatus        `json:"status"`
	Proxies      []FRPProxyStatus `json:"proxies"`
	ProxyCount   int              `json:"proxyCount"`
	TotalTraffic uint64           `json:"totalTraffic"`
}

// calculateTotalTraffic 计算总流量
func (m *FRPManager) calculateTotalTraffic() uint64 {
	var total uint64
	for _, p := range m.status.Proxies {
		total += p.TrafficIn + p.TrafficOut
	}
	return total
}

// ========== 日志写入器 ==========

type frpLogWriter struct {
	logger *zap.Logger
}

func (w *frpLogWriter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if msg == "" {
		return len(p), nil
	}

	// 根据日志级别记录
	if strings.Contains(msg, "error") || strings.Contains(msg, "ERROR") {
		w.logger.Error(msg)
	} else if strings.Contains(msg, "warn") || strings.Contains(msg, "WARN") {
		w.logger.Warn(msg)
	} else {
		w.logger.Info(msg)
	}

	return len(p), nil
}
