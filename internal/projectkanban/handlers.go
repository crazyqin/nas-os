package projectkanban

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
	kanban := r.Group("/projectkanban")
	{
		// 看板管理
		kanban.GET("/boards", h.listBoards)
		kanban.POST("/boards", h.createBoard)
		kanban.GET("/boards/:id", h.getBoard)
		kanban.PUT("/boards/:id/archive", h.archiveBoard)

		// 卡片管理
		kanban.POST("/boards/:id/cards", h.createCard)
		kanban.PUT("/boards/:id/cards/:cardId", h.updateCard)
		kanban.POST("/boards/:id/cards/:cardId/move", h.moveCard)

		// 统计
		kanban.GET("/boards/:id/stats", h.getBoardStats)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listBoards 获取看板列表
func (h *Handlers) listBoards(c *gin.Context) {
	boards := h.manager.ListBoards()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    boards,
	})
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

	board := h.manager.CreateBoard(&req)
	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "board created",
		Data:    board,
	})
}

// getBoard 获取看板详情
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

// archiveBoard 归档看板
func (h *Handlers) archiveBoard(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.ArchiveBoard(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "board archived",
	})
}

// createCard 创建卡片
func (h *Handlers) createCard(c *gin.Context) {
	boardID := c.Param("id")

	var req CreateCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	card, err := h.manager.CreateCard(boardID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "card created",
		Data:    card,
	})
}

// updateCard 更新卡片
func (h *Handlers) updateCard(c *gin.Context) {
	boardID := c.Param("id")
	cardID := c.Param("cardId")

	var req UpdateCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	card, err := h.manager.UpdateCard(boardID, cardID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "card updated",
		Data:    card,
	})
}

// moveCard 移动卡片
func (h *Handlers) moveCard(c *gin.Context) {
	boardID := c.Param("id")
	cardID := c.Param("cardId")

	var req MoveCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.MoveCard(boardID, cardID, &req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "card moved",
	})
}

// getBoardStats 获取看板统计
func (h *Handlers) getBoardStats(c *gin.Context) {
	id := c.Param("id")
	stats, err := h.manager.GetBoardStats(id)
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
		Data:    stats,
	})
}
