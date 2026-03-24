package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// 测试辅助函数：创建测试目录结构
func createTestDir(t *testing.T) string {
	tmpDir, err := os.MkdirTemp("", "space_analyzer_test_*")
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	// 创建测试文件结构
	//
	// tmpDir/
	// ├── videos/
	// │   ├── movie1.mp4 (1MB)
	// │   └── movie2.mkv (2MB)
	// ├── documents/
	// │   ├── report.pdf (100KB)
	// │   └── notes.txt (10KB)
	// ├── images/
	// │   ├── photo1.jpg (500KB)
	// │   └── photo2.png (300KB)
	// ├── large_file.iso (150MB)
	// └── .hidden/secret.txt (1KB)

	// 创建目录
	dirs := []string{"videos", "documents", "images", ".hidden"}
	for _, dir := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, dir), 0755); err != nil {
			t.Fatalf("创建目录失败: %v", err)
		}
	}

	// 创建测试文件
	testFiles := map[string]int64{
		"videos/movie1.mp4":    1 * 1024 * 1024,   // 1MB
		"videos/movie2.mkv":    2 * 1024 * 1024,   // 2MB
		"documents/report.pdf": 100 * 1024,        // 100KB
		"documents/notes.txt":  10 * 1024,         // 10KB
		"images/photo1.jpg":    500 * 1024,        // 500KB
		"images/photo2.png":    300 * 1024,        // 300KB
		"large_file.iso":       150 * 1024 * 1024, // 150MB
		".hidden/secret.txt":   1 * 1024,          // 1KB
	}

	for file, size := range testFiles {
		path := filepath.Join(tmpDir, file)
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i % 256)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	return tmpDir
}

// TestAnalyzeFileTypes 测试文件类型分析
func TestAnalyzeFileTypes(t *testing.T) {
	tmpDir := createTestDir(t)
	defer os.RemoveAll(tmpDir)

	// 创建模拟 Manager
	manager := &Manager{}
	sa := NewSpaceAnalyzer(manager, t.TempDir())

	opts := DefaultAnalyzeOptions
	opts.IncludeHidden = true

	dist, err := sa.analyzeFileTypes(tmpDir, opts)
	if err != nil {
		t.Fatalf("文件类型分析失败: %v", err)
	}

	// 验证分类统计
	tests := []struct {
		category string
		wantMin  uint64
	}{
		{"video", 3 * 1024 * 1024},  // video: mp4 + mkv >= 3MB
		{"image", 800 * 1024},       // image: jpg + png >= 800KB
		{"document", 110 * 1024},    // document: pdf + txt >= 110KB
		{"disk", 150 * 1024 * 1024}, // disk: iso >= 150MB
	}

	for _, tt := range tests {
		cat, ok := dist.Categories[tt.category]
		if !ok {
			t.Errorf("缺少分类: %s", tt.category)
			continue
		}
		if cat.Size < tt.wantMin {
			t.Errorf("分类 %s 大小不足: got %d, want >= %d", tt.category, cat.Size, tt.wantMin)
		}
	}

	// 验证扩展名统计
	if len(dist.ByExtension) == 0 {
		t.Error("扩展名统计为空")
	}
}

// TestFindLargeFiles 测试大文件检测
func TestFindLargeFiles(t *testing.T) {
	tmpDir := createTestDir(t)
	defer os.RemoveAll(tmpDir)

	manager := &Manager{}
	sa := NewSpaceAnalyzer(manager, t.TempDir())

	opts := DefaultAnalyzeOptions
	opts.LargeFileThreshold = 1 * 1024 * 1024 // 1MB

	largeFiles, err := sa.findLargeFiles(tmpDir, opts)
	if err != nil {
		t.Fatalf("大文件检测失败: %v", err)
	}

	// 应该找到至少3个大文件 (mp4, mkv, iso)
	if len(largeFiles) < 3 {
		t.Errorf("大文件检测数量不足: got %d, want >= 3", len(largeFiles))
	}

	// 验证最大文件是 iso
	if len(largeFiles) > 0 && largeFiles[0].Type != ".iso" {
		t.Errorf("最大文件类型错误: got %s, want .iso", largeFiles[0].Type)
	}

	// 验证文件大小排序
	for i := 1; i < len(largeFiles); i++ {
		if largeFiles[i].Size > largeFiles[i-1].Size {
			t.Errorf("大文件排序错误: %s (%d) > %s (%d)",
				largeFiles[i].Path, largeFiles[i].Size,
				largeFiles[i-1].Path, largeFiles[i-1].Size)
		}
	}
}

// TestRankDirectories 测试目录排行
func TestRankDirectories(t *testing.T) {
	tmpDir := createTestDir(t)
	defer os.RemoveAll(tmpDir)

	manager := &Manager{}
	sa := NewSpaceAnalyzer(manager, t.TempDir())

	opts := DefaultAnalyzeOptions
	opts.TopDirCount = 5

	ranking, err := sa.rankDirectories(tmpDir, opts)
	if err != nil {
		t.Fatalf("目录排行失败: %v", err)
	}

	// 应该有子目录排行
	if len(ranking) == 0 {
		t.Error("目录排行为空")
	}

	// 验证排序
	for i := 1; i < len(ranking); i++ {
		if ranking[i].Size > ranking[i-1].Size {
			t.Errorf("目录排序错误: %s (%d) > %s (%d)",
				ranking[i].Path, ranking[i].Size,
				ranking[i-1].Path, ranking[i-1].Size)
		}
	}
}

// TestPredictTrend 测试趋势预测
func TestPredictTrend(t *testing.T) {
	manager := &Manager{}
	sa := NewSpaceAnalyzer(manager, t.TempDir())

	// 添加模拟历史数据
	now := time.Now()
	testRecords := []struct {
		timeOffset int // 天数偏移
		used       uint64
	}{
		{-30, 100 * 1024 * 1024 * 1024}, // 30天前: 100GB
		{-20, 120 * 1024 * 1024 * 1024}, // 20天前: 120GB
		{-10, 140 * 1024 * 1024 * 1024}, // 10天前: 140GB
		{0, 160 * 1024 * 1024 * 1024},   // 现在: 160GB
	}

	for _, r := range testRecords {
		record := SpaceRecord{
			Timestamp: now.AddDate(0, 0, r.timeOffset),
			Volumes: []VolumeRecord{
				{
					Name:  "test",
					Used:  r.used,
					Total: 500 * 1024 * 1024 * 1024, // 500GB
				},
			},
		}
		sa.history.Records = append(sa.history.Records, record)
	}

	// 创建模拟卷
	vol := &Volume{
		Name: "test",
		Size: 500 * 1024 * 1024 * 1024, // 500GB
		Used: 160 * 1024 * 1024 * 1024, // 160GB
		Free: 340 * 1024 * 1024 * 1024, // 340GB
	}

	trend := sa.predictTrend("test", vol)

	// 验证趋势方向
	if trend.TrendDirection != "up" {
		t.Errorf("趋势方向错误: got %s, want up", trend.TrendDirection)
	}

	// 验证增长率计算 (30天增长60GB，平均每天2GB)
	if trend.GrowthRate30D < 1*1024*1024*1024 { // 应该 > 1GB/天
		t.Errorf("30天增长率过低: %d bytes/day", int(trend.GrowthRate30D))
	}

	// 验证预测值
	if trend.Predicted30D <= vol.Used {
		t.Errorf("30天预测值应该大于当前使用量: got %d, current %d",
			trend.Predicted30D, vol.Used)
	}
}

// TestFormatBytes 测试字节格式化
func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1536, "1.5 KB"},
		{1024 * 1024 * 1024 * 1024, "1.0 TB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %s, want %s", tt.bytes, result, tt.expected)
		}
	}
}

// TestGetFileTypeCategory 测试文件类型分类
func TestGetFileTypeCategory(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".mp4", "video"},
		{".mkv", "video"},
		{".MP4", "video"}, // 大小写
		{".mp3", "audio"},
		{".jpg", "image"},
		{".pdf", "document"},
		{".zip", "archive"},
		{".go", "code"},
		{".json", "data"},
		{".iso", "disk"},
		{".xyz", "other"},
		{"", "other"},
	}

	for _, tt := range tests {
		result := GetFileTypeCategory(tt.ext)
		if result != tt.expected {
			t.Errorf("GetFileTypeCategory(%s) = %s, want %s", tt.ext, result, tt.expected)
		}
	}
}

// TestExcludeHiddenFiles 测试排除隐藏文件
func TestExcludeHiddenFiles(t *testing.T) {
	tmpDir := createTestDir(t)
	defer os.RemoveAll(tmpDir)

	manager := &Manager{}
	sa := NewSpaceAnalyzer(manager, t.TempDir())

	// 不包含隐藏文件
	opts := DefaultAnalyzeOptions
	opts.IncludeHidden = false

	dist, err := sa.analyzeFileTypes(tmpDir, opts)
	if err != nil {
		t.Fatalf("文件类型分析失败: %v", err)
	}

	// 验证未统计隐藏文件
	// hidden 文件应该被排除
	for _, stat := range dist.ByExtension {
		if stat.Extension == ".txt" {
			// notes.txt 是 10KB，secret.txt 是 1KB
			// 如果包含隐藏文件，总共应该是 11KB
			// 如果排除隐藏文件，只有 10KB
			if stat.Size > 10*1024 {
				t.Error("应该排除隐藏文件，但统计到了")
			}
		}
	}
}

// TestSpaceHistory 测试历史记录
func TestSpaceHistory(t *testing.T) {
	tmpDir := t.TempDir()
	manager := &Manager{}
	sa := NewSpaceAnalyzer(manager, tmpDir)

	// 添加测试记录
	vol := &Volume{
		Name: "test",
		Size: 500 * 1024 * 1024 * 1024,
		Used: 160 * 1024 * 1024 * 1024,
		Free: 340 * 1024 * 1024 * 1024,
	}

	dist := FileTypeDistribution{
		ByExtension: []FileTypeStat{
			{Extension: ".mp4", Count: 100, Size: 1024 * 1024 * 1024, Percent: 50},
		},
	}

	sa.recordAnalysis("test", vol, dist)

	// 验证记录已添加
	if len(sa.history.Records) != 1 {
		t.Errorf("历史记录数量错误: got %d, want 1", len(sa.history.Records))
	}

	// 验证可以获取历史
	records, err := sa.GetHistory("test", 30)
	if err != nil {
		t.Fatalf("获取历史失败: %v", err)
	}

	if len(records) != 1 {
		t.Errorf("获取历史记录数量错误: got %d, want 1", len(records))
	}
}

// TestLargeFileSorting 测试大文件排序
func TestLargeFileSorting(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建多个大小不同的文件
	sizes := []struct {
		name string
		size int64
	}{
		{"small.bin", 1024},              // 1KB
		{"medium.bin", 10 * 1024 * 1024}, // 10MB
		{"large.bin", 100 * 1024 * 1024}, // 100MB
		{"huge.bin", 500 * 1024 * 1024},  // 500MB
	}

	for _, f := range sizes {
		path := filepath.Join(tmpDir, f.name)
		data := make([]byte, f.size)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("创建测试文件失败: %v", err)
		}
	}

	manager := &Manager{}
	sa := NewSpaceAnalyzer(manager, t.TempDir())

	opts := DefaultAnalyzeOptions
	opts.LargeFileThreshold = 5 * 1024 * 1024 // 5MB

	largeFiles, err := sa.findLargeFiles(tmpDir, opts)
	if err != nil {
		t.Fatalf("大文件检测失败: %v", err)
	}

	// 应该找到3个大文件 (medium, large, huge)
	if len(largeFiles) != 3 {
		t.Errorf("大文件数量错误: got %d, want 3", len(largeFiles))
	}

	// 验证排序：最大的在前
	expectedOrder := []string{"huge.bin", "large.bin", "medium.bin"}
	for i, expected := range expectedOrder {
		if i >= len(largeFiles) {
			break
		}
		if filepath.Base(largeFiles[i].Path) != expected {
			t.Errorf("大文件排序错误: 位置 %d, got %s, want %s",
				i, filepath.Base(largeFiles[i].Path), expected)
		}
	}
}

// TestEmptyDirectory 测试空目录
func TestEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &Manager{}
	sa := NewSpaceAnalyzer(manager, t.TempDir())

	opts := DefaultAnalyzeOptions

	// 文件类型分析
	dist, err := sa.analyzeFileTypes(tmpDir, opts)
	if err != nil {
		t.Fatalf("空目录分析失败: %v", err)
	}

	if len(dist.ByExtension) != 0 {
		t.Errorf("空目录不应该有文件类型: got %d", len(dist.ByExtension))
	}

	// 大文件检测
	largeFiles, err := sa.findLargeFiles(tmpDir, opts)
	if err != nil {
		t.Fatalf("空目录大文件检测失败: %v", err)
	}

	if len(largeFiles) != 0 {
		t.Errorf("空目录不应该有大文件: got %d", len(largeFiles))
	}

	// 目录排行
	ranking, err := sa.rankDirectories(tmpDir, opts)
	if err != nil {
		t.Fatalf("空目录排行失败: %v", err)
	}

	if len(ranking) != 0 {
		t.Errorf("空目录不应该有子目录排行: got %d", len(ranking))
	}
}
