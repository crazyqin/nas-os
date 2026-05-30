package backupverify

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers backup verification HTTP handlers
type Handlers struct {
	manager *Manager
}

// NewHandlers creates new handlers
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes registers HTTP routes
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	bv := api.Group("/backupverify")
	{
		// Verification tasks
		bv.POST("/tasks", h.createTask)
		bv.GET("/tasks/:id", h.getTask)
		bv.POST("/tasks/:id/run", h.runVerify)
		bv.POST("/tasks/:id/restore-test", h.runRestoreTest)

		// Health and reports
		bv.GET("/health/:backupId", h.getBackupHealth)
		bv.GET("/report", h.generateReport)
		bv.GET("/history/:taskId", h.getVerifyHistory)

		// Repair and recommendations
		bv.POST("/repair/:backupId", h.autoRepair)
		bv.GET("/recommendations/:backupId", h.getRecommendations)
	}
}

// Response API response structure
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// createTask creates a new verification task
func (h *Handlers) createTask(c *gin.Context) {
	var task VerifyTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Code:    400,
			Message: "Invalid request body: " + err.Error(),
		})
		return
	}

	result, err := h.manager.CreateVerifyTask(c.Request.Context(), task)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "Failed to create task: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, Response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// getTask returns a verification task by ID
func (h *Handlers) getTask(c *gin.Context) {
	taskID := c.Param("id")

	task, err := h.manager.GetTask(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    task,
	})
}

// runVerify executes a verification task
func (h *Handlers) runVerify(c *gin.Context) {
	taskID := c.Param("id")

	result, err := h.manager.RunVerify(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "Verification failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// runRestoreTest executes a restore test
func (h *Handlers) runRestoreTest(c *gin.Context) {
	taskID := c.Param("id")

	result, err := h.manager.RunRestoreTest(c.Request.Context(), taskID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "Restore test failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// getBackupHealth returns the health status of a backup
func (h *Handlers) getBackupHealth(c *gin.Context) {
	backupID := c.Param("backupId")

	health, err := h.manager.GetBackupHealth(c.Request.Context(), backupID)
	if err != nil {
		c.JSON(http.StatusNotFound, Response{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    health,
	})
}

// generateReport generates a verification report
func (h *Handlers) generateReport(c *gin.Context) {
	report, err := h.manager.GenerateReport(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "Failed to generate report: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// getVerifyHistory returns verification history for a task
func (h *Handlers) getVerifyHistory(c *gin.Context) {
	taskID := c.Param("taskId")

	history := h.manager.GetVerifyHistory(c.Request.Context(), taskID)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data:    history,
	})
}

// autoRepair attempts to repair corrupted files
func (h *Handlers) autoRepair(c *gin.Context) {
	backupID := c.Param("backupId")

	repaired, err := h.manager.AutoRepair(c.Request.Context(), backupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			Code:    500,
			Message: "Auto repair failed: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"backup_id":      backupID,
			"repaired_files": repaired,
			"timestamp":      time.Now(),
		},
	})
}

// getRecommendations returns recommendations for a backup
func (h *Handlers) getRecommendations(c *gin.Context) {
	backupID := c.Param("backupId")

	recommendations := h.manager.GetRecommendations(c.Request.Context(), backupID)

	c.JSON(http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"backup_id":      backupID,
			"recommendations": recommendations,
		},
	})
}
