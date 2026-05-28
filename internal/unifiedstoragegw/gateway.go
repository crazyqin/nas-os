// Package unifiedstoragegw 提供统一存储协议网关功能
// 支持 NFS/SMB/iSCSI/S3 多协议统一接入，自动协议协商
package unifiedstoragegw

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ==================== 类型定义 ====================

// Protocol 存储协议
type Protocol string

const (
	ProtocolNFS   Protocol = "nfs"
	ProtocolSMB   Protocol = "smb"
	ProtocolISCSI Protocol = "iscsi"
	ProtocolS3    Protocol = "s3"
	ProtocolAFP   Protocol = "afp"
)

// ProtocolStatus 协议状态
type ProtocolStatus string

const (
	StatusRunning  ProtocolStatus = "running"
	StatusStopped  ProtocolStatus = "stopped"
	StatusError    ProtocolStatus = "error"
	StatusStarting ProtocolStatus = "starting"
)

// GatewayConfig 网关配置
type GatewayConfig struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	ListenAddr  string            `json:"listenAddr"`
	ListenPort  int               `json:"listenPort"`
	Protocol    Protocol          `json:"protocol"`
	Enabled     bool              `json:"enabled"`
	Options     map[string]string `json:"options,omitempty"`
	TLSEnabled  bool              `json:"tlsEnabled"`
	TLSCertPath string            `json:"tlsCertPath,omitempty"`
	TLSKeyPath  string            `json:"tlsKeyPath,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// ProtocolEndpoint 协议端点
type ProtocolEndpoint struct {
	Protocol    Protocol       `json:"protocol"`
	Addr        string         `json:"addr"`
	Port        int            `json:"port"`
	Status      ProtocolStatus `json:"status"`
	Connections int            `json:"connections"`
	StartedAt   *time.Time     `json:"startedAt,omitempty"`
	ErrorMsg     string         `json:"errorMsg,omitempty"`
}

// ShareDefinition 共享定义
type ShareDefinition struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Protocols   []Protocol `json:"protocols"`
	ReadOnly    bool     `json:"readOnly"`
	GuestAccess bool     `json:"guestAccess"`
	ACL         []ACLEntry `json:"acl,omitempty"`
	Browseable  bool     `json:"browseable"`
	Comment     string   `json:"comment,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ACLEntry ACL 条目
type ACLEntry struct {
	Principal string `json:"principal"` // user:xxx 或 group:xxx
	Permission string `json:"permission"` // read, write, admin
}

// ClientConnection 客户端连接
type ClientConnection struct {
	ID         string    `json:"id"`
	Protocol   Protocol  `json:"protocol"`
	ClientAddr string    `json:"clientAddr"`
	ShareName  string    `json:"shareName"`
	UserName   string    `json:"userName,omitempty"`
	ConnectedAt time.Time `json:"connectedAt"`
	BytesRead  int64     `json:"bytesRead"`
	BytesWrite int64     `json:"bytesWrite"`
}

// ProtocolStats 协议统计
type ProtocolStats struct {
	Protocol       Protocol `json:"protocol"`
	TotalConns     int64    `json:"totalConns"`
	ActiveConns    int      `json:"activeConns"`
	BytesRead      int64    `json:"bytesRead"`
	BytesWritten   int64    `json:"bytesWritten"`
	ErrorCount     int64    `json:"errorCount"`
	AvgLatencyMs   float64  `json:"avgLatencyMs"`
	UptimeSeconds  int64    `json:"uptimeSeconds"`
}

// GatewayStatus 网关状态
type GatewayStatus struct {
	Endpoints   []*ProtocolEndpoint `json:"endpoints"`
	Connections int                 `json:"connections"`
	Shares      int                 `json:"shares"`
	Protocols   []Protocol          `json:"activeProtocols"`
}

// ==================== 网关管理器 ====================

// Gateway 统一存储网关
type Gateway struct {
	mu sync.RWMutex

	// 端点管理
	endpoints map[Protocol]*ProtocolEndpoint

	// 共享管理
	shares map[string]*ShareDefinition

	// 连接跟踪
	connections map[string]*ClientConnection

	// 协议统计
	stats map[Protocol]*ProtocolStatus

	// 配置
	configs map[string]*GatewayConfig

	// 事件日志
	events []GatewayEvent
}

// GatewayEvent 网关事件
type GatewayEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Protocol  Protocol  `json:"protocol,omitempty"`
	Event     string    `json:"event"`
	Details   string    `json:"details"`
	Client    string    `json:"client,omitempty"`
}

// NewGateway 创建统一存储网关
func NewGateway() *Gateway {
	gw := &Gateway{
		endpoints:   make(map[Protocol]*ProtocolEndpoint),
		shares:      make(map[string]*ShareDefinition),
		connections: make(map[string]*ClientConnection),
		stats:       make(map[Protocol]*ProtocolStatus),
		configs:     make(map[string]*GatewayConfig),
	}

	// 初始化默认端点
	gw.initDefaultEndpoints()

	return gw
}

// initDefaultEndpoints 初始化默认端点
func (gw *Gateway) initDefaultEndpoints() {
	defaults := []struct {
		protocol Protocol
		port     int
	}{
		{ProtocolNFS, 2049},
		{ProtocolSMB, 445},
		{ProtocolISCSI, 3260},
		{ProtocolS3, 9000},
	}

	for _, d := range defaults {
		gw.endpoints[d.protocol] = &ProtocolEndpoint{
			Protocol: d.protocol,
			Port:     d.port,
			Status:   StatusStopped,
		}
	}
}

// ==================== 协议管理 ====================

// StartProtocol 启动协议
func (gw *Gateway) StartProtocol(protocol Protocol, addr string, port int) error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	ep, exists := gw.endpoints[protocol]
	if !exists {
		ep = &ProtocolEndpoint{Protocol: protocol}
		gw.endpoints[protocol] = ep
	}

	if ep.Status == StatusRunning {
		return fmt.Errorf("协议 %s 已在运行", protocol)
	}

	ep.Addr = addr
	ep.Port = port
	ep.Status = StatusRunning
	now := time.Now()
	ep.StartedAt = &now
	ep.ErrorMsg = ""

	gw.addEvent(protocol, "protocol_start", fmt.Sprintf("启动 %s 服务于 %s:%d", protocol, addr, port))

	log.Printf("[统一网关] 启动 %s 服务: %s:%d", protocol, addr, port)
	return nil
}

// StopProtocol 停止协议
func (gw *Gateway) StopProtocol(protocol Protocol) error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	ep, exists := gw.endpoints[protocol]
	if !exists {
		return fmt.Errorf("协议 %s 不存在", protocol)
	}

	if ep.Status != StatusRunning {
		return fmt.Errorf("协议 %s 未在运行", protocol)
	}

	ep.Status = StatusStopped
	ep.StartedAt = nil

	gw.addEvent(protocol, "protocol_stop", fmt.Sprintf("停止 %s 服务", protocol))

	log.Printf("[统一网关] 停止 %s 服务", protocol)
	return nil
}

// GetProtocolStatus 获取协议状态
func (gw *Gateway) GetProtocolStatus(protocol Protocol) (*ProtocolEndpoint, error) {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	ep, exists := gw.endpoints[protocol]
	if !exists {
		return nil, fmt.Errorf("协议 %s 不存在", protocol)
	}
	return ep, nil
}

// ListEndpoints 列出所有端点
func (gw *Gateway) ListEndpoints() []*ProtocolEndpoint {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	eps := make([]*ProtocolEndpoint, 0, len(gw.endpoints))
	for _, ep := range gw.endpoints {
		eps = append(eps, ep)
	}
	return eps
}

// ==================== 共享管理 ====================

// CreateShare 创建共享
func (gw *Gateway) CreateShare(share *ShareDefinition) error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	if share.ID == "" {
		return fmt.Errorf("共享 ID 不能为空")
	}

	if _, exists := gw.shares[share.ID]; exists {
		return fmt.Errorf("共享 %s 已存在", share.ID)
	}

	share.CreatedAt = time.Now()
	gw.shares[share.ID] = share

	gw.addEvent("", "share_create", fmt.Sprintf("创建共享: %s -> %s", share.Name, share.Path))

	log.Printf("[统一网关] 创建共享: %s (%s)", share.Name, share.Path)
	return nil
}

// DeleteShare 删除共享
func (gw *Gateway) DeleteShare(id string) error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	if _, exists := gw.shares[id]; !exists {
		return fmt.Errorf("共享 %s 不存在", id)
	}

	delete(gw.shares, id)
	gw.addEvent("", "share_delete", fmt.Sprintf("删除共享: %s", id))

	log.Printf("[统一网关] 删除共享: %s", id)
	return nil
}

// GetShare 获取共享
func (gw *Gateway) GetShare(id string) (*ShareDefinition, error) {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	share, exists := gw.shares[id]
	if !exists {
		return nil, fmt.Errorf("共享 %s 不存在", id)
	}
	return share, nil
}

// ListShares 列出所有共享
func (gw *Gateway) ListShares() []*ShareDefinition {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	shares := make([]*ShareDefinition, 0, len(gw.shares))
	for _, s := range gw.shares {
		shares = append(shares, s)
	}
	return shares
}

// UpdateShareACL 更新共享 ACL
func (gw *Gateway) UpdateShareACL(id string, acl []ACLEntry) error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	share, exists := gw.shares[id]
	if !exists {
		return fmt.Errorf("共享 %s 不存在", id)
	}

	share.ACL = acl
	gw.addEvent("", "share_update_acl", fmt.Sprintf("更新共享 ACL: %s", id))

	log.Printf("[统一网关] 更新共享 ACL: %s, 条目: %d", id, len(acl))
	return nil
}

// ==================== 连接管理 ====================

// RegisterConnection 注册连接
func (gw *Gateway) RegisterConnection(conn *ClientConnection) {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	if conn.ID == "" {
		conn.ID = fmt.Sprintf("%s-%s-%d", conn.Protocol, conn.ClientAddr, time.Now().UnixNano())
	}
	conn.ConnectedAt = time.Now()

	gw.connections[conn.ID] = conn

	ep, exists := gw.endpoints[conn.Protocol]
	if exists {
		ep.Connections++
	}

	gw.addEvent(conn.Protocol, "client_connect",
		fmt.Sprintf("客户端连接: %s -> %s (用户: %s)", conn.ClientAddr, conn.ShareName, conn.UserName))
}

// UnregisterConnection 注销连接
func (gw *Gateway) UnregisterConnection(connID string) {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	conn, exists := gw.connections[connID]
	if !exists {
		return
	}

	ep, exists := gw.endpoints[conn.Protocol]
	if exists && ep.Connections > 0 {
		ep.Connections--
	}

	delete(gw.connections, connID)

	gw.addEvent(conn.Protocol, "client_disconnect",
		fmt.Sprintf("客户端断开: %s (读: %d bytes, 写: %d bytes)",
			conn.ClientAddr, conn.BytesRead, conn.BytesWrite))
}

// GetActiveConnections 获取活跃连接
func (gw *Gateway) GetActiveConnections() []*ClientConnection {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	conns := make([]*ClientConnection, 0, len(gw.connections))
	for _, c := range gw.connections {
		conns = append(conns, c)
	}
	return conns
}

// GetConnectionsByProtocol 按协议获取连接
func (gw *Gateway) GetConnectionsByProtocol(protocol Protocol) []*ClientConnection {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	var conns []*ClientConnection
	for _, c := range gw.connections {
		if c.Protocol == protocol {
			conns = append(conns, c)
		}
	}
	return conns
}

// ==================== 状态查询 ====================

// GetGatewayStatus 获取网关状态
func (gw *Gateway) GetGatewayStatus() *GatewayStatus {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	status := &GatewayStatus{
		Connections: len(gw.connections),
		Shares:      len(gw.shares),
	}

	for _, ep := range gw.endpoints {
		status.Endpoints = append(status.Endpoints, ep)
		if ep.Status == StatusRunning {
			status.Protocols = append(status.Protocols, ep.Protocol)
		}
	}

	return status
}

// GetProtocolStats 获取协议统计
func (gw *Gateway) GetProtocolStats(protocol Protocol) *ProtocolStats {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	ep, exists := gw.endpoints[protocol]
	if !exists {
		return nil
	}

	stats := &ProtocolStats{
		Protocol:    protocol,
		ActiveConns: ep.Connections,
	}

	if ep.StartedAt != nil {
		stats.UptimeSeconds = int64(time.Since(*ep.StartedAt).Seconds())
	}

	// 统计连接数据
	for _, conn := range gw.connections {
		if conn.Protocol == protocol {
			stats.TotalConns++
			stats.BytesRead += conn.BytesRead
			stats.BytesWritten += conn.BytesWrite
		}
	}

	return stats
}

// GetEvents 获取事件日志
func (gw *Gateway) GetEvents(limit int) []GatewayEvent {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	if limit <= 0 || limit > len(gw.events) {
		limit = len(gw.events)
	}

	start := len(gw.events) - limit
	if start < 0 {
		start = 0
	}
	return gw.events[start:]
}

// ==================== 协议协商 ====================

// NegotiateProtocol 协商最佳协议
func (gw *Gateway) NegotiateProtocol(clientAddr string, capabilities []Protocol) Protocol {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	// 优先级: NFS > SMB > iSCSI > S3
	priority := []Protocol{ProtocolNFS, ProtocolSMB, ProtocolISCSI, ProtocolS3}

	for _, p := range priority {
		ep, exists := gw.endpoints[p]
		if !exists || ep.Status != StatusRunning {
			continue
		}

		// 检查客户端能力
		for _, cap := range capabilities {
			if cap == p {
				return p
			}
		}
	}

	// 默认返回 SMB（最广泛支持）
	return ProtocolSMB
}

// GetBestProtocolForUseCase 根据使用场景推荐协议
func GetBestProtocolForUseCase(useCase string) Protocol {
	switch useCase {
	case "vm_storage", "block_storage":
		return ProtocolISCSI
	case "file_sharing", "documents":
		return ProtocolSMB
	case "linux_nfs", "container_storage":
		return ProtocolNFS
	case "object_storage", "backup_target":
		return ProtocolS3
	default:
		return ProtocolSMB
	}
}

// ==================== 辅助方法 ====================

// addEvent 添加事件
func (gw *Gateway) addEvent(protocol Protocol, event, details string) {
	gw.events = append(gw.events, GatewayEvent{
		Timestamp: time.Now(),
		Protocol:  protocol,
		Event:     event,
		Details:   details,
	})

	// 保留最近 1000 条事件
	if len(gw.events) > 1000 {
		gw.events = gw.events[len(gw.events)-1000:]
	}
}

// IsProtocolSupported 检查协议是否支持
func IsProtocolSupported(protocol Protocol) bool {
	switch protocol {
	case ProtocolNFS, ProtocolSMB, ProtocolISCSI, ProtocolS3, ProtocolAFP:
		return true
	default:
		return false
	}
}

// GetProtocolDefaultPort 获取协议默认端口
func GetProtocolDefaultPort(protocol Protocol) int {
	switch protocol {
	case ProtocolNFS:
		return 2049
	case ProtocolSMB:
		return 445
	case ProtocolISCSI:
		return 3260
	case ProtocolS3:
		return 9000
	case ProtocolAFP:
		return 548
	default:
		return 0
	}
}
