// Package smartcam 提供摄像头管理核心业务逻辑.
package smartcam

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 摄像头管理器.
type Manager struct {
	mu           sync.RWMutex
	cameras      map[string]*Camera
	recordings   []*Recording
	motionEvents []*MotionEvent
	activeRecord map[string]bool // cameraID -> is recording
	config       SystemConfig
	startTime    time.Time
	logger       *zap.Logger
	stopCh       chan struct{}
}

// SystemConfig 系统配置.
type SystemConfig struct {
	StoragePath    string `json:"storagePath"`
	MaxStorageGB   int    `json:"maxStorageGB"`
	AutoDiscovery  bool   `json:"autoDiscovery"`
	DiscoveryPorts []int  `json:"discoveryPorts"`
	StreamTimeout  int    `json:"streamTimeout"` // 秒
	WebhookURL     string `json:"webhookUrl"`
	MaxCameras     int    `json:"maxCameras"`
	RetentionDays  int    `json:"retentionDays"`
}

// NewManager 创建摄像头管理器.
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Manager{
		cameras:      make(map[string]*Camera),
		recordings:   make([]*Recording, 0),
		motionEvents: make([]*MotionEvent, 0),
		activeRecord: make(map[string]bool),
		config: SystemConfig{
			StoragePath:    "/volume1/surveillance",
			MaxStorageGB:   500,
			AutoDiscovery:  true,
			DiscoveryPorts: []int{80, 554, 8080, 8888},
			StreamTimeout:  30,
			MaxCameras:     64,
			RetentionDays:  30,
		},
		startTime: time.Now(),
		logger:    logger,
		stopCh:    make(chan struct{}),
	}
}

// ========== 摄像头管理 ==========

// AddCamera 添加摄像头.
func (m *Manager) AddCamera(req AddCameraRequest) (*Camera, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查数量限制
	if len(m.cameras) >= m.config.MaxCameras {
		return nil, fmt.Errorf("摄像头数量已达上限 (%d)", m.config.MaxCameras)
	}

	// 检查 IP 是否已存在
	for _, cam := range m.cameras {
		if cam.IPAddress == req.IPAddress && cam.Port == req.Port {
			return nil, fmt.Errorf("摄像头 %s:%d 已存在 (ID: %s)", req.IPAddress, req.Port, cam.ID)
		}
	}

	// 设置默认值
	protocol := req.Protocol
	if protocol == "" {
		protocol = ProtocolRTSP
	}
	port := req.Port
	if port == 0 {
		port = 554
	}
	frameRate := req.FrameRate
	if frameRate == 0 {
		frameRate = 25
	}
	resolution := req.Resolution
	if resolution == "" {
		resolution = "1080p"
	}

	now := time.Now()
	camera := &Camera{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Location:   req.Location,
		IPAddress:  req.IPAddress,
		Port:       port,
		Protocol:   protocol,
		StreamURL:  fmt.Sprintf("%s://%s:%d/stream", protocol, req.IPAddress, port),
		Username:   req.Username,
		Model:      "Unknown",
		Resolution: resolution,
		FrameRate:  frameRate,
		Status:     CameraStatusOnline,
		Enabled:    true,
		Tags:       req.Tags,
		CreatedAt:  now,
		UpdatedAt:  now,
		LastSeen:   &now,
	}

	// 设置录像配置
	if req.Recording != nil {
		camera.Recording = *req.Recording
	} else {
		camera.Recording = RecordingConfig{
			Enabled:       true,
			Mode:          RecordingModeMotion,
			Quality:       "medium",
			PreRecord:     5,
			PostRecord:    10,
			MaxDuration:   3600,
			RetentionDays: 30,
		}
	}

	// 设置移动侦测配置
	if req.Motion != nil {
		camera.Motion = *req.Motion
	} else {
		camera.Motion = MotionConfig{
			Enabled:     true,
			Sensitivity: SensitivityMedium,
			Threshold:   50,
			CooldownSec: 30,
			Actions: []MotionAction{
				{Type: "snapshot", Enabled: true},
				{Type: "record", Enabled: true},
			},
		}
	}

	m.cameras[camera.ID] = camera
	m.logger.Info("摄像头已添加", zap.String("id", camera.ID), zap.String("name", camera.Name), zap.String("ip", camera.IPAddress))

	return m.copyCamera(camera), nil
}

// GetCamera 获取摄像头信息.
func (m *Manager) GetCamera(id string) (*Camera, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	camera, ok := m.cameras[id]
	if !ok {
		return nil, fmt.Errorf("%s: %s", ErrCameraNotFound, id)
	}
	return m.copyCamera(camera), nil
}

// UpdateCamera 更新摄像头配置.
func (m *Manager) UpdateCamera(id string, req UpdateCameraRequest) (*Camera, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	camera, ok := m.cameras[id]
	if !ok {
		return nil, fmt.Errorf("%s: %s", ErrCameraNotFound, id)
	}

	if req.Name != nil {
		camera.Name = *req.Name
	}
	if req.Location != nil {
		camera.Location = *req.Location
	}
	if req.IPAddress != nil {
		camera.IPAddress = *req.IPAddress
		camera.StreamURL = fmt.Sprintf("%s://%s:%d/stream", camera.Protocol, camera.IPAddress, camera.Port)
	}
	if req.Port != nil {
		camera.Port = *req.Port
		camera.StreamURL = fmt.Sprintf("%s://%s:%d/stream", camera.Protocol, camera.IPAddress, camera.Port)
	}
	if req.Protocol != nil {
		camera.Protocol = *req.Protocol
		camera.StreamURL = fmt.Sprintf("%s://%s:%d/stream", camera.Protocol, camera.IPAddress, camera.Port)
	}
	if req.Username != nil {
		camera.Username = *req.Username
	}
	if req.Password != nil {
		camera.Password = *req.Password
	}
	if req.Resolution != nil {
		camera.Resolution = *req.Resolution
	}
	if req.FrameRate != nil {
		camera.FrameRate = *req.FrameRate
	}
	if req.Enabled != nil {
		camera.Enabled = *req.Enabled
		if !camera.Enabled {
			camera.Status = CameraStatusDisabled
		} else {
			camera.Status = CameraStatusOnline
		}
	}
	if req.Tags != nil {
		camera.Tags = req.Tags
	}
	camera.UpdatedAt = time.Now()

	m.logger.Info("摄像头配置已更新", zap.String("id", camera.ID), zap.String("name", camera.Name))
	return m.copyCamera(camera), nil
}

// RemoveCamera 移除摄像头.
func (m *Manager) RemoveCamera(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	camera, ok := m.cameras[id]
	if !ok {
		return fmt.Errorf("%s: %s", ErrCameraNotFound, id)
	}

	// 如果正在录像，先停止
	if m.activeRecord[id] {
		delete(m.activeRecord, id)
	}

	delete(m.cameras, id)
	m.logger.Info("摄像头已移除", zap.String("id", id), zap.String("name", camera.Name))
	return nil
}

// ListCameras 列出所有摄像头.
func (m *Manager) ListCameras() []*Camera {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cameras := make([]*Camera, 0, len(m.cameras))
	for _, cam := range m.cameras {
		cameras = append(cameras, m.copyCamera(cam))
	}
	return cameras
}

// ========== 摄像头发现 ==========

// DiscoverCameras 在局域网中发现摄像头.
func (m *Manager) DiscoverCameras(subnet string) (*DiscoverResult, error) {
	start := time.Now()

	// 解析子网
	_, ipNet, err := net.ParseCIDR(subnet)
	if err != nil {
		// 如果不是 CIDR 格式，尝试作为单个 IP
		ip := net.ParseIP(subnet)
		if ip == nil {
			return nil, fmt.Errorf("无效的子网或IP: %s", subnet)
		}
		ipNet = &net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)}
	}

	var discovered []Camera
	var mu sync.Mutex
	var wg sync.WaitGroup
	scannedCount := 0

	// 生成 IP 列表
	ips := m.expandIPRange(ipNet)

	// 限制并发数
	sem := make(chan struct{}, 50)

	for _, ip := range ips {
		for _, port := range m.config.DiscoveryPorts {
			wg.Add(1)
			sem <- struct{}{}
			go func(ipStr string, p int) {
				defer wg.Done()
				defer func() { <-sem }()

				addr := fmt.Sprintf("%s:%d", ipStr, p)
				conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
				if err != nil {
					return
				}
				conn.Close()
				//nolint:errcheck // Best effort close

				mu.Lock()
				scannedCount++
				// 开放端口，可能是摄像头
				cam := Camera{
					ID:         uuid.New().String(),
					IPAddress:  ipStr,
					Port:       p,
					Protocol:   ProtocolRTSP,
					StreamURL:  fmt.Sprintf("rtsp://%s:%d/stream", ipStr, p),
					Status:     CameraStatusOnline,
					Resolution: "1080p",
					FrameRate:  25,
					Enabled:    false, // 发现的摄像头默认不启用
				}
				discovered = append(discovered, cam)
				mu.Unlock()

				m.logger.Info("发现摄像头", zap.String("ip", ipStr), zap.Int("port", p))
			}(ip, port)
		}
	}

	wg.Wait()

	return &DiscoverResult{
		Found:      len(discovered),
		Cameras:    discovered,
		ScannedIPs: scannedCount,
		Elapsed:    time.Since(start).String(),
	}, nil
}

// expandIPRange 展开 IP 范围.
func (m *Manager) expandIPRange(ipNet *net.IPNet) []string {
	var ips []string
	for ip := ipNet.IP.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}
	// 去掉网络地址和广播地址
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}
	return ips
}

// inc 递增 IP.
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// ========== 录像管理 ==========

// StartRecording 开始录像.
func (m *Manager) StartRecording(cameraID string, mode string) (*Recording, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	camera, ok := m.cameras[cameraID]
	if !ok {
		return nil, fmt.Errorf("%s: %s", ErrCameraNotFound, cameraID)
	}

	if camera.Status != CameraStatusOnline {
		return nil, fmt.Errorf("%s: %s", ErrCameraOffline, cameraID)
	}

	if m.activeRecord[cameraID] {
		return nil, fmt.Errorf("%s: %s", ErrRecordingInProgress, cameraID)
	}

	recordingMode := RecordingModeManual
	if mode != "" {
		recordingMode = RecordingMode(mode)
	}

	now := time.Now()
	recording := &Recording{
		ID:         uuid.New().String(),
		CameraID:   cameraID,
		CameraName: camera.Name,
		StartTime:  now,
		Trigger:    string(recordingMode),
	}

	m.activeRecord[cameraID] = true
	m.recordings = append(m.recordings, recording)

	m.logger.Info("录像已开始",
		zap.String("recordingId", recording.ID),
		zap.String("cameraId", cameraID),
		zap.String("mode", mode),
	)

	return recording, nil
}

// StopRecording 停止录像.
func (m *Manager) StopRecording(cameraID string) (*Recording, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.activeRecord[cameraID] {
		return nil, fmt.Errorf("摄像头 %s 没有正在进行的录像", cameraID)
	}

	// 查找该摄像头的最新录像
	var recording *Recording
	for i := len(m.recordings) - 1; i >= 0; i-- {
		if m.recordings[i].CameraID == cameraID && m.recordings[i].EndTime.IsZero() {
			recording = m.recordings[i]
			break
		}
	}

	if recording == nil {
		return nil, fmt.Errorf("%s", ErrRecordingNotFound)
	}

	now := time.Now()
	recording.EndTime = now
	recording.Duration = now.Sub(recording.StartTime)
	recording.FileSize = int64(recording.Duration.Seconds()) * 1024 * 1024 // 模拟文件大小
	recording.FilePath = fmt.Sprintf("%s/%s/%s.mp4", m.config.StoragePath, cameraID, recording.ID)

	delete(m.activeRecord, cameraID)

	m.logger.Info("录像已停止",
		zap.String("recordingId", recording.ID),
		zap.Duration("duration", recording.Duration),
	)

	return recording, nil
}

// GetRecordings 获取录像列表.
func (m *Manager) GetRecordings(cameraID string, limit int) []*Recording {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Recording
	for i := len(m.recordings) - 1; i >= 0; i-- {
		rec := m.recordings[i]
		if cameraID != "" && rec.CameraID != cameraID {
			continue
		}
		result = append(result, rec)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// DeleteRecording 删除录像.
func (m *Manager) DeleteRecording(recordingID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, rec := range m.recordings {
		if rec.ID == recordingID {
			m.recordings = append(m.recordings[:i], m.recordings[i+1:]...)
			m.logger.Info("录像已删除", zap.String("recordingId", recordingID))
			return nil
		}
	}
	return fmt.Errorf("%s: %s", ErrRecordingNotFound, recordingID)
}

// ========== 移动侦测 ==========

// TriggerMotionEvent 触发移动侦测事件.
func (m *Manager) TriggerMotionEvent(cameraID, zoneID string, confidence float64, bbox *BoundingBox) (*MotionEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	camera, ok := m.cameras[cameraID]
	if !ok {
		return nil, fmt.Errorf("%s: %s", ErrCameraNotFound, cameraID)
	}

	if !camera.Motion.Enabled {
		return nil, fmt.Errorf("摄像头 %s 移动侦测未启用", cameraID)
	}

	// 检查冷却时间
	if len(m.motionEvents) > 0 {
		lastEvent := m.motionEvents[len(m.motionEvents)-1]
		if lastEvent.CameraID == cameraID {
			cooldown := time.Duration(camera.Motion.CooldownSec) * time.Second
			if time.Since(lastEvent.Timestamp) < cooldown {
				return nil, fmt.Errorf("移动侦测冷却中，还需等待 %v", cooldown-time.Since(lastEvent.Timestamp))
			}
		}
	}

	// 查找区域名称
	zoneName := "全屏"
	for _, zone := range camera.Motion.Zones {
		if zone.ID == zoneID {
			zoneName = zone.Name
			break
		}
	}

	now := time.Now()
	event := &MotionEvent{
		ID:          uuid.New().String(),
		CameraID:    cameraID,
		CameraName:  camera.Name,
		Timestamp:   now,
		ZoneID:      zoneID,
		ZoneName:    zoneName,
		Confidence:  confidence,
		BoundingBox: bbox,
		Handled:     false,
	}

	m.motionEvents = append(m.motionEvents, event)

	m.logger.Info("移动侦测事件",
		zap.String("cameraId", cameraID),
		zap.String("zoneId", zoneID),
		zap.Float64("confidence", confidence),
	)

	// 执行触发动作
	m.executeMotionActions(camera, event)

	return event, nil
}

// executeMotionActions 执行移动侦测触发动作.
func (m *Manager) executeMotionActions(camera *Camera, event *MotionEvent) {
	for _, action := range camera.Motion.Actions {
		if !action.Enabled {
			continue
		}
		switch action.Type {
		case "snapshot":
			event.Snapshot = fmt.Sprintf("%s/%s/snap_%s.jpg", m.config.StoragePath, camera.ID, event.ID)
			m.logger.Info("拍摄快照", zap.String("path", event.Snapshot))
		case "record":
			if !m.activeRecord[camera.ID] {
				m.activeRecord[camera.ID] = true
				recording := &Recording{
					ID:         uuid.New().String(),
					CameraID:   camera.ID,
					CameraName: camera.Name,
					StartTime:  time.Now(),
					Trigger:    "motion",
				}
				m.recordings = append(m.recordings, recording)
				m.logger.Info("自动开始录像", zap.String("recordingId", recording.ID))
			}
		case "webhook":
			m.logger.Info("发送 Webhook", zap.String("url", action.Target))
		case "email":
			m.logger.Info("发送邮件通知", zap.String("email", action.Target))
		}
	}
}

// GetMotionEvents 获取移动侦测事件
func (m *Manager) GetMotionEvents(query MotionEventQuery) []*MotionEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}

	var result []*MotionEvent
	for i := len(m.motionEvents) - 1; i >= 0; i-- {
		event := m.motionEvents[i]
		if query.CameraID != "" && event.CameraID != query.CameraID {
			continue
		}
		result = append(result, event)
		if len(result) >= limit {
			break
		}
	}
	return result
}

// ========== 系统状态 ==========

// GetStatus 获取系统状态
func (m *Manager) GetStatus() SystemStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	online, offline, recording := 0, 0, 0
	for _, cam := range m.cameras {
		if cam.Status == CameraStatusOnline {
			online++
		} else {
			offline++
		}
	}
	for range m.activeRecord {
		recording++
	}

	motion24h := 0
	cutoff := time.Now().Add(-24 * time.Hour)
	for _, event := range m.motionEvents {
		if event.Timestamp.After(cutoff) {
			motion24h++
		}
	}

	var totalSize int64
	for _, rec := range m.recordings {
		totalSize += rec.FileSize
	}

	uptime := time.Since(m.startTime)

	return SystemStatus{
		TotalCameras:    len(m.cameras),
		OnlineCameras:   online,
		OfflineCameras:  offline,
		RecordingCount:  recording,
		TotalRecordings: len(m.recordings),
		StorageUsed:     totalSize,
		StorageTotal:    int64(m.config.MaxStorageGB) * 1024 * 1024 * 1024,
		MotionEvents24h: motion24h,
		Uptime:          uptime.String(),
	}
}

// GetStorageStats 获取存储统计
func (m *Manager) GetStorageStats() StorageStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := StorageStats{
		TotalRecordings: len(m.recordings),
	}

	if len(m.recordings) == 0 {
		return stats
	}

	var totalSize int64
	var oldest, newest time.Time
	for _, rec := range m.recordings {
		totalSize += rec.FileSize
		if oldest.IsZero() || rec.StartTime.Before(oldest) {
			oldest = rec.StartTime
		}
		if newest.IsZero() || rec.StartTime.After(newest) {
			newest = rec.StartTime
		}
	}

	stats.TotalSize = totalSize
	stats.AvgFileSize = totalSize / int64(len(m.recordings))
	if !oldest.IsZero() {
		stats.OldestRecording = &oldest
	}
	if !newest.IsZero() {
		stats.NewestRecording = &newest
	}

	return stats
}

// GetConfig 获取系统配置
func (m *Manager) GetConfig() SystemConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新系统配置
func (m *Manager) UpdateConfig(cfg SystemConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	m.logger.Info("系统配置已更新")
}

// ========== 辅助方法 ==========

// copyCamera 深拷贝摄像头（隐藏密码）
func (m *Manager) copyCamera(cam *Camera) *Camera {
	cp := *cam
	cp.Password = "" // 不返回密码
	if cam.Tags != nil {
		cp.Tags = make([]string, len(cam.Tags))
		copy(cp.Tags, cam.Tags)
	}
	return &cp
}

// SimulateMotionEvent 模拟移动侦测（用于测试）
func (m *Manager) SimulateMotionEvent(cameraID string) (*MotionEvent, error) {
	return m.TriggerMotionEvent(cameraID, "", rand.Float64(), &BoundingBox{
		X:      rand.Intn(500),
		Y:      rand.Intn(500),
		Width:  100 + rand.Intn(200),
		Height: 100 + rand.Intn(200),
	})
}
