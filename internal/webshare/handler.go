// Package webshare HTTP 处理器，提供 REST API 端点。
// 包含文件浏览、上传、下载、管理、分享链接等 API。
package webshare

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handler WebShare HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建 HTTP 处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册 API 路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	// WebShare API 路由组
	ws := rg.Group("/webshare")
	{
		// 目录浏览
		ws.GET("/browse", h.Browse)
		ws.GET("/browse/*path", h.BrowsePath)

		// 文件操作
		ws.POST("/folder", h.CreateFolder)
		ws.DELETE("/entries", h.DeleteEntries)
		ws.POST("/move", h.MoveEntries)
		ws.POST("/copy", h.CopyEntries)
		ws.PUT("/rename", h.RenameEntry)

		// 文件上传下载
		ws.POST("/upload/*path", h.Upload)
		ws.GET("/download/*path", h.Download)

		// 分享链接
		ws.POST("/share", h.CreateShareLink)
		ws.GET("/share", h.ListShareLinks)
		ws.GET("/share/:id", h.GetShareLink)
		ws.PUT("/share/:id", h.UpdateShareLink)
		ws.DELETE("/share/:id", h.DeleteShareLink)

		// 快照
		ws.GET("/snapshots/*path", h.ListSnapshots)

		// 搜索（集成 TrueSearch）
		ws.GET("/search", h.Search)

		// 统计
		ws.GET("/stats", h.GetStats)
		ws.GET("/share-stats", h.GetShareStats)
	}

	// 公开分享链接访问（无需认证）
	public := rg.Group("/share")
	{
		public.GET("/:token", h.AccessShareLink)
		public.GET("/:token/download", h.DownloadShared)
		public.POST("/:token/verify", h.VerifySharePassword)
	}
}

// ==================== 目录浏览 API ====================

// Browse 列出根目录内容
func (h *Handler) Browse(c *gin.Context) {
	showHidden, _ := strconv.ParseBool(c.DefaultQuery("hidden", "false"))
	filter := c.DefaultQuery("filter", "")
	sortBy := SortField(c.DefaultQuery("sort", "name"))
	sortDir := SortDirection(c.DefaultQuery("dir", "asc"))

	listing, err := h.manager.ListDirectory("", showHidden, filter, sortBy, sortDir)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    listing,
	})
}

// BrowsePath 列出指定路径的目录内容
func (h *Handler) BrowsePath(c *gin.Context) {
	path := c.Param("path")
	showHidden, _ := strconv.ParseBool(c.DefaultQuery("hidden", "false"))
	filter := c.DefaultQuery("filter", "")
	sortBy := SortField(c.DefaultQuery("sort", "name"))
	sortDir := SortDirection(c.DefaultQuery("dir", "asc"))

	listing, err := h.manager.ListDirectory(path, showHidden, filter, sortBy, sortDir)
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    listing,
	})
}

// ==================== 文件操作 API ====================

// CreateFolder 创建文件夹
func (h *Handler) CreateFolder(c *gin.Context) {
	var req CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求: " + err.Error(),
		})
		return
	}

	if err := h.manager.CreateFolder(req.Path, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: fmt.Sprintf("文件夹 '%s' 创建成功", req.Name),
	})
}

// DeleteEntries 删除文件/目录
func (h *Handler) DeleteEntries(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求: " + err.Error(),
		})
		return
	}

	if err := h.manager.DeleteEntries(req.Paths); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: fmt.Sprintf("已删除 %d 个项目", len(req.Paths)),
	})
}

// MoveEntries 移动/重命名文件/目录
func (h *Handler) MoveEntries(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths" binding:"required"`
		Dest  string   `json:"dest" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求: " + err.Error(),
		})
		return
	}

	if err := h.manager.MoveEntries(req.Paths, req.Dest); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "移动成功",
	})
}

// CopyEntries 复制文件/目录
func (h *Handler) CopyEntries(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths" binding:"required"`
		Dest  string   `json:"dest" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求: " + err.Error(),
		})
		return
	}

	if err := h.manager.CopyEntries(req.Paths, req.Dest); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "复制成功",
	})
}

// RenameEntry 重命名文件/目录
func (h *Handler) RenameEntry(c *gin.Context) {
	var req struct {
		Path    string `json:"path" binding:"required"`
		NewName string `json:"new_name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求: " + err.Error(),
		})
		return
	}

	if err := h.manager.RenameEntry(req.Path, req.NewName); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: fmt.Sprintf("重命名为 '%s' 成功", req.NewName),
	})
}

// ==================== 文件上传下载 API ====================

// Upload 文件上传
func (h *Handler) Upload(c *gin.Context) {
	path := c.Param("path")

	// 获取上传的文件
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "获取上传文件失败: " + err.Error(),
		})
		return
	}
	defer file.Close()

	// 检查文件大小
	if header.Size > h.manager.config.MaxFileSize {
		c.JSON(http.StatusRequestEntityTooLarge, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("文件大小超过限制 (%d bytes)", h.manager.config.MaxFileSize),
		})
		return
	}

	// 构建保存路径
	dirPath := h.manager.sanitizePath(path)
	absDirPath := filepath.Join(h.manager.config.RootPath, dirPath)

	if !h.manager.isPathSafe(absDirPath) {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的上传路径",
		})
		return
	}

	// 确保目录存在
	if err := os.MkdirAll(absDirPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "创建目录失败: " + err.Error(),
		})
		return
	}

	// 保存文件
	absFilePath := filepath.Join(absDirPath, header.Filename)
	dst, err := os.Create(absFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "创建文件失败: " + err.Error(),
		})
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   "保存文件失败: " + err.Error(),
		})
		return
	}

	// 失效缓存
	h.manager.filterCache.invalidate(dirPath)

	relPath := filepath.Join(dirPath, header.Filename)
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "文件上传成功",
		Data: UploadResponse{
			FileName: header.Filename,
			Size:     written,
			Path:     relPath,
		},
	})
}

// Download 文件下载
func (h *Handler) Download(c *gin.Context) {
	path := c.Param("path")
	absPath := filepath.Join(h.manager.config.RootPath, h.manager.sanitizePath(path))

	if !h.manager.isPathSafe(absPath) {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的路径",
		})
		return
	}

	// 检查文件是否存在
	info, err := os.Stat(absPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "文件不存在",
		})
		return
	}

	if info.IsDir() {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "不能下载目录",
		})
		return
	}

	// 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filepath.Base(absPath)))
	c.Header("Content-Type", getMimeType(filepath.Base(absPath)))
	c.Header("Content-Length", strconv.FormatInt(info.Size(), 10))

	c.File(absPath)
}

// ==================== 分享链接 API ====================

// CreateShareLink 创建分享链接
func (h *Handler) CreateShareLink(c *gin.Context) {
	var req CreateShareLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求: " + err.Error(),
		})
		return
	}

	link, err := h.manager.CreateShareLink(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "分享链接创建成功",
		Data:    link,
	})
}

// ListShareLinks 列出分享链接
func (h *Handler) ListShareLinks(c *gin.Context) {
	createdBy := c.Query("created_by")
	includeInactive, _ := strconv.ParseBool(c.DefaultQuery("all", "false"))

	links := h.manager.ListShareLinks(createdBy, includeInactive)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    links,
	})
}

// GetShareLink 获取分享链接详情
func (h *Handler) GetShareLink(c *gin.Context) {
	id := c.Param("id")

	link, err := h.manager.GetShareLinkByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    link,
	})
}

// UpdateShareLink 更新分享链接
func (h *Handler) UpdateShareLink(c *gin.Context) {
	id := c.Param("id")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求: " + err.Error(),
		})
		return
	}

	link, err := h.manager.UpdateShareLink(id, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "分享链接更新成功",
		Data:    link,
	})
}

// DeleteShareLink 删除分享链接
func (h *Handler) DeleteShareLink(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteShareLink(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "分享链接已删除",
	})
}

// AccessShareLink 通过令牌访问分享链接
func (h *Handler) AccessShareLink(c *gin.Context) {
	token := c.Param("token")

	link, err := h.manager.GetShareLink(token)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// 如果有密码保护，检查是否已验证
	if link.Password != "" {
		verified := c.Query("verified")
		if verified != "true" {
			c.JSON(http.StatusOK, APIResponse{
				Success: true,
				Message: "需要密码验证",
				Data: gin.H{
					"requires_password": true,
					"token":             token,
				},
			})
			return
		}
	}

	// 记录访问
	h.manager.RecordAccess(link.ID, "view", link.Path, c.ClientIP(), c.GetHeader("User-Agent"), "")

	// 如果是文件，返回文件信息
	absPath := filepath.Join(h.manager.config.RootPath, h.manager.sanitizePath(link.Path))
	info, err := os.Stat(absPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "文件不存在",
		})
		return
	}

	if info.IsDir() {
		// 返回目录列表
		listing, err := h.manager.ListDirectory(link.Path, false, "", SortByName, SortAsc)
		if err != nil {
			c.JSON(http.StatusInternalServerError, APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data: gin.H{
				"link":    link,
				"listing": listing,
			},
		})
	} else {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data: gin.H{
				"link": link,
				"file": Entry{
					Name:      info.Name(),
					Path:      link.Path,
					Size:      info.Size(),
					ModTime:   info.ModTime(),
					Extension: getFileExtension(info.Name()),
					MimeType:  getMimeType(info.Name()),
				},
			},
		})
	}
}

// DownloadShared 下载分享的文件
func (h *Handler) DownloadShared(c *gin.Context) {
	token := c.Param("token")

	link, err := h.manager.GetShareLink(token)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	// 检查下载权限
	if link.Permission == PermissionView {
		c.JSON(http.StatusForbidden, APIResponse{
			Success: false,
			Error:   "没有下载权限",
		})
		return
	}

	absPath := filepath.Join(h.manager.config.RootPath, h.manager.sanitizePath(link.Path))

	// 检查文件是否存在
	info, err := os.Stat(absPath)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "文件不存在",
		})
		return
	}

	if info.IsDir() {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "不能下载目录",
		})
		return
	}

	// 记录下载
	h.manager.RecordAccess(link.ID, "download", link.Path, c.ClientIP(), c.GetHeader("User-Agent"), "")

	// 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", info.Name()))
	c.Header("Content-Type", getMimeType(info.Name()))
	c.Header("Content-Length", strconv.FormatInt(info.Size(), 10))

	c.File(absPath)
}

// VerifySharePassword 验证分享链接密码
func (h *Handler) VerifySharePassword(c *gin.Context) {
	token := c.Param("token")

	var req struct {
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的请求: " + err.Error(),
		})
		return
	}

	if h.manager.VerifySharePassword(token, req.Password) {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Message: "密码验证成功",
			Data: gin.H{
				"verified": true,
				"token":    token,
			},
		})
	} else {
		c.JSON(http.StatusUnauthorized, APIResponse{
			Success: false,
			Error:   "密码错误",
		})
	}
}

// ==================== 快照 API ====================

// ListSnapshots 列出快照
func (h *Handler) ListSnapshots(c *gin.Context) {
	path := c.Param("path")

	snapshots, err := h.manager.ListSnapshots(path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    snapshots,
	})
}

// ==================== 搜索 API ====================

// Search 搜索文件
func (h *Handler) Search(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "搜索关键词不能为空",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	path := c.Query("path")

	// 构建搜索请求
	req := SearchRequest{
		Query:  query,
		Path:   path,
		Limit:  limit,
		Offset: offset,
	}

	// TODO: 集成 TrueSearch 进行实际搜索
	// 目前返回基于文件名的简单匹配
	results, err := h.simpleSearch(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    results,
	})
}

// simpleSearch 简单文件名搜索（临时实现，待集成 TrueSearch）
func (h *Handler) simpleSearch(req SearchRequest) (map[string]interface{}, error) {
	if !h.manager.IsRunning() {
		return nil, fmt.Errorf("服务未运行")
	}

	results := make([]Entry, 0)
	query := strings.ToLower(req.Query)

	// 使用 filepath.Walk 搜索
	searchPath := h.manager.config.RootPath
	if req.Path != "" {
		searchPath = filepath.Join(h.manager.config.RootPath, h.manager.sanitizePath(req.Path))
	}

	if !h.manager.isPathSafe(searchPath) {
		return nil, fmt.Errorf("无效的搜索路径")
	}

	count := 0
	filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
		}

		// 限制结果数量
		if req.Limit > 0 && count >= req.Limit+req.Offset {
			return filepath.SkipAll
		}

		name := strings.ToLower(info.Name())
		if strings.Contains(name, query) {
			if count >= req.Offset {
				relPath, _ := filepath.Rel(h.manager.config.RootPath, path)
				results = append(results, Entry{
					Name:         info.Name(),
					Path:         relPath,
					AbsolutePath: path,
					Size:         info.Size(),
					ModTime:      info.ModTime(),
					IsHidden:     strings.HasPrefix(info.Name(), "."),
					Extension:    getFileExtension(info.Name()),
					MimeType:     getMimeType(info.Name()),
					Permission:   info.Mode().String(),
				})
			}
			count++
		}

		return nil
	})

	return map[string]interface{}{
		"query":       req.Query,
		"total_hits":  len(results),
		"results":     results,
		"took_ms":     0,
	}, nil
}

// ==================== 统计 API ====================

// GetStats 获取服务统计
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}

// GetShareStats 获取分享统计
func (h *Handler) GetShareStats(c *gin.Context) {
	stats := h.manager.GetShareStats()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}
