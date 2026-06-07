// Package objectimmutable 提供 S3 兼容的不可变对象存储功能
package objectimmutable

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager WORM 存储管理器.
type Manager struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	buckets    map[string]*ImmutableBucket
	objects    map[string]map[string]*ImmutableObject // bucketName -> objectKey -> object
	auditLogs  []*RetentionAuditEntry
	configPath string
}

// NewManager 创建 WORM 管理器.
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger:    logger,
		buckets:   make(map[string]*ImmutableBucket),
		objects:   make(map[string]map[string]*ImmutableObject),
		auditLogs: make([]*RetentionAuditEntry, 0),
	}
}

// NewManagerWithPath 创建带持久化路径的 WORM 管理器.
func NewManagerWithPath(logger *zap.Logger, configPath string) *Manager {
	m := &Manager{
		logger:     logger,
		buckets:    make(map[string]*ImmutableBucket),
		objects:    make(map[string]map[string]*ImmutableObject),
		auditLogs:  make([]*RetentionAuditEntry, 0),
		configPath: configPath,
	}

	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			logger.Warn("加载 WORM 配置失败，使用空配置", zap.Error(err))
		}
	}

	return m
}

// ========== 桶管理 ==========

// CreateBucket 创建不可变桶.
func (m *Manager) CreateBucket(name string, defaultImmutable bool, lockConfig *ObjectLockConfiguration) (*ImmutableBucket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.buckets[name]; exists {
		return nil, ErrBucketExists
	}

	now := time.Now()
	bucket := &ImmutableBucket{
		Name:             name,
		ObjectLockConfig: lockConfig,
		DefaultImmutable: defaultImmutable,
		ObjectCount:      0,
		TotalSize:        0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	m.buckets[name] = bucket
	m.objects[name] = make(map[string]*ImmutableObject)

	m.logger.Info("创建不可变桶",
		zap.String("bucket", name),
		zap.Bool("defaultImmutable", defaultImmutable),
	)

	return bucket, nil
}

// GetBucket 获取桶信息.
func (m *Manager) GetBucket(name string) (*ImmutableBucket, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bucket, exists := m.buckets[name]
	if !exists {
		return nil, ErrBucketNotFound
	}

	return bucket, nil
}

// ListBuckets 列出所有桶.
func (m *Manager) ListBuckets() []*ImmutableBucket {
	m.mu.RLock()
	defer m.mu.RUnlock()

	buckets := make([]*ImmutableBucket, 0, len(m.buckets))
	for _, bucket := range m.buckets {
		buckets = append(buckets, bucket)
	}

	return buckets
}

// SetBucketObjectLockConfig 设置桶的对象锁定配置.
func (m *Manager) SetBucketObjectLockConfig(name string, config *ObjectLockConfiguration) (*ImmutableBucket, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bucket, exists := m.buckets[name]
	if !exists {
		return nil, ErrBucketNotFound
	}

	bucket.ObjectLockConfig = config
	bucket.UpdatedAt = time.Now()

	m.logger.Info("设置桶对象锁定配置",
		zap.String("bucket", name),
		zap.Bool("enabled", config.Enabled),
	)

	return bucket, nil
}

// GetBucketObjectLockConfig 获取桶的对象锁定配置.
func (m *Manager) GetBucketObjectLockConfig(name string) (*ObjectLockConfiguration, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bucket, exists := m.buckets[name]
	if !exists {
		return nil, ErrBucketNotFound
	}

	return bucket.ObjectLockConfig, nil
}

// ========== 对象管理 ==========

// PutObject 上传对象.
func (m *Manager) PutObject(bucketName, objectKey string, data []byte, contentType string) (*ImmutableObject, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	bucket, exists := m.buckets[bucketName]
	if !exists {
		return nil, ErrBucketNotFound
	}

	// 检查对象是否已存在且被锁定
	if existingObj, exists := m.objects[bucketName][objectKey]; exists {
		if existingObj.WORMProtected {
			return nil, ErrObjectLocked
		}
		if existingObj.LegalHold != nil && existingObj.LegalHold.Enabled {
			return nil, ErrLegalHoldActive
		}
		if existingObj.Retention != nil && existingObj.Retention.Status == RetentionStatusLocked {
			if existingObj.Retention.RetainUntilDate.After(time.Now()) {
				return nil, ErrObjectLocked
			}
		}
	}

	// 计算 ETag
	etag := calculateETag(data)

	now := time.Now()
	obj := &ImmutableObject{
		ObjectKey:     objectKey,
		BucketName:    bucketName,
		Size:          int64(len(data)),
		ETag:          etag,
		ContentType:   contentType,
		Data:          data,
		WORMProtected: bucket.DefaultImmutable,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// 如果桶启用了对象锁定，自动设置默认保留
	if bucket.ObjectLockConfig != nil && bucket.ObjectLockConfig.Enabled && bucket.ObjectLockConfig.DefaultRetention != nil {
		retention := bucket.ObjectLockConfig.DefaultRetention
		retainUntil := calculateRetainUntil(retention.Mode, retention.Days, retention.Years)

		obj.Retention = &ObjectRetention{
			Mode:            retention.Mode,
			RetainUntilDate: retainUntil,
			Status:          RetentionStatusLocked,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		obj.WORMProtected = true
	}

	m.objects[bucketName][objectKey] = obj
	bucket.ObjectCount++
	bucket.TotalSize += obj.Size
	bucket.UpdatedAt = now

	m.logger.Info("上传对象",
		zap.String("bucket", bucketName),
		zap.String("objectKey", objectKey),
		zap.Int64("size", obj.Size),
		zap.Bool("wormProtected", obj.WORMProtected),
	)

	return obj, nil
}

// GetObject 获取对象.
func (m *Manager) GetObject(bucketName, objectKey string) (*ImmutableObject, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	bucketObjects, exists := m.objects[bucketName]
	if !exists {
		return nil, ErrBucketNotFound
	}

	obj, exists := bucketObjects[objectKey]
	if !exists {
		return nil, ErrObjectNotFound
	}

	return obj, nil
}

// DeleteObject 删除对象.
func (m *Manager) DeleteObject(bucketName, objectKey string, operator, ipAddress string, bypassGovernance bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	bucketObjects, exists := m.objects[bucketName]
	if !exists {
		return ErrBucketNotFound
	}

	obj, exists := bucketObjects[objectKey]
	if !exists {
		return ErrObjectNotFound
	}

	// 检查法律保留（最高优先级）
	if obj.LegalHold != nil && obj.LegalHold.Enabled {
		m.addAuditLog(objectKey, bucketName, AuditActionDeleteBlocked, nil, nil, "法律保留生效", operator, ipAddress, false, "法律保留生效中，无法删除")
		return ErrLegalHoldActive
	}

	// 检查保留期
	if obj.Retention != nil && obj.Retention.Status == RetentionStatusLocked {
		if obj.Retention.RetainUntilDate.After(time.Now()) {
			// 治理模式下可以绕过
			if obj.Retention.Mode == LockModeGovernance && bypassGovernance {
				m.logger.Info("绕过治理锁定删除对象",
					zap.String("bucket", bucketName),
					zap.String("objectKey", objectKey),
					zap.String("operator", operator),
				)
			} else {
				m.addAuditLog(objectKey, bucketName, AuditActionDeleteBlocked, obj.Retention, nil, "保留期未过期", operator, ipAddress, false, "保留期未过期，无法删除")
				return ErrObjectLocked
			}
		}
	}

	// 检查 WORM 保护（仅对非保留期导致的不可变生效）
	if obj.WORMProtected && (obj.Retention == nil || obj.Retention.Status != RetentionStatusLocked) {
		m.addAuditLog(objectKey, bucketName, AuditActionDeleteBlocked, nil, nil, "WORM 保护", operator, ipAddress, false, "对象处于 WORM 保护状态")
		return ErrWORMViolation
	}

	// 记录删除尝试
	m.addAuditLog(objectKey, bucketName, AuditActionDeleteAttempt, obj.Retention, nil, "删除对象", operator, ipAddress, true, "")

	// 执行删除
	delete(bucketObjects, objectKey)
	bucket := m.buckets[bucketName]
	bucket.ObjectCount--
	bucket.TotalSize -= obj.Size
	bucket.UpdatedAt = time.Now()

	m.logger.Info("删除对象",
		zap.String("bucket", bucketName),
		zap.String("objectKey", objectKey),
	)

	return nil
}

// ListObjects 列出桶中的对象.
func (m *Manager) ListObjects(req ListObjectsRequest) (*ListObjectsResponse, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.objects[req.BucketName]
	if !exists {
		return nil, ErrBucketNotFound
	}

	objects := make([]*ImmutableObject, 0)
	count := 0
	maxKeys := req.MaxKeys
	if maxKeys <= 0 {
		maxKeys = 1000
	}

	startAfter := req.ContinuationToken
	started := startAfter == ""

	for _, obj := range m.objects[req.BucketName] {
		// 前缀过滤
		if req.Prefix != "" && len(obj.ObjectKey) >= len(req.Prefix) {
			if obj.ObjectKey[:len(req.Prefix)] != req.Prefix {
				continue
			}
		}

		// 分页
		if !started {
			if obj.ObjectKey == startAfter {
				started = true
			}
			continue
		}

		if count >= maxKeys {
			return &ListObjectsResponse{
				Objects:               objects,
				IsTruncated:           true,
				NextContinuationToken: obj.ObjectKey,
			}, nil
		}

		objects = append(objects, obj)
		count++
	}

	return &ListObjectsResponse{
		Objects:     objects,
		IsTruncated: false,
	}, nil
}

// ========== 保留管理 ==========

// SetObjectRetention 设置对象保留.
func (m *Manager) SetObjectRetention(bucketName, objectKey string, req PutObjectRetentionRequest, operator, ipAddress string) (*ObjectRetention, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	obj, err := m.getObjectInternal(bucketName, objectKey)
	if err != nil {
		return nil, err
	}

	// 验证锁定模式
	if req.Mode != LockModeGovernance && req.Mode != LockModeCompliance {
		return nil, ErrInvalidLockMode
	}

	// 检查是否可以缩短保留期（合规模式不允许）
	if obj.Retention != nil && obj.Retention.Status == RetentionStatusLocked {
		if req.Mode == LockModeCompliance && req.RetainUntilDate.Before(obj.Retention.RetainUntilDate) {
			m.addAuditLog(objectKey, bucketName, AuditActionWORMViolation, obj.Retention, nil, "合规模式不允许缩短保留期", operator, ipAddress, false, "合规模式不允许缩短保留期")
			return nil, ErrWORMViolation
		}

		if obj.Retention.Mode == LockModeCompliance && req.Mode == LockModeGovernance {
			m.addAuditLog(objectKey, bucketName, AuditActionWORMViolation, obj.Retention, nil, "合规模式不能降级为治理模式", operator, ipAddress, false, "合规模式不能降级为治理模式")
			return nil, ErrWORMViolation
		}
	}

	oldRetention := obj.Retention
	now := time.Now()
	newRetention := &ObjectRetention{
		Mode:            req.Mode,
		RetainUntilDate: req.RetainUntilDate,
		Status:          RetentionStatusLocked,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	// 如果是延长保留期
	if oldRetention != nil {
		newRetention.CreatedAt = oldRetention.CreatedAt
		m.addAuditLog(objectKey, bucketName, AuditActionRetentionExtend, oldRetention, newRetention, "延长保留期", operator, ipAddress, true, "")
	} else {
		m.addAuditLog(objectKey, bucketName, AuditActionRetentionSet, nil, newRetention, "设置保留期", operator, ipAddress, true, "")
	}

	obj.Retention = newRetention
	obj.WORMProtected = true
	obj.UpdatedAt = now

	m.logger.Info("设置对象保留",
		zap.String("bucket", bucketName),
		zap.String("objectKey", objectKey),
		zap.String("mode", string(req.Mode)),
		zap.Time("retainUntil", req.RetainUntilDate),
	)

	return newRetention, nil
}

// GetObjectRetention 获取对象保留信息.
func (m *Manager) GetObjectRetention(bucketName, objectKey string) (*ObjectRetention, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	obj, err := m.getObjectInternal(bucketName, objectKey)
	if err != nil {
		return nil, err
	}

	if obj.Retention == nil {
		return nil, ErrObjectNotFound
	}

	return obj.Retention, nil
}

// ReleaseObjectRetention 释放对象保留.
func (m *Manager) ReleaseObjectRetention(bucketName, objectKey string, operator, ipAddress string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	obj, err := m.getObjectInternal(bucketName, objectKey)
	if err != nil {
		return err
	}

	if obj.Retention == nil {
		return nil
	}

	// 合规模式不允许释放
	if obj.Retention.Mode == LockModeCompliance {
		m.addAuditLog(objectKey, bucketName, AuditActionWORMViolation, obj.Retention, nil, "合规模式不允许释放保留", operator, ipAddress, false, "合规模式不允许释放保留")
		return ErrWORMViolation
	}

	// 检查保留期是否已过期
	if obj.Retention.RetainUntilDate.After(time.Now()) {
		m.addAuditLog(objectKey, bucketName, AuditActionRetentionRelease, obj.Retention, nil, "保留期未过期", operator, ipAddress, false, "保留期未过期，无法释放")
		return ErrRetentionExpired
	}

	m.addAuditLog(objectKey, bucketName, AuditActionRetentionRelease, obj.Retention, nil, "释放保留", operator, ipAddress, true, "")

	obj.Retention.Status = RetentionStatusExpired
	obj.WORMProtected = false
	obj.UpdatedAt = time.Now()

	m.logger.Info("释放对象保留",
		zap.String("bucket", bucketName),
		zap.String("objectKey", objectKey),
	)

	return nil
}

// ========== 法律保留管理 ==========

// SetObjectLegalHold 设置对象法律保留.
func (m *Manager) SetObjectLegalHold(bucketName, objectKey string, req PutObjectLegalHoldRequest, operator, ipAddress string) (*LegalHold, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	obj, err := m.getObjectInternal(bucketName, objectKey)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	legalHold := &LegalHold{
		Enabled:   req.Enabled,
		Reason:    req.Reason,
		SetBy:     req.SetBy,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if obj.LegalHold != nil {
		legalHold.CreatedAt = obj.LegalHold.CreatedAt
	}

	obj.LegalHold = legalHold
	obj.UpdatedAt = now

	if req.Enabled {
		m.addAuditLog(objectKey, bucketName, AuditActionLegalHoldSet, nil, nil, req.Reason, operator, ipAddress, true, "")
	} else {
		m.addAuditLog(objectKey, bucketName, AuditActionLegalHoldRelease, nil, nil, req.Reason, operator, ipAddress, true, "")
	}

	m.logger.Info("设置对象法律保留",
		zap.String("bucket", bucketName),
		zap.String("objectKey", objectKey),
		zap.Bool("enabled", req.Enabled),
	)

	return legalHold, nil
}

// GetObjectLegalHold 获取对象法律保留.
func (m *Manager) GetObjectLegalHold(bucketName, objectKey string) (*LegalHold, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	obj, err := m.getObjectInternal(bucketName, objectKey)
	if err != nil {
		return nil, err
	}

	if obj.LegalHold == nil {
		return &LegalHold{
			Enabled: false,
		}, nil
	}

	return obj.LegalHold, nil
}

// ========== 审计日志 ==========

// ListAuditLogs 列出审计日志.
func (m *Manager) ListAuditLogs(req ListAuditLogsRequest) *ListAuditLogsResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	logs := make([]*RetentionAuditEntry, 0)
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}

	for _, entry := range m.auditLogs {
		// 过滤
		if req.ObjectKey != "" && entry.ObjectKey != req.ObjectKey {
			continue
		}
		if req.BucketName != "" && entry.BucketName != req.BucketName {
			continue
		}
		if req.Action != "" && entry.Action != req.Action {
			continue
		}
		if req.StartTime != nil && entry.Timestamp.Before(*req.StartTime) {
			continue
		}
		if req.EndTime != nil && entry.Timestamp.After(*req.EndTime) {
			continue
		}

		logs = append(logs, entry)

		if len(logs) >= maxResults {
			break
		}
	}

	return &ListAuditLogsResponse{
		Logs:  logs,
		Total: len(m.auditLogs),
	}
}

// ========== 统计 ==========

// GetStats 获取 WORM 统计信息.
func (m *Manager) GetStats() *WORMStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &WORMStats{
		TotalBuckets:  len(m.buckets),
		AuditLogCount: len(m.auditLogs),
	}

	for _, bucket := range m.buckets {
		stats.TotalObjects += bucket.ObjectCount
		stats.TotalSize += bucket.TotalSize
	}

	for _, bucketObjects := range m.objects {
		for _, obj := range bucketObjects {
			if obj.WORMProtected {
				stats.ProtectedObjects++
			}
			if obj.LegalHold != nil && obj.LegalHold.Enabled {
				stats.LegalHoldObjects++
			}
		}
	}

	return stats
}

// ========== 内部方法 ==========

// getObjectInternal 获取对象（内部方法，需要调用方持有锁）.
func (m *Manager) getObjectInternal(bucketName, objectKey string) (*ImmutableObject, error) {
	bucketObjects, exists := m.objects[bucketName]
	if !exists {
		return nil, ErrBucketNotFound
	}

	obj, exists := bucketObjects[objectKey]
	if !exists {
		return nil, ErrObjectNotFound
	}

	return obj, nil
}

// addAuditLog 添加审计日志.
func (m *Manager) addAuditLog(objectKey, bucketName string, action AuditAction, oldRetention, newRetention *ObjectRetention, reason, operator, ipAddress string, success bool, errorMessage string) {
	entry := &RetentionAuditEntry{
		ID:           generateAuditID(),
		ObjectKey:    objectKey,
		BucketName:   bucketName,
		Action:       action,
		OldRetention: oldRetention,
		NewRetention: newRetention,
		Reason:       reason,
		Operator:     operator,
		Timestamp:    time.Now(),
		IPAddress:    ipAddress,
		Success:      success,
		ErrorMessage: errorMessage,
	}

	m.auditLogs = append(m.auditLogs, entry)
}

// generateAuditID 生成审计 ID.
func generateAuditID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// calculateETag 计算 ETag.
func calculateETag(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// calculateRetainUntil 计算保留截止日期.
func calculateRetainUntil(mode LockMode, days, years int) time.Time {
	now := time.Now()
	if years > 0 {
		return now.AddDate(years, 0, 0)
	}
	if days > 0 {
		return now.AddDate(0, 0, days)
	}
	// 默认保留 1 天
	return now.AddDate(0, 0, 1)
}

// ========== 持久化 ==========

// wormData 持久化数据结构.
type wormData struct {
	Buckets   map[string]*ImmutableBucket            `json:"buckets"`
	Objects   map[string]map[string]*ImmutableObject `json:"objects"`
	AuditLogs []*RetentionAuditEntry                 `json:"audit_logs"`
}

// SaveConfig 保存配置到文件.
func (m *Manager) SaveConfig() error {
	if m.configPath == "" {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	data := wormData{
		Buckets:   m.buckets,
		Objects:   m.objects,
		AuditLogs: m.auditLogs,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	if err := os.WriteFile(m.configPath, jsonData, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// loadConfig 从文件加载配置.
func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config wormData
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	if config.Buckets != nil {
		m.buckets = config.Buckets
	}
	if config.Objects != nil {
		m.objects = config.Objects
	}
	if config.AuditLogs != nil {
		m.auditLogs = config.AuditLogs
	}

	return nil
}
