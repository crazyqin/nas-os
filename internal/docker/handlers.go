package docker

import (
	"fmt"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers Docker 处理器
type Handlers struct {
	manager *Manager
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
		docker.GET("/containers/:id/logs", h.getContainerLogs)

		// 镜像管理
		docker.GET("/images", h.listImages)
		docker.POST("/images/pull", h.pullImage)
		docker.DELETE("/images/:id", h.removeImage)

		// 网络管理
		docker.GET("/networks", h.listNetworks)

		// 卷管理
		docker.GET("/volumes", h.listVolumes)
		docker.POST("/volumes", h.createVolume)
		docker.GET("/volumes/:name", h.getVolume)
		docker.DELETE("/volumes/:name", h.removeVolume)

		// 应用商店
		docker.GET("/apps", h.getAppCatalog)
		docker.POST("/apps/:name/install", h.installApp)

		// 系统状态
		docker.GET("/status", h.getStatus)
	}
}

// listContainers 列出容器
// @Summary 列出容器
// @Description 获取 Docker 容器列表
// @Tags docker
// @Accept json
// @Produce json
// @Param all query bool false "是否显示所有容器（包括已停止）"
// @Success 200 {object} api.Response{data=[]Container}
// @Failure 500 {object} api.Response
// @Router /docker/containers [get]
// @Security BearerAuth
func (h *Handlers) listContainers(c *gin.Context) {
	all := c.Query("all") == "true"

	containers, err := h.manager.ListContainers(all)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, containers)
}

// createContainer 创建容器
// @Summary 创建容器
// @Description 创建新的 Docker 容器
// @Tags docker
// @Accept json
// @Produce json
// @Param request body object{name=string,image=string,options=map[string]interface{}} true "容器配置"
// @Success 201 {object} api.Response{data=Container}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /docker/containers [post]
// @Security BearerAuth
func (h *Handlers) createContainer(c *gin.Context) {
	var req struct {
		Name    string                 `json:"name" binding:"required"`
		Image   string                 `json:"image" binding:"required"`
		Options map[string]interface{} `json:"options"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	container, err := h.manager.CreateContainer(req.Name, req.Image, req.Options)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, container)
}

// getContainer 获取容器详情
// @Summary 获取容器详情
// @Description 获取指定容器的详细信息
// @Tags docker
// @Accept json
// @Produce json
// @Param id path string true "容器 ID 或名称"
// @Success 200 {object} api.Response{data=Container}
// @Failure 404 {object} api.Response
// @Router /docker/containers/{id} [get]
// @Security BearerAuth
func (h *Handlers) getContainer(c *gin.Context) {
	id := c.Param("id")

	container, err := h.manager.GetContainer(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, container)
}

// removeContainer 删除容器
// @Summary 删除容器
// @Description 删除指定的 Docker 容器
// @Tags docker
// @Accept json
// @Produce json
// @Param id path string true "容器 ID 或名称"
// @Param force query bool false "强制删除"
// @Success 200 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /docker/containers/{id} [delete]
// @Security BearerAuth
func (h *Handlers) removeContainer(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"

	if err := h.manager.RemoveContainer(id, force); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "容器已删除", nil)
}

// startContainer 启动容器
// @Summary 启动容器
// @Description 启动指定的 Docker 容器
// @Tags docker
// @Accept json
// @Produce json
// @Param id path string true "容器 ID 或名称"
// @Success 200 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /docker/containers/{id}/start [post]
// @Security BearerAuth
func (h *Handlers) startContainer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.StartContainer(id); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "容器已启动", nil)
}

// stopContainer 停止容器
// @Summary 停止容器
// @Description 停止指定的 Docker 容器
// @Tags docker
// @Accept json
// @Produce json
// @Param id path string true "容器 ID 或名称"
// @Success 200 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /docker/containers/{id}/stop [post]
// @Security BearerAuth
func (h *Handlers) stopContainer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.StopContainer(id, 10); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "容器已停止", nil)
}

// restartContainer 重启容器
// @Summary 重启容器
// @Description 重启指定的 Docker 容器
// @Tags docker
// @Accept json
// @Produce json
// @Param id path string true "容器 ID 或名称"
// @Success 200 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /docker/containers/{id}/restart [post]
// @Security BearerAuth
func (h *Handlers) restartContainer(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.RestartContainer(id, 10); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "容器已重启", nil)
}

// getContainerStats 获取容器统计
// @Summary 获取容器资源统计
// @Description 获取指定容器的 CPU、内存等资源使用统计
// @Tags docker
// @Accept json
// @Produce json
// @Param id path string true "容器 ID 或名称"
// @Success 200 {object} api.Response{data=ContainerStats}
// @Failure 500 {object} api.Response
// @Router /docker/containers/{id}/stats [get]
// @Security BearerAuth
func (h *Handlers) getContainerStats(c *gin.Context) {
	id := c.Param("id")

	stats, err := h.manager.GetContainerStats(id)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, stats)
}

// getContainerLogs 获取容器日志
// @Summary 获取容器日志
// @Description 获取指定容器的日志输出
// @Tags docker
// @Accept json
// @Produce json
// @Param id path string true "容器 ID 或名称"
// @Param tail query int false "显示最后 N 行"
// @Param since query string false "显示指定时间之后的日志"
// @Param until query string false "显示指定时间之前的日志"
// @Param timestamps query bool false "是否显示时间戳"
// @Success 200 {object} api.Response{data=map[string]string}
// @Failure 500 {object} api.Response
// @Router /docker/containers/{id}/logs [get]
// @Security BearerAuth
func (h *Handlers) getContainerLogs(c *gin.Context) {
	id := c.Param("id")

	var opts LogOptions
	if tail := c.Query("tail"); tail != "" {
		_, _ = fmt.Sscanf(tail, "%d", &opts.Tail)
	}
	if since := c.Query("since"); since != "" {
		opts.Since = since
	}
	if until := c.Query("until"); until != "" {
		opts.Until = until
	}
	opts.Timestamps = c.Query("timestamps") == "true"

	logs, err := h.manager.GetContainerLogs(id, opts)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, map[string]string{"logs": logs})
}

// listImages 列出镜像
// @Summary 列出镜像
// @Description 获取 Docker 镜像列表
// @Tags docker
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=[]Image}
// @Failure 500 {object} api.Response
// @Router /docker/images [get]
// @Security BearerAuth
func (h *Handlers) listImages(c *gin.Context) {
	images, err := h.manager.ListImages()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, images)
}

// pullImage 拉取镜像
// @Summary 拉取镜像
// @Description 从仓库拉取 Docker 镜像
// @Tags docker
// @Accept json
// @Produce json
// @Param request body object{image=string} true "镜像配置"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /docker/images/pull [post]
// @Security BearerAuth
func (h *Handlers) pullImage(c *gin.Context) {
	var req struct {
		Image string `json:"image" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	if err := h.manager.PullImage(req.Image); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "镜像拉取成功", nil)
}

// removeImage 删除镜像
// @Summary 删除镜像
// @Description 删除指定的 Docker 镜像
// @Tags docker
// @Accept json
// @Produce json
// @Param id path string true "镜像 ID"
// @Param force query bool false "强制删除"
// @Success 200 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /docker/images/{id} [delete]
// @Security BearerAuth
func (h *Handlers) removeImage(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"

	if err := h.manager.RemoveImage(id, force); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "镜像已删除", nil)
}

// listNetworks 列出网络
// @Summary 列出网络
// @Description 获取 Docker 网络列表
// @Tags docker
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=[]Network}
// @Failure 500 {object} api.Response
// @Router /docker/networks [get]
// @Security BearerAuth
func (h *Handlers) listNetworks(c *gin.Context) {
	networks, err := h.manager.ListNetworks()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, networks)
}

// getAppCatalog 获取应用目录
// @Summary 获取应用目录
// @Description 获取可安装的应用列表
// @Tags docker
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=[]AppCatalog}
// @Router /docker/apps [get]
// @Security BearerAuth
func (h *Handlers) getAppCatalog(c *gin.Context) {
	apps := h.manager.GetAppCatalog()
	api.OK(c, apps)
}

// installApp 安装应用
// @Summary 安装应用
// @Description 从应用目录安装指定应用
// @Tags docker
// @Accept json
// @Produce json
// @Param name path string true "应用名称"
// @Success 200 {object} api.Response{data=Container}
// @Failure 404 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /docker/apps/{name}/install [post]
// @Security BearerAuth
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
		api.NotFound(c, "应用不存在")
		return
	}

	// 构建选项
	opts := make(map[string]interface{})

	// 端口映射
	for _, port := range app.Ports {
		ports, _ := opts["ports"].([]string)
		opts["ports"] = append(ports, fmt.Sprintf("%d:%d", port, port))
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
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "应用安装成功", container)
}

// getStatus 获取 Docker 状态
// @Summary 获取 Docker 状态
// @Description 获取 Docker 服务运行状态
// @Tags docker
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=map[string]bool}
// @Router /docker/status [get]
// @Security BearerAuth
func (h *Handlers) getStatus(c *gin.Context) {
	running := h.manager.IsRunning()

	api.OK(c, map[string]bool{
		"running": running,
	})
}

// listVolumes 列出卷
// @Summary 列出卷
// @Description 获取 Docker 卷列表
// @Tags docker
// @Accept json
// @Produce json
// @Success 200 {object} api.Response{data=[]Volume}
// @Failure 500 {object} api.Response
// @Router /docker/volumes [get]
// @Security BearerAuth
func (h *Handlers) listVolumes(c *gin.Context) {
	volumes, err := h.manager.ListVolumes()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, volumes)
}

// createVolume 创建卷
// @Summary 创建卷
// @Description 创建新的 Docker 卷
// @Tags docker
// @Accept json
// @Produce json
// @Param request body object{name=string,driver=string,opts=map[string]string} true "卷配置"
// @Success 201 {object} api.Response{data=Volume}
// @Failure 400 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /docker/volumes [post]
// @Security BearerAuth
func (h *Handlers) createVolume(c *gin.Context) {
	var req struct {
		Name   string            `json:"name"`
		Driver string            `json:"driver"`
		Opts   map[string]string `json:"opts"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	volume, err := h.manager.CreateVolume(req.Name, req.Driver, req.Opts)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, volume)
}

// getVolume 获取卷详情
// @Summary 获取卷详情
// @Description 获取指定 Docker 卷的详细信息
// @Tags docker
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} api.Response{data=Volume}
// @Failure 404 {object} api.Response
// @Router /docker/volumes/{name} [get]
// @Security BearerAuth
func (h *Handlers) getVolume(c *gin.Context) {
	name := c.Param("name")

	volume, err := h.manager.GetVolume(name)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, volume)
}

// removeVolume 删除卷
// @Summary 删除卷
// @Description 删除指定的 Docker 卷
// @Tags docker
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Param force query bool false "强制删除"
// @Success 200 {object} api.Response
// @Failure 500 {object} api.Response
// @Router /docker/volumes/{name} [delete]
// @Security BearerAuth
func (h *Handlers) removeVolume(c *gin.Context) {
	name := c.Param("name")
	force := c.Query("force") == "true"

	if err := h.manager.RemoveVolume(name, force); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "卷已删除", nil)
}
