package securityscore

import (
	"testing"
)

func TestRunAllChecks(t *testing.T) {
	m := NewManager()
	checks := m.RunAllChecks()

	if len(checks) == 0 {
		t.Error("expected at least 1 check")
	}

	// 验证不同分类的检查都存在
	categories := make(map[string]bool)
	for _, c := range checks {
		categories[c.Category] = true
	}

	expectedCategories := []string{"认证与授权", "网络安全", "数据保护", "日志与监控", "系统加固"}
	for _, cat := range expectedCategories {
		if !categories[cat] {
			t.Errorf("expected category %q to have checks", cat)
		}
	}
}

func TestCalculateScore(t *testing.T) {
	m := NewManager()
	m.RunAllChecks()

	score := m.CalculateScore()

	if score.Overall < 0 || score.Overall > 100 {
		t.Errorf("expected overall score between 0-100, got %.1f", score.Overall)
	}
	if score.Grade == "" {
		t.Error("expected grade to be set")
	}
	if len(score.Categories) == 0 {
		t.Error("expected categories to be populated")
	}
	if score.LastUpdated.IsZero() {
		t.Error("expected LastUpdated to be set")
	}
}

func TestGetScoreBeforeCalculation(t *testing.T) {
	m := NewManager()
	_, err := m.GetScore()
	if err == nil {
		t.Error("expected error when score not calculated")
	}
}

func TestGetScoreAfterCalculation(t *testing.T) {
	m := NewManager()
	m.RunAllChecks()
	m.CalculateScore()

	score, err := m.GetScore()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score.Overall == 0 {
		t.Error("expected non-zero overall score")
	}
}

func TestGetCheckDetails(t *testing.T) {
	m := NewManager()
	m.RunAllChecks()

	check, err := m.GetCheckDetails("AUTH-001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Name != "密码策略" {
		t.Errorf("expected name '密码策略', got %q", check.Name)
	}
	if check.Category != "认证与授权" {
		t.Errorf("expected category '认证与授权', got %q", check.Category)
	}
}

func TestGetCheckDetailsNotFound(t *testing.T) {
	m := NewManager()
	m.RunAllChecks()

	_, err := m.GetCheckDetails("NONEXISTENT")
	if err == nil {
		t.Error("expected error for nonexistent check")
	}
}

func TestScoreHistory(t *testing.T) {
	m := NewManager()
	m.RunAllChecks()

	// 计算多次评分
	m.CalculateScore()
	m.CalculateScore()
	m.CalculateScore()

	history := m.GetScoreHistory()
	if len(history) != 3 {
		t.Errorf("expected 3 history entries, got %d", len(history))
	}

	// 验证按时间倒序
	for i := 1; i < len(history); i++ {
		if history[i].Timestamp.After(history[i-1].Timestamp) {
			t.Error("expected history to be sorted by time descending")
		}
	}
}

func TestGetRecommendations(t *testing.T) {
	m := NewManager()
	m.RunAllChecks()

	recs := m.GetRecommendations()

	// 应该有 fail 和 warning 的检查产生建议
	if len(recs) == 0 {
		t.Error("expected at least 1 recommendation")
	}

	// 验证优先级排序
	for i := 1; i < len(recs); i++ {
		priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		if priorityOrder[recs[i].Priority] < priorityOrder[recs[i-1].Priority] {
			t.Error("expected recommendations sorted by priority")
		}
	}

	// 验证建议都有内容
	for _, rec := range recs {
		if rec.ID == "" {
			t.Error("expected recommendation ID")
		}
		if rec.Title == "" {
			t.Error("expected recommendation title")
		}
		if rec.Category == "" {
			t.Error("expected recommendation category")
		}
	}
}

func TestGradeMapping(t *testing.T) {
	m := NewManager()

	tests := []struct {
		score float64
		grade Grade
	}{
		{95, GradeA},
		{90, GradeA},
		{85, GradeB},
		{80, GradeB},
		{75, GradeC},
		{70, GradeC},
		{65, GradeD},
		{60, GradeD},
		{55, GradeF},
		{0, GradeF},
	}

	for _, tt := range tests {
		got := m.scoreToGrade(tt.score)
		if got != tt.grade {
			t.Errorf("scoreToGrade(%.1f) = %q, want %q", tt.score, got, tt.grade)
		}
	}
}

func TestCategoryScores(t *testing.T) {
	m := NewManager()
	m.RunAllChecks()
	score := m.CalculateScore()

	// 验证每个分类都有评分
	for name, cat := range score.Categories {
		if cat.Score < 0 || cat.Score > 100 {
			t.Errorf("category %q score should be 0-100, got %.1f", name, cat.Score)
		}
		if cat.Weight <= 0 {
			t.Errorf("category %q weight should be > 0", name)
		}
		if len(cat.Checks) == 0 {
			t.Errorf("category %q should have at least 1 check", name)
		}
	}

	// 验证权重总和约等于 1
	totalWeight := 0.0
	for _, cat := range score.Categories {
		totalWeight += cat.Weight
	}
	if totalWeight < 0.99 || totalWeight > 1.01 {
		t.Errorf("total weight should be ~1.0, got %.2f", totalWeight)
	}
}

func TestCheckStatusValues(t *testing.T) {
	m := NewManager()
	checks := m.RunAllChecks()

	for _, check := range checks {
		switch check.Status {
		case StatusPass, StatusFail, StatusWarning:
			// ok
		default:
			t.Errorf("check %q has invalid status %q", check.ID, check.Status)
		}
	}
}

func TestMultipleCalculateScore(t *testing.T) {
	m := NewManager()
	m.RunAllChecks()

	// 多次计算应该得到相同结果
	score1 := m.CalculateScore()
	score2 := m.CalculateScore()

	if score1.Overall != score2.Overall {
		t.Errorf("expected consistent scores, got %.1f and %.1f", score1.Overall, score2.Overall)
	}
}
