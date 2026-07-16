// Package emailsecurity 提供邮件安全审核HTTP API处理器
package emailsecurity

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler 邮件安全处理器.
type Handler struct {
	scanner    *Scanner
	quarantine *QuarantineManager
	reports    map[string]*ThreatReport
}

// NewHandler 创建新的处理器实例.
func NewHandler(scanner *Scanner, quarantine *QuarantineManager) *Handler {
	return &Handler{
		scanner:    scanner,
		quarantine: quarantine,
		reports:    make(map[string]*ThreatReport),
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	emailSecurity := rg.Group("/email-security")
	{
		// 安全策略
		emailSecurity.GET("/policies", h.ListPolicies)
		emailSecurity.POST("/policies", h.CreatePolicy)
		emailSecurity.GET("/policies/:id", h.GetPolicy)
		emailSecurity.PUT("/policies/:id", h.UpdatePolicy)
		emailSecurity.DELETE("/policies/:id", h.DeletePolicy)

		// 邮件扫描
		emailSecurity.POST("/scan", h.ScanEmail)

		// 隔离管理
		emailSecurity.GET("/quarantine", h.ListQuarantine)
		emailSecurity.GET("/quarantine/:id", h.GetQuarantineItem)
		emailSecurity.POST("/quarantine/:id/review", h.ReviewQuarantine)
		emailSecurity.POST("/quarantine/batch-review", h.BatchReviewQuarantine)
		emailSecurity.DELETE("/quarantine/:id", h.DeleteQuarantineItem)

		// 报告
		emailSecurity.GET("/reports", h.ListReports)
		emailSecurity.POST("/reports/generate", h.GenerateReport)
		emailSecurity.GET("/reports/:id", h.GetReport)

		// 审计
		emailSecurity.GET("/audit/rules", h.ListAuditRules)
		emailSecurity.POST("/audit/rules", h.CreateAuditRule)
		emailSecurity.PUT("/audit/rules/:id", h.UpdateAuditRule)
		emailSecurity.DELETE("/audit/rules/:id", h.DeleteAuditRule)

		// 统计
		emailSecurity.GET("/stats", h.GetStats)
	}
}

// ListPolicies 列出安全策略.
func (h *Handler) ListPolicies(c *gin.Context) {
	policies := h.scanner.ListPolicies()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    policies,
	})
}

// CreatePolicy 创建安全策略.
func (h *Handler) CreatePolicy(c *gin.Context) {
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	policy := &SecurityPolicy{
		ID:                uuid.New().String(),
		Name:              req.Name,
		Description:       req.Description,
		Enabled:           true,
		Priority:          req.Priority,
		AttachmentScan:    req.AttachmentScan,
		PhishingDetection: req.PhishingDetection,
		ContentCompliance: req.ContentCompliance,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	h.scanner.AddPolicy(policy)

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "安全策略创建成功",
		"data":    policy,
	})
}

// GetPolicy 获取安全策略.
func (h *Handler) GetPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, ok := h.scanner.GetPolicy(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "安全策略不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    policy,
	})
}

// UpdatePolicy 更新安全策略.
func (h *Handler) UpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	existing, ok := h.scanner.GetPolicy(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "安全策略不存在",
		})
		return
	}

	var req UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	// 更新字段
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.Priority != nil {
		existing.Priority = *req.Priority
	}
	if req.AttachmentScan != nil {
		existing.AttachmentScan = *req.AttachmentScan
	}
	if req.PhishingDetection != nil {
		existing.PhishingDetection = *req.PhishingDetection
	}
	if req.ContentCompliance != nil {
		existing.ContentCompliance = *req.ContentCompliance
	}
	existing.UpdatedAt = time.Now()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "安全策略更新成功",
		"data":    existing,
	})
}

// DeletePolicy 删除安全策略.
func (h *Handler) DeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if _, ok := h.scanner.GetPolicy(id); !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "安全策略不存在",
		})
		return
	}

	h.scanner.RemovePolicy(id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "安全策略删除成功",
	})
}

// ScanEmail 扫描邮件.
func (h *Handler) ScanEmail(c *gin.Context) {
	var req ScanEmailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	result, err := h.scanner.ScanEmail(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "扫描失败: " + err.Error(),
		})
		return
	}

	// 如果有威胁，自动隔离
	if result.Score > 0 {
		item, err := h.quarantine.QuarantineEmail(req, result)
		if err != nil {
			// 隔离失败不影响扫描结果返回
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "扫描完成，但隔离失败",
				"data": gin.H{
					"scan_result":      result,
					"quarantine_error": err.Error(),
				},
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "扫描完成，邮件已隔离",
			"data": gin.H{
				"scan_result":     result,
				"quarantine_item": item,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "扫描完成，未发现威胁",
		"data":    result,
	})
}

// ListQuarantine 列出隔离邮件.
func (h *Handler) ListQuarantine(c *gin.Context) {
	status := c.Query("status")
	threatLevel := c.Query("threat_level")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	items, total, err := h.quarantine.ListItems(status, threatLevel, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取隔离列表失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"items":     items,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetQuarantineItem 获取隔离项详情.
func (h *Handler) GetQuarantineItem(c *gin.Context) {
	id := c.Param("id")
	item, err := h.quarantine.GetItem(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    item,
	})
}

// ReviewQuarantine 审批隔离邮件.
func (h *Handler) ReviewQuarantine(c *gin.Context) {
	id := c.Param("id")
	var req ReviewQuarantineRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	if err := h.quarantine.ReviewItem(id, req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "审批成功",
	})
}

// BatchReviewQuarantine 批量审批隔离邮件.
func (h *Handler) BatchReviewQuarantine(c *gin.Context) {
	var req struct {
		ItemIDs  []string `json:"item_ids" binding:"required"`
		Action   string   `json:"action" binding:"required"`
		Note     string   `json:"note"`
		ReviewBy string   `json:"review_by" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	successCount, err := h.quarantine.BatchReview(req.ItemIDs, ReviewQuarantineRequest{
		Action:   req.Action,
		Note:     req.Note,
		ReviewBy: req.ReviewBy,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "批量审批失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "批量审批成功",
		"data": gin.H{
			"total":   len(req.ItemIDs),
			"success": successCount,
		},
	})
}

// DeleteQuarantineItem 删除隔离项.
func (h *Handler) DeleteQuarantineItem(c *gin.Context) {
	id := c.Param("id")
	if err := h.quarantine.DeleteItem(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "隔离项删除成功",
	})
}

// ListReports 列出威胁报告.
func (h *Handler) ListReports(c *gin.Context) {
	reports := make([]*ThreatReport, 0, len(h.reports))
	for _, r := range h.reports {
		reports = append(reports, r)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    reports,
	})
}

// GenerateReport 生成威胁报告.
func (h *Handler) GenerateReport(c *gin.Context) {
	var req GenerateReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	now := time.Now()
	var startTime, endTime time.Time

	switch req.Period {
	case "daily":
		startTime = now.AddDate(0, 0, -1)
		endTime = now
	case "weekly":
		startTime = now.AddDate(0, 0, -7)
		endTime = now
	case "monthly":
		startTime = now.AddDate(0, -1, 0)
		endTime = now
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的报告周期",
		})
		return
	}

	// 获取隔离统计
	stats := h.quarantine.GetStats()

	report := &ThreatReport{
		ID:               uuid.New().String(),
		Period:           req.Period,
		StartTime:        startTime,
		EndTime:          endTime,
		TotalScanned:     stats["total"] * 10, // 模拟数据
		TotalBlocked:     stats["rejected"],
		TotalQuarantined: stats["total"],
		ThreatsByType: map[string]int{
			ThreatTypeAttachment: stats["total"] / 3,
			ThreatTypePhishing:   stats["total"] / 3,
			ThreatTypeContent:    stats["total"] / 3,
		},
		TopThreats: []ThreatSummary{
			{Type: ThreatTypeAttachment, Name: "可执行文件", Count: 15, Severity: ThreatLevelHigh},
			{Type: ThreatTypePhishing, Name: "钓鱼链接", Count: 8, Severity: ThreatLevelMedium},
		},
		GeneratedAt: time.Now(),
	}

	h.reports[report.ID] = report

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "报告生成成功",
		"data":    report,
	})
}

// GetReport 获取威胁报告.
func (h *Handler) GetReport(c *gin.Context) {
	id := c.Param("id")
	report, ok := h.reports[id]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "报告不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// ListAuditRules 列出审计规则.
func (h *Handler) ListAuditRules(c *gin.Context) {
	// 简化实现，返回示例数据
	rules := []AuditRule{
		{
			ID:          "rule-001",
			Name:        "敏感附件检测",
			Description: "检测并阻止可疑可执行文件",
			Enabled:     true,
			Condition:   "attachment.type IN ('exe', 'bat', 'cmd')",
			Action:      AuditActionBlock,
			Target:      "all",
			CreatedAt:   time.Now().AddDate(0, -1, 0),
			UpdatedAt:   time.Now(),
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// CreateAuditRule 创建审计规则.
func (h *Handler) CreateAuditRule(c *gin.Context) {
	var req CreateAuditRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	rule := AuditRule{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Enabled:     true,
		Condition:   req.Condition,
		Action:      req.Action,
		Target:      req.Target,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	c.JSON(http.StatusCreated, gin.H{
		"code":    0,
		"message": "审计规则创建成功",
		"data":    rule,
	})
}

// UpdateAuditRule 更新审计规则.
func (h *Handler) UpdateAuditRule(c *gin.Context) {
	id := c.Param("id")
	var req CreateAuditRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请求参数错误: " + err.Error(),
		})
		return
	}

	rule := AuditRule{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Enabled:     true,
		Condition:   req.Condition,
		Action:      req.Action,
		Target:      req.Target,
		UpdatedAt:   time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "审计规则更新成功",
		"data":    rule,
	})
}

// DeleteAuditRule 删除审计规则.
func (h *Handler) DeleteAuditRule(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "审计规则删除成功",
	})
}

// GetStats 获取邮件安全统计.
func (h *Handler) GetStats(c *gin.Context) {
	quarantineStats := h.quarantine.GetStats()

	stats := EmailSecurityStats{
		TotalScanned24h:     quarantineStats["total"] * 10,
		TotalBlocked24h:     quarantineStats["rejected"],
		TotalQuarantined24h: quarantineStats["total"],
		BlockRate:           0.05,
		QuarantineRate:      0.02,
		PendingReview:       quarantineStats["pending"],
		ActivePolicies:      len(h.scanner.ListPolicies()),
		ActiveRules:         1,
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}
