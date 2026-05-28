package personalportal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPortalEngine(t *testing.T) {
	pe := NewPortalEngine()
	assert.NotNil(t, pe)
	assert.NotNil(t, pe.portals)
	assert.NotNil(t, pe.feedConfigs)
	assert.NotNil(t, pe.notifications)
}

// ========== 门户管理测试 ==========

func TestCreatePortal(t *testing.T) {
	pe := NewPortalEngine()
	portal, err := pe.CreatePortal("user1", "我的门户", "个人门户", ThemeDark)
	require.NoError(t, err)
	assert.NotEmpty(t, portal.ID)
	assert.Equal(t, "user1", portal.UserID)
	assert.Equal(t, "我的门户", portal.Name)
	assert.Equal(t, ThemeDark, portal.Theme)
	assert.Equal(t, 4, portal.Layout.Columns)
	assert.Empty(t, portal.Widgets)
}

func TestGetPortal(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)

	got, err := pe.GetPortal(portal.ID)
	require.NoError(t, err)
	assert.Equal(t, portal.ID, got.ID)

	_, err = pe.GetPortal("not-exist")
	assert.ErrorIs(t, err, ErrPortalNotFound)
}

func TestGetUserPortals(t *testing.T) {
	pe := NewPortalEngine()
	pe.CreatePortal("user1", "门户1", "", ThemeLight)
	pe.CreatePortal("user1", "门户2", "", ThemeDark)
	pe.CreatePortal("user2", "门户3", "", ThemeLight)

	portals := pe.GetUserPortals("user1")
	assert.Len(t, portals, 2)

	portals = pe.GetUserPortals("user2")
	assert.Len(t, portals, 1)
}

func TestUpdatePortal(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)

	updated, err := pe.UpdatePortal(portal.ID, map[string]interface{}{
		"name":        "新门户名",
		"description": "新描述",
		"theme":       ThemeDark,
	})
	require.NoError(t, err)
	assert.Equal(t, "新门户名", updated.Name)
	assert.Equal(t, ThemeDark, updated.Theme)

	_, err = pe.UpdatePortal("not-exist", nil)
	assert.ErrorIs(t, err, ErrPortalNotFound)
}

func TestDeletePortal(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)

	err := pe.DeletePortal(portal.ID)
	require.NoError(t, err)

	_, err = pe.GetPortal(portal.ID)
	assert.ErrorIs(t, err, ErrPortalNotFound)

	err = pe.DeletePortal("not-exist")
	assert.ErrorIs(t, err, ErrPortalNotFound)
}

func TestClonePortal(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "原门户", "", ThemeLight)
	widget := &Widget{
		Type:     WidgetTypeClock,
		Title:    "时钟",
		Position: WidgetPosition{X: 0, Y: 0, W: 1, H: 1},
	}
	pe.AddWidget(portal.ID, widget)

	clone, err := pe.ClonePortal(portal.ID, "克隆门户")
	require.NoError(t, err)
	assert.NotEqual(t, portal.ID, clone.ID)
	assert.Equal(t, "克隆门户", clone.Name)
	assert.Len(t, clone.Widgets, 1)
	assert.NotEqual(t, portal.Widgets[0].ID, clone.Widgets[0].ID)
}

func TestSetDefaultPortal(t *testing.T) {
	pe := NewPortalEngine()
	portal1, _ := pe.CreatePortal("user1", "门户1", "", ThemeLight)
	portal2, _ := pe.CreatePortal("user1", "门户2", "", ThemeDark)

	err := pe.SetDefaultPortal(portal1.ID)
	require.NoError(t, err)

	got1, _ := pe.GetPortal(portal1.ID)
	got2, _ := pe.GetPortal(portal2.ID)
	assert.True(t, got1.IsDefault)
	assert.False(t, got2.IsDefault)

	// 切换默认
	pe.SetDefaultPortal(portal2.ID)
	got1, _ = pe.GetPortal(portal1.ID)
	got2, _ = pe.GetPortal(portal2.ID)
	assert.False(t, got1.IsDefault)
	assert.True(t, got2.IsDefault)
}

// ========== 小组件管理测试 ==========

func TestAddWidget(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)

	widget := &Widget{
		Type:  WidgetTypeWeather,
		Title: "天气",
		Position: WidgetPosition{
			X: 0, Y: 0, W: 2, H: 1,
		},
		Config: WidgetConfig{
			City:  "北京",
			Units: "metric",
		},
		RefreshRate: 1800,
	}

	added, err := pe.AddWidget(portal.ID, widget)
	require.NoError(t, err)
	assert.NotEmpty(t, added.ID)
	assert.Equal(t, WidgetTypeWeather, added.Type)
	assert.True(t, added.Enabled)

	got, _ := pe.GetPortal(portal.ID)
	assert.Len(t, got.Widgets, 1)
}

func TestGetWidget(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)
	widget := &Widget{
		Type:     WidgetTypeClock,
		Title:    "时钟",
		Position: WidgetPosition{X: 0, Y: 0, W: 1, H: 1},
	}
	added, _ := pe.AddWidget(portal.ID, widget)

	got, err := pe.GetWidget(portal.ID, added.ID)
	require.NoError(t, err)
	assert.Equal(t, added.ID, got.ID)

	_, err = pe.GetWidget(portal.ID, "not-exist")
	assert.ErrorIs(t, err, ErrWidgetNotFound)
}

func TestUpdateWidget(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)
	widget := &Widget{
		Type:     WidgetTypeClock,
		Title:    "时钟",
		Position: WidgetPosition{X: 0, Y: 0, W: 1, H: 1},
	}
	added, _ := pe.AddWidget(portal.ID, widget)

	updated, err := pe.UpdateWidget(portal.ID, &Widget{
		ID:       added.ID,
		Type:     WidgetTypeClock,
		Title:    "新时钟",
		Position: added.Position,
	})
	require.NoError(t, err)
	assert.Equal(t, "新时钟", updated.Title)
}

func TestRemoveWidget(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)
	widget := &Widget{
		Type:     WidgetTypeClock,
		Title:    "时钟",
		Position: WidgetPosition{X: 0, Y: 0, W: 1, H: 1},
	}
	added, _ := pe.AddWidget(portal.ID, widget)

	err := pe.RemoveWidget(portal.ID, added.ID)
	require.NoError(t, err)

	got, _ := pe.GetPortal(portal.ID)
	assert.Empty(t, got.Widgets)

	err = pe.RemoveWidget(portal.ID, "not-exist")
	assert.ErrorIs(t, err, ErrWidgetNotFound)
}

func TestMoveWidget(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)
	widget := &Widget{
		Type:     WidgetTypeClock,
		Title:    "时钟",
		Position: WidgetPosition{X: 0, Y: 0, W: 1, H: 1},
	}
	added, _ := pe.AddWidget(portal.ID, widget)

	err := pe.MoveWidget(portal.ID, added.ID, WidgetPosition{X: 2, Y: 0, W: 1, H: 1})
	require.NoError(t, err)

	got, _ := pe.GetWidget(portal.ID, added.ID)
	assert.Equal(t, 2, got.Position.X)
}

func TestWidgetOverlap(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)

	widget1 := &Widget{
		Type:     WidgetTypeClock,
		Title:    "时钟1",
		Position: WidgetPosition{X: 0, Y: 0, W: 2, H: 1},
	}
	_, err := pe.AddWidget(portal.ID, widget1)
	require.NoError(t, err)

	// 重叠位置
	widget2 := &Widget{
		Type:     WidgetTypeClock,
		Title:    "时钟2",
		Position: WidgetPosition{X: 1, Y: 0, W: 2, H: 1},
	}
	_, err = pe.AddWidget(portal.ID, widget2)
	assert.ErrorIs(t, err, ErrWidgetOverlap)

	// 不重叠
	widget3 := &Widget{
		Type:     WidgetTypeClock,
		Title:    "时钟3",
		Position: WidgetPosition{X: 2, Y: 0, W: 1, H: 1},
	}
	_, err = pe.AddWidget(portal.ID, widget3)
	require.NoError(t, err)
}

func TestToggleWidget(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)
	widget := &Widget{
		Type:     WidgetTypeClock,
		Title:    "时钟",
		Position: WidgetPosition{X: 0, Y: 0, W: 1, H: 1},
	}
	added, _ := pe.AddWidget(portal.ID, widget)

	err := pe.ToggleWidget(portal.ID, added.ID)
	require.NoError(t, err)

	got, _ := pe.GetWidget(portal.ID, added.ID)
	assert.False(t, got.Enabled)
}

// ========== 布局管理测试 ==========

func TestUpdateLayout(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)

	err := pe.UpdateLayout(portal.ID, Layout{
		Columns:   6,
		RowHeight: 100,
		Gap:       20,
	})
	require.NoError(t, err)

	got, _ := pe.GetPortal(portal.ID)
	assert.Equal(t, 6, got.Layout.Columns)
	assert.Equal(t, 100, got.Layout.RowHeight)
}

func TestAutoLayout(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)

	pe.AddWidget(portal.ID, &Widget{
		Type:     WidgetTypeClock,
		Title:    "时钟",
		Position: WidgetPosition{X: 0, Y: 0, W: 1, H: 1},
	})
	pe.AddWidget(portal.ID, &Widget{
		Type:     WidgetTypeWeather,
		Title:    "天气",
		Position: WidgetPosition{X: 99, Y: 99, W: 2, H: 1},
	})
	pe.AddWidget(portal.ID, &Widget{
		Type:     WidgetTypeTodo,
		Title:    "待办",
		Position: WidgetPosition{X: 98, Y: 98, W: 1, H: 2},
	})

	err := pe.AutoLayout(portal.ID)
	require.NoError(t, err)

	widgets, _ := pe.ListWidgets(portal.ID)
	assert.Len(t, widgets, 3)

	// 验证位置已更新
	assert.Equal(t, 0, widgets[0].Position.X)
	assert.Equal(t, 1, widgets[1].Position.X)
}

// ========== 用户偏好测试 ==========

func TestPreferences(t *testing.T) {
	t.Run("GetDefaultPreferences", func(t *testing.T) {
		pe := NewPortalEngine()
		prefs, err := pe.GetPreferences("user1")
		require.NoError(t, err)
		assert.Equal(t, ThemeAuto, prefs.Theme)
		assert.Equal(t, "zh-CN", prefs.Language)
	})

	t.Run("UpdatePreferences", func(t *testing.T) {
		pe := NewPortalEngine()
		prefs, err := pe.UpdatePreferences("user1", &UserPreferences{
			Theme:    ThemeDark,
			Language: "en-US",
		})
		require.NoError(t, err)
		assert.Equal(t, ThemeDark, prefs.Theme)

		got, _ := pe.GetPreferences("user1")
		assert.Equal(t, ThemeDark, got.Theme)
	})
}

// ========== 通知管理测试 ==========

func TestNotifications(t *testing.T) {
	t.Run("AddNotification", func(t *testing.T) {
		pe := NewPortalEngine()
		err := pe.AddNotification("user1", &Notification{
			Title:   "测试通知",
			Message: "这是一条测试通知",
			Type:    "info",
			Source:  "system",
		})
		require.NoError(t, err)
	})

	t.Run("ListNotifications", func(t *testing.T) {
		pe := NewPortalEngine()
		pe.AddNotification("user1", &Notification{Title: "通知1", Type: "info"})
		pe.AddNotification("user1", &Notification{Title: "通知2", Type: "warning"})

		notifications := pe.ListNotifications("user1", false)
		assert.Len(t, notifications, 2)
	})

	t.Run("UnreadNotifications", func(t *testing.T) {
		pe := NewPortalEngine()
		pe.AddNotification("user1", &Notification{Title: "通知1", Type: "info"})
		pe.AddNotification("user1", &Notification{Title: "通知2", Type: "info"})

		assert.Equal(t, 2, pe.GetUnreadNotificationCount("user1"))

		notifications := pe.ListNotifications("user1", false)
		pe.MarkNotificationRead("user1", notifications[0].ID)

		assert.Equal(t, 1, pe.GetUnreadNotificationCount("user1"))
	})

	t.Run("MarkAllRead", func(t *testing.T) {
		pe := NewPortalEngine()
		pe.AddNotification("user1", &Notification{Title: "通知1", Type: "info"})
		pe.AddNotification("user1", &Notification{Title: "通知2", Type: "info"})

		pe.MarkAllNotificationsRead("user1")
		assert.Equal(t, 0, pe.GetUnreadNotificationCount("user1"))
	})
}

// ========== 统计测试 ==========

func TestPortalStats(t *testing.T) {
	pe := NewPortalEngine()
	portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)

	pe.AddWidget(portal.ID, &Widget{
		Type:     WidgetTypeClock,
		Title:    "时钟",
		Position: WidgetPosition{X: 0, Y: 0, W: 1, H: 1},
	})
	pe.AddWidget(portal.ID, &Widget{
		Type:     WidgetTypeWeather,
		Title:    "天气",
		Position: WidgetPosition{X: 1, Y: 0, W: 1, H: 1},
	})

	stats, err := pe.GetPortalStats(portal.ID)
	require.NoError(t, err)
	assert.Equal(t, 2, stats.WidgetCount)
}

// ========== 小组件管理器测试 ==========

func TestWidgetManager(t *testing.T) {
	t.Run("CreateWidgets", func(t *testing.T) {
		pe := NewPortalEngine()
		portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)
		wm := NewWidgetManager(pe)

		widget, err := wm.CreateWeatherWidget(portal.ID, "北京", WidgetPosition{X: 0, Y: 0, W: 2, H: 1})
		require.NoError(t, err)
		assert.Equal(t, WidgetTypeWeather, widget.Type)
		assert.Equal(t, "北京", widget.Config.City)

		widget, err = wm.CreateCalendarWidget(portal.ID, 7, WidgetPosition{X: 2, Y: 0, W: 2, H: 2})
		require.NoError(t, err)
		assert.Equal(t, WidgetTypeCalendar, widget.Type)

		widget, err = wm.CreateTodoWidget(portal.ID, "list1", 5, WidgetPosition{X: 0, Y: 1, W: 1, H: 2})
		require.NoError(t, err)
		assert.Equal(t, WidgetTypeTodo, widget.Type)

		widget, err = wm.CreateHealthWidget(portal.ID, WidgetPosition{X: 1, Y: 1, W: 1, H: 1})
		require.NoError(t, err)
		assert.Equal(t, WidgetTypeHealth, widget.Type)

		widget, err = wm.CreateFinanceWidget(portal.ID, "CNY", WidgetPosition{X: 2, Y: 2, W: 2, H: 1})
		require.NoError(t, err)
		assert.Equal(t, WidgetTypeFinance, widget.Type)
	})

	t.Run("RefreshWidgetData", func(t *testing.T) {
		pe := NewPortalEngine()
		portal, _ := pe.CreatePortal("user1", "我的门户", "", ThemeLight)
		wm := NewWidgetManager(pe)

		widget, _ := wm.CreateWeatherWidget(portal.ID, "上海", WidgetPosition{X: 0, Y: 0, W: 2, H: 1})
		data, err := wm.RefreshWidgetData(portal.ID, widget.ID)
		require.NoError(t, err)
		assert.NotNil(t, data)
	})

	t.Run("GetWidgetTypes", func(t *testing.T) {
		pe := NewPortalEngine()
		wm := NewWidgetManager(pe)

		types := wm.GetWidgetTypes()
		assert.Greater(t, len(types), 0)
	})

	t.Run("DefaultWidgetSize", func(t *testing.T) {
		size := GetDefaultWidgetSize(WidgetTypeWeather)
		assert.Equal(t, 2, size.W)
		assert.Equal(t, 1, size.H)

		size = GetDefaultWidgetSize(WidgetTypeCalendar)
		assert.Equal(t, 2, size.W)
		assert.Equal(t, 2, size.H)
	})
}

// ========== 信息流测试 ==========

func TestFeedAggregator(t *testing.T) {
	t.Run("AddFeedConfig", func(t *testing.T) {
		pe := NewPortalEngine()
		fa := NewFeedAggregator(pe)

		config, err := fa.AddFeedConfig("user1", "技术博客", FeedSourceRSS, "https://example.com/feed", 900, 10)
		require.NoError(t, err)
		assert.NotEmpty(t, config.ID)
		assert.Equal(t, FeedSourceRSS, config.Source)
	})

	t.Run("ListFeedConfigs", func(t *testing.T) {
		pe := NewPortalEngine()
		fa := NewFeedAggregator(pe)

		fa.AddFeedConfig("user1", "RSS1", FeedSourceRSS, "https://example.com/1", 900, 10)
		fa.AddFeedConfig("user1", "RSS2", FeedSourceRSS, "https://example.com/2", 900, 10)
		fa.AddFeedConfig("user2", "RSS3", FeedSourceRSS, "https://example.com/3", 900, 10)

		configs := fa.ListFeedConfigs("user1")
		assert.Len(t, configs, 2)
	})

	t.Run("UpdateDeleteFeedConfig", func(t *testing.T) {
		pe := NewPortalEngine()
		fa := NewFeedAggregator(pe)

		config, _ := fa.AddFeedConfig("user1", "RSS1", FeedSourceRSS, "https://example.com", 900, 10)

		updated, err := fa.UpdateFeedConfig(config.ID, map[string]interface{}{
			"name":    "新名称",
			"enabled": false,
		})
		require.NoError(t, err)
		assert.Equal(t, "新名称", updated.Name)
		assert.False(t, updated.Enabled)

		err = fa.DeleteFeedConfig(config.ID)
		require.NoError(t, err)

		_, err = fa.GetFeedConfig(config.ID)
		assert.ErrorIs(t, err, ErrFeedConfigNotFound)
	})

	t.Run("FeedItems", func(t *testing.T) {
		pe := NewPortalEngine()
		fa := NewFeedAggregator(pe)

		fa.AddFeedItem("user1", &FeedItem{
			Source:  FeedSourceRSS,
			Title:   "文章1",
			Summary: "摘要1",
		})
		fa.AddFeedItem("user1", &FeedItem{
			Source:  FeedSourceEmail,
			Title:   "邮件1",
			Summary: "摘要2",
		})

		items := fa.GetAggregatedFeed("user1", 10)
		assert.Len(t, items, 2)

		// 按来源筛选
		rssItems := fa.GetFeedBySource("user1", FeedSourceRSS, 10)
		assert.Len(t, rssItems, 1)
		assert.Equal(t, FeedSourceRSS, rssItems[0].Source)
	})

	t.Run("MarkFeedRead", func(t *testing.T) {
		pe := NewPortalEngine()
		fa := NewFeedAggregator(pe)

		fa.AddFeedItem("user1", &FeedItem{Source: FeedSourceRSS, Title: "文章1"})
		fa.AddFeedItem("user1", &FeedItem{Source: FeedSourceRSS, Title: "文章2"})

		items := fa.GetAggregatedFeed("user1", 10)
		assert.Len(t, items, 2)

		fa.MarkFeedItemRead("user1", items[0].ID)

		counts := fa.GetUnreadFeedCount("user1")
		assert.Equal(t, 1, counts[FeedSourceRSS])
	})

	t.Run("SearchFeedItems", func(t *testing.T) {
		pe := NewPortalEngine()
		fa := NewFeedAggregator(pe)

		fa.AddFeedItem("user1", &FeedItem{Source: FeedSourceRSS, Title: "Go 语言教程"})
		fa.AddFeedItem("user1", &FeedItem{Source: FeedSourceRSS, Title: "Python 入门"})

		results := fa.SearchFeedItems("user1", "Go", 10)
		assert.Len(t, results, 1)
		assert.Contains(t, results[0].Title, "Go")
	})

	t.Run("CleanupOldFeedItems", func(t *testing.T) {
		pe := NewPortalEngine()
		fa := NewFeedAggregator(pe)

		// 添加新旧项目
		fa.AddFeedItem("user1", &FeedItem{Source: FeedSourceRSS, Title: "新文章"})
		fa.engine.mu.Lock()
		fa.engine.feedItems["user1"] = append(fa.engine.feedItems["user1"], &FeedItem{
			ID:        "old1",
			Source:    FeedSourceRSS,
			Title:     "旧文章",
			Timestamp: time.Now().Add(-48 * time.Hour),
		})
		fa.engine.mu.Unlock()

		removed := fa.CleanupOldFeedItems("user1", 24*time.Hour)
		assert.Equal(t, 1, removed)

		items := fa.GetAggregatedFeed("user1", 10)
		assert.Len(t, items, 1)
	})

	t.Run("MockFeedData", func(t *testing.T) {
		pe := NewPortalEngine()
		fa := NewFeedAggregator(pe)

		fa.AddMockFeedData("user1")

		items := fa.GetAggregatedFeed("user1", 10)
		assert.Greater(t, len(items), 0)

		stats := fa.GetFeedStats("user1")
		assert.Greater(t, stats["total"].(int), 0)
	})
}
