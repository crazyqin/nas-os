// Package storageroi 提供 RESTful API 处理器。
package storageroi

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 存储ROI API 处理器.
type Handlers struct {
	calculator *ROICalculator
	logger     *zap.Logger

	// 存储数据（实际应用中应接入数据库）
	diskCosts    map[string]*DiskCostRecord
	utilizations map[string][]*CapacityUtilization
	lifetimes    map[string]*LifetimeTracker
}

// NewHandlers 创建处理器.
func NewHandlers(calculator *ROICalculator, logger *zap.Logger) *Handlers {
	if calculator == nil {
		calculator = NewROICalculator()
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{
		calculator:   calculator,
		logger:       logger,
		diskCosts:    make(map[string]*DiskCostRecord),
		utilizations: make(map[string][]*CapacityUtilization),
		lifetimes:    make(map[string]*LifetimeTracker),
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	sroi := r.Group("/storageroi")
	{
		// 磁盘成本记录管理
		sroi.POST("/disks", h.addDiskCost)
		sroi.GET("/disks", h.listDiskCosts)
		sroi.GET("/disks/:id", h.getDiskCost)

		// 容量利用率追踪
		sroi.POST("/utilization", h.addUtilization)
		sroi.GET("/utilization/:diskId", h.getUtilization)

		// 寿命追踪
		sroi.POST("/lifetime", h.addLifetime)
		sroi.GET("/lifetime/:diskId", h.getLifetime)

		// ROI 分析
		sroi.GET("/roi/:diskId", h.GetROI)
		sroi.GET("/tco/:diskId", h.GetTCO)
		sroi.GET("/recommendations/:diskId", h.GetRecommendations)

		// 全局汇总
		sroi.GET("/summary", h.GetSummary)
	}
}

// apiResponse 标准响应.
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ==================== 磁盘成本管理 ====================

// addDiskCost 添加磁盘成本记录.
func (h *Handlers) addDiskCost(c *gin.Context) {
	var cost DiskCostRecord
	if err := c.ShouldBindJSON(&cost); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "参数错误: " + err.Error()})
		return
	}
	if cost.ID == "" {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "磁盘ID不能为空"})
		return
	}
	if cost.Currency == "" {
		cost.Currency = "CNY"
	}

	h.diskCosts[cost.ID] = &cost
	c.JSON(http.StatusCreated, apiResponse{Code: 0, Message: "添加成功", Data: cost})
}

// listDiskCosts 列出所有磁盘成本记录.
func (h *Handlers) listDiskCosts(c *gin.Context) {
	costs := make([]*DiskCostRecord, 0, len(h.diskCosts))
	for _, cost := range h.diskCosts {
		costs = append(costs, cost)
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: costs})
}

// getDiskCost 获取单个磁盘成本记录.
func (h *Handlers) getDiskCost(c *gin.Context) {
	id := c.Param("id")
	cost, ok := h.diskCosts[id]
	if !ok {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: "磁盘成本记录不存在"})
		return
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: cost})
}

// ==================== 容量利用率 ====================

// addUtilization 添加容量利用率记录.
func (h *Handlers) addUtilization(c *gin.Context) {
	var util CapacityUtilization
	if err := c.ShouldBindJSON(&util); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "参数错误: " + err.Error()})
		return
	}
	if util.DiskID == "" {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "磁盘ID不能为空"})
		return
	}

	h.utilizations[util.DiskID] = append(h.utilizations[util.DiskID], &util)
	c.JSON(http.StatusCreated, apiResponse{Code: 0, Message: "添加成功", Data: util})
}

// getUtilization 获取磁盘容量利用率历史.
func (h *Handlers) getUtilization(c *gin.Context) {
	diskID := c.Param("diskId")
	utils, ok := h.utilizations[diskID]
	if !ok {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: "未找到该磁盘的利用率记录"})
		return
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: utils})
}

// ==================== 寿命追踪 ====================

// addLifetime 添加磁盘寿命追踪记录.
func (h *Handlers) addLifetime(c *gin.Context) {
	var lt LifetimeTracker
	if err := c.ShouldBindJSON(&lt); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "参数错误: " + err.Error()})
		return
	}
	if lt.DiskID == "" {
		c.JSON(http.StatusBadRequest, apiResponse{Code: 1, Message: "磁盘ID不能为空"})
		return
	}

	h.lifetimes[lt.DiskID] = &lt
	c.JSON(http.StatusCreated, apiResponse{Code: 0, Message: "添加成功", Data: lt})
}

// getLifetime 获取磁盘寿命追踪信息.
func (h *Handlers) getLifetime(c *gin.Context) {
	diskID := c.Param("diskId")
	lt, ok := h.lifetimes[diskID]
	if !ok {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: "未找到该磁盘的寿命记录"})
		return
	}
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: lt})
}

// ==================== ROI 分析接口 ====================

// GetROI 获取磁盘ROI评分.
func (h *Handlers) GetROI(c *gin.Context) {
	diskID := c.Param("diskId")

	cost, ok := h.diskCosts[diskID]
	if !ok {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: "磁盘成本记录不存在"})
		return
	}

	lifetime, ok := h.lifetimes[diskID]
	if !ok {
		lifetime = &LifetimeTracker{
			DiskID:       diskID,
			PurchaseDate: cost.PurchaseDate,
			Status:       DiskStatusActive,
			HealthScore:  80,
		}
	}

	utils := h.utilizations[diskID]

	tco := h.calculator.CalculateTCO(cost, lifetime)
	score := h.calculator.CalculateROI(utils, tco, lifetime, cost)

	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: score})
}

// GetTCO 获取磁盘TCO报告.
func (h *Handlers) GetTCO(c *gin.Context) {
	diskID := c.Param("diskId")

	cost, ok := h.diskCosts[diskID]
	if !ok {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: "磁盘成本记录不存在"})
		return
	}

	lifetime, ok := h.lifetimes[diskID]
	if !ok {
		lifetime = &LifetimeTracker{
			DiskID:       diskID,
			PurchaseDate: cost.PurchaseDate,
			Status:       DiskStatusActive,
			HealthScore:  80,
		}
	}

	tco := h.calculator.CalculateTCO(cost, lifetime)
	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: tco})
}

// GetRecommendations 获取优化建议.
func (h *Handlers) GetRecommendations(c *gin.Context) {
	diskID := c.Param("diskId")

	cost, ok := h.diskCosts[diskID]
	if !ok {
		c.JSON(http.StatusNotFound, apiResponse{Code: 1, Message: "磁盘成本记录不存在"})
		return
	}

	lifetime, ok := h.lifetimes[diskID]
	if !ok {
		lifetime = &LifetimeTracker{
			DiskID:       diskID,
			PurchaseDate: cost.PurchaseDate,
			Status:       DiskStatusActive,
			HealthScore:  80,
		}
	}

	utils := h.utilizations[diskID]
	tco := h.calculator.CalculateTCO(cost, lifetime)
	score := h.calculator.CalculateROI(utils, tco, lifetime, cost)

	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: score.Recommendations})
}

// GetSummary 获取所有磁盘的ROI汇总.
func (h *Handlers) GetSummary(c *gin.Context) {
	type diskSummary struct {
		DiskID   string     `json:"disk_id"`
		ROIScore *ROIScore  `json:"roi_score"`
		TCO      *TCOReport `json:"tco"`
	}

	summaries := make([]diskSummary, 0, len(h.diskCosts))
	for diskID, cost := range h.diskCosts {
		lifetime, ok := h.lifetimes[diskID]
		if !ok {
			lifetime = &LifetimeTracker{
				DiskID:       diskID,
				PurchaseDate: cost.PurchaseDate,
				Status:       DiskStatusActive,
				HealthScore:  80,
			}
		}
		utils := h.utilizations[diskID]
		tco := h.calculator.CalculateTCO(cost, lifetime)
		score := h.calculator.CalculateROI(utils, tco, lifetime, cost)
		summaries = append(summaries, diskSummary{
			DiskID:   diskID,
			ROIScore: score,
			TCO:      tco,
		})
	}

	c.JSON(http.StatusOK, apiResponse{Code: 0, Message: "success", Data: summaries})
}
