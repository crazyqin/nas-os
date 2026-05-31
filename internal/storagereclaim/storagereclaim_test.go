package storagereclaim

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewReclaimManager(t *testing.T) {
	rm := NewReclaimManager(nil)
	if rm == nil {
		t.Fatal("NewReclaimManager returned nil")
	}

	config := rm.GetConfig()
	if config == nil {
		t.Fatal("config is nil")
	}
	if config.ReclaimThreshold != 60.0 {
		t.Errorf("expected ReclaimThreshold 60.0, got %f", config.ReclaimThreshold)
	}
}

func TestDefaultReclaimConfig(t *testing.T) {
	config := DefaultReclaimConfig()
	if config == nil {
		t.Fatal("DefaultReclaimConfig returned nil")
	}

	if len(config.TempExtensions) == 0 {
		t.Error("expected TempExtensions to be non-empty")
	}

	if config.SizeWeight+config.AccessWeight+config.ImportanceWeight != 1.0 {
		t.Error("weights should sum to 1.0")
	}

	if config.RetentionDays != 30 {
		t.Errorf("expected RetentionDays 30, got %d", config.RetentionDays)
	}
}

func TestUpdateConfig(t *testing.T) {
	rm := NewReclaimManager(nil)

	// 无效配置
	err := rm.UpdateConfig(nil)
	if err != ErrInvalidConfig {
		t.Errorf("expected ErrInvalidConfig, got %v", err)
	}

	// 有效配置
	newConfig := DefaultReclaimConfig()
	newConfig.ReclaimThreshold = 70.0
	err = rm.UpdateConfig(newConfig)
	if err != nil {
		t.Fatal(err)
	}

	config := rm.GetConfig()
	if config.ReclaimThreshold != 70.0 {
		t.Errorf("expected ReclaimThreshold 70.0, got %f", config.ReclaimThreshold)
	}
}

func TestScanStatus(t *testing.T) {
	rm := NewReclaimManager(nil)

	status := rm.GetScanStatus()
	if status != ScanStatusIdle {
		t.Errorf("expected ScanStatusIdle, got %s", status)
	}
}

func TestScanWithTempFiles(t *testing.T) {
	// 创建临时测试目录
	tmpDir := t.TempDir()

	// 创建测试文件
	testFiles := []struct {
		name    string
		content string
	}{
		{"test.tmp", "temporary file"},
		{"cache.cache", "cached data"},
		{"normal.txt", "normal file"},
		{"backup.bak", "backup file"},
	}

	for _, f := range testFiles {
		path := filepath.Join(tmpDir, f.name)
		if err := os.WriteFile(path, []byte(f.content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	config := DefaultReclaimConfig()
	config.ScanPaths = []string{tmpDir}
	rm := NewReclaimManager(config)

	result, err := rm.Scan(nil)
	if err != nil {
		t.Fatal(err)
	}

	if result.TotalFiles != 4 {
		t.Errorf("expected 4 total files, got %d", result.TotalFiles)
	}

	// 应该检测到垃圾文件（.tmp, .cache, .bak）
	if result.JunkFiles == 0 {
		t.Errorf("expected junk files, got 0")
	}

	if rm.GetScanStatus() != ScanStatusCompleted {
		t.Errorf("expected ScanStatusCompleted, got %s", rm.GetScanStatus())
	}
}

func TestGetFiles(t *testing.T) {
	rm := NewReclaimManager(nil)

	// 手动添加一些文件
	rm.mu.Lock()
	rm.files["file1"] = &FileInfo{
		ID:       "file1",
		Path:     "/test/file1.tmp",
		IsJunk:   true,
		JunkType: JunkTypeTemp,
		ReclaimScore: 80,
	}
	rm.files["file2"] = &FileInfo{
		ID:       "file2",
		Path:     "/test/file2.txt",
		IsJunk:   false,
		ReclaimScore: 30,
	}
	rm.files["file3"] = &FileInfo{
		ID:       "file3",
		Path:     "/test/file3.cache",
		IsJunk:   true,
		JunkType: JunkTypeCache,
		ReclaimScore: 90,
	}
	rm.mu.Unlock()

	// 获取所有文件
	allFiles := rm.GetFiles(false, 0)
	if len(allFiles) != 3 {
		t.Errorf("expected 3 files, got %d", len(allFiles))
	}

	// 只获取垃圾文件
	junkFiles := rm.GetFiles(true, 0)
	if len(junkFiles) != 2 {
		t.Errorf("expected 2 junk files, got %d", len(junkFiles))
	}

	// 获取高分文件
	highScoreFiles := rm.GetFiles(false, 85)
	if len(highScoreFiles) != 1 {
		t.Errorf("expected 1 high score file, got %d", len(highScoreFiles))
	}
}

func TestGetFile(t *testing.T) {
	rm := NewReclaimManager(nil)

	rm.mu.Lock()
	rm.files["file1"] = &FileInfo{
		ID:   "file1",
		Path: "/test/file1.txt",
		Size: 1024,
	}
	rm.mu.Unlock()

	file, ok := rm.GetFile("file1")
	if !ok {
		t.Fatal("GetFile returned false")
	}
	if file.Path != "/test/file1.txt" {
		t.Errorf("expected path '/test/file1.txt', got '%s'", file.Path)
	}

	_, ok = rm.GetFile("nonexistent")
	if ok {
		t.Error("expected GetFile to return false for nonexistent file")
	}
}

func TestReclaimSpace(t *testing.T) {
	// 创建临时测试目录
	tmpDir := t.TempDir()
	recycleDir := filepath.Join(tmpDir, "recycle")

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "test.tmp")
	if err := os.WriteFile(testFile, []byte("test content for reclaim"), 0644); err != nil {
		t.Fatal(err)
	}

	config := DefaultReclaimConfig()
	config.RecycleBinPath = recycleDir
	rm := NewReclaimManager(config)

	// 先扫描
	rm.Scan([]string{tmpDir})

	// 执行回收（dry run）
	task, err := rm.ReclaimSpace(0, nil, 0, true)
	if err != nil {
		t.Fatal(err)
	}

	if !task.DryRun {
		t.Error("expected dry run task")
	}

	// 实际回收
	task, err = rm.ReclaimSpace(0, nil, 0, false)
	if err != nil {
		t.Fatal(err)
	}

	if task.DryRun {
		t.Error("expected non-dry run task")
	}

	if task.Status != "completed" {
		t.Errorf("expected status 'completed', got '%s'", task.Status)
	}
}

func TestRecycleBin(t *testing.T) {
	rm := NewReclaimManager(nil)

	// 手动添加回收站项目
	rm.mu.Lock()
	rm.recycleBin["file1"] = &RecycleBinItem{
		FileID:       "file1",
		OriginalPath: "/test/file1.txt",
		Name:         "file1.txt",
		Size:         1024,
		DeletedAt:    time.Now(),
		Status:       RecycleStatusActive,
	}
	rm.recycleBin["file2"] = &RecycleBinItem{
		FileID:       "file2",
		OriginalPath: "/test/file2.txt",
		Name:         "file2.txt",
		Size:         2048,
		DeletedAt:    time.Now().Add(-1 * time.Hour),
		Status:       RecycleStatusActive,
	}
	rm.mu.Unlock()

	// 获取回收站内容
	items := rm.GetRecycleBin(10, 0)
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	// 获取统计
	stats := rm.GetRecycleBinStats()
	if stats.ItemCount != 2 {
		t.Errorf("expected 2 items, got %d", stats.ItemCount)
	}
	if stats.TotalSize != 3072 {
		t.Errorf("expected total size 3072, got %d", stats.TotalSize)
	}

	// 分页测试
	items = rm.GetRecycleBin(1, 0)
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestStorageOverview(t *testing.T) {
	rm := NewReclaimManager(nil)

	rm.mu.Lock()
	rm.files["file1"] = &FileInfo{
		ID:       "file1",
		Path:     "/data/file1.txt",
		Size:     1024,
		Owner:    "user1",
		Extension: ".txt",
		IsJunk:   false,
	}
	rm.files["file2"] = &FileInfo{
		ID:       "file2",
		Path:     "/data/file2.tmp",
		Size:     2048,
		Owner:    "user1",
		Extension: ".tmp",
		IsJunk:   true,
		JunkType: JunkTypeTemp,
		ReclaimScore: 80,
	}
	rm.files["file3"] = &FileInfo{
		ID:       "file3",
		Path:     "/data/file3.txt",
		Size:     512,
		Owner:    "user2",
		Extension: ".txt",
		IsJunk:   false,
	}
	rm.mu.Unlock()

	overview := rm.GetStorageOverview()

	if overview.FileCount != 3 {
		t.Errorf("expected 3 files, got %d", overview.FileCount)
	}

	if overview.JunkCount != 1 {
		t.Errorf("expected 1 junk file, got %d", overview.JunkCount)
	}

	if len(overview.UserStats) != 2 {
		t.Errorf("expected 2 users, got %d", len(overview.UserStats))
	}
}

func TestDuplicateDetection(t *testing.T) {
	// 创建临时测试目录
	tmpDir := t.TempDir()

	// 创建重复文件
	content := "this is duplicate content"
	for i := 0; i < 3; i++ {
		path := filepath.Join(tmpDir, "dup"+string(rune('A'+i))+".txt")
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// 创建一个不重复的文件
	uniquePath := filepath.Join(tmpDir, "unique.txt")
	if err := os.WriteFile(uniquePath, []byte("unique content"), 0644); err != nil {
		t.Fatal(err)
	}

	config := DefaultReclaimConfig()
	config.ScanPaths = []string{tmpDir}
	rm := NewReclaimManager(config)

	result, err := rm.Scan(nil)
	if err != nil {
		t.Fatal(err)
	}

	// 应该检测到1组重复文件
	if result.Duplicates != 1 {
		t.Errorf("expected 1 duplicate group, got %d", result.Duplicates)
	}

	// 获取重复文件组
	groups := rm.GetDuplicates()
	if len(groups) != 1 {
		t.Fatalf("expected 1 duplicate group, got %d", len(groups))
	}

	group := groups[0]
	if group.FileCount != 3 {
		t.Errorf("expected 3 files in group, got %d", group.FileCount)
	}

	if group.WastedSize != int64(len(content))*2 {
		t.Errorf("expected wasted size %d, got %d", int64(len(content))*2, group.WastedSize)
	}
}

func TestReclaimHistory(t *testing.T) {
	rm := NewReclaimManager(nil)

	// 手动添加回收历史
	rm.mu.Lock()
	rm.reclaimHistory = []*ReclaimTask{
		{ID: "task1", Status: "completed", Reclaimed: 1024},
		{ID: "task2", Status: "completed", Reclaimed: 2048},
		{ID: "task3", Status: "completed", Reclaimed: 512},
	}
	rm.mu.Unlock()

	history := rm.GetReclaimHistory(2)
	if len(history) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(history))
	}

	// 应该返回最新的2个
	if history[0].ID != "task2" {
		t.Errorf("expected task2, got %s", history[0].ID)
	}
	if history[1].ID != "task3" {
		t.Errorf("expected task3, got %s", history[1].ID)
	}
}

func TestCalculateReclaimScore(t *testing.T) {
	rm := NewReclaimManager(nil)

	// 低重要性、很久没访问的大文件
	file := &FileInfo{
		Size:       1024 * 1024 * 1024, // 1GB
		AccessedAt: time.Now().Add(-90 * 24 * time.Hour), // 90天前
		Importance: ImportanceLow,
		IsJunk:     true,
	}

	score := rm.calculateReclaimScore(file)
	if score < 60 {
		t.Errorf("expected score >= 60 for junk file, got %f", score)
	}

	// 高重要性、刚访问的小文件
	file2 := &FileInfo{
		Size:       1024, // 1KB
		AccessedAt: time.Now(),
		Importance: ImportanceCritical,
		IsJunk:     false,
	}

	score2 := rm.calculateReclaimScore(file2)
	if score2 > 40 {
		t.Errorf("expected score < 40 for critical file, got %f", score2)
	}
}
