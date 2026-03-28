// Package shares provides secure sharing functionality with password protection and expiration
package shares

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// SecureShareLink represents a secure share link with protection
type SecureShareLink struct {
	ID           string            `json:"id"`
	Token        string            `json:"token"` // Unique access token
	ResourceType string            `json:"resourceType"` // file, album, folder
	ResourceID   string            `json:"resourceId"`
	ResourceName string            `json:"resourceName"`
	
	// Protection settings
	PasswordHash  string    `json:"passwordHash,omitempty"` // bcrypt hash
	PasswordSalt  string    `json:"passwordSalt,omitempty"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	MaxAccesses   int        `json:"maxAccesses,omitempty"` // 0 = unlimited
	CurrentAccesses int      `json:"currentAccesses"`
	
	// Owner info
	CreatedBy   string    `json:"createdBy"`
	CreatedAt   time.Time `json:"createdAt"`
	LastAccess  *time.Time `json:"lastAccess,omitempty"`
	
	// Permissions
	AllowDownload bool `json:"allowDownload"`
	AllowUpload   bool `json:"allowUpload"`
	AllowDelete   bool `json:"allowDelete"`
	
	// Access tracking
	AccessLog []ShareAccessLog `json:"accessLog,omitempty"`
	
	// Metadata
	Description string            `json:"description,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// ShareAccessLog records access to a share link
type ShareAccessLog struct {
	Timestamp   time.Time `json:"timestamp"`
	IPAddress   string    `json:"ipAddress,omitempty"`
	UserAgent   string    `json:"userAgent,omitempty"`
	Action      string    `json:"action"` // view, download, upload, delete
	Success     bool      `json:"success"`
	Reason      string    `json:"reason,omitempty"`
}

// SecureShareManager manages secure share links
type SecureShareManager struct {
	mu     sync.RWMutex
	links  map[string]*SecureShareLink
	tokens map[string]string // token -> link ID mapping
	
	// Configuration
	config SecureShareConfig
}

// SecureShareConfig contains configuration for secure sharing
type SecureShareConfig struct {
	DefaultExpiration   time.Duration `json:"defaultExpiration"`
	MaxExpirationDays   int           `json:"maxExpirationDays"`
	TokenLength         int           `json:"tokenLength"`
	MaxAccessesDefault  int           `json:"maxAccessesDefault"`
	EnableAccessLog     bool          `json:"enableAccessLog"`
	MaxAccessLogEntries int           `json:"maxAccessLogEntries"`
}

// DefaultSecureShareConfig returns default configuration
func DefaultSecureShareConfig() SecureShareConfig {
	return SecureShareConfig{
		DefaultExpiration:   7 * 24 * time.Hour, // 7 days
		MaxExpirationDays:   30,
		TokenLength:         32,
		MaxAccessesDefault:  100,
		EnableAccessLog:     true,
		MaxAccessLogEntries: 100,
	}
}

// NewSecureShareManager creates a new secure share manager
func NewSecureShareManager(config SecureShareConfig) *SecureShareManager {
	return &SecureShareManager{
		links:  make(map[string]*SecureShareLink),
		tokens: make(map[string]string),
		config: config,
	}
}

// CreateSecureLink creates a new secure share link
func (m *SecureShareManager) CreateSecureLink(ctx context.Context, req CreateSecureLinkRequest) (*SecureShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate request
	if err := m.validateRequest(req); err != nil {
		return nil, err
	}

	// Generate unique token
	token, err := m.generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create link
	link := &SecureShareLink{
		ID:           generateShareID(),
		Token:        token,
		ResourceType: req.ResourceType,
		ResourceID:   req.ResourceID,
		ResourceName: req.ResourceName,
		CreatedBy:    req.CreatedBy,
		CreatedAt:    time.Now(),
		AllowDownload: req.AllowDownload,
		AllowUpload:  req.AllowUpload,
		AllowDelete:  req.AllowDelete,
		Description:  req.Description,
		Metadata:     req.Metadata,
		AccessLog:    []ShareAccessLog{},
	}

	// Set password protection
	if req.Password != "" {
		hash, salt, err := hashPassword(req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		link.PasswordHash = hash
		link.PasswordSalt = salt
	}

	// Set expiration
	if req.ExpirationDays > 0 {
		if req.ExpirationDays > m.config.MaxExpirationDays {
			req.ExpirationDays = m.config.MaxExpirationDays
		}
		exp := time.Now().AddDate(0, 0, req.ExpirationDays)
		link.ExpiresAt = &exp
	} else {
		exp := time.Now().Add(m.config.DefaultExpiration)
		link.ExpiresAt = &exp
	}

	// Set max accesses
	if req.MaxAccesses > 0 {
		link.MaxAccesses = req.MaxAccesses
	} else {
		link.MaxAccesses = m.config.MaxAccessesDefault
	}

	m.links[link.ID] = link
	m.tokens[token] = link.ID

	return link, nil
}

// CreateSecureLinkRequest represents a request to create a secure link
type CreateSecureLinkRequest struct {
	ResourceType   string            `json:"resourceType"`
	ResourceID     string            `json:"resourceId"`
	ResourceName   string            `json:"resourceName"`
	Password       string            `json:"password,omitempty"`
	ExpirationDays int               `json:"expirationDays,omitempty"`
	MaxAccesses    int               `json:"maxAccesses,omitempty"`
	CreatedBy      string            `json:"createdBy"`
	AllowDownload  bool              `json:"allowDownload"`
	AllowUpload    bool              `json:"allowUpload"`
	AllowDelete    bool              `json:"allowDelete"`
	Description    string            `json:"description,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// validateRequest validates the create request
func (m *SecureShareManager) validateRequest(req CreateSecureLinkRequest) error {
	if req.ResourceType == "" {
		return fmt.Errorf("resource type is required")
	}
	if req.ResourceID == "" {
		return fmt.Errorf("resource ID is required")
	}
	if req.CreatedBy == "" {
		return fmt.Errorf("creator is required")
	}
	
	validTypes := map[string]bool{
		"file":   true,
		"album":  true,
		"folder": true,
		"photo":  true,
	}
	if !validTypes[req.ResourceType] {
		return fmt.Errorf("invalid resource type: %s", req.ResourceType)
	}
	
	return nil
}

// generateToken generates a secure random token
func (m *SecureShareManager) generateToken() (string, error) {
	bytes := make([]byte, m.config.TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// ValidateAccess validates access to a share link
func (m *SecureShareManager) ValidateAccess(ctx context.Context, token, password string) (*SecureShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Find link by token
	linkID, ok := m.tokens[token]
	if !ok {
		return nil, fmt.Errorf("invalid share link")
	}

	link, ok := m.links[linkID]
	if !ok {
		return nil, fmt.Errorf("share link not found")
	}

	// Check expiration
	if link.ExpiresAt != nil && time.Now().After(*link.ExpiresAt) {
		return nil, fmt.Errorf("share link has expired")
	}

	// Check max accesses
	if link.MaxAccesses > 0 && link.CurrentAccesses >= link.MaxAccesses {
		return nil, fmt.Errorf("share link has reached maximum accesses")
	}

	// Check password
	if link.PasswordHash != "" {
		if password == "" {
			return nil, fmt.Errorf("password required")
		}
		if !verifyPassword(password, link.PasswordHash, link.PasswordSalt) {
			return nil, fmt.Errorf("invalid password")
		}
	}

	// Update access count
	link.CurrentAccesses++
	now := time.Now()
	link.LastAccess = &now

	return link, nil
}

// LogAccess logs access to a share link
func (m *SecureShareManager) LogAccess(ctx context.Context, token string, log ShareAccessLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	linkID, ok := m.tokens[token]
	if !ok {
		return fmt.Errorf("invalid share link")
	}

	link, ok := m.links[linkID]
	if !ok {
		return fmt.Errorf("share link not found")
	}

	if !m.config.EnableAccessLog {
		return nil
	}

	// Add log entry
	log.Timestamp = time.Now()
	link.AccessLog = append(link.AccessLog, log)

	// Trim log if needed
	if len(link.AccessLog) > m.config.MaxAccessLogEntries {
		link.AccessLog = link.AccessLog[len(link.AccessLog)-m.config.MaxAccessLogEntries:]
	}

	return nil
}

// GetLink retrieves a share link by ID
func (m *SecureShareManager) GetLink(ctx context.Context, linkID string) (*SecureShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.links[linkID]
	if !ok {
		return nil, fmt.Errorf("share link not found")
	}

	return link, nil
}

// GetLinkByToken retrieves a share link by token
func (m *SecureShareManager) GetLinkByToken(ctx context.Context, token string) (*SecureShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	linkID, ok := m.tokens[token]
	if !ok {
		return nil, fmt.Errorf("invalid share link")
	}

	link, ok := m.links[linkID]
	if !ok {
		return nil, fmt.Errorf("share link not found")
	}

	return link, nil
}

// ListLinks lists share links created by a user
func (m *SecureShareManager) ListLinks(ctx context.Context, userID string) ([]*SecureShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var links []*SecureShareLink
	for _, link := range m.links {
		if link.CreatedBy == userID {
			links = append(links, link)
		}
	}

	// Sort by creation date (newest first)
	sortLinksByDate(links)

	return links, nil
}

// DeleteLink deletes a share link
func (m *SecureShareManager) DeleteLink(ctx context.Context, linkID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[linkID]
	if !ok {
		return fmt.Errorf("share link not found")
	}

	delete(m.links, linkID)
	delete(m.tokens, link.Token)

	return nil
}

// UpdateLink updates a share link
func (m *SecureShareManager) UpdateLink(ctx context.Context, linkID string, req UpdateLinkRequest) (*SecureShareLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[linkID]
	if !ok {
		return nil, fmt.Errorf("share link not found")
	}

	// Update fields
	if req.Password != "" {
		hash, salt, err := hashPassword(req.Password)
		if err != nil {
			return nil, fmt.Errorf("failed to hash password: %w", err)
		}
		link.PasswordHash = hash
		link.PasswordSalt = salt
	}

	if req.ExpirationDays > 0 {
		if req.ExpirationDays > m.config.MaxExpirationDays {
			req.ExpirationDays = m.config.MaxExpirationDays
		}
		exp := time.Now().AddDate(0, 0, req.ExpirationDays)
		link.ExpiresAt = &exp
	}

	if req.MaxAccesses >= 0 {
		link.MaxAccesses = req.MaxAccesses
	}

	if req.AllowDownload != nil {
		link.AllowDownload = *req.AllowDownload
	}
	if req.AllowUpload != nil {
		link.AllowUpload = *req.AllowUpload
	}
	if req.AllowDelete != nil {
		link.AllowDelete = *req.AllowDelete
	}

	return link, nil
}

// UpdateLinkRequest represents an update request
type UpdateLinkRequest struct {
	Password      string `json:"password,omitempty"`
	ExpirationDays int    `json:"expirationDays,omitempty"`
	MaxAccesses   int    `json:"maxAccesses,omitempty"`
	AllowDownload *bool  `json:"allowDownload,omitempty"`
	AllowUpload   *bool  `json:"allowUpload,omitempty"`
	AllowDelete   *bool  `json:"allowDelete,omitempty"`
}

// CleanupExpiredLinks removes expired links
func (m *SecureShareManager) CleanupExpiredLinks(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()

	for id, link := range m.links {
		if link.ExpiresAt != nil && now.After(*link.ExpiresAt) {
			delete(m.links, id)
			delete(m.tokens, link.Token)
			count++
		}
	}

	return count, nil
}

// hashPassword creates a password hash with salt
func hashPassword(password string) (hash, salt string, err error) {
	// Generate salt
	saltBytes := make([]byte, 32)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", "", err
	}
	salt = hex.EncodeToString(saltBytes)

	// Create hash
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte(password))
	hash = hex.EncodeToString(h.Sum(nil))

	return hash, salt, nil
}

// verifyPassword verifies a password against the stored hash
func verifyPassword(password, storedHash, salt string) bool {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte(password))
	computedHash := hex.EncodeToString(h.Sum(nil))

	return subtle.ConstantTimeCompare([]byte(computedHash), []byte(storedHash)) == 1
}

// generateShareID generates a unique share ID
func generateShareID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// sortLinksByDate sorts links by creation date (newest first)
func sortLinksByDate(links []*SecureShareLink) {
	for i := 0; i < len(links)-1; i++ {
		for j := i + 1; j < len(links); j++ {
			if links[i].CreatedAt.Before(links[j].CreatedAt) {
				links[i], links[j] = links[j], links[i]
			}
		}
	}
}

// ParseShareURL parses a share URL and extracts the token
func ParseShareURL(url string) (token string, err error) {
	// Expected format: /share/{token} or /s/{token}
	parts := strings.Split(strings.TrimSuffix(url, "/"), "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid share URL format")
	}

	// Find the last segment that looks like a token
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && parts[i] != "share" && parts[i] != "s" {
			return parts[i], nil
		}
	}

	return "", fmt.Errorf("token not found in URL")
}

// GenerateShareURL generates a share URL from a base URL and token
func GenerateShareURL(baseURL, token string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")
	return fmt.Sprintf("%s/share/%s", baseURL, token)
}

// IsPasswordProtected checks if a link requires a password
func (l *SecureShareLink) IsPasswordProtected() bool {
	return l.PasswordHash != ""
}

// IsExpired checks if a link is expired
func (l *SecureShareLink) IsExpired() bool {
	return l.ExpiresAt != nil && time.Now().After(*l.ExpiresAt)
}

// RemainingAccesses returns the number of remaining accesses
func (l *SecureShareLink) RemainingAccesses() int {
	if l.MaxAccesses <= 0 {
		return -1 // unlimited
	}
	remaining := l.MaxAccesses - l.CurrentAccesses
	if remaining < 0 {
		return 0
	}
	return remaining
}

// TimeRemaining returns the time remaining until expiration
func (l *SecureShareLink) TimeRemaining() time.Duration {
	if l.ExpiresAt == nil {
		return -1 // no expiration
	}
	remaining := time.Until(*l.ExpiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ToPublicInfo returns public information about the link (safe to share)
func (l *SecureShareLink) ToPublicInfo() PublicShareInfo {
	return PublicShareInfo{
		Token:         l.Token,
		ResourceType:  l.ResourceType,
		ResourceName:  l.ResourceName,
		RequiresPassword: l.IsPasswordProtected(),
		ExpiresAt:     l.ExpiresAt,
		AllowDownload: l.AllowDownload,
		AllowUpload:   l.AllowUpload,
		AllowDelete:   l.AllowDelete,
		Description:   l.Description,
	}
}

// PublicShareInfo contains public information about a share
type PublicShareInfo struct {
	Token            string     `json:"token"`
	ResourceType     string     `json:"resourceType"`
	ResourceName     string     `json:"resourceName"`
	RequiresPassword bool       `json:"requiresPassword"`
	ExpiresAt        *time.Time `json:"expiresAt,omitempty"`
	AllowDownload    bool       `json:"allowDownload"`
	AllowUpload      bool       `json:"allowUpload"`
	AllowDelete      bool       `json:"allowDelete"`
	Description      string     `json:"description,omitempty"`
}

// Stats returns statistics about secure shares
func (m *SecureShareManager) Stats() ShareStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ShareStats{}
	for _, link := range m.links {
		stats.TotalLinks++
		if link.IsPasswordProtected() {
			stats.PasswordProtected++
		}
		if link.IsExpired() {
			stats.Expired++
		}
		stats.TotalAccesses += link.CurrentAccesses
	}

	return stats
}

// ShareStats contains statistics about secure shares
type ShareStats struct {
	TotalLinks       int `json:"totalLinks"`
	PasswordProtected int `json:"passwordProtected"`
	Expired          int `json:"expired"`
	TotalAccesses    int `json:"totalAccesses"`
}

// GeneratePassword generates a random password for sharing
func GeneratePassword(length int) (string, error) {
	if length < 6 {
		length = 6
	}
	if length > 32 {
		length = 32
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}

	return string(b), nil
}

// ParseMaxAccesses parses max accesses from string
func ParseMaxAccesses(s string) (int, error) {
	if s == "" || s == "unlimited" {
		return 0, nil
	}
	return strconv.Atoi(s)
}