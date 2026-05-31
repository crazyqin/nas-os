// Package homefitness 家庭健身追踪模块
package homefitness

import "time"

// ExerciseType 运动类型
type ExerciseType string

const (
	ExerciseTypeRunning   ExerciseType = "running"   // 跑步
	ExerciseTypeCycling   ExerciseType = "cycling"   // 骑行
	ExerciseTypeStrength  ExerciseType = "strength"  // 力量训练
	ExerciseTypeYoga      ExerciseType = "yoga"      // 瑜伽
	ExerciseTypeSwimming  ExerciseType = "swimming"  // 游泳
	ExerciseTypeHIIT      ExerciseType = "hiit"      // HIIT
	ExerciseTypeWalking   ExerciseType = "walking"   // 步行
	ExerciseTypeStretch   ExerciseType = "stretch"   // 拉伸
	ExerciseTypeCustom    ExerciseType = "custom"    // 自定义
)

// GoalType 目标类型
type GoalType string

const (
	GoalTypeWeightLoss  GoalType = "weight_loss"  // 减重
	GoalTypeMuscleGain  GoalType = "muscle_gain"  // 增肌
	GoalTypeEndurance   GoalType = "endurance"    // 耐力
	GoalTypeFlexibility GoalType = "flexibility"  // 柔韧性
	GoalTypeCustom      GoalType = "custom"       // 自定义
)

// GoalStatus 目标状态
type GoalStatus string

const (
	GoalStatusActive    GoalStatus = "active"     // 进行中
	GoalStatusCompleted GoalStatus = "completed"  // 已完成
	GoalStatusPaused    GoalStatus = "paused"     // 暂停
	GoalStatusFailed    GoalStatus = "failed"     // 已失败
)

// Difficulty 难度等级
type Difficulty string

const (
	DifficultyEasy   Difficulty = "easy"   // 简单
	DifficultyMedium Difficulty = "medium" // 中等
	DifficultyHard   Difficulty = "hard"   // 困难
	DifficultyExpert Difficulty = "expert" // 专家
)

// AchievementType 成就类型
type AchievementType string

const (
	AchievementTypeMilestone  AchievementType = "milestone"  // 里程碑
	AchievementTypeStreak     AchievementType = "streak"     // 连续打卡
	AchievementTypeTotal      AchievementType = "total"      // 总量
	AchievementTypePersonal   AchievementType = "personal"   // 个人最佳
)

// Exercise 运动记录
type Exercise struct {
	ID          string       `json:"id"`
	UserID      string       `json:"user_id"`
	Type        ExerciseType `json:"type"`
	Name        string       `json:"name"`
	Duration    int          `json:"duration"`     // 分钟
	Calories    int          `json:"calories"`     // 卡路里
	Distance    float64      `json:"distance"`     // 公里（跑步/骑行）
	Sets        int          `json:"sets"`         // 组数（力量训练）
	Reps        int          `json:"reps"`         // 次数（力量训练）
	Weight      float64      `json:"weight"`       // 重量kg（力量训练）
	HeartRate   *HeartRate   `json:"heart_rate,omitempty"`
	Notes       string       `json:"notes,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	StartedAt   time.Time    `json:"started_at"`
	EndedAt     time.Time    `json:"ended_at"`
	CreatedAt   time.Time    `json:"created_at"`
}

// HeartRate 心率数据
type HeartRate struct {
	Average int `json:"average"` // 平均心率
	Max     int `json:"max"`     // 最大心率
	Min     int `json:"min"`     // 最小心率
}

// TrainingPlan 训练计划
type TrainingPlan struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Difficulty  Difficulty     `json:"difficulty"`
	Duration    int            `json:"duration"`     // 计划周期（天）
	Frequency   int            `json:"frequency"`    // 每周次数
	Workouts    []Workout      `json:"workouts"`
	IsActive    bool           `json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Workout 训练单元
type Workout struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	DayOfWeek   int          `json:"day_of_week"` // 0=周日, 1=周一, ...
	Exercises   []PlanExercise `json:"exercises"`
	Duration    int          `json:"duration"` // 预计时长（分钟）
}

// PlanExercise 计划中的运动
type PlanExercise struct {
	Type     ExerciseType `json:"type"`
	Name     string       `json:"name"`
	Duration int          `json:"duration"` // 分钟
	Sets     int          `json:"sets,omitempty"`
	Reps     int          `json:"reps,omitempty"`
	Weight   float64      `json:"weight,omitempty"`
	RestTime int          `json:"rest_time"` // 休息时间（秒）
}

// HealthMetric 健康指标
type HealthMetric struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Weight    float64   `json:"weight"`     // 体重kg
	Height    float64   `json:"height"`     // 身高cm
	BMI       float64   `json:"bmi"`        // BMI
	BodyFat   float64   `json:"body_fat"`   // 体脂率%
	HeartRate int       `json:"heart_rate"` // 静息心率
	Waist     float64   `json:"waist"`      // 腰围cm
	Hips      float64   `json:"hips"`       // 臀围cm
	RecordedAt time.Time `json:"recorded_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// Goal 健身目标
type Goal struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Type        GoalType   `json:"type"`
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Target      float64    `json:"target"`       // 目标值
	Current     float64    `json:"current"`      // 当前值
	Unit        string     `json:"unit"`         // 单位
	Deadline    *time.Time `json:"deadline,omitempty"`
	Status      GoalStatus `json:"status"`
	Progress    float64    `json:"progress"`     // 进度百分比
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Achievement 成就
type Achievement struct {
	ID          string          `json:"id"`
	UserID      string          `json:"user_id"`
	Type        AchievementType `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Icon        string          `json:"icon"`
	Criteria    AchievementCriteria `json:"criteria"`
	UnlockedAt  *time.Time      `json:"unlocked_at,omitempty"`
	IsUnlocked  bool            `json:"is_unlocked"`
	CreatedAt   time.Time       `json:"created_at"`
}

// AchievementCriteria 成就条件
type AchievementCriteria struct {
	Type       string  `json:"type"`       // exercises, calories, streak, distance
	Target     float64 `json:"target"`
	ExerciseType ExerciseType `json:"exercise_type,omitempty"`
}

// Streak 连续打卡记录
type Streak struct {
	Current    int        `json:"current"`     // 当前连续天数
	Longest    int        `json:"longest"`     // 最长连续天数
	LastDate   *time.Time `json:"last_date,omitempty"`
	TotalDays  int        `json:"total_days"`  // 总打卡天数
}

// DailyStats 每日统计
type DailyStats struct {
	Date        string `json:"date"`         // YYYY-MM-DD
	Exercises   int    `json:"exercises"`    // 运动次数
	TotalMinutes int   `json:"total_minutes"` // 总时长
	Calories    int    `json:"calories"`     // 总卡路里
	Distance    float64 `json:"distance"`    // 总距离
}

// WeeklyStats 每周统计
type WeeklyStats struct {
	WeekStart   string `json:"week_start"`   // YYYY-MM-DD
	WeekEnd     string `json:"week_end"`
	Exercises   int    `json:"exercises"`
	TotalMinutes int   `json:"total_minutes"`
	Calories    int    `json:"calories"`
	Distance    float64 `json:"distance"`
	AvgHeartRate int   `json:"avg_heart_rate"`
	WorkoutDays int    `json:"workout_days"`
}

// MonthlyStats 每月统计
type MonthlyStats struct {
	Month       string `json:"month"`        // YYYY-MM
	Exercises   int    `json:"exercises"`
	TotalMinutes int   `json:"total_minutes"`
	Calories    int    `json:"calories"`
	Distance    float64 `json:"distance"`
	WorkoutDays int    `json:"workout_days"`
	Streak      int    `json:"streak"`
}

// TrendData 趋势数据点
type TrendData struct {
	Date  string  `json:"date"`
	Value float64 `json:"value"`
}

// LeaderboardEntry 排行榜条目
type LeaderboardEntry struct {
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Score     int    `json:"score"`      // 积分
	Exercises int    `json:"exercises"`  // 运动次数
	Streak    int    `json:"streak"`     // 连续天数
	Rank      int    `json:"rank"`
}
