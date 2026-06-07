package mediaindex

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 媒体索引 HTTP 处理器.
type Handlers struct {
	indexer *Indexer
	search  *SearchEngine
}

// NewHandlers 创建处理器.
func NewHandlers(indexer *Indexer) *Handlers {
	return &Handlers{
		indexer: indexer,
		search:  NewSearchEngine(indexer),
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	mediaGroup := api.Group("/mediaindex")
	{
		// 索引管理
		mediaGroup.GET("/stats", h.getStats)
		mediaGroup.POST("/index", h.indexFile)
		mediaGroup.POST("/index-dir", h.indexDirectory)

		// 文件管理
		mediaGroup.GET("/files/:id", h.getFile)
		mediaGroup.DELETE("/files/:id", h.deleteFile)

		// 搜索
		mediaGroup.POST("/search", h.searchFiles)
		mediaGroup.GET("/search/by-tag/:tagId", h.searchByTag)
		mediaGroup.GET("/search/by-type/:type", h.searchByType)
		mediaGroup.GET("/search/by-location", h.searchByLocation)
		mediaGroup.GET("/search/duplicates", h.searchDuplicates)
		mediaGroup.GET("/recent", h.getRecent)

		// 时间线
		mediaGroup.GET("/timeline", h.getTimeline)

		// 标签管理
		mediaGroup.GET("/tags", h.getTags)
		mediaGroup.POST("/tags", h.createTag)
		mediaGroup.POST("/files/:id/tags/:tagId", h.tagFile)

		// 合集管理
		mediaGroup.GET("/collections", h.getCollections)
		mediaGroup.POST("/collections", h.createCollection)
		mediaGroup.POST("/collections/:colId/files/:fileId", h.addToCollection)
	}
}

func (h *Handlers) getStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.indexer.GetStats())
}

func (h *Handlers) indexFile(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	mf, err := h.indexer.IndexFile(req.Path)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, mf)
}

func (h *Handlers) indexDirectory(c *gin.Context) {
	var req struct {
		Path string `json:"path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	count, err := h.indexer.IndexDirectory(req.Path)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"indexed": count, "path": req.Path})
}

func (h *Handlers) getFile(c *gin.Context) {
	mf, err := h.indexer.Get(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, mf)
}

func (h *Handlers) deleteFile(c *gin.Context) {
	if err := h.indexer.Delete(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *Handlers) searchFiles(c *gin.Context) {
	var query SearchQuery
	if err := c.ShouldBindJSON(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result := h.search.Search(query)
	c.JSON(http.StatusOK, result)
}

func (h *Handlers) searchByTag(c *gin.Context) {
	files := h.search.SearchByTag(c.Param("tagId"))
	c.JSON(http.StatusOK, gin.H{"files": files, "total": len(files)})
}

func (h *Handlers) searchByType(c *gin.Context) {
	mt := MediaType(c.Param("type"))
	if mt != MediaTypeImage && mt != MediaTypeVideo && mt != MediaTypeAudio {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid media type"})
		return
	}
	files := h.search.SearchByType(mt)
	c.JSON(http.StatusOK, gin.H{"files": files, "total": len(files)})
}

func (h *Handlers) searchByLocation(c *gin.Context) {
	loc := c.Query("q")
	if loc == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q parameter required"})
		return
	}
	files := h.search.SearchByLocation(loc)
	c.JSON(http.StatusOK, gin.H{"files": files, "total": len(files)})
}

func (h *Handlers) searchDuplicates(c *gin.Context) {
	files := h.search.SearchDuplicates()
	c.JSON(http.StatusOK, gin.H{"files": files, "total": len(files)})
}

func (h *Handlers) getRecent(c *gin.Context) {
	limit := 20
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && l > 0 {
		limit = l
	}
	files := h.search.GetRecent(limit)
	c.JSON(http.StatusOK, gin.H{"files": files, "total": len(files)})
}

func (h *Handlers) getTimeline(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"timeline": h.indexer.GetTimeline()})
}

func (h *Handlers) getTags(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"tags": h.indexer.GetTags()})
}

func (h *Handlers) createTag(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	tag := h.indexer.AddTag(req.Name)
	c.JSON(http.StatusCreated, tag)
}

func (h *Handlers) tagFile(c *gin.Context) {
	if err := h.indexer.TagFile(c.Param("id"), c.Param("tagId")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "tagged"})
}

func (h *Handlers) getCollections(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"collections": h.indexer.GetCollections()})
}

func (h *Handlers) createCollection(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	col := h.indexer.CreateCollection(req.Name, req.Description)
	c.JSON(http.StatusCreated, col)
}

func (h *Handlers) addToCollection(c *gin.Context) {
	if err := h.indexer.AddToCollection(c.Param("colId"), c.Param("fileId")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "added to collection"})
}
