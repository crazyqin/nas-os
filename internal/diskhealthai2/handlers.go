// Package diskhealthai2 - HTTP API 处理器
package diskhealthai2

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	svc *DiskHealthService
}

// NewHandler 创建处理器
func NewHandler(svc *DiskHealthService) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(router *gin.RouterGroup) {
	group := router.Group("/diskhealthai2")
	{
		group.GET("/disks", h.ListDisks)
		group.GET("/disks/:id/smart", h.GetSMART)
		group.GET("/disks/:id/score", h.GetScore)
		group.GET("/disks/:id/predict", h.Predict)
		group.GET("/disks/:id/history", h.GetHistory)
		group.GET("/groups", h.ListGroups)
		group.GET("/groups/:id/health", h.GetGroupHealth)
		group.GET("/advice", h.GetAdvice)
		group.POST("/scan", h.TriggerScan)
		group.GET("/dashboard", h.Dashboard)
	}
}

// ListDisks 获取磁盘列表及健康状态
// GET /api/v1/diskhealthai2/disks
func (h *Handler) ListDisks(c *gin.Context) {
	devices := h.svc.Analyzer.GetDevices()
	var disks []DiskListItem

	for _, device := range devices {
		data, err := h.svc.Analyzer.GetLatestData(device)
		if err != nil {
			continue
		}

		score, err := h.svc.ScoreSys.Calculate(device)
		if err != nil {
			continue
		}

		disks = append(disks, DiskListItem{
			Device:       device,
			Model:        data.Model,
			Serial:       data.Serial,
			Status:       score.Status,
			Score:        score.Score,
			Grade:        score.Grade,
			IsSSD:        data.IsSSD,
			Capacity:     data.CapacityBytes,
			Temperature:  data.Temperature,
			PowerOnHours: data.PowerOnHours,
		})
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    disks,
	})
}

// GetSMART 获取 SMART 详情
// GET /api/v1/diskhealthai2/disks/:id/smart
func (h *Handler) GetSMART(c *gin.Context) {
	id := c.Param("id")

	data, err := h.svc.Analyzer.GetLatestData(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	analysis, err := h.svc.Analyzer.Analyze(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"smart_data": data,
			"analysis":   analysis,
		},
	})
}

// GetScore 获取健康评分
// GET /api/v1/diskhealthai2/disks/:id/score
func (h *Handler) GetScore(c *gin.Context) {
	id := c.Param("id")

	score, err := h.svc.ScoreSys.Calculate(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    score,
	})
}

// Predict 故障预测
// GET /api/v1/diskhealthai2/disks/:id/predict
func (h *Handler) Predict(c *gin.Context) {
	id := c.Param("id")

	prediction, err := h.svc.Predictor.Predict(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    prediction,
	})
}

// GetHistory 获取历史趋势
// GET /api/v1/diskhealthai2/disks/:id/history
func (h *Handler) GetHistory(c *gin.Context) {
	id := c.Param("id")

	history := h.svc.Analyzer.GetHistory(id)
	if len(history) == 0 {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   "无历史数据",
		})
		return
	}

	// 构建历史数据点
	var points []HealthHistoryPoint
	for _, data := range history {
		score, err := h.svc.ScoreSys.Calculate(id)
		if err != nil {
			continue
		}
		points = append(points, HealthHistoryPoint{
			Timestamp: data.CollectedAt,
			Score:     score.Score,
			Grade:     score.Grade,
			Status:    score.Status,
		})
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: HealthHistory{
			Device: id,
			Points: points,
			Period: "全部历史",
		},
	})
}

// ListGroups 获取磁盘组列表
// GET /api/v1/diskhealthai2/groups
func (h *Handler) ListGroups(c *gin.Context) {
	groups := h.svc.GroupMgr.EvaluateAllGroups()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    groups,
	})
}

// GetGroupHealth 获取磁盘组健康状态
// GET /api/v1/diskhealthai2/groups/:id/health
func (h *Handler) GetGroupHealth(c *gin.Context) {
	id := c.Param("id")

	group, err := h.svc.GroupMgr.EvaluateGroup(id)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    group,
	})
}

// GetAdvice 获取维护建议列表
// GET /api/v1/diskhealthai2/advice
func (h *Handler) GetAdvice(c *gin.Context) {
	advices, err := h.svc.Advisor.GenerateAdvice()
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    advices,
	})
}

// TriggerScan 触发全盘扫描
// POST /api/v1/diskhealthai2/scan
func (h *Handler) TriggerScan(c *gin.Context) {
	scanID, startedAt := h.svc.ScanAllDisk()

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: ScanTriggerResponse{
			ScanID:    scanID,
			Status:    "started",
			StartedAt: startedAt,
		},
		Message: "扫描任务已触发",
	})
}

// Dashboard 仪表板汇总
// GET /api/v1/diskhealthai2/dashboard
func (h *Handler) Dashboard(c *gin.Context) {
	devices := h.svc.Analyzer.GetDevices()

	dashboard := DashboardData{
		TotalDisks: len(devices),
		GeneratedAt: time.Now(),
	}

	worstScore := 100.0
	var worstDisk string

	for _, device := range devices {
		score, err := h.svc.ScoreSys.Calculate(device)
		if err != nil {
			continue
		}

		switch score.Status {
		case StatusHealthy:
			dashboard.HealthyDisks++
		case StatusWarning:
			dashboard.WarningDisks++
		case StatusCritical:
			dashboard.CriticalDisks++
		case StatusFailed:
			dashboard.FailedDisks++
		}

		dashboard.AverageScore += score.Score

		if score.Score < worstScore {
			worstScore = score.Score
			worstDisk = device
		}
	}

	if dashboard.TotalDisks > 0 {
		dashboard.AverageScore /= float64(dashboard.TotalDisks)
	}

	dashboard.WorstDisk = worstDisk
	dashboard.WorstScore = worstScore
	dashboard.Groups = len(h.svc.GroupMgr.ListGroups())

	advices, _ := h.svc.Advisor.GenerateAdvice()
	dashboard.Advices = len(advices)

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    dashboard,
	})
}
