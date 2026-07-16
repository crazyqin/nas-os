// Package filerequest 提供业务逻辑服务扩展
package filerequest

import (
	"context"
	"fmt"
	"time"
)

// UpdateRequest 更新文件请求.
func (s *Service) UpdateRequest(ctx context.Context, id string, title, description string, expiresAt *time.Time) (*FileRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.requests[id]
	if !ok {
		return nil, fmt.Errorf("文件请求不存在: %s", id)
	}

	if title != "" {
		req.Title = title
	}
	if description != "" {
		req.Description = description
	}
	if expiresAt != nil {
		req.ExpiresAt = expiresAt
	}
	req.UpdatedAt = time.Now()

	return req, nil
}

// DeleteUpload 删除已上传的文件记录.
func (s *Service) DeleteUpload(ctx context.Context, requestID, uploadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.requests[requestID]
	if !ok {
		return fmt.Errorf("文件请求不存在: %s", requestID)
	}

	uploads, ok := s.uploads[requestID]
	if !ok {
		return fmt.Errorf("请求 %s 无上传记录", requestID)
	}

	for i, u := range uploads {
		if u.ID == uploadID {
			s.uploads[requestID] = append(uploads[:i], uploads[i+1:]...)
			req.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("上传记录不存在: %s", uploadID)
}

// CheckExpired 检查并标记过期请求.
func (s *Service) CheckExpired(ctx context.Context) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	count := 0
	for _, req := range s.requests {
		if req.Status == RequestStatusActive && req.ExpiresAt != nil && now.After(*req.ExpiresAt) {
			req.Status = RequestStatusExpired
			req.UpdatedAt = now
			count++
		}
	}
	return count
}
