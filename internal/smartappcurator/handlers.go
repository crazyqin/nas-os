package smartappcurator

import (
	"encoding/json"
	"net/http"
)

// Handler 应用推荐 HTTP 处理器。
type Handler struct {
	curator *Curator
}

// NewHandler 创建处理器。
func NewHandler(curator *Curator) *Handler {
	return &Handler{curator: curator}
}

// RegisterRoutes 注册路由。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/smart-app-curator/recommend", h.Recommend)
	mux.HandleFunc("GET /api/smart-app-curator/apps", h.ListApps)
	mux.HandleFunc("GET /api/smart-app-curator/apps/{appId}", h.GetApp)
	mux.HandleFunc("POST /api/smart-app-curator/profile", h.UpdateProfile)
	mux.HandleFunc("GET /api/smart-app-curator/profile/{userId}", h.GetProfile)
}

// Recommend 生成推荐。
func (h *Handler) Recommend(w http.ResponseWriter, r *http.Request) {
	var req RecommendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体: "+err.Error(), http.StatusBadRequest)
		return
	}

	result, err := h.curator.Recommend(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ListApps 列出应用。
func (h *Handler) ListApps(w http.ResponseWriter, r *http.Request) {
	category := AppCategory(r.URL.Query().Get("category"))
	apps := h.curator.ListApps(category)
	writeJSON(w, http.StatusOK, apps)
}

// GetApp 获取应用详情。
func (h *Handler) GetApp(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("appId")
	app, err := h.curator.GetApp(appID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

// UpdateProfile 更新用户画像。
func (h *Handler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	var profile UserProfile
	if err := json.NewDecoder(r.Body).Decode(&profile); err != nil {
		http.Error(w, "无效的请求体: "+err.Error(), http.StatusBadRequest)
		return
	}

	h.curator.UpdateProfile(profile)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// GetProfile 获取用户画像。
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	profile, err := h.curator.GetProfile(userID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
