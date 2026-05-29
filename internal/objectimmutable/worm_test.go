// Package objectimmutable 提供 S3 兼容的不可变对象存储功能
package objectimmutable

import (
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestManager_CreateBucket(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	// 测试创建桶
	bucket, err := mgr.CreateBucket("test-bucket", false, nil)
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	if bucket.Name != "test-bucket" {
		t.Errorf("期望桶名 test-bucket，实际 %s", bucket.Name)
	}

	if bucket.DefaultImmutable {
		t.Error("期望 DefaultImmutable 为 false")
	}

	// 测试重复创建
	_, err = mgr.CreateBucket("test-bucket", false, nil)
	if err != ErrBucketExists {
		t.Errorf("期望 ErrBucketExists，实际 %v", err)
	}
}

func TestManager_CreateBucketWithLockConfig(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	lockConfig := &ObjectLockConfiguration{
		Enabled: true,
		DefaultRetention: &DefaultRetention{
			Mode:  LockModeCompliance,
			Days:  30,
			Years: 0,
		},
	}

	bucket, err := mgr.CreateBucket("immutable-bucket", true, lockConfig)
	if err != nil {
		t.Fatalf("创建桶失败: %v", err)
	}

	if !bucket.DefaultImmutable {
		t.Error("期望 DefaultImmutable 为 true")
	}

	if bucket.ObjectLockConfig == nil || !bucket.ObjectLockConfig.Enabled {
		t.Error("期望 ObjectLockConfig 已启用")
	}
}

func TestManager_GetBucket(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	// 测试获取不存在的桶
	_, err := mgr.GetBucket("non-existent")
	if err != ErrBucketNotFound {
		t.Errorf("期望 ErrBucketNotFound，实际 %v", err)
	}

	// 创建桶后获取
	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	bucket, err := mgr.GetBucket("test-bucket")
	if err != nil {
		t.Fatalf("获取桶失败: %v", err)
	}

	if bucket.Name != "test-bucket" {
		t.Errorf("期望桶名 test-bucket，实际 %s", bucket.Name)
	}
}

func TestManager_ListBuckets(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	// 空列表
	buckets := mgr.ListBuckets()
	if len(buckets) != 0 {
		t.Errorf("期望 0 个桶，实际 %d", len(buckets))
	}

	// 创建多个桶
	_, _ = mgr.CreateBucket("bucket1", false, nil)
	_, _ = mgr.CreateBucket("bucket2", true, nil)

	buckets = mgr.ListBuckets()
	if len(buckets) != 2 {
		t.Errorf("期望 2 个桶，实际 %d", len(buckets))
	}
}

func TestManager_PutObject(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)

	// 测试上传对象
	data := []byte("test data")
	obj, err := mgr.PutObject("test-bucket", "test-key", data, "text/plain")
	if err != nil {
		t.Fatalf("上传对象失败: %v", err)
	}

	if obj.ObjectKey != "test-key" {
		t.Errorf("期望对象键 test-key，实际 %s", obj.ObjectKey)
	}

	if obj.Size != int64(len(data)) {
		t.Errorf("期望大小 %d，实际 %d", len(data), obj.Size)
	}

	if obj.WORMProtected {
		t.Error("期望 WORMProtected 为 false")
	}

	// 测试在不存在的桶上传
	_, err = mgr.PutObject("non-existent", "test-key", data, "text/plain")
	if err != ErrBucketNotFound {
		t.Errorf("期望 ErrBucketNotFound，实际 %v", err)
	}
}

func TestManager_PutObjectWithDefaultLock(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	lockConfig := &ObjectLockConfiguration{
		Enabled: true,
		DefaultRetention: &DefaultRetention{
			Mode:  LockModeGovernance,
			Days:  7,
			Years: 0,
		},
	}

	_, _ = mgr.CreateBucket("locked-bucket", true, lockConfig)

	data := []byte("locked data")
	obj, err := mgr.PutObject("locked-bucket", "locked-key", data, "text/plain")
	if err != nil {
		t.Fatalf("上传对象失败: %v", err)
	}

	if !obj.WORMProtected {
		t.Error("期望 WORMProtected 为 true")
	}

	if obj.Retention == nil {
		t.Fatal("期望 Retention 不为 nil")
	}

	if obj.Retention.Mode != LockModeGovernance {
		t.Errorf("期望锁模式 GOVERNANCE，实际 %s", obj.Retention.Mode)
	}

	if obj.Retention.Status != RetentionStatusLocked {
		t.Errorf("期望状态 LOCKED，实际 %s", obj.Retention.Status)
	}
}

func TestManager_GetObject(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 测试获取对象
	obj, err := mgr.GetObject("test-bucket", "test-key")
	if err != nil {
		t.Fatalf("获取对象失败: %v", err)
	}

	if string(obj.Data) != "test data" {
		t.Errorf("期望数据 'test data'，实际 '%s'", string(obj.Data))
	}

	// 测试获取不存在的对象
	_, err = mgr.GetObject("test-bucket", "non-existent")
	if err != ErrObjectNotFound {
		t.Errorf("期望 ErrObjectNotFound，实际 %v", err)
	}
}

func TestManager_DeleteObject(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 测试删除对象
	err := mgr.DeleteObject("test-bucket", "test-key", "admin", "127.0.0.1", false)
	if err != nil {
		t.Fatalf("删除对象失败: %v", err)
	}

	// 验证对象已删除
	_, err = mgr.GetObject("test-bucket", "test-key")
	if err != ErrObjectNotFound {
		t.Errorf("期望 ErrObjectNotFound，实际 %v", err)
	}
}

func TestManager_DeleteObjectWithLegalHold(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 设置法律保留
	_, _ = mgr.SetObjectLegalHold("test-bucket", "test-key", PutObjectLegalHoldRequest{
		Enabled: true,
		Reason:  "法律诉讼",
	}, "admin", "127.0.0.1")

	// 尝试删除（应该失败）
	err := mgr.DeleteObject("test-bucket", "test-key", "admin", "127.0.0.1", false)
	if err != ErrLegalHoldActive {
		t.Errorf("期望 ErrLegalHoldActive，实际 %v", err)
	}
}

func TestManager_DeleteObjectWithRetention(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 设置保留期（未来时间）
	futureTime := time.Now().AddDate(0, 0, 30)
	_, _ = mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeGovernance,
		RetainUntilDate: futureTime,
	}, "admin", "127.0.0.1")

	// 尝试删除（应该失败，保留期未过期）
	err := mgr.DeleteObject("test-bucket", "test-key", "admin", "127.0.0.1", false)
	if err != ErrObjectLocked {
		t.Errorf("期望 ErrObjectLocked，实际 %v", err)
	}

	// 尝试绕过治理锁定删除
	err = mgr.DeleteObject("test-bucket", "test-key", "admin", "127.0.0.1", true)
	if err != nil {
		t.Fatalf("绕过治理锁定删除失败: %v", err)
	}
}

func TestManager_DeleteObjectWithComplianceRetention(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 设置合规保留期（未来时间）
	futureTime := time.Now().AddDate(0, 0, 30)
	_, _ = mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeCompliance,
		RetainUntilDate: futureTime,
	}, "admin", "127.0.0.1")

	// 尝试绕过治理锁定删除（应该失败，合规模式不允许绕过）
	err := mgr.DeleteObject("test-bucket", "test-key", "admin", "127.0.0.1", true)
	if err != ErrObjectLocked {
		t.Errorf("期望 ErrObjectLocked，实际 %v", err)
	}
}

func TestManager_SetObjectRetention(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 设置保留期
	futureTime := time.Now().AddDate(0, 0, 30)
	retention, err := mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeGovernance,
		RetainUntilDate: futureTime,
	}, "admin", "127.0.0.1")
	if err != nil {
		t.Fatalf("设置保留期失败: %v", err)
	}

	if retention.Mode != LockModeGovernance {
		t.Errorf("期望模式 GOVERNANCE，实际 %s", retention.Mode)
	}

	if retention.Status != RetentionStatusLocked {
		t.Errorf("期望状态 LOCKED，实际 %s", retention.Status)
	}

	// 延长保留期
	futureTime2 := time.Now().AddDate(0, 0, 60)
	retention2, err := mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeGovernance,
		RetainUntilDate: futureTime2,
	}, "admin", "127.0.0.1")
	if err != nil {
		t.Fatalf("延长保留期失败: %v", err)
	}

	if !retention2.RetainUntilDate.After(retention.RetainUntilDate) {
		t.Error("期望延长后的保留期更晚")
	}
}

func TestManager_SetObjectRetentionComplianceMode(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 设置合规保留期
	futureTime := time.Now().AddDate(0, 0, 30)
	_, err := mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeCompliance,
		RetainUntilDate: futureTime,
	}, "admin", "127.0.0.1")
	if err != nil {
		t.Fatalf("设置保留期失败: %v", err)
	}

	// 尝试缩短保留期（应该失败）
	pastTime := time.Now().AddDate(0, 0, 10)
	_, err = mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeCompliance,
		RetainUntilDate: pastTime,
	}, "admin", "127.0.0.1")
	if err != ErrWORMViolation {
		t.Errorf("期望 ErrWORMViolation，实际 %v", err)
	}

	// 尝试降级为治理模式（应该失败）
	_, err = mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeGovernance,
		RetainUntilDate: futureTime,
	}, "admin", "127.0.0.1")
	if err != ErrWORMViolation {
		t.Errorf("期望 ErrWORMViolation，实际 %v", err)
	}
}

func TestManager_SetObjectLegalHold(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 设置法律保留
	legalHold, err := mgr.SetObjectLegalHold("test-bucket", "test-key", PutObjectLegalHoldRequest{
		Enabled: true,
		Reason:  "法律诉讼",
		SetBy:   "法务部",
	}, "admin", "127.0.0.1")
	if err != nil {
		t.Fatalf("设置法律保留失败: %v", err)
	}

	if !legalHold.Enabled {
		t.Error("期望 Enabled 为 true")
	}

	if legalHold.Reason != "法律诉讼" {
		t.Errorf("期望原因 '法律诉讼'，实际 '%s'", legalHold.Reason)
	}

	// 获取法律保留
	legalHold2, err := mgr.GetObjectLegalHold("test-bucket", "test-key")
	if err != nil {
		t.Fatalf("获取法律保留失败: %v", err)
	}

	if !legalHold2.Enabled {
		t.Error("期望 Enabled 为 true")
	}

	// 释放法律保留
	_, err = mgr.SetObjectLegalHold("test-bucket", "test-key", PutObjectLegalHoldRequest{
		Enabled: false,
	}, "admin", "127.0.0.1")
	if err != nil {
		t.Fatalf("释放法律保留失败: %v", err)
	}
}

func TestManager_ReleaseObjectRetention(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 设置治理保留期（过去时间）
	pastTime := time.Now().AddDate(0, 0, -1)
	_, _ = mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeGovernance,
		RetainUntilDate: pastTime,
	}, "admin", "127.0.0.1")

	// 释放保留（应该成功，因为保留期已过期）
	err := mgr.ReleaseObjectRetention("test-bucket", "test-key", "admin", "127.0.0.1")
	if err != nil {
		t.Fatalf("释放保留失败: %v", err)
	}

	// 验证状态已更新
	retention, _ := mgr.GetObjectRetention("test-bucket", "test-key")
	if retention.Status != RetentionStatusExpired {
		t.Errorf("期望状态 EXPIRED，实际 %s", retention.Status)
	}
}

func TestManager_ReleaseObjectRetentionNotExpired(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 设置治理保留期（未来时间）
	futureTime := time.Now().AddDate(0, 0, 30)
	_, _ = mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeGovernance,
		RetainUntilDate: futureTime,
	}, "admin", "127.0.0.1")

	// 尝试释放保留（应该失败，保留期未过期）
	err := mgr.ReleaseObjectRetention("test-bucket", "test-key", "admin", "127.0.0.1")
	if err != ErrRetentionExpired {
		t.Errorf("期望 ErrRetentionExpired，实际 %v", err)
	}
}

func TestManager_ReleaseComplianceRetention(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 设置合规保留期（过去时间）
	pastTime := time.Now().AddDate(0, 0, -1)
	_, _ = mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeCompliance,
		RetainUntilDate: pastTime,
	}, "admin", "127.0.0.1")

	// 尝试释放合规保留（应该失败）
	err := mgr.ReleaseObjectRetention("test-bucket", "test-key", "admin", "127.0.0.1")
	if err != ErrWORMViolation {
		t.Errorf("期望 ErrWORMViolation，实际 %v", err)
	}
}

func TestManager_ListObjects(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)

	// 上传多个对象
	_, _ = mgr.PutObject("test-bucket", "prefix/file1.txt", []byte("data1"), "text/plain")
	_, _ = mgr.PutObject("test-bucket", "prefix/file2.txt", []byte("data2"), "text/plain")
	_, _ = mgr.PutObject("test-bucket", "other/file3.txt", []byte("data3"), "text/plain")

	// 列出所有对象
	resp, err := mgr.ListObjects(ListObjectsRequest{
		BucketName: "test-bucket",
	})
	if err != nil {
		t.Fatalf("列出对象失败: %v", err)
	}

	if len(resp.Objects) != 3 {
		t.Errorf("期望 3 个对象，实际 %d", len(resp.Objects))
	}

	// 按前缀过滤
	resp, err = mgr.ListObjects(ListObjectsRequest{
		BucketName: "test-bucket",
		Prefix:     "prefix/",
	})
	if err != nil {
		t.Fatalf("列出对象失败: %v", err)
	}

	if len(resp.Objects) != 2 {
		t.Errorf("期望 2 个对象，实际 %d", len(resp.Objects))
	}
}

func TestManager_AuditLogs(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 设置保留期
	futureTime := time.Now().AddDate(0, 0, 30)
	_, _ = mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeGovernance,
		RetainUntilDate: futureTime,
	}, "admin", "127.0.0.1")

	// 设置法律保留
	_, _ = mgr.SetObjectLegalHold("test-bucket", "test-key", PutObjectLegalHoldRequest{
		Enabled: true,
		Reason:  "法律诉讼",
	}, "admin", "127.0.0.1")

	// 获取审计日志
	logs := mgr.ListAuditLogs(ListAuditLogsRequest{})
	if logs.Total < 2 {
		t.Errorf("期望至少 2 条审计日志，实际 %d", logs.Total)
	}

	// 按对象键过滤
	logs = mgr.ListAuditLogs(ListAuditLogsRequest{
		ObjectKey: "test-key",
	})
	for _, log := range logs.Logs {
		if log.ObjectKey != "test-key" {
			t.Errorf("期望对象键 test-key，实际 %s", log.ObjectKey)
		}
	}
}

func TestManager_GetStats(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("bucket1", false, nil)
	_, _ = mgr.CreateBucket("bucket2", true, nil)

	_, _ = mgr.PutObject("bucket1", "key1", []byte("data1"), "text/plain")
	_, _ = mgr.PutObject("bucket1", "key2", []byte("data2"), "text/plain")
	_, _ = mgr.PutObject("bucket2", "key3", []byte("data3"), "text/plain")

	// 设置法律保留
	_, _ = mgr.SetObjectLegalHold("bucket1", "key1", PutObjectLegalHoldRequest{
		Enabled: true,
	}, "admin", "127.0.0.1")

	stats := mgr.GetStats()

	if stats.TotalBuckets != 2 {
		t.Errorf("期望 2 个桶，实际 %d", stats.TotalBuckets)
	}

	if stats.TotalObjects != 3 {
		t.Errorf("期望 3 个对象，实际 %d", stats.TotalObjects)
	}

	if stats.LegalHoldObjects != 1 {
		t.Errorf("期望 1 个法律保留对象，实际 %d", stats.LegalHoldObjects)
	}
}

func TestManager_SetBucketObjectLockConfig(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)

	lockConfig := &ObjectLockConfiguration{
		Enabled: true,
		DefaultRetention: &DefaultRetention{
			Mode:  LockModeGovernance,
			Days:  30,
			Years: 0,
		},
	}

	bucket, err := mgr.SetBucketObjectLockConfig("test-bucket", lockConfig)
	if err != nil {
		t.Fatalf("设置桶对象锁定配置失败: %v", err)
	}

	if bucket.ObjectLockConfig == nil || !bucket.ObjectLockConfig.Enabled {
		t.Error("期望 ObjectLockConfig 已启用")
	}

	// 获取配置
	config, err := mgr.GetBucketObjectLockConfig("test-bucket")
	if err != nil {
		t.Fatalf("获取桶对象锁定配置失败: %v", err)
	}

	if !config.Enabled {
		t.Error("期望 Enabled 为 true")
	}
}

func TestManager_InvalidRetentionRequests(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 测试无效的锁定模式
	futureTime := time.Now().AddDate(0, 0, 30)
	_, err := mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            "INVALID_MODE",
		RetainUntilDate: futureTime,
	}, "admin", "127.0.0.1")
	if err != ErrInvalidLockMode {
		t.Errorf("期望 ErrInvalidLockMode，实际 %v", err)
	}
}

func TestManager_ConcurrentAccess(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)

	// 并发写入
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			data := []byte("data")
			_, _ = mgr.PutObject("test-bucket", "key", data, "text/plain")
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	// 并发读取
	for i := 0; i < 10; i++ {
		go func() {
			_, _ = mgr.GetObject("test-bucket", "key")
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestManager_PutObjectOverwriteLockedObject(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 设置保留期
	futureTime := time.Now().AddDate(0, 0, 30)
	_, _ = mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeGovernance,
		RetainUntilDate: futureTime,
	}, "admin", "127.0.0.1")

	// 尝试覆盖已锁定的对象（应该失败）
	newData := []byte("new data")
	_, err := mgr.PutObject("test-bucket", "test-key", newData, "text/plain")
	if err != ErrObjectLocked {
		t.Errorf("期望 ErrObjectLocked，实际 %v", err)
	}
}

func TestManager_GetObjectRetention(t *testing.T) {
	logger := zap.NewNop()
	mgr := NewManager(logger)

	_, _ = mgr.CreateBucket("test-bucket", false, nil)
	data := []byte("test data")
	_, _ = mgr.PutObject("test-bucket", "test-key", data, "text/plain")

	// 获取不存在的保留期
	_, err := mgr.GetObjectRetention("test-bucket", "test-key")
	if err != ErrObjectNotFound {
		t.Errorf("期望 ErrObjectNotFound，实际 %v", err)
	}

	// 设置保留期后获取
	futureTime := time.Now().AddDate(0, 0, 30)
	_, _ = mgr.SetObjectRetention("test-bucket", "test-key", PutObjectRetentionRequest{
		Mode:            LockModeGovernance,
		RetainUntilDate: futureTime,
	}, "admin", "127.0.0.1")

	retention, err := mgr.GetObjectRetention("test-bucket", "test-key")
	if err != nil {
		t.Fatalf("获取保留期失败: %v", err)
	}

	if retention.Mode != LockModeGovernance {
		t.Errorf("期望模式 GOVERNANCE，实际 %s", retention.Mode)
	}
}
