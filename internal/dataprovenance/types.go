// Package dataprovenance 提供数据溯源追踪功能
package dataprovenance

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrRecordNotFound 溯源记录不存在.
	ErrRecordNotFound = errors.New("溯源记录不存在")
	// ErrFileNotFound 文件不存在.
	ErrFileNotFound = errors.New("文件不存在")
	// ErrInvalidInput 无效输入参数.
	ErrInvalidInput = errors.New("无效输入参数")
	// ErrIntegrityCheckFailed 完整性校验失败.
	ErrIntegrityCheckFailed = errors.New("数据完整性校验失败")
)

// ========== 操作类型 ==========

// OperationType 文件操作类型.
type OperationType string

const (
	// OpCreate 创建操作.
	OpCreate OperationType = "create"
	// OpModify 修改操作.
	OpModify OperationType = "modify"
	// OpDelete 删除操作.
	OpDelete OperationType = "delete"
	// OpMove 移动操作.
	OpMove OperationType = "move"
	// OpCopy 复制操作.
	OpCopy OperationType = "copy"
)

// ========== 数据来源 ==========

// DataSourceType 数据来源类型.
type DataSourceType string

const (
	// SourceUpload 用户上传.
	SourceUpload DataSourceType = "upload"
	// SourceDownload 网络下载.
	SourceDownload DataSourceType = "download"
	// SourceGenerate 系统生成.
	SourceGenerate DataSourceType = "generate"
	// SourceCopy 从其他文件复制.
	SourceCopy DataSourceType = "copy"
)

// ========== 核心数据结构 ==========

// ProvenanceRecord 溯源记录.
type ProvenanceRecord struct {
	// ID 记录唯一标识.
	ID string `json:"id"`
	// FileID 文件唯一标识.
	FileID string `json:"file_id"`
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// Operation 操作类型.
	Operation OperationType `json:"operation"`
	// UserID 操作用户ID.
	UserID string `json:"user_id"`
	// UserName 操作用户名.
	UserName string `json:"user_name"`
	// Timestamp 操作时间.
	Timestamp time.Time `json:"timestamp"`
	// Source 数据来源.
	Source DataSourceType `json:"source"`
	// ParentID 父文件ID（用于追踪血缘关系）.
	ParentID string `json:"parent_id,omitempty"`
	// PreviousHash 操作前文件哈希.
	PreviousHash string `json:"previous_hash,omitempty"`
	// CurrentHash 操作后文件哈希.
	CurrentHash string `json:"current_hash,omitempty"`
	// FileSize 文件大小（字节）.
	FileSize int64 `json:"file_size"`
	// Metadata 附加元数据.
	Metadata map[string]string `json:"metadata,omitempty"`
	// Description 操作描述.
	Description string `json:"description,omitempty"`
}

// FileLineage 文件血缘关系.
type FileLineage struct {
	// FileID 文件ID.
	FileID string `json:"file_id"`
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// DataType 数据类型.
	DataType string `json:"data_type,omitempty"`
	// DataClassification 数据分类标签.
	DataClassification []ComplianceTag `json:"data_classification,omitempty"`
	// Ancestors 祖先文件列表（从近到远）.
	Ancestors []LineageNode `json:"ancestors"`
	// Descendants 后代文件列表.
	Descendants []LineageNode `json:"descendants"`
}

// LineageNode 血缘关系节点.
type LineageNode struct {
	// FileID 文件ID.
	FileID string `json:"file_id"`
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// Operation 操作类型.
	Operation OperationType `json:"operation"`
	// UserID 操作用户ID.
	UserID string `json:"user_id"`
	// Timestamp 操作时间.
	Timestamp time.Time `json:"timestamp"`
}

// AuditEntry 审计条目.
type AuditEntry struct {
	// UserID 用户ID.
	UserID string `json:"user_id"`
	// UserName 用户名.
	UserName string `json:"user_name"`
	// Action 操作描述.
	Action string `json:"action"`
	// FileID 文件ID.
	FileID string `json:"file_id"`
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// Operation 操作类型.
	Operation OperationType `json:"operation"`
	// Timestamp 操作时间.
	Timestamp time.Time `json:"timestamp"`
	// Details 操作详情.
	Details string `json:"details,omitempty"`
}

// IntegrityResult 完整性校验结果.
type IntegrityResult struct {
	// FileID 文件ID.
	FileID string `json:"file_id"`
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// ExpectedHash 期望哈希.
	ExpectedHash string `json:"expected_hash"`
	// ActualHash 实际哈希.
	ActualHash string `json:"actual_hash"`
	// IsValid 是否完整.
	IsValid bool `json:"is_valid"`
	// CheckedAt 校验时间.
	CheckedAt time.Time `json:"checked_at"`
}

// ComplianceReport 合规报告.
type ComplianceReport struct {
	// ReportID 报告ID.
	ReportID string `json:"report_id"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generated_at"`
	// StartTime 统计开始时间.
	StartTime time.Time `json:"start_time"`
	// EndTime 统计结束时间.
	EndTime time.Time `json:"end_time"`
	// TotalOperations 总操作数.
	TotalOperations int64 `json:"total_operations"`
	// OperationsByType 按操作类型统计.
	OperationsByType map[OperationType]int64 `json:"operations_by_type"`
	// OperationsByUser 按用户统计.
	OperationsByUser map[string]int64 `json:"operations_by_user"`
	// TopModifiedFiles 最常修改的文件.
	TopModifiedFiles []FileOperationCount `json:"top_modified_files"`
	// IntegrityViolations 完整性违规记录.
	IntegrityViolations []IntegrityResult `json:"integrity_violations,omitempty"`
}

// FileOperationCount 文件操作计数.
type FileOperationCount struct {
	// FileID 文件ID.
	FileID string `json:"file_id"`
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// Count 操作次数.
	Count int64 `json:"count"`
}

// ComplianceTag 合规标签类型.
type ComplianceTag string

const (
	// TagGDPR GDPR合规标签.
	TagGDPR ComplianceTag = "GDPR"
	// TagHIPAA HIPAA合规标签.
	TagHIPAA ComplianceTag = "HIPAA"
	// TagSOX SOX合规标签.
	TagSOX ComplianceTag = "SOX"
	// TagPCI PCI DSS合规标签.
	TagPCI ComplianceTag = "PCI_DSS"
	// TagISO27001 ISO 27001合规标签.
	TagISO27001 ComplianceTag = "ISO_27001"
	// TagCIS CIS基准合规标签.
	TagCIS ComplianceTag = "CIS"
)

// ComplianceScope 合规范围定义.
type ComplianceScope struct {
	// Tag 合规标签.
	Tag ComplianceTag `json:"tag"`
	// Description 范围描述.
	Description string `json:"description"`
	// DataCategories 数据分类.
	DataCategories []string `json:"data_categories"`
	// RetentionDays 保留天数.
	RetentionDays int `json:"retention_days"`
	// RequiresEncryption 是否需要加密.
	RequiresEncryption bool `json:"requires_encryption"`
}

// AuditChain 审计链（区块链式结构）.
type AuditChain struct {
	// ChainID 链标识.
	ChainID string `json:"chain_id"`
	// Blocks 区块列表.
	Blocks []AuditBlock `json:"blocks"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// LastBlockHash 最后区块哈希.
	LastBlockHash string `json:"last_block_hash"`
}

// AuditBlock 审计区块.
type AuditBlock struct {
	// Index 区块索引.
	Index int64 `json:"index"`
	// Timestamp 区块时间戳.
	Timestamp time.Time `json:"timestamp"`
	// RecordID 溯源记录ID.
	RecordID string `json:"record_id"`
	// Data 区块数据.
	Data []byte `json:"data"`
	// PreviousHash 前一区块哈希.
	PreviousHash string `json:"previous_hash"`
	// Hash 当前区块哈希.
	Hash string `json:"hash"`
}

// ProvenanceQuery 高级溯源查询.
type ProvenanceQuery struct {
	// QueryID 查询ID.
	QueryID string `json:"query_id"`
	// Filter 查询过滤器.
	Filter QueryFilter `json:"filter"`
	// IncludeLineage 是否包含血缘关系.
	IncludeLineage bool `json:"include_lineage"`
	// IncludeImpact 是否包含影响分析.
	IncludeImpact bool `json:"include_impact"`
	// ComplianceFilter 合规标签过滤.
	ComplianceFilter []ComplianceTag `json:"compliance_filter,omitempty"`
}

// DataLineage 数据血缘关系（扩展版）.
type DataLineage struct {
	// DataID 数据标识.
	DataID string `json:"data_id"`
	// DataType 数据类型.
	DataType string `json:"data_type"`
	// Origin 数据来源.
	Origin *ProvenanceRecord `json:"origin"`
	// Transformations 变换历史.
	Transformations []Transformation `json:"transformations"`
	// CurrentLocation 当前位置.
	CurrentLocation string `json:"current_location"`
	// DataClassification 数据分类标签.
	DataClassification []ComplianceTag `json:"data_classification,omitempty"`
}

// Transformation 数据变换记录.
type Transformation struct {
	// RecordID 溯源记录ID.
	RecordID string `json:"record_id"`
	// Operation 操作类型.
	Operation OperationType `json:"operation"`
	// InputHash 输入数据哈希.
	InputHash string `json:"input_hash"`
	// OutputHash 输出数据哈希.
	OutputHash string `json:"output_hash"`
	// UserID 操作用户ID.
	UserID string `json:"user_id"`
	// Timestamp 变换时间.
	Timestamp time.Time `json:"timestamp"`
	// Description 变换描述.
	Description string `json:"description,omitempty"`
}

// AuditTrailExport 审计导出配置.
type AuditTrailExport struct {
	// Format 导出格式（json, csv, xml）.
	Format string `json:"format"`
	// StartTime 开始时间.
	StartTime time.Time `json:"start_time"`
	// EndTime 结束时间.
	EndTime time.Time `json:"end_time"`
	// IncludeLineage 是否包含血缘关系.
	IncludeLineage bool `json:"include_lineage"`
	// IncludeChain 是否包含审计链验证.
	IncludeChain bool `json:"include_chain"`
}

// AuditTrailResult 审计导出结果.
type AuditTrailResult struct {
	// ExportID 导出ID.
	ExportID string `json:"export_id"`
	// Format 导出格式.
	Format string `json:"format"`
	// GeneratedAt 生成时间.
	GeneratedAt time.Time `json:"generated_at"`
	// TotalRecords 记录总数.
	TotalRecords int `json:"total_records"`
	// Data 导出数据.
	Data []byte `json:"data"`
	// ChainVerified 审计链是否验证通过.
	ChainVerified bool `json:"chain_verified"`
}

// RetentionPolicy 溯源数据保留策略.
type RetentionPolicy struct {
	// MaxAge 最大保留时间.
	MaxAge time.Duration `json:"max_age"`
	// MaxRecords 最大记录数.
	MaxRecords int64 `json:"max_records"`
	// CleanedCount 已清理记录数.
	CleanedCount int64 `json:"cleaned_count"`
	// LastCleanupAt 上次清理时间.
	LastCleanupAt time.Time `json:"last_cleanup_at"`
}

// ========== 查询参数 ==========

// QueryFilter 溯源查询过滤器.
type QueryFilter struct {
	// FileID 文件ID过滤.
	FileID string `json:"file_id,omitempty"`
	// UserID 用户ID过滤.
	UserID string `json:"user_id,omitempty"`
	// Operation 操作类型过滤.
	Operation OperationType `json:"operation,omitempty"`
	// StartTime 开始时间.
	StartTime *time.Time `json:"start_time,omitempty"`
	// EndTime 结束时间.
	EndTime *time.Time `json:"end_time,omitempty"`
	// FilePath 文件路径（支持前缀匹配）.
	FilePath string `json:"file_path,omitempty"`
	// Limit 返回记录数限制.
	Limit int `json:"limit,omitempty"`
	// Offset 偏移量.
	Offset int `json:"offset,omitempty"`
}

// ImpactResult 影响分析结果.
type ImpactResult struct {
	// SourceFileID 源文件ID.
	SourceFileID string `json:"source_file_id"`
	// SourceFilePath 源文件路径.
	SourceFilePath string `json:"source_file_path"`
	// AffectedFiles 受影响的文件列表.
	AffectedFiles []AffectedFile `json:"affected_files"`
	// TotalAffected 受影响文件总数.
	TotalAffected int `json:"total_affected"`
}

// AffectedFile 受影响文件.
type AffectedFile struct {
	// FileID 文件ID.
	FileID string `json:"file_id"`
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// Relation 与源文件的关系.
	Relation string `json:"relation"`
	// AffectedAt 受影响时间.
	AffectedAt time.Time `json:"affected_at"`
}
