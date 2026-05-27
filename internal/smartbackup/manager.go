package smartbackup

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager 智能备份策略管理器
type Manager struct {
	logger     *zap.Logger
	policies   map[string]*BackupPolicy
	executions map[string]*BackupExecution
	mu         sync.RWMutex
}

// NewManager 创建新的管理器实例
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger:     logger,
		policies:   make(map[string]*BackupPolicy),
		executions: make(map[string]*BackupExecution),
	}
}

// CreatePolicy 创建备份策略
func (m *Manager) CreatePolicy(policy *BackupPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = uuid.New().String()
	}

	if err := m.validatePolicy(policy); err != nil {
		return err
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	m.policies[policy.ID] = policy
	m.logger.Info("Backup policy created", zap.String("policy_id", policy.ID))
	return nil
}

// GetPolicy 获取备份策略
func (m *Manager) GetPolicy(id string) (*BackupPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy not found: %s", id)
	}
	return policy, nil
}

// ListPolicies 列出所有备份策略
func (m *Manager) ListPolicies() []*BackupPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*BackupPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// UpdatePolicy 更新备份策略
func (m *Manager) UpdatePolicy(policy *BackupPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.policies[policy.ID]
	if !ok {
		return fmt.Errorf("policy not found: %s", policy.ID)
	}

	if err := m.validatePolicy(policy); err != nil {
		return err
	}

	policy.CreatedAt = existing.CreatedAt
	policy.UpdatedAt = time.Now()

	m.policies[policy.ID] = policy
	m.logger.Info("Backup policy updated", zap.String("policy_id", policy.ID))
	return nil
}

// DeletePolicy 删除备份策略
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("policy not found: %s", id)
	}

	delete(m.policies, id)
	m.logger.Info("Backup policy deleted", zap.String("policy_id", id))
	return nil
}

// AnalyzeStrategy 分析并推荐备份策略
func (m *Manager) AnalyzeStrategy(analysis *StrategyAnalysis) (*BackupStrategy, error) {
	if analysis == nil {
		return nil, fmt.Errorf("analysis cannot be nil")
	}

	strategy := &BackupStrategy{
		Recommendations: make([]string, 0),
	}

	// 根据数据变化频率推荐备份类型
	if analysis.ChangeFrequency != nil {
		strategy.RecommendedType = m.recommendBackupType(analysis)
		strategy.Reason = fmt.Sprintf("基于数据变化率 %.2f 推荐", analysis.ChangeFrequency.ChangeRate)
	} else {
		strategy.RecommendedType = BackupTypeFull
		strategy.Reason = "默认推荐全量备份"
	}

	// 估算备份大小和时间
	strategy.EstimatedSize = m.estimateBackupSize(analysis)
	strategy.EstimatedTime = m.estimateBackupDuration(analysis)

	// 检查 RPO/RTO 可行性
	if analysis.Requirements != nil {
		strategy.RPOFeasible = m.checkRPOFeasibility(analysis)
	}
	if analysis.RTORequirements != nil {
		strategy.RTOFeasible = m.checkRTOFeasibility(analysis)
	}

	// 生成建议
	strategy.Recommendations = m.generateRecommendations(analysis, strategy)
	strategy.Confidence = m.calculateConfidence(analysis, strategy)

	return strategy, nil
}

// OptimizeBackupWindow 优化备份窗口
func (m *Manager) OptimizeBackupWindow(policy *BackupPolicy, changeFreq *ChangeFrequency) (*WindowOptimization, error) {
	if policy == nil {
		return nil, fmt.Errorf("policy cannot be nil")
	}

	optimization := &WindowOptimization{
		Suggestions: make([]string, 0),
	}

	// 模拟分析峰值时间
	peakHours := []int{9, 10, 11, 14, 15, 16} // 工作时间
	offPeakHours := []int{0, 1, 2, 3, 4, 5, 6, 22, 23}

	start, end := m.findOptimalWindow(peakHours)

	optimization.RecommendedStart = start
	optimization.RecommendedEnd = end
	optimization.PeakHours = peakHours
	optimization.OffPeakHours = offPeakHours
	optimization.Reason = "推荐在低峰时段进行备份以减少业务影响"
	optimization.Suggestions = append(optimization.Suggestions,
		"建议在凌晨2-5点执行全量备份",
		"增量备份可在白天低峰时段执行",
	)

	return optimization, nil
}

// EvaluatePolicy 评估策略效果
func (m *Manager) EvaluatePolicy(policyID string) (*PolicyEvaluation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[policyID]
	if !ok {
		return nil, fmt.Errorf("policy not found: %s", policyID)
	}

	evaluation := &PolicyEvaluation{
		PolicyID:        policyID,
		Recommendations: make([]string, 0),
	}

	// 统计执行记录
	totalExecutions := 0
	failedExecutions := 0
	totalDuration := time.Duration(0)

	for _, exec := range m.executions {
		if exec.PolicyID == policyID {
			totalExecutions++
			if exec.Status == BackupStatusFailed {
				failedExecutions++
			}
			if !exec.EndTime.IsZero() {
				totalDuration += exec.EndTime.Sub(exec.StartTime)
			}
		}
	}

	evaluation.TotalExecutions = totalExecutions
	evaluation.FailedExecutions = failedExecutions

	if totalExecutions > 0 {
		evaluation.SuccessRate = float64(totalExecutions-failedExecutions) / float64(totalExecutions) * 100
		evaluation.AvgDuration = (totalDuration / time.Duration(totalExecutions)).String()
	}

	// 检查 RPO/RTO 合规性
	evaluation.RPOCompliance = policy.RPO == nil || evaluation.SuccessRate > 95
	evaluation.RTOCompliance = policy.RTO == nil || evaluation.SuccessRate > 90

	// 计算评分
	score := 100.0
	if evaluation.FailedExecutions > 0 {
		score -= float64(evaluation.FailedExecutions) * 10
	}
	if !evaluation.RPOCompliance {
		score -= 15
	}
	if !evaluation.RTOCompliance {
		score -= 15
	}
	evaluation.Score = math.Max(0, math.Min(100, score))

	// 生成建议
	if evaluation.FailedExecutions > 0 {
		evaluation.Recommendations = append(evaluation.Recommendations, "存在失败的备份执行，建议检查备份目标可用性")
	}
	if !evaluation.RPOCompliance {
		evaluation.Recommendations = append(evaluation.Recommendations, "RPO 不合规，建议增加备份频率")
	}

	return evaluation, nil
}

// RecordExecution 记录备份执行
func (m *Manager) RecordExecution(execution *BackupExecution) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if execution.ID == "" {
		execution.ID = uuid.New().String()
	}

	m.executions[execution.ID] = execution
	m.logger.Info("Backup execution recorded", zap.String("execution_id", execution.ID))
	return nil
}

// GetExecution 获取备份执行记录
func (m *Manager) GetExecution(id string) (*BackupExecution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exec, ok := m.executions[id]
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", id)
	}
	return exec, nil
}

// ListExecutions 列出执行记录
func (m *Manager) ListExecutions(policyID string) []*BackupExecution {
	m.mu.RLock()
	defer m.mu.RUnlock()

	executions := make([]*BackupExecution, 0)
	for _, exec := range m.executions {
		if policyID == "" || exec.PolicyID == policyID {
			executions = append(executions, exec)
		}
	}
	return executions
}

// validatePolicy 验证策略
func (m *Manager) validatePolicy(policy *BackupPolicy) error {
	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	if len(policy.SourcePaths) == 0 {
		return fmt.Errorf("at least one source path is required")
	}
	return nil
}

// recommendBackupType 推荐备份类型
func (m *Manager) recommendBackupType(analysis *StrategyAnalysis) BackupType {
	if analysis.ChangeFrequency == nil {
		return BackupTypeFull
	}

	rate := analysis.ChangeFrequency.ChangeRate
	if rate < 0.1 {
		return BackupTypeIncremental
	} else if rate < 0.3 {
		return BackupTypeDifferential
	}
	return BackupTypeFull
}

// estimateBackupSize 估算备份大小
func (m *Manager) estimateBackupSize(analysis *StrategyAnalysis) float64 {
	if analysis.ChangeFrequency == nil {
		return analysis.DataSizeGB * 1024 // MB
	}
	return float64(analysis.ChangeFrequency.DailyChanges)
}

// estimateBackupDuration 估算备份时间
func (m *Manager) estimateBackupDuration(analysis *StrategyAnalysis) float64 {
	// 假设 100MB/s 的备份速度
	sizeMB := m.estimateBackupSize(analysis)
	return sizeMB / 100 / 60 // 分钟
}

// checkRPOFeasibility 检查 RPO 可行性
func (m *Manager) checkRPOFeasibility(analysis *StrategyAnalysis) bool {
	if analysis.Requirements == nil {
		return true
	}
	// 简单检查：如果 RPO 要求大于1小时，增量备份可行
	return analysis.Requirements.MaxDataLoss >= time.Hour
}

// checkRTOFeasibility 检查 RTO 可行性
func (m *Manager) checkRTOFeasibility(analysis *StrategyAnalysis) bool {
	if analysis.RTORequirements == nil {
		return true
	}
	// 简单检查：如果 RTO 要求大于30分钟，可行
	return analysis.RTORequirements.MaxRecoveryTime >= 30*time.Minute
}

// generateRecommendations 生成建议
func (m *Manager) generateRecommendations(analysis *StrategyAnalysis, strategy *BackupStrategy) []string {
	recommendations := make([]string, 0)

	if strategy.RecommendedType == BackupTypeFull {
		recommendations = append(recommendations, "建议每周执行一次全量备份")
		recommendations = append(recommendations, "每日执行增量备份以减少备份时间")
	}

	if analysis.DataSizeGB > 100 {
		recommendations = append(recommendations, "大数据量建议使用增量备份减少备份窗口")
	}

	return recommendations
}

// calculateConfidence 计算置信度
func (m *Manager) calculateConfidence(analysis *StrategyAnalysis, strategy *BackupStrategy) float64 {
	confidence := 0.7

	if analysis.ChangeFrequency != nil {
		confidence += 0.1
	}
	if strategy.RPOFeasible {
		confidence += 0.1
	}
	if strategy.RTOFeasible {
		confidence += 0.1
	}

	return math.Min(1.0, confidence)
}

// findOptimalWindow 查找最优备份窗口
func (m *Manager) findOptimalWindow(peakHours []int) (int, int) {
	// 简单实现：返回凌晨2-5点
	peakMap := make(map[int]bool)
	for _, h := range peakHours {
		peakMap[h] = true
	}

	// 找最长连续非峰值时段
	bestStart := 0
	bestLength := 0
	currentStart := -1
	currentLength := 0

	for h := 0; h < 24; h++ {
		if !peakMap[h] {
			if currentStart == -1 {
				currentStart = h
			}
			currentLength++
			if currentLength > bestLength {
				bestStart = currentStart
				bestLength = currentLength
			}
		} else {
			currentStart = -1
			currentLength = 0
		}
	}

	return bestStart, (bestStart + bestLength) % 24
}
