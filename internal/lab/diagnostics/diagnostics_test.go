// Package diagnostics 单元测试
package diagnostics

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== Manager 测试 ==========

func TestNewManager(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	assert.NotNil(t, mgr)
	assert.Equal(t, cfg.MaxHistory, mgr.config.MaxHistory)
	assert.NotNil(t, mgr.history)
	assert.NotNil(t, mgr.stopCh)
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, 100, cfg.MaxHistory)
	assert.Equal(t, time.Hour, cfg.HistoryInterval)
	assert.Equal(t, 80.0, cfg.CPUThreshold)
	assert.Equal(t, 85.0, cfg.MemThreshold)
	assert.Equal(t, 90.0, cfg.DiskThreshold)
}

func TestManager_StartStop(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	// 测试启动
	mgr.Start()
	time.Sleep(100 * time.Millisecond) // 等待goroutine启动

	// 测试停止
	mgr.Stop()
}

func TestManager_GetHistory_Empty(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	history := mgr.GetHistory(10)
	assert.Empty(t, history)
}

func TestManager_GetLatestReport_Empty(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	report := mgr.GetLatestReport()
	assert.Nil(t, report)
}

func TestManager_RunDiagnostic_Mock(t *testing.T) {
	// 保存原始函数
	origReadLoadAvg := readLoadAvg
	origReadCPUTemp := readCPUTemp
	origReadMemInfo := readMemInfo
	origReadDiskPartitions := readDiskPartitions
	origReadDiskUsage := readDiskUsage
	origReadNetworkInterfaces := readNetworkInterfaces
	origCheckConnectivity := checkConnectivity
	origMeasureLatency := measureLatency

	// 恢复原始函数
	defer func() {
		readLoadAvg = origReadLoadAvg
		readCPUTemp = origReadCPUTemp
		readMemInfo = origReadMemInfo
		readDiskPartitions = origReadDiskPartitions
		readDiskUsage = origReadDiskUsage
		readNetworkInterfaces = origReadNetworkInterfaces
		checkConnectivity = origCheckConnectivity
		measureLatency = origMeasureLatency
	}()

	// Mock函数
	readLoadAvg = func() ([3]float64, error) {
		return [3]float64{0.5, 0.3, 0.2}, nil
	}
	readCPUTemp = func() (float64, error) {
		return 45.0, nil
	}
	readMemInfo = func() (map[string]uint64, error) {
		return map[string]uint64{
			"MemTotal":     8 * 1024 * 1024 * 1024, // 8GB
			"MemAvailable": 4 * 1024 * 1024 * 1024, // 4GB
			"SwapTotal":    2 * 1024 * 1024 * 1024, // 2GB
			"SwapFree":     2 * 1024 * 1024 * 1024, // 2GB
		}, nil
	}
	readDiskPartitions = func() ([]diskPartition, error) {
		return []diskPartition{
			{Device: "/dev/sda1", Mountpoint: "/", FSType: "ext4"},
		}, nil
	}
	readDiskUsage = func(mountpoint string) (diskUsage, error) {
		return diskUsage{
			Total:       100 * 1024 * 1024 * 1024, // 100GB
			Used:        50 * 1024 * 1024 * 1024,  // 50GB
			Free:        50 * 1024 * 1024 * 1024,  // 50GB
			UsedPercent: 50.0,
		}, nil
	}
	readNetworkInterfaces = func() ([]InterfaceInfo, error) {
		return []InterfaceInfo{
			{Name: "eth0", IP: "192.168.1.100", Status: "up", RxBytes: 1000, TxBytes: 500},
		}, nil
	}
	checkConnectivity = func() bool {
		return true
	}
	measureLatency = func() float64 {
		return 10.0
	}

	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	report, err := mgr.RunDiagnostic()
	assert.NoError(t, err)
	assert.NotNil(t, report)

	// 验证报告内容
	assert.NotEmpty(t, report.ID)
	assert.False(t, report.Timestamp.IsZero())
	assert.GreaterOrEqual(t, report.Score, 0)
	assert.LessOrEqual(t, report.Score, 100)
	assert.NotEmpty(t, report.Status)
	assert.NotNil(t, report.CPU)
	assert.NotNil(t, report.Memory)
	assert.NotNil(t, report.Disk)
	assert.NotNil(t, report.Network)
	assert.NotNil(t, report.Problems)
	assert.NotNil(t, report.Suggestions)
	assert.NotEmpty(t, report.Summary)

	// 验证CPU诊断
	assert.Equal(t, 0.5, report.CPU.LoadAvg1)
	assert.Equal(t, 45.0, report.CPU.Temperature)
	assert.Greater(t, report.CPU.Score, 0)

	// 验证内存诊断
	assert.Equal(t, uint64(8*1024*1024*1024), report.Memory.Total)
	assert.Equal(t, 50.0, report.Memory.UsedPercent)
	assert.Greater(t, report.Memory.Score, 0)

	// 验证磁盘诊断
	assert.Len(t, report.Disk.Partitions, 1)
	assert.Equal(t, 50.0, report.Disk.UsedPercent)
	assert.Greater(t, report.Disk.Score, 0)

	// 验证网络诊断
	assert.True(t, report.Network.Connectivity)
	assert.Len(t, report.Network.Interfaces, 1)
	assert.Greater(t, report.Network.Score, 0)

	// 验证历史记录
	assert.Equal(t, 1, len(mgr.GetHistory(10)))
}

func TestManager_GetTrend(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	// 手动添加历史记录
	now := time.Now()
	mgr.history = []DiagnosticReport{
		{
			Timestamp: now.Add(-2 * time.Hour),
			Score:     80,
			CPU:       &CPUDiag{Usage: 50},
			Memory:    &MemoryDiag{UsedPercent: 60},
			Disk:      &DiskDiag{UsedPercent: 70},
		},
		{
			Timestamp: now.Add(-1 * time.Hour),
			Score:     85,
			CPU:       &CPUDiag{Usage: 45},
			Memory:    &MemoryDiag{UsedPercent: 55},
			Disk:      &DiskDiag{UsedPercent: 71},
		},
		{
			Timestamp: now,
			Score:     90,
			CPU:       &CPUDiag{Usage: 40},
			Memory:    &MemoryDiag{UsedPercent: 50},
			Disk:      &DiskDiag{UsedPercent: 72},
		},
	}

	trend := mgr.GetTrend(3) // 最近3小时
	assert.Len(t, trend, 3)
	assert.Equal(t, 80, trend[0].Score)
	assert.Equal(t, 85, trend[1].Score)
	assert.Equal(t, 90, trend[2].Score)
}

func TestManager_GetTrend_EmptyHistory(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	trend := mgr.GetTrend(24)
	assert.Empty(t, trend)
}

// ========== 评分测试 ==========

func TestScoreToStatus(t *testing.T) {
	tests := []struct {
		score    int
		expected string
	}{
		{100, "excellent"},
		{90, "excellent"},
		{89, "good"},
		{75, "good"},
		{74, "fair"},
		{60, "fair"},
		{59, "poor"},
		{40, "poor"},
		{39, "critical"},
		{0, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			status := scoreToStatus(tt.score)
			assert.Equal(t, tt.expected, status)
		})
	}
}

func TestScoreCPU(t *testing.T) {
	tests := []struct {
		name     string
		diag     *CPUDiag
		minScore int
		maxScore int
	}{
		{
			name: "low usage",
			diag: &CPUDiag{
				Usage:       10,
				LoadAvg1:    0.1,
				Cores:       4,
				Temperature: 40,
			},
			minScore: 90,
			maxScore: 100,
		},
		{
			name: "high usage",
			diag: &CPUDiag{
				Usage:       90,
				LoadAvg1:    4.0,
				Cores:       4,
				Temperature: 85,
			},
			minScore: 0,
			maxScore: 50,
		},
		{
			name: "critical",
			diag: &CPUDiag{
				Usage:       98,
				LoadAvg1:    8.0,
				Cores:       4,
				Temperature: 95,
			},
			minScore: 0,
			maxScore: 20,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, status := scoreCPU(tt.diag)
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore)
			assert.NotEmpty(t, status)
		})
	}
}

func TestScoreMemory(t *testing.T) {
	tests := []struct {
		name     string
		diag     *MemoryDiag
		minScore int
		maxScore int
	}{
		{
			name: "low usage",
			diag: &MemoryDiag{
				UsedPercent: 30,
				SwapTotal:   1024,
				SwapUsed:    100,
			},
			minScore: 90,
			maxScore: 100,
		},
		{
			name: "high usage",
			diag: &MemoryDiag{
				UsedPercent: 90,
				SwapTotal:   1024,
				SwapUsed:    800,
			},
			minScore: 30,
			maxScore: 70,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, status := scoreMemory(tt.diag)
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore)
			assert.NotEmpty(t, status)
		})
	}
}

func TestScoreDisk(t *testing.T) {
	tests := []struct {
		name     string
		diag     *DiskDiag
		minScore int
		maxScore int
	}{
		{
			name: "low usage",
			diag: &DiskDiag{
				UsedPercent: 50,
				Partitions: []PartitionInfo{
					{UsedPercent: 50},
				},
			},
			minScore: 90,
			maxScore: 100,
		},
		{
			name: "high usage",
			diag: &DiskDiag{
				UsedPercent: 95,
				Partitions: []PartitionInfo{
					{UsedPercent: 98},
				},
			},
			minScore: 20,
			maxScore: 70,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, status := scoreDisk(tt.diag)
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore)
			assert.NotEmpty(t, status)
		})
	}
}

func TestScoreNetwork(t *testing.T) {
	tests := []struct {
		name     string
		diag     *NetworkDiag
		minScore int
		maxScore int
	}{
		{
			name: "good network",
			diag: &NetworkDiag{
				Connectivity: true,
				Latency:      10,
				Interfaces: []InterfaceInfo{
					{Status: "up", RxErrors: 0, TxErrors: 0},
				},
			},
			minScore: 90,
			maxScore: 100,
		},
		{
			name: "no connectivity",
			diag: &NetworkDiag{
				Connectivity: false,
				Latency:      0,
				Interfaces:   []InterfaceInfo{},
			},
			minScore: 0,
			maxScore: 50,
		},
		{
			name: "high latency",
			diag: &NetworkDiag{
				Connectivity: true,
				Latency:      300,
				Interfaces: []InterfaceInfo{
					{Status: "up", RxErrors: 10, TxErrors: 5},
				},
			},
			minScore: 30,
			maxScore: 85,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, status := scoreNetwork(tt.diag)
			assert.GreaterOrEqual(t, score, tt.minScore)
			assert.LessOrEqual(t, score, tt.maxScore)
			assert.NotEmpty(t, status)
		})
	}
}

// ========== 问题检测测试 ==========

func TestDetectProblems_HighCPU(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	report := &DiagnosticReport{
		CPU: &CPUDiag{
			Usage:       85,
			Temperature: 45,
		},
		Memory: &MemoryDiag{
			UsedPercent: 50,
		},
		Disk: &DiskDiag{
			Partitions: []PartitionInfo{},
		},
		Network: &NetworkDiag{
			Connectivity: true,
			Latency:      10,
		},
	}

	problems := mgr.detectProblems(report)
	assert.Len(t, problems, 1)
	assert.Equal(t, "high-cpu-usage", problems[0].ID)
	assert.Equal(t, "cpu", problems[0].Category)
	assert.Equal(t, "warning", problems[0].Severity)
}

func TestDetectProblems_HighMemory(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	report := &DiagnosticReport{
		CPU: &CPUDiag{
			Usage:       50,
			Temperature: 45,
		},
		Memory: &MemoryDiag{
			UsedPercent: 90,
		},
		Disk: &DiskDiag{
			Partitions: []PartitionInfo{},
		},
		Network: &NetworkDiag{
			Connectivity: true,
			Latency:      10,
		},
	}

	problems := mgr.detectProblems(report)
	assert.Len(t, problems, 1)
	assert.Equal(t, "high-memory-usage", problems[0].ID)
	assert.Equal(t, "memory", problems[0].Category)
	assert.Equal(t, "warning", problems[0].Severity)
}

func TestDetectProblems_NoProblems(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	report := &DiagnosticReport{
		CPU: &CPUDiag{
			Usage:       30,
			Temperature: 45,
		},
		Memory: &MemoryDiag{
			UsedPercent: 50,
		},
		Disk: &DiskDiag{
			Partitions: []PartitionInfo{
				{MountPoint: "/", UsedPercent: 60},
			},
		},
		Network: &NetworkDiag{
			Connectivity: true,
			Latency:      10,
		},
	}

	problems := mgr.detectProblems(report)
	assert.Empty(t, problems)
}

func TestDetectProblems_MultipleProblems(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	report := &DiagnosticReport{
		CPU: &CPUDiag{
			Usage:       90,
			Temperature: 85,
		},
		Memory: &MemoryDiag{
			UsedPercent: 92,
		},
		Disk: &DiskDiag{
			Partitions: []PartitionInfo{
				{MountPoint: "/data", UsedPercent: 95},
			},
		},
		Network: &NetworkDiag{
			Connectivity: false,
			Latency:      0,
		},
	}

	problems := mgr.detectProblems(report)
	assert.GreaterOrEqual(t, len(problems), 5) // CPU, CPU temp, memory, disk, network
}

// ========== 建议生成测试 ==========

func TestGenerateSuggestions_HighCPU(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	report := &DiagnosticReport{
		CPU: &CPUDiag{
			Usage:       85,
			Temperature: 45,
		},
		Memory: &MemoryDiag{
			UsedPercent: 50,
		},
		Disk: &DiskDiag{
			Partitions: []PartitionInfo{},
		},
		Network: &NetworkDiag{
			Connectivity: true,
		},
	}

	suggestions := mgr.generateSuggestions(report)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "reduce-cpu-usage", suggestions[0].ID)
	assert.Equal(t, "high", suggestions[0].Priority)
	assert.Equal(t, "cpu", suggestions[0].Category)
}

func TestGenerateSuggestions_Multiple(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)

	report := &DiagnosticReport{
		CPU: &CPUDiag{
			Usage:       85,
			Temperature: 75,
		},
		Memory: &MemoryDiag{
			UsedPercent: 80,
		},
		Disk: &DiskDiag{
			Partitions: []PartitionInfo{
				{MountPoint: "/data", UsedPercent: 90},
			},
		},
		Network: &NetworkDiag{
			Connectivity: false,
		},
	}

	suggestions := mgr.generateSuggestions(report)
	assert.GreaterOrEqual(t, len(suggestions), 4) // CPU, cooling, memory, network
}

// ========== 辅助函数测试 ==========

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatBytes(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/", "-"},
		{"/home", "-home"},
		{"/var/log", "-var-log"},
		{"test value", "test-value"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeID(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== HTTP Handlers 测试 ==========

func TestHandlers_RegisterRoutes(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)
	handlers := NewHandlers(mgr)

	router := gin.New()
	apiGroup := router.Group("/api")
	handlers.RegisterRoutes(apiGroup)

	// 验证路由注册成功（不检查具体路由数量，只验证不panic）
	assert.NotNil(t, router)
}

func TestHandlers_RunDiagnostic(t *testing.T) {
	// Mock函数
	origReadLoadAvg := readLoadAvg
	origReadCPUTemp := readCPUTemp
	origReadMemInfo := readMemInfo
	origReadDiskPartitions := readDiskPartitions
	origReadDiskUsage := readDiskUsage
	origReadNetworkInterfaces := readNetworkInterfaces
	origCheckConnectivity := checkConnectivity
	origMeasureLatency := measureLatency

	defer func() {
		readLoadAvg = origReadLoadAvg
		readCPUTemp = origReadCPUTemp
		readMemInfo = origReadMemInfo
		readDiskPartitions = origReadDiskPartitions
		readDiskUsage = origReadDiskUsage
		readNetworkInterfaces = origReadNetworkInterfaces
		checkConnectivity = origCheckConnectivity
		measureLatency = origMeasureLatency
	}()

	readLoadAvg = func() ([3]float64, error) { return [3]float64{0.5, 0.3, 0.2}, nil }
	readCPUTemp = func() (float64, error) { return 45.0, nil }
	readMemInfo = func() (map[string]uint64, error) {
		return map[string]uint64{"MemTotal": 8 * 1024 * 1024 * 1024, "MemAvailable": 4 * 1024 * 1024 * 1024, "SwapTotal": 0, "SwapFree": 0}, nil
	}
	readDiskPartitions = func() ([]diskPartition, error) { return nil, nil }
	readDiskUsage = func(mountpoint string) (diskUsage, error) { return diskUsage{}, nil }
	readNetworkInterfaces = func() ([]InterfaceInfo, error) { return nil, nil }
	checkConnectivity = func() bool { return true }
	measureLatency = func() float64 { return 10.0 }

	cfg := DefaultConfig()
	mgr := NewManager(cfg)
	handlers := NewHandlers(mgr)

	router := gin.New()
	apiGroup := router.Group("/api")
	handlers.RegisterRoutes(apiGroup)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/diagnostics/run", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetLatestReport_Empty(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)
	handlers := NewHandlers(mgr)

	router := gin.New()
	apiGroup := router.Group("/api")
	handlers.RegisterRoutes(apiGroup)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/diagnostics/latest", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestHandlers_GetHistory(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)
	handlers := NewHandlers(mgr)

	router := gin.New()
	apiGroup := router.Group("/api")
	handlers.RegisterRoutes(apiGroup)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/diagnostics/history?limit=5", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandlers_GetTrend(t *testing.T) {
	cfg := DefaultConfig()
	mgr := NewManager(cfg)
	handlers := NewHandlers(mgr)

	router := gin.New()
	apiGroup := router.Group("/api")
	handlers.RegisterRoutes(apiGroup)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/diagnostics/trend?hours=24", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHTTPStatusFromCode(t *testing.T) {
	tests := []struct {
		code     int
		expected int
	}{
		{http.StatusOK, http.StatusOK},
		{http.StatusBadRequest, http.StatusBadRequest},
		{http.StatusUnauthorized, http.StatusUnauthorized},
		{http.StatusForbidden, http.StatusForbidden},
		{http.StatusNotFound, http.StatusNotFound},
		{http.StatusInternalServerError, http.StatusInternalServerError},
		{999, http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			status := HTTPStatusFromCode(tt.code)
			assert.Equal(t, tt.expected, status)
		})
	}
}
