package media

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 媒体处理器
type Handlers struct {
	libraryMgr *LibraryManager
}

// NewHandlers 创建媒体处理器
func NewHandlers(libraryMgr *LibraryManager) *Handlers {
	return &Handlers{
		libraryMgr: libraryMgr,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	media := r.Group("/media")
	{
		// 媒体库管理
		media.GET("/libraries", h.listLibraries)
		media.POST("/libraries", h.createLibrary)
		media.GET("/libraries/:id", h.getLibrary)
		media.PUT("/libraries/:id", h.updateLibrary)
		media.DELETE("/libraries/:id", h.deleteLibrary)
		media.POST("/libraries/:id/scan", h.scanLibrary)

		// 媒体项目
		media.GET("/items", h.searchMedia)
		media.GET("/items/:id", h.getMediaItem)
		media.PUT("/items/:id", h.updateMediaItem)
		media.DELETE("/items/:id", h.deleteMediaItem)

		// 海报墙
		media.GET("/wall", h.getMediaWall)
		media.GET("/wall/movies", h.getMovieWall)
		media.GET("/wall/tv", h.getTVWall)
		media.GET("/wall/music", h.getMusicWall)

		// 元数据搜索
		media.GET("/metadata/search/movie", h.searchMovieMetadata)
		media.GET("/metadata/search/tv", h.searchTVMetadata)
		media.GET("/metadata/movie/:id", h.getMovieMetadata)
		media.GET("/metadata/tv/:id", h.getTVMetadata)

		// 播放历史
		media.GET("/history", h.getPlayHistory)
		media.POST("/history", h.addPlayHistory)

		// 收藏
		media.GET("/favorites", h.getFavorites)
		media.POST("/items/:id/favorite", h.toggleFavorite)
	}
}

// listLibraries 列出媒体库
func (h *Handlers) listLibraries(c *gin.Context) {
	libraries := h.libraryMgr.ListLibraries()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    libraries,
	})
}

// createLibrary 创建媒体库
func (h *Handlers) createLibrary(c *gin.Context) {
	var req struct {
		Name           string `json:"name" binding:"required"`
		Path           string `json:"path" binding:"required"`
		Type           string `json:"type" binding:"required"`
		Description    string `json:"description"`
		MetadataSource string `json:"metadataSource"`
		TMDBApiKey     string `json:"tmdbApiKey"`
		DoubanApiKey   string `json:"doubanApiKey"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	var mediaType MediaType
	switch req.Type {
	case "movie":
		mediaType = MediaTypeMovie
	case "tv":
		mediaType = MediaTypeTV
	case "music":
		mediaType = MediaTypeMusic
	case "photo":
		mediaType = MediaTypePhoto
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "不支持的媒体类型",
		})
		return
	}

	library, err := h.libraryMgr.CreateLibrary(req.Name, req.Path, mediaType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	// 配置 API 密钥
	updates := make(map[string]interface{})
	if req.TMDBApiKey != "" {
		updates["tmdbApiKey"] = req.TMDBApiKey
	}
	if req.DoubanApiKey != "" {
		updates["doubanApiKey"] = req.DoubanApiKey
	}
	if req.MetadataSource != "" {
		updates["metadataSource"] = req.MetadataSource
	}

	if len(updates) > 0 {
		_ = h.libraryMgr.UpdateLibrary(library.ID, updates)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "媒体库创建成功",
		"data":    library,
	})
}

// getLibrary 获取媒体库详情
func (h *Handlers) getLibrary(c *gin.Context) {
	id := c.Param("id")

	library := h.libraryMgr.GetLibrary(id)
	if library == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "媒体库不存在",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    library,
	})
}

// updateLibrary 更新媒体库
func (h *Handlers) updateLibrary(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name           string `json:"name"`
		Description    string `json:"description"`
		Path           string `json:"path"`
		Enabled        *bool  `json:"enabled"`
		AutoScan       *bool  `json:"autoScan"`
		ScanInterval   *int   `json:"scanInterval"`
		MetadataSource string `json:"metadataSource"`
		TMDBApiKey     string `json:"tmdbApiKey"`
		DoubanApiKey   string `json:"doubanApiKey"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Path != "" {
		updates["path"] = req.Path
	}
	if req.Enabled != nil {
		updates["enabled"] = *req.Enabled
	}
	if req.AutoScan != nil {
		updates["autoScan"] = *req.AutoScan
	}
	if req.ScanInterval != nil {
		updates["scanInterval"] = *req.ScanInterval
	}
	if req.MetadataSource != "" {
		updates["metadataSource"] = req.MetadataSource
	}
	if req.TMDBApiKey != "" {
		updates["tmdbApiKey"] = req.TMDBApiKey
	}
	if req.DoubanApiKey != "" {
		updates["doubanApiKey"] = req.DoubanApiKey
	}

	if err := h.libraryMgr.UpdateLibrary(id, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
	})
}

// deleteLibrary 删除媒体库
func (h *Handlers) deleteLibrary(c *gin.Context) {
	id := c.Param("id")

	if err := h.libraryMgr.DeleteLibrary(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
	})
}

// scanLibrary 扫描媒体库
func (h *Handlers) scanLibrary(c *gin.Context) {
	id := c.Param("id")

	if err := h.libraryMgr.ScanLibrary(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "扫描完成",
	})
}

// searchMedia 搜索媒体
func (h *Handlers) searchMedia(c *gin.Context) {
	query := c.Query("q")
	mediaType := c.Query("type")

	var mType MediaType
	switch mediaType {
	case "movie":
		mType = MediaTypeMovie
	case "tv":
		mType = MediaTypeTV
	case "music":
		mType = MediaTypeMusic
	case "photo":
		mType = MediaTypePhoto
	}

	items, err := h.libraryMgr.SearchMedia(query, mType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    items,
	})
}

// getMediaItem 获取媒体项详情
func (h *Handlers) getMediaItem(c *gin.Context) {
	id := c.Param("id")

	// TODO: 实现获取单个媒体项
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"id": id},
	})
}

// updateMediaItem 更新媒体项
func (h *Handlers) updateMediaItem(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Tags       []string `json:"tags"`
		Rating     float64  `json:"rating"`
		IsFavorite bool     `json:"isFavorite"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// TODO: 实现更新媒体项
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
		"data":    gin.H{"id": id},
	})
}

// deleteMediaItem 删除媒体项
func (h *Handlers) deleteMediaItem(c *gin.Context) {
	id := c.Param("id")

	// TODO: 实现删除媒体项
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
		"data":    gin.H{"id": id},
	})
}

// getMediaWall 获取海报墙
func (h *Handlers) getMediaWall(c *gin.Context) {
	mediaType := c.Query("type")
	limit := 50 // 默认 50 个

	var mType MediaType
	switch mediaType {
	case "movie":
		mType = MediaTypeMovie
	case "tv":
		mType = MediaTypeTV
	case "music":
		mType = MediaTypeMusic
	case "photo":
		mType = MediaTypePhoto
	}

	items, err := h.libraryMgr.GetMediaWall(mType, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    items,
	})
}

// getMovieWall 获取电影海报墙
func (h *Handlers) getMovieWall(c *gin.Context) {
	limit := 50
	items, err := h.libraryMgr.GetMediaWall(MediaTypeMovie, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    items,
	})
}

// getTVWall 获取电视剧海报墙
func (h *Handlers) getTVWall(c *gin.Context) {
	limit := 50
	items, err := h.libraryMgr.GetMediaWall(MediaTypeTV, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    items,
	})
}

// getMusicWall 获取音乐专辑墙
func (h *Handlers) getMusicWall(c *gin.Context) {
	limit := 50
	items, err := h.libraryMgr.GetMediaWall(MediaTypeMusic, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    items,
	})
}

// searchMovieMetadata 搜索电影元数据
func (h *Handlers) searchMovieMetadata(c *gin.Context) {
	query := c.Query("q")
	source := c.Query("source") // tmdb/douban

	// TODO: 实现元数据搜索
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":  query,
			"source": source,
		},
	})
}

// searchTVMetadata 搜索电视剧元数据
func (h *Handlers) searchTVMetadata(c *gin.Context) {
	query := c.Query("q")
	source := c.Query("source")

	// TODO: 实现元数据搜索
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":  query,
			"source": source,
		},
	})
}

// getMovieMetadata 获取电影元数据详情
func (h *Handlers) getMovieMetadata(c *gin.Context) {
	id := c.Param("id")

	// TODO: 实现获取元数据详情
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"id": id},
	})
}

// getTVMetadata 获取电视剧元数据详情
func (h *Handlers) getTVMetadata(c *gin.Context) {
	id := c.Param("id")

	// TODO: 实现获取元数据详情
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"id": id},
	})
}

// getPlayHistory 获取播放历史
func (h *Handlers) getPlayHistory(c *gin.Context) {
	// TODO: 实现播放历史
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    []interface{}{},
	})
}

// addPlayHistory 添加播放历史
func (h *Handlers) addPlayHistory(c *gin.Context) {
	var req struct {
		MediaID   string `json:"mediaId" binding:"required"`
		Position  int    `json:"position"`
		Duration  int    `json:"duration"`
		Completed bool   `json:"completed"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// TODO: 实现添加播放历史
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// getFavorites 获取收藏列表
func (h *Handlers) getFavorites(c *gin.Context) {
	// TODO: 实现收藏列表
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    []interface{}{},
	})
}

// toggleFavorite 切换收藏状态
func (h *Handlers) toggleFavorite(c *gin.Context) {
	id := c.Param("id")

	// TODO: 实现切换收藏
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    gin.H{"id": id},
	})
}
