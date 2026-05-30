package federatednas

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
)

// Handler provides HTTP handlers for federation operations.
type Handler struct {
	federation *Federation
}

// NewHandler creates a new Handler instance.
func NewHandler(federation *Federation) *Handler {
	return &Handler{federation: federation}
}

// RegisterRoutes registers all federation routes to the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/federation/nodes", h.handleNodes)
	mux.HandleFunc("/api/federation/nodes/", h.handleNodeByID)
	mux.HandleFunc("/api/federation/sync", h.handleSync)
	mux.HandleFunc("/api/federation/sync/", h.handleSyncByID)
	mux.HandleFunc("/api/federation/status", h.handleStatus)
	mux.HandleFunc("/api/federation/conflicts", h.handleConflicts)
	mux.HandleFunc("/api/federation/conflicts/", h.handleConflictByID)
	mux.HandleFunc("/api/federation/policies", h.handlePolicies)
	mux.HandleFunc("/api/federation/health/", h.handleHealth)
	mux.HandleFunc("/api/federation/namespace", h.handleNamespace)
	mux.HandleFunc("/api/federation/propagate", h.handlePropagate)
}

// handleNodes handles GET and POST for /api/federation/nodes.
func (h *Handler) handleNodes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listNodes(w, r)
	case http.MethodPost:
		h.registerNode(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleNodeByID handles GET and DELETE for /api/federation/nodes/{id}.
func (h *Handler) handleNodeByID(w http.ResponseWriter, r *http.Request) {
	nodeID := strings.TrimPrefix(r.URL.Path, "/api/federation/nodes/")
	if nodeID == "" {
		http.Error(w, "Node ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getNode(w, r, nodeID)
	case http.MethodDelete:
		h.removeNode(w, r, nodeID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listNodes returns all registered nodes.
func (h *Handler) listNodes(w http.ResponseWriter, r *http.Request) {
	nodes := h.federation.ListNodes()
	writeJSON(w, http.StatusOK, nodes)
}

// registerNode registers a new node.
func (h *Handler) registerNode(w http.ResponseWriter, r *http.Request) {
	var node FederationNode
	if err := json.NewDecoder(r.Body).Decode(&node); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.federation.RegisterNode(&node); err != nil {
		if errors.Is(err, ErrNodeAlreadyExists) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, &node)
}

// getNode returns a specific node.
func (h *Handler) getNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	node, err := h.federation.GetNode(nodeID)
	if err != nil {
		if errors.Is(err, ErrNodeNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, node)
}

// removeNode removes a node from the federation.
func (h *Handler) removeNode(w http.ResponseWriter, r *http.Request, nodeID string) {
	if err := h.federation.RemoveNode(nodeID); err != nil {
		if errors.Is(err, ErrNodeNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Node removed"})
}

// handleSync handles GET and POST for /api/federation/sync.
func (h *Handler) handleSync(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listSyncJobs(w, r)
	case http.MethodPost:
		h.startSync(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSyncByID handles GET for /api/federation/sync/{id}.
func (h *Handler) handleSyncByID(w http.ResponseWriter, r *http.Request) {
	jobID := strings.TrimPrefix(r.URL.Path, "/api/federation/sync/")
	if jobID == "" {
		http.Error(w, "Job ID required", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	job, err := h.federation.GetSyncJob(jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, job)
}

// listSyncJobs returns all sync jobs.
func (h *Handler) listSyncJobs(w http.ResponseWriter, r *http.Request) {
	jobs := h.federation.ListSyncJobs()
	writeJSON(w, http.StatusOK, jobs)
}

// startSync initiates a new sync job.
func (h *Handler) startSync(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SourceNodeID string `json:"source_node_id"`
		TargetNodeID string `json:"target_node_id"`
		Incremental  bool   `json:"incremental"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	job, err := h.federation.StartSync(req.SourceNodeID, req.TargetNodeID, req.Incremental)
	if err != nil {
		if errors.Is(err, ErrNodeNotFound) || errors.Is(err, ErrNodeOffline) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if errors.Is(err, ErrSyncInProgress) {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, job)
}

// handleStatus handles GET for /api/federation/status.
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status := h.federation.GetFederationStatus()
	writeJSON(w, http.StatusOK, status)
}

// handleConflicts handles GET for /api/federation/conflicts.
func (h *Handler) handleConflicts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	conflicts := h.federation.ListConflicts()
	writeJSON(w, http.StatusOK, conflicts)
}

// handleConflictByID handles GET and PUT for /api/federation/conflicts/{id}.
func (h *Handler) handleConflictByID(w http.ResponseWriter, r *http.Request) {
	conflictID := strings.TrimPrefix(r.URL.Path, "/api/federation/conflicts/")
	if conflictID == "" {
		http.Error(w, "Conflict ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		conflict, err := h.federation.GetConflict(conflictID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, conflict)

	case http.MethodPut:
		var req struct {
			Resolution string `json:"resolution"`
			ResolvedBy string `json:"resolved_by"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		if err := h.federation.ResolveConflict(conflictID, req.Resolution, req.ResolvedBy); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "Conflict resolved"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePolicies handles GET and POST for /api/federation/policies.
func (h *Handler) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		policies := h.federation.ListPolicies()
		writeJSON(w, http.StatusOK, policies)

	case http.MethodPost:
		var policy FederationPolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		h.federation.AddPolicy(&policy)
		writeJSON(w, http.StatusCreated, policy)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleHealth handles GET for /api/federation/health/{nodeID}.
func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodeID := strings.TrimPrefix(r.URL.Path, "/api/federation/health/")
	if nodeID == "" {
		http.Error(w, "Node ID required", http.StatusBadRequest)
		return
	}

	health, err := h.federation.GetNodeStatus(nodeID)
	if err != nil {
		if errors.Is(err, ErrNodeNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, health)
}

// handleNamespace handles GET for /api/federation/namespace.
func (h *Handler) handleNamespace(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		path = "/"
	}

	namespace := h.federation.GetNamespace(path)
	writeJSON(w, http.StatusOK, namespace)
}

// handlePropagate handles POST for /api/federation/propagate.
func (h *Handler) handlePropagate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		NodeID   string `json:"node_id"`
		FilePath string `json:"file_path"`
		IsDelete bool   `json:"is_delete"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.federation.PropagateChange(req.NodeID, req.FilePath, req.IsDelete); err != nil {
		if errors.Is(err, ErrNodeNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "Change propagated"})
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}
