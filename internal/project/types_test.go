package project

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== TaskStatus 测试 ==========

func TestTaskStatus_Constants(t *testing.T) {
	assert.Equal(t, TaskStatus("todo"), TaskStatusTodo)
	assert.Equal(t, TaskStatus("in_progress"), TaskStatusInProgress)
	assert.Equal(t, TaskStatus("review"), TaskStatusReview)
	assert.Equal(t, TaskStatus("done"), TaskStatusDone)
	assert.Equal(t, TaskStatus("cancelled"), TaskStatusCancelled)
}

// ========== TaskPriority 测试 ==========

func TestTaskPriority_Constants(t *testing.T) {
	assert.Equal(t, TaskPriority("low"), PriorityLow)
	assert.Equal(t, TaskPriority("medium"), PriorityMedium)
	assert.Equal(t, TaskPriority("high"), PriorityHigh)
	assert.Equal(t, TaskPriority("urgent"), PriorityUrgent)
}

// ========== Task 结构测试 ==========

func TestTask_Structure(t *testing.T) {
	now := time.Now()
	dueDate := now.Add(7 * 24 * time.Hour)

	task := Task{
		ID:             "task-1",
		Title:          "Implement feature",
		Description:    "Description of task",
		Status:         TaskStatusInProgress,
		Priority:       PriorityHigh,
		AssigneeID:     "user-1",
		ReporterID:     "user-2",
		ProjectID:      "project-1",
		MilestoneID:    "milestone-1",
		ParentID:       "",
		Tags:           []string{"backend", "api"},
		Labels:         []string{"enhancement"},
		DueDate:        &dueDate,
		StartDate:      &now,
		CompletedAt:    nil,
		EstimatedHours: 8.0,
		ActualHours:    5.5,
		Progress:       60,
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      "user-2",
	}

	assert.Equal(t, "task-1", task.ID)
	assert.Equal(t, "Implement feature", task.Title)
	assert.Equal(t, TaskStatusInProgress, task.Status)
	assert.Equal(t, PriorityHigh, task.Priority)
	assert.Len(t, task.Tags, 2)
	assert.Equal(t, 60, task.Progress)
}

func TestTask_Minimal(t *testing.T) {
	task := Task{
		ID:        "task-1",
		Title:     "Simple task",
		Status:    TaskStatusTodo,
		Priority:  PriorityMedium,
		CreatedAt: time.Now(),
	}

	assert.Equal(t, "task-1", task.ID)
	assert.Equal(t, "Simple task", task.Title)
	assert.Empty(t, task.Tags)
	assert.Empty(t, task.Labels)
	assert.Nil(t, task.DueDate)
}

// ========== Milestone 结构测试 ==========

func TestMilestone_Structure(t *testing.T) {
	now := time.Now()
	dueDate := now.Add(30 * 24 * time.Hour)

	milestone := Milestone{
		ID:          "milestone-1",
		Name:        "v1.0 Release",
		Description: "First release",
		ProjectID:   "project-1",
		Status:      "active",
		DueDate:     &dueDate,
		TaskCount:   10,
		DoneCount:   3,
		Progress:    30,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   "user-1",
	}

	assert.Equal(t, "milestone-1", milestone.ID)
	assert.Equal(t, "v1.0 Release", milestone.Name)
	assert.Equal(t, "active", milestone.Status)
	assert.Equal(t, 10, milestone.TaskCount)
	assert.Equal(t, 30, milestone.Progress)
}

func TestMilestone_Completed(t *testing.T) {
	now := time.Now()
	completedAt := now

	milestone := Milestone{
		ID:          "milestone-1",
		Name:        "Completed milestone",
		Status:      "completed",
		CompletedAt: &completedAt,
		TaskCount:   5,
		DoneCount:   5,
		Progress:    100,
		CreatedAt:   now,
	}

	assert.Equal(t, "completed", milestone.Status)
	assert.NotNil(t, milestone.CompletedAt)
	assert.Equal(t, 100, milestone.Progress)
}

// ========== Project 结构测试 ==========

func TestProject_Structure(t *testing.T) {
	now := time.Now()
	startDate := now
	endDate := now.Add(90 * 24 * time.Hour)

	project := Project{
		ID:          "project-1",
		Name:        "NAS-OS",
		Description: "NAS Operating System",
		Key:         "NAS",
		Status:      "active",
		OwnerID:     "user-1",
		MemberIDs:   []string{"user-1", "user-2", "user-3"},
		StartDtae:   &startDate,
		EndDate:     &endDate,
		TaskCount:   50,
		DoneCount:   20,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   "user-1",
	}

	assert.Equal(t, "project-1", project.ID)
	assert.Equal(t, "NAS-OS", project.Name)
	assert.Equal(t, "NAS", project.Key)
	assert.Equal(t, "active", project.Status)
	assert.Len(t, project.MemberIDs, 3)
}

func TestProject_Minimal(t *testing.T) {
	project := Project{
		ID:        "project-1",
		Name:      "Simple Project",
		Key:       "SIM",
		Status:    "active",
		OwnerID:   "user-1",
		CreatedAt: time.Now(),
	}

	assert.Equal(t, "project-1", project.ID)
	assert.Empty(t, project.MemberIDs)
}

// ========== TaskComment 结构测试 ==========

func TestTaskComment_Structure(t *testing.T) {
	now := time.Now()
	comment := TaskComment{
		ID:        "comment-1",
		TaskID:    "task-1",
		UserID:    "user-1",
		Content:   "This is a comment",
		CreatedAt: now,
		UpdatedAt: now,
	}

	assert.Equal(t, "comment-1", comment.ID)
	assert.Equal(t, "task-1", comment.TaskID)
	assert.Equal(t, "This is a comment", comment.Content)
}

// ========== TaskHistory 结构测试 ==========

func TestTaskHistory_Structure(t *testing.T) {
	now := time.Now()
	history := TaskHistory{
		ID:        "history-1",
		TaskID:    "task-1",
		Field:     "status",
		OldValue:  "todo",
		NewValue:  "in_progress",
		UserID:    "user-1",
		Timestamp: now,
		Metadata: map[string]interface{}{
			"source": "web",
		},
	}

	assert.Equal(t, "history-1", history.ID)
	assert.Equal(t, "status", history.Field)
	assert.Equal(t, "todo", history.OldValue)
	assert.Equal(t, "in_progress", history.NewValue)
	assert.NotNil(t, history.Metadata)
}

// ========== TaskFilter 结构测试 ==========

func TestTaskFilter_Structure(t *testing.T) {
	now := time.Now()
	filter := TaskFilter{
		Status:      []TaskStatus{TaskStatusTodo, TaskStatusInProgress},
		Priority:    []TaskPriority{PriorityHigh, PriorityUrgent},
		AssigneeID:  "user-1",
		ReporterID:  "user-2",
		ProjectID:   "project-1",
		MilestoneID: "milestone-1",
		Tags:        []string{"backend"},
		Labels:      []string{"bug"},
		Search:      "important",
		DueBefore:   &now,
		OrderBy:     "priority",
		OrderDesc:   true,
		Limit:       10,
		Offset:      0,
	}

	assert.Len(t, filter.Status, 2)
	assert.Len(t, filter.Priority, 2)
	assert.Equal(t, "user-1", filter.AssigneeID)
	assert.Equal(t, "important", filter.Search)
	assert.Equal(t, 10, filter.Limit)
}

func TestTaskFilter_Empty(t *testing.T) {
	filter := TaskFilter{}

	assert.Empty(t, filter.Status)
	assert.Empty(t, filter.Priority)
	assert.Empty(t, filter.AssigneeID)
	assert.Zero(t, filter.Limit)
}

// ========== TaskStats 结构测试 ==========

func TestTaskStats_Structure(t *testing.T) {
	stats := TaskStats{
		Total: 100,
		ByStatus: map[string]int{
			"todo":        20,
			"in_progress": 30,
			"done":        50,
		},
		ByPriority: map[string]int{
			"low":    30,
			"medium": 40,
			"high":   20,
			"urgent": 10,
		},
		ByAssignee: map[string]int{
			"user-1": 25,
			"user-2": 35,
		},
		Overdue:           5,
		CompletedThisWeek: 15,
		CreatedThisWeek:   10,
	}

	assert.Equal(t, 100, stats.Total)
	assert.Equal(t, 5, stats.Overdue)
	assert.Equal(t, 15, stats.CompletedThisWeek)
	assert.Len(t, stats.ByStatus, 3)
	assert.Len(t, stats.ByPriority, 4)
}

func TestTaskStats_Empty(t *testing.T) {
	stats := TaskStats{}

	assert.Zero(t, stats.Total)
	assert.Nil(t, stats.ByStatus)
	assert.Nil(t, stats.ByPriority)
	assert.Zero(t, stats.Overdue)
}

// ========== 边界测试 ==========

func TestTask_ProgressRange(t *testing.T) {
	tests := []struct {
		name     string
		progress int
		valid    bool
	}{
		{"零进度", 0, true},
		{"一半进度", 50, true},
		{"完成", 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := Task{Progress: tt.progress}
			assert.Equal(t, tt.progress, task.Progress)
		})
	}
}

func TestTask_Dates(t *testing.T) {
	now := time.Now()
	future := now.Add(7 * 24 * time.Hour)
	past := now.Add(-7 * 24 * time.Hour)

	task := Task{
		DueDate:     &future,
		StartDate:   &past,
		CompletedAt: nil,
	}

	assert.True(t, task.DueDate.After(*task.StartDate))
	assert.Nil(t, task.CompletedAt)
}

// ========== 基准测试 ==========

func BenchmarkTask_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Task{
			ID:        "task-1",
			Title:     "Benchmark task",
			Status:    TaskStatusTodo,
			Priority:  PriorityMedium,
			CreatedAt: time.Now(),
		}
	}
}

func BenchmarkProject_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Project{
			ID:        "project-1",
			Name:      "Benchmark Project",
			Key:       "BENCH",
			Status:    "active",
			OwnerID:   "user-1",
			CreatedAt: time.Now(),
		}
	}
}

func BenchmarkTaskFilter_Creation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = TaskFilter{
			Status:   []TaskStatus{TaskStatusTodo},
			Priority: []TaskPriority{PriorityHigh},
			Limit:    10,
		}
	}
}
