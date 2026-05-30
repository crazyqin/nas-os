package smartmigration

import (
	"encoding/json"
	"net/http"
)

// CreateMigrationRequest represents a create migration request
type CreateMigrationRequest struct {
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	Source      MigrationEndpoint `json:"source"`
	Destination MigrationEndpoint `json:"destination"`
	Options     MigrationOptions  `json:"options"`
}

// CreatePlanRequest represents a create plan request
type CreatePlanRequest struct {
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	Source      MigrationEndpoint `json:"source"`
	Destination MigrationEndpoint `json:"destination"`
}

// MigrationActionRequest represents a migration action request
type MigrationActionRequest struct {
	ID string `json:"id"`
}

// handleCreateMigration handles migration creation
func (m *Manager) handleCreateMigration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateMigrationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	migration, err := m.CreateMigration(req.Name, MigrationType(req.Type), req.Source, req.Destination, req.Options)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(migration)
}

// handleStartMigration handles starting a migration
func (m *Manager) handleStartMigration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MigrationActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := m.StartMigration(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// handlePauseMigration handles pausing a migration
func (m *Manager) handlePauseMigration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MigrationActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := m.PauseMigration(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
}

// handleResumeMigration handles resuming a migration
func (m *Manager) handleResumeMigration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MigrationActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := m.ResumeMigration(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
}

// handleCancelMigration handles cancelling a migration
func (m *Manager) handleCancelMigration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req MigrationActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := m.CancelMigration(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}

// handleGetMigration handles getting a migration
func (m *Manager) handleGetMigration(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Migration ID required", http.StatusBadRequest)
		return
	}

	migration, err := m.GetMigration(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(migration)
}

// handleListMigrations handles listing migrations
func (m *Manager) handleListMigrations(w http.ResponseWriter, r *http.Request) {
	status := MigrationStatus(r.URL.Query().Get("status"))
	migrations := m.ListMigrations(status)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"migrations": migrations,
		"total":      len(migrations),
	})
}

// handleCreatePlan handles creating a migration plan
func (m *Manager) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreatePlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	plan, err := m.CreatePlan(req.Name, MigrationType(req.Type), req.Source, req.Destination)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// handleGetPlan handles getting a plan
func (m *Manager) handleGetPlan(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Plan ID required", http.StatusBadRequest)
		return
	}

	plan, err := m.GetPlan(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(plan)
}

// handleEstimate handles migration estimation
func (m *Manager) handleEstimate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Source MigrationEndpoint `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	totalBytes, estimatedTime, err := m.EstimateMigration(req.Source)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_bytes":     totalBytes,
		"estimated_time":  estimatedTime.String(),
		"estimated_hours": estimatedTime.Hours(),
	})
}

// RegisterRoutes registers HTTP routes
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/migration/create", m.handleCreateMigration)
	mux.HandleFunc("/api/migration/start", m.handleStartMigration)
	mux.HandleFunc("/api/migration/pause", m.handlePauseMigration)
	mux.HandleFunc("/api/migration/resume", m.handleResumeMigration)
	mux.HandleFunc("/api/migration/cancel", m.handleCancelMigration)
	mux.HandleFunc("/api/migration/get", m.handleGetMigration)
	mux.HandleFunc("/api/migration/list", m.handleListMigrations)
	mux.HandleFunc("/api/migration/plan/create", m.handleCreatePlan)
	mux.HandleFunc("/api/migration/plan/get", m.handleGetPlan)
	mux.HandleFunc("/api/migration/estimate", m.handleEstimate)
}
