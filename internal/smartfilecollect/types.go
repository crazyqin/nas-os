// Package smartfilecollect - 智能文件收集模块
// 基于群晖 File Request 的增强版，通过链接安全收集文件
// 支持自动分类、去重、病毒扫描
package smartfilecollect

import (
	"time"
)

// ============================================================
// 配置类型
// ============================================================

// CollectConfig 收集模块配置.
type CollectConfig struct {
	// 基础配置
	MaxFileSize    int64    `json:"max_file_size"`   // 最大文件大小 (字节), 默认 1GB
	MaxTotalSize   int64    `json:"max_total_size"`  // 单次收集最大总量 (字节), 默认 10GB
	AllowedExts    []string `json:"allowed_exts"`    // 允许的文件扩展名, 空表示不限
	BlockedExts    []string `json:"blocked_exts"`    // 禁止的文件扩展名
	MaxSubmissions int      `json:"max_submissions"` // 最大提交数, 0表示不限
	RequireAuth    bool     `json:"require_auth"`    // 是否需要认证提交
	AllowAnonymous bool     `json:"allow_anonymous"` // 是否允许匿名提交

	// 安全配置
	EnableVirusScan bool `json:"enable_virus_scan"` // 启用病毒扫描
	EnableDedup     bool `json:"enable_dedup"`      // 启用去重
	EnableAutoClass bool `json:"enable_auto_class"` // 启用自动分类

	// 存储配置
	StoragePath string `json:"storage_path"` // 存储根路径
	TempPath    string `json:"temp_path"`    // 临时文件路径

	// 链接配置
	DefaultExpireDays int `json:"default_expire_days"` // 默认过期天数, 默认 7
	MaxExpireDays     int `json:"max_expire_days"`     // 最大过期天数, 默认 30
}

// DefaultCollectConfig 默认收集配置.
func DefaultCollectConfig() CollectConfig {
	return CollectConfig{
		MaxFileSize:       1 << 30,  // 1GB
		MaxTotalSize:      10 << 30, // 10GB
		AllowedExts:       []string{},
		BlockedExts:       []string{".exe", ".bat", ".cmd", ".vbs", ".js"},
		MaxSubmissions:    0,
		RequireAuth:       false,
		AllowAnonymous:    true,
		EnableVirusScan:   true,
		EnableDedup:       true,
		EnableAutoClass:   true,
		StoragePath:       "/data/collect",
		TempPath:          "/tmp/collect",
		DefaultExpireDays: 7,
		MaxExpireDays:     30,
	}
}

// ============================================================
// 收集请求状态
// ============================================================

// CollectStatus 收集请求状态.
type CollectStatus string

const (
	CollectStatusActive  CollectStatus = "active"  // 活跃
	CollectStatusPaused  CollectStatus = "paused"  // 暂停
	CollectStatusExpired CollectStatus = "expired" // 已过期
	CollectStatusClosed  CollectStatus = "closed"  // 已关闭
	CollectStatusFull    CollectStatus = "full"    // 已满
)

// ============================================================
// 提交状态
// ============================================================

// SubmissionStatus 提交状态.
type SubmissionStatus string

const (
	SubmissionStatusPending   SubmissionStatus = "pending"   // 待处理
	SubmissionStatusScanning  SubmissionStatus = "scanning"  // 扫描中
	SubmissionStatusClean     SubmissionStatus = "clean"     // 安全
	SubmissionStatusInfected  SubmissionStatus = "infected"  // 感染
	SubmissionStatusDuplicate SubmissionStatus = "duplicate" // 重复
	SubmissionStatusRejected  SubmissionStatus = "rejected"  // 拒绝
	SubmissionStatusAccepted  SubmissionStatus = "accepted"  // 已接受
)

// ============================================================
// 文件分类
// ============================================================

// FileCategory 文件分类.
type FileCategory string

const (
	CategoryDocument FileCategory = "document" // 文档
	CategoryImage    FileCategory = "image"    // 图片
	CategoryVideo    FileCategory = "video"    // 视频
	CategoryAudio    FileCategory = "audio"    // 音频
	CategoryArchive  FileCategory = "archive"  // 压缩包
	CategoryCode     FileCategory = "code"     // 代码
	CategoryOther    FileCategory = "other"    // 其他
)

// ============================================================
// 核心数据类型
// ============================================================

// CollectRequest 收集请求.
type CollectRequest struct {
	ID              string        `json:"id"`               // 请求ID
	Title           string        `json:"title"`            // 标题
	Description     string        `json:"description"`      // 描述
	CreatorID       string        `json:"creator_id"`       // 创建者ID
	CreatorName     string        `json:"creator_name"`     // 创建者名称
	ShareLink       string        `json:"share_link"`       // 分享链接
	AccessToken     string        `json:"access_token"`     // 访问令牌
	Status          CollectStatus `json:"status"`           // 状态
	TargetPath      string        `json:"target_path"`      // 目标存储路径
	Config          CollectPolicy `json:"config"`           // 收集策略
	SubmissionCount int           `json:"submission_count"` // 已提交数量
	TotalSize       int64         `json:"total_size"`       // 已接收总大小
	CreatedAt       time.Time     `json:"created_at"`       // 创建时间
	ExpiresAt       *time.Time    `json:"expires_at"`       // 过期时间
	UpdatedAt       time.Time     `json:"updated_at"`       // 更新时间
}

// CollectPolicy 收集策略.
type CollectPolicy struct {
	MaxFileSize     int64    `json:"max_file_size"`     // 最大文件大小
	MaxTotalSize    int64    `json:"max_total_size"`    // 最大总量
	AllowedExts     []string `json:"allowed_exts"`      // 允许的扩展名
	BlockedExts     []string `json:"blocked_exts"`      // 禁止的扩展名
	MaxSubmissions  int      `json:"max_submissions"`   // 最大提交数
	RequireAuth     bool     `json:"require_auth"`      // 需要认证
	AllowAnonymous  bool     `json:"allow_anonymous"`   // 允许匿名
	EnableVirusScan bool     `json:"enable_virus_scan"` // 启用病毒扫描
	EnableDedup     bool     `json:"enable_dedup"`      // 启用去重
	AutoClassify    bool     `json:"auto_classify"`     // 自动分类
	NotifyOnSubmit  bool     `json:"notify_on_submit"`  // 提交时通知
}

// FileSubmission 文件提交.
type FileSubmission struct {
	ID              string           `json:"id"`                // 提交ID
	CollectID       string           `json:"collect_id"`        // 所属收集请求ID
	FileName        string           `json:"file_name"`         // 文件名
	FileSize        int64            `json:"file_size"`         // 文件大小
	FileType        string           `json:"file_type"`         // MIME类型
	FileExt         string           `json:"file_ext"`          // 文件扩展名
	Category        FileCategory     `json:"category"`          // 文件分类
	Checksum        string           `json:"checksum"`          // 文件校验和 (SHA256)
	Status          SubmissionStatus `json:"status"`            // 状态
	SubmitterName   string           `json:"submitter_name"`    // 提交者名称
	SubmitterEmail  string           `json:"submitter_email"`   // 提交者邮箱
	SubmitterIP     string           `json:"submitter_ip"`      // 提交者IP
	StoragePath     string           `json:"storage_path"`      // 存储路径
	VirusScanResult *VirusScanResult `json:"virus_scan_result"` // 病毒扫描结果
	RejectionReason string           `json:"rejection_reason"`  // 拒绝原因
	SubmittedAt     time.Time        `json:"submitted_at"`      // 提交时间
	ProcessedAt     *time.Time       `json:"processed_at"`      // 处理时间
}

// VirusScanResult 病毒扫描结果.
type VirusScanResult struct {
	Clean      bool      `json:"clean"`       // 是否干净
	Engine     string    `json:"engine"`      // 扫描引擎
	ThreatName string    `json:"threat_name"` // 威胁名称
	ScannedAt  time.Time `json:"scanned_at"`  // 扫描时间
}

// ============================================================
// HTTP 请求/响应类型
// ============================================================

// CreateCollectRequest 创建收集请求.
type CreateCollectRequest struct {
	Title       string        `json:"title"`       // 标题
	Description string        `json:"description"` // 描述
	TargetPath  string        `json:"target_path"` // 目标路径
	ExpiresIn   int           `json:"expires_in"`  // 过期天数
	Policy      CollectPolicy `json:"policy"`      // 收集策略
}

// SubmitFileRequest 提交文件请求.
type SubmitFileRequest struct {
	SubmitterName  string `json:"submitter_name"`  // 提交者名称
	SubmitterEmail string `json:"submitter_email"` // 提交者邮箱
}

// CollectResponse 收集响应.
type CollectResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// CollectListResponse 收集列表响应.
type CollectListResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    []CollectRequest `json:"data,omitempty"`
}

// SubmissionListResponse 提交列表响应.
type SubmissionListResponse struct {
	Code    int              `json:"code"`
	Message string           `json:"message"`
	Data    []FileSubmission `json:"data,omitempty"`
}

// CollectStats 收集统计.
type CollectStats struct {
	TotalRequests    int   `json:"total_requests"`    // 总请求数
	ActiveRequests   int   `json:"active_requests"`   // 活跃请求数
	TotalSubmissions int   `json:"total_submissions"` // 总提交数
	TotalFiles       int   `json:"total_files"`       // 总文件数
	TotalSize        int64 `json:"total_size"`        // 总大小
	InfectedFiles    int   `json:"infected_files"`    // 感染文件数
	DuplicateFiles   int   `json:"duplicate_files"`   // 重复文件数
}

// StatsResponse 统计响应.
type StatsResponse struct {
	Code    int           `json:"code"`
	Message string        `json:"message"`
	Data    *CollectStats `json:"data,omitempty"`
}
