// Package filerequest 实现文件请求功能
// 对标群晖 DSM 7.3 File Request 特性
// 支持生成安全链接收集文件、过期管理、密码保护、文件大小限制
package filerequest

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 状态定义 ==========

// RequestStatus 文件请求状态
type RequestStatus string

const (
	RequestStatusActive  RequestStatus = "active"  // 活跃，可接受上传
	RequestStatusExpired RequestStatus = "expired" // 已过期
	RequestStatusClosed  RequestStatus = "closed"  // 已关闭
)

// UploadStatus 上传状态
type UploadStatus string

const (
	UploadStatusSuccess UploadStatus = "success" // 上传成功
	UploadStatusFailed  UploadStatus = "failed"  // 上传失败
)

// ========== 核心数据结构 ==========

// FileRequest 文件收集请求
type FileRequest struct {
	ID                string        `json:"id"`                  // 唯一标识
	Title             string        `json:"title"`               // 请求标题
	Description       string        `json:"description"`         // 请求描述
	CreatorID         string        `json:"creator_id"`          // 创建者ID
	CreatorName       string        `json:"creator_name"`        // 创建者名称
	DestinationPath   string        `json:"destination_path"`    // 文件保存目录
	Status            RequestStatus `json:"status"`              // 请求状态
	ExpiresAt         *time.Time    `json:"expires_at"`          // 过期时间
	AllowAnonymous    bool          `json:"allow_anonymous"`     // 是否允许匿名上传
	MaxFileCount      int           `json:"max_file_count"`      // 最大文件数（0不限制）
	MaxFileSize       int64         `json:"max_file_size"`       // 单文件最大大小（0不限制）
	AllowedExtensions []string      `json:"allowed_extensions"`  // 允许的扩展名
	HasPassword       bool          `json:"has_password"`        // 是否有密码保护
	password          string        // 内部密码字段
	CreatedAt         time.Time     `json:"created_at"`          // 创建时间
	UpdatedAt         time.Time     `json:"updated_at"`          // 更新时间
}

// RequestLink 分享链接
type RequestLink struct {
	ID            string     `json:"id"`             // 唯一标识
	RequestID     string     `json:"request_id"`     // 关联请求ID
	Token         string     `json:"token"`          // 访问令牌
	IsActive      bool       `json:"is_active"`      // 是否启用
	MaxAccessCount int       `json:"max_access_count"` // 最大访问次数（0不限制）
	AccessCount   int        `json:"access_count"`   // 已访问次数
	ExpiresAt     *time.Time `json:"expires_at"`     // 过期时间
	CreatedAt     time.Time  `json:"created_at"`     // 创建时间
}

// UploadInfo 上传文件信息
type UploadInfo struct {
	ID           string       `json:"id"`            // 唯一标识
	RequestID    string       `json:"request_id"`    // 关联请求ID
	OriginalName string       `json:"original_name"` // 原始文件名
	FileSize     int64        `json:"file_size"`     // 文件大小
	MimeType     string       `json:"mime_type"`     // MIME类型
	Extension    string       `json:"extension"`     // 扩展名
	UploaderName string       `json:"uploader_name"` // 上传者名称
	UploaderIP   string       `json:"uploader_ip"`   // 上传者IP
	Status       UploadStatus `json:"status"`        // 上传状态
	UploadedAt   time.Time    `json:"uploaded_at"`   // 上传时间
}

// RequestStats 请求统计
type RequestStats struct {
	TotalRequests    int   `json:"total_requests"`    // 总请求数
	ActiveRequests   int   `json:"active_requests"`   // 活跃请求数
	ExpiredRequests  int   `json:"expired_requests"`  // 过期请求数
	ClosedRequests   int   `json:"closed_requests"`   // 已关闭请求数
	TotalUploads     int   `json:"total_uploads"`     // 总上传数
	TotalUploadSize  int64 `json:"total_upload_size"` // 总上传大小
}

// ========== 请求/响应结构 ==========

// CreateRequestRequest 创建文件请求
type CreateRequestRequest struct {
	Title             string     `json:"title" binding:"required"`              // 请求标题
	Description       string     `json:"description"`                           // 请求描述
	CreatorID         string     `json:"creator_id" binding:"required"`         // 创建者ID
	CreatorName       string     `json:"creator_name" binding:"required"`       // 创建者名称
	DestinationPath   string     `json:"destination_path" binding:"required"`   // 文件保存目录
	ExpiresAt         *time.Time `json:"expires_at"`                            // 过期时间
	AllowAnonymous    bool       `json:"allow_anonymous"`                       // 允许匿名上传
	MaxFileCount      int        `json:"max_file_count"`                        // 最大文件数
	MaxFileSize       int64      `json:"max_file_size"`                         // 单文件最大大小
	AllowedExtensions []string   `json:"allowed_extensions"`                    // 允许的扩展名
	Password          string     `json:"password"`                              // 访问密码
}

// UploadFileRequest 上传文件请求
type UploadFileRequest struct {
	OriginalName string `json:"original_name" binding:"required"` // 原始文件名
	FileSize     int64  `json:"file_size" binding:"required"`     // 文件大小
	MimeType     string `json:"mime_type"`                        // MIME类型
	UploaderName string `json:"uploader_name"`                    // 上传者名称
}

// ListRequestsQuery 列出请求查询参数
type ListRequestsQuery struct {
	CreatorID string        `form:"creator_id"`                   // 按创建者过滤
	Status    RequestStatus `form:"status"`                       // 按状态过滤
	Page      int           `form:"page"`                         // 页码
	PageSize  int           `form:"page_size"`                    // 每页数量
}

// ========== 服务层 ==========

// Service 文件请求服务
type Service struct {
	mu       sync.RWMutex
	requests map[string]*FileRequest   // 请求集合
	links    map[string]*RequestLink   // token -> 链接
	uploads  map[string][]*UploadInfo  // requestID -> 上传列表
}

// NewService 创建文件请求服务
func NewService() *Service {
	return &Service{
		requests: make(map[string]*FileRequest),
		links:    make(map[string]*RequestLink),
		uploads:  make(map[string][]*UploadInfo),
	}
}

// CreateRequest 创建文件请求
func (s *Service) CreateRequest(ctx context.Context, req CreateRequestRequest) (*FileRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	fr := &FileRequest{
		ID:                "fr_" + uuid.New().String()[:8],
		Title:             req.Title,
		Description:       req.Description,
		CreatorID:         req.CreatorID,
		CreatorName:       req.CreatorName,
		DestinationPath:   req.DestinationPath,
		Status:            RequestStatusActive,
		ExpiresAt:         req.ExpiresAt,
		AllowAnonymous:    req.AllowAnonymous,
		MaxFileCount:      req.MaxFileCount,
		MaxFileSize:       req.MaxFileSize,
		AllowedExtensions: req.AllowedExtensions,
		HasPassword:       req.Password != "",
		password:          req.Password,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	s.requests[fr.ID] = fr

	// 自动创建分享链接
	link := &RequestLink{
		ID:        "link_" + uuid.New().String()[:8],
		RequestID: fr.ID,
		Token:     generateToken(),
		IsActive:  true,
		CreatedAt: now,
	}
	s.links[link.Token] = link

	return fr, nil
}

// ListRequests 列出文件请求
func (s *Service) ListRequests(ctx context.Context, creatorID string, status RequestStatus, page, pageSize int) ([]*FileRequest, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*FileRequest
	for _, req := range s.requests {
		if creatorID != "" && req.CreatorID != creatorID {
			continue
		}
		if status != "" && req.Status != status {
			continue
		}
		result = append(result, req)
	}

	total := len(result)
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}

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

// GetRequest 获取请求详情
func (s *Service) GetRequest(ctx context.Context, id string) (*FileRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	req, ok := s.requests[id]
	if !ok {
		return nil, fmt.Errorf("文件请求不存在: %s", id)
	}
	return req, nil
}

// GetRequestByToken 通过令牌获取请求
func (s *Service) GetRequestByToken(ctx context.Context, token string) (*FileRequest, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	link, ok := s.links[token]
	if !ok {
		return nil, fmt.Errorf("无效的访问令牌")
	}

	req, ok := s.requests[link.RequestID]
	if !ok {
		return nil, fmt.Errorf("文件请求不存在")
	}

	return req, nil
}

// GetLinkByToken 通过令牌获取链接
func (s *Service) GetLinkByToken(ctx context.Context, token string) (*RequestLink, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	link, ok := s.links[token]
	if !ok {
		return nil, fmt.Errorf("无效的访问令牌")
	}
	return link, nil
}

// VerifyPassword 验证访问密码
func (s *Service) VerifyPassword(ctx context.Context, token, password string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	link, ok := s.links[token]
	if !ok {
		return fmt.Errorf("无效的访问令牌")
	}

	req, ok := s.requests[link.RequestID]
	if !ok {
		return fmt.Errorf("文件请求不存在")
	}

	if req.password == "" {
		return nil // 无密码保护
	}

	if req.password != password {
		return fmt.Errorf("密码错误")
	}

	return nil
}

// RecordUpload 记录上传
func (s *Service) RecordUpload(ctx context.Context, token string, info *UploadFileRequest, uploaderIP string) (*UploadInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	link, ok := s.links[token]
	if !ok {
		return nil, fmt.Errorf("无效的访问令牌")
	}

	req, ok := s.requests[link.RequestID]
	if !ok {
		return nil, fmt.Errorf("文件请求不存在")
	}

	if req.Status != RequestStatusActive {
		return nil, fmt.Errorf("文件请求已关闭")
	}

	// 检查过期
	if req.ExpiresAt != nil && time.Now().After(*req.ExpiresAt) {
		req.Status = RequestStatusExpired
		return nil, fmt.Errorf("文件请求已过期")
	}

	// 检查文件数量限制
	if req.MaxFileCount > 0 && len(s.uploads[req.ID]) >= req.MaxFileCount {
		return nil, fmt.Errorf("已达到最大文件数量限制 %d", req.MaxFileCount)
	}

	// 检查文件大小限制
	if req.MaxFileSize > 0 && info.FileSize > req.MaxFileSize {
		return nil, fmt.Errorf("文件大小 %d 超过限制 %d", info.FileSize, req.MaxFileSize)
	}

	// 检查文件类型
	if len(req.AllowedExtensions) > 0 {
		ext := getExtension(info.OriginalName)
		allowed := false
		for _, a := range req.AllowedExtensions {
			if strings.EqualFold(ext, a) {
				allowed = true
				break
			}
		}
		if !allowed {
			return nil, fmt.Errorf("文件类型 %s 不在允许列表中", ext)
		}
	}

	link.AccessCount++

	upload := &UploadInfo{
		ID:           "up_" + uuid.New().String()[:8],
		RequestID:    req.ID,
		OriginalName: info.OriginalName,
		FileSize:     info.FileSize,
		MimeType:     info.MimeType,
		Extension:    getExtension(info.OriginalName),
		UploaderName: info.UploaderName,
		UploaderIP:   uploaderIP,
		Status:       UploadStatusSuccess,
		UploadedAt:   time.Now(),
	}

	s.uploads[req.ID] = append(s.uploads[req.ID], upload)
	req.UpdatedAt = time.Now()

	return upload, nil
}

// GetUploads 获取请求的上传列表
func (s *Service) GetUploads(ctx context.Context, requestID string) ([]*UploadInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uploads, ok := s.uploads[requestID]
	if !ok {
		return nil, nil
	}
	return uploads, nil
}

// DeleteRequest 删除文件请求
func (s *Service) DeleteRequest(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.requests[id]
	if !ok {
		return fmt.Errorf("文件请求不存在: %s", id)
	}

	// 删除关联链接
	for token, link := range s.links {
		if link.RequestID == id {
			delete(s.links, token)
		}
	}

	// 删除上传记录
	delete(s.uploads, id)
	delete(s.requests, id)

	_ = req // 避免未使用警告
	return nil
}

// GetStats 获取统计
func (s *Service) GetStats(ctx context.Context) (*RequestStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &RequestStats{}
	for _, req := range s.requests {
		stats.TotalRequests++
		switch req.Status {
		case RequestStatusActive:
			stats.ActiveRequests++
		case RequestStatusExpired:
			stats.ExpiredRequests++
		case RequestStatusClosed:
			stats.ClosedRequests++
		}
		stats.TotalUploads += len(s.uploads[req.ID])
		for _, u := range s.uploads[req.ID] {
			stats.TotalUploadSize += u.FileSize
		}
	}
	return stats, nil
}

// CloseRequest 关闭请求
func (s *Service) CloseRequest(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, ok := s.requests[id]
	if !ok {
		return fmt.Errorf("文件请求不存在: %s", id)
	}

	req.Status = RequestStatusClosed
	req.UpdatedAt = time.Now()
	return nil
}

// ========== 辅助函数 ==========

// generateToken 生成随机访问令牌
func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// getExtension 获取文件扩展名
func getExtension(filename string) string {
	idx := strings.LastIndex(filename, ".")
	if idx < 0 {
		return ""
	}
	return filename[idx:]
}
