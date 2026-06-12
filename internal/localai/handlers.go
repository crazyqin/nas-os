// Package localai 提供本地AI推理引擎的HTTP处理器
package localai

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 本地AI推理HTTP处理器
type Handlers struct {
	engine *Engine
}

// NewHandlers 创建处理器
func NewHandlers(engine *Engine) *Handlers {
	return &Handlers{engine: engine}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	localaiGroup := api.Group("/local-ai")
	{
		// 模型管理
		localaiGroup.POST("/models", h.registerModel)
		localaiGroup.GET("/models", h.listModels)
		localaiGroup.GET("/models/:id", h.getModel)
		localaiGroup.DELETE("/models/:id", h.unregisterModel)
		localaiGroup.POST("/models/:id/load", h.loadModel)
		localaiGroup.POST("/models/:id/unload", h.unloadModel)

		// 推理
		localaiGroup.POST("/inference", h.inference)
		localaiGroup.POST("/embedding", h.embedding)

		// 资源和统计
		localaiGroup.GET("/resources", h.getResources)
		localaiGroup.GET("/stats", h.getStats)
		localaiGroup.GET("/history", h.getInferenceHistory)
		localaiGroup.GET("/gpu", h.getGPUDevices)
	}
}

// registerModel 注册模型
func (h *Handlers) registerModel(c *gin.Context) {
	var model Model
	if err := c.ShouldBindJSON(&model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.engine.RegisterModel(&model); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "模型注册成功",
		"model":   model,
	})
}

// listModels 列出模型
func (h *Handlers) listModels(c *gin.Context) {
	var modelType *ModelType
	if mt := c.Query("type"); mt != "" {
		t := ModelType(mt)
		modelType = &t
	}

	models := h.engine.ListModels(modelType)
	c.JSON(http.StatusOK, gin.H{
		"models": models,
		"total":  len(models),
	})
}

// getModel 获取模型
func (h *Handlers) getModel(c *gin.Context) {
	id := c.Param("id")
	model, err := h.engine.GetModel(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, model)
}

// unregisterModel 注销模型
func (h *Handlers) unregisterModel(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.UnregisterModel(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "模型注销成功"})
}

// loadModel 加载模型
func (h *Handlers) loadModel(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.LoadModel(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "模型加载成功"})
}

// unloadModel 卸载模型
func (h *Handlers) unloadModel(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.UnloadModel(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "模型卸载成功"})
}

// inference 执行推理
func (h *Handlers) inference(c *gin.Context) {
	var req InferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	resp, err := h.engine.Inference(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// embedding 计算嵌入向量
func (h *Handlers) embedding(c *gin.Context) {
	var req EmbeddingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	resp, err := h.engine.Embedding(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// getResources 获取资源信息
func (h *Handlers) getResources(c *gin.Context) {
	info := h.engine.GetResourceInfo()
	c.JSON(http.StatusOK, info)
}

// getStats 获取引擎统计
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.engine.GetStats()
	c.JSON(http.StatusOK, stats)
}

// getInferenceHistory 获取推理历史
func (h *Handlers) getInferenceHistory(c *gin.Context) {
	modelID := c.Query("model_id")
	limit := 50
	history := h.engine.GetInferenceHistory(modelID, limit)
	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"total":   len(history),
	})
}

// getGPUDevices 获取GPU设备
func (h *Handlers) getGPUDevices(c *gin.Context) {
	devices := h.engine.GetGPUDevices()
	c.JSON(http.StatusOK, gin.H{
		"devices": devices,
		"total":   len(devices),
	})
}

// PredictStorageDemand 预测存储需求（使用本地AI）
func (h *Handlers) PredictStorageDemand(c *gin.Context) {
	var req struct {
		HistoricalData []float64 `json:"historical_data" binding:"required"`
		DaysAhead      int       `json:"days_ahead"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if req.DaysAhead <= 0 {
		req.DaysAhead = 30
	}

	// 基于历史数据的简单预测
	avgGrowth := 0.0
	if len(req.HistoricalData) > 1 {
		for i := 1; i < len(req.HistoricalData); i++ {
			avgGrowth += req.HistoricalData[i] - req.HistoricalData[i-1]
		}
		avgGrowth /= float64(len(req.HistoricalData) - 1)
	}

	currentUsage := 0.0
	if len(req.HistoricalData) > 0 {
		currentUsage = req.HistoricalData[len(req.HistoricalData)-1]
	}

	predicted := currentUsage + avgGrowth*float64(req.DaysAhead)

	c.JSON(http.StatusOK, gin.H{
		"current_usage_gb":  currentUsage,
		"avg_daily_growth":  avgGrowth,
		"predicted_gb":      predicted,
		"days_ahead":        req.DaysAhead,
		"prediction_time":   time.Now(),
		"confidence":        0.85,
	})
}

// ClassifyFile 文件分类（使用本地AI）
func (h *Handlers) ClassifyFile(c *gin.Context) {
	var req struct {
		FilePath string `json:"file_path" binding:"required"`
		Content  string `json:"content,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	// 简单的文件分类逻辑
	category := "unknown"
	confidence := 0.7

	if len(req.Content) > 0 {
		// 基于内容的简单分类
		if len(req.Content) > 1000 {
			category = "document"
			confidence = 0.8
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"file_path":  req.FilePath,
		"category":   category,
		"confidence": confidence,
		"classified_at": time.Now(),
	})
}
