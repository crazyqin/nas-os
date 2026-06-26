// Package websharepro - 协作功能模块
// 参考群晖 DSM 7.3 协作特性：
// - FileRequest：文件收集请求（生成链接让别人上传文件到指定目录）
// - SharedLabel：共享标签（团队文件标签管理）
// - FileComment：文件评论/备注
package websharepro

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// FileRequest - 文件收集请求
// ---------------------------------------------------------------------------

// FileRequestStatus 文件收集请求状态
type FileRequestStatus string

const (
	RequestActive  FileRequestStatus = "active"
	RequestPaused  FileRequestStatus = "paused"
	RequestClosed  FileRequestStatus = "closed"
	RequestExpired FileRequestStatus = "expired"
)

// UploadConstraint 上传约束
type UploadConstraint struct {
	MaxFileSize     int64    `json:"maxFileSize"`     // 单文件最大字节数，0=无限制
	MaxTotalSize    int64    `json:"maxTotalSize"`    // 总大小上限
	AllowedTypes    []string `json:"allowedTypes"`    // 允许的 MIME 类型，空=全部
	MaxFilesPerUser int      `json:"maxFilesPerUser"` // 每用户最大文件数，0=无限制
}

// FileRequest 文件收集请求
type FileRequest struct {
	ID          string `json:"id"`
	Token       string `json:"token"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	// 收集目标目录
	TargetDir string `json:"targetDir"`
	// 上传约束
	Constraints UploadConstraint `json:"constraints"`
	// 密码保护（可选）
	Password string `json:"password,omitempty"`
	HasPwd   bool   `json:"hasPwd"`
	// 有效期
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	// 上传次数限制
	MaxUploads  int `json:"maxUploads"` // 0=无限制
	UploadCount int `json:"uploadCount"`
	// 请求发起人
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	// 状态
	Status FileRequestStatus `json:"status"`
	// 上传记录
	Uploads []UploadRecord `json:"uploads,omitempty"`
	// 通知配置
	NotifyOnUpload bool   `json:"notifyOnUpload"`
	NotifyEmail    string `json:"notifyEmail,omitempty"`
	// 自定义 slug
	CustomSlug string `json:"customSlug,omitempty"`
	PublicURL  string `json:"publicUrl,omitempty"`
}

// UploadRecord 上传记录
type UploadRecord struct {
	ID         string    `json:"id"`
	FileName   string    `json:"fileName"`
	FileSize   int64     `json:"fileSize"`
	MimeType   string    `json:"mimeType"`
	UploadedBy string    `json:"uploadedBy"` // 匿名用户可为空
	RemoteIP   string    `json:"remoteIp"`
	UploadedAt time.Time `json:"uploadedAt"`
	Status     string    `json:"status"` // pending, accepted, rejected
}

// ---------------------------------------------------------------------------
// SharedLabel - 共享标签
// ---------------------------------------------------------------------------

// LabelColor 标签颜色
type LabelColor string

const (
	LabelRed    LabelColor = "red"
	LabelOrange LabelColor = "orange"
	LabelYellow LabelColor = "yellow"
	LabelGreen  LabelColor = "green"
	LabelBlue   LabelColor = "blue"
	LabelPurple LabelColor = "purple"
	LabelGray   LabelColor = "gray"
)

// SharedLabel 共享标签
type SharedLabel struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Color       LabelColor `json:"color"`
	Description string     `json:"description,omitempty"`
	// 标签创建者
	CreatedBy string    `json:"createdBy"`
	CreatedAt time.Time `json:"createdAt"`
	// 关联的文件路径列表
	FilePaths []string `json:"filePaths"`
	// 标签可见性：team（团队可见）/ private（仅创建者）
	Visibility string `json:"visibility"`
}

// ---------------------------------------------------------------------------
// FileComment - 文件评论
// ---------------------------------------------------------------------------

// FileComment 文件评论/备注
type FileComment struct {
	ID        string    `json:"id"`
	FilePath  string    `json:"filePath"`
	ParentID  string    `json:"parentId,omitempty"` // 回复的评论 ID
	Author    string    `json:"author"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Resolved  bool      `json:"resolved"` // 是否已解决
	Pinned    bool      `json:"pinned"`   // 是否置顶
	// 位置标注（可选，标注在文件预览的某个位置）
	Position *CommentPosition `json:"position,omitempty"`
	// 回复
	Replies []*FileComment `json:"replies,omitempty"`
}

// CommentPosition 评论位置标注
type CommentPosition struct {
	Page   int     `json:"page,omitempty"`   // PDF/文档页码
	X      float64 `json:"x,omitempty"`      // X 坐标
	Y      float64 `json:"y,omitempty"`      // Y 坐标
	Width  float64 `json:"width,omitempty"`  // 标注区域宽
	Height float64 `json:"height,omitempty"` // 标注区域高
}

// ---------------------------------------------------------------------------
// CollaborationManager 协作管理器
// ---------------------------------------------------------------------------

// CollaborationManager 协作功能管理器
type CollaborationManager struct {
	mu          sync.RWMutex
	requests    map[string]*FileRequest   // id -> request
	reqTokenIdx map[string]string         // token -> id
	labels      map[string]*SharedLabel   // id -> label
	comments    map[string][]*FileComment // filePath -> comments
	commentIdx  map[string]*FileComment   // id -> comment
	config      *CollaborationConfig
	logger      *zap.Logger
}

// CollaborationConfig 协作配置
type CollaborationConfig struct {
	DefaultFileRequestExpiryHours int    `json:"defaultFileRequestExpiryHours"`
	MaxUploadSize                 int64  `json:"maxUploadSize"`
	DefaultLabelVisibility        string `json:"defaultLabelVisibility"`
	BaseURL                       string `json:"baseUrl"`
}

// NewCollaborationManager 创建协作管理器
func NewCollaborationManager(config *CollaborationConfig, logger *zap.Logger) *CollaborationManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = &CollaborationConfig{
			DefaultFileRequestExpiryHours: 168,                    // 7 天
			MaxUploadSize:                 5 * 1024 * 1024 * 1024, // 5GB
			DefaultLabelVisibility:        "team",
		}
	}
	return &CollaborationManager{
		requests:    make(map[string]*FileRequest),
		reqTokenIdx: make(map[string]string),
		labels:      make(map[string]*SharedLabel),
		comments:    make(map[string][]*FileComment),
		commentIdx:  make(map[string]*FileComment),
		config:      config,
		logger:      logger,
	}
}

// ---------------------------------------------------------------------------
// FileRequest 操作
// ---------------------------------------------------------------------------

// CreateFileRequest 创建文件收集请求
func (cm *CollaborationManager) CreateFileRequest(title, targetDir, createdBy string, constraints UploadConstraint, password string, expiryHours int, customSlug string) (*FileRequest, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if title == "" {
		return nil, fmt.Errorf("title is required")
	}
	if targetDir == "" {
		return nil, fmt.Errorf("targetDir is required")
	}

	id := generateShareID()
	token := generateToken()

	req := &FileRequest{
		ID:             id,
		Token:          token,
		Title:          title,
		TargetDir:      targetDir,
		Constraints:    constraints,
		Password:       password,
		HasPwd:         password != "",
		CreatedBy:      createdBy,
		CreatedAt:      time.Now(),
		Status:         RequestActive,
		Uploads:        make([]UploadRecord, 0),
		NotifyOnUpload: true,
		CustomSlug:     customSlug,
	}

	// 有效期
	if expiryHours > 0 {
		t := time.Now().Add(time.Duration(expiryHours) * time.Hour)
		req.ExpiresAt = &t
	} else if cm.config.DefaultFileRequestExpiryHours > 0 {
		t := time.Now().Add(time.Duration(cm.config.DefaultFileRequestExpiryHours) * time.Hour)
		req.ExpiresAt = &t
	}

	// 构建 URL
	req.PublicURL = cm.buildRequestURL(req)

	cm.requests[id] = req
	cm.reqTokenIdx[token] = id

	cm.logger.Info("file request created",
		zap.String("id", id),
		zap.String("targetDir", targetDir),
		zap.String("createdBy", createdBy),
	)
	return req, nil
}

// GetFileRequest 获取文件收集请求
func (cm *CollaborationManager) GetFileRequest(id string) (*FileRequest, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	req, ok := cm.requests[id]
	if !ok {
		return nil, false
	}
	// 自动过期
	if req.Status == RequestActive && req.ExpiresAt != nil && req.ExpiresAt.Before(time.Now()) {
		req.Status = RequestExpired
	}
	return req, true
}

// GetFileRequestByToken 通过 Token 获取请求
func (cm *CollaborationManager) GetFileRequestByToken(token string) (*FileRequest, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	id, ok := cm.reqTokenIdx[token]
	if !ok {
		return nil, false
	}
	req, ok := cm.requests[id]
	if !ok {
		return nil, false
	}
	if req.Status != RequestActive {
		return nil, false
	}
	if req.ExpiresAt != nil && req.ExpiresAt.Before(time.Now()) {
		req.Status = RequestExpired
		return nil, false
	}
	return req, true
}

// ValidateFileRequestUpload 验证上传请求
func (cm *CollaborationManager) ValidateFileRequestUpload(reqID, password string, fileSize int64, mimeType string) error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	req, ok := cm.requests[reqID]
	if !ok {
		return ErrLinkNotFound
	}

	if req.Status != RequestActive {
		return fmt.Errorf("request is %s", req.Status)
	}

	// 密码验证
	if req.HasPwd && req.Password != password {
		return fmt.Errorf("incorrect password")
	}

	// 检查上传次数限制
	if req.MaxUploads > 0 && req.UploadCount >= req.MaxUploads {
		return fmt.Errorf("upload limit reached")
	}

	// 文件大小检查
	if req.Constraints.MaxFileSize > 0 && fileSize > req.Constraints.MaxFileSize {
		return fmt.Errorf("file too large (%d > %d)", fileSize, req.Constraints.MaxFileSize)
	}

	// MIME 类型检查
	if len(req.Constraints.AllowedTypes) > 0 {
		allowed := false
		for _, t := range req.Constraints.AllowedTypes {
			if t == mimeType || t == "*" {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("file type %s not allowed", mimeType)
		}
	}

	return nil
}

// RecordFileUpload 记录文件上传
func (cm *CollaborationManager) RecordFileUpload(reqID, fileName string, fileSize int64, mimeType, uploadedBy, remoteIP string) (*UploadRecord, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	req, ok := cm.requests[reqID]
	if !ok {
		return nil, ErrLinkNotFound
	}

	record := UploadRecord{
		ID:         generateShareID(),
		FileName:   fileName,
		FileSize:   fileSize,
		MimeType:   mimeType,
		UploadedBy: uploadedBy,
		RemoteIP:   remoteIP,
		UploadedAt: time.Now(),
		Status:     "accepted",
	}

	req.Uploads = append(req.Uploads, record)
	req.UploadCount++

	cm.logger.Info("file uploaded to request",
		zap.String("requestId", reqID),
		zap.String("fileName", fileName),
		zap.Int64("fileSize", fileSize),
	)
	return &record, nil
}

// CloseFileRequest 关闭文件收集请求
func (cm *CollaborationManager) CloseFileRequest(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	req, ok := cm.requests[id]
	if !ok {
		return ErrLinkNotFound
	}
	req.Status = RequestClosed
	cm.logger.Info("file request closed", zap.String("id", id))
	return nil
}

// ListFileRequests 列出文件收集请求
func (cm *CollaborationManager) ListFileRequests(createdBy string, status FileRequestStatus) []*FileRequest {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []*FileRequest
	for _, req := range cm.requests {
		if createdBy != "" && req.CreatedBy != createdBy {
			continue
		}
		if status != "" && req.Status != status {
			continue
		}
		result = append(result, req)
	}
	return result
}

// ---------------------------------------------------------------------------
// SharedLabel 操作
// ---------------------------------------------------------------------------

// CreateLabel 创建共享标签
func (cm *CollaborationManager) CreateLabel(name string, color LabelColor, description, createdBy string, filePaths []string, visibility string) (*SharedLabel, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if name == "" {
		return nil, fmt.Errorf("label name is required")
	}

	id := generateShareID()
	if visibility == "" {
		visibility = cm.config.DefaultLabelVisibility
	}

	label := &SharedLabel{
		ID:          id,
		Name:        name,
		Color:       color,
		Description: description,
		CreatedBy:   createdBy,
		CreatedAt:   time.Now(),
		FilePaths:   filePaths,
		Visibility:  visibility,
	}

	if label.FilePaths == nil {
		label.FilePaths = make([]string, 0)
	}

	cm.labels[id] = label
	cm.logger.Info("label created", zap.String("id", id), zap.String("name", name))
	return label, nil
}

// GetLabel 获取标签
func (cm *CollaborationManager) GetLabel(id string) (*SharedLabel, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	label, ok := cm.labels[id]
	return label, ok
}

// AddFilesToLabel 向标签添加文件
func (cm *CollaborationManager) AddFilesToLabel(id string, paths []string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	label, ok := cm.labels[id]
	if !ok {
		return ErrLinkNotFound
	}

	// 去重
	existing := make(map[string]bool, len(label.FilePaths))
	for _, p := range label.FilePaths {
		existing[p] = true
	}

	for _, p := range paths {
		if !existing[p] {
			label.FilePaths = append(label.FilePaths, p)
			existing[p] = true
		}
	}
	return nil
}

// RemoveFilesFromLabel 从标签移除文件
func (cm *CollaborationManager) RemoveFilesFromLabel(id string, paths []string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	label, ok := cm.labels[id]
	if !ok {
		return ErrLinkNotFound
	}

	removeSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		removeSet[p] = true
	}

	var filtered []string
	for _, p := range label.FilePaths {
		if !removeSet[p] {
			filtered = append(filtered, p)
		}
	}
	label.FilePaths = filtered
	return nil
}

// DeleteLabel 删除标签
func (cm *CollaborationManager) DeleteLabel(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if _, ok := cm.labels[id]; !ok {
		return ErrLinkNotFound
	}
	delete(cm.labels, id)
	cm.logger.Info("label deleted", zap.String("id", id))
	return nil
}

// ListLabels 列出标签
func (cm *CollaborationManager) ListLabels(createdBy, visibility string) []*SharedLabel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []*SharedLabel
	for _, label := range cm.labels {
		if createdBy != "" && label.CreatedBy != createdBy {
			continue
		}
		if visibility != "" && label.Visibility != visibility {
			continue
		}
		result = append(result, label)
	}
	return result
}

// GetLabelsForFile 获取文件关联的所有标签
func (cm *CollaborationManager) GetLabelsForFile(filePath string) []*SharedLabel {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var result []*SharedLabel
	for _, label := range cm.labels {
		for _, p := range label.FilePaths {
			if p == filePath {
				result = append(result, label)
				break
			}
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// FileComment 操作
// ---------------------------------------------------------------------------

// CreateComment 创建文件评论
func (cm *CollaborationManager) CreateComment(filePath, author, content, parentID string, position *CommentPosition) (*FileComment, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if filePath == "" {
		return nil, fmt.Errorf("filePath is required")
	}
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}

	id := generateShareID()
	now := time.Now()

	comment := &FileComment{
		ID:        id,
		FilePath:  filePath,
		ParentID:  parentID,
		Author:    author,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
		Position:  position,
		Replies:   make([]*FileComment, 0),
	}

	// 如果是回复
	if parentID != "" {
		parent, ok := cm.commentIdx[parentID]
		if !ok {
			return nil, fmt.Errorf("parent comment not found: %s", parentID)
		}
		parent.Replies = append(parent.Replies, comment)
	} else {
		cm.comments[filePath] = append(cm.comments[filePath], comment)
	}

	cm.commentIdx[id] = comment

	cm.logger.Info("comment created",
		zap.String("id", id),
		zap.String("filePath", filePath),
		zap.String("author", author),
	)
	return comment, nil
}

// GetComment 获取评论
func (cm *CollaborationManager) GetComment(id string) (*FileComment, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	c, ok := cm.commentIdx[id]
	return c, ok
}

// UpdateComment 更新评论内容
func (cm *CollaborationManager) UpdateComment(id, content string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	comment, ok := cm.commentIdx[id]
	if !ok {
		return ErrLinkNotFound
	}
	comment.Content = content
	comment.UpdatedAt = time.Now()
	return nil
}

// ResolveComment 标记评论为已解决
func (cm *CollaborationManager) ResolveComment(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	comment, ok := cm.commentIdx[id]
	if !ok {
		return ErrLinkNotFound
	}
	comment.Resolved = true
	comment.UpdatedAt = time.Now()
	return nil
}

// PinComment 置顶评论
func (cm *CollaborationManager) PinComment(id string, pinned bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	comment, ok := cm.commentIdx[id]
	if !ok {
		return ErrLinkNotFound
	}
	comment.Pinned = pinned
	comment.UpdatedAt = time.Now()
	return nil
}

// DeleteComment 删除评论
func (cm *CollaborationManager) DeleteComment(id string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	comment, ok := cm.commentIdx[id]
	if !ok {
		return ErrLinkNotFound
	}

	// 从文件评论列表中移除
	comments := cm.comments[comment.FilePath]
	for i, c := range comments {
		if c.ID == id {
			cm.comments[comment.FilePath] = append(comments[:i], comments[i+1:]...)
			break
		}
	}

	// 如果是顶级评论，从索引中移除所有子评论
	if comment.ParentID == "" {
		for _, reply := range comment.Replies {
			delete(cm.commentIdx, reply.ID)
		}
	}

	delete(cm.commentIdx, id)
	cm.logger.Info("comment deleted", zap.String("id", id))
	return nil
}

// ListComments 列出文件的评论
func (cm *CollaborationManager) ListComments(filePath string, includeResolved bool) []*FileComment {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	comments := cm.comments[filePath]
	result := make([]*FileComment, 0, len(comments))

	for _, c := range comments {
		if !includeResolved && c.Resolved {
			continue
		}
		result = append(result, c)
	}
	return result
}

// GetCommentCount 获取文件的评论数量
func (cm *CollaborationManager) GetCommentCount(filePath string) int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return len(cm.comments[filePath])
}

// ---------------------------------------------------------------------------
// 内部辅助
// ---------------------------------------------------------------------------

// buildRequestURL 构建文件收集链接 URL
func (cm *CollaborationManager) buildRequestURL(req *FileRequest) string {
	base := cm.config.BaseURL
	if base == "" {
		base = "http://localhost:8080"
	}
	if req.CustomSlug != "" {
		return base + "/upload/" + req.CustomSlug
	}
	return base + "/upload/" + req.Token
}
