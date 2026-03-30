package api

import (
	"net/http"
	"strconv"

	"nas-os/internal/container"

	"github.com/gin-gonic/gin"
)

// ContainerHandlers 容器 API 处理器。
type ContainerHandlers struct {
	manager        *container.Manager
	imageManager   *container.ImageManager
	networkManager *container.NetworkManager
	volumeManager  *container.VolumeManager
	composeManager *container.ComposeManager
}

// NewContainerHandlers 创建容器处理器。
func NewContainerHandlers() (*ContainerHandlers, error) {
	mgr, err := container.NewManager()
	if err != nil {
		return nil, err
	}

	return &ContainerHandlers{
		manager:        mgr,
		imageManager:   container.NewImageManager(mgr),
		networkManager: container.NewNetworkManager(mgr),
		volumeManager:  container.NewVolumeManager(mgr),
		composeManager: container.NewComposeManager(mgr),
	}, nil
}

// RegisterRoutes 注册路由.
func (h *ContainerHandlers) RegisterRoutes(r *gin.RouterGroup) {
	api := r.Group("/api/v1")
	{
		// Docker 状态
		api.GET("/docker/status", h.getDockerStatus)

		// 容器管理
		api.GET("/containers", h.listContainers)
		api.POST("/containers", h.createContainer)
		api.GET("/containers/:id", h.getContainer)
		api.DELETE("/containers/:id", h.removeContainer)
		api.POST("/containers/:id/start", h.startContainer)
		api.POST("/containers/:id/stop", h.stopContainer)
		api.POST("/containers/:id/restart", h.restartContainer)
		api.GET("/containers/:id/stats", h.getContainerStats)
		api.GET("/containers/:id/logs", h.getContainerLogs)

		// 容器批量操作
		api.POST("/containers/batch/start", h.batchStartContainers)
		api.POST("/containers/batch/stop", h.batchStopContainers)
		api.POST("/containers/batch/restart", h.batchRestartContainers)
		api.POST("/containers/batch/remove", h.batchRemoveContainers)
		api.POST("/containers/batch/execute", h.batchExecuteContainers)
		api.POST("/containers/prune", h.pruneContainers)
		api.POST("/containers/select", h.selectContainers)

		// 镜像管理
		api.GET("/images", h.listImages)
		api.POST("/images/pull", h.pullImage)
		api.POST("/images/push", h.pushImage)
		api.DELETE("/images/:id", h.removeImage)
		api.POST("/images/tag", h.tagImage)
		api.GET("/images/search", h.searchImages)
		api.POST("/images/prune", h.pruneImages)

		// 网络管理
		api.GET("/networks", h.listNetworks)
		api.POST("/networks", h.createNetwork)
		api.GET("/networks/:id", h.getNetwork)
		api.DELETE("/networks/:id", h.removeNetwork)
		api.POST("/networks/:id/connect", h.connectNetwork)
		api.POST("/networks/:id/disconnect", h.disconnectNetwork)
		api.POST("/networks/prune", h.pruneNetworks)
		api.GET("/networks/types", h.getNetworkTypes)

		// 存储卷管理
		api.GET("/volumes", h.listVolumes)
		api.POST("/volumes", h.createVolume)
		api.GET("/volumes/:name", h.getVolume)
		api.DELETE("/volumes/:name", h.removeVolume)
		api.POST("/volumes/:name/backup", h.backupVolume)
		api.POST("/volumes/restore", h.restoreVolume)
		api.POST("/volumes/prune", h.pruneVolumes)
		api.GET("/volumes/backups", h.listVolumeBackups)

		// Compose 管理
		api.POST("/compose/deploy", h.deployCompose)
		api.POST("/compose/stop", h.stopCompose)
		api.POST("/compose/restart", h.restartCompose)
		api.DELETE("/compose/remove", h.removeCompose)
		api.GET("/compose/services", h.getComposeServices)
		api.GET("/compose/logs", h.getComposeLogs)
		api.POST("/compose/validate", h.validateCompose)
	}
}

// getDockerStatus 获取 Docker 状态.
func (h *ContainerHandlers) getDockerStatus(c *gin.Context) {
	running := h.manager.IsRunning()

	response := gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"running": running,
		},
	}

	// 获取详细版本信息
	if running {
		version, err := h.manager.GetVersion()
		if err == nil {
			if data, ok := response["data"].(gin.H); ok {
				data["version"] = version
			}
		}
	}

	c.JSON(http.StatusOK, response)
}

// listContainers 列出容器.
func (h *ContainerHandlers) listContainers(c *gin.Context) {
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

// createContainer 创建容器.
func (h *ContainerHandlers) createContainer(c *gin.Context) {
	var config container.Config

	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	container, err := h.manager.CreateContainer(&config)
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

// getContainer 获取容器详情.
func (h *ContainerHandlers) getContainer(c *gin.Context) {
	id := c.Param("id")

	cont, err := h.manager.GetContainer(id)
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
		"data":    cont,
	})
}

// removeContainer 删除容器.
func (h *ContainerHandlers) removeContainer(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"
	removeVolumes := c.Query("volumes") == "true"

	if err := h.manager.RemoveContainer(id, force, removeVolumes); err != nil {
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

// startContainer 启动容器.
func (h *ContainerHandlers) startContainer(c *gin.Context) {
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

// stopContainer 停止容器.
func (h *ContainerHandlers) stopContainer(c *gin.Context) {
	id := c.Param("id")
	timeout := container.DefaultStopTimeout

	// 支持自定义超时时间
	if t := c.Query("timeout"); t != "" {
		if customTimeout, err := strconv.Atoi(t); err == nil && customTimeout > 0 {
			timeout = customTimeout
		}
	}

	if err := h.manager.StopContainer(id, timeout); err != nil {
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

// restartContainer 重启容器.
func (h *ContainerHandlers) restartContainer(c *gin.Context) {
	id := c.Param("id")
	timeout := container.DefaultStopTimeout

	// 支持自定义超时时间
	if t := c.Query("timeout"); t != "" {
		if customTimeout, err := strconv.Atoi(t); err == nil && customTimeout > 0 {
			timeout = customTimeout
		}
	}

	if err := h.manager.RestartContainer(id, timeout); err != nil {
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

// getContainerStats 获取容器实时统计.
func (h *ContainerHandlers) getContainerStats(c *gin.Context) {
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

// getContainerLogs 获取容器日志.
func (h *ContainerHandlers) getContainerLogs(c *gin.Context) {
	id := c.Param("id")
	tail := container.DefaultLogTail
	follow := c.Query("follow") == "true"

	// 支持自定义日志行数
	if t := c.Query("tail"); t != "" {
		if customTail, err := strconv.Atoi(t); err == nil && customTail > 0 {
			tail = customTail
		}
	}

	logs, err := h.manager.GetContainerLogs(id, tail, follow)
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
		"data":    logs,
	})
}

// listImages 列出镜像.
func (h *ContainerHandlers) listImages(c *gin.Context) {
	images, err := h.imageManager.ListImages()
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

// pullImage 拉取镜像.
func (h *ContainerHandlers) pullImage(c *gin.Context) {
	var config container.ImageConfig

	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.imageManager.PullImage(&config); err != nil {
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

// pushImage 推送镜像.
func (h *ContainerHandlers) pushImage(c *gin.Context) {
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

	if err := h.imageManager.PushImage(req.Image); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "镜像推送成功",
	})
}

// removeImage 删除镜像.
func (h *ContainerHandlers) removeImage(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"
	prune := c.Query("prune") == "true"

	if err := h.imageManager.RemoveImage(id, force, prune); err != nil {
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

// tagImage 标记镜像.
func (h *ContainerHandlers) tagImage(c *gin.Context) {
	var req struct {
		Source string `json:"source" binding:"required"`
		Target string `json:"target" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.imageManager.TagImage(req.Source, req.Target); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "镜像标记成功",
	})
}

// searchImages 搜索镜像.
func (h *ContainerHandlers) searchImages(c *gin.Context) {
	term := c.Query("term")
	limit := 10

	images, err := h.imageManager.SearchImages(term, limit)
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

// pruneImages 清理镜像.
func (h *ContainerHandlers) pruneImages(c *gin.Context) {
	var req struct {
		All bool `json:"all"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	reclaimed, err := h.imageManager.PruneImages(req.All)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "镜像清理完成",
		"data": gin.H{
			"reclaimed": reclaimed,
		},
	})
}

// listNetworks 列出网络.
func (h *ContainerHandlers) listNetworks(c *gin.Context) {
	networks, err := h.networkManager.ListNetworks()
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

// createNetwork 创建网络.
func (h *ContainerHandlers) createNetwork(c *gin.Context) {
	var config container.NetworkConfig

	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	network, err := h.networkManager.CreateNetwork(&config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "网络创建成功",
		"data":    network,
	})
}

// getNetwork 获取网络详情.
func (h *ContainerHandlers) getNetwork(c *gin.Context) {
	id := c.Param("id")

	network, err := h.networkManager.GetNetwork(id)
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
		"data":    network,
	})
}

// removeNetwork 删除网络.
func (h *ContainerHandlers) removeNetwork(c *gin.Context) {
	id := c.Param("id")

	if err := h.networkManager.RemoveNetwork(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "网络已删除",
	})
}

// connectNetwork 连接网络.
func (h *ContainerHandlers) connectNetwork(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		ContainerID string   `json:"containerId" binding:"required"`
		Aliases     []string `json:"aliases"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.networkManager.ConnectNetwork(id, req.ContainerID, req.Aliases); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "网络连接成功",
	})
}

// disconnectNetwork 断开网络.
func (h *ContainerHandlers) disconnectNetwork(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		ContainerID string `json:"containerId" binding:"required"`
		Force       bool   `json:"force"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.networkManager.DisconnectNetwork(id, req.ContainerID, req.Force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "网络已断开",
	})
}

// pruneNetworks 清理网络.
func (h *ContainerHandlers) pruneNetworks(c *gin.Context) {
	reclaimed, err := h.networkManager.PruneNetworks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "网络清理完成",
		"data": gin.H{
			"reclaimed": reclaimed,
		},
	})
}

// getNetworkTypes 获取网络类型.
func (h *ContainerHandlers) getNetworkTypes(c *gin.Context) {
	types := h.networkManager.GetNetworkTypes()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    types,
	})
}

// listVolumes 列出卷.
func (h *ContainerHandlers) listVolumes(c *gin.Context) {
	volumes, err := h.volumeManager.ListVolumes()
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
		"data":    volumes,
	})
}

// createVolume 创建卷.
func (h *ContainerHandlers) createVolume(c *gin.Context) {
	var config container.VolumeConfig

	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	volume, err := h.volumeManager.CreateVolume(&config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "卷创建成功",
		"data":    volume,
	})
}

// getVolume 获取卷详情.
func (h *ContainerHandlers) getVolume(c *gin.Context) {
	name := c.Param("name")

	volume, err := h.volumeManager.GetVolume(name)
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
		"data":    volume,
	})
}

// removeVolume 删除卷.
func (h *ContainerHandlers) removeVolume(c *gin.Context) {
	name := c.Param("name")
	force := c.Query("force") == "true"

	if err := h.volumeManager.RemoveVolume(name, force); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "卷已删除",
	})
}

// backupVolume 备份卷.
func (h *ContainerHandlers) backupVolume(c *gin.Context) {
	name := c.Param("name")
	var req struct {
		BackupPath string `json:"backupPath" binding:"required"`
		Compress   bool   `json:"compress"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	backup, err := h.volumeManager.BackupVolume(name, req.BackupPath, req.Compress)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "卷备份成功",
		"data":    backup,
	})
}

// restoreVolume 恢复卷.
func (h *ContainerHandlers) restoreVolume(c *gin.Context) {
	var req struct {
		BackupPath string `json:"backupPath" binding:"required"`
		VolumeName string `json:"volumeName" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.volumeManager.RestoreVolume(req.BackupPath, req.VolumeName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "卷恢复成功",
	})
}

// pruneVolumes 清理卷.
func (h *ContainerHandlers) pruneVolumes(c *gin.Context) {
	reclaimed, err := h.volumeManager.PruneVolumes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "卷清理完成",
		"data": gin.H{
			"reclaimed": reclaimed,
		},
	})
}

// listVolumeBackups 列出卷备份.
func (h *ContainerHandlers) listVolumeBackups(c *gin.Context) {
	backupDir := c.Query("dir")
	if backupDir == "" {
		backupDir = "/opt/nas/backups/volumes"
	}

	backups, err := h.volumeManager.ListBackups(backupDir)
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
		"data":    backups,
	})
}

// deployCompose 部署 Compose 项目.
func (h *ContainerHandlers) deployCompose(c *gin.Context) {
	var req struct {
		ComposePath string `json:"composePath" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.composeManager.Deploy(c.Request.Context(), req.ComposePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Compose 项目部署成功",
	})
}

// stopCompose 停止 Compose 项目.
func (h *ContainerHandlers) stopCompose(c *gin.Context) {
	var req struct {
		ComposePath string `json:"composePath" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.composeManager.Stop(c.Request.Context(), req.ComposePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Compose 项目已停止",
	})
}

// restartCompose 重启 Compose 项目.
func (h *ContainerHandlers) restartCompose(c *gin.Context) {
	var req struct {
		ComposePath string `json:"composePath" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.composeManager.Restart(c.Request.Context(), req.ComposePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Compose 项目已重启",
	})
}

// removeCompose 删除 Compose 项目.
func (h *ContainerHandlers) removeCompose(c *gin.Context) {
	var req struct {
		ComposePath   string `json:"composePath" binding:"required"`
		RemoveVolumes bool   `json:"removeVolumes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.composeManager.Remove(c.Request.Context(), req.ComposePath, req.RemoveVolumes); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Compose 项目已删除",
	})
}

// getComposeServices 获取 Compose 服务状态.
func (h *ContainerHandlers) getComposeServices(c *gin.Context) {
	composePath := c.Query("composePath")
	if composePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少 composePath 参数",
		})
		return
	}

	services, err := h.composeManager.GetServices(c.Request.Context(), composePath)
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
		"data":    services,
	})
}

// getComposeLogs 获取 Compose 日志.
func (h *ContainerHandlers) getComposeLogs(c *gin.Context) {
	composePath := c.Query("composePath")
	service := c.Query("service")
	tail := 100

	if composePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "缺少 composePath 参数",
		})
		return
	}

	logs, err := h.composeManager.GetLogs(c.Request.Context(), composePath, service, tail)
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
		"data":    logs,
	})
}

// validateCompose 验证 Compose 文件.
func (h *ContainerHandlers) validateCompose(c *gin.Context) {
	var req struct {
		ComposePath string `json:"composePath" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	if err := h.composeManager.ValidateComposeFile(c.Request.Context(), req.ComposePath); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "Compose 文件验证通过",
	})
}

// === 批量操作 API ===

// batchStartContainers 批量启动容器.
func (h *ContainerHandlers) batchStartContainers(c *gin.Context) {
	var req struct {
		ContainerIDs []string `json:"containerIds" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	batchManager := container.NewBatchManager(h.manager)
	response, err := batchManager.StartBatch(c.Request.Context(), req.ContainerIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量启动完成",
		"data":    response,
	})
}

// batchStopContainers 批量停止容器.
func (h *ContainerHandlers) batchStopContainers(c *gin.Context) {
	var req struct {
		ContainerIDs []string `json:"containerIds" binding:"required"`
		Timeout      int      `json:"timeout"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	batchManager := container.NewBatchManager(h.manager)
	response, err := batchManager.StopBatch(c.Request.Context(), req.ContainerIDs, req.Timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量停止完成",
		"data":    response,
	})
}

// batchRestartContainers 批量重启容器.
func (h *ContainerHandlers) batchRestartContainers(c *gin.Context) {
	var req struct {
		ContainerIDs []string `json:"containerIds" binding:"required"`
		Timeout      int      `json:"timeout"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	batchManager := container.NewBatchManager(h.manager)
	response, err := batchManager.RestartBatch(c.Request.Context(), req.ContainerIDs, req.Timeout)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量重启完成",
		"data":    response,
	})
}

// batchRemoveContainers 批量删除容器.
func (h *ContainerHandlers) batchRemoveContainers(c *gin.Context) {
	var req struct {
		ContainerIDs  []string `json:"containerIds" binding:"required"`
		Force         bool     `json:"force"`
		RemoveVolumes bool     `json:"removeVolumes"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	batchManager := container.NewBatchManager(h.manager)
	response, err := batchManager.RemoveBatch(c.Request.Context(), req.ContainerIDs, req.Force, req.RemoveVolumes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量删除完成",
		"data":    response,
	})
}

// batchExecuteContainers 执行通用批量操作.
func (h *ContainerHandlers) batchExecuteContainers(c *gin.Context) {
	var req container.BatchOperationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	batchManager := container.NewBatchManager(h.manager)
	response, err := batchManager.Execute(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量操作完成",
		"data":    response,
	})
}

// pruneContainers 清理停止的容器.
func (h *ContainerHandlers) pruneContainers(c *gin.Context) {
	result, err := h.manager.PruneContainers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "容器清理完成",
		"data":    result,
	})
}

// selectContainers 根据条件选择容器.
func (h *ContainerHandlers) selectContainers(c *gin.Context) {
	var filter container.ContainerFilter

	if err := c.ShouldBindJSON(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	batchManager := container.NewBatchManager(h.manager)
	ids, err := batchManager.SelectByFilter(filter)
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
		"data": map[string]interface{}{
			"containerIds": ids,
			"count":        len(ids),
		},
	})
}
