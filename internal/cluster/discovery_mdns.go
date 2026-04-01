// Package cluster 提供 mDNS 服务发现实现
package cluster

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
	"go.uber.org/zap"
)

// MDNSServer mDNS 服务广播器
type MDNSServer struct {
	serviceName string
	port        int
	node        *NodeInfo
	server      *zeroconf.Server
	logger      *zap.Logger
	mu          sync.Mutex
}

// NewMDNSServer 创建 mDNS 服务器
func NewMDNSServer(serviceName string, port int, node *NodeInfo, logger *zap.Logger) *MDNSServer {
	return &MDNSServer{
		serviceName: serviceName,
		port:        port,
		node:        node,
		logger:      logger,
	}
}

// Start 启动 mDNS 广播
func (s *MDNSServer) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 构造服务文本记录
	text := []string{
		fmt.Sprintf("node_id=%s", s.node.ID),
		fmt.Sprintf("name=%s", s.node.Name),
		fmt.Sprintf("version=%s", s.node.Version),
		fmt.Sprintf("role=%s", s.node.Role),
		fmt.Sprintf("capabilities=%s", strings.Join(s.node.Capabilities, ",")),
	}

	// 注册 mDNS 服务
	server, err := zeroconf.Register(
		fmt.Sprintf("nas-os-%s", s.node.ID),
		s.serviceName,
		"local.",
		s.port,
		text,
		nil,
	)
	if err != nil {
		return fmt.Errorf("注册 mDNS 服务失败: %w", err)
	}

	s.server = server
	s.logger.Info("mDNS 服务已广播",
		zap.String("service", s.serviceName),
		zap.String("node_id", s.node.ID),
		zap.Int("port", s.port))

	return nil
}

// Stop 停止 mDNS 广播
func (s *MDNSServer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.server != nil {
		s.server.Shutdown()
		s.server = nil
	}
}

// UpdateText 更新服务文本记录
func (s *MDNSServer) UpdateText(node *NodeInfo) {
	s.mu.Lock()
	s.node = node
	s.mu.Unlock()

	// 需要重新注册以更新文本
	s.Stop()
	_ = s.Start()
}

// MDNSResolver mDNS 服务解析器
type MDNSResolver struct {
	serviceName string
	resolver    *zeroconf.Resolver
	entries     chan *zeroconf.ServiceEntry
	callbacks   MDNSResolverCallbacks
	ctx         context.Context
	cancel      context.CancelFunc
	logger      *zap.Logger
}

// MDNSResolverCallbacks mDNS 解析回调
type MDNSResolverCallbacks struct {
	OnNodeFound func(node *NodeInfo)
}

// NewMDNSResolver 创建 mDNS 解析器
func NewMDNSResolver(serviceName string, logger *zap.Logger) *MDNSResolver {
	ctx, cancel := context.WithCancel(context.Background())
	return &MDNSResolver{
		serviceName: serviceName,
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
	}
}

// Start 启动 mDNS 发现
func (r *MDNSResolver) Start(ctx context.Context) error {
	r.ctx = ctx

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return fmt.Errorf("创建 mDNS 解析器失败: %w", err)
	}

	r.resolver = resolver
	r.entries = make(chan *zeroconf.ServiceEntry, 10)

	// 启动发现循环
	go r.processEntries()

	// 启动浏览
	go func() {
		if err := r.resolver.Browse(r.ctx, r.serviceName, "local.", r.entries); err != nil {
			r.logger.Error("mDNS 浏览失败", zap.Error(err))
		}
	}()

	r.logger.Info("mDNS 解析器已启动", zap.String("service", r.serviceName))
	return nil
}

// Stop 停止 mDNS 发现
func (r *MDNSResolver) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
}

// processEntries 处理发现的条目
func (r *MDNSResolver) processEntries() {
	for {
		select {
		case <-r.ctx.Done():
			return
		case entry := <-r.entries:
			if entry == nil {
				continue
			}
			r.handleEntry(entry)
		}
	}
}

// handleEntry 处理单个服务条目
func (r *MDNSResolver) handleEntry(entry *zeroconf.ServiceEntry) {
	if len(entry.AddrIPv4) == 0 {
		return
	}

	// 解析文本记录
	node := &NodeInfo{
		Address: entry.AddrIPv4[0].String(),
		Port:    entry.Port,
	}

	for _, text := range entry.Text {
		parts := strings.SplitN(text, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := parts[0]
		value := parts[1]

		switch key {
		case "node_id":
			node.ID = value
		case "name":
			node.Name = value
		case "version":
			node.Version = value
		case "role":
			node.Role = value
		case "capabilities":
			node.Capabilities = strings.Split(value, ",")
		}
	}

	// 验证节点 ID
	if node.ID == "" {
		return
	}

	r.logger.Debug("mDNS 发现节点",
		zap.String("node_id", node.ID),
		zap.String("name", node.Name),
		zap.String("address", node.Address))

	// 触发回调
	if r.callbacks.OnNodeFound != nil {
		go r.callbacks.OnNodeFound(node)
	}
}

// SetCallbacks 设置回调
func (r *MDNSResolver) SetCallbacks(callbacks MDNSResolverCallbacks) {
	r.callbacks = callbacks
}

// DiscoverOnce 执行一次性发现
func (r *MDNSResolver) DiscoverOnce(timeout time.Duration) []*NodeInfo {
	ctx, cancel := context.WithTimeout(r.ctx, timeout)
	defer cancel()

	entries := make(chan *zeroconf.ServiceEntry, 10)
	nodes := make([]*NodeInfo, 0)

	go func() {
		if err := r.resolver.Browse(ctx, r.serviceName, "local.", entries); err != nil {
			r.logger.Error("mDNS 浏览失败", zap.Error(err))
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nodes
		case entry := <-entries:
			if entry == nil || len(entry.AddrIPv4) == 0 {
				continue
			}

			node := r.parseEntry(entry)
			if node != nil {
				nodes = append(nodes, node)
			}
		}
	}
}

// parseEntry 解析服务条目
func (r *MDNSResolver) parseEntry(entry *zeroconf.ServiceEntry) *NodeInfo {
	if len(entry.AddrIPv4) == 0 {
		return nil
	}

	node := &NodeInfo{
		Address: entry.AddrIPv4[0].String(),
		Port:    entry.Port,
	}

	for _, text := range entry.Text {
		parts := strings.SplitN(text, "=", 2)
		if len(parts) != 2 {
			continue
		}

		switch parts[0] {
		case "node_id":
			node.ID = parts[1]
		case "name":
			node.Name = parts[1]
		case "version":
			node.Version = parts[1]
		case "role":
			node.Role = parts[1]
		}
	}

	if node.ID == "" {
		return nil
	}

	return node
}

// StaticDiscovery 静态节点发现器
type StaticDiscovery struct {
	nodes []NodeEndpoint
}

// NewStaticDiscovery 创建静态发现器
func NewStaticDiscovery(nodes []NodeEndpoint) *StaticDiscovery {
	return &StaticDiscovery{
		nodes: nodes,
	}
}

// Discover 返回静态节点列表
func (s *StaticDiscovery) Discover() []NodeEndpoint {
	return s.nodes
}

// AddNode 添加静态节点
func (s *StaticDiscovery) AddNode(node NodeEndpoint) {
	s.nodes = append(s.nodes, node)
}

// RemoveNode 移除静态节点
func (s *StaticDiscovery) RemoveNode(nodeID string) {
	for i, node := range s.nodes {
		if node.ID == nodeID {
			s.nodes = append(s.nodes[:i], s.nodes[i+1:]...)
			return
		}
	}
}

// APIDiscovery API 节点发现器
type APIDiscovery struct {
	endpoint   string
	httpClient HTTPClient
	logger     *zap.Logger
}

// HTTPClient HTTP 客户端接口
type HTTPClient interface {
	Do(req interface{}) (interface{}, error)
}

// NewAPIDiscovery 创建 API 发现器
func NewAPIDiscovery(endpoint string, logger *zap.Logger) *APIDiscovery {
	return &APIDiscovery{
		endpoint: endpoint,
		logger:   logger,
	}
}

// Discover 从 API 发现节点
func (a *APIDiscovery) Discover() ([]*NodeInfo, error) {
	// 简化实现：返回空列表
	// 实际实现应调用 API 端点获取节点列表
	return nil, nil
}

// Helper functions

// parsePort 从字符串解析端口
func parsePort(s string) int {
	port, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return port
}