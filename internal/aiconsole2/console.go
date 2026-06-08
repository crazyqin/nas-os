package aiconsole2

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// AIProvider represents an AI model provider
type AIProvider struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // openai, deepseek, kimi, etc.
	BaseURL     string    `json:"base_url"`
	APIKey      string    `json:"api_key,omitempty"`
	Models      []string  `json:"models"`
	Enabled     bool      `json:"enabled"`
	DailyLimit  int64     `json:"daily_limit"`  // Token daily limit
	MinuteLimit int64     `json:"minute_limit"` // Token per minute limit
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AIUsageRecord represents token usage record
type AIUsageRecord struct {
	ProviderID string    `json:"provider_id"`
	Model      string    `json:"model"`
	Tokens     int64     `json:"tokens"`
	Timestamp  time.Time `json:"timestamp"`
}

// AIConsole manages AI providers and usage
type AIConsole struct {
	mu        sync.RWMutex
	providers map[string]*AIProvider
	usage     []AIUsageRecord
	dailyUsed map[string]int64 // provider_id -> daily tokens used
}

// NewConsole creates a new AI console
func NewConsole() *AIConsole {
	return &AIConsole{
		providers: make(map[string]*AIProvider),
		usage:     make([]AIUsageRecord, 0),
		dailyUsed: make(map[string]int64),
	}
}

// AddProvider adds a new AI provider
func (c *AIConsole) AddProvider(provider AIProvider) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	provider.CreatedAt = time.Now()
	provider.UpdatedAt = time.Now()
	provider.Enabled = true
	c.providers[provider.ID] = &provider
	c.dailyUsed[provider.ID] = 0
	return nil
}

// RemoveProvider removes an AI provider
func (c *AIConsole) RemoveProvider(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.providers[id]; !exists {
		return fmt.Errorf("provider not found: %s", id)
	}
	delete(c.providers, id)
	delete(c.dailyUsed, id)
	return nil
}

// UpdateProvider updates an existing provider
func (c *AIConsole) UpdateProvider(id string, updates AIProvider) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	provider, exists := c.providers[id]
	if !exists {
		return fmt.Errorf("provider not found: %s", id)
	}

	if updates.Name != "" {
		provider.Name = updates.Name
	}
	if updates.BaseURL != "" {
		provider.BaseURL = updates.BaseURL
	}
	if updates.APIKey != "" {
		provider.APIKey = updates.APIKey
	}
	if len(updates.Models) > 0 {
		provider.Models = updates.Models
	}
	if updates.DailyLimit > 0 {
		provider.DailyLimit = updates.DailyLimit
	}
	if updates.MinuteLimit > 0 {
		provider.MinuteLimit = updates.MinuteLimit
	}
	provider.Enabled = updates.Enabled
	provider.UpdatedAt = time.Now()
	return nil
}

// GetProvider returns a provider by ID
func (c *AIConsole) GetProvider(id string) (*AIProvider, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	provider, exists := c.providers[id]
	if !exists {
		return nil, fmt.Errorf("provider not found: %s", id)
	}
	return provider, nil
}

// ListProviders returns all providers
func (c *AIConsole) ListProviders() []*AIProvider {
	c.mu.RLock()
	defer c.mu.RUnlock()

	providers := make([]*AIProvider, 0, len(c.providers))
	for _, p := range c.providers {
		providers = append(providers, p)
	}
	return providers
}

// RecordUsage records token usage for a provider
func (c *AIConsole) RecordUsage(providerID string, model string, tokens int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	provider, exists := c.providers[providerID]
	if !exists {
		return fmt.Errorf("provider not found: %s", providerID)
	}

	// Check daily limit
	if provider.DailyLimit > 0 && c.dailyUsed[providerID]+tokens > provider.DailyLimit {
		return fmt.Errorf("daily token limit exceeded for provider %s", providerID)
	}

	record := AIUsageRecord{
		ProviderID: providerID,
		Model:      model,
		Tokens:     tokens,
		Timestamp:  time.Now(),
	}
	c.usage = append(c.usage, record)
	c.dailyUsed[providerID] += tokens
	return nil
}

// GetUsageStats returns usage statistics
func (c *AIConsole) GetUsageStats() map[string]int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := make(map[string]int64)
	for id, used := range c.dailyUsed {
		stats[id] = used
	}
	return stats
}

// ResetDailyUsage resets daily usage counters
func (c *AIConsole) ResetDailyUsage() {
	c.mu.Lock()
	defer c.mu.Unlock()

	for id := range c.dailyUsed {
		c.dailyUsed[id] = 0
	}
}

// RegisterRoutes registers AI console HTTP routes
func (c *AIConsole) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/ai/providers", c.handleProviders)
	mux.HandleFunc("/api/ai/providers/add", c.handleAddProvider)
	mux.HandleFunc("/api/ai/providers/remove", c.handleRemoveProvider)
	mux.HandleFunc("/api/ai/providers/update", c.handleUpdateProvider)
	mux.HandleFunc("/api/ai/usage", c.handleUsage)
	mux.HandleFunc("/api/ai/usage/reset", c.handleResetUsage)
}

func (c *AIConsole) handleProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	providers := c.ListProviders()
	json.NewEncoder(w).Encode(providers)
}

func (c *AIConsole) handleAddProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var provider AIProvider
	if err := json.NewDecoder(r.Body).Decode(&provider); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := c.AddProvider(provider); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (c *AIConsole) handleRemoveProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := c.RemoveProvider(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (c *AIConsole) handleUpdateProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID      string      `json:"id"`
		Updates AIProvider  `json:"updates"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := c.UpdateProvider(req.ID, req.Updates); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (c *AIConsole) handleUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := c.GetUsageStats()
	json.NewEncoder(w).Encode(stats)
}

func (c *AIConsole) handleResetUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	c.ResetDailyUsage()
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
