// Package loadbalancer - 中间件链实现
package loadbalancer

import (
	"compress/gzip"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 中间件链
// ============================================================

// Middleware HTTP中间件函数类型.
type Middleware func(http.Handler) http.Handler

// Chain 中间件链.
type Chain struct {
	middlewares []Middleware
}

// NewChain 创建中间件链.
func NewChain(middlewares ...Middleware) *Chain {
	return &Chain{
		middlewares: middlewares,
	}
}

// Append 追加中间件.
func (c *Chain) Append(middlewares ...Middleware) *Chain {
	newChain := &Chain{
		middlewares: make([]Middleware, len(c.middlewares)+len(middlewares)),
	}
	copy(newChain.middlewares, c.middlewares)
	copy(newChain.middlewares[len(c.middlewares):], middlewares)
	return newChain
}

// Then 包装处理器.
func (c *Chain) Then(handler http.Handler) http.Handler {
	for i := len(c.middlewares) - 1; i >= 0; i-- {
		handler = c.middlewares[i](handler)
	}
	return handler
}

// ThenFunc 包装处理函数.
func (c *Chain) ThenFunc(fn http.HandlerFunc) http.Handler {
	return c.Then(fn)
}

// ============================================================
// 日志中间件
// ============================================================

// LoggingMiddleware 日志中间件.
type LoggingMiddleware struct {
	config LoggingConfig
}

// NewLoggingMiddleware 创建日志中间件.
func NewLoggingMiddleware(config LoggingConfig) *LoggingMiddleware {
	return &LoggingMiddleware{config: config}
}

// Handler HTTP中间件.
func (lm *LoggingMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !lm.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()

		// 包装ResponseWriter以捕获状态码
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// 执行下一个处理器
		next.ServeHTTP(wrapped, r)

		// 记录访问日志
		if lm.config.AccessLog {
			duration := time.Since(start)
			lm.logAccess(r, wrapped.statusCode, duration)
		}
	})
}

// logAccess 记录访问日志.
func (lm *LoggingMiddleware) logAccess(r *http.Request, statusCode int, duration time.Duration) {
	clientIP := getClientIP(r)
	method := r.Method
	path := r.URL.Path
	query := r.URL.RawQuery
	userAgent := r.UserAgent()

	if lm.config.Format == "json" {
		fmt.Printf(`{"time":"%s","ip":"%s","method":"%s","path":"%s","query":"%s","status":%d,"duration":"%s","user_agent":"%s"}`+"\n",
			time.Now().Format(time.RFC3339),
			clientIP,
			method,
			path,
			query,
			statusCode,
			duration.String(),
			userAgent,
		)
	} else {
		fmt.Printf("%s %s %s %s %s %d %s\n",
			time.Now().Format("2006-01-02 15:04:05"),
			clientIP,
			method,
			path,
			query,
			statusCode,
			duration.String(),
		)
	}
}

// responseWriter 包装ResponseWriter以捕获状态码.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader 写入状态码.
func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.statusCode = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

// Write 写入响应.
func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

// ============================================================
// CORS中间件
// ============================================================

// CORSMiddleware CORS中间件.
type CORSMiddleware struct {
	config CORSConfig
}

// NewCORSMiddleware 创建CORS中间件.
func NewCORSMiddleware(config CORSConfig) *CORSMiddleware {
	return &CORSMiddleware{config: config}
}

// Handler HTTP中间件.
func (cm *CORSMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !cm.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}

		// 检查是否允许该源
		if !cm.isAllowedOrigin(origin) {
			next.ServeHTTP(w, r)
			return
		}

		// 设置CORS头
		w.Header().Set("Access-Control-Allow-Origin", origin)
		if cm.config.AllowCredentials {
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// 处理预检请求
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", strings.Join(cm.config.AllowedMethods, ", "))
			w.Header().Set("Access-Control-Allow-Headers", strings.Join(cm.config.AllowedHeaders, ", "))
			if len(cm.config.ExposedHeaders) > 0 {
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(cm.config.ExposedHeaders, ", "))
			}
			if cm.config.MaxAge > 0 {
				w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%d", cm.config.MaxAge))
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// isAllowedOrigin 检查是否允许该源.
func (cm *CORSMiddleware) isAllowedOrigin(origin string) bool {
	for _, allowed := range cm.config.AllowedOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
		// 支持通配符 *.example.com
		if strings.HasPrefix(allowed, "*.") {
			suffix := allowed[1:]
			if strings.HasSuffix(origin, suffix) {
				return true
			}
		}
	}
	return false
}

// ============================================================
// 压缩中间件
// ============================================================

// CompressionMiddleware 压缩中间件.
type CompressionMiddleware struct {
	config CompressionConfig
	pool   sync.Pool
}

// NewCompressionMiddleware 创建压缩中间件.
func NewCompressionMiddleware(config CompressionConfig) *CompressionMiddleware {
	return &CompressionMiddleware{
		config: config,
		pool: sync.Pool{
			New: func() interface{} {
				w, _ := gzip.NewWriterLevel(nil, config.Level)
				return w
			},
		},
	}
}

// Handler HTTP中间件.
func (cwm *CompressionMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !cwm.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// 检查客户端是否支持gzip
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		// 包装ResponseWriter
		wrapped := &gzipResponseWriter{
			ResponseWriter: w,
			config:         cwm.config,
			pool:           &cwm.pool,
		}
		defer wrapped.Close()

		// 设置Content-Encoding头
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Del("Content-Length")

		next.ServeHTTP(wrapped, r)
	})
}

// gzipResponseWriter gzip响应写入器.
type gzipResponseWriter struct {
	http.ResponseWriter
	config    CompressionConfig
	pool      *sync.Pool
	writer    *gzip.Writer
	written   bool
	sniffDone bool
}

// Write 写入响应.
func (gw *gzipResponseWriter) Write(b []byte) (int, error) {
	if !gw.sniffDone {
		gw.sniffDone = true
		contentType := http.DetectContentType(b)
		if !gw.shouldCompress(contentType) {
			// 不压缩，直接写入
			gw.ResponseWriter.Header().Del("Content-Encoding")
			return gw.ResponseWriter.Write(b)
		}
	}

	if !gw.written {
		gw.written = true
		gw.writer = gw.pool.Get().(*gzip.Writer)
		gw.writer.Reset(gw.ResponseWriter)
	}

	return gw.writer.Write(b)
}

// Close 关闭gzip写入器.
func (gw *gzipResponseWriter) Close() {
	if gw.writer != nil {
		gw.writer.Close()
		gw.pool.Put(gw.writer)
	}
}

// shouldCompress 是否应该压缩.
func (gw *gzipResponseWriter) shouldCompress(contentType string) bool {
	// 检查最小大小
	// 注意：这里无法获取响应大小，所以跳过大小检查
	// 实际应用中可以通过缓冲响应来实现

	// 检查MIME类型
	for _, allowedType := range gw.config.Types {
		if strings.HasPrefix(contentType, allowedType) {
			return true
		}
	}
	return false
}

// ============================================================
// 缓存中间件
// ============================================================

// CacheMiddleware 缓存中间件.
type CacheMiddleware struct {
	config CacheConfig
	cache  map[string]*cacheEntry
	mu     sync.RWMutex
}

// cacheEntry 缓存条目.
type cacheEntry struct {
	statusCode int
	headers    http.Header
	body       []byte
	createdAt  time.Time
	expiresAt  time.Time
}

// NewCacheMiddleware 创建缓存中间件.
func NewCacheMiddleware(config CacheConfig) *CacheMiddleware {
	cm := &CacheMiddleware{
		config: config,
		cache:  make(map[string]*cacheEntry),
	}

	// 启动清理goroutine
	go cm.cleanup()

	return cm
}

// Handler HTTP中间件.
func (cm *CacheMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !cm.config.Enabled {
			next.ServeHTTP(w, r)
			return
		}

		// 只缓存指定方法
		if !cm.isCacheableMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		// 生成缓存key
		key := cm.generateKey(r)

		// 检查缓存
		cm.mu.RLock()
		entry, exists := cm.cache[key]
		cm.mu.RUnlock()

		if exists && time.Now().Before(entry.expiresAt) {
			// 命中缓存
			cm.serveFromCache(w, entry)
			return
		}

		// 包装ResponseWriter以捕获响应
		wrapped := &cacheResponseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           &strings.Builder{},
		}

		// 执行处理器
		next.ServeHTTP(wrapped, r)

		// 检查是否应该缓存
		if cm.isCacheableStatus(wrapped.statusCode) {
			entry = &cacheEntry{
				statusCode: wrapped.statusCode,
				headers:    wrapped.Header().Clone(),
				body:       []byte(wrapped.body.String()),
				createdAt:  time.Now(),
				expiresAt:  time.Now().Add(cm.config.TTL),
			}

			cm.mu.Lock()
			// 检查缓存大小限制
			if len(cm.cache) >= cm.config.MaxSize {
				cm.evictOldest()
			}
			cm.cache[key] = entry
			cm.mu.Unlock()
		}
	})
}

// generateKey 生成缓存key.
func (cm *CacheMiddleware) generateKey(r *http.Request) string {
	return fmt.Sprintf("%s:%s:%s", r.Method, r.URL.Path, r.URL.RawQuery)
}

// isCacheableMethod 检查是否可缓存的方法.
func (cm *CacheMiddleware) isCacheableMethod(method string) bool {
	for _, m := range cm.config.Methods {
		if m == method {
			return true
		}
	}
	return false
}

// isCacheableStatus 检查是否可缓存的状态码.
func (cm *CacheMiddleware) isCacheableStatus(status int) bool {
	for _, s := range cm.config.StatusCodes {
		if s == status {
			return true
		}
	}
	return false
}

// serveFromCache 从缓存提供响应.
func (cm *CacheMiddleware) serveFromCache(w http.ResponseWriter, entry *cacheEntry) {
	for key, values := range entry.headers {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.Header().Set("X-Cache", "HIT")
	w.WriteHeader(entry.statusCode)
	w.Write(entry.body)
}

// evictOldest 淘汰最旧的缓存.
func (cm *CacheMiddleware) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range cm.cache {
		if oldestKey == "" || entry.createdAt.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.createdAt
		}
	}

	if oldestKey != "" {
		delete(cm.cache, oldestKey)
	}
}

// cleanup 清理过期缓存.
func (cm *CacheMiddleware) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cm.mu.Lock()
		for key, entry := range cm.cache {
			if time.Now().After(entry.expiresAt) {
				delete(cm.cache, key)
			}
		}
		cm.mu.Unlock()
	}
}

// cacheResponseWriter 缓存响应写入器.
type cacheResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       *strings.Builder
}

// WriteHeader 写入状态码.
func (crw *cacheResponseWriter) WriteHeader(code int) {
	crw.statusCode = code
	crw.ResponseWriter.WriteHeader(code)
}

// Write 写入响应.
func (crw *cacheResponseWriter) Write(b []byte) (int, error) {
	crw.body.Write(b)
	return crw.ResponseWriter.Write(b)
}

// ============================================================
// 中间件工厂
// ============================================================

// NewMiddlewareChain 创建默认中间件链.
func NewMiddlewareChain(config MiddlewareConfig) *Chain {
	var middlewares []Middleware

	// 日志中间件 (最先执行)
	if config.Logging.Enabled {
		logging := NewLoggingMiddleware(config.Logging)
		middlewares = append(middlewares, logging.Handler)
	}

	// CORS中间件
	if config.CORS.Enabled {
		cors := NewCORSMiddleware(config.CORS)
		middlewares = append(middlewares, cors.Handler)
	}

	// 压缩中间件
	if config.Compression.Enabled {
		compression := NewCompressionMiddleware(config.Compression)
		middlewares = append(middlewares, compression.Handler)
	}

	// 缓存中间件
	if config.Cache.Enabled {
		cache := NewCacheMiddleware(config.Cache)
		middlewares = append(middlewares, cache.Handler)
	}

	return NewChain(middlewares...)
}

// ============================================================
// 工具函数
// ============================================================

// WrapHandler 用中间件包装处理器.
func WrapHandler(handler http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
}

// WrapHandlerFunc 用中间件包装处理函数.
func WrapHandlerFunc(fn http.HandlerFunc, middlewares ...Middleware) http.Handler {
	return WrapHandler(fn, middlewares...)
}
