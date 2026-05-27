// Package filetagger 文件智能标签系统
// 基于文件扩展名、MIME类型、路径模式和内容特征的自动标签引擎
package filetagger

import (
	"regexp"
	"sync"
	"time"
)

// ========== 文件分类枚举 ==========

// FileCategory 文件分类.
type FileCategory string

const (
	CategoryDocument FileCategory = "document"
	CategoryImage    FileCategory = "image"
	CategoryVideo    FileCategory = "video"
	CategoryAudio    FileCategory = "audio"
	CategoryCode     FileCategory = "code"
	CategoryArchive  FileCategory = "archive"
	CategoryData     FileCategory = "data"
	CategoryFont     FileCategory = "font"
	CategoryOther    FileCategory = "other"
)

// ========== 标签相关类型 ==========

// Tag 标签定义.
type Tag struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Category  FileCategory `json:"category"`          // 标签所属分类
	ParentID  string       `json:"parentId,omitempty"` // 父标签ID，用于层级关系
	Color     string       `json:"color,omitempty"`
	Icon      string       `json:"icon,omitempty"`
	IsAuto    bool         `json:"isAuto"`            // 是否自动标签
	AutoRule  string       `json:"autoRule,omitempty"` // 生成该标签的规则ID
	CreatedAt time.Time    `json:"createdAt"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

// TagWithChildren 带子标签的标签.
type TagWithChildren struct {
	Tag
	Children []TagWithChildren `json:"children,omitempty"`
}

// ========== 文件标签关联 ==========

// FileTag 文件标签关联.
type FileTag struct {
	FilePath  string    `json:"filePath"`
	TagID     string    `json:"tagId"`
	TagName   string    `json:"tagName"`
	IsAuto    bool      `json:"isAuto"`   // 是否由规则自动生成
	RuleID    string    `json:"ruleId,omitempty"`
	AppliedAt time.Time `json:"appliedAt"`
}

// FileTags 文件的所有标签.
type FileTags struct {
	FilePath    string    `json:"filePath"`
	Tags        []FileTag `json:"tags"`
	AutoTags    []FileTag `json:"autoTags"`
	ManualTags  []FileTag `json:"manualTags"`
	LastScanned time.Time `json:"lastScanned"`
}

// ========== 自动标签规则 ==========

// RuleType 规则类型.
type RuleType string

const (
	RuleTypeExtension RuleType = "extension" // 按扩展名匹配
	RuleTypeMIME      RuleType = "mime"      // 按MIME类型匹配
	RuleTypePath      RuleType = "path"      // 按路径模式匹配
	RuleTypeSize      RuleType = "size"      // 按文件大小匹配
	RuleTypeContent   RuleType = "content"   // 按内容特征匹配
	RuleTypeRegex     RuleType = "regex"     // 正则表达式匹配
	RuleTypeGlob      RuleType = "glob"      // glob模式匹配
)

// Operator 比较操作符.
type Operator string

const (
	OpEquals   Operator = "eq"
	OpNotEqual Operator = "ne"
	OpGreater  Operator = "gt"
	OpLess     Operator = "lt"
	OpBetween  Operator = "between"
	OpContains Operator = "contains"
)

// AutoRule 自动标签规则.
type AutoRule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Enabled     bool      `json:"enabled"`
	Priority    int       `json:"priority"`    // 优先级，数字越大越先执行
	Type        RuleType  `json:"type"`        // 规则类型
	TagIDs      []string  `json:"tagIds"`      // 匹配后应用的标签ID列表
	Conditions  Condition `json:"conditions"`  // 匹配条件
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Condition 规则条件.
type Condition struct {
	// 扩展名匹配
	Extensions []string `json:"extensions,omitempty"` // 如 [".jpg", ".png"]

	// MIME类型匹配
	MIMETypes []string `json:"mimeTypes,omitempty"` // 如 ["image/jpeg", "image/png"]

	// 路径模式匹配 (glob)
	PathPatterns []string `json:"pathPatterns,omitempty"` // 如 ["/photos/**", "*/backup/*"]

	// 正则表达式匹配
	PathRegex   string `json:"pathRegex,omitempty"`   // 正则表达式
	NameRegex   string `json:"nameRegex,omitempty"`   // 文件名正则

	// 文件大小条件
	SizeOp     Operator `json:"sizeOp,omitempty"`     // 比较操作
	SizeValue  int64    `json:"sizeValue,omitempty"`  // 字节数
	SizeValue2 int64    `json:"sizeValue2,omitempty"` // 用于 between 操作

	// 内容特征
	ContentMagic []string `json:"contentMagic,omitempty"` // 文件魔术字节(十六进制)

	// 组合条件的逻辑关系
	And []Condition `json:"and,omitempty"` // 所有条件都满足
	Or  []Condition `json:"or,omitempty"`  // 任一条件满足
	Not *Condition  `json:"not,omitempty"` // 条件取反
}

// ========== 批量操作 ==========

// BatchOperation 批量操作类型.
type BatchOperation string

const (
	BatchOpMove    BatchOperation = "move"    // 移动
	BatchOpArchive BatchOperation = "archive" // 归档
	BatchOpDelete  BatchOperation = "delete"  // 删除
	BatchOpCopy    BatchOperation = "copy"    // 复制
)

// BatchRequest 批量操作请求.
type BatchRequest struct {
	TagIDs    []string       `json:"tagIds" binding:"required"` // 按这些标签筛选文件
	Operation BatchOperation `json:"operation" binding:"required"`
	DestPath  string         `json:"destPath,omitempty"` // move/archive/copy 的目标路径
	Confirm   bool           `json:"confirm"`            // 确认执行删除等危险操作
}

// BatchResult 批量操作结果.
type BatchResult struct {
	Operation  BatchOperation `json:"operation"`
	TotalFiles int            `json:"totalFiles"`
	Processed  int            `json:"processed"`
	Failed     int            `json:"failed"`
	Errors     []FileError    `json:"errors,omitempty"`
	Duration   string         `json:"duration"`
}

// FileError 文件操作错误.
type FileError struct {
	FilePath string `json:"filePath"`
	Error    string `json:"error"`
}

// ========== 统计相关 ==========

// TagStat 标签统计.
type TagStat struct {
	TagID     string       `json:"tagId"`
	TagName   string       `json:"tagName"`
	Category  FileCategory `json:"category"`
	FileCount int64        `json:"fileCount"`
	TotalSize int64        `json:"totalSize"` // 字节
}

// CategoryStat 分类统计.
type CategoryStat struct {
	Category  FileCategory `json:"category"`
	FileCount int64        `json:"fileCount"`
	TotalSize int64        `json:"totalSize"`
	TagCount  int64        `json:"tagCount"`
}

// OverallStats 总体统计.
type OverallStats struct {
	TotalFiles    int64          `json:"totalFiles"`
	TotalTags     int64          `json:"totalTags"`
	TotalRules    int64          `json:"totalRules"`
	TotalSize     int64          `json:"totalSize"`
	ByCategory    []CategoryStat `json:"byCategory"`
	TopTags       []TagStat      `json:"topTags"`
	LastScanTime  time.Time      `json:"lastScanTime"`
}

// ========== 搜索相关 ==========

// SearchQuery 搜索查询.
type SearchQuery struct {
	Tags       []string     `json:"tags,omitempty"`       // 标签ID列表 (AND 逻辑)
	AnyTags    []string     `json:"anyTags,omitempty"`    // 标签ID列表 (OR 逻辑)
	ExcludeTags []string    `json:"excludeTags,omitempty"` // 排除的标签
	Category   FileCategory `json:"category,omitempty"`   // 文件分类
	PathPrefix string       `json:"pathPrefix,omitempty"` // 路径前缀
	MinSize    int64        `json:"minSize,omitempty"`    // 最小文件大小
	MaxSize    int64        `json:"maxSize,omitempty"`    // 最大文件大小
	IsAuto     *bool        `json:"isAuto,omitempty"`     // 是否自动标签
	Page       int          `json:"page"`                 // 页码
	PageSize   int          `json:"pageSize"`             // 每页数量
}

// SearchResult 搜索结果.
type SearchResult struct {
	Files    []FileTags `json:"files"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"pageSize"`
}

// ========== 导入导出 ==========

// ExportData 导出数据格式.
type ExportData struct {
	Version   string     `json:"version"`
	ExportedAt time.Time `json:"exportedAt"`
	Tags      []Tag      `json:"tags"`
	Rules     []AutoRule `json:"rules"`
	FileTags  []FileTag  `json:"fileTags"`
}

// ========== 引擎配置 ==========

// Config 文件标签引擎配置.
type Config struct {
	DBPath        string `json:"dbPath"`        // SQLite 数据库路径
	ScanInterval  int    `json:"scanInterval"`  // 自动扫描间隔(秒)
	MaxFileSize   int64  `json:"maxFileSize"`   // 最大扫描文件大小(字节)
	ContentReadKB int    `json:"contentReadKB"` // 读取内容特征的字节数(KB)
	Workers       int    `json:"workers"`       // 并发扫描工作线程数
}

// DefaultConfig 默认配置.
func DefaultConfig() Config {
	return Config{
		DBPath:        "filetagger.db",
		ScanInterval:  3600,
		MaxFileSize:   1024 * 1024 * 1024, // 1GB
		ContentReadKB: 4,
		Workers:       4,
	}
}

// ========== 内部类型 ==========

// compiledRule 编译后的规则（内部使用）.
type compiledRule struct {
	rule        AutoRule
	pathRegex   *regexp.Regexp
	nameRegex   *regexp.Regexp
	pathGlobs   []globPattern
}

// globPattern 编译后的glob模式.
type globPattern struct {
	pattern string
	regex   *regexp.Regexp
}

// Engine 标签引擎核心结构.
type Engine struct {
	config       Config
	mu           sync.RWMutex
	tags         map[string]*Tag       // tagID -> Tag
	rules        map[string]*compiledRule // ruleID -> compiledRule
	fileTags     map[string][]FileTag  // filePath -> []FileTag
	tagChildren  map[string][]string   // parentID -> []childID
}

// ScanRequest 扫描请求.
type ScanRequest struct {
	Paths   []string `json:"paths" binding:"required"`   // 要扫描的路径
	Force   bool     `json:"force"`                       // 强制重新扫描
	Workers int      `json:"workers,omitempty"`           // 并发数
}

// ScanResult 扫描结果.
type ScanResult struct {
	ScannedFiles   int    `json:"scannedFiles"`
	NewTags        int    `json:"newTags"`
	UpdatedFiles   int    `json:"updatedFiles"`
	SkippedFiles   int    `json:"skippedFiles"`
	Errors         int    `json:"errors"`
	Duration       string `json:"duration"`
}
