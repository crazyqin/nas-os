package containerorch

import (
	"encoding/json"
	"net/http"
)

type ContainerOrchHandler struct {
	manager *ContainerOrchManager
}

func NewContainerOrchHandler(manager *ContainerOrchManager) *ContainerOrchHandler {
	return &ContainerOrchHandler{manager: manager}
}

func (h *ContainerOrchHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/containers", h.handleListContainers)
	mux.HandleFunc("/api/container/create", h.handleCreateContainer)
	mux.HandleFunc("/api/container/get", h.handleGetContainer)
	mux.HandleFunc("/api/container/start", h.handleStartContainer)
	mux.HandleFunc("/api/container/stop", h.handleStopContainer)
	mux.HandleFunc("/api/container/restart", h.handleRestartContainer)
	mux.HandleFunc("/api/container/remove", h.handleRemoveContainer)
	mux.HandleFunc("/api/networks", h.handleListNetworks)
	mux.HandleFunc("/api/network/create", h.handleCreateNetwork)
	mux.HandleFunc("/api/network/remove", h.handleRemoveNetwork)
	mux.HandleFunc("/api/volumes", h.handleListVolumes)
	mux.HandleFunc("/api/volume/create", h.handleCreateVolume)
	mux.HandleFunc("/api/volume/remove", h.handleRemoveVolume)
	mux.HandleFunc("/api/stacks", h.handleListStacks)
	mux.HandleFunc("/api/stack/deploy", h.handleDeployStack)
	mux.HandleFunc("/api/stack/remove", h.handleRemoveStack)
	mux.HandleFunc("/api/container/stats", h.handleGetStats)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *ContainerOrchHandler) handleListContainers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListContainers()})
}

func (h *ContainerOrchHandler) handleCreateContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var ctr Container
	if err := json.NewDecoder(r.Body).Decode(&ctr); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	created, err := h.manager.CreateContainer(&ctr)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": created})
}

func (h *ContainerOrchHandler) handleGetContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少id参数"})
		return
	}
	ctr, err := h.manager.GetContainer(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": ctr})
}

func (h *ContainerOrchHandler) handleStartContainer(w http.ResponseWriter, r *http.Request) {
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
	if err := h.manager.StartContainer(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *ContainerOrchHandler) handleStopContainer(w http.ResponseWriter, r *http.Request) {
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
	if err := h.manager.StopContainer(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *ContainerOrchHandler) handleRestartContainer(w http.ResponseWriter, r *http.Request) {
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
	if err := h.manager.RestartContainer(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *ContainerOrchHandler) handleRemoveContainer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID    string `json:"id"`
		Force bool   `json:"force"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.RemoveContainer(req.ID, req.Force); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *ContainerOrchHandler) handleListNetworks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListNetworks()})
}

func (h *ContainerOrchHandler) handleCreateNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var net Network
	if err := json.NewDecoder(r.Body).Decode(&net); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	created, err := h.manager.CreateNetwork(&net)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": created})
}

func (h *ContainerOrchHandler) handleRemoveNetwork(w http.ResponseWriter, r *http.Request) {
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
	if err := h.manager.RemoveNetwork(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *ContainerOrchHandler) handleListVolumes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListVolumes()})
}

func (h *ContainerOrchHandler) handleCreateVolume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var vol Volume
	if err := json.NewDecoder(r.Body).Decode(&vol); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	created, err := h.manager.CreateVolume(&vol)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": created})
}

func (h *ContainerOrchHandler) handleRemoveVolume(w http.ResponseWriter, r *http.Request) {
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
	if err := h.manager.RemoveVolume(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *ContainerOrchHandler) handleListStacks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListStacks()})
}

func (h *ContainerOrchHandler) handleDeployStack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var stack Stack
	if err := json.NewDecoder(r.Body).Decode(&stack); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	deployed, err := h.manager.DeployStack(&stack)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": deployed})
}

func (h *ContainerOrchHandler) handleRemoveStack(w http.ResponseWriter, r *http.Request) {
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
	if err := h.manager.RemoveStack(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *ContainerOrchHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GetStats()})
}
