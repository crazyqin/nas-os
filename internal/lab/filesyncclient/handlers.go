// Package filesyncclient 提供 REST API 处理器
package filesyncclient

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 文件同步 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	sync := r.Group("/filesync")
	{
		// 客户端管理
		sync.POST("/clients", h.registerClient)
		sync.GET("/clients", h.listClients)
		sync.DELETE("/clients/:id", h.removeClient)

		// 同步文件夹管理
		sync.POST("/folders", h.createFolder)
		sync.GET("/folders", h.listFolders)
		sync.PUT("/folders/:id", h.updateFolder)

		// 同步操作
		sync.POST("/sync/:folderId", h.triggerSync)

		// 冲突管理
		sync.GET("/conflicts", h.listConflicts)
		sync.POST("/conflicts/:id/resolve", h.resolveConflict)

		// 统计和事件
		sync.GET("/stats", h.getStats)
		sync.GET("/events", h.getEvents)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// registerClient 注册客户端.
func (h *Handlers) registerClient(c *gin.Context) {
	var req RegisterClientRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	client, err := h.manager.RegisterClient(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "client registered",
		Data:    client,
	})
}

// listClients 列出客户端.
func (h *Handlers) listClients(c *gin.Context) {
	clients := h.manager.ListClients()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    clients,
	})
}

// removeClient 移除客户端.
func (h *Handlers) removeClient(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RemoveClient(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "client removed",
	})
}

// createFolder 创建同步文件夹.
func (h *Handlers) createFolder(c *gin.Context) {
	var req CreateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	folder, err := h.manager.CreateSyncFolder(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "folder created",
		Data:    folder,
	})
}

// listFolders 列出同步文件夹.
func (h *Handlers) listFolders(c *gin.Context) {
	folders := h.manager.ListFolders()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    folders,
	})
}

// updateFolder 更新同步文件夹.
func (h *Handlers) updateFolder(c *gin.Context) {
	id := c.Param("id")
	var req UpdateFolderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	folder, err := h.manager.UpdateFolder(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "folder updated",
		Data:    folder,
	})
}

// triggerSync 触发同步.
func (h *Handlers) triggerSync(c *gin.Context) {
	folderID := c.Param("folderId")
	if err := h.manager.TriggerSync(folderID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "sync triggered",
	})
}

// listConflicts 列出冲突.
func (h *Handlers) listConflicts(c *gin.Context) {
	conflicts := h.manager.ListConflicts()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    conflicts,
	})
}

// resolveConflict 解决冲突.
func (h *Handlers) resolveConflict(c *gin.Context) {
	id := c.Param("id")
	var req ResolveConflictRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.ResolveConflict(id, &req); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "conflict resolved",
	})
}

// getStats 获取统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// getEvents 获取事件.
func (h *Handlers) getEvents(c *gin.Context) {
	clientID := c.Query("clientID")
	events := h.manager.GetEvents(clientID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    events,
	})
}
