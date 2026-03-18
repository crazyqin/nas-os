// Package middleware provides HTTP middleware for the API
package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

// CacheBackend 定义缓存后端接口
type CacheBackend interface {
	Get(ctx context.Context, key string) ([]byte, bool)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration)
	Delete(ctx context.Context, key string)
	DeleteByPrefix(ctx context.Context, prefix string)
	Clear(ctx context.Context)
}

// MemoryCache 内存缓存实现
type MemoryCache struct {
	mu      sync.RWMutex
	entries map[string]*cacheEntry
	maxSize int
	ttl     time.Duration
}

type cacheEntry struct {
	data      []byte
	expiresAt time.Time
}

// NewMemoryCache 创建内存缓存
func NewMemoryCache(maxSize int, ttl time.Duration) *MemoryCache {
	mc := &MemoryCache{
		entries: make(map[string]*cacheEntry),
		maxSize: maxSize,
		ttl:     ttl,
	}
	// 启动后台清理
	go mc.cleanup()
	return mc
}

func (m *MemoryCache) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for k, v := range m.entries {
			if now.After(v.expiresAt) {
				delete(m.entries, k)
			}
		}
		m.mu.Unlock()
	}
}

// Get retrieves a value from the cache by key.
func (m *MemoryCache) Get(_ context.Context, key string) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	entry, ok := m.entries[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		return nil, false
	}
	return entry.data, true
}

// Set stores a value in the cache with the specified TTL.
func (m *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果超过最大大小，删除最旧的条目
	if len(m.entries) >= m.maxSize {
		// 简单策略：清空一半
		count := 0
		for k := range m.entries {
			delete(m.entries, k)
			count++
			if count >= m.maxSize/2 {
				break
			}
		}
	}

	m.entries[key] = &cacheEntry{
		data:      value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a key from the cache.
func (m *MemoryCache) Delete(_ context.Context, key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
}

// DeleteByPrefix removes all keys with the specified prefix from the cache.
func (m *MemoryCache) DeleteByPrefix(_ context.Context, prefix string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k := range m.entries {
		if strings.HasPrefix(k, prefix) {
			delete(m.entries, k)
		}
	}
}

// Clear removes all entries from the cache.
func (m *MemoryCache) Clear(_ context.Context) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]*cacheEntry)
}

// RedisCacheBackend Redis 缓存实现
type RedisCacheBackend struct {
	client *redis.Client
	prefix string
}

// NewRedisCacheBackend 创建 Redis 缓存后端
func NewRedisCacheBackend(addr, password string, db int) (*RedisCacheBackend, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &RedisCacheBackend{
		client: client,
		prefix: "nas-os:cache:",
	}, nil
}

// Get retrieves a value from Redis by key.
func (r *RedisCacheBackend) Get(ctx context.Context, key string) ([]byte, bool) {
	val, err := r.client.Get(ctx, r.prefix+key).Bytes()
	if err != nil {
		return nil, false
	}
	return val, true
}

// Set stores a value in Redis with the specified TTL.
func (r *RedisCacheBackend) Set(ctx context.Context, key string, value []byte, ttl time.Duration) {
	_ = r.client.Set(ctx, r.prefix+key, value, ttl)
}

// Delete removes a key from Redis.
func (r *RedisCacheBackend) Delete(ctx context.Context, key string) {
	_ = r.client.Del(ctx, r.prefix+key)
}

// DeleteByPrefix removes all keys with the specified prefix from Redis.
func (r *RedisCacheBackend) DeleteByPrefix(ctx context.Context, prefix string) {
	keys, _ := r.client.Keys(ctx, r.prefix+prefix+"*").Result()
	if len(keys) > 0 {
		_ = r.client.Del(ctx, keys...)
	}
}

// Clear removes all cache entries from Redis.
func (r *RedisCacheBackend) Clear(ctx context.Context) {
	keys, _ := r.client.Keys(ctx, r.prefix+"*").Result()
	if len(keys) > 0 {
		_ = r.client.Del(ctx, keys...)
	}
}

// Close 关闭 Redis 连接
func (r *RedisCacheBackend) Close() error {
	return r.client.Close()
}

// CacheConfig 缓存中间件配置
type CacheConfig struct {
	// 缓存后端（内存或 Redis）
	Backend CacheBackend

	// 默认 TTL
	DefaultTTL time.Duration

	// 是否启用 ETag 支持
	EnableETag bool

	// 是否只缓存 GET 请求
	GETOnly bool

	// 跳过的路径
	SkipPaths []string

	// 跳过的路径前缀
	SkipPathPrefixes []string

	// 需要缓存的路径模式（支持通配符）
	IncludePaths []string

	// 自定义缓存键生成函数
	KeyGenerator func(c *gin.Context) string

	// 自定义 TTL 生成函数
	TTLGenerator func(c *gin.Context) time.Duration

	// 是否缓存认证请求
	CacheAuthenticated bool

	// 响应状态码缓存策略
	StatusCodes []int

	// 是否记录日志
	Logger *zap.Logger

	// 是否启用
	Enabled bool

	// 缓存键前缀
	KeyPrefix string
}

// DefaultCacheConfig 默认缓存配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		Backend:            NewMemoryCache(1000, 5*time.Minute),
		DefaultTTL:         5 * time.Minute,
		EnableETag:         true,
		GETOnly:            true,
		SkipPaths:          []string{"/health", "/metrics", "/api/v1/auth"},
		SkipPathPrefixes:   []string{"/api/v1/websocket", "/api/v1/stream"},
		IncludePaths:       []string{"/api/"},
		CacheAuthenticated: false,
		StatusCodes:        []int{200, 201, 202, 203, 204, 301, 302, 304, 404},
		Enabled:            true,
		KeyPrefix:          "api_cache:",
	}
}

// CacheMiddleware 创建缓存中间件
func CacheMiddleware(config ...*CacheConfig) gin.HandlerFunc {
	cfg := DefaultCacheConfig()
	if len(config) > 0 && config[0] != nil {
		cfg = config[0]
	}

	// 构建跳过路径 map
	skipMap := make(map[string]bool)
	for _, path := range cfg.SkipPaths {
		skipMap[path] = true
	}

	return func(c *gin.Context) {
		// 检查是否启用
		if !cfg.Enabled {
			c.Next()
			return
		}

		// 检查是否跳过路径
		if skipMap[c.Request.URL.Path] {
			c.Next()
			return
		}

		// 检查路径前缀
		for _, prefix := range cfg.SkipPathPrefixes {
			if strings.HasPrefix(c.Request.URL.Path, prefix) {
				c.Next()
				return
			}
		}

		// 检查是否只缓存 GET 请求
		if cfg.GETOnly && c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		// 检查认证请求
		if !cfg.CacheAuthenticated {
			if _, exists := c.Get("user_id"); exists {
				c.Next()
				return
			}
			if c.GetHeader("Authorization") != "" {
				c.Next()
				return
			}
		}

		// 检查是否在包含路径中
		if len(cfg.IncludePaths) > 0 {
			included := false
			for _, pattern := range cfg.IncludePaths {
				if strings.HasPrefix(c.Request.URL.Path, pattern) {
					included = true
					break
				}
			}
			if !included {
				c.Next()
				return
			}
		}

		// 生成缓存键
		var cacheKey string
		if cfg.KeyGenerator != nil {
			cacheKey = cfg.KeyGenerator(c)
		} else {
			cacheKey = generateCacheKey(c, cfg.KeyPrefix)
		}

		// 尝试从缓存获取
		ctx := c.Request.Context()
		if data, ok := cfg.Backend.Get(ctx, cacheKey); ok {
			// 解析缓存数据
			var cached cachedResponse
			if err := json.Unmarshal(data, &cached); err == nil {
				// 检查 ETag
				if cfg.EnableETag {
					etag := generateETag(cached.Body)
					if match := c.GetHeader("If-None-Match"); match == etag {
						c.AbortWithStatus(http.StatusNotModified)
						return
					}
					c.Header("ETag", etag)
				}

				// 设置响应头
				for k, v := range cached.Headers {
					c.Header(k, v)
				}
				c.Header("X-Cache", "HIT")
				c.Header("X-Cache-TTL", time.Until(cached.ExpiresAt).Round(time.Second).String())

				// 返回缓存内容
				c.Data(cached.StatusCode, cached.ContentType, cached.Body)
				return
			}
		}

		// 缓存未命中，使用响应写入器捕获响应
		c.Header("X-Cache", "MISS")

		writer := &cacheResponseWriter{
			ResponseWriter: c.Writer,
			body:           &bytes.Buffer{},
		}
		c.Writer = writer

		// 处理请求
		c.Next()

		// 检查是否应该缓存此响应
		if !shouldCache(c.Writer.Status(), cfg.StatusCodes) {
			return
		}

		// 获取 TTL
		ttl := cfg.DefaultTTL
		if cfg.TTLGenerator != nil {
			ttl = cfg.TTLGenerator(c)
		}

		// 检查 Cache-Control 头
		if cacheControl := c.Writer.Header().Get("Cache-Control"); cacheControl != "" {
			if strings.Contains(cacheControl, "no-store") || strings.Contains(cacheControl, "private") {
				return
			}
			// 解析 max-age
			if strings.Contains(cacheControl, "max-age=") {
				parts := strings.Split(cacheControl, ",")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					if strings.HasPrefix(part, "max-age=") {
						ageStr := strings.TrimPrefix(part, "max-age=")
						if age, err := time.ParseDuration(ageStr + "s"); err == nil {
							ttl = age
						}
					}
				}
			}
		}

		// 准备缓存数据
		cached := cachedResponse{
			StatusCode:  c.Writer.Status(),
			Headers:     make(map[string]string),
			Body:        writer.body.Bytes(),
			ContentType: c.Writer.Header().Get("Content-Type"),
			ExpiresAt:   time.Now().Add(ttl),
			CreatedAt:   time.Now(),
		}

		// 复制响应头
		for k, v := range c.Writer.Header() {
			if len(v) > 0 && !isHopByHopHeader(k) {
				cached.Headers[k] = v[0]
			}
		}

		// 添加 ETag
		if cfg.EnableETag {
			etag := generateETag(cached.Body)
			cached.Headers["ETag"] = etag
			// 同时设置到实际响应头（第一次请求也需要返回 ETag）
			c.Header("ETag", etag)
		}

		// 序列化并存储
		if data, err := json.Marshal(cached); err == nil {
			cfg.Backend.Set(ctx, cacheKey, data, ttl)
		}

		if cfg.Logger != nil {
			cfg.Logger.Debug("cached response",
				zap.String("key", cacheKey),
				zap.Int("status", cached.StatusCode),
				zap.Duration("ttl", ttl),
			)
		}
	}
}

// cachedResponse 缓存的响应数据
type cachedResponse struct {
	StatusCode  int               `json:"statusCode"`
	Headers     map[string]string `json:"headers"`
	Body        []byte            `json:"body"`
	ContentType string            `json:"contentType"`
	ExpiresAt   time.Time         `json:"expiresAt"`
	CreatedAt   time.Time         `json:"createdAt"`
}

// cacheResponseWriter 用于捕获响应的写入器
type cacheResponseWriter struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (w *cacheResponseWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *cacheResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *cacheResponseWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// generateCacheKey 生成缓存键
func generateCacheKey(c *gin.Context, prefix string) string {
	var builder strings.Builder

	builder.WriteString(prefix)
	builder.WriteString(c.Request.Method)
	builder.WriteString(":")
	builder.WriteString(c.Request.URL.Path)

	// 包含查询参数
	if c.Request.URL.RawQuery != "" {
		builder.WriteString("?")
		builder.WriteString(c.Request.URL.RawQuery)
	}

	// 包含特定的请求头（如 Accept、Accept-Language）
	if accept := c.GetHeader("Accept"); accept != "" && accept != "*/*" {
		builder.WriteString("|accept:")
		builder.WriteString(accept)
	}
	if lang := c.GetHeader("Accept-Language"); lang != "" {
		builder.WriteString("|lang:")
		builder.WriteString(lang)
	}

	return builder.String()
}

// generateETag 生成 ETag
func generateETag(data []byte) string {
	hash := sha256.Sum256(data)
	return `"` + hex.EncodeToString(hash[:8]) + `"`
}

// shouldCache 判断是否应该缓存响应
func shouldCache(statusCode int, allowedCodes []int) bool {
	for _, code := range allowedCodes {
		if statusCode == code {
			return true
		}
	}
	return false
}

// isHopByHopHeader 判断是否为逐跳头
func isHopByHopHeader(header string) bool {
	h := strings.ToLower(header)
	hopByHop := []string{
		"connection",
		"keep-alive",
		"proxy-authenticate",
		"proxy-authorization",
		"te",
		"trailers",
		"transfer-encoding",
		"upgrade",
		"cache-control",
	}
	for _, v := range hopByHop {
		if h == v {
			return true
		}
	}
	return false
}

// ==================== 缓存控制 API ====================

// CacheManager 缓存管理器
type CacheManager struct {
	backends map[string]CacheBackend
	mu       sync.RWMutex
	logger   *zap.Logger
}

// NewCacheManager 创建缓存管理器
func NewCacheManager(logger *zap.Logger) *CacheManager {
	return &CacheManager{
		backends: make(map[string]CacheBackend),
		logger:   logger,
	}
}

// AddBackend 添加缓存后端
func (cm *CacheManager) AddBackend(name string, backend CacheBackend) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.backends[name] = backend
}

// GetBackend 获取缓存后端
func (cm *CacheManager) GetBackend(name string) CacheBackend {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.backends[name]
}

// RemoveBackend 移除缓存后端
func (cm *CacheManager) RemoveBackend(name string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.backends, name)
}

// Invalidate 使缓存失效
func (cm *CacheManager) Invalidate(ctx context.Context, pattern string) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for name, backend := range cm.backends {
		backend.DeleteByPrefix(ctx, pattern)
		if cm.logger != nil {
			cm.logger.Debug("cache invalidated",
				zap.String("backend", name),
				zap.String("pattern", pattern),
			)
		}
	}
}

// Clear 清空所有缓存
func (cm *CacheManager) Clear(ctx context.Context) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	for name, backend := range cm.backends {
		backend.Clear(ctx)
		if cm.logger != nil {
			cm.logger.Info("cache cleared", zap.String("backend", name))
		}
	}
}

// ==================== 缓存失效策略 ====================

// InvalidationStrategy 缓存失效策略
type InvalidationStrategy int

const (
	// InvalidationTTL TTL 失效
	InvalidationTTL InvalidationStrategy = iota
	// InvalidationManual 手动失效
	InvalidationManual
	// InvalidationEvent 事件驱动失效
	InvalidationEvent
	// InvalidationWrite 写时失效
	InvalidationWrite
)

// CachePolicy 缓存策略
type CachePolicy struct {
	Name         string               `json:"name"`
	PathPatterns []string             `json:"pathPatterns"`
	Methods      []string             `json:"methods"`
	TTL          time.Duration        `json:"ttl"`
	Strategy     InvalidationStrategy `json:"strategy"`
	EnableETag   bool                 `json:"enableETag"`
	VaryHeaders  []string             `json:"varyHeaders"`
	VaryParams   []string             `json:"varyParams"`
	MaxBodySize  int64                `json:"maxBodySize"` // 最大缓存体大小（字节）
}

// DefaultCachePolicies 默认缓存策略
var DefaultCachePolicies = []CachePolicy{
	{
		Name:         "static",
		PathPatterns: []string{"/static/", "/assets/", "/css/", "/js/", "/images/"},
		Methods:      []string{"GET"},
		TTL:          24 * time.Hour,
		Strategy:     InvalidationTTL,
		EnableETag:   true,
		MaxBodySize:  10 * 1024 * 1024, // 10MB
	},
	{
		Name:         "api_list",
		PathPatterns: []string{"/api/v1/users", "/api/v1/volumes", "/api/v1/shares"},
		Methods:      []string{"GET"},
		TTL:          1 * time.Minute,
		Strategy:     InvalidationWrite,
		EnableETag:   true,
		MaxBodySize:  1 * 1024 * 1024, // 1MB
	},
	{
		Name:         "api_detail",
		PathPatterns: []string{"/api/v1/users/", "/api/v1/volumes/", "/api/v1/shares/"},
		Methods:      []string{"GET"},
		TTL:          30 * time.Second,
		Strategy:     InvalidationWrite,
		EnableETag:   true,
		VaryParams:   []string{"fields", "expand"},
		MaxBodySize:  512 * 1024, // 512KB
	},
	{
		Name:         "health",
		PathPatterns: []string{"/health", "/metrics"},
		Methods:      []string{"GET"},
		TTL:          5 * time.Second,
		Strategy:     InvalidationTTL,
		EnableETag:   false,
		MaxBodySize:  1024, // 1KB
	},
}

// PolicyManager 策略管理器
type PolicyManager struct {
	policies map[string]CachePolicy
	mu       sync.RWMutex
}

// NewPolicyManager 创建策略管理器
func NewPolicyManager() *PolicyManager {
	pm := &PolicyManager{
		policies: make(map[string]CachePolicy),
	}

	// 加载默认策略
	for _, policy := range DefaultCachePolicies {
		pm.policies[policy.Name] = policy
	}

	return pm
}

// AddPolicy 添加策略
func (pm *PolicyManager) AddPolicy(policy CachePolicy) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.policies[policy.Name] = policy
}

// RemovePolicy 移除策略
func (pm *PolicyManager) RemovePolicy(name string) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	delete(pm.policies, name)
}

// GetPolicy 获取策略
func (pm *PolicyManager) GetPolicy(name string) (CachePolicy, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	policy, ok := pm.policies[name]
	return policy, ok
}

// MatchPolicy 匹配请求的策略
func (pm *PolicyManager) MatchPolicy(method, path string) (CachePolicy, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, policy := range pm.policies {
		// 检查方法
		methodMatch := false
		for _, m := range policy.Methods {
			if m == method || m == "*" {
				methodMatch = true
				break
			}
		}
		if !methodMatch {
			continue
		}

		// 检查路径
		for _, pattern := range policy.PathPatterns {
			if strings.HasPrefix(path, pattern) {
				return policy, true
			}
		}
	}

	return CachePolicy{}, false
}

// ListPolicies 列出所有策略
func (pm *PolicyManager) ListPolicies() []CachePolicy {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	policies := make([]CachePolicy, 0, len(pm.policies))
	for _, policy := range pm.policies {
		policies = append(policies, policy)
	}

	// 按名称排序
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Name < policies[j].Name
	})

	return policies
}

// ==================== 缓存失效事件 ====================

// CacheInvalidationEvent 缓存失效事件
type CacheInvalidationEvent struct {
	Type      string    `json:"type"`     // create, update, delete
	Resource  string    `json:"resource"` // users, volumes, shares
	ID        string    `json:"id"`       // 资源 ID
	Timestamp time.Time `json:"timestamp"`
}

// InvalidationBus 失效事件总线
type InvalidationBus struct {
	subscribers []chan CacheInvalidationEvent
	mu          sync.RWMutex
}

// NewInvalidationBus 创建失效事件总线
func NewInvalidationBus() *InvalidationBus {
	return &InvalidationBus{
		subscribers: make([]chan CacheInvalidationEvent, 0),
	}
}

// Subscribe 订阅失效事件
func (b *InvalidationBus) Subscribe() chan CacheInvalidationEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan CacheInvalidationEvent, 100)
	b.subscribers = append(b.subscribers, ch)
	return ch
}

// Unsubscribe 取消订阅
func (b *InvalidationBus) Unsubscribe(ch chan CacheInvalidationEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, sub := range b.subscribers {
		if sub == ch {
			b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
			close(ch)
			break
		}
	}
}

// Publish 发布失效事件
func (b *InvalidationBus) Publish(event CacheInvalidationEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscribers {
		select {
		case sub <- event:
		default:
			// 频道已满，跳过
		}
	}
}

// ==================== 缓存统计 ====================

// CacheStats 缓存统计
type CacheStats struct {
	Hits       int64   `json:"hits"`
	Misses     int64   `json:"misses"`
	Sets       int64   `json:"sets"`
	Deletes    int64   `json:"deletes"`
	Evictions  int64   `json:"evictions"`
	HitRatio   float64 `json:"hitRatio"`
	AvgLatency int64   `json:"avgLatencyNs"` // 纳秒
}

// StatsCollector 统计收集器
type StatsCollector struct {
	hits       int64
	misses     int64
	sets       int64
	deletes    int64
	evictions  int64
	latencies  []int64
	mu         sync.RWMutex
	maxSamples int
}

// NewStatsCollector 创建统计收集器
func NewStatsCollector(maxSamples int) *StatsCollector {
	if maxSamples <= 0 {
		maxSamples = 10000
	}
	return &StatsCollector{
		latencies:  make([]int64, 0, maxSamples),
		maxSamples: maxSamples,
	}
}

// RecordHit 记录命中
func (s *StatsCollector) RecordHit(latencyNs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits++
	s.recordLatency(latencyNs)
}

// RecordMiss 记录未命中
func (s *StatsCollector) RecordMiss(latencyNs int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.misses++
	s.recordLatency(latencyNs)
}

// RecordSet 记录设置
func (s *StatsCollector) RecordSet() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sets++
}

// RecordDelete 记录删除
func (s *StatsCollector) RecordDelete() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deletes++
}

// RecordEviction 记录驱逐
func (s *StatsCollector) RecordEviction() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictions++
}

func (s *StatsCollector) recordLatency(latencyNs int64) {
	if len(s.latencies) >= s.maxSamples {
		s.latencies = s.latencies[1:]
	}
	s.latencies = append(s.latencies, latencyNs)
}

// GetStats 获取统计
func (s *StatsCollector) GetStats() CacheStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	hits := s.hits
	misses := s.misses
	total := hits + misses

	var hitRatio float64
	if total > 0 {
		hitRatio = float64(hits) / float64(total)
	}

	var avgLatency int64
	if len(s.latencies) > 0 {
		var sum int64
		for _, l := range s.latencies {
			sum += l
		}
		avgLatency = sum / int64(len(s.latencies))
	}

	return CacheStats{
		Hits:       hits,
		Misses:     misses,
		Sets:       s.sets,
		Deletes:    s.deletes,
		Evictions:  s.evictions,
		HitRatio:   hitRatio,
		AvgLatency: avgLatency,
	}
}

// Reset 重置统计
func (s *StatsCollector) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits = 0
	s.misses = 0
	s.sets = 0
	s.deletes = 0
	s.evictions = 0
	s.latencies = s.latencies[:0]
}

// ==================== 缓存控制处理器 ====================

// CacheHandlers 缓存 API 处理器
type CacheHandlers struct {
	manager         *CacheManager
	policyManager   *PolicyManager
	statsCollector  *StatsCollector
	invalidationBus *InvalidationBus
}

// NewCacheHandlers 创建缓存处理器
func NewCacheHandlers(
	manager *CacheManager,
	policyManager *PolicyManager,
	statsCollector *StatsCollector,
	invalidationBus *InvalidationBus,
) *CacheHandlers {
	return &CacheHandlers{
		manager:         manager,
		policyManager:   policyManager,
		statsCollector:  statsCollector,
		invalidationBus: invalidationBus,
	}
}

// GetStats 获取缓存统计
// @Summary 获取缓存统计
// @Tags cache
// @Produce json
// @Success 200 {object} CacheStats
// @Router /api/v1/cache/stats [get]
func (h *CacheHandlers) GetStats(c *gin.Context) {
	stats := h.statsCollector.GetStats()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// ClearCache 清空缓存
// @Summary 清空缓存
// @Tags cache
// @Param backend query string false "后端名称（可选，不传则清空所有）"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cache/clear [post]
func (h *CacheHandlers) ClearCache(c *gin.Context) {
	backend := c.Query("backend")

	if backend != "" {
		b := h.manager.GetBackend(backend)
		if b == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"error":   "backend not found",
			})
			return
		}
		b.Clear(c.Request.Context())
	} else {
		h.manager.Clear(c.Request.Context())
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "cache cleared",
	})
}

// InvalidateCache 使缓存失效
// @Summary 使缓存失效
// @Tags cache
// @Param pattern query string true "缓存键模式"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cache/invalidate [post]
func (h *CacheHandlers) InvalidateCache(c *gin.Context) {
	pattern := c.Query("pattern")
	if pattern == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "pattern is required",
		})
		return
	}

	h.manager.Invalidate(c.Request.Context(), pattern)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "cache invalidated",
		"pattern": pattern,
	})
}

// ListPolicies 列出缓存策略
// @Summary 列出缓存策略
// @Tags cache
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cache/policies [get]
func (h *CacheHandlers) ListPolicies(c *gin.Context) {
	policies := h.policyManager.ListPolicies()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    policies,
	})
}

// AddPolicy 添加缓存策略
// @Summary 添加缓存策略
// @Tags cache
// @Accept json
// @Param policy body CachePolicy true "缓存策略"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cache/policies [post]
func (h *CacheHandlers) AddPolicy(c *gin.Context) {
	var policy CachePolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	h.policyManager.AddPolicy(policy)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "policy added",
		"data":    policy,
	})
}

// DeletePolicy 删除缓存策略
// @Summary 删除缓存策略
// @Tags cache
// @Param name path string true "策略名称"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/cache/policies/{name} [delete]
func (h *CacheHandlers) DeletePolicy(c *gin.Context) {
	name := c.Param("name")
	h.policyManager.RemovePolicy(name)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "policy deleted",
	})
}

// RegisterCacheRoutes 注册缓存路由
func RegisterCacheRoutes(r *gin.RouterGroup, handlers *CacheHandlers) {
	cache := r.Group("/cache")
	{
		cache.GET("/stats", handlers.GetStats)
		cache.POST("/clear", handlers.ClearCache)
		cache.POST("/invalidate", handlers.InvalidateCache)
		cache.GET("/policies", handlers.ListPolicies)
		cache.POST("/policies", handlers.AddPolicy)
		cache.DELETE("/policies/:name", handlers.DeletePolicy)
	}
}
