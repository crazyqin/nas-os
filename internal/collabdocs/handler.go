package collabdocs

import (
	"encoding/json"
	"net/http"
)

// Handler handles HTTP requests for collaborative docs.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new collaborative docs handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers the HTTP routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/docs", h.handleDocs)
	mux.HandleFunc("/api/v1/doc", h.handleDoc)
	mux.HandleFunc("/api/v1/doc/collaborators", h.handleCollaborators)
	mux.HandleFunc("/api/v1/doc/comments", h.handleComments)
	mux.HandleFunc("/api/v1/doc/versions", h.handleVersions)
	mux.HandleFunc("/api/v1/doc/session", h.handleSession)
	mux.HandleFunc("/api/v1/docs/templates", h.handleTemplates)
	mux.HandleFunc("/api/v1/docs/stats", h.handleStats)
}

// handleDocs handles document listing and creation.
func (h *Handler) handleDocs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		docs := h.manager.ListDocuments(userID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"documents": docs,
			"total":     len(docs),
		})
	case http.MethodPost:
		var doc Document
		if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.CreateDocument(&doc); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(doc)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDoc handles single document operations.
func (h *Handler) handleDoc(w http.ResponseWriter, r *http.Request) {
	docID := r.URL.Query().Get("id")
	if docID == "" {
		http.Error(w, "id parameter is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		doc, err := h.manager.GetDocument(docID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(doc)
	case http.MethodPut:
		var req struct {
			Content string `json:"content"`
			UserID  string `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.UpdateDocument(docID, req.Content, req.UserID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	case http.MethodDelete:
		if err := h.manager.DeleteDocument(docID); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCollaborators handles collaborator management.
func (h *Handler) handleCollaborators(w http.ResponseWriter, r *http.Request) {
	docID := r.URL.Query().Get("doc_id")
	if docID == "" {
		http.Error(w, "doc_id is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		var collab Collaborator
		if err := json.NewDecoder(r.Body).Decode(&collab); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.AddCollaborator(docID, collab); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(collab)
	case http.MethodDelete:
		userID := r.URL.Query().Get("user_id")
		if userID == "" {
			http.Error(w, "user_id is required", http.StatusBadRequest)
			return
		}
		if err := h.manager.RemoveCollaborator(docID, userID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleComments handles comment management.
func (h *Handler) handleComments(w http.ResponseWriter, r *http.Request) {
	docID := r.URL.Query().Get("doc_id")
	if docID == "" {
		http.Error(w, "doc_id is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		comments := h.manager.GetComments(docID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"comments": comments,
			"total":    len(comments),
		})
	case http.MethodPost:
		var comment Comment
		if err := json.NewDecoder(r.Body).Decode(&comment); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		comment.DocID = docID
		if err := h.manager.AddComment(&comment); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(comment)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleVersions handles version listing.
func (h *Handler) handleVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	docID := r.URL.Query().Get("doc_id")
	if docID == "" {
		http.Error(w, "doc_id is required", http.StatusBadRequest)
		return
	}

	versions := h.manager.GetVersions(docID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"versions": versions,
		"total":    len(versions),
	})
}

// handleSession handles session management.
func (h *Handler) handleSession(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			DocID  string `json:"doc_id"`
			UserID string `json:"user_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		session := h.manager.JoinSession(req.DocID, req.UserID)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(session)
	case http.MethodDelete:
		sessionID := r.URL.Query().Get("id")
		if sessionID == "" {
			http.Error(w, "id is required", http.StatusBadRequest)
			return
		}
		h.manager.LeaveSession(sessionID)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTemplates handles template listing.
func (h *Handler) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	templates := h.manager.GetTemplates()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}

// handleStats handles statistics requests.
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
