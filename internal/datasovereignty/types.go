// Package datasovereignty 提供数据主权标签管理功能，
// 按地理合规要求标记数据，限制数据跨域传输，记录审计日志。
package datasovereignty

import "time"

// ========== 合规框架类型 ==========

// ComplianceFramework 合规框架标识.
type ComplianceFramework string

const (
	// FrameworkGDPR 欧盟通用数据保护条例.
	FrameworkGDPR ComplianceFramework = "GDPR"
	// FrameworkPIPL 中国个人信息保护法.
	FrameworkPIPL ComplianceFramework = "PIPL"
	// FrameworkCCPA 加州消费者隐私法.
	FrameworkCCPA ComplianceFramework = "CCPA"
	// FrameworkLGPD 巴西通用数据保护法.
	FrameworkLGPD ComplianceFramework = "LGPD"
	// FrameworkPDPA 新加坡个人数据保护法.
	FrameworkPDPA ComplianceFramework = "PDPA"
)

// DataRegion 数据地理区域.
type DataRegion string

const (
	// RegionEU 欧盟区域.
	RegionEU DataRegion = "EU"
	// RegionCN 中国大陆.
	RegionCN DataRegion = "CN"
	// RegionUS 美国区域.
	RegionUS DataRegion = "US"
	// RegionCA 加拿大.
	RegionCA DataRegion = "CA"
	// RegionBR 巴西.
	RegionBR DataRegion = "BR"
	// RegionSG 新加坡.
	RegionSG DataRegion = "SG"
	// RegionGlobal 全球（无区域限制）.
	RegionGlobal DataRegion = "GLOBAL"
)

// ResourceType 资源类型.
type ResourceType string

const (
	// ResourceFile 单个文件.
	ResourceFile ResourceType = "file"
	// ResourceFolder 文件夹.
	ResourceFolder ResourceType = "folder"
	// ResourceStoragePool 存储池.
	ResourceStoragePool ResourceType = "storage_pool"
	// ResourceShare 共享目录.
	ResourceShare ResourceType = "share"
)

// ========== 标签类型 ==========

// SovereigntyTag 数据主权标签，标记数据的合规框架和地理约束.
type SovereigntyTag struct {
	ID                string                `json:"id"`                           // 标签唯一标识
	ResourcePath      string                `json:"resource_path"`                // 资源路径
	ResourceType      ResourceType          `json:"resource_type"`                // 资源类型
	Frameworks        []ComplianceFramework `json:"frameworks"`                   // 适用的合规框架
	AllowedRegions    []DataRegion          `json:"allowed_regions"`              // 允许存储的区域
	RestrictedRegions []DataRegion          `json:"restricted_regions,omitempty"` // 禁止存储的区域
	DataSubject       string                `json:"data_subject,omitempty"`       // 数据主体（如个人信息涉及的对象）
	Description       string                `json:"description,omitempty"`        // 标签描述
	CreatedAt         time.Time             `json:"created_at"`                   // 创建时间
	UpdatedAt         time.Time             `json:"updated_at"`                   // 更新时间
	CreatedBy         string                `json:"created_by"`                   // 创建者
}

// TransferAction 数据传输动作类型.
type TransferAction string

const (
	// ActionCopy 复制操作.
	ActionCopy TransferAction = "copy"
	// ActionMove 移动操作.
	ActionMove TransferAction = "move"
	// ActionReplicate 复制（存储池间）.
	ActionReplicate TransferAction = "replicate"
	// ActionSync 同步操作.
	ActionSync TransferAction = "sync"
	// ActionBackup 备份操作.
	ActionBackup TransferAction = "backup"
	// ActionDownload 下载操作.
	ActionDownload TransferAction = "download"
	// ActionUpload 上传操作.
	ActionUpload TransferAction = "upload"
)

// TransferStatus 传输合规状态.
type TransferStatus string

const (
	// TransferAllowed 允许传输.
	TransferAllowed TransferStatus = "allowed"
	// TransferBlocked 阻止传输.
	TransferBlocked TransferStatus = "blocked"
	// TransferWarning 警告（需人工审核）.
	TransferWarning TransferStatus = "warning"
)

// ========== 审计日志类型 ==========

// AuditEntry 数据跨域操作审计日志.
type AuditEntry struct {
	ID           string         `json:"id"`                  // 日志唯一标识
	Timestamp    time.Time      `json:"timestamp"`           // 操作时间
	ResourcePath string         `json:"resource_path"`       // 资源路径
	ResourceType ResourceType   `json:"resource_type"`       // 资源类型
	Action       TransferAction `json:"action"`              // 传输动作
	SourceRegion DataRegion     `json:"source_region"`       // 源区域
	TargetRegion DataRegion     `json:"target_region"`       // 目标区域
	Status       TransferStatus `json:"status"`              // 合规状态
	User         string         `json:"user"`                // 操作用户
	ClientIP     string         `json:"client_ip,omitempty"` // 客户端 IP
	Reason       string         `json:"reason,omitempty"`    // 阻止/警告原因
	TagID        string         `json:"tag_id,omitempty"`    // 关联标签 ID
}

// ========== 请求/响应类型 ==========

// TagRequest 创建/更新数据主权标签请求.
type TagRequest struct {
	ResourcePath      string                `json:"resource_path" binding:"required"`         // 资源路径
	ResourceType      ResourceType          `json:"resource_type" binding:"required"`         // 资源类型
	Frameworks        []ComplianceFramework `json:"frameworks" binding:"required,min=1"`      // 合规框架
	AllowedRegions    []DataRegion          `json:"allowed_regions" binding:"required,min=1"` // 允许区域
	RestrictedRegions []DataRegion          `json:"restricted_regions,omitempty"`             // 禁止区域
	DataSubject       string                `json:"data_subject,omitempty"`                   // 数据主体
	Description       string                `json:"description,omitempty"`                    // 描述
	CreatedBy         string                `json:"created_by" binding:"required"`            // 创建者
}

// CheckRequest 合规检查请求.
type CheckRequest struct {
	ResourcePath string         `json:"resource_path" binding:"required"` // 资源路径
	Action       TransferAction `json:"action" binding:"required"`        // 传输动作
	TargetRegion DataRegion     `json:"target_region" binding:"required"` // 目标区域
	User         string         `json:"user,omitempty"`                   // 操作用户
	ClientIP     string         `json:"client_ip,omitempty"`              // 客户端 IP
}

// CheckResponse 合规检查响应.
type CheckResponse struct {
	Allowed bool            `json:"allowed"`            // 是否允许
	Status  TransferStatus  `json:"status"`             // 合规状态
	Tag     *SovereigntyTag `json:"tag,omitempty"`      // 关联标签
	Reason  string          `json:"reason,omitempty"`   // 原因
	EntryID string          `json:"entry_id,omitempty"` // 审计日志 ID
}

// AuditQuery 审计日志查询条件.
type AuditQuery struct {
	ResourcePath string         `json:"resource_path,omitempty"` // 按资源路径过滤
	Action       TransferAction `json:"action,omitempty"`        // 按动作过滤
	Status       TransferStatus `json:"status,omitempty"`        // 按状态过滤
	User         string         `json:"user,omitempty"`          // 按用户过滤
	StartTime    *time.Time     `json:"start_time,omitempty"`    // 起始时间
	EndTime      *time.Time     `json:"end_time,omitempty"`      // 结束时间
	Limit        int            `json:"limit,omitempty"`         // 返回条数上限
}
