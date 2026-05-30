// Package airplay 提供 AirPlay 音视频投射服务功能
// 核心管理器 - 设备发现、流管理、多房间音频
package airplay

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager AirPlay 服务管理器
type Manager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	running   bool
	startedAt *time.Time

	// 设备管理
	devices    map[string]*AirPlayDevice
	receiver   *AirPlayReceiver
	sender     *AirPlaySender

	// 流管理
	audioStreams map[string]*AudioStream
	videoStreams map[string]*VideoStream
	mirrors      map[string]*ScreenMirror

	// 多房间音频
	groups map[string]*MultiRoomGroup

	// 配对管理
	pairings []PairingRecord

	// 统计
	stats AirPlayStats
}

// NewManager 创建 AirPlay 管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:       logger,
		devices:      make(map[string]*AirPlayDevice),
		audioStreams:  make(map[string]*AudioStream),
		videoStreams:  make(map[string]*VideoStream),
		mirrors:      make(map[string]*ScreenMirror),
		groups:       make(map[string]*MultiRoomGroup),
		pairings:     make([]PairingRecord, 0),
	}

	// 初始化默认接收器
	m.receiver = &AirPlayReceiver{
		ID:      "nas-receiver",
		Name:    "NAS AirPlay Receiver",
		Enabled: true,
		Port:    7000,
	}

	// 初始化发送器
	m.sender = &AirPlaySender{
		ID:     "nas-sender",
		Status: SenderStatusIdle,
	}

	return m
}

// ========== 服务生命周期 ==========

// Start 启动 AirPlay 服务
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("AirPlay 服务已在运行")
	}

	now := time.Now()
	m.startedAt = &now
	m.running = true

	m.logger.Info("AirPlay 服务已启动")
	return nil
}

// Stop 停止 AirPlay 服务
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("AirPlay 服务未运行")
	}

	// 停止所有活跃流
	for id := range m.audioStreams {
		m.audioStreams[id].Status = StreamStatusStopped
	}
	for id := range m.videoStreams {
		m.videoStreams[id].Status = StreamStatusStopped
	}
	for id := range m.mirrors {
		m.mirrors[id].Active = false
	}

	m.running = false
	m.startedAt = nil

	m.logger.Info("AirPlay 服务已停止")
	return nil
}

// GetStatus 获取服务状态
func (m *Manager) GetStatus() ServiceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeStreams := 0
	for _, s := range m.audioStreams {
		if s.Status == StreamStatusPlaying {
			activeStreams++
		}
	}
	for _, s := range m.videoStreams {
		if s.Status == StreamStatusPlaying {
			activeStreams++
		}
	}

	return ServiceStatus{
		Running:   m.running,
		StartedAt: m.startedAt,
		Devices:   len(m.devices),
		Streams:   activeStreams,
	}
}

// ========== 设备管理 ==========

// ListDevices 列出所有设备
func (m *Manager) ListDevices() []AirPlayDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]AirPlayDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, *d)
	}
	return devices
}

// GetDevice 获取设备详情
func (m *Manager) GetDevice(id string) (*AirPlayDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, ok := m.devices[id]
	if !ok {
		return nil, fmt.Errorf("设备 %s 不存在", id)
	}
	return device, nil
}

// RefreshDevices 刷新设备列表 (模拟 mDNS 发现)
func (m *Manager) RefreshDevices() []AirPlayDevice {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟发现一些设备
	mockDevices := []*AirPlayDevice{
		{
			ID:   "appletv-living",
			Name: "客厅 Apple TV",
			Type: DeviceTypeAppleTV,
			IP:   "192.168.1.100",
			Port: 7000,
			Online: true,
			Capabilities: DeviceCapabilities{Audio: true, Video: true, Screen: true},
			LastSeen: time.Now(),
		},
		{
			ID:   "homepod-kitchen",
			Name: "厨房 HomePod",
			Type: DeviceTypeHomePod,
			IP:   "192.168.1.101",
			Port: 7000,
			Online: true,
			Capabilities: DeviceCapabilities{Audio: true, Video: false, Screen: false},
			LastSeen: time.Now(),
		},
		{
			ID:   "speaker-bedroom",
			Name: "卧室音箱",
			Type: DeviceTypeSpeaker,
			IP:   "192.168.1.102",
			Port: 7000,
			Online: true,
			Capabilities: DeviceCapabilities{Audio: true, Video: false, Screen: false},
			LastSeen: time.Now(),
		},
	}

	for _, d := range mockDevices {
		m.devices[d.ID] = d
	}

	m.logger.Info("设备列表已刷新", zap.Int("count", len(m.devices)))

	devices := make([]AirPlayDevice, 0, len(m.devices))
	for _, d := range m.devices {
		devices = append(devices, *d)
	}
	return devices
}

// ========== 接收器管理 ==========

// GetReceiver 获取接收器配置
func (m *Manager) GetReceiver() *AirPlayReceiver {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.receiver
}

// UpdateReceiver 更新接收器配置
func (m *Manager) UpdateReceiver(name string, enabled bool, port int, passwordProtected bool, password string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name != "" {
		m.receiver.Name = name
	}
	m.receiver.Enabled = enabled
	if port > 0 {
		m.receiver.Port = port
	}
	m.receiver.PasswordProtected = passwordProtected
	if passwordProtected && password != "" {
		m.receiver.Password = password
	} else {
		m.receiver.Password = ""
	}

	m.logger.Info("接收器配置已更新", zap.String("name", m.receiver.Name))
}

// ========== 发送器管理 ==========

// GetSender 获取发送器状态
func (m *Manager) GetSender() *AirPlaySender {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sender
}

// Cast 发起投射
func (m *Manager) Cast(targetID string, media *MediaInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[targetID]
	if !ok {
		return fmt.Errorf("目标设备 %s 不存在", targetID)
	}

	if !device.Online {
		return fmt.Errorf("目标设备 %s 不在线", targetID)
	}

	m.sender.TargetID = targetID
	m.sender.TargetName = device.Name
	m.sender.Status = SenderStatusCasting
	m.sender.MediaInfo = media

	// 创建音频流
	if device.Capabilities.Audio {
		streamID := fmt.Sprintf("audio-%s-%d", targetID, time.Now().UnixMilli())
		m.audioStreams[streamID] = &AudioStream{
			ID:       streamID,
			DeviceID: targetID,
			Status:   StreamStatusPlaying,
			Volume:   50,
			CurrentTrack: media,
			Queue:    []MediaInfo{},
		}
	}

	m.logger.Info("投射已开始", zap.String("target", device.Name))
	return nil
}

// StopCast 停止投射
func (m *Manager) StopCast() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sender.Status != SenderStatusCasting {
		return fmt.Errorf("当前未在投射")
	}

	// 停止相关音频流
	for id, s := range m.audioStreams {
		if s.DeviceID == m.sender.TargetID {
			s.Status = StreamStatusStopped
			delete(m.audioStreams, id)
		}
	}

	m.sender.TargetID = ""
	m.sender.TargetName = ""
	m.sender.Status = SenderStatusIdle
	m.sender.MediaInfo = nil

	m.logger.Info("投射已停止")
	return nil
}

// ========== 音频流管理 ==========

// GetAudioQueue 获取播放队列
func (m *Manager) GetAudioQueue(streamID string) ([]MediaInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stream, ok := m.audioStreams[streamID]
	if !ok {
		return nil, fmt.Errorf("音频流 %s 不存在", streamID)
	}
	return stream.Queue, nil
}

// AddToQueue 添加到播放队列
func (m *Manager) AddToQueue(streamID string, media MediaInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, ok := m.audioStreams[streamID]
	if !ok {
		return fmt.Errorf("音频流 %s 不存在", streamID)
	}

	stream.Queue = append(stream.Queue, media)
	m.logger.Info("已添加到队列", zap.String("title", media.Title))
	return nil
}

// PlayAudio 播放音频
func (m *Manager) PlayAudio(streamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, ok := m.audioStreams[streamID]
	if !ok {
		return fmt.Errorf("音频流 %s 不存在", streamID)
	}

	stream.Status = StreamStatusPlaying
	return nil
}

// PauseAudio 暂停音频
func (m *Manager) PauseAudio(streamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, ok := m.audioStreams[streamID]
	if !ok {
		return fmt.Errorf("音频流 %s 不存在", streamID)
	}

	stream.Status = StreamStatusPaused
	return nil
}

// NextTrack 下一曲
func (m *Manager) NextTrack(streamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, ok := m.audioStreams[streamID]
	if !ok {
		return fmt.Errorf("音频流 %s 不存在", streamID)
	}

	if stream.QueueIndex < len(stream.Queue)-1 {
		stream.QueueIndex++
		stream.CurrentTrack = &stream.Queue[stream.QueueIndex]
	}
	return nil
}

// PrevTrack 上一曲
func (m *Manager) PrevTrack(streamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, ok := m.audioStreams[streamID]
	if !ok {
		return fmt.Errorf("音频流 %s 不存在", streamID)
	}

	if stream.QueueIndex > 0 {
		stream.QueueIndex--
		stream.CurrentTrack = &stream.Queue[stream.QueueIndex]
	}
	return nil
}

// SetVolume 设置音量
func (m *Manager) SetVolume(streamID string, volume int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if volume < 0 || volume > 100 {
		return fmt.Errorf("音量必须在 0-100 之间")
	}

	stream, ok := m.audioStreams[streamID]
	if !ok {
		return fmt.Errorf("音频流 %s 不存在", streamID)
	}

	stream.Volume = volume
	return nil
}

// ========== 视频流管理 ==========

// CastVideo 视频投射
func (m *Manager) CastVideo(targetID string, media *MediaInfo, resolution string, bitrate int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[targetID]
	if !ok {
		return fmt.Errorf("目标设备 %s 不存在", targetID)
	}

	if !device.Capabilities.Video {
		return fmt.Errorf("设备 %s 不支持视频", targetID)
	}

	streamID := fmt.Sprintf("video-%s-%d", targetID, time.Now().UnixMilli())
	m.videoStreams[streamID] = &VideoStream{
		ID:         streamID,
		DeviceID:   targetID,
		Status:     StreamStatusPlaying,
		Resolution: resolution,
		Bitrate:    bitrate,
		Media:      media,
	}

	m.logger.Info("视频投射已开始", zap.String("target", device.Name))
	return nil
}

// StopVideo 停止视频投射
func (m *Manager) StopVideo(streamID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	stream, ok := m.videoStreams[streamID]
	if !ok {
		return fmt.Errorf("视频流 %s 不存在", streamID)
	}

	stream.Status = StreamStatusStopped
	delete(m.videoStreams, streamID)
	return nil
}

// ========== 屏幕镜像 ==========

// StartMirror 开始屏幕镜像
func (m *Manager) StartMirror(sourceID, targetID, resolution string, frameRate int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	target, ok := m.devices[targetID]
	if !ok {
		return fmt.Errorf("目标设备 %s 不存在", targetID)
	}

	if !target.Capabilities.Screen {
		return fmt.Errorf("设备 %s 不支持屏幕镜像", targetID)
	}

	mirrorID := fmt.Sprintf("mirror-%s-%d", targetID, time.Now().UnixMilli())
	m.mirrors[mirrorID] = &ScreenMirror{
		ID:           mirrorID,
		SourceDevice: sourceID,
		TargetDevice: targetID,
		Resolution:   resolution,
		FrameRate:    frameRate,
		Latency:      50, // 默认 50ms 延迟
		Active:       true,
	}

	m.logger.Info("屏幕镜像已开始", zap.String("target", target.Name))
	return nil
}

// StopMirror 停止屏幕镜像
func (m *Manager) StopMirror(mirrorID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mirror, ok := m.mirrors[mirrorID]
	if !ok {
		return fmt.Errorf("镜像 %s 不存在", mirrorID)
	}

	mirror.Active = false
	delete(m.mirrors, mirrorID)
	return nil
}

// ========== 多房间音频 ==========

// ListGroups 列出多房间组
func (m *Manager) ListGroups() []MultiRoomGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]MultiRoomGroup, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, *g)
	}
	return groups
}

// CreateGroup 创建多房间组
func (m *Manager) CreateGroup(name, masterID string, slaveIDs []string) (*MultiRoomGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证主设备存在
	if _, ok := m.devices[masterID]; !ok {
		return nil, fmt.Errorf("主设备 %s 不存在", masterID)
	}

	// 验证从设备存在
	for _, slaveID := range slaveIDs {
		if _, ok := m.devices[slaveID]; !ok {
			return nil, fmt.Errorf("从设备 %s 不存在", slaveID)
		}
	}

	groupID := fmt.Sprintf("group-%d", time.Now().UnixMilli())
	group := &MultiRoomGroup{
		ID:         groupID,
		Name:       name,
		MasterID:   masterID,
		SlaveIDs:   slaveIDs,
		SyncStatus: SyncStatusSynced,
	}

	m.groups[groupID] = group
	m.logger.Info("多房间组已创建", zap.String("name", name))

	return group, nil
}

// UpdateGroup 更新多房间组
func (m *Manager) UpdateGroup(groupID, name string, slaveIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, ok := m.groups[groupID]
	if !ok {
		return fmt.Errorf("组 %s 不存在", groupID)
	}

	if name != "" {
		group.Name = name
	}
	if slaveIDs != nil {
		// 验证从设备存在
		for _, slaveID := range slaveIDs {
			if _, ok := m.devices[slaveID]; !ok {
				return fmt.Errorf("从设备 %s 不存在", slaveID)
			}
		}
		group.SlaveIDs = slaveIDs
	}

	group.SyncStatus = SyncStatusSyncing
	return nil
}

// DeleteGroup 删除多房间组
func (m *Manager) DeleteGroup(groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.groups[groupID]; !ok {
		return fmt.Errorf("组 %s 不存在", groupID)
	}

	delete(m.groups, groupID)
	m.logger.Info("多房间组已删除", zap.String("id", groupID))
	return nil
}

// ========== 设备配对 ==========

// ListPairings 列出配对设备
func (m *Manager) ListPairings() []PairingRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pairings
}

// TrustDevice 信任设备
func (m *Manager) TrustDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, p := range m.pairings {
		if p.DeviceID == deviceID {
			m.pairings[i].Trusted = true
			m.logger.Info("设备已信任", zap.String("deviceID", deviceID))
			return nil
		}
	}

	// 如果未配对，自动添加配对记录
	m.pairings = append(m.pairings, PairingRecord{
		DeviceID: deviceID,
		Name:     deviceID,
		PairedAt: time.Now(),
		Trusted:  true,
	})
	return nil
}

// UnpairDevice 取消配对
func (m *Manager) UnpairDevice(deviceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, p := range m.pairings {
		if p.DeviceID == deviceID {
			m.pairings = append(m.pairings[:i], m.pairings[i+1:]...)
			m.logger.Info("设备已取消配对", zap.String("deviceID", deviceID))
			return nil
		}
	}

	return fmt.Errorf("设备 %s 未配对", deviceID)
}

// ========== 统计 ==========

// GetStats 获取统计信息
func (m *Manager) GetStats() AirPlayStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	activeStreams := 0
	for _, s := range m.audioStreams {
		if s.Status == StreamStatusPlaying {
			activeStreams++
		}
	}
	for _, s := range m.videoStreams {
		if s.Status == StreamStatusPlaying {
			activeStreams++
		}
	}

	return AirPlayStats{
		SenderCount:   1,
		ReceiverCount: len(m.devices),
		ActiveStreams: activeStreams,
		TotalTraffic:  0, // 实际实现中应跟踪流量
	}
}
