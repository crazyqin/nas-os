// Package aiconttag 实现 AI 内容自动标签模块
// 对标群晖 DSM 7.4 AI 搜索和飞牛 AI 相册
// 支持AI内容分析、自动标签生成、标签分类管理及批量标签
package aiconttag

import (
	"time"

	"github.com/google/uuid"
)

// ========== 核心类型定义 ==========

// TagCategory 标签分类
type TagCategory string

const (
	CategoryContent  TagCategory = "content"  // 内容类型
	CategoryObject   TagCategory = "object"   // 物体识别
	CategoryScene    TagCategory = "scene"    // 场景识别
	CategoryEmotion  TagCategory = "emotion" // 情感/风格
	CategoryTopic    TagCategory = "topic"    // 主题/话题
	CategoryQuality  TagCategory = "quality"  // 质量属性
	CategoryCustom   TagCategory = "custom"   // 自定义
)

// TagSource 标签来源
type TagSource string

const (
	SourceAI       TagSource = "ai"        // AI 自动生成
	SourceManual   TagSource = "manual"    // 人工标注
	SourceRule     TagSource = "rule"      // 规则匹配
	SourceHybrid   TagSource = "hybrid"    // 混合来源
)

// TagStatus 标签状态
type TagStatus string

const (
	TagStatusActive   TagStatus = "active"    // 活跃
	TagStatusPending  TagStatus = "pending"   // 待确认
	TagStatusRejected TagStatus = "rejected"  // 已拒绝
	TagStatusMerged   TagStatus = "merged"    // 已合并
)

// ConfidenceLevel 置信度等级
type ConfidenceLevel string

const (
	ConfidenceHigh   ConfidenceLevel = "high"   // 高置信度 (>=0.9)
	ConfidenceMedium ConfidenceLevel = "medium" // 中置信度 (>=0.7)
	ConfidenceLow    ConfidenceLevel = "low"    // 低置信度 (<0.7)
)

// ContentTag AI 内容标签
type ContentTag struct {
	// 标签唯一标识
	ID string `json:"id"`
	// 标签名称
	Name string `json:"name"`
	// 标签分类
	Category TagCategory `json:"category"`
	// 标签来源
	Source TagSource `json:"source"`
	// 置信度（0.0 ~ 1.0）
	Confidence float64 `json:"confidence"`
	// 置信度等级
	ConfidenceLevel ConfidenceLevel `json:"confidence_level"`
	// AI 模型名称
	ModelName string `json:"model_name,omitempty"`
	// 状态
	Status TagStatus `json:"status"`
	// 描述
	Description string `json:"description,omitempty"`
	// 同义词/别名
	Synonyms []string `json:"synonyms,omitempty"`
	// 颜色
	Color string `json:"color,omitempty"`
	// 创建者
	CreatedBy string `json:"created_by"`
	// 创建时间
	CreatedAt time.Time `json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
	// 使用此标签的文件数
	FileCount int `json:"file_count"`
}

// TagRule 自动标签规则
type TagRule struct {
	// 规则唯一标识
	ID string `json:"id"`
	// 规则名称
	Name string `json:"name"`
	// 触发条件关键词
	Keywords []string `json:"keywords"`
	// 触发条件正则表达式
	RegexPatterns []string `json:"regex_patterns,omitempty"`
	// 匹配后应用的标签 ID
	ApplyTagIDs []string `json:"apply_tag_ids"`
	// 最小置信度要求
	MinConfidence float64 `json:"min_confidence"`
	// 规则优先级（越高越先执行）
	Priority int `json:"priority"`
	// 是否启用
	Enabled bool `json:"enabled"`
	// 创建时间
	CreatedAt time.Time `json:"created_at"`
	// 更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// AutoTagConfig AI 自动标签配置
type AutoTagConfig struct {
	// 是否启用自动标签
	Enabled bool `json:"enabled"`
	// AI 模型名称
	ModelName string `json:"model_name"`
	// 最小置信度阈值（低于此值不自动打标签）
	MinConfidenceThreshold float64 `json:"min_confidence_threshold"`
	// 是否自动确认高置信度标签
	AutoConfirmHighConfidence bool `json:"auto_confirm_high_confidence"`
	// 自动确认阈值
	AutoConfirmThreshold float64 `json:"auto_confirm_threshold"`
	// 支持的文件类型
	SupportedFileTypes []string `json:"supported_file_types"`
	// 批量处理大小
	BatchSize int `json:"batch_size"`
	// 是否启用规则引擎
	RuleEngineEnabled bool `json:"rule_engine_enabled"`
	// 最大标签数（每个文件）
	MaxTagsPerFile int `json:"max_tags_per_file"`
	// 是否启用图片分析
	EnableImageAnalysis bool `json:"enable_image_analysis"`
	// 是否启用文本分析
	EnableTextAnalysis bool `json:"enable_text_analysis"`
	// 是否启用音频分析
	EnableAudioAnalysis bool `json:"enable_audio_analysis"`
}

// DefaultConfig 默认 AI 自动标签配置
func DefaultConfig() *AutoTagConfig {
	return &AutoTagConfig{
		Enabled:                   true,
		ModelName:                 " nas-os-ai-tag-v1",
		MinConfidenceThreshold:    0.7,
		AutoConfirmHighConfidence: true,
		AutoConfirmThreshold:      0.9,
		SupportedFileTypes:        []string{"jpg", "jpeg", "png", "gif", "bmp", "txt", "pdf", "md", "doc", "docx", "mp3", "wav", "mp4", "mov"},
		BatchSize:                 10,
		RuleEngineEnabled:         true,
		MaxTagsPerFile:            20,
		EnableImageAnalysis:        true,
		EnableTextAnalysis:        true,
		EnableAudioAnalysis:        false,
	}
}

// FileContentTag 文件内容标签关联
type FileContentTag struct {
	// 文件路径
	FilePath string `json:"file_path"`
	// 文件 ID
	FileID string `json:"file_id,omitempty"`
	// 标签 ID
	TagID string `json:"tag_id"`
	// 标签名称
	TagName string `json:"tag_name"`
	// 标签分类
	TagCategory TagCategory `json:"tag_category"`
	// 置信度
	Confidence float64 `json:"confidence"`
	// 来源
	Source TagSource `json:"source"`
	// 分析时间
	AnalyzedAt time.Time `json:"analyzed_at"`
	// AI 模型
	ModelName string `json:"model_name,omitempty"`
	// 是否已确认
	Confirmed bool `json:"confirmed"`
}

// ContentAnalysisResult AI 内容分析结果
type ContentAnalysisResult struct {
	// 文件路径
	FilePath string `json:"file_path"`
	// 文件类型
	FileType string `json:"file_type"`
	// 检测到的标签列表
	Tags []TagDetection `json:"tags"`
	// 内容描述
	Description string `json:"description,omitempty"`
	// 分析耗时（毫秒）
	AnalysisTimeMs int64 `json:"analysis_time_ms"`
	// 模型名称
	ModelName string `json:"model_name"`
	// 分析时间
	AnalyzedAt time.Time `json:"analyzed_at"`
}

// TagDetection 标签检测结果
type TagDetection struct {
	// 标签名称
	Name string `json:"name"`
	// 分类
	Category TagCategory `json:"category"`
	// 置信度
	Confidence float64 `json:"confidence"`
	// 别名/同义词匹配
	MatchedSynonym string `json:"matched_synonym,omitempty"`
}

// TagStats 标签统计
type TagStats struct {
	TotalTags        int            `json:"total_tags"`
	TagsByCategory   map[string]int `json:"tags_by_category"`
	TagsBySource     map[string]int `json:"tags_by_source"`
	TagsByConfidence map[string]int `json:"tags_by_confidence"`
	AutoConfirmed    int            `json:"auto_confirmed"`
	PendingReview    int            `json:"pending_review"`
	TotalFiles       int            `json:"total_files"`
	RuleCount        int            `json:"rule_count"`
	EnabledRules      int            `json:"enabled_rules"`
}

// ========== 请求/响应结构 ==========

// CreateTagRequest 创建标签请求
type CreateTagRequest struct {
	Name        string      `json:"name" binding:"required"`
	Category    TagCategory `json:"category" binding:"required"`
	Description string      `json:"description"`
	Synonyms    []string    `json:"synonyms"`
	Color       string      `json:"color"`
	CreatedBy   string      `json:"created_by" binding:"required"`
}

// CreateRuleRequest 创建规则请求
type CreateRuleRequest struct {
	Name          string   `json:"name" binding:"required"`
	Keywords      []string `json:"keywords" binding:"required"`
	RegexPatterns []string `json:"regex_patterns"`
	ApplyTagIDs   []string `json:"apply_tag_ids" binding:"required"`
	MinConfidence float64  `json:"min_confidence"`
	Priority      int      `json:"priority"`
	Enabled       bool     `json:"enabled"`
}

// AnalyzeRequest AI 分析请求
type AnalyzeRequest struct {
	FilePath string `json:"file_path" binding:"required"`
	FileType string `json:"file_type"`
}

// BatchAnalyzeRequest 批量分析请求
type BatchAnalyzeRequest struct {
	FilePaths []string `json:"file_paths" binding:"required"`
}

// BatchAnalyzeResponse 批量分析响应
type BatchAnalyzeResponse struct {
	Results []ContentAnalysisResult `json:"results"`
	Total   int                     `json:"total"`
	Success int                     `json:"success"`
	Failed  int                     `json:"failed"`
}

// SearchByTagRequest 按标签搜索请求
type SearchByTagRequest struct {
	TagIDs    []string    `json:"tag_ids"`
	TagNames  []string    `json:"tag_names"`
	Category  TagCategory `json:"category"`
	MinConfidence float64 `json:"min_confidence"`
}

// TagSearchResult 标签搜索结果
type TagSearchResult struct {
	Files    []FileContentTag `json:"files"`
	Total    int               `json:"total"`
}

// ConfirmTagRequest 确认标签请求
type ConfirmTagRequest struct {
	FilePath string `json:"file_path" binding:"required"`
	TagID    string `json:"tag_id" binding:"required"`
}

// ManualTagRequest 手动打标签请求
type ManualTagRequest struct {
	FilePath  string   `json:"file_path" binding:"required"`
	TagNames  []string `json:"tag_names" binding:"required"`
	TaggedBy  string   `json:"tagged_by" binding:"required"`
}

// ========== 内部辅助函数 ==========

// newContentTag 创建 ContentTag 实例
func newContentTag(req *CreateTagRequest) *ContentTag {
	now := time.Now()
	tag := &ContentTag{
		ID:           "tag_" + uuid.New().String()[:12],
		Name:         req.Name,
		Category:     req.Category,
		Source:       SourceManual,
		Confidence:   1.0,
		ConfidenceLevel: ConfidenceHigh,
		Status:       TagStatusActive,
		Description:  req.Description,
		Synonyms:     req.Synonyms,
		Color:        req.Color,
		CreatedBy:    req.CreatedBy,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return tag
}

// newTagRule 创建 TagRule 实例
func newTagRule(req *CreateRuleRequest) *TagRule {
	now := time.Now()
	return &TagRule{
		ID:            "rule_" + uuid.New().String()[:12],
		Name:          req.Name,
		Keywords:      req.Keywords,
		RegexPatterns: req.RegexPatterns,
		ApplyTagIDs:   req.ApplyTagIDs,
		MinConfidence: req.MinConfidence,
		Priority:      req.Priority,
		Enabled:       req.Enabled,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// confidenceLevel 计算置信度等级
func confidenceLevel(conf float64) ConfidenceLevel {
	if conf >= 0.9 {
		return ConfidenceHigh
	}
	if conf >= 0.7 {
		return ConfidenceMedium
	}
	return ConfidenceLow
}

// shouldAutoConfirm 判断是否应自动确认标签
func shouldAutoConfirm(conf float64, cfg *AutoTagConfig) bool {
	return cfg.AutoConfirmHighConfidence && conf >= cfg.AutoConfirmThreshold
}

// meetsThreshold 判断置信度是否满足阈值
func meetsThreshold(conf float64, cfg *AutoTagConfig) bool {
	return conf >= cfg.MinConfidenceThreshold
}

// isSupportedFileType 判断文件类型是否支持分析
func isSupportedFileType(fileType string, cfg *AutoTagConfig) bool {
	if len(cfg.SupportedFileTypes) == 0 {
		return true // 未配置则全部支持
	}
	for _, ft := range cfg.SupportedFileTypes {
		if ft == fileType {
			return true
		}
	}
	return false
}