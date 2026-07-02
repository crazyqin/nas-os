package ollamamgr

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ModelStatus represents the status of a local LLM model.
type ModelStatus string

const (
	ModelStatusDownloading ModelStatus = "downloading"
	ModelStatusReady       ModelStatus = "ready"
	ModelStatusRunning     ModelStatus = "running"
	ModelStatusError       ModelStatus = "error"
)

// LLMModel represents a local LLM model.
type LLMModel struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Family        string      `json:"family"`
	ParameterSize string      `json:"parameter_size"`
	Quantization  string      `json:"quantization"`
	SizeBytes     int64       `json:"size_bytes"`
	Status        ModelStatus `json:"status"`
	DownloadPct   float64     `json:"download_pct"`
	LoadedAt      *time.Time  `json:"loaded_at,omitempty"`
	LastUsedAt    *time.Time  `json:"last_used_at,omitempty"`
	RequestCount  int64       `json:"request_count"`
	AvgLatencyMs  float64     `json:"avg_latency_ms"`
	VRAMUsageMB   int64       `json:"vram_usage_mb"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// InferenceRequest represents an inference request.
type InferenceRequest struct {
	ModelID string                 `json:"model_id"`
	Prompt  string                 `json:"prompt"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

// InferenceResponse represents an inference response.
type InferenceResponse struct {
	ModelID      string  `json:"model_id"`
	Response     string  `json:"response"`
	TokensTotal  int     `json:"tokens_total"`
	TokensPerSec float64 `json:"tokens_per_sec"`
	LatencyMs    float64 `json:"latency_ms"`
	Done         bool    `json:"done"`
}

// GPUDetector detects available GPU/NPU hardware.
type GPUDetector struct {
	Devices []GPUDevice `json:"devices"`
}

// GPUDevice represents a GPU/NPU device.
type GPUDevice struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"` // gpu, npu, cpu
	VRAMTotalMB int64  `json:"vram_total_mb"`
	VRAMUsedMB  int64  `json:"vram_used_mb"`
	Driver      string `json:"driver"`
	Supported   bool   `json:"supported"`
}

// OllamaManager manages local LLM inference on NAS.
type OllamaManager struct {
	mu            sync.RWMutex
	models        map[string]*LLMModel
	ollamaURL     string
	gpuDevices    []GPUDevice
	defaultModel  string
	maxConcurrent int
	activeReqs    int
	totalReqs     int64
	totalTokens   int64
}

// NewOllamaManager creates a new Ollama manager.
func NewOllamaManager(ollamaURL string) *OllamaManager {
	return &OllamaManager{
		models:        make(map[string]*LLMModel),
		ollamaURL:     ollamaURL,
		maxConcurrent: 4,
	}
}

// ListModels returns all available models.
func (m *OllamaManager) ListModels() []*LLMModel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	models := make([]*LLMModel, 0, len(m.models))
	for _, model := range m.models {
		models = append(models, model)
	}
	return models
}

// PullModel downloads a model from Ollama registry.
func (m *OllamaManager) PullModel(name string) (*LLMModel, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	model := &LLMModel{
		ID:          fmt.Sprintf("model_%d", time.Now().UnixNano()),
		Name:        name,
		Status:      ModelStatusDownloading,
		DownloadPct: 0,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.models[model.ID] = model

	// Simulate async pull
	go m.doPullModel(model.ID, name)

	return model, nil
}

func (m *OllamaManager) doPullModel(id, name string) {
	// In production, call Ollama API: POST /api/pull
	for pct := 0.0; pct <= 100; pct += 10 {
		m.mu.Lock()
		if model, ok := m.models[id]; ok {
			model.DownloadPct = pct
			model.UpdatedAt = time.Now()
		}
		m.mu.Unlock()
		time.Sleep(500 * time.Millisecond)
	}

	m.mu.Lock()
	if model, ok := m.models[id]; ok {
		model.Status = ModelStatusReady
		now := time.Now()
		model.LoadedAt = &now
		model.UpdatedAt = now
	}
	m.mu.Unlock()
}

// LoadModel loads a model into memory.
func (m *OllamaManager) LoadModel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	model, exists := m.models[id]
	if !exists {
		return fmt.Errorf("model not found: %s", id)
	}
	if model.Status != ModelStatusReady {
		return fmt.Errorf("model not ready, current status: %s", model.Status)
	}

	model.Status = ModelStatusRunning
	now := time.Now()
	model.LoadedAt = &now
	model.UpdatedAt = now
	return nil
}

// UnloadModel unloads a model from memory.
func (m *OllamaManager) UnloadModel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	model, exists := m.models[id]
	if !exists {
		return fmt.Errorf("model not found: %s", id)
	}

	model.Status = ModelStatusReady
	model.LoadedAt = nil
	model.UpdatedAt = time.Now()
	return nil
}

// Inference runs inference on a loaded model.
func (m *OllamaManager) Inference(req InferenceRequest) (*InferenceResponse, error) {
	m.mu.Lock()
	if m.activeReqs >= m.maxConcurrent {
		m.mu.Unlock()
		return nil, fmt.Errorf("max concurrent requests reached (%d)", m.maxConcurrent)
	}
	m.activeReqs++
	m.totalReqs++
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.activeReqs--
		m.mu.Unlock()
	}()

	m.mu.RLock()
	model, exists := m.models[req.ModelID]
	m.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("model not found: %s", req.ModelID)
	}
	if model.Status != ModelStatusRunning {
		return nil, fmt.Errorf("model not loaded")
	}

	start := time.Now()

	// In production, call Ollama API: POST /api/generate
	resp := &InferenceResponse{
		ModelID:      req.ModelID,
		Response:     fmt.Sprintf("Response from %s: processed prompt of %d chars", model.Name, len(req.Prompt)),
		TokensTotal:  len(req.Prompt) / 4, // rough estimate
		TokensPerSec: 42.5,
		LatencyMs:    float64(time.Since(start).Milliseconds()),
		Done:         true,
	}

	// Update model stats
	m.mu.Lock()
	model.RequestCount++
	model.LastUsedAt = &start
	model.AvgLatencyMs = (model.AvgLatencyMs*float64(model.RequestCount-1) + resp.LatencyMs) / float64(model.RequestCount)
	m.totalTokens += int64(resp.TokensTotal)
	m.mu.Unlock()

	return resp, nil
}

// DeleteModel removes a model.
func (m *OllamaManager) DeleteModel(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	model, exists := m.models[id]
	if !exists {
		return fmt.Errorf("model not found: %s", id)
	}
	if model.Status == ModelStatusRunning {
		return fmt.Errorf("cannot delete running model, unload first")
	}

	delete(m.models, id)
	return nil
}

// GetStats returns manager statistics.
func (m *OllamaManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_models":    len(m.models),
		"active_requests": m.activeReqs,
		"total_requests":  m.totalReqs,
		"total_tokens":    m.totalTokens,
		"max_concurrent":  m.maxConcurrent,
		"gpu_devices":     len(m.gpuDevices),
	}
}

// RegisterRoutes registers HTTP routes.
func (m *OllamaManager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/ollama/models", m.handleListModels)
	mux.HandleFunc("/api/ollama/models/pull", m.handlePullModel)
	mux.HandleFunc("/api/ollama/models/load", m.handleLoadModel)
	mux.HandleFunc("/api/ollama/models/unload", m.handleUnloadModel)
	mux.HandleFunc("/api/ollama/models/delete", m.handleDeleteModel)
	mux.HandleFunc("/api/ollama/inference", m.handleInference)
	mux.HandleFunc("/api/ollama/stats", m.handleStats)
}

func (m *OllamaManager) handleListModels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(m.ListModels())
}

func (m *OllamaManager) handlePullModel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	model, err := m.PullModel(req.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(model)
}

func (m *OllamaManager) handleLoadModel(w http.ResponseWriter, r *http.Request) {
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
	if err := m.LoadModel(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "loaded"})
}

func (m *OllamaManager) handleUnloadModel(w http.ResponseWriter, r *http.Request) {
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
	if err := m.UnloadModel(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "unloaded"})
}

func (m *OllamaManager) handleDeleteModel(w http.ResponseWriter, r *http.Request) {
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
	if err := m.DeleteModel(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (m *OllamaManager) handleInference(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req InferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	resp, err := m.Inference(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusTooManyRequests)
		return
	}
	json.NewEncoder(w).Encode(resp)
}

func (m *OllamaManager) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(m.GetStats())
}
