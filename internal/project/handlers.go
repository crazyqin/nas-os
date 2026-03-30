package project

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 项目管理 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{
		manager: mgr,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(router *gin.RouterGroup) {
	// ========== 项目管理 ==========
	projects := router.Group("/projects")
	{
		projects.GET("", h.listProjects)
		projects.POST("", h.createProject)
		projects.GET("/:id", h.getProject)
		projects.PUT("/:id", h.updateProject)
		projects.DELETE("/:id", h.deleteProject)
		projects.GET("/:id/stats", h.getProjectStats)

		// ========== 项目里程碑 ==========
		projects.GET("/:id/milestones", h.listMilestones)
		projects.POST("/:id/milestones", h.createMilestone)

		// ========== 项目任务 ==========
		projects.GET("/:id/tasks", h.listTasks)
		projects.POST("/:id/tasks", h.createTask)
	}

	// ========== 里程碑管理 ==========
	milestones := router.Group("/milestones")
	{
		milestones.GET("/:id", h.getMilestone)
		milestones.PUT("/:id", h.updateMilestone)
		milestones.DELETE("/:id", h.deleteMilestone)
	}

	// ========== 任务管理 ==========
	tasks := router.Group("/tasks")
	{
		tasks.GET("", h.listAllTasks)
		tasks.GET("/:id", h.getTask)
		tasks.PUT("/:id", h.updateTask)
		tasks.DELETE("/:id", h.deleteTask)
		tasks.GET("/:id/comments", h.getTaskComments)
		tasks.POST("/:id/comments", h.addTaskComment)
		tasks.GET("/:id/history", h.getTaskHistory)
	}

	// ========== 统计 ==========
	router.GET("/task-stats", h.getTaskStats)
}

// ========== 请求/响应结构 ==========

// CreateProjectRequest 创建项目请求.
type CreateProjectRequest struct {
	Name        string `json:"name" binding:"required"`
	Key         string `json:"key" binding:"required"`
	Description string `json:"description,omitempty"`
	OwnerID     string `json:"owner_id,omitempty"`
}

// UpdateProjectRequest 更新项目请求.
type UpdateProjectRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
}

// CreateMilestoneRequest 创建里程碑请求.
type CreateMilestoneRequest struct {
	Name        string     `json:"name" binding:"required"`
	Description string     `json:"description,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}

// UpdateMilestoneRequest 更新里程碑请求.
type UpdateMilestoneRequest struct {
	Name        string     `json:"name,omitempty"`
	Description string     `json:"description,omitempty"`
	Status      string     `json:"status,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}

// CreateTaskRequest 创建任务请求.
type CreateTaskRequest struct {
	Title          string       `json:"title" binding:"required"`
	Description    string       `json:"description,omitempty"`
	Priority       TaskPriority `json:"priority,omitempty"`
	AssigneeID     string       `json:"assignee_id,omitempty"`
	MilestoneID    string       `json:"milestone_id,omitempty"`
	ParentID       string       `json:"parent_id,omitempty"`
	Tags           []string     `json:"tags,omitempty"`
	Labels         []string     `json:"labels,omitempty"`
	DueDate        *time.Time   `json:"due_date,omitempty"`
	StartDate      *time.Time   `json:"start_date,omitempty"`
	EstimatedHours float64      `json:"estimated_hours,omitempty"`
}

// UpdateTaskRequest 更新任务请求.
type UpdateTaskRequest struct {
	Title          string       `json:"title,omitempty"`
	Description    string       `json:"description,omitempty"`
	Status         TaskStatus   `json:"status,omitempty"`
	Priority       TaskPriority `json:"priority,omitempty"`
	AssigneeID     string       `json:"assignee_id,omitempty"`
	MilestoneID    string       `json:"milestone_id,omitempty"`
	Tags           []string     `json:"tags,omitempty"`
	Labels         []string     `json:"labels,omitempty"`
	DueDate        *time.Time   `json:"due_date,omitempty"`
	StartDate      *time.Time   `json:"start_date,omitempty"`
	Progress       int          `json:"progress,omitempty"`
	EstimatedHours float64      `json:"estimated_hours,omitempty"`
	ActualHours    float64      `json:"actual_hours,omitempty"`
}

// AddCommentRequest 添加评论请求.
type AddCommentRequest struct {
	Content string `json:"content" binding:"required"`
}

// ========== 项目 API ==========

// listProjects 列出项目
// @Summary 列出项目
// @Description 获取项目列表
// @Tags projects
// @Accept json
// @Produce json
// @Param limit query int false "限制数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Param user_id query string false "用户ID筛选"
// @Success 200 {object} map[string]interface{}
// @Router /projects [get].
func (h *Handlers) listProjects(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	userID := c.Query("user_id")

	projects := h.manager.ListProjects(userID, limit, offset)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    projects,
	})
}

// createProject 创建项目
// @Summary 创建项目
// @Description 创建新项目
// @Tags projects
// @Accept json
// @Produce json
// @Param request body CreateProjectRequest true "项目创建参数"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Router /projects [post].
func (h *Handlers) createProject(c *gin.Context) {
	var req CreateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	// 从上下文获取用户信息
	userID, _ := c.Get("user_id")
	createdBy, _ := userID.(string)
	if createdBy == "" {
		createdBy = "system"
	}

	ownerID := req.OwnerID
	if ownerID == "" {
		ownerID = createdBy
	}

	project, err := h.manager.CreateProject(req.Name, req.Key, req.Description, ownerID, createdBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    project,
	})
}

// getProject 获取项目详情
// @Summary 获取项目详情
// @Description 根据ID获取项目详情
// @Tags projects
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /projects/{id} [get].
func (h *Handlers) getProject(c *gin.Context) {
	id := c.Param("id")

	project, err := h.manager.GetProject(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    project,
	})
}

// updateProject 更新项目
// @Summary 更新项目
// @Description 更新项目信息
// @Tags projects
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param request body UpdateProjectRequest true "项目更新参数"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /projects/{id} [put].
func (h *Handlers) updateProject(c *gin.Context) {
	id := c.Param("id")

	var req UpdateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	project, err := h.manager.UpdateProject(id, updates)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    project,
	})
}

// deleteProject 删除项目
// @Summary 删除项目
// @Description 删除项目及其关联数据
// @Tags projects
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /projects/{id} [delete].
func (h *Handlers) deleteProject(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteProject(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// getProjectStats 获取项目统计
// @Summary 获取项目统计
// @Description 获取项目任务统计信息
// @Tags projects
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Success 200 {object} map[string]interface{}
// @Router /projects/{id}/stats [get].
func (h *Handlers) getProjectStats(c *gin.Context) {
	id := c.Param("id")

	stats := h.manager.GetTaskStats(id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// ========== 里程碑 API ==========

// listMilestones 列出里程碑
// @Summary 列出里程碑
// @Description 获取项目的里程碑列表
// @Tags milestones
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Success 200 {object} map[string]interface{}
// @Router /projects/{id}/milestones [get].
func (h *Handlers) listMilestones(c *gin.Context) {
	projectID := c.Param("id")

	milestones := h.manager.ListMilestones(projectID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    milestones,
	})
}

// createMilestone 创建里程碑
// @Summary 创建里程碑
// @Description 为项目创建里程碑
// @Tags milestones
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param request body CreateMilestoneRequest true "里程碑创建参数"
// @Success 200 {object} map[string]interface{}
// @Router /projects/{id}/milestones [post].
func (h *Handlers) createMilestone(c *gin.Context) {
	projectID := c.Param("id")

	var req CreateMilestoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	createdBy, _ := userID.(string)
	if createdBy == "" {
		createdBy = "system"
	}

	milestone, err := h.manager.CreateMilestone(req.Name, req.Description, projectID, createdBy, req.DueDate)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    milestone,
	})
}

// getMilestone 获取里程碑详情
// @Summary 获取里程碑详情
// @Description 根据ID获取里程碑详情
// @Tags milestones
// @Accept json
// @Produce json
// @Param id path string true "里程碑ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /milestones/{id} [get].
func (h *Handlers) getMilestone(c *gin.Context) {
	id := c.Param("id")

	milestone, err := h.manager.GetMilestone(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    milestone,
	})
}

// updateMilestone 更新里程碑
// @Summary 更新里程碑
// @Description 更新里程碑信息
// @Tags milestones
// @Accept json
// @Produce json
// @Param id path string true "里程碑ID"
// @Param request body UpdateMilestoneRequest true "里程碑更新参数"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /milestones/{id} [put].
func (h *Handlers) updateMilestone(c *gin.Context) {
	id := c.Param("id")

	var req UpdateMilestoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.DueDate != nil {
		updates["due_date"] = req.DueDate
	}

	milestone, err := h.manager.UpdateMilestone(id, updates)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    milestone,
	})
}

// deleteMilestone 删除里程碑
// @Summary 删除里程碑
// @Description 删除里程碑
// @Tags milestones
// @Accept json
// @Produce json
// @Param id path string true "里程碑ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /milestones/{id} [delete].
func (h *Handlers) deleteMilestone(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteMilestone(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// ========== 任务 API ==========

// listTasks 列出项目任务
// @Summary 列出项目任务
// @Description 获取项目的任务列表
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param status query string false "状态筛选"
// @Param priority query string false "优先级筛选"
// @Param assignee_id query string false "负责人筛选"
// @Param limit query int false "限制数量" default(50)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /projects/{id}/tasks [get].
func (h *Handlers) listTasks(c *gin.Context) {
	projectID := c.Param("id")

	filter := TaskFilter{
		ProjectID: projectID,
	}

	if status := c.Query("status"); status != "" {
		filter.Status = []TaskStatus{TaskStatus(status)}
	}
	if priority := c.Query("priority"); priority != "" {
		filter.Priority = []TaskPriority{TaskPriority(priority)}
	}
	if assigneeID := c.Query("assignee_id"); assigneeID != "" {
		filter.AssigneeID = assigneeID
	}
	if milestoneID := c.Query("milestone_id"); milestoneID != "" {
		filter.MilestoneID = milestoneID
	}
	if search := c.Query("search"); search != "" {
		filter.Search = search
	}

	filter.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "50"))
	filter.Offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))
	filter.OrderBy = c.DefaultQuery("order_by", "created_at")
	filter.OrderDesc = c.DefaultQuery("order", "desc") == "desc"

	tasks := h.manager.ListTasks(filter)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tasks,
	})
}

// listAllTasks 列出所有任务
// @Summary 列出所有任务
// @Description 获取所有任务列表（可跨项目）
// @Tags tasks
// @Accept json
// @Produce json
// @Param project_id query string false "项目ID筛选"
// @Param status query string false "状态筛选"
// @Param priority query string false "优先级筛选"
// @Param assignee_id query string false "负责人筛选"
// @Param limit query int false "限制数量" default(50)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /tasks [get].
func (h *Handlers) listAllTasks(c *gin.Context) {
	filter := TaskFilter{}

	if projectID := c.Query("project_id"); projectID != "" {
		filter.ProjectID = projectID
	}
	if status := c.Query("status"); status != "" {
		filter.Status = []TaskStatus{TaskStatus(status)}
	}
	if priority := c.Query("priority"); priority != "" {
		filter.Priority = []TaskPriority{TaskPriority(priority)}
	}
	if assigneeID := c.Query("assignee_id"); assigneeID != "" {
		filter.AssigneeID = assigneeID
	}
	if reporterID := c.Query("reporter_id"); reporterID != "" {
		filter.ReporterID = reporterID
	}
	if milestoneID := c.Query("milestone_id"); milestoneID != "" {
		filter.MilestoneID = milestoneID
	}
	if search := c.Query("search"); search != "" {
		filter.Search = search
	}

	filter.Limit, _ = strconv.Atoi(c.DefaultQuery("limit", "50"))
	filter.Offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))
	filter.OrderBy = c.DefaultQuery("order_by", "created_at")
	filter.OrderDesc = c.DefaultQuery("order", "desc") == "desc"

	tasks := h.manager.ListTasks(filter)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tasks,
	})
}

// createTask 创建任务
// @Summary 创建任务
// @Description 为项目创建任务
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "项目ID"
// @Param request body CreateTaskRequest true "任务创建参数"
// @Success 200 {object} map[string]interface{}
// @Router /projects/{id}/tasks [post].
func (h *Handlers) createTask(c *gin.Context) {
	projectID := c.Param("id")

	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	reporterID, _ := userID.(string)
	if reporterID == "" {
		reporterID = "system"
	}

	priority := req.Priority
	if priority == "" {
		priority = PriorityMedium
	}

	task, err := h.manager.CreateTask(req.Title, req.Description, projectID, reporterID, priority)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	// 设置额外字段
	if req.AssigneeID != "" || req.MilestoneID != "" || req.DueDate != nil || len(req.Tags) > 0 {
		updates := make(map[string]interface{})
		if req.AssigneeID != "" {
			updates["assignee_id"] = req.AssigneeID
		}
		if req.MilestoneID != "" {
			updates["milestone_id"] = req.MilestoneID
		}
		if req.DueDate != nil {
			updates["due_date"] = req.DueDate
		}
		if len(req.Tags) > 0 {
			updates["tags"] = req.Tags
		}
		if len(req.Labels) > 0 {
			updates["labels"] = req.Labels
		}
		if req.EstimatedHours > 0 {
			updates["estimated_hours"] = req.EstimatedHours
		}
		task, _ = h.manager.UpdateTask(task.ID, reporterID, updates)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    task,
	})
}

// getTask 获取任务详情
// @Summary 获取任务详情
// @Description 根据ID获取任务详情
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /tasks/{id} [get].
func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")

	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    task,
	})
}

// updateTask 更新任务
// @Summary 更新任务
// @Description 更新任务信息
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param request body UpdateTaskRequest true "任务更新参数"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /tasks/{id} [put].
func (h *Handlers) updateTask(c *gin.Context) {
	id := c.Param("id")

	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	modifierID, _ := userID.(string)
	if modifierID == "" {
		modifierID = "system"
	}

	updates := make(map[string]interface{})
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Priority != "" {
		updates["priority"] = req.Priority
	}
	if req.AssigneeID != "" {
		updates["assignee_id"] = req.AssigneeID
	}
	if req.MilestoneID != "" {
		updates["milestone_id"] = req.MilestoneID
	}
	if req.DueDate != nil {
		updates["due_date"] = req.DueDate
	}
	if req.Progress >= 0 && req.Progress <= 100 {
		updates["progress"] = req.Progress
	}
	if req.EstimatedHours > 0 {
		updates["estimated_hours"] = req.EstimatedHours
	}
	if req.ActualHours > 0 {
		updates["actual_hours"] = req.ActualHours
	}
	if len(req.Tags) > 0 {
		updates["tags"] = req.Tags
	}
	if len(req.Labels) > 0 {
		updates["labels"] = req.Labels
	}

	task, err := h.manager.UpdateTask(id, modifierID, updates)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    task,
	})
}

// deleteTask 删除任务
// @Summary 删除任务
// @Description 删除任务
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /tasks/{id} [delete].
func (h *Handlers) deleteTask(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteTask(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// ========== 评论 API ==========

// getTaskComments 获取任务评论
// @Summary 获取任务评论
// @Description 获取任务的所有评论
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{}
// @Router /tasks/{id}/comments [get].
func (h *Handlers) getTaskComments(c *gin.Context) {
	taskID := c.Param("id")

	comments := h.manager.GetComments(taskID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    comments,
	})
}

// addTaskComment 添加任务评论
// @Summary 添加任务评论
// @Description 为任务添加评论
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Param request body AddCommentRequest true "评论内容"
// @Success 200 {object} map[string]interface{}
// @Router /tasks/{id}/comments [post].
func (h *Handlers) addTaskComment(c *gin.Context) {
	taskID := c.Param("id")

	var req AddCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	userID, _ := c.Get("user_id")
	commenterID, _ := userID.(string)
	if commenterID == "" {
		commenterID = "system"
	}

	comment, err := h.manager.AddComment(taskID, commenterID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    comment,
	})
}

// ========== 历史 API ==========

// getTaskHistory 获取任务历史
// @Summary 获取任务历史
// @Description 获取任务的变更历史
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "任务ID"
// @Success 200 {object} map[string]interface{}
// @Router /tasks/{id}/history [get].
func (h *Handlers) getTaskHistory(c *gin.Context) {
	taskID := c.Param("id")

	history := h.manager.GetHistory(taskID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    history,
	})
}

// ========== 统计 API ==========

// getTaskStats 获取任务统计
// @Summary 获取任务统计
// @Description 获取全局任务统计信息
// @Tags tasks
// @Accept json
// @Produce json
// @Param project_id query string false "项目ID筛选"
// @Success 200 {object} map[string]interface{}
// @Router /task-stats [get].
func (h *Handlers) getTaskStats(c *gin.Context) {
	projectID := c.Query("project_id")

	stats := h.manager.GetTaskStats(projectID)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}
