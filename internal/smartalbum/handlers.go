// Package smartalbum 提供智能相册 HTTP API handlers
package smartalbum

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 智能相册 API handlers
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建 handlers
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/smart-album")
	{
		// 照片管理
		g.POST("/photos", h.AddPhoto)
		g.GET("/photos/:id", h.GetPhoto)
		g.GET("/photos", h.ListPhotos)
		g.DELETE("/photos/:id", h.DeletePhoto)
		g.POST("/photos/:id/favorite", h.ToggleFavorite)

		// 人脸管理
		g.POST("/faces", h.RegisterFace)
		g.GET("/faces", h.ListFaces)
		g.GET("/faces/:id", h.GetFace)
		g.POST("/faces/:faceId/photos/:photoId", h.LinkFaceToPhoto)

		// 相册管理
		g.POST("/albums", h.CreateAlbum)
		g.GET("/albums", h.ListAlbums)
		g.GET("/albums/:id", h.GetAlbum)
		g.POST("/albums/:id/photos/:photoId", h.AddPhotoToAlbum)
		g.POST("/albums/smart", h.CreateSmartAlbum)

		// 搜索功能
		g.POST("/search", h.SearchPhotos)
		g.POST("/search/semantic", h.SemanticSearch)
		g.GET("/photos/:id/similar", h.FindSimilarPhotos)

		// 地图功能
		g.GET("/map/clusters", h.GetMapClusters)
		g.GET("/map/photos", h.GetPhotosByLocation)

		// 时间线
		g.GET("/timeline", h.GetTimeline)

		// 重复检测
		g.GET("/duplicates", h.DetectDuplicates)

		// 批量操作
		g.POST("/embeddings/batch", h.BatchAddEmbeddings)
		g.POST("/photos/:id/auto-tag", h.AutoTag)

		// 统计
		g.GET("/stats", h.GetStats)
	}
}

// ========== 照片管理 ==========

// AddPhoto 添加照片
func (h *Handlers) AddPhoto(c *gin.Context) {
	var photo Photo
	if err := c.ShouldBindJSON(&photo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	result, err := h.mgr.AddPhoto(photo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": result})
}

// GetPhoto 获取照片
func (h *Handlers) GetPhoto(c *gin.Context) {
	id := c.Param("id")
	photo, err := h.mgr.GetPhoto(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": photo})
}

// ListPhotos 列出照片
func (h *Handlers) ListPhotos(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	photos := h.mgr.ListPhotos(limit, offset)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": photos})
}

// DeletePhoto 删除照片
func (h *Handlers) DeletePhoto(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeletePhoto(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// ToggleFavorite 切换收藏
func (h *Handlers) ToggleFavorite(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.ToggleFavorite(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "toggled"})
}

// ========== 人脸管理 ==========

// RegisterFaceRequest 注册人脸请求
type RegisterFaceRequest struct {
	Name      string    `json:"name" binding:"required"`
	PhotoID   string    `json:"photoId" binding:"required"`
	Embedding []float64 `json:"embedding"`
}

// RegisterFace 注册人脸
func (h *Handlers) RegisterFace(c *gin.Context) {
	var req RegisterFaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	face, err := h.mgr.RegisterFace(req.Name, req.PhotoID, req.Embedding)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": face})
}

// ListFaces 列出人脸
func (h *Handlers) ListFaces(c *gin.Context) {
	faces := h.mgr.ListFaces()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": faces})
}

// GetFace 获取人脸
func (h *Handlers) GetFace(c *gin.Context) {
	id := c.Param("id")
	face, err := h.mgr.GetFace(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": face})
}

// LinkFaceToPhoto 关联人脸到照片
func (h *Handlers) LinkFaceToPhoto(c *gin.Context) {
	faceID := c.Param("faceId")
	photoID := c.Param("photoId")
	if err := h.mgr.LinkFaceToPhoto(faceID, photoID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "linked"})
}

// ========== 相册管理 ==========

// CreateAlbumRequest 创建相册请求
type CreateAlbumRequest struct {
	Name string    `json:"name" binding:"required"`
	Type AlbumType `json:"type" binding:"required"`
}

// CreateAlbum 创建相册
func (h *Handlers) CreateAlbum(c *gin.Context) {
	var req CreateAlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	album, err := h.mgr.CreateAlbum(req.Name, req.Type)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": album})
}

// ListAlbums 列出相册
func (h *Handlers) ListAlbums(c *gin.Context) {
	albums := h.mgr.ListAlbums()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": albums})
}

// GetAlbum 获取相册
func (h *Handlers) GetAlbum(c *gin.Context) {
	id := c.Param("id")
	album, err := h.mgr.GetAlbum(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": album})
}

// AddPhotoToAlbum 添加照片到相册
func (h *Handlers) AddPhotoToAlbum(c *gin.Context) {
	albumID := c.Param("id")
	photoID := c.Param("photoId")
	if err := h.mgr.AddPhotoToAlbum(albumID, photoID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "added"})
}

// CreateSmartAlbumRequest 创建智能相册请求
type CreateSmartAlbumRequest struct {
	Name     string        `json:"name" binding:"required"`
	Criteria AlbumCriteria `json:"criteria" binding:"required"`
}

// CreateSmartAlbum 创建智能相册
func (h *Handlers) CreateSmartAlbum(c *gin.Context) {
	var req CreateSmartAlbumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	album, err := h.mgr.CreateSmartAlbum(req.Name, req.Criteria)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": album})
}

// ========== 搜索功能 ==========

// SearchPhotosRequest 搜索照片请求
type SearchPhotosRequest struct {
	Query  string        `json:"query"`
	Tags   []string      `json:"tags"`
	Scene  SceneCategory `json:"scene"`
}

// SearchPhotos 搜索照片
func (h *Handlers) SearchPhotos(c *gin.Context) {
	var req SearchPhotosRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	results := h.mgr.SearchPhotos(req.Query, req.Tags, req.Scene)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": results})
}

// SemanticSearchRequest 语义搜索请求
type SemanticSearchRequest struct {
	Embedding []float64 `json:"embedding" binding:"required"`
	TopK      int       `json:"topK"`
	MinScore  float64   `json:"minScore"`
}

// SemanticSearch 语义搜索
func (h *Handlers) SemanticSearch(c *gin.Context) {
	var req SemanticSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	if req.TopK == 0 {
		req.TopK = 10
	}
	if req.MinScore == 0 {
		req.MinScore = 0.5
	}

	results := h.mgr.SemanticSearch(req.Embedding, req.TopK, req.MinScore)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": results})
}

// FindSimilarPhotos 查找相似照片
func (h *Handlers) FindSimilarPhotos(c *gin.Context) {
	id := c.Param("id")
	topK, _ := strconv.Atoi(c.DefaultQuery("topK", "10"))

	results, err := h.mgr.FindSimilarPhotos(id, topK)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": results})
}

// ========== 地图功能 ==========

// GetMapClusters 获取地图聚合点
func (h *Handlers) GetMapClusters(c *gin.Context) {
	zoomLevel, _ := strconv.Atoi(c.DefaultQuery("zoom", "10"))

	var bounds *MapBounds
	// 如果提供了边界参数
	if c.Query("northEast") != "" && c.Query("southWest") != "" {
		// 简化处理，实际应该解析坐标
		bounds = &MapBounds{}
	}

	clusters := h.mgr.GetMapClusters(bounds, zoomLevel)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": clusters})
}

// GetPhotosByLocation 按地点获取照片
func (h *Handlers) GetPhotosByLocation(c *gin.Context) {
	city := c.Query("city")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	if city == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "city required"})
		return
	}

	results := h.mgr.GetPhotosByLocation(city, limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": results})
}

// ========== 时间线 ==========

// GetTimeline 获取时间线
func (h *Handlers) GetTimeline(c *gin.Context) {
	timeline := h.mgr.GenerateTimeline()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": timeline})
}

// ========== 重复检测 ==========

// DetectDuplicates 检测重复照片
func (h *Handlers) DetectDuplicates(c *gin.Context) {
	groups := h.mgr.DetectDuplicates()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": groups})
}

// ========== 批量操作 ==========

// BatchAddEmbeddingsRequest 批量添加嵌入向量请求
type BatchAddEmbeddingsRequest struct {
	Embeddings map[string][]float64 `json:"embeddings" binding:"required"`
}

// BatchAddEmbeddings 批量添加嵌入向量
func (h *Handlers) BatchAddEmbeddings(c *gin.Context) {
	var req BatchAddEmbeddingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}

	count := h.mgr.BatchAddEmbeddings(req.Embeddings)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"count": count}})
}

// AutoTag 自动生成标签
func (h *Handlers) AutoTag(c *gin.Context) {
	id := c.Param("id")
	tags, err := h.mgr.AutoTag(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": tags})
}

// ========== 统计 ==========

// GetStats 获取统计
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.mgr.GetStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}
