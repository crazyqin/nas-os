// Package rdma 实现 iSCSI/iSER 和 NFS over RDMA 协议
package rdma

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ========== iSER (iSCSI Extensions for RDMA) ==========

// ISERConfig iSER 配置
type ISERConfig struct {
	// 启用 iSER
	Enabled bool `json:"enabled"`

	// 目标 IQN
	TargetIQN string `json:"targetIqn"`

	// 目标地址
	TargetAddr string `json:"targetAddr"`

	// 目标端口
	TargetPort int `json:"targetPort"`

	// 最大传输单元
	MaxMTU int `json:"maxMtu"`

	// 队列深度
	QueueDepth int `json:"queueDepth"`

	// 连接超时
	ConnectTimeout time.Duration `json:"connectTimeout"`

	// 认证
	CHAPUsername string `json:"chapUsername,omitempty"`
	CHAPSecret   string `json:"chapSecret,omitempty"`
}

// DefaultISERConfig 返回默认 iSER 配置
func DefaultISERConfig() *ISERConfig {
	return &ISERConfig{
		Enabled:        true,
		TargetPort:     3260,
		MaxMTU:         65536,
		QueueDepth:     32,
		ConnectTimeout: 30 * time.Second,
	}
}

// ISERSession iSER 会话
type ISERSession struct {
	mu sync.RWMutex

	config    *ISERConfig
	endpoint  *RDMAEndpoint
	sessionID string

	// 连接状态
	connected bool

	// 统计
	stats ISERStats
}

// ISERStats iSER 统计信息
type ISERStats struct {
	ReadOps      uint64 `json:"readOps"`
	WriteOps     uint64 `json:"writeOps"`
	BytesRead    uint64 `json:"bytesRead"`
	BytesWritten uint64 `json:"bytesWritten"`
	IOPSLatency  uint64 `json:"avgLatencyNs"` // 纳秒
	Errors       uint64 `json:"errors"`
}

// NewISERSession 创建 iSER 会话
func NewISERSession(config *ISERConfig) (*ISERSession, error) {
	if config == nil {
		config = DefaultISERConfig()
	}

	return &ISERSession{
		config:    config,
		sessionID: generateSessionID(),
	}, nil
}

// Connect 连接到 iSER 目标
func (s *ISERSession) Connect(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.connected {
		return nil
	}

	// 创建 RDMA 端点
	rdmaConfig := DefaultRDMAConfig()
	rdmaConfig.Transport = "iSER"
	rdmaConfig.Port = s.config.TargetPort

	endpoint, err := NewRDMAEndpoint(rdmaConfig)
	if err != nil {
		return fmt.Errorf("failed to create RDMA endpoint: %w", err)
	}

	// 连接
	addr := fmt.Sprintf("%s:%d", s.config.TargetAddr, s.config.TargetPort)
	if err := endpoint.Connect(ctx, addr); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	s.endpoint = endpoint
	s.connected = true

	return nil
}

// Disconnect 断开连接
func (s *ISERSession) Disconnect() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.connected {
		return nil
	}

	if s.endpoint != nil {
		_ = s.endpoint.Disconnect()
		s.endpoint = nil
	}

	s.connected = false
	return nil
}

// Read 执行读取操作
func (s *ISERSession) Read(ctx context.Context, lba uint64, buf []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.connected || s.endpoint == nil {
		return ErrNotConnected
	}

	// 发送 iSER 读请求
	// 实际实现需要封装 iSCSI PDU 并通过 RDMA 发送
	err := s.endpoint.Read(ctx, lba, buf)
	if err != nil {
		return err
	}

	// 更新统计
	s.stats.ReadOps++
	s.stats.BytesRead += uint64(len(buf))

	return nil
}

// Write 执行写入操作
func (s *ISERSession) Write(ctx context.Context, lba uint64, data []byte) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.connected || s.endpoint == nil {
		return ErrNotConnected
	}

	// 发送 iSER 写请求
	err := s.endpoint.Write(ctx, data, lba)
	if err != nil {
		return err
	}

	// 更新统计
	s.stats.WriteOps++
	s.stats.BytesWritten += uint64(len(data))

	return nil
}

// GetStats 获取统计信息
func (s *ISERSession) GetStats() ISERStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// IsConnected 检查是否已连接
func (s *ISERSession) IsConnected() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connected
}

// ========== NFS over RDMA ==========

// NFSRDMAConfig NFS over RDMA 配置
type NFSRDMAConfig struct {
	// 启用 NFS RDMA
	Enabled bool `json:"enabled"`

	// 导出路径
	ExportPath string `json:"exportPath"`

	// 服务器地址
	ServerAddr string `json:"serverAddr"`

	// NFS 版本
	NFSVersion string `json:"nfsVersion"` // nfs4, nfs4.1, nfs4.2

	// 传输协议
	Protocol string `json:"protocol"` // tcp, rdma

	// 挂载选项
	MountOptions []string `json:"mountOptions"`

	// 缓存设置
	AttrCacheTimeout   time.Duration `json:"attrCacheTimeout"`
	DirCacheTimeout    time.Duration `json:"dirCacheTimeout"`
	ReadAheadSize      int           `json:"readAheadSize"`
	WriteBufferSize    int           `json:"writeBufferSize"`

	// 性能调优
	MaxReadSize  int `json:"maxReadSize"`
	MaxWriteSize int `json:"maxWriteSize"`

	// 安全
	SecurityFlavor string `json:"securityFlavor"` // sys, krb5, krb5i, krb5p
}

// DefaultNFSRDMAConfig 返回默认 NFS RDMA 配置
func DefaultNFSRDMAConfig() *NFSRDMAConfig {
	return &NFSRDMAConfig{
		Enabled:          true,
		NFSVersion:       "nfs4.2",
		Protocol:         "rdma",
		MountOptions:     []string{"hard", "timeo=600", "retrans=2"},
		AttrCacheTimeout: 60 * time.Second,
		DirCacheTimeout:  60 * time.Second,
		ReadAheadSize:    1024 * 1024,   // 1MB
		WriteBufferSize:  1024 * 1024,   // 1MB
		MaxReadSize:      1024 * 1024,   // 1MB
		MaxWriteSize:     1024 * 1024,   // 1MB
		SecurityFlavor:   "sys",
	}
}

// NFSRDMAMount NFS RDMA 挂载
type NFSRDMAMount struct {
	mu sync.RWMutex

	config   *NFSRDMAConfig
	endpoint *RDMAEndpoint

	// 挂载状态
	mounted bool
	mountID string

	// 统计
	stats NFSRDMAStats
}

// NFSRDMAStats NFS RDMA 统计信息
type NFSRDMAStats struct {
	ReadOps      uint64 `json:"readOps"`
	WriteOps     uint64 `json:"writeOps"`
	BytesRead    uint64 `json:"bytesRead"`
	BytesWritten uint64 `json:"bytesWritten"`
	CacheHits    uint64 `json:"cacheHits"`
	CacheMisses  uint64 `json:"cacheMisses"`
	Errors       uint64 `json:"errors"`
}

// NewNFSRDMAMount 创建 NFS RDMA 挂载
func NewNFSRDMAMount(config *NFSRDMAConfig) (*NFSRDMAMount, error) {
	if config == nil {
		config = DefaultNFSRDMAConfig()
	}

	return &NFSRDMAMount{
		config:  config,
		mountID: generateMountID(),
	}, nil
}

// Mount 挂载 NFS 导出
func (m *NFSRDMAMount) Mount(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.mounted {
		return nil
	}

	// 创建 RDMA 端点
	rdmaConfig := DefaultRDMAConfig()
	rdmaConfig.Transport = "NFSoRDMA"
	rdmaConfig.Port = 20049 // NFS RDMA 端口

	endpoint, err := NewRDMAEndpoint(rdmaConfig)
	if err != nil {
		return fmt.Errorf("failed to create RDMA endpoint: %w", err)
	}

	// 连接
	addr := fmt.Sprintf("%s:20049", m.config.ServerAddr)
	if err := endpoint.Connect(ctx, addr); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	m.endpoint = endpoint
	m.mounted = true

	return nil
}

// Unmount 卸载
func (m *NFSRDMAMount) Unmount() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.mounted {
		return nil
	}

	if m.endpoint != nil {
		_ = m.endpoint.Disconnect()
		m.endpoint = nil
	}

	m.mounted = false
	return nil
}

// ReadFile 读取文件
func (m *NFSRDMAMount) ReadFile(ctx context.Context, path string, buf []byte, offset int64) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.mounted || m.endpoint == nil {
		return 0, ErrNotConnected
	}

	// 通过 RDMA 执行 NFS 读操作
	// 实际实现需要封装 NFS RPC 并通过 RDMA 发送
	n, err := m.endpoint.Receive(ctx, buf)
	if err != nil {
		m.stats.Errors++
		return 0, err
	}

	m.stats.ReadOps++
	m.stats.BytesRead += uint64(n)

	return n, nil
}

// WriteFile 写入文件
func (m *NFSRDMAMount) WriteFile(ctx context.Context, path string, data []byte, offset int64) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.mounted || m.endpoint == nil {
		return 0, ErrNotConnected
	}

	// 通过 RDMA 执行 NFS 写操作
	err := m.endpoint.Send(ctx, data)
	if err != nil {
		m.stats.Errors++
		return 0, err
	}

	m.stats.WriteOps++
	m.stats.BytesWritten += uint64(len(data))

	return len(data), nil
}

// GetStats 获取统计信息
func (m *NFSRDMAMount) GetStats() NFSRDMAStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// IsMounted 检查是否已挂载
func (m *NFSRDMAMount) IsMounted() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mounted
}

// ========== RDMA 性能监控 ==========

// RDMAPerfMonitor RDMA 性能监控
type RDMAPerfMonitor struct {
	mu sync.RWMutex

	// 采样间隔
	sampleInterval time.Duration

	// 历史数据
	history []PerfSample
	maxHistory int

	// 停止信号
	stopCh chan struct{}
}

// PerfSample 性能采样
type PerfSample struct {
	Timestamp    time.Time `json:"timestamp"`
	ThroughputMB float64   `json:"throughputMB"` // MB/s
	IOPS         float64   `json:"iops"`
	LatencyUs    float64   `json:"latencyUs"`    // 微秒
	CPUUsage     float64   `json:"cpuUsage"`     // 百分比
	MemoryMB     float64   `json:"memoryMB"`
}

// NewRDMAPerfMonitor 创建性能监控
func NewRDMAPerfMonitor(sampleInterval time.Duration, maxHistory int) *RDMAPerfMonitor {
	if sampleInterval <= 0 {
		sampleInterval = time.Second
	}
	if maxHistory <= 0 {
		maxHistory = 3600 // 1小时历史
	}

	return &RDMAPerfMonitor{
		sampleInterval: sampleInterval,
		history:        make([]PerfSample, 0, maxHistory),
		maxHistory:     maxHistory,
		stopCh:         make(chan struct{}),
	}
}

// Start 开始监控
func (m *RDMAPerfMonitor) Start(manager *RDMAManager) {
	ticker := time.NewTicker(m.sampleInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			sample := m.collectSample(manager)
			m.addSample(sample)
		}
	}
}

// Stop 停止监控
func (m *RDMAPerfMonitor) Stop() {
	close(m.stopCh)
}

// collectSample 收集采样数据
func (m *RDMAPerfMonitor) collectSample(manager *RDMAManager) PerfSample {
	stats := manager.GetStats()

	var totalBytes uint64
	var totalOps uint64
	for _, s := range stats {
		totalBytes += s.BytesSent + s.BytesReceived
		totalOps += s.OpsSent + s.OpsReceived
	}

	return PerfSample{
		Timestamp:    time.Now(),
		ThroughputMB: float64(totalBytes) / 1024 / 1024,
		IOPS:         float64(totalOps),
	}
}

// addSample 添加采样
func (m *RDMAPerfMonitor) addSample(sample PerfSample) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.history = append(m.history, sample)
	if len(m.history) > m.maxHistory {
		m.history = m.history[1:]
	}
}

// GetHistory 获取历史数据
func (m *RDMAPerfMonitor) GetHistory() []PerfSample {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]PerfSample, len(m.history))
	copy(result, m.history)
	return result
}

// GetLatest 获取最新采样
func (m *RDMAPerfMonitor) GetLatest() (PerfSample, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.history) == 0 {
		return PerfSample{}, false
	}
	return m.history[len(m.history)-1], true
}

// ========== 辅助函数 ==========

// generateSessionID 生成会话 ID
func generateSessionID() string {
	return fmt.Sprintf("iser-%d", time.Now().UnixNano())
}

// generateMountID 生成挂载 ID
func generateMountID() string {
	return fmt.Sprintf("nfsrdma-%d", time.Now().UnixNano())
}

// CheckRDMAAvailable 检查系统是否支持 RDMA
func CheckRDMAAvailable() (bool, error) {
	// 实际实现需要检查:
	// 1. /sys/class/infiniband 目录是否存在
	// 2. 内核模块是否加载 (ib_core, rdma_cm, etc.)
	// 3. 是否有 RDMA 网卡

	return true, nil
}

// GetRDMAStats 获取 RDMA 统计信息
func GetRDMAStats(deviceName string) (map[string]interface{}, error) {
	// 实际实现需要读取 /sys/class/infiniband/<device>/statistics/
	stats := map[string]interface{}{
		"portRcvData":        0,
		"portXmitData":       0,
		"portRcvPackets":     0,
		"portXmitPackets":    0,
		"portRcvErrors":      0,
		"portXmitDiscards":   0,
		"portRcvConstraintErrors": 0,
		"portXmitConstraintErrors": 0,
		"linkDowned":         0,
		"symbolError":        0,
	}

	return stats, nil
}

// Common errors
var (
	ErrInvalidConfig   = errors.New("invalid configuration")
	ErrSessionNotFound = errors.New("session not found")
	ErrMountNotFound   = errors.New("mount not found")
)