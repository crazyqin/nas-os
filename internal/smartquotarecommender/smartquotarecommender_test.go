package smartquotarecommender

import (
	"testing"
	"time"
)

func TestNewQuotaRecommender(t *testing.T) {
	qr := NewQuotaRecommender()
	if qr == nil {
		t.Fatal("NewQuotaRecommender 返回 nil")
	}
	if qr.profiles == nil {
		t.Fatal("profiles map 未初始化")
	}
	if qr.departments == nil {
		t.Fatal("departments map 未初始化")
	}
	if qr.forecastModel == nil {
		t.Fatal("forecastModel 未初始化")
	}
	if qr.anomalyDetector == nil {
		t.Fatal("anomalyDetector 未初始化")
	}
}

func TestAddAndGetProfile(t *testing.T) {
	qr := NewQuotaRecommender()

	// 测试添加空用户ID
	err := qr.AddProfile(&UserProfile{UserID: ""})
	if err == nil {
		t.Fatal("应该返回错误：用户ID为空")
	}

	// 测试正常添加
	profile := &UserProfile{
		UserID:        "user-001",
		Department:    "engineering",
		Role:          "developer",
		CurrentQuota:  10 * 1024 * 1024 * 1024, // 10GB
		CurrentUsage:  6 * 1024 * 1024 * 1024,  // 6GB
		BusinessType:  "开发",
		PriorityLevel: 3,
		CreatedAt:     time.Now(),
		LastActive:    time.Now(),
	}

	err = qr.AddProfile(profile)
	if err != nil {
		t.Fatalf("添加用户失败: %v", err)
	}

	// 测试获取
	got, err := qr.GetProfile("user-001")
	if err != nil {
		t.Fatalf("获取用户失败: %v", err)
	}
	if got.UserID != "user-001" {
		t.Fatalf("期望 user-001，得到 %s", got.UserID)
	}
	if got.Department != "engineering" {
		t.Fatalf("期望 engineering，得到 %s", got.Department)
	}

	// 测试获取不存在的用户
	_, err = qr.GetProfile("nonexistent")
	if err == nil {
		t.Fatal("应该返回错误：用户不存在")
	}
}

func TestAddDepartmentPolicy(t *testing.T) {
	qr := NewQuotaRecommender()

	// 测试空部门ID
	err := qr.AddDepartmentPolicy(&DepartmentPolicy{DepartmentID: ""})
	if err == nil {
		t.Fatal("应该返回错误：部门ID为空")
	}

	// 测试正常添加
	policy := &DepartmentPolicy{
		DepartmentID:   "engineering",
		Name:           "工程部",
		DefaultQuota:   10 * 1024 * 1024 * 1024,  // 10GB
		MaxQuota:       100 * 1024 * 1024 * 1024, // 100GB
		MinQuota:       1 * 1024 * 1024 * 1024,   // 1GB
		GrowthRate:     20.0,
		ReviewCycle:    30 * 24 * time.Hour,
		ApprovalNeeded: true,
	}

	err = qr.AddDepartmentPolicy(policy)
	if err != nil {
		t.Fatalf("添加部门策略失败: %v", err)
	}

	// 验证添加成功
	qr.mu.RLock()
	got, exists := qr.departments["engineering"]
	qr.mu.RUnlock()
	if !exists {
		t.Fatal("部门策略未添加")
	}
	if got.Name != "工程部" {
		t.Fatalf("期望 工程部，得到 %s", got.Name)
	}
}

func TestAnalyzeUsagePattern(t *testing.T) {
	qr := NewQuotaRecommender()

	// 添加用户（稳定增长趋势，基础用量高，增量小）
	baseTime := time.Now().AddDate(0, 0, -30)
	history := make([]UsageSnapshot, 30)
	for i := 0; i < 30; i++ {
		history[i] = UsageSnapshot{
			Timestamp: baseTime.AddDate(0, 0, i),
			Used:      int64(5000*1024*1024 + i*100*1024*1024), // 5GB基础，每天增100MB
			Quota:     10 * 1024 * 1024 * 1024,
		}
	}

	profile := &UserProfile{
		UserID:       "user-002",
		Department:   "engineering",
		CurrentQuota: 10 * 1024 * 1024 * 1024,
		CurrentUsage: 3 * 1024 * 1024 * 1024,
		History:      history,
	}
	qr.AddProfile(profile)

	// 测试分析
	pattern, err := qr.AnalyzeUsagePattern("user-002")
	if err != nil {
		t.Fatalf("分析使用模式失败: %v", err)
	}

	// 验证结果
	if pattern.Trend != TrendGrowing {
		t.Fatalf("期望增长趋势，得到 %s", pattern.Trend)
	}
	if pattern.PeakUsage != int64(5000*1024*1024+29*100*1024*1024) {
		t.Fatalf("峰值不正确: %d", pattern.PeakUsage)
	}
	if pattern.DailyGrowth <= 0 {
		t.Fatalf("日均增量应为正数: %f", pattern.DailyGrowth)
	}

	// 测试历史数据不足
	profile2 := &UserProfile{
		UserID: "user-003",
		History: []UsageSnapshot{
			{Timestamp: time.Now(), Used: 100},
		},
	}
	qr.AddProfile(profile2)
	_, err = qr.AnalyzeUsagePattern("user-003")
	if err == nil {
		t.Fatal("应该返回错误：历史数据不足")
	}
}

func TestRecommendQuota(t *testing.T) {
	qr := NewQuotaRecommender()

	// 添加部门策略
	qr.AddDepartmentPolicy(&DepartmentPolicy{
		DepartmentID: "design",
		MaxQuota:     50 * 1024 * 1024 * 1024,
		MinQuota:     5 * 1024 * 1024 * 1024,
	})

	// 添加用户（增长趋势）
	baseTime := time.Now().AddDate(0, 0, -30)
	history := make([]UsageSnapshot, 30)
	for i := 0; i < 30; i++ {
		history[i] = UsageSnapshot{
			Timestamp: baseTime.AddDate(0, 0, i),
			Used:      int64((i + 1) * 200 * 1024 * 1024),
			Quota:     20 * 1024 * 1024 * 1024,
		}
	}

	profile := &UserProfile{
		UserID:       "user-004",
		Department:   "design",
		CurrentQuota: 20 * 1024 * 1024 * 1024,
		CurrentUsage: 6 * 1024 * 1024 * 1024,
		History:      history,
	}
	qr.AddProfile(profile)

	// 测试推荐
	rec, err := qr.RecommendQuota("user-004")
	if err != nil {
		t.Fatalf("生成推荐失败: %v", err)
	}

	// 验证推荐结果
	if rec.UserID != "user-004" {
		t.Fatalf("期望 user-004，得到 %s", rec.UserID)
	}
	if rec.RecommendedQuota <= 0 {
		t.Fatal("推荐配额应为正数")
	}
	if rec.Confidence < 0 || rec.Confidence > 1 {
		t.Fatalf("置信度应在0-1之间: %f", rec.Confidence)
	}
	if len(rec.Reasons) == 0 {
		t.Fatal("推荐理由不能为空")
	}
	if rec.AlternativeQuota <= 0 {
		t.Fatal("备选配额应为正数")
	}

	// 测试不存在的用户
	_, err = qr.RecommendQuota("nonexistent")
	if err == nil {
		t.Fatal("应该返回错误：用户不存在")
	}
}

func TestForecastModel(t *testing.T) {
	model := NewForecastModel()

	// 测试未训练时预测
	_, err := model.Forecast(7)
	if err == nil {
		t.Fatal("应该返回错误：模型未训练")
	}

	// 测试数据不足
	data := []float64{100, 200, 300}
	err = model.Train(data)
	if err == nil {
		t.Fatal("应该返回错误：数据不足")
	}

	// 训练模型
	data = make([]float64, 30)
	for i := 0; i < 30; i++ {
		data[i] = float64(i * 100)
	}
	err = model.Train(data)
	if err != nil {
		t.Fatalf("训练失败: %v", err)
	}

	// 测试预测
	result, err := model.Forecast(7)
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	if len(result.Predictions) != 7 {
		t.Fatalf("期望7个预测值，得到 %d", len(result.Predictions))
	}
	if result.Method != MethodExponentialSmoothing {
		t.Fatalf("期望指数平滑方法，得到 %s", result.Method)
	}
	if result.MSE < 0 {
		t.Fatal("MSE应为非负数")
	}

	// 测试线性回归
	model.Method = MethodLinearRegression
	result, err = model.Forecast(7)
	if err != nil {
		t.Fatalf("线性回归预测失败: %v", err)
	}
	if result.Method != MethodLinearRegression {
		t.Fatalf("期望线性回归方法，得到 %s", result.Method)
	}
}

func TestAnomalyDetector(t *testing.T) {
	detector := NewAnomalyDetector()

	// 测试数据不足
	data := []float64{100, 200}
	anomalies := detector.DetectAnomalies("user-001", data)
	if anomalies != nil {
		t.Fatal("数据不足时应返回nil")
	}

	// 测试正常数据
	data = make([]float64, 30)
	for i := 0; i < 30; i++ {
		data[i] = 100 + float64(i%5)
	}
	anomalies = detector.DetectAnomalies("user-002", data)
	if len(anomalies) > 0 {
		t.Fatalf("正常数据不应检测到异常，得到 %d 个", len(anomalies))
	}

	// 测试异常数据
	data[29] = 1000 // 突然激增
	anomalies = detector.DetectAnomalies("user-003", data)
	if len(anomalies) == 0 {
		t.Fatal("应该检测到异常")
	}

	// 验证异常事件
	found := false
	for _, a := range anomalies {
		if a.Type == AnomalySuddenSpike || a.Type == AnomalyRapidGrowth {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("应该检测到激增类异常")
	}
}

func TestComplianceChecker(t *testing.T) {
	checker := NewComplianceChecker()

	// 添加规则
	checker.AddRule(ComplianceRule{
		ID:       "rule-001",
		Name:     "最小配额限制",
		MinQuota: 5 * 1024 * 1024 * 1024,
		Enabled:  true,
	})

	checker.AddRule(ComplianceRule{
		ID:         "rule-002",
		Name:       "设计部最大配额",
		Department: "design",
		MaxQuota:   30 * 1024 * 1024 * 1024,
		Enabled:    true,
	})

	// 测试合规情况
	profile := &UserProfile{
		UserID:       "user-001",
		Department:   "engineering",
		CurrentQuota: 10 * 1024 * 1024 * 1024,
	}
	policy := &DepartmentPolicy{
		MinQuota: 1 * 1024 * 1024 * 1024,
		MaxQuota: 100 * 1024 * 1024 * 1024,
	}

	result := checker.CheckCompliance(profile, policy)
	if !result.Compliant {
		t.Fatalf("应该合规，违规项: %v", result.Violations)
	}

	// 测试违规情况
	profile2 := &UserProfile{
		UserID:       "user-002",
		Department:   "design",
		CurrentQuota: 40 * 1024 * 1024 * 1024, // 超过设计部限制
	}

	result = checker.CheckCompliance(profile2, policy)
	if result.Compliant {
		t.Fatal("应该不合规")
	}
	if len(result.Violations) == 0 {
		t.Fatal("应该有违规项")
	}

	// 测试低于部门最低限制
	profile3 := &UserProfile{
		UserID:       "user-003",
		Department:   "engineering",
		CurrentQuota: 500 * 1024 * 1024, // 低于部门最低限制
	}

	result = checker.CheckCompliance(profile3, policy)
	if result.Compliant {
		t.Fatal("应该不合规（低于部门最低限制）")
	}
}

func TestReportGenerator(t *testing.T) {
	gen := NewReportGenerator()

	// 准备测试数据
	profiles := map[string]*UserProfile{
		"user-001": {
			UserID:       "user-001",
			CurrentQuota: 10 * 1024 * 1024 * 1024,
			CurrentUsage: 9 * 1024 * 1024 * 1024, // 90%
		},
		"user-002": {
			UserID:       "user-002",
			CurrentQuota: 20 * 1024 * 1024 * 1024,
			CurrentUsage: 10 * 1024 * 1024 * 1024, // 50%
		},
	}

	// 测试生成使用量报表
	report, err := gen.GenerateReport(ReportUsage, profiles, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("生成报表失败: %v", err)
	}

	if report.ID == "" {
		t.Fatal("报表ID不能为空")
	}
	if report.Title == "" {
		t.Fatal("报表标题不能为空")
	}
	if report.Summary.TotalUsers != 2 {
		t.Fatalf("期望2个用户，得到 %d", report.Summary.TotalUsers)
	}
	if report.Summary.TotalQuota != 30*1024*1024*1024 {
		t.Fatal("总配额计算错误")
	}
	if report.Summary.TotalUsage != 19*1024*1024*1024 {
		t.Fatal("总使用量计算错误")
	}
	if len(report.Summary.TopConsumers) != 1 {
		t.Fatalf("期望1个消耗大户，得到 %d 个", len(report.Summary.TopConsumers))
	}

	// 测试空数据
	_, err = gen.GenerateReport(ReportUsage, nil, time.Hour)
	if err == nil {
		t.Fatal("应该返回错误：没有用户数据")
	}

	// 测试获取报表
	reports := gen.GetReports()
	if len(reports) != 1 {
		t.Fatalf("期望1份报表，得到 %d 份", len(reports))
	}
}

func TestAlertThreshold(t *testing.T) {
	at := &AlertThreshold{
		thresholds: make(map[string]*ThresholdConfig),
	}

	// 测试设置无效阈值
	err := at.SetThreshold(&ThresholdConfig{
		Name:       "",
		Percentage: 80,
	})
	if err == nil {
		t.Fatal("应该返回错误：名称为空")
	}

	err = at.SetThreshold(&ThresholdConfig{
		Name:       "warning",
		Percentage: 150,
	})
	if err == nil {
		t.Fatal("应该返回错误：百分比超出范围")
	}

	// 设置有效阈值
	err = at.SetThreshold(&ThresholdConfig{
		Name:       "warning",
		Level:      AlertWarning,
		Percentage: 80,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("设置阈值失败: %v", err)
	}

	err = at.SetThreshold(&ThresholdConfig{
		Name:       "critical",
		Level:      AlertCritical,
		Percentage: 95,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("设置阈值失败: %v", err)
	}

	// 测试检查 - 不触发
	alerts := at.CheckThresholds("user-001", 70)
	if len(alerts) != 0 {
		t.Fatalf("70%% 使用率不应触发告警，得到 %d 个", len(alerts))
	}

	// 测试检查 - 触发 warning
	alerts = at.CheckThresholds("user-002", 85)
	if len(alerts) != 1 {
		t.Fatalf("85%% 应触发1个告警，得到 %d 个", len(alerts))
	}
	if alerts[0].Level != AlertWarning {
		t.Fatalf("期望 warning 级别，得到 %s", alerts[0].Level)
	}

	// 测试检查 - 触发两个
	alerts = at.CheckThresholds("user-003", 98)
	if len(alerts) != 2 {
		t.Fatalf("98%% 应触发2个告警，得到 %d 个", len(alerts))
	}
}

func TestForecastMethodSwitch(t *testing.T) {
	model := NewForecastModel()
	data := make([]float64, 50)
	for i := 0; i < 50; i++ {
		data[i] = float64(i*100 + i*i)
	}
	model.Train(data)

	// 测试不同方法
	methods := []ForecastMethod{
		MethodExponentialSmoothing,
		MethodLinearRegression,
	}

	for _, method := range methods {
		model.Method = method
		result, err := model.Forecast(7)
		if err != nil {
			t.Fatalf("方法 %s 预测失败: %v", method, err)
		}
		if result.Method != method {
			t.Fatalf("方法不匹配: 期望 %s，得到 %s", method, result.Method)
		}
		if len(result.Predictions) != 7 {
			t.Fatalf("方法 %s 期望7个预测，得到 %d", method, len(result.Predictions))
		}
	}
}
