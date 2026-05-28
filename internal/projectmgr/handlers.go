// Package projectmgr provides HTTP API handlers for project management.
package projectmgr

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for project management.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new project management HTTP handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers project management API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	projects := rg.Group("/projects")
	{
		// Project management
		projects.GET("", h.ListProjects)
		projects.POST("", h.CreateProject)
		projects.GET("/:id", h.GetProject)
		projects.PUT("/:id", h.UpdateProject)
		projects.DELETE("/:id", h.DeleteProject)

		// Member management
		projects.POST("/:id/members", h.AddMember)

		// Milestone management
		projects.POST("/:id/milestones", h.CreateMilestone)

		// Task management
		projects.GET("/:id/tasks", h.GetTasks)
		projects.POST("/:id/tasks", h.CreateTask)
		projects.PUT("/:id/tasks/:taskId", h.UpdateTask)

		// Time tracking
		projects.POST("/:id/timesheets", h.LogTime)

		// Comments
		projects.POST("/:id/tasks/:taskId/comments", h.AddComment)

		// Gantt chart
		projects.GET("/:id/gantt", h.GetGanttData)

		// Reports
		projects.GET("/:id/reports", h.GetProjectReport)
	}
}

// ListProjects handles GET /api/projects.
func (h *Handler) ListProjects(c *gin.Context) {
	projects := h.manager.ListProjects()
	c.JSON(http.StatusOK, gin.H{
		"projects": projects,
		"total":    len(projects),
	})
}

// CreateProject handles POST /api/projects.
func (h *Handler) CreateProject(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user ID from context
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	project, err := h.manager.CreateProject(req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, project)
}

// GetProject handles GET /api/projects/:id.
func (h *Handler) GetProject(c *gin.Context) {
	projectID := c.Param("id")

	project, err := h.manager.GetProject(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, project)
}

// UpdateProject handles PUT /api/projects/:id.
func (h *Handler) UpdateProject(c *gin.Context) {
	projectID := c.Param("id")

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	project, err := h.manager.UpdateProject(projectID, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, project)
}

// DeleteProject handles DELETE /api/projects/:id.
func (h *Handler) DeleteProject(c *gin.Context) {
	projectID := c.Param("id")

	if err := h.manager.DeleteProject(projectID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "project deleted"})
}

// AddMember handles POST /api/projects/:id/members.
func (h *Handler) AddMember(c *gin.Context) {
	projectID := c.Param("id")

	var req struct {
		UserID     string  `json:"user_id" binding:"required"`
		Username   string  `json:"username" binding:"required"`
		Role       string  `json:"role" binding:"required"`
		HourlyRate float64 `json:"hourly_rate"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	member, err := h.manager.AddMember(projectID, req.UserID, req.Username, req.Role, req.HourlyRate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, member)
}

// CreateMilestone handles POST /api/projects/:id/milestones.
func (h *Handler) CreateMilestone(c *gin.Context) {
	projectID := c.Param("id")

	var req CreateMilestoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	milestone, err := h.manager.CreateMilestone(projectID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, milestone)
}

// GetTasks handles GET /api/projects/:id/tasks.
func (h *Handler) GetTasks(c *gin.Context) {
	projectID := c.Param("id")

	tasks, err := h.manager.GetTasks(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"total": len(tasks),
	})
}

// CreateTask handles POST /api/projects/:id/tasks.
func (h *Handler) CreateTask(c *gin.Context) {
	projectID := c.Param("id")

	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user info from context
	userID := c.GetString("user_id")
	userName := c.GetString("username")
	if userID == "" {
		userID = "anonymous"
		userName = "Anonymous"
	}

	task, err := h.manager.CreateTask(projectID, req, userID, userName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// UpdateTask handles PUT /api/projects/:id/tasks/:taskId.
func (h *Handler) UpdateTask(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("taskId")

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.manager.UpdateTask(projectID, taskID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, task)
}

// LogTime handles POST /api/projects/:id/timesheets.
func (h *Handler) LogTime(c *gin.Context) {
	projectID := c.Param("id")

	var req LogTimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user info from context
	userID := c.GetString("user_id")
	userName := c.GetString("username")
	if userID == "" {
		userID = "anonymous"
		userName = "Anonymous"
	}

	timesheet, err := h.manager.LogTime(projectID, req, userID, userName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, timesheet)
}

// AddComment handles POST /api/projects/:id/tasks/:taskId/comments.
func (h *Handler) AddComment(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("taskId")

	var req AddCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user info from context
	userID := c.GetString("user_id")
	userName := c.GetString("username")
	if userID == "" {
		userID = "anonymous"
		userName = "Anonymous"
	}

	comment, err := h.manager.AddComment(projectID, taskID, userID, userName, req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, comment)
}

// GetGanttData handles GET /api/projects/:id/gantt.
func (h *Handler) GetGanttData(c *gin.Context) {
	projectID := c.Param("id")

	ganttTasks, err := h.manager.GetGanttData(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": ganttTasks,
		"total": len(ganttTasks),
	})
}

// GetProjectReport handles GET /api/projects/:id/reports.
func (h *Handler) GetProjectReport(c *gin.Context) {
	projectID := c.Param("id")

	report, err := h.manager.GetProjectReport(projectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}
