// Package personalportal 提供个人门户仪表盘功能，支持可拖拽布局和信息流聚合。
package personalportal

import "time"

// WidgetType 小组件类型。
type WidgetType string

const (
	WidgetTypeWeather   WidgetType = "weather"
	WidgetTypeCalendar  WidgetType = "calendar"
	WidgetTypeTodo      WidgetType = "todo"
	WidgetTypeHealth    WidgetType = "health"
	WidgetTypeFinance   WidgetType = "finance"
	WidgetTypeMedia     WidgetType = "media"
	WidgetTypeClock     WidgetType = "clock"
	WidgetTypeNotes     WidgetType = "notes"
	WidgetTypeRSS       WidgetType = "rss"
	WidgetTypeEmail     WidgetType = "email"
	WidgetTypeStats     WidgetType = "stats"
	WidgetTypeCustom    WidgetType = "custom"
)

// WidgetSize 小组件尺寸。
type WidgetSize string

const (
	WidgetSizeSmall   WidgetSize = "small"   // 1x1
	WidgetSizeMedium  WidgetSize = "medium"  // 2x1
	WidgetSizeLarge   WidgetSize = "large"   // 2x2
	WidgetSizeWide    WidgetSize = "wide"    // 3x1
	WidgetSizeFull    WidgetSize = "full"    // 4x1
)

// Theme 主题类型。
type Theme string

const (
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
	ThemeAuto   Theme = "auto"
	ThemeCustom Theme = "custom"
)

// FeedSource 信息流来源类型。
type FeedSource string

const (
	FeedSourceRSS      FeedSource = "rss"
	FeedSourceEmail    FeedSource = "email"
	FeedSourceCalendar FeedSource = "calendar"
	FeedSourceNotification FeedSource = "notification"
	FeedSourceCustom   FeedSource = "custom"
)

// Portal 个人门户。
type Portal struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Theme       Theme     `json:"theme"`
	Layout      Layout    `json:"layout"`
	Widgets     []*Widget `json:"widgets"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Layout 布局配置。
type Layout struct {
	Columns   int    `json:"columns"`    // 列数
	RowHeight int    `json:"row_height"` // 行高（像素）
	Gap       int    `json:"gap"`        // 间距
	Compact   bool   `json:"compact"`    // 紧凑模式
	Margin    []int  `json:"margin"`     // 外边距 [top, right, bottom, left]
}

// WidgetPosition 小组件位置。
type WidgetPosition struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	W      int `json:"w"` // 宽度（格数）
	H      int `json:"h"` // 高度（格数）
	MinW   int `json:"min_w,omitempty"`
	MinH   int `json:"min_h,omitempty"`
	MaxW   int `json:"max_w,omitempty"`
	MaxH   int `json:"max_h,omitempty"`
	Locked bool `json:"locked,omitempty"`
}

// Widget 小组件。
type Widget struct {
	ID          string         `json:"id"`
	Type        WidgetType     `json:"type"`
	Title       string         `json:"title"`
	Position    WidgetPosition `json:"position"`
	Config      WidgetConfig   `json:"config,omitempty"`
	Enabled     bool           `json:"enabled"`
	RefreshRate int            `json:"refresh_rate"` // 秒
	Data        interface{}    `json:"data,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// WidgetConfig 小组件配置。
type WidgetConfig struct {
	// 天气配置
	City        string `json:"city,omitempty"`
	Units       string `json:"units,omitempty"` // metric, imperial
	ShowForecast bool  `json:"show_forecast,omitempty"`

	// 日历配置
	CalendarIDs []string `json:"calendar_ids,omitempty"`
	DaysAhead   int      `json:"days_ahead,omitempty"`

	// 待办配置
	ListID      string   `json:"list_id,omitempty"`
	ShowDone    bool     `json:"show_done,omitempty"`
	MaxItems    int      `json:"max_items,omitempty"`

	// 健康配置
	ShowSteps   bool     `json:"show_steps,omitempty"`
	ShowSleep   bool     `json:"show_sleep,omitempty"`
	ShowHeart   bool     `json:"show_heart,omitempty"`

	// 财务配置
	Currency    string   `json:"currency,omitempty"`
	Accounts    []string `json:"accounts,omitempty"`

	// RSS 配置
	FeedURLs    []string `json:"feed_urls,omitempty"`
	MaxArticles int      `json:"max_articles,omitempty"`

	// 邮件配置
	Mailboxes   []string `json:"mailboxes,omitempty"`
	UnreadOnly  bool     `json:"unread_only,omitempty"`

	// 通用配置
	Theme       string   `json:"theme,omitempty"`
	Background  string   `json:"background,omitempty"`
	FontSize    int      `json:"font_size,omitempty"`
}

// WeatherData 天气数据。
type WeatherData struct {
	City        string        `json:"city"`
	Temperature float64       `json:"temperature"`
	FeelsLike   float64       `json:"feels_like"`
	Humidity    int           `json:"humidity"`
	WindSpeed   float64       `json:"wind_speed"`
	Description string        `json:"description"`
	Icon        string        `json:"icon"`
	Forecast    []ForecastDay `json:"forecast,omitempty"`
	UpdatedAt   time.Time     `json:"updated_at"`
}

// ForecastDay 天气预报。
type ForecastDay struct {
	Date        string  `json:"date"`
	TempMax     float64 `json:"temp_max"`
	TempMin     float64 `json:"temp_min"`
	Description string  `json:"description"`
	Icon        string  `json:"icon"`
}

// CalendarEvent 日历事件。
type CalendarEvent struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Start       time.Time `json:"start"`
	End         time.Time `json:"end"`
	Location    string    `json:"location,omitempty"`
	Description string    `json:"description,omitempty"`
	AllDay      bool      `json:"all_day"`
	Color       string    `json:"color,omitempty"`
}

// TodoItem 待办事项。
type TodoItem struct {
	ID          string     `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description,omitempty"`
	Completed   bool       `json:"completed"`
	Priority    string     `json:"priority"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	ListID      string     `json:"list_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// HealthData 健康数据。
type HealthData struct {
	Steps       int       `json:"steps"`
	StepGoal    int       `json:"step_goal"`
	Calories    int       `json:"calories"`
	SleepHours  float64   `json:"sleep_hours"`
	SleepGoal   float64   `json:"sleep_goal"`
	HeartRate   int       `json:"heart_rate"`
	Water       int       `json:"water"` // 毫升
	WaterGoal   int       `json:"water_goal"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// FinanceData 财务数据。
type FinanceData struct {
	Balance     float64          `json:"balance"`
	Income      float64          `json:"income"`
	Expenses    float64          `json:"expenses"`
	Currency    string           `json:"currency"`
	Accounts    []AccountSummary `json:"accounts"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// AccountSummary 账户摘要。
type AccountSummary struct {
	Name    string  `json:"name"`
	Balance float64 `json:"balance"`
	Type    string  `json:"type"`
}

// MediaData 媒体数据。
type MediaData struct {
	NowPlaying  *MediaItem   `json:"now_playing,omitempty"`
	RecentlyPlayed []MediaItem `json:"recently_played,omitempty"`
	Favorites   []MediaItem  `json:"favorites,omitempty"`
}

// MediaItem 媒体项目。
type MediaItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Artist   string `json:"artist,omitempty"`
	Album    string `json:"album,omitempty"`
	Cover    string `json:"cover,omitempty"`
	Duration int    `json:"duration"` // 秒
	Type     string `json:"type"`     // music, podcast, audiobook
}

// RSSFeed RSS 数据。
type RSSFeed struct {
	URL      string        `json:"url"`
	Title    string        `json:"title"`
	Articles []RSSArticle  `json:"articles"`
	UpdatedAt time.Time    `json:"updated_at"`
}

// RSSArticle RSS 文章。
type RSSArticle struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Link        string    `json:"link"`
	Description string    `json:"description"`
	Published   time.Time `json:"published"`
	Author      string    `json:"author,omitempty"`
	FeedTitle   string    `json:"feed_title"`
	Read        bool      `json:"read"`
}

// EmailSummary 邮件摘要。
type EmailSummary struct {
	ID          string    `json:"id"`
	From        string    `json:"from"`
	Subject     string    `json:"subject"`
	Preview     string    `json:"preview"`
	ReceivedAt  time.Time `json:"received_at"`
	IsRead      bool      `json:"is_read"`
	HasAttach   bool      `json:"has_attach"`
	Mailbox     string    `json:"mailbox"`
}

// Notification 通知。
type Notification struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	Type      string    `json:"type"` // info, warning, error, success
	Source    string    `json:"source"`
	Read      bool      `json:"read"`
	ActionURL string    `json:"action_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// FeedItem 信息流项目。
type FeedItem struct {
	ID        string     `json:"id"`
	Source    FeedSource `json:"source"`
	Title     string     `json:"title"`
	Summary   string     `json:"summary"`
	Link      string     `json:"link,omitempty"`
	Icon      string     `json:"icon,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
	Read      bool       `json:"read"`
	Category  string     `json:"category,omitempty"`
}

// FeedConfig 信息流配置。
type FeedConfig struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Name        string     `json:"name"`
	Source      FeedSource `json:"source"`
	URL         string     `json:"url,omitempty"`
	RefreshRate int        `json:"refresh_rate"` // 秒
	Enabled     bool       `json:"enabled"`
	MaxItems    int        `json:"max_items"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// UserPreferences 用户偏好设置。
type UserPreferences struct {
	UserID          string            `json:"user_id"`
	Theme           Theme             `json:"theme"`
	Language        string            `json:"language"`
	TimeZone        string            `json:"time_zone"`
	DateFormat      string            `json:"date_format"`
	NotificationOn  bool              `json:"notification_on"`
	CustomCSS       string            `json:"custom_css,omitempty"`
	WidgetDefaults  map[string]string `json:"widget_defaults,omitempty"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// PortalStats 门户统计。
type PortalStats struct {
	WidgetCount    int    `json:"widget_count"`
	FeedItemCount  int    `json:"feed_item_count"`
	UnreadCount    int    `json:"unread_count"`
	LastActive     time.Time `json:"last_active"`
}
