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
		shares.POST("/:name/close", h.closeShare)
		shares.POST("/:name/open", h.openShare)
	}

	// 状态和管理
	api.GET("/smb/status", h.getStatus)
	api.GET("/smb/connections", h.getConnections)
	api.POST("/smb/start", h.startService)
	api.POST("/smb/stop", h.stopService)
	api.POST("/smb/restart", h.restartService)
	api.POST("/smb/reload", h.reloadConfig)
	api.GET("/smb/test", h.testConfig)

	// 用户相关
	api.GET("/shares/smb/user", h.getUserShares)

	// 配置
	api.GET("/smb/config", h.getConfig)
	api.PUT("/smb/config", h.updateConfig)
}

// listShares 列出所有共享
func (h *Handlers) listShares(c *gin.Context) {
	shares, err := h.manager.ListShares()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(shares))
}

// createShare 创建共享
func (h *Handlers) createShare(c *gin.Context) {
	var req ShareInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	share := &Share{
		Name:          req.Name,
		Path:          req.Path,
		Comment:       req.Comment,
		Browseable:    req.Browseable,
		ReadOnly:      req.ReadOnly,
		GuestOK:       req.GuestOK,
		GuestAccess:   req.GuestAccess,
		Users:         req.Users,
		ValidUsers:    req.ValidUsers,
		WriteList:     req.WriteList,
		CreateMask:    req.CreateMask,
		DirectoryMask: req.DirectoryMask,
		VetoFiles:     req.VetoFiles,
	}

	if err := h.manager.CreateShare(share); err != nil {
		c.JSON(http.StatusConflict, Error(409, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(share))
}

// getShare 获取单个共享
func (h *Handlers) getShare(c *gin.Context) {
	name := c.Param("name")
	share, err := h.manager.GetShare(name)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(share))
}

// updateShare 更新共享
func (h *Handlers) updateShare(c *gin.Context) {
	name := c.Param("name")
	var req ShareInput
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	share := &Share{
		Path:          req.Path,
		Comment:       req.Comment,
		Browseable:    req.Browseable,
		ReadOnly:      req.ReadOnly,
		GuestOK:       req.GuestOK,
		GuestAccess:   req.GuestAccess,
		Users:         req.Users,
		ValidUsers:    req.ValidUsers,
		WriteList:     req.WriteList,
		CreateMask:    req.CreateMask,
		DirectoryMask: req.DirectoryMask,
		VetoFiles:     req.VetoFiles,
	}

	if err := h.manager.UpdateShare(name, share); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	updatedShare, _ := h.manager.GetShare(name)
	c.JSON(http.StatusOK, Success(updatedShare))
}

// deleteShare 删除共享
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

// setPermission 设置权限
func (h *Handlers) setPermission(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		Username  string `json:"username" binding:"required"`
		ReadWrite bool   `json:"read_write"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.SetSharePermission(name, req.Username, req.ReadWrite); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// removePermission 移除权限
func (h *Handlers) removePermission(c *gin.Context) {
	name := c.Param("name")
	username := c.Param("user")
	if err := h.manager.RemoveSharePermission(name, username); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// closeShare 关闭共享
func (h *Handlers) closeShare(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.CloseShare(name); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// openShare 打开共享
func (h *Handlers) openShare(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.OpenShare(name); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// getStatus 获取服务状态
func (h *Handlers) getStatus(c *gin.Context) {
	status, err := h.manager.Status()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(status))
}

// getConnections 获取当前连接
func (h *Handlers) getConnections(c *gin.Context) {
	connections, err := h.manager.Connections()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(connections))
}

// startService 启动服务
func (h *Handlers) startService(c *gin.Context) {
	if err := h.manager.Start(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

// stopService 停止服务
func (h *Handlers) stopService(c *gin.Context) {
	if err := h.manager.Stop(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

// restartService 重启服务
func (h *Handlers) restartService(c *gin.Context) {
	if err := h.manager.Restart(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

// reloadConfig 重新加载配置
func (h *Handlers) reloadConfig(c *gin.Context) {
	if err := h.manager.Reload(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}
	c.JSON(http.StatusOK, Success(nil))
}

// testConfig 测试配置
func (h *Handlers) testConfig(c *gin.Context) {
	ok, output, err := h.manager.TestConfig()
	c.JSON(http.StatusOK, Success(gin.H{
		"valid":  ok,
		"output": output,
		"error":  errToString(err),
	}))
}

// getUserShares 获取用户可访问的共享
func (h *Handlers) getUserShares(c *gin.Context) {
	username := c.Query("user")
	if username == "" {
		c.JSON(http.StatusBadRequest, Error(400, "需要用户名"))
		return
	}

	shares := h.manager.GetUserShares(username)
	c.JSON(http.StatusOK, Success(shares))
}

// getConfig 获取全局配置
func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, Success(config))
}

// updateConfig 更新全局配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var req Config
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.UpdateConfig(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.manager.ApplyConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "应用配置失败："+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// errToString 将错误转换为字符串
func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
