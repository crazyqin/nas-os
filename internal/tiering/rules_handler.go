// Package tiering Rules Engine API 处理器
package tiering

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RulesHandler 规则引擎 API 处理器.
type RulesHandler struct {
	engine *RulesEngine
}

// NewRulesHandler 创建规则引擎 API 处理器.
func NewRulesHandler(engine *RulesEngine) *RulesHandler {
	return &RulesHandler{engine: engine}
}

// RegisterRoutes 注册规则引擎路由.
func (h *RulesHandler) RegisterRoutes(r *gin.RouterGroup) {
	tiering := r.Group("/tiering")
	{
		tiering.POST("/rules", h.CreateRule)
		tiering.GET("/rules", h.ListRules)
		tiering.GET("/rules/:id", h.GetRule)
		tiering.PUT("/rules/:id", h.UpdateRule)
		tiering.DELETE("/rules/:id", h.DeleteRule)
		tiering.POST("/rules/execute", h.ExecuteRules)
	}
}

// CreateRule 创建规则.
func (h *RulesHandler) CreateRule(c *gin.Context) {
	var rule TieringRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	created, err := h.engine.AddRule(rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已创建",
		"data":    created,
	})
}

// ListRules 列出所有规则.
func (h *RulesHandler) ListRules(c *gin.Context) {
	rules := h.engine.ListRules()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    rules,
	})
}

// GetRule 获取规则详情.
func (h *RulesHandler) GetRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := h.engine.GetRule(id)
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

// UpdateRule 更新规则.
func (h *RulesHandler) UpdateRule(c *gin.Context) {
	id := c.Param("id")
	var rule TieringRule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	if err := h.engine.UpdateRule(id, rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已更新",
	})
}

// DeleteRule 删除规则.
func (h *RulesHandler) DeleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.RemoveRule(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则已删除",
	})
}

// ExecuteRules 手动触发规则执行.
func (h *RulesHandler) ExecuteRules(c *gin.Context) {
	result, err := h.engine.Execute()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "执行失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "规则执行完成",
		"data":    result,
	})
}
