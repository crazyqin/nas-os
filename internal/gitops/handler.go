// Package gitops provides Git-based infrastructure management and deployment automation.
package gitops

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler provides HTTP handlers for GitOps
type Handler struct {
	engine     *Engine
	reconciler *Reconciler
	logger     *zap.Logger
}

// NewHandler creates a new GitOps HTTP handler
func NewHandler(engine *Engine, reconciler *Reconciler, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		engine:     engine,
		reconciler: reconciler,
		logger:     logger,
	}
}

// RegisterRoutes registers GitOps API routes
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	gitops := rg.Group("/gitops")
	{
		// Repositories
		gitops.GET("/repos", h.ListRepos)
		gitops.GET("/repos/:id", h.GetRepo)
		gitops.POST("/repos", h.AddRepo)

		// Sync
		gitops.POST("/sync", h.TriggerSync)
		gitops.GET("/sync/:repo_id/:env", h.GetSyncStatus)

		// Deployments
		gitops.GET("/deployments", h.ListDeployments)
		gitops.GET("/deployments/:id", h.GetDeployment)
		gitops.POST("/rollback", h.Rollback)

		// Drift
		gitops.GET("/drift", h.GetDriftSummary)
		gitops.GET("/drift/:repo_id/:env", h.GetDriftDetails)
		gitops.POST("/drift/detect", h.DetectDrift)

		// Reconcile
		gitops.POST("/reconcile", h.TriggerReconcile)
	}
}

// ListRepos handles GET /api/v1/gitops/repos
func (h *Handler) ListRepos(c *gin.Context) {
	repos := h.engine.ListRepos()
	c.JSON(http.StatusOK, gin.H{"repos": repos})
}

// GetRepo handles GET /api/v1/gitops/repos/:id
func (h *Handler) GetRepo(c *gin.Context) {
	id := c.Param("id")
	repo := h.engine.GetRepo(id)
	if repo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repository not found"})
		return
	}
	c.JSON(http.StatusOK, repo)
}

// TriggerSync handles POST /api/v1/gitops/sync
func (h *Handler) TriggerSync(c *gin.Context) {
	var req SyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	go func() {
		if err := h.engine.SyncRepo(c.Request.Context(), req.RepoID); err != nil {
			h.logger.Error("sync failed",
				zap.String("repo", req.RepoID),
				zap.Error(err))
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message": "sync triggered",
		"repo_id": req.RepoID,
	})
}

// GetSyncStatus handles GET /api/v1/gitops/sync/:repo_id/:env
func (h *Handler) GetSyncStatus(c *gin.Context) {
	repoID := c.Param("repo_id")
	env := Environment(c.Param("env"))

	status := h.engine.GetSyncStatus(repoID, env)
	if status == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sync status not found"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// ListDeployments handles GET /api/v1/gitops/deployments
func (h *Handler) ListDeployments(c *gin.Context) {
	repoID := c.Query("repo_id")
	env := Environment(c.Query("env"))

	if repoID == "" || env == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo_id and env query params required"})
		return
	}

	deployments := h.engine.ListDeployments(repoID, env)
	c.JSON(http.StatusOK, gin.H{"deployments": deployments})
}

// GetDeployment handles GET /api/v1/gitops/deployments/:id
func (h *Handler) GetDeployment(c *gin.Context) {
	id := c.Param("id")
	deployment := h.engine.GetDeployment(id)
	if deployment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "deployment not found"})
		return
	}
	c.JSON(http.StatusOK, deployment)
}

// Rollback handles POST /api/v1/gitops/rollback
func (h *Handler) Rollback(c *gin.Context) {
	var req RollbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deployment, err := h.engine.Rollback(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, deployment)
}

// GetDriftSummary handles GET /api/v1/gitops/drift
func (h *Handler) GetDriftSummary(c *gin.Context) {
	summary := h.reconciler.GetDriftSummary()
	c.JSON(http.StatusOK, gin.H{"drift_summary": summary})
}

// GetDriftDetails handles GET /api/v1/gitops/drift/:repo_id/:env
func (h *Handler) GetDriftDetails(c *gin.Context) {
	repoID := c.Param("repo_id")
	env := Environment(c.Param("env"))

	status := h.engine.GetSyncStatus(repoID, env)
	if status == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "sync status not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"repo_id":       repoID,
		"env":           env,
		"drift_detected": status.DriftDetected,
		"drift_details":  status.DriftDetails,
		"last_sync_at":   status.LastSyncAt,
	})
}

// TriggerReconcile handles POST /api/v1/gitops/reconcile
func (h *Handler) TriggerReconcile(c *gin.Context) {
	go func() {
		h.reconciler.ReconcileAll(c.Request.Context())
	}()

	c.JSON(http.StatusAccepted, gin.H{"message": "reconciliation triggered"})
}

// AddRepo handles POST /api/v1/gitops/repos
func (h *Handler) AddRepo(c *gin.Context) {
	var req AddRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	repo, err := h.engine.AddRepo(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, repo)
}

// DetectDrift handles POST /api/v1/gitops/drift/detect
func (h *Handler) DetectDrift(c *gin.Context) {
	var req DriftDetectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	detection, err := h.engine.DetectDrift(req.RepoID, req.Environment)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, detection)
}
