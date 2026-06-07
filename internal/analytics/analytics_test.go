package analytics

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== 采集器测试 ==========

func TestDefaultCollectorConfig(t *testing.T) {
	config := DefaultCollectorConfig()

	assert.Equal(t, 30*time.Second, config.Interval)
	assert.Equal(t, 1000, config.HistorySize)
	assert.True(t, config.EnableCPU)
	assert.True(t, config.EnableMemory)
	assert.True(t, config.EnableDisk)
	assert.True(t, config.EnableNetwork)
}

func TestNewCollector(t *testing.T) {
	config := DefaultCollectorConfig()
	collector := NewCollector(config)

	assert.NotNil(t, collector)
	assert.NotNil(t, collector.history)
	assert.NotNil(t, collector.stopChan)
	assert.NotNil(t, collector.subscribers)
	assert.False(t, collector.running)
	assert.Equal(t, config, collector.config)
}

func TestCollector_StartStop(t *testing.T) {
	config := CollectorConfig{
		Interval:      100 * time.Millisecond,
		HistorySize:   10,
		EnableCPU:     false, // 禁用实际采集
		EnableMemory:  false,
		EnableDisk:    false,
		EnableNetwork: false,
	}

	collector := NewCollector(config)

	// 启动
	collector.Start()
	assert.True(t, collector.running)

	// 重复启动应安全
	collector.Start()
	assert.True(t, collector.running)

	// 停止
	collector.Stop()
	assert.False(t, collector.running)
}

func TestCollector_GetHistory(t *testing.T) {
	config := DefaultCollectorConfig()
	collector := NewCollector(config)

	// 空历史
	history := collector.GetHistory(10)
	assert.Empty(t, history)

	// 手动添加历史
	collector.history = append(collector.history, SystemMetrics{
		Timestamp: time.Now().Add(-2 * time.Minute),
	})
	collector.history = append(collector.history, SystemMetrics{
		Timestamp: time.Now().Add(-1 * time.Minute),
	})
	collector.history = append(collector.history, SystemMetrics{
		Timestamp: time.Now(),
	})

	// 获取所有历史
	history = collector.GetHistory(0)
	assert.Len(t, history, 3)

	// 获取限制数量
	history = collector.GetHistory(2)
	assert.Len(t, history, 2)
}

func TestCollector_GetLatest(t *testing.T) {
	config := DefaultCollectorConfig()
	collector := NewCollector(config)

	// 空历史返回nil
	latest := collector.GetLatest()
	assert.Nil(t, latest)

	// 添加历史
	now := time.Now()
	collector.history = append(collector.history, SystemMetrics{
		Timestamp: now,
		CPU: CPUMetrics{
			UsagePercent: 50.0,
		},
	})

	latest = collector.GetLatest()
	assert.NotNil(t, latest)
	assert.Equal(t, now, latest.Timestamp)
	assert.Equal(t, 50.0, latest.CPU.UsagePercent)
}

func TestCollector_GetHistoryByTimeRange(t *testing.T) {
	config := DefaultCollectorConfig()
	collector := NewCollector(config)

	now := time.Now()
	collector.history = []SystemMetrics{
		{Timestamp: now.Add(-3 * time.Hour)},
		{Timestamp: now.Add(-2 * time.Hour)},
		{Timestamp: now.Add(-1 * time.Hour)},
		{Timestamp: now},
	}

	// 获取最近2小时
	start := now.Add(-2 * time.Hour).Add(-1 * time.Minute)
	end := now.Add(1 * time.Minute)
	result := collector.GetHistoryByTimeRange(start, end)
	assert.Len(t, result, 3) // -2h, -1h, now
}

func TestCollector_SubscribeUnsubscribe(t *testing.T) {
	config := DefaultCollectorConfig()
	collector := NewCollector(config)

	// 订阅
	ch := collector.Subscribe()
	assert.NotNil(t, ch)
	assert.Len(t, collector.subscribers, 1)

	// 再次订阅
	ch2 := collector.Subscribe()
	assert.Len(t, collector.subscribers, 2)

	// 取消订阅
	collector.Unsubscribe(ch)
	assert.Len(t, collector.subscribers, 1)

	collector.Unsubscribe(ch2)
	assert.Len(t, collector.subscribers, 0)
}

func TestCollector_NotifySubscribers(t *testing.T) {
	config := DefaultCollectorConfig()
	collector := NewCollector(config)

	ch := collector.Subscribe()

	metrics := SystemMetrics{
		Timestamp: time.Now(),
		CPU: CPUMetrics{
			UsagePercent: 75.0,
		},
	}

	// 通知应发送到channel
	go collector.notifySubscribers(metrics)

	select {
	case received := <-ch:
		assert.Equal(t, 75.0, received.CPU.UsagePercent)
	case <-time.After(1 * time.Second):
		t.Fatal("超时等待通知")
	}
}

// ========== 统计计算测试 ==========

func TestCalculateCPUAverage(t *testing.T) {
	tests := []struct {
		name     string
		metrics  []SystemMetrics
		expected float64
	}{
		{"空数据", []SystemMetrics{}, 0},
		{"单个数据", []SystemMetrics{
			{CPU: CPUMetrics{UsagePercent: 50.0}},
		}, 50.0},
		{"多个数据", []SystemMetrics{
			{CPU: CPUMetrics{UsagePercent: 30.0}},
			{CPU: CPUMetrics{UsagePercent: 50.0}},
			{CPU: CPUMetrics{UsagePercent: 70.0}},
		}, 50.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateCPUAverage(tt.metrics)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestCalculateMemoryAverage(t *testing.T) {
	tests := []struct {
		name     string
		metrics  []SystemMetrics
		expected float64
	}{
		{"空数据", []SystemMetrics{}, 0},
		{"单个数据", []SystemMetrics{
			{Memory: MemoryMetrics{UsagePercent: 60.0}},
		}, 60.0},
		{"多个数据", []SystemMetrics{
			{Memory: MemoryMetrics{UsagePercent: 40.0}},
			{Memory: MemoryMetrics{UsagePercent: 60.0}},
			{Memory: MemoryMetrics{UsagePercent: 80.0}},
		}, 60.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateMemoryAverage(tt.metrics)
			assert.InDelta(t, tt.expected, result, 0.01)
		})
	}
}

func TestFindPeakCPUUsage(t *testing.T) {
	tests := []struct {
		name          string
		metrics       []SystemMetrics
		expectedPeak  float64
		expectedIndex int
	}{
		{"空数据", []SystemMetrics{}, 0, -1},
		{"单个数据", []SystemMetrics{
			{Timestamp: time.Now(), CPU: CPUMetrics{UsagePercent: 50.0}},
		}, 50.0, 0},
		{"多个数据", []SystemMetrics{
			{Timestamp: time.Now().Add(-2 * time.Hour), CPU: CPUMetrics{UsagePercent: 30.0}},
			{Timestamp: time.Now().Add(-1 * time.Hour), CPU: CPUMetrics{UsagePercent: 90.0}},
			{Timestamp: time.Now(), CPU: CPUMetrics{UsagePercent: 60.0}},
		}, 90.0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peakTime, peakUsage := FindPeakCPUUsage(tt.metrics)

			if len(tt.metrics) == 0 {
				assert.True(t, peakTime.IsZero())
				assert.Equal(t, 0.0, peakUsage)
			} else {
				assert.Equal(t, tt.metrics[tt.expectedIndex].Timestamp, peakTime)
				assert.Equal(t, tt.expectedPeak, peakUsage)
			}
		})
	}
}

func TestCalculateStandardDeviation(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		expected float64
	}{
		{"空数据", []float64{}, 0},
		{"单个数据", []float64{10}, 0},
		{"相同数据", []float64{5, 5, 5, 5}, 0},
		{"标准差", []float64{2, 4, 4, 4, 5, 5, 7, 9}, 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateStandardDeviation(tt.values)
			assert.InDelta(t, tt.expected, result, 0.1)
		})
	}
}

// ========== 用户行为分析器测试 ==========

func TestNewUserBehaviorAnalyzer(t *testing.T) {
	analyzer := NewUserBehaviorAnalyzer(1000)

	assert.NotNil(t, analyzer)
	assert.NotNil(t, analyzer.accessLogs)
	assert.NotNil(t, analyzer.hotFiles)
	assert.NotNil(t, analyzer.userActivity)
	assert.Equal(t, 1000, analyzer.maxLogs)
}

func TestNewUserBehaviorAnalyzerDefaultMaxLogs(t *testing.T) {
	analyzer := NewUserBehaviorAnalyzer(0)

	assert.Equal(t, 10000, analyzer.maxLogs)
}

func TestUserBehaviorAnalyzer_RecordAccess(t *testing.T) {
	analyzer := NewUserBehaviorAnalyzer(100)

	log := AccessLog{
		Timestamp:    time.Now(),
		UserID:       "user-1",
		Username:     "admin",
		FilePath:     "/data/file.txt",
		Action:       "read",
		BytesRead:    1024,
		BytesWritten: 0,
	}

	analyzer.RecordAccess(log)

	// 验证日志已记录
	assert.Len(t, analyzer.accessLogs, 1)

	// 验证热门文件已更新
	assert.Contains(t, analyzer.hotFiles, "/data/file.txt")
	assert.Equal(t, int64(1), analyzer.hotFiles["/data/file.txt"].AccessCount)
	assert.Contains(t, analyzer.hotFiles["/data/file.txt"].Users, "admin")

	// 验证用户活动已更新
	assert.Contains(t, analyzer.userActivity, "user-1")
	assert.Equal(t, int64(1), analyzer.userActivity["user-1"].AccessCount)
	assert.Equal(t, uint64(1024), analyzer.userActivity["user-1"].BytesRead)
}

func TestUserBehaviorAnalyzer_RecordAccessDuplicate(t *testing.T) {
	analyzer := NewUserBehaviorAnalyzer(100)

	log := AccessLog{
		Timestamp: time.Now(),
		UserID:    "user-1",
		Username:  "admin",
		FilePath:  "/data/file.txt",
		Action:    "read",
		BytesRead: 512,
	}

	analyzer.RecordAccess(log)
	analyzer.RecordAccess(log)

	// 验证访问次数增加
	assert.Equal(t, int64(2), analyzer.hotFiles["/data/file.txt"].AccessCount)
	assert.Equal(t, int64(2), analyzer.userActivity["user-1"].AccessCount)
	assert.Equal(t, uint64(1024), analyzer.userActivity["user-1"].BytesRead) // 512 * 2
}

func TestUserBehaviorAnalyzer_MaxLogs(t *testing.T) {
	analyzer := NewUserBehaviorAnalyzer(3)

	for i := 0; i < 5; i++ {
		analyzer.RecordAccess(AccessLog{
			Timestamp: time.Now(),
			UserID:    "user-1",
			Username:  "admin",
			FilePath:  "/data/file.txt",
			Action:    "read",
		})
	}

	// 应只保留最后3条日志
	assert.Len(t, analyzer.accessLogs, 3)
}

func TestUserBehaviorAnalyzer_Analyze(t *testing.T) {
	analyzer := NewUserBehaviorAnalyzer(100)

	// 添加一些访问日志
	now := time.Now()
	analyzer.RecordAccess(AccessLog{
		Timestamp: now,
		UserID:    "user-1",
		Username:  "admin",
		FilePath:  "/data/file1.txt",
		Action:    "read",
		BytesRead: 1024,
	})

	analyzer.RecordAccess(AccessLog{
		Timestamp:    now,
		UserID:       "user-2",
		Username:     "guest",
		FilePath:     "/data/file2.txt",
		Action:       "write",
		BytesWritten: 512,
	})

	result := analyzer.Analyze()

	assert.NotNil(t, result)
	assert.NotZero(t, result.Timestamp)
	assert.NotEmpty(t, result.HotFiles)
	assert.NotEmpty(t, result.UserActivity)
}

func TestUserBehaviorAnalyzer_GetUserAccessHistory(t *testing.T) {
	analyzer := NewUserBehaviorAnalyzer(100)

	analyzer.RecordAccess(AccessLog{
		Timestamp: time.Now(),
		UserID:    "user-1",
		Username:  "admin",
		FilePath:  "/data/file1.txt",
	})

	analyzer.RecordAccess(AccessLog{
		Timestamp: time.Now(),
		UserID:    "user-2",
		Username:  "guest",
		FilePath:  "/data/file2.txt",
	})

	analyzer.RecordAccess(AccessLog{
		Timestamp: time.Now(),
		UserID:    "user-1",
		Username:  "admin",
		FilePath:  "/data/file3.txt",
	})

	// 获取user-1的历史
	history := analyzer.GetUserAccessHistory("user-1", 0)
	assert.Len(t, history, 2)

	// 获取限制数量
	history = analyzer.GetUserAccessHistory("user-1", 1)
	assert.Len(t, history, 1)
}

func TestUserBehaviorAnalyzer_GetFileAccessHistory(t *testing.T) {
	analyzer := NewUserBehaviorAnalyzer(100)

	analyzer.RecordAccess(AccessLog{
		Timestamp: time.Now(),
		UserID:    "user-1",
		Username:  "admin",
		FilePath:  "/data/file1.txt",
	})

	analyzer.RecordAccess(AccessLog{
		Timestamp: time.Now(),
		UserID:    "user-2",
		Username:  "guest",
		FilePath:  "/data/file1.txt",
	})

	analyzer.RecordAccess(AccessLog{
		Timestamp: time.Now(),
		UserID:    "user-1",
		Username:  "admin",
		FilePath:  "/data/file2.txt",
	})

	// 获取file1.txt的历史
	history := analyzer.GetFileAccessHistory("/data/file1.txt", 0)
	assert.Len(t, history, 2)

	// 获取限制数量
	history = analyzer.GetFileAccessHistory("/data/file1.txt", 1)
	assert.Len(t, history, 1)
}

func TestUserBehaviorAnalyzer_GetAccessCountByTimeRange(t *testing.T) {
	analyzer := NewUserBehaviorAnalyzer(100)

	now := time.Now()
	analyzer.RecordAccess(AccessLog{
		Timestamp: now.Add(-2 * time.Hour),
		UserID:    "user-1",
		Username:  "admin",
	})

	analyzer.RecordAccess(AccessLog{
		Timestamp: now.Add(-30 * time.Minute),
		UserID:    "user-1",
		Username:  "admin",
	})

	analyzer.RecordAccess(AccessLog{
		Timestamp: now,
		UserID:    "user-1",
		Username:  "admin",
	})

	// 获取最近1小时的访问
	count := analyzer.GetAccessCountByTimeRange(now.Add(-1*time.Hour), now.Add(1*time.Minute))
	assert.Equal(t, int64(2), count) // -30min 和 now
}

func TestUserBehaviorAnalyzer_GetMostActiveHours(t *testing.T) {
	analyzer := NewUserBehaviorAnalyzer(100)

	// 添加不同时段的访问
	for i := 0; i < 5; i++ {
		analyzer.RecordAccess(AccessLog{
			Timestamp: time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC),
			UserID:    "user-1",
			Username:  "admin",
		})
	}

	for i := 0; i < 3; i++ {
		analyzer.RecordAccess(AccessLog{
			Timestamp: time.Date(2024, 1, 1, 14, 0, 0, 0, time.UTC),
			UserID:    "user-1",
			Username:  "admin",
		})
	}

	activeHours := analyzer.GetMostActiveHours(2)
	assert.Len(t, activeHours, 2)
	assert.Equal(t, 10, activeHours[0].Hour)
	assert.Equal(t, int64(5), activeHours[0].Count)
}

func TestUserBehaviorAnalyzer_ClearHistory(t *testing.T) {
	analyzer := NewUserBehaviorAnalyzer(100)

	analyzer.RecordAccess(AccessLog{
		Timestamp: time.Now(),
		UserID:    "user-1",
		Username:  "admin",
		FilePath:  "/data/file.txt",
	})

	assert.Len(t, analyzer.accessLogs, 1)
	assert.Len(t, analyzer.hotFiles, 1)
	assert.Len(t, analyzer.userActivity, 1)

	analyzer.ClearHistory()

	assert.Empty(t, analyzer.accessLogs)
	assert.Empty(t, analyzer.hotFiles)
	assert.Empty(t, analyzer.userActivity)
}

func TestUserBehaviorAnalyzer_GetStats(t *testing.T) {
	analyzer := NewUserBehaviorAnalyzer(100)

	analyzer.RecordAccess(AccessLog{
		Timestamp: time.Now(),
		UserID:    "user-1",
		Username:  "admin",
		FilePath:  "/data/file.txt",
	})

	stats := analyzer.GetStats()

	assert.Equal(t, 1, stats["totalLogs"])
	assert.Equal(t, 1, stats["uniqueFiles"])
	assert.Equal(t, 1, stats["uniqueUsers"])
	assert.Equal(t, 100, stats["maxLogCapacity"])
}

// ========== 格式化函数测试 ==========

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		name     string
		bytes    uint64
		expected string
	}{
		{"0字节", 0, "0 B"},
		{"字节", 500, "500 B"},
		{"KB", 1024, "1.00 KB"},
		{"MB", 1024 * 1024, "1.00 MB"},
		{"GB", 1024 * 1024 * 1024, "1.00 GB"},
		{"TB", 1024 * 1024 * 1024 * 1024, "1.00 TB"},
		{"混合", 1536, "1.50 KB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatHour(t *testing.T) {
	tests := []struct {
		hour     int
		expected string
	}{
		{0, "00:00-01:00"},
		{1, "01:00-02:00"},
		{12, "12:00-13:00"},
		{23, "23:00-24:00"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatHour(tt.hour)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== 文件分类测试 ==========

func TestCategorizeFile(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"photo.jpg", "图片"},
		{"image.png", "图片"},
		{"video.mp4", "视频"},
		{"movie.mkv", "视频"},
		{"song.mp3", "音频"},
		{"audio.flac", "音频"},
		{"document.pdf", "文档"},
		{"report.docx", "文档"},
		{"archive.zip", "压缩包"},
		{"backup.tar.gz", "压缩包"},
		{"main.go", "代码"},
		{"app.py", "代码"},
		{"program.exe", "可执行"},
		{"script.sh", "可执行"},
		{"data.db", "数据库"},
		{"config.json", "配置"},
		{"app.yaml", "配置"},
		{"server.log", "日志"},
		{"unknown.xyz", "其他"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := categorizeFile(tt.filename)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== 存储分析器测试 ==========

func TestNewStorageAnalyzer(t *testing.T) {
	analyzer := NewStorageAnalyzer("/tmp", 1000)

	assert.NotNil(t, analyzer)
	assert.Equal(t, "/tmp", analyzer.basePath)
	assert.Equal(t, 1000, analyzer.maxHistory)
	assert.NotNil(t, analyzer.history)
}

func TestNewStorageAnalyzerDefaultHistory(t *testing.T) {
	analyzer := NewStorageAnalyzer("/tmp", 0)

	assert.Equal(t, 1000, analyzer.maxHistory)
}

func TestStorageAnalyzer_GetLatest(t *testing.T) {
	analyzer := NewStorageAnalyzer("/tmp", 100)

	// 初始应为nil
	latest := analyzer.GetLatest()
	assert.Nil(t, latest)
}

func TestStorageAnalyzer_GetHistory(t *testing.T) {
	analyzer := NewStorageAnalyzer("/tmp", 100)

	// 初始应为空
	history := analyzer.GetHistory()
	assert.Empty(t, history)
}

func TestStorageAnalyzer_Analyze(t *testing.T) {
	// 使用临时目录进行测试
	tmpDir := t.TempDir()

	analyzer := NewStorageAnalyzer(tmpDir, 100)

	result, err := analyzer.Analyze()

	// 注意：statfs是模拟实现，可能返回固定值
	if err == nil {
		assert.NotNil(t, result)
		assert.NotZero(t, result.Timestamp)
		assert.NotZero(t, result.TotalCapacity)
	}

	// 验证GetLatest
	latest := analyzer.GetLatest()
	if result != nil {
		assert.NotNil(t, latest)
	}
}

// ========== 类型测试 ==========

func TestMetricTypeConstants(t *testing.T) {
	assert.Equal(t, MetricType("cpu"), MetricTypeCPU)
	assert.Equal(t, MetricType("memory"), MetricTypeMemory)
	assert.Equal(t, MetricType("disk"), MetricTypeDisk)
	assert.Equal(t, MetricType("network"), MetricTypeNetwork)
	assert.Equal(t, MetricType("iops"), MetricTypeIOPS)
	assert.Equal(t, MetricType("latency"), MetricTypeLatency)
}

func TestTimeRangeConstants(t *testing.T) {
	assert.Equal(t, TimeRange("1h"), TimeRangeHour)
	assert.Equal(t, TimeRange("24h"), TimeRangeDay)
	assert.Equal(t, TimeRange("7d"), TimeRangeWeek)
	assert.Equal(t, TimeRange("30d"), TimeRangeMonth)
	assert.Equal(t, TimeRange("1y"), TimeRangeYear)
}

func TestSystemMetricsStructure(t *testing.T) {
	now := time.Now()
	metrics := SystemMetrics{
		Timestamp: now,
		CPU: CPUMetrics{
			UsagePercent: 75.5,
			PerCore:      []float64{70.0, 80.0, 75.0, 77.0},
			LoadAvg1:     2.5,
			LoadAvg5:     2.0,
			LoadAvg15:    1.8,
			Temperature:  65.0,
			ProcessCount: 150,
		},
		Memory: MemoryMetrics{
			TotalBytes:     16 * 1024 * 1024 * 1024,
			UsedBytes:      12 * 1024 * 1024 * 1024,
			FreeBytes:      4 * 1024 * 1024 * 1024,
			AvailableBytes: 6 * 1024 * 1024 * 1024,
			UsagePercent:   75.0,
		},
	}

	assert.Equal(t, now, metrics.Timestamp)
	assert.Equal(t, 75.5, metrics.CPU.UsagePercent)
	assert.Len(t, metrics.CPU.PerCore, 4)
	assert.Equal(t, uint64(16*1024*1024*1024), metrics.Memory.TotalBytes)
}

func TestStorageAnalyticsStructure(t *testing.T) {
	analytics := StorageAnalytics{
		Timestamp:         time.Now(),
		TotalCapacity:     1024 * 1024 * 1024 * 1024, // 1TB
		UsedCapacity:      512 * 1024 * 1024 * 1024,  // 512GB
		AvailableCapacity: 512 * 1024 * 1024 * 1024,
		UsagePercent:      50.0,
		FileTypeDist: []FileTypeDistribution{
			{Category: "图片", FileCount: 100, TotalBytes: 1024 * 1024 * 100},
			{Category: "视频", FileCount: 50, TotalBytes: 1024 * 1024 * 1024 * 10},
		},
	}

	assert.Equal(t, uint64(1024*1024*1024*1024), analytics.TotalCapacity)
	assert.Equal(t, 50.0, analytics.UsagePercent)
	assert.Len(t, analytics.FileTypeDist, 2)
}

func TestUserBehaviorStructure(t *testing.T) {
	behavior := UserBehavior{
		Timestamp: time.Now(),
		HotFiles: []HotFile{
			{
				Path:         "/data/popular.txt",
				AccessCount:  100,
				LastAccessed: time.Now(),
				TotalBytes:   1024 * 1024,
				Users:        []string{"user1", "user2"},
			},
		},
		UserActivity: []UserActivity{
			{
				UserID:      "user-1",
				Username:    "admin",
				AccessCount: 50,
				BytesRead:   1024 * 1024 * 10,
			},
		},
	}

	assert.Len(t, behavior.HotFiles, 1)
	assert.Equal(t, int64(100), behavior.HotFiles[0].AccessCount)
	assert.Len(t, behavior.UserActivity, 1)
}

func TestPerformanceMetricsStructure(t *testing.T) {
	perf := PerformanceMetrics{
		Timestamp: time.Now(),
		IOPS: IOPSMetrics{
			ReadIOPS:  1000,
			WriteIOPS: 500,
			TotalIOPS: 1500,
		},
		Latency: LatencyMetrics{
			ReadLatencyAvg:  0.5,
			ReadLatencyP50:  0.3,
			ReadLatencyP99:  2.0,
			WriteLatencyAvg: 1.0,
			WriteLatencyP50: 0.8,
			WriteLatencyP99: 5.0,
		},
		Throughput: ThroughputMetrics{
			ReadBytesPS:  100 * 1024 * 1024,
			WriteBytesPS: 50 * 1024 * 1024,
			TotalBytesPS: 150 * 1024 * 1024,
		},
	}

	assert.Equal(t, 1500.0, perf.IOPS.TotalIOPS)
	assert.Equal(t, 0.5, perf.Latency.ReadLatencyAvg)
	assert.Equal(t, uint64(150*1024*1024), perf.Throughput.TotalBytesPS)
}

func TestAnalyticsSummaryStructure(t *testing.T) {
	summary := AnalyticsSummary{
		Timestamp: time.Now(),
		SystemHealth: HealthStatus{
			Status:    "healthy",
			Score:     95.0,
			CPUUsage:  45.0,
			MemUsage:  60.0,
			DiskUsage: 30.0,
		},
		StorageStatus: StorageStatus{
			Status:        "normal",
			TotalCapacity: 1024 * 1024 * 1024 * 1024,
			UsedCapacity:  512 * 1024 * 1024 * 1024,
			UsagePercent:  50.0,
		},
		Performance: PerformanceStatus{
			Status:       "optimal",
			TotalIOPS:    1500,
			AvgLatencyMs: 0.5,
			ThroughputMB: 150.0,
		},
		Alerts: []AnalyticsAlert{
			{
				ID:        "alert-1",
				Type:      "warning",
				Severity:  "medium",
				Message:   "CPU使用率偏高",
				Timestamp: time.Now(),
				Value:     85.0,
				Threshold: 80.0,
			},
		},
	}

	assert.Equal(t, "healthy", summary.SystemHealth.Status)
	assert.Equal(t, 95.0, summary.SystemHealth.Score)
	assert.Len(t, summary.Alerts, 1)
	assert.Equal(t, "warning", summary.Alerts[0].Type)
}
