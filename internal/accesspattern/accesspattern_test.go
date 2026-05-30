package accesspattern

import (
	"testing"
	"time"
)

func TestNewAccessPatternManager(t *testing.T) {
	manager := NewAccessPatternManager(nil)
	if manager == nil {
		t.Fatal("NewAccessPatternManager 返回 nil")
	}

	if manager.records == nil {
		t.Fatal("records map 未初始化")
	}

	if manager.analyses == nil {
		t.Fatal("analyses map 未初始化")
	}
}

func TestRecordAccess(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	req := &RecordAccessRequest{
		FilePath:   "/data/test.txt",
		FileSize:   1024,
		AccessMode: "read",
		UserID:     "user1",
	}

	record, err := manager.RecordAccess(req)
	if err != nil {
		t.Fatalf("记录访问失败: %v", err)
	}

	if record.ID == "" {
		t.Error("记录ID为空")
	}

	if record.FilePath != "/data/test.txt" {
		t.Errorf("文件路径不匹配: %s", record.FilePath)
	}

	// 验证统计更新
	stats := manager.GetStats()
	if stats.TotalRecords != 1 {
		t.Errorf("总记录数不正确: %d", stats.TotalRecords)
	}
}

func TestRecordAccessEmptyPath(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	req := &RecordAccessRequest{
		FilePath: "",
	}

	_, err := manager.RecordAccess(req)
	if err == nil {
		t.Error("空路径应该返回错误")
	}
}

func TestAnalyzeFile(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	// 先记录一些访问
	filePath := "/data/important.doc"
	for i := 0; i < 10; i++ {
		manager.RecordAccess(&RecordAccessRequest{
			FilePath:   filePath,
			FileSize:   1024,
			AccessMode: "read",
			UserID:     "user1",
		})
	}

	// 分析文件
	analysis, err := manager.AnalyzeFile(filePath)
	if err != nil {
		t.Fatalf("分析失败: %v", err)
	}

	if analysis.TotalAccesses != 10 {
		t.Errorf("总访问次数不正确: %d", analysis.TotalAccesses)
	}

	if analysis.HeatScore < 0 || analysis.HeatScore > 100 {
		t.Errorf("热度评分超出范围: %.1f", analysis.HeatScore)
	}

	if analysis.Temperature == "" {
		t.Error("数据温度为空")
	}
}

func TestAnalyzeFileNotFound(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	_, err := manager.AnalyzeFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("应该返回文件不存在错误")
	}
}

func TestAnalyzeAll(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	// 记录多个文件的访问
	files := []string{"/data/file1.txt", "/data/file2.txt", "/data/file3.txt"}
	for _, file := range files {
		for i := 0; i < 5; i++ {
			manager.RecordAccess(&RecordAccessRequest{
				FilePath:   file,
				FileSize:   1024,
				AccessMode: "read",
			})
		}
	}

	results := manager.AnalyzeAll()

	if len(results) != 3 {
		t.Errorf("期望3个分析结果，实际 %d", len(results))
	}
}

func TestGetAnalysis(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	// 记录访问
	filePath := "/data/test.txt"
	manager.RecordAccess(&RecordAccessRequest{
		FilePath: filePath,
		FileSize: 1024,
	})

	// 先分析
	manager.AnalyzeFile(filePath)

	// 获取分析结果
	analysis, err := manager.GetAnalysis(filePath)
	if err != nil {
		t.Fatalf("获取分析结果失败: %v", err)
	}

	if analysis.FilePath != filePath {
		t.Errorf("文件路径不匹配: %s", analysis.FilePath)
	}
}

func TestGetAnalysisNotFound(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	_, err := manager.GetAnalysis("/nonexistent/file.txt")
	if err == nil {
		t.Error("应该返回分析结果不存在错误")
	}
}

func TestGenerateHeatMap(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	// 记录多个文件的访问
	files := []string{"/data/hot.txt", "/data/warm.txt", "/data/cold.txt"}
	accessCounts := []int{20, 5, 1}

	for i, file := range files {
		for j := 0; j < accessCounts[i]; j++ {
			manager.RecordAccess(&RecordAccessRequest{
				FilePath: file,
				FileSize: 1024,
			})
		}
	}

	// 分析所有文件
	manager.AnalyzeAll()

	// 生成热力图
	startTime := time.Now().AddDate(0, 0, -30)
	endTime := time.Now()
	heatMap := manager.GenerateHeatMap(startTime, endTime, 100)

	if heatMap == nil {
		t.Fatal("热力图为 nil")
	}

	if len(heatMap.Entries) != 3 {
		t.Errorf("期望3个热力图条目，实际 %d", len(heatMap.Entries))
	}

	// 检查排序（按热度降序）
	if len(heatMap.Entries) >= 2 {
		if heatMap.Entries[0].HeatScore < heatMap.Entries[1].HeatScore {
			t.Error("热力图应该按热度降序排列")
		}
	}
}

func TestGetStats(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	// 记录访问
	for i := 0; i < 5; i++ {
		manager.RecordAccess(&RecordAccessRequest{
			FilePath:   "/data/test.txt",
			FileSize:   1024,
			AccessMode: "read",
		})
	}

	stats := manager.GetStats()

	if stats.TotalRecords != 5 {
		t.Errorf("总记录数不正确: %d", stats.TotalRecords)
	}

	if stats.UniqueFiles != 1 {
		t.Errorf("唯一文件数不正确: %d", stats.UniqueFiles)
	}

	if stats.ByAccessMode["read"] != 5 {
		t.Errorf("读取次数不正确: %d", stats.ByAccessMode["read"])
	}
}

func TestGenerateTieringReport(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	// 记录不同温度的文件
	// 热文件
	for i := 0; i < 20; i++ {
		manager.RecordAccess(&RecordAccessRequest{
			FilePath: "/data/hot.txt",
			FileSize: 1024,
		})
	}

	// 冷文件（只访问一次）
	manager.RecordAccess(&RecordAccessRequest{
		FilePath: "/data/cold.txt",
		FileSize: 1024,
	})

	// 分析
	manager.AnalyzeAll()

	// 生成分层报告
	report := manager.GenerateTieringReport()

	if report == nil {
		t.Fatal("分层报告为 nil")
	}

	if report.Summary.TotalFiles != 2 {
		t.Errorf("总文件数不正确: %d", report.Summary.TotalFiles)
	}
}

func TestCleanup(t *testing.T) {
	config := DefaultAccessPatternConfig()
	config.RetentionDays = 0 // 立即过期
	manager := NewAccessPatternManager(&config)

	// 记录访问
	manager.RecordAccess(&RecordAccessRequest{
		FilePath: "/data/test.txt",
		FileSize: 1024,
	})

	// 清理（应该清除所有记录，因为保留天数为0）
	removed := manager.Cleanup()

	if removed != 1 {
		t.Errorf("应该删除1条记录，实际 %d", removed)
	}

	// 验证记录已清除
	stats := manager.GetStats()
	if stats.TotalRecords != 0 {
		t.Errorf("总记录数应该为0: %d", stats.TotalRecords)
	}
}

func TestTemperatureDetermination(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	// 测试热数据判断
	analysis := &PatternAnalysis{
		LastAccess:    time.Now(),
		TotalAccesses: 15,
	}

	temp := manager.determineTemperature(analysis)
	if temp != TemperatureHot {
		t.Errorf("应该是热数据，实际: %s", temp)
	}

	// 测试冷数据判断
	analysis = &PatternAnalysis{
		LastAccess:    time.Now().AddDate(0, 0, -100),
		TotalAccesses: 1,
	}

	temp = manager.determineTemperature(analysis)
	if temp != TemperatureCold {
		t.Errorf("应该是冷数据，实际: %s", temp)
	}
}

func TestHeatScoreCalculation(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	// 创建测试记录
	records := make([]*AccessRecord, 10)
	for i := 0; i < 10; i++ {
		records[i] = &AccessRecord{
			AccessTime: time.Now().Add(-time.Duration(10-i) * time.Hour),
		}
	}

	analysis := &PatternAnalysis{}
	score := manager.calculateHeatScore(records, analysis)

	if score < 0 || score > 100 {
		t.Errorf("热度评分超出范围: %.1f", score)
	}

	// 10次近期访问应该有较高的分数
	if score < 50 {
		t.Errorf("10次近期访问的热度评分应该较高: %.1f", score)
	}
}

func TestAccessPatternDetection(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	// 创建顺序访问模式（等间隔）
	records := make([]*AccessRecord, 10)
	for i := 0; i < 10; i++ {
		records[i] = &AccessRecord{
			AccessTime: time.Now().Add(-time.Duration(10-i) * time.Hour),
		}
	}

	pattern := manager.analyzeAccessPattern(records)
	if pattern != "sequential" {
		t.Errorf("应该是顺序访问模式，实际: %s", pattern)
	}
}

func TestReadWriteRatio(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	records := []*AccessRecord{
		{AccessMode: "read"},
		{AccessMode: "read"},
		{AccessMode: "read"},
		{AccessMode: "write"},
	}

	ratio := manager.calculateReadWriteRatio(records)
	if ratio != 3.0 {
		t.Errorf("读写比应该为3.0，实际: %.1f", ratio)
	}
}

func TestSuggestTier(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	tests := []struct {
		temp     DataTemperature
		expected string
	}{
		{TemperatureHot, "ssd"},
		{TemperatureWarm, "hdd"},
		{TemperatureCold, "archive"},
	}

	for _, test := range tests {
		analysis := &PatternAnalysis{
			Temperature: test.temp,
		}

		tier := manager.suggestTier(analysis)
		if tier != test.expected {
			t.Errorf("温度 %s: 期望层级 %s, 实际 %s", test.temp, test.expected, tier)
		}
	}
}

func TestMultipleAccessModes(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	// 记录不同访问模式
	modes := []string{"read", "read", "write", "read", "execute"}
	for _, mode := range modes {
		manager.RecordAccess(&RecordAccessRequest{
			FilePath:   "/data/test.txt",
			FileSize:   1024,
			AccessMode: mode,
		})
	}

	stats := manager.GetStats()

	if stats.ByAccessMode["read"] != 3 {
		t.Errorf("读取次数不正确: %d", stats.ByAccessMode["read"])
	}

	if stats.ByAccessMode["write"] != 1 {
		t.Errorf("写入次数不正确: %d", stats.ByAccessMode["write"])
	}

	if stats.ByAccessMode["execute"] != 1 {
		t.Errorf("执行次数不正确: %d", stats.ByAccessMode["execute"])
	}
}

func TestHeatMapSummary(t *testing.T) {
	manager := NewAccessPatternManager(nil)

	// 记录访问并分析
	files := []string{"/data/file1.txt", "/data/file2.txt"}
	for _, file := range files {
		for i := 0; i < 5; i++ {
			manager.RecordAccess(&RecordAccessRequest{
				FilePath: file,
				FileSize: 1024,
			})
		}
	}

	manager.AnalyzeAll()

	// 生成热力图
	heatMap := manager.GenerateHeatMap(
		time.Now().AddDate(0, 0, -30),
		time.Now(),
		100,
	)

	if heatMap.Summary.TotalFiles != 2 {
		t.Errorf("总文件数不正确: %d", heatMap.Summary.TotalFiles)
	}

	if heatMap.Summary.TotalSize != 2048 {
		t.Errorf("总大小不正确: %d", heatMap.Summary.TotalSize)
	}
}
