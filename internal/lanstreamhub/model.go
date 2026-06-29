// Package lanstreamhub 局域网直播源中心
// 对标飞牛 fnOS 局域网直播源功能：自动发现局域网内的 UPnP/DLNA 设备、RTSP/HLS 流，
// 生成 M3U 播放列表，提供流转发与转码管理，让家里的直播内容接入观看更方便。
package lanstreamhub

import (
	"time"
)

// StreamProtocol 流媒体协议类型
type StreamProtocol string

const (
	ProtocolHLS  StreamProtocol = "hls"  // HLS (HTTP Live Streaming)
	ProtocolRTSP StreamProtocol = "rtsp" // RTSP 实时流传输协议
	ProtocolUDP  StreamProtocol = "udp"  // UDP/RTP 组播
	ProtocolFile StreamProtocol = "file" // 本地文件路径
)

// SourceCategory 直播源分类，对应飞牛 fnOS 的三类接入
type SourceCategory string

const (
	CategoryLive    SourceCategory = "live"    // 电视直播
	CategoryMedia   SourceCategory = "media"   // 媒体库
	CategoryApp     SourceCategory = "app"     // 影视应用
	CategoryCamera  SourceCategory = "camera"  // 网络摄像头
	CategoryScreen  SourceCategory = "screen"  // 屏幕镜像
	CategoryCustom  SourceCategory = "custom"  // 自定义
)

// DeviceType 局域网设备类型
type DeviceType string

const (
	DeviceTV          DeviceType = "tv"          // 智能电视
	DeviceSTB         DeviceType = "stb"         // 机顶盒
	DeviceNAS         DeviceType = "nas"         // NAS 存储
	DeviceCamera      DeviceType = "camera"      // 网络摄像头
	DeviceMediaServer DeviceType = "media_server" // 媒体服务器 (DLNA/UPnP)
	DevicePhone       DeviceType = "phone"       // 手机/平板
	DevicePC          DeviceType = "pc"          // 个人电脑
	DeviceUnknown     DeviceType = "unknown"     // 未知设备
)

// SourceStatus 直播源状态
type SourceStatus string

const (
	StatusOnline  SourceStatus = "online"  // 在线可用
	StatusOffline SourceStatus = "offline" // 离线
	StatusTesting SourceStatus = "testing" // 检测中
)

// StreamSource 直播源，描述一个局域网内可观看的流
type StreamSource struct {
	ID          string         `json:"id"`                    // 唯一标识
	Name        string         `json:"name"`                  // 显示名称
	URL         string         `json:"url"`                   // 流地址 (rtsp:// / http://...m3u8 / udp://@ / /path/to/file)
	Protocol    StreamProtocol `json:"protocol"`              // 流协议
	Category    SourceCategory `json:"category"`              // 内容分类
	Logo        string         `json:"logo,omitempty"`        // 频道台标 URL
	Group       string         `json:"group,omitempty"`       // 所属分组名称
	DeviceID    string         `json:"deviceId,omitempty"`    // 来源设备 ID
	DeviceType  DeviceType     `json:"deviceType,omitempty"`  // 来源设备类型
	DeviceName  string         `json:"deviceName,omitempty"`  // 来源设备名称
	Resolution  string         `json:"resolution,omitempty"`  // 分辨率 (如 1920x1080)
	Codec       string         `json:"codec,omitempty"`       // 编码格式 (如 H.264/H.265)
	Bitrate     int            `json:"bitrate,omitempty"`     // 码率 (kbps)
	BackupURLs  []string       `json:"backupUrls,omitempty"`  // 备用流地址
	Tags        []string       `json:"tags,omitempty"`        // 标签
	Status      SourceStatus   `json:"status"`                // 当前状态
	LastSeen    time.Time      `json:"lastSeen,omitempty"`    // 最后发现时间
	LastCheck   time.Time      `json:"lastCheck,omitempty"`   // 最后检测时间
	CreatedAt   time.Time      `json:"createdAt"`             // 创建时间
	UpdatedAt   time.Time      `json:"updatedAt"`             // 更新时间
}

// ChannelGroup 频道分组，用于播放列表中的频道编排
type ChannelGroup struct {
	ID       string   `json:"id"`              // 分组 ID
	Name     string   `json:"name"`            // 分组名称 (如 "央视"、"本地媒体")
	Icon     string   `json:"icon,omitempty"`  // 分组图标 URL
	Order    int      `json:"order"`           // 排序权重，越小越靠前
	SourceIDs []string `json:"sourceIds"`      // 包含的直播源 ID 列表
}

// PlaylistEntry 播放列表条目，对应 M3U 中的一条频道信息
type PlaylistEntry struct {
	Name      string         `json:"name"`                // 频道显示名称
	URL       string         `json:"url"`                 // 播放地址
	Logo      string         `json:"logo,omitempty"`      // 台标
	Group     string         `json:"group,omitempty"`     // 分组名称
	TvgID     string         `json:"tvgId,omitempty"`     // EPG 频道 ID
	TvgName   string         `json:"tvgName,omitempty"`   // EPG 频道名称
	TvgLogo   string         `json:"tvgLogo,omitempty"`   // EPG 台标
	Protocol  StreamProtocol `json:"protocol,omitempty"`  // 流协议
	SourceID  string         `json:"sourceId,omitempty"`  // 关联的直播源 ID
	Order     int            `json:"order,omitempty"`     // 排序权重
}

// DiscoveredDevice 发现的局域网设备信息
type DiscoveredDevice struct {
	ID          string     `json:"id"`                    // 设备唯一标识 (通常为 MAC 或 USN)
	Name        string     `json:"name"`                  // 设备友好名称
	Type        DeviceType `json:"type"`                  // 设备类型
	IP          string     `json:"ip"`                    // IP 地址
	Port        int        `json:"port,omitempty"`         // 端口
	Manufacturer string    `json:"manufacturer,omitempty"`// 制造商
	Model       string     `json:"model,omitempty"`        // 型号
	USN         string     `json:"usn,omitempty"`         // UPnP USN 标识
	Services    []string   `json:"services,omitempty"`    // 支持的 UPnP 服务
	StreamURLs  []string   `json:"streamUrls,omitempty"`  // 发现的流地址
	DiscoveredAt time.Time `json:"discoveredAt"`          // 发现时间
}

// TranscodeProfile 转码配置
type TranscodeProfile struct {
	Name       string `json:"name"`                  // 配置名称
	VideoCodec string `json:"videoCodec"`            // 视频编码 (如 h264, hevc)
	AudioCodec string `json:"audioCodec"`            // 音频编码 (如 aac, mp3)
	Resolution string `json:"resolution,omitempty"`  // 目标分辨率 (如 1280x720)
	Bitrate    int    `json:"bitrate,omitempty"`     // 目标码率 (kbps)
	FrameRate  int    `json:"frameRate,omitempty"`   // 帧率
	Format     string `json:"format,omitempty"`      // 封装格式 (如 ts, mp4)
}

// StreamSession 流转发会话
type StreamSession struct {
	ID           string           `json:"id"`                      // 会话 ID
	SourceID     string           `json:"sourceId"`               // 源直播源 ID
	SourceURL    string           `json:"sourceUrl"`              // 源流地址
	SourceName   string           `json:"sourceName"`             // 源名称
	TargetURL    string           `json:"targetUrl,omitempty"`    // 转发目标地址
	Profile      *TranscodeProfile `json:"profile,omitempty"`    // 转码配置 (nil 表示直通)
	Active       bool             `json:"active"`                 // 是否活跃
	ClientCount  int              `json:"clientCount"`            // 当前客户端数
	StartedAt    time.Time        `json:"startedAt"`              // 启动时间
	BytesIn      int64            `json:"bytesIn"`                // 接收字节数
	BytesOut     int64            `json:"bytesOut"`               // 发送字节数
}

// BandwidthTier 带宽适配等级
type BandwidthTier string

const (
	BandwidthAuto    BandwidthTier = "auto"    // 自动适配
	BandwidthHigh    BandwidthTier = "high"    // 高画质 (≥10Mbps)
	BandwidthMedium  BandwidthTier = "medium"  // 中画质 (3-10Mbps)
	BandwidthLow     BandwidthTier = "low"     // 低画质 (≤3Mbps)
)

// BandwidthConfig 带宽适配配置
type BandwidthConfig struct {
	Mode        BandwidthTier `json:"mode"`                  // 适配模式
	MaxBitrate  int           `json:"maxBitrate,omitempty"`   // 最大码率 (kbps)
	MinBitrate  int           `json:"minBitrate,omitempty"`   // 最小码率 (kbps)
	CurrentTier BandwidthTier `json:"currentTier"`            // 当前实际等级
}
