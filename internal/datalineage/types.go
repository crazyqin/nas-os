// Package datalineage 提供数据血缘追踪功能
// 对标群晖 File Station + TrueNAS Data Services
// 功能：文件来源追踪、修改历史、数据流向、依赖图谱、PII检测、生命周期报告
package datalineage

import (
	"time"
)

// ============================================================================
// 基础枚举类型
// ============================================================================

// EventType 文件事件类型
type EventType string

const (
	// EventCreate 文件创建
	EventCreate EventType = "create"
	// EventModify 文件修改
	EventModify EventType = "modify"
	// EventDelete 文件删除
	EventDelete EventType = "delete"
	// EventMove 文件移动
	EventMove EventType = "move"
	// EventCopy 文件复制
	EventCopy EventType = "copy"
	// EventRename 文件重命名
	EventRename EventType = "rename"
	// EventSync 文件同步
	EventSync EventType = "sync"
	// EventShare 文件分享
	EventShare EventType = "share"
	// EventRestore 文件恢复
	EventRestore EventType = "restore"
)

// OriginSource 文件来源类型
type OriginSource string

const (
	// OriginLocal 本地创建
	OriginLocal OriginSource = "local"
	// OriginUpload 用户上传
	OriginUpload OriginSource = "upload"
	// OriginSync 同步而来
	OriginSync OriginSource = "sync"
	// OriginDownload 下载而来
	OriginDownload OriginSource = "download"
	// OriginApp 应用生成
	OriginApp OriginSource = "app"
	// OriginBackup 备份恢复
	OriginBackup OriginSource = "backup"
	// OriginMigration 迁移而来
	OriginMigration OriginSource = "migration"
)

// DataClassification 数据分类等级
type DataClassification string

const (
	// ClassPublic 公开数据
	ClassPublic DataClassification = "public"
	// ClassInternal 内部数据
	ClassInternal DataClassification = "internal"
	// ClassConfidential 机密数据
	ClassConfidential DataClassification = "confidential"
	// ClassRestricted 受限数据
	ClassRestricted DataClassification = "restricted"
	// ClassTopSecret 绝密数据
	ClassTopSecret DataClassification = "top_secret"
)

// PIIType 个人可识别信息类型
type PIIType string

const (
	// PIIEmail 邮箱地址
	PIIEmail PIIType = "email"
	// PIIPhone 电话号码
	PIIPhone PIIType = "phone"
	// PIISocialSecurity 社会保障号
	PIISocialSecurity PIIType = "ssn"
	// PIICreditCard 信用卡号
	PIICreditCard PIIType = "credit_card"
	// PIIAddress 地址信息
	PIIAddress PIIType = "address"
	// PIIIDCard 身份证号
	PIIIDCard PIIType = "id_card"
	// PIIName 姓名
	PIIName PIIType = "name"
)

// DependencyType 依赖关系类型
type DependencyType string

const (
	// DepDerivedFrom 派生自（A由B生成）
	DepDerivedFrom DependencyType = "derived_from"
	// DepContains 包含（A包含B）
	DepContains DependencyType = "contains"
	// DepReferencedBy 被引用（A被B引用）
	DepReferencedBy DependencyType = "referenced_by"
	// DepLinkedTo 关联（A和B关联）
	DepLinkedTo DependencyType = "linked_to"
	// DepSyncedWith 同步（A与B同步）
	DepSyncedWith DependencyType = "synced_with"
)

// SyncStatus 同步状态
type SyncStatus string

const (
	// SyncPending 同步中
	SyncPending SyncStatus = "pending"
	// SyncCompleted 已同步
	SyncCompleted SyncStatus = "completed"
	// SyncFailed 同步失败
	SyncFailed SyncStatus = "failed"
	// SyncConflict 冲突
	SyncConflict SyncStatus = "conflict"
)

// LifecycleStage 生命周期阶段
type LifecycleStage string

const (
	// StageActive 活跃使用中
	StageActive LifecycleStage = "active"
	// StageArchive 已归档
	StageArchive LifecycleStage = "archived"
	// StageDeprecated 已废弃
	StageDeprecated LifecycleStage = "deprecated"
	// StageDeleted 已删除
	StageDeleted LifecycleStage = "deleted"
)

// ============================================================================
// 核心数据结构
// ============================================================================

// FileLineage 文件血缘记录，记录文件完整的来源和变更历史
type FileLineage struct {
	// ID 血缘记录唯一标识
	ID string `json:"id"`
	// FileID 关联的文件ID
	FileID string `json:"file_id"`
	// FilePath 文件路径
	FilePath string `json:"file_path"`
	// FileName 文件名
	FileName string `json:"file_name"`
	// FileSize 文件大小（字节）
	FileSize int64 `json:"file_size"`
	// MimeType MIME类型
	MimeType string `json:"mime_type"`
	// Checksum 文件校验和
	Checksum string `json:"checksum"`
	// Origin 来源信息
	Origin *FileOrigin `json:"origin"`
	// History 修改历史
	History []*LineageEvent `json:"history"`
	// Tags 分类标签
	Tags []string `json:"tags"`
	// Classification 数据分类
	Classification DataClassification `json:"classification"`
	// PIIDetected 检测到的PII
	PIIDetected []PIIDetection `json:"pii_detected"`
	// Lifecycle 生命周期信息
	Lifecycle *LifecycleInfo `json:"lifecycle"`
	// Devices 关联设备列表
	Devices []string `json:"devices"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 最后更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// FileOrigin 文件来源信息
type FileOrigin struct {
	// Source 来源类型
	Source OriginSource `json:"source"`
	// DeviceID 来源设备ID
	DeviceID string `json:"device_id"`
	// DeviceName 来源设备名称
	DeviceName string `json:"device_name"`
	// UserID 创建者用户ID
	UserID string `json:"user_id"`
	// UserName 创建者用户名
	UserName string `json:"user_name"`
	// Application 创建应用名称
	Application string `json:"application"`
	// OriginalPath 原始路径
	OriginalPath string `json:"original_path"`
	// OriginalName 原始文件名
	OriginalName string `json:"original_name"`
	// ExternalRef 外部引用（URL/共享链接等）
	ExternalRef string `json:"external_ref,omitempty"`
	// CreatedAt 来源记录时间
	CreatedAt time.Time `json:"created_at"`
}

// LineageEvent 血缘事件，记录文件每一次变更
type LineageEvent struct {
	// ID 事件唯一标识
	ID string `json:"id"`
	// Type 事件类型
	Type EventType `json:"type"`
	// Timestamp 事件时间戳
	Timestamp time.Time `json:"timestamp"`
	// DeviceID 操作设备ID
	DeviceID string `json:"device_id"`
	// DeviceName 操作设备名称
	DeviceName string `json:"device_name"`
	// UserID 操作用户ID
	UserID string `json:"user_id"`
	// UserName 操作用户名
	UserName string `json:"userName"`
	// Path 操作后的文件路径
	Path string `json:"path"`
	// OldPath 操作前的文件路径（重命名/移动时使用）
	OldPath string `json:"old_path,omitempty"`
	// Checksum 变更后的校验和
	Checksum string `json:"checksum,omitempty"`
	// SizeDelta 大小变更量（字节，正为增长）
	SizeDelta int64 `json:"size_delta"`
	// Description 事件描述
	Description string `json:"description"`
	// Metadata 附加元数据
	Metadata map[string]string `json:"metadata,omitempty"`
}

// PIIDetection PII检测结果
type PIIDetection struct {
	// Type PII类型
	Type PIIType `json:"type"`
	// Value 脱敏后的值
	Value string `json:"value"`
	// Confidence 置信度（0.0~1.0）
	Confidence float64 `json:"confidence"`
	// Location 发现位置描述
	Location string `json:"location"`
}

// LifecycleInfo 生命周期信息
type LifecycleInfo struct {
	// Stage 当前阶段
	Stage LifecycleStage `json:"stage"`
	// CreatedAt 文件创建时间
	CreatedAt time.Time `json:"created_at"`
	// LastAccessedAt 最后访问时间
	LastAccessedAt time.Time `json:"last_accessed_at"`
	// LastModifiedAt 最后修改时间
	LastModifiedAt time.Time `json:"last_modified_at"`
	// ExpiresAt 过期时间（可选）
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	// ArchivedAt 归档时间（可选）
	ArchivedAt *time.Time `json:"archived_at,omitempty"`
	// RetentionDays 保留天数
	RetentionDays int `json:"retention_days"`
	// AccessCount 累计访问次数
	AccessCount int64 `json:"access_count"`
}

// ============================================================================
// 依赖关系图谱
// ============================================================================

// Dependency 依赖关系
type Dependency struct {
	// ID 依赖关系唯一标识
	ID string `json:"id"`
	// SourceID 源文件ID
	SourceID string `json:"source_id"`
	// SourcePath 源文件路径
	SourcePath string `json:"source_path"`
	// TargetID 目标文件ID
	TargetID string `json:"target_id"`
	// TargetPath 目标文件路径
	TargetPath string `json:"target_path"`
	// Type 依赖类型
	Type DependencyType `json:"type"`
	// Description 关系描述
	Description string `json:"description"`
	// Strength 关系强度（0.0~1.0）
	Strength float64 `json:"strength"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
}

// DependencyGraph 依赖关系图谱
type DependencyGraph struct {
	// CenterID 中心文件ID
	CenterID string `json:"center_id"`
	// CenterPath 中心文件路径
	CenterPath string `json:"center_path"`
	// Nodes 图节点列表
	Nodes []*GraphNode `json:"nodes"`
	// Edges 图边列表（依赖关系）
	Edges []*GraphEdge `json:"edges"`
	// Depth 查询深度
	Depth int `json:"depth"`
	// GeneratedAt 生成时间
	GeneratedAt time.Time `json:"generated_at"`
}

// GraphNode 图节点
type GraphNode struct {
	// ID 文件ID
	ID string `json:"id"`
	// Path 文件路径
	Path string `json:"path"`
	// Name 文件名
	Name string `json:"name"`
	// Classification 数据分类
	Classification DataClassification `json:"classification"`
	// IsCenter 是否为中心节点
	IsCenter bool `json:"is_center"`
	// Level 距中心的层级
	Level int `json:"level"`
}

// GraphEdge 图边
type GraphEdge struct {
	// SourceID 源节点ID
	SourceID string `json:"source_id"`
	// TargetID 目标节点ID
	TargetID string `json:"target_id"`
	// Type 关系类型
	Type DependencyType `json:"type"`
	// Strength 关系强度
	Strength float64 `json:"strength"`
	// Label 边标签
	Label string `json:"label"`
}

// ============================================================================
// 数据流向
// ============================================================================

// DataFlow 数据流向记录
type DataFlow struct {
	// FileID 文件ID
	FileID string `json:"file_id"`
	// FilePath 文件路径
	FilePath string `json:"file_path"`
	// Stages 流向各阶段
	Stages []*FlowStage `json:"stages"`
	// CurrentLocation 当前位置
	CurrentLocation string `json:"current_location"`
	// TotalDistance 数据迁移总距离（设备/路径变更次数）
	TotalDistance int `json:"total_distance"`
}

// FlowStage 数据流向阶段
type FlowStage struct {
	// StageNumber 阶段编号（从1开始）
	StageNumber int `json:"stage_number"`
	// EventType 事件类型
	EventType EventType `json:"event_type"`
	// DeviceID 设备ID
	DeviceID string `json:"device_id"`
	// DeviceName 设备名称
	DeviceName string `json:"device_name"`
	// Path 文件路径
	Path string `json:"path"`
	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`
	// UserID 操作者ID
	UserID string `json:"user_id"`
	// Description 描述
	Description string `json:"description"`
}

// ============================================================================
// 跨设备同步追踪
// ============================================================================

// SyncRecord 同步记录
type SyncRecord struct {
	// ID 同步记录ID
	ID string `json:"id"`
	// FileID 文件ID
	FileID string `json:"file_id"`
	// FilePath 文件路径
	FilePath string `json:"file_path"`
	// SourceDeviceID 源设备ID
	SourceDeviceID string `json:"source_device_id"`
	// SourceDeviceName 源设备名称
	SourceDeviceName string `json:"source_device_name"`
	// TargetDeviceID 目标设备ID
	TargetDeviceID string `json:"target_device_id"`
	// TargetDeviceName 目标设备名称
	TargetDeviceName string `json:"target_device_name"`
	// Status 同步状态
	Status SyncStatus `json:"status"`
	// Direction 同步方向
	Direction string `json:"direction"` // push / pull / bidirectional
	// Size 同步数据量（字节）
	Size int64 `json:"size"`
	// Duration 同步耗时（毫秒）
	Duration int64 `json:"duration"`
	// ErrorMessage 失败时的错误信息
	ErrorMessage string `json:"error_message,omitempty"`
	// StartedAt 开始时间
	StartedAt time.Time `json:"started_at"`
	// CompletedAt 完成时间
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ============================================================================
// 数据生命周期报告
// ============================================================================

// LifecycleReport 数据生命周期报告
type LifecycleReport struct {
	// ReportID 报告ID
	ReportID string `json:"report_id"`
	// GeneratedAt 生成时间
	GeneratedAt time.Time `json:"generated_at"`
	// Period 报告周期
	Period *ReportPeriod `json:"period"`
	// Summary 概要统计
	Summary *ReportSummary `json:"summary"`
	// ByClassification 按分类统计
	ByClassification map[DataClassification]int `json:"by_classification"`
	// ByStage 按生命周期阶段统计
	ByStage map[LifecycleStage]int `json:"by_stage"`
	// BySource 按来源统计
	BySource map[OriginSource]int `json:"by_source"`
	// TopModifiedFiles 修改最频繁的文件
	TopModifiedFiles []*FileLineage `json:"top_modified_files"`
	// TopAccessedFiles 访问最多的文件
	TopAccessedFiles []*FileLineage `json:"top_accessed_files"`
	// PIISummary PII检测汇总
	PIISummary *PIISummary `json:"pii_summary"`
	// SyncSummary 同步统计
	SyncSummary *SyncSummary `json:"sync_summary"`
}

// ReportPeriod 报告周期
type ReportPeriod struct {
	// Start 开始时间
	Start time.Time `json:"start"`
	// End 结束时间
	End time.Time `json:"end"`
	// Label 周期标签（如 "2024-Q1"）
	Label string `json:"label"`
}

// ReportSummary 报告概要
type ReportSummary struct {
	// TotalFiles 追踪的文件总数
	TotalFiles int `json:"total_files"`
	// TotalEvents 总事件数
	TotalEvents int `json:"total_events"`
	// NewFiles 新增文件数
	NewFiles int `json:"new_files"`
	// DeletedFiles 删除文件数
	DeletedFiles int `json:"deleted_files"`
	// ModifiedFiles 修改文件数
	ModifiedFiles int `json:"modified_files"`
	// MovedFiles 移动文件数
	MovedFiles int `json:"moved_files"`
	// TotalSizeBytes 总文件大小（字节）
	TotalSizeBytes int64 `json:"total_size_bytes"`
	// AvgEventsPerFile 每文件平均事件数
	AvgEventsPerFile float64 `json:"avg_events_per_file"`
	// UniqueDevices 涉及的独立设备数
	UniqueDevices int `json:"unique_devices"`
	// UniqueUsers 涉及的独立用户数
	UniqueUsers int `json:"unique_users"`
}

// PIISummary PII检测汇总
type PIISummary struct {
	// TotalFiles 含PII的文件数
	TotalFiles int `json:"total_files"`
	// ByType 按PII类型统计
	ByType map[PIIType]int `json:"by_type"`
	// HighRiskFiles 高风险文件列表
	HighRiskFiles []string `json:"high_risk_files"`
}

// SyncSummary 同步统计汇总
type SyncSummary struct {
	// TotalSyncs 总同步次数
	TotalSyncs int `json:"total_syncs"`
	// SuccessCount 成功次数
	SuccessCount int `json:"success_count"`
	// FailureCount 失败次数
	FailureCount int `json:"failure_count"`
	// ConflictCount 冲突次数
	ConflictCount int `json:"conflict_count"`
	// TotalBytesSynced 同步总字节数
	TotalBytesSynced int64 `json:"total_bytes_synced"`
	// Devices 涉及设备列表
	Devices []string `json:"devices"`
}

// ============================================================================
// 查询/过滤器/配置
// ============================================================================

// LineageFilter 血缘查询过滤器
type LineageFilter struct {
	// FileID 按文件ID过滤
	FileID string `json:"file_id,omitempty"`
	// FilePath 按文件路径过滤（前缀匹配）
	FilePath string `json:"file_path,omitempty"`
	// UserID 按用户ID过滤
	UserID string `json:"user_id,omitempty"`
	// DeviceID 按设备ID过滤
	DeviceID string `json:"device_id,omitempty"`
	// Source 按来源类型过滤
	Source OriginSource `json:"source,omitempty"`
	// Classification 按数据分类过滤
	Classification DataClassification `json:"classification,omitempty"`
	// EventTypes 按事件类型过滤
	EventTypes []EventType `json:"event_types,omitempty"`
	// HasPII 仅包含有PII的文件
	HasPII *bool `json:"has_pii,omitempty"`
	// Tags 按标签过滤
	Tags []string `json:"tags,omitempty"`
	// StartTime 开始时间
	StartTime *time.Time `json:"start_time,omitempty"`
	// EndTime 结束时间
	EndTime *time.Time `json:"end_time,omitempty"`
	// Limit 返回数量限制
	Limit int `json:"limit,omitempty"`
	// Offset 分页偏移量
	Offset int `json:"offset,omitempty"`
}

// Config 数据血缘追踪配置
type Config struct {
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// TrackAllFiles 是否追踪所有文件
	TrackAllFiles bool `json:"track_all_files"`
	// WatchPaths 监控路径列表
	WatchPaths []string `json:"watch_paths"`
	// ExcludePaths 排除路径列表
	ExcludePaths []string `json:"exclude_paths"`
	// AutoTagging 是否启用自动标签
	AutoTagging bool `json:"auto_tagging"`
	// PIIDetection 是否启用PII检测
	PIIDetection bool `json:"pii_detection"`
	// MaxHistoryPerFile 每文件最大历史记录数
	MaxHistoryPerFile int `json:"max_history_per_file"`
	// RetentionDays 血缘记录保留天数
	RetentionDays int `json:"retention_days"`
	// SyncTracking 是否启用同步追踪
	SyncTracking bool `json:"sync_tracking"`
	// AutoClassify 是否启用自动分类
	AutoClassify bool `json:"auto_classify"`
	// ReportInterval 报告生成间隔（小时）
	ReportInterval int `json:"report_interval"`
}

// ============================================================================
// API 请求/响应
// ============================================================================

// RecordEventRequest 记录事件请求
type RecordEventRequest struct {
	// FileID 文件ID
	FileID string `json:"file_id" binding:"required"`
	// FilePath 文件路径
	FilePath string `json:"file_path" binding:"required"`
	// FileName 文件名
	FileName string `json:"file_name"`
	// EventType 事件类型
	EventType EventType `json:"event_type" binding:"required"`
	// DeviceID 设备ID
	DeviceID string `json:"device_id"`
	// DeviceName 设备名称
	DeviceName string `json:"device_name"`
	// UserID 用户ID
	UserID string `json:"user_id"`
	// UserName 用户名
	UserName string `json:"user_name"`
	// OldPath 原路径（移动/重命名时）
	OldPath string `json:"old_path,omitempty"`
	// Checksum 校验和
	Checksum string `json:"checksum,omitempty"`
	// Description 描述
	Description string `json:"description"`
	// Metadata 附加元数据
	Metadata map[string]string `json:"metadata,omitempty"`
}

// RecordOriginRequest 记录来源请求
type RecordOriginRequest struct {
	// FileID 文件ID
	FileID string `json:"file_id" binding:"required"`
	// FilePath 文件路径
	FilePath string `json:"file_path" binding:"required"`
	// FileName 文件名
	FileName string `json:"file_name"`
	// Source 来源类型
	Source OriginSource `json:"source" binding:"required"`
	// DeviceID 来源设备ID
	DeviceID string `json:"device_id"`
	// DeviceName 来源设备名称
	DeviceName string `json:"device_name"`
	// UserID 创建者ID
	UserID string `json:"user_id"`
	// UserName 创建者名称
	UserName string `json:"user_name"`
	// Application 来源应用
	Application string `json:"application"`
	// OriginalPath 原始路径
	OriginalPath string `json:"original_path"`
	// ExternalRef 外部引用
	ExternalRef string `json:"external_ref"`
}

// AddDependencyRequest 添加依赖关系请求
type AddDependencyRequest struct {
	// SourceID 源文件ID
	SourceID string `json:"source_id" binding:"required"`
	// TargetID 目标文件ID
	TargetID string `json:"target_id" binding:"required"`
	// Type 依赖类型
	Type DependencyType `json:"type" binding:"required"`
	// Description 关系描述
	Description string `json:"description"`
	// Strength 关系强度
	Strength float64 `json:"strength"`
}

// RecordSyncRequest 记录同步请求
type RecordSyncRequest struct {
	// FileID 文件ID
	FileID string `json:"file_id" binding:"required"`
	// FilePath 文件路径
	FilePath string `json:"file_path"`
	// SourceDeviceID 源设备ID
	SourceDeviceID string `json:"source_device_id" binding:"required"`
	// SourceDeviceName 源设备名称
	SourceDeviceName string `json:"source_device_name"`
	// TargetDeviceID 目标设备ID
	TargetDeviceID string `json:"target_device_id" binding:"required"`
	// TargetDeviceName 目标设备名称
	TargetDeviceName string `json:"target_device_name"`
	// Direction 同步方向
	Direction string `json:"direction"`
	// Size 同步数据量
	Size int64 `json:"size"`
}

// ClassifyFileRequest 分类打标请求
type ClassifyFileRequest struct {
	// FileID 文件ID
	FileID string `json:"file_id" binding:"required"`
	// Classification 数据分类
	Classification DataClassification `json:"classification" binding:"required"`
	// Tags 标签列表
	Tags []string `json:"tags"`
	// Reason 打标原因
	Reason string `json:"reason"`
}

// LifecycleReportRequest 生命周期报告请求
type LifecycleReportRequest struct {
	// Start 开始时间
	Start time.Time `json:"start"`
	// End 结束时间
	End time.Time `json:"end"`
	// Label 报告标签
	Label string `json:"label"`
}

// APIResponse 通用API响应
type APIResponse struct {
	// Success 是否成功
	Success bool `json:"success"`
	// Message 响应消息
	Message string `json:"message,omitempty"`
	// Data 响应数据
	Data interface{} `json:"data,omitempty"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
}
