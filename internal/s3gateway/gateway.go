// Package s3gateway 提供S3兼容对象存储网关功能
// 将本地存储暴露为标准S3 API接口，支持多租户、配额管理、生命周期管理
package s3gateway

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"
)

// GatewayConfig 网关配置
type GatewayConfig struct {
	StorageRoot   string        `json:"storageRoot"`   // 本地存储根目录
	DefaultPolicy BucketPolicy  `json:"defaultPolicy"` // 默认桶策略
	MaxBucketSize int64         `json:"maxBucketSize"` // 默认桶最大容量
	MaxObjectSize int64         `json:"maxObjectSize"` // 单对象最大大小
	EnableLogging bool          `json:"enableLogging"` // 启用访问日志
	Region        string        `json:"region"`        // 默认区域
}

// Gateway S3兼容对象存储网关
type Gateway struct {
	config      GatewayConfig
	buckets     map[string]*Bucket              // name -> bucket
	objects     map[string]map[string]*Object   // bucket -> key -> object
	lifecycle   map[string][]LifecycleRule       // bucket -> rules
	accessLog   []AccessLog
	stats       TrafficStats
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewGateway 创建S3网关实例
func NewGateway(config GatewayConfig) *Gateway {
	if config.DefaultPolicy == "" {
		config.DefaultPolicy = PolicyPrivate
	}
	if config.Region == "" {
		config.Region = "us-east-1"
	}
	ctx, cancel := context.WithCancel(context.Background())
	gw := &Gateway{
		config:    config,
		buckets:   make(map[string]*Bucket),
		objects:   make(map[string]map[string]*Object),
		lifecycle: make(map[string][]LifecycleRule),
		stats: TrafficStats{
			ByUser:      make(map[string]*UserStats),
			ByBucket:    make(map[string]*BucketStats),
			ByOperation: make(map[string]int64),
		},
		ctx:    ctx,
		cancel: cancel,
	}
	return gw
}

// CreateBucket 创建存储桶
func (gw *Gateway) CreateBucket(name, ownerID string, policy BucketPolicy, quota BucketQuota) (*Bucket, error) {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	if !isValidBucketName(name) {
		return nil, fmt.Errorf("invalid bucket name: %s", name)
	}
	if _, exists := gw.buckets[name]; exists {
		return nil, fmt.Errorf("bucket already exists: %s", name)
	}
	if policy == "" {
		policy = gw.config.DefaultPolicy
	}

	bucket := &Bucket{
		Name:      name,
		OwnerID:   ownerID,
		Policy:    policy,
		Quota:     quota,
		CreatedAt: time.Now(),
		Region:    gw.config.Region,
		Tags:      make(map[string]string),
	}

	gw.buckets[name] = bucket
	gw.objects[name] = make(map[string]*Object)

	// 更新统计
	gw.stats.TotalBuckets++
	userStats := gw.getUserStats(ownerID)
	userStats.Buckets++

	gw.recordLog(ownerID, name, "", "CreateBucket", 200, 0, 0)
	log.Printf("[S3Gateway] bucket created: %s (owner=%s)", name, ownerID)
	return bucket, nil
}

// DeleteBucket 删除存储桶
func (gw *Gateway) DeleteBucket(name, userID string) error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	bucket, exists := gw.buckets[name]
	if !exists {
		return fmt.Errorf("bucket not found: %s", name)
	}
	if bucket.OwnerID != userID {
		return fmt.Errorf("access denied: user %s is not owner of bucket %s", userID, name)
	}
	if len(gw.objects[name]) > 0 {
		return fmt.Errorf("bucket not empty: %s", name)
	}

	delete(gw.buckets, name)
	delete(gw.objects, name)
	delete(gw.lifecycle, name)
	gw.stats.TotalBuckets--

	userStats := gw.getUserStats(userID)
	userStats.Buckets--

	gw.recordLog(userID, name, "", "DeleteBucket", 200, 0, 0)
	log.Printf("[S3Gateway] bucket deleted: %s", name)
	return nil
}

// ListBuckets 列出用户的存储桶
func (gw *Gateway) ListBuckets(userID string) []*Bucket {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	var result []*Bucket
	for _, bucket := range gw.buckets {
		if bucket.OwnerID == userID {
			cp := *bucket
			result = append(result, &cp)
		}
	}
	gw.recordLog(userID, "", "", "ListBuckets", 200, 0, 0)
	return result
}

// PutObject 上传对象
func (gw *Gateway) PutObject(bucketName, key, userID string, data []byte, contentType string, metadata, tags map[string]string) (*Object, error) {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	bucket, exists := gw.buckets[bucketName]
	if !exists {
		return nil, fmt.Errorf("bucket not found: %s", bucketName)
	}
	if bucket.OwnerID != userID {
		return nil, fmt.Errorf("access denied: user %s is not owner of bucket %s", userID, bucketName)
	}

	// 检查对象大小限制
	maxObjSize := gw.config.MaxObjectSize
	if maxObjSize > 0 && int64(len(data)) > maxObjSize {
		return nil, fmt.Errorf("object size %d exceeds max %d", len(data), maxObjSize)
	}

	// 检查桶配额
	if bucket.Quota.MaxObjects > 0 && int64(len(gw.objects[bucketName])) >= bucket.Quota.MaxObjects {
		if _, exists := gw.objects[bucketName][key]; !exists {
			return nil, fmt.Errorf("quota exceeded: max %d objects in bucket %s", bucket.Quota.MaxObjects, bucketName)
		}
	}
	if bucket.Quota.MaxSize > 0 {
		currentSize := gw.getBucketSize(bucketName)
		oldSize := int64(0)
		if old, exists := gw.objects[bucketName][key]; exists {
			oldSize = old.Size
		}
		if currentSize-oldSize+int64(len(data)) > bucket.Quota.MaxSize {
			return nil, fmt.Errorf("quota exceeded: max %d bytes in bucket %s", bucket.Quota.MaxSize, bucketName)
		}
	}

	if contentType == "" {
		contentType = "application/octet-stream"
	}

	etag := fmt.Sprintf("%x", len(data)+len(key)) // 简化ETag

	obj := &Object{
		Key:          key,
		Bucket:       bucketName,
		Size:         int64(len(data)),
		ContentType:  contentType,
		ETag:         etag,
		StorageClass: StorageClassStandard,
		Metadata:     metadata,
		Tags:         tags,
		OwnerID:      userID,
		CreatedAt:    time.Now(),
		Data:         data,
	}

	if metadata == nil {
		obj.Metadata = make(map[string]string)
	}
	if tags == nil {
		obj.Tags = make(map[string]string)
	}

	gw.objects[bucketName][key] = obj

	// 更新统计
	gw.stats.TotalObjects++
	gw.stats.TotalSize += int64(len(data))
	gw.stats.ByOperation["PutObject"]++

	userStats := gw.getUserStats(userID)
	userStats.Objects++
	userStats.TotalSize += int64(len(data))
	userStats.Uploads++

	bucketStats := gw.getBucketStats(bucketName)
	bucketStats.Objects++
	bucketStats.TotalSize += int64(len(data))
	bucketStats.Uploads++

	gw.recordLog(userID, bucketName, key, "PutObject", 200, int64(len(data)), 0)
	return obj, nil
}

// GetObject 下载对象
func (gw *Gateway) GetObject(bucketName, key, userID string) (*Object, error) {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	bucket, exists := gw.buckets[bucketName]
	if !exists {
		return nil, fmt.Errorf("bucket not found: %s", bucketName)
	}

	// 公开桶允许任何人读取，私有桶只允许owner
	if bucket.Policy == PolicyPrivate && bucket.OwnerID != userID {
		return nil, fmt.Errorf("access denied: bucket %s is private", bucketName)
	}

	obj, exists := gw.objects[bucketName][key]
	if !exists {
		return nil, fmt.Errorf("object not found: %s/%s", bucketName, key)
	}

	// 检查过期
	if obj.ExpiresAt != nil && obj.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("object expired: %s/%s", bucketName, key)
	}

	// 更新统计
	gw.stats.ByOperation["GetObject"]++
	userStats := gw.getUserStats(userID)
	userStats.Downloads++
	bucketStats := gw.getBucketStats(bucketName)
	bucketStats.Downloads++

	gw.recordLog(userID, bucketName, key, "GetObject", 200, obj.Size, 0)

	// 返回副本，保护内部数据
	cp := *obj
	return &cp, nil
}

// HeadObject 获取对象元信息
func (gw *Gateway) HeadObject(bucketName, key, userID string) (*Object, error) {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	if _, exists := gw.buckets[bucketName]; !exists {
		return nil, fmt.Errorf("bucket not found: %s", bucketName)
	}

	obj, exists := gw.objects[bucketName][key]
	if !exists {
		return nil, fmt.Errorf("object not found: %s/%s", bucketName, key)
	}

	if obj.ExpiresAt != nil && obj.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("object expired: %s/%s", bucketName, key)
	}

	gw.stats.ByOperation["HeadObject"]++
	gw.recordLog(userID, bucketName, key, "HeadObject", 200, 0, 0)

	cp := *obj
	cp.Data = nil // HEAD不返回数据
	return &cp, nil
}

// DeleteObject 删除对象
func (gw *Gateway) DeleteObject(bucketName, key, userID string) error {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	bucket, exists := gw.buckets[bucketName]
	if !exists {
		return fmt.Errorf("bucket not found: %s", bucketName)
	}
	if bucket.OwnerID != userID {
		return fmt.Errorf("access denied: user %s is not owner of bucket %s", userID, bucketName)
	}

	obj, exists := gw.objects[bucketName][key]
	if !exists {
		return fmt.Errorf("object not found: %s/%s", bucketName, key)
	}

	delete(gw.objects[bucketName], key)

	// 更新统计
	gw.stats.TotalObjects--
	gw.stats.TotalSize -= obj.Size
	gw.stats.ByOperation["DeleteObject"]++

	userStats := gw.getUserStats(userID)
	userStats.Objects--
	userStats.TotalSize -= obj.Size

	bucketStats := gw.getBucketStats(bucketName)
	bucketStats.Objects--
	bucketStats.TotalSize -= obj.Size

	gw.recordLog(userID, bucketName, key, "DeleteObject", 200, obj.Size, 0)
	log.Printf("[S3Gateway] object deleted: %s/%s", bucketName, key)
	return nil
}

// ListObjects 列出存储桶中的对象
func (gw *Gateway) ListObjects(bucketName, prefix, userID string, maxKeys int) ([]*Object, bool, error) {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	bucket, exists := gw.buckets[bucketName]
	if !exists {
		return nil, false, fmt.Errorf("bucket not found: %s", bucketName)
	}
	if bucket.Policy == PolicyPrivate && bucket.OwnerID != userID {
		return nil, false, fmt.Errorf("access denied: bucket %s is private", bucketName)
	}

	if maxKeys <= 0 {
		maxKeys = 1000
	}

	var result []*Object
	truncated := false
	for key, obj := range gw.objects[bucketName] {
		if prefix != "" && !strings.HasPrefix(key, prefix) {
			continue
		}
		if obj.ExpiresAt != nil && obj.ExpiresAt.Before(time.Now()) {
			continue
		}
		if len(result) >= maxKeys {
			truncated = true
			break
		}
		cp := *obj
		cp.Data = nil // 列表不返回数据
		result = append(result, &cp)
	}

	gw.stats.ByOperation["ListObjects"]++
	gw.recordLog(userID, bucketName, prefix, "ListObjects", 200, 0, 0)
	return result, truncated, nil
}

// GetStats 获取流量统计
func (gw *Gateway) GetStats(userID string) *TrafficStats {
	gw.mu.RLock()
	defer gw.mu.RUnlock()

	stats := &TrafficStats{
		TotalBuckets: gw.stats.TotalBuckets,
		TotalObjects: gw.stats.TotalObjects,
		TotalSize:    gw.stats.TotalSize,
		ByUser:       make(map[string]*UserStats),
		ByBucket:     make(map[string]*BucketStats),
		ByOperation:  make(map[string]int64),
	}

	// 如果指定了用户，只返回该用户的统计
	if userID != "" {
		if us, ok := gw.stats.ByUser[userID]; ok {
			stats.ByUser[userID] = &UserStats{
				Buckets:   us.Buckets,
				Objects:   us.Objects,
				TotalSize: us.TotalSize,
				Uploads:   us.Uploads,
				Downloads: us.Downloads,
			}
		}
	} else {
		for uid, us := range gw.stats.ByUser {
			stats.ByUser[uid] = &UserStats{
				Buckets:   us.Buckets,
				Objects:   us.Objects,
				TotalSize: us.TotalSize,
				Uploads:   us.Uploads,
				Downloads: us.Downloads,
			}
		}
	}

	for bid, bs := range gw.stats.ByBucket {
		stats.ByBucket[bid] = &BucketStats{
			Objects:   bs.Objects,
			TotalSize: bs.TotalSize,
			Uploads:   bs.Uploads,
			Downloads: bs.Downloads,
		}
	}
	for op, count := range gw.stats.ByOperation {
		stats.ByOperation[op] = count
	}

	return stats
}

// GetConfig 获取网关配置
func (gw *Gateway) GetConfig() GatewayConfig {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	return gw.config
}

// GetAccessLog 获取访问日志
func (gw *Gateway) GetAccessLog(limit int) []AccessLog {
	gw.mu.RLock()
	defer gw.mu.RUnlock()
	if limit <= 0 || limit > len(gw.accessLog) {
		limit = len(gw.accessLog)
	}
	// 返回最近的日志
	start := len(gw.accessLog) - limit
	result := make([]AccessLog, limit)
	copy(result, gw.accessLog[start:])
	return result
}

// AddLifecycleRule 添加生命周期规则
func (gw *Gateway) AddLifecycleRule(rule LifecycleRule) {
	gw.mu.Lock()
	defer gw.mu.Unlock()
	gw.lifecycle[rule.Bucket] = append(gw.lifecycle[rule.Bucket], rule)
}

// RunLifecycle 执行生命周期规则（扫描并处理过期/转换对象）
func (gw *Gateway) RunLifecycle() (expired, transitioned int) {
	gw.mu.Lock()
	defer gw.mu.Unlock()

	now := time.Now()
	for bucketName, rules := range gw.lifecycle {
		objects, exists := gw.objects[bucketName]
		if !exists {
			continue
		}
		for _, rule := range rules {
			if !rule.Enabled {
				continue
			}
			for key, obj := range objects {
				// 前缀匹配
				if rule.Prefix != "" && !strings.HasPrefix(key, rule.Prefix) {
					continue
				}
				age := now.Sub(obj.CreatedAt).Hours() / 24

				// 过期删除
				if rule.ExpirationDays > 0 && int(age) >= rule.ExpirationDays {
					gw.stats.TotalObjects--
					gw.stats.TotalSize -= obj.Size
					delete(objects, key)
					expired++
					gw.recordLog(obj.OwnerID, bucketName, key, "LifecycleExpire", 200, obj.Size, 0)
					continue
				}

				// 存储类别转换
				if rule.TransitionDays > 0 && int(age) >= rule.TransitionDays && obj.StorageClass != rule.TargetClass {
					obj.StorageClass = rule.TargetClass
					transitioned++
					gw.recordLog(obj.OwnerID, bucketName, key, "LifecycleTransition", 200, 0, 0)
				}
			}
		}
	}

	if expired+transitioned > 0 {
		log.Printf("[S3Gateway] lifecycle run: expired=%d, transitioned=%d", expired, transitioned)
	}
	return
}

// getUserStats 获取用户统计（需持有锁）
func (gw *Gateway) getUserStats(userID string) *UserStats {
	us, ok := gw.stats.ByUser[userID]
	if !ok {
		us = &UserStats{}
		gw.stats.ByUser[userID] = us
	}
	return us
}

// getBucketStats 获取桶统计（需持有锁）
func (gw *Gateway) getBucketStats(bucketName string) *BucketStats {
	bs, ok := gw.stats.ByBucket[bucketName]
	if !ok {
		bs = &BucketStats{}
		gw.stats.ByBucket[bucketName] = bs
	}
	return bs
}

// getBucketSize 计算桶内所有对象总大小（需持有锁）
func (gw *Gateway) getBucketSize(bucketName string) int64 {
	var size int64
	for _, obj := range gw.objects[bucketName] {
		size += obj.Size
	}
	return size
}

// recordLog 记录访问日志（需持有锁）
func (gw *Gateway) recordLog(userID, bucket, key, operation string, status int, size int64, durationMs int64) {
	if !gw.config.EnableLogging {
		return
	}
	gw.accessLog = append(gw.accessLog, AccessLog{
		Timestamp: time.Now(),
		UserID:    userID,
		Bucket:    bucket,
		Key:       key,
		Operation: operation,
		Status:    status,
		Size:      size,
		Duration:  durationMs,
	})
}

// isValidBucketName 校验桶名称（简化版S3命名规则）
func isValidBucketName(name string) bool {
	if len(name) < 3 || len(name) > 63 {
		return false
	}
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return false
		}
	}
	if name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}
	return true
}

// Ensure io.Reader/io.Writer are referenced to avoid import errors
var _ io.Reader
