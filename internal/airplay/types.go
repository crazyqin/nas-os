// Package airplay 提供 AirPlay 音视频投射服务功能
// AirPlay 接收器管理、发送器管理、设备发现、音视频流管理、屏幕镜像、多房间音频、设备配对认证
package airplay

import (
	"time"
)

// ========== AirPlay 设备 ==========

// AirPlayDevice AirPlay 设备.
type AirPlayDevice struct {
	ID           string             `json:"id"`           // 唯一标识
	Name         string             `json:"name"`         // 设备名称
	Type         DeviceType         `json:"type"`         // 设备类型
	IP           string             `json:"ip"`           // IP 地址
	Port         int                `json:"port"`         // 端口号
	Online       bool               `json:"online"`       // 是否在线
	Capabilities DeviceCapabilities `json:"capabilities"` // 支持的功能
	LastSeen     time.Time          `json:"lastSeen"`     // 最后发现时间
}

// DeviceType 设备类型.
type DeviceType string

const (
	DeviceTypeAppleTV DeviceType = "AppleTV"
	DeviceTypeHomePod DeviceType = "HomePod"
	DeviceTypeSpeaker DeviceType = "Speaker"
	DeviceTypeNAS     DeviceType = "NAS"
)

// DeviceCapabilities 设备支持的功能.
type DeviceCapabilities struct {
	Audio  bool `json:"audio"`  // 支持音频
	Video  bool `json:"video"`  // 支持视频
	Screen bool `json:"screen"` // 支持屏幕镜像
}

// ========== AirPlay 接收器 ==========

// AirPlayReceiver AirPlay 接收器配置.
type AirPlayReceiver struct {
	ID                string          `json:"id"`                 // 唯一标识
	Name              string          `json:"name"`               // 接收器名称
	Enabled           bool            `json:"enabled"`            // 是否启用
	Port              int             `json:"port"`               // 监听端口
	PasswordProtected bool            `json:"passwordProtected"`  // 是否密码保护
	Password          string          `json:"password,omitempty"` // 访问密码
	PairedDevices     []PairingRecord `json:"pairedDevices"`      // 已配对设备列表
}

// ========== AirPlay 发送器 ==========

// AirPlaySender AirPlay 发送器状态.
type AirPlaySender struct {
	ID         string       `json:"id"`         // 唯一标识
	TargetID   string       `json:"targetId"`   // 当前投射目标设备 ID
	TargetName string       `json:"targetName"` // 目标设备名称
	Status     SenderStatus `json:"status"`     // 发送器状态
	MediaInfo  *MediaInfo   `json:"mediaInfo"`  // 当前媒体信息
}

// SenderStatus 发送器状态.
type SenderStatus string

const (
	SenderStatusIdle    SenderStatus = "idle"
	SenderStatusCasting SenderStatus = "casting"
	SenderStatusError   SenderStatus = "error"
)

// ========== 音频流 ==========

// AudioStream 音频流状态.
type AudioStream struct {
	ID           string       `json:"id"`           // 唯一标识
	DeviceID     string       `json:"deviceId"`     // 目标设备 ID
	Status       StreamStatus `json:"status"`       // 流状态
	Volume       int          `json:"volume"`       // 音量 (0-100)
	CurrentTrack *MediaInfo   `json:"currentTrack"` // 当前曲目
	Queue        []MediaInfo  `json:"queue"`        // 播放队列
	QueueIndex   int          `json:"queueIndex"`   // 当前播放索引
}

// StreamStatus 流状态.
type StreamStatus string

const (
	StreamStatusPlaying StreamStatus = "playing"
	StreamStatusPaused  StreamStatus = "paused"
	StreamStatusStopped StreamStatus = "stopped"
)

// ========== 视频流 ==========

// VideoStream 视频流状态.
type VideoStream struct {
	ID         string       `json:"id"`         // 唯一标识
	DeviceID   string       `json:"deviceId"`   // 目标设备 ID
	Status     StreamStatus `json:"status"`     // 流状态
	Resolution string       `json:"resolution"` // 分辨率 (如 1920x1080)
	Bitrate    int          `json:"bitrate"`    // 码率 (kbps)
	Media      *MediaInfo   `json:"media"`      // 当前媒体信息
}

// ========== 屏幕镜像 ==========

// ScreenMirror 屏幕镜像状态.
type ScreenMirror struct {
	ID           string `json:"id"`           // 唯一标识
	SourceDevice string `json:"sourceDevice"` // 源设备
	TargetDevice string `json:"targetDevice"` // 目标设备
	Resolution   string `json:"resolution"`   // 分辨率
	FrameRate    int    `json:"frameRate"`    // 帧率
	Latency      int    `json:"latency"`      // 延迟 (ms)
	Active       bool   `json:"active"`       // 是否活跃
}

// ========== 多房间音频 ==========

// MultiRoomGroup 多房间音频组.
type MultiRoomGroup struct {
	ID         string     `json:"id"`         // 唯一标识
	Name       string     `json:"name"`       // 组名称
	MasterID   string     `json:"masterId"`   // 主设备 ID
	SlaveIDs   []string   `json:"slaveIds"`   // 从设备 ID 列表
	SyncStatus SyncStatus `json:"syncStatus"` // 同步状态
}

// SyncStatus 同步状态.
type SyncStatus string

const (
	SyncStatusSynced    SyncStatus = "synced"
	SyncStatusSyncing   SyncStatus = "syncing"
	SyncStatusOutOfSync SyncStatus = "out_of_sync"
)

// ========== 媒体信息 ==========

// MediaInfo 媒体信息.
type MediaInfo struct {
	Title    string `json:"title"`    // 标题
	Artist   string `json:"artist"`   // 艺术家
	Album    string `json:"album"`    // 专辑
	CoverURL string `json:"coverUrl"` // 封面 URL
	Duration int    `json:"duration"` // 时长 (秒)
	Position int    `json:"position"` // 当前位置 (秒)
}

// ========== 设备配对 ==========

// PairingRecord 设备配对记录.
type PairingRecord struct {
	DeviceID string    `json:"deviceId"` // 设备 ID
	Name     string    `json:"name"`     // 设备名称
	PairedAt time.Time `json:"pairedAt"` // 配对时间
	Trusted  bool      `json:"trusted"`  // 是否信任
}

// ========== 统计数据 ==========

// AirPlayStats AirPlay 统计数据.
type AirPlayStats struct {
	SenderCount   int   `json:"senderCount"`   // 发送设备数
	ReceiverCount int   `json:"receiverCount"` // 接收设备数
	ActiveStreams int   `json:"activeStreams"` // 活跃流数
	TotalTraffic  int64 `json:"totalTraffic"`  // 总流量 (bytes)
}

// ========== 服务状态 ==========

// ServiceStatus AirPlay 服务状态.
type ServiceStatus struct {
	Running   bool       `json:"running"`   // 是否运行中
	StartedAt *time.Time `json:"startedAt"` // 启动时间
	Devices   int        `json:"devices"`   // 发现的设备数
	Streams   int        `json:"streams"`   // 活跃流数
}
