// Package notestation 提供 REST API 处理器
package notestation

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 笔记模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/notestation 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	ns := r.Group("/notestation")
	{
		// 笔记 CRUD
		ns.POST("/notes", h.createNote)
		ns.GET("/notes", h.listNotes)
		ns.GET("/notes/:id", h.getNote)
		ns.PUT("/notes/:id", h.updateNote)
		ns.DELETE("/notes/:id", h.deleteNote)

		// 笔记本 CRUD
		ns.POST("/notebooks", h.createNotebook)
		ns.GET("/notebooks", h.listNotebooks)
		ns.GET("/notebooks/:id", h.getNotebook)
		ns.PUT("/notebooks/:id", h.updateNotebook)
		ns.DELETE("/notebooks/:id", h.deleteNotebook)

		// 按笔记本列出笔记
		ns.GET("/notebooks/:id/notes", h.listNotesByNotebook)

		// 搜索
		ns.GET("/search", h.searchNotes)

		// 标签
		ns.GET("/tags", h.getTagStats)
		ns.GET("/tags/:tag", h.listNotesByTag)

		// 置顶 / 收藏
		ns.GET("/pinned", h.getPinnedNotes)
		ns.GET("/favorites", h.getFavoriteNotes)

		// 导入 / 导出
		ns.POST("/import", h.importMarkdown)
		ns.POST("/export", h.exportNotes)

		// 最近编辑
		ns.GET("/recent", h.getRecentNotes)
	}
}

// ========== Note Handlers ==========

func (h *Handlers) createNote(c *gin.Context) {
	var req CreateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	note := h.manager.CreateNote(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: note})
}

func (h *Handlers) getNote(c *gin.Context) {
	id := c.Param("id")
	note, err := h.manager.GetNote(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: note})
}

func (h *Handlers) listNotes(c *gin.Context) {
	notes := h.manager.ListNotes()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(notes),
			"notes": notes,
		},
	})
}

func (h *Handlers) updateNote(c *gin.Context) {
	id := c.Param("id")
	var req UpdateNoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	note, err := h.manager.UpdateNote(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: note})
}

func (h *Handlers) deleteNote(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteNote(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== Notebook Handlers ==========

func (h *Handlers) createNotebook(c *gin.Context) {
	var req CreateNotebookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	nb := h.manager.CreateNotebook(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: nb})
}

func (h *Handlers) getNotebook(c *gin.Context) {
	id := c.Param("id")
	nb, err := h.manager.GetNotebook(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: nb})
}

func (h *Handlers) listNotebooks(c *gin.Context) {
	nbs := h.manager.ListNotebooks()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     len(nbs),
			"notebooks": nbs,
		},
	})
}

func (h *Handlers) updateNotebook(c *gin.Context) {
	id := c.Param("id")
	var req UpdateNotebookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	nb, err := h.manager.UpdateNotebook(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: nb})
}

func (h *Handlers) deleteNotebook(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteNotebook(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

func (h *Handlers) listNotesByNotebook(c *gin.Context) {
	notebookID := c.Param("id")
	notes := h.manager.ListNotesByNotebook(notebookID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(notes),
			"notes": notes,
		},
	})
}

// ========== Search ==========

func (h *Handlers) searchNotes(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "query parameter 'q' is required"})
		return
	}

	notes := h.manager.SearchNotes(query)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(notes),
			"query":  query,
			"notes":  notes,
		},
	})
}

// ========== Tags ==========

func (h *Handlers) getTagStats(c *gin.Context) {
	stats := h.manager.GetTagStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(stats),
			"tags":  stats,
		},
	})
}

func (h *Handlers) listNotesByTag(c *gin.Context) {
	tag := c.Param("tag")
	notes := h.manager.ListNotesByTag(tag)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(notes),
			"tag":   tag,
			"notes": notes,
		},
	})
}

// ========== Pinned / Favorites ==========

func (h *Handlers) getPinnedNotes(c *gin.Context) {
	notes := h.manager.GetPinnedNotes()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(notes),
			"notes": notes,
		},
	})
}

func (h *Handlers) getFavoriteNotes(c *gin.Context) {
	notes := h.manager.GetFavoriteNotes()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(notes),
			"notes": notes,
		},
	})
}

// ========== Import / Export ==========

func (h *Handlers) importMarkdown(c *gin.Context) {
	var req ImportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	note := h.manager.ImportMarkdown(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "imported", Data: note})
}

func (h *Handlers) exportNotes(c *gin.Context) {
	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	files, err := h.manager.ExportNotes(req.NoteIDs)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "exported",
		Data: gin.H{
			"total": len(files),
			"files": files,
		},
	})
}

// ========== Recent ==========

func (h *Handlers) getRecentNotes(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "20")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 20
	}

	notes := h.manager.GetRecentNotes(limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(notes),
			"notes": notes,
		},
	})
}
