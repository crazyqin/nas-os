// Package digitallegacy 提供 REST API 处理器
package digitallegacy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Handlers 数字遗产 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux, prefix string) {
	if prefix == "" {
		prefix = "/api/v1/legacy"
	}

	// 遗产计划管理
	mux.HandleFunc(prefix+"/plans", h.handlePlans)
	mux.HandleFunc(prefix+"/plans/", h.handlePlanByID)
	mux.HandleFunc(prefix+"/plans/activate/", h.handleActivatePlan)
	mux.HandleFunc(prefix+"/plans/trigger/", h.handleTriggerPlan)

	// 受益人管理
	mux.HandleFunc(prefix+"/plans/beneficiaries/", h.handleBeneficiaries)

	// 数字资产管理
	mux.HandleFunc(prefix+"/plans/assets/", h.handleAssets)
	mux.HandleFunc(prefix+"/plans/assets/decrypt/", h.handleDecryptAsset)

	// 紧急联系人管理
	mux.HandleFunc(prefix+"/plans/contacts/", h.handleEmergencyContacts)

	// 死亡验证
	mux.HandleFunc(prefix+"/plans/death-verify/", h.handleDeathVerification)
	mux.HandleFunc(prefix+"/plans/confirm-death/", h.handleConfirmDeath)

	// 时间锁
	mux.HandleFunc(prefix+"/plans/timelock/", h.handleTimeLock)

	// 心跳
	mux.HandleFunc(prefix+"/heartbeat", h.handleHeartbeat)
	mux.HandleFunc(prefix+"/heartbeat/status/", h.handleHeartbeatStatus)

	// 信任联系人管理
	mux.HandleFunc(prefix+"/contacts", h.handleTrustContacts)

	// 访问授权管理
	mux.HandleFunc(prefix+"/access-grants/", h.handleAccessGrants)

	// 审计日志
	mux.HandleFunc(prefix+"/audit-logs/", h.handleAuditLogs)

	// 不活跃检查
	mux.HandleFunc(prefix+"/check-inactivity", h.handleCheckInactivity)

	// 配置
	mux.HandleFunc(prefix+"/config", h.handleConfig)
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, resp response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, response{
		Code:    status,
		Message: message,
	})
}

// getUserID 获取用户ID
func getUserID(r *http.Request) string {
	// 从请求头或上下文获取用户ID
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID
	}
	return "user-001"
}

// parseIDFromPath 从路径中解析 ID
func parseIDFromPath(path, prefix string) string {
	// 移除前缀，获取 ID
	id := strings.TrimPrefix(path, prefix)
	id = strings.TrimSuffix(id, "/")
	// 只取第一段
	parts := strings.Split(id, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return id
}

// parseIDsFromPath 从路径中解析多个 ID
func parseIDsFromPath(path, prefix string) (string, string) {
	id := strings.TrimPrefix(path, prefix)
	id = strings.TrimSuffix(id, "/")
	parts := strings.Split(id, "/")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	if len(parts) == 1 {
		return parts[0], ""
	}
	return "", ""
}

// handlePlans 处理遗产计划列表和创建
func (h *Handlers) handlePlans(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := getUserID(r)
		plans := h.manager.ListPlans(userID)
		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data:    plans,
		})

	case http.MethodPost:
		var req LegacyPlanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		userID := getUserID(r)
		plan, err := h.manager.CreatePlan(&req, userID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, response{
			Code:    0,
			Message: "plan created",
			Data:    plan,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handlePlanByID 处理单个遗产计划的 CRUD
func (h *Handlers) handlePlanByID(w http.ResponseWriter, r *http.Request) {
	// 解析 ID：/plans/{id}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/legacy/plans/")
	parts := strings.Split(strings.TrimSuffix(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "plan ID required")
		return
	}
	planID := parts[0]

	// 检查是否有子资源
	if len(parts) > 1 {
		h.handlePlanSubResource(w, r, planID, parts[1:])
		return
	}

	switch r.Method {
	case http.MethodGet:
		plan, err := h.manager.GetPlan(planID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data:    plan,
		})

	case http.MethodPut:
		var req LegacyPlanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		plan, err := h.manager.UpdatePlan(planID, &req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "plan updated",
			Data:    plan,
		})

	case http.MethodDelete:
		if err := h.manager.DeletePlan(planID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "plan deleted",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handlePlanSubResource 处理计划子资源
func (h *Handlers) handlePlanSubResource(w http.ResponseWriter, r *http.Request, planID string, subParts []string) {
	if len(subParts) == 0 {
		writeError(w, http.StatusBadRequest, "sub-resource required")
		return
	}

	subResource := subParts[0]
	switch subResource {
	case "beneficiaries":
		h.handleBeneficiariesByPlan(w, r, planID, subParts[1:])
	case "assets":
		h.handleAssetsByPlan(w, r, planID, subParts[1:])
	case "contacts":
		h.handleEmergencyContactsByPlan(w, r, planID, subParts[1:])
	case "will":
		h.handleWillByPlan(w, r, planID)
	case "timelock":
		h.handleTimeLockByPlan(w, r, planID)
	case "death-verify":
		h.handleDeathVerificationByPlan(w, r, planID)
	case "confirm-death":
		h.handleConfirmDeathByPlan(w, r, planID)
	case "access-grants":
		h.handleAccessGrantsByPlan(w, r, planID)
	case "audit-logs":
		h.handleAuditLogsByPlan(w, r, planID)
	default:
		writeError(w, http.StatusNotFound, fmt.Sprintf("unknown sub-resource: %s", subResource))
	}
}

// handleActivatePlan 处理激活计划
func (h *Handlers) handleActivatePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	planID := parseIDFromPath(r.URL.Path, "/api/v1/legacy/plans/activate/")
	if err := h.manager.ActivatePlan(planID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "plan activated",
	})
}

// handleTriggerPlan 处理触发计划
func (h *Handlers) handleTriggerPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	planID := parseIDFromPath(r.URL.Path, "/api/v1/legacy/plans/trigger/")

	var req TriggerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}
	req.PlanID = planID

	if err := h.manager.TriggerPlan(planID, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "plan triggered",
	})
}

// handleBeneficiaries 处理受益人路由
func (h *Handlers) handleBeneficiaries(w http.ResponseWriter, r *http.Request) {
	// 解析 planID 和 beneficiaryID
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/legacy/plans/beneficiaries/")
	path = strings.TrimSuffix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "plan ID required")
		return
	}

	planID := parts[0]
	h.handleBeneficiariesByPlan(w, r, planID, parts[1:])
}

// handleBeneficiariesByPlan 处理计划受益人
func (h *Handlers) handleBeneficiariesByPlan(w http.ResponseWriter, r *http.Request, planID string, subParts []string) {
	switch r.Method {
	case http.MethodGet:
		beneficiaries, err := h.manager.ListBeneficiaries(planID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data:    beneficiaries,
		})

	case http.MethodPost:
		var req BeneficiaryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		beneficiary, err := h.manager.AddBeneficiary(planID, &req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, response{
			Code:    0,
			Message: "beneficiary added",
			Data:    beneficiary,
		})

	case http.MethodPut:
		if len(subParts) < 1 || subParts[0] == "" {
			writeError(w, http.StatusBadRequest, "beneficiary ID required")
			return
		}

		var req BeneficiaryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		beneficiary, err := h.manager.UpdateBeneficiary(planID, subParts[0], &req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "beneficiary updated",
			Data:    beneficiary,
		})

	case http.MethodDelete:
		if len(subParts) < 1 || subParts[0] == "" {
			writeError(w, http.StatusBadRequest, "beneficiary ID required")
			return
		}

		if err := h.manager.RemoveBeneficiary(planID, subParts[0]); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "beneficiary removed",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAssets 处理资产路由
func (h *Handlers) handleAssets(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/legacy/plans/assets/")
	path = strings.TrimSuffix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "plan ID required")
		return
	}

	planID := parts[0]
	h.handleAssetsByPlan(w, r, planID, parts[1:])
}

// handleAssetsByPlan 处理计划资产
func (h *Handlers) handleAssetsByPlan(w http.ResponseWriter, r *http.Request, planID string, subParts []string) {
	switch r.Method {
	case http.MethodGet:
		assets, err := h.manager.ListAssets(planID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data:    assets,
		})

	case http.MethodPost:
		var req AssetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		asset, err := h.manager.AddAsset(planID, &req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, response{
			Code:    0,
			Message: "asset added",
			Data:    asset,
		})

	case http.MethodPut:
		if len(subParts) < 1 || subParts[0] == "" {
			writeError(w, http.StatusBadRequest, "asset ID required")
			return
		}

		var req AssetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		asset, err := h.manager.UpdateAsset(planID, subParts[0], &req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "asset updated",
			Data:    asset,
		})

	case http.MethodDelete:
		if len(subParts) < 1 || subParts[0] == "" {
			writeError(w, http.StatusBadRequest, "asset ID required")
			return
		}

		if err := h.manager.RemoveAsset(planID, subParts[0]); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "asset removed",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleDecryptAsset 处理解密资产
func (h *Handlers) handleDecryptAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	planID, assetID := parseIDsFromPath(r.URL.Path, "/api/v1/legacy/plans/assets/decrypt/")
	if planID == "" || assetID == "" {
		writeError(w, http.StatusBadRequest, "plan ID and asset ID required")
		return
	}

	data, err := h.manager.DecryptAssetData(planID, assetID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    map[string]string{"data": data},
	})
}

// handleEmergencyContacts 处理紧急联系人路由
func (h *Handlers) handleEmergencyContacts(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/legacy/plans/contacts/")
	path = strings.TrimSuffix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) < 1 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "plan ID required")
		return
	}

	planID := parts[0]
	h.handleEmergencyContactsByPlan(w, r, planID, parts[1:])
}

// handleEmergencyContactsByPlan 处理计划紧急联系人
func (h *Handlers) handleEmergencyContactsByPlan(w http.ResponseWriter, r *http.Request, planID string, subParts []string) {
	switch r.Method {
	case http.MethodGet:
		contacts, err := h.manager.ListEmergencyContacts(planID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data:    contacts,
		})

	case http.MethodPost:
		var req EmergencyContactRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		contact, err := h.manager.AddEmergencyContact(planID, &req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, response{
			Code:    0,
			Message: "emergency contact added",
			Data:    contact,
		})

	case http.MethodDelete:
		if len(subParts) < 1 || subParts[0] == "" {
			writeError(w, http.StatusBadRequest, "contact ID required")
			return
		}

		if err := h.manager.RemoveEmergencyContact(planID, subParts[0]); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "emergency contact removed",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleDeathVerification 处理死亡验证
func (h *Handlers) handleDeathVerification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	planID := parseIDFromPath(r.URL.Path, "/api/v1/legacy/plans/death-verify/")
	if planID == "" {
		writeError(w, http.StatusBadRequest, "plan ID required")
		return
	}

	var req DeathVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	dv, err := h.manager.StartDeathVerification(planID, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, response{
		Code:    0,
		Message: "death verification started",
		Data:    dv,
	})
}

// handleDeathVerificationByPlan 处理计划死亡验证
func (h *Handlers) handleDeathVerificationByPlan(w http.ResponseWriter, r *http.Request, planID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req DeathVerificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	dv, err := h.manager.StartDeathVerification(planID, &req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, response{
		Code:    0,
		Message: "death verification started",
		Data:    dv,
	})
}

// handleConfirmDeath 处理确认死亡
func (h *Handlers) handleConfirmDeath(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	planID := parseIDFromPath(r.URL.Path, "/api/v1/legacy/plans/confirm-death/")
	if planID == "" {
		writeError(w, http.StatusBadRequest, "plan ID required")
		return
	}

	var req struct {
		VerificationID    string            `json:"verification_id"`
		VerificationLevel VerificationLevel `json:"verification_level"`
		Evidence          string            `json:"evidence"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if err := h.manager.ConfirmDeath(planID, req.VerificationID, req.VerificationLevel, req.Evidence); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "death confirmed",
	})
}

// handleConfirmDeathByPlan 处理计划确认死亡
func (h *Handlers) handleConfirmDeathByPlan(w http.ResponseWriter, r *http.Request, planID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		VerificationID    string            `json:"verification_id"`
		VerificationLevel VerificationLevel `json:"verification_level"`
		Evidence          string            `json:"evidence"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if err := h.manager.ConfirmDeath(planID, req.VerificationID, req.VerificationLevel, req.Evidence); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "death confirmed",
	})
}

// handleTimeLock 处理时间锁
func (h *Handlers) handleTimeLock(w http.ResponseWriter, r *http.Request) {
	planID := parseIDFromPath(r.URL.Path, "/api/v1/legacy/plans/timelock/")
	if planID == "" {
		writeError(w, http.StatusBadRequest, "plan ID required")
		return
	}

	h.handleTimeLockByPlan(w, r, planID)
}

// handleTimeLockByPlan 处理计划时间锁
func (h *Handlers) handleTimeLockByPlan(w http.ResponseWriter, r *http.Request, planID string) {
	switch r.Method {
	case http.MethodGet:
		unlocked, err := h.manager.CheckTimeLock(planID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data:    map[string]bool{"unlocked": unlocked},
		})

	case http.MethodPost:
		var req TimeLockRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		tl, err := h.manager.SetTimeLock(planID, &req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, response{
			Code:    0,
			Message: "time lock set",
			Data:    tl,
		})

	case http.MethodPut:
		var req struct {
			VerificationLevel VerificationLevel `json:"verification_level"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		if err := h.manager.UnlockTimeLock(planID, req.VerificationLevel); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "time lock unlocked",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleHeartbeat 处理心跳
func (h *Handlers) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	userID := getUserID(r)

	var req struct {
		Note string `json:"note,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	record := h.manager.RecordHeartbeat(userID, req.Note)
	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "heartbeat recorded",
		Data:    record,
	})
}

// handleHeartbeatStatus 处理心跳状态查询
func (h *Handlers) handleHeartbeatStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ownerID := parseIDFromPath(r.URL.Path, "/api/v1/legacy/heartbeat/status/")
	if ownerID == "" {
		ownerID = getUserID(r)
	}

	status, err := h.manager.CheckHeartbeatStatus(ownerID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    map[string]string{"status": string(*status)},
	})
}

// handleTrustContacts 处理信任联系人
func (h *Handlers) handleTrustContacts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := getUserID(r)
		contacts := h.manager.GetTrustContacts(userID)
		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data:    contacts,
		})

	case http.MethodPost:
		var contact TrustContact
		if err := json.NewDecoder(r.Body).Decode(&contact); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		userID := getUserID(r)
		result, err := h.manager.AddTrustContact(userID, &contact)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, response{
			Code:    0,
			Message: "trust contact added",
			Data:    result,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAccessGrants 处理访问授权
func (h *Handlers) handleAccessGrants(w http.ResponseWriter, r *http.Request) {
	// 解析 grant ID
	grantID := parseIDFromPath(r.URL.Path, "/api/v1/legacy/access-grants/")

	switch r.Method {
	case http.MethodDelete:
		if grantID == "" {
			writeError(w, http.StatusBadRequest, "grant ID required")
			return
		}

		if err := h.manager.RevokeAccessGrant(grantID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "access grant revoked",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleAccessGrantsByPlan 处理计划访问授权
func (h *Handlers) handleAccessGrantsByPlan(w http.ResponseWriter, r *http.Request, planID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	grants := h.manager.GetAllAccessGrants(planID)
	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    grants,
	})
}

// handleAuditLogs 处理审计日志
func (h *Handlers) handleAuditLogs(w http.ResponseWriter, r *http.Request) {
	planID := parseIDFromPath(r.URL.Path, "/api/v1/legacy/audit-logs/")
	h.handleAuditLogsByPlan(w, r, planID)
}

// handleAuditLogsByPlan 处理计划审计日志
func (h *Handlers) handleAuditLogsByPlan(w http.ResponseWriter, r *http.Request, planID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	logs := h.manager.GetAuditLogs(planID, limit)
	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    logs,
	})
}

// handleCheckInactivity 处理不活跃检查
func (h *Handlers) handleCheckInactivity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	checks := h.manager.CheckInactivity()
	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    checks,
	})
}

// handleConfig 处理配置
func (h *Handlers) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := h.manager.GetConfig()
		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data:    cfg,
		})

	case http.MethodPut:
		var cfg DefaultLegacyConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		h.manager.UpdateConfig(&cfg)
		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "config updated",
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleWillByPlan 处理遗嘱文档
func (h *Handlers) handleWillByPlan(w http.ResponseWriter, r *http.Request, planID string) {
	switch r.Method {
	case http.MethodGet:
		doc, err := h.manager.GetWillDocument(planID, true)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data:    doc,
		})

	case http.MethodPost:
		var req WillDocumentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
			return
		}

		doc, err := h.manager.SetWillDocument(planID, &req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, response{
			Code:    0,
			Message: "will document set",
			Data:    doc,
		})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
