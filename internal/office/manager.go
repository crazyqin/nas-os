package office

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ManagerOption 管理器选项.
type ManagerOption func(*Manager)

// WithCleanupWorker 启用/禁用清理协程.
func WithCleanupWorker(enabled bool) ManagerOption {
	return func(m *Manager) {
		if !enabled {
			m.noCleanup = true
		}
	}
}

// Manager OnlyOffice 管理器.
type Manager struct {
	mu           sync.RWMutex
	config       *Config
	sessions     map[string]*EditingSession // sessionID -> Session
	fileSessions map[string][]string        // fileID -> []sessionID（一个文件可能有多个会话）
	associations map[string]FileAssociation

	// 配置存储
	configPath string

	// 文件访问回调（用于获取文件信息）
	fileAccessor FileAccessor

	// 停止信号
	stopCh chan struct{}

	// 测试选项：禁用清理协程
	noCleanup bool
}

// FileAccessor 文件访问接口（由外部提供实现）.
type FileAccessor interface {
	// GetFileInfo 获取文件信息
	GetFileInfo(fileID string) (*FileInfo, error)
	// GetFileURL 获取文件访问 URL
	GetFileURL(fileID string) (string, error)
	// SaveFile 保存文件
	SaveFile(fileID string, reader io.Reader) error
	// GetFilePath 获取文件物理路径
	GetFilePath(fileID string) (string, error)
}

// FileInfo 文件信息.
type FileInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
	OwnerID  string `json:"owner_id"`
}

// NewManager 创建 OnlyOffice 管理器.
func NewManager(configPath string, accessor FileAccessor, opts ...ManagerOption) (*Manager, error) {
	m := &Manager{
		config:       DefaultConfig(),
		sessions:     make(map[string]*EditingSession),
		fileSessions: make(map[string][]string),
		associations: DefaultFileAssociations(),
		configPath:   configPath,
		fileAccessor: accessor,
		stopCh:       make(chan struct{}),
	}

	// 应用选项
	for _, opt := range opts {
		opt(m)
	}

	// 加载配置
	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			return nil, fmt.Errorf("加载配置失败: %w", err)
		}
	}

	// 启动会话清理协程（除非禁用）
	if !m.noCleanup {
		go m.sessionCleanupWorker()
	}

	return m, nil
}

// loadConfig 加载配置.
func (m *Manager) loadConfig() error {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}

	m.mu.Lock()
	m.config = &cfg
	m.mu.Unlock()

	return nil
}

// saveConfig 保存配置.
func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return err
	}

	m.mu.RLock()
	data, err := json.MarshalIndent(m.config, "", "  ")
	m.mu.RUnlock()

	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0640)
}

// ========== 配置管理 ==========

// GetConfig 获取配置.
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(req UpdateConfigRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.ServerURL != nil {
		m.config.ServerURL = *req.ServerURL
	}
	if req.SecretKey != nil {
		m.config.SecretKey = *req.SecretKey
	}
	if req.Enabled != nil {
		m.config.Enabled = *req.Enabled
	}
	if req.CallbackAuth != nil {
		m.config.CallbackAuth = *req.CallbackAuth
	}
	if req.EnabledTypes != nil {
		m.config.EnabledTypes = req.EnabledTypes
	}
	if req.SessionTimeout != nil {
		m.config.SessionTimeout = *req.SessionTimeout
	}
	if req.EditorConfig != nil {
		m.config.EditorConfig = *req.EditorConfig
	}

	return m.saveConfig()
}

// IsEnabled 检查是否启用.
func (m *Manager) IsEnabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.Enabled
}

// CheckServer 检查 OnlyOffice 服务器是否可达.
func (m *Manager) CheckServer() error {
	m.mu.RLock()
	serverURL := m.config.ServerURL
	m.mu.RUnlock()

	if serverURL == "" {
		return errors.New("服务器 URL 未配置")
	}

	// 发送健康检查请求
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(serverURL + "/healthcheck")
	if err != nil {
		return fmt.Errorf("服务器不可达: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("服务器返回错误状态: %d", resp.StatusCode)
	}

	return nil
}

// ========== 会话管理 ==========

// CreateSession 创建编辑会话.
func (m *Manager) CreateSession(fileID, userID, userName, mode string) (*EditingSession, *EditorInitConfig, error) {
	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return nil, nil, errors.New(ErrNotEnabled)
	}
	m.mu.RUnlock()

	// 获取文件信息
	fileInfo, err := m.fileAccessor.GetFileInfo(fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取文件信息失败: %w", err)
	}

	// 检查文件类型是否支持
	ext := strings.TrimPrefix(filepath.Ext(fileInfo.Name), ".")
	if !m.isSupported(ext) {
		return nil, nil, errors.New(ErrFileTypeNotSupported)
	}

	// 获取文件访问 URL
	fileURL, err := m.fileAccessor.GetFileURL(fileID)
	if err != nil {
		return nil, nil, fmt.Errorf("获取文件 URL 失败: %w", err)
	}

	// 生成会话 ID 和文件 Key
	sessionID := uuid.New().String()
	fileKey := m.generateFileKey(fileID)

	// 获取文档类型
	docType := m.getDocumentType(ext)

	// 获取文件路径
	filePath, _ := m.fileAccessor.GetFilePath(fileID)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 创建会话
	now := time.Now()
	timeout := m.config.SessionTimeout
	if timeout <= 0 {
		timeout = 3600
	}

	session := &EditingSession{
		ID:           sessionID,
		FileID:       fileID,
		FileName:     fileInfo.Name,
		FileKey:      fileKey,
		FilePath:     filePath,
		FileSize:     fileInfo.Size,
		UserID:       userID,
		UserName:     userName,
		StartedAt:    now,
		LastActiveAt: now,
		ExpiresAt:    now.Add(time.Duration(timeout) * time.Second),
		CallbackURL:  m.buildCallbackURL(sessionID),
		DocumentURL:  fileURL,
		Status:       SessionStatusActive,
		Metadata:     make(map[string]interface{}),
	}

	// 存储会话
	m.sessions[sessionID] = session
	m.fileSessions[fileID] = append(m.fileSessions[fileID], sessionID)

	// 构建编辑器配置（已在锁内）
	editorConfig := m.buildEditorConfigLocked(session, fileInfo, fileURL, mode, docType)

	return session, editorConfig, nil
}

// GetSession 获取会话.
func (m *Manager) GetSession(sessionID string) (*EditingSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return nil, errors.New(ErrSessionNotFound)
	}

	return session, nil
}

// GetFileSessions 获取文件的所有会话.
func (m *Manager) GetFileSessions(fileID string) []*EditingSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessionIDs := m.fileSessions[fileID]
	sessions := make([]*EditingSession, 0, len(sessionIDs))
	for _, sid := range sessionIDs {
		if s, ok := m.sessions[sid]; ok {
			sessions = append(sessions, s)
		}
	}
	return sessions
}

// ListSessions 列出所有会话.
func (m *Manager) ListSessions(status SessionStatus, limit, offset int) ([]*EditingSession, int) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*EditingSession
	for _, s := range m.sessions {
		if status == "" || s.Status == status {
			result = append(result, s)
		}
	}

	// 按开始时间倒序排序
	sortSessionsByTime(result)

	total := len(result)

	// 分页
	if offset >= len(result) {
		return []*EditingSession{}, total
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}

	return result[offset:end], total
}

// CloseSession 关闭会话.
func (m *Manager) CloseSession(sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return errors.New(ErrSessionNotFound)
	}

	session.Status = SessionStatusClosed
	session.LastActiveAt = time.Now()

	// 从文件会话列表中移除
	fileSessions := m.fileSessions[session.FileID]
	for i, sid := range fileSessions {
		if sid == sessionID {
			m.fileSessions[session.FileID] = append(fileSessions[:i], fileSessions[i+1:]...)
			break
		}
	}

	// 删除会话（可选：保留一段时间用于历史查询）
	delete(m.sessions, sessionID)

	return nil
}

// UpdateSessionStatus 更新会话状态.
func (m *Manager) UpdateSessionStatus(sessionID string, status SessionStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return errors.New(ErrSessionNotFound)
	}

	session.Status = status
	session.LastActiveAt = time.Now()

	return nil
}

// ========== 回调处理 ==========

// HandleCallback 处理 OnlyOffice 回调.
func (m *Manager) HandleCallback(sessionID string, req CallbackRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		// 会话可能已过期，但仍然需要处理保存
		return m.handleCallbackWithoutSession(req)
	}

	// 更新最后活动时间
	session.LastActiveAt = time.Now()

	switch req.Status {
	case CallbackStatusEditing:
		// 正在编辑
		session.Status = SessionStatusEditing
		return nil

	case CallbackStatusSaved, CallbackStatusForceSave:
		// 文档已保存
		return m.handleSave(session, req)

	case CallbackStatusSaving:
		// 正在保存
		session.Status = SessionStatusSaving
		return nil

	case CallbackStatusClosed:
		// 文档关闭
		session.Status = SessionStatusClosed
		return nil

	case CallbackStatusCorrupted, CallbackStatusClosedErr:
		// 错误
		session.Status = SessionStatusError
		return fmt.Errorf("文档错误: status=%d", req.Status)

	default:
		return fmt.Errorf("未知的回调状态: %d", req.Status)
	}
}

// HandleCallbackByKey 通过 Key 处理 OnlyOffice 回调
// OnlyOffice 回调通过 body 中的 key 来标识文档.
func (m *Manager) HandleCallbackByKey(req CallbackRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 通过 Key 查找会话
	var session *EditingSession
	for _, s := range m.sessions {
		if s.FileKey == req.Key {
			session = s
			break
		}
	}

	if session == nil {
		// 会话可能已过期，尝试处理保存
		return m.handleCallbackWithoutSession(req)
	}

	// 更新最后活动时间
	session.LastActiveAt = time.Now()

	switch req.Status {
	case CallbackStatusEditing:
		// 正在编辑
		session.Status = SessionStatusEditing
		return nil

	case CallbackStatusSaved, CallbackStatusForceSave:
		// 文档已保存
		return m.handleSave(session, req)

	case CallbackStatusSaving:
		// 正在保存
		session.Status = SessionStatusSaving
		return nil

	case CallbackStatusClosed:
		// 文档关闭
		session.Status = SessionStatusClosed
		return nil

	case CallbackStatusCorrupted, CallbackStatusClosedErr:
		// 错误
		session.Status = SessionStatusError
		return fmt.Errorf("文档错误: status=%d", req.Status)

	default:
		return fmt.Errorf("未知的回调状态: %d", req.Status)
	}
}

// handleSave 处理保存.
func (m *Manager) handleSave(session *EditingSession, req CallbackRequest) error {
	if req.URL == "" {
		return errors.New("保存 URL 为空")
	}

	// 下载文档
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(req.URL)
	if err != nil {
		return fmt.Errorf("下载文档失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载文档失败: status=%d", resp.StatusCode)
	}

	// 保存文档
	if err := m.fileAccessor.SaveFile(session.FileID, resp.Body); err != nil {
		return fmt.Errorf("保存文档失败: %w", err)
	}

	// 更新会话状态
	session.Status = SessionStatusActive
	session.LastActiveAt = time.Now()

	// 更新文件 Key（下次编辑生成新的 Key）
	session.FileKey = m.generateFileKey(session.FileID)

	return nil
}

// handleCallbackWithoutSession 处理没有会话的回调.
func (m *Manager) handleCallbackWithoutSession(req CallbackRequest) error {
	// 尝试通过 Key 恢复文件信息
	// 这里需要外部提供 Key 到 FileID 的映射
	return fmt.Errorf("会话不存在: key=%s", req.Key)
}

// ========== 辅助方法 ==========

// isSupported 检查文件类型是否支持.
func (m *Manager) isSupported(ext string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range m.config.EnabledTypes {
		if strings.EqualFold(t, ext) {
			return true
		}
	}
	return false
}

// getDocumentType 获取文档类型.
func (m *Manager) getDocumentType(ext string) string {
	switch strings.ToLower(ext) {
	case "doc", "docx", "docm", "dotx", "dotm", "odt", "fodt", "ott", "rtf", "txt", "html", "htm", "mht", "pdf", "djvu", "fb2", "epub", "xps", "oxps":
		return "word"
	case "xls", "xlsx", "xlsm", "xlt", "xltx", "xltm", "ods", "fods", "ots", "csv":
		return "cell"
	case "ppt", "pptx", "pptm", "pot", "potx", "potm", "odp", "fodp", "otp", "ppsx", "ppsm", "pps", "ppam":
		return "slide"
	default:
		return "word"
	}
}

// generateFileKey 生成文件 Key.
func (m *Manager) generateFileKey(fileID string) string {
	// 使用 UUID 作为文件 Key，确保唯一性
	return uuid.New().String()
}

// buildCallbackURL 构建回调 URL.
func (m *Manager) buildCallbackURL(sessionID string) string {
	// 回调 URL 由 NAS-OS 提供
	// 格式: /api/v1/office/callback/:sessionId
	return fmt.Sprintf("/api/v1/office/callback/%s", sessionID)
}

// buildEditorConfig 构建编辑器配置（公共方法，获取锁）.
func (m *Manager) buildEditorConfig(session *EditingSession, fileInfo *FileInfo, fileURL, mode, docType string) *EditorInitConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.buildEditorConfigLocked(session, fileInfo, fileURL, mode, docType)
}

// buildEditorConfigLocked 构建编辑器配置（内部方法，不获取锁，调用者需持有锁）.
func (m *Manager) buildEditorConfigLocked(session *EditingSession, fileInfo *FileInfo, fileURL, mode, docType string) *EditorInitConfig {

	// 权限配置
	canEdit := mode == "edit"
	permissions := Permissions{
		Comment:              canEdit,
		Copy:                 true,
		Download:             true,
		Edit:                 canEdit,
		ModifyFilter:         canEdit,
		ModifyContentControl: canEdit,
		Print:                true,
		Protect:              canEdit,
		Review:               canEdit,
		FillForms:            canEdit,
	}

	// 文档配置
	docConfig := DocumentConfig{
		FileType:    strings.TrimPrefix(filepath.Ext(fileInfo.Name), "."),
		Key:         session.FileKey,
		Title:       fileInfo.Name,
		URL:         fileURL,
		Permissions: permissions,
	}

	// 编辑器选项
	editorOptions := EditorOptions{
		CallbackURL: session.CallbackURL,
		Lang:        m.config.EditorConfig.Lang,
		Mode:        mode,
		User: EditorUser{
			ID:   session.UserID,
			Name: session.UserName,
		},
		Customization: CustomizationOptions{
			Forcesave:     m.config.EditorConfig.Customization.Forcesave,
			HideRightMenu: m.config.EditorConfig.Customization.HideRightMenu,
		},
	}

	// 协作模式
	if m.config.EditorConfig.CoEditing.Enabled {
		editorOptions.CoEditing = CoEditingOptions{
			Mode: "fast", // fast 或 strict
		}
	}

	config := &EditorInitConfig{
		Document:     docConfig,
		DocumentType: docType,
		Editor:       editorOptions,
		Type:         "desktop",
	}

	// 如果配置了 JWT 密钥，生成 Token
	if m.config.SecretKey != "" {
		config.Token = m.generateJWT(config)
	}

	return config
}

// generateJWT 生成 JWT Token（简化版）.
func (m *Manager) generateJWT(config *EditorInitConfig) string {
	// 注意：实际生产环境应使用 jwt-go 库
	// 这里仅作为示例，使用 HMAC-SHA256 生成签名

	data, _ := json.Marshal(config)
	h := hmac.New(sha256.New, []byte(m.config.SecretKey))
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

// ValidateCallbackToken 验证回调 Token.
func (m *Manager) ValidateCallbackToken(token string, req CallbackRequest) bool {
	if m.config.SecretKey == "" {
		return true
	}

	// 使用 HMAC 验证
	data, _ := json.Marshal(req)
	h := hmac.New(sha256.New, []byte(m.config.SecretKey))
	h.Write(data)
	expected := hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(token), []byte(expected))
}

// sessionCleanupWorker 会话清理协程.
func (m *Manager) sessionCleanupWorker() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanupExpiredSessions()
		case <-m.stopCh:
			return
		}
	}
}

// ========== 协作编辑 ==========

// collaborationSessions 协作会话存储（内存）.
var collaborationSessions = make(map[string]*CollaborationSession)
var collaborationMu sync.RWMutex

// StartCollaboration 启动实时协作编辑.
func (m *Manager) StartCollaboration(docID string) (*CollaborationSession, error) {
	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return nil, errors.New(ErrNotEnabled)
	}
	m.mu.RUnlock()

	collaborationMu.Lock()
	defer collaborationMu.Unlock()

	// 检查是否已有协作会话
	if session, exists := collaborationSessions[docID]; exists {
		if session.Status == "active" {
			return session, nil
		}
	}

	// 创建新的协作会话
	sessionID := uuid.New().String()
	now := time.Now()

	session := &CollaborationSession{
		DocID:      docID,
		SessionID:  sessionID,
		Users:      []Collaborator{},
		StartedAt:  now,
		LastActive: now,
		Status:     "active",
		Cursors:    make(map[string]Cursor),
		Locks:      []DocumentLock{},
	}

	collaborationSessions[docID] = session
	return session, nil
}

// JoinCollaboration 加入协作编辑.
func (m *Manager) JoinCollaboration(docID, userID, userName string) (*CollaborationSession, error) {
	collaborationMu.Lock()
	defer collaborationMu.Unlock()

	session, exists := collaborationSessions[docID]
	if !exists {
		return nil, errors.New(ErrCollaborationNotFound)
	}

	// 生成用户颜色
	colors := []string{"#EF4444", "#10B981", "#3B82F6", "#F59E0B", "#8B5CF6", "#EC4899"}
	color := colors[len(session.Users)%len(colors)]

	// 添加用户
	collaborator := Collaborator{
		UserID:    userID,
		UserName:  userName,
		JoinedAt:  time.Now(),
		Color:     color,
		IsEditing: false,
	}

	session.Users = append(session.Users, collaborator)
	session.LastActive = time.Now()

	return session, nil
}

// LeaveCollaboration 离开协作编辑.
func (m *Manager) LeaveCollaboration(docID, userID string) error {
	collaborationMu.Lock()
	defer collaborationMu.Unlock()

	session, exists := collaborationSessions[docID]
	if !exists {
		return errors.New(ErrCollaborationNotFound)
	}

	// 移除用户
	for i, u := range session.Users {
		if u.UserID == userID {
			session.Users = append(session.Users[:i], session.Users[i+1:]...)
			break
		}
	}

	// 移除光标
	delete(session.Cursors, userID)

	// 如果没有用户了，关闭会话
	if len(session.Users) == 0 {
		session.Status = "closed"
	}

	session.LastActive = time.Now()
	return nil
}

// GetCollaborationSession 获取协作会话.
func (m *Manager) GetCollaborationSession(docID string) (*CollaborationSession, error) {
	collaborationMu.RLock()
	defer collaborationMu.RUnlock()

	session, exists := collaborationSessions[docID]
	if !exists {
		return nil, errors.New(ErrCollaborationNotFound)
	}

	return session, nil
}

// UpdateCursor 更新用户光标位置.
func (m *Manager) UpdateCursor(docID, userID string, line, column int) error {
	collaborationMu.Lock()
	defer collaborationMu.Unlock()

	session, exists := collaborationSessions[docID]
	if !exists {
		return errors.New(ErrCollaborationNotFound)
	}

	session.Cursors[userID] = Cursor{
		UserID: userID,
		Line:   line,
		Column: column,
	}
	session.LastActive = time.Now()

	return nil
}

// ========== 版本历史 ==========

// versionStore 版本存储（模拟）.
var versionStore = make(map[string][]DocumentVersion)
var versionMu sync.RWMutex

// GetVersionHistory 获取文档版本历史.
func (m *Manager) GetVersionHistory(docID string) (*VersionHistory, error) {
	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return nil, errors.New(ErrNotEnabled)
	}
	m.mu.RUnlock()

	versionMu.RLock()
	defer versionMu.RUnlock()

	versions, exists := versionStore[docID]
	if !exists {
		// 返回空历史
		return &VersionHistory{
			DocID:      docID,
			CurrentVer: 0,
			TotalVers:  0,
			Versions:   []DocumentVersion{},
			HasMore:    false,
		}, nil
	}

	currentVer := 0
	if len(versions) > 0 {
		currentVer = versions[len(versions)-1].VersionNum
	}

	return &VersionHistory{
		DocID:      docID,
		CurrentVer: currentVer,
		TotalVers:  len(versions),
		Versions:   versions,
		HasMore:    false,
	}, nil
}

// CreateVersion 创建文档版本.
func (m *Manager) CreateVersion(docID, userID, userName, description string) (*DocumentVersion, error) {
	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return nil, errors.New(ErrNotEnabled)
	}
	m.mu.RUnlock()

	versionMu.Lock()
	defer versionMu.Unlock()

	versions := versionStore[docID]
	versionNum := len(versions) + 1

	version := DocumentVersion{
		VersionID:   uuid.New().String(),
		DocID:       docID,
		VersionNum:  versionNum,
		CreatedAt:   time.Now(),
		Description: description,
		CreatedBy: CallbackUser{
			ID:   userID,
			Name: userName,
		},
		Changes: []VersionChange{},
	}

	versions = append(versions, version)
	versionStore[docID] = versions

	return &version, nil
}

// GetVersion 获取特定版本.
func (m *Manager) GetVersion(docID, versionID string) (*DocumentVersion, error) {
	versionMu.RLock()
	defer versionMu.RUnlock()

	versions, exists := versionStore[docID]
	if !exists {
		return nil, errors.New(ErrVersionNotFound)
	}

	for _, v := range versions {
		if v.VersionID == versionID {
			return &v, nil
		}
	}

	return nil, errors.New(ErrVersionNotFound)
}

// RestoreVersion 恢复到特定版本.
func (m *Manager) RestoreVersion(docID, versionID string) error {
	versionMu.RLock()
	versions, exists := versionStore[docID]
	if !exists {
		versionMu.RUnlock()
		return errors.New(ErrVersionNotFound)
	}

	var targetVersion *DocumentVersion
	for _, v := range versions {
		if v.VersionID == versionID {
			targetVersion = &v
			break
		}
	}
	versionMu.RUnlock()

	if targetVersion == nil {
		return errors.New(ErrVersionNotFound)
	}

	// 创建恢复版本记录
	_, err := m.CreateVersion(docID, "system", "系统", fmt.Sprintf("恢复到版本 %d", targetVersion.VersionNum))
	return err
}

// ========== 文档评论 ==========

// commentStore 评论存储.
var commentStore = make(map[string][]DocumentComment)
var commentMu sync.RWMutex

// AddComment 添加文档评论.
func (m *Manager) AddComment(docID, userID, comment string) (*DocumentComment, error) {
	return m.AddCommentWithPosition(docID, userID, comment, CommentPos{})
}

// AddCommentWithPosition 添加带位置的评论.
func (m *Manager) AddCommentWithPosition(docID, userID, comment string, pos CommentPos) (*DocumentComment, error) {
	m.mu.RLock()
	if !m.config.Enabled {
		m.mu.RUnlock()
		return nil, errors.New(ErrNotEnabled)
	}
	m.mu.RUnlock()

	commentMu.Lock()
	defer commentMu.Unlock()

	comments := commentStore[docID]

	// 获取用户名（从协作会话或使用默认值）
	userName := userID
	if sess, err := m.GetCollaborationSession(docID); err == nil {
		for _, u := range sess.Users {
			if u.UserID == userID {
				userName = u.UserName
				break
			}
		}
	}

	newComment := DocumentComment{
		CommentID: uuid.New().String(),
		DocID:     docID,
		UserID:    userID,
		UserName:  userName,
		Content:   comment,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Position:  pos,
		Resolved:  false,
		Replies:   []CommentReply{},
	}

	comments = append(comments, newComment)
	commentStore[docID] = comments

	return &newComment, nil
}

// GetComments 获取文档评论列表.
func (m *Manager) GetComments(docID string) (*CommentListResponse, error) {
	commentMu.RLock()
	defer commentMu.RUnlock()

	comments := commentStore[docID]
	if comments == nil {
		comments = []DocumentComment{}
	}

	unresolved := 0
	for _, c := range comments {
		if !c.Resolved {
			unresolved++
		}
	}

	return &CommentListResponse{
		DocID:      docID,
		Total:      len(comments),
		Comments:   comments,
		Unresolved: unresolved,
	}, nil
}

// ResolveComment 解决评论.
func (m *Manager) ResolveComment(docID, commentID string) error {
	commentMu.Lock()
	defer commentMu.Unlock()

	comments := commentStore[docID]
	for i := range comments {
		if comments[i].CommentID == commentID {
			comments[i].Resolved = true
			comments[i].UpdatedAt = time.Now()
			commentStore[docID] = comments
			return nil
		}
	}

	return errors.New(ErrCommentNotFound)
}

// ReplyComment 回复评论.
func (m *Manager) ReplyComment(docID, commentID, userID, reply string) error {
	commentMu.Lock()
	defer commentMu.Unlock()

	comments := commentStore[docID]
	for i := range comments {
		if comments[i].CommentID == commentID {
			// 获取用户名
			userName := userID
			if sess, err := m.GetCollaborationSession(docID); err == nil {
				for _, u := range sess.Users {
					if u.UserID == userID {
						userName = u.UserName
						break
					}
				}
			}

			comments[i].Replies = append(comments[i].Replies, CommentReply{
				ReplyID:   uuid.New().String(),
				UserID:    userID,
				UserName:  userName,
				Content:   reply,
				CreatedAt: time.Now(),
			})
			comments[i].UpdatedAt = time.Now()
			commentStore[docID] = comments
			return nil
		}
	}

	return errors.New(ErrCommentNotFound)
}

// DeleteComment 删除评论.
func (m *Manager) DeleteComment(docID, commentID string) error {
	commentMu.Lock()
	defer commentMu.Unlock()

	comments := commentStore[docID]
	for i := range comments {
		if comments[i].CommentID == commentID {
			comments = append(comments[:i], comments[i+1:]...)
			commentStore[docID] = comments
			return nil
		}
	}

	return errors.New(ErrCommentNotFound)
}

// Close 关闭管理器.
func (m *Manager) Close() {
	close(m.stopCh)
}

// cleanupExpiredSessions 清理过期会话.
func (m *Manager) cleanupExpiredSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, session := range m.sessions {
		if session.IsExpired() {
			session.Status = SessionStatusExpired
			delete(m.sessions, id)

			// 从文件会话列表中移除
			fileSessions := m.fileSessions[session.FileID]
			for i, sid := range fileSessions {
				if sid == id {
					m.fileSessions[session.FileID] = append(fileSessions[:i], fileSessions[i+1:]...)
					break
				}
			}
		}
	}
}

// GetFileAssociation 获取文件关联.
func (m *Manager) GetFileAssociation(ext string) (FileAssociation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	assoc, ok := m.associations[strings.ToLower(ext)]
	return assoc, ok
}

// GetAllFileAssociations 获取所有文件关联.
func (m *Manager) GetAllFileAssociations() map[string]FileAssociation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 返回副本
	result := make(map[string]FileAssociation, len(m.associations))
	for k, v := range m.associations {
		result[k] = v
	}
	return result
}

// ParseCallbackURL 从回调 URL 解析会话 ID.
func ParseCallbackURL(callbackURL string) (string, error) {
	// 格式: /api/v1/office/callback/:sessionId
	u, err := url.Parse(callbackURL)
	if err != nil {
		return "", err
	}

	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	// ["api", "v1", "office", "callback", "sessionId"]
	if len(parts) < 5 || parts[3] != "callback" {
		return "", errors.New("无效的回调 URL 格式")
	}

	return parts[4], nil
}

// sortSessionsByTime 按时间排序会话.
func sortSessionsByTime(sessions []*EditingSession) {
	for i := 0; i < len(sessions)-1; i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[i].StartedAt.Before(sessions[j].StartedAt) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}
}
