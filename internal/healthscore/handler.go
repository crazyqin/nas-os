package healthscore

import (
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler HTTP API 处理器.
type Handler struct {
	scorer    *Scorer
	logger    *zap.Logger
	scoring   int32 // 原子标志，防止并发评分
}

// NewHandler 创建HTTP处理器.
func NewHandler(scorer *Scorer, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		scorer: scorer,
		logger: logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	healthGroup := api.Group("/healthscore")
	{
		healthGroup.POST("/calculate", h.calculate)
		healthGroup.GET("/overall", h.overall)
		healthGroup.GET("/details", h.details)
		healthGroup.GET("/history", h.history)
		healthGroup.GET("/recommendations", h.recommendations)
	}
}

// calculate 执行健康评分.
func (h *Handler) calculate(c *gin.Context) {
	var req CalculateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空body，使用默认值
		req = CalculateRequest{}
	}

	// 防止并发评分
	if !atomic.CompareAndSwapInt32(&h.scoring, 0, 1) {
		c.JSON(http.StatusConflict, gin.H{"error": ErrScoringInProgress.Error()})
		return
	}
	defer atomic.StoreInt32(&h.scoring, 0)

	h.logger.Info("开始健康评分", zap.Float64("threshold", req.Threshold))

	score, err := h.scorer.Calculate(req)
	if err != nil {
		h.logger.Error("评分失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "评分失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "评分完成",
		"overall_score": score.Score,
		"level":         score.Level,
		"evaluated_at":  score.EvaluatedAt,
		"alerts":        score.Alerts,
	})
}

// overall 获取综合评分.
func (h *Handler) overall(c *gin.Context) {
	score, err := h.scorer.GetLastScore()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, score)
}

// details 获取评分详情.
func (h *Handler) details(c *gin.Context) {
	details, err := h.scorer.GetDetails()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, details)
}

// history 获取历史趋势.
func (h *Handler) history(c *gin.Context) {
	var query HistoryQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		query = HistoryQuery{}
	}

	// 默认值
	if query.Days <= 0 {
		query.Days = 30
	}
	if query.Limit <= 0 {
		query.Limit = 100
	}

	resp := h.scorer.GetHistory(query)
	c.JSON(http.StatusOK, resp)
}

// recommendations 获取健康建议.
func (h *Handler) recommendations(c *gin.Context) {
	resp, err := h.scorer.GetRecommendations()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
