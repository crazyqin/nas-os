// Package mpiofc 实现多路径 I/O 光纤通道管理
// 对标 TrueNAS 26 的 MPIO for Fibre Channel 功能
// 提供 HBA 端口检测、多路径配置、路径状态监控和故障切换能力
package mpiofc

import (
	"fmt"
	"time"
)

// PathPolicy 多路径策略
type PathPolicy string

const (
	PathPolicyRoundRobin   PathPolicy = "round-robin"   // 轮询（负载均衡）
	PathPolicyFailover     PathPolicy = "failover"      // 故障转移（主备）
	PathPolicyMinQueue     PathPolicy = "min-queue"     // 最小队列深度
	PathPolicyRoundRobin16 PathPolicy = "round-robin-16" // 轮询（16字节块）
)

// PathState 路径状态
type PathState string

const (
	PathStateActive  PathState = "active"  // 活跃（正常使用中）
	PathStateStandby PathState = "standby" // 待机（故障转移备用）
	PathStateFailed  PathState = "failed"  // 故障（不可用）
	PathStateUnknown PathState = "unknown" // 未知
)

// HBAPort 光纤通道 HBA 端口
type HBAPort struct {
	ID         string    `json:"id"`         // 端口 ID
	Name       string    `json:"name"`       // 端口名称（如 fc_host0）
	WWPN       string    `json:"wwpn"`       // World Wide Port Name
	WWNN       string    `json:"wwnn"`       // World Wide Node Name
	PortName   string    `json:"portName"`   // 端口友好名称
	FabricName string    `json:"fabricName"` // Fabric Name
	Speed      string    `json:"speed"`      // 端口速率（如 16G、32G）
	PortType   string    `json:"portType"`   // 端口类型（N_Port, NL_Port 等）
	State      PathState `json:"state"`      // 端口状态
	Online     bool      `json:"online"`     // 是否在线
	Supported  bool      `json:"supported"`  // 是否支持多路径
	UpdatedAt  time.Time `json:"updatedAt"`  // 最后更新时间
}

// MPIOPath 多路径
type MPIOPath struct {
	ID            string    `json:"id"`            // 路径 ID
	HBAPortID     string    `json:"hbaPortId"`     // HBA 端口 ID
	TargetWWPN    string    `json:"targetWwpn"`    // 目标 WWPN
	PathState     PathState `json:"pathState"`     // 路径状态
	Policy        PathPolicy `json:"policy"`       // 路径策略
	Priority      int       `json:"priority"`      // 路径优先级（数值越小优先级越高）
	Active        bool      `json:"active"`        // 是否为当前活跃路径
	IOLoad        int       `json:"ioLoad"`        // I/O 负载（百分比）
	FailoverCount int       `json:"failoverCount"` // 故障切换次数
	LastFailoverAt *time.Time `json:"lastFailoverAt,omitempty"` // 最后故障切换时间
	CreatedAt     time.Time `json:"createdAt"`     // 创建时间
	UpdatedAt     time.Time `json:"updatedAt"`     // 更新时间
}

// MPIOConfig 多路径配置
type MPIOConfig struct {
	TargetWWPN string      `json:"targetWwpn" binding:"required"` // 目标 WWPN
	Policy     PathPolicy  `json:"policy"`                        // 多路径策略
	Paths      []PathConfig `json:"paths"`                        // 路径配置列表
}

// PathConfig 单条路径配置
type PathConfig struct {
	HBAPortID string `json:"hbaPortId" binding:"required"` // HBA 端口 ID
	Priority int    `json:"priority"`                      // 优先级
}

// PathStatistics 路径统计信息
type PathStatistics struct {
	PathID         string  `json:"pathId"`         // 路径 ID
	HBAPortID      string  `json:"hbaPortId"`      // HBA 端口 ID
	TargetWWPN     string  `json:"targetWwpn"`     // 目标 WWPN
	IOPSRead       int64   `json:"iopsRead"`       // 读 IOPS
	IOPSWrite      int64   `json:"iopsWrite"`      // 写 IOPS
	IOPSTotal      int64   `json:"iopsTotal"`      // 总 IOPS
	ThroughputRead float64 `json:"throughputRead"` // 读吞吐（MB/s）
	ThroughputWrite float64 `json:"throughputWrite"` // 写吞吐（MB/s）
	LatencyAvgMs   float64 `json:"latencyAvgMs"`   // 平均延迟（ms）
	LatencyMaxMs   float64 `json:"latencyMaxMs"`   // 最大延迟（ms）
	ErrorCount     int64   `json:"errorCount"`     // 错误计数
	FailoverCount  int64   `json:"failoverCount"`  // 故障切换次数
	CollectedAt    time.Time `json:"collectedAt"`  // 采集时间
}

// MPIOStatus 多路径状态总览
type MPIOStatus struct {
	TotalPorts    int          `json:"totalPorts"`    // HBA 端口总数
	OnlinePorts   int          `json:"onlinePorts"`   // 在线端口数
	TotalPaths    int          `json:"totalPaths"`    // 路径总数
	ActivePaths   int          `json:"activePaths"`   // 活跃路径数
	StandbyPaths  int          `json:"standbyPaths"`  // 待机路径数
	FailedPaths   int          `json:"failedPaths"`   // 故障路径数
	Paths         []*MPIOPath  `json:"paths"`         // 路径列表
	FailoverEvents int         `json:"failoverEvents"` // 故障切换事件总数
}

// APIResponse 统一 API 响应格式
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Config 模块配置
type Config struct {
	SysFCBase  string // /sys/class/fc_host 基路径
	DMMPBase   string // device-mapper-multipath 配置路径
	MultiPathBin string // multipath 命令路径
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		SysFCBase:   "/sys/class/fc_host",
		DMMPBase:    "/etc/multipath",
		MultiPathBin: "/sbin/multipath",
	}
}

// Validate 校验多路径策略
func (p PathPolicy) Validate() error {
	switch p {
	case PathPolicyRoundRobin, PathPolicyFailover, PathPolicyMinQueue, PathPolicyRoundRobin16:
		return nil
	case "":
		return fmt.Errorf("路径策略不能为空")
	default:
		return fmt.Errorf("不支持的路径策略: %s", p)
	}
}

// ValidateWWPN 校验 WWPN 格式（简化版：冒号分隔的十六进制）
func ValidateWWPN(wwpn string) error {
	if len(wwpn) < 16 {
		return fmt.Errorf("WWPN 格式无效: %s（长度不足）", wwpn)
	}
	return nil
}
