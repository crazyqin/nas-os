// Package dockerorch 实现容器编排管理器 HTTP API
package dockerorch

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 容器编排 HTTP 处理器
type Handler struct {
	orchestrator *Orchestrator
}

// NewHandler 创建处理器
func NewHandler(orchestrator *Orchestrator) *Handler {
	return &Handler{orchestrator: orchestrator}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	docker := rg.Group("/docker-orch")
	{
		// 容器管理
		docker.GET("/containers", h.ListContainers)
		docker.POST("/containers", h.CreateContainer)
		docker.GET("/containers/:id", h.GetContainer)
		docker.POST("/containers/:id/start", h.StartContainer)
		docker.POST("/containers/:id/stop", h.StopContainer)
		docker.DELETE("/containers/:id", h.DeleteContainer)

		// 服务管理
		docker.GET("/services", h.ListServices)

		// 栈管理
		docker.GET("/stacks", h.ListStacks)
	}
}

// ListContainers 获取容器列表
func (h *Handler) ListContainers(c *gin.Context) {
	status := ContainerStatus(c.Query("status"))
	containers := h.orchestrator.ListContainers(status)

	c.JSON(http.StatusOK, gin.H{
		"containers": containers,
		"total":      len(containers),
	})
}

// CreateContainerRequest 创建容器请求
type CreateContainerRequest struct {
	ID            string            `json:"id" binding:"required"`
	Name          string            `json:"name" binding:"required"`
	Image         string            `json:"image" binding:"required"`
	Tag           string            `json:"tag"`
	Environment   map[string]string `json:"environment"`
	Labels        map[string]string `json:"labels"`
	Network       NetworkMode       `json:"network"`
	RestartPolicy RestartPolicy     `json:"restartPolicy"`
	CPUQuota      float64           `json:"cpuQuota"`
	MemoryLimit   int64             `json:"memoryLimit"`
}

// CreateContainer 创建容器
func (h *Handler) CreateContainer(c *gin.Context) {
	var req CreateContainerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Tag == "" {
		req.Tag = "latest"
	}
	if req.Network == "" {
		req.Network = NetworkModeBridge
	}
	if req.RestartPolicy == "" {
		req.RestartPolicy = RestartPolicyNo
	}

	container := Container{
		ID:            req.ID,
		Name:          req.Name,
		Image:         req.Image,
		Tag:           req.Tag,
		Status:        ContainerStatusCreated,
		Environment:   req.Environment,
		Labels:        req.Labels,
		Network:       req.Network,
		RestartPolicy: req.RestartPolicy,
		CPUQuota:      req.CPUQuota,
		MemoryLimit:   req.MemoryLimit,
	}

	if err := h.orchestrator.CreateContainer(container); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, container)
}

// GetContainer 获取容器详情
func (h *Handler) GetContainer(c *gin.Context) {
	id := c.Param("id")
	container, err := h.orchestrator.GetContainer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, container)
}

// StartContainer 启动容器
func (h *Handler) StartContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.orchestrator.StartContainer(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器已启动", "id": id})
}

// StopContainer 停止容器
func (h *Handler) StopContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.orchestrator.StopContainer(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器已停止", "id": id})
}

// DeleteContainer 删除容器
func (h *Handler) DeleteContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.orchestrator.RemoveContainer(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "容器已删除", "id": id})
}

// ListServices 获取服务列表
func (h *Handler) ListServices(c *gin.Context) {
	services := h.orchestrator.ListServices()

	c.JSON(http.StatusOK, gin.H{
		"services": services,
		"total":    len(services),
	})
}

// ListStacks 获取栈列表
func (h *Handler) ListStacks(c *gin.Context) {
	stacks := h.orchestrator.ListStacks()

	c.JSON(http.StatusOK, gin.H{
		"stacks": stacks,
		"total":  len(stacks),
	})
}
