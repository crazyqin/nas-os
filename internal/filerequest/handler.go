// Package filerequest 提供 REST API 处理器
package filerequest

import (
	"strconv"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器.
type Handler struct {
	svc *Service
}

// NewHandler 创建处理器.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes 注册路由到 gin 路由组
// 路由前缀: /api/v1/filerequest.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/filerequest")
	{
		g.POST("/create", h.createRequest)     // 创建文件请求
		g.GET("/list", h.listRequests)         // 列出文件请求
		g.POST("/upload/:token", h.uploadFile) // 通过令牌上传文件
		g.DELETE("/:id", h.deleteRequest)      // 删除文件请求
		g.GET("/:id", h.getRequest)            // 获取请求详情
		g.GET("/:id/uploads", h.listUploads)   // 获取上传列表
		g.POST("/:id/close", h.closeRequest)   // 关闭请求
		g.GET("/stats", h.getStats)            // 统计信息
	}
}

// createRequest 创建文件请求
// POST /api/v1/filerequest/create.
func (h *Handler) createRequest(c *gin.Context) {
	var req CreateRequestRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.svc.CreateRequest(c.Request.Context(), req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, result)
}

// listRequests 列出文件请求
// GET /api/v1/filerequest/list?creator_id=xxx&status=active&page=1&page_size=20.
func (h *Handler) listRequests(c *gin.Context) {
	var query ListRequestsQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		api.BadRequest(c, "查询参数错误: "+err.Error())
		return
	}

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 20
	}

	requests, total, err := h.svc.ListRequests(c.Request.Context(), query.CreatorID, query.Status, query.Page, query.PageSize)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.Page(c, requests, int64(total), query.Page, query.PageSize)
}

// uploadFile 通过令牌上传文件
// POST /api/v1/filerequest/upload/:token.
func (h *Handler) uploadFile(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		api.BadRequest(c, "缺少 token 参数")
		return
	}

	// 检查令牌有效性
	link, err := h.svc.GetLinkByToken(c.Request.Context(), token)
	if err != nil {
		api.NotFound(c, "无效的访问令牌")
		return
	}

	if !link.IsActive {
		api.BadRequest(c, "链接已被禁用")
		return
	}

	// 验证请求状态
	req, err := h.svc.GetRequestByToken(c.Request.Context(), token)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	if req.Status != RequestStatusActive {
		api.BadRequest(c, "文件请求已关闭或过期")
		return
	}

	// 如果有密码保护，验证密码
	if req.HasPassword {
		password := c.Query("password")
		if err := h.svc.VerifyPassword(c.Request.Context(), token, password); err != nil {
			api.Forbidden(c, err.Error())
			return
		}
	}

	// 解析上传信息（支持 multipart 和 JSON 两种方式）
	var uploadReq UploadFileRequest
	if err := c.ShouldBindJSON(&uploadReq); err != nil {
		// 尝试 multipart form
		file, header, ferr := c.Request.FormFile("file")
		if ferr != nil {
			api.BadRequest(c, "请提供文件上传信息或 multipart form")
			return
		}
		defer file.Close()
		uploadReq.OriginalName = header.Filename
		uploadReq.FileSize = header.Size
		uploadReq.MimeType = header.Header.Get("Content-Type")
		uploadReq.UploaderName = c.PostForm("uploader_name")
	}

	uploaderIP := c.ClientIP()
	result, err := h.svc.RecordUpload(c.Request.Context(), token, &uploadReq, uploaderIP)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, result)
}

// deleteRequest 删除文件请求
// DELETE /api/v1/filerequest/:id.
func (h *Handler) deleteRequest(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "缺少 id 参数")
		return
	}

	if err := h.svc.DeleteRequest(c.Request.Context(), id); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "文件请求已删除", nil)
}

// getRequest 获取请求详情
// GET /api/v1/filerequest/:id.
func (h *Handler) getRequest(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "缺少 id 参数")
		return
	}

	req, err := h.svc.GetRequest(c.Request.Context(), id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, req)
}

// listUploads 获取请求的上传文件列表
// GET /api/v1/filerequest/:id/uploads.
func (h *Handler) listUploads(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "缺少 id 参数")
		return
	}

	uploads, err := h.svc.GetUploads(c.Request.Context(), id)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, gin.H{"uploads": uploads, "count": len(uploads)})
}

// closeRequest 关闭文件请求
// POST /api/v1/filerequest/:id/close.
func (h *Handler) closeRequest(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		api.BadRequest(c, "缺少 id 参数")
		return
	}

	if err := h.svc.CloseRequest(c.Request.Context(), id); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "文件请求已关闭", nil)
}

// getStats 获取统计信息
// GET /api/v1/filerequest/stats.
func (h *Handler) getStats(c *gin.Context) {
	stats, err := h.svc.GetStats(c.Request.Context())
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, stats)
}

// formatFileSize 格式化文件大小（辅助函数）.
func formatFileSize(size int64) string {
	if size < 1024 {
		return strconv.FormatInt(size, 10) + " B"
	}
	if size < 1024*1024 {
		return strconv.FormatFloat(float64(size)/1024, 'f', 1, 64) + " KB"
	}
	if size < 1024*1024*1024 {
		return strconv.FormatFloat(float64(size)/(1024*1024), 'f', 1, 64) + " MB"
	}
	return strconv.FormatFloat(float64(size)/(1024*1024*1024), 'f', 1, 64) + " GB"
}
