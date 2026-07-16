// Package smartshare 提供访问统计分析功能
package smartshare

import (
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// AnalyticsEngine 统计分析引擎.
type AnalyticsEngine struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	logs     map[string][]*AccessLog    // shareID -> logs
	visitors map[string]map[string]bool // shareID -> {ip -> true} (UV)
}

// NewAnalyticsEngine 创建统计分析引擎.
func NewAnalyticsEngine(logger *zap.Logger) *AnalyticsEngine {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &AnalyticsEngine{
		logger:   logger,
		logs:     make(map[string][]*AccessLog),
		visitors: make(map[string]map[string]bool),
	}
}

// RecordAccess 记录访问日志.
func (ae *AnalyticsEngine) RecordAccess(log *AccessLog) {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	ae.logs[log.ShareID] = append(ae.logs[log.ShareID], log)

	// 记录独立访客
	if _, ok := ae.visitors[log.ShareID]; !ok {
		ae.visitors[log.ShareID] = make(map[string]bool)
	}
	ae.visitors[log.ShareID][log.IPAddress] = true
}

// GetAnalytics 获取分享链接统计.
func (ae *AnalyticsEngine) GetAnalytics(shareID string) *ShareAnalytics {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	logs := ae.logs[shareID]
	analytics := &ShareAnalytics{
		ShareID:          shareID,
		DeviceBreakdown:  make(map[DeviceType]int),
		OSBreakdown:      make(map[string]int),
		BrowserBreakdown: make(map[string]int),
		CountryBreakdown: make(map[string]int),
		RegionBreakdown:  make(map[string]int),
		HourlyTraffic:    make(map[int]int),
		DailyTraffic:     make(map[string]int),
		TopReferers:      make([]RefererStat, 0),
		GeneratedAt:      time.Now(),
	}

	if len(logs) == 0 {
		return analytics
	}

	// 统计独立访客
	analytics.UniqueVisitors = len(ae.visitors[shareID])

	// 记录唯一下载者
	downloaders := make(map[string]bool)

	// 用于统计来源
	refererCount := make(map[string]int)
	totalDuration := int64(0)
	bounceCount := 0

	for _, log := range logs {
		// PV
		if log.Action == "view" {
			analytics.TotalViews++
		}

		// 下载统计
		if log.Action == "download" {
			analytics.TotalDownloads++
			downloaders[log.IPAddress] = true
		}

		// 设备分布
		analytics.DeviceBreakdown[log.DeviceType]++

		// 操作系统分布
		if log.OS != "" {
			analytics.OSBreakdown[log.OS]++
		}

		// 浏览器分布
		if log.Browser != "" {
			analytics.BrowserBreakdown[log.Browser]++
		}

		// 地域分布
		if log.Country != "" {
			analytics.CountryBreakdown[log.Country]++
		}
		if log.Region != "" {
			analytics.RegionBreakdown[log.Region]++
		}

		// 小时流量分布
		hour := log.Timestamp.Hour()
		analytics.HourlyTraffic[hour]++

		// 日期流量分布
		day := log.Timestamp.Format("2006-01-02")
		analytics.DailyTraffic[day]++

		// 来源统计
		if log.Referer != "" {
			refererCount[log.Referer]++
		}

		// 访问时长
		totalDuration += log.Duration

		// 跳出率（访问时长 < 5秒视为跳出）
		if log.Duration < 5000 {
			bounceCount++
		}
	}

	analytics.UniqueDownloaders = len(downloaders)

	// 平均访问时长
	if len(logs) > 0 {
		analytics.AvgDuration = float64(totalDuration) / float64(len(logs))
	}

	// 跳出率
	if len(logs) > 0 {
		analytics.BounceRate = float64(bounceCount) / float64(len(logs)) * 100
	}

	// 排序来源统计
	refererList := make([]RefererStat, 0, len(refererCount))
	for referer, count := range refererCount {
		refererList = append(refererList, RefererStat{Referer: referer, Count: count})
	}
	sort.Slice(refererList, func(i, j int) bool {
		return refererList[i].Count > refererList[j].Count
	})
	if len(refererList) > 10 {
		refererList = refererList[:10]
	}
	analytics.TopReferers = refererList

	// 最近日志（最多100条）
	recentLogs := logs
	if len(recentLogs) > 100 {
		recentLogs = recentLogs[len(recentLogs)-100:]
	}
	analytics.RecentLogs = make([]*AccessLog, len(recentLogs))
	copy(analytics.RecentLogs, recentLogs)

	return analytics
}

// GetAnalyticsSummary 获取所有分享的统计摘要.
func (ae *AnalyticsEngine) GetAnalyticsSummary() *AnalyticsSummary {
	ae.mu.RLock()
	defer ae.mu.RUnlock()

	summary := &AnalyticsSummary{
		ShareCount:  len(ae.logs),
		GeneratedAt: time.Now(),
	}

	totalViews := 0
	totalDownloads := 0
	totalUniqueVisitors := 0

	for shareID, logs := range ae.logs {
		for _, log := range logs {
			if log.Action == "view" {
				totalViews++
			}
			if log.Action == "download" {
				totalDownloads++
			}
		}
		totalUniqueVisitors += len(ae.visitors[shareID])
	}

	summary.TotalViews = totalViews
	summary.TotalDownloads = totalDownloads
	summary.TotalUniqueVisitors = totalUniqueVisitors

	if summary.ShareCount > 0 {
		summary.AvgViewsPerShare = float64(totalViews) / float64(summary.ShareCount)
	}

	return summary
}

// AnalyticsSummary 统计摘要.
type AnalyticsSummary struct {
	ShareCount          int       `json:"share_count"`
	TotalViews          int       `json:"total_views"`
	TotalDownloads      int       `json:"total_downloads"`
	TotalUniqueVisitors int       `json:"total_unique_visitors"`
	AvgViewsPerShare    float64   `json:"avg_views_per_share"`
	GeneratedAt         time.Time `json:"generated_at"`
}

// DetectUserAgent 检测用户代理信息.
func DetectUserAgent(ua string) (deviceType DeviceType, os, browser string) {
	uaLower := strings.ToLower(ua)

	// 检测设备类型
	switch {
	case strings.Contains(uaLower, "mobile") || strings.Contains(uaLower, "android") && !strings.Contains(uaLower, "tablet"):
		deviceType = DeviceMobile
	case strings.Contains(uaLower, "tablet") || strings.Contains(uaLower, "ipad"):
		deviceType = DeviceTablet
	case strings.Contains(uaLower, "bot") || strings.Contains(uaLower, "crawler") || strings.Contains(uaLower, "spider"):
		deviceType = DeviceBot
	case strings.Contains(uaLower, "windows") || strings.Contains(uaLower, "macintosh") || strings.Contains(uaLower, "linux"):
		deviceType = DeviceDesktop
	default:
		deviceType = DeviceUnknown
	}

	// 检测操作系统（先检查移动设备，避免 iPhone UA 中的 Mac OS X 匹配错误）
	switch {
	case strings.Contains(uaLower, "android"):
		os = "Android"
	case strings.Contains(uaLower, "iphone") || strings.Contains(uaLower, "ipad"):
		os = "iOS"
	case strings.Contains(uaLower, "windows"):
		os = "Windows"
	case strings.Contains(uaLower, "macintosh") || strings.Contains(uaLower, "mac os"):
		os = "macOS"
	case strings.Contains(uaLower, "linux"):
		os = "Linux"
	default:
		os = "Unknown"
	}

	// 检测浏览器
	switch {
	case strings.Contains(uaLower, "edg/") || strings.Contains(uaLower, "edge"):
		browser = "Edge"
	case strings.Contains(uaLower, "chrome") && !strings.Contains(uaLower, "edg"):
		browser = "Chrome"
	case strings.Contains(uaLower, "firefox"):
		browser = "Firefox"
	case strings.Contains(uaLower, "safari") && !strings.Contains(uaLower, "chrome"):
		browser = "Safari"
	case strings.Contains(uaLower, "opera") || strings.Contains(uaLower, "opr"):
		browser = "Opera"
	default:
		browser = "Unknown"
	}

	return deviceType, os, browser
}

// CleanupLogs 清理过期日志.
func (ae *AnalyticsEngine) CleanupLogs(maxAge time.Duration) int {
	ae.mu.Lock()
	defer ae.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	totalRemoved := 0

	for shareID, logs := range ae.logs {
		validLogs := make([]*AccessLog, 0, len(logs))
		for _, log := range logs {
			if log.Timestamp.After(cutoff) {
				validLogs = append(validLogs, log)
			} else {
				totalRemoved++
			}
		}
		ae.logs[shareID] = validLogs
	}

	return totalRemoved
}
