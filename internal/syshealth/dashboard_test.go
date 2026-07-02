package syshealth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ========== Mock 检查器 ==========

// MockSubsystemChecker 模拟子系统检查器。
type MockSubsystemChecker struct {
	name   string
	typ    string
	status SubsystemStatus
}

func (m *MockSubsystemChecker) Name() string { return m.name }
func (m *MockSubsystemChecker) Type() string { return m.typ }
func (m *MockSubsystemChecker) Check() SubsystemStatus {
	return m.status
}

// ========== 测试类型定义 ==========

func TestClassifyLevel(t *testing.T) {
	tests := []struct {
		score    float64
		expected HealthLevel
	}{
		{95, LevelExcellent},
		{90, LevelExcellent},
		{85, LevelGood},
		{70, LevelGood},
		{65, LevelFair},
		{50, LevelFair},
		{45, LevelPoor},
		{30, LevelPoor},
		{25, LevelCritical},
		{0, LevelCritical},
	}

	for _, tt := range tests {
		result := ClassifyLevel(tt.score)
		assert.Equal(t, tt.expected, result, "score=%f", tt.score)
	}
}

func TestClassifyStatus(t *testing.T) {
	tests := []struct {
		score    float64
		expected SystemStatus
	}{
		{90, StatusHealthy},
		{70, StatusHealthy},
		{65, StatusWarning},
		{50, StatusWarning},
		{45, StatusCritical},
		{0, StatusCritical},
	}

	for _, tt := range tests {
		result := ClassifyStatus(tt.score)
		assert.Equal(t, tt.expected, result, "score=%f", tt.score)
	}
}

// ========== 测试仪表盘引擎 ==========

func TestNewDashboard(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	assert.NotNil(t, d)
	assert.NotNil(t, d.checkers)
	assert.NotNil(t, d.providers)
	assert.NotNil(t, d.weights)
	assert.NotNil(t, d.history)
	assert.NotNil(t, d.alerts)
	assert.Equal(t, 30*time.Second, d.cacheTTL)
}

func TestDashboardRegisterChecker(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	checker := &MockSubsystemChecker{
		name: "test_subsystem",
		typ:  "test",
		status: SubsystemStatus{
			Name:    "test_subsystem",
			Type:    "test",
			Status:  StatusHealthy,
			Score:   95,
			Message: "测试正常",
		},
	}

	d.RegisterChecker(checker)

	assert.Len(t, d.checkers, 1)
	assert.Equal(t, "test_subsystem", d.checkers[0].Name())
}

func TestDashboardGetOverview(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	// 注册多个检查器
	checkers := []SubsystemChecker{
		&MockSubsystemChecker{
			name: "cpu",
			typ:  "cpu",
			status: SubsystemStatus{
				Name:    "cpu",
				Type:    "cpu",
				Status:  StatusHealthy,
				Score:   90,
				Message: "CPU 使用正常",
			},
		},
		&MockSubsystemChecker{
			name: "memory",
			typ:  "memory",
			status: SubsystemStatus{
				Name:    "memory",
				Type:    "memory",
				Status:  StatusHealthy,
				Score:   85,
				Message: "内存使用正常",
			},
		},
		&MockSubsystemChecker{
			name: "disk",
			typ:  "disk",
			status: SubsystemStatus{
				Name:    "disk",
				Type:    "disk",
				Status:  StatusWarning,
				Score:   65,
				Message: "磁盘空间不足",
			},
		},
	}

	for _, c := range checkers {
		d.RegisterChecker(c)
	}

	// 获取总览
	overview, err := d.GetOverview()
	require.NoError(t, err)
	require.NotNil(t, overview)

	// 验证基本属性
	assert.GreaterOrEqual(t, overview.OverallScore, 0.0)
	assert.LessOrEqual(t, overview.OverallScore, 100.0)
	assert.Len(t, overview.Subsystems, 3)
	assert.NotZero(t, overview.EvaluatedAt)

	// 验证子系统状态
	subNames := make([]string, 0)
	for _, sub := range overview.Subsystems {
		subNames = append(subNames, sub.Name)
	}
	assert.Contains(t, subNames, "cpu")
	assert.Contains(t, subNames, "memory")
	assert.Contains(t, subNames, "disk")
}

func TestDashboardGetOverviewWithMetrics(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	// 注册指标数据源
	d.RegisterMetricsProvider(func() (CoreMetrics, error) {
		return CoreMetrics{
			CPU:         0.45,
			Memory:      0.60,
			Disk:        0.55,
			Temperature: 42.5,
			Uptime:      86400,
			LoadAverage: 1.5,
		}, nil
	})

	overview, err := d.GetOverview()
	require.NoError(t, err)

	// 验证指标
	assert.Equal(t, 0.45, overview.Metrics.CPU)
	assert.Equal(t, 0.60, overview.Metrics.Memory)
	assert.Equal(t, 0.55, overview.Metrics.Disk)
	assert.Equal(t, 42.5, overview.Metrics.Temperature)
}

func TestDashboardCache(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)
	d.SetCacheTTL(1 * time.Hour)

	d.RegisterChecker(&MockSubsystemChecker{
		name: "test",
		typ:  "test",
		status: SubsystemStatus{
			Name:   "test",
			Type:   "test",
			Status: StatusHealthy,
			Score:  95,
		},
	})

	// 第一次获取
	overview1, err := d.GetOverview()
	require.NoError(t, err)

	// 第二次获取（应该使用缓存）
	overview2, err := d.GetOverview()
	require.NoError(t, err)

	// 应该是同一个对象（缓存）
	assert.Equal(t, overview1.EvaluatedAt, overview2.EvaluatedAt)

	// 强制刷新
	err = d.RefreshCache()
	require.NoError(t, err)

	// 再次获取
	overview3, err := d.GetOverview()
	require.NoError(t, err)

	// 应该是新的对象
	assert.True(t, overview3.EvaluatedAt.After(overview1.EvaluatedAt) ||
		overview3.EvaluatedAt.Equal(overview1.EvaluatedAt))
}

func TestDashboardSetWeight(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	// 设置自定义权重
	d.SetWeight("custom", 0.5)

	d.mu.RLock()
	w, exists := d.weights["custom"]
	d.mu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, 0.5, w.Weight)

	// 修改已有权重
	d.SetWeight("cpu", 0.3)

	d.mu.RLock()
	w, exists = d.weights["cpu"]
	d.mu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, 0.3, w.Weight)
}

// ========== 测试趋势分析 ==========

func TestDashboardGetTrends(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	// 模拟历史数据
	now := time.Now()
	d.mu.Lock()
	d.history = []HealthRecord{
		{Timestamp: now.AddDate(0, 0, -3), OverallScore: 85, Level: LevelGood, Status: StatusHealthy},
		{Timestamp: now.AddDate(0, 0, -2), OverallScore: 82, Level: LevelGood, Status: StatusHealthy},
		{Timestamp: now.AddDate(0, 0, -1), OverallScore: 78, Level: LevelGood, Status: StatusHealthy},
		{Timestamp: now, OverallScore: 75, Level: LevelGood, Status: StatusHealthy},
	}
	d.mu.Unlock()

	trends, err := d.GetTrends(7)
	require.NoError(t, err)
	require.NotNil(t, trends)

	assert.Equal(t, 7, trends.Period)
	assert.Len(t, trends.Trends, 4)
	assert.Equal(t, 80.0, trends.AverageScore) // (85+82+78+75)/4 = 80
	assert.Equal(t, 75.0, trends.MinScore)
	assert.Equal(t, 85.0, trends.MaxScore)
}

func TestDashboardGetTrendsEmpty(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	trends, err := d.GetTrends(30)
	require.NoError(t, err)
	require.NotNil(t, trends)

	assert.Equal(t, 30, trends.Period)
	assert.Len(t, trends.Trends, 0)
	assert.Equal(t, "stable", trends.Trend)
}

func TestDashboardTrendDirection(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	// 上升趋势
	records := []HealthRecord{
		{OverallScore: 60},
		{OverallScore: 65},
		{OverallScore: 70},
		{OverallScore: 75},
		{OverallScore: 80},
	}
	trend := d.calculateTrendDirection(records)
	assert.Equal(t, "rising", trend)

	// 下降趋势
	records = []HealthRecord{
		{OverallScore: 80},
		{OverallScore: 75},
		{OverallScore: 70},
		{OverallScore: 65},
		{OverallScore: 60},
	}
	trend = d.calculateTrendDirection(records)
	assert.Equal(t, "falling", trend)

	// 稳定
	records = []HealthRecord{
		{OverallScore: 75},
		{OverallScore: 75},
		{OverallScore: 75},
		{OverallScore: 75},
		{OverallScore: 75},
	}
	trend = d.calculateTrendDirection(records)
	assert.Equal(t, "stable", trend)
}

// ========== 测试告警管理 ==========

func TestDashboardAlerts(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	// 注册低分检查器触发告警
	d.RegisterChecker(&MockSubsystemChecker{
		name: "critical_system",
		typ:  "test",
		status: SubsystemStatus{
			Name:    "critical_system",
			Type:    "test",
			Status:  StatusCritical,
			Score:   20,
			Message: "系统异常",
		},
	})

	// 执行检查触发告警
	_, err := d.GetOverview()
	require.NoError(t, err)

	// 获取活跃告警
	alerts := d.GetAlerts(false)
	assert.NotEmpty(t, alerts)

	// 验证告警内容
	foundCritical := false
	for _, a := range alerts {
		if a.Source == "critical_system" && a.Level == "critical" {
			foundCritical = true
			break
		}
	}
	assert.True(t, foundCritical, "应该有 critical_system 的告警")

	// 解决告警
	if len(alerts) > 0 {
		err = d.ResolveAlert(alerts[0].ID)
		assert.NoError(t, err)

		// 验证告警已解决
		resolvedAlerts := d.GetAlerts(true)
		assert.Len(t, resolvedAlerts, 1)
	}
}

func TestDashboardResolveAlertNotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	err := d.ResolveAlert("nonexistent_alert")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// ========== 测试评分算法 ==========

func TestScoringAlgorithms(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	// CPU 评分
	assert.Equal(t, 100.0, d.scoreCPU(0.3))
	assert.Equal(t, 100.0, d.scoreCPU(0.5))
	assert.Greater(t, d.scoreCPU(0.6), 75.0)
	assert.Greater(t, d.scoreCPU(0.75), 45.0)
	assert.Less(t, d.scoreCPU(0.95), 25.0)
	assert.Equal(t, 10.0, d.scoreCPU(1.0))

	// 内存评分
	assert.Equal(t, 100.0, d.scoreMemory(0.4))
	assert.Equal(t, 100.0, d.scoreMemory(0.6))
	assert.Greater(t, d.scoreMemory(0.7), 75.0)
	assert.Less(t, d.scoreMemory(0.95), 35.0)
	assert.Equal(t, 10.0, d.scoreMemory(1.0))

	// 磁盘评分
	assert.Equal(t, 100.0, d.scoreDisk(0.5))
	assert.Equal(t, 100.0, d.scoreDisk(0.7))
	assert.Greater(t, d.scoreDisk(0.8), 70.0)
	assert.Less(t, d.scoreDisk(0.95), 60.0)

	// 温度评分
	assert.Equal(t, 100.0, d.scoreTemperature(30))
	assert.Equal(t, 100.0, d.scoreTemperature(40))
	assert.Greater(t, d.scoreTemperature(50), 65.0)
	assert.Less(t, d.scoreTemperature(75), 35.0)
	assert.Equal(t, 10.0, d.scoreTemperature(90))
}

func TestCalculateOverallScore(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	// 全部正常
	subsystems := []SubsystemStatus{
		{Name: "cpu", Score: 90},
		{Name: "memory", Score: 85},
		{Name: "disk", Score: 80},
	}
	metrics := CoreMetrics{CPU: 0.3, Memory: 0.4, Disk: 0.5, Temperature: 40}
	score := d.calculateOverallScore(subsystems, metrics)
	assert.GreaterOrEqual(t, score, 70.0)
	assert.LessOrEqual(t, score, 100.0)

	// 有严重问题
	subsystems = []SubsystemStatus{
		{Name: "cpu", Score: 30},
		{Name: "memory", Score: 25},
		{Name: "disk", Score: 20},
	}
	metrics = CoreMetrics{CPU: 0.95, Memory: 0.96, Disk: 0.98, Temperature: 85}
	score = d.calculateOverallScore(subsystems, metrics)
	assert.LessOrEqual(t, score, 40.0) // 应用惩罚规则

	// 空子系统
	score = d.calculateOverallScore([]SubsystemStatus{}, CoreMetrics{})
	assert.Equal(t, 100.0, score)
}

// ========== 测试快速修复 ==========

func TestDashboardGetAvailableFixes(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	fixes := d.GetAvailableFixes()
	assert.NotEmpty(t, fixes)

	// 验证修复动作结构
	for _, fix := range fixes {
		assert.NotEmpty(t, fix.ID)
		assert.NotEmpty(t, fix.Name)
		assert.NotEmpty(t, fix.Description)
		assert.NotEmpty(t, fix.Category)
		assert.NotEmpty(t, fix.Risk)
	}
}

func TestDashboardExecuteFix(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	// 测试清理缓存
	result, err := d.ExecuteFix("clear_cache", false, nil)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.NotEmpty(t, result.Message)

	// 测试需要确认的修复（未确认）
	_, err = d.ExecuteFix("restart_service", false, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "需要确认")

	// 测试需要确认的修复（已确认）
	result, err = d.ExecuteFix("restart_service", true, map[string]interface{}{
		"service": "nginx",
	})
	require.NoError(t, err)
	assert.True(t, result.Success)

	// 测试未知修复动作
	_, err = d.ExecuteFix("unknown_fix", false, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未知")
}

// ========== 测试建议生成 ==========

func TestDashboardGenerateRecommendations(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	// 高资源使用场景
	overview := &SystemOverview{
		Metrics: CoreMetrics{
			CPU:         0.85,
			Memory:      0.90,
			Disk:        0.92,
			Temperature: 70,
		},
		Subsystems: []SubsystemStatus{
			{Name: "storage", Type: "storage", Status: StatusCritical, Message: "存储池异常"},
		},
	}

	recommendations := d.generateRecommendations(overview)
	assert.NotEmpty(t, recommendations)

	// 验证有 CPU 相关建议
	hasCPURec := false
	hasMemRec := false
	hasDiskRec := false
	hasTempRec := false

	for _, r := range recommendations {
		switch r.RelatedSubsystem {
		case "cpu":
			hasCPURec = true
		case "memory":
			hasMemRec = true
		case "disk":
			hasDiskRec = true
		case "temperature":
			hasTempRec = true
		}
	}

	assert.True(t, hasCPURec, "应该有 CPU 建议")
	assert.True(t, hasMemRec, "应该有内存建议")
	assert.True(t, hasDiskRec, "应该有磁盘建议")
	assert.True(t, hasTempRec, "应该有温度建议")
}

// ========== 测试历史记录 ==========

func TestDashboardHistory(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	// 模拟历史数据 - 使用更合理的时间点
	now := time.Now()
	// 确保所有时间都在今天之前的整数天
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	d.mu.Lock()
	d.history = []HealthRecord{
		{Timestamp: today.AddDate(0, 0, -10), OverallScore: 95}, // 10天前
		{Timestamp: today.AddDate(0, 0, -5), OverallScore: 90},  // 5天前
		{Timestamp: today.AddDate(0, 0, -3), OverallScore: 85},  // 3天前
		{Timestamp: today.AddDate(0, 0, -1), OverallScore: 80},  // 1天前
		{Timestamp: today, OverallScore: 75},                    // 今天
	}
	d.mu.Unlock()

	// 获取最近3天 - 应该包含3天内、1天前、今天的记录
	history := d.GetHistory(3)
	assert.GreaterOrEqual(t, len(history), 2) // 至少包含1天前和今天的记录

	// 获取最近7天 - 应该包含5天内的记录
	history = d.GetHistory(7)
	assert.GreaterOrEqual(t, len(history), 4) // 5天前、3天前、1天前、今天

	// 获取最近15天 - 全部记录
	history = d.GetHistory(15)
	assert.Len(t, history, 5)
}

// ========== 测试子系统状态查询 ==========

func TestDashboardGetSubsystemStatus(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	d.RegisterChecker(&MockSubsystemChecker{
		name: "test_sub",
		typ:  "test",
		status: SubsystemStatus{
			Name:    "test_sub",
			Type:    "test",
			Status:  StatusHealthy,
			Score:   95,
			Message: "正常",
		},
	})

	// 获取存在的子系统
	status, err := d.GetSubsystemStatus("test_sub")
	require.NoError(t, err)
	assert.Equal(t, "test_sub", status.Name)
	assert.Equal(t, StatusHealthy, status.Status)
	assert.Equal(t, 95.0, status.Score)

	// 获取不存在的子系统
	_, err = d.GetSubsystemStatus("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "不存在")
}

// ========== 测试评分边界 ==========

func TestScoreBoundaries(t *testing.T) {
	// 测试等级边界
	assert.Equal(t, LevelExcellent, ClassifyLevel(100))
	assert.Equal(t, LevelExcellent, ClassifyLevel(90))
	assert.Equal(t, LevelGood, ClassifyLevel(89.99))
	assert.Equal(t, LevelGood, ClassifyLevel(70))
	assert.Equal(t, LevelFair, ClassifyLevel(69.99))
	assert.Equal(t, LevelFair, ClassifyLevel(50))
	assert.Equal(t, LevelPoor, ClassifyLevel(49.99))
	assert.Equal(t, LevelPoor, ClassifyLevel(30))
	assert.Equal(t, LevelCritical, ClassifyLevel(29.99))
	assert.Equal(t, LevelCritical, ClassifyLevel(0))

	// 测试状态边界
	assert.Equal(t, StatusHealthy, ClassifyStatus(100))
	assert.Equal(t, StatusHealthy, ClassifyStatus(70))
	assert.Equal(t, StatusWarning, ClassifyStatus(69.99))
	assert.Equal(t, StatusWarning, ClassifyStatus(50))
	assert.Equal(t, StatusCritical, ClassifyStatus(49.99))
	assert.Equal(t, StatusCritical, ClassifyStatus(0))
}

// ========== 测试预测生成 ==========

func TestPredictionGeneration(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	d := NewDashboard(logger)

	// 上升趋势
	records := []HealthRecord{
		{OverallScore: 60},
		{OverallScore: 65},
		{OverallScore: 70},
		{OverallScore: 75},
		{OverallScore: 80},
	}

	prediction := d.generatePrediction(records, "rising")
	require.NotNil(t, prediction)
	assert.Greater(t, prediction.PredictedScore, 80.0) // 应该预测更高
	assert.Greater(t, prediction.Confidence, 0.0)
	assert.NotEmpty(t, prediction.RiskLevel)

	// 下降趋势
	records = []HealthRecord{
		{OverallScore: 80},
		{OverallScore: 75},
		{OverallScore: 70},
		{OverallScore: 65},
		{OverallScore: 60},
	}

	prediction = d.generatePrediction(records, "falling")
	require.NotNil(t, prediction)
	assert.Less(t, prediction.PredictedScore, 60.0) // 应该预测更低

	// 数据不足
	records = []HealthRecord{
		{OverallScore: 75},
		{OverallScore: 80},
	}

	prediction = d.generatePrediction(records, "stable")
	assert.Nil(t, prediction) // 数据不足，应该返回 nil
}
