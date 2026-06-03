package hwcompat

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler 硬件兼容性 HTTP 处理器
type Handler struct {
	checker *HWCompatChecker
}

// NewHandler 创建 HTTP 处理器
func NewHandler(checker *HWCompatChecker) *Handler {
	return &Handler{checker: checker}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	hw := rg.Group("/hwcompat")
	{
		// 硬件扫描
		hw.POST("/scan", h.scanHardware)
		hw.GET("/hardware", h.getHardware)

		// 驱动管理
		hw.GET("/drivers", h.listDrivers)
		hw.GET("/drivers/:name", h.getDriver)
		hw.GET("/drivers/status", h.checkDriverStatus)

		// 兼容性检查
		hw.POST("/check", h.runCompatCheck)
		hw.GET("/reports/:id", h.getCompatReport)
		hw.GET("/reports", h.listCompatReports)

		// 兼容性规则
		hw.GET("/rules", h.listCompatRules)
		hw.POST("/rules", h.addCompatRule)

		// 温度监控
		hw.GET("/temperature", h.getTemperatureStatus)
		hw.POST("/temperature", h.updateTemperature)
		hw.GET("/temperature/alerts", h.getTempAlerts)

		// 硬件报告
		hw.GET("/report", h.generateHardwareReport)
	}
}

// scanHardware 扫描硬件
func (h *Handler) scanHardware(c *gin.Context) {
	hw, err := h.checker.ScanHardware()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, hw)
}

// getHardware 获取硬件信息
func (h *Handler) getHardware(c *gin.Context) {
	hw := h.checker.GetHardware()
	if hw == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "硬件信息未初始化，请先运行扫描"})
		return
	}
	c.JSON(http.StatusOK, hw)
}

// listDrivers 列出驱动
func (h *Handler) listDrivers(c *gin.Context) {
	drivers := h.checker.ListDrivers()
	c.JSON(http.StatusOK, gin.H{"drivers": drivers, "total": len(drivers)})
}

// getDriver 获取驱动信息
func (h *Handler) getDriver(c *gin.Context) {
	driver, err := h.checker.GetDriver(c.Param("name"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, driver)
}

// checkDriverStatus 检查驱动状态
func (h *Handler) checkDriverStatus(c *gin.Context) {
	status := h.checker.CheckDriverStatus()
	c.JSON(http.StatusOK, gin.H{"drivers": status, "total": len(status)})
}

// runCompatCheck 运行兼容性检查
func (h *Handler) runCompatCheck(c *gin.Context) {
	report, err := h.checker.RunCompatCheck()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

// getCompatReport 获取兼容性报告
func (h *Handler) getCompatReport(c *gin.Context) {
	report, err := h.checker.GetCompatReport(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, report)
}

// listCompatReports 列出兼容性报告
func (h *Handler) listCompatReports(c *gin.Context) {
	reports := h.checker.ListCompatReports()
	c.JSON(http.StatusOK, gin.H{"reports": reports, "total": len(reports)})
}

// listCompatRules 列出兼容性规则
func (h *Handler) listCompatRules(c *gin.Context) {
	rules := h.checker.ListCompatRules()
	c.JSON(http.StatusOK, gin.H{"rules": rules, "total": len(rules)})
}

// addCompatRuleReq 添加规则请求
type addCompatRuleReq struct {
	ID          string `json:"id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
}

// addCompatRule 添加兼容性规则
func (h *Handler) addCompatRule(c *gin.Context) {
	var req addCompatRuleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rule := &CompatRule{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
		Category:    CompatCategory(req.Category),
		Severity:    CompatSeverity(req.Severity),
	}

	h.checker.AddCompatRule(rule)
	c.JSON(http.StatusCreated, rule)
}

// getTemperatureStatus 获取温度状态
func (h *Handler) getTemperatureStatus(c *gin.Context) {
	status := h.checker.GetTemperatureStatus()
	c.JSON(http.StatusOK, status)
}

// updateTemperatureReq 更新温度请求
type updateTemperatureReq struct {
	ID      string  `json:"id" binding:"required"`
	Name    string  `json:"name"`
	Type    string  `json:"type" binding:"required"`
	Current float64 `json:"current" binding:"required"`
}

// updateTemperature 更新温度
func (h *Handler) updateTemperature(c *gin.Context) {
	var req updateTemperatureReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sensor := &TempSensor{
		ID:      req.ID,
		Name:    req.Name,
		Type:    req.Type,
		Current: req.Current,
	}

	h.checker.UpdateTemperature(sensor)
	c.JSON(http.StatusOK, gin.H{"message": "温度已更新"})
}

// getTempAlerts 获取温度告警
func (h *Handler) getTempAlerts(c *gin.Context) {
	alerts := h.checker.GetTempAlerts()
	c.JSON(http.StatusOK, gin.H{"alerts": alerts, "total": len(alerts)})
}

// generateHardwareReport 生成硬件报告
func (h *Handler) generateHardwareReport(c *gin.Context) {
	report := h.checker.GenerateHardwareReport()
	c.JSON(http.StatusOK, report)
}
