// Package wiki 提供知识库 REST API 处理器
package wiki

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler 知识库 API 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	wiki := r.Group("/wiki")
	{
		// 知识库管理
		wiki.GET("", h.listWikis)
		wiki.POST("", h.createWiki)
		wiki.GET("/:wikiId", h.getWiki)
		wiki.DELETE("/:wikiId", h.deleteWiki)

		// 页面管理
		wiki.GET("/:wikiId/pages", h.listPages)
		wiki.POST("/:wikiId/pages", h.createPage)
		wiki.GET("/:wikiId/pages/:pageId", h.getPage)
		wiki.PUT("/:wikiId/pages/:pageId", h.updatePage)
		wiki.DELETE("/:wikiId/pages/:pageId", h.deletePage)

		// 版本历史
		wiki.GET("/:wikiId/pages/:pageId/history", h.getHistory)

		// 全文搜索
		wiki.POST("/search", h.searchPages)

		// 权限管理
		wiki.GET("/:wikiId/permissions", h.listPermissions)
		wiki.POST("/:wikiId/permissions", h.setPermission)
		wiki.DELETE("/:wikiId/permissions/:userId", h.removePermission)
	}
}

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (h *Handler) listWikis(c *gin.Context) {
	wikis := h.manager.ListWikis()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: wikis})
}

func (h *Handler) createWiki(c *gin.Context) {
	var req CreateWikiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	wiki, err := h.manager.CreateWiki(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "wiki created", Data: wiki})
}

func (h *Handler) getWiki(c *gin.Context) {
	wikiID := c.Param("wikiId")
	wiki, err := h.manager.GetWiki(wikiID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: wiki})
}

func (h *Handler) deleteWiki(c *gin.Context) {
	wikiID := c.Param("wikiId")
	if err := h.manager.DeleteWiki(wikiID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "wiki deleted"})
}

func (h *Handler) listPages(c *gin.Context) {
	wikiID := c.Param("wikiId")
	wiki, err := h.manager.GetWiki(wikiID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: wiki.Pages})
}

func (h *Handler) createPage(c *gin.Context) {
	wikiID := c.Param("wikiId")
	var req CreatePageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	page, err := h.manager.CreatePage(wikiID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "page created", Data: page})
}

func (h *Handler) getPage(c *gin.Context) {
	wikiID := c.Param("wikiId")
	pageID := c.Param("pageId")

	page, err := h.manager.GetPage(wikiID, pageID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: page})
}

func (h *Handler) updatePage(c *gin.Context) {
	wikiID := c.Param("wikiId")
	pageID := c.Param("pageId")

	var req UpdatePageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	page, err := h.manager.UpdatePage(wikiID, pageID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "page updated", Data: page})
}

func (h *Handler) deletePage(c *gin.Context) {
	wikiID := c.Param("wikiId")
	pageID := c.Param("pageId")

	if err := h.manager.DeletePage(wikiID, pageID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "page deleted"})
}

func (h *Handler) getHistory(c *gin.Context) {
	wikiID := c.Param("wikiId")
	pageID := c.Param("pageId")

	revisions, err := h.manager.GetHistory(wikiID, pageID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: revisions})
}

func (h *Handler) searchPages(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}

	results := h.manager.SearchPages(&req)
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: results})
}

func (h *Handler) listPermissions(c *gin.Context) {
	wikiID := c.Param("wikiId")
	wiki, err := h.manager.GetWiki(wikiID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: wiki.Permissions})
}

func (h *Handler) setPermission(c *gin.Context) {
	wikiID := c.Param("wikiId")
	var req SetPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	perm, err := h.manager.SetPermission(wikiID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "permission set", Data: perm})
}

func (h *Handler) removePermission(c *gin.Context) {
	wikiID := c.Param("wikiId")
	userID := c.Param("userId")

	if err := h.manager.RemovePermission(wikiID, userID); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "permission removed"})
}

// 解析分页参数
func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}
