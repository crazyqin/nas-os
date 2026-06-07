// Package complianceauto - 合规修复器
package complianceauto

import (
	"fmt"
	"sync"
	"time"
)

// Remediator 合规修复器
type Remediator struct {
	mu       sync.RWMutex
	actions  map[string]*RemediationAction
	executor RemediationExecutor
}

// RemediationExecutor 修复执行器接口
type RemediationExecutor interface {
	Execute(action *RemediationAction) (*RemediationResult, error)
}

// RemediationResult 修复结果
type RemediationResult struct {
	ActionID   string        `json:"actionId"`
	Success    bool          `json:"success"`
	Message    string        `json:"message"`
	ExecutedAt time.Time     `json:"executedAt"`
	Duration   time.Duration `json:"duration"`
	Changes    []string      `json:"changes,omitempty"`
}

// NewRemediator 创建修复器
func NewRemediator() *Remediator {
	return &Remediator{
		actions: make(map[string]*RemediationAction),
	}
}

// GetRemediations 获取修复建议列表
func (r *Remediator) GetRemediations(scan *ComplianceScan) []RemediationAction {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var actions []RemediationAction
	for _, check := range scan.Checks {
		if check.Result == ResultFail || check.Result == ResultWarning {
			if action, exists := r.actions[check.RuleID]; exists {
				actions = append(actions, *action)
			}
		}
	}
	return actions
}

// GetRemediation 获取单个修复建议
func (r *Remediator) GetRemediation(id string) (*RemediationAction, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	action, exists := r.actions[id]
	if !exists {
		return nil, fmt.Errorf("修复建议 %s 未找到", id)
	}
	return action, nil
}

// ExecuteRemediation 执行单个修复
func (r *Remediator) ExecuteRemediation(id string) (*RemediationResult, error) {
	r.mu.RLock()
	action, exists := r.actions[id]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("修复建议 %s 未找到", id)
	}

	start := time.Now()
	result := &RemediationResult{
		ActionID:   id,
		ExecutedAt: start,
	}

	// 模拟修复执行
	result.Success = true
	result.Message = fmt.Sprintf("修复 %s 执行成功", action.Title)
	result.Duration = time.Since(start)

	return result, nil
}

// ExecuteAll 批量执行修复
func (r *Remediator) ExecuteAll(scan *ComplianceScan, maxRisk SeverityLevel, dryRun bool) ([]RemediationResult, error) {
	r.mu.RLock()
	actions := r.GetRemediations(scan)
	r.mu.RUnlock()

	var results []RemediationResult
	for _, action := range actions {
		if action.RiskLevel <= maxRisk {
			if dryRun {
				results = append(results, RemediationResult{
					ActionID: action.ID,
					Success:  true,
					Message:  fmt.Sprintf("[试运行] 修复 %s", action.Title),
				})
			} else {
				result, err := r.ExecuteRemediation(action.ID)
				if err != nil {
					results = append(results, RemediationResult{
						ActionID: action.ID,
						Success:  false,
						Message:  err.Error(),
					})
				} else {
					results = append(results, *result)
				}
			}
		}
	}
	return results, nil
}

// RegisterAction 注册修复动作
func (r *Remediator) RegisterAction(action *RemediationAction) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actions[action.ID] = action
}
