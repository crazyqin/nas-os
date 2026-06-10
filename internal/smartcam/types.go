// Package smartcam 提供智能摄像头管理系统
// 参考群晖 Surveillance Station 设计理念，支持摄像头发现、配置、录像管理和移动侦测。
package smartcam

import (
	"time"
)

// ========== 错误定义 ==========

// 错误常量.
const (
	ErrCameraNotFound      = "摄像头不存在"
	ErrCameraOffline       = "摄像头离线"
	ErrInvalidConfig       = "无效的摄像头配置"
	ErrDuplicateCamera     = "摄像头已存在"
	ErrRecordingInProgress = "录像正在进行中"
	ErrRecordingNotFound   = "录像不存在"
	ErrMotionZoneInvalid   = "无效的移动侦测区域"
	ErrScheduleConflict    = "录像计划冲突"
	ErrStorageFull         = "存储空间已满"
	ErrStreamFailed        = "视频流连接失败"
)

// ========== 核心类型 ==========

// CameraStatus 摄像头状态.
type CameraStatus string

// 摄像头状态常量定义.
const (
	// CameraStatusOnline 摄像头在线.
	CameraStatusOnline CameraStatus = "online"
	// CameraStatusOffline 摄像头离线.
	CameraStatusOffline CameraStatus = "offline"
	// CameraStatusError 摄像头错误.
	CameraStatusError CameraStatus = "error"
	// CameraStatusDisabled 摄像头已禁用.
	CameraStatusDisabled CameraStatus = "disabled"
)

// RecordingMode 录像模式.
type RecordingMode string

// 录像模式常量定义.
const (
	// RecordingModeContinuous 持续录像.
	RecordingModeContinuous RecordingMode = "continuous"
	// RecordingModeMotion 移动侦测录像.
	RecordingModeMotion RecordingMode = "motion"
	// RecordingModeSchedule 计划录像.
	RecordingModeSchedule RecordingMode = "schedule"
	// RecordingModeManual 手动录像.
	RecordingModeManual RecordingMode = "manual"
)

// StreamProtocol 视频流协议.
type StreamProtocol string

// 视频流协议常量定义.
const (
	// ProtocolRTSP RTSP 协议.
	ProtocolRTSP StreamProtocol = "rtsp"
	// ProtocolONVIF ONVIF 协议.
	ProtocolONVIF StreamProtocol = "onvif"
	// ProtocolHTTP HTTP 协议.
	ProtocolHTTP StreamProtocol = "http"
)

// MotionSensitivity 移动侦测灵敏度.
type MotionSensitivity string

// 移动侦测灵敏度常量定义.
const (
	// SensitivityLow 低灵敏度.
	SensitivityLow MotionSensitivity = "low"
	// SensitivityMedium 中灵敏度.
	SensitivityMedium MotionSensitivity = "medium"
	// SensitivityHigh 高灵敏度.
	SensitivityHigh MotionSensitivity = "high"
)

// Camera 摄像头配置.
type Camera struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Location     string          `json:"location"`
	IPAddress    string          `json:"ipAddress"`
	Port         int             `json:"port"`
	Protocol     StreamProtocol  `json:"protocol"`
	StreamURL    string          `json:"streamUrl"`
	Username     string          `json:"username"`
	Password     string          `json:"password,omitempty"` // 不返回给前端
	Model        string          `json:"model"`
	Manufacturer string          `json:"manufacturer"`
	Firmware     string          `json:"firmware"`
	Status       CameraStatus    `json:"status"`
	Resolution   string          `json:"resolution"` // 1080p, 4K, etc.
	FrameRate    int             `json:"frameRate"`
	Enabled      bool            `json:"enabled"`
	Recording    RecordingConfig `json:"recording"`
	Motion       MotionConfig    `json:"motion"`
	Tags         []string        `json:"tags"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
	LastSeen     *time.Time      `json:"lastSeen,omitempty"`
}

// RecordingConfig 录像配置.
type RecordingConfig struct {
	Enabled       bool          `json:"enabled"`
	Mode          RecordingMode `json:"mode"`
	Quality       string        `json:"quality"`       // high, medium, low
	PreRecord     int           `json:"preRecord"`     // 预录秒数
	PostRecord    int           `json:"postRecord"`    // 后录秒数
	MaxDuration   int           `json:"maxDuration"`   // 最大录像时长（秒）
	RetentionDays int           `json:"retentionDays"` // 保留天数
	Schedule      []TimeSlot    `json:"schedule"`      // 录像时间表
}

// TimeSlot 时间段.
type TimeSlot struct {
	DayOfWeek int    `json:"dayOfWeek"` // 0-6, 0=Sunday
	StartTime string `json:"startTime"` // HH:MM
	EndTime   string `json:"endTime"`   // HH:MM
}

// MotionConfig 移动侦测配置.
type MotionConfig struct {
	Enabled     bool              `json:"enabled"`
	Sensitivity MotionSensitivity `json:"sensitivity"`
	Threshold   float64           `json:"threshold"`   // 0-100
	Zones       []MotionZone      `json:"zones"`       // 侦测区域
	Actions     []MotionAction    `json:"actions"`     // 触发动作
	CooldownSec int               `json:"cooldownSec"` // 冷却时间（秒）
}

// MotionZone 移动侦测区域.
type MotionZone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	X      int    `json:"x"` // 左上角 X
	Y      int    `json:"y"` // 左上角 Y
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// MotionAction 移动侦测触发动作.
type MotionAction struct {
	Type    string `json:"type"`   // email, webhook, snapshot, record
	Target  string `json:"target"` // 邮箱地址、webhook URL 等
	Enabled bool   `json:"enabled"`
}

// ========== 录像相关类型 ==========

// Recording 录像记录.
type Recording struct {
	ID           string        `json:"id"`
	CameraID     string        `json:"cameraId"`
	CameraName   string        `json:"cameraName"`
	StartTime    time.Time     `json:"startTime"`
	EndTime      time.Time     `json:"endTime"`
	Duration     time.Duration `json:"duration"`
	FileSize     int64         `json:"fileSize"` // bytes
	FilePath     string        `json:"filePath"`
	Thumbnail    string        `json:"thumbnail"`
	Trigger      string        `json:"trigger"` // manual, motion, schedule, continuous
	MotionEvents int           `json:"motionEvents"`
	Tags         []string      `json:"tags"`
}

// ========== 移动侦测事件类型 ==========

// MotionEvent 移动侦测事件.
type MotionEvent struct {
	ID          string       `json:"id"`
	CameraID    string       `json:"cameraId"`
	CameraName  string       `json:"cameraName"`
	Timestamp   time.Time    `json:"timestamp"`
	ZoneID      string       `json:"zoneId"`
	ZoneName    string       `json:"zoneName"`
	Confidence  float64      `json:"confidence"` // 0-1
	BoundingBox *BoundingBox `json:"boundingBox"`
	Snapshot    string       `json:"snapshot"` // 快照路径
	Handled     bool         `json:"handled"`
}

// BoundingBox 检测框.
type BoundingBox struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ========== 请求/响应类型 ==========

// AddCameraRequest 添加摄像头请求.
type AddCameraRequest struct {
	Name       string           `json:"name" binding:"required"`
	Location   string           `json:"location"`
	IPAddress  string           `json:"ipAddress" binding:"required"`
	Port       int              `json:"port"`
	Protocol   StreamProtocol   `json:"protocol"`
	Username   string           `json:"username"`
	Password   string           `json:"password"`
	Resolution string           `json:"resolution"`
	FrameRate  int              `json:"frameRate"`
	Recording  *RecordingConfig `json:"recording"`
	Motion     *MotionConfig    `json:"motion"`
	Tags       []string         `json:"tags"`
}

// UpdateCameraRequest 更新摄像头请求.
type UpdateCameraRequest struct {
	Name       *string         `json:"name"`
	Location   *string         `json:"location"`
	IPAddress  *string         `json:"ipAddress"`
	Port       *int            `json:"port"`
	Protocol   *StreamProtocol `json:"protocol"`
	Username   *string         `json:"username"`
	Password   *string         `json:"password"`
	Resolution *string         `json:"resolution"`
	FrameRate  *int            `json:"frameRate"`
	Enabled    *bool           `json:"enabled"`
	Tags       []string        `json:"tags"`
}

// StartRecordingRequest 开始录像请求.
type StartRecordingRequest struct {
	CameraID string `json:"cameraId" binding:"required"`
	Mode     string `json:"mode"` // manual, motion
}

// MotionEventQuery 移动侦测事件查询.
type MotionEventQuery struct {
	CameraID  string `form:"cameraId"`
	StartTime string `form:"startTime"`
	EndTime   string `form:"endTime"`
	Limit     int    `form:"limit"`
}

// ========== 系统状态类型 ==========

// SystemStatus 系统状态.
type SystemStatus struct {
	TotalCameras    int    `json:"totalCameras"`
	OnlineCameras   int    `json:"onlineCameras"`
	OfflineCameras  int    `json:"offlineCameras"`
	RecordingCount  int    `json:"recordingCount"`
	TotalRecordings int    `json:"totalRecordings"`
	StorageUsed     int64  `json:"storageUsed"`  // bytes
	StorageTotal    int64  `json:"storageTotal"` // bytes
	MotionEvents24h int    `json:"motionEvents24h"`
	Uptime          string `json:"uptime"`
}

// StorageStats 存储统计.
type StorageStats struct {
	TotalRecordings int        `json:"totalRecordings"`
	TotalSize       int64      `json:"totalSize"`
	OldestRecording *time.Time `json:"oldestRecording,omitempty"`
	NewestRecording *time.Time `json:"newestRecording,omitempty"`
	AvgFileSize     int64      `json:"avgFileSize"`
}

// DiscoverResult 发现结果.
type DiscoverResult struct {
	Found      int      `json:"found"`
	Cameras    []Camera `json:"cameras"`
	ScannedIPs int      `json:"scannedIPs"`
	Elapsed    string   `json:"elapsed"`
}
