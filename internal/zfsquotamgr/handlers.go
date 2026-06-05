package zfsquotamgr

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器
type Handler struct {
	manager *ZFSQuotaManager
}

// NewHandler 创建处理器
func NewHandler(manager *ZFSQuotaManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/zfsquotamgr/datasets", h.handleListDatasets)
	mux.HandleFunc("/api/zfsquotamgr/dataset/get", h.handleGetDataset)
	mux.HandleFunc("/api/zfsquotamgr/dataset/register", h.handleRegisterDataset)
	mux.HandleFunc("/api/zfsquotamgr/dataset/unregister", h.handleUnregisterDataset)
	mux.HandleFunc("/api/zfsquotamgr/dataset/quota", h.handleSetDatasetQuota)
	mux.HandleFunc("/api/zfsquotamgr/dataset/usage", h.handleUpdateUsage)
	mux.HandleFunc("/api/zfsquotamgr/user-quotas", h.handleListUserQuotas)
	mux.HandleFunc("/api/zfsquotamgr/user-quota/get", h.handleGetUserQuota)
	mux.HandleFunc("/api/zfsquotamgr/user-quota/set", h.handleSetUserQuota)
	mux.HandleFunc("/api/zfsquotamgr/group-quotas", h.handleListGroupQuotas)
	mux.HandleFunc("/api/zfsquotamgr/group-quota/get", h.handleGetGroupQuota)
	mux.HandleFunc("/api/zfsquotamgr/group-quota/set", h.handleSetGroupQuota)
	mux.HandleFunc("/api/zfsquotamgr/recommendations", h.handleRecommendations)
	mux.HandleFunc("/api/zfsquotamgr/stats", h.handleGetStats)
	mux.HandleFunc("/api/zfsquotamgr/alerts", h.handleGetAlerts)
	mux.HandleFunc("/api/zfsquotamgr/alert/resolve", h.handleResolveAlert)
	mux.HandleFunc("/api/zfsquotamgr/config", h.handleConfig)
}

func (h *Handler) handleListDatasets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListDatasets()})
}

func (h *Handler) handleGetDataset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := r.URL.Query().Get("name")
	if name == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少name参数"})
		return
	}
	ds, err := h.manager.GetDataset(name)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": ds})
}

func (h *Handler) handleRegisterDataset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var ds Dataset
	if err := json.NewDecoder(r.Body).Decode(&ds); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.RegisterDataset(&ds); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": ds})
}

func (h *Handler) handleUnregisterDataset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.UnregisterDataset(req.Name); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleSetDatasetQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name  string `json:"name"`
		Quota int64  `json:"quota_bytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.SetDatasetQuota(req.Name, req.Quota); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleUpdateUsage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Dataset string `json:"dataset"`
		Used    int64  `json:"used_bytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.UpdateUsage(req.Dataset, req.Used); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleListUserQuotas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dataset := r.URL.Query().Get("dataset")
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListUserQuotas(dataset)})
}

func (h *Handler) handleGetUserQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := r.URL.Query().Get("user_id")
	dataset := r.URL.Query().Get("dataset")
	if userID == "" || dataset == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少user_id或dataset参数"})
		return
	}
	q, err := h.manager.GetUserQuota(userID, dataset)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": q})
}

func (h *Handler) handleSetUserQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		UserID   string `json:"user_id"`
		Username string `json:"username"`
		Dataset  string `json:"dataset"`
		Quota    int64  `json:"quota_bytes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.SetUserQuota(req.UserID, req.Username, req.Dataset, req.Quota); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleListGroupQuotas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	dataset := r.URL.Query().Get("dataset")
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListGroupQuotas(dataset)})
}

func (h *Handler) handleGetGroupQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	groupID := r.URL.Query().Get("group_id")
	dataset := r.URL.Query().Get("dataset")
	if groupID == "" || dataset == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少group_id或dataset参数"})
		return
	}
	q, err := h.manager.GetGroupQuota(groupID, dataset)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": q})
}

func (h *Handler) handleSetGroupQuota(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		GroupID   string   `json:"group_id"`
		GroupName string   `json:"group_name"`
		Dataset   string   `json:"dataset"`
		Quota     int64    `json:"quota_bytes"`
		Members   []string `json:"members"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.SetGroupQuota(req.GroupID, req.GroupName, req.Dataset, req.Quota, req.Members); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GenerateRecommendations()})
}

func (h *Handler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GetStats()})
}

func (h *Handler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resolved := r.URL.Query().Get("resolved") == "true"
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GetAlerts(resolved)})
}

func (h *Handler) handleResolveAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.ResolveAlert(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GetConfig()})
	case http.MethodPost:
		var cfg QuotaConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
			return
		}
		h.manager.UpdateConfig(&cfg)
		writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
