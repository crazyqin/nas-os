// Package quota 提供存储配额管理功能
package quota

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// AutoExpandHandlers 自动扩展 HTTP 处理器
type AutoExpandHandlers struct {
	expandMgr *AutoExpandManager
	quotaMgr  *Manager
}

// NewAutoExpandHandlers 创建自动扩展处理器
func NewAutoExpandHandlers(expandMgr *AutoExpandManager, quotaMgr *Manager) *AutoExpandHandlers {
	return &AutoExpandHandlers{
		expandMgr: expandMgr,
		quotaMgr:  quotaMgr,
	}
}

// RegisterRoutes 注册路由
func (h *AutoExpandHandlers) RegisterRoutes(api *gin.RouterGroup) {
	// 策略管理
	policies := api.Group("/auto-expand/policies")
	{
		policies.GET("", h.listPolicies)
		policies.POST("", h.createPolicy)
		policies.GET("/:id", h.getPolicy)
		policies.PUT("/:id", h.updatePolicy)
		policies.DELETE("/:id", h.deletePolicy)
		policies.GET("/:id/stats", h.getPolicyStats)
	}

	// 扩展动作
	actions := api.Group("/auto-expand/actions")
	{
		actions.GET("", h.listActions)
		actions.GET("/:id", h.getAction)
		actions.POST("/:id/approve", h.approveAction)
		actions.POST("/:id/reject", h.rejectAction)
		actions.POST("/:id/rollback", h.rollbackAction)
	}

	// 手动扩展
	manual := api.Group("/auto-expand/manual")
	{
		manual.POST("/expand", h.manualExpand)
		manual.POST("/shrink", h.manualShrink)
		manual.POST("/simulate", h.simulateExpansion)
	}

	// 建议和预测
	api.GET("/auto-expand/recommendations", h.getRecommendations)
	api.GET("/auto-expand/history/:quotaId", h.getQuotaHistory)
}

// ========== 策略管理 API ==========

func (h *AutoExpandHandlers) listPolicies(c *gin.Context) {
	quotaID := c.Query("quota_id")
	volumeName := c.Query("volume")

	policies := h.expandMgr.ListPolicies(quotaID, volumeName)
	c.JSON(http.StatusOK, Success(policies))
}

func (h *AutoExpandHandlers) createPolicy(c *gin.Context) {
	var policy AutoExpandPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	created, err := h.expandMgr.CreatePolicy(policy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, Success(created))
}

func (h *AutoExpandHandlers) getPolicy(c *gin.Context) {
	id := c.Param("id")

	policy, err := h.expandMgr.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(policy))
}

func (h *AutoExpandHandlers) updatePolicy(c *gin.Context) {
	id := c.Param("id")

	var policy AutoExpandPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	updated, err := h.expandMgr.UpdatePolicy(id, policy)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(updated))
}

func (h *AutoExpandHandlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")

	if err := h.expandMgr.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(map[string]string{"status": "deleted"}))
}

func (h *AutoExpandHandlers) getPolicyStats(c *gin.Context) {
	id := c.Param("id")

	stats, err := h.expandMgr.GetPolicyStats(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(stats))
}

// ========== 动作管理 API ==========

func (h *AutoExpandHandlers) listActions(c *gin.Context) {
	policyID := c.Query("policy_id")
	quotaID := c.Query("quota_id")
	status := c.Query("status")
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	actions := h.expandMgr.ListActions(policyID, quotaID, status, limit)
	c.JSON(http.StatusOK, Success(actions))
}

func (h *AutoExpandHandlers) getAction(c *gin.Context) {
	id := c.Param("id")

	action, err := h.expandMgr.GetAction(id)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(action))
}

type approveRequest struct {
	Approver string `json:"approver"`
}

func (h *AutoExpandHandlers) approveAction(c *gin.Context) {
	id := c.Param("id")

	var req approveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.expandMgr.ApproveExpand(id, req.Approver); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(map[string]string{"status": "approved"}))
}

type rejectRequest struct {
	Reason string `json:"reason"`
}

func (h *AutoExpandHandlers) rejectAction(c *gin.Context) {
	id := c.Param("id")

	var req rejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.expandMgr.RejectExpand(id, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(map[string]string{"status": "rejected"}))
}

func (h *AutoExpandHandlers) rollbackAction(c *gin.Context) {
	id := c.Param("id")

	var req rejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	if err := h.expandMgr.RollbackExpand(id, req.Reason); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(map[string]string{"status": "rolled_back"}))
}

// ========== 手动操作 API ==========

type manualExpandRequest struct {
	QuotaID     string `json:"quota_id" binding:"required"`
	ExpandBytes uint64 `json:"expand_bytes" binding:"required"`
	Reason      string `json:"reason"`
}

func (h *AutoExpandHandlers) manualExpand(c *gin.Context) {
	var req manualExpandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	action, err := h.expandMgr.ManualExpand(req.QuotaID, req.ExpandBytes, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(action))
}

type manualShrinkRequest struct {
	QuotaID     string `json:"quota_id" binding:"required"`
	ShrinkBytes uint64 `json:"shrink_bytes" binding:"required"`
	Reason      string `json:"reason"`
}

func (h *AutoExpandHandlers) manualShrink(c *gin.Context) {
	var req manualShrinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	action, err := h.expandMgr.ManualShrink(req.QuotaID, req.ShrinkBytes, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(action))
}

type simulateRequest struct {
	QuotaID     string `json:"quota_id" binding:"required"`
	ExpandBytes uint64 `json:"expand_bytes" binding:"required"`
}

func (h *AutoExpandHandlers) simulateExpansion(c *gin.Context) {
	var req simulateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	result, err := h.expandMgr.SimulateExpansion(req.QuotaID, req.ExpandBytes)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(result))
}

// ========== 查询 API ==========

func (h *AutoExpandHandlers) getRecommendations(c *gin.Context) {
	recommendations := h.expandMgr.GetExpansionRecommendations()
	c.JSON(http.StatusOK, Success(recommendations))
}

func (h *AutoExpandHandlers) getQuotaHistory(c *gin.Context) {
	quotaID := c.Param("quotaId")
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil {
			limit = parsed
		}
	}

	history := h.expandMgr.GetQuotaExpansionHistory(quotaID, limit)
	c.JSON(http.StatusOK, Success(history))
}

// ========== 仪表盘扩展数据 ==========

// ExpandDashboardData 扩展仪表盘数据
type ExpandDashboardData struct {
	TotalPolicies     int                          `json:"total_policies"`
	EnabledPolicies   int                          `json:"enabled_policies"`
	PendingActions    int                          `json:"pending_actions"`
	TodayExpansions   int                          `json:"today_expansions"`
	TodayExpandedBytes uint64                      `json:"today_expanded_bytes"`
	TotalExpandedBytes uint64                      `json:"total_expanded_bytes"`
	RecentActions     []*ExpandAction              `json:"recent_actions"`
	Recommendations   []*ExpansionRecommendation   `json:"recommendations"`
	PolicyStats       map[string]*ExpandPolicyStats `json:"policy_stats"`
}

// GetExpandDashboardData 获取扩展仪表盘数据
func (h *AutoExpandHandlers) GetExpandDashboardData() *ExpandDashboardData {
	data := &ExpandDashboardData{
		PolicyStats: make(map[string]*ExpandPolicyStats),
	}

	// 统计策略
	policies := h.expandMgr.ListPolicies("", "")
	for _, p := range policies {
		data.TotalPolicies++
		if p.Enabled {
			data.EnabledPolicies++
		}
	}

	// 统计动作
	actions := h.expandMgr.ListActions("", "", "", 100)
	today := time.Now().Truncate(24 * time.Hour)

	for _, a := range actions {
		if a.Status == "pending" {
			data.PendingActions++
		}
		if a.ExecutedAt != nil && a.ExecutedAt.After(today) {
			data.TodayExpansions++
			data.TodayExpandedBytes += a.ExpandBytes
		}
		data.TotalExpandedBytes += a.ExpandBytes
	}

	// 获取最近动作
	data.RecentActions = h.expandMgr.ListActions("", "", "executed", 10)

	// 获取建议
	data.Recommendations = h.expandMgr.GetExpansionRecommendations()

	// 获取各策略统计
	for _, p := range policies {
		if stats, err := h.expandMgr.GetPolicyStats(p.ID); err == nil {
			data.PolicyStats[p.ID] = stats
		}
	}

	return data
}