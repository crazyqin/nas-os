package appgateway

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Router 应用网关路由器
type Router struct {
	manager *Manager
}

// NewRouter 创建路由器
func NewRouter(manager *Manager) *Router {
	return &Router{manager: manager}
}

// ServeHTTP 实现 http.Handler 接口
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	start := time.Now()
	clientIP := getClientIP(req)

	// 匹配路由
	route, app := r.manager.MatchRoute(req.Host, req.URL.Path)
	if route == nil || app == nil {
		http.Error(w, `{"error":"no matching route"}`, http.StatusNotFound)
		return
	}

	// 访问控制检查
	if err := r.manager.CheckAccess(app, clientIP); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusForbidden)
		r.logAccess(req, app, http.StatusForbidden, clientIP, start, err.Error(), false)
		return
	}

	// API Key 检查
	if app.Access != nil && app.Access.APIKey != "" {
		apiKey := req.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = req.URL.Query().Get("api_key")
		}
		if err := r.manager.CheckAPIKey(app, apiKey); err != nil {
			http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
			r.logAccess(req, app, http.StatusUnauthorized, clientIP, start, err.Error(), false)
			return
		}
	}

	// Basic Auth 检查
	if app.Access != nil && app.Access.RequireAuth && app.Access.BasicAuth != nil {
		if !r.checkBasicAuth(req, app) {
			w.Header().Set("WWW-Authenticate", `Basic realm="App Gateway"`)
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			r.logAccess(req, app, http.StatusUnauthorized, clientIP, start, "auth required", false)
			return
		}
	}

	// WebSocket 检测
	if isWebSocketRequest(req) && r.manager.config.WebSocketEnabled {
		r.handleWebSocket(w, req, app, clientIP, start)
		return
	}

	// HTTP 代理
	r.handleHTTP(w, req, route, app, clientIP, start)
}

// handleHTTP 处理 HTTP 请求
func (r *Router) handleHTTP(w http.ResponseWriter, req *http.Request, route *RouteRule, app *Application, clientIP string, start time.Time) {
	// 选择实例
	instance, err := r.manager.SelectInstance(app, clientIP)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
		r.logAccess(req, app, http.StatusBadGateway, clientIP, start, err.Error(), false)
		return
	}

	// 创建反向代理
	proxy := r.manager.createHTTPProxy(instance, route)
	if proxy == nil {
		http.Error(w, `{"error":"failed to create proxy"}`, http.StatusInternalServerError)
		r.logAccess(req, app, http.StatusInternalServerError, clientIP, start, "proxy creation failed", false)
		return
	}

	// 记录响应
	rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

	// 代理请求
	proxy.ServeHTTP(rec, req)

	// 记录日志
	r.logAccess(req, app, rec.statusCode, clientIP, start, "", false)
}

// handleWebSocket 处理 WebSocket 请求
func (r *Router) handleWebSocket(w http.ResponseWriter, req *http.Request, app *Application, clientIP string, start time.Time) {
	// 选择实例
	instance, err := r.manager.SelectInstance(app, clientIP)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
		r.logAccess(req, app, http.StatusBadGateway, clientIP, start, err.Error(), true)
		return
	}

	// 创建 WebSocket 代理
	err = r.manager.createWebSocketProxy(w, req, instance)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadGateway)
		r.logAccess(req, app, http.StatusBadGateway, clientIP, start, err.Error(), true)
		return
	}

	r.logAccess(req, app, http.StatusSwitchingProtocols, clientIP, start, "", true)
}

// checkBasicAuth 检查 Basic 认证
func (r *Router) checkBasicAuth(req *http.Request, app *Application) bool {
	auth := req.Header.Get("Authorization")
	if auth == "" {
		return false
	}

	if !strings.HasPrefix(auth, "Basic ") {
		return false
	}

	decoded, err := base64.StdEncoding.DecodeString(auth[6:])
	if err != nil {
		return false
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return false
	}

	return r.manager.CheckBasicAuth(app, parts[0], parts[1]) == nil
}

// logAccess 记录访问日志
func (r *Router) logAccess(req *http.Request, app *Application, statusCode int, clientIP string, start time.Time, errMsg string, isWS bool) {
	if !r.manager.config.LogEnabled {
		return
	}

	r.manager.LogRequest(&AccessLog{
		AppID:        app.ID,
		AppName:      app.Name,
		Method:       req.Method,
		Path:         req.URL.Path,
		StatusCode:   statusCode,
		ClientIP:     clientIP,
		UserAgent:    req.UserAgent(),
		RequestSize:  req.ContentLength,
		Duration:     time.Since(start),
		IsWebSocket:  isWS,
		Error:        errMsg,
	})
}

// isWebSocketRequest 判断是否是 WebSocket 请求
func isWebSocketRequest(req *http.Request) bool {
	return strings.EqualFold(req.Header.Get("Connection"), "upgrade") &&
		strings.EqualFold(req.Header.Get("Upgrade"), "websocket")
}

// getClientIP 获取客户端IP
func getClientIP(req *http.Request) string {
	// 优先检查 X-Forwarded-For
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// X-Real-IP
	if xri := req.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// RemoteAddr
	ip, _, err := net.SplitHostPort(req.RemoteAddr)
	if err != nil {
		return req.RemoteAddr
	}
	return ip
}

// responseRecorder 响应记录器
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader 记录状态码
func (r *responseRecorder) WriteHeader(code int) {
	if !r.written {
		r.statusCode = code
		r.written = true
	}
	r.ResponseWriter.WriteHeader(code)
}

// Write 写入响应
func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.statusCode = http.StatusOK
		r.written = true
	}
	return r.ResponseWriter.Write(b)
}

// RegisterRoutes 注册路由到 http.ServeMux
func (r *Router) RegisterRoutes(mux *http.ServeMux) {
	// 管理API
	mux.HandleFunc("/api/appgateway/apps", r.handleApps)
	mux.HandleFunc("/api/appgateway/routes", r.handleRoutes)
	mux.HandleFunc("/api/appgateway/stats", r.handleStats)
	mux.HandleFunc("/api/appgateway/logs", r.handleLogs)
	mux.HandleFunc("/api/appgateway/health/", r.handleHealthCheck)
}

// handleApps 处理应用管理请求
func (r *Router) handleApps(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		apps := r.manager.ListApps()
		respondJSON(w, apps)
	case http.MethodPost:
		var app Application
		if err := decodeJSON(req, &app); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if err := r.manager.RegisterApp(&app); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		respondJSON(w, app)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleRoutes 处理路由管理请求
func (r *Router) handleRoutes(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		routes := r.manager.ListRoutes()
		respondJSON(w, routes)
	case http.MethodPost:
		var route RouteRule
		if err := decodeJSON(req, &route); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if err := r.manager.AddRoute(&route); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
			return
		}
		respondJSON(w, route)
	default:
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
	}
}

// handleStats 处理统计请求
func (r *Router) handleStats(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	stats := r.manager.GetStats()
	respondJSON(w, stats)
}

// handleLogs 处理日志请求
func (r *Router) handleLogs(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	logs := r.manager.GetRequestLogs(100)
	respondJSON(w, logs)
}

// handleHealthCheck 处理健康检查请求
func (r *Router) handleHealthCheck(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	appID := strings.TrimPrefix(req.URL.Path, "/api/appgateway/health/")
	if appID == "" {
		http.Error(w, `{"error":"app_id required"}`, http.StatusBadRequest)
		return
	}

	result, err := r.manager.CheckHealth(appID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusNotFound)
		return
	}

	respondJSON(w, result)
}

// respondJSON 响应JSON
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// decodeJSON 解码JSON
func decodeJSON(req *http.Request, v interface{}) error {
	return json.NewDecoder(req.Body).Decode(v)
}
