// Package s3 provides HTTP handlers for S3 API
package s3

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers provides S3 API HTTP handlers.
type Handlers struct {
	manager *Manager
}

// NewHandlers creates new S3 handlers.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes registers S3 API routes.
func (h *Handlers) RegisterRoutes(r *gin.Engine) {
	// S3 API follows path-style or virtual-hosted-style URLs
	// Path-style: http://endpoint/bucket/key
	// Virtual-hosted-style: http://bucket.endpoint/key

	// Service-level operations
	r.GET("/", h.listBuckets)

	// Bucket operations (path-style)
	r.PUT("/:bucket", h.createBucket)
	r.GET("/:bucket", h.getBucket)
	r.DELETE("/:bucket", h.deleteBucket)
	r.HEAD("/:bucket", h.headBucket)
	r.GET("/:bucket?location", h.getBucketLocation)
	r.GET("/:bucket?versioning", h.getBucketVersioning)
	r.PUT("/:bucket?versioning", h.setBucketVersioning)
	r.GET("/:bucket?policy", h.getBucketPolicy)
	r.PUT("/:bucket?policy", h.setBucketPolicy)
	r.GET("/:bucket?uploads", h.listMultipartUploads)

	// Object operations
	r.PUT("/:bucket/*key", h.putObject)
	r.GET("/:bucket/*key", h.getObject)
	r.HEAD("/:bucket/*key", h.headObject)
	r.DELETE("/:bucket/*key", h.deleteObject)
	r.GET("/:bucket/*key?uploadId", h.getObjectWithVersion)

	// Multipart upload operations
	r.POST("/:bucket/*key?uploads", h.initiateMultipartUpload)
	r.PUT("/:bucket/*key?uploadId", h.uploadPart)
	r.POST("/:bucket/*key?uploadId", h.completeMultipartUpload)
	r.DELETE("/:bucket/*key?uploadId", h.abortMultipartUpload)

	// Presigned URL support is handled via query parameters
	r.GET("/:bucket/*key?AWSAccessKeyId", h.getObjectPresigned)
}

// listBuckets lists all buckets (ListBuckets operation).
func (h *Handlers) listBuckets(c *gin.Context) {
	buckets := h.manager.ListBuckets()

	type bucketXML struct {
		Name      string    `xml:"Name"`
		CreatedAt time.Time `xml:"CreationDate"`
	}

	type result struct {
		XMLName xml.Name    `xml:"ListAllMyBucketsResult"`
		Owner   struct {
			ID          string `xml:"ID"`
			DisplayName string `xml:"DisplayName"`
		} `xml:"Owner"`
		Buckets struct {
			Bucket []bucketXML `xml:"Bucket"`
		} `xml:"Buckets"`
	}

	resp := result{
		Owner: struct {
			ID          string `xml:"ID"`
			DisplayName string `xml:"DisplayName"`
		}{
			ID:          "admin",
			DisplayName: "admin",
		},
	}

	for _, b := range buckets {
		resp.Buckets.Bucket = append(resp.Buckets.Bucket, bucketXML{
			Name:      b.Name,
			CreatedAt: b.CreatedAt,
		})
	}

	c.XML(200, resp)
}

// createBucket creates a new bucket (CreateBucket operation).
func (h *Handlers) createBucket(c *gin.Context) {
	bucketName := c.Param("bucket")

	var input BucketInput
	if err := c.ShouldBindJSON(&input); err != nil {
		// JSON binding failed, use just the bucket name
		input = BucketInput{Name: bucketName}
	}
	input.Name = bucketName

	bucket, err := h.manager.CreateBucket(input)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	c.Header("Location", "/"+bucket.Name)
	c.Status(200)
}

// getBucket gets bucket info or lists objects (HeadBucket/ListObjectsV2).
func (h *Handlers) getBucket(c *gin.Context) {
	bucketName := c.Param("bucket")

	bucket, err := h.manager.GetBucket(bucketName)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	// Check if this is a list objects request
	if c.Query("list-type") == "2" || c.Query("max-keys") != "" {
		h.listObjectsV2(c, bucketName)
		return
	}

	// Return bucket info
	c.JSON(200, bucket)
}

// headBucket checks if bucket exists.
func (h *Handlers) headBucket(c *gin.Context) {
	bucketName := c.Param("bucket")

	_, err := h.manager.GetBucket(bucketName)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	c.Status(200)
}

// deleteBucket deletes a bucket.
func (h *Handlers) deleteBucket(c *gin.Context) {
	bucketName := c.Param("bucket")

	if err := h.manager.DeleteBucket(bucketName); err != nil {
		h.sendS3Error(c, err)
		return
	}

	c.Status(204)
}

// getBucketLocation returns bucket location.
func (h *Handlers) getBucketLocation(c *gin.Context) {
	bucketName := c.Param("bucket")

	_, err := h.manager.GetBucket(bucketName)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	type locationResult struct {
		XMLName           xml.Name `xml:"LocationConstraint"`
		LocationConstraint string   `xml:",chardata"`
	}

	c.XML(200, locationResult{
		LocationConstraint: h.manager.GetConfig().Region,
	})
}

// getBucketVersioning returns bucket versioning config.
func (h *Handlers) getBucketVersioning(c *gin.Context) {
	bucketName := c.Param("bucket")

	bucket, err := h.manager.GetBucket(bucketName)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	type versioningResult struct {
		XMLName xml.Name `xml:"VersioningConfiguration"`
		Status  string   `xml:"Status"`
	}

	c.XML(200, versioningResult{
		Status: string(bucket.Versioning.Status),
	})
}

// setBucketVersioning sets bucket versioning.
func (h *Handlers) setBucketVersioning(c *gin.Context) {
	bucketName := c.Param("bucket")

	var config struct {
		Status string `xml:"Status"`
	}
	if err := c.ShouldBindXML(&config); err != nil {
		c.XML(400, gin.H{"Error": "Invalid versioning configuration"})
		return
	}

	versioning := VersioningConfig{Status: VersioningSuspended}
	if config.Status == "Enabled" {
		versioning.Status = VersioningEnabled
	}

	if err := h.manager.SetBucketVersioning(bucketName, versioning); err != nil {
		h.sendS3Error(c, err)
		return
	}

	c.Status(200)
}

// getBucketPolicy gets bucket policy.
func (h *Handlers) getBucketPolicy(c *gin.Context) {
	bucketName := c.Param("bucket")

	policy, err := h.manager.GetBucketPolicy(bucketName)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	c.JSON(200, policy)
}

// setBucketPolicy sets bucket policy.
func (h *Handlers) setBucketPolicy(c *gin.Context) {
	bucketName := c.Param("bucket")

	var policy BucketPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(400, gin.H{"error": "Invalid policy"})
		return
	}

	if err := h.manager.SetBucketPolicy(bucketName, &policy); err != nil {
		h.sendS3Error(c, err)
		return
	}

	c.Status(200)
}

// listObjectsV2 lists objects in a bucket (ListObjectsV2 operation).
func (h *Handlers) listObjectsV2(c *gin.Context, bucketName string) {
	prefix := c.Query("prefix")
	delimiter := c.Query("delimiter")
	marker := c.Query("continuation-token")
	maxKeys, _ := strconv.Atoi(c.Query("max-keys"))
	if maxKeys == 0 {
		maxKeys = 1000
	}

	listResult, err := h.manager.ListObjects(c.Request.Context(), bucketName, prefix, delimiter, marker, maxKeys)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	type objectXML struct {
		Key          string `xml:"Key"`
		LastModified string `xml:"LastModified"`
		ETag         string `xml:"ETag"`
		Size         int64  `xml:"Size"`
		StorageClass string `xml:"StorageClass"`
	}

	type commonPrefixXML struct {
		Prefix string `xml:"Prefix"`
	}

	type listBucketResultXML struct {
		XMLName               xml.Name `xml:"ListBucketResult"`
		Name                  string   `xml:"Name"`
		Prefix                string   `xml:"Prefix"`
		KeyCount              int      `xml:"KeyCount"`
		MaxKeys               int      `xml:"MaxKeys"`
		IsTruncated           bool     `xml:"IsTruncated"`
		NextContinuationToken string   `xml:"NextContinuationToken,omitempty"`
		Contents              []objectXML `xml:"Contents"`
		CommonPrefixes        []commonPrefixXML `xml:"CommonPrefixes"`
	}

	resp := listBucketResultXML{
		Name:        bucketName,
		Prefix:      prefix,
		KeyCount:    len(listResult.Objects),
		MaxKeys:     maxKeys,
		IsTruncated: listResult.IsTruncated,
		NextContinuationToken: listResult.NextContinuationToken,
	}

	for _, obj := range listResult.Objects {
		resp.Contents = append(resp.Contents, objectXML{
			Key:          obj.Key,
			LastModified: obj.LastModified.Format(time.RFC3339),
			ETag:         "\"" + obj.ETag + "\"",
			Size:         obj.Size,
			StorageClass: string(obj.StorageClass),
		})
	}

	for _, prefix := range listResult.CommonPrefixes {
		resp.CommonPrefixes = append(resp.CommonPrefixes, commonPrefixXML{Prefix: prefix})
	}

	c.XML(200, resp)
}

// putObject uploads an object (PutObject operation).
func (h *Handlers) putObject(c *gin.Context) {
	bucketName := c.Param("bucket")
	key := strings.TrimPrefix(c.Param("key"), "/")

	contentType := c.GetHeader("Content-Type")
	contentLength, _ := strconv.ParseInt(c.GetHeader("Content-Length"), 10, 64)

	// Extract metadata from headers
	metadata := make(map[string]string)
	for k, v := range c.Request.Header {
		if strings.HasPrefix(k, "X-Amz-Meta-") {
			metaKey := strings.TrimPrefix(k, "X-Amz-Meta-")
			if len(v) > 0 {
				metadata[metaKey] = v[0]
			}
		}
	}

	obj, err := h.manager.PutObject(
		c.Request.Context(),
		bucketName,
		key,
		c.Request.Body,
		contentLength,
		contentType,
		metadata,
	)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	c.Header("ETag", "\""+obj.ETag+"\"")
	c.Status(200)
}

// getObject downloads an object (GetObject operation).
func (h *Handlers) getObject(c *gin.Context) {
	bucketName := c.Param("bucket")
	key := strings.TrimPrefix(c.Param("key"), "/")

	reader, obj, err := h.manager.GetObject(c.Request.Context(), bucketName, key)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}
	defer reader.Close()

	c.Header("Content-Type", obj.ContentType)
	c.Header("Content-Length", strconv.FormatInt(obj.Size, 10))
	c.Header("ETag", "\""+obj.ETag+"\"")
	c.Header("Last-Modified", obj.LastModified.Format(time.RFC1123))

	// Add metadata headers
	for k, v := range obj.Metadata {
		c.Header("X-Amz-Meta-"+k, v)
	}

	io.Copy(c.Writer, reader)
}

// headObject gets object metadata.
func (h *Handlers) headObject(c *gin.Context) {
	bucketName := c.Param("bucket")
	key := strings.TrimPrefix(c.Param("key"), "/")

	obj, err := h.manager.GetObjectInfo(c.Request.Context(), bucketName, key)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	c.Header("Content-Type", obj.ContentType)
	c.Header("Content-Length", strconv.FormatInt(obj.Size, 10))
	c.Header("ETag", "\""+obj.ETag+"\"")
	c.Header("Last-Modified", obj.LastModified.Format(time.RFC1123))
	c.Status(200)
}

// deleteObject deletes an object.
func (h *Handlers) deleteObject(c *gin.Context) {
	bucketName := c.Param("bucket")
	key := strings.TrimPrefix(c.Param("key"), "/")

	if err := h.manager.DeleteObject(c.Request.Context(), bucketName, key); err != nil {
		h.sendS3Error(c, err)
		return
	}

	c.Status(204)
}

// getObjectWithVersion gets object with version.
func (h *Handlers) getObjectWithVersion(c *gin.Context) {
	// Version support - currently same as getObject
	h.getObject(c)
}

// initiateMultipartUpload starts a multipart upload.
func (h *Handlers) initiateMultipartUpload(c *gin.Context) {
	bucketName := c.Param("bucket")
	key := strings.TrimPrefix(c.Param("key"), "/")

	upload, err := h.manager.InitiateMultipartUpload(
		c.Request.Context(),
		bucketName,
		key,
		UploadConfig{Bucket: bucketName, Key: key},
	)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	type result struct {
		XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
		Bucket   string   `xml:"Bucket"`
		Key      string   `xml:"Key"`
		UploadID string   `xml:"UploadId"`
	}

	c.XML(200, result{
		Bucket:   upload.Bucket,
		Key:      upload.Key,
		UploadID: upload.UploadID,
	})
}

// uploadPart uploads a part.
func (h *Handlers) uploadPart(c *gin.Context) {
	uploadID := c.Query("uploadId")
	partNumber, _ := strconv.Atoi(c.Query("partNumber"))
	contentLength, _ := strconv.ParseInt(c.GetHeader("Content-Length"), 10, 64)

	part, err := h.manager.UploadPart(
		c.Request.Context(),
		uploadID,
		partNumber,
		c.Request.Body,
		contentLength,
	)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	c.Header("ETag", "\""+part.ETag+"\"")
	c.Status(200)
}

// completeMultipartUpload completes a multipart upload.
func (h *Handlers) completeMultipartUpload(c *gin.Context) {
	uploadID := c.Query("uploadId")

	var input struct {
		Parts []struct {
			PartNumber int    `xml:"PartNumber"`
			ETag       string `xml:"ETag"`
		} `xml:"Part"`
	}
	if err := c.ShouldBindXML(&input); err != nil {
		c.XML(400, gin.H{"Error": "Invalid request body"})
		return
	}

	parts := make([]*CompletedPart, len(input.Parts))
	for i, p := range input.Parts {
		parts[i] = &CompletedPart{
			PartNumber: p.PartNumber,
			ETag:       strings.Trim(p.ETag, "\""),
		}
	}

	obj, err := h.manager.CompleteMultipartUpload(c.Request.Context(), uploadID, parts)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	type result struct {
		XMLName xml.Name `xml:"CompleteMultipartUploadResult"`
		Location string  `xml:"Location"`
		Bucket   string  `xml:"Bucket"`
		Key      string  `xml:"Key"`
		ETag     string  `xml:"ETag"`
	}

	c.XML(200, result{
		Location: fmt.Sprintf("http://%s/%s/%s", h.manager.GetConfig().Domain, obj.Bucket, obj.Key),
		Bucket:   obj.Bucket,
		Key:      obj.Key,
		ETag:     "\"" + obj.ETag + "\"",
	})
}

// abortMultipartUpload aborts a multipart upload.
func (h *Handlers) abortMultipartUpload(c *gin.Context) {
	uploadID := c.Query("uploadId")

	if err := h.manager.AbortMultipartUpload(c.Request.Context(), uploadID); err != nil {
		h.sendS3Error(c, err)
		return
	}

	c.Status(204)
}

// listMultipartUploads lists active multipart uploads.
func (h *Handlers) listMultipartUploads(c *gin.Context) {
	bucketName := c.Param("bucket")

	uploads, err := h.manager.ListMultipartUploads(c.Request.Context(), bucketName)
	if err != nil {
		h.sendS3Error(c, err)
		return
	}

	type uploadXML struct {
		Key       string `xml:"Key"`
		UploadID  string `xml:"UploadId"`
		Initiator struct {
			ID string `xml:"ID"`
		} `xml:"Initiator"`
		Owner struct {
			ID string `xml:"ID"`
		} `xml:"Owner"`
		StorageClass string `xml:"StorageClass"`
		Initiated    string `xml:"Initiated"`
	}

	type result struct {
		XMLName xml.Name `xml:"ListMultipartUploadsResult"`
		Bucket  string   `xml:"Bucket"`
		Uploads []uploadXML `xml:"Upload"`
	}

	resp := result{Bucket: bucketName}
	for _, u := range uploads {
		resp.Uploads = append(resp.Uploads, uploadXML{
			Key:       u.Key,
			UploadID:  u.UploadID,
			Initiated: u.CreatedAt.Format(time.RFC3339),
		})
	}

	c.XML(200, resp)
}

// getObjectPresigned handles presigned URL requests.
func (h *Handlers) getObjectPresigned(c *gin.Context) {
	// Validate presigned URL
	accessKey := c.Query("AWSAccessKeyId")
	expires := c.Query("Expires")
	signature := c.Query("Signature")

	if accessKey == "" || expires == "" || signature == "" {
		h.sendS3Error(c, ErrAccessDenied)
		return
	}

	// Verify expiration
	expiresUnix, err := strconv.ParseInt(expires, 10, 64)
	if err != nil {
		h.sendS3Error(c, ErrAccessDenied)
		return
	}

	if time.Now().Unix() > expiresUnix {
		h.sendS3Error(c, ErrAccessDenied)
		return
	}

	// For now, allow the request (signature validation would be done in production)
	h.getObject(c)
}

// sendS3Error sends an S3 error response.
func (h *Handlers) sendS3Error(c *gin.Context, err error) {
	if s3Err, ok := err.(*S3Error); ok {
		c.Data(s3Err.Code, "application/xml", []byte(s3Err.ToXML()))
		return
	}

	c.XML(500, gin.H{
		"Code":    "InternalError",
		"Message": err.Error(),
	})
}