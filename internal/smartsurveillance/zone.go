// Package smartsurveillance 提供智能监控中心功能
// zone.go - 区域管理，支持电子围栏、入侵检测、越线检测
package smartsurveillance

import (
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ZoneManager 区域管理器
type ZoneManager struct {
	mu     sync.RWMutex
	logger *zap.Logger
	engine *SurveillanceEngine
	zones  map[string]*Zone
}

// NewZoneManager 创建区域管理器
func NewZoneManager(logger *zap.Logger, engine *SurveillanceEngine) *ZoneManager {
	return &ZoneManager{
		logger: logger,
		engine: engine,
		zones:  make(map[string]*Zone),
	}
}

// CreateZone 创建监控区域
func (zm *ZoneManager) CreateZone(zone *Zone) error {
	zm.mu.Lock()
	defer zm.mu.Unlock()

	if zone.ID == "" {
		zone.ID = uuid.New().String()
	}

	if _, exists := zm.zones[zone.ID]; exists {
		return ErrZoneExists
	}

	// 验证摄像头存在
	if _, err := zm.engine.GetCamera(zone.CameraID); err != nil {
		return err
	}

	zone.Enabled = true
	zone.CreatedAt = time.Now()
	zone.UpdatedAt = time.Now()

	zm.zones[zone.ID] = zone
	zm.logger.Info("监控区域已创建",
		zap.String("id", zone.ID),
		zap.String("name", zone.Name),
		zap.String("type", string(zone.Type)))
	return nil
}

// UpdateZone 更新监控区域
func (zm *ZoneManager) UpdateZone(zone *Zone) error {
	zm.mu.Lock()
	defer zm.mu.Unlock()

	existing, exists := zm.zones[zone.ID]
	if !exists {
		return ErrZoneNotFound
	}

	zone.CreatedAt = existing.CreatedAt
	zone.UpdatedAt = time.Now()
	zm.zones[zone.ID] = zone
	return nil
}

// DeleteZone 删除监控区域
func (zm *ZoneManager) DeleteZone(zoneID string) error {
	zm.mu.Lock()
	defer zm.mu.Unlock()

	if _, exists := zm.zones[zoneID]; !exists {
		return ErrZoneNotFound
	}

	delete(zm.zones, zoneID)
	zm.logger.Info("监控区域已删除", zap.String("id", zoneID))
	return nil
}

// GetZone 获取监控区域
func (zm *ZoneManager) GetZone(zoneID string) (*Zone, error) {
	zm.mu.RLock()
	defer zm.mu.RUnlock()

	zone, exists := zm.zones[zoneID]
	if !exists {
		return nil, ErrZoneNotFound
	}
	return zone, nil
}

// ListZones 列出所有监控区域
func (zm *ZoneManager) ListZones() []*Zone {
	zm.mu.RLock()
	defer zm.mu.RUnlock()

	zones := make([]*Zone, 0, len(zm.zones))
	for _, zone := range zm.zones {
		zones = append(zones, zone)
	}
	return zones
}

// GetCameraZones 获取摄像头的所有区域
func (zm *ZoneManager) GetCameraZones(cameraID string) []*Zone {
	zm.mu.RLock()
	defer zm.mu.RUnlock()

	var zones []*Zone
	for _, zone := range zm.zones {
		if zone.CameraID == cameraID {
			zones = append(zones, zone)
		}
	}
	return zones
}

// CheckIntrusion 检测入侵
func (zm *ZoneManager) CheckIntrusion(cameraID string, position Position, detectType DetectionType) []*Zone {
	zm.mu.RLock()
	defer zm.mu.RUnlock()

	var triggeredZones []*Zone

	for _, zone := range zm.zones {
		if zone.CameraID != cameraID || !zone.Enabled {
			continue
		}

		// 检查检测类型是否匹配
		if !zm.isTypeEnabled(zone, detectType) {
			continue
		}

		// 检查位置是否在区域内
		if zm.isPointInZone(position, zone) {
			triggeredZones = append(triggeredZones, zone)
		}
	}

	return triggeredZones
}

// isTypeEnabled 检查区域是否启用该检测类型
func (zm *ZoneManager) isTypeEnabled(zone *Zone, detectType DetectionType) bool {
	for _, dt := range zone.DetectionTypes {
		if dt == detectType {
			return true
		}
	}
	return false
}

// isPointInZone 检测点是否在区域内
func (zm *ZoneManager) isPointInZone(pos Position, zone *Zone) bool {
	switch zone.Type {
	case ZoneTypeRectangle:
		if len(zone.Points) >= 2 {
			p1 := zone.Points[0]
			p2 := zone.Points[1]
			return pos.X >= p1.X && pos.X <= p2.X &&
				pos.Y >= p1.Y && pos.Y <= p2.Y
		}
	case ZoneTypePolygon:
		return zm.pointInPolygon(pos, zone.Points)
	case ZoneTypeLine, ZoneTypeTripwire:
		return zm.crossLine(pos, zone.Points)
	}
	return false
}

// pointInPolygon 射线法检测点是否在多边形内
func (zm *ZoneManager) pointInPolygon(pos Position, points []Point) bool {
	if len(points) < 3 {
		return false
	}

	inside := false
	j := len(points) - 1

	for i := 0; i < len(points); i++ {
		xi, yi := points[i].X, points[i].Y
		xj, yj := points[j].X, points[j].Y

		if ((yi > pos.Y) != (yj > pos.Y)) &&
			(pos.X < (xj-xi)*(pos.Y-yi)/(yj-yi)+xi) {
			inside = !inside
		}
		j = i
	}

	return inside
}

// crossLine 检测是否穿越线段（简化版）
func (zm *ZoneManager) crossLine(pos Position, points []Point) bool {
	if len(points) < 2 {
		return false
	}

	// 简化检测：点在线段附近
	p1 := points[0]
	p2 := points[1]

	// 计算点到线段的距离
	dx := p2.X - p1.X
	dy := p2.Y - p1.Y
	length := dx*dx + dy*dy

	if length == 0 {
		return false
	}

	t := ((pos.X-p1.X)*dx + (pos.Y-p1.Y)*dy) / length
	if t < 0 || t > 1 {
		return false
	}

	// 计算最近点
	nearestX := p1.X + t*dx
	nearestY := p1.Y + t*dy

	// 距离阈值
	threshold := 20.0
	distX := pos.X - nearestX
	distY := pos.Y - nearestY
	dist := distX*distX + distY*distY

	return dist <= threshold*threshold
}

// ProcessZoneEvent 处理区域事件
func (zm *ZoneManager) ProcessZoneEvent(cameraID string, position Position, detectType DetectionType) []*Event {
	triggeredZones := zm.CheckIntrusion(cameraID, position, detectType)

	var events []*Event
	for _, zone := range triggeredZones {
		event := &Event{
			CameraID:    cameraID,
			Type:        DetectionTypeIntrusion,
			Confidence:  0.95,
			Description: "区域入侵: " + zone.Name,
			ZoneID:      zone.ID,
			ZoneName:    zone.Name,
			Position:    &position,
		}

		if err := zm.engine.ReportEvent(event); err != nil {
			zm.logger.Error("上报区域事件失败", zap.Error(err))
			continue
		}
		events = append(events, event)
	}

	return events
}
