// Package nfs 提供NFS共享服务管理功能
package nfs

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"nas-os/internal/logging"
)

// Handlers NFS API处理器
type Handlers struct {
	manager *Manager
	parser  *ConfigParser
	logger  *logging.Logger
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{
		manager: mgr,
		parser:  NewConfigParser(),
		logger:  logging.NewLogger(nil).WithSource("nfs-api"),
	}
}

// Response 通用响应
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse 错误响应
func ErrorResponse(code int, message string) Response {
	return Response{Code: code, Message: message}
}

// SuccessResponse 成功响应
func SuccessResponse(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	nfs := api.Group("/nfs")
	{
		// 导出管理
		nfs.GET("/exports", h.ListExports)
		nfs.POST("/exports", h.CreateExport)
		nfs.GET("/exports/:path", h.GetExport)
		nfs.PUT("/exports/:path", h.UpdateExport)
		nfs.DELETE("/exports/:path", h.DeleteExport)

		// 服务管理
		nfs.GET("/status", h.GetStatus)
		nfs.POST("/start", h.StartService)
		nfs.POST("/stop", h.StopService)
		nfs.POST("/restart", h.RestartService)
		nfs.POST("/reload", h.ReloadConfig)

		// 客户端信息
		nfs.GET("/clients", h.GetClients)

		// 配置文件
		nfs.GET("/config/exports", h.GetExportsFile)
	}
}

// ListExports 列出所有导出
// @Summary 列出所有NFS导出
// @Tags NFS
// @Produce json
// @Success 200 {object} Response
// @Router /nfs/exports [get]
func (h *Handlers) ListExports(c *gin.Context) {
	exports, err := h.manager.ListExports()
	if err != nil {
		h.logger.Errorf("列出导出失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, "获取导出列表失败"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(exports))
}

// CreateExport 创建导出
// @Summary 创建NFS导出
// @Tags NFS
// @Accept json
// @Produce json
// @Param export body Export true "导出配置"
// @Success 200 {object} Response
// @Router /nfs/exports [post]
func (h *Handlers) CreateExport(c *gin.Context) {
	var req Export
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("解析请求失败: %v", err)
		c.JSON(http.StatusBadRequest, ErrorResponse(400, "无效的请求参数: "+err.Error()))
		return
	}

	// 验证导出配置
	if err := h.manager.ValidateExport(&req); err != nil {
		h.logger.Warnf("验证导出配置失败: %v", err)
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	// 创建导出
	if err := h.manager.CreateExport(&req); err != nil {
		h.logger.Errorf("创建导出失败: %v", err)
		c.JSON(http.StatusConflict, ErrorResponse(409, err.Error()))
		return
	}

	// 应用配置
	if err := h.manager.Reload(); err != nil {
		h.logger.Warnf("应用配置失败: %v", err)
		// 不返回错误，因为导出已创建
	}

	h.logger.Infof("创建导出成功: %s", req.Path)
	c.JSON(http.StatusOK, SuccessResponse(req))
}

// GetExport 获取单个导出
// @Summary 获取NFS导出详情
// @Tags NFS
// @Produce json
// @Param path path string true "导出路径"
// @Success 200 {object} Response
// @Router /nfs/exports/{path} [get]
func (h *Handlers) GetExport(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, "路径参数缺失"))
		return
	}

	export, err := h.manager.GetExport(path)
	if err != nil {
		h.logger.Debugf("获取导出失败: %s - %v", path, err)
		c.JSON(http.StatusNotFound, ErrorResponse(404, "导出不存在"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(export))
}

// UpdateExport 更新导出
// @Summary 更新NFS导出
// @Tags NFS
// @Accept json
// @Produce json
// @Param path path string true "导出路径"
// @Param export body Export true "导出配置"
// @Success 200 {object} Response
// @Router /nfs/exports/{path} [put]
func (h *Handlers) UpdateExport(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, "路径参数缺失"))
		return
	}

	var req Export
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Warnf("解析请求失败: %v", err)
		c.JSON(http.StatusBadRequest, ErrorResponse(400, "无效的请求参数: "+err.Error()))
		return
	}

	// 验证导出配置
	if err := h.manager.ValidateExport(&req); err != nil {
		h.logger.Warnf("验证导出配置失败: %v", err)
		c.JSON(http.StatusBadRequest, ErrorResponse(400, err.Error()))
		return
	}

	// 更新导出
	if err := h.manager.UpdateExport(path, &req); err != nil {
		h.logger.Errorf("更新导出失败: %v", err)
		c.JSON(http.StatusNotFound, ErrorResponse(404, err.Error()))
		return
	}

	// 应用配置
	if err := h.manager.Reload(); err != nil {
		h.logger.Warnf("应用配置失败: %v", err)
	}

	h.logger.Infof("更新导出成功: %s", path)
	c.JSON(http.StatusOK, SuccessResponse(req))
}

// DeleteExport 删除导出
// @Summary 删除NFS导出
// @Tags NFS
// @Param path path string true "导出路径"
// @Success 200 {object} Response
// @Router /nfs/exports/{path} [delete]
func (h *Handlers) DeleteExport(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse(400, "路径参数缺失"))
		return
	}

	if err := h.manager.DeleteExport(path); err != nil {
		h.logger.Errorf("删除导出失败: %v", err)
		c.JSON(http.StatusNotFound, ErrorResponse(404, err.Error()))
		return
	}

	// 应用配置
	if err := h.manager.Reload(); err != nil {
		h.logger.Warnf("应用配置失败: %v", err)
	}

	h.logger.Infof("删除导出成功: %s", path)
	c.JSON(http.StatusOK, SuccessResponse(nil))
}

// GetStatus 获取服务状态
// @Summary 获取NFS服务状态
// @Tags NFS
// @Produce json
// @Success 200 {object} Response
// @Router /nfs/status [get]
func (h *Handlers) GetStatus(c *gin.Context) {
	status, err := h.manager.Status()
	if err != nil {
		h.logger.Errorf("获取状态失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, "获取服务状态失败"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(status))
}

// StartService 启动服务
// @Summary 启动NFS服务
// @Tags NFS
// @Success 200 {object} Response
// @Router /nfs/start [post]
func (h *Handlers) StartService(c *gin.Context) {
	if err := h.manager.Start(); err != nil {
		h.logger.Errorf("启动服务失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, "启动服务失败: "+err.Error()))
		return
	}

	h.logger.Info("NFS服务已启动")
	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "服务已启动"}))
}

// StopService 停止服务
// @Summary 停止NFS服务
// @Tags NFS
// @Success 200 {object} Response
// @Router /nfs/stop [post]
func (h *Handlers) StopService(c *gin.Context) {
	if err := h.manager.Stop(); err != nil {
		h.logger.Errorf("停止服务失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, "停止服务失败: "+err.Error()))
		return
	}

	h.logger.Info("NFS服务已停止")
	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "服务已停止"}))
}

// RestartService 重启服务
// @Summary 重启NFS服务
// @Tags NFS
// @Success 200 {object} Response
// @Router /nfs/restart [post]
func (h *Handlers) RestartService(c *gin.Context) {
	if err := h.manager.Restart(); err != nil {
		h.logger.Errorf("重启服务失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, "重启服务失败: "+err.Error()))
		return
	}

	h.logger.Info("NFS服务已重启")
	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "服务已重启"}))
}

// ReloadConfig 重新加载配置
// @Summary 重新加载NFS配置
// @Tags NFS
// @Success 200 {object} Response
// @Router /nfs/reload [post]
func (h *Handlers) ReloadConfig(c *gin.Context) {
	if err := h.manager.Reload(); err != nil {
		h.logger.Errorf("重新加载配置失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, "重新加载配置失败: "+err.Error()))
		return
	}

	h.logger.Info("NFS配置已重新加载")
	c.JSON(http.StatusOK, SuccessResponse(gin.H{"message": "配置已重新加载"}))
}

// GetClients 获取连接的客户端
// @Summary 获取NFS客户端信息
// @Tags NFS
// @Produce json
// @Success 200 {object} Response
// @Router /nfs/clients [get]
func (h *Handlers) GetClients(c *gin.Context) {
	clients, err := h.manager.GetClients()
	if err != nil {
		h.logger.Errorf("获取客户端信息失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, "获取客户端信息失败"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{"clients": clients}))
}

// GetExportsFile 获取exports文件内容
// @Summary 获取exports文件内容
// @Tags NFS
// @Produce json
// @Success 200 {object} Response
// @Router /nfs/config/exports [get]
func (h *Handlers) GetExportsFile(c *gin.Context) {
	content, err := h.manager.GenerateExportsFile()
	if err != nil {
		h.logger.Errorf("生成exports文件失败: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse(500, "生成配置文件失败"))
		return
	}

	c.JSON(http.StatusOK, SuccessResponse(gin.H{
		"content": content,
		"path":    "/etc/exports",
	}))
}

// ExportRequest 创建/更新导出请求
type ExportRequest struct {
	Path    string   `json:"path" binding:"required"`
	Clients []Client `json:"clients"`
	Options struct {
		Ro           bool `json:"ro"`
		Rw           bool `json:"rw"`
		NoRootSquash bool `json:"no_root_squash"`
		Async        bool `json:"async"`
		Secure       bool `json:"secure"`
		SubtreeCheck bool `json:"subtree_check"`
	} `json:"options"`
	FSID    int    `json:"fsid"`
	Comment string `json:"comment"`
}

// ToExport 转换为Export结构
func (r *ExportRequest) ToExport() *Export {
	return &Export{
		Path:    r.Path,
		Clients: r.Clients,
		Options: ExportOptions{
			Ro:           r.Options.Ro,
			Rw:           r.Options.Rw,
			NoRootSquash: r.Options.NoRootSquash,
			Async:        r.Options.Async,
			Secure:       r.Options.Secure,
			SubtreeCheck: r.Options.SubtreeCheck,
		},
		FSID:    r.FSID,
		Comment: r.Comment,
	}
}
