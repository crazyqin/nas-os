// Package kanban 提供看板 REST API 处理器
package kanban

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler 看板 API 处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	kanban := r.Group("/kanban")
	{
		// 看板管理
		kanban.GET("/boards", h.listBoards)
		kanban.POST("/boards", h.createBoard)
		kanban.GET("/boards/:id", h.getBoard)
		kanban.DELETE("/boards/:id", h.deleteBoard)
		kanban.GET("/boards/:id/progress", h.getBoardProgress)

		// 卡片管理
		kanban.POST("/boards/:id/cards", h.addCard)
		kanban.PUT("/boards/:id/cards/:cardId/move", h.moveCard)

		// 标签管理
		kanban.POST("/boards/:id/labels", h.addLabel)
		kanban.DELETE("/boards/:id/labels/:labelId", h.deleteLabel)

		// 成员管理
		kanban.POST("/boards/:id/members", h.assignMember)
		kanban.DELETE("/boards/:id/members/:userId", h.removeMember)

		// 活动记录
		kanban.GET("/activity", h.getActivity)
	}
}

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (h *Handler) listBoards(c *gin.Context) {
	boards := h.manager.ListBoards()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: boards})
}

func (h *Handler) createBoard(c *gin.Context) {
	var req CreateBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	board, err := h.manager.CreateBoard(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "board created", Data: board})
}

func (h *Handler) getBoard(c *gin.Context) {
	id := c.Param("id")
	board, err := h.manager.GetBoard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: board})
}

func (h *Handler) deleteBoard(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteBoard(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "board deleted"})
}

func (h *Handler) getBoardProgress(c *gin.Context) {
	id := c.Param("id")
	progress, err := h.manager.GetBoardProgress(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: progress})
}

func (h *Handler) addCard(c *gin.Context) {
	boardID := c.Param("id")
	var req AddCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	card, err := h.manager.AddCard(boardID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "card created", Data: card})
}

func (h *Handler) moveCard(c *gin.Context) {
	boardID := c.Param("id")
	cardID := c.Param("cardId")
	var req MoveCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	card, err := h.manager.MoveCard(boardID, cardID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "card moved", Data: card})
}

func (h *Handler) addLabel(c *gin.Context) {
	boardID := c.Param("id")
	var req AddLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	label, err := h.manager.AddLabel(boardID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "label added", Data: label})
}

func (h *Handler) deleteLabel(c *gin.Context) {
	boardID := c.Param("id")
	labelID := c.Param("labelId")
	if err := h.manager.DeleteLabel(boardID, labelID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "label deleted"})
}

func (h *Handler) assignMember(c *gin.Context) {
	boardID := c.Param("id")
	var req AssignMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	member, err := h.manager.AssignMember(boardID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "member assigned", Data: member})
}

func (h *Handler) removeMember(c *gin.Context) {
	boardID := c.Param("id")
	userID := c.Param("userId")
	if err := h.manager.RemoveMember(boardID, userID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "member removed"})
}

func (h *Handler) getActivity(c *gin.Context) {
	boardID := c.Query("board_id")
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	activities := h.manager.GetActivity(boardID, limit)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: activities})
}
