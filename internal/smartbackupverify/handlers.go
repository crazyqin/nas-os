// Package smartbackupverify 提供备份智能验证功能
package smartbackupverify

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 备份智能验证 HTTP 处理器.
type Handlers struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager, logger *zap.Logger) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{
		manager: mgr,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	sbv := api.Group("/smartbackupverify")
	{
		// 备份管理
		sbv.POST("/backups", h.registerBackup)
		sbv.GET("/backups", h.listBackups)
		sbv.GET("/backups/:id", h.getBackup)

		// 验证任务
		sbv.POST("/verify", h.runVerification)
		sbv.GET("/verify/:id", h.getVerifyTask)

		// 恢复测试
		sbv.GET("/restore-tests/:id", h.getRestoreTest)

		// 健康度评分
		sbv.GET("/health/:backupId", h.getHealthScore)

		// 报告
		sbv.GET("/reports", h.listReports)
		sbv.GET("/reports/:id", h.getReport)

		// 告警
		sbv.GET("/alerts", h.listAlerts)
		sbv.POST("/alerts", h.createAlert)
		sbv.POST("/alerts/:id/resolve", h.resolveAlert)

		// 统计
		sbv.GET("/stats", h.getStats)
	}
}

// ========== 通用响应 ==========

// Response 通用 API 响应结构.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Success 返回成功响应.
func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

// Error 返回错误响应.
func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}

// ========== 备份 API ==========

// registerBackup 注册备份.
func (h *Handlers) registerBackup(c *gin.Context) {
	var req BackupRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	backup, err := h.manager.RegisterBackup(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(backup))
}

// listBackups 列出备份.
func (h *Handlers) listBackups(c *gin.Context) {
	backups := h.manager.ListBackups()
	c.JSON(http.StatusOK, Success(backups))
}

// getBackup 获取备份.
func (h *Handlers) getBackup(c *gin.Context) {
	id := c.Param("id")

	backup, err := h.manager.GetBackup(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(backup))
}

// ========== 验证 API ==========

// runVerification 运行验证.
func (h *Handlers) runVerification(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	task, err := h.manager.RunVerification(req)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(task))
}

// getVerifyTask 获取验证任务.
func (h *Handlers) getVerifyTask(c *gin.Context) {
	id := c.Param("id")

	// 这里简化处理，实际应有单独的 GetVerifyTask 方法
	tasks := make([]*VerifyTask, 0)
	h.manager.mu.RLock()
	for _, t := range h.manager.verifyTasks {
		if t.ID == id {
			tasks = append(tasks, t)
		}
	}
	h.manager.mu.RUnlock()

	if len(tasks) == 0 {
		c.JSON(http.StatusNotFound, Error(404, "验证任务不存在"))
		return
	}

	c.JSON(http.StatusOK, Success(tasks[0]))
}

// ========== 恢复测试 API ==========

// getRestoreTest 获取恢复测试.
func (h *Handlers) getRestoreTest(c *gin.Context) {
	id := c.Param("id")

	result, err := h.manager.GetRestoreTest(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(result))
}

// ========== 健康度 API ==========

// getHealthScore 获取健康度评分.
func (h *Handlers) getHealthScore(c *gin.Context) {
	backupID := c.Param("backupId")

	score, err := h.manager.GetHealthScore(backupID)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(score))
}

// ========== 报告 API ==========

// listReports 列出报告.
func (h *Handlers) listReports(c *gin.Context) {
	reports := h.manager.ListReports()
	c.JSON(http.StatusOK, Success(reports))
}

// getReport 获取报告.
func (h *Handlers) getReport(c *gin.Context) {
	id := c.Param("id")

	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(report))
}

// ========== 告警 API ==========

// listAlerts 列出告警.
func (h *Handlers) listAlerts(c *gin.Context) {
	alerts := h.manager.ListAlerts()
	c.JSON(http.StatusOK, Success(alerts))
}

// createAlert 创建告警.
func (h *Handlers) createAlert(c *gin.Context) {
	var req struct {
		BackupID string        `json:"backup_id" binding:"required"`
		Severity AlertSeverity `json:"severity" binding:"required"`
		Title    string        `json:"title" binding:"required"`
		Message  string        `json:"message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	alert := h.manager.CreateAlert(req.BackupID, req.Severity, req.Title, req.Message)
	c.JSON(http.StatusCreated, Success(alert))
}

// resolveAlert 解决告警.
func (h *Handlers) resolveAlert(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.ResolveAlert(id); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// ========== 统计 API ==========

// getStats 获取统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, Success(stats))
}
