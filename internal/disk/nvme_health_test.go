package disk

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== NVMeHealthInfo 测试 ==========

func TestNVMeHealthInfo_Struct(t *testing.T) {
	now := time.Now()
	info := &NVMeHealthInfo{
		Device:           "/dev/nvme0",
		Model:            "Samsung 980 PRO",
		Serial:           "S123456789",
		Firmware:         "1.0",
		Size:             1024 * 1024, // 1TB
		OverallHealth:    "ok",
		SmartStatus:      "PASSED",
		HealthPercentage: 95,
		Status:           StatusHealthy,
		Temperature: &NVMeTempInfo{
			Current:  45,
			Warning:  70,
			Critical: 80,
		},
		Usage: &NVMeUsageInfo{
			PercentageUsed: 10,
			TBW:            50.5,
		},
		PowerOnHours: 1000,
		PowerCycles:  50,
		MediaErrors:  0,
		LastCheck:    now,
	}

	assert.Equal(t, "/dev/nvme0", info.Device)
	assert.Equal(t, "Samsung 980 PRO", info.Model)
	assert.Equal(t, uint8(95), info.HealthPercentage)
	assert.Equal(t, StatusHealthy, info.Status)
	assert.NotNil(t, info.Temperature)
	assert.NotNil(t, info.Usage)
}

// ========== NVMeTempInfo 测试 ==========

func TestNVMeTempInfo_Struct(t *testing.T) {
	temp := &NVMeTempInfo{
		Current:        45,
		Warning:        70,
		Critical:       80,
		MinTemp:        30,
		MaxTemp:        55,
		CompositeTemp:  44,
		OverTempEvents: 0,
	}

	assert.Equal(t, uint8(45), temp.Current)
	assert.Equal(t, uint8(70), temp.Warning)
	assert.Equal(t, uint8(80), temp.Critical)
	assert.Equal(t, uint8(30), temp.MinTemp)
	assert.Equal(t, uint8(55), temp.MaxTemp)
}

// ========== NVMeUsageInfo 测试 ==========

func TestNVMeUsageInfo_Struct(t *testing.T) {
	usage := &NVMeUsageInfo{
		PercentageUsed:   25,
		DataUnitsWritten: 100000,
		TBW:              50.0,
		TotalWrites:      51200.0,
		TotalReads:       25600.0,
		WearLevel:        "low",
		EstimatedLife:    ">50%",
	}

	assert.Equal(t, uint8(25), usage.PercentageUsed)
	assert.Equal(t, 50.0, usage.TBW)
	assert.Equal(t, "low", usage.WearLevel)
}

// ========== NVMeSpareInfo 测试 ==========

func TestNVMeSpareInfo_Struct(t *testing.T) {
	spare := &NVMeSpareInfo{
		Available:  100,
		Threshold:  10,
		Percentage: 100,
	}

	assert.Equal(t, uint8(100), spare.Available)
	assert.Equal(t, uint8(10), spare.Threshold)
	assert.Equal(t, uint8(100), spare.Percentage)
}

// ========== NVMeTestResult 测试 ==========

func TestNVMeTestResult_Struct(t *testing.T) {
	now := time.Now()
	endTime := now.Add(2 * time.Minute)
	result := &NVMeTestResult{
		TestType:  NVMeTestShort,
		Device:    "/dev/nvme0",
		Status:    "complete",
		Result:    "pass",
		Progress:  100,
		StartTime: now,
		EndTime:   &endTime,
		Duration:  2 * time.Minute,
		NumErrors: 0,
	}

	assert.Equal(t, NVMeTestShort, result.TestType)
	assert.Equal(t, "complete", result.Status)
	assert.Equal(t, "pass", result.Result)
	assert.Equal(t, uint8(100), result.Progress)
}

// ========== NVMeTestType 测试 ==========

func TestNVMeTestType_Constants(t *testing.T) {
	testTypes := []NVMeTestType{
		NVMeTestShort,
		NVMeTestLong,
		NVMeTestVendor,
		NVMeTestVerify,
		NVMeTestReadWrite,
	}

	for _, testType := range testTypes {
		assert.NotEmpty(t, string(testType))
	}
}

// ========== NVMeMonitor 测试 ==========

func TestNewNVMeMonitor(t *testing.T) {
	monitor := NewNVMeMonitor()
	require.NotNil(t, monitor)
	assert.NotNil(t, monitor.devices)
	assert.NotNil(t, monitor.testQueue)
}

func TestNVMeMonitor_ClearCache(t *testing.T) {
	monitor := NewNVMeMonitor()
	require.NotNil(t, monitor)

	// 添加一些测试数据
	monitor.devices["/dev/nvme0"] = &NVMeHealthInfo{
		Device: "/dev/nvme0",
	}

	// 清除特定设备缓存
	monitor.ClearCache("/dev/nvme0")
	assert.Nil(t, monitor.devices["/dev/nvme0"])

	// 再次添加并清除所有
	monitor.devices["/dev/nvme0"] = &NVMeHealthInfo{Device: "/dev/nvme0"}
	monitor.devices["/dev/nvme1"] = &NVMeHealthInfo{Device: "/dev/nvme1"}

	monitor.ClearCache("")
	assert.Equal(t, 0, len(monitor.devices))
}

// ========== 健康评分计算测试 ==========

func TestNVMeMonitor_calculateNVMeTempScore(t *testing.T) {
	monitor := NewNVMeMonitor()

	tests := []struct {
		name           string
		temp           uint8
		expectedScore  int
		expectedStatus string
	}{
		{"低温", 35, 100, "ok"},
		{"正常温度", 45, 100, "ok"},
		{"偏高", 55, 85, "ok"},
		{"警告", 65, 65, "warning"},
		{"过高", 70, 40, "warning"},
		{"严重过高", 80, 15, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &NVMeHealthInfo{
				Temperature: &NVMeTempInfo{Current: tt.temp},
			}
			score := monitor.calculateNVMeTempScore(info)
			assert.Equal(t, tt.expectedScore, score.Score)
			assert.Equal(t, tt.expectedStatus, score.Status)
		})
	}
}

func TestNVMeMonitor_calculateNVMeWearScore(t *testing.T) {
	monitor := NewNVMeMonitor()

	tests := []struct {
		name           string
		percentUsed    uint8
		expectedScore  int
		expectedStatus string
	}{
		{"几乎全新", 5, 100, "ok"},
		{"轻微使用", 25, 90, "ok"},
		{"中等使用", 45, 75, "ok"},
		{"较重使用", 65, 50, "warning"},
		{"严重磨损", 85, 25, "warning"},
		{"即将耗尽", 95, 5, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &NVMeHealthInfo{
				Usage: &NVMeUsageInfo{PercentageUsed: tt.percentUsed},
			}
			score := monitor.calculateNVMeWearScore(info)
			assert.Equal(t, tt.expectedScore, score.Score)
			assert.Equal(t, tt.expectedStatus, score.Status)
		})
	}
}

func TestNVMeMonitor_calculateNVMeErrorScore(t *testing.T) {
	monitor := NewNVMeMonitor()

	tests := []struct {
		name           string
		mediaErrors    uint64
		integrityErr   uint64
		errorLog       uint64
		unsafeShutdown uint64
		expectedStatus string
	}{
		{"无错误", 0, 0, 0, 5, "ok"},
		{"少量错误", 1, 0, 2, 5, "ok"},
		{"中等错误", 5, 3, 10, 20, "warning"},
		{"大量错误", 20, 15, 30, 50, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &NVMeHealthInfo{
				MediaErrors:     tt.mediaErrors,
				IntegrityErrors: tt.integrityErr,
				ErrorLogEntries: tt.errorLog,
				UnsafeShutdowns: tt.unsafeShutdown,
			}
			score := monitor.calculateNVMeErrorScore(info)
			assert.Equal(t, tt.expectedStatus, score.Status)
		})
	}
}

func TestNVMeMonitor_calculateNVMeSpareScore(t *testing.T) {
	monitor := NewNVMeMonitor()

	tests := []struct {
		name           string
		spare          uint8
		threshold      uint8
		expectedScore  int
		expectedStatus string
	}{
		{"充足", 100, 10, 100, "ok"},
		{"正常", 70, 10, 80, "ok"},
		{"偏低", 30, 10, 50, "warning"},
		{"严重不足", 5, 10, 20, "critical"},
		{"已耗尽", 0, 10, 0, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &NVMeHealthInfo{
				AvailableSpare: &NVMeSpareInfo{
					Percentage: tt.spare,
					Threshold:  tt.threshold,
				},
			}
			score := monitor.calculateNVMeSpareScore(info)
			assert.Equal(t, tt.expectedScore, score.Score)
			assert.Equal(t, tt.expectedStatus, score.Status)
		})
	}
}

func TestNVMeMonitor_getNVMeGradeAndStatus(t *testing.T) {
	monitor := NewNVMeMonitor()

	tests := []struct {
		score          int
		criticalWarn   uint8
		expectedGrade  string
		expectedStatus DiskStatus
	}{
		{95, 0, "A", StatusHealthy},
		{85, 0, "B", StatusHealthy},
		{75, 0, "C", StatusWarning},
		{60, 0, "D", StatusWarning},
		{30, 0, "F", StatusCritical},
		{95, 1, "F", StatusCritical}, // 有critical warning直接失败
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			info := &NVMeHealthInfo{
				CriticalWarnings: tt.criticalWarn,
			}
			grade, status := monitor.getNVMeGradeAndStatus(tt.score, info)
			assert.Equal(t, tt.expectedGrade, grade)
			assert.Equal(t, tt.expectedStatus, status)
		})
	}
}

// ========== JSON序列化测试 ==========

func TestNVMeHealthInfo_JSON(t *testing.T) {
	info := &NVMeHealthInfo{
		Device:           "/dev/nvme0",
		Model:            "Samsung 980 PRO",
		Serial:           "S123",
		Firmware:         "1.0",
		Size:             1024,
		OverallHealth:    "ok",
		SmartStatus:      "PASSED",
		HealthPercentage: 95,
		Status:           StatusHealthy,
		Temperature: &NVMeTempInfo{
			Current:  45,
			Warning:  70,
			Critical: 80,
		},
		Usage: &NVMeUsageInfo{
			PercentageUsed: 10,
			TBW:            50.0,
			WearLevel:      "low",
		},
		PowerOnHours: 1000,
		PowerCycles:  50,
		MediaErrors:  0,
		LastCheck:    time.Now(),
	}

	// 序列化
	data, err := json.Marshal(info)
	require.NoError(t, err)

	// 反序列化
	var decoded NVMeHealthInfo
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, info.Device, decoded.Device)
	assert.Equal(t, info.Model, decoded.Model)
	assert.Equal(t, info.HealthPercentage, decoded.HealthPercentage)
	assert.NotNil(t, decoded.Temperature)
	assert.Equal(t, info.Temperature.Current, decoded.Temperature.Current)
}

func TestNVMeTestResult_JSON(t *testing.T) {
	now := time.Now()
	result := &NVMeTestResult{
		TestType:  NVMeTestShort,
		Device:    "/dev/nvme0",
		Status:    "complete",
		Result:    "pass",
		Progress:  100,
		StartTime: now,
		Duration:  2 * time.Minute,
		NumErrors: 0,
	}

	data, err := json.Marshal(result)
	require.NoError(t, err)

	var decoded NVMeTestResult
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, result.TestType, decoded.TestType)
	assert.Equal(t, result.Device, decoded.Device)
	assert.Equal(t, result.Status, decoded.Status)
}

// ========== 建议生成测试 ==========

func TestNVMeMonitor_generateNVMeRecommendations(t *testing.T) {
	monitor := NewNVMeMonitor()

	tests := []struct {
		name           string
		info           *NVMeHealthInfo
		components     *ScoreComponents
		expectContains []string
	}{
		{
			name: "健康设备",
			info: &NVMeHealthInfo{
				Temperature: &NVMeTempInfo{Current: 40},
				Usage:       &NVMeUsageInfo{PercentageUsed: 10},
			},
			components: &ScoreComponents{
				Temperature: ComponentScore{Status: "ok"},
				Errors:      ComponentScore{Status: "ok"},
			},
			expectContains: []string{"良好"},
		},
		{
			name: "温度过高",
			info: &NVMeHealthInfo{
				Temperature: &NVMeTempInfo{Current: 75},
			},
			components: &ScoreComponents{
				Temperature: ComponentScore{Status: "critical"},
			},
			expectContains: []string{"温度"},
		},
		{
			name: "使用寿命低",
			info: &NVMeHealthInfo{
				Usage: &NVMeUsageInfo{PercentageUsed: 85},
			},
			components:     &ScoreComponents{},
			expectContains: []string{"使用寿命"},
		},
		{
			name: "备用空间不足",
			info: &NVMeHealthInfo{
				AvailableSpare: &NVMeSpareInfo{Percentage: 10},
			},
			components:     &ScoreComponents{},
			expectContains: []string{"备用"},
		},
		{
			name: "非安全关机",
			info: &NVMeHealthInfo{
				UnsafeShutdowns: 20,
			},
			components:     &ScoreComponents{},
			expectContains: []string{"非安全关机"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recommendations := monitor.generateNVMeRecommendations(tt.info, tt.components)
			assert.NotEmpty(t, recommendations)

			for _, expected := range tt.expectContains {
				found := false
				for _, rec := range recommendations {
					if contains(rec, expected) {
						found = true
						break
					}
				}
				assert.True(t, found, "应包含关键词: %s", expected)
			}
		})
	}
}

// ========== 辅助函数 ==========

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ========== 并发测试 ==========

func TestNVMeMonitor_Concurrent(t *testing.T) {
	monitor := NewNVMeMonitor()

	// 并发清除缓存
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(i int) {
			monitor.ClearCache("/dev/nvme0")
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========== 测试状态检查 ==========

func TestNVMeMonitor_TestStatus(t *testing.T) {
	monitor := NewNVMeMonitor()

	// 没有测试时的状态
	_, err := monitor.GetTestStatus("/dev/nonexistent")
	assert.Error(t, err)

	// 中止不存在的测试
	err = monitor.AbortTest("/dev/nonexistent")
	assert.Error(t, err)
}

// ========== 边界条件测试 ==========

func TestNVMeHealthInfo_NilFields(t *testing.T) {
	info := &NVMeHealthInfo{
		Device: "/dev/nvme0",
	}

	// 确保空指针不会导致问题
	assert.Nil(t, info.Temperature)
	assert.Nil(t, info.Usage)
	assert.Nil(t, info.AvailableSpare)
	assert.Nil(t, info.HealthScore)
}

func TestNVMeTempInfo_ZeroValues(t *testing.T) {
	temp := &NVMeTempInfo{}

	assert.Equal(t, uint8(0), temp.Current)
	assert.Equal(t, uint8(0), temp.Warning)
	assert.Equal(t, uint8(0), temp.Critical)
}

func TestNVMeUsageInfo_EdgeCases(t *testing.T) {
	usage := &NVMeUsageInfo{
		PercentageUsed: 100, // 已用完
		TBW:            1000,
	}

	assert.Equal(t, uint8(100), usage.PercentageUsed)
}

// ========== v2.387.0 三级预警测试 ==========

func TestNVMeMonitor_EvaluateAlertLevel(t *testing.T) {
	monitor := NewNVMeMonitor()

	tests := []struct {
		name          string
		info          *NVMeHealthInfo
		expectedLevel AlertLevel
		expectReasons int
	}{
		{
			name: "正常状态",
			info: &NVMeHealthInfo{
				HealthPercentage: 95,
				Temperature:      &NVMeTempInfo{Current: 45},
				CriticalWarnings: 0,
				MediaErrors:      0,
				AvailableSpare:   &NVMeSpareInfo{Percentage: 100, Threshold: 10},
			},
			expectedLevel: AlertLevelNormal,
			expectReasons: 1, // "所有指标正常"
		},
		{
			name: "温度警告",
			info: &NVMeHealthInfo{
				HealthPercentage: 95,
				Temperature:      &NVMeTempInfo{Current: 72},
				CriticalWarnings: 0,
			},
			expectedLevel: AlertLevelWarning,
			expectReasons: 1,
		},
		{
			name: "温度严重",
			info: &NVMeHealthInfo{
				HealthPercentage: 95,
				Temperature:      &NVMeTempInfo{Current: 86},
				CriticalWarnings: 0,
			},
			expectedLevel: AlertLevelCritical,
			expectReasons: 1,
		},
		{
			name: "健康度警告",
			info: &NVMeHealthInfo{
				HealthPercentage: 88,
				Temperature:      &NVMeTempInfo{Current: 45},
				CriticalWarnings: 0,
			},
			expectedLevel: AlertLevelWarning,
			expectReasons: 1,
		},
		{
			name: "健康度严重",
			info: &NVMeHealthInfo{
				HealthPercentage: 75,
				Temperature:      &NVMeTempInfo{Current: 45},
				CriticalWarnings: 0,
			},
			expectedLevel: AlertLevelCritical,
			expectReasons: 1,
		},
		{
			name: "健康度紧急",
			info: &NVMeHealthInfo{
				HealthPercentage: 65,
				Temperature:      &NVMeTempInfo{Current: 45},
				CriticalWarnings: 0,
			},
			expectedLevel: AlertLevelEmergency,
			expectReasons: 1,
		},
		{
			name: "备用空间低于阈值",
			info: &NVMeHealthInfo{
				HealthPercentage: 95,
				AvailableSpare:   &NVMeSpareInfo{Percentage: 5, Threshold: 10},
				CriticalWarnings: 0,
			},
			expectedLevel: AlertLevelEmergency,
			expectReasons: 1,
		},
		{
			name: "媒体错误过多",
			info: &NVMeHealthInfo{
				HealthPercentage: 95,
				MediaErrors:      150,
				CriticalWarnings: 0,
			},
			expectedLevel: AlertLevelCritical,
			expectReasons: 1,
		},
		{
			name: "关键警告标志",
			info: &NVMeHealthInfo{
				HealthPercentage: 95,
				CriticalWarnings: 2,
			},
			expectedLevel: AlertLevelCritical,
			expectReasons: 1,
		},
		{
			name: "多指标异常",
			info: &NVMeHealthInfo{
				HealthPercentage: 75,
				Temperature:      &NVMeTempInfo{Current: 86},
				MediaErrors:      20,
				CriticalWarnings: 0,
			},
			expectedLevel: AlertLevelCritical,
			expectReasons: 3, // 温度+健康度+媒体错误
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			level, reasons := monitor.EvaluateAlertLevel(tt.info)
			assert.Equal(t, tt.expectedLevel, level)
			assert.Len(t, reasons, tt.expectReasons)
		})
	}
}

func TestNVMeMonitor_PredictRemainingLife(t *testing.T) {
	monitor := NewNVMeMonitor()

	tests := []struct {
		name                 string
		info                 *NVMeHealthInfo
		expectPrediction     bool
		expectDaysLeftMin    int
		expectDaysLeftMax    int
		expectConfidence     string
	}{
		{
			name: "无使用数据",
			info: &NVMeHealthInfo{
				Device: "/dev/nvme0",
			},
			expectPrediction: false,
		},
		{
			name: "低使用率-长时间运行",
			info: &NVMeHealthInfo{
				Device:        "/dev/nvme0",
				Size:          1024 * 1024, // 1TB
				PowerOnHours:  24 * 60,      // 60天
				Temperature:   &NVMeTempInfo{Current: 40},
				Usage: &NVMeUsageInfo{
					PercentageUsed: 10,
					TBW:            60,
					TotalWrites:    60000,
				},
			},
			expectPrediction:  true,
			expectDaysLeftMin: 300,  // 预期至少300天 (算法考虑温度和磨损影响)
			expectConfidence:  "high",
		},
		{
			name: "中等使用率",
			info: &NVMeHealthInfo{
				Device:        "/dev/nvme0",
				Size:          1024 * 1024,
				PowerOnHours:  24 * 30, // 30天
				Temperature:   &NVMeTempInfo{Current: 50},
				Usage: &NVMeUsageInfo{
					PercentageUsed: 50,
					TBW:            300,
					TotalWrites:    300000,
				},
			},
			expectPrediction:  true,
			expectDaysLeftMin: 10, // 中等使用率预期值调整
			expectConfidence:  "medium",
		},
		{
			name: "高使用率-高温",
			info: &NVMeHealthInfo{
				Device:        "/dev/nvme0",
				Size:          1024 * 1024,
				PowerOnHours:  24 * 7, // 7天
				Temperature:   &NVMeTempInfo{Current: 70},
				Usage: &NVMeUsageInfo{
					PercentageUsed: 85,
					TBW:            510,
					TotalWrites:    510000,
				},
			},
			expectPrediction:  true,
			expectDaysLeftMin: 0,
			expectDaysLeftMax: 100,
			expectConfidence:  "low", // 7天数据不足，置信度低
		},
		{
			name: "短时间运行-低置信度",
			info: &NVMeHealthInfo{
				Device:        "/dev/nvme0",
				Size:          1024 * 1024,
				PowerOnHours:  24, // 1天
				Usage: &NVMeUsageInfo{
					PercentageUsed: 5,
					TBW:            30,
					TotalWrites:    30000,
				},
			},
			expectPrediction: true,
			expectConfidence: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prediction := monitor.PredictRemainingLife(tt.info)
			if !tt.expectPrediction {
				assert.Nil(t, prediction)
				return
			}

			require.NotNil(t, prediction)
			assert.GreaterOrEqual(t, prediction.EstimatedDaysLeft, tt.expectDaysLeftMin)
			if tt.expectDaysLeftMax > 0 {
				assert.LessOrEqual(t, prediction.EstimatedDaysLeft, tt.expectDaysLeftMax)
			}
			assert.Equal(t, tt.expectConfidence, prediction.ConfidenceLevel)
			assert.GreaterOrEqual(t, prediction.RemainingLifePercent, 0.0)
			assert.LessOrEqual(t, prediction.RemainingLifePercent, 100.0)
			assert.NotZero(t, prediction.TemperatureImpact)
			assert.NotZero(t, prediction.WearImpact)
		})
	}
}

func TestNVMeMonitor_RecordTemperature(t *testing.T) {
	monitor := NewNVMeMonitor()

	// 记录多次温度
	device := "/dev/nvme0"
	for i := 0; i < 5; i++ {
		monitor.RecordTemperature(device, uint8(40+i*2), 5.0)
	}

	// 验证历史记录
	monitor.historyMu.RLock()
	history := monitor.tempHistory[device]
	monitor.historyMu.RUnlock()

	assert.Len(t, history, 5)
	for i, record := range history {
		assert.Equal(t, uint8(40+i*2), record.Temp)
		assert.Equal(t, 5.0, record.Duration)
	}
}

func TestNVMeMonitor_GetLifePrediction(t *testing.T) {
	monitor := NewNVMeMonitor()

	// 无预测时返回nil
	prediction := monitor.GetLifePrediction("/dev/nonexistent")
	assert.Nil(t, prediction)

	// 添加预测后可获取
	info := &NVMeHealthInfo{
		Device:        "/dev/nvme0",
		PowerOnHours:  24 * 30,
		Usage: &NVMeUsageInfo{
			PercentageUsed: 20,
			TBW:            100,
			TotalWrites:    100000,
		},
	}
	monitor.PredictRemainingLife(info)

	prediction = monitor.GetLifePrediction("/dev/nvme0")
	assert.NotNil(t, prediction)
}

func TestNVMeMonitor_GetAlertSummary(t *testing.T) {
	monitor := NewNVMeMonitor()

	// 添加几个设备
	monitor.mu.Lock()
	monitor.devices["/dev/nvme0"] = &NVMeHealthInfo{
		Device:           "/dev/nvme0",
		HealthPercentage: 95,
		Temperature:      &NVMeTempInfo{Current: 45},
	}
	monitor.devices["/dev/nvme1"] = &NVMeHealthInfo{
		Device:           "/dev/nvme1",
		HealthPercentage: 75,
		Temperature:      &NVMeTempInfo{Current: 50},
	}
	monitor.mu.Unlock()

	summary := monitor.GetAlertSummary()
	assert.Len(t, summary, 2)

	// 验证nvme0正常
	assert.Equal(t, AlertLevelNormal, summary["/dev/nvme0"].Level)
	assert.Equal(t, uint8(95), summary["/dev/nvme0"].HealthPct)

	// 验证nvme1警告
	assert.Equal(t, AlertLevelCritical, summary["/dev/nvme1"].Level)
}

func TestMaxAlertLevel(t *testing.T) {
	tests := []struct {
		a        AlertLevel
		b        AlertLevel
		expected AlertLevel
	}{
		{AlertLevelNormal, AlertLevelWarning, AlertLevelWarning},
		{AlertLevelWarning, AlertLevelNormal, AlertLevelWarning},
		{AlertLevelWarning, AlertLevelCritical, AlertLevelCritical},
		{AlertLevelCritical, AlertLevelEmergency, AlertLevelEmergency},
		{AlertLevelNormal, AlertLevelNormal, AlertLevelNormal},
		{AlertLevelEmergency, AlertLevelWarning, AlertLevelEmergency},
	}

	for _, tt := range tests {
		result := maxAlertLevel(tt.a, tt.b)
		assert.Equal(t, tt.expected, result)
	}
}

func TestAlertThresholds_Default(t *testing.T) {
	thresholds := DefaultAlertThresholds

	assert.Equal(t, uint8(70), thresholds.TempWarning)
	assert.Equal(t, uint8(85), thresholds.TempCritical)
	assert.Equal(t, uint8(90), thresholds.HealthWarning)
	assert.Equal(t, uint8(80), thresholds.HealthCritical)
	assert.Equal(t, uint8(70), thresholds.HealthEmergency)
	assert.Equal(t, uint8(80), thresholds.TBWWarning)
	assert.Equal(t, uint8(90), thresholds.TBWCritical)
}

func TestNVMeMonitor_SetAlertThresholds(t *testing.T) {
	monitor := NewNVMeMonitor()

	customThresholds := &AlertThresholds{
		TempWarning:     65,
		TempCritical:    80,
		HealthWarning:   85,
		HealthCritical:  75,
		HealthEmergency: 60,
		TBWWarning:      70,
		TBWCritical:     85,
	}

	monitor.SetAlertThresholds(customThresholds)
	assert.Equal(t, customThresholds, monitor.GetAlertThresholds())

	// 使用自定义阈值测试
	info := &NVMeHealthInfo{
		HealthPercentage: 80, // 使用自定义阈值后应为Warning
	}
	level, _ := monitor.EvaluateAlertLevel(info)
	assert.Equal(t, AlertLevelWarning, level) // 80 < 85 (custom warning)
}

// ========== 性能测试 ==========

func BenchmarkNVMeHealthInfo_JSON(b *testing.B) {
	info := &NVMeHealthInfo{
		Device:           "/dev/nvme0",
		Model:            "Samsung 980 PRO",
		HealthPercentage: 95,
		Status:           StatusHealthy,
		Temperature:      &NVMeTempInfo{Current: 45},
		Usage:            &NVMeUsageInfo{PercentageUsed: 10},
		LastCheck:        time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(info)
	}
}

func BenchmarkNVMeMonitor_CalculateHealthScore(b *testing.B) {
	monitor := NewNVMeMonitor()
	info := &NVMeHealthInfo{
		Device:           "/dev/nvme0",
		HealthPercentage: 95,
		Temperature:      &NVMeTempInfo{Current: 45},
		Usage:            &NVMeUsageInfo{PercentageUsed: 10},
		AvailableSpare:   &NVMeSpareInfo{Percentage: 100, Threshold: 10},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.calculateNVMeHealthScore(info)
	}
}
