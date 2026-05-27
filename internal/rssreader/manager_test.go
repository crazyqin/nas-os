// Package rssreader 单元测试
package rssreader

import (
	"encoding/xml"
	"testing"
)

func TestCreateFeed(t *testing.T) {
	m := NewManager()
	feed, err := m.CreateFeed(CreateFeedRequest{
		Title:   "测试订阅源",
		FeedURL: "https://example.com/feed.xml",
		Tags:    []string{"tech", "news"},
	})
	if err != nil {
		t.Fatalf("创建订阅源失败: %v", err)
	}
	if feed == nil {
		t.Fatal("订阅源不应为nil")
	}
	if feed.Title != "测试订阅源" {
		t.Errorf("标题不匹配: %s", feed.Title)
	}
	if feed.FeedURL != "https://example.com/feed.xml" {
		t.Errorf("URL不匹配: %s", feed.FeedURL)
	}
	if len(feed.Tags) != 2 {
		t.Errorf("标签数量不匹配: %d", len(feed.Tags))
	}
	if !feed.IsEnabled {
		t.Error("订阅源应默认启用")
	}
	if feed.FetchInterval != 30 {
		t.Errorf("默认抓取间隔应为30分钟，实际: %d", feed.FetchInterval)
	}
}

func TestCreateDuplicateFeed(t *testing.T) {
	m := NewManager()
	m.CreateFeed(CreateFeedRequest{
		Title:   "Feed 1",
		FeedURL: "https://example.com/feed.xml",
	})

	_, err := m.CreateFeed(CreateFeedRequest{
		Title:   "Feed 2",
		FeedURL: "https://example.com/feed.xml",
	})
	if err == nil {
		t.Error("重复URL应返回错误")
	}
}

func TestGetFeed(t *testing.T) {
	m := NewManager()
	feed, _ := m.CreateFeed(CreateFeedRequest{
		Title:   "test",
		FeedURL: "https://example.com/feed.xml",
	})

	got, err := m.GetFeed(feed.ID)
	if err != nil {
		t.Fatalf("获取订阅源失败: %v", err)
	}
	if got.Title != "test" {
		t.Errorf("标题不匹配")
	}
}

func TestGetFeedNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetFeed("nonexistent")
	if err == nil {
		t.Error("不存在的订阅源应返回错误")
	}
}

func TestUpdateFeed(t *testing.T) {
	m := NewManager()
	feed, _ := m.CreateFeed(CreateFeedRequest{
		Title:   "old",
		FeedURL: "https://example.com/feed.xml",
	})

	newTitle := "new"
	newInterval := 60
	updated, err := m.UpdateFeed(feed.ID, UpdateFeedRequest{
		Title:         &newTitle,
		FetchInterval: &newInterval,
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Title != "new" {
		t.Errorf("标题未更新: %s", updated.Title)
	}
	if updated.FetchInterval != 60 {
		t.Errorf("抓取间隔未更新: %d", updated.FetchInterval)
	}
}

func TestDeleteFeed(t *testing.T) {
	m := NewManager()
	feed, _ := m.CreateFeed(CreateFeedRequest{
		Title:   "to delete",
		FeedURL: "https://example.com/feed.xml",
	})

	err := m.DeleteFeed(feed.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	_, err = m.GetFeed(feed.ID)
	if err == nil {
		t.Error("已删除订阅源不应存在")
	}
}

func TestListFeeds(t *testing.T) {
	m := NewManager()
	m.CreateFeed(CreateFeedRequest{Title: "feed1", FeedURL: "https://example.com/1.xml"})
	m.CreateFeed(CreateFeedRequest{Title: "feed2", FeedURL: "https://example.com/2.xml"})
	m.CreateFeed(CreateFeedRequest{Title: "feed3", FeedURL: "https://example.com/3.xml"})

	feeds := m.ListFeeds()
	if len(feeds) != 3 {
		t.Errorf("期望3个订阅源，实际 %d", len(feeds))
	}
}

// ========== 文章测试 ==========

func TestCreateArticle(t *testing.T) {
	m := NewManager()
	feed, _ := m.CreateFeed(CreateFeedRequest{
		Title:   "test feed",
		FeedURL: "https://example.com/feed.xml",
	})

	// 直接创建文章（模拟抓取结果）
	article := &Article{
		ID:    "test-article-1",
		FeedID: feed.ID,
		Title: "测试文章",
		Link:  "https://example.com/article1",
		Content: "这是测试内容",
	}

	m.mu.Lock()
	m.articles[article.ID] = article
	m.articlesByLink[article.Link] = article.ID
	feed.ArticleCount++
	feed.UnreadCount++
	m.mu.Unlock()

	got, err := m.GetArticle(article.ID)
	if err != nil {
		t.Fatalf("获取文章失败: %v", err)
	}
	if got.Title != "测试文章" {
		t.Errorf("标题不匹配: %s", got.Title)
	}
}

func TestUpdateArticle(t *testing.T) {
	m := NewManager()
	feed, _ := m.CreateFeed(CreateFeedRequest{
		Title:   "test feed",
		FeedURL: "https://example.com/feed.xml",
	})

	article := &Article{
		ID:    "test-article-1",
		FeedID: feed.ID,
		Title: "test",
		IsRead: false,
	}
	m.mu.Lock()
	m.articles[article.ID] = article
	feed.UnreadCount = 1
	m.mu.Unlock()

	isRead := true
	isFav := true
	updated, err := m.UpdateArticle(article.ID, UpdateArticleRequest{
		IsRead:     &isRead,
		IsFavorite: &isFav,
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if !updated.IsRead {
		t.Error("文章应已标记为已读")
	}
	if !updated.IsFavorite {
		t.Error("文章应已收藏")
	}
	if feed.UnreadCount != 0 {
		t.Errorf("未读数应为0，实际: %d", feed.UnreadCount)
	}
}

func TestMarkFeedAsRead(t *testing.T) {
	m := NewManager()
	feed, _ := m.CreateFeed(CreateFeedRequest{
		Title:   "test feed",
		FeedURL: "https://example.com/feed.xml",
	})

	// 创建多篇文章
	m.mu.Lock()
	for i := 0; i < 5; i++ {
		article := &Article{
			ID:     "article-" + string(rune('0'+i)),
			FeedID: feed.ID,
			Title:  "Article " + string(rune('0'+i)),
			IsRead: false,
		}
		m.articles[article.ID] = article
	}
	feed.UnreadCount = 5
	m.mu.Unlock()

	err := m.MarkFeedAsRead(feed.ID)
	if err != nil {
		t.Fatalf("标记已读失败: %v", err)
	}

	if feed.UnreadCount != 0 {
		t.Errorf("未读数应为0，实际: %d", feed.UnreadCount)
	}
}

func TestSearchArticles(t *testing.T) {
	m := NewManager()
	feed, _ := m.CreateFeed(CreateFeedRequest{
		Title:   "test feed",
		FeedURL: "https://example.com/feed.xml",
	})

	m.mu.Lock()
	m.articles["a1"] = &Article{ID: "a1", FeedID: feed.ID, Title: "Go语言入门", Content: "学习Go编程"}
	m.articles["a2"] = &Article{ID: "a2", FeedID: feed.ID, Title: "Python教程", Content: "学习Python"}
	m.articles["a3"] = &Article{ID: "a3", FeedID: feed.ID, Title: "Go并发编程", Content: "Goroutine"}
	m.mu.Unlock()

	results := m.SearchArticles(SearchRequest{Query: "Go"})
	if len(results) != 2 {
		t.Errorf("搜索Go应有2个结果，实际 %d", len(results))
	}
}

// ========== 分类测试 ==========

func TestCreateCategory(t *testing.T) {
	m := NewManager()
	cat := m.CreateCategory(CreateCategoryRequest{
		Name:        "技术",
		Description: "技术相关",
	})
	if cat == nil {
		t.Fatal("分类不应为nil")
	}
	if cat.Name != "技术" {
		t.Errorf("名称不匹配: %s", cat.Name)
	}
}

func TestGetCategory(t *testing.T) {
	m := NewManager()
	cat := m.CreateCategory(CreateCategoryRequest{Name: "test"})

	got, err := m.GetCategory(cat.ID)
	if err != nil {
		t.Fatalf("获取分类失败: %v", err)
	}
	if got.Name != "test" {
		t.Errorf("名称不匹配")
	}
}

func TestUpdateCategory(t *testing.T) {
	m := NewManager()
	cat := m.CreateCategory(CreateCategoryRequest{Name: "old"})

	newName := "new"
	updated, err := m.UpdateCategory(cat.ID, UpdateCategoryRequest{Name: &newName})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != "new" {
		t.Errorf("名称未更新: %s", updated.Name)
	}
}

func TestDeleteCategory(t *testing.T) {
	m := NewManager()
	cat := m.CreateCategory(CreateCategoryRequest{Name: "to delete"})

	err := m.DeleteCategory(cat.ID)
	if err != nil {
		t.Fatalf("删除分类失败: %v", err)
	}
	_, err = m.GetCategory(cat.ID)
	if err == nil {
		t.Error("已删除分类不应存在")
	}
}

func TestListCategories(t *testing.T) {
	m := NewManager()
	m.CreateCategory(CreateCategoryRequest{Name: "cat1"})
	m.CreateCategory(CreateCategoryRequest{Name: "cat2"})

	cats := m.ListCategories()
	if len(cats) != 2 {
		t.Errorf("期望2个分类，实际 %d", len(cats))
	}
}

func TestCategoryFeedCount(t *testing.T) {
	m := NewManager()
	cat := m.CreateCategory(CreateCategoryRequest{Name: "tech"})

	m.CreateFeed(CreateFeedRequest{
		Title:      "feed1",
		FeedURL:    "https://example.com/1.xml",
		CategoryID: cat.ID,
	})
	m.CreateFeed(CreateFeedRequest{
		Title:      "feed2",
		FeedURL:    "https://example.com/2.xml",
		CategoryID: cat.ID,
	})

	got, _ := m.GetCategory(cat.ID)
	if got.FeedCount != 2 {
		t.Errorf("订阅源数应为2，实际: %d", got.FeedCount)
	}
}

// ========== 标签测试 ==========

func TestListFeedsByTag(t *testing.T) {
	m := NewManager()
	m.CreateFeed(CreateFeedRequest{Title: "f1", FeedURL: "https://example.com/1.xml", Tags: []string{"go", "backend"}})
	m.CreateFeed(CreateFeedRequest{Title: "f2", FeedURL: "https://example.com/2.xml", Tags: []string{"go", "frontend"}})
	m.CreateFeed(CreateFeedRequest{Title: "f3", FeedURL: "https://example.com/3.xml", Tags: []string{"python"}})

	goFeeds := m.ListFeedsByTag("go")
	if len(goFeeds) != 2 {
		t.Errorf("go标签应有2个订阅源，实际 %d", len(goFeeds))
	}
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	m := NewManager()
	feed, _ := m.CreateFeed(CreateFeedRequest{
		Title:   "test feed",
		FeedURL: "https://example.com/feed.xml",
	})

	m.mu.Lock()
	m.articles["a1"] = &Article{ID: "a1", FeedID: feed.ID, Title: "a1", IsRead: false}
	m.articles["a2"] = &Article{ID: "a2", FeedID: feed.ID, Title: "a2", IsRead: true}
	m.articles["a3"] = &Article{ID: "a3", FeedID: feed.ID, Title: "a3", IsRead: false, IsFavorite: true}
	feed.ArticleCount = 3
	feed.UnreadCount = 2
	m.mu.Unlock()

	stats := m.GetStats()
	if stats.TotalFeeds != 1 {
		t.Errorf("总订阅数应为1，实际: %d", stats.TotalFeeds)
	}
	if stats.TotalArticles != 3 {
		t.Errorf("总文章数应为3，实际: %d", stats.TotalArticles)
	}
	if stats.UnreadCount != 2 {
		t.Errorf("未读数应为2，实际: %d", stats.UnreadCount)
	}
	if stats.ReadCount != 1 {
		t.Errorf("已读数应为1，实际: %d", stats.ReadCount)
	}
	if stats.FavoriteCount != 1 {
		t.Errorf("收藏数应为1，实际: %d", stats.FavoriteCount)
	}
}

// ========== OPML测试 ==========

func TestImportOPML(t *testing.T) {
	m := NewManager()

	opmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<opml version="2.0">
  <head>
    <title>Test OPML</title>
  </head>
  <body>
    <outline text="Tech" title="Tech">
      <outline text="Hacker News" xmlUrl="https://hnrss.org/frontpage" htmlUrl="https://news.ycombinator.com"/>
      <outline text="Reddit" xmlUrl="https://www.reddit.com/.rss" htmlUrl="https://www.reddit.com"/>
    </outline>
    <outline text="Standalone" xmlUrl="https://example.com/feed.xml"/>
  </body>
</opml>`

	feeds, err := m.ImportOPML(opmlContent)
	if err != nil {
		t.Fatalf("导入OPML失败: %v", err)
	}
	if len(feeds) != 3 {
		t.Errorf("应导入3个订阅源，实际: %d", len(feeds))
	}
}

func TestExportOPML(t *testing.T) {
	m := NewManager()
	m.CreateFeed(CreateFeedRequest{Title: "Feed 1", FeedURL: "https://example.com/1.xml"})
	m.CreateFeed(CreateFeedRequest{Title: "Feed 2", FeedURL: "https://example.com/2.xml"})

	content := m.ExportOPML()
	if content == "" {
		t.Error("导出内容不应为空")
	}

	// 验证是有效的XML
	var doc OPMLDocument
	if err := xml.Unmarshal([]byte(content), &doc); err != nil {
		t.Errorf("导出的OPML不是有效XML: %v", err)
	}
}

// ========== 健康检测测试 ==========

func TestCheckFeedHealth(t *testing.T) {
	m := NewManager()
	feed, _ := m.CreateFeed(CreateFeedRequest{
		Title:   "test",
		FeedURL: "https://httpbin.org/status/200",
	})

	health, err := m.CheckFeedHealth(feed.ID)
	if err != nil {
		t.Fatalf("检测健康状态失败: %v", err)
	}
	if health == nil {
		t.Fatal("健康状态不应为nil")
	}
	if health.FeedID != feed.ID {
		t.Errorf("FeedID不匹配: %s", health.FeedID)
	}
}

func TestGetFeedHealth(t *testing.T) {
	m := NewManager()
	feed, _ := m.CreateFeed(CreateFeedRequest{
		Title:   "test",
		FeedURL: "https://example.com/feed.xml",
	})

	// 先检测一次
	m.CheckFeedHealth(feed.ID)

	health, err := m.GetFeedHealth(feed.ID)
	if err != nil {
		t.Fatalf("获取健康状态失败: %v", err)
	}
	if health == nil {
		t.Fatal("健康状态不应为nil")
	}
}

// ========== 分页测试 ==========

func TestListArticlesPagination(t *testing.T) {
	m := NewManager()
	feed, _ := m.CreateFeed(CreateFeedRequest{
		Title:   "test feed",
		FeedURL: "https://example.com/feed.xml",
	})

	// 创建10篇文章
	m.mu.Lock()
	for i := 0; i < 10; i++ {
		id := "article-" + string(rune('0'+i))
		m.articles[id] = &Article{
			ID:        id,
			FeedID:    feed.ID,
			Title:     "Article " + string(rune('0'+i)),
			IsRead:    i%2 == 0,
		}
	}
	feed.ArticleCount = 10
	feed.UnreadCount = 5
	m.mu.Unlock()

	// 测试分页
	page1 := m.ListArticles(ListArticlesRequest{Page: 1, PageSize: 3})
	if len(page1) != 3 {
		t.Errorf("第一页应有3篇文章，实际 %d", len(page1))
	}

	page2 := m.ListArticles(ListArticlesRequest{Page: 2, PageSize: 3})
	if len(page2) != 3 {
		t.Errorf("第二页应有3篇文章，实际 %d", len(page2))
	}

	// 测试过滤
	unread := m.ListArticles(ListArticlesRequest{IsRead: boolPtr(false)})
	if len(unread) != 5 {
		t.Errorf("未读文章应有5篇，实际 %d", len(unread))
	}
}

func boolPtr(b bool) *bool {
	return &b
}

// ========== 并发安全测试 ==========

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()

	// 并发创建订阅源
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(n int) {
			url := "https://example.com/feed" + string(rune('0'+n)) + ".xml"
			m.CreateFeed(CreateFeedRequest{
				Title:   "Feed " + string(rune('0'+n)),
				FeedURL: url,
			})
			done <- true
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < 10; i++ {
		<-done
	}

	feeds := m.ListFeeds()
	if len(feeds) != 10 {
		t.Errorf("期望10个订阅源，实际 %d", len(feeds))
	}
}
