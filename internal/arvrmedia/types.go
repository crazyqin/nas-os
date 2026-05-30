// Package arvrmedia 提供 AR/VR 沉浸式媒体体验，包括360°全景查看、VR画廊、空间音频、3D模型查看、沉浸式影院和WebXR支持
package arvrmedia

import (
	"time"
)

// MediaType 媒体类型
type MediaType string

const (
	MediaTypePanorama  MediaType = "panorama"  // 360°全景照片
	MediaType360Video  MediaType = "360video"  // 360°全景视频
	MediaType3DModel   MediaType = "3dmodel"   // 3D模型
	MediaTypeImmersive MediaType = "immersive" // 沉浸式影院媒体
)

// ModelFormat 3D模型格式
type ModelFormat string

const (
	ModelFormatGLTF ModelFormat = "gltf"
	ModelFormatOBJ  ModelFormat = "obj"
	ModelFormatSTL  ModelFormat = "stl"
)

// ProjectionType 投影类型
type ProjectionType string

const (
	ProjectionEquirectangular ProjectionType = "equirectangular" // 等距柱状投影
	ProjectionCubemap          ProjectionType = "cubemap"         // 立方体贴图
	ProjectionFisheye          ProjectionType = "fisheye"         // 鱼眼投影
)

// AudioMode 空间音频模式
type AudioMode string

const (
	AudioModeBinaural    AudioMode = "binaural"    // 双耳音频
	AudioModeAmbisonic   AudioMode = "ambisonic"   // 环绕声
	AudioModeSpatialized AudioMode = "spatialized" // 空间化音频
)

// XRMode WebXR模式
type XRMode string

const (
	XRModeVR  XRMode = "vr"  // 虚拟现实
	XRModeAR  XRMode = "ar"  // 增强现实
	XRModeMR  XRMode = "mr"  // 混合现实
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"    // 等待中
	TaskStatusProcessing TaskStatus = "processing" // 处理中
	TaskStatusCompleted  TaskStatus = "completed"  // 已完成
	TaskStatusFailed     TaskStatus = "failed"     // 失败
)

// PanoramaMedia 360°全景媒体
type PanoramaMedia struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Path         string          `json:"path"`
	MimeType     string          `json:"mime_type"`
	Size         int64           `json:"size"`
	Width        int             `json:"width"`
	Height       int             `json:"height"`
	Projection   ProjectionType  `json:"projection"`
	IsVideo      bool            `json:"is_video"`
	Duration     float64         `json:"duration,omitempty"` // 视频时长(秒)
	ThumbnailPath string         `json:"thumbnail_path"`
	Tags         []string        `json:"tags"`
	Metadata     *PanoramaMeta   `json:"metadata,omitempty"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// PanoramaMeta 全景媒体元数据
type PanoramaMeta struct {
	FOV            float64 `json:"fov"`                       // 视场角
	Is3D           bool    `json:"is_3d"`                     // 是否立体3D
	StereoscopicLayout string `json:"stereoscopic_layout"` // 立体布局: top-bottom / left-right / mono
	InitialYaw     float64 `json:"initial_yaw"`               // 初始偏航角
	InitialPitch   float64 `json:"initial_pitch"`             // 初始俯仰角
	MinFOV         float64 `json:"min_fov"`                   // 最小视场角
	MaxFOV         float64 `json:"max_fov"`                   // 最大视场角
}

// Model3D 3D模型
type Model3D struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Path         string      `json:"path"`
	Format       ModelFormat `json:"format"`
	Size         int64       `json:"size"`
	VertexCount  int         `json:"vertex_count"`
	FaceCount    int         `json:"face_count"`
	HasTextures  bool        `json:"has_textures"`
	HasAnimation bool        `json:"has_animation"`
	TexturePaths []string    `json:"texture_paths,omitempty"`
	BoundingBox  *BoundingBox `json:"bounding_box,omitempty"`
	ThumbnailPath string     `json:"thumbnail_path"`
	Tags         []string    `json:"tags"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

// BoundingBox 包围盒
type BoundingBox struct {
	MinX float64 `json:"min_x"`
	MinY float64 `json:"min_y"`
	MinZ float64 `json:"min_z"`
	MaxX float64 `json:"max_x"`
	MaxY float64 `json:"max_y"`
	MaxZ float64 `json:"max_z"`
}

// VREntry VR画廊条目
type VREntry struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	ThumbnailPath string  `json:"thumbnail_path"`
	Layout      string    `json:"layout"`      // 布局: grid / wall / showcase
	MediaIDs    []string  `json:"media_ids"`    // 关联的媒体ID
	Background  string    `json:"background"`   // 背景场景: museum / space / forest / custom
	Lighting    string    `json:"lighting"`     // 灯光: warm / cool / natural / dramatic
	SkyboxPath  string    `json:"skybox_path"`  // 自定义天空盒路径
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SpatialAudioConfig 空间音频配置
type SpatialAudioConfig struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Mode           AudioMode `json:"mode"`
	SourcePosition *Vector3  `json:"source_position"` // 音源位置
	ListenerPosition *Vector3 `json:"listener_position"` // 听者位置
	Gain           float64   `json:"gain"`            // 音量增益 0.0-2.0
	DopplerFactor  float64   `json:"doppler_factor"`  // 多普勒因子
	RolloffFactor  float64   `json:"rolloff_factor"`  // 衰减因子
	RefDistance    float64   `json:"ref_distance"`    // 参考距离
	MaxDistance    float64   `json:"max_distance"`    // 最大距离
	RoomSize       string    `json:"room_size"`       // 房间大小: small / medium / large / hall
	ReverbLevel    float64   `json:"reverb_level"`    // 混响级别 0.0-1.0
	Enabled        bool      `json:"enabled"`
	CreatedAt      time.Time `json:"created_at"`
}

// Vector3 三维向量
type Vector3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// ImmersiveTheater 沉浸式影院配置
type ImmersiveTheater struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	ScreenType    string    `json:"screen_type"`    // 屏幕类型: flat / curved / dome / sphere
	ScreenWidth   float64   `json:"screen_width"`   // 屏幕宽度(米)
	ScreenHeight  float64   `json:"screen_height"`  // 屏幕高度(米)
	Distance      float64   `json:"distance"`       // 观看距离(米)
	Environment   string    `json:"environment"`    // 环境: cinema / livingroom / outdoor / space
	SeatPosition  *Vector3  `json:"seat_position"`  // 座位位置
	AudioConfig   *SpatialAudioConfig `json:"audio_config,omitempty"`
	MaxViewers    int       `json:"max_viewers"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// WebXRSession WebXR会话
type WebXRSession struct {
	ID        string    `json:"id"`
	Mode      XRMode    `json:"mode"`
	DeviceID  string    `json:"device_id"`
	Status    string    `json:"status"` // connecting / active / paused / ended
	StartTime time.Time `json:"start_time"`
	EndTime   *time.Time `json:"end_time,omitempty"`
	Capabilities []string `json:"capabilities"`
	FrameRate int       `json:"frame_rate"`
	Resolution *Resolution `json:"resolution"`
}

// Resolution 分辨率
type Resolution struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ImportTask 媒体导入任务
type ImportTask struct {
	ID          string     `json:"id"`
	Status      TaskStatus `json:"status"`
	SourcePath  string     `json:"source_path"`
	MediaType   MediaType  `json:"media_type"`
	TotalFiles  int        `json:"total_files"`
	Processed   int        `json:"processed"`
	Failed      int        `json:"failed"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Errors      []string   `json:"errors"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Panoramas []PanoramaMedia `json:"panoramas,omitempty"`
	Models    []Model3D       `json:"models,omitempty"`
	Galleries []VREntry       `json:"galleries,omitempty"`
	Total     int             `json:"total"`
	Page      int             `json:"page"`
	PageSize  int             `json:"page_size"`
	HasMore   bool            `json:"has_more"`
}

// ARVRStats AR/VR 媒体统计
type ARVRStats struct {
	TotalPanoramas  int   `json:"total_panoramas"`
	TotalVideos360  int   `json:"total_videos_360"`
	TotalModels3D   int   `json:"total_models_3d"`
	TotalGalleries  int   `json:"total_galleries"`
	TotalTheaters   int   `json:"total_theaters"`
	TotalSize       int64 `json:"total_size"`
	ActiveSessions  int   `json:"active_sessions"`
}
