// Package smartsurveillance 提供智能监控中心功能
// engine.go - 监控引擎核心，负责摄像头管理和录像管理
package smartsurveillance

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SurveillanceEngine 监控引擎
type SurveillanceEngine struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	storagePath string
	cameras     map[string]*Camera
	recordings  map[string][]*Recording // cameraID -> recordings
	events      map[string][]*Event
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewSurveillanceEngine 创建监控引擎
func NewSurveillanceEngine(logger *zap.Logger, storagePath string) *SurveillanceEngine {
	ctx, cancel := context.WithCancel(context.Background())
	return &SurveillanceEngine{
		logger:      logger,
		storagePath: storagePath,
		cameras:     make(map[string]*Camera),
		recordings:  make(map[string][]*Recording),
		events:      make(map[string][]*Event),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// ========== 摄像头管理 ==========

// AddCamera 添加摄像头
func (e *SurveillanceEngine) AddCamera(camera *Camera) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if camera.ID == "" {
		camera.ID = uuid.New().String()
	}

	if _, exists := e.cameras[camera.ID]; exists {
		return ErrCameraExists
	}

	camera.Status = CameraStatusOffline
	camera.CreatedAt = time.Now()
	camera.UpdatedAt = time.Now()

	e.cameras[camera.ID] = camera
	e.logger.Info("摄像头已添加",
		zap.String("id", camera.ID),
		zap.String("name", camera.Name),
		zap.String("location", camera.Location))
	return nil
}

// UpdateCamera 更新摄像头
func (e *SurveillanceEngine) UpdateCamera(camera *Camera) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, exists := e.cameras[camera.ID]
	if !exists {
		return ErrCameraNotFound
	}

	camera.CreatedAt = existing.CreatedAt
	camera.Status = existing.Status
	camera.UpdatedAt = time.Now()
	e.cameras[camera.ID] = camera
	return nil
}

// RemoveCamera 移除摄像头
func (e *SurveillanceEngine) RemoveCamera(cameraID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.cameras[cameraID]; !exists {
		return ErrCameraNotFound
	}

	delete(e.cameras, cameraID)
	delete(e.recordings, cameraID)
	delete(e.events, cameraID)

	e.logger.Info("摄像头已移除", zap.String("id", cameraID))
	return nil
}

// GetCamera 获取摄像头
func (e *SurveillanceEngine) GetCamera(cameraID string) (*Camera, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	camera, exists := e.cameras[cameraID]
	if !exists {
		return nil, ErrCameraNotFound
	}
	return camera, nil
}

// ListCameras 列出所有摄像头
func (e *SurveillanceEngine) ListCameras() []*Camera {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cameras := make([]*Camera, 0, len(e.cameras))
	for _, camera := range e.cameras {
		cameras = append(cameras, camera)
	}
	return cameras
}

// UpdateCameraStatus 更新摄像头状态
func (e *SurveillanceEngine) UpdateCameraStatus(cameraID string, status CameraStatus) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	camera, exists := e.cameras[cameraID]
	if !exists {
		return ErrCameraNotFound
	}

	camera.Status = status
	camera.UpdatedAt = time.Now()
	return nil
}

// ========== 录像管理 ==========

// StartRecording 开始录像
func (e *SurveillanceEngine) StartRecording(cameraID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	camera, exists := e.cameras[cameraID]
	if !exists {
		return ErrCameraNotFound
	}

	camera.Status = CameraStatusRecording
	camera.UpdatedAt = time.Now()

	recording := &Recording{
		ID:        uuid.New().String(),
		CameraID:  cameraID,
		StartTime: time.Now(),
	}

	e.recordings[cameraID] = append(e.recordings[cameraID], recording)
	e.logger.Info("录像已开始", zap.String("camera", cameraID))
	return nil
}

// StopRecording 停止录像
func (e *SurveillanceEngine) StopRecording(cameraID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	camera, exists := e.cameras[cameraID]
	if !exists {
		return ErrCameraNotFound
	}

	camera.Status = CameraStatusOnline
	camera.UpdatedAt = time.Now()

	recordings := e.recordings[cameraID]
	if len(recordings) > 0 {
		last := recordings[len(recordings)-1]
		if last.EndTime.IsZero() {
			last.EndTime = time.Now()
			last.Duration = int(last.EndTime.Sub(last.StartTime).Seconds())
		}
	}

	e.logger.Info("录像已停止", zap.String("camera", cameraID))
	return nil
}

// GetRecordings 获取录像列表
func (e *SurveillanceEngine) GetRecordings(query RecordingQuery) []*Recording {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Recording

	if query.CameraID != "" {
		// 查询特定摄像头
		recordings := e.recordings[query.CameraID]
		for _, rec := range recordings {
			if matchRecording(rec, query) {
				result = append(result, rec)
			}
		}
	} else {
		// 查询所有摄像头
		for _, recordings := range e.recordings {
			for _, rec := range recordings {
				if matchRecording(rec, query) {
					result = append(result, rec)
				}
			}
		}
	}

	// 分页
	start := (query.Page - 1) * query.PageSize
	if start >= len(result) {
		return nil
	}
	end := start + query.PageSize
	if end > len(result) {
		end = len(result)
	}
	return result[start:end]
}

// matchRecording 匹配录像记录
func matchRecording(rec *Recording, query RecordingQuery) bool {
	if query.StartTime != nil && rec.StartTime.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && rec.EndTime.After(*query.EndTime) {
		return false
	}
	if query.HasEvents != nil && rec.HasEvents != *query.HasEvents {
		return false
	}
	return true
}

// ========== 事件管理 ==========

// ReportEvent 上报事件
func (e *SurveillanceEngine) ReportEvent(event *Event) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.cameras[event.CameraID]; !exists {
		return ErrCameraNotFound
	}

	if event.ID == "" {
		event.ID = uuid.New().String()
	}

	camera := e.cameras[event.CameraID]
	event.CameraName = camera.Name
	event.Timestamp = time.Now()

	e.events[event.CameraID] = append(e.events[event.CameraID], event)

	// 更新关联的录像
	recordings := e.recordings[event.CameraID]
	for _, rec := range recordings {
		if event.Timestamp.After(rec.StartTime) && (rec.EndTime.IsZero() || event.Timestamp.Before(rec.EndTime)) {
			rec.HasEvents = true
			rec.EventCount++
		}
	}

	e.logger.Info("事件已上报",
		zap.String("camera", event.CameraID),
		zap.String("type", string(event.Type)),
		zap.Float64("confidence", event.Confidence))
	return nil
}

// GetEvents 获取事件列表
func (e *SurveillanceEngine) GetEvents(query EventQuery) []*Event {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*Event

	if query.CameraID != "" {
		events := e.events[query.CameraID]
		for _, evt := range events {
			if matchEvent(evt, query) {
				result = append(result, evt)
			}
		}
	} else {
		for _, events := range e.events {
			for _, evt := range events {
				if matchEvent(evt, query) {
					result = append(result, evt)
				}
			}
		}
	}

	// 分页
	start := (query.Page - 1) * query.PageSize
	if start >= len(result) {
		return nil
	}
	end := start + query.PageSize
	if end > len(result) {
		end = len(result)
	}
	return result[start:end]
}

// matchEvent 匹配事件
func matchEvent(event *Event, query EventQuery) bool {
	if query.StartTime != nil && event.Timestamp.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && event.Timestamp.After(*query.EndTime) {
		return false
	}
	if query.MinConf != nil && event.Confidence < *query.MinConf {
		return false
	}
	if query.Handled != nil && event.Handled != *query.Handled {
		return false
	}
	if len(query.Types) > 0 {
		found := false
		for _, t := range query.Types {
			if event.Type == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// GetTimeline 获取时间线数据
func (e *SurveillanceEngine) GetTimeline(cameraID string, date time.Time) (*TimelineData, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, exists := e.cameras[cameraID]; !exists {
		return nil, ErrCameraNotFound
	}

	camera := e.cameras[cameraID]
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	timeline := &TimelineData{
		Date:       date,
		CameraID:   cameraID,
		CameraName: camera.Name,
	}

	// 收集录像片段
	for _, rec := range e.recordings[cameraID] {
		if rec.StartTime.Before(endOfDay) && (rec.EndTime.IsZero() || rec.EndTime.After(startOfDay)) {
			timeline.Recordings = append(timeline.Recordings, TimelineSegment{
				StartTime: rec.StartTime,
				EndTime:   rec.EndTime,
				HasEvents: rec.HasEvents,
			})
		}
	}

	// 收集事件
	for _, evt := range e.events[cameraID] {
		if evt.Timestamp.After(startOfDay) && evt.Timestamp.Before(endOfDay) {
			level := AlertLevelInfo
			if evt.Confidence > 0.9 {
				level = AlertLevelWarning
			}
			timeline.Events = append(timeline.Events, TimelineEvent{
				Timestamp:  evt.Timestamp,
				Type:       evt.Type,
				Level:      level,
				Confidence: evt.Confidence,
				Thumbnail:  evt.SnapshotURL,
			})
		}
	}

	return timeline, nil
}

// GetSystemStatus 获取系统状态
func (e *SurveillanceEngine) GetSystemStatus() *SystemStatus {
	e.mu.RLock()
	defer e.mu.RUnlock()

	status := &SystemStatus{
		TotalCameras: len(e.cameras),
	}

	for _, camera := range e.cameras {
		if camera.Status == CameraStatusOnline || camera.Status == CameraStatusRecording {
			status.OnlineCameras++
		}
		if camera.Status == CameraStatusRecording {
			status.RecordingCount++
		}
	}

	totalEvents := 0
	for _, events := range e.events {
		totalEvents += len(events)
	}
	status.TotalEvents = totalEvents

	return status
}

// Stop 停止引擎
func (e *SurveillanceEngine) Stop() {
	e.cancel()
	e.logger.Info("监控引擎已停止")
}
