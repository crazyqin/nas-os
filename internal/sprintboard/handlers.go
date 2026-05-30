// Package sprintboard 提供 REST API 处理器
package sprintboard

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 看板 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	board := r.Group("/sprintboard")
	{
		// 看板管理
		board.GET("/boards", h.listBoards)
		board.POST("/boards", h.createBoard)
		board.GET("/boards/:id", h.getBoard)
		board.DELETE("/boards/:id", h.deleteBoard)

		// 泳道管理
		board.POST("/boards/:id/swimlanes", h.addSwimLane)
		board.DELETE("/boards/:id/swimlanes/:lane_id", h.removeSwimLane)

		// Sprint 管理
		board.GET("/sprints", h.listSprints)
		board.POST("/sprints", h.createSprint)
		board.GET("/sprints/:id", h.getSprint)
		board.POST("/sprints/:id/start", h.startSprint)
		board.POST("/sprints/:id/complete", h.completeSprint)

		// 任务管理
		board.GET("/tasks", h.listTasks)
		board.POST("/tasks", h.addTask)
		board.GET("/tasks/:id", h.getTask)
		board.PUT("/tasks/:id", h.updateTask)
		board.DELETE("/tasks/:id", h.deleteTask)
		board.POST("/tasks/:id/move", h.moveTask)

		// 指标和报告
		board.GET("/sprints/:id/metrics", h.getMetrics)
		board.GET("/sprints/:id/burndown", h.getBurndown)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// createBoard 创建看板
func (h *Handlers) createBoard(c *gin.Context) {
	var req CreateBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	board, err := h.manager.CreateBoard(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "board created",
		Data:    board,
	})
}

// listBoards 列出看板
func (h *Handlers) listBoards(c *gin.Context) {
	boards := h.manager.ListBoards()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    boards,
	})
}

// getBoard 获取看板
func (h *Handlers) getBoard(c *gin.Context) {
	id := c.Param("id")
	board, err := h.manager.GetBoard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    board,
	})
}

// deleteBoard 删除看板
func (h *Handlers) deleteBoard(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteBoard(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "board deleted",
	})
}

// addSwimLane 添加泳道
func (h *Handlers) addSwimLane(c *gin.Context) {
	boardID := c.Param("id")
	var req CreateSwimLaneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	lane, err := h.manager.AddSwimLane(boardID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "swim lane added",
		Data:    lane,
	})
}

// removeSwimLane 移除泳道
func (h *Handlers) removeSwimLane(c *gin.Context) {
	boardID := c.Param("id")
	laneID := c.Param("lane_id")

	if err := h.manager.RemoveSwimLane(boardID, laneID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "swim lane removed",
	})
}

// createSprint 创建 Sprint
func (h *Handlers) createSprint(c *gin.Context) {
	var req CreateSprintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	sprint, err := h.manager.CreateSprint(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "sprint created",
		Data:    sprint,
	})
}

// listSprints 列出 Sprint
func (h *Handlers) listSprints(c *gin.Context) {
	boardID := c.Query("board_id")
	sprints := h.manager.ListSprints(boardID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    sprints,
	})
}

// getSprint 获取 Sprint
func (h *Handlers) getSprint(c *gin.Context) {
	id := c.Param("id")
	sprint, err := h.manager.GetSprint(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    sprint,
	})
}

// startSprint 启动 Sprint
func (h *Handlers) startSprint(c *gin.Context) {
	id := c.Param("id")
	sprint, err := h.manager.StartSprint(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "sprint started",
		Data:    sprint,
	})
}

// completeSprint 完成 Sprint
func (h *Handlers) completeSprint(c *gin.Context) {
	id := c.Param("id")
	sprint, err := h.manager.CompleteSprint(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "sprint completed",
		Data:    sprint,
	})
}

// addTask 添加任务
func (h *Handlers) addTask(c *gin.Context) {
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	task, err := h.manager.AddTask(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "task created",
		Data:    task,
	})
}

// listTasks 列出任务
func (h *Handlers) listTasks(c *gin.Context) {
	boardID := c.Query("board_id")
	sprintID := c.Query("sprint_id")
	tasks := h.manager.ListTasks(boardID, sprintID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    tasks,
	})
}

// getTask 获取任务
func (h *Handlers) getTask(c *gin.Context) {
	id := c.Param("id")
	task, err := h.manager.GetTask(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    task,
	})
}

// updateTask 更新任务
func (h *Handlers) updateTask(c *gin.Context) {
	id := c.Param("id")
	var req CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	task, err := h.manager.UpdateTask(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "task updated",
		Data:    task,
	})
}

// deleteTask 删除任务
func (h *Handlers) deleteTask(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteTask(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "task deleted",
	})
}

// moveTask 移动任务
func (h *Handlers) moveTask(c *gin.Context) {
	id := c.Param("id")
	var req MoveTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	task, err := h.manager.MoveTask(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "task moved",
		Data:    task,
	})
}

// getMetrics 获取 Sprint 指标
func (h *Handlers) getMetrics(c *gin.Context) {
	id := c.Param("id")
	metrics, err := h.manager.GetMetrics(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    metrics,
	})
}

// getBurndown 获取燃尽图数据
func (h *Handlers) getBurndown(c *gin.Context) {
	id := c.Param("id")
	burndown, err := h.manager.GenerateBurndown(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    burndown,
	})
}
