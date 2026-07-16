// Package mobile 移动端API适配模块
// 提供精简响应、分页统一、增量同步等移动端优化接口
package mobile

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ========== 数据结构 ==========

// SyncState 同步状态记录.
type SyncState struct {
	mu        sync.RWMutex
	lastSync  map[string]time.Time // 按资源类型记录最后同步时间
	changeLog []ChangeRecord       // 变更记录
	maxLog    int                  // 最大日志条数
}

// ChangeRecord 变更记录.
type ChangeRecord struct {
	ResourceType string    `json:"resourceType"` // 资源类型
	ResourceID   string    `json:"resourceId"`   // 资源ID
	Action       string    `json:"action"`       // create/update/delete
	Timestamp    time.Time `json:"timestamp"`
	Data         any       `json:"data,omitempty"`
}

// MobileResponse 移动端精简响应.
type MobileResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
	ETag    string `json:"etag,omitempty"` // 用于缓存验证
}

// PaginatedRequest 分页请求.
type PaginatedRequest struct {
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
	Since  string `json:"since"` // 增量同步时间戳
	Sort   string `json:"sort"`
	Order  string `json:"order"`
}

// PaginatedResponse 分页响应.
type PaginatedResponse struct {
	Items      any   `json:"items"`
	Total      int   `json:"total"`
	Limit      int   `json:"limit"`
	Offset     int   `json:"offset"`
	HasMore    bool  `json:"hasMore"`
	NextOffset int   `json:"nextOffset,omitempty"`
	ServerTime int64 `json:"serverTime"` // 服务器时间戳，用于下次增量同步
}

// OfflineAction 离线队列操作.
type OfflineAction struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"` // create/update/delete
	Resource  string          `json:"resource"`
	Payload   json.RawMessage `json:"payload"`
	Timestamp time.Time       `json:"timestamp"`
	ClientID  string          `json:"clientId"`
}

// OfflineResult 离线操作结果.
type OfflineResult struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ========== 适配器 ==========

// Adapter 移动端API适配器.
type Adapter struct {
	mu         sync.RWMutex
	syncStates map[string]*SyncState // 按设备ID分组
	actions    []OfflineAction       // 离线操作队列
}

// NewAdapter 创建移动端适配器.
func NewAdapter() *Adapter {
	return &Adapter{
		syncStates: make(map[string]*SyncState),
		actions:    make([]OfflineAction, 0),
	}
}

// NormalizePagination 标准化分页参数.
func (a *Adapter) NormalizePagination(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// BuildPaginatedResponse 构建分页响应.
func (a *Adapter) BuildPaginatedResponse(items any, total, limit, offset int) *PaginatedResponse {
	hasMore := offset+limit < total
	nextOffset := 0
	if hasMore {
		nextOffset = offset + limit
	}
	return &PaginatedResponse{
		Items:      items,
		Total:      total,
		Limit:      limit,
		Offset:     offset,
		HasMore:    hasMore,
		NextOffset: nextOffset,
		ServerTime: time.Now().Unix(),
	}
}

// GenerateETag 生成ETag.
func (a *Adapter) GenerateETag(data any) string {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return ""
	}
	hash := md5.Sum(jsonBytes)
	return fmt.Sprintf(`"%x"`, hash)
}

// RecordChange 记录变更（用于增量同步）.
func (a *Adapter) RecordChange(deviceID, resourceType, resourceID, action string, data any) {
	a.mu.Lock()
	defer a.mu.Unlock()

	state, exists := a.syncStates[deviceID]
	if !exists {
		state = &SyncState{
			lastSync: make(map[string]time.Time),
			maxLog:   1000,
		}
		a.syncStates[deviceID] = state
	}

	state.mu.Lock()
	defer state.mu.Unlock()

	record := ChangeRecord{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Action:       action,
		Timestamp:    time.Now(),
		Data:         data,
	}
	state.changeLog = append(state.changeLog, record)

	// 限制日志大小
	if len(state.changeLog) > state.maxLog {
		state.changeLog = state.changeLog[len(state.changeLog)-state.maxLog:]
	}
}

// GetChanges 获取增量变更.
func (a *Adapter) GetChanges(deviceID, resourceType string, since time.Time) []ChangeRecord {
	a.mu.RLock()
	defer a.mu.RUnlock()

	state, exists := a.syncStates[deviceID]
	if !exists {
		return nil
	}

	state.mu.RLock()
	defer state.mu.RUnlock()

	var result []ChangeRecord
	for _, record := range state.changeLog {
		if record.Timestamp.After(since) {
			if resourceType == "" || record.ResourceType == resourceType {
				result = append(result, record)
			}
		}
	}
	return result
}

// UpdateLastSync 更新最后同步时间.
func (a *Adapter) UpdateLastSync(deviceID, resourceType string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	state, exists := a.syncStates[deviceID]
	if !exists {
		state = &SyncState{
			lastSync: make(map[string]time.Time),
			maxLog:   1000,
		}
		a.syncStates[deviceID] = state
	}

	state.mu.Lock()
	defer state.mu.Unlock()
	state.lastSync[resourceType] = time.Now()
}

// SubmitOfflineActions 提交离线操作队列.
func (a *Adapter) SubmitOfflineActions(actions []OfflineAction) []OfflineResult {
	results := make([]OfflineResult, len(actions))

	a.mu.Lock()
	defer a.mu.Unlock()

	for i, action := range actions {
		// 验证操作
		if action.ID == "" || action.Type == "" || action.Resource == "" {
			results[i] = OfflineResult{
				ID:      action.ID,
				Success: false,
				Error:   "缺少必要字段",
			}
			continue
		}

		// 记录操作
		a.actions = append(a.actions, action)
		results[i] = OfflineResult{
			ID:      action.ID,
			Success: true,
		}
	}
	return results
}

// GetPendingActions 获取待处理的离线操作.
func (a *Adapter) GetPendingActions(clientID string, limit int) []OfflineAction {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []OfflineAction
	for _, action := range a.actions {
		if action.ClientID == clientID {
			result = append(result, action)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

// ========== HTTP Handler ==========

// Handler 移动端API处理器.
type Handler struct {
	adapter *Adapter
}

// NewHandler 创建移动端处理器.
func NewHandler() *Handler {
	return &Handler{
		adapter: NewAdapter(),
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/mobile/sync", h.handleSync)
	mux.HandleFunc("/api/v1/mobile/offline", h.handleOffline)
	mux.HandleFunc("/api/v1/mobile/status", h.handleStatus)
}

// handleSync 增量同步端点.
func (h *Handler) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, MobileResponse{Code: 405, Message: "方法不允许"})
		return
	}

	deviceID := r.Header.Get("X-Device-ID")
	if deviceID == "" {
		deviceID = "anonymous"
	}

	resourceType := r.URL.Query().Get("type")
	sinceStr := r.URL.Query().Get("since")

	var since time.Time
	if sinceStr != "" {
		ts, err := strconv.ParseInt(sinceStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, MobileResponse{Code: 400, Message: "无效的时间戳"})
			return
		}
		since = time.Unix(ts, 0)
	} else {
		since = time.Now().Add(-24 * time.Hour) // 默认最近24小时
	}

	changes := h.adapter.GetChanges(deviceID, resourceType, since)
	h.adapter.UpdateLastSync(deviceID, resourceType)

	writeJSON(w, http.StatusOK, MobileResponse{
		Code:    0,
		Message: "success",
		Data: map[string]any{
			"changes":    changes,
			"serverTime": time.Now().Unix(),
		},
	})
}

// handleOffline 离线操作提交端点.
func (h *Handler) handleOffline(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, MobileResponse{Code: 405, Message: "方法不允许"})
		return
	}

	var actions []OfflineAction
	if err := json.NewDecoder(r.Body).Decode(&actions); err != nil {
		writeJSON(w, http.StatusBadRequest, MobileResponse{Code: 400, Message: "无效的请求体"})
		return
	}

	results := h.adapter.SubmitOfflineActions(actions)
	writeJSON(w, http.StatusOK, MobileResponse{
		Code:    0,
		Message: "success",
		Data:    results,
	})
}

// handleStatus 移动端状态端点.
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, MobileResponse{Code: 405, Message: "方法不允许"})
		return
	}

	writeJSON(w, http.StatusOK, MobileResponse{
		Code:    0,
		Message: "success",
		Data: map[string]any{
			"version":     "2.483.0",
			"apiVersion":  "v1",
			"features":    []string{"sync", "offline", "pagination", "etag"},
			"serverTime":  time.Now().Unix(),
			"maxPageSize": 100,
		},
	})
}

// writeJSON 写入JSON响应.
func writeJSON(w http.ResponseWriter, statusCode int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// SlimResponse 精简响应（移除空字段，减少流量）.
func SlimResponse(data any) any {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return data
	}

	var m map[string]any
	if err := json.Unmarshal(jsonBytes, &m); err != nil {
		return data
	}

	// 移除空值字段
	removeEmptyFields(m)
	return m
}

// removeEmptyFields 移除空值字段.
func removeEmptyFields(m map[string]any) {
	for k, v := range m {
		if v == nil {
			delete(m, k)
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			delete(m, k)
			continue
		}
		if arr, ok := v.([]any); ok && len(arr) == 0 {
			delete(m, k)
			continue
		}
		if sub, ok := v.(map[string]any); ok {
			removeEmptyFields(sub)
			if len(sub) == 0 {
				delete(m, k)
			}
		}
	}
}

// ParsePagination 从HTTP请求解析分页参数.
func ParsePagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if val, err := strconv.Atoi(l); err == nil && val > 0 {
			limit = val
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if val, err := strconv.Atoi(o); err == nil && val >= 0 {
			offset = val
		}
	}

	if limit > 100 {
		limit = 100
	}
	return
}

// ParseSince 从HTTP请求解析增量同步时间戳.
func ParseSince(r *http.Request) time.Time {
	if s := r.URL.Query().Get("since"); s != "" {
		if ts, err := strconv.ParseInt(s, 10, 64); err == nil {
			return time.Unix(ts, 0)
		}
	}
	return time.Time{} // 零值表示全量
}

// IsMobileRequest 判断是否为移动端请求.
func IsMobileRequest(r *http.Request) bool {
	ua := strings.ToLower(r.UserAgent())
	return strings.Contains(ua, "mobile") ||
		strings.Contains(ua, "nas-os-app") ||
		r.Header.Get("X-Mobile-Client") != ""
}
