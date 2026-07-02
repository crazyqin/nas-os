// Package sandbox 提供安全沙箱隔离环境管理功能
package sandbox

import (
	"encoding/json"
	"net/http"
)

// Handlers HTTP处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// CreateSandbox 创建沙箱.
func (h *Handlers) CreateSandbox(w http.ResponseWriter, r *http.Request) {
	var req CreateSandboxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体: "+err.Error(), http.StatusBadRequest)
		return
	}

	sandbox, err := h.manager.Create(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sandbox)
}

// GetSandbox 获取沙箱.
func (h *Handlers) GetSandbox(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "沙箱ID必填", http.StatusBadRequest)
		return
	}

	sandbox, err := h.manager.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sandbox)
}

// ListSandbox 列出沙箱.
func (h *Handlers) ListSandbox(w http.ResponseWriter, r *http.Request) {
	sandboxes := h.manager.List()

	response := SandboxListResponse{
		Total: len(sandboxes),
		Items: sandboxes,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateSandbox 更新沙箱.
func (h *Handlers) UpdateSandbox(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "沙箱ID必填", http.StatusBadRequest)
		return
	}

	var req UpdateSandboxRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体: "+err.Error(), http.StatusBadRequest)
		return
	}

	sandbox, err := h.manager.Update(id, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sandbox)
}

// DeleteSandbox 删除沙箱.
func (h *Handlers) DeleteSandbox(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "沙箱ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// StartSandbox 启动沙箱.
func (h *Handlers) StartSandbox(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "沙箱ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.Start(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// StopSandbox 停止沙箱.
func (h *Handlers) StopSandbox(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "沙箱ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.Stop(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

// PauseSandbox 暂停沙箱.
func (h *Handlers) PauseSandbox(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "沙箱ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.Pause(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "paused"})
}

// ResumeSandbox 恢复沙箱.
func (h *Handlers) ResumeSandbox(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "沙箱ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.Resume(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "resumed"})
}

// GetResourceUsage 获取资源使用情况.
func (h *Handlers) GetResourceUsage(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "沙箱ID必填", http.StatusBadRequest)
		return
	}

	usage, err := h.manager.GetResourceUsage(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(usage)
}

// GetStats 获取统计信息.
func (h *Handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// CreateSnapshot 创建快照.
func (h *Handlers) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	sandboxID := r.URL.Query().Get("sandbox_id")
	if sandboxID == "" {
		http.Error(w, "沙箱ID必填", http.StatusBadRequest)
		return
	}

	var req CreateSnapshotRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体: "+err.Error(), http.StatusBadRequest)
		return
	}

	snapshot, err := h.manager.snapshots.Create(sandboxID, &req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 更新沙箱快照计数
	sandbox, err := h.manager.Get(sandboxID)
	if err == nil {
		sandbox.SnapshotCount++
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(snapshot)
}

// GetSnapshot 获取快照.
func (h *Handlers) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "快照ID必填", http.StatusBadRequest)
		return
	}

	snapshot, err := h.manager.snapshots.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(snapshot)
}

// ListSnapshot 列出快照.
func (h *Handlers) ListSnapshot(w http.ResponseWriter, r *http.Request) {
	sandboxID := r.URL.Query().Get("sandbox_id")

	var snapshots []*Snapshot
	if sandboxID != "" {
		snapshots = h.manager.snapshots.ListBySandbox(sandboxID)
	} else {
		snapshots = h.manager.snapshots.List()
	}

	response := SnapshotListResponse{
		Total: len(snapshots),
		Items: snapshots,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteSnapshot 删除快照.
func (h *Handlers) DeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "快照ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.snapshots.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

// RestoreSnapshot 从快照恢复.
func (h *Handlers) RestoreSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.URL.Query().Get("snapshot_id")
	if snapshotID == "" {
		http.Error(w, "快照ID必填", http.StatusBadRequest)
		return
	}

	sandboxID := r.URL.Query().Get("sandbox_id")
	if sandboxID == "" {
		http.Error(w, "沙箱ID必填", http.StatusBadRequest)
		return
	}

	// 检查沙箱是否存在且已停止
	sandbox, err := h.manager.Get(sandboxID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if sandbox.Status == SandboxStatusRunning {
		http.Error(w, "运行中的沙箱不能从快照恢复，请先停止", http.StatusBadRequest)
		return
	}

	// 恢复快照
	if err := h.manager.snapshots.Restore(snapshotID, sandbox.RootPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "restored"})
}
