// Package containerpro 提供容器管理功能
package containerpro

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 容器管理 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	v1 := r.Group("/v1")
	{
		// 容器管理
		v1.GET("/containers", h.listContainers)
		v1.GET("/containers/:id", h.getContainer)
		v1.POST("/containers/:id/start", h.startContainer)
		v1.POST("/containers/:id/stop", h.stopContainer)
		v1.POST("/containers/:id/restart", h.restartContainer)
		v1.DELETE("/containers/:id", h.removeContainer)
		v1.GET("/containers/:id/stats", h.getContainerStats)

		// Compose 管理
		v1.POST("/compose/deploy", h.deployComposeProject)
		v1.GET("/compose/projects", h.listComposeProjects)
		v1.POST("/compose/projects/:id/stop", h.stopComposeProject)

		// 镜像管理
		v1.POST("/images/pull", h.pullImage)
		v1.GET("/images", h.listImages)

		// 仓库管理
		v1.POST("/registries", h.addRegistry)
		v1.GET("/registries", h.listRegistries)
	}
}

// listContainers 列出容器.
func (h *Handlers) listContainers(c *gin.Context) {
	all, _ := strconv.ParseBool(c.DefaultQuery("all", "false"))

	containers, err := h.manager.ListContainers(all)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": containers})
}

// getContainer 获取容器详情.
func (h *Handlers) getContainer(c *gin.Context) {
	id := c.Param("id")

	container, err := h.manager.GetContainer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": container})
}

// startContainer 启动容器.
func (h *Handlers) startContainer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.StartContainer(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "container started"})
}

// stopContainer 停止容器.
func (h *Handlers) stopContainer(c *gin.Context) {
	id := c.Param("id")
	timeout, _ := strconv.Atoi(c.DefaultQuery("timeout", "10"))

	if err := h.manager.StopContainer(id, timeout); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "container stopped"})
}

// restartContainer 重启容器.
func (h *Handlers) restartContainer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.RestartContainer(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "container restarted"})
}

// removeContainer 删除容器.
func (h *Handlers) removeContainer(c *gin.Context) {
	id := c.Param("id")
	force, _ := strconv.ParseBool(c.DefaultQuery("force", "false"))

	if err := h.manager.RemoveContainer(id, force); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "container removed"})
}

// getContainerStats 获取容器统计.
func (h *Handlers) getContainerStats(c *gin.Context) {
	id := c.Param("id")

	stats, err := h.manager.GetContainerStats(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": stats})
}

// deployComposeProject 部署 Compose 项目.
func (h *Handlers) deployComposeProject(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := h.manager.DeployComposeProject(req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": project})
}

// listComposeProjects 列出 Compose 项目.
func (h *Handlers) listComposeProjects(c *gin.Context) {
	projects, err := h.manager.ListComposeProjects()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": projects})
}

// stopComposeProject 停止 Compose 项目.
func (h *Handlers) stopComposeProject(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.StopComposeProject(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "compose project stopped"})
}

// pullImage 拉取镜像.
func (h *Handlers) pullImage(c *gin.Context) {
	var req struct {
		Image      string `json:"image" binding:"required"`
		RegistryID string `json:"registry_id"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.PullImage(req.Image, req.RegistryID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "image pulled successfully"})
}

// listImages 列出镜像.
func (h *Handlers) listImages(c *gin.Context) {
	images, err := h.manager.ListImages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": images})
}

// addRegistry 添加仓库.
func (h *Handlers) addRegistry(c *gin.Context) {
	var registry Registry

	if err := c.ShouldBindJSON(&registry); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.AddRegistry(registry); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "registry added"})
}

// listRegistries 列出仓库.
func (h *Handlers) listRegistries(c *gin.Context) {
	registries, err := h.manager.ListRegistries()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": registries})
}
