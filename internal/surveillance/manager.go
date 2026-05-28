package surveillance

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 视频监控管理器.
type Manager struct {
	cameras     map[string]*Camera
	recordings  map[string]*RecordingJob
	events      []*SurveillanceEvent
	schedules   map[string]*RecordingSchedule
	streams     map[string]*StreamSession
	motionCfgs  map[string]*MotionDetectionConfig
	exportJobs  map[string]*ExportJob
	storageQuota map[string]*StorageQuota
	recordingDir string
	maxEvents    int
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewManager 创建监控管理器.
func NewManager(recordingDir string) (*Manager, error) {
	if recordingDir == "" {
		recordingDir = "/var/lib/nas-os/surveillance"
	}

	// 确保录制目录存在
	if err := os.MkdirAll(recordingDir, 0755); err != nil {
		return nil, fmt.Errorf("创建录制目录失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		cameras:      make(map[string]*Camera),
		recordings:   make(map[string]*RecordingJob),
		events:       make([]*SurveillanceEvent, 0),
		schedules:    make(map[string]*RecordingSchedule),
		streams:      make(map[string]*StreamSession),
		motionCfgs:   make(map[string]*MotionDetectionConfig),
		exportJobs:   make(map[string]*ExportJob),
		storageQuota: make(map[string]*StorageQuota),
		recordingDir: recordingDir,
		maxEvents:    10000,
		ctx:          ctx,
		cancel:       cancel,
	}

	// 启动后台任务
	go m.healthCheckLoop()
	go m.cleanupLoop()

	return m, nil
}

// Close 关闭管理器.
func (m *Manager) Close() {
	m.cancel()
}

// healthCheckLoop 定期检查摄像头状态.
func (m *Manager) healthCheckLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkCameraHealth()
		}
	}
}

// cleanupLoop 定期清理过期数据.
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.cleanupExpiredData()
		}
	}
}

// checkCameraHealth 检查摄像头健康状态.
func (m *Manager) checkCameraHealth() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, cam := range m.cameras {
		if !cam.Enabled {
			continue
		}

		// 模拟健康检查 - 实际应通过 RTSP/ONVIF 探测
		oldStatus := cam.Status
		cam.Status = CameraStatusOnline
		cam.UpdatedAt = time.Now()

		if oldStatus != cam.Status {
			m.addEvent(cam.ID, EventTypeCameraOnline, EventSeverityInfo,
				fmt.Sprintf("摄像头 %s 已上线", cam.Name))
		}
	}
}

// cleanupExpiredData 清理过期数据.
func (m *Manager) cleanupExpiredData() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 保留最近的事件
	if len(m.events) > m.maxEvents {
		m.events = m.events[len(m.events)-m.maxEvents:]
	}

	// 清理过期的导出任务
	for id, job := range m.exportJobs {
		if job.Status == "completed" || job.Status == "failed" {
			if time.Since(job.CreatedAt) > 24*time.Hour {
				delete(m.exportJobs, id)
			}
		}
	}
}

// addEvent 添加监控事件.
func (m *Manager) addEvent(cameraID string, eventType EventType, severity EventSeverity, message string) {
	event := &SurveillanceEvent{
		ID:        uuid.New().String(),
		CameraID:  cameraID,
		Type:      eventType,
		Severity:  severity,
		Message:   message,
		Timestamp: time.Now(),
	}
	m.events = append(m.events, event)
	log.Printf("[Surveillance] Event: %s - %s", eventType, message)
}

// ==================== 摄像头管理 ====================

// AddCamera 添加摄像头.
func (m *Manager) AddCamera(cam *Camera) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cam.ID == "" {
		cam.ID = uuid.New().String()
	}

	if _, exists := m.cameras[cam.ID]; exists {
		return fmt.Errorf("摄像头 %s 已存在", cam.ID)
	}

	// 默认状态为离线，除非已明确设置为在线
	if cam.Status != CameraStatusOnline {
		cam.Status = CameraStatusOffline
	}
	cam.CreatedAt = time.Now()
	cam.UpdatedAt = time.Now()

	m.cameras[cam.ID] = cam
	return nil
}

// UpdateCamera 更新摄像头.
func (m *Manager) UpdateCamera(cam *Camera) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[cam.ID]; !exists {
		return fmt.Errorf("摄像头 %s 不存在", cam.ID)
	}

	cam.UpdatedAt = time.Now()
	m.cameras[cam.ID] = cam
	return nil
}

// DeleteCamera 删除摄像头.
func (m *Manager) DeleteCamera(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[id]; !exists {
		return fmt.Errorf("摄像头 %s 不存在", id)
	}

	// 删除相关的录制、配置等
	delete(m.cameras, id)
	delete(m.motionCfgs, id)
	delete(m.storageQuota, id)

	return nil
}

// GetCamera 获取摄像头.
func (m *Manager) GetCamera(id string) (*Camera, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cam, exists := m.cameras[id]
	if !exists {
		return nil, fmt.Errorf("摄像头 %s 不存在", id)
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

// DiscoverCameras ONVIF 自动发现摄像头.
func (m *Manager) DiscoverCameras() ([]ONVIFDiscoveryResult, error) {
	// 实际实现应该使用 ONVIF WS-Discovery
	// 这里返回示例数据
	results := []ONVIFDiscoveryResult{
		{
			IPAddress:    "192.168.1.100",
			Port:         80,
			Manufacturer: "Hikvision",
			Model:        "DS-2CD2143G2-I",
			SerialNumber: "DS2CD2143G2I20230101AAWR",
			Endpoint:     "http://192.168.1.100/onvif/device_service",
		},
	}

	return results, nil
}

// ==================== 录制管理 ====================

// StartRecording 开始录制.
func (m *Manager) StartRecording(cameraID string, mode RecordingMode) (*RecordingJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cam, exists := m.cameras[cameraID]
	if !exists {
		return nil, fmt.Errorf("摄像头 %s 不存在", cameraID)
	}

	if cam.Status != CameraStatusOnline {
		return nil, fmt.Errorf("摄像头 %s 不在线", cameraID)
	}

	// 创建录制目录
	camDir := filepath.Join(m.recordingDir, cameraID)
	if err := os.MkdirAll(camDir, 0755); err != nil {
		return nil, fmt.Errorf("创建录制目录失败: %w", err)
	}

	job := &RecordingJob{
		ID:        uuid.New().String(),
		CameraID:  cameraID,
		Mode:      mode,
		StartTime: time.Now(),
		FilePath:  filepath.Join(camDir, fmt.Sprintf("%s.mp4", time.Now().Format("20060102_150405"))),
		Status:    "recording",
		CreatedAt: time.Now(),
	}

	m.recordings[job.ID] = job
	m.addEvent(cameraID, EventTypeRecordingStart, EventSeverityInfo,
		fmt.Sprintf("开始录制 %s", job.ID))

	return job, nil
}

// StopRecording 停止录制.
func (m *Manager) StopRecording(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, exists := m.recordings[jobID]
	if !exists {
		return fmt.Errorf("录制任务 %s 不存在", jobID)
	}

	if job.Status != "recording" {
		return fmt.Errorf("录制任务 %s 不在录制状态", jobID)
	}

	now := time.Now()
	job.EndTime = &now
	job.Status = "completed"
	job.Duration = int64(now.Sub(job.StartTime).Seconds())

	m.addEvent(job.CameraID, EventTypeRecordingStop, EventSeverityInfo,
		fmt.Sprintf("停止录制 %s", jobID))

	return nil
}

// GetRecordings 获取录制列表.
func (m *Manager) GetRecordings(cameraID string) []*RecordingJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*RecordingJob, 0)
	for _, job := range m.recordings {
		if cameraID == "" || job.CameraID == cameraID {
			result = append(result, job)
		}
	}

	return result
}

// ==================== 事件管理 ====================

// GetEvents 获取事件列表.
func (m *Manager) GetEvents(cameraID string, limit int) []*SurveillanceEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SurveillanceEvent, 0)
	for i := len(m.events) - 1; i >= 0; i-- {
		if cameraID == "" || m.events[i].CameraID == cameraID {
			result = append(result, m.events[i])
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	return result
}

// AddEvent 添加自定义事件.
func (m *Manager) AddEvent(cameraID, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.addEvent(cameraID, EventTypeCustom, EventSeverityInfo, message)
}

// ==================== 流媒体管理 ====================

// StartStream 开始流媒体会话.
func (m *Manager) StartStream(cameraID, protocol, clientID string) (*StreamSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cam, exists := m.cameras[cameraID]
	if !exists {
		return nil, fmt.Errorf("摄像头 %s 不存在", cameraID)
	}

	if cam.Status != CameraStatusOnline {
		return nil, fmt.Errorf("摄像头 %s 不在线", cameraID)
	}

	session := &StreamSession{
		ID:        uuid.New().String(),
		CameraID:  cameraID,
		ClientID:  clientID,
		Protocol:  protocol,
		URL:       fmt.Sprintf("/api/v1/surveillance/stream/%s/%s", cameraID, protocol),
		StartTime: time.Now(),
		Active:    true,
	}

	m.streams[session.ID] = session
	return session, nil
}

// StopStream 停止流媒体会话.
func (m *Manager) StopStream(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, exists := m.streams[sessionID]
	if !exists {
		return fmt.Errorf("流会话 %s 不存在", sessionID)
	}

	session.Active = false
	delete(m.streams, sessionID)
	return nil
}

// GetActiveStreams 获取活跃流.
func (m *Manager) GetActiveStreams() []*StreamSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	streams := make([]*StreamSession, 0)
	for _, s := range m.streams {
		if s.Active {
			streams = append(streams, s)
		}
	}

	return streams
}

// ==================== 移动侦测 ====================

// SetMotionDetection 设置移动侦测配置.
func (m *Manager) SetMotionDetection(cfg *MotionDetectionConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[cfg.CameraID]; !exists {
		return fmt.Errorf("摄像头 %s 不存在", cfg.CameraID)
	}

	m.motionCfgs[cfg.CameraID] = cfg
	return nil
}

// GetMotionDetection 获取移动侦测配置.
func (m *Manager) GetMotionDetection(cameraID string) (*MotionDetectionConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, exists := m.motionCfgs[cameraID]
	if !exists {
		return nil, fmt.Errorf("摄像头 %s 的移动侦测配置不存在", cameraID)
	}

	return cfg, nil
}

// ==================== 回放系统 ====================

// QueryPlayback 查询回放.
func (m *Manager) GetPlayback(query PlaybackQuery) ([]PlaybackSegment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 实际应查询文件系统或数据库
	segments := make([]PlaybackSegment, 0)

	for _, job := range m.recordings {
		if job.CameraID != query.CameraID {
			continue
		}
		if job.StartTime.Before(query.EndTime) && (job.EndTime == nil || job.EndTime.After(query.StartTime)) {
			seg := PlaybackSegment{
				ID:        job.ID,
				CameraID:  job.CameraID,
				StartTime: job.StartTime,
				FilePath:  job.FilePath,
				FileSize:  job.FileSize,
			}
			if job.EndTime != nil {
				seg.EndTime = *job.EndTime
			}
			segments = append(segments, seg)
		}
	}

	return segments, nil
}

// ==================== 导出系统 ====================

// CreateExport 创建导出任务.
func (m *Manager) CreateExport(req ExportRequest) (*ExportJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[req.CameraID]; !exists {
		return nil, fmt.Errorf("摄像头 %s 不存在", req.CameraID)
	}

	job := &ExportJob{
		ID:        uuid.New().String(),
		CameraID:  req.CameraID,
		StartTime: req.StartTime,
		EndTime:   req.EndTime,
		Format:    req.Format,
		Status:    "pending",
		Progress:  0,
		CreatedAt: time.Now(),
	}

	m.exportJobs[job.ID] = job

	// 异步处理导出
	go m.processExport(job.ID)

	return job, nil
}

// processExport 处理导出任务.
func (m *Manager) processExport(jobID string) {
	m.mu.Lock()
	job, exists := m.exportJobs[jobID]
	if !exists {
		m.mu.Unlock()
		return
	}
	job.Status = "processing"
	m.mu.Unlock()

	// 模拟导出处理
	time.Sleep(2 * time.Second)

	m.mu.Lock()
	defer m.mu.Unlock()

	job.Status = "completed"
	job.Progress = 100
	now := time.Now()
	job.CompletedAt = &now
	job.FilePath = filepath.Join(m.recordingDir, "exports", fmt.Sprintf("%s.%s", jobID, job.Format))
	job.FileSize = 1024 * 1024 * 100 // 100MB 示例
}

// GetExportJob 获取导出任务.
func (m *Manager) GetExportJob(jobID string) (*ExportJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.exportJobs[jobID]
	if !exists {
		return nil, fmt.Errorf("导出任务 %s 不存在", jobID)
	}

	return job, nil
}

// ==================== 存储配额 ====================

// SetStorageQuota 设置存储配额.
func (m *Manager) SetStorageQuota(quota *StorageQuota) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.storageQuota[quota.CameraID] = quota
	return nil
}

// GetStorageQuota 获取存储配额.
func (m *Manager) GetStorageQuota(cameraID string) (*StorageQuota, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quota, exists := m.storageQuota[cameraID]
	if !exists {
		return &StorageQuota{
			CameraID:      cameraID,
			MaxSizeGB:     100,
			RetentionDays: 30,
			AutoDelete:    true,
		}, nil
	}

	return quota, nil
}

// ==================== 统计 ====================

// GetStats 获取监控统计.
func (m *Manager) GetStats() *SurveillanceStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &SurveillanceStats{
		TotalCameras: len(m.cameras),
	}

	for _, cam := range m.cameras {
		switch cam.Status {
		case CameraStatusOnline:
			stats.OnlineCameras++
		case CameraStatusOffline:
			stats.OfflineCameras++
		}
	}

	for _, job := range m.recordings {
		if job.Status == "recording" {
			stats.ActiveRecordings++
		}
		stats.TotalRecordings++
	}

	for _, s := range m.streams {
		if s.Active {
			stats.ActiveStreams++
		}
	}

	// 今日事件数
	today := time.Now().Truncate(24 * time.Hour)
	for _, event := range m.events {
		if event.Timestamp.After(today) {
			stats.TodayEvents++
		}
	}

	stats.StorageUsedGB = 50.5  // 示例值
	stats.StorageTotalGB = 500.0 // 示例值

	return stats
}

// ==================== PTZ 控制 ====================

// SendPTZCommand 发送 PTZ 控制命令.
func (m *Manager) SendPTZCommand(cmd PTZCommand) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cam, exists := m.cameras[cmd.CameraID]
	if !exists {
		return fmt.Errorf("摄像头 %s 不存在", cmd.CameraID)
	}

	if cam.Status != CameraStatusOnline {
		return fmt.Errorf("摄像头 %s 不在线", cmd.CameraID)
	}

	// 实际应通过 ONVIF PTZ 服务发送命令
	log.Printf("[Surveillance] PTZ command: %s - %s", cmd.CameraID, cmd.Action)
	return nil
}

// ==================== 录制计划 ====================

// AddSchedule 添加录制计划.
func (m *Manager) AddSchedule(schedule *RecordingSchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[schedule.CameraID]; !exists {
		return fmt.Errorf("摄像头 %s 不存在", schedule.CameraID)
	}

	if schedule.ID == "" {
		schedule.ID = uuid.New().String()
	}

	schedule.CreatedAt = time.Now()
	m.schedules[schedule.ID] = schedule
	return nil
}

// DeleteSchedule 删除录制计划.
func (m *Manager) DeleteSchedule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.schedules[id]; !exists {
		return fmt.Errorf("录制计划 %s 不存在", id)
	}

	delete(m.schedules, id)
	return nil
}

// ListSchedules 列出录制计划.
func (m *Manager) ListSchedules(cameraID string) []*RecordingSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	schedules := make([]*RecordingSchedule, 0)
	for _, s := range m.schedules {
		if cameraID == "" || s.CameraID == cameraID {
			schedules = append(schedules, s)
		}
	}

	return schedules
}

// ReportMotion 报告移动侦测事件.
func (m *Manager) ReportMotion(event *MotionEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.ID == "" {
		event.ID = fmt.Sprintf("motion-%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	return nil
}

// GetTimeline 获取摄像头时间线.
func (m *Manager) GetTimeline(cameraID string, t time.Time) map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"camera_id": cameraID,
		"date":      t.Format("2006-01-02"),
		"events":    []interface{}{},
	}
}
