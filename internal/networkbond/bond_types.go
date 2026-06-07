// Package networkbond 提供网卡绑定/链路聚合功能
// 本文件定义 QoS、VLAN、带宽聚合等扩展类型
package networkbond

import (
	"fmt"
	"time"
)

// QoSAlgorithm QoS调度算法
type QoSAlgorithm int

const (
	QoSAlgorithmFIFO      QoSAlgorithm = 0 // 先进先出
	QoSAlgorithmPriority  QoSAlgorithm = 1 // 优先级调度
	QoSAlgorithmFairQueue QoSAlgorithm = 2 // 公平队列
	QoSAlgorithmHTB       QoSAlgorithm = 3 // 层级令牌桶
)

// QoSPriority QoS优先级
type QoSPriority int

const (
	QoSPriorityLow      QoSPriority = 0  // 低优先级
	QoSPriorityNormal   QoSPriority = 50 // 普通优先级
	QoSPriorityHigh     QoSPriority = 80 // 高优先级
	QoSPriorityCritical QoSPriority = 99 // 关键优先级
)

// QoSRule QoS规则
type QoSRule struct {
	Name         string      `json:"name"`         // 规则名称
	Protocol     string      `json:"protocol"`     // 协议 (tcp/udp/any)
	SrcIP        string      `json:"srcIP"`        // 源IP (CIDR)
	DstIP        string      `json:"dstIP"`        // 目标IP (CIDR)
	SrcPort      int         `json:"srcPort"`      // 源端口 (0=any)
	DstPort      int         `json:"dstPort"`      // 目标端口 (0=any)
	Priority     QoSPriority `json:"priority"`     // 优先级
	MinBandwidth int         `json:"minBandwidth"` // 最小带宽(Mbps)
	MaxBandwidth int         `json:"maxBandwidth"` // 最大带宽(Mbps)
	Burst        int         `json:"burst"`        // 突发大小(KB)
	Enabled      bool        `json:"enabled"`      // 是否启用
}

// VLANMode VLAN模式
type VLANMode int

const (
	VLANModeTagged   VLANMode = 0 // Tagged模式(802.1Q)
	VLANModeUntagged VLANMode = 1 // Untagged模式
	VLANModeAccess   VLANMode = 2 // Access模式
)

// VLANConfig VLAN配置
type VLANConfig struct {
	ID          int      `json:"id"`          // VLAN ID (1-4094)
	Name        string   `json:"name"`        // VLAN名称
	Mode        VLANMode `json:"mode"`        // VLAN模式
	Description string   `json:"description"` // 描述
	Tagged      bool     `json:"tagged"`      // 是否打标签
	MTU         int      `json:"mtu"`         // VLAN MTU
}

// Validate 校验VLAN配置
func (v *VLANConfig) Validate() error {
	if v.ID < 1 || v.ID > 4094 {
		return fmt.Errorf("VLAN ID must be between 1 and 4094, got %d", v.ID)
	}
	if v.Name == "" {
		return fmt.Errorf("VLAN name cannot be empty")
	}
	if v.MTU < 0 {
		return fmt.Errorf("VLAN MTU cannot be negative")
	}
	return nil
}

// BandwidthConfig 带宽聚合配置
type BandwidthConfig struct {
	AggregateMode AggregateMode `json:"aggregateMode"` // 聚合模式
	HashPolicy    HashPolicy    `json:"hashPolicy"`    // 哈希策略
	Resilience    bool          `json:"resilience"`    // 带宽弹性(降级不中断)
}

// AggregateMode 聚合模式
type AggregateMode int

const (
	AggregateBandwidth  AggregateMode = 0 // 带宽聚合
	AggregateRedundancy AggregateMode = 1 // 冗余模式
)

// HashPolicy 哈希策略
type HashPolicy int

const (
	HashLayer2  HashPolicy = 0 // 二层(MAC)
	HashLayer34 HashPolicy = 1 // 三层四层(IP+Port)
	HashLayer23 HashPolicy = 2 // 二层三层(MAC+IP)
	HashEncap23 HashPolicy = 3 // 封装二层三层
)

// LinkHealthState 链路健康状态
type LinkHealthState struct {
	InterfaceName string    `json:"interfaceName"` // 接口名称
	Latency       int64     `json:"latency"`       // 延迟(ms)
	PacketLoss    float64   `json:"packetLoss"`    // 丢包率(%)
	LastCheck     time.Time `json:"lastCheck"`     // 最后检查时间
	Healthy       bool      `json:"healthy"`       // 是否健康
}

// FailoverPolicy 故障切换策略
type FailoverPolicy struct {
	Enabled          bool          `json:"enabled"`          // 是否启用
	Interval         time.Duration `json:"interval"`         // 检测间隔
	FailThreshold    int           `json:"failThreshold"`    // 连续失败阈值
	RecoverThreshold int           `json:"recoverThreshold"` // 恢复阈值
	GracePeriod      time.Duration `json:"gracePeriod"`      // 切换后宽限期
}

// BondExtended 绑定扩展配置（QoS和VLAN）
type BondExtended struct {
	BondName  string           `json:"bondName"`  // 绑定名称
	QoSRules  []*QoSRule       `json:"qosRules"`  // QoS规则列表
	VLANs     []*VLANConfig    `json:"vlans"`     // VLAN列表
	Bandwidth *BandwidthConfig `json:"bandwidth"` // 带宽配置
	Failover  *FailoverPolicy  `json:"failover"`  // 故障切换策略
}

// Validate 校验QoS规则
func (q *QoSRule) Validate() error {
	if q.Name == "" {
		return fmt.Errorf("QoS rule name cannot be empty")
	}
	if q.MinBandwidth < 0 {
		return fmt.Errorf("min bandwidth cannot be negative")
	}
	if q.MaxBandwidth < 0 {
		return fmt.Errorf("max bandwidth cannot be negative")
	}
	if q.MaxBandwidth > 0 && q.MinBandwidth > q.MaxBandwidth {
		return fmt.Errorf("min bandwidth (%d) cannot exceed max bandwidth (%d)", q.MinBandwidth, q.MaxBandwidth)
	}
	if q.Priority < 0 || q.Priority > 99 {
		return fmt.Errorf("priority must be between 0 and 99, got %d", q.Priority)
	}
	if q.SrcPort < 0 || q.SrcPort > 65535 {
		return fmt.Errorf("source port must be between 0 and 65535, got %d", q.SrcPort)
	}
	if q.DstPort < 0 || q.DstPort > 65535 {
		return fmt.Errorf("destination port must be between 0 and 65535, got %d", q.DstPort)
	}
	return nil
}

// GetAlgorithmName 获取QoS算法名称
func GetAlgorithmName(algo QoSAlgorithm) string {
	switch algo {
	case QoSAlgorithmFIFO:
		return "fifo"
	case QoSAlgorithmPriority:
		return "priority"
	case QoSAlgorithmFairQueue:
		return "fair-queue"
	case QoSAlgorithmHTB:
		return "htb"
	default:
		return "unknown"
	}
}

// GetVLANModeName 获取VLAN模式名称
func GetVLANModeName(mode VLANMode) string {
	switch mode {
	case VLANModeTagged:
		return "tagged"
	case VLANModeUntagged:
		return "untagged"
	case VLANModeAccess:
		return "access"
	default:
		return "unknown"
	}
}

// GetHashPolicyName 获取哈希策略名称
func GetHashPolicyName(policy HashPolicy) string {
	switch policy {
	case HashLayer2:
		return "layer2"
	case HashLayer34:
		return "layer3+4"
	case HashLayer23:
		return "layer2+3"
	case HashEncap23:
		return "encap2+3"
	default:
		return "unknown"
	}
}
