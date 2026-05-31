// Package surveillance 提供视频监控管理功能
// 参考群晖 Surveillance Station，支持摄像头管理、实时流、录像、移动侦测等
package surveillance

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// CameraProtocol 摄像头协议
type CameraProtocol string

const (
	ProtocolONVIF CameraProtocol = "ONVIF"
	ProtocolRTSP  CameraProtocol = "RTSP"
)

// CameraStatus 摄像头状态
type CameraStatus string

const (
	CameraStatusOnline  CameraStatus = "online"
	CameraStatusOffline CameraStatus = "offline"
	CameraStatusError   CameraStatus = "error"
)

// RecordingMode 录像模式
type RecordingMode string

const (
	RecordingModeContinuous RecordingMode = "continuous" // 连续录像
	RecordingModeEvent      RecordingMode = "event"      // 事件触发
	RecordingModeSchedule   RecordingMode = "schedule"    // 计划录像
	RecordingModeManual     RecordingMode = "manual"      // 手动录像
)

// MotionSensitivity 移动侦测灵敏度
type MotionSensitivity string

const (
	SensitivityLow    MotionSensitivity = "low"
	SensitivityMedium MotionSensitivity = "medium"
	SensitivityHigh   MotionSensitivity = "high"
)

// EventType 告警事件类型
type EventType string

const (
	EventMotionDetection EventType = "motion_detection" // 移动侦测
	EventTampering       EventType = "tampering"        // 遮挡告警
	EventDisconnect      EventType = "disconnect"        // 断线告警
	EventReconnect       EventType = "reconnect"        // 恢复连接
)

// ActionTrigger 联动动作类型
type ActionTrigger string

const (
	ActionRecord    ActionTrigger = "record"    // 触发录像
	ActionNotify    ActionTrigger = "notify"    // 发送通知
	ActionBuzzer    ActionTrigger = "buzzer"    // 蜂鸣报警
	ActionSnapshot  ActionTrigger = "snapshot"  // 抓拍快照
)

// ========== 摄像头相关 ==========

// Camera 摄像头配置
type Camera struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Protocol    CameraProtocol `json:"protocol"`
	Host        string         `json:"host"`
	Port        int            `json:"port"`
	StreamPath  string         `json:"streamPath"`
	Username    string         `json:"username,omitempty"`
	Password    string         `json:"password,omitempty"`
	Status      CameraStatus   `json:"status"`
	GroupID     string         `json:"groupId,omitempty"`
	Resolution  string         `json:"resolution,omitempty"`  // 1920x1080
	Bitrate     int            `json:"bitrate,omitempty"`     // kbps
	FPS         int            `json:"fps,omitempty"`
	Manufacturer string        `json:"manufacturer,omitempty"`
	Model       string         `json:"model,omitempty"`
	Location    string         `json:"location,omitempty"`
	Enabled     bool           `json:"enabled"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// CameraStream 实时流信息
type CameraStream struct {
	CameraID    string    `json:"cameraId"`
	StreamURL   string    `json:"streamUrl"`
	Resolution  string    `json:"resolution"`
	Bitrate     int       `json:"bitrate"`
	FPS         int       `json:"fps"`
	Codec       string    `json:"codec"`
	StartTime   time.Time `json:"startTime"`
}

// ========== 录像相关 ==========

// Recording 录像记录
type Recording struct {
	ID         string        `json:"id"`
	CameraID   string        `json:"cameraId"`
	Mode       RecordingMode `json:"mode"`
	StartTime  time.Time     `json:"startTime"`
	EndTime    time.Time     `json:"endTime,omitempty"`
	Duration   time.Duration `json:"duration"`
	FilePath   string        `json:"filePath"`
	FileSize   int64         `json:"fileSize"`
	Resolution string        `json:"resolution"`
	Bitrate    int           `json:"bitrate"`
	HasEvent   bool          `json:"hasEvent"` // 是否包含事件
	EventIDs   []string      `json:"eventIds,omitempty"`
	CreatedAt  time.Time     `json:"createdAt"`
}

// RecordingSchedule 录像计划
type RecordingSchedule struct {
	ID       string          `json:"id"`
	CameraID string          `json:"cameraId"`
	Mode     RecordingMode   `json:"mode"`
	Days     []time.Weekday  `json:"days"`     // 生效的星期几
	Start    string          `json:"start"`     // HH:MM
	End      string          `json:"end"`       // HH:MM
	Enabled  bool            `json:"enabled"`
}

// ========== 移动侦测 ==========

// MotionDetection 移像侦测配置
type MotionDetection struct {
	CameraID    string            `json:"cameraId"`
	Enabled     bool              `json:"enabled"`
	Sensitivity MotionSensitivity `json:"sensitivity"`
	Regions     []MotionRegion    `json:"regions"`
	CooldownSec int               `json:"cooldownSec"` // 触发后冷却时间
}

// MotionRegion 移动侦测区域
type MotionRegion struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// ========== 告警事件 ==========

// Event 告警事件
type Event struct {
	ID        string    `json:"id"`
	CameraID  string    `json:"cameraId"`
	Type      EventType `json:"type"`
	Message   string    `json:"message"`
	ImageURL  string    `json:"imageUrl,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Acked     bool      `json:"acked"` // 已确认
	AckedBy   string    `json:"ackedBy,omitempty"`
	AckedAt   time.Time `json:"ackedAt,omitempty"`
}

// ActionRule 事件联动规则
type ActionRule struct {
	ID        string        `json:"id"`
	CameraID  string        `json:"cameraId"`
	EventType EventType     `json:"eventType"`
	Actions   []ActionTrigger `json:"actions"`
	Enabled   bool          `json:"enabled"`
}

// ========== 分组和布局 ==========

// CameraGroup 摄像头分组
type CameraGroup struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	CameraIDs   []string `json:"cameraIds"`
	Layout      Layout   `json:"layout"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Layout 布局配置
type Layout struct {
	Rows    int `json:"rows"`
	Columns int `json:"columns"`
}

// ========== 存储管理 ==========

// StorageQuota 存储配额
type StorageQuota struct {
	CameraID        string `json:"cameraId"`
	MaxSizeGB       int    `json:"maxSizeGb"`       // 最大存储 GB
	CurrentSizeGB   int    `json:"currentSizeGb"`   // 当前存储 GB
	RetentionDays   int    `json:"retentionDays"`   // 保留天数
	LoopRecording   bool   `json:"loopRecording"`   // 循环录像
}

// ========== Manager ==========

// Manager 监控中心管理器
type Manager struct {
	mu             sync.RWMutex
	cameras        map[string]*Camera
	streams        map[string]*CameraStream
	recordings     []*Recording
	schedules      map[string]*RecordingSchedule
	motions        map[string]*MotionDetection
	events         []*Event
	actionRules    map[string]*ActionRule
	groups         map[string]*CameraGroup
	quotas         map[string]*StorageQuota
	stopCh         chan struct{}
	running        bool
	onEvent        func(*Event) // 事件回调
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		cameras:     make(map[string]*Camera),
		streams:     make(map[string]*CameraStream),
		recordings:  make([]*Recording, 0),
		schedules:   make(map[string]*RecordingSchedule),
		motions:     make(map[string]*MotionDetection),
		events:      make([]*Event, 0),
		actionRules: make(map[string]*ActionRule),
		groups:      make(map[string]*CameraGroup),
		quotas:      make(map[string]*StorageQuota),
		stopCh:      make(chan struct{}),
	}
}

// ========== 摄像头管理 ==========

// AddCamera 添加摄像头
func (m *Manager) AddCamera(cam *Camera) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cam.ID == "" {
		return fmt.Errorf("camera ID is required")
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

// RemoveCamera 删除摄像头
func (m *Manager) RemoveCamera(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[id]; !exists {
		return fmt.Errorf("camera %s not found", id)
	}

	// 停止流
	delete(m.streams, id)
	delete(m.cameras, id)
	delete(m.motions, id)
	delete(m.quotas, id)
	log.Printf("[surveillance] camera removed: %s", id)
	return nil
}

// UpdateCamera 更新摄像头配置
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

// GetCamera 获取摄像头
func (m *Manager) GetCamera(id string) (*Camera, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cam, exists := m.cameras[id]
	if !exists {
		return nil, fmt.Errorf("camera %s not found", id)
	}
	return cam, nil
}

// ListCameras 列出所有摄像头
func (m *Manager) ListCameras() []*Camera {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cameras := make([]*Camera, 0, len(m.cameras))
	for _, cam := range m.cameras {
		cameras = append(cameras, cam)
	}
	return cameras
}

// UpdateCameraStatus 更新摄像头状态
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

	// 断线/恢复事件
	if oldStatus == CameraStatusOnline && status == CameraStatusOffline {
		m.addEvent(id, EventDisconnect, fmt.Sprintf("摄像头 %s 断线", cam.Name))
	} else if oldStatus == CameraStatusOffline && status == CameraStatusOnline {
		m.addEvent(id, EventReconnect, fmt.Sprintf("摄像头 %s 恢复连接", cam.Name))
	}

	return nil
}

// ========== 实时流管理 ==========

// StartStream 开始实时流
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

	// 构建流URL
	streamURL := fmt.Sprintf("rtsp://%s:%d%s", cam.Host, cam.Port, cam.StreamPath)

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

// StopStream 停止实时流
func (m *Manager) StopStream(cameraID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.streams, cameraID)
	log.Printf("[surveillance] stream stopped: %s", cameraID)
}

// GetStream 获取实时流信息
func (m *Manager) GetStream(cameraID string) (*CameraStream, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stream, exists := m.streams[cameraID]
	if !exists {
		return nil, fmt.Errorf("no active stream for camera %s", cameraID)
	}
	return stream, nil
}

// ListStreams 列出所有活动流
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

// StartRecording 开始录像
func (m *Manager) StartRecording(cameraID string, mode RecordingMode) (*Recording, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cam, exists := m.cameras[cameraID]
	if !exists {
		return nil, fmt.Errorf("camera %s not found", cameraID)
	}

	recording := &Recording{
		ID:         fmt.Sprintf("rec_%s_%d", cameraID, time.Now().UnixNano()),
		CameraID:   cameraID,
		Mode:       mode,
		StartTime:  time.Now(),
		Resolution: cam.Resolution,
		Bitrate:    cam.Bitrate,
		CreatedAt:  time.Now(),
	}

	m.recordings = append(m.recordings, recording)
	log.Printf("[surveillance] recording started: %s (camera: %s, mode: %s)", recording.ID, cameraID, mode)
	return recording, nil
}

// StopRecording 停止录像
func (m *Manager) StopRecording(recordingID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, rec := range m.recordings {
		if rec.ID == recordingID && rec.EndTime.IsZero() {
			rec.EndTime = time.Now()
			rec.Duration = rec.EndTime.Sub(rec.StartTime)
			log.Printf("[surveillance] recording stopped: %s (duration: %s)", recordingID, rec.Duration)
			return nil
		}
	}
	return fmt.Errorf("recording %s not found or already stopped", recordingID)
}

// ListRecordings 列出录像
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

// GetRecording 获取录像详情
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

// GetRecordingsByTime 按时间范围查询录像
func (m *Manager) GetRecordingsByTime(cameraID string, start, end time.Time) []*Recording {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Recording
	for _, rec := range m.recordings {
		if rec.CameraID != cameraID {
			continue
		}
		if !rec.StartTime.Before(start) && (rec.EndTime.IsZero() || !rec.EndTime.After(end)) {
			result = append(result, rec)
		}
	}
	return result
}

// AddSchedule 添加录像计划
func (m *Manager) AddSchedule(schedule *RecordingSchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if schedule.ID == "" {
		return fmt.Errorf("schedule ID is required")
	}
	if _, exists := m.cameras[schedule.CameraID]; !exists {
		return fmt.Errorf("camera %s not found", schedule.CameraID)
	}

	m.schedules[schedule.ID] = schedule
	log.Printf("[surveillance] schedule added: %s", schedule.ID)
	return nil
}

// RemoveSchedule 删除录像计划
func (m *Manager) RemoveSchedule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.schedules[id]; !exists {
		return fmt.Errorf("schedule %s not found", id)
	}

	delete(m.schedules, id)
	return nil
}

// ListSchedules 列出录像计划
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

// ========== 移动侦测 ==========

// SetMotionDetection 配置移动侦测
func (m *Manager) SetMotionDetection(config *MotionDetection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[config.CameraID]; !exists {
		return fmt.Errorf("camera %s not found", config.CameraID)
	}

	m.motions[config.CameraID] = config
	log.Printf("[surveillance] motion detection updated: camera=%s, enabled=%v", config.CameraID, config.Enabled)
	return nil
}

// GetMotionDetection 获取移动侦测配置
func (m *Manager) GetMotionDetection(cameraID string) (*MotionDetection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config, exists := m.motions[cameraID]
	if !exists {
		return nil, fmt.Errorf("motion detection not configured for camera %s", cameraID)
	}
	return config, nil
}

// TriggerMotionEvent 触发移动侦测事件
func (m *Manager) TriggerMotionEvent(cameraID string, region string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	config, exists := m.motions[cameraID]
	if !exists || !config.Enabled {
		return fmt.Errorf("motion detection not enabled for camera %s", cameraID)
	}

	cam := m.cameras[cameraID]
	msg := fmt.Sprintf("移动侦测触发: %s", cam.Name)
	if region != "" {
		msg += fmt.Sprintf(" (区域: %s)", region)
	}

	m.addEvent(cameraID, EventMotionDetection, msg)
	m.triggerActions(cameraID, EventMotionDetection)
	return nil
}

// ========== 告警事件 ==========

// addEvent 添加事件（内部方法，调用者需持有锁）
func (m *Manager) addEvent(cameraID string, eventType EventType, message string) {
	event := &Event{
		ID:        fmt.Sprintf("evt_%s_%d", cameraID, time.Now().UnixNano()),
		CameraID:  cameraID,
		Type:      eventType,
		Message:   message,
		Timestamp: time.Now(),
	}

	m.events = append(m.events, event)
	log.Printf("[surveillance] event: %s - %s", event.ID, message)

	// 调用回调
	if m.onEvent != nil {
		go m.onEvent(event)
	}
}

// triggerActions 触发联动动作（内部方法，调用者需持有锁）
func (m *Manager) triggerActions(cameraID string, eventType EventType) {
	for _, rule := range m.actionRules {
		if rule.CameraID == cameraID && rule.EventType == eventType && rule.Enabled {
			for _, action := range rule.Actions {
				switch action {
				case ActionRecord:
					// 触发录像
					go func() {
						m.StartRecording(cameraID, RecordingModeEvent)
					}()
				case ActionNotify:
					log.Printf("[surveillance] notification sent for camera %s", cameraID)
				case ActionBuzzer:
					log.Printf("[surveillance] buzzer triggered for camera %s", cameraID)
				case ActionSnapshot:
					log.Printf("[surveillance] snapshot taken for camera %s", cameraID)
				}
			}
		}
	}
}

// GetEvents 获取事件列表
func (m *Manager) GetEvents(cameraID string, limit int) []*Event {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Event
	count := 0
	for i := len(m.events) - 1; i >= 0 && (limit <= 0 || count < limit); i-- {
		if cameraID == "" || m.events[i].CameraID == cameraID {
			result = append(result, m.events[i])
			count++
		}
	}
	return result
}

// AckEvent 确认事件
func (m *Manager) AckEvent(eventID, ackedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, event := range m.events {
		if event.ID == eventID {
			event.Acked = true
			event.AckedBy = ackedBy
			event.AckedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("event %s not found", eventID)
}

// AddActionRule 添加联动规则
func (m *Manager) AddActionRule(rule *ActionRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("action rule ID is required")
	}
	if _, exists := m.cameras[rule.CameraID]; !exists {
		return fmt.Errorf("camera %s not found", rule.CameraID)
	}

	m.actionRules[rule.ID] = rule
	log.Printf("[surveillance] action rule added: %s", rule.ID)
	return nil
}

// RemoveActionRule 删除联动规则
func (m *Manager) RemoveActionRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.actionRules[id]; !exists {
		return fmt.Errorf("action rule %s not found", id)
	}

	delete(m.actionRules, id)
	return nil
}

// ListActionRules 列出联动规则
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

// ========== 分组和布局 ==========

// CreateGroup 创建摄像头分组
func (m *Manager) CreateGroup(group *CameraGroup) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if group.ID == "" {
		return fmt.Errorf("group ID is required")
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

// DeleteGroup 删除分组
func (m *Manager) DeleteGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.groups[id]; !exists {
		return fmt.Errorf("group %s not found", id)
	}

	delete(m.groups, id)
	return nil
}

// UpdateGroup 更新分组
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

// GetGroup 获取分组
func (m *Manager) GetGroup(id string) (*CameraGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, exists := m.groups[id]
	if !exists {
		return nil, fmt.Errorf("group %s not found", id)
	}
	return group, nil
}

// ListGroups 列出所有分组
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

// SetStorageQuota 设置存储配额
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

// GetStorageQuota 获取存储配额
func (m *Manager) GetStorageQuota(cameraID string) (*StorageQuota, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quota, exists := m.quotas[cameraID]
	if !exists {
		return nil, fmt.Errorf("storage quota not set for camera %s", cameraID)
	}
	return quota, nil
}

// CheckStorageQuota 检查存储配额
func (m *Manager) CheckStorageQuota(cameraID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quota, exists := m.quotas[cameraID]
	if !exists {
		return true, nil // 没有配额限制
	}

	return quota.CurrentSizeGB < quota.MaxSizeGB, nil
}

// ========== 系统管理 ==========

// SetEventCallback 设置事件回调
func (m *Manager) SetEventCallback(callback func(*Event)) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.onEvent = callback
}

// GetStatus 获取系统状态
func (m *Manager) GetStatus() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	onlineCount := 0
	for _, cam := range m.cameras {
		if cam.Status == CameraStatusOnline {
			onlineCount++
		}
	}

	return map[string]interface{}{
		"totalCameras":   len(m.cameras),
		"onlineCameras":  onlineCount,
		"offlineCameras": len(m.cameras) - onlineCount,
		"activeStreams":   len(m.streams),
		"totalRecordings": len(m.recordings),
		"totalEvents":     len(m.events),
		"totalGroups":     len(m.groups),
	}
}

// Start 启动管理器
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

// Stop 停止管理器
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

// monitorLoop 监控循环
func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkCameraStatus()
		}
	}
}

// checkCameraStatus 检查摄像头状态
func (m *Manager) checkCameraStatus() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 模拟检查摄像头在线状态
	for _, cam := range m.cameras {
		if cam.Enabled {
			// 这里实际实现应该是 ping 摄像头或检查 RTSP 连接
			// 暂时只记录日志
			log.Printf("[surveillance] checking camera status: %s", cam.ID)
		}
	}
}
