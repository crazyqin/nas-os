package smarthealthscore

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ========== 类型测试 ==========

func TestClassifyLevel(t *testing.T) {
	tests := []struct {
		score float64
		want  HealthLevel
	}{
		{95, LevelExcellent},
		{90, LevelExcellent},
		{75, LevelGood},
		{70, LevelGood},
		{55, LevelWarning},
		{50, LevelWarning},
		{35, LevelCritical},
		{30, LevelCritical},
		{15, LevelDanger},
		{0, LevelDanger},
	}
	for _, tt := range tests {
		got := ClassifyLevel(tt.score)
		if got != tt.want {
			t.Errorf("ClassifyLevel(%v) = %v, want %v", tt.score, got, tt.want)
		}
	}
}

func TestAllCategories(t *testing.T) {
	cats := AllCategories()
	if len(cats) != 5 {
		t.Errorf("AllCategories()返回%d个维度，期望5个", len(cats))
	}

	expected := map[ScoreCategory]bool{
		CategoryDisk:         true,
		CategoryNetwork:      true,
		CategorySecurity:     true,
		CategoryPerformance:  true,
		CategoryAvailability: true,
	}
	for _, cat := range cats {
		if !expected[cat] {
			t.Errorf("未知维度: %s", cat)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg == nil {
		t.Fatal("DefaultConfig()返回nil")
	}
	if cfg.Threshold != 60 {
		t.Errorf("默认阈值应为60，实际为 %f", cfg.Threshold)
	}
	if len(cfg.Weights) != 5 {
		t.Errorf("权重数量应为5，实际为 %d", len(cfg.Weights))
	}

	// 验证权重总和为1
	var sum float64
	for _, w := range cfg.Weights {
		sum += w
	}
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("权重总和应约等于1，实际为 %f", sum)
	}
}

// ========== 评分引擎测试 ==========

func TestNewScorer(t *testing.T) {
	scorer := NewScorer()
	if scorer == nil {
		t.Fatal("NewScorer()返回nil")
	}
	if scorer.config == nil {
		t.Fatal("配置不应为nil")
	}
}

func TestNewScorerWithConfig(t *testing.T) {
	cfg := &Config{
		Weights: map[ScoreCategory]float64{
			CategoryDisk: 0.5,
			CategoryNetwork: 0.5,
		},
		Threshold: 80,
	}
	scorer := NewScorerWithConfig(cfg)
	if scorer.config.Threshold != 80 {
		t.Errorf("阈值应为80，实际为 %f", scorer.config.Threshold)
	}
}

func TestNewScorerWithNilConfig(t *testing.T) {
	scorer := NewScorerWithConfig(nil)
	if scorer.config == nil {
		t.Fatal("nil配置应使用默认配置")
	}
	if scorer.config.Threshold != 60 {
		t.Errorf("应使用默认阈值60，实际为 %f", scorer.config.Threshold)
	}
}

func TestCalculateOverallScore(t *testing.T) {
	scorer := NewScorer()

	score, err := scorer.CalculateOverallScore()
	if err != nil {
		t.Fatalf("CalculateOverallScore失败: %v", err)
	}

	// 验证评分范围
	if score.Overall < 0 || score.Overall > 100 {
		t.Errorf("综合评分超出范围: %f", score.Overall)
	}

	// 验证等级
	if score.Level == "" {
		t.Error("健康等级不应为空")
	}

	// 验证维度数量
	if len(score.Components) != 5 {
		t.Errorf("维度评分数应为5，实际为 %d", len(score.Components))
	}

	// 验证评估时间
	if score.EvaluatedAt.IsZero() {
		t.Error("评估时间不应为空")
	}

	// 验证建议
	if len(score.Suggestions) == 0 {
		t.Error("改进建议不应为空")
	}
}

func TestScoreDisk(t *testing.T) {
	scorer := NewScorer()
	component := scorer.ScoreDisk()

	if component.Category != CategoryDisk {
		t.Errorf("类别应为disk，实际为 %s", component.Category)
	}
	if component.Score < 0 || component.Score > 100 {
		t.Errorf("评分超出范围: %f", component.Score)
	}
	if component.Weight <= 0 {
		t.Error("权重应大于0")
	}
	if component.Level == "" {
		t.Error("等级不应为空")
	}
	if len(component.Metrics) == 0 {
		t.Error("指标不应为空")
	}
}

func TestScoreNetwork(t *testing.T) {
	scorer := NewScorer()
	component := scorer.ScoreNetwork()

	if component.Category != CategoryNetwork {
		t.Errorf("类别应为network，实际为 %s", component.Category)
	}
	if component.Score < 0 || component.Score > 100 {
		t.Errorf("评分超出范围: %f", component.Score)
	}
}

func TestScoreSecurity(t *testing.T) {
	scorer := NewScorer()
	component := scorer.ScoreSecurity()

	if component.Category != CategorySecurity {
		t.Errorf("类别应为security，实际为 %s", component.Category)
	}
	if component.Score < 0 || component.Score > 100 {
		t.Errorf("评分超出范围: %f", component.Score)
	}
}

func TestScorePerformance(t *testing.T) {
	scorer := NewScorer()
	component := scorer.ScorePerformance()

	if component.Category != CategoryPerformance {
		t.Errorf("类别应为performance，实际为 %s", component.Category)
	}
	if component.Score < 0 || component.Score > 100 {
		t.Errorf("评分超出范围: %f", component.Score)
	}
}

func TestScoreAvailability(t *testing.T) {
	scorer := NewScorer()
	component := scorer.ScoreAvailability()

	if component.Category != CategoryAvailability {
		t.Errorf("类别应为availability，实际为 %s", component.Category)
	}
	if component.Score < 0 || component.Score > 100 {
		t.Errorf("评分超出范围: %f", component.Score)
	}
}

func TestGetLastScore(t *testing.T) {
	scorer := NewScorer()

	// 未评分前应返回错误
	_, err := scorer.GetLastScore()
	if err != ErrNoScoreData {
		t.Errorf("应返回ErrNoScoreData，实际: %v", err)
	}

	// 评分后可获取
	scorer.CalculateOverallScore()
	score, err := scorer.GetLastScore()
	if err != nil {
		t.Fatalf("GetLastScore失败: %v", err)
	}
	if score.Overall < 0 || score.Overall > 100 {
		t.Errorf("评分超出范围: %f", score.Overall)
	}
}

func TestSetWeights(t *testing.T) {
	scorer := NewScorer()

	newWeights := map[ScoreCategory]float64{
		CategoryDisk:    0.4,
		CategoryNetwork: 0.3,
		CategorySecurity: 0.3,
	}
	scorer.SetWeights(newWeights)

	cfg := scorer.GetConfig()
	if cfg.Weights[CategoryDisk] != 0.4 {
		t.Errorf("磁盘权重应为0.4，实际为 %f", cfg.Weights[CategoryDisk])
	}
}

func TestSetThreshold(t *testing.T) {
	scorer := NewScorer()

	scorer.SetThreshold(80)
	cfg := scorer.GetConfig()
	if cfg.Threshold != 80 {
		t.Errorf("阈值应为80，实际为 %f", cfg.Threshold)
	}
}

func TestGetTrends(t *testing.T) {
	scorer := NewScorer()

	// 多次评分产生趋势数据
	for i := 0; i < 5; i++ {
		scorer.CalculateOverallScore()
		time.Sleep(10 * time.Millisecond)
	}

	query := TrendQuery{Days: 1, Limit: 10}
	resp := scorer.GetTrends(query)

	if resp.TotalCount != 5 {
		t.Errorf("趋势记录数应为5，实际为 %d", resp.TotalCount)
	}
	if resp.AvgScore < 0 || resp.AvgScore > 100 {
		t.Errorf("平均分超出范围: %f", resp.AvgScore)
	}
	if resp.Trend == "" {
		t.Error("趋势方向不应为空")
	}
}

func TestGetTrendsWithCategory(t *testing.T) {
	scorer := NewScorer()

	scorer.CalculateOverallScore()

	query := TrendQuery{Days: 1, Category: CategoryDisk}
	resp := scorer.GetTrends(query)

	if resp.TotalCount != 1 {
		t.Errorf("趋势记录数应为1，实际为 %d", resp.TotalCount)
	}
}

func TestGetAlerts(t *testing.T) {
	scorer := NewScorer()

	// 设置高阈值以触发告警
	scorer.SetThreshold(99)
	scorer.CalculateOverallScore()

	query := AlertQuery{Days: 1}
	alerts := scorer.GetAlerts(query)

	if len(alerts) == 0 {
		t.Error("高阈值应触发告警")
	}
}

func TestGetAlertsNoAlerts(t *testing.T) {
	scorer := NewScorer()

	// 设置低阈值不触发告警
	scorer.SetThreshold(0)
	scorer.CalculateOverallScore()

	query := AlertQuery{Days: 1}
	alerts := scorer.GetAlerts(query)

	if len(alerts) != 0 {
		t.Errorf("低阈值不应触发告警，实际告警数: %d", len(alerts))
	}
}

func TestGetComponents(t *testing.T) {
	scorer := NewScorer()

	// 未评分前获取组件（会自动执行评分）
	components, err := scorer.GetComponents()
	if err != nil {
		t.Fatalf("GetComponents失败: %v", err)
	}
	if len(components) != 5 {
		t.Errorf("维度数应为5，实际为 %d", len(components))
	}
}

func TestGetComponentsAfterScore(t *testing.T) {
	scorer := NewScorer()

	scorer.CalculateOverallScore()
	components, err := scorer.GetComponents()
	if err != nil {
		t.Fatalf("GetComponents失败: %v", err)
	}
	if len(components) != 5 {
		t.Errorf("维度数应为5，实际为 %d", len(components))
	}
}

// ========== 工具函数测试 ==========

func TestRoundTo2(t *testing.T) {
	tests := []struct {
		input float64
		want  float64
	}{
		{1.234, 1.23},
		{1.235, 1.24},
		{100.0, 100.0},
		{0.0, 0.0},
	}
	for _, tt := range tests {
		got := roundTo2(tt.input)
		if got != tt.want {
			t.Errorf("roundTo2(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestPriorityWeight(t *testing.T) {
	tests := []struct {
		priority string
		want     int
	}{
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"unknown", 0},
	}
	for _, tt := range tests {
		got := priorityWeight(tt.priority)
		if got != tt.want {
			t.Errorf("priorityWeight(%q) = %d, want %d", tt.priority, got, tt.want)
		}
	}
}

func TestCategoryName(t *testing.T) {
	tests := []struct {
		cat  ScoreCategory
		want string
	}{
		{CategoryDisk, "磁盘"},
		{CategoryNetwork, "网络"},
		{CategorySecurity, "安全"},
		{CategoryPerformance, "性能"},
		{CategoryAvailability, "可用性"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		got := categoryName(tt.cat)
		if got != tt.want {
			t.Errorf("categoryName(%s) = %s, want %s", tt.cat, got, tt.want)
		}
	}
}

func TestIsValidCategory(t *testing.T) {
	tests := []struct {
		cat  ScoreCategory
		want bool
	}{
		{CategoryDisk, true},
		{CategoryNetwork, true},
		{CategorySecurity, true},
		{CategoryPerformance, true},
		{CategoryAvailability, true},
		{"invalid", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isValidCategory(tt.cat)
		if got != tt.want {
			t.Errorf("isValidCategory(%s) = %v, want %v", tt.cat, got, tt.want)
		}
	}
}

func TestCalcMetricAverage(t *testing.T) {
	metrics := []Metric{
		{Value: 80},
		{Value: 90},
		{Value: 70},
	}
	avg := calcMetricAverage(metrics)
	expected := 80.0
	if avg != expected {
		t.Errorf("calcMetricAverage = %f, want %f", avg, expected)
	}

	// 空指标
	emptyAvg := calcMetricAverage([]Metric{})
	if emptyAvg != 0 {
		t.Errorf("空指标平均分应为0，实际为 %f", emptyAvg)
	}
}

// ========== HTTP Handler 测试 ==========

func setupRouter() (*gin.Engine, *Scorer) {
	gin.SetMode(gin.TestMode)
	scorer := NewScorer()
	handlers := NewHandlers(scorer)

	router := gin.New()
	api := router.Group("/api")
	handlers.RegisterRoutes(api)

	return router, scorer
}

func TestGetHealthScoreHandler(t *testing.T) {
	router, _ := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/healthscore", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200，实际为 %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("JSON解析失败: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("success应为true")
	}

	data := response["data"].(map[string]interface{})
	if data["overall"] == nil {
		t.Error("overall不应为nil")
	}
}

func TestGetTrendsHandler(t *testing.T) {
	router, scorer := setupRouter()

	// 先产生一些趋势数据
	scorer.CalculateOverallScore()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/healthscore/trends?days=1&limit=10", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200，实际为 %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("JSON解析失败: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("success应为true")
	}
}

func TestGetTrendsHandlerWithCategory(t *testing.T) {
	router, scorer := setupRouter()

	scorer.CalculateOverallScore()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/healthscore/trends?category=disk", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200，实际为 %d", w.Code)
	}
}

func TestGetTrendsHandlerInvalidCategory(t *testing.T) {
	router, _ := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/healthscore/trends?category=invalid", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("状态码应为400，实际为 %d", w.Code)
	}
}

func TestGetAlertsHandler(t *testing.T) {
	router, scorer := setupRouter()

	// 设置高阈值触发告警
	scorer.SetThreshold(99)
	scorer.CalculateOverallScore()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/healthscore/alerts?days=1", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200，实际为 %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("JSON解析失败: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("success应为true")
	}
}

func TestGetAlertsHandlerWithCategory(t *testing.T) {
	router, scorer := setupRouter()

	scorer.SetThreshold(99)
	scorer.CalculateOverallScore()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/healthscore/alerts?category=disk", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200，实际为 %d", w.Code)
	}
}

func TestGetAlertsHandlerInvalidCategory(t *testing.T) {
	router, _ := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/healthscore/alerts?category=invalid", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("状态码应为400，实际为 %d", w.Code)
	}
}

func TestGetComponentsHandler(t *testing.T) {
	router, _ := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/healthscore/components", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("状态码应为200，实际为 %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("JSON解析失败: %v", err)
	}

	if !response["success"].(bool) {
		t.Error("success应为true")
	}

	data := response["data"].(map[string]interface{})
	if data["components"] == nil {
		t.Error("components不应为nil")
	}
	if data["config"] == nil {
		t.Error("config不应为nil")
	}
}

// ========== JSON序列化测试 ==========

func TestHealthScoreJSON(t *testing.T) {
	score := &HealthScore{
		Overall: 85.5,
		Level:   LevelGood,
		Components: []ComponentScore{
			{
				Category:    CategoryDisk,
				Score:       90,
				Weight:      0.25,
				Level:       LevelExcellent,
				Description: "磁盘健康",
				Metrics: []Metric{
					{Name: "使用率", Value: 70, Unit: "%", Status: "healthy"},
				},
			},
		},
		Alerts: []Alert{
			{
				Timestamp: time.Now(),
				Category:  CategoryDisk,
				Score:     90,
				Threshold: 60,
				Level:     LevelExcellent,
				Message:   "测试告警",
			},
		},
		Suggestions: []Suggestion{
			{
				Category:    CategoryDisk,
				Priority:    "low",
				Title:       "测试建议",
				Description: "测试描述",
				Action:      "测试操作",
			},
		},
		EvaluatedAt: time.Now(),
	}

	data, err := json.Marshal(score)
	if err != nil {
		t.Fatalf("JSON序列化失败: %v", err)
	}

	var decoded HealthScore
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("JSON反序列化失败: %v", err)
	}

	if decoded.Overall != score.Overall {
		t.Errorf("Overall不匹配: %f != %f", decoded.Overall, score.Overall)
	}
	if decoded.Level != score.Level {
		t.Errorf("Level不匹配: %s != %s", decoded.Level, score.Level)
	}
	if len(decoded.Components) != len(score.Components) {
		t.Errorf("Components数量不匹配: %d != %d", len(decoded.Components), len(score.Components))
	}
}
