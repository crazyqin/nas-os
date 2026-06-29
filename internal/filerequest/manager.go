package filerequest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// Manager 文件请求管理器
type Manager struct {
	mu       sync.RWMutex
	requests map[string]*FileRequest
	links    map[string]*RequestLink
	uploads  map[string][]*UploadInfo
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		requests: make(map[string]*FileRequest),
		links:    make(map[string]*RequestLink),
		uploads:  make(map[string][]*UploadInfo),
	}
}

// CreateRequest 创建文件请求
func (m *Manager) CreateRequest(title, description, creatorID, destPath string, maxFiles int, maxSizeMB int, expiresAt time.Time, allowOverwrite, requireAuth bool) (*FileRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	req := &FileRequest{
		ID:              generateID(),
		Title:           title,
		Description:     description,
		CreatorID:       creatorID,
		DestinationPath: destPath,
		Status:          RequestStatusActive,
		MaxFileCount:    maxFiles,
		MaxFileSize:     int64(maxSizeMB) * 1024 * 1024,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if !expiresAt.IsZero() {
		req.ExpiresAt = &expiresAt
	}

	m.requests[req.ID] = req
	return req, nil
}

// GetRequest 获取请求
func (m *Manager) GetRequest(ctx context.Context, id string) (*FileRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	req, ok := m.requests[id]
	if !ok {
		return nil, fmt.Errorf("request not found: %s", id)
	}
	return req, nil
}

// GetRequestByToken 通过令牌获取请求
func (m *Manager) GetRequestByToken(ctx context.Context, token string) (*FileRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.links[token]
	if !ok {
		return nil, fmt.Errorf("link not found")
	}

	req, ok := m.requests[link.RequestID]
	if !ok {
		return nil, fmt.Errorf("request not found")
	}

	return req, nil
}

// ListRequests 列出请求
func (m *Manager) ListRequests(ctx context.Context, creatorID string, status RequestStatus, page, pageSize int) ([]*FileRequest, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*FileRequest
	for _, req := range m.requests {
		if creatorID != "" && req.CreatorID != creatorID {
			continue
		}
		if status != "" && req.Status != status {
			continue
		}
		result = append(result, req)
	}

	total := len(result)
	start := (page - 1) * pageSize
	if start >= total {
		return nil, total, nil
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	return result[start:end], total, nil
}

// UpdateRequest 更新请求
func (m *Manager) UpdateRequest(ctx context.Context, id string, req *UpdateRequestRequest) (*FileRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.requests[id]
	if !ok {
		return nil, fmt.Errorf("request not found: %s", id)
	}

	if req.Title != "" {
		existing.Title = req.Title
	}
	if req.Description != "" {
		existing.Description = req.Description
	}
	if req.ExpiresAt != nil {
		existing.ExpiresAt = req.ExpiresAt
	}
	if req.MaxFileCount != nil {
		existing.MaxFileCount = *req.MaxFileCount
	}
	if req.MaxFileSize != nil {
		existing.MaxFileSize = *req.MaxFileSize
	}
	existing.UpdatedAt = time.Now()

	return existing, nil
}

// RevokeRequest 撤销请求
func (m *Manager) RevokeRequest(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[id]
	if !ok {
		return fmt.Errorf("request not found: %s", id)
	}

	req.Status = RequestStatusClosed
	req.UpdatedAt = time.Now()
	return nil
}

// CreateLink 创建分享链接
func (m *Manager) CreateLink(ctx context.Context, requestID string, req *CreateLinkRequest) (*RequestLink, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.requests[requestID]
	if !ok {
		return nil, fmt.Errorf("request not found: %s", requestID)
	}

	link := &RequestLink{
		ID:            generateID(),
		RequestID:     requestID,
		Token:         generateToken(),
		IsActive:      true,
		Password:      req.Password,
		MaxAccessCount: req.MaxAccessCount,
		ExpiresAt:     req.ExpiresAt,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	m.links[link.Token] = link
	return link, nil
}

// RecordUpload 记录上传
func (m *Manager) RecordUpload(ctx context.Context, requestID string, info *UploadInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[requestID]
	if !ok {
		return fmt.Errorf("request not found: %s", requestID)
	}

	info.ID = generateID()
	info.RequestID = requestID
	info.Status = UploadStatusSuccess
	info.UploadedAt = time.Now()

	m.uploads[requestID] = append(m.uploads[requestID], info)
	req.ReceivedFileCount++
	req.ReceivedTotalSize += info.FileSize
	req.UpdatedAt = time.Now()

	return nil
}

// GetUploads 获取上传列表
func (m *Manager) GetUploads(ctx context.Context, requestID string) ([]*UploadInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	uploads, ok := m.uploads[requestID]
	if !ok {
		return nil, nil
	}

	return uploads, nil
}

// GetStats 获取统计信息
func (m *Manager) GetStats(ctx context.Context) (*RequestStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &RequestStats{}
	for _, req := range m.requests {
		stats.TotalRequests++
		switch req.Status {
		case RequestStatusActive:
			stats.ActiveRequests++
		case RequestStatusExpired:
			stats.ExpiredRequests++
		case RequestStatusClosed:
			stats.ClosedRequests++
		}
		stats.TotalUploads += req.ReceivedFileCount
		stats.TotalUploadSize += req.ReceivedTotalSize
	}

	return stats, nil
}

// CloseRequest 关闭请求，不再接受上传
func (m *Manager) CloseRequest(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[id]
	if !ok {
		return fmt.Errorf("request not found: %s", id)
	}

	req.Status = RequestStatusClosed
	req.UpdatedAt = time.Now()
	return nil
}

// DeleteUpload 删除已上传的文件记录
func (m *Manager) DeleteUpload(ctx context.Context, requestID, uploadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, ok := m.requests[requestID]
	if !ok {
		return fmt.Errorf("request not found: %s", requestID)
	}

	uploads, ok := m.uploads[requestID]
	if !ok {
		return fmt.Errorf("no uploads found for request: %s", requestID)
	}

	found := false
	for i, u := range uploads {
		if u.ID == uploadID {
			m.uploads[requestID] = append(uploads[:i], uploads[i+1:]...)
			req.ReceivedFileCount--
			req.ReceivedTotalSize -= u.FileSize
			req.UpdatedAt = time.Now()
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("upload not found: %s", uploadID)
	}

	return nil
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
