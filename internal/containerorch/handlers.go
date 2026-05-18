// Package containerorch 提供 K3s 轻量级容器编排功能
package containerorch

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 容器编排 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	co := r.Group("/container-orch")
	{
		// 集群统计
		co.GET("/stats", h.getStats)

		// 容器管理
		co.GET("/containers", h.listContainers)
		co.POST("/containers", h.createContainer)
		co.GET("/containers/:id", h.getContainer)
		co.POST("/containers/:id/start", h.startContainer)
		co.POST("/containers/:id/stop", h.stopContainer)
		co.POST("/containers/:id/restart", h.restartContainer)
		co.DELETE("/containers/:id", h.removeContainer)
		co.GET("/containers/:id/logs", h.getContainerLogs)

		// Pod 管理
		co.GET("/pods", h.listPods)
		co.POST("/pods", h.createPod)
		co.GET("/pods/:id", h.getPod)
		co.POST("/pods/:id/start", h.startPod)
		co.POST("/pods/:id/stop", h.stopPod)
		co.DELETE("/pods/:id", h.removePod)

		// Deployment 管理
		co.GET("/deployments", h.listDeployments)
		co.POST("/deployments", h.createDeployment)
		co.GET("/deployments/:id", h.getDeployment)
		co.PUT("/deployments/:id/scale", h.scaleDeployment)
		co.DELETE("/deployments/:id", h.deleteDeployment)

		// Service 管理
		co.GET("/services", h.listServices)
		co.POST("/services", h.createService)
		co.GET("/services/:id", h.getService)
		co.DELETE("/services/:id", h.deleteService)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ==================== 集群统计 ====================

// getStats 获取集群统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// ==================== 容器管理 ====================

// listContainers 列出容器.
func (h *Handlers) listContainers(c *gin.Context) {
	containers := h.manager.ListContainers()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":      len(containers),
			"containers": containers,
		},
	})
}

// createContainer 创建容器.
func (h *Handlers) createContainer(c *gin.Context) {
	var req CreateContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	container, err := h.manager.CreateContainer(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "container created",
		Data:    container,
	})
}

// getContainer 获取容器.
func (h *Handlers) getContainer(c *gin.Context) {
	id := c.Param("id")
	container, ok := h.manager.GetContainer(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "container not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    container,
	})
}

// startContainer 启动容器.
func (h *Handlers) startContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartContainer(id); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "container started",
	})
}

// stopContainer 停止容器.
func (h *Handlers) stopContainer(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Timeout *int `json:"timeout"`
	}
	c.ShouldBindJSON(&req)

	if err := h.manager.StopContainer(id, req.Timeout); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "container stopped",
	})
}

// restartContainer 重启容器.
func (h *Handlers) restartContainer(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Timeout *int `json:"timeout"`
	}
	c.ShouldBindJSON(&req)

	if err := h.manager.RestartContainer(id, req.Timeout); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "container restarted",
	})
}

// removeContainer 删除容器.
func (h *Handlers) removeContainer(c *gin.Context) {
	id := c.Param("id")
	force, _ := strconv.ParseBool(c.Query("force"))

	if err := h.manager.RemoveContainer(id, force); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "container removed",
	})
}

// getContainerLogs 获取容器日志.
func (h *Handlers) getContainerLogs(c *gin.Context) {
	id := c.Param("id")

	// 解析查询参数
	opts := &LogOptions{
		Follow:     c.Query("follow") == "true",
		Timestamps: c.Query("timestamps") == "true",
	}
	if tail := c.Query("tail"); tail != "" {
		if n, err := strconv.Atoi(tail); err == nil {
			opts.Tail = n
		}
	}

	logs, err := h.manager.GetContainerLogs(id, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"containerId": id,
			"logs":        logs,
		},
	})
}

// ==================== Pod 管理 ====================

// listPods 列出 Pod.
func (h *Handlers) listPods(c *gin.Context) {
	namespace := c.Query("namespace")
	pods := h.manager.ListPods(namespace)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(pods),
			"pods":  pods,
		},
	})
}

// createPod 创建 Pod.
func (h *Handlers) createPod(c *gin.Context) {
	var req CreatePodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	pod, err := h.manager.CreatePod(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "pod created",
		Data:    pod,
	})
}

// getPod 获取 Pod.
func (h *Handlers) getPod(c *gin.Context) {
	id := c.Param("id")
	pod, ok := h.manager.GetPod(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "pod not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    pod,
	})
}

// startPod 启动 Pod.
func (h *Handlers) startPod(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartPod(id); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "pod started",
	})
}

// stopPod 停止 Pod.
func (h *Handlers) stopPod(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopPod(id); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "pod stopped",
	})
}

// removePod 删除 Pod.
func (h *Handlers) removePod(c *gin.Context) {
	id := c.Param("id")
	force, _ := strconv.ParseBool(c.Query("force"))

	if err := h.manager.RemovePod(id, force); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "pod removed",
	})
}

// ==================== Deployment 管理 ====================

// listDeployments 列出 Deployment.
func (h *Handlers) listDeployments(c *gin.Context) {
	namespace := c.Query("namespace")
	deployments := h.manager.ListDeployments(namespace)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":        len(deployments),
			"deployments":  deployments,
		},
	})
}

// createDeployment 创建 Deployment.
func (h *Handlers) createDeployment(c *gin.Context) {
	var req CreateDeploymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	deployment, err := h.manager.CreateDeployment(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "deployment created",
		Data:    deployment,
	})
}

// getDeployment 获取 Deployment.
func (h *Handlers) getDeployment(c *gin.Context) {
	id := c.Param("id")
	deployment, ok := h.manager.GetDeployment(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "deployment not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    deployment,
	})
}

// ScaleRequest 扩缩容请求.
type ScaleRequest struct {
	Replicas int `json:"replicas"` // 目标副本数
}

// scaleDeployment 扩缩容 Deployment.
func (h *Handlers) scaleDeployment(c *gin.Context) {
	id := c.Param("id")
	var req ScaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.ScaleDeployment(id, req.Replicas); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "deployment scaled",
	})
}

// deleteDeployment 删除 Deployment.
func (h *Handlers) deleteDeployment(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteDeployment(id); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "deployment deleted",
	})
}

// ==================== Service 管理 ====================

// listServices 列出 Service.
func (h *Handlers) listServices(c *gin.Context) {
	namespace := c.Query("namespace")
	services := h.manager.ListServices(namespace)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(services),
			"services": services,
		},
	})
}

// createService 创建 Service.
func (h *Handlers) createService(c *gin.Context) {
	var req CreateServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	service, err := h.manager.CreateService(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "service created",
		Data:    service,
	})
}

// getService 获取 Service.
func (h *Handlers) getService(c *gin.Context) {
	id := c.Param("id")
	service, ok := h.manager.GetService(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "service not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    service,
	})
}

// deleteService 删除 Service.
func (h *Handlers) deleteService(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteService(id); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "service deleted",
	})
}
