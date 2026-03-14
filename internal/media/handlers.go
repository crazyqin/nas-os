package media

import (
	"net/http"
	"strconv"
	"time"

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

	item, library := h.libraryMgr.GetMediaItemByID(id)
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "媒体项不存在",
		})
		return
	}

	// 返回包含库信息的完整数据
	result := gin.H{
		"id":           item.ID,
		"path":         item.Path,
		"name":         item.Name,
		"type":         item.Type,
		"size":         item.Size,
		"modifiedTime": item.ModifiedTime,
		"metadata":     item.Metadata,
		"posterPath":   item.PosterPath,
		"isFavorite":   item.IsFavorite,
		"tags":         item.Tags,
		"rating":       item.Rating,
		"playCount":    item.PlayCount,
		"lastPlayed":   item.LastPlayed,
		"libraryId":    library.ID,
		"libraryName":  library.Name,
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
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

	updates := make(map[string]interface{})
	if req.Tags != nil {
		updates["tags"] = req.Tags
	}
	updates["rating"] = req.Rating
	updates["isFavorite"] = req.IsFavorite

	if err := h.libraryMgr.UpdateMediaItem(id, updates); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	// 获取更新后的数据
	item, _ := h.libraryMgr.GetMediaItemByID(id)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "更新成功",
		"data":    item,
	})
}

// deleteMediaItem 删除媒体项
func (h *Handlers) deleteMediaItem(c *gin.Context) {
	id := c.Param("id")

	if err := h.libraryMgr.DeleteMediaItem(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
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

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请提供搜索关键词",
		})
		return
	}

	results, err := h.libraryMgr.SearchMovieMetadata(query, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":   query,
			"source":  source,
			"results": results,
			"total":   len(results),
		},
	})
}

// searchTVMetadata 搜索电视剧元数据
func (h *Handlers) searchTVMetadata(c *gin.Context) {
	query := c.Query("q")
	source := c.Query("source")

	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "请提供搜索关键词",
		})
		return
	}

	results, err := h.libraryMgr.SearchTVMetadata(query, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"query":   query,
			"source":  source,
			"results": results,
			"total":   len(results),
		},
	})
}

// getMovieMetadata 获取电影元数据详情
func (h *Handlers) getMovieMetadata(c *gin.Context) {
	id := c.Param("id")
	source := c.Query("source") // tmdb/douban

	movie, err := h.libraryMgr.GetMovieMetadata(id, source)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "获取元数据失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    movie,
	})
}

// getTVMetadata 获取电视剧元数据详情
func (h *Handlers) getTVMetadata(c *gin.Context) {
	id := c.Param("id")
	source := c.Query("source") // tmdb/douban

	tv, err := h.libraryMgr.GetTVMetadata(id, source)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "获取元数据失败：" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    tv,
	})
}

// getPlayHistory 获取播放历史
func (h *Handlers) getPlayHistory(c *gin.Context) {
	limit := 50
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	mediaType := c.Query("type")
	var mType MediaType
	switch mediaType {
	case "movie":
		mType = MediaTypeMovie
	case "tv":
		mType = MediaTypeTV
	case "music":
		mType = MediaTypeMusic
	}

	history := h.libraryMgr.GetPlayHistory(limit)

	// 按媒体类型过滤
	if mType != "" {
		filtered := make([]*PlayHistory, 0)
		for _, h := range history {
			if h.MediaType == mType {
				filtered = append(filtered, h)
			}
		}
		history = filtered
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"history": history,
			"total":   len(history),
		},
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

	// 获取媒体项信息
	item, library := h.libraryMgr.GetMediaItemByID(req.MediaID)
	if item == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "媒体项不存在",
		})
		return
	}

	// 创建播放历史记录
	history := &PlayHistory{
		MediaID:    req.MediaID,
		MediaName:  item.Name,
		MediaType:  item.Type,
		PosterPath: item.PosterPath,
		Position:   req.Position,
		Duration:   req.Duration,
		Completed:  req.Completed,
		PlayedAt:   time.Now(),
		LibraryID:  library.ID,
	}

	h.libraryMgr.AddPlayHistory(history)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    history,
	})
}

// getFavorites 获取收藏列表
func (h *Handlers) getFavorites(c *gin.Context) {
	mediaType := c.Query("type")
	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	var mType MediaType
	switch mediaType {
	case "movie":
		mType = MediaTypeMovie
	case "tv":
		mType = MediaTypeTV
	case "music":
		mType = MediaTypeMusic
	}

	favorites := h.libraryMgr.GetFavorites(mType)

	// 限制数量
	if limit > 0 && len(favorites) > limit {
		favorites = favorites[:limit]
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"favorites": favorites,
			"total":     len(favorites),
		},
	})
}

// toggleFavorite 切换收藏状态
func (h *Handlers) toggleFavorite(c *gin.Context) {
	id := c.Param("id")

	item, err := h.libraryMgr.ToggleFavorite(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	action := "已取消收藏"
	if item.IsFavorite {
		action = "已收藏"
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": action,
		"data": gin.H{
			"id":         id,
			"isFavorite": item.IsFavorite,
		},
	})
}
