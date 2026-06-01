// Package containerimagecache 提供容器镜像缓存加速服务
// 支持 Docker Hub、GHCR、阿里云镜像仓库等多仓库代理
// 实现本地缓存、智能预取、带宽控制和垃圾回收等功能
package containerimagecache

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// init 注册容器镜像缓存模块
// 模块启动时自动初始化，提供镜像缓存加速服务
func init() {
	log.Println("[ContainerImageCache] 模块已注册，提供容器镜像缓存加速服务")
}

// RegistryType 镜像仓库类型
type RegistryType string

const (
	// RegistryDockerHub Docker Hub 官方仓库
	RegistryDockerHub RegistryType = "dockerhub"
	// RegistryGHCR GitHub Container Registry
	RegistryGHCR RegistryType = "ghcr"
	// RegistryAliyun 阿里云容器镜像服务
	RegistryAliyun RegistryType = "aliyun"
	// RegistryCustom 自定义仓库
	RegistryCustom RegistryType = "custom"
)

// CacheStrategy 缓存策略类型
type CacheStrategy string

const (
	// StrategyLRU 最近最少使用策略
	StrategyLRU CacheStrategy = "lru"
	// StrategyLFU 最不经常使用策略
	StrategyLFU CacheStrategy = "lfu"
	// StrategyFIFO 先进先出策略
	StrategyFIFO CacheStrategy = "fifo"
)

// ImageInfo 镜像信息
type ImageInfo struct {
	// Name 镜像名称（如 nginx:latest）
	Name string `json:"name"`
	// Registry 所属仓库
	Registry RegistryType `json:"registry"`
	// Size 镜像大小（字节）
	Size int64 `json:"size"`
	// Digest 镜像摘要
	Digest string `json:"digest"`
	// CreatedAt 创建时间
	CreatedAt time.Time `json:"created_at"`
	// LastAccessed 最后访问时间
	LastAccessed time.Time `json:"last_accessed"`
	// AccessCount 访问次数
	AccessCount int64 `json:"access_count"`
	// Tags 镜像标签列表
	Tags []string `json:"tags"`
	// IsPinned 是否固定（不参与垃圾回收）
	IsPinned bool `json:"is_pinned"`
}

// RegistryConfig 仓库配置
type RegistryConfig struct {
	// Type 仓库类型
	Type RegistryType `json:"type"`
	// URL 仓库地址
	URL string `json:"url"`
	// Username 认证用户名
	Username string `json:"username,omitempty"`
	// Password 认证密码
	Password string `json:"password,omitempty"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// Priority 优先级（数字越小优先级越高）
	Priority int `json:"priority"`
}

// CacheStats 缓存统计信息
type CacheStats struct {
	// TotalImages 缓存镜像总数
	TotalImages int `json:"total_images"`
	// TotalSize 缓存总大小（字节）
	TotalSize int64 `json:"total_size"`
	// HitCount 缓存命中次数
	HitCount int64 `json:"hit_count"`
	// MissCount 缓存未命中次数
	MissCount int64 `json:"miss_count"`
	// HitRate 缓存命中率
	HitRate float64 `json:"hit_rate"`
	// PrefetchCount 预取次数
	PrefetchCount int64 `json:"prefetch_count"`
	// GCCount 垃圾回收次数
	GCCount int64 `json:"gc_count"`
	// GCFreedSize GC释放的空间（字节）
	GCFreedSize int64 `json:"gc_freed_size"`
	// BandwidthUsage 当前带宽使用（字节/秒）
	BandwidthUsage int64 `json:"bandwidth_usage"`
	// Uptime 运行时间
	Uptime time.Duration `json:"uptime"`
}

// PrefetchRule 预取规则
type PrefetchRule struct {
	// Name 规则名称
	Name string `json:"name"`
	// ImagePattern 镜像名称模式（支持通配符）
	ImagePattern string `json:"image_pattern"`
	// Schedule 预取调度（cron 表达式）
	Schedule string `json:"schedule"`
	// Enabled 是否启用
	Enabled bool `json:"enabled"`
	// Tags 要预取的标签
	Tags []string `json:"tags"`
}

// ImageCacheManager 容器镜像缓存管理器
// 提供完整的镜像缓存加速服务，包括多仓库代理、智能预取、缓存策略管理等功能
type ImageCacheManager struct {
	mu sync.RWMutex

	// 配置
	config *CacheConfig

	// 缓存存储
	cache map[string]*ImageInfo

	// 仓库配置
	registries map[RegistryType]*RegistryConfig

	// 预取规则
	prefetchRules []*PrefetchRule

	// 统计信息
	stats *CacheStats

	// 控制通道
	ctx    context.Context
	cancel context.CancelFunc

	// 带宽限制器
	bandwidthLimiter *BandwidthLimiter

	// 频率限制器（用于访问计数）
	frequencyMap map[string]int64

	// 启动时间
	startTime time.Time
}

// CacheConfig 缓存配置
type CacheConfig struct {
	// MaxSize 最大缓存大小（字节）
	MaxSize int64 `json:"max_size"`
	// MaxImages 最大镜像数量
	MaxImages int `json:"max_images"`
	// DefaultTTL 默认缓存过期时间
	DefaultTTL time.Duration `json:"default_ttl"`
	// GCInterval 垃圾回收间隔
	GCInterval time.Duration `json:"gc_interval"`
	// Strategy 缓存策略
	Strategy CacheStrategy `json:"strategy"`
	// BandwidthLimit 带宽限制（字节/秒），0表示不限制
	BandwidthLimit int64 `json:"bandwidth_limit"`
	// PrefetchEnabled 是否启用预取
	PrefetchEnabled bool `json:"prefetch_enabled"`
	// ListenPort 监听端口
	ListenPort int `json:"listen_port"`
	// StoragePath 缓存存储路径
	StoragePath string `json:"storage_path"`
}

// DefaultCacheConfig 返回默认缓存配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		MaxSize:         50 * 1024 * 1024 * 1024, // 50GB
		MaxImages:       1000,
		DefaultTTL:      7 * 24 * time.Hour, // 7天
		GCInterval:      1 * time.Hour,
		Strategy:        StrategyLRU,
		BandwidthLimit:  0, // 不限制
		PrefetchEnabled: true,
		ListenPort:      5000,
		StoragePath:     "/var/lib/container-image-cache",
	}
}

// BandwidthLimiter 带宽限制器
type BandwidthLimiter struct {
	mu sync.Mutex

	// limit 限制速率（字节/秒）
	limit int64

	// tokens 当前可用令牌数
	tokens int64

	// lastUpdate 上次更新时间
	lastUpdate time.Time

	// maxTokens 最大令牌数
	maxTokens int64
}

// NewBandwidthLimiter 创建带宽限制器
// limit: 限制速率（字节/秒），0表示不限制
func NewBandwidthLimiter(limit int64) *BandwidthLimiter {
	return &BandwidthLimiter{
		limit:      limit,
		tokens:     limit,
		lastUpdate: time.Now(),
		maxTokens:  limit,
	}
}

// Acquire 获取指定字节数的带宽令牌
// 如果令牌不足，将阻塞等待
func (bl *BandwidthLimiter) Acquire(bytes int64) {
	if bl.limit <= 0 {
		return // 不限制
	}

	bl.mu.Lock()
	defer bl.mu.Unlock()

	// 补充令牌
	now := time.Now()
	elapsed := now.Sub(bl.lastUpdate).Seconds()
	bl.tokens += int64(elapsed * float64(bl.limit))
	if bl.tokens > bl.maxTokens {
		bl.tokens = bl.maxTokens
	}
	bl.lastUpdate = now

	// 等待令牌
	for bl.tokens < bytes {
		waitTime := time.Duration(float64(bytes-bl.tokens) / float64(bl.limit) * float64(time.Second))
		bl.mu.Unlock()
		time.Sleep(waitTime)
		bl.mu.Lock()

		// 重新补充令牌
		now = time.Now()
		elapsed = now.Sub(bl.lastUpdate).Seconds()
		bl.tokens += int64(elapsed * float64(bl.limit))
		if bl.tokens > bl.maxTokens {
			bl.tokens = bl.maxTokens
		}
		bl.lastUpdate = now
	}

	bl.tokens -= bytes
}

// SetLimit 更新带宽限制
func (bl *BandwidthLimiter) SetLimit(limit int64) {
	bl.mu.Lock()
	defer bl.mu.Unlock()

	bl.limit = limit
	bl.maxTokens = limit
	bl.tokens = limit
	bl.lastUpdate = time.Now()
}

// New 创建新的镜像缓存管理器
// config: 缓存配置，传 nil 使用默认配置
func New(config *CacheConfig) *ImageCacheManager {
	if config == nil {
		config = DefaultCacheConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	manager := &ImageCacheManager{
		config:           config,
		cache:            make(map[string]*ImageInfo),
		registries:       make(map[RegistryType]*RegistryConfig),
		prefetchRules:    make([]*PrefetchRule, 0),
		stats:            &CacheStats{},
		ctx:              ctx,
		cancel:           cancel,
		bandwidthLimiter: NewBandwidthLimiter(config.BandwidthLimit),
		frequencyMap:     make(map[string]int64),
		startTime:        time.Now(),
	}

	// 初始化默认仓库配置
	manager.initDefaultRegistries()

	return manager
}

// initDefaultRegistries 初始化默认仓库配置
func (m *ImageCacheManager) initDefaultRegistries() {
	m.registries[RegistryDockerHub] = &RegistryConfig{
		Type:     RegistryDockerHub,
		URL:      "https://registry-1.docker.io",
		Enabled:  true,
		Priority: 1,
	}

	m.registries[RegistryGHCR] = &RegistryConfig{
		Type:     RegistryGHCR,
		URL:      "https://ghcr.io",
		Enabled:  true,
		Priority: 2,
	}

	m.registries[RegistryAliyun] = &RegistryConfig{
		Type:     RegistryAliyun,
		URL:      "https://registry.cn-hangzhou.aliyuncs.com",
		Enabled:  true,
		Priority: 3,
	}
}

// Start 启动镜像缓存管理器
// 启动后台任务：垃圾回收、预取调度、统计更新
func (m *ImageCacheManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Printf("[ContainerImageCache] 启动镜像缓存管理器，监听端口: %d", m.config.ListenPort)
	log.Printf("[ContainerImageCache] 缓存策略: %s, 最大容量: %d MB", m.config.Strategy, m.config.MaxSize/(1024*1024))

	// 启动垃圾回收
	go m.runGarbageCollector()

	// 启动预取调度器
	if m.config.PrefetchEnabled {
		go m.runPrefetchScheduler()
	}

	// 启动统计更新器
	go m.runStatsUpdater()

	// 启动带宽监控
	go m.runBandwidthMonitor()

	log.Println("[ContainerImageCache] 镜像缓存管理器启动完成")
	return nil
}

// Stop 停止镜像缓存管理器
func (m *ImageCacheManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Println("[ContainerImageCache] 正在停止镜像缓存管理器...")
	m.cancel()
	log.Println("[ContainerImageCache] 镜像缓存管理器已停止")
}

// Pull 拉取镜像到本地缓存
// imageName: 镜像名称（如 "nginx:latest"、"ghcr.io/owner/repo:tag"）
// 返回缓存的镜像信息和错误
func (m *ImageCacheManager) Pull(imageName string) (*ImageInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 解析镜像名称
	registry, name := m.parseImageName(imageName)

	// 检查缓存是否存在
	if cached, exists := m.cache[name]; exists {
		// 缓存命中
		cached.LastAccessed = time.Now()
		cached.AccessCount++
		m.stats.HitCount++
		m.updateHitRate()

		// 更新频率映射
		m.frequencyMap[name]++

		log.Printf("[ContainerImageCache] 缓存命中: %s (访问次数: %d)", name, cached.AccessCount)
		return cached, nil
	}

	// 缓存未命中，从上游拉取
	m.stats.MissCount++
	m.updateHitRate()

	log.Printf("[ContainerImageCache] 缓存未命中，开始拉取: %s", name)

	// 模拟从上游拉取（实际实现需要调用 Docker Registry API）
	imageInfo, err := m.fetchFromUpstream(registry, name)
	if err != nil {
		return nil, fmt.Errorf("拉取镜像失败: %w", err)
	}

	// 检查缓存容量
	if err := m.ensureCacheSpace(imageInfo.Size); err != nil {
		return nil, fmt.Errorf("缓存空间不足: %w", err)
	}

	// 存入缓存
	m.cache[name] = imageInfo
	m.stats.TotalImages++
	m.stats.TotalSize += imageInfo.Size

	// 更新频率映射
	m.frequencyMap[name] = 1

	log.Printf("[ContainerImageCache] 镜像拉取完成: %s (大小: %d MB)", name, imageInfo.Size/(1024*1024))
	return imageInfo, nil
}

// GetStats 获取缓存统计信息
func (m *ImageCacheManager) GetStats() *CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 更新运行时间
	m.stats.Uptime = time.Since(m.startTime)

	// 返回统计信息的副本
	stats := *m.stats
	return &stats
}

// AddRegistry 添加自定义仓库配置
func (m *ImageCacheManager) AddRegistry(config *RegistryConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.Type == "" {
		return fmt.Errorf("仓库类型不能为空")
	}

	if config.URL == "" {
		return fmt.Errorf("仓库URL不能为空")
	}

	m.registries[config.Type] = config
	log.Printf("[ContainerImageCache] 添加仓库: %s (%s)", config.Type, config.URL)
	return nil
}

// RemoveRegistry 移除仓库配置
func (m *ImageCacheManager) RemoveRegistry(registryType RegistryType) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.registries[registryType]; !exists {
		return fmt.Errorf("仓库不存在: %s", registryType)
	}

	delete(m.registries, registryType)
	log.Printf("[ContainerImageCache] 移除仓库: %s", registryType)
	return nil
}

// AddPrefetchRule 添加预取规则
func (m *ImageCacheManager) AddPrefetchRule(rule *PrefetchRule) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.prefetchRules = append(m.prefetchRules, rule)
	log.Printf("[ContainerImageCache] 添加预取规则: %s", rule.Name)
}

// SetBandwidthLimit 设置带宽限制
// limit: 限制速率（字节/秒），0表示不限制
func (m *ImageCacheManager) SetBandwidthLimit(limit int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config.BandwidthLimit = limit
	m.bandwidthLimiter.SetLimit(limit)
	log.Printf("[ContainerImageCache] 设置带宽限制: %d MB/s", limit/(1024*1024))
}

// Pin 固定镜像（不参与垃圾回收）
func (m *ImageCacheManager) Pin(imageName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cached, exists := m.cache[imageName]; exists {
		cached.IsPinned = true
		log.Printf("[ContainerImageCache] 固定镜像: %s", imageName)
		return nil
	}

	return fmt.Errorf("镜像不存在: %s", imageName)
}

// Unpin 取消固定镜像
func (m *ImageCacheManager) Unpin(imageName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cached, exists := m.cache[imageName]; exists {
		cached.IsPinned = false
		log.Printf("[ContainerImageCache] 取消固定镜像: %s", imageName)
		return nil
	}

	return fmt.Errorf("镜像不存在: %s", imageName)
}

// Delete 删除指定镜像
func (m *ImageCacheManager) Delete(imageName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cached, exists := m.cache[imageName]; exists {
		m.stats.TotalImages--
		m.stats.TotalSize -= cached.Size
		delete(m.cache, imageName)
		delete(m.frequencyMap, imageName)
		log.Printf("[ContainerImageCache] 删除镜像: %s", imageName)
		return nil
	}

	return fmt.Errorf("镜像不存在: %s", imageName)
}

// ListCachedImages 列出所有缓存的镜像
func (m *ImageCacheManager) ListCachedImages() []*ImageInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	images := make([]*ImageInfo, 0, len(m.cache))
	for _, info := range m.cache {
		images = append(images, info)
	}

	// 按最后访问时间排序
	sort.Slice(images, func(i, j int) bool {
		return images[i].LastAccessed.After(images[j].LastAccessed)
	})

	return images
}

// parseImageName 解析镜像名称
// 返回仓库类型和标准化的镜像名称
func (m *ImageCacheManager) parseImageName(imageName string) (RegistryType, string) {
	// 简化解析，实际实现需要更复杂的解析逻辑
	if len(imageName) > 8 && imageName[:8] == "ghcr.io/" {
		return RegistryGHCR, imageName
	}

	if len(imageName) > 28 && imageName[:28] == "registry.cn-hangzhou.aliyuncs.com/" {
		return RegistryAliyun, imageName
	}

	// 默认 Docker Hub
	return RegistryDockerHub, imageName
}

// fetchFromUpstream 从上游仓库拉取镜像
// 这里是模拟实现，实际需要调用 Docker Registry API
func (m *ImageCacheManager) fetchFromUpstream(registry RegistryType, imageName string) (*ImageInfo, error) {
	// 应用带宽限制
	// 模拟镜像大小：100MB
	imageSize := int64(100 * 1024 * 1024)
	m.bandwidthLimiter.Acquire(imageSize)

	// 模拟拉取延迟
	time.Sleep(100 * time.Millisecond)

	registryConfig, exists := m.registries[registry]
	if !exists {
		return nil, fmt.Errorf("未配置的仓库: %s", registry)
	}

	if !registryConfig.Enabled {
		return nil, fmt.Errorf("仓库已禁用: %s", registry)
	}

	return &ImageInfo{
		Name:         imageName,
		Registry:     registry,
		Size:         imageSize,
		Digest:       fmt.Sprintf("sha256:%x", time.Now().UnixNano()),
		CreatedAt:    time.Now(),
		LastAccessed: time.Now(),
		AccessCount:  1,
		Tags:         []string{"latest"},
		IsPinned:     false,
	}, nil
}

// ensureCacheSpace 确保缓存有足够空间
func (m *ImageCacheManager) ensureCacheSpace(requiredSize int64) error {
	// 检查镜像数量限制
	if len(m.cache) >= m.config.MaxImages {
		if err := m.evictImages(1); err != nil {
			return err
		}
	}

	// 检查大小限制
	for m.stats.TotalSize+requiredSize > m.config.MaxSize {
		if err := m.evictImages(1); err != nil {
			return err
		}
	}

	return nil
}

// evictImages 驱逐指定数量的镜像
func (m *ImageCacheManager) evictImages(count int) error {
	if len(m.cache) == 0 {
		return fmt.Errorf("缓存为空，无法驱逐")
	}

	// 根据策略选择要驱逐的镜像
	var candidates []*ImageInfo
	for _, info := range m.cache {
		if !info.IsPinned {
			candidates = append(candidates, info)
		}
	}

	if len(candidates) == 0 {
		return fmt.Errorf("所有镜像都已固定，无法驱逐")
	}

	// 根据策略排序
	switch m.config.Strategy {
	case StrategyLRU:
		// 按最后访问时间排序（最久未访问的在前）
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].LastAccessed.Before(candidates[j].LastAccessed)
		})
	case StrategyLFU:
		// 按访问次数排序（访问次数最少的在前）
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].AccessCount < candidates[j].AccessCount
		})
	case StrategyFIFO:
		// 按创建时间排序（最早创建的在前）
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].CreatedAt.Before(candidates[j].CreatedAt)
		})
	}

	// 驱逐镜像
	for i := 0; i < count && i < len(candidates); i++ {
		image := candidates[i]
		m.stats.TotalImages--
		m.stats.TotalSize -= image.Size
		delete(m.cache, image.Name)
		delete(m.frequencyMap, image.Name)
		log.Printf("[ContainerImageCache] 驱逐镜像: %s (策略: %s)", image.Name, m.config.Strategy)
	}

	return nil
}

// runGarbageCollector 运行垃圾回收
func (m *ImageCacheManager) runGarbageCollector() {
	ticker := time.NewTicker(m.config.GCInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.runGC()
		}
	}
}

// runGC 执行一次垃圾回收
func (m *ImageCacheManager) runGC() {
	m.mu.Lock()
	defer m.mu.Unlock()

	log.Println("[ContainerImageCache] 开始垃圾回收...")

	var freedSize int64
	var freedCount int
	var expiredImages []string

	// 查找过期镜像
	for name, info := range m.cache {
		if info.IsPinned {
			continue
		}

		// 检查是否过期
		if time.Since(info.CreatedAt) > m.config.DefaultTTL {
			expiredImages = append(expiredImages, name)
		}
	}

	// 删除过期镜像
	for _, name := range expiredImages {
		if info, exists := m.cache[name]; exists {
			freedSize += info.Size
			freedCount++
			m.stats.TotalImages--
			m.stats.TotalSize -= info.Size
			delete(m.cache, name)
			delete(m.frequencyMap, name)
		}
	}

	// 更新统计
	m.stats.GCCount++
	m.stats.GCFreedSize += freedSize

	if freedCount > 0 {
		log.Printf("[ContainerImageCache] 垃圾回收完成: 清理 %d 个镜像，释放 %d MB", freedCount, freedSize/(1024*1024))
	}
}

// runPrefetchScheduler 运行预取调度器
func (m *ImageCacheManager) runPrefetchScheduler() {
	ticker := time.NewTicker(5 * time.Minute) // 每5分钟检查一次
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.runPrefetch()
		}
	}
}

// runPrefetch 执行预取
func (m *ImageCacheManager) runPrefetch() {
	m.mu.RLock()
	rules := make([]*PrefetchRule, len(m.prefetchRules))
	copy(rules, m.prefetchRules)
	m.mu.RUnlock()

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// 简化实现：检查规则匹配的镜像是否已缓存
		// 实际实现需要解析 cron 表达式并执行预取
		log.Printf("[ContainerImageCache] 检查预取规则: %s", rule.Name)
	}
}

// runStatsUpdater 运行统计更新器
func (m *ImageCacheManager) runStatsUpdater() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.updateStats()
		}
	}
}

// updateStats 更新统计信息
func (m *ImageCacheManager) updateStats() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stats.Uptime = time.Since(m.startTime)
	m.updateHitRate()
}

// updateHitRate 更新命中率
func (m *ImageCacheManager) updateHitRate() {
	total := m.stats.HitCount + m.stats.MissCount
	if total > 0 {
		m.stats.HitRate = float64(m.stats.HitCount) / float64(total)
	}
}

// runBandwidthMonitor 运行带宽监控
func (m *ImageCacheManager) runBandwidthMonitor() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastTotalSize int64

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.mu.RLock()
			currentSize := m.stats.TotalSize
			m.mu.RUnlock()

			// 计算带宽使用（简化实现）
			m.stats.BandwidthUsage = currentSize - lastTotalSize
			lastTotalSize = currentSize
		}
	}
}

// GetCacheConfig 获取当前缓存配置
func (m *ImageCacheManager) GetCacheConfig() *CacheConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	config := *m.config
	return &config
}

// UpdateCacheConfig 更新缓存配置
func (m *ImageCacheManager) UpdateCacheConfig(config *CacheConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.config = config
	if config.BandwidthLimit != m.bandwidthLimiter.limit {
		m.bandwidthLimiter.SetLimit(config.BandwidthLimit)
	}

	log.Printf("[ContainerImageCache] 更新缓存配置")
}

// GetRegistries 获取所有仓库配置
func (m *ImageCacheManager) GetRegistries() map[RegistryType]*RegistryConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	registries := make(map[RegistryType]*RegistryConfig)
	for k, v := range m.registries {
		config := *v
		registries[k] = &config
	}

	return registries
}

// GetPrefetchRules 获取所有预取规则
func (m *ImageCacheManager) GetPrefetchRules() []*PrefetchRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*PrefetchRule, len(m.prefetchRules))
	copy(rules, m.prefetchRules)
	return rules
}

// ClearCache 清空所有缓存
func (m *ImageCacheManager) ClearCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache = make(map[string]*ImageInfo)
	m.frequencyMap = make(map[string]int64)
	m.stats.TotalImages = 0
	m.stats.TotalSize = 0

	log.Println("[ContainerImageCache] 缓存已清空")
}

// GetImageInfo 获取指定镜像信息
func (m *ImageCacheManager) GetImageInfo(imageName string) (*ImageInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if info, exists := m.cache[imageName]; exists {
		infoCopy := *info
		return &infoCopy, nil
	}

	return nil, fmt.Errorf("镜像不存在: %s", imageName)
}

// IsCached 检查镜像是否已缓存
func (m *ImageCacheManager) IsCached(imageName string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.cache[imageName]
	return exists
}

// GetCacheSize 获取当前缓存大小
func (m *ImageCacheManager) GetCacheSize() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats.TotalSize
}

// GetCacheImageCount 获取缓存镜像数量
func (m *ImageCacheManager) GetCacheImageCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats.TotalImages
}
