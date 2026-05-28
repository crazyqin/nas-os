package surveillance

import (
	"time"
)

// CameraStatus 摄像头状态.
type CameraStatus string

const (
	CameraStatusOnline  CameraStatus = "online"
	CameraStatusOffline CameraStatus = "offline"
	CameraStatusError   CameraStatus = "error"
)

// RecordingMode 录制模式.
type RecordingMode string

const (
	RecordingModeContinuous RecordingMode = "continuous"
	RecordingModeMotion     RecordingMode = "motion"
	RecordingModeSchedule   RecordingMode = "schedule"
	RecordingModeManual     RecordingMode = "manual"
)

// EventSeverity 事件严重程度.
type EventSeverity string

const (
	EventSeverityInfo     EventSeverity = "info"
	EventSeverityWarning  EventSeverity = "warning"
	EventSeverityCritical EventSeverity = "critical"
)

// EventType 事件类型.
type EventType string

const (
	EventTypeMotion      EventType = "motion"
	EventTypeCameraOnline  EventType = "camera_online"
	EventTypeCameraOffline EventType = "camera_offline"
	EventTypeRecordingStart EventType = "recording_start"
	EventTypeRecordingStop  EventType = "recording_stop"
	EventTypeStorageFull    EventType = "storage_full"
	EventTypeCustom         EventType = "custom"
)

// Camera 摄像头配置.
type Camera struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	URI           string       `json:"uri"`           // RTSP URI
	ONVIFEndpoint string       `json:"onvifEndpoint"` // ONVIF 地址
	Username      string       `json:"username,omitempty"`
	Password      string       `json:"password,omitempty"`
	Status        CameraStatus `json:"status"`
	Resolution    string       `json:"resolution"` // 如 "1920x1080"
	FPS           int          `json:"fps"`
	Codec         string       `json:"codec"` // 如 "h264", "h265"
	Location      string       `json:"location,omitempty"`
	Group         string       `json:"group,omitempty"`
	Enabled       bool         `json:"enabled"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
}

// RecordingJob 录制任务.
type RecordingJob struct {
	ID         string        `json:"id"`
	CameraID   string        `json:"cameraId"`
	Mode       RecordingMode `json:"mode"`
	StartTime  time.Time     `json:"startTime"`
	EndTime    *time.Time    `json:"endTime,omitempty"`
	FilePath   string        `json:"filePath"`
	FileSize   int64         `json:"fileSize"`
	Duration   int64         `json:"duration"` // 秒
	Status     string        `json:"status"`   // recording, completed, failed
	CreatedAt  time.Time     `json:"createdAt"`
}

// MotionDetectionConfig 移动侦测配置.
type MotionDetectionConfig struct {
	CameraID       string             `json:"cameraId"`
	Enabled        bool               `json:"enabled"`
	Sensitivity    int                `json:"sensitivity"` // 1-100
	Regions        []MotionRegion     `json:"regions"`
	Cooldown       int                `json:"cooldown"`       // 秒
	TriggerActions []TriggerAction    `json:"triggerActions"`
}

// MotionRegion 移动侦测区域.
type MotionRegion struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	X        int       `json:"x"`
	Y        int       `json:"y"`
	Width    int       `json:"width"`
	Height   int       `json:"height"`
	Enabled  bool      `json:"enabled"`
}

// TriggerAction 触发动作.
type TriggerAction struct {
	Type     string `json:"type"` // notify, record, snapshot
	Config   map[string]interface{} `json:"config,omitempty"`
}

// SurveillanceEvent 监控事件.
type SurveillanceEvent struct {
	ID        string          `json:"id"`
	CameraID  string          `json:"cameraId"`
	Type      EventType       `json:"type"`
	Severity  EventSeverity   `json:"severity"`
	Message   string          `json:"message"`
	ImageURL  string          `json:"imageUrl,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

// RecordingSchedule 录制计划.
type RecordingSchedule struct {
	ID       string        `json:"id"`
	CameraID string        `json:"cameraId"`
	Name     string        `json:"name"`
	Mode     RecordingMode `json:"mode"`
	// Cron 表达式或时间段
	DaysOfWeek []int     `json:"daysOfWeek"` // 0=周日, 1=周一, ...
	StartTime  string    `json:"startTime"`  // HH:MM
	EndTime    string    `json:"endTime"`    // HH:MM
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"createdAt"`
}

// StreamSession 流媒体会话.
type StreamSession struct {
	ID        string    `json:"id"`
	CameraID  string    `json:"cameraId"`
	ClientID  string    `json:"clientId"`
	Protocol  string    `json:"protocol"` // rtsp, rtmp, hls, webrtc
	URL       string    `json:"url"`
	StartTime time.Time `json:"startTime"`
	Active    bool      `json:"active"`
}

// PlaybackQuery 回放查询.
type PlaybackQuery struct {
	CameraID  string    `json:"cameraId"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	EventOnly bool      `json:"eventOnly"` // 只返回有事件的片段
}

// PlaybackSegment 回放片段.
type PlaybackSegment struct {
	ID        string    `json:"id"`
	CameraID  string    `json:"cameraId"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	FilePath  string    `json:"filePath"`
	FileSize  int64     `json:"fileSize"`
	HasMotion bool      `json:"hasMotion"`
	Events    []string  `json:"events"` // 关联的事件ID
}

// ExportRequest 导出请求.
type ExportRequest struct {
	CameraID  string    `json:"cameraId"`
	StartTime time.Time `json:"startTime"`
	EndTime   time.Time `json:"endTime"`
	Format    string    `json:"format"` // mp4, avi, mkv
}

// ExportJob 导出任务.
type ExportJob struct {
	ID         string    `json:"id"`
	CameraID   string    `json:"cameraId"`
	StartTime  time.Time `json:"startTime"`
	EndTime    time.Time `json:"endTime"`
	Format     string    `json:"format"`
	FilePath   string    `json:"filePath"`
	FileSize   int64     `json:"fileSize"`
	Status     string    `json:"status"` // pending, processing, completed, failed
	Progress   int       `json:"progress"`
	CreatedAt  time.Time `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

// StorageQuota 存储配额.
type StorageQuota struct {
	CameraID       string `json:"cameraId"`
	MaxSizeGB      int    `json:"maxSizeGb"`
	UsedSizeGB     int    `json:"usedSizeGb"`
	RetentionDays  int    `json:"retentionDays"`
	AutoDelete     bool   `json:"autoDelete"`
}

// PTZCommand PTZ 控制命令.
type PTZCommand struct {
	CameraID string `json:"cameraId"`
	Action   string `json:"action"` // pan, tilt, zoom, stop, go_preset
	Speed    int    `json:"speed"`  // 1-100
	PresetID string `json:"presetId,omitempty"`
}

// ONVIFDiscoveryResult ONVIF 发现结果.
type ONVIFDiscoveryResult struct {
	IPAddress    string `json:"ipAddress"`
	Port         int    `json:"port"`
	Manufacturer string `json:"manufacturer"`
	Model        string `json:"model"`
	SerialNumber string `json:"serialNumber"`
	Endpoint     string `json:"endpoint"`
}

// SurveillanceStats 监控统计.
type SurveillanceStats struct {
	TotalCameras     int     `json:"totalCameras"`
	OnlineCameras    int     `json:"onlineCameras"`
	OfflineCameras   int     `json:"offlineCameras"`
	ActiveRecordings int     `json:"activeRecordings"`
	TotalRecordings  int     `json:"totalRecordings"`
	StorageUsedGB    float64 `json:"storageUsedGb"`
	StorageTotalGB   float64 `json:"storageTotalGb"`
	TodayEvents      int     `json:"todayEvents"`
	ActiveStreams    int     `json:"activeStreams"`
}

// 兼容性别名 - 保持向后兼容
type SurveillanceManager = Manager

// MotionEvent 移动侦测事件
type MotionEvent struct {
	ID         string    `json:"id"`
	CameraID   string    `json:"cameraId"`
	Confidence float64   `json:"confidence"`
	Region     string    `json:"region"`
	Timestamp  time.Time `json:"timestamp"`
}

// NewSurveillanceManager 兼容旧接口
func NewSurveillanceManager(logger interface{}, dataPath string) *Manager {
	m, _ := NewManager(dataPath)
	return m
}

// NewHandler 兼容旧接口
func NewHandler(manager *Manager, logger interface{}) *Handlers {
	return NewHandlers(manager)
}
