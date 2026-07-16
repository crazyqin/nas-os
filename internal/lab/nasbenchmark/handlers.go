package nasbenchmark

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// RunRequest represents a benchmark run request.
type RunRequest struct {
	Type      string `json:"type"`
	Path      string `json:"path"`
	BlockSize int    `json:"block_size_kb"`
	FileSize  int    `json:"file_size_mb"`
	Threads   int    `json:"threads"`
	Duration  int    `json:"duration_seconds"`
}

// SuiteRequest represents a benchmark suite request.
type SuiteRequest struct {
	Name    string       `json:"name"`
	Configs []RunRequest `json:"configs"`
}

// CompareRequest represents a compare request.
type CompareRequest struct {
	Result1 string `json:"result1"`
	Result2 string `json:"result2"`
}

// handleRunBenchmark handles benchmark run.
func (m *Manager) handleRunBenchmark(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	config := BenchmarkConfig{
		Type:      BenchmarkType(req.Type),
		Path:      req.Path,
		BlockSize: req.BlockSize,
		FileSize:  req.FileSize,
		Threads:   req.Threads,
	}

	if config.BlockSize == 0 {
		config.BlockSize = 1024
	}
	if config.FileSize == 0 {
		config.FileSize = 1024
	}
	if config.Threads == 0 {
		config.Threads = 1
	}

	result, err := m.RunBenchmark(config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleRunSuite handles benchmark suite run.
func (m *Manager) handleRunSuite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req SuiteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	configs := make([]BenchmarkConfig, 0, len(req.Configs))
	for _, c := range req.Configs {
		config := BenchmarkConfig{
			Type:      BenchmarkType(c.Type),
			Path:      c.Path,
			BlockSize: c.BlockSize,
			FileSize:  c.FileSize,
			Threads:   c.Threads,
		}
		if config.BlockSize == 0 {
			config.BlockSize = 1024
		}
		if config.FileSize == 0 {
			config.FileSize = 1024
		}
		if config.Threads == 0 {
			config.Threads = 1
		}
		configs = append(configs, config)
	}

	suite, err := m.RunSuite(req.Name, configs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suite)
}

// handleGetResult handles getting a result.
func (m *Manager) handleGetResult(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Result ID required", http.StatusBadRequest)
		return
	}

	result, err := m.GetResult(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleListResults handles listing results.
func (m *Manager) handleListResults(w http.ResponseWriter, r *http.Request) {
	results := m.ListResults()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"results": results,
		"total":   len(results),
	})
}

// handleGetSuite handles getting a suite.
func (m *Manager) handleGetSuite(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Suite ID required", http.StatusBadRequest)
		return
	}

	suite, err := m.GetSuite(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(suite)
}

// handleListSuites handles listing suites.
func (m *Manager) handleListSuites(w http.ResponseWriter, r *http.Request) {
	suites := m.ListSuites()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"suites": suites,
		"total":  len(suites),
	})
}

// handleCompare handles comparing results.
func (m *Manager) handleCompare(w http.ResponseWriter, r *http.Request) {
	var req CompareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	comparison, err := m.CompareResults(req.Result1, req.Result2)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(comparison)
}

// RegisterRoutes registers HTTP routes.
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/benchmark/run", m.handleRunBenchmark)
	mux.HandleFunc("/api/benchmark/suite", m.handleRunSuite)
	mux.HandleFunc("/api/benchmark/result", m.handleGetResult)
	mux.HandleFunc("/api/benchmark/results", m.handleListResults)
	mux.HandleFunc("/api/benchmark/suite/get", m.handleGetSuite)
	mux.HandleFunc("/api/benchmark/suites", m.handleListSuites)
	mux.HandleFunc("/api/benchmark/compare", m.handleCompare)
	fmt.Println("Benchmark routes registered")
}
