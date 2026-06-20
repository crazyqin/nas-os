// Package aitranslator 提供AI实时翻译功能，支持多语言翻译、
// 翻译记忆、术语表和批量翻译。参考群晖ChatPlus实时翻译功能。
package aitranslator

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Language 语言
type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// Task 翻译任务
type Task struct {
	ID          string    `json:"id"`
	SourceLang  string    `json:"source_lang"`
	TargetLang  string    `json:"target_lang"`
	SourceText  string    `json:"source_text"`
	ResultText  string    `json:"result_text"`
	Status      string    `json:"status"` // pending, processing, completed, failed
	Error       string    `json:"error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Duration    int64     `json:"duration_ms"`
	CharCount   int       `json:"char_count"`
	Provider    string    `json:"provider"`
}

// Memory 翻译记忆
type Memory struct {
	ID         string    `json:"id"`
	SourceLang string    `json:"source_lang"`
	TargetLang string    `json:"target_lang"`
	SourceText string    `json:"source_text"`
	TargetText string    `json:"target_text"`
	UsageCount int       `json:"usage_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// Glossary 术语表
type Glossary struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Languages []string          `json:"languages"`
	Terms     map[string]string `json:"terms"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Config 翻译配置
type Config struct {
	DefaultProvider string `json:"default_provider"`
	MaxTextLength   int    `json:"max_text_length"`
	EnableMemory    bool   `json:"enable_memory"`
	EnableGlossary  bool   `json:"enable_glossary"`
	AutoDetect      bool   `json:"auto_detect"`
	CacheTTL        int    `json:"cache_ttl_seconds"`
}

// Provider 翻译提供者接口
type Provider interface {
	Translate(sourceLang, targetLang, text string) (string, error)
	GetName() string
}

// Manager 翻译管理器
type Manager struct {
	mu        sync.RWMutex
	config    *Config
	providers map[string]Provider
	memory    map[string]*Memory
	glossary  map[string]*Glossary
	tasks     map[string]*Task
	cache     map[string]*cacheEntry
}

type cacheEntry struct {
	result    string
	expiresAt time.Time
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		config: &Config{
			DefaultProvider: "local",
			MaxTextLength:   10000,
			EnableMemory:    true,
			EnableGlossary:  true,
			AutoDetect:      true,
			CacheTTL:        3600,
		},
		providers: make(map[string]Provider),
		memory:    make(map[string]*Memory),
		glossary:  make(map[string]*Glossary),
		tasks:     make(map[string]*Task),
		cache:     make(map[string]*cacheEntry),
	}
}

// Translate 翻译文本
func (m *Manager) Translate(sourceLang, targetLang, text string) (*Task, error) {
	start := time.Now()

	task := &Task{
		ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
		SourceLang: sourceLang,
		TargetLang: targetLang,
		SourceText: text,
		Status:     "processing",
		CreatedAt:  start,
		CharCount:  len(text),
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 检查缓存
	cacheKey := sourceLang + ":" + targetLang + ":" + text
	m.mu.RLock()
	entry, ok := m.cache[cacheKey]
	m.mu.RUnlock()

	if ok && entry.expiresAt.After(time.Now()) {
		task.ResultText = entry.result
		task.Status = "completed"
		task.CompletedAt = time.Now()
		task.Duration = time.Since(start).Milliseconds()
		return task, nil
	}

	// 检查翻译记忆
	if m.config.EnableMemory {
		m.mu.RLock()
		for _, mem := range m.memory {
			if mem.SourceLang == sourceLang && mem.TargetLang == targetLang && mem.SourceText == text {
				task.ResultText = mem.TargetText
				task.Status = "completed"
				task.CompletedAt = time.Now()
				task.Duration = time.Since(start).Milliseconds()
				mem.UsageCount++
				m.mu.RUnlock()
				return task, nil
			}
		}
		m.mu.RUnlock()
	}

	// 执行翻译
	m.mu.RLock()
	provider, ok := m.providers[m.config.DefaultProvider]
	m.mu.RUnlock()

	if !ok {
		task.Status = "failed"
		task.Error = "provider not found"
		task.CompletedAt = time.Now()
		task.Duration = time.Since(start).Milliseconds()
		return task, fmt.Errorf("provider not found")
	}

	result, err := provider.Translate(sourceLang, targetLang, text)
	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		task.CompletedAt = time.Now()
		task.Duration = time.Since(start).Milliseconds()
		return task, err
	}

	task.ResultText = result
	task.Status = "completed"
	task.CompletedAt = time.Now()
	task.Duration = time.Since(start).Milliseconds()
	task.Provider = provider.GetName()

	// 更新缓存
	m.mu.Lock()
	m.cache[cacheKey] = &cacheEntry{
		result:    result,
		expiresAt: time.Now().Add(time.Duration(m.config.CacheTTL) * time.Second),
	}
	m.mu.Unlock()

	// 更新翻译记忆
	if m.config.EnableMemory {
		m.mu.Lock()
		memKey := sourceLang + ":" + targetLang + ":" + text
		m.memory[memKey] = &Memory{
			ID:         fmt.Sprintf("%d", time.Now().UnixNano()),
			SourceLang: sourceLang,
			TargetLang: targetLang,
			SourceText: text,
			TargetText: result,
			UsageCount: 1,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
		m.mu.Unlock()
	}

	return task, nil
}

// GetTask 获取任务
func (m *Manager) GetTask(taskID string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return task, nil
}

// RegisterProvider 注册翻译提供者
func (m *Manager) RegisterProvider(provider Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.providers[provider.GetName()] = provider
}

// GetSupportedLanguages 获取支持的语言
func (m *Manager) GetSupportedLanguages() []Language {
	return []Language{
		{Code: "zh", Name: "Chinese"},
		{Code: "en", Name: "English"},
		{Code: "ja", Name: "Japanese"},
		{Code: "ko", Name: "Korean"},
		{Code: "fr", Name: "French"},
		{Code: "de", Name: "German"},
		{Code: "es", Name: "Spanish"},
		{Code: "ru", Name: "Russian"},
		{Code: "ar", Name: "Arabic"},
		{Code: "pt", Name: "Portuguese"},
		{Code: "it", Name: "Italian"},
		{Code: "th", Name: "Thai"},
		{Code: "vi", Name: "Vietnamese"},
	}
}

// AddGlossary 添加术语表
func (m *Manager) AddGlossary(glossary *Glossary) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	glossary.CreatedAt = time.Now()
	glossary.UpdatedAt = time.Now()
	m.glossary[glossary.ID] = glossary
	return nil
}

// GetGlossary 获取术语表
func (m *Manager) GetGlossary(id string) (*Glossary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	glossary, ok := m.glossary[id]
	if !ok {
		return nil, fmt.Errorf("glossary not found: %s", id)
	}
	return glossary, nil
}

// ListGlossaries 列出术语表
func (m *Manager) ListGlossaries() []*Glossary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Glossary
	for _, g := range m.glossary {
		result = append(result, g)
	}
	return result
}

// RegisterRoutes 注册HTTP路由
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/translator/translate", m.handleTranslate)
	mux.HandleFunc("/api/translator/task", m.handleGetTask)
	mux.HandleFunc("/api/translator/languages", m.handleGetLanguages)
	mux.HandleFunc("/api/translator/glossary", m.handleGlossary)
}

func (m *Manager) handleTranslate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SourceLang string `json:"source_lang"`
		TargetLang string `json:"target_lang"`
		Text       string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	task, err := m.Translate(req.SourceLang, req.TargetLang, req.Text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(task)
}

func (m *Manager) handleGetTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "Missing task ID", http.StatusBadRequest)
		return
	}

	task, err := m.GetTask(taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(task)
}

func (m *Manager) handleGetLanguages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	languages := m.GetSupportedLanguages()
	json.NewEncoder(w).Encode(languages)
}

func (m *Manager) handleGlossary(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			glossary, err := m.GetGlossary(id)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(glossary)
		} else {
			glossaries := m.ListGlossaries()
			json.NewEncoder(w).Encode(glossaries)
		}
	case http.MethodPost:
		var glossary Glossary
		if err := json.NewDecoder(r.Body).Decode(&glossary); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}
		if err := m.AddGlossary(&glossary); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
