// Package dataprovenance 提供数据溯源追踪功能
package dataprovenance

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 数据溯源 HTTP 处理器.
type Handlers struct {
	engine *Engine
}

// NewHandlers 创建处理器.
func NewHandlers(engine *Engine) *Handlers {
	return &Handlers{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	provGroup := api.Group("/data-provenance")
	{
		// 溯源记录管理
		provGroup.POST("/records", h.createRecord)
		provGroup.GET("/records/:id", h.getRecord)

		// 文件历史
		provGroup.GET("/files/:fileId/history", h.getFileHistory)
		provGroup.GET("/files/:fileId/lineage", h.getLineage)
		provGroup.GET("/files/:fileId/impact", h.getImpact)
		provGroup.POST("/files/:fileId/verify", h.verifyIntegrity)

		// 用户审计
		provGroup.GET("/users/:userId/audit", h.getUserAudit)

		// 查询
		provGroup.POST("/query", h.queryRecords)

		// 合规报告
		provGroup.POST("/reports/compliance", h.generateComplianceReport)

		// 保留策略
		provGroup.GET("/retention", h.getRetentionPolicy)
		provGroup.PUT("/retention", h.updateRetentionPolicy)
		provGroup.POST("/retention/cleanup", h.cleanupExpired)
	}
}

// createRecord 创建溯源记录.
func (h *Handlers) createRecord(c *gin.Context) {
	var record ProvenanceRecord
	if err := c.ShouldBindJSON(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.engine.RecordOperation(&record); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "溯源记录创建成功",
		"record":  record,
	})
}

// getRecord 获取溯源记录.
func (h *Handlers) getRecord(c *gin.Context) {
	id := c.Param("id")
	record, err := h.engine.GetRecord(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, record)
}

// getFileHistory 获取文件变更历史.
func (h *Handlers) getFileHistory(c *gin.Context) {
	fileID := c.Param("fileId")
	history, err := h.engine.GetFileHistory(fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"file_id":  fileID,
		"history":  history,
		"total":    len(history),
	})
}

// getLineage 获取文件血缘关系.
func (h *Handlers) getLineage(c *gin.Context) {
	fileID := c.Param("fileId")
	lineage, err := h.engine.GetLineage(fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, lineage)
}

// getImpact 分析文件影响范围.
func (h *Handlers) getImpact(c *gin.Context) {
	fileID := c.Param("fileId")
	impact, err := h.engine.AnalyzeImpact(fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, impact)
}

// verifyIntegrity 校验文件完整性.
func (h *Handlers) verifyIntegrity(c *gin.Context) {
	fileID := c.Param("fileId")

	var req struct {
		CurrentHash string `json:"current_hash" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	result, err := h.engine.VerifyIntegrity(fileID, req.CurrentHash)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// getUserAudit 获取用户审计记录.
func (h *Handlers) getUserAudit(c *gin.Context) {
	userID := c.Param("userId")
	entries := h.engine.GetUserAudit(userID)
	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"entries": entries,
		"total":   len(entries),
	})
}

// queryRecords 查询溯源记录.
func (h *Handlers) queryRecords(c *gin.Context) {
	var filter QueryFilter
	if err := c.ShouldBindJSON(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	results := h.engine.QueryRecords(filter)
	c.JSON(http.StatusOK, gin.H{
		"records": results,
		"total":   len(results),
	})
}

// generateComplianceReport 生成合规报告.
func (h *Handlers) generateComplianceReport(c *gin.Context) {
	var req struct {
		StartTime time.Time `json:"start_time" binding:"required"`
		EndTime   time.Time `json:"end_time" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	report := h.engine.GenerateComplianceReport(req.StartTime, req.EndTime)
	c.JSON(http.StatusOK, report)
}

// getRetentionPolicy 获取保留策略.
func (h *Handlers) getRetentionPolicy(c *gin.Context) {
	policy := h.engine.GetRetentionPolicy()
	c.JSON(http.StatusOK, policy)
}

// updateRetentionPolicy 更新保留策略.
func (h *Handlers) updateRetentionPolicy(c *gin.Context) {
	var policy RetentionPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}
	h.engine.UpdateRetentionPolicy(&policy)
	c.JSON(http.StatusOK, gin.H{"message": "保留策略更新成功"})
}

// cleanupExpired 清理过期数据.
func (h *Handlers) cleanupExpired(c *gin.Context) {
	cleaned := h.engine.CleanupExpired()
	c.JSON(http.StatusOK, gin.H{
		"message":        "清理完成",
		"cleaned_count":  cleaned,
	})
}
