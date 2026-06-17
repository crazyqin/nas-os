package websharepro2

import (
	"sync"
	"time"
)

// ShareType 分享类型。
type ShareType string

const (
	ShareTypeLink    ShareType = "link"     // 链接分享
	ShareTypeUser    ShareType = "user"     // 指定用户
	ShareTypeGroup   ShareType = "group"    // 用户组
	ShareTypePublic  ShareType = "public"   // 公开分享
)

// Permission 分享权限。
type Permission string

const (
	PermView   Permission = "view"
	PermDownload Permission = "download"
	PermEdit   Permission = "edit"
	PermAdmin  Permission = "admin"
)

// Share 分享记录。
type Share struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	FilePath    string       `json:"file_path"`
	Type        ShareType    `json:"type"`
	Permission  Permission   `json:"permission"`
	Token       string       `json:"token"`
	CreatorID   string       `json:"creator_id"`
	ExpiresAt   *time.Time   `json:"expires_at,omitempty"`
	Password    string       `json:"password,omitempty"`
	MaxDownloads int         `json:"max_downloads"`
	DownloadCount int        `json:"download_count"`
	AllowUpload  bool        `json:"allow_upload"`
	Enabled     bool         `json:"enabled"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// ShareAccess 分享访问记录。
type ShareAccess struct {
	ID        string    `json:"id"`
	ShareID   string    `json:"share_id"`
	UserID    string    `json:"user_id"`
	IP        string    `json:"ip"`
	Action    string    `json:"action"` // view, download, upload
	Timestamp time.Time `json:"timestamp"`
}

// ShareStats 分享统计。
type ShareStats struct {
	TotalShares    int64 `json:"total_shares"`
	ActiveShares   int64 `json:"active_shares"`
	TotalDownloads int64 `json:"total_downloads"`
	TotalViews     int64 `json:"total_views"`
	StorageUsedMB  int64 `json:"storage_used_mb"`
}

// Collaboration 协作会话。
type Collaboration struct {
	ID        string    `json:"id"`
	ShareID   string    `json:"share_id"`
	Users     []string  `json:"users"`
	StartedAt time.Time `json:"started_at"`
	Status    string    `json:"status"` // active, ended
}

// Engine WebShare Pro 引擎。
type Engine struct {
	mu           sync.RWMutex
	shares       map[string]*Share
	accessLog    []ShareAccess
	collabs      map[string]*Collaboration
	stats        ShareStats
}

// NewEngine 创建新的引擎。
func NewEngine() *Engine {
	return &Engine{
		shares:    make(map[string]*Share),
		collabs:   make(map[string]*Collaboration),
	}
}

// CreateShare 创建分享。
func (e *Engine) CreateShare(share *Share) {
	e.mu.Lock()
	defer e.mu.Unlock()

	share.Token = generateToken()
	share.CreatedAt = time.Now()
	share.UpdatedAt = time.Now()
	share.Enabled = true
	e.shares[share.ID] = share
	e.stats.TotalShares++
	e.stats.ActiveShares++
}

// GetShare 获取分享。
func (e *Engine) GetShare(id string) (*Share, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	share, exists := e.shares[id]
	return share, exists
}

// GetShareByToken 通过Token获取分享。
func (e *Engine) GetShareByToken(token string) (*Share, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, share := range e.shares {
		if share.Token == token && share.Enabled {
			return share, true
		}
	}
	return nil, false
}

// ListShares 列出所有分享。
func (e *Engine) ListShares() []*Share {
	e.mu.RLock()
	defer e.mu.RUnlock()
	result := make([]*Share, 0, len(e.shares))
	for _, s := range e.shares {
		result = append(result, s)
	}
	return result
}

// UpdateShare 更新分享。
func (e *Engine) UpdateShare(id string, updates *Share) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	share, exists := e.shares[id]
	if !exists {
		return ErrShareNotFound
	}
	if updates.Name != "" {
		share.Name = updates.Name
	}
	if updates.Permission != "" {
		share.Permission = updates.Permission
	}
	if updates.ExpiresAt != nil {
		share.ExpiresAt = updates.ExpiresAt
	}
	share.UpdatedAt = time.Now()
	return nil
}

// DeleteShare 删除分享。
func (e *Engine) DeleteShare(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.shares[id]; !exists {
		return ErrShareNotFound
	}
	delete(e.shares, id)
	e.stats.ActiveShares--
	return nil
}

// RecordAccess 记录访问。
func (e *Engine) RecordAccess(access ShareAccess) {
	e.mu.Lock()
	defer e.mu.Unlock()

	access.Timestamp = time.Now()
	e.accessLog = append(e.accessLog, access)

	if share, ok := e.shares[access.ShareID]; ok {
		if access.Action == "download" {
			share.DownloadCount++
			e.stats.TotalDownloads++
		} else if access.Action == "view" {
			e.stats.TotalViews++
		}
	}
}

// StartCollaboration 开始协作。
func (e *Engine) StartCollaboration(shareID string, users []string) *Collaboration {
	e.mu.Lock()
	defer e.mu.Unlock()

	collab := &Collaboration{
		ID:        generateToken(),
		ShareID:   shareID,
		Users:     users,
		StartedAt: time.Now(),
		Status:    "active",
	}
	e.collabs[collab.ID] = collab
	return collab
}

// GetStats 获取统计。
func (e *Engine) GetStats() ShareStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// GetAccessLog 获取访问日志。
func (e *Engine) GetAccessLog(shareID string) []ShareAccess {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []ShareAccess
	for _, a := range e.accessLog {
		if a.ShareID == shareID {
			result = append(result, a)
		}
	}
	return result
}

func generateToken() string {
	return time.Now().Format("20060102150405") + "token"
}

// 错误定义。
var (
	ErrShareNotFound = &ShareError{"share not found"}
)

type ShareError struct {
	msg string
}

func (e *ShareError) Error() string {
	return e.msg
}
