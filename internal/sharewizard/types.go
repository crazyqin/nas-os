// Package sharewizard 提供共享创建向导功能
// 引导用户创建SMB/NFS/FTP/WebDAV共享，自动配置权限和配额
// 对标群晖共享文件夹向导
package sharewizard

import (
	"fmt"
	"time"
)

// Protocol 共享协议
type Protocol string

const (
	ProtocolSMB    Protocol = "smb"
	ProtocolNFS    Protocol = "nfs"
	ProtocolFTP    Protocol = "ftp"
	ProtocolWebDAV Protocol = "webdav"
)

// Permission 权限级别
type Permission string

const (
	PermissionReadOnly  Permission = "readonly"
	PermissionReadWrite Permission = "readwrite"
	PermissionAdmin     Permission = "admin"
	PermissionNoAccess  Permission = "noaccess"
)

// ShareTemplate 共享模板
type ShareTemplate string

const (
	TemplateMedia    ShareTemplate = "media"    // 媒体共享
	TemplateDocument ShareTemplate = "document" // 文档共享
	TemplateBackup   ShareTemplate = "backup"   // 备份共享
	TemplateHome     ShareTemplate = "home"     // 家庭目录
	TemplatePublic   ShareTemplate = "public"   // 公共共享
	TemplateTeam     ShareTemplate = "team"     // 团队协作
)

// UserPermission 用户权限
type UserPermission struct {
	Username   string     `json:"username"`
	Permission Permission `json:"permission"`
}

// GroupPermission 组权限
type GroupPermission struct {
	Group      string     `json:"group"`
	Permission Permission `json:"permission"`
}

// QuotaConfig 配额配置
type QuotaConfig struct {
	Enabled  bool  `json:"enabled"`
	MaxSize  int64 `json:"max_size"`  // 字节
	MaxFiles int64 `json:"max_files"` // 文件数
}

// RecycleBinConfig 回收站配置
type RecycleBinConfig struct {
	Enabled        bool `json:"enabled"`
	CleanAfterDays int  `json:"clean_after_days"` // 天后自动清理
}

// EncryptionConfig 加密配置
type EncryptionConfig struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm"` // aes-256-xts 等
}

// ShareConfig 共享配置
type ShareConfig struct {
	Name        string            `json:"name" binding:"required"`
	Description string            `json:"description"`
	Path        string            `json:"path"`
	Protocols   []Protocol        `json:"protocols"`
	Template    ShareTemplate     `json:"template"`
	Users       []UserPermission  `json:"users"`
	Groups      []GroupPermission `json:"groups"`
	Quota       QuotaConfig       `json:"quota"`
	RecycleBin  RecycleBinConfig  `json:"recycle_bin"`
	Encryption  EncryptionConfig  `json:"encryption"`
	Hidden      bool              `json:"hidden"`
	ReadOnly    bool              `json:"read_only"`
	GuestAccess bool              `json:"guest_access"`
	AuditLog    bool              `json:"audit_log"`
	CreatedAt   time.Time         `json:"created_at"`
}

// ShareTemplateDef 模板定义
type ShareTemplateDef struct {
	Name        string        `json:"name"`
	Template    ShareTemplate `json:"template"`
	Description string        `json:"description"`
	Protocols   []Protocol    `json:"protocols"`
	RecycleBin  bool          `json:"recycle_bin"`
	AuditLog    bool          `json:"audit_log"`
	Quota       bool          `json:"quota"`
}

// DefaultTemplates 返回默认模板列表
func DefaultTemplates() []ShareTemplateDef {
	return []ShareTemplateDef{
		{
			Name:        "媒体共享",
			Template:    TemplateMedia,
			Description: "适用于照片、视频、音乐等媒体文件共享",
			Protocols:   []Protocol{ProtocolSMB, ProtocolWebDAV},
			RecycleBin:  true,
			AuditLog:    false,
			Quota:       false,
		},
		{
			Name:        "文档共享",
			Template:    TemplateDocument,
			Description: "适用于办公文档协作，启用版本控制",
			Protocols:   []Protocol{ProtocolSMB, ProtocolWebDAV},
			RecycleBin:  true,
			AuditLog:    true,
			Quota:       true,
		},
		{
			Name:        "备份共享",
			Template:    TemplateBackup,
			Description: "适用于Time Machine、系统备份",
			Protocols:   []Protocol{ProtocolSMB},
			RecycleBin:  false,
			AuditLog:    false,
			Quota:       true,
		},
		{
			Name:        "家庭目录",
			Template:    TemplateHome,
			Description: "每个用户的私人空间",
			Protocols:   []Protocol{ProtocolSMB, ProtocolFTP},
			RecycleBin:  true,
			AuditLog:    false,
			Quota:       true,
		},
		{
			Name:        "公共共享",
			Template:    TemplatePublic,
			Description: "所有人可访问的公共空间",
			Protocols:   []Protocol{ProtocolSMB, ProtocolFTP, ProtocolWebDAV},
			RecycleBin:  true,
			AuditLog:    true,
			Quota:       false,
		},
		{
			Name:        "团队协作",
			Template:    TemplateTeam,
			Description: "团队项目协作空间",
			Protocols:   []Protocol{ProtocolSMB, ProtocolWebDAV},
			RecycleBin:  true,
			AuditLog:    true,
			Quota:       true,
		},
	}
}

// ApplyTemplate 应用模板到配置
func ApplyTemplate(config *ShareConfig, template ShareTemplate) {
	defs := DefaultTemplates()
	for _, def := range defs {
		if def.Template == template {
			config.Template = template
			config.Protocols = def.Protocols
			config.RecycleBin.Enabled = def.RecycleBin
			config.AuditLog = def.AuditLog
			config.Quota.Enabled = def.Quota
			return
		}
	}
}

// ValidateShareConfig 验证共享配置
func ValidateShareConfig(config ShareConfig) error {
	if config.Name == "" {
		return fmt.Errorf("共享名称不能为空")
	}
	if len(config.Protocols) == 0 {
		return fmt.Errorf("至少选择一个共享协议")
	}
	if config.Quota.Enabled && config.Quota.MaxSize <= 0 {
		return fmt.Errorf("启用配额时必须设置最大容量")
	}
	return nil
}
