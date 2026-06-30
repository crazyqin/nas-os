// Package smbunixext 提供 SMB3 Unix Extensions 支持，
// 允许 Multi-Protocol 共享的 Linux 客户端使用 SMB3 POSIX 扩展。
// 对标 TrueNAS 26 的 SMB3 unix extensions 功能。
package smbunixext

import "time"

// ExtensionStatus 扩展启用状态
type ExtensionStatus string

const (
	ExtensionStatusEnabled  ExtensionStatus = "enabled"
	ExtensionStatusDisabled ExtensionStatus = "disabled"
)

// ShareProtocol 共享协议类型
type ShareProtocol string

const (
	ProtocolSMB    ShareProtocol = "smb"
	ProtocolNFS    ShareProtocol = "nfs"
	ProtocolAFP   ShareProtocol = "afp"
	ProtocolMulti ShareProtocol = "multi" // 多协议
)

// UnixExtensionConfig SMB3 Unix Extensions 配置
type UnixExtensionConfig struct {
	// 共享名称
	ShareName string `json:"share_name"`
	// 是否启用 Unix Extensions
	Enabled bool `json:"enabled"`
	// 共享协议模式
	Protocol ShareProtocol `json:"protocol"`
	// 是否为 Multi-Protocol 模式（自动判断）
	IsMultiProtocol bool `json:"is_multi_protocol"`
	// 支持的 POSIX 扩展能力列表
	Capabilities []string `json:"capabilities,omitempty"`
	// 配置更新时间
	UpdatedAt time.Time `json:"updated_at"`
}

// SetExtensionRequest 设置 Unix Extensions 请求
type SetExtensionRequest struct {
	ShareName string `json:"share_name"`
	Enabled   bool   `json:"enabled"`
}

// ExtensionStatusResponse 扩展状态响应
type ExtensionStatusResponse struct {
	ShareName       string          `json:"share_name"`
	Enabled         bool            `json:"enabled"`
	Protocol        ShareProtocol   `json:"protocol"`
	IsMultiProtocol bool            `json:"is_multi_protocol"`
	Status          ExtensionStatus `json:"status"`
	Capabilities    []string        `json:"capabilities,omitempty"`
}

// CapabilityDefaults 默认支持的 POSIX 扩展能力
var CapabilityDefaults = []string{
	"posix_path_operations",
	"posix_symlink_operations",
	"posix_file_range_lock",
	"posix_acl_operations",
	"posix_rename_operations",
	"posix_set_info",
}