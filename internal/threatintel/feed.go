// Package threatintel - feed.go 实现威胁情报订阅功能，包括情报源自动更新、
// 可信源管理和情报共享。
package threatintel

import (
	"sync"
	"time"
)

// ============================================================
// 情报订阅管理
// ============================================================

// FeedManager 情报订阅管理器
type FeedManager struct {
	engine     *Engine
	subscribers map[string][]chan *IOC
	mu         sync.RWMutex
	stopChan   chan struct{}
}

// NewFeedManager 创建情报订阅管理器
func NewFeedManager(engine *Engine) *FeedManager {
	return &FeedManager{
		engine:      engine,
		subscribers: make(map[string][]chan *IOC),
		stopChan:    make(chan struct{}),
	}
}

// Subscribe 订阅情报更新
func (fm *FeedManager) Subscribe(feedID string) <-chan *IOC {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	ch := make(chan *IOC, 100)
	fm.subscribers[feedID] = append(fm.subscribers[feedID], ch)
	return ch
}

// Unsubscribe 取消订阅
func (fm *FeedManager) Unsubscribe(feedID string, ch <-chan *IOC) {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	subs := fm.subscribers[feedID]
	for i, sub := range subs {
		if sub == ch {
			fm.subscribers[feedID] = append(subs[:i], subs[i+1:]...)
			break
		}
	}
}

// NotifySubscribers 通知订阅者有新的 IOC
func (fm *FeedManager) NotifySubscribers(feedID string, ioc *IOC) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	subs := fm.subscribers[feedID]
	for _, ch := range subs {
		select {
		case ch <- ioc:
		default:
			// 队列满，跳过
		}
	}
}

// Stop 停止订阅管理器
func (fm *FeedManager) Stop() {
	close(fm.stopChan)
}

// ============================================================
// 情报共享
// ============================================================

// SharingConfig 情报共享配置
type SharingConfig struct {
	// Enabled 是否启用共享
	Enabled bool `json:"enabled"`
	// SharedIOCs 共享的 IOC 列表
	SharedIOCs []string `json:"shared_iocs"`
	// ExportFormat 导出格式
	ExportFormat string `json:"export_format"` // "json", "csv", "stix"
	// AutoExport 是否自动导出
	AutoExport bool `json:"auto_export"`
	// ExportInterval 自动导出间隔
	ExportInterval time.Duration `json:"export_interval"`
}

// DefaultSharingConfig 默认共享配置
func DefaultSharingConfig() *SharingConfig {
	return &SharingConfig{
		Enabled:        true,
		SharedIOCs:     make([]string, 0),
		ExportFormat:   "json",
		AutoExport:     false,
		ExportInterval: 24 * time.Hour,
	}
}

// SharingManager 情报共享管理器
type SharingManager struct {
	config     *SharingConfig
	engine     *Engine
	exportChan chan *IOC
	mu         sync.RWMutex
}

// NewSharingManager 创建共享管理器
func NewSharingManager(config *SharingConfig, engine *Engine) *SharingManager {
	if config == nil {
		config = DefaultSharingConfig()
	}
	return &SharingManager{
		config:     config,
		engine:     engine,
		exportChan: make(chan *IOC, 1000),
	}
}

// ExportIOCs 导出 IOC 列表
func (sm *SharingManager) ExportIOCs() []*IOC {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	iocs := sm.engine.ListIOCs()
	var exportable []*IOC

	for _, ioc := range iocs {
		// 只导出未过期且有情报源的 IOC
		if ioc.ExpiresAt == nil || time.Now().Before(*ioc.ExpiresAt) {
			exportable = append(exportable, ioc)
		}
	}

	return exportable
}

// ImportIOCs 导入 IOC 列表
func (sm *SharingManager) ImportIOCs(iocs []*IOC, sourceID string) (int, int) {
	imported := 0
	skipped := 0

	validator := NewIOCValidator()
	for _, ioc := range iocs {
		if err := validator.ValidateIOC(ioc); err != nil {
			skipped++
			continue
		}

		// 检查是否已存在
		if existing := sm.engine.LookupIOC(ioc.Type, ioc.Value); existing != nil {
			// 更新最后发现时间
			existing.LastSeen = time.Now()
			if ioc.ThreatScore > existing.ThreatScore {
				existing.ThreatScore = ioc.ThreatScore
			}
			skipped++
			continue
		}

		ioc.SourceID = sourceID
		if err := sm.engine.AddIOC(ioc); err != nil {
			skipped++
			continue
		}

		imported++
	}

	return imported, skipped
}

// GetExportStats 获取导出统计
func (sm *SharingManager) GetExportStats() map[string]int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := make(map[string]int)
	iocs := sm.engine.ListIOCs()

	for _, ioc := range iocs {
		stats[string(ioc.Type)]++
	}

	stats["total"] = len(iocs)
	return stats
}

// ============================================================
// 可信源管理
// ============================================================

// TrustedSource 可信情报源
type TrustedSource struct {
	// ID 源 ID
	ID string `json:"id"`
	// Name 源名称
	Name string `json:"name"`
	// TrustLevel 信任级别（0-100）
	TrustLevel int `json:"trust_level"`
	// Category 源类别
	Category string `json:"category"` // "government", "research", "commercial", "community"
	// URL 源 URL
	URL string `json:"url"`
	// Description 描述
	Description string `json:"description"`
	// LastVerified 最后验证时间
	LastVerified time.Time `json:"last_verified"`
	// Verified 是否已验证
	Verified bool `json:"verified"`
}

// TrustedSourceManager 可信源管理器
type TrustedSourceManager struct {
	sources map[string]*TrustedSource
	mu      sync.RWMutex
}

// NewTrustedSourceManager 创建可信源管理器
func NewTrustedSourceManager() *TrustedSourceManager {
	tsm := &TrustedSourceManager{
		sources: make(map[string]*TrustedSource),
	}

	// 添加默认可信源
	tsm.AddDefaults()
	return tsm
}

// AddDefaults 添加默认可信源
func (tsm *TrustedSourceManager) AddDefaults() {
	defaults := []*TrustedSource{
		{
			ID: "cisa", Name: "CISA", TrustLevel: 95, Category: "government",
			URL: "https://www.cisa.gov", Description: "美国网络安全和基础设施安全局",
			Verified: true, LastVerified: time.Now(),
		},
		{
			ID: "mitre", Name: "MITRE ATT&CK", TrustLevel: 90, Category: "research",
			URL: "https://attack.mitre.org", Description: "MITRE ATT&CK 知识库",
			Verified: true, LastVerified: time.Now(),
		},
		{
			ID: "nvd", Name: "NVD", TrustLevel: 95, Category: "government",
			URL: "https://nvd.nist.gov", Description: "美国国家漏洞数据库",
			Verified: true, LastVerified: time.Now(),
		},
		{
			ID: "virustotal", Name: "VirusTotal", TrustLevel: 85, Category: "commercial",
			URL: "https://www.virustotal.com", Description: "VirusTotal 恶意软件分析平台",
			Verified: true, LastVerified: time.Now(),
		},
		{
			ID: "otx", Name: "AlienVault OTX", TrustLevel: 80, Category: "community",
			URL: "https://otx.alienvault.com", Description: "AlienVault 开放威胁情报交换",
			Verified: true, LastVerified: time.Now(),
		},
	}

	for _, src := range defaults {
		tsm.sources[src.ID] = src
	}
}

// Add 添加可信源
func (tsm *TrustedSourceManager) Add(source *TrustedSource) {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	tsm.sources[source.ID] = source
}

// Remove 移除可信源
func (tsm *TrustedSourceManager) Remove(id string) {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()
	delete(tsm.sources, id)
}

// Get 获取可信源
func (tsm *TrustedSourceManager) Get(id string) (*TrustedSource, bool) {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()
	src, exists := tsm.sources[id]
	return src, exists
}

// List 列出所有可信源
func (tsm *TrustedSourceManager) List() []*TrustedSource {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()

	sources := make([]*TrustedSource, 0, len(tsm.sources))
	for _, s := range tsm.sources {
		sources = append(sources, s)
	}
	return sources
}

// GetByTrustLevel 按信任级别筛选
func (tsm *TrustedSourceManager) GetByTrustLevel(minLevel int) []*TrustedSource {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()

	var sources []*TrustedSource
	for _, s := range tsm.sources {
		if s.TrustLevel >= minLevel {
			sources = append(sources, s)
		}
	}
	return sources
}

// GetVerified 获取已验证的可信源
func (tsm *TrustedSourceManager) GetVerified() []*TrustedSource {
	tsm.mu.RLock()
	defer tsm.mu.RUnlock()

	var sources []*TrustedSource
	for _, s := range tsm.sources {
		if s.Verified {
			sources = append(sources, s)
		}
	}
	return sources
}

// VerifySource 验证情报源
func (tsm *TrustedSourceManager) VerifySource(id string) error {
	tsm.mu.Lock()
	defer tsm.mu.Unlock()

	source, exists := tsm.sources[id]
	if !exists {
		return ErrFeedNotFound
	}

	source.Verified = true
	source.LastVerified = time.Now()
	return nil
}

// ============================================================
// 自动更新调度
// ============================================================

// UpdateScheduler 自动更新调度器
type UpdateScheduler struct {
	engine    *Engine
	feedMgr   *FeedManager
	interval  time.Duration
	stopChan  chan struct{}
	running   bool
	mu        sync.Mutex
}

// NewUpdateScheduler 创建自动更新调度器
func NewUpdateScheduler(engine *Engine, feedMgr *FeedManager, interval time.Duration) *UpdateScheduler {
	return &UpdateScheduler{
		engine:   engine,
		feedMgr:  feedMgr,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start 启动自动更新
func (us *UpdateScheduler) Start() {
	us.mu.Lock()
	if us.running {
		us.mu.Unlock()
		return
	}
	us.running = true
	us.mu.Unlock()

	go us.run()
}

// Stop 停止自动更新
func (us *UpdateScheduler) Stop() {
	us.mu.Lock()
	defer us.mu.Unlock()

	if us.running {
		close(us.stopChan)
		us.running = false
	}
}

// IsRunning 是否正在运行
func (us *UpdateScheduler) IsRunning() bool {
	us.mu.Lock()
	defer us.mu.Unlock()
	return us.running
}

func (us *UpdateScheduler) run() {
	ticker := time.NewTicker(us.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			us.updateFeeds()
		case <-us.stopChan:
			return
		}
	}
}

func (us *UpdateScheduler) updateFeeds() {
	feeds := us.engine.ListFeeds()
	for _, feed := range feeds {
		if feed.Status != FeedStatusActive || !feed.Enabled {
			continue
		}

		// 检查是否需要更新
		if time.Since(feed.LastUpdate) < feed.UpdateInterval {
			continue
		}

		// 更新情报源状态
		us.engine.UpdateFeedStatus(feed.ID, FeedStatusActive)
	}
}

// GetUpdateStatus 获取更新状态
func (us *UpdateScheduler) GetUpdateStatus() map[string]interface{} {
	us.mu.Lock()
	defer us.mu.Unlock()

	return map[string]interface{}{
		"running":      us.running,
		"interval":     us.interval.String(),
		"active_feeds": len(us.engine.ListFeeds()),
	}
}
