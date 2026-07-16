// HTTP API handlers for backup lifecycle management
package smartlifebackup

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Handlers HTTP API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/backup/lifecycle", h.handleLifecycle)
	mux.HandleFunc("/api/v1/backup/lifecycle/policies", h.handlePolicies)
	mux.HandleFunc("/api/v1/backup/lifecycle/backups", h.handleBackups)
	mux.HandleFunc("/api/v1/backup/lifecycle/tasks", h.handleTasks)
	mux.HandleFunc("/api/v1/backup/lifecycle/stats", h.handleStats)
	mux.HandleFunc("/api/v1/backup/lifecycle/cost", h.handleCost)
	mux.HandleFunc("/api/v1/backup/lifecycle/schedule", h.handleSchedule)
	mux.HandleFunc("/api/v1/backup/lifecycle/health", h.handleHealth)
}

// handleLifecycle 处理生命周期主端点.
func (h *Handlers) handleLifecycle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getLifecycleStatus(w, r)
	case http.MethodPost:
		h.executeLifecycle(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// getLifecycleStatus 获取生命周期状态.
func (h *Handlers) getLifecycleStatus(w http.ResponseWriter, r *http.Request) {
	policy, err := h.manager.GetActivePolicy()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	stats := h.manager.GetStats()

	response := map[string]interface{}{
		"policy": policy,
		"stats":  stats,
	}

	writeJSON(w, http.StatusOK, response)
}

// executeLifecycle 执行生命周期管理.
func (h *Handlers) executeLifecycle(w http.ResponseWriter, r *http.Request) {
	var req LifecycleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效的请求体："+err.Error())
		return
	}

	// 处理不同的action
	switch req.Action {
	case "execute":
		task, err := h.manager.ExecuteLifecycle(r.Context(), req.Options)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, LifecycleResponse{
			Success: true,
			Message: "生命周期任务已启动",
			TaskID:  task.ID,
			Data:    task,
		})

	case "dry_run":
		options := &ExecuteOptions{DryRun: true}
		if req.Options != nil {
			options = req.Options
			options.DryRun = true
		}
		task, err := h.manager.ExecuteLifecycle(r.Context(), options)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, LifecycleResponse{
			Success: true,
			Message: "试运行任务已启动",
			TaskID:  task.ID,
			Data:    task,
		})

	case "create_policy":
		if req.Policy == nil {
			writeError(w, http.StatusBadRequest, "策略信息不能为空")
			return
		}
		if err := h.manager.CreatePolicy(req.Policy); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, LifecycleResponse{
			Success: true,
			Message: "策略创建成功",
			Data:    req.Policy,
		})

	default:
		writeError(w, http.StatusBadRequest, "未知的操作："+req.Action)
	}
}

// handlePolicies 处理策略相关请求.
func (h *Handlers) handlePolicies(w http.ResponseWriter, r *http.Request) {
	// 提取策略ID（如果有）
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/backup/lifecycle/policies")
	policyID := strings.TrimPrefix(path, "/")

	switch r.Method {
	case http.MethodGet:
		if policyID != "" {
			// 获取单个策略
			policy, err := h.manager.GetPolicy(policyID)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, policy)
		} else {
			// 获取所有策略
			policies := h.manager.ListPolicies()
			writeJSON(w, http.StatusOK, policies)
		}

	case http.MethodPost:
		var policy BackupPolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求体："+err.Error())
			return
		}
		if err := h.manager.CreatePolicy(&policy); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, policy)

	case http.MethodPut:
		if policyID == "" {
			writeError(w, http.StatusBadRequest, "策略ID不能为空")
			return
		}
		var policy BackupPolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求体："+err.Error())
			return
		}
		if err := h.manager.UpdatePolicy(policyID, &policy); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "策略更新成功"})

	case http.MethodDelete:
		if policyID == "" {
			writeError(w, http.StatusBadRequest, "策略ID不能为空")
			return
		}
		if err := h.manager.DeletePolicy(policyID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "策略删除成功"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleBackups 处理备份项相关请求.
func (h *Handlers) handleBackups(w http.ResponseWriter, r *http.Request) {
	// 提取备份ID（如果有）
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/backup/lifecycle/backups")
	backupID := strings.TrimPrefix(path, "/")

	switch r.Method {
	case http.MethodGet:
		if backupID != "" {
			backup, err := h.manager.GetBackup(backupID)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, backup)
		} else {
			backups := h.manager.ListBackups()
			writeJSON(w, http.StatusOK, backups)
		}

	case http.MethodPost:
		var item BackupItem
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求体："+err.Error())
			return
		}
		if err := h.manager.RegisterBackup(&item); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, item)

	case http.MethodDelete:
		if backupID == "" {
			writeError(w, http.StatusBadRequest, "备份ID不能为空")
			return
		}
		if err := h.manager.DeleteBackup(backupID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "备份删除成功"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleTasks 处理任务相关请求.
func (h *Handlers) handleTasks(w http.ResponseWriter, r *http.Request) {
	// 提取任务ID（如果有）
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/backup/lifecycle/tasks")
	taskID := strings.TrimPrefix(path, "/")

	switch r.Method {
	case http.MethodGet:
		if taskID != "" {
			task, err := h.manager.GetTask(taskID)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, task)
		} else {
			tasks := h.manager.ListTasks()
			writeJSON(w, http.StatusOK, tasks)
		}

	case http.MethodDelete:
		if taskID == "" {
			writeError(w, http.StatusBadRequest, "任务ID不能为空")
			return
		}
		if err := h.manager.CancelTask(taskID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "任务已取消"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleStats 处理统计信息请求.
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	stats := h.manager.GetStats()
	writeJSON(w, http.StatusOK, stats)
}

// handleCost 处理成本相关请求.
func (h *Handlers) handleCost(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	report := h.manager.GetCostReport()
	writeJSON(w, http.StatusOK, report)
}

// handleSchedule 处理调度配置请求.
func (h *Handlers) handleSchedule(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := h.manager.GetScheduleConfig()
		writeJSON(w, http.StatusOK, config)

	case http.MethodPut:
		var config ScheduleConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求体："+err.Error())
			return
		}
		if err := h.manager.UpdateScheduleConfig(&config); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "调度配置更新成功"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
	}
}

// handleHealth 处理健康检查请求.
func (h *Handlers) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "方法不允许")
		return
	}

	health := h.manager.GetHealthCheck()
	writeJSON(w, http.StatusOK, health)
}

// ============================================================================
// 辅助函数
// ============================================================================

// writeJSON 写入JSON响应.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[SmartLifeBackup] 编码响应失败：%v", err)
	}
}

// writeError 写入错误响应.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]interface{}{
		"error":     true,
		"message":   message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// writeValidationError 写入验证错误响应.
func writeValidationError(w http.ResponseWriter, field string) {
	writeJSON(w, http.StatusBadRequest, map[string]interface{}{
		"error":   true,
		"message": fmt.Sprintf("参数验证失败：%s", field),
	})
}
