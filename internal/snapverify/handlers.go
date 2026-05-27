// Package snapverify 提供快照自动验证测试功能
package snapverify

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler 快照验证 HTTP 处理器
type Handler struct {
	manager *SnapVerifyManager
}

// NewHandler 创建处理器
func NewHandler(manager *SnapVerifyManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	verifyGroup := r.Group("/snap-verify")
	{
		// 测试管理
		verifyGroup.POST("/tests", h.HandleRunTest)
		verifyGroup.GET("/tests/:id", h.HandleGetTestResult)
		verifyGroup.GET("/tests", h.HandleListTests)

		// 策略管理
		verifyGroup.POST("/policies", h.HandleCreatePolicy)
		verifyGroup.PUT("/policies/:id", h.HandleUpdatePolicy)
		verifyGroup.DELETE("/policies/:id", h.HandleDeletePolicy)
		verifyGroup.GET("/policies", h.HandleListPolicies)

		// 其他操作
		verifyGroup.POST("/scheduled", h.HandleRunScheduled)
		verifyGroup.POST("/tests/:id/repair", h.HandleAutoRepair)
		verifyGroup.GET("/stats", h.HandleGetStats)
		verifyGroup.GET("/tests/:id/report", h.HandleGenerateReport)
	}
}

// HandleRunTest 运行测试
func (h *Handler) HandleRunTest(c *gin.Context) {
	var req RunTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	if req.TestType < TestTypeIntegrity || req.TestType > TestTypeFull {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_test_type",
			Message: "测试类型必须在 1-5 之间",
		})
		return
	}

	test, err := h.manager.RunTest(c.Request.Context(), req.SnapshotID, req.TestType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "run_test_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, test)
}

// HandleGetTestResult 获取测试结果
func (h *Handler) HandleGetTestResult(c *gin.Context) {
	testID := c.Param("id")
	if testID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_id",
			Message: "测试ID不能为空",
		})
		return
	}

	result, err := h.manager.GetTestResult(testID)
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, result)
}

// HandleListTests 列出测试
func (h *Handler) HandleListTests(c *gin.Context) {
	snapshotID := c.Query("snapshot_id")
	tests := h.manager.ListTests(snapshotID)

	if tests == nil {
		tests = []SnapshotTest{}
	}

	c.JSON(http.StatusOK, gin.H{
		"tests": tests,
		"total": len(tests),
	})
}

// HandleCreatePolicy 创建策略
func (h *Handler) HandleCreatePolicy(c *gin.Context) {
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	policy := VerifyPolicy{
		Name:          req.Name,
		Schedule:      req.Schedule,
		TestType:      req.TestType,
		AutoRepair:    req.AutoRepair,
		RetentionDays: req.RetentionDays,
		Enabled:       req.Enabled,
	}

	if err := h.manager.CreatePolicy(c.Request.Context(), policy); err != nil {
		code := http.StatusInternalServerError
		if err.Error() == "policy name is required" || err.Error() == "schedule is required" {
			code = http.StatusBadRequest
		}
		c.JSON(code, ErrorResponse{
			Error:   "create_policy_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "策略创建成功",
		"policy":  policy,
	})
}

// HandleUpdatePolicy 更新策略
func (h *Handler) HandleUpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_id",
			Message: "策略ID不能为空",
		})
		return
	}

	var req UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	policy := VerifyPolicy{}
	if req.Name != nil {
		policy.Name = *req.Name
	}
	if req.Schedule != nil {
		policy.Schedule = *req.Schedule
	}
	if req.TestType != nil {
		policy.TestType = *req.TestType
	}
	if req.AutoRepair != nil {
		policy.AutoRepair = *req.AutoRepair
	}
	if req.RetentionDays != nil {
		policy.RetentionDays = *req.RetentionDays
	}
	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
	}

	if err := h.manager.UpdatePolicy(id, policy); err != nil {
		code := http.StatusInternalServerError
		if err.Error() == "policy not found: "+id {
			code = http.StatusNotFound
		}
		c.JSON(code, ErrorResponse{
			Error:   "update_policy_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "策略更新成功",
		"policy_id": id,
	})
}

// HandleDeletePolicy 删除策略
func (h *Handler) HandleDeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_id",
			Message: "策略ID不能为空",
		})
		return
	}

	if err := h.manager.DeletePolicy(id); err != nil {
		code := http.StatusInternalServerError
		if err.Error() == "policy not found: "+id {
			code = http.StatusNotFound
		}
		c.JSON(code, ErrorResponse{
			Error:   "delete_policy_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "策略删除成功",
		"policy_id":  id,
	})
}

// HandleListPolicies 列出策略
func (h *Handler) HandleListPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()

	if policies == nil {
		policies = []VerifyPolicy{}
	}

	c.JSON(http.StatusOK, gin.H{
		"policies": policies,
		"total":    len(policies),
	})
}

// HandleRunScheduled 运行计划任务
func (h *Handler) HandleRunScheduled(c *gin.Context) {
	if err := h.manager.RunScheduledTests(); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "run_scheduled_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "计划任务已触发",
	})
}

// HandleAutoRepair 自动修复
func (h *Handler) HandleAutoRepair(c *gin.Context) {
	testID := c.Param("id")
	if testID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_id",
			Message: "测试ID不能为空",
		})
		return
	}

	if err := h.manager.AutoRepair(c.Request.Context(), testID); err != nil {
		code := http.StatusInternalServerError
		if err.Error() == "test result not found: "+testID {
			code = http.StatusNotFound
		} else if err.Error() == "test "+testID+" already passed, no repair needed" {
			code = http.StatusBadRequest
		}
		c.JSON(code, ErrorResponse{
			Error:   "auto_repair_failed",
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "自动修复完成",
		"test_id": testID,
	})
}

// HandleGetStats 获取统计信息
func (h *Handler) HandleGetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// HandleGenerateReport 生成报告
func (h *Handler) HandleGenerateReport(c *gin.Context) {
	testID := c.Param("id")
	if testID == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "missing_id",
			Message: "测试ID不能为空",
		})
		return
	}

	format := c.DefaultQuery("format", "json")
	if format != "json" && format != "text" {
		format = "json"
	}

	report, err := h.manager.GenerateReport(testID, format)
	if err != nil {
		code := http.StatusInternalServerError
		if err.Error() == "test not found: "+testID || err.Error() == "test result not found: "+testID {
			code = http.StatusNotFound
		}
		c.JSON(code, ErrorResponse{
			Error:   "generate_report_failed",
			Message: err.Error(),
		})
		return
	}

	if format == "text" {
		c.Data(http.StatusOK, "text/plain; charset=utf-8", report)
	} else {
		c.Data(http.StatusOK, "application/json; charset=utf-8", report)
	}
}

// HandleGetPoliciesWithPagination 分页获取策略（扩展功能）
func (h *Handler) HandleGetPoliciesWithPagination(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	policies := h.manager.ListPolicies()
	total := len(policies)

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	c.JSON(http.StatusOK, gin.H{
		"policies":  policies[start:end],
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
