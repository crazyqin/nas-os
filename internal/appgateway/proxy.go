package appgateway

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

// createHTTPProxy 创建 HTTP 反向代理
func (m *Manager) createHTTPProxy(instance *AppInstance, route *RouteRule) *httputil.ReverseProxy {
	scheme := "http"
	targetURL := &url.URL{
		Scheme: scheme,
		Host:   fmt.Sprintf("%s:%d", instance.Host, instance.Port),
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// 自定义 Director
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = targetURL.Host

		// 路径前缀处理
		if route != nil && route.StripPrefix && route.Path != "" {
			req.URL.Path = stripPathPrefix(req.URL.Path, route.Path)
		}

		// 添加代理头部
		req.Header.Set("X-Forwarded-Host", req.Host)
		req.Header.Set("X-Forwarded-Proto", scheme)
		req.Header.Set("X-Real-IP", req.RemoteAddr)

		// 添加自定义头部
		if route != nil {
			for k, v := range route.Headers {
				req.Header.Set(k, v)
			}
		}
	}

	// 配置超时
	proxy.Transport = &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}

	// 错误处理
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(fmt.Sprintf(`{"error":"proxy error: %s"}`, err.Error())))
	}

	return proxy
}

// createWebSocketProxy 创建 WebSocket 代理
func (m *Manager) createWebSocketProxy(w http.ResponseWriter, req *http.Request, instance *AppInstance) error {
	scheme := "ws"
	if req.TLS != nil {
		scheme = "wss"
	}

	targetURL := &url.URL{
		Scheme:   scheme,
		Host:     fmt.Sprintf("%s:%d", instance.Host, instance.Port),
		Path:     req.URL.Path,
		RawQuery: req.URL.RawQuery,
	}

	// 使用标准反向代理处理 WebSocket
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// 自定义 Director 以支持 WebSocket
	originalDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		originalDirector(r)
		r.Host = targetURL.Host

		// 保留 WebSocket 相关头部
		r.Header.Set("Connection", req.Header.Get("Connection"))
		r.Header.Set("Upgrade", req.Header.Get("Upgrade"))
		r.Header.Set("Sec-WebSocket-Key", req.Header.Get("Sec-WebSocket-Key"))
		r.Header.Set("Sec-WebSocket-Version", req.Header.Get("Sec-WebSocket-Version"))
		r.Header.Set("Sec-WebSocket-Extensions", req.Header.Get("Sec-WebSocket-Extensions"))
		r.Header.Set("Sec-WebSocket-Protocol", req.Header.Get("Sec-WebSocket-Protocol"))
	}

	proxy.ServeHTTP(w, req)

	return nil
}

// stripPathPrefix 剥离路径前缀
func stripPathPrefix(path, prefix string) string {
	if prefix == "" {
		return path
	}

	// 确保前缀以 / 结尾
	if prefix[len(prefix)-1] != '/' {
		prefix = prefix + "/"
	}

	if len(path) >= len(prefix) && path[:len(prefix)] == prefix {
		return "/" + path[len(prefix):]
	}

	return path
}

// ProxyHandler 代理处理器（用于外部集成）
type ProxyHandler struct {
	manager *Manager
}

// NewProxyHandler 创建代理处理器
func NewProxyHandler(manager *Manager) *ProxyHandler {
	return &ProxyHandler{manager: manager}
}

// Handler 返回 HTTP 处理函数
func (h *ProxyHandler) Handler() http.Handler {
	return NewRouter(h.manager)
}
