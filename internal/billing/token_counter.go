package billing

import (
	"context"
	"errors"
	"sync"
	"time"
)

// TokenCounter tracks API token usage
type TokenCounter struct {
	usage     map[string]*UserUsage
	records   map[string]*TokenRecord
	rate      TokenRate
	storage   BillingStorage
	mu        sync.RWMutex
}

// UserUsage tracks user's token usage
type UserUsage struct {
	UserID          string     `json:"user_id"`
	TotalTokens     int64      `json:"total_tokens"`
	InputTokens     int64      `json:"input_tokens"`
	OutputTokens    int64      `json:"output_tokens"`
	CurrentMonth    int64      `json:"current_month_tokens"`
	PreviousMonth   int64      `json:"previous_month_tokens"`
	LastUsage       time.Time  `json:"last_usage"`
	Quota          int64       `json:"quota"`
	QuotaResetDate time.Time   `json:"quota_reset_date"`
}

// TokenRecord represents a single token usage record
type TokenRecord struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	Model        string    `json:"model"`
	Service      string    `json:"service"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TotalTokens  int       `json:"total_tokens"`
	Cost         float64   `json:"cost"`
	Timestamp    time.Time `json:"timestamp"`
	SessionID    string    `json:"session_id,omitempty"`
}

// TokenRate defines pricing rates
type TokenRate struct {
	InputRate  float64 `json:"input_rate"`  // per 1K tokens
	OutputRate float64 `json:"output_rate"` // per 1K tokens
	Currency   string  `json:"currency"`
}

// BillingStorage interface for billing persistence
type BillingStorage interface {
	SaveUsage(usage *UserUsage) error
	SaveRecord(record *TokenRecord) error
	GetUsage(userID string) (*UserUsage, error)
	GetRecords(userID string, start, end time.Time) ([]TokenRecord, error)
}

// NewTokenCounter creates a new token counter
func NewTokenCounter(storage BillingStorage, rate TokenRate) *TokenCounter {
	return &TokenCounter{
		usage:   make(map[string]*UserUsage),
		records: make(map[string]*TokenRecord),
		rate:    rate,
		storage: storage,
	}
}

// CountTokens records token usage
func (t *TokenCounter) CountTokens(ctx context.Context, userID, model, service string,
		inputTokens, outputTokens int) (*TokenRecord, error) {
	
	t.mu.Lock()
	defer t.mu.Unlock()
	
	// Calculate cost
	cost := t.calculateCost(inputTokens, outputTokens)
	
	// Create record
	record := &TokenRecord{
		ID:           generateRecordID(),
		UserID:       userID,
		Model:        model,
		Service:      service,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		TotalTokens:  inputTokens + outputTokens,
		Cost:         cost,
		Timestamp:    time.Now(),
	}
	
	// Update user usage
	usage := t.getOrCreateUsage(userID)
	usage.InputTokens += int64(inputTokens)
	usage.OutputTokens += int64(outputTokens)
	usage.TotalTokens += int64(inputTokens + outputTokens)
	usage.CurrentMonth += int64(inputTokens + outputTokens)
	usage.LastUsage = time.Now()
	
	// Store
	t.records[record.ID] = record
	
	// Persist
	if err := t.storage.SaveRecord(record); err != nil {
		return nil, err
	}
	
	if err := t.storage.SaveUsage(usage); err != nil {
		return nil, err
	}
	
	return record, nil
}

// CountChatTokens counts tokens for chat completion
func (t *TokenCounter) CountChatTokens(ctx context.Context, userID, model string,
		promptTokens, completionTokens int) (*TokenRecord, error) {
	return t.CountTokens(ctx, userID, model, "chat", promptTokens, completionTokens)
}

// CountEmbeddingTokens counts tokens for embedding
func (t *TokenCounter) CountEmbeddingTokens(ctx context.Context, userID, model string,
		tokens int) (*TokenRecord, error) {
	return t.CountTokens(ctx, userID, model, "embedding", tokens, 0)
}

// calculateCost calculates cost based on rates
func (t *TokenCounter) calculateCost(inputTokens, outputTokens int) float64 {
	inputCost := float64(inputTokens) / 1000.0 * t.rate.InputRate
	outputCost := float64(outputTokens) / 1000.0 * t.rate.OutputRate
	return inputCost + outputCost
}

// getOrCreateUsage gets or creates user usage
func (t *TokenCounter) getOrCreateUsage(userID string) *UserUsage {
	if usage, exists := t.usage[userID]; exists {
		return usage
	}
	
	usage := &UserUsage{
		UserID: userID,
		Quota:  -1, // 无限制
	}
	t.usage[userID] = usage
	
	// Load from storage if exists
	stored, err := t.storage.GetUsage(userID)
	if err == nil {
		usage = stored
		t.usage[userID] = usage
	}
	
	return usage
}

// GetUserUsage retrieves user's usage
func (t *TokenCounter) GetUserUsage(ctx context.Context, userID string) (*UserUsage, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	if usage, exists := t.usage[userID]; exists {
		return usage, nil
	}
	
	return t.storage.GetUsage(userID)
}

// GetUserRecords retrieves user's usage records
func (t *TokenCounter) GetUserRecords(ctx context.Context, userID string,
		start, end time.Time) ([]TokenRecord, error) {
	return t.storage.GetRecords(userID, start, end)
}

// CheckQuota checks if user has quota available
func (t *TokenCounter) CheckQuota(ctx context.Context, userID string) (bool, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	usage := t.getOrCreateUsage(userID)
	
	if usage.Quota < 0 {
		return true, nil // 无限制
	}
	
	return usage.CurrentMonth < usage.Quota, nil
}

// SetUserQuota sets user's token quota
func (t *TokenCounter) SetUserQuota(ctx context.Context, userID string, quota int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	usage := t.getOrCreateUsage(userID)
	usage.Quota = quota
	
	return t.storage.SaveUsage(usage)
}

// ResetMonthlyUsage resets monthly usage counters
func (t *TokenCounter) ResetMonthlyUsage(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	for userID, usage := range t.usage {
		usage.PreviousMonth = usage.CurrentMonth
		usage.CurrentMonth = 0
		usage.QuotaResetDate = time.Now().AddDate(0, 1, 0)
		
		if err := t.storage.SaveUsage(usage); err != nil {
			return err
		}
	}
	
	return nil
}

// GetUserCost calculates total cost for user in time range
func (t *TokenCounter) GetUserCost(ctx context.Context, userID string,
		start, end time.Time) (UserCostReport, error) {
	
	records, err := t.storage.GetRecords(userID, start, end)
	if err != nil {
		return UserCostReport{}, err
	}
	
	report := UserCostReport{
		UserID:    userID,
		StartTime: start,
		EndTime:   end,
		ByModel:   make(map[string]ModelUsage),
		ByService: make(map[string]ServiceUsage),
	}
	
	for _, record := range records {
		report.TotalCost += record.Cost
		report.TotalInputTokens += record.InputTokens
		report.TotalOutputTokens += record.OutputTokens
		report.TotalTokens += record.TotalTokens
		
		// Aggregate by model
		modelUsage := report.ByModel[record.Model]
		modelUsage.Cost += record.Cost
		modelUsage.InputTokens += record.InputTokens
		modelUsage.OutputTokens += record.OutputTokens
		modelUsage.TotalTokens += record.TotalTokens
		modelUsage.Requests++
		report.ByModel[record.Model] = modelUsage
		
		// Aggregate by service
		serviceUsage := report.ByService[record.Service]
		serviceUsage.Cost += record.Cost
		serviceUsage.TotalTokens += record.TotalTokens
		serviceUsage.Requests++
		report.ByService[record.Service] = serviceUsage
	}
	
	return report, nil
}

// UserCostReport holds user cost analysis
type UserCostReport struct {
	UserID           string                    `json:"user_id"`
	StartTime        time.Time                 `json:"start_time"`
	EndTime          time.Time                 `json:"end_time"`
	TotalCost        float64                   `json:"total_cost"`
	TotalInputTokens int                       `json:"total_input_tokens"`
	TotalOutputTokens int                       `json:"total_output_tokens"`
	TotalTokens      int                       `json:"total_tokens"`
	ByModel          map[string]ModelUsage     `json:"by_model"`
	ByService        map[string]ServiceUsage   `json:"by_service"`
}

// ModelUsage aggregates usage by model
type ModelUsage struct {
	Cost          float64 `json:"cost"`
	InputTokens   int     `json:"input_tokens"`
	OutputTokens  int     `json:"output_tokens"`
	TotalTokens   int     `json:"total_tokens"`
	Requests      int     `json:"requests"`
}

// ServiceUsage aggregates usage by service
type ServiceUsage struct {
	Cost        float64 `json:"cost"`
	TotalTokens int     `json:"total_tokens"`
	Requests    int     `json:"requests"`
}

// GetStats returns overall token usage statistics
func (t *TokenCounter) GetStats(ctx context.Context) TokenStats {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	stats := TokenStats{}
	
	for _, usage := range t.usage {
		stats.TotalUsers++
		stats.TotalInputTokens += usage.InputTokens
		stats.TotalOutputTokens += usage.OutputTokens
		stats.TotalTokens += usage.TotalTokens
	}
	
	stats.Rate = t.rate
	
	return stats
}

// TokenStats holds overall statistics
type TokenStats struct {
	TotalUsers       int       `json:"total_users"`
	TotalTokens      int64     `json:"total_tokens"`
	TotalInputTokens int64     `json:"total_input_tokens"`
	TotalOutputTokens int64     `json:"total_output_tokens"`
	Rate             TokenRate `json:"rate"`
}

// DefaultTokenRates for common models
var DefaultTokenRates = map[string]TokenRate{
	"llama3.2":     {InputRate: 0.01, OutputRate: 0.02, Currency: "USD"},
	"llama3.1:8b":  {InputRate: 0.02, OutputRate: 0.03, Currency: "USD"},
	"mistral":      {InputRate: 0.02, OutputRate: 0.03, Currency: "USD"},
	"codellama":    {InputRate: 0.02, OutputRate: 0.04, Currency: "USD"},
	"gpt-4":        {InputRate: 0.03, OutputRate: 0.06, Currency: "USD"},
	"embedding":    {InputRate: 0.0001, OutputRate: 0, Currency: "USD"},
}

// GetTokenRate returns rate for a model
func (t *TokenCounter) GetTokenRate(model string) TokenRate {
	if rate, exists := DefaultTokenRates[model]; exists {
		return rate
	}
	return t.rate
}

// Helper
func generateRecordID() string {
	return "rec_" + time.Now().Format("20060102150405")
}

// Errors
var (
	ErrQuotaExceeded = errors.New("token quota exceeded")
	ErrUsageNotFound  = errors.New("usage record not found")
)