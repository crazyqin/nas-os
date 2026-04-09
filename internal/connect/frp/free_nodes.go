// Package frp provides FRP client implementation
// 免费公共服务器节点配置
package frp

import (
	"fmt"
	"sync"
	"time"
)

// NodeRegion 节点区域
type NodeRegion string

const (
	RegionCN NodeRegion = "cn" // 中国
	RegionUS NodeRegion = "us" // 美国
	RegionEU NodeRegion = "eu" // 欧洲
)

// FreeNode 免费节点信息
type FreeNode struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Region      NodeRegion `json:"region"`
	ServerAddr  string     `json:"server_addr"`
	ServerPort  int        `json:"server_port"`
	TLSEnable   bool       `json:"tls_enable"`
	Description string     `json:"description"`
	Priority    int        `json:"priority"` // 优先级，数字越小优先级越高
	Online      bool       `json:"online"`    // 是否在线
	Latency     int        `json:"latency"`   // 延迟（毫秒）
	LastCheck   time.Time  `json:"last_check"`
}

// FreeNodeConfig 免费节点配置
type FreeNodeConfig struct {
	nodes    map[string]*FreeNode
	regionMap map[NodeRegion][]string // 区域到节点ID的映射
	mu       sync.RWMutex
}

// DefaultFreeNodes 默认免费节点列表（参考飞牛FN Connect）
var defaultFreeNodes = []*FreeNode{
	// 中国节点
	{
		ID:          "cn-connect-1",
		Name:        "FN Connect CN-1",
		Region:      RegionCN,
		ServerAddr:  "connect.fnos.cn",
		ServerPort:  7000,
		TLSEnable:   true,
		Description: "飞牛FN Connect 中国节点1",
		Priority:    1,
	},
	{
		ID:          "cn-tunnel-1",
		Name:        "FN Tunnel CN-1",
		Region:      RegionCN,
		ServerAddr:  "tunnel.fnos.cn",
		ServerPort:  7000,
		TLSEnable:   true,
		Description: "飞牛FN Connect 中国节点2",
		Priority:    2,
	},
	// 美国节点
	{
		ID:          "us-connect-1",
		Name:        "FN Connect US-1",
		Region:      RegionUS,
		ServerAddr:  "connect.fnos.us",
		ServerPort:  7000,
		TLSEnable:   true,
		Description: "飞牛FN Connect 美国节点",
		Priority:    10,
	},
	// 欧洲节点
	{
		ID:          "eu-connect-1",
		Name:        "FN Connect EU-1",
		Region:      RegionEU,
		ServerAddr:  "connect.fnos.eu",
		ServerPort:  7000,
		TLSEnable:   true,
		Description: "飞牛FN Connect 欧洲节点",
		Priority:    20,
	},
}

// NewFreeNodeConfig 创建免费节点配置
func NewFreeNodeConfig() *FreeNodeConfig {
	config := &FreeNodeConfig{
		nodes:     make(map[string]*FreeNode),
		regionMap: make(map[NodeRegion][]string),
	}
	
	// 加载默认节点
	for _, node := range defaultFreeNodes {
		config.nodes[node.ID] = node
		config.regionMap[node.Region] = append(config.regionMap[node.Region], node.ID)
	}
	
	return config
}

// GetNode 获取指定节点
func (c *FreeNodeConfig) GetNode(id string) *FreeNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nodes[id]
}

// GetAllNodes 获取所有节点
func (c *FreeNodeConfig) GetAllNodes() []*FreeNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	nodes := make([]*FreeNode, 0, len(c.nodes))
	for _, node := range c.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

// GetNodesByRegion 获取指定区域的节点
func (c *FreeNodeConfig) GetNodesByRegion(region NodeRegion) []*FreeNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	ids, ok := c.regionMap[region]
	if !ok {
		return nil
	}
	
	nodes := make([]*FreeNode, 0, len(ids))
	for _, id := range ids {
		if node, exists := c.nodes[id]; exists {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

// GetBestNode 获取最优节点（按优先级和在线状态）
func (c *FreeNodeConfig) GetBestNode(region ...NodeRegion) *FreeNode {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	var bestNode *FreeNode
	
	// 如果指定了区域，只在该区域选择
	if len(region) > 0 {
		ids, ok := c.regionMap[region[0]]
		if !ok {
			return nil
		}
		
		for _, id := range ids {
			node, exists := c.nodes[id]
			if !exists {
				continue
			}
			if bestNode == nil || node.Priority < bestNode.Priority {
				bestNode = node
			}
		}
		return bestNode
	}
	
	// 未指定区域，选择所有节点中优先级最高的
	for _, node := range c.nodes {
		if bestNode == nil || node.Priority < bestNode.Priority {
			bestNode = node
		}
	}
	
	return bestNode
}

// AddNode 添加自定义节点
func (c *FreeNodeConfig) AddNode(node *FreeNode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if node.ID == "" {
		node.ID = fmt.Sprintf("custom-%d", time.Now().UnixNano())
	}
	
	c.nodes[node.ID] = node
	c.regionMap[node.Region] = append(c.regionMap[node.Region], node.ID)
}

// RemoveNode 移除节点
func (c *FreeNodeConfig) RemoveNode(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	node, exists := c.nodes[id]
	if !exists {
		return
	}
	
	// 从regionMap中移除
	region := node.Region
	ids := c.regionMap[region]
	for i, rid := range ids {
		if rid == id {
			c.regionMap[region] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	
	// 从nodes中移除
	delete(c.nodes, id)
}

// UpdateNodeStatus 更新节点状态
func (c *FreeNodeConfig) UpdateNodeStatus(id string, online bool, latency int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if node, exists := c.nodes[id]; exists {
		node.Online = online
		node.Latency = latency
		node.LastCheck = time.Now()
	}
}

// NodeToClientConfig 将节点转换为客户端配置
func NodeToClientConfig(node *FreeNode) *ClientConfig {
	return &ClientConfig{
		Common: CommonConfig{
			ServerAddr:        node.ServerAddr,
			ServerPort:        node.ServerPort,
			TLSEnable:         node.TLSEnable,
			HeartbeatInterval: 30,
			HeartbeatTimeout:  90,
			LogLevel:          "info",
			PoolCount:         5,
			TCPMux:            true,
			TCPMuxKeepalive:   60,
			Protocol:          "tcp",
			LoginFailExit:     false,
		},
		Tunnels: []TunnelConfig{},
	}
}

// QuickConnectConfig 一键连接配置
type QuickConnectConfig struct {
	NodeID     string       `json:"node_id,omitempty"`     // 指定节点ID，不指定则自动选择最优
	Region     NodeRegion   `json:"region,omitempty"`      // 指定区域，不指定则自动选择
	LocalPort  int          `json:"local_port"`            // 本地端口
	RemotePort int          `json:"remote_port,omitempty"` // 远程端口，不指定则自动分配
	TunnelName string       `json:"tunnel_name,omitempty"` // 隧道名称
	TunnelType TunnelType   `json:"tunnel_type,omitempty"` // 隧道类型
}

// QuickConnectResult 一键连接结果
type QuickConnectResult struct {
	Success    bool         `json:"success"`
	Node       *FreeNode    `json:"node"`
	TunnelID   string       `json:"tunnel_id"`
	PublicURL  string       `json:"public_url,omitempty"`
	Error      string       `json:"error,omitempty"`
	ConnectAt  time.Time    `json:"connect_at"`
}