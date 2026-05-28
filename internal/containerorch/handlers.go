// Package containerorch 提供 REST API 处理器
package containerorch

import (
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 容器编排模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/containerorch 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	co := r.Group("/containerorch")
	{
		// 项目 CRUD
		co.POST("/projects", h.createProject)
		co.GET("/projects", h.listProjects)
		co.GET("/projects/:id", h.getProject)
		co.PUT("/projects/:id", h.updateProject)
		co.DELETE("/projects/:id", h.deleteProject)

		// 项目生命周期
		co.POST("/projects/:id/start", h.startProject)
		co.POST("/projects/:id/stop", h.stopProject)
		co.POST("/projects/:id/restart", h.restartProject)

		// 服务管理
		co.GET("/projects/:id/services", h.listServices)
		co.GET("/projects/:id/services/:name", h.getService)
		co.POST("/projects/:id/services/:name/scale", h.scaleService)

		// 依赖管理
		co.GET("/projects/:id/startup-order", h.getStartupOrder)

		// 健康检查
		co.GET("/projects/:id/health", h.getHealthReport)
		co.PUT("/projects/:id/services/:name/health-check", h.updateHealthCheck)
		co.PUT("/projects/:id/services/:name/resources", h.updateResources)

		// 自动扩缩容
		co.PUT("/projects/:id/services/:name/auto-scale", h.updateAutoScale)
		co.POST("/projects/:id/services/:name/evaluate-auto-scale", h.evaluateAutoScale)
		co.GET("/projects/:id/auto-scale-events", h.getAutoScaleEvents)

		// 日志聚合
		co.GET("/projects/:id/logs", h.getServiceLogs)
		co.GET("/projects/:id/logs/stream", h.streamServiceLogs)

		// 统计
		co.GET("/projects/:id/stats", h.getProjectStats)
	}
}

// ========== 项目 Handlers ==========

func (h *Handlers) createProject(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	project, err := h.manager.CreateProject(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: project})
}

func (h *Handlers) getProject(c *gin.Context) {
	id := c.Param("id")
	project, err := h.manager.GetProject(id)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: project})
}

func (h *Handlers) listProjects(c *gin.Context) {
	namespace := c.Query("namespace")
	projects := h.manager.ListProjects(namespace)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(projects),
			"projects": projects,
		},
	})
}

func (h *Handlers) updateProject(c *gin.Context) {
	id := c.Param("id")
	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	project, err := h.manager.UpdateProject(id, req)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: project})
}

func (h *Handlers) deleteProject(c *gin.Context) {
	id := c.Param("id")
	force := c.Query("force") == "true"

	if err := h.manager.DeleteProject(id, force); err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 生命周期 Handlers ==========

func (h *Handlers) startProject(c *gin.Context) {
	id := c.Param("id")
	project, err := h.manager.StartProject(id)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "started", Data: project})
}

func (h *Handlers) stopProject(c *gin.Context) {
	id := c.Param("id")
	project, err := h.manager.StopProject(id)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "stopped", Data: project})
}

func (h *Handlers) restartProject(c *gin.Context) {
	id := c.Param("id")
	project, err := h.manager.RestartProject(id)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "restarted", Data: project})
}

// ========== 服务 Handlers ==========

func (h *Handlers) listServices(c *gin.Context) {
	id := c.Param("id")
	project, err := h.manager.GetProject(id)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	services := make([]*ServiceConfig, 0, len(project.Services))
	for _, svc := range project.Services {
		services = append(services, svc)
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(services),
			"services": services,
		},
	})
}

func (h *Handlers) getService(c *gin.Context) {
	projectID := c.Param("id")
	serviceName := c.Param("name")

	project, err := h.manager.GetProject(projectID)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	svc, ok := project.Services[serviceName]
	if !ok {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: fmt.Sprintf("service %q not found", serviceName)})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: svc})
}

func (h *Handlers) scaleService(c *gin.Context) {
	projectID := c.Param("id")
	serviceName := c.Param("name")

	var req ScaleServiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	svc, err := h.manager.ScaleService(projectID, serviceName, req.Replicas)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		if _, ok := err.(*ScaleError); ok {
			c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "scaled", Data: svc})
}

// ========== 依赖管理 Handlers ==========

func (h *Handlers) getStartupOrder(c *gin.Context) {
	id := c.Param("id")
	order, err := h.manager.GetStartupOrder(id)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		if _, ok := err.(*DependencyError); ok {
			c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: order})
}

// ========== 健康检查 Handlers ==========

func (h *Handlers) getHealthReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetHealthReport(id)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: report})
}

func (h *Handlers) updateHealthCheck(c *gin.Context) {
	projectID := c.Param("id")
	serviceName := c.Param("name")

	var req UpdateHealthCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	svc, err := h.manager.UpdateServiceHealthCheck(projectID, serviceName, req.HealthCheck)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: svc})
}

func (h *Handlers) updateResources(c *gin.Context) {
	projectID := c.Param("id")
	serviceName := c.Param("name")

	var req UpdateResourceLimitsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	svc, err := h.manager.UpdateServiceResources(projectID, serviceName, req.Resources)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: svc})
}

// ========== 自动扩缩容 Handlers ==========

func (h *Handlers) updateAutoScale(c *gin.Context) {
	projectID := c.Param("id")
	serviceName := c.Param("name")

	var req UpdateAutoScaleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	svc, err := h.manager.UpdateAutoScalePolicy(projectID, serviceName, req.AutoScale)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: svc})
}

func (h *Handlers) evaluateAutoScale(c *gin.Context) {
	projectID := c.Param("id")
	serviceName := c.Param("name")

	var metrics ContainerMetrics
	if err := c.ShouldBindJSON(&metrics); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	event, err := h.manager.EvaluateAutoScale(projectID, serviceName, &metrics)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "evaluated", Data: event})
}

func (h *Handlers) getAutoScaleEvents(c *gin.Context) {
	id := c.Param("id")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	events := h.manager.GetAutoScaleEvents(id, limit)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(events),
			"events": events,
		},
	})
}

// ========== 日志聚合 Handlers ==========

func (h *Handlers) getServiceLogs(c *gin.Context) {
	id := c.Param("id")

	var query LogQuery
	// 从查询参数解析
	if services := c.QueryArray("services"); len(services) > 0 {
		query.Services = services
	}
	if tailStr := c.Query("tail"); tailStr != "" {
		tail, _ := strconv.Atoi(tailStr)
		query.Tail = tail
	}
	if stream := c.Query("stream"); stream != "" {
		query.Stream = stream
	}
	if timestamps := c.Query("timestamps"); timestamps == "true" {
		query.Timestamps = true
	}
	if pattern := c.Query("pattern"); pattern != "" {
		query.Pattern = pattern
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		limit, _ := strconv.Atoi(limitStr)
		query.Limit = limit
	}

	entries, err := h.manager.GetServiceLogs(id, query)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(entries),
			"entries": entries,
		},
	})
}

func (h *Handlers) streamServiceLogs(c *gin.Context) {
	id := c.Param("id")

	var query LogQuery
	query.Follow = true
	if services := c.QueryArray("services"); len(services) > 0 {
		query.Services = services
	}

	stream, err := h.manager.StreamServiceLogs(id, query)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	// 使用 SSE 流式输出
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	c.Stream(func(w io.Writer) bool {
		select {
		case entry, ok := <-stream.Entries:
			if !ok {
				return false
			}
			c.SSEvent("log", entry)
			return true
		case <-c.Request.Context().Done():
			stream.Close()
			return false
		}
	})
}

// ========== 统计 Handlers ==========

func (h *Handlers) getProjectStats(c *gin.Context) {
	id := c.Param("id")
	stats, err := h.manager.GetProjectStats(id)
	if err != nil {
		if _, ok := err.(*NotFoundError); ok {
			c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}
