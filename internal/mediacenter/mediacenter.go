// Package mediacenter 实现媒体库管理器
// 学习飞牛影视库功能，提供智能分类、自动刮削、转码播放
package mediacenter

import (
	"fmt"
	"sync"
	"time"
)

// MediaType 媒体类型
type MediaType string

const (
	// MediaTypeMovie 电影
	MediaTypeMovie MediaType = "movie"
	// MediaTypeTVShow 电视剧
	MediaTypeTVShow MediaType = "tvshow"
	// MediaTypeMusic 音乐
	MediaTypeMusic MediaType = "music"
	// MediaTypePhoto 照片
	MediaTypePhoto MediaType = "photo"
	// MediaTypeOther 其他
	MediaTypeOther MediaType = "other"
)

// MediaStatus 媒体状态
type MediaStatus string

const (
	// MediaStatusAvailable 可用
	MediaStatusAvailable MediaStatus = "available"
	// MediaStatusProcessing 处理中
	MediaStatusProcessing MediaStatus = "processing"
	// MediaStatusError 错误
	MediaStatusError MediaStatus = "error"
	// MediaStatusUnavailable 不可用
	MediaStatusUnavailable MediaStatus = "unavailable"
)

// TranscodeStatus 转码状态
type TranscodeStatus string

const (
	// TranscodeStatusPending 待处理
	TranscodeStatusPending TranscodeStatus = "pending"
	// TranscodeStatusTranscoding 转码中
	TranscodeStatusTranscoding TranscodeStatus = "transcoding"
	// TranscodeStatusCompleted 完成
	TranscodeStatusCompleted TranscodeStatus = "completed"
	// TranscodeStatusFailed 失败
	TranscodeStatusFailed TranscodeStatus = "failed"
)

// MediaItem 媒体项
type MediaItem struct {
	// ID 媒体ID
	ID string `json:"id"`
	// Title 标题
	Title string `json:"title"`
	// Type 类型
	Type MediaType `json:"type"`
	// FilePath 文件路径
	FilePath string `json:"filePath"`
	// FileSize 文件大小
	FileSize int64 `json:"fileSize"`
	// Duration 时长 (秒)
	Duration int `json:"duration"`
	// Resolution 分辨率
	Resolution string `json:"resolution"`
	// Codec 编码格式
	Codec string `json:"codec"`
	// Bitrate 码率
	Bitrate int `json:"bitrate"`
	// Status 状态
	Status MediaStatus `json:"status"`
	// Metadata 元数据
	Metadata MediaMetadata `json:"metadata"`
	// Thumbnails 缩略图
	Thumbnails []string `json:"thumbnails"`
	// Subtitles 字幕
	Subtitles []Subtitle `json:"subtitles"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updatedAt"`
	// PlayedAt 播放时间
	PlayedAt time.Time `json:"playedAt,omitempty"`
	// PlayCount 播放次数
	PlayCount int `json:"playCount"`
}

// MediaMetadata 媒体元数据
type MediaMetadata struct {
	// Title 标题
	Title string `json:"title"`
	// OriginalTitle 原始标题
	OriginalTitle string `json:"originalTitle"`
	// Year 年份
	Year int `json:"year"`
	// Genre 类型/流派
	Genre []string `json:"genre"`
	// Director 导演
	Director string `json:"director"`
	// Actors 演员
	Actors []string `json:"actors"`
	// Description 描述
	Description string `json:"description"`
	// Rating 评分
	Rating float64 `json:"rating"`
	// Poster 海报
	Poster string `json:"poster"`
	// Backdrop 背景图
	Backdrop string `json:"backdrop"`
	// Studio 制片商
	Studio string `json:"studio"`
	// Country 国家
	Country string `json:"country"`
	// Language 语言
	Language string `json:"language"`
	// ReleaseDate 发行日期
	ReleaseDate string `json:"releaseDate"`
	// IMDBID IMDB ID
	IMDBID string `json:"imdbId"`
	// TMDBID TMDB ID
	TMDBID string `json:"tmdbId"`
}

// Subtitle 字幕
type Subtitle struct {
	// ID 字幕ID
	ID string `json:"id"`
	// Language 语言
	Language string `json:"language"`
	// Label 标签
	Label string `json:"label"`
	// FilePath 文件路径
	FilePath string `json:"filePath"`
	// Format 格式
	Format string `json:"format"`
	// Default 是否默认
	Default bool `json:"default"`
}

// MediaLibrary 媒体库
type MediaLibrary struct {
	// ID 库ID
	ID string `json:"id"`
	// Name 名称
	Name string `json:"name"`
	// Type 类型
	Type MediaType `json:"type"`
	// Path 路径
	Path string `json:"path"`
	// ItemCount 媒体数量
	ItemCount int `json:"itemCount"`
	// TotalSize 总大小
	TotalSize int64 `json:"totalSize"`
	// LastScanned 上次扫描时间
	LastScanned time.Time `json:"lastScanned"`
	// AutoScan 自动扫描
	AutoScan bool `json:"autoScan"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
}

// TranscodeTask 转码任务
type TranscodeTask struct {
	// ID 任务ID
	ID string `json:"id"`
	// MediaID 媒体ID
	MediaID string `json:"mediaId"`
	// SourcePath 源文件路径
	SourcePath string `json:"sourcePath"`
	// TargetPath 目标文件路径
	TargetPath string `json:"targetPath"`
	// TargetFormat 目标格式
	TargetFormat string `json:"targetFormat"`
	// TargetResolution 目标分辨率
	TargetResolution string `json:"targetResolution"`
	// TargetBitrate 目标码率
	TargetBitrate int `json:"targetBitrate"`
	// Status 状态
	Status TranscodeStatus `json:"status"`
	// Progress 进度 (0-100)
	Progress int `json:"progress"`
	// StartTime 开始时间
	StartTime time.Time `json:"startTime"`
	// EndTime 结束时间
	EndTime time.Time `json:"endTime"`
	// ErrorMessage 错误信息
	ErrorMessage string `json:"errorMessage,omitempty"`
}

// PlaySession 播放会话
type PlaySession struct {
	// ID 会话ID
	ID string `json:"id"`
	// MediaID 媒体ID
	MediaID string `json:"mediaId"`
	// UserID 用户ID
	UserID string `json:"userId"`
	// ClientIP 客户端IP
	ClientIP string `json:"clientIp"`
	// UserAgent 用户代理
	UserAgent string `json:"userAgent"`
	// StartTime 开始时间
	StartTime time.Time `json:"startTime"`
	// CurrentTime 当前播放位置
	CurrentTime int `json:"currentTime"`
	// Duration 总时长
	Duration int `json:"duration"`
	// Status 状态
	Status string `json:"status"`
}

// MediaCenter 媒体中心
type MediaCenter struct {
	mu         sync.RWMutex
	libraries  map[string]*MediaLibrary
	items      map[string]*MediaItem
	transcodes map[string]*TranscodeTask
	sessions   map[string]*PlaySession
}

// NewMediaCenter 创建媒体中心
func NewMediaCenter() *MediaCenter {
	return &MediaCenter{
		libraries:  make(map[string]*MediaLibrary),
		items:      make(map[string]*MediaItem),
		transcodes: make(map[string]*TranscodeTask),
		sessions:   make(map[string]*PlaySession),
	}
}

// AddLibrary 添加媒体库
func (mc *MediaCenter) AddLibrary(lib MediaLibrary) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.libraries[lib.ID] = &lib
	return nil
}

// RemoveLibrary 移除媒体库
func (mc *MediaCenter) RemoveLibrary(libID string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.libraries, libID)
	return nil
}

// GetLibrary 获取媒体库
func (mc *MediaCenter) GetLibrary(libID string) (*MediaLibrary, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	lib, ok := mc.libraries[libID]
	if !ok {
		return nil, fmt.Errorf("library not found: %s", libID)
	}

	return lib, nil
}

// ListLibraries 列出媒体库
func (mc *MediaCenter) ListLibraries() []*MediaLibrary {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	libs := make([]*MediaLibrary, 0, len(mc.libraries))
	for _, lib := range mc.libraries {
		libs = append(libs, lib)
	}
	return libs
}

// AddItem 添加媒体项
func (mc *MediaCenter) AddItem(item MediaItem) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items[item.ID] = &item
	return nil
}

// RemoveItem 移除媒体项
func (mc *MediaCenter) RemoveItem(itemID string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.items, itemID)
	return nil
}

// GetItem 获取媒体项
func (mc *MediaCenter) GetItem(itemID string) (*MediaItem, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	item, ok := mc.items[itemID]
	if !ok {
		return nil, fmt.Errorf("item not found: %s", itemID)
	}

	return item, nil
}

// ListItems 列出媒体项
func (mc *MediaCenter) ListItems(libID string, mediaType MediaType) []*MediaItem {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	items := make([]*MediaItem, 0)
	for _, item := range mc.items {
		if (libID == "" || item.ID == libID) &&
			(mediaType == "" || item.Type == mediaType) {
			items = append(items, item)
		}
	}
	return items
}

// SearchItems 搜索媒体项
func (mc *MediaCenter) SearchItems(query string) []*MediaItem {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	items := make([]*MediaItem, 0)
	for _, item := range mc.items {
		if contains(item.Title, query) || contains(item.Metadata.Description, query) {
			items = append(items, item)
		}
	}
	return items
}

// AddTranscodeTask 添加转码任务
func (mc *MediaCenter) AddTranscodeTask(task TranscodeTask) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.transcodes[task.ID] = &task
	return nil
}

// GetTranscodeTask 获取转码任务
func (mc *MediaCenter) GetTranscodeTask(taskID string) (*TranscodeTask, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	task, ok := mc.transcodes[taskID]
	if !ok {
		return nil, fmt.Errorf("transcode task not found: %s", taskID)
	}

	return task, nil
}

// ListTranscodeTasks 列出转码任务
func (mc *MediaCenter) ListTranscodeTasks(status TranscodeStatus) []*TranscodeTask {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	tasks := make([]*TranscodeTask, 0)
	for _, task := range mc.transcodes {
		if status == "" || task.Status == status {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// StartSession 开始播放会话
func (mc *MediaCenter) StartSession(session PlaySession) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.sessions[session.ID] = &session
	return nil
}

// EndSession 结束播放会话
func (mc *MediaCenter) EndSession(sessionID string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	delete(mc.sessions, sessionID)
	return nil
}

// GetSession 获取播放会话
func (mc *MediaCenter) GetSession(sessionID string) (*PlaySession, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	session, ok := mc.sessions[sessionID]
	if !ok {
		return nil, fmt.Errorf("session not found: %s", sessionID)
	}

	return session, nil
}

// ListSessions 列出播放会话
func (mc *MediaCenter) ListSessions(userID string) []*PlaySession {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	sessions := make([]*PlaySession, 0)
	for _, session := range mc.sessions {
		if userID == "" || session.UserID == userID {
			sessions = append(sessions, session)
		}
	}
	return sessions
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
