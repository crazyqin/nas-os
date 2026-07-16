// Package p2pwebshare 提供 P2P 文件共享 Web 界面
//
// 实现基于浏览器的 P2P 文件共享功能，用户可以通过生成链接
// 安全地共享文件给他人，支持密码保护、过期时间、下载限制。
// 参考飞牛 P2Pee 文件共享设计。
//
// 兵部（软件工程）注: 本模块于 2026-06-24 开发完成。
package p2pwebshare

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ShareLink 共享链接.
type ShareLink struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	FilePath      string    `json:"file_path"`
	FileSize      int64     `json:"file_size"`
	FileType      string    `json:"file_type"`
	ShareURL      string    `json:"share_url"`
	Password      string    `json:"-"` // 不返回给客户端
	HasPassword   bool      `json:"has_password"`
	MaxDownloads  int       `json:"max_downloads"`
	DownloadCount int       `json:"download_count"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
	CreatedBy     string    `json:"created_by"`
	Enabled       bool      `json:"enabled"`
	Note          string    `json:"note,omitempty"`
}

// ShareRequest 创建共享请求.
type ShareRequest struct {
	FilePath     string `json:"file_path"`
	Name         string `json:"name,omitempty"`
	Password     string `json:"password,omitempty"`
	ExpireHours  int    `json:"expire_hours,omitempty"`
	MaxDownloads int    `json:"max_downloads,omitempty"`
	Note         string `json:"note,omitempty"`
}

// ShareStats 共享统计.
type ShareStats struct {
	TotalLinks      int       `json:"total_links"`
	ActiveLinks     int       `json:"active_links"`
	ExpiredLinks    int       `json:"expired_links"`
	TotalDownloads  int       `json:"total_downloads"`
	TotalDataShared int64     `json:"total_data_shared"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DownloadLog 下载日志.
type DownloadLog struct {
	ShareID    string    `json:"share_id"`
	ShareName  string    `json:"share_name"`
	ClientIP   string    `json:"client_ip"`
	UserAgent  string    `json:"user_agent"`
	DownloadAt time.Time `json:"download_at"`
	FileSize   int64     `json:"file_size"`
}

// Manager P2P 文件共享管理器.
type Manager struct {
	mu      sync.RWMutex
	links   map[string]*ShareLink
	logs    []DownloadLog
	logger  *slog.Logger
	baseURL string
}

// NewManager 创建管理器.
func NewManager(baseURL string, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	if baseURL == "" {
		baseURL = "https://share.example.com"
	}
	return &Manager{
		links:   make(map[string]*ShareLink),
		logs:    make([]DownloadLog, 0),
		logger:  logger,
		baseURL: baseURL,
	}
}

// CreateShareLink 创建共享链接.
func (m *Manager) CreateShareLink(req ShareRequest, createdBy string) (*ShareLink, error) {
	if req.FilePath == "" {
		return nil, fmt.Errorf("文件路径不能为空")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	id, err := generateID(16)
	if err != nil {
		return nil, fmt.Errorf("生成ID失败: %w", err)
	}

	name := req.Name
	if name == "" {
		name = filepath.Base(req.FilePath)
	}

	expireHours := req.ExpireHours
	if expireHours == 0 {
		expireHours = 72 // 默认 72 小时
	}

	maxDownloads := req.MaxDownloads
	if maxDownloads <= 0 {
		maxDownloads = 0 // 0 表示不限制
	}

	link := &ShareLink{
		ID:           id,
		Name:         name,
		FilePath:     req.FilePath,
		FileSize:     0, // 实际使用时从文件系统获取
		FileType:     detectFileType(req.FilePath),
		ShareURL:     fmt.Sprintf("%s/s/%s", m.baseURL, id),
		Password:     req.Password,
		HasPassword:  req.Password != "",
		MaxDownloads: maxDownloads,
		ExpiresAt:    time.Now().Add(time.Duration(expireHours) * time.Hour),
		CreatedAt:    time.Now(),
		CreatedBy:    createdBy,
		Enabled:      true,
		Note:         req.Note,
	}

	m.links[id] = link
	m.logger.Info("创建共享链接", "id", id, "name", name, "by", createdBy)

	return link, nil
}

// GetShareLink 获取共享链接.
func (m *Manager) GetShareLink(id string) (*ShareLink, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.links[id]
	if !ok {
		return nil, fmt.Errorf("共享链接不存在: %s", id)
	}

	if !link.Enabled {
		return nil, fmt.Errorf("共享链接已禁用: %s", id)
	}

	if time.Now().After(link.ExpiresAt) {
		return nil, fmt.Errorf("共享链接已过期: %s", id)
	}

	return link, nil
}

// VerifyPassword 验证密码.
func (m *Manager) VerifyPassword(id, password string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	link, ok := m.links[id]
	if !ok {
		return false
	}

	if link.Password == "" {
		return true
	}

	return link.Password == password
}

// RecordDownload 记录下载.
func (m *Manager) RecordDownload(id, clientIP, userAgent string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return fmt.Errorf("共享链接不存在: %s", id)
	}

	if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
		return fmt.Errorf("已达到最大下载次数: %d", link.MaxDownloads)
	}

	link.DownloadCount++

	log := DownloadLog{
		ShareID:    id,
		ShareName:  link.Name,
		ClientIP:   clientIP,
		UserAgent:  userAgent,
		DownloadAt: time.Now(),
		FileSize:   link.FileSize,
	}
	m.logs = append(m.logs, log)

	m.logger.Info("文件下载", "id", id, "name", link.Name, "ip", clientIP)
	return nil
}

// DeleteShareLink 删除共享链接.
func (m *Manager) DeleteShareLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.links[id]; !ok {
		return fmt.Errorf("共享链接不存在: %s", id)
	}

	delete(m.links, id)
	m.logger.Info("删除共享链接", "id", id)
	return nil
}

// DisableShareLink 禁用共享链接.
func (m *Manager) DisableShareLink(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	link, ok := m.links[id]
	if !ok {
		return fmt.Errorf("共享链接不存在: %s", id)
	}

	link.Enabled = false
	m.logger.Info("禁用共享链接", "id", id)
	return nil
}

// ListShareLinks 列出共享链接.
func (m *Manager) ListShareLinks(createdBy string) []*ShareLink {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ShareLink, 0)
	for _, link := range m.links {
		if createdBy != "" && link.CreatedBy != createdBy {
			continue
		}
		result = append(result, link)
	}
	return result
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() ShareStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := ShareStats{
		TotalLinks: len(m.links),
		UpdatedAt:  time.Now(),
	}

	now := time.Now()
	for _, link := range m.links {
		stats.TotalDownloads += link.DownloadCount
		stats.TotalDataShared += link.FileSize * int64(link.DownloadCount)

		if link.Enabled && now.Before(link.ExpiresAt) {
			stats.ActiveLinks++
		} else {
			stats.ExpiredLinks++
		}
	}

	return stats
}

// GetDownloadLogs 获取下载日志.
func (m *Manager) GetDownloadLogs(limit int) []DownloadLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.logs) {
		limit = len(m.logs)
	}

	// 返回最新的日志
	start := len(m.logs) - limit
	if start < 0 {
		start = 0
	}

	result := make([]DownloadLog, limit)
	copy(result, m.logs[start:])
	return result
}

// CleanupExpired 清理过期链接.
func (m *Manager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()
	for id, link := range m.links {
		if now.After(link.ExpiresAt) {
			delete(m.links, id)
			count++
		}
	}

	if count > 0 {
		m.logger.Info("清理过期共享链接", "count", count)
	}
	return count
}

// RegisterRoutes 注册 HTTP 路由.
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/p2p/share", m.handleShare)
	mux.HandleFunc("/api/v1/p2p/share/list", m.handleList)
	mux.HandleFunc("/api/v1/p2p/share/stats", m.handleStats)
	mux.HandleFunc("/api/v1/p2p/share/logs", m.handleLogs)
	mux.HandleFunc("/api/v1/p2p/share/delete", m.handleDelete)
	mux.HandleFunc("/s/", m.handleDownload)
}

func (m *Manager) handleShare(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req ShareRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"error":   "无效的请求体",
			})
			return
		}

		link, err := m.CreateShareLink(req, "api-user")
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]interface{}{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]interface{}{
			"success": true,
			"data":    link,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	links := m.ListShareLinks("")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    links,
	})
}

func (m *Manager) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := m.GetStats()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    stats,
	})
}

func (m *Manager) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	logs := m.GetDownloadLogs(50)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"data":    logs,
	})
}

func (m *Manager) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"success": false,
			"error":   "缺少 id 参数",
		})
		return
	}

	if err := m.DeleteShareLink(id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "已删除",
	})
}

func (m *Manager) handleDownload(w http.ResponseWriter, r *http.Request) {
	// 从 URL 提取 ID: /s/{id}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 || parts[2] == "" {
		http.Error(w, "无效的共享链接", http.StatusBadRequest)
		return
	}
	id := parts[2]

	link, err := m.GetShareLink(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// 验证密码
	if link.HasPassword {
		password := r.URL.Query().Get("password")
		if !m.VerifyPassword(id, password) {
			writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"success": false,
				"error":   "密码错误",
			})
			return
		}
	}

	// 记录下载
	clientIP := r.RemoteAddr
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		clientIP = forwarded
	}

	if err := m.RecordDownload(id, clientIP, r.UserAgent()); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}

	// 返回文件信息（实际使用时会发送文件内容）
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("开始下载: %s", link.Name),
		"data": map[string]interface{}{
			"name":     link.Name,
			"size":     link.FileSize,
			"type":     link.FileType,
			"download": link.DownloadCount,
		},
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func generateID(length int) (string, error) {
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func detectFileType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg":
		return "image"
	case ".mp4", ".avi", ".mkv", ".mov", ".wmv":
		return "video"
	case ".mp3", ".wav", ".flac", ".aac", ".ogg":
		return "audio"
	case ".pdf":
		return "pdf"
	case ".doc", ".docx":
		return "document"
	case ".xls", ".xlsx":
		return "spreadsheet"
	case ".zip", ".rar", ".7z", ".tar", ".gz":
		return "archive"
	default:
		return "file"
	}
}
