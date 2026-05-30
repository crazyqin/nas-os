package backupdedup

import (
	"encoding/json"
	"net/http"
)

// ProcessFileRequest represents a file processing request
type ProcessFileRequest struct {
	FilePath string `json:"file_path"`
	FileSize int64  `json:"file_size"`
	Data     []byte `json:"data"`
}

// RunJobRequest represents a dedup job request
type RunJobRequest struct {
	Path string `json:"path"`
}

// handleProcessFile handles file processing
func (m *Manager) handleProcessFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ProcessFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	info, err := m.ProcessFile(req.FilePath, req.FileSize, req.Data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// handleRunJob handles running a dedup job
func (m *Manager) handleRunJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req RunJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	job, err := m.RunDedupJob(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// handleGetStats handles getting dedup stats
func (m *Manager) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := m.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleGetChunk handles getting chunk info
func (m *Manager) handleGetChunk(w http.ResponseWriter, r *http.Request) {
	hash := r.URL.Query().Get("hash")
	if hash == "" {
		http.Error(w, "Chunk hash required", http.StatusBadRequest)
		return
	}

	chunk, err := m.GetChunk(hash)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(chunk)
}

// handleListChunks handles listing chunks
func (m *Manager) handleListChunks(w http.ResponseWriter, r *http.Request) {
	chunks := m.ListChunks()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"chunks": chunks,
		"total":  len(chunks),
	})
}

// handleGetFile handles getting file info
func (m *Manager) handleGetFile(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "File path required", http.StatusBadRequest)
		return
	}

	file, err := m.GetFile(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(file)
}

// handleListFiles handles listing files
func (m *Manager) handleListFiles(w http.ResponseWriter, r *http.Request) {
	files := m.ListFiles()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"files": files,
		"total": len(files),
	})
}

// handleGetJob handles getting a job
func (m *Manager) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	job, err := m.GetJob(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// handleListJobs handles listing jobs
func (m *Manager) handleListJobs(w http.ResponseWriter, r *http.Request) {
	jobs := m.ListJobs()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jobs":  jobs,
		"total": len(jobs),
	})
}

// handleGetConfig handles getting config
func (m *Manager) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	config := m.GetConfig()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// handleUpdateConfig handles updating config
func (m *Manager) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config DedupConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := m.UpdateConfig(config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// RegisterRoutes registers HTTP routes
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/dedup/process", m.handleProcessFile)
	mux.HandleFunc("/api/dedup/job", m.handleRunJob)
	mux.HandleFunc("/api/dedup/stats", m.handleGetStats)
	mux.HandleFunc("/api/dedup/chunk", m.handleGetChunk)
	mux.HandleFunc("/api/dedup/chunks", m.handleListChunks)
	mux.HandleFunc("/api/dedup/file", m.handleGetFile)
	mux.HandleFunc("/api/dedup/files", m.handleListFiles)
	mux.HandleFunc("/api/dedup/job/get", m.handleGetJob)
	mux.HandleFunc("/api/dedup/jobs", m.handleListJobs)
	mux.HandleFunc("/api/dedup/config", m.handleGetConfig)
	mux.HandleFunc("/api/dedup/config/update", m.handleUpdateConfig)
}
