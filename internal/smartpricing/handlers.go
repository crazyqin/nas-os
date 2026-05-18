// Package smartpricing 提供智能存储定价分析功能
package smartpricing

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 智能定价 API 处理器.
type Handlers struct {
	analyzer  *Analyzer
	optimizer *Optimizer
}

// NewHandlers 创建处理器.
func NewHandlers(analyzer *Analyzer, optimizer *Optimizer) *Handlers {
	return &Handlers{
		analyzer:  analyzer,
		optimizer: optimizer,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	sp := r.Group("/smart-pricing")
	{
		// 成本分析
		sp.POST("/analyze", h.analyze)

		// 方案优化
		sp.POST("/optimize", h.optimize)

		// 方案管理
		sp.GET("/plans", h.listPlans)
		sp.GET("/plans/:id", h.getPlan)
		sp.POST("/plans", h.addPlan)

		// 方案比较
		sp.POST("/compare", h.comparePlans)

		// 报告生成
		sp.POST("/report/monthly", h.generateMonthlyReport)
		sp.POST("/report/annual", h.generateAnnualReport)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// AnalyzeRequest 成本分析请求.
type AnalyzeRequest struct {
	CapacityGB int64          `json:"capacityGB"` // 容量需求（GB）
	Tier       StorageTier    `json:"tier"`       // 存储层级
	Replica    ReplicaPolicy  `json:"replica"`    // 副本策略
	Workload   WorkloadType   `json:"workload"`   // 工作负载类型
}

// analyze 成本分析接口.
func (h *Handlers) analyze(c *gin.Context) {
	var req AnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// 设置默认值
	if req.Tier == "" {
		req.Tier = TierHybrid
	}
	if req.Replica == "" {
		req.Replica = ReplicaNone
	}
	if req.Workload == "" {
		req.Workload = WorkloadMixed
	}

	analysis, err := h.analyzer.Analyze(req.CapacityGB, req.Tier, req.Replica, req.Workload)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "analysis completed",
		Data:    analysis,
	})
}

// optimize 优化推荐接口.
func (h *Handlers) optimize(c *gin.Context) {
	var req OptimizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.optimizer.Optimize(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "optimization completed",
		Data:    result,
	})
}

// listPlans 获取所有方案.
func (h *Handlers) listPlans(c *gin.Context) {
	plans := h.analyzer.GetPlans()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(plans),
			"plans": plans,
		},
	})
}

// getPlan 获取单个方案.
func (h *Handlers) getPlan(c *gin.Context) {
	id := c.Param("id")
	plan, ok := h.analyzer.FindPlan(id)
	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    404,
			Message: "plan not found",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    plan,
	})
}

// AddPlanRequest 添加方案请求.
type AddPlanRequest struct {
	ID             string        `json:"id"`
	Name           string        `json:"name"`
	Tier           StorageTier   `json:"tier"`
	Replica        ReplicaPolicy `json:"replica"`
	UnitPrice      float64       `json:"unitPrice"`
	MinCapacity    int64         `json:"minCapacity"`
	MaxCapacity    int64         `json:"maxCapacity"`
	IOPSLimit      int           `json:"iopsLimit"`
	ThroughputMB   int           `json:"throughputMB"`
	ReadLatencyMs  float64       `json:"readLatencyMs"`
	WriteLatencyMs float64       `json:"writeLatencyMs"`
	MonthlyBaseFee float64       `json:"monthlyBaseFee"`
	TransferFee    float64       `json:"transferFee"`
	Description    string        `json:"description"`
}

// addPlan 添加自定义方案.
func (h *Handlers) addPlan(c *gin.Context) {
	var req AddPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if req.ID == "" || req.Name == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "id and name are required",
		})
		return
	}

	plan := PricingPlan{
		ID:             req.ID,
		Name:           req.Name,
		Tier:           req.Tier,
		Replica:        req.Replica,
		UnitPrice:      req.UnitPrice,
		MinCapacity:    req.MinCapacity,
		MaxCapacity:    req.MaxCapacity,
		IOPSLimit:      req.IOPSLimit,
		ThroughputMB:   req.ThroughputMB,
		ReadLatencyMs:  req.ReadLatencyMs,
		WriteLatencyMs: req.WriteLatencyMs,
		MonthlyBaseFee: req.MonthlyBaseFee,
		TransferFee:    req.TransferFee,
		Description:    req.Description,
	}

	h.analyzer.AddPlan(plan)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "plan added",
		Data:    plan,
	})
}

// CompareRequest 方案比较请求.
type CompareRequest struct {
	CapacityGB int64    `json:"capacityGB"` // 容量需求（GB）
	PlanIDs    []string `json:"planIDs"`    // 要比较的方案ID列表
}

// comparePlans 方案比较接口.
func (h *Handlers) comparePlans(c *gin.Context) {
	var req CompareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if len(req.PlanIDs) == 0 {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "planIDs is required",
		})
		return
	}

	results, err := h.analyzer.ComparePlans(req.CapacityGB, req.PlanIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "comparison completed",
		Data: gin.H{
			"capacityGB": req.CapacityGB,
			"total":      len(results),
			"comparisons": results,
		},
	})
}

// ReportRequest 报告生成请求.
type ReportRequest struct {
	CapacityGB int64        `json:"capacityGB"` // 总容量
	UsedGB     int64        `json:"usedGB"`     // 已用容量
	Tier       StorageTier  `json:"tier"`       // 主要存储层级
	Replica    ReplicaPolicy `json:"replica"`   // 副本策略
}

// generateMonthlyReport 生成月度报告.
func (h *Handlers) generateMonthlyReport(c *gin.Context) {
	var req ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	report, err := GenerateMonthlyReport(h.analyzer, req.CapacityGB, req.UsedGB, req.Tier, req.Replica)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "monthly report generated",
		Data:    report,
	})
}

// generateAnnualReport 生成年度报告.
func (h *Handlers) generateAnnualReport(c *gin.Context) {
	var req ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    400,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	report, err := GenerateAnnualReport(h.analyzer, req.CapacityGB, req.UsedGB, req.Tier, req.Replica)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "annual report generated",
		Data:    report,
	})
}
