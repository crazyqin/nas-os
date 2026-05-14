package notifycenter

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler 通知中心 HTTP 处理器.
type Handler struct {
	mgr *Manager
}

// NewHandler 创建处理器.
func NewHandler(mgr *Manager) *Handler {
	return &Handler{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/notify/notifications", h.handleNotifications)
	mux.HandleFunc("/api/notify/notifications/read/", h.handleMarkRead)
	mux.HandleFunc("/api/notify/notifications/read-all", h.handleMarkAllRead)
	mux.HandleFunc("/api/notify/notifications/unread-count", h.handleUnreadCount)
	mux.HandleFunc("/api/notify/channels", h.handleChannels)
	mux.HandleFunc("/api/notify/templates", h.handleTemplates)
	mux.HandleFunc("/api/notify/preferences/", h.handlePreferences)
	mux.HandleFunc("/api/notify/stats", h.handleStats)
}

func (h *Handler) handleNotifications(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		opts := ListOptions{
			UnreadOnly: r.URL.Query().Get("unread") == "true",
			Channel:    ChannelType(r.URL.Query().Get("channel")),
			Source:     r.URL.Query().Get("source"),
		}
		if l := r.URL.Query().Get("limit"); l != "" {
			if v, err := strconv.Atoi(l); err == nil {
				opts.Limit = v
			}
		}
		if opts.Limit == 0 {
			opts.Limit = 50
		}
		writeJSON(w, http.StatusOK, h.mgr.List(opts))
	case http.MethodPost:
		var notif Notification
		if err := json.NewDecoder(r.Body).Decode(&notif); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		h.mgr.Send(&notif)
		writeJSON(w, http.StatusCreated, notif)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	id := r.URL.Path[len("/api/notify/notifications/read/"):]
	if h.mgr.MarkRead(id) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "read"})
	} else {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (h *Handler) handleMarkAllRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	count := h.mgr.MarkAllRead()
	writeJSON(w, http.StatusOK, map[string]int{"marked": count})
}

func (h *Handler) handleUnreadCount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"unread": h.mgr.UnreadCount()})
}

func (h *Handler) handleChannels(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, h.mgr.ListChannels())
	case http.MethodPost:
		var ch Channel
		if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		h.mgr.AddChannel(&ch)
		writeJSON(w, http.StatusCreated, ch)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	var tmpl Template
	if err := json.NewDecoder(r.Body).Decode(&tmpl); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	h.mgr.AddTemplate(&tmpl)
	writeJSON(w, http.StatusCreated, tmpl)
}

func (h *Handler) handlePreferences(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Path[len("/api/notify/preferences/"):]
	switch r.Method {
	case http.MethodGet:
		pref, ok := h.mgr.GetPreference(userID)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusOK, pref)
	case http.MethodPut:
		var pref Preference
		if err := json.NewDecoder(r.Body).Decode(&pref); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		pref.UserID = userID
		h.mgr.SetPreference(&pref)
		writeJSON(w, http.StatusOK, pref)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, h.mgr.GetStats())
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
