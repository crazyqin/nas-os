package dataclassify

import (
	"encoding/json"
	"net/http"
)

// Handler handles HTTP requests for data classification
type Handler struct {
	manager *Manager
}

// NewHandler creates a new data classification handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers the HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/classify/file", h.handleFile)
	mux.HandleFunc("/api/v1/classify/stats", h.handleStats)
	mux.HandleFunc("/api/v1/classify/search", h.handleSearch)
	mux.HandleFunc("/api/v1/classify/rules", h.handleRules)
	mux.HandleFunc("/api/v1/classify/pii", h.handlePII)
}

// handleFile handles file classification operations
func (h *Handler) handleFile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}
		file, err := h.manager.GetFile(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(file)
	case http.MethodPost:
		var file ClassifiedFile
		if err := json.NewDecoder(r.Body).Decode(&file); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.ClassifyFile(&file); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(file)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleStats returns classification statistics
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.manager.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleSearch searches classified files
func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	class := r.URL.Query().Get("classification")
	tag := r.URL.Query().Get("tag")

	var results []*ClassifiedFile
	if class != "" {
		results = h.manager.SearchByClassification(Classification(class))
	} else if tag != "" {
		results = h.manager.SearchByTag(tag)
	} else {
		http.Error(w, "Missing search parameter", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleRules handles classification rules
func (h *Handler) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var rule ClassificationRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.AddRule(&rule); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(rule)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePII returns files containing PII
func (h *Handler) handlePII(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	files := h.manager.SearchPII()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}
