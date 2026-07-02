// Package smb3unixext 提供 SMB3 Unix 扩展支持。
// 对标 TrueNAS 26 的 SMB3 Unix Extensions 功能，允许 Linux 客户端
// 通过 SMB3 POSIX 支持使用文件系统原语（符号链接、权限、锁等）。
package smb3unixext

import "time"

// ExtensionStatus 扩展启用状态.
type ExtensionStatus string

const (
	// ExtensionStatusEnabled 已启用.
	ExtensionStatusEnabled ExtensionStatus = "enabled"
	// ExtensionStatusDisabled 已禁用.
	ExtensionStatusDisabled ExtensionStatus = "disabled"
)

// ClientCapability 客户端能力标志.
type ClientCapability string

const (
	// CapabilityPosixPath POSIX 路径操作.
	CapabilityPosixPath ClientCapability = "posix_path_operations"
	// CapabilityPosixSymlink POSIX 符号链接操作.
	CapabilityPosixSymlink ClientCapability = "posix_symlink_operations"
	// CapabilityPosixFileLock POSIX 文件范围锁.
	CapabilityPosixFileLock ClientCapability = "posix_file_range_lock"
	// CapabilityPosixACL POSIX ACL 操作.
	CapabilityPosixACL ClientCapability = "posix_acl_operations"
	// CapabilityPosixRename POSIX 重命名操作.
	CapabilityPosixRename ClientCapability = "posix_rename_operations"
	// CapabilityPosixSetInfo POSIX 属性设置.
	CapabilityPosixSetInfo ClientCapability = "posix_set_info"
)

// DefaultCapabilities 默认支持的 POSIX 扩展能力列表.
var DefaultCapabilities = []ClientCapability{
	CapabilityPosixPath,
	CapabilityPosixSymlink,
	CapabilityPosixFileLock,
	CapabilityPosixACL,
	CapabilityPosixRename,
	CapabilityPosixSetInfo,
}

// ShareProtocol 共享协议类型.
type ShareProtocol string

const (
	// ProtocolSMB 仅 SMB 协议.
	ProtocolSMB ShareProtocol = "smb"
	// ProtocolNFS 仅 NFS 协议.
	ProtocolNFS ShareProtocol = "nfs"
	// ProtocolAFP 仅 AFP 协议.
	ProtocolAFP ShareProtocol = "afp"
	// ProtocolMulti 多协议（SMB + NFS 等）.
	ProtocolMulti ShareProtocol = "multi"
)

// UnixExtensionConfig SMB3 Unix 扩展配置.
type UnixExtensionConfig struct {
	// 共享名称
	ShareName string `json:"share_name"`
	// 是否启用 Unix 扩展
	Enabled bool `json:"enabled"`
	// 共享协议模式
	Protocol ShareProtocol `json:"protocol"`
	// 是否为多协议模式（自动判断）
	IsMultiProtocol bool `json:"is_multi_protocol"`
	// 支持的 POSIX 扩展能力列表
	Capabilities []ClientCapability `json:"capabilities,omitempty"`
	// 客户端能力协商结果
	ClientNegotiated bool `json:"client_negotiated,omitempty"`
	// 协商到的客户端能力
	NegotiatedCapabilities []ClientCapability `json:"negotiated_capabilities,omitempty"`
	// 配置更新时间
	UpdatedAt time.Time `json:"updated_at"`
	// 配置创建时间
	CreatedAt time.Time `json:"created_at"`
}

// SetExtensionRequest 设置 Unix 扩展请求.
type SetExtensionRequest struct {
	// 共享名称
	ShareName string `json:"share_name" binding:"required"`
	// 是否启用
	Enabled bool `json:"enabled"`
}

// ClientCapabilityRequest 客户端能力协商请求.
type ClientCapabilityRequest struct {
	// 共享名称
	ShareName string `json:"share_name" binding:"required"`
	// 客户端支持的能力列表
	ClientCapabilities []ClientCapability `json:"client_capabilities" binding:"required"`
}

// ExtensionStatusResponse 扩展状态响应.
type ExtensionStatusResponse struct {
	// 共享名称
	ShareName string `json:"share_name"`
	// 是否启用
	Enabled bool `json:"enabled"`
	// 共享协议
	Protocol ShareProtocol `json:"protocol"`
	// 是否多协议
	IsMultiProtocol bool `json:"is_multi_protocol"`
	// 状态字符串
	Status ExtensionStatus `json:"status"`
	// 支持的能力列表
	Capabilities []ClientCapability `json:"capabilities,omitempty"`
	// 客户端是否已协商
	ClientNegotiated bool `json:"client_negotiated"`
	// 协商到的客户端能力
	NegotiatedCapabilities []ClientCapability `json:"negotiated_capabilities,omitempty"`
}

// SupportStatusResponse 全局支持状态响应.
type SupportStatusResponse struct {
	// 是否支持 SMB3 Unix 扩展（编译期/运行期能力）
	Supported bool `json:"supported"`
	// SMB 最低协议版本
	MinSMBVersion string `json:"min_smb_version"`
	// 已启用扩展的共享数量
	EnabledShares int `json:"enabled_shares"`
	// 已配置的共享总数
	TotalShares int `json:"total_shares"`
	// 默认能力列表
	DefaultCapabilities []ClientCapability `json:"default_capabilities"`
}
