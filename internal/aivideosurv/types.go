// Package aivideosurv 提供 AI 视频监控增强功能
package aivideosurv

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrCameraNotFound 摄像头不存在.
	ErrCameraNotFound = errors.New("摄像头不存在")
	// ErrCameraAlreadyExists 摄像头已存在.
	ErrCameraAlreadyExists = errors.New("摄像头已存在")
	// ErrZoneNotFound 区域不存在.
	ErrZoneNotFound = errors.New("区域不存在")
	// ErrZoneAlreadyExists 区域已存在.
	ErrZoneAlreadyExists = errors.New("区域已存在")
	// ErrEventNotFound 事件不存在.
	ErrEventNotFound = errors.New("事件不存在")
	// ErrTrackNotFound 跟踪记录不存在.
	ErrTrackNotFound = errors.New("跟踪记录不存在")
	// ErrAlertNotFound 告警不存在.
	ErrAlertNotFound = errors.New("告警不存在")
	// ErrInvalidQuery 无效查询参数.
	ErrInvalidQuery = errors.New("无效查询参数")
	// ErrInvalidZone 无效区域定义.
	ErrInvalidZone = errors.New("无效区域定义")
	// ErrInvalidCamera 无效摄像头配置.
	ErrInvalidCamera = errors.New("无效摄像头配置")
)

// ========== 摄像头状态 ==========

// CameraStatus 摄像头状态.
type CameraStatus string

const (
	// CameraOnline 在线.
	CameraOnline CameraStatus = "online"
	// CameraOffline 离线.
	CameraOffline CameraStatus = "offline"
	// CameraMaintenance 维护中.
	CameraMaintenance CameraStatus = "maintenance"
)

// ========== 事件类型 ==========

// EventType 检测事件类型.
type EventType string

const (
	// EventTypePerson 人员检测.
	EventTypePerson EventType = "person"
	// EventTypeVehicle 车辆检测.
	EventTypeVehicle EventType = "vehicle"
	// EventTypeAnimal 动物检测.
	EventTypeAnimal EventType = "animal"
	// EventTypeObject 物体检测.
	EventTypeObject EventType = "object"
)

// ========== 告警类型 ==========

// AlertType 行为告警类型.
type AlertType string

const (
	// AlertTypeLineCrossing 越界检测.
	AlertTypeLineCrossing AlertType = "line_crossing"
	// AlertTypeZoneIntrusion 区域入侵.
	AlertTypeZoneIntrusion AlertType = "zone_intrusion"
	// AlertTypeLoitering 徘徊检测.
	AlertTypeLoitering AlertType = "loitering"
)

// ========== 告警状态 ==========

// AlertStatus 告警状态.
type AlertStatus string

const (
	// AlertStatusActive 活跃.
	AlertStatusActive AlertStatus = "active"
	// AlertStatusAcknowledged 已确认.
	AlertStatusAcknowledged AlertStatus = "acknowledged"
	// AlertStatusResolved 已解决.
	AlertStatusResolved AlertStatus = "resolved"
)

// ========== 核心类型 ==========

// Point 二维坐标点.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// BoundingBox 检测框.
type BoundingBox struct {
	X      float64 `json:"x"`      // 左上角X坐标
	Y      float64 `json:"y"`      // 左上角Y坐标
	Width  float64 `json:"width"`  // 宽度
	Height float64 `json:"height"` // 高度
}

// Line 越界检测线.
type Line struct {
	Start Point `json:"start"`
	End   Point `json:"end"`
}

// Camera 摄像头配置.
type Camera struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Location    string       `json:"location"`
	URL         string       `json:"url"`
	Status      CameraStatus `json:"status"`
	Resolution  string       `json:"resolution"`   // 如 "1920x1080"
	FPS         int          `json:"fps"`
	EnableAI    bool         `json:"enable_ai"`     // 是否启用AI检测
	LastActive  time.Time    `json:"last_active"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// DetectionEvent AI检测事件.
type DetectionEvent struct {
	ID          string      `json:"id"`
	CameraID    string      `json:"camera_id"`
	Type        EventType   `json:"type"`
	Confidence  float64     `json:"confidence"` // 置信度 0-1
	BoundingBox BoundingBox `json:"bounding_box"`
	Position    Point       `json:"position"`    // 目标中心点
	Timestamp   time.Time   `json:"timestamp"`
	FrameURL    string      `json:"frame_url"`   // 截帧图片URL
	Attributes  map[string]string `json:"attributes,omitempty"` // 扩展属性
}

// ObjectTrack 目标跟踪记录.
type ObjectTrack struct {
	ID          string        `json:"id"`
	ObjectType  EventType     `json:"object_type"`
	CameraIDs   []string      `json:"camera_ids"`   // 出现过的摄像头
	FirstSeen   time.Time     `json:"first_seen"`
	LastSeen    time.Time     `json:"last_seen"`
	Positions   []TrackPoint  `json:"positions"`     // 轨迹点
	IsActive    bool          `json:"is_active"`
	Attributes  map[string]string `json:"attributes,omitempty"`
}

// TrackPoint 轨迹点.
type TrackPoint struct {
	CameraID  string    `json:"camera_id"`
	Position  Point     `json:"position"`
	Timestamp time.Time `json:"timestamp"`
}

// Zone 智能区域配置.
type Zone struct {
	ID          string   `json:"id"`
	CameraID    string   `json:"camera_id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Points      []Point  `json:"points"`       // 区域顶点（多边形）
	AlertTypes  []AlertType `json:"alert_types"` // 触发的告警类型
	Enabled     bool     `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
}

// BehaviorAlert 行为分析告警.
type BehaviorAlert struct {
	ID          string      `json:"id"`
	CameraID    string      `json:"camera_id"`
	ZoneID      string      `json:"zone_id,omitempty"`
	Type        AlertType   `json:"type"`
	Status      AlertStatus `json:"status"`
	TrackID     string      `json:"track_id"`     // 关联的跟踪ID
	Description string      `json:"description"`
	BoundingBox BoundingBox `json:"bounding_box"`
	Timestamp   time.Time   `json:"timestamp"`
	AckedAt     time.Time   `json:"acked_at,omitempty"`
	ResolvedAt  time.Time   `json:"resolved_at,omitempty"`
}

// ========== 查询类型 ==========

// EventQuery 事件查询条件.
type EventQuery struct {
	CameraID   string    `json:"camera_id,omitempty"`
	Type       EventType `json:"type,omitempty"`
	StartTime  time.Time `json:"start_time,omitempty"`
	EndTime    time.Time `json:"end_time,omitempty"`
	MinConf    float64   `json:"min_confidence,omitempty"`
	Offset     int       `json:"offset"`
	Limit      int       `json:"limit"`
}

// AlertQuery 告警查询条件.
type AlertQuery struct {
	CameraID  string      `json:"camera_id,omitempty"`
	ZoneID    string      `json:"zone_id,omitempty"`
	Type      AlertType   `json:"type,omitempty"`
	Status    AlertStatus `json:"status,omitempty"`
	StartTime time.Time   `json:"start_time,omitempty"`
	EndTime   time.Time   `json:"end_time,omitempty"`
	Offset    int         `json:"offset"`
	Limit     int         `json:"limit"`
}

// ========== 统计类型 ==========

// SurveillanceStats 监控统计信息.
type SurveillanceStats struct {
	TotalCameras      int            `json:"total_cameras"`
	OnlineCameras     int            `json:"online_cameras"`
	TotalEvents       int            `json:"total_events"`
	TotalAlerts       int            `json:"total_alerts"`
	ActiveAlerts      int            `json:"active_alerts"`
	ActiveTracks      int            `json:"active_tracks"`
	EventsByType      map[EventType]int    `json:"events_by_type"`
	AlertsByType      map[AlertType]int    `json:"alerts_by_type"`
}
