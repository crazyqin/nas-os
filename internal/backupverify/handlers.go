// Package backupverify 备份验证 - HTTP API
package backupverify

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/backup-verify")
	{
		group.POST("/verify", h.RunVerification)
		group.GET("/tasks", h.ListTasks)
		group.GET("/tasks/:id", h.GetTask)
		group.POST("/schedules", h.AddSchedule)
		group.GET("/schedules", h.GetSchedules)
		group.GET("/reports/:backup_id", h.GetReport)
	}
}

// RunVerification 启动验证
func (h *Handler) RunVerification(c *gin.Context) {
	var req struct {
		BackupID   string     `json:"backup_id"`
		BackupPath string     `json:"backup_path"`
		Mode       VerifyMode `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效参数: " + err.Error()})
		return
	}
	if req.Mode == "" {
		req.Mode = ModeChecksum
	}
	task, err := h.manager.RunVerification(req.BackupID, req.BackupPath, req.Mode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"code": 0, "message": "验证已启动", "data": task})
}

// ListTasks 列出任务
func (h *Handler) ListTasks(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": h.manager.ListTasks()})
}

// GetTask 获取任务
func (h *Handler) GetTask(c *gin.Context) {
	task, err := h.manager.GetTask(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": task})
}

// AddSchedule 添加计划
func (h *Handler) AddSchedule(c *gin.Context) {
	var schedule VerifySchedule
	if err := c.ShouldBindJSON(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效参数"})
		return
	}
	if err := h.manager.AddSchedule(&schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "计划已创建", "data": schedule})
}

// GetSchedules 获取计划
func (h *Handler) GetSchedules(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": h.manager.GetSchedules()})
}

// GetReport 获取报告
func (h *Handler) GetReport(c *gin.Context) {
	report, err := h.manager.GetReport(c.Param("backup_id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": report})
}
