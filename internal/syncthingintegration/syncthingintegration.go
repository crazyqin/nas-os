// Package syncthingintegration 提供 Syncthing 集成
// 对标 TrueNAS Syncthing 集成，实现点对点文件同步
package syncthingintegration

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// ========== Syncthing 设备管理 ==========

// SyncthingDevice Syncthing 设备
type SyncthingDevice struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	DeviceID        string            `json:"device_id"` // Syncthing 设备 ID
	Addresses       []string          `json:"addresses"`
	Compression     CompressionMode   `json:"compression"`
	Introducer      bool              `json:"introducer"`
	SkipIntroductionRemovals bool     `json:"skip_introduction_removals"`
	AutoAcceptFolders bool            `json:"auto_accept_folders"`
	MaxSendKbps     int               `json:"max_send_kbps"`
	MaxRecvKbps     int               `json:"max_recv_kbps"`
	Paused          bool              `json:"paused"`
	Stats           DeviceStats       `json:"stats"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	LastSeen        time.Time         `json:"last_seen"`
	CreatedAt       time.Time         `json:"created_at"`
}

// CompressionMode 压缩模式
type CompressionMode string

const (
	CompressionAlways  CompressionMode = "always"
	CompressionNever   CompressionMode = "never"
	CompressionMetadata CompressionMode = "metadata"
)

// DeviceStats 设备统计
type DeviceStats struct {
	BytesSent       int64     `json:"bytes_sent"`
	BytesReceived   int64     `json:"bytes_received"`
	FilesSent       int64     `json:"files_sent"`
	FilesReceived   int64     `json:"files_received"`
	Folders         int       `json:"folders"`
	LastSyncPercent float64   `json:"last_sync_percent"`
	LastSyncTime    time.Time `json:"last_sync_time"`
	ConnectionState string    `json:"connection_state"`
	Address         string    `json:"address"`
}

// ========== 同步文件夹管理 ==========

// SyncFolder 同步文件夹
type SyncFolder struct {
	ID                 string            `json:"id"`
	Label              string            `json:"label"`
	Path               string            `json:"path"`
	Type               FolderType        `json:"type"`
	Devices            []FolderDevice    `json:"devices"`
	RescanIntervalS    int               `json:"rescan_interval_s"`
	FSWatcherEnabled   bool              `json:"fs_watcher_enabled"`
	FSWatcherDelayS    int               `json:"fs_watcher_delay_s"`
	IgnorePerms        bool              `json:"ignore_perms"`
	AutoNormalize      bool              `json:"auto_normalize"`
	MinDiskFree        FolderMinDiskFree `json:"min_disk_free"`
	Versioning         FolderVersioning  `json:"versioning"`
	Copiers            int               `json:"copiers"`
	PullerMaxPendingKiB int              `json:"puller_max_pending_kib"`
	Hashers            int               `json:"hashers"`
	Order              PullOrder         `json:"order"`
	IgnoreDelete       bool              `json:"ignore_delete"`
	ScanProgressIntervalS int            `json:"scan_progress_interval_s"`
	PullerPauseS       int               `json:"puller_pause_s"`
	MaxConflicts       int               `json:"max_conflicts"`
	DisableSparseFiles bool             `json:"disable_sparse_files"`
	DisableTempIndexes bool             `json:"disable_temp_indexes"`
	Paused             bool              `json:"paused"`
	Stats              FolderStats       `json:"stats"`
	Metadata           map[string]string `json:"metadata,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

// FolderType 文件夹类型
type FolderType string

const (
	FolderTypeSendReceive FolderType = "sendreceive"
	FolderTypeSendOnly    FolderType = "sendonly"
	FolderTypeReceiveOnly FolderType = "receiveonly"
)

// PullOrder 拉取顺序
type PullOrder string

const (
	PullOrderRandom     PullOrder = "random"
	PullOrderAlphabetic PullOrder = "alphabetic"
	PullOrderSmallest   PullOrder = "smallestFirst"
	PullOrderLargest    PullOrder = "largestFirst"
	PullOrderOldest     PullOrder = "oldestFirst"
	PullOrderNewest     PullOrder = "newestFirst"
)

// FolderDevice 文件夹关联设备
type FolderDevice struct {
	DeviceID              string `json:"device_id"`
	IntroducedBy          string `json:"introduced_by"`
	EncryptionPassword    string `json:"encryption_password"`
	SkipIntroductionRemovals bool `json:"skip_introduction_removals"`
}

// FolderMinDiskFree 文件夹最小磁盘空间
type FolderMinDiskFree struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"` // %, kB, MB, GB, TB
}

// FolderVersioning 文件夹版本控制
type FolderVersioning struct {
	Type             string                 `json:"type"` // simple, staggered, external, trashcan
	Params           map[string]string      `json:"params"`
	CleanupIntervalS int                    `json:"cleanup_interval_s"`
}

// FolderStats 文件夹统计
type FolderStats struct {
	Files         int       `json:"files"`
	Directories   int       `json:"directories"`
	Symlinks      int       `json:"symlinks"`
	Bytes         int64     `json:"bytes"`
	DeletedFiles  int       `json:"deleted_files"`
	LastScanTime  time.Time `json:"last_scan_time"`
	LastScanDurationS float64 `json:"last_scan_duration_s"`
	NeedFiles     int       `json:"need_files"`
	NeedBytes     int64     `json:"need_bytes"`
	Sequence      int64     `json:"sequence"`
}

// ========== 同步管理器 ==========

// SyncthingManager Syncthing 管理器
type SyncthingManager struct {
	mu        sync.RWMutex
	devices   map[string]*SyncthingDevice
	folders   map[string]*SyncFolder
	config    SyncthingConfig
	stats     ManagerStats
	apiClient *APIClient
}

// SyncthingConfig Syncthing 配置
type SyncthingConfig struct {
	APIKey           string `json:"api_key"`
	APIURL           string `json:"api_url"`
	GUIAddress       string `json:"gui_address"`
	ListenAddress    string `json:"listen_address"`
	MaxSendKbps      int    `json:"max_send_kbps"`
	MaxRecvKbps      int    `json:"max_recv_kbps"`
	NATEnabled       bool   `json:"nat_enabled"`
	NATLeaseM        int    `json:"nat_lease_m"`
	NATRenewalM      int    `json:"nat_renewal_m"`
	NATTimeoutS      int    `json:"nat_timeout_s"`
	RelayEnabled     bool   `json:"relay_enabled"`
	RelayServers     []string `json:"relay_servers"`
	GlobalAnnounceEnabled bool `json:"global_announce_enabled"`
	LocalAnnounceEnabled  bool `json:"local_announce_enabled"`
	LocalAnnouncePort     int  `json:"local_announce_port"`
	AutoAcceptIncomming   bool `json:"auto_accept_incoming"`
	DefaultFolderPath     string `json:"default_folder_path"`
	TempIndexMinMessageSize int `json:"temp_index_min_message"`
}

// ManagerStats 管理器统计
type ManagerStats struct {
	TotalDevices    int       `json:"total_devices"`
	OnlineDevices   int       `json:"online_devices"`
	TotalFolders    int       `json:"total_folders"`
	SyncingFolders  int       `json:"syncing_folders"`
	TotalFiles      int       `json:"total_files"`
	TotalBytes      int64     `json:"total_bytes"`
	NeedFiles       int       `json:"need_files"`
	NeedBytes       int64     `json:"need_bytes"`
	LastSyncTime    time.Time `json:"last_sync_time"`
	Connections     int       `json:"connections"`
}

// APIClient API 客户端
type APIClient struct {
	baseURL string
	apiKey  string
}

// NewSyncthingManager 创建 Syncthing 管理器
func NewSyncthingManager(config SyncthingConfig) *SyncthingManager {
	// 设置默认值
	if config.GUIAddress == "" {
		config.GUIAddress = "127.0.0.1:8384"
	}
	if config.ListenAddress == "" {
		config.ListenAddress = "tcp://0.0.0.0:22000"
	}
	if config.NATLeaseM == 0 {
		config.NATLeaseM = 60
	}
	if config.NATRenewalM == 0 {
		config.NATRenewalM = 30
	}
	if config.NATTimeoutS == 0 {
		config.NATTimeoutS = 10
	}
	if config.LocalAnnouncePort == 0 {
		config.LocalAnnouncePort = 21027
	}
	if config.DefaultFolderPath == "" {
		config.DefaultFolderPath = "~/Sync"
	}
	if config.TempIndexMinMessageSize == 0 {
		config.TempIndexMinMessageSize = 1048576 // 1MB
	}

	return &SyncthingManager{
		devices: make(map[string]*SyncthingDevice),
		folders: make(map[string]*SyncFolder),
		config:  config,
		apiClient: &APIClient{
			baseURL: config.APIURL,
			apiKey:  config.APIKey,
		},
	}
}

// ========== 设备管理 ==========

// AddDevice 添加设备
func (m *SyncthingManager) AddDevice(device SyncthingDevice) (*SyncthingDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device.ID == "" {
		device.ID = fmt.Sprintf("device-%s-%d", device.Name, time.Now().UnixNano())
	}

	if _, exists := m.devices[device.ID]; exists {
		return nil, fmt.Errorf("设备已存在: %s", device.ID)
	}

	device.CreatedAt = time.Now()
	device.LastSeen = time.Now()

	m.devices[device.ID] = &device
	m.updateStats()

	return &device, nil
}

// RemoveDevice 移除设备
func (m *SyncthingManager) RemoveDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.devices[id]; !exists {
		return fmt.Errorf("设备不存在: %s", id)
	}

	// 从所有文件夹中移除设备关联
	for _, folder := range m.folders {
		newDevices := make([]FolderDevice, 0)
		for _, dev := range folder.Devices {
			if dev.DeviceID != id {
				newDevices = append(newDevices, dev)
			}
		}
		folder.Devices = newDevices
	}

	delete(m.devices, id)
	m.updateStats()

	return nil
}

// GetDevice 获取设备
func (m *SyncthingManager) GetDevice(id string) (*SyncthingDevice, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	device, exists := m.devices[id]
	if !exists {
		return nil, fmt.Errorf("设备不存在: %s", id)
	}

	return device, nil
}

// ListDevices 列出所有设备
func (m *SyncthingManager) ListDevices() []*SyncthingDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SyncthingDevice, 0, len(m.devices))
	for _, d := range m.devices {
		result = append(result, d)
	}

	return result
}

// ========== 文件夹管理 ==========

// CreateFolder 创建同步文件夹
func (m *SyncthingManager) CreateFolder(folder SyncFolder) (*SyncFolder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if folder.ID == "" {
		folder.ID = fmt.Sprintf("folder-%s-%d", folder.Label, time.Now().UnixNano())
	}

	if _, exists := m.folders[folder.ID]; exists {
		return nil, fmt.Errorf("文件夹已存在: %s", folder.ID)
	}

	// 设置默认值
	if folder.RescanIntervalS == 0 {
		folder.RescanIntervalS = 3600 // 1小时
	}
	if folder.FSWatcherDelayS == 0 {
		folder.FSWatcherDelayS = 10
	}
	if folder.Copiers == 0 {
		folder.Copiers = 1
	}
	if folder.PullerMaxPendingKiB == 0 {
		folder.PullerMaxPendingKiB = 0 // 无限制
	}
	if folder.Hashers == 0 {
		folder.Hashers = 0 // 自动
	}
	if folder.Order == "" {
		folder.Order = PullOrderRandom
	}
	if folder.MaxConflicts == 0 {
		folder.MaxConflicts = 10
	}
	if folder.Type == "" {
		folder.Type = FolderTypeSendReceive
	}
	if folder.MinDiskFree.Value == 0 {
		folder.MinDiskFree.Value = 1
		folder.MinDiskFree.Unit = "%"
	}

	folder.CreatedAt = time.Now()
	folder.UpdatedAt = time.Now()

	m.folders[folder.ID] = &folder
	m.updateStats()

	return &folder, nil
}

// UpdateFolder 更新文件夹
func (m *SyncthingManager) UpdateFolder(id string, folder SyncFolder) (*SyncFolder, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.folders[id]
	if !exists {
		return nil, fmt.Errorf("文件夹不存在: %s", id)
	}

	folder.ID = id
	folder.CreatedAt = existing.CreatedAt
	folder.UpdatedAt = time.Now()

	m.folders[id] = &folder

	return &folder, nil
}

// DeleteFolder 删除文件夹
func (m *SyncthingManager) DeleteFolder(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.folders[id]; !exists {
		return fmt.Errorf("文件夹不存在: %s", id)
	}

	delete(m.folders, id)
	m.updateStats()

	return nil
}

// GetFolder 获取文件夹
func (m *SyncthingManager) GetFolder(id string) (*SyncFolder, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	folder, exists := m.folders[id]
	if !exists {
		return nil, fmt.Errorf("文件夹不存在: %s", id)
	}

	return folder, nil
}

// ListFolders 列出所有文件夹
func (m *SyncthingManager) ListFolders() []*SyncFolder {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*SyncFolder, 0, len(m.folders))
	for _, f := range m.folders {
		result = append(result, f)
	}

	return result
}

// ========== 同步操作 ==========

// ScanFolder 扫描文件夹
func (m *SyncthingManager) ScanFolder(folderID string) (*ScanResult, error) {
	m.mu.RLock()
	folder, exists := m.folders[folderID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("文件夹不存在: %s", folderID)
	}

	startTime := time.Now()

	// 模拟扫描操作
	result := &ScanResult{
		FolderID:  folderID,
		Path:      folder.Path,
		StartTime: startTime,
		Stats: ScanStats{
			Files:       folder.Stats.Files,
			Directories: folder.Stats.Directories,
			Bytes:       folder.Stats.Bytes,
		},
	}

	endTime := time.Now()
	result.EndTime = endTime
	result.DurationS = endTime.Sub(startTime).Seconds()

	// 更新文件夹统计
	m.mu.Lock()
	folder.Stats.LastScanTime = endTime
	folder.Stats.LastScanDurationS = result.DurationS
	m.mu.Unlock()

	return result, nil
}

// ScanResult 扫描结果
type ScanResult struct {
	FolderID  string    `json:"folder_id"`
	Path      string    `json:"path"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	DurationS float64   `json:"duration_s"`
	Stats     ScanStats `json:"stats"`
	Errors    []string  `json:"errors,omitempty"`
}

// ScanStats 扫描统计
type ScanStats struct {
	Files       int   `json:"files"`
	Directories int   `json:"directories"`
	Symlinks    int   `json:"symlinks"`
	Bytes       int64 `json:"bytes"`
}

// PauseFolder 暂停文件夹
func (m *SyncthingManager) PauseFolder(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, exists := m.folders[id]
	if !exists {
		return fmt.Errorf("文件夹不存在: %s", id)
	}

	folder.Paused = true
	folder.UpdatedAt = time.Now()

	return nil
}

// ResumeFolder 恢复文件夹
func (m *SyncthingManager) ResumeFolder(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, exists := m.folders[id]
	if !exists {
		return fmt.Errorf("文件夹不存在: %s", id)
	}

	folder.Paused = false
	folder.UpdatedAt = time.Now()

	return nil
}

// ========== 版本控制 ==========

// SetupVersioning 设置版本控制
func (m *SyncthingManager) SetupVersioning(folderID string, versioning FolderVersioning) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	folder, exists := m.folders[folderID]
	if !exists {
		return fmt.Errorf("文件夹不存在: %s", folderID)
	}

	if versioning.Params == nil {
		versioning.Params = make(map[string]string)
	}

	folder.Versioning = versioning
	folder.UpdatedAt = time.Now()

	return nil
}

// GetVersions 获取文件版本
func (m *SyncthingManager) GetVersions(folderID, filePath string) ([]FileVersion, error) {
	m.mu.RLock()
	_, exists := m.folders[folderID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("文件夹不存在: %s", folderID)
	}

	// 模拟返回版本列表
	versions := []FileVersion{
		{
			Version:  1,
			Modified: time.Now().Add(-24 * time.Hour),
			Size:     1024,
			Path:     filePath,
		},
	}

	return versions, nil
}

// FileVersion 文件版本
type FileVersion struct {
	Version  int       `json:"version"`
	Modified time.Time `json:"modified"`
	Size     int64     `json:"size"`
	Path     string    `json:"path"`
}

// RestoreVersion 恢复版本
func (m *SyncthingManager) RestoreVersion(folderID, filePath string, version int) error {
	m.mu.RLock()
	_, exists := m.folders[folderID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("文件夹不存在: %s", folderID)
	}

	// 模拟恢复操作
	return nil
}

// ========== 辅助方法 ==========

// updateStats 更新统计
func (m *SyncthingManager) updateStats() {
	m.stats.TotalDevices = len(m.devices)
	m.stats.OnlineDevices = 0
	m.stats.TotalFolders = len(m.folders)
	m.stats.SyncingFolders = 0
	m.stats.TotalFiles = 0
	m.stats.TotalBytes = 0
	m.stats.NeedFiles = 0
	m.stats.NeedBytes = 0

	for _, d := range m.devices {
		if !d.Paused && d.Stats.ConnectionState == "connected" {
			m.stats.OnlineDevices++
		}
	}

	for _, f := range m.folders {
		if !f.Paused {
			m.stats.SyncingFolders++
		}
		m.stats.TotalFiles += f.Stats.Files
		m.stats.TotalBytes += f.Stats.Bytes
		m.stats.NeedFiles += f.Stats.NeedFiles
		m.stats.NeedBytes += f.Stats.NeedBytes
	}
}

// GetStats 获取统计
func (m *SyncthingManager) GetStats() ManagerStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// SaveConfig 保存配置
func (m *SyncthingManager) SaveConfig(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0640)
}

// LoadConfig 加载配置
func (m *SyncthingManager) LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return json.Unmarshal(data, &m.config)
}
