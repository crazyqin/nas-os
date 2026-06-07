// Package objectimmutable 提供 S3 兼容的不可变对象存储功能
package objectimmutable

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers Object Immutable HTTP 处理器.
type Handlers struct {
	logger *zap.Logger
	mgr    *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(logger *zap.Logger, mgr *Manager) *Handlers {
	return &Handlers{
		logger: logger,
		mgr:    mgr,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	oi := rg.Group("/object-immutable")
	{
		// 桶管理
		oi.POST("/buckets", h.createBucket)
		oi.GET("/buckets", h.listBuckets)
		oi.GET("/buckets/:name", h.getBucket)
		oi.PUT("/buckets/:name/lock", h.setBucketObjectLock)
		oi.GET("/buckets/:name/lock", h.getBucketObjectLock)

		// 对象管理
		oi.POST("/buckets/:bucket/objects", h.putObject)
		oi.GET("/buckets/:bucket/objects/:key", h.getObject)
		oi.DELETE("/buckets/:bucket/objects/:key", h.deleteObject)
		oi.GET("/buckets/:bucket/objects", h.listObjects)

		// 保留管理
		oi.PUT("/buckets/:bucket/objects/:key/retention", h.setObjectRetention)
		oi.GET("/buckets/:bucket/objects/:key/retention", h.getObjectRetention)
		oi.DELETE("/buckets/:bucket/objects/:key/retention", h.releaseObjectRetention)

		// 法律保留管理
		oi.PUT("/buckets/:bucket/objects/:key/legal-hold", h.setObjectLegalHold)
		oi.GET("/buckets/:bucket/objects/:key/legal-hold", h.getObjectLegalHold)

		// 审计日志
		oi.GET("/audit-logs", h.listAuditLogs)

		// 统计
		oi.GET("/stats", h.getStats)

		// 配置持久化
		oi.POST("/save", h.saveConfig)
	}
}

// ========== 桶管理 Handlers ==========

// createBucket 创建桶.
func (h *Handlers) createBucket(c *gin.Context) {
	var req struct {
		Name             string                   `json:"name" binding:"required"`
		DefaultImmutable bool                     `json:"default_immutable"`
		ObjectLockConfig *ObjectLockConfiguration `json:"object_lock_config,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	bucket, err := h.mgr.CreateBucket(req.Name, req.DefaultImmutable, req.ObjectLockConfig)
	if err != nil {
		if err == ErrBucketExists {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, bucket)
}

// listBuckets 列出桶.
func (h *Handlers) listBuckets(c *gin.Context) {
	buckets := h.mgr.ListBuckets()
	c.JSON(http.StatusOK, gin.H{"buckets": buckets})
}

// getBucket 获取桶.
func (h *Handlers) getBucket(c *gin.Context) {
	name := c.Param("name")

	bucket, err := h.mgr.GetBucket(name)
	if err != nil {
		if err == ErrBucketNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bucket)
}

// setBucketObjectLock 设置桶对象锁定配置.
func (h *Handlers) setBucketObjectLock(c *gin.Context) {
	name := c.Param("name")

	var req PutBucketObjectLockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	bucket, err := h.mgr.SetBucketObjectLockConfig(name, &req.ObjectLockConfig)
	if err != nil {
		if err == ErrBucketNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, bucket)
}

// getBucketObjectLock 获取桶对象锁定配置.
func (h *Handlers) getBucketObjectLock(c *gin.Context) {
	name := c.Param("name")

	config, err := h.mgr.GetBucketObjectLockConfig(name)
	if err != nil {
		if err == ErrBucketNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetBucketObjectLockResponse{
		ObjectLockConfig: config,
	})
}

// ========== 对象管理 Handlers ==========

// putObject 上传对象.
func (h *Handlers) putObject(c *gin.Context) {
	bucketName := c.Param("bucket")
	objectKey := c.Param("key")

	// 从 S3 兼容头获取内容类型
	contentType := c.GetHeader("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// 读取请求体
	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取请求体失败: " + err.Error()})
		return
	}

	obj, err := h.mgr.PutObject(bucketName, objectKey, data, contentType)
	if err != nil {
		if err == ErrBucketNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err == ErrObjectLocked || err == ErrLegalHoldActive {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// S3 兼容响应头
	c.Header("ETag", "\""+obj.ETag+"\"")
	c.Header("x-amz-version-id", "null")

	c.JSON(http.StatusOK, obj)
}

// getObject 获取对象.
func (h *Handlers) getObject(c *gin.Context) {
	bucketName := c.Param("bucket")
	objectKey := c.Param("key")

	obj, err := h.mgr.GetObject(bucketName, objectKey)
	if err != nil {
		if err == ErrBucketNotFound || err == ErrObjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// S3 兼容响应头
	c.Header("ETag", "\""+obj.ETag+"\"")
	c.Header("Content-Type", obj.ContentType)
	c.Header("Content-Length", strconv.FormatInt(obj.Size, 10))

	if obj.Retention != nil {
		c.Header("x-amz-object-lock-mode", string(obj.Retention.Mode))
		c.Header("x-amz-object-lock-retain-until-date", obj.Retention.RetainUntilDate.Format(time.RFC3339))
	}

	if obj.LegalHold != nil && obj.LegalHold.Enabled {
		c.Header("x-amz-object-lock-legal-hold", "ON")
	}

	c.Data(http.StatusOK, obj.ContentType, obj.Data)
}

// deleteObject 删除对象.
func (h *Handlers) deleteObject(c *gin.Context) {
	bucketName := c.Param("bucket")
	objectKey := c.Param("key")

	// S3 兼容头
	bypassGovernance := c.GetHeader("x-amz-bypass-governance-retention") == "true"
	operator := c.GetHeader("x-amz-operator")
	ipAddress := c.ClientIP()

	err := h.mgr.DeleteObject(bucketName, objectKey, operator, ipAddress, bypassGovernance)
	if err != nil {
		if err == ErrBucketNotFound || err == ErrObjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err == ErrObjectLocked || err == ErrLegalHoldActive || err == ErrWORMViolation {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// listObjects 列出对象.
func (h *Handlers) listObjects(c *gin.Context) {
	bucketName := c.Param("bucket")

	req := ListObjectsRequest{
		BucketName: bucketName,
	}

	if prefix := c.Query("prefix"); prefix != "" {
		req.Prefix = prefix
	}

	if maxKeys := c.Query("max-keys"); maxKeys != "" {
		if n, err := strconv.Atoi(maxKeys); err == nil {
			req.MaxKeys = n
		}
	}

	if token := c.Query("continuation-token"); token != "" {
		req.ContinuationToken = token
	}

	resp, err := h.mgr.ListObjects(req)
	if err != nil {
		if err == ErrBucketNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ========== 保留管理 Handlers ==========

// setObjectRetention 设置对象保留.
func (h *Handlers) setObjectRetention(c *gin.Context) {
	bucketName := c.Param("bucket")
	objectKey := c.Param("key")

	var req PutObjectRetentionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	// S3 兼容头
	if c.GetHeader("x-amz-bypass-governance-retention") == "true" {
		req.BypassGovernance = true
	}

	operator := c.GetHeader("x-amz-operator")
	ipAddress := c.ClientIP()

	retention, err := h.mgr.SetObjectRetention(bucketName, objectKey, req, operator, ipAddress)
	if err != nil {
		if err == ErrBucketNotFound || err == ErrObjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err == ErrInvalidLockMode || err == ErrInvalidRetention {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err == ErrWORMViolation {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// S3 兼容响应头
	c.Header("x-amz-object-lock-mode", string(retention.Mode))
	c.Header("x-amz-object-lock-retain-until-date", retention.RetainUntilDate.Format(time.RFC3339))

	c.JSON(http.StatusOK, PutObjectRetentionResponse{
		Retention: retention,
	})
}

// getObjectRetention 获取对象保留.
func (h *Handlers) getObjectRetention(c *gin.Context) {
	bucketName := c.Param("bucket")
	objectKey := c.Param("key")

	retention, err := h.mgr.GetObjectRetention(bucketName, objectKey)
	if err != nil {
		if err == ErrBucketNotFound || err == ErrObjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetObjectRetentionResponse{
		Retention: retention,
	})
}

// releaseObjectRetention 释放对象保留.
func (h *Handlers) releaseObjectRetention(c *gin.Context) {
	bucketName := c.Param("bucket")
	objectKey := c.Param("key")

	operator := c.GetHeader("x-amz-operator")
	ipAddress := c.ClientIP()

	err := h.mgr.ReleaseObjectRetention(bucketName, objectKey, operator, ipAddress)
	if err != nil {
		if err == ErrBucketNotFound || err == ErrObjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		if err == ErrWORMViolation {
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
			return
		}
		if err == ErrRetentionExpired {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "保留已释放"})
}

// ========== 法律保留 Handlers ==========

// setObjectLegalHold 设置对象法律保留.
func (h *Handlers) setObjectLegalHold(c *gin.Context) {
	bucketName := c.Param("bucket")
	objectKey := c.Param("key")

	var req PutObjectLegalHoldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数错误: " + err.Error()})
		return
	}

	operator := c.GetHeader("x-amz-operator")
	ipAddress := c.ClientIP()

	legalHold, err := h.mgr.SetObjectLegalHold(bucketName, objectKey, req, operator, ipAddress)
	if err != nil {
		if err == ErrBucketNotFound || err == ErrObjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// S3 兼容响应头
	if legalHold.Enabled {
		c.Header("x-amz-object-lock-legal-hold", "ON")
	} else {
		c.Header("x-amz-object-lock-legal-hold", "OFF")
	}

	c.JSON(http.StatusOK, PutObjectLegalHoldResponse{
		LegalHold: legalHold,
	})
}

// getObjectLegalHold 获取对象法律保留.
func (h *Handlers) getObjectLegalHold(c *gin.Context) {
	bucketName := c.Param("bucket")
	objectKey := c.Param("key")

	legalHold, err := h.mgr.GetObjectLegalHold(bucketName, objectKey)
	if err != nil {
		if err == ErrBucketNotFound || err == ErrObjectNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, GetObjectLegalHoldResponse{
		LegalHold: legalHold,
	})
}

// ========== 审计日志 Handlers ==========

// listAuditLogs 列出审计日志.
func (h *Handlers) listAuditLogs(c *gin.Context) {
	req := ListAuditLogsRequest{}

	if objectKey := c.Query("object_key"); objectKey != "" {
		req.ObjectKey = objectKey
	}

	if bucketName := c.Query("bucket_name"); bucketName != "" {
		req.BucketName = bucketName
	}

	if action := c.Query("action"); action != "" {
		req.Action = AuditAction(action)
	}

	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			req.StartTime = &t
		}
	}

	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			req.EndTime = &t
		}
	}

	if maxResults := c.Query("max_results"); maxResults != "" {
		if n, err := strconv.Atoi(maxResults); err == nil {
			req.MaxResults = n
		}
	}

	resp := h.mgr.ListAuditLogs(req)
	c.JSON(http.StatusOK, resp)
}

// ========== 统计 Handlers ==========

// getStats 获取统计信息.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.mgr.GetStats()
	c.JSON(http.StatusOK, stats)
}

// ========== 配置持久化 Handlers ==========

// saveConfig 保存配置.
func (h *Handlers) saveConfig(c *gin.Context) {
	if err := h.mgr.SaveConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存配置失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "配置已保存"})
}
