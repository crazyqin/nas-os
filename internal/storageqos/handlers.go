package storageqos

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// QoSHandler 存储QoS HTTP处理器
// 提供完整的REST API用于管理QoS策略、监控指标、违规记录和IO限制
type QoSHandler struct {
	manager     *QoSManager
	collector   *MetricsCollector
	detector    *ViolationDetector
	controller  *IOController
	targetMgr   *TargetManager
}

// NewQoSHandler 创建处理器
// manager: QoS策略管理器
// collector: 指标采集器
// detector: 违规检测器
// controller: IO控制器
// targetMgr: 目标管理器
func NewQoSHandler(
	manager *QoSManager,
	collector *MetricsCollector,
	detector *ViolationDetector,
	controller *IOController,
	targetMgr *TargetManager,
) *QoSHandler {
	return &QoSHandler{
		manager:    manager,
		collector:  collector,
		detector:   detector,
		controller: controller,
		targetMgr:  targetMgr,
	}
}

// RegisterRoutes 注册路由
// 提供以下API端点:
//   - /api/storageqos/policies - 策略CRUD
//   - /api/storageqos/policies/{id} - 单个策略操作
//   - /api/storageqos/policies/{id}/enable - 启用策略
//   - /api/storageqos/policies/{id}/disable - 禁用策略
//   - /api/storageqos/metrics/{targetId} - 获取目标指标
//   - /api/storageqos/metrics/history/{targetId} - 获取指标历史
//   - /api/storageqos/violations - 违规记录
//   - /api/storageqos/violations/resolve - 解决违规
//   - /api/storageqos/limits - IO限制管理
//   - /api/storageqos/limits/{id} - 单个IO限制操作
//   - /api/storageqos/stats - 统计信息
//   - /api/storageqos/config - 配置管理
//   - /api/storageqos/targets - 目标管理
//   - /api/storageqos/health - 健康检查
func (h *QoSHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/storageqos/policies", h.handlePolicies)
	mux.HandleFunc("/api/storageqos/policies/", h.handlePolicyByID)
	mux.HandleFunc("/api/storageqos/policies/enable/", h.handleEnablePolicy)
	mux.HandleFunc("/api/storageqos/policies/disable/", h.handleDisablePolicy)
	mux.HandleFunc("/api/storageqos/metrics/", h.handleMetrics)
	mux.HandleFunc("/api/storageqos/violations", h.handleViolations)
	mux.HandleFunc("/api/storageqos/violations/resolve", h.handleResolveViolation)
	mux.HandleFunc("/api/storageqos/limits", h.handleLimits)
	mux.HandleFunc("/api/storageqos/limits/", h.handleLimitByID)
	mux.HandleFunc("/api/storageqos/stats", h.handleStats)
	mux.HandleFunc("/api/storageqos/config", h.handleConfig)
	mux.HandleFunc("/api/storageqos/targets", h.handleTargets)
	mux.HandleFunc("/api/storageqos/targets/", h.handleTargetByID)
	mux.HandleFunc("/api/storageqos/health", h.handleHealth)
}

// handlePolicies 处理策略请求
func (h *QoSHandler) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleListPolicies(w, r)
	case http.MethodPost:
		h.handleCreatePolicy(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePolicyByID 处理单个策略请求
// GET /api/storageqos/policies/{id} - 获取策略详情
// PUT /api/storageqos/policies/{id} - 更新策略
// DELETE /api/storageqos/policies/{id} - 删除策略
func (h *QoSHandler) handlePolicyByID(w http.ResponseWriter, r *http.Request) {
	// 提取ID
	path := strings.TrimPrefix(r.URL.Path, "/api/storageqos/policies/")
	if path == "" {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "缺少策略ID",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGetPolicy(w, r, path)
	case http.MethodPut:
		h.handleUpdatePolicy(w, r, path)
	case http.MethodDelete:
		h.handleDeletePolicy(w, r, path)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleEnablePolicy 启用策略
// POST /api/storageqos/policies/enable/{id}
func (h *QoSHandler) handleEnablePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/storageqos/policies/enable/")
	if id == "" {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "缺少策略ID",
		})
		return
	}

	if err := h.manager.EnablePolicy(id); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	log.Printf("[StorageQoS] 策略已启用: %s", id)
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
	})
}

// handleDisablePolicy 禁用策略
// POST /api/storageqos/policies/disable/{id}
func (h *QoSHandler) handleDisablePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/storageqos/policies/disable/")
	if id == "" {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "缺少策略ID",
		})
		return
	}

	if err := h.manager.DisablePolicy(id); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	log.Printf("[StorageQoS] 策略已禁用: %s", id)
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
	})
}

// handleListPolicies 列出所有策略
func (h *QoSHandler) handleListPolicies(w http.ResponseWriter, r *http.Request) {
	policies := h.manager.ListPolicies()
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    policies,
	})
}

// handleCreatePolicy 创建策略
// POST /api/storageqos/policies
// Body: QoSPolicy JSON
// Returns: 创建的策略对象
func (h *QoSHandler) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	var policy QoSPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	created, err := h.manager.CreatePolicy(&policy)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 注册目标（如果需要）
	if policy.TargetID != "" && policy.TargetType != "" {
		target, err := h.targetMgr.GetTarget(policy.TargetID)
		if err == nil {
			// 设置IO限制
			if policy.MaxIOPS > 0 {
				h.controller.SetIOPSLimit(policy.TargetID, target.DevicePath, policy.MaxIOPS)
			}
			if policy.MaxBandwidth > 0 {
				h.controller.SetBandwidthLimit(policy.TargetID, target.DevicePath, policy.MaxBandwidth)
			}
		}
	}

	log.Printf("[StorageQoS] 策略已创建: %s (%s)", created.ID, created.Name)
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    created,
	})
}

// handleGetPolicy 获取策略
func (h *QoSHandler) handleGetPolicy(w http.ResponseWriter, r *http.Request, id string) {
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    policy,
	})
}

// handleUpdatePolicy 更新策略
func (h *QoSHandler) handleUpdatePolicy(w http.ResponseWriter, r *http.Request, id string) {
	var update QoSPolicy
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	updated, err := h.manager.UpdatePolicy(id, &update)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 更新IO限制
	if updated.TargetID != "" && updated.TargetType != "" {
		target, err := h.targetMgr.GetTarget(updated.TargetID)
		if err == nil {
			if updated.MaxIOPS > 0 {
				h.controller.SetIOPSLimit(updated.TargetID, target.DevicePath, updated.MaxIOPS)
			}
			if updated.MaxBandwidth > 0 {
				h.controller.SetBandwidthLimit(updated.TargetID, target.DevicePath, updated.MaxBandwidth)
			}
		}
	}

	log.Printf("[StorageQoS] 策略已更新: %s", updated.ID)
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    updated,
	})
}

// handleDeletePolicy 删除策略
func (h *QoSHandler) handleDeletePolicy(w http.ResponseWriter, r *http.Request, id string) {
	// 获取策略信息
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	// 移除IO限制
	if policy.TargetID != "" {
		h.controller.RemoveIOLimit(policy.TargetID)
	}

	if err := h.manager.DeletePolicy(id); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	log.Printf("[StorageQoS] 策略已删除: %s", id)
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
	})
}

// handleMetrics 处理指标请求
func (h *QoSHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 提取targetId
	path := strings.TrimPrefix(r.URL.Path, "/api/storageqos/metrics/")
	if path == "" {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "缺少目标ID",
		})
		return
	}

	metrics, err := h.collector.GetMetrics(path)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    metrics,
	})
}

// handleViolations 处理违规记录请求
func (h *QoSHandler) handleViolations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 支持查询参数
	query := r.URL.Query()
	policyID := query.Get("policy_id")
	unresolved := query.Get("unresolved")

	var violations []*QoSViolation

	if policyID != "" {
		violations = h.detector.GetViolationsByPolicy(policyID)
	} else if unresolved == "true" {
		violations = h.detector.GetUnresolvedViolations()
	} else {
		violations = h.detector.GetViolations()
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    violations,
	})
}

// handleResolveViolation 处理解决违规请求
func (h *QoSHandler) handleResolveViolation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "无效的请求体",
		})
		return
	}

	if err := h.detector.ResolveViolation(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
	})
}

// handleLimits 处理IO限制请求
func (h *QoSHandler) handleLimits(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limits := h.controller.ListIOLimits()
	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    limits,
	})
}

// handleLimitByID 处理单个IO限制请求
func (h *QoSHandler) handleLimitByID(w http.ResponseWriter, r *http.Request) {
	// 提取ID
	path := strings.TrimPrefix(r.URL.Path, "/api/storageqos/limits/")
	if path == "" {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "缺少目标ID",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.handleGetLimit(w, r, path)
	case http.MethodDelete:
		h.handleDeleteLimit(w, r, path)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetLimit 获取IO限制
func (h *QoSHandler) handleGetLimit(w http.ResponseWriter, r *http.Request, id string) {
	limit, err := h.controller.GetIOLimit(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    limit,
	})
}

// handleDeleteLimit 删除IO限制
func (h *QoSHandler) handleDeleteLimit(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.controller.RemoveIOLimit(id); err != nil {
		writeJSON(w, map[string]interface{}{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
	})
}

// handleStats 处理统计请求
// GET /api/storageqos/stats
// Returns: QoS系统统计概览，包括策略数量、违规记录、监控目标等
func (h *QoSHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	policies := h.manager.ListPolicies()
	enabledPolicies := h.manager.GetEnabledPolicies()
	violations := h.detector.GetViolations()
	unresolvedViolations := h.detector.GetUnresolvedViolations()
	allMetrics := h.collector.GetAllMetrics()
	limits := h.controller.ListIOLimits()

	stats := map[string]interface{}{
		"total_policies":       len(policies),
		"enabled_policies":     len(enabledPolicies),
		"total_violations":     len(violations),
		"unresolved_violations": len(unresolvedViolations),
		"monitored_targets":    len(allMetrics),
		"active_limits":        len(limits),
		"metrics":              allMetrics,
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// handleConfig 处理配置请求
// GET /api/storageqos/config - 获取当前配置
// PUT /api/storageqos/config - 更新配置
func (h *QoSHandler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := h.manager.GetConfig()
		writeJSON(w, map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    config,
		})
	case http.MethodPut:
		var config QoSConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeJSON(w, map[string]interface{}{
				"code":    400,
				"message": "无效的请求体",
			})
			return
		}
		if err := h.manager.UpdateConfig(&config); err != nil {
			writeJSON(w, map[string]interface{}{
				"code":    400,
				"message": err.Error(),
			})
			return
		}
		log.Printf("[StorageQoS] 配置已更新")
		writeJSON(w, map[string]interface{}{
			"code":    0,
			"message": "success",
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTargets 处理目标请求
// GET /api/storageqos/targets - 列出所有目标
// POST /api/storageqos/targets - 注册新目标
func (h *QoSHandler) handleTargets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		targets := h.targetMgr.ListTargets()
		writeJSON(w, map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    targets,
		})
	case http.MethodPost:
		var target QoSTarget
		if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
			writeJSON(w, map[string]interface{}{
				"code":    400,
				"message": "无效的请求体",
			})
			return
		}
		if err := h.targetMgr.RegisterTarget(&target); err != nil {
			writeJSON(w, map[string]interface{}{
				"code":    400,
				"message": err.Error(),
			})
			return
		}
		log.Printf("[StorageQoS] 目标已注册: %s (%s)", target.ID, target.Name)
		writeJSON(w, map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    target,
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTargetByID 处理单个目标请求
// GET /api/storageqos/targets/{id} - 获取目标详情
// DELETE /api/storageqos/targets/{id} - 注销目标
func (h *QoSHandler) handleTargetByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/storageqos/targets/")
	if path == "" {
		writeJSON(w, map[string]interface{}{
			"code":    400,
			"message": "缺少目标ID",
		})
		return
	}

	switch r.Method {
	case http.MethodGet:
		target, err := h.targetMgr.GetTarget(path)
		if err != nil {
			writeJSON(w, map[string]interface{}{
				"code":    404,
				"message": err.Error(),
			})
			return
		}
		writeJSON(w, map[string]interface{}{
			"code":    0,
			"message": "success",
			"data":    target,
		})
	case http.MethodDelete:
		if err := h.targetMgr.UnregisterTarget(path); err != nil {
			writeJSON(w, map[string]interface{}{
				"code":    404,
				"message": err.Error(),
			})
			return
		}
		log.Printf("[StorageQoS] 目标已注销: %s", path)
		writeJSON(w, map[string]interface{}{
			"code":    0,
			"message": "success",
		})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleHealth 健康检查
// GET /api/storageqos/health
// Returns: 系统健康状态
func (h *QoSHandler) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	unresolvedViolations := h.detector.GetUnresolvedViolations()
	allMetrics := h.collector.GetAllMetrics()

	status := "healthy"
	if len(unresolvedViolations) > 10 {
		status = "degraded"
	}

	health := map[string]interface{}{
		"status":              status,
		"timestamp":            time.Now(),
		"monitored_targets":    len(allMetrics),
		"unresolved_violations": len(unresolvedViolations),
		"active_limits":        len(h.controller.ListIOLimits()),
	}

	writeJSON(w, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data":    health,
	})
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
