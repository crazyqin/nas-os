// Package familyactivityhub 提供家庭活动中心
// 成员活动记录、任务分配跟踪、成就奖励系统、家庭统计报表、活动日历提醒、家庭公告板
package familyactivityhub

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

// ========== 常量 ==========

const (
	// Version 模块版本.
	Version = "1.0.0"

	// MaxMembers 最大成员数.
	MaxMembers = 50

	// MaxTasks 最大任务数.
	MaxTasks = 1000

	// MaxAnnouncements 最大公告数.
	MaxAnnouncements = 500

	// MaxActivities 最大活动记录数.
	MaxActivities = 5000

	// MaxAchievements 最大成就数.
	MaxAchievements = 200

	// DefaultReminderMinutes 默认提前提醒时间（分钟）.
	DefaultReminderMinutes = 30
)

// ========== 活动类型 ==========

// ActivityType 活动类型.
type ActivityType string

const (
	ActivityTypeWatching ActivityType = "watching" // 观影
	ActivityTypeGaming   ActivityType = "gaming"   // 游戏
	ActivityTypeLearning ActivityType = "learning" // 学习
	ActivityTypeExercise ActivityType = "exercise" // 运动
	ActivityTypeCooking  ActivityType = "cooking"  // 烹饪
	ActivityTypeCrafting ActivityType = "crafting" // 手工
	ActivityTypeReading  ActivityType = "reading"  // 阅读
	ActivityTypeTravel   ActivityType = "travel"   // 旅行
	ActivityTypeMusic    ActivityType = "music"    // 音乐
	ActivityTypeOther    ActivityType = "other"    // 其他
)

// ========== 任务状态 ==========

// TaskStatus 任务状态.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"     // 待处理
	TaskStatusInProgress TaskStatus = "in_progress" // 进行中
	TaskStatusCompleted  TaskStatus = "completed"   // 已完成
	TaskStatusCancelled  TaskStatus = "cancelled"   // 已取消
	TaskStatusOverdue    TaskStatus = "overdue"     // 已逾期
)

// ========== 任务优先级 ==========

// TaskPriority 任务优先级.
type TaskPriority string

const (
	PriorityLow    TaskPriority = "low"    // 低优先级
	PriorityMedium TaskPriority = "medium" // 中优先级
	PriorityHigh   TaskPriority = "high"   // 高优先级
	PriorityUrgent TaskPriority = "urgent" // 紧急
)

// ========== 公告类型 ==========

// AnnouncementType 公告类型.
type AnnouncementType string

const (
	AnnouncementTypeGeneral     AnnouncementType = "general"     // 一般公告
	AnnouncementTypeEvent       AnnouncementType = "event"       // 活动通知
	AnnouncementTypeReminder    AnnouncementType = "reminder"    // 提醒
	AnnouncementTypeAchievement AnnouncementType = "achievement" // 成就公告
	AnnouncementTypeEmergency   AnnouncementType = "emergency"   // 紧急通知
)

// ========== 成就等级 ==========

// AchievementLevel 成就等级.
type AchievementLevel string

const (
	LevelBronze   AchievementLevel = "bronze"   // 铜牌
	LevelSilver   AchievementLevel = "silver"   // 银牌
	LevelGold     AchievementLevel = "gold"     // 金牌
	LevelPlatinum AchievementLevel = "platinum" // 铂金
	LevelDiamond  AchievementLevel = "diamond"  // 钻石
)

// ========== 奖励类型 ==========

// RewardType 奖励类型.
type RewardType string

const (
	RewardTypePoints    RewardType = "points"    // 积分
	RewardTypeBadge     RewardType = "badge"     // 徽章
	RewardTypePrivilege RewardType = "privilege" // 权限
	RewardTypeGift      RewardType = "gift"      // 礼物
)

// ========== 数据结构 ==========

// Member 家庭成员.
type Member struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Avatar      string            `json:"avatar,omitempty"`
	Role        string            `json:"role"` // parent, child, elder, guest
	Birthday    *time.Time        `json:"birthday,omitempty"`
	JoinDate    time.Time         `json:"join_date"`
	Points      int64             `json:"points"`
	Level       int               `json:"level"`
	Badges      []string          `json:"badges,omitempty"`
	Preferences MemberPreferences `json:"preferences"`
	IsActive    bool              `json:"is_active"`
}

// MemberPreferences 成员偏好.
type MemberPreferences struct {
	FavoriteActivities []ActivityType `json:"favorite_activities,omitempty"`
	ReminderEnabled    bool           `json:"reminder_enabled"`
	ReminderMinutes    int            `json:"reminder_minutes"`
	NotificationEmail  string         `json:"notification_email,omitempty"`
}

// Activity 活动记录.
type Activity struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description,omitempty"`
	Type        ActivityType   `json:"type"`
	Members     []string       `json:"members"`
	StartTime   time.Time      `json:"start_time"`
	EndTime     *time.Time     `json:"end_time,omitempty"`
	Duration    *time.Duration `json:"duration,omitempty"`
	Location    string         `json:"location,omitempty"`
	Rating      float64        `json:"rating,omitempty"`
	Notes       string         `json:"notes,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	Photos      []string       `json:"photos,omitempty"`
	Points      int64          `json:"points"`
	CreatedBy   string         `json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Task 家庭任务.
type Task struct {
	ID          string       `json:"id"`
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority"`
	AssignedTo  []string     `json:"assigned_to"`
	CreatedBy   string       `json:"created_by"`
	DueDate     *time.Time   `json:"due_date,omitempty"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	CompletedBy string       `json:"completed_by,omitempty"`
	Category    string       `json:"category,omitempty"`
	Points      int64        `json:"points"`
	IsRecurring bool         `json:"is_recurring"`
	Recurrence  *Recurrence  `json:"recurrence,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Recurrence 重复规则.
type Recurrence struct {
	Frequency  string     `json:"frequency"`
	Interval   int        `json:"interval"`
	DayOfWeek  []int      `json:"day_of_week,omitempty"`
	DayOfMonth int        `json:"day_of_month,omitempty"`
	EndDate    *time.Time `json:"end_date,omitempty"`
}

// Achievement 成就.
type Achievement struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Icon        string              `json:"icon,omitempty"`
	Level       AchievementLevel    `json:"level"`
	Criteria    AchievementCriteria `json:"criteria"`
	Points      int64               `json:"points"`
	Rewards     []Reward            `json:"rewards,omitempty"`
	IsSecret    bool                `json:"is_secret"`
	CreatedAt   time.Time           `json:"created_at"`
}

// AchievementCriteria 成就条件.
type AchievementCriteria struct {
	Type         string        `json:"type"`
	Target       int64         `json:"target"`
	ActivityType *ActivityType `json:"activity_type,omitempty"`
	Category     string        `json:"category,omitempty"`
}

// MemberAchievement 成员已获得成就.
type MemberAchievement struct {
	MemberID      string    `json:"member_id"`
	AchievementID string    `json:"achievement_id"`
	EarnedAt      time.Time `json:"earned_at"`
	Progress      float64   `json:"progress"`
}

// Reward 奖励.
type Reward struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Type        RewardType `json:"type"`
	Value       int64      `json:"value"`
	Icon        string     `json:"icon,omitempty"`
}

// Announcement 公告.
type Announcement struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Content   string           `json:"content"`
	Type      AnnouncementType `json:"type"`
	Author    string           `json:"author"`
	IsPinned  bool             `json:"is_pinned"`
	ExpiresAt *time.Time       `json:"expires_at,omitempty"`
	Tags      []string         `json:"tags,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
}

// CalendarEvent 日历事件.
type CalendarEvent struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Description  string        `json:"description,omitempty"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time"`
	AllDay       bool          `json:"all_day"`
	Location     string        `json:"location,omitempty"`
	Members      []string      `json:"members"`
	ActivityType *ActivityType `json:"activity_type,omitempty"`
	TaskID       string        `json:"task_id,omitempty"`
	IsReminder   bool          `json:"is_reminder"`
	ReminderAt   *time.Time    `json:"reminder_at,omitempty"`
	ReminderSent bool          `json:"reminder_sent"`
	CreatedBy    string        `json:"created_by"`
	CreatedAt    time.Time     `json:"created_at"`
}

// FamilyStats 家庭统计.
type FamilyStats struct {
	Period             string               `json:"period"`
	TotalActivities    int                  `json:"total_activities"`
	TotalTasks         int                  `json:"total_tasks"`
	CompletedTasks     int                  `json:"completed_tasks"`
	TotalPoints        int64                `json:"total_points"`
	MemberStats        []MemberStats        `json:"member_stats"`
	ActivityByType     map[ActivityType]int `json:"activity_by_type"`
	TopActivities      []ActivitySummary    `json:"top_activities"`
	AchievementsEarned int                  `json:"achievements_earned"`
}

// MemberStats 成员统计.
type MemberStats struct {
	MemberID         string               `json:"member_id"`
	MemberName       string               `json:"member_name"`
	ActivityCount    int                  `json:"activity_count"`
	TaskCount        int                  `json:"task_count"`
	CompletedTasks   int                  `json:"completed_tasks"`
	TotalPoints      int64                `json:"total_points"`
	AchievementCount int                  `json:"achievement_count"`
	ActivityByType   map[ActivityType]int `json:"activity_by_type"`
}

// ActivitySummary 活动摘要.
type ActivitySummary struct {
	Type      ActivityType  `json:"type"`
	Count     int           `json:"count"`
	Duration  time.Duration `json:"duration"`
	AvgRating float64       `json:"avg_rating"`
}

// ========== 请求结构 ==========

// AddMemberRequest 添加成员请求.
type AddMemberRequest struct {
	Name               string         `json:"name"`
	Avatar             string         `json:"avatar,omitempty"`
	Role               string         `json:"role"`
	Birthday           *time.Time     `json:"birthday,omitempty"`
	FavoriteActivities []ActivityType `json:"favorite_activities,omitempty"`
	NotificationEmail  string         `json:"notification_email,omitempty"`
}

// UpdateMemberRequest 更新成员请求.
type UpdateMemberRequest struct {
	Name     string     `json:"name,omitempty"`
	Avatar   string     `json:"avatar,omitempty"`
	Role     string     `json:"role,omitempty"`
	Birthday *time.Time `json:"birthday,omitempty"`
	IsActive *bool      `json:"is_active,omitempty"`
}

// RecordActivityRequest 记录活动请求.
type RecordActivityRequest struct {
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Type        ActivityType `json:"type"`
	Members     []string     `json:"members"`
	StartTime   time.Time    `json:"start_time"`
	EndTime     *time.Time   `json:"end_time,omitempty"`
	Location    string       `json:"location,omitempty"`
	Rating      float64      `json:"rating,omitempty"`
	Notes       string       `json:"notes,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
	Photos      []string     `json:"photos,omitempty"`
	Points      int64        `json:"points,omitempty"`
	CreatedBy   string       `json:"created_by"`
}

// ActivityFilter 活动过滤器.
type ActivityFilter struct {
	MemberID  string       `json:"member_id,omitempty"`
	Type      ActivityType `json:"type,omitempty"`
	StartDate *time.Time   `json:"start_date,omitempty"`
	EndDate   *time.Time   `json:"end_date,omitempty"`
	Tags      []string     `json:"tags,omitempty"`
	MinRating float64      `json:"min_rating,omitempty"`
	Limit     int          `json:"limit,omitempty"`
}

// CreateTaskRequest 创建任务请求.
type CreateTaskRequest struct {
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	Priority    TaskPriority `json:"priority,omitempty"`
	AssignedTo  []string     `json:"assigned_to"`
	CreatedBy   string       `json:"created_by"`
	DueDate     *time.Time   `json:"due_date,omitempty"`
	Category    string       `json:"category,omitempty"`
	Points      int64        `json:"points,omitempty"`
	IsRecurring bool         `json:"is_recurring,omitempty"`
	Recurrence  *Recurrence  `json:"recurrence,omitempty"`
	Tags        []string     `json:"tags,omitempty"`
}

// TaskFilter 任务过滤器.
type TaskFilter struct {
	Status     TaskStatus   `json:"status,omitempty"`
	Priority   TaskPriority `json:"priority,omitempty"`
	AssignedTo string       `json:"assigned_to,omitempty"`
	Category   string       `json:"category,omitempty"`
	DueBefore  *time.Time   `json:"due_before,omitempty"`
	Limit      int          `json:"limit,omitempty"`
}

// CreateAnnouncementRequest 创建公告请求.
type CreateAnnouncementRequest struct {
	Title     string           `json:"title"`
	Content   string           `json:"content"`
	Type      AnnouncementType `json:"type,omitempty"`
	Author    string           `json:"author"`
	IsPinned  bool             `json:"is_pinned,omitempty"`
	ExpiresAt *time.Time       `json:"expires_at,omitempty"`
	Tags      []string         `json:"tags,omitempty"`
}

// CreateCalendarEventRequest 创建日历事件请求.
type CreateCalendarEventRequest struct {
	Title           string        `json:"title"`
	Description     string        `json:"description,omitempty"`
	StartTime       time.Time     `json:"start_time"`
	EndTime         time.Time     `json:"end_time"`
	AllDay          bool          `json:"all_day,omitempty"`
	Location        string        `json:"location,omitempty"`
	Members         []string      `json:"members"`
	ActivityType    *ActivityType `json:"activity_type,omitempty"`
	TaskID          string        `json:"task_id,omitempty"`
	IsReminder      bool          `json:"is_reminder,omitempty"`
	ReminderMinutes int           `json:"reminder_minutes,omitempty"`
	CreatedBy       string        `json:"created_by"`
}

// ========== 管理器 ==========

// Hub 家庭活动中心管理器.
type Hub struct {
	mu             sync.RWMutex
	members        map[string]*Member
	activities     []*Activity
	tasks          map[string]*Task
	achievements   map[string]*Achievement
	memberAchvs    map[string][]*MemberAchievement
	announcements  []*Announcement
	calendarEvents map[string]*CalendarEvent
	rewards        map[string]*Reward
}

// NewHub 创建家庭活动中心.
func NewHub() *Hub {
	h := &Hub{
		members:        make(map[string]*Member),
		activities:     make([]*Activity, 0, MaxActivities),
		tasks:          make(map[string]*Task),
		achievements:   make(map[string]*Achievement),
		memberAchvs:    make(map[string][]*MemberAchievement),
		announcements:  make([]*Announcement, 0, MaxAnnouncements),
		calendarEvents: make(map[string]*CalendarEvent),
		rewards:        make(map[string]*Reward),
	}
	h.initDefaultAchievements()
	h.initDefaultRewards()
	return h
}

// ========== 成员管理 ==========

// AddMember 添加家庭成员.
func (h *Hub) AddMember(req *AddMemberRequest) (*Member, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.members) >= MaxMembers {
		return nil, errors.New("已达最大成员数限制")
	}

	for _, m := range h.members {
		if m.Name == req.Name {
			return nil, errors.New("成员名称已存在")
		}
	}

	member := &Member{
		ID:       generateID(),
		Name:     req.Name,
		Avatar:   req.Avatar,
		Role:     req.Role,
		Birthday: req.Birthday,
		JoinDate: time.Now(),
		Points:   0,
		Level:    1,
		Badges:   make([]string, 0),
		Preferences: MemberPreferences{
			FavoriteActivities: req.FavoriteActivities,
			ReminderEnabled:    true,
			ReminderMinutes:    DefaultReminderMinutes,
			NotificationEmail:  req.NotificationEmail,
		},
		IsActive: true,
	}

	if member.Role == "" {
		member.Role = "member"
	}

	h.members[member.ID] = member
	return member, nil
}

// GetMember 获取成员.
func (h *Hub) GetMember(memberID string) (*Member, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	member, exists := h.members[memberID]
	if !exists {
		return nil, fmt.Errorf("成员 %s 不存在", memberID)
	}
	return member, nil
}

// UpdateMember 更新成员.
func (h *Hub) UpdateMember(memberID string, req *UpdateMemberRequest) (*Member, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	member, exists := h.members[memberID]
	if !exists {
		return nil, fmt.Errorf("成员 %s 不存在", memberID)
	}

	if req.Name != "" {
		member.Name = req.Name
	}
	if req.Avatar != "" {
		member.Avatar = req.Avatar
	}
	if req.Role != "" {
		member.Role = req.Role
	}
	if req.Birthday != nil {
		member.Birthday = req.Birthday
	}
	if req.IsActive != nil {
		member.IsActive = *req.IsActive
	}

	return member, nil
}

// RemoveMember 移除成员.
func (h *Hub) RemoveMember(memberID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, exists := h.members[memberID]; !exists {
		return fmt.Errorf("成员 %s 不存在", memberID)
	}

	delete(h.members, memberID)
	delete(h.memberAchvs, memberID)
	return nil
}

// ListMembers 列出所有成员.
func (h *Hub) ListMembers() []*Member {
	h.mu.RLock()
	defer h.mu.RUnlock()

	members := make([]*Member, 0, len(h.members))
	for _, m := range h.members {
		members = append(members, m)
	}

	sort.Slice(members, func(i, j int) bool {
		return members[i].JoinDate.Before(members[j].JoinDate)
	})

	return members
}

// ========== 活动记录 ==========

// RecordActivity 记录活动.
func (h *Hub) RecordActivity(req *RecordActivityRequest) (*Activity, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.activities) >= MaxActivities {
		h.activities = h.activities[1:]
	}

	for _, memberID := range req.Members {
		if _, exists := h.members[memberID]; !exists {
			return nil, fmt.Errorf("成员 %s 不存在", memberID)
		}
	}

	now := time.Now()
	activity := &Activity{
		ID:          generateID(),
		Title:       req.Title,
		Description: req.Description,
		Type:        req.Type,
		Members:     req.Members,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Location:    req.Location,
		Rating:      req.Rating,
		Notes:       req.Notes,
		Tags:        req.Tags,
		Photos:      req.Photos,
		Points:      req.Points,
		CreatedBy:   req.CreatedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if req.EndTime != nil {
		duration := req.EndTime.Sub(req.StartTime)
		activity.Duration = &duration
	}

	if activity.Points == 0 {
		activity.Points = 10
	}

	h.activities = append(h.activities, activity)

	for _, memberID := range req.Members {
		if member, exists := h.members[memberID]; exists {
			member.Points += activity.Points
			h.updateMemberLevel(member)
			h.checkAchievements(memberID)
		}
	}

	return activity, nil
}

// GetActivity 获取活动.
func (h *Hub) GetActivity(activityID string) (*Activity, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, a := range h.activities {
		if a.ID == activityID {
			return a, nil
		}
	}
	return nil, fmt.Errorf("活动 %s 不存在", activityID)
}

// ListActivities 列出活动.
func (h *Hub) ListActivities(filter *ActivityFilter) []*Activity {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*Activity
	for _, a := range h.activities {
		if h.matchActivityFilter(a, filter) {
			result = append(result, a)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartTime.After(result[j].StartTime)
	})

	if filter != nil && filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result
}

// ========== 任务管理 ==========

// CreateTask 创建任务.
func (h *Hub) CreateTask(req *CreateTaskRequest) (*Task, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.tasks) >= MaxTasks {
		return nil, errors.New("已达最大任务数限制")
	}

	for _, memberID := range req.AssignedTo {
		if _, exists := h.members[memberID]; !exists {
			return nil, fmt.Errorf("成员 %s 不存在", memberID)
		}
	}

	now := time.Now()
	task := &Task{
		ID:          generateID(),
		Title:       req.Title,
		Description: req.Description,
		Status:      TaskStatusPending,
		Priority:    req.Priority,
		AssignedTo:  req.AssignedTo,
		CreatedBy:   req.CreatedBy,
		DueDate:     req.DueDate,
		Category:    req.Category,
		Points:      req.Points,
		IsRecurring: req.IsRecurring,
		Recurrence:  req.Recurrence,
		Tags:        req.Tags,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if task.Points == 0 {
		task.Points = 20
	}
	if task.Priority == "" {
		task.Priority = PriorityMedium
	}

	h.tasks[task.ID] = task

	if req.DueDate != nil {
		h.createTaskReminder(task)
	}

	return task, nil
}

// UpdateTaskStatus 更新任务状态.
func (h *Hub) UpdateTaskStatus(taskID string, status TaskStatus, completedBy string) (*Task, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	task, exists := h.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}

	oldStatus := task.Status
	task.Status = status
	task.UpdatedAt = time.Now()

	if status == TaskStatusCompleted && oldStatus != TaskStatusCompleted {
		now := time.Now()
		task.CompletedAt = &now
		task.CompletedBy = completedBy

		for _, memberID := range task.AssignedTo {
			if member, exists := h.members[memberID]; exists {
				member.Points += task.Points
				h.updateMemberLevel(member)
				h.checkAchievements(memberID)
			}
		}
	}

	return task, nil
}

// GetTask 获取任务.
func (h *Hub) GetTask(taskID string) (*Task, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	task, exists := h.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}
	return task, nil
}

// ListTasks 列出任务.
func (h *Hub) ListTasks(filter *TaskFilter) []*Task {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var result []*Task
	for _, t := range h.tasks {
		if h.matchTaskFilter(t, filter) {
			result = append(result, t)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		priorityOrder := map[TaskPriority]int{
			PriorityUrgent: 0,
			PriorityHigh:   1,
			PriorityMedium: 2,
			PriorityLow:    3,
		}
		if priorityOrder[result[i].Priority] != priorityOrder[result[j].Priority] {
			return priorityOrder[result[i].Priority] < priorityOrder[result[j].Priority]
		}
		if result[i].DueDate != nil && result[j].DueDate != nil {
			return result[i].DueDate.Before(*result[j].DueDate)
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	if filter != nil && filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result
}

// ========== 成就系统 ==========

// GetAchievements 获取所有成就.
func (h *Hub) GetAchievements() []*Achievement {
	h.mu.RLock()
	defer h.mu.RUnlock()

	achievements := make([]*Achievement, 0, len(h.achievements))
	for _, a := range h.achievements {
		achievements = append(achievements, a)
	}

	sort.Slice(achievements, func(i, j int) bool {
		return achievements[i].Points > achievements[j].Points
	})

	return achievements
}

// GetMemberAchievements 获取成员成就.
func (h *Hub) GetMemberAchievements(memberID string) []*MemberAchievement {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.memberAchvs[memberID]
}

// GetMemberRewards 获取成员奖励.
func (h *Hub) GetMemberRewards(memberID string) []*Reward {
	h.mu.RLock()
	defer h.mu.RUnlock()

	member, exists := h.members[memberID]
	if !exists {
		return nil
	}

	rewards := make([]*Reward, 0)
	for _, badgeName := range member.Badges {
		for _, r := range h.rewards {
			if r.Name == badgeName {
				rewards = append(rewards, r)
			}
		}
	}

	return rewards
}

// checkAchievements 检查并授予成就.
func (h *Hub) checkAchievements(memberID string) {
	member, exists := h.members[memberID]
	if !exists {
		return
	}

	earnedAchvs := make(map[string]bool)
	for _, ma := range h.memberAchvs[memberID] {
		earnedAchvs[ma.AchievementID] = true
	}

	for _, achv := range h.achievements {
		if earnedAchvs[achv.ID] {
			continue
		}

		if h.checkAchievementCriteria(member, achv) {
			ma := &MemberAchievement{
				MemberID:      memberID,
				AchievementID: achv.ID,
				EarnedAt:      time.Now(),
				Progress:      100,
			}
			h.memberAchvs[memberID] = append(h.memberAchvs[memberID], ma)

			for _, reward := range achv.Rewards {
				h.grantReward(memberID, &reward, achv.Points)
			}

			h.createAchievementAnnouncement(member, achv)
		}
	}
}

// checkAchievementCriteria 检查成就条件.
func (h *Hub) checkAchievementCriteria(member *Member, achv *Achievement) bool {
	criteria := achv.Criteria

	switch criteria.Type {
	case "points":
		return member.Points >= criteria.Target
	case "activity_count":
		count := h.countMemberActivities(member.ID, criteria.ActivityType)
		return int64(count) >= criteria.Target
	case "task_count":
		count := h.countMemberCompletedTasks(member.ID)
		return int64(count) >= criteria.Target
	default:
		return false
	}
}

// ========== 公告板 ==========

// CreateAnnouncement 创建公告.
func (h *Hub) CreateAnnouncement(req *CreateAnnouncementRequest) (*Announcement, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.announcements) >= MaxAnnouncements {
		h.announcements = h.announcements[1:]
	}

	now := time.Now()
	announcement := &Announcement{
		ID:        generateID(),
		Title:     req.Title,
		Content:   req.Content,
		Type:      req.Type,
		Author:    req.Author,
		IsPinned:  req.IsPinned,
		ExpiresAt: req.ExpiresAt,
		Tags:      req.Tags,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if announcement.Type == "" {
		announcement.Type = AnnouncementTypeGeneral
	}

	h.announcements = append(h.announcements, announcement)
	return announcement, nil
}

// GetAnnouncements 获取公告列表.
func (h *Hub) GetAnnouncements(includeExpired bool) []*Announcement {
	h.mu.RLock()
	defer h.mu.RUnlock()

	now := time.Now()
	var result []*Announcement

	for _, a := range h.announcements {
		if !includeExpired && a.ExpiresAt != nil && a.ExpiresAt.Before(now) {
			continue
		}
		result = append(result, a)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsPinned != result[j].IsPinned {
			return result[i].IsPinned
		}
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// ========== 日历和提醒 ==========

// CreateCalendarEvent 创建日历事件.
func (h *Hub) CreateCalendarEvent(req *CreateCalendarEventRequest) (*CalendarEvent, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, memberID := range req.Members {
		if _, exists := h.members[memberID]; !exists {
			return nil, fmt.Errorf("成员 %s 不存在", memberID)
		}
	}

	now := time.Now()
	event := &CalendarEvent{
		ID:           generateID(),
		Title:        req.Title,
		Description:  req.Description,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
		AllDay:       req.AllDay,
		Location:     req.Location,
		Members:      req.Members,
		ActivityType: req.ActivityType,
		TaskID:       req.TaskID,
		IsReminder:   req.IsReminder,
		CreatedBy:    req.CreatedBy,
		CreatedAt:    now,
	}

	if req.ReminderMinutes > 0 {
		reminderAt := req.StartTime.Add(-time.Duration(req.ReminderMinutes) * time.Minute)
		event.ReminderAt = &reminderAt
	}

	h.calendarEvents[event.ID] = event
	return event, nil
}

// GetUpcomingEvents 获取即将到来的事件.
func (h *Hub) GetUpcomingEvents(memberID string, hours int) []*CalendarEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	now := time.Now()
	endTime := now.Add(time.Duration(hours) * time.Hour)

	var events []*CalendarEvent
	for _, e := range h.calendarEvents {
		if e.StartTime.After(now) && e.StartTime.Before(endTime) {
			if memberID == "" || containsString(e.Members, memberID) {
				events = append(events, e)
			}
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].StartTime.Before(events[j].StartTime)
	})

	return events
}

// GetPendingReminders 获取待发送提醒.
func (h *Hub) GetPendingReminders() []*CalendarEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()

	now := time.Now()
	var reminders []*CalendarEvent

	for _, e := range h.calendarEvents {
		if e.ReminderAt != nil && e.ReminderAt.Before(now) && !e.ReminderSent {
			reminders = append(reminders, e)
		}
	}

	return reminders
}

// MarkReminderSent 标记提醒已发送.
func (h *Hub) MarkReminderSent(eventID string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	event, exists := h.calendarEvents[eventID]
	if !exists {
		return fmt.Errorf("事件 %s 不存在", eventID)
	}

	event.ReminderSent = true
	return nil
}

// ========== 统计报表 ==========

// GetFamilyStats 获取家庭统计.
func (h *Hub) GetFamilyStats(period string, startDate, endDate time.Time) *FamilyStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := &FamilyStats{
		Period:         period,
		ActivityByType: make(map[ActivityType]int),
	}

	for _, a := range h.activities {
		if a.StartTime.After(startDate) && a.StartTime.Before(endDate) {
			stats.TotalActivities++
			stats.ActivityByType[a.Type]++
		}
	}

	for _, t := range h.tasks {
		if t.CreatedAt.After(startDate) && t.CreatedAt.Before(endDate) {
			stats.TotalTasks++
			if t.Status == TaskStatusCompleted {
				stats.CompletedTasks++
			}
		}
	}

	stats.MemberStats = make([]MemberStats, 0, len(h.members))
	for _, member := range h.members {
		ms := MemberStats{
			MemberID:       member.ID,
			MemberName:     member.Name,
			TotalPoints:    member.Points,
			ActivityByType: make(map[ActivityType]int),
		}

		for _, a := range h.activities {
			if containsString(a.Members, member.ID) && a.StartTime.After(startDate) && a.StartTime.Before(endDate) {
				ms.ActivityCount++
				ms.ActivityByType[a.Type]++
				stats.TotalPoints += a.Points
			}
		}

		for _, t := range h.tasks {
			if containsString(t.AssignedTo, member.ID) && t.CreatedAt.After(startDate) && t.CreatedAt.Before(endDate) {
				ms.TaskCount++
				if t.Status == TaskStatusCompleted {
					ms.CompletedTasks++
				}
			}
		}

		ms.AchievementCount = len(h.memberAchvs[member.ID])

		stats.MemberStats = append(stats.MemberStats, ms)
		stats.AchievementsEarned += ms.AchievementCount
	}

	stats.TopActivities = h.calculateTopActivities(startDate, endDate)

	return stats
}

// ========== 过滤匹配 ==========

// matchActivityFilter 匹配活动过滤器.
func (h *Hub) matchActivityFilter(activity *Activity, filter *ActivityFilter) bool {
	if filter == nil {
		return true
	}

	if filter.MemberID != "" && !containsString(activity.Members, filter.MemberID) {
		return false
	}

	if filter.Type != "" && activity.Type != filter.Type {
		return false
	}

	if filter.StartDate != nil && activity.StartTime.Before(*filter.StartDate) {
		return false
	}

	if filter.EndDate != nil && activity.StartTime.After(*filter.EndDate) {
		return false
	}

	if filter.MinRating > 0 && activity.Rating < filter.MinRating {
		return false
	}

	if len(filter.Tags) > 0 {
		hasTag := false
		for _, tag := range filter.Tags {
			if containsString(activity.Tags, tag) {
				hasTag = true
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	return true
}

// matchTaskFilter 匹配任务过滤器.
func (h *Hub) matchTaskFilter(task *Task, filter *TaskFilter) bool {
	if filter == nil {
		return true
	}

	if filter.Status != "" && task.Status != filter.Status {
		return false
	}

	if filter.Priority != "" && task.Priority != filter.Priority {
		return false
	}

	if filter.AssignedTo != "" && !containsString(task.AssignedTo, filter.AssignedTo) {
		return false
	}

	if filter.Category != "" && task.Category != filter.Category {
		return false
	}

	if filter.DueBefore != nil && task.DueDate != nil && task.DueDate.After(*filter.DueBefore) {
		return false
	}

	return true
}

// ========== 内部方法 ==========

// updateMemberLevel 更新成员等级.
func (h *Hub) updateMemberLevel(member *Member) {
	newLevel := int(member.Points/100) + 1
	if newLevel > member.Level {
		member.Level = newLevel
	}
}

// countMemberActivities 统计成员活动数.
func (h *Hub) countMemberActivities(memberID string, activityType *ActivityType) int {
	count := 0
	for _, a := range h.activities {
		if containsString(a.Members, memberID) {
			if activityType == nil || a.Type == *activityType {
				count++
			}
		}
	}
	return count
}

// countMemberCompletedTasks 统计成员完成任务数.
func (h *Hub) countMemberCompletedTasks(memberID string) int {
	count := 0
	for _, t := range h.tasks {
		if containsString(t.AssignedTo, memberID) && t.Status == TaskStatusCompleted {
			count++
		}
	}
	return count
}

// grantReward 授予奖励.
func (h *Hub) grantReward(memberID string, reward *Reward, points int64) {
	member, exists := h.members[memberID]
	if !exists {
		return
	}

	switch reward.Type {
	case RewardTypePoints:
		member.Points += reward.Value
	case RewardTypeBadge:
		member.Badges = append(member.Badges, reward.Name)
	case RewardTypePrivilege, RewardTypeGift:
		// 仅记录，实际授予由外部系统处理
	}

	member.Points += points
	h.updateMemberLevel(member)
}

// createAchievementAnnouncement 创建成就公告.
func (h *Hub) createAchievementAnnouncement(member *Member, achv *Achievement) {
	if len(h.announcements) >= MaxAnnouncements {
		h.announcements = h.announcements[1:]
	}

	announcement := &Announcement{
		ID:        generateID(),
		Title:     fmt.Sprintf("🏆 %s 获得了成就！", member.Name),
		Content:   fmt.Sprintf("恭喜 %s 获得了「%s」成就！\n%s\n奖励：%d 积分", member.Name, achv.Name, achv.Description, achv.Points),
		Type:      AnnouncementTypeAchievement,
		Author:    "system",
		IsPinned:  false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	h.announcements = append(h.announcements, announcement)
}

// createTaskReminder 创建任务提醒.
func (h *Hub) createTaskReminder(task *Task) {
	if task.DueDate == nil {
		return
	}

	reminderAt := task.DueDate.Add(-DefaultReminderMinutes * time.Minute)
	if reminderAt.Before(time.Now()) {
		return
	}

	event := &CalendarEvent{
		ID:          generateID(),
		Title:       fmt.Sprintf("📋 任务提醒: %s", task.Title),
		Description: task.Description,
		StartTime:   *task.DueDate,
		EndTime:     task.DueDate.Add(time.Hour),
		Members:     task.AssignedTo,
		TaskID:      task.ID,
		IsReminder:  true,
		ReminderAt:  &reminderAt,
		CreatedBy:   "system",
		CreatedAt:   time.Now(),
	}

	h.calendarEvents[event.ID] = event
}

// calculateTopActivities 计算热门活动.
func (h *Hub) calculateTopActivities(startDate, endDate time.Time) []ActivitySummary {
	typeMap := make(map[ActivityType]*ActivitySummary)

	for _, a := range h.activities {
		if a.StartTime.After(startDate) && a.StartTime.Before(endDate) {
			if _, exists := typeMap[a.Type]; !exists {
				typeMap[a.Type] = &ActivitySummary{
					Type: a.Type,
				}
			}
			summary := typeMap[a.Type]
			summary.Count++
			if a.Duration != nil {
				summary.Duration += *a.Duration
			}
			if a.Rating > 0 {
				summary.AvgRating = (summary.AvgRating*float64(summary.Count-1) + a.Rating) / float64(summary.Count)
			}
		}
	}

	result := make([]ActivitySummary, 0, len(typeMap))
	for _, s := range typeMap {
		result = append(result, *s)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	if len(result) > 10 {
		result = result[:10]
	}

	return result
}

// initDefaultAchievements 初始化默认成就.
func (h *Hub) initDefaultAchievements() {
	defaultAchvs := []*Achievement{
		{
			ID:          "first-activity",
			Name:        "初次体验",
			Description: "记录第一次家庭活动",
			Icon:        "🎬",
			Level:       LevelBronze,
			Criteria:    AchievementCriteria{Type: "activity_count", Target: 1},
			Points:      50,
			IsSecret:    false,
		},
		{
			ID:          "activity-10",
			Name:        "活跃家庭",
			Description: "累计完成10次家庭活动",
			Icon:        "🌟",
			Level:       LevelSilver,
			Criteria:    AchievementCriteria{Type: "activity_count", Target: 10},
			Points:      100,
			IsSecret:    false,
		},
		{
			ID:          "activity-50",
			Name:        "活动达人",
			Description: "累计完成50次家庭活动",
			Icon:        "🏆",
			Level:       LevelGold,
			Criteria:    AchievementCriteria{Type: "activity_count", Target: 50},
			Points:      200,
			IsSecret:    false,
		},
		{
			ID:          "first-task",
			Name:        "任务新手",
			Description: "完成第一个家庭任务",
			Icon:        "✅",
			Level:       LevelBronze,
			Criteria:    AchievementCriteria{Type: "task_count", Target: 1},
			Points:      30,
			IsSecret:    false,
		},
		{
			ID:          "task-10",
			Name:        "勤劳蜜蜂",
			Description: "累计完成10个家庭任务",
			Icon:        "🐝",
			Level:       LevelSilver,
			Criteria:    AchievementCriteria{Type: "task_count", Target: 10},
			Points:      150,
			IsSecret:    false,
		},
		{
			ID:          "points-100",
			Name:        "积分新手",
			Description: "累计获得100积分",
			Icon:        "💰",
			Level:       LevelBronze,
			Criteria:    AchievementCriteria{Type: "points", Target: 100},
			Points:      20,
			IsSecret:    false,
		},
		{
			ID:          "points-500",
			Name:        "积分高手",
			Description: "累计获得500积分",
			Icon:        "💎",
			Level:       LevelGold,
			Criteria:    AchievementCriteria{Type: "points", Target: 500},
			Points:      100,
			IsSecret:    false,
		},
		{
			ID:          "movie-lover",
			Name:        "电影爱好者",
			Description: "观看10部电影",
			Icon:        "🎥",
			Level:       LevelSilver,
			Criteria:    AchievementCriteria{Type: "activity_count", Target: 10, ActivityType: ptrActivityType(ActivityTypeWatching)},
			Points:      80,
			IsSecret:    false,
		},
		{
			ID:          "gamer",
			Name:        "游戏高手",
			Description: "进行10次游戏活动",
			Icon:        "🎮",
			Level:       LevelSilver,
			Criteria:    AchievementCriteria{Type: "activity_count", Target: 10, ActivityType: ptrActivityType(ActivityTypeGaming)},
			Points:      80,
			IsSecret:    false,
		},
		{
			ID:          "bookworm",
			Name:        "书虫",
			Description: "阅读10本书",
			Icon:        "📚",
			Level:       LevelSilver,
			Criteria:    AchievementCriteria{Type: "activity_count", Target: 10, ActivityType: ptrActivityType(ActivityTypeReading)},
			Points:      80,
			IsSecret:    false,
		},
	}

	for _, achv := range defaultAchvs {
		achv.CreatedAt = time.Now()
		h.achievements[achv.ID] = achv
	}
}

// initDefaultRewards 初始化默认奖励.
func (h *Hub) initDefaultRewards() {
	defaultRewards := []*Reward{
		{ID: "badge-newcomer", Name: "新人徽章", Description: "欢迎加入家庭活动中心", Type: RewardTypeBadge, Value: 0, Icon: "🎖️"},
		{ID: "badge-active", Name: "活跃徽章", Description: "积极参与家庭活动", Type: RewardTypeBadge, Value: 0, Icon: "⭐"},
		{ID: "badge-helper", Name: "帮手徽章", Description: "乐于助人完成任务", Type: RewardTypeBadge, Value: 0, Icon: "🤝"},
		{ID: "points-bonus-50", Name: "50积分奖励", Description: "额外获得50积分", Type: RewardTypePoints, Value: 50, Icon: "💰"},
		{ID: "points-bonus-100", Name: "100积分奖励", Description: "额外获得100积分", Type: RewardTypePoints, Value: 100, Icon: "💎"},
	}

	for _, r := range defaultRewards {
		h.rewards[r.ID] = r
	}
}

// ========== 工具函数 ==========

// generateID 生成唯一ID.
func generateID() string {
	return fmt.Sprintf("%d-%s", time.Now().UnixNano(), randomHex(8))
}

// randomHex 生成随机十六进制字符串.
func randomHex(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = "0123456789abcdef"[time.Now().UnixNano()%16]
		time.Sleep(1)
	}
	return string(b)
}

// containsString 检查字符串切片是否包含指定字符串.
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// ptrActivityType 返回ActivityType的指针.
func ptrActivityType(t ActivityType) *ActivityType {
	return &t
}
