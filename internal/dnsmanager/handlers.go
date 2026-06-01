// Package dnsmanager 提供 REST API 处理器
package dnsmanager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// response 标准响应结构
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers DNS 管理模块 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 http.ServeMux
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	// DNS 记录管理
	mux.HandleFunc("/api/dnsmanager/records", h.handleRecords)
	mux.HandleFunc("/api/dnsmanager/records/", h.handleRecordByID)

	// DNS 规则管理
	mux.HandleFunc("/api/dnsmanager/rules", h.handleRules)
	mux.HandleFunc("/api/dnsmanager/rules/", h.handleRuleByID)

	// 上游服务器管理
	mux.HandleFunc("/api/dnsmanager/upstreams", h.handleUpstreams)
	mux.HandleFunc("/api/dnsmanager/upstreams/", h.handleUpstreamByID)

	// 统计信息
	mux.HandleFunc("/api/dnsmanager/stats", h.handleStats)

	// 查询日志
	mux.HandleFunc("/api/dnsmanager/querylog", h.handleQueryLog)

	// DNS 解析测试
	mux.HandleFunc("/api/dnsmanager/resolve", h.handleResolve)

	// 拦截列表导入
	mux.HandleFunc("/api/dnsmanager/import", h.handleImport)

	// 配置导出
	mux.HandleFunc("/api/dnsmanager/export", h.handleExport)
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, code int, resp response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}

// parseIDFromPath 从路径中解析 ID
func parseIDFromPath(path, prefix string) string {
	id := strings.TrimPrefix(path, prefix)
	id = strings.TrimSuffix(id, "/")
	return id
}

// ========== DNS 记录处理 ==========

// handleRecords 处理 /api/dnsmanager/records
func (h *Handlers) handleRecords(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listRecords(w, r)
	case http.MethodPost:
		h.createRecord(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "方法不允许"})
	}
}

// handleRecordByID 处理 /api/dnsmanager/records/{id}
func (h *Handlers) handleRecordByID(w http.ResponseWriter, r *http.Request) {
	id := parseIDFromPath(r.URL.Path, "/api/dnsmanager/records/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "缺少记录 ID"})
		return
	}

	switch r.Method {
	case http.MethodPut:
		h.updateRecord(w, r, id)
	case http.MethodDelete:
		h.deleteRecord(w, r, id)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "方法不允许"})
	}
}

func (h *Handlers) listRecords(w http.ResponseWriter, r *http.Request) {
	zone := r.URL.Query().Get("zone")

	records, err := h.manager.ListRecords(zone)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"total":   len(records),
			"records": records,
		},
	})
}

func (h *Handlers) createRecord(w http.ResponseWriter, r *http.Request) {
	var req CreateRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "无效的请求: " + err.Error()})
		return
	}

	record, err := h.manager.AddRecord(req.Zone, DNSRecord{
		Name:     req.Name,
		Type:     req.Type,
		Value:    req.Value,
		TTL:      req.TTL,
		Priority: req.Priority,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, response{Code: 0, Message: "created", Data: record})
}

func (h *Handlers) updateRecord(w http.ResponseWriter, r *http.Request, id string) {
	var req UpdateRecordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "无效的请求: " + err.Error()})
		return
	}

	record, err := h.manager.UpdateRecord(id, req)
	if err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{Code: 0, Message: "updated", Data: record})
}

func (h *Handlers) deleteRecord(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.DeleteRecord(id); err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== DNS 规则处理 ==========

// handleRules 处理 /api/dnsmanager/rules
func (h *Handlers) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listRules(w, r)
	case http.MethodPost:
		h.createRule(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "方法不允许"})
	}
}

// handleRuleByID 处理 /api/dnsmanager/rules/{id}
func (h *Handlers) handleRuleByID(w http.ResponseWriter, r *http.Request) {
	id := parseIDFromPath(r.URL.Path, "/api/dnsmanager/rules/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "缺少规则 ID"})
		return
	}

	switch r.Method {
	case http.MethodPut:
		h.updateRule(w, r, id)
	case http.MethodDelete:
		h.deleteRule(w, r, id)
	case http.MethodPatch:
		h.toggleRule(w, r, id)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "方法不允许"})
	}
}

func (h *Handlers) listRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.manager.ListRules()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"total": len(rules),
			"rules": rules,
		},
	})
}

func (h *Handlers) createRule(w http.ResponseWriter, r *http.Request) {
	var req CreateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "无效的请求: " + err.Error()})
		return
	}

	rule, err := h.manager.AddRule(DNSRule{
		Pattern:  req.Pattern,
		Action:   req.Action,
		Target:   req.Target,
		Category: req.Category,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, response{Code: 0, Message: "created", Data: rule})
}

func (h *Handlers) updateRule(w http.ResponseWriter, r *http.Request, id string) {
	var req UpdateRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "无效的请求: " + err.Error()})
		return
	}

	rule, err := h.manager.UpdateRule(id, req)
	if err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{Code: 0, Message: "updated", Data: rule})
}

func (h *Handlers) deleteRule(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.DeleteRule(id); err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{Code: 0, Message: "deleted"})
}

func (h *Handlers) toggleRule(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.ToggleRule(id); err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{Code: 0, Message: "toggled"})
}

// ========== 上游服务器处理 ==========

// handleUpstreams 处理 /api/dnsmanager/upstreams
func (h *Handlers) handleUpstreams(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listUpstreams(w, r)
	case http.MethodPost:
		h.createUpstream(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "方法不允许"})
	}
}

// handleUpstreamByID 处理 /api/dnsmanager/upstreams/{id}
func (h *Handlers) handleUpstreamByID(w http.ResponseWriter, r *http.Request) {
	id := parseIDFromPath(r.URL.Path, "/api/dnsmanager/upstreams/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "缺少服务器 ID"})
		return
	}

	switch r.Method {
	case http.MethodDelete:
		h.deleteUpstream(w, r, id)
	case http.MethodPost:
		// 处理 /api/dnsmanager/upstreams/{id}/test
		if strings.HasSuffix(r.URL.Path, "/test") {
			h.testUpstream(w, r, id)
		} else {
			writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "方法不允许"})
		}
	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "方法不允许"})
	}
}

func (h *Handlers) listUpstreams(w http.ResponseWriter, r *http.Request) {
	h.manager.mu.RLock()
	upstreams := make([]UpstreamServer, 0, len(h.manager.upstreams))
	for _, u := range h.manager.upstreams {
		upstreams = append(upstreams, *u)
	}
	h.manager.mu.RUnlock()

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"total":     len(upstreams),
			"upstreams": upstreams,
		},
	})
}

func (h *Handlers) createUpstream(w http.ResponseWriter, r *http.Request) {
	var req CreateUpstreamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "无效的请求: " + err.Error()})
		return
	}

	server, err := h.manager.AddUpstream(UpstreamServer{
		Address:  req.Address,
		Port:     req.Port,
		Protocol: req.Protocol,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, response{Code: 0, Message: "created", Data: server})
}

func (h *Handlers) deleteUpstream(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.RemoveUpstream(id); err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{Code: 0, Message: "deleted"})
}

func (h *Handlers) testUpstream(w http.ResponseWriter, r *http.Request, id string) {
	latency, err := h.manager.TestUpstream(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"latency_ms": latency.Milliseconds(),
		},
	})
}

// ========== 统计信息处理 ==========

func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "方法不允许"})
		return
	}

	period := r.URL.Query().Get("period")

	stats, err := h.manager.GetStats(period)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}

// ========== 查询日志处理 ==========

func (h *Handlers) handleQueryLog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "方法不允许"})
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 100
	offset := 0

	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	logs, total, err := h.manager.GetQueryLog(limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"total":  total,
			"limit":  limit,
			"offset": offset,
			"logs":   logs,
		},
	})
}

// ========== DNS 解析处理 ==========

func (h *Handlers) handleResolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "方法不允许"})
		return
	}

	var req ResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "无效的请求: " + err.Error()})
		return
	}

	queryType := req.Type
	if queryType == "" {
		queryType = "A"
	}

	// 检查是否被拦截
	blocked, rule, _ := h.manager.ShouldBlock(req.Domain)

	// 尝试解析
	record, _ := h.manager.Resolve(req.Domain, queryType)

	// 记录查询
	h.manager.LogQuery(DNSQuery{
		Client:  r.RemoteAddr,
		Domain:  req.Domain,
		Type:    queryType,
		Blocked: blocked,
	})

	result := map[string]interface{}{
		"domain":   req.Domain,
		"type":     queryType,
		"blocked":  blocked,
		"rule":     rule,
		"resolved": record != nil,
	}

	if record != nil {
		result["answer"] = record.Value
		result["ttl"] = record.TTL
	}

	writeJSON(w, http.StatusOK, response{Code: 0, Message: "success", Data: result})
}

// ========== 拦截列表导入处理 ==========

func (h *Handlers) handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "方法不允许"})
		return
	}

	var req ImportBlockListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "无效的请求: " + err.Error()})
		return
	}

	count, err := h.manager.ImportBlockList(req.URL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    0,
		Message: fmt.Sprintf("成功导入 %d 条规则", count),
		Data: map[string]interface{}{
			"imported": count,
		},
	})
}

// ========== 配置导出处理 ==========

func (h *Handlers) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "方法不允许"})
		return
	}

	data, err := h.manager.ExportConfig()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", "attachment; filename=dns-config.json")
	w.Write(data)
}
