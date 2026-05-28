// Package loadbalancer - 反向代理引擎实现
package loadbalancer

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

// Proxy 反向代理引擎
type Proxy struct {
	config  ProxyConfig
	balancer *Balancer
	transport *http.Transport
}

// NewProxy 创建反向代理
func NewProxy(config ProxyConfig, balancer *Balancer) *Proxy {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   config.DialTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     config.MaxConnsPerHost,
		IdleConnTimeout:     config.IdleTimeout,
		TLSHandshakeTimeout: 10 * time.Second,
		ResponseHeaderTimeout: config.ResponseTimeout,
	}

	return &Proxy{
		config:   config,
		balancer: balancer,
		transport: transport,
	}
}

// ServeHTTP 处理HTTP请求
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 选择后端
	backend, err := p.balancer.Select(r)
	if err != nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	// 创建反向代理
	proxy := p.createReverseProxy(backend)

	// 更新统计
	backend.IncrConns()
	backend.IncrReqs()
	defer backend.DecrConns()

	// 代理请求
	proxy.ServeHTTP(w, r)
}

// createReverseProxy 创建单个后端的反向代理
func (p *Proxy) createReverseProxy(backend *Backend) *httputil.ReverseProxy {
	target, err := url.Parse(backend.URL)
	if err != nil {
		// 返回一个简单的错误处理器
		return &httputil.ReverseProxy{
			Director: func(req *http.Request) {},
			Transport: &errorTransport{err: err},
		}
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			p.director(req, target, backend)
		},
		Transport:     p.transport,
		FlushInterval: p.config.FlushInterval,
		ErrorHandler:  p.errorHandler(backend),
	}

	return proxy
}

// director 请求修改器
func (p *Proxy) director(req *http.Request, target *url.URL, backend *Backend) {
	// 设置目标URL
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host

	// 处理路径
	targetPath := target.Path
	reqPath := req.URL.Path

	if targetPath == "" || targetPath == "/" {
		// 保持原始路径
	} else {
		// 拼接路径
		req.URL.Path = strings.TrimSuffix(targetPath, "/") + "/" + strings.TrimPrefix(reqPath, "/")
	}

	// 设置Host头
	if p.config.PassHostHeader {
		req.Host = target.Host
	} else {
		req.Host = ""
	}

	// 添加代理头部
	if p.config.XForwardedFor {
		if prior := req.Header.Get("X-Forwarded-For"); prior != "" {
			req.Header.Set("X-Forwarded-For", prior+", "+req.RemoteAddr)
		} else {
			req.Header.Set("X-Forwarded-For", req.RemoteAddr)
		}
	}

	if p.config.XRealIP {
		ip := getClientIP(req)
		req.Header.Set("X-Real-IP", ip)
	}

	// 添加标准代理头
	req.Header.Set("X-Forwarded-Proto", req.URL.Scheme)
	req.Header.Set("X-Forwarded-Host", req.Host)
}

// errorHandler 错误处理器
func (p *Proxy) errorHandler(backend *Backend) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		backend.IncrErrors()

		// 记录错误
		fmt.Printf("Proxy error for backend %s: %v\n", backend.ID, err)

		// 返回502错误
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}
}

// Close 关闭代理
func (p *Proxy) Close() {
	p.transport.CloseIdleConnections()
}

// errorTransport 错误传输层
type errorTransport struct {
	err error
}

func (t *errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, t.err
}

// ============================================================
// WebSocket代理支持
// ============================================================

// WebSocketProxy WebSocket代理
type WebSocketProxy struct {
	config   ProxyConfig
	balancer *Balancer
}

// NewWebSocketProxy 创建WebSocket代理
func NewWebSocketProxy(config ProxyConfig, balancer *Balancer) *WebSocketProxy {
	return &WebSocketProxy{
		config:   config,
		balancer: balancer,
	}
}

// ServeHTTP 处理WebSocket升级请求
func (wp *WebSocketProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 检查是否为WebSocket请求
	if !isWebSocketRequest(r) {
		http.Error(w, "Not a WebSocket request", http.StatusBadRequest)
		return
	}

	// 选择后端
	backend, err := wp.balancer.Select(r)
	if err != nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	// 更新统计
	backend.IncrConns()
	backend.IncrReqs()
	defer backend.DecrConns()

	// 解析后端URL
	target, err := url.Parse(backend.URL)
	if err != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	// 创建到后端的连接
	targetURL := fmt.Sprintf("%s%s", strings.TrimSuffix(backend.URL, "/"), r.URL.Path)
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	// 使用标准的WebSocket代理
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ServeHTTP(w, r)
}

// isWebSocketRequest 检查是否为WebSocket请求
func isWebSocketRequest(r *http.Request) bool {
	return strings.EqualFold(r.Header.Get("Connection"), "upgrade") &&
		strings.EqualFold(r.Header.Get("Upgrade"), "websocket")
}

// ============================================================
// 流式响应支持
// ============================================================

// StreamProxy 流式响应代理
type StreamProxy struct {
	config   ProxyConfig
	balancer *Balancer
}

// NewStreamProxy 创建流式代理
func NewStreamProxy(config ProxyConfig, balancer *Balancer) *StreamProxy {
	return &StreamProxy{
		config:   config,
		balancer: balancer,
	}
}

// ServeHTTP 处理流式请求
func (sp *StreamProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 选择后端
	backend, err := sp.balancer.Select(r)
	if err != nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return
	}

	// 更新统计
	backend.IncrConns()
	backend.IncrReqs()
	defer backend.DecrConns()

	// 解析后端URL
	target, err := url.Parse(backend.URL)
	if err != nil {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	// 创建反向代理，配置流式支持
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
		},
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   sp.config.DialTimeout,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ResponseHeaderTimeout: sp.config.ResponseTimeout,
		},
		FlushInterval: sp.config.FlushInterval,
		ModifyResponse: func(resp *http.Response) error {
			// 确保支持流式传输
			if resp.Header.Get("Content-Type") == "text/event-stream" ||
				resp.Header.Get("Content-Type") == "application/octet-stream" {
				resp.Header.Del("Content-Length")
				resp.Header.Set("Transfer-Encoding", "chunked")
			}
			return nil
		},
	}

	proxy.ServeHTTP(w, r)
}

// ============================================================
// 工具函数
// ============================================================

// copyHeaders 复制请求头
func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

// removeHopHeaders 移除逐跳头
func removeHopHeaders(header http.Header) {
	hopHeaders := []string{
		"Connection",
		"Keep-Alive",
		"Proxy-Authenticate",
		"Proxy-Authorization",
		"Te",
		"Trailers",
		"Transfer-Encoding",
		"Upgrade",
	}

	for _, h := range hopHeaders {
		header.Del(h)
	}
}

// isUpgradeRequest 检查是否为升级请求
func isUpgradeRequest(r *http.Request) bool {
	for _, v := range r.Header.Values("Connection") {
		if strings.EqualFold(strings.TrimSpace(v), "upgrade") {
			return true
		}
	}
	return false
}
