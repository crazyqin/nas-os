package massdeploy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ==================== 辅助函数 ====================

func generateID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func extractID(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}

// ==================== 设备发现 Handlers ====================

func (m *Manager) handleDiscover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ScanRequest
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := m.ScanNetwork(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, result)
}

func (m *Manager) handleDiscovered(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices := m.ListDiscoveredDevices()
	writeJSON(w, devices)
}

// ==================== 资产管理 Handlers ====================

func (m *Manager) handleAssets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		assetType := AssetType(r.URL.Query().Get("type"))
		status := AssetStatus(r.URL.Query().Get("status"))
		assets := m.ListAssets(assetType, status)
		writeJSON(w, assets)
	case http.MethodPost:
		var asset Asset
		if err := decodeJSON(r, &asset); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.AddAsset(&asset); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, asset)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleAssetDetail(w http.ResponseWriter, r *http.Request) {
	assetID := extractID(r.URL.Path, "/api/massdeploy/assets/")
	if assetID == "" {
		http.Error(w, "asset_id required", http.StatusBadRequest)
		return
	}

	// 排除 hardware 路径
	if strings.HasPrefix(assetID, "hardware/") {
		m.handleHardwareInfo(w, r)
		return
	}

	switch r.Method {
	case http.MethodGet:
		asset, err := m.GetAsset(assetID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, asset)
	case http.MethodPut:
		var asset Asset
		if err := decodeJSON(r, &asset); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		asset.ID = assetID
		if err := m.UpdateAsset(&asset); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, asset)
	case http.MethodDelete:
		if err := m.RemoveAsset(assetID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleHardwareInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	assetID := extractID(r.URL.Path, "/api/massdeploy/assets/hardware/")
	if assetID == "" {
		http.Error(w, "asset_id required", http.StatusBadRequest)
		return
	}

	info, err := m.GetHardwareInfo(assetID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, info)
}

// ==================== 部署模板 Handlers ====================

func (m *Manager) handleTemplates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		templates := m.ListTemplates()
		writeJSON(w, templates)
	case http.MethodPost:
		var tmpl ConfigTemplate
		if err := decodeJSON(r, &tmpl); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateTemplate(&tmpl); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, tmpl)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleTemplateDetail(w http.ResponseWriter, r *http.Request) {
	templateID := extractID(r.URL.Path, "/api/massdeploy/templates/")
	if templateID == "" {
		http.Error(w, "template_id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		tmpl, err := m.GetTemplate(templateID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, tmpl)
	case http.MethodPut:
		var tmpl ConfigTemplate
		if err := decodeJSON(r, &tmpl); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		tmpl.ID = templateID
		if err := m.UpdateTemplate(&tmpl); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, tmpl)
	case http.MethodDelete:
		if err := m.DeleteTemplate(templateID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "deleted"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ==================== 部署任务 Handlers ====================

func (m *Manager) handleDeploy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status := JobStatus(r.URL.Query().Get("status"))
		jobs := m.ListDeployJobs(status)
		writeJSON(w, jobs)
	case http.MethodPost:
		var job DeployJob
		if err := decodeJSON(r, &job); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateDeployJob(&job); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, job)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleDeployDetail(w http.ResponseWriter, r *http.Request) {
	jobID := extractID(r.URL.Path, "/api/massdeploy/deploy/")
	if jobID == "" {
		http.Error(w, "job_id required", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	job, err := m.GetDeployJob(jobID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, job)
}

func (m *Manager) handleCancelDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := extractID(r.URL.Path, "/api/massdeploy/deploy/cancel/")
	if jobID == "" {
		http.Error(w, "job_id required", http.StatusBadRequest)
		return
	}

	if err := m.CancelDeployJob(jobID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "cancelled"})
}

func (m *Manager) handleRetryDeploy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobID := extractID(r.URL.Path, "/api/massdeploy/deploy/retry/")
	if jobID == "" {
		http.Error(w, "job_id required", http.StatusBadRequest)
		return
	}

	if err := m.RetryDeployJob(jobID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "retrying"})
}

// ==================== 固件管理 Handlers ====================

func (m *Manager) handleFirmware(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		model := r.URL.Query().Get("model")
		infos := m.ListFirmwareInfo(model)
		writeJSON(w, infos)
	case http.MethodPost:
		var info FirmwareInfo
		if err := decodeJSON(r, &info); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.AddFirmwareInfo(&info); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, info)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleFirmwareUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var job FirmwareUpgradeJob
	if err := decodeJSON(r, &job); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.CreateFirmwareUpgradeJob(&job); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 异步执行
	go m.ExecuteFirmwareUpgrade(job.ID)

	writeJSON(w, job)
}

func (m *Manager) handleFirmwareRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		DeviceID string `json:"device_id"`
		Version  string `json:"version"`
	}
	if err := decodeJSON(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.RollbackFirmware(req.DeviceID, req.Version); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"status": "rolled_back"})
}

func (m *Manager) handleFirmwareCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	updates := m.CheckFirmwareUpdates()
	writeJSON(w, updates)
}

// ==================== 费用统计 Handlers ====================

func (m *Manager) handleCosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		assetID := r.URL.Query().Get("asset_id")
		costType := CostType(r.URL.Query().Get("type"))
		records := m.GetCostRecords(assetID, costType)
		writeJSON(w, records)
	case http.MethodPost:
		var record CostRecord
		if err := decodeJSON(r, &record); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.AddCostRecord(&record); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, record)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleCostSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	period := r.URL.Query().Get("period")
	summary := m.GetCostSummary(period)
	writeJSON(w, summary)
}

func (m *Manager) handleDepreciation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	assetID := extractID(r.URL.Path, "/api/massdeploy/costs/depreciation/")
	if assetID == "" {
		http.Error(w, "asset_id required", http.StatusBadRequest)
		return
	}

	info, err := m.CalculateDepreciation(assetID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, info)
}

// ==================== 报告 Handlers ====================

func (m *Manager) handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	reportType := ReportType(r.URL.Query().Get("type"))
	reports := m.ListReports(reportType)
	writeJSON(w, reports)
}

func (m *Manager) handleDeployReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Period string `json:"period"`
	}
	decodeJSON(r, &req)

	report := m.GenerateDeployReport(req.Period)
	writeJSON(w, report)
}

func (m *Manager) handleAssetReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Period string `json:"period"`
	}
	decodeJSON(r, &req)

	report := m.GenerateAssetReport(req.Period)
	writeJSON(w, report)
}

func (m *Manager) handleCostReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Period string `json:"period"`
	}
	decodeJSON(r, &req)

	report := m.GenerateCostReport(req.Period)
	writeJSON(w, report)
}

// ==================== 统计信息 Handler ====================

func (m *Manager) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := m.GetStats()
	writeJSON(w, stats)
}

// ==================== 事件 Handler ====================

func (m *Manager) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}

	events := m.GetEvents(limit)
	writeJSON(w, events)
}
