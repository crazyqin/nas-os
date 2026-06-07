// Package filerequest provides secure file collection via shareable links.
// Inspired by Synology DSM 7.3 File Request feature.
package filerequest

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Status represents the lifecycle state of a file request.
type Status string

const (
	StatusActive    Status = "active"
	StatusExpired   Status = "expired"
	StatusRevoked   Status = "revoked"
	StatusCompleted Status = "completed"
)

// Request represents a file collection request with a shareable upload link.
type Request struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Token       string    `json:"token"`
	CreatorID   string    `json:"creator_id"`
	TargetPath  string    `json:"target_path"` // Where uploaded files land
	Status      Status    `json:"status"`
	MaxFiles    int       `json:"max_files"`   // 0 = unlimited
	MaxSizeMB   int       `json:"max_size_mb"` // 0 = unlimited
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
	UploadCount int       `json:"upload_count"`
	// Permissions
	AllowOverwrite bool `json:"allow_overwrite"`
	RequireAuth    bool `json:"require_auth"` // Uploaders must authenticate
}

// Upload tracks individual file uploads to a request.
type Upload struct {
	ID          string    `json:"id"`
	RequestID   string    `json:"request_id"`
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	ContentType string    `json:"content_type"`
	UploaderIP  string    `json:"uploader_ip"`
	UploadedAt  time.Time `json:"uploaded_at"`
}

// Manager handles file request lifecycle.
type Manager struct {
	mu       sync.RWMutex
	requests map[string]*Request
	uploads  map[string][]*Upload
}

// NewManager creates a new file request manager.
func NewManager() *Manager {
	return &Manager{
		requests: make(map[string]*Request),
		uploads:  make(map[string][]*Upload),
	}
}

// generateToken creates a cryptographically secure URL-safe token.
func generateToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CreateRequest creates a new file request with a unique upload link.
func (m *Manager) CreateRequest(title, description, creatorID, targetPath string, maxFiles, maxSizeMB int, expiresAt time.Time, allowOverwrite, requireAuth bool) (*Request, error) {
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if targetPath == "" {
		return nil, fmt.Errorf("target path is required")
	}
	if !expiresAt.After(time.Now()) {
		return nil, fmt.Errorf("expiration must be in the future")
	}

	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	idBytes := make([]byte, 8)
	rand.Read(idBytes)

	req := &Request{
		ID:             hex.EncodeToString(idBytes),
		Title:          title,
		Description:    description,
		Token:          token,
		CreatorID:      creatorID,
		TargetPath:     targetPath,
		Status:         StatusActive,
		MaxFiles:       maxFiles,
		MaxSizeMB:      maxSizeMB,
		ExpiresAt:      expiresAt,
		CreatedAt:      time.Now(),
		AllowOverwrite: allowOverwrite,
		RequireAuth:    requireAuth,
	}

	m.mu.Lock()
	m.requests[req.ID] = req
	m.mu.Unlock()

	return req, nil
}

// GetRequest retrieves a file request by ID.
func (m *Manager) GetRequest(id string) (*Request, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	req, ok := m.requests[id]
	if !ok {
		return nil, fmt.Errorf("request %s not found", id)
	}
	return req, nil
}

// GetRequestByToken retrieves a file request by its shareable token.
func (m *Manager) GetRequestByToken(token string) (*Request, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, req := range m.requests {
		if req.Token == token {
			return req, nil
		}
	}
	return nil, fmt.Errorf("request not found for token")
}

// ListRequests returns all file requests for a creator.
func (m *Manager) ListRequests(creatorID string) []*Request {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Request
	for _, req := range m.requests {
		if req.CreatorID == creatorID {
			result = append(result, req)
		}
	}
	return result
}

// RecordUpload records a file upload to a request.
func (m *Manager) RecordUpload(requestID, fileName string, fileSize int64, contentType, uploaderIP string) (*Upload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[requestID]
	if !ok {
		return nil, fmt.Errorf("request %s not found", requestID)
	}

	if req.Status != StatusActive {
		return nil, fmt.Errorf("request %s is not active (status: %s)", requestID, req.Status)
	}
	if time.Now().After(req.ExpiresAt) {
		req.Status = StatusExpired
		return nil, fmt.Errorf("request %s has expired", requestID)
	}
	if req.MaxFiles > 0 && req.UploadCount >= req.MaxFiles {
		req.Status = StatusCompleted
		return nil, fmt.Errorf("request %s has reached max files (%d)", requestID, req.MaxFiles)
	}
	if req.MaxSizeMB > 0 && fileSize > int64(req.MaxSizeMB)*1024*1024 {
		return nil, fmt.Errorf("file exceeds max size (%d MB)", req.MaxSizeMB)
	}

	upload := &Upload{
		ID:          fmt.Sprintf("%s_%d", requestID, req.UploadCount+1),
		RequestID:   requestID,
		FileName:    fileName,
		FileSize:    fileSize,
		ContentType: contentType,
		UploaderIP:  uploaderIP,
		UploadedAt:  time.Now(),
	}

	req.UploadCount++
	m.uploads[requestID] = append(m.uploads[requestID], upload)

	// Auto-complete if max files reached
	if req.MaxFiles > 0 && req.UploadCount >= req.MaxFiles {
		req.Status = StatusCompleted
	}

	return upload, nil
}

// RevokeRequest revokes a file request, preventing further uploads.
func (m *Manager) RevokeRequest(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[id]
	if !ok {
		return fmt.Errorf("request %s not found", id)
	}
	if req.Status != StatusActive {
		return fmt.Errorf("cannot revoke request in status %s", req.Status)
	}
	req.Status = StatusRevoked
	return nil
}

// GetUploads returns all uploads for a file request.
func (m *Manager) GetUploads(requestID string) []*Upload {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.uploads[requestID]
}

// GetStats returns aggregate statistics for a creator's requests.
func (m *Manager) GetStats(creatorID string) (totalRequests, activeRequests, totalUploads int, totalSize int64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, req := range m.requests {
		if req.CreatorID == creatorID {
			totalRequests++
			if req.Status == StatusActive {
				activeRequests++
			}
			for _, up := range m.uploads[req.ID] {
				totalUploads++
				totalSize += up.FileSize
			}
		}
	}
	return
}
