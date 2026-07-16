// Package datagovernance - REST API handlers
package datagovernance

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler HTTP 处理器.
type Handler struct {
	manager *Manager
}

// NewHandler 创建处理器.
func NewHandler(m *Manager) *Handler {
	return &Handler{manager: m}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// 引擎状态
	mux.HandleFunc("/api/v1/governance/status", h.handleStatus)
	mux.HandleFunc("/api/v1/governance/stats", h.handleStats)
	mux.HandleFunc("/api/v1/governance/config", h.handleConfig)

	// 数据资产
	mux.HandleFunc("/api/v1/governance/assets", h.handleAssets)
	mux.HandleFunc("/api/v1/governance/asset", h.handleAsset)
	mux.HandleFunc("/api/v1/governance/asset/register", h.handleRegisterAsset)
	mux.HandleFunc("/api/v1/governance/asset/classify", h.handleClassifyAsset)
	mux.HandleFunc("/api/v1/governance/asset/delete", h.handleDeleteAsset)
	mux.HandleFunc("/api/v1/governance/asset/relocate", h.handleRelocateAsset)
	mux.HandleFunc("/api/v1/governance/classify/auto", h.handleAutoClassify)

	// 驻留合规
	mux.HandleFunc("/api/v1/governance/residency/check", h.handleResidencyCheck)
	mux.HandleFunc("/api/v1/governance/residency/violations", h.handleResidencyViolations)

	// 保留策略
	mux.HandleFunc("/api/v1/governance/policies", h.handlePolicies)
	mux.HandleFunc("/api/v1/governance/policy", h.handlePolicy)
	mux.HandleFunc("/api/v1/governance/policy/create", h.handleCreatePolicy)
	mux.HandleFunc("/api/v1/governance/policy/update", h.handleUpdatePolicy)
	mux.HandleFunc("/api/v1/governance/policy/delete", h.handleDeletePolicy)
	mux.HandleFunc("/api/v1/governance/retention/enforce", h.handleEnforceRetention)

	// 审计追踪
	mux.HandleFunc("/api/v1/governance/audit", h.handleAuditLog)
	mux.HandleFunc("/api/v1/governance/audit/log", h.handleLogAudit)

	// 合规报告
	mux.HandleFunc("/api/v1/governance/report/generate", h.handleGenerateReport)
	mux.HandleFunc("/api/v1/governance/report", h.handleGetReport)
	mux.HandleFunc("/api/v1/governance/reports", h.handleListReports)

	// 数据血缘
	mux.HandleFunc("/api/v1/governance/lineage/add", h.handleAddLineage)
	mux.HandleFunc("/api/v1/governance/lineage", h.handleGetLineage)
	mux.HandleFunc("/api/v1/governance/lineage/upstream", h.handleGetLineageUpstream)
}

// ========== 引擎状态 ==========

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.manager.GetStats()
	writeJSON(w, map[string]interface{}{
		"running":             h.manager.IsRunning(),
		"totalAssets":         stats.TotalAssets,
		"totalPolicies":       stats.TotalPolicies,
		"activePolicies":      stats.ActivePolicies,
		"totalAuditRecords":   stats.TotalAuditRecords,
		"residencyViolations": stats.ResidencyViolations,
	})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, h.manager.GetStats())
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, h.manager.GetConfig())
	case http.MethodPut:
		var cfg Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		h.manager.UpdateConfig(cfg)
		writeJSON(w, map[string]string{"status": "updated"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// ========== 数据资产 ==========

func (h *Handler) handleAssets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sensitivity := SensitivityLevel(r.URL.Query().Get("sensitivity"))
	region := GeoRegion(r.URL.Query().Get("region"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 20
	}
	assets, total := h.manager.ListAssets(sensitivity, region, page, pageSize)
	writeJSON(w, map[string]interface{}{
		"assets":   assets,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) handleAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	asset, err := h.manager.GetAsset(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, asset)
}

func (h *Handler) handleRegisterAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var asset DataAsset
	if err := json.NewDecoder(r.Body).Decode(&asset); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.RegisterAsset(asset); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "registered", "id": asset.ID})
}

func (h *Handler) handleClassifyAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AssetID      string           `json:"assetId"`
		Sensitivity  SensitivityLevel `json:"sensitivity"`
		ClassifiedBy string           `json:"classifiedBy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.ClassifyAsset(req.AssetID, req.Sensitivity, req.ClassifiedBy); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "classified"})
}

func (h *Handler) handleDeleteAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		id = req.ID
	}
	if err := h.manager.DeleteAsset(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "deleted"})
}

func (h *Handler) handleRelocateAsset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AssetID   string    `json:"assetId"`
		NewRegion GeoRegion `json:"newRegion"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.RelocateAsset(req.AssetID, req.NewRegion); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "relocated"})
}

func (h *Handler) handleAutoClassify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	count := h.manager.AutoClassify()
	writeJSON(w, map[string]interface{}{"classified": count})
}

// ========== 驻留合规 ==========

func (h *Handler) handleResidencyCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	compliant, err := h.manager.CheckResidency(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]interface{}{"assetId": id, "compliant": compliant})
}

func (h *Handler) handleResidencyViolations(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	violations := h.manager.CheckAllResidency()
	writeJSON(w, map[string]interface{}{"violations": violations, "count": len(violations)})
}

// ========== 保留策略 ==========

func (h *Handler) handlePolicies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, h.manager.ListPolicies())
}

func (h *Handler) handlePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, policy)
}

func (h *Handler) handleCreatePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var policy RetentionPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.CreatePolicy(policy); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "created", "id": policy.ID})
}

func (h *Handler) handleUpdatePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	var policy RetentionPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.UpdatePolicy(id, policy); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "updated"})
}

func (h *Handler) handleDeletePolicy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		id = req.ID
	}
	if err := h.manager.DeletePolicy(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "deleted"})
}

func (h *Handler) handleEnforceRetention(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	expired := h.manager.EnforceRetention()
	writeJSON(w, map[string]interface{}{"expiredAssets": expired, "count": len(expired)})
}

// ========== 审计追踪 ==========

func (h *Handler) handleAuditLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := r.URL.Query().Get("userId")
	action := r.URL.Query().Get("action")
	assetID := r.URL.Query().Get("assetId")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 50
	}
	events, total := h.manager.GetAuditLog(userID, action, assetID, page, pageSize)
	writeJSON(w, map[string]interface{}{
		"records":  events,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *Handler) handleLogAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var record AuditRecord
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	h.manager.LogAudit(record)
	writeJSON(w, map[string]string{"status": "logged"})
}

// ========== 合规报告 ==========

func (h *Handler) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Framework ComplianceFramework `json:"framework"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	report := h.manager.GenerateReport(req.Framework)
	writeJSON(w, report)
}

func (h *Handler) handleGetReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	report, err := h.manager.GetReport(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, report)
}

func (h *Handler) handleListReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	framework := ComplianceFramework(r.URL.Query().Get("framework"))
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("pageSize"))
	if pageSize < 1 {
		pageSize = 20
	}
	reports, total := h.manager.ListReports(framework, page, pageSize)
	writeJSON(w, map[string]interface{}{
		"reports":  reports,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// ========== 数据血缘 ==========

func (h *Handler) handleAddLineage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var record LineageRecord
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.AddLineage(record); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]string{"status": "added", "id": record.ID})
}

func (h *Handler) handleGetLineage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	assetID := r.URL.Query().Get("assetId")
	records, err := h.manager.GetLineage(assetID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]interface{}{"assetId": assetID, "lineage": records, "count": len(records)})
}

func (h *Handler) handleGetLineageUpstream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	assetID := r.URL.Query().Get("assetId")
	chain := h.manager.GetLineageUpstream(assetID)
	writeJSON(w, map[string]interface{}{"assetId": assetID, "upstream": chain, "count": len(chain)})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
