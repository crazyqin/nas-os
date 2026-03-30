package ftp

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers FTP API 处理器.
type Handlers struct {
	server *Server
}

// NewHandlers 创建 FTP handlers.
func NewHandlers(server *Server) *Handlers {
	return &Handlers{server: server}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	ftp := api.Group("/ftp")
	{
		ftp.GET("/config", h.GetConfig)
		ftp.PUT("/config", h.UpdateConfig)
		ftp.GET("/status", h.GetStatus)
		ftp.POST("/restart", h.Restart)
	}
}

// GetConfig 获取 FTP 配置
// @Summary 获取 FTP 配置
// @Description 获取 FTP 服务器配置
// @Tags ftp
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ftp/config [get].
func (h *Handlers) GetConfig(c *gin.Context) {
	if h.server == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "FTP 服务未初始化",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    h.server.GetConfig(),
	})
}

// UpdateConfig 更新 FTP 配置
// @Summary 更新 FTP 配置
// @Description 更新 FTP 服务器配置
// @Tags ftp
// @Accept json
// @Produce json
// @Param config body Config true "配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ftp/config [put].
func (h *Handlers) UpdateConfig(c *gin.Context) {
	if h.server == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "FTP 服务未初始化",
		})
		return
	}

	var config Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.server.UpdateConfig(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    h.server.GetConfig(),
	})
}

// GetStatus 获取 FTP 状态
// @Summary 获取 FTP 状态
// @Description 获取 FTP 服务器运行状态
// @Tags ftp
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ftp/status [get].
func (h *Handlers) GetStatus(c *gin.Context) {
	if h.server == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "FTP 服务未初始化",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    h.server.GetStatus(),
	})
}

// Start 启动 FTP 服务器
// @Summary 启动 FTP 服务器
// @Description 启动 FTP 服务器
// @Tags ftp
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ftp/start [post].
func (h *Handlers) Start(c *gin.Context) {
	if h.server == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "FTP 服务未初始化",
		})
		return
	}

	config := h.server.GetConfig()
	config.Enabled = true
	if err := h.server.UpdateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "FTP 服务器已启动",
	})
}

// Restart 重启 FTP 服务器
// @Summary 重启 FTP 服务器
// @Description 重启 FTP 服务器
// @Tags ftp
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ftp/restart [post].
func (h *Handlers) Restart(c *gin.Context) {
	if h.server == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "FTP 服务未初始化",
		})
		return
	}

	// 停止服务器
	if err := h.server.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "停止服务器失败: " + err.Error(),
		})
		return
	}

	// 重新启动
	config := h.server.GetConfig()
	config.Enabled = true
	if err := h.server.UpdateConfig(config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "启动服务器失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "FTP 服务器已重启",
	})
}
