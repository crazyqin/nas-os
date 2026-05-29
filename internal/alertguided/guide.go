package alertguided

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// GuideEngine 引导式修复引擎
type GuideEngine struct {
	kb     *KnowledgeBase
	logger *zap.Logger
}

// NewGuideEngine 创建引导引擎
func NewGuideEngine(kb *KnowledgeBase, logger *zap.Logger) *GuideEngine {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &GuideEngine{
		kb:     kb,
		logger: logger,
	}
}

// GetGuide 根据告警获取修复指引
func (ge *GuideEngine) GetGuide(alert *GuidedAlert) *RepairGuide {
	// 尝试按知识库ID匹配
	entry, ok := ge.kb.Get(alert.Title)
	if !ok {
		// 尝试按类别匹配
		entries := ge.kb.LookupByCategory(alert.Category)
		if len(entries) > 0 {
			entry = entries[0]
		}
	}

	if entry == nil {
		return ge.buildGenericGuide(alert)
	}

	return ge.buildGuideFromEntry(alert, entry)
}

// GetGuideByKnowledgeID 按知识库ID获取指引
func (ge *GuideEngine) GetGuideByKnowledgeID(knowledgeID string) (*RepairGuide, error) {
	entry, ok := ge.kb.Get(knowledgeID)
	if !ok {
		return nil, fmt.Errorf("knowledge entry %s not found", knowledgeID)
	}
	return ge.buildGuideFromEntry(nil, entry), nil
}

// SearchGuides 搜索修复指引
func (ge *GuideEngine) SearchGuides(keyword string) []*RepairGuide {
	entries := ge.kb.Search(keyword)
	var guides []*RepairGuide
	for _, entry := range entries {
		guides = append(guides, ge.buildGuideFromEntry(nil, entry))
	}
	return guides
}

func (ge *GuideEngine) buildGuideFromEntry(alert *GuidedAlert, entry *KnowledgeEntry) *RepairGuide {
	guide := &RepairGuide{
		KnowledgeID: entry.ID,
		Title:       entry.Title,
		Category:    entry.Category,
		Severity:    entry.Severity,
		Causes:      entry.Causes,
		Symptoms:    entry.Symptoms,
		Steps:       make([]GuideStep, len(entry.Steps)),
		References:  entry.References,
	}

	for i, step := range entry.Steps {
		guide.Steps[i] = GuideStep{
			Order:          step.Order,
			Title:          step.Title,
			Description:    step.Description,
			Command:        step.Command,
			ExpectedResult: step.ExpectedResult,
			RiskLevel:      step.RiskLevel,
			RequiresAck:    step.RequiresAck,
			IsOptional:     step.IsOptional,
			Alternatives:   step.Alternatives,
			Status:         StepStatusPending,
		}
	}

	if alert != nil {
		guide.AlertID = alert.ID
		guide.Message = alert.Message
	}

	return guide
}

func (ge *GuideEngine) buildGenericGuide(alert *GuidedAlert) *RepairGuide {
	return &RepairGuide{
		AlertID:  alert.ID,
		Title:    fmt.Sprintf("通用排查: %s", alert.Title),
		Category: alert.Category,
		Severity: alert.Severity,
		Message:  alert.Message,
		Causes:   []string{"请根据告警信息分析具体原因"},
		Steps: []GuideStep{
			{Order: 1, Title: "查看系统日志", Description: "检查系统日志获取详细错误信息", Command: "journalctl -xe -n 50", Status: StepStatusPending},
			{Order: 2, Title: "检查系统资源", Description: "确认系统资源使用情况", Command: "top -bn1 && free -h && df -h", Status: StepStatusPending},
			{Order: 3, Title: "搜索相关知识", Description: "在知识库中搜索相关解决方案", Status: StepStatusPending},
		},
	}
}

// RepairGuide 修复指引
type RepairGuide struct {
	AlertID     string      `json:"alertId,omitempty"`
	KnowledgeID string      `json:"knowledgeId"`
	Title       string      `json:"title"`
	Category    Category    `json:"category"`
	Severity    Severity    `json:"severity"`
	Message     string      `json:"message,omitempty"`
	Causes      []string    `json:"causes"`
	Symptoms    []string    `json:"symptoms,omitempty"`
	Steps       []GuideStep `json:"steps"`
	References  []string    `json:"references,omitempty"`
}

// GuideStep 引导步骤
type GuideStep struct {
	Order          int      `json:"order"`
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Command        string   `json:"command,omitempty"`
	ExpectedResult string   `json:"expectedResult,omitempty"`
	RiskLevel      string   `json:"riskLevel,omitempty"`
	RequiresAck    bool     `json:"requiresAck,omitempty"`
	IsOptional     bool     `json:"isOptional,omitempty"`
	Alternatives   []string `json:"alternatives,omitempty"`
	Status         StepStatus `json:"status"`
	Note           string   `json:"note,omitempty"`
}

// StepStatus 步骤状态
type StepStatus string

const (
	StepStatusPending    StepStatus = "PENDING"
	StepStatusInProgress StepStatus = "IN_PROGRESS"
	StepStatusCompleted  StepStatus = "COMPLETED"
	StepStatusSkipped    StepStatus = "SKIPPED"
	StepStatusFailed     StepStatus = "FAILED"
)

// GuideTracker 修复进度追踪器
type GuideTracker struct {
	progress map[string]*RepairProgress // alertID -> progress
	mu       sync.RWMutex
	logger   *zap.Logger
}

// NewGuideTracker 创建追踪器
func NewGuideTracker(logger *zap.Logger) *GuideTracker {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &GuideTracker{
		progress: make(map[string]*RepairProgress),
		logger:   logger,
	}
}

// RepairProgress 修复进度
type RepairProgress struct {
	AlertID        string              `json:"alertId"`
	KnowledgeID    string              `json:"knowledgeId"`
	TotalSteps     int                 `json:"totalSteps"`
	CompletedSteps int                 `json:"completedSteps"`
	SkippedSteps   int                 `json:"skippedSteps"`
	FailedSteps    int                 `json:"failedSteps"`
	StepStatuses   map[int]StepStatus  `json:"stepStatuses"`
	Notes          map[int]string      `json:"notes,omitempty"`
	StartedAt      int64               `json:"startedAt"`
	UpdatedAt      int64               `json:"updatedAt"`
}

// StartRepair 开始修复流程
func (gt *GuideTracker) StartRepair(alertID, knowledgeID string, totalSteps int) *RepairProgress {
	gt.mu.Lock()
	defer gt.mu.Unlock()

	now := timestamp()
	progress := &RepairProgress{
		AlertID:      alertID,
		KnowledgeID:  knowledgeID,
		TotalSteps:   totalSteps,
		StepStatuses: make(map[int]StepStatus),
		Notes:        make(map[int]string),
		StartedAt:    now,
		UpdatedAt:    now,
	}
	for i := 1; i <= totalSteps; i++ {
		progress.StepStatuses[i] = StepStatusPending
	}
	gt.progress[alertID] = progress
	gt.logger.Info("repair started", zap.String("alertId", alertID))
	return progress
}

// UpdateStep 更新步骤状态
func (gt *GuideTracker) UpdateStep(alertID string, stepOrder int, status StepStatus, note string) error {
	gt.mu.Lock()
	defer gt.mu.Unlock()

	progress, ok := gt.progress[alertID]
	if !ok {
		return fmt.Errorf("no repair progress for alert %s", alertID)
	}

	oldStatus := progress.StepStatuses[stepOrder]
	progress.StepStatuses[stepOrder] = status
	progress.UpdatedAt = timestamp()
	if note != "" {
		progress.Notes[stepOrder] = note
	}

	// 更新计数
	switch oldStatus {
	case StepStatusCompleted:
		progress.CompletedSteps--
	case StepStatusSkipped:
		progress.SkippedSteps--
	case StepStatusFailed:
		progress.FailedSteps--
	}
	switch status {
	case StepStatusCompleted:
		progress.CompletedSteps++
	case StepStatusSkipped:
		progress.SkippedSteps++
	case StepStatusFailed:
		progress.FailedSteps++
	}

	gt.logger.Info("step updated",
		zap.String("alertId", alertID),
		zap.Int("step", stepOrder),
		zap.String("from", string(oldStatus)),
		zap.String("to", string(status)),
	)
	return nil
}

// GetProgress 获取修复进度
func (gt *GuideTracker) GetProgress(alertID string) (*RepairProgress, bool) {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	p, ok := gt.progress[alertID]
	return p, ok
}

// IsComplete 判断修复是否完成
func (gt *GuideTracker) IsComplete(alertID string) bool {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	p, ok := gt.progress[alertID]
	if !ok {
		return false
	}
	return p.CompletedSteps+p.SkippedSteps >= p.TotalSteps
}

// GetProgressPercent 获取完成百分比
func (gt *GuideTracker) GetProgressPercent(alertID string) float64 {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	p, ok := gt.progress[alertID]
	if !ok || p.TotalSteps == 0 {
		return 0
	}
	return float64(p.CompletedSteps+p.SkippedSteps) / float64(p.TotalSteps) * 100
}

// ListActive 列出进行中的修复
func (gt *GuideTracker) ListActive() []*RepairProgress {
	gt.mu.RLock()
	defer gt.mu.RUnlock()
	var result []*RepairProgress
	for _, p := range gt.progress {
		if p.CompletedSteps+p.SkippedSteps < p.TotalSteps {
			result = append(result, p)
		}
	}
	return result
}

// Remove 移除修复记录
func (gt *GuideTracker) Remove(alertID string) {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	delete(gt.progress, alertID)
}

func timestamp() int64 {
	return time.Now().Unix()
}
