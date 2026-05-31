// Package homefitness 家庭健身追踪模块
package homefitness

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Manager 健身管理器
type Manager struct {
	mu           sync.RWMutex
	exercises    map[string]*Exercise
	plans        map[string]*TrainingPlan
	metrics      map[string]*HealthMetric
	goals        map[string]*Goal
	achievements map[string]*Achievement
	streaks      map[string]*Streak
	logger       Logger
	ctx          context.Context
	cancel       context.CancelFunc
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewManager 创建健身管理器
func NewManager(logger Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		exercises:    make(map[string]*Exercise),
		plans:        make(map[string]*TrainingPlan),
		metrics:      make(map[string]*HealthMetric),
		goals:        make(map[string]*Goal),
		achievements: make(map[string]*Achievement),
		streaks:      make(map[string]*Streak),
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
	}

	// 初始化默认成就
	m.initDefaultAchievements()

	return m
}

// initDefaultAchievements 初始化默认成就
func (m *Manager) initDefaultAchievements() {
	defaults := []Achievement{
		{
			ID:          "first_workout",
			Type:        AchievementTypeMilestone,
			Name:        "初次锻炼",
			Description: "完成第一次运动记录",
			Icon:        "🎯",
			Criteria:    AchievementCriteria{Type: "exercises", Target: 1},
		},
		{
			ID:          "workout_10",
			Type:        AchievementTypeMilestone,
			Name:        "运动达人",
			Description: "累计完成10次运动",
			Icon:        "💪",
			Criteria:    AchievementCriteria{Type: "exercises", Target: 10},
		},
		{
			ID:          "workout_100",
			Type:        AchievementTypeMilestone,
			Name:        "健身狂人",
			Description: "累计完成100次运动",
			Icon:        "🏆",
			Criteria:    AchievementCriteria{Type: "exercises", Target: 100},
		},
		{
			ID:          "streak_7",
			Type:        AchievementTypeStreak,
			Name:        "一周坚持",
			Description: "连续打卡7天",
			Icon:        "🔥",
			Criteria:    AchievementCriteria{Type: "streak", Target: 7},
		},
		{
			ID:          "streak_30",
			Type:        AchievementTypeStreak,
			Name:        "月度坚持",
			Description: "连续打卡30天",
			Icon:        "⭐",
			Criteria:    AchievementCriteria{Type: "streak", Target: 30},
		},
		{
			ID:          "calories_10000",
			Type:        AchievementTypeTotal,
			Name:        "万卡燃烧",
			Description: "累计消耗10000卡路里",
			Icon:        "🔥",
			Criteria:    AchievementCriteria{Type: "calories", Target: 10000},
		},
		{
			ID:          "running_100km",
			Type:        AchievementTypeTotal,
			Name:        "百公里跑者",
			Description: "累计跑步100公里",
			Icon:        "🏃",
			Criteria:    AchievementCriteria{Type: "distance", Target: 100, ExerciseType: ExerciseTypeRunning},
		},
	}

	for i := range defaults {
		defaults[i].CreatedAt = time.Now()
		m.achievements[defaults[i].ID] = &defaults[i]
	}
}

// ========== 运动记录 ==========

// RecordExercise 记录运动
func (m *Manager) RecordExercise(exercise *Exercise) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if exercise.ID == "" {
		exercise.ID = generateID("exercise")
	}
	exercise.CreatedAt = time.Now()

	m.exercises[exercise.ID] = exercise

	// 更新打卡记录
	m.updateStreak(exercise.UserID, exercise.EndedAt)

	// 检查成就
	m.checkAchievements(exercise.UserID)

	m.logger.Info("运动记录已保存: %s - %s (%d分钟, %d卡路里)",
		exercise.Type, exercise.Name, exercise.Duration, exercise.Calories)
	return nil
}

// GetExercise 获取运动记录
func (m *Manager) GetExercise(id string) (*Exercise, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exercise, ok := m.exercises[id]
	if !ok {
		return nil, fmt.Errorf("运动记录不存在: %s", id)
	}
	return exercise, nil
}

// ListExercises 列出运动记录
func (m *Manager) ListExercises(userID string, limit int) []*Exercise {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exercises := make([]*Exercise, 0)
	for _, e := range m.exercises {
		if userID == "" || e.UserID == userID {
			exercises = append(exercises, e)
		}
	}

	// 按时间倒序排序
	sort.Slice(exercises, func(i, j int) bool {
		return exercises[i].StartedAt.After(exercises[j].StartedAt)
	})

	if limit > 0 && len(exercises) > limit {
		exercises = exercises[:limit]
	}
	return exercises
}

// DeleteExercise 删除运动记录
func (m *Manager) DeleteExercise(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.exercises[id]; !ok {
		return fmt.Errorf("运动记录不存在: %s", id)
	}
	delete(m.exercises, id)
	return nil
}

// ========== 训练计划 ==========

// CreatePlan 创建训练计划
func (m *Manager) CreatePlan(plan *TrainingPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if plan.ID == "" {
		plan.ID = generateID("plan")
	}
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()

	m.plans[plan.ID] = plan
	m.logger.Info("训练计划已创建: %s", plan.Name)
	return nil
}

// GetPlan 获取训练计划
func (m *Manager) GetPlan(id string) (*TrainingPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil, fmt.Errorf("训练计划不存在: %s", id)
	}
	return plan, nil
}

// ListPlans 列出训练计划
func (m *Manager) ListPlans(userID string) []*TrainingPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plans := make([]*TrainingPlan, 0)
	for _, p := range m.plans {
		if userID == "" || p.UserID == userID {
			plans = append(plans, p)
		}
	}
	return plans
}

// UpdatePlan 更新训练计划
func (m *Manager) UpdatePlan(plan *TrainingPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.plans[plan.ID]
	if !ok {
		return fmt.Errorf("训练计划不存在: %s", plan.ID)
	}

	plan.CreatedAt = existing.CreatedAt
	plan.UpdatedAt = time.Now()
	m.plans[plan.ID] = plan
	return nil
}

// DeletePlan 删除训练计划
func (m *Manager) DeletePlan(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.plans[id]; !ok {
		return fmt.Errorf("训练计划不存在: %s", id)
	}
	delete(m.plans, id)
	return nil
}

// ========== 健康指标 ==========

// RecordMetric 记录健康指标
func (m *Manager) RecordMetric(metric *HealthMetric) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if metric.ID == "" {
		metric.ID = generateID("metric")
	}
	metric.CreatedAt = time.Now()

	// 自动计算BMI
	if metric.Height > 0 {
		heightM := metric.Height / 100
		metric.BMI = math.Round(metric.Weight/(heightM*heightM)*10) / 10
	}

	m.metrics[metric.ID] = metric
	m.logger.Info("健康指标已记录: 体重%.1fkg, BMI%.1f", metric.Weight, metric.BMI)
	return nil
}

// GetLatestMetric 获取最新健康指标
func (m *Manager) GetLatestMetric(userID string) *HealthMetric {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latest *HealthMetric
	for _, metric := range m.metrics {
		if metric.UserID == userID {
			if latest == nil || metric.RecordedAt.After(latest.RecordedAt) {
				latest = metric
			}
		}
	}
	return latest
}

// GetMetricTrend 获取健康指标趋势
func (m *Manager) GetMetricTrend(userID, metricType string, days int) []TrendData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	startDate := time.Now().AddDate(0, 0, -days)
	dataMap := make(map[string]float64)

	for _, metric := range m.metrics {
		if metric.UserID == userID && metric.RecordedAt.After(startDate) {
			date := metric.RecordedAt.Format("2006-01-02")
			switch metricType {
			case "weight":
				dataMap[date] = metric.Weight
			case "bmi":
				dataMap[date] = metric.BMI
			case "body_fat":
				dataMap[date] = metric.BodyFat
			case "heart_rate":
				dataMap[date] = float64(metric.HeartRate)
			}
		}
	}

	trends := make([]TrendData, 0, len(dataMap))
	for date, value := range dataMap {
		trends = append(trends, TrendData{Date: date, Value: value})
	}

	sort.Slice(trends, func(i, j int) bool {
		return trends[i].Date < trends[j].Date
	})

	return trends
}

// ========== 目标设定 ==========

// CreateGoal 创建目标
func (m *Manager) CreateGoal(goal *Goal) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if goal.ID == "" {
		goal.ID = generateID("goal")
	}
	goal.Status = GoalStatusActive
	goal.Progress = 0
	goal.CreatedAt = time.Now()
	goal.UpdatedAt = time.Now()

	m.goals[goal.ID] = goal
	m.logger.Info("目标已创建: %s - %s", goal.Type, goal.Name)
	return nil
}

// UpdateGoalProgress 更新目标进度
func (m *Manager) UpdateGoalProgress(goalID string, current float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	goal, ok := m.goals[goalID]
	if !ok {
		return fmt.Errorf("目标不存在: %s", goalID)
	}

	goal.Current = current
	if goal.Target > 0 {
		goal.Progress = math.Min(current/goal.Target*100, 100)
	}

	// 检查是否完成
	if goal.Progress >= 100 {
		goal.Status = GoalStatusCompleted
	}

	goal.UpdatedAt = time.Now()
	return nil
}

// ListGoals 列出目标
func (m *Manager) ListGoals(userID string, status GoalStatus) []*Goal {
	m.mu.RLock()
	defer m.mu.RUnlock()

	goals := make([]*Goal, 0)
	for _, g := range m.goals {
		if (userID == "" || g.UserID == userID) &&
			(status == "" || g.Status == status) {
			goals = append(goals, g)
		}
	}
	return goals
}

// DeleteGoal 删除目标
func (m *Manager) DeleteGoal(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.goals[id]; !ok {
		return fmt.Errorf("目标不存在: %s", id)
	}
	delete(m.goals, id)
	return nil
}

// ========== 成就系统 ==========

// GetAchievements 获取成就列表
func (m *Manager) GetAchievements(userID string) []*Achievement {
	m.mu.RLock()
	defer m.mu.RUnlock()

	achievements := make([]*Achievement, 0)
	for _, a := range m.achievements {
		if userID == "" || a.UserID == userID || a.UserID == "" {
			achievements = append(achievements, a)
		}
	}
	return achievements
}

// GetUnlockedAchievements 获取已解锁成就
func (m *Manager) GetUnlockedAchievements(userID string) []*Achievement {
	m.mu.RLock()
	defer m.mu.RUnlock()

	achievements := make([]*Achievement, 0)
	for _, a := range m.achievements {
		if a.IsUnlocked && (userID == "" || a.UserID == userID) {
			achievements = append(achievements, a)
		}
	}
	return achievements
}

// checkAchievements 检查并解锁成就
func (m *Manager) checkAchievements(userID string) {
	// 统计用户数据
	totalExercises := 0
	totalCalories := 0
	totalDistance := make(map[ExerciseType]float64)

	for _, e := range m.exercises {
		if e.UserID == userID {
			totalExercises++
			totalCalories += e.Calories
			totalDistance[e.Type] += e.Distance
		}
	}

	streak := m.streaks[userID]
	currentStreak := 0
	if streak != nil {
		currentStreak = streak.Current
	}

	// 检查每个成就
	for _, a := range m.achievements {
		if a.IsUnlocked {
			continue
		}

		unlocked := false
		switch a.Criteria.Type {
		case "exercises":
			unlocked = float64(totalExercises) >= a.Criteria.Target
		case "calories":
			unlocked = float64(totalCalories) >= a.Criteria.Target
		case "streak":
			unlocked = float64(currentStreak) >= a.Criteria.Target
		case "distance":
			if a.Criteria.ExerciseType != "" {
				unlocked = totalDistance[a.Criteria.ExerciseType] >= a.Criteria.Target
			}
		}

		if unlocked {
			now := time.Now()
			a.IsUnlocked = true
			a.UnlockedAt = &now
			a.UserID = userID
			m.logger.Info("成就已解锁: %s - %s", a.Name, a.Description)
		}
	}
}

// ========== 打卡系统 ==========

// GetStreak 获取打卡记录
func (m *Manager) GetStreak(userID string) *Streak {
	m.mu.RLock()
	defer m.mu.RUnlock()

	streak, ok := m.streaks[userID]
	if !ok {
		return &Streak{Current: 0, Longest: 0, TotalDays: 0}
	}
	return streak
}

// updateStreak 更新打卡记录
func (m *Manager) updateStreak(userID string, exerciseDate time.Time) {
	streak, ok := m.streaks[userID]
	if !ok {
		streak = &Streak{}
		m.streaks[userID] = streak
	}

	today := time.Now().Format("2006-01-02")
	exerciseDay := exerciseDate.Format("2006-01-02")

	if streak.LastDate != nil {
		lastDay := streak.LastDate.Format("2006-01-02")
		if lastDay == exerciseDay {
			// 同一天，不更新
			return
		}

		// 检查是否连续
		yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		if lastDay == yesterday || lastDay == today {
			streak.Current++
		} else {
			streak.Current = 1
		}
	} else {
		streak.Current = 1
	}

	if streak.Current > streak.Longest {
		streak.Longest = streak.Current
	}

	streak.TotalDays++
	now := time.Now()
	streak.LastDate = &now
}

// ========== 排行榜 ==========

// GetLeaderboard 获取排行榜
func (m *Manager) GetLeaderboard(limit int) []LeaderboardEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 统计每个用户的数据
	userStats := make(map[string]*LeaderboardEntry)

	for _, e := range m.exercises {
		entry, ok := userStats[e.UserID]
		if !ok {
			entry = &LeaderboardEntry{
				UserID:   e.UserID,
				Username: e.UserID, // 简化：使用UserID作为用户名
			}
			userStats[e.UserID] = entry
		}
		entry.Exercises++
		entry.Score += e.Calories // 使用卡路里作为积分
	}

	// 添加打卡数据
	for userID, streak := range m.streaks {
		if entry, ok := userStats[userID]; ok {
			entry.Streak = streak.Current
		}
	}

	// 转换为切片并排序
	entries := make([]LeaderboardEntry, 0, len(userStats))
	for _, entry := range userStats {
		entries = append(entries, *entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Score > entries[j].Score
	})

	// 设置排名
	for i := range entries {
		entries[i].Rank = i + 1
	}

	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	return entries
}

// ========== 统计 ==========

// GetDailyStats 获取每日统计
func (m *Manager) GetDailyStats(userID string, date string) *DailyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &DailyStats{Date: date}

	for _, e := range m.exercises {
		if (userID == "" || e.UserID == userID) &&
			e.StartedAt.Format("2006-01-02") == date {
			stats.Exercises++
			stats.TotalMinutes += e.Duration
			stats.Calories += e.Calories
			stats.Distance += e.Distance
		}
	}

	return stats
}

// GetWeeklyStats 获取每周统计
func (m *Manager) GetWeeklyStats(userID string, weekStart string) *WeeklyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	start, err := time.Parse("2006-01-02", weekStart)
	if err != nil {
		return nil
	}
	end := start.AddDate(0, 0, 7)

	stats := &WeeklyStats{
		WeekStart: weekStart,
		WeekEnd:   end.Format("2006-01-02"),
	}

	workoutDays := make(map[string]bool)
	totalHeartRate := 0
	heartRateCount := 0

	for _, e := range m.exercises {
		if (userID == "" || e.UserID == userID) &&
			e.StartedAt.After(start) && e.StartedAt.Before(end) {
			stats.Exercises++
			stats.TotalMinutes += e.Duration
			stats.Calories += e.Calories
			stats.Distance += e.Distance
			workoutDays[e.StartedAt.Format("2006-01-02")] = true

			if e.HeartRate != nil {
				totalHeartRate += e.HeartRate.Average
				heartRateCount++
			}
		}
	}

	stats.WorkoutDays = len(workoutDays)
	if heartRateCount > 0 {
		stats.AvgHeartRate = totalHeartRate / heartRateCount
	}

	return stats
}

// GetMonthlyStats 获取每月统计
func (m *Manager) GetMonthlyStats(userID string, month string) *MonthlyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &MonthlyStats{Month: month}

	workoutDays := make(map[string]bool)

	for _, e := range m.exercises {
		if (userID == "" || e.UserID == userID) &&
			e.StartedAt.Format("2006-01") == month {
			stats.Exercises++
			stats.TotalMinutes += e.Duration
			stats.Calories += e.Calories
			stats.Distance += e.Distance
			workoutDays[e.StartedAt.Format("2006-01-02")] = true
		}
	}

	stats.WorkoutDays = len(workoutDays)

	// 获取打卡记录
	if streak, ok := m.streaks[userID]; ok {
		stats.Streak = streak.Current
	}

	return stats
}

// ========== HTTP API ==========

// RegisterRoutes 注册 HTTP 路由
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	// 运动记录
	mux.HandleFunc("/api/fitness/exercises", m.handleExercises)
	mux.HandleFunc("/api/fitness/exercises/", m.handleExerciseByID)

	// 训练计划
	mux.HandleFunc("/api/fitness/plans", m.handlePlans)
	mux.HandleFunc("/api/fitness/plans/", m.handlePlanByID)

	// 健康指标
	mux.HandleFunc("/api/fitness/metrics", m.handleMetrics)
	mux.HandleFunc("/api/fitness/metrics/trend", m.handleMetricTrend)

	// 目标
	mux.HandleFunc("/api/fitness/goals", m.handleGoals)
	mux.HandleFunc("/api/fitness/goals/", m.handleGoalByID)

	// 成就
	mux.HandleFunc("/api/fitness/achievements", m.handleAchievements)

	// 打卡
	mux.HandleFunc("/api/fitness/streak", m.handleStreak)

	// 排行榜
	mux.HandleFunc("/api/fitness/leaderboard", m.handleLeaderboard)

	// 统计
	mux.HandleFunc("/api/fitness/stats/daily", m.handleDailyStats)
	mux.HandleFunc("/api/fitness/stats/weekly", m.handleWeeklyStats)
	mux.HandleFunc("/api/fitness/stats/monthly", m.handleMonthlyStats)
}

// handleExercises 处理运动记录
func (m *Manager) handleExercises(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		limit := 50
		exercises := m.ListExercises(userID, limit)
		writeJSON(w, exercises)
	case http.MethodPost:
		var exercise Exercise
		if err := json.NewDecoder(r.Body).Decode(&exercise); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.RecordExercise(&exercise); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, exercise)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleExerciseByID 处理单个运动记录
func (m *Manager) handleExerciseByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/fitness/exercises/"):]
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		exercise, err := m.GetExercise(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, exercise)
	case http.MethodDelete:
		if err := m.DeleteExercise(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlans 处理训练计划
func (m *Manager) handlePlans(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		plans := m.ListPlans(userID)
		writeJSON(w, plans)
	case http.MethodPost:
		var plan TrainingPlan
		if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreatePlan(&plan); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, plan)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePlanByID 处理单个训练计划
func (m *Manager) handlePlanByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/fitness/plans/"):]
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		plan, err := m.GetPlan(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, plan)
	case http.MethodPut:
		var plan TrainingPlan
		if err := json.NewDecoder(r.Body).Decode(&plan); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		plan.ID = id
		if err := m.UpdatePlan(&plan); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, plan)
	case http.MethodDelete:
		if err := m.DeletePlan(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMetrics 处理健康指标
func (m *Manager) handleMetrics(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		metric := m.GetLatestMetric(userID)
		if metric == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, metric)
	case http.MethodPost:
		var metric HealthMetric
		if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.RecordMetric(&metric); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, metric)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleMetricTrend 处理健康指标趋势
func (m *Manager) handleMetricTrend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	metricType := r.URL.Query().Get("type")
	days := 30

	trends := m.GetMetricTrend(userID, metricType, days)
	writeJSON(w, trends)
}

// handleGoals 处理目标
func (m *Manager) handleGoals(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		status := GoalStatus(r.URL.Query().Get("status"))
		goals := m.ListGoals(userID, status)
		writeJSON(w, goals)
	case http.MethodPost:
		var goal Goal
		if err := json.NewDecoder(r.Body).Decode(&goal); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateGoal(&goal); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, goal)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGoalByID 处理单个目标
func (m *Manager) handleGoalByID(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/api/fitness/goals/"):]
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		var req struct {
			Current float64 `json:"current"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.UpdateGoalProgress(id, req.Current); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	case http.MethodDelete:
		if err := m.DeleteGoal(id); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAchievements 处理成就
func (m *Manager) handleAchievements(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	unlocked := r.URL.Query().Get("unlocked") == "true"

	var achievements []*Achievement
	if unlocked {
		achievements = m.GetUnlockedAchievements(userID)
	} else {
		achievements = m.GetAchievements(userID)
	}
	writeJSON(w, achievements)
}

// handleStreak 处理打卡记录
func (m *Manager) handleStreak(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	streak := m.GetStreak(userID)
	writeJSON(w, streak)
}

// handleLeaderboard 处理排行榜
func (m *Manager) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	leaderboard := m.GetLeaderboard(10)
	writeJSON(w, leaderboard)
}

// handleDailyStats 处理每日统计
func (m *Manager) handleDailyStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	stats := m.GetDailyStats(userID, date)
	writeJSON(w, stats)
}

// handleWeeklyStats 处理每周统计
func (m *Manager) handleWeeklyStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	weekStart := r.URL.Query().Get("week_start")
	if weekStart == "" {
		// 获取本周一
		now := time.Now()
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		weekStart = now.AddDate(0, 0, -weekday+1).Format("2006-01-02")
	}

	stats := m.GetWeeklyStats(userID, weekStart)
	writeJSON(w, stats)
}

// handleMonthlyStats 处理每月统计
func (m *Manager) handleMonthlyStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.URL.Query().Get("user_id")
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	stats := m.GetMonthlyStats(userID, month)
	writeJSON(w, stats)
}

// ========== 工具函数 ==========

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
