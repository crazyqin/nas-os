package datarights

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 数据权利 HTTP 处理器
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建处理器
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	dr := rg.Group("/datarights")
	{
		// 数据权利请求
		dr.POST("/requests", h.createRequest)
		dr.GET("/requests", h.listRequests)
		dr.GET("/requests/:id", h.getRequest)
		dr.POST("/requests/:id/process", h.processRequest)
		dr.POST("/requests/:id/reject", h.rejectRequest)

		// 删除结果
		dr.GET("/requests/:id/deletion-result", h.getDeletionResult)

		// 导出结果
		dr.GET("/requests/:id/export-result", h.getExportResult)

		// 隐私影响评估
		dr.POST("/pia", h.createPIA)
		dr.GET("/pia", h.listPIAs)
		dr.GET("/pia/:id", h.getPIA)

		// 合规报告
		dr.GET("/compliance/report", h.generateComplianceReport)

		// 统计
		dr.GET("/stats", h.getStats)
	}
}

// createRequest 创建数据权利请求
func (h *Handler) createRequest(c *gin.Context) {
	var req DataRightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.CreateRequest(&req); err != nil {
		h.logger.Error("failed to create request", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	h.logger.Info("data right request created", zap.String("id", req.ID), zap.String("type", string(req.Type)))
	c.JSON(http.StatusCreated, gin.H{"code": 0, "data": req})
}

// listRequests 列出所有请求
func (h *Handler) listRequests(c *gin.Context) {
	requests := h.manager.ListRequests()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"total":    len(requests),
		"requests": requests,
	}})
}

// getRequest 获取请求详情
func (h *Handler) getRequest(c *gin.Context) {
	id := c.Param("id")
	req, err := h.manager.GetRequest(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": req})
}

// processRequest 处理数据权利请求
func (h *Handler) processRequest(c *gin.Context) {
	id := c.Param("id")
	req, err := h.manager.GetRequest(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	switch req.Type {
	case RightAccess:
		result, err := h.manager.ProcessAccessRequest(id)
		if err != nil {
			h.logger.Error("failed to process access request", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		h.logger.Info("access request processed", zap.String("id", id))
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})

	case RightErasure:
		result, err := h.manager.ProcessDeletionRequest(id)
		if err != nil {
			h.logger.Error("failed to process erasure request", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		h.logger.Info("erasure request processed", zap.String("id", id))
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})

	case RightPortability:
		result, err := h.manager.ProcessPortabilityRequest(id)
		if err != nil {
			h.logger.Error("failed to process portability request", zap.String("id", id), zap.Error(err))
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		h.logger.Info("portability request processed", zap.String("id", id))
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})

	default:
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "unknown request type"})
	}
}

// rejectRequest 拒绝请求
func (h *Handler) rejectRequest(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RejectRequest(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	h.logger.Info("request rejected", zap.String("id", id))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "request rejected"})
}

// getDeletionResult 获取删除结果
func (h *Handler) getDeletionResult(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.GetDeletionResult(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// getExportResult 获取导出结果
func (h *Handler) getExportResult(c *gin.Context) {
	id := c.Param("id")
	result, err := h.manager.GetExportResult(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// createPIA 创建隐私影响评估
func (h *Handler) createPIA(c *gin.Context) {
	var pia PrivacyImpactAssessment
	if err := c.ShouldBindJSON(&pia); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if err := h.manager.CreatePIA(&pia); err != nil {
		h.logger.Error("failed to create PIA", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	h.logger.Info("PIA created", zap.String("id", pia.ID), zap.String("project", pia.ProjectName))
	c.JSON(http.StatusCreated, gin.H{"code": 0, "data": pia})
}

// listPIAs 列出所有隐私影响评估
func (h *Handler) listPIAs(c *gin.Context) {
	piaList := h.manager.ListPIAs()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{
		"total": len(piaList),
		"pias":  piaList,
	}})
}

// getPIA 获取隐私影响评估详情
func (h *Handler) getPIA(c *gin.Context) {
	id := c.Param("id")
	pia, err := h.manager.GetPIA(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": pia})
}

// generateComplianceReport 生成合规报告
func (h *Handler) generateComplianceReport(c *gin.Context) {
	report := h.manager.GenerateComplianceReport()
	h.logger.Info("compliance report generated", zap.String("id", report.ID), zap.Float64("score", report.ComplianceScore))
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": report})
}

// getStats 获取统计信息
func (h *Handler) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}
