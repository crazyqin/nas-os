// Package s3 provides HTTP handlers for enhanced S3 API
// This file implements handlers for bucket policy, object lock, lifecycle, and versioning.
package s3

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// PolicyHandlers provides HTTP handlers for S3 policy and management APIs.
type PolicyHandlers struct {
	manager       *Manager
	policyManager *PolicyManager
}

// NewPolicyHandlers creates new policy and management handlers.
func NewPolicyHandlers(manager *Manager) *PolicyHandlers {
	return &PolicyHandlers{
		manager:       manager,
		policyManager: NewPolicyManager(),
	}
}

// RegisterRoutes registers the enhanced S3 API routes.
func (ph *PolicyHandlers) RegisterRoutes(r *gin.Engine) {
	// Bucket Policy endpoints
	r.PUT("/api/v1/s3/buckets/:bucket/policy", ph.setBucketPolicy)
	r.GET("/api/v1/s3/buckets/:bucket/policy", ph.getBucketPolicy)
	r.DELETE("/api/v1/s3/buckets/:bucket/policy", ph.deleteBucketPolicy)

	// Object Lock endpoints
	r.PUT("/api/v1/s3/buckets/:bucket/object-lock", ph.setObjectLock)
	r.GET("/api/v1/s3/buckets/:bucket/object-lock", ph.getObjectLock)
	r.GET("/api/v1/s3/buckets/:bucket/object-lock/status", ph.getObjectLockStatus)

	// Lifecycle endpoints
	r.PUT("/api/v1/s3/buckets/:bucket/lifecycle", ph.setLifecycle)
	r.GET("/api/v1/s3/buckets/:bucket/lifecycle", ph.getLifecycle)
	r.GET("/api/v1/s3/buckets/:bucket/lifecycle/status", ph.getLifecycleStatus)
	r.POST("/api/v1/s3/buckets/:bucket/lifecycle/apply", ph.applyLifecycleRules)

	// Versioning endpoints
	r.PUT("/api/v1/s3/buckets/:bucket/versioning", ph.setVersioning)
	r.GET("/api/v1/s3/buckets/:bucket/versioning", ph.getVersioning)
	r.GET("/api/v1/s3/buckets/:bucket/versions", ph.listVersions)

	// Object retention and legal hold endpoints
	r.POST("/api/v1/s3/buckets/:bucket/objects/:key/retention", ph.setObjectRetention)
	r.GET("/api/v1/s3/buckets/:bucket/objects/:key/retention", ph.getObjectRetention)
	r.POST("/api/v1/s3/buckets/:bucket/objects/:key/legal-hold", ph.setObjectLegalHold)
	r.GET("/api/v1/s3/buckets/:bucket/objects/:key/legal-hold", ph.getObjectLegalHold)
}

// setBucketPolicy sets or updates the bucket policy.
func (ph *PolicyHandlers) setBucketPolicy(c *gin.Context) {
	bucketName := c.Param("bucket")

	var policy BucketPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		ph.sendError(c, http.StatusBadRequest, "InvalidPolicy", "Invalid policy JSON: "+err.Error())
		return
	}

	// Validate policy
	if err := ValidatePolicy(&policy); err != nil {
		ph.sendError(c, http.StatusBadRequest, "InvalidPolicy", err.Error())
		return
	}

	if err := ph.manager.SetBucketPolicy(bucketName, &policy); err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Bucket policy set successfully",
		"bucket":  bucketName,
	})
}

// getBucketPolicy retrieves the bucket policy.
func (ph *PolicyHandlers) getBucketPolicy(c *gin.Context) {
	bucketName := c.Param("bucket")

	policy, err := ph.manager.GetPolicy(bucketName)
	if err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, policy)
}

// deleteBucketPolicy removes the bucket policy.
func (ph *PolicyHandlers) deleteBucketPolicy(c *gin.Context) {
	bucketName := c.Param("bucket")

	if err := ph.manager.DeletePolicy(bucketName); err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Bucket policy deleted successfully",
		"bucket":  bucketName,
	})
}

// setObjectLock sets the object lock configuration for a bucket.
func (ph *PolicyHandlers) setObjectLock(c *gin.Context) {
	bucketName := c.Param("bucket")

	var config ObjectLockConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		ph.sendError(c, http.StatusBadRequest, "InvalidRequest", "Invalid object lock configuration: "+err.Error())
		return
	}

	if err := ph.manager.SetObjectLockConfig(bucketName, &config); err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Object lock configuration set successfully",
		"bucket":  bucketName,
		"enabled": config.Enabled,
	})
}

// getObjectLock retrieves the object lock configuration for a bucket.
func (ph *PolicyHandlers) getObjectLock(c *gin.Context) {
	bucketName := c.Param("bucket")

	config, err := ph.manager.GetObjectLockConfig(bucketName)
	if err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, config)
}

// getObjectLockStatus returns the object lock status summary for a bucket.
func (ph *PolicyHandlers) getObjectLockStatus(c *gin.Context) {
	bucketName := c.Param("bucket")

	status, err := ph.manager.GetObjectLockStatus(bucketName)
	if err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, status)
}

// setLifecycle sets the lifecycle configuration for a bucket.
func (ph *PolicyHandlers) setLifecycle(c *gin.Context) {
	bucketName := c.Param("bucket")

	var config LifecycleConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		ph.sendError(c, http.StatusBadRequest, "InvalidRequest", "Invalid lifecycle configuration: "+err.Error())
		return
	}

	if err := ph.manager.SetLifecycleConfig(bucketName, &config); err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Lifecycle configuration set successfully",
		"bucket":    bucketName,
		"ruleCount": len(config.Rules),
	})
}

// getLifecycle retrieves the lifecycle configuration for a bucket.
func (ph *PolicyHandlers) getLifecycle(c *gin.Context) {
	bucketName := c.Param("bucket")

	config, err := ph.manager.GetLifecycleConfig(bucketName)
	if err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, config)
}

// getLifecycleStatus returns the lifecycle management status for a bucket.
func (ph *PolicyHandlers) getLifecycleStatus(c *gin.Context) {
	bucketName := c.Param("bucket")

	status, err := ph.manager.GetLifecycleStatus(bucketName)
	if err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, status)
}

// applyLifecycleRules triggers immediate application of lifecycle rules.
func (ph *PolicyHandlers) applyLifecycleRules(c *gin.Context) {
	bucketName := c.Param("bucket")

	result, err := ph.manager.ApplyLifecycleRules(bucketName)
	if err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "Lifecycle rules applied successfully",
		"bucket":       bucketName,
		"expired":      result.Expired,
		"transitioned": result.Transitioned,
		"errors":       result.Errors,
	})
}

// setVersioning sets the versioning configuration for a bucket.
func (ph *PolicyHandlers) setVersioning(c *gin.Context) {
	bucketName := c.Param("bucket")

	var config VersioningConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		ph.sendError(c, http.StatusBadRequest, "InvalidRequest", "Invalid versioning configuration: "+err.Error())
		return
	}

	// Validate status
	if config.Status != VersioningEnabled && config.Status != VersioningSuspended {
		ph.sendError(c, http.StatusBadRequest, "InvalidVersioningStatus",
			"Status must be 'Enabled' or 'Suspended'")
		return
	}

	if err := ph.manager.SetBucketVersioning(bucketName, config); err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Versioning configuration set successfully",
		"bucket":  bucketName,
		"status":  config.Status,
	})
}

// getVersioning returns the versioning configuration and status for a bucket.
func (ph *PolicyHandlers) getVersioning(c *gin.Context) {
	bucketName := c.Param("bucket")

	status, err := ph.manager.GetVersioningStatus(bucketName)
	if err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, status)
}

// listVersions lists all versions of objects in a bucket.
func (ph *PolicyHandlers) listVersions(c *gin.Context) {
	bucketName := c.Param("bucket")
	prefix := c.Query("prefix")
	delimiter := c.Query("delimiter")
	keyMarker := c.Query("key-marker")
	versionIDMarker := c.Query("version-id-marker")
	maxKeys := 1000
	if v := c.Query("max-keys"); v != "" {
		if n, err := parseMaxKeys(v); err == nil && n > 0 {
			maxKeys = n
		}
	}

	versions, err := ph.manager.ListVersions(bucketName, prefix, delimiter, keyMarker, versionIDMarker, maxKeys)
	if err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, versions)
}

// setObjectRetention sets the retention configuration for a specific object.
func (ph *PolicyHandlers) setObjectRetention(c *gin.Context) {
	bucketName := c.Param("bucket")
	key := c.Param("key")

	var retention ObjectRetention
	if err := c.ShouldBindJSON(&retention); err != nil {
		ph.sendError(c, http.StatusBadRequest, "InvalidRequest", "Invalid retention configuration: "+err.Error())
		return
	}

	if err := ph.manager.SetRetention(bucketName, key, &retention); err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":         "Retention set successfully",
		"bucket":          bucketName,
		"key":             key,
		"mode":            retention.Mode,
		"retainUntilDate": retention.RetainUntilDate,
	})
}

// getObjectRetention retrieves the retention configuration for a specific object.
func (ph *PolicyHandlers) getObjectRetention(c *gin.Context) {
	bucketName := c.Param("bucket")
	key := c.Param("key")

	retention, err := ph.manager.GetRetention(bucketName, key)
	if err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"mode":            retention.Mode,
		"retainUntilDate": retention.RetainUntilDate,
	})
}

// setObjectLegalHold sets the legal hold for a specific object.
func (ph *PolicyHandlers) setObjectLegalHold(c *gin.Context) {
	bucketName := c.Param("bucket")
	key := c.Param("key")

	var req struct {
		Status LegalHold `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ph.sendError(c, http.StatusBadRequest, "InvalidRequest", "Invalid legal hold request: "+err.Error())
		return
	}

	if req.Status != LegalHoldOn && req.Status != LegalHoldOff {
		ph.sendError(c, http.StatusBadRequest, "InvalidLegalHoldStatus",
			"Status must be 'ON' or 'OFF'")
		return
	}

	if err := ph.manager.SetLegalHold(bucketName, key, req.Status); err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Legal hold set successfully",
		"bucket":  bucketName,
		"key":     key,
		"status":  req.Status,
	})
}

// getObjectLegalHold retrieves the legal hold status for a specific object.
func (ph *PolicyHandlers) getObjectLegalHold(c *gin.Context) {
	bucketName := c.Param("bucket")
	key := c.Param("key")

	hold, err := ph.manager.GetLegalHold(bucketName, key)
	if err != nil {
		ph.sendManagerError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": hold,
	})
}

// sendError sends a standardized error response.
func (ph *PolicyHandlers) sendError(c *gin.Context, code int, codeStr, message string) {
	c.JSON(code, gin.H{
		"error": gin.H{
			"code":    codeStr,
			"message": message,
		},
	})
}

// sendManagerError converts manager errors to HTTP responses.
func (ph *PolicyHandlers) sendManagerError(c *gin.Context, err error) {
	if s3Err, ok := err.(*S3Error); ok {
		c.JSON(s3Err.Code, gin.H{
			"error": gin.H{
				"code":     s3Err.CodeStr,
				"message":  s3Err.Message,
				"resource": s3Err.Resource,
			},
		})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{
		"error": gin.H{
			"code":    "InternalError",
			"message": err.Error(),
		},
	})
}

// parseMaxKeys parses max-keys query parameter.
func parseMaxKeys(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 1000, nil
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, nil
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
