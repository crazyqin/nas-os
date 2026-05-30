// Package gitopsmgr provides GitOps management with repository connection,
// config synchronization, drift detection, rollback, and deployment history.
package gitopsmgr

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for GitOps management
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler creates a new GitOps manager HTTP handler
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes registers GitOps manager API routes
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	gitopsmgr := rg.Group("/gitopsmgr")
	{
		// Repositories
		gitopsmgr.GET("/repos", h.ListRepos)
		gitopsmgr.POST("/repos", h.ConnectRepo)
		gitopsmgr.GET("/repos/:id", h.GetRepo)
		gitopsmgr.DELETE("/repos/:id", h.DeleteRepo)

		// Sync
		gitopsmgr.POST("/sync", h.TriggerSync)

		// Drift
		gitopsmgr.GET("/drift/:repo_id", h.GetDrift)
		gitopsmgr.POST("/drift/:repo_id/detect", h.DetectDrift)
		gitopsmgr.POST("/drift/:repo_id/:drift_id/resolve", h.ResolveDrift)

		// Deployments
		gitopsmgr.GET("/deployments", h.ListDeployments)
		gitopsmgr.GET("/deployments/:id", h.GetDeployment)
		gitopsmgr.POST("/rollback", h.Rollback)

		// History
		gitopsmgr.GET("/history/:repo_id", h.GetHistory)
	}
}

// ListRepos handles GET /api/v1/gitopsmgr/repos
func (h *Handler) ListRepos(c *gin.Context) {
	repos := h.manager.ListRepos()
	c.JSON(http.StatusOK, gin.H{"repos": repos})
}

// ConnectRepo handles POST /api/v1/gitopsmgr/repos
func (h *Handler) ConnectRepo(c *gin.Context) {
	var repo GitRepo
	if err := c.ShouldBindJSON(&repo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.manager.ConnectRepo(&repo); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, repo)
}

// GetRepo handles GET /api/v1/gitopsmgr/repos/:id
func (h *Handler) GetRepo(c *gin.Context) {
	id := c.Param("id")
	repo := h.manager.GetRepo(id)
	if repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}
	c.JSON(http.StatusOK, repo)
}

// DeleteRepo handles DELETE /api/v1/gitopsmgr/repos/:id
func (h *Handler) DeleteRepo(c *gin.Context) {
	id := c.Param("id")
	if !h.manager.DeleteRepo(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repo not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "repo disconnected"})
}

// TriggerSync handles POST /api/v1/gitopsmgr/sync
func (h *Handler) TriggerSync(c *gin.Context) {
	var req SyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deployment, err := h.manager.SyncConfig(c.Request.Context(), req.RepoID, req.Force)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, deployment)
}

// GetDrift handles GET /api/v1/gitopsmgr/drift/:repo_id
func (h *Handler) GetDrift(c *gin.Context) {
	repoID := c.Param("repo_id")
	drifts, err := h.manager.DetectDrift(c.Request.Context(), repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"drifts": drifts, "total": len(drifts)})
}

// DetectDrift handles POST /api/v1/gitopsmgr/drift/:repo_id/detect
func (h *Handler) DetectDrift(c *gin.Context) {
	repoID := c.Param("repo_id")
	drifts, err := h.manager.DetectDrift(c.Request.Context(), repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "drift detection completed",
		"repo_id":      repoID,
		"drifts_found": len(drifts),
		"drifts":       drifts,
	})
}

// ResolveDrift handles POST /api/v1/gitopsmgr/drift/:repo_id/:drift_id/resolve
func (h *Handler) ResolveDrift(c *gin.Context) {
	repoID := c.Param("repo_id")
	driftID := c.Param("drift_id")

	if !h.manager.ResolveDrift(repoID, driftID) {
		c.JSON(http.StatusNotFound, gin.H{"error": "drift not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "drift resolved"})
}

// ListDeployments handles GET /api/v1/gitopsmgr/deployments
func (h *Handler) ListDeployments(c *gin.Context) {
	deployments := h.manager.ListDeployments()
	c.JSON(http.StatusOK, gin.H{"deployments": deployments})
}

// GetDeployment handles GET /api/v1/gitopsmgr/deployments/:id
func (h *Handler) GetDeployment(c *gin.Context) {
	id := c.Param("id")
	deployment := h.manager.GetDeployment(id)
	if deployment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		return
	}
	c.JSON(http.StatusOK, deployment)
}

// Rollback handles POST /api/v1/gitopsmgr/rollback
func (h *Handler) Rollback(c *gin.Context) {
	var req RollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deployment, err := h.manager.Rollback(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// GetHistory handles GET /api/v1/gitopsmgr/history/:repo_id
func (h *Handler) GetHistory(c *gin.Context) {
	repoID := c.Param("repo_id")
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	history := h.manager.GetHistory(repoID, limit)
	c.JSON(http.StatusOK, gin.H{
		"repo_id": repoID,
		"history": history,
		"total":   len(history),
	})
}
