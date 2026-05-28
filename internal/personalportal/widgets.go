// Package personalportal 提供个人门户仪表盘功能。
// widgets.go 实现小组件系统，包括天气、日历、待办、健康、财务、媒体等。
package personalportal

import (
	"time"
)

// WidgetDataProvider 小组件数据提供者接口。
type WidgetDataProvider interface {
	GetData(widgetType WidgetType, config WidgetConfig) (interface{}, error)
}

// MockDataProvider 模拟数据提供者。
type MockDataProvider struct{}

// GetData 获取模拟数据。
func (m *MockDataProvider) GetData(widgetType WidgetType, config WidgetConfig) (interface{}, error) {
	switch widgetType {
	case WidgetTypeWeather:
		return m.getWeatherData(config), nil
	case WidgetTypeCalendar:
		return m.getCalendarData(config), nil
	case WidgetTypeTodo:
		return m.getTodoData(config), nil
	case WidgetTypeHealth:
		return m.getHealthData(config), nil
	case WidgetTypeFinance:
		return m.getFinanceData(config), nil
	case WidgetTypeMedia:
		return m.getMediaData(config), nil
	case WidgetTypeClock:
		return m.getClockData(), nil
	case WidgetTypeNotes:
		return m.getNotesData(), nil
	default:
		return nil, nil
	}
}

func (m *MockDataProvider) getWeatherData(config WidgetConfig) *WeatherData {
	return &WeatherData{
		City:        config.City,
		Temperature: 22.5,
		FeelsLike:   21.0,
		Humidity:    65,
		WindSpeed:   12.3,
		Description: "晴",
		Icon:        "sun",
		UpdatedAt:   time.Now(),
	}
}

func (m *MockDataProvider) getCalendarData(config WidgetConfig) []CalendarEvent {
	now := time.Now()
	return []CalendarEvent{
		{
			ID:    "event1",
			Title: "团队会议",
			Start: now.Add(2 * time.Hour),
			End:   now.Add(3 * time.Hour),
			Color: "#4CAF50",
		},
		{
			ID:    "event2",
			Title: "代码评审",
			Start: now.Add(5 * time.Hour),
			End:   now.Add(6 * time.Hour),
			Color: "#2196F3",
		},
	}
}

func (m *MockDataProvider) getTodoData(config WidgetConfig) []TodoItem {
	return []TodoItem{
		{
			ID:        "todo1",
			Title:     "完成报告",
			Completed: false,
			Priority:  "high",
		},
		{
			ID:        "todo2",
			Title:     "代码测试",
			Completed: false,
			Priority:  "medium",
		},
		{
			ID:        "todo3",
			Title:     "文档更新",
			Completed: true,
			Priority:  "low",
		},
	}
}

func (m *MockDataProvider) getHealthData(config WidgetConfig) *HealthData {
	return &HealthData{
		Steps:      8500,
		StepGoal:   10000,
		Calories:   1850,
		SleepHours: 7.5,
		SleepGoal:  8.0,
		HeartRate:  72,
		Water:      1500,
		WaterGoal:  2000,
		UpdatedAt:  time.Now(),
	}
}

func (m *MockDataProvider) getFinanceData(config WidgetConfig) *FinanceData {
	return &FinanceData{
		Balance:  15000.50,
		Income:   8000.00,
		Expenses: 5500.00,
		Currency: config.Currency,
		Accounts: []AccountSummary{
			{Name: "储蓄账户", Balance: 10000.00, Type: "savings"},
			{Name: "日常账户", Balance: 5000.50, Type: "checking"},
		},
		UpdatedAt: time.Now(),
	}
}

func (m *MockDataProvider) getMediaData(config WidgetConfig) *MediaData {
	return &MediaData{
		NowPlaying: &MediaItem{
			ID:       "media1",
			Title:    "放松音乐",
			Artist:   "自然之声",
			Duration: 240,
			Type:     "music",
		},
		RecentlyPlayed: []MediaItem{
			{ID: "media2", Title: "播客节目", Artist: "技术频道", Duration: 1800, Type: "podcast"},
		},
	}
}

func (m *MockDataProvider) getClockData() map[string]interface{} {
	now := time.Now()
	return map[string]interface{}{
		"time":     now.Format("15:04:05"),
		"date":     now.Format("2006-01-02"),
		"weekday":  now.Weekday().String(),
		"timezone": now.Location().String(),
	}
}

func (m *MockDataProvider) getNotesData() []map[string]string {
	return []map[string]string{
		{"title": "备忘录", "content": "记得买菜"},
		{"title": "想法", "content": "新项目规划"},
	}
}

// WidgetManager 小组件管理器。
type WidgetManager struct {
	engine     *PortalEngine
	dataProvider WidgetDataProvider
}

// NewWidgetManager 创建小组件管理器。
func NewWidgetManager(engine *PortalEngine) *WidgetManager {
	return &WidgetManager{
		engine:       engine,
		dataProvider: &MockDataProvider{},
	}
}

// CreateWeatherWidget 创建天气小组件。
func (wm *WidgetManager) CreateWeatherWidget(portalID, city string, position WidgetPosition) (*Widget, error) {
	widget := &Widget{
		Type:  WidgetTypeWeather,
		Title: "天气",
		Position: position,
		Config: WidgetConfig{
			City:         city,
			Units:        "metric",
			ShowForecast: true,
		},
		RefreshRate: 1800, // 30分钟
	}

	return wm.engine.AddWidget(portalID, widget)
}

// CreateCalendarWidget 创建日历小组件。
func (wm *WidgetManager) CreateCalendarWidget(portalID string, daysAhead int, position WidgetPosition) (*Widget, error) {
	widget := &Widget{
		Type:  WidgetTypeCalendar,
		Title: "日历",
		Position: position,
		Config: WidgetConfig{
			DaysAhead: daysAhead,
		},
		RefreshRate: 300, // 5分钟
	}

	return wm.engine.AddWidget(portalID, widget)
}

// CreateTodoWidget 创建待办小组件。
func (wm *WidgetManager) CreateTodoWidget(portalID, listID string, maxItems int, position WidgetPosition) (*Widget, error) {
	widget := &Widget{
		Type:  WidgetTypeTodo,
		Title: "待办事项",
		Position: position,
		Config: WidgetConfig{
			ListID:   listID,
			MaxItems: maxItems,
			ShowDone: false,
		},
		RefreshRate: 60, // 1分钟
	}

	return wm.engine.AddWidget(portalID, widget)
}

// CreateHealthWidget 创建健康小组件。
func (wm *WidgetManager) CreateHealthWidget(portalID string, position WidgetPosition) (*Widget, error) {
	widget := &Widget{
		Type:  WidgetTypeHealth,
		Title: "健康",
		Position: position,
		Config: WidgetConfig{
			ShowSteps: true,
			ShowSleep: true,
			ShowHeart: true,
		},
		RefreshRate: 300, // 5分钟
	}

	return wm.engine.AddWidget(portalID, widget)
}

// CreateFinanceWidget 创建财务小组件。
func (wm *WidgetManager) CreateFinanceWidget(portalID, currency string, position WidgetPosition) (*Widget, error) {
	widget := &Widget{
		Type:  WidgetTypeFinance,
		Title: "财务",
		Position: position,
		Config: WidgetConfig{
			Currency: currency,
		},
		RefreshRate: 600, // 10分钟
	}

	return wm.engine.AddWidget(portalID, widget)
}

// CreateMediaWidget 创建媒体小组件。
func (wm *WidgetManager) CreateMediaWidget(portalID string, position WidgetPosition) (*Widget, error) {
	widget := &Widget{
		Type:  WidgetTypeMedia,
		Title: "媒体",
		Position: position,
		RefreshRate: 30, // 30秒
	}

	return wm.engine.AddWidget(portalID, widget)
}

// CreateClockWidget 创建时钟小组件。
func (wm *WidgetManager) CreateClockWidget(portalID string, position WidgetPosition) (*Widget, error) {
	widget := &Widget{
		Type:  WidgetTypeClock,
		Title: "时钟",
		Position: position,
		RefreshRate: 1, // 1秒
	}

	return wm.engine.AddWidget(portalID, widget)
}

// CreateRSSWidget 创建 RSS 小组件。
func (wm *WidgetManager) CreateRSSWidget(portalID string, feedURLs []string, maxArticles int, position WidgetPosition) (*Widget, error) {
	widget := &Widget{
		Type:  WidgetTypeRSS,
		Title: "RSS 订阅",
		Position: position,
		Config: WidgetConfig{
			FeedURLs:    feedURLs,
			MaxArticles: maxArticles,
		},
		RefreshRate: 900, // 15分钟
	}

	return wm.engine.AddWidget(portalID, widget)
}

// CreateEmailWidget 创建邮件小组件。
func (wm *WidgetManager) CreateEmailWidget(portalID string, mailboxes []string, position WidgetPosition) (*Widget, error) {
	widget := &Widget{
		Type:  WidgetTypeEmail,
		Title: "邮件",
		Position: position,
		Config: WidgetConfig{
			Mailboxes:  mailboxes,
			UnreadOnly: true,
		},
		RefreshRate: 120, // 2分钟
	}

	return wm.engine.AddWidget(portalID, widget)
}

// RefreshWidgetData 刷新小组件数据。
func (wm *WidgetManager) RefreshWidgetData(portalID, widgetID string) (interface{}, error) {
	widget, err := wm.engine.GetWidget(portalID, widgetID)
	if err != nil {
		return nil, err
	}

	data, err := wm.dataProvider.GetData(widget.Type, widget.Config)
	if err != nil {
		return nil, err
	}

	widget.Data = data
	widget.UpdatedAt = time.Now()

	return data, nil
}

// RefreshAllWidgets 刷新所有小组件数据。
func (wm *WidgetManager) RefreshAllWidgets(portalID string) error {
	widgets, err := wm.engine.ListWidgets(portalID)
	if err != nil {
		return err
	}

	for _, widget := range widgets {
		if !widget.Enabled {
			continue
		}

		data, err := wm.dataProvider.GetData(widget.Type, widget.Config)
		if err != nil {
			continue
		}

		widget.Data = data
		widget.UpdatedAt = time.Now()
	}

	return nil
}

// GetWidgetTypes 获取支持的小组件类型。
func (wm *WidgetManager) GetWidgetTypes() []map[string]string {
	return []map[string]string{
		{"type": string(WidgetTypeWeather), "name": "天气", "icon": "cloud", "default_size": "medium"},
		{"type": string(WidgetTypeCalendar), "name": "日历", "icon": "calendar", "default_size": "medium"},
		{"type": string(WidgetTypeTodo), "name": "待办事项", "icon": "check-square", "default_size": "small"},
		{"type": string(WidgetTypeHealth), "name": "健康", "icon": "heart", "default_size": "medium"},
		{"type": string(WidgetTypeFinance), "name": "财务", "icon": "dollar-sign", "default_size": "medium"},
		{"type": string(WidgetTypeMedia), "name": "媒体", "icon": "music", "default_size": "small"},
		{"type": string(WidgetTypeClock), "name": "时钟", "icon": "clock", "default_size": "small"},
		{"type": string(WidgetTypeNotes), "name": "笔记", "icon": "edit", "default_size": "medium"},
		{"type": string(WidgetTypeRSS), "name": "RSS 订阅", "icon": "rss", "default_size": "large"},
		{"type": string(WidgetTypeEmail), "name": "邮件", "icon": "mail", "default_size": "medium"},
		{"type": string(WidgetTypeStats), "name": "统计", "icon": "bar-chart", "default_size": "medium"},
	}
}

// GetDefaultWidgetSize 获取默认小组件尺寸。
func GetDefaultWidgetSize(widgetType WidgetType) WidgetPosition {
	switch widgetType {
	case WidgetTypeWeather:
		return WidgetPosition{W: 2, H: 1}
	case WidgetTypeCalendar:
		return WidgetPosition{W: 2, H: 2}
	case WidgetTypeTodo:
		return WidgetPosition{W: 1, H: 2}
	case WidgetTypeHealth:
		return WidgetPosition{W: 2, H: 1}
	case WidgetTypeFinance:
		return WidgetPosition{W: 2, H: 1}
	case WidgetTypeMedia:
		return WidgetPosition{W: 1, H: 1}
	case WidgetTypeClock:
		return WidgetPosition{W: 1, H: 1}
	case WidgetTypeNotes:
		return WidgetPosition{W: 2, H: 2}
	case WidgetTypeRSS:
		return WidgetPosition{W: 2, H: 2}
	case WidgetTypeEmail:
		return WidgetPosition{W: 2, H: 2}
	default:
		return WidgetPosition{W: 1, H: 1}
	}
}
