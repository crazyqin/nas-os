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

		// 数据溯源
		provGroup.POST("/origin", h.recordOrigin)
		provGroup.GET("/trace/:id", h.traceLineage)
		provGroup.GET("/verify/:id", h.verifyChain)
		provGroup.POST("/export", h.exportAuditTrail)

		// 文件历史
		provGroup.GET("/files/:fileId/history", h.getFileHistory)
		provGroup.GET("/files/:fileId/lineage", h.getLineage)
		provGroup.GET("/files/:fileId/impact", h.getImpact)
		provGroup.POST("/files/:fileId/verify", h.verifyIntegrity)

		// 审计链
		provGroup.POST("/chain/:chainId/records", h.addToChain)
		provGroup.GET("/chain/:chainId", h.getAuditChain)

		// 合规标签
		provGroup.POST("/compliance/:dataId/tags", h.addComplianceTag)

		// 用户审计
		provGroup.GET("/users/:userId/audit", h.getUserAudit)

		// 查询
		provGroup.POST("/query", h.queryRecords)
		provGroup.POST("/query/advanced", h.queryProvenance)

		// 合规报告
		provGroup.POST("/reports/compliance", h.generateComplianceReport)

		// 保留策略
		provGroup.GET("/retention", h.getRetentionPolicy)
		provGroup.PUT("/retention", h.updateRetentionPolicy)
		provGroup.POST("/retention/cleanup", h.cleanupExpired)
	}
}

// recordOrigin 记录数据来源.
func (h *Handlers) recordOrigin(c *gin.Context) {
	var req struct {
		DataID   string            `json:"data_id" binding:"required"`
		DataType string            `json:"data_type" binding:"required"`
		Location string            `json:"location"`
		UserID   string            `json:"user_id"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	record, err := h.engine.RecordOrigin(req.DataID, req.DataType, req.Location, req.UserID, req.Metadata)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "数据来源记录成功",
		"record":  record,
	})
}

// traceLineage 追踪数据血缘关系.
func (h *Handlers) traceLineage(c *gin.Context) {
	id := c.Param("id")
	lineage, err := h.engine.TraceLineage(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, lineage)
}

// verifyChain 验证审计链完整性.
func (h *Handlers) verifyChain(c *gin.Context) {
	chainID := c.Param("id")
	valid, err := h.engine.VerifyChain(chainID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"chain_id": chainID,
		"valid":    valid,
		"verified_at": time.Now(),
	})
}

// exportAuditTrail 导出审计追踪.
func (h *Handlers) exportAuditTrail(c *gin.Context) {
	var req AuditTrailExport
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	result, err := h.engine.ExportAuditTrail(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// addToChain 将记录添加到审计链.
func (h *Handlers) addToChain(c *gin.Context) {
	chainID := c.Param("chainId")
	var req struct {
		RecordID string `json:"record_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.engine.AddToChain(chainID, req.RecordID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "记录已添加到审计链"})
}

// getAuditChain 获取审计链.
func (h *Handlers) getAuditChain(c *gin.Context) {
	chainID := c.Param("chainId")
	chain, err := h.engine.GetAuditChain(chainID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, chain)
}

// addComplianceTag 添加合规标签.
func (h *Handlers) addComplianceTag(c *gin.Context) {
	dataID := c.Param("dataId")
	var req struct {
		Tag ComplianceTag `json:"tag" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.engine.AddComplianceTag(dataID, req.Tag); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "合规标签添加成功"})
}

// queryProvenance 高级溯源查询.
func (h *Handlers) queryProvenance(c *gin.Context) {
	var query ProvenanceQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	results, err := h.engine.QueryProvenance(&query)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"records": results,
		"total":   len(results),
	})
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
