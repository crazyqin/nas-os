package smartlink

import (
	"sync"
	"time"
)

// SharePermission defines the permission level of a share link.
type SharePermission string

const (
	PermissionReadOnly  SharePermission = "read_only"
	PermissionReadWrite SharePermission = "read_write"
	PermissionPreview   SharePermission = "preview"
)

// ShareLink represents a smart share link.
type ShareLink struct {
	ID          string          `json:"id"`
	Token       string          `json:"token"`
	FileID      string          `json:"file_id"`
	CreatorID   string          `json:"creator_id"`
	Permission  SharePermission `json:"permission"`
	Password    string          `json:"password,omitempty"`
	MaxVisits   int             `json:"max_visits,omitempty"`
	VisitCount  int             `json:"visit_count"`
	ExpiresAt   *time.Time      `json:"expires_at,omitempty"`
	IsOneTime   bool            `json:"is_one_time"`
	IsActive    bool            `json:"is_active"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	Description string          `json:"description,omitempty"`
	mu          sync.RWMutex    `json:"-"`
}

// IsExpired checks if the link has expired.
func (sl *ShareLink) IsExpired() bool {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	if sl.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*sl.ExpiresAt)
}

// IsMaxVisitsReached checks if max visits limit is reached.
func (sl *ShareLink) IsMaxVisitsReached() bool {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	if sl.MaxVisits <= 0 {
		return false
	}
	return sl.VisitCount >= sl.MaxVisits
}

// IsAccessible checks if the link can be accessed.
func (sl *ShareLink) IsAccessible() bool {
	sl.mu.RLock()
	defer sl.mu.RUnlock()

	return sl.IsActive && !sl.IsExpired() && !sl.IsMaxVisitsReached()
}

// IncrementVisit increments visit count and deactivates if one-time.
func (sl *ShareLink) IncrementVisit() {
	sl.mu.Lock()
	defer sl.mu.Unlock()

	sl.VisitCount++
	sl.UpdatedAt = time.Now()

	if sl.IsOneTime {
		sl.IsActive = false
	}
}

// AccessLog represents an access log entry.
type AccessLog struct {
	ID         string    `json:"id"`
	LinkID     string    `json:"link_id"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent"`
	AccessedAt time.Time `json:"accessed_at"`
	Success    bool      `json:"success"`
	Reason     string    `json:"reason,omitempty"`
}

// SharePolicy defines sharing policy constraints.
type SharePolicy struct {
	MaxLinksPerUser   int           `json:"max_links_per_user"`
	DefaultExpiration time.Duration `json:"default_expiration"`
	MaxExpiration     time.Duration `json:"max_expiration"`
	AllowPassword     bool          `json:"allow_password"`
	AllowOneTime      bool          `json:"allow_one_time"`
	MaxVisitsPerLink  int           `json:"max_visits_per_link"`
}

// DefaultSharePolicy returns default sharing policy.
func DefaultSharePolicy() SharePolicy {
	return SharePolicy{
		MaxLinksPerUser:   100,
		DefaultExpiration: 7 * 24 * time.Hour,
		MaxExpiration:     30 * 24 * time.Hour,
		AllowPassword:     true,
		AllowOneTime:      true,
		MaxVisitsPerLink:  10000,
	}
}

// LinkStats represents statistics for a share link.
type LinkStats struct {
	LinkID          string     `json:"link_id"`
	TotalVisits     int        `json:"total_visits"`
	UniqueVisitors  int        `json:"unique_visitors"`
	LastAccessedAt  *time.Time `json:"last_accessed_at,omitempty"`
	FirstAccessedAt *time.Time `json:"first_accessed_at,omitempty"`
	IPAddresses     []string   `json:"ip_addresses,omitempty"`
}

// CreateLinkRequest represents request to create a share link.
type CreateLinkRequest struct {
	FileID      string          `json:"file_id" binding:"required"`
	Permission  SharePermission `json:"permission" binding:"required"`
	Password    string          `json:"password,omitempty"`
	MaxVisits   int             `json:"max_visits,omitempty"`
	ExpiresIn   *int            `json:"expires_in,omitempty"` // seconds
	IsOneTime   bool            `json:"is_one_time"`
	Description string          `json:"description,omitempty"`
}

// AccessLinkRequest represents request to access a share link.
type AccessLinkRequest struct {
	Password string `json:"password,omitempty"`
}

// LinkResponse represents share link response.
type LinkResponse struct {
	ID          string          `json:"id"`
	Token       string          `json:"token"`
	FileID      string          `json:"file_id"`
	Permission  SharePermission `json:"permission"`
	MaxVisits   int             `json:"max_visits,omitempty"`
	VisitCount  int             `json:"visit_count"`
	ExpiresAt   *time.Time      `json:"expires_at,omitempty"`
	IsOneTime   bool            `json:"is_one_time"`
	IsActive    bool            `json:"is_active"`
	CreatedAt   time.Time       `json:"created_at"`
	Description string          `json:"description,omitempty"`
	URL         string          `json:"url"`
}

// BatchCreateRequest represents batch creation request.
type BatchCreateRequest struct {
	Links []CreateLinkRequest `json:"links" binding:"required,min=1,max=50"`
}

// BatchCreateResponse represents batch creation response.
type BatchCreateResponse struct {
	Links   []LinkResponse `json:"links"`
	Success int            `json:"success"`
	Failed  int            `json:"failed"`
	Errors  []string       `json:"errors,omitempty"`
}
