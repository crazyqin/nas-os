// Package filesyncclient 提供文件同步客户端管理功能
// 支持多设备文件同步、冲突解决、同步状态监控等
package filesyncclient

import "time"

// DeviceType 设备类型
type DeviceType string

const (
	DeviceDesktop DeviceType = "desktop"
	DeviceMobile  DeviceType = "mobile"
	DeviceServer  DeviceType = "server"
)

// ClientStatus 客户端状态
type ClientStatus string

const (
	ClientOnline  ClientStatus = "online"
	ClientOffline ClientStatus = "offline"
)

// SyncMode 同步模式
type SyncMode string

const (
	SyncOneWay SyncMode = "one_way"
	SyncTwoWay SyncMode = "two_way"
	SyncSmart  SyncMode = "smart"
)

// FolderStatus 文件夹同步状态
type FolderStatus string

const (
	FolderActive  FolderStatus = "active"
	FolderPaused  FolderStatus = "paused"
	FolderError   FolderStatus = "error"
	FolderSyncing FolderStatus = "syncing"
)

// ConflictPolicy 冲突策略
type ConflictPolicy string

const (
	ConflictKeepLocal  ConflictPolicy = "keep_local"
	ConflictKeepRemote ConflictPolicy = "keep_remote"
	ConflictKeepBoth   ConflictPolicy = "keep_both"
	ConflictAsk        ConflictPolicy = "ask"
)

// SyncStatus 文件同步状态
type SyncStatus string

const (
	FileSynced   SyncStatus = "synced"
	FileSyncing  SyncStatus = "syncing"
	FileConflict SyncStatus = "conflict"
	FileError    SyncStatus = "error"
	FilePending  SyncStatus = "pending"
)

// EventType 事件类型
type EventType string

const (
	EventCreate   EventType = "create"
	EventUpdate   EventType = "update"
	EventDelete   EventType = "delete"
	EventConflict EventType = "conflict"
)

// SyncClient 同步客户端
type SyncClient struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	DeviceType DeviceType   `json:"device_type"`
	OS         string       `json:"os"`
	LastSeen   time.Time    `json:"last_seen"`
	Status     ClientStatus `json:"status"`
	PairedAt   time.Time    `json:"paired_at"`
}

// SyncFolder 同步文件夹
type SyncFolder struct {
	ID             string         `json:"id"`
	ClientID       string         `json:"client_id"`
	LocalPath      string         `json:"local_path"`
	RemotePath     string         `json:"remote_path"`
	SyncMode       SyncMode       `json:"sync_mode"`
	Status         FolderStatus   `json:"status"`
	LastSync       time.Time      `json:"last_sync"`
	FileCount      int            `json:"file_count"`
	SizeBytes      int64          `json:"size_bytes"`
	ConflictPolicy ConflictPolicy `json:"conflict_policy"`
}

// SyncConflict 同步冲突
type SyncConflict struct {
	ID            string    `json:"id"`
	FolderID      string    `json:"folder_id"`
	FilePath      string    `json:"file_path"`
	LocalVersion  string    `json:"local_version"`
	RemoteVersion string    `json:"remote_version"`
	LocalModTime  time.Time `json:"local_mod_time"`
	RemoteModTime time.Time `json:"remote_mod_time"`
	Resolution    string    `json:"resolution,omitempty"`
	ResolvedAt    time.Time `json:"resolved_at,omitempty"`
}

// SyncFile 同步文件
type SyncFile struct {
	ID           string     `json:"id"`
	FolderID     string     `json:"folder_id"`
	Path         string     `json:"path"`
	SizeBytes    int64      `json:"size_bytes"`
	Checksum     string     `json:"checksum"`
	Version      int        `json:"version"`
	LastModified time.Time  `json:"last_modified"`
	SyncStatus   SyncStatus `json:"sync_status"`
}

// SyncStats 同步统计
type SyncStats struct {
	TotalClients  int     `json:"total_clients"`
	TotalFolders  int     `json:"total_folders"`
	TotalFiles    int     `json:"total_files"`
	TotalSize     int64   `json:"total_size"`
	ActiveSyncs   int     `json:"active_syncs"`
	ConflictCount int     `json:"conflict_count"`
	UploadSpeed   float64 `json:"upload_speed"`
	DownloadSpeed float64 `json:"download_speed"`
}

// SyncEvent 同步事件
type SyncEvent struct {
	ID        string    `json:"id"`
	ClientID  string    `json:"client_id"`
	FolderID  string    `json:"folder_id"`
	EventType EventType `json:"event_type"`
	FilePath  string    `json:"file_path"`
	Timestamp time.Time `json:"timestamp"`
}

// RegisterClientRequest 注册客户端请求
type RegisterClientRequest struct {
	Name       string     `json:"name" binding:"required"`
	DeviceType DeviceType `json:"device_type" binding:"required"`
	OS         string     `json:"os"`
}

// CreateFolderRequest 创建同步文件夹请求
type CreateFolderRequest struct {
	ClientID       string         `json:"client_id" binding:"required"`
	LocalPath      string         `json:"local_path" binding:"required"`
	RemotePath     string         `json:"remote_path" binding:"required"`
	SyncMode       SyncMode       `json:"sync_mode"`
	ConflictPolicy ConflictPolicy `json:"conflict_policy"`
}

// UpdateFolderRequest 更新同步文件夹请求
type UpdateFolderRequest struct {
	SyncMode       SyncMode       `json:"sync_mode"`
	Status         FolderStatus   `json:"status"`
	ConflictPolicy ConflictPolicy `json:"conflict_policy"`
}

// ResolveConflictRequest 解决冲突请求
type ResolveConflictRequest struct {
	Resolution string `json:"resolution" binding:"required"`
}

// IsValidDeviceType 检查设备类型是否有效
func IsValidDeviceType(dt DeviceType) bool {
	switch dt {
	case DeviceDesktop, DeviceMobile, DeviceServer:
		return true
	}
	return false
}

// IsValidSyncMode 检查同步模式是否有效
func IsValidSyncMode(sm SyncMode) bool {
	switch sm {
	case SyncOneWay, SyncTwoWay, SyncSmart:
		return true
	}
	return false
}

// IsValidConflictPolicy 检查冲突策略是否有效
func IsValidConflictPolicy(cp ConflictPolicy) bool {
	switch cp {
	case ConflictKeepLocal, ConflictKeepRemote, ConflictKeepBoth, ConflictAsk:
		return true
	}
	return false
}

// IsValidEventType 检查事件类型是否有效
func IsValidEventType(et EventType) bool {
	switch et {
	case EventCreate, EventUpdate, EventDelete, EventConflict:
		return true
	}
	return false
}
