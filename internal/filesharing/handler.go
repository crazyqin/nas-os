// Package filesharing 提供高级文件分享功能的 HTTP API Handler
package filesharing

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Handler 文件分享 HTTP Handler
type Handler struct {
	manager *FileSharingManager
}

// NewHandler 创建 Handler
func NewHandler(manager *FileSharingManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/sharing/links", h.handleLinks)
	mux.HandleFunc("/api/sharing/links/", h.handleLinkByID)
	mux.HandleFunc("/api/sharing/s/", h.handlePublicAccess)
	mux.HandleFunc("/api/sharing/stats", h.handleStats)
}

// handleLinks 处理分享链接列表和创建
func (h *Handler) handleLinks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listLinks(w, r)
	case http.MethodPost:
		h.createLink(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleLinkByID 处理单个链接操作
func (h *Handler) handleLinkByID(w http.ResponseWriter, r *http.Request) {
	// 解析 ID
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/sharing/links/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing link ID", http.StatusBadRequest)
		return
	}

	id := parts[0]

	// 检查是否是 qrcode 请求
	if len(parts) > 1 && parts[1] == "qrcode" {
		h.generateQRCode(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getLink(w, r, id)
	case http.MethodPut:
		h.updateLink(w, r, id)
	case http.MethodDelete:
		h.deleteLink(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePublicAccess 处理公开访问
func (h *Handler) handlePublicAccess(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/sharing/s/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	tokenOrSlug := parts[0]

	// 检查子路径
	if len(parts) > 1 {
		action := parts[1]
		switch action {
		case "upload":
			h.handleUpload(w, r, tokenOrSlug)
			return
		case "download":
			h.handleDownload(w, r, tokenOrSlug)
			return
		}
	}

	// 默认是访问查看
	h.handleView(w, r, tokenOrSlug)
}

// handleStats 处理统计请求
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	createdBy := r.URL.Query().Get("createdBy")
	stats := h.manager.GetStats(createdBy)

	writeJSON(w, http.StatusOK, stats)
}

// ================== 链接操作 ==================

// createLink 创建分享链接
func (h *Handler) createLink(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path         string            `json:"path"`
		Password     string            `json:"password"`
		ExpireHours  int               `json:"expireHours"`
		MaxDownloads int               `json:"maxDownloads"`
		Permissions  []SharePermission `json:"permissions"`
		AllowUpload  bool              `json:"allowUpload"`
		CustomSlug   string            `json:"customSlug"`
		Description  string            `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	// 获取用户 ID (从认证中间件)
	userID := r.Header.Get("X-User-ID")

	// 默认过期时间 7 天
	if req.ExpireHours == 0 {
		req.ExpireHours = 168 // 7 days
	}

	link, err := h.manager.CreateShareLink(
		req.Path,
		req.Password,
		userID,
		req.Permissions,
		req.ExpireHours,
		req.MaxDownloads,
		req.AllowUpload,
		req.CustomSlug,
		req.Description,
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 生成访问 URL
	slug := link.CustomSlug
	if slug == "" {
		slug = link.Token
	}
	accessURL := fmt.Sprintf("/api/sharing/s/%s", slug)

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"link":      link,
		"accessURL": accessURL,
	})
}

// listLinks 列出分享链接
func (h *Handler) listLinks(w http.ResponseWriter, r *http.Request) {
	createdBy := r.URL.Query().Get("createdBy")
	status := ShareStatus(r.URL.Query().Get("status"))

	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}

	links, total := h.manager.ListShareLinks(createdBy, status, limit, offset)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"links":  links,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// getLink 获取链接详情
func (h *Handler) getLink(w http.ResponseWriter, r *http.Request, id string) {
	link, err := h.manager.GetShareLinkByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// 获取详细统计
	stats, _ := h.manager.GetLinkStats(id)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"link":  link,
		"stats": stats,
	})
}

// updateLink 更新链接
func (h *Handler) updateLink(w http.ResponseWriter, r *http.Request, id string) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	link, err := h.manager.UpdateShareLink(id, updates)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusOK, link)
}

// deleteLink 删除链接
func (h *Handler) deleteLink(w http.ResponseWriter, r *http.Request, id string) {
	// 支持撤销（软删除）和彻底删除
	action := r.URL.Query().Get("action")

	if action == "revoke" {
		if err := h.manager.RevokeShareLink(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Link revoked"})
	} else {
		if err := h.manager.DeleteShareLink(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Link deleted"})
	}
}

// ================== 公开访问 ==================

// handleView 处理查看请求
func (h *Handler) handleView(w http.ResponseWriter, r *http.Request, tokenOrSlug string) {
	link, err := h.manager.GetShareLink(tokenOrSlug)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// 验证密码
	if link.Password != "" {
		password := r.URL.Query().Get("password")
		if password == "" {
			password = r.Header.Get("X-Share-Password")
		}
		if !h.manager.VerifyPassword(link, password) {
			http.Error(w, "Invalid password", http.StatusUnauthorized)
			return
		}
	}

	// 检查查看权限
	if !h.manager.HasPermission(link, PermissionView) {
		http.Error(w, "View permission denied", http.StatusForbidden)
		return
	}

	// 记录访问
	h.manager.RecordAccess(link.Token, r.RemoteAddr, r.UserAgent(), "view")

	// 返回文件信息
	subPath := r.URL.Query().Get("path")
	fullPath := filepath.Join(link.Path, subPath)

	// 验证路径安全
	if !strings.HasPrefix(fullPath, link.Path) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "Path not found", http.StatusNotFound)
		return
	}

	if info.IsDir() {
		// 列出目录
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		items := make([]map[string]interface{}, 0)
		for _, entry := range entries {
			entryInfo, err := entry.Info()
			if err != nil {
				continue
			}

			item := map[string]interface{}{
				"name":    entry.Name(),
				"path":    filepath.Join(subPath, entry.Name()),
				"isDir":   entry.IsDir(),
				"size":    entryInfo.Size(),
				"modTime": entryInfo.ModTime(),
			}
			items = append(items, item)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"type":    "directory",
			"path":    subPath,
			"items":   items,
			"link":    link,
		})
	} else {
		// 文件预览
		preview, err := h.manager.GetPreviewInfo(fullPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"type":    "file",
			"preview": preview,
			"link":    link,
		})
	}
}

// handleDownload 处理下载请求
func (h *Handler) handleDownload(w http.ResponseWriter, r *http.Request, tokenOrSlug string) {
	link, err := h.manager.GetShareLink(tokenOrSlug)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// 验证密码
	if link.Password != "" {
		password := r.URL.Query().Get("password")
		if password == "" {
			password = r.Header.Get("X-Share-Password")
		}
		if !h.manager.VerifyPassword(link, password) {
			http.Error(w, "Invalid password", http.StatusUnauthorized)
			return
		}
	}

	// 检查下载权限
	if !h.manager.HasPermission(link, PermissionDownload) {
		http.Error(w, "Download permission denied", http.StatusForbidden)
		return
	}

	// 获取文件路径
	subPath := r.URL.Query().Get("path")
	fullPath := filepath.Join(link.Path, subPath)

	// 验证路径安全
	if !strings.HasPrefix(fullPath, link.Path) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// 记录访问
	h.manager.RecordAccess(link.Token, r.RemoteAddr, r.UserAgent(), "download")

	if info.IsDir() {
		// 打包目录下载
		format := r.URL.Query().Get("format")
		if format == "" {
			format = "zip"
		}

		fileName := filepath.Base(fullPath)
		if subPath != "" {
			fileName = filepath.Base(subPath)
		}

		switch format {
		case "zip":
			w.Header().Set("Content-Type", "application/zip")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, fileName))
			if err := h.manager.CreateZipArchive([]string{fullPath}, w); err != nil {
				log.Printf("ZIP 打包失败: %v", err)
			}
		case "tar":
			w.Header().Set("Content-Type", "application/x-tar")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.tar"`, fileName))
			if err := h.manager.CreateTarArchive([]string{fullPath}, w, false); err != nil {
				log.Printf("TAR 打包失败: %v", err)
			}
		case "tar.gz":
			w.Header().Set("Content-Type", "application/gzip")
			w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.tar.gz"`, fileName))
			if err := h.manager.CreateTarArchive([]string{fullPath}, w, true); err != nil {
				log.Printf("TAR.GZ 打包失败: %v", err)
			}
		default:
			http.Error(w, "Unsupported format", http.StatusBadRequest)
		}
	} else {
		// 单文件下载
		w.Header().Set("Content-Type", getMimeType(fullPath))
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filepath.Base(fullPath)))
		w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))

		file, err := os.Open(fullPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer file.Close()

		io.Copy(w, file)
	}
}

// handleUpload 处理上传请求
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request, tokenOrSlug string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	link, err := h.manager.GetShareLink(tokenOrSlug)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// 验证密码
	if link.Password != "" {
		password := r.URL.Query().Get("password")
		if password == "" {
			password = r.Header.Get("X-Share-Password")
		}
		if !h.manager.VerifyPassword(link, password) {
			http.Error(w, "Invalid password", http.StatusUnauthorized)
			return
		}
	}

	// 检查上传权限
	if !link.AllowUpload || !h.manager.HasPermission(link, PermissionUpload) {
		http.Error(w, "Upload not allowed", http.StatusForbidden)
		return
	}

	// 解析上传文件
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "File is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 检查文件大小
	if header.Size > h.manager.maxFileSize {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	// 确定保存路径
	subPath := r.FormValue("path")
	uploadDir := link.UploadDir
	if uploadDir == "" {
		uploadDir = filepath.Join(link.Path, "uploads")
		os.MkdirAll(uploadDir, 0755)
	}

	savePath := filepath.Join(uploadDir, header.Filename)
	if subPath != "" {
		savePath = filepath.Join(uploadDir, subPath, header.Filename)
	}

	// 验证路径安全
	if !strings.HasPrefix(savePath, uploadDir) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// 创建目录
	if err := os.MkdirAll(filepath.Dir(savePath), 0755); err != nil {
		http.Error(w, "Failed to create directory", http.StatusInternalServerError)
		return
	}

	// 保存文件
	dst, err := os.Create(savePath)
	if err != nil {
		http.Error(w, "Failed to create file", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}

	// 记录上传
	h.manager.IncrementUploadCount(link.Token)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":  "File uploaded successfully",
		"fileName": header.Filename,
		"size":     header.Size,
		"path":     savePath,
	})
}

// generateQRCode 生成二维码
func (h *Handler) generateQRCode(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	link, err := h.manager.GetShareLinkByID(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	size := 256
	if s := r.URL.Query().Get("size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			size = v
		}
	}

	qrData, err := h.manager.GenerateQRCode(link.Token, size)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"qrCode":  qrData,
		"token":   link.Token,
		"size":    size,
		"url":     qrData,
	})
}

// ================== 工具函数 ==================

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
