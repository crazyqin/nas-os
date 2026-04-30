// Package notes Note Station HTTP 处理器
package notes

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 笔记 HTTP 处理器.
type Handlers struct {
	store *Store
}

// NewHandlers 创建处理器.
func NewHandlers(store *Store) *Handlers {
	return &Handlers{store: store}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	notes := api.Group("/notes")
	{
		// 笔记 CRUD
		notes.POST("", h.createNote)
		notes.GET("", h.listNotes)
		notes.GET("/search", h.searchNotes)
		notes.GET("/trash", h.listTrashNotes)
		notes.GET("/tags", h.getAllTags)
		notes.GET("/stats", h.getStats)
		notes.POST("/trash/empty", h.emptyTrash)
		notes.GET("/:id", h.getNote)
		notes.PUT("/:id", h.updateNote)
		notes.DELETE("/:id", h.deleteNote)
		notes.POST("/:id/restore", h.restoreNote)
		notes.POST("/:id/permanent", h.permanentDelete)
		notes.POST("/:id/favorite", h.toggleFavorite)
		notes.POST("/:id/pin", h.togglePin)

		// 笔记本 CRUD
		notes.POST("/notebooks", h.createNotebook)
		notes.GET("/notebooks", h.listNotebooks)
		notes.GET("/notebooks/:id", h.getNotebook)
		notes.PUT("/notebooks/:id", h.updateNotebook)
		notes.DELETE("/notebooks/:id", h.deleteNotebook)

		// 分享管理
		notes.POST("/:id/shares", h.createShare)
		notes.GET("/:id/shares", h.listShares)
		notes.DELETE("/:id/shares/:shareId", h.deleteShare)

		// 公开分享访问
		notes.GET("/share/:token", h.accessShare)
	}
}

// ========== 笔记 API ==========

// createNote 创建笔记.
func (h *Handlers) createNote(c *gin.Context) {
	var input NoteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}
	userName := c.GetString("username")
	if userName == "" {
		userName = "匿名用户"
	}

	note, err := h.store.CreateNote(input, userID, userName)
	if err != nil {
		code := http.StatusBadRequest
		if err == ErrNotebookNotFound {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"code": code, "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": note})
}

// getNote 获取笔记.
func (h *Handlers) getNote(c *gin.Context) {
	id := c.Param("id")

	note, err := h.store.GetNote(id)
	if err != nil {
		code := http.StatusNotFound
		if err == ErrNoteInTrash {
			code = http.StatusGone
		}
		c.JSON(code, gin.H{"code": code, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": note})
}

// updateNote 更新笔记.
func (h *Handlers) updateNote(c *gin.Context) {
	id := c.Param("id")

	var input NoteInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	note, err := h.store.UpdateNote(id, input)
	if err != nil {
		code := http.StatusNotFound
		if err == ErrInvalidFormat {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"code": code, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": note})
}

// deleteNote 删除笔记.
func (h *Handlers) deleteNote(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.DeleteNote(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "笔记已移到回收站"})
}

// restoreNote 恢复笔记.
func (h *Handlers) restoreNote(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.RestoreNote(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "笔记已恢复"})
}

// permanentDelete 永久删除.
func (h *Handlers) permanentDelete(c *gin.Context) {
	id := c.Param("id")

	if err := h.store.PermanentDeleteNote(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "笔记已永久删除"})
}

// listNotes 列出笔记.
func (h *Handlers) listNotes(c *gin.Context) {
	notebookID := c.Query("notebook_id")
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	notes, total := h.store.ListNotes(notebookID, limit, offset)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"notes":  notes,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// listTrashNotes 列出回收站笔记.
func (h *Handlers) listTrashNotes(c *gin.Context) {
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	notes, total := h.store.ListTrashNotes(limit, offset)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"notes":  notes,
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// toggleFavorite 切换收藏.
func (h *Handlers) toggleFavorite(c *gin.Context) {
	id := c.Param("id")

	note, err := h.store.ToggleFavorite(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": note})
}

// togglePin 切换置顶.
func (h *Handlers) togglePin(c *gin.Context) {
	id := c.Param("id")

	note, err := h.store.TogglePin(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": note})
}

// getAllTags 获取所有标签.
func (h *Handlers) getAllTags(c *gin.Context) {
	tags := h.store.GetAllTags()

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": tags})
}

// getStats 获取统计.
func (h *Handlers) getStats(c *gin.Context) {
	userID := c.GetString("user_id")
	stats := h.store.GetNoteStats(userID)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": stats})
}

// emptyTrash 清空回收站.
func (h *Handlers) emptyTrash(c *gin.Context) {
	count := h.store.EmptyTrash()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "回收站已清空",
		"data": gin.H{
			"deleted_count": count,
		},
	})
}

// searchNotes 搜索笔记.
func (h *Handlers) searchNotes(c *gin.Context) {
	keyword := c.Query("keyword")
	notebookID := c.Query("notebook_id")
	format := NoteFormat(c.Query("format"))
	sortBy := c.Query("sort_by")
	sortOrder := c.Query("sort_order")

	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	var tags []string
	if t := c.Query("tags"); t != "" {
		for _, tag := range splitAndTrim(t, ",") {
			tags = append(tags, tag)
		}
	}

	var pinned *bool
	if p := c.Query("pinned"); p != "" {
		b := p == "true"
		pinned = &b
	}

	var favorite *bool
	if f := c.Query("favorite"); f != "" {
		b := f == "true"
		favorite = &b
	}

	query := SearchQuery{
		Keyword:    keyword,
		NotebookID: notebookID,
		Tags:       tags,
		Format:     format,
		Pinned:     pinned,
		Favorite:   favorite,
		Limit:      limit,
		Offset:     offset,
		SortBy:     sortBy,
		SortOrder:  sortOrder,
	}

	result := h.store.Search(query)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": result})
}

// ========== 笔记本 API ==========

// createNotebook 创建笔记本.
func (h *Handlers) createNotebook(c *gin.Context) {
	var input NotebookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	notebook, err := h.store.CreateNotebook(input, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": notebook})
}

// getNotebook 获取笔记本.
func (h *Handlers) getNotebook(c *gin.Context) {
	id := c.Param("id")

	notebook, err := h.store.GetNotebook(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": notebook})
}

// updateNotebook 更新笔记本.
func (h *Handlers) updateNotebook(c *gin.Context) {
	id := c.Param("id")

	var input NotebookInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	notebook, err := h.store.UpdateNotebook(id, input)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": notebook})
}

// deleteNotebook 删除笔记本.
func (h *Handlers) deleteNotebook(c *gin.Context) {
	id := c.Param("id")

	// 获取查询参数：移动笔记到的笔记本ID
	moveTo := c.Query("move_to")

	if err := h.store.DeleteNotebook(id, moveTo); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "笔记本已删除"})
}

// listNotebooks 列出笔记本.
func (h *Handlers) listNotebooks(c *gin.Context) {
	userID := c.GetString("user_id")

	notebooks := h.store.ListNotebooks(userID)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": notebooks})
}

// ========== 分享 API ==========

// createShare 创建分享.
func (h *Handlers) createShare(c *gin.Context) {
	noteID := c.Param("id")

	var input ShareInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		userID = "anonymous"
	}

	share, err := h.store.CreateShare(noteID, input, userID)
	if err != nil {
		code := http.StatusNotFound
		if err.Error() == "无效的权限类型" {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"code": code, "message": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"code": 0, "message": "success", "data": share})
}

// listShares 列出分享.
func (h *Handlers) listShares(c *gin.Context) {
	noteID := c.Param("id")

	shares := h.store.ListNoteShares(noteID)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": shares})
}

// deleteShare 删除分享.
func (h *Handlers) deleteShare(c *gin.Context) {
	shareID := c.Param("shareId")

	if err := h.store.DeleteShare(shareID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "分享已删除"})
}

// accessShare 访问公开分享.
func (h *Handlers) accessShare(c *gin.Context) {
	token := c.Param("token")
	password := c.Query("password")

	note, err := h.store.AccessShare(token, password)
	if err != nil {
		code := http.StatusForbidden
		switch err {
		case ErrShareNotFound:
			code = http.StatusNotFound
		case ErrShareExpired:
			code = http.StatusGone
		case ErrShareMaxViews:
			code = http.StatusTooManyRequests
		case ErrPasswordRequired:
			code = http.StatusUnauthorized
		case ErrPasswordWrong:
			code = http.StatusForbidden
		}
		c.JSON(code, gin.H{"code": code, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": note})
}

// ========== 辅助函数 ==========

// splitAndTrim 分割并去除空白.
func splitAndTrim(s, sep string) []string {
	var result []string
	for _, item := range splitString(s, sep) {
		trimmed := trimSpace(item)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// splitString 分割字符串.
func splitString(s, sep string) []string {
	if s == "" {
		return nil
	}
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == sep[0] {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	result = append(result, s[start:])
	return result
}

// trimSpace 去除首尾空白.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}
