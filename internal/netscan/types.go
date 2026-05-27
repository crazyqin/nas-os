package netscan

import (
	"sync"
	"time"
)

// DeviceState 设备状态.
type DeviceState string

const (
	DeviceStateOnline  DeviceState = "online"
	DeviceStateOffline DeviceState = "offline"
	DeviceStateUnknown DeviceState = "unknown"
)

// PortState 端口状态.
type PortState string

const (
	PortStateOpen     PortState = "open"
	PortStateClosed   PortState = "closed"
	PortStateFiltered PortState = "filtered"
)

// Protocol 协议类型.
type Protocol string

const (
	ProtocolTCP Protocol = "tcp"
	ProtocolUDP Protocol = "udp"
)

// Device 网络设备.
type Device struct {
	IP          string            `json:"ip"`
	MAC         string            `json:"mac,omitempty"`
	Hostname    string            `json:"hostname,omitempty"`
	Vendor      string            `json:"vendor,omitempty"`
	State       DeviceState       `json:"state"`
	OpenPorts   []Port            `json:"openPorts,omitempty"`
	Services    []Service         `json:"services,omitempty"`
	OS          string            `json:"os,omitempty"`
	TTL         int               `json:"ttl,omitempty"`
	RTT         time.Duration     `json:"rtt,omitempty"`
	LastSeen    time.Time         `json:"lastSeen"`
	Properties  map[string]string `json:"properties,omitempty"`
}

// Port 端口信息.
type Port struct {
	Number   int       `json:"number"`
	Protocol Protocol  `json:"protocol"`
	State    PortState `json:"state"`
	Service  string    `json:"service,omitempty"`
	Banner   string    `json:"banner,omitempty"`
	Version  string    `json:"version,omitempty"`
}

// Service 服务信息.
type Service struct {
	Name    string `json:"name"`
	Port    int    `json:"port"`
	Proto   string `json:"proto"`
	Version string `json:"version,omitempty"`
	Banner  string `json:"banner,omitempty"`
}

// ScanTarget 扫描目标.
type ScanTarget struct {
	IP       string   `json:"ip"`
	Ports    []int    `json:"ports,omitempty"`
	Protocols []Protocol `json:"protocols,omitempty"`
}

// ScanResult 扫描结果.
type ScanResult struct {
	Target    ScanTarget  `json:"target"`
	Devices   []Device    `json:"devices"`
	StartTime time.Time   `json:"startTime"`
	EndTime   time.Time   `json:"endTime"`
	Duration  time.Duration `json:"duration"`
	Error     string      `json:"error,omitempty"`
}

// DiscoveryConfig 设备发现配置.
type DiscoveryConfig struct {
	// Network CIDR 网段 (如 "192.168.1.0/24")
	Network string `json:"network"`
	// Methods 发现方式: arp, ping, both
	Methods []string `json:"methods"`
	// Timeout 超时时间
	Timeout time.Duration `json:"timeout"`
	// Concurrent 并发数
	Concurrent int `json:"concurrent"`
	// UseARP 是否使用 ARP 扫描
	UseARP bool `json:"useArp"`
	// UseICMP 是否使用 ICMP ping
	UseICMP bool `json:"useIcmp"`
}

// PortScanConfig 端口扫描配置.
type PortScanConfig struct {
	// Target 扫描目标 IP
	Target string `json:"target"`
	// Ports 端口列表 (为空则扫描常用端口)
	Ports []int `json:"ports,omitempty"`
	// Protocol 协议: tcp, udp, both
	Protocol Protocol `json:"protocol"`
	// Timeout 连接超时
	Timeout time.Duration `json:"timeout"`
	// Concurrent 并发数
	Concurrent int `json:"concurrent"`
	// TopPorts 扫描 Top N 常用端口 (0=自定义端口)
	TopPorts int `json:"topPorts"`
}

// ServiceDetectConfig 服务识别配置.
type ServiceDetectConfig struct {
	// Target 目标 IP
	Target string `json:"target"`
	// Ports 要识别的端口列表
	Ports []int `json:"ports"`
	// Timeout 超时时间
	Timeout time.Duration `json:"timeout"`
	// BannerGrab 是否抓取 Banner
	BannerGrab bool `json:"bannerGrab"`
}

// TopologyNode 拓扑节点.
type TopologyNode struct {
	ID       string            `json:"id"`
	IP       string            `json:"ip"`
	MAC      string            `json:"mac,omitempty"`
	Hostname string            `json:"hostname,omitempty"`
	Type     string            `json:"type"` // host, router, switch, nas
	Services []string          `json:"services,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// TopologyEdge 拓扑边.
type TopologyEdge struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Weight   int    `json:"weight"`
	Label    string `json:"label,omitempty"`
	Protocol string `json:"protocol,omitempty"`
}

// Topology 拓扑图.
type Topology struct {
	Nodes     []TopologyNode `json:"nodes"`
	Edges     []TopologyEdge `json:"edges"`
	UpdatedAt time.Time      `json:"updatedAt"`
}

// ScanTask 扫描任务.
type ScanTask struct {
	ID         string        `json:"id"`
	Type       string        `json:"type"` // discovery, portscan, servicedetect, topology
	Target     string        `json:"target"`
	Status     string        `json:"status"` // pending, running, completed, failed, cancelled
	Progress   float64       `json:"progress"`
	Result     interface{}   `json:"result,omitempty"`
	Error      string        `json:"error,omitempty"`
	StartTime  time.Time     `json:"startTime"`
	EndTime    time.Time     `json:"endTime"`
	Duration   time.Duration `json:"duration"`
	Config     interface{}   `json:"config"`
}

// TaskManager 扫描任务管理器.
type TaskManager struct {
	mu       sync.RWMutex
	tasks    map[string]*ScanTask
	workerCh chan struct{}
	maxWorkers int
}

// NewTaskManager 创建任务管理器.
func NewTaskManager(maxWorkers int) *TaskManager {
	if maxWorkers <= 0 {
		maxWorkers = 10
	}
	return &TaskManager{
		tasks:      make(map[string]*ScanTask),
		workerCh:   make(chan struct{}, maxWorkers),
		maxWorkers: maxWorkers,
	}
}

// CommonPorts 常用端口列表.
var CommonPorts = []int{
	20, 21, 22, 23, 25, 53, 80, 110, 143, 443,
	445, 993, 995, 1723, 3306, 3389, 5432, 5900,
	8080, 8443, 9090, 27017, 6379, 11211, 5000, 8000,
}

// TopPorts100 Top 100 常用端口.
var TopPorts100 = []int{
	7, 9, 13, 21, 22, 23, 25, 26, 37, 53, 79, 80, 81, 88, 106, 110, 111,
	113, 119, 135, 139, 143, 144, 179, 199, 389, 427, 443, 444, 445, 465,
	513, 514, 515, 543, 544, 548, 554, 587, 631, 646, 873, 990, 993, 995,
	1025, 1026, 1027, 1028, 1029, 1110, 1433, 1720, 1723, 1755, 1900, 2000,
	2001, 2049, 2100, 2103, 2121, 2199, 2717, 2869, 2967, 3000, 3001, 3128,
	3268, 3306, 3389, 3986, 4000, 4001, 4443, 4444, 4899, 5000, 5001, 5003,
	5009, 5050, 5051, 5060, 5101, 5120, 5190, 5357, 5432, 5555, 5631, 5666,
	5800, 5900, 6000, 6001, 6379, 6646, 7000, 7070, 7100, 7443, 7938, 8000,
	8008, 8009, 8080, 8081, 8082, 8083, 8084, 8085, 8088, 8090, 8443, 8888,
	9000, 9001, 9090, 9099, 9100, 9200, 9443, 9999, 10000, 27017, 28017,
	50000, 50070,
}
