// Package aiocr 提供 AI 文档识别处理系统
// 参考群晖 OCR 和飞牛 AI 功能，实现智能文档 OCR 和处理
package aiocr

import (
	"time"
)

// ========== OCR 请求和结果 ==========

// OCRRequest OCR 请求.
type OCRRequest struct {
	ID          string            `json:"id"`                     // 请求唯一标识
	FileID      string            `json:"file_id"`                // 文件ID
	FilePath    string            `json:"file_path"`              // 文件路径
	Language    string            `json:"language"`               // 识别语言
	Template    string            `json:"template,omitempty"`     // 文档模板
	Options     *OCROptions       `json:"options,omitempty"`      // 识别选项
	BatchID     string            `json:"batch_id,omitempty"`     // 批量处理ID
	Priority    int               `json:"priority"`               // 优先级(1-10)
	CreatedAt   time.Time         `json:"created_at"`             // 创建时间
	Metadata    map[string]string `json:"metadata,omitempty"`     // 元数据
}

// OCROptions OCR 识别选项.
type OCROptions struct {
	DetectLanguage    bool     `json:"detect_language"`     // 自动检测语言
	EnhanceImage      bool     `json:"enhance_image"`       // 图像增强
	Deskew            bool     `json:"deskew"`              // 自动矫正
	RemoveNoise       bool     `json:"remove_noise"`        // 去噪
	Binarize          bool     `json:"binarize"`            // 二值化
	ExtractTables     bool     `json:"extract_tables"`      // 提取表格
	ExtractForms      bool     `json:"extract_forms"`       // 提取表单
	Desensitize       bool     `json:"desensitize"`         // 敏感信息脱敏
	DesensitizeFields []string `json:"desensitize_fields"`  // 需脱敏字段
	MaxPages          int      `json:"max_pages"`           // 最大页数
	DPI               int      `json:"dpi"`                 // 图像DPI
	Confidence        float64  `json:"min_confidence"`      // 最低置信度
}

// OCRResult OCR 识别结果.
type OCRResult struct {
	ID            string             `json:"id"`                       // 结果唯一标识
	RequestID     string             `json:"request_id"`               // 请求ID
	FileID        string             `json:"file_id"`                  // 文件ID
	FileName      string             `json:"file_name"`                // 文件名
	Language      string             `json:"language"`                 // 识别语言
	Pages         []*PageResult      `json:"pages"`                    // 页面结果
	FullText      string             `json:"full_text"`                // 完整文本
	Template      string             `json:"template,omitempty"`       // 识别模板
	Structured    *StructuredData    `json:"structured,omitempty"`     // 结构化数据
	Confidence    float64            `json:"confidence"`               // 整体置信度
	ProcessingMs  int64              `json:"processing_ms"`            // 处理耗时(毫秒)
	CreatedAt     time.Time          `json:"created_at"`               // 创建时间
	Desensitized  bool               `json:"desensitized"`             // 是否已脱敏
	Metadata      map[string]string  `json:"metadata,omitempty"`       // 元数据
}

// PageResult 页面识别结果.
type PageResult struct {
	PageNumber int          `json:"page_number"`         // 页码
	Width      int          `json:"width"`               // 页面宽度
	Height     int          `json:"height"`              // 页面高度
	Text       string       `json:"text"`                // 文本内容
	Blocks     []*TextBlock `json:"blocks"`              // 文本块
	Tables     []*Table     `json:"tables,omitempty"`    // 表格
	Confidence float64      `json:"confidence"`          // 页面置信度
	Rotation   float64      `json:"rotation"`            // 旋转角度
}

// TextBlock 文本块.
type TextBlock struct {
	ID         string    `json:"id"`          // 块ID
	Text       string    `json:"text"`        // 文本内容
	X          int       `json:"x"`           // X坐标
	Y          int       `json:"y"`           // Y坐标
	Width      int       `json:"width"`       // 宽度
	Height     int       `json:"height"`      // 高度
	Confidence float64   `json:"confidence"`  // 置信度
	Type       string    `json:"type"`        // 类型(text/image/table)
	FontSize   float64   `json:"font_size"`   // 字体大小
	IsBold     bool      `json:"is_bold"`     // 是否粗体
	IsItalic   bool      `json:"is_italic"`   // 是否斜体
}

// Table 表格.
type Table struct {
	ID      string     `json:"id"`       // 表格ID
	Rows    int        `json:"rows"`     // 行数
	Cols    int        `json:"cols"`     // 列数
	Cells   [][]string `json:"cells"`    // 单元格数据
	X       int        `json:"x"`        // X坐标
	Y       int        `json:"y"`        // Y坐标
	Width   int        `json:"width"`    // 宽度
	Height  int        `json:"height"`   // 高度
}

// ========== 结构化数据 ==========

// StructuredData 结构化识别数据.
type StructuredData struct {
	DocumentType string                 `json:"document_type"` // 文档类型
	Template     string                 `json:"template"`      // 模板名称
	Fields       map[string]interface{} `json:"fields"`        // 提取字段
	Confidence   float64                `json:"confidence"`    // 置信度
	IsValid      bool                   `json:"is_valid"`      // 数据是否有效
	Errors       []string               `json:"errors"`        // 校验错误
}

// DocumentTemplate 文档模板.
type DocumentTemplate struct {
	ID          string            `json:"id"`            // 模板ID
	Name        string            `json:"name"`          // 模板名称
	Type        string            `json:"type"`          // 文档类型
	Description string            `json:"description"`   // 描述
	Fields      []*TemplateField  `json:"fields"`        // 字段定义
	Patterns    []string          `json:"patterns"`      // 匹配模式
	Keywords    []string          `json:"keywords"`      // 关键词
	Enabled     bool              `json:"enabled"`       // 是否启用
	CreatedAt   time.Time         `json:"created_at"`    // 创建时间
	UpdatedAt   time.Time         `json:"updated_at"`    // 更新时间
}

// TemplateField 模板字段.
type TemplateField struct {
	Name        string `json:"name"`         // 字段名称
	Type        string `json:"type"`         // 字段类型(string/number/date/amount)
	Label       string `json:"label"`        // 显示标签
	Pattern     string `json:"pattern"`      // 匹配正则
	Required    bool   `json:"required"`     // 是否必需
	Sensitive   bool   `json:"sensitive"`    // 是否敏感
	Description string `json:"description"`  // 描述
}

// ========== 文档分类 ==========

// DocumentCategory 文档分类.
type DocumentCategory string

const (
	CategoryInvoice    DocumentCategory = "invoice"    // 发票
	CategoryContract   DocumentCategory = "contract"   // 合同
	CategoryIDCard     DocumentCategory = "id_card"    // 身份证
	CategoryBankCard   DocumentCategory = "bank_card"  // 银行卡
	CategoryReceipt    DocumentCategory = "receipt"    // 收据
	CategoryBusiness   DocumentCategory = "business"   // 营业执照
	CategoryLetter     DocumentCategory = "letter"     // 信函
	CategoryReport     DocumentCategory = "report"     // 报告
	CategoryForm       DocumentCategory = "form"       // 表单
	CategoryOther      DocumentCategory = "other"      // 其他
)

// ClassificationResult 分类结果.
type ClassificationResult struct {
	Category    DocumentCategory `json:"category"`    // 分类
	Confidence  float64          `json:"confidence"`  // 置信度
	Labels      []string         `json:"labels"`      // 标签
	Suggestions []string         `json:"suggestions"` // 建议
}

// ========== 批量处理 ==========

// BatchTask 批量处理任务.
type BatchTask struct {
	ID          string        `json:"id"`           // 任务ID
	Name        string        `json:"name"`         // 任务名称
	Status      BatchStatus   `json:"status"`       // 任务状态
	TotalFiles  int           `json:"total_files"`  // 总文件数
	Processed   int           `json:"processed"`    // 已处理数
	Failed      int           `json:"failed"`       // 失败数
	Options     *OCROptions   `json:"options"`      // 识别选项
	Results     []*OCRResult  `json:"results"`      // 识别结果
	StartedAt   *time.Time    `json:"started_at"`   // 开始时间
	CompletedAt *time.Time    `json:"completed_at"` // 完成时间
	CreatedAt   time.Time     `json:"created_at"`   // 创建时间
	Errors      []string      `json:"errors"`       // 错误信息
}

// BatchStatus 批量任务状态.
type BatchStatus string

const (
	BatchStatusPending    BatchStatus = "pending"    // 待处理
	BatchStatusProcessing BatchStatus = "processing" // 处理中
	BatchStatusCompleted  BatchStatus = "completed"  // 已完成
	BatchStatusFailed     BatchStatus = "failed"     // 失败
	BatchStatusCancelled  BatchStatus = "cancelled"  // 已取消
)

// ========== 语言支持 ==========

// Language 支持的语言.
type Language struct {
	Code      string `json:"code"`      // 语言代码
	Name      string `json:"name"`      // 语言名称
	Enabled   bool   `json:"enabled"`   // 是否启用
	Installed bool   `json:"installed"` // 是否已安装模型
}

// ========== 配置 ==========

// Config OCR 配置.
type Config struct {
	Enabled          bool              `json:"enabled"`            // 是否启用
	Engine           string            `json:"engine"`             // OCR引擎
	DefaultLanguage  string            `json:"default_language"`   // 默认语言
	Languages        []string          `json:"languages"`          // 支持语言
	MaxFileSize      int64             `json:"max_file_size"`      // 最大文件大小
	MaxPages         int               `json:"max_pages"`          // 最大页数
	Workers          int               `json:"workers"`            // 工作线程数
	QueueSize        int               `json:"queue_size"`         // 队列大小
	Templates        []*DocumentTemplate `json:"templates"`        // 文档模板
	Desensitize      *DesensitizeConfig `json:"desensitize"`       // 脱敏配置
	ArchivePath      string            `json:"archive_path"`       // 归档路径
	IndexEnabled     bool              `json:"index_enabled"`      // 索引启用
	RetentionDays    int               `json:"retention_days"`     // 保留天数
}

// DesensitizeConfig 脱敏配置.
type DesensitizeConfig struct {
	Enabled       bool     `json:"enabled"`        // 是否启用
	IDCardPattern string   `json:"id_card_pattern"` // 身份证正则
	BankCardPattern string `json:"bank_card_pattern"` // 银行卡正则
	PhonePattern  string   `json:"phone_pattern"`  // 手机号正则
	EmailPattern  string   `json:"email_pattern"`  // 邮箱正则
	LicensePattern string  `json:"license_pattern"` // 营业执照正则
	CustomPatterns []string `json:"custom_patterns"` // 自定义正则
	MaskChar      string   `json:"mask_char"`      // 脱敏字符
	KeepPrefix    int      `json:"keep_prefix"`    // 保留前缀长度
	KeepSuffix    int      `json:"keep_suffix"`    // 保留后缀长度
}

// ========== 统计信息 ==========

// Stats OCR 统计信息.
type Stats struct {
	TotalRequests    int64   `json:"total_requests"`    // 总请求数
	SuccessRequests  int64   `json:"success_requests"`  // 成功数
	FailedRequests   int64   `json:"failed_requests"`   // 失败数
	TotalPages       int64   `json:"total_pages"`       // 总页数
	AvgConfidence    float64 `json:"avg_confidence"`    // 平均置信度
	AvgProcessingMs  float64 `json:"avg_processing_ms"` // 平均处理时间
	QueueLength      int     `json:"queue_length"`      // 队列长度
	ActiveWorkers    int     `json:"active_workers"`    // 活跃工作线程
}

// ========== 归档和索引 ==========

// ArchiveEntry 归档条目.
type ArchiveEntry struct {
	ID         string    `json:"id"`          // 条目ID
	DocID      string    `json:"doc_id"`      // 文档ID
	FilePath   string    `json:"file_path"`   // 文件路径
	Category   string    `json:"category"`    // 分类
	Tags       []string  `json:"tags"`        // 标签
	Summary    string    `json:"summary"`     // 摘要
	IndexedAt  time.Time `json:"indexed_at"`  // 索引时间
	Size       int64     `json:"size"`        // 文件大小
	Checksum   string    `json:"checksum"`    // 校验和
}

// SearchQuery 搜索查询.
type SearchQuery struct {
	Keyword    string   `json:"keyword"`     // 关键词
	Category   string   `json:"category"`    // 分类
	Tags       []string `json:"tags"`        // 标签
	StartTime  *time.Time `json:"start_time"` // 开始时间
	EndTime    *time.Time `json:"end_time"`   // 结束时间
	Limit      int      `json:"limit"`       // 限制数量
	Offset     int      `json:"offset"`      // 偏移量
	SortBy     string   `json:"sort_by"`     // 排序字段
	SortOrder  string   `json:"sort_order"`  // 排序方式
}
