// Package cloudmount 提供云存储挂载管理功能
// 支持多种云存储后端、缓存策略、传输限速、同步状态监控
package cloudmount

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// CloudProvider 云存储提供商
type CloudProvider string

const (
	ProviderS3         CloudProvider = "s3"
	ProviderOSS        CloudProvider = "oss"
	ProviderOneDrive   CloudProvider = "onedrive"
	ProviderGoogleDrive CloudProvider = "google_drive"
	ProviderDropbox    CloudProvider = "dropbox"
	ProviderSFTP       CloudProvider = "sftp"
)

// MountStatus 挂载状态
type MountStatus string

const (
	StatusMounted    MountStatus = "mounted"
	StatusUnmounted  MountStatus = "unmounted"
	StatusError      MountStatus = "error"
	StatusSyncing    MountStatus = "syncing"
)

// CacheStrategy 缓存策略
type CacheStrategy string

const (
	CacheNone     CacheStrategy = "none"      // 不缓存
	CacheMetadata CacheStrategy = "metadata"  // 只缓存元数据
	CacheAll      CacheStrategy = "all"       // 缓存元数据和文件内容
)

// MountOptions 挂载选项
type MountOptions struct {
	ReadOnly      bool          `json:"readOnly"`      // 只读模式
	CacheSize     int64         `json:"cacheSize"`     // 缓存大小 (MB)
	SpeedLimit    int64         `json:"speedLimit"`    // 限速 (KB/s, 0=不限)
	Prefetch      bool          `json:"prefetch"`      // 文件预取
	CacheStrategy CacheStrategy `json:"cacheStrategy"` // 缓存策略
	OfflineAccess bool          `json:"offlineAccess"` // 离线访问
}

// MountPoint 挂载点
type MountPoint struct {
	ID         string        `json:"id"`
	Name       string        `json:"name"`
	Provider   CloudProvider `json:"provider"`
	Bucket     string        `json:"bucket"`     // bucket/container 名称
	LocalPath  string        `json:"localPath"`  // 本地挂载路径
	Status     MountStatus   `json:"status"`
	Options    MountOptions  `json:"options"`
	AccountID  string        `json:"accountId"`  // 关联的账号ID
	CreatedAt  time.Time     `json:"createdAt"`
	MountedAt  time.Time     `json:"mountedAt,omitempty"`
	ErrorMsg  string        `json:"errorMsg,omitempty"`
}

// SyncStatus 同步状态
type SyncStatus struct {
	TotalFiles    int       `json:"totalFiles"`
	SyncedFiles   int       `json:"syncedFiles"`
	PendingFiles  int       `json:"pendingFiles"`
	ErrorFiles    int       `json:"errorFiles"`
	LastSyncTime  time.Time `json:"lastSyncTime"`
	SyncDirection string    `json:"syncDirection"` // up/down/bidirectional
}

// TransferStats 传输统计
type TransferStats struct {
	UploadSpeed   int64     `json:"uploadSpeed"`   // KB/s
	DownloadSpeed int64     `json:"downloadSpeed"` // KB/s
	TotalUpload   int64     `json:"totalUpload"`   // MB
	TotalDownload int64     `json:"totalDownload"` // MB
	ActiveTransfers int     `json:"activeTransfers"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// CloudAccount 云账号
type CloudAccount struct {
	ID        string        `json:"id"`
	Provider  CloudProvider `json:"provider"`
	Name      string        `json:"name"`
	Endpoint  string        `json:"endpoint,omitempty"` // S3/OSS/SFTP 用
	Region    string        `json:"region,omitempty"`   // S3/OSS 用
	Auth      AuthConfig    `json:"auth"`
	IsValid   bool          `json:"isValid"`
	CreatedAt time.Time     `json:"createdAt"`
}

// AuthConfig 认证配置
type AuthConfig struct {
	Type      string `json:"type"`      // key/token/oauth
	AccessKey string `json:"accessKey,omitempty"`
	SecretKey string `json:"secretKey,omitempty"`
	Token     string `json:"token,omitempty"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
}

// ========== Manager ==========

// Manager 云存储挂载管理器
type Manager struct {
	mu              sync.RWMutex
	accounts        map[string]*CloudAccount
	mounts          map[string]*MountPoint
	syncStatuses    map[string]*SyncStatus
	transferStats   *TransferStats
	uploadLimit     int64 // KB/s
	downloadLimit   int64 // KB/s
	nextID          int
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		accounts:      make(map[string]*CloudAccount),
		mounts:        make(map[string]*MountPoint),
		syncStatuses:  make(map[string]*SyncStatus),
		transferStats: &TransferStats{UpdatedAt: time.Now()},
	}
	return m
}

// generateID 生成ID
func (m *Manager) generateID(prefix string) string {
	m.nextID++
	return fmt.Sprintf("%s-%d", prefix, m.nextID)
}

// ========== 账号管理 ==========

// RegisterAccount 注册账号
func (m *Manager) RegisterAccount(account *CloudAccount) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if account.Name == "" {
		return fmt.Errorf("account name is required")
	}
	if account.Provider == "" {
		return fmt.Errorf("account provider is required")
	}

	account.ID = m.generateID("acc")
	account.IsValid = true
	account.CreatedAt = time.Now()

	m.accounts[account.ID] = account
	log.Printf("[云存储] 注册账号: %s (%s)", account.Name, account.Provider)
	return nil
}

// RemoveAccount 移除账号
func (m *Manager) RemoveAccount(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.accounts[id]; !ok {
		return fmt.Errorf("account %s not found", id)
	}

	// 检查是否有挂载点使用该账号
	for _, mount := range m.mounts {
		if mount.AccountID == id {
			return fmt.Errorf("account %s is in use by mount %s", id, mount.Name)
		}
	}

	delete(m.accounts, id)
	log.Printf("[云存储] 移除账号: %s", id)
	return nil
}

// ListAccounts 列出所有账号
func (m *Manager) ListAccounts() []CloudAccount {
	m.mu.RLock()
	defer m.mu.RUnlock()

	accounts := make([]CloudAccount, 0, len(m.accounts))
	for _, a := range m.accounts {
		accounts = append(accounts, *a)
	}
	return accounts
}

// ========== 挂载管理 ==========

// Mount 挂载云存储
func (m *Manager) Mount(point *MountPoint, opts *MountOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if point.Name == "" {
		return fmt.Errorf("mount name is required")
	}
	if point.Provider == "" {
		return fmt.Errorf("provider is required")
	}
	if point.Bucket == "" {
		return fmt.Errorf("bucket/container is required")
	}
	if point.LocalPath == "" {
		return fmt.Errorf("local path is required")
	}

	// 验证账号
	if point.AccountID != "" {
		acc, ok := m.accounts[point.AccountID]
		if !ok {
			return fmt.Errorf("account %s not found", point.AccountID)
		}
		if acc.Provider != point.Provider {
			return fmt.Errorf("account provider mismatch: expected %s, got %s", point.Provider, acc.Provider)
		}
	}

	point.ID = m.generateID("mnt")
	point.Status = StatusMounted
	point.CreatedAt = time.Now()
	point.MountedAt = time.Now()

	if opts != nil {
		point.Options = *opts
	} else {
		point.Options = MountOptions{
			ReadOnly:      false,
			CacheSize:     1024, // 1GB 默认缓存
			Prefetch:      true,
			CacheStrategy: CacheMetadata,
			OfflineAccess: false,
		}
	}

	m.mounts[point.ID] = point

	// 初始化同步状态
	m.syncStatuses[point.ID] = &SyncStatus{
		TotalFiles:    0,
		SyncedFiles:   0,
		PendingFiles:  0,
		ErrorFiles:    0,
		LastSyncTime:  time.Now(),
		SyncDirection: "bidirectional",
	}

	log.Printf("[云存储] 挂载: %s -> %s (%s)", point.Name, point.LocalPath, point.Provider)
	return nil
}

// Unmount 卸载云存储
func (m *Manager) Unmount(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mount, ok := m.mounts[id]
	if !ok {
		return fmt.Errorf("mount %s not found", id)
	}

	if mount.Status == StatusUnmounted {
		return fmt.Errorf("mount %s is already unmounted", id)
	}

	mount.Status = StatusUnmounted
	mount.MountedAt = time.Time{}
	log.Printf("[云存储] 卸载: %s", mount.Name)
	return nil
}

// GetMountStatus 获取挂载状态
func (m *Manager) GetMountStatus(id string) MountStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mount, ok := m.mounts[id]
	if !ok {
		return StatusUnmounted
	}
	return mount.Status
}

// ListMounts 列出所有挂载点
func (m *Manager) ListMounts() []MountPoint {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mounts := make([]MountPoint, 0, len(m.mounts))
	for _, mp := range m.mounts {
		mounts = append(mounts, *mp)
	}
	return mounts
}

// ========== 同步管理 ==========

// GetSyncStatus 获取同步状态
func (m *Manager) GetSyncStatus(id string) (*SyncStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.mounts[id]; !ok {
		return nil, fmt.Errorf("mount %s not found", id)
	}

	status, ok := m.syncStatuses[id]
	if !ok {
		return &SyncStatus{LastSyncTime: time.Now()}, nil
	}

	// 模拟更新
	status.LastSyncTime = time.Now()
	return status, nil
}

// GetTransferStats 获取传输统计
func (m *Manager) GetTransferStats() *TransferStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.transferStats
	stats.UpdatedAt = time.Now()

	// 计算活跃传输数
	active := 0
	for _, s := range m.syncStatuses {
		if s.PendingFiles > 0 {
			active++
		}
	}
	stats.ActiveTransfers = active

	return &stats
}

// SetSpeedLimit 设置传输限速
func (m *Manager) SetSpeedLimit(upload, download int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if upload < 0 || download < 0 {
		return fmt.Errorf("speed limit cannot be negative")
	}

	m.uploadLimit = upload
	m.downloadLimit = download
	log.Printf("[云存储] 设置限速: 上传=%dKB/s, 下载=%dKB/s", upload, download)
	return nil
}

// FlushCache 刷新缓存
func (m *Manager) FlushCache(mountID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mount, ok := m.mounts[mountID]
	if !ok {
		return fmt.Errorf("mount %s not found", mountID)
	}

	if mount.Status != StatusMounted {
		return fmt.Errorf("mount %s is not mounted", mountID)
	}

	log.Printf("[云存储] 刷新缓存: %s", mount.Name)
	return nil
}
