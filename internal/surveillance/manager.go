// Package surveillance 提供视频监控管理功能
// 参考群晖 Surveillance Station，支持摄像头管理、实时流、录像、移动侦测等
package surveillance

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// Manager 监控中心管理器.
type Manager struct {
	mu          sync.RWMutex
	cameras     map[string]*Camera
	streams     map[string]*CameraStream
	recordings  []*Recording
	schedules   map[string]*RecordingSchedule
	motions     map[string]*MotionDetection
	events      []*MotionEvent
	alerts      []*Alert
	actionRules map[string]*ActionRule
	groups      map[string]*CameraGroup
	quotas      map[string]*StorageQuota
	snapshots   []*Snapshot
	stopCh      chan struct{}
	running     bool
	onAlert     func(*Alert) // 告警回调
}

// NewManager 创建管理器.
func NewManager() *Manager {
	m := &Manager{
		cameras:     make(map[string]*Camera),
		streams:     make(map[string]*CameraStream),
		recordings:  make([]*Recording, 0),
		schedules:   make(map[string]*RecordingSchedule),
		motions:     make(map[string]*MotionDetection),
		events:      make([]*MotionEvent, 0),
		alerts:      make([]*Alert, 0),
		actionRules: make(map[string]*ActionRule),
		groups:      make(map[string]*CameraGroup),
		quotas:      make(map[string]*StorageQuota),
		snapshots:   make([]*Snapshot, 0),
		stopCh:      make(chan struct{}),
	}

	// 初始化模拟数据
	m.initMockData()

	return m
}

// generateID 生成唯一ID.
func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d_%04x", prefix, time.Now().UnixNano(), rand.Intn(0xffff))
}

// initMockData 初始化模拟数据.
func (m *Manager) initMockData() {
	// 添加模拟摄像头
	mockCameras := []Camera{
		{
			ID: "cam-001", Name: "前门摄像头", Protocol: ProtocolRTSP,
			RTSPUrl: "rtsp://192.168.1.100:554/stream1", Host: "192.168.1.100", Port: 554,
			StreamPath: "/stream1", Status: CameraStatusOnline, Resolution: "1920x1080",
			FPS: 25, Bitrate: 4096, Location: "前门", Enabled: true,
		},
		{
			ID: "cam-002", Name: "后院摄像头", Protocol: ProtocolRTSP,
			RTSPUrl: "rtsp://192.168.1.101:554/stream1", Host: "192.168.1.101", Port: 554,
			StreamPath: "/stream1", Status: CameraStatusOnline, Resolution: "1920x1080",
			FPS: 25, Bitrate: 4096, Location: "后院", Enabled: true,
		},
		{
			ID: "cam-003", Name: "车库摄像头", Protocol: ProtocolONVIF,
			RTSPUrl: "rtsp://192.168.1.102:554/stream1", Host: "192.168.1.102", Port: 554,
			StreamPath: "/stream1", Status: CameraStatusOffline, Resolution: "1280x720",
			FPS: 15, Bitrate: 2048, Location: "车库", Enabled: true,
		},
	}

	for i := range mockCameras {
		cam := &mockCameras[i]
		cam.CreatedAt = time.Now().Add(-time.Duration(rand.Intn(30)) * 24 * time.Hour)
		cam.UpdatedAt = time.Now()
		m.cameras[cam.ID] = cam
	}

	// 添加默认分组
	m.groups["group-default"] = &CameraGroup{
		ID:        "group-default",
		Name:      "默认分组",
		CameraIDs: []string{"cam-001", "cam-002", "cam-003"},
		Layout:    Layout{Rows: 2, Columns: 2},
		CreatedAt: time.Now(),
	}

	// 添加模拟告警
	m.alerts = append(m.alerts, &Alert{
		ID:         generateID("alert"),
		CameraID:   "cam-001",
		CameraName: "前门摄像头",
		EventType:  EventMotionDetection,
		Level:      AlertLevelWarning,
		Message:    "检测到前门区域有移动",
		Status:     AlertStatusPending,
		CreatedAt:  time.Now().Add(-10 * time.Minute),
	})
}

// ========== 摄像头管理 ==========

// AddCamera 添加摄像头.
func (m *Manager) AddCamera(cam *Camera) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cam.ID == "" {
		cam.ID = generateID("cam")
	}
	if _, exists := m.cameras[cam.ID]; exists {
		return fmt.Errorf("camera %s already exists", cam.ID)
	}

	cam.Status = CameraStatusOffline
	cam.Enabled = true
	cam.CreatedAt = time.Now()
	cam.UpdatedAt = time.Now()

	m.cameras[cam.ID] = cam
	log.Printf("[surveillance] camera added: %s (%s)", cam.ID, cam.Name)
	return nil
}

// RemoveCamera 删除摄像头.
func (m *Manager) RemoveCamera(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[id]; !exists {
		return fmt.Errorf("camera %s not found", id)
	}

	// 停止流并清理相关数据
	delete(m.streams, id)
	delete(m.cameras, id)
	delete(m.motions, id)
	delete(m.quotas, id)
	log.Printf("[surveillance] camera removed: %s", id)
	return nil
}

// UpdateCamera 更新摄像头配置.
func (m *Manager) UpdateCamera(cam *Camera) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.cameras[cam.ID]
	if !exists {
		return fmt.Errorf("camera %s not found", cam.ID)
	}

	cam.CreatedAt = existing.CreatedAt
	cam.UpdatedAt = time.Now()
	m.cameras[cam.ID] = cam
	return nil
}

// GetCamera 获取摄像头.
func (m *Manager) GetCamera(id string) (*Camera, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cam, exists := m.cameras[id]
	if !exists {
		return nil, fmt.Errorf("camera %s not found", id)
	}
	return cam, nil
}

// ListCameras 列出所有摄像头.
func (m *Manager) ListCameras() []*Camera {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cameras := make([]*Camera, 0, len(m.cameras))
	for _, cam := range m.cameras {
		cameras = append(cameras, cam)
	}
	return cameras
}

// UpdateCameraStatus 更新摄像头状态.
func (m *Manager) UpdateCameraStatus(id string, status CameraStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cam, exists := m.cameras[id]
	if !exists {
		return fmt.Errorf("camera %s not found", id)
	}

	oldStatus := cam.Status
	cam.Status = status
	cam.UpdatedAt = time.Now()

	// 断线/恢复事件生成告警
	if oldStatus == CameraStatusOnline && status == CameraStatusOffline {
		m.generateAlert(id, EventDisconnect, AlertLevelCritical,
			fmt.Sprintf("摄像头 %s 断线", cam.Name))
	} else if oldStatus == CameraStatusOffline && status == CameraStatusOnline {
		m.generateAlert(id, EventReconnect, AlertLevelInfo,
			fmt.Sprintf("摄像头 %s 恢复连接", cam.Name))
	}

	return nil
}

// ========== 实时流管理 ==========

// StartStream 开始实时流（模拟）.
func (m *Manager) StartStream(cameraID string) (*CameraStream, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cam, exists := m.cameras[cameraID]
	if !exists {
		return nil, fmt.Errorf("camera %s not found", cameraID)
	}
	if cam.Status != CameraStatusOnline {
		return nil, fmt.Errorf("camera %s is not online", cameraID)
	}

	streamURL := cam.RTSPUrl
	if streamURL == "" {
		streamURL = fmt.Sprintf("rtsp://%s:%d%s", cam.Host, cam.Port, cam.StreamPath)
	}

	stream := &CameraStream{
		CameraID:   cameraID,
		StreamURL:  streamURL,
		Resolution: cam.Resolution,
		Bitrate:    cam.Bitrate,
		FPS:        cam.FPS,
		Codec:      "H.264",
		StartTime:  time.Now(),
	}

	m.streams[cameraID] = stream
	log.Printf("[surveillance] stream started: %s", cameraID)
	return stream, nil
}

// StopStream 停止实时流.
func (m *Manager) StopStream(cameraID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.streams, cameraID)
	log.Printf("[surveillance] stream stopped: %s", cameraID)
}

// GetStream 获取实时流信息.
func (m *Manager) GetStream(cameraID string) (*CameraStream, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stream, exists := m.streams[cameraID]
	if !exists {
		return nil, fmt.Errorf("no active stream for camera %s", cameraID)
	}
	return stream, nil
}

// ListStreams 列出所有活动流.
func (m *Manager) ListStreams() []*CameraStream {
	m.mu.RLock()
	defer m.mu.RUnlock()

	streams := make([]*CameraStream, 0, len(m.streams))
	for _, s := range m.streams {
		streams = append(streams, s)
	}
	return streams
}

// ========== 录像管理 ==========

// StartRecording 开始录像.
func (m *Manager) StartRecording(cameraID string, mode RecordingMode) (*Recording, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cam, exists := m.cameras[cameraID]
	if !exists {
		return nil, fmt.Errorf("camera %s not found", cameraID)
	}

	recording := &Recording{
		ID:         generateID("rec"),
		CameraID:   cameraID,
		Mode:       mode,
		StartTime:  time.Now(),
		Resolution: cam.Resolution,
		Bitrate:    cam.Bitrate,
		FilePath:   fmt.Sprintf("/recordings/%s/%s.mp4", cameraID, time.Now().Format("20060102_150405")),
		CreatedAt:  time.Now(),
	}

	m.recordings = append(m.recordings, recording)
	log.Printf("[surveillance] recording started: %s (camera: %s, mode: %s)", recording.ID, cameraID, mode)
	return recording, nil
}

// StopRecording 停止录像.
func (m *Manager) StopRecording(recordingID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, rec := range m.recordings {
		if rec.ID == recordingID && rec.EndTime.IsZero() {
			rec.EndTime = time.Now()
			rec.Duration = rec.EndTime.Sub(rec.StartTime)
			rec.FileSize = int64(rec.Duration.Seconds()) * int64(rec.Bitrate) * 128 // 模拟文件大小
			log.Printf("[surveillance] recording stopped: %s (duration: %s)", recordingID, rec.Duration)
			return nil
		}
	}
	return fmt.Errorf("recording %s not found or already stopped", recordingID)
}

// ListRecordings 列出录像.
func (m *Manager) ListRecordings(cameraID string) []*Recording {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Recording
	for _, rec := range m.recordings {
		if cameraID == "" || rec.CameraID == cameraID {
			result = append(result, rec)
		}
	}
	return result
}

// GetRecording 获取录像详情.
func (m *Manager) GetRecording(id string) (*Recording, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, rec := range m.recordings {
		if rec.ID == id {
			return rec, nil
		}
	}
	return nil, fmt.Errorf("recording %s not found", id)
}

// ========== 录像计划 ==========

// AddSchedule 添加录像计划.
func (m *Manager) AddSchedule(schedule *RecordingSchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if schedule.ID == "" {
		schedule.ID = generateID("sched")
	}
	if _, exists := m.cameras[schedule.CameraID]; !exists {
		return fmt.Errorf("camera %s not found", schedule.CameraID)
	}

	schedule.CreatedAt = time.Now()
	m.schedules[schedule.ID] = schedule
	log.Printf("[surveillance] schedule added: %s", schedule.ID)
	return nil
}

// ListSchedules 列出录像计划.
func (m *Manager) ListSchedules(cameraID string) []*RecordingSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*RecordingSchedule
	for _, s := range m.schedules {
		if cameraID == "" || s.CameraID == cameraID {
			result = append(result, s)
		}
	}
	return result
}

// ========== 移动侦测和AI检测 ==========

// SetMotionDetection 配置移动侦测.
func (m *Manager) SetMotionDetection(config *MotionDetection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[config.CameraID]; !exists {
		return fmt.Errorf("camera %s not found", config.CameraID)
	}

	m.motions[config.CameraID] = config
	log.Printf("[surveillance] motion detection updated: camera=%s, enabled=%v, ai=%v",
		config.CameraID, config.Enabled, config.AIDetection)
	return nil
}

// GetMotionDetection 获取移动侦测配置.
func (m *Manager) GetMotionDetection(cameraID string) (*MotionDetection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.motions[cameraID]
	if !exists {
		return nil, fmt.Errorf("motion detection not configured for camera %s", cameraID)
	}
	return config, nil
}

// SimulateMotionEvent 模拟移动侦测事件（用于演示）.
func (m *Manager) SimulateMotionEvent(cameraID string) (*MotionEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cam, exists := m.cameras[cameraID]
	if !exists {
		return nil, fmt.Errorf("camera %s not found", cameraID)
	}

	config, hasMotion := m.motions[cameraID]
	isHuman := false
	confidence := 0.5 + rand.Float64()*0.5

	if hasMotion && config.AIDetection {
		// AI人形检测模拟 - 60%概率检测到人
		isHuman = rand.Float64() > 0.4
		if isHuman {
			confidence = 0.8 + rand.Float64()*0.2
		}
	}

	event := &MotionEvent{
		ID:          generateID("motion"),
		CameraID:    cameraID,
		Confidence:  confidence,
		IsHuman:     isHuman,
		SnapshotURL: fmt.Sprintf("/api/v1/surveillance/cameras/%s/snapshot", cameraID),
		Timestamp:   time.Now(),
	}

	// 如果有配置的区域，随机选择一个
	if hasMotion && len(config.Regions) > 0 {
		region := config.Regions[rand.Intn(len(config.Regions))]
		event.RegionID = region.ID
		event.RegionName = region.Name
	}

	m.events = append(m.events, event)

	// 生成告警
	level := AlertLevelInfo
	msg := fmt.Sprintf("摄像头 %s 检测到移动", cam.Name)
	if isHuman {
		level = AlertLevelWarning
		msg = fmt.Sprintf("摄像头 %s 检测到人形活动 (置信度: %.1f%%)", cam.Name, confidence*100)
	}
	m.generateAlert(cameraID, EventMotionDetection, level, msg)

	log.Printf("[surveillance] motion event: camera=%s, human=%v, confidence=%.2f",
		cameraID, isHuman, confidence)

	return event, nil
}

// GetMotionEvents 获取移动侦测事件列表.
func (m *Manager) GetMotionEvents(cameraID string, limit int) []*MotionEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*MotionEvent
	count := 0
	for i := len(m.events) - 1; i >= 0 && (limit <= 0 || count < limit); i-- {
		if cameraID == "" || m.events[i].CameraID == cameraID {
			result = append(result, m.events[i])
			count++
		}
	}
	return result
}

// ========== 告警管理 ==========

// generateAlert 生成告警（内部方法，调用者需持有锁）.
func (m *Manager) generateAlert(cameraID string, eventType EventType, level AlertLevel, message string) {
	cam, exists := m.cameras[cameraID]
	camName := cameraID
	if exists {
		camName = cam.Name
	}

	alert := &Alert{
		ID:         generateID("alert"),
		CameraID:   cameraID,
		CameraName: camName,
		EventType:  eventType,
		Level:      level,
		Message:    message,
		Status:     AlertStatusPending,
		CreatedAt:  time.Now(),
	}

	m.alerts = append(m.alerts, alert)
	log.Printf("[surveillance] alert generated: %s - %s", alert.ID, message)

	// 调用回调
	if m.onAlert != nil {
		go m.onAlert(alert)
	}
}

// GetAlerts 获取告警列表.
func (m *Manager) GetAlerts(cameraID string, status AlertStatus, limit int) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Alert
	count := 0
	for i := len(m.alerts) - 1; i >= 0 && (limit <= 0 || count < limit); i-- {
		alert := m.alerts[i]
		if cameraID != "" && alert.CameraID != cameraID {
			continue
		}
		if status != "" && alert.Status != status {
			continue
		}
		result = append(result, alert)
		count++
	}
	return result
}

// AckAlert 确认告警.
func (m *Manager) AckAlert(alertID, ackedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.alerts {
		if alert.ID == alertID {
			alert.Status = AlertStatusAcked
			alert.AckedBy = ackedBy
			alert.AckedAt = time.Now()
			log.Printf("[surveillance] alert acked: %s by %s", alertID, ackedBy)
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", alertID)
}

// SetAlertCallback 设置告警回调.
func (m *Manager) SetAlertCallback(callback func(*Alert)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onAlert = callback
}

// ========== 联动规则 ==========

// AddActionRule 添加联动规则.
func (m *Manager) AddActionRule(rule *ActionRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = generateID("rule")
	}
	if _, exists := m.cameras[rule.CameraID]; !exists {
		return fmt.Errorf("camera %s not found", rule.CameraID)
	}

	m.actionRules[rule.ID] = rule
	log.Printf("[surveillance] action rule added: %s", rule.ID)
	return nil
}

// ListActionRules 列出联动规则.
func (m *Manager) ListActionRules(cameraID string) []*ActionRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*ActionRule
	for _, rule := range m.actionRules {
		if cameraID == "" || rule.CameraID == cameraID {
			result = append(result, rule)
		}
	}
	return result
}

// ========== 快照 ==========

// TakeSnapshot 抓取快照（模拟）.
func (m *Manager) TakeSnapshot(cameraID string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cam, exists := m.cameras[cameraID]
	if !exists {
		return nil, fmt.Errorf("camera %s not found", cameraID)
	}

	// 解析分辨率
	width, height := 1920, 1080
	fmt.Sscanf(cam.Resolution, "%dx%d", &width, &height)

	snapshot := &Snapshot{
		ID:        generateID("snap"),
		CameraID:  cameraID,
		FilePath:  fmt.Sprintf("/snapshots/%s/%s.jpg", cameraID, time.Now().Format("20060102_150405")),
		Width:     width,
		Height:    height,
		Size:      int64(100+rand.Intn(400)) * 1024, // 100-500KB
		CreatedAt: time.Now(),
	}

	m.snapshots = append(m.snapshots, snapshot)
	log.Printf("[surveillance] snapshot taken: %s (camera: %s)", snapshot.ID, cameraID)
	return snapshot, nil
}

// GetSnapshots 获取快照列表.
func (m *Manager) GetSnapshots(cameraID string, limit int) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Snapshot
	count := 0
	for i := len(m.snapshots) - 1; i >= 0 && (limit <= 0 || count < limit); i-- {
		if cameraID == "" || m.snapshots[i].CameraID == cameraID {
			result = append(result, m.snapshots[i])
			count++
		}
	}
	return result
}

// ========== 分组管理 ==========

// CreateGroup 创建摄像头分组.
func (m *Manager) CreateGroup(group *CameraGroup) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if group.ID == "" {
		group.ID = generateID("group")
	}
	if _, exists := m.groups[group.ID]; exists {
		return fmt.Errorf("group %s already exists", group.ID)
	}

	// 验证摄像头存在
	for _, camID := range group.CameraIDs {
		if _, exists := m.cameras[camID]; !exists {
			return fmt.Errorf("camera %s not found", camID)
		}
	}

	group.CreatedAt = time.Now()
	m.groups[group.ID] = group
	log.Printf("[surveillance] group created: %s (%s)", group.ID, group.Name)
	return nil
}

// UpdateGroup 更新分组.
func (m *Manager) UpdateGroup(group *CameraGroup) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.groups[group.ID]; !exists {
		return fmt.Errorf("group %s not found", group.ID)
	}

	// 验证摄像头存在
	for _, camID := range group.CameraIDs {
		if _, exists := m.cameras[camID]; !exists {
			return fmt.Errorf("camera %s not found", camID)
		}
	}

	m.groups[group.ID] = group
	return nil
}

// DeleteGroup 删除分组.
func (m *Manager) DeleteGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.groups[id]; !exists {
		return fmt.Errorf("group %s not found", id)
	}

	delete(m.groups, id)
	return nil
}

// GetGroup 获取分组.
func (m *Manager) GetGroup(id string) (*CameraGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, exists := m.groups[id]
	if !exists {
		return nil, fmt.Errorf("group %s not found", id)
	}
	return group, nil
}

// ListGroups 列出所有分组.
func (m *Manager) ListGroups() []*CameraGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	groups := make([]*CameraGroup, 0, len(m.groups))
	for _, g := range m.groups {
		groups = append(groups, g)
	}
	return groups
}

// ========== 存储管理 ==========

// SetStorageQuota 设置存储配额.
func (m *Manager) SetStorageQuota(quota *StorageQuota) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[quota.CameraID]; !exists {
		return fmt.Errorf("camera %s not found", quota.CameraID)
	}

	m.quotas[quota.CameraID] = quota
	log.Printf("[surveillance] storage quota set for camera %s: %dGB", quota.CameraID, quota.MaxSizeGB)
	return nil
}

// GetStorageQuota 获取存储配额.
func (m *Manager) GetStorageQuota(cameraID string) (*StorageQuota, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quota, exists := m.quotas[cameraID]
	if !exists {
		return nil, fmt.Errorf("storage quota not set for camera %s", cameraID)
	}
	return quota, nil
}

// CheckAndCleanStorage 检查并清理存储（循环录像）.
func (m *Manager) CheckAndCleanStorage() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for cameraID, quota := range m.quotas {
		if !quota.LoopRecording {
			continue
		}
		if quota.CurrentSizeGB >= quota.MaxSizeGB {
			// 模拟清理旧录像
			log.Printf("[surveillance] cleaning old recordings for camera %s (used: %dGB, max: %dGB)",
				cameraID, quota.CurrentSizeGB, quota.MaxSizeGB)
			quota.CurrentSizeGB = quota.MaxSizeGB * 80 / 100 // 清理到80%
		}
	}
}

// ========== 统计 ==========

// GetStats 获取监控系统统计.
func (m *Manager) GetStats() *SurveillanceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SurveillanceStats{
		TotalCameras:    len(m.cameras),
		ActiveStreams:   len(m.streams),
		TotalRecordings: len(m.recordings),
		TotalEvents:     len(m.events),
		TotalAlerts:     len(m.alerts),
		TotalGroups:     len(m.groups),
		TotalSchedules:  len(m.schedules),
	}

	// 统计在线摄像头
	for _, cam := range m.cameras {
		switch cam.Status {
		case CameraStatusOnline:
			stats.OnlineCameras++
		case CameraStatusOffline:
			stats.OfflineCameras++
		}
	}

	// 统计待处理告警
	for _, alert := range m.alerts {
		if alert.Status == AlertStatusPending {
			stats.PendingAlerts++
		}
	}

	// 模拟存储使用
	stats.StorageUsedGB = float64(len(m.recordings)) * 0.5 // 每个录像约0.5GB
	stats.StorageTotalGB = 1000.0                          // 总共1TB

	return stats
}

// ========== 系统管理 ==========

// Start 启动管理器.
func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return
	}

	m.running = true
	m.stopCh = make(chan struct{})
	log.Println("[surveillance] manager started")

	// 启动状态监控
	go m.monitorLoop()
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	close(m.stopCh)
	m.running = false
	log.Println("[surveillance] manager stopped")
}

// monitorLoop 监控循环.
func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkCameraStatus()
			m.CheckAndCleanStorage()
		}
	}
}

// checkCameraStatus 检查摄像头状态.
func (m *Manager) checkCameraStatus() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟检查摄像头在线状态
	for _, cam := range m.cameras {
		if cam.Enabled && cam.Status == CameraStatusOnline {
			// 这里实际实现应该是 ping 摄像头或检查 RTSP 连接
			log.Printf("[surveillance] checking camera status: %s", cam.ID)
		}
	}
}
