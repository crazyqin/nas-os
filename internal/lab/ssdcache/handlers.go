package ssdcache

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler provides HTTP handlers for SSD cache management.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new SSD cache HTTP handler.
func NewHandler(m *Manager) *Handler {
	return &Handler{manager: m}
}

// RegisterRoutes registers SSD cache API routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/ssdcache/status", h.handleStatus)
	mux.HandleFunc("/api/v1/ssdcache/stats", h.handleStats)
	mux.HandleFunc("/api/v1/ssdcache/entries", h.handleEntries)
	mux.HandleFunc("/api/v1/ssdcache/flush", h.handleFlush)
	mux.HandleFunc("/api/v1/ssdcache/invalidate", h.handleInvalidate)
	mux.HandleFunc("/api/v1/ssdcache/resize", h.handleResize)
	mux.HandleFunc("/api/v1/ssdcache/config", h.handleConfig)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetStats()
	resp := map[string]interface{}{
		"enabled":    true,
		"mode":       h.manager.config.Mode,
		"policy":     h.manager.config.Policy,
		"devicePath": h.manager.config.DevicePath,
		"stats":      stats,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.manager.GetStats())
}

func (h *Handler) handleEntries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.manager.GetCacheEntries())
}

func (h *Handler) handleFlush(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flushed := h.manager.Flush()
	writeJSON(w, http.StatusOK, map[string]int{"flushed": flushed})
}

func (h *Handler) handleInvalidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	keyStr := r.URL.Query().Get("key")
	key, err := strconv.ParseUint(keyStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid key", http.StatusBadRequest)
		return
	}
	h.manager.Invalidate(key)
	writeJSON(w, http.StatusOK, map[string]string{"status": "invalidated"})
}

func (h *Handler) handleResize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sizeStr := r.URL.Query().Get("sizeMB")
	size, err := strconv.Atoi(sizeStr)
	if err != nil || size <= 0 {
		http.Error(w, "invalid size", http.StatusBadRequest)
		return
	}
	if err := h.manager.Resize(size); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"newSizeMB": size})
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.manager.config)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
