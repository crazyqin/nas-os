// Package mediaai 提供AI驱动的媒体智能管理功能
// 实现媒体分析、分类、标签、人脸识别等核心功能
package mediaai

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== AI 媒体分析器 ==========

// MediaAnalyzer AI媒体分析器.
type MediaAnalyzer struct {
	mu         sync.RWMutex
	config     *AnalyzerConfig
	models     map[string]AIModel // AI模型
	mediaStore MediaStore         // 媒体存储
	tagStore   TagStore           // 标签存储
	faceStore  FaceStore          // 人脸存储
	ctx        context.Context
	cancel     context.CancelFunc
}

// AnalyzerConfig 分析器配置.
type AnalyzerConfig struct {
	Enabled          bool          `json:"enabled"`
	MaxConcurrent    int           `json:"maxConcurrent"`    // 最大并发分析数
	BatchSize        int           `json:"batchSize"`        // 批处理大小
	SupportedFormats []string      `json:"supportedFormats"` // 支持的格式
	EnableFaceDetect bool          `json:"enableFaceDetect"` // 启用人脸检测
	EnableOCR        bool          `json:"enableOcr"`        // 启用OCR
	EnableNSFWDetect bool          `json:"enableNsfwDetect"` // 启用NSFW检测
	MinConfidence    float64       `json:"minConfidence"`    // 最小置信度
	CacheResults     bool          `json:"cacheResults"`     // 缓存结果
	CacheTTL         time.Duration `json:"cacheTtl"`         // 缓存TTL
}

// DefaultAnalyzerConfig 默认配置.
func DefaultAnalyzerConfig() *AnalyzerConfig {
	return &AnalyzerConfig{
		Enabled:          true,
		MaxConcurrent:    4,
		BatchSize:        10,
		SupportedFormats: []string{"jpg", "jpeg", "png", "gif", "webp", "mp4", "mov", "avi"},
		EnableFaceDetect: true,
		EnableOCR:        true,
		EnableNSFWDetect: true,
		MinConfidence:    0.7,
		CacheResults:     true,
		CacheTTL:         24 * time.Hour,
	}
}

// AIModel AI模型接口.
type AIModel interface {
	Name() string
	Type() string
	Analyze(ctx context.Context, input interface{}) (interface{}, error)
}

// MediaStore 媒体存储接口.
type MediaStore interface {
	GetMedia(ctx context.Context, mediaID string) (*MediaItem, error)
	ListMedia(ctx context.Context, filter *MediaFilter) ([]*MediaItem, error)
	UpdateMedia(ctx context.Context, media *MediaItem) error
}

// TagStore 标签存储接口.
type TagStore interface {
	SaveTags(ctx context.Context, mediaID string, tags []AITag) error
	GetTags(ctx context.Context, mediaID string) ([]AITag, error)
	SearchByTag(ctx context.Context, tag string) ([]string, error)
}

// FaceStore 人脸存储接口.
type FaceStore interface {
	SaveFaces(ctx context.Context, mediaID string, faces []*FaceDetection) error
	GetFaces(ctx context.Context, mediaID string) ([]*FaceDetection, error)
	SearchPerson(ctx context.Context, personID string) ([]string, error)
}

// MediaItem 媒体项.
type MediaItem struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Path       string           `json:"path"`
	Type       string           `json:"type"` // image, video, audio
	Size       int64            `json:"size"`
	Width      int              `json:"width,omitempty"`
	Height     int              `json:"height,omitempty"`
	Duration   float64          `json:"duration,omitempty"` // 秒
	Format     string           `json:"format"`
	Checksum   string           `json:"checksum"`
	CreatedAt  time.Time        `json:"createdAt"`
	ModifiedAt time.Time        `json:"modifiedAt"`
	AnalyzedAt time.Time        `json:"analyzedAt,omitempty"`
	Tags       []AITag          `json:"tags,omitempty"`
	Faces      []*FaceDetection `json:"faces,omitempty"`
}

// MediaFilter 媒体过滤器.
type MediaFilter struct {
	Type      string    `json:"type,omitempty"`
	Format    string    `json:"format,omitempty"`
	MinSize   int64     `json:"minSize,omitempty"`
	MaxSize   int64     `json:"maxSize,omitempty"`
	StartDate time.Time `json:"startDate,omitempty"`
	EndDate   time.Time `json:"endDate,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	Analyzed  *bool     `json:"analyzed,omitempty"`
}

// FaceDetection 人脸检测结果.
type FaceDetection struct {
	ID         string    `json:"id"`
	PersonID   string    `json:"personId,omitempty"`   // 人物ID
	PersonName string    `json:"personName,omitempty"` // 人物姓名
	Confidence float64   `json:"confidence"`
	Box        BBox      `json:"box"`
	Age        int       `json:"age,omitempty"`
	Gender     string    `json:"gender,omitempty"`    // male, female
	Emotion    string    `json:"emotion,omitempty"`   // happy, sad, neutral, surprise
	Embedding  []float64 `json:"embedding,omitempty"` // 人脸特征向量
}

// NewMediaAnalyzer 创建媒体分析器.
func NewMediaAnalyzer(config *AnalyzerConfig) *MediaAnalyzer {
	if config == nil {
		config = DefaultAnalyzerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &MediaAnalyzer{
		config: config,
		models: make(map[string]AIModel),
		ctx:    ctx,
		cancel: cancel,
	}
}

// RegisterModel 注册AI模型.
func (ma *MediaAnalyzer) RegisterModel(model AIModel) {
	ma.mu.Lock()
	defer ma.mu.Unlock()

	ma.models[model.Name()] = model
	log.Printf("注册AI模型: %s (%s)", model.Name(), model.Type())
}

// AnalyzeMedia 分析媒体.
func (ma *MediaAnalyzer) AnalyzeMedia(ctx context.Context, mediaID string) (*ClassificationResult, error) {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	// 获取媒体信息
	media, err := ma.mediaStore.GetMedia(ctx, mediaID)
	if err != nil {
		return nil, fmt.Errorf("获取媒体失败: %w", err)
	}

	// 检查格式支持
	if !ma.isSupportedFormat(media.Format) {
		return nil, fmt.Errorf("不支持的格式: %s", media.Format)
	}

	// 执行分析
	result := &ClassificationResult{
		MediaID:    mediaID,
		AnalyzedAt: time.Now(),
	}

	// 场景分类
	sceneModel, ok := ma.models["scene_classifier"]
	if ok {
		sceneResult, err := sceneModel.Analyze(ctx, media)
		if err == nil {
			if scenes, ok := sceneResult.([]SceneTag); ok {
				result.Scenes = scenes
			}
		}
	}

	// 物体检测
	objectModel, ok := ma.models["object_detector"]
	if ok {
		objectResult, err := objectModel.Analyze(ctx, media)
		if err == nil {
			if objects, ok := objectResult.([]ObjectTag); ok {
				result.Objects = objects
			}
		}
	}

	// 人脸检测
	if ma.config.EnableFaceDetect {
		faceModel, ok := ma.models["face_detector"]
		if ok {
			faceResult, err := faceModel.Analyze(ctx, media)
			if err == nil {
				if faces, ok := faceResult.([]*FaceDetection); ok {
					media.Faces = faces
					ma.faceStore.SaveFaces(ctx, mediaID, faces)
				}
			}
		}
	}

	// 质量评估
	qualityModel, ok := ma.models["quality_assessor"]
	if ok {
		qualityResult, err := qualityModel.Analyze(ctx, media)
		if err == nil {
			if qr, ok := qualityResult.(*QualityAssessment); ok {
				result.Quality = qr.Rating
				result.QualityScore = qr.Score
				result.IsBlurry = qr.IsBlurry
			}
		}
	}

	// NSFW检测
	if ma.config.EnableNSFWDetect {
		nsfwModel, ok := ma.models["nsfw_detector"]
		if ok {
			nsfwResult, err := nsfwModel.Analyze(ctx, media)
			if err == nil {
				if score, ok := nsfwResult.(float64); ok {
					result.NSFWScore = score
				}
			}
		}
	}

	// 生成自动标签
	result.AutoTags = ma.generateAutoTags(result)

	// 保存标签
	ma.tagStore.SaveTags(ctx, mediaID, result.AutoTags)

	// 更新媒体分析时间
	media.AnalyzedAt = time.Now()
	media.Tags = result.AutoTags
	ma.mediaStore.UpdateMedia(ctx, media)

	return result, nil
}

// AnalyzeBatch 批量分析媒体.
func (ma *MediaAnalyzer) AnalyzeBatch(ctx context.Context, mediaIDs []string) ([]*ClassificationResult, error) {
	results := make([]*ClassificationResult, 0, len(mediaIDs))

	// 使用信号量控制并发
	sem := make(chan struct{}, ma.config.MaxConcurrent)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, mediaID := range mediaIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := ma.AnalyzeMedia(ctx, id)
			if err != nil {
				log.Printf("分析媒体 %s 失败: %v", id, err)
				return
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
		}(mediaID)
	}

	wg.Wait()
	return results, nil
}

// SearchByScene 按场景搜索.
func (ma *MediaAnalyzer) SearchByScene(ctx context.Context, scene SceneCategory) ([]*MediaItem, error) {
	// 先通过标签搜索
	mediaIDs, err := ma.tagStore.SearchByTag(ctx, string(scene))
	if err != nil {
		return nil, err
	}

	// 获取媒体详情
	mediaItems := make([]*MediaItem, 0, len(mediaIDs))
	for _, id := range mediaIDs {
		media, err := ma.mediaStore.GetMedia(ctx, id)
		if err != nil {
			continue
		}
		mediaItems = append(mediaItems, media)
	}

	return mediaItems, nil
}

// SearchByPerson 按人物搜索.
func (ma *MediaAnalyzer) SearchByPerson(ctx context.Context, personID string) ([]*MediaItem, error) {
	// 通过人脸搜索
	mediaIDs, err := ma.faceStore.SearchPerson(ctx, personID)
	if err != nil {
		return nil, err
	}

	// 获取媒体详情
	mediaItems := make([]*MediaItem, 0, len(mediaIDs))
	for _, id := range mediaIDs {
		media, err := ma.mediaStore.GetMedia(ctx, id)
		if err != nil {
			continue
		}
		mediaItems = append(mediaItems, media)
	}

	return mediaItems, nil
}

// GetSimilarMedia 获取相似媒体.
func (ma *MediaAnalyzer) GetSimilarMedia(ctx context.Context, mediaID string, limit int) ([]*MediaItem, error) {
	// 获取媒体标签
	tags, err := ma.tagStore.GetTags(ctx, mediaID)
	if err != nil {
		return nil, err
	}

	// 使用标签搜索相似内容
	mediaSet := make(map[string]bool)
	for _, tag := range tags {
		ids, _ := ma.tagStore.SearchByTag(ctx, tag.Name)
		for _, id := range ids {
			if id != mediaID {
				mediaSet[id] = true
			}
		}
	}

	// 转换为列表
	mediaItems := make([]*MediaItem, 0)
	for id := range mediaSet {
		if len(mediaItems) >= limit {
			break
		}
		media, err := ma.mediaStore.GetMedia(ctx, id)
		if err != nil {
			continue
		}
		mediaItems = append(mediaItems, media)
	}

	return mediaItems, nil
}

// isSupportedFormat 检查格式是否支持.
func (ma *MediaAnalyzer) isSupportedFormat(format string) bool {
	for _, f := range ma.config.SupportedFormats {
		if f == format {
			return true
		}
	}
	return false
}

// generateAutoTags 生成自动标签.
func (ma *MediaAnalyzer) generateAutoTags(result *ClassificationResult) []AITag {
	tags := make([]AITag, 0)

	// 场景标签
	for _, scene := range result.Scenes {
		if scene.Confidence >= ma.config.MinConfidence {
			tags = append(tags, AITag{
				Name:       string(scene.Category),
				Category:   "scene",
				Confidence: scene.Confidence,
				Source:     "ai_vision",
			})
		}
	}

	// 物体标签
	for _, object := range result.Objects {
		if object.Confidence >= ma.config.MinConfidence {
			tags = append(tags, AITag{
				Name:       object.Name,
				Category:   "object",
				Confidence: object.Confidence,
				Source:     "ai_vision",
			})
		}
	}

	// 质量标签
	if result.IsBlurry {
		tags = append(tags, AITag{
			Name:       "blurry",
			Category:   "quality",
			Confidence: 1.0,
			Source:     "ai_analysis",
		})
	}

	return tags
}

// QualityAssessment 质量评估结果.
type QualityAssessment struct {
	Rating     ContentRating `json:"rating"`
	Score      float64       `json:"score"` // 0-100
	IsBlurry   bool          `json:"isBlurry"`
	IsDark     bool          `json:"isDark"`
	IsNoisy    bool          `json:"isNoisy"`
	Sharpness  float64       `json:"sharpness"`
	Brightness float64       `json:"brightness"`
	Contrast   float64       `json:"contrast"`
}

// GetAnalysisStats 获取分析统计.
func (ma *MediaAnalyzer) GetAnalysisStats() *AnalysisStats {
	ma.mu.RLock()
	defer ma.mu.RUnlock()

	return &AnalysisStats{
		TotalAnalyzed:    0, // 需要从存储中获取
		ModelsLoaded:     len(ma.models),
		SupportedFormats: ma.config.SupportedFormats,
		LastUpdated:      time.Now(),
	}
}

// AnalysisStats 分析统计.
type AnalysisStats struct {
	TotalAnalyzed    int       `json:"totalAnalyzed"`
	ModelsLoaded     int       `json:"modelsLoaded"`
	SupportedFormats []string  `json:"supportedFormats"`
	LastUpdated      time.Time `json:"lastUpdated"`
}

// Stop 停止分析器.
func (ma *MediaAnalyzer) Stop() {
	ma.cancel()
	log.Println("AI媒体分析器已停止")
}
