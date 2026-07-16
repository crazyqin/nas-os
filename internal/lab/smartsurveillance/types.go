// Package smartsurveillance 提供智能监控中心功能
// 对标高级监控系统，支持多摄像头管理、AI实时分析、人脸识别、智能告警、时间线回放
package smartsurveillance

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrCameraNotFound 摄像头不存在错误.
	ErrCameraNotFound = errors.New("摄像头不存在")
	// ErrCameraExists 摄像头已存在错误.
	ErrCameraExists = errors.New("摄像头已存在")
	// ErrEventNotFound 事件不存在错误.
	ErrEventNotFound = errors.New("事件不存在")
	// ErrRecordingNotFound 录像不存在错误.
	ErrRecordingNotFound = errors.New("录像不存在")
	// ErrAlertNotFound 告警不存在错误.
	ErrAlertNotFound = errors.New("告警不存在")
	// ErrZoneNotFound 区域不存在错误.
	ErrZoneNotFound = errors.New("区域不存在")
	// ErrZoneExists 区域已存在错误.
	ErrZoneExists = errors.New("区域已存在")
	// ErrModelNotFound AI模型不存在错误.
	ErrModelNotFound = errors.New("AI模型不存在")
	// ErrInvalidConfig 无效配置错误.
	ErrInvalidConfig = errors.New("无效配置")
)

// ========== 摄像头状态 ==========

// CameraStatus 摄像头状态.
type CameraStatus string

// 摄像头状态常量.
const (
	CameraStatusOnline    CameraStatus = "online"    // 在线
	CameraStatusOffline   CameraStatus = "offline"   // 离线
	CameraStatusRecording CameraStatus = "recording" // 录像中
	CameraStatusError     CameraStatus = "error"     // 错误
	CameraStatusDisabled  CameraStatus = "disabled"  // 禁用
)

// ========== 检测类型 ==========

// DetectionType 检测类型.
type DetectionType string

// 检测类型常量.
const (
	DetectionTypeFace      DetectionType = "face"      // 人脸识别
	DetectionTypeObject    DetectionType = "object"    // 物体检测
	DetectionTypeBehavior  DetectionType = "behavior"  // 行为分析
	DetectionTypePlate     DetectionType = "plate"     // 车牌识别
	DetectionTypeMotion    DetectionType = "motion"    // 移动侦测
	DetectionTypeIntrusion DetectionType = "intrusion" // 入侵检测
)

// ========== 告警级别 ==========

// AlertLevel 告警级别.
type AlertLevel string

// 告警级别常量.
const (
	AlertLevelInfo      AlertLevel = "info"      // 信息
	AlertLevelWarning   AlertLevel = "warning"   // 警告
	AlertLevelCritical  AlertLevel = "critical"  // 严重
	AlertLevelEmergency AlertLevel = "emergency" // 紧急
)

// ========== 告警状态 ==========

// AlertStatus 告警状态.
type AlertStatus string

// 告警状态常量.
const (
	AlertStatusPending   AlertStatus = "pending"   // 待处理
	AlertStatusActive    AlertStatus = "active"    // 活跃
	AlertStatusAcked     AlertStatus = "acked"     // 已确认
	AlertStatusResolved  AlertStatus = "resolved"  // 已解决
	AlertStatusDismissed AlertStatus = "dismissed" // 已忽略
)

// ========== 区域类型 ==========

// ZoneType 区域类型.
type ZoneType string

// 区域类型常量.
const (
	ZoneTypePolygon   ZoneType = "polygon"   // 多边形区域
	ZoneTypeRectangle ZoneType = "rectangle" // 矩形区域
	ZoneTypeLine      ZoneType = "line"      // 越线检测
	ZoneTypeTripwire  ZoneType = "tripwire"  // 绊线检测
)

// ========== 核心数据结构 ==========

// Camera 摄像头.
type Camera struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Protocol       string            `json:"protocol"` // rtsp, onvif, hls
	URL            string            `json:"url"`
	Location       string            `json:"location"`
	Status         CameraStatus      `json:"status"`
	Resolution     string            `json:"resolution"` // 1080p, 720p, 4K
	Codec          string            `json:"codec"`
	FPS            int               `json:"fps"`
	BitrateKbps    int               `json:"bitrate_kbps"`
	PTZEnabled     bool              `json:"ptz_enabled"`     // 云台控制
	AudioEnabled   bool              `json:"audio_enabled"`   // 音频
	NightVision    bool              `json:"night_vision"`    // 夜视
	AIEnabled      bool              `json:"ai_enabled"`      // AI分析
	DetectionTypes []DetectionType   `json:"detection_types"` // 启用的检测类型
	RecordingMode  string            `json:"recording_mode"`  // continuous, motion, schedule
	StoragePath    string            `json:"storage_path"`
	Tags           map[string]string `json:"tags,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

// Event 监控事件.
type Event struct {
	ID           string        `json:"id"`
	CameraID     string        `json:"camera_id"`
	CameraName   string        `json:"camera_name,omitempty"`
	Type         DetectionType `json:"type"`
	Timestamp    time.Time     `json:"timestamp"`
	Duration     int           `json:"duration_sec,omitempty"`
	Confidence   float64       `json:"confidence"` // 0-1
	Description  string        `json:"description"`
	SnapshotURL  string        `json:"snapshot_url,omitempty"`
	VideoClipURL string        `json:"video_clip_url,omitempty"`
	ZoneID       string        `json:"zone_id,omitempty"`
	ZoneName     string        `json:"zone_name,omitempty"`
	// AI识别详情
	FaceName     string            `json:"face_name,omitempty"`     // 识别的人脸名称
	PlateNumber  string            `json:"plate_number,omitempty"`  // 识别的车牌号
	ObjectType   string            `json:"object_type,omitempty"`   // 物体类型
	BehaviorType string            `json:"behavior_type,omitempty"` // 行为类型
	Position     *Position         `json:"position,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Handled      bool              `json:"handled"`
}

// Position 位置信息.
type Position struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width,omitempty"`
	Height float64 `json:"height,omitempty"`
}

// Recording 录像记录.
type Recording struct {
	ID         string    `json:"id"`
	CameraID   string    `json:"camera_id"`
	CameraName string    `json:"camera_name,omitempty"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Duration   int       `json:"duration_sec"`
	FileSize   int64     `json:"file_size_bytes"`
	FilePath   string    `json:"file_path"`
	Resolution string    `json:"resolution"`
	HasEvents  bool      `json:"has_events"`
	EventCount int       `json:"event_count"`
	Tags       []string  `json:"tags,omitempty"`
}

// Alert 智能告警.
type Alert struct {
	ID          string      `json:"id"`
	CameraID    string      `json:"camera_id"`
	CameraName  string      `json:"camera_name,omitempty"`
	EventID     string      `json:"event_id,omitempty"`
	Level       AlertLevel  `json:"level"`
	Status      AlertStatus `json:"status"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Timestamp   time.Time   `json:"timestamp"`
	AckedAt     *time.Time  `json:"acked_at,omitempty"`
	ResolvedAt  *time.Time  `json:"resolved_at,omitempty"`
	AckedBy     string      `json:"acked_by,omitempty"`
	NotifySent  bool        `json:"notify_sent"`
	Actions     []string    `json:"actions,omitempty"` // 执行的动作
}

// Zone 监控区域.
type Zone struct {
	ID             string          `json:"id"`
	CameraID       string          `json:"camera_id"`
	Name           string          `json:"name"`
	Type           ZoneType        `json:"type"`
	Enabled        bool            `json:"enabled"`
	Points         []Point         `json:"points"`              // 区域顶点
	Direction      string          `json:"direction,omitempty"` // 越线方向：in, out, both
	DetectionTypes []DetectionType `json:"detection_types"`     // 区域内启用的检测
	Schedule       string          `json:"schedule,omitempty"`  // 生效时间表
	AlertLevel     AlertLevel      `json:"alert_level"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// Point 坐标点.
type Point struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// AIModel AI模型.
type AIModel struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Type       DetectionType `json:"type"`
	Version    string        `json:"version"`
	Enabled    bool          `json:"enabled"`
	Confidence float64       `json:"min_confidence"`   // 最小置信度阈值
	Labels     []string      `json:"labels,omitempty"` // 支持的标签
	GPUEnabled bool          `json:"gpu_enabled"`
	LoadTime   float64       `json:"load_time_ms"`
	FPS        float64       `json:"inference_fps"`
	UpdatedAt  time.Time     `json:"updated_at"`
}

// AIAnalysisResult AI分析结果.
type AIAnalysisResult struct {
	ModelID   string             `json:"model_id"`
	CameraID  string             `json:"camera_id"`
	Timestamp time.Time          `json:"timestamp"`
	Objects   []DetectedObject   `json:"objects"`
	Faces     []DetectedFace     `json:"faces,omitempty"`
	Plates    []DetectedPlate    `json:"plates,omitempty"`
	Behaviors []DetectedBehavior `json:"behaviors,omitempty"`
	FrameURL  string             `json:"frame_url,omitempty"`
	ProcessMs float64            `json:"process_ms"` // 处理耗时
}

// DetectedObject 检测到的物体.
type DetectedObject struct {
	Label      string   `json:"label"`
	Confidence float64  `json:"confidence"`
	Position   Position `json:"position"`
	Tracking   string   `json:"tracking_id,omitempty"`
}

// DetectedFace 检测到的人脸.
type DetectedFace struct {
	Name       string    `json:"name,omitempty"`      // 识别结果
	PersonID   string    `json:"person_id,omitempty"` // 人员ID
	Confidence float64   `json:"confidence"`
	Position   Position  `json:"position"`
	Embedding  []float64 `json:"embedding,omitempty"` // 人脸特征向量
}

// DetectedPlate 检测到的车牌.
type DetectedPlate struct {
	Number     string   `json:"number"` // 车牌号
	Confidence float64  `json:"confidence"`
	Position   Position `json:"position"`
	Color      string   `json:"color,omitempty"`
}

// DetectedBehavior 检测到的行为.
type DetectedBehavior struct {
	Type       string   `json:"type"` // loitering, running, fighting, falling
	Confidence float64  `json:"confidence"`
	Position   Position `json:"position"`
	Duration   int      `json:"duration_sec,omitempty"`
}

// ========== 查询参数 ==========

// EventQuery 事件查询参数.
type EventQuery struct {
	CameraID  string          `json:"camera_id,omitempty"`
	Types     []DetectionType `json:"types,omitempty"`
	StartTime *time.Time      `json:"start_time,omitempty"`
	EndTime   *time.Time      `json:"end_time,omitempty"`
	MinConf   *float64        `json:"min_confidence,omitempty"`
	ZoneID    string          `json:"zone_id,omitempty"`
	Handled   *bool           `json:"handled,omitempty"`
	Page      int             `json:"page"`
	PageSize  int             `json:"page_size"`
}

// AlertQuery 告警查询参数.
type AlertQuery struct {
	CameraID  string        `json:"camera_id,omitempty"`
	Levels    []AlertLevel  `json:"levels,omitempty"`
	Statuses  []AlertStatus `json:"statuses,omitempty"`
	StartTime *time.Time    `json:"start_time,omitempty"`
	EndTime   *time.Time    `json:"end_time,omitempty"`
	Page      int           `json:"page"`
	PageSize  int           `json:"page_size"`
}

// RecordingQuery 录像查询参数.
type RecordingQuery struct {
	CameraID  string     `json:"camera_id,omitempty"`
	StartTime *time.Time `json:"start_time,omitempty"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	HasEvents *bool      `json:"has_events,omitempty"`
	Page      int        `json:"page"`
	PageSize  int        `json:"page_size"`
}

// ========== 系统状态 ==========

// SystemStatus 系统状态.
type SystemStatus struct {
	TotalCameras    int     `json:"total_cameras"`
	OnlineCameras   int     `json:"online_cameras"`
	RecordingCount  int     `json:"recording_count"`
	TotalEvents     int     `json:"total_events"`
	ActiveAlerts    int     `json:"active_alerts"`
	StorageUsedGB   float64 `json:"storage_used_gb"`
	StorageTotalGB  float64 `json:"storage_total_gb"`
	AIModelsLoaded  int     `json:"ai_models_loaded"`
	AvgInferenceFPS float64 `json:"avg_inference_fps"`
}

// TimelineData 时间线数据.
type TimelineData struct {
	Date       time.Time         `json:"date"`
	CameraID   string            `json:"camera_id"`
	CameraName string            `json:"camera_name"`
	Recordings []TimelineSegment `json:"recordings"`
	Events     []TimelineEvent   `json:"events"`
}

// TimelineSegment 时间线片段.
type TimelineSegment struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	HasEvents bool      `json:"has_events"`
}

// TimelineEvent 时间线事件.
type TimelineEvent struct {
	Timestamp  time.Time     `json:"timestamp"`
	Type       DetectionType `json:"type"`
	Level      AlertLevel    `json:"level"`
	Confidence float64       `json:"confidence"`
	Thumbnail  string        `json:"thumbnail,omitempty"`
}
