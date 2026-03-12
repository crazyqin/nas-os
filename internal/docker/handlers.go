package docker

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers Docker 处理器
type Handlers struct {
	manager *Manager
	// mu      sync.RWMutex - 保留用于未来需要并发控制的场景
}

// NewHandlers 创建 Docker 处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{
		manager: mgr,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	docker := r.Group("/docker")
	{
		// 容器管理
		docker.GET("/containers", h.listContainers)
		docker.POST("/containers", h.createContainer)
		docker.GET("/containers/:id", h.getContainer)
		docker.DELETE("/containers/:id", h.removeContainer)
		docker.POST("/containers/:id/start", h.startContainer)
		docker.POST("/containers/:id/stop", h.stopContainer)
		docker.POST("/containers/:id/restart", h.restartContainer)
		docker.GET("/containers/:id/stats", h.getContainerStats)

		// 镜像管理
		docker.GET("/images", h.listImages)
		docker.POST("/images/pull", h.pullImage)
		docker.DELETE("/images/:id", h.removeImage)

		// 网络管理
		docker.GET("/networks", h.listNetworks)

		// 应用商店
		docker.GET("/apps", h.getAppCatalog)
		docker.POST("/apps/:name/install", h.installApp)

		// 系统状态
		docker.GET("/status", h.getStatus)
	}
}

// listContainers 列出容器
func (h *Handlers) listContainers(c *gin.Context) {
	all := c.Query("all") == "true"

	containers, err := h.manager.ListContainers(all)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    containers,
	})
}

// createContainer 创建容器
func (h *Handlers) createContainer(c *gin.Context) {
	var req struct {
		Name    string                 `json:"name" binding:"required"`
		Image   string                 `json:"image" binding:"required"`
		Options map[string]interface{} `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	container, err := h.manager.CreateContainer(req.Name, req.Image, req.Options)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "容器创建成功",
		"data":    container,
	})
}

// getContainer 获取容器详情
func (h *Handlers) getContainer(c *gin.Context) {
	id := c.Param("id")

	container, err := h.manager.GetContainer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    container,
	})
}

// removeContainer 删除容器
func (h *Handlers) removeContainer(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"

	if err := h.manager.RemoveContainer(id, force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "容器已删除",
	})
}

// startContainer 启动容器
func (h *Handlers) startContainer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.StartContainer(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "容器已启动",
	})
}

// stopContainer 停止容器
func (h *Handlers) stopContainer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.StopContainer(id, 10); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "容器已停止",
	})
}

// restartContainer 重启容器
func (h *Handlers) restartContainer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.RestartContainer(id, 10); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "容器已重启",
	})
}

// getContainerStats 获取容器统计
func (h *Handlers) getContainerStats(c *gin.Context) {
	id := c.Param("id")

	stats, err := h.manager.GetContainerStats(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// listImages 列出镜像
func (h *Handlers) listImages(c *gin.Context) {
	images, err := h.manager.ListImages()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    images,
	})
}

// pullImage 拉取镜像
func (h *Handlers) pullImage(c *gin.Context) {
	var req struct {
		Image string `json:"image" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.manager.PullImage(req.Image); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "镜像拉取成功",
	})
}

// removeImage 删除镜像
func (h *Handlers) removeImage(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"

	if err := h.manager.RemoveImage(id, force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "镜像已删除",
	})
}

// listNetworks 列出网络
func (h *Handlers) listNetworks(c *gin.Context) {
	networks, err := h.manager.ListNetworks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    networks,
	})
}

// getAppCatalog 获取应用目录
func (h *Handlers) getAppCatalog(c *gin.Context) {
	apps := h.manager.GetAppCatalog()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    apps,
	})
}

// installApp 安装应用
func (h *Handlers) installApp(c *gin.Context) {
	name := c.Param("name")

	// 从目录查找应用
	var app *AppCatalog
	for _, a := range h.manager.GetAppCatalog() {
		if a.Name == name {
			app = a
			break
		}
	}

	if app == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "应用不存在",
		})
		return
	}

	// 构建选项
	opts := make(map[string]interface{})

	// 端口映射
	for _, port := range app.Ports {
		opts["ports"] = append(opts["ports"].([]string), fmt.Sprintf("%d:%d", port, port))
	}

	// 卷挂载
	if len(app.Volumes) > 0 {
		vols := make([]string, 0)
		for _, vol := range app.Volumes {
			vols = append(vols, fmt.Sprintf("/opt/nas/apps/%s%s:%s", name, vol, vol))
		}
		opts["volumes"] = vols
	}

	// 环境变量
	opts["env"] = app.Environment

	// 创建容器
	container, err := h.manager.CreateContainer(name, app.Image, opts)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "应用安装成功",
		"data":    container,
	})
}

// getStatus 获取 Docker 状态
func (h *Handlers) getStatus(c *gin.Context) {
	running := h.manager.IsRunning()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"running": running,
		},
	})
}