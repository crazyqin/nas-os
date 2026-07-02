package filemanager

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 文件管理器HTTP处理器.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建文件管理器处理器.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	fm := rg.Group("/filemanager")
	{
		// 文件浏览
		fm.GET("/browse", h.Browse)
		fm.GET("/browse/*path", h.BrowsePath)
		fm.GET("/tree", h.GetTree)
		fm.GET("/info/*path", h.GetFileInfo)
		fm.GET("/attributes/*path", h.GetFileAttributes)
		fm.GET("/disk-usage", h.GetDiskUsage)

		// 文件操作
		fm.POST("/copy", h.Copy)
		fm.POST("/move", h.Move)
		fm.POST("/delete", h.Delete)
		fm.POST("/rename", h.Rename)
		fm.POST("/batch", h.BatchOperation)
		fm.POST("/drag-drop", h.DragDrop)

		// 压缩/解压
		fm.POST("/compress", h.Compress)
		fm.POST("/extract", h.Extract)

		// 文件预览
		fm.GET("/preview/*path", h.GetPreview)
		fm.GET("/preview-content/*path", h.GetPreviewContent)
		fm.GET("/thumbnail/*path", h.GetThumbnail)

		// 文件搜索
		fm.POST("/search", h.Search)

		// 文件分享
		fm.POST("/share", h.CreateShare)
		fm.GET("/share/:id", h.GetShare)
		fm.PUT("/share/:id", h.UpdateShare)
		fm.DELETE("/share/:id", h.DeleteShare)
		fm.GET("/shares", h.ListShares)
		fm.GET("/share-stats", h.GetShareStats)

		// 公开分享访问（无需认证）
		fm.GET("/public/:token", h.PublicAccess)
		fm.POST("/public/:token/verify", h.PublicVerifyPassword)

		// 操作状态
		fm.GET("/operations", h.ListOperations)
		fm.GET("/operations/:id", h.GetOperation)
		fm.POST("/operations/:id/cancel", h.CancelOperation)

		// 收藏夹
		fm.GET("/favorites", h.ListFavorites)
		fm.POST("/favorites", h.AddFavorite)
		fm.DELETE("/favorites/:id", h.RemoveFavorite)

		// 版本管理
		fm.GET("/versions/*path", h.ListVersions)
		fm.POST("/versions/*path", h.CreateVersion)
		fm.POST("/versions/:id/restore", h.RestoreVersion)
	}
}

// Browse 浏览根目录.
func (h *Handler) Browse(c *gin.Context) {
	showHidden := c.Query("show_hidden") == "true"

	listing, err := h.manager.browser.ListDirectory("", showHidden)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, listing)
}

// BrowsePath 浏览指定路径.
func (h *Handler) BrowsePath(c *gin.Context) {
	path := c.Param("path")
	showHidden := c.Query("show_hidden") == "true"

	listing, err := h.manager.browser.ListDirectory(path, showHidden)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, listing)
}

// GetTreeRequest 获取树形目录请求.
type GetTreeRequest struct {
	Path       string `form:"path"`
	MaxDepth   int    `form:"max_depth"`
	ShowHidden bool   `form:"show_hidden"`
}

// GetTree 获取目录树.
func (h *Handler) GetTree(c *gin.Context) {
	var req GetTreeRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	opts := DefaultTreeOptions()
	if req.MaxDepth > 0 {
		opts.MaxDepth = req.MaxDepth
	}
	opts.ShowHidden = req.ShowHidden

	tree, err := h.manager.browser.GetTree(req.Path, opts)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, tree)
}

// GetFileInfo 获取文件信息.
func (h *Handler) GetFileInfo(c *gin.Context) {
	path := c.Param("path")

	node, err := h.manager.browser.GetFileNode(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, node)
}

// GetFileAttributes 获取文件属性.
func (h *Handler) GetFileAttributes(c *gin.Context) {
	path := c.Param("path")

	attrs, err := h.manager.browser.GetFileAttributes(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, attrs)
}

// GetDiskUsage 获取磁盘使用情况.
func (h *Handler) GetDiskUsage(c *gin.Context) {
	path := c.DefaultQuery("path", "/")

	// 使用browser的内部方法
	node, err := h.manager.browser.GetFileNode(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 返回基本磁盘信息
	c.JSON(http.StatusOK, gin.H{
		"path": path,
		"name": node.Name,
		"type": node.Type,
		"size": node.Size,
	})
}

// Copy 复制文件.
func (h *Handler) Copy(c *gin.Context) {
	var req BatchOperation
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := h.getUserID(c)
	op, err := h.manager.operations.Copy(req.Sources, req.Destination, req.Overwrite, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, op)
}

// Move 移动文件.
func (h *Handler) Move(c *gin.Context) {
	var req BatchOperation
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := h.getUserID(c)
	op, err := h.manager.operations.Move(req.Sources, req.Destination, req.Overwrite, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, op)
}

// DeleteRequest 删除请求.
type DeleteRequest struct {
	Sources []string `json:"sources" binding:"required,min=1"`
}

// Delete 删除文件.
func (h *Handler) Delete(c *gin.Context) {
	var req DeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := h.getUserID(c)
	op, err := h.manager.operations.Delete(req.Sources, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, op)
}

// RenameRequest 重命名请求.
type RenameRequest struct {
	Path    string `json:"path" binding:"required"`
	NewName string `json:"new_name" binding:"required"`
}

// Rename 重命名.
func (h *Handler) Rename(c *gin.Context) {
	var req RenameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := h.getUserID(c)
	op, err := h.manager.operations.Rename(req.Path, req.NewName, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, op)
}

// BatchOperation 批量操作.
func (h *Handler) BatchOperation(c *gin.Context) {
	var req BatchOperation
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := h.getUserID(c)
	op, err := h.manager.operations.BatchOperation(req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, op)
}

// DragDrop 拖拽操作.
func (h *Handler) DragDrop(c *gin.Context) {
	var req DragDropRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := h.getUserID(c)
	op, err := h.manager.operations.DragDrop(req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, op)
}

// Compress 压缩文件.
func (h *Handler) Compress(c *gin.Context) {
	var opts CompressOptions
	if err := c.ShouldBindJSON(&opts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := h.getUserID(c)
	op, err := h.manager.operations.Compress(opts, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, op)
}

// Extract 解压文件.
func (h *Handler) Extract(c *gin.Context) {
	var opts ExtractOptions
	if err := c.ShouldBindJSON(&opts); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := h.getUserID(c)
	op, err := h.manager.operations.Extract(opts, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, op)
}

// GetPreview 获取文件预览.
func (h *Handler) GetPreview(c *gin.Context) {
	path := c.Param("path")

	info, err := h.manager.preview.GetPreviewInfo(path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// GetPreviewContent 获取预览内容.
func (h *Handler) GetPreviewContent(c *gin.Context) {
	path := c.Param("path")
	maxLines, _ := strconv.Atoi(c.DefaultQuery("max_lines", "100"))

	content, err := h.manager.preview.GetPreviewContent(path, maxLines)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"path":    path,
		"content": content,
	})
}

// GetThumbnail 获取缩略图.
func (h *Handler) GetThumbnail(c *gin.Context) {
	path := c.Param("path")

	thumbnailPath, err := h.manager.preview.GenerateThumbnail(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"path":      path,
		"thumbnail": thumbnailPath,
	})
}

// SearchRequest 搜索请求.
type SearchRequest struct {
	Keyword       string   `json:"keyword" binding:"required"`
	Path          string   `json:"path"`
	Extensions    []string `json:"extensions"`
	FileType      string   `json:"file_type"`
	ContentSearch bool     `json:"content_search"`
	MaxResults    int      `json:"max_results"`
}

// Search 搜索文件.
func (h *Handler) Search(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	query := SearchQuery{
		Keyword:       req.Keyword,
		Path:          req.Path,
		Extensions:    req.Extensions,
		ContentSearch: req.ContentSearch,
		MaxResults:    req.MaxResults,
	}

	if req.FileType != "" {
		query.FileType = FileType(req.FileType)
	}

	result, err := h.manager.Search(query)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// CreateShare 创建分享.
func (h *Handler) CreateShare(c *gin.Context) {
	var req CreateShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := h.getUserID(c)
	link, err := h.manager.share.CreateLink(req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, link)
}

// GetShare 获取分享.
func (h *Handler) GetShare(c *gin.Context) {
	id := c.Param("id")

	link, err := h.manager.share.GetLink(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, link)
}

// UpdateShare 更新分享.
func (h *Handler) UpdateShare(c *gin.Context) {
	id := c.Param("id")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := h.getUserID(c)
	link, err := h.manager.share.UpdateLink(id, updates, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, link)
}

// DeleteShare 删除分享.
func (h *Handler) DeleteShare(c *gin.Context) {
	id := c.Param("id")

	userID := h.getUserID(c)
	if err := h.manager.share.DeleteLink(id, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "分享链接已删除"})
}

// ListShares 列出分享.
func (h *Handler) ListShares(c *gin.Context) {
	userID := h.getUserID(c)
	links := h.manager.share.ListLinks(userID)

	c.JSON(http.StatusOK, gin.H{
		"links": links,
		"total": len(links),
	})
}

// GetShareStats 获取分享统计.
func (h *Handler) GetShareStats(c *gin.Context) {
	stats := h.manager.share.GetStats()
	c.JSON(http.StatusOK, stats)
}

// PublicAccess 公开访问分享.
func (h *Handler) PublicAccess(c *gin.Context) {
	token := c.Param("token")
	password := c.Query("password")

	link, err := h.manager.share.GetLinkByToken(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// 验证密码
	if link.HasPassword {
		if password == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":        "需要密码",
				"require_pass": true,
			})
			return
		}

		ok, err := h.manager.share.VerifyPassword(token, password)
		if err != nil || !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
			return
		}
	}

	// 记录下载
	h.manager.share.RecordDownload(token)

	c.JSON(http.StatusOK, gin.H{
		"path":       link.Path,
		"name":       link.Name,
		"permission": link.Permission,
	})
}

// PublicVerifyPassword 验证公开分享密码.
func (h *Handler) PublicVerifyPassword(c *gin.Context) {
	token := c.Param("token")

	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ok, err := h.manager.share.VerifyPassword(token, req.Password)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"verified": true})
}

// ListOperations 列出操作.
func (h *Handler) ListOperations(c *gin.Context) {
	ops := h.manager.operations.ListOperations()
	c.JSON(http.StatusOK, gin.H{
		"operations": ops,
		"total":      len(ops),
	})
}

// GetOperation 获取操作状态.
func (h *Handler) GetOperation(c *gin.Context) {
	id := c.Param("id")

	op, err := h.manager.operations.GetOperation(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, op)
}

// CancelOperation 取消操作.
func (h *Handler) CancelOperation(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.operations.CancelOperation(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "操作已取消"})
}

// ListFavorites 列出收藏.
func (h *Handler) ListFavorites(c *gin.Context) {
	userID := h.getUserID(c)
	favorites := h.manager.ListFavorites(userID)

	c.JSON(http.StatusOK, gin.H{
		"favorites": favorites,
		"total":     len(favorites),
	})
}

// AddFavoriteRequest 添加收藏请求.
type AddFavoriteRequest struct {
	Path string `json:"path" binding:"required"`
}

// AddFavorite 添加收藏.
func (h *Handler) AddFavorite(c *gin.Context) {
	var req AddFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := h.getUserID(c)
	fav, err := h.manager.AddFavorite(req.Path, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, fav)
}

// RemoveFavorite 删除收藏.
func (h *Handler) RemoveFavorite(c *gin.Context) {
	id := c.Param("id")

	userID := h.getUserID(c)
	if err := h.manager.RemoveFavorite(id, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "收藏已删除"})
}

// ListVersions 列出版本.
func (h *Handler) ListVersions(c *gin.Context) {
	path := c.Param("path")

	versions, err := h.manager.ListVersions(path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"versions": versions,
		"total":    len(versions),
	})
}

// CreateVersion 创建版本.
func (h *Handler) CreateVersion(c *gin.Context) {
	path := c.Param("path")

	var req struct {
		Comment string `json:"comment"`
	}
	c.ShouldBindJSON(&req)

	userID := h.getUserID(c)
	version, err := h.manager.CreateVersion(path, req.Comment, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, version)
}

// RestoreVersionRequest 恢复版本请求.
type RestoreVersionRequest struct {
	VersionID string `json:"version_id" binding:"required"`
}

// RestoreVersion 恢复版本.
func (h *Handler) RestoreVersion(c *gin.Context) {
	var req RestoreVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.RestoreVersion(req.VersionID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "版本已恢复"})
}

// getUserID 获取用户ID.
func (h *Handler) getUserID(c *gin.Context) string {
	// 从JWT或session获取用户ID
	if userID, exists := c.Get("user_id"); exists {
		if uid, ok := userID.(string); ok {
			return uid
		}
	}
	return "anonymous"
}

// ErrorResponse 错误响应.
func ErrorResponse(code int, message string) gin.H {
	return gin.H{
		"error":     message,
		"code":      code,
		"timestamp": time.Now().Unix(),
	}
}
