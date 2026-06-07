package containermigrator

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器
type Handler struct {
	manager *ContainerMigrator
}

// NewHandler 创建处理器
func NewHandler(manager *ContainerMigrator) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/containermigrator/hosts", h.handleListHosts)
	mux.HandleFunc("/api/containermigrator/host/get", h.handleGetHost)
	mux.HandleFunc("/api/containermigrator/host/register", h.handleRegisterHost)
	mux.HandleFunc("/api/containermigrator/host/unregister", h.handleUnregisterHost)
	mux.HandleFunc("/api/containermigrator/containers", h.handleListContainers)
	mux.HandleFunc("/api/containermigrator/container/get", h.handleGetContainer)
	mux.HandleFunc("/api/containermigrator/container/register", h.handleRegisterContainer)
	mux.HandleFunc("/api/containermigrator/container/unregister", h.handleUnregisterContainer)
	mux.HandleFunc("/api/containermigrator/snapshots", h.handleListSnapshots)
	mux.HandleFunc("/api/containermigrator/snapshot/create", h.handleCreateSnapshot)
	mux.HandleFunc("/api/containermigrator/snapshot/delete", h.handleDeleteSnapshot)
	mux.HandleFunc("/api/containermigrator/migrations", h.handleListMigrations)
	mux.HandleFunc("/api/containermigrator/migration/get", h.handleGetMigration)
	mux.HandleFunc("/api/containermigrator/migration/start", h.handleStartMigration)
	mux.HandleFunc("/api/containermigrator/migration/update", h.handleUpdateMigration)
	mux.HandleFunc("/api/containermigrator/migration/rollback", h.handleRollbackMigration)
	mux.HandleFunc("/api/containermigrator/stats", h.handleGetStats)
	mux.HandleFunc("/api/containermigrator/config", h.handleConfig)
}

func (h *Handler) handleListHosts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListHosts()})
}

func (h *Handler) handleGetHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少id参数"})
		return
	}
	host, err := h.manager.GetHost(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": host})
}

func (h *Handler) handleRegisterHost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var host Host
	if err := json.NewDecoder(r.Body).Decode(&host); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.RegisterHost(&host); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": host})
}

func (h *Handler) handleUnregisterHost(w http.ResponseWriter, r *http.Request) {
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
	if err := h.manager.UnregisterHost(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleListContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListContainers()})
}

func (h *Handler) handleGetContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少id参数"})
		return
	}
	c, err := h.manager.GetContainer(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": c})
}

func (h *Handler) handleRegisterContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var c Container
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.RegisterContainer(&c); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": c})
}

func (h *Handler) handleUnregisterContainer(w http.ResponseWriter, r *http.Request) {
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
	if err := h.manager.UnregisterContainer(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	containerID := r.URL.Query().Get("container_id")
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListSnapshots(containerID)})
}

func (h *Handler) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ContainerID string `json:"container_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	snap, err := h.manager.CreateSnapshot(req.ContainerID)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": snap})
}

func (h *Handler) handleDeleteSnapshot(w http.ResponseWriter, r *http.Request) {
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
	if err := h.manager.DeleteSnapshot(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleListMigrations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListMigrations()})
}

func (h *Handler) handleGetMigration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少id参数"})
		return
	}
	task, err := h.manager.GetMigration(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": task})
}

func (h *Handler) handleStartMigration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ContainerID  string `json:"container_id"`
		TargetHostID string `json:"target_host_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	task, err := h.manager.StartMigration(req.ContainerID, req.TargetHostID)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": task})
}

func (h *Handler) handleUpdateMigration(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID          string  `json:"id"`
		Progress    float64 `json:"progress"`
		Phase       string  `json:"phase"`
		BytesSynced int64   `json:"bytes_synced"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.UpdateMigrationProgress(req.ID, req.Progress, req.Phase, req.BytesSynced); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleRollbackMigration(w http.ResponseWriter, r *http.Request) {
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
	if err := h.manager.RollbackMigration(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *Handler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GetStats()})
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GetConfig()})
	case http.MethodPost:
		var cfg MigrationConfig
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
