package healthscore

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewScorer(t *testing.T) {
	scorer := NewScorer(nil)
	if scorer == nil {
		t.Fatal("NewScorer返回nil")
	}
	if scorer.threshold != 50 {
		t.Errorf("默认阈值应为50，实际为 %f", scorer.threshold)
	}
}

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

func TestCalculate(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	scorer := NewScorer(logger)

	score, err := scorer.Calculate(CalculateRequest{})
	if err != nil {
		t.Fatalf("Calculate失败: %v", err)
	}

	if score.Score < 0 || score.Score > 100 {
		t.Errorf("综合评分超出范围: %f", score.Score)
	}
	if score.Level == "" {
		t.Error("健康等级为空")
	}
	if len(score.Categories) != 4 {
		t.Errorf("分项评分数应为4，实际为 %d", len(score.Categories))
	}
	if score.EvaluatedAt.IsZero() {
		t.Error("评估时间为空")
	}
}

func TestCalculateWithThreshold(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	scorer := NewScorer(logger)

	// 设一个很高的阈值，必定触发告警
	score, err := scorer.Calculate(CalculateRequest{Threshold: 99})
	if err != nil {
		t.Fatalf("Calculate失败: %v", err)
	}

	if len(score.Alerts) == 0 {
		t.Error("评分低于阈值99应触发告警")
	}
}

func TestGetLastScore(t *testing.T) {
	scorer := NewScorer(nil)

	// 未评分前应返回错误
	_, err := scorer.GetLastScore()
	if err != ErrNoScoreData {
		t.Errorf("应返回ErrNoScoreData，实际: %v", err)
	}

	// 评分后可获取
	scorer.Calculate(CalculateRequest{})
	score, err := scorer.GetLastScore()
	if err != nil {
		t.Fatalf("GetLastScore失败: %v", err)
	}
	if score.Score < 0 || score.Score > 100 {
		t.Errorf("评分超出范围: %f", score.Score)
	}
}

func TestGetDetails(t *testing.T) {
	scorer := NewScorer(nil)
	scorer.Calculate(CalculateRequest{})

	details, err := scorer.GetDetails()
	if err != nil {
		t.Fatalf("GetDetails失败: %v", err)
	}

	if details.SmartStatus.TotalDisks == 0 {
		t.Error("磁盘数不应为0")
	}
	if details.RaidStatus.Level == "" {
		t.Error("RAID级别不应为空")
	}
	if details.BackupInfo.Coverage < 0 || details.BackupInfo.Coverage > 1 {
		t.Errorf("备份覆盖率超出范围: %f", details.BackupInfo.Coverage)
	}
}

func TestGetHistory(t *testing.T) {
	scorer := NewScorer(nil)

	// 多次评分产生历史
	for i := 0; i < 5; i++ {
		scorer.Calculate(CalculateRequest{})
	}

	resp := scorer.GetHistory(HistoryQuery{Days: 1, Limit: 10})
	if resp.TotalCount != 5 {
		t.Errorf("历史记录数应为5，实际为 %d", resp.TotalCount)
	}
	if resp.AvgScore < 0 || resp.AvgScore > 100 {
		t.Errorf("平均分超出范围: %f", resp.AvgScore)
	}
	if resp.Trend == "" {
		t.Error("趋势不应为空")
	}
}

func TestGetRecommendations(t *testing.T) {
	scorer := NewScorer(nil)

	// 未评分前应返回错误
	_, err := scorer.GetRecommendations()
	if err != ErrNoScoreData {
		t.Errorf("应返回ErrNoScoreData，实际: %v", err)
	}

	scorer.Calculate(CalculateRequest{})
	resp, err := scorer.GetRecommendations()
	if err != nil {
		t.Fatalf("GetRecommendations失败: %v", err)
	}
	if resp.Level == "" {
		t.Error("等级不应为空")
	}
	if resp.GeneratedAt.IsZero() {
		t.Error("生成时间不应为空")
	}
}

func TestSetThreshold(t *testing.T) {
	scorer := NewScorer(nil)
	scorer.SetThreshold(80)
	if scorer.threshold != 80 {
		t.Errorf("阈值应为80，实际为 %f", scorer.threshold)
	}
}

func TestCategoryWeights(t *testing.T) {
	scorer := NewScorer(nil)
	score, _ := scorer.Calculate(CalculateRequest{})

	var weightSum float64
	for _, cat := range score.Categories {
		weightSum += cat.Weight
	}
	if weightSum < 0.99 || weightSum > 1.01 {
		t.Errorf("权重总和应约等于1，实际为 %f", weightSum)
	}
}

func TestSeverityWeight(t *testing.T) {
	tests := []struct {
		severity string
		want     int
	}{
		{"high", 3},
		{"medium", 2},
		{"low", 1},
		{"unknown", 0},
	}
	for _, tt := range tests {
		got := severityWeight(tt.severity)
		if got != tt.want {
			t.Errorf("severityWeight(%q) = %d, want %d", tt.severity, got, tt.want)
		}
	}
}

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
