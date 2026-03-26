// Package face REST API处理器
package face

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers REST API处理器
type Handlers struct {
	service *ServiceImpl
}

// NewHandlers 创建处理器
func NewHandlers(service *ServiceImpl) *Handlers {
	return &Handlers{service: service}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	face := r.Group("/face")
	{
		// 人脸检测
		face.POST("/detect", h.Detect)
		face.POST("/recognize", h.Recognize)
		face.POST("/batch", h.BatchRecognize)

		// 人物管理
		face.GET("/persons", h.ListPersons)
		face.POST("/persons", h.CreatePerson)
		face.GET("/persons/:id", h.GetPerson)
		face.PUT("/persons/:id", h.UpdatePerson)
		face.DELETE("/persons/:id", h.DeletePerson)

		// 人脸管理
		face.GET("/photos/:photoId/faces", h.GetPhotoFaces)
		face.GET("/persons/:personId/faces", h.GetPersonFaces)
		face.POST("/faces/:faceId/assign", h.AssignFace)
		face.POST("/faces/:faceId/unassign", h.UnassignFace)

		// 聚类
		face.POST("/cluster", h.ClusterFaces)
		face.POST("/auto-label", h.AutoLabel)

		// 统计
		face.GET("/stats", h.GetStats)

		// 比较和搜索
		face.POST("/compare", h.CompareFaces)
		face.POST("/search", h.SearchSimilarFaces)

		// 配置
		face.GET("/config", h.GetConfig)
		face.PUT("/config", h.UpdateConfig)
	}
}

// ==================== 请求/响应类型 ====================

// DetectRequest 检测请求
type DetectRequest struct {
	Image     string `json:"image" binding:"required"`     // Base64编码图像
	PhotoID   string `json:"photoId"`                      // 照片ID
	Format    string `json:"format"`                       // jpeg, png
	MinSize   int    `json:"minSize"`                      // 最小人脸尺寸
	MaxFaces  int    `json:"maxFaces"`                     // 最大人脸数
}

// DetectResponse 检测响应
type DetectResponse struct {
	Faces     []Face `json:"faces"`
	Count     int    `json:"count"`
	ProcessMs int64  `json:"processMs"`
}

// RecognizeRequest 识别请求
type RecognizeRequest struct {
	Image   string `json:"image" binding:"required"`
	PhotoID string `json:"photoId" binding:"required"`
	Format  string `json:"format"`
}

// RecognizeResponse 识别响应
type RecognizeResponse struct {
	Faces     []Face `json:"faces"`
	Count     int    `json:"count"`
	ProcessMs int64  `json:"processMs"`
}

// CreatePersonRequest 创建人物请求
type CreatePersonRequest struct {
	Name string `json:"name" binding:"required"`
}

// UpdatePersonRequest 更新人物请求
type UpdatePersonRequest struct {
	Name        string `json:"name"`
	CoverFaceID string `json:"coverFaceId"`
}

// AssignFaceRequest 分配人脸请求
type AssignFaceRequest struct {
	PersonID string `json:"personId" binding:"required"`
}

// ClusterRequest 聚类请求
type ClusterRequest struct {
	PhotoIDs []string `json:"photoIds" binding:"required"`
}

// AutoLabelRequest 自动标记请求
type AutoLabelRequest struct {
	PersonID  string   `json:"personId" binding:"required"`
	PhotoIDs  []string `json:"photoIds" binding:"required"`
}

// CompareRequest 比较请求
type CompareRequest struct {
	Embedding1 []float32 `json:"embedding1" binding:"required"`
	Embedding2 []float32 `json:"embedding2" binding:"required"`
}

// CompareResponse 比较响应
type CompareResponse struct {
	Similarity float64 `json:"similarity"`
	IsMatch    bool    `json:"isMatch"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Embedding []float32 `json:"embedding" binding:"required"`
	Threshold float64   `json:"threshold"`
	TopK      int       `json:"topK"`
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Results []FaceSearchResult `json:"results"`
}

// ==================== 处理器方法 ====================

// Detect 人脸检测
// @Summary 检测图像中的人脸
// @Tags face
// @Accept json
// @Produce json
// @Param request body DetectRequest true "检测请求"
// @Success 200 {object} DetectResponse
// @Router /face/detect [post]
func (h *Handlers) Detect(c *gin.Context) {
	var req DetectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 解码图像
	img, err := decodeBase64Image(req.Image, req.Format)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图像解码失败: " + err.Error()})
		return
	}

	// 检测人脸
	ctx := context.Background()
	adapter := NewGoImageAdapter(img)
	result, err := h.service.DetectFaces(ctx, adapter, req.PhotoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, DetectResponse{
		Faces:     result.Faces,
		Count:     len(result.Faces),
		ProcessMs: result.ProcessMs,
	})
}

// Recognize 人脸识别
// @Summary 检测并识别图像中的人脸
// @Tags face
// @Accept json
// @Produce json
// @Param request body RecognizeRequest true "识别请求"
// @Success 200 {object} RecognizeResponse
// @Router /face/recognize [post]
func (h *Handlers) Recognize(c *gin.Context) {
	var req RecognizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 解码图像
	img, err := decodeBase64Image(req.Image, req.Format)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图像解码失败: " + err.Error()})
		return
	}

	// 识别人脸
	ctx := context.Background()
	start := time.Now()
	faces, err := h.service.RecognizeFaces(ctx, NewGoImageAdapter(img), req.PhotoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, RecognizeResponse{
		Faces:     faces,
		Count:     len(faces),
		ProcessMs: time.Since(start).Milliseconds(),
	})
}

// BatchRecognize 批量识别
// @Summary 批量识别图像中的人脸
// @Tags face
// @Accept json
// @Produce json
// @Param images body map[string]string true "图像映射 (photoId -> base64)"
// @Success 200 {object} map[string][]Face
// @Router /face/batch [post]
func (h *Handlers) BatchRecognize(c *gin.Context) {
	var req map[string]string // photoId -> base64 image
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 解码图像
	images := make(map[string]image.Image)
	for photoID, base64Img := range req {
		img, err := decodeBase64Image(base64Img, "")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("图像 %s 解码失败: %v", photoID, err)})
			return
		}
		images[photoID] = img
	}

	// 批量处理
	ctx := context.Background()
	results, err := h.service.BatchProcessImages(ctx, images)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, results)
}

// ListPersons 列出人物
// @Summary 列出所有人物
// @Tags face
// @Produce json
// @Param limit query int false "限制数量" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /face/persons [get]
func (h *Handlers) ListPersons(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	ctx := context.Background()
	persons, total, err := h.service.ListPersons(ctx, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"persons": persons,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
	})
}

// CreatePerson 创建人物
// @Summary 创建人物
// @Tags face
// @Accept json
// @Produce json
// @Param request body CreatePersonRequest true "创建请求"
// @Success 200 {object} Person
// @Router /face/persons [post]
func (h *Handlers) CreatePerson(c *gin.Context) {
	var req CreatePersonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	person, err := h.service.CreatePerson(ctx, req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, person)
}

// GetPerson 获取人物
// @Summary 获取人物详情
// @Tags face
// @Produce json
// @Param id path string true "人物ID"
// @Success 200 {object} Person
// @Router /face/persons/{id} [get]
func (h *Handlers) GetPerson(c *gin.Context) {
	personID := c.Param("id")

	ctx := context.Background()
	person, err := h.service.GetPerson(ctx, personID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, person)
}

// UpdatePerson 更新人物
// @Summary 更新人物
// @Tags face
// @Accept json
// @Produce json
// @Param id path string true "人物ID"
// @Param request body UpdatePersonRequest true "更新请求"
// @Success 200 {object} Person
// @Router /face/persons/{id} [put]
func (h *Handlers) UpdatePerson(c *gin.Context) {
	personID := c.Param("id")

	var req UpdatePersonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.CoverFaceID != "" {
		updates["coverFaceId"] = req.CoverFaceID
	}

	ctx := context.Background()
	if err := h.service.UpdatePerson(ctx, personID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	person, _ := h.service.GetPerson(ctx, personID)
	c.JSON(http.StatusOK, person)
}

// DeletePerson 删除人物
// @Summary 删除人物
// @Tags face
// @Param id path string true "人物ID"
// @Success 200 {object} map[string]interface{}
// @Router /face/persons/{id} [delete]
func (h *Handlers) DeletePerson(c *gin.Context) {
	personID := c.Param("id")

	ctx := context.Background()
	if err := h.service.DeletePerson(ctx, personID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetPhotoFaces 获取照片的人脸
// @Summary 获取照片中的所有人脸
// @Tags face
// @Produce json
// @Param photoId path string true "照片ID"
// @Success 200 {object} map[string]interface{}
// @Router /face/photos/{photoId}/faces [get]
func (h *Handlers) GetPhotoFaces(c *gin.Context) {
	photoID := c.Param("photoId")

	ctx := context.Background()
	faces, err := h.service.GetFacesByPhoto(ctx, photoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"photoId": photoID,
		"faces":   faces,
		"count":   len(faces),
	})
}

// GetPersonFaces 获取人物的人脸
// @Summary 获取人物的所有人脸
// @Tags face
// @Produce json
// @Param personId path string true "人物ID"
// @Success 200 {object} map[string]interface{}
// @Router /face/persons/{personId}/faces [get]
func (h *Handlers) GetPersonFaces(c *gin.Context) {
	personID := c.Param("personId")

	ctx := context.Background()
	faces, err := h.service.GetFacesByPerson(ctx, personID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"personId": personID,
		"faces":    faces,
		"count":    len(faces),
	})
}

// AssignFace 分配人脸
// @Summary 将人脸分配给人物
// @Tags face
// @Accept json
// @Param faceId path string true "人脸ID"
// @Param request body AssignFaceRequest true "分配请求"
// @Success 200 {object} map[string]interface{}
// @Router /face/faces/{faceId}/assign [post]
func (h *Handlers) AssignFace(c *gin.Context) {
	faceID := c.Param("faceId")

	var req AssignFaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	if err := h.service.AssignFace(ctx, faceID, req.PersonID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "分配成功"})
}

// UnassignFace 取消分配
// @Summary 取消人脸分配
// @Tags face
// @Param faceId path string true "人脸ID"
// @Success 200 {object} map[string]interface{}
// @Router /face/faces/{faceId}/unassign [post]
func (h *Handlers) UnassignFace(c *gin.Context) {
	faceID := c.Param("faceId")

	ctx := context.Background()
	if err := h.service.UnassignFace(ctx, faceID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "取消分配成功"})
}

// ClusterFaces 聚类人脸
// @Summary 对人脸进行聚类
// @Tags face
// @Accept json
// @Produce json
// @Param request body ClusterRequest true "聚类请求"
// @Success 200 {object} ClusterResult
// @Router /face/cluster [post]
func (h *Handlers) ClusterFaces(c *gin.Context) {
	var req ClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	result, err := h.service.ClusterFaces(ctx, req.PhotoIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// AutoLabel 自动标记
// @Summary 自动标记人脸
// @Tags face
// @Accept json
// @Param request body AutoLabelRequest true "自动标记请求"
// @Success 200 {object} map[string]interface{}
// @Router /face/auto-label [post]
func (h *Handlers) AutoLabel(c *gin.Context) {
	var req AutoLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	if err := h.service.AutoLabel(ctx, req.PersonID, req.PhotoIDs); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "自动标记完成"})
}

// GetStats 获取统计
// @Summary 获取人脸识别统计
// @Tags face
// @Produce json
// @Success 200 {object} Stats
// @Router /face/stats [get]
func (h *Handlers) GetStats(c *gin.Context) {
	ctx := context.Background()
	stats, err := h.service.GetStats(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// CompareFaces 比较人脸
// @Summary 比较两个人脸的相似度
// @Tags face
// @Accept json
// @Produce json
// @Param request body CompareRequest true "比较请求"
// @Success 200 {object} CompareResponse
// @Router /face/compare [post]
func (h *Handlers) CompareFaces(c *gin.Context) {
	var req CompareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	similarity := h.service.CompareFaces(req.Embedding1, req.Embedding2)
	isMatch := similarity >= h.service.config.ClusterThresh

	c.JSON(http.StatusOK, CompareResponse{
		Similarity: similarity,
		IsMatch:    isMatch,
	})
}

// SearchSimilarFaces 搜索相似人脸
// @Summary 搜索相似人脸
// @Tags face
// @Accept json
// @Produce json
// @Param request body SearchRequest true "搜索请求"
// @Success 200 {object} SearchResponse
// @Router /face/search [post]
func (h *Handlers) SearchSimilarFaces(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	threshold := req.Threshold
	if threshold == 0 {
		threshold = h.service.config.ClusterThresh
	}

	topK := req.TopK
	if topK == 0 {
		topK = 10
	}

	ctx := context.Background()
	results := h.service.SearchSimilarFaces(ctx, req.Embedding, threshold, topK)

	c.JSON(http.StatusOK, SearchResponse{Results: results})
}

// GetConfig 获取配置
// @Summary 获取人脸识别配置
// @Tags face
// @Produce json
// @Success 200 {object} RecognitionConfig
// @Router /face/config [get]
func (h *Handlers) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.service.config)
}

// UpdateConfig 更新配置
// @Summary 更新人脸识别配置
// @Tags face
// @Accept json
// @Param config body RecognitionConfig true "配置"
// @Success 200 {object} map[string]interface{}
// @Router /face/config [put]
func (h *Handlers) UpdateConfig(c *gin.Context) {
	var config RecognitionConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 部分更新配置
	if config.MinFaceSize > 0 {
		h.service.config.MinFaceSize = config.MinFaceSize
	}
	if config.MaxFacesPerPhoto > 0 {
		h.service.config.MaxFacesPerPhoto = config.MaxFacesPerPhoto
	}
	if config.ConfidenceThresh > 0 {
		h.service.config.ConfidenceThresh = config.ConfidenceThresh
	}
	if config.ClusterThresh > 0 {
		h.service.config.ClusterThresh = config.ClusterThresh
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置更新成功"})
}

// ==================== 辅助函数 ====================

// decodeBase64Image 解码Base64图像
func decodeBase64Image(base64Str, format string) (image.Image, error) {
	// 移除data URL前缀
	if len(base64Str) > 22 && base64Str[:5] == "data:" {
		// 找到base64数据的起始位置
		for i := 0; i < len(base64Str)-1; i++ {
			if base64Str[i] == ',' {
				base64Str = base64Str[i+1:]
				break
			}
		}
	}

	data, err := base64.StdEncoding.DecodeString(base64Str)
	if err != nil {
		return nil, fmt.Errorf("base64解码失败: %w", err)
	}

	// 根据格式解码
	switch format {
	case "png":
		return png.Decode(bytes.NewReader(data))
	case "jpeg", "jpg":
		return jpeg.Decode(bytes.NewReader(data))
	default:
		// 尝试JPEG
		img, err := jpeg.Decode(bytes.NewReader(data))
		if err == nil {
			return img, nil
		}
		// 尝试PNG
		return png.Decode(bytes.NewReader(data))
	}
}