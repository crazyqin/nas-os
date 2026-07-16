// Package vmimport 提供虚拟机镜像导入导出功能
package vmimport

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

// 虚拟机导入导出相关错误.
var (
	// ErrImportNotFound 导入任务不存在.
	ErrImportNotFound = errors.New("导入任务不存在")
	// ErrExportNotFound 导出任务不存在.
	ErrExportNotFound = errors.New("导出任务不存在")
	// ErrImageNotFound 镜像不存在.
	ErrImageNotFound = errors.New("镜像不存在")
	// ErrUnsupportedFormat 不支持的镜像格式.
	ErrUnsupportedFormat = errors.New("不支持的镜像格式")
	// ErrInvalidFile 无效的镜像文件.
	ErrInvalidFile = errors.New("无效的镜像文件")
	// ErrTaskAlreadyRunning 任务已在运行中.
	ErrTaskAlreadyRunning = errors.New("任务已在运行中")
	// ErrTaskNotRunning 任务未在运行中.
	ErrTaskNotRunning = errors.New("任务未在运行中")
	// ErrConversionFailed 格式转换失败.
	ErrConversionFailed = errors.New("格式转换失败")
	// ErrValidationFailed 镜像验证失败.
	ErrValidationFailed = errors.New("镜像验证失败")
	// ErrStorageFull 存储空间不足.
	ErrStorageFull = errors.New("存储空间不足")
	// ErrDownloadFailed 文件下载失败.
	ErrDownloadFailed = errors.New("文件下载失败")
	// ErrUploadFailed 文件上传失败.
	ErrUploadFailed = errors.New("文件上传失败")
)

// ========== 磁盘格式定义 ==========

// DiskFormat 虚拟磁盘格式.
type DiskFormat string

// 支持的磁盘格式常量.
const (
	// FormatQCOW2 QEMU Copy-On-Write v2.
	FormatQCOW2 DiskFormat = "qcow2"
	// FormatQED QEMU Enhanced Disk.
	FormatQED DiskFormat = "qed"
	// FormatRAW 原始磁盘镜像.
	FormatRAW DiskFormat = "raw"
	// FormatVDI VirtualBox 磁盘镜像.
	FormatVDI DiskFormat = "vdi"
	// FormatVHDX Hyper-V 虚拟硬盘.
	FormatVHDX DiskFormat = "vhdx"
	// FormatVMDK VMware 虚拟磁盘.
	FormatVMDK DiskFormat = "vmdk"
)

// SupportedFormats 所有支持的格式列表.
var SupportedFormats = []DiskFormat{
	FormatQCOW2,
	FormatQED,
	FormatRAW,
	FormatVDI,
	FormatVHDX,
	FormatVMDK,
}

// FormatInfo 格式信息.
type FormatInfo struct {
	// Name 格式名称.
	Name DiskFormat `json:"name"`
	// Description 格式描述.
	Description string `json:"description"`
	// Extension 文件扩展名.
	Extension string `json:"extension"`
	// CanImport 是否支持导入.
	CanImport bool `json:"can_import"`
	// CanExport 是否支持导出.
	CanExport bool `json:"can_export"`
}

// ========== 压缩格式定义 ==========

// CompressFormat 压缩格式.
type CompressFormat string

// 支持的压缩格式常量.
const (
	// CompressNone 不压缩.
	CompressNone CompressFormat = "none"
	// CompressGzip gzip 压缩.
	CompressGzip CompressFormat = "gzip"
	// CompressZstd zstd 压缩.
	CompressZstd CompressFormat = "zstd"
)

// ========== 任务状态定义 ==========

// TaskStatus 任务状态.
type TaskStatus string

// 任务状态常量.
const (
	// StatusPending 等待中.
	StatusPending TaskStatus = "pending"
	// StatusRunning 运行中.
	StatusRunning TaskStatus = "running"
	// StatusCompleted 已完成.
	StatusCompleted TaskStatus = "completed"
	// StatusFailed 失败.
	StatusFailed TaskStatus = "failed"
	// StatusCancelled 已取消.
	StatusCancelled TaskStatus = "cancelled"
)

// ========== 核心数据结构 ==========

// ImportTask 导入任务.
type ImportTask struct {
	// ID 任务唯一标识.
	ID string `json:"id"`
	// Source 导入来源（文件路径或URL）.
	Source string `json:"source"`
	// SourceType 来源类型（file 或 url）.
	SourceType string `json:"source_type"`
	// TargetName 目标镜像名称.
	TargetName string `json:"target_name"`
	// TargetFormat 目标磁盘格式.
	TargetFormat DiskFormat `json:"target_format"`
	// SourceFormat 源磁盘格式（自动检测）.
	SourceFormat DiskFormat `json:"source_format"`
	// Status 任务状态.
	Status TaskStatus `json:"status"`
	// Progress 导入进度（0-100）.
	Progress float64 `json:"progress"`
	// TotalSize 总大小（字节）.
	TotalSize int64 `json:"total_size"`
	// ProcessedSize 已处理大小（字节）.
	ProcessedSize int64 `json:"processed_size"`
	// ErrorMessage 错误信息.
	ErrorMessage string `json:"error_message,omitempty"`
	// ImageID 导入完成后生成的镜像ID.
	ImageID string `json:"image_id,omitempty"`
	// CreateVM 是否自动创建VM配置.
	CreateVM bool `json:"create_vm"`
	// VMName 自动创建的VM名称.
	VMName string `json:"vm_name,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
	// CompletedAt 完成时间.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// cancelCh 用于取消任务的通道.
	cancelCh chan struct{} `json:"-"`
}

// ExportTask 导出任务.
type ExportTask struct {
	// ID 任务唯一标识.
	ID string `json:"id"`
	// ImageID 源镜像ID.
	ImageID string `json:"image_id"`
	// ImageName 源镜像名称.
	ImageName string `json:"image_name"`
	// TargetFormat 目标导出格式.
	TargetFormat DiskFormat `json:"target_format"`
	// Compress 压缩格式.
	Compress CompressFormat `json:"compress"`
	// OutputPath 导出文件路径.
	OutputPath string `json:"output_path"`
	// Status 任务状态.
	Status TaskStatus `json:"status"`
	// Progress 导出进度（0-100）.
	Progress float64 `json:"progress"`
	// TotalSize 总大小（字节）.
	TotalSize int64 `json:"total_size"`
	// ProcessedSize 已处理大小（字节）.
	ProcessedSize int64 `json:"processed_size"`
	// ErrorMessage 错误信息.
	ErrorMessage string `json:"error_message,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
	// CompletedAt 完成时间.
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	// cancelCh 用于取消任务的通道.
	cancelCh chan struct{} `json:"-"`
}

// VMImage 虚拟机镜像.
type VMImage struct {
	// ID 镜像唯一标识.
	ID string `json:"id"`
	// Name 镜像名称.
	Name string `json:"name"`
	// Description 镜像描述.
	Description string `json:"description,omitempty"`
	// Format 磁盘格式.
	Format DiskFormat `json:"format"`
	// FilePath 文件路径.
	FilePath string `json:"file_path"`
	// FileSize 文件大小（字节）.
	FileSize int64 `json:"file_size"`
	// VirtualSize 虚拟磁盘大小（字节）.
	VirtualSize int64 `json:"virtual_size"`
	// Checksum 文件校验和（SHA256）.
	Checksum string `json:"checksum,omitempty"`
	// SourceImportID 来源导入任务ID.
	SourceImportID string `json:"source_import_id,omitempty"`
	// VMConfigID 关联的VM配置ID.
	VMConfigID string `json:"vm_config_id,omitempty"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updated_at"`
}

// StorageUsage 存储空间使用情况.
type StorageUsage struct {
	// TotalSpace 总空间（字节）.
	TotalSpace int64 `json:"total_space"`
	// UsedSpace 已使用空间（字节）.
	UsedSpace int64 `json:"used_space"`
	// FreeSpace 可用空间（字节）.
	FreeSpace int64 `json:"free_space"`
	// ImageCount 镜像数量.
	ImageCount int `json:"image_count"`
	// ImagesTotalSize 镜像总大小（字节）.
	ImagesTotalSize int64 `json:"images_total_size"`
	// StoragePath 存储路径.
	StoragePath string `json:"storage_path"`
}

// ========== 请求/响应结构 ==========

// ImportRequest 导入请求.
type ImportRequest struct {
	// Source 来源（文件路径或URL）.
	Source string `json:"source" binding:"required"`
	// SourceType 来源类型（file 或 url）.
	SourceType string `json:"source_type" binding:"required"`
	// TargetName 目标镜像名称.
	TargetName string `json:"target_name" binding:"required"`
	// TargetFormat 目标格式（默认qcow2）.
	TargetFormat DiskFormat `json:"target_format"`
	// CreateVM 是否自动创建VM配置.
	CreateVM bool `json:"create_vm"`
	// VMName VM名称（CreateVM为true时必填）.
	VMName string `json:"vm_name"`
}

// ExportRequest 导出请求.
type ExportRequest struct {
	// ImageID 源镜像ID.
	ImageID string `json:"image_id" binding:"required"`
	// TargetFormat 导出格式.
	TargetFormat DiskFormat `json:"target_format" binding:"required"`
	// Compress 压缩格式.
	Compress CompressFormat `json:"compress"`
	// OutputPath 输出路径.
	OutputPath string `json:"output_path"`
}

// ValidateRequest 验证请求.
type ValidateRequest struct {
	// FilePath 文件路径.
	FilePath string `json:"file_path" binding:"required"`
}

// ConvertRequest 格式转换请求.
type ConvertRequest struct {
	// ImageID 源镜像ID.
	ImageID string `json:"image_id" binding:"required"`
	// TargetFormat 目标格式.
	TargetFormat DiskFormat `json:"target_format" binding:"required"`
	// OutputName 输出镜像名称.
	OutputName string `json:"output_name" binding:"required"`
}

// ValidateResult 验证结果.
type ValidateResult struct {
	// Valid 是否有效.
	Valid bool `json:"valid"`
	// Format 检测到的格式.
	Format DiskFormat `json:"format,omitempty"`
	// VirtualSize 虚拟磁盘大小.
	VirtualSize int64 `json:"virtual_size,omitempty"`
	// FileSize 文件大小.
	FileSize int64 `json:"file_size,omitempty"`
	// ErrorMessage 错误信息.
	ErrorMessage string `json:"error_message,omitempty"`
}
