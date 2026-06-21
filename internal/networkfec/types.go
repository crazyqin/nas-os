package networkfec

import (
	"sync"
	"time"
)

// FECMode 前向纠错模式
type FECMode string

const (
	FECModeNone    FECMode = "none"
	FECModeReedSolomon FECMode = "reed_solomon"
	FECModeXOR     FECMode = "xor"
	FECModeConvolutional FECMode = "convolutional"
)

// FECConfig 前向纠错配置
type FECConfig struct {
	Mode          FECMode `json:"mode"`          // FEC 模式
	DataShards    int     `json:"dataShards"`    // 数据分片数
	ParityShards  int     `json:"parityShards"`  // 校验分片数
	MaxPacketSize int     `json:"maxPacketSize"` // 最大包大小
	Enabled       bool    `json:"enabled"`       // 是否启用
}

// DefaultFECConfig 默认配置
func DefaultFECConfig() *FECConfig {
	return &FECConfig{
		Mode:          FECModeReedSolomon,
		DataShards:    10,
		ParityShards:  3,
		MaxPacketSize: 1500,
		Enabled:       true,
	}
}

// FECStats FEC 统计信息
type FECStats struct {
	mu              sync.RWMutex
	TotalPackets    int64     `json:"totalPackets"`    // 总包数
	RecoveredPackets int64   `json:"recoveredPackets"` // 恢复的包数
	LostPackets     int64     `json:"lostPackets"`     // 丢失的包数
	RecoveryRate    float64   `json:"recoveryRate"`    // 恢复率
	Overhead        float64   `json:"overhead"`        // 开销百分比
	LastUpdated     time.Time `json:"lastUpdated"`
}

// FECInterface FEC 网络接口
type FECInterface struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`        // 接口名称
	IPAddress   string    `json:"ipAddress"`   // IP 地址
	FECMode     FECMode   `json:"fecMode"`     // FEC 模式
	DataShards  int       `json:"dataShards"`
	ParityShards int      `json:"parityShards"`
	Enabled     bool      `json:"enabled"`
	Status      string    `json:"status"`      // up/down
	Stats       *FECStats `json:"stats"`
}

// FECManager FEC 管理器
type FECManager struct {
	mu         sync.RWMutex
	config     *FECConfig
	interfaces map[string]*FECInterface
	stats      *FECStats
	running    bool
	stopCh     chan struct{}
}

// FECEncoder FEC 编码器
type FECEncoder struct {
	dataShards   int
	parityShards int
	mode         FECMode
}

// FECDecoder FEC 解码器
type FECDecoder struct {
	dataShards   int
	parityShards int
	mode         FECMode
}

// FECPacket FEC 数据包
type FECPacket struct {
	SequenceNum uint32 `json:"sequenceNum"` // 序列号
	ShardIndex  int    `json:"shardIndex"`  // 分片索引
	IsParity    bool   `json:"isParity"`    // 是否为校验分片
	Data        []byte `json:"data"`        // 数据
	Checksum    uint32 `json:"checksum"`    // 校验和
}
