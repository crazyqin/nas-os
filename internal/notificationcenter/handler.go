package notificationcenter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Handler 提供通知中心的 HTTP API 接口。
type Handler struct {
	nc *NotificationCenter
}

// NewHandler 创建并返回一个新的通知中心 HTTP 处理器。
func NewHandler(nc *NotificationCenter) *Handler {
	return &Handler{nc: nc}
}

// RegisterRoutes 注册通知中心的所有 HTTP 路由。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/notifications", h.handleNotifications)
	mux.HandleFunc("/api/notifications/unread", h.handleUnread)
	mux.HandleFunc("/api/notifications/read-all", h.handleReadAll)
	mux.HandleFunc("/api/notifications/channels", h.handleChannels)
	mux.HandleFunc("/api/notifications/stats", h.handleStats)
	mux.HandleFunc("/api/notifications/send", h.handleSend)

	// 路由匹配: /api/notifications/:id/read 和 /api/notifications/:id
	mux.HandleFunc("/api/notifications/", h.handleNotificationByID)
}

// handleNotifications 处理 GET /api/notifications - 通知列表（支持过滤、分页）。
func (h *Handler) handleNotifications(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filter := NotificationFilter{
		Limit:  50,
		Offset: 0,
	}

	// 解析查询参数
	query := r.URL.Query()

	if levels := query.Get("levels"); levels != "" {
		for _, l := range strings.Split(levels, ",") {
			filter.Levels = append(filter.Levels, NotificationLevel(strings.TrimSpace(l)))
		}
	}

	if categories := query.Get("categories"); categories != "" {
		for _, c := range strings.Split(categories, ",") {
			filter.Categories = append(filter.Categories, NotificationCategory(strings.TrimSpace(c)))
		}
	}

	if sources := query.Get("sources"); sources != "" {
		filter.Sources = strings.Split(sources, ",")
	}

	if readStr := query.Get("read"); readStr != "" {
		read := readStr == "true"
		filter.Read = &read
	}

	if keyword := query.Get("keyword"); keyword != "" {
		filter.Keyword = keyword
	}

	if groupKey := query.Get("group_key"); groupKey != "" {
		filter.GroupKey = groupKey
	}

	if limitStr := query.Get("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			filter.Limit = limit
		}
	}

	if offsetStr := query.Get("offset"); offsetStr != "" {
		var offset int
		if _, err := fmt.Sscanf(offsetStr, "%d", &offset); err == nil && offset >= 0 {
			filter.Offset = offset
		}
	}

	if startTimeStr := query.Get("start_time"); startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			filter.StartTime = &t
		}
	}

	if endTimeStr := query.Get("end_time"); endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			filter.EndTime = &t
		}
	}

	notifications, err := h.nc.Query(filter)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"notifications": notifications,
		"total":         len(notifications),
		"limit":         filter.Limit,
		"offset":        filter.Offset,
	})
}

// handleUnread 处理 GET /api/notifications/unread - 未读通知。
func (h *Handler) handleUnread(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	notifications := h.nc.GetUnread()

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"notifications": notifications,
		"total":         len(notifications),
	})
}

// handleReadAll 处理 PUT /api/notifications/read-all - 全部标记已读。
func (h *Handler) handleReadAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	count := h.nc.MarkAllAsRead()

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"marked":  count,
	})
}

// handleSend 处理 POST /api/notifications/send - 发送通知。
func (h *Handler) handleSend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var notification Notification
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	result, err := h.nc.Send(notification)
	if err != nil {
		if strings.Contains(err.Error(), "rate limit") {
			h.writeError(w, http.StatusTooManyRequests, err.Error())
			return
		}
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, result)
}

// handleChannels 处理 GET /api/notifications/channels - 渠道列表。
func (h *Handler) handleChannels(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	channels := h.nc.GetChannels()

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"channels": channels,
		"total":    len(channels),
	})
}

// handleStats 处理 GET /api/notifications/stats - 通知统计。
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.nc.GetStats()

	h.writeJSON(w, http.StatusOK, stats)
}

// handleNotificationByID 处理 /api/notifications/:id 相关的路由。
func (h *Handler) handleNotificationByID(w http.ResponseWriter, r *http.Request) {
	// 提取路径中的 ID
	path := strings.TrimPrefix(r.URL.Path, "/api/notifications/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing notification ID", http.StatusBadRequest)
		return
	}

	notificationID := parts[0]

	// /api/notifications/:id/read - PUT 标记已读
	if len(parts) == 2 && parts[1] == "read" {
		h.handleMarkRead(w, r, notificationID)
		return
	}

	// /api/notifications/channels/:id - PUT 更新渠道
	if parts[0] == "channels" && len(parts) == 2 {
		h.handleUpdateChannel(w, r, parts[1])
		return
	}

	// /api/notifications/:id - DELETE 删除通知
	if r.Method == http.MethodDelete {
		h.handleDeleteNotification(w, r, notificationID)
		return
	}

	http.Error(w, "Not found", http.StatusNotFound)
}

// handleMarkRead 处理 PUT /api/notifications/:id/read - 标记已读。
func (h *Handler) handleMarkRead(w http.ResponseWriter, r *http.Request, notificationID string) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.nc.MarkAsRead(notificationID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"notification_id": notificationID,
	})
}

// handleDeleteNotification 处理 DELETE /api/notifications/:id - 删除通知。
func (h *Handler) handleDeleteNotification(w http.ResponseWriter, r *http.Request, notificationID string) {
	if err := h.nc.Delete(notificationID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":         true,
		"notification_id": notificationID,
	})
}

// handleUpdateChannel 处理 PUT /api/notifications/channels/:id - 更新渠道配置。
func (h *Handler) handleUpdateChannel(w http.ResponseWriter, r *http.Request, channelID string) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config ChannelConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return
	}

	if err := h.nc.UpdateChannel(channelID, config); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	updated, _ := h.nc.GetChannel(channelID)
	h.writeJSON(w, http.StatusOK, updated)
}

// writeJSON 写入 JSON 响应。
func (h *Handler) writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应。
func (h *Handler) writeError(w http.ResponseWriter, statusCode int, message string) {
	h.writeJSON(w, statusCode, map[string]interface{}{
		"error":   true,
		"message": message,
	})
}
