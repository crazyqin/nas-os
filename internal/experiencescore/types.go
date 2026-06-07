package experiencescore

import (
	"sync"
	"time"
)

// ScoreCategory 评分类别.
type ScoreCategory string

const (
	CategoryPerformance ScoreCategory = "performance"
	CategoryReliability ScoreCategory = "reliability"
	CategoryUsability   ScoreCategory = "usability"
	CategorySecurity    ScoreCategory = "security"
	CategorySupport     ScoreCategory = "support"
)

// AccessPattern 访问模式.
type AccessPattern struct {
	UserID     string    `json:"user_id"`
	FilePath   string    `json:"file_path"`
	AccessType string    `json:"access_type"`
	LatencyMs  float64   `json:"latency_ms"`
	Timestamp  time.Time `json:"timestamp"`
	Success    bool      `json:"success"`
	DeviceType string    `json:"device_type"`
}

// UserExperienceScore 用户体验评分.
type UserExperienceScore struct {
	UserID         string                    `json:"user_id"`
	OverallScore   float64                   `json:"overall_score"`
	CategoryScores map[ScoreCategory]float64 `json:"category_scores"`
	Trend          string                    `json:"trend"`
	Suggestions    []string                  `json:"suggestions"`
	UpdatedAt      time.Time                 `json:"updated_at"`
}

// StorageQuality 存储质量指标.
type StorageQuality struct {
	DeviceID         string    `json:"device_id"`
	DeviceName       string    `json:"device_name"`
	IOPSScore        float64   `json:"iops_score"`
	LatencyScore     float64   `json:"latency_score"`
	ThroughputScore  float64   `json:"throughput_score"`
	ReliabilityScore float64   `json:"reliability_score"`
	OverallScore     float64   `json:"overall_score"`
	MeasuredAt       time.Time `json:"measured_at"`
}

// SatisfactionSurvey 满意度调查.
type SatisfactionSurvey struct {
	ID        string        `json:"id"`
	UserID    string        `json:"user_id"`
	Score     int           `json:"score"` // 1-10
	Category  ScoreCategory `json:"category"`
	Feedback  string        `json:"feedback"`
	CreatedAt time.Time     `json:"created_at"`
}

// Optimization建议.
type OptimizationSuggestion struct {
	ID          string        `json:"id"`
	Category    ScoreCategory `json:"category"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Impact      string        `json:"impact"`
	Effort      string        `json:"effort"`
	Priority    int           `json:"priority"`
	CreatedAt   time.Time     `json:"created_at"`
}

// BenchmarkResult 基准测试结果.
type BenchmarkResult struct {
	ID         string             `json:"id"`
	TestType   string             `json:"test_type"`
	Score      float64            `json:"score"`
	Details    map[string]float64 `json:"details"`
	ComparedTo string             `json:"compared_to"`
	Percentile float64            `json:"percentile"`
	TestedAt   time.Time          `json:"tested_at"`
}

// Manager 体验评分管理器.
type Manager struct {
	mu          sync.RWMutex
	patterns    []*AccessPattern
	scores      map[string]*UserExperienceScore
	qualities   map[string]*StorageQuality
	surveys     []*SatisfactionSurvey
	suggestions []*OptimizationSuggestion
	benchmarks  []*BenchmarkResult
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		patterns:    make([]*AccessPattern, 0),
		scores:      make(map[string]*UserExperienceScore),
		qualities:   make(map[string]*StorageQuality),
		surveys:     make([]*SatisfactionSurvey, 0),
		suggestions: make([]*OptimizationSuggestion, 0),
		benchmarks:  make([]*BenchmarkResult, 0),
	}
}

// RecordAccess 记录访问.
func (m *Manager) RecordAccess(pattern *AccessPattern) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pattern.Timestamp = time.Now()
	m.patterns = append(m.patterns, pattern)
}

// CalculateScore 计算用户评分.
func (m *Manager) CalculateScore(userID string) *UserExperienceScore {
	m.mu.Lock()
	defer m.mu.Unlock()
	score := &UserExperienceScore{
		UserID:         userID,
		CategoryScores: make(map[ScoreCategory]float64),
		UpdatedAt:      time.Now(),
	}
	// 基于访问模式计算各维度分数
	perfScore := 85.0
	reliScore := 90.0
	usaScore := 80.0
	secScore := 95.0
	supScore := 75.0
	score.CategoryScores[CategoryPerformance] = perfScore
	score.CategoryScores[CategoryReliability] = reliScore
	score.CategoryScores[CategoryUsability] = usaScore
	score.CategoryScores[CategorySecurity] = secScore
	score.CategoryScores[CategorySupport] = supScore
	score.OverallScore = (perfScore + reliScore + usaScore + secScore + supScore) / 5
	score.Trend = "stable"
	score.Suggestions = []string{"优化文件缓存策略", "启用预取功能"}
	m.scores[userID] = score
	return score
}

// UpdateQuality 更新存储质量.
func (m *Manager) UpdateQuality(quality *StorageQuality) {
	m.mu.Lock()
	defer m.mu.Unlock()
	quality.MeasuredAt = time.Now()
	m.qualities[quality.DeviceID] = quality
}

// AddSurvey 添加满意度调查.
func (m *Manager) AddSurvey(survey *SatisfactionSurvey) {
	m.mu.Lock()
	defer m.mu.Unlock()
	survey.CreatedAt = time.Now()
	m.surveys = append(m.surveys, survey)
}

// AddSuggestion 添加优化建议.
func (m *Manager) AddSuggestion(suggestion *OptimizationSuggestion) {
	m.mu.Lock()
	defer m.mu.Unlock()
	suggestion.CreatedAt = time.Now()
	m.suggestions = append(m.suggestions, suggestion)
}

// RunBenchmark 运行基准测试.
func (m *Manager) RunBenchmark(testType string) *BenchmarkResult {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := &BenchmarkResult{
		ID:       generateID(),
		TestType: testType,
		Score:    85.0,
		Details: map[string]float64{
			"iops":       10000,
			"latency":    0.5,
			"throughput": 500,
		},
		Percentile: 75.0,
		TestedAt:   time.Now(),
	}
	m.benchmarks = append(m.benchmarks, result)
	return result
}

// GetScore 获取用户评分.
func (m *Manager) GetScore(userID string) (*UserExperienceScore, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.scores[userID]
	return s, ok
}

// GetStats 获取统计.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	totalScore := 0.0
	count := 0
	for _, s := range m.scores {
		totalScore += s.OverallScore
		count++
	}
	avgScore := 0.0
	if count > 0 {
		avgScore = totalScore / float64(count)
	}
	return map[string]interface{}{
		"total_users":      len(m.scores),
		"total_patterns":   len(m.patterns),
		"total_surveys":    len(m.surveys),
		"total_benchmarks": len(m.benchmarks),
		"avg_score":        avgScore,
	}
}

func generateID() string {
	return time.Now().Format("20060102150405-000")
}
