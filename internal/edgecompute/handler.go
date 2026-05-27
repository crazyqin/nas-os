package edgecompute

import (
	"encoding/json"
	"net/http"
)

// Handler handles HTTP requests for edge compute
type Handler struct {
	manager *Manager
}

// NewHandler creates a new edge compute handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers the HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/edge/functions", h.handleFunctions)
	mux.HandleFunc("/api/v1/edge/function", h.handleFunction)
	mux.HandleFunc("/api/v1/edge/invoke", h.handleInvoke)
	mux.HandleFunc("/api/v1/edge/workloads", h.handleWorkloads)
	mux.HandleFunc("/api/v1/edge/nodes", h.handleNodes)
	mux.HandleFunc("/api/v1/edge/stats", h.handleStats)
}

// handleFunctions handles function listing and deployment
func (h *Handler) handleFunctions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		functions := h.manager.ListFunctions()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"functions": functions,
			"total":     len(functions),
		})
	case http.MethodPost:
		var fn Function
		if err := json.NewDecoder(r.Body).Decode(&fn); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.DeployFunction(&fn); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(fn)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleFunction handles single function operations
func (h *Handler) handleFunction(w http.ResponseWriter, r *http.Request) {
	functionID := r.URL.Query().Get("id")
	if functionID == "" {
		http.Error(w, "id parameter is required", http.StatusBadRequest)
		return
	}
	
	switch r.Method {
	case http.MethodGet:
		fn, err := h.manager.GetFunction(functionID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fn)
	case http.MethodDelete:
		if err := h.manager.DeleteFunction(functionID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleInvoke handles function invocation
func (h *Handler) handleInvoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	var req struct {
		FunctionID string      `json:"function_id"`
		Input      interface{} `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	invocation, err := h.manager.InvokeFunction(r.Context(), req.FunctionID, req.Input)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invocation)
}

// handleWorkloads handles workload listing and submission
func (h *Handler) handleWorkloads(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.manager.mu.RLock()
		workloads := make([]*Workload, 0, len(h.manager.workloads))
		for _, wl := range h.manager.workloads {
			workloads = append(workloads, wl)
		}
		h.manager.mu.RUnlock()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"workloads": workloads,
			"total":     len(workloads),
		})
	case http.MethodPost:
		var wl Workload
		if err := json.NewDecoder(r.Body).Decode(&wl); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.SubmitWorkload(&wl); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(wl)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleNodes handles node listing
func (h *Handler) handleNodes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	h.manager.mu.RLock()
	nodes := make([]*Node, 0, len(h.manager.nodes))
	for _, n := range h.manager.nodes {
		nodes = append(nodes, n)
	}
	h.manager.mu.RUnlock()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": nodes,
		"total": len(nodes),
	})
}

// handleStats handles statistics requests
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	
	stats := h.manager.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
