// Package appleshare 提供 Apple 生态深度优化功能，包括 AirPlay 设备发现、Time Machine 共享、Spotlight 索引和 Apple 优化的 SMB 配置。
package appleshare

import "time"

// AirPlayDeviceStatus AirPlay 设备状态
type AirPlayDeviceStatus string

const (
	AirPlayStatusOnline    AirPlayDeviceStatus = "online"
	AirPlayStatusOffline   AirPlayDeviceStatus = "offline"
	AirPlayStatusStreaming AirPlayDeviceStatus = "streaming"
	AirPlayStatusStandby   AirPlayDeviceStatus = "standby"
)

// AirPlayDeviceType AirPlay 设备类型
type AirPlayDeviceType string

const (
	AirPlayTypeAppleTV    AirPlayDeviceType = "appletv"
	AirPlayTypeHomePod    AirPlayDeviceType = "homepod"
	AirPlayTypeSpeaker    AirPlayDeviceType = "speaker"
	AirPlayTypeReceiver   AirPlayDeviceType = "receiver"
	AirPlayTypeDisplay    AirPlayDeviceType = "display"
	AirPlayTypeUnknown    AirPlayDeviceType = "unknown"
)

// SpotlightIndexStatus Spotlight 索引状态
type SpotlightIndexStatus string

const (
	SpotlightStatusIdle      SpotlightIndexStatus = "idle"
	SpotlightStatusIndexing  SpotlightIndexStatus = "indexing"
	SpotlightStatusPaused    SpotlightIndexStatus = "paused"
	SpotlightStatusError     SpotlightIndexStatus = "error"
)

// AirPlayDevice AirPlay 设备信息
type AirPlayDevice struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	Model          string              `json:"model,omitempty"`
	IP             string              `json:"ip"`
	Port           int                 `json:"port"`
	MAC            string              `json:"mac,omitempty"`
	Type           AirPlayDeviceType   `json:"type"`
	Status         AirPlayDeviceStatus `json:"status"`
	SupportsVideo  bool                `json:"supports_video"`
	SupportsAudio  bool                `json:"supports_audio"`
	Resolution     string              `json:"resolution,omitempty"`
	LastSeen       time.Time           `json:"last_seen"`
}

// TimeMachineClient Time Machine 客户端信息
type TimeMachineClient struct {
	ClientID    string    `json:"client_id"`
	Hostname    string    `json:"hostname"`
	IPAddress   string    `json:"ip_address"`
	LastBackup  time.Time `json:"last_backup,omitempty"`
	BackupSize  int64     `json:"backup_size"`
	Status      string    `json:"status"`
}

// TimeMachineShare Time Machine 共享配置
type TimeMachineShare struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Path         string              `json:"path"`
	Quota        int64               `json:"quota"`
	UsedSpace    int64               `json:"used_space"`
	AFPEnabled   bool                `json:"afp_enabled"`
	SMBEnabled   bool                `json:"smb_enabled"`
	EncryptionKey string             `json:"encryption_key,omitempty"`
	Clients      []TimeMachineClient `json:"clients,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`
}

// SpotlightIndex Spotlight 索引状态
type SpotlightIndex struct {
	ID          string              `json:"id"`
	VolumeID    string              `json:"volume_id"`
	IndexPath   string              `json:"index_path"`
	Status      SpotlightIndexStatus `json:"status"`
	LastIndexed time.Time           `json:"last_indexed,omitempty"`
	TotalFiles  int64               `json:"total_files"`
	IndexSize   int64               `json:"index_size"`
	Progress    float64             `json:"progress"`
	UpdatedAt   time.Time           `json:"updated_at"`
}

// SMBConfig Apple 优化的 SMB 配置
type SMBConfig struct {
	Signing            bool `json:"signing"`
	AAPLExtensions     bool `json:"aapl_extensions"`
	Streams            bool `json:"streams"`
	VFSFruitEnabled    bool `json:"vfs_fruit_enabled"`
	SpotlightEnabled   bool `json:"spotlight_enabled"`
}

// CreateTimeMachineShareRequest 创建 Time Machine 共享请求
type CreateTimeMachineShareRequest struct {
	Name  string `json:"name" binding:"required"`
	Path  string `json:"path" binding:"required"`
	Quota int64  `json:"quota"`
}

// UpdateSMBConfigRequest 更新 SMB 配置请求
type UpdateSMBConfigRequest struct {
	Signing            bool `json:"signing"`
	AAPLExtensions     bool `json:"aapl_extensions"`
	Streams            bool `json:"streams"`
	VFSFruitEnabled    bool `json:"vfs_fruit_enabled"`
	SpotlightEnabled   bool `json:"spotlight_enabled"`
}

// AppleShareConfig Apple 共享模块配置
type AppleShareConfig struct {
	Enabled            bool   `json:"enabled"`
	MDNSInterface      string `json:"mdns_interface,omitempty"`
	MDNSDomain         string `json:"mdns_domain,omitempty"`
	SMBConfigPath      string `json:"smb_config_path,omitempty"`
	SpotlightIndexPath string `json:"spotlight_index_path,omitempty"`
	TimeMachinePath    string `json:"time_machine_path,omitempty"`
	DiscoveryTimeout   int    `json:"discovery_timeout"`
	MaxDevices         int    `json:"max_devices"`
}

// DefaultAppleShareConfig 默认配置
func DefaultAppleShareConfig() *AppleShareConfig {
	return &AppleShareConfig{
		Enabled:            true,
		MDNSInterface:      "eth0",
		MDNSDomain:         "local.",
		SMBConfigPath:      "/etc/samba/smb.conf",
		SpotlightIndexPath: "/var/lib/spotlight",
		TimeMachinePath:    "/srv/timemachine",
		DiscoveryTimeout:   5,
		MaxDevices:         50,
	}
}
