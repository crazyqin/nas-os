// Package phototimeline provides photo timeline management for NAS-OS.
package phototimeline

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 照片时间线 HTTP handler
type Handler struct {
	timeline  *TimelineManager
	albums    *AlbumManager
	dedup     *DedupManager
	config    Config
	logger    *zap.Logger
}

// NewHandler 创建 HTTP handler
func NewHandler(timeline *TimelineManager, albums *AlbumManager, dedup *DedupManager, config Config, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		timeline: timeline,
		albums:   albums,
		dedup:    dedup,
		config:   config,
		logger:   logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	photos := rg.Group("/photos")
	{
		// 照片管理
		photos.POST("", h.UploadPhoto)
		photos.GET("", h.ListPhotos)
		photos.GET("/:id", h.GetPhoto)
		photos.PUT("/:id", h.UpdatePhoto)
		photos.DELETE("/:id", h.DeletePhoto)

		// 时间线
		photos.GET("/timeline", h.GetTimeline)
		photos.GET("/timeline/stats", h.GetTimelineStats)

		// 搜索
		photos.GET("/search", h.SearchPhotos)

		// 批量操作
		photos.POST("/batch", h.BatchOperation)

		// 地图
		photos.GET("/map", h.GetMapData)

		// EXIF
		photos.GET("/:id/exif", h.GetEXIF)
		photos.PUT("/:id/exif", h.UpdateEXIF)

		// 标签
		photos.POST("/:id/tags", h.AddTags)
		photos.DELETE("/:id/tags", h.RemoveTags)

		// 人物
		photos.POST("/:id/people", h.AddPeople)
		photos.DELETE("/:id/people", h.RemovePeople)
	}

	// 相册管理
	albums := rg.Group("/albums")
	{
		albums.POST("", h.CreateAlbum)
		albums.GET("", h.ListAlbums)
		albums.GET("/:id", h.GetAlbum)
		albums.PUT("/:id", h.UpdateAlbum)
		albums.DELETE("/:id", h.DeleteAlbum)
		albums.GET("/:id/photos", h.GetAlbumPhotos)
		albums.POST("/:id/photos", h.AddPhotosToAlbum)
		albums.DELETE("/:id/photos", h.RemovePhotosFromAlbum)
	}

	// 去重
	dedup := rg.Group("/dedup")
	{
		dedup.GET("/stats", h.GetDedupStats)
		dedup.GET("/groups", h.GetDuplicateGroups)
		dedup.POST("/groups/:id/remove", h.RemoveDuplicates)
	}

	// 分享
	shares := rg.Group("/shares")
	{
		shares.POST("", h.CreateShareLink)
		shares.GET("/:token", h.GetShareLink)
		shares.DELETE("/:id", h.DeleteShareLink)
	}
}

// UploadPhoto 上传照片
func (h *Handler) UploadPhoto(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	if file.Size > h.config.MaxUploadSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file too large"})
		return
	}

	// TODO: 实现文件保存、EXIF 提取、缩略图生成
	photo := &Photo{
		ID:         generateID(),
		Filename:   file.Filename,
		Size:       file.Size,
		ModifiedAt: time.Now(),
		ImportedAt: time.Now(),
	}

	if err := h.timeline.AddPhoto(photo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, photo)
}

// ListPhotos 列出照片
func (h *Handler) ListPhotos(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	result, err := h.timeline.GetTimeline(TimelineViewMonth, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetPhoto 获取照片
func (h *Handler) GetPhoto(c *gin.Context) {
	id := c.Param("id")

	photo, err := h.timeline.GetPhoto(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, photo)
}

// UpdatePhoto 更新照片
func (h *Handler) UpdatePhoto(c *gin.Context) {
	id := c.Param("id")

	photo, err := h.timeline.GetPhoto(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var update Photo
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 更新允许的字段
	if update.Favorite != photo.Favorite {
		photo.Favorite = update.Favorite
	}
	if update.Rating != photo.Rating {
		photo.Rating = update.Rating
	}
	if update.Location != "" {
		photo.Location = update.Location
	}
	if len(update.Tags) > 0 {
		photo.Tags = update.Tags
	}

	if err := h.timeline.UpdatePhoto(photo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, photo)
}

// DeletePhoto 删除照片
func (h *Handler) DeletePhoto(c *gin.Context) {
	id := c.Param("id")

	if err := h.timeline.RemovePhoto(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "photo deleted"})
}

// GetTimeline 获取时间线
func (h *Handler) GetTimeline(c *gin.Context) {
	view := TimelineView(c.DefaultQuery("view", "month"))
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	result, err := h.timeline.GetTimeline(view, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetTimelineStats 获取时间线统计
func (h *Handler) GetTimelineStats(c *gin.Context) {
	stats := h.timeline.GetStats()
	c.JSON(http.StatusOK, stats)
}

// SearchPhotos 搜索照片
func (h *Handler) SearchPhotos(c *gin.Context) {
	var query SearchQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// TODO: 实现完整的搜索逻辑
	// 目前返回时间线数据
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 50
	}

	result, err := h.timeline.GetTimeline(TimelineViewMonth, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// BatchOperation 批量操作
func (h *Handler) BatchOperation(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.PhotoIDs) > h.config.MaxBatchSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "batch size exceeded"})
		return
	}

	result := &BatchResult{
		Total: len(req.PhotoIDs),
	}

	for _, id := range req.PhotoIDs {
		photo, err := h.timeline.GetPhoto(id)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err.Error())
			continue
		}

		switch req.Operation {
		case BatchOpDelete:
			if err := h.timeline.RemovePhoto(id); err != nil {
				result.Failed++
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.Success++
			}
		case BatchOpFav:
			photo.Favorite = true
			h.timeline.UpdatePhoto(photo)
			result.Success++
		case BatchOpUnfav:
			photo.Favorite = false
			h.timeline.UpdatePhoto(photo)
			result.Success++
		case BatchOpTag:
			if req.Value != "" && !contains(photo.Tags, req.Value) {
				photo.Tags = append(photo.Tags, req.Value)
				h.timeline.UpdatePhoto(photo)
			}
			result.Success++
		case BatchOpUntag:
			photo.Tags = removeFromSlice(photo.Tags, req.Value)
			h.timeline.UpdatePhoto(photo)
			result.Success++
		case BatchOpTrash:
			photo.Trashed = true
			h.timeline.UpdatePhoto(photo)
			result.Success++
		default:
			result.Failed++
			result.Errors = append(result.Errors, "unsupported operation")
		}
	}

	c.JSON(http.StatusOK, result)
}

// GetMapData 获取地图数据
func (h *Handler) GetMapData(c *gin.Context) {
	// TODO: 实现地图聚合逻辑
	c.JSON(http.StatusOK, MapResponse{
		Clusters: []MapCluster{},
		Total:    0,
	})
}

// GetEXIF 获取 EXIF 信息
func (h *Handler) GetEXIF(c *gin.Context) {
	id := c.Param("id")

	photo, err := h.timeline.GetPhoto(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, photo.EXIF)
}

// UpdateEXIF 更新 EXIF 信息
func (h *Handler) UpdateEXIF(c *gin.Context) {
	id := c.Param("id")

	photo, err := h.timeline.GetPhoto(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var exif EXIFData
	if err := c.ShouldBindJSON(&exif); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	photo.EXIF = exif
	if err := h.timeline.UpdatePhoto(photo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, photo.EXIF)
}

// AddTags 添加标签
func (h *Handler) AddTags(c *gin.Context) {
	id := c.Param("id")

	photo, err := h.timeline.GetPhoto(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		Tags []string `json:"tags" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, tag := range req.Tags {
		if !contains(photo.Tags, tag) {
			photo.Tags = append(photo.Tags, tag)
		}
	}

	h.timeline.UpdatePhoto(photo)
	c.JSON(http.StatusOK, photo.Tags)
}

// RemoveTags 删除标签
func (h *Handler) RemoveTags(c *gin.Context) {
	id := c.Param("id")

	photo, err := h.timeline.GetPhoto(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		Tags []string `json:"tags" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, tag := range req.Tags {
		photo.Tags = removeFromSlice(photo.Tags, tag)
	}

	h.timeline.UpdatePhoto(photo)
	c.JSON(http.StatusOK, photo.Tags)
}

// AddPeople 添加人物
func (h *Handler) AddPeople(c *gin.Context) {
	id := c.Param("id")

	photo, err := h.timeline.GetPhoto(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		People []string `json:"people" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, person := range req.People {
		if !contains(photo.People, person) {
			photo.People = append(photo.People, person)
		}
	}

	h.timeline.UpdatePhoto(photo)
	c.JSON(http.StatusOK, photo.People)
}

// RemovePeople 删除人物
func (h *Handler) RemovePeople(c *gin.Context) {
	id := c.Param("id")

	photo, err := h.timeline.GetPhoto(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	var req struct {
		People []string `json:"people" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for _, person := range req.People {
		photo.People = removeFromSlice(photo.People, person)
	}

	h.timeline.UpdatePhoto(photo)
	c.JSON(http.StatusOK, photo.People)
}

// CreateAlbum 创建相册
func (h *Handler) CreateAlbum(c *gin.Context) {
	var album Album
	if err := c.ShouldBindJSON(&album); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if album.ID == "" {
		album.ID = generateID()
	}

	if err := h.albums.CreateAlbum(&album); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, album)
}

// ListAlbums 列出相册
func (h *Handler) ListAlbums(c *gin.Context) {
	albumType := AlbumType(c.Query("type"))

	albums := h.albums.ListAlbums(albumType)
	c.JSON(http.StatusOK, gin.H{
		"albums": albums,
		"total":  len(albums),
	})
}

// GetAlbum 获取相册
func (h *Handler) GetAlbum(c *gin.Context) {
	id := c.Param("id")

	album, err := h.albums.GetAlbum(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, album)
}

// UpdateAlbum 更新相册
func (h *Handler) UpdateAlbum(c *gin.Context) {
	id := c.Param("id")

	album, err := h.albums.GetAlbum(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	if err := c.ShouldBindJSON(album); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.albums.UpdateAlbum(album); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, album)
}

// DeleteAlbum 删除相册
func (h *Handler) DeleteAlbum(c *gin.Context) {
	id := c.Param("id")

	if err := h.albums.DeleteAlbum(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "album deleted"})
}

// GetAlbumPhotos 获取相册照片
func (h *Handler) GetAlbumPhotos(c *gin.Context) {
	id := c.Param("id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "50"))

	result, err := h.albums.GetAlbumPhotos(id, page, pageSize)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// AddPhotosToAlbum 添加照片到相册
func (h *Handler) AddPhotosToAlbum(c *gin.Context) {
	albumID := c.Param("id")

	var req struct {
		PhotoIDs []string `json:"photo_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.albums.AddPhotosToAlbum(albumID, req.PhotoIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "photos added to album"})
}

// RemovePhotosFromAlbum 从相册删除照片
func (h *Handler) RemovePhotosFromAlbum(c *gin.Context) {
	albumID := c.Param("id")

	var req struct {
		PhotoIDs []string `json:"photo_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.albums.RemovePhotosFromAlbum(albumID, req.PhotoIDs); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "photos removed from album"})
}

// GetDedupStats 获取去重统计
func (h *Handler) GetDedupStats(c *gin.Context) {
	stats := h.dedup.GetDedupStats()
	c.JSON(http.StatusOK, stats)
}

// GetDuplicateGroups 获取重复组
func (h *Handler) GetDuplicateGroups(c *gin.Context) {
	groups := h.dedup.FindDuplicates()
	c.JSON(http.StatusOK, gin.H{
		"groups": groups,
		"total":  len(groups),
	})
}

// RemoveDuplicates 删除重复照片
func (h *Handler) RemoveDuplicates(c *gin.Context) {
	groupID := c.Param("id")

	var req struct {
		KeepPhotoID string `json:"keep_photo_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.dedup.RemoveDuplicates(groupID, req.KeepPhotoID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "duplicates removed"})
}

// CreateShareLink 创建分享链接
func (h *Handler) CreateShareLink(c *gin.Context) {
	var link ShareLink
	if err := c.ShouldBindJSON(&link); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if link.ID == "" {
		link.ID = generateID()
	}
	if link.Token == "" {
		link.Token = generateToken()
	}
	link.CreatedAt = time.Now()

	// TODO: 保存分享链接

	c.JSON(http.StatusCreated, link)
}

// GetShareLink 获取分享链接
func (h *Handler) GetShareLink(c *gin.Context) {
	token := c.Param("token")

	// TODO: 查找分享链接
	_ = token

	c.JSON(http.StatusNotFound, gin.H{"error": "share link not found"})
}

// DeleteShareLink 删除分享链接
func (h *Handler) DeleteShareLink(c *gin.Context) {
	id := c.Param("id")

	// TODO: 删除分享链接
	_ = id

	c.JSON(http.StatusOK, gin.H{"message": "share link deleted"})
}

// 辅助函数
func generateID() string {
	// 简单的 ID 生成，实际应使用 UUID
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func generateToken() string {
	// 简单的 token 生成，实际应使用 crypto/rand
	return generateID() + generateID()
}
