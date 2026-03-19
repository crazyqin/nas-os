// Package service 提供系统服务管理功能
package service

import (
	"strconv"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 服务管理 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	services := r.Group("/services")
	{
		// 服务列表和操作
		services.GET("", h.listServices)
		services.GET("/summary", h.getServiceSummary)
		services.GET("/failed", h.getFailedServices)

		// 单个服务操作
		services.GET("/:name", h.getService)
		services.POST("/:name/start", h.startService)
		services.POST("/:name/stop", h.stopService)
		services.POST("/:name/restart", h.restartService)
		services.POST("/:name/enable", h.enableService)
		services.POST("/:name/disable", h.disableService)
		services.GET("/:name/status", h.getServiceStatus)
		services.GET("/:name/logs", h.getServiceLogs)

		// 高级操作
		services.POST("/:name/mask", h.maskService)
		services.POST("/:name/unmask", h.unmaskService)
		services.POST("/:name/reset-failed", h.resetFailedService)
		services.GET("/:name/dependencies", h.getServiceDependencies)

		// 系统级操作
		services.POST("/daemon-reload", h.daemonReload)
		services.POST("/refresh", h.refreshAllServices)

		// 服务注册
		services.POST("/register", h.registerService)
		services.DELETE("/:name", h.unregisterService)
	}
}

// ServiceRequest 服务操作请求
type ServiceRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Type        string `json:"type"`
	UnitFile    string `json:"unitFile"`
}

// ServiceSummary 服务统计摘要
type ServiceSummary struct {
	Total    int `json:"total"`
	Running  int `json:"running"`
	Stopped  int `json:"stopped"`
	Enabled  int `json:"enabled"`
	Disabled int `json:"disabled"`
	Failed   int `json:"failed"`
}

// listServices 列出所有服务
func (h *Handlers) listServices(c *gin.Context) {
	// 先刷新状态
	_ = h.manager.RefreshAll()

	services, err := h.manager.List()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, services)
}

// getServiceSummary 获取服务统计摘要
func (h *Handlers) getServiceSummary(c *gin.Context) {
	services, err := h.manager.List()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	summary := ServiceSummary{
		Total: len(services),
	}

	for _, svc := range services {
		if svc.Status.Running {
			summary.Running++
		} else {
			summary.Stopped++
		}
		if svc.Enabled {
			summary.Enabled++
		} else {
			summary.Disabled++
		}
		if svc.Status.LastError != "" && !svc.Status.Running {
			summary.Failed++
		}
	}

	api.OK(c, summary)
}

// getFailedServices 获取失败的服务列表
func (h *Handlers) getFailedServices(c *gin.Context) {
	backend, ok := h.manager.backend.(*SystemdBackend)
	if !ok {
		api.InternalError(c, "当前后端不支持此操作")
		return
	}

	failed, err := backend.GetFailedServices()
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, failed)
}

// getService 获取单个服务详情
func (h *Handlers) getService(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	service, err := h.manager.Get(name)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, service)
}

// startService 启动服务
func (h *Handlers) startService(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	if err := h.manager.Start(name); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "服务已启动", gin.H{"name": name})
}

// stopService 停止服务
func (h *Handlers) stopService(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	if err := h.manager.Stop(name); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "服务已停止", gin.H{"name": name})
}

// restartService 重启服务
func (h *Handlers) restartService(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	if err := h.manager.Restart(name); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "服务已重启", gin.H{"name": name})
}

// enableService 启用服务开机自启
func (h *Handlers) enableService(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	if err := h.manager.Enable(name); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "服务已设置为开机自启", gin.H{"name": name})
}

// disableService 禁用服务开机自启
func (h *Handlers) disableService(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	if err := h.manager.Disable(name); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "服务已取消开机自启", gin.H{"name": name})
}

// getServiceStatus 获取服务状态
func (h *Handlers) getServiceStatus(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	status, err := h.manager.Status(name)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OK(c, status)
}

// getServiceLogs 获取服务日志
func (h *Handlers) getServiceLogs(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	// 解析参数
	lines := 100
	if l := c.Query("lines"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			lines = n
		}
	}

	follow := c.Query("follow") == "true"

	// 获取 systemd 后端
	backend, ok := h.manager.backend.(*SystemdBackend)
	if !ok {
		api.InternalError(c, "当前后端不支持此操作")
		return
	}

	logs, err := backend.GetServiceLogs(name, lines, follow)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"name":  name,
		"lines": lines,
		"logs":  logs,
	})
}

// maskService 屏蔽服务
func (h *Handlers) maskService(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	backend, ok := h.manager.backend.(*SystemdBackend)
	if !ok {
		api.InternalError(c, "当前后端不支持此操作")
		return
	}

	if err := backend.Mask(name); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "服务已屏蔽", gin.H{"name": name})
}

// unmaskService 取消屏蔽服务
func (h *Handlers) unmaskService(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	backend, ok := h.manager.backend.(*SystemdBackend)
	if !ok {
		api.InternalError(c, "当前后端不支持此操作")
		return
	}

	if err := backend.Unmask(name); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "服务已取消屏蔽", gin.H{"name": name})
}

// resetFailedService 重置服务失败状态
func (h *Handlers) resetFailedService(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	backend, ok := h.manager.backend.(*SystemdBackend)
	if !ok {
		api.InternalError(c, "当前后端不支持此操作")
		return
	}

	if err := backend.ResetFailed(name); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "服务失败状态已重置", gin.H{"name": name})
}

// getServiceDependencies 获取服务依赖
func (h *Handlers) getServiceDependencies(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	backend, ok := h.manager.backend.(*SystemdBackend)
	if !ok {
		api.InternalError(c, "当前后端不支持此操作")
		return
	}

	deps, err := backend.GetServiceDependencies(name)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, gin.H{
		"name":         name,
		"dependencies": deps,
	})
}

// daemonReload 重载 systemd 配置
func (h *Handlers) daemonReload(c *gin.Context) {
	backend, ok := h.manager.backend.(*SystemdBackend)
	if !ok {
		api.InternalError(c, "当前后端不支持此操作")
		return
	}

	if err := backend.DaemonReload(); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "systemd 配置已重载", nil)
}

// refreshAllServices 刷新所有服务状态
func (h *Handlers) refreshAllServices(c *gin.Context) {
	if err := h.manager.RefreshAll(); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "服务状态已刷新", nil)
}

// registerService 注册新服务
func (h *Handlers) registerService(c *gin.Context) {
	var req ServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	svc := &Service{
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		UnitFile:    req.UnitFile,
	}

	if svc.Type == "" {
		svc.Type = "systemd"
	}
	if svc.UnitFile == "" {
		svc.UnitFile = svc.Name + ".service"
	}

	if err := h.manager.Register(svc); err != nil {
		api.Conflict(c, err.Error())
		return
	}

	api.Created(c, svc)
}

// unregisterService 注销服务
func (h *Handlers) unregisterService(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		api.BadRequest(c, "服务名称不能为空")
		return
	}

	if err := h.manager.Unregister(name); err != nil {
		api.NotFound(c, err.Error())
		return
	}

	api.OKWithMessage(c, "服务已注销", gin.H{"name": name})
}

// BatchOperationRequest 批量操作请求结构
type BatchOperationRequest struct {
	Names []string `json:"names" binding:"required"`
}

// BatchOperationResponse 批量操作响应结构
type BatchOperationResponse struct {
	Success []string          `json:"success"`
	Failed  map[string]string `json:"failed,omitempty"`
}

// BatchStart 批量启动服务
func (h *Handlers) BatchStart(names []string) *BatchOperationResponse {
	result := &BatchOperationResponse{
		Success: make([]string, 0),
		Failed:  make(map[string]string),
	}

	for _, name := range names {
		if err := h.manager.Start(name); err != nil {
			result.Failed[name] = err.Error()
		} else {
			result.Success = append(result.Success, name)
		}
	}

	return result
}

// BatchStop 批量停止服务
func (h *Handlers) BatchStop(names []string) *BatchOperationResponse {
	result := &BatchOperationResponse{
		Success: make([]string, 0),
		Failed:  make(map[string]string),
	}

	for _, name := range names {
		if err := h.manager.Stop(name); err != nil {
			result.Failed[name] = err.Error()
		} else {
			result.Success = append(result.Success, name)
		}
	}

	return result
}

// BatchRestart 批量重启服务
func (h *Handlers) BatchRestart(names []string) *BatchOperationResponse {
	result := &BatchOperationResponse{
		Success: make([]string, 0),
		Failed:  make(map[string]string),
	}

	for _, name := range names {
		if err := h.manager.Restart(name); err != nil {
			result.Failed[name] = err.Error()
		} else {
			result.Success = append(result.Success, name)
		}
	}

	return result
}
