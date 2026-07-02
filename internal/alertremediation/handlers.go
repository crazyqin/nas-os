package alertremediation

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Handlers 引导式告警修复 API 处理器.
type Handlers struct {
	engine *RemediationEngine
	logger *zap.Logger
}

// NewHandlers 创建处理器.
func NewHandlers(engine *RemediationEngine, logger *zap.Logger) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{engine: engine, logger: logger}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	ar := r.Group("/alerts/remediations")
	{
		ar.GET("", h.listRemediations)
		ar.GET("/:id", h.getRemediation)
		ar.POST("/:id/execute", h.executeAction)
		ar.POST("/analyze", h.analyzeAlert)
		ar.GET("/:id/steps/:step/complete", h.completeStep)
	}
}

// apiResponse 标准 API 响应.
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listRemediations 获取所有待处理告警修复建议.
// GET /api/v1/alerts/remediations.
func (h *Handlers) listRemediations(c *gin.Context) {
	plans := h.engine.ListPlans()

	c.JSON(http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(plans),
			"plans": plans,
		},
	})
}

// getRemediation 获取单个告警修复详情.
// GET /api/v1/alerts/remediations/{id}.
func (h *Handlers) getRemediation(c *gin.Context) {
	id := c.Param("id")

	plan, ok := h.engine.GetPlan(id)
	if !ok {
		c.JSON(http.StatusNotFound, apiResponse{
			Code:    1,
			Message: "remediation plan not found: " + id,
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    plan,
	})
}

// executeRequest 执行动作请求体.
type executeRequest struct {
	ActionID string `json:"action_id" binding:"required"`
}

// executeAction 执行修复操作.
// POST /api/v1/alerts/remediations/{id}/execute.
func (h *Handlers) executeAction(c *gin.Context) {
	planID := c.Param("id")

	var req executeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 检查方案是否存在
	plan, ok := h.engine.GetPlan(planID)
	if !ok {
		c.JSON(http.StatusNotFound, apiResponse{
			Code:    1,
			Message: "remediation plan not found: " + planID,
		})
		return
	}

	// 检查动作是否需要确认
	for _, action := range plan.Actions {
		if action.ID == req.ActionID && action.RequiresConfirm {
			// 检查请求中是否包含确认标记
			confirm := c.Query("confirm")
			if confirm != "true" {
				c.JSON(http.StatusConflict, apiResponse{
					Code:    2,
					Message: "this action requires confirmation, add ?confirm=true",
					Data: gin.H{
						"action_id":   action.ID,
						"description": action.Description,
						"destructive": action.Destructive,
					},
				})
				return
			}
			break
		}
	}

	result := h.engine.ExecuteAction(c.Request.Context(), planID, req.ActionID)

	httpStatus := http.StatusOK
	if !result.Success {
		httpStatus = http.StatusInternalServerError
	}

	c.JSON(httpStatus, apiResponse{
		Code:    boolToInt(result.Success),
		Message: result.Message,
		Data:    result,
	})
}

// analyzeRequest 手动分析请求体.
type analyzeRequest struct {
	Title    string            `json:"title" binding:"required"`
	Message  string            `json:"message" binding:"required"`
	Severity AlertSeverity     `json:"severity"`
	Category AlertCategory     `json:"category"`
	Source   string            `json:"source"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// analyzeAlert 手动分析告警.
// POST /api/v1/alerts/remediations/analyze.
func (h *Handlers) analyzeAlert(c *gin.Context) {
	var req analyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 设置默认值
	if req.Severity == "" {
		req.Severity = SeverityWarning
	}
	if req.Category == "" {
		req.Category = CatSystem
	}

	// 构造告警对象
	alert := &Alert{
		ID:        uuid.New().String(),
		Title:     req.Title,
		Message:   req.Message,
		Severity:  req.Severity,
		Category:  req.Category,
		Source:    req.Source,
		Labels:    req.Labels,
		Timestamp: time.Now(),
	}

	plan := h.engine.Analyze(alert)

	h.logger.Info("alert analyzed",
		zap.String("alert_id", alert.ID),
		zap.String("plan_id", plan.ID),
		zap.String("rule_id", plan.RuleID),
	)

	c.JSON(http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data:    plan,
	})
}

// completeStep 标记排查步骤为已完成.
// GET /api/v1/alerts/remediations/{id}/steps/{step}/complete
// 注: 生产环境建议用 POST/PUT，这里简化为 GET 便于浏览器测试.
func (h *Handlers) completeStep(c *gin.Context) {
	planID := c.Param("id")
	stepStr := c.Param("step")

	var stepOrder int
	if _, err := fmt.Sscanf(stepStr, "%d", &stepOrder); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			Code:    1,
			Message: "invalid step number: " + stepStr,
		})
		return
	}

	if err := h.engine.CompleteStep(planID, stepOrder); err != nil {
		c.JSON(http.StatusNotFound, apiResponse{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		Code:    0,
		Message: "step completed",
	})
}

// boolToInt 布尔值转整数（0/1）.
func boolToInt(b bool) int {
	if b {
		return 0
	}
	return 1
}
