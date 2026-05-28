package smarttag

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handlers struct{ mgr *Manager }
func NewHandlers(mgr *Manager) *Handlers { return &Handlers{mgr: mgr} }

func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/smart-tag")
	{
		g.GET("/tags", h.ListTags)
		g.POST("/tags", h.CreateTag)
		g.GET("/tags/:id", h.GetTag)
		g.DELETE("/tags/:id", h.DeleteTag)

		g.POST("/files/tag", h.TagFile)
		g.POST("/files/untag", h.UntagFile)
		g.GET("/files", h.GetFileTags)

		g.POST("/classify", h.ClassifyFile)
		g.POST("/classify/batch", h.BatchClassify)
		g.POST("/search", h.SearchByTag)
		g.GET("/stats", h.GetStats)
	}
}

func (h *Handlers) ListTags(c *gin.Context) {
	tagType := TagType(c.Query("type"))
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.ListTags(tagType)})
}

func (h *Handlers) CreateTag(c *gin.Context) {
	var tag Tag
	if err := c.ShouldBindJSON(&tag); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.CreateTag(&tag); err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": tag})
}

func (h *Handlers) GetTag(c *gin.Context) {
	tag, err := h.mgr.GetTag(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": tag})
}

func (h *Handlers) DeleteTag(c *gin.Context) {
	if err := h.mgr.DeleteTag(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

func (h *Handlers) TagFile(c *gin.Context) {
	var req struct {
		FilePath string   `json:"file_path"`
		TagIDs   []string `json:"tag_ids"`
		UserID   string   `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.TagFile(req.FilePath, req.TagIDs, req.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "tagged"})
}

func (h *Handlers) UntagFile(c *gin.Context) {
	var req struct {
		FilePath string   `json:"file_path"`
		TagIDs   []string `json:"tag_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	if err := h.mgr.UntagFile(req.FilePath, req.TagIDs); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "untagged"})
}

func (h *Handlers) GetFileTags(c *gin.Context) {
	path := c.Query("path")
	ft := h.mgr.GetFileTags(path)
	if ft == nil {
		c.JSON(http.StatusOK, gin.H{"code": 0, "data": nil, "message": "no tags"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": ft})
}

func (h *Handlers) ClassifyFile(c *gin.Context) {
	var req struct {
		FilePath string `json:"file_path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.ClassifyFile(req.FilePath)})
}

func (h *Handlers) BatchClassify(c *gin.Context) {
	var req struct {
		FilePaths []string `json:"file_paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.BatchClassify(req.FilePaths)})
}

func (h *Handlers) SearchByTag(c *gin.Context) {
	var req struct {
		TagIDs []string `json:"tag_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.SearchByTag(req.TagIDs)})
}

func (h *Handlers) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": h.mgr.GetStats()})
}
