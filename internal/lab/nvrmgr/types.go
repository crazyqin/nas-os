// Package nvrmgr 提供 NVR 视频管理功能
// 支持摄像头管理、录像回放、移动侦测、告警、存储策略等
package nvrmgr

import "time"

// ========== 摄像头相关 ==========

// CameraProtocol 摄像头协议.
type CameraProtocol string

const (
	ProtocolRTSP  CameraProtocol = "RTSP"
	ProtocolONVIF CameraProtocol = "ONVIF"
	ProtocolHTTP  CameraProtocol = "HTTP"
)

// CameraStatus 摄像头状态.
type CameraStatus string

const (
	CameraStatusOnline  CameraStatus = "online"
	CameraStatusOffline CameraStatus = "offline"
	CameraStatusError   CameraStatus = "error"
)

// Camera 摄像头配置.
type Camera struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	URL        string         `json:"url"`
	Protocol   CameraProtocol `json:"protocol"`
	Status     CameraStatus   `json:"status"`
	Resolution string         `json:"resolution,omitempty"` // 1920x1080
	FPS        int            `json:"fps,omitempty"`
	Codec      string         `json:"codec,omitempty"`
	Location   string         `json:"location,omitempty"`
	Enabled    bool           `json:"enabled"`
	LastSeen   time.Time      `json:"lastSeen"`
	CreatedAt  time.Time      `json:"createdAt"`
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// ========== 录像相关 ==========

// Recording 录像记录.
type Recording struct {
	ID            string    `json:"id"`
	CameraID      string    `json:"cameraId"`
	StartTime     time.Time `json:"startTime"`
	EndTime       time.Time `json:"endTime,omitempty"`
	FilePath      string    `json:"filePath"`
	Size          int64     `json:"size"`     // 字节
	Duration      int64     `json:"duration"` // 秒
	HasMotion     bool      `json:"hasMotion"`
	HasAlert      bool      `json:"hasAlert"`
	ThumbnailPath string    `json:"thumbnailPath,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
}

// ========== 移动侦测 ==========

// MotionEvent 移动侦测事件.
type MotionEvent struct {
	ID           string    `json:"id"`
	CameraID     string    `json:"cameraId"`
	Timestamp    time.Time `json:"timestamp"`
	Duration     int       `json:"duration"`   // 秒
	Confidence   float64   `json:"confidence"` // 0.0-1.0
	Zone         string    `json:"zone,omitempty"`
	SnapshotPath string    `json:"snapshotPath,omitempty"`
	RecordingID  string    `json:"recordingId,omitempty"`
}

// MotionRule 移动侦测规则.
type MotionRule struct {
	CameraID    string    `json:"cameraId"`
	Zone        string    `json:"zone"`
	Sensitivity float64   `json:"sensitivity"` // 0.0-1.0
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
}

// ========== 告警相关 ==========

// AlertType 告警类型.
type AlertType string

const (
	AlertMotion  AlertType = "motion"
	AlertPerson  AlertType = "person"
	AlertVehicle AlertType = "vehicle"
	AlertObject  AlertType = "object"
)

// Alert 告警.
type Alert struct {
	ID           string    `json:"id"`
	CameraID     string    `json:"cameraId"`
	Type         AlertType `json:"type"`
	Message      string    `json:"message"`
	Timestamp    time.Time `json:"timestamp"`
	SnapshotPath string    `json:"snapshotPath,omitempty"`
	Acknowledged bool      `json:"acknowledged"`
	AckedAt      time.Time `json:"ackedAt,omitempty"`
	AckedBy      string    `json:"ackedBy,omitempty"`
}

// ========== 存储策略 ==========

// StoragePlan 存储策略.
type StoragePlan struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	RetentionDays int       `json:"retentionDays"`
	MaxSize       int64     `json:"maxSize"` // 字节
	Cameras       []string  `json:"cameras,omitempty"`
	Quality       string    `json:"quality,omitempty"`  // high/medium/low
	Schedule      string    `json:"schedule,omitempty"` // 24x7/weekday/weekend
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// ========== 时间线 ==========

// TimelineSegment 时间线片段.
type TimelineSegment struct {
	StartTime    time.Time `json:"startTime"`
	EndTime      time.Time `json:"endTime"`
	HasRecording bool      `json:"hasRecording"`
	HasMotion    bool      `json:"hasMotion"`
}

// Timeline 时间线.
type Timeline struct {
	CameraID string            `json:"cameraId"`
	Date     string            `json:"date"` // YYYY-MM-DD
	Segments []TimelineSegment `json:"segments"`
}

// ========== 存储统计 ==========

// StorageUsage 存储使用情况.
type StorageUsage struct {
	CameraID       string `json:"cameraId"`
	UsedBytes      int64  `json:"usedBytes"`
	TotalBytes     int64  `json:"totalBytes"`
	RecordingCount int    `json:"recordingCount"`
}
