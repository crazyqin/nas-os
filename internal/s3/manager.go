// Package s3 implements S3-compatible object storage for NAS-OS
package s3

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 安全验证正则表达式
var (
	bucketNameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`)
	safeKeyRegex    = regexp.MustCompile(`^[a-zA-Z0-9!_.*'()-/]+$`)
)

// Manager manages S3-compatible object storage.
type Manager struct {
	mu          sync.RWMutex
	buckets     map[string]*Bucket
	objects     map[string]map[string]*Object // bucket -> key -> object
	uploads     map[string]*MultipartUpload   // uploadID -> upload
	config      *Config
	configPath  string
	dataDir     string
	accessKey   string
	secretKey   string
}

// persistentConfig is the on-disk configuration structure.
type persistentConfig struct {
	Config  *Config            `json:"config"`
	Buckets map[string]*Bucket `json:"buckets"`
}

// NewManager creates a new S3 manager.
func NewManager(configPath, dataDir string) (*Manager, error) {
	m := &Manager{
		buckets:    make(map[string]*Bucket),
		objects:    make(map[string]map[string]*Object),
		uploads:    make(map[string]*MultipartUpload),
		configPath: configPath,
		dataDir:    dataDir,
		config: &Config{
			Enabled:       true,
			BindAddress:   "0.0.0.0",
			Port:          9000,
			Domain:        "s3.nas-os.local",
			Region:        "default",
			AccessKey:     "minioadmin",
			SecretKey:     "minioadmin",
			MaxUploadSize: 5 * 1024 * 1024 * 1024 * 1024, // 5TB
		},
		accessKey: "minioadmin",
		secretKey: "minioadmin",
	}

	// Load existing configuration
	if err := m.loadConfig(); err != nil {
		return nil, err
	}

	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	// Load objects from disk
	if err := m.loadObjects(); err != nil {
		return nil, fmt.Errorf("failed to load objects: %w", err)
	}

	return m, nil
}

// loadConfig loads configuration from disk.
func (m *Manager) loadConfig() error {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var pc persistentConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if pc.Config != nil {
		m.config = pc.Config
		if pc.Config.AccessKey != "" {
			m.accessKey = pc.Config.AccessKey
		}
		if pc.Config.SecretKey != "" {
			m.secretKey = pc.Config.SecretKey
		}
	}
	if pc.Buckets != nil {
		m.buckets = pc.Buckets
	}

	return nil
}

// saveConfig saves configuration to disk.
func (m *Manager) saveConfig() error {
	pc := persistentConfig{
		Config:  m.config,
		Buckets: m.buckets,
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0640)
}

// loadObjects loads objects from disk.
func (m *Manager) loadObjects() error {
	for bucketName := range m.buckets {
		bucketDir := filepath.Join(m.dataDir, bucketName)
		if _, err := os.Stat(bucketDir); os.IsNotExist(err) {
			continue
		}

		m.objects[bucketName] = make(map[string]*Object)

		// Walk through bucket directory and load object metadata
		err := filepath.Walk(bucketDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			relPath, _ := filepath.Rel(bucketDir, path)
			key := filepath.ToSlash(relPath)
			metaPath := path + ".meta"

			// Load metadata
			obj := &Object{
				Key:          key,
				Bucket:       bucketName,
				Size:         info.Size(),
				LastModified: info.ModTime(),
				StorageClass: StorageClassStandard,
			}

			if metaData, err := os.ReadFile(metaPath); err == nil {
				var meta struct {
					ETag        string            `json:"etag"`
					ContentType string            `json:"contentType"`
					Metadata    map[string]string `json:"metadata"`
					VersionID   string            `json:"versionId"`
				}
				if json.Unmarshal(metaData, &meta) == nil {
					obj.ETag = meta.ETag
					obj.ContentType = meta.ContentType
					obj.Metadata = meta.Metadata
					obj.VersionID = meta.VersionID
				}
			}

			m.objects[bucketName][key] = obj
			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to load objects for bucket %s: %w", bucketName, err)
		}
	}

	return nil
}

// validateBucketName validates bucket name according to S3 rules.
func validateBucketName(name string) error {
	if len(name) < 3 || len(name) > 63 {
		return ErrInvalidBucketName
	}
	if !bucketNameRegex.MatchString(name) {
		return ErrInvalidBucketName
	}
	if strings.HasPrefix(name, "xn--") || strings.HasSuffix(name, "-s3alias") {
		return ErrInvalidBucketName
	}
	return nil
}

// validateKey validates object key.
func validateKey(key string) error {
	if len(key) == 0 || len(key) > 1024 {
		return ErrInvalidKey
	}
	return nil
}

// CreateBucket creates a new bucket.
func (m *Manager) CreateBucket(input BucketInput) (*Bucket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate bucket name
	if err := validateBucketName(input.Name); err != nil {
		return nil, err
	}

	// Check for existing bucket
	if _, exists := m.buckets[input.Name]; exists {
		return nil, ErrBucketExists
	}

	// Create bucket
	bucket := &Bucket{
		Name:       input.Name,
		CreatedAt:  time.Now(),
		Versioning: VersioningConfig{Status: VersioningSuspended},
		Tags:       input.Tags,
	}

	if input.Versioning != nil {
		bucket.Versioning = *input.Versioning
	}
	if input.Quota != nil {
		bucket.Quota = input.Quota
	}
	if input.Encryption != nil {
		bucket.Encryption = input.Encryption
	}
	if input.ObjectLock != nil {
		bucket.ObjectLock = input.ObjectLock
	}

	m.buckets[input.Name] = bucket
	m.objects[input.Name] = make(map[string]*Object)

	// Create bucket directory
	bucketDir := filepath.Join(m.dataDir, input.Name)
	if err := os.MkdirAll(bucketDir, 0750); err != nil {
		delete(m.buckets, input.Name)
		delete(m.objects, input.Name)
		return nil, fmt.Errorf("failed to create bucket directory: %w", err)
	}

	if err := m.saveConfig(); err != nil {
		delete(m.buckets, input.Name)
		delete(m.objects, input.Name)
		return nil, err
	}

	return bucket, nil
}

// GetBucket retrieves a bucket by name.
func (m *Manager) GetBucket(name string) (*Bucket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bucket, exists := m.buckets[name]
	if !exists {
		return nil, ErrBucketNotFound
	}
	return bucket, nil
}

// ListBuckets lists all buckets.
func (m *Manager) ListBuckets() []*Bucket {
	m.mu.RLock()
	defer m.mu.RUnlock()

	buckets := make([]*Bucket, 0, len(m.buckets))
	for _, b := range m.buckets {
		buckets = append(buckets, b)
	}

	// Sort by name
	sort.Slice(buckets, func(i, j int) bool {
		return buckets[i].Name < buckets[j].Name
	})

	return buckets
}

// DeleteBucket deletes a bucket.
func (m *Manager) DeleteBucket(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.buckets[name]; !exists {
		return ErrBucketNotFound
	}

	// Check if bucket is empty
	if objects, ok := m.objects[name]; ok && len(objects) > 0 {
		return ErrBucketNotEmpty
	}

	delete(m.buckets, name)
	delete(m.objects, name)

	// Remove bucket directory
	bucketDir := filepath.Join(m.dataDir, name)
	_ = os.RemoveAll(bucketDir)

	return m.saveConfig()
}

// PutObject uploads an object.
func (m *Manager) PutObject(ctx context.Context, bucketName, key string, reader io.Reader, size int64, contentType string, metadata map[string]string) (*Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Validate
	if _, exists := m.buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}
	if err := validateKey(key); err != nil {
		return nil, err
	}

	// Check quota
	if bucket := m.buckets[bucketName]; bucket.Quota != nil && bucket.Quota.Enabled {
		var currentSize int64
		for _, obj := range m.objects[bucketName] {
			currentSize += obj.Size
		}
		if currentSize+size > bucket.Quota.Size {
			return nil, ErrQuotaExceeded
		}
	}

	// Check max upload size
	if size > m.config.MaxUploadSize {
		return nil, ErrEntityTooLarge
	}

	// Create object path
	objPath := filepath.Join(m.dataDir, bucketName, key)
	objDir := filepath.Dir(objPath)
	if err := os.MkdirAll(objDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create object directory: %w", err)
	}

	// Write object data
	file, err := os.Create(objPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create object file: %w", err)
	}
	defer file.Close()

	// Calculate ETag while writing
	hash := md5.New()
	multiWriter := io.MultiWriter(file, hash)

	written, err := io.CopyN(multiWriter, reader, size)
	if err != nil && err != io.EOF {
		_ = os.Remove(objPath)
		return nil, fmt.Errorf("failed to write object: %w", err)
	}

	etag := hex.EncodeToString(hash.Sum(nil))

	// Create object metadata
	obj := &Object{
		Key:          key,
		Bucket:       bucketName,
		Size:         written,
		ETag:         etag,
		ContentType:  contentType,
		LastModified: time.Now(),
		Metadata:     metadata,
		IsLatest:     true,
		StorageClass: StorageClassStandard,
	}

	// Generate version ID if versioning enabled
	if bucket := m.buckets[bucketName]; bucket.Versioning.Status == VersioningEnabled {
		obj.VersionID = uuid.New().String()
	}

	// Save metadata
	metaData, _ := json.Marshal(map[string]interface{}{
		"etag":        obj.ETag,
		"contentType": obj.ContentType,
		"metadata":    obj.Metadata,
		"versionId":   obj.VersionID,
	})
	_ = os.WriteFile(objPath+".meta", metaData, 0640)

	// Store in memory
	if m.objects[bucketName] == nil {
		m.objects[bucketName] = make(map[string]*Object)
	}
	m.objects[bucketName][key] = obj

	return obj, nil
}

// GetObject retrieves an object.
func (m *Manager) GetObject(ctx context.Context, bucketName, key string) (io.ReadCloser, *Object, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return nil, nil, ErrBucketNotFound
	}

	obj, exists := m.objects[bucketName][key]
	if !exists {
		return nil, nil, ErrObjectNotFound
	}

	objPath := filepath.Join(m.dataDir, bucketName, key)
	file, err := os.Open(objPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open object file: %w", err)
	}

	return file, obj, nil
}

// DeleteObject deletes an object.
func (m *Manager) DeleteObject(ctx context.Context, bucketName, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return ErrBucketNotFound
	}

	if _, exists := m.objects[bucketName][key]; !exists {
		return ErrObjectNotFound
	}

	objPath := filepath.Join(m.dataDir, bucketName, key)
	_ = os.Remove(objPath)
	_ = os.Remove(objPath + ".meta")

	delete(m.objects[bucketName], key)

	return nil
}

// ListObjects lists objects in a bucket.
func (m *Manager) ListObjects(ctx context.Context, bucketName, prefix, delimiter, marker string, maxKeys int) (*ObjectList, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}

	if maxKeys <= 0 {
		maxKeys = 1000
	}

	result := &ObjectList{
		Bucket:     bucketName,
		Prefix:     prefix,
		Delimiter:  delimiter,
		MaxKeys:    maxKeys,
		Objects:    make([]*ObjectInfo, 0),
	}

	objects := m.objects[bucketName]
	keys := make([]string, 0, len(objects))
	for key := range objects {
		if prefix == "" || strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}

	// Sort keys
	sort.Strings(keys)

	// Handle pagination
	startIdx := 0
	if marker != "" {
		for i, key := range keys {
			if key > marker {
				startIdx = i
				break
			}
		}
	}

	commonPrefixes := make(map[string]bool)
	count := 0

	for i := startIdx; i < len(keys) && count < maxKeys; i++ {
		key := keys[i]

		if delimiter != "" {
			// Check for common prefix
			afterPrefix := strings.TrimPrefix(key, prefix)
			if idx := strings.Index(afterPrefix, delimiter); idx >= 0 {
				commonPrefix := prefix + afterPrefix[:idx+1]
				commonPrefixes[commonPrefix] = true
				continue
			}
		}

		obj := objects[key]
		result.Objects = append(result.Objects, &ObjectInfo{
			Key:          obj.Key,
			Bucket:       obj.Bucket,
			Size:         obj.Size,
			ETag:         obj.ETag,
			ContentType:  obj.ContentType,
			LastModified: obj.LastModified,
			Metadata:     obj.Metadata,
			VersionID:    obj.VersionID,
			StorageClass: obj.StorageClass,
		})
		count++
	}

	// Add common prefixes
	for prefix := range commonPrefixes {
		result.CommonPrefixes = append(result.CommonPrefixes, prefix)
	}
	sort.Strings(result.CommonPrefixes)

	// Check if truncated
	if startIdx+count < len(keys) {
		result.IsTruncated = true
		if len(result.Objects) > 0 {
			result.NextMarker = result.Objects[len(result.Objects)-1].Key
		}
	}

	return result, nil
}

// GetObjectInfo returns object metadata.
func (m *Manager) GetObjectInfo(ctx context.Context, bucketName, key string) (*ObjectInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}

	obj, exists := m.objects[bucketName][key]
	if !exists {
		return nil, ErrObjectNotFound
	}

	return &ObjectInfo{
		Key:          obj.Key,
		Bucket:       obj.Bucket,
		Size:         obj.Size,
		ETag:         obj.ETag,
		ContentType:  obj.ContentType,
		LastModified: obj.LastModified,
		Metadata:     obj.Metadata,
		VersionID:    obj.VersionID,
		StorageClass: obj.StorageClass,
	}, nil
}

// CopyObject copies an object.
func (m *Manager) CopyObject(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string, metadata map[string]string) (*Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.buckets[srcBucket]; !exists {
		return nil, ErrBucketNotFound
	}
	if _, exists := m.buckets[dstBucket]; !exists {
		return nil, ErrBucketNotFound
	}

	srcObj, exists := m.objects[srcBucket][srcKey]
	if !exists {
		return nil, ErrObjectNotFound
	}

	// Read source file
	srcPath := filepath.Join(m.dataDir, srcBucket, srcKey)
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open source object: %w", err)
	}
	defer srcFile.Close()

	// Create destination
	dstPath := filepath.Join(m.dataDir, dstBucket, dstKey)
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create destination directory: %w", err)
	}

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy content
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return nil, fmt.Errorf("failed to copy object: %w", err)
	}

	// Create new object
	newObj := &Object{
		Key:          dstKey,
		Bucket:       dstBucket,
		Size:         srcObj.Size,
		ETag:         srcObj.ETag,
		ContentType:  srcObj.ContentType,
		LastModified: time.Now(),
		Metadata:     metadata,
		IsLatest:     true,
		StorageClass: srcObj.StorageClass,
	}

	if newMetadata := metadata; newMetadata == nil {
		newObj.Metadata = srcObj.Metadata
	}

	// Save metadata
	metaData, _ := json.Marshal(map[string]interface{}{
		"etag":        newObj.ETag,
		"contentType": newObj.ContentType,
		"metadata":    newObj.Metadata,
	})
	_ = os.WriteFile(dstPath+".meta", metaData, 0640)

	m.objects[dstBucket][dstKey] = newObj

	return newObj, nil
}

// GeneratePresignedURL generates a presigned URL for an object.
func (m *Manager) GeneratePresignedURL(req PresignedURLRequest) (*PresignedURL, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.buckets[req.Bucket]; !exists {
		return nil, ErrBucketNotFound
	}

	if req.Duration == 0 {
		req.Duration = 15 * time.Minute
	}

	if req.Method == "" {
		req.Method = "GET"
	}

	// Calculate expiration
	expires := time.Now().Add(req.Duration)
	expiresUnix := expires.Unix()

	// Build string to sign
	// Format: METHOD\n\ncontent-type\nexpires\n/bucket/key
	stringToSign := fmt.Sprintf("%s\n\n%s\n%d\n/%s/%s",
		req.Method,
		req.ContentType,
		expiresUnix,
		req.Bucket,
		req.Key)

	// Calculate signature
	signature := m.calculateSignature(stringToSign)

	// Build URL
	values := url.Values{}
	values.Set("AWSAccessKeyId", m.accessKey)
	values.Set("Expires", fmt.Sprintf("%d", expiresUnix))
	values.Set("Signature", signature)

	presignedURL := fmt.Sprintf("http://%s:%d/%s/%s?%s",
		m.config.Domain,
		m.config.Port,
		req.Bucket,
		url.PathEscape(req.Key),
		values.Encode())

	return &PresignedURL{
		URL:       presignedURL,
		ExpiresAt: expires,
		Method:    req.Method,
		ObjectKey: req.Key,
		Bucket:    req.Bucket,
	}, nil
}

// calculateSignature calculates HMAC-SHA1 signature.
func (m *Manager) calculateSignature(stringToSign string) string {
	h := hmac.New(sha256.New, []byte(m.secretKey))
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// InitiateMultipartUpload initiates a multipart upload.
func (m *Manager) InitiateMultipartUpload(ctx context.Context, bucketName, key string, config UploadConfig) (*MultipartUpload, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}

	uploadID := uuid.New().String()
	upload := &MultipartUpload{
		UploadID:  uploadID,
		Bucket:    bucketName,
		Key:       key,
		CreatedAt: time.Now(),
		Parts:     make([]*Part, 0),
	}

	m.uploads[uploadID] = upload

	// Create upload directory
	uploadDir := filepath.Join(m.dataDir, bucketName, ".uploads", uploadID)
	_ = os.MkdirAll(uploadDir, 0750)

	return upload, nil
}

// UploadPart uploads a part in a multipart upload.
func (m *Manager) UploadPart(ctx context.Context, uploadID string, partNumber int, reader io.Reader, size int64) (*Part, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	upload, exists := m.uploads[uploadID]
	if !exists {
		return nil, ErrInvalidUploadID
	}

	if partNumber < 1 || partNumber > 10000 {
		return nil, fmt.Errorf("invalid part number: must be between 1 and 10000")
	}

	// Create part file
	partPath := filepath.Join(m.dataDir, upload.Bucket, ".uploads", uploadID, fmt.Sprintf("part-%d", partNumber))
	file, err := os.Create(partPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create part file: %w", err)
	}
	defer file.Close()

	// Write and calculate ETag
	hash := md5.New()
	multiWriter := io.MultiWriter(file, hash)

	written, err := io.CopyN(multiWriter, reader, size)
	if err != nil && err != io.EOF {
		_ = os.Remove(partPath)
		return nil, fmt.Errorf("failed to write part: %w", err)
	}

	etag := hex.EncodeToString(hash.Sum(nil))

	part := &Part{
		PartNumber:   partNumber,
		ETag:         etag,
		Size:         written,
		LastModified: time.Now(),
	}

	// Update upload
	for i, p := range upload.Parts {
		if p.PartNumber == partNumber {
			upload.Parts[i] = part
			return part, nil
		}
	}
	upload.Parts = append(upload.Parts, part)

	return part, nil
}

// CompleteMultipartUpload completes a multipart upload.
func (m *Manager) CompleteMultipartUpload(ctx context.Context, uploadID string, parts []*CompletedPart) (*Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	upload, exists := m.uploads[uploadID]
	if !exists {
		return nil, ErrInvalidUploadID
	}

	// Validate parts
	if len(parts) == 0 {
		return nil, fmt.Errorf("no parts provided")
	}

	// Sort parts by part number
	sort.Slice(parts, func(i, j int) bool {
		return parts[i].PartNumber < parts[j].PartNumber
	})

	// Create final object
	objPath := filepath.Join(m.dataDir, upload.Bucket, upload.Key)
	objDir := filepath.Dir(objPath)
	_ = os.MkdirAll(objDir, 0750)

	file, err := os.Create(objPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create object file: %w", err)
	}
	defer file.Close()

	// Concatenate parts
	hash := md5.New()
	var totalSize int64

	for _, cp := range parts {
		partPath := filepath.Join(m.dataDir, upload.Bucket, ".uploads", uploadID, fmt.Sprintf("part-%d", cp.PartNumber))
		partFile, err := os.Open(partPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open part %d: %w", cp.PartNumber, err)
		}

		multiWriter := io.MultiWriter(file, hash)
		written, err := io.Copy(multiWriter, partFile)
		partFile.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to copy part %d: %w", cp.PartNumber, err)
		}

		totalSize += written
	}

	etag := hex.EncodeToString(hash.Sum(nil))

	// Create object
	obj := &Object{
		Key:          upload.Key,
		Bucket:       upload.Bucket,
		Size:         totalSize,
		ETag:         etag,
		LastModified: time.Now(),
		IsLatest:     true,
		StorageClass: StorageClassStandard,
	}

	// Save metadata
	metaData, _ := json.Marshal(map[string]interface{}{
		"etag":        obj.ETag,
		"contentType": "application/octet-stream",
	})
	_ = os.WriteFile(objPath+".meta", metaData, 0640)

	// Store in memory
	m.objects[upload.Bucket][upload.Key] = obj

	// Clean up upload
	uploadDir := filepath.Join(m.dataDir, upload.Bucket, ".uploads", uploadID)
	_ = os.RemoveAll(uploadDir)
	delete(m.uploads, uploadID)

	return obj, nil
}

// AbortMultipartUpload aborts a multipart upload.
func (m *Manager) AbortMultipartUpload(ctx context.Context, uploadID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	upload, exists := m.uploads[uploadID]
	if !exists {
		return ErrInvalidUploadID
	}

	// Clean up upload
	uploadDir := filepath.Join(m.dataDir, upload.Bucket, ".uploads", uploadID)
	_ = os.RemoveAll(uploadDir)
	delete(m.uploads, uploadID)

	return nil
}

// ListMultipartUploads lists active multipart uploads.
func (m *Manager) ListMultipartUploads(ctx context.Context, bucketName string) ([]*MultipartUpload, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, exists := m.buckets[bucketName]; !exists {
		return nil, ErrBucketNotFound
	}

	uploads := make([]*MultipartUpload, 0)
	for _, upload := range m.uploads {
		if upload.Bucket == bucketName {
			uploads = append(uploads, upload)
		}
	}

	return uploads, nil
}

// SetBucketPolicy sets the bucket policy.
func (m *Manager) SetBucketPolicy(bucketName string, policy *BucketPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bucket, exists := m.buckets[bucketName]
	if !exists {
		return ErrBucketNotFound
	}

	bucket.Policy = policy
	return m.saveConfig()
}

// GetBucketPolicy gets the bucket policy.
func (m *Manager) GetBucketPolicy(bucketName string) (*BucketPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bucket, exists := m.buckets[bucketName]
	if !exists {
		return nil, ErrBucketNotFound
	}

	return bucket.Policy, nil
}

// SetBucketVersioning sets bucket versioning.
func (m *Manager) SetBucketVersioning(bucketName string, config VersioningConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bucket, exists := m.buckets[bucketName]
	if !exists {
		return ErrBucketNotFound
	}

	bucket.Versioning = config
	return m.saveConfig()
}

// GetBucketStats returns bucket statistics.
func (m *Manager) GetBucketStats(bucketName string) (*BucketStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.buckets[bucketName]
	if !exists {
		return nil, ErrBucketNotFound
	}

	stats := &BucketStats{
		Name: bucketName,
	}

	if objects, ok := m.objects[bucketName]; ok {
		stats.ObjectCount = int64(len(objects))
		for _, obj := range objects {
			stats.Size += obj.Size
			if obj.LastModified.After(stats.LastModified) {
				stats.LastModified = obj.LastModified
			}
		}
	}

	return stats, nil
}

// GetConfig returns the current configuration.
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

// SetAccessKeys sets the access keys.
func (m *Manager) SetAccessKeys(accessKey, secretKey string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accessKey = accessKey
	m.secretKey = secretKey
	m.config.AccessKey = accessKey
	m.config.SecretKey = secretKey
	_ = m.saveConfig()
}

// Authenticate validates access key and secret key.
func (m *Manager) Authenticate(accessKey, secretKey string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accessKey == accessKey && m.secretKey == secretKey
}