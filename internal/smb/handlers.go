package smb

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

// Handlers SMB 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	shares := api.Group("/shares/smb")
	{
		shares.GET("", h.listShares)
		shares.POST("", h.createShare)
		shares.GET("/:name", h.getShare)
		shares.PUT("/:name", h.updateShare)
		shares.DELETE("/:name", h.deleteShare)
		shares.POST("/:name/permission", h.setPermission)
		shares.DELETE("/:name/permission/:user", h.removePermission)
	}

	api.GET("/shares/smb/user", h.getUserShares)
	api.POST("/shares/smb/restart", h.restartService)
	api.GET("/shares/smb/status", h.getStatus)
}

func (h *Handlers) listShares(c *gin.Context) {
	shares := h.manager.ListShares()
	c.JSON(http.StatusOK, Success(shares))
}

func (h *Handlers) createShare(c *gin.Context) {
	var req ShareInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	share, err := h.manager.CreateShare(req)
	if err != nil {
		c.JSON(http.StatusConflict, Error(409, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(share))
}

func (h *Handlers) getShare(c *gin.Context) {
	name := c.Param("name")
	share, err := h.manager.GetShare(name)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(share))
}

func (h *Handlers) updateShare(c *gin.Context) {
	name := c.Param("name")
	var req ShareInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	share, err := h.manager.UpdateShare(name, req)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(share))
}

func (h *Handlers) deleteShare(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.DeleteShare(name); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) setPermission(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Username   string `json:"username" binding:"required"`
		ReadWrite  bool   `json:"read_write"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.SetSharePermission(name, req.Username, req.ReadWrite); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) removePermission(c *gin.Context) {
	name := c.Param("name")
	username := c.Param("user")
	if err := h.manager.RemoveSharePermission(name, username); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

func (h *Handlers) getUserShares(c *gin.Context) {
	username := c.Query("user")
	if username == "" {
		c.JSON(http.StatusBadRequest, Error(400, "需要用户名"))
		return
	}

	shares := h.manager.GetUserShares(username)
	c.JSON(http.StatusOK, Success(shares))
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
