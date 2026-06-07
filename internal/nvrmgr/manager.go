// Package nvrmgr 提供 NVR 视频管理核心业务逻辑
package nvrmgr

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// Manager NVR 管理器
type Manager struct {
	mu           sync.RWMutex
	cameras      map[string]*Camera
	recordings   []*Recording
	motionRules  map[string][]*MotionRule
	motionEvents []*MotionEvent
	alerts       []*Alert
	storagePlans map[string]*StoragePlan
	recordingID  int64
	motionID     int64
	alertID      int64
	planID       int64
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		cameras:      make(map[string]*Camera),
		recordings:   make([]*Recording, 0),
		motionRules:  make(map[string][]*MotionRule),
		motionEvents: make([]*MotionEvent, 0),
		alerts:       make([]*Alert, 0),
		storagePlans: make(map[string]*StoragePlan),
	}

	// 预置 3 种存储计划
	m.initDefaultStoragePlans()

	return m
}

// initDefaultStoragePlans 初始化默认存储计划
func (m *Manager) initDefaultStoragePlans() {
	defaultPlans := []*StoragePlan{
		{
			ID:            "plan-7d",
			Name:          "7天存储",
			RetentionDays: 7,
			MaxSize:       100 * 1024 * 1024 * 1024, // 100GB
			Quality:       "high",
			Schedule:      "24x7",
		},
		{
			ID:            "plan-30d",
			Name:          "30天存储",
			RetentionDays: 30,
			MaxSize:       500 * 1024 * 1024 * 1024, // 500GB
			Quality:       "medium",
			Schedule:      "24x7",
		},
		{
			ID:            "plan-90d",
			Name:          "90天存储",
			RetentionDays: 90,
			MaxSize:       1024 * 1024 * 1024 * 1024, // 1TB
			Quality:       "low",
			Schedule:      "24x7",
		},
	}

	for _, plan := range defaultPlans {
		plan.CreatedAt = time.Now()
		plan.UpdatedAt = time.Now()
		m.storagePlans[plan.ID] = plan
	}
}

// ========== 摄像头管理 ==========

// AddCamera 添加摄像头
func (m *Manager) AddCamera(cam *Camera) (*Camera, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cam.ID == "" {
		return nil, fmt.Errorf("摄像头 ID 不能为空")
	}
	if _, exists := m.cameras[cam.ID]; exists {
		return nil, fmt.Errorf("摄像头 %s 已存在", cam.ID)
	}

	cam.Status = CameraStatusOffline
	cam.Enabled = true
	cam.CreatedAt = time.Now()
	cam.UpdatedAt = time.Now()

	m.cameras[cam.ID] = cam
	log.Printf("[nvrmgr] 摄像头已添加: %s (%s)", cam.ID, cam.Name)
	return cam, nil
}

// UpdateCamera 更新摄像头
func (m *Manager) UpdateCamera(id string, cam *Camera) (*Camera, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.cameras[id]
	if !exists {
		return nil, fmt.Errorf("摄像头 %s 不存在", id)
	}

	cam.ID = id
	cam.CreatedAt = existing.CreatedAt
	cam.UpdatedAt = time.Now()
	cam.LastSeen = existing.LastSeen

	m.cameras[id] = cam
	log.Printf("[nvrmgr] 摄像头已更新: %s", id)
	return cam, nil
}

// DeleteCamera 删除摄像头
func (m *Manager) DeleteCamera(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[id]; !exists {
		return fmt.Errorf("摄像头 %s 不存在", id)
	}

	delete(m.cameras, id)
	delete(m.motionRules, id)

	// 删除相关录像
	filtered := make([]*Recording, 0)
	for _, rec := range m.recordings {
		if rec.CameraID != id {
			filtered = append(filtered, rec)
		}
	}
	m.recordings = filtered

	log.Printf("[nvrmgr] 摄像头已删除: %s", id)
	return nil
}

// GetCamera 获取摄像头
func (m *Manager) GetCamera(id string) (*Camera, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cam, exists := m.cameras[id]
	if !exists {
		return nil, fmt.Errorf("摄像头 %s 不存在", id)
	}
	return cam, nil
}

// ListCameras 列出所有摄像头
func (m *Manager) ListCameras() ([]Camera, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cameras := make([]Camera, 0, len(m.cameras))
	for _, cam := range m.cameras {
		cameras = append(cameras, *cam)
	}
	return cameras, nil
}

// ========== 录像管理 ==========

// StartRecording 开始录像
func (m *Manager) StartRecording(cameraID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[cameraID]; !exists {
		return fmt.Errorf("摄像头 %s 不存在", cameraID)
	}

	m.recordingID++
	recording := &Recording{
		ID:        fmt.Sprintf("rec-%d", m.recordingID),
		CameraID:  cameraID,
		StartTime: time.Now(),
		FilePath:  fmt.Sprintf("/recordings/%s/%s.mp4", cameraID, fmt.Sprintf("rec-%d", m.recordingID)),
		CreatedAt: time.Now(),
	}

	m.recordings = append(m.recordings, recording)
	log.Printf("[nvrmgr] 开始录像: %s (摄像头: %s)", recording.ID, cameraID)
	return nil
}

// StopRecording 停止录像
func (m *Manager) StopRecording(cameraID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[cameraID]; !exists {
		return fmt.Errorf("摄像头 %s 不存在", cameraID)
	}

	// 查找最近的未结束录像
	for i := len(m.recordings) - 1; i >= 0; i-- {
		rec := m.recordings[i]
		if rec.CameraID == cameraID && rec.EndTime.IsZero() {
			rec.EndTime = time.Now()
			rec.Duration = int64(rec.EndTime.Sub(rec.StartTime).Seconds())
			log.Printf("[nvrmgr] 停止录像: %s (时长: %d秒)", rec.ID, rec.Duration)
			return nil
		}
	}

	return fmt.Errorf("摄像头 %s 没有正在录制的录像", cameraID)
}

// GetRecordings 获取录像列表（分页）
func (m *Manager) GetRecordings(cameraID string, from, to time.Time, page, pageSize int) ([]Recording, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filtered []Recording
	for _, rec := range m.recordings {
		if cameraID != "" && rec.CameraID != cameraID {
			continue
		}
		if !from.IsZero() && rec.StartTime.Before(from) {
			continue
		}
		if !to.IsZero() && rec.StartTime.After(to) {
			continue
		}
		filtered = append(filtered, *rec)
	}

	total := len(filtered)

	// 按开始时间倒序排序
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].StartTime.After(filtered[j].StartTime)
	})

	// 分页
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	if start >= total {
		return []Recording{}, total, nil
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	return filtered[start:end], total, nil
}

// DeleteRecording 删除录像
func (m *Manager) DeleteRecording(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, rec := range m.recordings {
		if rec.ID == id {
			m.recordings = append(m.recordings[:i], m.recordings[i+1:]...)
			log.Printf("[nvrmgr] 录像已删除: %s", id)
			return nil
		}
	}

	return fmt.Errorf("录像 %s 不存在", id)
}

// ========== 时间线 ==========

// GetTimeline 获取时间线
func (m *Manager) GetTimeline(cameraID string, date time.Time) (*Timeline, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.cameras[cameraID]; !exists {
		return nil, fmt.Errorf("摄像头 %s 不存在", cameraID)
	}

	dateStr := date.Format("2006-01-02")
	dayStart := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
	dayEnd := dayStart.Add(24 * time.Hour)

	// 收集当天的所有录像片段
	segments := make([]TimelineSegment, 0)
	for _, rec := range m.recordings {
		if rec.CameraID != cameraID {
			continue
		}

		recStart := rec.StartTime
		recEnd := rec.EndTime
		if recEnd.IsZero() {
			recEnd = time.Now()
		}

		// 检查是否与当天有交集
		if recEnd.Before(dayStart) || recStart.After(dayEnd) {
			continue
		}

		// 调整到当天范围内
		segStart := recStart
		if segStart.Before(dayStart) {
			segStart = dayStart
		}
		segEnd := recEnd
		if segEnd.After(dayEnd) {
			segEnd = dayEnd
		}

		segments = append(segments, TimelineSegment{
			StartTime:    segStart,
			EndTime:      segEnd,
			HasRecording: true,
			HasMotion:    rec.HasMotion,
		})
	}

	// 检查移动侦测事件
	for _, evt := range m.motionEvents {
		if evt.CameraID != cameraID {
			continue
		}
		evtTime := evt.Timestamp
		if evtTime.Before(dayStart) || evtTime.After(dayEnd) {
			continue
		}

		// 标记对应片段有移动侦测
		for i := range segments {
			if !evtTime.Before(segments[i].StartTime) && !evtTime.After(segments[i].EndTime) {
				segments[i].HasMotion = true
			}
		}
	}

	// 按开始时间排序
	sort.Slice(segments, func(i, j int) bool {
		return segments[i].StartTime.Before(segments[j].StartTime)
	})

	return &Timeline{
		CameraID: cameraID,
		Date:     dateStr,
		Segments: segments,
	}, nil
}

// ========== 移动侦测 ==========

// AddMotionRule 添加移动侦测规则
func (m *Manager) AddMotionRule(cameraID, zone string, sensitivity float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[cameraID]; !exists {
		return fmt.Errorf("摄像头 %s 不存在", cameraID)
	}

	if sensitivity < 0 || sensitivity > 1 {
		return fmt.Errorf("灵敏度必须在 0.0-1.0 之间")
	}

	rule := &MotionRule{
		CameraID:    cameraID,
		Zone:        zone,
		Sensitivity: sensitivity,
		Enabled:     true,
		CreatedAt:   time.Now(),
	}

	m.motionRules[cameraID] = append(m.motionRules[cameraID], rule)
	log.Printf("[nvrmgr] 移动侦测规则已添加: 摄像头=%s, 区域=%s, 灵敏度=%.2f", cameraID, zone, sensitivity)
	return nil
}

// GetMotionEvents 获取移动侦测事件
func (m *Manager) GetMotionEvents(cameraID string, from, to time.Time) ([]MotionEvent, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var events []MotionEvent
	for _, evt := range m.motionEvents {
		if cameraID != "" && evt.CameraID != cameraID {
			continue
		}
		if !from.IsZero() && evt.Timestamp.Before(from) {
			continue
		}
		if !to.IsZero() && evt.Timestamp.After(to) {
			continue
		}
		events = append(events, *evt)
	}

	// 按时间倒序排序
	sort.Slice(events, func(i, j int) bool {
		return events[i].Timestamp.After(events[j].Timestamp)
	})

	return events, nil
}

// ========== 告警管理 ==========

// CreateAlert 创建告警
func (m *Manager) CreateAlert(alert *Alert) (*Alert, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.cameras[alert.CameraID]; !exists {
		return nil, fmt.Errorf("摄像头 %s 不存在", alert.CameraID)
	}

	m.alertID++
	alert.ID = fmt.Sprintf("alert-%d", m.alertID)
	alert.Timestamp = time.Now()
	alert.Acknowledged = false

	m.alerts = append(m.alerts, alert)
	log.Printf("[nvrmgr] 告警已创建: %s (摄像头: %s, 类型: %s)", alert.ID, alert.CameraID, alert.Type)
	return alert, nil
}

// ListAlerts 获取告警列表
func (m *Manager) ListAlerts(unread bool) ([]Alert, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var alerts []Alert
	for _, alert := range m.alerts {
		if unread && alert.Acknowledged {
			continue
		}
		alerts = append(alerts, *alert)
	}

	// 按时间倒序排序
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].Timestamp.After(alerts[j].Timestamp)
	})

	return alerts, nil
}

// AcknowledgeAlert 确认告警
func (m *Manager) AcknowledgeAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.alerts {
		if alert.ID == id {
			alert.Acknowledged = true
			alert.AckedAt = time.Now()
			log.Printf("[nvrmgr] 告警已确认: %s", id)
			return nil
		}
	}

	return fmt.Errorf("告警 %s 不存在", id)
}

// ========== 存储策略 ==========

// CreateStoragePlan 创建存储策略
func (m *Manager) CreateStoragePlan(plan *StoragePlan) (*StoragePlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if plan.ID == "" {
		return nil, fmt.Errorf("存储策略 ID 不能为空")
	}
	if _, exists := m.storagePlans[plan.ID]; exists {
		return nil, fmt.Errorf("存储策略 %s 已存在", plan.ID)
	}

	m.planID++
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()

	m.storagePlans[plan.ID] = plan
	log.Printf("[nvrmgr] 存储策略已创建: %s (%s)", plan.ID, plan.Name)
	return plan, nil
}

// ApplyStoragePlan 应用存储策略到摄像头
func (m *Manager) ApplyStoragePlan(planID string, cameraIDs []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, exists := m.storagePlans[planID]
	if !exists {
		return fmt.Errorf("存储策略 %s 不存在", planID)
	}

	// 验证摄像头存在
	for _, camID := range cameraIDs {
		if _, exists := m.cameras[camID]; !exists {
			return fmt.Errorf("摄像头 %s 不存在", camID)
		}
	}

	plan.Cameras = cameraIDs
	plan.UpdatedAt = time.Now()

	log.Printf("[nvrmgr] 存储策略 %s 已应用到 %d 个摄像头", planID, len(cameraIDs))
	return nil
}

// CleanupOldRecordings 清理过期录像
func (m *Manager) CleanupOldRecordings() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cleaned := 0
	now := time.Now()

	// 按摄像头分组，检查保留策略
	cameraRecordings := make(map[string][]*Recording)
	for _, rec := range m.recordings {
		cameraRecordings[rec.CameraID] = append(cameraRecordings[rec.CameraID], rec)
	}

	for camID, recs := range cameraRecordings {
		// 查找该摄像头的存储策略
		retentionDays := 30 // 默认保留 30 天
		for _, plan := range m.storagePlans {
			for _, pid := range plan.Cameras {
				if pid == camID {
					retentionDays = plan.RetentionDays
					break
				}
			}
		}

		// 删除过期录像
		var kept []*Recording
		for _, rec := range recs {
			age := now.Sub(rec.StartTime)
			if age.Hours()/24 > float64(retentionDays) {
				cleaned++
				log.Printf("[nvrmgr] 清理过期录像: %s (摄像头: %s, 录制时间: %s)", rec.ID, camID, rec.StartTime.Format("2006-01-02"))
			} else {
				kept = append(kept, rec)
			}
		}
		cameraRecordings[camID] = kept
	}

	// 重建录像列表
	newRecordings := make([]*Recording, 0)
	for _, recs := range cameraRecordings {
		newRecordings = append(newRecordings, recs...)
	}
	m.recordings = newRecordings

	log.Printf("[nvrmgr] 清理完成，删除 %d 条过期录像", cleaned)
	return cleaned, nil
}

// GetStorageUsage 获取存储使用情况
func (m *Manager) GetStorageUsage() (map[string]int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	usage := make(map[string]int64)
	var total int64
	for _, rec := range m.recordings {
		usage[rec.CameraID] += rec.Size
		total += rec.Size
	}
	usage["total"] = total

	return usage, nil
}

// GetCameraStatus 获取摄像头状态统计
func (m *Manager) GetCameraStatus() (map[string]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	status := map[string]int{
		"total":   len(m.cameras),
		"online":  0,
		"offline": 0,
		"error":   0,
	}

	for _, cam := range m.cameras {
		switch cam.Status {
		case CameraStatusOnline:
			status["online"]++
		case CameraStatusOffline:
			status["offline"]++
		case CameraStatusError:
			status["error"]++
		}
	}

	return status, nil
}
