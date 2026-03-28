// Package ransomware 提供增强型勒索软件检测组件
// 实现熵值分析、快速变更追踪、进程监控和高级模式匹配
package ransomware

import (
	"math"
	"regexp"
	"sync"
	"time"
)

// EntropyAnalyzer 熵值分析器
// 用于检测加密文件（加密文件通常具有高熵值）
type EntropyAnalyzer struct {
	mu sync.RWMutex

	// 配置
	config EntropyConfig

	// 熵值缓存
	entropyCache map[string]float64

	// 统计
	stats EntropyStats
}

// EntropyConfig 熵值分析配置
type EntropyConfig struct {
	// 熵值阈值（0-8，默认7.5）
	Threshold float64 `json:"threshold"`

	// 是否启用
	Enabled bool `json:"enabled"`

	// 采样大小（字节）
	SampleSize int64 `json:"sampleSize"`

	// 缓存TTL
	CacheTTL Duration `json:"cacheTtl"`
}

// EntropyStats 熵值统计
type EntropyStats struct {
	FilesAnalyzed   int64   `json:"filesAnalyzed"`
	HighEntropyFiles int64  `json:"highEntropyFiles"`
	AvgEntropy      float64 `json:"avgEntropy"`
}

// NewEntropyAnalyzer 创建熵值分析器
func NewEntropyAnalyzer(config EntropyConfig) *EntropyAnalyzer {
	if config.Threshold <= 0 {
		config.Threshold = 7.5
	}
	if config.SampleSize <= 0 {
		config.SampleSize = 4096
	}

	return &EntropyAnalyzer{
		config:       config,
		entropyCache: make(map[string]float64),
	}
}

// Analyze 分析文件的熵值
func (e *EntropyAnalyzer) Analyze(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	// 计算字节频率
	freq := make(map[byte]int)
	for _, b := range data {
		freq[b]++
	}

	// 计算熵值
	var entropy float64
	length := float64(len(data))
	for _, count := range freq {
		if count > 0 {
			p := float64(count) / length
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

// IsHighEntropy 判断是否为高熵值（可能是加密文件）
func (e *EntropyAnalyzer) IsHighEntropy(entropy float64) bool {
	return entropy >= e.config.Threshold
}

// GetStats 获取统计信息
func (e *EntropyAnalyzer) GetStats() EntropyStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.stats
}

// RapidChangeTracker 快速变更追踪器
// 检测短时间内大量文件修改（勒索软件特征行为）
type RapidChangeTracker struct {
	mu sync.RWMutex

	// 配置
	config RapidChangeConfig

	// 事件记录
	events []RapidChangeEvent

	// 统计
	stats RapidChangeStats
}

// RapidChangeConfig 快速变更配置
type RapidChangeConfig struct {
	// 时间窗口（秒）
	TimeWindow int `json:"timeWindow"`

	// 文件数量阈值
	FileThreshold int `json:"fileThreshold"`

	// 扩展名变更阈值
	ExtensionChangeThreshold int `json:"extensionChangeThreshold"`

	// 是否启用
	Enabled bool `json:"enabled"`
}

// RapidChangeEvent 快速变更事件
type RapidChangeEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Path      string    `json:"path"`
	Operation string    `json:"operation"`
	Size      int64     `json:"size"`
	Extension string    `json:"extension"`
}

// RapidChangeStats 快速变更统计
type RapidChangeStats struct {
	TotalEvents        int64 `json:"totalEvents"`
	RecentEventCount   int   `json:"recentEventCount"`
	ExtensionChanges   int   `json:"extensionChanges"`
	HighActivityPeriods int  `json:"highActivityPeriods"`
}

// NewRapidChangeTracker 创建快速变更追踪器
func NewRapidChangeTracker(config RapidChangeConfig) *RapidChangeTracker {
	if config.TimeWindow <= 0 {
		config.TimeWindow = 60 // 默认60秒
	}
	if config.FileThreshold <= 0 {
		config.FileThreshold = 100 // 默认100个文件
	}

	return &RapidChangeTracker{
		config: config,
		events:  make([]RapidChangeEvent, 0),
	}
}

// RecordEvent 记录变更事件
func (r *RapidChangeTracker) RecordEvent(event RapidChangeEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.events = append(r.events, event)
	r.stats.TotalEvents++

	// 清理过期事件
	r.cleanupOldEvents()
}

// CheckRapidChange 检查是否发生快速变更
func (r *RapidChangeTracker) CheckRapidChange() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cleanupOldEvents()
	return len(r.events) >= r.config.FileThreshold
}

// GetRecentEventCount 获取最近事件数量
func (r *RapidChangeTracker) GetRecentEventCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	r.cleanupOldEvents()
	return len(r.events)
}

// cleanupOldEvents 清理过期事件
func (r *RapidChangeTracker) cleanupOldEvents() {
	cutoff := time.Now().Add(-time.Duration(r.config.TimeWindow) * time.Second)
	newEvents := make([]RapidChangeEvent, 0)
	for _, e := range r.events {
		if e.Timestamp.After(cutoff) {
			newEvents = append(newEvents, e)
		}
	}
	r.events = newEvents
	r.stats.RecentEventCount = len(newEvents)
}

// GetStats 获取统计信息
func (r *RapidChangeTracker) GetStats() RapidChangeStats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.stats
}

// ProcessActivityMonitor 进程活动监控器
// 监控可疑进程的文件操作行为
type ProcessActivityMonitor struct {
	mu sync.RWMutex

	// 配置
	config ProcessMonitorConfig

	// 进程活动记录
	processActivities map[string]*ProcessActivity

	// 可疑进程列表
	suspiciousProcesses map[string]bool

	// 统计
	stats ProcessMonitorStats
}

// ProcessMonitorConfig 进程监控配置
type ProcessMonitorConfig struct {
	// 是否启用
	Enabled bool `json:"enabled"`

	// 监控间隔
	MonitorInterval Duration `json:"monitorInterval"`

	// 可疑行为阈值
	SuspiciousThreshold int `json:"suspiciousThreshold"`

	// 白名单进程
	Whitelist []string `json:"whitelist"`

	// 黑名单进程
	Blacklist []string `json:"blacklist"`
}

// ProcessActivity 进程活动记录
type ProcessActivity struct {
	PID           int       `json:"pid"`
	Name          string    `json:"name"`
	Path          string    `json:"path"`
	User          string    `json:"user"`
	FileOperations int      `json:"fileOperations"`
	FileReads     int64     `json:"fileReads"`
	FileWrites    int64     `json:"fileWrites"`
	FileDeletes   int64     `json:"fileDeletes"`
	FileRenames   int64     `json:"fileRenames"`
	ExtensionsModified []string `json:"extensionsModified"`
	FirstSeen     time.Time `json:"firstSeen"`
	LastSeen      time.Time `json:"lastSeen"`
	SuspicionScore int      `json:"suspicionScore"`
	IsSuspicious  bool      `json:"isSuspicious"`
}

// ProcessMonitorStats 进程监控统计
type ProcessMonitorStats struct {
	MonitoredProcesses int `json:"monitoredProcesses"`
	SuspiciousProcesses int `json:"suspiciousProcesses"`
	BlockedProcesses   int `json:"blockedProcesses"`
}

// NewProcessActivityMonitor 创建进程活动监控器
func NewProcessActivityMonitor(config ProcessMonitorConfig) *ProcessActivityMonitor {
	return &ProcessActivityMonitor{
		config:              config,
		processActivities:   make(map[string]*ProcessActivity),
		suspiciousProcesses: make(map[string]bool),
	}
}

// RecordProcessActivity 记录进程活动
func (p *ProcessActivityMonitor) RecordProcessActivity(pid int, name, operation string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := name
	activity, exists := p.processActivities[key]
	if !exists {
		activity = &ProcessActivity{
			PID:       pid,
			Name:      name,
			FirstSeen: time.Now(),
		}
		p.processActivities[key] = activity
	}

	activity.LastSeen = time.Now()
	switch operation {
	case "read":
		activity.FileReads++
	case "write":
		activity.FileWrites++
		activity.FileOperations++
	case "delete":
		activity.FileDeletes++
		activity.FileOperations++
	case "rename":
		activity.FileRenames++
		activity.FileOperations++
	}

	// 计算可疑分数
	p.calculateSuspicionScore(activity)

	p.stats.MonitoredProcesses = len(p.processActivities)
	p.stats.SuspiciousProcesses = len(p.suspiciousProcesses)
}

// calculateSuspicionScore 计算可疑分数
func (p *ProcessActivityMonitor) calculateSuspicionScore(activity *ProcessActivity) {
	score := 0

	// 高频操作
	if activity.FileOperations > 100 {
		score += 20
	}
	if activity.FileOperations > 500 {
		score += 30
	}

	// 大量重命名（加密典型特征）
	if activity.FileRenames > 50 {
		score += 30
	}

	// 检查是否在白名单
	for _, whitelist := range p.config.Whitelist {
		if activity.Name == whitelist || activity.Path == whitelist {
			score = 0
			break
		}
	}

	// 检查黑名单
	for _, blacklist := range p.config.Blacklist {
		if activity.Name == blacklist || activity.Path == blacklist {
			score += 50
			break
		}
	}

	activity.SuspicionScore = score
	activity.IsSuspicious = score >= p.config.SuspiciousThreshold

	if activity.IsSuspicious {
		p.suspiciousProcesses[activity.Name] = true
	}
}

// GetSuspiciousProcesses 获取可疑进程列表
func (p *ProcessActivityMonitor) GetSuspiciousProcesses() []*ProcessActivity {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var suspicious []*ProcessActivity
	for _, activity := range p.processActivities {
		if activity.IsSuspicious {
			suspicious = append(suspicious, activity)
		}
	}
	return suspicious
}

// GetStats 获取统计信息
func (p *ProcessActivityMonitor) GetStats() ProcessMonitorStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stats
}

// AdvancedPatternMatcher 高级模式匹配器
// 使用正则表达式和启发式规则检测勒索软件特征
type AdvancedPatternMatcher struct {
	mu sync.RWMutex

	// 配置
	config PatternMatcherConfig

	// 已知勒索软件扩展名模式
	ransomwareExtensions []*regexp.Regexp

	// 已知勒索软件文件名模式
	ransomwareFilenames []*regexp.Regexp

	// 已知勒索软件内容模式
	ransomwarePatterns []*regexp.Regexp

	// 统计
	stats PatternMatcherStats
}

// PatternMatcherConfig 模式匹配配置
type PatternMatcherConfig struct {
	// 是否启用
	Enabled bool `json:"enabled"`

	// 自定义扩展名模式
	CustomExtensions []string `json:"customExtensions"`

	// 自定义文件名模式
	CustomFilenames []string `json:"customFilenames"`

	// 自定义内容模式
	CustomPatterns []string `json:"customPatterns"`
}

// PatternMatcherStats 模式匹配统计
type PatternMatcherStats struct {
	FilesScanned      int64 `json:"filesScanned"`
	PatternMatches    int64 `json:"patternMatches"`
	ExtensionMatches  int64 `json:"extensionMatches"`
	FilenameMatches   int64 `json:"filenameMatches"`
}

// NewAdvancedPatternMatcher 创建高级模式匹配器
func NewAdvancedPatternMatcher(config PatternMatcherConfig) *AdvancedPatternMatcher {
	m := &AdvancedPatternMatcher{
		config: config,
	}

	// 初始化默认模式
	m.initDefaultPatterns()

	// 添加自定义模式
	for _, ext := range config.CustomExtensions {
		m.ransomwareExtensions = append(m.ransomwareExtensions, regexp.MustCompile(ext))
	}
	for _, filename := range config.CustomFilenames {
		m.ransomwareFilenames = append(m.ransomwareFilenames, regexp.MustCompile(filename))
	}
	for _, pattern := range config.CustomPatterns {
		m.ransomwarePatterns = append(m.ransomwarePatterns, regexp.MustCompile(pattern))
	}

	return m
}

// initDefaultPatterns 初始化默认模式
func (m *AdvancedPatternMatcher) initDefaultPatterns() {
	// 常见勒索软件加密扩展名模式
	defaultExtensions := []string{
		`\.encrypted$`,
		`\.locked$`,
		`\.crypto$`,
		`\.[a-z]{8,}$`,               // 长随机扩展名
		`\.[a-z0-9]{6,}$`,            // 混合随机扩展名
		`\.id-[a-f0-9]+\.email$`,     // ID+邮箱格式
	}

	for _, ext := range defaultExtensions {
		m.ransomwareExtensions = append(m.ransomwareExtensions, regexp.MustCompile(ext))
	}

	// 常见勒索信文件名
	defaultFilenames := []string{
		`(?i)^decrypt.*\.txt$`,
		`(?i)^readme.*\.txt$`,
		`(?i)^how.*decrypt.*\.txt$`,
		`(?i)^restore.*\.txt$`,
		`(?i)^ransom.*\.txt$`,
		`(?i)_readme\.txt$`,
		`(?i)_decrypt\.txt$`,
	}

	for _, filename := range defaultFilenames {
		m.ransomwareFilenames = append(m.ransomwareFilenames, regexp.MustCompile(filename))
	}

	// 勒索信内容模式
	defaultPatterns := []string{
		`(?i)your files are encrypted`,
		`(?i)you have been hacked`,
		`(?i)pay.*bitcoin`,
		`(?i)pay.*btc`,
		`(?i)decrypt.*key`,
		`(?i)ransomware`,
		`(?i)send.*btc to`,
		`(?i)private key`,
	}

	for _, pattern := range defaultPatterns {
		m.ransomwarePatterns = append(m.ransomwarePatterns, regexp.MustCompile(pattern))
	}
}

// MatchExtension 检查扩展名是否匹配勒索软件模式
func (m *AdvancedPatternMatcher) MatchExtension(ext string) bool {
	for _, pattern := range m.ransomwareExtensions {
		if pattern.MatchString(ext) {
			m.mu.Lock()
			m.stats.ExtensionMatches++
			m.mu.Unlock()
			return true
		}
	}
	return false
}

// MatchFilename 检查文件名是否匹配勒索信模式
func (m *AdvancedPatternMatcher) MatchFilename(filename string) bool {
	for _, pattern := range m.ransomwareFilenames {
		if pattern.MatchString(filename) {
			m.mu.Lock()
			m.stats.FilenameMatches++
			m.mu.Unlock()
			return true
		}
	}
	return false
}

// MatchContent 检查内容是否匹配勒索信模式
func (m *AdvancedPatternMatcher) MatchContent(content []byte) bool {
	contentStr := string(content)
	for _, pattern := range m.ransomwarePatterns {
		if pattern.MatchString(contentStr) {
			m.mu.Lock()
			m.stats.PatternMatches++
			m.mu.Unlock()
			return true
		}
	}
	return false
}

// ScanFile 扫描文件
func (m *AdvancedPatternMatcher) ScanFile(filename, ext string, content []byte) []PatternMatchResult {
	var results []PatternMatchResult

	m.mu.Lock()
	m.stats.FilesScanned++
	m.mu.Unlock()

	if m.MatchExtension(ext) {
		results = append(results, PatternMatchResult{
			Type:    "extension",
			Match:   ext,
			Message: "文件扩展名匹配勒索软件模式",
		})
	}

	if m.MatchFilename(filename) {
		results = append(results, PatternMatchResult{
			Type:    "filename",
			Match:   filename,
			Message: "文件名匹配勒索信模式",
		})
	}

	if len(content) > 0 && m.MatchContent(content) {
		results = append(results, PatternMatchResult{
			Type:    "content",
			Match:   "content",
			Message: "文件内容匹配勒索信模式",
		})
	}

	return results
}

// GetStats 获取统计信息
func (m *AdvancedPatternMatcher) GetStats() PatternMatcherStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// PatternMatchResult 模式匹配结果
type PatternMatchResult struct {
	Type    string `json:"type"`
	Match   string `json:"match"`
	Message string `json:"message"`
}

// Duration 类型别名，用于JSON序列化
type Duration time.Duration