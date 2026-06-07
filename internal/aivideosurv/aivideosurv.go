// Package aivideosurv 提供 AI 视频监控增强功能
package aivideosurv

import (
	"fmt"
	"sync"
	"time"
)

// SurveillanceEngine AI视频监控引擎.
type SurveillanceEngine struct {
	mu      sync.RWMutex
	cameras map[string]*Camera
	events  []*DetectionEvent
	tracks  map[string]*ObjectTrack
	zones   map[string]*Zone
	alerts  []*BehaviorAlert
	config  *EngineConfig
}

// EngineConfig 引擎配置.
type EngineConfig struct {
	MaxEvents      int           `json:"max_events"`      // 最大事件数
	MaxAlerts      int           `json:"max_alerts"`      // 最大告警数
	TrackTimeout   time.Duration `json:"track_timeout"`   // 跟踪超时
	LoiterDuration time.Duration `json:"loiter_duration"` // 徘徊判定时长
	MinConfidence  float64       `json:"min_confidence"`  // 最小置信度
}

// DefaultEngineConfig 默认引擎配置.
func DefaultEngineConfig() *EngineConfig {
	return &EngineConfig{
		MaxEvents:      10000,
		MaxAlerts:      5000,
		TrackTimeout:   5 * time.Minute,
		LoiterDuration: 30 * time.Second,
		MinConfidence:  0.5,
	}
}

// NewEngine 创建监控引擎.
func NewEngine(config *EngineConfig) *SurveillanceEngine {
	if config == nil {
		config = DefaultEngineConfig()
	}
	return &SurveillanceEngine{
		cameras: make(map[string]*Camera),
		events:  make([]*DetectionEvent, 0),
		tracks:  make(map[string]*ObjectTrack),
		zones:   make(map[string]*Zone),
		alerts:  make([]*BehaviorAlert, 0),
		config:  config,
	}
}

// ========== 摄像头管理 ==========

// AddCamera 添加摄像头.
func (e *SurveillanceEngine) AddCamera(cam *Camera) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if cam.ID == "" {
		return ErrInvalidCamera
	}
	if _, exists := e.cameras[cam.ID]; exists {
		return ErrCameraAlreadyExists
	}

	now := time.Now()
	cam.CreatedAt = now
	cam.UpdatedAt = now
	if cam.Status == "" {
		cam.Status = CameraOffline
	}

	e.cameras[cam.ID] = cam
	return nil
}

// RemoveCamera 删除摄像头.
func (e *SurveillanceEngine) RemoveCamera(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.cameras[id]; !exists {
		return ErrCameraNotFound
	}

	delete(e.cameras, id)
	return nil
}

// GetCamera 获取摄像头信息.
func (e *SurveillanceEngine) GetCamera(id string) (*Camera, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cam, exists := e.cameras[id]
	if !exists {
		return nil, ErrCameraNotFound
	}
	return cam, nil
}

// UpdateCameraStatus 更新摄像头状态.
func (e *SurveillanceEngine) UpdateCameraStatus(id string, status CameraStatus) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	cam, exists := e.cameras[id]
	if !exists {
		return ErrCameraNotFound
	}

	cam.Status = status
	cam.UpdatedAt = time.Now()
	if status == CameraOnline {
		cam.LastActive = time.Now()
	}
	return nil
}

// ListCameras 列出所有摄像头.
func (e *SurveillanceEngine) ListCameras() []*Camera {
	e.mu.RLock()
	defer e.mu.RUnlock()

	cameras := make([]*Camera, 0, len(e.cameras))
	for _, cam := range e.cameras {
		cameras = append(cameras, cam)
	}
	return cameras
}

// ========== AI事件记录 ==========

// RecordEvent 记录检测事件.
func (e *SurveillanceEngine) RecordEvent(event *DetectionEvent) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 验证摄像头存在
	if _, exists := e.cameras[event.CameraID]; !exists {
		return ErrCameraNotFound
	}

	// 设置事件属性
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%d-%s", time.Now().UnixNano(), event.CameraID)
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// 检查置信度
	if event.Confidence < e.config.MinConfidence {
		return nil // 忽略低置信度事件
	}

	// 存储事件
	e.events = append(e.events, event)

	// 截断旧事件
	if len(e.events) > e.config.MaxEvents {
		e.events = e.events[len(e.events)-e.config.MaxEvents:]
	}

	// 更新跟踪
	e.updateTrack(event)

	// 检查行为告警
	e.checkBehaviorAlerts(event)

	return nil
}

// GetEvent 获取事件.
func (e *SurveillanceEngine) GetEvent(id string) (*DetectionEvent, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, event := range e.events {
		if event.ID == id {
			return event, nil
		}
	}
	return nil, ErrEventNotFound
}

// QueryEvents 查询事件.
func (e *SurveillanceEngine) QueryEvents(query EventQuery) []*DetectionEvent {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []*DetectionEvent
	for _, event := range e.events {
		if !matchEventQuery(event, query) {
			continue
		}
		results = append(results, event)
	}

	// 分页
	total := len(results)
	if query.Offset >= total {
		return []*DetectionEvent{}
	}
	end := total
	if query.Limit > 0 && query.Offset+query.Limit < total {
		end = query.Offset + query.Limit
	}
	return results[query.Offset:end]
}

// matchEventQuery 匹配事件查询条件.
func matchEventQuery(event *DetectionEvent, query EventQuery) bool {
	if query.CameraID != "" && event.CameraID != query.CameraID {
		return false
	}
	if query.Type != "" && event.Type != query.Type {
		return false
	}
	if !query.StartTime.IsZero() && event.Timestamp.Before(query.StartTime) {
		return false
	}
	if !query.EndTime.IsZero() && event.Timestamp.After(query.EndTime) {
		return false
	}
	if query.MinConf > 0 && event.Confidence < query.MinConf {
		return false
	}
	return true
}

// ========== 目标跟踪 ==========

// updateTrack 更新目标跟踪.
func (e *SurveillanceEngine) updateTrack(event *DetectionEvent) {
	trackID := findMatchingTrack(e, event)
	if trackID != "" {
		// 更新已有跟踪
		track := e.tracks[trackID]
		track.LastSeen = event.Timestamp
		track.Positions = append(track.Positions, TrackPoint{
			CameraID:  event.CameraID,
			Position:  event.Position,
			Timestamp: event.Timestamp,
		})
		// 添加摄像头（去重）
		found := false
		for _, cid := range track.CameraIDs {
			if cid == event.CameraID {
				found = true
				break
			}
		}
		if !found {
			track.CameraIDs = append(track.CameraIDs, event.CameraID)
		}
	} else {
		// 创建新跟踪
		trackID = fmt.Sprintf("trk-%d-%s", time.Now().UnixNano(), event.CameraID)
		e.tracks[trackID] = &ObjectTrack{
			ID:         trackID,
			ObjectType: event.Type,
			CameraIDs:  []string{event.CameraID},
			FirstSeen:  event.Timestamp,
			LastSeen:   event.Timestamp,
			Positions: []TrackPoint{
				{
					CameraID:  event.CameraID,
					Position:  event.Position,
					Timestamp: event.Timestamp,
				},
			},
			IsActive: true,
		}
	}
}

// findMatchingTrack 查找匹配的跟踪记录.
func findMatchingTrack(e *SurveillanceEngine, event *DetectionEvent) string {
	var bestMatch string
	bestTime := time.Time{}

	for id, track := range e.tracks {
		if track.ObjectType != event.Type || !track.IsActive {
			continue
		}
		// 时间窗口内
		if event.Timestamp.Sub(track.LastSeen) > e.config.TrackTimeout {
			continue
		}
		// 优先同一摄像头近距离
		if track.LastSeen.After(bestTime) {
			bestTime = track.LastSeen
			bestMatch = id
		}
	}
	return bestMatch
}

// GetTrack 获取跟踪记录.
func (e *SurveillanceEngine) GetTrack(id string) (*ObjectTrack, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	track, exists := e.tracks[id]
	if !exists {
		return nil, ErrTrackNotFound
	}
	return track, nil
}

// ListActiveTracks 列出活跃跟踪.
func (e *SurveillanceEngine) ListActiveTracks() []*ObjectTrack {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tracks := make([]*ObjectTrack, 0)
	for _, track := range e.tracks {
		if track.IsActive {
			tracks = append(tracks, track)
		}
	}
	return tracks
}

// CloseTrack 关闭跟踪.
func (e *SurveillanceEngine) CloseTrack(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	track, exists := e.tracks[id]
	if !exists {
		return ErrTrackNotFound
	}
	track.IsActive = false
	return nil
}

// CleanupStaleTracks 清理过期跟踪.
func (e *SurveillanceEngine) CleanupStaleTracks() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	count := 0
	for _, track := range e.tracks {
		if track.IsActive && now.Sub(track.LastSeen) > e.config.TrackTimeout {
			track.IsActive = false
			count++
		}
	}
	return count
}

// ========== 行为分析告警 ==========

// checkBehaviorAlerts 检查行为告警.
func (e *SurveillanceEngine) checkBehaviorAlerts(event *DetectionEvent) {
	// 检查该摄像头的所有区域
	for _, zone := range e.zones {
		if zone.CameraID != event.CameraID || !zone.Enabled {
			continue
		}

		for _, alertType := range zone.AlertTypes {
			switch alertType {
			case AlertTypeZoneIntrusion:
				if isPointInPolygon(event.Position, zone.Points) {
					e.generateAlert(event, zone, AlertTypeZoneIntrusion, "目标进入禁区")
				}
			case AlertTypeLoitering:
				if isPointInPolygon(event.Position, zone.Points) {
					e.checkLoitering(event, zone)
				}
			}
		}
	}
}

// generateAlert 生成告警.
func (e *SurveillanceEngine) generateAlert(event *DetectionEvent, zone *Zone, alertType AlertType, desc string) {
	alert := &BehaviorAlert{
		ID:          fmt.Sprintf("alert-%d-%s", time.Now().UnixNano(), event.CameraID),
		CameraID:    event.CameraID,
		ZoneID:      zone.ID,
		Type:        alertType,
		Status:      AlertStatusActive,
		Description: desc,
		BoundingBox: event.BoundingBox,
		Timestamp:   time.Now(),
	}
	e.alerts = append(e.alerts, alert)

	// 截断旧告警
	if len(e.alerts) > e.config.MaxAlerts {
		e.alerts = e.alerts[len(e.alerts)-e.config.MaxAlerts:]
	}
}

// checkLoitering 检查徘徊行为.
func (e *SurveillanceEngine) checkLoitering(event *DetectionEvent, zone *Zone) {
	// 查找该区域内最近的同类事件
	recentCount := 0
	for i := len(e.events) - 1; i >= 0; i-- {
		evt := e.events[i]
		if evt.CameraID != event.CameraID || evt.Type != event.Type {
			continue
		}
		if time.Since(evt.Timestamp) > e.config.LoiterDuration {
			break
		}
		if isPointInPolygon(evt.Position, zone.Points) {
			recentCount++
		}
	}
	// 超过阈值则触发徘徊告警
	if recentCount >= 3 {
		e.generateAlert(event, zone, AlertTypeLoitering, "检测到徘徊行为")
	}
}

// isPointInPolygon 射线法判断点是否在多边形内.
func isPointInPolygon(p Point, polygon []Point) bool {
	if len(polygon) < 3 {
		return false
	}

	inside := false
	j := len(polygon) - 1
	for i := 0; i < len(polygon); i++ {
		xi, yi := polygon[i].X, polygon[i].Y
		xj, yj := polygon[j].X, polygon[j].Y

		if ((yi > p.Y) != (yj > p.Y)) && (p.X < (xj-xi)*(p.Y-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}
	return inside
}

// ========== 智能区域配置 ==========

// AddZone 添加区域.
func (e *SurveillanceEngine) AddZone(zone *Zone) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if zone.ID == "" {
		return ErrInvalidZone
	}
	if _, exists := e.zones[zone.ID]; exists {
		return ErrZoneAlreadyExists
	}
	if len(zone.Points) < 3 {
		return ErrInvalidZone
	}
	if _, exists := e.cameras[zone.CameraID]; !exists {
		return ErrCameraNotFound
	}

	if zone.CreatedAt.IsZero() {
		zone.CreatedAt = time.Now()
	}
	e.zones[zone.ID] = zone
	return nil
}

// RemoveZone 删除区域.
func (e *SurveillanceEngine) RemoveZone(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.zones[id]; !exists {
		return ErrZoneNotFound
	}

	delete(e.zones, id)
	return nil
}

// GetZone 获取区域.
func (e *SurveillanceEngine) GetZone(id string) (*Zone, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	zone, exists := e.zones[id]
	if !exists {
		return nil, ErrZoneNotFound
	}
	return zone, nil
}

// ListZones 列出所有区域.
func (e *SurveillanceEngine) ListZones() []*Zone {
	e.mu.RLock()
	defer e.mu.RUnlock()

	zones := make([]*Zone, 0, len(e.zones))
	for _, zone := range e.zones {
		zones = append(zones, zone)
	}
	return zones
}

// ListZonesByCamera 列出指定摄像头的区域.
func (e *SurveillanceEngine) ListZonesByCamera(cameraID string) []*Zone {
	e.mu.RLock()
	defer e.mu.RUnlock()

	zones := make([]*Zone, 0)
	for _, zone := range e.zones {
		if zone.CameraID == cameraID {
			zones = append(zones, zone)
		}
	}
	return zones
}

// ========== 告警管理 ==========

// GetAlert 获取告警.
func (e *SurveillanceEngine) GetAlert(id string) (*BehaviorAlert, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, alert := range e.alerts {
		if alert.ID == id {
			return alert, nil
		}
	}
	return nil, ErrAlertNotFound
}

// QueryAlerts 查询告警.
func (e *SurveillanceEngine) QueryAlerts(query AlertQuery) []*BehaviorAlert {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var results []*BehaviorAlert
	for _, alert := range e.alerts {
		if !matchAlertQuery(alert, query) {
			continue
		}
		results = append(results, alert)
	}

	// 分页
	total := len(results)
	if query.Offset >= total {
		return []*BehaviorAlert{}
	}
	end := total
	if query.Limit > 0 && query.Offset+query.Limit < total {
		end = query.Offset + query.Limit
	}
	return results[query.Offset:end]
}

// matchAlertQuery 匹配告警查询条件.
func matchAlertQuery(alert *BehaviorAlert, query AlertQuery) bool {
	if query.CameraID != "" && alert.CameraID != query.CameraID {
		return false
	}
	if query.ZoneID != "" && alert.ZoneID != query.ZoneID {
		return false
	}
	if query.Type != "" && alert.Type != query.Type {
		return false
	}
	if query.Status != "" && alert.Status != query.Status {
		return false
	}
	if !query.StartTime.IsZero() && alert.Timestamp.Before(query.StartTime) {
		return false
	}
	if !query.EndTime.IsZero() && alert.Timestamp.After(query.EndTime) {
		return false
	}
	return true
}

// AcknowledgeAlert 确认告警.
func (e *SurveillanceEngine) AcknowledgeAlert(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, alert := range e.alerts {
		if alert.ID == id {
			alert.Status = AlertStatusAcknowledged
			alert.AckedAt = time.Now()
			return nil
		}
	}
	return ErrAlertNotFound
}

// ResolveAlert 解决告警.
func (e *SurveillanceEngine) ResolveAlert(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, alert := range e.alerts {
		if alert.ID == id {
			alert.Status = AlertStatusResolved
			alert.ResolvedAt = time.Now()
			return nil
		}
	}
	return ErrAlertNotFound
}

// ========== 越界检测 ==========

// CheckLineCrossing 检查越界事件.
func (e *SurveillanceEngine) CheckLineCrossing(trackID string, line Line) (bool, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	track, exists := e.tracks[trackID]
	if !exists {
		return false, ErrTrackNotFound
	}

	if len(track.Positions) < 2 {
		return false, nil
	}

	// 获取最后两个位置
	n := len(track.Positions)
	p1 := track.Positions[n-2].Position
	p2 := track.Positions[n-1].Position

	// 线段相交检测
	return segmentsIntersect(p1, p2, line.Start, line.End), nil
}

// segmentsIntersect 检测两条线段是否相交.
func segmentsIntersect(p1, p2, p3, p4 Point) bool {
	d1 := direction(p3, p4, p1)
	d2 := direction(p3, p4, p2)
	d3 := direction(p1, p2, p3)
	d4 := direction(p1, p2, p4)

	if ((d1 > 0 && d2 < 0) || (d1 < 0 && d2 > 0)) &&
		((d3 > 0 && d4 < 0) || (d3 < 0 && d4 > 0)) {
		return true
	}

	if d1 == 0 && onSegment(p3, p4, p1) {
		return true
	}
	if d2 == 0 && onSegment(p3, p4, p2) {
		return true
	}
	if d3 == 0 && onSegment(p1, p2, p3) {
		return true
	}
	if d4 == 0 && onSegment(p1, p2, p4) {
		return true
	}

	return false
}

// direction 计算方向.
func direction(p1, p2, p3 Point) float64 {
	return (p3.X-p1.X)*(p2.Y-p1.Y) - (p2.X-p1.X)*(p3.Y-p1.Y)
}

// onSegment 检查点是否在线段上.
func onSegment(p1, p2, p3 Point) bool {
	if p3.X >= min(p1.X, p2.X) && p3.X <= max(p1.X, p2.X) &&
		p3.Y >= min(p1.Y, p2.Y) && p3.Y <= max(p1.Y, p2.Y) {
		return true
	}
	return false
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// ========== 统计 ==========

// GetStats 获取监控统计信息.
func (e *SurveillanceEngine) GetStats() *SurveillanceStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &SurveillanceStats{
		TotalCameras: len(e.cameras),
		EventsByType: make(map[EventType]int),
		AlertsByType: make(map[AlertType]int),
	}

	// 统计在线摄像头
	for _, cam := range e.cameras {
		if cam.Status == CameraOnline {
			stats.OnlineCameras++
		}
	}

	// 统计事件
	stats.TotalEvents = len(e.events)
	for _, event := range e.events {
		stats.EventsByType[event.Type]++
	}

	// 统计告警
	stats.TotalAlerts = len(e.alerts)
	for _, alert := range e.alerts {
		stats.AlertsByType[alert.Type]++
		if alert.Status == AlertStatusActive {
			stats.ActiveAlerts++
		}
	}

	// 统计活跃跟踪
	for _, track := range e.tracks {
		if track.IsActive {
			stats.ActiveTracks++
		}
	}

	return stats
}
