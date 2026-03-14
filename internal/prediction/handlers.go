package prediction

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	prediction := r.Group("/prediction")
	{
		prediction.GET("/volumes", h.ListVolumes)
		prediction.GET("/volumes/:name", h.GetPrediction)
		prediction.GET("/volumes/:name/history", h.GetHistory)
		prediction.POST("/volumes/:name/record", h.RecordUsage)
		prediction.GET("/all", h.GetAllPredictions)
		prediction.GET("/config", h.GetConfig)
		prediction.PUT("/config", h.UpdateConfig)
		prediction.GET("/advices", h.GetAllAdvices)
	}
}

// VolumeListResponse 卷列表响应
type VolumeListResponse struct {
	Volumes []VolumeInfo `json:"volumes"`
	Count   int          `json:"count"`
}

// VolumeInfo 卷信息
type VolumeInfo struct {
	Name         string    `json:"name"`
	LastUpdated  time.Time `json:"lastUpdated"`
	RecordCount  int       `json:"recordCount"`
	CurrentUsage float64   `json:"currentUsage"`
	CurrentTotal float64   `json:"currentTotal"`
	UsageRate    float64   `json:"usageRate"`
}

// PredictionResponse 预测响应
type PredictionResponse struct {
	Success bool              `json:"success"`
	Data    *PredictionResult `json:"data,omitempty"`
	Error   string            `json:"error,omitempty"`
}

// HistoryResponse 历史数据响应
type HistoryResponse struct {
	Success bool          `json:"success"`
	Data    []UsageRecord `json:"data,omitempty"`
	Count   int           `json:"count"`
	Error   string        `json:"error,omitempty"`
}

// RecordRequest 记录请求
type RecordRequest struct {
	UsedGB  float64 `json:"usedGB" binding:"required"`
	TotalGB float64 `json:"totalGB" binding:"required"`
}

// ConfigResponse 配置响应
type ConfigResponse struct {
	Success bool    `json:"success"`
	Data    *Config `json:"data,omitempty"`
	Error   string  `json:"error,omitempty"`
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	CollectionInterval   string  `json:"collectionInterval"`
	HistoryRetentionDays int     `json:"historyRetentionDays"`
	PredictionDays       int     `json:"predictionDays"`
	AnomalySensitivity   float64 `json:"anomalySensitivity"`
	WarningThreshold     float64 `json:"warningThreshold"`
	CriticalThreshold    float64 `json:"criticalThreshold"`
	EnableAutoAdvice     bool    `json:"enableAutoAdvice"`
}

// AllPredictionsResponse 全量预测响应
type AllPredictionsResponse struct {
	Success bool                         `json:"success"`
	Data    map[string]*PredictionResult `json:"data,omitempty"`
	Count   int                          `json:"count"`
	Error   string                       `json:"error,omitempty"`
}

// AdviceSummary 建议摘要
type AdviceSummary struct {
	VolumeName string   `json:"volumeName"`
	Advices    []Advice `json:"advices"`
}

// AllAdvicesResponse 全部建议响应
type AllAdvicesResponse struct {
	Success bool            `json:"success"`
	Data    []AdviceSummary `json:"data,omitempty"`
	Total   int             `json:"total"`
	Error   string          `json:"error,omitempty"`
}

// ListVolumes 列出所有有历史数据的卷
// @Summary 列出预测卷
// @Description 列出所有有历史数据的卷
// @Tags prediction
// @Produce json
// @Success 200 {object} VolumeListResponse
// @Router /prediction/volumes [get]
func (h *Handlers) ListVolumes(c *gin.Context) {
	volumeNames := h.manager.ListVolumes()

	volumes := make([]VolumeInfo, 0, len(volumeNames))
	for _, name := range volumeNames {
		info := VolumeInfo{Name: name}

		// 获取历史数据
		records, err := h.manager.GetHistory(name, 1)
		if err == nil && len(records) > 0 {
			latest := records[len(records)-1]
			info.LastUpdated = latest.Timestamp
			info.RecordCount = len(records)
			info.CurrentUsage = latest.UsedGB
			info.CurrentTotal = latest.TotalGB
			info.UsageRate = latest.UsageRate
		}

		volumes = append(volumes, info)
	}

	c.JSON(http.StatusOK, VolumeListResponse{
		Volumes: volumes,
		Count:   len(volumes),
	})
}

// GetPrediction 获取卷的预测结果
// @Summary 获取预测结果
// @Description 获取指定卷的存储使用预测
// @Tags prediction
// @Produce json
// @Param name path string true "卷名称"
// @Success 200 {object} PredictionResponse
// @Failure 404 {object} PredictionResponse
// @Router /prediction/volumes/{name} [get]
func (h *Handlers) GetPrediction(c *gin.Context) {
	volumeName := c.Param("name")

	result, err := h.manager.Predict(volumeName)
	if err != nil {
		c.JSON(http.StatusNotFound, PredictionResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, PredictionResponse{
		Success: true,
		Data:    result,
	})
}

// GetHistory 获取卷的历史数据
// @Summary 获取历史数据
// @Description 获取指定卷的历史使用数据
// @Tags prediction
// @Produce json
// @Param name path string true "卷名称"
// @Param days query int false "天数" default(7)
// @Success 200 {object} HistoryResponse
// @Failure 404 {object} HistoryResponse
// @Router /prediction/volumes/{name}/history [get]
func (h *Handlers) GetHistory(c *gin.Context) {
	volumeName := c.Param("name")
	days := 7
	if d := c.Query("days"); d != "" {
		if parsed, err := time.ParseDuration(d + "h"); err == nil {
			days = int(parsed.Hours() / 24)
		}
	}

	records, err := h.manager.GetHistory(volumeName, days)
	if err != nil {
		c.JSON(http.StatusNotFound, HistoryResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, HistoryResponse{
		Success: true,
		Data:    records,
		Count:   len(records),
	})
}

// RecordUsage 记录使用数据
// @Summary 记录使用数据
// @Description 记录卷的使用数据（供外部采集器调用）
// @Tags prediction
// @Accept json
// @Produce json
// @Param name path string true "卷名称"
// @Param request body RecordRequest true "使用数据"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /prediction/volumes/{name}/record [post]
func (h *Handlers) RecordUsage(c *gin.Context) {
	volumeName := c.Param("name")

	var req RecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "无效的请求数据: " + err.Error(),
		})
		return
	}

	if err := h.manager.RecordUsage(volumeName, req.UsedGB, req.TotalGB); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "数据已记录",
	})
}

// GetAllPredictions 获取所有卷的预测
// @Summary 获取全量预测
// @Description 获取所有卷的存储使用预测
// @Tags prediction
// @Produce json
// @Success 200 {object} AllPredictionsResponse
// @Router /prediction/all [get]
func (h *Handlers) GetAllPredictions(c *gin.Context) {
	results, err := h.manager.GetAllPredictions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, AllPredictionsResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, AllPredictionsResponse{
		Success: true,
		Data:    results,
		Count:   len(results),
	})
}

// GetConfig 获取配置
// @Summary 获取预测配置
// @Description 获取当前的预测管理器配置
// @Tags prediction
// @Produce json
// @Success 200 {object} ConfigResponse
// @Router /prediction/config [get]
func (h *Handlers) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, ConfigResponse{
		Success: true,
		Data:    h.manager.GetConfig(),
	})
}

// UpdateConfig 更新配置
// @Summary 更新预测配置
// @Description 更新预测管理器配置
// @Tags prediction
// @Accept json
// @Produce json
// @Param request body UpdateConfigRequest true "配置数据"
// @Success 200 {object} ConfigResponse
// @Failure 400 {object} ConfigResponse
// @Router /prediction/config [put]
func (h *Handlers) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ConfigResponse{
			Success: false,
			Error:   "无效的请求数据: " + err.Error(),
		})
		return
	}

	cfg := h.manager.GetConfig()

	// 更新字段
	if req.CollectionInterval != "" {
		if duration, err := time.ParseDuration(req.CollectionInterval); err == nil {
			cfg.CollectionInterval = duration
		}
	}
	if req.HistoryRetentionDays > 0 {
		cfg.HistoryRetentionDays = req.HistoryRetentionDays
	}
	if req.PredictionDays > 0 {
		cfg.PredictionDays = req.PredictionDays
	}
	if req.AnomalySensitivity > 0 && req.AnomalySensitivity <= 1 {
		cfg.AnomalySensitivity = req.AnomalySensitivity
	}
	if req.WarningThreshold > 0 {
		cfg.WarningThreshold = req.WarningThreshold
	}
	if req.CriticalThreshold > 0 {
		cfg.CriticalThreshold = req.CriticalThreshold
	}
	cfg.EnableAutoAdvice = req.EnableAutoAdvice

	if err := h.manager.UpdateConfig(cfg); err != nil {
		c.JSON(http.StatusBadRequest, ConfigResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ConfigResponse{
		Success: true,
		Data:    cfg,
	})
}

// GetAllAdvices 获取所有建议
// @Summary 获取全部建议
// @Description 获取所有卷的优化建议
// @Tags prediction
// @Produce json
// @Success 200 {object} AllAdvicesResponse
// @Router /prediction/advices [get]
func (h *Handlers) GetAllAdvices(c *gin.Context) {
	results, err := h.manager.GetAllPredictions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, AllAdvicesResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	summaries := make([]AdviceSummary, 0)
	total := 0

	for volName, result := range results {
		if len(result.Advices) > 0 {
			summaries = append(summaries, AdviceSummary{
				VolumeName: volName,
				Advices:    result.Advices,
			})
			total += len(result.Advices)
		}
	}

	c.JSON(http.StatusOK, AllAdvicesResponse{
		Success: true,
		Data:    summaries,
		Total:   total,
	})
}
