package lxcmanager

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler LXC Manager HTTP 处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建 LXC Manager 处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	lxc := rg.Group("/lxcmanager")
	{
		lxc.GET("/containers", h.ListContainers)
		lxc.GET("/containers/:name", h.GetContainer)
		lxc.POST("/containers", h.CreateContainer)
		lxc.DELETE("/containers/:name", h.DestroyContainer)
		lxc.POST("/containers/:name/start", h.StartContainer)
		lxc.POST("/containers/:name/stop", h.StopContainer)
		lxc.POST("/containers/:name/pause", h.PauseContainer)
		lxc.PUT("/containers/:name/resources", h.SetResourceLimits)
		lxc.PUT("/containers/:name/network", h.ConfigureNetwork)

		lxc.GET("/templates", h.ListTemplates)
		lxc.POST("/templates", h.RegisterTemplate)
		lxc.DELETE("/templates/:name", h.DeleteTemplate)
	}
}

// ListContainers 列出所有容器
// GET /api/v1/lxcmanager/containers.
func (h *Handler) ListContainers(c *gin.Context) {
	containers := h.manager.ListContainers()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    containers,
	})
}

// GetContainer 获取容器详情
// GET /api/v1/lxcmanager/containers/:name.
func (h *Handler) GetContainer(c *gin.Context) {
	name := c.Param("name")
	info, err := h.manager.GetContainer(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    info,
	})
}

// CreateContainer 创建容器
// POST /api/v1/lxcmanager/containers.
func (h *Handler) CreateContainer(c *gin.Context) {
	var cfg ContainerConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	info, err := h.manager.CreateContainer(c.Request.Context(), cfg)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "ok",
		"data":    info,
	})
}

// DestroyContainer 销毁容器
// DELETE /api/v1/lxcmanager/containers/:name.
func (h *Handler) DestroyContainer(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.DestroyContainer(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// StartContainer 启动容器
// POST /api/v1/lxcmanager/containers/:name/start.
func (h *Handler) StartContainer(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.StartContainer(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// StopContainer 停止容器
// POST /api/v1/lxcmanager/containers/:name/stop.
func (h *Handler) StopContainer(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.StopContainer(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// PauseContainer 暂停容器
// POST /api/v1/lxcmanager/containers/:name/pause.
func (h *Handler) PauseContainer(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.PauseContainer(c.Request.Context(), name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// SetResourceLimits 设置资源限制
// PUT /api/v1/lxcmanager/containers/:name/resources.
func (h *Handler) SetResourceLimits(c *gin.Context) {
	name := c.Param("name")
	var limits ResourceLimits
	if err := c.ShouldBindJSON(&limits); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if err := h.manager.SetResourceLimits(c.Request.Context(), name, limits); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// ConfigureNetwork 配置容器网络
// PUT /api/v1/lxcmanager/containers/:name/network.
func (h *Handler) ConfigureNetwork(c *gin.Context) {
	name := c.Param("name")
	var netCfg NetworkConfig
	if err := c.ShouldBindJSON(&netCfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if err := h.manager.ConfigureNetwork(c.Request.Context(), name, netCfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// ListTemplates 列出可用模板
// GET /api/v1/lxcmanager/templates.
func (h *Handler) ListTemplates(c *gin.Context) {
	templates := h.manager.ListTemplates()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    templates,
	})
}

// RegisterTemplate 注册模板
// POST /api/v1/lxcmanager/templates.
func (h *Handler) RegisterTemplate(c *gin.Context) {
	var tmpl TemplateInfo
	if err := c.ShouldBindJSON(&tmpl); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    1,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}
	if err := h.manager.RegisterTemplate(tmpl); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "ok",
	})
}

// DeleteTemplate 删除模板
// DELETE /api/v1/lxcmanager/templates/:name.
func (h *Handler) DeleteTemplate(c *gin.Context) {
	name := c.Param("name")
	if err := h.manager.DeleteTemplate(name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    1,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
	})
}
