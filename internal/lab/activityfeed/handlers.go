package activityfeed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Handler 提供活动流的 HTTP API 处理器。
type Handler struct {
	feed *Feed
}

// NewHandler 创建一个新的活动流 API 处理器。
// feed 参数是活动流的核心管理器实例。
func NewHandler(feed *Feed) *Handler {
	return &Handler{feed: feed}
}

// RegisterRoutes 注册所有活动流相关的 HTTP 路由。
// 它将处理器绑定到指定的路由前缀下。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/activities", h.handleActivities)
	mux.HandleFunc("/api/activities/summary", h.handleSummary)
	mux.HandleFunc("/api/activities/export", h.handleExport)
	mux.HandleFunc("/api/activities/subscribe", h.handleSubscribe)
}

// handleActivities 处理活动列表的 GET 和 POST 请求。
// GET: 查询活动列表，支持多种过滤参数
// POST: 记录新的活动.
func (h *Handler) handleActivities(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getActivities(w, r)
	case http.MethodPost:
		h.createActivity(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getActivities 处理 GET /api/activities 请求。
// 支持的查询参数：
//   - service: 服务类型过滤（可多个，逗号分隔）
//   - actor: 执行者ID过滤（可多个，逗号分隔）
//   - severity: 严重级别过滤（可多个，逗号分隔）
//   - start_time: 开始时间（RFC3339格式）
//   - end_time: 结束时间（RFC3339格式）
//   - resource: 资源关键词
//   - keyword: 搜索关键词
//   - limit: 返回数量限制（默认100）
//   - offset: 分页偏移量
func (h *Handler) getActivities(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := ActivityFilter{
		Limit: 100,
	}

	// 解析服务过滤
	if services := query.Get("service"); services != "" {
		for _, s := range strings.Split(services, ",") {
			filter.Services = append(filter.Services, ServiceType(strings.TrimSpace(s)))
		}
	}

	// 解析执行者过滤
	if actors := query.Get("actor"); actors != "" {
		for _, a := range strings.Split(actors, ",") {
			filter.ActorIDs = append(filter.ActorIDs, strings.TrimSpace(a))
		}
	}

	// 解析严重级别过滤
	if severities := query.Get("severity"); severities != "" {
		for _, s := range strings.Split(severities, ",") {
			filter.Severities = append(filter.Severities, Severity(strings.TrimSpace(s)))
		}
	}

	// 解析时间范围
	if startTime := query.Get("start_time"); startTime != "" {
		t, err := time.Parse(time.RFC3339, startTime)
		if err != nil {
			http.Error(w, "Invalid start_time format, use RFC3339", http.StatusBadRequest)
			return
		}
		filter.StartTime = &t
	}

	if endTime := query.Get("end_time"); endTime != "" {
		t, err := time.Parse(time.RFC3339, endTime)
		if err != nil {
			http.Error(w, "Invalid end_time format, use RFC3339", http.StatusBadRequest)
			return
		}
		filter.EndTime = &t
	}

	// 解析资源和关键词
	filter.Resource = query.Get("resource")
	filter.Keyword = query.Get("keyword")

	// 解析分页参数
	if limit := query.Get("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filter.Limit = l
		}
	}

	if offset := query.Get("offset"); offset != "" {
		if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
			filter.Offset = o
		}
	}

	// 执行查询
	activities, err := h.feed.QueryActivities(filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 返回结果
	response := map[string]interface{}{
		"activities": activities,
		"count":      len(activities),
		"filter":     filter,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// createActivity 处理 POST /api/activities 请求。
// 请求体应为 JSON 格式的 Activity 对象。
func (h *Handler) createActivity(w http.ResponseWriter, r *http.Request) {
	var activity Activity
	if err := json.NewDecoder(r.Body).Decode(&activity); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	// 记录活动
	recorded, err := h.feed.RecordActivity(activity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(recorded)
}

// handleSummary 处理 GET /api/activities/summary 请求。
// 支持的查询参数：
//   - period: 摘要类型（daily/weekly，默认daily）
//   - start_time: 开始时间（RFC3339格式）
//   - end_time: 结束时间（RFC3339格式）
func (h *Handler) handleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	period := query.Get("period")
	if period == "" {
		period = "daily"
	}

	// 解析时间范围，默认最近24小时
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)

	if st := query.Get("start_time"); st != "" {
		t, err := time.Parse(time.RFC3339, st)
		if err != nil {
			http.Error(w, "Invalid start_time format", http.StatusBadRequest)
			return
		}
		startTime = t
	}

	if et := query.Get("end_time"); et != "" {
		t, err := time.Parse(time.RFC3339, et)
		if err != nil {
			http.Error(w, "Invalid end_time format", http.StatusBadRequest)
			return
		}
		endTime = t
	}

	// 生成摘要
	summary, err := h.feed.GetSummary(period, startTime, endTime)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// handleExport 处理 GET /api/activities/export 请求。
// 支持的查询参数：
//   - format: 导出格式（json/csv，默认json）
//   - 其他过滤参数同 GET /api/activities
func (h *Handler) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	format := ExportFormat(query.Get("format"))
	if format == "" {
		format = FormatJSON
	}

	// 构建过滤条件
	filter := ActivityFilter{
		Limit: 10000, // 导出时允许更多
	}

	if services := query.Get("service"); services != "" {
		for _, s := range strings.Split(services, ",") {
			filter.Services = append(filter.Services, ServiceType(strings.TrimSpace(s)))
		}
	}

	if actors := query.Get("actor"); actors != "" {
		for _, a := range strings.Split(actors, ",") {
			filter.ActorIDs = append(filter.ActorIDs, strings.TrimSpace(a))
		}
	}

	if severities := query.Get("severity"); severities != "" {
		for _, s := range strings.Split(severities, ",") {
			filter.Severities = append(filter.Severities, Severity(strings.TrimSpace(s)))
		}
	}

	if startTime := query.Get("start_time"); startTime != "" {
		t, err := time.Parse(time.RFC3339, startTime)
		if err != nil {
			http.Error(w, "Invalid start_time format", http.StatusBadRequest)
			return
		}
		filter.StartTime = &t
	}

	if endTime := query.Get("end_time"); endTime != "" {
		t, err := time.Parse(time.RFC3339, endTime)
		if err != nil {
			http.Error(w, "Invalid end_time format", http.StatusBadRequest)
			return
		}
		filter.EndTime = &t
	}

	filter.Resource = query.Get("resource")
	filter.Keyword = query.Get("keyword")

	// 执行导出
	exportData, err := h.feed.ExportFeed(filter, format)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 设置响应头
	switch format {
	case FormatJSON:
		w.Header().Set("Content-Type", "application/json")
	case FormatCSV:
		w.Header().Set("Content-Type", "text/csv")
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", exportData.Filename))

	w.Write(exportData.Content)
}

// handleSubscribe 处理 POST /api/activities/subscribe 请求。
// 请求体应为 JSON 格式，包含 url 和 filter 字段。
// 返回订阅ID，后续可通过 SSE 或 WebSocket 接收事件。
func (h *Handler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		URL    string         `json:"url"`
		Filter ActivityFilter `json:"filter"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	// 创建订阅
	subID, _, err := h.feed.Subscribe(req.URL, req.Filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"subscription_id": subID,
		"message":         "Subscription created successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}
