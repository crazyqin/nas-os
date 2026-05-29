package alertguided

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RepairTracker 修复状态追踪器
// 追踪告警修复进度，支持多步骤修复流程
type RepairTracker struct {
	repairs map[string]*RepairRecord // alertID -> record
	mu      sync.RWMutex
	logger  *zap.Logger
}

// NewRepairTracker 创建修复追踪器
func NewRepairTracker(logger *zap.Logger) *RepairTracker {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RepairTracker{
		repairs: make(map[string]*RepairRecord),
		logger:  logger,
	}
}

// RepairRecord 修复记录
type RepairRecord struct {
	AlertID        string               `json:"alertId"`
	KnowledgeID    string               `json:"knowledgeId"`
	Title          string               `json:"title"`
	Status         RepairStatus         `json:"status"`
	TotalSteps     int                  `json:"totalSteps"`
	CurrentStep    int                  `json:"currentStep"`
	StepRecords    map[int]*StepRecord  `json:"stepRecords"`
	Assignee       string               `json:"assignee,omitempty"`
	StartedAt      time.Time            `json:"startedAt"`
	UpdatedAt      time.Time            `json:"updatedAt"`
	CompletedAt    *time.Time           `json:"completedAt,omitempty"`
	Notes          []RepairNote         `json:"notes,omitempty"`
}

// StepRecord 步骤记录
type StepRecord struct {
	Order      int        `json:"order"`
	Title      string     `json:"title"`
	Status     StepStatus `json:"status"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	Output     string     `json:"output,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// RepairNote 修复备注
type RepairNote struct {
	Content   string    `json:"content"`
	Author    string    `json:"author,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// RepairStatus 修复状态
type RepairStatus string

const (
	RepairStatusNotStarted RepairStatus = "NOT_STARTED"
	RepairStatusInProgress RepairStatus = "IN_PROGRESS"
	RepairStatusCompleted  RepairStatus = "COMPLETED"
	RepairStatusFailed     RepairStatus = "FAILED"
	RepairStatusAbandoned  RepairStatus = "ABANDONED"
)

// Start 开始修复
func (rt *RepairTracker) Start(alertID, knowledgeID, title string, totalSteps int, assignee string) *RepairRecord {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	now := time.Now()
	record := &RepairRecord{
		AlertID:     alertID,
		KnowledgeID: knowledgeID,
		Title:       title,
		Status:      RepairStatusInProgress,
		TotalSteps:  totalSteps,
		CurrentStep: 1,
		StepRecords: make(map[int]*StepRecord),
		Assignee:    assignee,
		StartedAt:   now,
		UpdatedAt:   now,
	}
	for i := 1; i <= totalSteps; i++ {
		record.StepRecords[i] = &StepRecord{
			Order:  i,
			Status: StepStatusPending,
		}
	}
	rt.repairs[alertID] = record
	rt.logger.Info("repair started",
		zap.String("alertId", alertID),
		zap.String("title", title),
		zap.Int("totalSteps", totalSteps),
	)
	return record
}

// CompleteStep 完成步骤
func (rt *RepairTracker) CompleteStep(alertID string, stepOrder int, output string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	record, ok := rt.repairs[alertID]
	if !ok {
		return fmt.Errorf("repair record not found for alert %s", alertID)
	}
	step, ok := record.StepRecords[stepOrder]
	if !ok {
		return fmt.Errorf("step %d not found", stepOrder)
	}

	now := time.Now()
	step.Status = StepStatusCompleted
	step.CompletedAt = &now
	step.Output = output
	record.UpdatedAt = now

	// 更新当前步骤
	record.CurrentStep = stepOrder + 1

	// 检查是否全部完成
	completed := 0
	for _, s := range record.StepRecords {
		if s.Status == StepStatusCompleted || s.Status == StepStatusSkipped {
			completed++
		}
	}
	if completed >= record.TotalSteps {
		record.Status = RepairStatusCompleted
		record.CompletedAt = &now
		rt.logger.Info("repair completed", zap.String("alertId", alertID))
	}

	return nil
}

// FailStep 标记步骤失败
func (rt *RepairTracker) FailStep(alertID string, stepOrder int, errMsg string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	record, ok := rt.repairs[alertID]
	if !ok {
		return fmt.Errorf("repair record not found for alert %s", alertID)
	}
	step, ok := record.StepRecords[stepOrder]
	if !ok {
		return fmt.Errorf("step %d not found", stepOrder)
	}

	now := time.Now()
	step.Status = StepStatusFailed
	step.CompletedAt = &now
	step.Error = errMsg
	record.UpdatedAt = now
	record.Status = RepairStatusFailed

	rt.logger.Warn("repair step failed",
		zap.String("alertId", alertID),
		zap.Int("step", stepOrder),
		zap.String("error", errMsg),
	)
	return nil
}

// SkipStep 跳过步骤
func (rt *RepairTracker) SkipStep(alertID string, stepOrder int) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	record, ok := rt.repairs[alertID]
	if !ok {
		return fmt.Errorf("repair record not found for alert %s", alertID)
	}
	step, ok := record.StepRecords[stepOrder]
	if !ok {
		return fmt.Errorf("step %d not found", stepOrder)
	}

	step.Status = StepStatusSkipped
	record.UpdatedAt = time.Now()
	return nil
}

// AddNote 添加备注
func (rt *RepairTracker) AddNote(alertID, content, author string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	record, ok := rt.repairs[alertID]
	if !ok {
		return fmt.Errorf("repair record not found for alert %s", alertID)
	}
	record.Notes = append(record.Notes, RepairNote{
		Content:   content,
		Author:    author,
		CreatedAt: time.Now(),
	})
	record.UpdatedAt = time.Now()
	return nil
}

// Get 获取修复记录
func (rt *RepairTracker) Get(alertID string) (*RepairRecord, bool) {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	r, ok := rt.repairs[alertID]
	return r, ok
}

// Abandon 放弃修复
func (rt *RepairTracker) Abandon(alertID string) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	record, ok := rt.repairs[alertID]
	if !ok {
		return fmt.Errorf("repair record not found for alert %s", alertID)
	}
	record.Status = RepairStatusAbandoned
	record.UpdatedAt = time.Now()
	return nil
}

// ProgressPercent 完成百分比
func (rt *RepairTracker) ProgressPercent(alertID string) float64 {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	record, ok := rt.repairs[alertID]
	if !ok || record.TotalSteps == 0 {
		return 0
	}
	done := 0
	for _, s := range record.StepRecords {
		if s.Status == StepStatusCompleted || s.Status == StepStatusSkipped {
			done++
		}
	}
	return float64(done) / float64(record.TotalSteps) * 100
}

// ListActive 列出进行中的修复
func (rt *RepairTracker) ListActive() []*RepairRecord {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	var result []*RepairRecord
	for _, r := range rt.repairs {
		if r.Status == RepairStatusInProgress {
			result = append(result, r)
		}
	}
	return result
}

// ListAll 列出所有修复记录
func (rt *RepairTracker) ListAll() []*RepairRecord {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	result := make([]*RepairRecord, 0, len(rt.repairs))
	for _, r := range rt.repairs {
		result = append(result, r)
	}
	return result
}

// Remove 移除修复记录
func (rt *RepairTracker) Remove(alertID string) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	delete(rt.repairs, alertID)
}
