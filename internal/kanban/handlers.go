// Package kanban provides HTTP API handlers for Kanban board management.
package kanban

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler provides HTTP handlers for Kanban management.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new Kanban HTTP handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers Kanban API routes.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	kanban := rg.Group("/kanban")
	{
		// Board management
		kanban.GET("/boards", h.ListBoards)
		kanban.POST("/boards", h.CreateBoard)
		kanban.GET("/boards/:id", h.GetBoard)
		kanban.PUT("/boards/:id", h.UpdateBoard)
		kanban.DELETE("/boards/:id", h.DeleteBoard)

		// Card management
		kanban.POST("/cards", h.CreateCard)
		kanban.PUT("/cards/:id/move", h.MoveCard)

		// Comment management
		kanban.POST("/cards/:id/comments", h.AddComment)

		// Tag management
		kanban.POST("/boards/:id/tags", h.AddTag)

		// Member management
		kanban.POST("/boards/:id/members", h.AddMember)

		// Templates
		kanban.GET("/templates", h.ListTemplates)
	}
}

// ListBoards handles GET /api/kanban/boards.
func (h *Handler) ListBoards(c *gin.Context) {
	boards := h.manager.ListBoards()
	c.JSON(http.StatusOK, gin.H{
		"boards": boards,
		"total":  len(boards),
	})
}

// CreateBoard handles POST /api/kanban/boards.
func (h *Handler) CreateBoard(c *gin.Context) {
	var req CreateBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user ID from context (set by auth middleware)
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	board, err := h.manager.CreateBoard(req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, board)
}

// GetBoard handles GET /api/kanban/boards/:id.
func (h *Handler) GetBoard(c *gin.Context) {
	boardID := c.Param("id")

	board, err := h.manager.GetBoard(boardID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, board)
}

// UpdateBoard handles PUT /api/kanban/boards/:id.
func (h *Handler) UpdateBoard(c *gin.Context) {
	boardID := c.Param("id")

	var req UpdateBoardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	board, err := h.manager.UpdateBoard(boardID, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, board)
}

// DeleteBoard handles DELETE /api/kanban/boards/:id.
func (h *Handler) DeleteBoard(c *gin.Context) {
	boardID := c.Param("id")

	if err := h.manager.DeleteBoard(boardID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "board deleted"})
}

// CreateCard handles POST /api/kanban/cards.
func (h *Handler) CreateCard(c *gin.Context) {
	boardID := c.Query("board_id")
	if boardID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "board_id is required"})
		return
	}

	var req CreateCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user ID from context
	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	card, err := h.manager.CreateCard(boardID, req, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, card)
}

// MoveCard handles PUT /api/kanban/cards/:id/move.
func (h *Handler) MoveCard(c *gin.Context) {
	cardID := c.Param("id")
	boardID := c.Query("board_id")
	if boardID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "board_id is required"})
		return
	}

	var req MoveCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	card, err := h.manager.MoveCard(boardID, cardID, req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, card)
}

// AddComment handles POST /api/kanban/cards/:id/comments.
func (h *Handler) AddComment(c *gin.Context) {
	cardID := c.Param("id")
	boardID := c.Query("board_id")
	if boardID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "board_id is required"})
		return
	}

	var req AddCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user info from context
	userID := c.GetString("user_id")
	username := c.GetString("username")
	if userID == "" {
		userID = "anonymous"
		username = "Anonymous"
	}

	comment, err := h.manager.AddComment(boardID, cardID, userID, username, req.Content)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, comment)
}

// AddTag handles POST /api/kanban/boards/:id/tags.
func (h *Handler) AddTag(c *gin.Context) {
	boardID := c.Param("id")

	var req struct {
		Name  string `json:"name" binding:"required"`
		Color string `json:"color" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tag, err := h.manager.AddTag(boardID, req.Name, req.Color)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, tag)
}

// AddMember handles POST /api/kanban/boards/:id/members.
func (h *Handler) AddMember(c *gin.Context) {
	boardID := c.Param("id")

	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		Username string `json:"username" binding:"required"`
		Role     string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	member, err := h.manager.AddMember(boardID, req.UserID, req.Username, req.Role)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, member)
}

// ListTemplates handles GET /api/kanban/templates.
func (h *Handler) ListTemplates(c *gin.Context) {
	templates := h.manager.GetTemplates()
	c.JSON(http.StatusOK, gin.H{
		"templates": templates,
		"total":     len(templates),
	})
}
