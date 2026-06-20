package sharedtags

import (
	"fmt"
	"strings"
	"time"
)

// TagCategoryType represents the type of tag category
type TagCategoryType string

const (
	CategoryProject    TagCategoryType = "project"    // 项目分类
	CategoryDepartment TagCategoryType = "department" // 部门分类
	CategoryPriority   TagCategoryType = "priority"   // 优先级分类
	CategoryCustom     TagCategoryType = "custom"     // 自定义分类
)

// TagPriority represents tag priority levels
type TagPriority string

const (
	PriorityLow    TagPriority = "low"    // 低优先级
	PriorityMedium TagPriority = "medium" // 中优先级
	PriorityHigh   TagPriority = "high"   // 高优先级
	PriorityUrgent TagPriority = "urgent" // 紧急
)

// TagSharePermission represents sharing permission levels
type TagSharePermission string

const (
	ShareView   TagSharePermission = "view"   // 查看权限
	ShareUse    TagSharePermission = "use"    // 使用权限
	ShareEdit   TagSharePermission = "edit"   // 编辑权限
	ShareManage TagSharePermission = "manage" // 管理权限
)

// SearchOperator represents search combination operators
type SearchOperator string

const (
	OpAnd SearchOperator = "AND" // 与运算
	OpOr  SearchOperator = "OR"  // 或运算
	OpNot SearchOperator = "NOT" // 非运算
)

// Tag represents a single tag entity
type Tag struct {
	ID          string         `json:"id"`          // 标签唯一标识
	Name        string         `json:"name"`        // 标签名称
	Description string         `json:"description"` // 标签描述
	CategoryID  string         `json:"categoryId"`  // 所属分类ID
	Color       string         `json:"color"`       // 标签颜色
	Icon        string         `json:"icon"`        // 标签图标
	Owner       string         `json:"owner"`       // 创建者
	IsSystem    bool           `json:"isSystem"`    // 是否系统标签
	IsShared    bool           `json:"isShared"`    // 是否共享标签
	UsageCount  int64          `json:"usageCount"`  // 使用次数
	CreatedAt   time.Time      `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time      `json:"updatedAt"`   // 更新时间
	Metadata    map[string]any `json:"metadata"`    // 扩展元数据
}

// TagCategory represents a tag category with hierarchical support
type TagCategory struct {
	ID          string         `json:"id"`          // 分类唯一标识
	Name        string         `json:"name"`        // 分类名称
	Type        TagCategoryType `json:"type"`       // 分类类型
	Description string         `json:"description"` // 分类描述
	ParentID    string         `json:"parentId"`    // 父分类ID（支持多级分类）
	Level       int            `json:"level"`       // 层级深度
	SortOrder   int            `json:"sortOrder"`   // 排序顺序
	IsSystem    bool           `json:"isSystem"`    // 是否系统分类
	Owner       string         `json:"owner"`       // 创建者
	CreatedAt   time.Time      `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time      `json:"updatedAt"`   // 更新时间
}

// FileTag represents the association between a file and a tag
type FileTag struct {
	ID        string    `json:"id"`        // 关联唯一标识
	FilePath  string    `json:"filePath"`  // 文件路径
	TagID     string    `json:"tagId"`     // 标签ID
	TagName   string    `json:"tagName"`   // 标签名称（冗余字段，便于查询）
	TaggedBy  string    `json:"taggedBy"`  // 打标签的用户
	IsAuto    bool      `json:"isAuto"`    // 是否自动打标
	Confidence float64  `json:"confidence"` // 自动打标置信度
	CreatedAt time.Time `json:"createdAt"` // 关联创建时间
}

// TagShare represents a tag sharing configuration
type TagShare struct {
	ID         string             `json:"id"`         // 共享唯一标识
	TagID      string             `json:"tagId"`      // 标签ID
	TagName    string             `json:"tagName"`    // 标签名称
	SharedBy   string             `json:"sharedBy"`   // 共享发起者
	SharedWith string             `json:"sharedWith"` // 共享目标（用户或组）
	TargetType string             `json:"targetType"` // 目标类型（user/group）
	Permission TagSharePermission `json:"permission"` // 共享权限
	Notify     bool               `json:"notify"`     // 是否订阅通知
	CreatedAt  time.Time          `json:"createdAt"`  // 共享创建时间
	ExpiresAt  *time.Time         `json:"expiresAt"`  // 过期时间（可选）
}

// TagStatsResult represents tag usage statistics
type TagStatsResult struct {
	TagID       string    `json:"tagId"`       // 标签ID
	TagName     string    `json:"tagName"`     // 标签名称
	FileCount   int64     `json:"fileCount"`   // 关联文件数
	UsageCount  int64     `json:"usageCount"`  // 使用次数
	LastUsedAt  time.Time `json:"lastUsedAt"`  // 最后使用时间
	TrendScore  float64   `json:"trendScore"`  // 趋势分数
	CategoryName string   `json:"categoryName"` // 所属分类名称
}

// TagTrendPoint represents a point in tag usage trend
type TagTrendPoint struct {
	Date      time.Time `json:"date"`      // 日期
	TagID     string    `json:"tagId"`     // 标签ID
	TagName   string    `json:"tagName"`   // 标签名称
	NewFiles  int64     `json:"newFiles"`  // 新增文件数
	TotalFiles int64    `json:"totalFiles"` // 累计文件数
}

// SearchQuery represents a tag-based search query
type SearchQuery struct {
	Operator   SearchOperator `json:"operator"`   // 组合运算符
	Tags       []string       `json:"tags"`       // 标签ID列表
	CategoryID string         `json:"categoryId"` // 限定分类
	Keyword    string         `json:"keyword"`    // 关键词
	Owner      string         `json:"owner"`      // 创建者过滤
	DateFrom   *time.Time     `json:"dateFrom"`   // 起始日期
	DateTo     *time.Time     `json:"dateTo"`     // 结束日期
	Limit      int            `json:"limit"`      // 返回数量限制
	Offset     int            `json:"offset"`     // 分页偏移
}

// SearchResult represents search results
type SearchResult struct {
	Files      []FileTag `json:"files"`      // 匹配的文件标签关联
	Total      int64     `json:"total"`      // 总匹配数
	HasMore    bool      `json:"hasMore"`    // 是否有更多结果
	Query      SearchQuery `json:"query"`    // 原始查询
}

// AutoTagSuggestion represents an automatic tag suggestion
type AutoTagSuggestion struct {
	TagID      string  `json:"tagId"`      // 建议的标签ID
	TagName    string  `json:"tagName"`    // 建议的标签名称
	Confidence float64 `json:"confidence"` // 置信度
	Reason     string  `json:"reason"`     // 建议原因
}

// CreateTagRequest represents a request to create a tag
type CreateTagRequest struct {
	Name        string         `json:"name"`        // 标签名称
	Description string         `json:"description"` // 标签描述
	CategoryID  string         `json:"categoryId"`  // 所属分类ID
	Color       string         `json:"color"`       // 标签颜色
	Icon        string         `json:"icon"`        // 标签图标
	Owner       string         `json:"owner"`       // 创建者
	Metadata    map[string]any `json:"metadata"`    // 扩展元数据
}

// UpdateTagRequest represents a request to update a tag
type UpdateTagRequest struct {
	Name        *string        `json:"name"`        // 标签名称
	Description *string        `json:"description"` // 标签描述
	CategoryID  *string        `json:"categoryId"`  // 所属分类ID
	Color       *string        `json:"color"`       // 标签颜色
	Icon        *string        `json:"icon"`        // 标签图标
	Metadata    map[string]any `json:"metadata"`    // 扩展元数据
}

// CreateCategoryRequest represents a request to create a tag category
type CreateCategoryRequest struct {
	Name        string          `json:"name"`        // 分类名称
	Type        TagCategoryType `json:"type"`        // 分类类型
	Description string          `json:"description"` // 分类描述
	ParentID    string          `json:"parentId"`    // 父分类ID
	SortOrder   int             `json:"sortOrder"`   // 排序顺序
	Owner       string          `json:"owner"`       // 创建者
}

// UpdateCategoryRequest represents a request to update a tag category
type UpdateCategoryRequest struct {
	Name        *string `json:"name"`        // 分类名称
	Description *string `json:"description"` // 分类描述
	SortOrder   *int    `json:"sortOrder"`   // 排序顺序
}

// BatchTagRequest represents a batch tagging request
type BatchTagRequest struct {
	Files  []string `json:"files"`  // 文件路径列表
	Tags   []string `json:"tags"`   // 标签ID列表
	TaggedBy string `json:"taggedBy"` // 操作用户
}

// ShareTagRequest represents a request to share a tag
type ShareTagRequest struct {
	TagID      string             `json:"tagId"`      // 标签ID
	SharedWith string             `json:"sharedWith"` // 共享目标
	TargetType string             `json:"targetType"` // 目标类型
	Permission TagSharePermission `json:"permission"` // 共享权限
	Notify     bool               `json:"notify"`     // 是否订阅通知
	ExpiresAt  *time.Time         `json:"expiresAt"`  // 过期时间
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`   // 出错字段
	Message string `json:"message"` // 错误信息
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: %s - %s", e.Field, e.Message)
}

// TagError represents a tag operation error
type TagError struct {
	Code    string `json:"code"`    // 错误代码
	Message string `json:"message"` // 错误信息
}

// Error implements the error interface
func (e *TagError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Predefined errors
var (
	ErrTagNotFound = &TagError{Code: "TAG_NOT_FOUND", Message: "标签不存在"}
	ErrTagExists   = &TagError{Code: "TAG_EXISTS", Message: "标签已存在"}
	ErrCategoryNotFound = &TagError{Code: "CATEGORY_NOT_FOUND", Message: "分类不存在"}
	ErrCategoryExists   = &TagError{Code: "CATEGORY_EXISTS", Message: "分类已存在"}
	ErrInvalidInput     = &TagError{Code: "INVALID_INPUT", Message: "输入参数无效"}
	ErrPermissionDenied = &TagError{Code: "PERMISSION_DENIED", Message: "权限不足"}
	ErrShareNotFound    = &TagError{Code: "SHARE_NOT_FOUND", Message: "共享记录不存在"}
	ErrFileNotTagged    = &TagError{Code: "FILE_NOT_TAGGED", Message: "文件未打标签"}
	ErrDuplicateShare   = &TagError{Code: "DUPLICATE_SHARE", Message: "重复共享"}
	ErrSystemTag        = &TagError{Code: "SYSTEM_TAG", Message: "系统标签不可删除"}
)

// Validate validates CreateTagRequest
func (r *CreateTagRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return &ValidationError{Field: "name", Message: "标签名称不能为空"}
	}
	if len(r.Name) > 64 {
		return &ValidationError{Field: "name", Message: "标签名称不能超过64个字符"}
	}
	if strings.TrimSpace(r.Owner) == "" {
		return &ValidationError{Field: "owner", Message: "创建者不能为空"}
	}
	return nil
}

// Validate validates UpdateTagRequest
func (r *UpdateTagRequest) Validate() error {
	if r.Name != nil {
		if strings.TrimSpace(*r.Name) == "" {
			return &ValidationError{Field: "name", Message: "标签名称不能为空"}
		}
		if len(*r.Name) > 64 {
			return &ValidationError{Field: "name", Message: "标签名称不能超过64个字符"}
		}
	}
	return nil
}

// Validate validates CreateCategoryRequest
func (r *CreateCategoryRequest) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return &ValidationError{Field: "name", Message: "分类名称不能为空"}
	}
	if len(r.Name) > 64 {
		return &ValidationError{Field: "name", Message: "分类名称不能超过64个字符"}
	}
	if strings.TrimSpace(r.Owner) == "" {
		return &ValidationError{Field: "owner", Message: "创建者不能为空"}
	}
	// Validate category type
	switch r.Type {
	case CategoryProject, CategoryDepartment, CategoryPriority, CategoryCustom:
		// valid
	default:
		return &ValidationError{Field: "type", Message: "无效的分类类型"}
	}
	return nil
}

// Validate validates BatchTagRequest
func (r *BatchTagRequest) Validate() error {
	if len(r.Files) == 0 {
		return &ValidationError{Field: "files", Message: "文件列表不能为空"}
	}
	if len(r.Tags) == 0 {
		return &ValidationError{Field: "tags", Message: "标签列表不能为空"}
	}
	if strings.TrimSpace(r.TaggedBy) == "" {
		return &ValidationError{Field: "taggedBy", Message: "操作用户不能为空"}
	}
	return nil
}

// Validate validates ShareTagRequest
func (r *ShareTagRequest) Validate() error {
	if strings.TrimSpace(r.TagID) == "" {
		return &ValidationError{Field: "tagId", Message: "标签ID不能为空"}
	}
	if strings.TrimSpace(r.SharedWith) == "" {
		return &ValidationError{Field: "sharedWith", Message: "共享目标不能为空"}
	}
	if strings.TrimSpace(r.TargetType) == "" {
		return &ValidationError{Field: "targetType", Message: "目标类型不能为空"}
	}
	switch r.TargetType {
	case "user", "group":
		// valid
	default:
		return &ValidationError{Field: "targetType", Message: "目标类型只能是 user 或 group"}
	}
	// Validate permission
	switch r.Permission {
	case ShareView, ShareUse, ShareEdit, ShareManage:
		// valid
	default:
		return &ValidationError{Field: "permission", Message: "无效的共享权限"}
	}
	return nil
}

// Validate validates SearchQuery
func (q *SearchQuery) Validate() error {
	switch q.Operator {
	case OpAnd, OpOr, OpNot:
		// valid
	case "":
		q.Operator = OpAnd // default
	default:
		return &ValidationError{Field: "operator", Message: "无效的搜索运算符"}
	}
	if q.Limit <= 0 {
		q.Limit = 50 // default
	}
	if q.Limit > 1000 {
		q.Limit = 1000 // max
	}
	return nil
}
