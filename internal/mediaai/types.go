// Package mediaai 提供AI驱动的媒体智能管理功能
// 对标飞牛fnOS相册AI识别、群晖Synology Photos
// 核心能力：AI分类标签、人脸识别、智能推荐、跨设备同步
package mediaai

import (
	"fmt"
	"sync"
	"time"
)

// ============================================================
// AI Classifier Types
// ============================================================

// SceneCategory AI识别的场景分类
type SceneCategory string

const (
	SceneLandscape   SceneCategory = "landscape"   // 风景
	ScenePortrait    SceneCategory = "portrait"    // 人像
	SceneFood        SceneCategory = "food"        // 美食
	SceneAnimal      SceneCategory = "animal"      // 动物
	SceneArchitecture SceneCategory = "architecture" // 建筑
	SceneVehicle     SceneCategory = "vehicle"      // 交通工具
	SceneSports      SceneCategory = "sports"       // 运动
	SceneDocument    SceneCategory = "document"     // 文档
	SceneScreenshot  SceneCategory = "screenshot"   // 截图
	SceneNight       SceneCategory = "night"        // 夜景
	SceneIndoor      SceneCategory = "indoor"       // 室内
	SceneOutdoor     SceneCategory = "outdoor"      // 室外
	SceneConcert     SceneCategory = "concert"      // 演出/演唱会
	SceneNature      SceneCategory = "nature"       // 自然
	SceneCity        SceneCategory = "city"         // 城市
	SceneUnknown     SceneCategory = "unknown"
)

// ContentRating 内容质量评级
type ContentRating string

const (
	RatingExcellent ContentRating = "excellent" // 极佳
	RatingGood      ContentRating = "good"      // 良好
	RatingAverage   ContentRating = "average"   // 一般
	RatingPoor      ContentRating = "poor"      // 较差
	RatingBlurry    ContentRating = "blurry"    // 模糊
)

// AITag AI自动生成的标签
type AITag struct {
	Name       string  `json:"name"`
	Category   string  `json:"category"` // scene, object, emotion, style
	Confidence float64 `json:"confidence"` // 0.0 - 1.0
	Source     string  `json:"source"`     // ai_vision, ai_text, metadata
}

// ClassificationResult 分类结果
type ClassificationResult struct {
	MediaID      string          `json:"media_id"`
	Scenes       []SceneTag      `json:"scenes"`
	Objects      []ObjectTag     `json:"objects"`
	Emotions     []EmotionTag    `json:"emotions"`
	Style        []StyleTag      `json:"style"`
	AutoTags     []AITag         `json:"auto_tags"`
	Quality      ContentRating   `json:"quality"`
	QualityScore float64         `json:"quality_score"` // 0-100
	IsScreenshot bool            `json:"is_screenshot"`
	IsBlurry     bool            `json:"is_blurry"`
	NSFWScore    float64         `json:"nsfw_score"`    // 0-1, safe content
	AnalyzedAt   time.Time       `json:"analyzed_at"`
}

// SceneTag 场景标签
type SceneTag struct {
	Category   SceneCategory `json:"category"`
	Confidence float64       `json:"confidence"`
}

// ObjectTag 物体标签
type ObjectTag struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
	Box        *BBox   `json:"box,omitempty"`
}

// BBox 边界框
type BBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// EmotionTag 情感标签
type EmotionTag struct {
	Emotion    string  `json:"emotion"` // happy, sad, surprise, neutral
	Confidence float64 `json:"confidence"`
}

// StyleTag 风格标签
type StyleTag struct {
	Style      string  `json:"style"` // vintage, modern, cinematic, artistic
	Confidence float64 `json:"confidence"`
}

// ============================================================
// Face Recognition Types
// ============================================================

// Person 人物实体
type Person struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	IsNamed      bool      `json:"is_named"` // 是否已命名
	AvatarPath   string    `json:"avatar_path,omitempty"`
	FaceCount    int       `json:"face_count"`
	MediaCount   int       `json:"media_count"`
	FirstSeenAt  time.Time `json:"first_seen_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	Embedding    []float32 `json:"-"` // 人脸特征向量(不序列化)
}

// Face 人脸实例
type Face struct {
	ID         string    `json:"id"`
	MediaID    string    `json:"media_id"`
	PersonID   string    `json:"person_id"`
	Confidence float64   `json:"confidence"`
	Box        BBox      `json:"box"`
	Embedding  []float32 `json:"-"`
	Angle      FaceAngle `json:"angle"`
	Verified   bool      `json:"verified"` // 是否人工确认
}

// FaceAngle 人脸角度
type FaceAngle struct {
	Yaw   float64 `json:"yaw"`   // 左右转头
	Pitch float64 `json:"pitch"` // 上下点头
	Roll  float64 `json:"roll"`  // 歪头
}

// SmartAlbum 智能相册
type SmartAlbum struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Type         AlbumType   `json:"type"`
	PersonIDs   []string     `json:"person_ids,omitempty"`
	SceneTags   []string     `json:"scene_tags,omitempty"`
	Location    string       `json:"location,omitempty"`
	DateRange   *DateRange   `json:"date_range,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	MediaIDs    []string     `json:"media_ids"`
	MediaCount  int          `json:"media_count"`
	CoverPath   string       `json:"cover_path,omitempty"`
	IsAuto      bool         `json:"is_auto"` // 自动生成
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// AlbumType 相册类型
type AlbumType string

const (
	AlbumTypeFace     AlbumType = "face"      // 人脸相册
	AlbumTypeScene    AlbumType = "scene"     // 场景相册
	AlbumTypeLocation AlbumType = "location"  // 地点相册
	AlbumTypeTime     AlbumType = "time"      // 时间相册(某天/某月/某年)
	AlbumTypeTrip     AlbumType = "trip"      // 旅行相册
	AlbumTypeEvent    AlbumType = "event"     // 事件相册
	AlbumTypeCustom   AlbumType = "custom"    // 自定义
)

// DateRange 日期范围
type DateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// FaceClusterConfig 人脸聚类配置
type FaceClusterConfig struct {
	SimilarityThreshold float64 `json:"similarity_threshold"` // 0.0-1.0, 默认0.65
	MinClusterSize      int     `json:"min_cluster_size"`     // 最小聚类大小, 默认2
	MaxClusters         int     `json:"max_clusters"`         // 最大聚类数
	Dimension           int     `json:"dimension"`            // 嵌入维度, 默认128
}

// ============================================================
// Recommendation Types
// ============================================================

// Recommendation 推荐项
type Recommendation struct {
	MediaID      string             `json:"media_id"`
	Score        float64            `json:"score"`   // 0-100
	Reasons      []RecommendReason  `json:"reasons"`
	Type         RecommendationType `json:"type"`
	GeneratedAt  time.Time          `json:"generated_at"`
}

// RecommendationType 推荐类型
type RecommendationType string

const (
	RecTypeSimilar    RecommendationType = "similar"    // 相似推荐
	RecTypeContinue   RecommendationType = "continue"   // 继续观看
	RecTypeTrending   RecommendationType = "trending"   // 热门趋势
	RecTypeSeasonal   RecommendationType = "seasonal"   // 季节/节日相关
	RecTypeBecause    RecommendationType = "because"    // 因为你喜欢X
	RecTypeNew        RecommendationType = "new"        // 新添加
	RecTypeRediscover RecommendationType = "rediscover" // 重新发现(很久没看的)
)

// RecommendReason 推荐原因
type RecommendReason struct {
	Type    string `json:"type"`    // genre, actor, director, scene, tag
	Value   string `json:"value"`
	Weight  float64 `json:"weight"`
}

// UserProfile 用户画像
type UserProfile struct {
	UserID           string           `json:"user_id"`
	GenrePreferences map[string]float64 `json:"genre_preferences"`  // genre -> weight
	ActorPreferences map[string]float64 `json:"actor_preferences"`
	DirectorPrefs    map[string]float64 `json:"director_preferences"`
	ScenePreferences map[string]float64 `json:"scene_preferences"`
	TagPreferences   map[string]float64 `json:"tag_preferences"`
	QualityPreference string           `json:"quality_preference"`
	WatchHistory     []WatchEvent      `json:"watch_history"`
	Favorites        []string          `json:"favorites"`
	TotalWatchTime   int64             `json:"total_watch_time"` // seconds
	LastActive       time.Time         `json:"last_active"`
}

// WatchEvent 观看事件
type WatchEvent struct {
	MediaID    string    `json:"media_id"`
	StartTime  time.Time `json:"start_time"`
	Duration   int       `json:"duration"` // seconds watched
	Completed  bool      `json:"completed"`
	Rating     int       `json:"rating"` // user rating 1-5
}

// MediaFeature 媒体特征(用于相似度计算)
type MediaFeature struct {
	MediaID    string   `json:"media_id"`
	Genres     []string `json:"genres"`
	Tags       []string `json:"tags"`
	Actors     []string `json:"actors"`
	Directors  []string `json:"directors"`
	Scene      string   `json:"scene"`
	Year       int      `json:"year"`
	Rating     float64  `json:"rating"`
	Popularity float64  `json:"popularity"`
	Embedding  []float32 `json:"-"` // 内容嵌入向量
}

// ============================================================
// Sync Types
// ============================================================

// SyncDevice 同步设备
type SyncDevice struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // desktop, mobile, tablet, tv, nas
	Platform    string    `json:"platform"` // ios, android, windows, macos, linux
	LastSyncAt  time.Time `json:"last_sync_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
	Status      string    `json:"status"` // online, offline, syncing
	SyncVersion int64     `json:"sync_version"`
	SyncPaths   []string  `json:"sync_paths"` // 同步路径列表
	StorageUsed int64     `json:"storage_used"`
	StorageTotal int64    `json:"storage_total"`
}

// SyncTask 同步任务
type SyncTask struct {
	ID          string     `json:"id"`
	SourceDevice string    `json:"source_device"`
	TargetDevice string    `json:"target_device"`
	Status      string     `json:"status"` // pending, syncing, completed, failed, conflict
	TotalFiles  int        `json:"total_files"`
	SyncedFiles int        `json:"synced_files"`
	FailedFiles int        `json:"failed_files"`
	TotalBytes  int64      `json:"total_bytes"`
	SyncedBytes int64      `json:"synced_bytes"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string     `json:"error,omitempty"`
	Conflicts   []SyncConflict `json:"conflicts,omitempty"`
}

// SyncConflict 同步冲突
type SyncConflict struct {
	MediaID       string    `json:"media_id"`
	Path          string    `json:"path"`
	SourceVersion int64     `json:"source_version"`
	TargetVersion int64     `json:"target_version"`
	SourceModTime time.Time `json:"source_mod_time"`
	TargetModTime time.Time `json:"target_mod_time"`
	Resolution    string    `json:"resolution"` // keep_source, keep_target, keep_both, manual
}

// SyncState 同步状态
type SyncState struct {
	Version     int64     `json:"version"`
	LastSyncAt  time.Time `json:"last_sync_at"`
	Checksum    string    `json:"checksum"`
	DeviceID    string    `json:"device_id"`
}

// MediaSyncEntry 媒体同步条目
type MediaSyncEntry struct {
	MediaID     string    `json:"media_id"`
	Path        string    `json:"path"`
	Checksum    string    `json:"checksum"`
	Size        int64     `json:"size"`
	Version     int64     `json:"version"`
	ModifiedAt  time.Time `json:"modified_at"`
	Action      string    `json:"action"` // create, update, delete
}

// ============================================================
// Engine Types
// ============================================================

// Config AI媒体引擎配置
type Config struct {
	Enabled         bool               `json:"enabled"`
	ClassifierPath  string             `json:"classifier_path"` // AI模型路径
	FaceModelPath   string             `json:"face_model_path"`
	EmbeddingDim    int                `json:"embedding_dim"`
	FaceCluster     FaceClusterConfig  `json:"face_cluster"`
	RecommendLimit  int                `json:"recommend_limit"`
	SyncEnabled     bool               `json:"sync_enabled"`
	StoragePath     string             `json:"storage_path"` // 数据存储路径
	MaxConcurrency  int                `json:"max_concurrency"`
	AutoClassify    bool               `json:"auto_classify"`
	AutoFaceDetect  bool               `json:"auto_face_detect"`
	AutoSync        bool               `json:"auto_sync"`
	SyncInterval    time.Duration      `json:"sync_interval"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:        true,
		EmbeddingDim:   128,
		FaceCluster: FaceClusterConfig{
			SimilarityThreshold: 0.65,
			MinClusterSize:      2,
			MaxClusters:         1000,
			Dimension:           128,
		},
		RecommendLimit: 20,
		SyncEnabled:    true,
		MaxConcurrency: 4,
		AutoClassify:   true,
		AutoFaceDetect: true,
		AutoSync:       true,
		SyncInterval:   5 * time.Minute,
	}
}

// ============================================================
// Stats
// ============================================================

// AIStats AI媒体管理统计
type AIStats struct {
	TotalAnalyzed   int            `json:"total_analyzed"`
	TotalFaces      int            `json:"total_faces"`
	TotalPersons    int            `json:"total_persons"`
	NamedPersons    int            `json:"named_persons"`
	TotalAlbums     int            `json:"total_albums"`
	AutoAlbums      int            `json:"auto_albums"`
	TotalDevices    int            `json:"total_devices"`
	OnlineDevices   int            `json:"online_devices"`
	PendingSync     int            `json:"pending_sync"`
	LastClassifyAt  *time.Time     `json:"last_classify_at,omitempty"`
	LastFaceScanAt  *time.Time     `json:"last_face_scan_at,omitempty"`
	LastSyncAt      *time.Time     `json:"last_sync_at,omitempty"`
	SceneDistribution map[string]int `json:"scene_distribution"`
}

// NewAIStats 创建空统计
func NewAIStats() *AIStats {
	return &AIStats{
		SceneDistribution: make(map[string]int),
	}
}

// ============================================================
// Engine Error Types
// ============================================================

// ErrNotFound 未找到
type ErrNotFound struct {
	Resource string
	ID       string
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// ErrAlreadyExists 已存在
type ErrAlreadyExists struct {
	Resource string
	ID       string
}

func (e *ErrAlreadyExists) Error() string {
	return fmt.Sprintf("%s already exists: %s", e.Resource, e.ID)
}

// EngineInterface 引擎接口(用于测试mock)
type EngineInterface interface {
	// 分类
	ClassifyMedia(mediaID, filePath string) (*ClassificationResult, error)
	BatchClassify(mediaIDs []string) ([]*ClassificationResult, error)
	GetClassification(mediaID string) (*ClassificationResult, error)

	// 人脸
	DetectFaces(mediaID, filePath string) ([]*Face, error)
	CreatePerson(name string) (*Person, error)
	AssignFaceToPerson(faceID, personID string) error
	GetPersonFaces(personID string) ([]*Face, error)
	ListPersons() ([]*Person, error)
	RenamePerson(personID, name string) error
	MergePersons(sourceID, targetID string) error
	CreateSmartAlbum(name string, albumType AlbumType, opts ...SmartAlbumOption) (*SmartAlbum, error)
	GetSmartAlbums() ([]*SmartAlbum, error)

	// 推荐
	GetRecommendations(userID string, limit int) ([]*Recommendation, error)
	RecordWatch(userID string, event *WatchEvent) error
	FindSimilar(mediaID string, limit int) ([]*Recommendation, error)
	UpdateUserProfile(userID string, profile *UserProfile) error

	// 同步
	RegisterDevice(device *SyncDevice) error
	ListDevices() ([]*SyncDevice, error)
	StartSync(sourceDevice, targetDevice string) (*SyncTask, error)
	GetSyncStatus(taskID string) (*SyncTask, error)
	GetSyncState(deviceID string) (*SyncState, error)

	// 统计
	GetStats() *AIStats
}

// SmartAlbumOption 智能相册选项
type SmartAlbumOption func(*SmartAlbum)

// WithPersonIDs 设置人物ID
func WithPersonIDs(ids ...string) SmartAlbumOption {
	return func(a *SmartAlbum) {
		a.PersonIDs = ids
	}
}

// WithSceneTags 设置场景标签
func WithSceneTags(tags ...string) SmartAlbumOption {
	return func(a *SmartAlbum) {
		a.SceneTags = tags
	}
}

// WithLocation 设置地点
func WithLocation(loc string) SmartAlbumOption {
	return func(a *SmartAlbum) {
		a.Location = loc
	}
}

// WithDateRange 设置日期范围
func WithDateRange(start, end time.Time) SmartAlbumOption {
	return func(a *SmartAlbum) {
		a.DateRange = &DateRange{Start: start, End: end}
	}
}

// WithAlbumTags 设置标签过滤
func WithAlbumTags(tags ...string) SmartAlbumOption {
	return func(a *SmartAlbum) {
		a.Tags = tags
	}
}

// Ensure EngineInterface is implemented at compile time
// var _ EngineInterface = (*Engine)(nil) // TODO: implement engine methods

// Engine 前向声明 - 在 engine.go 中实现
type Engine struct {
	mu       sync.RWMutex
	config   *Config

	// 子系统存储
	classifications map[string]*ClassificationResult
	persons         map[string]*Person
	faces           map[string]*Face
	smartAlbums     map[string]*SmartAlbum
	recommendations map[string][]*Recommendation
	profiles        map[string]*UserProfile
	devices         map[string]*SyncDevice
	syncTasks       map[string]*SyncTask
	syncStates      map[string]*SyncState

	// 特征索引
	mediaFeatures map[string]*MediaFeature
	faceIndex     map[string][]string // personID -> faceIDs
	albumIndex    map[string][]string // mediaID -> albumIDs
}
