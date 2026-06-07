// Package rssreader 提供 RSS 阅读器核心业务逻辑
package rssreader

import (
	"crypto/md5"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager RSS阅读器管理器.
type Manager struct {
	feeds          map[string]*Feed
	articles       map[string]*Article
	categories     map[string]*Category
	feedsByURL     map[string]string // feed_url -> feed_id
	articlesByGUID map[string]string // feed_id:guid -> article_id
	articlesByLink map[string]string // link -> article_id
	health         map[string]*FeedHealth
	mu             sync.RWMutex
	stopChan       chan struct{}
}

// NewManager 创建RSS阅读器管理器.
func NewManager() *Manager {
	m := &Manager{
		feeds:          make(map[string]*Feed),
		articles:       make(map[string]*Article),
		categories:     make(map[string]*Category),
		feedsByURL:     make(map[string]string),
		articlesByGUID: make(map[string]string),
		articlesByLink: make(map[string]string),
		health:         make(map[string]*FeedHealth),
		stopChan:       make(chan struct{}),
	}
	return m
}

// Start 启动自动抓取定时器.
func (m *Manager) Start() {
	go m.autoFetchLoop()
}

// Stop 停止自动抓取定时器.
func (m *Manager) Stop() {
	close(m.stopChan)
}

// ========== 订阅源管理 ==========

// CreateFeed 创建订阅源.
func (m *Manager) CreateFeed(req CreateFeedRequest) (*Feed, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查URL是否已存在
	if _, exists := m.feedsByURL[req.FeedURL]; exists {
		return nil, fmt.Errorf("订阅源URL已存在: %s", req.FeedURL)
	}

	// 默认抓取间隔30分钟
	interval := req.FetchInterval
	if interval <= 0 {
		interval = 30
	}

	now := time.Now()
	feed := &Feed{
		ID:            uuid.New().String(),
		Title:         req.Title,
		FeedURL:       req.FeedURL,
		CategoryID:    req.CategoryID,
		Tags:          req.Tags,
		IsEnabled:     true,
		FetchInterval: interval,
		ArticleCount:  0,
		UnreadCount:   0,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if feed.Tags == nil {
		feed.Tags = []string{}
	}

	m.feeds[feed.ID] = feed
	m.feedsByURL[feed.FeedURL] = feed.ID

	// 更新分类计数
	if feed.CategoryID != "" {
		if cat, ok := m.categories[feed.CategoryID]; ok {
			cat.FeedCount++
		}
	}

	// 立即尝试抓取
	go m.fetchFeed(feed.ID)

	return feed, nil
}

// GetFeed 获取订阅源.
func (m *Manager) GetFeed(id string) (*Feed, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	feed, ok := m.feeds[id]
	if !ok {
		return nil, ErrFeedNotFound
	}
	return feed, nil
}

// ListFeeds 列出所有订阅源.
func (m *Manager) ListFeeds() []*Feed {
	m.mu.RLock()
	defer m.mu.RUnlock()

	feeds := make([]*Feed, 0, len(m.feeds))
	for _, f := range m.feeds {
		feeds = append(feeds, f)
	}

	sort.Slice(feeds, func(i, j int) bool {
		return feeds[i].CreatedAt.After(feeds[j].CreatedAt)
	})

	return feeds
}

// UpdateFeed 更新订阅源.
func (m *Manager) UpdateFeed(id string, req UpdateFeedRequest) (*Feed, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	feed, ok := m.feeds[id]
	if !ok {
		return nil, ErrFeedNotFound
	}

	if req.Title != nil {
		feed.Title = *req.Title
	}
	if req.CategoryID != nil {
		// 更新旧分类计数
		if feed.CategoryID != "" {
			if oldCat, ok := m.categories[feed.CategoryID]; ok {
				oldCat.FeedCount--
			}
		}
		feed.CategoryID = *req.CategoryID
		// 更新新分类计数
		if feed.CategoryID != "" {
			if newCat, ok := m.categories[feed.CategoryID]; ok {
				newCat.FeedCount++
			}
		}
	}
	if req.Tags != nil {
		feed.Tags = req.Tags
	}
	if req.IsEnabled != nil {
		feed.IsEnabled = *req.IsEnabled
	}
	if req.FetchInterval != nil && *req.FetchInterval > 0 {
		feed.FetchInterval = *req.FetchInterval
	}

	feed.UpdatedAt = time.Now()
	return feed, nil
}

// DeleteFeed 删除订阅源及其所有文章.
func (m *Manager) DeleteFeed(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	feed, ok := m.feeds[id]
	if !ok {
		return ErrFeedNotFound
	}

	// 删除该订阅源的所有文章
	for aid, article := range m.articles {
		if article.FeedID == id {
			// 清理去重索引
			if article.GUID != "" {
				delete(m.articlesByGUID, id+":"+article.GUID)
			}
			if article.Link != "" {
				delete(m.articlesByLink, article.Link)
			}
			delete(m.articles, aid)
		}
	}

	// 更新分类计数
	if feed.CategoryID != "" {
		if cat, ok := m.categories[feed.CategoryID]; ok {
			cat.FeedCount--
		}
	}

	// 清理索引
	delete(m.feedsByURL, feed.FeedURL)
	delete(m.health, id)
	delete(m.feeds, id)

	return nil
}

// RefreshFeed 手动刷新订阅源.
func (m *Manager) RefreshFeed(id string) error {
	m.mu.RLock()
	_, ok := m.feeds[id]
	m.mu.RUnlock()

	if !ok {
		return ErrFeedNotFound
	}

	go m.fetchFeed(id)
	return nil
}

// ========== 文章管理 ==========

// GetArticle 获取文章.
func (m *Manager) GetArticle(id string) (*Article, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	article, ok := m.articles[id]
	if !ok {
		return nil, ErrArticleNotFound
	}
	return article, nil
}

// ListArticles 列出文章（支持过滤和分页）.
func (m *Manager) ListArticles(req ListArticlesRequest) []*Article {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Article

	for _, a := range m.articles {
		// 按订阅源过滤
		if req.FeedID != "" && a.FeedID != req.FeedID {
			continue
		}

		// 按分类过滤
		if req.CategoryID != "" {
			feed, ok := m.feeds[a.FeedID]
			if !ok || feed.CategoryID != req.CategoryID {
				continue
			}
		}

		// 按状态过滤
		if req.IsRead != nil && a.IsRead != *req.IsRead {
			continue
		}
		if req.IsFavorite != nil && a.IsFavorite != *req.IsFavorite {
			continue
		}
		if req.IsMarked != nil && a.IsMarked != *req.IsMarked {
			continue
		}

		result = append(result, a)
	}

	// 按发布时间排序（最新在前）
	sort.Slice(result, func(i, j int) bool {
		return result[i].PublishedAt.After(result[j].PublishedAt)
	})

	// 分页处理
	page := req.Page
	pageSize := req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	start := (page - 1) * pageSize
	if start >= len(result) {
		return []*Article{}
	}

	end := start + pageSize
	if end > len(result) {
		end = len(result)
	}

	return result[start:end]
}

// UpdateArticle 更新文章状态.
func (m *Manager) UpdateArticle(id string, req UpdateArticleRequest) (*Article, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	article, ok := m.articles[id]
	if !ok {
		return nil, ErrArticleNotFound
	}

	// 更新未读计数
	feed := m.feeds[article.FeedID]

	if req.IsRead != nil {
		if article.IsRead && !*req.IsRead && feed != nil {
			feed.UnreadCount++
		} else if !article.IsRead && *req.IsRead && feed != nil {
			feed.UnreadCount--
		}
		article.IsRead = *req.IsRead
	}
	if req.IsFavorite != nil {
		article.IsFavorite = *req.IsFavorite
	}
	if req.IsMarked != nil {
		article.IsMarked = *req.IsMarked
	}

	article.UpdatedAt = time.Now()
	return article, nil
}

// MarkFeedAsRead 标记订阅源所有文章为已读.
func (m *Manager) MarkFeedAsRead(feedID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	feed, ok := m.feeds[feedID]
	if !ok {
		return ErrFeedNotFound
	}

	for _, article := range m.articles {
		if article.FeedID == feedID && !article.IsRead {
			article.IsRead = true
			article.UpdatedAt = time.Now()
		}
	}

	feed.UnreadCount = 0
	return nil
}

// MarkAllAsRead 标记所有文章为已读.
func (m *Manager) MarkAllAsRead() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, article := range m.articles {
		if !article.IsRead {
			article.IsRead = true
			article.UpdatedAt = time.Now()
		}
	}

	for _, feed := range m.feeds {
		feed.UnreadCount = 0
	}
}

// ========== 分类管理 ==========

// CreateCategory 创建分类.
func (m *Manager) CreateCategory(req CreateCategoryRequest) *Category {
	m.mu.Lock()
	defer m.mu.Unlock()

	cat := &Category{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		ParentID:    req.ParentID,
		FeedCount:   0,
		CreatedAt:   time.Now(),
	}

	m.categories[cat.ID] = cat
	return cat
}

// GetCategory 获取分类.
func (m *Manager) GetCategory(id string) (*Category, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cat, ok := m.categories[id]
	if !ok {
		return nil, ErrCategoryNotFound
	}
	return cat, nil
}

// ListCategories 列出所有分类.
func (m *Manager) ListCategories() []*Category {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cats := make([]*Category, 0, len(m.categories))
	for _, c := range m.categories {
		cats = append(cats, c)
	}

	sort.Slice(cats, func(i, j int) bool {
		return cats[i].CreatedAt.After(cats[j].CreatedAt)
	})

	return cats
}

// UpdateCategory 更新分类.
func (m *Manager) UpdateCategory(id string, req UpdateCategoryRequest) (*Category, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	cat, ok := m.categories[id]
	if !ok {
		return nil, ErrCategoryNotFound
	}

	if req.Name != nil {
		cat.Name = *req.Name
	}
	if req.Description != nil {
		cat.Description = *req.Description
	}
	if req.ParentID != nil {
		cat.ParentID = *req.ParentID
	}

	return cat, nil
}

// DeleteCategory 删除分类.
func (m *Manager) DeleteCategory(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cat, ok := m.categories[id]
	if !ok {
		return ErrCategoryNotFound
	}

	// 将该分类下的订阅源移出
	for _, feed := range m.feeds {
		if feed.CategoryID == id {
			feed.CategoryID = ""
		}
	}

	// 删除子分类
	for cid, child := range m.categories {
		if child.ParentID == id {
			child.ParentID = cat.ParentID
			_ = cid
		}
	}

	delete(m.categories, id)
	return nil
}

// ========== 标签管理 ==========

// ListFeedsByTag 按标签过滤订阅源.
func (m *Manager) ListFeedsByTag(tag string) []*Feed {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Feed
	for _, f := range m.feeds {
		for _, t := range f.Tags {
			if t == tag {
				result = append(result, f)
				break
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// ========== 搜索 ==========

// SearchArticles 全文搜索文章.
func (m *Manager) SearchArticles(req SearchRequest) []*Article {
	m.mu.RLock()
	defer m.mu.RUnlock()

	q := strings.ToLower(req.Query)
	var result []*Article

	for _, a := range m.articles {
		// 按订阅源过滤
		if req.FeedID != "" && a.FeedID != req.FeedID {
			continue
		}

		// 按状态过滤
		if req.IsRead != nil && a.IsRead != *req.IsRead {
			continue
		}
		if req.IsFavorite != nil && a.IsFavorite != *req.IsFavorite {
			continue
		}

		// 搜索标题、内容、摘要
		if strings.Contains(strings.ToLower(a.Title), q) ||
			strings.Contains(strings.ToLower(a.Content), q) ||
			strings.Contains(strings.ToLower(a.Summary), q) {
			result = append(result, a)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].PublishedAt.After(result[j].PublishedAt)
	})

	return result
}

// ========== OPML 导入/导出 ==========

// ImportOPML 导入OPML文件.
func (m *Manager) ImportOPML(content string) ([]*Feed, error) {
	var doc OPMLDocument
	if err := xml.Unmarshal([]byte(content), &doc); err != nil {
		return nil, fmt.Errorf("解析OPML失败: %w", err)
	}

	var imported []*Feed

	for _, outline := range doc.Body.Outlines {
		feeds := m.importOPMLOutline(outline, "")
		imported = append(imported, feeds...)
	}

	return imported, nil
}

// importOPMLOutline 递归导入OPML条目.
func (m *Manager) importOPMLOutline(outline OPMLOutline, parentCategoryID string) []*Feed {
	var result []*Feed

	// 如果有XMLURL，说明是订阅源
	if outline.XMLURL != "" {
		req := CreateFeedRequest{
			Title:      outline.Text,
			FeedURL:    outline.XMLURL,
			CategoryID: parentCategoryID,
		}
		if req.Title == "" {
			req.Title = outline.Title
		}
		feed, err := m.CreateFeed(req)
		if err == nil {
			result = append(result, feed)
		}
		return result
	}

	// 否则作为分类处理
	cat := m.CreateCategory(CreateCategoryRequest{
		Name:        outline.Text,
		Description: outline.Description,
		ParentID:    parentCategoryID,
	})

	// 递归处理子条目
	for _, child := range outline.Outlines {
		feeds := m.importOPMLOutline(child, cat.ID)
		result = append(result, feeds...)
	}

	return result
}

// ExportOPML 导出OPML文件.
func (m *Manager) ExportOPML() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc := OPMLDocument{
		Head: OPMLHead{
			Title:       "RSS Reader Export",
			DateCreated: time.Now().Format(time.RFC1123Z),
		},
	}

	// 按分类组织
	categoryFeeds := make(map[string][]*Feed)
	for _, feed := range m.feeds {
		categoryFeeds[feed.CategoryID] = append(categoryFeeds[feed.CategoryID], feed)
	}

	// 生成分类条目
	for catID, feeds := range categoryFeeds {
		if catID == "" {
			// 无分类的订阅源
			for _, feed := range feeds {
				doc.Body.Outlines = append(doc.Body.Outlines, OPMLOutline{
					Text:    feed.Title,
					Type:    "rss",
					XMLURL:  feed.FeedURL,
					HTMLURL: feed.SiteURL,
				})
			}
		} else {
			// 有分类的订阅源
			cat, ok := m.categories[catID]
			if !ok {
				continue
			}
			outline := OPMLOutline{
				Text: cat.Name,
			}
			for _, feed := range feeds {
				outline.Outlines = append(outline.Outlines, OPMLOutline{
					Text:    feed.Title,
					Type:    "rss",
					XMLURL:  feed.FeedURL,
					HTMLURL: feed.SiteURL,
				})
			}
			doc.Body.Outlines = append(doc.Body.Outlines, outline)
		}
	}

	data, _ := xml.MarshalIndent(doc, "", "  ")
	return xml.Header + string(data)
}

// ========== 健康检测 ==========

// CheckFeedHealth 检测订阅源健康状态.
func (m *Manager) CheckFeedHealth(id string) (*FeedHealth, error) {
	m.mu.RLock()
	feed, ok := m.feeds[id]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrFeedNotFound
	}

	start := time.Now()
	resp, err := http.Get(feed.FeedURL)
	responseTime := time.Since(start).Milliseconds()

	health := &FeedHealth{
		FeedID:        id,
		LastCheckedAt: time.Now(),
		ResponseTime:  responseTime,
	}

	if err != nil {
		health.IsReachable = false
		health.ErrorMessage = err.Error()
	} else {
		defer resp.Body.Close()
		health.IsReachable = resp.StatusCode >= 200 && resp.StatusCode < 400
		if !health.IsReachable {
			health.ErrorMessage = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
	}

	m.mu.Lock()
	m.health[id] = health
	m.mu.Unlock()

	return health, nil
}

// GetFeedHealth 获取订阅源健康状态.
func (m *Manager) GetFeedHealth(id string) (*FeedHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health, ok := m.health[id]
	if !ok {
		return nil, fmt.Errorf("未找到订阅源 %s 的健康状态", id)
	}
	return health, nil
}

// ========== 统计信息 ==========

// GetStats 获取统计信息.
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		TotalFeeds: len(m.feeds),
	}

	for _, a := range m.articles {
		stats.TotalArticles++
		if a.IsRead {
			stats.ReadCount++
		} else {
			stats.UnreadCount++
		}
		if a.IsFavorite {
			stats.FavoriteCount++
		}
		if a.IsMarked {
			stats.MarkedCount++
		}
	}

	// 订阅源统计
	stats.FeedStats = make([]FeedStat, 0, len(m.feeds))
	for _, feed := range m.feeds {
		stats.FeedStats = append(stats.FeedStats, FeedStat{
			FeedID:      feed.ID,
			FeedTitle:   feed.Title,
			TotalCount:  feed.ArticleCount,
			UnreadCount: feed.UnreadCount,
		})
	}

	return stats
}

// ========== 内部方法 ==========

// fetchFeed 抓取订阅源.
func (m *Manager) fetchFeed(feedID string) {
	m.mu.RLock()
	feed, ok := m.feeds[feedID]
	m.mu.RUnlock()

	if !ok || !feed.IsEnabled {
		return
	}

	resp, err := http.Get(feed.FeedURL)
	if err != nil {
		m.mu.Lock()
		feed.LastError = err.Error()
		now := time.Now()
		feed.LastFetchedAt = &now
		m.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		m.mu.Lock()
		feed.LastError = err.Error()
		now := time.Now()
		feed.LastFetchedAt = &now
		m.mu.Unlock()
		return
	}

	// 简单解析RSS/Atom（实际项目中应使用专门的RSS解析库）
	articles := m.parseFeedContent(feedID, body)

	m.mu.Lock()
	for _, article := range articles {
		// 去重检查
		dupKey := ""
		if article.GUID != "" {
			dupKey = feedID + ":" + article.GUID
			if _, exists := m.articlesByGUID[dupKey]; exists {
				continue
			}
		} else if article.Link != "" {
			if _, exists := m.articlesByLink[article.Link]; exists {
				continue
			}
		}

		// 生成文章ID并存储
		article.ID = uuid.New().String()
		article.FeedID = feedID
		article.CreatedAt = time.Now()
		article.UpdatedAt = time.Now()
		m.articles[article.ID] = article

		// 更新索引
		if article.GUID != "" {
			m.articlesByGUID[feedID+":"+article.GUID] = article.ID
		}
		if article.Link != "" {
			m.articlesByLink[article.Link] = article.ID
		}

		// 更新计数
		feed.ArticleCount++
		feed.UnreadCount++
	}

	feed.LastError = ""
	now := time.Now()
	feed.LastFetchedAt = &now
	feed.UpdatedAt = now
	m.mu.Unlock()
}

// parseFeedContent 解析RSS/Atom内容.
func (m *Manager) parseFeedContent(feedID string, content []byte) []*Article {
	// 简单实现：尝试解析RSS 2.0格式
	// 实际项目中应使用 github.com/mmcdole/gofeed 等库
	var result []*Article

	// 这里返回空切片，实际实现需要解析XML
	// 为了演示，我们创建一些示例文章
	_ = content
	_ = feedID

	return result
}

// autoFetchLoop 自动抓取循环.
func (m *Manager) autoFetchLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.autoFetchAll()
		}
	}
}

// autoFetchAll 自动抓取所有需要更新的订阅源.
func (m *Manager) autoFetchAll() {
	m.mu.RLock()
	var toFetch []string
	now := time.Now()

	for _, feed := range m.feeds {
		if !feed.IsEnabled {
			continue
		}

		// 检查是否需要抓取
		if feed.LastFetchedAt == nil {
			toFetch = append(toFetch, feed.ID)
			continue
		}

		elapsed := now.Sub(*feed.LastFetchedAt)
		if elapsed >= time.Duration(feed.FetchInterval)*time.Minute {
			toFetch = append(toFetch, feed.ID)
		}
	}
	m.mu.RUnlock()

	// 逐个抓取（避免并发过多）
	for _, feedID := range toFetch {
		m.fetchFeed(feedID)
	}
}

// generateContentHash 生成内容哈希用于去重.
func generateContentHash(content string) string {
	hash := md5.Sum([]byte(content))
	return fmt.Sprintf("%x", hash)
}
