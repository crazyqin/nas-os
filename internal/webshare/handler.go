// Package webshare 提供 REST API 处理器
package webshare

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes 注册路由到 gin 路由组
// 路由前缀: /api/v1/webshare
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/webshare")
	{
		// 分享管理
		g.POST("/shares", h.createShare)               // 创建分享
		g.GET("/shares", h.listShares)                  // 列出分享
		g.GET("/shares/:id", h.getShare)               // 获取分享详情
		g.DELETE("/shares/:id", h.deleteShare)          // 删除分享
		g.POST("/shares/:id/revoke", h.revokeShare)     // 撤销分享
		g.PUT("/shares/:id/permission", h.updatePerm)  // 更新权限
		g.PUT("/shares/:id/password", h.setPassword)   // 设置密码
		g.PUT("/shares/:id/fips", h.toggleFIPS)        // 切换 FIPS

		// 分享链接
		g.GET("/shares/:id/link", h.generateLink)      // 生成分享链接

		// 会话管理
		g.POST("/sessions", h.createSession)           // 创建会话（需要令牌+密码）
		g.DELETE("/sessions/:id", h.destroySession)    // 销毁会话
		g.GET("/sessions/:id", h.getSession)           // 获取会话

		// 文件操作（通过会话）
		g.GET("/sessions/:id/files", h.listFiles)      // 浏览文件
		g.POST("/sessions/:id/folder", h.createFolder) // 创建文件夹
		g.POST("/sessions/:id/upload", h.uploadFile)   // 上传文件
		g.GET("/sessions/:id/download", h.downloadFile) // 下载文件
		g.DELETE("/sessions/:id/files", h.deleteFile)   // 删除文件
		g.PUT("/sessions/:id/rename", h.renameFile)    // 重命名文件

		// 统计
		g.GET("/stats", h.getStats)                    // 分享统计

		// 配置
		g.GET("/config", h.getConfig)
		g.PUT("/config", h.updateConfig)
	}
}

// createShare 创建 Web 分享
// POST /api/v1/webshare/shares
func (h *Handler) createShare(c *gin.Context) {
	var req CreateShareRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	share, err := h.svc.CreateShare(c.Request.Context(), &req)
	if err != nil {
		api.Conflict(c, err.Error())
		return
	}

	api.Created(c, share)
}

// listShares 列出分享
// GET /api/v1/webshare/shares?status=active
func (h *Handler) listShares(c *gin.Context) {
	status := ShareStatus(c.Query("status"))

	shares, err := h.svc.ListShares(c.Request.Context(), status)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, gin.H{"shares": shares, "total": len(shares)})
}

// getShare 获取分享详情
// GET /api/v1/webshare/shares/:id
func (h *Handler) getShare(c *gin.Context) {
	shareID := c.Param("id")

	share, err := h.svc.GetShare(c.Request.Context(), shareID)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, share)
}

// deleteShare 删除分享
// DELETE /api/v1/webshare/shares/:id
func (h *Handler) deleteShare(c *gin.Context) {
	shareID := c.Param("id")

	if err := h.svc.DeleteShare(c.Request.Context(), shareID); err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OKWithMessage(c, "分享已删除", nil)
}

// revokeShare 撤销分享
// POST /api/v1/webshare/shares/:id/revoke
func (h *Handler) revokeShare(c *gin.Context) {
	shareID := c.Param("id")

	if err := h.svc.RevokeShare(c.Request.Context(), shareID); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "分享已撤销", nil)
}

// updatePerm 更新分享权限
// PUT /api/v1/webshare/shares/:id/permission
func (h *Handler) updatePerm(c *gin.Context) {
	shareID := c.Param("id")

	var perm SharePermission
	if err := c.ShouldBindJSON(&perm); err != nil {
		api.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	share, err := h.svc.UpdateSharePermission(c.Request.Context(), shareID, &perm)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, share)
}

// setPassword 设置分享密码
// PUT /api/v1/webshare/shares/:id/password
func (h *Handler) setPassword(c *gin.Context) {
	shareID := c.Param("id")

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.svc.SetPassword(c.Request.Context(), shareID, req.Password); err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OKWithMessage(c, "密码已更新", nil)
}

// toggleFIPS 切换 FIPS 加密
// PUT /api/v1/webshare/shares/:id/fips
func (h *Handler) toggleFIPS(c *gin.Context) {
	shareID := c.Param("id")

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	var (
		share *WebShare
		err   error
	)
	if req.Enabled {
		share, err = h.svc.EnableFIPS(c.Request.Context(), shareID)
	} else {
		share, err = h.svc.DisableFIPS(c.Request.Context(), shareID)
	}
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, share)
}

// generateLink 生成分享链接
// GET /api/v1/webshare/shares/:id/link?base_url=https://example.com
func (h *Handler) generateLink(c *gin.Context) {
	shareID := c.Param("id")
	baseURL := c.Query("base_url")
	if baseURL == "" {
		baseURL = "https://nas.local"
	}

	link, err := h.svc.GenerateShareLink(c.Request.Context(), shareID, baseURL)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, link)
}

// createSession 创建会话
// POST /api/v1/webshare/sessions
func (h *Handler) createSession(c *gin.Context) {
	var req struct {
		Token     string `json:"token" binding:"required"`
		Password  string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	clientIP := c.ClientIP()
	userAgent := c.Request.UserAgent()

	sess, err := h.svc.CreateSession(c.Request.Context(), req.Token, clientIP, userAgent, req.Password)
	if err != nil {
		api.Unauthorized(c, err.Error())
		return
	}

	api.Created(c, SessionResponse{
		SessionToken: sess.ID,
		ExpiresAt:    sess.ExpiresAt,
	})
}

// destroySession 销毁会话
// DELETE /api/v1/webshare/sessions/:id
func (h *Handler) destroySession(c *gin.Context) {
	sessionID := c.Param("id")

	if err := h.svc.DestroySession(c.Request.Context(), sessionID); err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OKWithMessage(c, "会话已销毁", nil)
}

// getSession 获取会话
// GET /api/v1/webshare/sessions/:id
func (h *Handler) getSession(c *gin.Context) {
	sessionID := c.Param("id")

	sess, err := h.svc.GetSession(c.Request.Context(), sessionID)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, sess)
}

// listFiles 浏览文件
// GET /api/v1/webshare/sessions/:id/files?path=/
func (h *Handler) listFiles(c *gin.Context) {
	sessionID := c.Param("id")
	path := c.Query("path")

	entries, err := h.svc.ListFiles(c.Request.Context(), sessionID, path)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, FileListResponse{
		Path:    path,
		Entries: entries,
		Total:   len(entries),
	})
}

// createFolder 创建文件夹
// POST /api/v1/webshare/sessions/:id/folder
func (h *Handler) createFolder(c *gin.Context) {
	sessionID := c.Param("id")

	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.svc.CreateFolder(c.Request.Context(), sessionID, req.Path); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.CreatedWithMessage(c, "文件夹创建成功", nil)
}

// uploadFile 上传文件
// POST /api/v1/webshare/sessions/:id/upload
func (h *Handler) uploadFile(c *gin.Context) {
	sessionID := c.Param("id")
	path := c.PostForm("path")
	if path == "" {
		var req UploadFileRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			api.BadRequest(c, "请求参数错误: "+err.Error())
			return
		}
		path = req.Path
	}

	file, err := c.FormFile("file")
	if err != nil {
		api.BadRequest(c, "上传文件获取失败: "+err.Error())
		return
	}

	if err := h.svc.UploadFile(c.Request.Context(), sessionID, path, file.Size); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "文件上传成功", gin.H{
		"filename": file.Filename,
		"size":     file.Size,
	})
}

// downloadFile 下载文件
// GET /api/v1/webshare/sessions/:id/download?path=/file.txt
func (h *Handler) downloadFile(c *gin.Context) {
	sessionID := c.Param("id")
	path := c.Query("path")
	if path == "" {
		api.BadRequest(c, "path 参数不能为空")
		return
	}

	if err := h.svc.DownloadFile(c.Request.Context(), sessionID, path); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 实际应返回文件流
	api.OKWithMessage(c, "下载授权通过", nil)
}

// deleteFile 删除文件
// DELETE /api/v1/webshare/sessions/:id/files?path=/file.txt
func (h *Handler) deleteFile(c *gin.Context) {
	sessionID := c.Param("id")
	path := c.Query("path")
	if path == "" {
		api.BadRequest(c, "path 参数不能为空")
		return
	}

	if err := h.svc.DeleteFile(c.Request.Context(), sessionID, path); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "文件已删除", nil)
}

// renameFile 重命名文件
// PUT /api/v1/webshare/sessions/:id/rename
func (h *Handler) renameFile(c *gin.Context) {
	sessionID := c.Param("id")

	var req struct {
		OldPath string `json:"old_path" binding:"required"`
		NewPath string `json:"new_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.svc.RenameFile(c.Request.Context(), sessionID, req.OldPath, req.NewPath); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "重命名成功", nil)
}

// getStats 获取分享统计
// GET /api/v1/webshare/stats
func (h *Handler) getStats(c *gin.Context) {
	stats, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, stats)
}

// getConfig 获取配置
// GET /api/v1/webshare/config
func (h *Handler) getConfig(c *gin.Context) {
	cfg := h.svc.GetConfig()
	api.OK(c, cfg)
}

// updateConfig 更新配置
// PUT /api/v1/webshare/config
func (h *Handler) updateConfig(c *gin.Context) {
	var cfg WebShareConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		api.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	h.svc.UpdateConfig(&cfg)
	api.OKWithMessage(c, "配置已更新", nil)
}