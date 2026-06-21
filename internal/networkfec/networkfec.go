package networkfec

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
)

// NewFECManager 创建 FEC 管理器
func NewFECManager(config *FECConfig) *FECManager {
	if config == nil {
		config = DefaultFECConfig()
	}

	return &FECManager{
		config:     config,
		interfaces: make(map[string]*FECInterface),
		stats: &FECStats{
			LastUpdated: time.Now(),
		},
		stopCh: make(chan struct{}),
	}
}

// Start 启动 FEC 管理器
func (m *FECManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("FEC 管理器已在运行")
	}

	m.running = true
	log.Println("[NetworkFEC] 前向纠错管理器启动")

	// 启动统计更新器
	go m.statsUpdater()

	return nil
}

// Stop 停止 FEC 管理器
func (m *FECManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopCh)
	m.running = false
	log.Println("[NetworkFEC] 前向纠错管理器停止")
}

// IsRunning 检查是否运行中
func (m *FECManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// AddInterface 添加网络接口
func (m *FECManager) AddInterface(name, ipAddress string, mode FECMode) (*FECInterface, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	iface := &FECInterface{
		ID:           uuid.New().String(),
		Name:         name,
		IPAddress:    ipAddress,
		FECMode:      mode,
		DataShards:   m.config.DataShards,
		ParityShards: m.config.ParityShards,
		Enabled:      true,
		Status:       "up",
		Stats: &FECStats{
			LastUpdated: time.Now(),
		},
	}

	m.interfaces[iface.ID] = iface
	log.Printf("[NetworkFEC] 添加接口: %s (%s) - %s", name, ipAddress, mode)

	return iface, nil
}

// RemoveInterface 移除网络接口
func (m *FECManager) RemoveInterface(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.interfaces[id]; !exists {
		return fmt.Errorf("接口不存在: %s", id)
	}

	delete(m.interfaces, id)
	log.Printf("[NetworkFEC] 移除接口: %s", id)

	return nil
}

// GetInterface 获取接口信息
func (m *FECManager) GetInterface(id string) (*FECInterface, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	iface, exists := m.interfaces[id]
	if !exists {
		return nil, fmt.Errorf("接口不存在: %s", id)
	}

	return iface, nil
}

// ListInterfaces 列出所有接口
func (m *FECManager) ListInterfaces() []*FECInterface {
	m.mu.RLock()
	defer m.mu.RUnlock()

	interfaces := make([]*FECInterface, 0, len(m.interfaces))
	for _, iface := range m.interfaces {
		interfaces = append(interfaces, iface)
	}
	return interfaces
}

// UpdateInterfaceFEC 更新接口 FEC 配置
func (m *FECManager) UpdateInterfaceFEC(id string, mode FECMode, dataShards, parityShards int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	iface, exists := m.interfaces[id]
	if !exists {
		return fmt.Errorf("接口不存在: %s", id)
	}

	iface.FECMode = mode
	iface.DataShards = dataShards
	iface.ParityShards = parityShards

	log.Printf("[NetworkFEC] 更新接口 FEC 配置: %s - %s (%d+%d)", id, mode, dataShards, parityShards)

	return nil
}

// Encode 编码数据
func (m *FECManager) Encode(data []byte) ([]*FECPacket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running {
		return nil, fmt.Errorf("FEC 管理器未运行")
	}

	encoder := &FECEncoder{
		dataShards:   m.config.DataShards,
		parityShards: m.config.ParityShards,
		mode:         m.config.Mode,
	}

	packets, err := encoder.Encode(data)
	if err != nil {
		return nil, err
	}

	// 更新统计
	m.stats.mu.Lock()
	m.stats.TotalPackets += int64(len(packets))
	m.stats.LastUpdated = time.Now()
	m.stats.mu.Unlock()

	return packets, nil
}

// Decode 解码数据
func (m *FECManager) Decode(packets []*FECPacket) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running {
		return nil, fmt.Errorf("FEC 管理器未运行")
	}

	decoder := &FECDecoder{
		dataShards:   m.config.DataShards,
		parityShards: m.config.ParityShards,
		mode:         m.config.Mode,
	}

	data, err := decoder.Decode(packets)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// GetStats 获取统计信息
func (m *FECManager) GetStats() *FECStats {
	m.stats.mu.RLock()
	defer m.stats.mu.RUnlock()

	return &FECStats{
		TotalPackets:     m.stats.TotalPackets,
		RecoveredPackets: m.stats.RecoveredPackets,
		LostPackets:      m.stats.LostPackets,
		RecoveryRate:     m.stats.RecoveryRate,
		Overhead:         m.stats.Overhead,
		LastUpdated:      m.stats.LastUpdated,
	}
}

// GetConfig 获取配置
func (m *FECManager) GetConfig() *FECConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.config
}

// UpdateConfig 更新配置
func (m *FECManager) UpdateConfig(config *FECConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	log.Printf("[NetworkFEC] 配置已更新")
}

// statsUpdater 统计更新器
func (m *FECManager) statsUpdater() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.updateStats()
		}
	}
}

// updateStats 更新统计信息
func (m *FECManager) updateStats() {
	m.stats.mu.Lock()
	defer m.stats.mu.Unlock()

	// 计算恢复率
	if m.stats.TotalPackets > 0 {
		m.stats.RecoveryRate = float64(m.stats.RecoveredPackets) / float64(m.stats.TotalPackets) * 100
	}

	// 计算开销
	totalDataPackets := m.stats.TotalPackets - m.stats.RecoveredPackets
	if totalDataPackets > 0 {
		m.stats.Overhead = float64(m.stats.RecoveredPackets) / float64(totalDataPackets) * 100
	}

	m.stats.LastUpdated = time.Now()
}

// Encode 实现 FEC 编码
func (e *FECEncoder) Encode(data []byte) ([]*FECPacket, error) {
	// 计算分片大小
	shardSize := (len(data) + e.dataShards - 1) / e.dataShards

	// 创建数据分片
	dataShards := make([][]byte, e.dataShards)
	for i := 0; i < e.dataShards; i++ {
		start := i * shardSize
		end := start + shardSize
		if end > len(data) {
			end = len(data)
		}
		dataShards[i] = data[start:end]
	}

	// 创建校验分片
	parityShards := make([][]byte, e.parityShards)
	for i := 0; i < e.parityShards; i++ {
		parityShards[i] = make([]byte, shardSize)
	}

	// 计算校验和（简化实现，使用 XOR）
	for i := 0; i < e.parityShards; i++ {
		for j := 0; j < shardSize; j++ {
			var parity byte
			for k := 0; k < e.dataShards; k++ {
				if j < len(dataShards[k]) {
					parity ^= dataShards[k][j]
				}
			}
			parityShards[i][j] = parity
		}
	}

	// 创建数据包
	packets := make([]*FECPacket, 0, e.dataShards+e.parityShards)
	seqNum := uint32(time.Now().UnixNano())

	// 数据包
	for i, shard := range dataShards {
		checksum := calculateChecksum(shard)
		packets = append(packets, &FECPacket{
			SequenceNum: seqNum,
			ShardIndex:  i,
			IsParity:    false,
			Data:        shard,
			Checksum:    checksum,
		})
	}

	// 校验包
	for i, shard := range parityShards {
		checksum := calculateChecksum(shard)
		packets = append(packets, &FECPacket{
			SequenceNum: seqNum,
			ShardIndex:  e.dataShards + i,
			IsParity:    true,
			Data:        shard,
			Checksum:    checksum,
		})
	}

	return packets, nil
}

// Decode 实现 FEC 解码
func (d *FECDecoder) Decode(packets []*FECPacket) ([]byte, error) {
	if len(packets) < d.dataShards {
		return nil, fmt.Errorf("数据包不足: 需要 %d, 收到 %d", d.dataShards, len(packets))
	}

	// 按分片索引排序
	sortedPackets := make([]*FECPacket, len(packets))
	copy(sortedPackets, packets)

	// 简化实现：假设没有丢包
	dataPackets := make([][]byte, d.dataShards)
	for _, packet := range sortedPackets {
		if !packet.IsParity && packet.ShardIndex < d.dataShards {
			dataPackets[packet.ShardIndex] = packet.Data
		}
	}

	// 合并数据
	var result []byte
	for _, shard := range dataPackets {
		if shard != nil {
			result = append(result, shard...)
		}
	}

	return result, nil
}

// calculateChecksum 计算校验和
func calculateChecksum(data []byte) uint32 {
	hash := sha256.Sum256(data)
	return binary.BigEndian.Uint32(hash[:4])
}

// RecoverPacket 恢复丢失的数据包
func (m *FECManager) RecoverPacket(packets []*FECPacket, lostIndex int) (*FECPacket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.running {
		return nil, fmt.Errorf("FEC 管理器未运行")
	}

	// 检查是否有足够的包进行恢复
	availableData := 0
	for _, p := range packets {
		if !p.IsParity {
			availableData++
		}
	}

	if availableData < m.config.DataShards-1 {
		return nil, fmt.Errorf("无法恢复：数据包不足")
	}

	// 使用校验分片恢复
	recovered := &FECPacket{
		SequenceNum: packets[0].SequenceNum,
		ShardIndex:  lostIndex,
		IsParity:    false,
		Data:        make([]byte, len(packets[0].Data)),
	}

	// XOR 恢复
	for _, p := range packets {
		if p.IsParity {
			for i := 0; i < len(recovered.Data) && i < len(p.Data); i++ {
				recovered.Data[i] ^= p.Data[i]
			}
		} else if p.ShardIndex != lostIndex {
			for i := 0; i < len(recovered.Data) && i < len(p.Data); i++ {
				recovered.Data[i] ^= p.Data[i]
			}
		}
	}

	recovered.Checksum = calculateChecksum(recovered.Data)

	// 更新统计
	m.stats.mu.Lock()
	m.stats.RecoveredPackets++
	m.stats.LastUpdated = time.Now()
	m.stats.mu.Unlock()

	log.Printf("[NetworkFEC] 成功恢复数据包: %d", lostIndex)

	return recovered, nil
}

// GetInterfaceStats 获取接口统计
func (m *FECManager) GetInterfaceStats(id string) (*FECStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	iface, exists := m.interfaces[id]
	if !exists {
		return nil, fmt.Errorf("接口不存在: %s", id)
	}

	return iface.Stats, nil
}

// ResetStats 重置统计信息
func (m *FECManager) ResetStats() {
	m.stats.mu.Lock()
	defer m.stats.mu.Unlock()

	m.stats.TotalPackets = 0
	m.stats.RecoveredPackets = 0
	m.stats.LostPackets = 0
	m.stats.RecoveryRate = 0
	m.stats.Overhead = 0
	m.stats.LastUpdated = time.Now()

	log.Println("[NetworkFEC] 统计信息已重置")
}
