package containerimagecache

import (
	"encoding/json"
	"net/http"
)

// Handler 容器镜像缓存 HTTP 处理器.
type Handler struct {
	mgr *ImageCacheManager
}

// NewHandler 创建处理器.
func NewHandler(mgr *ImageCacheManager) *Handler {
	return &Handler{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/containerimagecache/stats", h.handleStats)
	mux.HandleFunc("/api/v1/containerimagecache/images", h.handleImages)
	mux.HandleFunc("/api/v1/containerimagecache/pull", h.handlePull)
	mux.HandleFunc("/api/v1/containerimagecache/pin", h.handlePin)
	mux.HandleFunc("/api/v1/containerimagecache/unpin", h.handleUnpin)
	mux.HandleFunc("/api/v1/containerimagecache/delete", h.handleDelete)
	mux.HandleFunc("/api/v1/containerimagecache/registries", h.handleRegistries)
	mux.HandleFunc("/api/v1/containerimagecache/prefetch", h.handlePrefetch)
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.mgr.GetStats())
}

func (h *Handler) handleImages(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, h.mgr.ListCachedImages())
}

func (h *Handler) handlePull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ImageName string `json:"image_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	info, err := h.mgr.Pull(req.ImageName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, info)
}

func (h *Handler) handlePin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ImageName string `json:"image_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.mgr.Pin(req.ImageName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "pinned"})
}

func (h *Handler) handleUnpin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ImageName string `json:"image_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.mgr.Unpin(req.ImageName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unpinned"})
}

func (h *Handler) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		ImageName string `json:"image_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	if err := h.mgr.Delete(req.ImageName); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *Handler) handleRegistries(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handlePrefetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		Images []string `json:"images"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "prefetch_scheduled"})
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
