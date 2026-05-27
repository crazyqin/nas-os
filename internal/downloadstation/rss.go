package downloadstation

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"
)

// RSSManager RSS 订阅管理器.
type RSSManager struct {
	mu        sync.RWMutex
	feeds     map[string]*RSSFeed
	feedItems map[string][]RSSItem
	manager   *Manager
	ctx       context.Context
	cancel    context.CancelFunc
}

// NewRSSManager 创建 RSS 管理器.
func NewRSSManager(manager *Manager) *RSSManager {
	ctx, cancel := context.WithCancel(context.Background())

	return &RSSManager{
		feeds:     make(map[string]*RSSFeed),
		feedItems: make(map[string][]RSSItem),
		manager:   manager,
		ctx:       ctx,
		cancel:    cancel,
	}
}

// AddFeed 添加 RSS 订阅.
func (rm *RSSManager) AddFeed(req AddRSSRequest) (*RSSFeed, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 验证 URL
	if req.URL == "" {
		return nil, fmt.Errorf("RSS URL 不能为空")
	}

	// 设置默认间隔
	interval := req.Interval
	if interval <= 0 {
		interval = 30 // 默认 30 分钟
	}

	feedID := uuid.New().String()
	now := time.Now()
	nextCheck := now.Add(time.Duration(interval) * time.Minute)

	feed := &RSSFeed{
		ID:        feedID,
		URL:       req.URL,
		Title:     req.Title,
		Enabled:   true,
		Interval:  interval,
		Filter:    req.Filter,
		CreatedAt: now,
		UpdatedAt: now,
		NextCheck: &nextCheck,
	}

	// 尝试获取 feed 信息
	if err := rm.fetchFeedInfo(feed); err != nil {
		// 获取失败不影响添加，只是标题可能为空
		if feed.Title == "" {
			feed.Title = req.URL
		}
	}

	rm.feeds[feedID] = feed
	return feed, nil
}

// GetFeed 获取 RSS 订阅.
func (rm *RSSManager) GetFeed(feedID string) (*RSSFeed, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	feed, ok := rm.feeds[feedID]
	if !ok {
		return nil, fmt.Errorf("feed not found: %s", feedID)
	}

	return feed, nil
}

// ListFeeds 列出所有 RSS 订阅.
func (rm *RSSManager) ListFeeds() []*RSSFeed {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	feeds := make([]*RSSFeed, 0, len(rm.feeds))
	for _, feed := range rm.feeds {
		feeds = append(feeds, feed)
	}

	return feeds
}

// UpdateFeed 更新 RSS 订阅.
func (rm *RSSManager) UpdateFeed(feedID string, req AddRSSRequest) (*RSSFeed, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	feed, ok := rm.feeds[feedID]
	if !ok {
		return nil, fmt.Errorf("feed not found: %s", feedID)
	}

	if req.URL != "" {
		feed.URL = req.URL
	}
	if req.Title != "" {
		feed.Title = req.Title
	}
	if req.Interval > 0 {
		feed.Interval = req.Interval
		nextCheck := time.Now().Add(time.Duration(req.Interval) * time.Minute)
		feed.NextCheck = &nextCheck
	}
	if req.Filter.AutoDownload {
		feed.Filter.AutoDownload = req.Filter.AutoDownload
	}
	if len(req.Filter.IncludePatterns) > 0 {
		feed.Filter.IncludePatterns = req.Filter.IncludePatterns
	}
	if len(req.Filter.ExcludePatterns) > 0 {
		feed.Filter.ExcludePatterns = req.Filter.ExcludePatterns
	}
	if req.Filter.MaxSize > 0 {
		feed.Filter.MaxSize = req.Filter.MaxSize
	}
	if req.Filter.MinSize > 0 {
		feed.Filter.MinSize = req.Filter.MinSize
	}
	if req.Filter.DownloadDir != "" {
		feed.Filter.DownloadDir = req.Filter.DownloadDir
	}

	feed.UpdatedAt = time.Now()

	return feed, nil
}

// DeleteFeed 删除 RSS 订阅.
func (rm *RSSManager) DeleteFeed(feedID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.feeds[feedID]; !ok {
		return fmt.Errorf("feed not found: %s", feedID)
	}

	delete(rm.feeds, feedID)
	delete(rm.feedItems, feedID)

	return nil
}

// EnableFeed 启用 RSS 订阅.
func (rm *RSSManager) EnableFeed(feedID string, enabled bool) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	feed, ok := rm.feeds[feedID]
	if !ok {
		return fmt.Errorf("feed not found: %s", feedID)
	}

	feed.Enabled = enabled
	feed.UpdatedAt = time.Now()

	return nil
}

// GetFeedItems 获取 RSS 条目.
func (rm *RSSManager) GetFeedItems(feedID string) ([]RSSItem, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	if _, ok := rm.feeds[feedID]; !ok {
		return nil, fmt.Errorf("feed not found: %s", feedID)
	}

	items := rm.feedItems[feedID]
	if items == nil {
		items = make([]RSSItem, 0)
	}

	return items, nil
}

// RefreshFeed 刷新 RSS 订阅.
func (rm *RSSManager) RefreshFeed(feedID string) ([]RSSItem, error) {
	rm.mu.Lock()
	feed, ok := rm.feeds[feedID]
	if !ok {
		rm.mu.Unlock()
		return nil, fmt.Errorf("feed not found: %s", feedID)
	}
	rm.mu.Unlock()

	// 获取 RSS 内容
	items, err := rm.fetchFeed(feed.URL)
	if err != nil {
		return nil, fmt.Errorf("获取 RSS 失败: %w", err)
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now()
	feed.LastCheck = &now
	nextCheck := now.Add(time.Duration(feed.Interval) * time.Minute)
	feed.NextCheck = &nextCheck
	feed.UpdatedAt = now

	// 过滤并保存条目
	filteredItems := rm.filterItems(feed, items)
	rm.feedItems[feedID] = filteredItems

	// 如果启用了自动下载，创建下载任务
	if feed.Filter.AutoDownload {
		rm.autoDownload(feed, filteredItems)
	}

	return filteredItems, nil
}

// CheckAllFeeds 检查所有需要更新的订阅.
func (rm *RSSManager) CheckAllFeeds() {
	rm.mu.RLock()
	feeds := make([]*RSSFeed, 0)
	for _, feed := range rm.feeds {
		if feed.Enabled && feed.NextCheck != nil && time.Now().After(*feed.NextCheck) {
			feeds = append(feeds, feed)
		}
	}
	rm.mu.RUnlock()

	for _, feed := range feeds {
		_, _ = rm.RefreshFeed(feed.ID)
	}
}

// StartAutoCheck 启动自动检查.
func (rm *RSSManager) StartAutoCheck() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-rm.ctx.Done():
				return
			case <-ticker.C:
				rm.CheckAllFeeds()
			}
		}
	}()
}

// Stop 停止 RSS 管理器.
func (rm *RSSManager) Stop() {
	rm.cancel()
}

// fetchFeedInfo 获取 RSS feed 信息.
func (rm *RSSManager) fetchFeedInfo(feed *RSSFeed) error {
	resp, err := http.Get(feed.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// 尝试解析 RSS
	var rss RSS
	if err := xml.Unmarshal(body, &rss); err == nil {
		feed.Title = rss.Channel.Title
		feed.Description = rss.Channel.Description
		return nil
	}

	// 尝试解析 Atom
	var atom Atom
	if err := xml.Unmarshal(body, &atom); err == nil {
		feed.Title = atom.Title
		feed.Description = atom.Subtitle
		return nil
	}

	return fmt.Errorf("无法解析 RSS feed")
}

// fetchFeed 获取 RSS feed 内容.
func (rm *RSSManager) fetchFeed(feedURL string) ([]RSSItem, error) {
	resp, err := http.Get(feedURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 尝试解析 RSS
	var rss RSS
	if err := xml.Unmarshal(body, &rss); err == nil {
		items := make([]RSSItem, 0, len(rss.Channel.Items))
		for _, item := range rss.Channel.Items {
			rssItem := RSSItem{
				Title:       item.Title,
				Link:        item.Link,
				Description: item.Description,
				Size:        item.Enclosure.Length,
				PubDate:     parseTime(item.PubDate),
			}
			items = append(items, rssItem)
		}
		return items, nil
	}

	// 尝试解析 Atom
	var atom Atom
	if err := xml.Unmarshal(body, &atom); err == nil {
		items := make([]RSSItem, 0, len(atom.Entries))
		for _, entry := range atom.Entries {
			rssItem := RSSItem{
				Title:       entry.Title,
				Link:        entry.Link.Href,
				Description: entry.Summary,
				PubDate:     parseTime(entry.Published),
			}
			items = append(items, rssItem)
		}
		return items, nil
	}

	return nil, fmt.Errorf("无法解析 RSS feed")
}

// filterItems 过滤 RSS 条目.
func (rm *RSSManager) filterItems(feed *RSSFeed, items []RSSItem) []RSSItem {
	filtered := make([]RSSItem, 0, len(items))

	for _, item := range items {
		// 检查包含模式
		if len(feed.Filter.IncludePatterns) > 0 {
			matched := false
			for _, pattern := range feed.Filter.IncludePatterns {
				if matchPattern(pattern, item.Title) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}

		// 检查排除模式
		excluded := false
		for _, pattern := range feed.Filter.ExcludePatterns {
			if matchPattern(pattern, item.Title) {
				excluded = true
				break
			}
		}
		if excluded {
			continue
		}

		// 检查文件大小
		if feed.Filter.MaxSize > 0 && item.Size > feed.Filter.MaxSize {
			continue
		}
		if feed.Filter.MinSize > 0 && item.Size < feed.Filter.MinSize {
			continue
		}

		filtered = append(filtered, item)
	}

	return filtered
}

// autoDownload 自动下载.
func (rm *RSSManager) autoDownload(feed *RSSFeed, items []RSSItem) {
	for i, item := range items {
		if item.Downloaded {
			continue
		}

		// 创建下载任务
		downloadDir := feed.Filter.DownloadDir
		if downloadDir == "" {
			downloadDir = rm.manager.downloadDir
		}

		req := CreateTaskRequest{
			URL:      item.Link,
			Name:     item.Title,
			FilePath: downloadDir,
		}

		task, err := rm.manager.CreateTask(req)
		if err != nil {
			continue
		}

		// 更新条目状态
		items[i].Downloaded = true
		items[i].TaskID = task.ID
	}
}

// matchPattern 匹配模式（支持正则表达式）.
func matchPattern(pattern, text string) bool {
	matched, err := regexp.MatchString(pattern, text)
	if err != nil {
		// 如果正则表达式无效，使用简单字符串匹配
		return contains(text, pattern)
	}
	return matched
}

// contains 检查字符串是否包含子串.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// parseTime 解析时间.
func parseTime(s string) time.Time {
	formats := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}

	return time.Time{}
}

// RSS 结构体.
type RSS struct {
	XMLName xml.Name   `xml:"rss"`
	Channel RSSChannel `xml:"channel"`
}

// RSSChannel RSS 频道.
type RSSChannel struct {
	Title       string       `xml:"title"`
	Description string       `xml:"description"`
	Link        string       `xml:"link"`
	Items       []RSSItemXML `xml:"item"`
}

// RSSItemXML RSS 条目 XML 结构.
type RSSItemXML struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Enclosure   struct {
		URL  string `xml:"url,attr"`
		Type string `xml:"type,attr"`
		Length int64 `xml:"length,attr"`
	} `xml:"enclosure"`
}

// Atom 结构体.
type Atom struct {
	XMLName  xml.Name    `xml:"feed"`
	Title    string      `xml:"title"`
	Subtitle string      `xml:"subtitle"`
	Entries  []AtomEntry `xml:"entry"`
}

// AtomEntry Atom 条目.
type AtomEntry struct {
	Title     string `xml:"title"`
	Link      struct {
		Href string `xml:"href,attr"`
	} `xml:"link"`
	Summary   string `xml:"summary"`
	Published string `xml:"published"`
}
