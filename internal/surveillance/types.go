// Package surveillance 提供视频监控管理功能
// 参考群晖 Surveillance Station，支持摄像头管理、实时流、录像、移动侦测等
package surveillance

import "time"

// ========== 摄像头协议和状态 ==========

// CameraProtocol 摄像头协议.
type CameraProtocol string

const (
	ProtocolONVIF CameraProtocol = "ONVIF"
	ProtocolRTSP  CameraProtocol = "RTSP"
)

// CameraStatus 摄像头状态.
type CameraStatus string

const (
	CameraStatusOnline  CameraStatus = "online"
	CameraStatusOffline CameraStatus = "offline"
	CameraStatusError   CameraStatus = "error"
)

// ========== 录像相关 ==========

// RecordingMode 录像模式.
type RecordingMode string

const (
	RecordingModeContinuous RecordingMode = "continuous" // 连续录像
	RecordingModeEvent      RecordingMode = "event"      // 事件触发
	RecordingModeSchedule   RecordingMode = "schedule"   // 计划录像
	RecordingModeManual     RecordingMode = "manual"     // 手动录像
)

// ========== 移动侦测 ==========

// MotionSensitivity 移动侦测灵敏度.
type MotionSensitivity string

const (
	SensitivityLow    MotionSensitivity = "low"
	SensitivityMedium MotionSensitivity = "medium"
	SensitivityHigh   MotionSensitivity = "high"
)

// ========== 告警相关 ==========

// EventType 告警事件类型.
type EventType string

const (
	EventMotionDetection EventType = "motion_detection" // 移动侦测
	EventHumanDetection  EventType = "human_detection"  // 人形检测
	EventTampering       EventType = "tampering"        // 遮挡告警
	EventDisconnect      EventType = "disconnect"       // 断线告警
	EventReconnect       EventType = "reconnect"        // 恢复连接
)

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// AlertStatus 告警处理状态.
type AlertStatus string

const (
	AlertStatusPending  AlertStatus = "pending"
	AlertStatusAcked    AlertStatus = "acked"
	AlertStatusResolved AlertStatus = "resolved"
	AlertStatusIgnored  AlertStatus = "ignored"
)

// ========== 联动动作 ==========

// ActionTrigger 联动动作类型.
type ActionTrigger string

const (
	ActionRecord   ActionTrigger = "record"   // 触发录像
	ActionNotify   ActionTrigger = "notify"   // 发送通知
	ActionBuzzer   ActionTrigger = "buzzer"   // 蜂鸣报警
	ActionSnapshot ActionTrigger = "snapshot" // 抓拍快照
)

// ========== 摄像头相关 ==========

// Camera 摄像头配置.
type Camera struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Protocol     CameraProtocol `json:"protocol"`
	RTSPUrl      string         `json:"rtspUrl"` // RTSP 地址
	Host         string         `json:"host"`
	Port         int            `json:"port"`
	StreamPath   string         `json:"streamPath"`
	Username     string         `json:"username,omitempty"`
	Password     string         `json:"password,omitempty"`
	Status       CameraStatus   `json:"status"`
	Resolution   string         `json:"resolution,omitempty"` // 1920x1080
	FPS          int            `json:"fps,omitempty"`
	Bitrate      int            `json:"bitrate,omitempty"` // kbps
	Manufacturer string         `json:"manufacturer,omitempty"`
	Model        string         `json:"model,omitempty"`
	Location     string         `json:"location,omitempty"`
	GroupID      string         `json:"groupId,omitempty"`
	Enabled      bool           `json:"enabled"`
	CreatedAt    time.Time      `json:"createdAt"`
	UpdatedAt    time.Time      `json:"updatedAt"`
}

// CameraStream 实时流信息.
type CameraStream struct {
	CameraID   string    `json:"cameraId"`
	StreamURL  string    `json:"streamUrl"`
	Resolution string    `json:"resolution"`
	Bitrate    int       `json:"bitrate"`
	FPS        int       `json:"fps"`
	Codec      string    `json:"codec"`
	StartTime  time.Time `json:"startTime"`
}

// ========== 录像相关 ==========

// Recording 录像记录.
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
	HasEvent   bool          `json:"hasEvent"`           // 是否由事件触发
	EventIDs   []string      `json:"eventIds,omitempty"` // 关联的事件ID
	CreatedAt  time.Time     `json:"createdAt"`
}

// RecordingSchedule 录像计划.
type RecordingSchedule struct {
	ID        string         `json:"id"`
	CameraID  string         `json:"cameraId"`
	Name      string         `json:"name"`
	Mode      RecordingMode  `json:"mode"`
	Type      ScheduleType   `json:"type"`            // all_day / timed / event
	Days      []time.Weekday `json:"days"`            // 生效的星期几
	Start     string         `json:"start,omitempty"` // HH:MM (timed模式)
	End       string         `json:"end,omitempty"`   // HH:MM (timed模式)
	Enabled   bool           `json:"enabled"`
	CreatedAt time.Time      `json:"createdAt"`
}

// ScheduleType 计划类型.
type ScheduleType string

const (
	ScheduleTypeAllDay ScheduleType = "all_day" // 全天录像
	ScheduleTypeTimed  ScheduleType = "timed"   // 定时录像
	ScheduleTypeEvent  ScheduleType = "event"   // 事件触发录像
)

// ========== 移动侦测和AI检测 ==========

// MotionDetection 移动侦测配置.
type MotionDetection struct {
	CameraID    string            `json:"cameraId"`
	Enabled     bool              `json:"enabled"`
	Sensitivity MotionSensitivity `json:"sensitivity"`
	Regions     []MotionRegion    `json:"regions"`
	CooldownSec int               `json:"cooldownSec"` // 触发后冷却时间
	AIDetection bool              `json:"aiDetection"` // 启用AI人形检测
}

// MotionRegion 移动侦测区域.
type MotionRegion struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// MotionEvent 移动侦测事件.
type MotionEvent struct {
	ID          string    `json:"id"`
	CameraID    string    `json:"cameraId"`
	RegionID    string    `json:"regionId,omitempty"`
	RegionName  string    `json:"regionName,omitempty"`
	Confidence  float64   `json:"confidence"` // 0.0 - 1.0
	IsHuman     bool      `json:"isHuman"`    // AI人形检测结果
	SnapshotURL string    `json:"snapshotUrl,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// ========== 告警 ==========

// Alert 告警.
type Alert struct {
	ID         string      `json:"id"`
	CameraID   string      `json:"cameraId"`
	CameraName string      `json:"cameraName"`
	EventType  EventType   `json:"eventType"`
	Level      AlertLevel  `json:"level"`
	Message    string      `json:"message"`
	ImageURL   string      `json:"imageUrl,omitempty"`
	Status     AlertStatus `json:"status"`
	AckedBy    string      `json:"ackedBy,omitempty"`
	AckedAt    time.Time   `json:"ackedAt,omitempty"`
	CreatedAt  time.Time   `json:"createdAt"`
}

// ActionRule 事件联动规则.
type ActionRule struct {
	ID        string          `json:"id"`
	CameraID  string          `json:"cameraId"`
	EventType EventType       `json:"eventType"`
	Actions   []ActionTrigger `json:"actions"`
	Enabled   bool            `json:"enabled"`
}

// ========== 分组和布局 ==========

// CameraGroup 摄像头分组.
type CameraGroup struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CameraIDs   []string  `json:"cameraIds"`
	Layout      Layout    `json:"layout"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Layout 布局配置.
type Layout struct {
	Rows    int `json:"rows"`
	Columns int `json:"columns"`
}

// ========== 存储管理 ==========

// StorageQuota 存储配额.
type StorageQuota struct {
	CameraID      string `json:"cameraId"`
	MaxSizeGB     int    `json:"maxSizeGb"`     // 最大存储 GB
	CurrentSizeGB int    `json:"currentSizeGb"` // 当前存储 GB
	RetentionDays int    `json:"retentionDays"` // 保留天数
	LoopRecording bool   `json:"loopRecording"` // 循环录像
}

// ========== 快照 ==========

// Snapshot 快照.
type Snapshot struct {
	ID        string    `json:"id"`
	CameraID  string    `json:"cameraId"`
	FilePath  string    `json:"filePath"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"createdAt"`
}

// ========== 统计 ==========

// SurveillanceStats 监控系统统计.
type SurveillanceStats struct {
	TotalCameras    int     `json:"totalCameras"`
	OnlineCameras   int     `json:"onlineCameras"`
	OfflineCameras  int     `json:"offlineCameras"`
	ActiveStreams   int     `json:"activeStreams"`
	TotalRecordings int     `json:"totalRecordings"`
	TotalEvents     int     `json:"totalEvents"`
	TotalAlerts     int     `json:"totalAlerts"`
	PendingAlerts   int     `json:"pendingAlerts"`
	TotalGroups     int     `json:"totalGroups"`
	TotalSchedules  int     `json:"totalSchedules"`
	StorageUsedGB   float64 `json:"storageUsedGb"`
	StorageTotalGB  float64 `json:"storageTotalGb"`
}
