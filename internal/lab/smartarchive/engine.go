package smartarchive

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Engine 归档引擎核心.
type Engine struct {
	mu sync.RWMutex

	// 配置
	config   EngineConfig
	tiers    map[StorageTier]*StorageTierConfig
	policies map[string]*ArchivePolicy
	rules    map[string]*RetentionRule

	// 子组件
	analyzer  *Analyzer
	scheduler *Scheduler
	tierMgr   *TierManager
	costMgr   *CostManager

	// 任务管理
	jobs     map[string]*ArchiveJob
	records  map[string]*ArchiveRecord
	auditLog []AuditEntry

	// 运行状态
	running bool
	ctx     context.Context
	cancel  context.CancelFunc

	// 回调
	onJobComplete    func(job *ArchiveJob)
	onPolicyTrigger  func(policy *ArchivePolicy)
	onRetentionAlert func(rule *RetentionRule, files []string)
}

// EngineConfig 引擎配置.
type EngineConfig struct {
	// 基本配置
	DataRoot    string `json:"dataRoot"`    // 数据根目录
	TempDir     string `json:"tempDir"`     // 临时目录
	WorkerCount int    `json:"workerCount"` // 工作线程数
	BatchSize   int    `json:"batchSize"`   // 批处理大小

	// 高级配置
	EnableAutoArchive  bool `json:"enableAutoArchive"`  // 启用自动归档
	EnableCompression  bool `json:"enableCompression"`  // 启用压缩
	EnableDedup        bool `json:"enableDedup"`        // 启用去重
	EnableRetention    bool `json:"enableRetention"`    // 启用保留策略
	EnableCostAnalysis bool `json:"enableCostAnalysis"` // 启用成本分析

	// 安全配置
	DryRun         bool `json:"dryRun"`         // 试运行模式
	VerifyChecksum bool `json:"verifyChecksum"` // 验证校验和
	EncryptArchive bool `json:"encryptArchive"` // 加密归档

	// 分析配置
	AnalyzerConfig  AnalyzerConfig  `json:"analyzerConfig"`
	SchedulerConfig SchedulerConfig `json:"schedulerConfig"`
}

// DefaultEngineConfig 默认引擎配置.
func DefaultEngineConfig() EngineConfig {
	return EngineConfig{
		DataRoot:           "/data",
		TempDir:            "/tmp/smartarchive",
		WorkerCount:        4,
		BatchSize:          100,
		EnableAutoArchive:  true,
		EnableCompression:  true,
		EnableDedup:        false,
		EnableRetention:    true,
		EnableCostAnalysis: true,
		DryRun:             false,
		VerifyChecksum:     true,
		EncryptArchive:     false,
		AnalyzerConfig:     DefaultAnalyzerConfig(),
		SchedulerConfig:    DefaultSchedulerConfig(),
	}
}

// NewEngine 创建归档引擎.
func NewEngine(config EngineConfig) *Engine {
	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		config:   config,
		tiers:    make(map[StorageTier]*StorageTierConfig),
		policies: make(map[string]*ArchivePolicy),
		rules:    make(map[string]*RetentionRule),
		jobs:     make(map[string]*ArchiveJob),
		records:  make(map[string]*ArchiveRecord),
		auditLog: make([]AuditEntry, 0),
		ctx:      ctx,
		cancel:   cancel,
	}

	// 初始化子组件
	e.analyzer = NewAnalyzer(config.AnalyzerConfig)
	e.scheduler = NewScheduler(config.SchedulerConfig)
	e.tierMgr = NewTierManager()
	e.costMgr = NewCostManager()

	// 初始化默认存储层
	e.initDefaultTiers()

	return e
}

// initDefaultTiers 初始化默认存储层配置.
func (e *Engine) initDefaultTiers() {
	e.tiers = map[StorageTier]*StorageTierConfig{
		TierHot: {
			Tier:          TierHot,
			Name:          "热数据层（NVMe SSD）",
			Path:          "/data/hot",
			Capacity:      1024 * 1024 * 1024 * 500, // 500GB
			Threshold:     80,
			CostPerGB:     0.5,
			IOPSMax:       100000,
			ThroughputMax: 3500,
			Enabled:       true,
			Encrypted:     true,
			Compressed:    false,
			Redundancy:    1,
		},
		TierWarm: {
			Tier:          TierWarm,
			Name:          "温数据层（SATA SSD）",
			Path:          "/data/warm",
			Capacity:      1024 * 1024 * 1024 * 2000, // 2TB
			Threshold:     85,
			CostPerGB:     0.2,
			IOPSMax:       50000,
			ThroughputMax: 550,
			Enabled:       true,
			Encrypted:     true,
			Compressed:    false,
			Redundancy:    1,
		},
		TierCold: {
			Tier:          TierCold,
			Name:          "冷数据层（HDD）",
			Path:          "/data/cold",
			Capacity:      1024 * 1024 * 1024 * 8000, // 8TB
			Threshold:     90,
			CostPerGB:     0.05,
			IOPSMax:       200,
			ThroughputMax: 200,
			Enabled:       true,
			Encrypted:     true,
			Compressed:    true,
			Redundancy:    2,
		},
		TierIce: {
			Tier:          TierIce,
			Name:          "冰冻层（归档存储）",
			Path:          "/data/ice",
			Capacity:      1024 * 1024 * 1024 * 20000, // 20TB
			Threshold:     95,
			CostPerGB:     0.01,
			IOPSMax:       10,
			ThroughputMax: 50,
			Enabled:       true,
			Encrypted:     true,
			Compressed:    true,
			Redundancy:    3,
		},
	}
}

// Start 启动引擎.
func (e *Engine) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.running {
		return fmt.Errorf("引擎已在运行中")
	}

	log.Println("[SmartArchive] 启动归档引擎...")

	// 启动分析器
	if err := e.analyzer.Start(); err != nil {
		return fmt.Errorf("启动分析器失败: %w", err)
	}

	// 启动调度器
	e.scheduler.Start()

	// 启动策略引擎
	if e.config.EnableAutoArchive {
		go e.runPolicyEngine()
	}

	// 启动保留策略检查
	if e.config.EnableRetention {
		go e.runRetentionChecker()
	}

	// 启动成本分析
	if e.config.EnableCostAnalysis {
		go e.runCostAnalyzer()
	}

	e.running = true
	e.addAuditEntry("engine", "system", "engine_start", "归档引擎启动", "success")

	log.Println("[SmartArchive] 归档引擎已启动")
	return nil
}

// Stop 停止引擎.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}

	log.Println("[SmartArchive] 停止归档引擎...")

	// 取消上下文
	e.cancel()

	// 停止子组件
	e.analyzer.Stop()
	e.scheduler.Stop()

	e.running = false
	e.addAuditEntry("engine", "system", "engine_stop", "归档引擎停止", "success")

	log.Println("[SmartArchive] 归档引擎已停止")
}

// AddPolicy 添加归档策略.
func (e *Engine) AddPolicy(policy *ArchivePolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if policy.ID == "" {
		return fmt.Errorf("策略 ID 不能为空")
	}

	if _, exists := e.policies[policy.ID]; exists {
		return fmt.Errorf("策略 %s 已存在", policy.ID)
	}

	now := time.Now()
	policy.CreatedAt = now
	policy.UpdatedAt = now

	e.policies[policy.ID] = policy
	e.addAuditEntry("policy", policy.ID, "policy_created",
		fmt.Sprintf("创建归档策略: %s", policy.Name), "success")

	log.Printf("[SmartArchive] 添加归档策略: %s (%s)", policy.Name, policy.ID)
	return nil
}

// UpdatePolicy 更新归档策略.
func (e *Engine) UpdatePolicy(policy *ArchivePolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.policies[policy.ID]; !exists {
		return fmt.Errorf("策略 %s 不存在", policy.ID)
	}

	policy.UpdatedAt = time.Now()
	e.policies[policy.ID] = policy
	e.addAuditEntry("policy", policy.ID, "policy_updated",
		fmt.Sprintf("更新归档策略: %s", policy.Name), "success")

	return nil
}

// RemovePolicy 删除归档策略.
func (e *Engine) RemovePolicy(policyID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	policy, exists := e.policies[policyID]
	if !exists {
		return fmt.Errorf("策略 %s 不存在", policyID)
	}

	delete(e.policies, policyID)
	e.addAuditEntry("policy", policyID, "policy_deleted",
		fmt.Sprintf("删除归档策略: %s", policy.Name), "success")

	return nil
}

// GetPolicy 获取归档策略.
func (e *Engine) GetPolicy(policyID string) (*ArchivePolicy, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policy, exists := e.policies[policyID]
	if !exists {
		return nil, fmt.Errorf("策略 %s 不存在", policyID)
	}

	return policy, nil
}

// ListPolicies 列出所有归档策略.
func (e *Engine) ListPolicies() []*ArchivePolicy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policies := make([]*ArchivePolicy, 0, len(e.policies))
	for _, p := range e.policies {
		policies = append(policies, p)
	}

	return policies
}

// AddRetentionRule 添加保留规则.
func (e *Engine) AddRetentionRule(rule *RetentionRule) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("规则 ID 不能为空")
	}

	if _, exists := e.rules[rule.ID]; exists {
		return fmt.Errorf("规则 %s 已存在", rule.ID)
	}

	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	e.rules[rule.ID] = rule
	e.addAuditEntry("retention", rule.ID, "rule_created",
		fmt.Sprintf("创建保留规则: %s", rule.Name), "success")

	return nil
}

// RunManualArchive 手动触发归档.
func (e *Engine) RunManualArchive(policyID string, paths []string) (*ArchiveJob, error) {
	e.mu.RLock()
	policy, exists := e.policies[policyID]
	e.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("策略 %s 不存在", policyID)
	}

	job := &ArchiveJob{
		ID:          generateID(),
		PolicyID:    policyID,
		PolicyName:  policy.Name,
		Status:      JobStatusPending,
		Action:      policy.Action,
		SourceTier:  e.getSourceTier(policy),
		TargetTier:  policy.TargetTier,
		Compression: policy.Compression,
		CreatedAt:   time.Now(),
	}

	e.mu.Lock()
	e.jobs[job.ID] = job
	e.mu.Unlock()

	// 异步执行
	go e.executeJob(job, paths)

	return job, nil
}

// GetJob 获取归档任务.
func (e *Engine) GetJob(jobID string) (*ArchiveJob, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	job, exists := e.jobs[jobID]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", jobID)
	}

	return job, nil
}

// ListJobs 列出归档任务.
func (e *Engine) ListJobs(status JobStatus, limit int) []*ArchiveJob {
	e.mu.RLock()
	defer e.mu.RUnlock()

	jobs := make([]*ArchiveJob, 0)
	for _, j := range e.jobs {
		if status != "" && j.Status != status {
			continue
		}
		jobs = append(jobs, j)
	}

	// 按创建时间排序（最新的在前）
	sortJobs(jobs)

	if limit > 0 && len(jobs) > limit {
		jobs = jobs[:limit]
	}

	return jobs
}

// CancelJob 取消归档任务.
func (e *Engine) CancelJob(jobID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	job, exists := e.jobs[jobID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", jobID)
	}

	if job.Status != JobStatusPending && job.Status != JobStatusRunning {
		return fmt.Errorf("任务 %s 状态为 %s，无法取消", jobID, job.Status)
	}

	job.Status = JobStatusCancelled
	now := time.Now()
	job.CompletedAt = now

	e.addAuditEntry("job", jobID, "job_cancelled",
		fmt.Sprintf("取消归档任务: %s", jobID), "success")

	return nil
}

// GetArchiveRecords 获取归档记录.
func (e *Engine) GetArchiveRecords(filePath string, limit int) []*ArchiveRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()

	records := make([]*ArchiveRecord, 0)
	for _, r := range e.records {
		if filePath != "" && r.FilePath != filePath {
			continue
		}
		records = append(records, r)
	}

	if limit > 0 && len(records) > limit {
		records = records[:limit]
	}

	return records
}

// GetAuditLog 获取审计日志.
func (e *Engine) GetAuditLog(eventType string, limit int) []AuditEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	entries := make([]AuditEntry, 0)
	for _, entry := range e.auditLog {
		if eventType != "" && entry.EventType != eventType {
			continue
		}
		entries = append(entries, entry)
	}

	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	return entries
}

// GetSummary 获取归档摘要.
func (e *Engine) GetSummary() *ArchiveSummary {
	e.mu.RLock()
	defer e.mu.RUnlock()

	summary := &ArchiveSummary{
		GeneratedAt: time.Now(),
		TierStats:   make(map[StorageTier]*TierArchiveStats),
		PolicyStats: make(map[string]*PolicyArchiveStats),
		RecentJobs:  make([]*ArchiveJob, 0),
		Issues:      make([]string, 0),
	}

	// 统计任务
	for _, job := range e.jobs {
		switch job.Status {
		case JobStatusCompleted:
			summary.TotalArchived += job.ProcessedFiles
			summary.TotalSize += job.ProcessedBytes
			summary.SavedSpace += job.OriginalBytes - job.ProcessedBytes
		}
	}

	// 统计层级
	for tier, config := range e.tiers {
		summary.TierStats[tier] = &TierArchiveStats{
			Tier:         tier,
			TotalFiles:   0,
			TotalSize:    0,
			Utilization:  float64(config.Used) / float64(config.Capacity) * 100,
			CostPerMonth: float64(config.Used) / (1024 * 1024 * 1024) * config.CostPerGB,
		}
	}

	// 计算健康分数
	summary.HealthScore = e.calculateHealthScore()
	summary.Issues = e.collectIssues()

	return summary
}

// runPolicyEngine 策略引擎主循环.
func (e *Engine) runPolicyEngine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.evaluatePolicies()
		}
	}
}

// evaluatePolicies 评估所有策略.
func (e *Engine) evaluatePolicies() {
	e.mu.RLock()
	policies := make([]*ArchivePolicy, 0, len(e.policies))
	for _, p := range e.policies {
		if p.Enabled {
			policies = append(policies, p)
		}
	}
	e.mu.RUnlock()

	for _, policy := range policies {
		if e.shouldExecutePolicy(policy) {
			log.Printf("[SmartArchive] 触发策略: %s", policy.Name)
			if e.onPolicyTrigger != nil {
				e.onPolicyTrigger(policy)
			}
			// 执行策略
			go e.executePolicy(policy)
		}
	}
}

// shouldExecutePolicy 判断策略是否应该执行.
func (e *Engine) shouldExecutePolicy(policy *ArchivePolicy) bool {
	// 检查调度时间
	if policy.Schedule != "" {
		// 简化的调度检查
		return true
	}

	// 检查条件
	// 实际实现需要检查文件系统状态
	return true
}

// executePolicy 执行归档策略.
func (e *Engine) executePolicy(policy *ArchivePolicy) {
	// 创建任务
	job := &ArchiveJob{
		ID:          generateID(),
		PolicyID:    policy.ID,
		PolicyName:  policy.Name,
		Status:      JobStatusRunning,
		Action:      policy.Action,
		TargetTier:  policy.TargetTier,
		Compression: policy.Compression,
		CreatedAt:   time.Now(),
		StartedAt:   time.Now(),
	}

	e.mu.Lock()
	e.jobs[job.ID] = job
	e.mu.Unlock()

	// 扫描符合条件的文件
	// 这里是简化实现，实际需要遍历文件系统
	log.Printf("[SmartArchive] 执行策略 %s，任务 %s", policy.Name, job.ID)

	// 完成任务
	job.Status = JobStatusCompleted
	now := time.Now()
	job.CompletedAt = now

	e.addAuditEntry("job", job.ID, "job_completed",
		fmt.Sprintf("完成归档任务: %s", job.PolicyName), "success")

	if e.onJobComplete != nil {
		e.onJobComplete(job)
	}
}

// executeJob 执行归档任务.
func (e *Engine) executeJob(job *ArchiveJob, paths []string) {
	e.mu.Lock()
	job.Status = JobStatusRunning
	now := time.Now()
	job.StartedAt = now
	e.mu.Unlock()

	// 处理路径
	for _, path := range paths {
		e.processPath(job, path)
	}

	// 完成任务
	e.mu.Lock()
	job.Status = JobStatusCompleted
	completedAt := time.Now()
	job.CompletedAt = completedAt
	e.mu.Unlock()

	e.addAuditEntry("job", job.ID, "job_completed",
		fmt.Sprintf("完成手动归档任务，处理 %d 个文件", job.ProcessedFiles), "success")
}

// processPath 处理单个路径.
func (e *Engine) processPath(job *ArchiveJob, path string) {
	// 简化实现：记录处理
	job.TotalFiles++
	job.ProcessedFiles++

	// 创建归档记录
	record := &ArchiveRecord{
		ID:         generateID(),
		JobID:      job.ID,
		FilePath:   path,
		Action:     job.Action,
		SourceTier: job.SourceTier,
		TargetTier: job.TargetTier,
		ArchivedAt: time.Now(),
		Status:     "completed",
	}

	e.mu.Lock()
	e.records[record.ID] = record
	e.mu.Unlock()
}

// runRetentionChecker 保留策略检查器.
func (e *Engine) runRetentionChecker() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.checkRetentionRules()
		}
	}
}

// checkRetentionRules 检查保留规则.
func (e *Engine) checkRetentionRules() {
	e.mu.RLock()
	rules := make([]*RetentionRule, 0, len(e.rules))
	for _, r := range e.rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	e.mu.RUnlock()

	for _, rule := range rules {
		// 检查规则条件
		// 实际实现需要遍历文件系统检查过期文件
		_ = rule
	}
}

// runCostAnalyzer 成本分析器.
func (e *Engine) runCostAnalyzer() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-e.ctx.Done():
			return
		case <-ticker.C:
			e.analyzeCost()
		}
	}
}

// analyzeCost 分析存储成本.
func (e *Engine) analyzeCost() {
	report := e.costMgr.GenerateReport(e.tiers, e.jobs)
	if report != nil {
		log.Printf("[SmartArchive] 成本报告: 当前月成本 %.2f 元，潜在节省 %.2f 元",
			report.CurrentCost, report.PotentialSaving)
	}
}

// calculateHealthScore 计算健康分数.
func (e *Engine) calculateHealthScore() float64 {
	score := 100.0

	// 检查存储层使用率
	for _, tier := range e.tiers {
		if !tier.Enabled {
			score -= 10
			continue
		}
		usage := float64(tier.Used) / float64(tier.Capacity) * 100
		if usage > float64(tier.Threshold) {
			score -= (usage - float64(tier.Threshold)) * 0.5
		}
	}

	// 检查失败任务
	failedCount := 0
	for _, job := range e.jobs {
		if job.Status == JobStatusFailed {
			failedCount++
		}
	}
	score -= float64(failedCount) * 5

	if score < 0 {
		score = 0
	}

	return score
}

// collectIssues 收集问题.
func (e *Engine) collectIssues() []string {
	issues := make([]string, 0)

	for tier, config := range e.tiers {
		if !config.Enabled {
			issues = append(issues, fmt.Sprintf("存储层 %s 已禁用", tier))
			continue
		}
		usage := float64(config.Used) / float64(config.Capacity) * 100
		if usage > float64(config.Threshold) {
			issues = append(issues, fmt.Sprintf("存储层 %s 使用率 %.1f%% 超过阈值 %d%%",
				tier, usage, config.Threshold))
		}
	}

	return issues
}

// addAuditEntry 添加审计条目.
func (e *Engine) addAuditEntry(eventType, resource, action, details, status string) {
	entry := AuditEntry{
		ID:        generateID(),
		Timestamp: time.Now(),
		EventType: eventType,
		Actor:     "system",
		Resource:  resource,
		Action:    action,
		Details:   details,
		Status:    status,
	}
	e.auditLog = append(e.auditLog, entry)

	// 限制审计日志大小
	if len(e.auditLog) > 10000 {
		e.auditLog = e.auditLog[len(e.auditLog)-10000:]
	}
}

// getSourceTier 获取源存储层.
func (e *Engine) getSourceTier(policy *ArchivePolicy) StorageTier {
	if len(policy.Conditions.SourceTiers) > 0 {
		return policy.Conditions.SourceTiers[0]
	}
	return TierHot
}

// generateID 生成唯一 ID.
func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// sortJobs 按创建时间排序任务.
func sortJobs(jobs []*ArchiveJob) {
	for i := 0; i < len(jobs); i++ {
		for j := i + 1; j < len(jobs); j++ {
			if jobs[j].CreatedAt.After(jobs[i].CreatedAt) {
				jobs[i], jobs[j] = jobs[j], jobs[i]
			}
		}
	}
}
