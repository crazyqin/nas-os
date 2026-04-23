// Package ha 集群配置管理
// 实现主备节点配置管理，参考群晖 Synology High Availability 的配置机制
package ha

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// 集群配置相关错误
var (
	ErrClusterAlreadyExists  = errors.New("cluster already exists")
	ErrClusterNotFound       = errors.New("cluster not found")
	ErrNodeAlreadyJoined     = errors.New("node already joined cluster")
	ErrNodeNotInCluster      = errors.New("node not in cluster")
	ErrInvalidClusterConfig  = errors.New("invalid cluster configuration")
	ErrClusterNotActive      = errors.New("cluster not active")
	ErrPrimaryRequired       = errors.New("primary node required")
	ErrSecondaryRequired     = errors.New("secondary node required")
	ErrClusterStateConflict  = errors.New("cluster state conflict")
	ErrNodeCapacityExceeded  = errors.New("node capacity exceeded")
)

// ClusterConfig 集群配置
// 参考 Synology HA 的双节点架构设计
type ClusterConfig struct {
	// 集群基本信息
	ClusterID   string    `json:"cluster_id"`
	ClusterName string    `json:"cluster_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Version     int       `json:"version"` // 配置版本号

	// 集群类型
	ClusterType ClusterType `json:"cluster_type"`

	// 节点配置
	PrimaryNode   *NodeConfig `json:"primary_node"`
	SecondaryNode *NodeConfig `json:"secondary_node"`

	// 网络配置
	ClusterNetwork *ClusterNetworkConfig `json:"cluster_network"`

	// 存储配置
	StorageConfig *ClusterStorageConfig `json:"storage_config"`

	// 心跳配置
	HeartbeatConfig *ClusterHeartbeatConfig `json:"heartbeat_config"`

	// 故障转移配置
	FailoverConfig *ClusterFailoverConfig `json:"failover_config"`

	// 同步配置
	SyncConfig *ClusterSyncConfig `json:"sync_config"`

	// 状态
	State       ClusterState `json:"state"`
	ActiveSince time.Time    `json:"active_since,omitempty"`

	// 元数据
	Metadata map[string]string `json:"metadata,omitempty"`
}

// ClusterType 集群类型
type ClusterType string

const (
	// ActivePassive 主备模式（群晖 HA 标准）
	ClusterTypeActivePassive ClusterType = "active-passive"
	// ActiveActive 双活模式
	ClusterTypeActiveActive ClusterType = "active-active"
	// MultiNode 多节点模式（扩展）
	ClusterTypeMultiNode ClusterType = "multi-node"
)

// ClusterState 集群状态
type ClusterState string

const (
	ClusterStateCreating    ClusterState = "creating"    // 创建中
	ClusterStateInitializing ClusterState = "initializing" // 初始化中
	ClusterStateActive      ClusterState = "active"      // 正常运行
	ClusterStateDegraded    ClusterState = "degraded"    // 降级运行（单节点）
	ClusterStateFailover    ClusterState = "failover"    // 故障转移中
	ClusterStateSyncing     ClusterState = "syncing"     // 数据同步中
	ClusterStateMaintenance  ClusterState = "maintenance" // 维护模式
	ClusterStateDisabled     ClusterState = "disabled"    // 已禁用
	ClusterStateError       ClusterState = "error"       // 错误状态
)

// NodeConfig 节点配置
type NodeConfig struct {
	// 基本信息
	NodeID      string    `json:"node_id"`
	NodeName    string    `json:"node_name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`

	// 网络配置
	Addresses     []NodeAddress `json:"addresses"`      // 网络地址列表
	ManagementIP  string        `json:"management_ip"`  // 管理IP
	HeartbeatIP   string        `json:"heartbeat_ip"`   // 心跳IP（专用）
	DataSyncIP    string        `json:"data_sync_ip"`   // 数据同步IP（专用）
	FloatingIP    string        `json:"floating_ip"`    // 浮动IP（主节点持有）

	// 角色配置
	Role          HARole       `json:"role"`           // primary/secondary
	State         HAState      `json:"state"`          // 当前状态
	Priority      int          `json:"priority"`       // 优先级（1-100）
	Weight        int          `json:"weight"`         // 权重（负载均衡用）

	// 硬件信息
	HardwareInfo *NodeHardwareInfo `json:"hardware_info,omitempty"`

	// 服务配置
	Services     []string `json:"services"`        // 提供的服务列表
	Capabilities []string `json:"capabilities"`    // 能力列表

	// 心跳端口
	HeartbeatPort int `json:"heartbeat_port"` // 心跳端口

	// 状态信息
	LastHeartbeat time.Time `json:"last_heartbeat"`
	HealthScore   float64   `json:"health_score"` // 0-100
	Uptime        time.Duration `json:"uptime"`

	// 同步状态
	SyncProgress  float64   `json:"sync_progress"` // 0-100
	LastSyncTime  time.Time `json:"last_sync_time"`
	SyncLatency   time.Duration `json:"sync_latency"`
}

// NodeAddress 节点网络地址
type NodeAddress struct {
	Type       string `json:"type"`        // management, heartbeat, data_sync, floating
	IP         string `json:"ip"`
	Port       int    `json:"port"`
	Interface  string `json:"interface"`   // 网络接口名
	Network    string `json:"network"`     // 所属网络
	IsPrimary  bool   `json:"is_primary"`  // 是否主要地址
}

// NodeHardwareInfo 节点硬件信息
type NodeHardwareInfo struct {
	CPUModel      string `json:"cpu_model"`
	CPUCores      int    `json:"cpu_cores"`
	MemoryGB      int    `json:"memory_gb"`
	StorageGB     int    `json:"storage_gb"`
	NetworkCards  []string `json:"network_cards"`
	SerialNumber  string `json:"serial_number"`
}

// ClusterNetworkConfig 集群网络配置
type ClusterNetworkConfig struct {
	// 心跳网络
	HeartbeatNetwork *NetworkInfo `json:"heartbeat_network"`
	HeartbeatPort    int          `json:"heartbeat_port"`
	HeartbeatVLAN    int          `json:"heartbeat_vlan,omitempty"`

	// 数据同步网络
	DataSyncNetwork *NetworkInfo `json:"data_sync_network"`
	DataSyncPort    int          `json:"data_sync_port"`

	// 客户端访问网络
	ClientNetwork  *NetworkInfo `json:"client_network"`
	FloatingIP     string       `json:"floating_ip"`      // 浮动VIP
	FloatingIPMask string       `json:"floating_ip_mask"` // 子网掩码
	FloatingIPIface string      `json:"floating_ip_iface"` // 绑定接口

	// 网络冗余
	EnableNICFailover bool `json:"enable_nic_failover"` // 网卡故障转移
	BondingMode       int  `json:"bonding_mode"`       // 绑定模式
}

// NetworkInfo 网络信息
type NetworkInfo struct {
	NetworkName string `json:"network_name"`
	Subnet      string `json:"subnet"`
	Gateway     string `json:"gateway"`
	VLAN        int    `json:"vlan,omitempty"`
	DNS         []string `json:"dns"`
}

// ClusterStorageConfig 集群存储配置
// 参考 Synology HA 的共享存储架构
type ClusterStorageConfig struct {
	// 存储类型
	StorageType StorageType `json:"storage_type"`

	// 共享存储配置
	SharedStorage *SharedStorageConfig `json:"shared_storage,omitempty"`

	// 复制存储配置
	ReplicatedStorage *ReplicatedStorageConfig `json:"replicated_storage,omitempty"`

	// 存储池配置
	StoragePools []StoragePoolConfig `json:"storage_pools"`

	// 同步策略
	SyncPolicy StorageSyncPolicy `json:"sync_policy"`
}

// StorageType 存储类型
type StorageType string

const (
	StorageTypeShared    StorageType = "shared"    // 共享存储（SAN/iSCSI）
	StorageTypeReplicated StorageType = "replicated" // 复制存储（本地同步）
	StorageTypeHybrid    StorageType = "hybrid"    // 混合模式
)

// SharedStorageConfig 共享存储配置
type SharedStorageConfig struct {
	Type         string   `json:"type"`         // iscsi, fc, nfs
	Target       string   `json:"target"`       // 存储目标地址
	LUN          string   `json:"lun"`          // LUN ID
	MountPoint   string   `json:"mount_point"`  // 挂载点
	FileSystem   string   `json:"file_system"`  // 文件系统类型
	Multipath    bool     `json:"multipath"`    // 多路径支持
	MultipathPaths []string `json:"multipath_paths"` // 多路径列表
}

// ReplicatedStorageConfig 复制存储配置
type ReplicatedStorageConfig struct {
	// 复制模式
	ReplicationMode ReplicationMode `json:"replication_mode"`

	// 同步配置
	SyncInterval     time.Duration `json:"sync_interval"`
	SyncMethod       string        `json:"sync_method"` // rsync, btrfs, zfs
	Compression      bool          `json:"compression"`
	Encryption       bool          `json:"encryption"`

	// 存储路径
	PrimaryPath   string `json:"primary_path"`
	SecondaryPath string `json:"secondary_path"`

	// 增量同步
	EnableIncremental bool `json:"enable_incremental"`
	BlockSizeKB       int  `json:"block_size_kb"`

	// 监控
	CheckInterval time.Duration `json:"check_interval"`
}

// ReplicationMode 复制模式
type ReplicationMode string

const (
	ReplicationModeSync   ReplicationMode = "sync"   // 同步复制
	ReplicationModeAsync  ReplicationMode = "async"  // 异步复制
	ReplicationModeSemiSync ReplicationMode = "semi_sync" // 半同步
)

// StoragePoolConfig 存储池配置
type StoragePoolConfig struct {
	PoolName    string   `json:"pool_name"`
	Volumes     []string `json:"volumes"`
	SizeGB      int      `json:"size_gb"`
	UsedGB      int      `json:"used_gb"`
	FileSystem  string   `json:"file_system"`
	RAIDLevel   string   `json:"raid_level"`
}

// StorageSyncPolicy 存储同步策略
type StorageSyncPolicy struct {
	SyncMode       string        `json:"sync_mode"`       // full, incremental
	SyncInterval   time.Duration `json:"sync_interval"`
	RetryCount     int           `json:"retry_count"`
	BandwidthLimit int           `json:"bandwidth_limit"` // MB/s, 0=无限制
	Priority       int           `json:"priority"`        // 同步优先级
}

// ClusterHeartbeatConfig 集群心跳配置
type ClusterHeartbeatConfig struct {
	// 心跳方式
	HeartbeatMethods []HeartbeatMethod `json:"heartbeat_methods"`

	// 心跳间隔
	Interval     time.Duration `json:"interval"`     // 心跳发送间隔
	Timeout      time.Duration `json:"timeout"`      // 心跳超时时间
	MissThreshold int          `json:"miss_threshold"` // 丢失阈值

	// Phi 检测器配置
	PhiThreshold float64 `json:"phi_threshold"` // Phi 阈值
	SampleWindow int     `json:"sample_window"` // 样本窗口大小

	// 心跳内容
	IncludeMetrics bool `json:"include_metrics"` // 包含性能指标
	IncludeServices bool `json:"include_services"` // 包含服务状态

	// 多路径心跳
	EnableMultiPath bool `json:"enable_multi_path"` // 多路径心跳
}

// HeartbeatMethod 心跳方式
type HeartbeatMethod string

const (
	HeartbeatMethodUDP    HeartbeatMethod = "udp"     // UDP心跳
	HeartbeatMethodTCP    HeartbeatMethod = "tcp"     // TCP心跳
	HeartbeatMethodHTTP   HeartbeatMethod = "http"    // HTTP心跳
	HeartbeatMethodMulticast HeartbeatMethod = "multicast" // 组播心跳
	HeartbeatMethodSerial HeartbeatMethod = "serial"  // 串口心跳（直连）
	HeartbeatMethodShared HeartbeatMethod = "shared"  // 共享存储心跳
)

// ClusterFailoverConfig 集群故障转移配置
type ClusterFailoverConfig struct {
	// 故障转移策略
	Strategy FailoverStrategyType `json:"strategy"`

	// 基本配置
	Enabled         bool          `json:"enabled"`
	DetectionDelay  time.Duration `json:"detection_delay"`  // 故障检测延迟
	ConfirmationWait time.Duration `json:"confirmation_wait"` // 确认等待时间
	TakeoverDelay   time.Duration `json:"takeover_delay"`   // 接管延迟
	VerificationTime time.Duration `json:"verification_time"` // 验证时间

	// 自动回切
	AutoFallback    bool          `json:"auto_fallback"`    // 自动回切
	FallbackDelay   time.Duration `json:"fallback_delay"`   // 回切延迟
	FallbackRequireConfirmation bool `json:"fallback_require_confirmation"` // 回切需确认

	// 服务优先级
	ServicePriority []ServicePriority `json:"service_priority"`

	// SMB 有状态故障转移
	EnableSMBStateful bool `json:"enable_smb_stateful"` // SMB有状态转移
	SMBSessionTimeout time.Duration `json:"smb_session_timeout"`

	// 通知配置
	NotifyOnFailover bool     `json:"notify_on_failover"`
	NotifyChannels   []string `json:"notify_channels"`
}

// FailoverStrategyType 故障转移策略类型
type FailoverStrategyType string

const (
	FailoverStrategyAuto      FailoverStrategyType = "auto"      // 自动故障转移
	FailoverStrategyManual    FailoverStrategyType = "manual"    // 手动故障转移
	FailoverStrategySmart     FailoverStrategyType = "smart"     // 智能故障转移（基于负载）
	FailoverStrategyScheduled FailoverStrategyType = "scheduled" // 定时故障转移
)

// ServicePriority 服务优先级
type ServicePriority struct {
	ServiceName string `json:"service_name"`
	Priority    int    `json:"priority"`
	StartDelay  time.Duration `json:"start_delay"`
	StopDelay   time.Duration `json:"stop_delay"`
}

// ClusterSyncConfig 集群同步配置
type ClusterSyncConfig struct {
	// 同步范围
	SyncScope []SyncScopeType `json:"sync_scope"`

	// 同步间隔
	ConfigSyncInterval time.Duration `json:"config_sync_interval"`
	StateSyncInterval  time.Duration `json:"state_sync_interval"`
	DataSyncInterval   time.Duration `json:"data_sync_interval"`

	// 同步方式
	SyncMethod    SyncMethodType `json:"sync_method"`
	Compression   bool           `json:"compression"`
	Encryption    bool           `json:"encryption"`
	BandwidthLimit int           `json:"bandwidth_limit"`

	// 增量同步
	EnableIncremental bool `json:"enable_incremental"`
	BlockSizeKB       int  `json:"block_size_kb"`
	CheckpointInterval time.Duration `json:"checkpoint_interval"`

	// 验证
	EnableChecksum bool          `json:"enable_checksum"`
	VerifyInterval time.Duration `json:"verify_interval"`
}

// SyncScopeType 同步范围类型
type SyncScopeType string

const (
	SyncScopeConfig  SyncScopeType = "config"  // 配置同步
	SyncScopeState   SyncScopeType = "state"   // 状态同步
	SyncScopeData    SyncScopeType = "data"    // 数据同步
	SyncScopeMetadata SyncScopeType = "metadata" // 元数据同步
	SyncScopeLogs    SyncScopeType = "logs"    // 日志同步
)

// SyncMethodType 同步方式
type SyncMethodType string

const (
	SyncMethodPush    SyncMethodType = "push"    // 推送同步
	SyncMethodPull    SyncMethodType = "pull"    // 拉取同步
	SyncMethodBidirectional SyncMethodType = "bidirectional" // 双向同步
	SyncMethodEventDriven SyncMethodType = "event_driven" // 事件驱动
)

// ClusterConfigManager 集群配置管理器
type ClusterConfigManager struct {
	config      *ClusterConfig
	configFile  string
	dataDir     string
	mu          sync.RWMutex
	logger      *zap.Logger
	hooks       []ClusterConfigHook
	ctx         context.Context
	cancel      context.CancelFunc
}

// ClusterConfigHook 配置变更钩子
type ClusterConfigHook interface {
	OnConfigChange(oldConfig, newConfig *ClusterConfig)
	OnNodeRoleChange(nodeID string, oldRole, newRole HARole)
	OnClusterStateChange(oldState, newState ClusterState)
}

// NewClusterConfigManager 创建集群配置管理器
func NewClusterConfigManager(dataDir string, logger *zap.Logger) (*ClusterConfigManager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	mgr := &ClusterConfigManager{
		configFile: filepath.Join(dataDir, "cluster_config.json"),
		dataDir:    dataDir,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}

	// 加载已有配置
	if err := mgr.loadConfig(); err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to load cluster config", zap.Error(err))
		}
	}

	return mgr, nil
}

// CreateCluster 创建集群
func (mgr *ClusterConfigManager) CreateCluster(config *ClusterConfig) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// 检查是否已存在集群
	if mgr.config != nil && mgr.config.ClusterID != "" {
		return ErrClusterAlreadyExists
	}

	// 验证配置
	if err := mgr.validateClusterConfig(config); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidClusterConfig, err)
	}

	// 设置初始状态
	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()
	config.Version = 1
	config.State = ClusterStateCreating

	// 设置节点状态
	if config.PrimaryNode != nil {
		config.PrimaryNode.State = HAStateStandby
		config.PrimaryNode.Role = HARolePrimary
		config.PrimaryNode.HealthScore = 100.0
	}
	if config.SecondaryNode != nil {
		config.SecondaryNode.State = HAStateStandby
		config.SecondaryNode.Role = HARoleSecondary
		config.PrimaryNode.HealthScore = 100.0
	}

	mgr.config = config

	// 保存配置
	if err := mgr.saveConfig(); err != nil {
		return err
	}

	mgr.logger.Info("Cluster created",
		zap.String("cluster_id", config.ClusterID),
		zap.String("name", config.ClusterName),
		zap.String("type", string(config.ClusterType)),
	)

	return nil
}

// validateClusterConfig 验证集群配置
func (mgr *ClusterConfigManager) validateClusterConfig(config *ClusterConfig) error {
	if config.ClusterName == "" {
		return errors.New("cluster name required")
	}

	// 验证主备节点
	if config.ClusterType == ClusterTypeActivePassive {
		if config.PrimaryNode == nil {
			return ErrPrimaryRequired
		}
		if config.SecondaryNode == nil {
			return ErrSecondaryRequired
		}

		// 验证节点配置
		if err := mgr.validateNodeConfig(config.PrimaryNode); err != nil {
			return fmt.Errorf("primary node: %v", err)
		}
		if err := mgr.validateNodeConfig(config.SecondaryNode); err != nil {
			return fmt.Errorf("secondary node: %v", err)
		}

		// 验证节点不能相同
		if config.PrimaryNode.NodeID == config.SecondaryNode.NodeID {
			return errors.New("primary and secondary must be different nodes")
		}
	}

	// 验证网络配置
	if config.ClusterNetwork != nil {
		if err := mgr.validateNetworkConfig(config.ClusterNetwork); err != nil {
			return fmt.Errorf("network: %v", err)
		}
	}

	return nil
}

// validateNodeConfig 验证节点配置
func (mgr *ClusterConfigManager) validateNodeConfig(node *NodeConfig) error {
	if node.NodeID == "" {
		return errors.New("node_id required")
	}
	if node.NodeName == "" {
		return errors.New("node_name required")
	}
	if node.ManagementIP == "" {
		return errors.New("management_ip required")
	}
	if node.HeartbeatIP == "" {
		return errors.New("heartbeat_ip required")
	}
	return nil
}

// validateNetworkConfig 验证网络配置
func (mgr *ClusterConfigManager) validateNetworkConfig(net *ClusterNetworkConfig) error {
	if net.HeartbeatPort <= 0 || net.HeartbeatPort > 65535 {
		return errors.New("invalid heartbeat port")
	}
	return nil
}

// GetClusterConfig 获取集群配置
func (mgr *ClusterConfigManager) GetClusterConfig() *ClusterConfig {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	return mgr.config
}

// UpdateClusterConfig 更新集群配置
func (mgr *ClusterConfigManager) UpdateClusterConfig(newConfig *ClusterConfig) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.config == nil {
		return ErrClusterNotFound
	}

	oldConfig := mgr.config

	// 验证新配置
	if err := mgr.validateClusterConfig(newConfig); err != nil {
		return err
	}

	// 更新配置
	newConfig.UpdatedAt = time.Now()
	newConfig.Version = mgr.config.Version + 1
	mgr.config = newConfig

	// 保存
	if err := mgr.saveConfig(); err != nil {
		mgr.config = oldConfig // 回滚
		return err
	}

	// 触发钩子
	mgr.notifyConfigChange(oldConfig, newConfig)

	mgr.logger.Info("Cluster config updated",
		zap.Int("version", newConfig.Version),
	)

	return nil
}

// SetPrimaryNode 设置主节点
func (mgr *ClusterConfigManager) SetPrimaryNode(node *NodeConfig) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.config == nil {
		return ErrClusterNotFound
	}

	oldPrimary := mgr.config.PrimaryNode

	// 检查节点是否已在集群中
	if mgr.config.SecondaryNode != nil && mgr.config.SecondaryNode.NodeID == node.NodeID {
		// 交换角色
		mgr.config.SecondaryNode.Role = HARolePrimary
		mgr.config.SecondaryNode.State = HAStateActive
		mgr.config.PrimaryNode = mgr.config.SecondaryNode
		node.Role = HARoleSecondary
		node.State = HAStatePassive
		mgr.config.SecondaryNode = node
	} else {
		// 直接设置新主节点
		node.Role = HARolePrimary
		node.State = HAStateActive
		mgr.config.PrimaryNode = node
	}

	mgr.config.UpdatedAt = time.Now()
	mgr.config.Version++

	if err := mgr.saveConfig(); err != nil {
		return err
	}

	// 触发钩子
	if oldPrimary != nil {
		mgr.notifyNodeRoleChange(oldPrimary.NodeID, HARolePrimary, HARoleSecondary)
	}
	mgr.notifyNodeRoleChange(node.NodeID, HARoleSecondary, HARolePrimary)

	mgr.logger.Info("Primary node set",
		zap.String("node_id", node.NodeID),
		zap.String("name", node.NodeName),
	)

	return nil
}

// SetSecondaryNode 设置备节点
func (mgr *ClusterConfigManager) SetSecondaryNode(node *NodeConfig) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.config == nil {
		return ErrClusterNotFound
	}

	node.Role = HARoleSecondary
	node.State = HAStatePassive
	mgr.config.SecondaryNode = node
	mgr.config.UpdatedAt = time.Now()
	mgr.config.Version++

	if err := mgr.saveConfig(); err != nil {
		return err
	}

	mgr.logger.Info("Secondary node set",
		zap.String("node_id", node.NodeID),
	)

	return nil
}

// UpdateNodeState 更新节点状态
func (mgr *ClusterConfigManager) UpdateNodeState(nodeID string, state HAState, healthScore float64) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.config == nil {
		return ErrClusterNotFound
	}

	// 查找节点
	var node *NodeConfig
	if mgr.config.PrimaryNode != nil && mgr.config.PrimaryNode.NodeID == nodeID {
		node = mgr.config.PrimaryNode
	} else if mgr.config.SecondaryNode != nil && mgr.config.SecondaryNode.NodeID == nodeID {
		node = mgr.config.SecondaryNode
	}

	if node == nil {
		return ErrNodeNotInCluster
	}

	node.State = state
	node.HealthScore = healthScore
	node.LastHeartbeat = time.Now()

	return nil
}

// SetClusterState 设置集群状态
func (mgr *ClusterConfigManager) SetClusterState(state ClusterState) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.config == nil {
		return ErrClusterNotFound
	}

	oldState := mgr.config.State
	mgr.config.State = state
	mgr.config.UpdatedAt = time.Now()

	if state == ClusterStateActive {
		mgr.config.ActiveSince = time.Now()
	}

	if err := mgr.saveConfig(); err != nil {
		return err
	}

	mgr.notifyClusterStateChange(oldState, state)

	mgr.logger.Info("Cluster state changed",
		zap.String("old", string(oldState)),
		zap.String("new", string(state)),
	)

	return nil
}

// GetPrimaryNode 获取主节点配置
func (mgr *ClusterConfigManager) GetPrimaryNode() *NodeConfig {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	if mgr.config == nil {
		return nil
	}
	return mgr.config.PrimaryNode
}

// GetSecondaryNode 获取备节点配置
func (mgr *ClusterConfigManager) GetSecondaryNode() *NodeConfig {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()
	if mgr.config == nil {
		return nil
	}
	return mgr.config.SecondaryNode
}

// GetNodeByID 根据ID获取节点配置
func (mgr *ClusterConfigManager) GetNodeByID(nodeID string) *NodeConfig {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if mgr.config == nil {
		return nil
	}

	if mgr.config.PrimaryNode != nil && mgr.config.PrimaryNode.NodeID == nodeID {
		return mgr.config.PrimaryNode
	}
	if mgr.config.SecondaryNode != nil && mgr.config.SecondaryNode.NodeID == nodeID {
		return mgr.config.SecondaryNode
	}

	return nil
}

// SwapRoles 交换主备角色
func (mgr *ClusterConfigManager) SwapRoles() error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.config == nil {
		return ErrClusterNotFound
	}

	if mgr.config.PrimaryNode == nil || mgr.config.SecondaryNode == nil {
		return ErrClusterNotActive
	}

	// 检查备节点健康状态
	if mgr.config.SecondaryNode.HealthScore < 80 {
		return fmt.Errorf("secondary node health too low: %.2f", mgr.config.SecondaryNode.HealthScore)
	}

	// 交换角色
	oldPrimaryID := mgr.config.PrimaryNode.NodeID
	oldSecondaryID := mgr.config.SecondaryNode.NodeID

	mgr.config.PrimaryNode.Role = HARoleSecondary
	mgr.config.PrimaryNode.State = HAStatePassive

	mgr.config.SecondaryNode.Role = HARolePrimary
	mgr.config.SecondaryNode.State = HAStateActive

	// 交换节点引用
	mgr.config.PrimaryNode, mgr.config.SecondaryNode = mgr.config.SecondaryNode, mgr.config.PrimaryNode

	mgr.config.UpdatedAt = time.Now()
	mgr.config.Version++

	if err := mgr.saveConfig(); err != nil {
		return err
	}

	// 触发钩子
	mgr.notifyNodeRoleChange(oldPrimaryID, HARolePrimary, HARoleSecondary)
	mgr.notifyNodeRoleChange(oldSecondaryID, HARoleSecondary, HARolePrimary)

	mgr.logger.Info("Roles swapped",
		zap.String("new_primary", mgr.config.PrimaryNode.NodeID),
		zap.String("new_secondary", mgr.config.SecondaryNode.NodeID),
	)

	return nil
}

// DeleteCluster 删除集群
func (mgr *ClusterConfigManager) DeleteCluster() error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	if mgr.config == nil {
		return ErrClusterNotFound
	}

	// 检查集群状态
	if mgr.config.State == ClusterStateActive || mgr.config.State == ClusterStateFailover {
		return errors.New("cannot delete active or failover cluster")
	}

	clusterID := mgr.config.ClusterID
	mgr.config = nil

	// 删除配置文件
	if err := os.Remove(mgr.configFile); err != nil && !os.IsNotExist(err) {
		return err
	}

	mgr.logger.Info("Cluster deleted",
		zap.String("cluster_id", clusterID),
	)

	return nil
}

// RegisterHook 注册配置变更钩子
func (mgr *ClusterConfigManager) RegisterHook(hook ClusterConfigHook) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.hooks = append(mgr.hooks, hook)
}

// 钩子通知方法
func (mgr *ClusterConfigManager) notifyConfigChange(old, new *ClusterConfig) {
	for _, hook := range mgr.hooks {
		go hook.OnConfigChange(old, new)
	}
}

func (mgr *ClusterConfigManager) notifyNodeRoleChange(nodeID string, oldRole, newRole HARole) {
	for _, hook := range mgr.hooks {
		go hook.OnNodeRoleChange(nodeID, oldRole, newRole)
	}
}

func (mgr *ClusterConfigManager) notifyClusterStateChange(oldState, newState ClusterState) {
	for _, hook := range mgr.hooks {
		go hook.OnClusterStateChange(oldState, newState)
	}
}

// loadConfig 加载配置
func (mgr *ClusterConfigManager) loadConfig() error {
	data, err := os.ReadFile(mgr.configFile)
	if err != nil {
		return err
	}

	var config ClusterConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	mgr.config = &config
	return nil
}

// saveConfig 保存配置
func (mgr *ClusterConfigManager) saveConfig() error {
	if mgr.config == nil {
		return nil
	}

	data, err := json.MarshalIndent(mgr.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(mgr.configFile, data, 0600)
}

// ExportConfig 导出配置
func (mgr *ClusterConfigManager) ExportConfig() ([]byte, error) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if mgr.config == nil {
		return nil, ErrClusterNotFound
	}

	return json.MarshalIndent(mgr.config, "", "  ")
}

// ImportConfig 导入配置
func (mgr *ClusterConfigManager) ImportConfig(data []byte) error {
	var config ClusterConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	return mgr.CreateCluster(&config)
}

// GetClusterStatus 获取集群状态摘要
func (mgr *ClusterConfigManager) GetClusterStatus() *ClusterStatus {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	if mgr.config == nil {
		return nil
	}

	status := &ClusterStatus{
		ClusterID:   mgr.config.ClusterID,
		ClusterName: mgr.config.ClusterName,
		State:       mgr.config.State,
		ClusterType: mgr.config.ClusterType,
		ActiveSince: mgr.config.ActiveSince,
	}

	if mgr.config.PrimaryNode != nil {
		status.PrimaryNode = &NodeStatus{
			NodeID:      mgr.config.PrimaryNode.NodeID,
			NodeName:    mgr.config.PrimaryNode.NodeName,
			Role:        mgr.config.PrimaryNode.Role,
			State:       mgr.config.PrimaryNode.State,
			HealthScore: mgr.config.PrimaryNode.HealthScore,
			ManagementIP: mgr.config.PrimaryNode.ManagementIP,
		}
	}

	if mgr.config.SecondaryNode != nil {
		status.SecondaryNode = &NodeStatus{
			NodeID:      mgr.config.SecondaryNode.NodeID,
			NodeName:    mgr.config.SecondaryNode.NodeName,
			Role:        mgr.config.SecondaryNode.Role,
			State:       mgr.config.SecondaryNode.State,
			HealthScore: mgr.config.SecondaryNode.HealthScore,
			ManagementIP: mgr.config.SecondaryNode.ManagementIP,
		}
	}

	return status
}

// ClusterStatus 集群状态摘要
type ClusterStatus struct {
	ClusterID   string       `json:"cluster_id"`
	ClusterName string       `json:"cluster_name"`
	State       ClusterState `json:"state"`
	ClusterType ClusterType  `json:"cluster_type"`
	ActiveSince time.Time    `json:"active_since,omitempty"`
	PrimaryNode *NodeStatus  `json:"primary_node"`
	SecondaryNode *NodeStatus `json:"secondary_node"`
}

// NodeStatus 节点状态摘要
type NodeStatus struct {
	NodeID      string  `json:"node_id"`
	NodeName    string  `json:"node_name"`
	Role        HARole  `json:"role"`
	State       HAState `json:"state"`
	HealthScore float64 `json:"health_score"`
	ManagementIP string `json:"management_ip"`
}

// Stop 停止管理器
func (mgr *ClusterConfigManager) Stop() {
	mgr.cancel()
}