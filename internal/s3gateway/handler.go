package s3gateway

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler S3网关HTTP处理器.
type Handler struct {
	gw *Gateway
}

// NewHandler 创建处理器.
func NewHandler(gw *Gateway) *Handler {
	return &Handler{gw: gw}
}

// RegisterRoutes 注册路由到 /api/v1/s3.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	s3 := rg.Group("/s3")
	{
		s3.GET("/buckets", h.ListBuckets)
		s3.POST("/buckets", h.CreateBucket)
		s3.DELETE("/buckets/:name", h.DeleteBucket)
		s3.GET("/buckets/:name/objects", h.ListObjects)
		s3.PUT("/buckets/:name/objects/:key", h.PutObject)
		s3.GET("/buckets/:name/objects/:key", h.GetObject)
		s3.HEAD("/buckets/:name/objects/:key", h.HeadObject)
		s3.DELETE("/buckets/:name/objects/:key", h.DeleteObject)
		s3.GET("/stats", h.GetStats)
		s3.GET("/config", h.GetConfig)
	}
}

// CreateBucketRequest 创建桶请求.
type CreateBucketRequest struct {
	Name   string       `json:"name" binding:"required"`
	Policy BucketPolicy `json:"policy"`
	Quota  BucketQuota  `json:"quota"`
}

// ListBuckets GET /buckets.
func (h *Handler) ListBuckets(c *gin.Context) {
	userID := c.GetString("userId")
	if userID == "" {
		userID = c.Query("userId")
	}
	buckets := h.gw.ListBuckets(userID)
	c.JSON(http.StatusOK, gin.H{
		"buckets": buckets,
		"count":   len(buckets),
	})
}

// CreateBucket POST /buckets.
func (h *Handler) CreateBucket(c *gin.Context) {
	var req CreateBucketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetString("userId")
	if userID == "" {
		userID = c.Query("userId")
	}
	bucket, err := h.gw.CreateBucket(req.Name, userID, req.Policy, req.Quota)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, bucket)
}

// DeleteBucket DELETE /buckets/:name.
func (h *Handler) DeleteBucket(c *gin.Context) {
	name := c.Param("name")
	userID := c.GetString("userId")
	if userID == "" {
		userID = c.Query("userId")
	}
	if err := h.gw.DeleteBucket(name, userID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "bucket deleted"})
}

// ListObjects GET /buckets/:name/objects.
func (h *Handler) ListObjects(c *gin.Context) {
	bucketName := c.Param("name")
	userID := c.GetString("userId")
	if userID == "" {
		userID = c.Query("userId")
	}
	prefix := c.Query("prefix")
	maxKeys, _ := strconv.Atoi(c.DefaultQuery("maxKeys", "1000"))

	objects, truncated, err := h.gw.ListObjects(bucketName, prefix, userID, maxKeys)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"objects":   objects,
		"count":     len(objects),
		"truncated": truncated,
	})
}

// PutObject PUT /buckets/:name/objects/:key.
func (h *Handler) PutObject(c *gin.Context) {
	bucketName := c.Param("name")
	key := c.Param("key")
	userID := c.GetString("userId")
	if userID == "" {
		userID = c.Query("userId")
	}

	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read body"})
		return
	}

	contentType := c.GetHeader("Content-Type")
	if contentType == "" {
		contentType = c.ContentType()
	}

	// 解析自定义元数据 (X-Amz-Meta-*)
	metadata := make(map[string]string)
	for k, vals := range c.Request.Header {
		if len(k) > 13 && k[:13] == "X-Amz-Meta-" {
			metadata[k[13:]] = vals[0]
		}
	}

	// 解析标签
	tags := make(map[string]string)
	tagHeader := c.GetHeader("X-Amz-Tagging")
	if tagHeader != "" {
		tags["tagging"] = tagHeader
	}

	obj, err := h.gw.PutObject(bucketName, key, userID, data, contentType, metadata, tags)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.Header("ETag", obj.ETag)
	c.JSON(http.StatusOK, gin.H{
		"key":  obj.Key,
		"etag": obj.ETag,
		"size": obj.Size,
	})
}

// GetObject GET /buckets/:name/objects/:key.
func (h *Handler) GetObject(c *gin.Context) {
	bucketName := c.Param("name")
	key := c.Param("key")
	userID := c.GetString("userId")
	if userID == "" {
		userID = c.Query("userId")
	}

	obj, err := h.gw.GetObject(bucketName, key, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", obj.ContentType)
	c.Header("Content-Length", strconv.FormatInt(obj.Size, 10))
	c.Header("ETag", obj.ETag)
	for k, v := range obj.Metadata {
		c.Header("X-Amz-Meta-"+k, v)
	}
	c.Data(http.StatusOK, obj.ContentType, obj.Data)
}

// HeadObject HEAD /buckets/:name/objects/:key.
func (h *Handler) HeadObject(c *gin.Context) {
	bucketName := c.Param("name")
	key := c.Param("key")
	userID := c.GetString("userId")
	if userID == "" {
		userID = c.Query("userId")
	}

	obj, err := h.gw.HeadObject(bucketName, key, userID)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", obj.ContentType)
	c.Header("Content-Length", strconv.FormatInt(obj.Size, 10))
	c.Header("ETag", obj.ETag)
	c.Header("X-Amz-Storage-Class", string(obj.StorageClass))
	c.Status(http.StatusOK)
}

// DeleteObject DELETE /buckets/:name/objects/:key.
func (h *Handler) DeleteObject(c *gin.Context) {
	bucketName := c.Param("name")
	key := c.Param("key")
	userID := c.GetString("userId")
	if userID == "" {
		userID = c.Query("userId")
	}

	if err := h.gw.DeleteObject(bucketName, key, userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "object deleted"})
}

// GetStats GET /stats.
func (h *Handler) GetStats(c *gin.Context) {
	userID := c.Query("userId")
	stats := h.gw.GetStats(userID)
	c.JSON(http.StatusOK, stats)
}

// GetConfig GET /config.
func (h *Handler) GetConfig(c *gin.Context) {
	config := h.gw.GetConfig()
	c.JSON(http.StatusOK, config)
}
