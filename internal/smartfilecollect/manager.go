// Package smartfilecollect - 智能文件收集管理器
// 收集链接创建、文件接收、自动分类、策略管理
package smartfilecollect

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CollectManager 文件收集管理器
type CollectManager struct {
	mu          sync.RWMutex
	config      CollectConfig
	requests    map[string]*CollectRequest   // 收集请求
	submissions map[string][]*FileSubmission // 收集ID -> 提交列表
	fileHashes  map[string]string            // 文件哈希 -> 提交ID (用于去重)
}

// NewCollectManager 创建收集管理器
func NewCollectManager(config *CollectConfig) *CollectManager {
	cfg := DefaultCollectConfig()
	if config != nil {
		cfg = *config
	}

	return &CollectManager{
		config:      cfg,
		requests:    make(map[string]*CollectRequest),
		submissions: make(map[string][]*FileSubmission),
		fileHashes:  make(map[string]string),
	}
}

// CreateCollectRequest 创建收集请求
func (m *CollectManager) CreateCollectRequest(req *CreateCollectRequest, creatorID, creatorName string) (*CollectRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Title == "" {
		return nil, fmt.Errorf("标题不能为空")
	}

	// 生成ID和令牌
	id := generateID()
	accessToken := generateToken()

	// 计算过期时间
	expireDays := m.config.DefaultExpireDays
	if req.ExpiresIn > 0 {
		if req.ExpiresIn > m.config.MaxExpireDays {
			expireDays = m.config.MaxExpireDays
		} else {
			expireDays = req.ExpiresIn
		}
	}
	expiresAt := time.Now().AddDate(0, 0, expireDays)

	// 设置默认策略
	policy := req.Policy
	if policy.MaxFileSize == 0 {
		policy.MaxFileSize = m.config.MaxFileSize
	}
	if policy.MaxTotalSize == 0 {
		policy.MaxTotalSize = m.config.MaxTotalSize
	}
	if len(policy.AllowedExts) == 0 {
		policy.AllowedExts = m.config.AllowedExts
	}
	if len(policy.BlockedExts) == 0 {
		policy.BlockedExts = m.config.BlockedExts
	}
	// 设置安全策略默认值
	if !policy.EnableDedup {
		policy.EnableDedup = m.config.EnableDedup
	}
	if !policy.EnableVirusScan {
		policy.EnableVirusScan = m.config.EnableVirusScan
	}
	if !policy.AutoClassify {
		policy.AutoClassify = m.config.EnableAutoClass
	}

	// 创建收集请求
	collectReq := &CollectRequest{
		ID:          id,
		Title:       req.Title,
		Description: req.Description,
		CreatorID:   creatorID,
		CreatorName: creatorName,
		ShareLink:   fmt.Sprintf("/collect/%s", id),
		AccessToken: accessToken,
		Status:      CollectStatusActive,
		TargetPath:  req.TargetPath,
		Config:      policy,
		CreatedAt:   time.Now(),
		ExpiresAt:   &expiresAt,
		UpdatedAt:   time.Now(),
	}

	m.requests[id] = collectReq
	m.submissions[id] = make([]*FileSubmission, 0)

	return collectReq, nil
}

// GetCollectRequest 获取收集请求
func (m *CollectManager) GetCollectRequest(id string) (*CollectRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	req, exists := m.requests[id]
	if !exists {
		return nil, fmt.Errorf("收集请求不存在: %s", id)
	}

	// 检查是否过期
	if req.ExpiresAt != nil && time.Now().After(*req.ExpiresAt) {
		req.Status = CollectStatusExpired
	}

	return req, nil
}

// ListCollectRequests 列出收集请求
func (m *CollectManager) ListCollectRequests(creatorID string) []CollectRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	requests := make([]CollectRequest, 0)
	for _, req := range m.requests {
		if creatorID == "" || req.CreatorID == creatorID {
			// 检查是否过期
			if req.ExpiresAt != nil && time.Now().After(*req.ExpiresAt) {
				req.Status = CollectStatusExpired
			}
			requests = append(requests, *req)
		}
	}

	return requests
}

// UpdateCollectRequest 更新收集请求
func (m *CollectManager) UpdateCollectRequest(id string, updates map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	req, exists := m.requests[id]
	if !exists {
		return fmt.Errorf("收集请求不存在: %s", id)
	}

	if title, ok := updates["title"].(string); ok {
		req.Title = title
	}
	if desc, ok := updates["description"].(string); ok {
		req.Description = desc
	}
	if status, ok := updates["status"].(CollectStatus); ok {
		req.Status = status
	}

	req.UpdatedAt = time.Now()
	return nil
}

// DeleteCollectRequest 删除收集请求
func (m *CollectManager) DeleteCollectRequest(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.requests[id]; !exists {
		return fmt.Errorf("收集请求不存在: %s", id)
	}

	delete(m.requests, id)
	delete(m.submissions, id)

	return nil
}

// SubmitFile 提交文件
func (m *CollectManager) SubmitFile(collectID string, file io.Reader, fileName string, req *SubmitFileRequest, clientIP string) (*FileSubmission, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证收集请求
	collectReq, exists := m.requests[collectID]
	if !exists {
		return nil, fmt.Errorf("收集请求不存在: %s", collectID)
	}

	// 检查状态
	if collectReq.Status != CollectStatusActive {
		return nil, fmt.Errorf("收集请求已关闭或过期")
	}

	// 检查过期
	if collectReq.ExpiresAt != nil && time.Now().After(*collectReq.ExpiresAt) {
		collectReq.Status = CollectStatusExpired
		return nil, fmt.Errorf("收集请求已过期")
	}

	// 检查最大提交数
	if collectReq.Config.MaxSubmissions > 0 {
		if len(m.submissions[collectID]) >= collectReq.Config.MaxSubmissions {
			collectReq.Status = CollectStatusFull
			return nil, fmt.Errorf("已达到最大提交数")
		}
	}

	// 验证文件扩展名
	ext := strings.ToLower(filepath.Ext(fileName))
	if !m.isAllowedExt(ext, collectReq.Config.AllowedExts, collectReq.Config.BlockedExts) {
		return nil, fmt.Errorf("不允许的文件类型: %s", ext)
	}

	// 创建临时文件
	tempFile, err := os.CreateTemp(m.config.TempPath, "collect-*"+ext)
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	// 写入临时文件并计算大小和哈希
	hasher := sha256.New()
	writer := io.MultiWriter(tempFile, hasher)
	fileSize, err := io.Copy(writer, file)
	if err != nil {
		return nil, fmt.Errorf("写入文件失败: %w", err)
	}

	// 检查文件大小
	if fileSize > collectReq.Config.MaxFileSize {
		return nil, fmt.Errorf("文件大小超过限制")
	}

	// 检查总量
	if collectReq.Config.MaxTotalSize > 0 {
		if collectReq.TotalSize+fileSize > collectReq.Config.MaxTotalSize {
			return nil, fmt.Errorf("超过总容量限制")
		}
	}

	// 计算哈希
	checksum := hex.EncodeToString(hasher.Sum(nil))

	// 检查重复
	if collectReq.Config.EnableDedup {
		if _, exists := m.fileHashes[checksum]; exists {
			return &FileSubmission{
				ID:              generateID(),
				CollectID:       collectID,
				FileName:        fileName,
				FileSize:        fileSize,
				FileExt:         ext,
				Category:        m.classifyFile(ext),
				Checksum:        checksum,
				Status:          SubmissionStatusDuplicate,
				SubmitterName:   req.SubmitterName,
				SubmitterEmail:  req.SubmitterEmail,
				SubmitterIP:     clientIP,
				RejectionReason: "文件已存在",
				SubmittedAt:     time.Now(),
			}, nil
		}
	}

	// 检测MIME类型
	mimeType := mime.TypeByExtension(ext)

	// 分类文件
	category := CategoryOther
	if collectReq.Config.AutoClassify {
		category = m.classifyFile(ext)
	}

	// 生成存储路径
	submissionID := generateID()
	storagePath := filepath.Join(collectReq.TargetPath, submissionID+ext)

	// 创建存储目录
	if err := os.MkdirAll(filepath.Dir(storagePath), 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	// 移动文件到最终位置
	if err := os.Rename(tempFile.Name(), storagePath); err != nil {
		return nil, fmt.Errorf("移动文件失败: %w", err)
	}

	// 创建提交记录
	submission := &FileSubmission{
		ID:             submissionID,
		CollectID:      collectID,
		FileName:       fileName,
		FileSize:       fileSize,
		FileType:       mimeType,
		FileExt:        ext,
		Category:       category,
		Checksum:       checksum,
		Status:         SubmissionStatusPending,
		SubmitterName:  req.SubmitterName,
		SubmitterEmail: req.SubmitterEmail,
		SubmitterIP:    clientIP,
		StoragePath:    storagePath,
		SubmittedAt:    time.Now(),
	}

	// 如果启用病毒扫描，设置为扫描中
	if collectReq.Config.EnableVirusScan {
		submission.Status = SubmissionStatusScanning
	}

	// 更新统计
	collectReq.SubmissionCount++
	collectReq.TotalSize += fileSize
	collectReq.UpdatedAt = time.Now()

	// 记录哈希
	m.fileHashes[checksum] = submissionID

	// 保存提交
	m.submissions[collectID] = append(m.submissions[collectID], submission)

	return submission, nil
}

// GetSubmission 获取提交
func (m *CollectManager) GetSubmission(collectID, submissionID string) (*FileSubmission, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subs, exists := m.submissions[collectID]
	if !exists {
		return nil, fmt.Errorf("收集请求不存在: %s", collectID)
	}

	for _, sub := range subs {
		if sub.ID == submissionID {
			return sub, nil
		}
	}

	return nil, fmt.Errorf("提交不存在")
}

// ListSubmissions 列出提交
func (m *CollectManager) ListSubmissions(collectID string) ([]FileSubmission, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	subs, exists := m.submissions[collectID]
	if !exists {
		return nil, fmt.Errorf("收集请求不存在: %s", collectID)
	}

	result := make([]FileSubmission, len(subs))
	for i, sub := range subs {
		result[i] = *sub
	}
	return result, nil
}

// UpdateSubmissionStatus 更新提交状态
func (m *CollectManager) UpdateSubmissionStatus(collectID, submissionID string, status SubmissionStatus, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subs, exists := m.submissions[collectID]
	if !exists {
		return fmt.Errorf("收集请求不存在: %s", collectID)
	}

	for _, sub := range subs {
		if sub.ID == submissionID {
			sub.Status = status
			sub.RejectionReason = reason
			now := time.Now()
			sub.ProcessedAt = &now
			return nil
		}
	}

	return fmt.Errorf("提交不存在")
}

// UpdateVirusScanResult 更新病毒扫描结果
func (m *CollectManager) UpdateVirusScanResult(collectID, submissionID string, result *VirusScanResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	subs, exists := m.submissions[collectID]
	if !exists {
		return fmt.Errorf("收集请求不存在: %s", collectID)
	}

	for _, sub := range subs {
		if sub.ID == submissionID {
			sub.VirusScanResult = result
			if result.Clean {
				sub.Status = SubmissionStatusAccepted
			} else {
				sub.Status = SubmissionStatusInfected
				sub.RejectionReason = fmt.Sprintf("检测到威胁: %s", result.ThreatName)
			}
			now := time.Now()
			sub.ProcessedAt = &now
			return nil
		}
	}

	return fmt.Errorf("提交不存在")
}

// GetStats 获取统计信息
func (m *CollectManager) GetStats(creatorID string) *CollectStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &CollectStats{}

	for _, req := range m.requests {
		if creatorID != "" && req.CreatorID != creatorID {
			continue
		}
		stats.TotalRequests++
		if req.Status == CollectStatusActive {
			stats.ActiveRequests++
		}

		subs := m.submissions[req.ID]
		stats.TotalSubmissions += len(subs)
		for _, sub := range subs {
			stats.TotalFiles++
			stats.TotalSize += sub.FileSize
			if sub.Status == SubmissionStatusInfected {
				stats.InfectedFiles++
			}
			if sub.Status == SubmissionStatusDuplicate {
				stats.DuplicateFiles++
			}
		}
	}

	return stats
}

// PauseCollectRequest 暂停收集请求
func (m *CollectManager) PauseCollectRequest(id string) error {
	return m.UpdateCollectRequest(id, map[string]interface{}{
		"status": CollectStatusPaused,
	})
}

// ResumeCollectRequest 恢复收集请求
func (m *CollectManager) ResumeCollectRequest(id string) error {
	return m.UpdateCollectRequest(id, map[string]interface{}{
		"status": CollectStatusActive,
	})
}

// CloseCollectRequest 关闭收集请求
func (m *CollectManager) CloseCollectRequest(id string) error {
	return m.UpdateCollectRequest(id, map[string]interface{}{
		"status": CollectStatusClosed,
	})
}

// ValidateAccessToken 验证访问令牌
func (m *CollectManager) ValidateAccessToken(id, token string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	req, exists := m.requests[id]
	if !exists {
		return false
	}

	return req.AccessToken == token
}

// isAllowedExt 检查扩展名是否允许
func (m *CollectManager) isAllowedExt(ext string, allowed, blocked []string) bool {
	// 检查黑名单
	for _, b := range blocked {
		if strings.EqualFold(ext, b) {
			return false
		}
	}

	// 如果白名单为空，允许所有
	if len(allowed) == 0 {
		return true
	}

	// 检查白名单
	for _, a := range allowed {
		if strings.EqualFold(ext, a) {
			return true
		}
	}

	return false
}

// classifyFile 根据扩展名分类文件
func (m *CollectManager) classifyFile(ext string) FileCategory {
	ext = strings.ToLower(ext)

	docExts := map[string]bool{
		".doc": true, ".docx": true, ".pdf": true, ".txt": true,
		".rtf": true, ".odt": true, ".xls": true, ".xlsx": true,
		".ppt": true, ".pptx": true, ".csv": true,
	}
	imgExts := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true,
		".bmp": true, ".webp": true, ".svg": true, ".ico": true,
	}
	videoExts := map[string]bool{
		".mp4": true, ".avi": true, ".mkv": true, ".mov": true,
		".wmv": true, ".flv": true, ".webm": true,
	}
	audioExts := map[string]bool{
		".mp3": true, ".wav": true, ".flac": true, ".aac": true,
		".ogg": true, ".wma": true,
	}
	archiveExts := map[string]bool{
		".zip": true, ".rar": true, ".7z": true, ".tar": true,
		".gz": true, ".bz2": true, ".xz": true,
	}
	codeExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true,
		".java": true, ".c": true, ".cpp": true, ".h": true,
		".rs": true, ".rb": true, ".php": true, ".html": true,
		".css": true, ".json": true, ".xml": true, ".yaml": true,
		".yml": true, ".toml": true, ".sh": true,
	}

	switch {
	case docExts[ext]:
		return CategoryDocument
	case imgExts[ext]:
		return CategoryImage
	case videoExts[ext]:
		return CategoryVideo
	case audioExts[ext]:
		return CategoryAudio
	case archiveExts[ext]:
		return CategoryArchive
	case codeExts[ext]:
		return CategoryCode
	default:
		return CategoryOther
	}
}

// generateID 生成随机ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// generateToken 生成访问令牌
func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GetCollectRequestByLink 通过链接获取收集请求
func (m *CollectManager) GetCollectRequestByLink(link string) (*CollectRequest, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, req := range m.requests {
		if req.ShareLink == link {
			return req, nil
		}
	}

	return nil, fmt.Errorf("收集链接不存在")
}
