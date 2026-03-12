package ai_classify

import (
	"time"
)

// Category 文件分类
type Category struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Keywords    []string  `json:"keywords"`
	Extensions  []string  `json:"extensions"`
	Patterns    []string  `json:"patterns"` // 正则模式
	Color       string    `json:"color"`    // UI 显示颜色
	Icon        string    `json:"icon"`     // 图标
	ParentID    string    `json:"parentId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Tag 标签
type Tag struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	Category  string    `json:"category,omitempty"` // 所属分类
	Count     int       `json:"count"`              // 使用次数
	CreatedAt time.Time `json:"createdAt"`
}

// FileClassification 文件分类结果
type FileClassification struct {
	Path        string    `json:"path"`
	FileName    string    `json:"fileName"`
	Extension   string    `json:"extension"`
	Category    Category  `json:"category"`
	Tags        []Tag     `json:"tags"`
	Confidence  float64   `json:"confidence"` // 分类置信度 (0-1)
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"modTime"`
	ContentHash string    `json:"contentHash,omitempty"` // 内容哈希
	Features    Features  `json:"features,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Features 文件特征
type Features struct {
	// 文本特征
	WordCount int      `json:"wordCount,omitempty"`
	Language  string   `json:"language,omitempty"`
	Keywords  []string `json:"keywords,omitempty"`
	Entities  []string `json:"entities,omitempty"` // 命名实体

	// 图像特征
	Width          int      `json:"width,omitempty"`
	Height         int      `json:"height,omitempty"`
	ColorDepth     int      `json:"colorDepth,omitempty"`
	DominantColors []string `json:"dominantColors,omitempty"`

	// 视频/音频特征
	Duration      int     `json:"duration,omitempty"` // 秒
	BitRate       int     `json:"bitRate,omitempty"`
	FrameRate     float64 `json:"frameRate,omitempty"`
	AudioChannels int     `json:"audioChannels,omitempty"`

	// 代码特征
	LineCount     int    `json:"lineCount,omitempty"`
	FunctionCount int    `json:"functionCount,omitempty"`
	ImportCount   int    `json:"importCount,omitempty"`
	LanguageVer   string `json:"languageVer,omitempty"`

	// 文档特征
	PageCount int    `json:"pageCount,omitempty"`
	Title     string `json:"title,omitempty"`
	Author    string `json:"author,omitempty"`
}

// Similarity 相似度结果
type Similarity struct {
	FileA      string    `json:"fileA"`
	FileB      string    `json:"fileB"`
	Score      float64   `json:"score"`   // 相似度分数 (0-1)
	SimType    SimType   `json:"simType"` // 相似类型
	Reason     string    `json:"reason"`  // 相似原因
	DetectedAt time.Time `json:"detectedAt"`
}

// SimType 相似类型
type SimType string

const (
	SimTypeContent  SimType = "content"  // 内容相似
	SimTypeName     SimType = "name"     // 文件名相似
	SimTypeMetadata SimType = "metadata" // 元数据相似
	SimTypeHash     SimType = "hash"     // 哈希相同（完全相同）
	SimTypeSemantic SimType = "semantic" // 语义相似
)

// ClassificationRule 分类规则
type ClassificationRule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Priority    int         `json:"priority"` // 规则优先级
	Enabled     bool        `json:"enabled"`
	Conditions  []Condition `json:"conditions"`
	Actions     []Action    `json:"actions"`
	HitCount    int         `json:"hitCount"` // 命中次数
	LastHit     time.Time   `json:"lastHit"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

// Condition 条件
type Condition struct {
	Type     ConditionType `json:"type"`
	Field    string        `json:"field"`
	Operator string        `json:"operator"` // eq, ne, contains, matches, gt, lt, in
	Value    interface{}   `json:"value"`
}

// ConditionType 条件类型
type ConditionType string

const (
	CondTypeFileName  ConditionType = "filename"
	CondTypeExtension ConditionType = "extension"
	CondTypePath      ConditionType = "path"
	CondTypeSize      ConditionType = "size"
	CondTypeContent   ConditionType = "content"
	CondTypeMetadata  ConditionType = "metadata"
	CondTypeTime      ConditionType = "time"
	CondTypeMIME      ConditionType = "mime"
)

// Action 动作
type Action struct {
	Type       ActionType `json:"type"`
	CategoryID string     `json:"categoryId,omitempty"`
	TagNames   []string   `json:"tagNames,omitempty"`
	MoveTo     string     `json:"moveTo,omitempty"`
	Rename     string     `json:"rename,omitempty"`
}

// ActionType 动作类型
type ActionType string

const (
	ActionClassify ActionType = "classify"
	ActionTag      ActionType = "tag"
	ActionMove     ActionType = "move"
	ActionRename   ActionType = "rename"
	ActionNotify   ActionType = "notify"
)

// LearningData 学习数据
type LearningData struct {
	ID           string    `json:"id"`
	FilePath     string    `json:"filePath"`
	Features     Features  `json:"features"`
	CategoryID   string    `json:"categoryId"`
	TagIDs       []string  `json:"tagIds"`
	UserAction   string    `json:"userAction"` // 用户修正动作
	Corrected    bool      `json:"corrected"`  // 是否被用户修正
	Feedback     string    `json:"feedback"`   // 用户反馈
	ModelVersion string    `json:"modelVersion"`
	CreatedAt    time.Time `json:"createdAt"`
}

// Config 分类器配置
type Config struct {
	// 基础配置
	DataDir        string `json:"dataDir"`        // 数据存储目录
	ModelPath      string `json:"modelPath"`      // 模型路径
	EnableLearning bool   `json:"enableLearning"` // 启用学习

	// 分类配置
	ConfidenceThreshold float64 `json:"confidenceThreshold"` // 置信度阈值
	MaxTags             int     `json:"maxTags"`             // 最大标签数

	// 相似度配置
	SimilarityThreshold float64 `json:"similarityThreshold"` // 相似度阈值
	MaxSimilarFiles     int     `json:"maxSimilarFiles"`     // 最大相似文件数

	// 特征提取配置
	MaxContentSize  int64 `json:"maxContentSize"`  // 最大内容读取大小
	EnableHashCache bool  `json:"enableHashCache"` // 启用哈希缓存
	EnableImageFeat bool  `json:"enableImageFeat"` // 提取图像特征
	EnableTextFeat  bool  `json:"enableTextFeat"`  // 提取文本特征
	EnableVideoFeat bool  `json:"enableVideoFeat"` // 提取视频特征
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		DataDir:             "/var/lib/nas-os/ai_classify",
		ModelPath:           "/usr/share/nas-os/models",
		EnableLearning:      true,
		ConfidenceThreshold: 0.7,
		MaxTags:             10,
		SimilarityThreshold: 0.8,
		MaxSimilarFiles:     50,
		MaxContentSize:      10 * 1024 * 1024, // 10MB
		EnableHashCache:     true,
		EnableImageFeat:     true,
		EnableTextFeat:      true,
		EnableVideoFeat:     false, // 视频特征提取较慢
	}
}
