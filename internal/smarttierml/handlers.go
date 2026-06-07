// Package smarttierml - REST API handlers
package smarttierml

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler HTTP 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器
func NewHandler(m *Manager) *Handler {
	return &Handler{manager: m}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/smarttier/status", h.handleStatus)
	mux.HandleFunc("/api/v1/smarttier/stats", h.handleStats)
	mux.HandleFunc("/api/v1/smarttier/config", h.handleConfig)
	mux.HandleFunc("/api/v1/smarttier/predict", h.handlePredict)
	mux.HandleFunc("/api/v1/smarttier/predictions", h.handlePredictions)
	mux.HandleFunc("/api/v1/smarttier/migrate", h.handleMigrate)
	mux.HandleFunc("/api/v1/smarttier/migrations", h.handleMigrations)
	mux.HandleFunc("/api/v1/smarttier/items", h.handleItems)
	mux.HandleFunc("/api/v1/smarttier/train", h.handleTrain)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.manager.GetTieringStats()
	writeJSON(w, map[string]interface{}{
		"running":         true,
		"totalItems":      stats.TotalItems,
		"modelAccuracy":   stats.ModelAccuracy,
		"totalMigrations": stats.TotalMigrations,
	})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, h.manager.GetTieringStats())
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, h.manager.GetConfig())
	case http.MethodPut:
		var cfg TierConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.UpdateConfig(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "updated"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handlePredict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	itemID := r.URL.Query().Get("itemId")
	if itemID == "" {
		http.Error(w, "itemId is required", http.StatusBadRequest)
		return
	}
	result, err := h.manager.PredictTier(itemID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, result)
}

func (h *Handler) handlePredictions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, h.manager.GetPredictions())
}

func (h *Handler) handleMigrate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ItemID     string `json:"itemId"`
		TargetTier Tier   `json:"targetTier"`
		Reason     string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	task, err := h.manager.RunMigration(req.ItemID, req.TargetTier, req.Reason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, task)
}

func (h *Handler) handleMigrations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status := r.URL.Query().Get("status")
	writeJSON(w, h.manager.GetMigrationTasks(status))
}

func (h *Handler) handleItems(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	tier := Tier(r.URL.Query().Get("tier"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 20
	}
	items, total := h.manager.GetItems(tier, page, pageSize)
	writeJSON(w, map[string]interface{}{
		"items":    items,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) handleTrain(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.manager.TrainModel()
	writeJSON(w, map[string]string{"status": "trained"})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
