// Package webshare2 implements an enhanced browser-based file sharing system
// inspired by TrueNAS WebShare. It provides Dropbox-like file sharing with
// FIPS 140 encrypted transport, cross-protocol support, and mobile-friendly UI.
package webshare2

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// SharePermission defines access levels for shared files
type SharePermission string

const (
	PermissionView     SharePermission = "view"
	PermissionDownload SharePermission = "download"
	PermissionEdit     SharePermission = "edit"
	PermissionAdmin    SharePermission = "admin"
)

// ShareLink represents a shared file/folder link
type ShareLink struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Path        string          `json:"path"`
	Token       string          `json:"token"`
	Permission  SharePermission `json:"permission"`
	Password    string          `json:"password,omitempty"`
	MaxDownloads int            `json:"max_downloads,omitempty"`
	DownloadCount int           `json:"download_count"`
	ExpiresAt   *time.Time      `json:"expires_at,omitempty"`
	CreatedBy   string          `json:"created_by"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	IsActive    bool            `json:"is_active"`
	Metadata    ShareMetadata   `json:"metadata"`
}

// ShareMetadata contains file metadata
type ShareMetadata struct {
	Size        int64     `json:"size"`
	MimeType    string    `json:"mime_type"`
	Checksum    string    `json:"checksum,omitempty"`
	Thumbnail   string    `json:"thumbnail,omitempty"`
	Previewable bool      `json:"previewable"`
	ModTime     time.Time `json:"mod_time"`
}

// AccessLog records access to shared links
type AccessLog struct {
	ID        string    `json:"id"`
	ShareID   string    `json:"share_id"`
	Action    string    `json:"action"` // view, download, upload
	IP        string    `json:"ip"`
	UserAgent string    `json:"user_agent"`
	Timestamp time.Time `json:"timestamp"`
}

// WebShareConfig configuration for the web share service
type WebShareConfig struct {
	BaseURL          string        `json:"base_url"`
	MaxFileSize      int64         `json:"max_file_size"`      // bytes
	DefaultExpiry    time.Duration `json:"default_expiry"`
	EnablePassword   bool          `json:"enable_password"`
	EnableEncryption bool          `json:"enable_encryption"`
	EnableThumbnails bool          `json:"enable_thumbnails"`
	MaxActiveLinks   int           `json:"max_active_links"`
	AllowedTypes     []string      `json:"allowed_types,omitempty"`
}

// WebShareService manages file sharing operations
type WebShareService struct {
	config    WebShareConfig
	links     map[string]*ShareLink
	accessLog []AccessLog
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewWebShareService creates a new web share service
func NewWebShareService(config WebShareConfig) *WebShareService {
	ctx, cancel := context.WithCancel(context.Background())

	if config.MaxFileSize == 0 {
		config.MaxFileSize = 10 * 1024 * 1024 * 1024 // 10GB default
	}
	if config.DefaultExpiry == 0 {
		config.DefaultExpiry = 7 * 24 * time.Hour // 7 days default
	}
	if config.MaxActiveLinks == 0 {
		config.MaxActiveLinks = 1000
	}

	return &WebShareService{
		config:    config,
		links:     make(map[string]*ShareLink),
		accessLog: make([]AccessLog, 0),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// Start begins the web share service
func (s *WebShareService) Start() error {
	log.Println("[WebShare2] Starting enhanced file sharing service")

	// Start cleanup goroutine
	go s.cleanupExpiredLinks()

	// Start stats collector
	go s.collectStats()

	log.Println("[WebShare2] Service started successfully")
	return nil
}

// Stop gracefully stops the service
func (s *WebShareService) Stop() error {
	s.cancel()
	log.Println("[WebShare2] Service stopped")
	return nil
}

// CreateShareLink creates a new share link
func (s *WebShareService) CreateShareLink(req CreateShareRequest) (*ShareLink, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check active link limit
	if len(s.links) >= s.config.MaxActiveLinks {
		return nil, fmt.Errorf("maximum active share links reached (%d)", s.config.MaxActiveLinks)
	}

	// Generate unique token
	token, err := generateToken(32)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	link := &ShareLink{
		ID:         generateID(),
		Name:       req.Name,
		Path:       req.Path,
		Token:      token,
		Permission: req.Permission,
		Password:   req.Password,
		MaxDownloads: req.MaxDownloads,
		CreatedBy:  req.CreatedBy,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		IsActive:   true,
		Metadata: ShareMetadata{
			Size:        req.Size,
			MimeType:    req.MimeType,
			Previewable: isPreviewable(req.MimeType),
			ModTime:     time.Now(),
		},
	}

	if req.Expiry > 0 {
		expiry := time.Now().Add(req.Expiry)
		link.ExpiresAt = &expiry
	} else if s.config.DefaultExpiry > 0 {
		expiry := time.Now().Add(s.config.DefaultExpiry)
		link.ExpiresAt = &expiry
	}

	s.links[link.ID] = link
	log.Printf("[WebShare2] Created share link: %s -> %s", link.ID, link.Path)
	return link, nil
}

// GetShareLink retrieves a share link by token
func (s *WebShareService) GetShareLink(token string) (*ShareLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, link := range s.links {
		if link.Token == token && link.IsActive {
			if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
				return nil, fmt.Errorf("share link has expired")
			}
			return link, nil
		}
	}
	return nil, fmt.Errorf("share link not found")
}

// DeleteShareLink deactivates a share link
func (s *WebShareService) DeleteShareLink(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	link, exists := s.links[id]
	if !exists {
		return fmt.Errorf("share link not found: %s", id)
	}

	link.IsActive = false
	link.UpdatedAt = time.Now()
	log.Printf("[WebShare2] Deactivated share link: %s", id)
	return nil
}

// ListShareLinks returns all share links for a user
func (s *WebShareService) ListShareLinks(createdBy string) []*ShareLink {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ShareLink
	for _, link := range s.links {
		if link.IsActive && (createdBy == "" || link.CreatedBy == createdBy) {
			result = append(result, link)
		}
	}
	return result
}

// RecordAccess logs an access event
func (s *WebShareService) RecordAccess(shareID, action, ip, userAgent string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log := AccessLog{
		ID:        generateID(),
		ShareID:   shareID,
		Action:    action,
		IP:        ip,
		UserAgent: userAgent,
		Timestamp: time.Now(),
	}
	s.accessLog = append(s.accessLog, log)

	// Update download count
	if link, exists := s.links[shareID]; exists {
		if action == "download" {
			link.DownloadCount++
			link.UpdatedAt = time.Now()
		}
	}
}

// GetAccessLogs returns access logs for a share link
func (s *WebShareService) GetAccessLogs(shareID string) []AccessLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []AccessLog
	for _, log := range s.accessLog {
		if log.ShareID == shareID {
			result = append(result, log)
		}
	}
	return result
}

// GetStats returns sharing statistics
func (s *WebShareService) GetStats() ShareStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := ShareStats{
		TotalLinks:    len(s.links),
		TotalAccesses: len(s.accessLog),
	}

	for _, link := range s.links {
		if link.IsActive {
			stats.ActiveLinks++
			stats.TotalDownloads += link.DownloadCount
		}
	}

	return stats
}

// cleanupExpiredLinks removes expired share links
func (s *WebShareService) cleanupExpiredLinks() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			for id, link := range s.links {
				if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
					link.IsActive = false
					link.UpdatedAt = time.Now()
					log.Printf("[WebShare2] Expired share link: %s", id)
				}
			}
			s.mu.Unlock()
		}
	}
}

// collectStats periodically collects statistics
func (s *WebShareService) collectStats() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			stats := s.GetStats()
			log.Printf("[WebShare2] Stats: %d active links, %d total downloads",
				stats.ActiveLinks, stats.TotalDownloads)
		}
	}
}

// CreateShareRequest represents a request to create a share link
type CreateShareRequest struct {
	Name         string          `json:"name"`
	Path         string          `json:"path"`
	Permission   SharePermission `json:"permission"`
	Password     string          `json:"password,omitempty"`
	MaxDownloads int             `json:"max_downloads,omitempty"`
	Expiry       time.Duration   `json:"expiry,omitempty"`
	CreatedBy    string          `json:"created_by"`
	Size         int64           `json:"size"`
	MimeType     string          `json:"mime_type"`
}

// ShareStats contains sharing statistics
type ShareStats struct {
	TotalLinks      int   `json:"total_links"`
	ActiveLinks     int   `json:"active_links"`
	TotalDownloads  int   `json:"total_downloads"`
	TotalAccesses   int   `json:"total_accesses"`
}

// Helper functions
func generateID() string {
	return fmt.Sprintf("ws_%d", time.Now().UnixNano())
}

func generateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func isPreviewable(mimeType string) bool {
	previewableTypes := map[string]bool{
		"image/jpeg":    true,
		"image/png":     true,
		"image/gif":     true,
		"image/webp":    true,
		"video/mp4":     true,
		"video/webm":    true,
		"audio/mpeg":    true,
		"application/pdf": true,
		"text/plain":    true,
		"text/html":     true,
	}
	return previewableTypes[mimeType]
}
