package chaosengineer

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 混沌工程HTTP处理器.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建混沌工程HTTP处理器.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes 注册混沌工程API路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	chaos := rg.Group("/chaos")
	{
		// 实验管理
		chaos.POST("/experiments", h.CreateExperiment)
		chaos.GET("/experiments", h.ListExperiments)
		chaos.GET("/experiments/:id", h.GetExperiment)
		chaos.PUT("/experiments/:id", h.UpdateExperiment)
		chaos.DELETE("/experiments/:id", h.DeleteExperiment)

		// 实验执行
		chaos.POST("/experiments/:id/start", h.StartExperiment)
		chaos.POST("/experiments/:id/stop", h.StopExperiment)

		// 韧性评估
		chaos.POST("/reports/generate", h.GenerateReport)
		chaos.GET("/reports", h.ListReports)
		chaos.GET("/reports/:id", h.GetReport)

		// 仪表盘
		chaos.GET("/dashboard", h.GetDashboard)
	}
}

// CreateExperiment handles POST /api/v1/chaos/experiments.
func (h *Handler) CreateExperiment(c *gin.Context) {
	var exp Experiment
	if err := c.ShouldBindJSON(&exp); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	created, err := h.manager.CreateExperiment(&exp)
	if err != nil {
		if err == ErrInvalidFaultType || err == ErrInvalidSeverity || err == ErrNoTargetSpecified {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	h.logger.Info("experiment created", zap.String("id", created.ID), zap.String("name", created.Name))
	c.JSON(http.StatusCreated, created)
}

// GetExperiment handles GET /api/v1/chaos/experiments/:id.
func (h *Handler) GetExperiment(c *gin.Context) {
	id := c.Param("id")
	exp, err := h.manager.GetExperiment(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, exp)
}

// ListExperiments handles GET /api/v1/chaos/experiments.
func (h *Handler) ListExperiments(c *gin.Context) {
	experiments := h.manager.ListExperiments()
	c.JSON(http.StatusOK, gin.H{
		"experiments": experiments,
		"total":       len(experiments),
	})
}

// UpdateExperiment handles PUT /api/v1/chaos/experiments/:id.
func (h *Handler) UpdateExperiment(c *gin.Context) {
	id := c.Param("id")
	var update Experiment
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updated, err := h.manager.UpdateExperiment(id, &update)
	if err != nil {
		switch err {
		case ErrExperimentNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case ErrExperimentRunning:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case ErrInvalidFaultType, ErrInvalidSeverity:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	h.logger.Info("experiment updated", zap.String("id", id))
	c.JSON(http.StatusOK, updated)
}

// DeleteExperiment handles DELETE /api/v1/chaos/experiments/:id.
func (h *Handler) DeleteExperiment(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteExperiment(id); err != nil {
		switch err {
		case ErrExperimentNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case ErrExperimentRunning:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	h.logger.Info("experiment deleted", zap.String("id", id))
	c.JSON(http.StatusOK, gin.H{"message": "experiment deleted"})
}

// StartExperiment handles POST /api/v1/chaos/experiments/:id/start.
func (h *Handler) StartExperiment(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StartExperiment(id); err != nil {
		switch err {
		case ErrExperimentNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case ErrExperimentRunning:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case ErrSafetyViolation:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	h.logger.Info("experiment started", zap.String("id", id))
	c.JSON(http.StatusOK, gin.H{"message": "experiment started"})
}

// StopExperiment handles POST /api/v1/chaos/experiments/:id/stop.
func (h *Handler) StopExperiment(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.StopExperiment(id); err != nil {
		switch err {
		case ErrExperimentNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case ErrExperimentNotRun:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	h.logger.Info("experiment stopped", zap.String("id", id))
	c.JSON(http.StatusOK, gin.H{"message": "experiment stopped"})
}

// GenerateReport handles POST /api/v1/chaos/reports/generate.
func (h *Handler) GenerateReport(c *gin.Context) {
	report := h.manager.GenerateReport()

	h.logger.Info("resilience report generated", zap.String("id", report.ID), zap.Float64("score", report.OverallScore))
	c.JSON(http.StatusCreated, report)
}

// ListReports handles GET /api/v1/chaos/reports.
func (h *Handler) ListReports(c *gin.Context) {
	reports := h.manager.ListReports()
	c.JSON(http.StatusOK, gin.H{
		"reports": reports,
		"total":   len(reports),
	})
}

// GetReport handles GET /api/v1/chaos/reports/:id.
func (h *Handler) GetReport(c *gin.Context) {
	id := c.Param("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

// GetDashboard handles GET /api/v1/chaos/dashboard.
func (h *Handler) GetDashboard(c *gin.Context) {
	dashboard := h.manager.GetDashboard()
	c.JSON(http.StatusOK, dashboard)
}
