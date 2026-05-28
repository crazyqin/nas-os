package containerhealthpro

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 增强版容器健康监控 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建增强版容器健康监控处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册增强版容器健康监控路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	ch := rg.Group("/container-health-pro")
	{
		// 容器管理
		ch.GET("/containers", h.listContainers)
		ch.POST("/containers", h.registerContainer)
		ch.GET("/containers/:id", h.getContainer)
		ch.DELETE("/containers/:id", h.unregisterContainer)

		// 健康检查
		ch.POST("/containers/:id/check", h.checkHealth)
		ch.POST("/check-all", h.checkAll)

		// 容器操作
		ch.POST("/containers/:id/restart", h.restartContainer)
		ch.PUT("/containers/:id/auto-restart", h.setAutoRestart)
		ch.PUT("/containers/:id/recovery-policy", h.setRecoveryPolicy)

		// 依赖关系
		ch.POST("/containers/:id/dependency", h.setDependency)
		ch.GET("/dependency-graph", h.getDependencyGraph)

		// 资源监控
		ch.PUT("/containers/:id/resource-usage", h.updateResourceUsage)

		// 日志分析
		ch.POST("/containers/:id/analyze-logs", h.analyzeLogs)
		ch.POST("/log-patterns", h.addLogPattern)

		// 性能基线
		ch.POST("/containers/:id/update-baseline", h.updateBaseline)

		// 安全扫描
		ch.POST("/containers/:id/security-scan", h.runSecurityScan)

		// 趋势和告警
		ch.GET("/containers/:id/trend", h.getHealthTrend)
		ch.GET("/containers/:id/alerts", h.getAlerts)

		// 报告
		ch.GET("/report", h.getReport)
	}
}

// listContainers 列出所有监控中的容器
func (h *Handlers) listContainers(c *gin.Context) {
	containers := h.manager.ListContainers()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": containers})
}

// registerContainer 注册容器到增强健康监控
func (h *Handlers) registerContainer(c *gin.Context) {
	var req ContainerHealthPro
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.RegisterContainer(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "容器已注册到增强健康监控"})
}

// getContainer 获取容器健康状态详情
func (h *Handlers) getContainer(c *gin.Context) {
	id := c.Param("id")
	container, err := h.manager.GetContainer(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": container})
}

// unregisterContainer 注销容器健康监控
func (h *Handlers) unregisterContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.UnregisterContainer(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "容器已注销"})
}

// checkHealth 手动检查容器健康状态
func (h *Handlers) checkHealth(c *gin.Context) {
	id := c.Param("id")
	container, err := h.manager.CheckHealth(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": container})
}

// checkAll 检查所有容器健康状态
func (h *Handlers) checkAll(c *gin.Context) {
	results := h.manager.CheckAllHealth()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": results})
}

// restartContainer 重启容器
func (h *Handlers) restartContainer(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RestartContainer(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "容器重启请求已发送"})
}

// setAutoRestart 设置容器自动重启开关
func (h *Handlers) setAutoRestart(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Enable bool `json:"enable"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.SetAutoRestart(id, req.Enable); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "自动重启设置已更新"})
}

// setRecoveryPolicy 设置容器恢复策略
func (h *Handlers) setRecoveryPolicy(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Policy RecoveryPolicy `json:"policy"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.SetRecoveryPolicy(id, req.Policy); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "恢复策略设置已更新"})
}

// setDependency 设置容器依赖关系
func (h *Handlers) setDependency(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		DependsOn []string `json:"depends_on"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.SetDependency(id, req.DependsOn); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "容器依赖关系已设置"})
}

// getDependencyGraph 获取容器依赖关系图
func (h *Handlers) getDependencyGraph(c *gin.Context) {
	graph := h.manager.GetDependencyGraph()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": graph})
}

// updateResourceUsage 更新容器资源使用情况
func (h *Handlers) updateResourceUsage(c *gin.Context) {
	id := c.Param("id")
	var req ResourceUsage
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := h.manager.UpdateResourceUsage(id, req); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "资源使用情况已更新"})
}

// analyzeLogs 分析容器日志
func (h *Handlers) analyzeLogs(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Logs []LogEntry `json:"logs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	patterns := h.manager.AnalyzeLogs(id, req.Logs)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": patterns})
}

// addLogPattern 添加日志异常模式
func (h *Handlers) addLogPattern(c *gin.Context) {
	var req LogPattern
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	h.manager.AddLogPattern(req)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "日志异常模式已添加"})
}

// updateBaseline 更新容器性能基线
func (h *Handlers) updateBaseline(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.UpdateBaseline(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "性能基线已更新"})
}

// runSecurityScan 执行安全扫描
func (h *Handlers) runSecurityScan(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.RunSecurityScan(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// getHealthTrend 获取容器健康趋势
func (h *Handlers) getHealthTrend(c *gin.Context) {
	id := c.Param("id")
	period := c.DefaultQuery("period", "24h")
	trend, err := h.manager.GetHealthTrend(id, period)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": trend})
}

// getAlerts 获取容器告警列表
func (h *Handlers) getAlerts(c *gin.Context) {
	id := c.Param("id")
	alerts := h.manager.GetAlerts(id)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": alerts})
}

// getReport 获取健康统计报告
func (h *Handlers) getReport(c *gin.Context) {
	report := h.manager.GetHealthReport()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": report})
}
