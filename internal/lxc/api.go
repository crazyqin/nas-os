package lxc

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// APIHandler LXC 容器 REST API 处理器.
type APIHandler struct {
	manager *Manager
}

// NewAPIHandler 创建 API 处理器.
func NewAPIHandler(manager *Manager) *APIHandler {
	return &APIHandler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *APIHandler) RegisterRoutes(r *gin.RouterGroup) {
	api := r.Group("/api/v1/lxc")
	{
		// 系统状态
		api.GET("/status", h.getStatus)

		// 容器生命周期管理
		api.GET("/containers", h.listContainers)
		api.POST("/containers", h.createContainer)
		api.GET("/containers/:id", h.getContainer)
		api.PUT("/containers/:id", h.updateContainer)
		api.DELETE("/containers/:id", h.deleteContainer)
		api.POST("/containers/:id/start", h.startContainer)
		api.POST("/containers/:id/stop", h.stopContainer)
		api.POST("/containers/:id/restart", h.restartContainer)
		api.GET("/containers/:id/stats", h.getContainerStats)

		// 批量操作
		api.POST("/containers/batch/start", h.batchStart)
		api.POST("/containers/batch/stop", h.batchStop)
		api.POST("/containers/batch/delete", h.batchDelete)

		// 快照管理
		api.GET("/containers/:id/snapshots", h.listSnapshots)
		api.POST("/containers/:id/snapshots", h.createSnapshot)
		api.POST("/snapshots/:id/restore", h.restoreSnapshot)
		api.DELETE("/snapshots/:id", h.deleteSnapshot)

		// 网络管理
		api.POST("/containers/:id/network/isolate", h.isolateNetwork)
		api.POST("/containers/:id/ports", h.addPortMapping)
		api.DELETE("/containers/:id/ports/:port", h.removePortMapping)

		// 存储卷管理
		api.POST("/containers/:id/volumes", h.addVolume)
		api.DELETE("/containers/:id/volumes", h.removeVolume)

		// 高可用
		api.POST("/containers/:id/ha/enable", h.enableHA)
		api.POST("/containers/:id/ha/disable", h.disableHA)

		// 模板管理
		api.GET("/templates", h.listTemplates)
		api.GET("/templates/:name", h.getTemplate)
		api.POST("/templates", h.createTemplate)
		api.PUT("/templates/:name", h.updateTemplate)
		api.DELETE("/templates/:name", h.deleteTemplate)
	}
}

// ========== 系统状态 ==========

func (h *APIHandler) getStatus(c *gin.Context) {
	summary := h.manager.GetStatusSummary()
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    summary,
	})
}

// ========== 容器生命周期管理 ==========

func (h *APIHandler) listContainers(c *gin.Context) {
	statusFilter := c.Query("status")

	var containers []*Container
	if statusFilter != "" {
		containers = h.manager.ListByStatus(ContainerStatus(statusFilter))
	} else {
		containers = h.manager.ListContainers()
	}

	// 统计
	var running, stopped, errCount int
	for _, ct := range containers {
		switch ct.Status {
		case StatusRunning:
			running++
		case StatusStopped, StatusCreated:
			stopped++
		case StatusError:
			errCount++
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: ContainerListResponse{
			Total:      len(containers),
			Running:    running,
			Stopped:    stopped,
			Error:      errCount,
			Containers: containers,
		},
	})
}

func (h *APIHandler) createContainer(c *gin.Context) {
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	container, err := h.manager.CreateContainer(req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "容器创建成功",
		Data:    container,
	})
}

func (h *APIHandler) getContainer(c *gin.Context) {
	id := c.Param("id")

	container, err := h.manager.GetContainer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    container,
	})
}

func (h *APIHandler) updateContainer(c *gin.Context) {
	id := c.Param("id")

	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	container, err := h.manager.GetContainer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	if req.Resources != nil {
		if err := h.manager.UpdateResources(id, *req.Resources); err != nil {
			c.JSON(http.StatusBadRequest, APIResponse{
				Success: false,
				Error:   err.Error(),
			})
			return
		}
	}

	if req.Tags != nil {
		container.Tags = req.Tags
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "容器更新成功",
		Data:    container,
	})
}

func (h *APIHandler) deleteContainer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteContainer(id); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "容器删除成功",
	})
}

func (h *APIHandler) startContainer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.StartContainer(id); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "容器启动成功",
	})
}

func (h *APIHandler) stopContainer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.StopContainer(id); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "容器停止成功",
	})
}

func (h *APIHandler) restartContainer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.RestartContainer(id); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "容器重启成功",
	})
}

func (h *APIHandler) getContainerStats(c *gin.Context) {
	id := c.Param("id")

	stats, err := h.manager.GetStats(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}

// ========== 批量操作 ==========

func (h *APIHandler) batchStart(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	result := h.manager.BatchStart(req.ContainerIDs)

	var succeeded, failed int
	for _, err := range result {
		if err != nil {
			failed++
		} else {
			succeeded++
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "批量启动完成",
		Data: gin.H{
			"succeeded": succeeded,
			"failed":    failed,
			"details":   result,
		},
	})
}

func (h *APIHandler) batchStop(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	result := h.manager.BatchStop(req.ContainerIDs)

	var succeeded, failed int
	for _, err := range result {
		if err != nil {
			failed++
		} else {
			succeeded++
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "批量停止完成",
		Data: gin.H{
			"succeeded": succeeded,
			"failed":    failed,
			"details":   result,
		},
	})
}

func (h *APIHandler) batchDelete(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	result := h.manager.BatchDelete(req.ContainerIDs)

	var succeeded, failed int
	for _, err := range result {
		if err != nil {
			failed++
		} else {
			succeeded++
		}
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "批量删除完成",
		Data: gin.H{
			"succeeded": succeeded,
			"failed":    failed,
			"details":   result,
		},
	})
}

// ========== 快照管理 ==========

func (h *APIHandler) listSnapshots(c *gin.Context) {
	containerID := c.Param("id")

	snapshots, err := h.manager.ListSnapshots(containerID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
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

func (h *APIHandler) createSnapshot(c *gin.Context) {
	containerID := c.Param("id")

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	snapshot, err := h.manager.CreateSnapshot(containerID, req.Name, req.Description)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "快照创建成功",
		Data:    snapshot,
	})
}

func (h *APIHandler) restoreSnapshot(c *gin.Context) {
	snapshotID := c.Param("id")

	if err := h.manager.RestoreSnapshot(snapshotID); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "快照恢复成功",
	})
}

func (h *APIHandler) deleteSnapshot(c *gin.Context) {
	snapshotID := c.Param("id")

	if err := h.manager.DeleteSnapshot(snapshotID); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "快照删除成功",
	})
}

// ========== 网络管理 ==========

func (h *APIHandler) isolateNetwork(c *gin.Context) {
	containerID := c.Param("id")

	if err := h.manager.CreateIsolatedNetwork(containerID); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "网络隔离配置成功",
	})
}

func (h *APIHandler) addPortMapping(c *gin.Context) {
	containerID := c.Param("id")

	var port PortMap
	if err := c.ShouldBindJSON(&port); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.manager.AddPortMapping(containerID, port); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "端口映射添加成功",
	})
}

func (h *APIHandler) removePortMapping(c *gin.Context) {
	containerID := c.Param("id")
	portStr := c.Param("port")
	protocol := c.Query("protocol")
	if protocol == "" {
		protocol = "tcp"
	}

	var portNum int
	if _, err := fmt.Sscanf(portStr, "%d", &portNum); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "无效的端口号",
		})
		return
	}

	if err := h.manager.RemovePortMapping(containerID, portNum, protocol); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "端口映射移除成功",
	})
}

// ========== 存储卷管理 ==========

func (h *APIHandler) addVolume(c *gin.Context) {
	containerID := c.Param("id")

	var vol VolumeMount
	if err := c.ShouldBindJSON(&vol); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.manager.AddVolume(containerID, vol); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "存储卷添加成功",
	})
}

func (h *APIHandler) removeVolume(c *gin.Context) {
	containerID := c.Param("id")
	destination := c.Query("destination")

	if destination == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请指定挂载点路径",
		})
		return
	}

	if err := h.manager.RemoveVolume(containerID, destination); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "存储卷移除成功",
	})
}

// ========== 高可用 ==========

func (h *APIHandler) enableHA(c *gin.Context) {
	containerID := c.Param("id")

	var haCfg HAConfig
	if err := c.ShouldBindJSON(&haCfg); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.manager.EnableHA(containerID, haCfg); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "高可用配置成功",
	})
}

func (h *APIHandler) disableHA(c *gin.Context) {
	containerID := c.Param("id")

	if err := h.manager.DisableHA(containerID); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "高可用已禁用",
	})
}

// ========== 模板管理 ==========

func (h *APIHandler) listTemplates(c *gin.Context) {
	category := c.Query("category")
	distro := c.Query("distro")

	var templates []*Template
	if category != "" {
		templates = h.manager.TemplateManager().ListByCategory(TemplateCategory(category))
	} else if distro != "" {
		templates = h.manager.TemplateManager().ListByDistro(distro)
	} else {
		templates = h.manager.TemplateManager().List()
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    templates,
	})
}

func (h *APIHandler) getTemplate(c *gin.Context) {
	name := c.Param("name")

	tmpl, err := h.manager.TemplateManager().Get(name)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    tmpl,
	})
}

func (h *APIHandler) createTemplate(c *gin.Context) {
	var tmpl Template
	if err := c.ShouldBindJSON(&tmpl); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.manager.TemplateManager().Register(&tmpl); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Message: "模板创建成功",
		Data:    tmpl,
	})
}

func (h *APIHandler) updateTemplate(c *gin.Context) {
	name := c.Param("name")

	var tmpl Template
	if err := c.ShouldBindJSON(&tmpl); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.manager.TemplateManager().Update(name, &tmpl); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "模板更新成功",
	})
}

func (h *APIHandler) deleteTemplate(c *gin.Context) {
	name := c.Param("name")

	if err := h.manager.TemplateManager().Delete(name); err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "模板删除成功",
	})
}
