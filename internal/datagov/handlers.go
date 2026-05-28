// Package datagov - 数据治理 HTTP API 处理器
package datagov

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 数据治理 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/datagov 路由组
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	datagov := r.Group("/datagov")
	{
		// 数据资产管理
		datagov.POST("/assets", h.createAsset)
		datagov.GET("/assets", h.listAssets)
		datagov.GET("/assets/:id", h.getAsset)
		datagov.PUT("/assets/:id", h.updateAsset)
		datagov.DELETE("/assets/:id", h.deleteAsset)

		// 扫描规则
		datagov.POST("/scan-rules", h.createScanRule)
		datagov.GET("/scan-rules", h.listScanRules)

		// 数据扫描
		datagov.POST("/assets/:id/scan", h.scanAsset)

		// 保留策略
		datagov.POST("/retention-policies", h.createRetentionPolicy)
		datagov.GET("/retention-policies", h.listRetentionPolicies)

		// 访问审计
		datagov.POST("/access-events", h.recordAccessEvent)
		datagov.GET("/access-patterns", h.getAccessPatterns)

		// 异常检测
		datagov.POST("/anomaly-rules", h.createAnomalyRule)
		datagov.POST("/anomaly-detection", h.detectAnomalies)

		// 数据血缘
		datagov.POST("/data-flows", h.createDataFlow)
		datagov.GET("/assets/:id/lineage", h.getDataLineage)

		// 数据脱敏
		datagov.POST("/masking-rules", h.createMaskingRule)
		datagov.GET("/masking-rules", h.listMaskingRules)

		// 匿名化任务
		datagov.POST("/anonymization-tasks", h.createAnonymizationTask)
		datagov.GET("/anonymization-tasks/:id", h.getAnonymizationTask)

		// 合规报告
		datagov.POST("/compliance-reports", h.generateComplianceReport)
		datagov.GET("/compliance-reports", h.listComplianceReports)

		// 配置
		datagov.GET("/config", h.getConfig)
		datagov.PUT("/config", h.updateConfig)
	}
}

// ========== 数据资产处理器 ==========

func (h *Handlers) createAsset(c *gin.Context) {
	var req DataAsset
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	asset, err := h.manager.CreateAsset(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, asset)
}

func (h *Handlers) listAssets(c *gin.Context) {
	classification := DataClassification(c.Query("classification"))
	owner := c.Query("owner")

	assets := h.manager.ListAssets(classification, owner)
	c.JSON(http.StatusOK, gin.H{
		"assets": assets,
		"total":  len(assets),
	})
}

func (h *Handlers) getAsset(c *gin.Context) {
	id := c.Param("id")
	asset, err := h.manager.GetAsset(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, asset)
}

func (h *Handlers) updateAsset(c *gin.Context) {
	id := c.Param("id")
	var req DataAsset
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	asset, err := h.manager.UpdateAsset(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, asset)
}

func (h *Handlers) deleteAsset(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteAsset(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "资产已删除"})
}

// ========== 扫描规则处理器 ==========

func (h *Handlers) createScanRule(c *gin.Context) {
	var req ScanRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule, err := h.manager.CreateScanRule(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

func (h *Handlers) listScanRules(c *gin.Context) {
	rules := h.manager.ListScanRules(nil)
	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"total": len(rules),
	})
}

// ========== 数据扫描处理器 ==========

func (h *Handlers) scanAsset(c *gin.Context) {
	assetID := c.Param("id")
	results, err := h.manager.ScanAsset(assetID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(results),
	})
}

// ========== 保留策略处理器 ==========

func (h *Handlers) createRetentionPolicy(c *gin.Context) {
	var req RetentionPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy, err := h.manager.CreateRetentionPolicy(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

func (h *Handlers) listRetentionPolicies(c *gin.Context) {
	policies := h.manager.ListRetentionPolicies()
	c.JSON(http.StatusOK, gin.H{
		"policies": policies,
		"total":    len(policies),
	})
}

// ========== 访问审计处理器 ==========

func (h *Handlers) recordAccessEvent(c *gin.Context) {
	var req AccessEvent
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.RecordAccessEvent(req)
	c.JSON(http.StatusCreated, gin.H{"message": "访问事件已记录"})
}

func (h *Handlers) getAccessPatterns(c *gin.Context) {
	patterns := h.manager.GetAccessPatterns()
	c.JSON(http.StatusOK, gin.H{
		"patterns": patterns,
		"total":    len(patterns),
	})
}

// ========== 异常检测处理器 ==========

func (h *Handlers) createAnomalyRule(c *gin.Context) {
	var req AnomalyRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule, err := h.manager.CreateAnomalyRule(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

func (h *Handlers) detectAnomalies(c *gin.Context) {
	alerts := h.manager.DetectAnomalies()
	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// ========== 数据血缘处理器 ==========

func (h *Handlers) createDataFlow(c *gin.Context) {
	var req DataFlow
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	flow, err := h.manager.CreateDataFlow(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, flow)
}

func (h *Handlers) getDataLineage(c *gin.Context) {
	assetID := c.Param("id")
	lineage, err := h.manager.GetDataLineage(assetID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, lineage)
}

// ========== 数据脱敏处理器 ==========

func (h *Handlers) createMaskingRule(c *gin.Context) {
	var req MaskingRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule, err := h.manager.CreateMaskingRule(req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

func (h *Handlers) listMaskingRules(c *gin.Context) {
	rules := h.manager.ListMaskingRules()
	c.JSON(http.StatusOK, gin.H{
		"rules": rules,
		"total": len(rules),
	})
}

// ========== 匿名化任务处理器 ==========

func (h *Handlers) createAnonymizationTask(c *gin.Context) {
	var req AnonymizationTask
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.manager.CreateAnonymizationTask(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

func (h *Handlers) getAnonymizationTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetAnonymizationTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// ========== 合规报告处理器 ==========

func (h *Handlers) generateComplianceReport(c *gin.Context) {
	var req struct {
		Framework ComplianceFramework `json:"framework" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	report, err := h.manager.GenerateComplianceReport(req.Framework)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, report)
}

func (h *Handlers) listComplianceReports(c *gin.Context) {
	reports := h.manager.ListComplianceReports()
	c.JSON(http.StatusOK, gin.H{
		"reports": reports,
		"total":   len(reports),
	})
}

// ========== 配置处理器 ==========

func (h *Handlers) getConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, config)
}

func (h *Handlers) updateConfig(c *gin.Context) {
	var config Config
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.manager.UpdateConfig(config)
	c.JSON(http.StatusOK, gin.H{"message": "配置已更新"})
}
