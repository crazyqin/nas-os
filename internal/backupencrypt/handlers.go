package backupencrypt

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler handles HTTP requests for backup encryption operations
type Handler struct {
	manager *Manager
}

// NewHandler creates a new backup encryption handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers HTTP routes with gin router
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	v1 := r.Group("/backup")
	{
		v1.POST("/encrypted", h.CreateBackup)
		v1.GET("/encrypted", h.ListBackups)
		v1.GET("/encrypted/:id", h.GetBackup)

		v1.POST("/restore", h.RestoreBackup)
		v1.GET("/restore/:id", h.GetRestoreJob)

		v1.POST("/keys", h.GenerateKey)
		v1.GET("/keys", h.ListKeys)
		v1.POST("/keys/:id/revoke", h.RevokeKey)

		v1.POST("/schedules", h.CreateSchedule)

		v1.POST("/integrity-check/:id", h.RunIntegrityCheck)

		v1.GET("/stats", h.GetBackupStats)
	}
}

// CreateBackup handles POST /api/v1/backup/encrypted
func (h *Handler) CreateBackup(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		SourcePath string `json:"source_path" binding:"required"`
		DestPath   string `json:"dest_path" binding:"required"`
		KeyID      string `json:"key_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	backup, err := h.manager.CreateBackup(req.Name, req.SourcePath, req.DestPath, req.KeyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, backup)
}

// GetBackup handles GET /api/v1/backup/encrypted/:id
func (h *Handler) GetBackup(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing backup ID"})
		return
	}

	backup, err := h.manager.GetBackup(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, backup)
}

// ListBackups handles GET /api/v1/backup/encrypted
func (h *Handler) ListBackups(c *gin.Context) {
	backups, err := h.manager.ListBackups()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, backups)
}

// RestoreBackup handles POST /api/v1/backup/restore
func (h *Handler) RestoreBackup(c *gin.Context) {
	var req struct {
		BackupID  string `json:"backup_id" binding:"required"`
		DestPath  string `json:"dest_path" binding:"required"`
		KeyID     string `json:"key_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	job, err := h.manager.RestoreBackup(req.BackupID, req.DestPath, req.KeyID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, job)
}

// GetRestoreJob handles GET /api/v1/backup/restore/:id
func (h *Handler) GetRestoreJob(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing restore job ID"})
		return
	}

	job, err := h.manager.GetRestoreJob(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, job)
}

// GenerateKey handles POST /api/v1/backup/keys
func (h *Handler) GenerateKey(c *gin.Context) {
	var req struct {
		Name      string `json:"name" binding:"required"`
		Algorithm string `json:"algorithm" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	key, err := h.manager.GenerateKey(req.Name, req.Algorithm)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, key)
}

// ListKeys handles GET /api/v1/backup/keys
func (h *Handler) ListKeys(c *gin.Context) {
	keys, err := h.manager.ListKeys()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, keys)
}

// RevokeKey handles POST /api/v1/backup/keys/:id/revoke
func (h *Handler) RevokeKey(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing key ID"})
		return
	}

	if err := h.manager.RevokeKey(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "revoked"})
}

// CreateSchedule handles POST /api/v1/backup/schedules
func (h *Handler) CreateSchedule(c *gin.Context) {
	var schedule BackupSchedule

	if err := c.ShouldBindJSON(&schedule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.CreateSchedule(schedule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "created", "id": schedule.ID})
}

// RunIntegrityCheck handles POST /api/v1/backup/integrity-check/:id
func (h *Handler) RunIntegrityCheck(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing backup ID"})
		return
	}

	check, err := h.manager.RunIntegrityCheck(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, check)
}

// GetBackupStats handles GET /api/v1/backup/stats
func (h *Handler) GetBackupStats(c *gin.Context) {
	stats, err := h.manager.GetBackupStats()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
