// Package iscsifc 提供 iSCSI 和光纤通道管理
// 对标 TrueNAS iSCSI/Fibre Channel，提供 SAN 存储支持
package iscsifc

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// ========== iSCSI 目标管理 ==========

// ISCSITarget iSCSI 目标.
type ISCSITarget struct {
	ID                 string            `json:"id"`
	Name               string            `json:"name"`
	Alias              string            `json:"alias"`
	IQN                string            `json:"iqn"` // iSCSI Qualified Name
	PortalGroupTag     int               `json:"portal_group_tag"`
	InitiatorGroupIDs  []int             `json:"initiator_group_ids"`
	AuthMethod         AuthMethod        `json:"auth_method"`
	AuthGroup          int               `json:"auth_group"`
	MaxSessions        int               `json:"max_sessions"`
	MaxRecvDataSegment int               `json:"max_recv_data_segment_length"`
	MaxXmitDataSegment int               `json:"max_xmit_data_segment_length"`
	MaxBurstLength     int               `json:"max_burst_length"`
	FirstBurstLength   int               `json:"first_burst_length"`
	DefaultTime2Wait   int               `json:"default_time_2_wait"`
	DefaultTime2Retain int               `json:"default_time_2_retain"`
	Enabled            bool              `json:"enabled"`
	Stats              TargetStats       `json:"stats"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

// AuthMethod 认证方法.
type AuthMethod string

const (
	AuthMethodNone       AuthMethod = "none"
	AuthMethodCHAP       AuthMethod = "chap"
	AuthMethodCHAPMutual AuthMethod = "chap_mutual"
)

// TargetStats 目标统计.
type TargetStats struct {
	ActiveSessions    int       `json:"active_sessions"`
	TotalSessions     int64     `json:"total_sessions"`
	BytesRead         int64     `json:"bytes_read"`
	BytesWritten      int64     `json:"bytes_written"`
	CommandsProcessed int64     `json:"commands_processed"`
	Errors            int64     `json:"errors"`
	LastActivity      time.Time `json:"last_activity"`
}

// ========== iSCSI 门户管理 ==========

// ISCSIPortal iSCSI 门户.
type ISCSIPortal struct {
	ID             string            `json:"id"`
	Tag            int               `json:"tag"`
	IP             string            `json:"ip"`
	Port           int               `json:"port"`
	Protocol       PortalProtocol    `json:"protocol"`
	DiscoveryAuth  AuthMethod        `json:"discovery_auth"`
	MaxConnections int               `json:"max_connections"`
	Enabled        bool              `json:"enabled"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

// PortalProtocol 门户协议.
type PortalProtocol string

const (
	PortalProtocolIPv4 PortalProtocol = "ipv4"
	PortalProtocolIPv6 PortalProtocol = "ipv6"
)

// ========== iSCSI LUN 管理 ==========

// ISCSILun iSCSI LUN.
type ISCSILun struct {
	ID              string            `json:"id"`
	TargetID        string            `json:"target_id"`
	LUN             int               `json:"lun"`
	Type            LUNType           `json:"type"`
	Path            string            `json:"path"`
	Size            int64             `json:"size"` // 字节
	BlockSize       int               `json:"block_size"`
	ThinProvisioned bool              `json:"thin_provisioned"`
	ReadOnly        bool              `json:"read_only"`
	ScsiID          string            `json:"scsi_id"`
	ScsiSN          string            `json:"scsi_sn"`
	ProductId       string            `json:"product_id"`
	VendorId        string            `json:"vendor_id"`
	Enabled         bool              `json:"enabled"`
	Stats           LUNStats          `json:"stats"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// LUNType LUN 类型.
type LUNType string

const (
	LUNTypeDisk        LUNType = "disk"
	LUNTypeFile        LUNType = "file"
	LUNTypeZvol        LUNType = "zvol"
	LUNTypePassthrough LUNType = "passthrough"
)

// LUNStats LUN 统计.
type LUNStats struct {
	ReadOps        int64     `json:"read_ops"`
	WriteOps       int64     `json:"write_ops"`
	ReadBytes      int64     `json:"read_bytes"`
	WriteBytes     int64     `json:"write_bytes"`
	ReadLatencyNs  int64     `json:"read_latency_ns"`
	WriteLatencyNs int64     `json:"write_latency_ns"`
	QueueDepth     int       `json:"queue_depth"`
	LastActivity   time.Time `json:"last_activity"`
}

// ========== 光纤通道管理 ==========

// FCPort 光纤通道端口.
type FCPort struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	WorldWideName string            `json:"wwn"` // WWN
	PortID        string            `json:"port_id"`
	Type          FCPortType        `json:"type"`
	Speed         int               `json:"speed"` // Gbps
	State         FCPortState       `json:"state"`
	Topology      FCTopology        `json:"topology"`
	FabricName    string            `json:"fabric_name"`
	NodeName      string            `json:"node_name"`
	MaxFrameSize  int               `json:"max_frame_size"`
	Stats         FCPortStats       `json:"stats"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	LastChange    time.Time         `json:"last_change"`
	CreatedAt     time.Time         `json:"created_at"`
}

// FCPortType FC 端口类型.
type FCPortType string

const (
	FCPortTypeNPort  FCPortType = "n_port"  // 点对点
	FCPortTypeNLPort FCPortType = "nl_port" // 仲裁环路
	FCPortTypeEPort  FCPortType = "e_port"  // 扩展端口
	FCPortTypeFPort  FCPortType = "f_port"  // 交换端口
	FCPortTypeFLPort FCPortType = "fl_port" // 交换环路
)

// FCPortState FC 端口状态.
type FCPortState string

const (
	FCPortStateOnline  FCPortState = "online"
	FCPortStateOffline FCPortState = "offline"
	FCPortStateError   FCPortState = "error"
	FCPortStateBypass  FCPortState = "bypass"
	FCPortStateDiag    FCPortState = "diag"
)

// FCTopology FC 拓扑.
type FCTopology string

const (
	FCTopologyPointToPoint   FCTopology = "point_to_point"
	FCTopologyArbitratedLoop FCTopology = "arbitrated_loop"
	FCTopologySwitchedFabric FCTopology = "switched_fabric"
)

// FCPortStats FC 端口统计.
type FCPortStats struct {
	RxBytes       int64     `json:"rx_bytes"`
	TxBytes       int64     `json:"tx_bytes"`
	RxFrames      int64     `json:"rx_frames"`
	TxFrames      int64     `json:"tx_frames"`
	Errors        int64     `json:"errors"`
	DroppedFrames int64     `json:"dropped_frames"`
	LinkResets    int64     `json:"link_resets"`
	LastActivity  time.Time `json:"last_activity"`
}

// ========== iSCSI/FC 管理器 ==========

// ISCSIFCManager iSCSI/FC 管理器.
type ISCSIFCManager struct {
	mu      sync.RWMutex
	targets map[string]*ISCSITarget
	portals map[string]*ISCSIPortal
	luns    map[string]*ISCSILun
	fcPorts map[string]*FCPort
	config  ManagerConfig
	stats   ManagerStats
}

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	ISCSIEnabled       bool       `json:"iscsi_enabled"`
	FCEnabled          bool       `json:"fc_enabled"`
	DefaultPortalIP    string     `json:"default_portal_ip"`
	DefaultPortalPort  int        `json:"default_portal_port"`
	MaxTargets         int        `json:"max_targets"`
	MaxLUNsPerTarget   int        `json:"max_luns_per_target"`
	DefaultBlockSize   int        `json:"default_block_size"`
	AuthRequired       bool       `json:"auth_required"`
	DiscoveryAuth      AuthMethod `json:"discovery_auth"`
	MaxSessions        int        `json:"max_sessions"`
	MaxRecvDataSegment int        `json:"max_recv_data_segment"`
	MaxXmitDataSegment int        `json:"max_xmit_data_segment"`
	MaxBurstLength     int        `json:"max_burst_length"`
	FirstBurstLength   int        `json:"first_burst_length"`
}

// ManagerStats 管理器统计.
type ManagerStats struct {
	TotalTargets      int       `json:"total_targets"`
	EnabledTargets    int       `json:"enabled_targets"`
	TotalPortals      int       `json:"total_portals"`
	TotalLUNs         int       `json:"total_luns"`
	TotalFCPorts      int       `json:"total_fc_ports"`
	OnlineFCPorts     int       `json:"online_fc_ports"`
	ActiveSessions    int       `json:"active_sessions"`
	TotalBytesRead    int64     `json:"total_bytes_read"`
	TotalBytesWritten int64     `json:"total_bytes_written"`
	LastActivity      time.Time `json:"last_activity"`
}

// NewISCSIFCManager 创建 iSCSI/FC 管理器.
func NewISCSIFCManager(config ManagerConfig) *ISCSIFCManager {
	// 设置默认值
	if config.DefaultPortalPort == 0 {
		config.DefaultPortalPort = 3260
	}
	if config.MaxTargets == 0 {
		config.MaxTargets = 256
	}
	if config.MaxLUNsPerTarget == 0 {
		config.MaxLUNsPerTarget = 256
	}
	if config.DefaultBlockSize == 0 {
		config.DefaultBlockSize = 512
	}
	if config.MaxSessions == 0 {
		config.MaxSessions = 65536
	}
	if config.MaxRecvDataSegment == 0 {
		config.MaxRecvDataSegment = 262144 // 256KB
	}
	if config.MaxXmitDataSegment == 0 {
		config.MaxXmitDataSegment = 262144
	}
	if config.MaxBurstLength == 0 {
		config.MaxBurstLength = 16776192 // 16MB
	}
	if config.FirstBurstLength == 0 {
		config.FirstBurstLength = 65536 // 64KB
	}

	return &ISCSIFCManager{
		targets: make(map[string]*ISCSITarget),
		portals: make(map[string]*ISCSIPortal),
		luns:    make(map[string]*ISCSILun),
		fcPorts: make(map[string]*FCPort),
		config:  config,
	}
}

// ========== iSCSI 目标管理 ==========

// CreateTarget 创建 iSCSI 目标.
func (m *ISCSIFCManager) CreateTarget(target ISCSITarget) (*ISCSITarget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.ISCSIEnabled {
		return nil, fmt.Errorf("iSCSI 未启用")
	}

	if len(m.targets) >= m.config.MaxTargets {
		return nil, fmt.Errorf("已达到最大目标数: %d", m.config.MaxTargets)
	}

	if target.ID == "" {
		target.ID = fmt.Sprintf("target-%s-%d", target.Name, time.Now().UnixNano())
	}

	if _, exists := m.targets[target.ID]; exists {
		return nil, fmt.Errorf("目标已存在: %s", target.ID)
	}

	// 设置默认值
	if target.MaxSessions == 0 {
		target.MaxSessions = m.config.MaxSessions
	}
	if target.MaxRecvDataSegment == 0 {
		target.MaxRecvDataSegment = m.config.MaxRecvDataSegment
	}
	if target.MaxXmitDataSegment == 0 {
		target.MaxXmitDataSegment = m.config.MaxXmitDataSegment
	}
	if target.MaxBurstLength == 0 {
		target.MaxBurstLength = m.config.MaxBurstLength
	}
	if target.FirstBurstLength == 0 {
		target.FirstBurstLength = m.config.FirstBurstLength
	}

	target.Enabled = true
	target.CreatedAt = time.Now()
	target.UpdatedAt = time.Now()

	m.targets[target.ID] = &target
	m.updateStats()

	return &target, nil
}

// DeleteTarget 删除 iSCSI 目标.
func (m *ISCSIFCManager) DeleteTarget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.targets[id]
	if !exists {
		return fmt.Errorf("目标不存在: %s", id)
	}

	if target.Stats.ActiveSessions > 0 {
		return fmt.Errorf("目标有活跃会话，无法删除")
	}

	// 删除关联的 LUN
	for lunID, lun := range m.luns {
		if lun.TargetID == id {
			delete(m.luns, lunID)
		}
	}

	delete(m.targets, id)
	m.updateStats()

	return nil
}

// GetTarget 获取 iSCSI 目标.
func (m *ISCSIFCManager) GetTarget(id string) (*ISCSITarget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	target, exists := m.targets[id]
	if !exists {
		return nil, fmt.Errorf("目标不存在: %s", id)
	}

	return target, nil
}

// ListTargets 列出所有 iSCSI 目标.
func (m *ISCSIFCManager) ListTargets() []*ISCSITarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ISCSITarget, 0, len(m.targets))
	for _, t := range m.targets {
		result = append(result, t)
	}

	return result
}

// ========== 门户管理 ==========

// CreatePortal 创建门户.
func (m *ISCSIFCManager) CreatePortal(portal ISCSIPortal) (*ISCSIPortal, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if portal.ID == "" {
		portal.ID = fmt.Sprintf("portal-%s-%d", portal.IP, time.Now().UnixNano())
	}

	if _, exists := m.portals[portal.ID]; exists {
		return nil, fmt.Errorf("门户已存在: %s", portal.ID)
	}

	if portal.Port == 0 {
		portal.Port = m.config.DefaultPortalPort
	}
	if portal.Protocol == "" {
		portal.Protocol = PortalProtocolIPv4
	}

	portal.Enabled = true
	portal.CreatedAt = time.Now()

	m.portals[portal.ID] = &portal
	m.updateStats()

	return &portal, nil
}

// DeletePortal 删除门户.
func (m *ISCSIFCManager) DeletePortal(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.portals[id]; !exists {
		return fmt.Errorf("门户不存在: %s", id)
	}

	delete(m.portals, id)
	m.updateStats()

	return nil
}

// ListPortals 列出所有门户.
func (m *ISCSIFCManager) ListPortals() []*ISCSIPortal {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ISCSIPortal, 0, len(m.portals))
	for _, p := range m.portals {
		result = append(result, p)
	}

	return result
}

// ========== LUN 管理 ==========

// CreateLUN 创建 LUN.
func (m *ISCSIFCManager) CreateLUN(lun ISCSILun) (*ISCSILun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, exists := m.targets[lun.TargetID]
	if !exists {
		return nil, fmt.Errorf("目标不存在: %s", lun.TargetID)
	}

	// 检查目标的 LUN 数量
	targetLUNs := 0
	for _, l := range m.luns {
		if l.TargetID == lun.TargetID {
			targetLUNs++
		}
	}
	if targetLUNs >= m.config.MaxLUNsPerTarget {
		return nil, fmt.Errorf("目标已达到最大 LUN 数: %d", m.config.MaxLUNsPerTarget)
	}

	if lun.ID == "" {
		lun.ID = fmt.Sprintf("lun-%s-%d", lun.TargetID, time.Now().UnixNano())
	}

	// 设置默认值
	if lun.BlockSize == 0 {
		lun.BlockSize = m.config.DefaultBlockSize
	}
	if lun.Type == "" {
		lun.Type = LUNTypeDisk
	}
	if lun.ScsiID == "" {
		lun.ScsiID = lun.ID
	}
	if lun.ProductId == "" {
		lun.ProductId = "NAS-OS Storage"
	}
	if lun.VendorId == "" {
		lun.VendorId = "NAS-OS"
	}

	lun.Enabled = true
	lun.CreatedAt = time.Now()
	lun.UpdatedAt = time.Now()

	m.luns[lun.ID] = &lun
	m.updateStats()

	_ = target // 避免未使用变量警告

	return &lun, nil
}

// DeleteLUN 删除 LUN.
func (m *ISCSIFCManager) DeleteLUN(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.luns[id]; !exists {
		return fmt.Errorf("LUN 不存在: %s", id)
	}

	delete(m.luns, id)
	m.updateStats()

	return nil
}

// ListLUNs 列出所有 LUN.
func (m *ISCSIFCManager) ListLUNs() []*ISCSILun {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ISCSILun, 0, len(m.luns))
	for _, l := range m.luns {
		result = append(result, l)
	}

	return result
}

// ListLUNsByTarget 列出目标的所有 LUN.
func (m *ISCSIFCManager) ListLUNsByTarget(targetID string) []*ISCSILun {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ISCSILun, 0)
	for _, l := range m.luns {
		if l.TargetID == targetID {
			result = append(result, l)
		}
	}

	return result
}

// ========== 光纤通道管理 ==========

// RegisterFCPort 注册 FC 端口.
func (m *ISCSIFCManager) RegisterFCPort(port FCPort) (*FCPort, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.config.FCEnabled {
		return nil, fmt.Errorf("光纤通道未启用")
	}

	if port.ID == "" {
		port.ID = fmt.Sprintf("fc-%s-%d", port.Name, time.Now().UnixNano())
	}

	if _, exists := m.fcPorts[port.ID]; exists {
		return nil, fmt.Errorf("FC 端口已存在: %s", port.ID)
	}

	if port.MaxFrameSize == 0 {
		port.MaxFrameSize = 2112 // 标准 FC 帧大小
	}

	port.State = FCPortStateOffline
	port.CreatedAt = time.Now()
	port.LastChange = time.Now()

	m.fcPorts[port.ID] = &port
	m.updateStats()

	return &port, nil
}

// UnregisterFCPort 注销 FC 端口.
func (m *ISCSIFCManager) UnregisterFCPort(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.fcPorts[id]; !exists {
		return fmt.Errorf("FC 端口不存在: %s", id)
	}

	delete(m.fcPorts, id)
	m.updateStats()

	return nil
}

// GetFCPort 获取 FC 端口.
func (m *ISCSIFCManager) GetFCPort(id string) (*FCPort, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	port, exists := m.fcPorts[id]
	if !exists {
		return nil, fmt.Errorf("FC 端口不存在: %s", id)
	}

	return port, nil
}

// ListFCPorts 列出所有 FC 端口.
func (m *ISCSIFCManager) ListFCPorts() []*FCPort {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*FCPort, 0, len(m.fcPorts))
	for _, p := range m.fcPorts {
		result = append(result, p)
	}

	return result
}

// UpdateFCPortState 更新 FC 端口状态.
func (m *ISCSIFCManager) UpdateFCPortState(id string, state FCPortState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	port, exists := m.fcPorts[id]
	if !exists {
		return fmt.Errorf("FC 端口不存在: %s", id)
	}

	port.State = state
	port.LastChange = time.Now()
	m.updateStats()

	return nil
}

// ========== 辅助方法 ==========

// updateStats 更新统计.
func (m *ISCSIFCManager) updateStats() {
	m.stats.TotalTargets = len(m.targets)
	m.stats.EnabledTargets = 0
	m.stats.TotalPortals = len(m.portals)
	m.stats.TotalLUNs = len(m.luns)
	m.stats.TotalFCPorts = len(m.fcPorts)
	m.stats.OnlineFCPorts = 0
	m.stats.ActiveSessions = 0

	for _, t := range m.targets {
		if t.Enabled {
			m.stats.EnabledTargets++
		}
		m.stats.ActiveSessions += t.Stats.ActiveSessions
	}

	for _, p := range m.fcPorts {
		if p.State == FCPortStateOnline {
			m.stats.OnlineFCPorts++
		}
	}
}

// GetStats 获取统计.
func (m *ISCSIFCManager) GetStats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// SaveConfig 保存配置.
func (m *ISCSIFCManager) SaveConfig(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0640)
}

// LoadConfig 加载配置.
func (m *ISCSIFCManager) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return json.Unmarshal(data, &m.config)
}
