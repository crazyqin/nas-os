// Package usage provides AI service usage tracking and cost management
// tracker.go - Token usage tracking manager
package usage

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	ErrRecordNotFound     = fmt.Errorf("usage record not found")
	ErrQuotaNotFound      = fmt.Errorf("user quota not found")
	ErrQuotaExceeded      = fmt.Errorf("quota exceeded")
	ErrPricingNotFound    = fmt.Errorf("model pricing not found")
	ErrInvalidTimeRange   = fmt.Errorf("invalid time range")
	ErrAllocationNotFound = fmt.Errorf("cost allocation not found")
)

// ========== Token使用量追踪器 ==========

// TokenTracker Token使用量追踪管理器
type TokenTracker struct {
	config  *UsageConfig
	dataDir string
	mu      sync.RWMutex

	// 数据存储
	records       map[string]*UsageRecord    // 使用记录
	userQuotas    map[string]*UserQuota      // 用户配额
	modelPricings map[string]*ModelPricing   // 模型定价
	allocations   map[string]*CostAllocation // 成本分摊

	// 聚合缓存
	userSummaries  map[string]*UsageSummary // 用户汇总
	modelSummaries map[string]*UsageSummary // 模型汇总
	dailySummaries map[string]*UsageSummary // 每日汇总

	// 统计计数器
	recordCounter int64
	alertCounter  int64
}

// NewTokenTracker 创建Token追踪器
func NewTokenTracker(config *UsageConfig) (*TokenTracker, error) {
	if config == nil {
		config = DefaultUsageConfig()
	}

	// 确保数据目录存在
	if err := os.MkdirAll(config.DataDir, 0750); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	t := &TokenTracker{
		config:         config,
		dataDir:        config.DataDir,
		records:        make(map[string]*UsageRecord),
		userQuotas:     make(map[string]*UserQuota),
		modelPricings:  make(map[string]*ModelPricing),
		allocations:    make(map[string]*CostAllocation),
		userSummaries:  make(map[string]*UsageSummary),
		modelSummaries: make(map[string]*UsageSummary),
		dailySummaries: make(map[string]*UsageSummary),
	}

	// 加载已有数据
	if err := t.load(); err != nil {
		return nil, fmt.Errorf("加载数据失败: %w", err)
	}

	// 初始化默认模型定价
	t.initDefaultPricings()

	return t, nil
}

// ========== 使用记录管理 ==========

// RecordUsage 记录AI使用量
func (t *TokenTracker) RecordUsage(ctx context.Context, input *UsageRecordInput) (*UsageRecord, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 获取模型定价
	pricing, err := t.getPricing(input.ModelID)
	if err != nil {
		// 使用默认定价
		pricing = t.getDefaultPricing(input.Provider)
	}

	// 计算成本
	inputCost, outputCost := t.calculateCost(input.InputTokens, input.OutputTokens, pricing)
	totalCost := inputCost + outputCost

	// 计算实际计费金额（扣除免费额度）
	billedAmount := t.calculateBilledAmount(input.InputTokens, input.OutputTokens, pricing, inputCost, outputCost)

	// 创建记录
	record := &UsageRecord{
		ID:              generateUsageID(),
		UserID:          input.UserID,
		UserName:        input.UserName,
		SessionID:       input.SessionID,
		RequestType:     input.RequestType,
		ModelID:         input.ModelID,
		ModelName:       input.ModelName,
		Provider:        input.Provider,
		BackendType:     input.BackendType,
		InputTokens:     input.InputTokens,
		OutputTokens:    input.OutputTokens,
		TotalTokens:     input.InputTokens + input.OutputTokens,
		InputCost:       inputCost,
		OutputCost:      outputCost,
		TotalCost:       totalCost,
		Currency:        pricing.Currency,
		BilledAmount:    billedAmount,
		RequestDuration: input.RequestDuration,
		Success:         input.Success,
		ErrorMessage:    input.ErrorMessage,
		Streaming:       input.Streaming,
		FinishReason:    input.FinishReason,
		PromptHash:      input.PromptHash,
		ResponseHash:    input.ResponseHash,
		Metadata:        input.Metadata,
		Labels:          input.Labels,
		Timestamp:       input.Timestamp,
		CreatedAt:       time.Now(),
	}

	// 检查配额
	if quota, exists := t.userQuotas[input.UserID]; exists {
		if err := t.checkQuota(quota, record); err != nil {
			return nil, err
		}
		// 更新配额使用量
		t.updateQuotaUsage(quota, record)
	}

	// 保存记录
	t.records[record.ID] = record
	t.recordCounter++

	// 更新汇总
	t.updateSummaries(record)

	// 持久化
	if err := t.save(); err != nil {
		return nil, err
	}

	return record, nil
}

// GetRecord 获取使用记录
func (t *TokenTracker) GetRecord(id string) (*UsageRecord, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	record, ok := t.records[id]
	if !ok {
		return nil, ErrRecordNotFound
	}
	return record, nil
}

// ListRecords 列出使用记录
func (t *TokenTracker) ListRecords(filter *RecordFilter) ([]*UsageRecord, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []*UsageRecord
	for _, r := range t.records {
		if filter != nil && !t.matchFilter(r, filter) {
			continue
		}
		result = append(result, r)
	}

	// 按时间倒序排序
	sortRecordsByTime(result, true)

	// 分页
	if filter != nil {
		result = paginateRecords(result, filter.Offset, filter.Limit)
	}

	return result, nil
}

// RecordFilter 记录过滤器
type RecordFilter struct {
	UserID      string
	ModelID     string
	Provider    string
	RequestType RequestType
	StartTime   time.Time
	EndTime     time.Time
	Success     *bool
	Offset      int
	Limit       int
}

// ========== Token 统计 ==========

// GetUserUsageSummary 获取用户使用汇总
func (t *TokenTracker) GetUserUsageSummary(userID string, start, end time.Time) (*UsageSummary, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if start.IsZero() {
		start = time.Now().AddDate(0, -1, 0) // 默认近一个月
	}
	if end.IsZero() {
		end = time.Now()
	}

	summary := &UsageSummary{
		UserID:             userID,
		PeriodStart:        start,
		PeriodEnd:          end,
		RequestsByType:     make(map[RequestType]int64),
		HourlyDistribution: make(map[int]int64),
	}

	var totalLatency int64
	var latencies []int64
	modelUsageMap := make(map[string]*ModelUsage)

	for _, r := range t.records {
		if r.UserID != userID {
			continue
		}
		if r.Timestamp.Before(start) || r.Timestamp.After(end) {
			continue
		}

		// Token统计
		summary.TotalInputTokens += r.InputTokens
		summary.TotalOutputTokens += r.OutputTokens
		summary.TotalTokens += r.TotalTokens

		// 成本统计
		summary.TotalInputCost += r.InputCost
		summary.TotalOutputCost += r.OutputCost
		summary.TotalCost += r.TotalCost

		// 请求统计
		summary.TotalRequests++
		if r.Success {
			summary.SuccessRequests++
		} else {
			summary.FailedRequests++
		}
		if r.Streaming {
			summary.StreamingRequests++
		}

		// 请求类型分布
		summary.RequestsByType[r.RequestType]++

		// 模型使用
		if _, ok := modelUsageMap[r.ModelID]; !ok {
			modelUsageMap[r.ModelID] = &ModelUsage{
				ModelID:   r.ModelID,
				ModelName: r.ModelName,
				Provider:  r.Provider,
			}
		}
		mu := modelUsageMap[r.ModelID]
		mu.RequestCount++
		mu.InputTokens += r.InputTokens
		mu.OutputTokens += r.OutputTokens
		mu.TotalTokens += r.TotalTokens
		mu.TotalCost += r.TotalCost

		// 延迟统计
		totalLatency += r.RequestDuration.Milliseconds()
		latencies = append(latencies, r.RequestDuration.Milliseconds())

		// 时段分布
		hour := r.Timestamp.Hour()
		summary.HourlyDistribution[hour]++

		// 最大/最小token
		if summary.MaxTokensPerReq == 0 || r.TotalTokens > summary.MaxTokensPerReq {
			summary.MaxTokensPerReq = r.TotalTokens
		}
		if summary.MinTokensPerReq == 0 || r.TotalTokens < summary.MinTokensPerReq {
			summary.MinTokensPerReq = r.TotalTokens
		}
	}

	// 计算平均值
	if summary.TotalRequests > 0 {
		summary.AvgTokensPerReq = float64(summary.TotalTokens) / float64(summary.TotalRequests)
		summary.AvgCostPerReq = summary.TotalCost / float64(summary.TotalRequests)
		summary.AvgLatencyMs = totalLatency / summary.TotalRequests
		summary.SuccessRate = float64(summary.SuccessRequests) / float64(summary.TotalRequests) * 100
	}

	if summary.TotalTokens > 0 {
		summary.AvgCostPerToken = summary.TotalCost / float64(summary.TotalTokens)
	}

	// 计算P95延迟
	if len(latencies) > 0 {
		summary.P95LatencyMs = calculateP95(latencies)
		summary.MaxLatencyMs = max(latencies)
		summary.MinLatencyMs = min(latencies)
	}

	// 模型列表
	for _, mu := range modelUsageMap {
		if mu.RequestCount > 0 {
			mu.SuccessRate = float64(mu.RequestCount) / float64(summary.TotalRequests) * 100
		}
		summary.ModelsUsed = append(summary.ModelsUsed, *mu)
	}

	return summary, nil
}

// GetModelUsageSummary 获取模型使用汇总
func (t *TokenTracker) GetModelUsageSummary(modelID string, start, end time.Time) (*UsageSummary, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	summary := &UsageSummary{
		PeriodStart:        start,
		PeriodEnd:          end,
		RequestsByType:     make(map[RequestType]int64),
		HourlyDistribution: make(map[int]int64),
	}

	var totalLatency int64
	var latencies []int64
	userSet := make(map[string]bool)

	for _, r := range t.records {
		if r.ModelID != modelID {
			continue
		}
		if !start.IsZero() && r.Timestamp.Before(start) {
			continue
		}
		if !end.IsZero() && r.Timestamp.After(end) {
			continue
		}

		summary.TotalInputTokens += r.InputTokens
		summary.TotalOutputTokens += r.OutputTokens
		summary.TotalTokens += r.TotalTokens
		summary.TotalInputCost += r.InputCost
		summary.TotalOutputCost += r.OutputCost
		summary.TotalCost += r.TotalCost
		summary.TotalRequests++

		if r.Success {
			summary.SuccessRequests++
		}

		summary.RequestsByType[r.RequestType]++
		totalLatency += r.RequestDuration.Milliseconds()
		latencies = append(latencies, r.RequestDuration.Milliseconds())
		userSet[r.UserID] = true

		hour := r.Timestamp.Hour()
		summary.HourlyDistribution[hour]++
	}

	if summary.TotalRequests > 0 {
		summary.AvgTokensPerReq = float64(summary.TotalTokens) / float64(summary.TotalRequests)
		summary.AvgCostPerReq = summary.TotalCost / float64(summary.TotalRequests)
		summary.AvgLatencyMs = totalLatency / summary.TotalRequests
		summary.SuccessRate = float64(summary.SuccessRequests) / float64(summary.TotalRequests) * 100
	}

	if len(latencies) > 0 {
		summary.P95LatencyMs = calculateP95(latencies)
	}

	return summary, nil
}

// GetGlobalUsageSummary 获取全局使用汇总
func (t *TokenTracker) GetGlobalUsageSummary(start, end time.Time) (*UsageSummary, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	summary := &UsageSummary{
		PeriodStart:        start,
		PeriodEnd:          end,
		RequestsByType:     make(map[RequestType]int64),
		HourlyDistribution: make(map[int]int64),
	}

	var totalLatency int64
	var latencies []int64
	modelUsageMap := make(map[string]*ModelUsage)

	for _, r := range t.records {
		if !start.IsZero() && r.Timestamp.Before(start) {
			continue
		}
		if !end.IsZero() && r.Timestamp.After(end) {
			continue
		}

		summary.TotalInputTokens += r.InputTokens
		summary.TotalOutputTokens += r.OutputTokens
		summary.TotalTokens += r.TotalTokens
		summary.TotalInputCost += r.InputCost
		summary.TotalOutputCost += r.OutputCost
		summary.TotalCost += r.TotalCost
		summary.TotalRequests++

		if r.Success {
			summary.SuccessRequests++
		} else {
			summary.FailedRequests++
		}

		summary.RequestsByType[r.RequestType]++
		totalLatency += r.RequestDuration.Milliseconds()
		latencies = append(latencies, r.RequestDuration.Milliseconds())

		// 模型统计
		if _, ok := modelUsageMap[r.ModelID]; !ok {
			modelUsageMap[r.ModelID] = &ModelUsage{
				ModelID:   r.ModelID,
				ModelName: r.ModelName,
				Provider:  r.Provider,
			}
		}
		mu := modelUsageMap[r.ModelID]
		mu.RequestCount++
		mu.TotalTokens += r.TotalTokens
		mu.TotalCost += r.TotalCost

		hour := r.Timestamp.Hour()
		summary.HourlyDistribution[hour]++
	}

	if summary.TotalRequests > 0 {
		summary.AvgTokensPerReq = float64(summary.TotalTokens) / float64(summary.TotalRequests)
		summary.AvgCostPerReq = summary.TotalCost / float64(summary.TotalRequests)
		summary.AvgLatencyMs = totalLatency / summary.TotalRequests
		summary.SuccessRate = float64(summary.SuccessRequests) / float64(summary.TotalRequests) * 100
	}

	if len(latencies) > 0 {
		summary.P95LatencyMs = calculateP95(latencies)
	}

	for _, mu := range modelUsageMap {
		summary.ModelsUsed = append(summary.ModelsUsed, *mu)
	}

	return summary, nil
}

// ========== 成本计算 ==========

// calculateCost 计算Token成本
func (t *TokenTracker) calculateCost(inputTokens, outputTokens int64, pricing *ModelPricing) (inputCost, outputCost float64) {
	switch pricing.PricingModel {
	case TokenPricingModelTiered:
		inputCost = t.calculateTieredCost(inputTokens, pricing.TieredPricing, true)
		outputCost = t.calculateTieredCost(outputTokens, pricing.TieredPricing, false)
	case TokenPricingModelDynamic:
		inputCost, outputCost = t.calculateDynamicCost(inputTokens, outputTokens, pricing.DynamicPricingConfig)
	default: // TokenPricingModelFixed
		inputCost = float64(inputTokens) / 1000 * pricing.InputPricePer1K
		outputCost = float64(outputTokens) / 1000 * pricing.OutputPricePer1K
	}

	// 应用折扣
	if pricing.DiscountPercent > 0 {
		inputCost = inputCost * (1 - pricing.DiscountPercent/100)
		outputCost = outputCost * (1 - pricing.DiscountPercent/100)
	}

	return inputCost, outputCost
}

// calculateTieredCost 阶梯定价计算
func (t *TokenTracker) calculateTieredCost(tokens int64, tiers []TokenTier, isInput bool) float64 {
	if len(tiers) == 0 {
		return 0
	}

	var totalCost float64
	remaining := tokens

	for _, tier := range tiers {
		if remaining <= 0 {
			break
		}

		tierSize := tier.MaxTokens - tier.MinTokens
		pricePerK := tier.InputPricePer1K
		if !isInput {
			pricePerK = tier.OutputPricePer1K
		}

		if tier.MaxTokens < 0 { // 无限阶梯
			totalCost += float64(remaining) / 1000 * pricePerK
			break
		}

		if remaining <= tierSize {
			totalCost += float64(remaining) / 1000 * pricePerK
			break
		}

		totalCost += float64(tierSize) / 1000 * pricePerK
		remaining -= tierSize
	}

	return totalCost
}

// calculateDynamicCost 动态定价计算
func (t *TokenTracker) calculateDynamicCost(inputTokens, outputTokens int64, config *DynamicPricingConfig) (inputCost, outputCost float64) {
	if config == nil {
		return 0, 0
	}

	now := time.Now()
	hour := now.Hour()
	isWeekend := now.Weekday() == time.Saturday || now.Weekday() == time.Sunday

	// 检查是否高峰时段
	isPeak := false
	for _, peakHour := range config.PeakHours {
		if hour == peakHour {
			isPeak = true
			break
		}
	}

	// 计算输入成本
	inputCost = float64(inputTokens) / 1000 * config.BaseInputPrice
	if isPeak {
		inputCost *= config.PeakMultiplier
	} else if config.OffPeakDiscount > 0 {
		inputCost *= (1 - config.OffPeakDiscount)
	}
	if isWeekend && config.WeekendDiscount > 0 {
		inputCost *= (1 - config.WeekendDiscount)
	}

	// 计算输出成本
	outputCost = float64(outputTokens) / 1000 * config.BaseOutputPrice
	if isPeak {
		outputCost *= config.PeakMultiplier
	} else if config.OffPeakDiscount > 0 {
		outputCost *= (1 - config.OffPeakDiscount)
	}
	if isWeekend && config.WeekendDiscount > 0 {
		outputCost *= (1 - config.WeekendDiscount)
	}

	return inputCost, outputCost
}

// calculateBilledAmount 计算实际计费金额
func (t *TokenTracker) calculateBilledAmount(inputTokens, outputTokens int64, pricing *ModelPricing, inputCost, outputCost float64) float64 {
	// 扣除免费额度
	freeInputTokens := pricing.FreeInputTokens
	freeOutputTokens := pricing.FreeOutputTokens

	billedInputTokens := inputTokens - freeInputTokens
	if billedInputTokens < 0 {
		billedInputTokens = 0
	}
	billedOutputTokens := outputTokens - freeOutputTokens
	if billedOutputTokens < 0 {
		billedOutputTokens = 0
	}

	// 重新计算计费金额
	if billedInputTokens > 0 || billedOutputTokens > 0 {
		billedInputCost, billedOutputCost := t.calculateCost(billedInputTokens, billedOutputTokens, pricing)
		return billedInputCost + billedOutputCost
	}

	return 0
}

// ========== 模型定价管理 ==========

// SetModelPricing 设置模型定价
func (t *TokenTracker) SetModelPricing(pricing *ModelPricing) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if pricing.EffectiveAt.IsZero() {
		pricing.EffectiveAt = time.Now()
	}

	t.modelPricings[pricing.ModelID] = pricing
	return t.save()
}

// GetModelPricing 获取模型定价
func (t *TokenTracker) GetModelPricing(modelID string) (*ModelPricing, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	pricing, ok := t.modelPricings[modelID]
	if !ok {
		return nil, ErrPricingNotFound
	}
	return pricing, nil
}

// ListModelPricings 列出所有模型定价
func (t *TokenTracker) ListModelPricings() []*ModelPricing {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make([]*ModelPricing, 0, len(t.modelPricings))
	for _, p := range t.modelPricings {
		result = append(result, p)
	}
	return result
}

// getPricing 获取定价（内部方法）
func (t *TokenTracker) getPricing(modelID string) (*ModelPricing, error) {
	pricing, ok := t.modelPricings[modelID]
	if !ok {
		return nil, ErrPricingNotFound
	}
	return pricing, nil
}

// getDefaultPricing 获取默认定价
func (t *TokenTracker) getDefaultPricing(provider string) *ModelPricing {
	// 根据提供商返回默认定价
	switch provider {
	case "openai":
		return &ModelPricing{
			ModelID:          "default",
			InputPricePer1K:  0.0015, // GPT-3.5-turbo 价格
			OutputPricePer1K: 0.002,
			Currency:         "USD",
		}
	case "deepseek":
		return &ModelPricing{
			ModelID:          "default",
			InputPricePer1K:  0.001,
			OutputPricePer1K: 0.002,
			Currency:         "CNY",
		}
	default:
		return &ModelPricing{
			ModelID:          "default",
			InputPricePer1K:  0.001,
			OutputPricePer1K: 0.001,
			Currency:         t.config.DefaultCurrency,
		}
	}
}

// initDefaultPricings 初始化默认定价
func (t *TokenTracker) initDefaultPricings() {
	// OpenAI 模型定价
	t.modelPricings["gpt-4"] = &ModelPricing{
		ModelID:          "gpt-4",
		ModelName:        "GPT-4",
		Provider:         "openai",
		InputPricePer1K:  0.03,
		OutputPricePer1K: 0.06,
		Currency:         "USD",
		Enabled:          true,
		EffectiveAt:      time.Now(),
	}

	t.modelPricings["gpt-4-turbo"] = &ModelPricing{
		ModelID:          "gpt-4-turbo",
		ModelName:        "GPT-4 Turbo",
		Provider:         "openai",
		InputPricePer1K:  0.01,
		OutputPricePer1K: 0.03,
		Currency:         "USD",
		Enabled:          true,
		EffectiveAt:      time.Now(),
	}

	t.modelPricings["gpt-3.5-turbo"] = &ModelPricing{
		ModelID:          "gpt-3.5-turbo",
		ModelName:        "GPT-3.5 Turbo",
		Provider:         "openai",
		InputPricePer1K:  0.0015,
		OutputPricePer1K: 0.002,
		Currency:         "USD",
		Enabled:          true,
		EffectiveAt:      time.Now(),
	}

	// 国产模型定价
	t.modelPricings["deepseek-chat"] = &ModelPricing{
		ModelID:          "deepseek-chat",
		ModelName:        "DeepSeek Chat",
		Provider:         "deepseek",
		InputPricePer1K:  0.001,
		OutputPricePer1K: 0.002,
		Currency:         "CNY",
		Enabled:          true,
		EffectiveAt:      time.Now(),
	}

	t.modelPricings["glm-4"] = &ModelPricing{
		ModelID:          "glm-4",
		ModelName:        "GLM-4",
		Provider:         "zhipuai",
		InputPricePer1K:  0.1,
		OutputPricePer1K: 0.1,
		Currency:         "CNY",
		Enabled:          true,
		EffectiveAt:      time.Now(),
	}

	t.modelPricings["qwen-turbo"] = &ModelPricing{
		ModelID:          "qwen-turbo",
		ModelName:        "Qwen Turbo",
		Provider:         "qwen",
		InputPricePer1K:  0.002,
		OutputPricePer1K: 0.006,
		Currency:         "CNY",
		Enabled:          true,
		EffectiveAt:      time.Now(),
	}

	// 本地模型（免费）
	t.modelPricings["local"] = &ModelPricing{
		ModelID:          "local",
		ModelName:        "Local LLM",
		Provider:         "local",
		InputPricePer1K:  0,
		OutputPricePer1K: 0,
		Currency:         "CNY",
		Enabled:          true,
		EffectiveAt:      time.Now(),
		FreeInputTokens:  math.MaxInt64,
		FreeOutputTokens: math.MaxInt64,
	}
}

// ========== 辅助方法 ==========

// matchFilter 匹配过滤器
func (t *TokenTracker) matchFilter(r *UsageRecord, f *RecordFilter) bool {
	if f.UserID != "" && r.UserID != f.UserID {
		return false
	}
	if f.ModelID != "" && r.ModelID != f.ModelID {
		return false
	}
	if f.Provider != "" && r.Provider != f.Provider {
		return false
	}
	if f.RequestType != "" && r.RequestType != f.RequestType {
		return false
	}
	if !f.StartTime.IsZero() && r.Timestamp.Before(f.StartTime) {
		return false
	}
	if !f.EndTime.IsZero() && r.Timestamp.After(f.EndTime) {
		return false
	}
	if f.Success != nil && r.Success != *f.Success {
		return false
	}
	return true
}

// updateSummaries 更新汇总缓存
func (t *TokenTracker) updateSummaries(r *UsageRecord) {
	// TODO: 实现增量更新汇总缓存
}

// sortRecordsByTime 按时间排序记录
func sortRecordsByTime(records []*UsageRecord, desc bool) {
	if desc {
		for i := 0; i < len(records)-1; i++ {
			for j := i + 1; j < len(records); j++ {
				if records[i].Timestamp.Before(records[j].Timestamp) {
					records[i], records[j] = records[j], records[i]
				}
			}
		}
	} else {
		for i := 0; i < len(records)-1; i++ {
			for j := i + 1; j < len(records); j++ {
				if records[i].Timestamp.After(records[j].Timestamp) {
					records[i], records[j] = records[j], records[i]
				}
			}
		}
	}
}

// paginateRecords 分页
func paginateRecords(records []*UsageRecord, offset, limit int) []*UsageRecord {
	if offset >= len(records) {
		return nil
	}
	end := offset + limit
	if limit <= 0 || end > len(records) {
		end = len(records)
	}
	return records[offset:end]
}

// calculateP95 计算P95
func calculateP95(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}

	// 简单排序
	sorted := make([]int64, len(values))
	copy(sorted, values)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := int(float64(len(sorted)) * 0.95)
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

// max 最大值
func max(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values {
		if v > m {
			m = v
		}
	}
	return m
}

// min 最小值
func min(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	m := values[0]
	for _, v := range values {
		if v < m {
			m = v
		}
	}
	return m
}

// generateUsageID 生成使用记录ID
func generateUsageID() string {
	return fmt.Sprintf("usage-%d-%s", time.Now().UnixNano(), randomString(6))
}

// randomString 随机字符串
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	randBytes := make([]byte, n)
	if _, err := cryptorand.Read(randBytes); err == nil {
		for i := range b {
			b[i] = letters[int(randBytes[i])%len(letters)]
		}
		return string(b)
	}
	// Fallback
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range b {
		b[i] = letters[rng.Intn(len(letters))]
	}
	return string(b)
}

// ========== 数据持久化 ==========

// load 加载数据
func (t *TokenTracker) load() error {
	// 加载使用记录
	recordsPath := filepath.Join(t.dataDir, "usage_records.json")
	if data, err := os.ReadFile(recordsPath); err == nil {
		var records []*UsageRecord
		if err := json.Unmarshal(data, &records); err != nil {
			return fmt.Errorf("解析使用记录失败: %w", err)
		}
		for _, r := range records {
			t.records[r.ID] = r
		}
	}

	// 加载用户配额
	quotasPath := filepath.Join(t.dataDir, "user_quotas.json")
	if data, err := os.ReadFile(quotasPath); err == nil {
		var quotas []*UserQuota
		if err := json.Unmarshal(data, &quotas); err != nil {
			return fmt.Errorf("解析用户配额失败: %w", err)
		}
		for _, q := range quotas {
			t.userQuotas[q.UserID] = q
		}
	}

	// 加载模型定价
	pricingsPath := filepath.Join(t.dataDir, "model_pricings.json")
	if data, err := os.ReadFile(pricingsPath); err == nil {
		var pricings []*ModelPricing
		if err := json.Unmarshal(data, &pricings); err != nil {
			return fmt.Errorf("解析模型定价失败: %w", err)
		}
		for _, p := range pricings {
			t.modelPricings[p.ModelID] = p
		}
	}

	return nil
}

// save 保存数据
func (t *TokenTracker) save() error {
	// 保存使用记录
	records := make([]*UsageRecord, 0, len(t.records))
	for _, r := range t.records {
		records = append(records, r)
	}
	recordsData, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化使用记录失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(t.dataDir, "usage_records.json"), recordsData, 0600); err != nil {
		return fmt.Errorf("保存使用记录失败: %w", err)
	}

	// 保存用户配额
	quotas := make([]*UserQuota, 0, len(t.userQuotas))
	for _, q := range t.userQuotas {
		quotas = append(quotas, q)
	}
	quotasData, err := json.MarshalIndent(quotas, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化用户配额失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(t.dataDir, "user_quotas.json"), quotasData, 0600); err != nil {
		return fmt.Errorf("保存用户配额失败: %w", err)
	}

	// 保存模型定价
	pricings := make([]*ModelPricing, 0, len(t.modelPricings))
	for _, p := range t.modelPricings {
		pricings = append(pricings, p)
	}
	pricingsData, err := json.MarshalIndent(pricings, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化模型定价失败: %w", err)
	}
	if err := os.WriteFile(filepath.Join(t.dataDir, "model_pricings.json"), pricingsData, 0600); err != nil {
		return fmt.Errorf("保存模型定价失败: %w", err)
	}

	return nil
}

// UsageRecordInput 使用记录输入
type UsageRecordInput struct {
	UserID          string
	UserName        string
	SessionID       string
	RequestType     RequestType
	ModelID         string
	ModelName       string
	Provider        string
	BackendType     string
	InputTokens     int64
	OutputTokens    int64
	RequestDuration time.Duration
	Success         bool
	ErrorMessage    string
	Streaming       bool
	FinishReason    string
	PromptHash      string
	ResponseHash    string
	Metadata        map[string]interface{}
	Labels          map[string]string
	Timestamp       time.Time
}
