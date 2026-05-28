// Package notes 提供 REST API 处理器
package notes

import (
	"fmt"
	"net/http"

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

// RegisterRoutes 注册路由到 /api/notes 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	ns := r.Group("/notes")
	{
		// 笔记 CRUD
		ns.GET("", h.listNotes)
		ns.POST("", h.createNote)
		ns.GET("/:id", h.getNote)
		ns.PUT("/:id", h.updateNote)
		ns.DELETE("/:id", h.deleteNote)

		// 笔记本 CRUD
		ns.GET("/notebooks", h.listNotebooks)
		ns.POST("/notebooks", h.createNotebook)
		ns.GET("/notebooks/:id", h.getNotebook)
		ns.PUT("/notebooks/:id", h.updateNotebook)
		ns.DELETE("/notebooks/:id", h.deleteNotebook)

		// 按笔记本列出笔记
		ns.GET("/notebooks/:id/notes", h.listNotesByNotebook)

		// 标签
		ns.GET("/tags", h.listTags)
		ns.GET("/tags/:tag", h.listNotesByTag)

		// 搜索
		ns.GET("/search", h.searchNotes)

		// 分享链接
		ns.POST("/:id/share", h.createShareLink)
		ns.GET("/:id/shares", h.listShareLinks)
		ns.DELETE("/shares/:id", h.deleteShareLink)
		ns.GET("/shared/:token", h.accessSharedNote)

		// 版本历史
		ns.GET("/:id/versions", h.listVersions)
		ns.GET("/:id/versions/:version", h.getVersion)
		ns.POST("/:id/versions/:version/restore", h.restoreVersion)

		// 统计
		ns.GET("/stats", h.getStats)
	}
}

// ========== 笔记处理 ==========

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

// ========== 笔记本处理 ==========

func (h *Handlers) listNotebooks(c *gin.Context) {
	notebooks := h.manager.ListNotebooks()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":     len(notebooks),
			"notebooks": notebooks,
		},
	})
}

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

// ========== 标签处理 ==========

func (h *Handlers) listTags(c *gin.Context) {
	tags := h.manager.ListTags()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(tags),
			"tags":  tags,
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
			"notes": notes,
		},
	})
}

// ========== 搜索处理 ==========

func (h *Handlers) searchNotes(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "query parameter 'q' is required"})
		return
	}

	notebookID := c.Query("notebook_id")
	tag := c.Query("tag")

	var notes []*Note
	if notebookID != "" || tag != "" {
		notes = h.manager.SearchNotesAdvanced(query, notebookID, tag)
	} else {
		notes = h.manager.SearchNotes(query)
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(notes),
			"query": query,
			"notes": notes,
		},
	})
}

// ========== 分享链接处理 ==========

func (h *Handlers) createShareLink(c *gin.Context) {
	noteID := c.Param("id")
	var req CreateShareLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	link, err := h.manager.CreateShareLink(noteID, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: link})
}

func (h *Handlers) listShareLinks(c *gin.Context) {
	noteID := c.Param("id")
	links := h.manager.ListShareLinks(noteID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(links),
			"links": links,
		},
	})
}

func (h *Handlers) deleteShareLink(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteShareLink(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

func (h *Handlers) accessSharedNote(c *gin.Context) {
	token := c.Param("token")
	password := c.Query("password")

	note, err := h.manager.AccessSharedNote(token, password)
	if err != nil {
		c.JSON(http.StatusForbidden, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: note})
}

// ========== 版本历史处理 ==========

func (h *Handlers) listVersions(c *gin.Context) {
	noteID := c.Param("id")
	versions := h.manager.GetNoteVersions(noteID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(versions),
			"versions": versions,
		},
	})
}

func (h *Handlers) getVersion(c *gin.Context) {
	noteID := c.Param("id")
	versionStr := c.Param("version")
	version := 0
	if _, err := fmt.Sscanf(versionStr, "%d", &version); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid version number"})
		return
	}

	v, err := h.manager.GetNoteVersion(noteID, version)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: v})
}

func (h *Handlers) restoreVersion(c *gin.Context) {
	noteID := c.Param("id")
	versionStr := c.Param("version")
	version := 0
	if _, err := fmt.Sscanf(versionStr, "%d", &version); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid version number"})
		return
	}

	note, err := h.manager.RestoreNoteVersion(noteID, version)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "restored", Data: note})
}

// ========== 统计信息 ==========

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}
