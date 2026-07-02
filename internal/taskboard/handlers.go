package taskboard

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 任务看板 HTTP 处理器.
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/taskboard")
	{
		// 看板管理
		g.POST("/boards", h.CreateBoard)
		g.GET("/boards", h.ListBoards)
		g.GET("/boards/:id", h.GetBoard)
		g.DELETE("/boards/:id", h.DeleteBoard)
		g.GET("/boards/:id/stats", h.GetBoardStats)

		// 任务CRUD
		g.POST("/boards/:boardId/tasks", h.CreateTask)
		g.GET("/boards/:boardId/tasks", h.ListTasks)
		g.GET("/tasks/:id", h.GetTask)
		g.PUT("/tasks/:id", h.UpdateTask)
		g.DELETE("/tasks/:id", h.DeleteTask)
		g.PUT("/tasks/:id/move", h.MoveTask)
		g.PUT("/tasks/:id/progress", h.UpdateProgress)

		// 标签管理
		g.POST("/boards/:boardId/labels", h.CreateLabel)
		g.GET("/boards/:boardId/labels", h.ListLabels)
		g.DELETE("/labels/:id", h.DeleteLabel)
		g.POST("/tasks/:taskId/labels/:labelId", h.AddLabelToTask)
		g.DELETE("/tasks/:taskId/labels/:labelId", h.RemoveLabelFromTask)
	}
}

// CreateBoard 创建看板.
func (h *Handlers) CreateBoard(c *gin.Context) {
	var req CreateBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "参数错误: " + err.Error()})
		return
	}

	createdBy := c.GetString("user_id")
	board, err := h.mgr.CreateBoard(req.Name, req.Description, req.OwnerID, createdBy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 0, "data": board})
}

// GetBoard 获取看板.
func (h *Handlers) GetBoard(c *gin.Context) {
	id := c.Param("id")
	board, err := h.mgr.GetBoard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": board})
}

// DeleteBoard 删除看板.
func (h *Handlers) DeleteBoard(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeleteBoard(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "看板已删除"})
}

// ListBoards 列出看板.
func (h *Handlers) ListBoards(c *gin.Context) {
	ownerID := c.Query("owner_id")
	boards := h.mgr.ListBoards(ownerID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": boards})
}

// CreateTask 创建任务.
func (h *Handlers) CreateTask(c *gin.Context) {
	boardID := c.Param("boardId")
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "参数错误: " + err.Error()})
		return
	}

	createdBy := c.GetString("user_id")
	task, err := h.mgr.CreateTask(boardID, req.Title, req.Description, createdBy, req.Priority)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}

	// 设置可选字段
	if req.AssigneeID != "" {
		task.AssigneeID = req.AssigneeID
	}
	if req.DueDate != nil {
		task.DueDate = req.DueDate
	}
	if req.Labels != nil {
		task.Labels = req.Labels
	}

	c.JSON(http.StatusCreated, gin.H{"code": 0, "data": task})
}

// GetTask 获取任务.
func (h *Handlers) GetTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.mgr.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

// UpdateTask 更新任务.
func (h *Handlers) UpdateTask(c *gin.Context) {
	id := c.Param("id")
	var req UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "参数错误: " + err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Title != nil {
		updates["title"] = *req.Title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Priority != nil {
		updates["priority"] = *req.Priority
	}
	if req.AssigneeID != nil {
		updates["assignee_id"] = *req.AssigneeID
	}
	if req.DueDate != nil {
		updates["due_date"] = req.DueDate
	}
	if req.Labels != nil {
		updates["labels"] = req.Labels
	}

	task, err := h.mgr.UpdateTask(id, updates)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

// DeleteTask 删除任务.
func (h *Handlers) DeleteTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeleteTask(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "任务已删除"})
}

// ListTasks 列出任务.
func (h *Handlers) ListTasks(c *gin.Context) {
	boardID := c.Param("boardId")

	filter := TaskFilter{
		Limit: 50, // 默认限制
	}

	// 解析过滤参数
	if status := c.QueryArray("status"); len(status) > 0 {
		for _, s := range status {
			filter.Status = append(filter.Status, TaskStatus(s))
		}
	}
	if priority := c.QueryArray("priority"); len(priority) > 0 {
		for _, p := range priority {
			filter.Priority = append(filter.Priority, TaskPriority(p))
		}
	}
	filter.AssigneeID = c.Query("assignee_id")
	filter.Search = c.Query("search")
	filter.OrderBy = c.Query("order_by")
	filter.OrderDesc = c.Query("order_desc") == "true"

	if limit, err := strconv.Atoi(c.Query("limit")); err == nil && limit > 0 {
		filter.Limit = limit
	}
	if offset, err := strconv.Atoi(c.Query("offset")); err == nil && offset >= 0 {
		filter.Offset = offset
	}

	tasks := h.mgr.ListTasks(boardID, filter)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": tasks})
}

// MoveTask 移动任务状态.
func (h *Handlers) MoveTask(c *gin.Context) {
	id := c.Param("id")
	var req MoveTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "参数错误: " + err.Error()})
		return
	}

	task, err := h.mgr.TransitionTask(id, req.Status)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": task})
}

// UpdateProgress 更新任务进度.
func (h *Handlers) UpdateProgress(c *gin.Context) {
	id := c.Param("id")
	var req UpdateProgressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "参数错误: " + err.Error()})
		return
	}

	if err := h.mgr.UpdateProgress(id, req.Progress); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "进度已更新"})
}

// GetBoardStats 获取看板统计.
func (h *Handlers) GetBoardStats(c *gin.Context) {
	boardID := c.Param("id")
	stats, err := h.mgr.GetBoardStats(boardID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}

// CreateLabel 创建标签.
func (h *Handlers) CreateLabel(c *gin.Context) {
	boardID := c.Param("boardId")
	var req CreateLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "参数错误: " + err.Error()})
		return
	}

	label, err := h.mgr.CreateLabel(boardID, req.Name, req.Color)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 0, "data": label})
}

// ListLabels 列出标签.
func (h *Handlers) ListLabels(c *gin.Context) {
	boardID := c.Param("boardId")
	labels := h.mgr.ListLabels(boardID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": labels})
}

// DeleteLabel 删除标签.
func (h *Handlers) DeleteLabel(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeleteLabel(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "标签已删除"})
}

// AddLabelToTask 给任务添加标签.
func (h *Handlers) AddLabelToTask(c *gin.Context) {
	taskID := c.Param("taskId")
	labelID := c.Param("labelId")

	if err := h.mgr.AddLabelToTask(taskID, labelID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "标签已添加"})
}

// RemoveLabelFromTask 从任务移除标签.
func (h *Handlers) RemoveLabelFromTask(c *gin.Context) {
	taskID := c.Param("taskId")
	labelID := c.Param("labelId")

	if err := h.mgr.RemoveLabelFromTask(taskID, labelID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "标签已移除"})
}
