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
	targets    map[string]*BackupTarget
	chains     map[string]*BackupChain
	verifications map[string]*VerificationResult
	mu         sync.RWMutex
}

// NewManager 创建新的管理器实例
func NewManager(logger *zap.Logger) *Manager {
	return &Manager{
		logger:        logger,
		policies:      make(map[string]*BackupPolicy),
		executions:    make(map[string]*BackupExecution),
		targets:       make(map[string]*BackupTarget),
		chains:        make(map[string]*BackupChain),
		verifications: make(map[string]*VerificationResult),
	}
}

// ==================== 策略管理 ====================

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
	policy.HealthScore = 100

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

// ==================== 目标管理 ====================

// CreateTarget 创建备份目标
func (m *Manager) CreateTarget(target *BackupTarget) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if target.ID == "" {
		target.ID = uuid.New().String()
	}

	if err := m.validateTarget(target); err != nil {
		return err
	}

	now := time.Now()
	target.CreatedAt = now
	target.UpdatedAt = now
	target.Status = "active"

	m.targets[target.ID] = target
	m.logger.Info("Backup target created", zap.String("target_id", target.ID), zap.String("type", string(target.Type)))
	return nil
}

// GetTarget 获取备份目标
func (m *Manager) GetTarget(id string) (*BackupTarget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	target, ok := m.targets[id]
	if !ok {
		return nil, fmt.Errorf("target not found: %s", id)
	}
	return target, nil
}

// ListTargets 列出所有备份目标
func (m *Manager) ListTargets() []*BackupTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := make([]*BackupTarget, 0, len(m.targets))
	for _, t := range m.targets {
		targets = append(targets, t)
	}
	return targets
}

// DeleteTarget 删除备份目标
func (m *Manager) DeleteTarget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.targets[id]; !ok {
		return fmt.Errorf("target not found: %s", id)
	}

	delete(m.targets, id)
	m.logger.Info("Backup target deleted", zap.String("target_id", id))
	return nil
}

// ==================== 策略分析 ====================

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

	// 检查3-2-1合规性
	strategy.ThreeTwoOne = m.checkThreeTwoOneCompliance(analysis)

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
	peakHours := []int{9, 10, 11, 14, 15, 16}
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

// ==================== 执行记录管理 ====================

// RecordExecution 记录备份执行
func (m *Manager) RecordExecution(execution *BackupExecution) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if execution.ID == "" {
		execution.ID = uuid.New().String()
	}

	m.executions[execution.ID] = execution

	// 更新备份链路
	if execution.ChainID != "" {
		m.updateBackupChain(execution)
	}

	// 更新策略健康评分
	if policy, ok := m.policies[execution.PolicyID]; ok {
		m.updatePolicyHealthScore(policy)
	}

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

// ==================== 备份链路追踪 ====================

// CreateBackupChain 创建备份链路
func (m *Manager) CreateBackupChain(chain *BackupChain) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if chain.ID == "" {
		chain.ID = uuid.New().String()
	}

	now := time.Now()
	chain.CreatedAt = now
	chain.UpdatedAt = now
	chain.HealthScore = 100

	m.chains[chain.ID] = chain
	m.logger.Info("Backup chain created", zap.String("chain_id", chain.ID))
	return nil
}

// GetBackupChain 获取备份链路
func (m *Manager) GetBackupChain(id string) (*BackupChain, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chain, ok := m.chains[id]
	if !ok {
		return nil, fmt.Errorf("chain not found: %s", id)
	}
	return chain, nil
}

// ListBackupChains 列出备份链路
func (m *Manager) ListBackupChains(policyID string) []*BackupChain {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chains := make([]*BackupChain, 0)
	for _, chain := range m.chains {
		if policyID == "" || chain.PolicyID == policyID {
			chains = append(chains, chain)
		}
	}
	return chains
}

// ==================== 备份验证与恢复测试 ====================

// VerifyBackup 验证备份完整性
func (m *Manager) VerifyBackup(executionID string) (*VerificationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	exec, ok := m.executions[executionID]
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}

	result := &VerificationResult{
		ID:          uuid.New().String(),
		ExecutionID: executionID,
		CheckedAt:   time.Now(),
		Errors:      make([]string, 0),
	}

	// 模拟验证过程
	if exec.Status == BackupStatusSuccess {
		result.Status = "passed"
		result.FilesChecked = exec.FilesTotal
		result.FilesPassed = exec.FilesCopied
		result.FilesFailed = exec.FilesFailed
	} else {
		result.Status = "failed"
		result.FilesChecked = exec.FilesTotal
		result.FilesFailed = exec.FilesTotal
		result.Errors = append(result.Errors, "备份执行状态异常")
	}

	m.verifications[result.ID] = result
	m.logger.Info("Backup verification completed",
		zap.String("verification_id", result.ID),
		zap.String("status", result.Status))
	return result, nil
}

// TestRecovery 测试恢复能力
func (m *Manager) TestRecovery(executionID string, testPath string) (*VerificationResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	exec, ok := m.executions[executionID]
	if !ok {
		return nil, fmt.Errorf("execution not found: %s", executionID)
	}

	result := &VerificationResult{
		ID:          uuid.New().String(),
		ExecutionID: executionID,
		CheckedAt:   time.Now(),
		Errors:      make([]string, 0),
	}

	// 模拟恢复测试
	startTime := time.Now()
	if exec.Status == BackupStatusSuccess {
		// 模拟恢复时间
		time.Sleep(10 * time.Millisecond)
		restoreTime := time.Since(startTime).Seconds()

		result.Status = "passed"
		result.FilesChecked = exec.FilesTotal
		result.FilesPassed = exec.FilesCopied
		result.RecoveryTest = &RecoveryTestResult{
			ID:          uuid.New().String(),
			Status:      "passed",
			TestPath:    testPath,
			RestoreTime: restoreTime,
			TestedAt:    time.Now(),
		}
	} else {
		result.Status = "failed"
		result.Errors = append(result.Errors, "备份执行状态异常，无法进行恢复测试")
		result.RecoveryTest = &RecoveryTestResult{
			ID:       uuid.New().String(),
			Status:   "failed",
			TestPath: testPath,
			TestedAt: time.Now(),
			Error:    "备份执行状态异常",
		}
	}

	m.verifications[result.ID] = result
	m.logger.Info("Recovery test completed",
		zap.String("verification_id", result.ID),
		zap.String("status", result.Status))
	return result, nil
}

// GetVerification 获取验证结果
func (m *Manager) GetVerification(id string) (*VerificationResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result, ok := m.verifications[id]
	if !ok {
		return nil, fmt.Errorf("verification not found: %s", id)
	}
	return result, nil
}

// ListVerifications 列出验证结果
func (m *Manager) ListVerifications(executionID string) []*VerificationResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	results := make([]*VerificationResult, 0)
	for _, v := range m.verifications {
		if executionID == "" || v.ExecutionID == executionID {
			results = append(results, v)
		}
	}
	return results
}

// ==================== 智能调度 ====================

// OptimizeSchedule 根据系统负载优化调度
func (m *Manager) OptimizeSchedule(metrics *LoadMetrics) (*ScheduleOptimization, error) {
	if metrics == nil {
		return nil, fmt.Errorf("metrics cannot be nil")
	}

	optimization := &ScheduleOptimization{}

	// 计算负载评分
	loadScore := m.calculateLoadScore(metrics)
	optimization.LoadScore = loadScore

	// 根据负载决定是否立即执行
	if loadScore >= 80 {
		// 负载低，可以立即执行
		optimization.RecommendedTime = time.Now()
		optimization.Reason = "系统负载较低，建议立即执行备份"
		optimization.WaitMinutes = 0
	} else if loadScore >= 50 {
		// 负载中等，建议稍后执行
		optimization.RecommendedTime = time.Now().Add(30 * time.Minute)
		optimization.Reason = "系统负载中等，建议30分钟后执行"
		optimization.WaitMinutes = 30
	} else {
		// 负载高，建议延迟执行
		optimization.RecommendedTime = time.Now().Add(2 * time.Hour)
		optimization.Reason = "系统负载较高，建议2小时后执行"
		optimization.WaitMinutes = 120
	}

	return optimization, nil
}

// ==================== 统计 ====================

// GetStats 获取备份统计
func (m *Manager) GetStats() *BackupStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &BackupStats{}
	stats.TotalPolicies = len(m.policies)

	for _, p := range m.policies {
		if p.Status == BackupStatusRunning || p.Status == BackupStatusPending {
			stats.ActivePolicies++
		}
		stats.AvgHealthScore += p.HealthScore
	}

	if stats.TotalPolicies > 0 {
		stats.AvgHealthScore /= float64(stats.TotalPolicies)
	}

	stats.TotalExecutions = len(m.executions)
	for _, exec := range m.executions {
		stats.TotalSizeBytes += exec.SizeBytes
		if exec.Status == BackupStatusSuccess {
			stats.SuccessfulBackups++
		} else if exec.Status == BackupStatusFailed {
			stats.FailedBackups++
		}
	}

	// 计算3-2-1合规率
	compliantPolicies := 0
	for _, p := range m.policies {
		if len(p.TargetIDs) >= 3 {
			compliantPolicies++
		}
	}
	if stats.TotalPolicies > 0 {
		stats.ComplianceRate = float64(compliantPolicies) / float64(stats.TotalPolicies) * 100
	}

	return stats
}

// ==================== 内部辅助方法 ====================

func (m *Manager) validatePolicy(policy *BackupPolicy) error {
	if policy.Name == "" {
		return fmt.Errorf("policy name is required")
	}
	if len(policy.SourcePaths) == 0 {
		return fmt.Errorf("at least one source path is required")
	}
	return nil
}

func (m *Manager) validateTarget(target *BackupTarget) error {
	if target.Name == "" {
		return fmt.Errorf("target name is required")
	}
	if target.Type == "" {
		return fmt.Errorf("target type is required")
	}
	switch target.Type {
	case TargetTypeLocal:
		if target.LocalPath == "" {
			return fmt.Errorf("local path is required for local target")
		}
	case TargetTypeRemoteNAS:
		if target.Endpoint == "" {
			return fmt.Errorf("endpoint is required for remote NAS target")
		}
	case TargetTypeS3:
		if target.Endpoint == "" || target.Bucket == "" {
			return fmt.Errorf("endpoint and bucket are required for S3 target")
		}
	default:
		return fmt.Errorf("unsupported target type: %s", target.Type)
	}
	return nil
}

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

func (m *Manager) estimateBackupSize(analysis *StrategyAnalysis) float64 {
	if analysis.ChangeFrequency == nil {
		return analysis.DataSizeGB * 1024 // MB
	}
	return float64(analysis.ChangeFrequency.DailyChanges)
}

func (m *Manager) estimateBackupDuration(analysis *StrategyAnalysis) float64 {
	sizeMB := m.estimateBackupSize(analysis)
	return sizeMB / 100 / 60 // 分钟
}

func (m *Manager) checkRPOFeasibility(analysis *StrategyAnalysis) bool {
	if analysis.Requirements == nil {
		return true
	}
	return analysis.Requirements.MaxDataLoss >= time.Hour
}

func (m *Manager) checkRTOFeasibility(analysis *StrategyAnalysis) bool {
	if analysis.RTORequirements == nil {
		return true
	}
	return analysis.RTORequirements.MaxRecoveryTime >= 30*time.Minute
}

func (m *Manager) checkThreeTwoOneCompliance(analysis *StrategyAnalysis) *ThreeTwoOneCompliance {
	compliance := &ThreeTwoOneCompliance{}

	// 检查目标数量
	if analysis.TargetCount >= 3 {
		compliance.TotalCopies = 3
	} else {
		compliance.TotalCopies = analysis.TargetCount
	}

	// 模拟检查介质类型（实际应从目标信息中获取）
	compliance.MediaTypes = 2
	compliance.OffsiteCopies = 1

	compliance.Compliant = compliance.TotalCopies >= 3 &&
		compliance.MediaTypes >= 2 &&
		compliance.OffsiteCopies >= 1

	return compliance
}

func (m *Manager) generateRecommendations(analysis *StrategyAnalysis, strategy *BackupStrategy) []string {
	recommendations := make([]string, 0)

	if strategy.RecommendedType == BackupTypeFull {
		recommendations = append(recommendations, "建议每周执行一次全量备份")
		recommendations = append(recommendations, "每日执行增量备份以减少备份时间")
	}

	if analysis.DataSizeGB > 100 {
		recommendations = append(recommendations, "大数据量建议使用增量备份减少备份窗口")
	}

	if strategy.ThreeTwoOne != nil && !strategy.ThreeTwoOne.Compliant {
		recommendations = append(recommendations, "建议增加备份目标以满足3-2-1规则")
	}

	return recommendations
}

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

func (m *Manager) findOptimalWindow(peakHours []int) (int, int) {
	peakMap := make(map[int]bool)
	for _, h := range peakHours {
		peakMap[h] = true
	}

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

func (m *Manager) updateBackupChain(execution *BackupExecution) {
	chain, ok := m.chains[execution.ChainID]
	if !ok {
		return
	}

	if execution.BackupType == BackupTypeFull {
		chain.FullBackup = execution
	} else {
		chain.Incremental = append(chain.Incremental, execution)
	}

	chain.ChainLength = 1 + len(chain.Incremental)
	chain.TotalSize += execution.SizeBytes
	chain.UpdatedAt = time.Now()

	// 更新链路健康评分
	m.updateChainHealthScore(chain)
}

func (m *Manager) updateChainHealthScore(chain *BackupChain) {
	if chain.FullBackup == nil {
		chain.HealthScore = 0
		return
	}

	score := 100.0

	// 检查全量备份状态
	if chain.FullBackup.Status != BackupStatusSuccess {
		score -= 30
	}

	// 检查增量备份
	failedCount := 0
	for _, inc := range chain.Incremental {
		if inc.Status == BackupStatusFailed {
			failedCount++
		}
	}
	if len(chain.Incremental) > 0 {
		failRate := float64(failedCount) / float64(len(chain.Incremental))
		score -= failRate * 50
	}

	// 链路过长扣分
	if chain.ChainLength > 10 {
		score -= float64(chain.ChainLength-10) * 2
	}

	chain.HealthScore = math.Max(0, math.Min(100, score))
}

func (m *Manager) updatePolicyHealthScore(policy *BackupPolicy) {
	totalScore := 0.0
	count := 0

	for _, chain := range m.chains {
		if chain.PolicyID == policy.ID {
			totalScore += chain.HealthScore
			count++
		}
	}

	if count > 0 {
		policy.HealthScore = totalScore / float64(count)
	}
}

func (m *Manager) calculateLoadScore(metrics *LoadMetrics) float64 {
	// CPU 占 40%，内存占 30%，磁盘IO占 30%
	cpuScore := (100 - metrics.CPUPercent) * 0.4
	memScore := (100 - metrics.MemoryPercent) * 0.3
	diskScore := (100 - metrics.DiskIOPercent) * 0.3

	return cpuScore + memScore + diskScore
}
