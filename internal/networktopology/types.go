package networktopology

import (
	"sync"
	"time"
)

// ========== 设备类型与状态 ==========

// DeviceType 设备类型.
type DeviceType string

const (
	DeviceTypeRouter   DeviceType = "router"   // 路由器
	DeviceTypeSwitch   DeviceType = "switch"    // 交换机
	DeviceTypeNAS      DeviceType = "nas"       // NAS 存储
	DeviceTypeServer   DeviceType = "server"    // 服务器
	DeviceTypeCamera   DeviceType = "camera"    // 摄像头
	DeviceTypeIoT      DeviceType = "iot"       // IoT 设备
	DeviceTypePrinter  DeviceType = "printer"   // 打印机
	DeviceTypeFirewall DeviceType = "firewall"  // 防火墙
	DeviceTypeAP       DeviceType = "ap"        // 无线接入点
	DeviceTypeHost     DeviceType = "host"      // 普通主机
	DeviceTypeUnknown  DeviceType = "unknown"   // 未知设备
)

// DeviceState 设备在线状态.
type DeviceState string

const (
	DeviceStateOnline  DeviceState = "online"   // 在线
	DeviceStateOffline DeviceState = "offline"  // 离线
	DeviceStateUnknown DeviceState = "unknown"  // 未知
)

// RiskLevel 安全风险等级.
type RiskLevel string

const (
	RiskLevelLow      RiskLevel = "low"      // 低风险
	RiskLevelMedium   RiskLevel = "medium"   // 中风险
	RiskLevelHigh     RiskLevel = "high"     // 高风险
	RiskLevelCritical RiskLevel = "critical" // 严重风险
)

// ScanMethod 扫描发现方式.
type ScanMethod string

const (
	ScanMethodARP   ScanMethod = "arp"   // ARP 扫描
	ScanMethodICMP  ScanMethod = "icmp"  // ICMP Ping
	ScanMethodMDNS  ScanMethod = "mdns"  // mDNS 发现
	ScanMethodSNMP  ScanMethod = "snmp"  // SNMP 查询
	ScanMethodAll   ScanMethod = "all"   // 全部方式
)

// ========== 核心数据结构 ==========

// TopologyDevice 网络拓扑设备.
type TopologyDevice struct {
	ID          string            `json:"id"`                    // 设备唯一标识（MAC 或 IP）
	IP          string            `json:"ip"`                    // IP 地址
	MAC         string            `json:"mac,omitempty"`         // MAC 地址
	Hostname    string            `json:"hostname,omitempty"`    // 主机名
	Vendor      string            `json:"vendor,omitempty"`      // 设备厂商
	DeviceType  DeviceType        `json:"deviceType"`            // 设备类型
	State       DeviceState       `json:"state"`                 // 在线状态
	OS          string            `json:"os,omitempty"`          // 操作系统
	Firmware    string            `json:"firmware,omitempty"`    // 固件版本
	Uptime      time.Duration     `json:"uptime,omitempty"`      // 运行时长
	RTT         time.Duration     `json:"rtt,omitempty"`         // 网络延迟
	OpenPorts   []PortInfo        `json:"openPorts,omitempty"`   // 开放端口
	Services    []ServiceInfo     `json:"services,omitempty"`    // 运行服务
	Interfaces  []InterfaceInfo   `json:"interfaces,omitempty"`  // 网络接口
	VLAN        string            `json:"vlan,omitempty"`        // 所属 VLAN
	Subnet      string            `json:"subnet,omitempty"`      // 所属子网
	Tags        []string          `json:"tags,omitempty"`        // 标签
	Properties  map[string]string `json:"properties,omitempty"`  // 扩展属性
	FirstSeen   time.Time         `json:"firstSeen"`             // 首次发现时间
	LastSeen    time.Time         `json:"lastSeen"`              // 最后发现时间
}

// PortInfo 端口信息.
type PortInfo struct {
	Number   int    `json:"number"`             // 端口号
	Protocol string `json:"protocol"`           // 协议 (tcp/udp)
	State    string `json:"state"`              // 状态 (open/closed/filtered)
	Service  string `json:"service,omitempty"`  // 服务名
	Banner   string `json:"banner,omitempty"`   // Banner 信息
	Version  string `json:"version,omitempty"`  // 服务版本
}

// ServiceInfo 服务信息.
type ServiceInfo struct {
	Name    string `json:"name"`              // 服务名称
	Port    int    `json:"port"`              // 监听端口
	Proto   string `json:"proto"`             // 协议
	Version string `json:"version,omitempty"` // 版本
	Banner  string `json:"banner,omitempty"`  // Banner
}

// InterfaceInfo 网络接口信息.
type InterfaceInfo struct {
	Name      string `json:"name"`            // 接口名称
	MAC       string `json:"mac,omitempty"`   // MAC 地址
	IP        string `json:"ip,omitempty"`    // IP 地址
	Speed     string `json:"speed,omitempty"` // 链路速率
	Duplex    string `json:"duplex,omitempty"` // 双工模式
	AdminUp   bool   `json:"adminUp"`         // 管理状态
	OperState bool   `json:"operState"`       // 操作状态
}

// ========== 网络拓扑 ==========

// TopologyNode 拓扑节点.
type TopologyNode struct {
	ID         string            `json:"id"`                   // 节点 ID
	IP         string            `json:"ip"`                   // IP 地址
	MAC        string            `json:"mac,omitempty"`        // MAC 地址
	Hostname   string            `json:"hostname,omitempty"`   // 主机名
	DeviceType DeviceType        `json:"deviceType"`           // 设备类型
	State      DeviceState       `json:"state"`                // 在线状态
	Services   []string          `json:"services,omitempty"`   // 服务列表
	Vendor     string            `json:"vendor,omitempty"`     // 厂商
	Subnet     string            `json:"subnet,omitempty"`     // 子网
	VLAN       string            `json:"vlan,omitempty"`       // VLAN
	Metadata   map[string]string `json:"metadata,omitempty"`   // 元数据
}

// TopologyEdge 拓扑边.
type TopologyEdge struct {
	Source   string `json:"source"`             // 源节点 ID
	Target   string `json:"target"`             // 目标节点 ID
	Weight   int    `json:"weight"`             // 权重
	Label    string `json:"label,omitempty"`    // 标签
	Protocol string `json:"protocol,omitempty"` // 协议
	LinkType string `json:"linkType,omitempty"` // 链路类型 (wired/wireless/vpn)
	Speed    string `json:"speed,omitempty"`    // 链路速率
}

// NetworkTopology 网络拓扑图.
type NetworkTopology struct {
	Nodes     []TopologyNode `json:"nodes"`              // 节点列表
	Edges     []TopologyEdge `json:"edges"`              // 边列表
	Subnets   []SubnetInfo   `json:"subnets,omitempty"`  // 子网列表
	VLANs     []VLANInfo     `json:"vlans,omitempty"`    // VLAN 列表
	UpdatedAt time.Time      `json:"updatedAt"`          // 更新时间
}

// ========== 子网与 VLAN ==========

// SubnetInfo 子网信息.
type SubnetInfo struct {
	CIDR        string   `json:"cidr"`                  // 子网 CIDR (如 192.168.1.0/24)
	Name        string   `json:"name,omitempty"`        // 子网名称
	Description string   `json:"description,omitempty"` // 描述
	Gateway     string   `json:"gateway,omitempty"`     // 网关地址
	DNS         []string `json:"dns,omitempty"`         // DNS 服务器
	VLAN        string   `json:"vlan,omitempty"`        // 关联 VLAN
	DeviceCount int      `json:"deviceCount"`           // 设备数量
	OnlineCount int      `json:"onlineCount"`           // 在线设备数
}

// VLANInfo VLAN 信息.
type VLANInfo struct {
	ID          int    `json:"id"`                     // VLAN ID
	Name        string `json:"name"`                   // VLAN 名称
	Description string `json:"description,omitempty"`  // 描述
	Subnet      string `json:"subnet,omitempty"`       // 关联子网
	Ports       string `json:"ports,omitempty"`        // 端口成员
	DeviceCount int    `json:"deviceCount"`            // 设备数量
}

// ========== 网络性能监控 ==========

// PerformanceMetrics 网络性能指标.
type PerformanceMetrics struct {
	DeviceID    string        `json:"deviceId"`              // 设备 ID
	IP          string        `json:"ip"`                    // IP 地址
	Latency     time.Duration `json:"latency"`               // 延迟
	Jitter      time.Duration `json:"jitter,omitempty"`      // 抖动
	PacketLoss  float64       `json:"packetLoss"`            // 丢包率 (%)
	BandwidthIn int64         `json:"bandwidthIn"`           // 入向带宽 (bytes/s)
	BandwidthOut int64        `json:"bandwidthOut"`          // 出向带宽 (bytes/s)
	Timestamp   time.Time     `json:"timestamp"`             // 采集时间
}

// PerformanceHistory 性能历史记录.
type PerformanceHistory struct {
	DeviceID string               `json:"deviceId"` // 设备 ID
	IP       string               `json:"ip"`       // IP 地址
	Metrics  []PerformanceMetrics `json:"metrics"`   // 历史指标
}

// ========== 安全风险评估 ==========

// SecurityRisk 安全风险.
type SecurityRisk struct {
	ID          string    `json:"id"`                    // 风险 ID
	DeviceID    string    `json:"deviceId"`              // 设备 ID
	DeviceIP    string    `json:"deviceIp"`              // 设备 IP
	Level       RiskLevel `json:"level"`                 // 风险等级
	Category    string    `json:"category"`              // 风险类别
	Title       string    `json:"title"`                 // 风险标题
	Description string    `json:"description"`           // 风险描述
	Suggestion  string    `json:"suggestion,omitempty"`  // 修复建议
	Ports       []int     `json:"ports,omitempty"`       // 相关端口
	DetectedAt  time.Time `json:"detectedAt"`            // 检测时间
	Resolved    bool      `json:"resolved"`              // 是否已解决
	ResolvedAt  time.Time `json:"resolvedAt,omitempty"`  // 解决时间
}

// RiskReport 风险评估报告.
type RiskReport struct {
	Summary       RiskSummary     `json:"summary"`            // 风险摘要
	Risks         []SecurityRisk  `json:"risks"`              // 风险列表
	DeviceCount   int             `json:"deviceCount"`        // 设备总数
	ScannedAt     time.Time       `json:"scannedAt"`          // 扫描时间
}

// RiskSummary 风险摘要.
type RiskSummary struct {
	Total    int `json:"total"`    // 风险总数
	Critical int `json:"critical"` // 严重风险数
	High     int `json:"high"`     // 高风险数
	Medium   int `json:"medium"`   // 中风险数
	Low      int `json:"low"`      // 低风险数
	Score    int `json:"score"`    // 安全评分 (0-100)
}

// ========== 扫描与任务 ==========

// ScanConfig 网络扫描配置.
type ScanConfig struct {
	Network    string        `json:"network"`              // 网络 CIDR
	Methods    []ScanMethod  `json:"methods"`              // 发现方式
	Timeout    time.Duration `json:"timeout"`              // 超时时间
	Concurrent int           `json:"concurrent"`           // 并发数
	PortsTop   int           `json:"portsTop,omitempty"`   // 扫描 Top N 端口
	Ports      []int         `json:"ports,omitempty"`      // 自定义端口列表
	DeepScan   bool          `json:"deepScan,omitempty"`   // 深度扫描（含服务识别）
	SNMPComm   string        `json:"snmpComm,omitempty"`   // SNMP Community 字符串
}

// ScanTask 扫描任务.
type ScanTask struct {
	ID         string        `json:"id"`                   // 任务 ID
	Type       string        `json:"type"`                 // 任务类型 (scan/topology/riskcheck)
	Status     string        `json:"status"`               // 任务状态 (pending/running/completed/failed/cancelled)
	Progress   float64       `json:"progress"`             // 进度百分比 (0-100)
	Network    string        `json:"network"`              // 目标网段
	Config     ScanConfig    `json:"config"`               // 扫描配置
	Result     interface{}   `json:"result,omitempty"`     // 扫描结果
	Error      string        `json:"error,omitempty"`      // 错误信息
	StartTime  time.Time     `json:"startTime"`            // 开始时间
	EndTime    time.Time     `json:"endTime"`              // 结束时间
	Duration   time.Duration `json:"duration"`             // 耗时
}

// ========== 监控追踪 ==========

// MonitorTarget 监控目标.
type MonitorTarget struct {
	ID       string        `json:"id"`                // 目标 ID
	IP       string        `json:"ip"`                // IP 地址
	Name     string        `json:"name,omitempty"`    // 名称
	Interval time.Duration `json:"interval"`          // 监控间隔
	Enabled  bool          `json:"enabled"`           // 是否启用
}

// DeviceEvent 设备事件.
type DeviceEvent struct {
	ID        string      `json:"id"`          // 事件 ID
	DeviceID  string      `json:"deviceId"`    // 设备 ID
	DeviceIP  string      `json:"deviceIp"`    // 设备 IP
	EventType string      `json:"eventType"`   // 事件类型 (online/offline/port_change/risk)
	Message   string      `json:"message"`     // 事件消息
	Timestamp time.Time   `json:"timestamp"`   // 发生时间
}

// ========== 设备指纹规则 ==========

// FingerprintRule 设备指纹识别规则.
type FingerprintRule struct {
	Name       string            `json:"name"`                 // 规则名称
	DeviceType DeviceType        `json:"deviceType"`           // 设备类型
	Ports      []int             `json:"ports,omitempty"`      // 特征端口
	Services   []string          `json:"services,omitempty"`   // 特征服务
	BannerKeys []string          `json:"bannerKeys,omitempty"` // Banner 关键词
	MACPrefix  []string          `json:"macPrefix,omitempty"`  // MAC 前缀 (OUI)
	OSPattern  string            `json:"osPattern,omitempty"`  // OS 模式
	Priority   int               `json:"priority"`             // 优先级（越高越优先）
}

// ========== 服务层配置与状态 ==========

// TopologyConfig 拓扑服务全局配置.
type TopologyConfig struct {
	AutoScanEnabled  bool          `json:"autoScanEnabled"`            // 自动扫描开关
	AutoScanInterval time.Duration `json:"autoScanInterval"`           // 自动扫描间隔
	MonitorEnabled   bool          `json:"monitorEnabled"`             // 设备监控开关
	MonitorInterval  time.Duration `json:"monitorInterval"`            // 设备监控间隔
	RiskCheckEnabled bool          `json:"riskCheckEnabled"`           // 风险检查开关
	DefaultNetworks  []string      `json:"defaultNetworks,omitempty"`  // 默认扫描网段
	RetainHistory    time.Duration `json:"retainHistory"`              // 历史数据保留时长
}

// TopologyService 网络拓扑服务.
type TopologyService struct {
	mu             sync.RWMutex
	devices        map[string]*TopologyDevice  // 设备表 (key=ID)
	topology       *NetworkTopology            // 当前拓扑图
	risks          []SecurityRisk              // 风险列表
	perfHistory    map[string]*PerformanceHistory // 性能历史 (key=deviceID)
	events         []DeviceEvent               // 事件记录
	monitors       map[string]*MonitorTarget   // 监控目标 (key=ID)
	tasks          map[string]*ScanTask        // 任务表 (key=ID)
	config         TopologyConfig              // 服务配置
	fingerprints   []FingerprintRule           // 指纹规则
	maxEvents      int                         // 最大事件数
	maxHistory     int                         // 最大历史指标数
}

// ========== 请求/响应结构 ==========

// ScanRequest 扫描请求.
type ScanRequest struct {
	Network    string   `json:"network" binding:"required"` // 网络 CIDR
	Methods    []string `json:"methods"`                    // 发现方式
	DeepScan   bool     `json:"deepScan"`                   // 深度扫描
	PortsTop   int      `json:"portsTop"`                   // 扫描 Top N 端口
}

// AddMonitorRequest 添加监控请求.
type AddMonitorRequest struct {
	IP       string `json:"ip" binding:"required"`       // IP 地址
	Name     string `json:"name"`                        // 名称
	Interval int    `json:"interval"`                    // 监控间隔（秒）
}

// AddSubnetRequest 添加子网请求.
type AddSubnetRequest struct {
	CIDR        string `json:"cidr" binding:"required"`   // 子网 CIDR
	Name        string `json:"name"`                      // 名称
	Description string `json:"description"`               // 描述
	Gateway     string `json:"gateway"`                   // 网关
}

// AddVLANRequest 添加 VLAN 请求.
type AddVLANRequest struct {
	ID          int    `json:"id" binding:"required"`     // VLAN ID
	Name        string `json:"name" binding:"required"`   // 名称
	Description string `json:"description"`               // 描述
	Subnet      string `json:"subnet"`                    // 子网
}

// TopologyOverview 拓扑概览.
type TopologyOverview struct {
	TotalDevices   int               `json:"totalDevices"`     // 设备总数
	OnlineDevices  int               `json:"onlineDevices"`    // 在线设备数
	OfflineDevices int               `json:"offlineDevices"`   // 离线设备数
	SubnetCount    int               `json:"subnetCount"`      // 子网数量
	VLANCount      int               `json:"vlanCount"`        // VLAN 数量
	RiskCount      int               `json:"riskCount"`        // 风险数量
	SecurityScore  int               `json:"securityScore"`    // 安全评分
	DeviceTypes    map[DeviceType]int `json:"deviceTypes"`     // 设备类型统计
	LastScanTime   time.Time         `json:"lastScanTime"`     // 最后扫描时间
}

// PerfQuery 性能查询参数.
type PerfQuery struct {
	DeviceID string `json:"deviceId" form:"deviceId"` // 设备 ID
	IP       string `json:"ip" form:"ip"`             // IP 地址
	Minutes  int    `json:"minutes" form:"minutes"`   // 查询时间范围（分钟）
}
