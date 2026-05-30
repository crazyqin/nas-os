package fileversioncontrol

import (
	"fmt"
	"testing"
)

func TestNewFileVersionControl(t *testing.T) {
	fvc := NewFileVersionControl(nil)
	if fvc == nil {
		t.Fatal("Expected FileVersionControl instance, got nil")
	}

	// 验证默认配置
	if fvc.config.AutoSnapshotOnModify != true {
		t.Error("Expected AutoSnapshotOnModify to be true")
	}

	if fvc.config.MaxVersionsPerFile != 100 {
		t.Errorf("Expected MaxVersionsPerFile 100, got %d", fvc.config.MaxVersionsPerFile)
	}

	if fvc.config.ChecksumAlgorithm != "sha256" {
		t.Errorf("Expected ChecksumAlgorithm 'sha256', got %s", fvc.config.ChecksumAlgorithm)
	}
}

func TestCreateVersion(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	content := []byte("Hello, World!")
	version, err := fvc.CreateVersion("/test/file.txt", int64(len(content)), content, "Initial version", "user1")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	if version.Version != 1 {
		t.Errorf("Expected version 1, got %d", version.Version)
	}

	if version.Status != StatusCurrent {
		t.Errorf("Expected status 'current', got %s", version.Status)
	}

	if version.Comment != "Initial version" {
		t.Errorf("Expected comment 'Initial version', got %s", version.Comment)
	}

	if version.CreatedBy != "user1" {
		t.Errorf("Expected creator 'user1', got %s", version.CreatedBy)
	}
}

func TestCreateVersionWithSameContent(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	content := []byte("Same content")

	// 创建第一个版本
	version1, err := fvc.CreateVersion("/test/file.txt", int64(len(content)), content, "Version 1", "user1")
	if err != nil {
		t.Fatalf("Failed to create version 1: %v", err)
	}

	// 尝试创建相同内容的版本
	version2, err := fvc.CreateVersion("/test/file.txt", int64(len(content)), content, "Version 2", "user1")
	if err != nil {
		t.Fatalf("Failed to create version 2: %v", err)
	}

	// 应该返回相同的版本（因为内容相同）
	if version1.Version != version2.Version {
		t.Errorf("Expected same version number for same content, got %d and %d", version1.Version, version2.Version)
	}
}

func TestCreateMultipleVersions(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	// 创建多个版本
	for i := 1; i <= 5; i++ {
		content := []byte(fmt.Sprintf("Content version %d", i))
		_, err := fvc.CreateVersion("/test/file.txt", int64(len(content)), content, fmt.Sprintf("Version %d", i), "user1")
		if err != nil {
			t.Fatalf("Failed to create version %d: %v", i, err)
		}
	}

	// 验证版本历史
	history := fvc.GetVersionHistory("/test/file.txt", 0)
	if len(history) != 5 {
		t.Errorf("Expected 5 versions, got %d", len(history))
	}

	// 验证最新版本
	latest, err := fvc.GetLatestVersion("/test/file.txt")
	if err != nil {
		t.Fatalf("Failed to get latest version: %v", err)
	}

	if latest.Version != 5 {
		t.Errorf("Expected latest version 5, got %d", latest.Version)
	}

	if latest.Status != StatusCurrent {
		t.Errorf("Expected latest status 'current', got %s", latest.Status)
	}

	// 验证历史版本状态
	for _, v := range history[1:] {
		if v.Status != StatusPrevious {
			t.Errorf("Expected previous version status 'previous', got %s for version %d", v.Status, v.Version)
		}
	}
}

func TestGetVersion(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	content := []byte("Test content")
	_, err := fvc.CreateVersion("/test/file.txt", int64(len(content)), content, "Test", "user1")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	// 获取存在的版本
	version, err := fvc.GetVersion("/test/file.txt", 1)
	if err != nil {
		t.Fatalf("Failed to get version: %v", err)
	}

	if version.Version != 1 {
		t.Errorf("Expected version 1, got %d", version.Version)
	}

	// 获取不存在的版本
	_, err = fvc.GetVersion("/test/file.txt", 999)
	if err == nil {
		t.Error("Expected error for non-existent version, got nil")
	}
}

func TestGetLatestVersion(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	// 测试空文件
	_, err := fvc.GetLatestVersion("/nonexistent/file.txt")
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}

	// 添加版本
	content := []byte("Test content")
	_, err = fvc.CreateVersion("/test/file.txt", int64(len(content)), content, "Test", "user1")
	if err != nil {
		t.Fatalf("Failed to create version: %v", err)
	}

	latest, err := fvc.GetLatestVersion("/test/file.txt")
	if err != nil {
		t.Fatalf("Failed to get latest version: %v", err)
	}

	if latest.Version != 1 {
		t.Errorf("Expected version 1, got %d", latest.Version)
	}
}

func TestGetVersionHistory(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	// 创建多个版本
	for i := 1; i <= 10; i++ {
		content := []byte(fmt.Sprintf("Content %d", i))
		_, err := fvc.CreateVersion("/test/file.txt", int64(len(content)), content, fmt.Sprintf("V%d", i), "user1")
		if err != nil {
			t.Fatalf("Failed to create version %d: %v", i, err)
		}
	}

	// 获取所有历史
	allHistory := fvc.GetVersionHistory("/test/file.txt", 0)
	if len(allHistory) != 10 {
		t.Errorf("Expected 10 versions, got %d", len(allHistory))
	}

	// 获取有限历史
	limitedHistory := fvc.GetVersionHistory("/test/file.txt", 5)
	if len(limitedHistory) != 5 {
		t.Errorf("Expected 5 versions, got %d", len(limitedHistory))
	}

	// 验证按版本号降序排序
	if limitedHistory[0].Version != 10 {
		t.Errorf("Expected first version 10, got %d", limitedHistory[0].Version)
	}
}

func TestCreateSnapshot(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	// 创建一些版本
	for i := 1; i <= 3; i++ {
		content := []byte(fmt.Sprintf("Content %d", i))
		fvc.CreateVersion(fmt.Sprintf("/test/file%d.txt", i), int64(len(content)), content, "Test", "user1")
	}

	// 创建快照
	filePaths := []string{"/test/file1.txt", "/test/file2.txt", "/test/file3.txt"}
	snapshot, err := fvc.CreateSnapshot("Test Snapshot", "Snapshot for testing", filePaths, "user1")
	if err != nil {
		t.Fatalf("Failed to create snapshot: %v", err)
	}

	if snapshot.Name != "Test Snapshot" {
		t.Errorf("Expected snapshot name 'Test Snapshot', got %s", snapshot.Name)
	}

	if len(snapshot.Versions) != 3 {
		t.Errorf("Expected 3 file versions in snapshot, got %d", len(snapshot.Versions))
	}

	if snapshot.TotalSize == 0 {
		t.Error("Expected non-zero total size")
	}
}

func TestRollbackToVersion(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	// 创建多个版本
	for i := 1; i <= 5; i++ {
		content := []byte(fmt.Sprintf("Content version %d", i))
		_, err := fvc.CreateVersion("/test/file.txt", int64(len(content)), content, fmt.Sprintf("Version %d", i), "user1")
		if err != nil {
			t.Fatalf("Failed to create version %d: %v", i, err)
		}
	}

	// 回滚到版本2
	rollbackVersion, err := fvc.RollbackToVersion("/test/file.txt", 2)
	if err != nil {
		t.Fatalf("Failed to rollback: %v", err)
	}

	// 验证创建了新版本
	if rollbackVersion.Version != 6 {
		t.Errorf("Expected new version 6, got %d", rollbackVersion.Version)
	}

	if rollbackVersion.Status != StatusCurrent {
		t.Errorf("Expected status 'current', got %s", rollbackVersion.Status)
	}

	// 验证版本历史
	history := fvc.GetVersionHistory("/test/file.txt", 0)
	if len(history) != 6 {
		t.Errorf("Expected 6 versions total, got %d", len(history))
	}
}

func TestRollbackToNonExistentVersion(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	content := []byte("Test")
	fvc.CreateVersion("/test/file.txt", int64(len(content)), content, "Test", "user1")

	_, err := fvc.RollbackToVersion("/test/file.txt", 999)
	if err == nil {
		t.Error("Expected error for non-existent version, got nil")
	}
}

func TestCompareVersions(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	// 创建两个不同版本
	content1 := []byte("Version 1 content")
	fvc.CreateVersion("/test/file.txt", int64(len(content1)), content1, "V1", "user1")

	content2 := []byte("Version 2 content with more data")
	fvc.CreateVersion("/test/file.txt", int64(len(content2)), content2, "V2", "user1")

	// 比较版本
	diff, err := fvc.CompareVersions("/test/file.txt", 1, 2)
	if err != nil {
		t.Fatalf("Failed to compare versions: %v", err)
	}

	if diff.ChecksumMatch {
		t.Error("Expected checksums to be different")
	}

	if diff.SizeDiff == 0 {
		t.Error("Expected non-zero size difference")
	}

	if diff.Version1 != 1 || diff.Version2 != 2 {
		t.Errorf("Expected versions 1 and 2, got %d and %d", diff.Version1, diff.Version2)
	}
}

func TestCompareSameVersions(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	content := []byte("Same content")
	fvc.CreateVersion("/test/file.txt", int64(len(content)), content, "V1", "user1")

	diff, err := fvc.CompareVersions("/test/file.txt", 1, 1)
	if err != nil {
		t.Fatalf("Failed to compare versions: %v", err)
	}

	if !diff.ChecksumMatch {
		t.Error("Expected checksums to match for same version")
	}

	if diff.SizeDiff != 0 {
		t.Errorf("Expected zero size difference, got %d", diff.SizeDiff)
	}
}

func TestDeleteVersion(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	content := []byte("Test content")
	fvc.CreateVersion("/test/file.txt", int64(len(content)), content, "Test", "user1")

	// 删除版本
	err := fvc.DeleteVersion("/test/file.txt", 1)
	if err != nil {
		t.Fatalf("Failed to delete version: %v", err)
	}

	// 验证版本状态
	version, _ := fvc.GetVersion("/test/file.txt", 1)
	if version.Status != StatusDeleted {
		t.Errorf("Expected status 'deleted', got %s", version.Status)
	}
}

func TestRestoreDeletedVersion(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	content := []byte("Test content")
	fvc.CreateVersion("/test/file.txt", int64(len(content)), content, "Test", "user1")

	// 删除版本
	fvc.DeleteVersion("/test/file.txt", 1)

	// 恢复版本
	err := fvc.RestoreDeletedVersion("/test/file.txt", 1)
	if err != nil {
		t.Fatalf("Failed to restore version: %v", err)
	}

	// 验证版本状态
	version, _ := fvc.GetVersion("/test/file.txt", 1)
	if version.Status != StatusPrevious {
		t.Errorf("Expected status 'previous', got %s", version.Status)
	}
}

func TestPurgeOldVersions(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	// 创建多个版本
	for i := 1; i <= 10; i++ {
		content := []byte(fmt.Sprintf("Content %d", i))
		fvc.CreateVersion("/test/file.txt", int64(len(content)), content, fmt.Sprintf("V%d", i), "user1")
	}

	// 清理旧版本，保留3个
	count, err := fvc.PurgeOldVersions("/test/file.txt", 3)
	if err != nil {
		t.Fatalf("Failed to purge versions: %v", err)
	}

	if count != 7 {
		t.Errorf("Expected 7 purged versions, got %d", count)
	}

	// 验证剩余版本
	history := fvc.GetVersionHistory("/test/file.txt", 0)
	if len(history) != 3 {
		t.Errorf("Expected 3 remaining versions, got %d", len(history))
	}
}

func TestSetRetentionPolicy(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	policy := &RetentionPolicy{
		ID:          "test_policy",
		Name:        "Test Policy",
		Description: "Test retention policy",
		MaxVersions: 50,
		MaxAgeDays:  14,
		Enabled:     true,
	}

	err := fvc.SetRetentionPolicy(policy)
	if err != nil {
		t.Fatalf("Failed to set retention policy: %v", err)
	}

	if _, exists := fvc.retentionPolicies["test_policy"]; !exists {
		t.Error("Retention policy not found after setting")
	}
}

func TestSetRetentionPolicyWithEmptyID(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	policy := &RetentionPolicy{
		Name: "Test Policy",
	}

	err := fvc.SetRetentionPolicy(policy)
	if err == nil {
		t.Error("Expected error for empty ID, got nil")
	}
}

func TestGetStats(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	// 创建一些版本
	for i := 1; i <= 5; i++ {
		content := []byte(fmt.Sprintf("Content %d", i))
		fvc.CreateVersion(fmt.Sprintf("/test/file%d.txt", i), int64(len(content)), content, "Test", "user1")
	}

	stats := fvc.GetStats()

	if stats.TotalFiles != 5 {
		t.Errorf("Expected 5 files, got %d", stats.TotalFiles)
	}

	if stats.TotalVersions != 5 {
		t.Errorf("Expected 5 versions, got %d", stats.TotalVersions)
	}

	if stats.TotalSize == 0 {
		t.Error("Expected non-zero total size")
	}

	if stats.OldestVersion == nil {
		t.Error("Expected oldest version to be set")
	}

	if stats.NewestVersion == nil {
		t.Error("Expected newest version to be set")
	}
}

func TestMarshalJSON(t *testing.T) {
	fvc := NewFileVersionControl(nil)

	content := []byte("Test content")
	fvc.CreateVersion("/test/file.txt", int64(len(content)), content, "Test", "user1")

	data, err := fvc.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty JSON data")
	}
}

func TestGenerateDefaultRetentionPolicy(t *testing.T) {
	policy := GenerateDefaultRetentionPolicy()

	if policy.ID != "default" {
		t.Errorf("Expected policy ID 'default', got %s", policy.ID)
	}

	if !policy.Enabled {
		t.Error("Expected default policy to be enabled")
	}

	if policy.MaxVersions != 100 {
		t.Errorf("Expected MaxVersions 100, got %d", policy.MaxVersions)
	}

	if policy.MaxAgeDays != 30 {
		t.Errorf("Expected MaxAgeDays 30, got %d", policy.MaxAgeDays)
	}
}

func BenchmarkCreateVersion(b *testing.B) {
	fvc := NewFileVersionControl(nil)
	content := []byte("Benchmark content for testing")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fvc.CreateVersion(
			fmt.Sprintf("/bench/file_%d.txt", i%100),
			int64(len(content)),
			content,
			"Benchmark",
			"bench_user",
		)
	}
}

func BenchmarkGetVersionHistory(b *testing.B) {
	fvc := NewFileVersionControl(nil)

	// 预先创建版本
	for i := 0; i < 100; i++ {
		content := []byte(fmt.Sprintf("Content %d", i))
		fvc.CreateVersion("/bench/file.txt", int64(len(content)), content, "Setup", "user")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fvc.GetVersionHistory("/bench/file.txt", 10)
	}
}
