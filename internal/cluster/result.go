package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// 聚合状态
const (
	AggregationStatusPending   = "pending"
	AggregationStatusRunning   = "running"
	AggregationStatusCompleted = "completed"
	AggregationStatusFailed    = "failed"
	AggregationStatusPartial   = "partial"
)

// 聚合策略
const (
	AggregationStrategyAll    = "all"    // 等待所有结果
	AggregationStrategyAny    = "any"    // 任一结果即可
	AggregationStrategyQuorum = "quorum" // 多数结果
	AggregationStrategyFirst  = "first"  // 第一个结果
)

// AggregatedResult 聚合结果
type AggregatedResult struct {
	ID             string                 `json:"id"`
	TaskID         string                 `json:"task_id"`
	Strategy       string                 `json:"strategy"`
	Status         string                 `json:"status"`
	TotalExpected  int                    `json:"total_expected"`
	TotalReceived  int                    `json:"total_received"`
	TotalSuccess   int                    `json:"total_success"`
	TotalFailed    int                    `json:"total_failed"`
	Results        []*TaskResult          `json:"results"`
	AggregatedData json.RawMessage        `json:"aggregated_data"`
	StartTime      time.Time              `json:"start_time"`
	EndTime        time.Time              `json:"end_time"`
	Duration       time.Duration          `json:"duration"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// AggregationRule 聚合规则
type AggregationRule struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	TaskPattern string        `json:"task_pattern"` // 任务 ID 模式
	Strategy    string        `json:"strategy"`     // 聚合策略
	MinResults  int           `json:"min_results"`  // 最小结果数
	MaxResults  int           `json:"max_results"`  // 最大结果数
	Timeout     time.Duration `json:"timeout"`      // 超时时间
	ProcessFunc string        `json:"process_func"` // 处理函数名
	Enabled     bool          `json:"enabled"`
	CreatedAt   time.Time     `json:"created_at"`
}

// ResultAggregatorConfig 结果聚合器配置
type ResultAggregatorConfig struct {
	DataDir        string `json:"data_dir"`
	MaxResults     int    `json:"max_results"`     // 最大保存结果数
	Timeout        int    `json:"timeout"`         // 默认超时（秒）
	ProcessWorkers int    `json:"process_workers"` // 处理工作线程数
}

// ResultAggregator 结果聚合器
type ResultAggregator struct {
	config       ResultAggregatorConfig
	aggregations map[string]*AggregatedResult
	aggMutex     sync.RWMutex
	rules        map[string]*AggregationRule
	rulesMutex   sync.RWMutex
	pending      chan *TaskResult
	ctx          context.Context
	cancel       context.CancelFunc
	logger       *zap.Logger
	callbacks    ResultCallbacks
}

// ResultCallbacks 结果回调
type ResultCallbacks struct {
	OnResultReceived      func(result *TaskResult)
	OnAggregationComplete func(agg *AggregatedResult)
	OnAggregationFailed   func(agg *AggregatedResult, err error)
}

// NewResultAggregator 创建结果聚合器
func NewResultAggregator(config ResultAggregatorConfig, logger *zap.Logger) (*ResultAggregator, error) {
	if config.DataDir == "" {
		config.DataDir = "/var/lib/nas-os/edge/results"
	}
	if config.MaxResults == 0 {
		config.MaxResults = 10000
	}
	if config.Timeout == 0 {
		config.Timeout = 300
	}
	if config.ProcessWorkers == 0 {
		config.ProcessWorkers = 4
	}

	ctx, cancel := context.WithCancel(context.Background())

	ra := &ResultAggregator{
		config:       config,
		aggregations: make(map[string]*AggregatedResult),
		rules:        make(map[string]*AggregationRule),
		pending:      make(chan *TaskResult, 1000),
		ctx:          ctx,
		cancel:       cancel,
		logger:       logger,
	}

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		cancel()
		return nil, fmt.Errorf("创建结果数据目录失败：%w", err)
	}

	// 加载持久化数据
	if err := ra.loadAggregations(); err != nil {
		logger.Warn("加载聚合结果失败", zap.Error(err))
	}

	return ra, nil
}

// Initialize 初始化结果聚合器
func (ra *ResultAggregator) Initialize() error {
	ra.logger.Info("初始化结果聚合器")

	// 启动处理工作线程
	for i := 0; i < ra.config.ProcessWorkers; i++ {
		go ra.processWorker(i)
	}

	ra.logger.Info("结果聚合器初始化完成")
	return nil
}

// SetCallbacks 设置回调
func (ra *ResultAggregator) SetCallbacks(callbacks ResultCallbacks) {
	ra.callbacks = callbacks
}

// SubmitResult 提交结果
func (ra *ResultAggregator) SubmitResult(result *TaskResult) error {
	select {
	case ra.pending <- result:
		ra.logger.Debug("提交结果",
			zap.String("task_id", result.TaskID),
			zap.String("node_id", result.NodeID),
			zap.Bool("success", result.Success))
		return nil
	default:
		return fmt.Errorf("结果队列已满")
	}
}

// CreateAggregation 创建聚合
func (ra *ResultAggregator) CreateAggregation(taskID string, strategy string, expectedCount int) (*AggregatedResult, error) {
	ra.aggMutex.Lock()
	defer ra.aggMutex.Unlock()

	agg := &AggregatedResult{
		ID:            generateAggregationID(),
		TaskID:        taskID,
		Strategy:      strategy,
		Status:        AggregationStatusPending,
		TotalExpected: expectedCount,
		Results:       make([]*TaskResult, 0),
		StartTime:     time.Now(),
	}

	ra.aggregations[agg.ID] = agg

	ra.logger.Info("创建聚合",
		zap.String("agg_id", agg.ID),
		zap.String("task_id", taskID),
		zap.String("strategy", strategy),
		zap.Int("expected", expectedCount))

	return agg, nil
}

// GetAggregation 获取聚合
func (ra *ResultAggregator) GetAggregation(aggID string) (*AggregatedResult, bool) {
	ra.aggMutex.RLock()
	defer ra.aggMutex.RUnlock()

	agg, exists := ra.aggregations[aggID]
	return agg, exists
}

// GetAggregations 获取所有聚合
func (ra *ResultAggregator) GetAggregations() []*AggregatedResult {
	ra.aggMutex.RLock()
	defer ra.aggMutex.RUnlock()

	aggs := make([]*AggregatedResult, 0, len(ra.aggregations))
	for _, agg := range ra.aggregations {
		aggs = append(aggs, agg)
	}
	return aggs
}

// CreateRule 创建聚合规则
func (ra *ResultAggregator) CreateRule(rule *AggregationRule) error {
	ra.rulesMutex.Lock()
	defer ra.rulesMutex.Unlock()

	if rule.ID == "" {
		rule.ID = generateRuleID()
	}
	rule.CreatedAt = time.Now()

	ra.rules[rule.ID] = rule

	ra.logger.Info("创建聚合规则",
		zap.String("rule_id", rule.ID),
		zap.String("name", rule.Name),
		zap.String("strategy", rule.Strategy))

	return ra.saveRules()
}

// GetRules 获取所有规则
func (ra *ResultAggregator) GetRules() []*AggregationRule {
	ra.rulesMutex.RLock()
	defer ra.rulesMutex.RUnlock()

	rules := make([]*AggregationRule, 0, len(ra.rules))
	for _, rule := range ra.rules {
		rules = append(rules, rule)
	}
	return rules
}

// processWorker 处理工作线程
func (ra *ResultAggregator) processWorker(id int) {
	for {
		select {
		case <-ra.ctx.Done():
			return
		case result := <-ra.pending:
			ra.processResult(result)
		}
	}
}

// processResult 处理结果
func (ra *ResultAggregator) processResult(result *TaskResult) {
	// 触发回调
	if ra.callbacks.OnResultReceived != nil {
		go ra.callbacks.OnResultReceived(result)
	}

	// 查找匹配的聚合
	ra.aggMutex.Lock()
	defer ra.aggMutex.Unlock()

	for _, agg := range ra.aggregations {
		if agg.Status != AggregationStatusPending && agg.Status != AggregationStatusRunning {
			continue
		}

		// 检查是否匹配
		if ra.resultMatchesAggregation(result, agg) {
			ra.addResultToAggregation(result, agg)
		}
	}
}

// resultMatchesAggregation 检查结果是否匹配聚合
func (ra *ResultAggregator) resultMatchesAggregation(result *TaskResult, agg *AggregatedResult) bool {
	// 简单匹配：任务 ID 相同或父任务 ID 相同
	if result.TaskID == agg.TaskID {
		return true
	}

	// 检查规则
	ra.rulesMutex.RLock()
	defer ra.rulesMutex.RUnlock()

	for _, rule := range ra.rules {
		if !rule.Enabled {
			continue
		}
		// 简化处理，实际应该使用正则匹配
		if rule.TaskPattern != "" && result.TaskID != "" {
			return true
		}
	}

	return false
}

// addResultToAggregation 添加结果到聚合
func (ra *ResultAggregator) addResultToAggregation(result *TaskResult, agg *AggregatedResult) {
	agg.Status = AggregationStatusRunning
	agg.Results = append(agg.Results, result)
	agg.TotalReceived++

	if result.Success {
		agg.TotalSuccess++
	} else {
		agg.TotalFailed++
	}

	ra.logger.Debug("添加结果到聚合",
		zap.String("agg_id", agg.ID),
		zap.String("task_id", result.TaskID),
		zap.Int("received", agg.TotalReceived),
		zap.Int("expected", agg.TotalExpected))

	// 检查是否完成
	if ra.checkAggregationComplete(agg) {
		ra.finalizeAggregation(agg)
	}
}

// checkAggregationComplete 检查聚合是否完成
func (ra *ResultAggregator) checkAggregationComplete(agg *AggregatedResult) bool {
	switch agg.Strategy {
	case AggregationStrategyAll:
		return agg.TotalReceived >= agg.TotalExpected

	case AggregationStrategyAny:
		return agg.TotalReceived > 0

	case AggregationStrategyQuorum:
		return agg.TotalReceived >= (agg.TotalExpected/2 + 1)

	case AggregationStrategyFirst:
		return agg.TotalReceived >= 1

	default:
		return agg.TotalReceived >= agg.TotalExpected
	}
}

// finalizeAggregation 完成聚合
func (ra *ResultAggregator) finalizeAggregation(agg *AggregatedResult) {
	agg.EndTime = time.Now()
	agg.Duration = agg.EndTime.Sub(agg.StartTime)

	// 执行聚合处理
	aggregatedData, err := ra.aggregateResults(agg)
	if err != nil {
		agg.Status = AggregationStatusFailed
		ra.logger.Error("聚合处理失败",
			zap.String("agg_id", agg.ID),
			zap.Error(err))

		if ra.callbacks.OnAggregationFailed != nil {
			go ra.callbacks.OnAggregationFailed(agg, err)
		}
		return
	}

	agg.AggregatedData = aggregatedData

	// 判断最终状态
	if agg.TotalFailed > 0 && agg.TotalSuccess == 0 {
		agg.Status = AggregationStatusFailed
	} else if agg.TotalFailed > 0 {
		agg.Status = AggregationStatusPartial
	} else {
		agg.Status = AggregationStatusCompleted
	}

	ra.logger.Info("聚合完成",
		zap.String("agg_id", agg.ID),
		zap.String("status", agg.Status),
		zap.Int("success", agg.TotalSuccess),
		zap.Int("failed", agg.TotalFailed),
		zap.Duration("duration", agg.Duration))

	// 触发回调
	if ra.callbacks.OnAggregationComplete != nil {
		go ra.callbacks.OnAggregationComplete(agg)
	}

	// 持久化
	_ = ra.saveAggregations()
}

// aggregateResults 聚合结果数据
func (ra *ResultAggregator) aggregateResults(agg *AggregatedResult) (json.RawMessage, error) {
	// 收集所有成功结果的数据
	var allData []json.RawMessage
	for _, result := range agg.Results {
		if result.Success && len(result.Data) > 0 {
			allData = append(allData, result.Data)
		}
	}

	if len(allData) == 0 {
		return json.RawMessage("[]"), nil
	}

	// 根据策略处理
	switch agg.Strategy {
	case AggregationStrategyAll, AggregationStrategyQuorum:
		// 合并所有结果
		return ra.mergeResults(allData)

	case AggregationStrategyAny, AggregationStrategyFirst:
		// 返回第一个结果
		return allData[0], nil

	default:
		return ra.mergeResults(allData)
	}
}

// mergeResults 合并结果
func (ra *ResultAggregator) mergeResults(results []json.RawMessage) (json.RawMessage, error) {
	// 简化处理：创建一个数组包含所有结果
	merged := make([]interface{}, 0)
	for _, data := range results {
		var item interface{}
		if err := json.Unmarshal(data, &item); err != nil {
			ra.logger.Warn("解析结果数据失败", zap.Error(err))
			continue
		}
		merged = append(merged, item)
	}

	return json.Marshal(merged)
}

// ResultStats 结果统计
type ResultStats struct {
	TotalAggregations int            `json:"total_aggregations"`
	ByStatus          map[string]int `json:"by_status"`
	TotalResults      int            `json:"total_results"`
}

// GetStats 获取统计
func (ra *ResultAggregator) GetStats() map[string]interface{} {
	ra.aggMutex.RLock()
	defer ra.aggMutex.RUnlock()

	stats := &ResultStats{
		TotalAggregations: len(ra.aggregations),
		ByStatus:          make(map[string]int),
		TotalResults:      0,
	}

	for _, agg := range ra.aggregations {
		stats.ByStatus[agg.Status]++
		stats.TotalResults += agg.TotalReceived
	}

	// 返回 map 格式以保持 API 兼容
	return map[string]interface{}{
		"total_aggregations": stats.TotalAggregations,
		"by_status":          stats.ByStatus,
		"total_results":      stats.TotalResults,
	}
}

// CleanupOldResults 清理旧结果
func (ra *ResultAggregator) CleanupOldResults(maxAge time.Duration) int {
	ra.aggMutex.Lock()
	defer ra.aggMutex.Unlock()

	cutoff := time.Now().Add(-maxAge)
	count := 0

	for id, agg := range ra.aggregations {
		if agg.EndTime.Before(cutoff) {
			delete(ra.aggregations, id)
			count++
		}
	}

	if count > 0 {
		ra.logger.Info("清理旧聚合结果", zap.Int("count", count))
		_ = ra.saveAggregations()
	}

	return count
}

// Shutdown 关闭结果聚合器
func (ra *ResultAggregator) Shutdown() error {
	ra.cancel()
	_ = ra.saveAggregations()
	ra.logger.Info("结果聚合器已关闭")
	return nil
}

// 持久化

func (ra *ResultAggregator) saveAggregations() error {
	ra.aggMutex.RLock()
	defer ra.aggMutex.RUnlock()

	aggsFile := filepath.Join(ra.config.DataDir, "aggregations.json")

	data, err := json.MarshalIndent(ra.aggregations, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(aggsFile, data, 0640)
}

func (ra *ResultAggregator) loadAggregations() error {
	aggsFile := filepath.Join(ra.config.DataDir, "aggregations.json")

	data, err := os.ReadFile(aggsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &ra.aggregations)
}

func (ra *ResultAggregator) saveRules() error {
	ra.rulesMutex.RLock()
	defer ra.rulesMutex.RUnlock()

	rulesFile := filepath.Join(ra.config.DataDir, "aggregation_rules.json")

	data, err := json.MarshalIndent(ra.rules, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(rulesFile, data, 0640)
}

// loadRules 加载聚合规则 - 保留用于未来从文件加载规则的场景
// func (ra *ResultAggregator) loadRules() error {
// 	rulesFile := filepath.Join(ra.config.DataDir, "aggregation_rules.json")
//
// 	data, err := os.ReadFile(rulesFile)
// 	if err != nil {
// 		if os.IsNotExist(err) {
// 			return nil
// 		}
// 		return err
// 	}
//
// 	return json.Unmarshal(data, &ra.rules)
// }

// 辅助函数

func generateAggregationID() string {
	return fmt.Sprintf("agg-%d", time.Now().UnixNano())
}
