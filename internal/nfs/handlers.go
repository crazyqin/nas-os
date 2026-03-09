package nfs

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response 通用响应
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}

// Handlers NFS 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	exports := api.Group("/shares/nfs")
	{
		exports.GET("", h.listExports)
		exports.POST("", h.createExport)
		exports.GET("/:name", h.getExport)
		exports.PUT("/:name", h.updateExport)
		exports.DELETE("/:name", h.deleteExport)
	}

	api.POST("/shares/nfs/restart", h.restartService)
	api.GET("/shares/nfs/status", h.getStatus)
	api.GET("/shares/nfs/clients", h.getClients)
}

func (h *Handlers) listExports(c *gin.Context) {
	exports := h.manager.ListExports()
	c.JSON(http.StatusOK, Success(exports))
}

func (h *Handlers) createExport(c *gin.Context) {
	var req ExportInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	exp, err := h.manager.CreateExport(req)
	if err != nil {
		c.JSON(http.StatusConflict, Error(409, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(exp))
}

func (h *Handlers) getExport(c *gin.Context) {
	name := c.Param("name")
	exp, err := h.manager.GetExport(name)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(exp))
}

func (h *Handlers) updateExport(c *gin.Context) {
	name := c.Param("name")
	var req ExportInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	exp, err := h.manager.UpdateExport(name, req)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(exp))
}

func (h *Handlers) deleteExport(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.DeleteExport(name); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) restartService(c *gin.Context) {
	if err := h.manager.Restart(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) getStatus(c *gin.Context) {
	running, err := h.manager.GetStatus()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(gin.H{"running": running}))
}

func (h *Handlers) getClients(c *gin.Context) {
	clients, err := h.manager.GetClientInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(gin.H{"clients": clients}))
}
