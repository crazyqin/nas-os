// Package securityscore 提供 REST API 处理器
package securityscore

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 安全评分模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/security 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	sec := r.Group("/security")
	{
		// 安全评分
		sec.GET("/score", h.getScore)

		// 安全检查
		sec.POST("/checks/run", h.runAllChecks)
		sec.GET("/checks", h.listChecks)
		sec.GET("/checks/:id", h.getCheckDetails)

		// 评分历史
		sec.GET("/history", h.getScoreHistory)

		// 改进建议
		sec.GET("/recommendations", h.getRecommendations)
	}
}

func (h *Handlers) getScore(c *gin.Context) {
	// 如果还没有计算过评分，先运行检查并计算
	_, err := h.manager.GetScore()
	if err != nil {
		h.manager.RunAllChecks()
		h.manager.CalculateScore()
	}

	score, err := h.manager.GetScore()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: score})
}

func (h *Handlers) runAllChecks(c *gin.Context) {
	checks := h.manager.RunAllChecks()
	score := h.manager.CalculateScore()

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "checks completed",
		Data: gin.H{
			"checks_count": len(checks),
			"score":        score,
		},
	})
}

func (h *Handlers) listChecks(c *gin.Context) {
	// 如果还没有运行过检查，先运行
	checks, err := h.manager.GetScore()
	if err != nil || checks == nil {
		h.manager.RunAllChecks()
	}

	// 通过 GetScore 获取分类中的检查
	score, _ := h.manager.GetScore()
	var allChecks []SecurityCheck
	if score != nil {
		for _, cat := range score.Categories {
			allChecks = append(allChecks, cat.Checks...)
		}
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(allChecks),
			"checks": allChecks,
		},
	})
}

func (h *Handlers) getCheckDetails(c *gin.Context) {
	id := c.Param("id")
	check, err := h.manager.GetCheckDetails(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: check})
}

func (h *Handlers) getScoreHistory(c *gin.Context) {
	history := h.manager.GetScoreHistory()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(history),
			"history": history,
		},
	})
}

func (h *Handlers) getRecommendations(c *gin.Context) {
	// 确保检查已运行
	_, err := h.manager.GetScore()
	if err != nil {
		h.manager.RunAllChecks()
		h.manager.CalculateScore()
	}

	recommendations := h.manager.GetRecommendations()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":           len(recommendations),
			"recommendations": recommendations,
		},
	})
}
