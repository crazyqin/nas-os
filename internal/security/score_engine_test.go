package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultScoreWeights(t *testing.T) {
	weights := DefaultScoreWeights()

	assert.Equal(t, 0.20, weights.Authentication)
	assert.Equal(t, 0.15, weights.AccessControl)
	assert.Equal(t, 0.20, weights.DataProtection)
	assert.Equal(t, 0.15, weights.NetworkSecurity)
	assert.Equal(t, 0.15, weights.AuditLogging)
	assert.Equal(t, 0.15, weights.SystemHardening)
}

func TestNewScoreEngine(t *testing.T) {
	// 默认配置
	config := ScoreConfig{}
	engine := NewScoreEngine(config)

	assert.NotNil(t, engine)
	assert.NotNil(t, engine.scores)
	assert.Equal(t, float64(100), engine.config.MaxScore)

	// 自定义配置
	config = ScoreConfig{
		MaxScore: 50,
		Weights: ScoreWeights{
			Authentication: 0.5,
		},
	}
	engine = NewScoreEngine(config)
	assert.Equal(t, float64(50), engine.config.MaxScore)
	assert.Equal(t, 0.5, engine.config.Weights.Authentication)
}

func TestScoreEngine_CalculateScore(t *testing.T) {
	engine := NewScoreEngine(ScoreConfig{})
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: true,
	})

	bm := NewBaselineManager()

	report := engine.CalculateScore(am, bm)

	assert.NotNil(t, report)
	assert.True(t, report.OverallScore >= 0)
	assert.True(t, report.OverallScore <= 100)
	assert.NotEmpty(t, report.Grade)
	assert.NotNil(t, report.Categories)
	assert.NotEmpty(t, report.Categories)
}

func TestScoreEngine_CalculateScore_WithLoginStats(t *testing.T) {
	engine := NewScoreEngine(ScoreConfig{})
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      100,
		AutoSave:     false,
		AlertEnabled: true,
	})

	// 添加登录记录
	for i := 0; i < 10; i++ {
		am.LogLogin(LoginLogEntry{
			Username: "admin",
			IP:       "192.168.1.1",
			Status:   "success",
		})
	}
	for i := 0; i < 2; i++ {
		am.LogLogin(LoginLogEntry{
			Username: "admin",
			IP:       "10.0.0.1",
			Status:   "failure",
			Reason:   "密码错误",
		})
	}

	bm := NewBaselineManager()
	report := engine.CalculateScore(am, bm)

	assert.NotNil(t, report)
	assert.NotEmpty(t, report.Grade)
}

func TestScoreEngine_ScoreToGrade(t *testing.T) {
	engine := NewScoreEngine(ScoreConfig{})

	tests := []struct {
		score    float64
		expected string
	}{
		{95, "A"},
		{85, "B"},
		{75, "C"},
		{65, "D"},
		{55, "F"},
		{45, "F"},
	}

	for _, tt := range tests {
		grade := engine.scoreToGrade(tt.score)
		assert.Equal(t, tt.expected, grade, "Score %f should be grade %s", tt.score, tt.expected)
	}
}

func TestScoreEngine_CalculateOverallScore(t *testing.T) {
	engine := NewScoreEngine(ScoreConfig{})

	categories := map[string]*CategoryScore{
		"authentication": {Score: 80, MaxScore: 100},
		"access_control": {Score: 90, MaxScore: 100},
	}

	score := engine.calculateOverallScore(categories)
	assert.True(t, score >= 0 && score <= 100)
}

func TestScoreEngine_AnalyzeStrengthsWeaknesses(t *testing.T) {
	engine := NewScoreEngine(ScoreConfig{})

	report := &ScoreReport{
		Categories: map[string]*CategoryScore{
			"authentication": {
				Category: "authentication",
				Score:    90,
				MaxScore: 100,
				Items: []ScoreItem{
					{RuleID: "auth_1", Name: "MFA", Score: 45, MaxScore: 50, Status: "pass"},
					{RuleID: "auth_2", Name: "Password", Score: 45, MaxScore: 50, Status: "pass"},
				},
			},
			"network": {
				Category: "network",
				Score:    40,
				MaxScore: 100,
				Items: []ScoreItem{
					{RuleID: "net_1", Name: "Firewall", Score: 20, MaxScore: 50, Status: "warning"},
					{RuleID: "net_2", Name: "IDS", Score: 20, MaxScore: 50, Status: "warning"},
				},
			},
		},
		Strengths:  make([]string, 0),
		Weaknesses: make([]string, 0),
	}

	engine.analyzeStrengthsWeaknesses(report)

	assert.NotEmpty(t, report.Strengths)
	assert.NotEmpty(t, report.Weaknesses)
}

func TestScoreEngine_GenerateRecommendations(t *testing.T) {
	engine := NewScoreEngine(ScoreConfig{})

	report := &ScoreReport{
		Categories: map[string]*CategoryScore{
			"authentication": {
				Category: "authentication",
				Score:    40,
				MaxScore: 100,
				Items: []ScoreItem{
					{RuleID: "auth_1", Name: "MFA", Score: 20, MaxScore: 50, Status: "fail", Suggestions: []string{"启用MFA"}},
				},
			},
		},
		Recommendations: make([]Recommendation, 0),
	}

	engine.generateRecommendations(report)

	assert.NotEmpty(t, report.Recommendations)
}

func TestScoreEngine_GetCategoryScore(t *testing.T) {
	engine := NewScoreEngine(ScoreConfig{})
	am := NewAuditManager()
	bm := NewBaselineManager()

	report := engine.CalculateScore(am, bm)

	// 验证所有分类都有评分
	expectedCategories := []string{
		"authentication",
		"access_control",
		"data_protection",
		"network_security",
		"audit_logging",
		"system_hardening",
	}

	for _, cat := range expectedCategories {
		score, exists := report.Categories[cat]
		assert.True(t, exists, "Category %s should exist", cat)
		assert.NotNil(t, score)
		assert.True(t, score.Score >= 0)
	}
}

func TestScoreConfig_DefaultValues(t *testing.T) {
	config := ScoreConfig{}

	engine := NewScoreEngine(config)

	// 验证默认值已设置
	assert.Equal(t, float64(100), engine.config.MaxScore)
	assert.NotZero(t, engine.config.Weights.Authentication)
}

func TestScoreEngine_UpdateInterval(t *testing.T) {
	config := ScoreConfig{
		UpdateInterval: time.Hour,
	}

	engine := NewScoreEngine(config)
	assert.Equal(t, time.Hour, engine.config.UpdateInterval)
}

func TestScoreWeights_Sum(t *testing.T) {
	weights := DefaultScoreWeights()

	sum := weights.Authentication +
		weights.AccessControl +
		weights.DataProtection +
		weights.NetworkSecurity +
		weights.AuditLogging +
		weights.SystemHardening

	assert.Equal(t, 1.0, sum)
}

func TestCategoryScore_Fields(t *testing.T) {
	now := time.Now()
	category := &CategoryScore{
		Category:    "test",
		Score:       75.5,
		MaxScore:    100,
		LastChecked: now,
		Items: []ScoreItem{
			{
				RuleID:      "rule_1",
				Name:        "Test Rule",
				Score:       40,
				MaxScore:    50,
				Status:      "pass",
				Message:     "All good",
				Suggestions: []string{"Keep it up"},
			},
		},
	}

	assert.Equal(t, "test", category.Category)
	assert.Equal(t, 75.5, category.Score)
	assert.Equal(t, 100.0, category.MaxScore)
	assert.Len(t, category.Items, 1)
}

func TestScoreItem_Fields(t *testing.T) {
	item := ScoreItem{
		RuleID:      "SEC-001",
		Name:        "Password Policy",
		Score:       80,
		MaxScore:    100,
		Status:      "pass",
		Message:     "Password policy is configured correctly",
		Suggestions: []string{"Consider increasing minimum length"},
	}

	assert.Equal(t, "SEC-001", item.RuleID)
	assert.Equal(t, "Password Policy", item.Name)
	assert.Equal(t, float64(80), item.Score)
	assert.Equal(t, "pass", item.Status)
}

func TestScoreReport_Fields(t *testing.T) {
	now := time.Now()
	report := &ScoreReport{
		OverallScore: 85.5,
		Grade:        "B",
		Categories:   make(map[string]*CategoryScore),
		Strengths:    []string{"Strong authentication", "Good firewall rules"},
		Weaknesses:   []string{"MFA not fully enabled"},
		Recommendations: []Recommendation{
			{
				Priority:    "high",
				Category:    "authentication",
				Title:       "Enable MFA",
				Description: "Enable multi-factor authentication for all admin accounts",
				Impact:      "Significantly reduces risk of unauthorized access",
			},
		},
		GeneratedAt: now,
		ValidUntil:  now.Add(time.Hour),
	}

	assert.Equal(t, 85.5, report.OverallScore)
	assert.Equal(t, "B", report.Grade)
	assert.Len(t, report.Strengths, 2)
	assert.Len(t, report.Weaknesses, 1)
	assert.Len(t, report.Recommendations, 1)
}

func TestRecommendation_Fields(t *testing.T) {
	rec := Recommendation{
		Priority:    "high",
		Category:    "security",
		Title:       "Update Firewall Rules",
		Description: "Review and update firewall rules to block unnecessary ports",
		Impact:      "Reduces attack surface",
	}

	assert.Equal(t, "high", rec.Priority)
	assert.Equal(t, "security", rec.Category)
	assert.NotEmpty(t, rec.Title)
	assert.NotEmpty(t, rec.Description)
}

func TestScoringRule_Fields(t *testing.T) {
	rule := ScoringRule{
		ID:           "RULE-001",
		Category:     "authentication",
		Name:         "MFA Enabled",
		Description:  "Check if MFA is enabled for all users",
		Weight:       0.3,
		MaxDeduction: 20,
	}

	assert.Equal(t, "RULE-001", rule.ID)
	assert.Equal(t, "authentication", rule.Category)
	assert.Equal(t, float64(0.3), rule.Weight)
	assert.Equal(t, float64(20), rule.MaxDeduction)
}

func TestScoreEngine_ConcurrentCalculate(t *testing.T) {
	engine := NewScoreEngine(ScoreConfig{})
	am := NewAuditManager()
	am.SetConfig(AuditConfig{
		Enabled:      true,
		MaxLogs:      1000,
		AutoSave:     false,
		AlertEnabled: false,
	})
	bm := NewBaselineManager()

	done := make(chan bool)

	// 并发计算评分
	for i := 0; i < 5; i++ {
		go func() {
			report := engine.CalculateScore(am, bm)
			assert.NotNil(t, report)
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}
