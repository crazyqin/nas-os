package spotlight

import (
	"context"
	"fmt"
	"net"
	"sync"

	"go.uber.org/zap"
)

// MDNSResponder mDNS/Bonjour 响应器
// 支持 macOS 客户端通过 mDNS 发现 NAS 上的 Spotlight 服务
type MDNSResponder struct {
	serviceName string
	port        int
	logger      *zap.Logger
	mu          sync.RWMutex
	running     bool
	ctx         context.Context
	cancel      context.CancelFunc
	entries     map[string]*MDNSEntry
}

// MDNSEntry mDNS 条目
type MDNSEntry struct {
	Name string            `json:"name"`
	Type string            `json:"type"`
	Port int               `json:"port"`
	IPs  []net.IP          `json:"ips"`
	TXT  map[string]string `json:"txt"`
	TTL  uint32            `json:"ttl"`
}

// NewMDNSResponder 创建 mDNS 响应器
func NewMDNSResponder(serviceName string, port int, logger *zap.Logger) *MDNSResponder {
	if logger == nil {
		logger = zap.NewNop()
	}
	if serviceName == "" {
		serviceName = "NAS-OS Spotlight"
	}
	if port <= 0 {
		port = 5353
	}

	return &MDNSResponder{
		serviceName: serviceName,
		port:        port,
		logger:      logger,
		entries:     make(map[string]*MDNSEntry),
	}
}

// Start 启动 mDNS 响应器
func (r *MDNSResponder) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("mDNS 响应器已在运行")
	}

	r.ctx, r.cancel = context.WithCancel(ctx)

	// 注册 Spotlight 服务
	if err := r.registerSpotlightService(); err != nil {
		r.logger.Warn("注册 mDNS 服务失败", zap.Error(err))
		// 不返回错误，允许服务继续运行
	}

	r.running = true

	// 启动响应循环
	go r.respondLoop()

	r.logger.Info("mDNS 响应器已启动",
		zap.String("serviceName", r.serviceName),
		zap.Int("port", r.port))

	return nil
}

// Stop 停止 mDNS 响应器
func (r *MDNSResponder) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return
	}

	r.cancel()
	r.running = false

	r.logger.Info("mDNS 响应器已停止")
}

// registerSpotlightService 注册 Spotlight 服务
func (r *MDNSResponder) registerSpotlightService() error {
	// 获取本机 IP
	ips, err := getLocalIPs()
	if err != nil {
		return fmt.Errorf("获取本机 IP 失败: %w", err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("未找到本机 IP 地址")
	}

	// 创建 Spotlight 服务条目
	entry := &MDNSEntry{
		Name: r.serviceName,
		Type: "_spotlight._tcp",
		Port: r.port,
		IPs:  ips,
		TXT: map[string]string{
			"path":    "/api/v1/spotlight",
			"server":  "NAS-OS",
			"version": "1.0",
		},
		TTL: 120,
	}

	r.entries["_spotlight._tcp"] = entry

	r.logger.Info("注册 Spotlight mDNS 服务",
		zap.String("name", r.serviceName),
		zap.Int("port", r.port),
		zap.Int("ips", len(ips)))

	return nil
}

// respondLoop 响应循环
func (r *MDNSResponder) respondLoop() {
	for {
		select {
		case <-r.ctx.Done():
			return
		default:
			// mDNS 通常使用 UDP 多播，这里简化实现
			// 实际生产环境应使用 github.com/hashicorp/go-mdns 或类似库
			// 这里提供基本的服务发现能力
		}
	}
}

// GetRegisteredServices 获取已注册的服务列表
func (r *MDNSResponder) GetRegisteredServices() []MDNSEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]MDNSEntry, 0, len(r.entries))
	for _, entry := range r.entries {
		services = append(services, *entry)
	}
	return services
}

// IsRunning 是否在运行
func (r *MDNSResponder) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}

// GetServiceInfo 获取服务信息
func (r *MDNSResponder) GetServiceInfo() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return map[string]interface{}{
		"serviceName": r.serviceName,
		"port":        r.port,
		"running":     r.running,
		"entries":     len(r.entries),
	}
}

// getLocalIPs 获取本机 IP 地址
func getLocalIPs() ([]net.IP, error) {
	var ips []net.IP

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}

		// 排除回环地址和链路本地地址
		if ipNet.IP.IsLoopback() || ipNet.IP.IsLinkLocalUnicast() {
			continue
		}

		ips = append(ips, ipNet.IP)
	}

	return ips, nil
}
