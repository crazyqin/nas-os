// Package surveillance - 监控中心模块
// 对标群晖 Surveillance Station，支持摄像头管理、录像、移动侦测、时间线回放
package surveillance

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Camera 摄像头信息
type Camera struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Protocol    string            `json:"protocol"` // rtsp, onvif, hls
	URL         string            `json:"url"`
	Location    string            `json:"location"`
	Status      string            `json:"status"` // online, offline, error
	Resolution  string            `json:"resolution"`
	Codec       string            `json:"codec"`
	FPS         int               `json:"fps"`
	BitrateKbps int               `json:"bitrate_kbps"`
	MotionEnabled bool            `json:"motion_enabled"`
	MotionSensitivity int         `json:"motion_sensitivity"` // 1-100
	RecordingMode string          `json:"recording_mode"` // continuous, motion, schedule
	StoragePath string            `json:"storage_path"`
	Tags        map[string]string `json:"tags,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Recording 录像记录
type Recording struct {
	ID          string    `json:"id"`
	CameraID    string    `json:"camera_id"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
	Duration    int       `json:"duration_sec"`
	FileSize    int64     `json:"file_size_bytes"`
	FilePath    string    `json:"file_path"`
	HasMotion   bool      `json:"has_motion"`
	MotionEvents []MotionEvent `json:"motion_events,omitempty"`
}

// MotionEvent 移动侦测事件
type MotionEvent struct {
	ID          string    `json:"id"`
	CameraID    string    `json:"camera_id"`
	Timestamp   time.Time `json:"timestamp"`
	Duration    int       `json:"duration_sec"`
	Confidence  float64   `json:"confidence"` // 0-1
	Region      string    `json:"region"`     // detection region
	SnapshotURL string    `json:"snapshot_url,omitempty"`
	Handled     bool      `json:"handled"`
}

// RecordingSchedule 录像计划
type RecordingSchedule struct {
	ID        string          `json:"id"`
	CameraID  string          `json:"camera_id"`
	Name      string          `json:"name"`
	Enabled   bool            `json:"enabled"`
	Type      string          `json:"type"` // continuous, motion, schedule
	Schedules []ScheduleSlot  `json:"schedules"`
	Retention int             `json:"retention_days"` // 保留天数
}

// ScheduleSlot 时间段
type ScheduleSlot struct {
	DayOfWeek int    `json:"day_of_week"` // 0=Sunday
	StartTime string `json:"start_time"`  // HH:MM
	EndTime   string `json:"end_time"`
}

// StorageQuota 存储配额
type StorageQuota struct {
	CameraID       string `json:"camera_id"`
	TotalBytes     int64  `json:"total_bytes"`
	UsedBytes      int64  `json:"used_bytes"`
	AvailableBytes int64  `json:"available_bytes"`
	RecordingDays  int    `json:"recording_days"`
}

// SurveillanceManager 监控管理器
type SurveillanceManager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	storagePath string

	cameras    map[string]*Camera
	recordings map[string][]*Recording // cameraID -> recordings
	motions    map[string][]*MotionEvent
	schedules  map[string][]*RecordingSchedule
	quotas     map[string]*StorageQuota

	// 录像控制
	recordingCtx    context.Context
	recordingCancel context.CancelFunc
}

// NewSurveillanceManager 创建监控管理器
func NewSurveillanceManager(logger *zap.Logger, storagePath string) *SurveillanceManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &SurveillanceManager{
		logger:          logger,
		storagePath:     storagePath,
		cameras:         make(map[string]*Camera),
		recordings:      make(map[string][]*Recording),
		motions:         make(map[string][]*MotionEvent),
		schedules:       make(map[string][]*RecordingSchedule),
		quotas:          make(map[string]*StorageQuota),
		recordingCtx:    ctx,
		recordingCancel: cancel,
	}
}

// AddCamera 添加摄像头
func (sm *SurveillanceManager) AddCamera(ctx context.Context, cam *Camera) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if cam.ID == "" {
		cam.ID = fmt.Sprintf("cam-%d", time.Now().UnixNano())
	}
	if _, exists := sm.cameras[cam.ID]; exists {
		return fmt.Errorf("camera %s already exists", cam.ID)
	}

	cam.Status = "offline"
	cam.CreatedAt = time.Now()
	cam.UpdatedAt = time.Now()
	sm.cameras[cam.ID] = cam

	sm.logger.Info("摄像头已添加", zap.String("id", cam.ID), zap.String("name", cam.Name))
	return nil
}

// UpdateCamera 更新摄像头
func (sm *SurveillanceManager) UpdateCamera(ctx context.Context, cam *Camera) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	existing, exists := sm.cameras[cam.ID]
	if !exists {
		return fmt.Errorf("camera %s not found", cam.ID)
	}

	cam.CreatedAt = existing.CreatedAt
	cam.UpdatedAt = time.Now()
	sm.cameras[cam.ID] = cam
	return nil
}

// RemoveCamera 移除摄像头
func (sm *SurveillanceManager) RemoveCamera(ctx context.Context, cameraID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.cameras[cameraID]; !exists {
		return fmt.Errorf("camera %s not found", cameraID)
	}

	delete(sm.cameras, cameraID)
	delete(sm.recordings, cameraID)
	delete(sm.motions, cameraID)
	delete(sm.schedules, cameraID)
	delete(sm.quotas, cameraID)

	sm.logger.Info("摄像头已移除", zap.String("id", cameraID))
	return nil
}

// GetCamera 获取摄像头
func (sm *SurveillanceManager) GetCamera(ctx context.Context, cameraID string) (*Camera, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	cam, exists := sm.cameras[cameraID]
	if !exists {
		return nil, fmt.Errorf("camera %s not found", cameraID)
	}
	return cam, nil
}

// ListCameras 列出所有摄像头
func (sm *SurveillanceManager) ListCameras(ctx context.Context) []*Camera {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	cameras := make([]*Camera, 0, len(sm.cameras))
	for _, cam := range sm.cameras {
		cameras = append(cameras, cam)
	}
	return cameras
}

// StartRecording 开始录像
func (sm *SurveillanceManager) StartRecording(ctx context.Context, cameraID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cam, exists := sm.cameras[cameraID]
	if !exists {
		return fmt.Errorf("camera %s not found", cameraID)
	}

	cam.Status = "recording"
	cam.UpdatedAt = time.Now()

	recording := &Recording{
		ID:        fmt.Sprintf("rec-%d", time.Now().UnixNano()),
		CameraID:  cameraID,
		StartTime: time.Now(),
	}
	sm.recordings[cameraID] = append(sm.recordings[cameraID], recording)

	sm.logger.Info("录像已开始", zap.String("camera", cameraID))
	return nil
}

// StopRecording 停止录像
func (sm *SurveillanceManager) StopRecording(ctx context.Context, cameraID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	cam, exists := sm.cameras[cameraID]
	if !exists {
		return fmt.Errorf("camera %s not found", cameraID)
	}

	cam.Status = "online"
	cam.UpdatedAt = time.Now()

	recordings := sm.recordings[cameraID]
	if len(recordings) > 0 {
		last := recordings[len(recordings)-1]
		if last.EndTime.IsZero() {
			last.EndTime = time.Now()
			last.Duration = int(last.EndTime.Sub(last.StartTime).Seconds())
		}
	}

	sm.logger.Info("录像已停止", zap.String("camera", cameraID))
	return nil
}

// GetRecordings 获取录像列表
func (sm *SurveillanceManager) GetRecordings(ctx context.Context, cameraID string, start, end time.Time) []*Recording {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	recordings := sm.recordings[cameraID]
	var result []*Recording
	for _, rec := range recordings {
		if !rec.StartTime.Before(start) && !rec.StartTime.After(end) {
			result = append(result, rec)
		}
	}
	return result
}

// ReportMotion 上报移动侦测
func (sm *SurveillanceManager) ReportMotion(ctx context.Context, event *MotionEvent) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.cameras[event.CameraID]; !exists {
		return fmt.Errorf("camera %s not found", event.CameraID)
	}

	if event.ID == "" {
		event.ID = fmt.Sprintf("motion-%d", time.Now().UnixNano())
	}
	event.Timestamp = time.Now()

	sm.motions[event.CameraID] = append(sm.motions[event.CameraID], event)

	sm.logger.Info("移动侦测事件",
		zap.String("camera", event.CameraID),
		zap.Float64("confidence", event.Confidence))
	return nil
}

// GetMotionEvents 获取移动侦测事件
func (sm *SurveillanceManager) GetMotionEvents(ctx context.Context, cameraID string, start, end time.Time) []*MotionEvent {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	events := sm.motions[cameraID]
	var result []*MotionEvent
	for _, evt := range events {
		if !evt.Timestamp.Before(start) && !evt.Timestamp.After(end) {
			result = append(result, evt)
		}
	}
	return result
}

// SetRecordingSchedule 设置录像计划
func (sm *SurveillanceManager) SetRecordingSchedule(ctx context.Context, schedule *RecordingSchedule) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.cameras[schedule.CameraID]; !exists {
		return fmt.Errorf("camera %s not found", schedule.CameraID)
	}

	if schedule.ID == "" {
		schedule.ID = fmt.Sprintf("sched-%d", time.Now().UnixNano())
	}

	sm.schedules[schedule.CameraID] = append(sm.schedules[schedule.CameraID], schedule)
	return nil
}

// GetStorageQuota 获取存储配额
func (sm *SurveillanceManager) GetStorageQuota(ctx context.Context, cameraID string) (*StorageQuota, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	quota, exists := sm.quotas[cameraID]
	if !exists {
		return &StorageQuota{
			CameraID:       cameraID,
			TotalBytes:     100 * 1024 * 1024 * 1024, // 100GB default
			UsedBytes:      0,
			AvailableBytes: 100 * 1024 * 1024 * 1024,
			RecordingDays:  30,
		}, nil
	}
	return quota, nil
}

// GetTimeline 获取时间线数据
func (sm *SurveillanceManager) GetTimeline(ctx context.Context, cameraID string, date time.Time) map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	recordings := sm.recordings[cameraID]
	motions := sm.motions[cameraID]

	var dayRecordings []*Recording
	for _, rec := range recordings {
		if !rec.StartTime.Before(startOfDay) && !rec.StartTime.After(endOfDay) {
			dayRecordings = append(dayRecordings, rec)
		}
	}

	var dayMotions []*MotionEvent
	for _, evt := range motions {
		if !evt.Timestamp.Before(startOfDay) && !evt.Timestamp.After(endOfDay) {
			dayMotions = append(dayMotions, evt)
		}
	}

	return map[string]interface{}{
		"date":       date.Format("2006-01-02"),
		"camera_id":  cameraID,
		"recordings": dayRecordings,
		"motions":    dayMotions,
	}
}

// Stop 停止监控管理器
func (sm *SurveillanceManager) Stop() {
	sm.recordingCancel()
	sm.logger.Info("监控管理器已停止")
}
