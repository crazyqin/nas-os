// Package globalfilelock HTTP API 处理器
// 提供 RESTful API 接口用于全局文件锁管理
package globalfilelock

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Handlers HTTP API 处理器
type Handlers struct {
	manager    *LockManager
	resolver   *ConflictResolver
}

// NewHandlers 创建 HTTP API 处理器
func NewHandlers(manager *LockManager) *Handlers {
	return &Handlers{
		manager:  manager,
		resolver: NewConflictResolver(manager),
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	prefix := "/api/v1/filelock"

	// 锁操作
	mux.HandleFunc(prefix+"/lock", h.handleLock)
	mux.HandleFunc(prefix+"/unlock", h.handleUnlock)
	mux.HandleFunc(prefix+"/renew", h.handleRenew)

	// 查询接口
	mux.HandleFunc(prefix+"/status", h.handleStatus)
	mux.HandleFunc(prefix+"/conflicts", h.handleConflicts)
	mux.HandleFunc(prefix+"/stats", h.handleStats)

	// 高级操作
	mux.HandleFunc(prefix+"/upgrade", h.handleUpgrade)
	mux.HandleFunc(prefix+"/downgrade", h.handleDowngrade)
	mux.HandleFunc(prefix+"/resolve", h.handleResolve)
	mux.HandleFunc(prefix+"/sites", h.handleSites)
	mux.HandleFunc(prefix+"/history", h.handleHistory)
}

// handleLock 处理获取锁请求
// POST /api/v1/filelock/lock
func (h *Handlers) handleLock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "只支持 POST 方法")
		return
	}

	var req AcquireLockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
		return
	}

	lock, err := h.manager.AcquireLock(&req)
	if err != nil {
		// 冲突返回 409
		if strings.Contains(err.Error(), "冲突") || strings.Contains(err.Error(), "已持有") {
			h.writeError(w, http.StatusConflict, err.Error())
			return
		}
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeSuccess(w, http.StatusCreated, "锁已获取", lock)
}

// handleUnlock 处理释放锁请求
// POST /api/v1/filelock/unlock
func (h *Handlers) handleUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "只支持 POST 方法")
		return
	}

	var req ReleaseLockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
		return
	}

	if err := h.manager.ReleaseLock(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeSuccess(w, http.StatusOK, "锁已释放", nil)
}

// handleRenew 处理续期请求
// POST /api/v1/filelock/renew
func (h *Handlers) handleRenew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "只支持 POST 方法")
		return
	}

	var req RenewLockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
		return
	}

	lock, err := h.manager.RenewLock(&req)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeSuccess(w, http.StatusOK, "锁已续期", lock)
}

// handleStatus 处理锁状态查询
// GET /api/v1/filelock/status?file_path=xxx&holder_id=xxx
func (h *Handlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "只支持 GET 方法")
		return
	}

	filePath := r.URL.Query().Get("file_path")
	holderID := r.URL.Query().Get("holder_id")
	lockID := r.URL.Query().Get("lock_id")

	// 查询特定锁
	if lockID != "" {
		lock, err := h.manager.GetLock(lockID)
		if err != nil {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.writeSuccess(w, http.StatusOK, "success", lock)
		return
	}

	// 查询文件锁
	if filePath != "" {
		locks := h.manager.GetFileLocks(filePath)
		h.writeSuccess(w, http.StatusOK, "success", map[string]interface{}{
			"file_path": filePath,
			"locks":     locks,
			"count":     len(locks),
		})
		return
	}

	// 查询用户锁
	if holderID != "" {
		locks := h.manager.GetUserLocks(holderID)
		h.writeSuccess(w, http.StatusOK, "success", map[string]interface{}{
			"holder_id": holderID,
			"locks":     locks,
			"count":     len(locks),
		})
		return
	}

	h.writeError(w, http.StatusBadRequest, "请提供 file_path、holder_id 或 lock_id 参数")
}

// handleConflicts 处理冲突查询
// GET /api/v1/filelock/conflicts?resolved=true/false
func (h *Handlers) handleConflicts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "只支持 GET 方法")
		return
	}

	var resolved *bool
	if resolvedStr := r.URL.Query().Get("resolved"); resolvedStr != "" {
		v := resolvedStr == "true"
		resolved = &v
	}

	conflicts := h.manager.ListConflicts(resolved)
	h.writeSuccess(w, http.StatusOK, "success", map[string]interface{}{
		"conflicts": conflicts,
		"count":     len(conflicts),
	})
}

// handleStats 处理统计查询
// GET /api/v1/filelock/stats
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "只支持 GET 方法")
		return
	}

	stats := h.manager.GetStatistics()
	h.writeSuccess(w, http.StatusOK, "success", stats)
}

// handleUpgrade 处理锁升级请求
// POST /api/v1/filelock/upgrade
func (h *Handlers) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "只支持 POST 方法")
		return
	}

	var req UpgradeLockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
		return
	}

	lock, err := h.manager.UpgradeLock(&req)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeSuccess(w, http.StatusOK, "锁已升级", lock)
}

// handleDowngrade 处理锁降级请求
// POST /api/v1/filelock/downgrade
func (h *Handlers) handleDowngrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "只支持 POST 方法")
		return
	}

	var req DowngradeLockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
		return
	}

	lock, err := h.manager.DowngradeLock(&req)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeSuccess(w, http.StatusOK, "锁已降级", lock)
}

// handleResolve 处理冲突解决请求
// POST /api/v1/filelock/resolve
func (h *Handlers) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "只支持 POST 方法")
		return
	}

	var req ResolveConflictRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "无效的请求格式: "+err.Error())
		return
	}

	result, err := h.resolver.ResolveConflict(&req)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeSuccess(w, http.StatusOK, "冲突已解决", result)
}

// handleSites 处理站点查询
// GET /api/v1/filelock/sites
func (h *Handlers) handleSites(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "只支持 GET 方法")
		return
	}

	sites := h.manager.GetSites()
	h.writeSuccess(w, http.StatusOK, "success", map[string]interface{}{
		"sites": sites,
		"count": len(sites),
	})
}

// handleHistory 处理历史查询
// GET /api/v1/filelock/history?limit=50
func (h *Handlers) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "只支持 GET 方法")
		return
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err != nil {
			limit = 50
		}
	}

	history := h.manager.GetHistory(limit)
	h.writeSuccess(w, http.StatusOK, "success", map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}

// ============================================================
// 响应辅助方法
// ============================================================

// writeSuccess 写入成功响应
func (h *Handlers) writeSuccess(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := APIResponse{
		Code:    0,
		Message: message,
		Data:    data,
	}
	json.NewEncoder(w).Encode(resp)
}

// writeError 写入错误响应
func (h *Handlers) writeError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := APIResponse{
		Code:    1,
		Message: message,
	}
	json.NewEncoder(w).Encode(resp)
}
