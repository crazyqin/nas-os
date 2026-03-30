// Package web NAS-OS API 类型和文档模型
package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ========== 通用响应结构 ==========

// Response 通用 API 响应.
type Response struct {
	Code    int         `json:"code" example:"0"`
	Message string      `json:"message" example:"success"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse 错误响应.
type ErrorResponse struct {
	Code    int    `json:"code" example:"400"`
	Message string `json:"message" example:"请求参数错误"`
}

// ========== 统一错误响应辅助函数 ==========

// respondSuccess 成功响应.
func respondSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

// respondSuccessMessage 成功响应（仅消息）.
func respondSuccessMessage(c *gin.Context, message string) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": message,
	})
}

// respondBadRequest 400 错误响应.
func respondBadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"code":    400,
		"message": message,
	})
}

// respondNotFound 404 错误响应.
func respondNotFound(c *gin.Context, message string) {
	c.JSON(http.StatusNotFound, gin.H{
		"code":    404,
		"message": message,
	})
}

// respondInternalError 500 错误响应.
func respondInternalError(c *gin.Context, message string) {
	c.JSON(http.StatusInternalServerError, gin.H{
		"code":    500,
		"message": message,
	})
}

// respondServiceUnavailable 503 错误响应.
func respondServiceUnavailable(c *gin.Context, message string) {
	c.JSON(http.StatusServiceUnavailable, gin.H{
		"code":    503,
		"message": message,
	})
}

// respondError 通用错误响应.
func respondError(c *gin.Context, statusCode int, code int, message string) {
	c.JSON(statusCode, gin.H{
		"code":    code,
		"message": message,
	})
}

// ========== 卷管理 API 模型 ==========

// VolumeCreateRequest 创建卷请求.
type VolumeCreateRequest struct {
	Name    string   `json:"name" binding:"required" example:"data-vol"`
	Devices []string `json:"devices" binding:"required" example:"/dev/sda,/dev/sdb"`
	Profile string   `json:"profile" example:"raid1"` // single, raid0, raid1, raid10, raid5, raid6
}

// Volume 卷信息.
type Volume struct {
	Name       string   `json:"name"`
	UUID       string   `json:"uuid"`
	Mounted    bool     `json:"mounted"`
	MountPoint string   `json:"mountPoint"`
	TotalSize  uint64   `json:"totalSize"`
	UsedSize   uint64   `json:"usedSize"`
	FreeSize   uint64   `json:"freeSize"`
	Profile    string   `json:"profile"`
	Devices    []string `json:"devices"`
	SubVolumes []string `json:"subvolumes,omitempty"`
}

// VolumeUsage 卷使用量.
type VolumeUsage struct {
	Total uint64 `json:"total" example:"107374182400"`
	Used  uint64 `json:"used" example:"21474836480"`
	Free  uint64 `json:"free" example:"85899345920"`
}

// DeviceAddRequest 添加设备请求.
type DeviceAddRequest struct {
	Device string `json:"device" binding:"required" example:"/dev/sdc"`
}

// ========== 子卷管理 API 模型 ==========

// SubVolumeCreateRequest 创建子卷请求.
type SubVolumeCreateRequest struct {
	Name string `json:"name" binding:"required" example:"documents"`
}

// SubVolumeReadOnlyRequest 设置子卷只读请求.
type SubVolumeReadOnlyRequest struct {
	ReadOnly bool `json:"readOnly" example:"true"`
}

// ========== 快照管理 API 模型 ==========

// SnapshotCreateRequest 创建快照请求.
type SnapshotCreateRequest struct {
	SubVolumeName string `json:"subvolume" binding:"required" example:"documents"`
	Name          string `json:"name" binding:"required" example:"backup-2024-01-01"`
	ReadOnly      bool   `json:"readonly" example:"true"`
}

// SnapshotRestoreRequest 恢复快照请求.
type SnapshotRestoreRequest struct {
	TargetName string `json:"target" binding:"required" example:"documents-restored"`
}

// ========== RAID 配置 API 模型 ==========

// RAIDConvertRequest RAID 转换请求.
type RAIDConvertRequest struct {
	DataProfile string `json:"dataProfile" example:"raid1"`
	MetaProfile string `json:"metaProfile" example:"raid1"`
}

// ========== 用户管理 API 模型 ==========

// UserInput 用户输入.
type UserInput struct {
	Username string `json:"username" binding:"required" example:"john"`
	Password string `json:"password" binding:"required,min=6" example:"secret123"`
	Shell    string `json:"shell" example:"/bin/bash"`
	HomeDir  string `json:"homeDir" example:"/home/john"`
	Role     string `json:"role" example:"user"` // admin, user, guest
}

// User 用户信息.
type User struct {
	Username string   `json:"username"`
	UID      int      `json:"uid"`
	GID      int      `json:"gid"`
	HomeDir  string   `json:"homeDir"`
	Shell    string   `json:"shell"`
	Disabled bool     `json:"disabled"`
	Role     string   `json:"role"`
	Groups   []string `json:"groups"`
}

// LoginRequest 登录请求.
type LoginRequest struct {
	Username string `json:"username" binding:"required" example:"admin"`
	Password string `json:"password" binding:"required" example:"password"`
}

// LoginResponse 登录响应.
type LoginResponse struct {
	Token     string `json:"token" example:"eyJhbGciOiJIUzI1NiIs..."`
	ExpiresAt string `json:"expires_at" example:"2024-01-02T15:04:05Z"`
	User      *User  `json:"user"`
}

// ChangePasswordRequest 修改密码请求.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required" example:"oldpass"`
	NewPassword string `json:"new_password" binding:"required,min=6" example:"newpass"`
}

// ResetPasswordRequest 重置密码请求.
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=6" example:"newpass"`
}

// SetRoleRequest 设置角色请求.
type SetRoleRequest struct {
	Role string `json:"role" binding:"required" example:"admin"`
}

// GroupInput 用户组输入.
type GroupInput struct {
	Name string `json:"name" binding:"required" example:"developers"`
	GID  int    `json:"gid" example:"1001"`
}

// ========== 共享管理 API 模型 ==========

// ShareOverview 共享概览.
type ShareOverview struct {
	Type   string      `json:"type" example:"smb"` // "smb" or "nfs"
	Name   string      `json:"name" example:"documents"`
	Path   string      `json:"path" example:"/mnt/data/documents"`
	Config interface{} `json:"config"`
}

// SMBShareInput SMB 共享输入.
type SMBShareInput struct {
	Name        string          `json:"name" binding:"required" example:"documents"`
	Path        string          `json:"path" binding:"required" example:"/mnt/data/documents"`
	Comment     string          `json:"comment" example:"文档共享"`
	Browseable  bool            `json:"browseable" example:"true"`
	ReadOnly    bool            `json:"readOnly" example:"false"`
	GuestOK     bool            `json:"guestOk" example:"false"`
	Permissions map[string]bool `json:"permissions"` // username -> readWrite
}

// SMBPermissionRequest SMB 权限设置请求.
type SMBPermissionRequest struct {
	Username  string `json:"username" binding:"required" example:"john"`
	ReadWrite bool   `json:"read_write" example:"true"`
}

// NFSExportInput NFS 导出输入.
type NFSExportInput struct {
	Name    string   `json:"name" binding:"required" example:"media"`
	Path    string   `json:"path" binding:"required" example:"/mnt/data/media"`
	Clients []string `json:"clients" example:"192.168.1.0/24,10.0.0.1"`
	Options []string `json:"options" example:"rw,sync,no_subtree_check"`
}

// ========== 网络管理 API 模型 ==========

// InterfaceConfig 网络接口配置.
type InterfaceConfig struct {
	IP      string `json:"ip" example:"192.168.1.100"`
	Netmask string `json:"netmask" example:"255.255.255.0"`
	Gateway string `json:"gateway" example:"192.168.1.1"`
	DNS     string `json:"dns" example:"8.8.8.8"`
	DHCP    bool   `json:"dhcp" example:"false"`
}

// ToggleInterfaceRequest 启用/禁用接口请求.
type ToggleInterfaceRequest struct {
	Up bool `json:"up" example:"true"`
}

// DDNSConfig DDNS 配置.
type DDNSConfig struct {
	Domain   string `json:"domain" binding:"required" example:"mynas.example.com"`
	Provider string `json:"provider" binding:"required" example:"cloudflare"` // cloudflare, noip, duckdns
	Username string `json:"username" example:"user@example.com"`
	Password string `json:"password" example:"api_token"`
	Interval int    `json:"interval" example:"300"` // 秒
	Enabled  bool   `json:"enabled" example:"true"`
}

// PortForward 端口转发规则.
type PortForward struct {
	Name         string `json:"name" binding:"required" example:"web-server"`
	Protocol     string `json:"protocol" binding:"required" example:"tcp"` // tcp, udp
	ExternalIP   string `json:"externalIp" example:"0.0.0.0"`
	ExternalPort int    `json:"externalPort" binding:"required" example:"8080"`
	InternalIP   string `json:"internalIp" binding:"required" example:"192.168.1.100"`
	InternalPort int    `json:"internalPort" binding:"required" example:"80"`
	Enabled      bool   `json:"enabled" example:"true"`
}

// FirewallRule 防火墙规则.
type FirewallRule struct {
	Name        string   `json:"name" binding:"required" example:"allow-ssh"`
	Chain       string   `json:"chain" binding:"required" example:"INPUT"` // INPUT, OUTPUT, FORWARD
	Protocol    string   `json:"protocol" example:"tcp"`
	Source      string   `json:"source" example:"192.168.1.0/24"`
	Destination string   `json:"destination" example:"0.0.0.0/0"`
	Ports       []string `json:"ports" example:"22,80,443"`
	Action      string   `json:"action" binding:"required" example:"ACCEPT"` // ACCEPT, DROP, REJECT
	Enabled     bool     `json:"enabled" example:"true"`
}

// DefaultPolicyRequest 默认策略设置请求.
type DefaultPolicyRequest struct {
	Chain  string `json:"chain" binding:"required" example:"INPUT"`
	Policy string `json:"policy" binding:"required" example:"DROP"` // ACCEPT, DROP
}

// FlushRulesRequest 清空规则请求.
type FlushRulesRequest struct {
	Chain string `json:"chain" example:"INPUT"` // 空=全部
}

// ========== 系统信息 API 模型 ==========

// SystemInfo 系统信息.
type SystemInfo struct {
	Hostname string `json:"hostname" example:"nas-os"`
	Version  string `json:"version" example:"1.0.0"`
}

// HealthResponse 健康检查响应.
type HealthResponse struct {
	Code    int    `json:"code" example:"0"`
	Message string `json:"message" example:"healthy"`
}
