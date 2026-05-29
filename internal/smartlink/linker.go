package smartlink

import (
	"crypto/rand"
	"crypto/subtle"
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrLinkNotFound      = errors.New("share link not found")
	ErrLinkExpired       = errors.New("share link has expired")
	ErrLinkInactive      = errors.New("share link is inactive")
	ErrMaxVisitsReached  = errors.New("max visits reached")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrInvalidPermission = errors.New("invalid permission")
	ErrPolicyViolation   = errors.New("policy violation")
)

// Linker manages share links
type Linker struct {
	links    map[string]*ShareLink
	byToken  map[string]*ShareLink
	byFileID map[string][]*ShareLink
	logs     []AccessLog
	stats    map[string]*LinkStats
	policy   SharePolicy
	mu       sync.RWMutex
}

// NewLinker creates a new Linker instance
func NewLinker(policy SharePolicy) *Linker {
	return &Linker{
		links:    make(map[string]*ShareLink),
		byToken:  make(map[string]*ShareLink),
		byFileID: make(map[string][]*ShareLink),
		logs:     make([]AccessLog, 0),
		stats:    make(map[string]*LinkStats),
		policy:   policy,
	}
}

// generateToken generates a random 8-character base62 token
func generateToken() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// Encode to base62-like string (using alphanumeric)
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	token := make([]byte, 8)
	for i := range token {
		token[i] = charset[int(b[i%6])%len(charset)]
	}
	return string(token), nil
}

// CreateLink creates a new share link
func (l *Linker) CreateLink(creatorID string, req CreateLinkRequest) (*ShareLink, error) {
	if !isValidPermission(req.Permission) {
		return nil, ErrInvalidPermission
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check policy: max links per user
	userLinks := 0
	for _, link := range l.links {
		if link.CreatorID == creatorID {
			userLinks++
		}
	}
	if userLinks >= l.policy.MaxLinksPerUser {
		return nil, ErrPolicyViolation
	}

	// Check policy: max visits
	if req.MaxVisits > 0 && req.MaxVisits > l.policy.MaxVisitsPerLink {
		return nil, ErrPolicyViolation
	}

	// Generate unique ID and token
	id, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate ID: %w", err)
	}

	token, err := generateToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Ensure uniqueness
	for l.links[id] != nil {
		id, err = generateToken()
		if err != nil {
			return nil, err
		}
	}
	for l.byToken[token] != nil {
		token, err = generateToken()
		if err != nil {
			return nil, err
		}
	}

	now := time.Now()
	link := &ShareLink{
		ID:          id,
		Token:       token,
		FileID:      req.FileID,
		CreatorID:   creatorID,
		Permission:  req.Permission,
		Password:    req.Password,
		MaxVisits:   req.MaxVisits,
		IsOneTime:   req.IsOneTime,
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
		Description: req.Description,
	}

	// Set expiration
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		expiresAt := now.Add(time.Duration(*req.ExpiresIn) * time.Second)
		// Check max expiration policy
		maxExpires := now.Add(l.policy.MaxExpiration)
		if expiresAt.After(maxExpires) {
			expiresAt = maxExpires
		}
		link.ExpiresAt = &expiresAt
	} else if l.policy.DefaultExpiration > 0 {
		expiresAt := now.Add(l.policy.DefaultExpiration)
		link.ExpiresAt = &expiresAt
	}

	// Store link
	l.links[id] = link
	l.byToken[token] = link
	l.byFileID[req.FileID] = append(l.byFileID[req.FileID], link)

	// Initialize stats
	l.stats[id] = &LinkStats{
		LinkID: id,
	}

	return link, nil
}

// AccessLink accesses a share link by token
func (l *Linker) AccessLink(token, password, ip, userAgent string) (*ShareLink, error) {
	l.mu.RLock()
	link, exists := l.byToken[token]
	l.mu.RUnlock()

	if !exists {
		l.logAccess("", ip, userAgent, false, "link not found")
		return nil, ErrLinkNotFound
	}

	// Check if accessible
	if !link.IsActive {
		l.logAccess(link.ID, ip, userAgent, false, "link inactive")
		return nil, ErrLinkInactive
	}

	if link.IsExpired() {
		l.logAccess(link.ID, ip, userAgent, false, "link expired")
		return nil, ErrLinkExpired
	}

	if link.IsMaxVisitsReached() {
		l.logAccess(link.ID, ip, userAgent, false, "max visits reached")
		return nil, ErrMaxVisitsReached
	}

	// Check password if set
	if link.Password != "" {
		if subtle.ConstantTimeCompare([]byte(password), []byte(link.Password)) != 1 {
			l.logAccess(link.ID, ip, userAgent, false, "invalid password")
			return nil, ErrInvalidPassword
		}
	}

	// Increment visit count
	link.IncrementVisit()

	// Update stats
	l.mu.Lock()
	stats := l.stats[link.ID]
	if stats != nil {
		stats.TotalVisits = link.VisitCount
		now := time.Now()
		stats.LastAccessedAt = &now
		if stats.FirstAccessedAt == nil {
			stats.FirstAccessedAt = &now
		}
		// Track unique IPs
		found := false
		for _, existingIP := range stats.IPAddresses {
			if existingIP == ip {
				found = true
				break
			}
		}
		if !found {
			stats.IPAddresses = append(stats.IPAddresses, ip)
			stats.UniqueVisitors = len(stats.IPAddresses)
		}
	}
	l.mu.Unlock()

	l.logAccess(link.ID, ip, userAgent, true, "")

	return link, nil
}

// GetLink retrieves a link by ID
func (l *Linker) GetLink(id string) (*ShareLink, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	link, exists := l.links[id]
	if !exists {
		return nil, ErrLinkNotFound
	}
	return link, nil
}

// GetLinkByToken retrieves a link by token
func (l *Linker) GetLinkByToken(token string) (*ShareLink, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	link, exists := l.byToken[token]
	if !exists {
		return nil, ErrLinkNotFound
	}
	return link, nil
}

// ListLinksByFileID lists all links for a file
func (l *Linker) ListLinksByFileID(fileID string) []*ShareLink {
	l.mu.RLock()
	defer l.mu.RUnlock()

	links := l.byFileID[fileID]
	result := make([]*ShareLink, len(links))
	copy(result, links)
	return result
}

// ListLinksByCreator lists all links created by a user
func (l *Linker) ListLinksByCreator(creatorID string) []*ShareLink {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []*ShareLink
	for _, link := range l.links {
		if link.CreatorID == creatorID {
			result = append(result, link)
		}
	}
	return result
}

// DeactivateLink deactivates a share link
func (l *Linker) DeactivateLink(id, creatorID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	link, exists := l.links[id]
	if !exists {
		return ErrLinkNotFound
	}

	if link.CreatorID != creatorID {
		return errors.New("unauthorized")
	}

	link.mu.Lock()
	link.IsActive = false
	link.UpdatedAt = time.Now()
	link.mu.Unlock()

	return nil
}

// GetLinkStats returns statistics for a link
func (l *Linker) GetLinkStats(linkID string) (*LinkStats, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	stats, exists := l.stats[linkID]
	if !exists {
		return nil, ErrLinkNotFound
	}
	return stats, nil
}

// GetAccessLogs returns access logs for a link
func (l *Linker) GetAccessLogs(linkID string, limit, offset int) []AccessLog {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []AccessLog
	for _, log := range l.logs {
		if log.LinkID == linkID {
			result = append(result, log)
		}
	}

	// Apply pagination
	if offset >= len(result) {
		return []AccessLog{}
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end]
}

// CleanupExpiredLinks removes expired and inactive links
func (l *Linker) CleanupExpiredLinks() int {
	l.mu.Lock()
	defer l.mu.Unlock()

	removed := 0
	for id, link := range l.links {
		if link.IsExpired() || !link.IsActive {
			delete(l.links, id)
			delete(l.byToken, link.Token)
			delete(l.stats, id)

			// Remove from byFileID
			fileLinks := l.byFileID[link.FileID]
			for i, fl := range fileLinks {
				if fl.ID == id {
					l.byFileID[link.FileID] = append(fileLinks[:i], fileLinks[i+1:]...)
					break
				}
			}
			removed++
		}
	}
	return removed
}

// logAccess logs an access attempt
func (l *Linker) logAccess(linkID, ip, userAgent string, success bool, reason string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	log := AccessLog{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		LinkID:     linkID,
		IP:         ip,
		UserAgent:  userAgent,
		AccessedAt: time.Now(),
		Success:    success,
		Reason:     reason,
	}
	l.logs = append(l.logs, log)
}

// isValidPermission checks if permission is valid
func isValidPermission(p SharePermission) bool {
	switch p {
	case PermissionReadOnly, PermissionReadWrite, PermissionPreview:
		return true
	default:
		return false
	}
}
