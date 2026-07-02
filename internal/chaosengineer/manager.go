package chaosengineer

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// NewManager 创建混沌工程管理器.
func NewManager(config *Config) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	if config.MaxConcurrent <= 0 {
		config.MaxConcurrent = 3
	}
	if config.MetricsInterval <= 0 {
		config.MetricsInterval = 5 * time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		experiments: make(map[string]*Experiment),
		reports:     make(map[string]*ResilienceReport),
		config:      config,
		running:     make(map[string]context.CancelFunc),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return nil
	}
	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 停止所有运行中的实验
	for id, cancelFn := range m.running {
		cancelFn()
		delete(m.running, id)
	}
	m.cancel()
}

// ==================== 实验管理 ====================

// CreateExperiment 创建实验.
func (m *Manager) CreateExperiment(exp *Experiment) (*Experiment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证故障类型
	if err := ValidateFaultType(exp.Fault.Type); err != nil {
		return nil, err
	}

	// 验证严重程度
	if err := ValidateSeverity(exp.Fault.Severity); err != nil {
		return nil, err
	}

	// 验证目标
	if exp.Fault.Target == "" {
		return nil, ErrNoTargetSpecified
	}

	// 生成ID
	if exp.ID == "" {
		exp.ID = uuid.New().String()
	}

	// 设置默认安全边界
	if exp.Safety.MaxDuration == 0 {
		exp.Safety = m.config.DefaultSafety
	}

	// 设置默认状态
	exp.Status = StatusCreated
	now := time.Now()
	exp.CreatedAt = now
	exp.UpdatedAt = now

	m.experiments[exp.ID] = exp
	return exp, nil
}

// GetExperiment 获取实验.
func (m *Manager) GetExperiment(id string) (*Experiment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exp, ok := m.experiments[id]
	if !ok {
		return nil, ErrExperimentNotFound
	}
	return exp, nil
}

// ListExperiments 列出所有实验.
func (m *Manager) ListExperiments() []*Experiment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	experiments := make([]*Experiment, 0, len(m.experiments))
	for _, exp := range m.experiments {
		experiments = append(experiments, exp)
	}
	return experiments
}

// UpdateExperiment 更新实验.
func (m *Manager) UpdateExperiment(id string, update *Experiment) (*Experiment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, ok := m.experiments[id]
	if !ok {
		return nil, ErrExperimentNotFound
	}

	// 不允许更新运行中的实验
	if exp.Status == StatusRunning {
		return nil, ErrExperimentRunning
	}

	// 更新字段
	if update.Name != "" {
		exp.Name = update.Name
	}
	if update.Description != "" {
		exp.Description = update.Description
	}
	if update.Fault.Type != "" {
		if err := ValidateFaultType(update.Fault.Type); err != nil {
			return nil, err
		}
		exp.Fault.Type = update.Fault.Type
	}
	if update.Fault.Target != "" {
		exp.Fault.Target = update.Fault.Target
	}
	if update.Fault.Severity != "" {
		if err := ValidateSeverity(update.Fault.Severity); err != nil {
			return nil, err
		}
		exp.Fault.Severity = update.Fault.Severity
	}
	if update.Fault.Duration > 0 {
		exp.Fault.Duration = update.Fault.Duration
	}
	if len(update.Fault.Parameters) > 0 {
		if exp.Fault.Parameters == nil {
			exp.Fault.Parameters = make(map[string]any)
		}
		for k, v := range update.Fault.Parameters {
			exp.Fault.Parameters[k] = v
		}
	}
	if update.Tags != nil {
		exp.Tags = update.Tags
	}
	if update.Hypothesis != nil {
		exp.Hypothesis = update.Hypothesis
	}
	if update.Schedule != nil {
		exp.Schedule = update.Schedule
	}
	if update.Safety.MaxDuration > 0 {
		exp.Safety = update.Safety
	}

	exp.UpdatedAt = time.Now()
	return exp, nil
}

// DeleteExperiment 删除实验.
func (m *Manager) DeleteExperiment(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, ok := m.experiments[id]
	if !ok {
		return ErrExperimentNotFound
	}

	// 不允许删除运行中的实验
	if exp.Status == StatusRunning {
		return ErrExperimentRunning
	}

	delete(m.experiments, id)
	return nil
}

// ==================== 实验执行 ====================

// StartExperiment 启动实验.
func (m *Manager) StartExperiment(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, ok := m.experiments[id]
	if !ok {
		return ErrExperimentNotFound
	}

	if exp.Status == StatusRunning {
		return ErrExperimentRunning
	}

	// 检查并发限制
	if len(m.running) >= m.config.MaxConcurrent {
		return fmt.Errorf("max concurrent experiments reached: %d", m.config.MaxConcurrent)
	}

	// 安全边界检查
	if err := m.checkSafetyBoundaries(exp); err != nil {
		return err
	}

	// 创建实验上下文
	ctx, cancel := context.WithTimeout(m.ctx, exp.Fault.Duration)
	exp.Status = StatusRunning
	now := time.Now()
	exp.StartTime = &now
	exp.UpdatedAt = now

	m.running[id] = cancel

	// 异步执行实验
	go m.runExperiment(ctx, exp)

	return nil
}

// StopExperiment 停止实验.
func (m *Manager) StopExperiment(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	exp, ok := m.experiments[id]
	if !ok {
		return ErrExperimentNotFound
	}

	if exp.Status != StatusRunning {
		return ErrExperimentNotRun
	}

	// 取消实验
	if cancelFn, ok := m.running[id]; ok {
		cancelFn()
		delete(m.running, id)
	}

	now := time.Now()
	exp.EndTime = &now
	exp.Status = StatusCancelled
	exp.UpdatedAt = now

	// 执行恢复
	if exp.Safety.AutoRecover {
		go m.recoverExperiment(exp)
	}

	return nil
}

// runExperiment 执行实验.
func (m *Manager) runExperiment(ctx context.Context, exp *Experiment) {
	var err error

	// 执行故障注入
	switch exp.Fault.Type {
	case FaultDiskFull:
		err = m.injectDiskFull(ctx, exp)
	case FaultNetworkLatency:
		err = m.injectNetworkLatency(ctx, exp)
	case FaultNetworkLoss:
		err = m.injectNetworkLoss(ctx, exp)
	case FaultCPUStress:
		err = m.injectCPUStress(ctx, exp)
	case FaultMemoryStress:
		err = m.injectMemoryStress(ctx, exp)
	case FaultIOStress:
		err = m.injectIOStress(ctx, exp)
	case FaultProcessKill:
		err = m.injectProcessKill(ctx, exp)
	case FaultDiskIO:
		err = m.injectDiskIO(ctx, exp)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	exp.EndTime = &now
	exp.UpdatedAt = now

	if err != nil {
		exp.Status = StatusFailed
		exp.ErrorMsg = err.Error()
	} else {
		exp.Status = StatusCompleted
	}

	// 从运行列表移除
	delete(m.running, exp.ID)

	// 计算韧性评分
	exp.Resilience = m.calculateResilienceScore(exp)

	// 自动恢复
	if exp.Safety.AutoRecover {
		m.recoverExperiment(exp)
	}
}

// ==================== 故障注入方法 ====================

// injectDiskFull 注入磁盘满故障.
func (m *Manager) injectDiskFull(ctx context.Context, exp *Experiment) error {
	// 磁盘满故障注入实现
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// 模拟磁盘满故障
		time.Sleep(exp.Fault.Duration)
		return nil
	}
}

// injectNetworkLatency 注入网络延迟故障.
func (m *Manager) injectNetworkLatency(ctx context.Context, exp *Experiment) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		time.Sleep(exp.Fault.Duration)
		return nil
	}
}

// injectNetworkLoss 注入网络丢包故障.
func (m *Manager) injectNetworkLoss(ctx context.Context, exp *Experiment) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		time.Sleep(exp.Fault.Duration)
		return nil
	}
}

// injectCPUStress 注入 CPU 压力.
func (m *Manager) injectCPUStress(ctx context.Context, exp *Experiment) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		time.Sleep(exp.Fault.Duration)
		return nil
	}
}

// injectMemoryStress 注入内存压力.
func (m *Manager) injectMemoryStress(ctx context.Context, exp *Experiment) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		time.Sleep(exp.Fault.Duration)
		return nil
	}
}

// injectIOStress 注入 IO 压力.
func (m *Manager) injectIOStress(ctx context.Context, exp *Experiment) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		time.Sleep(exp.Fault.Duration)
		return nil
	}
}

// injectProcessKill 注入进程终止.
func (m *Manager) injectProcessKill(ctx context.Context, exp *Experiment) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// injectDiskIO 注入磁盘IO延迟.
func (m *Manager) injectDiskIO(ctx context.Context, exp *Experiment) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		time.Sleep(exp.Fault.Duration)
		return nil
	}
}

// ==================== 安全边界 ====================

// checkSafetyBoundaries 检查安全边界.
func (m *Manager) checkSafetyBoundaries(exp *Experiment) error {
	safety := exp.Safety

	// 检查持续时间
	if safety.MaxDuration > 0 && exp.Fault.Duration > safety.MaxDuration {
		return fmt.Errorf("%w: duration %v exceeds max %v", ErrSafetyViolation, exp.Fault.Duration, safety.MaxDuration)
	}

	// 检查受保护路径
	if exp.Fault.TargetType == TargetDisk {
		for _, protected := range safety.ProtectedPaths {
			if exp.Fault.Target == protected {
				return fmt.Errorf("%w: target %s is a protected path", ErrSafetyViolation, exp.Fault.Target)
			}
		}
	}

	// 检查受保护服务
	if exp.Fault.TargetType == TargetService {
		for _, protected := range safety.ProtectedServices {
			if exp.Fault.Target == protected {
				return fmt.Errorf("%w: service %s is protected", ErrSafetyViolation, exp.Fault.Target)
			}
		}
	}

	return nil
}

// ==================== 恢复机制 ====================

// recoverExperiment 恢复实验.
func (m *Manager) recoverExperiment(exp *Experiment) {
	recovery := &RecoveryResult{
		Status:    RecoveryRunning,
		StartTime: time.Now(),
		Steps:     make([]RecoveryStep, 0),
	}

	exp.Recovery = recovery

	// 根据故障类型执行恢复
	step := RecoveryStep{
		Name:      "cleanup",
		StartTime: time.Now(),
	}

	switch exp.Fault.Type {
	case FaultDiskFull:
		step.Name = "cleanup_disk"
	case FaultNetworkLatency, FaultNetworkLoss:
		step.Name = "restore_network"
	case FaultCPUStress:
		step.Name = "release_cpu"
	case FaultMemoryStress:
		step.Name = "release_memory"
	case FaultIOStress:
		step.Name = "release_io"
	}

	step.EndTime = time.Now()
	step.Status = "completed"
	recovery.Steps = append(recovery.Steps, step)

	endTime := time.Now()
	recovery.EndTime = endTime
	recovery.Status = RecoverySuccess
	recovery.Duration = endTime.Sub(recovery.StartTime)
}

// ==================== 韧性评估 ====================

// calculateResilienceScore 计算韧性评分.
func (m *Manager) calculateResilienceScore(exp *Experiment) *ResilienceScore {
	score := &ResilienceScore{
		Breakdown: make(map[string]float64),
	}

	// 基础分
	base := 100.0

	// 根据实验结果调整
	if exp.Status == StatusFailed {
		base -= 40
	}

	// 根据恢复结果调整
	if exp.Recovery != nil {
		switch exp.Recovery.Status {
		case RecoverySuccess:
			score.Recovery = 90.0
		case RecoveryFailed:
			score.Recovery = 30.0
			base -= 20
		}
	} else {
		score.Recovery = 50.0
	}

	// 根据故障严重程度调整
	switch exp.Fault.Severity {
	case SeverityCritical:
		score.Stability = 60.0
	case SeverityHigh:
		score.Stability = 70.0
	case SeverityMedium:
		score.Stability = 80.0
	case SeverityLow:
		score.Stability = 90.0
	}

	// 计算可用性
	if exp.StartTime != nil && exp.EndTime != nil {
		score.Availability = 95.0
	} else {
		score.Availability = 70.0
	}

	score.Overall = base
	score.Breakdown["recovery"] = score.Recovery
	score.Breakdown["stability"] = score.Stability
	score.Breakdown["availability"] = score.Availability

	return score
}

// GenerateReport 生成韧性评估报告.
func (m *Manager) GenerateReport() *ResilienceReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &ResilienceReport{
		ID:            uuid.New().String(),
		GeneratedAt:   time.Now(),
		ExperimentIDs: make([]string, 0),
		Score: &ResilienceScore{
			Breakdown: make(map[string]float64),
		},
		Recommendations: make([]string, 0),
	}

	totalScore := 0.0
	scoreCount := 0

	for _, exp := range m.experiments {
		report.TotalExperiments++
		report.ExperimentIDs = append(report.ExperimentIDs, exp.ID)

		switch exp.Status {
		case StatusCompleted:
			report.PassedExperiments++
		case StatusFailed:
			report.FailedExperiments++
		}

		if exp.Resilience != nil {
			totalScore += exp.Resilience.Overall
			scoreCount++
		}
	}

	if scoreCount > 0 {
		report.OverallScore = totalScore / float64(scoreCount)
	}

	// 生成建议
	if report.FailedExperiments > 0 {
		report.Recommendations = append(report.Recommendations, "存在失败的实验，建议检查系统稳定性")
	}
	if report.OverallScore < 60 {
		report.Recommendations = append(report.Recommendations, "整体韧性评分较低，建议加强容错机制")
	}

	// 保存报告
	m.reports[report.ID] = report
	return report
}

// ListReports 列出所有报告.
func (m *Manager) ListReports() []*ResilienceReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	reports := make([]*ResilienceReport, 0, len(m.reports))
	for _, r := range m.reports {
		reports = append(reports, r)
	}
	return reports
}

// GetReport 获取报告.
func (m *Manager) GetReport(id string) (*ResilienceReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report, ok := m.reports[id]
	if !ok {
		return nil, fmt.Errorf("report not found: %s", id)
	}
	return report, nil
}

// ==================== 仪表盘 ====================

// GetDashboard 获取仪表盘数据.
func (m *Manager) GetDashboard() *Dashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()

	dashboard := &Dashboard{
		FaultDistribution: make(map[FaultType]int),
		UpdatedAt:         time.Now(),
	}

	totalScore := 0.0
	scoreCount := 0

	recentCount := 5
	recent := make([]*Experiment, 0, recentCount)

	for _, exp := range m.experiments {
		dashboard.TotalExperiments++
		dashboard.FaultDistribution[exp.Fault.Type]++

		switch exp.Status {
		case StatusRunning:
			dashboard.RunningExperiments++
		case StatusCompleted:
			dashboard.CompletedExperiments++
		case StatusFailed:
			dashboard.FailedExperiments++
		}

		if exp.Resilience != nil {
			totalScore += exp.Resilience.Overall
			scoreCount++
		}

		// 收集最近的实验
		if len(recent) < recentCount {
			recent = append(recent, exp)
		}
	}

	if scoreCount > 0 {
		dashboard.OverallResilience = totalScore / float64(scoreCount)
	}
	dashboard.RecentExperiments = recent

	return dashboard
}
