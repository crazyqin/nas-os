// Package tiering 智能分层规则引擎 API 处理器 - 提供智能规则CRUD、预设模板、迁移计划和分层效果报告的HTTP接口。
package tiering

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SmartTieringHandler 智能分层 API 处理器.
type SmartTieringHandler struct {
	engine   *AutoTierEngine
	analyzer *CostAnalyzer
}

// NewSmartTieringHandler 创建智能分层 API 处理器.
func NewSmartTieringHandler(engine *AutoTierEngine, analyzer *CostAnalyzer) *SmartTieringHandler {
	return &SmartTieringHandler{
		engine:   engine,
		analyzer: analyzer,
	}
}

// RegisterRoutes 注册智能分层路由.
func (h *SmartTieringHandler) RegisterRoutes(r *gin.RouterGroup) {
	tiering := r.Group("/tiering")
	{
		// 智能分层规则 CRUD
		tiering.GET("/smart-rules", h.ListSmartRules)
		tiering.POST("/smart-rules", h.CreateSmartRule)
		tiering.GET("/smart-rules/:id", h.GetSmartRule)
		tiering.PUT("/smart-rules/:id", h.UpdateSmartRule)
		tiering.DELETE("/smart-rules/:id", h.DeleteSmartRule)
		tiering.POST("/smart-rules/execute", h.ExecuteSmartRules)

		// 预设模板
		tiering.GET("/templates", h.ListTemplates)

		// 迁移计划
		tiering.POST("/migration-plan", h.GenerateMigrationPlan)

		// 分层效果报告
		tiering.GET("/report", h.GetTieringReport)
		tiering.GET("/report/roi", h.GetROIReport)
	}
}

// ListSmartRules 获取智能分层规则列表.
//
//	@Summary      获取智能分层规则列表
//	@Description  返回所有已配置的智能分层规则，按优先级排序
//	@Tags         智能分层
//	@Accept       json
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Router       /api/v1/tiering/smart-rules [get]
func (h *SmartTieringHandler) ListSmartRules(c *gin.Context) {
	rules := h.engine.ListSmartRules()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// CreateSmartRule 创建智能分层规则.
//
//	@Summary      创建智能分层规则
//	@Description  创建一条新的智能分层规则，支持基于age/frequency/size的组合条件
//	@Tags         智能分层
//	@Accept       json
//	@Produce      json
//	@Param        rule  body      SmartRule  true  "智能分层规则"
//	@Success      200   {object}  map[string]interface{}
//	@Failure      400   {object}  map[string]interface{}
//	@Router       /api/v1/tiering/smart-rules [post]
func (h *SmartTieringHandler) CreateSmartRule(c *gin.Context) {
	var rule SmartRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	created, err := h.engine.AddSmartRule(rule)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "智能规则已创建",
		"data":    created,
	})
}

// GetSmartRule 获取智能分层规则详情.
//
//	@Summary      获取智能分层规则详情
//	@Description  根据ID获取单条智能分层规则
//	@Tags         智能分层
//	@Accept       json
//	@Produce      json
//	@Param        id   path      string  true  "规则ID"
//	@Success      200  {object}  map[string]interface{}
//	@Failure      404  {object}  map[string]interface{}
//	@Router       /api/v1/tiering/smart-rules/{id} [get]
func (h *SmartTieringHandler) GetSmartRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.engine.GetSmartRule(id)
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
		"data":    rule,
	})
}

// UpdateSmartRule 更新智能分层规则.
//
//	@Summary      更新智能分层规则
//	@Description  根据ID更新智能分层规则
//	@Tags         智能分层
//	@Accept       json
//	@Produce      json
//	@Param        id    path      string     true  "规则ID"
//	@Param        rule  body      SmartRule  true  "更新内容"
//	@Success      200   {object}  map[string]interface{}
//	@Failure      400   {object}  map[string]interface{}
//	@Failure      404   {object}  map[string]interface{}
//	@Router       /api/v1/tiering/smart-rules/{id} [put]
func (h *SmartTieringHandler) UpdateSmartRule(c *gin.Context) {
	id := c.Param("id")
	var rule SmartRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	if err := h.engine.UpdateSmartRule(id, rule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "智能规则已更新",
	})
}

// DeleteSmartRule 删除智能分层规则.
//
//	@Summary      删除智能分层规则
//	@Description  根据ID删除智能分层规则
//	@Tags         智能分层
//	@Accept       json
//	@Produce      json
//	@Param        id   path      string  true  "规则ID"
//	@Success      200  {object}  map[string]interface{}
//	@Failure      404  {object}  map[string]interface{}
//	@Router       /api/v1/tiering/smart-rules/{id} [delete]
func (h *SmartTieringHandler) DeleteSmartRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.DeleteSmartRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "智能规则已删除",
	})
}

// ExecuteSmartRules 手动触发智能规则执行.
//
//	@Summary      执行智能分层规则
//	@Description  手动触发所有启用的智能规则执行分层迁移
//	@Tags         智能分层
//	@Accept       json
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Failure      500  {object}  map[string]interface{}
//	@Router       /api/v1/tiering/smart-rules/execute [post]
func (h *SmartTieringHandler) ExecuteSmartRules(c *gin.Context) {
	result, err := h.engine.ExecuteSmartRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "执行失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "智能规则执行完成",
		"data":    result,
	})
}

// ListTemplates 获取预设策略模板.
//
//	@Summary      获取预设策略模板列表
//	@Description  返回所有预设的分层策略模板（高性能/均衡/大容量/归档）
//	@Tags         智能分层
//	@Accept       json
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Router       /api/v1/tiering/templates [get]
func (h *SmartTieringHandler) ListTemplates(c *gin.Context) {
	templates := GetPresetTemplates()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    templates,
	})
}

// GenerateMigrationPlan 生成迁移计划.
//
//	@Summary      生成迁移计划
//	@Description  基于当前启用的智能规则生成迁移计划，预估迁移时间和节省成本
//	@Tags         智能分层
//	@Accept       json
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Failure      500  {object}  map[string]interface{}
//	@Router       /api/v1/tiering/migration-plan [post]
func (h *SmartTieringHandler) GenerateMigrationPlan(c *gin.Context) {
	plan, err := h.engine.GenerateMigrationPlan()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "生成迁移计划失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    plan,
	})
}

// GetTieringReport 获取分层效果报告.
//
//	@Summary      获取分层效果报告
//	@Description  获取分层前后的性能对比、成本分析和优化建议
//	@Tags         智能分层
//	@Accept       json
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Router       /api/v1/tiering/report [get]
func (h *SmartTieringHandler) GetTieringReport(c *gin.Context) {
	report := h.analyzer.GenerateTieringEffectReport()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}

// GetROIReport 获取分层ROI报告.
//
//	@Summary      获取分层ROI报告
//	@Description  获取分层投资回报率分析，包括投资成本、收益和回本周期
//	@Tags         智能分层
//	@Accept       json
//	@Produce      json
//	@Success      200  {object}  map[string]interface{}
//	@Router       /api/v1/tiering/report/roi [get]
func (h *SmartTieringHandler) GetROIReport(c *gin.Context) {
	report := h.analyzer.GenerateROIReport()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    report,
	})
}
